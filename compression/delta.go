package compression

import (
	"math"
)

type TimestampEncoder struct {
	w          *BitWriter
	prevTime   int64
	prevDelta  int64
	firstPoint bool
}

func NewTimestampEncoder(w *BitWriter) *TimestampEncoder {
	return &TimestampEncoder{
		w:          w,
		firstPoint: true,
	}
}

func (e *TimestampEncoder) Encode(timestamp int64) {
	if e.firstPoint {
		WriteVarint(e.w, timestamp)
		e.prevTime = timestamp
		e.prevDelta = 0
		e.firstPoint = false
		return
	}

	delta := timestamp - e.prevTime
	dod := delta - e.prevDelta

	if dod == 0 {
		e.w.WriteBit(false)
	} else if dod >= -63 && dod <= 64 {
		e.w.WriteBits(0x2, 2)
		e.w.WriteBits(uint64(dod&0x7F), 7)
	} else if dod >= -255 && dod <= 256 {
		e.w.WriteBits(0x6, 3)
		e.w.WriteBits(uint64(dod&0x1FF), 9)
	} else if dod >= -2047 && dod <= 2048 {
		e.w.WriteBits(0xE, 4)
		e.w.WriteBits(uint64(dod&0xFFF), 12)
	} else {
		e.w.WriteBits(0xF, 4)
		e.w.WriteBits(uint64(int64(dod)), 32)
	}

	e.prevDelta = delta
	e.prevTime = timestamp
}

type TimestampDecoder struct {
	r          *BitReader
	prevTime   int64
	prevDelta  int64
	firstPoint bool
}

func NewTimestampDecoder(r *BitReader) *TimestampDecoder {
	return &TimestampDecoder{
		r:          r,
		firstPoint: true,
	}
}

func (d *TimestampDecoder) Decode() (int64, error) {
	if d.firstPoint {
		t, err := ReadVarint(d.r)
		if err != nil {
			return 0, err
		}
		d.prevTime = t
		d.prevDelta = 0
		d.firstPoint = false
		return t, nil
	}

	bit, err := d.r.ReadBit()
	if err != nil {
		return 0, err
	}

	var dod int64
	if !bit {
		dod = 0
	} else {
		bit2, err := d.r.ReadBit()
		if err != nil {
			return 0, err
		}
		if !bit2 {
			v, err := d.r.ReadBits(7)
			if err != nil {
				return 0, err
			}
			dod = signExtend(int64(v), 7)
		} else {
			bit3, err := d.r.ReadBit()
			if err != nil {
				return 0, err
			}
			if !bit3 {
				v, err := d.r.ReadBits(9)
				if err != nil {
					return 0, err
				}
				dod = signExtend(int64(v), 9)
			} else {
				bit4, err := d.r.ReadBit()
				if err != nil {
					return 0, err
				}
				if !bit4 {
					v, err := d.r.ReadBits(12)
					if err != nil {
						return 0, err
					}
					dod = signExtend(int64(v), 12)
				} else {
					v, err := d.r.ReadBits(32)
					if err != nil {
						return 0, err
					}
					dod = signExtend(int64(v), 32)
				}
			}
		}
	}

	delta := d.prevDelta + dod
	timestamp := d.prevTime + delta

	d.prevDelta = delta
	d.prevTime = timestamp

	return timestamp, nil
}

func signExtend(x int64, n int) int64 {
	if n >= 64 {
		return x
	}
	mask := int64(1) << uint(n-1)
	if x&mask != 0 {
		x |= ^((int64(1) << uint(n)) - 1)
	}
	return x
}

type XOREncoder struct {
	w          *BitWriter
	prevValue  uint64
	prevLZ     int
	prevTZ     int
	firstPoint bool
}

func NewXOREncoder(w *BitWriter) *XOREncoder {
	return &XOREncoder{
		w:          w,
		firstPoint: true,
	}
}

func (e *XOREncoder) Encode(value float64) {
	v := math.Float64bits(value)

	if e.firstPoint {
		e.w.WriteBits(v, 64)
		e.prevValue = v
		e.prevLZ = 0
		e.prevTZ = 0
		e.firstPoint = false
		return
	}

	xor := e.prevValue ^ v

	if xor == 0 {
		e.w.WriteBit(false)
	} else {
		e.w.WriteBit(true)

		lz := countLeadingZeros(xor)
		tz := countTrailingZeros(xor)

		if lz >= e.prevLZ && tz >= e.prevTZ {
			e.w.WriteBit(false)
			e.w.WriteBits(xor>>uint(tz), 64-e.prevLZ-e.prevTZ)
		} else {
			e.w.WriteBit(true)
			e.w.WriteBits(uint64(lz), 6)
			e.w.WriteBits(uint64(64-lz-tz), 6)
			e.w.WriteBits(xor>>uint(tz), 64-lz-tz)
			e.prevLZ = lz
			e.prevTZ = tz
		}
	}

	e.prevValue = v
}

type XORDecoder struct {
	r          *BitReader
	prevValue  uint64
	prevLZ     int
	prevTZ     int
	firstPoint bool
}

func NewXORDecoder(r *BitReader) *XORDecoder {
	return &XORDecoder{
		r:          r,
		firstPoint: true,
	}
}

func (d *XORDecoder) Decode() (float64, error) {
	if d.firstPoint {
		v, err := d.r.ReadBits(64)
		if err != nil {
			return 0, err
		}
		d.prevValue = v
		d.prevLZ = 0
		d.prevTZ = 0
		d.firstPoint = false
		return math.Float64frombits(v), nil
	}

	bit, err := d.r.ReadBit()
	if err != nil {
		return 0, err
	}

	if !bit {
		return math.Float64frombits(d.prevValue), nil
	}

	bit2, err := d.r.ReadBit()
	if err != nil {
		return 0, err
	}

	var lz, tz, nbits int
	var xor uint64

	if !bit2 {
		lz = d.prevLZ
		tz = d.prevTZ
		nbits = 64 - lz - tz
		v, err := d.r.ReadBits(nbits)
		if err != nil {
			return 0, err
		}
		xor = v << uint(tz)
	} else {
		lz6, err := d.r.ReadBits(6)
		if err != nil {
			return 0, err
		}
		lz = int(lz6)

		nbits6, err := d.r.ReadBits(6)
		if err != nil {
			return 0, err
		}
		nbits = int(nbits6)
		tz = 64 - lz - nbits

		v, err := d.r.ReadBits(nbits)
		if err != nil {
			return 0, err
		}
		xor = v << uint(tz)

		d.prevLZ = lz
		d.prevTZ = tz
	}

	value := d.prevValue ^ xor
	d.prevValue = value

	return math.Float64frombits(value), nil
}

func countLeadingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for (x & (1 << 63)) == 0 {
		n++
		x <<= 1
	}
	return n
}

func countTrailingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for (x & 1) == 0 {
		n++
		x >>= 1
	}
	return n
}
