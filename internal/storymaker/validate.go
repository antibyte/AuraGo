package storymaker

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	assetIDPattern = regexp.MustCompile(`^(?:(?:char|prop|env|sfx)\.[a-z0-9][a-z0-9.-]{0,63}|(?:my|lib)\.(?:char|prop|bg)\.[a-z0-9][a-z0-9-]{0,63})$`)
	voiceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	cuePattern     = regexp.MustCompile(`\{(\d{1,3})\}`)
)

// voiceProviders are the provider ids a saved voice may name, sorted. They are the TTS dispatcher
// ids of internal/tools/tts.go synthesizeProvider; the dispatcher also accepts the aliases s2s and
// s2s-lab for speech_lab, but a story saves only the canonical id. This package does not import
// internal/tools, so the list is kept here and must follow the dispatcher.
var voiceProviders = []string{
	"elevenlabs", "google", "minimax", "mistral", "openai", "phonon", "piper",
	"resemble", "sanotts", "speech_lab", "stepfun", "supertonic", "xai",
}

// assetRole is the kind of asset a field takes.
type assetRole struct {
	kind     string   // catalog kind
	label    string   // for messages
	prefixes []string // accepted ID prefixes (built in, project, library)
}

var (
	roleCharacter = assetRole{"character", "character", []string{"char.", "my.char.", "lib.char."}}
	roleProp      = assetRole{"prop", "prop", []string{"prop.", "my.prop.", "lib.prop."}}
	roleEnv       = assetRole{"environment", "environment", []string{"env.", "my.bg.", "lib.bg."}}
	roleSound     = assetRole{"sfx", "sound", []string{"sfx."}}
)

// nameSpec describes an enum-like field that takes a bare name.
type nameSpec struct {
	field     string          // JSON key, for examples
	kind      string          // catalog kind, "" when the catalog has no entries for it
	known     []string        // closed set the validator knows, nil when only the catalog decides
	prefixes  []string        // ID prefixes that canonicalisation strips
	supported map[string]bool // static phase-1 support; nil means the catalog decides
}

var (
	specPose       = nameSpec{field: "pose", kind: "pose", known: PoseNames, prefixes: []string{"pose."}}
	specGesture    = nameSpec{field: "gesture", kind: "gesture", known: GestureNames, prefixes: []string{"gesture."}}
	specExpression = nameSpec{field: "expression", kind: "expression", known: ExpressionNames, prefixes: []string{"expr.", "expression."}}
	specTransition = nameSpec{field: "transition", kind: "transition", known: TransitionNames, prefixes: []string{"trans.", "transition."}, supported: phase1Transitions}
	specEase       = nameSpec{field: "ease", known: EaseNames, prefixes: []string{"ease."}}
	specPalette    = nameSpec{field: "palette", kind: "palette", prefixes: []string{"palette."}}
	specTextStyle  = nameSpec{field: "style", kind: "text_style", prefixes: []string{"text."}}
)

var beatExamples = map[string]string{
	"enter":     `{"do":"enter","who":"anna","from":"left","to":"desk"}`,
	"exit":      `{"do":"exit","who":"anna","to":"door"}`,
	"walk":      `{"do":"walk","who":"anna","to":"door"}`,
	"move":      `{"do":"move","who":"anna","to":"door"}`,
	"pose":      `{"do":"pose","who":"anna","pose":"typing"}`,
	"gesture":   `{"do":"gesture","who":"anna","gesture":"point","count":2}`,
	"emote":     `{"do":"emote","who":"anna","expression":"surprised"}`,
	"look":      `{"do":"look","who":"anna","target":"camera"}`,
	"say":       `{"do":"say","who":"anna","text":"Das ist ein Paket.","style":"speech"}`,
	"show":      `{"do":"show","item":"prop.lock","at":"right","entry":"pop"}`,
	"hide":      `{"do":"hide","target":"prop.lock"}`,
	"highlight": `{"do":"highlight","target":"prop.lock","style":"pulse"}`,
	"camera":    `{"do":"camera","op":"focus","target":"anna"}`,
	"chart":     `{"do":"chart","chart":"bar","data":{"labels":["Mo","Di"],"values":[2,5]},"at":"right"}`,
	"sfx":       `{"do":"sfx","sound":"sfx.pop"}`,
	"wait":      `{"do":"wait","seconds":1}`,
}

type validator struct {
	s       *Story
	cat     *Catalog
	lim     Limits
	out     []Issue
	cast    map[string]int
	castIDs []string
	sceneID string
}

// Validate checks a story against the format, the limits and (when cat is not nil) the
// catalog. It returns issues in document order: errors block a render, warnings do not, and
// minor issues are notes (not_supported_yet, canonicalised). It canonicalises prefixed names in
// place (pose.typing becomes typing) and reports each change. A nil catalog skips the checks
// that need it (asset ids, spots, catalog names). Validate does not fill defaults; it
// understands a story with or without ApplyDefaults.
func Validate(s *Story, cat *Catalog, lim Limits) []Issue {
	if s == nil {
		return []Issue{{Severity: SeverityError, Path: "", Code: CodeMissingField, Message: "There is no story."}}
	}
	if lim == (Limits{}) {
		lim = DefaultLimits()
	}
	v := &validator{s: s, cat: cat, lim: lim, cast: map[string]int{}}
	v.story()
	return v.out
}

func (v *validator) add(sev, code, path, msg, fix string) {
	v.out = append(v.out, Issue{Severity: sev, SceneID: v.sceneID, Path: path, Code: code, Message: msg, Fix: fix})
}
func (v *validator) errorf(code, path, msg, fix string) { v.add(SeverityError, code, path, msg, fix) }
func (v *validator) warnf(code, path, msg, fix string)  { v.add(SeverityWarning, code, path, msg, fix) }
func (v *validator) minorf(code, path, msg, fix string) { v.add(SeverityMinor, code, path, msg, fix) }
func (v *validator) notYet(path, msg string)            { v.minorf(CodeNotSupportedYet, path, msg, "") }

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func listOf(names []string) string { return strings.Join(names, ", ") }

