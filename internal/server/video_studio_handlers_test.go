package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/desktop"
)

func TestVideoStudioStatusResponseShapeAndProviderCapabilities(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	response := videoStudioAPIRequest(s, readToken, "GET", "/status", nil, nil)
	if response.Code != 200 {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status struct {
		Enabled        bool   `json:"enabled"`
		ReadOnly       bool   `json:"read_only"`
		DesktopEnabled bool   `json:"desktop_enabled"`
		FFmpegReady    bool   `json:"ffmpeg_ready"`
		Issue          string `json:"issue"`
		Generation     struct {
			Configured       bool     `json:"configured"`
			Provider         string   `json:"provider"`
			Model            string   `json:"model"`
			DurationsSeconds []int    `json:"durations_seconds"`
			ImageModes       []string `json:"image_modes"`
		} `json:"generation"`
		Limits struct {
			MaxAssetSizeBytes   int64 `json:"max_asset_size_bytes"`
			MaxProjectSizeBytes int64 `json:"max_project_size_bytes"`
			FPS                 int   `json:"fps"`
			CanvasSizes         []struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"canvas_sizes"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.ReadOnly || !status.DesktopEnabled || status.FFmpegReady || status.Issue != "ffmpeg_unavailable" {
		t.Fatalf("unexpected status readiness fields: %+v", status)
	}
	if status.Generation.Provider != "minimax" || status.Generation.Model != "hailuo-02" || !status.Generation.Configured || len(status.Generation.ImageModes) != 1 || status.Generation.ImageModes[0] != "first_frame" {
		t.Fatalf("unexpected MiniMax capabilities: %+v", status.Generation)
	}
	if len(status.Generation.DurationsSeconds) != 2 || status.Generation.DurationsSeconds[0] != 6 || status.Generation.DurationsSeconds[1] != 10 {
		t.Fatalf("unexpected MiniMax duration options: %v", status.Generation.DurationsSeconds)
	}
	if status.Limits.FPS != 30 || status.Limits.MaxAssetSizeBytes <= 0 || status.Limits.MaxProjectSizeBytes < status.Limits.MaxAssetSizeBytes || len(status.Limits.CanvasSizes) != 6 || status.Limits.CanvasSizes[0] != (struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}{Width: 1280, Height: 720}) {
		t.Fatalf("unexpected status limits/canvas shape: %+v", status.Limits)
	}
	s.Cfg.VideoGeneration.ProviderType = "agnes"
	response = videoStudioAPIRequest(s, readToken, "GET", "/status", nil, nil)
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Generation.ImageModes) != 0 || len(status.Generation.DurationsSeconds) != 30 || status.Generation.DurationsSeconds[0] != 1 || status.Generation.DurationsSeconds[29] != 30 {
		t.Fatalf("unexpected Agnes capabilities: %+v", status.Generation)
	}
}

func TestVideoStudioGenerationBudgetGatePreventsAdmission(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken, _, err := s.TokenManager.Create("video studio admin fixture", []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, _ := videoStudioAPICreate(t, s, writeToken)
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-only"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	s.Cfg.VideoGeneration.DefaultDurationSeconds = 6
	s.Cfg.VideoGeneration.DefaultResolution = "720p"
	s.Cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	budgetConfig := &config.Config{}
	budgetConfig.Budget.Enabled = true
	budgetConfig.Budget.DailyLimitUSD = 1
	budgetConfig.Budget.Enforcement = "full"
	tracker := budget.NewTracker(budgetConfig, slog.Default(), s.Cfg.Directories.DataDir)
	tracker.RecordCostForCategory("seed", 2)
	s.BudgetTracker = tracker
	response := videoStudioAPIRequest(s, adminToken, "POST", "/projects/"+project.ID+"/jobs", strings.NewReader(`{"kind":"generate","prompt":"fixture","duration_seconds":6}`), map[string]string{"Idempotency-Key": "blocked-generation"})
	if response.Code != 429 || !strings.Contains(response.Body.String(), "budget_blocked") {
		t.Fatalf("budget-blocked generation = %d %s", response.Code, response.Body.String())
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if jobs := manager.jobsForProject(project.ID); len(jobs) != 0 {
		t.Fatalf("blocked paid request created jobs: %+v", jobs)
	}
}

func TestVideoStudioGenerationImagesUseNormalizedProviderFormats(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 12, 7))
	source.Set(1, 2, color.RGBA{R: 200, G: 40, B: 20, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		provider string
		prefix   string
	}{
		{provider: "minimax", prefix: "data:image/png;base64,"},
		{provider: "google_veo", prefix: ""},
	} {
		payload, err := videoStudioGenerationImagePayload(test.provider, encoded.Bytes())
		if err != nil {
			t.Fatalf("%s payload: %v", test.provider, err)
		}
		if !strings.HasPrefix(payload, test.prefix) {
			t.Fatalf("%s payload prefix = %q", test.provider, payload[:min(len(payload), 30)])
		}
		dataURI := strings.TrimPrefix(payload, test.prefix)
		pngBytes, err := base64.StdEncoding.DecodeString(dataURI)
		if err != nil {
			t.Fatalf("%s payload base64: %v", test.provider, err)
		}
		imageConfig, format, err := image.DecodeConfig(bytes.NewReader(pngBytes))
		if err != nil || format != "png" || imageConfig.Width != 12 || imageConfig.Height != 7 {
			t.Fatalf("%s payload is not normalized PNG: config=%+v format=%q err=%v", test.provider, imageConfig, format, err)
		}
	}
	if _, err := videoStudioGenerationImagePayload("minimax", []byte("not an image")); err == nil {
		t.Fatal("invalid image data was accepted")
	}
}

func TestVideoStudioAdmissionBindsOnlyRelevantConfigChanges(t *testing.T) {
	initial := &config.Config{}
	server := &Server{Cfg: initial}
	dataDir := t.TempDir()
	manager := &videoStudioManager{
		server: server, path: filepath.Join(dataDir, "jobs.json"),
		jobs: make(map[string]*videoStudioJob), idem: make(map[string]videoStudioIdempotency),
		works: make(map[string]*videoStudioWork), media: make(map[string]map[string]videoStudioMediaRecord),
		queue: make(chan string, 2),
	}
	updated := *initial
	updated.LLM.Provider = "unrelated-setting"
	server.Cfg = &updated
	work := &videoStudioWork{admittedConfig: initial, admittedEpoch: 0}
	if _, duplicate, err := manager.enqueue(lifecycleProjectID, "probe", "unrelated", "unrelated-key", work); err != nil || duplicate {
		t.Fatalf("unrelated config edit rejected existing admission: duplicate=%v err=%v", duplicate, err)
	}
	providerChanged := updated
	providerChanged.VideoGeneration.ProviderType = "minimax"
	server.Cfg = &providerChanged
	work = &videoStudioWork{admittedConfig: initial, admittedEpoch: 0}
	if _, duplicate, err := manager.enqueue(lifecycleProjectID, "generate", "provider-change", "provider-key", work); err == nil || duplicate || !strings.Contains(err.Error(), "configuration_changed") {
		t.Fatalf("provider change admission = duplicate=%v err=%v, want configuration_changed", duplicate, err)
	}
	server.videoStudioConfigRevoking.Store(true)
	defer server.videoStudioConfigRevoking.Store(false)
	work = &videoStudioWork{admittedConfig: &providerChanged, admittedEpoch: 0}
	if _, duplicate, err := manager.enqueue(lifecycleProjectID, "probe", "revoking", "revoking-key", work); err == nil || duplicate || !strings.Contains(err.Error(), "configuration_changed") {
		t.Fatalf("revocation-latched admission = duplicate=%v err=%v, want configuration_changed", duplicate, err)
	}
}

func TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	root := t.TempDir()
	workingDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := filepath.Rel(workingDir, filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	dataDir, err := filepath.Rel(workingDir, filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.WorkspaceDir = workspace
	s.Cfg.Directories.DataDir = dataDir
	s.Cfg.SQLite.VirtualDesktopPath = ""
	s.Cfg.SQLite.MediaRegistryPath = ""
	s.Cfg.SQLite.ImageGalleryPath = ""
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
	})
	first, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("repeated request recreated the Desktop service for the same relative config")
	}
	if _, err := second.GetDirectorySize(context.Background(), "Documents"); err != nil {
		t.Fatalf("reused service is not usable: %v", err)
	}
	canonical, err := desktop.NormalizeConfig(desktop.ConfigFromAuraConfig(s.ConfigSnapshot()))
	if err != nil {
		t.Fatal(err)
	}
	got := first.Config()
	if got.WorkspaceDir != canonical.WorkspaceDir || got.DataDir != canonical.DataDir || got.DBPath != canonical.DBPath || got.DocumentDir != canonical.DocumentDir || got.MediaRegistryPath != canonical.MediaRegistryPath || got.ImageGalleryPath != canonical.ImageGalleryPath {
		t.Fatalf("service paths were not canonicalized: got=(%q,%q,%q,%q,%q,%q) want=(%q,%q,%q,%q,%q,%q)", got.WorkspaceDir, got.DataDir, got.DBPath, got.DocumentDir, got.MediaRegistryPath, got.ImageGalleryPath, canonical.WorkspaceDir, canonical.DataDir, canonical.DBPath, canonical.DocumentDir, canonical.MediaRegistryPath, canonical.ImageGalleryPath)
	}
}
