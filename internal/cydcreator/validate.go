package cydcreator

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Issue severities (Diagnostic additionally uses SeverityNote).
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityNote    = "note"
)

// Validation issue codes (overview "Validation issue codes"); the UI
// translates them as cyd_creator.issue.<CODE>.
const (
	IssueNameInvalid             = "NAME_INVALID"
	IssueNameDuplicate           = "NAME_DUPLICATE"
	IssueNameRequired            = "NAME_REQUIRED"
	IssueScreenNameInvalid       = "SCREEN_NAME_INVALID"
	IssueScreenNameDuplicate     = "SCREEN_NAME_DUPLICATE"
	IssueStartScreenMissing      = "START_SCREEN_MISSING"
	IssueUnknownType             = "UNKNOWN_TYPE"
	IssueUnknownProp             = "UNKNOWN_PROP"
	IssuePropInvalid             = "PROP_INVALID"
	IssueTextTooLong             = "TEXT_TOO_LONG"
	IssueHintTooLong             = "HINT_TOO_LONG"
	IssueOutOfBounds             = "OUT_OF_BOUNDS"
	IssueSizeInvalid             = "SIZE_INVALID"
	IssueTooManyScreens          = "TOO_MANY_SCREENS"
	IssueTooManyWidgets          = "TOO_MANY_WIDGETS"
	IssueNestingTooDeep          = "NESTING_TOO_DEEP"
	IssueChildrenNotAllowed      = "CHILDREN_NOT_ALLOWED"
	IssueTabInvalid              = "TAB_INVALID"
	IssueEventNotAllowed         = "EVENT_NOT_ALLOWED"
	IssueActionInvalid           = "ACTION_INVALID"
	IssueScreenTargetMissing     = "SCREEN_TARGET_MISSING"
	IssueAssetMissing            = "ASSET_MISSING"
	IssueAssetBudgetExceeded     = "ASSET_BUDGET_EXCEEDED"
	IssueThemeUnknown            = "THEME_UNKNOWN"
	IssueBoardUnknown            = "BOARD_UNKNOWN"
	IssueLibraryUnknown          = "LIBRARY_UNKNOWN"
	IssueWiFiModeInvalid         = "WIFI_MODE_INVALID"
	IssueLinkRequiresWiFi        = "LINK_REQUIRES_WIFI"
	IssueFontUnknown             = "FONT_UNKNOWN"
	IssueFontSizeInvalid         = "FONT_SIZE_INVALID"
	IssueSymbolUnknown           = "SYMBOL_UNKNOWN"
	IssueColorInvalid            = "COLOR_INVALID"
	IssueLanguageUnknown         = "LANGUAGE_UNKNOWN"
	IssueOrientationInvalid      = "ORIENTATION_INVALID"
	IssueTransitionInvalid       = "TRANSITION_INVALID"
	IssueIDDuplicate             = "ID_DUPLICATE"
	IssuePartlyOutOfBounds       = "PARTLY_OUT_OF_BOUNDS"
	IssueStatusbarClockNeedsWiFi = "STATUSBAR_CLOCK_NEEDS_WIFI"
	IssueCanvasLarge             = "CANVAS_LARGE"
)

var allIssueCodes = []string{
	IssueNameInvalid, IssueNameDuplicate, IssueNameRequired, IssueScreenNameInvalid, IssueScreenNameDuplicate,
	IssueStartScreenMissing, IssueUnknownType, IssueUnknownProp, IssuePropInvalid, IssueTextTooLong,
	IssueHintTooLong, IssueOutOfBounds, IssueSizeInvalid, IssueTooManyScreens, IssueTooManyWidgets,
	IssueNestingTooDeep, IssueChildrenNotAllowed, IssueTabInvalid, IssueEventNotAllowed, IssueActionInvalid,
	IssueScreenTargetMissing, IssueAssetMissing, IssueAssetBudgetExceeded, IssueThemeUnknown, IssueBoardUnknown,
	IssueLibraryUnknown, IssueWiFiModeInvalid, IssueLinkRequiresWiFi, IssueFontUnknown, IssueFontSizeInvalid,
	IssueSymbolUnknown, IssueColorInvalid, IssueLanguageUnknown, IssueOrientationInvalid, IssueTransitionInvalid,
	IssueIDDuplicate, IssuePartlyOutOfBounds, IssueStatusbarClockNeedsWiFi, IssueCanvasLarge,
}

// IssueCodes returns every validation code in overview order (errors, then warnings).
func IssueCodes() []string { return slices.Clone(allIssueCodes) }

