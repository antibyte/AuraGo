# Part A — storyplan core, capability profile, config

Part A builds the pure-Go plan model and validator (`internal/videostudio/storyplan`), the server capability profile (`internal/server/video_studio_generation_profile.go`), which now also decides manual generation's durations and image modes, and the `video_studio.plan` config keys with every place the repo needs for a new nested key: template, loader default, config merger, provider-reference inventory, decisions-only guard and the configuration page in 16 languages.

Conventions for every task in this part:
- Run all commands from the worktree root in Git Bash.
- Go and locale JSON files are LF. Never convert them.
- After writing Go files, run `gofmt -w` on them before the PASS run.
- **Impact checks (CLAUDE.md).** Before editing an existing function, run GitNexus impact from the main checkout root (`C:\Users\Andi\Documents\repo\AuraGo`):
  - Command: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
  - Report the callers and the risk. The callers listed in each task were checked by text search.
  - Stop and report to the controller on HIGH or CRITICAL.
  - Treat UNKNOWN as unresolved, never as safe.
- Commit after every task: `git add` the listed files, then `git commit -m "<subject>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"`.
- Run `git status --short` afterwards and make sure no new file is ignored.

---

### Task A1: storyplan plan model (profile.go, plan.go)

**Files:**
- Create: `internal/videostudio/storyplan/profile.go` (`Profile`, `Reason*` constants)
- Create: `internal/videostudio/storyplan/plan.go` (package doc, constants, caps, types, `NewPlan`, `Decode`, `Marshal`, `Normalize`)
- Test: `internal/videostudio/storyplan/plan_test.go`

- [ ] **Step 1: Write the failing test.** Create `internal/videostudio/storyplan/plan_test.go`:

```go
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
```

- [ ] **Step 2: Run it.** `go test ./internal/videostudio/storyplan/... -count=1`. Expected: FAIL, a build failure with `undefined: NewPlan` (and other undefined names).

- [ ] **Step 3: Implement.** Create `internal/videostudio/storyplan/profile.go`:

```go
package storyplan

// Reasons a Profile reports plan_supported false (Profile.Reason).
const (
	ReasonGenerationUnavailable       = "generation_unavailable"          // video generation is off or lacks a key or provider type
	ReasonProviderImagesPublicURLOnly = "provider_images_public_url_only" // start and end images only as public URLs (Agnes)
	ReasonProviderUnsupported         = "provider_unsupported"            // a provider type without a Video Studio generation mode
)

// Profile is what the configured video generation API can do at one resolution (spec §4.1). The server derives it
// (videoStudioGenerationProfile in internal/server); storyplan only reads it.
//
// PlanSupported says whether plan mode can use the provider and Reason says why not. Provider is the lowercased
// provider type ("google_veo" is reported as "google"), Model the model id sent to the API and Resolution the
// resolution the profile is for. DurationsSeconds are the allowed generate_seconds. AspectRatios is empty when the clip
// follows the first frame (AspectFromFirstFrame). FirstFrame and LastFrame are the image inputs; NativeAudio says the
// clip has sound. MaxPromptChars caps every prompt, CostPerClipEstimate is in USD and Parallel bounds concurrent
// generations.
type Profile struct {
	PlanSupported        bool     `json:"plan_supported"`
	Reason               string   `json:"reason,omitempty"`
	Provider             string   `json:"provider"`
	Model                string   `json:"model"`
	Resolution           string   `json:"resolution"`
	DurationsSeconds     []int    `json:"durations_seconds"`
	AspectRatios         []string `json:"aspect_ratios"`
	AspectFromFirstFrame bool     `json:"aspect_from_first_frame"`
	FirstFrame           bool     `json:"first_frame"`
	LastFrame            bool     `json:"last_frame"`
	NativeAudio          bool     `json:"native_audio"`
	MaxPromptChars       int      `json:"max_prompt_chars"`
	CostPerClipEstimate  float64  `json:"cost_per_clip_estimate"`
	Parallel             int      `json:"parallel"`
}
```

Create `internal/videostudio/storyplan/plan.go`:

```go
// Package storyplan is the Video Studio plan (plan.json): the storyboard the planning agent writes and the user
// edits, its normalization, and the validator shared by the agent tools, the storyboard panel and approval
// (docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md §5 and §8). It is pure Go: the server derives
// the capability profile and the validator environment from its configuration and passes them in.
package storyplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Version is the plan.json format version.
const Version = 1

// Plan status.
const (
	StatusPlanning = "planning"
	StatusReview   = "review"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusPartial  = "partial"
	StatusFailed   = "failed"
	StatusStopped  = "stopped"
)

// Transition kinds of a scene's transition_in (spec §4.2). The first scene always uses cut.
const (
	TransitionCut      = "cut"
	TransitionContinue = "continue"
	TransitionBridge   = "bridge"
	TransitionBlend    = "blend"
)

// Keyframe sources: where a scene's first frame comes from.
const (
	KeyframeGenerated = "generated"
	KeyframeLibrary   = "library"
	KeyframeChain     = "chain"
	KeyframeBridge    = "bridge"
	KeyframeNone      = "none"
)

// Scene states. A new scene is pending; the runner (slice 2) writes the others.
const (
	SceneRuntimePending = "pending"
	SceneKeyframeReady  = "keyframe_ready"
	SceneGenerating     = "generating"
	SceneReady          = "ready"
	SceneFailed         = "failed"
	SceneUncertain      = "uncertain"
	SceneStale          = "stale"
)

// Clip audio modes of the assembled timeline (spec §10).
const (
	ClipAudioKeep = "keep"
	ClipAudioDuck = "duck"
	ClipAudioMute = "mute"
)

// Limits (spec §4.3 and §5). Text limits count characters (runes).
const (
	MaxPlanBytes                 = 256 << 10
	MaxScenes                    = 12
	MaxTotalSeconds              = 120
	MaxChainScenes               = 4
	MaxIdeaChars                 = 4000
	MaxTitleChars                = 120
	MaxNarrationChars            = 600
	MaxStyleChars                = 2000
	MaxCharacters                = 6
	MaxCharacterDescriptionChars = 600
	MaxNoteChars                 = 1000
	MinTargetSeconds             = 5
)

// maxSceneIDLength bounds a scene id. Normalize replaces longer ids and ids with characters other than ASCII letters,
// digits, "_" and "-".
const maxSceneIDLength = 32

// Decode and Marshal errors.
var (
	// ErrPlanTooLarge reports JSON larger than MaxPlanBytes.
	ErrPlanTooLarge = errors.New("plan_too_large")
	// ErrPlanInvalid reports data that is not exactly one plan object: bad syntax, an unknown field, a wrong type or
	// trailing data.
	ErrPlanInvalid = errors.New("plan_invalid")
	// ErrPlanVersion reports a version other than Version.
	ErrPlanVersion = errors.New("plan_version_unsupported")
)

// ProfileRef is the snapshot of the capability profile a plan was made for.
type ProfileRef struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Resolution string `json:"resolution"`
}

// Character is a recurring person or figure. Scenes name characters; the server prepends their descriptions.
type Character struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	ReferenceAssetID string `json:"reference_asset_id,omitempty"`
}

// Transition joins a scene to the previous one. Type (a Video Studio transition name) and Seconds are for blend only.
type Transition struct {
	Kind    string  `json:"kind"`
	Type    string  `json:"type,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
}

// Keyframe is the scene's first frame: a generated image (ImagePrompt), a project image (AssetID with source
// library), the previous clip's last frame (chain), a bridge frame (bridge) or none (text only).
type Keyframe struct {
	Source      string `json:"source"`
	ImagePrompt string `json:"image_prompt,omitempty"`
	AssetID     string `json:"asset_id,omitempty"`
}

// Narration is the spoken text of a scene; AssetID and MeasuredSeconds are server-owned (preview step).
type Narration struct {
	Text            string  `json:"text,omitempty"`
	AssetID         string  `json:"asset_id,omitempty"`
	MeasuredSeconds float64 `json:"measured_seconds,omitempty"`
}

// Scene is one generated clip of the storyboard.
type Scene struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	VideoPrompt     string     `json:"video_prompt"`
	NegativePrompt  string     `json:"negative_prompt,omitempty"`
	GenerateSeconds int        `json:"generate_seconds"`
	UseSeconds      float64    `json:"use_seconds"`
	TransitionIn    Transition `json:"transition_in"`
	Keyframe        Keyframe   `json:"keyframe"`
	Narration       Narration  `json:"narration"`
	State           string     `json:"state"`
	ClipAssetID     string     `json:"clip_asset_id,omitempty"`
	StillAssetID    string     `json:"still_asset_id,omitempty"`
	TimelineClipID  string     `json:"timeline_clip_id,omitempty"`
	Cost            float64    `json:"cost,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// Music is the music part of the plan.
type Music struct {
	Prompt       string `json:"prompt,omitempty"`
	Instrumental bool   `json:"instrumental"`
	AssetID      string `json:"asset_id,omitempty"`
}

// Audio holds voice, music and the clip audio mode.
type Audio struct {
	Voice     string `json:"voice,omitempty"`
	Music     Music  `json:"music"`
	ClipAudio string `json:"clip_audio"`
}

// Cost is the estimated and the spent generation cost in USD.
type Cost struct {
	Estimate float64 `json:"estimate"`
	Spent    float64 `json:"spent"`
}

// Plan is plan.json, one per Video Studio project.
type Plan struct {
	Version       int         `json:"version"`
	ID            string      `json:"id"`
	Status        string      `json:"status"`
	Idea          string      `json:"idea"`
	TargetSeconds int         `json:"target_seconds"`
	Language      string      `json:"language"`
	Profile       ProfileRef  `json:"profile"`
	Style         string      `json:"style"`
	Characters    []Character `json:"characters"`
	Scenes        []Scene     `json:"scenes"`
	Audio         Audio       `json:"audio"`
	Cost          Cost        `json:"cost"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// NewPlan returns an empty plan in status planning: no scenes and no characters (empty lists, not null), clip audio
// duck and instrumental music.
func NewPlan(id, idea string, targetSeconds int, language string, ref ProfileRef) Plan {
	return Plan{
		Version:       Version,
		ID:            id,
		Status:        StatusPlanning,
		Idea:          idea,
		TargetSeconds: targetSeconds,
		Language:      language,
		Profile:       ref,
		Characters:    []Character{},
		Scenes:        []Scene{},
		Audio:         Audio{Music: Music{Instrumental: true}, ClipAudio: ClipAudioDuck},
	}
}

// Decode parses plan.json. It refuses data over MaxPlanBytes (ErrPlanTooLarge); unknown fields, wrong types and
// trailing data (ErrPlanInvalid); and versions other than Version (ErrPlanVersion). It never panics and does not
// normalize.
func Decode(data []byte) (Plan, error) {
	if len(data) > MaxPlanBytes {
		return Plan{}, ErrPlanTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plan Plan
	if err := decoder.Decode(&plan); err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrPlanInvalid, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Plan{}, fmt.Errorf("%w: data after the plan object", ErrPlanInvalid)
	}
	if plan.Version != Version {
		return Plan{}, fmt.Errorf("%w: version %d", ErrPlanVersion, plan.Version)
	}
	return plan, nil
}

// Marshal encodes p as JSON and refuses a result over MaxPlanBytes (ErrPlanTooLarge).
func Marshal(p Plan) ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPlanBytes {
		return nil, ErrPlanTooLarge
	}
	return data, nil
}

// Normalize prepares p for storage and validation:
//   - It trims every string and lowercases transition kinds and types, keyframe sources and clip_audio.
//   - It forces the first scene's transition to cut and drops type and seconds from every transition that is not a
//     blend.
//   - It derives the keyframe source from the transition: chain for a continue scene, bridge for a bridge scene, and
//     none for a chain or bridge source on any other scene (the first scene included).
//   - It sets an empty keyframe source to none, an empty scene state to pending and an empty clip_audio to duck.
//   - It replaces nil characters and scenes with empty lists.
//   - It gives scenes with an empty, duplicate or invalid id (longer than 32 characters, or characters other than ASCII
//     letters, digits, "_" and "-") the id "s<position>", or the smallest free "s<n>" when that is taken. Other scenes
//     keep their ids.
//
// It never validates.
func Normalize(p *Plan) {
	p.ID = strings.TrimSpace(p.ID)
	p.Status = strings.TrimSpace(p.Status)
	p.Idea = strings.TrimSpace(p.Idea)
	p.Language = strings.TrimSpace(p.Language)
	p.Style = strings.TrimSpace(p.Style)
	p.Profile = ProfileRef{Provider: strings.TrimSpace(p.Profile.Provider), Model: strings.TrimSpace(p.Profile.Model), Resolution: strings.TrimSpace(p.Profile.Resolution)}
	if p.Characters == nil {
		p.Characters = []Character{}
	}
	for i := range p.Characters {
		c := &p.Characters[i]
		c.Name = strings.TrimSpace(c.Name)
		c.Description = strings.TrimSpace(c.Description)
		c.ReferenceAssetID = strings.TrimSpace(c.ReferenceAssetID)
	}
	if p.Scenes == nil {
		p.Scenes = []Scene{}
	}
	for i := range p.Scenes {
		normalizeScene(&p.Scenes[i], i == 0)
	}
	normalizeSceneIDs(p.Scenes)
	a := &p.Audio
	a.Voice = strings.TrimSpace(a.Voice)
	a.Music.Prompt = strings.TrimSpace(a.Music.Prompt)
	a.Music.AssetID = strings.TrimSpace(a.Music.AssetID)
	a.ClipAudio = strings.ToLower(strings.TrimSpace(a.ClipAudio))
	if a.ClipAudio == "" {
		a.ClipAudio = ClipAudioDuck
	}
}

func normalizeScene(s *Scene, first bool) {
	s.ID = strings.TrimSpace(s.ID)
	s.Title = strings.TrimSpace(s.Title)
	s.VideoPrompt = strings.TrimSpace(s.VideoPrompt)
	s.NegativePrompt = strings.TrimSpace(s.NegativePrompt)
	s.TransitionIn.Kind = strings.ToLower(strings.TrimSpace(s.TransitionIn.Kind))
	s.TransitionIn.Type = strings.ToLower(strings.TrimSpace(s.TransitionIn.Type))
	if first {
		s.TransitionIn.Kind = TransitionCut
	}
	if s.TransitionIn.Kind != TransitionBlend {
		s.TransitionIn.Type = ""
		s.TransitionIn.Seconds = 0
	}
	s.Keyframe.Source = strings.ToLower(strings.TrimSpace(s.Keyframe.Source))
	// The transition decides chain and bridge (spec §4.2): a continue scene starts on the previous clip's last frame
	// and a bridge scene on the boundary keyframe, and no other scene can use either source.
	switch {
	case s.TransitionIn.Kind == TransitionContinue:
		s.Keyframe.Source = KeyframeChain
	case s.TransitionIn.Kind == TransitionBridge:
		s.Keyframe.Source = KeyframeBridge
	case s.Keyframe.Source == KeyframeChain || s.Keyframe.Source == KeyframeBridge:
		s.Keyframe.Source = KeyframeNone
	}
	if s.Keyframe.Source == "" {
		s.Keyframe.Source = KeyframeNone
	}
	s.Keyframe.ImagePrompt = strings.TrimSpace(s.Keyframe.ImagePrompt)
	s.Keyframe.AssetID = strings.TrimSpace(s.Keyframe.AssetID)
	s.Narration.Text = strings.TrimSpace(s.Narration.Text)
	s.Narration.AssetID = strings.TrimSpace(s.Narration.AssetID)
	s.State = strings.TrimSpace(s.State)
	if s.State == "" {
		s.State = SceneRuntimePending
	}
	s.ClipAssetID = strings.TrimSpace(s.ClipAssetID)
	s.StillAssetID = strings.TrimSpace(s.StillAssetID)
	s.TimelineClipID = strings.TrimSpace(s.TimelineClipID)
	s.Error = strings.TrimSpace(s.Error)
}

// normalizeSceneIDs keeps every valid first occurrence of an id and gives the other scenes "s<position>", or the
// smallest free "s<n>" when that is taken.
func normalizeSceneIDs(scenes []Scene) {
	used := make(map[string]bool, len(scenes))
	var missing []int
	for i := range scenes {
		id := scenes[i].ID
		if !validSceneID(id) || used[id] {
			missing = append(missing, i)
			continue
		}
		used[id] = true
	}
	for _, i := range missing {
		id := fmt.Sprintf("s%d", i+1)
		for n := 1; used[id]; n++ {
			id = fmt.Sprintf("s%d", n)
		}
		used[id] = true
		scenes[i].ID = id
	}
}

