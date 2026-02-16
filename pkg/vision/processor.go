package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gocv.io/x/gocv"
)

// toGrey converts input to single-channel grayscale
func toGrey(input gocv.Mat) gocv.Mat {
	grey := gocv.NewMat()
	if input.Channels() == 3 {
		gocv.CvtColor(input, &grey, gocv.ColorBGRToGray)
	} else {
		input.CopyTo(&grey)
	}
	return grey
}

// Preprocess converts the raw frame into an edge map optimised for board outline detection.
// Uses Canny edge detection with a larger blur (7x7) to suppress internal board textures,
// then morphological closing (dilate→erode) to seal gaps in the board outline while
// keeping edges relatively thin.
func Preprocess(input gocv.Mat) gocv.Mat {
	if input.Empty() {
		fmt.Println("Vision Error: Input Mat is empty")
		return gocv.NewMat()
	}

	grey := toGrey(input)
	if grey.Empty() {
		fmt.Println("Vision Error: Grey Mat is empty")
		return grey
	}

	// Larger 7x7 blur suppresses internal square edges while preserving the strong board outline
	blurred := gocv.NewMat()
	gocv.GaussianBlur(grey, &blurred, image.Pt(7, 7), 0, 0, gocv.BorderDefault)
	grey.Close()

	// Canny edge detection — these thresholds work well for a wooden board against a darker surface
	edges := gocv.NewMat()
	gocv.Canny(blurred, &edges, 50, 150)
	blurred.Close()

	// Morphological close (dilate then erode) seals small gaps in the board outline
	// without thickening edges as much as dilation alone
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 5))
	defer kernel.Close()

	closed := gocv.NewMat()
	gocv.MorphologyEx(edges, &closed, gocv.MorphClose, kernel)
	edges.Close()

	return closed
}

