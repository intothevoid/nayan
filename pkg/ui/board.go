package ui

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	lightSquare      = color.NRGBA{R: 0xf0, G: 0xd9, B: 0xb5, A: 0xff} // #f0d9b5
	darkSquare       = color.NRGBA{R: 0xb5, G: 0x88, B: 0x63, A: 0xff} // #b58863
	highlightFrom    = color.NRGBA{R: 0x00, G: 0x88, B: 0xff, A: 0x80} // blue semi-transparent
	highlightTo      = color.NRGBA{R: 0x00, G: 0xcc, B: 0x44, A: 0x80} // green semi-transparent
	highlightInvalid = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0x80} // red semi-transparent
	arrowColor       = color.NRGBA{R: 0x33, G: 0x99, B: 0xff, A: 0xaa} // bright blue translucent
)

// greyedTranslucency is the translucency applied to pieces in pre-game mode.
const greyedTranslucency = 0.7

// labelFontSize for rank/file labels on the board.
const labelFontSize = 17

// BoardWidget is a lichess-style virtual chessboard that shows piece images.
type BoardWidget struct {
	widget.BaseWidget

	mu     sync.Mutex
	pieces [8][8]PieceType

	// Flash state for invalid moves
	flashMu   sync.Mutex
	flashStop chan struct{} // closed to stop flash goroutine; nil when idle

	// Arrow state for recommended moves
	arrowActive bool
	arrowFrom   [2]int // [row, col]
	arrowTo     [2]int

	// Cached layout parameters (set in Layout, read in ShowArrow)
	layoutOffX float32
	layoutOffY float32
	layoutSqSz float32

	// Pre-built canvas objects
	squares    [8][8]*canvas.Rectangle
	highlights [8][8]*canvas.Rectangle
	pieceImgs  [8][8]*canvas.Image
	arrowShaft *canvas.Line
	arrowHead1 *canvas.Line
	arrowHead2 *canvas.Line
	labels     []fyne.CanvasObject
	root       *fyne.Container
}

