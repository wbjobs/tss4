package prometheus

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Label struct {
	Name  string
	Value string
}

type Sample struct {
	Timestamp int64
	Value     float64
}

type TimeSeries struct {
	Labels  []Label
	Samples []Sample
}

type WriteRequest struct {
	TimeSeries []TimeSeries
}

type LabelMatcherType int32

const (
	MatchEqual     LabelMatcherType = 0
	MatchNotEqual  LabelMatcherType = 1
	MatchRegexp    LabelMatcherType = 2
	MatchNotRegexp LabelMatcherType = 3
)

type LabelMatcher struct {
	Type  LabelMatcherType
	Name  string
	Value string
}

type Query struct {
	StartTimestampMs int64
	EndTimestampMs   int64
	Matchers         []LabelMatcher
}

type ReadRequest struct {
	Queries []Query
}

type QueryResult struct {
	TimeSeries []TimeSeries
}

type ReadResponse struct {
	Results []QueryResult
}

const (
	wireVarint     = 0
	wire64bit      = 1
	wireDelimited  = 2
	wire32bit      = 5
)

func makeTag(fieldNum int, wireType int) int {
	return (fieldNum << 3) | wireType
}

func encodeVarint(buf *[]byte, x uint64) {
	for x >= 0x80 {
		*buf = append(*buf, byte(x)|0x80)
		x >>= 7
	}
	*buf = append(*buf, byte(x))
}

func decodeVarint(data []byte, pos int) (uint64, int, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		if pos+i >= len(data) {
			return 0, 0, fmt.Errorf("unexpected EOF")
		}
		b := data[pos+i]
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, 0, fmt.Errorf("varint overflow")
			}
			return x | uint64(b)<<s, pos + i + 1, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

func encodeString(buf *[]byte, fieldNum int, s string) {
	encodeVarint(buf, uint64(makeTag(fieldNum, wireDelimited)))
	encodeVarint(buf, uint64(len(s)))
	*buf = append(*buf, []byte(s)...)
}

func encodeDouble(buf *[]byte, fieldNum int, v float64) {
	encodeVarint(buf, uint64(makeTag(fieldNum, wire64bit)))
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	*buf = append(*buf, b[:]...)
}

func encodeInt64(buf *[]byte, fieldNum int, v int64) {
	encodeVarint(buf, uint64(makeTag(fieldNum, wireVarint)))
	encodeVarint(buf, uint64(v))
}

func encodeMessage(buf *[]byte, fieldNum int, msg []byte) {
	encodeVarint(buf, uint64(makeTag(fieldNum, wireDelimited)))
	encodeVarint(buf, uint64(len(msg)))
	*buf = append(*buf, msg...)
}

func (l Label) Marshal() []byte {
	var buf []byte
	encodeString(&buf, 1, l.Name)
	encodeString(&buf, 2, l.Value)
	return buf
}

func (s Sample) Marshal() []byte {
	var buf []byte
	encodeDouble(&buf, 1, s.Value)
	encodeInt64(&buf, 2, s.Timestamp)
	return buf
}

func (ts TimeSeries) Marshal() []byte {
	var buf []byte
	for _, l := range ts.Labels {
		encodeMessage(&buf, 1, l.Marshal())
	}
	for _, s := range ts.Samples {
		encodeMessage(&buf, 2, s.Marshal())
	}
	return buf
}

func (wr WriteRequest) Marshal() []byte {
	var buf []byte
	for _, ts := range wr.TimeSeries {
		encodeMessage(&buf, 1, ts.Marshal())
	}
	return buf
}

