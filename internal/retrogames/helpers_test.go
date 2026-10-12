package retrogames

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(t.TempDir(), discardLogger())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func testGame(id, title, platform string, year int, downloads int64) Game {
	system := SystemFor(platform, "dosbox")
	if platform == PlatformConsole {
		system = "Atari 2600"
	}
	return Game{Identifier: id, Title: title, Year: year, Platform: platform, System: system, Emulator: "dosbox", Downloads: downloads, Tags: title}
}

func countGames(t *testing.T, st *Store, platform string) int {
	t.Helper()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM games WHERE platform = ?`, platform).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
