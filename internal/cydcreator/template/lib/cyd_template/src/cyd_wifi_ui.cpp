// cyd_wifi_ui.cpp - the themed Wi-Fi setup screen with two paths:
//   phone (default): captive portal, QR code WIFI:T:WPA;S:<ap>;P:<pass>;;
//   on device: LVGL scan list, hidden-SSID entry and on-screen keyboard.
// View switches and closing are deferred to wifi_ui_loop() so no widget is
// deleted inside its own event callback.
//
// Layout rules (every board, orientation, theme and language; layout_check()
// verifies them after every view change): only the network list scrolls, and
// only vertically; every label inside a container has a width and a long mode
// (wrap for prose and button captions, dots for single-line values); the
// content area ends at the keyboard while it is shown, so the fields stay
// above it, and the status replaces the keyboard while connecting.
#include "cyd_private.h"

#include <WiFi.h>

#include <vector>

namespace cyd {
namespace {

enum class View { None, Phone, List, Password };

struct SetupUi {
  lv_obj_t* scr = nullptr;
  lv_obj_t* prev = nullptr;
  lv_obj_t* title = nullptr;
  lv_obj_t* back_btn = nullptr;
  lv_obj_t* rescan_btn = nullptr;
  lv_obj_t* content = nullptr;
  lv_obj_t* status = nullptr;
  lv_obj_t* list = nullptr;
  lv_obj_t* ta_ssid = nullptr;
  lv_obj_t* ta_pass = nullptr;
  lv_obj_t* kb = nullptr;
  int32_t header_h = 0;
  int32_t kb_h = 0;
  bool closable = false;
  View view = View::None;
  View pending = View::Phone;
  bool close_requested = false;
  bool scanning = false;
  bool hidden = false;
  bool connecting = false;
  uint32_t connect_started = 0;
  uint32_t close_at = 0;
  uint32_t portal_retry_at = 0;
  String ap_ssid, ap_pass, sel_ssid;
  std::vector<String> found;
};

SetupUi* g_ui = nullptr;

constexpr int32_t kPad = 8;       // content padding
constexpr int32_t kGap = 6;       // gap between stacked items
constexpr int32_t kIconW = 44;    // header icon buttons
constexpr int32_t kQrMin = 96;    // QR code side (without the quiet zone)
constexpr int32_t kQrMax = 220;
constexpr int32_t kQrQuiet = 4;   // white frame around the code (quiet zone on dark themes)
constexpr int32_t kQrStep = 8;
constexpr size_t kMaxNetworks = 20;

int32_t line_h(const lv_font_t* f) { return lv_font_get_line_height(f); }

// A transparent flex container that never scrolls.
lv_obj_t* box(lv_obj_t* parent, lv_flex_flow_t flow) {
  lv_obj_t* b = lv_obj_create(parent);
  lv_obj_remove_style_all(b);
  lv_obj_remove_flag(b, LV_OBJ_FLAG_SCROLLABLE);
  lv_obj_set_flex_flow(b, flow);
  lv_obj_set_style_pad_row(b, kGap, 0);
  lv_obj_set_style_pad_column(b, 10, 0);
  return b;
}

// Prose that wraps inside the parent's width.
lv_obj_t* prose(lv_obj_t* parent, const char* text, const lv_font_t* font, lv_color_t color) {
  lv_obj_t* l = detail::make_label(parent, text, font, color);
  lv_obj_set_width(l, lv_pct(100));
  lv_label_set_long_mode(l, LV_LABEL_LONG_MODE_WRAP);
  return l;
}

// One line that ends in "..." when it is too long (titles, SSIDs).
void single_line(lv_obj_t* l, const lv_font_t* font) {
  lv_label_set_long_mode(l, LV_LABEL_LONG_MODE_DOTS);
  lv_obj_set_height(l, line_h(font));
}

lv_obj_t* icon_button(lv_obj_t* parent, const char* symbol, lv_event_cb_t cb) {
  lv_obj_t* b = detail::make_button(parent, symbol, cb, nullptr);
  lv_obj_set_size(b, kIconW, g_ui->header_h - 8);
  lv_obj_set_style_pad_all(b, 0, 0);
  return b;
}

// Full-width button whose caption wraps instead of running out of it.
lv_obj_t* wide_button(lv_obj_t* parent, const char* text, lv_event_cb_t cb) {
  lv_obj_t* b = detail::make_button(parent, text, cb, nullptr);
  lv_obj_set_size(b, lv_pct(100), LV_SIZE_CONTENT);
  lv_obj_set_style_pad_ver(b, 6, 0);
  lv_obj_t* l = lv_obj_get_child(b, 0);
  lv_obj_set_width(l, lv_pct(100));
  lv_label_set_long_mode(l, LV_LABEL_LONG_MODE_WRAP);
  lv_obj_set_style_text_align(l, LV_TEXT_ALIGN_CENTER, 0);
  return b;
}

void set_status(const char* text, lv_color_t color) {
  if (g_ui == nullptr || g_ui->status == nullptr) return;
  lv_label_set_text(g_ui->status, text);
  lv_obj_set_style_text_color(g_ui->status, color, 0);
}

// Keyboard shown: the content ends at its top edge and the status (under the
// keyboard anyway) is hidden. Keyboard hidden: the status is readable.
void show_keyboard(bool show) {
  if (g_ui->kb == nullptr) return;
  int32_t h = CYD_SCREEN_H - g_ui->header_h;
  if (show) {
    lv_obj_remove_flag(g_ui->kb, LV_OBJ_FLAG_HIDDEN);
    lv_obj_add_flag(g_ui->status, LV_OBJ_FLAG_HIDDEN);
    h -= g_ui->kb_h;
  } else {
    lv_obj_add_flag(g_ui->kb, LV_OBJ_FLAG_HIDDEN);
    lv_obj_remove_flag(g_ui->status, LV_OBJ_FLAG_HIDDEN);
  }
  lv_obj_set_height(g_ui->content, h);
  detail::layout_check(g_ui->scr, show ? "password" : "password-status", g_ui->kb, nullptr, true);
}

void on_back(lv_event_t*) {
  if (g_ui->view == View::Password) {
    g_ui->pending = View::List;
  } else if (g_ui->view == View::List) {
    g_ui->pending = View::Phone;
  } else if (g_ui->closable) {
    g_ui->close_requested = true;
  }
}

void on_to_device(lv_event_t*) { g_ui->pending = View::List; }
void on_rescan(lv_event_t*) { g_ui->pending = View::List; }

void on_hidden(lv_event_t*) {
  g_ui->hidden = true;
  g_ui->sel_ssid = "";
  g_ui->pending = View::Password;
}

void on_network(lv_event_t* e) {
  size_t idx = (size_t)(uintptr_t)lv_event_get_user_data(e);
  if (idx >= g_ui->found.size()) return;
  g_ui->hidden = false;
  g_ui->sel_ssid = g_ui->found[idx];
  g_ui->pending = View::Password;
}

void start_connect() {
  const ThemeColors& t = ui_theme_colors();
  String ssid = g_ui->hidden ? String(lv_textarea_get_text(g_ui->ta_ssid)) : g_ui->sel_ssid;
  ssid.trim();
  if (ssid.length() == 0) return;
  g_ui->sel_ssid = ssid;
  g_ui->connecting = true;
  g_ui->connect_started = millis();
  set_status(CYD_S_WIFI_CONNECTING, t.text_muted);
  show_keyboard(false);
  detail::wifi_connect(ssid, String(lv_textarea_get_text(g_ui->ta_pass)));
}

// Tapping a field brings the keyboard back (it is hidden while connecting and
// after a failure, so the status stays readable until then).
void on_ta_focus(lv_event_t* e) {
  if (g_ui->kb == nullptr || g_ui->connecting) return;
  lv_obj_t* ta = lv_event_get_target_obj(e);
  lv_keyboard_set_textarea(g_ui->kb, ta);
  if (lv_obj_has_flag(g_ui->kb, LV_OBJ_FLAG_HIDDEN)) show_keyboard(true);
}

void on_kb_ready(lv_event_t*) {
  if (g_ui->connecting) return;
  if (g_ui->hidden && lv_keyboard_get_textarea(g_ui->kb) == g_ui->ta_ssid) {
    lv_keyboard_set_textarea(g_ui->kb, g_ui->ta_pass);
    lv_obj_add_state(g_ui->ta_pass, LV_STATE_FOCUSED);
    lv_obj_remove_state(g_ui->ta_ssid, LV_STATE_FOCUSED);
    return;
  }
  start_connect();
}

void on_kb_cancel(lv_event_t*) { g_ui->pending = View::List; }

// One-line field with compact padding, so two fit above the keyboard on
// 240 px tall screens.
lv_obj_t* make_textarea(lv_obj_t* parent, const char* placeholder, bool password) {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* ta = lv_textarea_create(parent);
  lv_textarea_set_one_line(ta, true);
  lv_textarea_set_placeholder_text(ta, placeholder);
  lv_textarea_set_max_length(ta, password ? 64 : 32);
  if (password) {
    lv_textarea_set_password_mode(ta, true);
    lv_textarea_set_password_bullet(ta, "*");  // ASCII, always in the font
  }
  lv_obj_set_width(ta, lv_pct(100));
  lv_obj_set_style_pad_ver(ta, 4, 0);
  lv_obj_set_style_pad_hor(ta, 8, 0);
  lv_obj_set_style_text_font(ta, t.font_normal, 0);
  lv_obj_set_style_bg_color(ta, t.surface, 0);
  lv_obj_set_style_text_color(ta, t.text, 0);
  lv_obj_set_style_border_color(ta, t.border, 0);
  lv_obj_set_style_border_color(ta, t.focus, LV_STATE_FOCUSED);
  lv_obj_set_style_radius(ta, t.radius, 0);
  lv_obj_add_event_cb(ta, on_ta_focus, LV_EVENT_FOCUSED, nullptr);
  lv_obj_add_event_cb(ta, on_ta_focus, LV_EVENT_CLICKED, nullptr);
  return ta;
}

void clear_view() {
  if (g_ui->kb != nullptr) {  // the keyboard lives on the screen, not in content
    lv_obj_delete(g_ui->kb);
    g_ui->kb = nullptr;
  }
  g_ui->status = nullptr;
  g_ui->list = nullptr;
  g_ui->ta_ssid = nullptr;
  g_ui->ta_pass = nullptr;
  lv_obj_clean(g_ui->content);
  lv_obj_set_height(g_ui->content, CYD_SCREEN_H - g_ui->header_h);
  lv_obj_set_style_pad_ver(g_ui->content, kPad, 0);
  lv_obj_set_flex_flow(g_ui->content, LV_FLEX_FLOW_COLUMN);
  lv_obj_set_flex_align(g_ui->content, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_START);
}

bool phone_fits() { return detail::layout_check(g_ui->content, "phone", nullptr, nullptr, false) == 0; }

// The largest QR side from max down to kQrMin (in kQrStep steps) with which
// the phone view fits; -1 when even the smallest does not.
int32_t fit_qr(lv_obj_t* frame, int32_t max) {
  for (int32_t q = max; q >= kQrMin; q -= kQrStep) {
    lv_obj_set_size(frame, q + 2 * kQrQuiet, q + 2 * kQrQuiet);
    if (phone_fits()) return q;
  }
  return -1;
}

// Landscape: QR code and info side by side, the button below. Portrait: QR
// code on top, info below, the button at the bottom. Too little room: the QR
// code shrinks stepwise to kQrMin, then the hint goes (QR code and title
// explain themselves).
void build_phone() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  bool landscape = CYD_SCREEN_W >= CYD_SCREEN_H;
  lv_obj_set_flex_align(c, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_CENTER, LV_FLEX_ALIGN_CENTER);
  lv_obj_t* top = c;
  if (landscape) {
    top = box(c, LV_FLEX_FLOW_ROW);
    lv_obj_set_width(top, lv_pct(100));
    lv_obj_set_flex_grow(top, 1);
    lv_obj_set_flex_align(top, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_CENTER, LV_FLEX_ALIGN_CENTER);
  }
  lv_obj_t* frame = box(top, LV_FLEX_FLOW_COLUMN);
  lv_obj_set_style_bg_color(frame, lv_color_white(), 0);
  lv_obj_set_style_bg_opa(frame, LV_OPA_COVER, 0);
  lv_obj_set_style_pad_all(frame, kQrQuiet, 0);

