// cyd_wifi_ui.cpp - the themed Wi-Fi setup screen with two paths:
//   phone (default): captive portal, QR code WIFI:T:WPA;S:<ap>;P:<pass>;;
//   on device: LVGL scan list, hidden-SSID entry and on-screen keyboard.
// View switches and closing are deferred to wifi_ui_loop() so no widget is
// deleted inside its own event callback.
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
  lv_obj_t* content = nullptr;
  lv_obj_t* status = nullptr;
  lv_obj_t* list = nullptr;
  lv_obj_t* ta_ssid = nullptr;
  lv_obj_t* ta_pass = nullptr;
  lv_obj_t* kb = nullptr;
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

const int kHeaderH = 36;

void set_status(const char* text, lv_color_t color) {
  if (g_ui == nullptr || g_ui->status == nullptr) return;
  lv_label_set_text(g_ui->status, text);
  lv_obj_set_style_text_color(g_ui->status, color, 0);
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
void on_to_phone(lv_event_t*) { g_ui->pending = View::Phone; }
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
  detail::wifi_connect(ssid, String(lv_textarea_get_text(g_ui->ta_pass)));
}

void on_ta_focus(lv_event_t* e) {
  lv_obj_t* ta = lv_event_get_target_obj(e);
  if (g_ui->kb != nullptr) lv_keyboard_set_textarea(g_ui->kb, ta);
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
  lv_obj_set_style_bg_color(ta, t.surface, 0);
  lv_obj_set_style_text_color(ta, t.text, 0);
  lv_obj_set_style_border_color(ta, t.border, 0);
  lv_obj_set_style_border_color(ta, t.focus, LV_STATE_FOCUSED);
  lv_obj_set_style_radius(ta, t.radius, 0);
  lv_obj_add_event_cb(ta, on_ta_focus, LV_EVENT_FOCUSED, nullptr);
  lv_obj_add_event_cb(ta, on_ta_focus, LV_EVENT_CLICKED, nullptr);
  return ta;
}

lv_obj_t* column(lv_obj_t* parent) {
  lv_obj_t* c = lv_obj_create(parent);
  lv_obj_remove_style_all(c);
  lv_obj_set_flex_flow(c, LV_FLEX_FLOW_COLUMN);
  lv_obj_set_style_pad_row(c, 6, 0);
  return c;
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
  lv_obj_set_flex_flow(g_ui->content, LV_FLEX_FLOW_COLUMN);
  lv_obj_set_flex_align(g_ui->content, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_START);
}

void build_phone() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  bool landscape = CYD_SCREEN_W >= CYD_SCREEN_H;
  lv_obj_set_flex_flow(c, landscape ? LV_FLEX_FLOW_ROW : LV_FLEX_FLOW_COLUMN);
  lv_obj_set_flex_align(c, LV_FLEX_ALIGN_START, LV_FLEX_ALIGN_CENTER, LV_FLEX_ALIGN_CENTER);
  int avail_w = CYD_SCREEN_W - 16;
  int avail_h = CYD_SCREEN_H - kHeaderH - 16;
  int qr = landscape ? (avail_h < avail_w / 2 ? avail_h : avail_w / 2) : (avail_w < avail_h / 2 ? avail_w : avail_h / 2);
  qr -= 12;
  if (qr > 220) qr = 220;

  String payload = "WIFI:T:WPA;S:" + g_ui->ap_ssid + ";P:" + g_ui->ap_pass + ";;";
  lv_obj_t* code = lv_qrcode_create(c);
  lv_qrcode_set_size(code, qr);
  lv_qrcode_set_dark_color(code, lv_color_black());
  lv_qrcode_set_light_color(code, lv_color_white());
  lv_qrcode_update(code, payload.c_str(), payload.length());
  lv_obj_set_style_border_color(code, lv_color_white(), 0);
  lv_obj_set_style_border_width(code, 4, 0);  // quiet zone on dark themes

  lv_obj_t* info = column(c);
  lv_obj_set_width(info, landscape ? avail_w - qr - 20 : avail_w);
  lv_obj_set_height(info, LV_SIZE_CONTENT);
  lv_obj_t* hint = detail::make_label(info, CYD_S_WIFI_PHONE_HINT, t.font_small, t.text_muted);
  lv_obj_set_width(hint, lv_pct(100));
  lv_label_set_long_mode(hint, LV_LABEL_LONG_MODE_WRAP);
  String net = String(CYD_S_WIFI_NETWORK) + ": " + g_ui->ap_ssid;
  String pw = String(CYD_S_WIFI_PASSWORD) + ": " + g_ui->ap_pass;
  detail::make_label(info, net.c_str(), t.font_small, t.text);
  detail::make_label(info, pw.c_str(), t.font_mono, t.accent);
  lv_obj_t* b = detail::make_button(info, CYD_S_WIFI_ON_DEVICE, on_to_device, nullptr);
  lv_obj_set_width(b, lv_pct(100));
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

