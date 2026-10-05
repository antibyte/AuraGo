package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRootedDesktopContentRegular(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "/index.html"} {
		body, _, err := readRootedDesktopContent(root, name)
		if err != nil || string(body) != "ok" {
			t.Fatalf("read %q = %q, %v", name, body, err)
		}
	}
}

func TestDesktopHTMLServingContainsSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Apps", "safe"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "index.html"), []byte("outside-sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "Apps", "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/escape/index.html", nil)
	if !serveDesktopExactIndexFile(w, r, root, nil) || w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "outside-sentinel") {
		t.Fatalf("outside app served: %d %s", w.Code, w.Body.String())
	}
	if err := os.WriteFile(filepath.Join(root, "Apps", "safe", "index.html"), []byte(`<html><script src="code.js"></script></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "code.js"), []byte("outside-script-sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "code.js"), filepath.Join(root, "Apps", "safe", "code.js")); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/safe/index.html", nil)
	serveDesktopExactIndexFile(w, r, root, nil)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "outside-script-sentinel") {
		t.Fatalf("unsafe script inline: %d %s", w.Code, w.Body.String())
	}
	if err := os.Symlink(filepath.Join(root, "Apps", "safe"), filepath.Join(root, "Apps", "alias")); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/alias/index.html", nil)
	serveDesktopExactIndexFile(w, r, root, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("valid internal app link failed: %d %s", w.Code, w.Body.String())
	}
}
