package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
		"'/js/desktop/apps/chess-commentary.js'",
		"'/js/desktop/apps/chess-fx.js'",
		"'/js/desktop/apps/chess-table.js'",
		"'/js/desktop/apps/chess-3d-stage.js'",
		"'/js/desktop/apps/chess-3d.js'",
		"'/js/desktop/apps/chess.js'",
		"'/js/vendor/three.min.js'",
		"'/js/vendor/GLTFLoader.min.js'",
		"'/js/vendor/OrbitControls.min.js'",
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
		/* the coordinates live in the wood rim and the plates paint the clocks */
		"showCoordinates: false",
		"window.ChessTable?.mount(state)",
		"window.ChessTable?.update(state)",
		"state.refs.root.classList.toggle('is-flipped', black)",
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
		/* the table around the board and the lit playing surface */
		"data-plate=\"' + side + '\"",
		"data-plate-clock",
		"data-rim-coords",
		"vd-chess-surface",
		"vd-chess-keys",
		/* six skins: the vendor four plus marble and slate */
		"'marble', 'slate'",
		"state.refs.root.dataset.chessSkin = skin",
	} {
		if !strings.Contains(fx, want) {
			t.Fatalf("desktop chess fx missing marker %q", want)
		}
	}

	table := readEmbeddedText(t, "js/desktop/apps/chess-table.js")
	for _, want := range []string{
		"window.ChessTable = { mount, update }",
		/* the plate on top belongs to the side that is not at the bottom */
		"renderPlate(ui.top, state, blackAtBottom ? 'w' : 'b')",
		"renderPlate(ui.bottom, state, blackAtBottom ? 'b' : 'w')",
		/* the rim is a 10x10 grid: one rim cell around eight board cells */
		"span.style.gridArea = row + ' / ' + col;",
		"desktop.chess_you",
		"desktop.chess_opponent_computer",
	} {
		if !strings.Contains(table, want) {
			t.Fatalf("desktop chess table missing marker %q", want)
		}
	}

	stage := readEmbeddedText(t, "js/desktop/apps/chess-3d-stage.js")
	for _, want := range []string{
		"window.ChessStage3D",
		"createGrainVariants",
		"createStudioEnvironment",
		"createCoordTexture",
		"createPoolTexture",
	} {
		if !strings.Contains(stage, want) {
			t.Fatalf("desktop chess 3D stage missing marker %q", want)
		}
	}

	engine := readEmbeddedText(t, "js/desktop/apps/chess-engine.js")
	for _, want := range []string{
		"'/js/vendor/stockfish/stockfish-19-lite-single.js'",
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

	view3d := readEmbeddedText(t, "js/desktop/apps/chess-3d.js")
	for _, want := range []string{
		"window.createChess3D",
		"window.ChessView3D",
		"'/3d/chess/v1/'",
		"GLTFLoader",
		"OrbitControls",
		/* A capture keeps the mover's code, and a taken piece shares the target
		   square with its captor, so reconciliation must retire leftovers by
		   entry instead of by square key. */
		"const live = new Set();",
		"if (!live.has(entry)) removePiece(entry);",
		/* the lamp pool on the floor and the camera glide that starts after the first frame */
		"createPoolTexture",
		"createGrainVariants",
		"state.introPending = false",
		"!window.ChessStage3D",
	} {
		if !strings.Contains(view3d, want) {
			t.Fatalf("desktop chess 3D board missing marker %q", want)
		}
	}
}

