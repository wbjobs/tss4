package block

import (
	"fmt"
	"math"
	"os"
	"testing"
)

func TestBlockChecksum(t *testing.T) {
	b := NewBlock()

	for i := 0; i < 10; i++ {
		added := b.Add(DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     float64(i),
		})
		if !added {
			t.Fatal("Failed to add point")
		}
	}

	data, err := b.Compress()
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(data) < 26 {
		t.Fatalf("Compressed data too short: %d bytes", len(data))
	}

	b2, err := Decompress(data)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if len(b2.Points) != len(b.Points) {
		t.Errorf("Point count mismatch: %d vs %d", len(b2.Points), len(b.Points))
	}

	for i, p := range b2.Points {
		if p.Timestamp != b.Points[i].Timestamp || p.Value != b.Points[i].Value {
			t.Errorf("Point %d mismatch", i)
		}
	}

	fmt.Printf("Original points: %d, Compressed size: %d bytes\n", len(b.Points), len(data))
}

func TestBlockChecksumCorruption(t *testing.T) {
	b := NewBlock()

	for i := 0; i < 10; i++ {
		b.Add(DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     float64(i),
		})
	}

	data, err := b.Compress()
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	corrupted := make([]byte, len(data))
	copy(corrupted, data)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = Decompress(corrupted)
	if err == nil {
		t.Error("Expected checksum error for corrupted data")
	} else {
		fmt.Printf("Correctly detected corruption: %v\n", err)
	}
}

func TestBlockChecksumNaN(t *testing.T) {
	b := NewBlock()

	b.Add(DataPoint{Timestamp: 1000, Value: 1.0})
	b.Add(DataPoint{Timestamp: 2000, Value: math.NaN()})
	b.Add(DataPoint{Timestamp: 3000, Value: 3.0})

	data, err := b.Compress()
	if err != nil {
		t.Fatalf("Compress with NaN failed: %v", err)
	}

	b2, err := Decompress(data)
	if err != nil {
		t.Fatalf("Decompress with NaN failed: %v", err)
	}

	if len(b2.Points) != 3 {
		t.Errorf("Expected 3 points, got %d", len(b2.Points))
	}

	if !math.IsNaN(b2.Points[1].Value) {
		t.Error("Expected NaN at index 1")
	}
}

func TestBlockSortAndInsert(t *testing.T) {
	b := NewBlock()

	b.Add(DataPoint{Timestamp: 3000, Value: 3.0})
	b.Add(DataPoint{Timestamp: 1000, Value: 1.0})
	b.Add(DataPoint{Timestamp: 2000, Value: 2.0})
	b.Add(DataPoint{Timestamp: 1000, Value: 10.0})

	if len(b.Points) != 3 {
		t.Errorf("Expected 3 points (one duplicate updated), got %d", len(b.Points))
	}

	if b.Points[0].Value != 10.0 {
		t.Errorf("Expected updated value 10.0, got %f", b.Points[0].Value)
	}

	for i := 1; i < len(b.Points); i++ {
		if b.Points[i].Timestamp <= b.Points[i-1].Timestamp {
			t.Errorf("Points not sorted at index %d", i)
		}
	}
}

func TestBlockWriteAndReadFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "block-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	b := NewBlock()
	for i := 0; i < 64; i++ {
		b.Add(DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     math.Sin(float64(i) * 0.1),
		})
	}

	compressed, err := b.Compress()
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	filePath := tmpDir + "/test.block"
	if err := os.WriteFile(filePath, compressed, 0644); err != nil {
		t.Fatalf("Write file failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Read file failed: %v", err)
	}

	b2, err := Decompress(data)
	if err != nil {
		t.Fatalf("Decompress from file failed: %v", err)
	}

	if len(b2.Points) != len(b.Points) {
		t.Errorf("Point count mismatch: %d vs %d", len(b2.Points), len(b.Points))
	}

	for i, p := range b2.Points {
		if p.Timestamp != b.Points[i].Timestamp {
			t.Errorf("Timestamp mismatch at %d", i)
		}
		if math.Abs(p.Value - b.Points[i].Value) > 0.0001 {
			t.Errorf("Value mismatch at %d", i)
		}
	}

	fmt.Printf("Block written and read successfully, %d points, %d bytes\n",
		len(b.Points), len(data))
}
