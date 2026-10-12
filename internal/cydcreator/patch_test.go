package cydcreator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func opInt(v int) *int { return &v }

// patchFixture: home (s_0001, start) with lbl w_0001, pnl w_0002, tabs w_0003,
// btn_go w_0004 (clicked → details); details s_0002 empty.
func patchFixture(t *testing.T) (Project, *SeqIDs) {
	t.Helper()
	ids := &SeqIDs{}
	p, err := NewProject("Demo", "esp32-2432s028r", "clear", "en", "", ids)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err = ApplyOps(p, []LayoutOp{
		{Op: "add_screen", ScreenData: &Screen{Name: "details", Title: "Details"}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "label", Name: "lbl", X: 10, Y: 10}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "panel", Name: "pnl", X: 0, Y: 40}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "tabview", Name: "tabs"}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "button", Name: "btn_go", X: 200, Y: 200,
			Events: []EventBinding{{On: "clicked", Action: Action{Type: "goto_screen", Screen: "details"}}}}},
	}, ids)
	if err != nil {
		t.Fatal(err)
	}
	return p, ids
}

func mustMarshal(t *testing.T, p Project) string {
	t.Helper()
	b, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyOpsFixtureShape(t *testing.T) {
	p, _ := patchFixture(t)
	if len(p.Screens) != 2 || p.Screens[1].ID != "s_0002" || p.Screens[1].Transition != "none" {
		t.Fatalf("screens: %+v", p.Screens)
	}
	got := []string{}
	for _, w := range p.Screens[0].Widgets {
		got = append(got, w.ID+":"+w.Name)
	}
	if strings.Join(got, ",") != "w_0001:lbl,w_0002:pnl,w_0003:tabs,w_0004:btn_go" {
		t.Fatalf("widgets: %v", got)
	}
	if btn := p.Screens[0].Widgets[3]; btn.W != 120 || btn.H != 40 || btn.Events[0].Action.Screen != "s_0002" {
		t.Fatalf("defaults/name resolution: %+v", btn)
	}
	if issues := Validate(p, ValidateOptions{}); len(issues) != 0 {
		t.Fatalf("fixture issues:\n%s", fmtIssues(issues))
	}
}

func TestApplyOpsAllOps(t *testing.T) {
	ids := &SeqIDs{}
	p, err := NewProject("Demo", "esp32-2432s028r", "clear", "en", "", ids)
	if err != nil {
		t.Fatal(err)
	}
	ops := []LayoutOp{
		{Op: "add_screen", ScreenData: &Screen{Name: "details", Title: "Details"}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "label", Name: "lbl_temp", X: 10, Y: 10, Props: map[string]any{"text": "21 °C"}}},
		{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "panel", Name: "pnl_box", Y: 60}},
		{Op: "add_widget", Screen: "home", Parent: "pnl_box", Widget: &Widget{Type: "button", Name: "btn_go", X: 4, Y: 4,
			Events: []EventBinding{{On: "clicked", Action: Action{Type: "goto_screen", Screen: "details"}}}}},
		{Op: "update_widget", ID: "lbl_temp", Set: map[string]any{"props": map[string]any{"size": 28}, "x": 12}},
		{Op: "move_widget", ID: "btn_go", Screen: "home", X: opInt(200), Y: opInt(10)},
		{Op: "delete_widget", ID: "w_0002"},
		{Op: "update_screen", ID: "details", Set: map[string]any{"title": "Mehr", "transition": "slide_left"}},
		{Op: "set_theme", Theme: "synthwave"},
		{Op: "set_settings", Set: map[string]any{"wifi": "required", "libraries": []any{"arduinojson"}, "aurago_link": true}},
		{Op: "add_screen", ScreenData: &Screen{Name: "spare"}},
		{Op: "delete_screen", ID: "spare"},
	}
	out, results, err := ApplyOps(p, ops, ids)
	if err != nil {
		t.Fatal(err)
	}
	want := []OpResult{
		{0, "add_screen", "s_0002", "added screen details"},
		{1, "add_widget", "w_0001", "added label lbl_temp to home"},
		{2, "add_widget", "w_0002", "added panel pnl_box to home"},
		{3, "add_widget", "w_0003", "added button btn_go to home/pnl_box"},
		{4, "update_widget", "w_0001", "updated label lbl_temp (props, x)"},
		{5, "move_widget", "w_0003", "moved button btn_go to home at 200,10"},
		{6, "delete_widget", "w_0002", "deleted panel pnl_box"},
		{7, "update_screen", "s_0002", "updated screen details (title, transition)"},
		{8, "set_theme", "synthwave", "theme set to synthwave"},
		{9, "set_settings", "", "settings updated (aurago_link, libraries, wifi)"},
		{10, "add_screen", "s_0003", "added screen spare"},
		{11, "delete_screen", "s_0003", "deleted screen spare"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results:\n%+v\nwant\n%+v", results, want)
	}
	home := out.Screens[0]
	if len(home.Widgets) != 2 || home.Widgets[0].Name != "lbl_temp" || home.Widgets[1].Name != "btn_go" {
		t.Fatalf("home widgets: %+v", home.Widgets)
	}
	lbl := home.Widgets[0]
	if lbl.X != 12 || lbl.Props["text"] != "21 °C" || lbl.Props["size"] != 28 {
		t.Fatalf("label merge: %+v", lbl)
	}
	btn := home.Widgets[1]
	if btn.X != 200 || btn.Y != 10 || btn.Events[0].Action.Screen != "s_0002" {
		t.Fatalf("button: %+v", btn)
	}
	if out.Screens[1].Title != "Mehr" || out.Screens[1].Transition != "slide_left" || len(out.Screens) != 2 {
		t.Fatalf("screens: %+v", out.Screens)
	}
	if out.Theme != "synthwave" || out.Settings.WiFi != "required" || !out.Settings.AuraGoLink ||
		!reflect.DeepEqual(out.Settings.Libraries, []string{"arduinojson"}) {
		t.Fatalf("theme/settings: %s %+v", out.Theme, out.Settings)
	}
	if p.Theme != "clear" || len(p.Screens) != 1 {
		t.Fatal("ApplyOps modified its input project")
	}
}

