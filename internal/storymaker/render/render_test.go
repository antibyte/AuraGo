package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/videostudio"
)

const uiDirForTests = "../../../ui"

// memPublisher is the desktop workspace of the tests: create-only streams and replaceable small files.
type memPublisher struct {
	mu    sync.Mutex
	files map[string][]byte
	fail  error
}

func newMemPublisher() *memPublisher { return &memPublisher{files: map[string][]byte{}} }

func (p *memPublisher) WriteStream(_ context.Context, path string, r io.Reader, max int64) error {
	if p.fail != nil {
		return p.fail
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.files[path]; exists {
		return errors.New("exists")
	}
	p.files[path] = data
	return nil
}

func (p *memPublisher) WriteBytes(_ context.Context, path string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.files[path] = append([]byte(nil), data...)
	return nil
}

func (p *memPublisher) get(path string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, ok := p.files[path]
	return d, ok
}

func (p *memPublisher) names() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for k := range p.files {
		out = append(out, k)
	}
	return out
}

// fakePage plays the render page against the listener over plain HTTP.
type fakePage struct {
	script  func(ctx context.Context, base string)
	crashed chan struct{}
	closed  atomic.Bool
	capture func(n int) []byte
}

func (p *fakePage) Navigate(ctx context.Context, u string) error {
	go p.script(ctx, strings.TrimSuffix(u, "/"))
	return nil
}
func (p *fakePage) Status(context.Context) string   { return `{"phase":"fake"}` }
func (p *fakePage) WaitReady(context.Context) error { return nil }
func (p *fakePage) Crashed() <-chan struct{}        { return p.crashed }
func (p *fakePage) Close()                          { p.closed.Store(true) }
func (p *fakePage) Capture(_ context.Context, n int) ([]byte, error) {
	if p.capture == nil {
		return nil, errors.New("no capture")
	}
	return p.capture(n), nil
}

// playOptions steer the fake page.
type playOptions struct {
	frames      int
	frame       func(n int) (contentType string, body []byte)
	mutateRep   func(rep map[string]any)
	stallAfter  int // stop sending after this many frames (0 = never)
	errorAfter  int // POST /error after this many frames (0 = never)
	errorBody   string
	onRefused   func(answer ReportAnswer)
	onRestart   func()
	framesSent  *atomic.Int64
	skipReport  bool
	reportTwice bool
}

func httpDo(ctx context.Context, method, url, contentType string, header map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return data, res.StatusCode, nil
}

