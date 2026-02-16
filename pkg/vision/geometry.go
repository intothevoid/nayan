package vision

import (
	"image"
	"math"
)

// DistanceBetweenPoints calculates the Euclidean distance between two points
func DistanceBetweenPoints(p1, p2 image.Point) float64 {
	dx := float64(p2.X - p1.X)
	dy := float64(p2.Y - p1.Y)
	return math.Sqrt(dx*dx + dy*dy)
}
