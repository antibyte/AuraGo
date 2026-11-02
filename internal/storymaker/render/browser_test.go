package render

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "chrome.exe"
	}
	return "chrome"
}

func fakeBrowserFile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, exeName())
	if err := os.WriteFile(p, []byte("not a real browser"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestBrowsers(t *testing.T) (*Browsers, *atomic.Int32) {
	t.Helper()
	b := NewBrowsers(t.TempDir())
	probes := &atomic.Int32{}
	b.launchProbe = func(ctx context.Context, bin string) (string, error) {
		probes.Add(1)
		if strings.Contains(filepath.Base(bin), "broken") {
			return "", errors.New("exec format error")
		}
		return "FakeChrome/1.0", nil
	}
	b.lookPath = func() (string, bool) { return "", false }
	b.pins = map[string]PinnedChromium{}
	return b, probes
}

func TestBrowserResolutionOrder(t *testing.T) {
	b, _ := newTestBrowsers(t)
	configured := fakeBrowserFile(t, t.TempDir())
	system := fakeBrowserFile(t, t.TempDir())
	b.lookPath = func() (string, bool) { return system, true }

	st, bin := b.Ensure(context.Background(), configured, nil)
	if !st.Ready || st.Source != SourceConfig || bin != configured || st.Version != "FakeChrome/1.0" {
		t.Fatalf("configured: %+v %q", st, bin)
	}
	st, bin = b.Ensure(context.Background(), "", nil)
	if !st.Ready || st.Source != SourceSystem || bin != system {
		t.Fatalf("system: %+v %q", st, bin)
	}
	// A configured path that does not exist is reported, not replaced by the system browser.
	st, bin = b.Ensure(context.Background(), filepath.Join(t.TempDir(), "nope"), nil)
	if st.Ready || st.Source != SourceConfig || bin != "" || !strings.Contains(st.Issue, "browser_path") {
		t.Fatalf("missing configured: %+v %q", st, bin)
	}
	// So is a configured browser that does not start.
	broken := filepath.Join(t.TempDir(), "broken-chrome")
	if err := os.WriteFile(broken, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, _ = b.Ensure(context.Background(), broken, nil)
	if st.Ready || st.Source != SourceConfig || !strings.Contains(st.Issue, "exec format error") {
		t.Fatalf("broken configured: %+v", st)
	}
}

func TestBrowserMissingAndSystemProbeFailure(t *testing.T) {
	b, _ := newTestBrowsers(t)
	st := b.Status(context.Background(), "")
	if st.Ready || st.Source != SourceMissing || st.Issue == "" || st.CanDownload {
		t.Fatalf("nothing available (no pin): %+v", st)
	}
	broken := filepath.Join(t.TempDir(), "broken-chrome")
	if err := os.WriteFile(broken, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	b.lookPath = func() (string, bool) { return broken, true }
	st = b.Status(context.Background(), "")
	if st.Ready || st.Source != SourceSystem || !strings.Contains(st.Issue, "exec format error") {
		t.Fatalf("a system browser that does not start: %+v", st)
	}
}

func TestBrowserProbeIsCached(t *testing.T) {
	b, probes := newTestBrowsers(t)
	system := fakeBrowserFile(t, t.TempDir())
	b.lookPath = func() (string, bool) { return system, true }
	for i := 0; i < 5; i++ {
		if st := b.Status(context.Background(), ""); !st.Ready {
			t.Fatalf("status %d: %+v", i, st)
		}
	}
	if probes.Load() != 1 {
		t.Fatalf("%d probes for 5 status requests", probes.Load())
	}
	b.ForgetProbes()
	b.Status(context.Background(), "")
	if probes.Load() != 2 {
		t.Fatalf("%d probes after ForgetProbes", probes.Load())
	}
}

// fakeChromiumZip builds a snapshot-shaped archive: chrome-x/<exe>, a data file and a folder.
func fakeChromiumZip(t *testing.T, topDir string, extra func(w *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add := func(name string, body string, mode os.FileMode) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(body))
	}
	add(topDir+"/", "", os.ModeDir|0o755)
	add(topDir+"/"+exeName(), "MZ fake browser", 0o755)
	add(topDir+"/resources/data.pak", "data", 0o644)
	if extra != nil {
		extra(w)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pinFor(url string, archive []byte) PinnedChromium {
	sum := sha256.Sum256(archive)
	return PinnedChromium{URL: url, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(archive)), Dir: "chrome-x", Exe: exeName()}
}

func serveArchive(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPinnedChromiumDownloadVerifiesAndInstalls(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := serveArchive(t, archive)
	b, probes := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}

	// A status request never downloads.
	st := b.Status(context.Background(), "")
	if st.Ready || st.Source != SourcePinned || !st.CanDownload || !strings.Contains(st.Issue, "first export") {
		t.Fatalf("status before the download: %+v", st)
	}
	if _, err := os.Stat(b.PinnedDir()); !os.IsNotExist(err) {
		t.Fatal("a status request created the install folder")
	}

	var last int64
	st, bin := b.Ensure(context.Background(), "", func(done, total int64) { last = done })
	if !st.Ready || st.Source != SourcePinned || bin == "" {
		t.Fatalf("Ensure: %+v %q", st, bin)
	}
	if last != int64(len(archive)) {
		t.Fatalf("progress ended at %d of %d", last, len(archive))
	}
	if _, err := os.Stat(filepath.Join(b.PinnedDir(), verifiedMarker)); err != nil {
		t.Fatalf("no verified marker: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.PinnedDir(), "resources", "data.pak")); string(got) != "data" {
		t.Fatalf("data file = %q", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*.part"))
	tmp, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*.tmp-*"))
	if len(leftovers)+len(tmp) != 0 {
		t.Fatalf("temporary files left: %v %v", leftovers, tmp)
	}
	// Second time: installed, no new download, status is ready without Ensure.
	srv.Close()
	b.ForgetProbes()
	st = b.Status(context.Background(), "")
	if !st.Ready || st.Source != SourcePinned {
		t.Fatalf("status after the install: %+v", st)
	}
	_ = probes
}

func TestPinnedChromiumRefusesWrongChecksumSizeAndStatus(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := serveArchive(t, archive)
	cases := map[string]PinnedChromium{}
	badHash := pinFor(srv.URL, archive)
	badHash.SHA256 = strings.Repeat("0", 64)
	cases["checksum"] = badHash
	badSize := pinFor(srv.URL, archive)
	badSize.Size++
	cases["size"] = badSize
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)
	cases["status"] = pinFor(notFound.URL, archive)
	for name, pin := range cases {
		b, _ := newTestBrowsers(t)
		b.pins = map[string]PinnedChromium{b.platform: pin}
		st, bin := b.Ensure(context.Background(), "", nil)
		if st.Ready || bin != "" || !strings.Contains(st.Issue, "could not be downloaded") {
			t.Errorf("%s: %+v %q", name, st, bin)
		}
		if _, err := os.Stat(b.PinnedDir()); !os.IsNotExist(err) {
			t.Errorf("%s: an unverified archive was installed", name)
		}
		if parts, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*")); len(parts) != 0 {
			t.Errorf("%s: left %v", name, parts)
		}
	}
}

func TestExtractChromiumRefusesEscapesAndForeignEntries(t *testing.T) {
	cases := map[string]func(w *zip.Writer){
		"dot-dot": func(w *zip.Writer) { f, _ := w.Create("chrome-x/../evil.txt"); _, _ = f.Write([]byte("x")) },
		"outside": func(w *zip.Writer) { f, _ := w.Create("other/evil.txt"); _, _ = f.Write([]byte("x")) },
		"abs":     func(w *zip.Writer) { f, _ := w.Create("chrome-x//etc/evil"); _, _ = f.Write([]byte("x")) },
		"backslash": func(w *zip.Writer) {
			f, _ := w.Create("chrome-x/a\\..\\..\\evil")
			_, _ = f.Write([]byte("x"))
		},
		"symlink-escape": func(w *zip.Writer) {
			h := &zip.FileHeader{Name: "chrome-x/link"}
			h.SetMode(os.ModeSymlink | 0o777)
			f, _ := w.CreateHeader(h)
			_, _ = f.Write([]byte("../../outside"))
		},
		"symlink-abs": func(w *zip.Writer) {
			h := &zip.FileHeader{Name: "chrome-x/link"}
			h.SetMode(os.ModeSymlink | 0o777)
			f, _ := w.CreateHeader(h)
			_, _ = f.Write([]byte("/etc/passwd"))
		},
	}
	for name, extra := range cases {
		dir := t.TempDir()
		zipPath := filepath.Join(dir, "a.zip")
		if err := os.WriteFile(zipPath, fakeChromiumZip(t, "chrome-x", extra), 0o600); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(dir, "out")
		if err := extractChromium(zipPath, dest, "chrome-x"); err == nil {
			t.Errorf("%s: the archive was extracted", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
			t.Errorf("%s: a file escaped the folder", name)
		}
	}
}

func TestExtractChromiumKeepsRelativeSymlinksInsideTheFolder(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	data := fakeChromiumZip(t, "chrome-x", func(w *zip.Writer) {
		h := &zip.FileHeader{Name: "chrome-x/resources/current"}
		h.SetMode(os.ModeSymlink | 0o777)
		f, _ := w.CreateHeader(h)
		_, _ = f.Write([]byte("data.pak"))
	})
	if err := os.WriteFile(zipPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	err := extractChromium(zipPath, dest, "chrome-x")
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks need a privilege on this Windows host: %v", err)
		}
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(dest, "resources", "current")); err != nil || target != "data.pak" {
		t.Fatalf("symlink = %q, %v", target, err)
	}
}

func TestPinnedTableCoversTheSupportedPlatforms(t *testing.T) {
	for _, platform := range []string{"windows_amd64", "linux_amd64", "darwin_amd64", "darwin_arm64"} {
		pin, ok := pinnedChromium[platform]
		if !ok {
			t.Errorf("no pin for %s", platform)
			continue
		}
		if len(pin.SHA256) != 64 || pin.Size < 100<<20 || !strings.HasPrefix(pin.URL, "https://storage.googleapis.com/chromium-browser-snapshots/") ||
			!strings.Contains(pin.URL, "/1321438/") || pin.Dir == "" || pin.Exe == "" {
			t.Errorf("%s: incomplete pin %+v", platform, pin)
		}
	}
}

// TestPinnedArchivesExtract checks the four real snapshot archives against the pinned table: SHA-256,
// size and a clean extraction (it does not start the browser). It runs when AURAGO_SM_PINNED_DIR names
// a folder holding windows_amd64.zip, linux_amd64.zip, darwin_amd64.zip and darwin_arm64.zip (the
// downloads of the pinned revision); symlink-bearing macOS archives are only extracted on hosts that
// may create symlinks.
func TestPinnedArchivesExtract(t *testing.T) {
	dir := os.Getenv("AURAGO_SM_PINNED_DIR")
	if dir == "" {
		t.Skip("AURAGO_SM_PINNED_DIR is not set")
	}
	for platform, pin := range pinnedChromium {
		t.Run(platform, func(t *testing.T) {
			archive := filepath.Join(dir, platform+".zip")
			data, err := os.ReadFile(archive)
			if err != nil {
				t.Skipf("no archive for %s: %v", platform, err)
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != pin.SHA256 || int64(len(data)) != pin.Size {
				t.Fatalf("%s does not match the pinned hash or size", archive)
			}
			if strings.HasPrefix(platform, "darwin") && runtime.GOOS == "windows" {
				t.Skip("macOS archives carry symlinks that Windows may not create")
			}
			dest := filepath.Join(t.TempDir(), "out")
			if err := extractChromium(archive, dest, pin.Dir); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(pin.Exe))); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("executable %s missing after extraction: %v", pin.Exe, err)
			}
		})
	}
}

// The resolver of tests above this package (NewBrowsersWithProbe) must still know the pinned
// Chromium, so a status can say "downloadable", and must never download it.
func TestBrowsersWithProbeKnowsThePinnedChromiumButNeverDownloadsIt(t *testing.T) {
	if _, ok := pinnedChromium[runtime.GOOS+"_"+runtime.GOARCH]; !ok {
		t.Skip("no pinned Chromium for this platform")
	}
	b := NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "", false },
		func(context.Context, string) (string, error) { return "", errors.New("no browser") })
	st := b.Status(context.Background(), "")
	if st.Ready || !st.CanDownload || st.Source != SourcePinned || !strings.Contains(st.Issue, "first export downloads") {
		t.Fatalf("status = %+v, want not ready, downloadable, source pinned", st)
	}
	st, exe := b.Ensure(context.Background(), "", nil)
	if exe != "" || st.Ready || !strings.Contains(st.Issue, "switched off") {
		t.Fatalf("Ensure = %+v %q, want no download", st, exe)
	}
}

// stalledArchiveServer sends the headers and the first bytes of the archive and then goes silent
// without closing the connection.
func stalledArchiveServer(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		_, _ = w.Write(archive[:100])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv
}

func TestPinnedChromiumDownloadGivesUpOnAStalledConnection(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := stalledArchiveServer(t, archive)
	b, _ := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = 300 * time.Millisecond

	// The guard only turns a missing stall timeout into a failure instead of a hung test.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	st, bin := b.Ensure(ctx, "", nil)
	if st.Ready || bin != "" || !strings.Contains(st.Issue, "stalled") {
		t.Fatalf("Ensure after a stall: %+v %q", st, bin)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("the stalled download held the caller for %s", took)
	}
	if parts, _ := filepath.Glob(filepath.Join(b.dir, "chromium-*")); len(parts) != 0 {
		t.Fatalf("a stalled download left %v", parts)
	}
	// The lock is free again: a second export is not stuck behind the first.
	done := make(chan struct{})
	go func() { b.Ensure(ctx, "", nil); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the download lock was not released")
	}
}

// The stall limit is an idle limit, not a deadline for the whole download: a slow link that keeps
// delivering must finish.
func TestPinnedChromiumDownloadKeepsASlowConnectionAlive(t *testing.T) {
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		for off := 0; off < len(archive); off += 60 {
			end := min(off+60, len(archive))
			_, _ = w.Write(archive[off:end])
			w.(http.Flusher).Flush()
			time.Sleep(120 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)
	b, _ := newTestBrowsers(t)
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = 500 * time.Millisecond
	start := time.Now()
	st, bin := b.Ensure(context.Background(), "", nil)
	if !st.Ready || bin == "" {
		t.Fatalf("a slow download was cut: %+v", st)
	}
	if took := time.Since(start); took < b.idleTimeout {
		t.Fatalf("the download finished in %s, which does not exercise the idle limit (%s)", took, b.idleTimeout)
	}
}
