// cyd_link.cpp - AuraGo link: device config partition, heartbeat, OTA pull.
//
// Config partition "cydcc" (data, subtype 0x41, 4 KiB): "AGCC", version 1,
// byte 5 = 0, little-endian uint16 payload length at bytes 6-7, JSON
// {url, device_id, token, cert_sha256} from byte 8, zero padded.
// HTTPS connections are opened first, the leaf certificate's SHA-256 is
// compared with cert_sha256, and only then is the bearer token sent.
#include "cyd_private.h"

#if CYD_AURAGO_LINK
#include <HTTPClient.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <esp_ota_ops.h>
#include <esp_partition.h>
#include <mbedtls/sha256.h>

#include <atomic>
#include <memory>
#endif

namespace cyd {
namespace {

lv_obj_t* g_overlay = nullptr;
lv_obj_t* g_overlay_label = nullptr;
lv_obj_t* g_overlay_bar = nullptr;

#if CYD_AURAGO_LINK
constexpr const char* kSysNs = "cyd_sys";

struct LinkConfig {
  String url;  // without trailing slash
  String device_id;
  String token;
  String cert_sha256;
};

struct UpdateOffer {
  String build_id;
  uint32_t size = 0;
  String sha256;
  String url;
};

LinkConfig g_cfg;
bool g_enabled = false;
std::atomic<bool> g_busy{false};          // a worker job is running
std::atomic<bool> g_cert_changed{false};  // pin mismatch seen: stop talking to the server
uint32_t g_next_heartbeat = 0;   // millis(); 0 = as soon as Wi-Fi is up
uint32_t g_interval_ms = 60000;
String g_ota_state = "idle";
String g_ota_error;
bool g_confirm_pending = false;  // this boot is the first of a fresh OTA image
String g_report_failed_build;    // build that was rolled back (report once)
String g_backoff_build;          // update that just failed
uint32_t g_backoff_until = 0;

bool read_config(LinkConfig* out) {
  const esp_partition_t* part =
      esp_partition_find_first(ESP_PARTITION_TYPE_DATA, (esp_partition_subtype_t)CYD_CONFIG_SUBTYPE, "cydcc");
  if (part == nullptr) {
    log::warn("link: partition cydcc not found");
    return false;
  }
  std::unique_ptr<uint8_t[]> buf(new uint8_t[CYD_CONFIG_SIZE]);
  if (esp_partition_read(part, 0, buf.get(), CYD_CONFIG_SIZE) != ESP_OK) return false;
  if (memcmp(buf.get(), "AGCC", 4) != 0 || buf[4] != 1 || buf[5] != 0) {
    log::warn("link: no device config (flash this app from CYD Creator with AuraGo link)");
    return false;
  }
  uint16_t n = (uint16_t)(buf[6] | (buf[7] << 8));
  if (n == 0 || n > CYD_CONFIG_MAX_PAYLOAD) return false;
  JsonDocument doc;
  if (deserializeJson(doc, (const char*)buf.get() + 8, n)) return false;
  out->url = doc["url"] | "";
  out->device_id = doc["device_id"] | "";
  out->token = doc["token"] | "";
  out->cert_sha256 = doc["cert_sha256"] | "";
  while (out->url.endsWith("/")) out->url.remove(out->url.length() - 1);
  bool https = out->url.startsWith("https://");
  if (!(https || out->url.startsWith("http://")) || out->device_id.length() == 0 || out->token.length() == 0) return false;
  if (https && out->cert_sha256.length() != 64) {
    log::error("link: https URL without certificate pin, link disabled");
    return false;
  }
  return true;
}

const char* reset_reason() {
  switch (esp_reset_reason()) {
    case ESP_RST_POWERON:
      return "poweron";
    case ESP_RST_EXT:
      return "ext";
    case ESP_RST_SW:
      return "software";
    case ESP_RST_PANIC:
      return "panic";
    case ESP_RST_INT_WDT:
      return "int_wdt";
    case ESP_RST_TASK_WDT:
      return "task_wdt";
    case ESP_RST_WDT:
      return "wdt";
    case ESP_RST_DEEPSLEEP:
      return "deepsleep";
    case ESP_RST_BROWNOUT:
      return "brownout";
    case ESP_RST_SDIO:
      return "sdio";
    default:
      return "unknown";
  }
}

// One request to the AuraGo server on a fresh connection. For https the
// TLS session is verified against the pinned leaf hash before any byte of
// the request (and the token) is sent. On success *http stays open for the
// caller to read the body; the caller must call http->end().
struct LinkConn {
  std::unique_ptr<WiFiClient> client;
  HTTPClient http;
};

int link_open(LinkConn* c, bool post, const String& path_or_url, const String& body, String* err) {
  String url = path_or_url.startsWith("http") ? path_or_url : g_cfg.url + path_or_url;
  int scheme_len = url.startsWith("https://") ? 8 : 7;
  int host_end = url.indexOf('/', scheme_len);
  String hostport = url.substring(scheme_len, host_end < 0 ? url.length() : host_end);
  String host = hostport;
  uint16_t port = scheme_len == 8 ? 443 : 80;
  int colon = hostport.lastIndexOf(':');  // host names and IPv4 only
  if (colon > 0) {
    host = hostport.substring(0, colon);
    port = (uint16_t)hostport.substring(colon + 1).toInt();
  }
  if (scheme_len == 8) {
    auto* tls = new WiFiClientSecure();
    tls->setInsecure();  // trust comes from the pin check below, not from a CA
    tls->setHandshakeTimeout(15);
    c->client.reset(tls);
    if (!tls->connect(host.c_str(), port)) {
      *err = "connect failed";
      return 0;
    }
    if (!tls->verify(g_cfg.cert_sha256.c_str(), nullptr)) {
      tls->stop();
      g_cert_changed = true;
      *err = "certificate changed";
      return 0;
    }
  } else {
    c->client.reset(new WiFiClient());
    if (!c->client->connect(host.c_str(), port)) {
      *err = "connect failed";
      return 0;
    }
  }
  c->http.setReuse(false);
  c->http.setTimeout(15000);
  c->http.begin(*c->client, url);  // reuses the connected (and verified) client
  c->http.addHeader("Authorization", "Bearer " + g_cfg.token);
  int code;
  if (post) {
    c->http.addHeader("Content-Type", "application/json");
    code = c->http.POST(body);
  } else {
    code = c->http.GET();
  }
  if (code <= 0) {
    *err = HTTPClient::errorToString(code);
    c->http.end();
    return 0;
  }
  return code;
}

int link_json(bool post, const String& path, const String& body, JsonDocument* out, String* err) {
  LinkConn c;
  int code = link_open(&c, post, path, body, err);
  if (code <= 0) return 0;
  if (out != nullptr && code == 200) {
    String text = c.http.getString();
    if (text.length() > 4096 || deserializeJson(*out, text)) *err = "invalid response";
  }
  c.http.end();
  return code;
}

void post_ota_result(const String& build_id, bool ok, const String& error) {
  JsonDocument d;
  d["build_id"] = build_id;
  d["ok"] = ok;
  d["error"] = error;
  String body;
  serializeJson(d, body);
  detail::worker_post([body]() {
    String err;
    int code = link_json(true, "/api/cyd-creator/device/ota-result", body, nullptr, &err);
    log::info("link: ota-result -> %d %s", code, err.c_str());
  });
}

// Called from the worker; every state change happens on the loop task.
void ota_fail(const UpdateOffer& u, const String& why) {
  log::error("ota: %s failed: %s", u.build_id.c_str(), why.c_str());
  String build = u.build_id;
  ui_async([build, why]() {
    g_ota_state = "failed";
    g_ota_error = why;
    g_backoff_build = build;
    g_backoff_until = millis() + 10UL * 60UL * 1000UL;
    detail::ota_overlay_show(CYD_S_OTA_FAILED);
    lv_timer_t* t = lv_timer_create([](lv_timer_t*) { detail::ota_overlay_hide(); }, 3000, nullptr);
    lv_timer_set_repeat_count(t, 1);
    post_ota_result(build, false, why);
    g_busy = false;
  });
}

void show_cert_banner() {
  ui_async([]() {
    const ThemeColors& t = ui_theme_colors();
    lv_obj_t* banner = lv_obj_create(lv_layer_top());
    lv_obj_set_size(banner, CYD_SCREEN_W, LV_SIZE_CONTENT);
    lv_obj_align(banner, LV_ALIGN_TOP_MID, 0, 0);
    lv_obj_set_style_bg_color(banner, t.danger, 0);
    lv_obj_set_style_bg_opa(banner, LV_OPA_COVER, 0);
    lv_obj_set_style_radius(banner, 0, 0);
    lv_obj_set_style_border_width(banner, 0, 0);
    lv_obj_t* l = detail::make_label(banner, CYD_S_OTA_CERT_CHANGED, t.font_small, t.on_accent);
    lv_obj_set_width(l, lv_pct(100));
    lv_label_set_long_mode(l, LV_LABEL_LONG_MODE_WRAP);
    lv_obj_add_flag(banner, LV_OBJ_FLAG_CLICKABLE);  // tap hides it; the timer deletes it
    lv_obj_add_event_cb(
        banner, [](lv_event_t* e) { lv_obj_add_flag(lv_event_get_current_target_obj(e), LV_OBJ_FLAG_HIDDEN); },
        LV_EVENT_CLICKED, nullptr);
    lv_timer_t* timer = lv_timer_create([](lv_timer_t* tm) { lv_obj_delete((lv_obj_t*)lv_timer_get_user_data(tm)); },
                                        15000, banner);
    lv_timer_set_repeat_count(timer, 1);
  });
}

// Runs in the worker task: download into the inactive slot, verify, switch.
void run_ota(UpdateOffer u) {
  ui_async([]() { detail::ota_overlay_show(CYD_S_OTA_INSTALLING); });
  const esp_partition_t* target = esp_ota_get_next_update_partition(nullptr);
  if (target == nullptr) return ota_fail(u, "no ota partition");
  if (u.size == 0 || u.size > target->size) return ota_fail(u, "image does not fit the app slot");

  LinkConn c;
  String err;
  String path = u.url.length() > 0 ? u.url : "/api/cyd-creator/device/firmware/" + u.build_id;
  int code = link_open(&c, false, path, String(), &err);
  if (code != 200) {
    if (code > 0) c.http.end();
    return ota_fail(u, code > 0 ? "http " + String(code) : err);
  }
  int len = c.http.getSize();
  if (len != (int)u.size) {
    c.http.end();
    return ota_fail(u, "size mismatch");
  }
  esp_ota_handle_t handle = 0;
  if (esp_ota_begin(target, u.size, &handle) != ESP_OK) {
    c.http.end();
    return ota_fail(u, "esp_ota_begin failed");
  }
  mbedtls_sha256_context sha;
  mbedtls_sha256_init(&sha);
  mbedtls_sha256_starts(&sha, 0);
  std::unique_ptr<uint8_t[]> buf(new uint8_t[CYD_OTA_CHUNK]);
  WiFiClient* stream = c.http.getStreamPtr();
  uint32_t done = 0;
  uint32_t last_data = millis();
  int last_pct = -1;
  while (done < u.size) {
    size_t avail = stream->available();
    if (avail == 0) {
      if (!stream->connected() || millis() - last_data > 15000) break;
      delay(5);
      continue;
    }
    size_t want = u.size - done;
    if (want > CYD_OTA_CHUNK) want = CYD_OTA_CHUNK;
    if (want > avail) want = avail;
    int got = stream->readBytes(buf.get(), want);
    if (got <= 0) continue;
    if (esp_ota_write(handle, buf.get(), got) != ESP_OK) break;
    mbedtls_sha256_update(&sha, buf.get(), got);
    done += got;
    last_data = millis();
    int pct = (int)((uint64_t)done * 100 / u.size);
    if (pct != last_pct && (pct % 2 == 0 || done == u.size)) {
      last_pct = pct;
      ui_async([pct]() { detail::ota_overlay_progress(pct); });
    }
  }
  c.http.end();
  uint8_t digest[32];
  mbedtls_sha256_finish(&sha, digest);
  mbedtls_sha256_free(&sha);
  if (done != u.size) {
    esp_ota_abort(handle);
    return ota_fail(u, "download interrupted");
  }
  char hex[65];
  for (int i = 0; i < 32; ++i) snprintf(hex + i * 2, 3, "%02x", digest[i]);
  String want_hash = u.sha256;
  want_hash.toLowerCase();
  if (want_hash != hex) {
    esp_ota_abort(handle);
    return ota_fail(u, "sha256 mismatch");
  }
  if (esp_ota_end(handle) != ESP_OK) return ota_fail(u, "image invalid");
  if (esp_ota_set_boot_partition(target) != ESP_OK) return ota_fail(u, "set boot partition failed");
  detail::sys_set_string(kSysNs, "ota_pend", u.build_id);  // confirmed by the new image
  log::info("ota: %s installed into %s, restarting", u.build_id.c_str(), target->label);
  ui_async([]() { detail::ota_overlay_show(CYD_S_OTA_RESTARTING); });
  delay(1500);
  esp_restart();
}

void heartbeat_done(int code, bool ok, uint32_t interval_s, bool has_update, UpdateOffer offer, const String& err) {
  g_busy = false;
  g_next_heartbeat = millis() + g_interval_ms;
  if (g_cert_changed) {
    log::error("link: AuraGo certificate changed, link stopped until the device is reflashed");
    show_cert_banner();
    return;
  }
  if (!ok) {
    log::warn("link: heartbeat failed (%d %s)", code, err.c_str());
    return;
  }
  if (interval_s < 30) interval_s = 30;
  if (interval_s > 600) interval_s = 600;
  g_interval_ms = interval_s * 1000UL;
  g_next_heartbeat = millis() + g_interval_ms;
  detail::rollback_mark_stable("heartbeat");
  if (g_confirm_pending) {
    g_confirm_pending = false;
    detail::sys_set_string(kSysNs, "ota_pend", "");
    post_ota_result(detail::build_id(), true, "");
  }
  if (g_report_failed_build.length() > 0) {
    post_ota_result(g_report_failed_build, false, "rolled_back");
    g_report_failed_build = "";
  }
  if (has_update && offer.build_id != detail::build_id() &&
      !(offer.build_id == g_backoff_build && (int32_t)(millis() - g_backoff_until) < 0)) {
    g_busy = true;  // cleared by ota_fail; success restarts the device
    g_ota_state = "downloading";
    g_ota_error = "";
    if (!detail::worker_post([offer]() { run_ota(offer); })) {
      g_busy = false;
      g_ota_state = "idle";
    }
  }
}

void send_heartbeat() {
  JsonDocument d;
  d["device_id"] = g_cfg.device_id;
  d["build_id"] = detail::build_id();
  d["template_version"] = CYD_TEMPLATE_VERSION;
  d["uptime_s"] = (uint32_t)(millis() / 1000);
  d["ip"] = WiFi.localIP().toString();
  d["rssi"] = (int)WiFi.RSSI();
  d["free_heap"] = (uint32_t)ESP.getFreeHeap();
  d["reset_reason"] = reset_reason();
  d["ota_state"] = g_ota_state;
  d["ota_error"] = g_ota_error;
  String body;
  serializeJson(d, body);
  g_busy = true;
  bool posted = detail::worker_post([body]() {
    JsonDocument resp;
    String err;
    int code = link_json(true, "/api/cyd-creator/device/heartbeat", body, &resp, &err);
    bool ok = code == 200 && err.length() == 0;
    uint32_t interval = resp["interval_s"] | 60;
    UpdateOffer offer;
    bool has_update = ok && resp["update"].is<JsonObject>();
    if (has_update) {
      offer.build_id = resp["update"]["build_id"] | "";
      offer.size = resp["update"]["size"] | 0;
      offer.sha256 = resp["update"]["sha256"] | "";
      offer.url = resp["update"]["url"] | "";
      has_update = offer.build_id.length() > 0 && offer.sha256.length() == 64;
    }
    ui_async([code, ok, interval, has_update, offer, err]() { heartbeat_done(code, ok, interval, has_update, offer, err); });
  });
  if (!posted) {
    g_busy = false;
    g_next_heartbeat = millis() + 5000;
  }
}
#endif  // CYD_AURAGO_LINK

}  // namespace

namespace detail {

void link_begin() {
#if CYD_AURAGO_LINK
  g_enabled = read_config(&g_cfg);
  if (!g_enabled) return;
  String pend = sys_get_string(kSysNs, "ota_pend");
  String rb_from = sys_get_string(kSysNs, "rb_from");
  if (rb_from.length() > 0 || (pend.length() > 0 && pend != build_id())) {
    // We are the previous image again: the new one never confirmed itself.
    g_report_failed_build = rb_from.length() > 0 ? rb_from : pend;
    g_ota_state = "rolled_back";
    g_ota_error = "rolled back from " + g_report_failed_build;
    sys_set_string(kSysNs, "ota_pend", "");
    sys_set_string(kSysNs, "rb_from", "");
    log::warn("link: %s", g_ota_error.c_str());
  } else if (pend == build_id()) {
    g_confirm_pending = true;  // confirmed after the first good heartbeat
  }
  log::info("link: device %s -> %s", g_cfg.device_id.c_str(), g_cfg.url.c_str());
#endif
}

void link_loop() {
#if CYD_AURAGO_LINK
  if (!g_enabled || g_busy || g_cert_changed || !wifi::connected()) return;
  if (g_next_heartbeat != 0 && (int32_t)(millis() - g_next_heartbeat) < 0) return;
  send_heartbeat();
#endif
}

void ota_overlay_show(const char* text) {
  const ThemeColors& t = ui_theme_colors();
  if (g_overlay == nullptr) {
    g_overlay = lv_obj_create(lv_layer_top());
    lv_obj_set_size(g_overlay, CYD_SCREEN_W, CYD_SCREEN_H);
    lv_obj_set_style_bg_color(g_overlay, t.bg, 0);
    lv_obj_set_style_bg_opa(g_overlay, LV_OPA_90, 0);
    lv_obj_set_style_border_width(g_overlay, 0, 0);
    lv_obj_set_style_radius(g_overlay, 0, 0);
    lv_obj_add_flag(g_overlay, LV_OBJ_FLAG_CLICKABLE);  // swallow touches
    g_overlay_label = make_label(g_overlay, "", t.font_normal, t.text);
    lv_obj_set_width(g_overlay_label, CYD_SCREEN_W - 32);
    lv_obj_set_style_text_align(g_overlay_label, LV_TEXT_ALIGN_CENTER, 0);
    lv_label_set_long_mode(g_overlay_label, LV_LABEL_LONG_MODE_WRAP);
    lv_obj_align(g_overlay_label, LV_ALIGN_CENTER, 0, -18);
    g_overlay_bar = lv_bar_create(g_overlay);
    lv_obj_set_size(g_overlay_bar, CYD_SCREEN_W * 2 / 3, 12);
    lv_obj_align(g_overlay_bar, LV_ALIGN_CENTER, 0, 18);
    lv_obj_set_style_bg_color(g_overlay_bar, t.surface_alt, LV_PART_MAIN);
    lv_obj_set_style_bg_color(g_overlay_bar, t.accent, LV_PART_INDICATOR);
    lv_bar_set_range(g_overlay_bar, 0, 100);
  }
  lv_label_set_text(g_overlay_label, text);
}

void ota_overlay_progress(int pct) {
  if (g_overlay_bar != nullptr) lv_bar_set_value(g_overlay_bar, pct, LV_ANIM_OFF);
}

void ota_overlay_hide() {
  if (g_overlay != nullptr) {
    lv_obj_delete(g_overlay);
    g_overlay = nullptr;
    g_overlay_label = nullptr;
    g_overlay_bar = nullptr;
  }
}

}  // namespace detail
}  // namespace cyd
