package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
)

func videoStudioAPIFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, readToken, writeToken := testDesktopPermissionServer(t)
	configureDesktopAppsHandlerTest(t, s)
	s.Logger = slog.Default()
	s.Cfg.VideoStudio.Enabled = true
	if err := config.NormalizeVideoStudioConfig(&s.Cfg.VideoStudio); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.closeVideoStudioManager)
	return s, readToken, writeToken
}

func videoStudioAPIRequest(s *Server, token, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, videoStudioAPIBase+path, body)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	handleVideoStudio(s).ServeHTTP(w, r)
	return w
}

func videoStudioAPICreate(t *testing.T, s *Server, token string) (videoStudioProjectItem, string) {
	t.Helper()
	w := videoStudioAPIRequest(s, token, http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"API fixture","width":1280,"height":720}`), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var project videoStudioProjectItem
	if err := json.Unmarshal(w.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	return project, w.Header().Get("ETag")
}

func TestVideoStudioAPIProjectPermissionsAndCAS(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	denied := videoStudioAPIRequest(s, readToken, http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"denied"}`), nil)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("read token created project: %d", denied.Code)
	}
	project, originalVersion := videoStudioAPICreate(t, s, writeToken)
	videoStudioAPICreate(t, s, writeToken) // The shared project root may already exist.
	path := "/projects/" + project.ID
	project.Project.Name = "Saved edit"
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	missing := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), nil)
	if missing.Code != http.StatusPreconditionRequired {
		t.Fatalf("save without ETag: %d %s", missing.Code, missing.Body.String())
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": originalVersion})
	if saved.Code != http.StatusOK || saved.Header().Get("ETag") == originalVersion {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	stale := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": originalVersion})
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale save: %d %s", stale.Code, stale.Body.String())
	}
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if read.Code != http.StatusOK || !bytes.Contains(read.Body.Bytes(), []byte("Saved edit")) {
		t.Fatalf("saved project unreadable: %d %s", read.Code, read.Body.String())
	}
	staleRender := videoStudioAPIRequest(s, writeToken, http.MethodPost, path+"/jobs", bytes.NewBufferString(`{"kind":"render"}`), map[string]string{"If-Match": originalVersion, "Idempotency-Key": "stale-render"})
	if staleRender.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale render snapshot: %d %s", staleRender.Code, staleRender.Body.String())
	}
	next := *s.ConfigSnapshot()
	next.VideoStudio.ReadOnly = true
	s.replaceConfigSnapshot(&next)
	denied = videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": saved.Header().Get("ETag")})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("read-only save: %d %s", denied.Code, denied.Body.String())
	}
	if read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); read.Code != http.StatusOK {
		t.Fatalf("read-only project cannot be read: %d", read.Code)
	}
}

func TestVideoStudioAPIImportCopiesMediaAndOwnsMetadata(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	path := "/projects/" + project.ID
	var source bytes.Buffer
	bitmap := image.NewRGBA(image.Rect(0, 0, 64, 64))
	bitmap.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&source, bitmap); err != nil {
		t.Fatal(err)
	}
	upload := func() *httptest.ResponseRecorder {
		var form bytes.Buffer
		writer := multipart.NewWriter(&form)
		part, err := writer.CreateFormFile("file", "source.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(source.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return videoStudioAPIRequest(s, writeToken, http.MethodPost, path+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "same-upload"})
	}
	imported := upload()
	if imported.Code != http.StatusAccepted {
		t.Fatalf("import: %d %s", imported.Code, imported.Body.String())
	}
	var result struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(imported.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	m, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		job := m.job(result.Job.ID)
		if job.Status == "succeeded" {
			break
		}
		if job.Status == "failed" || job.Status == "cancelled" || time.Now().After(deadline) {
			t.Fatalf("import job: %+v", job)
		}
		time.Sleep(20 * time.Millisecond)
	}
	repeated := upload()
	var duplicate struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(repeated.Body.Bytes(), &duplicate); err != nil || repeated.Code != http.StatusAccepted || duplicate.Job.ID != result.Job.ID {
		t.Fatalf("duplicate import: %d %s (%v)", repeated.Code, repeated.Body.String(), err)
	}
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if err := json.Unmarshal(read.Body.Bytes(), &project); err != nil || read.Code != http.StatusOK || len(project.Project.Assets) != 1 {
		t.Fatalf("imported project: %d %s (%v)", read.Code, read.Body.String(), err)
	}
	asset := project.Project.Assets[0]
	copyPath := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), asset.Path)
	copyData, err := os.ReadFile(copyPath)
	if err != nil || !bytes.Equal(copyData, source.Bytes()) {
		t.Fatalf("media was not copied intact: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(copyPath))
	if err != nil || len(entries) != 1 {
		t.Fatalf("duplicate import leaked files: %d (%v)", len(entries), err)
	}
	project.Project.Assets[0].Path = "../../outside.png"
	project.Project.Assets[0].Width = 9999
	body, _ := json.Marshal(project.Project)
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": read.Header().Get("ETag")})
	if err := json.Unmarshal(saved.Body.Bytes(), &project); err != nil || saved.Code != http.StatusOK || project.Project.Assets[0].Path != asset.Path || project.Project.Assets[0].Width != 64 {
		t.Fatalf("client replaced server media metadata: %d %s (%v)", saved.Code, saved.Body.String(), err)
	}
	if err := os.WriteFile(copyPath, []byte("changed after probe"), 0600); err != nil {
		t.Fatal(err)
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	record, ok := m.mediaRecord(project.ID, asset.ID)
	if !ok {
		t.Fatal("trusted media record missing")
	}
	if _, err := m.stageAsset(context.Background(), svc, s.ConfigSnapshot(), record, t.TempDir(), 0); err == nil {
		t.Fatal("changed source bytes were accepted with stale probe metadata")
	}
}
