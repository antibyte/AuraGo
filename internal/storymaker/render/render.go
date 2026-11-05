package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/storymaker"
	"aurago/internal/videostudio"
)

// DefaultTransport is the frame transport used when storymaker.render.transport is empty. The
// transport spike (plan task B12) measured png as the fastest of the four deterministic transports
// on the production path (strict proxy and CDP guard active): 10 to 16 fps at 1080p against 4.5
// for lossless WebP, 4.1 for screenshots and 1.5 for raw RGBA on the development box.
const DefaultTransport = TransportPNG

// measuredFPS is the rendering speed the render timeout assumes per transport: the slowest rate the
// spike measured, cut by about 40 percent for hosts slower than the development box. The timeout is
// max(render.timeout_seconds, 3 x frames / measuredFPS), so a 15-minute video (27000 frames at 30
// fps) gets 13500 s with png, inside the 14400 s ceiling of render.timeout_seconds.
var measuredFPS = map[string]float64{TransportPNG: 6, TransportWebP: 2.5, TransportRaw: 1, TransportScreenshot: 2.5}

// ProjectsRoot is the desktop folder of the projects.
const ProjectsRoot = "Documents/Storymaker"

// The waits of a render; variables so the tests can shorten them.
var (
	reportWait      = 3 * time.Minute
	noFrameWatchdog = 10 * time.Second
)

// frameDeadlineMS is the per-frame deadline offered to the render page (see PageParams).
const frameDeadlineMS = 5000

const (
	maxOutputBytes     = int64(2) << 30
	minFreeStagingByte = int64(256) << 20
	proxyHeaderTimeout = 5 * time.Minute
)

// Publisher writes render outputs into the desktop workspace.
type Publisher interface {
	// WriteStream creates path from r and fails when it exists (create-only). max is the size budget.
	WriteStream(ctx context.Context, path string, r io.Reader, max int64) error
	// WriteBytes writes a small file, replacing an earlier one.
	WriteBytes(ctx context.Context, path string, data []byte) error
}

// Settings are the render settings of one job, read when it starts.
type Settings struct {
	TimeoutSeconds     int
	MaxDurationSeconds int
	BrowserPath        string
	Transport          string // "" selects DefaultTransport
	FFmpegPath         string
	Encoder            videostudio.Encoder
	EncoderName        string
	Limits             storymaker.Limits
}

// Deps are the services a render needs.
type Deps struct {
	// Dir is <DataDir>/storymaker.
	Dir string
	// UI is the ui tree (production: webassets.Namespace("ui")).
	UI        fs.FS
	Catalog   *storymaker.Catalog
	Projects  *storymaker.ProjectStore
	Browsers  *Browsers
	Publisher Publisher
	Settings  func(ctx context.Context) (Settings, error)
	Logger    *slog.Logger
	// MusicKey returns the key of the story's music. nil means key "".
	MusicKey storymaker.MusicKeyResolver

	// OpenPage starts the browser; nil uses the real Chrome. Tests replace it.
	OpenPage pageOpener
	// FrameObserver, when set, sees every frame before it is encoded (the transport spike and the
	// determinism test hash them). It must not keep or change data.
	FrameObserver func(n int, transport string, data []byte)
	// SessionTweak, when set, may change the browser session options before the browser starts (tests
	// switch the guard or the proxy off to measure and to prove each layer).
	SessionTweak func(o *sessionOptions)
	// ReportObserver, when set, sees every compile report body the page posts.
	ReportObserver func(body []byte)
	// ListenerObserver, when set, receives the request counts of the listener when the job ends.
	ListenerObserver func(counts map[string]int64)
}

// Plan is the snapshot of a render request: the validated story after defaults and the bundles of
// its code scenes, taken when the job is queued (so edits while it waits change nothing).
type Plan struct {
	ProjectID string
	Story     storymaker.Story
	StoryJSON []byte
	Bundles   map[string][]byte
	StoryETag string
	Music     *storymaker.MusicRef // nil unless the key's file and meta exist; volume 0 has no key
	Blocks    []storymaker.NarrationBlock
	AudioInfo storymaker.AudioInfo
	// AudioJSON is AudioInfo marshalled once. boot.audio_info serves these bytes, so app and render compile the same input.
	AudioJSON []byte
}

