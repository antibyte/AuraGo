package cydcreator

import (
	"strconv"
	"strings"
)

// Style helpers of ui_theme.cpp in declaration order (plan 1b).
var genHelperOrder = []string{"screen", "panel", "statusbar", "button", "input", "slider", "switch", "checkbox", "bar", "arc", "selector", "items", "line", "muted"}

// genTypeHelper maps a widget type to the style helper ui.cpp calls for it.
var genTypeHelper = map[string]string{
	"panel": "panel", "chart": "panel", "gauge": "panel", "statusbar": "statusbar", "button": "button",
	"textarea": "input", "spinbox": "input", "slider": "slider", "switch": "switch", "checkbox": "checkbox",
	"bar": "bar", "arc": "arc", "spinner": "arc", "dropdown": "selector", "roller": "selector",
	"keypad": "items", "table": "items", "list": "items", "line": "line",
}

var genEventCodes = map[string]string{
	"clicked": "LV_EVENT_CLICKED", "long_pressed": "LV_EVENT_LONG_PRESSED",
	"value_changed": "LV_EVENT_VALUE_CHANGED", "ready": "LV_EVENT_READY",
}

// genWidget is one widget in generation order with its generated names.
type genWidget struct {
	w        *Widget
	spec     *WidgetSpec
	scr      *genScreen
	parent   *genWidget
	depth    int    // 1 for widgets placed on the screen
	index    int    // 1-based depth-first position across the project
	key      string // name, or the decimal index for unnamed widgets (never a valid name)
	handle   string // C expression of the widget object
	offX     int    // added to x/y: panels compensate their border (plan 4b note)
	offY     int
	children []*genWidget
}

type genScreen struct {
	s         *Screen
	roots     []*genWidget
	textareas bool // the screen gets one shared keyboard
	theme     bool // a create line reads ui_theme_colors()
}

// genStatusbar describes the parts of one statusbar.
type genStatusbar struct {
	gw                   *genWidget
	wifi, clock, builtin bool
}

// uiGen renders ui.h, ui.cpp and ui_hooks_weak.cpp for a validated project.
type uiGen struct {
	p       Project
	theme   Theme
	images  []genImage
	screens []*genScreen
	all     []*genWidget
	sbs     []genStatusbar
	used    map[string]bool // style helpers ui.cpp calls
	counter int
}

func newUIGen(p Project, th Theme, images []genImage) *uiGen {
	g := &uiGen{p: p, theme: th, images: images, used: map[string]bool{"screen": true}}
	for i := range p.Screens {
		gs := &genScreen{s: &p.Screens[i]}
		g.screens = append(g.screens, gs)
		for j := range gs.s.Widgets {
			gs.roots = append(gs.roots, g.add(gs, nil, &gs.s.Widgets[j]))
		}
	}
	return g
}

func (g *uiGen) add(gs *genScreen, parent *genWidget, w *Widget) *genWidget {
	spec, _ := widgetSpecRef(w.Type)
	g.counter++
	gw := &genWidget{w: w, spec: spec, scr: gs, parent: parent, depth: 1, index: g.counter, key: w.Name}
	if parent != nil {
		gw.depth = parent.depth + 1
		if parent.w.Type == "panel" {
			bw := g.borderWidth(parent.w)
			gw.offX, gw.offY = -bw, -bw
		}
	}
	switch {
	case w.Name != "":
		gw.handle = "ui_" + w.Name
	case gw.depth == 1:
		gw.handle = "obj"
	default:
		gw.handle = "obj" + strconv.Itoa(gw.depth)
	}
	if gw.key == "" {
		gw.key = strconv.Itoa(gw.index)
	}
	if h, ok := genTypeHelper[w.Type]; ok {
		g.used[h] = true
	}
	switch w.Type {
	case "textarea":
		gs.textareas = true
	case "spinbox", "msgbox":
		g.used["button"] = true
	case "statusbar":
		sb := genStatusbar{
			gw:      gw,
			wifi:    genPropBool(w, spec, "show_wifi") && g.p.Settings.WiFi != "off",
			clock:   genPropBool(w, spec, "show_clock") && g.p.Settings.StatusbarClock,
			builtin: g.p.Settings.WiFi != "off" && !hasEvent(w, "long_pressed"),
		}
		g.sbs = append(g.sbs, sb)
	}
	if g.readsTheme(gw) {
		gs.theme = true
	}
	g.all = append(g.all, gw)
	for i := range w.Children {
		gw.children = append(gw.children, g.add(gs, gw, &w.Children[i]))
	}
	return gw
}

