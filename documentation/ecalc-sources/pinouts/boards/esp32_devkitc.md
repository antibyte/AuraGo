# esp32_devkitc — ESP32-DevKitC V4 headers / ESP32-DevKitC V4 Stiftleisten

Kind: authoritative maker document
Pins: 38; types: power 2; ground 3; signal 1; data 6; io 26

## View
Board from the component side, portrait, Micro-USB at the top (`connector: top`, as planned), which is Espressif's figure turned by 180°. Basis:
- Espressif's pin layout figure (`esp32_devkitC_v4_pinlayout.png`, downloaded and looked at) shows the module and antenna at the top and the USB socket at the bottom, with pin labels printed beside the pins: left top to bottom 3V3, EN, VP, VN, 34, 35, 32, 33, 25, 26, 27, 14, 12, GND, 13, D2, D3, CMD, 5V; right top to bottom GND, 23, 22, TX, RX, 21, GND, 19, 18, 5, 17, 16, 4, 0, 2, 15, D1, D0, CLK.
- These two orders are exactly tables J2 and J3 of the guide, and the schematic's J2 and J3 (pins 1–19, zoomed render).
- Espressif's photo (`esp32-devkitc-v4-functional-overview.png`) shows the real board's silk: USB left, antenna right; upper row from the USB end 5V, CMD, D3 …, 3V3; lower row from the USB end CLK, D0, D1, 15 …, GND. Turned so that USB points up, the lower row becomes the left column and the upper row the right one.

Turning by 180° gives the lists: left J3.19 (CLK) … J3.1 (GND), right J2.19 (5V) … J2.1 (3V3). A rotation, not a mirror: the module side stays towards the viewer.

## Sources
1. Espressif, ESP32-DevKitC V4 User Guide (esp-dev-kits, latest), https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32/esp32-devkitc/user_guide.html — tables J2 and J3, notes on flash and GPIO16/17, power warning, accessed 2026-11-04
2. Espressif, pin layout figure of that guide, https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32/_images/esp32_devkitC_v4_pinlayout.png — positions, input-only marks, strapping marks, SPI and Arduino functions, accessed 2026-11-04
3. Espressif, functional overview photo of that guide, https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32/_images/esp32-devkitc-v4-functional-overview.png — silk screen and orientation, accessed 2026-11-04
4. Espressif, ESP32_DevKitc_V4 schematic (Dec 2017), https://dl.espressif.com/dl/schematics/esp32_devkitc_v4-sch.pdf — connectors J2 and J3, accessed 2026-11-04
5. Espressif, ESP32 Series Datasheet v5.3, https://www.espressif.com/sites/default/files/documentation/esp32_datasheet_en.pdf — section 2 pin overview, section 3 strapping pins (tables 3-1, 3-3, 3-4, section 3.2), table 5-3 VIH, accessed 2026-11-04

## Cross-check
Guide tables, figure and schematic agree on all 38 pins. Details:
- The guide's text does not call any pin strapping or input-only. Input-only comes from the figure's arrows and the type `I` in the tables; strapping pins come from the datasheet's section 3 (GPIO0, GPIO2, MTDI = GPIO12, MTDO = GPIO15, GPIO5).
- The figure gives STRAP labels to GPIO0, GPIO12, GPIO15 and GPIO5 but none to GPIO2. The datasheet lists GPIO2 as strapping (boot mode), so I mark it.
- The figure labels GPIO12 as `VDD_FLASH`, GPIO15 as `LOG` and GPIO5 as `SDIO`, matching the datasheet's 3.2, 3.3 and 3.4.
- The guide says D0–D3, CMD, CLK "may disrupt" flash access; the figure says they cannot be used as regular GPIO. The notes use the figure's stricter wording.
- UART2 on GPIO16/17 is from the datasheet's pin list, not from the guide.
- The Arduino I2C names (`WIRE_SDA`, `WIRE_SCL`) and `VSPI`/`HSPI` come from the figure. I2C has no fixed hardware pin on the chip, so `function` says "in the Arduino core".
No UNVERIFIED pin.

## Choices
- `connector: top` as planned, although Espressif draws the board with USB at the bottom; the doc text says so, because a reader comparing with Espressif's picture will see it turned.
- Names are `GPIOnn` for every GPIO as planned, so GPIO36 instead of the silk `VP`, GPIO1 instead of `TX`, GPIO9 instead of `D2`. The silk label stands in `function`.
- GPIO6–GPIO11 are type `data`, not `io`: Espressif's table calls them I/O, but the lines are the SPI flash bus and a user scanning for free (green) pins must not take them. The catalog schema's `data` covers bus lines.
- GPIO16 and GPIO17 stay `io` with a note, because they are free on WROOM boards.
- The drawing covers the V4 with any of the listed modules (WROOM-32E/UE, WROVER-E/IE); the WROVER difference is GPIO16/17.