func play(t *testing.T, o playOptions) func(ctx context.Context, base string) {
	return func(ctx context.Context, base string) {
		bootBytes, status, err := httpDo(ctx, "GET", base+"/boot.json", "", nil, nil)
		if err != nil || status != 200 {
			return
		}
		var boot struct {
			FPS, Width, Height int
			Transport          string
			Token              string
		}
		_ = json.Unmarshal(bootBytes, &boot)
		var rawBoot map[string]any
		_ = json.Unmarshal(bootBytes, &rawBoot)
		catalogBytes, _, _ := httpDo(ctx, "GET", base+"/catalog/catalog.json", "", nil, nil)
		var cat struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(catalogBytes, &cat)
		storyBytes, _, _ := httpDo(ctx, "GET", base+"/story.json", "", nil, nil)
		var story struct {
			Scenes []struct {
				ID string `json:"id"`
			} `json:"scenes"`
		}
		_ = json.Unmarshal(storyBytes, &story)
		// Scene i starts i frames in and every scene ends with the video, so the ranges move forward.
		scenes := []map[string]any{}
		for i := range story.Scenes {
			scenes = append(scenes, map[string]any{"id": story.Scenes[i].ID, "start_frame": i, "end_frame": o.frames, "kind": "storyboard"})
		}
		rep := map[string]any{"frame_count": o.frames, "fps": boot.FPS, "width": boot.Width, "height": boot.Height,
			"duration_ms": o.frames * 1000 / max(boot.FPS, 1), "catalog_version": cat.Version, "scenes": scenes,
			"audio_events": []any{}, "captions": []any{}, "checks": []any{}, "unverified": []any{}, "issues": []any{}}
		if o.mutateRep != nil {
			o.mutateRep(rep)
		}
		body, _ := json.Marshal(rep)
		if !o.skipReport {
			out, st, err := httpDo(context.WithoutCancel(ctx), "POST", base+"/report?t="+boot.Token, "application/json", nil, body)
			if err != nil || st != 200 {
				return
			}
			var answer ReportAnswer
			_ = json.Unmarshal(out, &answer)
			if o.reportTwice {
				out2, _, _ := httpDo(ctx, "POST", base+"/report?t="+boot.Token, "application/json", nil, body)
				var second ReportAnswer
				_ = json.Unmarshal(out2, &second)
				if second.Start {
					t.Error("a second report was accepted")
				}
			}
			if !answer.Start {
				if o.onRefused != nil {
					o.onRefused(answer)
				}
				return
			}
		}
		if boot.Transport == TransportScreenshot {
			<-ctx.Done() // the Go side drives the screenshots
			return
		}
		for n := 0; n < o.frames; {
			if o.stallAfter > 0 && n >= o.stallAfter {
				<-ctx.Done()
				return
			}
			if o.errorAfter > 0 && n >= o.errorAfter {
				httpDo(ctx, "POST", base+"/error?t="+boot.Token, "application/json", nil, []byte(o.errorBody))
				<-ctx.Done()
				return
			}
			ct, data := o.frame(n)
			out, st, err := httpDo(ctx, "POST", base+"/frame?t="+boot.Token, ct, map[string]string{"X-SM-Frame": strconv.Itoa(n)}, data)
			if err != nil {
				return
			}
			if o.framesSent != nil {
				o.framesSent.Add(1)
			}
			var a FrameAnswer
			_ = json.Unmarshal(out, &a)
			switch {
			case a.Stop || st >= 400 && !a.Restart:
				return
			case a.Restart:
				if o.onRestart != nil {
					o.onRestart()
				}
				n = 0
			case a.Done:
				return
			default:
				n++
			}
		}
	}
}

type runFixture struct {
	t         *testing.T
	dir       string
	deps      Deps
	runner    *Runner
	mgr       *Manager
	projects  *storymaker.ProjectStore
	pub       *memPublisher
	projectID string
	settings  Settings
	page      *fakePage
	pageSetup func(p *fakePage)
	pngFrame  []byte
}

