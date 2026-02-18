package ui

import (
	"embed"

	"fyne.io/fyne/v2"
)

//go:embed pieces/*.svg
var pieceFS embed.FS

// PieceType represents a chess piece for display on the board widget.
type PieceType int8

const (
	NoPieceType PieceType = iota
	WhiteKing
	WhiteQueen
	WhiteRook
	WhiteBishop
	WhiteKnight
	WhitePawn
	BlackKing
	BlackQueen
	BlackRook
	BlackBishop
	BlackKnight
	BlackPawn
)

// pieceFiles maps PieceType to the embedded SVG filename.
var pieceFiles = map[PieceType]string{
	WhiteKing:   "pieces/wK.svg",
	WhiteQueen:  "pieces/wQ.svg",
	WhiteRook:   "pieces/wR.svg",
	WhiteBishop: "pieces/wB.svg",
	WhiteKnight: "pieces/wN.svg",
	WhitePawn:   "pieces/wP.svg",
	BlackKing:   "pieces/bK.svg",
	BlackQueen:  "pieces/bQ.svg",
	BlackRook:   "pieces/bR.svg",
	BlackBishop: "pieces/bB.svg",
	BlackKnight: "pieces/bN.svg",
	BlackPawn:   "pieces/bP.svg",
}

// pieceResources caches loaded Fyne resources.
var pieceResources = map[PieceType]fyne.Resource{}

// PieceResource returns the Fyne resource for a given piece type.
// Returns nil for NoPieceType.
func PieceResource(pt PieceType) fyne.Resource {
	if pt == NoPieceType {
		return nil
	}

	if r, ok := pieceResources[pt]; ok {
		return r
	}

	filename, ok := pieceFiles[pt]
	if !ok {
		return nil
	}

	data, err := pieceFS.ReadFile(filename)
	if err != nil {
		return nil
	}

	r := fyne.NewStaticResource(filename, data)
	pieceResources[pt] = r
	return r
}

// StartingPosition returns the standard chess starting position as a piece grid.
func StartingPosition() [8][8]PieceType {
	return [8][8]PieceType{
		// Row 0 = rank 8 (black back rank)
		{BlackRook, BlackKnight, BlackBishop, BlackQueen, BlackKing, BlackBishop, BlackKnight, BlackRook},
		// Row 1 = rank 7 (black pawns)
		{BlackPawn, BlackPawn, BlackPawn, BlackPawn, BlackPawn, BlackPawn, BlackPawn, BlackPawn},
		// Rows 2-5 = empty
		{}, {}, {}, {},
		// Row 6 = rank 2 (white pawns)
		{WhitePawn, WhitePawn, WhitePawn, WhitePawn, WhitePawn, WhitePawn, WhitePawn, WhitePawn},
		// Row 7 = rank 1 (white back rank)
		{WhiteRook, WhiteKnight, WhiteBishop, WhiteQueen, WhiteKing, WhiteBishop, WhiteKnight, WhiteRook},
	}
}
