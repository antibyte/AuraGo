package cydcreator

import (
	"slices"
	"strconv"
	"strings"
)

// LVGL constructor per widget type (msgbox is created on a backdrop).
var genCreateFunc = map[string]string{
	"label": "lv_label_create", "button": "lv_button_create", "image": "lv_image_create", "panel": "lv_obj_create",
	"line": "lv_line_create", "switch": "lv_switch_create", "checkbox": "lv_checkbox_create",
	"slider": "lv_slider_create", "arc": "lv_arc_create", "dropdown": "lv_dropdown_create",
	"roller": "lv_roller_create", "textarea": "lv_textarea_create", "spinbox": "lv_spinbox_create",
	"keypad": "lv_buttonmatrix_create", "bar": "lv_bar_create", "spinner": "lv_spinner_create",
	"led": "lv_led_create", "chart": "lv_chart_create", "gauge": "lv_scale_create", "table": "lv_table_create",
	"qrcode": "lv_qrcode_create", "tabview": "lv_tabview_create", "list": "lv_list_create",
	"msgbox": "lv_msgbox_create", "canvas": "lv_canvas_create", "statusbar": "lv_obj_create",
}

var (
	genLongModes    = map[string]string{"wrap": "LV_LABEL_LONG_MODE_WRAP", "dots": "LV_LABEL_LONG_MODE_DOTS", "scroll": "LV_LABEL_LONG_MODE_SCROLL_CIRCULAR", "clip": "LV_LABEL_LONG_MODE_CLIP"}
	genTextAligns   = map[string]string{"left": "LV_TEXT_ALIGN_LEFT", "center": "LV_TEXT_ALIGN_CENTER", "right": "LV_TEXT_ALIGN_RIGHT"}
	genBarPositions = map[string]string{"top": "LV_DIR_TOP", "bottom": "LV_DIR_BOTTOM", "left": "LV_DIR_LEFT", "right": "LV_DIR_RIGHT"}
	// Chart series colors cycle through these theme roles (as the designer preview does).
	genSeriesColors = []string{"t.accent", "t.success", "t.warning", "t.danger"}
	// Keys of the built-in keypad layouts (the designer preview shows the same).
	genKeypadNumeric   = []string{"1", "2", "3", "\n", "4", "5", "6", "\n", "7", "8", "9", "\n", ".", "0", "LV_SYMBOL_BACKSPACE"}
	genKeypadNumericOK = []string{"1", "2", "3", "\n", "4", "5", "6", "\n", "7", "8", "9", "\n", "LV_SYMBOL_BACKSPACE", "0", "OK"}
)

// renderWidget writes one widget block: create, geometry, style helper, type
// lines, style overrides, hidden flag, events, then the children. Unnamed
// widgets live in a block with a local handle.
func (g *uiGen) renderWidget(w *codeWriter, gw *genWidget, parent string, ind int) {
	if gw.w.Name == "" {
		w.line(ind, "{")
		ind++
	}
	g.renderCreate(w, gw, parent, ind)
	g.renderTypeLines(w, gw, parent, ind)
	g.renderStyle(w, gw.handle, gw.w.Style, gw.w.Type, ind)
	if gw.w.Hidden && gw.w.Type != "msgbox" {
		w.line(ind, "lv_obj_add_flag(%s, LV_OBJ_FLAG_HIDDEN);", gw.handle)
	}
	g.renderEvents(w, gw, ind)
	g.renderChildren(w, gw, ind)
	if gw.w.Name == "" {
		w.line(ind-1, "}")
	}
}

