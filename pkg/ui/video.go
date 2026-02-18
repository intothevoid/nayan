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
	mu         sync.Mutex
	image      *canvas.Image
	imgW, imgH int // original image dimensions (set in UpdateFrame)

	// OnTapped is called with image-space coordinates when the user taps the display
	OnTapped func(imgX, imgY int)
}

// NewVideoDisplay is used to create widget instance
func NewVideoDisplay() *VideoDisplay {
	v := &VideoDisplay{}
	v.ExtendBaseWidget(v)

	// Create the internal canvas image
	v.image = canvas.NewImageFromImage(nil)
	v.image.FillMode = canvas.ImageFillContain
	return v
}

// UpdateFrame is a thread safe way to send a new image
func (v *VideoDisplay) UpdateFrame(img image.Image) {
	v.mu.Lock()
	v.image.Image = img
	if img != nil {
		v.imgW = img.Bounds().Dx()
		v.imgH = img.Bounds().Dy()
	}
	v.mu.Unlock()

	// Ask Fyne to queue redraw image on the main UI thread
	v.Refresh()
}

// Tapped maps widget-space tap coordinates to image-space and calls OnTapped.
func (v *VideoDisplay) Tapped(e *fyne.PointEvent) {
	if v.OnTapped == nil {
		return
	}

	v.mu.Lock()
	iw, ih := v.imgW, v.imgH
	v.mu.Unlock()

	if iw == 0 || ih == 0 {
		return
	}

	// Widget dimensions
	widgetW := float64(v.Size().Width)
	widgetH := float64(v.Size().Height)

	// ImageFillContain scaling: uniform scale that fits the image inside the widget
	scale := widgetW / float64(iw)
	if s := widgetH / float64(ih); s < scale {
		scale = s
	}

	// Offset to center the image within the widget
	offsetX := (widgetW - float64(iw)*scale) / 2
	offsetY := (widgetH - float64(ih)*scale) / 2

	// Map tap position to image coordinates
	imgX := int((float64(e.Position.X) - offsetX) / scale)
	imgY := int((float64(e.Position.Y) - offsetY) / scale)

	// Only call back if within image bounds
	if imgX >= 0 && imgX < iw && imgY >= 0 && imgY < ih {
		v.OnTapped(imgX, imgY)
	}
}

// TappedSecondary is a no-op required by fyne.Tappable.
func (v *VideoDisplay) TappedSecondary(*fyne.PointEvent) {}

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
	return fyne.NewSize(100, 75)
}

// Objects implements [fyne.WidgetRenderer].
func (r *videoRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.v.image}
}

// Refresh implements [fyne.WidgetRenderer].
func (r *videoRenderer) Refresh() {
	r.v.mu.Lock()
	fyne.Do(func() {
		r.v.image.Refresh()
	})
	r.v.mu.Unlock()
}

func (r *videoRenderer) Layout(s fyne.Size) {
	r.v.image.Resize(s)
}
