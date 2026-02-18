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

// minBlobArea is the minimum number of pixels a connected region must have
// to be considered a piece. Filters out noise and small shadows.
const minBlobArea = 200

// ScanBoard computes a whole-board diff, finds connected blobs, and assigns
// each blob to the square containing its centroid. This prevents a single
// tall piece from triggering multiple squares — no matter how much the piece
// image bleeds across square boundaries, its centroid stays on the square
// where the base sits.
func ScanBoard(live, reference gocv.Mat) [8][8]bool {
	var occupancy [8][8]bool

	// 1. Full-board absolute difference
	diff := gocv.NewMat()
	defer diff.Close()
	gocv.AbsDiff(live, reference, &diff)

	// 2. Convert to greyscale and threshold
	grey := gocv.NewMat()
	defer grey.Close()
	gocv.CvtColor(diff, &grey, gocv.ColorBGRToGray)
	gocv.Threshold(grey, &grey, 40, 255, gocv.ThresholdBinary)

	// 3. Morphological open to remove small noise, then close to merge nearby regions
	kernel := gocv.GetStructuringElement(gocv.MorphEllipse, image.Pt(5, 5))
	defer kernel.Close()
	gocv.MorphologyEx(grey, &grey, gocv.MorphOpen, kernel)
	gocv.MorphologyEx(grey, &grey, gocv.MorphClose, kernel)

	// 4. Find connected components (contours)
	contours := gocv.FindContours(grey, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	for i := 0; i < contours.Size(); i++ {
		cnt := contours.At(i)
		area := gocv.ContourArea(cnt)
		if area < minBlobArea {
			continue
		}

		// Compute centroid from bounding rectangle center
		br := gocv.BoundingRect(cnt)
		cx := br.Min.X + br.Dx()/2
		cy := br.Min.Y + br.Dy()/2

		// Map centroid pixel to board square
		col := cx / 100
		row := cy / 100
		if col >= 0 && col < 8 && row >= 0 && row < 8 {
			occupancy[row][col] = true
		}
	}

	return occupancy
}

// Detection thresholds for ScanBoardAbsolute.
const (
	// absVarianceThreshold is the minimum greyscale stddev to consider occupied.
	// Catches dark pieces on light squares and most high-contrast cases.
	absVarianceThreshold = 22.0

	// absEdgeThreshold is the minimum percentage of edge pixels in a square
	// ROI to consider occupied. Catches light pieces on light squares where
	// variance is low but the piece's 3D shape still produces edges.
	absEdgeThreshold = 4.0
)

// ScanBoardAbsolute detects occupied squares without a reference board using
// two complementary signals:
//  1. Greyscale variance (stddev) — pieces produce texture/shadows
//  2. Canny edge density — pieces have 3D shape with edges regardless of contrast
//
// A square is marked occupied if EITHER signal exceeds its threshold.
func ScanBoardAbsolute(warped gocv.Mat) [8][8]bool {
	var occupancy [8][8]bool

	// Prepare greyscale for variance analysis
	grey := gocv.NewMat()
	defer grey.Close()
	gocv.CvtColor(warped, &grey, gocv.ColorBGRToGray)

	// Prepare edge map for edge density analysis
	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(grey, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(blurred, &edges, 30, 100)

	mean := gocv.NewMat()
	defer mean.Close()
	stddev := gocv.NewMat()
	defer stddev.Close()

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			// Variance check
			roiGrey := GetSquare(grey, col, row)
			gocv.MeanStdDev(roiGrey, &mean, &stddev)
			sd := stddev.GetDoubleAt(0, 0)
			roiGrey.Close()

			// Edge density check
			roiEdge := GetSquare(edges, col, row)
			totalPixels := float64(roiEdge.Rows() * roiEdge.Cols())
			edgePixels := float64(gocv.CountNonZero(roiEdge))
			edgePct := (edgePixels / totalPixels) * 100
			roiEdge.Close()

			if sd > absVarianceThreshold || edgePct > absEdgeThreshold {
				occupancy[row][col] = true
			}
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