func validSceneID(id string) bool {
	if id == "" || len(id) > maxSceneIDLength {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run it.** `gofmt -w internal/videostudio/storyplan && go test ./internal/videostudio/storyplan/... -count=1`. Expected: `ok  aurago/internal/videostudio/storyplan`.

- [ ] **Step 5: Commit.**
```
git add internal/videostudio/storyplan/profile.go internal/videostudio/storyplan/plan.go internal/videostudio/storyplan/plan_test.go
git commit -m "feat(video-studio): storyplan plan model with decode, marshal and normalize" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A2: word count and narration budget (words.go)

**Files:**
- Create: `internal/videostudio/storyplan/words.go` (`WordBudget`, `CountWords`)
- Test: `internal/videostudio/storyplan/words_test.go`

- [ ] **Step 1: Write the failing test.** Create `internal/videostudio/storyplan/words_test.go`:

```go
package storyplan

import "testing"

func TestWordBudget(t *testing.T) {
	for _, tc := range []struct {
		language string
		seconds  float64
		want     int
	}{
		{"de", 5, 9},       // floor(2.2 × 5) − 2
		{"DE-at", 5, 9},    // first two letters, any case
		{"en", 4, 8},       // floor(2.5 × 4) − 2
		{"en-US", 9.5, 21}, // floor(23.75) − 2
		{"fr", 10, 21},     // other languages: floor(2.3 × 10) − 2
		{"", 10, 21},
		{"en", 0.5, 0}, // never below 0
		{"de", 0, 0},
		{"de", -3, 0},
	} {
		if got := WordBudget(tc.language, tc.seconds); got != tc.want {
			t.Errorf("WordBudget(%q, %v) = %d, want %d", tc.language, tc.seconds, got, tc.want)
		}
	}
}

func TestCountWords(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"", 0},
		{"Der Fischer fährt hinaus.", 4},
		{"  Hallo — Welt  ", 2}, // a lone dash is not a word
		{"渔夫出海了", 3},          // five Han characters: 5 ÷ 2 rounded up
		{"漁師が海へ出る", 4},        // seven Han and Hiragana characters
		{"AuraGo 很好", 2},
	} {
		if got := CountWords(tc.text); got != tc.want {
			t.Errorf("CountWords(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/videostudio/storyplan/... -count=1`. Expected: FAIL, a build failure with `undefined: WordBudget`.

- [ ] **Step 3: Implement.** Create `internal/videostudio/storyplan/words.go`:

```go
package storyplan

import (
	"math"
	"strings"
	"unicode"
)

// Narration speaking rates in words per second of use_seconds (spec §8), and the words every scene keeps free.
const (
	wordsPerSecondGerman  = 2.2
	wordsPerSecondEnglish = 2.5
	wordsPerSecondOther   = 2.3
	wordBudgetReserve     = 2
)

// WordBudget is the number of narration words that fit into useSeconds: floor(rate × useSeconds) − 2, at least 0. The
// rate is 2.2 words per second for German, 2.5 for English and 2.3 for every other language. The language is matched
// by its first two letters, case-insensitive ("de-AT" counts as German).
func WordBudget(language string, useSeconds float64) int {
	if useSeconds <= 0 {
		return 0
	}
	rate := wordsPerSecondOther
	switch languagePrefix(language) {
	case "de":
		rate = wordsPerSecondGerman
	case "en":
		rate = wordsPerSecondEnglish
	}
	// The small epsilon keeps products like 2.2 × 5 = 11.000000000000002 and 2.3 × 10 on the whole number.
	return max(int(math.Floor(rate*useSeconds+1e-9))-wordBudgetReserve, 0)
}

// CountWords counts the words of a narration. The text is split on Unicode white space, and a piece counts as a word
// when it holds a letter or a digit (a lone dash does not). Chinese and Japanese are written without spaces, so their
// characters (Han, Hiragana, Katakana) are counted on their own: every two characters count as one word, rounded up.
// The count does not depend on the plan language; the script decides.
func CountWords(text string) int {
	words, cjk := 0, 0
	for _, field := range strings.Fields(text) {
		hasWord := false
		for _, r := range field {
			switch {
			case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana):
				cjk++
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				hasWord = true
			}
		}
		if hasWord {
			words++
		}
	}
	return words + (cjk+1)/2
}

func languagePrefix(language string) string {
	l := strings.ToLower(strings.TrimSpace(language))
	if len(l) > 2 {
		l = l[:2]
	}
	return l
}
```

- [ ] **Step 4: Run it.** `gofmt -w internal/videostudio/storyplan && go test ./internal/videostudio/storyplan/... -count=1`. Expected: `ok`.

- [ ] **Step 5: Commit.**
```
git add internal/videostudio/storyplan/words.go internal/videostudio/storyplan/words_test.go
git commit -m "feat(video-studio): storyplan word count and narration budget" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A3: validator (validate.go)

**Files:**
- Create: `internal/videostudio/storyplan/validate.go` (`Env`, `Issue`, `Code*`, `Phase*`, `Validate`, `HasBlocking`, `PhaseComplete`, `TotalSeconds`)
- Test: `internal/videostudio/storyplan/validate_test.go`. The fixture transition names come from `internal/videostudio/effects.go` `registryTransitions` (served as `EffectRegistry().Transitions`); storyplan does not import videostudio.

- [ ] **Step 1: Write the failing test.** Create `internal/videostudio/storyplan/validate_test.go`:

```go
package storyplan

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// registryTransitionNames are the transition ids of Video Studio's effect registry (videostudio.EffectRegistry()
// .Transitions, defined in internal/videostudio/effects.go registryTransitions). storyplan does not import
// videostudio: the server passes these names in Env.KnownTransitions.
var registryTransitionNames = []string{
	"dissolve", "black", "wipeleft", "wiperight", "fadewhite", "fadegrays", "pixelize", "wipeup", "wipedown",
	"smoothleft", "smoothright", "diagtl", "diagbr", "radial", "slideleft", "slideright", "slideup", "slidedown",
	"squeezeh", "zoomin", "circleopen", "circleclose", "circlecrop", "rectcrop",
}

// allIssueCodes lists every code of validate.go; TestValidateReportsEveryRule has a case for each one.
var allIssueCodes = []string{
	CodeNoScenes, CodeTooManyScenes, CodeTargetSecondsInvalid, CodeTotalLengthMismatch, CodeTotalLengthTooLong,
	CodeIdeaTooLong, CodeStyleTooLong, CodeTooManyCharacters, CodeCharacterInvalid, CodeCharacterReferenceInvalid,
	CodeTitleMissing, CodeTitleTooLong, CodePromptMissing, CodePromptTooLong, CodeNegativePromptTooLong,
	CodeDurationUnsupported, CodeUseSecondsInvalid, CodeTransitionUnknown, CodeTransitionUnsupported,
	CodeFirstSceneTransition, CodeChainTooLong, CodeBlendTypeUnknown, CodeBlendTypeUnavailable, CodeBlendOverlapInvalid,
	CodeKeyframeSourceInvalid, CodeKeyframePromptMissing, CodeKeyframeAssetInvalid, CodeNarrationUnavailable,
	CodeNarrationTooLong, CodeMusicUnavailable, CodeClipAudioInvalid, CodePlanUnsupportedProvider, CodeProfileChanged,
}

func validEnv() Env {
	return Env{
		Profile: Profile{
			PlanSupported: true, Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P",
			DurationsSeconds: []int{6, 10}, AspectRatios: []string{}, AspectFromFirstFrame: true,
			FirstFrame: true, LastFrame: true, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2,
		},
		ImageGeneration:        true,
		TTS:                    true,
		Music:                  true,
		UnavailableTransitions: []string{"fadegrays"},
		KnownTransitions:       append([]string{}, registryTransitionNames...),
		ImageAssetIDs:          map[string]bool{"img_harbor": true},
	}
}

// validPlan is a 28.5 s plan for a 30 s target: a cut, a seamless continuation and a one-second dissolve.
func validPlan() Plan {
	p := NewPlan("plan_fixture", "A fisherman heads out at dawn.", 30, "de", ProfileRef{Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P"})
	p.Status = StatusReview
	p.Style = "Muted dawn colours, slow handheld camera."
	p.Characters = []Character{{Name: "Fischer", Description: "Old man in a yellow raincoat."}}
	p.Scenes = []Scene{
		{ID: "s1", Title: "Hafen im Nebel", VideoPrompt: "Slow push-in on a foggy harbour at dawn, Fischer unties his boat.", GenerateSeconds: 10, UseSeconds: 9.5,
			TransitionIn: Transition{Kind: TransitionCut}, Keyframe: Keyframe{Source: KeyframeGenerated, ImagePrompt: "Foggy harbour at dawn, Fischer beside his boat."},
			Narration: Narration{Text: "Der Tag beginnt im Hafen."}, State: SceneRuntimePending},
		{ID: "s2", Title: "Hinaus", VideoPrompt: "The boat glides out of the harbour, the camera follows.", GenerateSeconds: 10, UseSeconds: 10,
			TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeChain}, State: SceneRuntimePending},
		{ID: "s3", Title: "Die Bucht", VideoPrompt: "Wide shot of the bay as the sun rises.", GenerateSeconds: 10, UseSeconds: 10,
			TransitionIn: Transition{Kind: TransitionBlend, Type: "dissolve", Seconds: 1}, Keyframe: Keyframe{Source: KeyframeLibrary, AssetID: "img_harbor"}, State: SceneRuntimePending},
	}
	p.Audio.Music.Prompt = "Calm acoustic guitar."
	return p
}

func TestValidateAcceptsAValidPlan(t *testing.T) {
	p, env := validPlan(), validEnv()
	before, _ := json.Marshal(p)
	issues := Validate(p, env)
	if issues == nil || len(issues) != 0 {
		t.Fatalf("issues = %#v, want an empty list", issues)
	}
	if after, _ := json.Marshal(p); string(before) != string(after) {
		t.Fatal("Validate changed the plan")
	}
	if got := TotalSeconds(p); got != 28.5 {
		t.Fatalf("TotalSeconds = %v, want 28.5", got)
	}
}

func TestValidateReportsEveryRule(t *testing.T) {
	type ruleCase struct {
		name   string
		mutate func(p *Plan, env *Env)
		code   string
		scene  string
	}
	cases := []ruleCase{
		{"provider without plan mode", func(p *Plan, env *Env) {
			env.Profile.PlanSupported, env.Profile.Reason = false, ReasonProviderImagesPublicURLOnly
		}, CodePlanUnsupportedProvider, ""},
		{"profile changed", func(p *Plan, env *Env) { p.Profile.Model = "MiniMax-Hailuo-2.3" }, CodeProfileChanged, ""},
		{"target below 5 s", func(p *Plan, env *Env) { p.TargetSeconds = 4 }, CodeTargetSecondsInvalid, ""},
		{"target above 120 s", func(p *Plan, env *Env) { p.TargetSeconds = 121 }, CodeTargetSecondsInvalid, ""},
		{"length off the target", func(p *Plan, env *Env) { p.TargetSeconds = 60 }, CodeTotalLengthMismatch, ""},
		{"longer than 120 s", func(p *Plan, env *Env) {
			env.Profile.DurationsSeconds = []int{6, 10, 120}
			p.Scenes[0].GenerateSeconds, p.Scenes[0].UseSeconds, p.TargetSeconds = 120, 120, 120
		}, CodeTotalLengthTooLong, ""},
		{"idea too long", func(p *Plan, env *Env) { p.Idea = strings.Repeat("i", MaxIdeaChars+1) }, CodeIdeaTooLong, ""},
		{"style too long", func(p *Plan, env *Env) { p.Style = strings.Repeat("s", MaxStyleChars+1) }, CodeStyleTooLong, ""},
		{"too many characters", func(p *Plan, env *Env) {
			for len(p.Characters) <= MaxCharacters {
				p.Characters = append(p.Characters, Character{Name: fmt.Sprintf("Figur %d", len(p.Characters))})
			}
		}, CodeTooManyCharacters, ""},
		{"character without name", func(p *Plan, env *Env) { p.Characters[0].Name = "" }, CodeCharacterInvalid, ""},
		{"character name twice", func(p *Plan, env *Env) {
			p.Characters = append(p.Characters, Character{Name: "fischer", Description: "The twin."})
		}, CodeCharacterInvalid, ""},
		{"character reference outside the project", func(p *Plan, env *Env) { p.Characters[0].ReferenceAssetID = "img_missing" }, CodeCharacterReferenceInvalid, ""},
		{"no scenes", func(p *Plan, env *Env) { p.Scenes = []Scene{} }, CodeNoScenes, ""},
		{"too many scenes", func(p *Plan, env *Env) {
			for len(p.Scenes) <= MaxScenes {
				extra := p.Scenes[2]
				extra.ID = fmt.Sprintf("x%d", len(p.Scenes))
				p.Scenes = append(p.Scenes, extra)
			}
		}, CodeTooManyScenes, ""},
		{"title missing", func(p *Plan, env *Env) { p.Scenes[1].Title = " " }, CodeTitleMissing, "s2"},
		{"title too long", func(p *Plan, env *Env) { p.Scenes[1].Title = strings.Repeat("t", MaxTitleChars+1) }, CodeTitleTooLong, "s2"},
		{"video prompt missing", func(p *Plan, env *Env) { p.Scenes[1].VideoPrompt = "" }, CodePromptMissing, "s2"},
		{"video prompt too long", func(p *Plan, env *Env) { p.Scenes[1].VideoPrompt = strings.Repeat("p", 2001) }, CodePromptTooLong, "s2"},
		{"image prompt too long", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.ImagePrompt = strings.Repeat("p", 2001) }, CodePromptTooLong, "s1"},
		{"music prompt too long", func(p *Plan, env *Env) { p.Audio.Music.Prompt = strings.Repeat("m", MaxStyleChars+1) }, CodePromptTooLong, ""},
		{"negative prompt too long", func(p *Plan, env *Env) { p.Scenes[1].NegativePrompt = strings.Repeat("n", 2001) }, CodeNegativePromptTooLong, "s2"},
		{"duration the provider lacks", func(p *Plan, env *Env) { p.Scenes[0].GenerateSeconds, p.Scenes[0].UseSeconds = 7, 6 }, CodeDurationUnsupported, "s1"},
		{"use seconds zero", func(p *Plan, env *Env) { p.Scenes[1].UseSeconds = 0 }, CodeUseSecondsInvalid, "s2"},
		{"use seconds above generate", func(p *Plan, env *Env) { p.Scenes[1].UseSeconds = 10.5 }, CodeUseSecondsInvalid, "s2"},
		{"unknown transition kind", func(p *Plan, env *Env) { p.Scenes[1].TransitionIn.Kind = "fade" }, CodeTransitionUnknown, "s2"},
		{"continue without first frame", func(p *Plan, env *Env) { env.Profile.FirstFrame = false }, CodeTransitionUnsupported, "s2"},
		{"bridge without last frame", func(p *Plan, env *Env) {
			env.Profile.LastFrame = false
			p.Scenes[1].TransitionIn.Kind, p.Scenes[1].Keyframe.Source = TransitionBridge, KeyframeBridge
		}, CodeTransitionUnsupported, "s2"},
		{"first scene not cut", func(p *Plan, env *Env) { p.Scenes[0].TransitionIn.Kind = TransitionContinue }, CodeFirstSceneTransition, "s1"},
		{"chain of five scenes", func(p *Plan, env *Env) {
			p.Scenes[2].TransitionIn, p.Scenes[2].Keyframe = Transition{Kind: TransitionContinue}, Keyframe{Source: KeyframeChain}
			for _, id := range []string{"c4", "c5"} {
				p.Scenes = append(p.Scenes, Scene{ID: id, Title: "Weiter", VideoPrompt: "The boat keeps moving.", GenerateSeconds: 6, UseSeconds: 6,
					TransitionIn: Transition{Kind: TransitionContinue}, Keyframe: Keyframe{Source: KeyframeChain}, State: SceneRuntimePending})
			}
		}, CodeChainTooLong, "c5"},
		{"unknown blend type", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Type = "spin" }, CodeBlendTypeUnknown, "s3"},
		{"blend type missing on this server", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Type = "fadegrays" }, CodeBlendTypeUnavailable, "s3"},
		{"blend too short", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Seconds = 0.2 }, CodeBlendOverlapInvalid, "s3"},
		{"blend longer than the scenes allow", func(p *Plan, env *Env) { p.Scenes[2].TransitionIn.Seconds = 9.6 }, CodeBlendOverlapInvalid, "s3"},
		{"unknown keyframe source", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.Source = "photo" }, CodeKeyframeSourceInvalid, "s1"},
		{"chain keyframe without continue", func(p *Plan, env *Env) { p.Scenes[2].Keyframe = Keyframe{Source: KeyframeChain} }, CodeKeyframeSourceInvalid, "s3"},
		{"bridge keyframe without bridge", func(p *Plan, env *Env) { p.Scenes[2].Keyframe = Keyframe{Source: KeyframeBridge} }, CodeKeyframeSourceInvalid, "s3"},
		{"generated keyframe without image generation", func(p *Plan, env *Env) { env.ImageGeneration = false }, CodeKeyframeSourceInvalid, "s1"},
		{"generated keyframe without prompt", func(p *Plan, env *Env) { p.Scenes[0].Keyframe.ImagePrompt = "" }, CodeKeyframePromptMissing, "s1"},
		{"library keyframe outside the project", func(p *Plan, env *Env) { p.Scenes[2].Keyframe.AssetID = "img_other" }, CodeKeyframeAssetInvalid, "s3"},
		{"narration without TTS", func(p *Plan, env *Env) { env.TTS = false }, CodeNarrationUnavailable, "s1"},
		{"narration over the word budget", func(p *Plan, env *Env) { p.Scenes[0].Narration.Text = strings.Repeat("Wort ", 19) }, CodeNarrationTooLong, "s1"},
		{"narration over 600 characters", func(p *Plan, env *Env) { p.Scenes[0].Narration.Text = strings.Repeat("a", MaxNarrationChars+1) }, CodeNarrationTooLong, "s1"},
		{"measured narration spills over 1 s", func(p *Plan, env *Env) { p.Scenes[0].Narration.MeasuredSeconds = 10.5 }, CodeNarrationTooLong, "s1"},
		{"music without music generation", func(p *Plan, env *Env) { env.Music = false }, CodeMusicUnavailable, ""},
		{"unknown clip audio", func(p *Plan, env *Env) { p.Audio.ClipAudio = "loud" }, CodeClipAudioInvalid, ""},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.code] = true
		t.Run(tc.name, func(t *testing.T) {
			p, env := validPlan(), validEnv()
			tc.mutate(&p, &env)
			issues := Validate(p, env)
			for _, issue := range issues {
				if issue.Code == tc.code && issue.SceneID == tc.scene {
					if issue.Blocking != (tc.code != CodeProfileChanged) || issue.Message == "" || issue.Field == "" {
						t.Fatalf("issue %+v: wrong blocking flag, empty message or empty field", issue)
					}
					return
				}
			}
			t.Fatalf("no %s issue for scene %q in %+v", tc.code, tc.scene, issues)
		})
	}
	for _, code := range allIssueCodes {
		if !covered[code] {
			t.Errorf("no test case for %s", code)
		}
	}
}

func TestValidateOrdersPlanIssuesFirst(t *testing.T) {
	p, env := validPlan(), validEnv()
	p.Scenes[1].Title = ""
	p.Scenes[0].VideoPrompt = ""
	p.Audio.ClipAudio = "loud"
	var got []string
	for _, issue := range Validate(p, env) {
		got = append(got, issue.Code+"@"+issue.SceneID)
	}
	want := []string{CodeClipAudioInvalid + "@", CodePromptMissing + "@s1", CodeTitleMissing + "@s2"}
	if !slices.Equal(got, want) {
		t.Fatalf("issue order = %v, want %v", got, want)
	}
}

func TestHasBlockingIgnoresWarnings(t *testing.T) {
	p, env := validPlan(), validEnv()
	p.Profile.Resolution = "1080P"
	issues := Validate(p, env)
	if len(issues) != 1 || issues[0].Code != CodeProfileChanged || HasBlocking(issues) {
		t.Fatalf("a changed profile alone must only warn: %+v", issues)
	}
	p.Scenes[0].Title = ""
	if !HasBlocking(Validate(p, env)) {
		t.Fatal("a missing title must block")
	}
}