// PrepareError is a request that cannot become a job.
type PrepareError struct {
	Code    string
	Message string
	Issues  []storymaker.Issue
	// Stale is true when the story changed since the caller read it (HTTP 412).
	Stale bool
}

func (e *PrepareError) Error() string { return e.Code + ": " + e.Message }

// Runner executes render jobs.
type Runner struct{ deps Deps }

// NewRunner returns a Runner. Deps.Logger defaults to slog.Default().
func NewRunner(deps Deps) *Runner {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.OpenPage == nil {
		deps.OpenPage = openSession
	}
	return &Runner{deps: deps}
}

// Prepare reads the project's story, checks it against ifMatch (an etag; "" skips the check),
// decodes, defaults and validates it like the save endpoint does and collects the bundles of its
// code scenes. A story with an error issue is refused with CodeInvalidStory and the issues.
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings) (*Plan, error) {
	data, etag, err := r.deps.Projects.LoadStory(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if ifMatch != "" && ifMatch != etag {
		return nil, &PrepareError{Code: "stale_story", Message: "The story changed since it was loaded.", Stale: true}
	}
	story, issues, err := storymaker.DecodeStory(data, st.Limits)
	if err != nil {
		var de *storymaker.DecodeError
		if errors.As(err, &de) {
			return nil, &PrepareError{Code: CodeInvalidStory, Message: de.Message, Issues: de.Issues}
		}
		return nil, err
	}
	_ = issues
	storymaker.ApplyDefaults(&story)
	issues = storymaker.Validate(&story, r.deps.Catalog, st.Limits)
	status, err := r.deps.Projects.CodeSceneStatus(ctx, projectID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if storymaker.HasErrors(issues) {
		first := issues[0]
		for _, is := range issues {
			if is.Severity == storymaker.SeverityError {
				first = is
				break
			}
		}
		return nil, &PrepareError{Code: CodeInvalidStory, Message: clip(first.Message, 300), Issues: issues}
	}
	answer, err := r.deps.Projects.AudioAnswer(ctx, projectID, &story, r.deps.MusicKey)
	if err != nil {
		return nil, err
	}
	audioInfo := answer.Info()
	audioJSON, err := json.Marshal(audioInfo)
	if err != nil {
		return nil, err
	}
	bundles := map[string][]byte{}
	for _, sc := range story.Scenes {
		if sc.Kind != "code" {
			continue
		}
		b, err := r.deps.Projects.LoadCodeBundle(ctx, projectID, sc.ID)
		if err != nil {
			return nil, &PrepareError{Code: CodeCodeError, Message: "The code scene " + sc.ID + " has no bundle: " + clip(err.Error(), 200)}
		}
		bundles[sc.ID] = b
	}
	storyJSON, err := storymaker.CanonicalJSON(story)
	if err != nil {
		return nil, err
	}
	return &Plan{ProjectID: projectID, Story: story, StoryJSON: storyJSON, Bundles: bundles, StoryETag: etag, Music: answer.Music, Blocks: storymaker.Blocks(&story), AudioInfo: audioInfo, AudioJSON: audioJSON}, nil
}

// CanvasSize returns the export size of an aspect: 1920x1080, 1080x1920 or 1080x1080.
func CanvasSize(aspect string) (int, int) {
	switch aspect {
	case "9:16":
		return 1080, 1920
	case "1:1":
		return 1080, 1080
	}
	return 1920, 1080
}

// ChooseTransport resolves the configured transport.
func ChooseTransport(configured string) string {
	switch configured {
	case TransportPNG, TransportWebP, TransportRaw, TransportScreenshot:
		return configured
	}
	return DefaultTransport
}

// RenderTimeout is max(render.timeout_seconds, 3 x frame_count / measured fps of the transport).
func RenderTimeout(timeoutSeconds, frames int, transport string) time.Duration {
	base := time.Duration(timeoutSeconds) * time.Second
	fps := measuredFPS[transport]
	if fps <= 0 {
		fps = 4
	}
	scaled := time.Duration(3 * float64(frames) / fps * float64(time.Second))
	if scaled > base {
		return scaled
	}
	return base
}

// Run is the RunFunc of the job manager.
func (r *Runner) Run(ctx context.Context, job Job, payload any, h *Handle) (Output, error) {
	plan, ok := payload.(*Plan)
	if !ok || plan == nil {
		return Output{}, Fail(CodeRenderFailed, "The render job has no snapshot.")
	}
	settings, err := r.deps.Settings(ctx)
	if err != nil {
		var je *JobError
		if errors.As(err, &je) {
			return Output{}, je
		}
		return Output{}, Fail(CodeFFmpegUnavailable, "%v", err)
	}
	run := &renderRun{r: r, job: job, h: h, plan: plan, st: settings, failCh: make(chan *JobError, 1), reportCh: make(chan struct{}), doneCh: make(chan struct{})}
	return run.execute(ctx)
}

// renderRun is the state of one running render.
type renderRun struct {
	r    *Runner
	job  Job
	h    *Handle
	plan *Plan
	st   Settings

	transport  string
	width      int
	height     int
	fps        int
	frameLimit int
	encoder    videostudio.Encoder
	encName    string
	output     string // staging MP4
	ingest     *Ingest
	base       string
	prefix     string
	token      string
	catVersion string

	failCh   chan *JobError
	reportCh chan struct{}
	doneCh   chan struct{}

	mu        sync.Mutex
	ctx       context.Context
	reported  bool
	restarted bool
	frames    int
	ff        *FFmpeg
	watchdog  *time.Timer
	closeDone sync.Once
	report    compileReport
	startedAt time.Time
	renderEnd time.Time
}

// fail records the first failure of the run.
func (run *renderRun) fail(err *JobError) {
	select {
	case run.failCh <- err:
	default:
	}
	if run.ingest != nil {
		run.ingest.Stop()
	}
}

func (run *renderRun) execute(ctx context.Context) (Output, error) {
	r, h := run.r, run.h
	run.ctx = ctx
	run.startedAt = time.Now()
	aspect := run.plan.Story.Format.Aspect
	run.width, run.height = CanvasSize(aspect)
	run.fps = run.plan.Story.Format.FPS
	if run.fps <= 0 {
		run.fps = storymaker.DefaultFPS
	}
	run.transport = ChooseTransport(run.st.Transport)
	run.frameLimit = int(run.st.Limits.MaxDurationSeconds) * run.fps
	if run.frameLimit <= 0 {
		run.frameLimit = run.st.MaxDurationSeconds * run.fps
	}
	run.encoder, run.encName = run.st.Encoder, run.st.EncoderName
	if run.encName == "" {
		run.encName = videostudio.CodecX264
		if run.encoder.Hardware() {
			run.encName = run.encoder.Codec
		}
	}
	run.catVersion = r.deps.Catalog.Version()
	h.SetPhase(PhaseStarting)

	if run.st.FFmpegPath == "" {
		return Output{}, Fail(CodeFFmpegUnavailable, "FFmpeg was not found.")
	}
	// A first export may download the pinned Chromium. That happens before the render clock starts
	// (wait), so the render timeout bounds it here; OpenPage keeps the job context, because rod ties
	// the browser's lifetime to the context it is given.
	setupTimeout := time.Duration(run.st.TimeoutSeconds) * time.Second
	if setupTimeout <= 0 {
		setupTimeout = time.Hour
	}
	setupCtx, cancelSetup := context.WithTimeout(ctx, setupTimeout)
	status, bin := r.deps.Browsers.Ensure(setupCtx, run.st.BrowserPath, func(done, total int64) {
		if total > 0 {
			h.SetProgress(0.05 * float64(done) / float64(total))
		}
	})
	cancelSetup()
	if !status.Ready {
		return Output{}, Fail(CodeBrowserUnavailable, "%s", status.Issue)
	}

	staging := filepath.Join(StagingRoot(r.deps.Dir), run.job.ID)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return Output{}, Fail(CodeRenderFailed, "The render folder could not be created: %v", err)
	}
	defer os.RemoveAll(staging)
	defer func() {
		run.mu.Lock()
		if run.watchdog != nil {
			run.watchdog.Stop()
		}
		run.mu.Unlock()
	}()
	if free, err := fileutil.FreeDiskBytes(staging); err == nil && free < minFreeStagingByte {
		return Output{}, Fail(CodeRenderFailed, "Not enough free disk space for the render (%d MB free).", free>>20)
	}
	run.output = filepath.Join(staging, "out.mp4")

	snap, err := run.snapshot()
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	prefix, err := NewPrefix()
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	// NewPrefix gives 128 bits from crypto/rand as 32 hex characters, which is what the token needs too.
	token, err := NewPrefix()
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	run.prefix, run.token = prefix, token
	h.SetPrefixHash(HashPrefix(prefix))
	run.ingest = NewIngest(IngestConfig{Width: run.width, Height: run.height, Transport: run.transport, FrameLimit: run.frameLimit})
	listener, err := StartListener(prefix, snap, PageParams{Width: run.width, Height: run.height, FPS: run.fps, Transport: run.transport, FrameLimit: run.frameLimit, FrameDeadlineMS: frameDeadlineMS, Token: token, AudioInfo: run.plan.AudioJSON},
		Hooks{Report: run.onReport, Frame: run.onFrame, Error: run.onPageError})
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	defer listener.Close()
	run.base = listener.Base()
	if obs := r.deps.ListenerObserver; obs != nil {
		defer func() { obs(listener.Counts()) }()
	}

	origin, _ := url.Parse(listener.Origin())
	maxFrame := MaxFrameBytes(IngestConfig{Width: run.width, Height: run.height, Transport: run.transport})
	proxy, err := security.StartStrictCaptureProxy(origin, security.CaptureProxyLimits{
		MaxConcurrent:         16,
		MaxConnections:        400 + run.frameLimit,
		MaxBytes:              int64(run.frameLimit)*maxFrame + (256 << 20),
		ResponseHeaderTimeout: proxyHeaderTimeout,
	})
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	defer proxy.Close()

	opts := sessionOptions{Bin: bin, Profile: filepath.Join(staging, "profile"), ProxyAddr: proxy.Addr(), Base: run.base, Width: run.width, Height: run.height}
	if r.deps.SessionTweak != nil {
		r.deps.SessionTweak(&opts)
	}
	page, err := r.deps.OpenPage(ctx, opts)
	if err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeBrowserUnavailable, "%s", launchIssue(err))
	}
	defer page.Close()
	defer func() {
		run.mu.Lock()
		ff := run.ff
		run.mu.Unlock()
		if ff != nil {
			ff.Discard()
		}
	}()

	h.SetPhase(PhaseCompiling)
	h.SetProgress(0.05)
	if err := page.Navigate(ctx, run.base+"/"); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "The render page could not be opened: %v", err)
	}

	if err := run.wait(ctx, page); err != nil {
		return Output{}, err
	}
	return run.finish(ctx)
}

