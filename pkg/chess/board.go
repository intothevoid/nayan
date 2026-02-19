package chess

import (
	"fmt"

	"github.com/notnil/chess"
)

// Color represents the human player's colour.
type Color int

const (
	White Color = iota
	Black
)

// SquareFromRowCol converts vision grid coordinates to a chess.Square.
// Vision grid: row 0 = rank 8 (top of board), col 0 = file a (left).
func SquareFromRowCol(row, col int) chess.Square {
	rank := 7 - row // row 0 → rank 7 (8th rank)
	file := col      // col 0 → file 0 (a-file)
	return chess.NewSquare(chess.File(file), chess.Rank(rank))
}

// RowColFromSquare converts a chess.Square back to vision grid coordinates.
func RowColFromSquare(sq chess.Square) (row, col int) {
	row = 7 - int(sq.Rank())
	col = int(sq.File())
	return
}

// GameState tracks the chess game, linking vision occupancy to game logic.
type GameState struct {
	game       *chess.Game
	HumanColor Color
}

// NewGame creates a new game from the standard starting position.
func NewGame(humanColor Color) *GameState {
	return &GameState{
		game:       chess.NewGame(),
		HumanColor: humanColor,
	}
}

// Game returns the underlying chess.Game for engine queries.
func (gs *GameState) Game() *chess.Game {
	return gs.game
}

// FEN returns the FEN string of the current position.
func (gs *GameState) FEN() string {
	return gs.game.FEN()
}

// IsHumanTurn returns true if it's the human player's turn.
func (gs *GameState) IsHumanTurn() bool {
	turn := gs.game.Position().Turn()
	if gs.HumanColor == White {
		return turn == chess.White
	}
	return turn == chess.Black
}

// IsGameOver returns true if the game has ended.
func (gs *GameState) IsGameOver() bool {
	return gs.game.Outcome() != chess.NoOutcome
}

// Outcome returns a human-readable game result string.
func (gs *GameState) Outcome() string {
	outcome := gs.game.Outcome()
	method := gs.game.Method()
	switch outcome {
	case chess.WhiteWon:
		return fmt.Sprintf("White wins (%s)", method)
	case chess.BlackWon:
		return fmt.Sprintf("Black wins (%s)", method)
	case chess.Draw:
		return fmt.Sprintf("Draw (%s)", method)
	default:
		return "In progress"
	}
}

// MoveToAlgebraic returns standard algebraic notation for a move.
func (gs *GameState) MoveToAlgebraic(m *chess.Move) string {
	return chess.AlgebraicNotation{}.Encode(gs.game.Position(), m)
}

// ExpectedOccupancy generates an 8x8 occupancy grid from the current game state.
// true = square has a piece, false = empty.
func (gs *GameState) ExpectedOccupancy() [8][8]bool {
	var occ [8][8]bool
	board := gs.game.Position().Board()
	for sq := chess.A1; sq <= chess.H8; sq++ {
		if board.Piece(sq) != chess.NoPiece {
			row, col := RowColFromSquare(sq)
			occ[row][col] = true
		}
	}
	return occ
}

// InferMove finds the legal move that transforms the current position's
// occupancy into the observed occupancy from the vision system.
//
// For each legal move, it simulates the resulting position and compares
// occupancy grids. This naturally handles castling (2 pieces move),
// en passant (extra square vacated), and captures.
//
// When multiple moves produce the same occupancy (e.g. different promotion
// choices), queen promotion is preferred.
func (gs *GameState) InferMove(observed [8][8]bool) (*chess.Move, error) {
	pos := gs.game.Position()
	validMoves := pos.ValidMoves()

	var matches []*chess.Move
	for _, move := range validMoves {
		simPos := pos.Update(move)
		simOcc := occupancyFromBoard(simPos.Board())
		if simOcc == observed {
			matches = append(matches, move)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no legal move matches the observed board state")
	}

	// If multiple matches (e.g. different promotion types), prefer queen
	if len(matches) == 1 {
		return matches[0], nil
	}
	for _, m := range matches {
		if m.Promo() == chess.Queen {
			return m, nil
		}
	}
	// Fallback to first match
	return matches[0], nil
}

// ApplyMove applies a move to the game state.
func (gs *GameState) ApplyMove(m *chess.Move) error {
	return gs.game.Move(m)
}

// PieceGrid returns the current board as an 8x8 grid of chess.Piece values.
// Row 0 = rank 8 (top), col 0 = file a (left).
func (gs *GameState) PieceGrid() [8][8]chess.Piece {
	var grid [8][8]chess.Piece
	board := gs.game.Position().Board()
	for sq := chess.A1; sq <= chess.H8; sq++ {
		row, col := RowColFromSquare(sq)
		grid[row][col] = board.Piece(sq)
	}
	return grid
}

// CheckedKingSquare returns the row/col of the king in check after the given
// move was applied. Call this AFTER ApplyMove. Returns inCheck=false if the
// move does not give check.
func (gs *GameState) CheckedKingSquare(m *chess.Move) (row, col int, inCheck bool) {
	if !m.HasTag(chess.Check) {
		return 0, 0, false
	}
	pos := gs.game.Position()
	turn := pos.Turn() // side to move is the one in check
	kingPiece := chess.WhiteKing
	if turn == chess.Black {
		kingPiece = chess.BlackKing
	}
	board := pos.Board()
	for sq := chess.A1; sq <= chess.H8; sq++ {
		if board.Piece(sq) == kingPiece {
			row, col = RowColFromSquare(sq)
			return row, col, true
		}
	}
	return 0, 0, false
}

// PieceGridFromPosition returns a piece grid from any chess.Position.
// Useful for displaying historical positions (e.g. move history viewer).
func PieceGridFromPosition(pos *chess.Position) [8][8]chess.Piece {
	var grid [8][8]chess.Piece
	board := pos.Board()
	for sq := chess.A1; sq <= chess.H8; sq++ {
		row, col := RowColFromSquare(sq)
		grid[row][col] = board.Piece(sq)
	}
	return grid
}

// occupancyFromBoard generates an occupancy grid from a chess.Board.
func occupancyFromBoard(board *chess.Board) [8][8]bool {
	var occ [8][8]bool
	for sq := chess.A1; sq <= chess.H8; sq++ {
		if board.Piece(sq) != chess.NoPiece {
			row, col := RowColFromSquare(sq)
			occ[row][col] = true
		}
	}
	return occ
}