  lv_obj_t* info = box(top, LV_FLEX_FLOW_COLUMN);
  lv_obj_set_style_pad_row(info, 4, 0);
  if (landscape) {
    lv_obj_set_flex_grow(info, 1);
    lv_obj_set_height(info, LV_SIZE_CONTENT);
  } else {
    lv_obj_set_width(info, lv_pct(100));
    lv_obj_set_flex_grow(info, 1);
  }
  lv_obj_t* hint = prose(info, CYD_S_WIFI_PHONE_HINT, t.font_small, t.text_muted);
  String net = String(CYD_S_WIFI_NETWORK) + ": " + g_ui->ap_ssid;
  prose(info, net.c_str(), t.font_small, t.text);
  String pw = String(CYD_S_WIFI_PASSWORD) + ": " + g_ui->ap_pass;
  prose(info, pw.c_str(), t.font_mono, t.accent);
  wide_button(c, CYD_S_WIFI_ON_DEVICE, on_to_device);

  int32_t max = landscape ? CYD_SCREEN_W * 2 / 5 - 2 * kQrQuiet : CYD_SCREEN_W - 2 * kPad - 2 * kQrQuiet;
  max = LV_CLAMP(kQrMin, max, kQrMax);
  int32_t q = fit_qr(frame, max);
  if (q < 0) {
    lv_obj_add_flag(hint, LV_OBJ_FLAG_HIDDEN);
    q = fit_qr(frame, max);
    if (q < 0) q = kQrMin;  // layout_check() reports what still does not fit
    lv_obj_set_size(frame, q + 2 * kQrQuiet, q + 2 * kQrQuiet);
  }

