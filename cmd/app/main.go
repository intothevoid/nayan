package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/intothevoid/nayan/pkg/camera"
	"github.com/intothevoid/nayan/pkg/ui"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
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
	vidWidget := ui.NewVideoDisplay()

	// 4. The Background Loop (Goroutine)
	go func() {
		for {
			// A. Get the newest frame
			frame, err := stream.Read()
			if err != nil {
				fmt.Println("Error reading frame:", err)
				time.Sleep(time.Second) // Wait a bit before retrying
				continue
			}

			// B. Update the UI widget in a thread safe way
			vidWidget.UpdateFrame(frame)

			// Optional: Cap the frame rate to save CPU (approx 30 FPS)
			time.Sleep(time.Millisecond * 33)
		}
	}()

	// 5. Layout and Run
	// We put the image inside a container with a background
	content := container.NewMax(
		canvas.NewRectangle(color.Black), // Dark background
		vidWidget,
	)

	window.SetContent(content)
	window.Resize(fyne.NewSize(800, 600))
	window.ShowAndRun()
}
