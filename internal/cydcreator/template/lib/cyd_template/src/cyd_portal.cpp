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

WiFiManager* g_wm = nullptr;  // created once, never deleted (owns WiFi event hooks)
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
    g_wm = new WiFiManager();
    g_wm->setRestorePersistent(false);  // keep WiFi.persistent(false)
    g_wm->setDebugOutput(false);
    g_wm->setConfigPortalBlocking(false);
    g_wm->setConnectTimeout(15);
    g_wm->setShowInfoUpdate(false);
    g_wm->setShowInfoErase(false);
    static const char* menu[] = {"wifi", "exit"};
    g_wm->setMenu(menu, 2);
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

// True once a phone saved credentials and the station connected; WiFiManager
// has then already shut its portal down.
bool portal_loop() {
  if (!g_running) return false;
  if (g_wm->process()) {
    g_running = false;
    log::info("portal: connected through the phone path");
    return true;
  }
  return false;
}

void portal_stop() {
  if (!g_running) return;
  g_wm->stopConfigPortal();
  g_running = false;
  WiFi.softAPdisconnect(true);  // AP off, station link untouched
  log::info("portal: stopped");
}

}  // namespace detail
}  // namespace cyd
