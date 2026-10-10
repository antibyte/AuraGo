// cyd_private.h - internal interfaces between the runtime modules. Not part of
// the app API; app code includes only cyd.h (through the generated ui.h).
#pragma once
#include "cyd.h"
#include "cyd_config.h"
#include "cyd_strings.h"

class HTTPClient;

namespace cyd {
namespace detail {

// cyd_build_info.cpp
const char* build_id();

// cyd_log.cpp
void logf(char level, const char* fmt, va_list ap);

// cyd_board.cpp - LovyanGFX panel, touch, backlight and on-board extras.
void board_begin();
void panel_push(int32_t x, int32_t y, int32_t w, int32_t h, const uint16_t* rgb565);
bool touch_screen(int16_t* x, int16_t* y);   // pressed point in screen coordinates
bool touch_raw(uint16_t* x, uint16_t* y);    // raw controller values (resistive only)
void touch_set_affine(const float affine[6]); // raw -> panel memory frame (resistive only)
void touch_screen_to_memory(float sx, float sy, float* mx, float* my);
void touch_default_affine(float affine[6]);  // from the board profile calibration
bool boot_button_pressed();
void board_extras_begin();

// cyd_display.cpp - LVGL display/input glue and the cross-task UI queue.
void display_begin();
void display_loop();
void ui_queue_begin();
void ui_queue_drain();

// cyd_storage.cpp - NVS helpers for the runtime's own namespaces.
bool sys_get_blob(const char* ns, const char* key, void* out, size_t len);
void sys_set_blob(const char* ns, const char* key, const void* data, size_t len);
String sys_get_string(const char* ns, const char* key);
void sys_set_string(const char* ns, const char* key, const String& value);
void sys_clear(const char* ns);

// cyd_screens.cpp - splash and shared overlay helpers (themed by ui_theme_colors()).
void splash_show();
void splash_status(const char* text);
void splash_hide();
void splash_window();                 // BOOT hold (Wi-Fi reset) and touch hold (calibration)
lv_obj_t* make_screen();              // themed empty screen
lv_obj_t* make_button(lv_obj_t* parent, const char* text, lv_event_cb_t cb, void* user);
lv_obj_t* make_label(lv_obj_t* parent, const char* text, const lv_font_t* font, lv_color_t color);
void pump_ui(uint32_t ms);            // run LVGL for ms (used by blocking boot phases)

// cyd_touchcal.cpp
void touchcal_begin();                // apply stored or default calibration
bool touchcal_needed();               // resistive, profile asks for it, nothing stored
void touchcal_run();                  // blocking calibration screen, stores the result

// cyd_wifi.cpp / cyd_wifi_ui.cpp
void wifi_begin();                    // boot: stored credentials (<= 20 s), required -> setup
void wifi_loop();
void wifi_forget_credentials();       // BOOT reset path (no UI)
bool wifi_load_credentials(String* ssid, String* pass);
void wifi_save_credentials(const String& ssid, const String& pass);
void wifi_set_state(wifi::State s);
void wifi_connect(const String& ssid, const String& pass);
void wifi_ui_open(bool closable);     // setup screen (portal + on-device path)
void wifi_ui_close();
bool wifi_ui_active();
void wifi_ui_loop();
void portal_start(String* ap_ssid, String* ap_pass);
void portal_stop();
bool portal_active();
bool portal_loop();                   // true once the phone path connected
void time_begin();                    // NTP + TZ after the first connection

// cyd_http.cpp - one worker task for blocking network work.
void worker_begin();
bool worker_post(std::function<void()> job);
bool http_read_body(HTTPClient& http, size_t cap, String* out);  // false when the body is longer than cap

// cyd_link.cpp - AuraGo link (config partition, heartbeat, OTA).
void link_begin();
void link_loop();
void ota_overlay_show(const char* text);
void ota_overlay_progress(int pct);
void ota_overlay_hide();

// cyd_rollback.cpp - boot counter in RTC memory + bootloader rollback.
void rollback_guard_early();
void rollback_loop();
void rollback_mark_stable(const char* why);
bool rollback_pending_verify();
void rollback_remember_update();      // before an OTA switch: ties a rollback to this OTA (running = target, new = source)

}  // namespace detail
}  // namespace cyd
