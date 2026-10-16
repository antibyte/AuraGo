package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

// videoStudioSeedMedia stores bytes as the imported media of a project and records it.
func videoStudioSeedMedia(t *testing.T, s *Server, projectID string, asset videostudio.Asset, data []byte) videoStudioMediaRecord {
	t.Helper()
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	record := videoStudioMediaRecord{ProjectID: projectID, Asset: asset, Path: "media/" + asset.ID + "_" + asset.Name, CreatedAt: time.Now().UTC()}
	_ = svc.CreateDirectory(ctx, videoStudioProjectPath(projectID)+"/media", desktop.SourceUser)
	if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(projectID)+"/"+record.Path, data, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := manager.recordMedia(record); err != nil {
		t.Fatal(err)
	}
	return record
}

// videoStudioSeedPoster stores a small JPEG as the poster of an asset.
func videoStudioSeedPoster(t *testing.T, s *Server, projectID, assetID string) {
	t.Helper()
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	_ = svc.CreateDirectory(ctx, videoStudioProjectPath(projectID)+"/posters", desktop.SourceUser)
	if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(projectID)+"/posters/"+assetID+".jpg", buf.Bytes(), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
}

func TestVideoStudioPosterRouteAndUploadFolder(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	const withPoster, withoutPoster = "123e4567-e89b-12d3-a456-426614174301", "123e4567-e89b-12d3-a456-426614174302"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: withPoster, Name: "a.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2, HasPoster: true}, []byte("a"))
	videoStudioSeedPoster(t, s, project.ID, withPoster)
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: withoutPoster, Name: "b.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("b"))
	poster := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media/"+withPoster+"/poster.jpg", nil, nil)
	if poster.Code != http.StatusOK || poster.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("poster: %d %q %s", poster.Code, poster.Header().Get("Content-Type"), poster.Body.String())
	}
	if missing := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media/"+withoutPoster+"/poster.jpg", nil, nil); missing.Code != http.StatusNotFound {
		t.Fatalf("asset without poster: %d", missing.Code)
	}
	if bad := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media/not-an-id/poster.jpg", nil, nil); bad.Code != http.StatusNotFound {
		t.Fatalf("invalid asset id: %d", bad.Code)
	}

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	_ = writer.WriteField("folder", "Trips/2024")
	part, err := writer.CreateFormFile("file", "trip.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("not really a png"))
	_ = writer.Close()
	upload := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "upload-folder"})
	if upload.Code != http.StatusAccepted {
		t.Fatalf("upload with folder: %d %s", upload.Code, upload.Body.String())
	}
	var accepted struct {
		Asset videostudio.Asset `json:"asset"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Asset.Folder != "Trips/2024" {
		t.Fatalf("upload folder not recorded: %+v", accepted.Asset)
	}
}

// videoStudioUploadFile uploads one file into a folder through the multipart route and returns the probe job id.
func videoStudioUploadFile(t *testing.T, s *Server, token, projectID, folder, name string, data []byte) string {
	t.Helper()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	_ = writer.WriteField("folder", folder)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = writer.Close()
	upload := videoStudioAPIRequest(s, token, http.MethodPost, "/projects/"+projectID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "upload-" + name})
	var accepted struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &accepted); err != nil || upload.Code != http.StatusAccepted {
		t.Fatalf("upload %s: %d %s (%v)", name, upload.Code, upload.Body.String(), err)
	}
	return accepted.Job.ID
}

// TestVideoStudioImportsWritePosters runs real probes: a JPEG upload (its staged copy is "<id>.jpg", the poster's own
// name must not collide with it), a video upload and a derived frame capture all end with a 320 px poster.
func TestVideoStudioImportsWritePosters(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=64x36:rate=30:duration=1", "-an",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-f", "mp4", video}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	videoData, err := os.ReadFile(video)
	if err != nil {
		t.Fatal(err)
	}
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 40, 20)), nil); err != nil {
		t.Fatal(err)
	}
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	checkPoster := func(label string, asset *videostudio.Asset, folder string, wantWidth, wantHeight int) {
		t.Helper()
		if asset == nil || !asset.HasPoster || asset.Folder != folder {
			t.Fatalf("%s: asset %+v, want has_poster and folder %q", label, asset, folder)
		}
		w := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media/"+asset.ID+"/poster.jpg", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s poster: %d %s", label, w.Code, w.Body.String())
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
		if err != nil || cfg.Width != wantWidth || cfg.Height != wantHeight {
			t.Fatalf("%s poster is %+v (%v), want %dx%d", label, cfg, err, wantWidth, wantHeight)
		}
	}
	photoJob := videoStudioDeriveWait(t, s, readToken, videoStudioUploadFile(t, s, writeToken, project.ID, "Stills", "photo.jpg", photo.Bytes()))
	checkPoster("jpeg upload", photoJob.Result.Asset, "Stills", 320, 160)
	clipJob := videoStudioDeriveWait(t, s, readToken, videoStudioUploadFile(t, s, writeToken, project.ID, " Trips / 2024 ", "clip.mp4", videoData))
	checkPoster("video upload", clipJob.Result.Asset, "Trips/2024", 320, 180)
	freezeID := videoStudioDeriveSubmit(t, s, writeToken, project.ID, "poster-freeze", fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":5,"purpose":"freeze"}`, clipJob.Result.Asset.ID))
	checkPoster("frame capture", videoStudioDeriveWait(t, s, readToken, freezeID).Result.Asset, "", 320, 180)

	read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil)
	var saved videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &saved); err != nil || read.Code != http.StatusOK {
		t.Fatalf("project: %d %s (%v)", read.Code, read.Body.String(), err)
	}
	for _, asset := range saved.Project.Assets {
		if !asset.HasPoster {
			t.Fatalf("saved project asset without poster: %+v", asset)
		}
	}
}

