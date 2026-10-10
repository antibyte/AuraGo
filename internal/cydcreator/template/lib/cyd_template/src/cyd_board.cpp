// cyd_board.cpp - LovyanGFX panel + touch built only from include/board_config.h,
// plus backlight, RGB LED, light sensor, speaker and BOOT button.
#include "cyd_private.h"

#define LGFX_USE_V1
#include <LovyanGFX.hpp>
#if defined(CYD_PANEL_BUS_RGB)
#include <lgfx/v1/platforms/esp32s3/Bus_RGB.hpp>
#include <lgfx/v1/platforms/esp32s3/Panel_RGB.hpp>
#endif
#if defined(CYD_PANEL_DRIVER_NV3041A)
#include <lgfx/v1/panel/Panel_NV3041A.hpp>
#endif
#include <esp_timer.h>

#if defined(CYD_PANEL_DRIVER_ILI9341)
using CydPanel = lgfx::Panel_ILI9341;
#elif defined(CYD_PANEL_DRIVER_ST7789)
using CydPanel = lgfx::Panel_ST7789;
#elif defined(CYD_PANEL_DRIVER_ST7796)
using CydPanel = lgfx::Panel_ST7796;
#elif defined(CYD_PANEL_DRIVER_NV3041A)
using CydPanel = lgfx::Panel_NV3041A;
#elif defined(CYD_PANEL_DRIVER_ST7262) || defined(CYD_PANEL_DRIVER_EK9716)
using CydPanel = lgfx::Panel_RGB;
#else
#error "board_config.h: unknown CYD_PANEL_DRIVER_*"
#endif

// The NV3041A cannot rotate by register (LovyanGFX note), so a portrait
// orientation cannot work: such boards stay landscape. The internal LovyanGFX
// rotation is the project rotation plus the panel offset rotation; odd values
// swap the axes (see internal_rotation() below).
#if defined(CYD_PANEL_DRIVER_NV3041A) && ((CYD_ROTATION + CYD_PANEL_OFFSET_ROTATION) & 1)
#error "NV3041A panels support landscape only"
#endif

#if defined(CYD_TOUCH_DRIVER_XPT2046)
using CydTouch = lgfx::Touch_XPT2046;
#elif defined(CYD_TOUCH_DRIVER_GT911)
using CydTouch = lgfx::Touch_GT911;
#elif defined(CYD_TOUCH_DRIVER_CST820)
using CydTouch = lgfx::Touch_CST816S;  // CST820 speaks the CST816S protocol
#else
#error "board_config.h: unknown CYD_TOUCH_DRIVER_*"
#endif

#if defined(CYD_PANEL_BUS_RGB)
using CydBus = lgfx::Bus_RGB;
#else
using CydBus = lgfx::Bus_SPI;  // SPI and quad SPI (NV3041A)
#endif

// LEDC channels: RGB LED 0..2, speaker 4, LovyanGFX backlight 7 (ESP32-S3 has 8).
#define CYD_LEDC_LED_R 0
#define CYD_LEDC_LED_G 1
#define CYD_LEDC_LED_B 2
#define CYD_LEDC_SPEAKER 4
#define CYD_LEDC_BACKLIGHT 7

namespace {

class CydLGFX : public lgfx::LGFX_Device {
 public:
  CydPanel panel_;
  CydBus bus_;
  CydTouch touch_;
  lgfx::Light_PWM light_;

  CydLGFX() {
    configure_bus();
    configure_panel();
    configure_touch();
#if CYD_PIN_LCD_BL >= 0 && CYD_BL_PWM
    {
      auto cfg = light_.config();
      cfg.pin_bl = CYD_PIN_LCD_BL;
      cfg.invert = false;
      cfg.freq = 12000;
      cfg.pwm_channel = CYD_LEDC_BACKLIGHT;
      light_.config(cfg);
      panel_.setLight(&light_);
    }
#endif
    setPanel(&panel_);
  }

