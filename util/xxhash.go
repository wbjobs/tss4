package util

const (
	xxhPrime64_1 uint64 = 11400714785074694791
	xxhPrime64_2 uint64 = 14029467366897019727
	xxhPrime64_3 uint64 = 1609587929392839161
	xxhPrime64_4 uint64 = 9650029242287828579
	xxhPrime64_5 uint64 = 2870177450012600261
)

func XXHash64(data []byte, seed uint64) uint64 {
	n := len(data)
	var h64 uint64

	if n >= 32 {
		v1 := seed + xxhPrime64_1 + xxhPrime64_2
		v2 := seed + xxhPrime64_2
		v3 := seed + 0
		v4 := seed - xxhPrime64_1

		p := 0
		for n >= 32 {
			v1 = xxh64Round(v1, u64(data[p:p+8]))
			v2 = xxh64Round(v2, u64(data[p+8:p+16]))
			v3 = xxh64Round(v3, u64(data[p+16:p+24]))
			v4 = xxh64Round(v4, u64(data[p+24:p+32]))
			p += 32
			n -= 32
		}

		h64 = rotl64(v1, 1) + rotl64(v2, 7) + rotl64(v3, 12) + rotl64(v4, 18)
		h64 = xxh64MergeRound(h64, v1)
		h64 = xxh64MergeRound(h64, v2)
		h64 = xxh64MergeRound(h64, v3)
		h64 = xxh64MergeRound(h64, v4)
	} else {
		h64 = seed + xxhPrime64_5
	}

	h64 += uint64(len(data))

	p := len(data) - n
	for n >= 8 {
		k1 := xxh64Round(0, u64(data[p:p+8]))
		h64 ^= k1
		h64 = rotl64(h64, 27)*xxhPrime64_1 + xxhPrime64_4
		p += 8
		n -= 8
	}

	if n >= 4 {
		h64 ^= uint64(u32(data[p:p+4])) * xxhPrime64_1
		h64 = rotl64(h64, 23)*xxhPrime64_2 + xxhPrime64_3
		p += 4
		n -= 4
	}

	for n > 0 {
		h64 ^= uint64(data[p]) * xxhPrime64_5
		h64 = rotl64(h64, 11) * xxhPrime64_1
		p++
		n--
	}

	h64 ^= h64 >> 33
	h64 *= xxhPrime64_2
	h64 ^= h64 >> 29
	h64 *= xxhPrime64_3
	h64 ^= h64 >> 32

	return h64
}

func xxh64Round(acc, input uint64) uint64 {
	acc += input * xxhPrime64_2
	acc = rotl64(acc, 31)
	acc *= xxhPrime64_1
	return acc
}

func xxh64MergeRound(acc, val uint64) uint64 {
	val = xxh64Round(0, val)
	acc ^= val
	acc = acc*xxhPrime64_1 + xxhPrime64_4
	return acc
}

func rotl64(x uint64, r int) uint64 {
	return (x << uint(r)) | (x >> uint(64-r))
}

func u64(b []byte) uint64 {
	return uint64(b[0]) |
		uint64(b[1])<<8 |
		uint64(b[2])<<16 |
		uint64(b[3])<<24 |
		uint64(b[4])<<32 |
		uint64(b[5])<<40 |
		uint64(b[6])<<48 |
		uint64(b[7])<<56
}

func u32(b []byte) uint32 {
	return uint32(b[0]) |
		uint32(b[1])<<8 |
		uint32(b[2])<<16 |
		uint32(b[3])<<24
}

func XXHash64Sum(data []byte) uint64 {
	return XXHash64(data, 0)
}