func newRunFixture(t *testing.T) *runFixture {
	t.Helper()
	ffmpeg := testBinary(t, "ffmpeg")
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	fx := &runFixture{t: t, dir: t.TempDir(), pub: newMemPublisher()}
	fx.projects = storymaker.NewProjectStore(storymaker.NewMemStore(), ProjectsRoot)
	proj, _, _, err := fx.projects.Create(context.Background(), "Was ist eine Firewall?", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	fx.projectID = proj.ID
	lim := storymaker.DefaultLimits()
	fx.settings = Settings{TimeoutSeconds: 60, MaxDurationSeconds: 900, FFmpegPath: ffmpeg, EncoderName: "libx264", Limits: lim}
	b := NewBrowsers(fx.dir)
	b.lookPath = func() (string, bool) { return "/fake/chrome", true }
	b.launchProbe = func(context.Context, string) (string, error) { return "Fake/1", nil }
	fx.page = &fakePage{crashed: make(chan struct{})}
	fx.deps = Deps{
		Dir: fx.dir, UI: ui, Catalog: cat, Projects: fx.projects, Browsers: b, Publisher: fx.pub,
		Settings: func(context.Context) (Settings, error) { return fx.settings, nil },
		OpenPage: func(ctx context.Context, o sessionOptions) (browserPage, error) {
			if fx.pageSetup != nil {
				fx.pageSetup(fx.page)
			}
			if !strings.HasPrefix(o.ProxyAddr, "127.0.0.1:") || o.Width != 1920 || o.Height != 1080 || !strings.HasPrefix(o.Base, "http://127.0.0.1:") {
				t.Errorf("session options = %+v", o)
			}
			return fx.page, nil
		},
	}
	fx.runner = NewRunner(fx.deps)
	mgr, err := NewManager(ManagerOptions{Dir: fx.dir, Run: fx.runner.Run})
	if err != nil {
		t.Fatal(err)
	}
	fx.mgr = mgr
	t.Cleanup(mgr.Close)
	fx.pngFrame = solidPNG(t, 1920, 1080, color.NRGBA{R: 30, G: 90, B: 160, A: 255})
	return fx
}

func (fx *runFixture) playing(o playOptions) {
	if o.frames == 0 {
		o.frames = 24
	}
	if o.frame == nil {
		o.frame = func(int) (string, []byte) { return "image/png", fx.pngFrame }
	}
	fx.page.script = play(fx.t, o)
}

func (fx *runFixture) render(t *testing.T) Job {
	t.Helper()
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	job, _, err := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	if err != nil {
		t.Fatal(err)
	}
	return waitJobFor(t, fx.mgr, job.ID, terminal, 8*time.Minute)
}

func TestRunRendersAndPublishesTheVideo(t *testing.T) {
	ffprobe := testBinary(t, "ffprobe")
	fx := newRunFixture(t)
	var sent atomic.Int64
	fx.playing(playOptions{frames: 24, framesSent: &sent})
	job := fx.render(t)
	if job.Status != StatusSucceeded || job.Output == nil || job.Error != nil {
		t.Fatalf("job = %+v (error %+v)", job, job.Error)
	}
	wantPath := "Documents/Storymaker/" + fx.projectID + "/exports/Was-ist-eine-Firewall-" + job.ID[:8] + ".mp4"
	if job.Output.Path != wantPath {
		t.Fatalf("output path = %q, want %q", job.Output.Path, wantPath)
	}
	mp4, ok := fx.pub.get(wantPath)
	if !ok || int64(len(mp4)) != job.Output.Size || len(mp4) == 0 {
		t.Fatalf("published file: ok=%v size=%d output=%+v", ok, len(mp4), job.Output)
	}
	if job.FramesDone != 24 || job.FramesTotal != 24 || job.Progress != 1 {
		t.Errorf("progress = %+v", job)
	}
	meta, ok := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !ok {
		t.Fatal("no export record")
	}
	var rec map[string]any
	if err := json.Unmarshal(meta, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["job_id"] != job.ID || rec["output_path"] != wantPath || rec["frames"] != float64(24) || rec["encoder"] != "libx264" || rec["transport"] != "png" || rec["title"] != "Was ist eine Firewall?" {
		t.Errorf("record = %v", rec)
	}
	tmp := filepath.Join(t.TempDir(), "check.mp4")
	if err := os.WriteFile(tmp, mp4, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := ffprobeFile(t, ffprobe, tmp)
	okVideo := false
	for _, s := range doc.Streams {
		if s.CodecType == "video" && s.Width == 1920 && s.Height == 1080 && s.AvgFrameRate == "30/1" && s.NbFrames == "24" {
			okVideo = true
		}
	}
	if !okVideo {
		t.Errorf("streams = %+v", doc.Streams)
	}
	if !fx.page.closed.Load() {
		t.Error("the browser page was not closed")
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
	// The stored job holds a prefix hash and never a prefix.
	raw, _ := os.ReadFile(filepath.Join(fx.dir, "jobs.json"))
	if h := fx.mgr.PrefixHash(job.ID); len(h) != 64 || !strings.Contains(string(raw), h) {
		t.Errorf("prefix hash = %q", h)
	}
	if strings.Contains(string(raw), "127.0.0.1") {
		t.Error("jobs.json mentions the listener address")
	}
}

func TestRunRefusesReportsThatDisagreeWithTheJob(t *testing.T) {
	cases := map[string]struct {
		mut  func(rep map[string]any)
		code string
		text string
	}{
		"catalog version": {func(rep map[string]any) { rep["catalog_version"] = "deadbeef0000" }, CodeRenderFailed, "catalog version"},
		"frame limit": {func(rep map[string]any) {
			rep["frame_count"] = 27001
			rep["scenes"].([]map[string]any)[0]["end_frame"] = 27001
		}, CodeInvalidStory, "limit"},
		"fps":               {func(rep map[string]any) { rep["fps"] = 24 }, CodeRenderFailed, "24 fps"},
		"canvas":            {func(rep map[string]any) { rep["width"] = 1280 }, CodeRenderFailed, "1280x"},
		"scene id":          {func(rep map[string]any) { rep["scenes"].([]map[string]any)[0]["id"] = "other" }, CodeRenderFailed, "other"},
		"scene count":       {func(rep map[string]any) { rep["scenes"] = []map[string]any{} }, CodeRenderFailed, "0 scenes"},
		"range":             {func(rep map[string]any) { rep["scenes"].([]map[string]any)[0]["end_frame"] = 0 }, CodeRenderFailed, "invalid range"},
		"short frame count": {func(rep map[string]any) { rep["frame_count"] = 10 }, CodeRenderFailed, "invalid range"},
		"zero frames":       {func(rep map[string]any) { rep["frame_count"] = 0 }, CodeRenderFailed, "no frames"},
		"error issue": {func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "unknown_asset", "message": "no such asset"}}
		}, CodeInvalidStory, "no such asset"},
		"code error issue": {func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "code_error", "message": "scene threw"}}
		}, CodeCodeError, "scene threw"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newRunFixture(t)
			var refused ReportAnswer
			told := make(chan struct{})
			fx.playing(playOptions{frames: 24, mutateRep: tc.mut, onRefused: func(a ReportAnswer) { refused = a; close(told) }})
			job := fx.render(t)
			select {
			case <-told:
			case <-time.After(10 * time.Second):
				t.Fatal("the page was never told about the refusal")
			}
			if job.Status != StatusFailed || job.Error == nil || job.Error.Code != tc.code || !strings.Contains(job.Error.Message, tc.text) {
				t.Fatalf("job = %+v error %+v", job, job.Error)
			}
			if refused.Start || refused.Error != tc.code {
				t.Errorf("the page was told %+v", refused)
			}
			if len(fx.pub.names()) != 0 {
				t.Errorf("files published for a refused report: %v", fx.pub.names())
			}
		})
	}
}

