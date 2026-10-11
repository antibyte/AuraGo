package cydcreator

import (
	"strconv"
	"strings"
)

// renderTheme writes src/generated/ui_theme.cpp: fixed text (plan 1b
// reference) with the theme's tokens, button style, dark flag and font roles
// substituted. Every style helper is always defined (non-static), so ui.cpp
// may call any of them without unused-function warnings.
func renderTheme(th Theme) string {
	roles := themeFontRoles(th)
	tk := th.Tokens
	style := map[string]string{"filled": "0", "outline": "1", "bracket": "2"}[th.ButtonStyle]
	r := strings.NewReplacer(
		"{{ID}}", th.ID,
		"{{BUTTON_STYLE}}", style,
		"{{DARK}}", strconv.FormatBool(colorLuma(tk.Bg) < 128),
		"{{BG}}", cHexColor(tk.Bg),
		"{{SURFACE}}", cHexColor(tk.Surface),
		"{{SURFACE_ALT}}", cHexColor(tk.SurfaceAlt),
		"{{TEXT}}", cHexColor(tk.Text),
		"{{TEXT_MUTED}}", cHexColor(tk.TextMuted),
		"{{ACCENT}}", cHexColor(tk.Accent),
		"{{ON_ACCENT}}", cHexColor(tk.OnAccent),
		"{{SUCCESS}}", cHexColor(tk.Success),
		"{{WARNING}}", cHexColor(tk.Warning),
		"{{DANGER}}", cHexColor(tk.Danger),
		"{{BORDER}}", cHexColor(tk.Border),
		"{{FOCUS}}", cHexColor(tk.Focus),
		"{{BORDER_WIDTH}}", strconv.Itoa(tk.BorderWidth),
		"{{RADIUS}}", strconv.Itoa(tk.Radius),
		"{{FONT_SMALL}}", roles.Small.symbol(),
		"{{FONT_NORMAL}}", roles.Normal.symbol(),
		"{{FONT_LARGE}}", roles.Large.symbol(),
		"{{FONT_MONO}}", roles.Mono.symbol(),
	)
	return genHeader("ui_theme.cpp") + r.Replace(themeTemplate)
}

// colorLuma is the perceived brightness (0..255) of "#RRGGBB":
// (299 R + 587 G + 114 B) / 1000.
func colorLuma(color string) int {
	v, err := strconv.ParseUint(strings.TrimPrefix(color, "#"), 16, 32)
	if err != nil {
		return 0
	}
	r, g, b := int(v>>16&0xFF), int(v>>8&0xFF), int(v&0xFF)
	return (299*r + 587*g + 114*b) / 1000
}