  String payload = "WIFI:T:WPA;S:" + g_ui->ap_ssid + ";P:" + g_ui->ap_pass + ";;";
  lv_obj_t* code = lv_qrcode_create(frame);
  lv_qrcode_set_size(code, q);
  lv_qrcode_set_dark_color(code, lv_color_black());
  lv_qrcode_set_light_color(code, lv_color_white());
  lv_qrcode_update(code, payload.c_str(), payload.length());
}

void start_scan() {
  WiFi.scanDelete();
  if (WiFi.scanNetworks(true, true) == WIFI_SCAN_FAILED) {
    g_ui->scanning = false;
    set_status(CYD_S_WIFI_NO_NETWORKS, ui_theme_colors().warning);
    return;
  }
  g_ui->scanning = true;
}

lv_obj_t* add_entry(const char* icon, const char* text, lv_event_cb_t cb, void* user) {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* b = lv_list_add_button(g_ui->list, icon, text);
  lv_obj_set_style_bg_color(b, t.surface, 0);
  lv_obj_set_style_text_color(b, t.text, 0);
  lv_obj_set_style_bg_color(b, t.surface_alt, LV_STATE_PRESSED);
  single_line(lv_obj_get_child(b, -1), t.font_normal);  // the label is the last child
  lv_obj_add_event_cb(b, cb, LV_EVENT_CLICKED, user);
  return b;
}

// The networks of the last scan (strongest first), then "hidden network".
void fill_entries() {
  lv_obj_clean(g_ui->list);
  for (size_t i = 0; i < g_ui->found.size() && i < kMaxNetworks; ++i) {
    add_entry(LV_SYMBOL_WIFI, g_ui->found[i].c_str(), on_network, (void*)(uintptr_t)i);
  }
  add_entry(LV_SYMBOL_EDIT, CYD_S_WIFI_HIDDEN, on_hidden, nullptr);
}

// Status line (while scanning or when nothing was found), the list takes the
// rest; rescan is the header's refresh button.
void build_list() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  g_ui->status = prose(c, CYD_S_WIFI_SCANNING, t.font_small, t.text_muted);
  g_ui->list = lv_list_create(c);
  lv_obj_set_width(g_ui->list, lv_pct(100));
  lv_obj_set_flex_grow(g_ui->list, 1);
  lv_obj_set_scroll_dir(g_ui->list, LV_DIR_VER);
  lv_obj_set_style_bg_color(g_ui->list, t.surface, 0);
  lv_obj_set_style_border_color(g_ui->list, t.border, 0);
  lv_obj_set_style_radius(g_ui->list, t.radius, 0);
  g_ui->found.clear();
  fill_entries();
  start_scan();
}

