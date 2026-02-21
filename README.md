# Nayan

> *Nayan* means "vision" in Hindi

A chess companion that uses a webcam mounted above a physical chessboard to detect the board and pieces via OpenCV, then recommends moves using a local Stockfish engine. Built with Go as a learning exercise for GoCV/OpenCV.

![Nayan UI](docs/nayan-ui.gif)

## Features

- **Board detection** — Manual corner calibration with click-to-select, perspective-warps to a top-down 800x800 view
- **Piece detection** — Detects occupied vs empty squares using variance and edge detection against a calibration reference
- **Move inference** — Tracks game state from the known starting position; infers moves by comparing vision occupancy against all legal moves (handles castling, en passant, promotions)
- **Stockfish integration** — Queries a local Stockfish engine (via UCI) for recommended moves with configurable difficulty (depth 1-20)
- **Virtual chessboard** — Lichess-style board with SVG piece icons, rank/file labels, move highlights (blue from-square, green to-square), check indicator (red overlay on king), and flashing red highlights for invalid board states
- **Move history viewer** — Popup window with a graphical chessboard and prev/next navigation to step through the game move by move
- **CPU vs CPU mode** — Watch Stockfish play against itself 
- **Play as White or Black** — Choose your colour before starting a game

## Prerequisites

- **Go** 1.21+
- **OpenCV** — required for GoCV to compile
  ```bash
  # macOS
  brew install opencv
  ```
- **Stockfish** — chess engine binary on your PATH
  ```bash
  # macOS
  brew install stockfish
  ```
- **Webcam** — mounted above the board looking down

## Build & Run

```bash
# Build
go build -v ./...

# Run
go run ./cmd/app/main.go

# Test
go test -v ./...
```

## How It Works

1. The app opens a webcam feed and displays it in the left panel
2. Click **Calibrate** and then click the four corners of the board (top-left, top-right, bottom-right, bottom-left) on the webcam feed
3. The board region is perspective-warped to a flat 800x800 image and a 2-second settle period captures the reference state
4. Choose your colour (White/Black) and click **Start Game**
5. Make a move on the physical board — after 5 stable frames with the same occupancy change, the app infers the legal move and updates the game state
6. Stockfish recommends the opponent's response, highlighted on the virtual board (blue = from, green = to)
7. Physically make the recommended move — the cycle repeats until checkmate, stalemate, or you stop the game
8. Illegal board states (e.g. moving the wrong piece) are flagged with flashing red squares until corrected

## Project Structure

```
cmd/app/main.go          Entry point — camera, vision pipeline, game loop, UI
pkg/camera/
  camera.go              VideoStream wrapping GoCV's VideoCapture (640x480)
pkg/chess/
  board.go               Game state, move inference, FEN, coordinate mapping, check detection
  board_test.go          Unit tests for coordinates, occupancy, move inference
pkg/engine/
  stockfish.go           Stockfish UCI wrapper (BestMove, configurable depth)
pkg/ui/
  board.go               Lichess-style virtual chessboard widget (Fyne custom widget)
  video.go               Custom Fyne widget for thread-safe video frame display
  assets.go              Embedded SVG piece resources and PieceType mapping
  pieces/                SVG piece images (wK, wQ, wR, wB, wN, wP, bK, bQ, bR, bB, bN, bP)
pkg/vision/
  processor.go           Preprocessing, board contour detection, perspective warp, grid drawing
  squares.go             Square extraction, occupancy detection, board scanning
  geometry.go            Euclidean distance helper
```

## Dependencies

- [GoCV](https://gocv.io/) — OpenCV bindings for Go
- [Fyne](https://fyne.io/) — Cross-platform GUI toolkit
- [notnil/chess](https://github.com/notnil/chess) — Chess game logic, move generation, FEN, UCI
- [Stockfish](https://stockfishchess.org/) — Chess engine for move recommendations

## License

MIT
