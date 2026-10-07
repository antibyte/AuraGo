package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/desktop"
)

func desktopAuditPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPixelSaveRejectsOversizeBeforeReplacingOriginal(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.MaxFileSizeMB = 1
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := desktopAuditPNG(t)
	if err := svc.WriteFileBytes(context.Background(), "Pictures/limit.png", original, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"path": "Pictures/limit.png", "data": base64.StdEncoding.EncodeToString(make([]byte, 2<<20))})
	r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(body))
	r.Header.Set("If-Match", desktop.NoteVersion(original))
	w := httptest.NewRecorder()
	handlePixelSave(s)(w, r)
	if w.Code != 413 {
		t.Fatalf("size limit: %d %s", w.Code, w.Body.String())
	}
	got, _, err := svc.ReadFileBytes(context.Background(), "Pictures/limit.png")
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("original changed")
	}
}

func TestDesktopTruncatedUploadPreservesObservedFile(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(context.Background(), "Documents/upload.txt", "original", desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("path", "Documents")
	part, err := writer.CreateFormFile("file", "upload.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("partial"))
	// Deliberately omit the terminating multipart boundary, as on disconnect.
	r := httptest.NewRequest("POST", "/api/desktop/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("If-Match", desktop.NoteVersion([]byte("original")))
	w := httptest.NewRecorder()
	handleDesktopUpload(s)(w, r)
	if w.Code < 400 {
		t.Fatal("accepted truncated upload")
	}
	got, _, err := svc.ReadFileBytes(context.Background(), "Documents/upload.txt")
	if err != nil || string(got) != "original" {
		t.Fatal("original changed")
	}
}

func TestDesktopFileConcurrentVersionsPreserveWinner(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(context.Background(), "Documents/race.txt", "initial", desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	read := httptest.NewRecorder()
	handleDesktopFile(s)(read, httptest.NewRequest("GET", "/api/desktop/file?path=Documents/race.txt", nil))
	version := read.Header().Get("ETag")
	if version == "" {
		t.Fatal("missing read version")
	}
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for _, content := range []string{"first", "second"} {
		wg.Add(1)
		go func(content string) {
			defer wg.Done()
			<-start
			body, _ := json.Marshal(map[string]string{"path": "Documents/race.txt", "content": content})
			r := httptest.NewRequest("PUT", "/api/desktop/file", bytes.NewReader(body))
			r.Header.Set("If-Match", version)
			w := httptest.NewRecorder()
			handleDesktopFile(s)(w, r)
			results <- w
		}(content)
	}
	close(start)
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for result := range results {
		counts[result.Code]++
	}
	if counts[200] != 1 || counts[412] != 1 {
		t.Fatalf("concurrent results: %v", counts)
	}
	r := httptest.NewRequest("PUT", "/api/desktop/file", strings.NewReader(`{"path":"Documents/race.txt","content":"lost"}`))
	w := httptest.NewRecorder()
	handleDesktopFile(s)(w, r)
	if w.Code != 428 {
		t.Fatalf("missing precondition: %d", w.Code)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "Documents/race.txt"))
	if err != nil || (string(data) != "first" && string(data) != "second") {
		t.Fatalf("winner lost: %q %v", data, err)
	}
}

func TestDesktopArchiveActiveContentHeaders(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"x.html": []byte(`<script>parent.pwned=true</script>`),
		"x.htm":  []byte(`<form action="/api/desktop/file"></form>`),
		"x.js":   []byte(`window.pwned=true`), "x.mjs": []byte(`window.pwned=true`),
		"x.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="parent.pwned=true"/>`),
		"x.txt": []byte("plain text"), "x.png": desktopAuditPNG(t),
	}
	writeDesktopTestZip(t, svc, "Documents/active.zip", files)
	for name := range files {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(name+"/"+method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/api/desktop/archive/entry?path=Documents/active.zip&entry="+url.QueryEscape(name), nil)
				w := httptest.NewRecorder()
				handleDesktopArchiveEntry(s)(w, req)
				if w.Code != 200 {
					t.Fatalf("status %d: %s", w.Code, w.Body.String())
				}
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatal("missing nosniff")
				}
				switch filepath.Ext(name) {
				case ".html", ".htm", ".js", ".mjs":
					if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
						t.Fatal("executable MIME type")
					}
				case ".svg":
					csp := w.Header().Get("Content-Security-Policy")
					if !strings.Contains(csp, "sandbox;") || !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "allow-") {
						t.Fatalf("unsafe SVG CSP: %q", csp)
					}
				}
				if method == http.MethodHead && w.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
			})
		}
	}
	r := httptest.NewRequest("GET", "/api/desktop/archive/entry?path=Documents/active.zip&entry=x.txt", nil)
	r.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()
	handleDesktopArchiveEntry(s)(w, r)
	if w.Code != 206 || w.Body.String() != "plain" {
		t.Fatalf("range: %d %q", w.Code, w.Body.String())
	}
}

