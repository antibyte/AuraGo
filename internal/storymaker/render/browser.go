package render

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/fileutil"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser sources reported by the status endpoint.
const (
	SourceConfig  = "config"
	SourceSystem  = "system"
	SourcePinned  = "pinned"
	SourceMissing = "missing"
)

// PinnedRevision is the Chromium snapshot revision AuraGo downloads when no usable browser is
// installed: the revision rod v0.116.2 ships as its default (Chromium 128.0.6568.0). The archives
// below were downloaded once from storage.googleapis.com/chromium-browser-snapshots and their
// SHA-256 was computed; a download is verified against it before anything is unpacked or run.
const PinnedRevision = 1321438

// PinnedChromium describes one platform's snapshot archive.
type PinnedChromium struct {
	URL    string
	SHA256 string
	Size   int64
	// Dir is the single top-level folder inside the archive; it is stripped on extraction.
	Dir string
	// Exe is the executable below the install folder.
	Exe string
}

func snapshotURL(prefix, zipName string) string {
	return fmt.Sprintf("https://storage.googleapis.com/chromium-browser-snapshots/%s/%d/%s", prefix, PinnedRevision, zipName)
}

// pinnedChromium is keyed by GOOS_GOARCH. There is no mirror list: one host, one checksum.
var pinnedChromium = map[string]PinnedChromium{
	"windows_amd64": {URL: snapshotURL("Win_x64", "chrome-win.zip"), SHA256: "f8ddee43d6865a974fc8acfcfcb9f6a42e84aadc427482ae2fa2e6a36a413454", Size: 267483258, Dir: "chrome-win", Exe: "chrome.exe"},
	"linux_amd64":   {URL: snapshotURL("Linux_x64", "chrome-linux.zip"), SHA256: "b5fb87b55fd81eaba9325d67220585cc12c12df529bd8ae46906d43dd5699e4e", Size: 190157449, Dir: "chrome-linux", Exe: "chrome"},
	"darwin_amd64":  {URL: snapshotURL("Mac", "chrome-mac.zip"), SHA256: "b25cd79dc73ebf423b05a4a0d65f772a78e3b87d394e9a2e7d0a4cc9201bda7b", Size: 149328540, Dir: "chrome-mac", Exe: "Chromium.app/Contents/MacOS/Chromium"},
	"darwin_arm64":  {URL: snapshotURL("Mac_Arm", "chrome-mac.zip"), SHA256: "4a478b542f1f6eaa160e64aab76100501e061d3bba46f6a480897ef1103c70c5", Size: 135605548, Dir: "chrome-mac", Exe: "Chromium.app/Contents/MacOS/Chromium"},
}

const (
	verifiedMarker      = ".storymaker-verified"
	maxExtractFiles     = 5000
	maxExtractBytes     = 2 << 30
	probeOKCacheFor     = 10 * time.Minute
	probeFailCacheFor   = 30 * time.Second
	browserProbeTimeout = 45 * time.Second
	// downloadIdleTimeout is how long the pinned Chromium download may go without receiving a byte
	// (the wait for the response headers included) before it is given up. The size of the archive
	// is not limited by it: a slow link that keeps delivering is left alone.
	downloadIdleTimeout = 60 * time.Second
)

// BrowserStatus is the browser part of GET /status.
type BrowserStatus struct {
	// Ready is true only after a successful headless launch probe of the browser that Source names.
	Ready  bool   `json:"ready"`
	Source string `json:"source"` // config | system | pinned | missing
	Issue  string `json:"issue,omitempty"`
	// CanDownload tells that no browser is ready but the pinned Chromium can be downloaded by the
	// first export (the download is never started by a status request).
	CanDownload bool   `json:"can_download,omitempty"`
	Version     string `json:"version,omitempty"`
}

// BrowserProgress receives download progress (bytes done, bytes total).
type BrowserProgress func(done, total int64)

// Browsers resolves, provisions and probes the render browser.
type Browsers struct {
	dir string // <DataDir>/storymaker

	// Seams for tests.
	lookPath    func() (string, bool)
	launchProbe func(ctx context.Context, bin string) (string, error)
	client      *http.Client
	pins        map[string]PinnedChromium
	platform    string
	idleTimeout time.Duration // download stall limit; 0 selects downloadIdleTimeout

	probeMu sync.Mutex
	probes  map[string]probeResult
	dlMu    sync.Mutex
}

