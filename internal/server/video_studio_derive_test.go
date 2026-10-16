package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
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

func TestVideoStudioDeriveFailureCodes(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		kind string
		err  error
		code string
	}{
		{"reverse", errors.New("ffmpeg failed"), "reverse_failed"},
		{"still", errors.New("ffmpeg failed"), "still_failed"},
		{"reverse", videostudio.ErrReverseRangeTooLong, "reverse_range_too_long"},
	}
	for _, tc := range cases {
		status, code, message := videoStudioJobFailure(tc.kind, tc.err, ctx)
		if status != "failed" || code != tc.code || message == "" {
			t.Fatalf("%s with %v: status=%q code=%q message=%q, want failed with code %q", tc.kind, tc.err, status, code, message, tc.code)
		}
	}
}

func TestVideoStudioDeriveNames(t *testing.T) {
	if got := reversedAssetName("Beach take 2.mov"); got != "Beach take 2 (reversed).mp4" {
		t.Fatalf("reversedAssetName = %q", got)
	}
	if got := stillAssetName("clip.mp4", 37); got != "clip (frame 37).png" {
		t.Fatalf("stillAssetName = %q", got)
	}
}

// A still request with a speed names the clip position; Frame must be the source frame floor(offset + clip_frame·speed)
// it shows, and the capture then samples at that speed. Without a speed, Frame is the source frame at 1x.
func TestVideoStudioStillPosition(t *testing.T) {
	cases := []struct {
		name    string
		request videoStudioJobRequest
		valid   bool
		want    videostudio.StillOptions
	}{
		{"legacy source frame", videoStudioJobRequest{Frame: 37}, true, videostudio.StillOptions{Offset: 37, Speed: 1}},
		{"clip at 1.5x", videoStudioJobRequest{Frame: 8, Offset: 2, ClipFrame: 4, Speed: 1.5}, true, videostudio.StillOptions{Offset: 2, Frame: 4, Speed: 1.5}},
		{"clip at 1x", videoStudioJobRequest{Frame: 9, Offset: 5, ClipFrame: 4, Speed: 1}, true, videostudio.StillOptions{Offset: 5, Frame: 4, Speed: 1}},
		{"frame past the position", videoStudioJobRequest{Frame: 9, Offset: 2, ClipFrame: 4, Speed: 1.5}, false, videostudio.StillOptions{}},
		{"frame before the offset", videoStudioJobRequest{Frame: 1, Offset: 2, ClipFrame: 0, Speed: 1.5}, false, videostudio.StillOptions{}},
		{"negative clip frame", videoStudioJobRequest{Frame: 2, Offset: 3, ClipFrame: -1, Speed: 1}, false, videostudio.StillOptions{}},
		{"speed below the minimum", videoStudioJobRequest{Frame: 2, Offset: 2, ClipFrame: 1, Speed: 0.1}, false, videostudio.StillOptions{}},
		{"offset without a speed", videoStudioJobRequest{Frame: 8, Offset: 2}, false, videostudio.StillOptions{}},
	}
	for _, tc := range cases {
		if got := validVideoStudioStillPosition(tc.request); got != tc.valid {
			t.Fatalf("%s: valid = %v, want %v", tc.name, got, tc.valid)
		}
		if tc.valid {
			if got := videoStudioStillOptions(tc.request); got != tc.want {
				t.Fatalf("%s: options = %+v, want %+v", tc.name, got, tc.want)
			}
		}
	}
}

