package vision

import (
	"image"

	"gocv.io/x/gocv"
)

// IsSquareOccupied compares the live square to a reference square
func IsSquareOccupied(live, ref gocv.Mat) (bool, float64) {
	diff := gocv.NewMat()
	defer diff.Close()

	// 1. Get the absolute difference
	gocv.AbsDiff(live, ref, &diff)

	// 2. Convert to grayscale and threshold to remove minor noise/shadows
	gocv.CvtColor(diff, &diff, gocv.ColorBGRToGray)
	gocv.Threshold(diff, &diff, 40, 255, gocv.ThresholdBinary)

	// 3. Count non-zero pixels (white pixels = change)
	changedPixels := gocv.CountNonZero(diff)
	percentage := (float64(changedPixels) / 10000.0) * 100

	// If > 5% of the square changed, something is there
	return percentage > 5.0, percentage
}

// GetSquare extracts a 100x100 region based on chess coordinates (0-7)
// col: 0=a, 4=e | row: 0=8, 7=1 (OpenCV Y starts from top)
func GetSquare(warped gocv.Mat, col, row int) gocv.Mat {
	// Calculate the rectangle for the square
	x := col * 100
	y := row * 100
	rect := image.Rect(x, y, x+100, y+100)

	// Return a Region of Interest (ROI)
	return warped.Region(rect)
}
