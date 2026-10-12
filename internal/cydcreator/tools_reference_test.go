package cydcreator

import (
	"context"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestAPIReferenceDescribesThisProject(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	logo, _, err := env.svc.PutAsset(ctx, p.ID, "logo.png", pngBytes(t, 8, 8, false))
	if err != nil {
		t.Fatal(err)
	}
	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Settings.Libraries = []string{"dht"}
	layout.Screens[0].Widgets = append(layout.Screens[0].Widgets,
		Widget{ID: "w_btn", Type: "button", Name: "btn_refresh", X: 10, Y: 10, W: 120, H: 40,
			Events: []EventBinding{{On: "clicked", Action: Action{Type: "logic", Hint: "reload the weather"}}}},
		Widget{ID: "w_chart", Type: "chart", Name: "chart_temp", X: 10, Y: 60, W: 200, H: 120, Props: map[string]any{"series": []any{"in", "out"}}},
		Widget{ID: "w_lbl", Type: "label", Name: "lbl_big", X: 10, Y: 190, W: 100, H: 40, Props: map[string]any{"size": float64(28)}},
		Widget{ID: "w_gauge", Type: "gauge", Name: "gauge_speed", X: 180, Y: 10, W: 120, H: 120})
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, v, OriginUser); err != nil {
		t.Fatal(err)
	}
	topics := map[string]string{}
	var badErr error
	inPhase(t, env, runner, p, PhaseCoding, func(ctx context.Context, jobID string) {
		for _, topic := range ReferenceTopics {
			text, err := env.svc.APIReference(ctx, jobID, topic)
			if err != nil {
				t.Errorf("%s: %v", topic, err)
			}
			topics[topic] = text
		}
		_, badErr = env.svc.APIReference(ctx, jobID, "rust")
	})
	wantCode(t, badErr, CodeBadRequest)
	checks := map[string][]string{
		"overview":  {"Wetter", "app_setup()", "cyd::ui_async", "1966080 bytes"},
		"cyd":       {"namespace cyd", "ui_async"},
		"lvgl":      {"lv_label_set_text", "cyd::ui_async"},
		"widgets":   {"lv_obj_t *ui_scr_home; enum UI_SCREEN_HOME (start screen)", "lv_obj_t *ui_btn_refresh;", "void on_btn_refresh_clicked(lv_event_t *e); // reload the weather", "lv_chart_series_t *ui_chart_temp_series[2];", "ui_font_montserrat_28", "lv_obj_get_child(ui_gauge_speed, 0)", "exist only for named widgets", "LV_IMAGE_DECLARE(ui_img_" + logo.ID + ");"},
		"libraries": {"dht: DHT sensor library 1.4.7, #include <DHT.h>, enabled", "arduinojson"},
		"board":     {"ESP32-2432S028R", "RgbLed", "LightSensor"},
	}
	for topic, wants := range checks {
		for _, want := range wants {
			if !strings.Contains(topics[topic], want) {
				t.Errorf("topic %s lacks %q:\n%s", topic, want, topics[topic])
			}
		}
	}
	// The generated names follow the generator: all four theme font roles
	// (night: montserrat 16, small 14, large 20, mono jetbrains_mono 16), the
	// gauge needle length of ui.h and the catalog's single default series.
	for _, want := range []string{"ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20",
		"ui_font_jetbrains_mono_16", "length " + strconv.Itoa(gaugeNeedle(&Widget{W: 120, H: 120}))} {
		if !strings.Contains(topics["widgets"], want) {
			t.Errorf("topic widgets lacks %q:\n%s", want, topics["widgets"])
		}
	}
}

