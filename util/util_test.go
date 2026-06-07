package util

import (
	"bytes"
	"fmt"
	"testing"
)

func TestXXHash64Sum(t *testing.T) {
	data := []byte("Hello, World!")
	hash1 := XXHash64Sum(data)
	hash2 := XXHash64Sum(data)

	if hash1 != hash2 {
		t.Error("Same input should produce same hash")
	}

	data2 := []byte("Hello, World?")
	hash3 := XXHash64Sum(data2)

	if hash1 == hash3 {
		t.Error("Different inputs should produce different hashes")
	}

	fmt.Printf("Hash of 'Hello, World!': 0x%x\n", hash1)
	fmt.Printf("Hash of 'Hello, World?': 0x%x\n", hash3)
}

func TestXXHash64SumEmpty(t *testing.T) {
	hash := XXHash64Sum([]byte{})
	fmt.Printf("Hash of empty: 0x%x\n", hash)
}

func TestXXHash64LargeData(t *testing.T) {
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	hash := XXHash64Sum(data)
	fmt.Printf("Hash of 10KB data: 0x%x\n", hash)
}

func TestSnappyRoundtrip(t *testing.T) {
	original := []byte("The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog.")
	compressed := SnappyCompress(original)
	decompressed, err := SnappyDecompress(compressed)

	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !bytes.Equal(original, decompressed) {
		t.Error("Roundtrip failed: data mismatch")
	}

	fmt.Printf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f\n",
		len(original), len(compressed), float64(len(compressed))/float64(len(original)))
}

func TestSnappyEmpty(t *testing.T) {
	original := []byte{}
	compressed := SnappyCompress(original)
	decompressed, err := SnappyDecompress(compressed)

	if err != nil {
		t.Fatalf("Decompress empty failed: %v", err)
	}

	if len(decompressed) != 0 {
		t.Error("Empty roundtrip failed")
	}
}

func TestSnappyCompressibleData(t *testing.T) {
	var original []byte
	for i := 0; i < 1000; i++ {
		original = append(original, []byte("AAAAAABBBBBBCCCCCCDDDDDD")...)
	}

	compressed := SnappyCompress(original)
	decompressed, err := SnappyDecompress(compressed)

	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !bytes.Equal(original, decompressed) {
		t.Error("Compressible data roundtrip failed")
	}

	ratio := float64(len(compressed)) / float64(len(original))
	fmt.Printf("Compressible data: Original: %d, Compressed: %d, Ratio: %.4f\n",
		len(original), len(compressed), ratio)

	if ratio > 0.5 {
		t.Logf("Compression ratio is high: %.4f", ratio)
	}
}

func TestSnappyRandomData(t *testing.T) {
	original := make([]byte, 10000)
	for i := range original {
		original[i] = byte(i * 17 % 256)
	}

	compressed := SnappyCompress(original)
	decompressed, err := SnappyDecompress(compressed)

	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !bytes.Equal(original, decompressed) {
		t.Error("Random data roundtrip failed")
	}

	ratio := float64(len(compressed)) / float64(len(original))
	fmt.Printf("Random data: Original: %d, Compressed: %d, Ratio: %.4f\n",
		len(original), len(compressed), ratio)
}

func BenchmarkXXHash64Sum(b *testing.B) {
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		XXHash64Sum(data)
	}
}

func BenchmarkSnappyCompress(b *testing.B) {
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SnappyCompress(data)
	}
}

func BenchmarkSnappyDecompress(b *testing.B) {
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	compressed := SnappyCompress(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SnappyDecompress(compressed)
	}
}