// Issue is one validation finding.
type Issue struct {
	Severity string `json:"severity"` // error | warning
	Code     string `json:"code"`
	Message  string `json:"message"` // English, for logs and the agent; UI translates by Code
	Screen   string `json:"screen,omitempty"`
	Widget   string `json:"widget,omitempty"`
	Path     string `json:"path,omitempty"` // screens/0/widgets/2/children/1/props/text
}

// ValidateOptions carries data that is not part of the layout.
type ValidateOptions struct {
	AssetSizes map[string]int64 // asset id → decoded RGB565(A8) byte size; nil skips the budget check
}

// Enumerations used by the layout.
var (
	orientations   = []string{"landscape", "portrait", "landscape_flipped", "portrait_flipped"}
	transitions    = []string{"none", "fade", "slide_left", "slide_right", "slide_up", "slide_down"}
	wifiModes      = []string{"off", "optional", "required"}
	actionTypes    = []string{"goto_screen", "back", "open_wifi_setup", "logic"}
	namePattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	colorPattern   = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	assetFileRegex = regexp.MustCompile(`^assets/[a-z0-9][a-z0-9_-]{0,62}\.(png|jpg|jpeg)$`)
	// Widget names become ui_<name>, on_<name>_<event>, ui_show_<name>,
	// ui_hide_<name> and ui_<name>_series; these would collide with other
	// generated symbols (ui_init, ui_goto, ui_back, ui_theme_colors,
	// ui_event_index, ui_async (cyd::ui_async), the types ui_screen_t and
	// ui_transition_t, ui_scr_*, ui_font_*). ui_img_<asset id> is checked
	// against the project's assets.
	reservedNames        = []string{"init", "goto", "back", "theme_colors", "event_index", "async", "screen_t", "transition_t"}
	reservedNamePrefixes = []string{"scr_", "show_", "hide_", "font_"}
	reservedNameSuffixes = []string{"_series"}

	// Asset ids become the C symbol ui_img_<id>.
	assetSymbolPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

const (
	// maxEchoBytes bounds every user-controlled value (names, ids, types, prop
	// keys, ...) that an issue message repeats; a layout may carry up to 1 MB
	// of such text.
	maxEchoBytes = 64
	// maxUnknownPropIssues caps the UNKNOWN_PROP issues of one widget; the rest
	// is summarised in one more issue with the same code.
	maxUnknownPropIssues = 20
	maxTimezoneBytes     = 64
)

// clipEcho shortens s to maxEchoBytes at a rune boundary and appends "…".
func clipEcho(s string) string {
	if len(s) <= maxEchoBytes {
		return s
	}
	cut := maxEchoBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// Validate checks a project against spec §3/§4 and the catalog. Issues are
// returned in a deterministic order: project, screens in order (each screen
// before its widgets, widgets depth-first), assets.
func Validate(p Project, opts ValidateOptions) []Issue {
	v := &validator{
		p:           p,
		opts:        opts,
		screenIDs:   map[string]bool{},
		screenNames: map[string]bool{},
		ids:         map[string]bool{},
		names:       map[string]bool{},
		assetIDs:    map[string]bool{},
	}
	for _, s := range p.Screens {
		v.screenIDs[s.ID] = true
	}
	for _, a := range p.Assets {
		v.assetIDs[a.ID] = true
	}
	v.project()
	for si := range p.Screens {
		v.screen(si)
	}
	v.assets()
	return v.issues
}

// HasErrors reports whether any issue has error severity.
func HasErrors(issues []Issue) bool {
	for _, is := range issues {
		if is.Severity == SeverityError {
			return true
		}
	}
	return false
}

type validator struct {
	p           Project
	opts        ValidateOptions
	issues      []Issue
	board       *Board
	screenIDs   map[string]bool
	screenNames map[string]bool // names of the screens seen so far
	ids         map[string]bool // screen and widget ids
	names       map[string]bool // widget names
	assetIDs    map[string]bool
}

func (v *validator) add(sev, code, screen, widget, path, format string, args ...any) {
	v.issues = append(v.issues, Issue{Severity: sev, Code: code, Message: fmt.Sprintf(format, args...),
		Screen: screen, Widget: widget, Path: path})
}

// projectNameProblem mirrors NewProject's name rule (1..maxProjectNameRunes
// characters, no control characters, not blank) and additionally requires
// valid UTF-8, since the name reaches generated C code and file names.
func projectNameProblem(name string) string {
	switch {
	case !utf8.ValidString(name):
		return "is not valid UTF-8 text"
	case strings.TrimSpace(name) == "":
		return "must not be empty"
	case utf8.RuneCountInString(name) > maxProjectNameRunes:
		return fmt.Sprintf("is %d characters, maximum %d", utf8.RuneCountInString(name), maxProjectNameRunes)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "must not contain control characters"
	}
	return ""
}

// timezoneProblem checks a POSIX TZ string: 1..maxTimezoneBytes printable
// ASCII characters. NewProject turns an empty timezone into "UTC0", so an
// empty stored value never comes from it.
func timezoneProblem(tz string) string {
	if tz == "" {
		return "must not be empty"
	}
	if len(tz) > maxTimezoneBytes {
		return fmt.Sprintf("is %d bytes, maximum %d", len(tz), maxTimezoneBytes)
	}
	for i := 0; i < len(tz); i++ {
		if tz[i] < 0x20 || tz[i] > 0x7e {
			return "must be printable ASCII"
		}
	}
	return ""
}

func (v *validator) project() {
	p := v.p
	if reason := projectNameProblem(p.Name); reason != "" {
		v.add(SeverityError, IssuePropInvalid, "", "", "name", "project name %s", reason)
	}
	if b, err := BoardByID(p.Board); err != nil {
		v.add(SeverityError, IssueBoardUnknown, "", "", "board", "unknown board %q", clipEcho(p.Board))
	} else {
		v.board = &b
	}
	if _, err := ThemeByID(p.Theme); err != nil {
		v.add(SeverityError, IssueThemeUnknown, "", "", "theme", "unknown theme %q", clipEcho(p.Theme))
	}
	if !slices.Contains(orientations, p.Orientation) {
		v.add(SeverityError, IssueOrientationInvalid, "", "", "orientation", "orientation %q must be one of %s",
			clipEcho(p.Orientation), strings.Join(orientations, ", "))
	} else if v.board != nil && v.board.Display.Driver == "NV3041A" &&
		strings.HasPrefix(p.Orientation, "portrait") != (v.board.Display.NativeOrientation == "portrait") {
		// The NV3041A cannot rotate by register (LovyanGFX), so only the board's
		// native orientation family works; the template has a matching #error.
		v.add(SeverityError, IssueOrientationInvalid, "", "", "orientation",
			"orientation %q is not available: this panel supports %s only", clipEcho(p.Orientation),
			v.board.Display.NativeOrientation)
	}
	if !slices.Contains(Languages, p.Language) {
		v.add(SeverityError, IssueLanguageUnknown, "", "", "language", "unknown language %q", clipEcho(p.Language))
	}
	if reason := timezoneProblem(p.Timezone); reason != "" {
		v.add(SeverityError, IssuePropInvalid, "", "", "timezone", "timezone %s", reason)
	}
	st := p.Settings
	if !slices.Contains(wifiModes, st.WiFi) {
		v.add(SeverityError, IssueWiFiModeInvalid, "", "", "settings/wifi", "wifi mode %q must be off, optional or required", clipEcho(st.WiFi))
	}
	if st.AuraGoLink && st.WiFi == "off" {
		v.add(SeverityError, IssueLinkRequiresWiFi, "", "", "settings/aurago_link", "the AuraGo link needs Wi-Fi (wifi is off)")
	}
	for i, id := range st.Libraries {
		if _, ok := libraryByID(id); !ok {
			v.add(SeverityError, IssueLibraryUnknown, "", "", fmt.Sprintf("settings/libraries/%d", i), "unknown library %q", clipEcho(id))
		}
	}
	if st.Backlight < 10 || st.Backlight > 100 {
		v.add(SeverityError, IssuePropInvalid, "", "", "settings/backlight", "backlight %d must be within 10..100", st.Backlight)
	}
	if st.StatusbarClock && st.WiFi == "off" {
		v.add(SeverityWarning, IssueStatusbarClockNeedsWiFi, "", "", "settings/statusbar_clock",
			"the status bar clock needs Wi-Fi for NTP (wifi is off)")
	}
	if len(p.Screens) > MaxScreens {
		v.add(SeverityError, IssueTooManyScreens, "", "", "screens", "%d screens, maximum %d", len(p.Screens), MaxScreens)
	}
	if !v.screenIDs[p.StartScreen] {
		v.add(SeverityError, IssueStartScreenMissing, "", "", "start_screen", "start screen %q does not exist", clipEcho(p.StartScreen))
	}
}

func (v *validator) screen(si int) {
	s := &v.p.Screens[si]
	path := fmt.Sprintf("screens/%d", si)
	if v.ids[s.ID] {
		v.add(SeverityError, IssueIDDuplicate, s.ID, "", path+"/id", "id %q is used twice", clipEcho(s.ID))
	}
	v.ids[s.ID] = true
	switch {
	case !namePattern.MatchString(s.Name):
		v.add(SeverityError, IssueScreenNameInvalid, s.ID, "", path+"/name",
			"screen name %q must match ^[a-z][a-z0-9_]{0,31}$", clipEcho(s.Name))
	case v.screenNames[s.Name]:
		v.add(SeverityError, IssueScreenNameDuplicate, s.ID, "", path+"/name", "screen name %q is used twice", clipEcho(s.Name))
	}
	v.screenNames[s.Name] = true
	v.text(s.ID, "", path+"/title", "screen title", s.Title)
	if s.Transition != "" && !slices.Contains(transitions, s.Transition) {
		v.add(SeverityError, IssueTransitionInvalid, s.ID, "", path+"/transition", "unknown transition %q", clipEcho(s.Transition))
	}
	v.style(s.ID, "", path+"/style", s.Style)

	count := 0
	walkWidgetList(s, nil, s.Widgets, func(*Screen, *Widget, *Widget) bool { count++; return true })
	if count > MaxWidgetsPerScreen {
		v.add(SeverityError, IssueTooManyWidgets, s.ID, "", path+"/widgets",
			"screen %s has %d widgets, maximum %d", clipEcho(s.Name), count, MaxWidgetsPerScreen)
	}
	areaW, areaH, known := 0, 0, v.board != nil
	if known {
		areaW, areaH = v.board.ScreenSize(v.p.Orientation)
	}
	for i := range s.Widgets {
		v.widget(s, nil, &s.Widgets[i], fmt.Sprintf("%s/widgets/%d", path, i), 1, areaW, areaH, known)
	}
}

// widgetLabel names a widget in messages: its name, else its id, clipped.
func widgetLabel(w *Widget) string {
	if w.Name != "" {
		return clipEcho(w.Name)
	}
	return clipEcho(w.ID)
}

func reservedWidgetName(name string) bool {
	if slices.Contains(reservedNames, name) {
		return true
	}
	for _, p := range reservedNamePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, s := range reservedNameSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func (v *validator) widget(s *Screen, parent *Widget, w *Widget, path string, depth, areaW, areaH int, hasArea bool) {
	label := widgetLabel(w)
	add := func(sev, code, sub, format string, args ...any) {
		v.add(sev, code, s.ID, w.ID, path+sub, format, args...)
	}
	if v.ids[w.ID] {
		add(SeverityError, IssueIDDuplicate, "/id", "id %q is used twice", clipEcho(w.ID))
	}
	v.ids[w.ID] = true
	spec, known := widgetSpecRef(w.Type)
	if !known {
		add(SeverityError, IssueUnknownType, "/type", "widget %s has unknown type %q", label, clipEcho(w.Type))
	}
	if w.Name != "" {
		switch {
		case !namePattern.MatchString(w.Name):
			add(SeverityError, IssueNameInvalid, "/name", "name %q must match ^[a-z][a-z0-9_]{0,31}$", clipEcho(w.Name))
		case reservedWidgetName(w.Name):
			add(SeverityError, IssueNameInvalid, "/name", "name %q is reserved for generated code", clipEcho(w.Name))
		case strings.HasPrefix(w.Name, "img_") && v.assetIDs[strings.TrimPrefix(w.Name, "img_")]:
			add(SeverityError, IssueNameInvalid, "/name", "name %q collides with the image symbol ui_%s of asset %s",
				clipEcho(w.Name), clipEcho(w.Name), clipEcho(strings.TrimPrefix(w.Name, "img_")))
		case v.names[w.Name]:
			add(SeverityError, IssueNameDuplicate, "/name", "name %q is used twice", clipEcho(w.Name))
		}
		v.names[w.Name] = true
	}
	if len(w.Events) > 0 && w.Name == "" {
		add(SeverityError, IssueNameRequired, "/name", "widget %s has events and needs a name", label)
	}

	// Compare against both bounds instead of using an absolute value:
	// -math.MinInt overflows.
	sizeOK := w.W >= 1 && w.H >= 1 && w.W <= maxWidgetSide && w.H <= maxWidgetSide &&
		w.X >= -maxCoordinate && w.X <= maxCoordinate && w.Y >= -maxCoordinate && w.Y <= maxCoordinate
	switch {
	case !sizeOK:
		add(SeverityError, IssueSizeInvalid, "", "widget %s geometry %d,%d %dx%d is invalid", label, w.X, w.Y, w.W, w.H)
	case hasArea && (w.X >= areaW || w.Y >= areaH || w.X+w.W <= 0 || w.Y+w.H <= 0):
		add(SeverityError, IssueOutOfBounds, "", "widget %s at %d,%d %dx%d is outside its %dx%d parent",
			label, w.X, w.Y, w.W, w.H, areaW, areaH)
	case hasArea && (w.X < 0 || w.Y < 0 || w.X+w.W > areaW || w.Y+w.H > areaH):
		add(SeverityWarning, IssuePartlyOutOfBounds, "", "widget %s at %d,%d %dx%d is partly outside its %dx%d parent",
			label, w.X, w.Y, w.W, w.H, areaW, areaH)
	}
	if depth > MaxNesting {
		add(SeverityError, IssueNestingTooDeep, "", "widget %s is nested %d levels deep, maximum %d", label, depth, MaxNesting)
	}
	if parent != nil && parent.Type == "tabview" {
		if tabs := len(v.listProp(parent, "tabs")); w.Tab < 0 || w.Tab >= tabs {
			add(SeverityError, IssueTabInvalid, "/tab", "tab %d does not exist (tabview has %d tabs)", w.Tab, tabs)
		}
	} else if w.Tab != 0 {
		add(SeverityError, IssueTabInvalid, "/tab", "tab is only allowed inside a tabview")
	}
	if len(w.Children) > 0 && known && !spec.Container {
		add(SeverityError, IssueChildrenNotAllowed, "/children", "%s widgets cannot contain children", w.Type)
	}
	if known {
		v.props(s, w, spec, path)
	}
	v.style(s.ID, w.ID, path+"/style", w.Style)
	v.events(s, w, spec, known, path)

	if known && w.Type == "statusbar" && v.p.Settings.WiFi == "off" && v.boolProp(w, spec, "show_clock") {
		add(SeverityWarning, IssueStatusbarClockNeedsWiFi, "/props/show_clock", "status bar %s shows a clock but wifi is off", label)
	}
	if known && w.Type == "canvas" && v.board != nil && !v.board.PSRAM && sizeOK && w.W*w.H*2 > canvasWarnBytes {
		add(SeverityWarning, IssueCanvasLarge, "", "canvas %s needs %d bytes of RAM on a board without PSRAM", label, w.W*w.H*2)
	}

	childW, childH := w.W, w.H
	if w.Type == "tabview" {
		barSize, _ := propInt(v.effectiveProp(w, spec, "bar_size"))
		switch v.effectiveProp(w, spec, "bar_position") {
		case "left", "right":
			childW -= barSize
		default:
			childH -= barSize
		}
	}
	for i := range w.Children {
		v.widget(s, w, &w.Children[i], fmt.Sprintf("%s/children/%d", path, i), depth+1, childW, childH, sizeOK)
	}
}

func coordAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// propValue returns the widget's prop or the catalog default.
func (v *validator) propValue(w *Widget, spec *WidgetSpec, name string) any {
	if val, ok := w.Props[name]; ok {
		return val
	}
	if spec == nil {
		return nil
	}
	if ps, ok := spec.prop(name); ok {
		return ps.Default
	}
	return nil
}

// effectiveProp is propValue for derived checks (the tab count, the child
// area): an explicit value that fails propProblem is already reported on the
// prop itself, so the catalog default stands in for it. Otherwise one bad
// tabview prop would add OUT_OF_BOUNDS or TAB_INVALID to every child.
func (v *validator) effectiveProp(w *Widget, spec *WidgetSpec, name string) any {
	if spec == nil {
		return nil
	}
	ps, ok := spec.prop(name)
	if !ok {
		return nil
	}
	if val, set := w.Props[name]; set {
		if code, _ := v.propProblem(ps, val); code == "" {
			return val
		}
	}
	return ps.Default
}

func (v *validator) boolProp(w *Widget, spec *WidgetSpec, name string) bool {
	b, _ := v.propValue(w, spec, name).(bool)
	return b
}

func (v *validator) listProp(w *Widget, name string) []string {
	spec, _ := widgetSpecRef(w.Type)
	list, _ := propList(v.effectiveProp(w, spec, name))
	return list
}

// propInt accepts the integer forms a layout may carry (Go ints, JSON float64
// without fraction, json.Number).
func propInt(val any) (int, bool) {
	switch n := val.(type) {
	case int:
		return n, true
	case int64:
		return int(n), n >= math.MinInt32 && n <= math.MaxInt32
	case float64:
		if n != math.Trunc(n) || n < math.MinInt32 || n > math.MaxInt32 {
			return 0, false
		}
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil && i >= math.MinInt32 && i <= math.MaxInt32
	}
	return 0, false
}

func propList(val any) ([]string, bool) {
	switch l := val.(type) {
	case []string:
		return l, true
	case []any:
		out := make([]string, len(l))
		for i, item := range l {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// textProblem reports why a device string is unusable ("" when fine); limit
// is the maximum length in bytes.
func textProblem(s string, limit int) (code, reason string) {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return IssuePropInvalid, "is not valid UTF-8 text"
	}
	if len(s) > limit {
		return IssueTextTooLong, fmt.Sprintf("is %d bytes, maximum %d", len(s), limit)
	}
	return "", ""
}

func (v *validator) text(screen, widget, path, what, s string) {
	if code, reason := textProblem(s, MaxTextBytes); code != "" {
		v.add(SeverityError, code, screen, widget, path, "%s %s", what, reason)
	}
}

func (v *validator) props(s *Screen, w *Widget, spec *WidgetSpec, path string) {
	label := widgetLabel(w)
	valid := map[string]bool{}
	unknown := 0
	for _, name := range slices.Sorted(maps.Keys(w.Props)) {
		val := w.Props[name]
		ppath := path + "/props/" + name
		ps, ok := spec.prop(name)
		if !ok {
			if unknown++; unknown <= maxUnknownPropIssues {
				v.add(SeverityError, IssueUnknownProp, s.ID, w.ID, ppath, "%s has no prop %q", w.Type, clipEcho(name))
			}
			continue
		}
		if code, msg := v.propProblem(ps, val); code != "" {
			v.add(SeverityError, code, s.ID, w.ID, ppath, "%s.%s %s", label, name, msg)
			continue
		}
		valid[name] = true
	}
	if unknown > maxUnknownPropIssues {
		v.add(SeverityError, IssueUnknownProp, s.ID, w.ID, path+"/props", "%s has %d more unknown props",
			label, unknown-maxUnknownPropIssues)
	}
	// Cross-prop rules use effective values; skip a rule when one of its
	// explicit inputs was already reported.
	ok := func(names ...string) bool {
		for _, n := range names {
			if _, set := w.Props[n]; set && !valid[n] {
				return false
			}
		}
		return true
	}
	geti := func(name string) int { n, _ := propInt(v.propValue(w, spec, name)); return n }
	getl := func(name string) []string { l, _ := propList(v.propValue(w, spec, name)); return l }
	bad := func(prop, format string, args ...any) {
		v.add(SeverityError, IssuePropInvalid, s.ID, w.ID, path+"/props/"+prop, "%s: "+format, append([]any{label}, args...)...)
	}
	if _, has := spec.prop("min"); has && ok("min", "max") {
		lo, hi := geti("min"), geti("max")
		if lo >= hi {
			bad("min", "min %d must be below max %d", lo, hi)
		} else if _, hasValue := spec.prop("value"); hasValue && ok("value") {
			if val := geti("value"); val < lo || val > hi {
				bad("value", "value %d must be within %d..%d", val, lo, hi)
			}
		}
	}
	if _, has := spec.prop("options"); has && ok("options", "selected") {
		opts := getl("options")
		if len(opts) == 0 {
			bad("options", "needs at least one option")
		} else if sel := geti("selected"); sel < 0 || sel >= len(opts) {
			bad("selected", "selected %d must be within 0..%d", sel, len(opts)-1)
		}
	}
	switch w.Type {
	case "tabview":
		if ok("tabs") && len(getl("tabs")) == 0 {
			bad("tabs", "needs at least one tab")
		}
	case "chart":
		if ok("series") && len(getl("series")) == 0 {
			bad("series", "needs at least one series")
		}
	case "msgbox":
		// LVGL creates the footer with the first button: without one the
		// box could never be closed and lv_msgbox_get_footer stays NULL.
		if ok("buttons") && len(getl("buttons")) == 0 {
			bad("buttons", "a message box needs at least one button")
		}
	case "keypad":
		if ok("layout", "keys") && v.propValue(w, spec, "layout") == "custom" && len(getl("keys")) == 0 {
			bad("keys", "a custom keypad needs keys")
		}
	case "table":
		if ok("cells", "rows", "columns") {
			if n, limit := len(getl("cells")), geti("rows")*geti("columns"); n > limit {
				bad("cells", "%d cells do not fit %d rows x %d columns", n, geti("rows"), geti("columns"))
			}
		}
	case "spinbox":
		if ok("digits", "decimals") && geti("decimals") >= geti("digits") {
			bad("decimals", "decimals %d must be below digits %d", geti("decimals"), geti("digits"))
		} else if ok("digits", "min", "max") {
			limit := int(math.Pow10(geti("digits")))
			if coordAbs(geti("min")) >= limit || coordAbs(geti("max")) >= limit {
				bad("digits", "min/max need more than %d digits", geti("digits"))
			}
		}
	case "textarea":
		if ok("text", "max_length") {
			text, _ := v.propValue(w, spec, "text").(string)
			if n := utf8.RuneCountInString(text); n > geti("max_length") {
				bad("text", "text has %d characters, max_length is %d", n, geti("max_length"))
			}
		}
	}
}

// propProblem checks one prop value against its spec ("" when valid).
func (v *validator) propProblem(ps *PropSpec, val any) (code, msg string) {
	switch ps.Kind {
	case PropString, PropText:
		s, isStr := val.(string)
		if !isStr {
			return IssuePropInvalid, "must be a string"
		}
		limit := ps.MaxLen
		if limit <= 0 {
			limit = MaxTextBytes
		}
		if code, reason := textProblem(s, limit); code != "" {
			return code, reason
		}
	case PropInt:
		n, isInt := propInt(val)
		lo, hi := -intPropLimit, intPropLimit
		if ps.Min != nil {
			lo = *ps.Min
		}
		if ps.Max != nil {
			hi = *ps.Max
		}
		if !isInt || n < lo || n > hi {
			return IssuePropInvalid, fmt.Sprintf("must be an integer within %d..%d", lo, hi)
		}
	case PropBool:
		if _, isBool := val.(bool); !isBool {
			return IssuePropInvalid, "must be true or false"
		}
	case PropEnum:
		if s, _ := val.(string); !slices.Contains(ps.Options, s) {
			return IssuePropInvalid, "must be one of " + strings.Join(ps.Options, ", ")
		}
	case PropColor:
		if s, isStr := val.(string); !isStr || (s != "" && !colorPattern.MatchString(s)) {
			return IssueColorInvalid, "must be #RRGGBB or empty"
		}
	case PropFont:
		if s, isStr := val.(string); !isStr || (s != "" && !slices.Contains(FontFamilies, s)) {
			return IssueFontUnknown, "must be one of " + strings.Join(FontFamilies, ", ") + " or empty"
		}
	case PropFontSize:
		if n, isInt := propInt(val); !isInt || (n != 0 && !slices.Contains(FontSizes, n)) {
			return IssueFontSizeInvalid, fmt.Sprintf("must be one of %v or 0", FontSizes)
		}
	case PropAsset:
		s, isStr := val.(string)
		if !isStr {
			return IssuePropInvalid, "must be an asset id"
		}
		if s != "" && !v.assetIDs[s] {
			return IssueAssetMissing, fmt.Sprintf("references missing asset %q", clipEcho(s))
		}
	case PropSymbol:
		if s, isStr := val.(string); !isStr || (s != "" && !slices.Contains(Symbols, s)) {
			return IssueSymbolUnknown, "is not a known symbol"
		}
	case PropList:
		list, isList := propList(val)
		if !isList {
			return IssuePropInvalid, "must be a list of strings"
		}
		if len(list) > ps.MaxLen {
			return IssuePropInvalid, fmt.Sprintf("has %d items, maximum %d", len(list), ps.MaxLen)
		}
		for i, item := range list {
			if code, reason := textProblem(item, MaxTextBytes); code != "" {
				return code, fmt.Sprintf("item %d %s", i, reason)
			}
		}
	}
	return "", ""
}

func (v *validator) style(screen, widget, path string, st Style) {
	colors := []struct{ key, val string }{
		{"bg_color", st.BgColor}, {"text_color", st.TextColor}, {"accent_color", st.AccentColor}, {"border_color", st.BorderColor},
	}
	for _, c := range colors {
		if c.val != "" && !colorPattern.MatchString(c.val) {
			v.add(SeverityError, IssueColorInvalid, screen, widget, path+"/"+c.key, "style.%s %q must be #RRGGBB", c.key, clipEcho(c.val))
		}
	}
	ranges := []struct {
		key    string
		val    *int
		lo, hi int
	}{
		{"bg_opacity", st.BgOpacity, 0, 255}, {"border_width", st.BorderWidth, 0, 8}, {"radius", st.Radius, 0, 64},
	}
	for _, r := range ranges {
		if r.val != nil && (*r.val < r.lo || *r.val > r.hi) {
			v.add(SeverityError, IssuePropInvalid, screen, widget, path+"/"+r.key, "style.%s %d must be within %d..%d",
				r.key, *r.val, r.lo, r.hi)
		}
	}
}

func (v *validator) events(s *Screen, w *Widget, spec *WidgetSpec, known bool, path string) {
	label := widgetLabel(w)
	seen := map[string]bool{}
	for i, ev := range w.Events {
		epath := fmt.Sprintf("%s/events/%d", path, i)
		add := func(code, sub, format string, args ...any) {
			v.add(SeverityError, code, s.ID, w.ID, epath+sub, format, args...)
		}
		switch {
		case known && !slices.Contains(spec.Events, ev.On):
			add(IssueEventNotAllowed, "/on", "%s widgets have no %q event", w.Type, clipEcho(ev.On))
		case seen[ev.On]:
			add(IssueEventNotAllowed, "/on", "event %q of %s is bound twice", clipEcho(ev.On), label)
		}
		seen[ev.On] = true
		a := ev.Action
		switch a.Type {
		case "goto_screen":
			if a.Screen == "" {
				add(IssueActionInvalid, "/action/screen", "goto_screen needs a target screen")
			} else if !v.screenIDs[a.Screen] {
				add(IssueScreenTargetMissing, "/action/screen", "goto_screen target %q does not exist", clipEcho(a.Screen))
			}
			if a.Transition != "" && !slices.Contains(transitions, a.Transition) {
				add(IssueTransitionInvalid, "/action/transition", "unknown transition %q", clipEcho(a.Transition))
			}
			if a.Hint != "" {
				add(IssueActionInvalid, "/action/hint", "hint is only allowed for logic actions")
			}
		case "back", "open_wifi_setup", "logic":
			if a.Screen != "" || a.Transition != "" {
				add(IssueActionInvalid, "/action", "%s actions take no screen or transition", a.Type)
			}
			if a.Type != "logic" && a.Hint != "" {
				add(IssueActionInvalid, "/action/hint", "hint is only allowed for logic actions")
			}
			if a.Type == "open_wifi_setup" && v.p.Settings.WiFi == "off" {
				add(IssueActionInvalid, "/action/type", "open_wifi_setup needs Wi-Fi (wifi is off)")
			}
			if a.Type == "logic" {
				if !utf8.ValidString(a.Hint) || strings.ContainsRune(a.Hint, 0) {
					add(IssueActionInvalid, "/action/hint", "hint is not valid UTF-8 text")
				} else if n := utf8.RuneCountInString(a.Hint); n > MaxHintRunes {
					add(IssueHintTooLong, "/action/hint", "hint has %d characters, maximum %d", n, MaxHintRunes)
				}
			}
		default:
			add(IssueActionInvalid, "/action/type", "action type %q must be one of %s", clipEcho(a.Type), strings.Join(actionTypes, ", "))
		}
	}
}

func (v *validator) assets() {
	seen := map[string]bool{}
	var total int64
	for i, a := range v.p.Assets {
		path := fmt.Sprintf("assets/%d", i)
		if seen[a.ID] {
			v.add(SeverityError, IssueIDDuplicate, "", "", path+"/id", "asset id %q is used twice", clipEcho(a.ID))
		}
		seen[a.ID] = true
		if !assetSymbolPattern.MatchString(a.ID) {
			v.add(SeverityError, IssuePropInvalid, "", "", path+"/id",
				"asset id %q may only contain letters, digits and _ (it becomes the C symbol ui_img_<id>)", clipEcho(a.ID))
		}
		if !assetFileRegex.MatchString(a.File) {
			v.add(SeverityError, IssuePropInvalid, "", "", path+"/file", "asset file %q must match %s", clipEcho(a.File), assetFileRegex)
		}
		switch {
		case a.W < 1 || a.H < 1:
			v.add(SeverityError, IssueSizeInvalid, "", "", path, "asset %s has invalid size %dx%d", clipEcho(a.ID), a.W, a.H)
		case v.board != nil:
			long, short := max(v.board.Display.Width, v.board.Display.Height), min(v.board.Display.Width, v.board.Display.Height)
			if max(a.W, a.H) > long || min(a.W, a.H) > short {
				v.add(SeverityError, IssueSizeInvalid, "", "", path, "asset %s (%dx%d) is larger than the %dx%d display",
					clipEcho(a.ID), a.W, a.H, long, short)
			}
		}
		total += v.opts.AssetSizes[a.ID]
	}
	if v.opts.AssetSizes != nil && v.board != nil {
		if budget := AssetBudget(v.board.FlashMB); total > budget {
			v.add(SeverityError, IssueAssetBudgetExceeded, "", "", "assets",
				"images need %d bytes, the budget for %d MB flash is %d", total, v.board.FlashMB, budget)
		}
	}
}
