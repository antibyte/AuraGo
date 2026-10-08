package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

// Shutdown must stop the Video Studio worker while Desktop storage is still
// open, and a closed Desktop service must not be lazily reopened afterwards.
func TestVideoStudioShutdownStopsWorkerBeforeClosingDesktop(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	queued, _, err := manager.enqueue(project.ID, "render", "fingerprint", "queued-at-shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}
	desktopAtCancel := make(chan error, 1)
	manager.mu.Lock()
	running := &videoStudioJob{ID: uidNew(), ProjectID: project.ID, Kind: "render", Status: "running", CreatedAt: time.Now().UTC()}
	manager.jobs[running.ID] = running
	manager.active[running.ID] = func() {
		// A worker unwinding after cancellation still needs Desktop storage.
		_, listErr := svc.ListFiles(context.Background(), "Documents")
		select {
		case desktopAtCancel <- listErr:
		default:
		}
	}
	manager.mu.Unlock()

	s.shutdownDesktopStorage()

	select {
	case listErr := <-desktopAtCancel:
		if listErr != nil {
			t.Fatalf("Desktop storage was closed before the Video Studio worker was cancelled: %v", listErr)
		}
	default:
		t.Fatal("shutdown closed Desktop storage without cancelling the Video Studio worker")
	}
	if got := manager.job(queued.ID); got == nil || got.Status != "cancelled" || got.Error != "server_shutdown" {
		t.Fatalf("queued job after shutdown = %+v, want server_shutdown cancellation", got)
	}
	if again, _, err := s.getDesktopService(context.Background()); err == nil || again != nil {
		t.Fatalf("getDesktopService reopened Desktop storage after shutdown: svc=%v err=%v", again, err)
	}
	s.DesktopMu.Lock()
	reopened := s.DesktopService != nil
	s.DesktopMu.Unlock()
	if reopened {
		t.Fatal("Desktop service pointer was re-created after shutdown")
	}
}

// A cancel that arrives after the publication gate committed the output must
// not flip the job to cancelled: the file is in the project, so it succeeds.
func TestVideoStudioCancelAfterPublicationCommitKeepsJobSucceeding(t *testing.T) {
	for _, canceller := range []string{"cancelJob", "cancelAll", "close"} {
		t.Run(canceller, func(t *testing.T) {
			srv := videoStudioLifecycleServer()
			manager := newLifecycleVideoStudioManager(t, srv, t.TempDir())
			t.Cleanup(manager.close)
			job, _, err := manager.enqueue(lifecycleProjectID, "render", "fingerprint", "key", nil)
			if err != nil {
				t.Fatal(err)
			}
			manager.mu.Lock()
			manager.jobs[job.ID].Status = "running"
			manager.mu.Unlock()
			ctx := manager.publicationContext(context.Background(), job.ID)
			if err := fileutil.PublishContext(ctx, func() error { return nil }); err != nil {
				t.Fatalf("publication: %v", err)
			}
			switch canceller {
			case "cancelJob":
				if manager.cancelJob(job.ID) {
					t.Fatal("cancelJob accepted a job whose output was already published")
				}
			case "cancelAll":
				manager.cancelAll()
			case "close":
				manager.close()
			}
			if got := manager.job(job.ID); got == nil || got.Status != "running" {
				t.Fatalf("job after %s = %+v, want it still running until the worker records success", canceller, got)
			}
			manager.finish(job.ID, "succeeded", "", "", &videoStudioArtifact{Name: job.ID + ".mp4", Path: "exports/" + job.ID + ".mp4", Size: 1}, nil)
			if got := manager.job(job.ID); got == nil || got.Status != "succeeded" {
				t.Fatalf("job after finish = %+v, want succeeded", got)
			}
		})
	}
}

func videoStudioLifecycleServer() *Server {
	cfg := &config.Config{}
	cfg.VideoStudio.Enabled = true
	cfg.VirtualDesktop.Enabled = true
	return &Server{Cfg: cfg}
}

// Generation must not claim an uncertain provider outcome when no request ever
// reached the provider (here: the provider endpoint refuses connections).
func TestVideoStudioGenerationUnreachableProviderIsNotUncertain(t *testing.T) {
	provider := newVideoStudioGenerationProvider(false)
	unreachable := provider.server.URL
	provider.server.Close()
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken := configureVideoStudioGenerationTest(t, s, provider)
	s.Cfg.VideoGeneration.BaseURL = unreachable + "/v1"
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	jobID := submitVideoStudioGeneration(t, s, adminToken, project.ID, "unreachable-provider")
	job := waitVideoStudioJobTerminal(t, manager, jobID)
	if job.Status != "failed" || job.Error != "generation_failed" || job.ExternalStatusUnknown || strings.Contains(job.Message, "may still") {
		t.Fatalf("unreachable provider result = %+v, want a plain generation failure", job)
	}
}

// Once a request is on its way to the provider, the uncertainty must already be
// recorded; the provider's output stays in the shared Videos library.
func TestVideoStudioGenerationMarksUncertainBeforeProviderSeesRequest(t *testing.T) {
	provider := newVideoStudioGenerationProvider(true)
	defer provider.server.Close()
	defer provider.release()
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken := configureVideoStudioGenerationTest(t, s, provider)
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "missing-ffmpeg.exe")
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	jobID := submitVideoStudioGeneration(t, s, adminToken, project.ID, "uncertain-before-provider")
	select {
	case <-provider.createStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("provider create request did not start")
	}
	if job := manager.job(jobID); job == nil || !job.ExternalStatusUnknown {
		t.Fatalf("job while the provider handles the request = %+v, want external_status_unknown", job)
	}
	provider.release()
	job := waitVideoStudioJobTerminal(t, manager, jobID)
	if job.Error != "generation_import_failed" || job.ExternalStatusUnknown {
		t.Fatalf("generation after provider answer = %+v, want resolved import failure", job)
	}
	entries, err := os.ReadDir(filepath.Join(s.Cfg.Directories.DataDir, "generated_videos"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("provider copy in the Videos library: %d entries (%v), want it kept", len(entries), err)
	}
}

// A failed project delete must leave the project's jobs alone and must not be
// reported as a missing project.
func TestVideoStudioProjectDeleteFailureKeepsJobs(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	queued, _, err := manager.enqueue(project.ID, "render", "fingerprint", "delete-failure", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A revoked publication gate makes the Desktop delete fail.
	manager.mu.Lock()
	manager.configChanging = true
	manager.mu.Unlock()
	failed := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID, nil, nil)
	if failed.Code == http.StatusOK || failed.Code == http.StatusNotFound {
		t.Fatalf("refused delete answered %d %s, want an error that is not 404", failed.Code, failed.Body.String())
	}
	if got := manager.job(queued.ID); got == nil || got.Status != "queued" {
		t.Fatalf("job after failed delete = %+v, want still queued", got)
	}
	if read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil); read.Code != http.StatusOK {
		t.Fatalf("project after failed delete: %d %s", read.Code, read.Body.String())
	}
	manager.mu.Lock()
	manager.configChanging = false
	manager.mu.Unlock()
	deleted := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID, nil, nil)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
	if got := manager.job(queued.ID); got == nil || got.Status != "cancelled" || got.Error != "project_deleted" {
		t.Fatalf("job after delete = %+v, want project_deleted cancellation", got)
	}
}

