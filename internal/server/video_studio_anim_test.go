package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// In and out animations are stored with the project (plan 2e, index §4.1).
//   - The PUT decodes them under DisallowUnknownFields. GET returns them in Go's JSON form: anim_in and anim_out after
//     effects, only when set, each with preset and duration.
//   - validateAnim refusals come back as 400 invalid_project with the field path. The duration range follows the
//     project's rate (AnimFrameRange). A duration longer than the clip is valid: the render clamps it.
func TestVideoStudioProjectAnimRoundTrip(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const videoID, songID = "123e4567-e89b-12d3-a456-426614176501", "123e4567-e89b-12d3-a456-426614176502"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: videoID, Name: "clip.mp4", Kind: videostudio.AssetVideo, DurationFrames: 300, DurationUs: 10_000_000, VideoDurationUs: 10_000_000, Width: 64, Height: 36}, []byte("clip"))
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: songID, Name: "song.mp3", Kind: videostudio.AssetAudio, DurationFrames: 300, DurationUs: 10_000_000, HasAudio: true}, []byte("song"))
	minFrames, maxFrames, _ := videostudio.AnimFrameRange(videostudio.ProjectRate(project.Project))
	in := &videostudio.ClipAnim{Preset: "pop", Duration: minFrames}
	out := &videostudio.ClipAnim{Preset: "slide_left", Duration: maxFrames} // longer than the 60-frame clip
	project.Project.Assets = []videostudio.Asset{{ID: videoID, Name: "clip.mp4"}, {ID: songID, Name: "song.mp3"}}
	project.Project.Tracks = []videostudio.Track{
		{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{{
			ID: "c1", AssetID: videoID, Duration: 60, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: videostudio.FitContain,
			AnimIn: in, AnimOut: out,
		}}},
		{ID: "a1", Kind: videostudio.TrackAudio, Name: "Audio 1", Clips: []videostudio.Clip{{ID: "s1", AssetID: songID, Duration: 60, Volume: 1}}},
	}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save animations: %d %s", saved.Code, saved.Body.String())
	}
	etag = saved.Header().Get("ETag")
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	stored := fmt.Sprintf(`"fit":"contain","anim_in":{"preset":"pop","duration":%d},"anim_out":{"preset":"slide_left","duration":%d}}`, minFrames, maxFrames)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), stored) {
		t.Fatalf("read animations: %d %s\nwant %s", read.Code, read.Body.String(), stored)
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if clip := item.Project.Tracks[0].Clips[0]; !reflect.DeepEqual(clip.AnimIn, in) || !reflect.DeepEqual(clip.AnimOut, out) {
		t.Fatalf("the round trip changed the animations: %+v %+v", clip.AnimIn, clip.AnimOut)
	}
	if song := item.Project.Tracks[1].Clips[0]; song.AnimIn != nil || song.AnimOut != nil {
		t.Fatalf("the audio clip gained an animation: %+v %+v", song.AnimIn, song.AnimOut)
	}
	inJSON := fmt.Sprintf(`"anim_in":{"preset":"pop","duration":%d}`, minFrames)
	outJSON := fmt.Sprintf(`"anim_out":{"preset":"slide_left","duration":%d}`, maxFrames)
	for _, c := range []struct{ name, from, to, field string }{
		{"an empty preset", inJSON, fmt.Sprintf(`"anim_in":{"preset":"","duration":%d}`, minFrames), "clips/c1/anim_in/preset"},
		{"an unknown preset", outJSON, fmt.Sprintf(`"anim_out":{"preset":"spin","duration":%d}`, maxFrames), "clips/c1/anim_out/preset"},
		{"a duration below the range", inJSON, fmt.Sprintf(`"anim_in":{"preset":"pop","duration":%d}`, minFrames-1), "clips/c1/anim_in/duration"},
		{"a duration above the range", outJSON, fmt.Sprintf(`"anim_out":{"preset":"slide_left","duration":%d}`, maxFrames+1), "clips/c1/anim_out/duration"},
		{"an animation on the audio clip", `"id":"s1",`, fmt.Sprintf(`"id":"s1","anim_in":{"preset":"fade","duration":%d},`, minFrames), "clips/s1/anim_in"},
		{"an unknown key inside an animation", `"anim_in":{"preset":"pop",`, `"anim_in":{"preset":"pop","ease":"out",`, ""},
	} {
		broken := bytes.Replace(body, []byte(c.from), []byte(c.to), 1)
		if bytes.Equal(broken, body) {
			t.Fatalf("%s: %s not found in the request body", c.name, c.from)
		}
		refused := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(broken), map[string]string{"If-Match": etag})
		var reply struct {
			Code  string `json:"code"`
			Field string `json:"field"`
		}
		if err := json.Unmarshal(refused.Body.Bytes(), &reply); err != nil {
			t.Fatalf("%s: %v in %s", c.name, err, refused.Body.String())
		}
		if refused.Code != http.StatusBadRequest || reply.Code != "invalid_project" || reply.Field != c.field {
			t.Fatalf("%s: %d %s, want 400 invalid_project with field %q", c.name, refused.Code, refused.Body.String(), c.field)
		}
	}
}