// videoStudioDeriveRecord records trusted media metadata without a file; enqueue validation reads only the record.
func videoStudioDeriveRecord(t *testing.T, m *videoStudioManager, projectID, name string, kind videostudio.AssetKind, durationUs int64) string {
	t.Helper()
	id := uidNew()
	path := "media/" + id + "_" + name
	record := videoStudioMediaRecord{ProjectID: projectID, Path: path, SHA256: strings.Repeat("0", 64), CreatedAt: time.Now().UTC(),
		Asset: videostudio.Asset{ID: id, Name: name, Path: path, Kind: kind, DurationUs: durationUs, Width: 64, Height: 36}}
	if err := m.recordMedia(record); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestVideoStudioDeriveEnqueueValidationAndPermissions(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	if rate := videostudio.ProjectRate(project.Project); rate.Num != 30 || rate.Den != 1 {
		t.Fatalf("fixture project rate %v, want 30 fps", rate)
	}
	m, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	video := videoStudioDeriveRecord(t, m, project.ID, "clip.mp4", videostudio.AssetVideo, 2_000_000) // 60 frames
	image := videoStudioDeriveRecord(t, m, project.ID, "still.png", videostudio.AssetImage, 0)
	long := videoStudioDeriveRecord(t, m, project.ID, "long.mp4", videostudio.AssetVideo, 400_000_000)
	path := "/projects/" + project.ID + "/jobs"
	cases := []struct {
		name   string
		token  string
		body   string
		status int
		code   string
	}{
		{"read scope", readToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":0,"duration":10}`, video), http.StatusForbidden, ""},
		{"missing asset", writeToken, `{"kind":"reverse","offset":0,"duration":10}`, http.StatusBadRequest, "invalid_asset_id"},
		{"unknown asset", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":0,"purpose":"freeze"}`, uidNew()), http.StatusNotFound, "media_not_found"},
		{"reverse image", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":0,"duration":10}`, image), http.StatusBadRequest, "derive_requires_video"},
		{"still image", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":0,"purpose":"freeze"}`, image), http.StatusBadRequest, "derive_requires_video"},
		{"empty range", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":0,"duration":0}`, video), http.StatusBadRequest, "invalid_range"},
		{"negative offset", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":-1,"duration":10}`, video), http.StatusBadRequest, "invalid_range"},
		{"past the end", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":50,"duration":11}`, video), http.StatusBadRequest, "invalid_range"},
		{"overflowing offset", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":9223372036854775807,"duration":1}`, video), http.StatusBadRequest, "invalid_range"},
		{"over five minutes", writeToken, fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":0,"duration":9001}`, long), http.StatusBadRequest, "reverse_range_too_long"},
		{"frame past the end", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":60,"purpose":"export"}`, video), http.StatusBadRequest, "invalid_range"},
		{"negative frame", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":-1,"purpose":"export"}`, video), http.StatusBadRequest, "invalid_range"},
		{"no purpose", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":59}`, video), http.StatusBadRequest, "invalid_purpose"},
		{"unknown purpose", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":59,"purpose":"thumbnail"}`, video), http.StatusBadRequest, "invalid_purpose"},
		{"frame not at the clip position", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":10,"offset":2,"clip_frame":4,"speed":1.5,"purpose":"freeze"}`, video), http.StatusBadRequest, "invalid_range"},
		{"speed out of range", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":42,"offset":2,"clip_frame":2,"speed":20,"purpose":"freeze"}`, video), http.StatusBadRequest, "invalid_range"},
		{"clip frame without a speed", writeToken, fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":8,"offset":2,"clip_frame":4,"purpose":"freeze"}`, video), http.StatusBadRequest, "invalid_range"},
	}
	for i, tc := range cases {
		w := videoStudioAPIRequest(s, tc.token, http.MethodPost, path, strings.NewReader(tc.body), map[string]string{"Idempotency-Key": fmt.Sprintf("derive-validation-%d", i)})
		var body struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != tc.status || (tc.code != "" && body.Code != tc.code) {
			t.Fatalf("%s: %d %s, want %d %s", tc.name, w.Code, w.Body.String(), tc.status, tc.code)
		}
	}
	if jobs := m.jobsForProject(project.ID); len(jobs) != 0 {
		t.Fatalf("refused derive requests enqueued %d jobs", len(jobs))
	}
	valid := fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":0,"duration":10}`, video)
	for _, readOnly := range []string{"video studio", "desktop"} {
		before := *s.ConfigSnapshot()
		next := before
		if readOnly == "desktop" {
			next.VirtualDesktop.ReadOnly = true
		} else {
			next.VideoStudio.ReadOnly = true
		}
		s.replaceConfigSnapshot(&next)
		w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, strings.NewReader(valid), map[string]string{"Idempotency-Key": "derive-read-only-" + readOnly})
		s.replaceConfigSnapshot(&before)
		if w.Code != http.StatusForbidden {
			t.Fatalf("read-only %s accepted a reverse job: %d %s", readOnly, w.Code, w.Body.String())
		}
	}
}

type videoStudioDeriveJobView struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	Message  string `json:"message"`
	Artifact *struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		DownloadURL string `json:"download_url"`
		Size        int64  `json:"size"`
	} `json:"artifact"`
	Result struct {
		Purpose string             `json:"purpose"`
		Asset   *videostudio.Asset `json:"asset"`
	} `json:"result"`
}

func videoStudioDeriveSubmit(t *testing.T, s *Server, token, projectID, key, body string) string {
	t.Helper()
	w := videoStudioAPIRequest(s, token, http.MethodPost, "/projects/"+projectID+"/jobs", strings.NewReader(body), map[string]string{"Idempotency-Key": key})
	var created struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || w.Code != http.StatusAccepted || created.Job.ID == "" {
		t.Fatalf("submit %s: %d %s (%v)", body, w.Code, w.Body.String(), err)
	}
	return created.Job.ID
}

// videoStudioDeriveWait polls the public job endpoint, so the test reads the JSON shape the editor reads.
func videoStudioDeriveWait(t *testing.T, s *Server, token, jobID string) videoStudioDeriveJobView {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		w := videoStudioAPIRequest(s, token, http.MethodGet, "/jobs/"+jobID, nil, nil)
		var job videoStudioDeriveJobView
		if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil || w.Code != http.StatusOK {
			t.Fatalf("job %s: %d %s (%v)", jobID, w.Code, w.Body.String(), err)
		}
		switch job.Status {
		case "succeeded":
			return job
		case "failed", "cancelled", "interrupted":
			t.Fatalf("job %s (%s) ended %s: %s %s", jobID, job.Kind, job.Status, job.Error, job.Message)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s did not finish: %+v", jobID, job)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestVideoStudioDeriveJobsImportAndPublish(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	source := filepath.Join(t.TempDir(), "clip.mp4")
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=64x36:rate=30:duration=2", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=2",
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
	imported := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "derive-import"})
	var importJob struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(imported.Body.Bytes(), &importJob); err != nil || imported.Code != http.StatusAccepted {
		t.Fatalf("import: %d %s (%v)", imported.Code, imported.Body.String(), err)
	}
	probe := videoStudioDeriveWait(t, s, readToken, importJob.Job.ID)
	if probe.Result.Asset == nil || probe.Result.Asset.Kind != videostudio.AssetVideo {
		t.Fatalf("probe result: %+v", probe.Result)
	}
	clipID := probe.Result.Asset.ID

	reverseID := videoStudioDeriveSubmit(t, s, writeToken, project.ID, "derive-reverse", fmt.Sprintf(`{"kind":"reverse","asset_id":%q,"offset":10,"duration":30}`, clipID))
	reversed := videoStudioDeriveWait(t, s, readToken, reverseID)
	if reversed.Artifact != nil || reversed.Result.Purpose != "reverse" || reversed.Result.Asset == nil {
		t.Fatalf("reverse result: %+v", reversed)
	}
	if asset := reversed.Result.Asset; asset.Name != "clip (reversed).mp4" || asset.Kind != videostudio.AssetVideo || asset.DurationFrames != 30 || asset.Width != 64 || asset.Height != 36 || !strings.HasPrefix(asset.Path, "media/"+asset.ID+"_") {
		t.Fatalf("reversed asset: %+v", asset)
	}

	freezeID := videoStudioDeriveSubmit(t, s, writeToken, project.ID, "derive-freeze", fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":15,"purpose":"freeze"}`, clipID))
	freeze := videoStudioDeriveWait(t, s, readToken, freezeID)
	if freeze.Artifact != nil || freeze.Result.Purpose != "freeze" || freeze.Result.Asset == nil {
		t.Fatalf("freeze result: %+v", freeze)
	}
	if asset := freeze.Result.Asset; asset.Name != "clip (frame 15).png" || asset.Kind != videostudio.AssetImage || asset.Width != 64 || asset.Height != 36 {
		t.Fatalf("freeze asset: %+v", asset)
	}

	exportID := videoStudioDeriveSubmit(t, s, writeToken, project.ID, "derive-export", fmt.Sprintf(`{"kind":"still","asset_id":%q,"frame":59,"purpose":"export"}`, clipID))
	exported := videoStudioDeriveWait(t, s, readToken, exportID)
	wantURL := "/api/desktop/video-studio/projects/" + project.ID + "/exports/" + exportID + ".png"
	if exported.Result.Purpose != "export" || exported.Result.Asset != nil || exported.Artifact == nil ||
		exported.Artifact.Path != "exports/"+exportID+".png" || exported.Artifact.DownloadURL != wantURL || exported.Artifact.Size <= 0 {
		t.Fatalf("export result: %+v artifact %+v", exported.Result, exported.Artifact)
	}

	read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil)
	var saved videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &saved); err != nil || read.Code != http.StatusOK {
		t.Fatalf("project: %d %s (%v)", read.Code, read.Body.String(), err)
	}
	names := map[string]string{}
	for _, asset := range saved.Project.Assets {
		names[asset.ID] = asset.Name
	}
	if len(saved.Project.Assets) != 3 || names[reversed.Result.Asset.ID] != "clip (reversed).mp4" || names[freeze.Result.Asset.ID] != "clip (frame 15).png" {
		t.Fatalf("project assets after derive jobs: %+v", saved.Project.Assets)
	}

	png200 := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/exports/"+exportID+".png", nil, nil)
	if png200.Code != http.StatusOK || png200.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("PNG export: %d %q %s", png200.Code, png200.Header().Get("Content-Type"), png200.Body.String())
	}
	frame, err := png.DecodeConfig(bytes.NewReader(png200.Body.Bytes()))
	if err != nil || frame.Width != 64 || frame.Height != 36 {
		t.Fatalf("PNG export is not a 64x36 PNG: %+v (%v)", frame, err)
	}
	for _, name := range []string{exportID + ".mp4", reverseID + ".png", freezeID + ".png"} {
		if w := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/exports/"+name, nil, nil); w.Code != http.StatusNotFound {
			t.Fatalf("export route served %s: %d", name, w.Code)
		}
	}
}