 private:
  void configure_bus() {
    auto cfg = bus_.config();
#if defined(CYD_PANEL_BUS_RGB)
    cfg.panel = &panel_;
    cfg.pin_d0 = CYD_PIN_LCD_B0;
    cfg.pin_d1 = CYD_PIN_LCD_B1;
    cfg.pin_d2 = CYD_PIN_LCD_B2;
    cfg.pin_d3 = CYD_PIN_LCD_B3;
    cfg.pin_d4 = CYD_PIN_LCD_B4;
    cfg.pin_d5 = CYD_PIN_LCD_G0;
    cfg.pin_d6 = CYD_PIN_LCD_G1;
    cfg.pin_d7 = CYD_PIN_LCD_G2;
    cfg.pin_d8 = CYD_PIN_LCD_G3;
    cfg.pin_d9 = CYD_PIN_LCD_G4;
    cfg.pin_d10 = CYD_PIN_LCD_G5;
    cfg.pin_d11 = CYD_PIN_LCD_R0;
    cfg.pin_d12 = CYD_PIN_LCD_R1;
    cfg.pin_d13 = CYD_PIN_LCD_R2;
    cfg.pin_d14 = CYD_PIN_LCD_R3;
    cfg.pin_d15 = CYD_PIN_LCD_R4;
    cfg.pin_henable = CYD_PIN_LCD_DE;
    cfg.pin_vsync = CYD_PIN_LCD_VSYNC;
    cfg.pin_hsync = CYD_PIN_LCD_HSYNC;
    cfg.pin_pclk = CYD_PIN_LCD_PCLK;
    cfg.freq_write = CYD_RGB_PCLK_HZ;
    cfg.hsync_polarity = CYD_RGB_HSYNC_POLARITY;
    cfg.hsync_front_porch = CYD_RGB_HSYNC_FRONT_PORCH;
    cfg.hsync_pulse_width = CYD_RGB_HSYNC_PULSE_WIDTH;
    cfg.hsync_back_porch = CYD_RGB_HSYNC_BACK_PORCH;
    cfg.vsync_polarity = CYD_RGB_VSYNC_POLARITY;
    cfg.vsync_front_porch = CYD_RGB_VSYNC_FRONT_PORCH;
    cfg.vsync_pulse_width = CYD_RGB_VSYNC_PULSE_WIDTH;
    cfg.vsync_back_porch = CYD_RGB_VSYNC_BACK_PORCH;
    // LovyanGFX drives the pixel clock edge through pclk_idle_high; the
    // profile's pclk_active_neg is the fallback when pclk_idle_high is absent.
    cfg.pclk_idle_high = (CYD_RGB_PCLK_IDLE_HIGH >= 0) ? CYD_RGB_PCLK_IDLE_HIGH : CYD_RGB_PCLK_ACTIVE_NEG;
    cfg.de_idle_high = (CYD_RGB_DE_IDLE_HIGH >= 0) ? CYD_RGB_DE_IDLE_HIGH : 0;
#elif defined(CYD_PANEL_BUS_QSPI)
    cfg.spi_host = SPI2_HOST;
    cfg.spi_mode = 1;
    cfg.freq_write = CYD_PANEL_HZ;
    cfg.freq_read = 16000000;
    cfg.spi_3wire = true;
    cfg.use_lock = true;
    cfg.dma_channel = SPI_DMA_CH_AUTO;
    cfg.pin_sclk = CYD_PIN_LCD_SCLK;
    cfg.pin_io0 = CYD_PIN_LCD_D0;
    cfg.pin_io1 = CYD_PIN_LCD_D1;
    cfg.pin_io2 = CYD_PIN_LCD_D2;
    cfg.pin_io3 = CYD_PIN_LCD_D3;
#else
#if defined(CYD_CHIP_ESP32)
    cfg.spi_host = HSPI_HOST;  // display on SPI2, touch (if separate) on SPI3
#else
    cfg.spi_host = SPI2_HOST;
#endif
    cfg.spi_mode = 0;
    cfg.freq_write = CYD_PANEL_HZ;
    cfg.freq_read = 16000000;
    cfg.spi_3wire = false;
    cfg.use_lock = true;
    cfg.dma_channel = SPI_DMA_CH_AUTO;
    cfg.pin_sclk = CYD_PIN_LCD_SCLK;
    cfg.pin_mosi = CYD_PIN_LCD_MOSI;
    cfg.pin_miso = CYD_PIN_LCD_MISO;
    cfg.pin_dc = CYD_PIN_LCD_DC;
#endif
    bus_.config(cfg);
    panel_.setBus(&bus_);
  }

