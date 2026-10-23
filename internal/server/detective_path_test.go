package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/detective"
)

// The configured path wins, and the legacy location next to the Game Maker database only
// survives while that file is already there (audit 4.20).
func TestDetectivePathDefaultsAndOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLite.DetectivePath = "./data/detective.db"
	if got := detectiveStorePath(cfg); got != "./data/detective.db" {
		t.Fatalf("configured path = %q", got)
	}
	cfg.SQLite.DetectivePath = "  ./custom/research.db  "
	if got := detectiveStorePath(cfg); got != "./custom/research.db" {
		t.Fatalf("padded path = %q", got)
	}
}

func TestDetectivePathKeepsLegacyLocation(t *testing.T) {
	dir := t.TempDir()
	gameMaker := filepath.Join(dir, "game_maker.db")
	legacy := filepath.Join(dir, "detective.db")
	cfg := &config.Config{}
	cfg.SQLite.GameMakerPath = gameMaker

	// Without the legacy file the service uses the data directory.
	if got := detectiveStorePath(cfg); got != "data/detective.db" {
		t.Fatalf("fresh install path = %q", got)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectiveStorePath(cfg); got != legacy {
		t.Fatalf("legacy path = %q, want %q", got, legacy)
	}
}

// A store failure becomes a stable code; the raw error stays out of the API.
func TestDetectiveInitializationErrorCodes(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{nil, ""},
		{detective.ErrSchemaTooNew, "schema_too_new"},
		{fs.ErrPermission, "permission_denied"},
		{fs.ErrNotExist, "path_missing"},
		{errString("disk I/O error"), "store_failed"},
	}
	for _, c := range cases {
		if got := detectiveInitCode(c.err); got != c.code {
			t.Errorf("%v: code %q, want %q", c.err, got, c.code)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }
