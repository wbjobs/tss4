package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"gorilla-tsdb/block"
	"gorilla-tsdb/prometheus"
	"gorilla-tsdb/tsdb"
	"gorilla-tsdb/util"
	"io/ioutil"
	"net/http"
	"strconv"
)

type PrometheusHandler struct {
	mdb *tsdb.MultiTSDB
}

func NewPrometheusHandler(mdb *tsdb.MultiTSDB) *PrometheusHandler {
	return &PrometheusHandler{mdb: mdb}
}

func (h *PrometheusHandler) RemoteWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "read body: %v", err)
		return
	}

	contentEncoding := r.Header.Get("Content-Encoding")
	if contentEncoding == "snappy" {
		body, err = util.SnappyDecompress(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "decompress snappy: %v", err)
			return
		}
	}

	wr, err := prometheus.UnmarshalWriteRequest(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unmarshal write request: %v", err)
		return
	}

	for _, ts := range wr.TimeSeries {
		seriesKey := prometheus.LabelsToKey(ts.Labels)

		labelsMap := make(map[string]string, len(ts.Labels))
		for _, l := range ts.Labels {
			labelsMap[l.Name] = l.Value
		}

		for _, s := range ts.Samples {
			point := block.DataPoint{
				Timestamp: s.Timestamp * 1e6,
				Value:     s.Value,
			}

			if err := h.mdb.Write(seriesKey, labelsMap, point); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "write point: %v", err)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PrometheusHandler) RemoteRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "read body: %v", err)
		return
	}

	contentEncoding := r.Header.Get("Content-Encoding")
	if contentEncoding == "snappy" {
		body, err = util.SnappyDecompress(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "decompress snappy: %v", err)
			return
		}
	}

	rr, err := prometheus.UnmarshalReadRequest(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unmarshal read request: %v", err)
		return
	}

	downsampleThreshold := 0
	if dsStr := r.URL.Query().Get("downsample"); dsStr != "" {
		if ds, err := strconv.Atoi(dsStr); err == nil && ds > 0 {
			downsampleThreshold = ds
		}
	}

	response := prometheus.ReadResponse{}

	for _, query := range rr.Queries {
		matchers := make(map[string]string)
		for _, m := range query.Matchers {
			if m.Type == prometheus.MatchEqual {
				matchers[m.Name] = m.Value
			}
		}

		seriesKeys := h.mdb.FindSeriesByMatcher(matchers)

		qr := prometheus.QueryResult{}

		for _, seriesKey := range seriesKeys {
			labelsMap, _ := h.mdb.GetLabels(seriesKey)

			startNs := query.StartTimestampMs * 1e6
			endNs := query.EndTimestampMs * 1e6

			points, err := h.mdb.Query(seriesKey, startNs, endNs, downsampleThreshold)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "query: %v", err)
				return
			}

			labels := make([]prometheus.Label, 0, len(labelsMap))
			for k, v := range labelsMap {
				labels = append(labels, prometheus.Label{Name: k, Value: v})
			}

			samples := make([]prometheus.Sample, 0, len(points))
			for _, p := range points {
				samples = append(samples, prometheus.Sample{
					Timestamp: p.Timestamp / 1e6,
					Value:     p.Value,
				})
			}

			qr.TimeSeries = append(qr.TimeSeries, prometheus.TimeSeries{
				Labels:  labels,
				Samples: samples,
			})
		}

		response.Results = append(response.Results, qr)
	}

	respData := response.Marshal()

	acceptEncoding := r.Header.Get("Accept-Encoding")
	if acceptEncoding == "snappy" {
		respData = util.SnappyCompress(respData)
		w.Header().Set("Content-Encoding", "snappy")
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Length", strconv.Itoa(len(respData)))
	w.WriteHeader(http.StatusOK)
	w.Write(respData)
}

func (h *PrometheusHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/write", h.RemoteWrite)
	mux.HandleFunc("/api/v1/read", h.RemoteRead)
}

func ParseUint64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

func WriteUint64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}
