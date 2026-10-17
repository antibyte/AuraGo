package server

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// The status JSON serves the effect registry and the features this server's FFmpeg lacks next to the limits (spec §9,
// plan index §4.2 and §4.3). The four unavailable lists are arrays even without a runtime, so the editor never reads null.
func TestVideoStudioStatusServesRegistry(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	read := func(t *testing.T) map[string][]string {
		t.Helper()
		response := videoStudioAPIRequest(s, readToken, "GET", "/status", nil, nil)
		if response.Code != 200 {
			t.Fatalf("status: %d %s", response.Code, response.Body.String())
		}
		var body struct {
			Limits   json.RawMessage `json:"limits"`
			Registry struct {
				Version int `json:"version"`
				Base    []struct {
					ID          string `json:"id"`
					Keyframable bool   `json:"keyframable"`
				} `json:"base"`
				Effects     []json.RawMessage `json:"effects"`
				Transitions []struct {
					ID    string `json:"id"`
					XFade string `json:"xfade"`
				} `json:"transitions"`
				BlendModes  []json.RawMessage `json:"blend_modes"`
				AnimPresets []json.RawMessage `json:"anim_presets"`
				Looks       []json.RawMessage `json:"looks"`
			} `json:"registry"`
			Unavailable map[string]json.RawMessage `json:"unavailable"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		registry := body.Registry
		if len(body.Limits) == 0 || registry.Version != videostudio.RegistryVersion {
			t.Fatalf("status registry version = %d (limits %s)", registry.Version, body.Limits)
		}
		// The shape version is pinned apart from the constant: changing it is a contract change (plan index §4.2).
		if want := 1; registry.Version != want {
			t.Fatalf("status registry version = %d, want %d", registry.Version, want)
		}
		var ids, xfades []string
		for _, transition := range registry.Transitions {
			ids = append(ids, transition.ID)
			xfades = append(xfades, transition.XFade)
		}
		if strings.Join(ids, ",") != "dissolve,black,wipeleft,wiperight" || strings.Join(xfades, ",") != "fade,fadeblack,wiperight,wipeleft" {
			t.Fatalf("registry transitions = %v with xfade names %v", ids, xfades)
		}
		keyframableOpacity := false
		for _, param := range registry.Base {
			keyframableOpacity = keyframableOpacity || param.ID == "opacity" && param.Keyframable
		}
		if !keyframableOpacity {
			t.Fatalf("registry base lacks a keyframable opacity: %+v", registry.Base)
		}
		if registry.Effects == nil || registry.BlendModes == nil || registry.AnimPresets == nil || registry.Looks == nil {
			t.Fatalf("registry lists must be arrays, not null: %s", response.Body.String())
		}
		// 2c serves the eight built-in looks (plan index §4.2).
		if len(registry.Looks) != 8 {
			t.Fatalf("registry looks = %d, want 8", len(registry.Looks))
		}
		lists := make(map[string][]string, 4)
		for _, key := range []string{"effects", "transitions", "blend_modes", "anim_presets"} {
			raw := bytes.TrimSpace(body.Unavailable[key])
			if len(raw) == 0 || raw[0] != '[' {
				t.Fatalf("unavailable.%s = %q, want an array", key, raw)
			}
			var list []string
			if err := json.Unmarshal(raw, &list); err != nil {
				t.Fatalf("unavailable.%s: %v", key, err)
			}
			lists[key] = list
		}
		return lists
	}

	t.Run("ffmpeg missing", func(t *testing.T) {
		s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
		for key, list := range read(t) {
			if len(list) != 0 {
				t.Errorf("unavailable.%s = %v without an FFmpeg runtime", key, list)
			}
		}
	})
	t.Run("local ffmpeg", func(t *testing.T) {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			t.Skip("ffmpeg is not on PATH")
		}
		s.Cfg.VideoStudio.FFmpegPath = "" // PATH lookup, as configured by default
		// A disabled studio still probes the runtime but skips the slow encoder test the status runs otherwise.
		s.Cfg.VideoStudio.Enabled = false
		for _, id := range read(t)["transitions"] {
			if _, ok := videostudio.TransitionByID(videostudio.TransitionType(id)); !ok {
				t.Errorf("unavailable transition %q is not a registry transition", id)
			}
		}
	})
}
