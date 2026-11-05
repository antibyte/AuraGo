package storymaker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// DecodeError is returned by DecodeStory. Issues always holds at least one entry, so an
// HTTP layer can answer 422 with the issues.
type DecodeError struct {
	Code    string
	Message string
	Issues  []Issue
}

func (e *DecodeError) Error() string { return "storymaker: " + e.Message }

func decodeFailure(issues ...Issue) (Story, []Issue, error) {
	return Story{}, issues, &DecodeError{Code: issues[0].Code, Message: issues[0].Message, Issues: issues}
}

// maxDecodeIssues bounds the unknown-field findings of one decode.
const maxDecodeIssues = 20

// DecodeStory parses a story document strictly: one JSON object, the storymaker/1 schema,
// no unknown fields (each is reported with a hint for common mistakes) and the size limit.
// Findings that need the catalog or cross references are Validate's job; a successful
// decode returns no issues. A failure returns a *DecodeError and its issues.
func DecodeStory(data []byte, lim Limits) (Story, []Issue, error) {
	if lim.MaxStoryBytes > 0 && len(data) > lim.MaxStoryBytes {
		return decodeFailure(Issue{
			Severity: SeverityError, Path: "", Code: CodeTooLarge,
			Message: fmt.Sprintf("The story is %d bytes; the limit is %d bytes.", len(data), lim.MaxStoryBytes),
			Fix:     "Remove scenes or shorten narration, or move long animation into code scenes.",
		})
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return decodeFailure(syntaxIssue("The story is empty.", "Send the complete story JSON object."))
	}

	var tree any
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tree); err != nil {
		return decodeFailure(jsonErrorIssue(data, err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return decodeFailure(syntaxIssue("There is more data after the story object.", "Send exactly one JSON object."))
	}
	root, ok := tree.(map[string]any)
	if !ok {
		return decodeFailure(syntaxIssue("The story must be a JSON object.", `Start with {"schema":"storymaker/1","title":"...","scenes":[...]}.`))
	}
	if sch, _ := root["schema"].(string); sch != Schema {
		msg := fmt.Sprintf("The schema must be %q.", Schema)
		if sch != "" {
			msg = fmt.Sprintf("The schema %q is not supported; this version reads %q.", sch, Schema)
		}
		return decodeFailure(Issue{
			Severity: SeverityError, Path: "/schema", Code: CodeSchemaMismatch, Message: msg,
			Fix: `Set "schema": "storymaker/1".`,
		})
	}

	var hits []unknownHit
	collectUnknown(root, reflect.TypeOf(Story{}), "", &hits)
	if len(hits) > 0 {
		order := keyOrder(data)
		sort.SliceStable(hits, func(i, j int) bool { return order[hits[i].path] < order[hits[j].path] })
		if len(hits) > maxDecodeIssues {
			hits = hits[:maxDecodeIssues]
		}
		issues := make([]Issue, 0, len(hits))
		for _, h := range hits {
			issues = append(issues, h.issue())
		}
		return decodeFailure(issues...)
	}

	var s Story
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&s); err != nil {
		return decodeFailure(jsonErrorIssue(data, err))
	}
	return s, nil, nil
}

func syntaxIssue(msg, fix string) Issue {
	return Issue{Severity: SeverityError, Path: "", Code: CodeInvalidJSON, Message: msg, Fix: fix}
}

// jsonErrorIssue maps an encoding/json error to an issue with the best location.
func jsonErrorIssue(data []byte, err error) Issue {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syn):
		line, col := lineColumn(data, syn.Offset)
		return syntaxIssue(fmt.Sprintf("Invalid JSON at line %d, column %d: %s.", line, col, syn.Error()),
			"Fix the syntax (quotes, commas, brackets) and send the complete story.")
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return syntaxIssue("The JSON ends too early.", "Close every string, array and object and send the complete story.")
	case errors.As(err, &typ):
		path := pointerAtOffset(data, typ.Offset)
		if path == "" && typ.Field != "" {
			path = "/" + strings.ReplaceAll(typ.Field, ".", "/")
		}
		return Issue{
			Severity: SeverityError, Path: path, Code: CodeInvalidJSON,
			Message: fmt.Sprintf("Wrong type at %s: expected %s, got %s.", pathLabel(path), jsonTypeName(typ.Type), typ.Value),
			Fix:     "Use the value type of the story format, for example a number without quotes for fps.",
		}
	}
	return syntaxIssue("The story could not be read: "+err.Error()+".", "Check the story against the storymaker/1 format.")
}

func pathLabel(p string) string {
	if p == "" {
		return "the story root"
	}
	return p
}

func jsonTypeName(t reflect.Type) string {
	if t == nil {
		return "another type"
	}
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.Struct, reflect.Map:
		return "an object"
	case reflect.Pointer:
		return jsonTypeName(t.Elem())
	}
	return t.String()
}