// renderCreate writes the constructor and geometry lines.
func (g *uiGen) renderCreate(w *codeWriter, gw *genWidget, parent string, ind int) {
	h, wd := gw.handle, gw.w
	decl := h
	if wd.Name == "" {
		decl = "lv_obj_t *" + h
	}
	x, y := wd.X+gw.offX, wd.Y+gw.offY
	switch wd.Type {
	case "msgbox":
		ax, ay := g.absPos(gw)
		w.line(ind, "%s = lv_msgbox_create(lv_obj_create(lv_layer_top()));", decl)
		w.line(ind, "lv_obj_align(%s, LV_ALIGN_TOP_LEFT, %d, %d);", h, ax, ay)
		w.line(ind, "lv_obj_set_size(%s, %d, %d);", h, wd.W, wd.H)
		return
	case "qrcode":
		size := min(wd.W, wd.H)
		w.line(ind, "%s = lv_qrcode_create(%s);", decl, parent)
		w.line(ind, "lv_qrcode_set_size(%s, %d);", h, size)
		w.line(ind, "lv_obj_set_pos(%s, %d, %d);", h, x+(wd.W-size)/2, y+(wd.H-size)/2)
		return
	case "gauge":
		// lv_scale only draws a line needle on a square scale.
		size := min(wd.W, wd.H)
		w.line(ind, "%s = lv_scale_create(%s);", decl, parent)
		w.line(ind, "lv_obj_set_pos(%s, %d, %d);", h, x+(wd.W-size)/2, y+(wd.H-size)/2)
		w.line(ind, "lv_obj_set_size(%s, %d, %d);", h, size, size)
	case "spinbox":
		w.line(ind, "%s = lv_spinbox_create(%s);", decl, parent)
		w.line(ind, "lv_obj_set_pos(%s, %d, %d);", h, x+wd.H+4, y)
		w.line(ind, "lv_obj_set_size(%s, %d, %d);", h, max(wd.W-2*wd.H-8, 1), wd.H)
	default:
		w.line(ind, "%s = %s(%s);", decl, genCreateFunc[wd.Type], parent)
		w.line(ind, "lv_obj_set_pos(%s, %d, %d);", h, x, y)
		w.line(ind, "lv_obj_set_size(%s, %d, %d);", h, wd.W, wd.H)
	}
	if helper, ok := genTypeHelper[wd.Type]; ok {
		w.line(ind, "ui__style_%s(%s);", helper, h)
	}
}

// absPos is a widget's position relative to the screen, as the designer
// shows it (panel children relative to the panel's outer box, tabview
// children relative to the content area).
func (g *uiGen) absPos(gw *genWidget) (int, int) {
	x, y := gw.w.X, gw.w.Y
	for a := gw.parent; a != nil; a = a.parent {
		x += a.w.X
		y += a.w.Y
		if a.w.Type == "tabview" {
			size := genPropInt(a.w, a.spec, "bar_size")
			switch genPropString(a.w, a.spec, "bar_position") {
			case "top":
				y += size
			case "left":
				x += size
			}
		}
	}
	return x, y
}

// buttonText is the label expression of a button: optional symbol, optional
// text, wrapped in "[ " … " ]" for bracket themes. The symbol becomes the
// macro LV_SYMBOL_<UPPER> unquoted, so only names of the catalog's Symbols
// list are used (Validate already guarantees it; anything else is dropped
// here as well, so no other identifier can reach the C source).
func (g *uiGen) buttonText(symbol, text string) string {
	if !slices.Contains(Symbols, symbol) {
		symbol = ""
	}
	bracket := g.theme.ButtonStyle == "bracket"
	var parts []string
	if bracket {
		parts = append(parts, `"[ "`)
	}
	if symbol != "" {
		parts = append(parts, "LV_SYMBOL_"+strings.ToUpper(symbol))
	}
	switch {
	case symbol != "" && text != "":
		text = " " + text
	case symbol == "" && text == "" && !bracket:
		return `""`
	}
	if bracket {
		text += " ]"
	}
	if text != "" {
		parts = append(parts, cString(text))
	}
	return strings.Join(parts, " ")
}

func (g *uiGen) renderFont(w *codeWriter, gw *genWidget, ind int) {
	if f, ok := widgetFont(gw.w, gw.spec, g.theme); ok {
		w.line(ind, "lv_obj_set_style_text_font(%s, &%s, 0);", gw.handle, f.symbol())
	}
}

// accentColor is the expression for the widget's accent: the override, or
// the theme accent (only valid where ui_theme_colors() is in scope as t).
func accentColor(wd *Widget) string {
	if wd.Style.AccentColor != "" {
		return cHexColor(wd.Style.AccentColor)
	}
	return "t.accent"
}

