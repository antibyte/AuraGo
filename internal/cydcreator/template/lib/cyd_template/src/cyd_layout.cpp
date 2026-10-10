// cyd_layout.cpp - layout checker for the runtime's own screens and, only in
// builds with CYD_LAYOUT_SELFTEST 1, the on-device self-test of the Wi-Fi setup
// screen (views, keyboard, failure status, reconnect after a successful setup).
#include "cyd_private.h"

#include <WiFi.h>

namespace cyd {
namespace {

struct Check {
  const char* view;
  const lv_obj_t* overlay;
  const lv_obj_t* scroller;
  lv_area_t visible;
  bool log;
  int problems;
};

const char* kind_of(const lv_obj_t* o) {
  if (lv_obj_check_type(o, &lv_label_class)) return "label";
  if (lv_obj_check_type(o, &lv_list_button_class)) return "list item";
  if (lv_obj_check_type(o, &lv_button_class)) return "button";
  if (lv_obj_check_type(o, &lv_textarea_class)) return "textarea";
  if (lv_obj_check_type(o, &lv_keyboard_class)) return "keyboard";
  if (lv_obj_check_type(o, &lv_list_class)) return "list";
  if (lv_obj_check_type(o, &lv_qrcode_class)) return "qrcode";
  return "obj";
}

// Widgets that manage their own children (or have none worth checking).
bool is_leaf(const lv_obj_t* o) {
  return lv_obj_check_type(o, &lv_textarea_class) || lv_obj_check_type(o, &lv_keyboard_class) ||
         lv_obj_check_type(o, &lv_qrcode_class);
}

// "<kind> "<text>" [x,y wxh]": the object's own text or its first label's,
// cut at a UTF-8 boundary.
void describe(const lv_obj_t* o, char* buf, size_t n) {
  const lv_obj_t* l = lv_obj_check_type(o, &lv_label_class) ? o : nullptr;
  for (uint32_t i = 0; l == nullptr && i < lv_obj_get_child_count(o); ++i) {
    const lv_obj_t* c = lv_obj_get_child(o, (int32_t)i);
    if (lv_obj_check_type(c, &lv_label_class)) l = c;
  }
  char text[28] = "";
  if (l != nullptr) {
    const char* s = lv_label_get_text(l);
    size_t len = s ? strlen(s) : 0;
    if (len > sizeof(text) - 1) {
      len = sizeof(text) - 1;
      while (len > 0 && ((uint8_t)s[len] & 0xC0) == 0x80) --len;
    }
    memcpy(text, s, len);
    text[len] = '\0';
  }
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  snprintf(buf, n, "%s \"%s\" [%ld,%ld %ldx%ld]", kind_of(o), text, (long)a.x1, (long)a.y1,
           (long)lv_area_get_width(&a), (long)lv_area_get_height(&a));
}

void report(Check& c, const lv_obj_t* o, const char* what) {
  ++c.problems;
  if (!c.log) return;
  char d[96];
  describe(o, d, sizeof(d));
  log::warn("layout: %s %s %s", c.view, d, what);
}

bool inside(const lv_area_t& a, const lv_area_t& box, bool vertical) {
  if (a.x1 < box.x1 || a.x2 > box.x2) return false;
  return !vertical || (a.y1 >= box.y1 && a.y2 <= box.y2);
}

bool shown(const lv_obj_t* o) { return !lv_obj_has_flag(o, LV_OBJ_FLAG_HIDDEN); }

void check_obj(Check& c, lv_obj_t* o, bool in_scroller) {
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  // Scroller children are checked against the scroller (horizontally) below.
  if (!in_scroller && !inside(a, c.visible, true) && o != c.overlay) report(c, o, "is outside the visible area");
  if (o != c.scroller && lv_obj_get_class(o) == &lv_obj_class && lv_obj_has_flag(o, LV_OBJ_FLAG_SCROLLABLE)) {
    report(c, o, "is a scrollable container");
  }
  if (lv_obj_check_type(o, &lv_label_class) && lv_label_get_long_mode(o) == LV_LABEL_LONG_MODE_WRAP &&
      (lv_obj_get_self_width(o) > lv_obj_get_content_width(o) ||
       lv_obj_get_self_height(o) > lv_obj_get_content_height(o))) {
    report(c, o, "text does not fit");
  }
  if (is_leaf(o)) return;

  lv_area_t box = a;
  box.x1 += lv_obj_get_style_space_left(o, LV_PART_MAIN);
  box.x2 -= lv_obj_get_style_space_right(o, LV_PART_MAIN);
  box.y1 += lv_obj_get_style_space_top(o, LV_PART_MAIN);
  box.y2 -= lv_obj_get_style_space_bottom(o, LV_PART_MAIN);
  bool scrolls = o == c.scroller;
  uint32_t n = lv_obj_get_child_count(o);
  for (uint32_t i = 0; i < n; ++i) {
    lv_obj_t* ch = lv_obj_get_child(o, (int32_t)i);
    if (!shown(ch) || lv_obj_has_flag(ch, LV_OBJ_FLAG_FLOATING)) continue;
    lv_area_t ca;
    lv_obj_get_coords(ch, &ca);
    if (!inside(ca, box, !scrolls)) report(c, ch, "leaves its parent's content box");
    // Siblings must not overlap (absolute layouts such as the header); the
    // overlay (keyboard) is meant to cover the content.
    for (uint32_t j = i + 1; !scrolls && ch != c.overlay && j < n; ++j) {
      lv_obj_t* sib = lv_obj_get_child(o, (int32_t)j);
      if (!shown(sib) || sib == c.overlay || lv_obj_has_flag(sib, LV_OBJ_FLAG_FLOATING)) continue;
      lv_area_t sa;
      lv_obj_get_coords(sib, &sa);
      if (ca.x1 <= sa.x2 && sa.x1 <= ca.x2 && ca.y1 <= sa.y2 && sa.y1 <= ca.y2) report(c, ch, "overlaps a sibling");
    }
    check_obj(c, ch, in_scroller || scrolls);
  }
}

}  // namespace

namespace detail {

int layout_check(lv_obj_t* root, const char* view, const lv_obj_t* overlay, const lv_obj_t* scroller, bool log) {
  if (root == nullptr) return 0;
  lv_obj_update_layout(root);
  Check c{view, overlay, scroller, {0, 0, CYD_SCREEN_W - 1, CYD_SCREEN_H - 1}, log, 0};
  if (overlay != nullptr && shown(overlay)) {
    lv_area_t oa;
    lv_obj_get_coords(overlay, &oa);
    c.visible.y2 = oa.y1 - 1;
  } else {
    c.overlay = nullptr;
  }
  if (shown(root)) check_obj(c, root, false);
  return c.problems;
}

}  // namespace detail

#if CYD_LAYOUT_SELFTEST
namespace {

constexpr uint32_t kStartAfterMs = 4000;                    // after the first loop() pass
const char kLongSsid[] = "Selftest Network 32 Characters!";  // 31 bytes, SSIDs have at most 32
const char kMissingSsid[] = "CYD-Selftest-Missing-Network";

struct Transition {
  uint32_t ms;
  wifi::State to;
};
Transition g_tr[24];
volatile int g_tr_n = 0;

const char* state_name(wifi::State s) {
  switch (s) {
    case wifi::State::Off:
      return "off";
    case wifi::State::Connecting:
      return "connecting";
    case wifi::State::Connected:
      return "connected";
    case wifi::State::Offline:
      return "offline";
    case wifi::State::Setup:
      return "setup";
  }
  return "?";
}

// Logs every change of the station status / Wi-Fi mode the loop task can see.
void watch_link() {
  static int last_status = -1, last_mode = -1;
  int st = (int)WiFi.status(), mode = (int)WiFi.getMode();
  if (st == last_status && mode == last_mode) return;
  last_status = st;
  last_mode = mode;
  log::info("wifi-st: status %d mode %d channel %d", st, mode, (int)WiFi.channel());
}

// The loop task is ours while the test runs: keep LVGL, Wi-Fi and the
// rollback timer going like wifi_begin() does in required mode.
void pump(uint32_t ms) {
  uint32_t t0 = millis();
  do {
    detail::display_loop();
    detail::wifi_loop();
    detail::rollback_loop();
    watch_link();
    delay(5);
  } while (millis() - t0 < ms);
}

template <typename F>
bool pump_until(F cond, uint32_t timeout_ms) {
  uint32_t t0 = millis();
  while (!cond()) {
    if (millis() - t0 >= timeout_ms) return false;
    pump(10);
  }
  return true;
}

detail::WifiUiProbe probe() {
  detail::WifiUiProbe p{};
  detail::wifi_ui_probe(&p);
  return p;
}

bool visible_now(const lv_obj_t* o) { return o != nullptr && !lv_obj_has_flag(o, LV_OBJ_FLAG_HIDDEN); }

int32_t top_of(const lv_obj_t* o) {
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  return a.y1;
}

int32_t bottom_of(const lv_obj_t* o) {
  lv_area_t a;
  lv_obj_get_coords(o, &a);
  return a.y2;
}

int fail(const char* view, const char* what) {
  log::warn("layout: %s %s", view, what);
  return 1;
}

// A textarea the user types into must be completely above the keyboard.
int textarea_above_keyboard(const char* view, const lv_obj_t* ta, const lv_obj_t* kb, const char* which) {
  if (ta == nullptr || !visible_now(kb)) return 0;
  if (bottom_of(ta) < top_of(kb) && top_of(ta) >= 0) return 0;
  char buf[96];
  snprintf(buf, sizeof(buf), "%s textarea (y %ld..%ld) is not above the keyboard (y %ld)", which, (long)top_of(ta),
           (long)bottom_of(ta), (long)top_of(kb));
  return fail(view, buf);
}

// Runs the checker plus the view-specific assertions and prints one line.
int check_view(const char* name) {
  detail::WifiUiProbe p = probe();
  if (p.scr == nullptr) return fail(name, "setup screen is not open");
  pump(250);  // render once, so what we measure is what the panel shows
  p = probe();
  int n = detail::layout_check(p.scr, name, p.kb, p.list, true);
  lv_obj_update_layout(p.scr);
  if (p.view == 2) {
    // The network list must show at least three rows.
    int32_t row = 0;
    for (uint32_t i = 0; p.list != nullptr && i < lv_obj_get_child_count(p.list) && row == 0; ++i) {
      lv_obj_t* ch = lv_obj_get_child(p.list, (int32_t)i);
      if (visible_now(ch)) row = lv_obj_get_height(ch);
    }
    if (row == 0) row = lv_font_get_line_height(ui_theme_colors().font_normal) + 16;  // empty list: themed row estimate
    int32_t h = (p.list != nullptr && visible_now(p.list)) ? lv_obj_get_content_height(p.list) : 0;
    if (h < 3 * row) {
      char buf[80];
      snprintf(buf, sizeof(buf), "list shows %ld px = %.1f rows of %ld px, want >= 3", (long)h, (double)h / row,
               (long)row);
      n += fail(name, buf);
    }
  }
  if (p.view == 3 && visible_now(p.kb)) {
    n += textarea_above_keyboard(name, lv_keyboard_get_textarea(p.kb), p.kb, "focused");
    n += textarea_above_keyboard(name, p.ta_ssid, p.kb, "SSID");
    n += textarea_above_keyboard(name, p.ta_pass, p.kb, "password");
  }
  if (n == 0) {
    log::info("layout-selftest: %s PASS", name);
  } else {
    log::info("layout-selftest: %s FAIL %d", name, n);
  }
  return n;
}

// After a failed attempt the error must be readable.
int check_failure_status(const char* name) {
  detail::WifiUiProbe p = probe();
  if (p.status == nullptr || !visible_now(p.status)) return fail(name, "status label is hidden");
  const char* text = lv_label_get_text(p.status);
  if (strcmp(text, CYD_S_WIFI_FAILED) != 0) return fail(name, "status does not show the failure text");
  if (visible_now(p.kb) && bottom_of(p.status) >= top_of(p.kb)) return fail(name, "status is under the keyboard");
  return 0;
}

int layout_views() {
  int fails = 0;
  if (!pump_until([] { return probe().view == 1; }, 5000)) return fail("phone", "phone view did not open");
  fails += check_view("phone");

  detail::wifi_ui_test(detail::WifiUiTest::ToDevice);
  pump_until([] { return probe().view == 2; }, 3000);
  fails += check_view("list-scanning");
  pump_until([] { return !probe().scanning; }, 15000);
  fails += check_view("list");

  detail::wifi_ui_test(detail::WifiUiTest::Network, kLongSsid);
  pump_until([] { return probe().view == 3; }, 3000);
  fails += check_view("password");

  detail::wifi_ui_test(detail::WifiUiTest::Hidden);
  pump_until([] { return probe().view == 3 && probe().ta_ssid != nullptr; }, 3000);
  fails += check_view("password-hidden");

  // A real attempt against a network that does not exist, through the
  // keyboard's OK key (first OK moves to the password field, second connects).
  detail::WifiUiProbe p = probe();
  if (p.ta_ssid != nullptr && p.ta_pass != nullptr && p.kb != nullptr) {
    lv_textarea_set_text(p.ta_ssid, kMissingSsid);
    lv_textarea_set_text(p.ta_pass, "selftest-password");
    lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
    pump(100);
    lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
    pump(400);
    fails += check_view("password-connecting");
    pump_until([] { return !probe().connecting; }, CYD_WIFI_CONNECT_MS + 5000);
    int n = check_failure_status("password-failed");
    n += check_view("password-failed");
    fails += n;
    // Tapping a field brings the keyboard back for the next attempt.
    p = probe();
    if (p.ta_pass != nullptr) lv_obj_send_event(p.ta_pass, LV_EVENT_CLICKED, nullptr);
    fails += check_view("password-retry");
  } else {
    fails += fail("password-failed", "hidden-network view has no textareas or keyboard");
  }

  detail::wifi_ui_test(detail::WifiUiTest::Close);
  pump_until([] { return !detail::wifi_ui_active(); }, 5000);
  return fails;
}

// The on-device path with the stored network: open the setup screen, pick the
// network, type the password (first a wrong one when wrong_pass is set),
// connect, let the screen close itself, then watch the state for flaps.
// PASS = connected within 10 s of the close and no state other than
// "connected" after it.
bool reconnect_case(const char* name, const String& ssid, const String& pass, const char* wrong_pass) {
  log::info("layout-selftest: reconnect case %s", name);
  wifi::open_setup();
  if (!pump_until([] { return probe().view == 1; }, 5000)) return false;
  pump(500);
  detail::wifi_ui_test(detail::WifiUiTest::ToDevice);
  pump_until([] { return probe().view == 2 && !probe().scanning; }, 15000);
  detail::wifi_ui_test(detail::WifiUiTest::Network, ssid.c_str());
  pump_until([] { return probe().view == 3; }, 3000);
  detail::WifiUiProbe p = probe();
  if (p.ta_pass == nullptr || p.kb == nullptr) return false;
  if (wrong_pass != nullptr) {
    lv_textarea_set_text(p.ta_pass, wrong_pass);
    lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
    pump_until([] { return !probe().connecting; }, CYD_WIFI_CONNECT_MS + 5000);
    log::info("layout-selftest: %s wrong password -> %s", name, check_failure_status(name) == 0 ? "failure shown" : "?");
    p = probe();
    lv_obj_send_event(p.ta_pass, LV_EVENT_CLICKED, nullptr);  // tap the field: keyboard back
    pump(300);
  }
  lv_textarea_set_text(p.ta_pass, pass.c_str());
  uint32_t t_start = millis();
  lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
  pump_until([] { return probe().closing || !probe().connecting; }, CYD_WIFI_CONNECT_MS + 5000);
  bool success = probe().closing;
  uint32_t t_success = millis();
  pump_until([] { return !detail::wifi_ui_active(); }, 5000);
  uint32_t t_closed = millis();
  int from = g_tr_n;
  bool up = pump_until([] { return wifi::state() == wifi::State::Connected && WiFi.status() == WL_CONNECTED; },
                       15000);
  uint32_t t_up = millis();
  pump(5000);  // flaps after the link came up count too
  int flaps = 0;
  for (int i = from; i < g_tr_n && i < (int)(sizeof(g_tr) / sizeof(g_tr[0])); ++i) {
    log::info("layout-selftest: %s +%lu ms -> %s", name, (unsigned long)(g_tr[i].ms - t_closed),
              state_name(g_tr[i].to));
    if (g_tr[i].to != wifi::State::Connected) ++flaps;
  }
  uint32_t ms = up ? t_up - t_closed : 0;
  bool ok = success && up && ms <= 10000 && flaps == 0 && wifi::state() == wifi::State::Connected;
  log::info("layout-selftest: reconnect %s %lu (%s: connect %lu ms, close after %lu ms, %d flaps, rssi %d)",
            ok ? "PASS" : "FAIL", (unsigned long)ms, name, (unsigned long)(t_success - t_start),
            (unsigned long)(t_closed - t_success), flaps, (int)WiFi.RSSI());
  return ok;
}

void run_selftest() {
  log::info("layout-selftest: start %dx%d, language %s, free heap %lu", CYD_SCREEN_W, CYD_SCREEN_H, CYD_LANG,
            (unsigned long)ESP.getFreeHeap());
  wifi::on_change([](wifi::State s) {
    int i = g_tr_n;
    if (i < (int)(sizeof(g_tr) / sizeof(g_tr[0]))) {
      g_tr[i] = {millis(), s};
      g_tr_n = i + 1;
    }
  });
  String ssid, pass;
  bool have = CYD_LAYOUT_SELFTEST == 1 && detail::wifi_load_credentials(&ssid, &pass);
  // First-time setup: the station never connected since boot (wifi_begin()
  // skipped the stored network in this build).
  if (have) reconnect_case("fresh", ssid, pass, nullptr);

  wifi::open_setup();
  int fails = layout_views();
  if (fails == 0) {
    log::info("layout-selftest: PASS");
  } else {
    log::info("layout-selftest: FAIL %d", fails);
  }

  if (!have) {
    log::info("layout-selftest: reconnect skipped (%s)", CYD_LAYOUT_SELFTEST == 1 ? "no stored network" : "layout only");
  } else {
    // The failed attempt above dropped the link; closing the screen went back
    // to the stored network.
    pump_until([] { return wifi::state() == wifi::State::Connected; }, CYD_WIFI_CONNECT_MS + 5000);
    pump(2000);
    reconnect_case("link-up", ssid, pass, nullptr);
    reconnect_case("retry", ssid, pass, "selftest-wrong-password");
    WiFi.disconnect(false, false);  // the station is down when the screen opens
    pump_until([] { return WiFi.status() != WL_CONNECTED; }, 3000);
    reconnect_case("link-down", ssid, pass, nullptr);
  }
  log::info("layout-selftest: done, free heap %lu", (unsigned long)ESP.getFreeHeap());
}

}  // namespace

namespace detail {

// Before wifi_begin(): log the driver's events from the first connection on.
void layout_selftest_begin() {
  WiFi.onEvent([](arduino_event_id_t ev, arduino_event_info_t info) {
    if (ev == ARDUINO_EVENT_WIFI_STA_DISCONNECTED) {
      log::info("wifi-ev: %s reason %u", WiFi.eventName(ev), (unsigned)info.wifi_sta_disconnected.reason);
    } else if (ev != ARDUINO_EVENT_WIFI_SCAN_DONE) {
      log::info("wifi-ev: %s", WiFi.eventName(ev));
    }
  });
}

void layout_selftest_loop() {
  static uint32_t first = 0;
  static bool done = false;
  if (done) return;
  if (first == 0) first = millis() | 1;
  if (millis() - first < kStartAfterMs || wifi_ui_active()) return;
  done = true;
  run_selftest();
}

}  // namespace detail
#endif  // CYD_LAYOUT_SELFTEST
}  // namespace cyd