// readsTheme reports whether a widget's create lines use ui_theme_colors().
func (g *uiGen) readsTheme(gw *genWidget) bool {
	noAccent := gw.w.Style.AccentColor == ""
	switch gw.w.Type {
	case "tabview", "msgbox", "canvas":
		return true
	case "gauge":
		return noAccent
	case "chart":
		n := len(genPropList(gw.w, gw.spec, "series"))
		return n > 1 || (n == 1 && noAccent)
	case "led":
		return genPropString(gw.w, gw.spec, "color") == "" && noAccent
	}
	return false
}

func hasEvent(w *Widget, on string) bool {
	for _, ev := range w.Events {
		if ev.On == on {
			return true
		}
	}
	return false
}

// borderWidth is the effective border width of a panel (override or theme).
func (g *uiGen) borderWidth(w *Widget) int {
	if w.Style.BorderWidth != nil {
		return *w.Style.BorderWidth
	}
	return g.theme.Tokens.BorderWidth
}

func (g *uiGen) screenByID(id string) *Screen {
	for i := range g.p.Screens {
		if g.p.Screens[i].ID == id {
			return &g.p.Screens[i]
		}
	}
	return nil
}

func screenEnum(s *Screen) string { return "UI_SCREEN_" + strings.ToUpper(s.Name) }

func transitionEnum(t string) string {
	if t == "" {
		t = "none"
	}
	return "UI_TR_" + strings.ToUpper(t)
}

// callback is the function an event binding registers.
func callback(gw *genWidget, ev EventBinding) string {
	if ev.Action.Type == "logic" {
		return "on_" + gw.w.Name + "_" + ev.On
	}
	return "ui__ev_" + gw.w.Name + "_" + ev.On
}

func (g *uiGen) hasType(typ string) bool {
	for _, gw := range g.all {
		if gw.w.Type == typ {
			return true
		}
	}
	return false
}

func (g *uiGen) startScreen() *Screen { return g.screenByID(g.p.StartScreen) }

// renderHeader writes src/generated/ui.h.
func (g *uiGen) renderHeader() string {
	var w codeWriter
	w.raw(genHeader("ui.h"))
	w.line(0, "#pragma once")
	w.line(0, "")
	w.line(0, "#include <lvgl.h>")
	w.line(0, "")
	w.line(0, `#include "cyd.h"`)
	w.line(0, "")
	w.line(0, "enum ui_screen_t {")
	for _, gs := range g.screens {
		w.line(1, "%s,", screenEnum(gs.s))
	}
	w.line(0, "};")
	w.line(0, "")
	w.raw(genTransitionEnum)
	w.line(0, "")
	w.line(0, "// Screens")
	for _, gs := range g.screens {
		w.line(0, "extern lv_obj_t *ui_scr_%s;", gs.s.Name)
	}
	if named := g.named(); len(named) > 0 {
		w.line(0, "")
		w.line(0, "// Named widgets")
		for _, gw := range named {
			if gw.w.Type == "gauge" {
				w.line(0, "extern lv_obj_t *%s;  // gauge: needle = lv_obj_get_child(%s, 0), length %d", gw.handle, gw.handle, gaugeNeedle(gw.w))
				continue
			}
			w.line(0, "extern lv_obj_t *%s;", gw.handle)
		}
	}
	if charts := g.namedOfType("chart"); len(charts) > 0 {
		w.line(0, "")
		w.line(0, "// Chart series (set values with lv_chart_set_next_value / lv_chart_set_series_values)")
		for _, gw := range charts {
			w.line(0, "extern lv_chart_series_t *%s_series[%d];", gw.handle, len(genPropList(gw.w, gw.spec, "series")))
		}
	}
	if len(g.images) > 0 {
		w.line(0, "")
		w.line(0, "// Images")
		for _, im := range g.images {
			w.line(0, "LV_IMAGE_DECLARE(%s);", im.symbol())
		}
	}
	w.line(0, "")
	w.line(0, "// Fonts (src/generated/fonts/*.c, converted by cyd-build from fonts.json)")
	for _, f := range projectFonts(g.p, g.theme) {
		w.line(0, "LV_FONT_DECLARE(%s);", f.symbol())
	}
	w.line(0, "")
	w.line(0, "void ui_init(void);")
	w.line(0, "void ui_goto(ui_screen_t s, ui_transition_t t);")
	w.line(0, "void ui_back(void);")
	w.line(0, "int ui_event_index(lv_event_t *e);")
	w.line(0, "const cyd::ThemeColors &ui_theme_colors(void);")
	if boxes := g.namedOfType("msgbox"); len(boxes) > 0 {
		w.line(0, "")
		w.line(0, "// Message boxes (modal, on the top layer; every button closes the box)")
		for _, gw := range boxes {
			w.line(0, "void ui_show_%s(void);", gw.w.Name)
			w.line(0, "void ui_hide_%s(void);", gw.w.Name)
		}
	}
	if hooks := g.hooks(); len(hooks) > 0 {
		w.line(0, "")
		w.line(0, "// Logic hooks: implement them in src/app/*.cpp (weak defaults in ui_hooks_weak.cpp).")
		for _, h := range hooks {
			if hint := cComment(h.hint); hint != "" {
				w.line(0, "void %s(lv_event_t *e);  // %s", h.name, hint)
			} else {
				w.line(0, "void %s(lv_event_t *e);", h.name)
			}
		}
	}
	return w.String()
}