void fill_list(int n) {
  const ThemeColors& t = ui_theme_colors();
  g_ui->found.clear();
  for (int i = 0; i < n; ++i) {
    String ssid = WiFi.SSID(i);
    if (ssid.length() == 0) continue;
    bool dup = false;
    for (auto& s : g_ui->found) dup = dup || s == ssid;
    if (!dup) g_ui->found.push_back(ssid);  // scan results are sorted by RSSI
  }
  WiFi.scanDelete();
  if (g_ui->list == nullptr) return;
  fill_entries();
  if (g_ui->found.empty()) {
    set_status(CYD_S_WIFI_NO_NETWORKS, t.warning);
  } else {
    lv_obj_add_flag(g_ui->status, LV_OBJ_FLAG_HIDDEN);  // the list says it all
  }
  detail::layout_check(g_ui->scr, "list", nullptr, g_ui->list, true);
}

// The SSID is the header title; the keyboard takes at most half the screen,
// less when the fields need the room.
void build_password() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  lv_obj_set_style_pad_ver(c, kGap, 0);
  if (g_ui->hidden) g_ui->ta_ssid = make_textarea(c, CYD_S_WIFI_SSID, false);
  g_ui->ta_pass = make_textarea(c, CYD_S_WIFI_PASSWORD, true);
  g_ui->status = prose(c, "", t.font_small, t.text_muted);

  lv_obj_update_layout(g_ui->scr);
  int32_t need = 2 * kGap + lv_obj_get_height(g_ui->ta_pass);
  if (g_ui->ta_ssid != nullptr) need += kGap + lv_obj_get_height(g_ui->ta_ssid);
  g_ui->kb_h = LV_MIN(CYD_SCREEN_H / 2, CYD_SCREEN_H - g_ui->header_h - need);

  g_ui->kb = lv_keyboard_create(g_ui->scr);
  lv_obj_set_size(g_ui->kb, CYD_SCREEN_W, g_ui->kb_h);
  lv_obj_align(g_ui->kb, LV_ALIGN_BOTTOM_MID, 0, 0);
  lv_obj_set_style_bg_color(g_ui->kb, t.surface_alt, 0);  // white keys of light themes stay visible
  // Four key rows: fall back to the small font when the normal one is taller than a key.
  bool roomy = g_ui->kb_h / 4 >= line_h(t.font_normal) + 6;
  lv_obj_set_style_text_font(g_ui->kb, roomy ? t.font_normal : t.font_small, 0);
  lv_obj_add_event_cb(g_ui->kb, on_kb_ready, LV_EVENT_READY, nullptr);
  lv_obj_add_event_cb(g_ui->kb, on_kb_cancel, LV_EVENT_CANCEL, nullptr);
  lv_obj_t* first = g_ui->hidden ? g_ui->ta_ssid : g_ui->ta_pass;
  lv_keyboard_set_textarea(g_ui->kb, first);
  lv_obj_add_state(first, LV_STATE_FOCUSED);
  show_keyboard(true);
}

