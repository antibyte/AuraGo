package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/image/webp"
)

// browserSmoke gates the tests that start a real Chrome: AURAGO_RUN_BROWSER_SMOKE=1. On the Windows
// development box run them through `go test` with GOTMPDIR=D:\tmp\go-build (repo-built executables
// cannot start Chrome there).
func browserSmoke(t *testing.T) string {
	t.Helper()
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the real browser tests")
	}
	chrome := os.Getenv("AURAGO_TEST_CHROME")
	if chrome == "" {
		found, ok := launcher.LookPath()
		if !ok {
			t.Skip("no Chrome or Chromium is installed")
		}
		chrome = found
	}
	if os.Getenv("AURAGO_TEST_NO_SANDBOX") == "1" {
		SetNoSandboxForTests(true)
		t.Cleanup(func() { SetNoSandboxForTests(false) })
	}
	return chrome
}

// realFixture is a run fixture whose Browsers and OpenPage are the real ones.
func newRealFixture(t *testing.T, chrome string) *runFixture {
	t.Helper()
	fx := newRunFixture(t)
	fx.deps.Browsers = NewBrowsers(fx.dir)
	fx.deps.OpenPage = openSession
	fx.settings.BrowserPath = chrome
	return fx
}

func (fx *runFixture) rebuild(t *testing.T) {
	t.Helper()
	fx.mgr.Close()
	fx.runner = NewRunner(fx.deps)
	mgr, err := NewManager(ManagerOptions{Dir: fx.dir, Run: fx.runner.Run})
	if err != nil {
		t.Fatal(err)
	}
	fx.mgr = mgr
	t.Cleanup(mgr.Close)
}

// loadFirewall puts the firewall story and its code scene into the fixture's project.
func (fx *runFixture) loadFirewall(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	story, err := os.ReadFile("../testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, lf(story), ""); err != nil {
		t.Fatal(err)
	}
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", lf(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
}

// frameHashes records the decoded-pixel hash of every frame of a render.
type frameHashes struct {
	mu     sync.Mutex
	hashes map[int]string
}

func newFrameHashes() *frameHashes { return &frameHashes{hashes: map[int]string{}} }

func (f *frameHashes) observe(t *testing.T) func(n int, transport string, data []byte) {
	return func(n int, transport string, data []byte) {
		h := decodedHash(t, transport, data)
		f.mu.Lock()
		f.hashes[n] = h // a restart overwrites earlier frames, as the encoder does
		f.mu.Unlock()
	}
}

func (f *frameHashes) sequence() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]int, 0, len(f.hashes))
	for k := range f.hashes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	sum := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(sum, "%d:%s\n", k, f.hashes[k])
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func (f *frameHashes) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.hashes)
}

