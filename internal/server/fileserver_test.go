package server

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const rootBoundSecret = "top secret"

// linkDirForTest links link to the directory target: a symlink where the OS
// allows one, else (Windows without the symlink privilege) a junction. Both
// carry an absolute target, so in-root links exercise the resolve fallback.
func linkDirForTest(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	} else if runtime.GOOS != "windows" {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// Paths are test-owned temporary directories, passed as separate args.
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junctions unavailable: %v %s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

func writeRootBoundFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func rootBoundGet(t *testing.T, handler http.Handler, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for key, values := range header {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func rootBoundHandler(dir string) http.Handler {
	return http.StripPrefix("/files/", http.FileServer(neuteredFileSystem{rootBoundFileSystem(dir)}))
}

func expectRootBoundStatus(t *testing.T, handler http.Handler, target string, wantStatus int, wantBody string) {
	t.Helper()
	rec := rootBoundGet(t, handler, target, nil)
	body := rec.Body.String()
	if rec.Code != wantStatus {
		t.Fatalf("%s: status %d, want %d (body %q)", target, rec.Code, wantStatus, body)
	}
	if wantBody != "" && body != wantBody {
		t.Fatalf("%s: body %q, want %q", target, body, wantBody)
	}
	if strings.Contains(body, rootBoundSecret) {
		t.Fatalf("%s leaked a file outside the served directory", target)
	}
}

func TestRootBoundFileSystemRefusesEscapingSymlinkAndKeepsInRootLinks(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte(rootBoundSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "ok.txt"), []byte("fine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ws, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink("ok.txt", filepath.Join(ws, "inroot.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	srv := httptest.NewServer(http.StripPrefix("/files/", http.FileServer(neuteredFileSystem{rootBoundFileSystem(ws)})))
	defer srv.Close()

	for path, wantStatus := range map[string]int{"/files/ok.txt": 200, "/files/inroot.txt": 200, "/files/escape.txt": 404, "/files/": 404} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Fatalf("%s: status %d, want %d (body %q)", path, resp.StatusCode, wantStatus, body)
		}
		if strings.Contains(string(body), rootBoundSecret) {
			t.Fatalf("%s leaked the symlink target", path)
		}
	}
}

func TestRootBoundFileSystemServesAbsoluteInRootSymlink(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(ws, "ok.txt"), "fine")
	// os.Root refuses absolute link targets; the resolve fallback serves them
	// while they stay inside the served directory.
	if err := os.Symlink(filepath.Join(ws, "ok.txt"), filepath.Join(ws, "absinroot.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	expectRootBoundStatus(t, rootBoundHandler(ws), "/files/absinroot.txt", http.StatusOK, "fine")
}

// The resolve fallback opens the link target; the served name (and so the
// Content-Type) must still come from the requested path, as with http.Dir.
func TestRootBoundFileSystemNamesFallbackFilesAfterTheRequest(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(ws, "evil.html"), "<script>alert(1)</script>")
	if err := os.Symlink(filepath.Join(ws, "evil.html"), filepath.Join(ws, "report.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	rec := rootBoundGet(t, rootBoundHandler(ws), "/files/report.txt", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type %q, want text/plain from the requested name", ct)
	}
}

// Windows variant through a directory junction: a differently cased request
// reaches the file through the fallback, which must report the requested name.
func TestRootBoundFileSystemNamesFallbackFilesAfterTheRequestThroughJunction(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("needs a case-insensitive file system")
	}
	ws := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(ws, "inner", "report.txt"), "fine")
	linkDirForTest(t, filepath.Join(ws, "inner"), filepath.Join(ws, "docs"))
	f, err := rootBoundFileSystem(ws).Open("/docs/REPORT.TXT")
	if err != nil {
		t.Fatalf("open through junction: %v", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "REPORT.TXT" {
		t.Fatalf("served name %q, want the requested %q", info.Name(), "REPORT.TXT")
	}
}

// A link out of the served directory into an unreadable directory must not
// answer 403: that would reveal what exists outside.
func TestRootBoundFileSystemHidesUnreadableLinkTargets(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("directory modes do not restrict access on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	writeRootBoundFixture(t, filepath.Join(locked, "secret.txt"), rootBoundSecret)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	ws := t.TempDir()
	if err := os.Symlink(locked, filepath.Join(ws, "esc")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	h := rootBoundHandler(ws)
	expectRootBoundStatus(t, h, "/files/esc/secret.txt", http.StatusNotFound, "")
	expectRootBoundStatus(t, h, "/files/esc/missing.txt", http.StatusNotFound, "")
}

func TestRootBoundFileSystemFollowsDirectoryLinksOnlyInsideRoot(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(outside, "secret.txt"), rootBoundSecret)
	ws := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(ws, "inner", "ok.txt"), "fine")
	linkDirForTest(t, filepath.Join(ws, "inner"), filepath.Join(ws, "indir"))
	linkDirForTest(t, outside, filepath.Join(ws, "escdir"))

	h := rootBoundHandler(ws)
	expectRootBoundStatus(t, h, "/files/indir/ok.txt", http.StatusOK, "fine")
	expectRootBoundStatus(t, h, "/files/indir", http.StatusNotFound, "")
	expectRootBoundStatus(t, h, "/files/escdir/secret.txt", http.StatusNotFound, "")
	expectRootBoundStatus(t, h, "/files/escdir/", http.StatusNotFound, "")
}

func TestRootBoundFileSystemServesLinkedMountDirectory(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(outside, "secret.txt"), rootBoundSecret)
	base := t.TempDir()
	ws := filepath.Join(base, "ws")
	writeRootBoundFixture(t, filepath.Join(ws, "inner", "ok.txt"), "fine")
	linkDirForTest(t, filepath.Join(ws, "inner"), filepath.Join(ws, "indir"))
	linkDirForTest(t, outside, filepath.Join(ws, "escdir"))
	mount := filepath.Join(base, "mount")
	linkDirForTest(t, ws, mount)

	h := rootBoundHandler(mount)
	expectRootBoundStatus(t, h, "/files/inner/ok.txt", http.StatusOK, "fine")
	expectRootBoundStatus(t, h, "/files/indir/ok.txt", http.StatusOK, "fine")
	expectRootBoundStatus(t, h, "/files/escdir/secret.txt", http.StatusNotFound, "")
}

func TestRootBoundFileSystemServesOnlyRegularFilesInsideRoot(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(base, "secret.txt"), rootBoundSecret)
	ws := filepath.Join(base, "ws")
	writeRootBoundFixture(t, filepath.Join(ws, "ok.txt"), "fine")
	writeRootBoundFixture(t, filepath.Join(ws, ".hidden"), "dot")
	writeRootBoundFixture(t, filepath.Join(ws, "sub", "nested.txt"), "nested")

	h := rootBoundHandler(ws)
	for _, tc := range []struct {
		target string
		status int
		body   string
	}{
		{"/files/ok.txt", http.StatusOK, "fine"},
		{"/files/.hidden", http.StatusOK, "dot"},
		{"/files/sub/nested.txt", http.StatusOK, "nested"},
		{"/files/", http.StatusNotFound, ""},
		{"/files/sub", http.StatusNotFound, ""},
		{"/files/sub/", http.StatusNotFound, ""},
		{"/files/missing.txt", http.StatusNotFound, ""},
		{"/files/../secret.txt", http.StatusNotFound, ""},
		{"/files/sub/../../secret.txt", http.StatusNotFound, ""},
		{"/files/..%2fsecret.txt", http.StatusNotFound, ""},
		{"/files/..%5csecret.txt", http.StatusNotFound, ""},
		{"/files/ok.txt::$DATA", http.StatusNotFound, ""},
		{"/files/NUL", http.StatusNotFound, ""},
		{"/files/CON", http.StatusNotFound, ""},
	} {
		expectRootBoundStatus(t, h, tc.target, tc.status, tc.body)
	}
}

