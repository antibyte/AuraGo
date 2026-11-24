package storyplan

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewPlanDefaults(t *testing.T) {
	ref := ProfileRef{Provider: "google", Model: "veo-3.1-generate-preview", Resolution: "720p"}
	p := NewPlan("plan_1", "Ein Fischer fährt hinaus.", 30, "de", ref)
	if p.Version != Version || p.ID != "plan_1" || p.Status != StatusPlanning || p.Idea != "Ein Fischer fährt hinaus." || p.TargetSeconds != 30 || p.Language != "de" || p.Profile != ref {
		t.Fatalf("plan = %+v", p)
	}
	if p.Audio.ClipAudio != ClipAudioDuck || !p.Audio.Music.Instrumental {
		t.Fatalf("audio defaults = %+v", p.Audio)
	}
	data, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"scenes":[]`) || !strings.Contains(string(data), `"characters":[]`) {
		t.Fatalf("empty lists must encode as arrays: %s", data)
	}
}

func TestDecodeRoundTripAndRefusals(t *testing.T) {
	p := NewPlan("plan_1", "Idee", 30, "de", ProfileRef{Provider: "minimax", Model: "MiniMax-Hailuo-2.3", Resolution: "768P"})
	p.Scenes = []Scene{{ID: "s1", Title: "Hafen", VideoPrompt: "A harbour at dawn.", GenerateSeconds: 6, UseSeconds: 5.5,
		TransitionIn: Transition{Kind: TransitionCut}, Keyframe: Keyframe{Source: KeyframeNone}, State: SceneRuntimePending}}
	p.UpdatedAt = time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	data, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil || !reflect.DeepEqual(decoded, p) {
		t.Fatalf("round trip = %+v (%v), want %+v", decoded, err, p)
	}
	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{"unknown field", `{"version":1,"surprise":true}`, ErrPlanInvalid},
		{"unknown scene field", `{"version":1,"scenes":[{"id":"s1","camera":"dolly"}]}`, ErrPlanInvalid},
		{"wrong type", `{"version":1,"target_seconds":"30"}`, ErrPlanInvalid},
		{"truncated", `{"version":1,`, ErrPlanInvalid},
		{"trailing data", `{"version":1} {}`, ErrPlanInvalid},
		{"array", `[]`, ErrPlanInvalid},
		{"empty", ``, ErrPlanInvalid},
		{"other version", `{"version":2}`, ErrPlanVersion},
		{"null", `null`, ErrPlanVersion},
		{"too large", `{"version":1,"idea":"` + strings.Repeat("a", MaxPlanBytes) + `"}`, ErrPlanTooLarge},
	} {
		if _, err := Decode([]byte(tc.data)); !errors.Is(err, tc.want) {
			t.Errorf("%s: Decode error = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestMarshalRefusesOversizedPlans(t *testing.T) {
	p := NewPlan("plan_1", strings.Repeat("a", MaxPlanBytes), 30, "de", ProfileRef{})
	if _, err := Marshal(p); !errors.Is(err, ErrPlanTooLarge) {
		t.Fatalf("Marshal error = %v, want ErrPlanTooLarge", err)
	}
}

func TestNormalizeDerivesKeyframeSourceFromTransition(t *testing.T) {
	p := Plan{Version: Version, Scenes: []Scene{
		{ID: "s1", TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeChain}},
		{ID: "s2", TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeGenerated, ImagePrompt: "Harbour at dawn."}},
		{ID: "s3", TransitionIn: Transition{Kind: " Bridge "}, Keyframe: Keyframe{Source: KeyframeNone, ImagePrompt: "The boat at the harbour mouth."}},
		{ID: "s4", TransitionIn: Transition{Kind: TransitionCut}, Keyframe: Keyframe{Source: KeyframeChain}},
		{ID: "s5", TransitionIn: Transition{Kind: TransitionBlend, Type: "dissolve", Seconds: 1}, Keyframe: Keyframe{Source: " Bridge "}},
		{ID: "s6", TransitionIn: Transition{Kind: TransitionCut}, Keyframe: Keyframe{Source: KeyframeLibrary, AssetID: "img_1"}},
	}}
	Normalize(&p)
	var sources []string
	for _, s := range p.Scenes {
		sources = append(sources, s.Keyframe.Source)
	}
	// The first scene is forced to cut, so it loses chain. continue and bridge set the source; any other scene loses
	// chain and bridge and keeps the rest.
	if want := []string{KeyframeNone, KeyframeChain, KeyframeBridge, KeyframeNone, KeyframeNone, KeyframeLibrary}; !slices.Equal(sources, want) {
		t.Fatalf("keyframe sources = %v, want %v", sources, want)
	}
	if p.Scenes[2].Keyframe.ImagePrompt != "The boat at the harbour mouth." || p.Scenes[5].Keyframe.AssetID != "img_1" {
		t.Fatalf("Normalize dropped keyframe content: %+v", p.Scenes)
	}
}

func TestNormalize(t *testing.T) {
	p := Plan{Version: Version, Idea: "  Idee  ", Language: " de ", Style: " ruhig ", Audio: Audio{ClipAudio: " "},
		Scenes: []Scene{
			{Title: " Eins ", TransitionIn: Transition{Kind: "Blend", Type: "dissolve", Seconds: 1}},
			{ID: "s1", TransitionIn: Transition{Kind: " CONTINUE ", Type: "dissolve", Seconds: 1}, Keyframe: Keyframe{Source: " Chain "}},
			{ID: "s1", TransitionIn: Transition{Kind: "blend", Type: " Dissolve ", Seconds: 0.8}, State: "ready"},
			{ID: "bad id!", TransitionIn: Transition{Kind: "cut"}},
		}}
	Normalize(&p)
	var ids []string
	for _, s := range p.Scenes {
		ids = append(ids, s.ID)
	}
	// Only the empty, duplicate and invalid ids change: each takes s<position>, else the smallest free s<n>.
	if !slices.Equal(ids, []string{"s2", "s1", "s3", "s4"}) {
		t.Fatalf("scene ids = %v", ids)
	}
	if first := p.Scenes[0]; first.Title != "Eins" || first.TransitionIn != (Transition{Kind: TransitionCut}) || first.Keyframe.Source != KeyframeNone || first.State != SceneRuntimePending {
		t.Fatalf("first scene = %+v", first)
	}
	if second := p.Scenes[1]; second.TransitionIn != (Transition{Kind: TransitionContinue}) || second.Keyframe.Source != KeyframeChain {
		t.Fatalf("a non-blend transition must drop type and seconds: %+v", second)
	}
	if third := p.Scenes[2]; third.TransitionIn != (Transition{Kind: TransitionBlend, Type: "dissolve", Seconds: 0.8}) || third.State != "ready" {
		t.Fatalf("blend scene = %+v", third)
	}
	if p.Idea != "Idee" || p.Language != "de" || p.Style != "ruhig" || p.Audio.ClipAudio != ClipAudioDuck || p.Characters == nil {
		t.Fatalf("plan fields = %+v", p)
	}
	var empty Plan
	Normalize(&empty)
	if empty.Scenes == nil || empty.Characters == nil || len(empty.Scenes) != 0 || empty.Audio.ClipAudio != ClipAudioDuck {
		t.Fatalf("empty plan = %+v", empty)
	}
}
