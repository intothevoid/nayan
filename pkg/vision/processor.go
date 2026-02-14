package vision

import (
	"fmt"
	"image"

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
	gocv.GaussianBlur(input, &blurred, image.Pt(5, 5), 0, 0, gocv.BorderDefault)

	if blurred.Empty() {
		fmt.Println("Vision Error: Blurred Mat is empty")
		return blurred
	}

	// canny edge detection
	// adjust (50,150) depending on lighting
	edges := gocv.NewMat()
	gocv.Canny(blurred, &edges, 50, 150)

	// Final check before returning
	if edges.Empty() {
		fmt.Println("Vision Error: Edges Mat is empty after Canny")
	}

	// clean up intermediate mats
	grey.Close()
	blurred.Close()

	return edges
}
