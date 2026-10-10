// cyd.h - public API of the CYD Creator device runtime (template 1.0.0).
//
// Threading rule: LVGL may only be called from the Arduino loop task. Code that
// runs anywhere else (HTTP callbacks are already marshalled for you) must hand
// UI work to cyd::ui_async().
#pragma once
#include <Arduino.h>
#include <ArduinoJson.h>
#include <lvgl.h>
#include <functional>

void app_setup();
void app_loop();

namespace cyd {

struct ThemeColors {
  lv_color_t bg, surface, surface_alt, text, text_muted, accent, on_accent, success, warning, danger, border, focus;
  uint8_t border_width, radius;
  const lv_font_t *font_small, *font_normal, *font_large, *font_mono;
  bool bracket_buttons;
};

namespace wifi {
enum class State { Off, Connecting, Connected, Offline, Setup };
State state();
bool connected();
void on_change(std::function<void(State)> cb);
void open_setup();
void forget();
String ip();
int rssi();
}  // namespace wifi

namespace http {
// The callback runs on the loop task, so it may update LVGL directly.
// status is the HTTP status (0 on transport errors); error is "" on success.
using JsonCallback = std::function<void(int status, JsonDocument& doc, const String& error)>;
void get_json_async(const String& url, JsonCallback cb, uint32_t timeout_ms = 10000);
void post_json_async(const String& url, const JsonDocument& body, JsonCallback cb, uint32_t timeout_ms = 10000);
}  // namespace http

namespace time {
bool synced();
time_t now();
String format(const char* strftime_fmt);
}  // namespace time

namespace storage {
int32_t get_int(const char* key, int32_t def);
void set_int(const char* key, int32_t v);
float get_float(const char* key, float def);
void set_float(const char* key, float v);
bool get_bool(const char* key, bool def);
void set_bool(const char* key, bool v);
String get_string(const char* key, const String& def);
void set_string(const char* key, const String& v);
void remove(const char* key);
}  // namespace storage

namespace board {
enum class Feature { RgbLed, LightSensor, Speaker, SdCard };
bool has(Feature f);
void backlight(uint8_t pct);
void rgb(uint8_t r, uint8_t g, uint8_t b);
int light_level();
void beep(uint16_t hz, uint16_t ms);
}  // namespace board

// Run fn on the loop task (the only place LVGL may be called). Safe from any task.
void ui_async(std::function<void()> fn);

namespace log {
void info(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void warn(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
void error(const char* fmt, ...) __attribute__((format(printf, 1, 2)));
}  // namespace log

}  // namespace cyd

// Provided by the generated code (src/generated/ui.cpp, ui_theme.cpp).
void ui_init(void);
const cyd::ThemeColors& ui_theme_colors(void);
