//go:build windows

package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPixelSaveRejectsWindowsJunction(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(svc.Config().WorkspaceDir, "junction")
	// Paths are fixed test-owned temporary directories, passed as separate args.
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v %s", err, out)
	}
	defer os.Remove(link)
	body, _ := json.Marshal(map[string]string{"path": "junction/escape.png", "data": base64.StdEncoding.EncodeToString(desktopAuditPNG(t))})
	r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(body))
	r.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	handlePixelSave(s)(w, r)
	if w.Code < 400 {
		t.Fatal("junction accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.png")); !os.IsNotExist(err) {
		t.Fatal("outside file created")
	}
}
