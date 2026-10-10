// cyd_storage.cpp - NVS access. App values live in namespace "app"; the
// runtime uses "cyd_wifi" (credentials) and "cyd_sys" (calibration, OTA state).
// Every call opens and closes its namespace under one mutex, so the API is
// safe from any task.
#include "cyd_private.h"

#include <Preferences.h>

namespace cyd {
namespace {

SemaphoreHandle_t lock_handle() {
  static StaticSemaphore_t buf;
  static SemaphoreHandle_t h = xSemaphoreCreateMutexStatic(&buf);  // C++11 thread-safe init
  return h;
}

struct Locked {
  Locked() { xSemaphoreTake(lock_handle(), portMAX_DELAY); }
  ~Locked() { xSemaphoreGive(lock_handle()); }
};

const char* const kAppNs = "app";

}  // namespace

namespace storage {

int32_t get_int(const char* key, int32_t def) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, true)) return def;
  int32_t v = p.getInt(key, def);
  p.end();
  return v;
}

void set_int(const char* key, int32_t v) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, false)) return;
  p.putInt(key, v);
  p.end();
}

float get_float(const char* key, float def) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, true)) return def;
  float v = p.getFloat(key, def);
  p.end();
  return v;
}

void set_float(const char* key, float v) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, false)) return;
  p.putFloat(key, v);
  p.end();
}

bool get_bool(const char* key, bool def) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, true)) return def;
  bool v = p.getBool(key, def);
  p.end();
  return v;
}

void set_bool(const char* key, bool v) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, false)) return;
  p.putBool(key, v);
  p.end();
}

String get_string(const char* key, const String& def) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, true)) return def;
  String v = p.isKey(key) ? p.getString(key, def) : def;
  p.end();
  return v;
}

void set_string(const char* key, const String& v) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, false)) return;
  p.putString(key, v);
  p.end();
}

void remove(const char* key) {
  Locked l;
  Preferences p;
  if (!p.begin(kAppNs, false)) return;
  p.remove(key);
  p.end();
}

}  // namespace storage

namespace detail {

bool sys_get_blob(const char* ns, const char* key, void* out, size_t len) {
  Locked l;
  Preferences p;
  if (!p.begin(ns, true)) return false;
  bool ok = p.isKey(key) && p.getBytesLength(key) == len && p.getBytes(key, out, len) == len;
  p.end();
  return ok;
}

void sys_set_blob(const char* ns, const char* key, const void* data, size_t len) {
  Locked l;
  Preferences p;
  if (!p.begin(ns, false)) return;
  p.putBytes(key, data, len);
  p.end();
}

String sys_get_string(const char* ns, const char* key) {
  Locked l;
  Preferences p;
  if (!p.begin(ns, true)) return String();
  String v = p.isKey(key) ? p.getString(key, "") : String();
  p.end();
  return v;
}

void sys_set_string(const char* ns, const char* key, const String& value) {
  Locked l;
  Preferences p;
  if (!p.begin(ns, false)) return;
  if (value.length() == 0) {
    p.remove(key);
  } else {
    p.putString(key, value);
  }
  p.end();
}

void sys_clear(const char* ns) {
  Locked l;
  Preferences p;
  if (!p.begin(ns, false)) return;
  p.clear();
  p.end();
}

}  // namespace detail
}  // namespace cyd
