package cydcreator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ProjectFormat is the cyd.project.json format this package reads and writes.
const ProjectFormat = 1

// newProjectTemplateVersion is stamped into new projects. It must equal
// TemplateVersion from template.go (plan 1b), which adds a test for that.
const newProjectTemplateVersion = "1.0.0"

// Languages are the AuraGo locales a project may use for device strings.
// The template falls back to English for el, hi, ja and zh.
var Languages = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// Project is cyd.project.json, the UI source of truth. It carries no server state.
type Project struct {
	Format          int      `json:"format"`
	TemplateVersion string   `json:"template_version"`
	Name            string   `json:"name"`
	Board           string   `json:"board"`
	Orientation     string   `json:"orientation"` // landscape portrait landscape_flipped portrait_flipped
	Theme           string   `json:"theme"`
	Language        string   `json:"language"` // any AuraGo locale; device strings fall back to en
	Timezone        string   `json:"timezone"` // POSIX TZ
	Settings        Settings `json:"settings"`
	StartScreen     string   `json:"start_screen"` // screen id
	Screens         []Screen `json:"screens"`
	Assets          []Asset  `json:"assets"`
}

// Settings are the per-app firmware options.
type Settings struct {
	WiFi           string   `json:"wifi"` // off optional required
	AuraGoLink     bool     `json:"aurago_link"`
	Libraries      []string `json:"libraries"` // Library.ID values
	Backlight      int      `json:"backlight"` // 10..100
	StatusbarClock bool     `json:"statusbar_clock"`
}

// Screen is one LVGL screen.
type Screen struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"` // C identifier
	Title      string   `json:"title"`
	Transition string   `json:"transition"` // none fade slide_left slide_right slide_up slide_down ("" = none)
	Style      Style    `json:"style"`
	Widgets    []Widget `json:"widgets"`
}

// Widget is one placed widget. Coordinates are pixels relative to the parent.
type Widget struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name,omitempty"`
	X        int            `json:"x"`
	Y        int            `json:"y"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	Tab      int            `json:"tab,omitempty"` // tab index when the parent is a tabview
	Locked   bool           `json:"locked,omitempty"`
	Hidden   bool           `json:"hidden,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	Style    Style          `json:"style,omitempty"`
	Events   []EventBinding `json:"events,omitempty"`
	Children []Widget       `json:"children,omitempty"`
}

// Style holds per-widget or per-screen overrides; empty = theme.
type Style struct {
	BgColor     string `json:"bg_color,omitempty"`
	BgOpacity   *int   `json:"bg_opacity,omitempty"` // 0..255
	TextColor   string `json:"text_color,omitempty"`
	AccentColor string `json:"accent_color,omitempty"`
	BorderColor string `json:"border_color,omitempty"`
	BorderWidth *int   `json:"border_width,omitempty"` // 0..8
	Radius      *int   `json:"radius,omitempty"`       // 0..64
}

// EventBinding connects a widget event to an action.
type EventBinding struct {
	On     string `json:"on"` // clicked long_pressed value_changed ready
	Action Action `json:"action"`
}

// Action is a built-in action or a logic hook for the agent.
type Action struct {
	Type       string `json:"type"`                 // goto_screen back open_wifi_setup logic
	Screen     string `json:"screen,omitempty"`     // screen id for goto_screen
	Transition string `json:"transition,omitempty"` // "" = the target screen's transition
	Hint       string `json:"hint,omitempty"`       // logic: ≤ MaxHintRunes runes
}

// Asset is an uploaded image; the generator converts it at build time.
type Asset struct {
	ID    string `json:"id"`
	File  string `json:"file"` // "assets/<name>.png|.jpg|.jpeg"
	W     int    `json:"w"`
	H     int    `json:"h"`
	Alpha bool   `json:"alpha"`
}

