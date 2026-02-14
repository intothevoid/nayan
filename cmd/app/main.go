package main

import (
	"fmt"
	"time"

	"github.com/intothevoid/nayan/pkg/camera"
	"github.com/intothevoid/nayan/pkg/ui"
	"github.com/intothevoid/nayan/pkg/vision"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

func main() {
	// 1. Setup the Fyne UI App
	myApp := app.New()
	window := myApp.NewWindow("Nayan - OpenCV Chess Companion")

	// 2. Initialize the Camera
	// 0 is usually the default webcam
	stream, err := camera.NewVideoStream(0)
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
			// A. Get the newest raw frame
			mat, err := stream.ReadRaw()
			if err != nil || mat.Empty() {
				continue
			}

			// B. Prepare the original image for the UI
			origImg, _ := mat.ToImage()
			mainDisplay.UpdateFrame(origImg)

			// C. Process the image for the debug UI
			processedMat := vision.Preprocess(*mat)
			if processedMat.Empty() || processedMat.Rows() == 0 {
				fmt.Println("Warning: processedMat is empty!")
			}
			debugImg, _ := processedMat.ToImage()
			debugDisplay.UpdateFrame(debugImg)

			// D. Cleanup
			processedMat.Close()

			// E. Cap the frame rate to save CPU (approx 30 FPS)
			time.Sleep(time.Millisecond * 33)
		}
	}()

	// 5. Layout and Run
	// We put the image inside a container with a background
	window.SetContent(splitView)
	window.Resize(fyne.NewSize(1280, 960))
	window.ShowAndRun()
}
