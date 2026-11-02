package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

const storymakerCatalogPath = "img/storymaker/catalog.json"

// storymakerRuntime is everything Storymaker keeps alive between requests: the project store, the
// catalog, the browser resolver, the render job manager and its runner.
type storymakerRuntime struct {
	dataDir  string
	store    *storymakerStore
	projects *storymaker.ProjectStore
	catalog  *storymaker.Catalog
	browsers *render.Browsers
	runner   *render.Runner
	jobs     *render.Manager
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
		old.jobs.Close()
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
	jobs, err := render.NewManager(render.ManagerOptions{Dir: dir, Run: run, Logger: logger})
	if err != nil {
		return nil, err
	}
	rt.jobs = jobs
	return rt, nil
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
		rt.jobs.Close()
	}
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
