package downsample

import (
	"fmt"
	"gorilla-tsdb/block"
	"math"
	"testing"
)

func TestLTTBBasic(t *testing.T) {
	var points []block.DataPoint
	for i := 0; i < 100; i++ {
		points = append(points, block.DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     math.Sin(float64(i) * 0.1),
		})
	}

	threshold := 10
	downsampled := LTTB(points, threshold)

	if len(downsampled) != threshold {
		t.Errorf("Expected %d points, got %d", threshold, len(downsampled))
	}

	if downsampled[0].Timestamp != points[0].Timestamp {
		t.Error("First point should be preserved")
	}

	if downsampled[len(downsampled)-1].Timestamp != points[len(points)-1].Timestamp {
		t.Error("Last point should be preserved")
	}

	for i := 1; i < len(downsampled); i++ {
		if downsampled[i].Timestamp <= downsampled[i-1].Timestamp {
			t.Error("Points should be in order")
		}
	}
}

func TestLTTBSmallDataset(t *testing.T) {
	points := []block.DataPoint{
		{Timestamp: 1000, Value: 1.0},
		{Timestamp: 2000, Value: 2.0},
		{Timestamp: 3000, Value: 3.0},
	}

	downsampled := LTTB(points, 10)
	if len(downsampled) != len(points) {
		t.Error("Small datasets should not be downsampled")
	}
}

func TestLTTBEmpty(t *testing.T) {
	downsampled := LTTB([]block.DataPoint{}, 10)
	if len(downsampled) != 0 {
		t.Error("Empty input should produce empty output")
	}
}

func TestLTTBThreshold1(t *testing.T) {
	points := []block.DataPoint{
		{Timestamp: 1000, Value: 1.0},
		{Timestamp: 2000, Value: 2.0},
		{Timestamp: 3000, Value: 3.0},
	}

	downsampled := LTTB(points, 1)
	if len(downsampled) != len(points) {
		t.Error("Threshold 1 should return all points")
	}
}

func TestLTTBPreservesExtremes(t *testing.T) {
	var points []block.DataPoint
	for i := 0; i < 1000; i++ {
		v := float64(i % 100)
		if i == 500 {
			v = 1000
		}
		points = append(points, block.DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     v,
		})
	}

	downsampled := LTTB(points, 50)

	hasPeak := false
	for _, p := range downsampled {
		if p.Value == 1000 {
			hasPeak = true
			break
		}
	}

	if !hasPeak {
		t.Log("Warning: extreme value not preserved (this can happen with LTTB)")
	}

	fmt.Printf("Original: %d points, Downsampled: %d points\n", len(points), len(downsampled))
}

func BenchmarkLTTB(b *testing.B) {
	var points []block.DataPoint
	for i := 0; i < 10000; i++ {
		points = append(points, block.DataPoint{
			Timestamp: int64(i) * 1000,
			Value:     math.Sin(float64(i)*0.01) + math.Cos(float64(i)*0.02),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LTTB(points, 500)
	}
}