// TestAPIReferenceMatchesTheTemplate pins the facts the reference states
// against the embedded template, the generator and the catalog, so a template
// or pin change cannot leave the agent with a stale reference.
func TestAPIReferenceMatchesTheTemplate(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := fs.ReadFile(TemplateFS(), name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	cydH := read("lib/cyd_template/src/cyd.h")
	header, err := templateHeader()
	if err != nil || !strings.Contains(header, cydH) {
		t.Fatalf("the cyd topic must contain cyd.h verbatim (err %v)", err)
	}
	all := referenceLVGL + referenceRuntimeNotes + referenceBoard(mustBoard(t, "esp32-2432s028r"), Project{Orientation: "landscape"})
	// Every API name the reference uses must exist in cyd.h.
	for _, name := range []string{"ui_async", "get_json_async", "post_json_async", "connected()", "on_change", "synced()",
		"format(", "backlight(", "rgb(", "light_level(", "beep(", "RgbLed", "LightSensor", "Speaker", "SdCard"} {
		if !strings.Contains(cydH, name) || !strings.Contains(all, name) {
			t.Errorf("%q must appear in cyd.h and in the reference", name)
		}
	}
	// The ThemeColors fields, exactly and in declaration order.
	block := regexp.MustCompile(`(?s)struct ThemeColors \{(.*?)\};`).FindStringSubmatch(cydH)
	if block == nil {
		t.Fatal("cyd.h has no struct ThemeColors")
	}
	var fields []string
	lastIdent := regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*$`)
	for _, decl := range strings.Split(block[1], ";") {
		for _, part := range strings.Split(decl, ",") {
			if m := lastIdent.FindStringSubmatch(strings.TrimSpace(part)); m != nil {
				fields = append(fields, m[1])
			}
		}
	}
	listed := regexp.MustCompile(`ui_theme_colors\(\) returns the theme: ([^\n]*)\.\n`).FindStringSubmatch(referenceLVGL)
	if listed == nil {
		t.Fatal("referenceLVGL does not list the ThemeColors fields")
	}
	if names := strings.Split(regexp.MustCompile(`\s*\([^)]*\)`).ReplaceAllString(listed[1], ""), ", "); !slices.Equal(names, fields) {
		t.Errorf("ThemeColors fields: reference %v, cyd.h %v", names, fields)
	}
	// Runtime facts, each grepped from the source that defines it.
	src := map[string]string{
		"main":    read("lib/cyd_template/src/cyd_main.cpp"),
		"http":    read("lib/cyd_template/src/cyd_http.cpp"),
		"wifi":    read("lib/cyd_template/src/cyd_wifi.cpp"),
		"storage": read("lib/cyd_template/src/cyd_storage.cpp"),
		"board":   read("lib/cyd_template/src/cyd_board.cpp"),
		"gen":     genNavigation,
	}
	for _, pin := range []struct{ src, code, text string }{
		{"main", "SET_LOOP_TASK_STACK_SIZE(12 * 1024);", "has a 12 KiB stack"},
		{"gen", "#define UI__HISTORY 16\n", "a history of 16 screens"},
		{"http", "if (timeout_ms > 65535) timeout_ms = 65535;", "timeout_ms is capped at 65535"},
		{"wifi", "  char buf[64];\n  size_t n = strftime(buf, sizeof(buf), strftime_fmt, &tmv);", "longer than 63 bytes"},
		{"storage", "strlen(key) <= 15) return;", "keys have at most 15 characters"},
		{"board", "return 100 - (raw * 100) / 4095;", "light_level() returns 0 (dark) to 100 (bright)"},
		{"board", "#else\n  return -1;\n#endif", "light_level() returns -1"},
	} {
		if !strings.Contains(src[pin.src], pin.code) || !strings.Contains(all, pin.text) {
			t.Errorf("%s source %q / reference %q out of step", pin.src, pin.code, pin.text)
		}
	}
	// HTTP limits of cyd_config.h.
	cfg := read("lib/cyd_template/src/cyd_config.h")
	for macro, fact := range map[string]string{"CYD_HTTP_MAX_BODY 32768": "above 32 KiB", "CYD_HTTP_QUEUE_LEN 8": "at most 8 wait"} {
		if !strings.Contains(cfg, macro) || !strings.Contains(referenceRuntimeNotes, fact) {
			t.Errorf("cyd_config.h %q / runtime note %q out of step", macro, fact)
		}
	}
	// Built-in fonts: lv_conf.h enables only Montserrat 14.
	conf := read("lib/cyd_template/config/lv_conf.h")
	if got := regexp.MustCompile(`(?m)^#define LV_FONT_MONTSERRAT_\d+ 1`).FindAllString(conf, -1); !slices.Equal(got, []string{"#define LV_FONT_MONTSERRAT_14 1"}) ||
		!strings.Contains(referenceLVGL, "lv_font_montserrat_14 is the only built-in font") {
		t.Errorf("built-in fonts %v out of step with the reference", got)
	}
	if !slices.Equal(genFontRanges, []string{"0x20-0x7E", "0xA0-0x17F"}) || !strings.Contains(referenceLVGL, "U+0020-U+007E and U+00A0-U+017F") {
		t.Errorf("font ranges %v out of step with the reference", genFontRanges)
	}
	// Pinned versions of the generated platformio.ini.
	versions := []string{strings.TrimPrefix(genPlatform, "espressif32@")}
	for _, dep := range genTemplateLibDeps {
		if strings.HasPrefix(dep, "lvgl/") || strings.HasPrefix(dep, "lovyan03/") {
			versions = append(versions, dep[strings.LastIndex(dep, "@")+1:])
		}
	}
	for _, v := range versions {
		if !strings.Contains(referenceLVGL, " "+v) {
			t.Errorf("referenceLVGL lacks version %s", v)
		}
	}
	// Every curated library has an include header.
	for _, l := range Libraries() {
		if libraryHeaders[l.ID] == "" {
			t.Errorf("library %s has no header in libraryHeaders", l.ID)
		}
	}
	// Name rules come from the validator.
	overview := referenceOverview(mustBoard(t, "esp32-2432s028r"), Project{Name: "x", Orientation: "landscape", Theme: "night"})
	for _, name := range append(append(slices.Clone(reservedNames), reservedNamePrefixes...), reservedNameSuffixes...) {
		if !strings.Contains(overview, name) {
			t.Errorf("overview lacks the reserved name %q", name)
		}
	}
}

func mustBoard(t *testing.T, id string) Board {
	t.Helper()
	b, err := BoardByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