// videoStudioFakeJob places a job of the project in the manager's table and returns a function that removes it. The
// job never runs: it stands in for an import, render or preview that is still queued or running (or one that ended).
func videoStudioFakeJob(t *testing.T, s *Server, projectID, kind, status string) (*videoStudioJob, func()) {
	t.Helper()
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	job := &videoStudioJob{ID: uidNew(), ProjectID: projectID, Kind: kind, Status: status, CreatedAt: time.Now().UTC()}
	manager.mu.Lock()
	manager.jobs[job.ID] = job
	manager.mu.Unlock()
	return job, func() {
		manager.mu.Lock()
		delete(manager.jobs, job.ID)
		manager.mu.Unlock()
	}
}

func TestVideoStudioMediaRemovalAPI(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614174401"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "clip.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2, HasPoster: true}, []byte("clip"))
	videoStudioSeedPoster(t, s, project.ID, assetID)
	path := "/projects/" + project.ID + "/media/" + assetID
	if denied := videoStudioAPIRequest(s, readToken, http.MethodDelete, path, nil, nil); denied.Code != http.StatusForbidden {
		t.Fatalf("read token removed media: %d", denied.Code)
	}
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "clip.bin"}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save with the asset: %d %s", saved.Code, saved.Body.String())
	}
	if inUse := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil); inUse.Code != http.StatusConflict || !strings.Contains(inUse.Body.String(), "media_in_use") {
		t.Fatalf("removal of used media: %d %s", inUse.Code, inUse.Body.String())
	}
	project.Project.Assets = nil
	body, err = json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	saved = videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": saved.Header().Get("ETag")})
	if saved.Code != http.StatusOK {
		t.Fatalf("save without the asset: %d %s", saved.Code, saved.Body.String())
	}
	// A render, preview, probe, reverse, still or duplicate job of the project may still read the file.
	for _, kind := range []string{"render", "preview", "probe", "reverse", "still", "duplicate"} {
		_, remove := videoStudioFakeJob(t, s, project.ID, kind, "running")
		busy := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil)
		remove()
		if busy.Code != http.StatusConflict || !strings.Contains(busy.Body.String(), "media_in_use") {
			t.Fatalf("removal during a %s job: %d %s", kind, busy.Code, busy.Body.String())
		}
	}
	_, removeGenerate := videoStudioFakeJob(t, s, project.ID, "generate", "queued")
	defer removeGenerate()
	_, removeOther := videoStudioFakeJob(t, s, "123e4567-e89b-12d3-a456-426614174499", "render", "running")
	defer removeOther()
	next := *s.ConfigSnapshot()
	next.VideoStudio.ReadOnly = true
	s.replaceConfigSnapshot(&next)
	if readOnly := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil); readOnly.Code != http.StatusForbidden {
		t.Fatalf("read-only removal: %d %s", readOnly.Code, readOnly.Body.String())
	}
	next.VideoStudio.ReadOnly = false
	s.replaceConfigSnapshot(&next)

	// A cached preview of the asset is part of its storage and goes with it.
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	preview, removePreview := videoStudioFakeJob(t, s, project.ID, "preview", "succeeded")
	defer removePreview()
	manager.mu.Lock()
	preview.Artifact = &videoStudioArtifact{Name: "preview.mp4", Size: 7}
	preview.Result = map[string]interface{}{"asset_id": assetID}
	manager.mu.Unlock()
	previewFile := filepath.Join(manager.previewDir, project.ID, preview.ID+".mp4")
	if err := os.MkdirAll(filepath.Dir(previewFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previewFile, []byte("preview"), 0o600); err != nil {
		t.Fatal(err)
	}

	if removed := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil); removed.Code != http.StatusOK {
		t.Fatalf("removal of unused media: %d %s", removed.Code, removed.Body.String())
	}
	if poster := videoStudioAPIRequest(s, readToken, http.MethodGet, path+"/poster.jpg", nil, nil); poster.Code != http.StatusNotFound {
		t.Fatalf("poster after removal: %d", poster.Code)
	}
	if _, ok := manager.mediaRecord(project.ID, assetID); ok {
		t.Fatal("media record survived the removal")
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReadFileBytes(context.Background(), videoStudioProjectPath(project.ID)+"/media/"+assetID+"_clip.bin"); err == nil {
		t.Fatal("media file survived the removal")
	}
	if _, _, err := svc.ReadFileBytes(context.Background(), manager.posterPath(project.ID, assetID)); err == nil {
		t.Fatal("poster file survived the removal")
	}
	if _, err := os.Stat(previewFile); !os.IsNotExist(err) {
		t.Fatalf("cached preview survived the removal: %v", err)
	}
	if job := manager.job(preview.ID); job == nil || job.Artifact != nil {
		t.Fatalf("preview job still offers its artifact: %+v", job)
	}
	if again := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil); again.Code != http.StatusNotFound || !strings.Contains(again.Body.String(), "media_not_found") {
		t.Fatalf("second removal: %d %s", again.Code, again.Body.String())
	}
	if bad := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID+"/media/not-an-id", nil, nil); bad.Code != http.StatusNotFound {
		t.Fatalf("invalid asset id: %d", bad.Code)
	}
}
