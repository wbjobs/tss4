package compression

import (
	"bytes"
	"encoding/binary"
	"io"
)

type BitWriter struct {
	buf    *bytes.Buffer
	byte   byte
	offset int
}

func NewBitWriter() *BitWriter {
	return &BitWriter{
		buf:    bytes.NewBuffer(nil),
		byte:   0,
		offset: 0,
	}
}

func (w *BitWriter) WriteBit(bit bool) {
	if bit {
		w.byte |= 1 << (7 - w.offset)
	}
	w.offset++
	if w.offset == 8 {
		w.buf.WriteByte(w.byte)
		w.byte = 0
		w.offset = 0
	}
}

func (w *BitWriter) WriteBits(value uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		bit := (value>>uint(i))&1 == 1
		w.WriteBit(bit)
	}
}

func (w *BitWriter) WriteByte(b byte) {
	w.WriteBits(uint64(b), 8)
}

func (w *BitWriter) Bytes() []byte {
	if w.offset > 0 {
		w.buf.WriteByte(w.byte)
	}
	return w.buf.Bytes()
}

type BitReader struct {
	data   []byte
	byte   byte
	offset int
	pos    int
}

func NewBitReader(data []byte) *BitReader {
	r := &BitReader{
		data:   data,
		offset: 8,
		pos:    0,
	}
	return r
}

func (r *BitReader) ReadBit() (bool, error) {
	if r.offset == 8 {
		if r.pos >= len(r.data) {
			return false, io.EOF
		}
		r.byte = r.data[r.pos]
		r.pos++
		r.offset = 0
	}
	bit := (r.byte >> (7 - r.offset)) & 1
	r.offset++
	return bit == 1, nil
}

func (r *BitReader) ReadBits(n int) (uint64, error) {
	var value uint64
	for i := 0; i < n; i++ {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		value <<= 1
		if bit {
			value |= 1
		}
	}
	return value, nil
}

func (r *BitReader) ReadByte() (byte, error) {
	v, err := r.ReadBits(8)
	return byte(v), err
}

func (r *BitReader) ReadUint64() (uint64, error) {
	var result uint64
	for i := 0; i < 8; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result = (result << 8) | uint64(b)
	}
	return result, nil
}

func (r *BitReader) ReadInt64() (int64, error) {
	v, err := r.ReadUint64()
	return int64(v), err
}

func WriteVarint(w *BitWriter, x int64) {
	if x >= 0 {
		WriteUvarint(w, uint64(x)<<1)
	} else {
		WriteUvarint(w, uint64(^x<<1)|1)
	}
}

func WriteUvarint(w *BitWriter, x uint64) {
	for x >= 0x80 {
		w.WriteBits(uint64(x&0x7f)|0x80, 8)
		x >>= 7
	}
	w.WriteBits(uint64(x), 8)
}

func ReadVarint(r *BitReader) (int64, error) {
	ux, err := ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return x, nil
}

func ReadUvarint(r *BitReader) (uint64, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, binary.ErrOverflow
			}
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}
