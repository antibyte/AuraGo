package cydcreator

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// validFixture: screens home (s_0001, start) and details (s_0002), one asset, no issues.
func validFixture(t *testing.T) Project {
	t.Helper()
	p := newTestProject(t)
	p.Screens = append(p.Screens, Screen{ID: "s_0002", Name: "details", Title: "Details", Transition: "fade", Widgets: []Widget{}})
	p.Assets = []Asset{{ID: "a_sun", File: "assets/sun.png", W: 48, H: 48, Alpha: true}}
	return p
}

func addTo(p *Project, screen int, ws ...Widget) {
	p.Screens[screen].Widgets = append(p.Screens[screen].Widgets, ws...)
}

func lbl(id string) Widget { return Widget{ID: id, Type: "label", W: 120, H: 24} }

func fmtIssues(issues []Issue) string {
	var b strings.Builder
	for _, is := range issues {
		fmt.Fprintf(&b, "  %s %s %s: %s\n", is.Severity, is.Code, is.Path, is.Message)
	}
	return b.String()
}

func TestValidateCleanProjects(t *testing.T) {
	p := validFixture(t)
	if issues := Validate(p, ValidateOptions{AssetSizes: map[string]int64{"a_sun": 48 * 48 * 3}}); len(issues) != 0 {
		t.Fatalf("fixture has issues:\n%s", fmtIssues(issues))
	}
	// Every widget type with every prop set explicitly to its default.
	for i, spec := range WidgetCatalog() {
		w := Widget{ID: fmt.Sprintf("w_%04d", i+1), Type: spec.Type, W: spec.DefaultW, H: spec.DefaultH, Props: map[string]any{}}
		if w.W == 0 {
			w.W = 320
		}
		for _, ps := range spec.Props {
			w.Props[ps.Name] = ps.Default
		}
		addTo(&p, 0, w)
	}
	// Every allowed event with a valid action.
	addTo(&p, 1,
		Widget{ID: "w_b1", Type: "button", Name: "btn_go", W: 100, H: 40, Events: []EventBinding{
			{On: "clicked", Action: Action{Type: "goto_screen", Screen: "s_0001", Transition: "slide_left"}},
			{On: "long_pressed", Action: Action{Type: "logic", Hint: "lädt das Wetter neu"}},
		}},
		Widget{ID: "w_t1", Type: "textarea", Name: "ta_ssid", Y: 50, W: 200, H: 40, Events: []EventBinding{
			{On: "ready", Action: Action{Type: "back"}},
			{On: "value_changed", Action: Action{Type: "open_wifi_setup"}},
		}},
		Widget{ID: "w_p1", Type: "tabview", Y: 0, W: 300, H: 200, Props: map[string]any{"tabs": []any{"A", "B", "C"}},
			Children: []Widget{{ID: "w_p1c", Type: "label", Tab: 2, W: 100, H: 24}}},
		Widget{ID: "w_img", Type: "image", W: 48, H: 48, Props: map[string]any{"asset": "a_sun"},
			Style: Style{BgColor: "#a0b0c0", Radius: new(int)}},
	)
	if issues := Validate(p, ValidateOptions{}); len(issues) != 0 {
		t.Fatalf("all-defaults project has issues:\n%s", fmtIssues(issues))
	}
}

