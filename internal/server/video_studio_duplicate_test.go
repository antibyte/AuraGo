package server

import (
	"bytes"
	"context"
	"encoding/json"
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

// waitVideoStudioJob polls the manager until the job reaches a terminal state.
func waitVideoStudioJob(t *testing.T, manager *videoStudioManager, id string) *videoStudioJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if job := manager.job(id); job != nil && (job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" || job.Status == "interrupted") {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", id)
	return nil
}

// videoStudioSubmitJob posts a job and returns its id; the request must be accepted.
func videoStudioSubmitJob(t *testing.T, s *Server, token, projectID, body string, headers map[string]string) string {
	t.Helper()
	accepted := videoStudioAPIRequest(s, token, http.MethodPost, "/projects/"+projectID+"/jobs", bytes.NewBufferString(body), headers)
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("job %s: %d %s", body, accepted.Code, accepted.Body.String())
	}
	var created struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.Job.ID
}

// videoStudioProjectFolders counts the project folders below the Video Studio root.
func videoStudioProjectFolders(t *testing.T, ctx context.Context, svc *desktop.Service) int {
	t.Helper()
	entries, err := svc.ListFiles(ctx, "Documents/Video Studio")
	if err != nil {
		t.Fatal(err)
	}
	folders := 0
	for _, entry := range entries {
		if entry.Type == "directory" && validVideoStudioProjectID(entry.Name) {
			folders++
		}
	}
	return folders
}

