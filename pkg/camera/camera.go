package camera

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"
)

// VideoStream manages the webcam connection
type VideoStream struct {
	deviceID int
	webcam   *gocv.VideoCapture
	frame    *gocv.Mat // Keep a reusable matrix to save memory
}

// NewVideoStream initializes the camera
func NewVideoStream(id int) (*VideoStream, error) {
	cam, err := gocv.VideoCaptureDevice(id)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %v", err)
	}

	// Optional: Set resolution (keeps processing fast)
	cam.Set(gocv.VideoCaptureFrameWidth, 640)
	cam.Set(gocv.VideoCaptureFrameHeight, 480)

	mat := gocv.NewMat()
	return &VideoStream{
		deviceID: id,
		webcam:   cam,
		frame:    &mat,
	}, nil
}

// Read returns the current frame as a standard Go image
// This is crucial for Fyne compatibility!
func (vs *VideoStream) Read() (image.Image, error) {
	if !vs.webcam.Read(vs.frame) {
		return nil, fmt.Errorf("cannot read frame")
	}
	if vs.frame.Empty() {
		return nil, fmt.Errorf("frame is empty")
	}

	// GoCV Mat -> Go Image conversion
	img, err := vs.frame.ToImage()
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (vs *VideoStream) Close() {
	vs.webcam.Close()
	vs.frame.Close()
}
