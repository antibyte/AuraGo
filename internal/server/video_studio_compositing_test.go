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

// Blend mode, mask, mask keys and a chroma_key entry are stored with the project (plan 2d, index §4.1).
//   - The PUT decodes them under DisallowUnknownFields, and GET returns them in Go's JSON form: blend only when it is not
//     normal, the mask with all eight keys.
//   - validateBlend and validateMask refusals come back as 400 invalid_project with the field path. Mask key values are
//     refused by validateKeyframes, without a path, like every base key.
func TestVideoStudioProjectCompositingRoundTrip(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const assetID = "123e4567-e89b-12d3-a456-426614176401"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: assetID, Name: "green.mp4", Kind: videostudio.AssetVideo, DurationFrames: 300, DurationUs: 10_000_000, VideoDurationUs: 10_000_000, Width: 64, Height: 36}, []byte("clip"))
	mask := &videostudio.Mask{Shape: videostudio.MaskEllipse, X: 0.25, Y: 0.2, W: 0.5, H: 0.6, Rotation: 15, Feather: 0.04}
	keyframes := map[string][]videostudio.Keyframe{
		"mask.feather": {{F: 0, V: 0.02, E: videostudio.EaseHold}, {F: 20, V: 0.1, E: videostudio.EaseLinear}},
		"mask.x":       {{F: 0, V: 0.25, E: videostudio.EaseLinear}, {F: 30, V: 0.5, E: videostudio.EaseOut}},
	}
	effects := []videostudio.EffectInstance{{ID: "chroma_key", On: true,
		Params:    map[string]any{"color": "#00b140", "similarity": 0.3, "smoothness": 0.1, "spill": 0.5},
		Keyframes: map[string][]videostudio.Keyframe{"similarity": {{F: 0, V: 0.2, E: videostudio.EaseLinear}, {F: 30, V: 0.4, E: videostudio.EaseLinear}}},
	}}
	project.Project.Assets = []videostudio.Asset{{ID: assetID, Name: "green.mp4"}}
	project.Project.Tracks = []videostudio.Track{{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{{
		ID: "c1", AssetID: assetID, Duration: 60, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: videostudio.FitContain,
		Keyframes: keyframes, Blend: "multiply", Mask: mask, Effects: effects,
	}}}}
	body, err := json.Marshal(project.Project)
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + project.ID
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": etag})
	if saved.Code != http.StatusOK {
		t.Fatalf("save compositing: %d %s", saved.Code, saved.Body.String())
	}
	etag = saved.Header().Get("ETag")
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	for _, stored := range []string{
		`"keyframes":{"mask.feather":[{"f":0,"v":0.02,"e":"hold"},{"f":20,"v":0.1,"e":"linear"}],"mask.x":[{"f":0,"v":0.25,"e":"linear"},{"f":30,"v":0.5,"e":"ease_out"}]}`,
		`"blend":"multiply","mask":{"shape":"ellipse","x":0.25,"y":0.2,"w":0.5,"h":0.6,"rotation":15,"feather":0.04,"invert":false},"effects":[{"id":"chroma_key","on":true,"params":{"color":"#00b140","similarity":0.3,"smoothness":0.1,"spill":0.5},"keyframes":{"similarity":[{"f":0,"v":0.2,"e":"linear"},{"f":30,"v":0.4,"e":"linear"}]}}]`,
	} {
		if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), stored) {
			t.Fatalf("read compositing: %d %s\nwant %s", read.Code, read.Body.String(), stored)
		}
	}
	var item videoStudioProjectItem
	if err := json.Unmarshal(read.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	clip := item.Project.Tracks[0].Clips[0]
	if clip.Blend != "multiply" || !reflect.DeepEqual(clip.Mask, mask) || !reflect.DeepEqual(clip.Keyframes, keyframes) || !reflect.DeepEqual(clip.Effects, effects) {
		t.Fatalf("the round trip changed the compositing fields: %q %+v %+v %+v", clip.Blend, clip.Mask, clip.Keyframes, clip.Effects)
	}
	maskJSON := `"mask":{"shape":"ellipse","x":0.25,"y":0.2,"w":0.5,"h":0.6,"rotation":15,"feather":0.04,"invert":false}`
	for _, c := range []struct{ name, from, to, field string }{
		{"blend normal", `"blend":"multiply"`, `"blend":"normal"`, "clips/c1/blend"},
		{"an unknown blend mode", `"blend":"multiply"`, `"blend":"glow"`, "clips/c1/blend"},
		{"a mask shape star", `"shape":"ellipse"`, `"shape":"star"`, "clips/c1/mask/shape"},
		{"a feather above 0.5", `"feather":0.04`, `"feather":0.6`, "clips/c1/mask/feather"},
		{"mask keys without a mask", maskJSON, `"mask":null`, "clips/c1/mask"},
		{"a key colour that is not #rrggbb", `"color":"#00b140"`, `"color":"green"`, "clips/c1/effects/0/params/color"},
		{"a similarity below 0.01", `"similarity":0.3`, `"similarity":0`, "clips/c1/effects/0/params/similarity"},
		{"a mask key outside its row", `"mask.x":[{"f":0,"v":0.25`, `"mask.x":[{"f":0,"v":1.5`, ""},
		{"an unknown key inside the mask", `"invert":false}`, `"invert":false,"radius":2}`, ""},
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
