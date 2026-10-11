#!/usr/bin/env bash
set -euo pipefail
# Run inside the pinned ESP-IDF image with only a disposable firmware copy mounted.
cd /project
test "$(tr -d '\r\n' < version.txt)" = "0.8.12"
for spec in esp32s3:4 esp32s3:8 esp32c6:4; do
  chip=${spec%:*}; size=${spec#*:}
  for console in uart cdc; do
    id="$chip-${size}mb-$console"
    mkdir -p "out/$id"
    defaults=sdkconfig.defaults
    if [ "$size" = 4 ] && [ "$chip" = esp32s3 ]; then defaults=sdkconfig.defaults.4mb; fi
    cat "$defaults" > "out/$id/sdkconfig.defaults"
    if [ "$chip" = esp32c6 ]; then cat sdkconfig.defaults.esp32c6 >> "out/$id/sdkconfig.defaults"; fi
    # Generic profiles must not drive a display, mmWave accessory or require PSRAM.
    sed -i '/CONFIG_ESP_CONSOLE_/d; /CONFIG_DISPLAY_ENABLE/d; /CONFIG_WASM_ENABLE/d; /CONFIG_SPIRAM/d; /CONFIG_MMWAVE_ENABLE/d' "out/$id/sdkconfig.defaults"
    printf '\nCONFIG_DISPLAY_ENABLE=n\nCONFIG_WASM_ENABLE=n\nCONFIG_SPIRAM=n\nCONFIG_MMWAVE_ENABLE=n\n' >> "out/$id/sdkconfig.defaults"
    if [ "$console" = uart ]; then printf 'CONFIG_ESP_CONSOLE_UART_DEFAULT=y\nCONFIG_ESP_CONSOLE_USB_SERIAL_JTAG=n\n' >> "out/$id/sdkconfig.defaults";
    else printf 'CONFIG_ESP_CONSOLE_USB_SERIAL_JTAG=y\nCONFIG_ESP_CONSOLE_UART_DEFAULT=n\n' >> "out/$id/sdkconfig.defaults"; fi
    idf.py -B "out/$id/build" -D "SDKCONFIG=$PWD/out/$id/sdkconfig" -D "SDKCONFIG_DEFAULTS=$PWD/out/$id/sdkconfig.defaults" -D "IDF_TARGET=$chip" build
  done
done