const genTransitionEnum = `enum ui_transition_t {
  UI_TR_NONE,
  UI_TR_FADE,
  UI_TR_SLIDE_LEFT,
  UI_TR_SLIDE_RIGHT,
  UI_TR_SLIDE_UP,
  UI_TR_SLIDE_DOWN,
};
`

func (g *uiGen) named() []*genWidget {
	var out []*genWidget
	for _, gw := range g.all {
		if gw.w.Name != "" {
			out = append(out, gw)
		}
	}
	return out
}

func (g *uiGen) namedOfType(typ string) []*genWidget {
	var out []*genWidget
	for _, gw := range g.named() {
		if gw.w.Type == typ {
			out = append(out, gw)
		}
	}
	return out
}

type genHook struct{ name, hint string }

// hooks lists every logic hook in traversal and event order.
func (g *uiGen) hooks() []genHook {
	var out []genHook
	for _, gw := range g.all {
		for _, ev := range gw.w.Events {
			if ev.Action.Type == "logic" {
				out = append(out, genHook{callback(gw, ev), ev.Action.Hint})
			}
		}
	}
	return out
}

// renderHooks writes src/generated/ui_hooks_weak.cpp.
func (g *uiGen) renderHooks() string {
	var w codeWriter
	w.raw(genHeader("ui_hooks_weak.cpp"))
	w.line(0, "// Weak defaults: a strong definition in src/app/*.cpp replaces each of them at link time.")
	w.line(0, `#include "ui.h"`)
	w.line(0, "")
	w.line(0, "__attribute__((weak)) void app_setup() {}")
	w.line(0, "")
	w.line(0, "__attribute__((weak)) void app_loop() {}")
	for _, h := range g.hooks() {
		w.line(0, "")
		w.line(0, "__attribute__((weak)) void %s(lv_event_t *e) {", h.name)
		w.line(1, "(void)e;")
		w.line(1, "static bool warned = false;")
		w.line(1, "if (!warned) {")
		w.line(2, "warned = true;")
		w.line(2, `cyd::log::warn("hook not implemented: %s");`, h.name)
		w.line(1, "}")
		w.line(0, "}")
	}
	return w.String()
}