func (v *validator) story() {
	s := v.s
	if s.Schema != Schema {
		v.errorf(CodeSchemaMismatch, "/schema", fmt.Sprintf("The schema must be %q.", Schema), `Set "schema": "storymaker/1".`)
	}
	switch title := strings.TrimSpace(s.Title); {
	case title == "":
		v.errorf(CodeMissingField, "/title", "The story needs a title.", `Add "title": "Was ist eine Firewall?".`)
	case runeLen(title) > 120:
		v.errorf(CodeInvalidValue, "/title", fmt.Sprintf("The title has %d characters; the limit is 120.", runeLen(title)), "Shorten the title.")
	}
	v.format()
	v.style()
	v.checkVoice("/voice", s.Voice)
	v.music()
	v.castList()
	v.scenes()
}

func (v *validator) format() {
	f := v.s.Format
	switch f.Aspect {
	case "16:9", "9:16", "1:1":
	case "":
		v.errorf(CodeMissingField, "/format/aspect", "The format needs an aspect.", `Add "format": {"aspect":"16:9","fps":30}.`)
	default:
		v.errorf(CodeInvalidValue, "/format/aspect", fmt.Sprintf("The aspect %q is not supported.", f.Aspect), `Use "16:9", "9:16" or "1:1": {"aspect":"16:9"}.`)
	}
	switch f.FPS {
	case 24, 25, 30:
	case 0:
		v.errorf(CodeMissingField, "/format/fps", "The format needs fps.", `Add "fps": 30.`)
	default:
		v.errorf(CodeInvalidValue, "/format/fps", fmt.Sprintf("%d fps is not supported.", f.FPS), `Use 24, 25 or 30: {"fps":30}.`)
	}
}

func (v *validator) style() {
	st := &v.s.Style
	if st.Palette == "" {
		v.errorf(CodeMissingField, "/style/palette", "The style needs a palette.", `Add "style": {"palette":"sunny","font":"default","captions":true}.`)
	} else {
		v.bareName("/style/palette", &st.Palette, specPalette)
	}
	if st.Font == "" {
		v.errorf(CodeMissingField, "/style/font", "The style needs a font.", `Add "font": "default".`)
	} else if v.cat != nil && st.Font != "default" && !containsName(v.cat.Names("font"), st.Font) {
		alts := closestNames(st.Font, append([]string{"default"}, v.cat.Names("font")...), 5)
		v.errorf(CodeUnknownName, "/style/font", fmt.Sprintf("The font %q is not in the catalog.", st.Font),
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("font", alts[0]))
	}
	switch st.Rhythm {
	case "", "calm", "standard", "lively":
	default:
		v.errorf(CodeInvalidValue, "/style/rhythm", fmt.Sprintf("The rhythm %q is not supported.", clip(st.Rhythm, 20)), `Use "calm", "standard" or "lively": {"rhythm":"calm"}.`)
	}
}

func (v *validator) checkVoice(path string, voice *Voice) {
	if voice == nil {
		return
	}
	switch p := strings.ToLower(strings.TrimSpace(voice.Provider)); {
	case p == "":
		v.errorf(CodeMissingField, path+"/provider", "The voice needs a provider.", `Add "provider": "piper".`)
	case p == "active":
		v.errorf(CodeInvalidValue, path+"/provider", `The voice provider "active" is not saved; a saved story names a concrete provider.`,
			"Use one of: "+listOf(voiceProviders)+". Example: "+exampleJSON("provider", "piper"))
	case !containsName(voiceProviders, p):
		alts := closestNames(p, voiceProviders, 5)
		v.errorf(CodeUnknownName, path+"/provider", fmt.Sprintf("The voice provider %q is not known.", clip(strings.TrimSpace(voice.Provider), 40)),
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("provider", alts[0]))
	}
	if strings.TrimSpace(voice.Language) == "" {
		v.errorf(CodeMissingField, path+"/language", "The voice needs a language.", `Add "language": "de".`)
	}
	if voice.Voice != "" && !voiceIDPattern.MatchString(voice.Voice) {
		v.errorf(CodeInvalidValue, path+"/voice", fmt.Sprintf("The voice id %q is not valid.", clip(voice.Voice, 40)),
			"A voice id has 1 to 128 letters, digits, dots, dashes or underscores, for example de_DE-thorsten-high.")
	}
}