// A bridge's boundary keyframe K is an image of the project, or a generated image (image_prompt) with image
// generation configured.
func TestValidateBridgeKeyframe(t *testing.T) {
	bridged := func(keyframe Keyframe) Plan {
		p := validPlan()
		p.Scenes[1].TransitionIn, p.Scenes[1].Keyframe = Transition{Kind: TransitionBridge}, keyframe
		return p
	}
	noImages := validEnv()
	noImages.ImageGeneration = false
	prompt := "The boat at the harbour mouth, wide shot, dawn light."
	for _, tc := range []struct {
		name string
		plan Plan
		env  Env
		want string // the keyframe issue of s2; "" for none
	}{
		{"generated K", bridged(Keyframe{Source: KeyframeBridge, ImagePrompt: prompt}), validEnv(), ""},
		{"project image K", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_harbor"}), validEnv(), ""},
		{"project image K without image generation", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_harbor"}), noImages, ""},
		{"no K", bridged(Keyframe{Source: KeyframeBridge}), validEnv(), CodeKeyframePromptMissing},
		{"K outside the project", bridged(Keyframe{Source: KeyframeBridge, AssetID: "img_other"}), validEnv(), CodeKeyframeAssetInvalid},
		{"prompt without image generation", bridged(Keyframe{Source: KeyframeBridge, ImagePrompt: prompt}), noImages, CodeKeyframeAssetInvalid},
	} {
		var got []string
		for _, issue := range Validate(tc.plan, tc.env) {
			if issue.SceneID == "s2" && strings.HasPrefix(issue.Code, "keyframe_") {
				got = append(got, issue.Code)
			}
		}
		var want []string
		if tc.want != "" {
			want = []string{tc.want}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: keyframe issues of s2 = %v, want %v", tc.name, got, want)
		}
	}
}

func TestPhaseComplete(t *testing.T) {
	env := validEnv()
	outline := validPlan()
	for i := range outline.Scenes {
		outline.Scenes[i].VideoPrompt = ""
		outline.Scenes[i].Keyframe.ImagePrompt = ""
	}
	full := validPlan()
	broken := validPlan()
	broken.Scenes[2].TransitionIn.Type = "spin"
	empty := validPlan()
	empty.Scenes = []Scene{}
	noKind := validPlan()
	noKind.Scenes[1].TransitionIn.Kind = ""
	for _, tc := range []struct {
		name  string
		plan  Plan
		phase string
		want  bool
	}{
		{"outline done", outline, PhaseOutline, true},
		{"details missing prompts", outline, PhaseDetails, false},
		{"details done", full, PhaseDetails, true},
		{"details ignore blocking issues", broken, PhaseDetails, true},
		{"check passes", full, PhaseCheck, true},
		{"check blocked", broken, PhaseCheck, false},
		{"no scenes", empty, PhaseOutline, false},
		{"missing transition kind", noKind, PhaseOutline, false},
		{"unknown phase", full, "revise", false},
	} {
		if got := PhaseComplete(tc.plan, tc.phase, env); got != tc.want {
			t.Errorf("%s: PhaseComplete(%s) = %v, want %v", tc.name, tc.phase, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/videostudio/storyplan/... -count=1`. Expected: FAIL, a build failure with `undefined: Env` / `undefined: Validate`.

- [ ] **Step 3: Implement.** Create `internal/videostudio/storyplan/validate.go`:

```go
package storyplan

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Env is everything the validator needs besides the plan. The server builds it (videoStudioPlanEnv).
type Env struct {
	// Profile is the capability profile for the plan's resolution.
	Profile Profile
	// ImageGeneration reports configured image generation (keyframe source generated).
	ImageGeneration bool
	// TTS reports text-to-speech ready for Video Studio (tools.TTSReady(cfg, "video_studio")).
	TTS bool
	// Music reports configured music generation.
	Music bool
	// UnavailableTransitions are registry transitions this server's FFmpeg lacks.
	UnavailableTransitions []string
	// KnownTransitions are the transition names of the Video Studio effect registry.
	KnownTransitions []string
	// ImageAssetIDs are the image assets of the project (keyframe source library, character references).
	ImageAssetIDs map[string]bool
}

// Issue is one finding. Field is a JSON path into the plan with 0-based indexes ("scenes[2].generate_seconds",
// "target_seconds"); Message is short English for the agent and logs (the UI translates by Code).
type Issue struct {
	Code     string `json:"code"`
	SceneID  string `json:"scene_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

// Issue codes (spec §8). Every code blocks except CodeProfileChanged, which warns.
const (
	CodeNoScenes                  = "no_scenes"
	CodeTooManyScenes             = "too_many_scenes"
	CodeTargetSecondsInvalid      = "target_seconds_invalid"
	CodeTotalLengthMismatch       = "total_length_mismatch"
	CodeTotalLengthTooLong        = "total_length_too_long"
	CodeIdeaTooLong               = "idea_too_long"
	CodeStyleTooLong              = "style_too_long"
	CodeTooManyCharacters         = "too_many_characters"
	CodeCharacterInvalid          = "character_invalid"
	CodeCharacterReferenceInvalid = "character_reference_invalid"
	CodeTitleMissing              = "title_missing"
	CodeTitleTooLong              = "title_too_long"
	CodePromptMissing             = "prompt_missing"
	CodePromptTooLong             = "prompt_too_long"
	CodeNegativePromptTooLong     = "negative_prompt_too_long"
	CodeDurationUnsupported       = "duration_unsupported"
	CodeUseSecondsInvalid         = "use_seconds_invalid"
	CodeTransitionUnknown         = "transition_unknown"
	CodeTransitionUnsupported     = "transition_unsupported"
	CodeFirstSceneTransition      = "first_scene_transition"
	CodeChainTooLong              = "chain_too_long"
	CodeBlendTypeUnknown          = "blend_type_unknown"
	CodeBlendTypeUnavailable      = "blend_type_unavailable"
	CodeBlendOverlapInvalid       = "blend_overlap_invalid"
	CodeKeyframeSourceInvalid     = "keyframe_source_invalid"
	CodeKeyframePromptMissing     = "keyframe_prompt_missing"
	CodeKeyframeAssetInvalid      = "keyframe_asset_invalid"
	CodeNarrationUnavailable      = "narration_unavailable"
	CodeNarrationTooLong          = "narration_too_long"
	CodeMusicUnavailable          = "music_unavailable"
	CodeClipAudioInvalid          = "clip_audio_invalid"
	CodePlanUnsupportedProvider   = "plan_unsupported_provider"
	CodeProfileChanged            = "profile_changed"
)

// Phases of a planning job (PhaseComplete).
const (
	PhaseOutline = "outline"
	PhaseDetails = "details"
	PhaseCheck   = "check"
)

const (
	lengthTolerance          = 0.15 // share of target_seconds the total length may differ by
	minBlendSeconds          = 0.5  // shortest blend
	blendMargin              = 0.5  // part of each clip a blend must leave unblended
	narrationStartSeconds    = 0.3  // a narration starts this far into its scene (spec §10)
	narrationMaxSpillSeconds = 1.0  // and may run this far into the next scene
	epsilon                  = 1e-9 // absorbs float rounding in second comparisons
	maxQuotedRunes           = 40   // plan values quoted in messages are cut to this length
)

// Validate checks p against env (spec §8) and returns every issue: plan-level issues first, then each scene in
// order. It never changes p and returns an empty, non-nil list for a valid plan.
func Validate(p Plan, env Env) []Issue {
	v := &validator{plan: p, env: env, issues: []Issue{}}
	v.checkPlan()
	for i := range p.Scenes {
		v.checkScene(i)
	}
	return v.issues
}

// HasBlocking reports whether any issue blocks.
func HasBlocking(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}

// PhaseComplete reports whether a planning phase has done its work:
//   - outline: at least one scene, and every scene has a title, generate_seconds, use_seconds and a transition kind;
//   - details: outline, and every scene has a video prompt and, for a generated keyframe, an image prompt (narration
//     stays optional);
//   - check: no blocking issue.
//
// Any other phase is not complete.
func PhaseComplete(p Plan, phase string, env Env) bool {
	switch phase {
	case PhaseOutline:
		return outlineComplete(p)
	case PhaseDetails:
		if !outlineComplete(p) {
			return false
		}
		for _, s := range p.Scenes {
			if strings.TrimSpace(s.VideoPrompt) == "" {
				return false
			}
			if s.Keyframe.Source == KeyframeGenerated && strings.TrimSpace(s.Keyframe.ImagePrompt) == "" {
				return false
			}
		}
		return true
	case PhaseCheck:
		return !HasBlocking(Validate(p, env))
	}
	return false
}

// TotalSeconds is the length the scenes fill on the timeline: the sum of use_seconds minus every blend overlap.
func TotalSeconds(p Plan) float64 {
	total := 0.0
	for i, s := range p.Scenes {
		total += s.UseSeconds
		if i > 0 && s.TransitionIn.Kind == TransitionBlend {
			total -= s.TransitionIn.Seconds
		}
	}
	return total
}

func outlineComplete(p Plan) bool {
	if len(p.Scenes) == 0 {
		return false
	}
	for _, s := range p.Scenes {
		if strings.TrimSpace(s.Title) == "" || s.GenerateSeconds <= 0 || s.UseSeconds <= 0 || strings.TrimSpace(s.TransitionIn.Kind) == "" {
			return false
		}
	}
	return true
}

type validator struct {
	plan   Plan
	env    Env
	issues []Issue
}

func (v *validator) add(code, sceneID, field, message string) {
	v.issues = append(v.issues, Issue{Code: code, SceneID: sceneID, Field: field, Message: message, Blocking: code != CodeProfileChanged})
}

func (v *validator) checkPlan() {
	p, env := v.plan, v.env
	if !env.Profile.PlanSupported {
		reason := env.Profile.Reason
		if reason == "" {
			reason = ReasonProviderUnsupported
		}
		v.add(CodePlanUnsupportedProvider, "", "profile", "The configured video provider cannot be used for plan mode ("+reason+").")
	}
	if !sameRef(p.Profile, env.Profile) {
		v.add(CodeProfileChanged, "", "profile", fmt.Sprintf("The plan was made for %s; the server now uses %s.",
			describeRef(p.Profile.Provider, p.Profile.Model, p.Profile.Resolution), describeRef(env.Profile.Provider, env.Profile.Model, env.Profile.Resolution)))
	}
	targetValid := p.TargetSeconds >= MinTargetSeconds && p.TargetSeconds <= MaxTotalSeconds
	if !targetValid {
		v.add(CodeTargetSecondsInvalid, "", "target_seconds", fmt.Sprintf("target_seconds must be %d to %d.", MinTargetSeconds, MaxTotalSeconds))
	}
	if n := utf8.RuneCountInString(p.Idea); n > MaxIdeaChars {
		v.add(CodeIdeaTooLong, "", "idea", fmt.Sprintf("The idea has %d characters; at most %d are allowed.", n, MaxIdeaChars))
	}
	if n := utf8.RuneCountInString(p.Style); n > MaxStyleChars {
		v.add(CodeStyleTooLong, "", "style", fmt.Sprintf("The style has %d characters; at most %d are allowed.", n, MaxStyleChars))
	}
	v.checkCharacters()
	switch {
	case len(p.Scenes) == 0:
		v.add(CodeNoScenes, "", "scenes", "The plan has no scenes.")
	case len(p.Scenes) > MaxScenes:
		v.add(CodeTooManyScenes, "", "scenes", fmt.Sprintf("The plan has %d scenes; at most %d are allowed.", len(p.Scenes), MaxScenes))
	}
	if len(p.Scenes) > 0 {
		total := TotalSeconds(p)
		if total > MaxTotalSeconds+epsilon {
			v.add(CodeTotalLengthTooLong, "", "scenes", fmt.Sprintf("The scenes run %.1f s; a plan holds at most %d s.", total, MaxTotalSeconds))
		}
		target := float64(p.TargetSeconds)
		if targetValid && math.Abs(total-target) > lengthTolerance*target+epsilon {
			v.add(CodeTotalLengthMismatch, "", "target_seconds", fmt.Sprintf("The scenes run %.1f s (use_seconds minus blends); a target of %d s allows %.1f to %.1f s.",
				total, p.TargetSeconds, target*(1-lengthTolerance), target*(1+lengthTolerance)))
		}
	}
	if prompt := strings.TrimSpace(p.Audio.Music.Prompt); prompt != "" {
		if !env.Music {
			v.add(CodeMusicUnavailable, "", "audio.music.prompt", "Music generation is not configured; leave the music prompt empty.")
		}
		if n := utf8.RuneCountInString(prompt); n > MaxStyleChars {
			v.add(CodePromptTooLong, "", "audio.music.prompt", fmt.Sprintf("The music prompt has %d characters; at most %d are allowed.", n, MaxStyleChars))
		}
	}
	switch p.Audio.ClipAudio {
	case ClipAudioKeep, ClipAudioDuck, ClipAudioMute:
	default:
		v.add(CodeClipAudioInvalid, "", "audio.clip_audio", fmt.Sprintf("clip_audio %s is not keep, duck or mute.", quote(p.Audio.ClipAudio)))
	}
}

func (v *validator) checkCharacters() {
	characters := v.plan.Characters
	if len(characters) > MaxCharacters {
		v.add(CodeTooManyCharacters, "", "characters", fmt.Sprintf("The plan has %d characters; at most %d are allowed.", len(characters), MaxCharacters))
	}
	seen := make(map[string]bool, len(characters))
	for i, c := range characters {
		field := fmt.Sprintf("characters[%d]", i)
		name := strings.TrimSpace(c.Name)
		key := strings.ToLower(name)
		switch {
		case name == "":
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("Character %d has no name.", i+1))
		case utf8.RuneCountInString(name) > MaxTitleChars:
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("Character %d has a name longer than %d characters.", i+1, MaxTitleChars))
		case seen[key]:
			v.add(CodeCharacterInvalid, "", field+".name", fmt.Sprintf("The character name %s is used twice.", quote(name)))
		}
		if name != "" {
			seen[key] = true
		}
		if utf8.RuneCountInString(c.Description) > MaxCharacterDescriptionChars {
			v.add(CodeCharacterInvalid, "", field+".description", fmt.Sprintf("Character %d has a description longer than %d characters.", i+1, MaxCharacterDescriptionChars))
		}
		if id := strings.TrimSpace(c.ReferenceAssetID); id != "" && !v.env.ImageAssetIDs[id] {
			v.add(CodeCharacterReferenceInvalid, "", field+".reference_asset_id", fmt.Sprintf("Character %d refers to %s, which is not an image of this project.", i+1, quote(id)))
		}
	}
}

func (v *validator) checkScene(i int) {
	s := v.plan.Scenes[i]
	path := fmt.Sprintf("scenes[%d]", i)
	label := "Scene " + s.ID
	if s.ID == "" {
		label = fmt.Sprintf("Scene %d", i+1)
	}
	maxPrompt := v.env.Profile.MaxPromptChars
	if title := strings.TrimSpace(s.Title); title == "" {
		v.add(CodeTitleMissing, s.ID, path+".title", label+" has no title.")
	} else if n := utf8.RuneCountInString(title); n > MaxTitleChars {
		v.add(CodeTitleTooLong, s.ID, path+".title", fmt.Sprintf("%s: the title has %d characters; at most %d are allowed.", label, n, MaxTitleChars))
	}
	if prompt := strings.TrimSpace(s.VideoPrompt); prompt == "" {
		v.add(CodePromptMissing, s.ID, path+".video_prompt", label+" has no video prompt.")
	} else if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
		v.add(CodePromptTooLong, s.ID, path+".video_prompt", fmt.Sprintf("%s: the video prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(s.NegativePrompt)); maxPrompt > 0 && n > maxPrompt {
		v.add(CodeNegativePromptTooLong, s.ID, path+".negative_prompt", fmt.Sprintf("%s: the negative prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
	}
	if !slices.Contains(v.env.Profile.DurationsSeconds, s.GenerateSeconds) {
		v.add(CodeDurationUnsupported, s.ID, path+".generate_seconds", fmt.Sprintf("%s: generate_seconds %d is not offered by the provider (allowed: %s).", label, s.GenerateSeconds, joinInts(v.env.Profile.DurationsSeconds)))
	}
	if s.UseSeconds <= 0 || s.UseSeconds > float64(s.GenerateSeconds)+epsilon {
		v.add(CodeUseSecondsInvalid, s.ID, path+".use_seconds", fmt.Sprintf("%s: use_seconds %.2f must be above 0 and at most generate_seconds (%d).", label, s.UseSeconds, s.GenerateSeconds))
	}
	v.checkTransition(i, path, label)
	v.checkKeyframe(i, path, label)
	v.checkNarration(i, path, label)
}

func (v *validator) checkTransition(i int, path, label string) {
	scenes := v.plan.Scenes
	s := scenes[i]
	kind := s.TransitionIn.Kind
	field := path + ".transition_in.kind"
	if i == 0 {
		if kind != TransitionCut {
			v.add(CodeFirstSceneTransition, s.ID, field, label+" opens the clip; its transition_in must be cut.")
		}
		return
	}
	profile := v.env.Profile
	switch kind {
	case TransitionCut:
	case TransitionContinue:
		if !profile.FirstFrame {
			v.add(CodeTransitionUnsupported, s.ID, field, label+": continue needs a provider that takes a first frame.")
		}
		if n := chainLength(scenes, i); n > MaxChainScenes {
			v.add(CodeChainTooLong, s.ID, field, fmt.Sprintf("%s is scene %d of a seamless chain; a chain holds at most %d scenes.", label, n, MaxChainScenes))
		}
	case TransitionBridge:
		if !profile.FirstFrame || !profile.LastFrame {
			v.add(CodeTransitionUnsupported, s.ID, field, label+": bridge needs a provider that takes a first and a last frame.")
		}
	case TransitionBlend:
		v.checkBlend(i, path, label)
	default:
		v.add(CodeTransitionUnknown, s.ID, field, fmt.Sprintf("%s: transition_in.kind %s is not cut, continue, bridge or blend.", label, quote(kind)))
	}
}

// chainLength is the number of scenes in the seamless chain that ends at scene i.
func chainLength(scenes []Scene, i int) int {
	n := 1
	for j := i; j > 0 && scenes[j].TransitionIn.Kind == TransitionContinue; j-- {
		n++
	}
	return n
}

func (v *validator) checkBlend(i int, path, label string) {
	s := v.plan.Scenes[i]
	blendType := s.TransitionIn.Type
	typeField := path + ".transition_in.type"
	switch {
	case !slices.Contains(v.env.KnownTransitions, blendType):
		v.add(CodeBlendTypeUnknown, s.ID, typeField, fmt.Sprintf("%s: blend type %s is not a Video Studio transition.", label, quote(blendType)))
	case slices.Contains(v.env.UnavailableTransitions, blendType):
		v.add(CodeBlendTypeUnavailable, s.ID, typeField, fmt.Sprintf("%s: blend type %s is not available on this server.", label, quote(blendType)))
	}
	limit := math.Min(v.plan.Scenes[i-1].UseSeconds, s.UseSeconds) - blendMargin
	if seconds := s.TransitionIn.Seconds; seconds < minBlendSeconds-epsilon || seconds > limit+epsilon {
		v.add(CodeBlendOverlapInvalid, s.ID, path+".transition_in.seconds", fmt.Sprintf("%s: the blend lasts %.2f s; it must last %.1f to %.2f s (the shorter of both scenes minus %.1f s).",
			label, seconds, minBlendSeconds, math.Max(limit, 0), blendMargin))
	}
}

func (v *validator) checkKeyframe(i int, path, label string) {
	s := v.plan.Scenes[i]
	field := path + ".keyframe.source"
	maxPrompt := v.env.Profile.MaxPromptChars
	switch s.Keyframe.Source {
	case KeyframeNone:
	case KeyframeGenerated:
		if !v.env.ImageGeneration {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": image generation is not configured; use a library image or none.")
		}
		if prompt := strings.TrimSpace(s.Keyframe.ImagePrompt); prompt == "" {
			v.add(CodeKeyframePromptMissing, s.ID, path+".keyframe.image_prompt", label+": a generated keyframe needs an image_prompt.")
		} else if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
			v.add(CodePromptTooLong, s.ID, path+".keyframe.image_prompt", fmt.Sprintf("%s: the image prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
		}
	case KeyframeLibrary:
		if !v.env.ImageAssetIDs[strings.TrimSpace(s.Keyframe.AssetID)] {
			v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": keyframe.asset_id must be an image of this project.")
		}
	case KeyframeChain:
		if i == 0 || s.TransitionIn.Kind != TransitionContinue {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": keyframe source chain belongs to a continue transition.")
		}
	case KeyframeBridge:
		if i == 0 || s.TransitionIn.Kind != TransitionBridge {
			v.add(CodeKeyframeSourceInvalid, s.ID, field, label+": keyframe source bridge belongs to a bridge transition.")
		} else {
			v.checkBridgeKeyframe(s, path, label)
		}
	default:
		v.add(CodeKeyframeSourceInvalid, s.ID, field, fmt.Sprintf("%s: keyframe.source %s is not generated, library, chain, bridge or none.", label, quote(s.Keyframe.Source)))
	}
}

// checkBridgeKeyframe checks the boundary keyframe K of a bridge scene (spec §4.2): an image of the project
// (keyframe.asset_id), or a generated image (keyframe.image_prompt) when image generation is configured.
func (v *validator) checkBridgeKeyframe(s Scene, path, label string) {
	maxPrompt := v.env.Profile.MaxPromptChars
	assetID, prompt := strings.TrimSpace(s.Keyframe.AssetID), strings.TrimSpace(s.Keyframe.ImagePrompt)
	switch {
	case assetID != "":
		if !v.env.ImageAssetIDs[assetID] {
			v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": the bridge keyframe.asset_id must be an image of this project.")
		}
	case !v.env.ImageGeneration:
		v.add(CodeKeyframeAssetInvalid, s.ID, path+".keyframe.asset_id", label+": image generation is not configured, so a bridge needs an image of this project as its keyframe; otherwise use another transition.")
	case prompt == "":
		v.add(CodeKeyframePromptMissing, s.ID, path+".keyframe.image_prompt", label+": a bridge needs an image_prompt for the frame where the scene before ends and this scene starts.")
	default:
		if n := utf8.RuneCountInString(prompt); maxPrompt > 0 && n > maxPrompt {
			v.add(CodePromptTooLong, s.ID, path+".keyframe.image_prompt", fmt.Sprintf("%s: the image prompt has %d characters; at most %d are allowed.", label, n, maxPrompt))
		}
	}
}

func (v *validator) checkNarration(i int, path, label string) {
	s := v.plan.Scenes[i]
	text := strings.TrimSpace(s.Narration.Text)
	if text == "" {
		return
	}
	field := path + ".narration.text"
	if !v.env.TTS {
		v.add(CodeNarrationUnavailable, s.ID, field, label+": text-to-speech is not ready; leave the narration empty.")
	}
	if n := utf8.RuneCountInString(text); n > MaxNarrationChars {
		v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration has %d characters; at most %d are allowed.", label, n, MaxNarrationChars))
		return
	}
	if measured := s.Narration.MeasuredSeconds; measured > 0 {
		// After the preview step the measured voice length replaces the word estimate (spec §8).
		if narrationStartSeconds+measured > s.UseSeconds+narrationMaxSpillSeconds+epsilon {
			v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration lasts %.1f s and runs more than %.0f s into the next scene; shorten it.", label, measured, narrationMaxSpillSeconds))
		}
		return
	}
	if words, budget := CountWords(text), WordBudget(v.plan.Language, s.UseSeconds); words > budget {
		v.add(CodeNarrationTooLong, s.ID, field, fmt.Sprintf("%s: the narration has %d words; %.1f s fit at most %d.", label, words, s.UseSeconds, budget))
	}
}

func sameRef(ref ProfileRef, profile Profile) bool {
	return strings.EqualFold(strings.TrimSpace(ref.Provider), profile.Provider) &&
		strings.EqualFold(strings.TrimSpace(ref.Model), profile.Model) &&
		strings.EqualFold(strings.TrimSpace(ref.Resolution), profile.Resolution)
}

func describeRef(provider, model, resolution string) string {
	var parts []string
	for _, part := range []string{provider, model, resolution} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "no provider"
	}
	return strings.Join(parts, " / ")
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ", ")
}

// quote quotes a plan value for a message, cut to maxQuotedRunes characters.
func quote(value string) string {
	if utf8.RuneCountInString(value) > maxQuotedRunes {
		value = string([]rune(value)[:maxQuotedRunes]) + "…"
	}
	return strconv.Quote(value)
}
```

- [ ] **Step 4: Run it.** `gofmt -w internal/videostudio/storyplan && go test ./internal/videostudio/storyplan/... -count=1`. Expected: `ok` (all subtests of `TestValidateReportsEveryRule` PASS, no `no test case for` error).

- [ ] **Step 5: Commit.**
```
git add internal/videostudio/storyplan/validate.go internal/videostudio/storyplan/validate_test.go
git commit -m "feat(video-studio): storyplan validator with issue codes and phase checks" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A4: tools.MiniMaxVideoAPIModel

**Files:**
- Modify: `internal/tools/video_generation.go` (add `MiniMaxVideoAPIModel` after `miniMaxVideoModelForAPI`, ~line 584)
- Test: `internal/tools/video_generation_test.go` (new test after `TestMiniMaxVideoModelForAPI`)

- [ ] **Step 1: Write the failing test.** In `internal/tools/video_generation_test.go`:

**Old:**
```go
			gotAPI, gotDisplay := miniMaxVideoModelForAPI(tt.input)
			if gotAPI != tt.wantAPI || gotDisplay != tt.wantDisplay {
				t.Fatalf("miniMaxVideoModelForAPI(%q) = (%q, %q), want (%q, %q)", tt.input, gotAPI, gotDisplay, tt.wantAPI, tt.wantDisplay)
			}
		})
	}
}
```
**New:**
```go
			gotAPI, gotDisplay := miniMaxVideoModelForAPI(tt.input)
			if gotAPI != tt.wantAPI || gotDisplay != tt.wantDisplay {
				t.Fatalf("miniMaxVideoModelForAPI(%q) = (%q, %q), want (%q, %q)", tt.input, gotAPI, gotDisplay, tt.wantAPI, tt.wantDisplay)
			}
		})
	}
}