// optionText joins list items for LVGL option strings ('\n' separated); line
// breaks inside an item would split it, so control characters become spaces.
func optionText(items []string) string {
	clean := make([]string, len(items))
	for i, it := range items {
		clean[i] = strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7F {
				return ' '
			}
			return r
		}, it)
	}
	return strings.Join(clean, "\n")
}

// renderTypeLines writes the lines that turn props into LVGL calls.
func (g *uiGen) renderTypeLines(w *codeWriter, gw *genWidget, parent string, ind int) {
	h, wd, spec := gw.handle, gw.w, gw.spec
	pi := func(name string) int { return genPropInt(wd, spec, name) }
	ps := func(name string) string { return genPropString(wd, spec, name) }
	pb := func(name string) bool { return genPropBool(wd, spec, name) }
	pl := func(name string) []string { return genPropList(wd, spec, name) }
	switch wd.Type {
	case "label":
		w.line(ind, "lv_label_set_text(%s, %s);", h, cString(ps("text")))
		w.line(ind, "lv_label_set_long_mode(%s, %s);", h, genLongModes[ps("long_mode")])
		w.line(ind, "lv_obj_set_style_text_align(%s, %s, 0);", h, genTextAligns[ps("align")])
		g.renderFont(w, gw, ind)
	case "button":
		w.line(ind, "{")
		w.line(ind+1, "lv_obj_t *label = lv_label_create(%s);", h)
		w.line(ind+1, "lv_label_set_text(label, %s);", g.buttonText(ps("symbol"), ps("text")))
		w.line(ind+1, "lv_obj_center(label);")
		w.line(ind, "}")
		g.renderFont(w, gw, ind)
	case "image":
		if asset := ps("asset"); asset != "" {
			w.line(ind, "lv_image_set_src(%s, &%s);", h, imageSymbol(asset))
		}
		w.line(ind, "lv_image_set_inner_align(%s, LV_IMAGE_ALIGN_CENTER);", h)
	case "panel":
		if !pb("scrollable") {
			w.line(ind, "lv_obj_remove_flag(%s, LV_OBJ_FLAG_SCROLLABLE);", h)
		}
	case "line":
		t := pi("thickness")
		x0, y0, x1, y1 := t/2, wd.H/2, wd.W-1-t/2, wd.H/2
		if ps("orientation") == "vertical" {
			x0, y0, x1, y1 = wd.W/2, t/2, wd.W/2, wd.H-1-t/2
		}
		w.line(ind, "lv_obj_set_style_line_width(%s, %d, 0);", h, t)
		w.line(ind, "{")
		w.line(ind+1, "static const lv_point_precise_t points[] = {{%d, %d}, {%d, %d}};", x0, y0, x1, y1)
		w.line(ind+1, "lv_line_set_points(%s, points, 2);", h)
		w.line(ind, "}")
	case "switch":
		if pb("checked") {
			w.line(ind, "lv_obj_add_state(%s, LV_STATE_CHECKED);", h)
		}
	case "checkbox":
		w.line(ind, "lv_checkbox_set_text(%s, %s);", h, cString(ps("text")))
		if pb("checked") {
			w.line(ind, "lv_obj_add_state(%s, LV_STATE_CHECKED);", h)
		}
	case "slider":
		w.line(ind, "lv_slider_set_range(%s, %d, %d);", h, pi("min"), pi("max"))
		w.line(ind, "lv_slider_set_value(%s, %d, LV_ANIM_OFF);", h, pi("value"))
	case "arc":
		w.line(ind, "lv_arc_set_range(%s, %d, %d);", h, pi("min"), pi("max"))
		w.line(ind, "lv_arc_set_value(%s, %d);", h, pi("value"))
	case "bar":
		w.line(ind, "lv_bar_set_range(%s, %d, %d);", h, pi("min"), pi("max"))
		w.line(ind, "lv_bar_set_value(%s, %d, LV_ANIM_OFF);", h, pi("value"))
	case "dropdown":
		w.line(ind, "lv_dropdown_set_options(%s, %s);", h, cString(optionText(pl("options"))))
		w.line(ind, "lv_dropdown_set_selected(%s, %d);", h, pi("selected"))
	case "roller":
		w.line(ind, "lv_roller_set_options(%s, %s, LV_ROLLER_MODE_NORMAL);", h, cString(optionText(pl("options"))))
		w.line(ind, "lv_roller_set_visible_row_count(%s, %d);", h, pi("visible_rows"))
		w.line(ind, "lv_roller_set_selected(%s, %d, LV_ANIM_OFF);", h, pi("selected"))
	case "textarea":
		if pb("one_line") {
			w.line(ind, "lv_textarea_set_one_line(%s, true);", h)
		}
		if pb("password") {
			w.line(ind, "lv_textarea_set_password_mode(%s, true);", h)
			w.line(ind, `lv_textarea_set_password_bullet(%s, "*");`, h) // LVGL's default bullet U+2022 is in no converted font
		}
		w.line(ind, "lv_textarea_set_max_length(%s, %d);", h, pi("max_length"))
		if ps("keyboard") == "number" {
			w.line(ind, `lv_textarea_set_accepted_chars(%s, "0123456789.,+-");`, h)
		}
		if p := ps("placeholder"); p != "" {
			w.line(ind, "lv_textarea_set_placeholder_text(%s, %s);", h, cString(p))
		}
		w.line(ind, "lv_textarea_set_text(%s, %s);", h, cString(ps("text")))
		w.line(ind, "lv_obj_add_event_cb(%s, ui__textarea_event, LV_EVENT_ALL, &ui__scr_%s_kb);", h, gw.scr.s.Name)
	case "spinbox":
		digits, decimals := pi("digits"), pi("decimals")
		sep := 0
		if decimals > 0 {
			sep = digits - decimals
		}
		w.line(ind, "lv_spinbox_set_digit_format(%s, %d, %d);", h, digits, sep)
		w.line(ind, "lv_spinbox_set_range(%s, %d, %d);", h, pi("min"), pi("max"))
		w.line(ind, "lv_spinbox_set_step(%s, %d);", h, pi("step"))
		w.line(ind, "lv_spinbox_set_value(%s, %d);", h, pi("value"))
		w.line(ind, "lv_obj_set_style_text_align(%s, LV_TEXT_ALIGN_CENTER, 0);", h)
		x, y := wd.X+gw.offX, wd.Y+gw.offY
		w.line(ind, "{")
		for _, b := range []struct {
			name, symbol, cb string
			x                int
		}{{"minus", "LV_SYMBOL_MINUS", "ui__spinbox_dec", x}, {"plus", "LV_SYMBOL_PLUS", "ui__spinbox_inc", x + wd.W - wd.H}} {
			w.line(ind+1, "lv_obj_t *%s = lv_button_create(%s);", b.name, parent)
			w.line(ind+1, "lv_obj_set_pos(%s, %d, %d);", b.name, b.x, y)
			w.line(ind+1, "lv_obj_set_size(%s, %d, %d);", b.name, wd.H, wd.H)
			w.line(ind+1, "ui__style_button(%s);", b.name)
			w.line(ind+1, "lv_obj_t *%s_label = lv_label_create(%s);", b.name, b.name)
			w.line(ind+1, "lv_label_set_text(%s_label, %s);", b.name, b.symbol)
			w.line(ind+1, "lv_obj_center(%s_label);", b.name)
			w.line(ind+1, "lv_obj_add_event_cb(%s, %s, LV_EVENT_ALL, %s);", b.name, b.cb, h)
			if wd.Hidden {
				w.line(ind+1, "lv_obj_add_flag(%s, LV_OBJ_FLAG_HIDDEN);", b.name)
			}
		}
		w.line(ind, "}")
	case "keypad":
		w.line(ind, "{")
		w.line(ind+1, "static const char *const map[] = {%s};", strings.Join(keypadMap(ps("layout"), pl("keys")), ", "))
		w.line(ind+1, "lv_buttonmatrix_set_map(%s, map);", h)
		w.line(ind, "}")
	case "led":
		color := accentColor(wd)
		if c := ps("color"); c != "" {
			color = cHexColor(c)
		}
		w.line(ind, "lv_led_set_color(%s, %s);", h, color)
		if pb("on") {
			w.line(ind, "lv_led_set_brightness(%s, %d);", h, pi("brightness"))
		} else {
			w.line(ind, "lv_led_off(%s);", h)
		}
	case "chart":
		kind := "LV_CHART_TYPE_LINE"
		if ps("kind") == "bar" {
			kind = "LV_CHART_TYPE_BAR"
		}
		w.line(ind, "lv_chart_set_type(%s, %s);", h, kind)
		w.line(ind, "lv_chart_set_point_count(%s, %d);", h, pi("points"))
		w.line(ind, "lv_chart_set_axis_range(%s, LV_CHART_AXIS_PRIMARY_Y, %d, %d);", h, pi("min"), pi("max"))
		for i := range pl("series") {
			color := genSeriesColors[i%len(genSeriesColors)]
			if i == 0 {
				color = accentColor(wd)
			}
			if wd.Name != "" {
				w.line(ind, "%s_series[%d] = lv_chart_add_series(%s, %s, LV_CHART_AXIS_PRIMARY_Y);", h, i, h, color)
			} else {
				w.line(ind, "lv_chart_add_series(%s, %s, LV_CHART_AXIS_PRIMARY_Y);", h, color)
			}
		}
	case "gauge":
		w.line(ind, "lv_obj_set_style_radius(%s, LV_RADIUS_CIRCLE, 0);", h)
		w.line(ind, "lv_scale_set_mode(%s, LV_SCALE_MODE_ROUND_INNER);", h)
		w.line(ind, "lv_scale_set_range(%s, %d, %d);", h, pi("min"), pi("max"))
		w.line(ind, "lv_scale_set_total_tick_count(%s, %d);", h, pi("ticks"))
		w.line(ind, "lv_scale_set_major_tick_every(%s, %d);", h, pi("label_every"))
		w.line(ind, "lv_scale_set_label_show(%s, true);", h)
		w.line(ind, "lv_scale_set_angle_range(%s, 270);", h)
		w.line(ind, "lv_scale_set_rotation(%s, 135);", h)
		w.line(ind, "{")
		w.line(ind+1, "lv_obj_t *needle = lv_line_create(%s);", h)
		w.line(ind+1, "lv_obj_set_style_line_width(needle, 3, 0);")
		w.line(ind+1, "lv_obj_set_style_line_rounded(needle, true, 0);")
		w.line(ind+1, "lv_obj_set_style_line_color(needle, %s, 0);", accentColor(wd))
		w.line(ind+1, "lv_scale_set_line_needle_value(%s, needle, %d, %d);", h, gaugeNeedle(wd), pi("value"))
		w.line(ind, "}")
	case "table":
		cols, rows := pi("columns"), pi("rows")
		w.line(ind, "lv_table_set_column_count(%s, %d);", h, cols)
		w.line(ind, "lv_table_set_row_count(%s, %d);", h, rows)
		inner := wd.W - 2*g.inputBorder(wd)
		for c := 0; c < cols; c++ {
			w.line(ind, "lv_table_set_column_width(%s, %d, %d);", h, c, max(inner/cols, 1))
		}
		for i, cell := range pl("cells") {
			if cell != "" {
				w.line(ind, "lv_table_set_cell_value(%s, %d, %d, %s);", h, i/cols, i%cols, cString(cell))
			}
		}
	case "qrcode":
		w.line(ind, "lv_qrcode_set_dark_color(%s, lv_color_black());", h)
		w.line(ind, "lv_qrcode_set_light_color(%s, lv_color_white());", h)
		if text := ps("text"); text != "" {
			w.line(ind, "lv_qrcode_update(%s, %s, %d);", h, cString(text), len(text))
		}
	case "tabview":
		w.line(ind, "lv_tabview_set_tab_bar_position(%s, %s);", h, genBarPositions[ps("bar_position")])
		w.line(ind, "lv_tabview_set_tab_bar_size(%s, %d);", h, pi("bar_size"))
		w.line(ind, "lv_obj_set_style_bg_color(%s, t.bg, 0);", h)
		w.line(ind, "lv_obj_set_style_bg_color(lv_tabview_get_tab_bar(%s), t.surface_alt, 0);", h)
	case "list":
		for _, item := range pl("items") {
			w.line(ind, "lv_list_add_button(%s, NULL, %s);", h, cString(item))
		}
	case "msgbox":
		w.line(ind, "{")
		w.line(ind+1, "lv_obj_t *backdrop = lv_obj_get_parent(%s);", h)
		w.line(ind+1, "lv_obj_remove_style_all(backdrop);")
		w.line(ind+1, "lv_obj_set_size(backdrop, LV_PCT(100), LV_PCT(100));")
		w.line(ind+1, "lv_obj_set_style_bg_color(backdrop, lv_color_black(), 0);")
		w.line(ind+1, "lv_obj_set_style_bg_opa(backdrop, LV_OPA_50, 0);")
		w.line(ind+1, "lv_obj_add_flag(backdrop, LV_OBJ_FLAG_HIDDEN);")
		w.line(ind, "}")
		w.line(ind, "lv_obj_set_style_bg_color(%s, t.surface, 0);", h)
		w.line(ind, "lv_obj_set_style_border_color(%s, t.border, 0);", h)
		w.line(ind, "lv_obj_set_style_border_width(%s, %d, 0);", h, max(g.theme.Tokens.BorderWidth, 1))
		w.line(ind, "lv_obj_set_style_radius(%s, t.radius, 0);", h)
		w.line(ind, "lv_obj_set_style_text_color(%s, t.text, 0);", h)
		w.line(ind, "lv_obj_set_style_text_font(%s, t.font_normal, 0);", h)
		if title := ps("title"); title != "" {
			w.line(ind, "lv_msgbox_add_title(%s, %s);", h, cString(title))
			w.line(ind, "lv_obj_set_style_bg_color(lv_msgbox_get_header(%s), t.surface_alt, 0);", h)
		}
		if text := ps("text"); text != "" {
			w.line(ind, "lv_msgbox_add_text(%s, %s);", h, cString(text))
		}
		buttons := pl("buttons")
		for _, b := range buttons {
			w.line(ind, "ui__style_button(lv_msgbox_add_footer_button(%s, %s));", h, g.buttonText("", b))
		}
		if len(buttons) > 0 {
			w.line(ind, "for (int32_t i = 0; i < (int32_t)lv_obj_get_child_count(lv_msgbox_get_footer(%s)); i++) {", h)
			w.line(ind+1, "lv_obj_add_event_cb(lv_obj_get_child(lv_msgbox_get_footer(%s), i), ui__msgbox_close, LV_EVENT_CLICKED, NULL);", h)
			w.line(ind, "}")
		}
	case "canvas":
		w.line(ind, "{")
		w.line(ind+1, "size_t bytes = LV_CANVAS_BUF_SIZE(%d, %d, 16, LV_DRAW_BUF_STRIDE_ALIGN);", wd.W, wd.H)
		w.line(0, "#if CYD_HAS_PSRAM")
		w.line(ind+1, "void *buf = heap_caps_malloc(bytes, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT);")
		w.line(0, "#else")
		w.line(ind+1, "void *buf = heap_caps_malloc(bytes, MALLOC_CAP_8BIT);")
		w.line(0, "#endif")
		w.line(ind+1, "if (buf != NULL) {")
		w.line(ind+2, "lv_canvas_set_buffer(%s, buf, %d, %d, LV_COLOR_FORMAT_RGB565);", h, wd.W, wd.H)
		w.line(ind+2, "lv_canvas_fill_bg(%s, t.surface_alt, LV_OPA_COVER);", h)
		w.line(ind+1, "} else {")
		w.line(ind+2, `cyd::log::error("canvas %s: no memory for %%u bytes", (unsigned)bytes);`, gw.key)
		w.line(ind+1, "}")
		w.line(ind, "}")
	case "statusbar":
		g.renderStatusbarParts(w, gw, ind)
	}
}

