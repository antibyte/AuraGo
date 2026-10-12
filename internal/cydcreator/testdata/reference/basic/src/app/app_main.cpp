// src/app/app_main.cpp - reference app logic (hand-written "agent" code for the reference sample).
#include "ui.h"

static uint32_t g_last_tick = 0;
static int g_seconds = 0;
static bool g_probe_done = false;
static int32_t g_level_to_save = -1;  // NVS write at most once per second

// One HTTPS request after the first Wi-Fi connection: proves that TLS fits
// into the heap next to LVGL, Wi-Fi and the portal (hardware check, plan 1b).
static void tls_probe() {
  cyd::log::info("tls probe: free heap %lu before", (unsigned long)ESP.getFreeHeap());
  cyd::http::get_json_async("https://httpbin.org/json", [](int status, JsonDocument &doc, const String &err) {
    const char *title = doc["slideshow"]["title"] | "";
    cyd::log::info("tls probe: status %d error \"%s\" title \"%s\", free heap %lu", status, err.c_str(), title,
                   (unsigned long)ESP.getFreeHeap());
  });
}

void app_setup() {
  cyd::log::info("reference app started, light sensor %d", cyd::board::light_level());
  cyd::wifi::on_change([](cyd::wifi::State s) {
    if (s == cyd::wifi::State::Connected && !g_probe_done) {
      g_probe_done = true;
      cyd::log::info("wifi up, ip %s, rssi %d", cyd::wifi::ip().c_str(), cyd::wifi::rssi());
      tls_probe();
    }
  });
  if (cyd::wifi::connected() && !g_probe_done) {
    g_probe_done = true;
    tls_probe();
  }
  int32_t level = cyd::storage::get_int("level", 100);
  lv_slider_set_value(ui_sld_level, level, LV_ANIM_OFF);
  cyd::board::backlight((uint8_t)level);
}

void app_loop() {
  if (millis() - g_last_tick < 1000) return;
  g_last_tick = millis();
  g_seconds++;
  if (g_level_to_save >= 0) {
    cyd::storage::set_int("level", g_level_to_save);
    g_level_to_save = -1;
  }
  if (g_seconds % 10 == 0) {
    lv_label_set_text_fmt(ui_lbl_temp, "%d s", g_seconds);
  }
}

void on_sld_level_value_changed(lv_event_t *e) {
  int32_t v = lv_slider_get_value(lv_event_get_target_obj(e));
  cyd::board::backlight((uint8_t)v);
  g_level_to_save = v;
}

void on_sw_light_value_changed(lv_event_t *e) {
  bool on = lv_obj_has_state(lv_event_get_target_obj(e), LV_STATE_CHECKED);
  cyd::board::rgb(on ? 255 : 0, on ? 160 : 0, on ? 40 : 0);
  if (on) cyd::board::beep(1800, 40);
}