func clip(s string, n int) string {
	if runeLen(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

func (v *validator) music() {
	m := v.s.Music
	if m == nil {
		return
	}
	switch m.Mode {
	case "none", "generated", "file":
	case "":
		v.errorf(CodeMissingField, "/music/mode", "The music needs a mode.", `Use "mode": "none", "generated" or "file".`)
		return
	default:
		v.errorf(CodeInvalidValue, "/music/mode", fmt.Sprintf("The music mode %q is not supported.", m.Mode), `Use "none", "generated" or "file": {"mode":"none"}.`)
		return
	}
	if m.Volume != nil && (*m.Volume < 0 || *m.Volume > 1 || math.IsNaN(*m.Volume)) {
		v.errorf(CodeInvalidValue, "/music/volume", "The music volume must be between 0 and 1.", `Use "volume": 0.25.`)
	}
	if m.Mode == "file" {
		switch {
		case m.File == "" && m.Track == "":
			v.errorf(CodeMissingField, "/music/file", `The music mode "file" needs a file or a track.`, `Add "file": "Documents/Music/theme.mp3".`)
		case m.File != "":
			if why := badDesktopPath(m.File); why != "" {
				v.errorf(CodeInvalidValue, "/music/file", fmt.Sprintf("The music file %q is not a desktop path: %s.", clip(m.File, 60), why),
					`Use a rooted relative path inside the desktop, for example "Documents/Music/theme.mp3".`)
			}
		}
	}
	if m.Mode != "none" {
		v.notYet("/music", "Background music is accepted but not played yet.")
	}
}

// badDesktopPath explains why p is not a rooted relative desktop path ("" when it is fine).
func badDesktopPath(p string) string {
	switch {
	case len(p) > 260:
		return "it is longer than 260 characters"
	case strings.ContainsAny(p, "\x00\r\n"):
		return "it contains control characters"
	case strings.Contains(p, "://") || strings.Contains(p, ":"):
		return "it contains a scheme or drive (colon)"
	case strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\"):
		return "it starts with a slash"
	case strings.Contains(p, "\\"):
		return "it contains a backslash"
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "":
			return "it has an empty segment"
		case ".", "..":
			return "it contains . or .."
		}
	}
	return ""
}

func (v *validator) castList() {
	for i, c := range v.s.Cast {
		base := jp("cast", i)
		switch {
		case c.ID == "":
			v.errorf(CodeMissingField, base+"/id", "A cast member needs an id.", `Add "id": "anna".`)
		case !validSlug(c.ID):
			v.errorf(CodeInvalidValue, base+"/id", fmt.Sprintf("The cast id %q is not a slug.", clip(c.ID, 40)),
				"Use lowercase letters, digits and dashes (at most 48), for example anna.")
		default:
			if first, dup := v.cast[c.ID]; dup {
				v.errorf(CodeDuplicateID, base+"/id", fmt.Sprintf("The cast id %q is used twice (also at cast[%d]).", c.ID, first), "Give each cast member its own id.")
			} else {
				v.cast[c.ID] = i
				v.castIDs = append(v.castIDs, c.ID)
			}
		}
		if c.Character == "" {
			v.errorf(CodeMissingField, base+"/character", "A cast member needs a character.", `Add "character": "char.office-woman-1".`)
		} else {
			v.checkAsset(base+"/character", "character", c.Character, roleCharacter)
		}
		if runeLen(c.Name) > 80 {
			v.errorf(CodeInvalidValue, base+"/name", "The cast name is longer than 80 characters.", "Shorten the name.")
		}
		v.checkVoice(base+"/voice", c.Voice)
	}
}

func (v *validator) scenes() {
	s := v.s
	if len(s.Scenes) == 0 {
		v.errorf(CodeMissingField, "/scenes", "The story needs at least one scene.", `Add "scenes": [{"id":"s1","narration":"..."}].`)
		return
	}
	if len(s.Scenes) > v.lim.MaxScenes {
		v.errorf(CodeLimitExceeded, "/scenes", fmt.Sprintf("The story has %d scenes; the limit is %d.", len(s.Scenes), v.lim.MaxScenes), "Remove scenes or split the story into several videos.")
	}
	seen := map[string]int{}
	prevEnv := ""
	explicit := 0.0
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		v.sceneID = sc.ID
		env := sc.Environment
		if env == "" {
			env = prevEnv
		}
		if env == "" {
			env = DefaultEnvironment
		}
		prevEnv = env
		v.scene(i, sc, env, seen)
		if d := sc.Duration; d != nil && !d.Auto && len(d.Invalid) == 0 && d.Seconds > 0 {
			explicit += d.Seconds
		}
	}
	v.sceneID = ""
	if v.lim.MaxDurationSeconds > 0 && explicit > v.lim.MaxDurationSeconds {
		v.errorf(CodeLimitExceeded, "/scenes", fmt.Sprintf("The explicit scene durations add up to %.0f seconds; the limit is %.0f seconds.", explicit, v.lim.MaxDurationSeconds),
			"Shorten scene durations or remove scenes.")
	}
}

// sceneRefs are the things a beat can point at in one scene.
type sceneRefs struct {
	instances  map[string]bool // prop ids and show-beat ids (explicit or derived)
	items      map[string][]string
	spots      map[string]Spot
	spotsKnown bool
	markers    map[int]bool
}

