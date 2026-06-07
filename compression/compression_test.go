package compression

import (
	"math"
	"math/rand"
	"testing"
)

func TestBitWriterReader(t *testing.T) {
	w := NewBitWriter()

	w.WriteBit(true)
	w.WriteBit(false)
	w.WriteBit(true)
	w.WriteBits(0xA5, 8)
	w.WriteBits(0x1234, 16)

	data := w.Bytes()

	r := NewBitReader(data)

	bit, err := r.ReadBit()
	if err != nil || !bit {
		t.Fatalf("expected true, got %v, err %v", bit, err)
	}

	bit, err = r.ReadBit()
	if err != nil || bit {
		t.Fatalf("expected false, got %v, err %v", bit, err)
	}

	bit, err = r.ReadBit()
	if err != nil || !bit {
		t.Fatalf("expected true, got %v, err %v", bit, err)
	}

	v, err := r.ReadBits(8)
	if err != nil || v != 0xA5 {
		t.Fatalf("expected 0xA5, got 0x%x, err %v", v, err)
	}

	v, err = r.ReadBits(16)
	if err != nil || v != 0x1234 {
		t.Fatalf("expected 0x1234, got 0x%x, err %v", v, err)
	}
}

func TestVarint(t *testing.T) {
	tests := []int64{0, 1, -1, 63, -64, 127, -128, 255, -256, 1000, -1000, 1<<30, -(1 << 30)}

	for _, x := range tests {
		w := NewBitWriter()
		WriteVarint(w, x)

		r := NewBitReader(w.Bytes())
		y, err := ReadVarint(r)
		if err != nil {
			t.Fatalf("varint %d: read error: %v", x, err)
		}
		if y != x {
			t.Fatalf("varint %d: got %d", x, y)
		}
	}
}

func TestTimestampCompression(t *testing.T) {
	tests := []struct {
		name      string
		timestamps []int64
	}{
		{
			name: "constant interval 1s",
			timestamps: generateTimestamps(100, 1000, 1000),
		},
		{
			name: "constant interval 10s",
			timestamps: generateTimestamps(100, 1000000000, 10000000000),
		},
		{
			name: "irregular intervals",
			timestamps: generateIrregularTimestamps(100, 1000),
		},
		{
			name: "single point",
			timestamps: []int64{1234567890},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewBitWriter()
			enc := NewTimestampEncoder(w)

			for _, ts := range tt.timestamps {
				enc.Encode(ts)
			}

			data := w.Bytes()
			t.Logf("Compressed %d timestamps to %d bytes", len(tt.timestamps), len(data))

			r := NewBitReader(data)
			dec := NewTimestampDecoder(r)

			for i, expected := range tt.timestamps {
				got, err := dec.Decode()
				if err != nil {
					t.Fatalf("timestamp %d: decode error: %v", i, err)
				}
				if got != expected {
					t.Fatalf("timestamp %d: expected %d, got %d", i, expected, got)
				}
			}
		})
	}
}

func TestXORCompression(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
	}{
		{
			name:   "constant value",
			values: generateConstantValues(100, 42.5),
		},
		{
			name:   "slowly changing",
			values: generateSlowlyChangingValues(100, 100.0, 0.01),
		},
		{
			name:   "random floats",
			values: generateRandomFloats(100),
		},
		{
			name:   "single value",
			values: []float64{3.14159},
		},
		{
			name:   "zero and one",
			values: []float64{0, 1, 0, 1, 0, 1, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewBitWriter()
			enc := NewXOREncoder(w)

			for _, v := range tt.values {
				enc.Encode(v)
			}

			data := w.Bytes()
			t.Logf("Compressed %d values to %d bytes", len(tt.values), len(data))

			r := NewBitReader(data)
			dec := NewXORDecoder(r)

			for i, expected := range tt.values {
				got, err := dec.Decode()
				if err != nil {
					t.Fatalf("value %d: decode error: %v", i, err)
				}
				if math.Abs(got-expected) > 1e-12 {
					expectedBits := math.Float64bits(expected)
					gotBits := math.Float64bits(got)
					if expectedBits != gotBits {
						t.Fatalf("value %d: expected %v (0x%x), got %v (0x%x)", i, expected, expectedBits, got, gotBits)
					}
				}
			}
		})
	}
}

func TestSignExtend(t *testing.T) {
	tests := []struct {
		x      int64
		n      int
		expect int64
	}{
		{0x3F, 7, 63},
		{0x40, 7, -64},
		{0x7F, 7, -1},
		{0x1FF, 9, -1},
		{0x100, 9, -256},
	}

	for _, tt := range tests {
		got := signExtend(tt.x, tt.n)
		if got != tt.expect {
			t.Errorf("signExtend(0x%x, %d) = %d, expected %d", tt.x, tt.n, got, tt.expect)
		}
	}
}

func TestCountLeadingZeros(t *testing.T) {
	tests := []struct {
		x      uint64
		expect int
	}{
		{0, 64},
		{1, 63},
		{0x8000000000000000, 0},
		{0x0000FFFFFFFFFFFF, 16},
		{0x00000000FFFFFFFF, 32},
	}

	for _, tt := range tests {
		got := countLeadingZeros(tt.x)
		if got != tt.expect {
			t.Errorf("countLeadingZeros(0x%x) = %d, expected %d", tt.x, got, tt.expect)
		}
	}
}

func TestCountTrailingZeros(t *testing.T) {
	tests := []struct {
		x      uint64
		expect int
	}{
		{0, 64},
		{1, 0},
		{0x8000000000000000, 63},
		{0xFFFFFFFFFFFF0000, 16},
		{0xFFFFFFFF00000000, 32},
	}

	for _, tt := range tests {
		got := countTrailingZeros(tt.x)
		if got != tt.expect {
			t.Errorf("countTrailingZeros(0x%x) = %d, expected %d", tt.x, got, tt.expect)
		}
	}
}

func generateTimestamps(n int, start int64, interval int64) []int64 {
	ts := make([]int64, n)
	for i := 0; i < n; i++ {
		ts[i] = start + int64(i)*interval
	}
	return ts
}

func generateIrregularTimestamps(n int, start int64) []int64 {
	ts := make([]int64, n)
	ts[0] = start
	for i := 1; i < n; i++ {
		delta := rand.Int63n(5000) + 500
		ts[i] = ts[i-1] + delta
	}
	return ts
}

func generateConstantValues(n int, v float64) []float64 {
	values := make([]float64, n)
	for i := 0; i < n; i++ {
		values[i] = v
	}
	return values
}

func generateSlowlyChangingValues(n int, start float64, step float64) []float64 {
	values := make([]float64, n)
	values[0] = start
	for i := 1; i < n; i++ {
		values[i] = values[i-1] + step
	}
	return values
}

func generateRandomFloats(n int) []float64 {
	values := make([]float64, n)
	for i := 0; i < n; i++ {
		values[i] = rand.Float64() * 1000
	}
	return values
}
