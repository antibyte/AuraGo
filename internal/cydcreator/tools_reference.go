package cydcreator

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// API reference topics (cyd_creator_api_reference).
var ReferenceTopics = []string{"overview", "cyd", "lvgl", "widgets", "libraries", "board"}

// libraryHeaders maps curated libraries to the header app code includes.
var libraryHeaders = map[string]string{
	"arduinojson": "ArduinoJson.h", "pubsubclient": "PubSubClient.h", "onewire": "OneWire.h",
	"dallastemperature": "DallasTemperature.h", "dht": "DHT.h", "bme280": "Adafruit_BME280.h",
	"bmp280": "Adafruit_BMP280.h", "neopixel": "Adafruit_NeoPixel.h",
}

// referenceLVGL states the toolchain facts of the builder image and the
// generated platformio.ini (TestAPIReferenceMatchesTheTemplate pins them).
const referenceLVGL = `LVGL 9 in CYD Creator apps (LVGL 9.5.0, LovyanGFX 1.2.30, Arduino-ESP32 2.0.17 on espressif32 6.13.0, GCC 8.4)
- App code is C++11 (gnu++11) without RTTI: no std::make_unique, auto lambda parameters, std::optional, structured bindings, if constexpr, dynamic_cast or typeid. C files are gnu99.
- Write LVGL 9 names: lv_button_create, lv_image_create/lv_image_set_src, lv_buttonmatrix_*, lv_obj_add_event_cb, lv_obj_remove_flag, lv_obj_remove_state, lv_display_get_default, lv_screen_active. LVGL 8 code is not supported: a few old names (lv_btn_create, lv_scr_act, lv_obj_clear_flag) still compile through compatibility macros, removed APIs (lv_meter_*, lv_disp_drv_t, lv_disp_draw_buf_t, lv_chart_set_axis_tick) do not, and lv_event_get_target(e) returns void * (C++ does not convert it to lv_obj_t *): use lv_event_get_target_obj(e).
- LVGL runs on the Arduino loop task only. app_setup(), app_loop(), event hooks, lv_timer callbacks and cyd::http callbacks run there and may call lv_* directly. Code on any other task (FreeRTOS tasks, esp_timer, Wi-Fi event or library callbacks) must wrap UI work in cyd::ui_async([]{ ... }); never call cyd::ui_async from an interrupt handler.
- Never block the loop: no delay(), no while-waits, no synchronous HTTP. Use lv_timer_create(cb, period_ms, user_data) or millis() checks in app_loop(). The loop task has a 12 KiB stack: keep large buffers off the stack.
- Text: lv_label_set_text(ui_x, "..."), lv_label_set_text_fmt(ui_x, "%d °C", v). A button's text is its child label: lv_label_set_text(lv_obj_get_child(ui_btn, 0), "...").
- Values: lv_slider_set_value(ui_x, v, LV_ANIM_OFF), lv_bar_set_value(ui_x, v, LV_ANIM_ON), lv_arc_set_value(ui_x, v), lv_spinbox_set_value(ui_x, v).
- Read input: lv_slider_get_value(ui_x), lv_obj_has_state(ui_x, LV_STATE_CHECKED) (switch/checkbox), lv_dropdown_get_selected(ui_x), lv_roller_get_selected(ui_x), lv_textarea_get_text(ui_x), lv_spinbox_get_value(ui_x).
- State: lv_obj_add_state/lv_obj_remove_state(ui_x, LV_STATE_CHECKED | LV_STATE_DISABLED); visibility: lv_obj_add_flag/lv_obj_remove_flag(ui_x, LV_OBJ_FLAG_HIDDEN) (message boxes: ui_show_<name>()/ui_hide_<name>() instead).
- Charts: lv_chart_set_next_value(ui_x, ui_x_series[0], v); lv_chart_refresh(ui_x).
- LEDs: lv_led_on(ui_x), lv_led_off(ui_x), lv_led_set_brightness(ui_x, 0..255), lv_led_set_color(ui_x, lv_color_hex(0x00FF00)).
- Tables: lv_table_set_cell_value(ui_x, row, col, "text"). QR codes: lv_qrcode_update(ui_x, data, len).
- Colors: lv_color_hex(0xRRGGBB). ui_theme_colors() returns the theme: bg, surface, surface_alt, text, text_muted, accent, on_accent, success, warning, danger, border, focus (lv_color_t), border_width, radius, font_small, font_normal, font_large, font_mono (const lv_font_t *), bracket_buttons.
- Fonts: use the theme fonts above or the ui_font_<family>_<size> fonts listed in topic widgets; LVGL's lv_font_montserrat_14 is the only built-in font (no other lv_font_* exists). Every converted font holds U+0020-U+007E and U+00A0-U+017F (printable ASCII, Latin-1 Supplement, Latin Extended-A), the LV_SYMBOL_* icons and the characters used in the layout's texts. Other characters your code shows (euro sign, arrows, Greek, Cyrillic, emoji, ...) are not drawn: avoid them or put them into a layout text (a hidden label works).
- Event hooks receive lv_event_t *e: lv_event_get_target_obj(e) is the object the event fired on (for list and message-box hooks the clicked item or button), lv_event_get_code(e) the event. ui_event_index(e) is the 0-based index of the list item, message-box button or keypad key (keys in reading order; row breaks are not keys), else -1.
- Screens: ui_goto(UI_SCREEN_<NAME>, UI_TR_FADE) (going to the current screen does nothing); ui_back() returns through a history of 16 screens. Message boxes: ui_show_<name>(), ui_hide_<name>().
- Do not create or delete screens or the generated widgets in code; change the layout with cyd_creator_update_layout instead. Creating extra child objects at runtime (e.g. list rows) is fine, but the generated hooks only fire for widgets and list items of the layout: register your own lv_obj_add_event_cb on objects you create.`

