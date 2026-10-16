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
