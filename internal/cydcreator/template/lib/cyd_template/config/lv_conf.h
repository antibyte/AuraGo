// lv_conf.h - LVGL 9.5 configuration for CYD Creator template 1.0.0.
// Found through the build flags "-DLV_CONF_INCLUDE_SIMPLE -Ilib/cyd_template/config";
// everything not set here keeps the default from lv_conf_internal.h.
#ifndef LV_CONF_H
#define LV_CONF_H

// No #include here: LVGL also includes this file from assembler sources (.S).

#define LV_COLOR_DEPTH 16

// Use the ESP-IDF heap (internal RAM, PSRAM for large blocks when present)
// instead of a fixed LVGL pool, so LVGL, Wi-Fi and TLS share one heap.
#define LV_USE_STDLIB_MALLOC LV_STDLIB_CLIB
#define LV_USE_STDLIB_STRING LV_STDLIB_CLIB
#define LV_USE_STDLIB_SPRINTF LV_STDLIB_CLIB

// LVGL is only called from the Arduino loop task (cyd.h threading rule).
#define LV_USE_OS LV_OS_NONE

#define LV_DEF_REFR_PERIOD 33
#define LV_DPI_DEF 130

#define LV_USE_LOG 1
#define LV_LOG_LEVEL LV_LOG_LEVEL_WARN
#define LV_LOG_PRINTF 0

// A failed LVGL assert aborts (backtrace on the serial monitor, then reboot)
// instead of spinning forever.
#define LV_ASSERT_HANDLER_INCLUDE <stdlib.h>
#define LV_ASSERT_HANDLER abort();

// One built-in font as fallback; every themed font comes from src/generated/fonts.
// cyd-build converts fonts with RLE compression (about 40 % less flash).
#define LV_USE_FONT_COMPRESSED 1
#define LV_FONT_MONTSERRAT_14 1
#define LV_FONT_DEFAULT &lv_font_montserrat_14

// Only the destination formats the runtime renders into: RGB565 (display),
// RGB565A8 (images), ARGB8888 (layers for opacity/transform), I1 (QR codes), A8.
#define LV_DRAW_SW_SUPPORT_RGB565 1
#define LV_DRAW_SW_SUPPORT_RGB565_SWAPPED 0
#define LV_DRAW_SW_SUPPORT_RGB565A8 1
#define LV_DRAW_SW_SUPPORT_RGB888 0
#define LV_DRAW_SW_SUPPORT_XRGB8888 0
#define LV_DRAW_SW_SUPPORT_ARGB8888 1
#define LV_DRAW_SW_SUPPORT_ARGB8888_PREMULTIPLIED 0
#define LV_DRAW_SW_SUPPORT_L8 0
#define LV_DRAW_SW_SUPPORT_AL88 0
#define LV_DRAW_SW_SUPPORT_A8 1
#define LV_DRAW_SW_SUPPORT_I1 1

#define LV_USE_THEME_DEFAULT 1
#define LV_USE_FLEX 1
#define LV_USE_GRID 1
#define LV_USE_QRCODE 1

#define LV_USE_SYSMON 0
#define LV_USE_PERF_MONITOR 0
#define LV_USE_MEM_MONITOR 0
#define LV_BUILD_EXAMPLES 0
#define LV_BUILD_DEMOS 0

#endif  // LV_CONF_H
