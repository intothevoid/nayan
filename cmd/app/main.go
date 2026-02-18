package main

import (
	"fmt"
	"image"
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

// Manual calibration state machine
type calibState int

const (
	calibIdle      calibState = iota // waiting for user to click Calibrate
	calibSelecting                   // user is clicking corners (0-3 collected)
	calibDone                        // corners captured, detecting pieces
)

// Corner labels in selection order
var cornerNames = [4]string{"top-left", "top-right", "bottom-right", "bottom-left"}

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

	// ── Status bar widgets (declared early so callbacks can reference them) ──
	statusLabel := widget.NewLabel("Starting up...")
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}
	statusLabel.Wrapping = fyne.TextWrapWord

	debugLabel := widget.NewLabel("")
	debugLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Helper to update status label from any goroutine
	setStatus := func(msg string) {
		fyne.Do(func() {
			statusLabel.SetText(msg)
		})
	}

	// Debug log buffer (keeps last few messages)
	statusTitle := widget.NewRichTextFromMarkdown("**Status**")
	debugTitle := widget.NewRichTextFromMarkdown("**Debug**")

	statusPanel := container.NewBorder(statusTitle, nil, nil, nil, statusLabel)
	debugScroll := container.NewVScroll(debugLabel)
	debugPanel := container.NewBorder(debugTitle, nil, nil, nil, debugScroll)

	statusBar := container.NewHSplit(statusPanel, debugPanel)
	statusBar.Offset = 0.5

	statusWrapper := container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), statusBar)
	fixedStatusBar := container.New(&fixedHeightLayout{height: 120}, statusWrapper)

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

	// ── Calibration state (protected by calibMu) ──
	var calibMu sync.Mutex
	calibMode := calibIdle
	calibCorners := make([]image.Point, 0, 4)
	var manualCorners []image.Point // final 4 corners for warping
	calibDoneFrame := 0            // frame counter for "Calibration complete!" overlay

	// Calibrate button — green with white text, in the toolbar
	calibrateBtn := widget.NewButton("Calibrate", func() {
		calibMu.Lock()
		calibMode = calibSelecting
		calibCorners = calibCorners[:0]
		manualCorners = nil
		calibDoneFrame = 0
		calibMu.Unlock()

		setStatus("Click the 4 board corners: TL, TR, BR, BL")
		addDebug("Calibration started — click 4 corners on camera feed")
	})
	calibrateBtn.Importance = widget.SuccessImportance

	checkboxBar := container.NewBorder(nil, nil, nil, calibrateBtn,
		container.NewHBox(greyCheck, edgesCheck, warpedCheck))

	// ── Left panel ──
	debugRow := container.NewGridWithColumns(3, greyDisplay, edgesDisplay, warpedDisplay)
	leftContent := container.NewVSplit(mainDisplay, debugRow)
	leftContent.Offset = 0.67
	leftPanel := container.NewBorder(checkboxBar, nil, nil, nil, leftContent)

	// ── Right panel ──
	fenLabel := widget.NewLabel("FEN: (waiting for calibration)")
	fenLabel.TextStyle = fyne.TextStyle{Monospace: true}
	fenLabel.Wrapping = fyne.TextWrapWord

	moveLabel := widget.NewLabel("Recommended: --")
	moveLabel.TextStyle = fyne.TextStyle{Bold: true}

	analysisPanel := container.NewVBox(fenLabel, moveLabel)
	rightPanel := container.NewBorder(nil, analysisPanel, nil, nil, boardWidget)

	// ── Top area ──
	topSplit := container.NewHSplit(leftPanel, rightPanel)
	topSplit.Offset = 0.6

	// ── Overall layout ──
	mainLayout := container.NewBorder(nil, fixedStatusBar, nil, nil, topSplit)

	var lastOccupancy [8][8]bool

	setStatus("Waiting for camera...")
	addDebug("Application started")

	// ── Tap handler for corner selection ──
	mainDisplay.OnTapped = func(imgX, imgY int) {
		calibMu.Lock()
		defer calibMu.Unlock()

		if calibMode != calibSelecting {
			return
		}

		calibCorners = append(calibCorners, image.Point{X: imgX, Y: imgY})
		n := len(calibCorners)
		addDebug(fmt.Sprintf("Corner %d/4 selected at (%d, %d) — %s", n, imgX, imgY, cornerNames[n-1]))

		if n < 4 {
			setStatus(fmt.Sprintf("Corner %d/4 selected. Click %s corner next", n, cornerNames[n]))
			return
		}

		// All 4 corners collected — finalize calibration
		manualCorners = vision.ReorderPoints(calibCorners)
		calibMode = calibDone
		calibDoneFrame = 0
		setStatus("Calibration complete! Corners locked.")
		addDebug("All 4 corners captured, calibration done")
	}

	// 4. The Background Loop (Goroutine)
	go func() {
		frameCount := 0
		for {
			mat, err := stream.ReadRaw()
			if err != nil || mat.Empty() {
				continue
			}

			// Mirror the camera feed so it feels natural
			gocv.Flip(*mat, mat, -1)
			frameCount++

			if frameCount == 1 {
				setStatus("Click CALIBRATE, then click the 4 board corners")
				addDebug("First frame received from camera")
			}

			// Run preprocessing for debug views
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

			// Snapshot calibration state for this frame
			calibMu.Lock()
			mode := calibMode
			cornersCopy := make([]image.Point, len(calibCorners))
			copy(cornersCopy, calibCorners)
			var warpCorners []image.Point
			if manualCorners != nil {
				warpCorners = make([]image.Point, 4)
				copy(warpCorners, manualCorners)
			}
			doneFrame := calibDoneFrame
			calibDoneFrame++
			calibMu.Unlock()

			// Draw overlay depending on calibration state
			switch mode {
			case calibIdle:
				// Prompt the user to click the Calibrate button
				text := "Click the Calibrate button to begin..."
				gocv.PutTextWithParams(mat, text,
					image.Pt(mat.Cols()/2-250, mat.Rows()/2),
					gocv.FontHersheyDuplex, 0.7,
					color.RGBA{255, 255, 255, 0}, 2, gocv.LineAA, false)

			case calibSelecting:
				// Draw already-clicked corners as numbered circles
				colours := []color.RGBA{
					{0, 255, 0, 0},   // green
					{0, 200, 255, 0}, // cyan
					{255, 165, 0, 0}, // orange
					{255, 0, 255, 0}, // magenta
				}
				for i, pt := range cornersCopy {
					gocv.Circle(mat, pt, 10, colours[i], 3)
					gocv.PutTextWithParams(mat, fmt.Sprintf("%d", i+1),
						image.Pt(pt.X+14, pt.Y-6),
						gocv.FontHersheyDuplex, 0.6,
						colours[i], 2, gocv.LineAA, false)
				}

				next := len(cornersCopy)
				if next < 4 {
					gocv.PutTextWithParams(mat,
						fmt.Sprintf("Click corner %d/4: %s", next+1, cornerNames[next]),
						image.Pt(20, 40),
						gocv.FontHersheyDuplex, 0.7,
						color.RGBA{255, 255, 0, 0}, 2, gocv.LineAA, false)
				}

			case calibDone:
				// Show "Calibration complete!" briefly (~2 seconds = ~60 frames)
				if doneFrame < 60 {
					gocv.PutTextWithParams(mat, "Calibration complete!",
						image.Pt(20, 40),
						gocv.FontHersheyDuplex, 0.8,
						color.RGBA{0, 255, 0, 0}, 2, gocv.LineAA, false)
				}

				// Warp using manual corners
				warpedMat := vision.WarpBoard(*mat, warpCorners)

				// Detect pieces using variance-based detection (no reference needed)
				occupancy := vision.ScanBoardAbsolute(warpedMat)
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

				// Draw grid and update warped debug view
				vision.DrawGrid(&warpedMat)

				if wantWarped {
					warpedImg, _ := warpedMat.ToImage()
					warpedDisplay.UpdateFrame(warpedImg)
				}

				// Draw corner markers on camera feed
				for _, pt := range warpCorners {
					gocv.Circle(mat, pt, 8, color.RGBA{255, 255, 255, 0}, 2)
				}

				warpedMat.Close()
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
