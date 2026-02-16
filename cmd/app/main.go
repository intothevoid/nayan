package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/intothevoid/nayan/pkg/camera"
	"github.com/intothevoid/nayan/pkg/ui"
	"github.com/intothevoid/nayan/pkg/vision"
	"gocv.io/x/gocv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

const (
	DEVICE_ID_IPHONE int = 0
	DEVICE_ID_WEBCAM int = 1
)

func main() {
	// 1. Setup the Fyne UI App
	myApp := app.New()
	window := myApp.NewWindow("Nayan - OpenCV Chess Companion")

	// 2. Initialize the Camera
	// 0 is usually the default webcam
	stream, err := camera.NewVideoStream(DEVICE_ID_WEBCAM)
	if err != nil {
		panic(fmt.Sprintf("Could not open camera: %v", err))
	}
	// Ensure we tidy up when the app closes
	defer stream.Close()

	// 3. Create a placeholder for the video feed
	// We create a blank image initially
	mainDisplay := ui.NewVideoDisplay()
	debugDisplay := ui.NewVideoDisplay()

	// Create splitView container
	splitView := container.NewHSplit(mainDisplay, debugDisplay)
	splitView.Offset = 0.7

	smoother := vision.NewBoardSmoother(0.3)

	// 4. The Background Loop (Goroutine)
	go func() {
		for {
			// Get the newest raw frame
			mat, err := stream.ReadRaw()
			if err != nil || mat.Empty() {
				continue
			}

			// Process the image for the debug view
			// We clone the mat here to ensure the Preprocess doesn't affect our main display
			tempMat := mat.Clone()
			processedMat := vision.Preprocess(tempMat)
			tempMat.Close()

			if processedMat.Empty() || processedMat.Rows() == 0 {
				fmt.Println("Warning: processedMat is empty!")
			} else {
				// Try to find the 4 rawCorners
				rawCorners := vision.DetectBoard(processedMat)
				stableCorners := smoother.Smooth(rawCorners)

				// Draw circles on the original mat if corners found
				// Draw once per frame
				if len(stableCorners) == 4 {
					// Stage 1: Warp by outer frame corners
					outerWarp := vision.WarpBoard(*mat, stableCorners)

					// Stage 2: Crop to just the 64 squares by removing the wooden border.
					// insetRatio 0.07 = 7% border on each side (adjust if your board border differs)
					innerRect := vision.DetectInnerBoard(outerWarp, 0.01)
					warpedMat := vision.CropAndRewarp(outerWarp, innerRect)
					outerWarp.Close()

					// Draw the grid on the inner-cropped warped mat
					vision.DrawGrid(&warpedMat)

					// Convert to image for debug display
					warpedImg, _ := warpedMat.ToImage()
					debugDisplay.UpdateFrame(warpedImg)

					// Draw circles on the original mat for the main view
					for _, pt := range stableCorners {
						gocv.Circle(mat, pt, 10, color.RGBA{0, 255, 0, 0}, 2)
					}

					warpedMat.Close()
				} else {
					// If no board is found yet, show the edge map so we can troubleshoot
					debugImg, _ := processedMat.ToImage()
					debugDisplay.UpdateFrame(debugImg)
				}
			}

			// Update the original display, green circles included
			origImg, _ := mat.ToImage()
			mainDisplay.UpdateFrame(origImg)

			// Cleanup
			processedMat.Close()

			// Cap the frame rate to save CPU (approx 30 FPS)
			time.Sleep(time.Millisecond * 33)
		}
	}()

	// 5. Layout and Run
	// We put the image inside a container with a background
	window.SetContent(splitView)
	window.Resize(fyne.NewSize(1280, 960))
	window.ShowAndRun()
}