func TestMiniMaxVideoAPIModel(t *testing.T) {
	for input, want := range map[string]string{
		"":                        "MiniMax-Hailuo-2.3",
		"hailuo-02":               "MiniMax-Hailuo-02",
		"Hailuo-2.3-768P":         "MiniMax-Hailuo-2.3",
		"MiniMax-M2.7":            "MiniMax-Hailuo-2.3",
		"MiniMax-Hailuo-2.3-Fast": "MiniMax-Hailuo-2.3-Fast",
	} {
		if got := MiniMaxVideoAPIModel(input); got != want {
			t.Errorf("MiniMaxVideoAPIModel(%q) = %q, want %q", input, got, want)
		}
	}
}
```

- [ ] **Step 2: Run it.** `go test ./internal/tools -run '^TestMiniMaxVideoAPIModel$' -count=1`. Expected: FAIL, a build failure with `undefined: MiniMaxVideoAPIModel`.

- [ ] **Step 3: Implement.** In `internal/tools/video_generation.go`:

**Old:**
```go
	if strings.Contains(lower, "hailuo") {
		return trimmed, trimmed
	}

	return defaultMiniMaxVideoModel, defaultMiniMaxVideoModel
}
```
**New:**
```go
	if strings.Contains(lower, "hailuo") {
		return trimmed, trimmed
	}

	return defaultMiniMaxVideoModel, defaultMiniMaxVideoModel
}

// MiniMaxVideoAPIModel returns the model id a generation request sends to MiniMax's video API for a configured model
// name: aliases and the legacy preset map to the official ids, and chat or Google model names fall back to
// MiniMax-Hailuo-2.3 (miniMaxVideoModelForAPI).
func MiniMaxVideoAPIModel(model string) string {
	apiModel, _ := miniMaxVideoModelForAPI(model)
	return apiModel
}
```

- [ ] **Step 4: Run it.** `go test ./internal/tools -run '(?i)(TestMiniMaxVideoAPIModel|TestMiniMaxVideoModelForAPI)' -count=1`. Expected: `ok`.

- [ ] **Step 5: Commit.**
```
git add internal/tools/video_generation.go internal/tools/video_generation_test.go
git commit -m "feat(video-studio): export the MiniMax video API model mapping" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A5: server capability profile, duration and image-mode wrappers, status JSON

**Files:**
- Create: `internal/server/video_studio_generation_profile.go`:
  - `videoStudioGenerationProfile`, `videoStudioGenerationConfigured`, `videoStudioProviderProfile`
  - `videoStudioDurationOptions` and `videoStudioImageModes`, moved here with the same signatures
- Modify: `internal/server/video_studio_handlers.go`:
  - `handleVideoStudioStatus` (~lines 179 and 220-228): `configured` and the `"profile"` key
  - delete the old `videoStudioDurationOptions` and `videoStudioImageModes` (lines 261-299)
- Test: `internal/server/video_studio_generation_profile_test.go`. The existing `TestVideoStudioStatusResponseShapeAndProviderCapabilities` (`video_studio_handlers_test.go:21`) pins today's durations and image modes in the status JSON and must stay green.

- [ ] **Step 1: Impact check.** Run upstream impact for `videoStudioDurationOptions`, `videoStudioImageModes` and `handleVideoStudioStatus`. Expected callers:
  - `videoStudioDurationOptions`: `handleVideoStudioStatus` (`video_studio_handlers.go:225`) and `handleVideoStudioJobCreate` (`:1033`).
  - `videoStudioImageModes`: `handleVideoStudioStatus` (`:226`).
  - `handleVideoStudioStatus`: the `handleVideoStudio` route.

- [ ] **Step 2: Write the failing test.** Create `internal/server/video_studio_generation_profile_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)

func videoStudioProfileConfig(provider, model, defaultResolution string) *config.Config {
	cfg := &config.Config{}
	cfg.VideoGeneration.Enabled = true
	cfg.VideoGeneration.APIKey = "fixture-key"
	cfg.VideoGeneration.ProviderType = provider
	cfg.VideoGeneration.Model = model
	cfg.VideoGeneration.DefaultResolution = defaultResolution
	return cfg
}

// The spec §4.1 table: MiniMax Hailuo-2.3 and -02, Google Veo, Agnes and an unknown provider.
func TestVideoStudioGenerationProfileTable(t *testing.T) {
	minimax := func(model, resolution string, durations []int, lastFrame bool) storyplan.Profile {
		return storyplan.Profile{PlanSupported: true, Provider: "minimax", Model: model, Resolution: resolution, DurationsSeconds: durations,
			AspectRatios: []string{}, AspectFromFirstFrame: true, FirstFrame: true, LastFrame: lastFrame, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2}
	}
	google := func(model, resolution string, durations []int, nativeAudio bool) storyplan.Profile {
		return storyplan.Profile{PlanSupported: true, Provider: "google", Model: model, Resolution: resolution, DurationsSeconds: durations,
			AspectRatios: []string{"16:9", "9:16"}, FirstFrame: true, NativeAudio: nativeAudio, MaxPromptChars: 2000, CostPerClipEstimate: 0.50, Parallel: 2}
	}
	generationOff := minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)
	generationOff.PlanSupported, generationOff.Reason = false, storyplan.ReasonGenerationUnavailable
	for _, tc := range []struct {
		name                                    string
		provider, model, defaultRes, resolution string
		enabled                                 bool
		want                                    storyplan.Profile
	}{
		{"hailuo 2.3", "minimax", "MiniMax-Hailuo-2.3", "", "768P", true, minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)},
		{"hailuo 2.3 fast at 1080P", "minimax", "MiniMax-Hailuo-2.3-Fast", "", "1080P", true, minimax("MiniMax-Hailuo-2.3-Fast", "1080P", []int{6}, false)},
		{"hailuo 02 alias at 768P", "minimax", "hailuo-02", "", "768P", true, minimax("MiniMax-Hailuo-02", "768P", []int{6, 10}, true)},
		{"hailuo 02 at 1080P", "minimax", "MiniMax-Hailuo-02", "", "1080P", true, minimax("MiniMax-Hailuo-02", "1080P", []int{6}, true)},
		{"hailuo 02 at 512P has no last frame", "minimax", "MiniMax-Hailuo-02", "", "512P", true, minimax("MiniMax-Hailuo-02", "512P", []int{6, 10}, false)},
		{"minimax defaults", "minimax", "", "", "", true, minimax("MiniMax-Hailuo-2.3", "768P", []int{6, 10}, false)},
		{"default resolution from the config", "minimax", "MiniMax-Hailuo-02", "1080P", "", true, minimax("MiniMax-Hailuo-02", "1080P", []int{6}, true)},
		{"non-hailuo minimax model", "minimax", "S2V-01", "", "768P", true, minimax("S2V-01", "768P", []int{6}, false)},
		{"veo 3.1", "google_veo", "veo-3.1-generate-preview", "", "720p", true, google("veo-3.1-generate-preview", "720p", []int{4, 6, 8}, true)},
		{"veo default model at 1080p", "google", "", "", "1080p", true, google("veo-3.1-generate-preview", "1080p", []int{8}, true)},
		{"veo 2 has no audio", "google", "veo-2.0-generate-001", "", "720p", true, google("veo-2.0-generate-001", "720p", []int{4, 6, 8}, false)},
		{"agnes", "agnes", "agnes-3.0-flash", "", "720P", true, storyplan.Profile{Reason: storyplan.ReasonProviderImagesPublicURLOnly, Provider: "agnes",
			Model: "agnes-video-2.5-flash", Resolution: "720P", DurationsSeconds: []int{4, 5, 6, 7, 8, 9, 10, 11, 12}, AspectRatios: []string{}}},
		{"unknown provider", "acme", "acme-video", "", "720p", true, storyplan.Profile{Reason: storyplan.ReasonProviderUnsupported, Provider: "acme",
			Model: "acme-video", Resolution: "720p", DurationsSeconds: []int{}, AspectRatios: []string{}}},
		{"generation off keeps the capabilities", "minimax", "", "", "", false, generationOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := videoStudioProfileConfig(tc.provider, tc.model, tc.defaultRes)
			cfg.VideoGeneration.Enabled = tc.enabled
			if got := videoStudioGenerationProfile(cfg, tc.resolution); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("profile = %+v\nwant      %+v", got, tc.want)
			}
		})
	}
}

// The wrappers return exactly what manual generation offered before the profile existed.
func TestVideoStudioGenerationProfileKeepsManualDurationsAndImageModes(t *testing.T) {
	for _, tc := range []struct {
		provider, model, defaultRes, resolution string
		references                              bool
		want                                    []int
	}{
		{"google", "", "", "720p", false, []int{4, 6, 8}},
		{"google", "", "", "720p", true, []int{8}},
		{"google", "", "", "1080p", false, []int{8}},
		{"google_veo", "", "1080P", "", false, []int{8}},
		{"minimax", "hailuo-02", "", "", false, []int{6, 10}},
		{"minimax", "hailuo-02", "", "", true, []int{6, 10}},
		{"minimax", "hailuo-02", "", "1080P", false, []int{6}},
		{"minimax", "S2V-01", "", "", false, []int{6}},
		{"minimax", "", "", "", false, []int{6, 10}},
		{"agnes", "", "", "", false, []int{4, 5, 6, 7, 8, 9, 10, 11, 12}},
		{"", "", "", "", false, []int{}},
		{"acme", "", "", "", false, []int{}},
	} {
		cfg := videoStudioProfileConfig(tc.provider, tc.model, tc.defaultRes)
		if got := videoStudioDurationOptions(cfg, tc.resolution, tc.references); got == nil || !slices.Equal(got, tc.want) {
			t.Errorf("videoStudioDurationOptions(%q %q %q %q refs=%v) = %#v, want %v", tc.provider, tc.model, tc.defaultRes, tc.resolution, tc.references, got, tc.want)
		}
	}
	for provider, want := range map[string][]string{
		"minimax": {"first_frame"}, "google": {"first_frame"}, "google_veo": {"first_frame"}, "GOOGLE_VEO": {"first_frame"},
		"agnes": {}, "": {}, "acme": {},
	} {
		if got := videoStudioImageModes(provider); got == nil || !slices.Equal(got, want) {
			t.Errorf("videoStudioImageModes(%q) = %#v, want %v", provider, got, want)
		}
	}
}

func TestVideoStudioGenerationProfileInStatus(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	s.Cfg.VideoGeneration.ResolvedModel = ""
	s.Cfg.VideoGeneration.DefaultResolution = "768P"
	response := videoStudioAPIRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status struct {
		Generation struct {
			Profile *storyplan.Profile `json:"profile"`
		} `json:"generation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	want := storyplan.Profile{PlanSupported: true, Provider: "minimax", Model: "MiniMax-Hailuo-02", Resolution: "768P", DurationsSeconds: []int{6, 10},
		AspectRatios: []string{}, AspectFromFirstFrame: true, FirstFrame: true, LastFrame: true, MaxPromptChars: 2000, CostPerClipEstimate: 0.20, Parallel: 2}
	if status.Generation.Profile == nil || !reflect.DeepEqual(*status.Generation.Profile, want) {
		t.Fatalf("generation.profile = %+v, want %+v", status.Generation.Profile, want)
	}
}
```

- [ ] **Step 3: Run it.** `go test ./internal/server -run '(?i)VideoStudioGenerationProfile' -count=1`. Expected: FAIL, a build failure with `undefined: videoStudioGenerationProfile`.

- [ ] **Step 4: Implement.** Create `internal/server/video_studio_generation_profile.go`:

```go
package server

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/videostudio/storyplan"
)

