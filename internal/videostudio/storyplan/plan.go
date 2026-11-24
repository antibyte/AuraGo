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
