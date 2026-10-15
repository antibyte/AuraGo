package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

func TestVideoStudioV2CreateUpgradeAndRates(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)

	created := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"PAL 4K","width":3840,"height":2160,"fps":25}`), nil)
	var pal videoStudioProjectItem
	if err := json.Unmarshal(created.Body.Bytes(), &pal); err != nil || created.Code != http.StatusCreated {
		t.Fatalf("create 25 fps: %d %s (%v)", created.Code, created.Body.String(), err)
	}
	if pal.Project.Version != videostudio.CurrentVersion || pal.Project.FPS != 25 || pal.Project.NTSC || pal.Project.Width != 3840 {
		t.Fatalf("created project = %+v", pal.Project)
	}
	for _, body := range []string{`{"name":"Bad","fps":23}`, `{"name":"Bad","fps":25,"ntsc":true}`, `{"name":"Bad","width":640,"height":480}`} {
		if bad := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects", bytes.NewBufferString(body), nil); bad.Code != http.StatusBadRequest {
			t.Fatalf("create %s: %d %s", body, bad.Code, bad.Body.String())
		}
	}

	// A version 1 file on disk is upgraded on read and saved as version 2.
	project, _ := videoStudioAPICreate(t, s, writeToken)
	file := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, videoStudioProjectPath(project.ID), "project.json")
	if err := os.WriteFile(file, []byte(`{"version":1,"name":"Old","width":1280,"height":720,"fps":30,"assets":[],"tracks":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil)
	var item videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &item); err != nil || read.Code != http.StatusOK || item.Project.Version != videostudio.CurrentVersion || item.Project.Name != "Old" {
		t.Fatalf("read v1: %d %s (%v)", read.Code, read.Body.String(), err)
	}
	body, _ := json.Marshal(item.Project)
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": read.Header().Get("ETag")})
	if saved.Code != http.StatusOK {
		t.Fatalf("save upgraded: %d %s", saved.Code, saved.Body.String())
	}
	if data, _ := os.ReadFile(file); !bytes.Contains(data, []byte(`"version":2`)) {
		t.Fatalf("saved file is not version 2: %s", data)
	}

	// API clients may still submit version 1 documents.
	submitted := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewBufferString(`{"version":1,"name":"Client v1","width":1280,"height":720,"fps":30,"assets":[],"tracks":[]}`), map[string]string{"If-Match": saved.Header().Get("ETag")})
	if err := json.Unmarshal(submitted.Body.Bytes(), &item); err != nil || submitted.Code != http.StatusOK || item.Project.Version != videostudio.CurrentVersion {
		t.Fatalf("submit v1: %d %s (%v)", submitted.Code, submitted.Body.String(), err)
	}

	// Asset frames follow the project rate; records from before version 2 only know 30 fps frames.
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	assetID := "123e4567-e89b-12d3-a456-426614174000"
	mediaPath := "media/" + assetID + "_clip.mp4"
	record := videoStudioMediaRecord{ProjectID: project.ID, Path: mediaPath, SHA256: "fixture", CreatedAt: time.Now().UTC(),
		Asset: videostudio.Asset{ID: assetID, Name: "clip.mp4", Path: mediaPath, Kind: videostudio.AssetVideo, DurationFrames: 60, Width: 64, Height: 36}}
	if err := manager.recordMedia(record); err != nil {
		t.Fatal(err)
	}
	sixty := fmt.Sprintf(`{"version":2,"name":"Rates","width":1280,"height":720,"fps":60,"assets":[{"id":%q,"name":"clip.mp4","kind":"video","duration_frames":1,"width":1,"height":1,"has_audio":false}],"tracks":[]}`, assetID)
	rated := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewBufferString(sixty), map[string]string{"If-Match": submitted.Header().Get("ETag")})
	if err := json.Unmarshal(rated.Body.Bytes(), &item); err != nil || rated.Code != http.StatusOK {
		t.Fatalf("save 60 fps: %d %s (%v)", rated.Code, rated.Body.String(), err)
	}
	if got := item.Project.Assets[0]; got.DurationFrames != 120 || got.DurationUs != 2_000_000 || got.Width != 64 {
		t.Fatalf("asset at 60 fps = %+v", got)
	}
	media := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media", nil, nil)
	var list struct {
		Media []videoStudioMediaItem `json:"media"`
	}
	if err := json.Unmarshal(media.Body.Bytes(), &list); err != nil || len(list.Media) != 1 || list.Media[0].Asset.DurationFrames != 120 {
		t.Fatalf("media list at 60 fps: %s (%v)", media.Body.String(), err)
	}

	status := videoStudioAPIRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	var limits struct {
		Limits struct {
			MaxDurationSeconds int              `json:"max_duration_seconds"`
			MaxTracks          map[string]int   `json:"max_tracks"`
			MaxClips           int              `json:"max_clips"`
			CanvasSizes        []map[string]int `json:"canvas_sizes"`
			FrameRates         []struct {
				FPS  int  `json:"fps"`
				NTSC bool `json:"ntsc"`
			} `json:"frame_rates"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &limits); err != nil {
		t.Fatal(err)
	}
	if limits.Limits.MaxDurationSeconds != 3600 || limits.Limits.MaxTracks["video"] != 8 || limits.Limits.MaxClips != 2000 || len(limits.Limits.CanvasSizes) != 11 || len(limits.Limits.FrameRates) != 8 {
		t.Fatalf("status limits = %+v", limits.Limits)
	}
}
