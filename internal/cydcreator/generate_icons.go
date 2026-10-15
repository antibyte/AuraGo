package cydcreator

import "strings"

func (g *uiGen) renderIcon(w *codeWriter, gw *genWidget, handle, asset string, ind int) {
	for _, a := range g.p.Assets {
		if a.ID == asset && a.Encoding == "a8" {
			color := "lv_obj_get_style_text_color(" + handle + ", LV_PART_MAIN)"
			if gw.w.Style.TextColor != "" {
				color = cHexColor(gw.w.Style.TextColor)
			}
			if tint := genPropString(gw.w, gw.spec, "icon_tint"); tint != "" {
				color = cHexColor(tint)
			}
			w.line(ind, "lv_obj_set_style_image_recolor(%s, %s, 0);", handle, color)
			w.line(ind, "lv_obj_set_style_image_recolor_opa(%s, LV_OPA_COVER, 0);", handle)
			break
		}
	}
	if !genPropBool(gw.w, gw.spec, "icon_animate") {
		return
	}
	modes := map[string]int{"pulse": 0, "bob": 1, "rotate": 2, "frames": 3}
	mode := modes[genPropString(gw.w, gw.spec, "icon_motion")]
	frames := genPropList(gw.w, gw.spec, "icon_frames")
	w.line(ind, "{")
	var symbols []string
	if mode == 3 {
		for _, id := range frames {
			symbols = append(symbols, "&"+imageSymbol(id))
		}
	}
	if len(symbols) == 0 {
		symbols = append(symbols, "&"+imageSymbol(asset))
	}
	w.line(ind+1, "static const lv_image_dsc_t *const frames[] = {%s};", strings.Join(symbols, ", "))
	w.line(ind+1, "ui__icon_start(%s, frames, %d, %d);", handle, len(symbols), mode)
	w.line(ind, "}")
}

func (g *uiGen) renderIconHelpers(w *codeWriter) {
	for _, gw := range g.all {
		if (gw.w.Type == "image" || gw.w.Type == "button") && genPropBool(gw.w, gw.spec, "icon_animate") {
			w.raw(genIconHelpers)
			return
		}
	}
}

const genIconHelpers = `
struct ui__icon_state {
    lv_obj_t *obj;
    lv_timer_t *timer;
    const lv_image_dsc_t *const *frames;
    uint32_t start;
    uint8_t count, mode, frame;
    bool visible;
};

static void ui__icon_tick(lv_timer_t *timer) {
    auto *s = static_cast<ui__icon_state *>(lv_timer_get_user_data(timer));
    if(lv_obj_get_screen(s->obj) != lv_screen_active() || !lv_obj_is_visible(s->obj)) {
        s->visible = false;
        return;
    }
    if(!s->visible) { s->visible = true; s->start = lv_tick_get(); }
    uint32_t elapsed = lv_tick_elaps(s->start);
    if(s->mode == 3) {
        uint8_t frame = (elapsed % 1000) * s->count / 1000;
        if(frame != s->frame) { lv_image_set_src(s->obj, s->frames[frame]); s->frame = frame; }
    } else {
        int phase = elapsed % 1600;
        int wave = phase < 800 ? phase : 1600 - phase;
        if(s->mode == 0) lv_obj_set_style_image_opa(s->obj, 205 + wave * 50 / 800, 0);
        if(s->mode == 1) lv_image_set_offset_y(s->obj, wave * 2 / 800);
        if(s->mode == 2) lv_image_set_rotation(s->obj, elapsed % 2400 * 3600 / 2400);
    }
}

static void ui__icon_delete(lv_event_t *event) {
    auto *s = static_cast<ui__icon_state *>(lv_event_get_user_data(event));
    lv_timer_delete(s->timer);
    lv_free(s);
}

static void ui__icon_start(lv_obj_t *obj, const lv_image_dsc_t *const *frames, uint8_t count, uint8_t mode) {
    auto *s = static_cast<ui__icon_state *>(lv_malloc(sizeof(ui__icon_state)));
    if(!s) return;
    *s = {obj, nullptr, frames, 0, count, mode, 0, false};
    s->timer = lv_timer_create(ui__icon_tick, 84, s);
    if(!s->timer) { lv_free(s); return; }
    lv_obj_add_event_cb(obj, ui__icon_delete, LV_EVENT_DELETE, s);
}

`