func TestApplyOpsIsAtomic(t *testing.T) {
	p, ids := patchFixture(t)
	before := mustMarshal(t, p)
	out, results, err := ApplyOps(p, []LayoutOp{
		{Op: "update_widget", ID: "lbl", Set: map[string]any{"x": 20, "props": map[string]any{"text": "changed"}}},
		{Op: "add_widget", Screen: "nope", Widget: &Widget{Type: "label"}},
	}, ids)
	if ErrorCode(err) != CodeInvalidLayout || results != nil {
		t.Fatalf("err = %v, results = %v", err, results)
	}
	details := err.(*Error).Details
	if details["index"] != 1 || details["op"] != "add_widget" || !strings.Contains(details["reason"].(string), `screen "nope" not found`) {
		t.Fatalf("details = %v", details)
	}
	if mustMarshal(t, out) != before || mustMarshal(t, p) != before {
		t.Fatal("a failed ApplyOps changed the project")
	}
}

func TestApplyOpsFailures(t *testing.T) {
	cases := []struct {
		name   string
		ops    []LayoutOp
		reason string
		code   string // validation code when a new issue caused the failure
	}{
		{"unknown op", []LayoutOp{{Op: "explode"}}, `unknown op "explode"`, ""},
		{"unknown screen", []LayoutOp{{Op: "add_widget", Screen: "nope", Widget: &Widget{Type: "label"}}}, `screen "nope" not found`, ""},
		{"missing screen", []LayoutOp{{Op: "add_widget", Widget: &Widget{Type: "label"}}}, "screen is required", ""},
		{"missing widget", []LayoutOp{{Op: "add_widget", Screen: "home"}}, "widget is required", ""},
		{"unknown type", []LayoutOp{{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "slider2"}}}, `unknown widget type "slider2"`, ""},
		{"parent not container", []LayoutOp{{Op: "add_widget", Screen: "home", Parent: "lbl", Widget: &Widget{Type: "label"}}}, "cannot contain widgets", ""},
		{"parent missing", []LayoutOp{{Op: "add_widget", Screen: "home", Parent: "nope", Widget: &Widget{Type: "label"}}}, `parent: widget "nope" not found`, ""},
		{"tab without tabview", []LayoutOp{{Op: "add_widget", Screen: "home", Tab: opInt(1), Widget: &Widget{Type: "label"}}}, "tab is only allowed", ""},
		{"tab out of range", []LayoutOp{{Op: "add_widget", Screen: "home", Parent: "tabs", Tab: opInt(5), Widget: &Widget{Type: "label"}}}, "tab 5 does not exist", IssueTabInvalid},
		{"duplicate name", []LayoutOp{{Op: "add_widget", Screen: "details", Widget: &Widget{Type: "label", Name: "lbl"}}}, `name "lbl" is used twice`, IssueNameDuplicate},
		{"out of bounds", []LayoutOp{{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "label", X: 500}}}, "outside its 320x240 parent", IssueOutOfBounds},
		{"events need name", []LayoutOp{{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "button",
			Events: []EventBinding{{On: "clicked", Action: Action{Type: "back"}}}}}}, "needs a name", IssueNameRequired},
		{"update id", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"id": "x"}}}, "id cannot be changed", ""},
		{"update type", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"type": "button"}}}, "type cannot be changed", ""},
		{"update children", []LayoutOp{{Op: "update_widget", ID: "pnl", Set: map[string]any{"children": []any{}}}}, "children cannot be changed", ""},
		{"update unknown field", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"colour": 1}}}, `unknown widget field "colour"`, ""},
		{"update wrong type", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"x": "ten"}}}, "set.x: must be an integer", ""},
		{"update props not object", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"props": "x"}}}, "set.props: must be an object", ""},
		{"update unknown style", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"style": map[string]any{"shadow": 1}}}}, `unknown style field "shadow"`, ""},
		{"update bad events", []LayoutOp{{Op: "update_widget", ID: "btn_go", Set: map[string]any{"events": []any{map[string]any{"when": "clicked"}}}}}, "set.events", ""},
		{"update invalid prop", []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"props": map[string]any{"size": 15}}}}, "size must be one of", IssueFontSizeInvalid},
		{"update empty set", []LayoutOp{{Op: "update_widget", ID: "lbl"}}, "set is empty", ""},
		{"update missing widget", []LayoutOp{{Op: "update_widget", ID: "nope", Set: map[string]any{"x": 1}}}, `widget "nope" not found`, ""},
		{"update wrong screen", []LayoutOp{{Op: "update_widget", ID: "lbl", Screen: "details", Set: map[string]any{"x": 1}}}, `widget "lbl" not found`, ""},
		{"move into itself", []LayoutOp{{Op: "move_widget", ID: "pnl", Parent: "pnl"}}, "into itself", ""},
		{"move without target", []LayoutOp{{Op: "move_widget", ID: "lbl"}}, "needs x/y, screen or parent", ""},
		{"move into non-container", []LayoutOp{{Op: "move_widget", ID: "lbl", Parent: "btn_go"}}, "cannot contain widgets", ""},
		{"move tab to root", []LayoutOp{{Op: "move_widget", ID: "lbl", Screen: "details", Tab: opInt(1)}}, "tab is only allowed", ""},
		{"delete targeted screen", []LayoutOp{{Op: "delete_screen", ID: "details"}}, `goto_screen target "s_0002" does not exist`, IssueScreenTargetMissing},
		{"delete missing widget", []LayoutOp{{Op: "delete_widget", ID: "nope"}}, `widget "nope" not found`, ""},
		{"set_theme unknown", []LayoutOp{{Op: "set_theme", Theme: "klar"}}, `unknown theme "klar"`, ""},
		{"set_settings unknown", []LayoutOp{{Op: "set_settings", Set: map[string]any{"volume": 3}}}, `unknown settings field "volume"`, ""},
		{"set_settings wrong type", []LayoutOp{{Op: "set_settings", Set: map[string]any{"libraries": "arduinojson"}}}, "must be a list of strings", ""},
		{"link needs wifi", []LayoutOp{{Op: "set_settings", Set: map[string]any{"wifi": "off", "aurago_link": true}}}, "needs Wi-Fi", IssueLinkRequiresWiFi},
		{"unknown library", []LayoutOp{{Op: "set_settings", Set: map[string]any{"libraries": []any{"lvgl"}}}}, `unknown library "lvgl"`, IssueLibraryUnknown},
		{"update_screen id", []LayoutOp{{Op: "update_screen", ID: "home", Set: map[string]any{"id": "x"}}}, "id cannot be changed", ""},
		{"update_screen bad name", []LayoutOp{{Op: "update_screen", ID: "home", Set: map[string]any{"name": "Home"}}}, "must match", IssueScreenNameInvalid},
		{"add_screen no data", []LayoutOp{{Op: "add_screen"}}, "screen_data is required", ""},
		{"add_screen duplicate", []LayoutOp{{Op: "add_screen", ScreenData: &Screen{Name: "home"}}}, `screen name "home" is used twice`, IssueScreenNameDuplicate},
		{"second op fails", []LayoutOp{
			{Op: "set_theme", Theme: "night"},
			{Op: "update_widget", ID: "lbl", Set: map[string]any{"w": 0}},
		}, "geometry", IssueSizeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ids := patchFixture(t)
			_, _, err := ApplyOps(p, tc.ops, ids)
			if ErrorCode(err) != CodeInvalidLayout {
				t.Fatalf("err = %v, want CYD_INVALID_LAYOUT", err)
			}
			d := err.(*Error).Details
			if d["index"] != len(tc.ops)-1 {
				t.Errorf("index = %v, want %d", d["index"], len(tc.ops)-1)
			}
			if reason, _ := d["reason"].(string); !strings.Contains(reason, tc.reason) {
				t.Errorf("reason = %q, want it to contain %q", reason, tc.reason)
			}
			if code, _ := d["code"].(string); code != tc.code {
				t.Errorf("code = %q, want %q", code, tc.code)
			}
		})
	}
}