func TestRunRefusesASecondReport(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 12, reportTwice: true})
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunFailsWhenNoFrameArrivesAfterStart(t *testing.T) {
	old := noFrameWatchdog
	noFrameWatchdog = 600 * time.Millisecond
	t.Cleanup(func() { noFrameWatchdog = old })
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24, stallAfter: 3})
	started := time.Now()
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeCodeError || !strings.Contains(job.Error.Message, "no frame") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if time.Since(started) > 20*time.Second {
		t.Fatal("the watchdog was slow")
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}

// The watchdog is for a page that stopped producing frames. The time Go spends on a frame itself (the write
// to FFmpeg, which may take up to 30 s, the settle of a hardware encoder, the software restart and the start
// of the replacement process) passes while the page waits for the answer and must not count against it.
func TestRunWatchdogIgnoresTheTimeGoSpendsOnAFrame(t *testing.T) {
	old := noFrameWatchdog
	noFrameWatchdog = 800 * time.Millisecond
	t.Cleanup(func() { noFrameWatchdog = old })
	cases := []struct {
		name      string
		hardware  bool
		slowFrame int // the frame whose handling takes longer than the watchdog
		slowCall  int // which arrival of that frame is slow (2 is the first frame after the restart)
	}{
		{"a slow frame", false, 5, 1},
		{"the first frame after the software restart", true, 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newRunFixture(t)
			if tc.hardware {
				fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
				fx.settings.EncoderName = videostudio.CodecVAAPI
			}
			var mu sync.Mutex
			arrivals := map[int]int{}
			var slowed atomic.Bool
			fx.deps.FrameObserver = func(n int, _ string, _ []byte) {
				mu.Lock()
				arrivals[n]++
				slow := n == tc.slowFrame && arrivals[n] == tc.slowCall
				mu.Unlock()
				if slow {
					slowed.Store(true)
					time.Sleep(2 * noFrameWatchdog)
				}
			}
			fx.rebuild(t)
			var restarts atomic.Int32
			fx.playing(playOptions{frames: 18, onRestart: func() { restarts.Add(1) }})
			job := fx.render(t)
			if job.Status != StatusSucceeded {
				t.Fatalf("job = %+v %+v", job, job.Error)
			}
			if !slowed.Load() {
				t.Fatal("the slow frame never arrived, the test proved nothing")
			}
			if want := int32(map[bool]int{false: 0, true: 1}[tc.hardware]); restarts.Load() != want {
				t.Errorf("the page was told to restart %d times, want %d", restarts.Load(), want)
			}
		})
	}
}

