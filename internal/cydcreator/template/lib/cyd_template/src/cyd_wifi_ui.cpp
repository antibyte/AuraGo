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
  bool sel_listed = false;    // the selected network came from the scan list
  bool sel_locked = false;    // ... and the scan saw it encrypted
  bool phone_failed = false;  // the last phone-path join failed (the phone view says so)
  bool connecting = false;
  uint32_t ip_gen = 0;     // wifi_ip_generation() when the attempt started
  uint32_t fail_gen = 0;   // wifi_fail_generation() when the attempt started
  uint32_t seen_fail = 0;  // wifi_fail_generation() last looked at
  uint32_t no_ap = 0;      // NO_AP_FOUND joins of this attempt
  uint32_t connect_started = 0;
  uint32_t scan_started = 0;
  uint32_t close_at = 0;
  uint32_t portal_retry_at = 0;
  String ap_ssid, ap_pass, sel_ssid;
  std::vector<String> found;
  std::vector<bool> found_locked;  // per found[] entry: not WIFI_AUTH_OPEN
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
constexpr uint32_t kMaxAttempts = 3;  // failed joins (the driver retries itself) before an attempt fails
constexpr uint32_t kScanMs = 15000;  // a scan that has not reported by then found nothing

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
  g_ui->sel_listed = false;
  g_ui->sel_locked = false;
  g_ui->sel_ssid = "";
  g_ui->pending = View::Password;
}

// A network by SSID with what the scan said about it; one the scan did not
// list counts as encrypted, so the password view asks for a password.
void select_network(const String& ssid) {
  g_ui->hidden = false;
  g_ui->sel_ssid = ssid;
  g_ui->sel_listed = false;
  g_ui->sel_locked = true;
  for (size_t i = 0; i < g_ui->found.size(); ++i) {
    if (g_ui->found[i] == ssid) {
      g_ui->sel_listed = true;
      g_ui->sel_locked = g_ui->found_locked[i];
    }
  }
  g_ui->pending = View::Password;
}

void on_network(lv_event_t* e) {
  size_t idx = (size_t)(uintptr_t)lv_event_get_user_data(e);
  if (idx < g_ui->found.size()) select_network(g_ui->found[idx]);
}

// WPA2 passphrases have 8..63 characters, a raw key has 64 hex digits; an open
// network (hidden or not) has none.
bool password_ok(const String& p, bool required) {
  size_t n = p.length();
  if (n == 0) return !required;
  if (n >= 8 && n <= 63) return true;
  if (n != 64) return false;
  for (size_t i = 0; i < n; ++i) {
    if (!isxdigit((unsigned char)p[i])) return false;
  }
  return true;
}

void finish_success();

void start_connect() {
  const ThemeColors& t = ui_theme_colors();
  String ssid = g_ui->hidden ? String(lv_textarea_get_text(g_ui->ta_ssid)) : g_ui->sel_ssid;
  ssid.trim();
  if (ssid.length() == 0) return;
  String pass = g_ui->ta_pass != nullptr ? String(lv_textarea_get_text(g_ui->ta_pass)) : String();
  if (!password_ok(pass, g_ui->sel_locked && !g_ui->hidden)) {
    show_keyboard(false);  // the reason stays readable until a field is tapped
    set_status(CYD_S_WIFI_PASSWORD_LENGTH, t.warning);
    return;
  }
  g_ui->sel_ssid = ssid;
  g_ui->phone_failed = false;
  set_status(CYD_S_WIFI_CONNECTING, t.text_muted);
  show_keyboard(false);
  if (WiFi.status() == WL_CONNECTED && WiFi.SSID() == ssid && WiFi.psk() == pass) {
    finish_success();  // already on this network: a new begin() would only drop and rejoin it
    return;
  }
  // Join as a plain station. The radio has one channel: next to the SoftAP
  // on another channel first joins timed out (AUTH_EXPIRE, NO_AP_FOUND).
  // stop_attempt() brings the portal back if the attempt fails or is left.
  detail::portal_stop();
  WiFi.setAutoReconnect(true);  // the driver retries transient failures; connect_failed() ends that
  g_ui->connecting = true;
  g_ui->connect_started = millis();
  g_ui->ip_gen = detail::wifi_ip_generation();
  g_ui->fail_gen = detail::wifi_fail_generation();
  g_ui->seen_fail = g_ui->fail_gen;
  g_ui->no_ap = 0;
  detail::wifi_connect(ssid, pass);
}

// Starts the portal again (new password, so the phone view shows a new QR
// code). Auto-reconnect goes back on: WiFiManager's phone-path join tries only
// once and relies on the driver to retry a first join that times out.
void restart_portal() {
  detail::portal_start(&g_ui->ap_ssid, &g_ui->ap_pass);
  WiFi.setAutoReconnect(true);
  g_ui->portal_retry_at = millis() + 3000;
}