func lineColumn(data []byte, offset int64) (line, col int) {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	if offset < 0 {
		offset = 0
	}
	line, col = 1, 1
	for _, c := range string(data[:offset]) {
		if c == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// pointerAtOffset returns the JSON Pointer of the value whose last byte lies at offset (the
// position encoding/json reports in an UnmarshalTypeError). It returns "" when it cannot tell.
func pointerAtOffset(data []byte, offset int64) string {
	type frame struct {
		obj     bool
		key     string
		haveKey bool
		idx     int
	}
	var stack []frame
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		n := len(stack)
		delim, isDelim := tok.(json.Delim)
		if isDelim && (delim == '}' || delim == ']') {
			stack = stack[:n-1]
			if n >= 2 && stack[n-2].obj {
				stack[n-2].haveKey = false
			}
			continue
		}
		if n > 0 && stack[n-1].obj && !stack[n-1].haveKey {
			key, _ := tok.(string)
			stack[n-1].key, stack[n-1].haveKey = key, true
			continue
		}
		if n > 0 && !stack[n-1].obj {
			stack[n-1].idx++
		}
		if dec.InputOffset() >= offset {
			var b strings.Builder
			for _, f := range stack {
				if f.obj {
					b.WriteString("/" + escapePointer(f.key))
				} else {
					b.WriteString("/" + strconv.Itoa(f.idx))
				}
			}
			return b.String()
		}
		if isDelim {
			stack = append(stack, frame{obj: delim == '{', idx: -1})
			continue
		}
		if n > 0 && stack[n-1].obj {
			stack[n-1].haveKey = false
		}
	}
}

// escapePointer escapes one JSON Pointer segment (RFC 6901).
func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

// keyOrder numbers every object key of the document by its position, keyed by its JSON Pointer.
func keyOrder(data []byte) map[string]int {
	type frame struct {
		obj     bool
		key     string
		haveKey bool
		idx     int
	}
	order := map[string]int{}
	var stack []frame
	dec := json.NewDecoder(bytes.NewReader(data))
	pointer := func() string {
		var b strings.Builder
		for _, f := range stack {
			if f.obj {
				b.WriteString("/" + escapePointer(f.key))
			} else {
				b.WriteString("/" + strconv.Itoa(f.idx))
			}
		}
		return b.String()
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return order
		}
		n := len(stack)
		delim, isDelim := tok.(json.Delim)
		if isDelim && (delim == '}' || delim == ']') {
			stack = stack[:n-1]
			if n >= 2 && stack[n-2].obj {
				stack[n-2].haveKey = false
			}
			continue
		}
		if n > 0 && stack[n-1].obj && !stack[n-1].haveKey {
			key, _ := tok.(string)
			stack[n-1].key, stack[n-1].haveKey = key, true
			order[pointer()] = len(order)
			continue
		}
		if n > 0 && !stack[n-1].obj {
			stack[n-1].idx++
		}
		if isDelim {
			stack = append(stack, frame{obj: delim == '{', idx: -1})
			continue
		}
		if n > 0 && stack[n-1].obj {
			stack[n-1].haveKey = false
		}
	}
}

// ---------------------------------------------------------------------------
// Unknown fields: a reflection walk over the generic JSON tree finds every unknown key
// with its path (the typed strict decode is the safety net behind it).

type unknownHit struct {
	path    string
	typ     string
	key     string
	allowed []string
}

func (h unknownHit) issue() Issue {
	fix := fieldHints[h.typ+"."+h.key]
	if fix == "" {
		fix = fieldHints["*."+h.key]
	}
	if fix == "" {
		fix = "Remove it, or use one of the fields of a " + typeLabel(h.typ) + ": " + strings.Join(h.allowed, ", ") + "."
	}
	return Issue{
		Severity: SeverityError, Path: h.path, Code: CodeUnknownField,
		Message: fmt.Sprintf("A %s has no field %q.", typeLabel(h.typ), h.key),
		Fix:     fix,
	}
}

func typeLabel(t string) string {
	if t == "CastMember" {
		return "cast member"
	}
	if t == "ChartData" {
		return "chart data"
	}
	return strings.ToLower(t)
}

type structFields struct {
	types map[string]reflect.Type
	order []string
}

var (
	structFieldsCache sync.Map // reflect.Type -> *structFields
	positionType      = reflect.TypeOf(Position{})
	durationType      = reflect.TypeOf(Duration{})
)

