package util

import (
	"encoding/binary"
	"fmt"
)

const (
	tagLiteral    = 0x00
	tagCopy1      = 0x01
	tagCopy2      = 0x02
	tagCopy4      = 0x03
	maxBacklogSize = 65536
)

func SnappyDecompress(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("snappy: input too short")
	}

	uncompressedLen, n := binary.Uvarint(input)
	if n <= 0 || n > 4 {
		return nil, fmt.Errorf("snappy: invalid length")
	}
	input = input[n:]

	if uncompressedLen > 1<<32 {
		return nil, fmt.Errorf("snappy: uncompressed length too large: %d", uncompressedLen)
	}

	output := make([]byte, 0, uncompressedLen)

	for len(input) > 0 {
		tag := input[0]
		input = input[1:]

		switch tag & 0x03 {
		case tagLiteral:
			length := int(tag>>2) + 1
			if length <= 60 {
			} else if length == 61 {
				if len(input) < 1 {
					return nil, fmt.Errorf("snappy: unexpected EOF")
				}
				length = int(input[0]) + 1
				input = input[1:]
			} else if length == 62 {
				if len(input) < 2 {
					return nil, fmt.Errorf("snappy: unexpected EOF")
				}
				length = int(binary.LittleEndian.Uint16(input[:2])) + 1
				input = input[2:]
			} else if length == 63 {
				if len(input) < 4 {
					return nil, fmt.Errorf("snappy: unexpected EOF")
				}
				length = int(binary.LittleEndian.Uint32(input[:4])) + 1
				input = input[4:]
			}

			if len(input) < length {
				return nil, fmt.Errorf("snappy: literal length exceeds input")
			}

			output = append(output, input[:length]...)
			input = input[length:]

		case tagCopy1:
			offset := int(uint32(tag>>2)&0x07)<<8 | uint32(input[0])
			length := int((tag>>5)&0x07) + 4
			input = input[1:]

			if offset <= 0 || offset > len(output) {
				return nil, fmt.Errorf("snappy: invalid copy offset")
			}

			for i := 0; i < length; i++ {
				output = append(output, output[len(output)-offset])
			}

		case tagCopy2:
			if len(input) < 2 {
				return nil, fmt.Errorf("snappy: unexpected EOF")
			}
			offset := int(binary.LittleEndian.Uint16(input[:2]))
			length := int(tag>>2) + 1
			input = input[2:]

			if offset <= 0 || offset > len(output) {
				return nil, fmt.Errorf("snappy: invalid copy offset")
			}

			for i := 0; i < length; i++ {
				output = append(output, output[len(output)-offset])
			}

		case tagCopy4:
			if len(input) < 4 {
				return nil, fmt.Errorf("snappy: unexpected EOF")
			}
			offset := int(binary.LittleEndian.Uint32(input[:4]))
			length := int(tag>>2) + 1
			input = input[4:]

			if offset <= 0 || offset > len(output) {
				return nil, fmt.Errorf("snappy: invalid copy offset")
			}

			for i := 0; i < length; i++ {
				output = append(output, output[len(output)-offset])
			}
		}
	}

	if uint64(len(output)) != uncompressedLen {
		return nil, fmt.Errorf("snappy: decompressed length mismatch: got %d, expected %d", len(output), uncompressedLen)
	}

	return output, nil
}

func SnappyCompress(input []byte) []byte {
	var output []byte

	lenBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(lenBuf, uint64(len(input)))
	output = append(output, lenBuf[:n]...)

	i := 0
	for i < len(input) {
		if i+16 <= len(input) {
			literalLen := 16
			output = append(output, byte(((literalLen-1)<<2)|tagLiteral))
			output = append(output, input[i:i+literalLen]...)
			i += literalLen
		} else {
			literalLen := len(input) - i
			if literalLen <= 60 {
				output = append(output, byte(((literalLen-1)<<2)|tagLiteral))
			} else if literalLen < 256 {
				output = append(output, byte((60<<2)|tagLiteral))
				output = append(output, byte(literalLen-1))
			}
			output = append(output, input[i:]...)
			i = len(input)
		}
	}

	return output
}
