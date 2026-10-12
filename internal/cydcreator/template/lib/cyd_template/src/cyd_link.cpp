// cyd_link.cpp - AuraGo link: device config partition, heartbeat, OTA pull.
//
// Config partition "cydcc" (data, subtype 0x41, 4 KiB): "AGCC", version 1,
// byte 5 = 0, little-endian uint16 payload length at bytes 6-7, JSON
// {url, device_id, token, cert_sha256} from byte 8, zero padded.
// HTTPS connections are opened first, the leaf certificate's SHA-256 is
// compared with cert_sha256, and only then is the bearer token sent. Requests
// only ever go to paths below the configured URL, so the token cannot be sent
// to another host.
//
// Threading: the worker task only talks to the server. It never touches LVGL
// or the link state; it hands its results to the loop task (g_result under a
// mutex, g_ota_progress, g_ota_restarting), and link_loop() consumes them. A
// dropped ui_async can therefore never leave the link busy or the overlay up.
#include "cyd_private.h"

#if CYD_AURAGO_LINK
#include <HTTPClient.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <esp_ota_ops.h>
#include <esp_partition.h>
#include <freertos/semphr.h>
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
constexpr uint32_t kBusyWatchdogMs = 15UL * 60UL * 1000UL;  // a job that never reports back
constexpr size_t kMaxResponse = 4096;                       // heartbeat / ota-result answers

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

// What the worker hands back to the loop task (one job in flight at a time).
struct JobResult {
  enum class Kind { None, Heartbeat, OtaFailed };
  Kind kind = Kind::None;
  int code = 0;
  bool ok = false;
  uint32_t interval_s = 60;
  bool has_update = false;
  UpdateOffer offer;  // Heartbeat: the offered update; OtaFailed: the build that failed
  String err;         // Heartbeat: transport error; OtaFailed: the reason
};

LinkConfig g_cfg;
bool g_enabled = false;
bool g_busy = false;  // loop task only: a heartbeat or OTA job is in flight
uint32_t g_busy_deadline = 0;
std::atomic<bool> g_cert_changed{false};  // pin mismatch seen: stop talking to the server
bool g_banner_shown = false;
String g_bad_offer;  // worker task only: the last invalid update offer that was logged
uint32_t g_next_heartbeat = 0;  // millis(); 0 = as soon as Wi-Fi is up
uint32_t g_interval_ms = 60000;
String g_ota_state = "idle";
String g_ota_error;
bool g_confirm_pending = false;  // this boot is the first of a fresh OTA image
String g_report_failed_build;    // build that was rolled back (report once)
String g_backoff_build;          // update that just failed
uint32_t g_backoff_until = 0;

SemaphoreHandle_t g_result_mu = nullptr;
JobResult g_result;
std::atomic<bool> g_result_ready{false};
std::atomic<int> g_ota_progress{-1};  // worker -> overlay bar, -1 = nothing yet
std::atomic<bool> g_ota_restarting{false};
int g_ota_progress_shown = -1;
bool g_ota_restarting_shown = false;

bool is_hex64(const String& s) {  // lowercase, like the server writes it
  if (s.length() != 64) return false;
  for (unsigned i = 0; i < 64; ++i) {
    char c = s[i];
    if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))) return false;
  }
  return true;
}

bool is_build_id(const String& s) {
  if (s.length() == 0 || s.length() > 64) return false;
  for (unsigned i = 0; i < s.length(); ++i) {
    char c = s[i];
    if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '-' ||
          c == '.')) {
      return false;
    }
  }
  return true;
}

void set_busy() {
  g_busy = true;
  g_busy_deadline = millis() + kBusyWatchdogMs;
}

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
  if (https && !is_hex64(out->cert_sha256)) {
    log::error("link: https URL without a valid certificate pin, link disabled");
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

// path must start with "/": it is appended to the configured URL.
int link_open(LinkConn* c, bool post, const String& path, const String& body, String* err) {
  if (path.length() == 0 || path[0] != '/') {
    *err = "bad path";
    return 0;
  }
  String url = g_cfg.url + path;
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
    if (c.http.getSize() > (int)kMaxResponse) {
      *err = "invalid response";
    } else {
      String text;  // the read is capped, also for chunked bodies
      if (!detail::http_read_body(c.http, kMaxResponse, &text) || deserializeJson(*out, text)) {
        *err = "invalid response";
      }
    }
  }
  c.http.end();
  return code;
}