type probeResult struct {
	version string
	err     error
	at      time.Time
}

// NewBrowsers returns a resolver that keeps the pinned Chromium below dir.
func NewBrowsers(dir string) *Browsers {
	return &Browsers{
		dir: dir, lookPath: launcher.LookPath, launchProbe: probeLaunch,
		client: &http.Client{Timeout: 0}, pins: pinnedChromium, platform: runtime.GOOS + "_" + runtime.GOARCH,
		probes: map[string]probeResult{},
	}
}

func (b *Browsers) pin() (PinnedChromium, bool) {
	p, ok := b.pins[b.platform]
	return p, ok
}

// PinnedDir is the install folder of the pinned Chromium.
func (b *Browsers) PinnedDir() string {
	return filepath.Join(b.dir, fmt.Sprintf("chromium-%d", PinnedRevision))
}

func (b *Browsers) pinnedInstalled() (string, bool) {
	pin, ok := b.pin()
	if !ok {
		return "", false
	}
	marker, err := os.ReadFile(filepath.Join(b.PinnedDir(), verifiedMarker))
	if err != nil || strings.TrimSpace(string(marker)) != pin.SHA256 {
		return "", false
	}
	exe := filepath.Join(b.PinnedDir(), filepath.FromSlash(pin.Exe))
	if info, err := os.Stat(exe); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return exe, true
}

// NewBrowsersWithProbe is NewBrowsers with the system browser lookup and the launch probe replaced
// (tests of the layers above this package use it to avoid starting a browser). The pinned Chromium
// stays known, so a status can say that it is downloadable, but the resolver never downloads it.
func NewBrowsersWithProbe(dir string, lookPath func() (string, bool), probe func(ctx context.Context, bin string) (string, error)) *Browsers {
	b := NewBrowsers(dir)
	b.lookPath, b.launchProbe = lookPath, probe
	b.client = &http.Client{Transport: refuseDownloads{}}
	return b
}

// refuseDownloads is the transport of the test resolver: nothing is fetched.
type refuseDownloads struct{}

func (refuseDownloads) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("downloads are switched off in this resolver")
}

// Status reports the browser without downloading anything.
func (b *Browsers) Status(ctx context.Context, configuredPath string) BrowserStatus {
	st, _ := b.resolve(ctx, configuredPath, false, nil)
	return st
}

// Ensure resolves the browser like Status and, when only the pinned Chromium is left, downloads and
// verifies it first. It returns the executable to launch.
func (b *Browsers) Ensure(ctx context.Context, configuredPath string, progress BrowserProgress) (BrowserStatus, string) {
	return b.resolve(ctx, configuredPath, true, progress)
}

func (b *Browsers) resolve(ctx context.Context, configuredPath string, download bool, progress BrowserProgress) (BrowserStatus, string) {
	// 1. An operator-configured browser is the only candidate: a broken one is reported, not replaced.
	if p := strings.TrimSpace(configuredPath); p != "" {
		st := BrowserStatus{Source: SourceConfig}
		if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
			st.Issue = "The configured browser path (storymaker.render.browser_path) does not point to an executable file."
			return st, ""
		}
		if ver, err := b.probe(ctx, p); err != nil {
			st.Issue = launchIssue(err)
			return st, ""
		} else {
			st.Ready, st.Version = true, ver
		}
		return st, p
	}

	// 2. A browser the system provides.
	systemIssue := ""
	if found, ok := b.lookPath(); ok {
		if ver, err := b.probe(ctx, found); err == nil {
			return BrowserStatus{Ready: true, Source: SourceSystem, Version: ver}, found
		} else {
			systemIssue = launchIssue(err)
		}
	}

	// 3. The pinned Chromium.
	pin, hasPin := b.pin()
	if !hasPin {
		issue := "No Chrome or Chromium was found. Install one, or set storymaker.render.browser_path."
		if systemIssue != "" {
			return BrowserStatus{Source: SourceSystem, Issue: systemIssue}, ""
		}
		return BrowserStatus{Source: SourceMissing, Issue: issue}, ""
	}
	exe, installed := b.pinnedInstalled()
	if !installed && download {
		var err error
		if exe, err = b.install(ctx, pin, progress); err != nil {
			st := BrowserStatus{Source: SourcePinned, CanDownload: true, Issue: "The pinned Chromium could not be downloaded: " + clip(err.Error(), 300)}
			return st, ""
		}
		installed = true
	}
	if !installed {
		issue := fmt.Sprintf("No usable Chrome or Chromium was found. The first export downloads a pinned Chromium (about %d MB) into the data folder.", pin.Size>>20)
		if systemIssue != "" {
			issue = systemIssue + " " + issue
		}
		return BrowserStatus{Source: SourcePinned, CanDownload: true, Issue: issue}, ""
	}
	ver, err := b.probe(ctx, exe)
	if err != nil {
		return BrowserStatus{Source: SourcePinned, Issue: launchIssue(err)}, ""
	}
	return BrowserStatus{Ready: true, Source: SourcePinned, Version: ver}, exe
}