func TestValidateIssueCodes(t *testing.T) {
	type tc struct {
		name   string
		mutate func(p *Project, o *ValidateOptions)
		code   string
		sev    string
		path   string
	}
	E, W := SeverityError, SeverityWarning
	ev := func(on string, a Action) []EventBinding { return []EventBinding{{On: on, Action: a}} }
	cases := []tc{
		{"name invalid", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.Name = "Bad-Name"; addTo(p, 0, w) },
			IssueNameInvalid, E, "screens/0/widgets/0/name"},
		{"name reserved", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.Name = "show_dialog"; addTo(p, 0, w) },
			IssueNameInvalid, E, "screens/0/widgets/0/name"},
		{"name duplicate", func(p *Project, _ *ValidateOptions) {
			a, b := lbl("w_1"), lbl("w_2")
			a.Name, b.Name = "lbl", "lbl"
			addTo(p, 0, a)
			addTo(p, 1, b)
		}, IssueNameDuplicate, E, "screens/1/widgets/0/name"},
		{"name required", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Events = ev("clicked", Action{Type: "back"})
			addTo(p, 0, w)
		}, IssueNameRequired, E, "screens/0/widgets/0/name"},
		{"screen name invalid", func(p *Project, _ *ValidateOptions) { p.Screens[1].Name = "Details" },
			IssueScreenNameInvalid, E, "screens/1/name"},
		{"screen name duplicate", func(p *Project, _ *ValidateOptions) { p.Screens[1].Name = "home" },
			IssueScreenNameDuplicate, E, "screens/1/name"},
		{"start screen missing", func(p *Project, _ *ValidateOptions) { p.StartScreen = "s_9999" },
			IssueStartScreenMissing, E, "start_screen"},
		{"unknown type", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.Type = "slider2"; addTo(p, 0, w) },
			IssueUnknownType, E, "screens/0/widgets/0/type"},
		{"unknown prop", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Props = map[string]any{"colour": "red"}
			addTo(p, 0, w)
		}, IssueUnknownProp, E, "screens/0/widgets/0/props/colour"},
		{"prop out of range", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "slider", W: 160, H: 24, Props: map[string]any{"value": 500}})
		}, IssuePropInvalid, E, "screens/0/widgets/0/props/value"},
		{"prop wrong type", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "switch", W: 56, H: 28, Props: map[string]any{"checked": "yes"}})
		}, IssuePropInvalid, E, "screens/0/widgets/0/props/checked"},
		{"backlight", func(p *Project, _ *ValidateOptions) { p.Settings.Backlight = 5 },
			IssuePropInvalid, E, "settings/backlight"},
		{"text too long", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Props = map[string]any{"text": strings.Repeat("x", MaxTextBytes+1)}
			addTo(p, 0, w)
		}, IssueTextTooLong, E, "screens/0/widgets/0/props/text"},
		{"hint too long", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn_x", W: 100, H: 40,
				Events: ev("clicked", Action{Type: "logic", Hint: strings.Repeat("ä", MaxHintRunes+1)})})
		}, IssueHintTooLong, E, "screens/0/widgets/0/events/0/action/hint"},
		{"out of bounds", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.X = 400; addTo(p, 0, w) },
			IssueOutOfBounds, E, "screens/0/widgets/0"},
		{"partly out of bounds", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.X = 300; addTo(p, 0, w) },
			IssuePartlyOutOfBounds, W, "screens/0/widgets/0"},
		{"size invalid", func(p *Project, _ *ValidateOptions) { w := lbl("w_1"); w.W = 0; addTo(p, 0, w) },
			IssueSizeInvalid, E, "screens/0/widgets/0"},
		{"too many screens", func(p *Project, _ *ValidateOptions) {
			for i := 0; i < MaxScreens; i++ {
				p.Screens = append(p.Screens, Screen{ID: fmt.Sprintf("s_x%d", i), Name: fmt.Sprintf("extra_%d", i), Transition: "none"})
			}
		}, IssueTooManyScreens, E, "screens"},
		{"too many widgets", func(p *Project, _ *ValidateOptions) {
			for i := 0; i <= MaxWidgetsPerScreen; i++ {
				addTo(p, 0, lbl(fmt.Sprintf("w_%d", i)))
			}
		}, IssueTooManyWidgets, E, "screens/0/widgets"},
		{"nesting too deep", func(p *Project, _ *ValidateOptions) {
			leaf := Widget{ID: "w_5", Type: "panel", W: 160, H: 100}
			for i := 4; i >= 1; i-- {
				leaf = Widget{ID: fmt.Sprintf("w_%d", i), Type: "panel", W: 160, H: 100, Children: []Widget{leaf}}
			}
			addTo(p, 0, leaf)
		}, IssueNestingTooDeep, E, "screens/0/widgets/0/children/0/children/0/children/0/children/0"},
		{"children not allowed", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Children = []Widget{lbl("w_2")}
			addTo(p, 0, w)
		}, IssueChildrenNotAllowed, E, "screens/0/widgets/0/children"},
		{"tab out of range", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "tabview", W: 300, H: 200,
				Children: []Widget{{ID: "w_2", Type: "label", Tab: 2, W: 100, H: 24}}})
		}, IssueTabInvalid, E, "screens/0/widgets/0/children/0/tab"},
		{"tab outside tabview", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "panel", W: 160, H: 100,
				Children: []Widget{{ID: "w_2", Type: "label", Tab: 1, W: 100, H: 24}}})
		}, IssueTabInvalid, E, "screens/0/widgets/0/children/0/tab"},
		{"event not allowed", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Name, w.Events = "lbl", ev("value_changed", Action{Type: "back"})
			addTo(p, 0, w)
		}, IssueEventNotAllowed, E, "screens/0/widgets/0/events/0/on"},
		{"event bound twice", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn", W: 100, H: 40, Events: []EventBinding{
				{On: "clicked", Action: Action{Type: "back"}}, {On: "clicked", Action: Action{Type: "back"}}}})
		}, IssueEventNotAllowed, E, "screens/0/widgets/0/events/1/on"},
		{"action unknown", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn", W: 100, H: 40, Events: ev("clicked", Action{Type: "explode"})})
		}, IssueActionInvalid, E, "screens/0/widgets/0/events/0/action/type"},
		{"wifi setup without wifi", func(p *Project, _ *ValidateOptions) {
			p.Settings.WiFi = "off"
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn", W: 100, H: 40, Events: ev("clicked", Action{Type: "open_wifi_setup"})})
		}, IssueActionInvalid, E, "screens/0/widgets/0/events/0/action/type"},
		{"screen target missing", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn", W: 100, H: 40,
				Events: ev("clicked", Action{Type: "goto_screen", Screen: "s_9999"})})
		}, IssueScreenTargetMissing, E, "screens/0/widgets/0/events/0/action/screen"},
		{"asset missing", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "image", W: 64, H: 64, Props: map[string]any{"asset": "a_missing"}})
		}, IssueAssetMissing, E, "screens/0/widgets/0/props/asset"},
		{"asset budget", func(_ *Project, o *ValidateOptions) { o.AssetSizes = map[string]int64{"a_sun": AssetBudget4MB + 1} },
			IssueAssetBudgetExceeded, E, "assets"},
		{"theme unknown", func(p *Project, _ *ValidateOptions) { p.Theme = "klar" }, IssueThemeUnknown, E, "theme"},
		{"board unknown", func(p *Project, _ *ValidateOptions) { p.Board = "esp32-nope" }, IssueBoardUnknown, E, "board"},
		{"library unknown", func(p *Project, _ *ValidateOptions) { p.Settings.Libraries = []string{"arduinojson", "lvgl"} },
			IssueLibraryUnknown, E, "settings/libraries/1"},
		{"wifi mode", func(p *Project, _ *ValidateOptions) { p.Settings.WiFi = "sometimes" },
			IssueWiFiModeInvalid, E, "settings/wifi"},
		{"link requires wifi", func(p *Project, _ *ValidateOptions) { p.Settings.WiFi, p.Settings.AuraGoLink = "off", true },
			IssueLinkRequiresWiFi, E, "settings/aurago_link"},
		{"font unknown", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Props = map[string]any{"font": "comic"}
			addTo(p, 0, w)
		}, IssueFontUnknown, E, "screens/0/widgets/0/props/font"},
		{"font size", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Props = map[string]any{"size": 15}
			addTo(p, 0, w)
		}, IssueFontSizeInvalid, E, "screens/0/widgets/0/props/size"},
		{"symbol unknown", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", W: 100, H: 40, Props: map[string]any{"symbol": "rocket"}})
		}, IssueSymbolUnknown, E, "screens/0/widgets/0/props/symbol"},
		{"style color", func(p *Project, _ *ValidateOptions) {
			w := lbl("w_1")
			w.Style.BgColor = "red"
			addTo(p, 0, w)
		}, IssueColorInvalid, E, "screens/0/widgets/0/style/bg_color"},
		{"prop color", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "led", W: 24, H: 24, Props: map[string]any{"color": "#12345"}})
		}, IssueColorInvalid, E, "screens/0/widgets/0/props/color"},
		{"language", func(p *Project, _ *ValidateOptions) { p.Language = "tlh" }, IssueLanguageUnknown, E, "language"},
		{"orientation", func(p *Project, _ *ValidateOptions) { p.Orientation = "upside_down" },
			IssueOrientationInvalid, E, "orientation"},
		{"screen transition", func(p *Project, _ *ValidateOptions) { p.Screens[1].Transition = "spin" },
			IssueTransitionInvalid, E, "screens/1/transition"},
		{"action transition", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "button", Name: "btn", W: 100, H: 40,
				Events: ev("clicked", Action{Type: "goto_screen", Screen: "s_0002", Transition: "spin"})})
		}, IssueTransitionInvalid, E, "screens/0/widgets/0/events/0/action/transition"},
		{"id duplicate", func(p *Project, _ *ValidateOptions) { addTo(p, 0, lbl("w_dup"), lbl("w_dup")) },
			IssueIDDuplicate, E, "screens/0/widgets/1/id"},
		{"statusbar clock setting", func(p *Project, _ *ValidateOptions) { p.Settings.WiFi, p.Settings.StatusbarClock = "off", true },
			IssueStatusbarClockNeedsWiFi, W, "settings/statusbar_clock"},
		{"statusbar widget clock", func(p *Project, _ *ValidateOptions) {
			p.Settings.WiFi = "off"
			addTo(p, 0, Widget{ID: "w_1", Type: "statusbar", W: 320, H: 24})
		}, IssueStatusbarClockNeedsWiFi, W, "screens/0/widgets/0/props/show_clock"},
		{"canvas large", func(p *Project, _ *ValidateOptions) {
			addTo(p, 0, Widget{ID: "w_1", Type: "canvas", W: 300, H: 200})
		}, IssueCanvasLarge, W, "screens/0/widgets/0"},
	}
	covered := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validFixture(t)
			opts := ValidateOptions{}
			c.mutate(&p, &opts)
			issues := Validate(p, opts)
			found := false
			for _, is := range issues {
				if is.Code == c.code && is.Severity == c.sev && is.Path == c.path {
					found = true
					if is.Message == "" {
						t.Errorf("issue without message: %+v", is)
					}
				}
				if is.Severity == SeverityError && is.Code != c.code {
					t.Errorf("collateral error %s at %s: %s", is.Code, is.Path, is.Message)
				}
			}
			if !found {
				t.Fatalf("want %s %s at %s, got:\n%s", c.sev, c.code, c.path, fmtIssues(issues))
			}
		})
		covered[c.code] = true
	}
	for _, code := range IssueCodes() {
		if !covered[code] {
			t.Errorf("issue code %s has no test case", code)
		}
	}
}