func TestRootBoundFileSystemServesRangeRequests(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(ws, "media.bin"), "0123456789")

	rec := rootBoundGet(t, rootBoundHandler(ws), "/files/media.bin", http.Header{"Range": {"bytes=2-5"}})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d, want 206 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "2345" {
		t.Fatalf("range body %q, want %q", got, "2345")
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 2-5/10" {
		t.Fatalf("Content-Range %q, want %q", got, "bytes 2-5/10")
	}
}

func TestRootBoundFileSystemServesDirectoryCreatedOrRecreatedLater(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "later")
	h := rootBoundHandler(dir)

	expectRootBoundStatus(t, h, "/files/a.txt", http.StatusNotFound, "")
	writeRootBoundFixture(t, filepath.Join(dir, "a.txt"), "one")
	expectRootBoundStatus(t, h, "/files/a.txt", http.StatusOK, "one")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	expectRootBoundStatus(t, h, "/files/a.txt", http.StatusNotFound, "")
	writeRootBoundFixture(t, filepath.Join(dir, "a.txt"), "two")
	expectRootBoundStatus(t, h, "/files/a.txt", http.StatusOK, "two")
}

type modeTestFileSystem struct {
	mode   fs.FileMode
	closed *bool
}

func (m modeTestFileSystem) Open(string) (http.File, error) {
	return &modeTestFile{info: modeTestInfo{mode: m.mode}, closed: m.closed}, nil
}

