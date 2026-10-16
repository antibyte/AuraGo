package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/videostudio"
)

type videoStudioVersionList struct {
	Versions []videoStudioVersion `json:"versions"`
}

func videoStudioVersionsOf(t *testing.T, s *Server, token, projectID string) []videoStudioVersion {
	t.Helper()
	list := videoStudioAPIRequest(s, token, http.MethodGet, "/projects/"+projectID+"/versions", nil, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list versions: %d %s", list.Code, list.Body.String())
	}
	var body videoStudioVersionList
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Versions
}

// videoStudioSaveProject saves the project document with If-Match and returns the response.
func videoStudioSaveProject(t *testing.T, s *Server, token, projectID string, project videostudio.Project, etag string) (int, string, string) {
	t.Helper()
	body, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	w := videoStudioAPIRequest(s, token, http.MethodPut, "/projects/"+projectID, bytes.NewReader(body), map[string]string{"If-Match": etag})
	return w.Code, w.Header().Get("ETag"), w.Body.String()
}

func TestVideoStudioVersionsAPIAndRestore(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	path := "/projects/" + project.ID + "/versions"
	if denied := videoStudioAPIRequest(s, readToken, http.MethodPost, path, bytes.NewBufferString(`{"label":"x"}`), nil); denied.Code != http.StatusForbidden {
		t.Fatalf("read token saved a version: %d", denied.Code)
	}
	if long := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, bytes.NewBufferString(`{"label":"`+strings.Repeat("x", videoStudioVersionLabelRunes+1)+`"}`), nil); long.Code != http.StatusBadRequest {
		t.Fatalf("label over the limit: %d %s", long.Code, long.Body.String())
	}
	if control := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, bytes.NewBufferString(`{"label":"a\nb"}`), nil); control.Code != http.StatusBadRequest {
		t.Fatalf("label with a control character: %d %s", control.Code, control.Body.String())
	}
	created := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, bytes.NewBufferString(`{"label":"  Before the cut "}`), nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("save version: %d %s", created.Code, created.Body.String())
	}
	var saved struct {
		Version videoStudioVersion `json:"version"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version.Kind != videoStudioVersionManual || saved.Version.Label != "Before the cut" || saved.Version.Name != "API fixture" || saved.Version.SizeBytes <= 0 || !validVideoStudioAssetID(saved.Version.ID) {
		t.Fatalf("manual version entry: %+v", saved.Version)
	}
	if one := videoStudioAPIRequest(s, readToken, http.MethodGet, path+"/"+saved.Version.ID, nil, nil); one.Code != http.StatusOK || !strings.Contains(one.Body.String(), "project") {
		t.Fatalf("get version: %d %s", one.Code, one.Body.String())
	}
	if missing := videoStudioAPIRequest(s, readToken, http.MethodGet, path+"/123e4567-e89b-12d3-a456-426614174999", nil, nil); missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "version_not_found") {
		t.Fatalf("unknown version: %d %s", missing.Code, missing.Body.String())
	}

	// A save after the version: the project now differs from it, and the creation ETag is stale.
	project.Project.Name = "Changed"
	code, changedETag, body := videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, etag)
	if code != http.StatusOK {
		t.Fatalf("save: %d %s", code, body)
	}
	restore := path + "/" + saved.Version.ID + "/restore"
	if noMatch := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, nil); noMatch.Code != http.StatusPreconditionRequired {
		t.Fatalf("restore without If-Match: %d", noMatch.Code)
	}
	// Like a save, a malformed ETag is a bad request and a well-formed one that no longer matches is 412.
	if malformed := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": `"stale"`}); malformed.Code != http.StatusBadRequest {
		t.Fatalf("restore with a malformed ETag: %d", malformed.Code)
	}
	if stale := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": `"` + strings.Repeat("0", 64) + `"`}); stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("restore with a stale ETag: %d %s", stale.Code, stale.Body.String())
	}
	if stale := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": etag}); stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("restore with the ETag from before the save: %d %s", stale.Code, stale.Body.String())
	}
	if bad := videoStudioAPIRequest(s, writeToken, http.MethodPost, path+"/not-an-id/restore", nil, map[string]string{"If-Match": changedETag}); bad.Code != http.StatusNotFound {
		t.Fatalf("restore of an invalid version id: %d", bad.Code)
	}

	next := *s.ConfigSnapshot()
	next.VideoStudio.ReadOnly = true
	s.replaceConfigSnapshot(&next)
	if readOnly := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, bytes.NewBufferString(`{}`), nil); readOnly.Code != http.StatusForbidden {
		t.Fatalf("read-only version save: %d", readOnly.Code)
	}
	if readOnly := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": changedETag}); readOnly.Code != http.StatusForbidden {
		t.Fatalf("read-only restore: %d", readOnly.Code)
	}
	if listed := videoStudioVersionsOf(t, s, readToken, project.ID); len(listed) != 1 {
		t.Fatalf("read-only listing: %+v", listed)
	}
	next.VideoStudio.ReadOnly = false
	s.replaceConfigSnapshot(&next)

	// A render reads a snapshot of its own, so it does not block a restore.
	_, removeRender := videoStudioFakeJob(t, s, project.ID, "render", "running")
	defer removeRender()
	restored := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": changedETag})
	restoredETag := restored.Header().Get("ETag")
	if restored.Code != http.StatusOK || restoredETag == "" || restoredETag == changedETag {
		t.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(restored.Body.Bytes(), &item); err != nil || item.Project.Name != "API fixture" || item.ETag != restoredETag {
		t.Fatalf("restored project: %+v (%v)", item, err)
	}
	kinds := map[string]int{}
	for _, version := range videoStudioVersionsOf(t, s, readToken, project.ID) {
		kinds[version.Kind]++
		if version.Kind == videoStudioVersionBeforeRestore && version.Name != "Changed" {
			t.Fatalf("before_restore keeps the replaced document: %+v", version)
		}
	}
	if kinds[videoStudioVersionManual] != 1 || kinds[videoStudioVersionBeforeRestore] != 1 || len(kinds) != 2 {
		t.Fatalf("restore must keep a before_restore copy next to the manual version: %v", kinds)
	}
	// An editor that still holds the state from before the restore cannot overwrite it.
	project.Project.Name = "Lost edit"
	if code, _, body := videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, changedETag); code != http.StatusPreconditionFailed {
		t.Fatalf("save over the restore with the old ETag: %d %s", code, body)
	}
	if code, _, body := videoStudioSaveProject(t, s, writeToken, project.ID, item.Project, restoredETag); code != http.StatusOK {
		t.Fatalf("save after the restore: %d %s", code, body)
	}
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"name":"API fixture"`) {
		t.Fatalf("project after the restore: %d %s", read.Code, read.Body.String())
	}
}

