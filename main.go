package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"gocv.io/x/gocv"
)

func main() {
	myApp := app.New()
	window := myApp.NewWindow("nayan") // 1. Setup the Webcam feed widget
	webcamFeed := canvas.NewImageFromImage(nil)
	webcamFeed.FillMode = canvas.ImageFillContain
	webcamFeed.SetMinSize(fyne.NewSize(320, 240)) // 2. Setup the 3-column layout (Placeholder for now)
	mainLayout := container.New(
		layout.NewGridLayout(3),
		webcamFeed,
		canvas.NewText("Virtual Board Placeholder", nil),
		canvas.NewText("AI Move Panel Placeholder", nil),
	)
	window.SetContent(mainLayout) // 3. Start the background Vision Loop

	go func() {
		webcam, _ := gocv.VideoCaptureDevice(0)
		defer webcam.Close()
		img := gocv.NewMat()
		defer img.Close()

		for {
			if ok := webcam.Read(&img); !ok || img.Empty() {
				continue
			} // Update Fyne thread-safely

			golangImage, _ := img.ToImage()
			fyne.Do(func() {
				webcamFeed.Image = golangImage
				webcamFeed.Refresh()
			})
			time.Sleep(33 * time.Millisecond) // ~30 FPS
		}
	}()

	window.ShowAndRun()
}
