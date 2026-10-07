package server

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/tools"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func noisemakerCoverTestProvider(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	data := base64.StdEncoding.EncodeToString([]byte("fake png"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/images/generations" {
			t.Errorf("request = %s %s, want POST /images/generations", r.Method, r.URL.Path)
		}
		_, _ = fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, data)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func noisemakerCoverTestGallery(t *testing.T) *sql.DB {
	t.Helper()
	db, err := tools.InitImageGalleryDB(filepath.Join(t.TempDir(), "image_gallery.db"))
	if err != nil {
		t.Fatalf("InitImageGalleryDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func noisemakerCoverTestBudget(t *testing.T, dailyLimit float64) *budget.Tracker {
	t.Helper()
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = dailyLimit
	cfg.Budget.Enforcement = "full"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return budget.NewTracker(cfg, logger, t.TempDir())
}

func resetNoisemakerCoverMonthlyReservations(t *testing.T) {
	t.Helper()
	noisemakerCoverMonthlyMu.Lock()
	oldMonth, oldReservations := noisemakerCoverMonthlyReservationMonth, noisemakerCoverMonthlyReservations
	noisemakerCoverMonthlyReservationMonth = ""
	noisemakerCoverMonthlyReservations = 0
	noisemakerCoverMonthlyMu.Unlock()
	t.Cleanup(func() {
		noisemakerCoverMonthlyMu.Lock()
		noisemakerCoverMonthlyReservationMonth = oldMonth
		noisemakerCoverMonthlyReservations = oldReservations
		noisemakerCoverMonthlyMu.Unlock()
	})
}

func TestNoisemakerCoverBudgetAndLimitsBlockProvider(t *testing.T) {
	resetNoisemakerCoverMonthlyReservations(t)
	tests := []struct {
		name  string
		setup func(*testing.T, *config.Config, *Server)
	}{
		{
			name: "blocked budget",
			setup: func(t *testing.T, cfg *config.Config, s *Server) {
				s.BudgetTracker = noisemakerCoverTestBudget(t, 0.01)
				s.BudgetTracker.RecordCostForCategory("seed", 0.02)
			},
		},
		{
			name: "monthly verification unavailable",
			setup: func(t *testing.T, cfg *config.Config, _ *Server) {
				cfg.ImageGeneration.MaxMonthly = 1
			},
		},
		{
			name: "monthly limit reached",
			setup: func(t *testing.T, cfg *config.Config, s *Server) {
				cfg.ImageGeneration.MaxMonthly = 1
				s.ImageGalleryDB = noisemakerCoverTestGallery(t)
				if _, err := tools.SaveGeneratedImage(s.ImageGalleryDB, &tools.ImageGenResult{Filename: "existing.png", Prompt: "existing", Provider: "openai", Model: "dall-e-3"}); err != nil {
					t.Fatalf("seed gallery: %v", err)
				}
			},
		},
		{
			name: "monthly count unavailable",
			setup: func(t *testing.T, cfg *config.Config, s *Server) {
				cfg.ImageGeneration.MaxMonthly = 1
				s.ImageGalleryDB = noisemakerCoverTestGallery(t)
				if err := s.ImageGalleryDB.Close(); err != nil {
					t.Fatalf("close gallery: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, calls := noisemakerCoverTestProvider(t)
			cfg := &config.Config{}
			cfg.Directories.DataDir = t.TempDir()
			cfg.ImageGeneration.ProviderType = "openai"
			cfg.ImageGeneration.BaseURL = provider.URL
			cfg.ImageGeneration.APIKey = "test-key"
			cfg.ImageGeneration.ResolvedModel = "dall-e-3"
			s := &Server{Cfg: cfg}
			tt.setup(t, cfg, s)

			url, coverErr := s.noisemakerGenerateCover(t.Context(), cfg, "title", "style", "idea", 0)
			if url != "" || coverErr == "" {
				t.Fatalf("cover result = (%q, %q), want denial", url, coverErr)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("provider calls = %d, want 0", got)
			}
		})
	}
}

func TestNoisemakerCoverDailyLimitUsesSubprocess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestNoisemakerCoverDailyLimitProcess$")
	cmd.Env = append(os.Environ(), "AURAGO_NOISEMAKER_COVER_DAILY_TEST=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("daily-limit subprocess failed: %v\n%s", err, output)
	}
}

func TestNoisemakerCoverDailyLimitProcess(t *testing.T) {
	if os.Getenv("AURAGO_NOISEMAKER_COVER_DAILY_TEST") != "1" {
		t.Skip("runs in an isolated process to avoid mutating the shared image counter")
	}
	if _, allowed := tools.ImageGenCounterIncrement(1); !allowed {
		t.Fatal("fresh process should admit the first image")
	}
	provider, calls := noisemakerCoverTestProvider(t)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ImageGeneration.ProviderType = "openai"
	cfg.ImageGeneration.BaseURL = provider.URL
	cfg.ImageGeneration.APIKey = "test-key"
	cfg.ImageGeneration.ResolvedModel = "dall-e-3"
	cfg.ImageGeneration.MaxDaily = 1
	s := &Server{Cfg: cfg}

	if _, coverErr := s.noisemakerGenerateCover(t.Context(), cfg, "title", "style", "idea", 0); coverErr == "" || !strings.Contains(strings.ToLower(coverErr), "daily image generation limit reached") {
		t.Fatalf("cover error = %q, want daily limit denial", coverErr)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("provider calls = %d, want 0", got)
	}
}

func TestNoisemakerCoverRecordsImageCostAndMonthlyUsage(t *testing.T) {
	resetNoisemakerCoverMonthlyReservations(t)
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	provider, calls := noisemakerCoverTestProvider(t)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ImageGeneration.ProviderType = "openai"
	cfg.ImageGeneration.BaseURL = provider.URL
	cfg.ImageGeneration.APIKey = "test-key"
	cfg.ImageGeneration.ResolvedModel = "dall-e-3"
	cfg.ImageGeneration.MaxMonthly = 1
	s := &Server{
		Cfg:            cfg,
		ImageGalleryDB: noisemakerCoverTestGallery(t),
		BudgetTracker:  noisemakerCoverTestBudget(t, 10),
	}

	url, coverErr := s.noisemakerGenerateCover(t.Context(), cfg, "title", "style", "idea", 0)
	if coverErr != "" || url == "" {
		t.Fatalf("cover result = (%q, %q), want successful cover", url, coverErr)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	if got := s.BudgetTracker.CategorySpendUSD("image_generation"); got != 0.04 {
		t.Fatalf("image spend = %f, want 0.04", got)
	}
	if count, err := tools.ImageGalleryMonthlyCount(s.ImageGalleryDB); err != nil || count != 1 {
		t.Fatalf("monthly count = %d, err = %v; want 1, nil", count, err)
	}
}

func TestNoisemakerCoverReservesMonthlySlotWhenGallerySaveFails(t *testing.T) {
	resetNoisemakerCoverMonthlyReservations(t)
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	provider, calls := noisemakerCoverTestProvider(t)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ImageGeneration.ProviderType = "openai"
	cfg.ImageGeneration.BaseURL = provider.URL
	cfg.ImageGeneration.APIKey = "test-key"
	cfg.ImageGeneration.ResolvedModel = "dall-e-3"
	cfg.ImageGeneration.MaxMonthly = 1
	gallery := noisemakerCoverTestGallery(t)
	if _, err := gallery.Exec(`CREATE TRIGGER reject_cover BEFORE INSERT ON generated_images BEGIN SELECT RAISE(FAIL, 'test failure'); END`); err != nil {
		t.Fatalf("create gallery trigger: %v", err)
	}
	s := &Server{Cfg: cfg, ImageGalleryDB: gallery, BudgetTracker: noisemakerCoverTestBudget(t, 10)}

	if _, coverErr := s.noisemakerGenerateCover(t.Context(), cfg, "title", "style", "idea", 0); coverErr == "" {
		t.Fatal("first cover should report gallery persistence failure")
	}
	if got := s.BudgetTracker.CategorySpendUSD("image_generation"); got != 0.04 {
		t.Fatalf("image spend after gallery failure = %f, want 0.04", got)
	}
	if _, coverErr := s.noisemakerGenerateCover(t.Context(), cfg, "title", "style", "idea", 0); coverErr == "" || !strings.Contains(strings.ToLower(coverErr), "monthly image generation limit reached") {
		t.Fatalf("second cover error = %q, want monthly limit denial", coverErr)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1 after failed gallery save", got)
	}
}
