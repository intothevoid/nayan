package engine

import (
	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
)

// Engine wraps a UCI chess engine (e.g. Stockfish).
type Engine struct {
	eng *uci.Engine
}

// NewEngine starts a UCI engine process. If no path is given, "stockfish"
// is used (expected to be on PATH).
func NewEngine(path ...string) (*Engine, error) {
	bin := "stockfish"
	if len(path) > 0 && path[0] != "" {
		bin = path[0]
	}

	eng, err := uci.New(bin)
	if err != nil {
		return nil, err
	}

	// Initialize UCI protocol
	if err := eng.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		eng.Close()
		return nil, err
	}

	return &Engine{eng: eng}, nil
}

// BestMove queries the engine for the best move at the given depth.
func (e *Engine) BestMove(game *chess.Game, depth int) (*chess.Move, error) {
	cmdPos := uci.CmdPosition{Position: game.Position()}
	cmdGo := uci.CmdGo{Depth: depth}

	if err := e.eng.Run(cmdPos, cmdGo); err != nil {
		return nil, err
	}

	return e.eng.SearchResults().BestMove, nil
}

// Close shuts down the engine process.
func (e *Engine) Close() {
	if e.eng != nil {
		e.eng.Close()
	}
}