func TestIssueCodesMatchContract(t *testing.T) {
	want := "NAME_INVALID NAME_DUPLICATE NAME_REQUIRED SCREEN_NAME_INVALID SCREEN_NAME_DUPLICATE START_SCREEN_MISSING " +
		"UNKNOWN_TYPE UNKNOWN_PROP PROP_INVALID TEXT_TOO_LONG HINT_TOO_LONG OUT_OF_BOUNDS SIZE_INVALID TOO_MANY_SCREENS " +
		"TOO_MANY_WIDGETS NESTING_TOO_DEEP CHILDREN_NOT_ALLOWED TAB_INVALID EVENT_NOT_ALLOWED ACTION_INVALID " +
		"SCREEN_TARGET_MISSING ASSET_MISSING ASSET_BUDGET_EXCEEDED THEME_UNKNOWN BOARD_UNKNOWN LIBRARY_UNKNOWN " +
		"WIFI_MODE_INVALID LINK_REQUIRES_WIFI FONT_UNKNOWN FONT_SIZE_INVALID SYMBOL_UNKNOWN COLOR_INVALID " +
		"LANGUAGE_UNKNOWN ORIENTATION_INVALID TRANSITION_INVALID ID_DUPLICATE " +
		"PARTLY_OUT_OF_BOUNDS STATUSBAR_CLOCK_NEEDS_WIFI CANVAS_LARGE"
	if got := strings.Join(IssueCodes(), " "); got != want {
		t.Fatalf("IssueCodes() =\n%s\nwant\n%s", got, want)
	}
}

