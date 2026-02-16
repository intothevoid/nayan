# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Nayan ("vision" in Hindi) is a chess companion that uses a webcam mounted above a physical chessboard to detect the board and pieces via OpenCV, then recommends moves by passing board state (FEN) to a local Stockfish binary. The Fyne GUI shows a side-by-side layout: webcam feed on the left, virtual chessboard + analysis on the right.

## Project Goals & Roadmap

The project is being built incrementally as a learning exercise for GoCV/OpenCV:

1. **Board detection** (current focus) — Detect board corners consistently via edge/contour detection, perspective-warp to top-down view
2. **Piece detection** — Detect which squares are occupied vs empty using pixel differencing against a calibration reference
3. **Piece recognition** — Identify piece types (future: possibly MobileNet or chess-specific model)
4. **Stockfish integration** — Pass FEN string to local Stockfish binary, display recommended move
5. **Full UI** — Virtual chessboard with standard piece icons, move history, analysis panel, dark theme

## Build & Run Commands

```bash
# Build
go build -v ./...

# Run
go run ./cmd/app/main.go

# Test
go test -v ./...

# Run a single test
go test -v ./pkg/vision/ -run TestName
```

**System requirement:** OpenCV must be installed for GoCV to compile. On macOS: `brew install opencv`.

## Architecture

The app launches a Fyne split-view window, starts webcam capture in a goroutine, and runs a vision processing pipeline each frame:

1. **`cmd/app/main.go`** — Entry point. Orchestrates camera, vision pipeline, UI updates, and a 3-second calibration timer to capture a reference (empty) board.

2. **`pkg/camera/`** — `VideoStream` wraps GoCV's `VideoCapture` for webcam access (640x480). Returns both `gocv.Mat` and `image.Image`.

3. **`pkg/vision/`** — Core computer vision logic, split across three files:
   - **`processor.go`** — Image preprocessing (grayscale → blur → Canny → dilate), board contour detection (largest quadrilateral >10% screen area), corner reordering, perspective warp to 800x800, grid drawing, and `BoardSmoother` (exponential moving average with 50px jump threshold).
   - **`geometry.go`** — Euclidean distance helper.
   - **`squares.go`** — Extracts individual square regions (100x100px each from the 800x800 warped board) and determines occupancy by comparing against the calibration reference (5% pixel-change threshold).

4. **`pkg/ui/`** — `VideoDisplay` custom Fyne widget with thread-safe frame updates via mutex.

5. **`pkg/chess/`** — (planned) Chess logic, board state, FEN generation, move validation.

6. **`pkg/engine/`** — (planned) Interface to local Stockfish binary via CLI calls.

7. **`pkg/ui/`** will expand to include: virtual chessboard display with piece icons, move history list, and analysis panel.

## Key Constants

- Warped board size: 800x800 pixels (100px per square)
- Board detection: contour approximation at 2% arc length, diagonal ratio within 25%
- Occupancy detection: binary threshold at 40, change threshold at 5%
- Camera: device index 0 (iPhone) or 1 (webcam), 640x480 resolution

## Design Decisions

- **Stockfish integration:** Call the local binary directly (not a library/service) to keep it simple
- **Webcam setup:** Fixed-height mount above the board; macOS primary target
- **Piece detection strategy:** Start with occupied-vs-empty detection via pixel differencing, add piece type recognition later
- **UI layout:** Side-by-side split — webcam feed (left), virtual board + analysis (right), Fyne dark theme
- **This is a learning project:** Code should be well-explained; prefer clarity over abstraction

## Dependencies

- **GoCV** (`gocv.io/x/gocv`) — OpenCV bindings for all vision processing
- **Fyne** (`fyne.io/fyne/v2`) — Cross-platform GUI toolkit (dark mode)
- **Stockfish** — (planned) Local binary for move recommendations
