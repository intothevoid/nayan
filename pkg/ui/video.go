package ui

import (
	"image"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// Custom widget for displaying video frames
type VideoDisplay struct {
	widget.BaseWidget

	// mu ensures we don't read / write the image at the same time
	mu    sync.Mutex
	image *canvas.Image
}

// NewVideoDisplay is used to create widget instance
func NewVideoDisplay() *VideoDisplay {
	v := &VideoDisplay{}
	v.ExtendBaseWidget(v)

	// Create the internal canvas image
	v.image = canvas.NewImageFromImage(nil)
	v.image.FillMode = canvas.ImageFillOriginal
	return v
}

// UpdateFrame is a thread safe way to send a new image
func (v *VideoDisplay) UpdateFrame(img image.Image) {
	v.mu.Lock()
	v.image.Image = img
	v.image.Refresh()
	v.mu.Unlock()

	// Ask Fyne to redraw image
	v.Refresh()
}

// CreateRenderer is used to create a video renderer
func (v *VideoDisplay) CreateRenderer() fyne.WidgetRenderer {
	return &videoRenderer{v}
}

// videoRenderer implements the logic to draw the widget
type videoRenderer struct {
	v *VideoDisplay
}

// Destroy implements [fyne.WidgetRenderer].
func (r *videoRenderer) Destroy() {}

// MinSize implements [fyne.WidgetRenderer].
func (r *videoRenderer) MinSize() fyne.Size {
	return r.v.image.MinSize()
}

// Objects implements [fyne.WidgetRenderer].
func (r *videoRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.v.image}
}

// Refresh implements [fyne.WidgetRenderer].
func (r *videoRenderer) Refresh() {
	r.v.mu.Lock()
	defer r.v.mu.Unlock()
	r.v.image.Refresh()
}

func (r *videoRenderer) Layout(s fyne.Size) {
	r.v.image.Resize(s)
}