// Values of the capability profile (spec 2026-11-23-video-studio-plan-mode-design.md §4.1). The clip costs are the
// estimates tools.GenerateVideoResult books when a provider reports none; the Google default model is
// tools.defaultGoogleVideoModel.
const (
	videoStudioDefaultMiniMaxResolution = "768P"
	videoStudioDefaultGoogleVideoModel  = "veo-3.1-generate-preview"
	videoStudioMiniMaxClipCostEstimate  = 0.20
	videoStudioGoogleClipCostEstimate   = 0.50
	videoStudioPlanMaxPromptChars       = 2000
	videoStudioPlanParallel             = 2
)

// videoStudioGenerationProfile is the capability profile of the configured video generation API (spec
// 2026-11-23-video-studio-plan-mode-design.md §4.1). It is the single source for the status JSON
// (generation.profile), the manual generate admission, the plan validator and the planning agent. resolution ""
// means cfg.VideoGeneration.DefaultResolution (MiniMax falls back to 768P). When generation is off or lacks a key or
// a provider type, the profile keeps the provider's capabilities, so manual durations stay visible, and reports
// plan_supported false with reason generation_unavailable.
func videoStudioGenerationProfile(cfg *config.Config, resolution string) storyplan.Profile {
	model := cfg.VideoGeneration.ResolvedModel
	if model == "" {
		model = cfg.VideoGeneration.Model
	}
	profile := videoStudioProviderProfile(cfg.VideoGeneration.ProviderType, model, firstNonEmptyVideoStudio(resolution, cfg.VideoGeneration.DefaultResolution))
	if !videoStudioGenerationConfigured(cfg) {
		profile.PlanSupported = false
		profile.Reason = storyplan.ReasonGenerationUnavailable
	}
	return profile
}

// videoStudioGenerationConfigured reports video generation switched on with an API key and a provider type.
func videoStudioGenerationConfigured(cfg *config.Config) bool {
	return cfg.VideoGeneration.Enabled && strings.TrimSpace(cfg.VideoGeneration.APIKey) != "" && cfg.VideoGeneration.ProviderType != ""
}

// videoStudioProviderProfile derives the profile from the provider type, the configured model (ResolvedModel, else
// Model) and the resolution, without looking at whether generation is configured. The durations are the ones manual
// generation has always offered: MiniMax 6 and 10 s (6 s at 1080P, or when the configured model name is not a Hailuo
// model), Google Veo 4, 6 and 8 s (8 s at 1080P), Agnes 4 to 12 s.
func videoStudioProviderProfile(providerType, model, resolution string) storyplan.Profile {
	provider := strings.ToLower(providerType)
	if provider == "google_veo" {
		provider = "google"
	}
	resolution = strings.TrimSpace(resolution)
	is1080 := strings.Contains(strings.ToLower(resolution), "1080")
	profile := storyplan.Profile{Provider: provider, Model: strings.TrimSpace(model), Resolution: resolution, DurationsSeconds: []int{}, AspectRatios: []string{}}
	switch provider {
	case "minimax":
		profile.PlanSupported = true
		profile.Model = tools.MiniMaxVideoAPIModel(model)
		if profile.Resolution == "" {
			profile.Resolution = videoStudioDefaultMiniMaxResolution
		}
		lowerModel := strings.ToLower(model)
		if is1080 || (!strings.Contains(lowerModel, "hailuo") && lowerModel != "") {
			profile.DurationsSeconds = []int{6}
		} else {
			profile.DurationsSeconds = []int{6, 10}
		}
		profile.FirstFrame = true
		// MiniMax documents first and last frame only for MiniMax-Hailuo-02 at 768P and 1080P (checked 2026-11-23).
		profile.LastFrame = profile.Model == "MiniMax-Hailuo-02" && (strings.EqualFold(profile.Resolution, "768P") || strings.EqualFold(profile.Resolution, "1080P"))
		profile.AspectFromFirstFrame = true
		profile.CostPerClipEstimate = videoStudioMiniMaxClipCostEstimate
	case "google":
		profile.PlanSupported = true
		profile.Model = firstNonEmptyVideoStudio(model, videoStudioDefaultGoogleVideoModel)
		if is1080 {
			profile.DurationsSeconds = []int{8}
		} else {
			profile.DurationsSeconds = []int{4, 6, 8}
		}
		profile.AspectRatios = []string{"16:9", "9:16"}
		// The adapter sends only instances[0].image; last frame stays off until a live check passes (spec §4.1).
		profile.FirstFrame = true
		profile.NativeAudio = strings.Contains(strings.ToLower(profile.Model), "veo-3")
		profile.CostPerClipEstimate = videoStudioGoogleClipCostEstimate
	case "agnes":
		// Agnes takes start and end images only as public URLs; manual text-only generation keeps its durations.
		profile.Reason = storyplan.ReasonProviderImagesPublicURLOnly
		profile.Model = tools.AgnesVideoModel(model)
		for seconds := tools.AgnesVideoMinSeconds; seconds <= tools.AgnesVideoMaxSeconds; seconds++ {
			profile.DurationsSeconds = append(profile.DurationsSeconds, seconds)
		}
	default:
		profile.Reason = storyplan.ReasonProviderUnsupported
	}
	if profile.PlanSupported {
		profile.MaxPromptChars = videoStudioPlanMaxPromptChars
		profile.Parallel = videoStudioPlanParallel
	}
	return profile
}

// videoStudioDurationOptions are the durations manual generation offers for a resolution ("" = default). Never nil.
// Google with reference images keeps the 8-second rule of manual generation.
func videoStudioDurationOptions(cfg *config.Config, resolution string, hasReferences bool) []int {
	profile := videoStudioGenerationProfile(cfg, resolution)
	if hasReferences && profile.Provider == "google" {
		return []int{8}
	}
	return profile.DurationsSeconds
}

// videoStudioImageModes are the image inputs manual generation accepts for a provider type: one first frame where the
// profile has first_frame (MiniMax, Google Veo), none otherwise. Never nil.
func videoStudioImageModes(provider string) []string {
	if videoStudioProviderProfile(provider, "", "").FirstFrame {
		return []string{"first_frame"}
	}
	return []string{}
}
```

In `internal/server/video_studio_handlers.go`, make three edits:

**Old:**
```go
	configured := cfg.VideoGeneration.Enabled && strings.TrimSpace(cfg.VideoGeneration.APIKey) != "" && provider != ""
```
**New:**
```go
	configured := videoStudioGenerationConfigured(cfg)
```

**Old:**
```go
		"durations_seconds": videoStudioDurationOptions(cfg, "", false),
		"image_modes":       videoStudioImageModes(provider),
		"budget_blocked":    s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("video_generation"),
	}
```
**New:**
```go
		"durations_seconds": videoStudioDurationOptions(cfg, "", false),
		"image_modes":       videoStudioImageModes(provider),
		"budget_blocked":    s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("video_generation"),
		// The capability profile at the default resolution; plan mode and the storyboard read it.
		"profile": videoStudioGenerationProfile(cfg, ""),
	}
```

**Old** (delete both old bodies, keep `firstNonEmptyVideoStudio`):
```go
func videoStudioDurationOptions(cfg *config.Config, resolution string, hasReferences bool) []int {
	provider := strings.ToLower(cfg.VideoGeneration.ProviderType)
	resolution = strings.ToLower(firstNonEmptyVideoStudio(resolution, cfg.VideoGeneration.DefaultResolution))
	switch provider {
	case "google", "google_veo":
		if strings.Contains(resolution, "1080") || hasReferences {
			return []int{8}
		}
		return []int{4, 6, 8}
	case "minimax":
		model := strings.ToLower(cfg.VideoGeneration.ResolvedModel)
		if model == "" {
			model = strings.ToLower(cfg.VideoGeneration.Model)
		}
		if strings.Contains(resolution, "1080") || (!strings.Contains(model, "hailuo") && model != "") {
			return []int{6}
		}
		return []int{6, 10}
	case "agnes":
		values := make([]int, tools.AgnesVideoMaxSeconds-tools.AgnesVideoMinSeconds+1)
		for i := range values {
			values[i] = i + tools.AgnesVideoMinSeconds
		}
		return values
	default:
		return []int{}
	}
}

func videoStudioImageModes(provider string) []string {
	switch strings.ToLower(provider) {
	case "minimax":
		return []string{"first_frame"}
	case "google", "google_veo":
		return []string{"first_frame"}
	default:
		return []string{}
	}
}

