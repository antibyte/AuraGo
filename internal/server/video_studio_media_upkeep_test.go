package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

// videoStudioProjectFileExists reports whether a project-relative file exists on the Desktop.
func videoStudioProjectFileExists(t *testing.T, svc *desktop.Service, projectID, path string) bool {
	t.Helper()
	_, _, err := svc.ReadFileBytes(context.Background(), videoStudioProjectPath(projectID)+"/"+path)
	return err == nil
}

// A removal that stopped after its first step (media file deleted, poster and record left) hides the asset at once and
// finishes on a retry of DELETE, at the next start (finishInterruptedMediaRemovals) and at the next listing.
func TestVideoStudioInterruptedMediaRemovalFinishes(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"123e4567-e89b-12d3-a456-426614175501", "123e4567-e89b-12d3-a456-426614175502", "123e4567-e89b-12d3-a456-426614175503"}
	interrupt := func(id string) videoStudioMediaRecord {
		record := videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: id, Name: "clip.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2, HasPoster: true}, []byte("clip"))
		videoStudioSeedPoster(t, s, project.ID, id)
		if _, err := manager.markMediaRemoving(project.ID, id); err != nil {
			t.Fatal(err)
		}
		if err := svc.DeletePath(context.Background(), videoStudioProjectPath(project.ID)+"/"+record.Path, desktop.SourceUser); err != nil {
			t.Fatal(err)
		}
		return record
	}
	gone := func(id string) bool {
		_, recorded := manager.mediaRecordForRemoval(project.ID, id)
		return !recorded && !videoStudioProjectFileExists(t, svc, project.ID, "posters/"+id+".jpg")
	}

	interrupt(ids[0])
	if _, ok := manager.mediaRecord(project.ID, ids[0]); ok {
		t.Fatal("a media record whose removal started is still visible")
	}
	if poster := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media/"+ids[0]+"/poster.jpg", nil, nil); poster.Code != http.StatusNotFound {
		t.Fatalf("poster of media being removed: %d", poster.Code)
	}
	if retry := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID+"/media/"+ids[0], nil, nil); retry.Code != http.StatusOK {
		t.Fatalf("retried removal: %d %s", retry.Code, retry.Body.String())
	}
	if !gone(ids[0]) {
		t.Fatal("the retried removal left its record or poster")
	}

	interrupt(ids[1])
	manager.finishInterruptedMediaRemovals()
	if !gone(ids[1]) {
		t.Fatal("the start did not finish the interrupted removal")
	}

	interrupt(ids[2])
	listed := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media", nil, nil)
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), ids[2]) {
		t.Fatalf("listing shows media being removed: %d %s", listed.Code, listed.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for !gone(ids[2]) {
		if time.Now().After(deadline) {
			t.Fatal("the listing did not finish the interrupted removal")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The sweep deletes media files and posters without a record and records without a media file, but never what the saved
// project uses, files the studio did not name, anything younger than the minimum age, or files while a job that writes
// media runs.
func TestVideoStudioMediaSweepRemovesLeftovers(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const (
		orphan     = "123e4567-e89b-12d3-a456-426614176601"
		posterOnly = "123e4567-e89b-12d3-a456-426614176602"
		used       = "123e4567-e89b-12d3-a456-426614176603"
		dangling   = "123e4567-e89b-12d3-a456-426614176604"
		kept       = "123e4567-e89b-12d3-a456-426614176605"
	)
	write := func(path string) {
		t.Helper()
		_ = svc.CreateDirectory(ctx, videoStudioProjectPath(project.ID)+"/media", desktop.SourceUser)
		_ = svc.CreateDirectory(ctx, videoStudioProjectPath(project.ID)+"/posters", desktop.SourceUser)
		if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(project.ID)+"/"+path, []byte("x"), desktop.SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	write("media/" + orphan + "_lost.bin")
	write("posters/" + posterOnly + ".jpg")
	write("media/" + used + "_restored.bin")
	write("media/notes.txt")
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: kept, Name: "kept.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("k"))
	record := videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: dangling, Name: "gone.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2, HasPoster: true}, []byte("g"))
	videoStudioSeedPoster(t, s, project.ID, dangling)
	if err := svc.DeletePath(ctx, videoStudioProjectPath(project.ID)+"/"+record.Path, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	// The saved project lists an asset whose record is missing (a lost manifest): its file must stay.
	doc := project.Project
	doc.Assets = []videostudio.Asset{{ID: used, Name: "restored.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFileBytes(ctx, videoStudioProjectPath(project.ID)+"/project.json", data, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	present := func() map[string]bool {
		return map[string]bool{
			"orphan": videoStudioProjectFileExists(t, svc, project.ID, "media/"+orphan+"_lost.bin"),
			"poster": videoStudioProjectFileExists(t, svc, project.ID, "posters/"+posterOnly+".jpg"),
			"used":   videoStudioProjectFileExists(t, svc, project.ID, "media/"+used+"_restored.bin"),
			"notes":  videoStudioProjectFileExists(t, svc, project.ID, "media/notes.txt"),
			"kept":   videoStudioProjectFileExists(t, svc, project.ID, "media/"+kept+"_kept.bin"),
		}
	}
	// Everything is younger than the minimum age, so nothing goes.
	manager.sweepProjectMedia(ctx, svc, project.ID)
	if got := present(); !got["orphan"] || !got["poster"] {
		t.Fatalf("young leftovers were deleted: %v", got)
	}
	if _, ok := manager.mediaRecordForRemoval(project.ID, dangling); !ok {
		t.Fatal("a young record without a file was removed")
	}

	previous := videoStudioLeftoverMinAge
	videoStudioLeftoverMinAge = -time.Minute
	defer func() { videoStudioLeftoverMinAge = previous }()
	// A running reverse job writes its file before it records it, so files stay while it runs.
	_, removeJob := videoStudioFakeJob(t, s, project.ID, "reverse", "running")
	manager.sweepProjectMedia(ctx, svc, project.ID)
	removeJob()
	if got := present(); !got["orphan"] || !got["poster"] {
		t.Fatalf("leftovers were deleted while a reverse job ran: %v", got)
	}

	manager.sweepProjectMedia(ctx, svc, project.ID)
	got := present()
	if got["orphan"] || got["poster"] {
		t.Fatalf("leftovers without a record survived the sweep: %v", got)
	}
	if !got["used"] || !got["notes"] || !got["kept"] {
		t.Fatalf("the sweep deleted a used, foreign or recorded file: %v", got)
	}
	if _, ok := manager.mediaRecordForRemoval(project.ID, dangling); ok {
		t.Fatal("the record without a media file survived the sweep")
	}
	if videoStudioProjectFileExists(t, svc, project.ID, "posters/"+dangling+".jpg") {
		t.Fatal("the poster of the record without a media file survived the sweep")
	}
	if _, ok := manager.mediaRecord(project.ID, kept); !ok {
		t.Fatal("the sweep removed a record with its media file")
	}
}