type modeTestFile struct {
	info   modeTestInfo
	closed *bool
}

func (f *modeTestFile) Close() error                       { *f.closed = true; return nil }
func (f *modeTestFile) Read([]byte) (int, error)           { return 0, io.EOF }
func (f *modeTestFile) Seek(int64, int) (int64, error)     { return 0, nil }
func (f *modeTestFile) Readdir(int) ([]fs.FileInfo, error) { return nil, nil }
func (f *modeTestFile) Stat() (fs.FileInfo, error)         { return f.info, nil }

type modeTestInfo struct{ mode fs.FileMode }

func (i modeTestInfo) Name() string       { return "entry" }
func (i modeTestInfo) Size() int64        { return 0 }
func (i modeTestInfo) Mode() fs.FileMode  { return i.mode }
func (i modeTestInfo) ModTime() time.Time { return time.Time{} }
func (i modeTestInfo) IsDir() bool        { return i.mode.IsDir() }
func (i modeTestInfo) Sys() any           { return nil }

func TestNeuteredFileSystemServesRegularFilesOnly(t *testing.T) {
	t.Parallel()
	served := []fs.FileMode{0o644}
	refused := []fs.FileMode{fs.ModeDir | 0o755, fs.ModeNamedPipe, fs.ModeDevice | fs.ModeCharDevice, fs.ModeDevice, fs.ModeSocket, fs.ModeSymlink}
	if runtime.GOOS == "windows" {
		// Cloud placeholders and WOF-compressed files carry non-link reparse
		// tags that Go reports as irregular; http.Dir served them.
		served = append(served, fs.ModeIrregular|0o644)
	} else {
		refused = append(refused, fs.ModeIrregular)
	}
	for _, mode := range served {
		closed := false
		f, err := neuteredFileSystem{modeTestFileSystem{mode: mode, closed: &closed}}.Open("/file")
		if err != nil {
			t.Fatalf("mode %v refused: %v", mode, err)
		}
		if closed {
			t.Fatalf("mode %v: served file was closed", mode)
		}
		f.Close()
	}
	for _, mode := range refused {
		closed := false
		f, err := neuteredFileSystem{modeTestFileSystem{mode: mode, closed: &closed}}.Open("/entry")
		if err == nil {
			f.Close()
			t.Fatalf("mode %v served, want not found", mode)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("mode %v: error %v, want not found", mode, err)
		}
		if !closed {
			t.Fatalf("mode %v: refused entry was not closed", mode)
		}
	}
}

// TestUIRoutesFilesMountsAreRootBound checks every /files/ mount except the
// authenticated desktop one: regular files serve, links leaving the directory
// do not.
func TestUIRoutesFilesMountsAreRootBound(t *testing.T) {
	outside := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(outside, "secret.txt"), rootBoundSecret)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.WorkspaceDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	if _, err := s.registerUIRoutes(mux, make(chan struct{})); err != nil {
		t.Fatalf("registerUIRoutes: %v", err)
	}
	downloadsDir, err := tools.ResolveVideoDownloadDir(cfg)
	if err != nil {
		downloadsDir = filepath.Join(cfg.Directories.DataDir, "downloads")
	}

	for _, mount := range []struct{ prefix, dir string }{
		{"/files/documents/", filepath.Join(cfg.Directories.DataDir, "documents")},
		{"/files/audio/", filepath.Join(cfg.Directories.DataDir, "audio")},
		{"/files/generated_images/", filepath.Join(cfg.Directories.DataDir, "generated_images")},
		{"/files/generated_videos/", filepath.Join(cfg.Directories.DataDir, "generated_videos")},
		{"/files/launchpad_icons/", filepath.Join(cfg.Directories.DataDir, "launchpad_icons")},
		{"/files/frigate_media/", filepath.Join(cfg.Directories.DataDir, "frigate_media")},
		{"/files/go2rtc/snapshots/", filepath.Join(cfg.Directories.DataDir, "go2rtc", "snapshots")},
		{"/files/3d_printer_media/", filepath.Join(cfg.Directories.DataDir, "3d_printer_media")},
		{"/files/downloads/", downloadsDir},
		{"/files/", cfg.Directories.WorkspaceDir},
	} {
		if _, err := os.Stat(mount.dir); err != nil {
			t.Fatalf("%s: mount directory not created at startup: %v", mount.prefix, err)
		}
		writeRootBoundFixture(t, filepath.Join(mount.dir, "ok.txt"), "fine")
		linkDirForTest(t, outside, filepath.Join(mount.dir, "escdir"))
		expectRootBoundStatus(t, mux, mount.prefix+"ok.txt", http.StatusOK, "fine")
		expectRootBoundStatus(t, mux, mount.prefix+"escdir/secret.txt", http.StatusNotFound, "")
	}
}