// themeTemplate is ui_theme.cpp without its header line.
const themeTemplate = `// Theme "{{ID}}".
#include "ui.h"

static const int ui__button_style = {{BUTTON_STYLE}};  // 0 filled, 1 outline, 2 bracket
static const bool ui__dark = {{DARK}};

static cyd::ThemeColors ui__colors;
static bool ui__colors_ready = false;

const cyd::ThemeColors &ui_theme_colors(void) {
  if (!ui__colors_ready) {
    ui__colors.bg = {{BG}};
    ui__colors.surface = {{SURFACE}};
    ui__colors.surface_alt = {{SURFACE_ALT}};
    ui__colors.text = {{TEXT}};
    ui__colors.text_muted = {{TEXT_MUTED}};
    ui__colors.accent = {{ACCENT}};
    ui__colors.on_accent = {{ON_ACCENT}};
    ui__colors.success = {{SUCCESS}};
    ui__colors.warning = {{WARNING}};
    ui__colors.danger = {{DANGER}};
    ui__colors.border = {{BORDER}};
    ui__colors.focus = {{FOCUS}};
    ui__colors.border_width = {{BORDER_WIDTH}};
    ui__colors.radius = {{RADIUS}};
    ui__colors.font_small = &{{FONT_SMALL}};
    ui__colors.font_normal = &{{FONT_NORMAL}};
    ui__colors.font_large = &{{FONT_LARGE}};
    ui__colors.font_mono = &{{FONT_MONO}};
    ui__colors.bracket_buttons = ui__button_style == 2;
    ui__colors_ready = true;
  }
  return ui__colors;
}

static lv_style_t ui__st_screen;
static lv_style_t ui__st_panel;
static lv_style_t ui__st_statusbar;
static lv_style_t ui__st_button;
static lv_style_t ui__st_button_pressed;
static lv_style_t ui__st_focus;
static lv_style_t ui__st_input;
static lv_style_t ui__st_track;
static lv_style_t ui__st_indicator;
static lv_style_t ui__st_knob;
static lv_style_t ui__st_arc_track;
static lv_style_t ui__st_arc_indicator;
static lv_style_t ui__st_selected;
static lv_style_t ui__st_item;
static lv_style_t ui__st_line;
static lv_style_t ui__st_muted;

void ui__theme_init(void) {
  const cyd::ThemeColors &t = ui_theme_colors();
  lv_display_t *disp = lv_display_get_default();
  lv_display_set_theme(disp, lv_theme_default_init(disp, t.accent, t.focus, ui__dark, t.font_normal));

  lv_style_init(&ui__st_screen);
  lv_style_set_bg_color(&ui__st_screen, t.bg);
  lv_style_set_bg_opa(&ui__st_screen, LV_OPA_COVER);
  lv_style_set_text_color(&ui__st_screen, t.text);
  lv_style_set_text_font(&ui__st_screen, t.font_normal);
  lv_style_set_pad_all(&ui__st_screen, 0);

  lv_style_init(&ui__st_panel);
  lv_style_set_bg_color(&ui__st_panel, t.surface);
  lv_style_set_bg_opa(&ui__st_panel, LV_OPA_COVER);
  lv_style_set_border_color(&ui__st_panel, t.border);
  lv_style_set_border_width(&ui__st_panel, t.border_width);
  lv_style_set_radius(&ui__st_panel, t.radius);
  lv_style_set_pad_all(&ui__st_panel, 0);
  lv_style_set_shadow_width(&ui__st_panel, 0);
  lv_style_set_text_color(&ui__st_panel, t.text);

  lv_style_init(&ui__st_statusbar);
  lv_style_set_bg_color(&ui__st_statusbar, t.surface);
  lv_style_set_bg_opa(&ui__st_statusbar, LV_OPA_COVER);
  lv_style_set_border_color(&ui__st_statusbar, t.border);
  lv_style_set_border_width(&ui__st_statusbar, t.border_width);
  lv_style_set_border_side(&ui__st_statusbar, LV_BORDER_SIDE_BOTTOM);
  lv_style_set_radius(&ui__st_statusbar, 0);
  lv_style_set_pad_all(&ui__st_statusbar, 0);
  lv_style_set_text_color(&ui__st_statusbar, t.text);
  lv_style_set_text_font(&ui__st_statusbar, t.font_small);

  lv_style_init(&ui__st_button);
  lv_style_set_radius(&ui__st_button, t.radius);
  lv_style_set_shadow_width(&ui__st_button, 0);
  lv_style_set_text_font(&ui__st_button, t.font_normal);
  if (ui__button_style == 0) {
    lv_style_set_bg_color(&ui__st_button, t.accent);
    lv_style_set_bg_opa(&ui__st_button, LV_OPA_COVER);
    lv_style_set_text_color(&ui__st_button, t.on_accent);
    lv_style_set_border_width(&ui__st_button, 0);
  } else {
    lv_style_set_bg_opa(&ui__st_button, LV_OPA_TRANSP);
    lv_style_set_text_color(&ui__st_button, t.accent);
    lv_style_set_border_color(&ui__st_button, t.accent);
    lv_style_set_border_width(&ui__st_button, ui__button_style == 1 ? (t.border_width > 0 ? t.border_width : 1) : 0);
  }

  lv_style_init(&ui__st_button_pressed);
  lv_style_set_bg_color(&ui__st_button_pressed, ui__button_style == 0 ? lv_color_darken(t.accent, 40) : t.accent);
  lv_style_set_bg_opa(&ui__st_button_pressed, LV_OPA_COVER);
  lv_style_set_text_color(&ui__st_button_pressed, t.on_accent);

  lv_style_init(&ui__st_focus);
  lv_style_set_outline_color(&ui__st_focus, t.focus);
  lv_style_set_outline_width(&ui__st_focus, 2);
  lv_style_set_outline_pad(&ui__st_focus, 2);

  lv_style_init(&ui__st_input);
  lv_style_set_bg_color(&ui__st_input, t.surface);
  lv_style_set_bg_opa(&ui__st_input, LV_OPA_COVER);
  lv_style_set_border_color(&ui__st_input, t.border);
  lv_style_set_border_width(&ui__st_input, t.border_width > 0 ? t.border_width : 1);
  lv_style_set_radius(&ui__st_input, t.radius);
  lv_style_set_text_color(&ui__st_input, t.text);

  lv_style_init(&ui__st_track);
  lv_style_set_bg_color(&ui__st_track, t.surface_alt);
  lv_style_set_bg_opa(&ui__st_track, LV_OPA_COVER);
  lv_style_set_radius(&ui__st_track, LV_RADIUS_CIRCLE);

  lv_style_init(&ui__st_indicator);
  lv_style_set_bg_color(&ui__st_indicator, t.accent);
  lv_style_set_bg_opa(&ui__st_indicator, LV_OPA_COVER);

  lv_style_init(&ui__st_knob);
  lv_style_set_bg_color(&ui__st_knob, t.text);
  lv_style_set_bg_opa(&ui__st_knob, LV_OPA_COVER);
  lv_style_set_border_color(&ui__st_knob, t.accent);
  lv_style_set_border_width(&ui__st_knob, 2);

  lv_style_init(&ui__st_arc_track);
  lv_style_set_arc_color(&ui__st_arc_track, t.surface_alt);

  lv_style_init(&ui__st_arc_indicator);
  lv_style_set_arc_color(&ui__st_arc_indicator, t.accent);

  lv_style_init(&ui__st_selected);
  lv_style_set_bg_color(&ui__st_selected, t.accent);
  lv_style_set_bg_opa(&ui__st_selected, LV_OPA_COVER);
  lv_style_set_text_color(&ui__st_selected, t.on_accent);

  lv_style_init(&ui__st_item);
  lv_style_set_bg_color(&ui__st_item, t.surface_alt);
  lv_style_set_bg_opa(&ui__st_item, LV_OPA_COVER);
  lv_style_set_text_color(&ui__st_item, t.text);
  lv_style_set_border_color(&ui__st_item, t.border);
  lv_style_set_radius(&ui__st_item, t.radius);

  lv_style_init(&ui__st_line);
  lv_style_set_line_color(&ui__st_line, t.border);
  lv_style_set_line_rounded(&ui__st_line, true);

  lv_style_init(&ui__st_muted);
  lv_style_set_text_color(&ui__st_muted, t.text_muted);
}

// One helper per widget class; ui.cpp calls the helper of every widget it creates.

void ui__style_screen(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_screen, 0);
  lv_obj_remove_flag(obj, LV_OBJ_FLAG_SCROLLABLE);
}

void ui__style_panel(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_panel, 0);
}

void ui__style_statusbar(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_statusbar, 0);
  lv_obj_remove_flag(obj, LV_OBJ_FLAG_SCROLLABLE);
}

void ui__style_button(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_button, 0);
  lv_obj_add_style(obj, &ui__st_button_pressed, LV_STATE_PRESSED);
  lv_obj_add_style(obj, &ui__st_focus, LV_STATE_FOCUS_KEY);
}

void ui__style_input(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_input, 0);
  lv_obj_add_style(obj, &ui__st_focus, LV_STATE_FOCUSED);
}

void ui__style_slider(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_track, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_indicator, LV_PART_INDICATOR);
  lv_obj_add_style(obj, &ui__st_knob, LV_PART_KNOB);
}

void ui__style_switch(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_track, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_indicator, LV_PART_INDICATOR | LV_STATE_CHECKED);
  lv_obj_add_style(obj, &ui__st_knob, LV_PART_KNOB);
}

void ui__style_checkbox(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_input, LV_PART_INDICATOR);
  lv_obj_add_style(obj, &ui__st_indicator, LV_PART_INDICATOR | LV_STATE_CHECKED);
}

void ui__style_bar(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_track, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_indicator, LV_PART_INDICATOR);
}

void ui__style_arc(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_arc_track, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_arc_indicator, LV_PART_INDICATOR);
  lv_obj_add_style(obj, &ui__st_knob, LV_PART_KNOB);
}

void ui__style_selector(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_input, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_selected, LV_PART_SELECTED);
}

void ui__style_items(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_input, LV_PART_MAIN);
  lv_obj_add_style(obj, &ui__st_item, LV_PART_ITEMS);
  lv_obj_add_style(obj, &ui__st_selected, LV_PART_ITEMS | LV_STATE_PRESSED);
}

void ui__style_line(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_line, 0);
}

void ui__style_muted(lv_obj_t *obj) {
  lv_obj_add_style(obj, &ui__st_muted, 0);
}
`