func TestPixelSaveUsesWorkspaceAndConditionalImageWrites(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	pngBytes := desktopAuditPNG(t)
	call := func(path string, data []byte, header string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"path": path, "format": "png", "data": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)})
		r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(body))
		if header != "" {
			r.Header.Set("If-None-Match", header)
		}
		w := httptest.NewRecorder()
		handlePixelSave(s)(w, r)
		return w
	}
	for _, path := range []string{"../escape.png", "Documents/../../escape.png", "/absolute.png", `C:\outside.png`, `\\host\share\outside.png`, "wrong.jpg", "wrong.svg"} {
		w := call(path, pngBytes, "*")
		if w.Code < 400 {
			t.Errorf("accepted invalid path/extension %q", path)
		}
	}
	if w := call("Pictures/invalid.png", []byte("not an image"), "*"); w.Code != 400 {
		t.Fatalf("invalid image: %d", w.Code)
	}
	if w := call("Pictures/test.png", pngBytes, ""); w.Code != 428 {
		t.Fatalf("missing precondition: %d", w.Code)
	}
	w := call("Pictures/test.png", pngBytes, "*")
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), s.Cfg.Directories.DataDir) || w.Header().Get("ETag") == "" {
		t.Fatal("missing version or host path exposed")
	}
	data, err := os.ReadFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Pictures", "test.png"))
	if err != nil || !bytes.Equal(data, pngBytes) {
		t.Fatalf("workspace content: %v", err)
	}
	if w := call("Pictures/test.png", pngBytes, "*"); w.Code != 412 {
		t.Fatalf("collision: %d", w.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if w := call("Pictures/readonly.png", pngBytes, "*"); w.Code != 403 || !strings.Contains(w.Body.String(), "desktop_readonly") {
		t.Fatalf("readonly: %d %s", w.Code, w.Body.String())
	}
}

func TestPixelMutationsDenyReadonlyBeforeWork(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.ReadOnly = true
	for _, handler := range []http.HandlerFunc{handlePixelGenerate(s), handlePixelEnhance(s), handlePixelRemoveBG(s), handlePixelUpscale(s)} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("POST", "/api/pixel/action", strings.NewReader(`{}`)))
		if w.Code != 403 || !strings.Contains(w.Body.String(), "desktop_readonly") {
			t.Errorf("readonly mutation: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPixelSaveRejectsLinkedDirectory(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(svc.Config().WorkspaceDir, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	body, _ := json.Marshal(map[string]string{"path": "linked/escape.png", "data": base64.StdEncoding.EncodeToString(desktopAuditPNG(t))})
	r := httptest.NewRequest("POST", "/api/pixel/save", bytes.NewReader(body))
	r.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	handlePixelSave(s)(w, r)
	if w.Code < 400 {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.png")); !os.IsNotExist(err) {
		t.Fatal("outside file created")
	}
}