// decodedHash hashes the straight RGBA pixels of a frame, whatever its transport.
func decodedHash(t *testing.T, transport string, data []byte) string {
	if transport == TransportRaw {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	var img image.Image
	var err error
	if transport == TransportWebP {
		img, err = webp.Decode(bytes.NewReader(data))
	} else {
		img, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		t.Errorf("a %s frame did not decode: %v", transport, err)
		return ""
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	sum := sha256.Sum256(nrgba.Pix)
	return hex.EncodeToString(sum[:])
}

func TestBrowserStatusProbeLaunchesRealChrome(t *testing.T) {
	chrome := browserSmoke(t)
	b := NewBrowsers(t.TempDir())
	st := b.Status(context.Background(), chrome)
	if !st.Ready || st.Source != SourceConfig || st.Version == "" {
		t.Fatalf("status = %+v", st)
	}
	t.Logf("browser %s", st.Version)
	st = b.Status(context.Background(), "")
	t.Logf("resolution without a configured path: %+v", st)
}

func renderFirewall(t *testing.T, chrome, transport string, observer func(int, string, []byte)) (*runFixture, Job, time.Duration) {
	t.Helper()
	fx := newRealFixture(t, chrome)
	fx.settings.Transport = transport
	fx.deps.FrameObserver = observer
	fx.rebuild(t)
	fx.loadFirewall(t)
	started := time.Now()
	job := fx.render(t)
	return fx, job, time.Since(started)
}

func TestRealRenderOfTheFirewallStory(t *testing.T) {
	chrome := browserSmoke(t)
	ffprobe := testBinary(t, "ffprobe")
	hashes := newFrameHashes()
	fx, job, took := renderFirewall(t, chrome, "", hashes.observe(t))
	if job.Status != StatusSucceeded || job.Output == nil {
		t.Fatalf("job = %+v error %+v", job, job.Error)
	}
	t.Logf("firewall.json: %d frames in %v (%.1f fps overall)", job.FramesTotal, took.Round(time.Millisecond), float64(job.FramesTotal)/took.Seconds())
	mp4, ok := fx.pub.get(job.Output.Path)
	if !ok || len(mp4) < 10_000 {
		t.Fatalf("published file: %v %d bytes", ok, len(mp4))
	}
	tmp := filepath.Join(t.TempDir(), "firewall.mp4")
	if err := os.WriteFile(tmp, mp4, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := ffprobeFile(t, ffprobe, tmp)
	var video, audio bool
	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			video = s.Width == 1920 && s.Height == 1080 && s.AvgFrameRate == "30/1" && s.NbFrames == strconv.Itoa(job.FramesTotal) && s.PixFmt == "yuv420p" && s.ColorSpace == "bt709"
		case "audio":
			audio = s.CodecName == "aac"
		}
	}
	duration, _ := strconv.ParseFloat(doc.Format.Duration, 64)
	want := float64(job.FramesTotal) / 30
	if !video || !audio || duration < want-0.15 || duration > want+0.15 || job.FramesTotal < 250 {
		t.Fatalf("ffprobe: video=%v audio=%v duration=%v (want about %v) frames=%d streams=%+v", video, audio, duration, want, job.FramesTotal, doc.Streams)
	}
	if hashes.count() != job.FramesTotal {
		t.Errorf("observed %d frames of %d", hashes.count(), job.FramesTotal)
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging left: %v", entries)
	}
}

func TestRealRenderIsDeterministic(t *testing.T) {
	chrome := browserSmoke(t)
	var sequences []string
	for i := 0; i < 2; i++ {
		hashes := newFrameHashes()
		_, job, _ := renderFirewall(t, chrome, "", hashes.observe(t))
		if job.Status != StatusSucceeded {
			t.Fatalf("render %d: %+v %+v", i, job, job.Error)
		}
		sequences = append(sequences, hashes.sequence())
		if hashes.count() != job.FramesTotal {
			t.Fatalf("render %d observed %d of %d frames", i, hashes.count(), job.FramesTotal)
		}
	}
	if sequences[0] != sequences[1] {
		t.Fatalf("two renders of the same story differ:\n%s\n%s", sequences[0], sequences[1])
	}
}

// TestTransportSpike is the phase 1 frame-transport spike (spec step 4): firewall.json at 1080p30
// through every transport on the production path (strict proxy and CDP interception active), twice
// each. It prints frames per second, whether the two renders are identical, whether the transports
// agree with each other and the peak memory. Run it with AURAGO_STORYMAKER_SPIKE=1; the table is
// also written to AURAGO_STORYMAKER_SPIKE_OUT when that names a file.
func TestTransportSpike(t *testing.T) {
	chrome := browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_SPIKE") != "1" {
		t.Skip("set AURAGO_STORYMAKER_SPIKE=1 to run the transport spike")
	}
	type row struct {
		Transport   string  `json:"transport"`
		Frames      int     `json:"frames"`
		RenderFPS   float64 `json:"render_fps"`
		JobSeconds  float64 `json:"job_seconds"`
		Identical   bool    `json:"two_renders_identical"`
		Sequence    string  `json:"sequence_hash"`
		ChromePeak  int64   `json:"chrome_peak_rss_mb"`
		GoHeapPeak  int64   `json:"go_heap_peak_mb"`
		MP4Bytes    int     `json:"mp4_bytes"`
		FrameBytes  int64   `json:"frame_bytes_avg"`
		Environment string  `json:"environment"`
	}
	var rows []row
	transports := []string{TransportPNG, TransportWebP, TransportRaw, TransportScreenshot}
	if only := os.Getenv("AURAGO_STORYMAKER_SPIKE_TRANSPORTS"); only != "" {
		transports = strings.Split(only, ",")
	}
	for _, transport := range transports {
		var seqs []string
		var best row
		runs := 2
		if n, err := strconv.Atoi(os.Getenv("AURAGO_STORYMAKER_SPIKE_RUNS")); err == nil && n > 0 {
			runs = n
		}
		for run := 0; run < runs; run++ {
			hashes := newFrameHashes()
			var totalBytes int64
			var firstFrame, lastFrame time.Time
			var fmu sync.Mutex
			observer := func(n int, tr string, data []byte) {
				fmu.Lock()
				if firstFrame.IsZero() {
					firstFrame = time.Now()
				}
				lastFrame = time.Now()
				totalBytes += int64(len(data))
				fmu.Unlock()
				hashes.observe(t)(n, tr, data)
			}
			fx := newRealFixture(t, chrome)
			fx.settings.Transport = transport
			if os.Getenv("AURAGO_STORYMAKER_SPIKE_PLAYER") == "recommended" {
				fx.deps.UI = patchedUI{FS: fx.deps.UI, edit: recommendedPlayerChanges}
			}
			fx.deps.FrameObserver = observer
			var pid int
			var pidMu sync.Mutex
			fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
				p, err := openSession(ctx, o)
				if s, ok := p.(*session); ok && err == nil {
					pidMu.Lock()
					pid = s.l.PID()
					pidMu.Unlock()
				}
				return p, err
			}
			fx.rebuild(t)
			fx.loadFirewall(t)

			stop := make(chan struct{})
			var chromePeak, heapPeak int64
			var wg sync.WaitGroup
			sample := os.Getenv("AURAGO_STORYMAKER_SPIKE_MEM") == "1"
			if sample {
				wg.Add(1)
			}
			go func() {
				if !sample {
					return
				}
				defer wg.Done()
				tick := time.NewTicker(250 * time.Millisecond)
				defer tick.Stop()
				for {
					select {
					case <-stop:
						return
					case <-tick.C:
						pidMu.Lock()
						p := pid
						pidMu.Unlock()
						if rss := treeRSS(p); rss > chromePeak {
							chromePeak = rss
						}
						var ms runtime.MemStats
						runtime.ReadMemStats(&ms)
						if h := int64(ms.HeapInuse); h > heapPeak {
							heapPeak = h
						}
					}
				}
			}()
			started := time.Now()
			job := fx.render(t)
			elapsed := time.Since(started)
			close(stop)
			wg.Wait()
			if job.Status != StatusSucceeded {
				t.Fatalf("%s run %d: %+v %+v", transport, run, job, job.Error)
			}
			seqs = append(seqs, hashes.sequence())
			span := lastFrame.Sub(firstFrame).Seconds()
			mp4, _ := fx.pub.get(job.Output.Path)
			r := row{Transport: transport, Frames: job.FramesTotal, RenderFPS: float64(job.FramesTotal) / span, JobSeconds: elapsed.Seconds(),
				Sequence: seqs[run], ChromePeak: chromePeak >> 20, GoHeapPeak: heapPeak >> 20, MP4Bytes: len(mp4), FrameBytes: totalBytes / int64(max(job.FramesTotal, 1)),
				Environment: fmt.Sprintf("%s/%s %d CPU", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())}
			t.Logf("%s run %d: %d frames, %.1f fps, job %.1fs, chrome peak %d MB, go heap peak %d MB, %d bytes/frame", transport, run, r.Frames, r.RenderFPS, r.JobSeconds, r.ChromePeak, r.GoHeapPeak, r.FrameBytes)
			if run == 0 || r.RenderFPS < best.RenderFPS {
				best = r // the slower run is the honest figure
			}
		}
		best.Identical = len(seqs) < 2 || seqs[0] == seqs[1]
		rows = append(rows, best)
	}
	for _, r := range rows[1:] {
		t.Logf("pixels of %s equal to %s: %v", r.Transport, rows[0].Transport, r.Sequence == rows[0].Sequence)
	}
	out, _ := json.MarshalIndent(rows, "", "  ")
	t.Logf("spike result:\n%s", out)
	if path := os.Getenv("AURAGO_STORYMAKER_SPIKE_OUT"); path != "" {
		_ = os.WriteFile(path, out, 0o644)
	}
}