// NewProject returns a fresh project with one empty start screen "home".
// Empty language defaults to "en", empty timezone to "UTC0". Invalid input
// returns *Error CYD_BAD_REQUEST.
func NewProject(name, boardID, themeID, language, timezone string, ids IDSource) (Project, error) {
	if ids == nil {
		ids = RandomIDs{}
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxProjectNameRunes || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Project{}, NewError(CodeBadRequest,
			fmt.Sprintf("project name must be 1..%d characters without control characters", maxProjectNameRunes), nil)
	}
	board, err := BoardByID(boardID)
	if err != nil {
		return Project{}, err
	}
	if _, err := ThemeByID(themeID); err != nil {
		return Project{}, err
	}
	if language == "" {
		language = "en"
	}
	if !slices.Contains(Languages, language) {
		return Project{}, NewError(CodeBadRequest, fmt.Sprintf("unknown language %q", language), map[string]any{"language": language})
	}
	if timezone == "" {
		timezone = "UTC0"
	}
	screenID := ids.NewID("s")
	return Project{
		Format:          ProjectFormat,
		TemplateVersion: newProjectTemplateVersion,
		Name:            name,
		Board:           board.ID,
		Orientation:     board.Display.NativeOrientation,
		Theme:           themeID,
		Language:        language,
		Timezone:        timezone,
		Settings:        Settings{WiFi: "optional", Libraries: []string{}, Backlight: 100},
		StartScreen:     screenID,
		Screens:         []Screen{{ID: screenID, Name: "home", Title: "Home", Transition: "none", Widgets: []Widget{}}},
		Assets:          []Asset{},
	}, nil
}

// layoutError builds the CYD_INVALID_LAYOUT envelope. The reason can echo
// attacker-controlled text (an unknown key), so it is clipped to
// maxDiagMessageBytes at a rune boundary before it reaches the message and
// details.reason.
func layoutError(reason string, details map[string]any) *Error {
	if details == nil {
		details = map[string]any{}
	}
	reason = clipReason(reason)
	details["reason"] = reason
	return NewError(CodeInvalidLayout, "invalid layout: "+reason, details)
}

func clipReason(s string) string {
	if len(s) <= maxDiagMessageBytes {
		return s
	}
	cut := maxDiagMessageBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// layoutShapeFrame is one open JSON container in checkLayoutShape.
type layoutShapeFrame struct {
	array  bool
	commas int
}

// checkLayoutShape scans untrusted layout JSON without allocating per element
// and rejects nesting deeper than maxLayoutDepth or an array with more than
// maxLayoutArrayElems elements. It tracks string and escape state so brackets
// and commas inside strings do not count. It does not validate the syntax:
// malformed input that passes here is rejected by the decoder.
func checkLayoutShape(data []byte) error {
	stack := make([]layoutShapeFrame, 0, maxLayoutDepth)
	inString, escaped := false, false
	for _, c := range data {
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			if len(stack) >= maxLayoutDepth {
				return layoutError(fmt.Sprintf("nesting deeper than %d levels", maxLayoutDepth),
					map[string]any{"limit": maxLayoutDepth})
			}
			stack = append(stack, layoutShapeFrame{array: c == '['})
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ',':
			if n := len(stack); n > 0 && stack[n-1].array {
				stack[n-1].commas++
				// k commas separate k+1 elements.
				if stack[n-1].commas >= maxLayoutArrayElems {
					return layoutError(fmt.Sprintf("array with more than %d elements", maxLayoutArrayElems),
						map[string]any{"limit": maxLayoutArrayElems})
				}
			}
		}
	}
	return nil
}

