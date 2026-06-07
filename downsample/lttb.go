package downsample

import (
	"gorilla-tsdb/block"
	"math"
)

func LTTB(points []block.DataPoint, threshold int) []block.DataPoint {
	if threshold <= 0 {
		return points
	}

	if len(points) <= threshold {
		result := make([]block.DataPoint, len(points))
		copy(result, points)
		return result
	}

	if threshold == 1 {
		mid := len(points) / 2
		return []block.DataPoint{points[mid]}
	}

	if threshold == 2 {
		return []block.DataPoint{points[0], points[len(points)-1]}
	}

	sampled := make([]block.DataPoint, 0, threshold)
	sampled = append(sampled, points[0])

	bucketSize := float64(len(points)-2) / float64(threshold-2)

	a := 0

	for i := 0; i < threshold-2; i++ {
		avgRangeStart := int(math.Floor(float64(i+1)*bucketSize)) + 1
		avgRangeEnd := int(math.Floor(float64(i+2)*bucketSize)) + 1

		if avgRangeEnd > len(points)-1 {
			avgRangeEnd = len(points) - 1
		}

		avgX := 0.0
		avgY := 0.0
		avgCount := avgRangeEnd - avgRangeStart

		for j := avgRangeStart; j < avgRangeEnd; j++ {
			avgX += float64(points[j].Timestamp)
			avgY += points[j].Value
		}

		if avgCount > 0 {
			avgX /= float64(avgCount)
			avgY /= float64(avgCount)
		}

		rangeStart := int(math.Floor(float64(i)*bucketSize)) + 1
		rangeEnd := int(math.Floor(float64(i+1)*bucketSize)) + 1

		if rangeEnd > len(points)-1 {
			rangeEnd = len(points) - 1
		}

		maxArea := -1.0
		nextA := rangeStart

		ax := float64(points[a].Timestamp)
		ay := points[a].Value

		for j := rangeStart; j < rangeEnd; j++ {
			area := triangleArea(
				ax, ay,
				avgX, avgY,
				float64(points[j].Timestamp), points[j].Value,
			)

			if area > maxArea {
				maxArea = area
				nextA = j
			}
		}

		sampled = append(sampled, points[nextA])
		a = nextA
	}

	sampled = append(sampled, points[len(points)-1])

	return sampled
}

func triangleArea(x1, y1, x2, y2, x3, y3 float64) float64 {
	return math.Abs((x1*(y2-y3) + x2*(y3-y1) + x3*(y1-y2)) * 0.5)
}
