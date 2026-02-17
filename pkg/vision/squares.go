package vision

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

// IsSquareOccupied compares the live square to a reference square.
// Both Mats should be the same size (the center crop from GetSquare).
func IsSquareOccupied(live, ref gocv.Mat) (bool, float64) {
	diff := gocv.NewMat()
	defer diff.Close()

	// 1. Get the absolute difference
	gocv.AbsDiff(live, ref, &diff)

	// 2. Convert to grayscale and threshold to remove minor noise/shadows
	gocv.CvtColor(diff, &diff, gocv.ColorBGRToGray)
	gocv.Threshold(diff, &diff, 40, 255, gocv.ThresholdBinary)

	// 3. Count non-zero pixels (white pixels = change)
	totalPixels := float64(diff.Rows() * diff.Cols())
	changedPixels := gocv.CountNonZero(diff)
	percentage := (float64(changedPixels) / totalPixels) * 100

	// 8% threshold — higher than before because the center crop concentrates
	// the signal (piece base) while excluding edge spillover from neighbours
	return percentage > 8.0, percentage
}

// squareInset is the number of pixels to crop from each edge of a 100x100
// square. This extracts only the center region where the piece base sits,
// avoiding bleed from neighbouring squares caused by piece height + perspective.
const squareInset = 20

// GetSquare extracts the center region of a square based on chess coordinates (0-7).
// col: 0=a, 4=e | row: 0=8, 7=1 (OpenCV Y starts from top).
// Returns a 60x60 ROI (100 - 2*20 inset) to reduce neighbour bleed.
func GetSquare(warped gocv.Mat, col, row int) gocv.Mat {
	x := col*100 + squareInset
	y := row*100 + squareInset
	size := 100 - 2*squareInset
	rect := image.Rect(x, y, x+size, y+size)

	return warped.Region(rect)
}

// ScanBoard compares every square of the live warped board against the
// reference (empty board) and returns an 8x8 occupancy grid.
// true = occupied, false = empty.
func ScanBoard(live, reference gocv.Mat) [8][8]bool {
	var occupancy [8][8]bool
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			liveSq := GetSquare(live, col, row)
			refSq := GetSquare(reference, col, row)
			occupied, _ := IsSquareOccupied(liveSq, refSq)
			occupancy[row][col] = occupied
		}
	}
	return occupancy
}

// DrawOccupancy draws a semi-transparent green rectangle on each occupied square.
func DrawOccupancy(img *gocv.Mat, occupancy [8][8]bool) {
	green := color.RGBA{0, 200, 0, 0}
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if occupancy[row][col] {
				x := col * 100
				y := row * 100
				pt1 := image.Pt(x+5, y+5)
				pt2 := image.Pt(x+95, y+95)
				gocv.Rectangle(img, image.Rectangle{Min: pt1, Max: pt2}, green, 3)
			}
		}
	}
}

// FormatOccupancy returns a text representation of the occupancy grid.
// 'X' = occupied, '.' = empty. Columns are a-h, rows are 8-1.
func FormatOccupancy(occupancy [8][8]bool) string {
	var s string
	s += "  a b c d e f g h\n"
	for row := 0; row < 8; row++ {
		s += fmt.Sprintf("%d ", 8-row)
		for col := 0; col < 8; col++ {
			if occupancy[row][col] {
				s += "X "
			} else {
				s += ". "
			}
		}
		s += "\n"
	}
	return s
}

// PrintOccupancy prints the occupancy grid to stdout.
func PrintOccupancy(occupancy [8][8]bool) {
	fmt.Print(FormatOccupancy(occupancy))
	fmt.Println()
}