// Worker task: tell the server how an update ended.
void report_ota_result(const String& build_id, bool ok, const String& error) {
  JsonDocument d;
  d["build_id"] = build_id;
  d["ok"] = ok;
  d["error"] = error;
  String body;
  serializeJson(d, body);
  String err;
  int code = link_json(true, "/api/cyd-creator/device/ota-result", body, nullptr, &err);
  log::info("link: ota-result -> %d %s", code, err.c_str());
}

// Loop task: queue the report on the worker.
void post_ota_result(const String& build_id, bool ok, const String& error) {
  detail::worker_post([build_id, ok, error]() { report_ota_result(build_id, ok, error); });
}

// Worker -> loop task.
void publish_result(JobResult r) {
  if (g_result_mu == nullptr) return;
  xSemaphoreTake(g_result_mu, portMAX_DELAY);
  g_result = std::move(r);
  g_result_ready = true;
  xSemaphoreGive(g_result_mu);
}

bool take_result(JobResult* out) {
  if (g_result_mu == nullptr || !g_result_ready.load()) return false;
  xSemaphoreTake(g_result_mu, portMAX_DELAY);
  *out = std::move(g_result);
  g_result = JobResult();
  g_result_ready = false;
  xSemaphoreGive(g_result_mu);
  return true;
}

// Worker task: the update failed; the loop task shows it, the server hears it now.
void ota_fail(const UpdateOffer& u, const String& why) {
  log::error("ota: %s failed: %s", u.build_id.c_str(), why.c_str());
  JobResult r;
  r.kind = JobResult::Kind::OtaFailed;
  r.offer = u;
  r.err = why;
  publish_result(std::move(r));
  report_ota_result(u.build_id, false, why);
}

// Loop task.
void show_cert_banner() {
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
}

// The download path of an offer. It may be a path below the configured AuraGo
// URL or an absolute URL below it; anything else could carry the bearer token
// to another host and is replaced by the default path.
String firmware_path(const UpdateOffer& u) {
  String fallback = "/api/cyd-creator/device/firmware/" + u.build_id;
  if (u.url.length() == 0) return fallback;
  String p = u.url;
  if (p.startsWith(g_cfg.url + "/")) p = p.substring(g_cfg.url.length());  // "https://host/x" -> "/x"
  bool ok = p.length() > 1 && p.length() <= 256 && p[0] == '/' && p[1] != '/';
  for (unsigned i = 0; ok && i < p.length(); ++i) ok = p[i] >= 0x21 && p[i] <= 0x7E;
  if (!ok) {
    log::warn("ota: ignoring the offered url (not a path below the AuraGo URL), using the default path");
    return fallback;
  }
  return p;
}