func TestVideoStudioVersionRestoreRefusesDirtyStateAndRemovedMedia(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614174501"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "still.bin", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("still"))
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "Renamed still", Folder: "Stills"}}
	code, etag, body := videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, etag)
	if code != http.StatusOK {
		t.Fatalf("save with the asset: %d %s", code, body)
	}
	path := "/projects/" + project.ID + "/versions"
	created := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, nil, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("save version without a body: %d %s", created.Code, created.Body.String())
	}
	var saved struct {
		Version videoStudioVersion `json:"version"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	project.Project.Assets = nil
	code, etag, body = videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, etag)
	if code != http.StatusOK {
		t.Fatalf("save without the asset: %d %s", code, body)
	}
	restore := path + "/" + saved.Version.ID + "/restore"
	before := len(videoStudioVersionsOf(t, s, readToken, project.ID))
	for _, kind := range []string{"probe", "generate", "reverse", "still"} {
		for _, status := range []string{"queued", "running"} {
			_, remove := videoStudioFakeJob(t, s, project.ID, kind, status)
			dirty := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": etag})
			remove()
			if dirty.Code != http.StatusConflict || !strings.Contains(dirty.Body.String(), "version_restore_dirty") {
				t.Fatalf("restore during a %s %s job: %d %s", status, kind, dirty.Code, dirty.Body.String())
			}
		}
	}
	if versions := videoStudioVersionsOf(t, s, readToken, project.ID); len(versions) != before {
		t.Fatalf("a refused restore kept a snapshot: %+v", versions)
	}
	// The version still names its media while the asset exists: it restores the owned name and folder too.
	restored := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": etag})
	if restored.Code != http.StatusOK {
		t.Fatalf("restore with the media present: %d %s", restored.Code, restored.Body.String())
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(restored.Body.Bytes(), &item); err != nil || len(item.Project.Assets) != 1 || item.Project.Assets[0].Name != "Renamed still" || item.Project.Assets[0].Folder != "Stills" {
		t.Fatalf("restored assets: %+v (%v)", item.Project.Assets, err)
	}
	item.Project.Assets = nil
	code, etag, body = videoStudioSaveProject(t, s, writeToken, project.ID, item.Project, restored.Header().Get("ETag"))
	if code != http.StatusOK {
		t.Fatalf("save without the asset again: %d %s", code, body)
	}
	if removed := videoStudioAPIRequest(s, writeToken, http.MethodDelete, "/projects/"+project.ID+"/media/"+assetID, nil, nil); removed.Code != http.StatusOK {
		t.Fatalf("remove the media: %d %s", removed.Code, removed.Body.String())
	}
	gone := videoStudioAPIRequest(s, writeToken, http.MethodPost, restore, nil, map[string]string{"If-Match": etag})
	if gone.Code != http.StatusConflict || !strings.Contains(gone.Body.String(), "project_asset_invalid") {
		t.Fatalf("restore of a version whose media was removed: %d %s", gone.Code, gone.Body.String())
	}
}

func TestVideoStudioVersionsRespectQuota(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	next := *s.ConfigSnapshot()
	next.VideoStudio.MaxProjectSizeMB = 1
	s.replaceConfigSnapshot(&next)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Close to the limit with room left for a save but not for a snapshot of the document and its index.
	if err := svc.WriteFileBytes(context.Background(), videoStudioProjectPath(project.ID)+"/media/ballast.bin", make([]byte, (1<<20)-1024), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	full := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/versions", bytes.NewBufferString(`{}`), nil)
	if full.Code != http.StatusRequestEntityTooLarge || !strings.Contains(full.Body.String(), "project_size_limit") {
		t.Fatalf("version over the quota: %d %s", full.Code, full.Body.String())
	}
	// The automatic snapshot of a save is skipped when it does not fit; the save itself still succeeds.
	project.Project.Name = "Saved near the limit"
	if code, _, body := videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, etag); code != http.StatusOK {
		t.Fatalf("save near the limit: %d %s", code, body)
	}
	if versions := videoStudioVersionsOf(t, s, readToken, project.ID); len(versions) != 0 {
		t.Fatalf("an automatic version went over the quota: %+v", versions)
	}
}

func TestVideoStudioAutoVersionOnSaveIsRateLimitedAndPruned(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	previous := videoStudioNow
	videoStudioNow = func() time.Time { return now }
	t.Cleanup(func() { videoStudioNow = previous })
	save := func(name string) {
		t.Helper()
		project.Project.Name = name
		code, next, body := videoStudioSaveProject(t, s, writeToken, project.ID, project.Project, etag)
		if code != http.StatusOK {
			t.Fatalf("save %s: %d %s", name, code, body)
		}
		etag = next
	}
	save("First")
	save("Second")
	first := videoStudioVersionsOf(t, s, readToken, project.ID)
	if len(first) != 1 || first[0].Kind != videoStudioVersionAuto || first[0].Name != "API fixture" {
		t.Fatalf("automatic versions after two quick saves: %+v, want the created document once", first)
	}
	now = now.Add(9 * time.Minute)
	save("Too soon")
	if got := len(videoStudioVersionsOf(t, s, readToken, project.ID)); got != 1 {
		t.Fatalf("automatic versions nine minutes later: %d, want 1", got)
	}
	now = now.Add(2 * time.Minute)
	save("Third")
	if got := len(videoStudioVersionsOf(t, s, readToken, project.ID)); got != 2 {
		t.Fatalf("automatic versions after a later save: %d, want 2", got)
	}
	for i := 0; i < videoStudioAutoVersionLimit+2; i++ {
		now = now.Add(11 * time.Minute)
		save(fmt.Sprintf("Round %d", i))
	}
	versions := videoStudioVersionsOf(t, s, readToken, project.ID)
	if len(versions) != videoStudioAutoVersionLimit {
		t.Fatalf("automatic versions after pruning: %d, want %d", len(versions), videoStudioAutoVersionLimit)
	}
	// The oldest snapshots went first and their documents with them; the list stays oldest first.
	if versions[len(versions)-1].Name != fmt.Sprintf("Round %d", videoStudioAutoVersionLimit) {
		t.Fatalf("newest automatic version: %+v", versions[len(versions)-1])
	}
	for i := 1; i < len(versions); i++ {
		if !versions[i].CreatedAt.After(versions[i-1].CreatedAt) {
			t.Fatalf("versions out of order at %d: %+v", i, versions)
		}
	}
	if missing := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/versions/"+first[0].ID, nil, nil); missing.Code != http.StatusNotFound {
		t.Fatalf("pruned version is still readable: %d", missing.Code)
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReadFileBytes(context.Background(), videoStudioVersionDocPath(project.ID, first[0].ID)); err == nil {
		t.Fatal("pruned version document survived")
	}
}

func TestVideoStudioVersionPruningKeepsNewestPerPool(t *testing.T) {
	var versions []videoStudioVersion
	for i := 0; i < 25; i++ {
		kind := videoStudioVersionAuto
		if i%5 == 4 {
			kind = videoStudioVersionBeforeRestore
		}
		versions = append(versions, videoStudioVersion{ID: fmt.Sprintf("a%02d", i), Kind: kind})
		if i%2 == 0 {
			versions = append(versions, videoStudioVersion{ID: fmt.Sprintf("m%02d", i/2), Kind: videoStudioVersionManual})
		}
	}
	kept, removed := pruneVideoStudioVersions(versions)
	removedIDs := map[string]bool{}
	for _, version := range removed {
		removedIDs[version.ID] = true
	}
	// Automatic and before_restore snapshots share one pool of 20; manual versions have their own pool of 10.
	want := []string{"a00", "a01", "a02", "a03", "a04", "m00", "m01", "m02"}
	if len(removed) != len(want) {
		t.Fatalf("removed %d versions, want %v", len(removed), want)
	}
	for _, id := range want {
		if !removedIDs[id] {
			t.Fatalf("version %s was kept, removed: %+v", id, removed)
		}
	}
	var expected []string
	for _, version := range versions {
		if !removedIDs[version.ID] {
			expected = append(expected, version.ID)
		}
	}
	if len(kept) != len(expected) {
		t.Fatalf("kept %d versions, want %d", len(kept), len(expected))
	}
	for i, version := range kept {
		if version.ID != expected[i] {
			t.Fatalf("kept order changed at %d: %s, want %s", i, version.ID, expected[i])
		}
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	if !autoVersionDue(videoStudioVersionIndex{}, now) {
		t.Fatal("a project without versions needs its first automatic version")
	}
	recent := videoStudioVersionIndex{Versions: []videoStudioVersion{{CreatedAt: now.Add(-time.Hour)}, {CreatedAt: now.Add(-9 * time.Minute)}, {CreatedAt: now.Add(-2 * time.Hour)}}}
	if autoVersionDue(recent, now) {
		t.Fatal("a version nine minutes old must suppress the automatic version")
	}
	if !autoVersionDue(recent, now.Add(time.Minute)) {
		t.Fatal("a version ten minutes old must allow the automatic version")
	}
}
