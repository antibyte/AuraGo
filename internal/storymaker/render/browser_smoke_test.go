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
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/videostudio"

	"github.com/go-rod/rod/lib/launcher"
	gnet "github.com/shirou/gopsutil/v4/net"
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
	fx.seedStoredNarration(t)
}

// seedStoredNarration stores generated audio for every missing piece of the project's story, because Prepare refuses
// a narrated story without it. With 2 s per piece firewall.json stays above the 250 frames TestRealRenderOfTheFirewallStory pins:
// s1 139 frames, s2 at least 85, s3 150, minus two 15-frame transition overlaps.
func (fx *runFixture) seedStoredNarration(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	data, _, err := fx.projects.LoadStory(ctx, fx.projectID)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, &story, 2); err != nil {
		t.Fatal(err)
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
					pid = s.proc.PID()
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

// recommendedPlayerChanges are the two changes of task B15 to the player: frames as CORS-simple
// requests (no preflight per frame) and a render-mode frame deadline from boot.json
// (frame_deadline_ms) instead of the preview's 250 ms. Before B15 the spike applies them to the
// bundle text in memory; on a player that has them they change nothing.
func recommendedPlayerChanges(src []byte) []byte {
	src = simpleFrameRequests(src)
	if bytes.Contains(src, []byte("boot.frame_deadline_ms")) {
		return src
	}
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

// preflightFrameRequests is the reverse of simpleFrameRequests: the page sends frames with the
// X-SM-Frame header, which Chrome preflights for an opaque origin (the player before task B15).
func preflightFrameRequests(src []byte) []byte {
	return bytes.Replace(src,
		[]byte("post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' })"),
		[]byte("post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) })"), 1)
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
		{"preflight per frame, hybrid guard", preflightFrameRequests, nil},
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

// pageEndSpy keeps the last __smStatus of the render page. execute closes the page when the job ends, so
// the wrapper reads the status first.
type pageEndSpy struct {
	mu   sync.Mutex
	last string
}

type pageEndPage struct {
	browserPage
	spy *pageEndSpy
}

func (p *pageEndPage) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	status := p.browserPage.Status(ctx)
	cancel()
	p.spy.mu.Lock()
	p.spy.last = status
	p.spy.mu.Unlock()
	p.browserPage.Close()
}

// end decodes what the parallel path leaves in __smStatus. A serial render reports no workers.
func (s *pageEndSpy) end(t *testing.T) (workers, restarts int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var doc struct {
		Workers  int `json:"workers"`
		Restarts int `json:"restarts"`
	}
	if err := json.Unmarshal([]byte(s.last), &doc); err != nil {
		t.Fatalf("page status %q: %v", s.last, err)
	}
	return doc.Workers, doc.Restarts
}

// newParallelFixture gives the stated CPU count enough memory to exercise the pool.
func newParallelFixture(t *testing.T, chrome string, cpus int, observer func(int, string, []byte)) (*runFixture, *pageEndSpy) {
	t.Helper()
	fx := newRealFixture(t, chrome)
	fx.settings.LogicalCPUs = cpus
	fx.settings.FreeMemoryMB = 16384
	fx.settings.TimeoutSeconds = 600
	fx.deps.FrameObserver = observer
	spy := &pageEndSpy{}
	fx.deps.OpenPage = func(ctx context.Context, o sessionOptions) (browserPage, error) {
		p, err := openSession(ctx, o)
		if err != nil {
			return nil, err
		}
		return &pageEndPage{browserPage: p, spy: spy}, nil
	}
	fx.rebuild(t)
	return fx, spy
}

// loadParallelStory saves a story fixture, gives each code scene the s3 source and seeds the narration, so
// the narration_incomplete gate passes. SeedNarration writes missing pieces only.
func (fx *runFixture) loadParallelStory(t *testing.T, name string) storymaker.Story {
	t.Helper()
	ctx := context.Background()
	lf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	data, err := os.ReadFile("../testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, lf(data), ""); err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(lf(data), fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range story.Scenes {
		if sc.Kind != "code" {
			continue
		}
		if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, sc.ID, lf(src), ""); err != nil || len(res.Errors) != 0 {
			t.Fatalf("SaveCodeScene %s: %v %+v", sc.ID, err, res)
		}
	}
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, &story, 1.5); err != nil {
		t.Fatal(err)
	}
	return story
}

// requireSpeechCoverage keeps the parity test honest: the speech fixture must hold a bubble and a sliding
// text in the CJK face, the case where a wrap could depend on the frames a worker drew before.
func requireSpeechCoverage(t *testing.T, fx *runFixture, story *storymaker.Story) {
	t.Helper()
	glyphs := fx.deps.Catalog.Glyphs()
	if glyphs == nil {
		t.Fatal("the real catalog names no glyphs file")
	}
	bubble, sliding := false, false
	for _, sc := range story.Scenes {
		for _, b := range sc.Beats {
			if b.Do == "say" && b.Text != "" {
				bubble = true
			}
			if b.Do != "show" || b.Entry != "slide" {
				continue
			}
			for _, r := range b.Text {
				if i := glyphs.FaceOf(r); i >= 0 && glyphs.Faces[i].ID == "sm-cjk" {
					sliding = true
				}
			}
		}
	}
	if !bubble {
		t.Fatal("speech.json has no say with text, so the parity test would not cover a bubble")
	}
	if !sliding {
		t.Fatal("speech.json has no sliding text in the CJK face, so the parity test would not cover the layout memo at moving widths")
	}
}

type parallelRun struct {
	hashes            *frameHashes
	workers, restarts int
}