// inputBorder is the border width of widgets styled by ui__style_input/items.
func (g *uiGen) inputBorder(wd *Widget) int {
	if wd.Style.BorderWidth != nil {
		return *wd.Style.BorderWidth
	}
	return max(g.theme.Tokens.BorderWidth, 1)
}

// keypadMap builds the button matrix map: "\n" items break rows, a custom
// layout without breaks and more than four keys gets rows of four (designer
// preview rule), empty keys become a space (an empty string ends the map).
func keypadMap(layout string, keys []string) []string {
	var src []string
	switch layout {
	case "numeric":
		src = genKeypadNumeric
	case "numeric_ok":
		src = genKeypadNumericOK
	default:
		breaks := false
		for _, k := range keys {
			if k == "\n" {
				breaks = true
			}
		}
		for i, k := range keys {
			if !breaks && len(keys) > 4 && i > 0 && i%4 == 0 {
				src = append(src, "\n")
			}
			if k == "" {
				k = " "
			}
			src = append(src, k)
		}
	}
	out := make([]string, 0, len(src)+1)
	for _, k := range src {
		// Only the built-in layouts carry symbol macros; every other key
		// (custom, or an unknown layout) is user text and always quoted.
		if strings.HasPrefix(k, "LV_SYMBOL_") && (layout == "numeric" || layout == "numeric_ok") {
			out = append(out, k)
			continue
		}
		out = append(out, cString(k))
	}
	return append(out, `""`)
}