// renderSource writes src/generated/ui.cpp.
func (g *uiGen) renderSource() string {
	var w codeWriter
	w.raw(genHeader("ui.cpp"))
	w.line(0, `#include "ui.h"`)
	w.line(0, "")
	w.line(0, "#include <string.h>")
	if g.hasType("canvas") {
		w.line(0, "")
		w.line(0, "#include <esp_heap_caps.h>")
		w.line(0, "")
		w.line(0, `#include "board_config.h"`)
	}
	w.line(0, "")
	w.line(0, "void ui__theme_init(void);")
	for _, h := range genHelperOrder {
		if g.used[h] {
			w.line(0, "void ui__style_%s(lv_obj_t *obj);", h)
		}
	}
	w.line(0, "")
	for _, gs := range g.screens {
		w.line(0, "lv_obj_t *ui_scr_%s;", gs.s.Name)
	}
	if named := g.named(); len(named) > 0 {
		w.line(0, "")
		for _, gw := range named {
			w.line(0, "lv_obj_t *%s;", gw.handle)
		}
	}
	if charts := g.namedOfType("chart"); len(charts) > 0 {
		w.line(0, "")
		for _, gw := range charts {
			w.line(0, "lv_chart_series_t *%s_series[%d];", gw.handle, len(genPropList(gw.w, gw.spec, "series")))
		}
	}
	var statics []string
	for _, sb := range g.sbs {
		if sb.wifi {
			statics = append(statics, "ui__"+sb.gw.key+"_wifi")
		}
		if sb.clock {
			statics = append(statics, "ui__"+sb.gw.key+"_clock")
		}
	}
	for _, gs := range g.screens {
		if gs.textareas {
			statics = append(statics, "ui__scr_"+gs.s.Name+"_kb")
		}
	}
	if len(statics) > 0 {
		w.line(0, "")
		for _, s := range statics {
			w.line(0, "static lv_obj_t *%s;", s)
		}
	}
	w.line(0, "")
	w.line(0, "static lv_obj_t **const ui__screens[] = {")
	for _, gs := range g.screens {
		w.line(2, "&ui_scr_%s,", gs.s.Name)
	}
	w.line(0, "};")
	w.line(0, "")
	w.raw(strings.Replace(genNavigation, "{{START}}", screenEnum(g.startScreen()), 1))
	g.renderStatusbarTick(&w)
	g.renderSharedCallbacks(&w)
	for _, gw := range g.all {
		for _, ev := range gw.w.Events {
			if ev.Action.Type != "logic" {
				g.renderBuiltinAction(&w, gw, ev)
			}
		}
	}
	for _, gs := range g.screens {
		g.renderScreen(&w, gs)
	}
	for _, gw := range g.namedOfType("msgbox") {
		w.line(0, "")
		w.line(0, "void ui_show_%s(void) {", gw.w.Name)
		w.line(1, "lv_obj_remove_flag(lv_obj_get_parent(%s), LV_OBJ_FLAG_HIDDEN);", gw.handle)
		w.line(1, "lv_obj_move_foreground(lv_obj_get_parent(%s));", gw.handle)
		w.line(0, "}")
		w.line(0, "")
		w.line(0, "void ui_hide_%s(void) {", gw.w.Name)
		w.line(1, "lv_obj_add_flag(lv_obj_get_parent(%s), LV_OBJ_FLAG_HIDDEN);", gw.handle)
		w.line(0, "}")
	}
	start := g.startScreen()
	w.line(0, "")
	w.line(0, "void ui_init(void) {")
	w.line(1, "ui__theme_init();")
	for _, gs := range g.screens {
		w.line(1, "ui__scr_%s_create();", gs.s.Name)
	}
	w.line(1, "ui__current = %s;", screenEnum(start))
	w.line(1, "lv_screen_load(ui_scr_%s);", start.Name)
	if g.hasStatusbarParts() {
		w.line(1, "lv_timer_create(ui__statusbar_tick, 1000, NULL);")
		w.line(1, "ui__statusbar_tick(NULL);")
	}
	w.line(0, "}")
	return w.String()
}

func (g *uiGen) hasStatusbarParts() bool {
	for _, sb := range g.sbs {
		if sb.wifi || sb.clock {
			return true
		}
	}
	return false
}

func (g *uiGen) renderStatusbarTick(w *codeWriter) {
	if !g.hasStatusbarParts() {
		return
	}
	wifi, clock := false, false
	for _, sb := range g.sbs {
		wifi = wifi || sb.wifi
		clock = clock || sb.clock
	}
	w.line(0, "")
	w.line(0, "static void ui__statusbar_tick(lv_timer_t *timer) {")
	w.line(1, "(void)timer;")
	if wifi {
		w.line(1, "const cyd::ThemeColors &t = ui_theme_colors();")
		w.line(1, "lv_color_t wifi = cyd::wifi::connected() ? t.text : t.text_muted;")
	}
	if clock {
		w.line(1, "%s", `String clock = cyd::time::format("%H:%M");`)
	}
	for _, sb := range g.sbs {
		if sb.wifi {
			w.line(1, "lv_obj_set_style_text_color(ui__%s_wifi, wifi, 0);", sb.gw.key)
		}
		if sb.clock {
			w.line(1, "lv_label_set_text(ui__%s_clock, clock.c_str());", sb.gw.key)
		}
	}
	w.line(0, "}")
}