func TestApplyOpsDeleteLastScreen(t *testing.T) {
	p := newTestProject(t)
	_, _, err := ApplyOps(p, []LayoutOp{{Op: "delete_screen", ID: "home"}}, &SeqIDs{})
	if ErrorCode(err) != CodeInvalidLayout || err.(*Error).Details["code"] != IssueStartScreenMissing {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyOpsTooManyOps(t *testing.T) {
	p := newTestProject(t)
	ops := make([]LayoutOp, maxLayoutOps+1)
	for i := range ops {
		ops[i] = LayoutOp{Op: "set_theme", Theme: "night"}
	}
	_, _, err := ApplyOps(p, ops, nil)
	if ErrorCode(err) != CodeInvalidLayout || err.(*Error).Details["index"] != maxLayoutOps {
		t.Fatalf("err = %v", err)
	}
	if _, res, err := ApplyOps(p, ops[:maxLayoutOps], nil); err != nil || len(res) != maxLayoutOps {
		t.Fatalf("%d ops rejected: %v", maxLayoutOps, err)
	}
}

func TestApplyOpsSetMergeSemantics(t *testing.T) {
	p, ids := patchFixture(t)
	p, _, err := ApplyOps(p, []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{
		"props": map[string]any{"text": "A", "align": "center"},
		"style": map[string]any{"bg_color": "#111111", "radius": 4},
	}}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err = ApplyOps(p, []LayoutOp{{Op: "update_widget", ID: "w_0001", Set: map[string]any{
		"props": map[string]any{"text": "B", "align": nil},
		"style": map[string]any{"radius": nil, "text_color": "#EEEEEE"},
	}}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	lbl := p.Screens[0].Widgets[0]
	if !reflect.DeepEqual(lbl.Props, map[string]any{"text": "B"}) {
		t.Fatalf("props = %v", lbl.Props)
	}
	if lbl.Style.BgColor != "#111111" || lbl.Style.Radius != nil || lbl.Style.TextColor != "#EEEEEE" {
		t.Fatalf("style = %+v", lbl.Style)
	}
	p, _, err = ApplyOps(p, []LayoutOp{
		{Op: "update_widget", ID: "lbl", Set: map[string]any{"props": nil, "style": nil}},
		{Op: "update_widget", ID: "btn_go", Set: map[string]any{"events": []any{
			map[string]any{"on": "long_pressed", "action": map[string]any{"type": "logic", "hint": "reload"}}}}},
	}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if lbl := p.Screens[0].Widgets[0]; lbl.Props != nil || !reflect.DeepEqual(lbl.Style, Style{}) {
		t.Fatalf("reset failed: %+v", lbl)
	}
	if ev := p.Screens[0].Widgets[3].Events; len(ev) != 1 || ev[0].On != "long_pressed" || ev[0].Action.Hint != "reload" {
		t.Fatalf("events replaced: %+v", ev)
	}
	p, _, err = ApplyOps(p, []LayoutOp{{Op: "update_widget", ID: "btn_go", Set: map[string]any{"events": nil}}}, ids)
	if err != nil || p.Screens[0].Widgets[3].Events != nil {
		t.Fatalf("events cleared: %v %+v", err, p.Screens[0].Widgets[3].Events)
	}
}

func TestApplyOpsDecodedJSON(t *testing.T) {
	p, ids := patchFixture(t)
	var ops []LayoutOp
	raw := `[{"op":"update_widget","id":"lbl","set":{"x":20,"props":{"size":24,"text":"Hi"}}},
	         {"op":"add_widget","screen":"details","widget":{"type":"slider","name":"sld","x":10,"y":10,"props":{"value":70}}},
	         {"op":"move_widget","id":"lbl","parent":"pnl","x":5,"y":5},
	         {"op":"add_widget","screen":"home","parent":"tabs","tab":1,"widget":{"type":"switch","x":10,"y":10}}]`
	if err := json.Unmarshal([]byte(raw), &ops); err != nil {
		t.Fatal(err)
	}
	out, results, err := ApplyOps(p, ops, ids)
	if err != nil {
		t.Fatal(err)
	}
	if results[1].ID != "w_0005" || results[3].ID != "w_0006" {
		t.Fatalf("results: %+v", results)
	}
	pnl := out.Screens[0].Widgets[0]
	if pnl.Name != "pnl" || len(pnl.Children) != 1 || pnl.Children[0].Name != "lbl" || pnl.Children[0].X != 5 {
		t.Fatalf("moved label: %+v", pnl)
	}
	tabs := out.Screens[0].Widgets[1]
	if len(tabs.Children) != 1 || tabs.Children[0].Tab != 1 || tabs.Children[0].W != 56 {
		t.Fatalf("tab child: %+v", tabs.Children)
	}
	if issues := Validate(out, ValidateOptions{}); HasErrors(issues) {
		t.Fatalf("issues:\n%s", fmtIssues(issues))
	}
}

func TestApplyOpsPreexistingErrorsDoNotBlock(t *testing.T) {
	p, ids := patchFixture(t)
	p.Settings.Libraries = []string{"lvgl"}
	p.Screens[0].Widgets[0].X = 400 // lbl out of bounds
	p.Screens[0].Widgets[0].Props = map[string]any{"text": strings.Repeat("x", MaxTextBytes+1)}
	out, _, err := ApplyOps(p, []LayoutOp{
		{Op: "add_widget", Screen: "details", Widget: &Widget{Type: "label", Name: "lbl_new"}},
		{Op: "update_widget", ID: "lbl", Set: map[string]any{"name": "lbl_renamed", "y": 30}},
		{Op: "delete_widget", ID: "pnl"}, // shifts lbl's siblings; keys must stay stable
	}, ids)
	if err != nil {
		t.Fatalf("pre-existing errors blocked an unrelated op: %v", err)
	}
	if !HasErrors(Validate(out, ValidateOptions{})) {
		t.Fatal("pre-existing errors vanished")
	}
	if _, _, err := ApplyOps(out, []LayoutOp{{Op: "update_widget", ID: "lbl_new", Set: map[string]any{"x": 900}}}, ids); err == nil {
		t.Fatal("a new error on another widget must still fail")
	}
}

func TestApplyOpsFreshIDsAndDefaults(t *testing.T) {
	p, _ := patchFixture(t)
	restarted := &SeqIDs{} // would hand out w_0001 again
	out, res, err := ApplyOps(p, []LayoutOp{
		{Op: "add_widget", Screen: "details", Widget: &Widget{ID: "custom", Type: "panel", Name: "pnl2",
			Children: []Widget{{ID: "custom", Type: "statusbar"}}}},
		{Op: "add_screen", ScreenData: &Screen{ID: "s_custom", Name: "third", Transition: "fade", Widgets: []Widget{
			{Type: "button", Name: "btn_self", Events: []EventBinding{{On: "clicked", Action: Action{Type: "goto_screen", Screen: "third"}}}},
		}}},
	}, restarted)
	if err != nil {
		t.Fatal(err)
	}
	pnl := out.Screens[1].Widgets[0]
	if pnl.ID != "w_0005" || pnl.Children[0].ID != "w_0006" || res[0].ID != "w_0005" {
		t.Fatalf("ids: panel %s child %s", pnl.ID, pnl.Children[0].ID)
	}
	if pnl.W != 160 || pnl.H != 100 || pnl.Children[0].W != 160 || pnl.Children[0].H != 24 {
		t.Fatalf("default sizes: %+v", pnl)
	}
	third := out.Screens[2]
	if third.ID != "s_0003" || third.Transition != "fade" || third.Widgets[0].ID != "w_0007" ||
		third.Widgets[0].Events[0].Action.Screen != "s_0003" || third.Widgets[0].W != 120 {
		t.Fatalf("added screen: %+v", third)
	}
	sb, _, err := ApplyOps(p, []LayoutOp{{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "statusbar"}}}, restarted)
	if err != nil {
		t.Fatal(err)
	}
	if w := sb.Screens[0].Widgets[4]; w.W != 320 || w.H != 24 {
		t.Fatalf("statusbar default width: %+v", w)
	}
}

func TestApplyOpsMoveVariants(t *testing.T) {
	p, ids := patchFixture(t)
	p, _, err := ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "lbl", Parent: "pnl", X: opInt(4), Y: opInt(4)}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if pnl := p.Screens[0].Widgets[0]; pnl.Name != "pnl" || pnl.Children[0].Name != "lbl" {
		t.Fatalf("into panel: %+v", p.Screens[0].Widgets)
	}
	p, _, err = ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "lbl", Parent: "tabs", Tab: opInt(1)}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if tabs := p.Screens[0].Widgets[1]; tabs.Children[0].Name != "lbl" || tabs.Children[0].Tab != 1 || tabs.Children[0].X != 4 {
		t.Fatalf("into tabview: %+v", tabs)
	}
	p, _, err = ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "lbl", Screen: "details", X: opInt(30), Y: opInt(40)}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if d := p.Screens[1].Widgets; len(d) != 1 || d[0].Name != "lbl" || d[0].Tab != 0 || d[0].X != 30 || d[0].Y != 40 {
		t.Fatalf("to other screen: %+v", d)
	}
	p, _, err = ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "lbl", X: opInt(50)}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if w := p.Screens[1].Widgets[0]; w.X != 50 || w.Y != 40 {
		t.Fatalf("x only: %+v", w)
	}
}