// wait runs until the last frame was accepted or the job failed, timed out or was cancelled.
func (run *renderRun) wait(ctx context.Context, page browserPage) error {
	reportTimer := time.NewTimer(reportWait)
	defer reportTimer.Stop()
	overall := time.NewTimer(RenderTimeout(run.st.TimeoutSeconds, 0, run.transport))
	defer overall.Stop()
	reportC := run.reportCh
	reportTimeout := reportTimer.C
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case je := <-run.failCh:
			return je
		case <-page.Crashed():
			return Fail(CodeRenderFailed, "The browser tab crashed.")
		case <-reportC:
			reportC = nil
			reportTimeout = nil
			// The report is in: the frame count is known and fixes the timeout.
			if !overall.Stop() {
				select {
				case <-overall.C:
				default:
				}
			}
			overall.Reset(RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport))
			if run.transport == TransportScreenshot {
				go run.driveScreenshots(ctx, page)
			}
		case <-reportTimeout:
			msg := "The render page did not report within " + reportWait.String() + "."
			if status := page.Status(ctx); status != "" && status != "null" {
				msg += " Page status: " + clip(withoutSecrets(status, run.prefix, run.token), 300)
			}
			return Fail(CodeRenderFailed, "%s", msg)
		case <-overall.C:
			return Fail(CodeRenderTimeout, "The render took longer than %s and was stopped.", RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport).Round(time.Second))
		case <-run.doneCh:
			return nil
		}
	}
}