func TestDesktopChess3DAssetPackIsComplete(t *testing.T) {
	t.Parallel()

	manifestRaw, err := Content.ReadFile("3d/chess/v1/manifest.json")
	if err != nil {
		t.Fatalf("chess 3D manifest missing: %v", err)
	}
	var manifest struct {
		Units       string  `json:"units"`
		UpAxis      string  `json:"up_axis"`
		SquareSize  float64 `json:"square_size"`
		License     string  `json:"license"`
		BudgetBytes int64   `json:"budget_bytes"`
		Pieces      map[string]struct {
			File   string  `json:"file"`
			Bytes  int64   `json:"bytes"`
			SHA256 string  `json:"sha256"`
			Height float64 `json:"height"`
			Radius float64 `json:"radius"`
		} `json:"pieces"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("chess 3D manifest is not valid JSON: %v", err)
	}
	if manifest.Units != "metres" || manifest.UpAxis != "Y" {
		t.Fatalf("chess 3D manifest must be metric Y-up, got %q/%q", manifest.Units, manifest.UpAxis)
	}
	if manifest.License != "MIT" {
		t.Fatalf("chess 3D manifest must keep the MIT license, got %q", manifest.License)
	}
	if manifest.SquareSize <= 0 {
		t.Fatalf("chess 3D manifest must declare the board square size, got %v", manifest.SquareSize)
	}
	want := []string{"pawn", "knight", "bishop", "rook", "queen", "king"}
	if len(manifest.Pieces) != len(want) {
		t.Fatalf("chess 3D manifest must describe %d pieces, got %d", len(want), len(manifest.Pieces))
	}
	var total int64
	for _, name := range want {
		entry, ok := manifest.Pieces[name]
		if !ok {
			t.Fatalf("chess 3D manifest missing piece %q", name)
		}
		if entry.File != name+".glb" {
			t.Fatalf("chess 3D piece %q must use file %q, got %q", name, name+".glb", entry.File)
		}
		if entry.Height <= 0 || entry.Radius <= 0 {
			t.Fatalf("chess 3D piece %q must declare height and radius", name)
		}
		if entry.Radius*2 >= manifest.SquareSize {
			t.Fatalf("chess 3D piece %q base must stay inside its square", name)
		}
		raw, err := Content.ReadFile("3d/chess/v1/" + entry.File)
		if err != nil {
			t.Fatalf("chess 3D piece file missing %s: %v", entry.File, err)
		}
		if int64(len(raw)) != entry.Bytes {
			t.Fatalf("chess 3D piece %q size mismatch: manifest %d, file %d", name, entry.Bytes, len(raw))
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			t.Fatalf("chess 3D piece %q hash mismatch", name)
		}
		if !bytes.HasPrefix(raw, []byte("glTF")) {
			t.Fatalf("chess 3D piece %q is not a glTF binary", name)
		}
		total += entry.Bytes
	}
	if total > manifest.BudgetBytes {
		t.Fatalf("chess 3D pack is %d bytes, over the %d byte budget", total, manifest.BudgetBytes)
	}
	if _, err := Content.ReadFile("3d/chess/v1/LICENSE.txt"); err != nil {
		t.Fatalf("chess 3D pack must ship its license: %v", err)
	}
}

func TestDesktopChessBoardLayoutKeepsStatusVisible(t *testing.T) {
	t.Parallel()

	css := readEmbeddedText(t, "css/desktop-app-chess.css")
	for _, want := range []string{
		"grid-template-rows: auto minmax(0, 1fr) auto;",
		".vd-chess-board-shell",
		".vd-chess-board-frame",
		"overflow: hidden;",
		/* the board fits one square box that never leaves the pane height */
		"container-type: size;",
		"width: min(100%, var(--chess-column), 100cqh);",
		".vd-chess-pane-head",
		/* the turn status is a pill in the head row, the remark a thin strip
		   under the board - both keep their height off the board */
		".vd-chess-status",
		".vd-chess-comment",
		/* a wrapped head row costs board height and starves the board in a loop */
		"flex-wrap: nowrap;",
		".vd-chess-material-bar",
		"is-capture-fx",
		"prefers-reduced-motion",
		/* cm-chessboard context must not own the wood-rim padding */
		"cm-chessboard sizes its SVG to context.clientWidth",
		/* the app sizes the frame through a variable, the 3D scene fills the area */
		".vd-chess-board-frame.is-sized",
		".vd-chess.is-3d .vd-chess-board-frame",
		/* the sidebar folds its settings away and scrolls instead of clipping */
		".vd-chess-settings",
		".vd-chess-settings-grid",
		"overflow-y: auto;",
		/* panels carry their own display rule and need the hidden reset */
		".vd-chess-moves[hidden]",
		/* the table stacks the plates around the board and the surface is lit above the squares */
		".vd-chess-table",
		".vd-chess-plate",
		".vd-chess-rim-coords",
		".vd-chess-surface",
		/* the stacked layout gives the sidebar a share of the window height, never the viewport */
		"grid-template-rows: minmax(0, 1fr) clamp(150px, 34%, 280px);",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop chess layout CSS missing marker %q", want)
		}
	}
	if strings.Contains(css, "100vh") {
		t.Fatal("desktop chess board must not size itself from the browser viewport")
	}

	fx := readEmbeddedText(t, "js/desktop/apps/chess-fx.js")
	if !strings.Contains(fx, `data-chess-board-frame`) || !strings.Contains(fx, `data-chess-board`) {
		t.Fatal("chess template must keep a separate board frame and cm-chessboard context")
	}
	app := readEmbeddedText(t, "js/desktop/apps/chess.js")
	if !strings.Contains(app, "boardFrame:") || !strings.Contains(app, "handleResize") {
		t.Fatal("chess app must size the frame and reflow the board view")
	}
	if !strings.Contains(app, "state.refitBoard") || !strings.Contains(app, "--chess-frame-size") {
		t.Fatal("chess app must let the 3D view refit the board frame")
	}
	view3d := readEmbeddedText(t, "js/desktop/apps/chess-3d.js")
	for _, want := range []string{
		/* the camera fits the projected board, not its circumscribed sphere */
		"fitCorners",
		"applyDefaultAngle",
		"classList.toggle('is-3d'",
		/* The brass fillet must stay in the board plane: rotateY would tip
		   this 4-segment ring into a giant square crossing the scene. */
		"filletRing.rotateZ(Math.PI / 4);",
	} {
		if !strings.Contains(view3d, want) {
			t.Fatalf("desktop chess 3D board missing marker %q", want)
		}
	}
	if strings.Contains(view3d, "filletRing.rotateY") {
		t.Fatal("desktop chess 3D fillet must rotate inside its plane, not tip out of the board")
	}
}

func TestDesktopChessAppFilesStaySmall(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/desktop/apps/chess.js",
		"js/desktop/apps/chess-engine.js",
		"js/desktop/apps/chess-agent.js",
		"js/desktop/apps/chess-commentary.js",
		"js/desktop/apps/chess-fx.js",
		"js/desktop/apps/chess-table.js",
		"js/desktop/apps/chess-3d-stage.js",
		"js/desktop/apps/chess-3d.js",
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

func TestDesktopChessCommentaryAndPolishContracts(t *testing.T) {
	t.Parallel()

	commentary := readEmbeddedText(t, "js/desktop/apps/chess-commentary.js")
	for _, want := range []string{
		"'/api/desktop/chess/commentary'",
		"'aurago.desktop.chess.commentary'",
		"window.createChessCommentary",
		"AbortController",
	} {
		if !strings.Contains(commentary, want) {
			t.Fatalf("chess commentary module missing marker %q", want)
		}
	}

	fx := readEmbeddedText(t, "js/desktop/apps/chess-fx.js")
	for _, want := range []string{
		"data-sound-toggle",
		"data-commentary-toggle",
		"data-chess-reset-cam",
		"data-copy-pgn",
		"data-copy-fen",
		"playVoice",
		"stopVoice",
		"window.ChessChrome",
		"syncCommentaryUI",
		"playMoveSound",
	} {
		if !strings.Contains(fx, want) {
			t.Fatalf("chess fx/template missing polish marker %q", want)
		}
	}

	newKeys := []string{
		"desktop.chess_sound",
		"desktop.chess_sound_on",
		"desktop.chess_sound_off",
		"desktop.chess_commentary",
		"desktop.chess_commentary_hint",
		"desktop.chess_commentary_on",
		"desktop.chess_commentary_off",
		"desktop.chess_commentary_speaking",
		"desktop.chess_commentary_no_audio",
		"desktop.chess_reset_camera",
		"desktop.chess_copy_pgn",
		"desktop.chess_copy_fen",
		"desktop.chess_copied",
		"desktop.chess_phase",
		"desktop.chess_phase_opening",
		"desktop.chess_phase_middlegame",
		"desktop.chess_phase_endgame",
		"desktop.chess_you",
		"desktop.chess_skin_marble",
		"desktop.chess_skin_slate",
		"desktop.chess_shortcuts",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		raw, err := Content.ReadFile("lang/desktop/" + lang + ".json")
		if err != nil {
			t.Fatalf("read %s locale: %v", lang, err)
		}
		var dict map[string]string
		if err := json.Unmarshal(raw, &dict); err != nil {
			t.Fatalf("parse %s locale: %v", lang, err)
		}
		for _, key := range newKeys {
			if strings.TrimSpace(dict[key]) == "" {
				t.Fatalf("locale %s missing %s", lang, key)
			}
		}
	}
}
