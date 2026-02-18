package ui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	lightSquare = color.NRGBA{R: 0xf0, G: 0xd9, B: 0xb5, A: 0xff} // #f0d9b5
	darkSquare  = color.NRGBA{R: 0xb5, G: 0x88, B: 0x63, A: 0xff} // #b58863
	markerColor = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xcc} // dark semi-transparent
)

// BoardWidget is a lichess-style virtual chessboard that shows occupancy markers.
type BoardWidget struct {
	widget.BaseWidget

	mu        sync.Mutex
	occupancy [8][8]bool

	// Pre-built canvas objects
	squares [8][8]*canvas.Rectangle
	markers [8][8]*canvas.Circle
	labels  []fyne.CanvasObject
	root    *fyne.Container
}

// NewBoardWidget creates a new virtual chessboard widget.
func NewBoardWidget() *BoardWidget {
	b := &BoardWidget{}
	b.ExtendBaseWidget(b)

	// Build squares, markers, and labels
	// Labels: 0-7 bottom files, 8-15 left ranks, 16-23 top files, 24-31 right ranks
	objects := make([]fyne.CanvasObject, 0, 64+64+32)

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			c := lightSquare
			if (row+col)%2 == 1 {
				c = darkSquare
			}
			rect := canvas.NewRectangle(c)
			b.squares[row][col] = rect
			objects = append(objects, rect)

			circle := canvas.NewCircle(markerColor)
			circle.Hidden = true
			b.markers[row][col] = circle
			objects = append(objects, circle)
		}
	}

	// File labels (a-h) along the bottom (indices 0-7)
	for col := 0; col < 8; col++ {
		t := canvas.NewText(string(rune('a'+col)), color.White)
		t.TextSize = 11
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// Rank labels (8-1) along the left (indices 8-15)
	for row := 0; row < 8; row++ {
		t := canvas.NewText(string(rune('8'-row)), color.White)
		t.TextSize = 11
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// File labels (a-h) along the top (indices 16-23)
	for col := 0; col < 8; col++ {
		t := canvas.NewText(string(rune('a'+col)), color.White)
		t.TextSize = 11
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	// Rank labels (8-1) along the right (indices 24-31)
	for row := 0; row < 8; row++ {
		t := canvas.NewText(string(rune('8'-row)), color.White)
		t.TextSize = 11
		t.Alignment = fyne.TextAlignCenter
		b.labels = append(b.labels, t)
		objects = append(objects, t)
	}

	b.root = container.NewWithoutLayout(objects...)
	return b
}

// UpdateOccupancy sets the occupancy grid and refreshes the display.
func (b *BoardWidget) UpdateOccupancy(occ [8][8]bool) {
	b.mu.Lock()
	b.occupancy = occ
	b.mu.Unlock()

	fyne.Do(func() {
		for row := 0; row < 8; row++ {
			for col := 0; col < 8; col++ {
				b.markers[row][col].Hidden = !occ[row][col]
				b.markers[row][col].Refresh()
			}
		}
	})
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
	labelMargin := float32(16)

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

	r.b.root.Resize(size)

	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x := offsetX + float32(col)*sqSize
			y := offsetY + float32(row)*sqSize

			r.b.squares[row][col].Move(fyne.NewPos(x, y))
			r.b.squares[row][col].Resize(fyne.NewSize(sqSize, sqSize))

			// Marker is a circle inset within the square
			markerSize := sqSize * 0.4
			markerOffset := (sqSize - markerSize) / 2
			r.b.markers[row][col].Move(fyne.NewPos(x+markerOffset, y+markerOffset))
			r.b.markers[row][col].Resize(fyne.NewSize(markerSize, markerSize))
		}
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