// Ends an on-device attempt that has no link yet: the driver stops joining in
// the background (auto-reconnect would carry on while the screen says the
// attempt ended) and the portal comes back right away.
void stop_attempt() {
  g_ui->connecting = false;
  WiFi.setAutoReconnect(false);
  WiFi.disconnect(false, false);
  restart_portal();
}

// The error stays readable (keyboard hidden) until the user taps a field.
void connect_failed() {
  log::warn("wifi: joining \"%s\" failed (status %d after %lu ms)", g_ui->sel_ssid.c_str(), (int)WiFi.status(),
            (unsigned long)(millis() - g_ui->connect_started));
  stop_attempt();
  set_status(CYD_S_WIFI_FAILED, ui_theme_colors().danger);
}

// The keyboard types into ta, and only ta looks focused (no LVGL group tracks
// the focus of these fields, so nothing clears the other one's state).
void focus_field(lv_obj_t* ta) {
  lv_keyboard_set_textarea(g_ui->kb, ta);
  for (lv_obj_t* f : {g_ui->ta_ssid, g_ui->ta_pass}) {
    if (f != nullptr && f != ta) lv_obj_remove_state(f, LV_STATE_FOCUSED);
  }
  lv_obj_add_state(ta, LV_STATE_FOCUSED);
}

// Tapping a field brings the keyboard back (it is hidden while connecting and
// after a failure, so the status stays readable until then).
void on_ta_focus(lv_event_t* e) {
  if (g_ui->kb == nullptr || g_ui->connecting) return;
  focus_field(lv_event_get_target_obj(e));
  if (lv_obj_has_flag(g_ui->kb, LV_OBJ_FLAG_HIDDEN)) show_keyboard(true);
}

