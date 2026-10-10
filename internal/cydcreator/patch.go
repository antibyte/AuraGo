package cydcreator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// LayoutOp is one edit of cyd_creator_update_layout (and of the designer's
// mirrored JS model). Targets (Screen, Parent, ID) accept an id or a name; ids
// are matched first.
type LayoutOp struct {
	Op         string         `json:"op"` // add_widget update_widget move_widget delete_widget add_screen update_screen delete_screen set_theme set_settings
	Screen     string         `json:"screen,omitempty"`
	Parent     string         `json:"parent,omitempty"` // container widget ("" = screen root)
	Tab        *int           `json:"tab,omitempty"`
	ID         string         `json:"id,omitempty"` // target widget/screen
	Widget     *Widget        `json:"widget,omitempty"`
	ScreenData *Screen        `json:"screen_data,omitempty"`
	Set        map[string]any `json:"set,omitempty"` // merge: top-level fields; "props"/"style" merge one level deep, null deletes
	X          *int           `json:"x,omitempty"`
	Y          *int           `json:"y,omitempty"`
	Theme      string         `json:"theme,omitempty"`
}

// OpResult reports one applied op.
type OpResult struct {
	Index   int    `json:"index"`
	Op      string `json:"op"`
	ID      string `json:"id"` // id of the created/changed element
	Summary string `json:"summary"`
}

// ApplyOps applies ops atomically: on the first failing op nothing changes and
// a *Error CYD_INVALID_LAYOUT with details {"index": i, "op": ..., "reason":
// ...} is returned (plus "code" and "path" when a validation error caused it).
// An op fails when it is structurally impossible or when it introduces a
// validation error that the layout did not have before; warnings and
// pre-existing errors never block. Added widgets and screens always get fresh
// ids from ids (nil = RandomIDs); goto_screen actions may name the target
// screen, which is stored as its id.
func ApplyOps(p Project, ops []LayoutOp, ids IDSource) (Project, []OpResult, error) {
	if ids == nil {
		ids = RandomIDs{}
	}
	if len(ops) > maxLayoutOps {
		return p, nil, opFailure(maxLayoutOps, "", fmt.Sprintf("too many ops (maximum %d per call)", maxLayoutOps), nil)
	}
	work := cloneProject(p)
	known := errorKeys(&work, Validate(work, ValidateOptions{}))
	results := make([]OpResult, 0, len(ops))
	for i, op := range ops {
		res, err := applyOp(&work, op, ids)
		if err != nil {
			return p, nil, opFailure(i, op.Op, err.Error(), nil)
		}
		issues := Validate(work, ValidateOptions{})
		for _, is := range issues {
			if is.Severity == SeverityError && !known[issueKey(&work, is)] {
				return p, nil, opFailure(i, op.Op, is.Message, &is)
			}
		}
		known = errorKeys(&work, issues)
		res.Index, res.Op = i, op.Op
		results = append(results, res)
	}
	return work, results, nil
}

func opFailure(index int, op, reason string, is *Issue) *Error {
	details := map[string]any{"index": index, "op": op, "reason": reason}
	if is != nil {
		details["code"] = is.Code
		details["path"] = is.Path
	}
	return NewError(CodeInvalidLayout, fmt.Sprintf("layout op %d (%s) failed: %s", index, op, reason), details)
}

// issuePathPrefix matches the index-based part of an issue path so keys stay
// stable when widgets or screens are inserted, moved or deleted.
var issuePathPrefix = regexp.MustCompile(`^screens/\d+(/(widgets|children)/\d+)*`)

// duplicatePathPattern matches the path of a duplicate-name or duplicate-id
// issue: screens/N[/widgets/I[/children/J...]]/name|id or assets/N/id.
var duplicatePathPattern = regexp.MustCompile(`^(screens|assets)/(\d+)((?:/(?:widgets|children)/\d+)*)/(name|id)$`)

var pathIndexPattern = regexp.MustCompile(`\d+`)