void build_list() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  g_ui->status = detail::make_label(c, CYD_S_WIFI_SCANNING, t.font_small, t.text_muted);
  g_ui->list = lv_list_create(c);
  lv_obj_set_width(g_ui->list, lv_pct(100));
  lv_obj_set_flex_grow(g_ui->list, 1);
  lv_obj_set_style_bg_color(g_ui->list, t.surface, 0);
  lv_obj_set_style_border_color(g_ui->list, t.border, 0);
  lv_obj_set_style_radius(g_ui->list, t.radius, 0);
  lv_obj_t* row = lv_obj_create(c);
  lv_obj_remove_style_all(row);
  lv_obj_set_size(row, lv_pct(100), LV_SIZE_CONTENT);
  lv_obj_set_flex_flow(row, LV_FLEX_FLOW_ROW_WRAP);
  lv_obj_set_style_pad_column(row, 6, 0);
  lv_obj_set_style_pad_row(row, 6, 0);
  detail::make_button(row, CYD_S_WIFI_RESCAN, on_rescan, nullptr);
  detail::make_button(row, CYD_S_WIFI_HIDDEN, on_hidden, nullptr);
  detail::make_button(row, CYD_S_WIFI_WITH_PHONE, on_to_phone, nullptr);
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
  lv_obj_clean(g_ui->list);
  for (size_t i = 0; i < g_ui->found.size() && i < 20; ++i) {
    lv_obj_t* b = lv_list_add_button(g_ui->list, LV_SYMBOL_WIFI, g_ui->found[i].c_str());
    lv_obj_set_style_bg_color(b, t.surface, 0);
    lv_obj_set_style_text_color(b, t.text, 0);
    lv_obj_set_style_bg_color(b, t.surface_alt, LV_STATE_PRESSED);
    lv_obj_add_event_cb(b, on_network, LV_EVENT_CLICKED, (void*)(uintptr_t)i);
  }
  if (g_ui->found.empty()) {
    set_status(CYD_S_WIFI_NO_NETWORKS, t.warning);
  } else {
    set_status(CYD_S_WIFI_SETUP_TITLE, t.text_muted);
  }
}