func TestApplyOpsDeleteStartScreenMovesStart(t *testing.T) {
	p, ids := patchFixture(t)
	out, res, err := ApplyOps(p, []LayoutOp{
		{Op: "delete_widget", ID: "btn_go"},
		{Op: "delete_screen", Screen: "home"},
	}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if out.StartScreen != "s_0002" || len(out.Screens) != 1 {
		t.Fatalf("start = %s, screens = %d", out.StartScreen, len(out.Screens))
	}
	if res[1].Summary != "deleted screen home" || res[0].Summary != "deleted button btn_go" {
		t.Fatalf("summaries: %+v", res)
	}
	nested, _, err := ApplyOps(p, []LayoutOp{
		{Op: "add_widget", Screen: "home", Parent: "pnl", Widget: &Widget{Type: "label", Name: "inner"}},
		{Op: "delete_widget", ID: "pnl"},
	}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := nested.FindWidget("inner"); ok {
		t.Fatal("nested widget survived its parent")
	}
}

// issueWidgets lists the widget ids that carry an issue with the given code.
func issueWidgets(p Project, code string) []string {
	var out []string
	for _, is := range Validate(p, ValidateOptions{}) {
		if is.Code == code {
			out = append(out, is.Widget)
		}
	}
	return out
}

func countIssues(p Project, code string) int {
	n := 0
	for _, is := range Validate(p, ValidateOptions{}) {
		if is.Code == code {
			n++
		}
	}
	return n
}

// Validate attributes NAME_DUPLICATE to the later of two clashing widgets, so
// moving the earlier one shifts the issue to it. That is the same pre-existing
// error, not a new one.
func TestApplyOpsMovingPreexistingDuplicateNameIsNotNew(t *testing.T) {
	p, ids := patchFixture(t)
	p.Screens[0].Widgets[1].Name = "lbl" // pnl w_0002 now shares the name of lbl w_0001
	if got := issueWidgets(p, IssueNameDuplicate); !reflect.DeepEqual(got, []string{"w_0002"}) {
		t.Fatalf("precondition: NAME_DUPLICATE on %v, want [w_0002]", got)
	}
	out, _, err := ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "w_0001", Screen: "home"}}, ids)
	if err != nil {
		t.Fatalf("moving a widget with a pre-existing duplicate name was rejected: %v", err)
	}
	if got := issueWidgets(out, IssueNameDuplicate); !reflect.DeepEqual(got, []string{"w_0001"}) {
		t.Fatalf("NAME_DUPLICATE is on %v after the move, want [w_0001] (the attribution should have shifted)", got)
	}
}