func TestVideoStudioDuplicateJobCopiesMediaAndSkipsExports(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614174501"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "clip.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2, HasPoster: true}, []byte("clip"))
	videoStudioSeedPoster(t, s, project.ID, assetID)
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "Renamed clip", Folder: "Shots"}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	if saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag}); saved.Code != http.StatusOK {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(project.ID)+"/exports/old.mp4", []byte("export"), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotVideoStudioVersion(ctx, svc, project.ID, videoStudioVersionManual, "", "API fixture", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	jobPath := "/projects/" + project.ID + "/jobs"
	if denied := videoStudioAPIRequest(s, readToken, http.MethodPost, jobPath, bytes.NewBufferString(`{"kind":"duplicate","name":"Copy"}`), map[string]string{"Idempotency-Key": "dup-denied"}); denied.Code != http.StatusForbidden {
		t.Fatalf("read token duplicated a project: %d", denied.Code)
	}
	if blank := videoStudioAPIRequest(s, writeToken, http.MethodPost, jobPath, bytes.NewBufferString(`{"kind":"duplicate","name":"   "}`), map[string]string{"Idempotency-Key": "dup-blank"}); blank.Code != http.StatusBadRequest || !strings.Contains(blank.Body.String(), "invalid_job") {
		t.Fatalf("a copy without a name: %d %s", blank.Code, blank.Body.String())
	}
	jobID := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"duplicate","name":"  Copy  "}`, map[string]string{"Idempotency-Key": "dup-1"})
	finished := waitVideoStudioJob(t, manager, jobID)
	if finished.Status != "succeeded" {
		t.Fatalf("duplicate finished %s: %s", finished.Status, finished.Error)
	}
	copyID, _ := finished.Result["project_id"].(string)
	if finished.Result["name"] != "Copy" || finished.Artifact != nil {
		t.Fatalf("duplicate result: %+v artifact %+v", finished.Result, finished.Artifact)
	}
	if finished.CopyProjectID != "" {
		t.Fatalf("a finished copy is still recorded as in progress: %q", finished.CopyProjectID)
	}
	copied, _, _, err := readVideoStudioProject(ctx, svc, videoStudioProjectPath(copyID))
	if err != nil || copied.Name != "Copy" || len(copied.Assets) != 1 || copied.Assets[0].Name != "Renamed clip" || copied.Assets[0].Folder != "Shots" {
		t.Fatalf("copy document: %+v, %v", copied, err)
	}
	entries, err := svc.ListFiles(ctx, videoStudioProjectPath(copyID))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name] = true
	}
	if !names["media"] || !names["posters"] || names["exports"] || names["versions"] {
		t.Fatalf("copy folders: %v (media and posters expected; exports and versions must not be copied)", names)
	}
	media, _, err := svc.ReadFileBytes(ctx, videoStudioProjectPath(copyID)+"/media/"+assetID+"_clip.bin")
	if err != nil || string(media) != "clip" {
		t.Fatalf("copied media: %q %v", media, err)
	}
	if _, _, err := svc.ReadFileBytes(ctx, videoStudioProjectPath(project.ID)+"/exports/old.mp4"); err != nil {
		t.Fatalf("the source export was touched: %v", err)
	}
	record, ok := manager.mediaRecord(copyID, assetID)
	if !ok || !record.Asset.HasPoster || record.ProjectID != copyID {
		t.Fatalf("the copy's media record: %+v (found %v)", record, ok)
	}
	if poster := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+copyID+"/media/"+assetID+"/poster.jpg", nil, nil); poster.Code != http.StatusOK {
		t.Fatalf("the copy's poster: %d %s", poster.Code, poster.Body.String())
	}
	opened := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+copyID, nil, nil)
	if opened.Code != http.StatusOK {
		t.Fatalf("the copy does not open: %d %s", opened.Code, opened.Body.String())
	}
}

func TestVideoStudioDuplicateQuotaAndRangeAdmission(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614174503"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "ballast.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, make([]byte, 1<<20))
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "ballast.bin"}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	etag = saved.Header().Get("ETag")
	// The limit is lowered after the media landed: the 1 MiB file plus the document no longer fit one copy.
	next := *s.ConfigSnapshot()
	next.VideoStudio.MaxProjectSizeMB = 1
	s.replaceConfigSnapshot(&next)
	jobPath := "/projects/" + project.ID + "/jobs"
	tooLarge := videoStudioAPIRequest(s, writeToken, http.MethodPost, jobPath, bytes.NewBufferString(`{"kind":"duplicate","name":"Copy"}`), map[string]string{"Idempotency-Key": "dup-big"})
	if tooLarge.Code != http.StatusRequestEntityTooLarge || !strings.Contains(tooLarge.Body.String(), "duplicate_quota") {
		t.Fatalf("duplicate over the quota: %d %s", tooLarge.Code, tooLarge.Body.String())
	}
	// The project has no clips, so no range fits inside it; an inverted or negative range is refused the same way,
	// and only render jobs take a range at all.
	for i, body := range []string{`{"kind":"render","range":{"start":0,"end":30}}`, `{"kind":"render","range":{"start":10,"end":5}}`, `{"kind":"render","range":{"start":-1,"end":5}}`} {
		ranged := videoStudioAPIRequest(s, writeToken, http.MethodPost, jobPath, bytes.NewBufferString(body), map[string]string{"If-Match": etag, "Idempotency-Key": "range-" + string(rune('a'+i))})
		if ranged.Code != http.StatusBadRequest || !strings.Contains(ranged.Body.String(), "invalid_range") {
			t.Fatalf("range %s: %d %s", body, ranged.Code, ranged.Body.String())
		}
	}
	notRender := videoStudioAPIRequest(s, writeToken, http.MethodPost, jobPath, bytes.NewBufferString(`{"kind":"duplicate","name":"Copy","range":{"start":0,"end":1}}`), map[string]string{"Idempotency-Key": "range-dup"})
	if notRender.Code != http.StatusBadRequest || !strings.Contains(notRender.Body.String(), "invalid_job") {
		t.Fatalf("a range on a duplicate: %d %s", notRender.Code, notRender.Body.String())
	}
}

// videoStudioProjectETag reads the saved project's ETag.
func videoStudioProjectETag(ctx context.Context, svc *desktop.Service, projectID string) (videostudio.Project, string, error) {
	project, _, version, err := readVideoStudioProject(ctx, svc, videoStudioProjectPath(projectID))
	return project, version, err
}

func TestVideoStudioDuplicateFailsWhenMediaVanished(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614174502"
	record := videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "gone.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("gone"))
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "gone.bin"}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	if saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag}); saved.Code != http.StatusOK {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	// A file that disappears after the snapshot (the bytes are copied outside the project lock) fails the copy, and
	// the partial copy is removed: it never became a project.
	snap, err := manager.snapshotDuplicateSource(ctx, svc, s.ConfigSnapshot(), project.ID, "Copy")
	if err != nil {
		t.Fatal(err)
	}
	mediaPath := videoStudioProjectPath(project.ID) + "/" + record.Path
	if err := svc.DeletePath(ctx, mediaPath, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.copyDuplicate(ctx, svc, "", project.ID, snap); err == nil {
		t.Fatal("a copy of a vanished file succeeded")
	}
	if folders := videoStudioProjectFolders(t, ctx, svc); folders != 1 {
		t.Fatalf("a failed copy left %d project folders, want the source only", folders)
	}
	// A file that is already gone when the job starts fails the job with duplicate_failed.
	jobID := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"duplicate","name":"Copy"}`, map[string]string{"Idempotency-Key": "dup-vanished"})
	finished := waitVideoStudioJob(t, manager, jobID)
	if finished.Status != "failed" || finished.Error != "duplicate_failed" {
		t.Fatalf("copy of a vanished file: status %s, code %s", finished.Status, finished.Error)
	}
	if folders := videoStudioProjectFolders(t, ctx, svc); folders != 1 {
		t.Fatalf("a failed copy left %d project folders, want the source only", folders)
	}
	manager.mu.Lock()
	recorded := len(manager.media)
	manager.mu.Unlock()
	if recorded != 1 {
		t.Fatalf("media records exist for %d projects after a failed copy, want the source only", recorded)
	}
}

