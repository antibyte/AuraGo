// cyd_config.h - pulls in the generated include/board_config.h and checks that
// the macro contract (plan 1b "board_config.h contract") is complete.
#pragma once
#include "board_config.h"

#ifndef CYD_BOARD_ID
#error "include/board_config.h is missing or incomplete (CYD_BOARD_ID undefined)"
#endif
#if defined(CYD_CHIP_ESP32) == defined(CYD_CHIP_ESP32S3)
#error "board_config.h must define exactly one of CYD_CHIP_ESP32 / CYD_CHIP_ESP32S3"
#endif
#if (defined(CYD_PANEL_BUS_SPI) + defined(CYD_PANEL_BUS_QSPI) + defined(CYD_PANEL_BUS_RGB)) != 1
#error "board_config.h must define exactly one CYD_PANEL_BUS_* macro"
#endif
#if (defined(CYD_TOUCH_BUS_SPI) + defined(CYD_TOUCH_BUS_I2C)) != 1
#error "board_config.h must define exactly one CYD_TOUCH_BUS_* macro"
#endif
#if (defined(CYD_WIFI_MODE) + defined(CYD_AURAGO_LINK) + defined(CYD_ROTATION) + defined(CYD_SCREEN_W) + \
     defined(CYD_SCREEN_H) + defined(CYD_DRAW_BUF_LINES) + defined(CYD_TEMPLATE_VERSION)) != 7
#error "board_config.h is missing project macros"
#endif
#if CYD_AURAGO_LINK && CYD_WIFI_MODE == 0
#error "CYD_AURAGO_LINK requires CYD_WIFI_MODE 1 or 2"
#endif

// Build id: cyd-build writes include/cyd_build_id.h (#define CYD_BUILD_ID "...")
// into the build tree; only cyd_build_info.cpp includes it, so a new id
// recompiles one file instead of the whole project.

#define CYD_WIFI_OFF 0
#define CYD_WIFI_OPTIONAL 1
#define CYD_WIFI_REQUIRED 2

// Test builds only: the on-device self-test of the Wi-Fi setup screen runs a few
// seconds after boot (cyd_layout.cpp): 1 = layout and reconnect (boots without
// joining the stored network), 2 = layout only. Never set by the generator.
#ifndef CYD_LAYOUT_SELFTEST
#define CYD_LAYOUT_SELFTEST 0
#endif

// Limits shared by the runtime.
#define CYD_HTTP_MAX_BODY 32768        // app HTTP responses larger than this fail with "too large"
#define CYD_HTTP_QUEUE_LEN 8           // pending async HTTP jobs
#define CYD_UI_QUEUE_LEN 32            // pending ui_async callbacks
#define CYD_WIFI_CONNECT_MS 20000      // boot-time connection attempt with stored credentials
#define CYD_WIFI_RETRY_MS 60000        // background retry while offline (optional mode)
#define CYD_BOOT_RESET_HOLD_MS 5000    // BOOT button hold that clears Wi-Fi credentials
#define CYD_CAL_HOLD_MS 3000           // screen hold at the splash that re-runs touch calibration
#define CYD_SPLASH_WINDOW_MS 1500      // how long the splash waits for BOOT / touch holds
#define CYD_STABLE_UPTIME_MS 120000    // uptime after which a boot counts as stable
#define CYD_ROLLBACK_THRESHOLD 3       // unstable boots in a row before switching back
#define CYD_CONFIG_SUBTYPE 0x41        // data partition "cydcc"
#define CYD_CONFIG_SIZE 4096
#define CYD_CONFIG_MAX_PAYLOAD 4088
#define CYD_OTA_CHUNK 4096
