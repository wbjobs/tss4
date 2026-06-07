package prometheus

import (
	"bytes"
	"fmt"
	"gorilla-tsdb/util"
	"testing"
)

func TestLabelMarshalUnmarshal(t *testing.T) {
	labels := []Label{
		{Name: "__name__", Value: "cpu_usage"},
		{Name: "host", Value: "server1"},
		{Name: "instance", Value: "0"},
	}

	labelsMap := make(map[string]string)
	for _, l := range labels {
		labelsMap[l.Name] = l.Value
	}

	wr := WriteRequest{
		TimeSeries: []TimeSeries{
			{
				Labels: labels,
				Samples: []Sample{
					{Timestamp: 1600000000, Value: 0.5},
					{Timestamp: 1600000001, Value: 0.6},
					{Timestamp: 1600000002, Value: 0.7},
				},
			},
		},
	}

	data := wr.Marshal()
	fmt.Printf("Marshaled size: %d bytes\n", len(data))

	wr2, err := UnmarshalWriteRequest(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(wr2.TimeSeries) != 1 {
		t.Fatalf("Expected 1 time series, got %d", len(wr2.TimeSeries))
	}

	ts := wr2.TimeSeries[0]
	if len(ts.Labels) != len(labels) {
		t.Errorf("Expected %d labels, got %d", len(labels), len(ts.Labels))
	}

	for i, l := range ts.Labels {
		if l.Name != labels[i].Name || l.Value != labels[i].Value {
			t.Errorf("Label mismatch at %d: expected %s=%s, got %s=%s",
				i, labels[i].Name, labels[i].Value, l.Name, l.Value)
		}
	}

	if len(ts.Samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(ts.Samples))
	}

	if ts.Samples[0].Timestamp != 1600000000 || ts.Samples[0].Value != 0.5 {
		t.Errorf("Sample 0 mismatch")
	}

	seriesKey := LabelsToKey(labels)
	fmt.Printf("Series key: %s\n", seriesKey)

	metricName, hasName := GetMetricName(labels)
	if !hasName || metricName != "cpu_usage" {
		t.Errorf("Expected metric name 'cpu_usage', got '%s'", metricName)
	}
}

func TestReadRequestMarshal(t *testing.T) {
	rr := ReadRequest{
		Queries: []Query{
			{
				StartTimestampMs: 1600000000,
				EndTimestampMs:   1600001000,
				Matchers: []LabelMatcher{
					{Type: MatchEqual, Name: "__name__", Value: "cpu_usage"},
					{Type: MatchEqual, Name: "host", Value: "server1"},
				},
			},
		},
	}

	data := rr.Marshal()
	fmt.Printf("ReadRequest marshaled size: %d bytes\n", len(data))

	rr2, err := UnmarshalReadRequest(data)
	if err != nil {
		t.Fatalf("Unmarshal ReadRequest failed: %v", err)
	}

	if len(rr2.Queries) != 1 {
		t.Fatalf("Expected 1 query, got %d", len(rr2.Queries))
	}

	q := rr2.Queries[0]
	if q.StartTimestampMs != 1600000000 {
		t.Errorf("Start timestamp mismatch")
	}
	if q.EndTimestampMs != 1600001000 {
		t.Errorf("End timestamp mismatch")
	}
	if len(q.Matchers) != 2 {
		t.Errorf("Expected 2 matchers, got %d", len(q.Matchers))
	}
}

func TestReadResponseMarshal(t *testing.T) {
	rr := ReadResponse{
		Results: []QueryResult{
			{
				TimeSeries: []TimeSeries{
					{
						Labels: []Label{
							{Name: "__name__", Value: "cpu_usage"},
							{Name: "host", Value: "server1"},
						},
						Samples: []Sample{
							{Timestamp: 1600000000, Value: 0.5},
							{Timestamp: 1600000001, Value: 0.6},
						},
					},
				},
			},
		},
	}

	data := rr.Marshal()
	fmt.Printf("ReadResponse marshaled size: %d bytes\n", len(data))

	rr2, err := UnmarshalReadResponse(data)
	if err != nil {
		t.Fatalf("Unmarshal ReadResponse failed: %v", err)
	}

	if len(rr2.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(rr2.Results))
	}

	result := rr2.Results[0]
	if len(result.TimeSeries) != 1 {
		t.Fatalf("Expected 1 time series in result, got %d", len(result.TimeSeries))
	}

	ts := result.TimeSeries[0]
	if len(ts.Samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(ts.Samples))
	}
}

func TestLabelsToKey(t *testing.T) {
	labels1 := []Label{
		{Name: "b", Value: "2"},
		{Name: "a", Value: "1"},
	}

	labels2 := []Label{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}

	key1 := LabelsToKey(labels1)
	key2 := LabelsToKey(labels2)

	if key1 != key2 {
		t.Error("Same labels in different order should produce same key")
	}

	fmt.Printf("Labels key: %s\n", key1)
}

func TestGetMetricName(t *testing.T) {
	labels := []Label{
		{Name: "host", Value: "server1"},
		{Name: "__name__", Value: "my_metric"},
	}

	name, hasName := GetMetricName(labels)
	if !hasName {
		t.Error("Should find metric name")
	}
	if name != "my_metric" {
		t.Errorf("Expected 'my_metric', got '%s'", name)
	}

	labelsNoName := []Label{
		{Name: "host", Value: "server1"},
	}

	_, hasName = GetMetricName(labelsNoName)
	if hasName {
		t.Error("Should not find metric name")
	}
}

func TestSnappyRoundtripWithProtobuf(t *testing.T) {
	wr := WriteRequest{
		TimeSeries: []TimeSeries{
			{
				Labels: []Label{
					{Name: "__name__", Value: "test_metric"},
				},
				Samples: make([]Sample, 100),
			},
		},
	}

	for i := range wr.TimeSeries[0].Samples {
		wr.TimeSeries[0].Samples[i] = Sample{
			Timestamp: int64(1600000000 + i),
			Value:     float64(i) * 0.1,
		}
	}

	data := wr.Marshal()
	fmt.Printf("Protobuf size: %d bytes\n", len(data))

	// Test with snappy compression
	var buf bytes.Buffer
	compressed := util.SnappyCompress(data)
	fmt.Printf("Snappy compressed size: %d bytes, ratio: %.2f\n",
		len(compressed), float64(len(compressed))/float64(len(data)))
	_ = buf

	decompressed, err := util.SnappyDecompress(compressed)
	if err != nil {
		t.Fatalf("Snappy decompress failed: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Error("Snappy roundtrip failed")
	}

	wr2, err := UnmarshalWriteRequest(decompressed)
	if err != nil {
		t.Fatalf("Unmarshal after snappy failed: %v", err)
	}

	if len(wr2.TimeSeries[0].Samples) != 100 {
		t.Error("Sample count mismatch after snappy roundtrip")
	}
}