// duplicatedValue resolves the path of a duplicate issue against the project
// the issue came from and returns the name or id that is used twice, tagged
// with its namespace (screen ids and widget ids share one, widget names,
// screen names and asset ids have their own).
func duplicatedValue(p *Project, path string) (string, bool) {
	m := duplicatePathPattern.FindStringSubmatch(path)
	if m == nil {
		return "", false
	}
	idx, err := strconv.Atoi(m[2])
	if err != nil {
		return "", false
	}
	if m[1] == "assets" {
		if m[4] != "id" || idx >= len(p.Assets) {
			return "", false
		}
		return "asset_id\x00" + p.Assets[idx].ID, true
	}
	if idx >= len(p.Screens) {
		return "", false
	}
	s := &p.Screens[idx]
	if m[3] == "" {
		if m[4] == "name" {
			return "screen_name\x00" + s.Name, true
		}
		return "id\x00" + s.ID, true
	}
	list := s.Widgets
	var w *Widget
	for _, seg := range pathIndexPattern.FindAllString(m[3], -1) {
		i, err := strconv.Atoi(seg)
		if err != nil || i >= len(list) {
			return "", false
		}
		w = &list[i]
		list = w.Children
	}
	if w == nil {
		return "", false
	}
	if m[4] == "name" {
		return "widget_name\x00" + w.Name, true
	}
	return "id\x00" + w.ID, true
}

// issueKey identifies a validation error across ops; p is the project the
// issue was produced for. Most keys are code + screen id + widget id + the path
// below the element. Two families are keyed differently because the element an
// issue is attributed to is not stable:
//   - NAME_DUPLICATE, ID_DUPLICATE and SCREEN_NAME_DUPLICATE land on the later
//     of the clashing elements, which changes when elements are reordered or
//     moved; they are keyed by code + the duplicated name or id.
//   - UNKNOWN_PROP lists at most maxUnknownPropIssues props per widget (the
//     rest is summarised), so removing one prop changes which props are
//     listed; it is keyed by code + screen + widget.
func issueKey(p *Project, is Issue) string {
	switch is.Code {
	case IssueNameDuplicate, IssueIDDuplicate, IssueScreenNameDuplicate:
		if v, ok := duplicatedValue(p, is.Path); ok {
			return is.Code + "\x00dup\x00" + v
		}
	case IssueUnknownProp:
		return is.Code + "\x00" + is.Screen + "\x00" + is.Widget
	}
	return is.Code + "\x00" + is.Screen + "\x00" + is.Widget + "\x00" + issuePathPrefix.ReplaceAllString(is.Path, "")
}

func errorKeys(p *Project, issues []Issue) map[string]bool {
	keys := map[string]bool{}
	for _, is := range issues {
		if is.Severity == SeverityError {
			keys[issueKey(p, is)] = true
		}
	}
	return keys
}

func applyOp(p *Project, op LayoutOp, ids IDSource) (OpResult, error) {
	switch op.Op {
	case "add_widget":
		return opAddWidget(p, op, ids)
	case "update_widget":
		return opUpdateWidget(p, op)
	case "move_widget":
		return opMoveWidget(p, op)
	case "delete_widget":
		return opDeleteWidget(p, op)
	case "add_screen":
		return opAddScreen(p, op, ids)
	case "update_screen":
		return opUpdateScreen(p, op)
	case "delete_screen":
		return opDeleteScreen(p, op)
	case "set_theme":
		if _, err := ThemeByID(op.Theme); err != nil {
			return OpResult{}, fmt.Errorf("unknown theme %q", op.Theme)
		}
		p.Theme = op.Theme
		return OpResult{ID: op.Theme, Summary: "theme set to " + op.Theme}, nil
	case "set_settings":
		return opSetSettings(p, op)
	}
	return OpResult{}, fmt.Errorf("unknown op %q", op.Op)
}

// ---- lookup helpers ----

