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
		"'/js/desktop/apps/chess-fx.js'",
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
		"createChessFx",
		"window.ChessApp = { render, dispose }",
		"requestPromotionChoice",
		"ResizeObserver",
		"fitBoardToShell",
		"desktop.cancel",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("desktop chess app missing marker %q", want)
		}
	}

	fx := readEmbeddedText(t, "js/desktop/apps/chess-fx.js")
	for _, want := range []string{
		"window.createChessFx",
		"window.createChessAudio",
		"window.renderChessTemplate",
		"playMoveFx",
		"showResult",
		"vd-chess-material-bar",
		"data-board-skin",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(fx, want) {
			t.Fatalf("desktop chess fx missing marker %q", want)
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

func TestDesktopChessBoardLayoutKeepsStatusVisible(t *testing.T) {
	t.Parallel()

	css := readEmbeddedText(t, "css/desktop-app-chess.css")
	for _, want := range []string{
		"grid-template-rows: auto minmax(0, 1fr) auto;",
		".vd-chess-board-shell",
		"overflow: hidden;",
		"height: min(100%, 620px);",
		".vd-chess-ribbon",
		".vd-chess-material-bar",
		"is-capture-fx",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop chess layout CSS missing marker %q", want)
		}
	}
	if strings.Contains(css, "100vh") {
		t.Fatal("desktop chess board must not size itself from the browser viewport")
	}
}

func TestDesktopChessAppFilesStaySmall(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/desktop/apps/chess.js",
		"js/desktop/apps/chess-engine.js",
		"js/desktop/apps/chess-agent.js",
		"js/desktop/apps/chess-fx.js",
	} {
		source := readEmbeddedText(t, path)
		lines := strings.Count(strings.ReplaceAll(source, "\r\n", "\n"), "\n") + 1
		if lines > 1100 {
			t.Fatalf("%s has %d lines, want at most 1100", path, lines)
		}
	}
}

func TestDesktopChessUsesThemeIcons(t *testing.T) {
	t.Parallel()

	foundation := readEmbeddedText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "chess: 'chess'") {
		t.Fatal("desktop app icon map must route chess to the chess theme icon")
	}
	for _, path := range []string{
		"img/papirus/manifest.json",
		"img/whitesur/manifest.json",
		"img/papirus/icons/chess.svg",
		"img/whitesur/icons/chess.svg",
	} {
		if _, err := Content.ReadFile(path); err != nil {
			t.Fatalf("embedded chess theme icon asset missing %s: %v", path, err)
		}
	}
	if !strings.Contains(readEmbeddedText(t, "img/papirus/manifest.json"), `"chess": "img/papirus/icons/chess.svg"`) {
		t.Fatal("Papirus manifest must expose the chess theme icon")
	}
	if !strings.Contains(readEmbeddedText(t, "img/whitesur/manifest.json"), `"chess":  "img/whitesur/icons/chess.svg"`) {
		t.Fatal("WhiteSur manifest must expose the chess theme icon")
	}
	for _, path := range []string{
		"img/papirus/icons/chess.svg",
		"img/whitesur/icons/chess.svg",
	} {
		icon := readEmbeddedText(t, path)
		if !strings.Contains(icon, "<title>Chess</title>") || !strings.Contains(icon, "chess-king") {
			t.Fatalf("chess theme icon must stay recognizable as a chess piece: %s", path)
		}
	}

	app := readEmbeddedText(t, "js/desktop/apps/chess.js")
	fx := readEmbeddedText(t, "js/desktop/apps/chess-fx.js")
	if !strings.Contains(app, "{ id: 'flip', labelKey: 'desktop.chess_flip', icon: 'refresh'") {
		t.Fatal("desktop chess flip menu must use the refresh theme icon")
	}
	for _, want := range []string{
		"data-flip>",
		"icon('refresh', 'F')",
		"icon('help', 'H')",
		"icon('stop', 'R')",
	} {
		if !strings.Contains(fx, want) {
			t.Fatalf("desktop chess template must use theme icon marker %q", want)
		}
	}
	for _, source := range []string{app, fx} {
		for _, unwanted := range []string{
			"iconMarkup('rotate'",
			"icon: 'rotate'",
		} {
			if strings.Contains(source, unwanted) {
				t.Fatalf("desktop chess must not use unavailable rotate icon key %q", unwanted)
			}
		}
	}
}