func (v *validator) scene(i int, sc *Scene, env string, seen map[string]int) {
	base := jp("scenes", i)
	switch {
	case sc.ID == "":
		v.errorf(CodeMissingField, base+"/id", "A scene needs an id.", `Add "id": "s1".`)
	case !validSlug(sc.ID):
		v.errorf(CodeInvalidValue, base+"/id", fmt.Sprintf("The scene id %q is not a slug.", clip(sc.ID, 40)),
			"Use lowercase letters, digits and dashes (at most 48), for example s1 or intro.")
	default:
		if first, dup := seen[sc.ID]; dup {
			v.errorf(CodeDuplicateID, base+"/id", fmt.Sprintf("The scene id %q is used twice (also at scenes[%d]).", sc.ID, first), "Give each scene its own id.")
		} else {
			seen[sc.ID] = i
		}
	}
	switch sc.Kind {
	case "", "storyboard", "code":
	default:
		v.errorf(CodeInvalidValue, base+"/kind", fmt.Sprintf("The scene kind %q is not supported.", sc.Kind), `Use "storyboard" (the default) or "code".`)
	}

	if sc.Environment != "" {
		v.checkAsset(base+"/environment", "environment", sc.Environment, roleEnv)
	}
	switch sc.Variant {
	case "", "day", "night":
		if sc.Variant != "" && v.cat != nil {
			if vs := v.cat.Variants(env); len(vs) > 0 && !containsName(vs, sc.Variant) {
				v.warnf(CodeInvalidValue, base+"/variant", fmt.Sprintf("The environment %s has no %s variant.", env, sc.Variant), "Use one of: "+listOf(vs)+".")
			}
		}
	default:
		v.errorf(CodeInvalidValue, base+"/variant", fmt.Sprintf("The variant %q is not supported.", sc.Variant), `Use "day" or "night": {"variant":"night"}.`)
	}

	refs := v.refsOf(sc, env)
	v.narration(base, sc, refs)
	v.speaker(base, sc)
	if sc.Transition != "" {
		v.bareName(base+"/transition", &sc.Transition, specTransition)
	}
	v.duration(base, sc)
	if c := sc.Camera; c != nil {
		switch c.Framing {
		case "", "wide", "medium", "close":
		default:
			v.errorf(CodeInvalidValue, base+"/camera/framing", fmt.Sprintf("The camera framing %q is not supported.", c.Framing), `Use "wide", "medium" or "close": {"framing":"wide"}.`)
		}
	}

	if items := len(sc.Actors) + len(sc.Props); items > v.lim.MaxItemsPerScene {
		v.errorf(CodeLimitExceeded, base+"/actors", fmt.Sprintf("The scene has %d actors and props; the limit is %d.", items, v.lim.MaxItemsPerScene), "Remove props or split the scene.")
	}
	if len(sc.Beats) > v.lim.MaxBeatsPerScene {
		v.errorf(CodeLimitExceeded, base+"/beats", fmt.Sprintf("The scene has %d beats; the limit is %d.", len(sc.Beats), v.lim.MaxBeatsPerScene), "Split the scene into two scenes.")
	}
	if sc.Kind == "code" && (len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0) {
		v.warnf(CodeInvalidValue, base+"/kind", "A code scene draws everything in its code; its actors, props and beats are ignored.",
			`Remove actors, props and beats, or set "kind" to "storyboard".`)
	}

	if sc.Kind != "code" {
		v.actors(i, sc, env)
		v.props(i, sc, env)
		v.beats(i, sc, env, refs)
	}
}

// refsOf collects the instance ids, items, spots and cue markers of a scene.
func (v *validator) refsOf(sc *Scene, env string) *sceneRefs {
	r := &sceneRefs{instances: map[string]bool{}, items: map[string][]string{}, markers: map[int]bool{}}
	for _, p := range sc.Props {
		if p.ID != "" {
			r.instances[p.ID] = true
		}
		if p.Item != "" {
			r.items[p.Item] = append(r.items[p.Item], p.ID)
		}
	}
	derived := deriveShowIDs(sc)
	for j, b := range sc.Beats {
		if b.Do != "show" {
			continue
		}
		id := b.ID
		if id == "" {
			id = derived[j]
		}
		if id != "" {
			r.instances[id] = true
		}
		if b.Item != "" {
			r.items[b.Item] = append(r.items[b.Item], id)
		}
	}
	if v.cat != nil {
		r.spots, r.spotsKnown = v.cat.Spots(env, v.s.Format.Aspect)
	}
	for _, m := range cuePattern.FindAllStringSubmatch(sc.Narration, -1) {
		n, _ := strconv.Atoi(m[1])
		r.markers[n] = true
	}
	return r
}

func (v *validator) narration(base string, sc *Scene, _ *sceneRefs) {
	if n := runeLen(sc.Narration); n > v.lim.MaxNarrationChars {
		v.errorf(CodeLimitExceeded, base+"/narration", fmt.Sprintf("The narration has %d characters; the limit is %d.", n, v.lim.MaxNarrationChars),
			"Shorten the narration or split the scene.")
	}
}

func (v *validator) speaker(base string, sc *Scene) {
	switch sc.Speaker {
	case "", "narrator":
	default:
		if _, ok := v.cast[sc.Speaker]; !ok {
			v.errorf(CodeUnknownCast, base+"/speaker", fmt.Sprintf("The speaker %q is not in the cast.", clip(sc.Speaker, 40)),
				v.castFix("speaker", sc.Speaker))
			return
		}
		v.notYet(base+"/speaker", "A cast speaker (lip-sync and speech bubble) is accepted but not shown yet; the scene plays like a narrator scene.")
	}
}

// castFix suggests the cast ids closest to a wrong one.
func (v *validator) castFix(field, got string) string {
	if len(v.castIDs) == 0 {
		return `The story has no cast yet. Add "cast": [{"id":"anna","character":"char.office-woman-1"}].`
	}
	alts := closestNames(got, v.castIDs, 5)
	return "Use a cast id: " + listOf(alts) + ". Example: " + exampleJSON(field, alts[0])
}

func (v *validator) duration(base string, sc *Scene) {
	d := sc.Duration
	if d == nil || d.Auto {
		return
	}
	switch {
	case len(d.Invalid) > 0:
		v.errorf(CodeInvalidValue, base+"/duration", "The duration must be \"auto\" or a number of seconds.", `Use "duration": "auto" or "duration": 6.`)
	case d.Seconds <= 0 || math.IsNaN(d.Seconds):
		v.errorf(CodeInvalidValue, base+"/duration", "The duration must be greater than 0 seconds.", `Use "duration": 6 or "duration": "auto".`)
	case v.lim.MaxDurationSeconds > 0 && d.Seconds > v.lim.MaxDurationSeconds:
		v.errorf(CodeLimitExceeded, base+"/duration", fmt.Sprintf("The duration %.0f s exceeds the limit of %.0f s.", d.Seconds, v.lim.MaxDurationSeconds), "Use a shorter duration.")
	}
}