// finish encodes the tail, publishes the files and returns the output.
func (run *renderRun) finish(ctx context.Context) (Output, error) {
	h := run.h
	run.mu.Lock()
	ff := run.ff
	if run.watchdog != nil {
		run.watchdog.Stop()
	}
	run.mu.Unlock()
	h.SetPhase(PhaseEncoding)
	if ff == nil {
		return Output{}, Fail(CodeRenderFailed, "The encoder is missing.")
	}
	if err := ff.Wait(ctx); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "%v", err)
	}
	info, err := os.Stat(run.output)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return Output{}, Fail(CodeRenderFailed, "The encoder produced no file.")
	}
	if info.Size() > maxOutputBytes {
		return Output{}, Fail(CodeRenderFailed, "The video is larger than %d GB.", maxOutputBytes>>30)
	}

	h.SetPhase(PhaseWriting)
	h.SetProgress(0.97)
	slug := storymaker.Slugify(run.plan.Story.Title, run.plan.ProjectID)
	exportDir := ProjectsRoot + "/" + run.plan.ProjectID + "/exports"
	outPath := exportDir + "/" + slug + "-" + run.job.ID[:8] + ".mp4"
	file, err := os.Open(run.output)
	if err != nil {
		return Output{}, Fail(CodeRenderFailed, "The video could not be read: %v", err)
	}
	defer file.Close()
	if err := run.r.deps.Publisher.WriteStream(ctx, outPath, file, maxOutputBytes); err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "The video could not be saved to the project: %v", err)
	}
	meta := map[string]any{
		"job_id": run.job.ID, "title": run.plan.Story.Title, "output_path": outPath, "size": info.Size(),
		"frames": run.report.FrameCount, "duration_ms": run.report.DurationMS, "fps": run.fps, "width": run.width, "height": run.height,
		"encoder": run.currentEncoderName(), "transport": run.transport, "catalog_version": run.catVersion,
		"created_at": time.Now().UTC().Format(time.RFC3339), "render_seconds": time.Since(run.startedAt).Seconds(),
	}
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		if err := run.r.deps.Publisher.WriteBytes(ctx, exportDir+"/"+run.job.ID+".json", append(data, '\n')); err != nil {
			run.r.deps.Logger.Warn("[Storymaker] The export record could not be written", "error", err)
		}
	}
	return Output{Path: outPath, Size: info.Size()}, nil
}