// renderSharedCallbacks writes the generator-private callbacks the project needs.
func (g *uiGen) renderSharedCallbacks(w *codeWriter) {
	for _, sb := range g.sbs {
		if sb.builtin {
			w.raw(genStatusbarOpenSetup)
			break
		}
	}
	for _, gs := range g.screens {
		if gs.textareas {
			w.raw(genTextareaEvent)
			break
		}
	}
	if g.hasType("spinbox") {
		w.raw(genSpinboxStep)
	}
	if g.hasType("msgbox") {
		w.raw(genMsgboxClose)
	}
}

const genStatusbarOpenSetup = `
static void ui__statusbar_open_setup(lv_event_t *e) {
  (void)e;
  cyd::wifi::open_setup();
}
`

// genTextareaEvent shows the screen's shared keyboard while a text area is
// focused (or tapped again) and hides it on defocus, Enter or cancel. The
// user data is the address of the screen's keyboard handle.
const genTextareaEvent = `
static void ui__textarea_event(lv_event_t *e) {
  lv_event_code_t code = lv_event_get_code(e);
  lv_obj_t *ta = lv_event_get_target_obj(e);
  lv_obj_t *kb = *(lv_obj_t **)lv_event_get_user_data(e);
  if (kb == NULL) return;
  if (code == LV_EVENT_FOCUSED || code == LV_EVENT_CLICKED) {
    lv_keyboard_set_mode(kb, lv_textarea_get_accepted_chars(ta) != NULL ? LV_KEYBOARD_MODE_NUMBER : LV_KEYBOARD_MODE_TEXT_LOWER);
    lv_keyboard_set_textarea(kb, ta);
    lv_obj_remove_flag(kb, LV_OBJ_FLAG_HIDDEN);
    lv_obj_move_foreground(kb);
  } else if (code == LV_EVENT_DEFOCUSED || code == LV_EVENT_READY || code == LV_EVENT_CANCEL) {
    lv_keyboard_set_textarea(kb, NULL);
    lv_obj_add_flag(kb, LV_OBJ_FLAG_HIDDEN);
  }
}
`

const genSpinboxStep = `
static void ui__spinbox_dec(lv_event_t *e) {
  lv_event_code_t code = lv_event_get_code(e);
  if (code == LV_EVENT_SHORT_CLICKED || code == LV_EVENT_LONG_PRESSED_REPEAT) {
    lv_spinbox_decrement((lv_obj_t *)lv_event_get_user_data(e));
  }
}

static void ui__spinbox_inc(lv_event_t *e) {
  lv_event_code_t code = lv_event_get_code(e);
  if (code == LV_EVENT_SHORT_CLICKED || code == LV_EVENT_LONG_PRESSED_REPEAT) {
    lv_spinbox_increment((lv_obj_t *)lv_event_get_user_data(e));
  }
}
`

// genMsgboxClose hides the backdrop of the message box whose footer button
// was clicked (button -> footer -> msgbox -> backdrop).
const genMsgboxClose = `
static void ui__msgbox_close(lv_event_t *e) {
  lv_obj_t *box = lv_obj_get_parent(lv_obj_get_parent(lv_event_get_target_obj(e)));
  lv_obj_add_flag(lv_obj_get_parent(box), LV_OBJ_FLAG_HIDDEN);
}
`

func (g *uiGen) renderBuiltinAction(w *codeWriter, gw *genWidget, ev EventBinding) {
	w.line(0, "")
	w.line(0, "static void %s(lv_event_t *e) {", callback(gw, ev))
	w.line(1, "(void)e;")
	switch ev.Action.Type {
	case "goto_screen":
		target := g.screenByID(ev.Action.Screen)
		tr := ev.Action.Transition
		if tr == "" {
			tr = target.Transition
		}
		w.line(1, "ui_goto(%s, %s);", screenEnum(target), transitionEnum(tr))
	case "back":
		w.line(1, "ui_back();")
	case "open_wifi_setup":
		w.line(1, "cyd::wifi::open_setup();")
	}
	w.line(0, "}")
}