func (v *validator) actors(si int, sc *Scene, env string) {
	seen := map[string]int{}
	for j := range sc.Actors {
		a := &sc.Actors[j]
		ab := jp("scenes", si, "actors", j)
		switch {
		case a.Cast == "":
			v.errorf(CodeMissingField, ab+"/cast", "An actor needs a cast id.", `Add "cast": "anna" (an id of the story's cast).`)
		default:
			if _, ok := v.cast[a.Cast]; !ok {
				v.errorf(CodeUnknownCast, ab+"/cast", fmt.Sprintf("The cast id %q is not in the story's cast.", clip(a.Cast, 40)), v.castFix("cast", a.Cast))
			}
			if first, dup := seen[a.Cast]; dup {
				v.errorf(CodeDuplicateID, ab+"/cast", fmt.Sprintf("%q is on stage twice (also actors[%d]).", a.Cast, first), "A cast member appears once per scene; move it with beats.")
			} else {
				seen[a.Cast] = j
			}
		}
		v.position(ab+"/at", a.At, env)
		if a.Pose != "" {
			v.bareName(ab+"/pose", &a.Pose, specPose)
		}
		switch a.Facing {
		case "", "left", "right", "auto":
		default:
			v.errorf(CodeInvalidValue, ab+"/facing", fmt.Sprintf("The facing %q is not supported.", a.Facing), `Use "left", "right" or "auto": {"facing":"auto"}.`)
		}
		if a.Expression != "" {
			v.bareName(ab+"/expression", &a.Expression, specExpression)
		}
		v.scale(ab+"/scale", a.Scale)
	}
}

func (v *validator) scale(path string, scale float64) {
	if scale != 0 && (scale < 0.5 || scale > 2 || math.IsNaN(scale)) {
		v.errorf(CodeInvalidValue, path, fmt.Sprintf("The scale %g is outside 0.5 to 2.", scale), `Use "scale": 1.`)
	}
}

func (v *validator) props(si int, sc *Scene, env string) {
	seen := map[string]bool{}
	for j := range sc.Props {
		p := &sc.Props[j]
		pb := jp("scenes", si, "props", j)
		if p.ID != "" {
			switch {
			case !validSlug(p.ID):
				v.errorf(CodeInvalidValue, pb+"/id", fmt.Sprintf("The instance id %q is not a slug.", clip(p.ID, 40)), "Use lowercase letters, digits and dashes, for example lock-1.")
			case seen[p.ID]:
				v.errorf(CodeDuplicateID, pb+"/id", fmt.Sprintf("The instance id %q is used twice.", p.ID), "Give each prop its own id.")
			default:
				seen[p.ID] = true
			}
		}
		if p.Item == "" {
			v.errorf(CodeMissingField, pb+"/item", "A prop needs an item.", `Add "item": "prop.lock".`)
		} else {
			v.checkAsset(pb+"/item", "item", p.Item, roleProp)
		}
		v.position(pb+"/at", p.At, env)
		switch p.Layer {
		case "", "back", "front":
		default:
			v.errorf(CodeInvalidValue, pb+"/layer", fmt.Sprintf("The layer %q is not supported.", p.Layer), `Use "back" or "front": {"layer":"front"}.`)
		}
		v.scale(pb+"/scale", p.Scale)
	}
}

// bareName validates an enum-like field. It strips an ID prefix with a canonicalised note,
// accepts catalog names, reports known names the catalog lacks as not_supported_yet and unknown
// names as unknown_name with alternatives.
func (v *validator) bareName(path string, val *string, spec nameSpec) {
	name := *val
	if name == "" {
		return
	}
	for _, p := range spec.prefixes {
		if strings.HasPrefix(name, p) && len(name) > len(p) {
			canon := name[len(p):]
			v.minorf(CodeCanonicalised, path, fmt.Sprintf("%q is a catalog id; the field %s takes the bare name, so %q was used.", name, spec.field, canon),
				"Write "+exampleJSON(spec.field, canon)+" next time.")
			*val, name = canon, canon
			break
		}
	}
	catalogBacked := spec.kind != ""
	known := containsName(spec.known, name)
	switch {
	case v.cat != nil && catalogBacked && v.cat.HasName(spec.kind, name):
		return
	case spec.supported != nil && known:
		if !spec.supported[name] {
			v.notYet(path, fmt.Sprintf("The %s %q is accepted but not played yet.", spec.field, name))
		}
		return
	case known && (!catalogBacked || v.cat == nil):
		return
	case known:
		v.notYet(path, fmt.Sprintf("The %s %q is not in the catalog yet; %s.", spec.field, name, fallbackText(spec.field)))
		return
	case spec.known == nil && v.cat == nil:
		return // only the catalog could judge this name, and there is none
	}
	pool := spec.known
	if v.cat != nil && catalogBacked {
		if names := v.cat.Names(spec.kind); len(names) > 0 {
			pool = names
		}
	}
	alts := closestNames(name, pool, 5)
	fix := "No names are available for this field."
	if len(alts) > 0 {
		fix = "Use one of: " + listOf(alts) + ". Example: " + exampleJSON(spec.field, alts[0])
	}
	v.errorf(CodeUnknownName, path, fmt.Sprintf("The %s %q is not known.", spec.field, clip(name, 40)), fix)
}

func fallbackText(field string) string {
	switch field {
	case "pose":
		return "the actor keeps the stand pose"
	case "expression":
		return "the actor keeps the neutral face"
	case "gesture":
		return "no gesture is played"
	}
	return "the player uses a fallback"
}