func TestVideoStudioProjectListSizesOnRequest(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(project.ID)+"/media/ballast.bin", make([]byte, 4096), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), "project.json"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	plain := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects", nil, nil)
	if plain.Code != http.StatusOK || strings.Contains(plain.Body.String(), "storage_bytes") {
		t.Fatalf("the plain list must not walk project folders: %d %s", plain.Code, plain.Body.String())
	}
	type listing struct {
		Projects []struct {
			StorageBytes *int64    `json:"storage_bytes"`
			UpdatedAt    time.Time `json:"updated_at"`
		} `json:"projects"`
	}
	var listed listing
	sized := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects?sizes=1", nil, nil)
	if err := json.Unmarshal(sized.Body.Bytes(), &listed); err != nil || sized.Code != http.StatusOK || len(listed.Projects) != 1 || listed.Projects[0].StorageBytes == nil || *listed.Projects[0].StorageBytes < 4096 {
		t.Fatalf("sized list: %d %v %s", sized.Code, err, sized.Body.String())
	}
	if !listed.Projects[0].UpdatedAt.Equal(stamp) {
		t.Fatalf("updated_at = %v, want the time of project.json %v", listed.Projects[0].UpdatedAt, stamp)
	}
	previous := videoStudioProjectSizeWalkEntries
	videoStudioProjectSizeWalkEntries = 2
	t.Cleanup(func() { videoStudioProjectSizeWalkEntries = previous })
	listed = listing{}
	capped := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects?sizes=1", nil, nil)
	if err := json.Unmarshal(capped.Body.Bytes(), &listed); err != nil || capped.Code != http.StatusOK || len(listed.Projects) != 1 || listed.Projects[0].StorageBytes != nil {
		t.Fatalf("a project over the walk budget must be listed without storage_bytes: %d %v %s", capped.Code, err, capped.Body.String())
	}
}

// A range export renders exactly the frames of the range, reports them, and names the range in the result.
func TestVideoStudioRangeRenderExportsTheRange(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	imported := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": "range-import"})
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
	asset := saved.Assets[0]
	saved.Tracks = []videostudio.Track{{ID: "video", Name: "Video", Kind: videostudio.TrackVideo, Clips: []videostudio.Clip{{ID: "clip", AssetID: asset.ID, Start: 0, Duration: 90, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain}}}}
	body, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	put := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if put.Code != http.StatusOK {
		t.Fatalf("save with a clip: %d %s", put.Code, put.Body.String())
	}
	etag = put.Header().Get("ETag")
	tooLong := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/jobs", bytes.NewBufferString(`{"kind":"render","range":{"start":15,"end":91}}`), map[string]string{"If-Match": etag, "Idempotency-Key": "range-long"})
	if tooLong.Code != http.StatusBadRequest || !strings.Contains(tooLong.Body.String(), "invalid_range") {
		t.Fatalf("a range past the last clip: %d %s", tooLong.Code, tooLong.Body.String())
	}
	jobID := videoStudioSubmitJob(t, s, writeToken, project.ID, `{"kind":"render","range":{"start":15,"end":75}}`, map[string]string{"If-Match": etag, "Idempotency-Key": "range-render"})
	rendered := waitVideoStudioJob(t, manager, jobID)
	if rendered.Status != "succeeded" || rendered.Artifact == nil {
		t.Fatalf("range render: %s %s %s", rendered.Status, rendered.Error, rendered.Message)
	}
	view := videoStudioAPIRequest(s, readToken, http.MethodGet, "/jobs/"+jobID, nil, nil)
	var result struct {
		Result struct {
			DurationFrames int            `json:"duration_frames"`
			Range          map[string]int `json:"range"`
		} `json:"result"`
	}
	if err := json.Unmarshal(view.Body.Bytes(), &result); err != nil || result.Result.DurationFrames != 60 || result.Result.Range["start"] != 15 || result.Result.Range["end"] != 75 {
		t.Fatalf("range result: %s (%v)", view.Body.String(), err)
	}
	exported := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), rendered.Artifact.Path)
	probed, err := videostudio.Probe(ctx, ffmpeg, exported)
	if err != nil {
		t.Fatal(err)
	}
	if frames := videostudio.ProjectRate(saved).FramesAt(probed.DurationUs); frames < 59 || frames > 61 {
		t.Fatalf("the range export has %d frames, want 60 (%d us)", frames, probed.DurationUs)
	}
}

