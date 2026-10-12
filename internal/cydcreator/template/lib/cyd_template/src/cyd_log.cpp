// cyd_log.cpp - serial logging ("[  12.345][I] message") and the LVGL log hook.
#include "cyd_private.h"

#include <stdarg.h>

namespace cyd {
namespace detail {

void logf(char level, const char* fmt, va_list ap) {
  char buf[256];
  int n = vsnprintf(buf, sizeof(buf), fmt, ap);
  if (n < 0) return;
  uint32_t ms = millis();
  Serial.printf("[%6lu.%03lu][%c] %s%s\n", (unsigned long)(ms / 1000), (unsigned long)(ms % 1000), level, buf,
                n >= (int)sizeof(buf) ? "..." : "");
}

}  // namespace detail

namespace log {

void info(const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  detail::logf('I', fmt, ap);
  va_end(ap);
}

void warn(const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  detail::logf('W', fmt, ap);
  va_end(ap);
}

void error(const char* fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  detail::logf('E', fmt, ap);
  va_end(ap);
}

}  // namespace log
}  // namespace cyd
