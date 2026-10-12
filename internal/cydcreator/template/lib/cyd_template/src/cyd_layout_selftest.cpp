// cyd_layout_selftest.cpp - on-device self-test of the Wi-Fi setup screen,
// compiled only with CYD_LAYOUT_SELFTEST (test builds; the generator never sets
// it): views, keyboard, failure status and screen captures (all modes),
// reconnect after an on-device setup (mode 1), a soak of view switches with
// heap numbers (mode 3).
#include "cyd_private.h"

#if CYD_LAYOUT_SELFTEST
#include <WiFi.h>
#include <esp_heap_caps.h>

namespace cyd {
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

// Screen capture for the host: "shot:<view>:<x>,<y>,<w>,<h>:<base64>" per
// flushed band, the payload a run-length list of (count, RGB565) uint16 LE
// pairs. Only on screens up to 480x320 (larger ones would take minutes at
// 115200 baud).
struct Base64Out {
  uint8_t acc[3];
  int n = 0;
  void put(uint8_t b) {
    acc[n++] = b;
    if (n == 3) flush();
  }
  void flush() {
    static const char kAlpha[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    if (n == 0) return;
    uint32_t v = (uint32_t)acc[0] << 16 | (n > 1 ? (uint32_t)acc[1] << 8 : 0) | (n > 2 ? acc[2] : 0);
    char out[4] = {kAlpha[(v >> 18) & 63], kAlpha[(v >> 12) & 63], n > 1 ? kAlpha[(v >> 6) & 63] : '=',
                   n > 2 ? kAlpha[v & 63] : '='};
    Serial.write((const uint8_t*)out, 4);
    n = 0;
  }
  void put16(uint16_t v) {
    put(v & 0xFF);
    put(v >> 8);
  }
};

const char* g_shot = nullptr;

void shot_band(int32_t x, int32_t y, int32_t w, int32_t h, const uint16_t* px) {
  Serial.printf("shot:%s:%ld,%ld,%ld,%ld:", g_shot, (long)x, (long)y, (long)w, (long)h);
  Base64Out b;
  int32_t total = w * h;
  for (int32_t i = 0; i < total;) {
    uint16_t c = px[i];
    int32_t run = 1;
    while (i + run < total && px[i + run] == c && run < 0xFFFF) ++run;
    b.put16((uint16_t)run);
    b.put16(c);
    i += run;
  }
  b.flush();
  Serial.write('\n');
}

void shoot(const char* view) {
  if (CYD_SCREEN_W * CYD_SCREEN_H > 480 * 320) return;
  g_shot = view;
  detail::flush_tap = shot_band;
  lv_obj_invalidate(lv_screen_active());
  lv_refr_now(nullptr);
  detail::flush_tap = nullptr;
  Serial.printf("shot-end:%s\n", view);
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
  shoot(name);
  return n;
}

// The status must show `want` and be readable (not hidden, not under the keyboard).
int check_status(const char* name, const char* want) {
  detail::WifiUiProbe p = probe();
  if (p.status == nullptr || !visible_now(p.status)) return fail(name, "status label is hidden");
  if (strcmp(lv_label_get_text(p.status), want) != 0) return fail(name, "status shows another text");
  if (visible_now(p.kb) && bottom_of(p.status) >= top_of(p.kb)) return fail(name, "status is under the keyboard");
  return 0;
}

// After a failed attempt the error must be readable.
int check_failure_status(const char* name) { return check_status(name, CYD_S_WIFI_FAILED); }

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

  // A too short password for an encrypted network: no join, the reason shows.
  {
    detail::WifiUiProbe q = probe();
    if (q.ta_pass != nullptr && q.kb != nullptr) {
      lv_textarea_set_text(q.ta_pass, "short");
      lv_obj_send_event(q.kb, LV_EVENT_READY, nullptr);
      pump(300);
      q = probe();
      int n = check_status("password-short", CYD_S_WIFI_PASSWORD_LENGTH);
      if (q.connecting || !q.portal) n += fail("password-short", "a join started");
      n += check_view("password-short");
      fails += n;
    }
  }

  // An open network joins right away: no field, no keyboard, the status shows.
  // (The next view change cancels this attempt against a missing network.)
  detail::wifi_ui_test(detail::WifiUiTest::OpenNetwork, kMissingSsid);
  pump_until([] { return probe().view == 3; }, 3000);
  {
    int n = check_status("password-open", CYD_S_WIFI_CONNECTING);
    detail::WifiUiProbe q = probe();
    if (!q.connecting || q.kb != nullptr || q.ta_pass != nullptr) n += fail("password-open", "no direct join");
    n += check_view("password-open");
    fails += n;
  }

