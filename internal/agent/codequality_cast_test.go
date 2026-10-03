package agent

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestCastPublishedMediaRetainsContentsWhenSourceNameIsReused(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = work
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 8080
	var paths []string
	for _, content := range []string{"first", "second"} {
		if err := os.WriteFile(filepath.Join(work, "same.mp3"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		req := chromecastArgs{LocalPath: "same.mp3"}
		if err := prepareChromecastLocalMediaURL(cfg, &req); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(req.URL)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.Join(cfg.Directories.DataDir, "cast_media", filepath.Base(u.Path)))
	}
	if paths[0] == paths[1] {
		t.Fatal("ticket path reused")
	}
	for i, want := range []string{"first", "second"} {
		got, err := os.ReadFile(paths[i])
		if err != nil || string(got) != want {
			t.Fatalf("snapshot %d=%q err=%v", i, got, err)
		}
	}
}
