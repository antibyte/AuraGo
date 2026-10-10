// cyd_main.cpp - Arduino entry points and the boot sequence of spec §7:
// serial log -> boot counter -> NVS -> board -> LVGL -> touch calibration ->
// Wi-Fi -> AuraGo link -> ui_init() -> app_setup(); then the loop task runs
// LVGL, Wi-Fi/portal, the heartbeat timer and app_loop().
#include "cyd_private.h"

#include <nvs_flash.h>

// LVGL rendering, the portal web server and app code share the loop task.
SET_LOOP_TASK_STACK_SIZE(12 * 1024);

void setup() {
  Serial.begin(115200);
  cyd::detail::rollback_guard_early();
  cyd::log::info("CYD Creator template %s, build %s, board %s, project \"%s\"", CYD_TEMPLATE_VERSION,
                 cyd::detail::build_id(), CYD_BOARD_ID, CYD_PROJECT_NAME);
  esp_err_t nvs = nvs_flash_init();  // already done by the Arduino core; repairs a full/old NVS
  if (nvs == ESP_ERR_NVS_NO_FREE_PAGES || nvs == ESP_ERR_NVS_NEW_VERSION_FOUND) {
    nvs_flash_erase();
    nvs_flash_init();
  }
  cyd::detail::board_begin();
  cyd::detail::display_begin();
  cyd::detail::touchcal_begin();
  cyd::detail::splash_show();
  cyd::detail::splash_window();
  if (cyd::detail::touchcal_needed()) cyd::detail::touchcal_run();
  cyd::detail::worker_begin();
  cyd::detail::wifi_begin();
  cyd::detail::link_begin();
  ui_init();
  cyd::detail::splash_hide();
  app_setup();
  cyd::log::info("boot complete, free heap %lu, largest block %lu", (unsigned long)ESP.getFreeHeap(),
                 (unsigned long)heap_caps_get_largest_free_block(MALLOC_CAP_8BIT));
}

void loop() {
  cyd::detail::display_loop();
  cyd::detail::wifi_loop();
  cyd::detail::link_loop();
  cyd::detail::rollback_loop();
  app_loop();
  delay(1);  // yield to the idle task (task watchdog)
}
