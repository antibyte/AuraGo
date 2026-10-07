package server

import (
	"testing"

	"aurago/internal/videostudio"
)

func TestVideoStudioRenderDoesNotStageZeroVolumeAudio(t *testing.T) {
	project := videostudio.Project{Tracks: []videostudio.Track{
		{ID: "silent-track", Kind: videostudio.TrackAudio, Clips: []videostudio.Clip{{ID: "silent", AssetID: "silent-audio", Volume: 0}}},
		{ID: "audible-track", Kind: videostudio.TrackAudio, Clips: []videostudio.Clip{{ID: "audible", AssetID: "audible-audio", Volume: 0.5}}},
		{ID: "visual-track", Kind: videostudio.TrackVideo, Clips: []videostudio.Clip{{ID: "visual", AssetID: "visual-video", Volume: 0}}},
		{ID: "muted-track", Kind: videostudio.TrackAudio, Muted: true, Clips: []videostudio.Clip{{ID: "muted", AssetID: "muted-audio", Volume: 1}}},
	}}
	active := videoStudioRenderAssetIDs(project)
	if _, ok := active["silent-audio"]; ok {
		t.Fatal("zero-volume audio clip was staged despite being inaudible")
	}
	if _, ok := active["audible-audio"]; !ok {
		t.Fatal("audible audio clip was not selected for staging")
	}
	if _, ok := active["visual-video"]; !ok {
		t.Fatal("zero-volume video clip was omitted even though its frames are visible")
	}
	if _, ok := active["muted-audio"]; ok {
		t.Fatal("muted audio clip was selected for staging")
	}
}
