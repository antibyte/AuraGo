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

// Crop, flips and the effect stack are stored with the project (plan 2c, index §4.1).
//   - The PUT decodes them under DisallowUnknownFields.
//   - A lut effect may reference a lut asset of the project.
//   - GET returns them in Go's JSON form.
//   - A broken value is refused with 400 invalid_project, with the path of the field when Validate knows it.
func TestVideoStudioProjectEffectsRoundTrip(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID, lutID = "123e4567-e89b-12d3-a456-426614176301", "123e4567-e89b-12d3-a456-426614176302"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "clip.mp4", Kind: videostudio.AssetVideo, DurationFrames: 300, DurationUs: 10_000_000, VideoDurationUs: 10_000_000, Width: 64, Height: 36}, []byte("clip"))
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: lutID, Name: "grade.cube", Kind: videostudio.AssetLUT, LUTSize: 17, SizeBytes: 4}, []byte("cube"))
	crop := &videostudio.Crop{L: 0.1, T: 0, R: 0.1, B: 0.05}
	effects := []videostudio.EffectInstance{
		{ID: "color", On: true, Params: map[string]any{"contrast": 1.1, "exposure": 0.5}, Keyframes: map[string][]videostudio.Keyframe{
			"exposure": {{F: 0, V: 0, E: videostudio.EaseLinear}, {F: 25, V: 1, E: videostudio.EaseLinear}},
		}},
		{ID: "lut", On: true, Params: map[string]any{"intensity": 0.6, "lut": lutID}},
		{ID: "vignette", On: false, Params: map[string]any{}},
	}
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "clip.mp4"}, {ID: lutID, Name: "grade.cube"}}
	project.Project.Tracks = []videostudio.Track{{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{{
		ID: "c1", AssetID: assetID, Duration: 60, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: videostudio.FitContain,
		Crop: crop, FlipH: true, Effects: effects,
	}}}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save effects: %d %s", saved.Code, saved.Body.String())
	}
	etag = saved.Header().Get("ETag")
	stored := `"crop":{"l":0.1,"t":0,"r":0.1,"b":0.05},"flip_h":true,"effects":[{"id":"color","on":true,"params":{"contrast":1.1,"exposure":0.5},"keyframes":{"exposure":[{"f":0,"v":0,"e":"linear"},{"f":25,"v":1,"e":"linear"}]}},{"id":"lut","on":true,"params":{"intensity":0.6,"lut":"` + lutID + `"}},{"id":"vignette","on":false,"params":{}}]`
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), stored) {
		t.Fatalf("read effects: %d %s", read.Code, read.Body.String())
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	clip := item.Project.Tracks[0].Clips[0]
	if !reflect.DeepEqual(clip.Crop, crop) || !clip.FlipH || clip.FlipV || !reflect.DeepEqual(clip.Effects, effects) {
		t.Fatalf("the round trip changed the picture fields: %+v %v %v %+v", clip.Crop, clip.FlipH, clip.FlipV, clip.Effects)
	}
	lutRecorded := false
	for _, asset := range item.Project.Assets {
		lutRecorded = lutRecorded || asset.ID == lutID && asset.Kind == videostudio.AssetLUT && asset.LUTSize == 17
	}
	if !lutRecorded {
		t.Fatalf("the project lost its lut asset: %+v", item.Project.Assets)
	}
	for _, c := range []struct{ name, from, to, field string }{
		{"an intensity above 1", `"intensity":0.6`, `"intensity":1.5`, "clips/c1/effects/1/params/intensity"},
		{"a null params object", `"on":false,"params":{}`, `"on":false,"params":null`, "clips/c1/effects/2/params"},
		{"a LUT that is not in the project", `"lut":"` + lutID + `"`, `"lut":"123e4567-e89b-12d3-a456-426614179999"`, "clips/c1/effects/1/params/lut"},
		{"keyframes on the LUT reference", `"lut":"` + lutID + `"}`, `"lut":"` + lutID + `"},"keyframes":{"lut":[{"f":0,"v":0,"e":"linear"}]}`, "clips/c1/effects/1/keyframes/lut"},
		{"an all-zero crop", `"crop":{"l":0.1,"t":0,"r":0.1,"b":0.05}`, `"crop":{"l":0,"t":0,"r":0,"b":0}`, "clips/c1/crop"},
		{"an unknown key inside an effect entry", `"id":"vignette","on":false`, `"id":"vignette","open":true,"on":false`, ""},
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
