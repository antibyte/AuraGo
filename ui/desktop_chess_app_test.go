package ui

import (
	"strings"
	"testing"
)

func TestDesktopChessAppAssetsAreLocalAndLazy(t *testing.T) {
	t.Parallel()

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'chess'",
		"'/css/cm-chessboard.css'",
		"'/css/desktop-app-chess.css'",
		"'/js/desktop/apps/chess-engine.js'",
		"'/js/desktop/apps/chess-agent.js'",
		"'/js/desktop/apps/chess.js'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop chess lazy asset registry missing marker %q", want)
		}
	}

	app := readEmbeddedText(t, "js/desktop/apps/chess.js")
	for _, want := range []string{
		"import('/js/vendor/chess-vendor.esm.js')",
		"createChessEngine",
		"createChessAgentClient",
		"window.ChessApp = { render, dispose }",
		"requestPromotionChoice",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("desktop chess app missing marker %q", want)
		}
	}

	engine := readEmbeddedText(t, "js/desktop/apps/chess-engine.js")
	for _, want := range []string{
		"'/js/vendor/stockfish/stockfish-18-lite-single.js'",
		"new Worker(workerUrl)",
		"postMessage('uci')",
		"postMessage('go depth '",
	} {
		if !strings.Contains(engine, want) {
			t.Fatalf("desktop chess engine missing marker %q", want)
		}
	}

	agent := readEmbeddedText(t, "js/desktop/apps/chess-agent.js")
	for _, want := range []string{
		"'/api/desktop/chess/agent-move'",
		"legal_moves",
		"player_color",
	} {
		if !strings.Contains(agent, want) {
			t.Fatalf("desktop chess agent client missing marker %q", want)
		}
	}
}

func TestDesktopChessAppFilesStaySmall(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/desktop/apps/chess.js",
		"js/desktop/apps/chess-engine.js",
		"js/desktop/apps/chess-agent.js",
	} {
		source := readEmbeddedText(t, path)
		lines := strings.Count(strings.ReplaceAll(source, "\r\n", "\n"), "\n") + 1
		if lines > 1100 {
			t.Fatalf("%s has %d lines, want at most 1100", path, lines)
		}
	}
}