func TestRunTreatsAnyPageErrorAsFatal(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24, errorAfter: 4, errorBody: `{"code":"code_error","message":"boom in the scene","scene_id":"s1"}`})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeCodeError || !strings.Contains(job.Error.Message, "boom") || !strings.Contains(job.Error.Message, "s1") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	fx2 := newRunFixture(t)
	fx2.playing(playOptions{frames: 24, errorAfter: 2, errorBody: `{"code":"readiness","message":"fonts did not load"}`})
	job2 := fx2.render(t)
	if job2.Status != StatusFailed || job2.Error.Code != CodeRenderFailed {
		t.Fatalf("job2 = %+v %+v", job2, job2.Error)
	}
}

// failingPage fails the way the real page does: Status gives __smStatus, WaitReady quotes its message, and a Capture
// error quotes the exception of __smRenderFrame, whose stack names the bundle URL under the base.
type failingPage struct {
	fakePage
	status     string
	readyErr   error
	captureErr error
}

func (p *failingPage) Status(context.Context) string   { return p.status }
func (p *failingPage) WaitReady(context.Context) error { return p.readyErr }
func (p *failingPage) Capture(context.Context, int) ([]byte, error) {
	return nil, p.captureErr
}

func TestRunKeepsThePrefixAndTheTokenOutOfAPageFailure(t *testing.T) {
	old := reportWait
	reportWait = time.Millisecond
	t.Cleanup(func() { reportWait = old })
	prefix, token := strings.Repeat("a", 32), strings.Repeat("b", 32)
	text := "boom at http://127.0.0.1:9/" + prefix + "/js/storymaker/host.bundle.js?t=" + token
	newRun := func() *renderRun {
		return &renderRun{prefix: prefix, token: token, frames: 1, st: Settings{TimeoutSeconds: 60}, failCh: make(chan *JobError, 1)}
	}
	check := func(name, got string) {
		t.Helper()
		if strings.Contains(got, prefix) || strings.Contains(got, token) || !strings.Contains(got, "boom") {
			t.Errorf("%s: %q", name, got)
		}
	}
	for name, page := range map[string]*failingPage{
		"ready":   {readyErr: errors.New(text)},
		"capture": {captureErr: errors.New(text)},
	} {
		run := newRun()
		run.driveScreenshots(context.Background(), page)
		select {
		case je := <-run.failCh:
			check(name, je.Message)
		default:
			t.Errorf("%s: no failure", name)
		}
	}
	// No report arrives, so wait ends with the report timeout, and that message quotes __smStatus.
	if err := newRun().wait(context.Background(), &failingPage{status: text}); err == nil {
		t.Error("status: no failure")
	} else {
		check("status", err.Error())
	}
}

