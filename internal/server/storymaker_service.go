package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const storymakerCatalogPath = "img/storymaker/catalog.json"

// storymakerMaxQueuedNarrations is the number of waiting jobs in the narrate queue.
const storymakerMaxQueuedNarrations = 128

// storymakerRuntime is everything Storymaker keeps alive between requests: the project store, the
// catalog, the browser resolver, the render job manager and its runner.
type storymakerRuntime struct {
	dataDir  string
	voices   *storymakerVoiceCache
	store    *storymakerStore
	projects *storymaker.ProjectStore
	catalog  *storymaker.Catalog
	browsers *render.Browsers
	ffmpeg   *storymakerFFmpegCache // the FFmpeg check of the status endpoint
	runner   *render.Runner
	jobs     *render.Manager           // the render queue
	narrate  *render.Manager           // the narrate and music queue
	usage    *storymakerNarrationUsage // daily characters of the cloud voices
}

// storymakerUIFiles returns the ui tree the catalog and the player bundles are read from: the
// injected one (tests) or the embedded asset set.
func (s *Server) storymakerUIFiles() fs.FS {
	if s.storymakerUI != nil {
		return s.storymakerUI
	}
	return uiFiles
}

// storymakerDir is the folder of jobs.json, the staging area and the pinned browser.
func storymakerDir(dataDir string) string { return filepath.Join(dataDir, "storymaker") }

// storymakerRuntime returns the runtime for the current data directory, creating it on first use.
func (s *Server) storymakerRuntime() (*storymakerRuntime, error) {
	if s == nil {
		return nil, errors.New("storymaker unavailable")
	}
	s.storymakerMu.Lock()
	defer s.storymakerMu.Unlock()
	if s.storymakerClosed {
		return nil, errors.New("storymaker is shutting down")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" {
		return nil, errors.New("the data directory is unavailable")
	}
	dataDir := normalizedVideoStudioRoot(cfg.Directories.DataDir)
	if s.storymaker != nil {
		if s.storymaker.dataDir == dataDir {
			return s.storymaker, nil
		}
		old := s.storymaker
		s.storymaker = nil
		old.close()
	}
	rt, err := s.newStorymakerRuntime(dataDir)
	if err != nil {
		return nil, err
	}
	s.storymaker = rt
	return rt, nil
}

func (s *Server) newStorymakerRuntime(dataDir string) (*storymakerRuntime, error) {
	ui := s.storymakerUIFiles()
	catalog, err := storymaker.LoadCatalog(ui, storymakerCatalogPath)
	if err != nil {
		return nil, fmt.Errorf("the Storymaker catalog could not be loaded: %w", err)
	}
	dir := storymakerDir(dataDir)
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	store := &storymakerStore{}
	rt := &storymakerRuntime{
		dataDir:  dataDir,
		store:    store,
		projects: storymaker.NewProjectStore(store, render.ProjectsRoot),
		catalog:  catalog,
		browsers: render.NewBrowsers(dir),
		ffmpeg:   newStorymakerFFmpegCache(),
		voices:   newStorymakerVoiceCache(),
		usage:    newStorymakerNarrationUsage(filepath.Join(dir, "narrate")),
	}
	if s.storymakerBrowsers != nil {
		rt.browsers = s.storymakerBrowsers
	}
	rt.runner = render.NewRunner(render.Deps{
		Dir: dir, UI: ui, Catalog: catalog, Projects: rt.projects, Browsers: rt.browsers, Publisher: store,
		Settings: func(ctx context.Context) (render.Settings, error) { return s.storymakerSettings(ctx) },
		Logger:   logger, FrameObserver: s.storymakerFrameObserver,
	})
	run := rt.runner.Run
	if s.storymakerRunOverride != nil {
		run = s.storymakerRunOverride
	}
	audioRun := func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		return s.storymakerAudioRun(ctx, rt, job, payload, h)
	}
	if s.storymakerAudioRunOverride != nil {
		audioRun = s.storymakerAudioRunOverride
	}
	jobs, narrate, err := openStorymakerManagers(dir, run, audioRun, logger, render.NewManager)
	if err != nil {
		return nil, err
	}
	rt.jobs, rt.narrate = jobs, narrate
	return rt, nil
}

// openStorymakerManagers opens the render queue and the narrate queue. A failure of the second closes
// the first, or its worker would stay alive with no owner.
func openStorymakerManagers(dir string, run, audioRun render.RunFunc, logger *slog.Logger, open func(render.ManagerOptions) (*render.Manager, error)) (*render.Manager, *render.Manager, error) {
	jobs, err := open(render.ManagerOptions{Dir: dir, Run: run, Logger: logger})
	if err != nil {
		return nil, nil, err
	}
	narrate, err := open(render.ManagerOptions{
		Dir:       filepath.Join(dir, "narrate"),
		Kinds:     []string{render.KindNarrate, render.KindMusic},
		MaxQueued: storymakerMaxQueuedNarrations,
		Workers:   1,
		Run:       audioRun,
		Logger:    logger,
	})
	if err != nil {
		jobs.Close()
		return nil, nil, err
	}
	return jobs, narrate, nil
}