// NewBoardWidget creates a new virtual chessboard widget.
// It initializes with the standard starting position in greyed-out mode.
func NewBoardWidget() *BoardWidget {
	b := &BoardWidget{}
	b.ExtendBaseWidget(b)

	// Build squares, highlights, piece images, arrow, and labels
	objects := make([]fyne.CanvasObject, 0, 64+64+64+3+32)

	startPos := StartingPosition()
	b.pieces = startPos

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			c := lightSquare
			if (row+col)%2 == 1 {
				c = darkSquare
			}
			rect := canvas.NewRectangle(c)
			b.squares[row][col] = rect
			objects = append(objects, rect)

			hl := canvas.NewRectangle(color.Transparent)
			hl.Hidden = true
			b.highlights[row][col] = hl
			objects = append(objects, hl)

			// Piece image
			pt := startPos[row][col]
			img := canvas.NewImageFromResource(nil)
			img.FillMode = canvas.ImageFillContain
			img.ScaleMode = canvas.ImageScaleSmooth
			if res := PieceResource(pt); res != nil {
				img.Resource = res
				img.Translucency = greyedTranslucency
			} else {
				img.Hidden = true
			}
			b.pieceImgs[row][col] = img
			objects = append(objects, img)
		}
	}

	// Arrow lines (shaft + 2 arrowhead lines), drawn on top of pieces
	b.arrowShaft = canvas.NewLine(arrowColor)
	b.arrowShaft.Hidden = true
	objects = append(objects, b.arrowShaft)

	b.arrowHead1 = canvas.NewLine(arrowColor)
	b.arrowHead1.Hidden = true
	objects = append(objects, b.arrowHead1)

	b.arrowHead2 = canvas.NewLine(arrowColor)
	b.arrowHead2.Hidden = true
	objects = append(objects, b.arrowHead2)

	// File labels (a-h) along the bottom (indices 0-7)
	for col := 0; col < 8; col++ {
		t := canvas.NewText(string(rune('a'+col)), color.White)
		t.TextSize = labelFontSize
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// Rank labels (8-1) along the left (indices 8-15)
	for row := 0; row < 8; row++ {
		t := canvas.NewText(string(rune('8'-row)), color.White)
		t.TextSize = labelFontSize
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// File labels (a-h) along the top (indices 16-23)
	for col := 0; col < 8; col++ {
		t := canvas.NewText(string(rune('a'+col)), color.White)
		t.TextSize = labelFontSize
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// Rank labels (8-1) along the right (indices 24-31)
	for row := 0; row < 8; row++ {
		t := canvas.NewText(string(rune('8'-row)), color.White)
		t.TextSize = labelFontSize
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	b.root = container.NewWithoutLayout(objects...)
	return b
}

// UpdatePieces sets the piece grid and refreshes the display.
// If greyed is true, pieces are shown with reduced opacity (pre-game mode).
func (b *BoardWidget) UpdatePieces(pieces [8][8]PieceType, greyed bool) {
	b.mu.Lock()
	b.pieces = pieces
	b.mu.Unlock()

	translucency := float64(0)
	if greyed {
		translucency = greyedTranslucency
	}

	fyne.Do(func() {
		for row := 0; row < 8; row++ {
			for col := 0; col < 8; col++ {
				img := b.pieceImgs[row][col]
				pt := pieces[row][col]
				res := PieceResource(pt)
				if res != nil {
					img.Resource = res
					img.Translucency = translucency
					img.Hidden = false
				} else {
					img.Hidden = true
				}
				img.Refresh()
			}
		}
	})
}

// HighlightMove shows from/to square highlights on the board.
func (b *BoardWidget) HighlightMove(fromRow, fromCol, toRow, toCol int) {
	fyne.Do(func() {
		b.clearHighlightsUnsafe()
		b.highlights[fromRow][fromCol].FillColor = highlightFrom
		b.highlights[fromRow][fromCol].Hidden = false
		b.highlights[fromRow][fromCol].Refresh()
		b.highlights[toRow][toCol].FillColor = highlightTo
		b.highlights[toRow][toCol].Hidden = false
		b.highlights[toRow][toCol].Refresh()
	})
}

// ClearHighlight hides all highlight rectangles.
func (b *BoardWidget) ClearHighlight() {
	fyne.Do(func() {
		b.clearHighlightsUnsafe()
	})
}

func (b *BoardWidget) clearHighlightsUnsafe() {
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			b.highlights[row][col].Hidden = true
			b.highlights[row][col].Refresh()
		}
	}
}

// ShowArrow draws a translucent arrow from one square to another.
// Used to show recommended/engine moves.
func (b *BoardWidget) ShowArrow(fromRow, fromCol, toRow, toCol int) {
	b.mu.Lock()
	b.arrowFrom = [2]int{fromRow, fromCol}
	b.arrowTo = [2]int{toRow, toCol}
	b.arrowActive = true
	offX := b.layoutOffX
	offY := b.layoutOffY
	sq := b.layoutSqSz
	b.mu.Unlock()

	if sq <= 0 {
		return
	}

	fyne.Do(func() {
		b.positionArrow(offX, offY, sq)
		b.arrowShaft.Hidden = false
		b.arrowHead1.Hidden = false
		b.arrowHead2.Hidden = false
		b.arrowShaft.Refresh()
		b.arrowHead1.Refresh()
		b.arrowHead2.Refresh()
	})
}

// ClearArrow hides the arrow overlay.
func (b *BoardWidget) ClearArrow() {
	b.mu.Lock()
	b.arrowActive = false
	b.mu.Unlock()

	fyne.Do(func() {
		b.arrowShaft.Hidden = true
		b.arrowHead1.Hidden = true
		b.arrowHead2.Hidden = true
		b.arrowShaft.Refresh()
		b.arrowHead1.Refresh()
		b.arrowHead2.Refresh()
	})
}

// positionArrow sets the line endpoints for the arrow. Must be called on main thread.
func (b *BoardWidget) positionArrow(offX, offY, sq float32) {
	fromX := offX + float32(b.arrowFrom[1])*sq + sq/2
	fromY := offY + float32(b.arrowFrom[0])*sq + sq/2
	toX := offX + float32(b.arrowTo[1])*sq + sq/2
	toY := offY + float32(b.arrowTo[0])*sq + sq/2

	strokeW := sq * 0.1
	b.arrowShaft.StrokeWidth = strokeW
	b.arrowShaft.Position1 = fyne.NewPos(fromX, fromY)
	b.arrowShaft.Position2 = fyne.NewPos(toX, toY)

	// Arrowhead
	dx := float64(toX - fromX)
	dy := float64(toY - fromY)
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 1 {
		return
	}
	ndx := dx / length
	ndy := dy / length

	arrowLen := float64(sq) * 0.35
	backAngle := math.Pi - 25.0*math.Pi/180.0

	cos1, sin1 := math.Cos(backAngle), math.Sin(backAngle)
	cos2, sin2 := math.Cos(-backAngle), math.Sin(-backAngle)

	ah1x := float64(toX) + arrowLen*(ndx*cos1-ndy*sin1)
	ah1y := float64(toY) + arrowLen*(ndx*sin1+ndy*cos1)
	ah2x := float64(toX) + arrowLen*(ndx*cos2-ndy*sin2)
	ah2y := float64(toY) + arrowLen*(ndx*sin2+ndy*cos2)

	headStroke := sq * 0.08
	b.arrowHead1.StrokeWidth = headStroke
	b.arrowHead1.Position1 = fyne.NewPos(toX, toY)
	b.arrowHead1.Position2 = fyne.NewPos(float32(ah1x), float32(ah1y))

	b.arrowHead2.StrokeWidth = headStroke
	b.arrowHead2.Position1 = fyne.NewPos(toX, toY)
	b.arrowHead2.Position2 = fyne.NewPos(float32(ah2x), float32(ah2y))
}

// FlashInvalid starts flashing red highlights on the given squares.
// Each entry in diffs is [row, col]. Flashes toggle every 2 seconds (4s full cycle).
// Calling again replaces any existing flash.
func (b *BoardWidget) FlashInvalid(diffs [][2]int) {
	b.stopFlash() // stop any existing flash goroutine

	if len(diffs) == 0 {
		return
	}

	stop := make(chan struct{})
	b.flashMu.Lock()
	b.flashStop = stop
	b.flashMu.Unlock()

	// Show red highlights immediately
	fyne.Do(func() {
		b.clearHighlightsUnsafe()
		for _, d := range diffs {
			b.highlights[d[0]][d[1]].FillColor = highlightInvalid
			b.highlights[d[0]][d[1]].Hidden = false
			b.highlights[d[0]][d[1]].Refresh()
		}
	})

	// Toggle flash in background (2s on / 2s off = 4s cycle)
	diffsCopy := make([][2]int, len(diffs))
	copy(diffsCopy, diffs)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		visible := true
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				visible = !visible
				fyne.Do(func() {
					for _, d := range diffsCopy {
						b.highlights[d[0]][d[1]].Hidden = !visible
						b.highlights[d[0]][d[1]].Refresh()
					}
				})
			}
		}
	}()
}

