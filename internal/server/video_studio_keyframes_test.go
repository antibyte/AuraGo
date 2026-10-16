package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// Keyframes and speed ramps are stored with the project (plan index §4.1, §4.18): the PUT decodes them under
// DisallowUnknownFields, GET returns them in Go's JSON form, and broken ones are refused with 400.
func TestVideoStudioProjectKeyframesRoundTrip(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614176201"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "clip.mp4", Kind: videostudio.AssetVideo, DurationFrames: 300, DurationUs: 10_000_000, VideoDurationUs: 10_000_000, Width: 64, Height: 36}, []byte("clip"))
	keyframes := map[string][]videostudio.Keyframe{
		"opacity": {{F: 0, V: 0, E: videostudio.EaseLinear}, {F: 25, V: 1, E: videostudio.EaseHold}},
		"x":       {{F: 0, V: 0.1, E: videostudio.EaseInOut}, {F: 40, V: 0.4, E: videostudio.EaseLinear}},
	}
	ramp := []videostudio.RampPoint{{F: 0, Speed: 1, E: videostudio.EaseInOut}, {F: 49, Speed: 2, E: videostudio.EaseLinear}}
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "clip.mp4"}}
	project.Project.Tracks = []videostudio.Track{{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{{
		ID: "c1", AssetID: assetID, Duration: 60, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: videostudio.FitContain,
		Keyframes: keyframes, Ramp: ramp,
	}}}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save keyframes: %d %s", saved.Code, saved.Body.String())
	}
	etag = saved.Header().Get("ETag")
	const stored = `"keyframes":{"opacity":[{"f":0,"v":0,"e":"linear"},{"f":25,"v":1,"e":"hold"}],"x":[{"f":0,"v":0.1,"e":"ease_in_out"},{"f":40,"v":0.4,"e":"linear"}]},"ramp":[{"f":0,"speed":1,"e":"ease_in_out"},{"f":49,"speed":2,"e":"linear"}]`
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), stored) {
		t.Fatalf("read keyframes: %d %s", read.Code, read.Body.String())
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	clip := item.Project.Tracks[0].Clips[0]
	if !reflect.DeepEqual(clip.Keyframes, keyframes) || !reflect.DeepEqual(clip.Ramp, ramp) {
		t.Fatalf("the round trip changed the animation: %+v %+v", clip.Keyframes, clip.Ramp)
	}
	for _, c := range []struct{ name, from, to string }{
		{"an unknown key inside a keyframe", `"e":"hold"`, `"e":"hold","ease":"in"`},
		{"a repeated keyframe frame", `{"f":40,"v":0.4,"e":"linear"}`, `{"f":0,"v":0.4,"e":"linear"}`},
		{"an unknown easing", `"e":"hold"`, `"e":"bounce"`},
		{"a constant speed beside the ramp", `"ramp":[`, `"speed":2,"ramp":[`},
	} {
		broken := bytes.Replace(body, []byte(c.from), []byte(c.to), 1)
		if bytes.Equal(broken, body) {
			t.Fatalf("%s: %s not found in the request body", c.name, c.from)
		}
		refused := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(broken), map[string]string{"If-Match": etag})
		if refused.Code != http.StatusBadRequest || !strings.Contains(refused.Body.String(), `"invalid_project"`) {
			t.Fatalf("%s: %d %s", c.name, refused.Code, refused.Body.String())
		}
	}
}