// checkAsset validates an asset id field.
func (v *validator) checkAsset(path, field, id string, role assetRole) {
	if !assetIDPattern.MatchString(id) {
		v.errorf(CodeUnknownAsset, path, fmt.Sprintf("%q is not an asset id.", clip(id, 60)), v.assetFix(field, id, role, "Use the full prefixed id."))
		return
	}
	prefixOK := false
	for _, p := range role.prefixes {
		if strings.HasPrefix(id, p) {
			prefixOK = true
			break
		}
	}
	if !prefixOK {
		v.errorf(CodeInvalidValue, path, fmt.Sprintf("%q is not a %s; this field takes a %s id.", id, wrongKindLabel(id), role.label), v.assetFix(field, id, role, ""))
		return
	}
	if strings.HasPrefix(id, "my.") || strings.HasPrefix(id, "lib.") {
		return // project and library assets are resolved when the project is opened
	}
	if v.cat == nil {
		return
	}
	if role.kind == "sfx" && len(v.cat.Names("sfx")) == 0 {
		return // the sound catalog arrives with the sound phase
	}
	if e, ok := v.cat.Lookup(id); ok && e.Kind == role.kind {
		return
	}
	v.errorf(CodeUnknownAsset, path, fmt.Sprintf("The %s %q is not in the catalog.", role.label, id), v.assetFix(field, id, role, ""))
}

func wrongKindLabel(id string) string {
	switch {
	case strings.HasPrefix(id, "char."), strings.HasPrefix(id, "my.char."), strings.HasPrefix(id, "lib.char."):
		return "character"
	case strings.HasPrefix(id, "prop."), strings.HasPrefix(id, "my.prop."), strings.HasPrefix(id, "lib.prop."):
		return "prop"
	case strings.HasPrefix(id, "env."), strings.HasPrefix(id, "my.bg."), strings.HasPrefix(id, "lib.bg."):
		return "environment"
	case strings.HasPrefix(id, "sfx."):
		return "sound"
	}
	return "different kind of asset"
}

// assetFix names the catalog assets closest to a wrong id.
func (v *validator) assetFix(field, id string, role assetRole, lead string) string {
	alts := v.assetAlternatives(id, role.kind)
	if len(alts) == 0 {
		return strings.TrimSpace(lead + " Search the catalog for a " + role.label + ".")
	}
	return strings.TrimSpace(lead + " Use one of: " + listOf(alts) + ". Example: " + exampleJSON(field, alts[0]))
}

// assetAlternatives returns at most five catalog ids of a kind that resemble id.
func (v *validator) assetAlternatives(id, kind string) []string {
	if v.cat == nil {
		return nil
	}
	suffix := id
	if parts := strings.Split(id, "."); len(parts) > 1 {
		suffix = strings.Join(parts[1:], ".")
		if (parts[0] == "my" || parts[0] == "lib") && len(parts) > 2 {
			suffix = strings.Join(parts[2:], ".")
		}
	}
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] && len(out) < 5 {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, t := range splitTerms(suffix) {
		for _, e := range v.cat.Search(t, kind, 5) {
			add(e.ID)
		}
	}
	for _, s := range closestNames(id, v.cat.Names(kind), 5) {
		add(s)
	}
	return out
}

// position validates a spot or point against the environment.
func (v *validator) position(path string, p *Position, env string) {
	if p == nil {
		return
	}
	switch {
	case len(p.Invalid) > 0:
		v.errorf(CodeInvalidValue, path, "A position is a spot name or an {x, y} object.", `Use "at": "center" or "at": {"x":50,"y":88}.`)
	case p.Point:
		if p.X < 0 || p.X > 100 || p.Y < 0 || p.Y > 100 || math.IsNaN(p.X) || math.IsNaN(p.Y) {
			v.errorf(CodeInvalidValue, path, "A point's x and y are percentages between 0 and 100.", `Use {"x":50,"y":88}.`)
		}
	case p.Spot == "":
		v.errorf(CodeInvalidValue, path, "A position needs a spot name or an {x, y} point.", `Use "center" or {"x":50,"y":88}.`)
	case v.cat != nil:
		spots, ok := v.cat.Spots(env, v.s.Format.Aspect)
		if !ok {
			return // the environment itself is unknown and already reported
		}
		if _, found := spots[p.Spot]; !found {
			names := v.cat.SpotNames(env)
			alts := closestNames(p.Spot, names, 5)
			fix := "Use a spot of " + env + ": " + listOf(names) + "."
			if len(alts) > 0 {
				fix += " Example: " + exampleJSON("at", alts[0])
			}
			v.errorf(CodeUnknownSpot, path, fmt.Sprintf("The spot %q does not exist in %s.", clip(p.Spot, 40), env), fix)
		}
	}
}

func (v *validator) beats(si int, sc *Scene, env string, refs *sceneRefs) {
	stage := map[string]bool{}
	for _, a := range sc.Actors {
		stage[a.Cast] = true
	}
	explicit := map[string]bool{}
	for _, p := range sc.Props {
		if p.ID != "" {
			explicit[p.ID] = true
		}
	}
	for j := range sc.Beats {
		b := &sc.Beats[j]
		bb := jp("scenes", si, "beats", j)
		v.beat(bb, j, b, sc, env, refs, stage, explicit)
	}
}

func (v *validator) missing(bb string, b *Beat, field string, what string) {
	v.errorf(CodeMissingField, bb+"/"+field, fmt.Sprintf("A %q beat needs %q%s.", b.Do, field, what),
		fmt.Sprintf("Add %q. Example: %s", field, beatExamples[b.Do]))
}