// A restart in the middle of a duplicate leaves a copy folder without project.json. The job recorded the folder before
// copying, so the next start removes it with its media records. A copy that was finished (it holds project.json) is
// never touched, even when its job did not get to record the success.
func TestVideoStudioRestartRemovesInterruptedDuplicateCopy(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const orphanID = "123e4567-e89b-12d3-a456-426614175601"
	const orphanAsset, finishedAsset = "123e4567-e89b-12d3-a456-426614175611", "123e4567-e89b-12d3-a456-426614175612"
	videoStudioSeedMedia(t, s, orphanID, videostudio.Asset{ID: orphanAsset, Name: "half.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("half"))
	finished, _ := videoStudioAPICreate(t, s, writeToken)
	videoStudioSeedMedia(t, s, finished.ID, videostudio.Asset{ID: finishedAsset, Name: "done.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("done"))
	source, _ := videoStudioAPICreate(t, s, writeToken)

	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	interrupted := &videoStudioJob{ID: uidNew(), ProjectID: source.ID, Kind: "duplicate", Status: "running", CreatedAt: time.Now().UTC(), CopyProjectID: orphanID}
	completed := &videoStudioJob{ID: uidNew(), ProjectID: source.ID, Kind: "duplicate", Status: "running", CreatedAt: time.Now().UTC(), CopyProjectID: finished.ID}
	manager.mu.Lock()
	manager.jobs[interrupted.ID] = interrupted
	manager.jobs[completed.ID] = completed
	err = manager.persistLocked()
	manager.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// An abrupt exit: the worker stops without the orderly shutdown that would mark the jobs cancelled.
	manager.cancel()
	manager.wg.Wait()
	s.videoStudioMu.Lock()
	s.videoStudio = nil
	s.videoStudioMu.Unlock()
	restarted, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		a, b := restarted.job(interrupted.ID), restarted.job(completed.ID)
		if a != nil && b != nil && a.CopyProjectID == "" && b.CopyProjectID == "" {
			if a.Status != "interrupted" || b.Status != "interrupted" {
				t.Fatalf("jobs after restart: %s and %s, want interrupted", a.Status, b.Status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the restart did not check the recorded copies: %+v %+v", a, b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	entries, err := svc.ListFiles(ctx, "Documents/Video Studio")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name == orphanID {
			t.Fatal("the incomplete copy folder survived the restart")
		}
	}
	if _, ok := restarted.mediaRecord(orphanID, orphanAsset); ok {
		t.Fatal("the media record of the incomplete copy survived the restart")
	}
	if got := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+finished.ID, nil, nil); got.Code != http.StatusOK {
		t.Fatalf("the finished copy was touched: GET = %d %s", got.Code, got.Body.String())
	}
	if _, ok := restarted.mediaRecord(finished.ID, finishedAsset); !ok {
		t.Fatal("the media record of the finished copy was removed")
	}
	if _, _, err := svc.ReadFileBytes(ctx, videoStudioProjectPath(finished.ID)+"/media/"+finishedAsset+"_done.bin"); err != nil {
		t.Fatalf("the media file of the finished copy was removed: %v", err)
	}
}