  detail::wifi_ui_test(detail::WifiUiTest::Hidden);
  pump_until([] { return probe().view == 3 && probe().ta_ssid != nullptr; }, 3000);
  fails += check_view("password-hidden");

  // Back during an attempt cancels it: the portal is up again and the list
  // scans next to it.
  detail::WifiUiProbe p = probe();
  if (p.ta_ssid != nullptr && p.ta_pass != nullptr && p.kb != nullptr) {
    lv_textarea_set_text(p.ta_ssid, kMissingSsid);
    lv_textarea_set_text(p.ta_pass, "selftest-password");
    lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
    pump(100);
    lv_obj_send_event(p.kb, LV_EVENT_READY, nullptr);
    pump(800);
    bool was_connecting = probe().connecting;
    detail::wifi_ui_test(detail::WifiUiTest::Back);
    pump_until([] { return probe().view == 2; }, 3000);
    int n = check_view("list-after-cancel");
    p = probe();
    if (!was_connecting || p.connecting || !p.portal) {
      n += fail("list-after-cancel", "attempt not cancelled or portal not back");
    }
    pump_until([] { return !probe().scanning; }, 16000);
    log::info("layout-selftest: list-after-cancel scan listed %u networks", (unsigned)probe().networks);
    fails += n;
    detail::wifi_ui_test(detail::WifiUiTest::Hidden);
    pump_until([] { return probe().view == 3 && probe().ta_ssid != nullptr; }, 3000);
  }

  // A real attempt against a network that does not exist, through the
  // keyboard's OK key (first OK moves to the password field, second connects).
  p = probe();
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

  // The phone view after a failed phone-path join (state injected).
  detail::wifi_ui_test(detail::WifiUiTest::PhoneFailed);
  pump_until([] { return probe().view == 1; }, 3000);
  fails += check_failure_status("phone-failed") + check_view("phone-failed");

  detail::wifi_ui_test(detail::WifiUiTest::Close);
  pump_until([] { return !detail::wifi_ui_active(); }, 5000);
  return fails;
}

// Repeated view switches must not leak: phone -> list -> password -> back ->
// hidden network -> back -> back, heap after one warm-up cycle and after all
// cycles (no scan running at either point).
void soak(int cycles) {
  wifi::open_setup();
  if (!pump_until([] { return probe().view == 1; }, 5000)) return;
  auto cycle = [] {
    detail::wifi_ui_test(detail::WifiUiTest::ToDevice);
    pump_until([] { return probe().view == 2; }, 3000);
    detail::wifi_ui_test(detail::WifiUiTest::Network, kLongSsid);
    pump_until([] { return probe().view == 3; }, 3000);
    detail::wifi_ui_test(detail::WifiUiTest::Back);
    pump_until([] { return probe().view == 2; }, 3000);
    detail::wifi_ui_test(detail::WifiUiTest::Hidden);
    pump_until([] { return probe().view == 3 && probe().ta_ssid != nullptr; }, 3000);
    detail::wifi_ui_test(detail::WifiUiTest::Back);
    pump_until([] { return probe().view == 2; }, 3000);
    detail::wifi_ui_test(detail::WifiUiTest::Back);
    pump_until([] { return probe().view == 1; }, 3000);
  };
  auto settle = [] {
    pump_until([] { return !probe().scanning; }, 16000);
    pump(1000);
  };
  cycle();
  settle();
  uint32_t heap0 = ESP.getFreeHeap(), block0 = heap_caps_get_largest_free_block(MALLOC_CAP_8BIT);
  uint32_t t0 = millis();
  for (int i = 0; i < cycles; ++i) cycle();
  settle();
  uint32_t heap1 = ESP.getFreeHeap(), block1 = heap_caps_get_largest_free_block(MALLOC_CAP_8BIT);
  log::info("layout-selftest: soak %d cycles in %lu ms: free heap %lu -> %lu, largest block %lu -> %lu, min free %lu",
            cycles, (unsigned long)(millis() - t0), (unsigned long)heap0, (unsigned long)heap1, (unsigned long)block0,
            (unsigned long)block1, (unsigned long)ESP.getMinFreeHeap());
  detail::wifi_ui_test(detail::WifiUiTest::Close);
  pump_until([] { return !detail::wifi_ui_active(); }, 5000);
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
  if (!success) detail::wifi_ui_test(detail::WifiUiTest::Close);  // like the user giving up
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

  if (CYD_LAYOUT_SELFTEST == 3) {
    pump_until([] { return wifi::state() != wifi::State::Setup; }, 5000);
    soak(50);
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
}  // namespace cyd
#endif  // CYD_LAYOUT_SELFTEST