// ClearInvalid stops any active flash and hides all highlights.
func (b *BoardWidget) ClearInvalid() {
	b.stopFlash()
	fyne.Do(func() {
		b.clearHighlightsUnsafe()
	})
}

// stopFlash stops the flash goroutine if one is running.
func (b *BoardWidget) stopFlash() {
	b.flashMu.Lock()
	if b.flashStop != nil {
		close(b.flashStop)
		b.flashStop = nil
	}
	b.flashMu.Unlock()
}

func (b *BoardWidget) CreateRenderer() fyne.WidgetRenderer {
	return &boardRenderer{b: b}
}

type boardRenderer struct {
	b *BoardWidget
}

func (r *boardRenderer) Destroy() {}

func (r *boardRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, 100)
}

func (r *boardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.b.root}
}

func (r *boardRenderer) Refresh() {
	r.b.root.Refresh()
}

func (r *boardRenderer) Layout(size fyne.Size) {
	labelMargin := float32(20)

	// Reserve space on all four sides for labels
	boardW := size.Width - 2*labelMargin
	boardH := size.Height - 2*labelMargin
	boardSize := boardW
	if boardH < boardSize {
		boardSize = boardH
	}

	sqSize := boardSize / 8

	// Center the board within the available space
	totalBoardW := sqSize*8 + 2*labelMargin
	totalBoardH := sqSize*8 + 2*labelMargin
	offsetX := labelMargin + (size.Width-totalBoardW)/2
	offsetY := labelMargin + (size.Height-totalBoardH)/2

	// Cache layout params for ShowArrow
	r.b.mu.Lock()
	r.b.layoutOffX = offsetX
	r.b.layoutOffY = offsetY
	r.b.layoutSqSz = sqSize
	arrowActive := r.b.arrowActive
	r.b.mu.Unlock()

	r.b.root.Resize(size)

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x := offsetX + float32(col)*sqSize
			y := offsetY + float32(row)*sqSize

			r.b.squares[row][col].Move(fyne.NewPos(x, y))
			r.b.squares[row][col].Resize(fyne.NewSize(sqSize, sqSize))

			r.b.highlights[row][col].Move(fyne.NewPos(x, y))
			r.b.highlights[row][col].Resize(fyne.NewSize(sqSize, sqSize))

			// Piece image fills the square with a small inset
			inset := sqSize * 0.05
			r.b.pieceImgs[row][col].Move(fyne.NewPos(x+inset, y+inset))
			r.b.pieceImgs[row][col].Resize(fyne.NewSize(sqSize-2*inset, sqSize-2*inset))
		}
	}

	// Reposition arrow if active
	if arrowActive {
		r.b.positionArrow(offsetX, offsetY, sqSize)
	}

	// File labels (a-h) below the board (indices 0-7)
	for i := 0; i < 8; i++ {
		lbl := r.b.labels[i]
		x := offsetX + float32(i)*sqSize
		y := offsetY + 8*sqSize
		lbl.Move(fyne.NewPos(x, y))
		lbl.Resize(fyne.NewSize(sqSize, labelMargin))
	}

	// Rank labels (8-1) to the left of the board (indices 8-15)
	for i := 0; i < 8; i++ {
		lbl := r.b.labels[8+i]
		x := offsetX - labelMargin
		y := offsetY + float32(i)*sqSize
		lbl.Move(fyne.NewPos(x, y))
		lbl.Resize(fyne.NewSize(labelMargin, sqSize))
	}

	// File labels (a-h) above the board (indices 16-23)
	for i := 0; i < 8; i++ {
		lbl := r.b.labels[16+i]
		x := offsetX + float32(i)*sqSize
		y := offsetY - labelMargin
		lbl.Move(fyne.NewPos(x, y))
		lbl.Resize(fyne.NewSize(sqSize, labelMargin))
	}

	// Rank labels (8-1) to the right of the board (indices 24-31)
	for i := 0; i < 8; i++ {
		lbl := r.b.labels[24+i]
		x := offsetX + 8*sqSize
		y := offsetY + float32(i)*sqSize
		lbl.Move(fyne.NewPos(x, y))
		lbl.Resize(fyne.NewSize(labelMargin, sqSize))
	}
}