void on_kb_ready(lv_event_t*) {
  if (g_ui->connecting) return;
  if (g_ui->hidden && lv_keyboard_get_textarea(g_ui->kb) == g_ui->ta_ssid) {
    focus_field(g_ui->ta_pass);
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

// Only placement findings: what a smaller QR code or a dropped hint can fix.
bool phone_fits() { return detail::layout_check(g_ui->content, "phone", nullptr, nullptr, false, true) == 0; }

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
// explain themselves), then the network and the password lines (the QR code
// carries both); a failure message always stays.
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
  // A failed phone-path join stays visible here until the next attempt.
  if (g_ui->phone_failed) g_ui->status = prose(info, CYD_S_WIFI_FAILED, t.font_small, t.danger);
  lv_obj_t* hint = prose(info, CYD_S_WIFI_PHONE_HINT, t.font_small, t.text_muted);
  String net = String(CYD_S_WIFI_NETWORK) + ": " + g_ui->ap_ssid;
  lv_obj_t* net_line = prose(info, net.c_str(), t.font_small, t.text);
  String pw = String(CYD_S_WIFI_PASSWORD) + ": " + g_ui->ap_pass;
  lv_obj_t* pw_line = prose(info, pw.c_str(), t.font_mono, t.accent);
  wide_button(c, CYD_S_WIFI_ON_DEVICE, on_to_device);

  int32_t max = landscape ? CYD_SCREEN_W * 2 / 5 - 2 * kQrQuiet : CYD_SCREEN_W - 2 * kPad - 2 * kQrQuiet;
  max = LV_CLAMP(kQrMin, max, kQrMax);
  int32_t q = fit_qr(frame, max);
  for (lv_obj_t* optional : {hint, net_line, pw_line}) {
    if (q >= 0) break;
    lv_obj_add_flag(optional, LV_OBJ_FLAG_HIDDEN);
    q = fit_qr(frame, max);
  }
  if (q < 0) {
    q = kQrMin;  // layout_check() reports what still does not fit
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
  // A scan that outlasted scanComplete()'s 6 s still delivers its results,
  // also into a rebuilt list; a second scan would fail and drop them.
  if (g_ui->scanning) return;
  WiFi.scanDelete();
  if (WiFi.scanNetworks(true, true) == WIFI_SCAN_FAILED) {
    g_ui->scanning = false;
    set_status(CYD_S_WIFI_NO_NETWORKS, ui_theme_colors().warning);
    return;
  }
  g_ui->scanning = true;
  g_ui->scan_started = millis();
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
  if (!g_ui->scanning) {  // a list rebuilt during a scan keeps waiting for it
    g_ui->found.clear();
    g_ui->found_locked.clear();
  }
  fill_entries();
  start_scan();
}

void fill_list(int n) {
  const ThemeColors& t = ui_theme_colors();
  g_ui->found.clear();
  g_ui->found_locked.clear();
  for (int i = 0; i < n; ++i) {
    String ssid = WiFi.SSID(i);
    if (ssid.length() == 0) continue;
    bool dup = false;
    for (auto& s : g_ui->found) dup = dup || s == ssid;
    if (dup) continue;  // scan results are sorted by RSSI: the strongest entry stays
    g_ui->found.push_back(ssid);
    g_ui->found_locked.push_back(WiFi.encryptionType(i) != WIFI_AUTH_OPEN);
  }
  WiFi.scanDelete();
  log::info("wifi: scan found %u networks", (unsigned)g_ui->found.size());
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
  if (!g_ui->hidden && !g_ui->sel_locked) {  // open network: nothing to type, join right away
    g_ui->status = prose(c, "", t.font_small, t.text_muted);
    start_connect();
    return;
  }
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
  focus_field(g_ui->hidden ? g_ui->ta_ssid : g_ui->ta_pass);
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
  // Leaving the password view cancels its attempt; the portal is back before
  // the next view is built (its QR code, its scan next to the SoftAP).
  if (g_ui->connecting) {
    log::info("wifi: joining \"%s\" cancelled", g_ui->sel_ssid.c_str());
    stop_attempt();
  }
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
  bool joined = g_ui->close_at != 0;  // closing after a successful setup
  portal_stop();
  WiFi.scanDelete();
  WiFi.setAutoReconnect(true);
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
  if (joined) {
    // The new network was up a moment ago and the driver is rejoining it on
    // its own; a new begin() would only restart that.
    wifi_await_link();
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
  PortalEvent portal = portal_loop();
  if (portal == PortalEvent::Joined) {
    finish_success();
    return;
  }
  if (portal == PortalEvent::JoinFailed) {
    g_ui->phone_failed = true;
    if (g_ui->view == View::Phone && g_ui->pending == View::None) g_ui->pending = View::Phone;
  }
  if (!portal_active() && !g_ui->connecting && g_ui->close_at == 0 &&
      (int32_t)(millis() - g_ui->portal_retry_at) >= 0) {
    // The portal ended on its own: restart it, or the QR code would be dead.
    restart_portal();
    if (g_ui->view == View::Phone && g_ui->pending == View::None) g_ui->pending = View::Phone;
  }
  if (g_ui->scanning) {
    // Arduino-ESP32 2.0.17 scanComplete() says WIFI_SCAN_FAILED once a scan
    // runs longer than 20 x 300 ms, which scans next to the SoftAP and a
    // station link regularly do; their results still arrive, so keep polling.
    int n = WiFi.scanComplete();
    if (n >= 0) {
      g_ui->scanning = false;
      fill_list(n);
    } else if (n == WIFI_SCAN_FAILED && millis() - g_ui->scan_started > kScanMs) {
      g_ui->scanning = false;
      fill_list(0);
    }
  }
  if (g_ui->connecting) {
    // Success needs an address handed out after begin(): WiFi.status() can
    // still say WL_CONNECTED for the link begin() is about to drop.
    wl_status_t st = WiFi.status();
    uint32_t elapsed = millis() - g_ui->connect_started;
    uint32_t fails = detail::wifi_fail_generation();
    uint32_t failed_joins = fails - g_ui->fail_gen;
    if (fails != g_ui->seen_fail) {
      g_ui->seen_fail = fails;
      if (detail::wifi_last_fail_reason() == WIFI_REASON_NO_AP_FOUND) ++g_ui->no_ap;
    }
    if (st == WL_CONNECTED && detail::wifi_ip_generation() != g_ui->ip_gen) {
      g_ui->connecting = false;
      log::info("wifi: joined \"%s\" in %lu ms", g_ui->sel_ssid.c_str(), (unsigned long)elapsed);
      finish_success();
    } else if (elapsed > CYD_WIFI_CONNECT_MS || (st == WL_CONNECT_FAILED && elapsed > 3000) ||
               failed_joins >= kMaxAttempts || g_ui->no_ap >= (g_ui->sel_listed ? 2u : 1u)) {
      // Wrong password (the driver gives up), timeout, or the attempt cap that
      // keeps a wrong password from hammering the access point. With the
      // SoftAP off "network not found" is real: once for a typed SSID, twice
      // for one the scan has just listed (a missed beacon).
      connect_failed();
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
  out->portal = portal_active();
  out->networks = g_ui->found.size();
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
      select_network(ssid != nullptr ? ssid : "");
      break;
    case WifiUiTest::PhoneFailed:  // as after a failed phone-path join
      g_ui->phone_failed = true;
      g_ui->pending = View::Phone;
      break;
    case WifiUiTest::Back:
      on_back(nullptr);
      break;
    case WifiUiTest::Close:
      g_ui->close_requested = true;
      break;
  }
}
#endif

}  // namespace detail
}  // namespace cyd