// treeRSS sums the resident memory of a process and its descendants.
func treeRSS(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0
	}
	var total int64
	var walk func(p *process.Process)
	walk = func(p *process.Process) {
		if mem, err := p.MemoryInfo(); err == nil && mem != nil {
			total += int64(mem.RSS)
		}
		if kids, err := p.Children(); err == nil {
			for _, k := range kids {
				walk(k)
			}
		}
	}
	walk(root)
	return total
}

var _ = storymaker.Schema

// patchedUI returns the ui tree with host.bundle.js changed by edit, to try a change to the player
// without touching its files.
type patchedUI struct {
	fs.FS
	edit func([]byte) []byte
}

func (p patchedUI) Open(name string) (fs.File, error) {
	if name != "js/storymaker/host.bundle.js" {
		return p.FS.Open(name)
	}
	data, err := fs.ReadFile(p.FS, name)
	if err != nil {
		return nil, err
	}
	return &memFile{name: path.Base(name), r: bytes.NewReader(p.edit(data)), size: int64(len(p.edit(data)))}, nil
}

type memFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *memFile) Stat() (fs.FileInfo, error) { return memInfo{f}, nil }
func (f *memFile) Read(b []byte) (int, error) { return f.r.Read(b) }
func (f *memFile) Close() error               { return nil }