func UnmarshalLabel(data []byte) (Label, error) {
	var l Label
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return l, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch fieldNum {
		case 1:
			if wireType != wireDelimited {
				return l, fmt.Errorf("invalid wire type for name")
			}
			strLen, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return l, err
			}
			pos = newPos
			l.Name = string(data[pos : pos+int(strLen)])
			pos += int(strLen)
		case 2:
			if wireType != wireDelimited {
				return l, fmt.Errorf("invalid wire type for value")
			}
			strLen, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return l, err
			}
			pos = newPos
			l.Value = string(data[pos : pos+int(strLen)])
			pos += int(strLen)
		default:
			if wireType == wireVarint {
				_, newPos, _ = decodeVarint(data, pos)
				pos = newPos
			} else if wireType == wire64bit {
				pos += 8
			} else if wireType == wireDelimited {
				strLen, newPos, _ := decodeVarint(data, pos)
				pos = newPos + int(strLen)
			} else if wireType == wire32bit {
				pos += 4
			}
		}
	}
	return l, nil
}

func UnmarshalSample(data []byte) (Sample, error) {
	var s Sample
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return s, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch fieldNum {
		case 1:
			if wireType != wire64bit {
				return s, fmt.Errorf("invalid wire type for value")
			}
			if pos+8 > len(data) {
				return s, fmt.Errorf("unexpected EOF")
			}
			s.Value = math.Float64frombits(binary.LittleEndian.Uint64(data[pos : pos+8]))
			pos += 8
		case 2:
			if wireType != wireVarint {
				return s, fmt.Errorf("invalid wire type for timestamp")
			}
			ts, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return s, err
			}
			s.Timestamp = int64(ts)
			pos = newPos
		default:
			if wireType == wireVarint {
				_, newPos, _ = decodeVarint(data, pos)
				pos = newPos
			} else if wireType == wire64bit {
				pos += 8
			} else if wireType == wireDelimited {
				strLen, newPos, _ := decodeVarint(data, pos)
				pos = newPos + int(strLen)
			} else if wireType == wire32bit {
				pos += 4
			}
		}
	}
	return s, nil
}

func UnmarshalTimeSeries(data []byte) (TimeSeries, error) {
	var ts TimeSeries
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return ts, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		if wireType != wireDelimited {
			return ts, fmt.Errorf("expected length-delimited field")
		}

		msgLen, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return ts, err
		}
		pos = newPos

		msgData := data[pos : pos+int(msgLen)]
		pos += int(msgLen)

		switch fieldNum {
		case 1:
			l, err := UnmarshalLabel(msgData)
			if err != nil {
				return ts, err
			}
			ts.Labels = append(ts.Labels, l)
		case 2:
			s, err := UnmarshalSample(msgData)
			if err != nil {
				return ts, err
			}
			ts.Samples = append(ts.Samples, s)
		}
	}
	return ts, nil
}

func UnmarshalWriteRequest(data []byte) (WriteRequest, error) {
	var wr WriteRequest
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return wr, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		if wireType != wireDelimited {
			return wr, fmt.Errorf("expected length-delimited field")
		}

		msgLen, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return wr, err
		}
		pos = newPos

		msgData := data[pos : pos+int(msgLen)]
		pos += int(msgLen)

		if fieldNum == 1 {
			ts, err := UnmarshalTimeSeries(msgData)
			if err != nil {
				return wr, err
			}
			wr.TimeSeries = append(wr.TimeSeries, ts)
		}
	}
	return wr, nil
}

func (q Query) Marshal() []byte {
	var buf []byte
	encodeInt64(&buf, 1, q.StartTimestampMs)
	encodeInt64(&buf, 2, q.EndTimestampMs)
	for _, m := range q.Matchers {
		encodeMessage(&buf, 3, m.Marshal())
	}
	return buf
}

func (m LabelMatcher) Marshal() []byte {
	var buf []byte
	encodeVarint(&buf, uint64(makeTag(1, wireVarint)))
	encodeVarint(&buf, uint64(m.Type))
	encodeString(&buf, 2, m.Name)
	encodeString(&buf, 3, m.Value)
	return buf
}

func (rr ReadRequest) Marshal() []byte {
	var buf []byte
	for _, q := range rr.Queries {
		encodeMessage(&buf, 1, q.Marshal())
	}
	return buf
}

