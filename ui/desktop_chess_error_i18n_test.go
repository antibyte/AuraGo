package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopChessErrorI18n(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/chess.js")
	for _, want := range []string{
		"function formatOpponentError(state, err)",
		"err.chessCode = code",
		"'desktop.chess_agent_unavailable'",
		"'desktop.chess_agent_no_move'",
		"'desktop.chess_engine_unavailable'",
		"'desktop.chess_engine_no_move'",
		"'desktop.chess_engine_worker_failed'",
		"'desktop.chess_engine_timeout'",
		"'desktop.chess_engine_illegal'",
		"'desktop.chess_opponent_illegal'",
		"'desktop.chess_move_failed'",
		"const message = formatOpponentError(state, err)",
		"notify(state, message)",
		"setStatus(state, message)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("chess error i18n missing marker %q", want)
		}
	}
	if strings.Contains(app, "notify(state, (err && err.message)") {
		t.Fatal("chess notify must not show raw err.message")
	}

	engine := readDesktopAssetText(t, "js/desktop/apps/chess-engine.js")
	for _, want := range []string{
		"chessErr('engine_no_move'",
		"chessErr('engine_worker_failed'",
		"chessErr('engine_timeout'",
		"chessErr('engine_illegal'",
	} {
		if !strings.Contains(engine, want) {
			t.Fatalf("chess engine error i18n missing marker %q", want)
		}
	}
	if strings.Contains(engine, "event.message") {
		t.Fatal("chess engine must not leak worker event.message into UI errors")
	}

	agent := readDesktopAssetText(t, "js/desktop/apps/chess-agent.js")
	if !strings.Contains(agent, "err.chessCode = 'agent_no_move'") {
		t.Fatal("chess agent must tag missing moves with chessCode")
	}

	keys := []string{
		"desktop.chess_agent_unavailable",
		"desktop.chess_agent_no_move",
		"desktop.chess_engine_unavailable",
		"desktop.chess_engine_no_move",
		"desktop.chess_engine_worker_failed",
		"desktop.chess_engine_timeout",
		"desktop.chess_engine_illegal",
		"desktop.chess_opponent_illegal",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.chess_engine_timeout"] == "The chess engine timed out." {
			t.Fatalf("%s must not copy the English chess engine timeout", path)
		}
		if lang == "fr" && values["desktop.chess_agent_unavailable"] == "Agent is not available." {
			t.Fatalf("%s must not copy the English chess agent unavailable string", path)
		}
	}
}
