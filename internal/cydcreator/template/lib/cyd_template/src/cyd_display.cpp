// cyd_display.cpp - LVGL display/input glue and the ui_async queue.
#include "cyd_private.h"

#include <esp_heap_caps.h>
#include <freertos/queue.h>

namespace cyd {
namespace {

lv_display_t* g_disp = nullptr;
lv_indev_t* g_indev = nullptr;
QueueHandle_t g_ui_queue = nullptr;
TaskHandle_t g_loop_task = nullptr;

uint32_t tick_cb() { return millis(); }

void flush_cb(lv_display_t* disp, const lv_area_t* area, uint8_t* px_map) {
  int32_t w = lv_area_get_width(area);
  int32_t h = lv_area_get_height(area);
  detail::panel_push(area->x1, area->y1, w, h, (const uint16_t*)px_map);
#if CYD_LAYOUT_SELFTEST
  if (detail::flush_tap != nullptr) detail::flush_tap(area->x1, area->y1, w, h, (const uint16_t*)px_map);
#endif
  lv_display_flush_ready(disp);
}

void touch_cb(lv_indev_t* indev, lv_indev_data_t* data) {
  (void)indev;
  int16_t x, y;
  if (detail::touch_screen(&x, &y)) {
    data->point.x = x;
    data->point.y = y;
    data->state = LV_INDEV_STATE_PRESSED;
  } else {
    data->state = LV_INDEV_STATE_RELEASED;
  }
}

void log_cb(lv_log_level_t level, const char* buf) {
  (void)level;
  Serial.print("[lvgl] ");
  Serial.print(buf);
}

// Draw buffers: internal DMA-capable RAM first (fast SPI flushes), PSRAM as
// fallback on boards that have it; halve the line count until it fits.
void* alloc_draw_buffer(size_t bytes) {
  void* p = heap_caps_malloc(bytes, MALLOC_CAP_DMA | MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT);
#if CYD_HAS_PSRAM
  if (p == nullptr) p = heap_caps_malloc(bytes, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT);
#endif
  return p;
}

}  // namespace

namespace detail {

#if CYD_LAYOUT_SELFTEST
void (*flush_tap)(int32_t x, int32_t y, int32_t w, int32_t h, const uint16_t* px) = nullptr;
#endif

void display_begin() {
  g_loop_task = xTaskGetCurrentTaskHandle();  // setup() runs on the loop task
  ui_queue_begin();
  lv_init();
  lv_tick_set_cb(tick_cb);
  lv_log_register_print_cb(log_cb);

  g_disp = lv_display_create(CYD_SCREEN_W, CYD_SCREEN_H);
  lv_display_set_color_format(g_disp, LV_COLOR_FORMAT_RGB565);
  lv_display_set_flush_cb(g_disp, flush_cb);

  const uint32_t line_px = (CYD_SCREEN_W > CYD_SCREEN_H) ? CYD_SCREEN_W : CYD_SCREEN_H;
  uint32_t lines = CYD_DRAW_BUF_LINES;
  void* buf1 = nullptr;
  void* buf2 = nullptr;
  // Halve the line count until one buffer fits; the second (double) buffer is
  // optional: flushes are synchronous, so a single buffer renders correctly.
  while (lines >= 1) {
    size_t bytes = (size_t)line_px * lines * 2;
    buf1 = alloc_draw_buffer(bytes);
    if (buf1 != nullptr) {
      if (CYD_DRAW_BUF_DOUBLE) {
        buf2 = alloc_draw_buffer(bytes);
        if (buf2 == nullptr) log::warn("no memory for a second draw buffer, using a single buffer");
      }
      break;
    }
    lines /= 2;
  }
  if (buf1 == nullptr) {
    log::error("no memory for LVGL draw buffers (not even one line)");
    Serial.flush();
    abort();
  }
  if (lines != CYD_DRAW_BUF_LINES) log::warn("draw buffer reduced to %lu lines", (unsigned long)lines);
  lv_display_set_buffers(g_disp, buf1, buf2, line_px * lines * 2, LV_DISPLAY_RENDER_MODE_PARTIAL);

  g_indev = lv_indev_create();
  lv_indev_set_type(g_indev, LV_INDEV_TYPE_POINTER);
  lv_indev_set_read_cb(g_indev, touch_cb);
  lv_indev_set_display(g_indev, g_disp);

  log::info("lvgl %d.%d.%d: %dx%d, %lu-line buffer%s, free heap %lu", LVGL_VERSION_MAJOR, LVGL_VERSION_MINOR,
            LVGL_VERSION_PATCH, CYD_SCREEN_W, CYD_SCREEN_H, (unsigned long)lines, buf2 ? " x2" : "",
            (unsigned long)ESP.getFreeHeap());
}

void ui_queue_begin() {
  if (g_ui_queue != nullptr) return;
  g_ui_queue = xQueueCreate(CYD_UI_QUEUE_LEN, sizeof(std::function<void()>*));
  if (g_ui_queue == nullptr) log::error("could not create the ui_async queue, ui_async calls will be dropped");
}

void ui_queue_drain() {
  if (g_ui_queue == nullptr) return;
  std::function<void()>* fn = nullptr;
  // Bounded per loop pass so a flood of callbacks cannot starve rendering.
  for (int i = 0; i < CYD_UI_QUEUE_LEN && xQueueReceive(g_ui_queue, &fn, 0) == pdTRUE; ++i) {
    (*fn)();
    delete fn;
  }
}

void display_loop() {
  ui_queue_drain();
  lv_timer_handler();
}

}  // namespace detail

void ui_async(std::function<void()> fn) {
  // The queue is created once by display_begin() on the loop task before any
  // other task starts; creating it lazily here would race between tasks.
  if (g_ui_queue == nullptr) {
    log::warn("ui_async called before the runtime started, callback dropped");
    return;  // nothing was allocated yet
  }
  auto* heap_fn = new std::function<void()>(std::move(fn));
  // Other tasks wait briefly for room; the loop task itself must not wait on
  // the queue it drains.
  TickType_t wait = (xTaskGetCurrentTaskHandle() == g_loop_task) ? 0 : pdMS_TO_TICKS(1000);
  if (xQueueSend(g_ui_queue, &heap_fn, wait) != pdTRUE) {
    log::warn("ui_async queue full, callback dropped");
    delete heap_fn;
  }
}

}  // namespace cyd
