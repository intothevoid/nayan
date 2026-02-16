package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gocv.io/x/gocv"
)

// Preprocess converts the raw frame into a simplified edge map
func Preprocess(input gocv.Mat) gocv.Mat {
	if input.Empty() {
		fmt.Println("Vision Error: Input Mat is empty")
		return gocv.NewMat()
	}

	// convert to greyscale
	// Check if input is already 1-channel (grayscale)
	grey := gocv.NewMat()
	if input.Channels() == 3 {
		gocv.CvtColor(input, &grey, gocv.ColorBGRToGray)
	} else {
		input.CopyTo(&grey)
	}

	if grey.Empty() {
		fmt.Println("Vision Error: Grey Mat is empty")
		return grey
	}

	// gaussian blur to reduce noise (5x5 kernel)
	blurred := gocv.NewMat()
	gocv.GaussianBlur(grey, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	if blurred.Empty() {
		fmt.Println("Vision Error: Blurred Mat is empty")
		return blurred
	}

	// canny edge detection
	// adjust (50,150) depending on lighting
	edges := gocv.NewMat()
	gocv.Canny(blurred, &edges, 50, 150)

	// check edges
	if edges.Empty() {
		fmt.Println("Vision Error: Edges Mat is empty after Canny")
	}

	// create a 3x3 kernel
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
	defer kernel.Close()

	// dilate edges to close small gaps
	dilated := gocv.NewMat()
	gocv.Dilate(edges, &dilated, kernel)

	// check dilated
	if dilated.Empty() {
		fmt.Println("Vision Error: Dilated Mat is empty after Dilation")
	}

	// clean up intermediate mats
	grey.Close()
	blurred.Close()
	edges.Close()

	return dilated
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
	LastCorners []image.Point
	Alpha       float64 // Smoothing factor (0.1 = very smooth, 0.9 = very reactive)
}

// NewBoardSmoother creates a new instance of the board smoother
func NewBoardSmoother(alpha float64) *BoardSmoother {
	return &BoardSmoother{Alpha: alpha}
}

// Smooth smooths out jitter from the boards corners due to lighting, noise variations
func (s *BoardSmoother) Smooth(newCorners []image.Point) []image.Point {
	if len(newCorners) != 4 {
		return s.LastCorners
	}

	// If its our first detection, accept it
	if len(s.LastCorners) != 4 {
		s.LastCorners = newCorners
		return newCorners
	}

	sortedNew := ReorderPoints(newCorners)
	const maxJump = 50.0 // 50 pixels threshold for sudden unexpected jumps

	smoothed := make([]image.Point, 4)
	for i := 0; i < 4; i++ {
		// Calculate how far the corner moved
		movement := DistanceBetweenPoints(s.LastCorners[i], sortedNew[i])

		if movement > maxJump {
			// We jumped too far, use the last position
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