// APIReference returns one reference topic for the job's working copy.
func (s *Service) APIReference(ctx context.Context, jobID, topic string) (string, error) {
	run, err := s.jobTool(jobID, ToolAPIReference)
	if err != nil {
		return "", err
	}
	if !slices.Contains(ReferenceTopics, topic) {
		return "", NewError(CodeBadRequest, "unknown topic; use one of "+strings.Join(ReferenceTopics, ", "),
			map[string]any{"topic": clipEcho(topic)})
	}
	root, err := os.OpenRoot(run.stage)
	if err != nil {
		return "", s.internalError("api reference", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return "", s.internalError("api reference", err)
	}
	board, err := BoardByID(p.Board)
	if err != nil {
		return "", err
	}
	switch topic {
	case "cyd":
		return templateHeader()
	case "lvgl":
		return referenceLVGL, nil
	case "widgets":
		return referenceWidgets(p), nil
	case "libraries":
		return referenceLibraries(p), nil
	case "board":
		return referenceBoard(board, p), nil
	}
	return referenceOverview(board, p), nil
}

// templateHeader is the exact cyd.h of the embedded template (plan 1b);
// TemplateFS() is rooted at template/.
func templateHeader() (string, error) {
	data, err := fs.ReadFile(TemplateFS(), "lib/cyd_template/src/cyd.h")
	if err != nil {
		return "", NewError(CodeInternal, "the template API is not available", nil)
	}
	return "cyd.h (template " + TemplateVersion + "), included by ui.h:\n\n" + string(data) + "\n" + referenceRuntimeNotes, nil
}

// referenceRuntimeNotes are facts about the template runtime (plan 1b) every
// app must respect; the cyd topic appends them to the header.
const referenceRuntimeNotes = `Runtime notes (template 1.0.0):
- cyd::http::get_json_async/post_json_async: check cyd::wifi::connected() first. Requests run one at a time on one worker task that the AuraGo link shares (a firmware download delays them); at most 8 wait, a further request fails at once with error "busy". The callback runs on the loop task (status 0 on transport errors). Responses above 32 KiB fail with "response too large"; other errors are "offline", "unsupported url" (only http:// and https://), "http <status>" for a non-2xx answer (doc still holds a JSON body), "invalid json: ..." and "empty response". timeout_ms is capped at 65535.
- https through cyd::http is encrypted but does NOT verify server certificates (no CA bundle on 4 MB boards). Never send passwords, API keys, tokens or other secrets through it; prefer public APIs that need no secret.
- cyd::time::now() and format() use the project timezone; format() returns "" until cyd::time::synced() is true (NTP after Wi-Fi connects) and for results longer than 63 bytes.
- cyd::storage (NVS namespace "app", safe from any task): keys have at most 15 characters (longer keys are never stored); every call opens NVS, so write on change, not in a loop.
- The template does not mount LittleFS. An app that needs files calls LittleFS.begin(true, "/littlefs", 10, "littlefs") once in app_setup() (include <LittleFS.h>). Small settings belong in cyd::storage (NVS).
- Builds are not byte-for-byte reproducible (every firmware embeds its own build id): judge a build by its status and diagnostics, never by firmware hashes.`

func referenceOverview(b Board, p Project) string {
	info := boardInfo(b, p.Orientation)
	ram := "no PSRAM: keep buffers small, Wi-Fi and TLS need tens of KB of the internal heap"
	if b.PSRAM {
		ram = "PSRAM available"
	}
	flash := fmt.Sprintf("the app slot holds %d bytes", info.AppSlotBytes)
	if b.FlashMB <= 4 {
		flash += " and is tight: measured builds use 70-82 % of it (template, LVGL, Wi-Fi/TLS and fonts; 350-550 KB stay free) and every extra font size costs tens of KB, so keep code, tables, fonts and images small"
	}
	wifi := ""
	if p.Settings.WiFi == "off" {
		wifi = ` Wi-Fi is off in this project: every request fails with "offline" until set_settings sets wifi to optional or required.`
	}
	return fmt.Sprintf(`CYD Creator project %q: board %s (%s), %dx%d %s, theme %s, Wi-Fi %s, AuraGo link %t.
Files you own: src/app/**, files ending in .cpp, .h, .hpp or .c, at most 4 folder levels below src/app, paths up to 160 bytes; every segment has 1-64 ASCII letters, digits, _ . or -, starts with a letter, digit or _, does not end with a dot and is no Windows device name (con, aux, nul, com1, ...). Paths are exact-case: a name that differs only in letter case from an existing file or folder is a conflict. At most 64 files, 256 KiB each, 2 MiB together, UTF-8 text without NUL bytes.
app_setup() runs once after ui_init(), app_loop() on every loop pass (scaffold: src/app/app_main.cpp); define each once (empty weak defaults exist). Include "ui.h" (generated symbols, includes cyd.h).
Generated and read-only: src/generated/** (ui.h, ui.cpp, ui_theme.cpp, ui_assets.cpp, ui_hooks_weak.cpp, fonts.json), include/** (board_config.h), platformio.ini, partitions.csv, lib/cyd_template/**; cyd_creator_read_file shows them rendered from your working copy. The layout (cyd.project.json) changes only through cyd_creator_update_layout; images (assets) come only from the user's uploads.
Rules:
- LVGL 9 only on the loop task; other tasks use cyd::ui_async(). C++11 without RTTI. See topic lvgl.
- Never block app_loop (no delay()); use lv_timer_create or millis().
- Network: cyd::http::get_json_async/post_json_async; check cyd::wifi::connected() first. https does not verify certificates: never send secrets through it.%s
- Flash: %s. Images count with their decoded size (w*h*2 bytes, w*h*3 with transparency) against an asset budget of %d bytes. RAM: %s.
- Only the curated libraries (topic libraries); enable one with update_layout op set_settings.
- Widget names (needed for a ui_<name> handle; a widget with events must have one): ^[a-z][a-z0-9_]{0,31}$, unique in the project; reserved are %s, the prefixes %s, the suffix %s and img_<asset id>. A message box needs at least one button (an op that empties its buttons is refused).
- Implement every logic hook listed in topic widgets; an unimplemented hook only logs a warning once.
Topics: cyd (template API and runtime notes), lvgl (LVGL 9 usage), widgets (generated names of this project), libraries, board.`,
		p.Name, info.Label, info.Chip, info.ScreenW, info.ScreenH, p.Orientation, p.Theme, p.Settings.WiFi,
		p.Settings.AuraGoLink, wifi, flash, AssetBudget(b.FlashMB), ram, strings.Join(reservedNames, ", "),
		strings.Join(reservedNamePrefixes, ", "), strings.Join(reservedNameSuffixes, ", "))
}

// referenceTypeNotes explain how app code reaches a generated widget of each
// type (generate_ui.go / generate_widgets.go); topic widgets lists the notes of
// the types its named widgets use.
var referenceTypeNotes = map[string]string{
	"button":    "button: the text is the child label: lv_label_set_text(lv_obj_get_child(ui_x, 0), \"...\").",
	"msgbox":    "msgbox: ui_x is the box, its parent the hidden backdrop on the top layer; show and hide it only with ui_show_x()/ui_hide_x(). Every button closes the box; a clicked hook gets the button index from ui_event_index(e).",
	"list":      "list: the items are buttons; the clicked hook is registered on the layout's items (index via ui_event_index(e)); rows added with lv_list_add_button at runtime need their own lv_obj_add_event_cb.",
	"keypad":    "keypad: a button matrix; the value_changed hook gets the key index from ui_event_index(e), the text from lv_buttonmatrix_get_button_text(ui_x, index).",
	"textarea":  "textarea: one keyboard per screen opens on focus; read lv_textarea_get_text(ui_x); ready fires on the keyboard's OK; password fields show *.",
	"spinbox":   "spinbox: the value is an integer (lv_spinbox_get_value/lv_spinbox_set_value; with decimals d the display shows value / 10^d); the -/+ buttons are separate objects beside it.",
	"tabview":   "tabview: lv_tabview_get_tab_active(ui_x), lv_tabview_set_active(ui_x, index, LV_ANIM_OFF); its child widgets sit on the tab pages.",
	"dropdown":  "dropdown: lv_dropdown_get_selected(ui_x) (0-based).",
	"roller":    "roller: lv_roller_get_selected(ui_x) (0-based).",
	"switch":    "switch: lv_obj_has_state(ui_x, LV_STATE_CHECKED).",
	"checkbox":  "checkbox: lv_obj_has_state(ui_x, LV_STATE_CHECKED).",
	"led":       "led: lv_led_on/lv_led_off(ui_x), lv_led_set_brightness(ui_x, 0..255), lv_led_set_color(ui_x, color).",
	"image":     "image: lv_image_set_src(ui_x, &ui_img_<asset id>).",
	"qrcode":    "qrcode: lv_qrcode_update(ui_x, text, strlen(text)).",
	"canvas":    "canvas: an RGB565 buffer of the widget size is allocated at start (from PSRAM when the board has it); draw only when lv_canvas_get_draw_buf(ui_x) != NULL (lv_canvas_set_px, lv_canvas_init_layer/lv_canvas_finish_layer).",
	"statusbar": "statusbar: the runtime updates its Wi-Fi icon and clock (when shown) every second; a long press opens the Wi-Fi setup unless the layout binds long_pressed.",
}

// referenceWidgets lists the C symbols the generator emits for this layout
// (overview "Generated symbol names"), with the generator's own naming,
// series counts, needle lengths and font set.
func referenceWidgets(p Project) string {
	var b strings.Builder
	b.WriteString("Generated symbols of this project (src/generated/ui.h):\n\nScreens:\n")
	for i := range p.Screens {
		sc := &p.Screens[i]
		start := ""
		if sc.ID == p.StartScreen {
			start = " (start screen)"
		}
		fmt.Fprintf(&b, "- lv_obj_t *ui_scr_%s; enum %s%s\n", sc.Name, screenEnum(sc), start)
	}
	b.WriteString("Transitions: UI_TR_NONE UI_TR_FADE UI_TR_SLIDE_LEFT UI_TR_SLIDE_RIGHT UI_TR_SLIDE_UP UI_TR_SLIDE_DOWN\n")
	b.WriteString("void ui_goto(ui_screen_t s, ui_transition_t t); void ui_back(void); int ui_event_index(lv_event_t *e); const cyd::ThemeColors &ui_theme_colors(void);\n\nNamed widgets:\n")
	var hooks, extras, types []string
	p.WalkWidgets(func(screen *Screen, _ *Widget, w *Widget) bool {
		if w.Name == "" {
			return true
		}
		spec, _ := widgetSpecRef(w.Type)
		fmt.Fprintf(&b, "- lv_obj_t *ui_%s; // %s on %s\n", w.Name, w.Type, screen.Name)
		if !slices.Contains(types, w.Type) {
			types = append(types, w.Type)
		}
		switch w.Type {
		case "msgbox":
			extras = append(extras, fmt.Sprintf("- void ui_show_%s(void); void ui_hide_%s(void);", w.Name, w.Name))
		case "chart":
			extras = append(extras, fmt.Sprintf("- lv_chart_series_t *ui_%s_series[%d];", w.Name, len(genPropList(w, spec, "series"))))
		case "gauge":
			n := gaugeNeedle(w)
			extras = append(extras, fmt.Sprintf("- gauge %s: the needle is lv_obj_get_child(ui_%s, 0), length %d; move it with lv_scale_set_line_needle_value(ui_%s, lv_obj_get_child(ui_%s, 0), %d, value)",
				w.Name, w.Name, n, w.Name, w.Name, n))
		}
		for _, ev := range w.Events {
			if ev.Action.Type != "logic" {
				continue
			}
			hook := fmt.Sprintf("- void on_%s_%s(lv_event_t *e);", w.Name, ev.On)
			if hint := cComment(ev.Action.Hint); hint != "" {
				hook += " // " + hint
			}
			hooks = append(hooks, hook)
		}
		return true
	})
	if len(types) == 0 {
		b.WriteString("- none\n")
	}
	if len(extras) > 0 {
		b.WriteString("\nHelpers:\n" + strings.Join(extras, "\n") + "\n")
	}
	b.WriteString("Message-box show/hide functions and chart series arrays exist only for named widgets; unnamed widgets have no ui_ handle (name them with update_layout first).\n")
	var notes []string
	for _, typ := range types {
		if note, ok := referenceTypeNotes[typ]; ok {
			notes = append(notes, "- "+note)
		}
	}
	if len(notes) > 0 {
		b.WriteString("\nUsing these widgets:\n" + strings.Join(notes, "\n") + "\n")
	}
	if len(p.Assets) > 0 {
		b.WriteString("\nImages (use lv_image_set_src(obj, &ui_img_<id>)):\n")
		for _, a := range p.Assets {
			fmt.Fprintf(&b, "- LV_IMAGE_DECLARE(%s); // %s, %dx%d\n", imageSymbol(a.ID), a.File, a.W, a.H)
		}
	}
	b.WriteString("\nLogic hooks to implement in src/app (the weak defaults only log a warning once):\n")
	if len(hooks) == 0 {
		b.WriteString("- none\n")
	} else {
		b.WriteString(strings.Join(hooks, "\n") + "\n")
	}
	theme, err := ThemeByID(p.Theme)
	if err != nil {
		fmt.Fprintf(&b, "\nFonts: the theme %q is unknown; fix it with update_layout op set_theme.\n", clipEcho(p.Theme))
		return b.String()
	}
	var names []string
	for _, f := range projectFonts(p, theme) {
		names = append(names, f.symbol())
	}
	roles := themeFontRoles(theme)
	fmt.Fprintf(&b, "\nFonts (const lv_font_t, use &name; no other font exists): %s\nTheme roles in ui_theme_colors(): font_normal %s, font_small %s, font_large %s, font_mono %s\n",
		strings.Join(names, ", "), roles.Normal.symbol(), roles.Small.symbol(), roles.Large.symbol(), roles.Mono.symbol())
	return b.String()
}

func referenceLibraries(p Project) string {
	var b strings.Builder
	b.WriteString("Curated libraries (offline builds; pinned versions). Only these can be used, and the header of a library that is not enabled does not compile.\n")
	for _, l := range Libraries() {
		state := "not enabled"
		if slices.Contains(p.Settings.Libraries, l.ID) {
			state = "enabled"
		}
		fmt.Fprintf(&b, "- %s: %s %s, #include <%s>, %s\n", l.ID, l.Name, l.Version, libraryHeaders[l.ID], state)
	}
	b.WriteString("ArduinoJson (version 7: use JsonDocument; StaticJsonDocument and DynamicJsonDocument are deprecated version 6 names) is always available through cyd.h. Enable another library with update_layout op set_settings, e.g. {\"op\":\"set_settings\",\"set\":{\"libraries\":[\"dht\"]}} (the list replaces the old one).\n")
	return b.String()
}

func referenceBoard(board Board, p Project) string {
	info := boardInfo(board, p.Orientation)
	var b strings.Builder
	fmt.Fprintf(&b, "Board %s (%s, %s status)\nChip %s, flash %d MB, PSRAM %t, app slot %d bytes\n", info.Label, info.ID,
		info.Status, info.Chip, info.FlashMB, info.PSRAM, info.AppSlotBytes)
	fmt.Fprintf(&b, "Display %s, %dx%d in %s orientation; touch %s\n", info.Display, info.ScreenW, info.ScreenH, p.Orientation, info.Touch)
	b.WriteString("Features (cyd::board::has(cyd::board::Feature::...)):\n")
	feature := map[string]string{
		"rgb_led":      "RgbLed: cyd::board::rgb(r, g, b), 0-255 each",
		"light_sensor": "LightSensor: cyd::board::light_level() returns 0 (dark) to 100 (bright)",
		"speaker":      "Speaker: cyd::board::beep(hz, ms), non-blocking",
		"sd_card":      "SdCard: a card slot the runtime does not mount (read the SPI note in cyd.h first)",
	}
	if len(info.Features) == 0 {
		b.WriteString("- none\n")
	}
	for _, f := range info.Features {
		b.WriteString("- " + feature[f] + "\n")
	}
	b.WriteString("Without the feature these calls do nothing (light_level() returns -1).\n")
	b.WriteString("Backlight: cyd::board::backlight(percent), 0-100. Pins used by the display, touch and features are taken; do not reconfigure them. LEDC channels: see cyd.h (never tone() or analogWrite()).\n")
	if board.Display.Driver == "NV3041A" {
		native := board.Display.NativeOrientation
		fmt.Fprintf(&b, "This board's NV3041A panel cannot rotate: only %s and %s_flipped work (any other orientation is ORIENTATION_INVALID).\n", native, native)
	}
	return b.String()
}
