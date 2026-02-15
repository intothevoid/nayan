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
				// Try to find the 4 corners
				corners := vision.DetectBoard(processedMat)

				// Draw circles on the original mat if corners found
				for _, pt := range corners {
					// Params: mat, centre, radius, color (green), thickness
					gocv.Circle(mat, pt, 10, color.RGBA{0, 255, 0, 0}, 3)
				}

				// Update the debug display with the edge map
				debugImg, _ := processedMat.ToImage()
				debugDisplay.UpdateFrame(debugImg)
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