func UnmarshalLabelMatcher(data []byte) (LabelMatcher, error) {
	var m LabelMatcher
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return m, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch fieldNum {
		case 1:
			if wireType != wireVarint {
				return m, fmt.Errorf("invalid wire type for type")
			}
			t, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return m, err
			}
			m.Type = LabelMatcherType(t)
			pos = newPos
		case 2:
			if wireType != wireDelimited {
				return m, fmt.Errorf("invalid wire type for name")
			}
			strLen, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return m, err
			}
			pos = newPos
			m.Name = string(data[pos : pos+int(strLen)])
			pos += int(strLen)
		case 3:
			if wireType != wireDelimited {
				return m, fmt.Errorf("invalid wire type for value")
			}
			strLen, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return m, err
			}
			pos = newPos
			m.Value = string(data[pos : pos+int(strLen)])
			pos += int(strLen)
		default:
			if wireType == wireVarint {
				_, newPos, _ = decodeVarint(data, pos)
				pos = newPos
			} else if wireType == wire64bit {
				pos += 8
			} else if wireType == wireDelimited {
				strLen, newPos, _ := decodeVarint(data, pos)
				pos = newPos + int(strLen)
			} else if wireType == wire32bit {
				pos += 4
			}
		}
	}
	return m, nil
}

func UnmarshalQuery(data []byte) (Query, error) {
	var q Query
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return q, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch fieldNum {
		case 1:
			if wireType != wireVarint {
				return q, fmt.Errorf("invalid wire type for start")
			}
			v, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return q, err
			}
			q.StartTimestampMs = int64(v)
			pos = newPos
		case 2:
			if wireType != wireVarint {
				return q, fmt.Errorf("invalid wire type for end")
			}
			v, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return q, err
			}
			q.EndTimestampMs = int64(v)
			pos = newPos
		case 3:
			if wireType != wireDelimited {
				return q, fmt.Errorf("expected length-delimited")
			}
			msgLen, newPos, err := decodeVarint(data, pos)
			if err != nil {
				return q, err
			}
			pos = newPos
			msgData := data[pos : pos+int(msgLen)]
			pos += int(msgLen)
			m, err := UnmarshalLabelMatcher(msgData)
			if err != nil {
				return q, err
			}
			q.Matchers = append(q.Matchers, m)
		default:
			if wireType == wireVarint {
				_, newPos, _ = decodeVarint(data, pos)
				pos = newPos
			} else if wireType == wire64bit {
				pos += 8
			} else if wireType == wireDelimited {
				strLen, newPos, _ := decodeVarint(data, pos)
				pos = newPos + int(strLen)
			} else if wireType == wire32bit {
				pos += 4
			}
		}
	}
	return q, nil
}

func UnmarshalReadRequest(data []byte) (ReadRequest, error) {
	var rr ReadRequest
	pos := 0
	for pos < len(data) {
		tag, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return rr, err
		}
		pos = newPos

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		if wireType != wireDelimited {
			return rr, fmt.Errorf("expected length-delimited field")
		}

		msgLen, newPos, err := decodeVarint(data, pos)
		if err != nil {
			return rr, err
		}
		pos = newPos

		msgData := data[pos : pos+int(msgLen)]
		pos += int(msgLen)

		if fieldNum == 1 {
			q, err := UnmarshalQuery(msgData)
			if err != nil {
				return rr, err
			}
			rr.Queries = append(rr.Queries, q)
		}
	}
	return rr, nil
}

func (qr QueryResult) Marshal() []byte {
	var buf []byte
	for _, ts := range qr.TimeSeries {
		encodeMessage(&buf, 1, ts.Marshal())
	}
	return buf
}

func (rr ReadResponse) Marshal() []byte {
	var buf []byte
	for _, qr := range rr.Results {
		encodeMessage(&buf, 1, qr.Marshal())
	}
	return buf
}

func LabelsToKey(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}

	sorted := make([]Label, len(labels))
	copy(sorted, labels)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var parts []string
	for _, l := range sorted {
		parts = append(parts, fmt.Sprintf("%s=%s", l.Name, l.Value))
	}
	return strings.Join(parts, ",")
}

func GetMetricName(labels []Label) string {
	for _, l := range labels {
		if l.Name == "__name__" {
			return l.Value
		}
	}
	return ""
}