// Runs in the worker task: download into the inactive slot, verify, switch.
void run_ota(UpdateOffer u) {
  const esp_partition_t* target = esp_ota_get_next_update_partition(nullptr);
  if (target == nullptr) return ota_fail(u, "no ota partition");
  if (u.size == 0 || u.size > target->size) return ota_fail(u, "image does not fit the app slot");
  // Erase the slot before the download starts: with the connection open, the
  // erase would leave it idle for seconds.
  esp_ota_handle_t handle = 0;
  if (esp_ota_begin(target, u.size, &handle) != ESP_OK) return ota_fail(u, "esp_ota_begin failed");

  LinkConn c;
  String err;
  int code = link_open(&c, false, firmware_path(u), String(), &err);
  if (code != 200) {
    if (code > 0) c.http.end();
    esp_ota_abort(handle);
    return ota_fail(u, code > 0 ? "http " + String(code) : err);
  }
  if (c.http.getSize() != (int)u.size) {
    c.http.end();
    esp_ota_abort(handle);
    return ota_fail(u, "size mismatch");
  }
  WiFiClient* stream = c.http.getStreamPtr();  // null when the server already closed the connection
  if (stream == nullptr) {
    c.http.end();
    esp_ota_abort(handle);
    return ota_fail(u, "connection lost");
  }
  mbedtls_sha256_context sha;
  mbedtls_sha256_init(&sha);
  mbedtls_sha256_starts(&sha, 0);
  std::unique_ptr<uint8_t[]> buf(new uint8_t[CYD_OTA_CHUNK]);
  uint32_t done = 0;
  uint32_t last_data = millis();
  int last_pct = -1;
  bool write_failed = false;
  while (done < u.size && millis() - last_data <= 15000) {
    int avail = stream->available();
    if (avail <= 0) {
      if (!stream->connected()) break;
      delay(5);
      continue;
    }
    size_t want = u.size - done;
    if (want > CYD_OTA_CHUNK) want = CYD_OTA_CHUNK;
    if (want > (size_t)avail) want = (size_t)avail;
    int got = stream->read(buf.get(), want);
    if (got <= 0) {
      delay(1);
      continue;
    }
    if (esp_ota_write(handle, buf.get(), got) != ESP_OK) {
      write_failed = true;
      break;
    }
    mbedtls_sha256_update(&sha, buf.get(), got);
    done += got;
    last_data = millis();
    int pct = (int)((uint64_t)done * 100 / u.size);
    if (pct != last_pct) {
      last_pct = pct;
      g_ota_progress = pct;  // the loop task draws it
    }
    delay(1);  // let the idle task run (task watchdog)
  }
  c.http.end();
  uint8_t digest[32];
  mbedtls_sha256_finish(&sha, digest);
  mbedtls_sha256_free(&sha);
  if (write_failed) {
    esp_ota_abort(handle);
    return ota_fail(u, "flash write failed");
  }
  if (done != u.size) {
    esp_ota_abort(handle);
    return ota_fail(u, "download interrupted");
  }
  char hex[65];
  for (int i = 0; i < 32; ++i) snprintf(hex + i * 2, 3, "%02x", digest[i]);
  if (u.sha256 != hex) {  // the offer's hash is validated as lowercase hex
    esp_ota_abort(handle);
    return ota_fail(u, "sha256 mismatch");
  }
  if (esp_ota_end(handle) != ESP_OK) return ota_fail(u, "image invalid");
  detail::rollback_remember_update();                      // ties a rollback to this OTA
  detail::sys_set_string(kSysNs, "ota_pend", u.build_id);  // confirmed by the new image
  if (esp_ota_set_boot_partition(target) != ESP_OK) {
    detail::sys_set_string(kSysNs, "ota_pend", "");
    return ota_fail(u, "set boot partition failed");
  }
  log::info("ota: %s installed into %s, restarting", u.build_id.c_str(), target->label);
  g_ota_restarting = true;  // the loop task shows it
  delay(1500);
  esp_restart();
}

// Loop task: start the download of an offered update.
void start_ota(const UpdateOffer& offer) {
  g_ota_progress = -1;
  g_ota_progress_shown = -1;
  g_ota_restarting = false;
  g_ota_restarting_shown = false;
  set_busy();
  g_ota_state = "downloading";
  g_ota_error = "";
  detail::ota_overlay_show(CYD_S_OTA_INSTALLING);
  detail::ota_overlay_progress(0);
  if (!detail::worker_post([offer]() { run_ota(offer); })) {
    g_busy = false;
    g_ota_state = "idle";
    detail::ota_overlay_hide();
  }
}

// Loop task.
void heartbeat_done(const JobResult& r) {
  g_busy = false;
  g_next_heartbeat = millis() + g_interval_ms;
  if (g_cert_changed) return;  // link_loop() shows the banner and the link stays down
  if (!r.ok) {
    log::warn("link: heartbeat failed (%d %s)", r.code, r.err.c_str());
    return;
  }
  uint32_t interval_s = r.interval_s;
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
  if (r.has_update && r.offer.build_id != detail::build_id() &&
      !(r.offer.build_id == g_backoff_build && (int32_t)(millis() - g_backoff_until) < 0)) {
    start_ota(r.offer);
  }
}