  void configure_panel() {
    auto cfg = panel_.config();
#if defined(CYD_PANEL_BUS_RGB)
    cfg.memory_width = cfg.panel_width = CYD_PANEL_NATIVE_W;
    cfg.memory_height = cfg.panel_height = CYD_PANEL_NATIVE_H;
    cfg.offset_x = 0;
    cfg.offset_y = 0;
    cfg.offset_rotation = CYD_PANEL_OFFSET_ROTATION;
    panel_.config(cfg);
    auto detail = panel_.config_detail();
    detail.use_psram = 1;  // frame buffer in PSRAM
    panel_.config_detail(detail);
#else
    // panel_width/height keep the driver's controller-RAM geometry
    // (ILI9341/ST7789 240x320, ST7796 320x480, NV3041A 480x272);
    // CYD_PANEL_OFFSET_ROTATION turns that frame into the board's native one.
    cfg.pin_cs = CYD_PIN_LCD_CS;
    cfg.pin_rst = CYD_PIN_LCD_RST;
    cfg.pin_busy = -1;
    cfg.offset_x = 0;
    cfg.offset_y = 0;
    cfg.offset_rotation = CYD_PANEL_OFFSET_ROTATION;
    cfg.readable = false;
    cfg.invert = CYD_PANEL_INVERT;
    cfg.rgb_order = CYD_PANEL_BGR;
    cfg.dlen_16bit = false;
    // Touch on the display's SPI bus needs LovyanGFX to release the bus_.
    cfg.bus_shared = (CYD_PIN_TOUCH_SCLK >= 0 && CYD_PIN_TOUCH_SCLK == CYD_PIN_LCD_SCLK);
    panel_.config(cfg);
#endif
  }

  void configure_touch() {
    auto cfg = touch_.config();
#if defined(CYD_TOUCH_BUS_SPI)
    cfg.x_min = 0;
    cfg.x_max = 4095;
    cfg.y_min = 0;
    cfg.y_max = 4095;
    cfg.offset_rotation = 0;  // XPT2046 orientation comes from the calibration affine
    cfg.pin_int = CYD_PIN_TOUCH_IRQ;
    cfg.pin_cs = CYD_PIN_TOUCH_CS;
    cfg.pin_sclk = CYD_PIN_TOUCH_SCLK;
    cfg.pin_mosi = CYD_PIN_TOUCH_MOSI;
    cfg.pin_miso = CYD_PIN_TOUCH_MISO;
    cfg.freq = 2500000;
    cfg.bus_shared = (CYD_PIN_TOUCH_SCLK == CYD_PIN_LCD_SCLK);
#if defined(CYD_CHIP_ESP32)
    cfg.spi_host = cfg.bus_shared ? HSPI_HOST : VSPI_HOST;
#else
    cfg.spi_host = cfg.bus_shared ? SPI2_HOST : SPI3_HOST;
#endif
#else
    // Capacitive: raw coordinates already span the controller-RAM frame of
    // the panel; CYD_TOUCH_OFFSET_ROTATION corrects boards whose touch layer
    // is mounted turned or mirrored.
    int w = panel_.config().panel_width;
    int h = panel_.config().panel_height;
    if (CYD_TOUCH_OFFSET_ROTATION & 1) {
      int t = w;
      w = h;
      h = t;
    }
    cfg.x_min = 0;
    cfg.x_max = w - 1;
    cfg.y_min = 0;
    cfg.y_max = h - 1;
    cfg.offset_rotation = CYD_TOUCH_OFFSET_ROTATION;
    cfg.bus_shared = false;
    cfg.i2c_port = 1;
    cfg.i2c_addr = CYD_TOUCH_I2C_ADDR;
    cfg.pin_sda = CYD_PIN_TOUCH_SDA;
    cfg.pin_scl = CYD_PIN_TOUCH_SCL;
    cfg.pin_int = CYD_PIN_TOUCH_INT;
    cfg.pin_rst = CYD_PIN_TOUCH_RST;
    cfg.freq = 400000;
#endif
    touch_.config(cfg);
    panel_.setTouch(&touch_);
  }
};

CydLGFX* g_lcd = nullptr;
esp_timer_handle_t g_beep_timer = nullptr;

// Inverse of the transform LovyanGFX applies in Panel_Device::convertRawXY,
// which maps the panel memory frame (controller RAM) to the screen frame of
// internal rotation r: swap axes for odd r, then flip x for r&2 and flip y for
// r in {1,2,4,7}, using the screen size sw x sh.
void screen_to_memory(int r, int sw, int sh, float sx, float sy, float* mx, float* my) {
  float tx = sx, ty = sy;
  if ((1 << r) & 0b10010110) ty = (sh - 1) - ty;
  if (r & 2) tx = (sw - 1) - tx;
  if (r & 1) {
    float t = tx;
    tx = ty;
    ty = t;
  }
  *mx = tx;
  *my = ty;
}

int internal_rotation(int user_rotation) {
  int off = CYD_PANEL_OFFSET_ROTATION;
  return ((user_rotation + off) & 3) | ((user_rotation & 4) ^ (off & 4));
}

}  // namespace