func TestVideoStudioProjectDeleteFailureStatus(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/", nil)
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "missing", err: fmt.Errorf("delete desktop path: %w", os.ErrNotExist), status: http.StatusNotFound, code: "project_not_found"},
		{name: "revoked", err: fmt.Errorf("delete desktop path: %w", context.Canceled), status: http.StatusConflict, code: "configuration_changed"},
		{name: "disk", err: errors.New("delete desktop path: access is denied"), status: http.StatusInternalServerError, code: "project_delete_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeVideoStudioDeleteError(w, request, tc.err)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("delete error %v = %d %s, want %d %s", tc.err, w.Code, w.Body.String(), tc.status, tc.code)
			}
		})
	}
}

// A configuration change between admission and enqueue is a conflict, as on
// the upload path, not an internal error.
func TestVideoStudioJobCreateConfigurationChangedIsConflict(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.configChanging = true
	manager.mu.Unlock()
	response := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/jobs", strings.NewReader(`{"kind":"render"}`), map[string]string{"If-Match": etag, "Idempotency-Key": "config-changed"})
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"configuration_changed"`) {
		t.Fatalf("enqueue during configuration change: %d %s", response.Code, response.Body.String())
	}
}

type videoStudioFailingReader struct{ err error }

func (r videoStudioFailingReader) Read([]byte) (int, error) { return 0, r.err }

func videoStudioUploadForm(t *testing.T, size int) ([]byte, string) {
	t.Helper()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte{0x42}, size)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return form.Bytes(), writer.FormDataContentType()
}

// Only real size-limit failures are 413; a broken client stream is 400, a
// storage failure 500 and a revoked publication 409.
func TestVideoStudioMediaUploadErrorStatuses(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.MaxAssetSizeMB = 1
	s.Cfg.VideoStudio.MaxProjectSizeMB = 2
	project, _ := videoStudioAPICreate(t, s, writeToken)
	upload := func(key string, body io.Reader, contentType string) *httptest.ResponseRecorder {
		return videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", body, map[string]string{"Content-Type": contentType, "Idempotency-Key": key})
	}
	expect := func(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if response.Code != status || !strings.Contains(response.Body.String(), `"code":"`+code+`"`) {
			t.Fatalf("upload answered %d %s, want %d %s", response.Code, response.Body.String(), status, code)
		}
	}

	t.Run("asset limit", func(t *testing.T) {
		body, contentType := videoStudioUploadForm(t, 1<<20+1024)
		expect(t, upload("too-large", bytes.NewReader(body), contentType), http.StatusRequestEntityTooLarge, "asset_size_limit")
	})
	t.Run("client abort", func(t *testing.T) {
		body, contentType := videoStudioUploadForm(t, 256<<10)
		broken := io.MultiReader(bytes.NewReader(body[:len(body)/2]), videoStudioFailingReader{err: errors.New("client went away")})
		expect(t, upload("aborted", broken, contentType), http.StatusBadRequest, "media_upload_failed")
	})
	t.Run("revoked", func(t *testing.T) {
		manager, err := s.videoStudioManager()
		if err != nil {
			t.Fatal(err)
		}
		manager.mu.Lock()
		manager.configChanging = true
		manager.mu.Unlock()
		defer func() {
			manager.mu.Lock()
			manager.configChanging = false
			manager.mu.Unlock()
		}()
		body, contentType := videoStudioUploadForm(t, 1024)
		expect(t, upload("revoked", bytes.NewReader(body), contentType), http.StatusConflict, "configuration_changed")
	})
	t.Run("storage failure", func(t *testing.T) {
		media := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), "media")
		if err := os.RemoveAll(media); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(media, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		body, contentType := videoStudioUploadForm(t, 1024)
		expect(t, upload("disk", bytes.NewReader(body), contentType), http.StatusInternalServerError, "media_store_failed")
	})
	t.Run("project limit", func(t *testing.T) {
		media := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), "media")
		if err := os.Remove(media); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		filler := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), "filler.bin")
		if err := os.WriteFile(filler, make([]byte, 1<<20+512<<10), 0o600); err != nil {
			t.Fatal(err)
		}
		body, contentType := videoStudioUploadForm(t, 768<<10)
		expect(t, upload("project-full", bytes.NewReader(body), contentType), http.StatusRequestEntityTooLarge, "project_size_limit")
	})
}

// Cancelling a job that already finished is a conflict that still carries the
// job, so a client reading only the status code cannot mistake it for success.
func TestVideoStudioCancelFinishedJobIsConflict(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := manager.enqueue(project.ID, "render", "fingerprint", "finished-job", nil)
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.jobs[job.ID].Status = "running"
	manager.mu.Unlock()
	manager.finish(job.ID, "succeeded", "", "", &videoStudioArtifact{Name: job.ID + ".mp4", Path: "exports/" + job.ID + ".mp4", Size: 1}, nil)
	response := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/jobs/"+job.ID+"/cancel", nil, nil)
	var body struct {
		Code string          `json:"code"`
		Job  *videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || body.Code != "job_finished" || body.Job == nil || body.Job.Status != "succeeded" {
		t.Fatalf("cancel of finished job: %d %s", response.Code, response.Body.String())
	}
	queued, _, err := manager.enqueue(project.ID, "render", "other", "queued-job", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelled := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/jobs/"+queued.ID+"/cancel", nil, nil)
	if cancelled.Code != http.StatusOK || !strings.Contains(cancelled.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("cancel of queued job: %d %s", cancelled.Code, cancelled.Body.String())
	}
}

// The project list reads only the most recently changed projects.
func TestVideoStudioProjectListIsBounded(t *testing.T) {
	previous := videoStudioProjectListLimit
	videoStudioProjectListLimit = 2
	t.Cleanup(func() { videoStudioProjectListLimit = previous })
	s, readToken, writeToken := videoStudioAPIFixture(t)
	base := time.Now().Add(-time.Hour)
	ids := make([]string, 3)
	for i := range ids {
		project, _ := videoStudioAPICreate(t, s, writeToken)
		ids[i] = project.ID
		stamp := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID)), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	response := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects", nil, nil)
	var body struct {
		Projects  []videoStudioProjectItem `json:"projects"`
		Truncated bool                     `json:"truncated"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK {
		t.Fatalf("list: %d %s (%v)", response.Code, response.Body.String(), err)
	}
	if len(body.Projects) != 2 || body.Projects[0].ID != ids[2] || body.Projects[1].ID != ids[1] || !body.Truncated {
		t.Fatalf("bounded list = %d projects (truncated=%v): %s", len(body.Projects), body.Truncated, response.Body.String())
	}
}
