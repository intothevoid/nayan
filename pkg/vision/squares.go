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
	// With CLAHE normalization, pieces typically have stddev 24-80+.
	// Set to 20 to reject shadows (e.g. a5 shadow reads ~18-19) while still
	// catching low-contrast pieces via the edge signal.
	absVarianceThreshold = 20.0

	// absEdgeThreshold is the minimum percentage of edge pixels in a square
	// ROI to consider occupied. With CLAHE, occupied squares typically have
	// 7-14% edges. Set to 6.0 to reject wood grain/shadow edge noise
	// (e.g. b6 reads ~5%) while catching pieces via combined threshold.
	absEdgeThreshold = 6.0

	// Combined thresholds: if BOTH signals are moderately above these lower
	// minimums, consider occupied. Catches dark pieces on dark squares
	// (e.g. d8 queen: var~19, edge~4-8%) where neither individual threshold
	// triggers consistently but both signals clearly indicate a piece.
	// Rejects shadows (a5: var=19 but edge=0) and grain (c3: var=9, edge~5).
	absCombinedVarMin  = 16.0
	absCombinedEdgeMin = 3.0
)

// ScanBoardAbsolute detects occupied squares without a reference board using
// two complementary signals:
//  1. Greyscale variance (stddev) — pieces produce texture/shadows
//  2. Canny edge density — pieces have 3D shape with edges regardless of contrast
//
// CLAHE (Contrast Limited Adaptive Histogram Equalization) is applied first
// to normalize uneven lighting and reduce the effect of shadows.
// A square is marked occupied if EITHER signal exceeds its threshold.
func ScanBoardAbsolute(warped gocv.Mat) [8][8]bool {
	var occupancy [8][8]bool

	// Prepare greyscale
	grey := gocv.NewMat()
	defer grey.Close()
	gocv.CvtColor(warped, &grey, gocv.ColorBGRToGray)

	// Apply CLAHE to normalize uneven lighting and reduce shadow effects.
	// clipLimit=2.0, tileGridSize=4x4 (one tile per 2 squares on the 800px board).
	clahe := gocv.NewCLAHEWithParams(2.0, image.Pt(4, 4))
	defer clahe.Close()
	normalized := gocv.NewMat()
	defer normalized.Close()
	clahe.Apply(grey, &normalized)

	// Prepare edge map for edge density analysis (on CLAHE-normalized image)
	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(normalized, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(blurred, &edges, 30, 100)

	mean := gocv.NewMat()
	defer mean.Close()
	stddev := gocv.NewMat()
	defer stddev.Close()

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			// Variance check (on CLAHE-normalized greyscale)
			roiGrey := GetSquare(normalized, col, row)
			gocv.MeanStdDev(roiGrey, &mean, &stddev)
			sd := stddev.GetDoubleAt(0, 0)
			roiGrey.Close()

			// Edge density check
			roiEdge := GetSquare(edges, col, row)
			totalPixels := float64(roiEdge.Rows() * roiEdge.Cols())
			edgePixels := float64(gocv.CountNonZero(roiEdge))
			edgePct := (edgePixels / totalPixels) * 100
			roiEdge.Close()

			if sd > absVarianceThreshold || edgePct > absEdgeThreshold || (sd > absCombinedVarMin && edgePct > absCombinedEdgeMin) {
				occupancy[row][col] = true
			}
		}
	}

	return occupancy
}

// SquareMetrics holds per-square detection metrics for debugging.
type SquareMetrics struct {
	Row      int
	Col      int
	StdDev   float64
	EdgePct  float64
	Occupied bool
}

// ScanBoardDebug returns the same occupancy grid as ScanBoardAbsolute
// plus per-square metrics to help tune detection thresholds.
func ScanBoardDebug(warped gocv.Mat) ([8][8]bool, [64]SquareMetrics) {
	var occupancy [8][8]bool
	var metrics [64]SquareMetrics

	grey := gocv.NewMat()
	defer grey.Close()
	gocv.CvtColor(warped, &grey, gocv.ColorBGRToGray)

	clahe := gocv.NewCLAHEWithParams(2.0, image.Pt(4, 4))
	defer clahe.Close()
	normalized := gocv.NewMat()
	defer normalized.Close()
	clahe.Apply(grey, &normalized)

	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(normalized, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(blurred, &edges, 30, 100)

	mean := gocv.NewMat()
	defer mean.Close()
	stddev := gocv.NewMat()
	defer stddev.Close()

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			roiGrey := GetSquare(normalized, col, row)
			gocv.MeanStdDev(roiGrey, &mean, &stddev)
			sd := stddev.GetDoubleAt(0, 0)
			roiGrey.Close()

			roiEdge := GetSquare(edges, col, row)
			totalPixels := float64(roiEdge.Rows() * roiEdge.Cols())
			edgePixels := float64(gocv.CountNonZero(roiEdge))
			edgePct := (edgePixels / totalPixels) * 100
			roiEdge.Close()

			occupied := sd > absVarianceThreshold || edgePct > absEdgeThreshold || (sd > absCombinedVarMin && edgePct > absCombinedEdgeMin)
			occupancy[row][col] = occupied

			idx := row*8 + col
			metrics[idx] = SquareMetrics{
				Row:      row,
				Col:      col,
				StdDev:   sd,
				EdgePct:  edgePct,
				Occupied: occupied,
			}
		}
	}

	return occupancy, metrics
}

// FormatMetrics returns a human-readable table of per-square detection metrics.
// Squares that are borderline (close to thresholds) are marked with '!' .
func FormatMetrics(metrics [64]SquareMetrics) string {
	s := "  a       b       c       d       e       f       g       h\n"
	for row := 0; row < 8; row++ {
		s += fmt.Sprintf("%d ", 8-row)
		for col := 0; col < 8; col++ {
			m := metrics[row*8+col]
			marker := "."
			if m.Occupied {
				marker = "X"
			}
			// Mark borderline squares (within 20% of any threshold)
			if !m.Occupied && (m.StdDev > absVarianceThreshold*0.8 || m.EdgePct > absEdgeThreshold*0.8 ||
				(m.StdDev > absCombinedVarMin*0.8 && m.EdgePct > absCombinedEdgeMin*0.8)) {
				marker = "!"
			}
			s += fmt.Sprintf("%s%2.0f/%1.0f ", marker, m.StdDev, m.EdgePct)
		}
		s += "\n"
	}
	s += fmt.Sprintf("Thresholds: var>%.0f edge>%.1f%%  Legend: X=occupied .=empty !=borderline\n",
		absVarianceThreshold, absEdgeThreshold)
	return s
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