// Header: title (dots), [rescan], back. The title gets what the buttons leave.
void layout_header(View v) {
  bool back = v != View::Phone || g_ui->closable;
  bool rescan = v == View::List;
  lv_obj_set_flag(g_ui->back_btn, LV_OBJ_FLAG_HIDDEN, !back);
  lv_obj_set_flag(g_ui->rescan_btn, LV_OBJ_FLAG_HIDDEN, !rescan);
  lv_label_set_text(lv_obj_get_child(g_ui->back_btn, 0), v == View::Phone ? LV_SYMBOL_CLOSE : LV_SYMBOL_LEFT);
  const char* title = CYD_S_WIFI_SETUP_TITLE;
  if (v == View::Password) title = g_ui->hidden ? CYD_S_WIFI_HIDDEN : g_ui->sel_ssid.c_str();
  lv_label_set_text(g_ui->title, title);
  int32_t w = CYD_SCREEN_W - 10 - 4 - 6;
  if (back) w -= kIconW;
  if (rescan) w -= kIconW + 4;
  lv_obj_set_width(g_ui->title, w);
}

void apply_view(View v) {
  clear_view();
  g_ui->view = v;
  layout_header(v);
  switch (v) {
    case View::Phone:
      build_phone();
      detail::layout_check(g_ui->scr, "phone", nullptr, nullptr, true);
      break;
    case View::List:
      build_list();
      detail::layout_check(g_ui->scr, "list", nullptr, g_ui->list, true);
      break;
    case View::Password:
      build_password();  // checked by show_keyboard()
      break;
    default:
      break;
  }
}

