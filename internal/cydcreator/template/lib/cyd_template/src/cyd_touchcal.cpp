// cyd_touchcal.cpp - resistive (XPT2046) touch calibration.
//
// The calibration is an affine map raw -> panel memory frame, exactly the
// representation LovyanGFX uses internally (Panel_Device::setCalibrateAffine),
// so LovyanGFX keeps handling every rotation. It is stored in NVS namespace
// "cyd_sys", key "tcal" and survives app updates and orientation changes.
#include "cyd_private.h"

namespace cyd {
namespace {

constexpr uint32_t kCalMagic = 0x31414354;  // "TCA1"
constexpr const char* kNs = "cyd_sys";
constexpr const char* kKey = "tcal";

struct StoredCal {
  uint32_t magic;
  uint32_t board_key;  // FNV-1a of board id, panel offset rotation and native size
  float affine[6];
};

#if defined(CYD_TOUCH_DRIVER_XPT2046)
constexpr float kMaxResidualPx = 10.0f;     // a fit worse than this at any point is rejected
                                            // (one bad corner tap of error e leaves only about e/4 residual)
constexpr uint32_t kCalIdleMs = 60000;      // nobody touches the screen: give up, store nothing
constexpr uint32_t kCalReleaseMs = 10000;   // a touch that never releases: give up too

// A stored calibration only fits the hardware it was made for. The key covers
// everything that changes the raw -> panel memory mapping (board profile, the
// controller frame offset and the native panel size), so reflashing a different
// board or profile offers a fresh calibration instead of a wrong one.
uint32_t board_key() {
  uint32_t h = 2166136261u;
  for (const char* p = CYD_BOARD_ID; *p != 0; ++p) {
    h ^= (uint8_t)*p;
    h *= 16777619u;
  }
  const uint32_t extra[3] = {CYD_PANEL_OFFSET_ROTATION, CYD_PANEL_NATIVE_W, CYD_PANEL_NATIVE_H};
  for (int i = 0; i < 3; ++i) {
    for (int b = 0; b < 4; ++b) {
      h ^= (extra[i] >> (8 * b)) & 0xFF;
      h *= 16777619u;
    }
  }
  return h;
}

bool load_cal(StoredCal* out) {
  return detail::sys_get_blob(kNs, kKey, out, sizeof(*out)) && out->magic == kCalMagic &&
         out->board_key == board_key();
}

// Least squares fit of target = a*rx + b*ry + c over n samples.
bool fit_axis(const float* rx, const float* ry, const float* target, int n, float out[3]) {
  double m[3][4] = {};
  for (int i = 0; i < n; ++i) {
    double row[3] = {rx[i], ry[i], 1.0};
    for (int r = 0; r < 3; ++r) {
      for (int c = 0; c < 3; ++c) m[r][c] += row[r] * row[c];
      m[r][3] += row[r] * target[i];
    }
  }
  // Singular means "small compared to the matrix", not "smaller than 1e-9":
  // the entries scale with the squared raw range (about 1e7).
  double scale = 0;
  for (int r = 0; r < 3; ++r)
    for (int c = 0; c < 4; ++c)
      if (fabs(m[r][c]) > scale) scale = fabs(m[r][c]);
  const double eps = 1e-9 * scale;
  for (int col = 0; col < 3; ++col) {
    int pivot = col;
    for (int r = col + 1; r < 3; ++r)
      if (fabs(m[r][col]) > fabs(m[pivot][col])) pivot = r;
    if (fabs(m[pivot][col]) <= eps) return false;
    for (int c = 0; c < 4; ++c) {
      double t = m[col][c];
      m[col][c] = m[pivot][c];
      m[pivot][c] = t;
    }
    for (int r = 0; r < 3; ++r) {
      if (r == col) continue;
      double f = m[r][col] / m[col][col];
      for (int c = col; c < 4; ++c) m[r][c] -= f * m[col][c];
    }
  }
  for (int i = 0; i < 3; ++i) out[i] = (float)(m[i][3] / m[i][i]);
  return true;
}

// Largest |fitted - target| over the samples, in panel pixels.
float max_residual(const float* rx, const float* ry, const float* target, int n, const float c[3]) {
  float worst = 0;
  for (int i = 0; i < n; ++i) {
    float err = fabsf(c[0] * rx[i] + c[1] * ry[i] + c[2] - target[i]);
    if (err > worst) worst = err;
  }
  return worst;
}

lv_obj_t* make_cross(lv_obj_t* parent, int x, int y, lv_color_t color) {
  lv_obj_t* box = lv_obj_create(parent);
  lv_obj_remove_style_all(box);
  lv_obj_set_size(box, 25, 25);
  lv_obj_set_pos(box, x - 12, y - 12);
  lv_obj_t* h = lv_obj_create(box);
  lv_obj_remove_style_all(h);
  lv_obj_set_size(h, 25, 3);
  lv_obj_set_pos(h, 0, 11);
  lv_obj_set_style_bg_color(h, color, 0);
  lv_obj_set_style_bg_opa(h, LV_OPA_COVER, 0);
  lv_obj_t* v = lv_obj_create(box);
  lv_obj_remove_style_all(v);
  lv_obj_set_size(v, 3, 25);
  lv_obj_set_pos(v, 11, 0);
  lv_obj_set_style_bg_color(v, color, 0);
  lv_obj_set_style_bg_opa(v, LV_OPA_COVER, 0);
  return box;
}

// Waits for a steady press and returns the averaged raw point; then waits for
// release. Returns false when nobody touches the screen for kCalIdleMs (or the
// touch never releases within kCalReleaseMs): the caller abandons calibration.
bool sample_point(float* rx, float* ry) {
  constexpr int kNeed = 8;
  constexpr int kMaxJitter = 20;
  uint32_t sx = 0, sy = 0;
  int got = 0;
  uint16_t lx = 0, ly = 0;
  const uint32_t t0 = millis();
  while (got < kNeed) {
    if (millis() - t0 >= kCalIdleMs) return false;
    detail::pump_ui(10);
    uint16_t x, y;
    if (!detail::touch_raw(&x, &y)) {
      got = 0;
      sx = sy = 0;
      continue;
    }
    if (got > 0 && (abs((int)x - (int)lx) > kMaxJitter || abs((int)y - (int)ly) > kMaxJitter)) {
      got = 0;
      sx = sy = 0;
      continue;
    }
    lx = x;
    ly = y;
    sx += x;
    sy += y;
    ++got;
  }
  *rx = (float)sx / kNeed;
  *ry = (float)sy / kNeed;
  const uint32_t t1 = millis();
  uint16_t x, y;
  while (detail::touch_raw(&x, &y)) {
    if (millis() - t1 >= kCalReleaseMs) return false;
    detail::pump_ui(10);
  }
  return true;
}
#endif

}  // namespace

namespace detail {

void touchcal_begin() {
#if defined(CYD_TOUCH_DRIVER_XPT2046)
  StoredCal cal;
  if (load_cal(&cal)) {
    touch_set_affine(cal.affine);
    log::info("touch: stored calibration");
  } else {
    float a[6];
    touch_default_affine(a);
    touch_set_affine(a);
    log::info("touch: profile default calibration");
  }
#endif
}

bool touchcal_needed() {
#if defined(CYD_TOUCH_DRIVER_XPT2046)
  StoredCal cal;
  return CYD_TOUCH_NEEDS_CAL && !load_cal(&cal);
#else
  return false;
#endif
}

void touchcal_run() {
#if defined(CYD_TOUCH_DRIVER_XPT2046)
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* prev = lv_screen_active();
  lv_obj_t* scr = make_screen();
  lv_obj_t* title = make_label(scr, CYD_S_CAL_TITLE, t.font_normal, t.text);
  lv_obj_align(title, LV_ALIGN_CENTER, 0, -14);
  lv_obj_t* hint = make_label(scr, CYD_S_CAL_HINT, t.font_small, t.text_muted);
  lv_obj_set_width(hint, CYD_SCREEN_W - 80);
  lv_obj_set_style_text_align(hint, LV_TEXT_ALIGN_CENTER, 0);
  lv_label_set_long_mode(hint, LV_LABEL_LONG_MODE_WRAP);
  lv_obj_align(hint, LV_ALIGN_CENTER, 0, 14);
  lv_screen_load(scr);

  const int inset = 20;
  const int xs[4] = {inset, CYD_SCREEN_W - 1 - inset, inset, CYD_SCREEN_W - 1 - inset};
  const int ys[4] = {inset, inset, CYD_SCREEN_H - 1 - inset, CYD_SCREEN_H - 1 - inset};
  bool saved = false;
  for (;;) {
    float rx[4] = {}, ry[4] = {}, mx[4] = {}, my[4] = {};
    bool gave_up = false;
    for (int i = 0; i < 4; ++i) {
      lv_obj_t* cross = make_cross(scr, xs[i], ys[i], t.accent);
      pump_ui(300);  // ignore a touch that is still held from the previous point
      if (!sample_point(&rx[i], &ry[i])) {
        gave_up = true;
        break;  // the cross is deleted together with the screen below
      }
      touch_screen_to_memory((float)xs[i], (float)ys[i], &mx[i], &my[i]);
      lv_obj_delete(cross);
      board::beep(2000, 30);
    }
    if (gave_up) {
      // Nothing is stored and the mapping set by touchcal_begin() (stored or
      // profile default) stays active; the next boot offers calibration again.
      log::warn("touch: calibration timed out, keeping the current mapping");
      break;
    }
    StoredCal cal = {};
    cal.magic = kCalMagic;
    cal.board_key = board_key();
    if (fit_axis(rx, ry, mx, 4, &cal.affine[0]) && fit_axis(rx, ry, my, 4, &cal.affine[3])) {
      float worst = max_residual(rx, ry, mx, 4, &cal.affine[0]);
      float worst_y = max_residual(rx, ry, my, 4, &cal.affine[3]);
      if (worst_y > worst) worst = worst_y;
      if (worst <= kMaxResidualPx) {
        sys_set_blob(kNs, kKey, &cal, sizeof(cal));
        touch_set_affine(cal.affine);
        log::info("touch: calibration saved (%.4f %.4f %.1f / %.4f %.4f %.1f, residual %.1f px)", cal.affine[0],
                  cal.affine[1], cal.affine[2], cal.affine[3], cal.affine[4], cal.affine[5], (double)worst);
        saved = true;
        break;
      }
      log::warn("touch: calibration residual %.1f px is too large, retrying", (double)worst);
    } else {
      log::warn("touch: calibration points degenerate, retrying");
    }
    // Same hint again, now in the danger color, then the crosses start over.
    lv_label_set_text(hint, CYD_S_CAL_HINT);
    lv_obj_set_style_text_color(hint, t.danger, 0);
  }
  if (saved) {
    lv_obj_set_style_text_color(hint, t.text_muted, 0);
    lv_label_set_text(hint, CYD_S_CAL_DONE);
    pump_ui(800);
  }
  lv_screen_load(prev);
  lv_obj_delete(scr);
#endif
}

}  // namespace detail
}  // namespace cyd