func (v *validator) beat(bb string, j int, b *Beat, sc *Scene, env string, refs *sceneRefs, stage, explicit map[string]bool) {
	if b.Do == "" {
		v.errorf(CodeMissingField, bb+"/do", "A beat needs a kind in \"do\".", `Add "do": for example {"do":"walk","who":"anna","to":"center"}.`)
		return
	}
	if !containsName(BeatKinds, b.Do) {
		alts := closestNames(b.Do, BeatKinds, 5)
		v.errorf(CodeUnknownName, bb+"/do", fmt.Sprintf("The beat kind %q is not known.", clip(b.Do, 40)),
			"Use one of: "+listOf(alts)+". Example: "+beatExamples[alts[0]])
		return
	}
	if !phase1Beats[b.Do] {
		v.notYet(bb, fmt.Sprintf("The %q beat is accepted but not played yet; the player skips it.", b.Do))
	}

	// common fields
	switch b.With {
	case "":
	case "prev":
		if j == 0 {
			v.warnf(CodeInvalidValue, bb+"/with", `The first beat has no previous beat to run with.`, `Remove "with" from the first beat.`)
		}
	default:
		v.errorf(CodeInvalidValue, bb+"/with", fmt.Sprintf("\"with\" must be \"prev\", not %q.", clip(b.With, 20)), `Use {"with":"prev"} to start together with the previous beat.`)
	}
	if b.Cue != 0 {
		switch {
		case sc.Silent:
			v.warnf(CodeUnknownCue, bb+"/cue", "The scene is silent, so its narration markers are ignored and this beat never starts.",
				`Remove "cue" or clear "silent".`)
		case b.Cue < 0:
			v.errorf(CodeInvalidValue, bb+"/cue", "A cue is a narration marker number, 1 or more.", `Use "cue": 1 and put {1} into the narration.`)
		case !refs.markers[b.Cue]:
			v.errorf(CodeUnknownCue, bb+"/cue", fmt.Sprintf("The narration has no marker {%d}.", b.Cue), v.cueFix(refs, b.Cue))
		}
	}
	if b.Delay < 0 || b.Delay > 600 {
		v.errorf(CodeInvalidValue, bb+"/delay", "The delay must be between 0 and 600 seconds.", `Use "delay": 0.5.`)
	}
	if b.Duration < 0 || b.Duration > 600 {
		v.errorf(CodeInvalidValue, bb+"/duration", "The duration must be between 0 and 600 seconds.", `Use "duration": 1.`)
	}
	if b.Ease != "" {
		v.bareName(bb+"/ease", &b.Ease, specEase)
	}

	needWho := func(enter bool) {
		if b.Who == "" {
			v.missing(bb, b, "who", " (a cast id)")
			return
		}
		if _, ok := v.cast[b.Who]; !ok {
			v.errorf(CodeUnknownCast, bb+"/who", fmt.Sprintf("The cast id %q is not in the story's cast.", clip(b.Who, 40)), v.castFix("who", b.Who))
			return
		}
		if !enter && !stage[b.Who] {
			v.warnf(CodeUnknownCast, bb+"/who", fmt.Sprintf("%q is not on stage here; the beat is ignored.", b.Who),
				`Add the actor to "actors" or use an enter beat before this one.`)
		}
	}
	needPos := func(field string, p *Position) {
		if p.IsZero() {
			v.missing(bb, b, field, " (a spot name or {x, y})")
			return
		}
		v.position(bb+"/"+field, p, env)
	}

	switch b.Do {
	case "enter":
		needWho(true)
		needPos("from", b.From)
		needPos("to", b.To)
		if b.Who != "" {
			stage[b.Who] = true
		}
	case "exit":
		needWho(false)
		needPos("to", b.To)
		delete(stage, b.Who)
	case "walk", "move":
		switch {
		case b.Who == "" && b.Target == "":
			v.missing(bb, b, "who", " or \"target\"")
		case b.Who != "":
			needWho(false)
		default:
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		needPos("to", b.To)
	case "pose":
		needWho(false)
		if b.Pose == "" {
			v.missing(bb, b, "pose", "")
		} else {
			v.bareName(bb+"/pose", &b.Pose, specPose)
		}
	case "gesture":
		needWho(false)
		if b.Gesture == "" {
			v.missing(bb, b, "gesture", "")
		} else {
			v.bareName(bb+"/gesture", &b.Gesture, specGesture)
		}
		if b.Count < 0 || b.Count > 5 {
			v.errorf(CodeInvalidValue, bb+"/count", "The count must be between 1 and 5.", `Use "count": 2.`)
		}
		if b.Target != "" {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
	case "emote":
		needWho(false)
		if b.Expression == "" {
			v.missing(bb, b, "expression", "")
		} else {
			v.bareName(bb+"/expression", &b.Expression, specExpression)
		}
	case "look":
		needWho(false)
		if b.Target == "" {
			v.missing(bb, b, "target", " (a cast id, instance id, spot or camera)")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
	case "say":
		needWho(false)
		switch b.Style {
		case "speech", "thought":
		case "":
			v.missing(bb, b, "style", " (speech or thought)")
		default:
			v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("A say style is \"speech\" or \"thought\", not %q.", clip(b.Style, 20)), `Use "style": "speech".`)
		}
	case "show":
		v.show(bb, b, refs, explicit, env)
	case "hide":
		if b.Target == "" {
			v.missing(bb, b, "target", " (an instance id, or an item id that is unique in the scene)")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
			if ids := refs.items[b.Target]; len(ids) > 1 {
				v.errorf(CodeInvalidValue, bb+"/target", fmt.Sprintf("%d instances of %s are in this scene, so the item id is ambiguous.", len(ids), b.Target),
					"Give the instances ids and hide one: "+exampleJSON("target", "lock-1")+".")
			}
		}
	case "highlight":
		if b.Target == "" {
			v.missing(bb, b, "target", "")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		switch b.Style {
		case "pulse", "outline", "arrow":
		case "":
			v.missing(bb, b, "style", " (pulse, outline or arrow)")
		default:
			v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("A highlight style is pulse, outline or arrow, not %q.", clip(b.Style, 20)), `Use "style": "pulse".`)
		}
	case "camera":
		switch b.Op {
		case "zoom", "pan", "focus", "shake":
		case "":
			v.missing(bb, b, "op", " (zoom, pan, focus or shake)")
		default:
			v.errorf(CodeInvalidValue, bb+"/op", fmt.Sprintf("A camera op is zoom, pan, focus or shake, not %q.", clip(b.Op, 20)), `Use "op": "zoom".`)
		}
		if b.Target != "" {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		if b.Amount < 0 {
			v.errorf(CodeInvalidValue, bb+"/amount", "The camera amount must not be negative.", `Use "amount": 1.2.`)
		}
	case "chart":
		switch b.Chart {
		case "bar", "line", "pie", "counter":
		case "":
			v.missing(bb, b, "chart", " (bar, line, pie or counter)")
		default:
			v.errorf(CodeInvalidValue, bb+"/chart", fmt.Sprintf("A chart is bar, line, pie or counter, not %q.", clip(b.Chart, 20)), `Use "chart": "bar".`)
		}
		v.chartData(bb, b)
		needPos("at", b.At)
	case "sfx":
		if b.Sound == "" {
			v.missing(bb, b, "sound", " (an sfx id)")
		} else {
			v.checkAsset(bb+"/sound", "sound", b.Sound, roleSound)
		}
	case "wait":
		switch {
		case b.Seconds <= 0:
			v.missing(bb, b, "seconds", " greater than 0")
		case b.Seconds > 600:
			v.errorf(CodeInvalidValue, bb+"/seconds", "A wait is at most 600 seconds.", `Use "seconds": 1.`)
		}
	}
}

func (v *validator) cueFix(refs *sceneRefs, got int) string {
	if len(refs.markers) == 0 {
		return fmt.Sprintf("The scene's narration has no markers. Put {%d} at the word where the beat should start, or remove \"cue\".", got)
	}
	var ns []string
	for n := 1; n <= 999; n++ {
		if refs.markers[n] {
			ns = append(ns, "{"+strconv.Itoa(n)+"}")
		}
	}
	return "Use one of the markers in the narration: " + listOf(ns) + ", or add {" + strconv.Itoa(got) + "} to the narration."
}

func (v *validator) show(bb string, b *Beat, refs *sceneRefs, explicit map[string]bool, env string) {
	kinds := 0
	for _, set := range []bool{b.Item != "", b.Text != "", b.Icon != "", b.Chart != ""} {
		if set {
			kinds++
		}
	}
	switch {
	case kinds == 0:
		v.errorf(CodeMissingField, bb+"/item", `A "show" beat needs one of "item", "text", "icon" or "chart".`, "Add \"item\". Example: "+beatExamples["show"])
	case kinds > 1:
		v.errorf(CodeInvalidValue, bb+"/item", `A "show" beat shows one thing: use only one of "item", "text", "icon" or "chart".`, "Split it into two show beats.")
	}
	if b.Item != "" {
		v.checkAsset(bb+"/item", "item", b.Item, roleProp)
	}
	if b.Text != "" && b.Style != "" {
		v.bareName(bb+"/style", &b.Style, specTextStyle)
	}
	if b.Icon != "" || b.Chart != "" {
		v.notYet(bb, "A show beat with an icon or a chart is accepted but not shown yet.")
	}
	if b.At.IsZero() {
		v.missing(bb, b, "at", " (a spot name or {x, y})")
	} else {
		v.position(bb+"/at", b.At, env)
	}
	switch b.Entry {
	case "pop", "fade", "slide", "none":
	case "":
		v.missing(bb, b, "entry", " (pop, fade, slide or none)")
	default:
		v.errorf(CodeInvalidValue, bb+"/entry", fmt.Sprintf("An entry is pop, fade, slide or none, not %q.", clip(b.Entry, 20)), `Use "entry": "pop".`)
	}
	if b.ID != "" {
		switch {
		case !validSlug(b.ID):
			v.errorf(CodeInvalidValue, bb+"/id", fmt.Sprintf("The instance id %q is not a slug.", clip(b.ID, 40)), "Use lowercase letters, digits and dashes, for example lock-1.")
		case explicit[b.ID]:
			v.errorf(CodeDuplicateID, bb+"/id", fmt.Sprintf("The instance id %q is used twice in this scene.", b.ID), "Give each shown thing its own id.")
		default:
			explicit[b.ID] = true
		}
	}
}

func (v *validator) chartData(bb string, b *Beat) {
	d := b.Data
	switch {
	case d == nil:
		v.missing(bb, b, "data", " ({labels, values})")
	case len(d.Labels) == 0 || len(d.Labels) != len(d.Values):
		v.errorf(CodeInvalidValue, bb+"/data", "Chart data needs the same number (at least one) of labels and values.", `Use "data": {"labels":["Mo","Di"],"values":[2,5]}.`)
	case len(d.Labels) > 24:
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("The chart has %d entries; the limit is 24.", len(d.Labels)), "Show fewer values.")
	}
}

// checkTarget validates a beat target: a cast id, an instance id, an item id of the scene, a
// spot or "camera".
func (v *validator) checkTarget(path, target string, refs *sceneRefs) {
	if target == "camera" {
		return
	}
	if _, ok := v.cast[target]; ok {
		return
	}
	if refs.instances[target] {
		return
	}
	if _, ok := refs.items[target]; ok {
		return
	}
	if refs.spotsKnown {
		if _, ok := refs.spots[target]; ok {
			return
		}
	} else if v.cat == nil && validSlug(target) {
		return // spots cannot be checked without a catalog
	}
	pool := append([]string{"camera"}, v.castIDs...)
	for id := range refs.instances {
		pool = append(pool, id)
	}
	for item := range refs.items {
		pool = append(pool, item)
	}
	for name := range refs.spots {
		pool = append(pool, name)
	}
	sort.Strings(pool)
	alts := closestNames(target, pool, 5)
	v.errorf(CodeUnknownName, path, fmt.Sprintf("The target %q is not in this scene.", clip(target, 40)),
		"Use a cast id, an instance id, an item id of this scene, a spot or camera: "+listOf(alts)+". Example: "+exampleJSON("target", alts[0]))
}