// launchIssue turns a launch error into the text the status shows: the cause, and for the sandbox
// case the way out (the sandbox is never turned off).
func launchIssue(err error) string {
	text := clip(strings.TrimSpace(err.Error()), 400)
	low := strings.ToLower(text)
	switch {
	case strings.Contains(low, "no usable sandbox"), strings.Contains(low, "sandbox"):
		return "The browser could not start with its sandbox: " + text + ". On Ubuntu 23.10 or later install the AppArmor profile from the installation guide, or set storymaker.render.browser_path to a Chrome that can run here."
	case strings.Contains(low, "error while loading shared libraries"), strings.Contains(low, "shared libraries"):
		return "The browser is missing system libraries: " + text + ". In a container, install Chromium's libraries or set storymaker.render.browser_path."
	}
	return "The browser could not be started: " + text
}

// probe runs (or reuses) the headless launch probe of one executable.
func (b *Browsers) probe(ctx context.Context, bin string) (string, error) {
	key := bin
	if info, err := os.Stat(bin); err == nil {
		key = fmt.Sprintf("%s|%d|%d", bin, info.Size(), info.ModTime().UnixNano())
	}
	b.probeMu.Lock()
	defer b.probeMu.Unlock()
	if r, ok := b.probes[key]; ok {
		ttl := probeOKCacheFor
		if r.err != nil {
			ttl = probeFailCacheFor
		}
		if time.Since(r.at) < ttl {
			return r.version, r.err
		}
	}
	version, err := b.launchProbe(ctx, bin)
	if ctx.Err() != nil {
		return version, ctx.Err() // an aborted probe says nothing about the browser
	}
	b.probes[key] = probeResult{version: version, err: err, at: time.Now()}
	return version, err
}

// ForgetProbes drops the cached probe results (the next status request launches again).
func (b *Browsers) ForgetProbes() {
	b.probeMu.Lock()
	b.probes = map[string]probeResult{}
	b.probeMu.Unlock()
}

// noSandboxForTests lets the tests of a host that cannot run Chrome's sandbox launch it anyway.
// It is a variable of this package only: no configuration or environment value reaches it.
var noSandboxForTests bool

// SetNoSandboxForTests is the test hook for the variable above. Production code never calls it.
func SetNoSandboxForTests(on bool) { noSandboxForTests = on }

// launchOptions describes one Chrome launch.
type launchOptions struct {
	Bin         string
	UserDataDir string
	// ProxyAddr is the strict capture proxy (host:port); empty launches without a proxy (the probe).
	ProxyAddr  string
	WindowSize [2]int
}

