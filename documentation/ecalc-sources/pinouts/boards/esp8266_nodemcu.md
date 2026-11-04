# esp8266_nodemcu — NodeMCU DevKit 1.0 headers / NodeMCU DevKit 1.0 Stiftleisten

Kind: authoritative maker document
Pins: 30; types: power 4; ground 4; signal 3; data 4; io 13; nc 2

## View
Board from the component side, portrait, Micro-USB at the top (`connector: top`, as planned), which is the project's pin map turned by 180°. Basis:
- The pin map `NODEMCU_DEVKIT_V1.0_PINMAP.png` (downloaded and looked at) shows the board with the chamfered antenna end up: left column A0, RSV, RSV, SD3, SD2, SD1, CMD, SD0, CLK, GND, 3V3, EN, RST, GND, Vin; right column D0, D1, D2, D3, D4, 3V3, GND, D5, D6, D7, D8, D9, D10, GND, 3V3. Both orders equal J1 and J2 of the schematic's IO CONN sheet, pin 1 at the antenna end.
- The figure itself does not state the side. The photo `NodeMCU_DEVKIT_1.0.jpg` in the same repository settles it: with the ESP-12 module facing the camera and the antenna end up, the silk RSV, RSV, SD3, SD2, SD1, CMD, SD0, CLK, GND, 3V, EN … is on the left and … D5, D6, D7, D8, RX, TX, GND, 3V on the right, so the pin map is the component side. I zoomed into both rows.

Turning by 180° gives: left J2.15 (3V3) … J2.1 (D0), right J1.15 (VIN) … J1.1 (A0). A rotation, not a mirror.

Revision drawn: NodeMCU DEVKIT V1.0 (`NODEMCU_DEVKIT_V1.0.PDF`), ESP-12E core, per the repository README.

## Sources
1. NodeMCU team, `NODEMCU_DEVKIT_V1.0.PDF` schematic, https://raw.githubusercontent.com/nodemcu/nodemcu-devkit-v1.0/master/NODEMCU_DEVKIT_V1.0.PDF — sheets ESP-12 CORE, IO CONN, KEY, ADC rendered and read, accessed 2026-11-04
2. NodeMCU team, pin map, https://raw.githubusercontent.com/nodemcu/nodemcu-devkit-v1.0/master/Documents/NODEMCU_DEVKIT_V1.0_PINMAP.png, and the same map in `NODEMCU-DEVKIT-V1.0-INSTRUCTION-EN.pdf` (V1.0, 2015-1-28), https://raw.githubusercontent.com/nodemcu/nodemcu-devkit-v1.0/master/Documents/NODEMCU-DEVKIT-V1.0-INSTRUCTION-EN.pdf, accessed 2026-11-04
3. NodeMCU team, board photo, https://raw.githubusercontent.com/nodemcu/nodemcu-devkit-v1.0/master/Documents/NodeMCU_DEVKIT_1.0.jpg, and repository README, https://raw.githubusercontent.com/nodemcu/nodemcu-devkit-v1.0/master/README.md, accessed 2026-11-04
4. NodeMCU firmware documentation, modules gpio, i2c, pwm, spi, https://raw.githubusercontent.com/nodemcu/nodemcu-firmware/release/docs/modules/gpio.md (and i2c.md, pwm.md, spi.md in the same folder) — IO index table, HSPI pins, accessed 2026-11-04
5. Espressif, ESP8266EX Datasheet v7.1, https://www.espressif.com/sites/default/files/documentation/0a-esp8266ex_datasheet_en.pdf — tables 2-1, 4-2, 4-3, 4-4, 4-6, 4-7, 5-1, section 4.9, accessed 2026-11-04. Espressif publishes the chip data; the ESP-12E module itself is an Ai-Thinker part, and I did not read a module datasheet.
6. ESP8266 Arduino core documentation, Libraries, https://arduino-esp8266.readthedocs.io/en/latest/libraries.html — default I2C pins, accessed 2026-11-04

## Cross-check
Schematic, pin map and firmware table agree on every D-pin. Disagreements:
- **D1/D2.** `Documents/新版本管脚对应关系.txt` in the devkit repository lists D1 GPIO4 and D2 GPIO5. Schematic J2, pin map and firmware `gpio.md` all say D1 GPIO5, D2 GPIO4. I chose the three documents that agree over the single text file.
- **SD pins.** The schematic labels SD_CMD as SPI_MOSI and SD_D1 as SPI_INT; the pin map and Espressif's tables 2-1 and 4-2 say SD_CMD = SPI_CS0 and SD_D1 = SPI_MOSI. I follow Espressif, as they are its chip's pins. Nobody should wire these anyway.
- **J1.2.** The pin map calls it RSV (RESERVED); the schematic connects it to the `ADC` net behind the divider. Name and type follow the pin map, the pin's note states the schematic's fact.
- **D0 and I2C.** The pin map's footnote and `gpio.md` exclude I2C on D0; `i2c.md` allows GPIO16 as SCL with a build option. The note follows the footnote.
- **I2C pins.** Espressif's table 4-4 names GPIO14 and GPIO2; the firmware takes any IO index, with an example using pin 1 as SDA and pin 2 as SCL; the Arduino core defaults to GPIO4 SDA and GPIO5 SCL. The functions name only the Arduino default, labelled as such; the notes say that it is software and that the firmware's examples differ.
- **Title blocks.** The IO CONN, KEY and ADC sheets still say VER 0.9, the ESP-12 CORE sheet says 1.0, all in the same `V1.0` PDF. The pin map of the same revision agrees with them, so I treat them as the V1.0 board.
- The A0 range "3.2 V" is my calculation from the schematic (220 kΩ + 100 kΩ) and the datasheet's 1.0 V limit on TOUT, not a figure printed in a source.
No UNVERIFIED pin.

## Choices
- `RX` and `TX` as names (silk in the photo); the pin map's D9 and D10 are in `function`.
- `3V3` for the silk `3V`, `VIN` for `Vin`, as in the pin map.
- SD3 and SD2 are `io`, because the pin map calls them GPIO10 and GPIO9 and the firmware gives them IO indexes 12 and 11. SD1, CMD, SD0 and CLK are `data`: the pin map gives them only SPI and SD roles and they are the flash bus.
- A0 is `signal`, not `io`: it is an ADC input only, unlike the A pins of an Arduino.
- The two RSV pins are `nc`, as `reserved` is in that type by the catalog schema.
- "Clones differ" is stated in general terms as planned; no source lists which clone differs how, so I name no clone.
- The LED on D0 is from the KEY sheet (LED1 from 3.3 V through 470 Ω to GPIO16). Any LED of the ESP-12E module itself is not in the sources read and is left out.