func (run *renderRun) currentEncoderName() string {
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.restarted {
		return videostudio.CodecX264
	}
	return run.encName
}

// snapshot copies everything the page may read.
func (run *renderRun) snapshot() (*Snapshot, error) {
	ui := run.r.deps.UI
	cat := run.r.deps.Catalog
	snap := &Snapshot{Story: run.plan.StoryJSON, CatalogFiles: map[string][]byte{}, CodeBundles: run.plan.Bundles}
	read := func(name string) ([]byte, error) {
		data, err := fs.ReadFile(ui, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return data, nil
	}
	var err error
	if snap.HostBundle, err = read("js/storymaker/host.bundle.js"); err != nil {
		return nil, err
	}
	if snap.PlayerBundle, err = read("js/storymaker/player.bundle.js"); err != nil {
		return nil, err
	}
	files := map[string]bool{"catalog.json": true}
	if cat.Rig.File != "" {
		files[cat.Rig.File] = true
	}
	if glyphs := cat.GlyphsFile(); glyphs != "" {
		files[glyphs] = true
	}
	for _, e := range cat.Entries() {
		if e.File != "" && e.Kind != "sfx" { // the page never reads WAVs
			files[e.File] = true
		}
	}
	for rel := range files {
		clean := path.Clean(rel)
		if clean != rel || !fs.ValidPath(clean) || strings.HasPrefix(clean, "/") {
			return nil, fmt.Errorf("the catalog names an unsafe file %q", rel)
		}
		if snap.CatalogFiles[rel], err = read("img/storymaker/" + rel); err != nil {
			return nil, err
		}
	}
	readFont := func(file string) ([]byte, error) {
		clean := strings.TrimPrefix(path.Clean(file), "/")
		if !strings.HasPrefix(file, "/fonts/") || !fs.ValidPath(clean) {
			return nil, fmt.Errorf("the catalog font file %q is not a /fonts/ path", file)
		}
		return read(clean)
	}
	catalogFamilies := map[string]bool{}
	for _, f := range cat.Fonts {
		data, err := readFont(f.File)
		if err != nil {
			return nil, err
		}
		catalogFamilies[f.Family] = true
		snap.Fonts = append(snap.Fonts, SnapshotFont{Name: f.Name, Family: f.Family, Weight: f.Weight, File: f.File, Data: data})
	}
	// Every fallback face is offered, and the page fetches one only when compile needs it. A face whose
	// family is a catalog font loads through catalog.fonts, so it is not listed twice.
	if glyphs := cat.Glyphs(); glyphs != nil {
		for _, face := range glyphs.Faces {
			if catalogFamilies[face.Family] {
				continue
			}
			data, err := readFont(face.File)
			if err != nil {
				return nil, err
			}
			snap.Fonts = append(snap.Fonts, SnapshotFont{
				Name: face.ID, Family: face.Family, Weight: 400, File: face.File, Data: data,
				Fallback: true, UnicodeRange: face.UnicodeRange, WeightRange: face.Weight, MaxBytes: face.MaxBytes,
			})
		}
	}
	return snap, nil
}

// onReport is the hook behind POST /report: it validates the report, starts FFmpeg and tells the
// page to begin. The ingest starts after the run lock is released (the ingest calls back into the
// run while it holds its own lock, so the lock order is always ingest, then run).
func (run *renderRun) onReport(body []byte) ReportAnswer {
	if obs := run.r.deps.ReportObserver; obs != nil {
		obs(body)
	}
	run.mu.Lock()
	if run.reported {
		run.mu.Unlock()
		return ReportAnswer{Error: "report_repeated", Message: "A compile report was already accepted."}
	}
	run.reported = true
	ids := make([]string, len(run.plan.Story.Scenes))
	for i, sc := range run.plan.Story.Scenes {
		ids[i] = sc.ID
	}
	blocks, captionText, maxEvents := planReportBlocks(run.plan)
	rep, refusal := parseReport(body, reportExpect{FPS: run.fps, Width: run.width, Height: run.height, MaxFrames: run.frameLimit, CatalogVersion: run.catVersion, Catalog: run.r.deps.Catalog, SFXEvents: sfxEventBound(&run.plan.Story, run.r.deps.Catalog), SceneIDs: ids,
		Blocks: blocks, CaptionText: captionText, MaxEvents: maxEvents,
		Prefix: run.prefix, Token: run.token})
	if refusal != nil {
		run.mu.Unlock()
		run.fail(&JobError{Code: refusal.Code, Message: refusal.Message})
		return ReportAnswer{Error: refusal.Code, Message: refusal.Message}
	}
	ff, err := run.startFFmpeg(run.encoder, rep.FrameCount)
	if err != nil {
		run.mu.Unlock()
		run.fail(Fail(CodeFFmpegUnavailable, "%v", err))
		return ReportAnswer{Error: CodeFFmpegUnavailable, Message: err.Error()}
	}
	run.report = rep
	run.frames = rep.FrameCount
	run.ff = ff
	run.watchdog = time.AfterFunc(noFrameWatchdog, func() {
		run.fail(Fail(CodeCodeError, "The page rendered no frame for %s. A code scene may be stuck.", noFrameWatchdog))
	})
	run.mu.Unlock()

	run.ingest.Start(rep.FrameCount, ff, run.recoverEncoder, run.onFrameAccepted)
	run.h.SetPhase(PhaseRendering)
	run.h.SetFrames(0, rep.FrameCount)
	close(run.reportCh)
	return ReportAnswer{Start: true}
}

func (run *renderRun) startFFmpeg(enc videostudio.Encoder, frames int) (*FFmpeg, error) {
	return StartFFmpeg(run.ctx, FFmpegConfig{
		Path: run.st.FFmpegPath, Encoder: enc, Transport: run.transport,
		Width: run.width, Height: run.height, FPS: run.fps, Frames: frames, Output: run.output,
	})
}

// recoverEncoder is the one-shot software restart after a hardware encoder failed in the stream.
// It is called with the ingest lock held and the run lock free.
func (run *renderRun) recoverEncoder(cause error) (FrameSink, bool) {
	run.mu.Lock()
	defer run.mu.Unlock()
	if !run.encoder.Hardware() || run.restarted {
		return nil, false
	}
	run.restarted = true
	run.r.deps.Logger.Warn("[Storymaker] The hardware encoder failed, restarting the render with libx264", "encoder", run.encName, "error", cause)
	if run.ff != nil {
		run.ff.Discard()
	}
	ff, err := run.startFFmpeg(videostudio.Encoder{Quality: run.encoder.Quality}, run.frames)
	if err != nil {
		return nil, false
	}
	run.ff = ff
	return ff, true
}

func (run *renderRun) onFrameAccepted(n int) {
	run.mu.Lock()
	if run.watchdog != nil {
		run.watchdog.Reset(noFrameWatchdog)
	}
	total := run.frames
	run.mu.Unlock()
	run.h.SetFrames(n+1, total)
}

// holdWatchdog stops the no-frame timer while Go works on a frame the page has already delivered. The
// watchdog is there for a page that stops producing frames; the write to FFmpeg (bounded by
// ffmpegWriteTimeout), the settle of a hardware encoder, the software restart and the start of the
// replacement process all pass while the page waits for the answer, and on a loaded host they add up to
// more than the window. The timer runs again when the frame is accepted (onFrameAccepted for a taken
// frame or a restart); a refused frame fails the job, which stops the timer for good.
func (run *renderRun) holdWatchdog() {
	run.mu.Lock()
	if run.watchdog != nil {
		run.watchdog.Stop()
	}
	run.mu.Unlock()
}

// accept feeds one frame to the ingest and maps its outcome; it is the common path of the HTTP hook
// and the screenshot driver.
func (run *renderRun) accept(n int, contentType string, data []byte) (FrameOutcome, *IngestError) {
	run.holdWatchdog()
	if obs := run.r.deps.FrameObserver; obs != nil {
		obs(n, run.transport, data)
	}
	outcome, err := run.ingest.Accept(n, contentType, data)
	if err != nil {
		var ie *IngestError
		if !errors.As(err, &ie) {
			ie = refuse(500, "encoder_failed", "%v", err)
		}
		if ie.Code == "encoder_failed" {
			run.fail(Fail(CodeRenderFailed, "%s", ie.Message))
		} else {
			run.fail(Fail(CodeRenderFailed, "A frame was refused (%s): %s", ie.Code, ie.Message))
		}
		return outcome, ie
	}
	if outcome == FrameRestart {
		run.onFrameAccepted(-1)
	}
	if outcome == FrameDone {
		run.closeDone.Do(func() { close(run.doneCh) })
		run.h.SetPhase(PhaseEncoding)
	}
	return outcome, nil
}

func (run *renderRun) onFrame(n int, contentType string, data []byte) (FrameAnswer, int) {
	outcome, ie := run.accept(n, contentType, data)
	if ie != nil {
		return FrameAnswer{Stop: true, Error: ie.Code, Message: ie.Message}, ie.Status
	}
	switch outcome {
	case FrameDone:
		return FrameAnswer{OK: true, Done: true}, 200
	case FrameRestart:
		return FrameAnswer{Restart: true}, 200
	}
	return FrameAnswer{OK: true}, 200
}

// onPageError makes every error the page reports fatal (spec: a code scene that fails at frame time
// keeps rendering a placeholder, which would produce a video that looks fine and is not).
func (run *renderRun) onPageError(e PageError) {
	code := CodeRenderFailed
	switch e.Code {
	case storymaker.CodeCodeError, storymaker.CodeDeadline:
		code = CodeCodeError
	}
	msg := e.Message
	if e.SceneID != "" {
		msg = "Scene " + e.SceneID + ": " + msg
	}
	run.fail(&JobError{Code: code, Message: clip(msg, 500)})
}

// driveScreenshots renders and captures every frame for the screenshot transport.
func (run *renderRun) driveScreenshots(ctx context.Context, page browserPage) {
	if err := page.WaitReady(ctx); err != nil {
		// WaitReady's error quotes the message in __smStatus.
		run.fail(Fail(CodeRenderFailed, "%s", withoutSecrets(err.Error(), run.prefix, run.token)))
		return
	}
	total := run.frames
	for n := 0; n < total; {
		if ctx.Err() != nil {
			return
		}
		shot, err := page.Capture(ctx, n)
		if err != nil {
			if ctx.Err() == nil {
				// rod's error quotes the exception of __smRenderFrame, and its stack names the bundle URL under the base.
				run.fail(Fail(CodeRenderFailed, "Frame %d could not be captured: %s", n, withoutSecrets(err.Error(), run.prefix, run.token)))
			}
			return
		}
		outcome, ie := run.accept(n, "image/png", shot)
		if ie != nil {
			return
		}
		switch outcome {
		case FrameDone:
			return
		case FrameRestart:
			n = 0
		default:
			n++
		}
	}
}