func issueAt(issues []Issue, code, path string) bool {
	for _, is := range issues {
		if is.Code == code && is.Path == path {
			return true
		}
	}
	return false
}

func TestValidateContainerBounds(t *testing.T) {
	cases := []struct {
		name   string
		parent Widget
		child  Widget
		code   string
	}{
		{"panel outside", Widget{ID: "w_p", Type: "panel", W: 160, H: 100}, Widget{ID: "w_c", Type: "label", X: 170, W: 50, H: 20}, IssueOutOfBounds},
		{"panel partly", Widget{ID: "w_p", Type: "panel", W: 160, H: 100}, Widget{ID: "w_c", Type: "label", X: 150, W: 50, H: 20}, IssuePartlyOutOfBounds},
		{"tab bar top outside", Widget{ID: "w_p", Type: "tabview", W: 300, H: 200}, Widget{ID: "w_c", Type: "label", Y: 170, W: 50, H: 20}, IssueOutOfBounds},
		{"tab bar top partly", Widget{ID: "w_p", Type: "tabview", W: 300, H: 200}, Widget{ID: "w_c", Type: "label", Y: 150, W: 50, H: 20}, IssuePartlyOutOfBounds},
		{"tab bar left outside", Widget{ID: "w_p", Type: "tabview", W: 300, H: 200, Props: map[string]any{"bar_position": "left"}},
			Widget{ID: "w_c", Type: "label", X: 270, W: 20, H: 20}, IssueOutOfBounds},
		{"tab bar left inside", Widget{ID: "w_p", Type: "tabview", W: 300, H: 200, Props: map[string]any{"bar_position": "left"}},
			Widget{ID: "w_c", Type: "label", X: 0, Y: 170, W: 20, H: 20}, ""},
	}
	for _, tc := range cases {
		p := validFixture(t)
		tc.parent.Children = []Widget{tc.child}
		addTo(&p, 0, tc.parent)
		issues := Validate(p, ValidateOptions{})
		if tc.code == "" {
			if len(issues) != 0 {
				t.Errorf("%s: unexpected issues:\n%s", tc.name, fmtIssues(issues))
			}
			continue
		}
		if !issueAt(issues, tc.code, "screens/0/widgets/0/children/0") {
			t.Errorf("%s: want %s, got:\n%s", tc.name, tc.code, fmtIssues(issues))
		}
	}
}

func TestValidatePortraitSwapsCanvas(t *testing.T) {
	p := validFixture(t)
	p.Orientation = "portrait"
	tall := Widget{ID: "w_tall", Type: "label", Y: 290, W: 120, H: 24}
	wide := Widget{ID: "w_wide", Type: "label", X: 250, W: 60, H: 24}
	addTo(&p, 0, tall, wide)
	issues := Validate(p, ValidateOptions{})
	if len(issues) != 1 || !issueAt(issues, IssueOutOfBounds, "screens/0/widgets/1") {
		t.Fatalf("portrait 240x320 issues:\n%s", fmtIssues(issues))
	}
}

func TestValidateCrossPropRules(t *testing.T) {
	cases := []struct {
		name  string
		w     Widget
		code  string
		path  string
		clean bool
	}{
		{"min above max", Widget{Type: "slider", W: 100, H: 20, Props: map[string]any{"min": 100, "max": 0}}, IssuePropInvalid, "min", false},
		{"empty options", Widget{Type: "dropdown", W: 100, H: 40, Props: map[string]any{"options": []any{}}}, IssuePropInvalid, "options", false},
		{"selected out of range", Widget{Type: "dropdown", W: 100, H: 40, Props: map[string]any{"selected": 3}}, IssuePropInvalid, "selected", false},
		{"visible rows range", Widget{Type: "roller", W: 100, H: 100, Props: map[string]any{"visible_rows": 9}}, IssuePropInvalid, "visible_rows", false},
		{"decimals vs digits", Widget{Type: "spinbox", W: 100, H: 40, Props: map[string]any{"digits": 3, "decimals": 3}}, IssuePropInvalid, "decimals", false},
		{"max needs more digits", Widget{Type: "spinbox", W: 100, H: 40, Props: map[string]any{"max": 5000}}, IssuePropInvalid, "digits", false},
		{"cells overflow", Widget{Type: "table", W: 100, H: 100, Props: map[string]any{"cells": []any{"1", "2", "3", "4", "5", "6", "7"}}}, IssuePropInvalid, "cells", false},
		{"textarea text longer than max_length", Widget{Type: "textarea", W: 100, H: 40, Props: map[string]any{"text": "abcdef", "max_length": 3}}, IssuePropInvalid, "text", false},
		{"custom keypad without keys", Widget{Type: "keypad", W: 100, H: 100, Props: map[string]any{"layout": "custom"}}, IssuePropInvalid, "keys", false},
		{"tabview without tabs", Widget{Type: "tabview", W: 100, H: 100, Props: map[string]any{"tabs": []string{}}}, IssuePropInvalid, "tabs", false},
		{"chart without series", Widget{Type: "chart", W: 100, H: 100, Props: map[string]any{"series": []any{}}}, IssuePropInvalid, "series", false},
		{"too many list items", Widget{Type: "list", W: 100, H: 100, Props: map[string]any{"items": make([]string, 65)}}, IssuePropInvalid, "items", false},
		{"list item not text", Widget{Type: "list", W: 100, H: 100, Props: map[string]any{"items": []any{"ok", 3}}}, IssuePropInvalid, "items", false},
		{"invalid utf8 item", Widget{Type: "list", W: 100, H: 100, Props: map[string]any{"items": []any{"\xff"}}}, IssuePropInvalid, "items", false},
		{"nul in text", Widget{Type: "label", W: 100, H: 20, Props: map[string]any{"text": "a\x00b"}}, IssuePropInvalid, "text", false},
		{"fractional int", Widget{Type: "slider", W: 100, H: 20, Props: map[string]any{"value": 1.5}}, IssuePropInvalid, "value", false},
		{"json float int is fine", Widget{Type: "slider", W: 100, H: 20, Props: map[string]any{"value": float64(60), "max": json.Number("70")}}, "", "", true},
		{"custom keypad with keys", Widget{Type: "keypad", W: 100, H: 100, Props: map[string]any{"layout": "custom", "keys": []any{"A", "B"}}}, "", "", true},
		{"negative spinbox", Widget{Type: "spinbox", W: 100, H: 40, Props: map[string]any{"min": -999, "max": 999, "value": -5}}, "", "", true},
	}
	for _, tc := range cases {
		p := validFixture(t)
		tc.w.ID = "w_1"
		addTo(&p, 0, tc.w)
		issues := Validate(p, ValidateOptions{})
		if tc.clean {
			if len(issues) != 0 {
				t.Errorf("%s: unexpected issues:\n%s", tc.name, fmtIssues(issues))
			}
			continue
		}
		if !issueAt(issues, tc.code, "screens/0/widgets/0/props/"+tc.path) {
			t.Errorf("%s: want %s at props/%s, got:\n%s", tc.name, tc.code, tc.path, fmtIssues(issues))
		}
	}
}