func (g *uiGen) statusbarInfo(gw *genWidget) genStatusbar {
	for _, sb := range g.sbs {
		if sb.gw == gw {
			return sb
		}
	}
	return genStatusbar{}
}

// renderStatusbarParts writes the title, clock and Wi-Fi labels.
func (g *uiGen) renderStatusbarParts(w *codeWriter, gw *genWidget, ind int) {
	sb := g.statusbarInfo(gw)
	title := genPropString(gw.w, gw.spec, "title")
	if title == "" {
		title = gw.scr.s.Title
	}
	if title != "" {
		w.line(ind, "{")
		w.line(ind+1, "lv_obj_t *title = lv_label_create(%s);", gw.handle)
		w.line(ind+1, "lv_label_set_text(title, %s);", cString(title))
		w.line(ind+1, "lv_obj_align(title, LV_ALIGN_LEFT_MID, 6, 0);")
		w.line(ind, "}")
	}
	if sb.clock {
		c := "ui__" + gw.key + "_clock"
		w.line(ind, "%s = lv_label_create(%s);", c, gw.handle)
		w.line(ind, `lv_label_set_text(%s, "");`, c)
		w.line(ind, "lv_obj_align(%s, LV_ALIGN_RIGHT_MID, -6, 0);", c)
	}
	if sb.wifi {
		x := -6
		if sb.clock {
			x = -52
		}
		c := "ui__" + gw.key + "_wifi"
		w.line(ind, "%s = lv_label_create(%s);", c, gw.handle)
		w.line(ind, "lv_label_set_text(%s, LV_SYMBOL_WIFI);", c)
		w.line(ind, "lv_obj_align(%s, LV_ALIGN_RIGHT_MID, %d, 0);", c, x)
	}
}

