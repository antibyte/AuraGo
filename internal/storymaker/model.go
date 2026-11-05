// Package storymaker is the Go core of the Storymaker desktop app: the story document
// (schema storymaker/1), its validation and defaults, the catalog index, code-scene
// bundling and the project storage layout. It has no server dependency.
package storymaker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Schema is the only story schema string this package accepts.
const Schema = "storymaker/1"

// Issue severities.
const (
	SeverityError   = "error"   // blocks render
	SeverityWarning = "warning" // does not block render
	SeverityMinor   = "minor"   // not_supported_yet and canonicalisation notes
)

// Issue codes of phase 1. The desktop app localises them as storymaker.issue.<code>.
const (
	CodeInvalidJSON      = "invalid_json"
	CodeUnknownField     = "unknown_field"
	CodeSchemaMismatch   = "schema_mismatch"
	CodeTooLarge         = "too_large"
	CodeLimitExceeded    = "limit_exceeded"
	CodeMissingField     = "missing_field"
	CodeInvalidValue     = "invalid_value"
	CodeDuplicateID      = "duplicate_id"
	CodeUnknownName      = "unknown_name"
	CodeUnknownAsset     = "unknown_asset"
	CodeUnknownSpot      = "unknown_spot"
	CodeUnknownCast      = "unknown_cast"
	CodeUnknownCue       = "unknown_cue"
	CodeCanonicalised    = "canonicalised"
	CodeNotSupportedYet  = "not_supported_yet"
	CodeNarrationMissing = "narration_missing"
	CodeCodeError        = "code_error"
	CodeDeadline         = "deadline"
	CodeLayoutUnverified = "layout_unverified"
)

// Issue codes of phase 2 that only the player emits. The desktop app localises them as storymaker.issue.<code>.
const (
	CodeCueLate = "cue_late" // warning, player
)

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
type Issue struct {
	Severity  string   `json:"severity"`
	SceneID   string   `json:"scene_id,omitempty"`
	Path      string   `json:"path"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	AtSeconds *float64 `json:"at_seconds,omitempty"`
	Fix       string   `json:"fix,omitempty"`
}

// DefaultApplied records one default that ApplyDefaults filled in.
type DefaultApplied struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// Limits are the story size limits. The zero value of a field means "no limit" for
// DecodeStory and BundleCodeScene; Validate replaces an all-zero Limits by DefaultLimits.
type Limits struct {
	MaxScenes          int
	MaxItemsPerScene   int // actors plus props
	MaxBeatsPerScene   int
	MaxNarrationChars  int // per scene, in characters
	MaxStoryBytes      int
	MaxCodeBytes       int
	MaxDurationSeconds float64
}

// DefaultLimits returns the limits of the design spec.
func DefaultLimits() Limits {
	return Limits{
		MaxScenes:          200,
		MaxItemsPerScene:   40,
		MaxBeatsPerScene:   60,
		MaxNarrationChars:  1500,
		MaxStoryBytes:      1 << 20,
		MaxCodeBytes:       64 << 10,
		MaxDurationSeconds: 15 * 60,
	}
}

// Story is the storymaker/1 document. Field order is the key order of story.json.
type Story struct {
	Schema string       `json:"schema"`
	Title  string       `json:"title"`
	Format Format       `json:"format"`
	Style  Style        `json:"style"`
	Voice  *Voice       `json:"voice"`
	Music  *Music       `json:"music"`
	Cast   []CastMember `json:"cast"`
	Scenes []Scene      `json:"scenes"`
}

// Format is the canvas: aspect 16:9 (1920x1080), 9:16 (1080x1920) or 1:1 (1080x1080).
type Format struct {
	Aspect string `json:"aspect"`
	FPS    int    `json:"fps"`
}

// Style names a catalog palette and font, switches captions and sets the rhythm.
type Style struct {
	Palette  string `json:"palette"`
	Font     string `json:"font"`
	Captions bool   `json:"captions"`
	Rhythm   string `json:"rhythm,omitempty"` // "" | calm | standard | lively; Go never writes a default
}

// UnmarshalJSON defaults captions to true when a style object omits the key.
func (s *Style) UnmarshalJSON(b []byte) error {
	if string(bytes.TrimSpace(b)) == "null" {
		return nil
	}
	type plain Style
	p := plain{Captions: true}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	*s = Style(p)
	return nil
}

// Voice is a narration voice. Phase 1 accepts and validates its shape only.
type Voice struct {
	Provider string `json:"provider"`
	Voice    string `json:"voice"`
	Language string `json:"language"`
}

// Music is the background music setting.
type Music struct {
	Mode   string   `json:"mode"` // none | generated | file
	Mood   string   `json:"mood,omitempty"`
	Prompt string   `json:"prompt,omitempty"`
	File   string   `json:"file,omitempty"`
	Track  string   `json:"track,omitempty"`
	Volume *float64 `json:"volume,omitempty"`
}

// CastMember is a recurring character of the story.
type CastMember struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Character string `json:"character"`
	Voice     *Voice `json:"voice,omitempty"`
}

// Scene is one scene. Kind "code" scenes keep their source in scenes/<id>.js.
type Scene struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind,omitempty"`
	Environment string    `json:"environment,omitempty"`
	Variant     string    `json:"variant,omitempty"`
	Narration   string    `json:"narration,omitempty"`
	Silent      bool      `json:"silent,omitempty"`
	Speaker     string    `json:"speaker,omitempty"`
	Actors      []Actor   `json:"actors,omitempty"`
	Props       []Prop    `json:"props,omitempty"`
	Beats       []Beat    `json:"beats,omitempty"`
	Transition  string    `json:"transition,omitempty"`
	Duration    *Duration `json:"duration,omitempty"`
	Camera      *Camera   `json:"camera,omitempty"`
}