type memInfo struct{ f *memFile }

func (i memInfo) Name() string       { return i.f.name }
func (i memInfo) Size() int64        { return i.f.size }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }

// recommendedPlayerChanges are the two changes to the player the plan asks part C for: frames as
// CORS-simple requests (no preflight per frame) and a render-mode frame deadline from boot.json
// (frame_deadline_ms) instead of the preview's 250 ms. Until C lands them the spike applies them to
// the bundle text in memory.
func recommendedPlayerChanges(src []byte) []byte {
	src = simpleFrameRequests(src)
	return bytes.Replace(src, []byte("canvas.height = boot.height;\n"),
		[]byte("canvas.height = boot.height;\n            if (boot.frame_deadline_ms > 0) env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);\n"), 1)
}

// simpleFrameRequests makes the page send frames as CORS-simple requests (text/plain, frame number in
// the query), which Chrome does not preflight.
func simpleFrameRequests(src []byte) []byte {
	return bytes.Replace(src,
		[]byte("post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) })"),
		[]byte("post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' })"), 1)
}

// TestGuardAndRequestOverheadSpike times the png transport with the protection layers switched off
// one by one, and with the two player changes the plan asks for (AURAGO_STORYMAKER_SPIKE=1). It is
// the evidence for the hybrid guard and for frames as CORS-simple requests.
func TestGuardAndRequestOverheadSpike(t *testing.T) {
	chrome := browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_SPIKE") != "1" {
		t.Skip("set AURAGO_STORYMAKER_SPIKE=1")
	}
	type variant struct {
		name  string
		edit  func([]byte) []byte
		tweak func(o *sessionOptions)
	}
	for i, v := range []variant{
		{"preflight per frame, hybrid guard", func(b []byte) []byte { return b }, nil},
		{"simple requests, hybrid guard", simpleFrameRequests, nil},
		{"simple requests, full guard", simpleFrameRequests, func(o *sessionOptions) { o.GuardFull = true }},
		{"simple requests, no guard", simpleFrameRequests, func(o *sessionOptions) { o.NoGuard = true }},
		{"simple requests, no proxy, no guard", simpleFrameRequests, func(o *sessionOptions) { o.ProxyAddr, o.NoGuard = "", true }},
	} {
		var first, last time.Time
		var mu sync.Mutex
		var opened *session
		fx := newRealFixture(t, chrome)
		fx.settings.TimeoutSeconds = 3600
		fx.deps.UI = patchedUI{FS: fx.deps.UI, edit: v.edit}
		fx.deps.SessionTweak = v.tweak
		fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
			p, err := openSession(ctx, o)
			opened, _ = p.(*session)
			return p, err
		}
		fx.deps.ListenerObserver = func(c map[string]int64) { t.Logf("variant %d listener requests: %v", i, c) }
		fx.deps.FrameObserver = func(n int, tr string, data []byte) {
			mu.Lock()
			if first.IsZero() {
				first = time.Now()
			}
			last = time.Now()
			mu.Unlock()
		}
		fx.rebuild(t)
		fx.loadFirewall(t)
		job := fx.render(t)
		if job.Status != StatusSucceeded {
			t.Fatalf("variant %d: %+v %+v", i, job, job.Error)
		}
		t.Logf("variant %d %s: %d frames in %.1fs = %.1f fps (guard %q: paused %d, blocked %d, workers %d guarded / %d failed)", i, v.name, job.FramesTotal, last.Sub(first).Seconds(),
			float64(job.FramesTotal)/last.Sub(first).Seconds(), opened.guard.Mode(), opened.guard.Seen(), opened.guard.Blocked(), opened.guard.workers.Load(), opened.guard.workerFailed.Load())
	}
}