// renderStyle writes the per-widget (or per-screen) style overrides in Style
// field order. A background color without an opacity is drawn opaque.
func (g *uiGen) renderStyle(w *codeWriter, h string, st Style, typ string, ind int) {
	if st.BgColor != "" {
		w.line(ind, "lv_obj_set_style_bg_color(%s, %s, 0);", h, cHexColor(st.BgColor))
	}
	switch {
	case st.BgOpacity != nil:
		w.line(ind, "lv_obj_set_style_bg_opa(%s, %d, 0);", h, *st.BgOpacity)
	case st.BgColor != "":
		w.line(ind, "lv_obj_set_style_bg_opa(%s, LV_OPA_COVER, 0);", h)
	}
	if st.TextColor != "" {
		w.line(ind, "lv_obj_set_style_text_color(%s, %s, 0);", h, cHexColor(st.TextColor))
	}
	if st.AccentColor != "" {
		c := cHexColor(st.AccentColor)
		switch typ {
		case "arc", "spinner":
			w.line(ind, "lv_obj_set_style_arc_color(%s, %s, LV_PART_INDICATOR);", h, c)
		case "slider", "bar":
			w.line(ind, "lv_obj_set_style_bg_color(%s, %s, LV_PART_INDICATOR);", h, c)
		case "switch", "checkbox":
			w.line(ind, "lv_obj_set_style_bg_color(%s, %s, LV_PART_INDICATOR | LV_STATE_CHECKED);", h, c)
		case "roller":
			w.line(ind, "lv_obj_set_style_bg_color(%s, %s, LV_PART_SELECTED);", h, c)
		case "line":
			w.line(ind, "lv_obj_set_style_line_color(%s, %s, 0);", h, c)
		}
	}
	if st.BorderColor != "" {
		w.line(ind, "lv_obj_set_style_border_color(%s, %s, 0);", h, cHexColor(st.BorderColor))
	}
	if st.BorderWidth != nil {
		w.line(ind, "lv_obj_set_style_border_width(%s, %d, 0);", h, *st.BorderWidth)
	}
	if st.Radius != nil {
		w.line(ind, "lv_obj_set_style_radius(%s, %d, 0);", h, *st.Radius)
	}
}

