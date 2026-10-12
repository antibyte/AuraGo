// cyd_screens.cpp - boot splash and small themed widget helpers shared by the
// runtime's own screens (Wi-Fi setup, calibration, OTA overlay).
#include "cyd_private.h"

namespace cyd {
namespace {

lv_obj_t* g_splash = nullptr;
lv_obj_t* g_splash_status = nullptr;

}  // namespace

namespace detail {

void pump_ui(uint32_t ms) {
  uint32_t start = millis();
  do {
    display_loop();
    delay(5);
  } while (millis() - start < ms);
}

lv_obj_t* make_screen() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* scr = lv_obj_create(nullptr);
  lv_obj_set_style_bg_color(scr, t.bg, 0);
  lv_obj_set_style_bg_opa(scr, LV_OPA_COVER, 0);
  lv_obj_set_style_text_color(scr, t.text, 0);
  lv_obj_set_style_text_font(scr, t.font_normal, 0);
  lv_obj_remove_flag(scr, LV_OBJ_FLAG_SCROLLABLE);
  return scr;
}

lv_obj_t* make_label(lv_obj_t* parent, const char* text, const lv_font_t* font, lv_color_t color) {
  lv_obj_t* l = lv_label_create(parent);
  lv_label_set_text(l, text);
  lv_obj_set_style_text_font(l, font, 0);
  lv_obj_set_style_text_color(l, color, 0);
  return l;
}

lv_obj_t* make_button(lv_obj_t* parent, const char* text, lv_event_cb_t cb, void* user) {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* b = lv_button_create(parent);
  lv_obj_set_style_radius(b, t.radius, 0);
  lv_obj_set_style_shadow_width(b, 0, 0);
  if (t.bracket_buttons) {
    lv_obj_set_style_bg_opa(b, LV_OPA_TRANSP, 0);
    lv_obj_set_style_border_width(b, t.border_width ? t.border_width : 1, 0);
    lv_obj_set_style_border_color(b, t.accent, 0);
  } else {
    lv_obj_set_style_bg_color(b, t.accent, 0);
    lv_obj_set_style_bg_opa(b, LV_OPA_COVER, 0);
  }
  lv_obj_set_style_bg_color(b, t.focus, LV_STATE_PRESSED);
  lv_obj_set_style_bg_opa(b, LV_OPA_COVER, LV_STATE_PRESSED);
  lv_obj_t* l = lv_label_create(b);
  lv_label_set_text(l, text);
  lv_obj_set_style_text_font(l, t.font_normal, 0);
  lv_obj_set_style_text_color(l, t.bracket_buttons ? t.accent : t.on_accent, 0);
  lv_obj_center(l);
  if (cb != nullptr) lv_obj_add_event_cb(b, cb, LV_EVENT_CLICKED, user);
  return b;
}

void splash_show() {
  const ThemeColors& t = ui_theme_colors();
  g_splash = make_screen();
  lv_obj_t* title = make_label(g_splash, CYD_PROJECT_NAME, t.font_large, t.text);
  lv_obj_set_width(title, CYD_SCREEN_W - 24);
  lv_obj_set_style_text_align(title, LV_TEXT_ALIGN_CENTER, 0);
  lv_label_set_long_mode(title, LV_LABEL_LONG_MODE_DOTS);
  lv_obj_align(title, LV_ALIGN_CENTER, 0, -36);

  lv_obj_t* spinner = lv_spinner_create(g_splash);
  lv_obj_set_size(spinner, 36, 36);
  lv_obj_set_style_arc_color(spinner, t.accent, LV_PART_INDICATOR);
  lv_obj_set_style_arc_color(spinner, t.surface_alt, LV_PART_MAIN);
  lv_obj_set_style_arc_width(spinner, 4, LV_PART_INDICATOR);
  lv_obj_set_style_arc_width(spinner, 4, LV_PART_MAIN);
  lv_obj_align(spinner, LV_ALIGN_CENTER, 0, 10);

  g_splash_status = make_label(g_splash, CYD_S_SPLASH_STARTING, t.font_small, t.text_muted);
  lv_obj_set_width(g_splash_status, CYD_SCREEN_W - 24);
  lv_obj_set_style_text_align(g_splash_status, LV_TEXT_ALIGN_CENTER, 0);
  lv_label_set_long_mode(g_splash_status, LV_LABEL_LONG_MODE_WRAP);
  lv_obj_align(g_splash_status, LV_ALIGN_CENTER, 0, 52);

  lv_obj_t* version = make_label(g_splash, "v" CYD_TEMPLATE_VERSION, t.font_small, t.text_muted);
  lv_obj_align(version, LV_ALIGN_BOTTOM_RIGHT, -6, -4);

  lv_screen_load(g_splash);
  pump_ui(30);
}

void splash_status(const char* text) {
  if (g_splash_status != nullptr) lv_label_set_text(g_splash_status, text);
  log::info("boot: %s", text);
  pump_ui(10);
}

void splash_hide() {
  // ui_init() has loaded the start screen; the splash is no longer shown.
  if (g_splash != nullptr && lv_screen_active() != g_splash) {
    lv_obj_delete(g_splash);
    g_splash = nullptr;
    g_splash_status = nullptr;
  }
}

// BOOT held for 5 s during the splash window erases the Wi-Fi credentials
// (press it right after power-on: holding it through reset enters the ROM
// download mode). On resistive boards a 3 s screen hold re-runs calibration.
void splash_window() {
  uint32_t t0 = millis();
  uint32_t boot_since = 0;
  uint32_t touch_since = 0;
  bool force_cal = false;
  bool wiped = false;
  for (;;) {
    pump_ui(20);
    uint32_t now = millis();
    if (!wiped && boot_button_pressed()) {
      if (boot_since == 0) {
        boot_since = now;
        splash_status(CYD_S_WIFI_RESET_HOLD);
      } else if (now - boot_since >= CYD_BOOT_RESET_HOLD_MS) {
        wifi_forget_credentials();
        wiped = true;
        splash_status(CYD_S_WIFI_RESET_DONE);
        while (boot_button_pressed()) pump_ui(20);
        pump_ui(1500);
        boot_since = 0;
        t0 = millis();
        splash_status(CYD_S_SPLASH_STARTING);
      }
    } else if (boot_since != 0) {
      boot_since = 0;
      splash_status(CYD_S_SPLASH_STARTING);
    }
#if defined(CYD_TOUCH_DRIVER_XPT2046)
    uint16_t rx, ry;
    if (touch_raw(&rx, &ry)) {
      if (touch_since == 0) {
        touch_since = now;
      } else if (!force_cal && now - touch_since >= CYD_CAL_HOLD_MS) {
        force_cal = true;
        splash_status(CYD_S_CAL_RELEASE);
      }
    } else {
      if (force_cal) break;
      touch_since = 0;
    }
#endif
    if (boot_since == 0 && touch_since == 0 && now - t0 >= CYD_SPLASH_WINDOW_MS) break;
  }
  if (force_cal) touchcal_run();
}

}  // namespace detail
}  // namespace cyd