// ParseProject decodes cyd.project.json strictly: size ≤ MaxLayoutBytes,
// nesting ≤ 32 levels, no array with more than 4096 elements, no unknown
// fields, no trailing data, format == ProjectFormat and well-formed
// screen/widget/asset ids. Semantic problems are left to Validate. Every
// failure is *Error CYD_INVALID_LAYOUT with details.reason.
func ParseProject(data []byte) (Project, error) {
	if len(data) > MaxLayoutBytes {
		return Project{}, layoutError(fmt.Sprintf("layout is %d bytes, maximum %d", len(data), MaxLayoutBytes),
			map[string]any{"limit": MaxLayoutBytes})
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	if err := checkLayoutShape(data); err != nil {
		return Project{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Project
	if err := dec.Decode(&p); err != nil {
		return Project{}, layoutError("decode: "+err.Error(), nil)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Project{}, layoutError("trailing data after the layout", nil)
	}
	if p.Format != ProjectFormat {
		return Project{}, layoutError(fmt.Sprintf("unsupported format %d (want %d)", p.Format, ProjectFormat),
			map[string]any{"format": p.Format})
	}
	normalizeProjectSlices(&p)
	if path, ok := firstInvalidID(p); !ok {
		return Project{}, layoutError("invalid id at "+path, map[string]any{"path": path})
	}
	return p, nil
}

func normalizeProjectSlices(p *Project) {
	if p.Screens == nil {
		p.Screens = []Screen{}
	}
	if p.Assets == nil {
		p.Assets = []Asset{}
	}
	if p.Settings.Libraries == nil {
		p.Settings.Libraries = []string{}
	}
	for i := range p.Screens {
		if p.Screens[i].Widgets == nil {
			p.Screens[i].Widgets = []Widget{}
		}
	}
}

func firstInvalidID(p Project) (string, bool) {
	for si, s := range p.Screens {
		base := fmt.Sprintf("screens/%d", si)
		if !validElementID(s.ID) {
			return base + "/id", false
		}
		if path, ok := firstInvalidWidgetID(s.Widgets, base+"/widgets"); !ok {
			return path, false
		}
	}
	for ai, a := range p.Assets {
		if !validElementID(a.ID) {
			return fmt.Sprintf("assets/%d/id", ai), false
		}
	}
	return "", true
}

func firstInvalidWidgetID(list []Widget, base string) (string, bool) {
	for i, w := range list {
		path := fmt.Sprintf("%s/%d", base, i)
		if !validElementID(w.ID) {
			return path + "/id", false
		}
		if p, ok := firstInvalidWidgetID(w.Children, path+"/children"); !ok {
			return p, false
		}
	}
	return "", true
}

// MarshalProject writes the canonical form: struct field order, sorted map
// keys, 2-space indent, no HTML escaping, trailing newline. nil slices are
// written as []. The input is not modified. A canonical document larger than
// MaxLayoutBytes could not be parsed back, so it fails with *Error
// CYD_INVALID_LAYOUT (details.limit, details.bytes).
func MarshalProject(p Project) ([]byte, error) {
	screens := make([]Screen, len(p.Screens))
	copy(screens, p.Screens)
	p.Screens = screens
	normalizeProjectSlices(&p)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return nil, fmt.Errorf("encode project: %w", err)
	}
	if buf.Len() > MaxLayoutBytes {
		return nil, layoutError(fmt.Sprintf("canonical layout is %d bytes, maximum %d", buf.Len(), MaxLayoutBytes),
			map[string]any{"limit": MaxLayoutBytes, "bytes": buf.Len()})
	}
	return buf.Bytes(), nil
}

// FindWidget looks a widget up by id first, then by name, across all screens.
// path holds the child indexes from the screen's root list.
func (p Project) FindWidget(idOrName string) (screenIdx int, path []int, ok bool) {
	if idOrName == "" {
		return -1, nil, false
	}
	for _, match := range []func(*Widget) bool{
		func(w *Widget) bool { return w.ID == idOrName },
		func(w *Widget) bool { return w.Name == idOrName },
	} {
		for si := range p.Screens {
			if path := findWidgetPath(p.Screens[si].Widgets, match); path != nil {
				return si, path, true
			}
		}
	}
	return -1, nil, false
}

func findWidgetPath(list []Widget, match func(*Widget) bool) []int {
	for i := range list {
		if match(&list[i]) {
			return []int{i}
		}
		if sub := findWidgetPath(list[i].Children, match); sub != nil {
			return append([]int{i}, sub...)
		}
	}
	return nil
}

// WalkWidgets visits every widget depth-first in document order (parents
// before children). parent is nil for widgets placed directly on a screen.
// Returning false stops the walk. The pointers alias p's slices, which share
// their backing arrays with the caller's Project.
func (p Project) WalkWidgets(fn func(screen *Screen, parent *Widget, w *Widget) bool) {
	for si := range p.Screens {
		s := &p.Screens[si]
		if !walkWidgetList(s, nil, s.Widgets, fn) {
			return
		}
	}
}

func walkWidgetList(s *Screen, parent *Widget, list []Widget, fn func(*Screen, *Widget, *Widget) bool) bool {
	for i := range list {
		w := &list[i]
		if !fn(s, parent, w) {
			return false
		}
		if !walkWidgetList(s, w, w.Children, fn) {
			return false
		}
	}
	return true
}
