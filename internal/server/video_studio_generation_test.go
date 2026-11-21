package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktop"
)

type videoStudioGenerationProvider struct {
	server        *httptest.Server
	calls         atomic.Int32
	createStarted chan struct{}
	releaseCreate chan struct{}
	releaseOnce   sync.Once
}

func newVideoStudioGenerationProvider(blockCreate bool) *videoStudioGenerationProvider {
	fixture := &videoStudioGenerationProvider{
		createStarted: make(chan struct{}, 1),
		releaseCreate: make(chan struct{}),
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/video_generation":
			fixture.calls.Add(1)
			if blockCreate {
				fixture.createStarted <- struct{}{}
				<-fixture.releaseCreate
			}
			_, _ = w.Write([]byte(`{"task_id":"review-task","base_resp":{"status_code":0}}`))
		case "/v1/query/video_generation":
			_, _ = w.Write([]byte(`{"status":"Success","file_id":"review-file","base_resp":{"status_code":0}}`))
		case "/v1/files/retrieve":
			_, _ = w.Write([]byte(`{"file":{"download_url":"` + fixture.server.URL + `/download/video.mp4"},"base_resp":{"status_code":0}}`))
		case "/download/video.mp4":
			_, _ = w.Write([]byte("fixture mp4 bytes for import tests"))
		default:
			http.NotFound(w, r)
		}
	}))
	return fixture
}

func (p *videoStudioGenerationProvider) release() {
	p.releaseOnce.Do(func() { close(p.releaseCreate) })
}

func configureVideoStudioGenerationTest(t *testing.T, s *Server, provider *videoStudioGenerationProvider) string {
	t.Helper()
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	s.Cfg.VideoStudio.MaxAssetSizeMB = 1
	s.Cfg.VideoStudio.MaxProjectSizeMB = 1
	s.Cfg.VideoStudio.MaxExportSizeMB = 1
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "video-studio-test-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.BaseURL = provider.server.URL + "/v1"
	s.Cfg.VideoGeneration.ResolvedModel = "MiniMax-Hailuo-2.3"
	s.Cfg.VideoGeneration.DefaultDurationSeconds = 6
	s.Cfg.VideoGeneration.DefaultResolution = "768P"
	s.Cfg.VideoGeneration.PollIntervalSeconds = -1
	token, _, err := s.TokenManager.Create("video generation test", []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func submitVideoStudioGeneration(t *testing.T, s *Server, token, projectID, key string) string {
	t.Helper()
	response := videoStudioAPIRequest(s, token, http.MethodPost, "/projects/"+projectID+"/jobs", strings.NewReader(`{"kind":"generate","prompt":"fixture generation","duration_seconds":6}`), map[string]string{"Idempotency-Key": key})
	if response.Code != http.StatusAccepted {
		t.Fatalf("generation admission: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Job.ID
}

func TestVideoStudioGenerationDistinguishesProviderRejectionFromUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name       string
		httpStatus int
		code       string
		unknown    bool
	}{
		{"rejected", http.StatusBadRequest, "generation_provider_failed", false},
		{"unavailable", http.StatusBadGateway, "generation_failed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, writeToken := videoStudioAPIFixture(t)
			project, _ := videoStudioAPICreate(t, s, writeToken)
			var calls atomic.Int32
			provider := &videoStudioGenerationProvider{server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/videos" {
					t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
				}
				calls.Add(1)
				http.Error(w, "private provider response", tc.httpStatus)
			}))}
			defer provider.server.Close()
			token := configureVideoStudioGenerationTest(t, s, provider)
			s.Cfg.VideoGeneration.ProviderType = "agnes"
			s.Cfg.VideoGeneration.ResolvedModel = "agnes-3.0-flash"
			manager, err := s.videoStudioManager()
			if err != nil {
				t.Fatal(err)
			}
			job := waitVideoStudioJobTerminal(t, manager, submitVideoStudioGeneration(t, s, token, project.ID, tc.name))
			if job.Status != "failed" || job.Error != tc.code || job.ExternalStatusUnknown != tc.unknown || calls.Load() != 1 {
				t.Fatalf("job = %+v, provider calls = %d", job, calls.Load())
			}
			if !tc.unknown && (job.Result["provider_http_status"] != tc.httpStatus || !strings.Contains(job.Message, "HTTP 400")) {
				t.Fatalf("missing provider status in job: %+v", job)
			}
			if strings.Contains(job.Message, "private provider response") {
				t.Fatalf("provider body leaked in job: %+v", job)
			}
		})
	}
}

func waitVideoStudioJobTerminal(t *testing.T, manager *videoStudioManager, jobID string) *videoStudioJob {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		job := manager.job(jobID)
		if job != nil && (job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" || job.Status == "interrupted") {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("generation job %s did not finish: %+v", jobID, manager.job(jobID))
	return nil
}

func fillVideoStudioProjectToQuota(t *testing.T, s *Server, projectID string) {
	t.Helper()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.WriteFileStreamConditional(context.Background(), videoStudioProjectPath(projectID)+"/quota-fill.bin", bytes.NewReader(make([]byte, 1<<20)), 2<<20, desktop.SourceUser, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVideoStudioGenerationRejectsFullProjectBeforeProviderCall(t *testing.T) {
	provider := newVideoStudioGenerationProvider(false)
	defer provider.server.Close()
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken := configureVideoStudioGenerationTest(t, s, provider)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	fillVideoStudioProjectToQuota(t, s, project.ID)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	jobID := submitVideoStudioGeneration(t, s, adminToken, project.ID, "full-project-generation")
	job := waitVideoStudioJobTerminal(t, manager, jobID)
	if provider.calls.Load() != 0 || job.Status != "failed" || job.Error != "project_size_limit" || job.ExternalStatusUnknown {
		t.Fatalf("full-project generation result: provider_calls=%d job=%+v", provider.calls.Load(), job)
	}
}

func TestVideoStudioGenerationRechecksQuotaAfterProviderCall(t *testing.T) {
	provider := newVideoStudioGenerationProvider(true)
	defer provider.server.Close()
	defer provider.release()
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken := configureVideoStudioGenerationTest(t, s, provider)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	jobID := submitVideoStudioGeneration(t, s, adminToken, project.ID, "late-quota-generation")
	select {
	case <-provider.createStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("provider create request did not start")
	}
	fillVideoStudioProjectToQuota(t, s, project.ID)
	provider.release()
	job := waitVideoStudioJobTerminal(t, manager, jobID)
	if provider.calls.Load() != 1 || job.Status != "failed" || job.Error != "project_size_limit" {
		t.Fatalf("late-quota generation result: provider_calls=%d job=%+v", provider.calls.Load(), job)
	}
}

func TestVideoStudioGenerationReportsLocalImportFailureSeparately(t *testing.T) {
	provider := newVideoStudioGenerationProvider(false)
	defer provider.server.Close()
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken := configureVideoStudioGenerationTest(t, s, provider)
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "missing-ffmpeg.exe")
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	jobID := submitVideoStudioGeneration(t, s, adminToken, project.ID, "local-import-failure")
	job := waitVideoStudioJobTerminal(t, manager, jobID)
	if provider.calls.Load() != 1 || job.Status != "failed" || job.Error != "generation_import_failed" || job.ExternalStatusUnknown {
		t.Fatalf("local-import failure result: provider_calls=%d job=%+v", provider.calls.Load(), job)
	}
}