// The same for ID_DUPLICATE: moving a widget across screens changes the screen
// the issue is attributed to.
func TestApplyOpsMovingPreexistingDuplicateIDIsNotNew(t *testing.T) {
	p, ids := patchFixture(t)
	p.Screens[1].Widgets = append(p.Screens[1].Widgets, Widget{ID: "w_0001", Type: "label", Name: "twin", W: 120, H: 24})
	var onScreen []string
	for _, is := range Validate(p, ValidateOptions{}) {
		if is.Code == IssueIDDuplicate {
			onScreen = append(onScreen, is.Screen)
		}
	}
	if !reflect.DeepEqual(onScreen, []string{"s_0002"}) {
		t.Fatalf("precondition: ID_DUPLICATE on screens %v, want [s_0002]", onScreen)
	}
	out, _, err := ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "twin", Screen: "home"}}, ids)
	if err != nil {
		t.Fatalf("moving a widget with a pre-existing duplicate id was rejected: %v", err)
	}
	onScreen = nil
	for _, is := range Validate(out, ValidateOptions{}) {
		if is.Code == IssueIDDuplicate {
			onScreen = append(onScreen, is.Screen)
		}
	}
	if !reflect.DeepEqual(onScreen, []string{"s_0001"}) {
		t.Fatalf("ID_DUPLICATE is on screens %v after the move, want [s_0001]", onScreen)
	}
}