// newLauncher builds the rod launcher every Storymaker browser launch uses. It never sets
// NoSandbox (rod adds --no-sandbox by itself inside containers; that is removed here), except
// through the test hook.
func newLauncher(ctx context.Context, o launchOptions) *launcher.Launcher {
	l := launcher.New().Context(ctx).Bin(o.Bin).Headless(true)
	l.Delete("no-sandbox")
	if noSandboxForTests {
		l.Set("no-sandbox")
	}
	l.UserDataDir(o.UserDataDir)
	l.Set("disable-gpu")
	l.Set("hide-scrollbars")
	l.Set("mute-audio")
	l.Set("font-render-hinting", "none")
	l.Set("no-default-browser-check")
	l.Set("disable-extensions")
	l.Set("disable-component-update")
	l.Set("no-pings")
	l.Set("disable-domain-reliability")
	// rod's list keeps the site-per-process switch off; add what a render page never needs.
	l.Set("disable-features", "site-per-process", "TranslateUI", "OptimizationHints", "MediaRouter", "InterestFeedContentSuggestions", "DialMediaRouteProvider")
	if o.WindowSize[0] > 0 && o.WindowSize[1] > 0 {
		l.Set("window-size", fmt.Sprintf("%d,%d", o.WindowSize[0], o.WindowSize[1]))
	}
	if o.ProxyAddr != "" {
		// Every byte goes through the strict proxy, loopback included; STUN/WebRTC leaks are closed
		// (Chrome 156 ignores the force- spelling for STUN, so both are set).
		l.Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp")
		l.Set("webrtc-ip-handling-policy", "disable_non_proxied_udp")
		l.Proxy(o.ProxyAddr)
		l.Set("proxy-bypass-list", "<-loopback>")
	}
	return l
}

// probeLaunch starts the browser headless, opens about:blank, evaluates a trivial script and
// closes it again. It returns the product version.
func probeLaunch(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, browserProbeTimeout)
	defer cancel()
	profile, err := os.MkdirTemp("", "aurago-sm-probe-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profile)
	l := newLauncher(ctx, launchOptions{Bin: bin, UserDataDir: profile})
	controlURL, err := l.Launch()
	if err != nil {
		return "", err
	}
	defer func() { l.Kill(); l.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return "", err
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = browser.Context(closeCtx).Close()
	}()
	version := ""
	if v, err := (proto.BrowserGetVersion{}).Call(browser); err == nil {
		version = v.Product
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return version, err
	}
	res, err := page.Eval(`() => 6 * 7`)
	if err != nil {
		return version, err
	}
	if res.Value.Int() != 42 {
		return version, errors.New("the browser did not evaluate a script")
	}
	return version, nil
}

// install downloads the pinned archive, verifies its SHA-256, unpacks it next to the final folder
// and moves it into place. A verified install carries a marker with the archive hash.
func (b *Browsers) install(ctx context.Context, pin PinnedChromium, progress BrowserProgress) (string, error) {
	b.dlMu.Lock()
	defer b.dlMu.Unlock()
	if exe, ok := b.pinnedInstalled(); ok {
		return exe, nil // another job finished the install while this one waited
	}
	if err := os.MkdirAll(b.dir, 0o700); err != nil {
		return "", err
	}
	if free, err := fileutil.FreeDiskBytes(b.dir); err == nil && free < pin.Size*4 {
		return "", fmt.Errorf("not enough free disk space for the browser download (%d MB needed)", pin.Size*4>>20)
	}
	archive := filepath.Join(b.dir, fmt.Sprintf("chromium-%d.zip.part", PinnedRevision))
	defer os.Remove(archive)
	if err := b.download(ctx, pin, archive, progress); err != nil {
		return "", err
	}
	stage := filepath.Join(b.dir, fmt.Sprintf("chromium-%d.tmp-%s", PinnedRevision, rand.Text()))
	defer os.RemoveAll(stage)
	if err := extractChromium(archive, stage, pin.Dir); err != nil {
		return "", err
	}
	exe := filepath.Join(stage, filepath.FromSlash(pin.Exe))
	if info, err := os.Stat(exe); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("the archive does not contain the browser executable")
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(exe, 0o755)
	}
	if err := os.WriteFile(filepath.Join(stage, verifiedMarker), []byte(pin.SHA256+"\n"), 0o600); err != nil {
		return "", err
	}
	final := b.PinnedDir()
	_ = os.RemoveAll(final)
	if err := fileutil.Rename(stage, final); err != nil {
		return "", fmt.Errorf("move the browser into place: %w", err)
	}
	return filepath.Join(final, filepath.FromSlash(pin.Exe)), nil
}