// Loop task: the worker reported a failed update.
void ota_failed_done(const JobResult& r) {
  g_busy = false;
  g_ota_progress = -1;
  g_ota_state = "failed";
  g_ota_error = r.err;
  g_backoff_build = r.offer.build_id;
  g_backoff_until = millis() + 10UL * 60UL * 1000UL;
  detail::ota_overlay_show(CYD_S_OTA_FAILED);
  lv_timer_t* t = lv_timer_create([](lv_timer_t*) { detail::ota_overlay_hide(); }, 3000, nullptr);
  lv_timer_set_repeat_count(t, 1);
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
  set_busy();
  bool posted = detail::worker_post([body]() {
    JsonDocument resp;
    JobResult r;
    r.kind = JobResult::Kind::Heartbeat;
    r.code = link_json(true, "/api/cyd-creator/device/heartbeat", body, &resp, &r.err);
    r.ok = r.code == 200 && r.err.length() == 0;
    r.interval_s = resp["interval_s"] | 60;
    bool offered = r.ok && !resp["update"].isNull();
    r.has_update = offered && resp["update"].is<JsonObject>();
    if (r.has_update) {
      r.offer.build_id = resp["update"]["build_id"] | "";
      r.offer.size = resp["update"]["size"] | 0;
      r.offer.sha256 = resp["update"]["sha256"] | "";
      r.offer.url = resp["update"]["url"] | "";
      r.offer.sha256.toLowerCase();
      r.has_update = is_build_id(r.offer.build_id) && is_hex64(r.offer.sha256);
    }
    if (offered && !r.has_update) {  // log each bad offer once, not at every heartbeat
      String key = r.offer.build_id.substring(0, 80) + "/" + r.offer.sha256.substring(0, 80);
      if (key != g_bad_offer) {
        g_bad_offer = key;
        log::warn("link: ignoring an invalid update offer (build_id %s, sha256 %s)",
                  is_build_id(r.offer.build_id) ? "valid" : "invalid", is_hex64(r.offer.sha256) ? "valid" : "invalid");
      }
    }
    publish_result(std::move(r));
  });
  if (!posted) {
    g_busy = false;
    g_next_heartbeat = millis() + 5000;
  }
}

// Loop task: draw what the worker reports about a running update.
void poll_ota_ui() {
  int pct = g_ota_progress.load();
  if (pct >= 0 && pct != g_ota_progress_shown) {
    g_ota_progress_shown = pct;
    detail::ota_overlay_progress(pct);
  }
  if (g_ota_restarting.load() && !g_ota_restarting_shown) {
    g_ota_restarting_shown = true;
    detail::ota_overlay_show(CYD_S_OTA_RESTARTING);
  }
}

// Loop task: a job that never reports back must not block the link for good.
void check_busy_watchdog() {
  if (!g_busy || (int32_t)(millis() - g_busy_deadline) < 0) return;
  log::error("link: a server job did not finish within 15 minutes, link state reset");
  g_busy = false;
  g_next_heartbeat = millis() + g_interval_ms;
  if (g_ota_state == "downloading") {
    g_ota_state = "failed";
    g_ota_error = "timeout";
    detail::ota_overlay_hide();
  }
}
#endif  // CYD_AURAGO_LINK

}  // namespace

namespace detail {

void link_begin() {
#if CYD_AURAGO_LINK
  g_enabled = read_config(&g_cfg);
  if (!g_enabled) return;
  g_result_mu = xSemaphoreCreateMutex();
  if (g_result_mu == nullptr) {
    log::error("link: out of memory, link disabled");
    g_enabled = false;
    return;
  }
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
  if (!g_enabled) return;
  poll_ota_ui();
  JobResult r;
  if (take_result(&r)) {
    if (r.kind == JobResult::Kind::Heartbeat) heartbeat_done(r);
    if (r.kind == JobResult::Kind::OtaFailed) ota_failed_done(r);
  }
  if (g_cert_changed && !g_banner_shown) {
    g_banner_shown = true;
    log::error("link: AuraGo certificate changed, link stopped until the device is reflashed");
    show_cert_banner();
  }
  check_busy_watchdog();
  if (g_busy || g_cert_changed || !wifi::connected()) return;
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
