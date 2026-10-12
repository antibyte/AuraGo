// cyd_http.cpp - the worker task for blocking network work and the async JSON
// helpers of the app API. Results are handed back with ui_async, so app
// callbacks run on the loop task and may touch LVGL.
//
// App requests to https:// URLs are encrypted but the server certificate is
// not verified (no CA bundle on 4 MB boards); never send secrets through them.
// The AuraGo link uses its own pinned connection (cyd_link.cpp).
#include "cyd_private.h"

#include <HTTPClient.h>
#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <freertos/queue.h>

#include <memory>

namespace cyd {
namespace {

QueueHandle_t g_jobs = nullptr;
bool g_worker_failed = false;  // the task could not be created: every job fails fast

void worker_main(void*) {
  for (;;) {
    std::function<void()>* job = nullptr;
    if (xQueueReceive(g_jobs, &job, portMAX_DELAY) == pdTRUE && job != nullptr) {
      (*job)();
      delete job;
    }
  }
}

// Collects a response body up to a byte limit (HTTPClient decodes chunking).
class CapStream : public Stream {
 public:
  explicit CapStream(size_t cap) : cap_(cap) {}
  size_t write(uint8_t c) override { return write(&c, 1); }
  size_t write(const uint8_t* buf, size_t len) override {
    if (body.length() + len > cap_) {
      over = true;
      return 0;  // makes HTTPClient stop reading
    }
    body.concat((const char*)buf, len);
    return len;
  }
  int available() override { return 0; }
  int read() override { return -1; }
  int peek() override { return -1; }
  void flush() override {}
  String body;
  bool over = false;

 private:
  size_t cap_;
};

struct Response {
  int status = 0;
  String body;
  String error;
};

Response do_request(bool post, const String& url, const String& body, uint32_t timeout_ms) {
  Response r;
  if (timeout_ms > 65535) timeout_ms = 65535;  // HTTPClient::setTimeout takes a uint16_t
  if (!wifi::connected()) {
    r.error = "offline";
    return r;
  }
  std::unique_ptr<WiFiClient> client;
  if (url.startsWith("https://")) {
    auto* tls = new WiFiClientSecure();
    tls->setInsecure();
    tls->setHandshakeTimeout((timeout_ms + 999) / 1000);
    client.reset(tls);
  } else if (url.startsWith("http://")) {
    client.reset(new WiFiClient());
  } else {
    r.error = "unsupported url";
    return r;
  }
  HTTPClient http;
  http.setReuse(false);
  http.setTimeout(timeout_ms);
  http.setConnectTimeout(timeout_ms);
  http.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);
  if (!http.begin(*client, url)) {
    r.error = "bad url";
    return r;
  }
  http.addHeader("Accept", "application/json");
  int code;
  if (post) {
    http.addHeader("Content-Type", "application/json");
    code = http.POST(body);
  } else {
    code = http.GET();
  }
  if (code <= 0) {
    r.error = HTTPClient::errorToString(code);
    http.end();
    return r;
  }
  r.status = code;
  int size = http.getSize();
  if (size > CYD_HTTP_MAX_BODY) {
    r.error = "response too large";
    http.end();
    return r;
  }
  CapStream sink(CYD_HTTP_MAX_BODY);
  http.writeToStream(&sink);
  http.end();
  if (sink.over) {
    r.error = "response too large";
    return r;
  }
  r.body = sink.body;
  if (code < 200 || code > 299) r.error = "http " + String(code);
  return r;
}

void run_json(bool post, const String& url, const String& body, http::JsonCallback cb, uint32_t timeout_ms) {
  auto job = [post, url, body, cb, timeout_ms]() {
    Response res = do_request(post, url, body, timeout_ms);
    auto doc = std::make_shared<JsonDocument>();
    String err = res.error;
    if (res.body.length() > 0) {
      DeserializationError de = deserializeJson(*doc, res.body);
      if (de && err.length() == 0) err = String("invalid json: ") + de.c_str();
    } else if (err.length() == 0) {
      err = "empty response";
    }
    int status = res.status;
    ui_async([cb, status, doc, err]() { cb(status, *doc, err); });
  };
  if (!detail::worker_post(job)) {
    ui_async([cb]() {
      JsonDocument empty;
      cb(0, empty, g_worker_failed ? "worker unavailable" : "busy");
    });
  }
}

}  // namespace

namespace detail {

void worker_begin() {
  if (g_jobs != nullptr || g_worker_failed) return;
  g_jobs = xQueueCreate(CYD_HTTP_QUEUE_LEN, sizeof(std::function<void()>*));
  // 12 KiB: an mbedTLS handshake needs ~7 KiB of stack.
  if (g_jobs == nullptr ||
      xTaskCreatePinnedToCore(worker_main, "cyd_worker", 12288, nullptr, 1, nullptr, 0) != pdPASS) {
    log::error("http: could not start the worker task (free heap %lu), network requests will fail",
               (unsigned long)ESP.getFreeHeap());
    if (g_jobs != nullptr) vQueueDelete(g_jobs);
    g_jobs = nullptr;  // worker_post() now fails fast
    g_worker_failed = true;
  }
}

// For the AuraGo link: the same capped read, so a chunked body cannot exhaust the heap.
bool http_read_body(HTTPClient& http, size_t cap, String* out) {
  CapStream sink(cap);
  http.writeToStream(&sink);
  if (sink.over) return false;
  *out = sink.body;
  return true;
}

bool worker_post(std::function<void()> job) {
  if (g_jobs == nullptr) return false;
  auto* heap_job = new std::function<void()>(std::move(job));
  if (xQueueSend(g_jobs, &heap_job, 0) != pdTRUE) {
    delete heap_job;
    return false;
  }
  return true;
}

}  // namespace detail

namespace http {

void get_json_async(const String& url, JsonCallback cb, uint32_t timeout_ms) {
  run_json(false, url, String(), std::move(cb), timeout_ms);
}

void post_json_async(const String& url, const JsonDocument& body, JsonCallback cb, uint32_t timeout_ms) {
  String payload;
  serializeJson(body, payload);
  run_json(true, url, payload, std::move(cb), timeout_ms);
}

}  // namespace http
}  // namespace cyd