func TestValidateStyleRanges(t *testing.T) {
	p := validFixture(t)
	big, neg := 65, -1
	w := lbl("w_1")
	w.Style = Style{Radius: &big, BgOpacity: &neg}
	addTo(&p, 0, w)
	nine := 9
	p.Screens[1].Style.BorderWidth = &nine
	issues := Validate(p, ValidateOptions{})
	for _, path := range []string{"screens/0/widgets/0/style/radius", "screens/0/widgets/0/style/bg_opacity", "screens/1/style/border_width"} {
		if !issueAt(issues, IssuePropInvalid, path) {
			t.Errorf("missing PROP_INVALID at %s:\n%s", path, fmtIssues(issues))
		}
	}
}

func TestValidateAssetRules(t *testing.T) {
	p := validFixture(t)
	p.Assets = append(p.Assets,
		Asset{ID: "a_bad", File: "assets/../x.png", W: 10, H: 10},
		Asset{ID: "a_zero", File: "assets/zero.png", W: 0, H: 10},
		Asset{ID: "a_huge", File: "assets/huge.jpg", W: 400, H: 400},
		Asset{ID: "a_full", File: "assets/full.jpeg", W: 320, H: 240},
		Asset{ID: "a_tall", File: "assets/tall.png", W: 240, H: 320},
		Asset{ID: "a_sun", File: "assets/sun2.png", W: 10, H: 10},
	)
	issues := Validate(p, ValidateOptions{})
	want := []struct{ code, path string }{
		{IssuePropInvalid, "assets/1/file"}, {IssueSizeInvalid, "assets/2"}, {IssueSizeInvalid, "assets/3"},
		{IssueIDDuplicate, "assets/6/id"},
	}
	for _, w := range want {
		if !issueAt(issues, w.code, w.path) {
			t.Errorf("missing %s at %s", w.code, w.path)
		}
	}
	if len(issues) != len(want) {
		t.Errorf("got %d issues, want %d:\n%s", len(issues), len(want), fmtIssues(issues))
	}
	exact := validFixture(t)
	if issues := Validate(exact, ValidateOptions{AssetSizes: map[string]int64{"a_sun": AssetBudget4MB}}); len(issues) != 0 {
		t.Errorf("budget exactly reached must pass:\n%s", fmtIssues(issues))
	}
}

func TestValidateDeterministic(t *testing.T) {
	p := validFixture(t)
	p.Theme, p.Settings.WiFi = "nope", "off"
	for i := 0; i < 20; i++ {
		w := lbl(fmt.Sprintf("w_%d", i))
		w.X = 300 + i
		w.Props = map[string]any{"zeta": 1, "alpha": 2, "size": 7}
		addTo(&p, i%2, w)
	}
	first := Validate(p, ValidateOptions{})
	for i := 0; i < 5; i++ {
		if again := Validate(p, ValidateOptions{}); !reflect.DeepEqual(first, again) {
			t.Fatal("Validate output order is not deterministic")
		}
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) || HasErrors([]Issue{{Severity: SeverityWarning}}) {
		t.Fatal("warnings are not errors")
	}
	if !HasErrors([]Issue{{Severity: SeverityWarning}, {Severity: SeverityError}}) {
		t.Fatal("error not detected")
	}
}

// errorsOf returns the error-severity issues only.
func errorsOf(issues []Issue) []Issue {
	var out []Issue
	for _, is := range issues {
		if is.Severity == SeverityError {
			out = append(out, is)
		}
	}
	return out
}

