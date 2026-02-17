package main

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"github.com/intothevoid/nayan/pkg/camera"
	"github.com/intothevoid/nayan/pkg/ui"
	"github.com/intothevoid/nayan/pkg/vision"
	"gocv.io/x/gocv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	DEVICE_ID_IPHONE int = 0
	DEVICE_ID_WEBCAM int = 1
)

// fixedHeightLayout gives its children a fixed height and the full available width.
type fixedHeightLayout struct {
	height float32
}

func (l *fixedHeightLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, l.height)
}

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(size.Width, l.height))
	}
}

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
	mainDisplay := ui.NewVideoDisplay()   // Camera feed (large)
	greyDisplay := ui.NewVideoDisplay()   // Greyscale debug view
	edgesDisplay := ui.NewVideoDisplay()  // Edge map debug view
	warpedDisplay := ui.NewVideoDisplay() // Warped top-down debug view
	boardWidget := ui.NewBoardWidget()    // Virtual chessboard

	// Debug view visibility toggles (thread-safe)
	var toggleMu sync.Mutex
	showGrey := true
	showEdges := true
	showWarped := true

	// Checkbox controls
	greyCheck := widget.NewCheck("Greyscale", func(checked bool) {
		toggleMu.Lock()
		showGrey = checked
		toggleMu.Unlock()
		fyne.Do(func() {
			if checked {
				greyDisplay.Show()
			} else {
				greyDisplay.Hide()
			}
		})
	})
	greyCheck.Checked = true

	edgesCheck := widget.NewCheck("Edges", func(checked bool) {
		toggleMu.Lock()
		showEdges = checked
		toggleMu.Unlock()
		fyne.Do(func() {
			if checked {
				edgesDisplay.Show()
			} else {
				edgesDisplay.Hide()
			}
		})
	})
	edgesCheck.Checked = true

	warpedCheck := widget.NewCheck("Warped", func(checked bool) {
		toggleMu.Lock()
		showWarped = checked
		toggleMu.Unlock()
		fyne.Do(func() {
			if checked {
				warpedDisplay.Show()
			} else {
				warpedDisplay.Hide()
			}
		})
	})
	warpedCheck.Checked = true

	checkboxBar := container.NewHBox(greyCheck, edgesCheck, warpedCheck)

	// ── Left panel ──
	// Camera feed on top (takes 2x height), three debug views in a row below (1x height).
	// Using VSplit at 0.67 gives the camera 2/3 and the debug row 1/3.
	debugRow := container.NewGridWithColumns(3, greyDisplay, edgesDisplay, warpedDisplay)
	leftContent := container.NewVSplit(mainDisplay, debugRow)
	leftContent.Offset = 0.67
	leftPanel := container.NewBorder(checkboxBar, nil, nil, nil, leftContent)

	// ── Right panel ──
	// Top: graphical board filling available space. Bottom: FEN + recommended move labels.
	fenLabel := widget.NewLabel("FEN: (waiting for detection)")
	fenLabel.TextStyle = fyne.TextStyle{Monospace: true}
	fenLabel.Wrapping = fyne.TextWrapWord

	moveLabel := widget.NewLabel("Recommended: --")
	moveLabel.TextStyle = fyne.TextStyle{Bold: true}

	analysisPanel := container.NewVBox(fenLabel, moveLabel)

	rightPanel := container.NewBorder(nil, analysisPanel, nil, nil, boardWidget)

	// ── Top area ──
	topSplit := container.NewHSplit(leftPanel, rightPanel)
	topSplit.Offset = 0.6

	// ── Bottom status bar (fixed height) ──
	statusLabel := widget.NewLabel("Starting up...")
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}
	statusLabel.Wrapping = fyne.TextWrapWord

	debugLabel := widget.NewLabel("")
	debugLabel.TextStyle = fyne.TextStyle{Monospace: true}

	statusTitle := widget.NewRichTextFromMarkdown("**Status**")
	debugTitle := widget.NewRichTextFromMarkdown("**Debug**")

	statusPanel := container.NewBorder(statusTitle, nil, nil, nil, statusLabel)
	debugScroll := container.NewVScroll(debugLabel)
	debugPanel := container.NewBorder(debugTitle, nil, nil, nil, debugScroll)

	statusBar := container.NewHSplit(statusPanel, debugPanel)
	statusBar.Offset = 0.5

	statusWrapper := container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), statusBar)
	fixedStatusBar := container.New(&fixedHeightLayout{height: 120}, statusWrapper)

	// ── Overall layout ──
	// Border layout: fixed status bar pinned at bottom, top split fills the rest.
	mainLayout := container.NewBorder(nil, fixedStatusBar, nil, nil, topSplit)

	smoother := vision.NewBoardSmoother(0.3)

	// Helper to update status label from the goroutine
	setStatus := func(msg string) {
		fyne.Do(func() {
			statusLabel.SetText(msg)
		})
	}

	// Debug log buffer (keeps last few messages)
	var debugMu sync.Mutex
	debugLines := make([]string, 0, 20)
	addDebug := func(msg string) {
		debugMu.Lock()
		debugLines = append(debugLines, msg)
		if len(debugLines) > 15 {
			debugLines = debugLines[len(debugLines)-15:]
		}
		combined := ""
		for _, l := range debugLines {
			combined += l + "\n"
		}
		debugMu.Unlock()
		fyne.Do(func() {
			debugLabel.SetText(combined)
			debugScroll.ScrollToBottom()
		})
	}

	// Calibration state
	var referenceBoard gocv.Mat
	calibrated := false
	var boardDetectedSince time.Time
	boardStable := false
	var lastOccupancy [8][8]bool

	setStatus("Waiting for camera...")
	addDebug("Application started")

	// 4. The Background Loop (Goroutine)
	go func() {
		frameCount := 0
		for {
			mat, err := stream.ReadRaw()
			if err != nil || mat.Empty() {
				continue
			}

			// Mirror the camera feed horizontally so it feels natural
			gocv.Flip(*mat, mat, -1)
			frameCount++

			if frameCount == 1 {
				setStatus("Camera ready. Looking for board...")
				addDebug("First frame received from camera")
			}

			// Run preprocessing and get intermediate stages for debug views
			tempMat := mat.Clone()
			stages := vision.PreprocessStages(tempMat)
			tempMat.Close()

			// Update debug views only if enabled
			toggleMu.Lock()
			wantGrey := showGrey
			wantEdges := showEdges
			wantWarped := showWarped
			toggleMu.Unlock()

			if wantGrey {
				greyImg, _ := stages.Grey.ToImage()
				greyDisplay.UpdateFrame(greyImg)
			}

			if wantEdges {
				edgesImg, _ := stages.Edges.ToImage()
				edgesDisplay.UpdateFrame(edgesImg)
			}

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
							setStatus("Board detected. Starting calibration... please wait")
							addDebug("Board corners found, starting stability timer")
						} else {
							elapsed := time.Since(boardDetectedSince)
							remaining := 3*time.Second - elapsed
							if remaining > 0 {
								setStatus(fmt.Sprintf("Calibrating... %.1fs remaining", remaining.Seconds()))
							}
							if elapsed >= 3*time.Second {
								referenceBoard = warpedMat.Clone()
								calibrated = true
								setStatus("Calibration complete! Place pieces on the board")
								addDebug("Reference board captured successfully")
								fmt.Println("Calibration complete — reference board captured")
							}
						}
					}

					// If calibrated, scan for occupancy and update board widget
					if calibrated {
						occupancy := vision.ScanBoard(warpedMat, referenceBoard)
						vision.DrawOccupancy(&warpedMat, occupancy)

						if occupancy != lastOccupancy {
							vision.PrintOccupancy(occupancy)
							boardWidget.UpdateOccupancy(occupancy)

							count := 0
							for r := 0; r < 8; r++ {
								for c := 0; c < 8; c++ {
									if occupancy[r][c] {
										count++
									}
								}
							}
							addDebug(fmt.Sprintf("Occupancy changed: %d squares occupied", count))
							lastOccupancy = occupancy
						}
					}

					// Draw the grid on the warped mat and update debug view
					vision.DrawGrid(&warpedMat)

					if wantWarped {
						warpedImg, _ := warpedMat.ToImage()
						warpedDisplay.UpdateFrame(warpedImg)
					}

					// Draw corner markers on the original mat (thin white circles)
					for _, pt := range stableCorners {
						gocv.Circle(mat, pt, 8, color.RGBA{255, 255, 255, 0}, 1)
					}

					warpedMat.Close()
				} else {
					if boardStable && !calibrated {
						setStatus("Board lost. Looking for board...")
						addDebug("Board detection lost, resetting timer")
					}
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
	window.Resize(fyne.NewSize(1280, 900))
	window.SetFullScreen(true)
	window.ShowAndRun()
}