func (b *Browsers) download(ctx context.Context, pin PinnedChromium, dest string, progress BrowserProgress) error {
	idle := b.idleTimeout
	if idle <= 0 {
		idle = downloadIdleTimeout
	}
	// A peer that keeps the connection open but sends nothing would hold the caller (the single
	// render worker) forever: the timer cancels the request when no byte arrived for idle.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool
	timer := time.AfterFunc(idle, func() { stalled.Store(true); cancel() })
	defer timer.Stop()
	fail := func(err error) error {
		if stalled.Load() {
			return fmt.Errorf("download: no data for %s, the connection stalled", idle)
		}
		return fmt.Errorf("download: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.URL, nil)
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer res.Body.Close()
	timer.Reset(idle)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download: the server answered %d", res.StatusCode)
	}
	if res.ContentLength > 0 && res.ContentLength != pin.Size {
		return fmt.Errorf("download: the archive is %d bytes, the pinned size is %d", res.ContentLength, pin.Size)
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	counter := &countingWriter{progress: progress, total: pin.Size}
	body := tickReader{r: io.LimitReader(res.Body, pin.Size+1), tick: func() { timer.Reset(idle) }}
	n, copyErr := io.Copy(io.MultiWriter(out, hash, counter), body)
	closeErr := out.Close()
	if copyErr != nil {
		return fail(copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n != pin.Size {
		return fmt.Errorf("download: received %d bytes, the pinned size is %d", n, pin.Size)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != pin.SHA256 {
		return fmt.Errorf("download: the SHA-256 of the archive (%s) does not match the pinned value", got)
	}
	return nil
}

// tickReader calls tick after every read that delivered bytes.
type tickReader struct {
	r    io.Reader
	tick func()
}

func (t tickReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.tick()
	}
	return n, err
}

type countingWriter struct {
	done     int64
	total    int64
	progress BrowserProgress
	last     time.Time
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	if c.progress != nil && (time.Since(c.last) > 200*time.Millisecond || c.done == c.total) {
		c.last = time.Now()
		c.progress(c.done, c.total)
	}
	return len(p), nil
}

// extractChromium unpacks the archive into dest, stripping the top-level folder topDir. It
// refuses names that leave dest, entries outside topDir, absolute or escaping symlinks, and
// archives over the file and size limits.
func extractChromium(archive, dest, topDir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer zr.Close()
	if len(zr.File) > maxExtractFiles {
		return errors.New("the archive has too many entries")
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	prefix := topDir + "/"
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("the archive entry %q is outside %q", name, topDir)
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}
		if strings.Contains(rel, "\\") || strings.ContainsRune(rel, 0) || path.IsAbs(rel) || !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("the archive entry %q has an unsafe name", name)
		}
		localRel := filepath.FromSlash(strings.TrimSuffix(rel, "/"))
		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			if err := root.MkdirAll(localRel, 0o700); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			target, err := readZipEntry(f, 4096)
			if err != nil {
				return err
			}
			t := string(target)
			resolved := path.Join(path.Dir(strings.TrimSuffix(rel, "/")), t)
			if path.IsAbs(t) || strings.Contains(t, "\\") || !filepath.IsLocal(filepath.FromSlash(resolved)) {
				return fmt.Errorf("the archive symlink %q points outside the install folder", name)
			}
			if err := root.MkdirAll(filepath.Dir(localRel), 0o700); err != nil {
				return err
			}
			if err := root.Symlink(filepath.FromSlash(t), localRel); err != nil {
				return fmt.Errorf("create the symlink %q: %w", rel, err)
			}
		case mode.IsRegular():
			total += int64(f.UncompressedSize64)
			if total > maxExtractBytes || f.UncompressedSize64 > maxExtractBytes {
				return errors.New("the archive is larger than allowed")
			}
			if err := root.MkdirAll(filepath.Dir(localRel), 0o700); err != nil {
				return err
			}
			perm := os.FileMode(0o600)
			if mode&0o111 != 0 {
				perm = 0o700
			}
			if err := writeZipFile(root, f, localRel, perm); err != nil {
				return err
			}
		default:
			return fmt.Errorf("the archive entry %q has an unsupported type", name)
		}
	}
	return nil
}

func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("an archive entry is too large")
	}
	return data, nil
}

func writeZipFile(root *os.Root, f *zip.File, rel string, perm os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if uint64(n) != f.UncompressedSize64 {
		return fmt.Errorf("the archive entry %q does not match its recorded size", f.Name)
	}
	return nil
}