// closeStorymaker stops the job manager (a running render ends as interrupted).
func (s *Server) closeStorymaker() {
	if s == nil {
		return
	}
	s.storymakerMu.Lock()
	s.storymakerClosed = true
	rt := s.storymaker
	s.storymaker = nil
	s.storymakerMu.Unlock()
	if rt != nil {
		rt.close()
	}
}

// jobByID finds a job in either queue and returns the manager that holds it.
func (rt *storymakerRuntime) jobByID(id string) (render.Job, *render.Manager, bool) {
	if job, ok := rt.jobs.Get(id); ok {
		return job, rt.jobs, true
	}
	if job, ok := rt.narrate.Get(id); ok {
		return job, rt.narrate, true
	}
	return render.Job{}, nil, false
}

// listJobs merges the jobs of a project from both queues, newest first.
func (rt *storymakerRuntime) listJobs(projectID string) []render.Job {
	jobs := append(rt.jobs.List(projectID), rt.narrate.List(projectID)...)
	sort.Slice(jobs, func(i, j int) bool {
		if !jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
		}
		return jobs[i].ID > jobs[j].ID
	})
	return jobs
}

// initialJobs is the first list of the event stream, newest first: every unfinished job of both
// queues plus the merged latest five, so newer narrate jobs cannot push a running export out of it.
func (rt *storymakerRuntime) initialJobs(projectID string) []render.Job {
	all := rt.listJobs(projectID)
	out := make([]render.Job, 0, len(all))
	for i, job := range all {
		if i < 5 || !job.Terminal() {
			out = append(out, job)
		}
	}
	return out
}

// cancelProject cancels every unfinished job of a project in both queues.
func (rt *storymakerRuntime) cancelProject(projectID string) {
	rt.jobs.CancelProject(projectID)
	rt.narrate.CancelProject(projectID)
}

// storymakerDeleteWait bounds how long a project delete waits for the project's jobs to stop.
const storymakerDeleteWait = 15 * time.Second

// waitProject waits until the project has no unfinished job in either queue. A job that was cancelled
// can still hold a file open or write one, so a delete waits before it removes the folder. A job that
// outlasts the bound does not block the delete, so the outcome of the wait is not returned.
func (rt *storymakerRuntime) waitProject(ctx context.Context, projectID string) {
	ctx, cancel := context.WithTimeout(ctx, storymakerDeleteWait)
	defer cancel()
	_ = rt.jobs.WaitProject(ctx, projectID)
	_ = rt.narrate.WaitProject(ctx, projectID)
}

// close stops both managers.
func (rt *storymakerRuntime) close() {
	rt.jobs.Close()
	rt.narrate.Close()
}

// normalizedStorymakerConfig returns a copy of the configuration with the Storymaker section
// normalized.
func normalizedStorymakerConfig(s *Server) (*config.Config, error) {
	if s == nil {
		return nil, errors.New("server unavailable")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, errors.New("configuration unavailable")
	}
	cp := *cfg
	if err := config.NormalizeStorymakerConfig(&cp.Storymaker); err != nil {
		return nil, err
	}
	return &cp, nil
}

// storymakerLimits is storymaker.DefaultLimits with the configured maximum duration, so the
// validator and the render agree.
func storymakerLimits(cfg *config.Config) storymaker.Limits {
	lim := storymaker.DefaultLimits()
	lim.MaxDurationSeconds = float64(cfg.Storymaker.Render.MaxDurationSeconds)
	return lim
}

// storymakerSettings reads the render settings of a job: the Storymaker section, FFmpeg and the
// encoder (selected like Video Studio does and cached the same way).
func (s *Server) storymakerSettings(ctx context.Context) (render.Settings, error) {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return render.Settings{}, err
	}
	// A job that waited in the queue while the feature was switched off, or the desktop made read-only,
	// ends here instead of rendering for nothing.
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return render.Settings{}, render.Fail(render.CodeDisabled, "Storymaker was switched off before this export started.")
	}
	if cfg.VirtualDesktop.ReadOnly {
		return render.Settings{}, render.Fail(render.CodeRenderFailed, "The desktop is read-only; the export was not started.")
	}
	ffmpeg, err := videostudio.ResolveFFmpeg(cfg.VideoStudio.FFmpegPath)
	if err != nil {
		return render.Settings{}, fmt.Errorf("FFmpeg is not available: %v", err)
	}
	enc := videoStudioEncoder(ctx, cfg)
	return render.Settings{
		TimeoutSeconds:     cfg.Storymaker.Render.TimeoutSeconds,
		MaxDurationSeconds: cfg.Storymaker.Render.MaxDurationSeconds,
		BrowserPath:        cfg.Storymaker.Render.BrowserPath,
		Transport:          cfg.Storymaker.Render.Transport,
		FFmpegPath:         ffmpeg,
		Encoder:            enc.Encoder,
		EncoderName:        enc.Name,
		Limits:             storymakerLimits(cfg),
	}, nil
}

// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
// enabled virtual desktop.
func (s *Server) storymakerAvailable() bool {
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.Storymaker.Enabled && cfg.VirtualDesktop.Enabled
}

// warmStorymakerBrowser probes the browser in the background so the first status request finds the
// result cached.
func (rt *storymakerRuntime) warmBrowser(cfg *config.Config) {
	path := cfg.Storymaker.Render.BrowserPath
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = rt.browsers.Status(ctx, path)
	}()
}