func (p *Project) screenIndex(idOrName string) (int, error) {
	if idOrName == "" {
		return -1, errors.New("screen is required")
	}
	for i := range p.Screens {
		if p.Screens[i].ID == idOrName {
			return i, nil
		}
	}
	for i := range p.Screens {
		if p.Screens[i].Name == idOrName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("screen %q not found", idOrName)
}

type widgetLoc struct {
	screen int
	path   []int
}

func (p *Project) widgetAt(loc widgetLoc) *Widget {
	list := &p.Screens[loc.screen].Widgets
	var w *Widget
	for _, i := range loc.path {
		w = &(*list)[i]
		list = &w.Children
	}
	return w
}

// parentList returns the slice that holds the widget at loc and its parent (nil = screen root).
func (p *Project) parentList(loc widgetLoc) (*[]Widget, *Widget) {
	if len(loc.path) == 1 {
		return &p.Screens[loc.screen].Widgets, nil
	}
	parent := p.widgetAt(widgetLoc{loc.screen, loc.path[:len(loc.path)-1]})
	return &parent.Children, parent
}

// findWidgetLoc finds a widget by id, then name; screen ("" = all) restricts the search.
func (p *Project) findWidgetLoc(idOrName, screen string) (widgetLoc, error) {
	if idOrName == "" {
		return widgetLoc{}, errors.New("id is required")
	}
	only := -1
	if screen != "" {
		si, err := p.screenIndex(screen)
		if err != nil {
			return widgetLoc{}, err
		}
		only = si
	}
	for _, match := range []func(*Widget) bool{
		func(w *Widget) bool { return w.ID == idOrName },
		func(w *Widget) bool { return w.Name == idOrName },
	} {
		for si := range p.Screens {
			if only >= 0 && si != only {
				continue
			}
			if path := findWidgetPath(p.Screens[si].Widgets, match); path != nil {
				return widgetLoc{si, path}, nil
			}
		}
	}
	return widgetLoc{}, fmt.Errorf("widget %q not found", idOrName)
}

func (p *Project) usedIDs() map[string]bool {
	used := map[string]bool{}
	for _, s := range p.Screens {
		used[s.ID] = true
	}
	for _, a := range p.Assets {
		used[a.ID] = true
	}
	p.WalkWidgets(func(_ *Screen, _ *Widget, w *Widget) bool { used[w.ID] = true; return true })
	return used
}

func freshID(ids IDSource, prefix string, used map[string]bool) (string, error) {
	for i := 0; i < 1000; i++ {
		id := ids.NewID(prefix)
		if validElementID(id) && !used[id] {
			used[id] = true
			return id, nil
		}
	}
	return "", errors.New("could not allocate a unique id")
}

// prepareNewWidget assigns fresh ids and default sizes to w and its children.
func prepareNewWidget(w *Widget, ids IDSource, used map[string]bool, areaW int) error {
	id, err := freshID(ids, "w", used)
	if err != nil {
		return err
	}
	w.ID = id
	if spec, ok := widgetSpecRef(w.Type); ok {
		if w.W == 0 {
			w.W = spec.DefaultW
			if w.W == 0 {
				w.W = areaW
			}
		}
		if w.H == 0 {
			w.H = spec.DefaultH
		}
	}
	for i := range w.Children {
		if err := prepareNewWidget(&w.Children[i], ids, used, w.W); err != nil {
			return err
		}
	}
	return nil
}

// resolveActionScreens replaces screen names in goto_screen actions by ids.
func resolveActionScreens(p *Project, w *Widget) {
	for i := range w.Events {
		a := &w.Events[i].Action
		if a.Type != "goto_screen" || a.Screen == "" {
			continue
		}
		if si, err := p.screenIndex(a.Screen); err == nil {
			a.Screen = p.Screens[si].ID
		}
	}
	for i := range w.Children {
		resolveActionScreens(p, &w.Children[i])
	}
}

func (p *Project) screenWidth() int {
	if b, err := BoardByID(p.Board); err == nil {
		w, _ := b.ScreenSize(p.Orientation)
		return w
	}
	return 0
}

func describeWidget(w *Widget) string { return w.Type + " " + widgetLabel(w) }

// ---- widget ops ----

func opAddWidget(p *Project, op LayoutOp, ids IDSource) (OpResult, error) {
	si, err := p.screenIndex(op.Screen)
	if err != nil {
		return OpResult{}, err
	}
	if op.Widget == nil {
		return OpResult{}, errors.New("widget is required")
	}
	w := cloneWidget(*op.Widget)
	if _, ok := widgetSpecRef(w.Type); !ok {
		return OpResult{}, fmt.Errorf("unknown widget type %q", w.Type)
	}
	if op.X != nil {
		w.X = *op.X
	}
	if op.Y != nil {
		w.Y = *op.Y
	}
	list, where := &p.Screens[si].Widgets, p.Screens[si].Name
	areaW := p.screenWidth()
	tab := w.Tab
	if op.Tab != nil {
		tab = *op.Tab
	}
	w.Tab = 0
	if op.Parent != "" {
		loc, err := p.findWidgetLoc(op.Parent, p.Screens[si].ID)
		if err != nil {
			return OpResult{}, fmt.Errorf("parent: %w", err)
		}
		parent := p.widgetAt(loc)
		if spec, _ := widgetSpecRef(parent.Type); spec == nil || !spec.Container {
			return OpResult{}, fmt.Errorf("parent %s cannot contain widgets", describeWidget(parent))
		}
		if parent.Type == "tabview" {
			w.Tab, tab = tab, 0
		}
		list, where, areaW = &parent.Children, where+"/"+widgetLabel(parent), parent.W
	}
	if tab != 0 {
		return OpResult{}, errors.New("tab is only allowed when the parent is a tabview")
	}
	if err := prepareNewWidget(&w, ids, p.usedIDs(), areaW); err != nil {
		return OpResult{}, err
	}
	resolveActionScreens(p, &w)
	*list = append(*list, w)
	return OpResult{ID: w.ID, Summary: fmt.Sprintf("added %s to %s", describeWidget(&w), where)}, nil
}

func opUpdateWidget(p *Project, op LayoutOp) (OpResult, error) {
	loc, err := p.findWidgetLoc(op.ID, op.Screen)
	if err != nil {
		return OpResult{}, err
	}
	if len(op.Set) == 0 {
		return OpResult{}, errors.New("set is empty")
	}
	w := p.widgetAt(loc)
	updated := cloneWidget(*w)
	keys := slices.Sorted(maps.Keys(op.Set))
	for _, k := range keys {
		if err := setWidgetField(&updated, k, op.Set[k]); err != nil {
			return OpResult{}, fmt.Errorf("set.%s: %w", k, err)
		}
	}
	resolveActionScreens(p, &updated)
	*w = updated
	return OpResult{ID: w.ID, Summary: fmt.Sprintf("updated %s (%s)", describeWidget(w), strings.Join(keys, ", "))}, nil
}

func setWidgetField(w *Widget, key string, val any) error {
	var err error
	switch key {
	case "name":
		w.Name, err = setString(val)
	case "x":
		w.X, err = setInt(val)
	case "y":
		w.Y, err = setInt(val)
	case "w":
		w.W, err = setInt(val)
	case "h":
		w.H, err = setInt(val)
	case "tab":
		w.Tab, err = setInt(val)
	case "locked":
		w.Locked, err = setBool(val)
	case "hidden":
		w.Hidden, err = setBool(val)
	case "props":
		w.Props, err = mergeProps(w.Props, val)
	case "style":
		err = mergeStyle(&w.Style, val)
	case "events":
		var events []EventBinding
		if val != nil {
			err = decodeStrict(val, &events)
		}
		w.Events = events
	case "id", "type", "children":
		err = fmt.Errorf("%s cannot be changed with update_widget", key)
	default:
		err = fmt.Errorf("unknown widget field %q", key)
	}
	return err
}

func opMoveWidget(p *Project, op LayoutOp) (OpResult, error) {
	loc, err := p.findWidgetLoc(op.ID, "")
	if err != nil {
		return OpResult{}, err
	}
	if op.X == nil && op.Y == nil && op.Screen == "" && op.Parent == "" {
		return OpResult{}, errors.New("move_widget needs x/y, screen or parent")
	}
	w := p.widgetAt(loc)
	if op.Screen == "" && op.Parent == "" {
		applyXY(w, op)
		return OpResult{ID: w.ID, Summary: fmt.Sprintf("moved %s to %d,%d", describeWidget(w), w.X, w.Y)}, nil
	}
	destScreen := loc.screen
	if op.Screen != "" {
		if destScreen, err = p.screenIndex(op.Screen); err != nil {
			return OpResult{}, err
		}
	}
	moved := cloneWidget(*w)
	if op.Parent != "" {
		ploc, err := p.findWidgetLoc(op.Parent, p.Screens[destScreen].ID)
		if err != nil {
			return OpResult{}, fmt.Errorf("parent: %w", err)
		}
		if ploc.screen == loc.screen && len(ploc.path) >= len(loc.path) && slices.Equal(ploc.path[:len(loc.path)], loc.path) {
			return OpResult{}, errors.New("cannot move a widget into itself")
		}
	}
	// Detach first; the parent is looked up again afterwards because removing
	// the widget can shift sibling indexes.
	list, _ := p.parentList(loc)
	idx := loc.path[len(loc.path)-1]
	*list = slices.Delete(*list, idx, idx+1)
	dest, where := &p.Screens[destScreen].Widgets, p.Screens[destScreen].Name
	tab := 0
	if op.Tab != nil {
		tab = *op.Tab
	}
	moved.Tab = 0
	if op.Parent != "" {
		ploc, _ := p.findWidgetLoc(op.Parent, p.Screens[destScreen].ID)
		parent := p.widgetAt(ploc)
		if spec, _ := widgetSpecRef(parent.Type); spec == nil || !spec.Container {
			return OpResult{}, fmt.Errorf("parent %s cannot contain widgets", describeWidget(parent))
		}
		if parent.Type == "tabview" {
			moved.Tab, tab = tab, 0
		}
		dest, where = &parent.Children, where+"/"+widgetLabel(parent)
	}
	if tab != 0 {
		return OpResult{}, errors.New("tab is only allowed when the parent is a tabview")
	}
	applyXY(&moved, op)
	*dest = append(*dest, moved)
	return OpResult{ID: moved.ID, Summary: fmt.Sprintf("moved %s to %s at %d,%d", describeWidget(&moved), where, moved.X, moved.Y)}, nil
}

func applyXY(w *Widget, op LayoutOp) {
	if op.X != nil {
		w.X = *op.X
	}
	if op.Y != nil {
		w.Y = *op.Y
	}
}

func opDeleteWidget(p *Project, op LayoutOp) (OpResult, error) {
	loc, err := p.findWidgetLoc(op.ID, op.Screen)
	if err != nil {
		return OpResult{}, err
	}
	w := p.widgetAt(loc)
	id, desc := w.ID, describeWidget(w)
	children := -1
	walkWidgetList(nil, nil, []Widget{*w}, func(*Screen, *Widget, *Widget) bool { children++; return true })
	list, _ := p.parentList(loc)
	idx := loc.path[len(loc.path)-1]
	*list = slices.Delete(*list, idx, idx+1)
	summary := "deleted " + desc
	if children > 0 {
		summary += fmt.Sprintf(" and %d nested widgets", children)
	}
	return OpResult{ID: id, Summary: summary}, nil
}

// ---- screen ops ----

func opAddScreen(p *Project, op LayoutOp, ids IDSource) (OpResult, error) {
	if op.ScreenData == nil {
		return OpResult{}, errors.New("screen_data is required")
	}
	s := cloneScreen(*op.ScreenData)
	used := p.usedIDs()
	id, err := freshID(ids, "s", used)
	if err != nil {
		return OpResult{}, err
	}
	s.ID = id
	if s.Transition == "" {
		s.Transition = "none"
	}
	if s.Widgets == nil {
		s.Widgets = []Widget{}
	}
	areaW := p.screenWidth()
	for i := range s.Widgets {
		if err := prepareNewWidget(&s.Widgets[i], ids, used, areaW); err != nil {
			return OpResult{}, err
		}
	}
	p.Screens = append(p.Screens, s)
	added := &p.Screens[len(p.Screens)-1]
	for i := range added.Widgets {
		resolveActionScreens(p, &added.Widgets[i])
	}
	if p.StartScreen == "" {
		p.StartScreen = s.ID
	}
	return OpResult{ID: s.ID, Summary: "added screen " + s.Name}, nil
}

func opUpdateScreen(p *Project, op LayoutOp) (OpResult, error) {
	target := op.ID
	if target == "" {
		target = op.Screen
	}
	si, err := p.screenIndex(target)
	if err != nil {
		return OpResult{}, err
	}
	if len(op.Set) == 0 {
		return OpResult{}, errors.New("set is empty")
	}
	s := cloneScreen(p.Screens[si])
	keys := slices.Sorted(maps.Keys(op.Set))
	for _, k := range keys {
		val := op.Set[k]
		switch k {
		case "name":
			s.Name, err = setString(val)
		case "title":
			s.Title, err = setString(val)
		case "transition":
			s.Transition, err = setString(val)
		case "style":
			err = mergeStyle(&s.Style, val)
		case "id", "widgets":
			err = fmt.Errorf("%s cannot be changed with update_screen", k)
		default:
			err = fmt.Errorf("unknown screen field %q", k)
		}
		if err != nil {
			return OpResult{}, fmt.Errorf("set.%s: %w", k, err)
		}
	}
	p.Screens[si] = s
	return OpResult{ID: s.ID, Summary: fmt.Sprintf("updated screen %s (%s)", s.Name, strings.Join(keys, ", "))}, nil
}

func opDeleteScreen(p *Project, op LayoutOp) (OpResult, error) {
	target := op.ID
	if target == "" {
		target = op.Screen
	}
	si, err := p.screenIndex(target)
	if err != nil {
		return OpResult{}, err
	}
	s := p.Screens[si]
	p.Screens = slices.Delete(p.Screens, si, si+1)
	if p.StartScreen == s.ID {
		p.StartScreen = ""
		if len(p.Screens) > 0 {
			p.StartScreen = p.Screens[0].ID
		}
	}
	return OpResult{ID: s.ID, Summary: "deleted screen " + s.Name}, nil
}

func opSetSettings(p *Project, op LayoutOp) (OpResult, error) {
	if len(op.Set) == 0 {
		return OpResult{}, errors.New("set is empty")
	}
	st := p.Settings
	keys := slices.Sorted(maps.Keys(op.Set))
	for _, k := range keys {
		var err error
		val := op.Set[k]
		switch k {
		case "wifi":
			st.WiFi, err = setString(val)
		case "aurago_link":
			st.AuraGoLink, err = setBool(val)
		case "libraries":
			st.Libraries, err = setStringList(val)
		case "backlight":
			st.Backlight, err = setInt(val)
		case "statusbar_clock":
			st.StatusbarClock, err = setBool(val)
		default:
			err = fmt.Errorf("unknown settings field %q", k)
		}
		if err != nil {
			return OpResult{}, fmt.Errorf("set.%s: %w", k, err)
		}
	}
	p.Settings = st
	return OpResult{Summary: "settings updated (" + strings.Join(keys, ", ") + ")"}, nil
}

// ---- value conversion for Set ----

func setString(val any) (string, error) {
	s, ok := val.(string)
	if !ok {
		return "", errors.New("must be a string")
	}
	return s, nil
}

func setBool(val any) (bool, error) {
	b, ok := val.(bool)
	if !ok {
		return false, errors.New("must be true or false")
	}
	return b, nil
}

func setInt(val any) (int, error) {
	n, ok := propInt(val)
	if !ok {
		return 0, errors.New("must be an integer")
	}
	return n, nil
}

func setStringList(val any) ([]string, error) {
	if val == nil {
		return []string{}, nil
	}
	list, ok := propList(val)
	if !ok {
		return nil, errors.New("must be a list of strings")
	}
	return slices.Clone(list), nil
}

func mergeProps(props map[string]any, val any) (map[string]any, error) {
	if val == nil {
		return nil, nil // reset every prop to its default
	}
	set, ok := val.(map[string]any)
	if !ok {
		return nil, errors.New("must be an object")
	}
	out := maps.Clone(props)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range set {
		if v == nil {
			delete(out, k)
		} else {
			out[k] = cloneValue(v)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func mergeStyle(st *Style, val any) error {
	if val == nil {
		*st = Style{}
		return nil
	}
	set, ok := val.(map[string]any)
	if !ok {
		return errors.New("must be an object")
	}
	for _, k := range slices.Sorted(maps.Keys(set)) {
		v := set[k]
		var err error
		switch k {
		case "bg_color":
			st.BgColor, err = styleColor(v)
		case "text_color":
			st.TextColor, err = styleColor(v)
		case "accent_color":
			st.AccentColor, err = styleColor(v)
		case "border_color":
			st.BorderColor, err = styleColor(v)
		case "bg_opacity":
			st.BgOpacity, err = styleInt(v)
		case "border_width":
			st.BorderWidth, err = styleInt(v)
		case "radius":
			st.Radius, err = styleInt(v)
		default:
			err = fmt.Errorf("unknown style field %q", k)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

func styleColor(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	return setString(v)
}

func styleInt(v any) (*int, error) {
	if v == nil {
		return nil, nil
	}
	n, err := setInt(v)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// decodeStrict converts a generic JSON value (or Go value) into dst, rejecting unknown fields.
func decodeStrict(val any, dst any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

// ---- deep copies ----

func cloneProject(p Project) Project {
	out := p
	out.Settings.Libraries = slices.Clone(p.Settings.Libraries)
	out.Screens = make([]Screen, len(p.Screens))
	for i, s := range p.Screens {
		out.Screens[i] = cloneScreen(s)
	}
	out.Assets = slices.Clone(p.Assets)
	if p.Screens == nil {
		out.Screens = nil
	}
	return out
}

func cloneScreen(s Screen) Screen {
	out := s
	out.Style = cloneStyle(s.Style)
	if s.Widgets != nil {
		out.Widgets = make([]Widget, len(s.Widgets))
		for i, w := range s.Widgets {
			out.Widgets[i] = cloneWidget(w)
		}
	}
	return out
}

func cloneWidget(w Widget) Widget {
	out := w
	out.Style = cloneStyle(w.Style)
	if w.Props != nil {
		out.Props = make(map[string]any, len(w.Props))
		for k, v := range w.Props {
			out.Props[k] = cloneValue(v)
		}
	}
	out.Events = slices.Clone(w.Events)
	if w.Children != nil {
		out.Children = make([]Widget, len(w.Children))
		for i, c := range w.Children {
			out.Children[i] = cloneWidget(c)
		}
	}
	return out
}

func cloneStyle(s Style) Style {
	out := s
	out.BgOpacity = dupIntPtr(s.BgOpacity)
	out.BorderWidth = dupIntPtr(s.BorderWidth)
	out.Radius = dupIntPtr(s.Radius)
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = cloneValue(item)
		}
		return out
	case []string:
		return slices.Clone(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = cloneValue(item)
		}
		return out
	}
	return v
}
