package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

// Admission of range_render (index §4.15): If-Match is required (428 without, 412 when stale), the range is required
// and must lie inside the enabled timeline, and at most 60 s are accepted; a longer range is refused, never shortened.
func TestVideoStudioRangeRenderJobAdmission(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, firstETag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614175701"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "still.png", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("still"))
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "still.png"}}
	// 3000 frames at 30 fps: 100 s, longer than one range check.
	project.Project.Tracks = []videostudio.Track{{ID: "o1", Kind: videostudio.TrackOverlay, Name: "Overlay 1", Clips: []videostudio.Clip{{ID: "c1", AssetID: assetID, Start: 0, Duration: 3000, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}}}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": firstETag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	etag := saved.Header().Get("ETag")
	jobPath := "/projects/" + project.ID + "/jobs"
	for _, tc := range []struct {
		name, body string
		headers    map[string]string
		status     int
		code       string
	}{
		{"no If-Match", `{"kind":"range_render","range":{"start":0,"end":30}}`, map[string]string{"Idempotency-Key": "rr-1"}, http.StatusPreconditionRequired, "precondition_required"},
		{"stale If-Match", `{"kind":"range_render","range":{"start":0,"end":30}}`, map[string]string{"If-Match": firstETag, "Idempotency-Key": "rr-2"}, http.StatusPreconditionFailed, ""},
		{"no range", `{"kind":"range_render"}`, map[string]string{"If-Match": etag, "Idempotency-Key": "rr-3"}, http.StatusBadRequest, "invalid_range"},
		{"empty range", `{"kind":"range_render","range":{"start":10,"end":10}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "rr-4"}, http.StatusBadRequest, "invalid_range"},
		{"past the enabled end", `{"kind":"range_render","range":{"start":2990,"end":3001}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "rr-5"}, http.StatusBadRequest, "invalid_range"},
		{"longer than 60 s", `{"kind":"range_render","range":{"start":0,"end":1801}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "rr-6"}, http.StatusBadRequest, "range_too_long"},
		{"a range on a preview job", `{"kind":"preview","asset_id":"` + assetID + `","range":{"start":0,"end":30}}`, map[string]string{"Idempotency-Key": "rr-7"}, http.StatusBadRequest, "invalid_job"},
	} {
		got := videoStudioAPIRequest(s, writeToken, http.MethodPost, jobPath, bytes.NewBufferString(tc.body), tc.headers)
		if got.Code != tc.status || (tc.code != "" && !strings.Contains(got.Body.String(), `"`+tc.code+`"`)) {
			t.Fatalf("%s: %d %s", tc.name, got.Code, got.Body.String())
		}
	}
	// Exactly 60 s is admitted. The seeded file has no import checksum, so staging refuses it: the job fails with the
	// range code, which also proves the failure mapping for a non-segment error.
	jobID := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"range_render","range":{"start":0,"end":1800}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "rr-ok"})
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitVideoStudioJob(t, manager, jobID); finished.Kind != "range_render" || finished.Status != "failed" || finished.Error != "range_render_failed" {
		t.Fatalf("60 s range check: kind %s, status %s, code %s", finished.Kind, finished.Status, finished.Error)
	}
}

// A finished range check is served from the project's preview folder (as range.mp4); other kinds and other projects are
// not. While one is queued or running, media removal waits; trimming old jobs removes its file with the job.
func TestVideoStudioRangeRenderResultServingAndCleanup(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	writePreviewFile := func(jobID, content string) string {
		t.Helper()
		path := filepath.Join(manager.previewDir, project.ID, jobID+".mp4")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	rangeJob, removeRange := videoStudioFakeJob(t, s, project.ID, "range_render", "succeeded")
	defer removeRange()
	exportJob, removeExport := videoStudioFakeJob(t, s, project.ID, "render", "succeeded")
	defer removeExport()
	manager.mu.Lock()
	rangeJob.Artifact = &videoStudioArtifact{Name: "range.mp4", DownloadURL: "/api/desktop/video-studio/projects/" + project.ID + "/previews/" + rangeJob.ID, Size: 5}
	exportJob.Artifact = &videoStudioArtifact{Name: exportJob.ID + ".mp4", Path: "exports/" + exportJob.ID + ".mp4", Size: 6}
	manager.mu.Unlock()
	rangeFile := writePreviewFile(rangeJob.ID, "range")
	writePreviewFile(exportJob.ID, "export")
	previews := "/projects/" + project.ID + "/previews/"
	served := videoStudioAPIRequest(s, readToken, http.MethodGet, previews+rangeJob.ID, nil, nil)
	if served.Code != http.StatusOK || served.Body.String() != "range" || served.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("range check file: %d %q %s", served.Code, served.Header().Get("Content-Type"), served.Body.String())
	}
	if other := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/123e4567-e89b-12d3-a456-426614175799/previews/"+rangeJob.ID, nil, nil); other.Code != http.StatusNotFound {
		t.Fatalf("a range check was served for another project: %d", other.Code)
	}
	if export := videoStudioAPIRequest(s, readToken, http.MethodGet, previews+exportJob.ID, nil, nil); export.Code != http.StatusNotFound {
		t.Fatalf("an export job was served from the preview folder: %d", export.Code)
	}

	// A range check reads the staged media, so removing media waits while one is queued or running.
	const spareID = "123e4567-e89b-12d3-a456-426614175702"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: spareID, Name: "spare.png", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("spare"))
	for _, status := range []string{"queued", "running"} {
		_, remove := videoStudioFakeJob(t, s, project.ID, "range_render", status)
		busy := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID+"/media/"+spareID, nil, nil)
		remove()
		if busy.Code != http.StatusConflict || !strings.Contains(busy.Body.String(), "media_in_use") {
			t.Fatalf("media removal during a %s range check: %d %s", status, busy.Code, busy.Body.String())
		}
	}

	// Trimming the job table removes a range check's file with its job, like a proxy's.
	manager.mu.Lock()
	rangeJob.CreatedAt = time.Now().Add(-time.Hour).UTC()
	for i := 0; i < videoStudioJobRetention; i++ {
		filler := &videoStudioJob{ID: uidNew(), ProjectID: project.ID, Kind: "probe", Status: "succeeded", CreatedAt: time.Now().UTC()}
		manager.jobs[filler.ID] = filler
	}
	manager.trimJobsLocked()
	_, kept := manager.jobs[rangeJob.ID]
	manager.mu.Unlock()
	if kept {
		t.Fatal("trimming kept the oldest finished job")
	}
	if _, err := os.Stat(rangeFile); !os.IsNotExist(err) {
		t.Fatalf("trimming left the range check file: %v", err)
	}
}

func TestVideoStudioRangeRenderFailureCodes(t *testing.T) {
	ctx := context.Background()
	segment := &videostudio.SegmentError{Index: 0, Start: 1, End: 3, Err: errors.New("encoder exited")}
	if status, code, message := videoStudioJobFailure("range_render", fmt.Errorf("render: %w", segment), ctx); status != "failed" || code != "render_segment_failed" || !strings.Contains(message, "video segment 1 (0:01–0:03)") {
		t.Fatalf("segment failure: %s %s %q", status, code, message)
	}
	if _, code, _ := videoStudioJobFailure("range_render", errors.New("ffmpeg exited"), ctx); code != "range_render_failed" {
		t.Fatalf("plain failure: %s", code)
	}
	capability := &videostudio.CapabilityError{Feature: "transition:black", Missing: "xfade:fadeblack"}
	if _, code, _ := videoStudioJobFailure("range_render", fmt.Errorf("render: %w", capability), ctx); code != "feature_unavailable" {
		t.Fatalf("missing filter: %s", code)
	}
}

// A range check renders the range at 540p through the export pipeline into the project's preview folder, reports the
// size it rendered, serves the file as range.mp4, replaces the previous result and is removed with the project.
func TestVideoStudioRangeRenderJobRendersAndReplaces(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	source := filepath.Join(t.TempDir(), "clip.mp4")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=64x36:rate=30:duration=3", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=3",
		"-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "-f", "mp4", source}
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	imported := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "range-check-import"})
	var importJob struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(imported.Body.Bytes(), &importJob); err != nil || imported.Code != http.StatusAccepted {
		t.Fatalf("import: %d %s (%v)", imported.Code, imported.Body.String(), err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if probe := waitVideoStudioJob(t, manager, importJob.Job.ID); probe.Status != "succeeded" {
		t.Fatalf("probe: %s %s", probe.Status, probe.Error)
	}
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	saved, etag, err := videoStudioProjectETag(ctx, svc, project.ID)
	if err != nil || len(saved.Assets) != 1 {
		t.Fatalf("probed project: %+v %v", saved, err)
	}
	saved.Tracks = []videostudio.Track{{ID: "video", Name: "Video", Kind: videostudio.TrackVideo, Clips: []videostudio.Clip{{ID: "clip", AssetID: saved.Assets[0].ID, Start: 0, Duration: 90, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}}}}
	body, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	put := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if put.Code != http.StatusOK {
		t.Fatalf("save with a clip: %d %s", put.Code, put.Body.String())
	}
	etag = put.Header().Get("ETag")

	first := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"range_render","range":{"start":15,"end":75}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "range-check-1"})
	if done := waitVideoStudioJob(t, manager, first); done.Status != "succeeded" || done.Artifact == nil {
		t.Fatalf("range check: %s %s %s", done.Status, done.Error, done.Message)
	}
	view := videoStudioAPIRequest(s, readToken, http.MethodGet, "/jobs/"+first, nil, nil)
	var reported struct {
		Kind     string `json:"kind"`
		Artifact struct {
			Name        string `json:"name"`
			Path        string `json:"path"`
			DownloadURL string `json:"download_url"`
		} `json:"artifact"`
		Result struct {
			Range           map[string]int `json:"range"`
			DurationFrames  int            `json:"duration_frames"`
			Width           int            `json:"width"`
			Height          int            `json:"height"`
			Encoder         string         `json:"encoder"`
			EncoderFallback *bool          `json:"encoder_fallback"`
		} `json:"result"`
	}
	if err := json.Unmarshal(view.Body.Bytes(), &reported); err != nil {
		t.Fatal(err)
	}
	if reported.Kind != "range_render" || reported.Result.Width != 960 || reported.Result.Height != 540 || reported.Result.DurationFrames != 60 ||
		reported.Result.Range["start"] != 15 || reported.Result.Range["end"] != 75 || reported.Result.Encoder == "" || reported.Result.EncoderFallback == nil {
		t.Fatalf("range check result: %s", view.Body.String())
	}
	if reported.Artifact.Name != "range.mp4" || reported.Artifact.Path != "" || reported.Artifact.DownloadURL != "/api/desktop/video-studio/projects/"+project.ID+"/previews/"+first {
		t.Fatalf("range check artifact: %s", view.Body.String())
	}
	firstFile := filepath.Join(manager.previewDir, project.ID, first+".mp4")
	probed, err := videostudio.Probe(ctx, ffmpeg, firstFile)
	if err != nil {
		t.Fatalf("the result is not in the preview folder: %v", err)
	}
	if frames := videostudio.ProjectRate(saved).FramesAt(probed.DurationUs); probed.Width != 960 || probed.Height != 540 || frames < 59 || frames > 61 {
		t.Fatalf("range check file: %dx%d, %d frames", probed.Width, probed.Height, frames)
	}
	if _, _, err := svc.ReadFileBytes(ctx, videoStudioProjectPath(project.ID)+"/exports/"+first+".mp4"); err == nil {
		t.Fatal("a range check was written to exports/")
	}
	info, err := os.Stat(firstFile)
	if err != nil {
		t.Fatal(err)
	}
	served := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/previews/"+first, nil, nil)
	if served.Code != http.StatusOK || served.Header().Get("Content-Type") != "video/mp4" || int64(served.Body.Len()) != info.Size() {
		t.Fatalf("serving the range check: %d %q %d bytes", served.Code, served.Header().Get("Content-Type"), served.Body.Len())
	}

	// The next range check replaces the previous file and withdraws its download.
	second := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"range_render","range":{"start":0,"end":30}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "range-check-2"})
	if done := waitVideoStudioJob(t, manager, second); done.Status != "succeeded" {
		t.Fatalf("second range check: %s %s %s", done.Status, done.Error, done.Message)
	}
	if _, err := os.Stat(firstFile); !os.IsNotExist(err) {
		t.Fatalf("the previous range check survived: %v", err)
	}
	if job := manager.job(first); job == nil || job.Artifact != nil || job.Message == "" {
		t.Fatalf("the replaced range check still offers its file: %+v", job)
	}
	if gone := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/previews/"+first, nil, nil); gone.Code != http.StatusNotFound {
		t.Fatalf("the replaced range check is still served: %d", gone.Code)
	}
	if _, err := os.Stat(filepath.Join(manager.previewDir, project.ID, second+".mp4")); err != nil {
		t.Fatalf("the new range check is missing: %v", err)
	}

	// Deleting the project removes its range check.
	if deleted := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID, nil, nil); deleted.Code != http.StatusOK {
		t.Fatalf("delete project: %d %s", deleted.Code, deleted.Body.String())
	}
	if _, err := os.Stat(filepath.Join(manager.previewDir, project.ID)); !os.IsNotExist(err) {
		t.Fatalf("the project's range check survived its deletion: %v", err)
	}
}
