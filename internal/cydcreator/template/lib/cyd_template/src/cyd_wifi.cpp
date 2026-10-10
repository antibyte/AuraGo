// cyd_wifi.cpp - Wi-Fi state machine (off | optional | required), credentials
// in NVS namespace "cyd_wifi", reconnects, NTP and the cyd::wifi / cyd::time API.
#include "cyd_private.h"

#include <WiFi.h>
#include <esp_wifi.h>
#include <time.h>

#include <vector>

namespace cyd {
namespace {

constexpr const char* kNs = "cyd_wifi";

wifi::State g_state = wifi::State::Off;
std::vector<std::function<void(wifi::State)>> g_listeners;
String g_ssid;
String g_pass;
bool g_have_creds = false;
uint32_t g_connect_started = 0;
uint32_t g_last_retry = 0;
bool g_time_started = false;

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

#if CYD_WIFI_MODE != CYD_WIFI_OFF
void apply_hostname() {
  uint8_t mac[6];
  WiFi.macAddress(mac);
  char host[16];
  snprintf(host, sizeof(host), "cyd-%02x%02x", mac[4], mac[5]);
  WiFi.setHostname(host);
}
#endif

}  // namespace

namespace detail {

bool wifi_load_credentials(String* ssid, String* pass) {
  *ssid = sys_get_string(kNs, "ssid");
  *pass = sys_get_string(kNs, "pass");
  return ssid->length() > 0;
}

void wifi_save_credentials(const String& ssid, const String& pass) {
  sys_set_string(kNs, "ssid", ssid);
  sys_set_string(kNs, "pass", pass);
  g_ssid = ssid;
  g_pass = pass;
  g_have_creds = ssid.length() > 0;
  log::info("wifi: credentials saved for \"%s\"", ssid.c_str());
}

void wifi_forget_credentials() {
  sys_clear(kNs);
  g_ssid = "";
  g_pass = "";
  g_have_creds = false;
#if CYD_WIFI_MODE != CYD_WIFI_OFF
  // Also clear what the IDF keeps in its own NVS namespace (older firmware,
  // WiFiManager). The BOOT reset runs before wifi_begin(), so the driver may not
  // be started yet: eraseAP() starts it and we stop it again afterwards.
  bool was_off = WiFi.getMode() == WIFI_OFF;
  bool erased = WiFi.eraseAP();
  esp_wifi_set_storage(WIFI_STORAGE_RAM);  // the driver was started with flash storage before WiFi.persistent(false)
  if (was_off) WiFi.mode(WIFI_OFF);
  log::info("wifi: credentials erased (driver store %s)", erased ? "cleared" : "unchanged");
#else
  log::info("wifi: credentials erased");
#endif
}

void wifi_set_state(wifi::State s) {
  if (s == g_state) return;
  log::info("wifi: %s -> %s", state_name(g_state), state_name(s));
  g_state = s;
  for (auto& cb : g_listeners) cb(s);
}

void wifi_connect(const String& ssid, const String& pass) {
  g_ssid = ssid;
  g_pass = pass;
  WiFi.begin(ssid.c_str(), pass.length() > 0 ? pass.c_str() : nullptr);
  g_connect_started = millis();
  g_last_retry = g_connect_started;
  if (!wifi_ui_active()) wifi_set_state(wifi::State::Connecting);
}

void time_begin() {
  if (g_time_started) return;
  g_time_started = true;
  configTzTime(CYD_TZ, "pool.ntp.org", "time.google.com", "time.cloudflare.com");
  log::info("time: NTP started (TZ %s)", CYD_TZ);
}

void wifi_begin() {
#if CYD_WIFI_MODE == CYD_WIFI_OFF
  WiFi.mode(WIFI_OFF);
  g_state = wifi::State::Off;
#else
  WiFi.persistent(false);  // credentials live in our own NVS namespace
  WiFi.setAutoReconnect(true);
  apply_hostname();
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  g_have_creds = wifi_load_credentials(&g_ssid, &g_pass);
  if (g_have_creds) {
    splash_status(CYD_S_WIFI_CONNECTING);
    wifi_connect(g_ssid, g_pass);
    while (WiFi.status() != WL_CONNECTED && millis() - g_connect_started < CYD_WIFI_CONNECT_MS) pump_ui(50);
  }
  if (WiFi.status() == WL_CONNECTED) {
    wifi_set_state(wifi::State::Connected);
    time_begin();
    log::info("wifi: connected to \"%s\", ip %s, rssi %d, free heap %lu", g_ssid.c_str(),
              WiFi.localIP().toString().c_str(), (int)WiFi.RSSI(), (unsigned long)ESP.getFreeHeap());
    return;
  }
#if CYD_WIFI_MODE == CYD_WIFI_REQUIRED
  // The app cannot start without a network: stay in setup until connected.
  wifi_ui_open(false);
  while (g_state != wifi::State::Connected) {
    display_loop();
    wifi_loop();
    rollback_loop();  // waiting in setup for minutes is no unstable boot
    delay(5);
  }
#else
  wifi_set_state(wifi::State::Offline);
#endif
#endif
}

void wifi_loop() {
#if CYD_WIFI_MODE != CYD_WIFI_OFF
  if (wifi_ui_active()) {
    wifi_ui_loop();
    return;
  }
  uint32_t now = millis();
  bool up = WiFi.status() == WL_CONNECTED;
  switch (g_state) {
    case wifi::State::Connecting:
      if (up) {
        wifi_set_state(wifi::State::Connected);
        time_begin();
      } else if (now - g_connect_started > CYD_WIFI_CONNECT_MS) {
        wifi_set_state(wifi::State::Offline);
      }
      break;
    case wifi::State::Connected:
      if (!up) {
        g_connect_started = now;  // the driver reconnects on its own (auto-reconnect)
        wifi_set_state(wifi::State::Connecting);
      }
      break;
    case wifi::State::Offline:
      if (up) {
        wifi_set_state(wifi::State::Connected);
        time_begin();
      } else if (g_have_creds && now - g_last_retry > CYD_WIFI_RETRY_MS) {
        wifi_connect(g_ssid, g_pass);
      }
      break;
    default:
      break;
  }
#endif
}

}  // namespace detail

namespace wifi {

State state() { return g_state; }

bool connected() { return g_state == State::Connected && WiFi.status() == WL_CONNECTED; }

void on_change(std::function<void(State)> cb) { g_listeners.push_back(std::move(cb)); }

void open_setup() {
#if CYD_WIFI_MODE == CYD_WIFI_OFF
  log::warn("wifi: open_setup() ignored, Wi-Fi is off for this app");
#else
  if (detail::wifi_ui_active()) return;
  detail::wifi_ui_open(CYD_WIFI_MODE == CYD_WIFI_OPTIONAL || connected());
#endif
}

void forget() {
#if CYD_WIFI_MODE != CYD_WIFI_OFF
  detail::wifi_forget_credentials();
  WiFi.disconnect(false, true);  // also drop anything the IDF stored
  detail::wifi_set_state(State::Offline);
#if CYD_WIFI_MODE == CYD_WIFI_REQUIRED
  open_setup();
#endif
#endif
}

String ip() { return connected() ? WiFi.localIP().toString() : String(); }

int rssi() { return connected() ? (int)WiFi.RSSI() : 0; }

}  // namespace wifi

namespace time {

bool synced() { return ::time(nullptr) > 1700000000; }  // after 2023-11-14

time_t now() { return ::time(nullptr); }

String format(const char* strftime_fmt) {
  if (!synced()) return String();
  time_t t = ::time(nullptr);
  struct tm tmv;
  localtime_r(&t, &tmv);
  char buf[64];
  size_t n = strftime(buf, sizeof(buf), strftime_fmt, &tmv);
  if (n == 0) return String();  // too long for the buffer (or an empty result): buf is undefined then
  return String(buf);
}

}  // namespace time
}  // namespace cyd