func TestServeDesktopExactIndexFileRefusesIndexOutsideDesktopDir(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(outside, "index.html"), "<html><body>"+rootBoundSecret+"</body></html>")
	writeRootBoundFixture(t, filepath.Join(outside, "lib.js"), "window.leak='"+rootBoundSecret+"';")
	desktopDir := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(desktopDir, "Apps", "app", "index.html"), `<html><head><script src="vendor/lib.js"></script></head><body>OK</body></html>`)
	linkDirForTest(t, outside, filepath.Join(desktopDir, "Apps", "evil"))
	linkDirForTest(t, outside, filepath.Join(desktopDir, "Apps", "app", "vendor"))

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/evil/index.html", nil)
	rec := httptest.NewRecorder()
	if !serveDesktopExactIndexFile(rec, req, desktopDir, nil) {
		t.Fatal("index request was not handled")
	}
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), rootBoundSecret) {
		t.Fatalf("index outside desktop dir: status %d body %q, want 404", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/app/index.html", nil)
	rec = httptest.NewRecorder()
	if !serveDesktopExactIndexFile(rec, req, desktopDir, nil) {
		t.Fatal("regular index was not served")
	}
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "OK") {
		t.Fatalf("regular index: status %d body %q, want 200", rec.Code, body)
	}
	if strings.Contains(body, rootBoundSecret) {
		t.Fatalf("sibling script outside desktop dir was inlined: %q", body)
	}
}

// Sibling scripts behind links that stay inside the desktop directory (the
// desktop symlink API writes relative ones) are still inlined.
func TestServeDesktopExactIndexFileInlinesSiblingBehindInDesktopLink(t *testing.T) {
	t.Parallel()
	desktopDir := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(desktopDir, "shared", "lib.js"), "window.SHARED_LIB=1;")
	writeRootBoundFixture(t, filepath.Join(desktopDir, "Apps", "app", "index.html"), `<html><head><script src="vendor/lib.js"></script><script src="rel.js"></script></head><body>OK</body></html>`)
	linkDirForTest(t, filepath.Join(desktopDir, "shared"), filepath.Join(desktopDir, "Apps", "app", "vendor"))

	serve := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/files/desktop/Apps/app/index.html", nil)
		rec := httptest.NewRecorder()
		if !serveDesktopExactIndexFile(rec, req, desktopDir, nil) || rec.Code != http.StatusOK {
			t.Fatalf("index not served: status %d", rec.Code)
		}
		return rec.Body.String()
	}
	if body := serve(); strings.Contains(body, `src="vendor/lib.js`) || !strings.Contains(body, "window.SHARED_LIB=1;") {
		t.Fatalf("sibling behind directory link was not inlined: %q", body)
	}

	if err := os.Symlink(filepath.Join("..", "..", "shared", "lib.js"), filepath.Join(desktopDir, "Apps", "app", "rel.js")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if body := serve(); strings.Contains(body, `src="rel.js`) {
		t.Fatalf("sibling behind relative file link was not inlined: %q", body)
	}
}

func TestDesktopWidgetAutoResizeRefusesWidgetOutsideDesktopDir(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeRootBoundFixture(t, filepath.Join(outside, "weather.html"), "<html><body>"+rootBoundSecret+"</body></html>")
	desktopDir := t.TempDir()
	linkDirForTest(t, outside, filepath.Join(desktopDir, "Widgets"))

	req := httptest.NewRequest(http.MethodGet, "/files/desktop/Widgets/weather.html?widget_id=weather", nil)
	rec := httptest.NewRecorder()
	if serveDesktopWidgetAutoResizeHTML(rec, req, desktopDir, nil) {
		t.Fatalf("widget outside desktop dir was served: status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), rootBoundSecret) {
		t.Fatalf("widget outside desktop dir leaked: %q", rec.Body.String())
	}
}