func firstNonEmptyVideoStudio(values ...string) string {
```
**New:**
```go
func firstNonEmptyVideoStudio(values ...string) string {
```

(`tools` stays imported in `video_studio_handlers.go`: `tools.AgnesVideoModel` in the status handler and `tools.AgnesVideoMinSeconds` in the admission still use it.)

- [ ] **Step 5: Run it.** `gofmt -w internal/server/video_studio_generation_profile.go internal/server/video_studio_handlers.go internal/server/video_studio_generation_profile_test.go && go test ./internal/server -run '(?i)(VideoStudioGenerationProfile|VideoStudioStatusResponseShape)' -count=1`. Expected: `ok`.

- [ ] **Step 6: Commit.**
```
git add internal/server/video_studio_generation_profile.go internal/server/video_studio_generation_profile_test.go internal/server/video_studio_handlers.go
git commit -m "feat(video-studio): capability profile for video generation in the status JSON" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A6: generate admission reads the capability profile

**Files:**
- Modify: `internal/server/video_studio_handlers.go`, `handleVideoStudioJobCreate`: the `kind == "generate"` block, the image-mode and provider checks after the duration check (~lines 1037-1048)
- Test: `internal/server/video_studio_generation_admission_test.go` (new). This is a characterization test: it passes before and after the change.

- [ ] **Step 1: Impact check.** Run upstream impact for `handleVideoStudioJobCreate`. Expected: its only caller is the `handleVideoStudio` route (`projects/{id}/jobs`).

- [ ] **Step 2: Write the test (characterization: it must PASS on the current code).** Create `internal/server/video_studio_generation_admission_test.go`:

```go
package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The generate admission decides durations and image modes from the capability profile. These answers are the ones
// manual generation gave before the profile existed.
func TestVideoStudioGenerationProfileDrivesGenerateAdmission(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	adminToken, _, err := s.TokenManager.Create("video studio admin fixture", []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, _ := videoStudioAPICreate(t, s, writeToken)
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-only"
	s.Cfg.VideoGeneration.ResolvedModel = ""
	s.Cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	for i, tc := range []struct {
		name, provider, model, resolution, fields string
		status                                    int
		code, message                             string
	}{
		{"minimax refuses a last frame", "minimax", "MiniMax-Hailuo-02", "768P", `"duration_seconds":6,"last_frame_asset_id":"img"`, http.StatusBadRequest, "unsupported_image_mode", "first-frame"},
		{"minimax refuses references", "minimax", "MiniMax-Hailuo-2.3", "768P", `"duration_seconds":6,"reference_asset_ids":["img"]`, http.StatusBadRequest, "unsupported_image_mode", "first-frame"},
		{"minimax first frame passes the mode check", "minimax", "MiniMax-Hailuo-2.3", "768P", `"duration_seconds":6,"first_frame_asset_id":"img"`, http.StatusBadRequest, "invalid_image_asset", ""},
		{"veo references need 8 s", "google_veo", "", "720p", `"duration_seconds":6,"reference_asset_ids":["img"]`, http.StatusBadRequest, "unsupported_duration", ""},
		{"minimax has no 10 s at 1080P", "minimax", "MiniMax-Hailuo-2.3", "1080P", `"duration_seconds":10`, http.StatusBadRequest, "unsupported_duration", ""},
		{"agnes is text only", "agnes", "agnes-3.0-flash", "", `"duration_seconds":6,"first_frame_asset_id":"img"`, http.StatusBadRequest, "unsupported_image_mode", "text-only"},
		{"unknown provider has no durations", "acme", "", "", `"duration_seconds":6`, http.StatusBadRequest, "unsupported_duration", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.Cfg.VideoGeneration.ProviderType = tc.provider
			s.Cfg.VideoGeneration.Model = tc.model
			s.Cfg.VideoGeneration.DefaultResolution = tc.resolution
			body := `{"kind":"generate","prompt":"fixture",` + tc.fields + `}`
			response := videoStudioAPIRequest(s, adminToken, http.MethodPost, "/projects/"+project.ID+"/jobs", strings.NewReader(body), map[string]string{"Idempotency-Key": fmt.Sprintf("admission-%d", i)})
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) || !strings.Contains(response.Body.String(), tc.message) {
				t.Fatalf("%d %s, want %d %s %q", response.Code, response.Body.String(), tc.status, tc.code, tc.message)
			}
		})
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if jobs := manager.jobsForProject(project.ID); len(jobs) != 0 {
		t.Fatalf("refused generation requests created jobs: %+v", jobs)
	}
}
```

- [ ] **Step 3: Run it on the current code.** `go test ./internal/server -run '(?i)VideoStudioGenerationProfileDrivesGenerateAdmission' -count=1`. Expected: `ok` (PASS: it pins today's behaviour).

- [ ] **Step 4: Implement.** In `internal/server/video_studio_handlers.go`:

**Old:**
```go
		if provider == "agnes" && (request.FirstFrameAssetID != "" || request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "This provider accepts text-only generation in Video Studio.")
			return
		}
		if (provider == "google" || provider == "google_veo" || provider == "minimax") && (request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "Video Studio currently supports only one first-frame image for this provider.")
			return
		}
		if provider != "agnes" && provider != "google" && provider != "google_veo" && provider != "minimax" {
			writeVideoStudioError(w, http.StatusConflict, "generation_unavailable", "The configured provider has no supported Video Studio generation mode.")
			return
		}
```
**New:**
```go
		// The capability profile decides the image modes, the same profile plan mode validates against (spec
		// 2026-11-23 §4.1). Manual generation sends at most one first-frame image: last frames and references stay
		// refused even where the profile reports last_frame (MiniMax-Hailuo-02), and providers without first_frame
		// (Agnes) generate from text only.
		profile := videoStudioGenerationProfile(cfg, request.Resolution)
		withImages := request.FirstFrameAssetID != "" || request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0
		if !profile.FirstFrame && withImages {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "This provider accepts text-only generation in Video Studio.")
			return
		}
		if profile.FirstFrame && (request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "Video Studio currently supports only one first-frame image for this provider.")
			return
		}
		// A provider without durations has no generation mode; the duration check above already refuses it.
		if len(profile.DurationsSeconds) == 0 {
			writeVideoStudioError(w, http.StatusConflict, "generation_unavailable", "The configured provider has no supported Video Studio generation mode.")
			return
		}
```

- [ ] **Step 5: Run it.** `gofmt -w internal/server/video_studio_handlers.go && go test ./internal/server -run '(?i)VideoStudioGeneration' -count=1`. This includes `TestVideoStudioGenerationBudgetGatePreventsAdmission` and the A5 tests. Expected: `ok`.

- [ ] **Step 6: Commit.**
```
git add internal/server/video_studio_handlers.go internal/server/video_studio_generation_admission_test.go
git commit -m "refactor(video-studio): generate admission reads the capability profile" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A7: config `video_studio.plan`

**Files:**
- Modify `internal/config/video_studio.go`:
  - `VideoStudioConfig` gains `Plan`
  - new `VideoStudioPlanConfig`, its defaults and `normalizeVideoStudioPlan`
  - `NormalizeVideoStudioConfig` calls `normalizeVideoStudioPlan`
- Modify `internal/config/config.go`, `load` defaults (~line 599): `cfg.VideoStudio.Plan.Enabled = true`
- Modify `internal/config/provider_references.go`, `ProviderReferences` (~line 75): add the reference
- Modify `internal/config/decisions.go`, `validateDecisionsOnlyProviderReferences` (~line 159): add the reference
- Modify `config_template.yaml`: the `video_studio:` block (~line 2391) gains `plan:`
- Modify `internal/server/video_studio_service.go`, `videoStudioConfigRootsChanged` (~line 272): compare `VideoStudio` with `Plan` zeroed on both sides, so saving `video_studio.plan.*` does not cancel running Video Studio FFmpeg jobs (controller decision)
- Test (new): `internal/config/video_studio_plan_test.go`
- Test (new): `cmd/config-merger/video_studio_plan_merge_test.go`
- Test (new): `internal/server/video_studio_plan_config_test.go`

Existing suites this task must keep green, in `go test ./internal/config` and `go test ./cmd/config-merger`:
- `TestVideoStudioConfigRoundTripAndLimits`: compares whole structs; still compiles because `Plan` holds only comparable fields.
- `TestRunUpgradesPreProVideoStudioConfigThatStillLoads`
- `TestVideoStudioMigrationForcesTheInPlaceWrite`: needs the four limit lines to appear exactly once in the template; they still do.
- `TestRunVideoStudioMigrationRunsOnce`
- `TestVideoStudioMigrationTargetsAreTheCurrentDefaults`

`cmd/config-merger` needs no code change: `deepMerge` adds missing nested keys.

- [ ] **Step 1: Impact check.** Run upstream impact for `NormalizeVideoStudioConfig`, `ProviderReferences` and `validateDecisionsOnlyProviderReferences`. Expected callers of `NormalizeVideoStudioConfig`:
  - `config.load` (`config.go:1130`)
  - `internal/server/config_handlers_main.go:702`
  - `internal/server/config_patch.go:112`
  - `normalizeVideoStudioConfigSnapshot` (`video_studio_handlers.go:151`)
  - tests

  All edits only add behaviour.

  Also run upstream impact for `videoStudioConfigRootsChanged`. Expected callers: `(*Server).replaceConfigSnapshot` (`server.go:681`), the job admission in `video_studio_handlers.go:640` and the two worker checks in `video_studio_service.go` (`:471`, `:678`). The edit only stops `video_studio.plan.*` changes from counting; every other compared field still counts.

- [ ] **Step 2: Write the failing tests.** Create `internal/config/video_studio_plan_test.go`:

```go
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVideoStudioPlanConfigDefaultsAndClamps(t *testing.T) {
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); cfg.VideoStudio.Plan != want {
		t.Fatalf("defaults = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
	cfg, err = loadStorymakerYAML(t, "video_studio:\n  plan:\n    enabled: false\n    agent_provider: \" main \"\n    agent_model: \" m1 \"\n    max_daily_jobs: 5000\n    job_timeout_seconds: 30\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (VideoStudioPlanConfig{AgentProvider: "main", AgentModel: "m1", MaxDailyJobs: 1000, JobTimeoutSeconds: 120}); cfg.VideoStudio.Plan != want {
		t.Fatalf("explicit values = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
	for _, tc := range []struct{ in, want VideoStudioPlanConfig }{
		{VideoStudioPlanConfig{MaxDailyJobs: -3, JobTimeoutSeconds: 7200}, VideoStudioPlanConfig{MaxDailyJobs: 1, JobTimeoutSeconds: 3600}},
		{VideoStudioPlanConfig{MaxDailyJobs: 1000, JobTimeoutSeconds: 120}, VideoStudioPlanConfig{MaxDailyJobs: 1000, JobTimeoutSeconds: 120}},
		{VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 1, JobTimeoutSeconds: 3600}, VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 1, JobTimeoutSeconds: 3600}},
	} {
		c := VideoStudioConfig{Plan: tc.in}
		if err := NormalizeVideoStudioConfig(&c); err != nil {
			t.Fatalf("%+v rejected: %v", tc.in, err)
		}
		if c.Plan != tc.want {
			t.Errorf("normalize %+v = %+v, want %+v", tc.in, c.Plan, tc.want)
		}
	}
}

// The shipped template must describe the same defaults the loader applies, plan block included.
func TestVideoStudioTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["video_studio"]
	if !ok {
		t.Fatal("config_template.yaml has no video_studio: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl VideoStudioConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode into VideoStudioConfig: %v", err)
	}
	if want := (VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); tpl.Plan != want {
		t.Fatalf("the template must write the plan defaults: %+v", tpl.Plan)
	}
	if err := NormalizeVideoStudioConfig(&tpl); err != nil {
		t.Fatalf("template values rejected: %v", err)
	}
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.VideoStudio {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.VideoStudio)
	}
}

func TestVideoStudioPlanAgentProviderReferences(t *testing.T) {
	var c Config
	c.VideoStudio.Plan.AgentProvider = "p1"
	found := false
	for _, ref := range c.ProviderReferences() {
		if ref.Path == "video_studio.plan.agent_provider" && ref.Role == "video_studio_plan_agent" && ref.ProviderID == "p1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("video_studio.plan.agent_provider reference missing: %+v", c.ProviderReferences())
	}
	c.Providers = []ProviderEntry{{ID: "p1", Type: "openrouter", Usage: ProviderUsageDecisions}}
	if err := validateDecisionsOnlyProviderReferences(&c); err == nil || !strings.Contains(err.Error(), "video_studio.plan.agent_provider") {
		t.Fatalf("a decisions-only provider must not plan clips: %v", err)
	}
}
```

Create `cmd/config-merger/video_studio_plan_merge_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// A config written before plan mode gains the video_studio.plan block from the template, and config.Load reads its
// defaults.
func TestRunAddsVideoStudioPlanDefaults(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(sourcePath, []byte("server:\n    port: 8088\n"+preProVideoStudioSection), 0o600); err != nil {
		t.Fatal(err)
	}
	run(sourcePath, filepath.Join("..", "..", "config_template.yaml"), sourcePath)
	merged := mergedFileMap(t, sourcePath)
	for key, want := range map[string]interface{}{"enabled": true, "agent_provider": "", "agent_model": "", "max_daily_jobs": 20, "job_timeout_seconds": 900} {
		if got := mergedValue(merged, "video_studio.plan."+key); got != want {
			t.Fatalf("video_studio.plan.%s = %#v, want %#v", key, got, want)
		}
	}
	cfg, err := config.Load(sourcePath)
	if err != nil {
		t.Fatalf("Load(merged): %v", err)
	}
	if want := (config.VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); cfg.VideoStudio.Plan != want {
		t.Fatalf("plan after the upgrade = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
}
```

Create `internal/server/video_studio_plan_config_test.go` (controller decision: plan settings must not cancel running Video Studio FFmpeg jobs):

```go
package server

import (
	"testing"

	"aurago/internal/config"
)

// Plan mode settings only steer the planning agent. Saving them must not count as a Video Studio root change, which
// cancels running FFmpeg jobs; every other Video Studio setting still counts.
func TestVideoStudioPlanSettingsKeepVideoStudioJobs(t *testing.T) {
	previous := &config.Config{}
	previous.VideoStudio.Enabled = true
	previous.VideoStudio.Plan = config.VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}
	next := *previous
	next.VideoStudio.Plan = config.VideoStudioPlanConfig{AgentProvider: "p1", AgentModel: "m1", MaxDailyJobs: 5, JobTimeoutSeconds: 120}
	if videoStudioConfigRootsChanged(previous, &next) {
		t.Fatal("a plan settings save counts as a Video Studio root change")
	}
	next.VideoStudio.RenderTimeoutSeconds = 600
	if !videoStudioConfigRootsChanged(previous, &next) {
		t.Fatal("a render timeout change must still count as a Video Studio root change")
	}
}
```

- [ ] **Step 3: Run them.** `go test ./internal/config -run '(?i)VideoStudio' -count=1`, `go test ./cmd/config-merger -run 'TestRunAddsVideoStudioPlanDefaults' -count=1` and `go test ./internal/server -run '(?i)VideoStudioPlanSettings' -count=1`. Expected: FAIL, all three builds fail with `undefined: VideoStudioPlanConfig` / `undefined: config.VideoStudioPlanConfig`.

- [ ] **Step 4: Implement.** In `internal/config/video_studio.go`:

**Old:**
```go
	HardwareEncoding     string `yaml:"hardware_encoding" json:"hardware_encoding"`
	HardwareDevice       string `yaml:"hardware_device" json:"hardware_device"`
}
```
**New:**
```go
	HardwareEncoding     string `yaml:"hardware_encoding" json:"hardware_encoding"`
	HardwareDevice       string `yaml:"hardware_device" json:"hardware_device"`

	// Plan configures plan mode: the agent that plans a clip as a storyboard.
	Plan VideoStudioPlanConfig `yaml:"plan" json:"plan"`
}

// VideoStudioPlanConfig configures plan mode: an agent plans a clip from an idea as a storyboard that the user reviews
// and edits (docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md). Enabled defaults to true in the
// loader.
type VideoStudioPlanConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// AgentProvider is a provider id for planning jobs; empty means the main route.
	AgentProvider string `yaml:"agent_provider" json:"agent_provider"`
	// AgentModel overrides the provider's model; empty means the provider's model.
	AgentModel string `yaml:"agent_model" json:"agent_model"`
	// MaxDailyJobs limits planning job starts (new plans and scene revisions) per local day.
	MaxDailyJobs int `yaml:"max_daily_jobs" json:"max_daily_jobs"`
	// JobTimeoutSeconds bounds one planning job.
	JobTimeoutSeconds int `yaml:"job_timeout_seconds" json:"job_timeout_seconds"`
}
```

**Old:**
```go
// NormalizeVideoStudioConfig applies defaults without enabling the editor.
func NormalizeVideoStudioConfig(c *VideoStudioConfig) error {
	if c.MaxProjectSizeMB == 0 {
```
**New:**
```go
// NormalizeVideoStudioConfig applies defaults without enabling the editor. The plan settings go through
// normalizeVideoStudioPlan, which never fails.
func NormalizeVideoStudioConfig(c *VideoStudioConfig) error {
	normalizeVideoStudioPlan(&c.Plan)
	if c.MaxProjectSizeMB == 0 {
```

**Old:**
```go
	c.MaxAssetSizeMB = min(c.MaxAssetSizeMB, c.MaxProjectSizeMB)
	c.MaxExportSizeMB = min(c.MaxExportSizeMB, c.MaxProjectSizeMB)
	return nil
}
```
**New:**
```go
	c.MaxAssetSizeMB = min(c.MaxAssetSizeMB, c.MaxProjectSizeMB)
	c.MaxExportSizeMB = min(c.MaxExportSizeMB, c.MaxProjectSizeMB)
	return nil
}

const (
	videoStudioPlanDefaultMaxDailyJobs      = 20
	videoStudioPlanDefaultJobTimeoutSeconds = 900
)

// normalizeVideoStudioPlan trims the agent route and applies the defaults for unset (zero) values. Limits outside
// their range are clamped, not rejected: 1 to 1000 jobs per day, 120 to 3600 seconds per job. It does not change
// Enabled.
func normalizeVideoStudioPlan(p *VideoStudioPlanConfig) {
	p.AgentProvider = strings.TrimSpace(p.AgentProvider)
	p.AgentModel = strings.TrimSpace(p.AgentModel)
	if p.MaxDailyJobs == 0 {
		p.MaxDailyJobs = videoStudioPlanDefaultMaxDailyJobs
	}
	p.MaxDailyJobs = min(max(p.MaxDailyJobs, 1), 1000)
	if p.JobTimeoutSeconds == 0 {
		p.JobTimeoutSeconds = videoStudioPlanDefaultJobTimeoutSeconds
	}
	p.JobTimeoutSeconds = min(max(p.JobTimeoutSeconds, 120), 3600)
}
```

In `internal/config/config.go` (`load` defaults):

**Old:**
```go
	cfg.Storymaker.Enabled = true
	cfg.Storymaker.Agent.Enabled = true
```
**New:**
```go
	cfg.Storymaker.Enabled = true
	cfg.Storymaker.Agent.Enabled = true

	// Video Studio plan mode is on by default; the editor itself stays opt-in (video_studio.enabled).
	cfg.VideoStudio.Plan.Enabled = true
```

In `internal/config/provider_references.go`:

**Old:**
```go
	add("storymaker.agent.provider", "storymaker_agent", c.Storymaker.Agent.Provider)
```
**New:**
```go
	add("storymaker.agent.provider", "storymaker_agent", c.Storymaker.Agent.Provider)
	add("video_studio.plan.agent_provider", "video_studio_plan_agent", c.VideoStudio.Plan.AgentProvider)
```

In `internal/config/decisions.go`:

**Old:**
```go
		{"storymaker.agent.provider", cfg.Storymaker.Agent.Provider},
```
**New:**
```go
		{"storymaker.agent.provider", cfg.Storymaker.Agent.Provider},
		{"video_studio.plan.agent_provider", cfg.VideoStudio.Plan.AgentProvider},
```

In `config_template.yaml` (4-space YAML indentation):

**Old:**
```yaml
    render_timeout_seconds: 7200
    hardware_encoding: auto
    hardware_device: ""
```
**New:**
```yaml
    render_timeout_seconds: 7200
    hardware_encoding: auto
    hardware_device: ""
    plan:
        enabled: true               # plan mode: the agent plans a clip as a storyboard you review before anything is generated
        agent_provider: ""          # provider id for planning jobs; empty = main route
        agent_model: ""             # model override for planning jobs; empty = the provider's model
        max_daily_jobs: 20          # planning jobs (new plans and scene revisions) per local day (1-1000)
        job_timeout_seconds: 900    # limit of one planning job (120-3600)
```

In `internal/server/video_studio_service.go`, `videoStudioConfigRootsChanged` (plan mode settings only steer the planning agent):

**Old:**
```go
func videoStudioConfigRootsChanged(previous, next *config.Config) bool {
	if previous == nil || next == nil {
		return false
	}
	if previous.VideoStudio != next.VideoStudio {
		return true
	}
```
**New:**
```go
func videoStudioConfigRootsChanged(previous, next *config.Config) bool {
	if previous == nil || next == nil {
		return false
	}
	// Plan mode settings only steer the planning agent, so saving them must not cancel running FFmpeg jobs.
	previousStudio, nextStudio := previous.VideoStudio, next.VideoStudio
	previousStudio.Plan, nextStudio.Plan = config.VideoStudioPlanConfig{}, config.VideoStudioPlanConfig{}
	if previousStudio != nextStudio {
		return true
	}
```

- [ ] **Step 5: Run them.** `gofmt -w internal/config/video_studio.go internal/config/config.go internal/config/provider_references.go internal/config/decisions.go internal/config/video_studio_plan_test.go cmd/config-merger/video_studio_plan_merge_test.go internal/server/video_studio_service.go internal/server/video_studio_plan_config_test.go`, then `go test ./internal/config -run '(?i)VideoStudio' -count=1`, `go test ./cmd/config-merger -run '(?i)VideoStudio' -count=1` and `go test ./internal/server -run '(?i)VideoStudioPlanSettings' -count=1`. Expected: all three `ok`, including the existing Video Studio merger and migration tests.

- [ ] **Step 6: Commit.**
```
git add internal/config/video_studio.go internal/config/config.go internal/config/provider_references.go internal/config/decisions.go internal/config/video_studio_plan_test.go config_template.yaml cmd/config-merger/video_studio_plan_merge_test.go internal/server/video_studio_service.go internal/server/video_studio_plan_config_test.go
git commit -m "feat(video-studio): video_studio.plan config keys" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A8: plan mode settings on the configuration page

**Files:**
- Modify: `ui/cfg/virtual_desktop.js`, `vdCfgVideoStudio()` (lines 179-203)
- Modify: `ui/lang/meta.json` (after the `help.storymaker.agent.provider` entry)
- Modify: the 16 files `ui/lang/config/common/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`, inserting after `  "config.video_studio.title": "Video Studio",` (line 2 in every file)
- Test (new): `ui/config_video_studio_plan_test.go`
- Test (modify): `ui/config_video_studio_browser_test.go`, `TestConfigVideoStudioBrowser`
- Unchanged but must stay green: `TestTranslations_AllKeysPresentInAllLanguages` and `TestTranslations_ConfigCfgJSKeysExist` (`ui/i18n_lint_test.go`)

- [ ] **Step 1: Impact check.** `vdCfgVideoStudio` is called only from the Virtual Desktop section renderer (`ui/cfg/virtual_desktop.js:91`).

- [ ] **Step 2: Write the failing tests.** Create `ui/config_video_studio_plan_test.go`:

```go
package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The plan mode block of the Video Studio settings on the Virtual Desktop configuration page:
// video_studio.plan.{enabled, agent_provider, agent_model, max_daily_jobs, job_timeout_seconds} and its sixteen-language
// texts in lang/config/common.
var videoStudioPlanConfigKeys = []string{
	"plan_title", "plan_help", "plan_enabled", "plan_agent_provider", "plan_agent_provider_help", "plan_agent_provider_main",
	"plan_agent_model", "plan_agent_model_help", "plan_max_daily_jobs", "plan_max_daily_jobs_help", "plan_job_timeout",
	"plan_job_timeout_help",
}

func TestConfigVideoStudioPlanBlockIsWired(t *testing.T) {
	t.Parallel()
	src := strings.ReplaceAll(string(mustReadUIFile(t, filepath.Join("cfg", "virtual_desktop.js"))), "\r\n", "\n")
	for _, marker := range []string{
		"if (typeof plan.enabled !== 'boolean') plan.enabled = true;",
		"'video_studio.plan.enabled'",
		`data-path="video_studio.plan.agent_provider" data-config-choice="providers"`,
		"cfgChoiceOptionsHTML('providers', helpTexts['video_studio.plan.agent_provider'] || {}, plan.agent_provider || '')",
		`data-path="video_studio.plan.agent_model"`,
		`min="1" max="1000" value="' + escapeAttr(String(plan.max_daily_jobs || 20)) + '" data-path="video_studio.plan.max_daily_jobs"`,
		`min="120" max="3600" value="' + escapeAttr(String(plan.job_timeout_seconds || 900)) + '" data-path="video_studio.plan.job_timeout_seconds"`,
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}
	for _, key := range videoStudioPlanConfigKeys {
		if key != "plan_agent_provider_main" && !strings.Contains(src, "'"+key+"'") {
			t.Errorf("virtual_desktop.js never uses config.video_studio.%s", key)
		}
	}
	var meta map[string]map[string]any
	if err := json.Unmarshal(mustReadUIFile(t, filepath.Join("lang", "meta.json")), &meta); err != nil {
		t.Fatal(err)
	}
	if entry := meta["help.video_studio.plan.agent_provider"]; entry["provider_ref"] != true || entry["empty_label_key"] != "config.video_studio.plan_agent_provider_main" {
		t.Fatalf("lang/meta.json help.video_studio.plan.agent_provider = %v", entry)
	}
	// The form limits and defaults are the ones of normalizeVideoStudioPlan.
	normalize := string(mustReadUIFile(t, filepath.Join("..", "internal", "config", "video_studio.go")))
	for _, fragment := range []string{"min(max(p.MaxDailyJobs, 1), 1000)", "min(max(p.JobTimeoutSeconds, 120), 3600)", "= 20", "= 900"} {
		if !strings.Contains(normalize, fragment) {
			t.Errorf("internal/config/video_studio.go no longer holds %q, which the form promises", fragment)
		}
	}
}

func TestConfigVideoStudioPlanTranslations(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "config/common", "en")
	for _, lang := range storymakerLanguages {
		values := storymakerLocale(t, "config/common", lang)
		for _, key := range videoStudioPlanConfigKeys {
			full := "config.video_studio." + key
			text := strings.TrimSpace(values[full])
			if text == "" || strings.ContainsRune(text, utf8.RuneError) {
				t.Errorf("%s: %s is missing or broken", lang, full)
				continue
			}
			if lang != "en" && text == strings.TrimSpace(english[full]) {
				t.Errorf("%s: %s is untranslated", lang, full)
			}
			if lang == "de" && (strings.Contains(" "+text+" ", " Sie ") || strings.Contains(" "+text+" ", " Ihre ") || strings.Contains(" "+text+" ", " Ihnen ")) {
				t.Errorf("de: %s uses formal address", full)
			}
		}
	}
}
```

In `ui/config_video_studio_browser_test.go`:

**Old:**
```go
	if !page.MustEval(`() => document.querySelector('[data-path="video_studio.max_asset_size_mb"]').value==='4096' && !document.querySelector('[data-path="video_studio.enabled"]').classList.contains('on')`).Bool() {
		t.Fatal("Video Studio defaults must be visible and disabled")
	}
	page.MustEval(`() => {
        toggleBool(document.querySelector('[data-path="video_studio.enabled"]'));
        const input = document.querySelector('[data-path="video_studio.ffmpeg_path"]');
        input.value = 'C:/Media Tools/ffmpeg.exe'; input.dispatchEvent(new Event('change', {bubbles:true}));
    }`)
```
**New:**
```go
	if !page.MustEval(`() => document.querySelector('[data-path="video_studio.max_asset_size_mb"]').value==='4096' && !document.querySelector('[data-path="video_studio.enabled"]').classList.contains('on')`).Bool() {
		t.Fatal("Video Studio defaults must be visible and disabled")
	}
	// Plan mode is on by default and shows the loader's limits and the main LLM.
	if !page.MustEval(`() => document.querySelector('[data-path="video_studio.plan.enabled"]').classList.contains('on') && document.querySelector('[data-path="video_studio.plan.max_daily_jobs"]').value==='20' && document.querySelector('[data-path="video_studio.plan.job_timeout_seconds"]').value==='900' && document.querySelector('[data-path="video_studio.plan.agent_provider"]').value===''`).Bool() {
		t.Fatal("the plan mode defaults are not what the server applies")
	}
	page.MustEval(`() => {
        toggleBool(document.querySelector('[data-path="video_studio.enabled"]'));
        const input = document.querySelector('[data-path="video_studio.ffmpeg_path"]');
        input.value = 'C:/Media Tools/ffmpeg.exe'; input.dispatchEvent(new Event('change', {bubbles:true}));
        const jobs = document.querySelector('[data-path="video_studio.plan.max_daily_jobs"]');
        jobs.value = '5'; jobs.dispatchEvent(new Event('change', {bubbles:true}));
    }`)
```

**Old:**
```go
	if !page.MustEval(`() => videoSavedConfig.video_studio.enabled===true && videoSavedConfig.video_studio.ffmpeg_path==='C:/Media Tools/ffmpeg.exe'`).Bool() {
```
**New:**
```go
	if !page.MustEval(`() => videoSavedConfig.video_studio.enabled===true && videoSavedConfig.video_studio.ffmpeg_path==='C:/Media Tools/ffmpeg.exe' && videoSavedConfig.video_studio.plan.enabled===true && videoSavedConfig.video_studio.plan.max_daily_jobs===5`).Bool() {
```

- [ ] **Step 3: Run them.** `go test ./ui -run '(?i)ConfigVideoStudioPlan' -count=1`. Expected: FAIL with `virtual_desktop.js lacks ...`, `lang/meta.json help.video_studio.plan.agent_provider = map[]` and `missing or broken` errors.

- [ ] **Step 4: Implement.** In `ui/cfg/virtual_desktop.js`:

**Old:**
```js
function vdCfgVideoStudio() {
    const data = configData.video_studio || {};
    const prefix = 'config.video_studio.';
```
**New:**
```js
function vdCfgVideoStudio() {
    const data = configData.video_studio || (configData.video_studio = {});
    // Plan mode is on unless the YAML switches it off (config.Load default).
    const plan = data.plan || (data.plan = {});
    if (typeof plan.enabled !== 'boolean') plan.enabled = true;
    const prefix = 'config.video_studio.';
```

**Old:**
```js
    html += '</div><div class="field-group"><button type="button" id="vd-video-studio-btn" class="btn-save dc-test-btn" data-config-action-draft="false" onclick="vdCfgTestVideoStudio(this)">' + t(prefix + 'check') + '</button>' + cfgActionResult.html('vd-video-studio-result') + '</div>';
    return html;
}
```
**New:**
```js
    html += '</div><div class="field-group"><button type="button" id="vd-video-studio-btn" class="btn-save dc-test-btn" data-config-action-draft="false" onclick="vdCfgTestVideoStudio(this)">' + t(prefix + 'check') + '</button>' + cfgActionResult.html('vd-video-studio-result') + '</div>';
    html += '<h3>' + t(prefix + 'plan_title') + '</h3>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow(prefix + 'plan_enabled', prefix + 'plan_help', plan.enabled === true, 'video_studio.plan.enabled');
    html += '</div><div class="field-grid two-cols">';
    html += vdCfgField(prefix + 'plan_agent_provider', prefix + 'plan_agent_provider_help', '<select class="field-select" data-path="video_studio.plan.agent_provider" data-config-choice="providers" aria-label="' + escapeAttr(t(prefix + 'plan_agent_provider')) + '">' + cfgChoiceOptionsHTML('providers', helpTexts['video_studio.plan.agent_provider'] || {}, plan.agent_provider || '') + '</select>' + cfgChoiceHintHTML('providers'));
    html += vdCfgField(prefix + 'plan_agent_model', prefix + 'plan_agent_model_help', '<input class="field-input" type="text" spellcheck="false" autocomplete="off" value="' + escapeAttr(plan.agent_model || '') + '" data-path="video_studio.plan.agent_model">');
    html += vdCfgField(prefix + 'plan_max_daily_jobs', prefix + 'plan_max_daily_jobs_help', '<input class="field-input" type="number" min="1" max="1000" value="' + escapeAttr(String(plan.max_daily_jobs || 20)) + '" data-path="video_studio.plan.max_daily_jobs">');
    html += vdCfgField(prefix + 'plan_job_timeout', prefix + 'plan_job_timeout_help', '<input class="field-input" type="number" min="120" max="3600" value="' + escapeAttr(String(plan.job_timeout_seconds || 900)) + '" data-path="video_studio.plan.job_timeout_seconds">');
    html += '</div>';
    return html;
}
```

In `ui/lang/meta.json`:

**Old:**
```json
  "help.storymaker.agent.provider": {
    "provider_ref": true,
    "empty_label_key": "config.storymaker.agent_provider_main"
  },
```
**New:**
```json
  "help.storymaker.agent.provider": {
    "provider_ref": true,
    "empty_label_key": "config.storymaker.agent_provider_main"
  },
  "help.video_studio.plan.agent_provider": {
    "provider_ref": true,
    "empty_label_key": "config.video_studio.plan_agent_provider_main"
  },
```

In each `ui/lang/config/common/<lang>.json`, insert that language's block below **after** the anchor line `  "config.video_studio.title": "Video Studio",` (2-space indentation, UTF-8, LF):

`en.json`:
```json
  "config.video_studio.plan_title": "Plan mode",
  "config.video_studio.plan_help": "The agent turns an idea into a storyboard of scenes that you review and edit in Video Studio. Planning only uses the LLM; it generates no images, voices or videos. Needs Video Generation with MiniMax or Google Veo.",
  "config.video_studio.plan_enabled": "Enable plan mode",
  "config.video_studio.plan_agent_provider": "Planning provider",
  "config.video_studio.plan_agent_provider_help": "The LLM provider for planning jobs. Main LLM uses the main route with its configured failover. A chosen provider is pinned to the job without failover.",
  "config.video_studio.plan_agent_provider_main": "Main LLM",
  "config.video_studio.plan_agent_model": "Planning model (optional)",
  "config.video_studio.plan_agent_model_help": "Replaces the model for planning jobs. Empty means the model of the provider.",
  "config.video_studio.plan_max_daily_jobs": "Planning jobs per day",
  "config.video_studio.plan_max_daily_jobs_help": "New plans and scene revisions. Between 1 and 1000.",
  "config.video_studio.plan_job_timeout": "Planning time limit (seconds)",
  "config.video_studio.plan_job_timeout_help": "A planning job that runs longer stops; the scenes planned so far stay. Between 120 and 3600.",
```

`de.json`:
```json
  "config.video_studio.plan_title": "Planmodus",
  "config.video_studio.plan_help": "Der Agent macht aus einer Idee ein Storyboard aus Szenen, das du in Video Studio prüfst und bearbeitest. Die Planung nutzt nur das LLM und erzeugt keine Bilder, Stimmen oder Videos. Braucht die Videoerzeugung mit MiniMax oder Google Veo.",
  "config.video_studio.plan_enabled": "Planmodus aktivieren",
  "config.video_studio.plan_agent_provider": "Anbieter für die Planung",
  "config.video_studio.plan_agent_provider_help": "Der LLM-Anbieter für Planungsaufträge. Haupt-LLM nutzt die Hauptroute mit ihrem eingerichteten Failover. Ein gewählter Anbieter bleibt für den Auftrag fest, ohne Failover.",
  "config.video_studio.plan_agent_provider_main": "Haupt-LLM",
  "config.video_studio.plan_agent_model": "Modell für die Planung (optional)",
  "config.video_studio.plan_agent_model_help": "Ersetzt das Modell für Planungsaufträge. Leer heißt: das Modell des Anbieters.",
  "config.video_studio.plan_max_daily_jobs": "Planungsaufträge pro Tag",
  "config.video_studio.plan_max_daily_jobs_help": "Neue Pläne und Szenen-Überarbeitungen. Zwischen 1 und 1000.",
  "config.video_studio.plan_job_timeout": "Zeitlimit der Planung (Sekunden)",
  "config.video_studio.plan_job_timeout_help": "Ein längerer Planungsauftrag stoppt, die bis dahin geplanten Szenen bleiben erhalten. Zwischen 120 und 3600.",
```

`cs.json`:
```json
  "config.video_studio.plan_title": "Režim plánu",
  "config.video_studio.plan_help": "Agent z nápadu vytvoří storyboard ze scén, který ve Video Studiu zkontrolujete a upravíte. Plánování používá jen LLM a negeneruje žádné obrázky, hlasy ani videa. Vyžaduje generování videa s MiniMax nebo Google Veo.",
  "config.video_studio.plan_enabled": "Zapnout režim plánu",
  "config.video_studio.plan_agent_provider": "Poskytovatel plánování",
  "config.video_studio.plan_agent_provider_help": "Poskytovatel LLM pro úlohy plánování. Hlavní LLM používá hlavní trasu s nastaveným failoverem. Zvolený poskytovatel je pro úlohu pevně daný, bez failoveru.",
  "config.video_studio.plan_agent_provider_main": "Hlavní LLM",
  "config.video_studio.plan_agent_model": "Model plánování (volitelné)",
  "config.video_studio.plan_agent_model_help": "Nahradí model pro úlohy plánování. Prázdné znamená model poskytovatele.",
  "config.video_studio.plan_max_daily_jobs": "Úlohy plánování za den",
  "config.video_studio.plan_max_daily_jobs_help": "Nové plány a úpravy scén. Od 1 do 1000.",
  "config.video_studio.plan_job_timeout": "Časový limit plánování (sekundy)",
  "config.video_studio.plan_job_timeout_help": "Déle běžící úloha plánování se zastaví a dosud naplánované scény zůstanou. Od 120 do 3600.",
```

`da.json`:
```json
  "config.video_studio.plan_title": "Planlægningstilstand",
  "config.video_studio.plan_help": "Agenten gør en idé til et storyboard af scener, som du gennemgår og redigerer i Video Studio. Planlægningen bruger kun LLM'en og genererer ingen billeder, stemmer eller videoer. Kræver videogenerering med MiniMax eller Google Veo.",
  "config.video_studio.plan_enabled": "Aktivér planlægningstilstand",
  "config.video_studio.plan_agent_provider": "Udbyder til planlægning",
  "config.video_studio.plan_agent_provider_help": "LLM-udbyderen til planlægningsjob. Primær LLM bruger hovedruten med dens opsatte failover. En valgt udbyder er fast for jobbet, uden failover.",
  "config.video_studio.plan_agent_provider_main": "Primær LLM",
  "config.video_studio.plan_agent_model": "Model til planlægning (valgfri)",
  "config.video_studio.plan_agent_model_help": "Erstatter modellen til planlægningsjob. Tom betyder udbyderens model.",
  "config.video_studio.plan_max_daily_jobs": "Planlægningsjob pr. dag",
  "config.video_studio.plan_max_daily_jobs_help": "Nye planer og revisioner af scener. Mellem 1 og 1000.",
  "config.video_studio.plan_job_timeout": "Tidsgrænse for planlægning (sekunder)",
  "config.video_studio.plan_job_timeout_help": "Et planlægningsjob, der kører længere, stopper, og de hidtil planlagte scener bliver. Mellem 120 og 3600.",
```

`el.json`:
```json
  "config.video_studio.plan_title": "Λειτουργία σχεδίου",
  "config.video_studio.plan_help": "Ο πράκτορας μετατρέπει μια ιδέα σε storyboard με σκηνές, το οποίο ελέγχετε και επεξεργάζεστε στο Video Studio. Ο σχεδιασμός χρησιμοποιεί μόνο το LLM και δεν δημιουργεί εικόνες, φωνές ή βίντεο. Απαιτεί δημιουργία βίντεο με MiniMax ή Google Veo.",
  "config.video_studio.plan_enabled": "Ενεργοποίηση λειτουργίας σχεδίου",
  "config.video_studio.plan_agent_provider": "Πάροχος σχεδιασμού",
  "config.video_studio.plan_agent_provider_help": "Ο πάροχος LLM για τις εργασίες σχεδιασμού. Το κύριο LLM χρησιμοποιεί την κύρια διαδρομή με το ρυθμισμένο failover της. Ένας επιλεγμένος πάροχος μένει σταθερός για την εργασία, χωρίς failover.",
  "config.video_studio.plan_agent_provider_main": "Κύριο LLM",
  "config.video_studio.plan_agent_model": "Μοντέλο σχεδιασμού (προαιρετικό)",
  "config.video_studio.plan_agent_model_help": "Αντικαθιστά το μοντέλο για τις εργασίες σχεδιασμού. Κενό σημαίνει το μοντέλο του παρόχου.",
  "config.video_studio.plan_max_daily_jobs": "Εργασίες σχεδιασμού ανά ημέρα",
  "config.video_studio.plan_max_daily_jobs_help": "Νέα σχέδια και αναθεωρήσεις σκηνών. Από 1 έως 1000.",
  "config.video_studio.plan_job_timeout": "Χρονικό όριο σχεδιασμού (δευτερόλεπτα)",
  "config.video_studio.plan_job_timeout_help": "Μια εργασία σχεδιασμού που διαρκεί περισσότερο σταματά και οι σκηνές που έχουν ήδη σχεδιαστεί μένουν. Από 120 έως 3600.",
```

`es.json`:
```json
  "config.video_studio.plan_title": "Modo plan",
  "config.video_studio.plan_help": "El agente convierte una idea en un storyboard de escenas que revisas y editas en Video Studio. La planificación solo usa el LLM y no genera imágenes, voces ni vídeos. Requiere la generación de vídeo con MiniMax o Google Veo.",
  "config.video_studio.plan_enabled": "Activar el modo plan",
  "config.video_studio.plan_agent_provider": "Proveedor de la planificación",
  "config.video_studio.plan_agent_provider_help": "El proveedor de LLM para los trabajos de planificación. LLM principal usa la ruta principal con su failover configurado. Un proveedor elegido queda fijado para el trabajo, sin failover.",
  "config.video_studio.plan_agent_provider_main": "LLM principal",
  "config.video_studio.plan_agent_model": "Modelo de la planificación (opcional)",
  "config.video_studio.plan_agent_model_help": "Sustituye el modelo para los trabajos de planificación. Vacío significa el modelo del proveedor.",
  "config.video_studio.plan_max_daily_jobs": "Trabajos de planificación por día",
  "config.video_studio.plan_max_daily_jobs_help": "Planes nuevos y revisiones de escenas. Entre 1 y 1000.",
  "config.video_studio.plan_job_timeout": "Límite de tiempo de la planificación (segundos)",
  "config.video_studio.plan_job_timeout_help": "Un trabajo de planificación que dura más se detiene y las escenas planificadas hasta entonces se conservan. Entre 120 y 3600.",
```

`fr.json`:
```json
  "config.video_studio.plan_title": "Mode plan",
  "config.video_studio.plan_help": "L'agent transforme une idée en storyboard de scènes que vous vérifiez et modifiez dans Video Studio. La planification utilise seulement le LLM et ne génère ni images, ni voix, ni vidéos. Nécessite la génération vidéo avec MiniMax ou Google Veo.",
  "config.video_studio.plan_enabled": "Activer le mode plan",
  "config.video_studio.plan_agent_provider": "Fournisseur de la planification",
  "config.video_studio.plan_agent_provider_help": "Le fournisseur LLM des tâches de planification. LLM principal utilise la route principale avec son failover configuré. Un fournisseur choisi reste fixé pour la tâche, sans failover.",
  "config.video_studio.plan_agent_provider_main": "LLM principal",
  "config.video_studio.plan_agent_model": "Modèle de la planification (facultatif)",
  "config.video_studio.plan_agent_model_help": "Remplace le modèle des tâches de planification. Vide signifie le modèle du fournisseur.",
  "config.video_studio.plan_max_daily_jobs": "Tâches de planification par jour",
  "config.video_studio.plan_max_daily_jobs_help": "Nouveaux plans et révisions de scènes. Entre 1 et 1000.",
  "config.video_studio.plan_job_timeout": "Limite de temps de la planification (secondes)",
  "config.video_studio.plan_job_timeout_help": "Une tâche de planification plus longue s'arrête et les scènes déjà planifiées sont conservées. Entre 120 et 3600.",
```

`hi.json`:
```json
  "config.video_studio.plan_title": "प्लान मोड",
  "config.video_studio.plan_help": "एजेंट किसी विचार को दृश्यों वाले स्टोरीबोर्ड में बदलता है, जिसे आप Video Studio में जाँचते और संपादित करते हैं। योजना केवल LLM का उपयोग करती है और कोई चित्र, आवाज़ या वीडियो नहीं बनाती। इसके लिए MiniMax या Google Veo के साथ वीडियो निर्माण चाहिए।",
  "config.video_studio.plan_enabled": "प्लान मोड चालू करें",
  "config.video_studio.plan_agent_provider": "योजना प्रदाता",
  "config.video_studio.plan_agent_provider_help": "योजना जॉब के लिए LLM प्रदाता। मुख्य LLM अपने सेट किए गए failover के साथ मुख्य रूट का उपयोग करता है। चुना गया प्रदाता जॉब के लिए तय रहता है, बिना failover के।",
  "config.video_studio.plan_agent_provider_main": "मुख्य LLM",
  "config.video_studio.plan_agent_model": "योजना मॉडल (वैकल्पिक)",
  "config.video_studio.plan_agent_model_help": "योजना जॉब के लिए मॉडल बदलता है। ख़ाली का अर्थ है प्रदाता का मॉडल।",
  "config.video_studio.plan_max_daily_jobs": "प्रति दिन योजना जॉब",
  "config.video_studio.plan_max_daily_jobs_help": "नई योजनाएँ और दृश्यों के संशोधन। 1 से 1000 के बीच।",
  "config.video_studio.plan_job_timeout": "योजना समय सीमा (सेकंड)",
  "config.video_studio.plan_job_timeout_help": "इससे लंबा चलने वाला योजना जॉब रुक जाता है और अब तक योजनाबद्ध दृश्य बने रहते हैं। 120 से 3600 के बीच।",
```

`it.json`:
```json
  "config.video_studio.plan_title": "Modalità piano",
  "config.video_studio.plan_help": "L'agente trasforma un'idea in uno storyboard di scene che controlli e modifichi in Video Studio. La pianificazione usa solo l'LLM e non genera immagini, voci o video. Richiede la generazione video con MiniMax o Google Veo.",
  "config.video_studio.plan_enabled": "Attiva la modalità piano",
  "config.video_studio.plan_agent_provider": "Provider della pianificazione",
  "config.video_studio.plan_agent_provider_help": "Il provider LLM per i processi di pianificazione. LLM principale usa il percorso principale con il failover configurato. Un provider scelto resta fisso per il processo, senza failover.",
  "config.video_studio.plan_agent_provider_main": "LLM principale",
  "config.video_studio.plan_agent_model": "Modello della pianificazione (facoltativo)",
  "config.video_studio.plan_agent_model_help": "Sostituisce il modello per i processi di pianificazione. Vuoto significa il modello del provider.",
  "config.video_studio.plan_max_daily_jobs": "Processi di pianificazione al giorno",
  "config.video_studio.plan_max_daily_jobs_help": "Nuovi piani e revisioni delle scene. Tra 1 e 1000.",
  "config.video_studio.plan_job_timeout": "Limite di tempo della pianificazione (secondi)",
  "config.video_studio.plan_job_timeout_help": "Un processo di pianificazione più lungo si ferma e le scene pianificate fin lì restano. Tra 120 e 3600.",
```

`ja.json`:
```json
  "config.video_studio.plan_title": "プランモード",
  "config.video_studio.plan_help": "エージェントがアイデアをシーンのストーリーボードにまとめ、Video Studio で確認・編集できます。プランニングは LLM だけを使い、画像・音声・動画は生成しません。MiniMax または Google Veo の動画生成が必要です。",
  "config.video_studio.plan_enabled": "プランモードを有効にする",
  "config.video_studio.plan_agent_provider": "プランニングのプロバイダー",
  "config.video_studio.plan_agent_provider_help": "プランニングジョブの LLM プロバイダーです。メイン LLM は設定済みのフェイルオーバーを含むメインルートを使います。選んだプロバイダーはジョブ中固定され、フェイルオーバーしません。",
  "config.video_studio.plan_agent_provider_main": "メイン LLM",
  "config.video_studio.plan_agent_model": "プランニングのモデル (任意)",
  "config.video_studio.plan_agent_model_help": "プランニングジョブのモデルを置き換えます。空欄ならプロバイダーのモデルを使います。",
  "config.video_studio.plan_max_daily_jobs": "1 日あたりのプランニングジョブ",
  "config.video_studio.plan_max_daily_jobs_help": "新しいプランとシーンの修正です。1〜1000。",
  "config.video_studio.plan_job_timeout": "プランニングの制限時間 (秒)",
  "config.video_studio.plan_job_timeout_help": "これより長いジョブは停止し、それまでに計画したシーンは残ります。120〜3600。",
```

`nl.json`:
```json
  "config.video_studio.plan_title": "Planmodus",
  "config.video_studio.plan_help": "De agent maakt van een idee een storyboard met scènes dat je in Video Studio controleert en bewerkt. Het plannen gebruikt alleen het LLM en maakt geen afbeeldingen, stemmen of video's. Vereist videogeneratie met MiniMax of Google Veo.",
  "config.video_studio.plan_enabled": "Planmodus inschakelen",
  "config.video_studio.plan_agent_provider": "Provider voor het plannen",
  "config.video_studio.plan_agent_provider_help": "De LLM-provider voor plantaken. Hoofd-LLM gebruikt de hoofdroute met de ingestelde failover. Een gekozen provider blijft vast voor de taak, zonder failover.",
  "config.video_studio.plan_agent_provider_main": "Hoofd-LLM",
  "config.video_studio.plan_agent_model": "Model voor het plannen (optioneel)",
  "config.video_studio.plan_agent_model_help": "Vervangt het model voor plantaken. Leeg betekent het model van de provider.",
  "config.video_studio.plan_max_daily_jobs": "Plantaken per dag",
  "config.video_studio.plan_max_daily_jobs_help": "Nieuwe plannen en herzieningen van scènes. Tussen 1 en 1000.",
  "config.video_studio.plan_job_timeout": "Tijdslimiet van het plannen (seconden)",
  "config.video_studio.plan_job_timeout_help": "Een plantaak die langer duurt stopt, en de tot dan geplande scènes blijven bewaard. Tussen 120 en 3600.",
```

`no.json`:
```json
  "config.video_studio.plan_title": "Planleggingsmodus",
  "config.video_studio.plan_help": "Agenten gjør en idé om til et storyboard med scener som du går gjennom og redigerer i Video Studio. Planleggingen bruker bare LLM-en og lager ingen bilder, stemmer eller videoer. Krever videogenerering med MiniMax eller Google Veo.",
  "config.video_studio.plan_enabled": "Aktiver planleggingsmodus",
  "config.video_studio.plan_agent_provider": "Leverandør for planlegging",
  "config.video_studio.plan_agent_provider_help": "LLM-leverandøren for planleggingsjobber. Hoved-LLM bruker hovedruten med den oppsatte failoveren. En valgt leverandør er fast for jobben, uten failover.",
  "config.video_studio.plan_agent_provider_main": "Hoved-LLM",
  "config.video_studio.plan_agent_model": "Modell for planlegging (valgfritt)",
  "config.video_studio.plan_agent_model_help": "Erstatter modellen for planleggingsjobber. Tom betyr leverandørens modell.",
  "config.video_studio.plan_max_daily_jobs": "Planleggingsjobber per dag",
  "config.video_studio.plan_max_daily_jobs_help": "Nye planer og revisjoner av scener. Mellom 1 og 1000.",
  "config.video_studio.plan_job_timeout": "Tidsgrense for planlegging (sekunder)",
  "config.video_studio.plan_job_timeout_help": "En planleggingsjobb som varer lenger, stopper, og scenene som er planlagt så langt, blir værende. Mellom 120 og 3600.",
```

`pl.json`:
```json
  "config.video_studio.plan_title": "Tryb planu",
  "config.video_studio.plan_help": "Agent zamienia pomysł w storyboard ze scen, który sprawdzasz i edytujesz w Video Studio. Planowanie używa tylko LLM i nie generuje obrazów, głosów ani filmów. Wymaga generowania wideo z MiniMax lub Google Veo.",
  "config.video_studio.plan_enabled": "Włącz tryb planu",
  "config.video_studio.plan_agent_provider": "Dostawca planowania",
  "config.video_studio.plan_agent_provider_help": "Dostawca LLM dla zadań planowania. Główny LLM używa głównej trasy ze skonfigurowanym failoverem. Wybrany dostawca jest stały dla zadania, bez failovera.",
  "config.video_studio.plan_agent_provider_main": "Główny LLM",
  "config.video_studio.plan_agent_model": "Model planowania (opcjonalnie)",
  "config.video_studio.plan_agent_model_help": "Zastępuje model dla zadań planowania. Puste pole oznacza model dostawcy.",
  "config.video_studio.plan_max_daily_jobs": "Zadania planowania dziennie",
  "config.video_studio.plan_max_daily_jobs_help": "Nowe plany i poprawki scen. Od 1 do 1000.",
  "config.video_studio.plan_job_timeout": "Limit czasu planowania (sekundy)",
  "config.video_studio.plan_job_timeout_help": "Dłuższe zadanie planowania zatrzymuje się, a dotychczas zaplanowane sceny zostają. Od 120 do 3600.",
```

`pt.json`:
```json
  "config.video_studio.plan_title": "Modo de plano",
  "config.video_studio.plan_help": "O agente transforma uma ideia em um storyboard de cenas que você revisa e edita no Video Studio. O planejamento usa apenas o LLM e não gera imagens, vozes nem vídeos. Requer a geração de vídeo com MiniMax ou Google Veo.",
  "config.video_studio.plan_enabled": "Ativar o modo de plano",
  "config.video_studio.plan_agent_provider": "Provedor do planejamento",
  "config.video_studio.plan_agent_provider_help": "O provedor de LLM para as tarefas de planejamento. LLM principal usa a rota principal com o failover configurado. Um provedor escolhido fica fixo para a tarefa, sem failover.",
  "config.video_studio.plan_agent_provider_main": "LLM principal",
  "config.video_studio.plan_agent_model": "Modelo do planejamento (opcional)",
  "config.video_studio.plan_agent_model_help": "Substitui o modelo das tarefas de planejamento. Vazio significa o modelo do provedor.",
  "config.video_studio.plan_max_daily_jobs": "Tarefas de planejamento por dia",
  "config.video_studio.plan_max_daily_jobs_help": "Planos novos e revisões de cenas. Entre 1 e 1000.",
  "config.video_studio.plan_job_timeout": "Limite de tempo do planejamento (segundos)",
  "config.video_studio.plan_job_timeout_help": "Uma tarefa de planejamento mais longa para, e as cenas planejadas até ali permanecem. Entre 120 e 3600.",
```

`sv.json`:
```json
  "config.video_studio.plan_title": "Planeringsläge",
  "config.video_studio.plan_help": "Agenten gör en idé till ett storyboard med scener som du granskar och redigerar i Video Studio. Planeringen använder bara LLM:en och skapar inga bilder, röster eller videor. Kräver videogenerering med MiniMax eller Google Veo.",
  "config.video_studio.plan_enabled": "Aktivera planeringsläge",
  "config.video_studio.plan_agent_provider": "Leverantör för planering",
  "config.video_studio.plan_agent_provider_help": "LLM-leverantören för planeringsjobb. Huvud-LLM använder huvudrutten med dess inställda failover. En vald leverantör är fast för jobbet, utan failover.",
  "config.video_studio.plan_agent_provider_main": "Huvud-LLM",
  "config.video_studio.plan_agent_model": "Modell för planering (valfritt)",
  "config.video_studio.plan_agent_model_help": "Ersätter modellen för planeringsjobb. Tomt betyder leverantörens modell.",
  "config.video_studio.plan_max_daily_jobs": "Planeringsjobb per dag",
  "config.video_studio.plan_max_daily_jobs_help": "Nya planer och revideringar av scener. Mellan 1 och 1000.",
  "config.video_studio.plan_job_timeout": "Tidsgräns för planering (sekunder)",
  "config.video_studio.plan_job_timeout_help": "Ett planeringsjobb som tar längre tid stoppas, och de scener som planerats hittills finns kvar. Mellan 120 och 3600.",
```

`zh.json`:
```json
  "config.video_studio.plan_title": "规划模式",
  "config.video_studio.plan_help": "智能体把一个想法变成由场景组成的故事板，你可以在 Video Studio 中检查和编辑。规划只使用 LLM，不会生成图片、语音或视频。需要使用 MiniMax 或 Google Veo 的视频生成。",
  "config.video_studio.plan_enabled": "启用规划模式",
  "config.video_studio.plan_agent_provider": "规划提供商",
  "config.video_studio.plan_agent_provider_help": "规划任务使用的 LLM 提供商。主 LLM 使用主路由及其已配置的故障转移。选定的提供商在任务期间固定，不会故障转移。",
  "config.video_studio.plan_agent_provider_main": "主 LLM",
  "config.video_studio.plan_agent_model": "规划模型（可选）",
  "config.video_studio.plan_agent_model_help": "替换规划任务使用的模型。留空表示使用提供商的模型。",
  "config.video_studio.plan_max_daily_jobs": "每天的规划任务数",
  "config.video_studio.plan_max_daily_jobs_help": "新计划和场景修改。范围 1 到 1000。",
  "config.video_studio.plan_job_timeout": "规划时间限制（秒）",
  "config.video_studio.plan_job_timeout_help": "运行更久的规划任务会停止，已规划的场景会保留。范围 120 到 3600。",
```

- [ ] **Step 5: Run them.**
  - `gofmt -w ui/config_video_studio_plan_test.go ui/config_video_studio_browser_test.go`
  - `go test ./ui -run '(?i)(ConfigVideoStudioPlan|TestTranslations_AllKeysPresentInAllLanguages|TestTranslations_ConfigCfgJSKeysExist|TestTranslations_AllJSONFilesParse)' -count=1`. Expected: `ok`.
  - `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestConfigVideoStudioBrowser' -count=1 -timeout 15m`. Expected: `ok`.
  - `npm run check:ui`. Expected: no errors.

- [ ] **Step 6: Commit.**
```
git add ui/cfg/virtual_desktop.js ui/lang/meta.json ui/lang/config/common/*.json ui/config_video_studio_plan_test.go ui/config_video_studio_browser_test.go
git commit -m "feat(video-studio): plan mode settings on the configuration page" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Batch end (after A8, no commit)

Run the existing suites once:
- `go test ./internal/videostudio/storyplan/... -count=1`
- `go test ./internal/server -run '(?i)VideoStudio' -count=1`
- `go test ./internal/videostudio -count=1`
- `go test ./internal/config -count=1`
- `go test ./cmd/config-merger -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudio' -count=1 -timeout 30m`

All must be green. Then run `detect-changes --scope all` (CLAUDE.md) on the branch before the final merge. A result with `partial: true` or `truncated: true` is not a clean check.

## Contract notes

1. **Names added beyond the brief.** All are additive, and parts B–D may use them:
   - storyplan: `Version` (= 1); `ErrPlanTooLarge`, `ErrPlanInvalid`, `ErrPlanVersion` (wrapped by `Decode`, test with `errors.Is`); `ReasonGenerationUnavailable`, `ReasonProviderImagesPublicURLOnly`, `ReasonProviderUnsupported`; `PhaseOutline`, `PhaseDetails`, `PhaseCheck`; `TotalSeconds(p Plan) float64` (Σ use_seconds − blend overlaps, for the bottom bar and the agent data packet).
   - server: `videoStudioGenerationConfigured(cfg)`, which part B can reuse for availability, and `videoStudioProviderProfile(...)`.
   - Part B suggestion: map `ErrPlanTooLarge` to 413 or 400 and the other two to 400.
2. **Normalize scene ids.** Only offending scenes get a new id. An id counts as offending when it is empty, a duplicate, longer than 32 characters, or holds characters other than `[A-Za-z0-9_-]`. The new id is `s<position>`, or the smallest free `s<n>` when that is taken. Scenes are not all renumbered, so the server fields that PUT matches by scene id stay attached. Normalize also lowercases transition kind and type, keyframe source and clip_audio.
3. **`CountWords(text)` has no language parameter.** So CJK counting (letters ÷ 2, rounded up) is decided by script (Han, Hiragana, Katakana), not by the plan's zh/ja language.
4. **Validator extras inside existing codes:**
   - `prompt_too_long` also covers `keyframe.image_prompt` (> profile.MaxPromptChars) and `audio.music.prompt` (> 2000).
   - `character_invalid` covers a missing name, a name over 120 characters, a duplicate name (case-insensitive) and a description over 600 characters.
   - `narration_too_long` uses the spec §8 measured rule when `measured_seconds > 0`: 0.3 s start + measured > use_seconds + 1 s. Slice 1 never measures, so this is dormant.
   - Prompt-length checks are skipped when `MaxPromptChars` is 0 (unsupported profile); `plan_unsupported_provider` blocks then.
   - `Issue.Field` uses 0-based indexes.
   - The `profile_changed` comparison is trimmed and case-insensitive.
5. **Keyframe source follows the transition (controller decision).** Normalize sets keyframe.source to chain for a continue scene and to bridge for a bridge scene, and turns a chain or bridge source on any other scene into none, so the agent and the UI never set chain or bridge themselves. The validator checks a bridge's boundary keyframe K (spec §4.2): a project image in keyframe.asset_id (else `keyframe_asset_invalid`), or an image_prompt with image generation configured (`keyframe_prompt_missing` without a prompt, `keyframe_asset_invalid` when image generation is off and no project image is set). Slice 1's PUT keeps keyframe.asset_id server-owned for source bridge (brief §7), so a user-picked library K for a bridge waits for slice 2.
6. **Profile details:**
   - `MaxPromptChars` 2000 and `Parallel` 2 are set only for supported providers (MiniMax, Google); Agnes and unknown providers get 0.
   - The spec's `resolutions` field is not in the brief's `Profile` and was not added.
   - When generation is not configured, the profile keeps the provider's capabilities and durations and only sets `plan_supported: false` with `generation_unavailable`. This is needed so `videoStudioDurationOptions` keeps today's results: the status shows durations even with generation off.
   - Today's MiniMax quirk is kept: the duration rule reads the configured model name, so a chat-model name such as `MiniMax-M2.7` yields `[6]` although the API model falls back to Hailuo-2.3.
   - The Google default model `veo-3.1-generate-preview` is duplicated as a server constant, because `tools.defaultGoogleVideoModel` is unexported and the brief allows only one exported tools helper.
7. **Admission (A6).** The configured check (untrimmed `APIKey`) stays the admission's own and was not moved to the profile, because it is not a provider-support or image-mode decision. The old "no generation mode" 409 is now `len(profile.DurationsSeconds) == 0`. As before, it is unreachable because the duration check refuses unknown providers first. Manual generation still refuses last frames even where the profile reports `last_frame` (MiniMax-Hailuo-02).
8. **Config additions beyond the brief, found as requirements of a new provider key:**
   - `video_studio.plan.agent_provider` was added to `Config.ProviderReferences()` (role `video_studio_plan_agent`; provider deletion guard) and to `validateDecisionsOnlyProviderReferences`.
   - The configuration page section adds 12 `config.video_studio.plan_*` keys in 16 languages and the `lang/meta.json` provider_ref entry.
   - Normalization clamps out-of-range values as the brief says. Storymaker, by contrast, rejects them.
   - The status handler's `configured` value is now computed by `videoStudioGenerationConfigured(cfg)`, with the same expression as before.
9. **Docs left out.** `documentation/configuration.md` and the manuals' configuration chapters are not updated; that is slice 4.
10. **Plan settings and running renders (controller decision).** `videoStudioConfigRootsChanged` compares `VideoStudio` with `Plan` zeroed on both sides (A7), so saving `video_studio.plan.*` does not cancel running Video Studio FFmpeg jobs or bump the manager epoch; `TestVideoStudioPlanSettingsKeepVideoStudioJobs` pins it.