func renderParallelStory(t *testing.T, chrome, name string, cpus int) parallelRun {
	t.Helper()
	hashes := newFrameHashes()
	fx, spy := newParallelFixture(t, chrome, cpus, hashes.observe(t))
	story := fx.loadParallelStory(t, name)
	if name == "speech" {
		requireSpeechCoverage(t, fx, &story)
	}
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("%s on %d CPUs: %+v %+v", name, cpus, job, job.Error)
	}
	if hashes.count() != job.FramesTotal {
		t.Fatalf("%s on %d CPUs: observed %d of %d frames", name, cpus, hashes.count(), job.FramesTotal)
	}
	workers, restarts := spy.end(t)
	return parallelRun{hashes: hashes, workers: workers, restarts: restarts}
}

// firstFrameDifference is the first frame whose decoded RGBA differs, or -1.
func firstFrameDifference(a, b *frameHashes) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	for n := 0; n < max(len(a.hashes), len(b.hashes)); n++ {
		if a.hashes[n] != b.hashes[n] {
			return n
		}
	}
	return -1
}

func distinctFrames(f *frameHashes) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	for _, h := range f.hashes {
		seen[h] = true
	}
	return len(seen)
}

func TestParallelParityBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	old := renderPreRoll
	t.Cleanup(func() { renderPreRoll = old })
	// 14 CPUs give floor((14-2)/3) = 4 workers with no memory bound. 1 CPU clamps to 1, the serial path.
	for _, name := range []string{"firewall", "speech"} {
		t.Run(name, func(t *testing.T) {
			renderPreRoll = 0
			serial := renderParallelStory(t, chrome, name, 1)
			if serial.workers != 0 {
				t.Fatalf("the serial run reports %d pool workers", serial.workers)
			}
			if distinctFrames(serial.hashes) < 2 {
				t.Fatalf("all frames of %s are the same, the comparison proves nothing", name)
			}
			for _, preRoll := range []int{0, 2} {
				renderPreRoll = preRoll
				run := renderParallelStory(t, chrome, name, 14)
				if run.workers != 4 {
					t.Fatalf("pre-roll %d: the page ran %d pool workers, want 4", preRoll, run.workers)
				}
				if at := firstFrameDifference(serial.hashes, run.hashes); at >= 0 {
					t.Errorf("pre-roll %d: frame %d differs from the serial render", preRoll, at)
				}
			}
		})
	}
}

func TestParallelEncoderFailureBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	var mu sync.Mutex
	arrivals := map[int]int{}
	hashes := newFrameHashes()
	observe := hashes.observe(t)
	// 14 CPUs give floor((14-2)/3) = 4 workers.
	fx, spy := newParallelFixture(t, chrome, 14, func(n int, transport string, data []byte) {
		mu.Lock()
		arrivals[n]++
		mu.Unlock()
		observe(n, transport, data)
	})
	// The missing device makes the hardware encoder fail once. recoverEncoder then restarts with libx264.
	fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	fx.settings.EncoderName = videostudio.CodecVAAPI
	fx.loadParallelStory(t, "firewall")
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	workers, restarts := spy.end(t)
	if workers != 4 || restarts != 1 {
		t.Fatalf("the page ran %d pool workers and %d restarts, want 4 and 1", workers, restarts)
	}
	mu.Lock()
	first := arrivals[0]
	mu.Unlock()
	// One failure, one restart: frame 0 reaches Go once before the restart and once after it.
	if first != 2 {
		t.Errorf("frame 0 arrived %d times, want 2", first)
	}
	if hashes.count() != job.FramesTotal {
		t.Errorf("observed %d of %d frames", hashes.count(), job.FramesTotal)
	}
	meta, _ := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !strings.Contains(string(meta), `"encoder": "libx264"`) {
		t.Errorf("record = %s", meta)
	}
}

// The render browser must not open a DevTools port. Such a port listens on loopback, which every
// account of the host can reach: GET /json/list names the render URL (and with it the secret prefix of
// the listener), and the websocket drives the browser, past the page CSP and the CDP guard. Chrome
// writes DevToolsActivePort into the profile when it opens a port; the pipe transport has none.
func TestRenderBrowserOpensNoDevToolsPort(t *testing.T) {
	chrome := browserSmoke(t)
	profile := filepath.Join(t.TempDir(), "profile")
	s, err := openRodSession(context.Background(), sessionOptions{Bin: chrome, Profile: profile, Base: "http://127.0.0.1:1/x", Width: 640, Height: 360})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer s.Close()
	res, err := s.page.Timeout(30 * time.Second).Eval(`() => 6 * 7`)
	if err != nil || res.Value.Int() != 42 {
		t.Fatalf("the tab does not run scripts: %v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(profile, "DevToolsActivePort")); !os.IsNotExist(err) {
		t.Fatalf("the browser opened a DevTools port (DevToolsActivePort: %v)", err)
	}
	if listening := listeningSockets(s.proc.PID()); len(listening) != 0 {
		t.Fatalf("the browser tree listens on %v", listening)
	}
}

// listeningSockets lists the TCP sockets of a process and its descendants that wait for connections: in
// LISTEN state, or without a remote end (the state column of the Windows socket table is unreliable).
func listeningSockets(pid int) []string {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	tree := map[int32]bool{}
	var walk func(p *process.Process)
	walk = func(p *process.Process) {
		tree[p.Pid] = true
		if kids, err := p.Children(); err == nil {
			for _, k := range kids {
				walk(k)
			}
		}
	}
	walk(root)
	all, err := gnet.Connections("tcp")
	if err != nil {
		return []string{"the socket table cannot be read: " + err.Error()}
	}
	var out []string
	for _, c := range all {
		if tree[c.Pid] && (c.Status == "LISTEN" || c.Raddr.Port == 0) {
			out = append(out, fmt.Sprintf("%s:%d (pid %d)", c.Laddr.IP, c.Laddr.Port, c.Pid))
		}
	}
	return out
}
