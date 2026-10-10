// src/app/app_main.cpp - your app logic. CYD Creator creates this file once and never overwrites it.
//
// Rules for app code:
// - LVGL only runs on the loop task: app_setup(), app_loop(), event hooks and HTTP
//   callbacks are fine; from other tasks use cyd::ui_async().
// - No blocking delay() in app_loop(); use millis() timers or lv_timer_create().
// - Network requests go through cyd::http::get_json_async / post_json_async.
// - Widgets are reachable as ui_<name>; hooks for "logic" events are declared in ui.h.
#include "ui.h"

void app_setup() {
  cyd::log::info("app started");
}

void app_loop() {
}