func fieldsOf(t reflect.Type) *structFields {
	if v, ok := structFieldsCache.Load(t); ok {
		return v.(*structFields)
	}
	sf := &structFields{types: map[string]reflect.Type{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		sf.types[name] = f.Type
		sf.order = append(sf.order, name)
	}
	structFieldsCache.Store(t, sf)
	return sf
}

func collectUnknown(v any, t reflect.Type, path string, out *[]unknownHit) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == positionType || t == durationType {
		return // polymorphic values validate their own shape
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return // the typed decode reports the wrong type
		}
		sf := fieldsOf(t)
		for k, val := range obj {
			ft, known := sf.types[k]
			p := path + "/" + escapePointer(k)
			if !known {
				*out = append(*out, unknownHit{path: p, typ: t.Name(), key: k, allowed: sf.order})
				continue
			}
			collectUnknown(val, ft, p, out)
		}
	case reflect.Slice:
		arr, ok := v.([]any)
		if !ok {
			return
		}
		for i, e := range arr {
			collectUnknown(e, t.Elem(), path+"/"+strconv.Itoa(i), out)
		}
	}
}

// fieldHints maps "<Type>.<key>" (or "*.<key>") of common model mistakes to a one-sentence fix.
var fieldHints = map[string]string{
	"Story.characters":  `Recurring people go in "cast": [{"id":"anna","name":"Anna","character":"char.office-woman-1"}].`,
	"Story.actors":      `Actors belong to a scene: scenes[].actors, each {"cast":"anna","at":"center"}.`,
	"Story.language":    `The narration language is voice.language, for example "voice":{"provider":"piper","voice":"de_DE-thorsten-high","language":"de"}.`,
	"Story.aspect":      `Put aspect and fps in "format": {"aspect":"16:9","fps":30}.`,
	"Story.fps":         `Put aspect and fps in "format": {"aspect":"16:9","fps":30}.`,
	"Story.resolution":  `The canvas is "format": {"aspect":"16:9","fps":30}; there is no free resolution.`,
	"Story.palette":     `The palette is "style": {"palette":"sunny"}.`,
	"Story.description": `A story has only a title; put the spoken text into scene narration.`,
	"Story.summary":     `A story has only a title; put the spoken text into scene narration.`,
	"Story.duration":    `Durations belong to scenes: scenes[].duration is "auto" or a number of seconds.`,
	"Story.id":          `A story has no id; the project id is kept by the app.`,
	"Story.name":        `Use "title".`,
	"Story.rhythm":      `The rhythm belongs to the style: "style": {"rhythm":"calm"}.`,

	"Scene.text":        `The spoken text is "narration"; on-screen text is a show beat: {"do":"show","text":"...","at":"center","entry":"fade"}.`,
	"Scene.script":      `The spoken text is "narration".`,
	"Scene.dialogue":    `The spoken text is "narration"; one scene per speaker turn with "speaker":"<cast id>".`,
	"Scene.voiceover":   `The spoken text is "narration".`,
	"Scene.voice_over":  `The spoken text is "narration".`,
	"Scene.description": `The spoken text is "narration"; the place is "environment".`,
	"Scene.summary":     `The spoken text is "narration".`,
	"Scene.background":  `The place is "environment", a full id such as "env.office".`,
	"Scene.setting":     `The place is "environment", a full id such as "env.office".`,
	"Scene.location":    `The place is "environment", a full id such as "env.office".`,
	"Scene.backdrop":    `The place is "environment", a full id such as "env.office".`,
	"Scene.characters":  `People on stage are "actors": [{"cast":"anna","at":"left"}].`,
	"Scene.cast":        `People on stage are "actors": [{"cast":"anna","at":"left"}].`,
	"Scene.items":       `Objects on stage are "props": [{"item":"prop.laptop","at":"right"}].`,
	"Scene.objects":     `Objects on stage are "props": [{"item":"prop.laptop","at":"right"}].`,
	"Scene.actions":     `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.animations":  `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.events":      `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.steps":       `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.code":        `A code scene has no code key: set "kind":"code"; the source is saved as scenes/<id>.js.`,
	"Scene.title":       `A scene has no title; use "narration" or a show beat with text.`,
	"Scene.name":        `A scene has no name; its "id" is a short slug such as "s1".`,
	"Scene.length":      `Use "duration": "auto" or a number of seconds.`,
	"Scene.time":        `Use "duration": "auto" or a number of seconds.`,
	"Scene.seconds":     `Use "duration": "auto" or a number of seconds.`,
	"Scene.music":       `Music is set once for the story: "music": {"mode":"none"}.`,
	"Scene.framing":     `The camera is "camera": {"framing":"wide"}.`,
	"Scene.rhythm":      `The rhythm is set once for the story: "style": {"rhythm":"calm"}.`,

	"Actor.character": `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.id":        `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.name":      `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.who":       `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.position":  `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.location":  `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.spot":      `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.x":         `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.y":         `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.emotion":   `Use "expression", for example "happy".`,
	"Actor.mood":      `Use "expression", for example "happy".`,
	"Actor.animation": `Use "pose", for example "typing".`,
	"Actor.action":    `Use "pose", for example "typing"; timed changes are beats.`,
	"Actor.direction": `Use "facing": "left", "right" or "auto".`,

	"Prop.name":     `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.asset":    `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.asset_id": `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.prop":     `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.object":   `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.position": `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.location": `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.x":        `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.y":        `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.z":        `Use "layer": "back" or "front".`,
	"Prop.z_index":  `Use "layer": "back" or "front".`,

	"Beat.type":         `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.action":       `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.kind":         `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.event":        `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.actor":        `The cast member is "who", for example "anna".`,
	"Beat.character":    `The cast member is "who", for example "anna".`,
	"Beat.cast":         `The cast member is "who", for example "anna".`,
	"Beat.name":         `The cast member is "who", for example "anna".`,
	"Beat.position":     `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.location":     `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.destination":  `Use "to" for walk, move, enter and exit.`,
	"Beat.where":        `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.x":            `Use "to" or "at": a spot name or {"x":50,"y":88}.`,
	"Beat.y":            `Use "to" or "at": a spot name or {"x":50,"y":88}.`,
	"Beat.emotion":      `Use "expression" with do "emote", for example {"do":"emote","who":"anna","expression":"happy"}.`,
	"Beat.mood":         `Use "expression" with do "emote".`,
	"Beat.animation":    `Use "pose" (do "pose") or "gesture" (do "gesture").`,
	"Beat.anim":         `Use "pose" (do "pose") or "gesture" (do "gesture").`,
	"Beat.time":         `Use "delay" (seconds after the previous beat starts) or "cue" (a narration marker number).`,
	"Beat.start":        `Use "delay" (seconds after the previous beat starts) or "cue" (a narration marker number).`,
	"Beat.at_seconds":   `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.offset":       `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.t":            `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.sfx":          `Use "sound" with an sfx id: {"do":"sfx","sound":"sfx.pop"}.`,
	"Beat.sound_effect": `Use "sound" with an sfx id: {"do":"sfx","sound":"sfx.pop"}.`,
	"Beat.image":        `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.prop":         `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.asset":        `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.object":       `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.message":      `Use "text".`,
	"Beat.content":      `Use "text".`,
	"Beat.label":        `Use "text".`,
	"Beat.caption":      `Use "text".`,
	"Beat.speech":       `Use "text" with do "say".`,
	"Beat.direction":    `Use "from" (enter) or "to" (exit, walk).`,
	"Beat.repeat":       `Use "count" (1 to 5) with do "gesture".`,
	"Beat.times":        `Use "count" (1 to 5) with do "gesture".`,
	"Beat.easing":       `Use "ease": linear, ease-in, ease-out, ease-in-out, back-out or bounce-out.`,
	"Beat.curve":        `Use "ease": linear, ease-in, ease-out, ease-in-out, back-out or bounce-out.`,
	"Beat.wait":         `Use {"do":"wait","seconds":1}.`,

	"CastMember.char":        `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.avatar":      `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.asset":       `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.displayname": `Use "name".`,
	"CastMember.label":       `Use "name".`,

	"Format.width":      `The canvas is "aspect": "16:9", "9:16" or "1:1"; there is no free size.`,
	"Format.height":     `The canvas is "aspect": "16:9", "9:16" or "1:1"; there is no free size.`,
	"Format.resolution": `The canvas is "aspect": "16:9", "9:16" or "1:1".`,
	"Format.framerate":  `Use "fps": 24, 25 or 30.`,

	"Style.color":     `Use "palette" with a palette name such as "sunny".`,
	"Style.colors":    `Use "palette" with a palette name such as "sunny".`,
	"Style.theme":     `Use "palette" with a palette name such as "sunny".`,
	"Style.typeface":  `Use "font": "default" or a bundled font name.`,
	"Style.subtitles": `Use "captions": true or false.`,
	"Style.tempo":     `Use "rhythm": "calm", "standard" or "lively".`,
	"Style.pace":      `Use "rhythm": "calm", "standard" or "lively".`,

	"Voice.lang":   `Use "language", for example "de".`,
	"Voice.engine": `Use "provider", for example "piper".`,
	"Voice.tts":    `Use "provider", for example "piper".`,
	"Voice.id":     `Use "voice" with the voice id.`,
	"Voice.name":   `Use "voice" with the voice id.`,

	"Music.path":  `Use "file" with a desktop path and "mode":"file".`,
	"Music.url":   `Music cannot be loaded from a URL; use "mode":"file" with a desktop path.`,
	"Music.style": `Use "mood", for example "upbeat-corporate".`,
	"Music.genre": `Use "mood", for example "upbeat-corporate".`,
	"Music.gain":  `Use "volume" between 0 and 1.`,
}