func TestIssueKeyScreenNameDuplicateIgnoresAttribution(t *testing.T) {
	p, _ := patchFixture(t)
	p.Screens[1].Name = "home" // SCREEN_NAME_DUPLICATE lands on s_0002
	q := cloneProject(p)
	q.Screens[0], q.Screens[1] = q.Screens[1], q.Screens[0] // ... and now on s_0001
	kp, kq := errorKeys(&p, Validate(p, ValidateOptions{})), errorKeys(&q, Validate(q, ValidateOptions{}))
	if len(kp) == 0 || !reflect.DeepEqual(kp, kq) {
		t.Fatalf("keys differ after swapping the screens:\n%v\n%v", kp, kq)
	}
}

// Validate lists at most maxUnknownPropIssues unknown props per widget and
// summarises the rest; removing one prop shifts which keys are listed.
func TestApplyOpsRemovingUnknownPropFromCappedWidgetIsNotNew(t *testing.T) {
	p, ids := patchFixture(t)
	props := map[string]any{}
	for i := 0; i < 22; i++ {
		props["zz"+string(rune('a'+i))] = 1
	}
	p.Screens[0].Widgets[0].Props = props
	if n := countIssues(p, IssueUnknownProp); n != maxUnknownPropIssues+1 {
		t.Fatalf("precondition: %d UNKNOWN_PROP issues, want %d", n, maxUnknownPropIssues+1)
	}
	out, _, err := ApplyOps(p, []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{
		"props": map[string]any{"zza": nil}}}}, ids)
	if err != nil {
		t.Fatalf("removing one of 22 unknown props was rejected: %v", err)
	}
	if got := len(out.Screens[0].Widgets[0].Props); got != 21 {
		t.Fatalf("%d props left, want 21", got)
	}
	if n := countIssues(out, IssueUnknownProp); n != maxUnknownPropIssues+1 {
		t.Fatalf("%d UNKNOWN_PROP issues after the op, want %d", n, maxUnknownPropIssues+1)
	}
}