// Camera is the starting framing of a scene.
type Camera struct {
	Framing string `json:"framing"` // wide | medium | close
	Target  string `json:"target,omitempty"`
}

// Actor places a cast member in a scene.
type Actor struct {
	Cast       string    `json:"cast"`
	At         *Position `json:"at,omitempty"`
	Pose       string    `json:"pose,omitempty"`
	Facing     string    `json:"facing,omitempty"` // left | right | auto
	Expression string    `json:"expression,omitempty"`
	Scale      float64   `json:"scale,omitempty"` // 0.5 to 2, 0 = unset (1)
}

// Prop places an item in a scene.
type Prop struct {
	ID    string    `json:"id,omitempty"`
	Item  string    `json:"item"`
	At    *Position `json:"at,omitempty"`
	Layer string    `json:"layer,omitempty"` // back | front
	Scale float64   `json:"scale,omitempty"` // 0.5 to 2, 0 = unset (1)
}

// ChartData is the data of a chart.
type ChartData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

// Beat is one timed action of a storyboard scene. Per-kind required fields are checked by
// Validate. Field order is the key order of story.json.
type Beat struct {
	Do         string     `json:"do"`
	Who        string     `json:"who,omitempty"`
	Target     string     `json:"target,omitempty"`
	From       *Position  `json:"from,omitempty"`
	To         *Position  `json:"to,omitempty"`
	Pose       string     `json:"pose,omitempty"`
	Gesture    string     `json:"gesture,omitempty"`
	Count      int        `json:"count,omitempty"`
	Expression string     `json:"expression,omitempty"`
	Text       string     `json:"text,omitempty"`
	Style      string     `json:"style,omitempty"`
	Item       string     `json:"item,omitempty"`
	Icon       string     `json:"icon,omitempty"`
	Chart      string     `json:"chart,omitempty"`
	Data       *ChartData `json:"data,omitempty"`
	At         *Position  `json:"at,omitempty"`
	Entry      string     `json:"entry,omitempty"`
	ID         string     `json:"id,omitempty"`
	Op         string     `json:"op,omitempty"`
	Amount     float64    `json:"amount,omitempty"`
	Sound      string     `json:"sound,omitempty"`
	Seconds    float64    `json:"seconds,omitempty"`
	With       string     `json:"with,omitempty"` // "prev"
	Cue        int        `json:"cue,omitempty"`
	Delay      float64    `json:"delay,omitempty"`
	Duration   float64    `json:"duration,omitempty"`
	Ease       string     `json:"ease,omitempty"`
}

// Position is either a spot name of the scene's environment or a point {x, y} in percent
// of the safe area (y is the feet line). A value of a different JSON shape is kept as
// Invalid so that Validate can report it at the right path and the story can be saved
// unchanged.
type Position struct {
	Spot    string
	X, Y    float64
	Point   bool
	Invalid json.RawMessage
}

// SpotAt returns a position naming a spot.
func SpotAt(name string) *Position { return &Position{Spot: name} }

// PointAt returns a position at x, y percent.
func PointAt(x, y float64) *Position { return &Position{Point: true, X: x, Y: y} }

// MarshalJSON writes the spot name or {"x":..,"y":..}.
func (p Position) MarshalJSON() ([]byte, error) {
	switch {
	case len(p.Invalid) > 0:
		return p.Invalid, nil
	case p.Point:
		return json.Marshal(struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}{p.X, p.Y})
	default:
		return json.Marshal(p.Spot)
	}
}

