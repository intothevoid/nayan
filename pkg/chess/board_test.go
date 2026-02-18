package chess

import (
	"testing"

	"github.com/notnil/chess"
)

func TestSquareFromRowCol(t *testing.T) {
	tests := []struct {
		row, col int
		want     chess.Square
	}{
		{0, 0, chess.A8}, // top-left = a8
		{0, 7, chess.H8}, // top-right = h8
		{7, 0, chess.A1}, // bottom-left = a1
		{7, 7, chess.H1}, // bottom-right = h1
		{6, 4, chess.E2}, // e2
		{4, 3, chess.D4}, // d4
	}
	for _, tt := range tests {
		got := SquareFromRowCol(tt.row, tt.col)
		if got != tt.want {
			t.Errorf("SquareFromRowCol(%d, %d) = %s, want %s", tt.row, tt.col, got, tt.want)
		}
	}
}

func TestRowColFromSquare(t *testing.T) {
	tests := []struct {
		sq       chess.Square
		wantRow  int
		wantCol  int
	}{
		{chess.A8, 0, 0},
		{chess.H8, 0, 7},
		{chess.A1, 7, 0},
		{chess.H1, 7, 7},
		{chess.E2, 6, 4},
		{chess.D4, 4, 3},
	}
	for _, tt := range tests {
		row, col := RowColFromSquare(tt.sq)
		if row != tt.wantRow || col != tt.wantCol {
			t.Errorf("RowColFromSquare(%s) = (%d, %d), want (%d, %d)", tt.sq, row, col, tt.wantRow, tt.wantCol)
		}
	}
}

func TestExpectedOccupancyStarting(t *testing.T) {
	gs := NewGame(White)
	occ := gs.ExpectedOccupancy()

	// Rows 0-1 (ranks 8-7) should be occupied (black pieces)
	for row := 0; row < 2; row++ {
		for col := 0; col < 8; col++ {
			if !occ[row][col] {
				t.Errorf("row %d, col %d should be occupied (black pieces)", row, col)
			}
		}
	}

	// Rows 2-5 (ranks 6-3) should be empty
	for row := 2; row < 6; row++ {
		for col := 0; col < 8; col++ {
			if occ[row][col] {
				t.Errorf("row %d, col %d should be empty", row, col)
			}
		}
	}

	// Rows 6-7 (ranks 2-1) should be occupied (white pieces)
	for row := 6; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if !occ[row][col] {
				t.Errorf("row %d, col %d should be occupied (white pieces)", row, col)
			}
		}
	}
}

func TestInferMoveE4(t *testing.T) {
	gs := NewGame(White)

	// Simulate e2-e4: starting position but e2 empty, e4 occupied
	observed := gs.ExpectedOccupancy()
	observed[6][4] = false // e2 vacated
	observed[4][4] = true  // e4 occupied

	move, err := gs.InferMove(observed)
	if err != nil {
		t.Fatalf("InferMove failed: %v", err)
	}

	if move.S1() != chess.E2 || move.S2() != chess.E4 {
		t.Errorf("expected e2e4, got %s%s", move.S1(), move.S2())
	}
}

func TestInferMoveCapture(t *testing.T) {
	// Set up Italian Game position where white can capture on d5
	// 1. e4 e5 2. Nf3 d5 — now white can play exd5
	gs := NewGame(White)
	gs.game.MoveStr("e4")
	gs.game.MoveStr("e5")
	gs.game.MoveStr("Nf3")
	gs.game.MoveStr("d5")

	// Simulate exd5: e4 vacated, d5 was occupied (black pawn) and stays occupied (white pawn)
	observed := gs.ExpectedOccupancy()
	observed[4][4] = false // e4 vacated (white pawn leaves)
	observed[3][3] = true  // d5 stays occupied (capture — white pawn replaces black pawn)

	move, err := gs.InferMove(observed)
	if err != nil {
		t.Fatalf("InferMove failed: %v", err)
	}

	if move.S1() != chess.E4 || move.S2() != chess.D5 {
		t.Errorf("expected e4d5, got %s%s", move.S1(), move.S2())
	}
}

func TestInferMoveCastle(t *testing.T) {
	// Set up position where white can castle kingside
	// 1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 — white can now O-O
	gs := NewGame(White)
	gs.game.MoveStr("e4")
	gs.game.MoveStr("e5")
	gs.game.MoveStr("Nf3")
	gs.game.MoveStr("Nc6")
	gs.game.MoveStr("Bc4")
	gs.game.MoveStr("Bc5")

	// Simulate O-O: king e1→g1, rook h1→f1
	observed := gs.ExpectedOccupancy()
	observed[7][4] = false // e1 vacated (king)
	observed[7][7] = false // h1 vacated (rook)
	observed[7][6] = true  // g1 occupied (king)
	observed[7][5] = true  // f1 occupied (rook)

	move, err := gs.InferMove(observed)
	if err != nil {
		t.Fatalf("InferMove failed: %v", err)
	}

	if !move.HasTag(chess.KingSideCastle) {
		t.Errorf("expected kingside castle, got %s%s", move.S1(), move.S2())
	}
}