func TestRunRefusesBadFrames(t *testing.T) {
	fx := newRunFixture(t)
	wrong := solidPNG(t, 1280, 720, color.NRGBA{A: 255})
	fx.playing(playOptions{frames: 24, frame: func(n int) (string, []byte) {
		if n == 3 {
			return "image/png", wrong
		}
		return "image/png", fx.pngFrame
	}})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "frame_size") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunCancelStopsEverything(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 200, stallAfter: 5})
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	job, _, _ := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	waitJob(t, fx.mgr, job.ID, func(j Job) bool { return j.Status == StatusRunning && j.FramesDone >= 4 })
	fx.mgr.Cancel(job.ID)
	done := waitJob(t, fx.mgr, job.ID, terminal)
	if done.Status != StatusCancelled || done.Error == nil || done.Error.Code != CodeCancelled {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if !fx.page.closed.Load() {
		t.Error("the page was not closed")
	}
	if len(fx.pub.names()) != 0 {
		t.Errorf("a cancelled render published %v", fx.pub.names())
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}

func TestRunRestartsWithSoftwareWhenTheHardwareEncoderFails(t *testing.T) {
	fx := newRunFixture(t)
	fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	fx.settings.EncoderName = videostudio.CodecVAAPI
	var restarts atomic.Int32
	fx.playing(playOptions{frames: 18, onRestart: func() { restarts.Add(1) }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if restarts.Load() != 1 {
		t.Fatalf("the page was told to restart %d times", restarts.Load())
	}
	meta, _ := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !strings.Contains(string(meta), `"encoder": "libx264"`) {
		t.Fatalf("record = %s", meta)
	}
}

func TestRunTimesOut(t *testing.T) {
	old := measuredFPS[TransportPNG]
	measuredFPS[TransportPNG] = 100000
	t.Cleanup(func() { measuredFPS[TransportPNG] = old })
	fx := newRunFixture(t)
	fx.settings.TimeoutSeconds = 1
	fx.playing(playOptions{frames: 400, frame: func(n int) (string, []byte) {
		time.Sleep(60 * time.Millisecond)
		return "image/png", fx.pngFrame
	}})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderTimeout {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestRunReportsCapabilityFailures(t *testing.T) {
	fx := newRunFixture(t)
	fx.settings.FFmpegPath = ""
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeFFmpegUnavailable {
		t.Fatalf("no ffmpeg: %+v %+v", job, job.Error)
	}

	fx = newRunFixture(t)
	fx.deps.Browsers.lookPath = func() (string, bool) { return "", false }
	fx.deps.Browsers.pins = map[string]PinnedChromium{}
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeBrowserUnavailable || !strings.Contains(job.Error.Message, "browser_path") {
		t.Fatalf("no browser: %+v %+v", job, job.Error)
	}

	fx = newRunFixture(t)
	fx.pub.fail = errors.New("disk full")
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "disk full") {
		t.Fatalf("publish failure: %+v %+v", job, job.Error)
	}
}

func TestRunRendersRawAndScreenshotTransports(t *testing.T) {
	ffprobe := testBinary(t, "ffprobe")
	for _, transport := range []string{TransportRaw, TransportScreenshot} {
		t.Run(transport, func(t *testing.T) {
			fx := newRunFixture(t)
			fx.settings.Transport = transport
			raw := bytes.Repeat([]byte{20, 60, 120, 255}, 1920*1080)
			fx.page.capture = func(int) []byte { return fx.pngFrame }
			fx.playing(playOptions{frames: 10, frame: func(int) (string, []byte) { return "application/octet-stream", raw }})
			job := fx.render(t)
			if job.Status != StatusSucceeded {
				t.Fatalf("job = %+v %+v", job, job.Error)
			}
			mp4, _ := fx.pub.get(job.Output.Path)
			tmp := filepath.Join(t.TempDir(), "t.mp4")
			_ = os.WriteFile(tmp, mp4, 0o600)
			doc := ffprobeFile(t, ffprobe, tmp)
			ok := false
			for _, s := range doc.Streams {
				ok = ok || (s.CodecType == "video" && s.NbFrames == "10")
			}
			if !ok {
				t.Fatalf("streams = %+v", doc.Streams)
			}
		})
	}
}

func TestRenderTimeoutFormula(t *testing.T) {
	// 15 minutes at 30 fps with the png rate must fit: 27000 frames x 3 / 6 fps = 13500 s.
	if got := RenderTimeout(3600, 27000, TransportPNG); got != 13500*time.Second {
		t.Errorf("15 minute png = %v", got)
	}
	if got := RenderTimeout(3600, 300, TransportPNG); got != 3600*time.Second {
		t.Errorf("short video keeps the configured timeout, got %v", got)
	}
	if got := RenderTimeout(60, 0, "unknown"); got != 60*time.Second {
		t.Errorf("zero frames = %v", got)
	}
}

func TestPrepareChecksTheStoryLikeSaveDoes(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil || plan.StoryETag == "" || len(plan.StoryJSON) == 0 || plan.Story.Scenes[0].Environment == "" {
		t.Fatalf("Prepare = %+v, %v", plan, err)
	}
	// A stale etag is reported as such.
	if _, err := fx.runner.Prepare(ctx, fx.projectID, `"stale"`, fx.settings); err == nil {
		t.Fatal("a stale etag was accepted")
	} else if pe := new(PrepareError); !errors.As(err, &pe) || !pe.Stale {
		t.Fatalf("stale error = %v", err)
	}
	// The current etag passes.
	if _, err := fx.runner.Prepare(ctx, fx.projectID, plan.StoryETag, fx.settings); err != nil {
		t.Fatalf("current etag: %v", err)
	}
	// A story with an error issue is refused with its issues.
	bad := `{"schema":"storymaker/1","title":"Bad","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(bad), ""); err != nil {
		t.Fatal(err)
	}
	_, err = fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	var pe *PrepareError
	if !errors.As(err, &pe) || pe.Code != CodeInvalidStory || len(pe.Issues) == 0 || pe.Issues[0].Code != "unknown_asset" {
		t.Fatalf("invalid story error = %v (%+v)", err, pe)
	}
	// Unreadable JSON is an invalid story as well.
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(`{"schema":`), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = fx.runner.Prepare(ctx, fx.projectID, "", fx.settings); !errors.As(err, &pe) || pe.Code != CodeInvalidStory || len(pe.Issues) == 0 {
		t.Fatalf("broken JSON error = %v", err)
	}
	if _, err := fx.runner.Prepare(ctx, "33333333-3333-4333-8333-333333333333", "", fx.settings); err == nil {
		t.Fatal("a missing project was accepted")
	}
}

// narratedStoryJSON has one scene with a two-sentence narration and a voice, so Blocks gives one block.
const narratedStoryJSON = `{"schema":"storymaker/1","title":"Narrated","voice":{"provider":"sanotts","voice":"thorsten","language":"de"},"scenes":[{"id":"s1","environment":"env.plain-stage","narration":"Hallo Welt, das ist der erste Satz. Und hier kommt noch ein zweiter Satz."}]}`

// seedPieces stores audio and a valid envelope for every piece of blocks, as a finished narrate job leaves them.
func seedPieces(t *testing.T, fx *runFixture, blocks []storymaker.NarrationBlock) {
	t.Helper()
	samples := make([]float32, 24000) // 1 s at 24 kHz
	for i := range samples {
		samples[i] = 0.1 // -20 dBFS, above the speech threshold of -40 dBFS
	}
	env := storymaker.ComputeEnvelope(storymaker.DecodedAudio{Rate: 24000, Channels: 1, Samples: samples})
	for _, b := range blocks {
		for _, p := range storymaker.PiecesOf(b) {
			if err := fx.projects.WritePiece(context.Background(), fx.projectID, storymaker.PieceKey(b.Voice, p.Text), "wav", []byte("RIFF"), env); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// bootOfJob runs plan as a job, returns the boot document the page reads, and cancels the job.
func bootOfJob(t *testing.T, fx *runFixture, plan *Plan) []byte {
	t.Helper()
	got := make(chan []byte, 1)
	fx.page.script = func(ctx context.Context, base string) {
		if data, _, err := httpDo(ctx, "GET", base+"/boot.json", "", nil, nil); err == nil {
			got <- data
		}
	}
	job, _, err := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		fx.mgr.Cancel(job.ID)
		waitJob(t, fx.mgr, job.ID, terminal)
	}()
	select {
	case data := <-got:
		return data
	case <-time.After(time.Minute):
		t.Fatal("the page never read boot.json")
		return nil
	}
}

func TestBootCarriesThePlanAudioInfo(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(narratedStoryJSON), ""); err != nil {
		t.Fatal(err)
	}
	unsynthesised, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(unsynthesised.AudioJSON) != `{"narration":{}}` {
		t.Errorf("AudioJSON without audio = %s", unsynthesised.AudioJSON)
	}
	if len(unsynthesised.Blocks) != 1 || unsynthesised.Blocks[0].ID != "s1:n" {
		t.Fatalf("Blocks = %+v", unsynthesised.Blocks)
	}
	seedPieces(t, fx, unsynthesised.Blocks)

	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	// Two sentences of 7 words each: neither merges, so the block has two pieces.
	if blocks := plan.AudioInfo.Narration["s1"].Blocks; len(blocks) != 1 || len(blocks[0].Pieces) != 2 {
		t.Fatalf("AudioInfo = %+v", plan.AudioInfo)
	}
	var doc struct {
		AudioInfo struct {
			Narration json.RawMessage `json:"narration"`
		} `json:"audio_info"`
	}
	if err := json.Unmarshal(bootOfJob(t, fx, plan), &doc); err != nil {
		t.Fatal(err)
	}
	// GET /narration encodes AudioAnswer with encoding/json, so this is the narration value of that route.
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, &plan.Story, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(answer.Narration)
	if err != nil {
		t.Fatal(err)
	}
	if string(doc.AudioInfo.Narration) != string(want) {
		t.Errorf("boot.audio_info.narration:\n%s\nwant:\n%s", doc.AudioInfo.Narration, want)
	}
}

func TestBootServesEmptyAudioInfoWithoutAPlanValue(t *testing.T) {
	fx := newListenerFixture(t)
	_, body := fx.do(t, http.MethodGet, fx.l.Base()+"/boot.json", nil, nil)
	var doc struct {
		AudioInfo json.RawMessage `json:"audio_info"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc.AudioInfo) != `{}` {
		t.Errorf("audio_info = %s", doc.AudioInfo)
	}
}

func TestPrepareWithoutAMusicKeyMeansNoMusic(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	story := `{"schema":"storymaker/1","title":"With music","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1","environment":"env.plain-stage"}]}`
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(story), ""); err != nil {
		t.Fatal(err)
	}
	// fx.deps has no MusicKey, so the key is "" and nothing is to be played.
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare with a nil MusicKey: %v", err)
	}
	if string(plan.AudioJSON) != `{"narration":{}}` {
		t.Errorf("AudioJSON = %s", plan.AudioJSON)
	}
}

func TestPrepareCollectsCodeSceneBundles(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	story, err := os.ReadFile("../testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, bytes.ReplaceAll(story, []byte("\r\n"), []byte("\n")), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings); err == nil {
		t.Fatal("a code scene without a source was accepted")
	} else if pe := new(PrepareError); !errors.As(err, &pe) || pe.Code != CodeInvalidStory {
		t.Fatalf("missing code scene error = %v", err)
	}
	src, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(plan.Bundles) != 1 || !bytes.Contains(plan.Bundles["s3"], []byte("__smScene")) {
		t.Fatalf("bundles = %v", plan.Bundles)
	}
}

// The browser setup of a first export (the pinned Chromium download) runs before the render clock
// starts, so the render timeout has to bound it separately: a peer that stops sending must not
// hold the single render worker.
func TestRunBoundsTheBrowserSetupByTheRenderTimeout(t *testing.T) {
	fx := newRunFixture(t)
	archive := fakeChromiumZip(t, "chrome-x", nil)
	srv := stalledArchiveServer(t, archive)
	b := fx.deps.Browsers
	b.lookPath = func() (string, bool) { return "", false }
	b.pins = map[string]PinnedChromium{b.platform: pinFor(srv.URL, archive)}
	b.idleTimeout = time.Hour // only the render timeout can end this download
	fx.settings.TimeoutSeconds = 1
	fx.playing(playOptions{frames: 10})
	start := time.Now()
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeBrowserUnavailable || !strings.Contains(job.Error.Message, "downloaded") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("the setup ran %s although render.timeout_seconds is 1", took)
	}
}
