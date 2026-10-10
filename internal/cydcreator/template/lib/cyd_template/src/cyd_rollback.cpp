// cyd_rollback.cpp - two independent guards against a bad app image.
//
// 1. RTC boot counter (spec §9): incremented at the very start of setup(),
//    cleared after the first successful heartbeat or 120 s of uptime. At 3
//    unstable boots in a row the other OTA slot is booted (if it holds an app)
//    and "rb_from" in NVS tells that app to report the rollback.
// 2. Bootloader rollback: the Arduino-ESP32 2.0.17 bootloader is built with
//    CONFIG_BOOTLOADER_APP_ROLLBACK_ENABLE. verifyRollbackLater() returning
//    true stops the core from marking a fresh OTA image valid during boot; we
//    call esp_ota_mark_app_valid_cancel_rollback() once the image proved
//    stable. If it resets before that, the bootloader boots the old slot.
#include "cyd_private.h"

#include <esp_attr.h>
#include <esp_ota_ops.h>

// Strong definition overrides the weak one in esp32-hal-misc.c (C linkage).
// The library is linked without an archive (library.json libArchive=false),
// so this object is always part of the link.
extern "C" bool verifyRollbackLater() { return true; }

namespace cyd {
namespace {

constexpr uint32_t kRtcMagic = 0xC7D0B007;
RTC_NOINIT_ATTR uint32_t g_rtc_magic;
RTC_NOINIT_ATTR uint32_t g_rtc_count;
bool g_stable = false;

// Hex of the ELF SHA-256 in an app descriptor: identifies one exact build.
void elf_sha_hex(const esp_app_desc_t& d, char out[65]) {
  for (int i = 0; i < 32; ++i) snprintf(out + i * 2, 3, "%02x", d.app_elf_sha256[i]);
}

}  // namespace

namespace detail {

// Called right before an OTA switch: the running image is what the other slot
// will hold after the reboot, and the only image a rollback may return to.
void rollback_remember_running() {
  const esp_partition_t* running = esp_ota_get_running_partition();
  esp_app_desc_t desc;
  if (running == nullptr || esp_ota_get_partition_description(running, &desc) != ESP_OK) {
    sys_set_string("cyd_sys", "prev_elf", "");
    return;
  }
  char hex[65];
  elf_sha_hex(desc, hex);
  sys_set_string("cyd_sys", "prev_elf", hex);
}

bool rollback_pending_verify() {
  esp_ota_img_states_t st;
  const esp_partition_t* running = esp_ota_get_running_partition();
  return running != nullptr && esp_ota_get_state_partition(running, &st) == ESP_OK &&
         st == ESP_OTA_IMG_PENDING_VERIFY;
}

void rollback_guard_early() {
  esp_reset_reason_t why = esp_reset_reason();
  if (g_rtc_magic != kRtcMagic || why == ESP_RST_POWERON) {
    g_rtc_magic = kRtcMagic;  // RTC memory is random after power-on
    g_rtc_count = 0;
  }
  // Brown-outs and deliberate restarts (our own OTA/rollback restart, esp_restart()
  // in app code) say nothing about the stability of the image.
  if (why != ESP_RST_BROWNOUT && why != ESP_RST_SW) g_rtc_count++;
  const esp_partition_t* running = esp_ota_get_running_partition();
  log::info("boot: slot %s, unstable boots %lu%s", running ? running->label : "?", (unsigned long)g_rtc_count,
            rollback_pending_verify() ? ", image pending verification" : "");
  if (g_rtc_count < CYD_ROLLBACK_THRESHOLD) return;
  g_rtc_count = 0;
  const esp_partition_t* other = esp_ota_get_next_update_partition(nullptr);
  esp_app_desc_t desc;
  if (other == nullptr || other == running || esp_ota_get_partition_description(other, &desc) != ESP_OK) {
    log::warn("boot: %d unstable boots, but the other slot holds no app", CYD_ROLLBACK_THRESHOLD);
    return;
  }
  esp_ota_img_states_t other_state;
  if (esp_ota_get_state_partition(other, &other_state) == ESP_OK &&
      (other_state == ESP_OTA_IMG_INVALID || other_state == ESP_OTA_IMG_ABORTED)) {
    log::warn("boot: %d unstable boots, but the other slot is marked bad", CYD_ROLLBACK_THRESHOLD);
    return;
  }
  // Only the image this device updated from is a rollback target: after a USB
  // flash the other slot may hold a stale build of another project.
  char other_hex[65];
  elf_sha_hex(desc, other_hex);
  if (sys_get_string("cyd_sys", "prev_elf") != other_hex) {
    log::warn("boot: %d unstable boots, but the other slot is not the image this device updated from",
              CYD_ROLLBACK_THRESHOLD);
    return;
  }
  sys_set_string("cyd_sys", "rb_from", build_id());
  if (esp_ota_set_boot_partition(other) == ESP_OK) {
    log::error("boot: %d unstable boots, switching back to %s", CYD_ROLLBACK_THRESHOLD, other->label);
    delay(100);
    esp_restart();
  }
  sys_set_string("cyd_sys", "rb_from", "");
  log::error("boot: rollback to %s failed", other->label);
}

void rollback_mark_stable(const char* why) {
  if (g_stable) return;
  g_stable = true;
  g_rtc_count = 0;
  if (rollback_pending_verify()) {
    esp_err_t err = esp_ota_mark_app_valid_cancel_rollback();
    log::info("boot: image marked valid after %s (%s)", why, esp_err_to_name(err));
  } else {
    log::info("boot: stable after %s", why);
  }
}

void rollback_loop() {
  if (!g_stable && millis() > CYD_STABLE_UPTIME_MS) rollback_mark_stable("120 s uptime");
}

}  // namespace detail
}  // namespace cyd