func TestValidateInvalidTabviewPropDoesNotCascade(t *testing.T) {
	cases := []struct {
		name  string
		prop  string
		value any
	}{
		{"bar size out of range", "bar_size", 500},
		{"bar size wrong type", "bar_size", "wide"},
		{"bar position unknown", "bar_position", "diagonal"},
		{"tabs not a list", "tabs", "oops"},
		{"too many tabs", "tabs", make([]string, 9)},
	}
	for _, tc := range cases {
		p := validFixture(t)
		addTo(&p, 0, Widget{ID: "w_tv", Type: "tabview", W: 300, H: 200, Props: map[string]any{tc.prop: tc.value},
			Children: []Widget{
				{ID: "w_c0", Type: "label", W: 100, H: 24},
				{ID: "w_c1", Type: "label", Tab: 1, Y: 120, W: 100, H: 24},
			}})
		issues := Validate(p, ValidateOptions{})
		if len(issues) != 1 || !issueAt(issues, IssuePropInvalid, "screens/0/widgets/0/props/"+tc.prop) {
			t.Errorf("%s: want exactly one PROP_INVALID at props/%s, got:\n%s", tc.name, tc.prop, fmtIssues(issues))
		}
	}

	// A valid explicit bar_size still shrinks the child area (200-80 = 120 high).
	p := validFixture(t)
	addTo(&p, 0, Widget{ID: "w_tv", Type: "tabview", W: 300, H: 200, Props: map[string]any{"bar_size": 80},
		Children: []Widget{{ID: "w_c0", Type: "label", Y: 150, W: 100, H: 20}}})
	if issues := Validate(p, ValidateOptions{}); !issueAt(issues, IssueOutOfBounds, "screens/0/widgets/0/children/0") {
		t.Errorf("valid bar_size must still apply:\n%s", fmtIssues(issues))
	}
}

func TestValidateExtremeCoordinates(t *testing.T) {
	cases := []struct {
		name string
		x, y int
		code string
	}{
		{"x min int", math.MinInt, 0, IssueSizeInvalid},
		{"y min int", 0, math.MinInt, IssueSizeInvalid},
		{"x max int", math.MaxInt, 0, IssueSizeInvalid},
		{"y max int", 0, math.MaxInt, IssueSizeInvalid},
		{"x beyond limit", maxCoordinate + 1, 0, IssueSizeInvalid},
		{"y beyond negative limit", 0, -maxCoordinate - 1, IssueSizeInvalid},
		{"x at negative limit", -maxCoordinate, 0, IssueOutOfBounds},
		{"y at limit", 0, maxCoordinate, IssueOutOfBounds},
	}
	for _, tc := range cases {
		p := validFixture(t)
		addTo(&p, 0, Widget{ID: "w_1", Type: "label", X: tc.x, Y: tc.y, W: 10, H: 10})
		issues := Validate(p, ValidateOptions{})
		if len(issues) != 1 || !issueAt(issues, tc.code, "screens/0/widgets/0") {
			t.Errorf("%s: want exactly one %s, got:\n%s", tc.name, tc.code, fmtIssues(issues))
		}
	}
}