void build_password() {
  const ThemeColors& t = ui_theme_colors();
  lv_obj_t* c = g_ui->content;
  if (g_ui->hidden) {
    g_ui->ta_ssid = make_textarea(c, CYD_S_WIFI_SSID, false);
  } else {
    lv_obj_t* l = detail::make_label(c, g_ui->sel_ssid.c_str(), t.font_normal, t.text);
    lv_label_set_long_mode(l, LV_LABEL_LONG_MODE_DOTS);
    lv_obj_set_width(l, lv_pct(100));
  }
  g_ui->ta_pass = make_textarea(c, CYD_S_WIFI_PASSWORD, true);
  g_ui->status = detail::make_label(c, "", t.font_small, t.text_muted);
  lv_obj_set_width(g_ui->status, lv_pct(100));
  lv_label_set_long_mode(g_ui->status, LV_LABEL_LONG_MODE_WRAP);

  g_ui->kb = lv_keyboard_create(g_ui->scr);
  lv_obj_set_size(g_ui->kb, CYD_SCREEN_W, CYD_SCREEN_H / 2);
  lv_obj_align(g_ui->kb, LV_ALIGN_BOTTOM_MID, 0, 0);
  lv_obj_set_style_bg_color(g_ui->kb, t.surface, 0);
  lv_obj_set_style_text_font(g_ui->kb, t.font_normal, 0);
  lv_obj_add_event_cb(g_ui->kb, on_kb_ready, LV_EVENT_READY, nullptr);
  lv_obj_add_event_cb(g_ui->kb, on_kb_cancel, LV_EVENT_CANCEL, nullptr);
  lv_obj_t* first = g_ui->hidden ? g_ui->ta_ssid : g_ui->ta_pass;
  lv_keyboard_set_textarea(g_ui->kb, first);
  lv_obj_add_state(first, LV_STATE_FOCUSED);
}

void apply_view(View v) {
  clear_view();
  g_ui->view = v;
  bool back = v != View::Phone || g_ui->closable;
  if (back) {
    lv_obj_remove_flag(g_ui->back_btn, LV_OBJ_FLAG_HIDDEN);
  } else {
    lv_obj_add_flag(g_ui->back_btn, LV_OBJ_FLAG_HIDDEN);
  }
  lv_label_set_text(lv_obj_get_child(g_ui->back_btn, 0), v == View::Phone ? LV_SYMBOL_CLOSE : LV_SYMBOL_LEFT);
  switch (v) {
    case View::Phone:
      build_phone();
      break;
    case View::List:
      build_list();
      break;
    case View::Password:
      build_password();
      break;
    default:
      break;
  }
}

void finish_success() {
  const ThemeColors& t = ui_theme_colors();
  detail::wifi_save_credentials(WiFi.SSID(), WiFi.psk());
  detail::portal_stop();
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
  wifi_set_state(wifi::State::Setup);
  WiFi.mode(WIFI_AP_STA);
  portal_start(&g_ui->ap_ssid, &g_ui->ap_pass);

  g_ui->scr = make_screen();
  lv_obj_t* header = lv_obj_create(g_ui->scr);
  lv_obj_remove_style_all(header);
  lv_obj_set_size(header, CYD_SCREEN_W, kHeaderH);
  lv_obj_set_style_bg_color(header, t.surface, 0);
  lv_obj_set_style_bg_opa(header, LV_OPA_COVER, 0);
  g_ui->title = make_label(header, CYD_S_WIFI_SETUP_TITLE, t.font_normal, t.text);
  lv_obj_align(g_ui->title, LV_ALIGN_LEFT_MID, 10, 0);
  g_ui->back_btn = make_button(header, LV_SYMBOL_CLOSE, on_back, nullptr);
  lv_obj_set_size(g_ui->back_btn, 44, kHeaderH - 6);
  lv_obj_align(g_ui->back_btn, LV_ALIGN_RIGHT_MID, -4, 0);

  g_ui->content = lv_obj_create(g_ui->scr);
  lv_obj_remove_style_all(g_ui->content);
  lv_obj_set_size(g_ui->content, CYD_SCREEN_W, CYD_SCREEN_H - kHeaderH);
  lv_obj_set_pos(g_ui->content, 0, kHeaderH);
  lv_obj_set_style_pad_all(g_ui->content, 8, 0);
  lv_obj_set_style_pad_row(g_ui->content, 6, 0);
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
      g_ui->connecting = false;
      WiFi.disconnect(false, false);
      set_status(CYD_S_WIFI_FAILED, ui_theme_colors().danger);
    }
  }
}

}  // namespace detail
}  // namespace cyd