// DetectBoard detects the largest contour in the board and ensures it is a square (4 points)
// DetectBoard detects the largest 4-cornered shape in the image
func DetectBoard(edges gocv.Mat) []image.Point {
	// 1. Find contours
	contours := gocv.FindContours(edges, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	var bestQuad []image.Point
	maxArea := 0.0

	// 2. Filter Area: Don't look at shapes smaller than 10% of the screen
	minArea := float64(edges.Rows()*edges.Cols()) * 0.10

	for i := 0; i < contours.Size(); i++ {
		cnt := contours.At(i)
		area := gocv.ContourArea(cnt)

		// Optimization: Skip small contours early
		if area < minArea {
			continue
		}

		// 3. Approximation: Using 2% (0.02) helps capture the outer frame better
		// than 4%, which might round off the corners too much.
		peri := gocv.ArcLength(cnt, true)
		approx := gocv.ApproxPolyDP(cnt, 0.02*peri, true)

		// 4. Geometry Check: Must have exactly 4 corners
		if !approx.IsNil() && approx.Size() == 4 {
			points := approx.ToPoints()

			// 5. Squareness Check: Compare Diagonals
			// Calculate diagonal distances (TopLeft-BottomRight vs TopRight-BottomLeft)
			d1 := DistanceBetweenPoints(points[0], points[2])
			d2 := DistanceBetweenPoints(points[1], points[3])

			// If diagonals are within 25% of each other, it's a valid square/rectangle
			if math.Abs(d1-d2)/d1 < 0.25 {
				// 6. "King of the Hill" Logic
				// We keep the LARGEST valid square we find.
				// This ensures we prefer the outer wood frame over the inner notation border.
				if area > maxArea {
					maxArea = area
					bestQuad = points
				}
			}
		}
		approx.Close()
	}
	return bestQuad
}

// ReorderPoints reorders the points in order tl, tr, br, bl
func ReorderPoints(pts []image.Point) []image.Point {
	if len(pts) != 4 {
		return pts
	}

	newPts := make([]image.Point, 4)

	// TL: min sum, BR: max sum
	// TR: min diff, BL: max diff
	minSum, maxSum := pts[0].X+pts[0].Y, pts[0].X+pts[0].Y
	minDiff, maxDiff := pts[0].Y-pts[0].X, pts[0].Y-pts[0].X

	// Start with the first point as the default for all
	tl, tr, br, bl := pts[0], pts[0], pts[0], pts[0]

	for _, p := range pts {
		sum := p.X + p.Y
		diff := p.Y - p.X

		if sum < minSum {
			minSum = sum
			tl = p
		}
		if sum > maxSum {
			maxSum = sum
			br = p
		}
		if diff < minDiff {
			minDiff = diff
			tr = p
		}
		if diff > maxDiff {
			maxDiff = diff
			bl = p
		}
	}

	newPts[0] = tl // top left
	newPts[1] = tr // top right
	newPts[2] = br // bottom right
	newPts[3] = bl // bottom left

	return newPts
}

// WarpBoard creates a warp corrected gocv.Mat from a input gocv.Mat to remove
// perspective distortion
func WarpBoard(input gocv.Mat, corners []image.Point) gocv.Mat {
	sortedCorners := ReorderPoints(corners)

	// Convert corners to float32 for OpenCV math
	src := gocv.NewPointVectorFromPoints(sortedCorners)
	defer src.Close()

	// Define target square
	dest := gocv.NewPointVectorFromPoints([]image.Point{
		{0, 0}, {800, 0}, {800, 800}, {0, 800},
	})
	defer dest.Close()

	// Calculate the transformation matrix
	m := gocv.GetPerspectiveTransform(src, dest)
	defer m.Close()

	// Apply warp
	warped := gocv.NewMat()
	gocv.WarpPerspective(input, &warped, m, image.Pt(800, 800))

	return warped
}

// DetectInnerBoard finds the inner playing area within a warped board image.
// It looks for the strong edges between the wooden border and the squares using
// line detection. Falls back to a configurable inset ratio if lines aren't found.
func DetectInnerBoard(warped gocv.Mat, fallbackInsetRatio float64) image.Rectangle {
	size := warped.Rows() // should be 800

	// Convert to grayscale and detect edges
	grey := toGrey(warped)
	defer grey.Close()

	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(grey, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(blurred, &edges, 50, 150)

	// Use HoughLinesP to find line segments
	lines := gocv.NewMat()
	defer lines.Close()
	// params: rho=1px, theta=pi/180, threshold=100, minLineLength=200, maxLineGap=20
	gocv.HoughLinesPWithParams(edges, &lines, 1, math.Pi/180, 100, 200, 20)

	// Collect horizontal and vertical line positions
	// We want the innermost lines near each edge (the border-to-squares boundary)
	var hLines []int // Y positions of horizontal lines
	var vLines []int // X positions of vertical lines

	for i := 0; i < lines.Rows(); i++ {
		x1 := int(lines.GetVeciAt(i, 0)[0])
		y1 := int(lines.GetVeciAt(i, 0)[1])
		x2 := int(lines.GetVeciAt(i, 0)[2])
		y2 := int(lines.GetVeciAt(i, 0)[3])

		dx := math.Abs(float64(x2 - x1))
		dy := math.Abs(float64(y2 - y1))

		// Horizontal line: small vertical change, spans a good width
		if dy < 10 && dx > 150 {
			avgY := (y1 + y2) / 2
			hLines = append(hLines, avgY)
		}
		// Vertical line: small horizontal change, spans a good height
		if dx < 10 && dy > 150 {
			avgX := (x1 + x2) / 2
			vLines = append(vLines, avgX)
		}
	}

	mid := size / 2 // 400

	// Find the innermost border lines (closest to center from each edge)
	top := findClosestToCenter(hLines, mid, true)  // largest Y that is < mid and in the top region
	bottom := findClosestToCenter(hLines, mid, false)
	left := findClosestToCenter(vLines, mid, true)
	right := findClosestToCenter(vLines, mid, false)

	// Validate: lines should be in the border region (outer 5%-20% of each edge)
	minBorder := int(float64(size) * 0.03)
	maxBorder := int(float64(size) * 0.20)

	valid := top >= minBorder && top <= maxBorder &&
		(size-bottom) >= minBorder && (size-bottom) <= maxBorder &&
		left >= minBorder && left <= maxBorder &&
		(size-right) >= minBorder && (size-right) <= maxBorder

	if valid {
		return image.Rect(left, top, right, bottom)
	}

	// Fallback: use the configured inset ratio
	inset := int(float64(size) * fallbackInsetRatio)
	return image.Rect(inset, inset, size-inset, size-inset)
}

// findClosestToCenter finds the line position closest to the center from one side.
// If fromLow is true, searches for lines below center (in the "top" or "left" border region).
func findClosestToCenter(positions []int, center int, fromLow bool) int {
	best := -1
	for _, pos := range positions {
		if fromLow {
			// Looking for the highest position that's still below center (top/left border edge)
			if pos < center && (best == -1 || pos > best) {
				best = pos
			}
		} else {
			// Looking for the lowest position that's still above center (bottom/right border edge)
			if pos > center && (best == -1 || pos < best) {
				best = pos
			}
		}
	}
	return best
}

// CropAndRewarp extracts the inner playing area from a warped board image
// and scales it back to 800x800 for consistent square extraction.
func CropAndRewarp(warped gocv.Mat, innerRect image.Rectangle) gocv.Mat {
	roi := warped.Region(innerRect)
	defer roi.Close()

	result := gocv.NewMat()
	gocv.Resize(roi, &result, image.Pt(800, 800), 0, 0, gocv.InterpolationLinear)
	return result
}

// DrawGrid draws an 8x8 grid across the board
func DrawGrid(img *gocv.Mat) {
	white := color.RGBA{255, 255, 255, 0}

	for i := 1; i < 9; i++ {
		// Vertical lines
		pos := i * 100
		gocv.Line(img, image.Pt(pos, 0), image.Pt(pos, 800), white, 1)

		// Horizontal lines
		gocv.Line(img, image.Pt(0, pos), image.Pt(800, pos), white, 1)
	}
}

// BoardSmoother is a struct to encapsulate corner smoothing
type BoardSmoother struct {
	LastCorners         []image.Point
	Alpha               float64 // Smoothing factor (0.1 = very smooth, 0.9 = very reactive)
	FramesSinceDetected int     // Counts frames since last successful detection
}

// NewBoardSmoother creates a new instance of the board smoother
func NewBoardSmoother(alpha float64) *BoardSmoother {
	return &BoardSmoother{Alpha: alpha}
}

// Smooth smooths out jitter from the boards corners due to lighting, noise variations.
// If detection is lost for too long, it relaxes constraints to allow re-acquisition.
func (s *BoardSmoother) Smooth(newCorners []image.Point) []image.Point {
	if len(newCorners) != 4 {
		s.FramesSinceDetected++

		// After ~1 second (30 frames) of no detection, reset so we can re-acquire
		if s.FramesSinceDetected > 30 {
			s.LastCorners = nil
		}
		return s.LastCorners
	}

	// Successful detection — reset counter
	s.FramesSinceDetected = 0

	// If its our first detection, accept it
	if len(s.LastCorners) != 4 {
		s.LastCorners = ReorderPoints(newCorners)
		return s.LastCorners
	}

	sortedNew := ReorderPoints(newCorners)

	// Base threshold for unexpected jumps.
	// After 15 frames (~0.5s) of no detection, gradually relax to allow re-acquisition.
	maxJump := 50.0
	if s.FramesSinceDetected > 15 {
		maxJump = 150.0
	}

	smoothed := make([]image.Point, 4)
	for i := 0; i < 4; i++ {
		movement := DistanceBetweenPoints(s.LastCorners[i], sortedNew[i])

		if movement > maxJump {
			smoothed[i] = s.LastCorners[i]
		} else {
			// Lerp formula: current + (target - current) * alpha
			smoothed[i].X = int(float64(s.LastCorners[i].X) + float64(sortedNew[i].X-s.LastCorners[i].X)*s.Alpha)
			smoothed[i].Y = int(float64(s.LastCorners[i].Y) + float64(sortedNew[i].Y-s.LastCorners[i].Y)*s.Alpha)
		}
	}
	s.LastCorners = smoothed
	return smoothed
}