// Every value an op can make an error echo is bounded: the message stays
// within about 1 KB however large the offending input is.
func TestApplyOpsBoundsEchoedInput(t *testing.T) {
	big := strings.Repeat("ä", 1<<19) // 1 MiB
	cases := []struct {
		name string
		op   LayoutOp
	}{
		{"op name", LayoutOp{Op: big}},
		{"screen", LayoutOp{Op: "add_widget", Screen: big, Widget: &Widget{Type: "label"}}},
		{"parent", LayoutOp{Op: "add_widget", Screen: "home", Parent: big, Widget: &Widget{Type: "label"}}},
		{"widget type", LayoutOp{Op: "add_widget", Screen: "home", Widget: &Widget{Type: big}}},
		{"update target", LayoutOp{Op: "update_widget", ID: big, Set: map[string]any{"x": 1}}},
		{"update screen filter", LayoutOp{Op: "update_widget", ID: "lbl", Screen: big, Set: map[string]any{"x": 1}}},
		{"move target", LayoutOp{Op: "move_widget", ID: big, X: opInt(1)}},
		{"delete target", LayoutOp{Op: "delete_widget", ID: big}},
		{"delete screen", LayoutOp{Op: "delete_screen", ID: big}},
		{"theme", LayoutOp{Op: "set_theme", Theme: big}},
		{"widget set key", LayoutOp{Op: "update_widget", ID: "lbl", Set: map[string]any{big: 1}}},
		{"style key", LayoutOp{Op: "update_widget", ID: "lbl", Set: map[string]any{"style": map[string]any{big: 1}}}},
		{"screen set key", LayoutOp{Op: "update_screen", ID: "home", Set: map[string]any{big: 1}}},
		{"settings key", LayoutOp{Op: "set_settings", Set: map[string]any{big: 1}}},
		{"event field", LayoutOp{Op: "update_widget", ID: "btn_go", Set: map[string]any{"events": []any{
			map[string]any{"on": "clicked", "action": map[string]any{"type": "back"}, big: 1}}}}},
		{"prop key (validation issue)", LayoutOp{Op: "update_widget", ID: "lbl", Set: map[string]any{"props": map[string]any{big: 1}}}},
		{"screen name (validation issue)", LayoutOp{Op: "update_screen", ID: "home", Set: map[string]any{"name": big}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ids := patchFixture(t)
			_, _, err := ApplyOps(p, []LayoutOp{tc.op}, ids)
			if ErrorCode(err) != CodeInvalidLayout {
				t.Fatalf("err = %.200v, want CYD_INVALID_LAYOUT", err)
			}
			if n := len(err.Error()); n > 1100 {
				t.Errorf("error text is %d bytes", n)
			}
			d := err.(*Error).Details
			for _, k := range []string{"reason", "path", "op"} {
				if s, _ := d[k].(string); len(s) > 1100 {
					t.Errorf("details.%s is %d bytes", k, len(s))
				}
			}
			if s, _ := d["op"].(string); len(s) > 2*maxEchoBytes {
				t.Errorf("details.op is %d bytes", len(s))
			}
		})
	}
	t.Run("summaries", func(t *testing.T) {
		p, ids := patchFixture(t)
		p.Screens[1].Name = strings.Repeat("a", 100_000) // pre-existing SCREEN_NAME_INVALID
		_, res, err := ApplyOps(p, []LayoutOp{{Op: "update_screen", ID: "s_0002", Set: map[string]any{"title": "x"}}}, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(res[0].Summary) > 200 {
			t.Fatalf("summary is %d bytes", len(res[0].Summary))
		}
	})
}

func TestApplyOpsRejectsOversizedResult(t *testing.T) {
	p, ids := patchFixture(t)
	before := mustMarshal(t, p)
	cells := make([]any, 256)
	for i := range cells {
		cells[i] = strings.Repeat("a", 1000)
	}
	var tables []Widget
	for i := 0; i < 5; i++ {
		tables = append(tables, Widget{Type: "table", Props: map[string]any{"columns": 8, "rows": 32, "cells": cells}})
	}
	out, res, err := ApplyOps(p, []LayoutOp{
		{Op: "set_theme", Theme: "night"},
		{Op: "add_screen", ScreenData: &Screen{Name: "big", Widgets: tables}},
	}, ids)
	if ErrorCode(err) != CodeInvalidLayout || res != nil {
		t.Fatalf("err = %.200v, results = %v", err, res)
	}
	d := err.(*Error).Details
	if d["index"] != 1 || d["op"] != "add_screen" || d["limit"] != MaxLayoutBytes {
		t.Fatalf("details = %v", d)
	}
	if n, _ := d["bytes"].(int); n <= MaxLayoutBytes {
		t.Fatalf("details.bytes = %v", d["bytes"])
	}
	if reason, _ := d["reason"].(string); !strings.Contains(reason, "the ops make the layout too large or too complex") {
		t.Fatalf("reason = %q", reason)
	}
	if _, has := d["code"]; has {
		t.Fatalf("a size failure has no validation code: %v", d)
	}
	if mustMarshal(t, out) != before || mustMarshal(t, p) != before {
		t.Fatal("a failed ApplyOps changed the project")
	}
}

// The unknown prop is masked from the per-op gate by the widget's earlier
// unknown prop (UNKNOWN_PROP is keyed per widget), so only the result check
// can reject the nesting.
func TestApplyOpsRejectsTooDeepResult(t *testing.T) {
	nest := func(depth int) any {
		v := any(1)
		for i := 0; i < depth; i++ {
			v = map[string]any{"k": v}
		}
		return v
	}
	set := func(depth int) []LayoutOp {
		return []LayoutOp{{Op: "update_widget", ID: "lbl", Set: map[string]any{"props": map[string]any{"deep": nest(depth)}}}}
	}
	p, ids := patchFixture(t)
	p.Screens[0].Widgets[0].Props = map[string]any{"zz": 1}
	before := mustMarshal(t, p)
	if _, _, err := ApplyOps(p, set(3), ids); err != nil {
		t.Fatalf("shallow nesting rejected: %v", err)
	}
	out, _, err := ApplyOps(p, set(40), ids)
	if ErrorCode(err) != CodeInvalidLayout {
		t.Fatalf("err = %v", err)
	}
	d := err.(*Error).Details
	if reason, _ := d["reason"].(string); !strings.Contains(reason, "the ops make the layout too large or too complex") ||
		!strings.Contains(reason, "nesting deeper than") || d["limit"] != maxLayoutDepth || d["index"] != 0 {
		t.Fatalf("details = %v", d)
	}
	if mustMarshal(t, out) != before || mustMarshal(t, p) != before {
		t.Fatal("a failed ApplyOps changed the project")
	}
}

func nestedPanels(depth int) Widget {
	w := Widget{Type: "label"}
	for i := 1; i < depth; i++ {
		w = Widget{Type: "panel", Children: []Widget{w}}
	}
	return w
}

func TestApplyOpsRejectsOversizedSubtreesEarly(t *testing.T) {
	flat := make([]Widget, MaxWidgetsPerScreen+1)
	for i := range flat {
		flat[i] = Widget{Type: "label"}
	}
	cases := []struct {
		name   string
		op     LayoutOp
		reason string
	}{
		{"widget 50 deep", LayoutOp{Op: "add_widget", Screen: "home", Widget: ptr(nestedPanels(50))}, "nested deeper than 4 levels"},
		{"widget 5 deep", LayoutOp{Op: "add_widget", Screen: "home", Widget: ptr(nestedPanels(MaxNesting + 1))}, "nested deeper than 4 levels"},
		{"screen 50 deep", LayoutOp{Op: "add_screen", ScreenData: &Screen{Name: "deep", Widgets: []Widget{nestedPanels(50)}}}, "nested deeper than 4 levels"},
		{"screen too many widgets", LayoutOp{Op: "add_screen", ScreenData: &Screen{Name: "many", Widgets: flat}}, "more than 200 widgets"},
		{"widget too many children", LayoutOp{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "panel", Children: flat}}, "more than 200 widgets"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ids := patchFixture(t)
			_, _, err := ApplyOps(p, []LayoutOp{tc.op}, ids)
			if ErrorCode(err) != CodeInvalidLayout {
				t.Fatalf("err = %v", err)
			}
			d := err.(*Error).Details
			if reason, _ := d["reason"].(string); !strings.Contains(reason, tc.reason) {
				t.Errorf("reason = %q, want it to contain %q", reason, tc.reason)
			}
			if _, has := d["code"]; has {
				t.Errorf("rejected by the validation gate instead of the pre-check: %v", d)
			}
		})
	}
	if err := checkNewTree([]Widget{nestedPanels(MaxNesting)}); err != nil {
		t.Fatalf("a tree of the maximum depth was rejected: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

// A widget keeps its pre-existing errors when it moves to another screen: the
// gate key must not contain the screen.
func TestApplyOpsMovingWidgetToOtherScreenKeepsPreexistingErrors(t *testing.T) {
	cases := []struct {
		name  string
		props map[string]any
		code  string
	}{
		{"text too long", map[string]any{"text": strings.Repeat("x", MaxTextBytes+1)}, IssueTextTooLong},
		{"unknown prop", map[string]any{"colour": "red"}, IssueUnknownProp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ids := patchFixture(t)
			p.Screens[0].Widgets[0].Props = tc.props
			if countIssues(p, tc.code) == 0 {
				t.Fatalf("precondition: no %s issue", tc.code)
			}
			out, _, err := ApplyOps(p, []LayoutOp{{Op: "move_widget", ID: "lbl", Screen: "details", X: opInt(5), Y: opInt(5)}}, ids)
			if err != nil {
				t.Fatalf("moving a widget with a pre-existing %s error was rejected: %v", tc.code, err)
			}
			if d := out.Screens[1].Widgets; len(d) != 1 || d[0].Name != "lbl" {
				t.Fatalf("details widgets: %+v", d)
			}
			if countIssues(out, tc.code) == 0 {
				t.Fatalf("%s vanished", tc.code)
			}
			// the same kind of error on another widget is still new
			if _, _, err := ApplyOps(out, []LayoutOp{{Op: "update_widget", ID: "btn_go", Set: map[string]any{"props": tc.props}}}, ids); err == nil {
				t.Fatal("a new error on another widget must still fail")
			}
		})
	}
}

// Parents are searched on the screen that was resolved, even when screen ids
// are not unique (a pre-existing error).
func TestApplyOpsParentLookupUsesTheResolvedScreen(t *testing.T) {
	p, ids := patchFixture(t)
	p.Screens[1].ID = "s_0001" // duplicate screen id: screenIndex("s_0001") finds home first
	p.Screens[1].Widgets = []Widget{{ID: "w_0099", Type: "panel", Name: "pnl2", W: 160, H: 100}}
	out, _, err := ApplyOps(p, []LayoutOp{
		{Op: "add_widget", Screen: "details", Parent: "pnl2", Widget: &Widget{Type: "label", Name: "lbl2"}},
		{Op: "move_widget", ID: "lbl", Screen: "details", Parent: "pnl2", X: opInt(3), Y: opInt(3)},
	}, ids)
	if err != nil {
		t.Fatal(err)
	}
	pnl := out.Screens[1].Widgets[0]
	if len(pnl.Children) != 2 || pnl.Children[0].Name != "lbl2" || pnl.Children[1].Name != "lbl" {
		t.Fatalf("pnl2 children: %+v", pnl.Children)
	}
}

// A duplicate that did not exist before still fails, also next to a
// pre-existing duplicate of another name.
func TestApplyOpsNewDuplicateNameIsStillRejected(t *testing.T) {
	cases := []struct {
		name string
		ops  []LayoutOp
	}{
		{"rename", []LayoutOp{{Op: "update_widget", ID: "btn_go", Set: map[string]any{"name": "tabs"}}}},
		{"add", []LayoutOp{{Op: "add_widget", Screen: "details", Widget: &Widget{Type: "label", Name: "tabs"}}}},
		{"second op of the call", []LayoutOp{
			{Op: "add_widget", Screen: "details", Widget: &Widget{Type: "label", Name: "fresh"}},
			{Op: "update_widget", ID: "btn_go", Set: map[string]any{"name": "fresh"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ids := patchFixture(t)
			p.Screens[0].Widgets[1].Name = "lbl" // pre-existing duplicate "lbl"
			_, _, err := ApplyOps(p, tc.ops, ids)
			if ErrorCode(err) != CodeInvalidLayout {
				t.Fatalf("err = %v, want CYD_INVALID_LAYOUT", err)
			}
			if d := err.(*Error).Details; d["code"] != IssueNameDuplicate || d["index"] != len(tc.ops)-1 {
				t.Fatalf("details = %v", d)
			}
		})
	}
}