// renderEvents registers the built-in statusbar long press and every event
// binding (list items and message box buttons carry their index + 1).
func (g *uiGen) renderEvents(w *codeWriter, gw *genWidget, ind int) {
	h, wd := gw.handle, gw.w
	if wd.Type == "statusbar" && g.statusbarInfo(gw).builtin {
		w.line(ind, "lv_obj_add_flag(%s, LV_OBJ_FLAG_CLICKABLE);", h)
		w.line(ind, "lv_obj_add_event_cb(%s, ui__statusbar_open_setup, LV_EVENT_LONG_PRESSED, NULL);", h)
	}
	if len(wd.Events) == 0 || !g.registersEvents(gw) {
		return
	}
	switch wd.Type {
	case "label", "image", "statusbar":
		w.line(ind, "lv_obj_add_flag(%s, LV_OBJ_FLAG_CLICKABLE);", h)
	}
	for _, ev := range wd.Events {
		cb, code := callback(gw, ev), genEventCodes[ev.On]
		switch wd.Type {
		case "list":
			w.line(ind, "for (int32_t i = 0; i < (int32_t)lv_obj_get_child_count(%s); i++) {", h)
			w.line(ind+1, "lv_obj_add_event_cb(lv_obj_get_child(%s, i), %s, %s, (void *)(intptr_t)(i + 1));", h, cb, code)
			w.line(ind, "}")
		case "msgbox":
			w.line(ind, "for (int32_t i = 0; i < (int32_t)lv_obj_get_child_count(lv_msgbox_get_footer(%s)); i++) {", h)
			w.line(ind+1, "lv_obj_add_event_cb(lv_obj_get_child(lv_msgbox_get_footer(%s), i), %s, %s, (void *)(intptr_t)(i + 1));", h, cb, code)
			w.line(ind, "}")
		default:
			w.line(ind, "lv_obj_add_event_cb(%s, %s, %s, NULL);", h, cb, code)
		}
	}
}

// renderChildren writes the children of containers: panel children use the
// panel as parent, tabview children the page of their tab.
func (g *uiGen) renderChildren(w *codeWriter, gw *genWidget, ind int) {
	if gw.w.Type == "tabview" {
		tabs := genPropList(gw.w, gw.spec, "tabs")
		w.line(ind, "{")
		pages := make([]string, len(tabs))
		for i, tab := range tabs {
			pages[i] = "page" + strconv.Itoa(gw.depth) + "_" + strconv.Itoa(i)
			w.line(ind+1, "lv_obj_t *%s = lv_tabview_add_tab(%s, %s);", pages[i], gw.handle, cString(tab))
			w.line(ind+1, "lv_obj_set_style_pad_all(%s, 0, 0);", pages[i])
		}
		for _, c := range gw.children {
			g.renderWidget(w, c, pages[c.w.Tab], ind+1)
		}
		w.line(ind, "}")
		return
	}
	for _, c := range gw.children {
		g.renderWidget(w, c, gw.handle, ind)
	}
}