func (g *uiGen) renderScreen(w *codeWriter, gs *genScreen) {
	scr := "ui_scr_" + gs.s.Name
	w.line(0, "")
	w.line(0, "static void ui__scr_%s_create(void) {", gs.s.Name)
	if gs.theme {
		w.line(1, "const cyd::ThemeColors &t = ui_theme_colors();")
	}
	w.line(1, "%s = lv_obj_create(NULL);", scr)
	w.line(1, "ui__style_screen(%s);", scr)
	g.renderStyle(w, scr, gs.s.Style, "", 1)
	for _, gw := range gs.roots {
		w.line(0, "")
		g.renderWidget(w, gw, scr, 1)
	}
	if gs.textareas {
		kb := "ui__scr_" + gs.s.Name + "_kb"
		w.line(0, "")
		w.line(1, "%s = lv_keyboard_create(%s);", kb, scr)
		w.line(1, "lv_obj_add_flag(%s, LV_OBJ_FLAG_HIDDEN);", kb)
	}
	w.line(0, "}")
}

// genNavigation is the fixed navigation block of ui.cpp (plan 1b reference).
const genNavigation = `#define UI__HISTORY 16
static ui_screen_t ui__current = {{START}};
static ui_screen_t ui__history[UI__HISTORY];
static ui_transition_t ui__history_tr[UI__HISTORY];
static int ui__history_len = 0;

static lv_screen_load_anim_t ui__anim(ui_transition_t t) {
  switch (t) {
    case UI_TR_FADE:
      return LV_SCREEN_LOAD_ANIM_FADE_IN;
    case UI_TR_SLIDE_LEFT:
      return LV_SCREEN_LOAD_ANIM_MOVE_LEFT;
    case UI_TR_SLIDE_RIGHT:
      return LV_SCREEN_LOAD_ANIM_MOVE_RIGHT;
    case UI_TR_SLIDE_UP:
      return LV_SCREEN_LOAD_ANIM_MOVE_TOP;
    case UI_TR_SLIDE_DOWN:
      return LV_SCREEN_LOAD_ANIM_MOVE_BOTTOM;
    default:
      return LV_SCREEN_LOAD_ANIM_NONE;
  }
}

static ui_transition_t ui__reverse(ui_transition_t t) {
  switch (t) {
    case UI_TR_SLIDE_LEFT:
      return UI_TR_SLIDE_RIGHT;
    case UI_TR_SLIDE_RIGHT:
      return UI_TR_SLIDE_LEFT;
    case UI_TR_SLIDE_UP:
      return UI_TR_SLIDE_DOWN;
    case UI_TR_SLIDE_DOWN:
      return UI_TR_SLIDE_UP;
    default:
      return t;
  }
}

static void ui__load(ui_screen_t s, ui_transition_t t) {
  ui__current = s;
  lv_screen_load_anim(*ui__screens[s], ui__anim(t), t == UI_TR_NONE ? 0 : 250, 0, false);
}

void ui_goto(ui_screen_t s, ui_transition_t t) {
  if (s == ui__current) return;
  if (ui__history_len == UI__HISTORY) {
    memmove(&ui__history[0], &ui__history[1], (UI__HISTORY - 1) * sizeof(ui__history[0]));
    memmove(&ui__history_tr[0], &ui__history_tr[1], (UI__HISTORY - 1) * sizeof(ui__history_tr[0]));
    ui__history_len = UI__HISTORY - 1;
  }
  ui__history[ui__history_len] = ui__current;
  ui__history_tr[ui__history_len] = t;
  ui__history_len++;
  ui__load(s, t);
}

void ui_back(void) {
  if (ui__history_len == 0) return;
  ui__history_len--;
  ui__load(ui__history[ui__history_len], ui__reverse(ui__history_tr[ui__history_len]));
}

int ui_event_index(lv_event_t *e) {
  intptr_t tag = (intptr_t)lv_event_get_user_data(e);
  if (tag > 0) return (int)(tag - 1);
  lv_obj_t *target = lv_event_get_target_obj(e);
  if (target != NULL && lv_obj_check_type(target, &lv_buttonmatrix_class)) {
    uint32_t sel = lv_buttonmatrix_get_selected_button(target);
    return sel == LV_BUTTONMATRIX_BUTTON_NONE ? -1 : (int)sel;
  }
  return -1;
}
`

// gaugeNeedle is the needle length of a gauge (pixels from the centre).
func gaugeNeedle(w *Widget) int {
	return max(min(w.W, w.H)/2-16, 4)
}