namespace cyd {
namespace detail {

void board_begin() {
  g_lcd = new CydLGFX();
  if (!g_lcd->init()) {
    // Typical causes: no PSRAM for an RGB panel, wrong pins in the board profile.
    log::error("display init failed on board %s (panel driver, bus pins or PSRAM)", CYD_BOARD_ID);
    Serial.flush();
    abort();
  }
  g_lcd->setRotation(CYD_ROTATION);
  g_lcd->setColorDepth(16);
  g_lcd->fillScreen(TFT_BLACK);
  if (g_lcd->width() != CYD_SCREEN_W || g_lcd->height() != CYD_SCREEN_H) {
    log::error("display is %dx%d but the project expects %dx%d (rotation %d)", (int)g_lcd->width(),
               (int)g_lcd->height(), CYD_SCREEN_W, CYD_SCREEN_H, CYD_ROTATION);
  }
#if CYD_PIN_LCD_BL >= 0 && !CYD_BL_PWM
  pinMode(CYD_PIN_LCD_BL, OUTPUT);
#endif
  board::backlight(CYD_BACKLIGHT_DEFAULT);
  board_extras_begin();
  log::info("board %s: %dx%d rotation %d, panel %dx%d", CYD_BOARD_ID, (int)g_lcd->width(), (int)g_lcd->height(),
            CYD_ROTATION, (int)g_lcd->panel()->config().panel_width, (int)g_lcd->panel()->config().panel_height);
}

void board_extras_begin() {
  pinMode(CYD_PIN_BOOT, INPUT_PULLUP);
#if CYD_HAS_RGB_LED
  ledcSetup(CYD_LEDC_LED_R, 5000, 8);
  ledcSetup(CYD_LEDC_LED_G, 5000, 8);
  ledcSetup(CYD_LEDC_LED_B, 5000, 8);
  ledcAttachPin(CYD_PIN_LED_R, CYD_LEDC_LED_R);
  ledcAttachPin(CYD_PIN_LED_G, CYD_LEDC_LED_G);
  ledcAttachPin(CYD_PIN_LED_B, CYD_LEDC_LED_B);
  board::rgb(0, 0, 0);
#endif
#if CYD_HAS_LDR
  pinMode(CYD_PIN_LDR, INPUT);
#endif
#if CYD_HAS_SPEAKER
  ledcSetup(CYD_LEDC_SPEAKER, 2000, 8);
  ledcAttachPin(CYD_PIN_SPEAKER, CYD_LEDC_SPEAKER);
  ledcWrite(CYD_LEDC_SPEAKER, 0);
  esp_timer_create_args_t args = {};
  args.callback = [](void*) { ledcWrite(CYD_LEDC_SPEAKER, 0); };
  args.name = "cyd_beep";
  esp_timer_create(&args, &g_beep_timer);
#endif
}

void panel_push(int32_t x, int32_t y, int32_t w, int32_t h, const uint16_t* rgb565) {
  g_lcd->pushImage(x, y, w, h, (const lgfx::rgb565_t*)rgb565);
}

bool touch_screen(int16_t* x, int16_t* y) {
  lgfx::touch_point_t tp;
  if (g_lcd == nullptr || g_lcd->getTouch(&tp, 1) == 0) return false;
  // Touch layers often reach a little past the visible area: clamp those
  // points to the edge and drop only readings far outside.
  constexpr int kSlack = 20;
  const int w = g_lcd->width();
  const int h = g_lcd->height();
  int tx = tp.x;
  int ty = tp.y;
  if (tx < -kSlack || ty < -kSlack || tx > w - 1 + kSlack || ty > h - 1 + kSlack) return false;
  if (tx < 0) tx = 0;
  if (tx > w - 1) tx = w - 1;
  if (ty < 0) ty = 0;
  if (ty > h - 1) ty = h - 1;
  *x = (int16_t)tx;
  *y = (int16_t)ty;
  return true;
}

bool touch_raw(uint16_t* x, uint16_t* y) {
  lgfx::touch_point_t tp;
  if (g_lcd == nullptr || g_lcd->getTouchRaw(&tp, 1) == 0) return false;
  *x = (uint16_t)tp.x;
  *y = (uint16_t)tp.y;
  return true;
}

void touch_set_affine(const float affine[6]) {
  float a[6];
  memcpy(a, affine, sizeof(a));
  g_lcd->panel()->setCalibrateAffine(a);
}

void touch_screen_to_memory(float sx, float sy, float* mx, float* my) {
  screen_to_memory(internal_rotation(CYD_ROTATION), g_lcd->width(), g_lcd->height(), sx, sy, mx, my);
}

// Default XPT2046 mapping from the profile calibration. The profile values use
// the XPT2046_Touchscreen setRotation(1) convention on the board's native
// orientation: native_x = map(X, x_min..x_max, 0..W-1) with X = channel 0x91,
// native_y = map(Y, y_min..y_max, 0..H-1) with Y = channel 0xD1. LovyanGFX
// reports raw.x = channel 0xD1 and raw.y = channel 0x91, so X = raw.y, Y = raw.x.
// Native coordinates are the screen frame of user rotation 0; convert them to
// the panel memory frame, which is what LovyanGFX's affine produces.
void touch_default_affine(float affine[6]) {
#if defined(CYD_TOUCH_DRIVER_XPT2046)
  const float sx = (CYD_PANEL_NATIVE_W - 1) / (float)(CYD_TOUCH_CAL_X_MAX - CYD_TOUCH_CAL_X_MIN);
  const float sy = (CYD_PANEL_NATIVE_H - 1) / (float)(CYD_TOUCH_CAL_Y_MAX - CYD_TOUCH_CAL_Y_MIN);
  const int r0 = internal_rotation(0);
  auto native_to_memory = [&](float nx, float ny, float* mx, float* my) {
    screen_to_memory(r0, CYD_PANEL_NATIVE_W, CYD_PANEL_NATIVE_H, nx, ny, mx, my);
  };
  auto raw_to_memory = [&](float rawx, float rawy, float* mx, float* my) {
    float nx = (rawy - CYD_TOUCH_CAL_X_MIN) * sx;
    float ny = (rawx - CYD_TOUCH_CAL_Y_MIN) * sy;
    native_to_memory(nx, ny, mx, my);
  };
  float c0x, c0y, c1x, c1y, c2x, c2y;
  raw_to_memory(0, 0, &c0x, &c0y);
  raw_to_memory(1, 0, &c1x, &c1y);
  raw_to_memory(0, 1, &c2x, &c2y);
  affine[0] = c1x - c0x;  // d mx / d raw.x
  affine[1] = c2x - c0x;  // d mx / d raw.y
  affine[2] = c0x;
  affine[3] = c1y - c0y;
  affine[4] = c2y - c0y;
  affine[5] = c0y;
#else
  (void)affine;
#endif
}

bool boot_button_pressed() { return digitalRead(CYD_PIN_BOOT) == LOW; }

}  // namespace detail

namespace board {

bool has(Feature f) {
  switch (f) {
    case Feature::RgbLed:
      return CYD_HAS_RGB_LED;
    case Feature::LightSensor:
      return CYD_HAS_LDR;
    case Feature::Speaker:
      return CYD_HAS_SPEAKER;
    case Feature::SdCard:
      return CYD_HAS_SD;
  }
  return false;
}

void backlight(uint8_t pct) {
  if (pct > 100) pct = 100;
#if CYD_PIN_LCD_BL >= 0
#if CYD_BL_PWM
  if (g_lcd != nullptr) g_lcd->setBrightness((uint8_t)((pct * 255 + 50) / 100));
#else
  digitalWrite(CYD_PIN_LCD_BL, pct > 0 ? HIGH : LOW);
#endif
#endif
}

void rgb(uint8_t r, uint8_t g, uint8_t b) {
#if CYD_HAS_RGB_LED
  if (CYD_LED_ACTIVE_LOW) {
    r = 255 - r;
    g = 255 - g;
    b = 255 - b;
  }
  ledcWrite(CYD_LEDC_LED_R, r);
  ledcWrite(CYD_LEDC_LED_G, g);
  ledcWrite(CYD_LEDC_LED_B, b);
#else
  (void)r;
  (void)g;
  (void)b;
#endif
}

// 0 (dark) .. 100 (bright); -1 when the board has no light sensor.
int light_level() {
#if CYD_HAS_LDR
  int raw = analogRead(CYD_PIN_LDR);  // CYD LDR: dark = high ADC value
  return 100 - (raw * 100) / 4095;
#else
  return -1;
#endif
}

// Non-blocking: starts the tone and stops it from an esp_timer after ms.
void beep(uint16_t hz, uint16_t ms) {
#if CYD_HAS_SPEAKER
  if (hz == 0 || ms == 0 || g_beep_timer == nullptr) return;
  esp_timer_stop(g_beep_timer);
  ledcWriteTone(CYD_LEDC_SPEAKER, hz);
  esp_timer_start_once(g_beep_timer, (uint64_t)ms * 1000ULL);
#else
  (void)hz;
  (void)ms;
#endif
}

}  // namespace board
}  // namespace cyd
