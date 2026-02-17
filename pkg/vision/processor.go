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

// PreprocessResult holds intermediate preprocessing stages for debug visualisation.
// Caller is responsible for closing both Mats.
type PreprocessResult struct {
	Grey  gocv.Mat
	Edges gocv.Mat
}

// PreprocessStages runs the same pipeline as Preprocess but returns both the
// greyscale and final edge Mat so they can be displayed as debug views.
func PreprocessStages(input gocv.Mat) PreprocessResult {
	if input.Empty() {
		fmt.Println("Vision Error: Input Mat is empty")
		return PreprocessResult{Grey: gocv.NewMat(), Edges: gocv.NewMat()}
	}

	grey := toGrey(input)
	if grey.Empty() {
		fmt.Println("Vision Error: Grey Mat is empty")
		return PreprocessResult{Grey: grey, Edges: gocv.NewMat()}
	}

	blurred := gocv.NewMat()
	gocv.GaussianBlur(grey, &blurred, image.Pt(7, 7), 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	gocv.Canny(blurred, &edges, 50, 150)
	blurred.Close()

	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 5))
	defer kernel.Close()

	closed := gocv.NewMat()
	gocv.MorphologyEx(edges, &closed, gocv.MorphClose, kernel)
	edges.Close()

	return PreprocessResult{Grey: grey, Edges: closed}
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

// DetectInnerBoard returns the rectangle of the inner playing area within
// a warped board image. Uses a fixed inset ratio — since the outer frame
// detection is consistent, the border width is a predictable fraction of
// the warped image size.
//
// insetRatio is the fraction of the image taken up by the border on each side.
// For a typical wooden board with notation labels, 0.06–0.10 works well.
func DetectInnerBoard(warped gocv.Mat, insetRatio float64) image.Rectangle {
	size := warped.Rows() // should be 800
	inset := int(float64(size) * insetRatio)
	return image.Rect(inset, inset, size-inset, size-inset)
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