// UnmarshalJSON reads a spot name or an {x, y} object.
func (p *Position) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*p = Position{}
	switch {
	case len(b) == 0 || string(b) == "null":
		return nil
	case b[0] == '"':
		if err := json.Unmarshal(b, &p.Spot); err != nil {
			p.Spot = ""
			p.Invalid = compactRaw(b)
		}
	case b[0] == '{':
		var o struct {
			X *float64 `json:"x"`
			Y *float64 `json:"y"`
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&o); err != nil || o.X == nil || o.Y == nil {
			p.Invalid = compactRaw(b)
			return nil
		}
		p.Point, p.X, p.Y = true, *o.X, *o.Y
	default:
		p.Invalid = compactRaw(b)
	}
	return nil
}

// IsZero reports whether the position is unset.
func (p *Position) IsZero() bool {
	return p == nil || (!p.Point && p.Spot == "" && len(p.Invalid) == 0)
}

// String is the spot name, or "x,y" for a point. It is used in messages.
func (p *Position) String() string {
	switch {
	case p == nil:
		return ""
	case len(p.Invalid) > 0:
		return string(p.Invalid)
	case p.Point:
		return fmt.Sprintf("%g,%g", p.X, p.Y)
	}
	return p.Spot
}

// Duration is a scene length: "auto" or a number of seconds.
type Duration struct {
	Auto    bool
	Seconds float64
	Invalid json.RawMessage
}

// AutoDuration is the default scene duration.
func AutoDuration() *Duration { return &Duration{Auto: true} }

// SecondsDuration is an explicit scene duration.
func SecondsDuration(s float64) *Duration { return &Duration{Seconds: s} }

// MarshalJSON writes "auto" or the number.
func (d Duration) MarshalJSON() ([]byte, error) {
	switch {
	case len(d.Invalid) > 0:
		return d.Invalid, nil
	case d.Auto:
		return []byte(`"auto"`), nil
	}
	return json.Marshal(d.Seconds)
}

// UnmarshalJSON reads "auto" or a number.
func (d *Duration) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*d = Duration{}
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if json.Unmarshal(b, &s) == nil && strings.EqualFold(strings.TrimSpace(s), "auto") {
			d.Auto = true
			return nil
		}
		d.Invalid = compactRaw(b)
		return nil
	}
	if err := json.Unmarshal(b, &d.Seconds); err != nil {
		d.Seconds = 0
		d.Invalid = compactRaw(b)
	}
	return nil
}

// CanonicalJSON is the stable story.json form: two-space indentation, struct key order,
// no HTML escaping and a trailing newline. Empty cast and scenes are written as [].
func CanonicalJSON(s Story) ([]byte, error) {
	if s.Cast == nil {
		s.Cast = []CastMember{}
	}
	if s.Scenes == nil {
		s.Scenes = []Scene{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("storymaker: encode story: %w", err)
	}
	return buf.Bytes(), nil
}

// compactRaw removes insignificant whitespace, so an invalid value reads back the same from
// the canonical (indented) file.
func compactRaw(b []byte) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return append(json.RawMessage(nil), b...)
	}
	return buf.Bytes()
}

// validSlug reports whether s matches ^[a-z0-9][a-z0-9-]{0,47}$.
func validSlug(s string) bool {
	if s == "" || len(s) > 48 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 {
			continue
		}
		return false
	}
	return true
}

// Closed name sets of the story format. The validator knows all of them from phase 1; the
// catalog provides a subset until the catalog is complete.
var (
	PoseNames       = []string{"stand", "sit", "typing", "point-left", "point-right", "wave", "arms-crossed", "hands-on-hips", "shrug", "think", "present", "thumbs-up", "celebrate", "facepalm", "phone", "hold", "lean"}
	GestureNames    = []string{"point", "wave", "shrug", "think", "present", "thumbs-up", "celebrate", "facepalm", "count"}
	ExpressionNames = []string{"neutral", "happy", "laugh", "sad", "angry", "surprised", "confused", "thinking", "worried", "determined"}
	MouthNames      = []string{"rest", "ai", "e", "o", "u", "mbp", "fv"}
	TransitionNames = []string{"cut", "fade", "slide-left", "slide-right", "slide-up", "slide-down", "wipe", "zoom", "iris", "whip"}
	EaseNames       = []string{"linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out"}
	BeatKinds       = []string{"enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "say", "show", "hide", "highlight", "camera", "chart", "sfx", "wait"}
)

// Phase 1 plays these beats (show only with item or text) and transitions; everything else
// is accepted, ignored by the player and reported as a minor not_supported_yet issue.
var (
	phase1Beats       = nameSet("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "highlight", "camera", "wait")
	phase1Transitions = nameSet("cut", "fade", "slide-left", "wipe")
)

func nameSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func containsName(list []string, s string) bool {
	for _, n := range list {
		if n == s {
			return true
		}
	}
	return false
}