void finish_success() {
  const ThemeColors& t = ui_theme_colors();
  detail::wifi_save_credentials(WiFi.SSID(), WiFi.psk());
  detail::portal_stop();
  if (g_ui->kb != nullptr) show_keyboard(false);
  if (g_ui->status != nullptr) set_status(CYD_S_WIFI_SAVED, t.success);
  g_ui->close_at = millis() + 800;
}

}  // namespace

namespace detail {

bool wifi_ui_active() { return g_ui != nullptr; }

void wifi_ui_open(bool closable) {
  if (g_ui != nullptr) return;
  const ThemeColors& t = ui_theme_colors();
  g_ui = new SetupUi();
  g_ui->closable = closable;
  g_ui->prev = lv_screen_active();
  g_ui->header_h = LV_MAX(36, line_h(t.font_normal) + 12);
  wifi_set_state(wifi::State::Setup);
  WiFi.mode(WIFI_AP_STA);
  portal_start(&g_ui->ap_ssid, &g_ui->ap_pass);

  g_ui->scr = make_screen();
  lv_obj_t* header = lv_obj_create(g_ui->scr);
  lv_obj_remove_style_all(header);
  lv_obj_remove_flag(header, LV_OBJ_FLAG_SCROLLABLE);
  lv_obj_set_size(header, CYD_SCREEN_W, g_ui->header_h);
  lv_obj_set_style_bg_color(header, t.surface, 0);
  lv_obj_set_style_bg_opa(header, LV_OPA_COVER, 0);
  g_ui->title = make_label(header, CYD_S_WIFI_SETUP_TITLE, t.font_normal, t.text);
  single_line(g_ui->title, t.font_normal);
  lv_obj_align(g_ui->title, LV_ALIGN_LEFT_MID, 10, 0);
  g_ui->back_btn = icon_button(header, LV_SYMBOL_CLOSE, on_back);
  lv_obj_align(g_ui->back_btn, LV_ALIGN_RIGHT_MID, -4, 0);
  g_ui->rescan_btn = icon_button(header, LV_SYMBOL_REFRESH, on_rescan);
  lv_obj_align_to(g_ui->rescan_btn, g_ui->back_btn, LV_ALIGN_OUT_LEFT_MID, -4, 0);

  g_ui->content = lv_obj_create(g_ui->scr);
  lv_obj_remove_style_all(g_ui->content);
  lv_obj_remove_flag(g_ui->content, LV_OBJ_FLAG_SCROLLABLE);
  lv_obj_set_size(g_ui->content, CYD_SCREEN_W, CYD_SCREEN_H - g_ui->header_h);
  lv_obj_set_pos(g_ui->content, 0, g_ui->header_h);
  lv_obj_set_style_pad_all(g_ui->content, kPad, 0);
  lv_obj_set_style_pad_row(g_ui->content, kGap, 0);
  lv_obj_set_style_pad_column(g_ui->content, 10, 0);

  lv_screen_load(g_ui->scr);
  g_ui->pending = View::Phone;
  log::info("wifi: setup screen open (%s)", closable ? "closable" : "required");
}

void wifi_ui_close() {
  if (g_ui == nullptr) return;
  portal_stop();
  WiFi.scanDelete();
  lv_obj_t* scr = g_ui->scr;
  lv_screen_load(g_ui->prev);
  lv_obj_delete(scr);
  delete g_ui;
  g_ui = nullptr;
  if (WiFi.status() == WL_CONNECTED) {
    wifi_set_state(wifi::State::Connected);
    time_begin();
    return;
  }
  // A failed attempt must not replace the working network: go back to the stored
  // one (wifi_connect() enters Connecting now that the setup screen is gone).
  String ssid, pass;
  if (wifi_load_credentials(&ssid, &pass)) {
    wifi_connect(ssid, pass);
  } else {
    wifi_set_state(wifi::State::Offline);
  }
}

void wifi_ui_loop() {
  if (g_ui == nullptr) return;
  if (g_ui->close_at != 0 && (int32_t)(millis() - g_ui->close_at) >= 0) {
    wifi_ui_close();
    return;
  }
  if (g_ui->close_requested) {
    wifi_ui_close();
    return;
  }
  if (g_ui->pending != View::None) {
    View v = g_ui->pending;
    g_ui->pending = View::None;
    apply_view(v);
  }
  if (portal_loop()) {
    finish_success();
    return;
  }
  if (!portal_active() && !g_ui->connecting && g_ui->close_at == 0 &&
      (int32_t)(millis() - g_ui->portal_retry_at) >= 0) {
    // The portal ended on its own: restart it, or the QR code would be dead. The
    // new password makes the phone view show a new QR code.
    g_ui->portal_retry_at = millis() + 3000;
    portal_start(&g_ui->ap_ssid, &g_ui->ap_pass);
    if (g_ui->view == View::Phone && g_ui->pending == View::None) g_ui->pending = View::Phone;
  }
  if (g_ui->scanning) {
    int n = WiFi.scanComplete();
    if (n >= 0) {
      g_ui->scanning = false;
      fill_list(n);
    } else if (n == WIFI_SCAN_FAILED) {
      g_ui->scanning = false;
      fill_list(0);
    }
  }
  if (g_ui->connecting) {
    wl_status_t st = WiFi.status();
    if (st == WL_CONNECTED) {
      g_ui->connecting = false;
      finish_success();
    } else if (millis() - g_ui->connect_started > CYD_WIFI_CONNECT_MS ||
               ((st == WL_CONNECT_FAILED || st == WL_NO_SSID_AVAIL) && millis() - g_ui->connect_started > 3000)) {
      // The error stays readable (keyboard hidden) until the user taps a field.
      g_ui->connecting = false;
      WiFi.disconnect(false, false);
      set_status(CYD_S_WIFI_FAILED, ui_theme_colors().danger);
    }
  }
}

#if CYD_LAYOUT_SELFTEST
bool wifi_ui_probe(WifiUiProbe* out) {
  *out = WifiUiProbe{};
  if (g_ui == nullptr) return false;
  out->scr = g_ui->scr;
  out->content = g_ui->content;
  out->list = g_ui->list;
  out->kb = g_ui->kb;
  out->status = g_ui->status;
  out->ta_ssid = g_ui->ta_ssid;
  out->ta_pass = g_ui->ta_pass;
  // A requested view is built by the next wifi_ui_loop(): until then report none.
  if (g_ui->pending == View::None) {
    out->view = g_ui->view == View::Phone ? 1 : g_ui->view == View::List ? 2 : g_ui->view == View::Password ? 3 : 0;
  }
  out->scanning = g_ui->scanning;
  out->connecting = g_ui->connecting;
  out->closing = g_ui->close_at != 0;
  return true;
}

void wifi_ui_test(WifiUiTest action, const char* ssid) {
  if (g_ui == nullptr) return;
  switch (action) {
    case WifiUiTest::ToDevice:
      on_to_device(nullptr);
      break;
    case WifiUiTest::Hidden:
      on_hidden(nullptr);
      break;
    case WifiUiTest::Network:  // on_network() for any SSID, also one the scan did not list
      g_ui->hidden = false;
      g_ui->sel_ssid = ssid != nullptr ? ssid : "";
      g_ui->pending = View::Password;
      break;
    case WifiUiTest::Close:
      g_ui->close_requested = true;
      break;
  }
}
#endif

}  // namespace detail
}  // namespace cyd
