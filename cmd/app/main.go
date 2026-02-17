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
	stream, err := camera.NewVideoStream(DEVICE_ID_WEBCAM)
	if err != nil {
		panic(fmt.Sprintf("Could not open camera: %v", err))
	}
	defer stream.Close()

	// 3. Create display widgets
	mainDisplay := ui.NewVideoDisplay()   // Camera feed (left)
	greyDisplay := ui.NewVideoDisplay()   // Greyscale debug view
	edgesDisplay := ui.NewVideoDisplay()  // Edge map debug view
	warpedDisplay := ui.NewVideoDisplay() // Warped board debug view
	boardWidget := ui.NewBoardWidget()    // Virtual chessboard status bar

	// Right panel: 3 stacked debug views
	rightGrid := container.NewGridWithRows(3, greyDisplay, edgesDisplay, warpedDisplay)

	// Top area: camera on left, debug views on right
	topSplit := container.NewHSplit(mainDisplay, rightGrid)
	topSplit.Offset = 0.6

	// Overall: top area above, virtual chessboard below
	mainLayout := container.NewVSplit(topSplit, boardWidget)
	mainLayout.Offset = 0.75

	smoother := vision.NewBoardSmoother(0.3)

	// Calibration state
	var referenceBoard gocv.Mat
	calibrated := false
	var boardDetectedSince time.Time
	boardStable := false
	var lastOccupancy [8][8]bool

	// 4. The Background Loop (Goroutine)
	go func() {
		for {
			mat, err := stream.ReadRaw()
			if err != nil || mat.Empty() {
				continue
			}

			// Run preprocessing and get intermediate stages for debug views
			tempMat := mat.Clone()
			stages := vision.PreprocessStages(tempMat)
			tempMat.Close()

			// Always update grey and edges debug views
			greyImg, _ := stages.Grey.ToImage()
			greyDisplay.UpdateFrame(greyImg)

			edgesImg, _ := stages.Edges.ToImage()
			edgesDisplay.UpdateFrame(edgesImg)

			if !stages.Edges.Empty() && stages.Edges.Rows() > 0 {
				rawCorners := vision.DetectBoard(stages.Edges)
				stableCorners := smoother.Smooth(rawCorners)

				if len(stableCorners) == 4 {
					// Stage 1: Warp by outer frame corners
					outerWarp := vision.WarpBoard(*mat, stableCorners)

					// Stage 2: Crop to just the 64 squares
					innerRect := vision.DetectInnerBoard(outerWarp, 0.01)
					warpedMat := vision.CropAndRewarp(outerWarp, innerRect)
					outerWarp.Close()

					// Calibration: after board detected stably for 3 seconds, capture reference
					if !calibrated {
						if !boardStable {
							boardDetectedSince = time.Now()
							boardStable = true
						} else if time.Since(boardDetectedSince) >= 3*time.Second {
							referenceBoard = warpedMat.Clone()
							calibrated = true
							fmt.Println("Calibration complete — reference board captured")
						}
					}

					// If calibrated, scan for occupancy and update board widget
					if calibrated {
						occupancy := vision.ScanBoard(warpedMat, referenceBoard)
						vision.DrawOccupancy(&warpedMat, occupancy)

						if occupancy != lastOccupancy {
							vision.PrintOccupancy(occupancy)
							boardWidget.UpdateOccupancy(occupancy)
							lastOccupancy = occupancy
						}
					}

					// Draw the grid on the warped mat and update debug view
					vision.DrawGrid(&warpedMat)
					warpedImg, _ := warpedMat.ToImage()
					warpedDisplay.UpdateFrame(warpedImg)

					// Draw corner circles on the original mat
					for _, pt := range stableCorners {
						gocv.Circle(mat, pt, 10, color.RGBA{0, 255, 0, 0}, 2)
					}

					warpedMat.Close()
				} else {
					// Board lost — reset stability timer
					boardStable = false
				}
			}

			// Update the main camera display
			origImg, _ := mat.ToImage()
			mainDisplay.UpdateFrame(origImg)

			// Cleanup intermediate Mats
			stages.Grey.Close()
			stages.Edges.Close()

			// Cap the frame rate (~30 FPS)
			time.Sleep(time.Millisecond * 33)
		}
	}()

	// 5. Layout and Run
	window.SetContent(mainLayout)
	window.Resize(fyne.NewSize(1280, 960))
	window.ShowAndRun()
}
