// cyd_portal.cpp - WiFiManager captive portal in non-blocking mode.
//
// SoftAP "CYD-Setup-XXXX" (last two MAC bytes) protected by WPA2 with a fresh
// random 8-character password from an unambiguous alphabet. The page texts
// come from cyd_wm_strings.h (project language); the colours from the theme.
#include "cyd_private.h"

#include <WiFi.h>
#include <WiFiManager.h>
#include <esp_random.h>

namespace cyd {
namespace {

// WiFiManager 2.0.17 stopConfigPortal() runs shutdownConfigPortal(), which
// rewrites the SoftAP config (empty SSID on channel 6, so the AP restarts),
// blocks the loop for a second, forces its own default mode (STA) and calls
// WiFi.reconnect() when the station sits between association and DHCP; each
// step can restart the station link a setup just brought up. stop_quietly()
// only frees what the portal allocated (web and DNS server, which the next
// startConfigPortal() creates again) and leaves the radio to portal_stop().
// It uses protected members of the pinned WiFiManager version.
class CydWiFiManager : public WiFiManager {
 public:
  void stop_quietly() {
    if (server) {
      server->stop();
      server.reset();
    }
    if (dnsServer) {
      dnsServer->stop();
      dnsServer.reset();
    }
    configPortalActive = false;
    _end();
  }
};

CydWiFiManager* g_wm = nullptr;  // created once, never deleted (owns WiFi event hooks)
bool g_running = false;
String g_css;

String hex_color(lv_color_t c) {
  char buf[8];
  snprintf(buf, sizeof(buf), "#%06lX", (unsigned long)(lv_color_to_u32(c) & 0xFFFFFF));
  return String(buf);
}

bool is_dark(lv_color_t c) {
  uint32_t v = lv_color_to_u32(c);
  uint32_t r = (v >> 16) & 0xFF, g = (v >> 8) & 0xFF, b = v & 0xFF;
  return (r * 299 + g * 587 + b * 114) / 1000 < 128;
}

void build_css() {
  const ThemeColors& t = ui_theme_colors();
  String radius = String((int)t.radius) + "px";
  g_css = "<style>";
  g_css += "body,.wrap{background:" + hex_color(t.bg) + ";color:" + hex_color(t.text) + "}";
  g_css += "h1,h3,label{color:" + hex_color(t.text) + "}";
  g_css += "button,input[type='button'],input[type='submit']{background:" + hex_color(t.accent) +
           ";color:" + hex_color(t.on_accent) + ";border-radius:" + radius + ";border:0}";
  g_css += "input,select{background:" + hex_color(t.surface) + ";color:" + hex_color(t.text) + ";border:1px solid " +
           hex_color(t.border) + ";border-radius:" + radius + "}";
  g_css += "a{color:" + hex_color(t.accent) + "}";
  g_css += ".msg{background:" + hex_color(t.surface) + ";border-left-color:" + hex_color(t.accent) +
           ";color:" + hex_color(t.text) + "}";
  g_css += ".msg.S{border-left-color:" + hex_color(t.success) + "}.msg.D{border-left-color:" + hex_color(t.danger) + "}";
  g_css += "</style>";
}

// WiFiManager registers admin routes next to the Wi-Fi pages (firmware upload,
// erase, restart, info, exit, ...) that nobody on the setup SoftAP may use, and
// /exit shuts the portal down behind our back. WiFiManager calls this hook after
// it has reset its web server and before it registers its own routes, and the
// first matching handler wins, so these routes answer 404 instead.
void block_admin_routes() {
  static const char* const kBlocked[] = {"/update", "/u", "/erase", "/restart", "/info",
                                         "/param",  "/paramsave", "/exit", "/close"};
  if (g_wm == nullptr || !g_wm->server) return;
  for (const char* path : kBlocked) {
    g_wm->server->on(path, []() { g_wm->server->send(404, "text/plain", "Not found"); });
  }
}

}  // namespace

namespace detail {

void portal_start(String* ap_ssid, String* ap_pass) {
  static const char kAlphabet[] = "abcdefghjkmnpqrstuvwxyz23456789";  // no 0/o, 1/l/i
  uint8_t mac[6];
  WiFi.macAddress(mac);
  char ssid[20];
  snprintf(ssid, sizeof(ssid), "CYD-Setup-%02X%02X", mac[4], mac[5]);
  char pass[9];
  for (int i = 0; i < 8; ++i) pass[i] = kAlphabet[esp_random() % (sizeof(kAlphabet) - 1)];
  pass[8] = '\0';
  *ap_ssid = ssid;
  *ap_pass = pass;

  if (g_wm == nullptr) {
    g_wm = new CydWiFiManager();
    g_wm->setRestorePersistent(false);  // keep WiFi.persistent(false)
    g_wm->setDisableConfigPortal(false);  // a phone-path connect must not run shutdownConfigPortal() either
    g_wm->setDebugOutput(false);
    g_wm->setConfigPortalBlocking(false);
    g_wm->setConnectTimeout(15);
    g_wm->setShowInfoUpdate(false);
    g_wm->setShowInfoErase(false);
    static const char* menu[] = {"wifi"};
    g_wm->setMenu(menu, 1);
    g_wm->setWebServerCallback(block_admin_routes);
  }
  build_css();
  g_wm->setTitle(CYD_S_WIFI_SETUP_TITLE);
  g_wm->setDarkMode(is_dark(ui_theme_colors().bg));
  g_wm->setCustomHeadElement(g_css.c_str());  // WiFiManager keeps the pointer; g_css stays alive
  g_wm->startConfigPortal(ssid, pass);       // returns at once in non-blocking mode
  g_running = true;
  log::info("portal: SoftAP %s started", ssid);
}

bool portal_active() { return g_running; }

// True once a phone saved credentials and the station connected; the portal
// keeps running until portal_stop(). If WiFiManager ever shuts it down on its
// own, its web server is freed: stop_quietly() checks for that.
bool portal_loop() {
  if (!g_running) return false;
  bool connected = g_wm->process();
  if (connected || !g_wm->getConfigPortalActive()) g_running = false;
  if (connected) {
    log::info("portal: connected through the phone path");
  } else if (!g_running) {
    log::warn("portal: ended without a connection");
  }
  return connected;
}

void portal_stop() {
  if (g_wm == nullptr) return;
  bool active = g_wm->getConfigPortalActive();
  if (!g_running && !active) return;
  g_running = false;
  if (active) g_wm->stop_quietly();
  // SoftAP off: AP+STA -> STA keeps an associated station associated (no AP
  // config rewrite, no reconnect); from AP only the station starts idle.
  WiFi.mode(WIFI_STA);
  log::info("portal: stopped");
}

}  // namespace detail
}  // namespace cyd