func TestClipEcho(t *testing.T) {
	long3 := strings.Repeat("€", 30) // 90 bytes, 64 falls inside a rune
	cases := []struct{ name, in, want string }{
		{"short", "abc", "abc"},
		{"exactly the limit", strings.Repeat("a", maxEchoBytes), strings.Repeat("a", maxEchoBytes)},
		{"one over", strings.Repeat("a", maxEchoBytes+1), strings.Repeat("a", maxEchoBytes) + "…"},
		{"two byte runes", strings.Repeat("ä", 40), strings.Repeat("ä", 32) + "…"},
		{"cut inside a rune", long3, strings.Repeat("€", 21) + "…"},
		{"only continuation bytes", strings.Repeat("\x80", 100), "…"},
	}
	for _, tc := range cases {
		if got := clipEcho(tc.in); got != tc.want {
			t.Errorf("%s: clipEcho = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := clipEcho(long3); !utf8.ValidString(got) {
		t.Errorf("clipped text is not valid UTF-8: %q", got)
	}
}

func TestValidateClipsEchoedValues(t *testing.T) {
	huge := strings.Repeat("ä", 100<<10) // 200 KB
	bounded := func(t *testing.T, label string, issues []Issue, wantCodes ...string) {
		t.Helper()
		for _, is := range issues {
			if len(is.Message) > 512 {
				t.Errorf("%s: %s message is %d bytes: %.80q", label, is.Code, len(is.Message), is.Message)
			}
		}
		for _, code := range wantCodes {
			found := false
			for _, is := range issues {
				found = found || is.Code == code
			}
			if !found {
				t.Errorf("%s: missing %s in:\n%.2000s", label, code, fmtIssues(issues))
			}
		}
	}

	// Project-level values (the board is unknown, so only these checks run).
	p := validFixture(t)
	p.Board, p.Theme, p.Orientation, p.Language, p.StartScreen = huge, huge, huge, huge, huge
	p.Settings.WiFi, p.Settings.Libraries = huge, []string{huge}
	bounded(t, "project", Validate(p, ValidateOptions{}), IssueBoardUnknown, IssueThemeUnknown, IssueOrientationInvalid,
		IssueLanguageUnknown, IssueWiFiModeInvalid, IssueLibraryUnknown, IssueStartScreenMissing)

	// Screens, widgets, events, actions and assets under a known board.
	p = validFixture(t)
	p.Screens[1].Name, p.Screens[1].Transition = huge, huge
	badName := lbl("w_1")
	badName.Name = huge
	badType := lbl("w_2")
	badType.Type = huge
	// An unknown type has no event list, so the "bound twice" branch echoes the event name.
	badType.Name, badType.Events = "t_unknown", []EventBinding{{On: huge, Action: Action{Type: "back"}}, {On: huge, Action: Action{Type: "back"}}}
	unknownProp := lbl("w_3")
	unknownProp.Props = map[string]any{huge: 1}
	badEvent := lbl("w_4")
	badEvent.Name, badEvent.Events = "lbl_ev", []EventBinding{{On: huge, Action: Action{Type: "back"}}, {On: huge, Action: Action{Type: "back"}}}
	badAction := Widget{ID: "w_5", Type: "button", Name: "btn_a", W: 100, H: 40, Events: []EventBinding{
		{On: "clicked", Action: Action{Type: huge}}}}
	badTarget := Widget{ID: "w_6", Type: "button", Name: "btn_b", W: 100, H: 40, Events: []EventBinding{
		{On: "clicked", Action: Action{Type: "goto_screen", Screen: huge, Transition: huge}}}}
	badAsset := Widget{ID: "w_7", Type: "image", W: 64, H: 64, Props: map[string]any{"asset": huge + "z"}} // not the huge asset id declared below
	badColor := lbl("w_8")
	badColor.Style.BgColor = huge
	dupA, dupB := lbl(huge), lbl(huge)
	addTo(&p, 0, badName, badType, unknownProp, badEvent, badAction, badTarget, badAsset, badColor, dupA, dupB)
	p.Assets = append(p.Assets,
		Asset{ID: huge, File: huge, W: 0, H: 0},
		Asset{ID: huge, File: "assets/ok.png", W: 10, H: 10})
	bounded(t, "layout", Validate(p, ValidateOptions{}), IssueScreenNameInvalid, IssueTransitionInvalid, IssueNameInvalid,
		IssueUnknownType, IssueUnknownProp, IssueEventNotAllowed, IssueActionInvalid, IssueScreenTargetMissing,
		IssueAssetMissing, IssueColorInvalid, IssueIDDuplicate, IssuePropInvalid, IssueSizeInvalid)

	// A duplicate screen name.
	p = validFixture(t)
	p.Screens[0].Name, p.Screens[1].Name = "dup", "dup"
	bounded(t, "dup", Validate(p, ValidateOptions{}), IssueScreenNameDuplicate)
}

func TestValidateUnknownPropCap(t *testing.T) {
	build := func(n int) []Issue {
		props := map[string]any{}
		for i := 0; i < n; i++ {
			props[fmt.Sprintf("k%04d", i)] = i
		}
		p := validFixture(t)
		addTo(&p, 0, Widget{ID: "w_1", Type: "label", W: 120, H: 24, Props: props})
		return Validate(p, ValidateOptions{})
	}
	for _, n := range []int{1, maxUnknownPropIssues} {
		issues := build(n)
		if len(issues) != n {
			t.Errorf("%d unknown props: got %d issues:\n%s", n, len(issues), fmtIssues(issues))
		}
	}
	issues := build(maxUnknownPropIssues + 1)
	if len(issues) != maxUnknownPropIssues+1 || !issueAt(issues, IssueUnknownProp, "screens/0/widgets/0/props") {
		t.Errorf("21 unknown props: want 20 plus one summary, got:\n%s", fmtIssues(issues))
	}

	issues = build(1000)
	if len(issues) != maxUnknownPropIssues+1 {
		t.Fatalf("1000 unknown props: got %d issues, want %d:\n%s", len(issues), maxUnknownPropIssues+1, fmtIssues(issues))
	}
	for i, is := range issues {
		if is.Code != IssueUnknownProp || is.Severity != SeverityError {
			t.Errorf("issue %d is %s %s", i, is.Severity, is.Code)
		}
	}
	if !issueAt(issues, IssueUnknownProp, "screens/0/widgets/0/props/k0000") ||
		!issueAt(issues, IssueUnknownProp, "screens/0/widgets/0/props/k0019") ||
		issueAt(issues, IssueUnknownProp, "screens/0/widgets/0/props/k0020") {
		t.Errorf("the first 20 sorted keys must be reported individually:\n%s", fmtIssues(issues))
	}
	last := issues[len(issues)-1]
	if last.Path != "screens/0/widgets/0/props" || !strings.Contains(last.Message, "980") {
		t.Errorf("summary issue = %+v", last)
	}
}

func TestValidateProjectNameAndTimezone(t *testing.T) {
	names := []struct {
		name  string
		value string
		ok    bool
	}{
		{"plain", "Wetterstation", true},
		{"64 runes", strings.Repeat("ä", 64), true},
		{"empty", "", false},
		{"blank", " \t ", false},
		{"65 runes", strings.Repeat("x", 65), false},
		{"tab inside", "a\tb", false},
		{"newline", "a\nb", false},
		{"nul", "a\x00b", false},
		{"invalid utf8", "a\xffb", false},
	}
	for _, tc := range names {
		p := validFixture(t)
		p.Name = tc.value
		issues := Validate(p, ValidateOptions{})
		if tc.ok {
			if len(issues) != 0 {
				t.Errorf("name %s: unexpected issues:\n%s", tc.name, fmtIssues(issues))
			}
			continue
		}
		if len(issues) != 1 || !issueAt(issues, IssuePropInvalid, "name") || issues[0].Severity != SeverityError {
			t.Errorf("name %s: want exactly one PROP_INVALID at name, got:\n%s", tc.name, fmtIssues(issues))
		}
	}

	// Validate must not be looser than NewProject for names NewProject can see.
	for _, tc := range names {
		if !utf8.ValidString(tc.value) {
			continue // NewProject does not check UTF-8; Validate does
		}
		_, err := NewProject(tc.value, "esp32-2432s028r", "night", "de", "", &SeqIDs{})
		if (err == nil) != tc.ok {
			t.Errorf("name %s: NewProject ok=%v, Validate ok=%v", tc.name, err == nil, tc.ok)
		}
	}

	zones := []struct {
		name  string
		value string
		ok    bool
	}{
		{"posix rule", "CET-1CEST,M3.5.0,M10.5.0/3", true},
		{"angle brackets", "<+03>-3", true},
		{"utc", "UTC0", true},
		{"64 bytes", strings.Repeat("A", 64), true},
		{"empty", "", false},
		{"65 bytes", strings.Repeat("A", 65), false},
		{"non ascii", "Zürich", false},
		{"control", "UTC0\n", false},
		{"nul", "UTC\x000", false},
		{"del", "UTC0\x7f", false},
	}
	for _, tc := range zones {
		p := validFixture(t)
		p.Timezone = tc.value
		issues := Validate(p, ValidateOptions{})
		if tc.ok {
			if len(issues) != 0 {
				t.Errorf("timezone %s: unexpected issues:\n%s", tc.name, fmtIssues(issues))
			}
			continue
		}
		if len(issues) != 1 || !issueAt(issues, IssuePropInvalid, "timezone") || issues[0].Severity != SeverityError {
			t.Errorf("timezone %s: want exactly one PROP_INVALID at timezone, got:\n%s", tc.name, fmtIssues(issues))
		}
	}

	// NewProject's own output (including the "UTC0" default) validates.
	p, err := NewProject("Demo", "esp32-2432s028r", "night", "", "", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	if issues := Validate(p, ValidateOptions{}); len(issues) != 0 {
		t.Errorf("fresh project has issues:\n%s", fmtIssues(issues))
	}
}

func TestValidateReservedWidgetNames(t *testing.T) {
	reserved := []string{
		"init", "goto", "back", "theme_colors", "event_index", "async",
		"scr_home", "show_x", "hide_x", "font_big", "x_series", "my_series",
	}
	fine := []string{
		"initial", "gotos", "back_button", "backup", "theme", "event", "asynchronous", "async_x", "xasync",
		"scr", "scrx", "shown", "hidden", "hide", "fonts", "series", "seriesx", "font", "show",
	}
	for _, name := range reserved {
		p := validFixture(t)
		w := lbl("w_1")
		w.Name = name
		addTo(&p, 0, w)
		issues := Validate(p, ValidateOptions{})
		if len(issues) != 1 || !issueAt(issues, IssueNameInvalid, "screens/0/widgets/0/name") {
			t.Errorf("name %q: want exactly one NAME_INVALID, got:\n%s", name, fmtIssues(issues))
		}
	}
	for _, name := range fine {
		p := validFixture(t)
		w := lbl("w_1")
		w.Name = name
		addTo(&p, 0, w)
		if issues := Validate(p, ValidateOptions{}); len(issues) != 0 {
			t.Errorf("name %q must be allowed, got:\n%s", name, fmtIssues(issues))
		}
	}
}

func TestPropProblemStringLimit(t *testing.T) {
	v := &validator{}
	limited := &PropSpec{Name: "s", Kind: PropString, MaxLen: 5}
	for _, tc := range []struct {
		val  string
		code string
	}{
		{"abcde", ""},
		{"abcdef", IssueTextTooLong},
		{"ää", ""},
		{"äää", IssueTextTooLong}, // 6 bytes: the limit counts bytes, not runes
	} {
		if code, _ := v.propProblem(limited, tc.val); code != tc.code {
			t.Errorf("MaxLen 5, %q: code %q, want %q", tc.val, code, tc.code)
		}
	}
	unset := &PropSpec{Name: "t", Kind: PropText}
	if code, _ := v.propProblem(unset, strings.Repeat("x", MaxTextBytes)); code != "" {
		t.Errorf("MaxLen 0 must fall back to MaxTextBytes, got %q", code)
	}
	if code, _ := v.propProblem(unset, strings.Repeat("x", MaxTextBytes+1)); code != IssueTextTooLong {
		t.Errorf("MaxLen 0 must cap at MaxTextBytes, got %q", code)
	}
}

func TestValidateScreenNames(t *testing.T) {
	p := validFixture(t)
	p.Screens = append(p.Screens, Screen{ID: "s_0003", Name: "home", Transition: "none", Widgets: []Widget{}})
	p.Screens[1].Name = "Bad"
	p.Screens = append(p.Screens, Screen{ID: "s_0004", Name: "Bad", Transition: "none", Widgets: []Widget{}})
	issues := Validate(p, ValidateOptions{})
	want := []struct{ code, path string }{
		{IssueScreenNameInvalid, "screens/1/name"},
		{IssueScreenNameDuplicate, "screens/2/name"},
		{IssueScreenNameInvalid, "screens/3/name"}, // an invalid name is reported as invalid, not as a duplicate
	}
	for _, w := range want {
		if !issueAt(issues, w.code, w.path) {
			t.Errorf("missing %s at %s", w.code, w.path)
		}
	}
	if len(issues) != len(want) {
		t.Errorf("got %d issues, want %d:\n%s", len(issues), len(want), fmtIssues(issues))
	}
}