// TestPinnedChromiumDownloadsVerifiesAndLaunches is the acceptance test of the pinned Chromium on a
// host that has none installed (aurago-test, a container): the first export's path through
// Browsers.Ensure with no configured path and no system browser. It downloads about 190 MB from the
// snapshot host, verifies the SHA-256, unpacks, and starts the browser headless with its sandbox.
// Run it with AURAGO_STORYMAKER_PINNED=1 (and AURAGO_RUN_BROWSER_SMOKE=1); it hides the system
// browser from the lookup so the pinned one is the only candidate.
func TestPinnedChromiumDownloadsVerifiesAndLaunches(t *testing.T) {
	browserSmoke(t)
	if os.Getenv("AURAGO_STORYMAKER_PINNED") != "1" {
		t.Skip("set AURAGO_STORYMAKER_PINNED=1 to download and start the pinned Chromium")
	}
	b := NewBrowsers(t.TempDir())
	b.lookPath = func() (string, bool) { return "", false }
	if st := b.Status(context.Background(), ""); st.Ready || !st.CanDownload || st.Source != SourcePinned {
		t.Fatalf("status before the download: %+v", st)
	}
	var last int64
	started := time.Now()
	st, bin := b.Ensure(context.Background(), "", func(done, total int64) { last = done })
	if !st.Ready || st.Source != SourcePinned || bin == "" {
		t.Fatalf("Ensure: %+v %q", st, bin)
	}
	t.Logf("pinned Chromium %s ready after %v (%d MB downloaded): %s", st.Version, time.Since(started).Round(time.Second), last>>20, bin)
	if again := b.Status(context.Background(), ""); !again.Ready {
		t.Fatalf("status after the install: %+v", again)
	}
	// The player must run on the pinned revision (Chromium 128) as well: export the firewall story with it.
	fx := newRealFixture(t, bin)
	fx.loadFirewall(t)
	fx.rebuild(t)
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("export with the pinned Chromium: %+v error %+v", job, job.Error)
	}
}
