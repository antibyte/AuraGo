# esp32_s3_devkitc1 — ESP32-S3-DevKitC-1 v1.1 headers / ESP32-S3-DevKitC-1 v1.1 Stiftleisten

Kind: authoritative maker document (Espressif user guide v1.1, pin layout figure v1.1, annotated photo, schematic V1.1; ESP32-S3 datasheet for chip limits), board ESP32-S3-DevKitC-1 / -1U v1.1 (modules ESP32-S3-WROOM-1, -1U or -2)
Pins: 44; types: power 3; ground 4; signal 1; data 3; io 33; nc 0
Status: complete (pin positions); one caution about GPIO47/48 on 1.8 V variants left partly unverified

## View
Board from the component side, portrait. Espressif's pin layout figure draws the module (antenna) at the top and the two Micro-USB sockets at the bottom (UART left, USB right): left column J1.1 (3V3, top) … J1.22 (GND, bottom), right column J3.1 (GND, top) … J3.22 (GND, bottom). Basis:
- Pin layout figure `ESP32-S3_DevKitC-1_pinlayout_v1.1.jpg` (looked at): left 3V3, 3V3, RST, GPIO4, 5, 6, 7, 15, 16, 17, 18, 8, 3, 46, 9, 10, 11, 12, 13, 14, 5V0, GND; right GND, GPIO43 (U0TXD), GPIO44 (U0RXD), GPIO1, 2, 42, 41, 40, 39, 38 (RGB_LED), 37, 36, 35, 0 (BOOT), 45, 48, 47, 21, 20 (USB_D+), 19 (USB_D-), GND, GND; STRAP marks on GPIO3 (JTAG), GPIO46 (LOG), GPIO0 (BOOT), GPIO45 (VSPI); "RGB@IO38" printed beside the LED.
- Annotated photo `ESP32-S3-DevKitC-1_v2-annotated-photo.png` ("front", zoomed): landscape with the module left; lower row silk 3V3, 3V3, RST, 4, 5, 6, 7, 15, 16, 17, 18, 8, 3, 46, 9, 10, 11, 12, 13, 14, 5V, G; upper row G, TX, RX, 1, 2, 42, 41, 40, 39, 38, 37, 36, 35, 0, 45, 48, 47, 21, 20, 19, G, G. Turned 90° clockwise (module up) the lower row is the left column, the upper row the right one, and the USB-to-UART socket ends up lower left, the USB (OTG) socket lower right — as in the figure.

Lists as Espressif draws it (`connector: bottom`):
- left (top to bottom): J1.1 … J1.22
- right (top to bottom): J3.1 … J3.22
For `connector: top`, turn by 180°: left J3.22 … J3.1, right J1.22 … J1.1 (rotation, not mirror; the UART socket is then upper right, the USB socket upper left).

## Pins
| position | label | function en | function de | type |
|---|---|---|---|---|
| J1.1 | 3V3 | 3.3 V supply (LDO output, or input) | 3,3-V-Versorgung (LDO-Ausgang oder Eingang) | power |
| J1.2 | 3V3 | 3.3 V supply (LDO output, or input) | 3,3-V-Versorgung (LDO-Ausgang oder Eingang) | power |
| J1.3 | RST | EN (CHIP_PU); low resets; RESET button | EN (CHIP_PU); low setzt zurück; RESET-Taster | signal |
| J1.4 | GPIO4 | TOUCH4; ADC1_CH3 (RTC_GPIO4) | TOUCH4; ADC1_CH3 (RTC_GPIO4) | io |
| J1.5 | GPIO5 | TOUCH5; ADC1_CH4 (RTC_GPIO5) | TOUCH5; ADC1_CH4 (RTC_GPIO5) | io |
| J1.6 | GPIO6 | TOUCH6; ADC1_CH5 (RTC_GPIO6) | TOUCH6; ADC1_CH5 (RTC_GPIO6) | io |
| J1.7 | GPIO7 | TOUCH7; ADC1_CH6 (RTC_GPIO7) | TOUCH7; ADC1_CH6 (RTC_GPIO7) | io |
| J1.8 | GPIO15 | U0RTS; ADC2_CH4; XTAL_32K_P | U0RTS; ADC2_CH4; XTAL_32K_P | io |
| J1.9 | GPIO16 | U0CTS; ADC2_CH5; XTAL_32K_N | U0CTS; ADC2_CH5; XTAL_32K_N | io |
| J1.10 | GPIO17 | U1TXD; ADC2_CH6 | U1TXD; ADC2_CH6 | io |
| J1.11 | GPIO18 | U1RXD; ADC2_CH7; CLK_OUT3 | U1RXD; ADC2_CH7; CLK_OUT3 | io |
| J1.12 | GPIO8 | TOUCH8; ADC1_CH7; SUBSPICS1 | TOUCH8; ADC1_CH7; SUBSPICS1 | io |
| J1.13 | GPIO3 | TOUCH3; ADC1_CH2; strapping pin (JTAG source) | TOUCH3; ADC1_CH2; Strapping-Pin (JTAG-Quelle) | io |
| J1.14 | GPIO46 | Strapping pin (boot mode, ROM log) | Strapping-Pin (Boot-Modus, ROM-Log) | io |
| J1.15 | GPIO9 | TOUCH9; ADC1_CH8; FSPIHD | TOUCH9; ADC1_CH8; FSPIHD | io |
| J1.16 | GPIO10 | TOUCH10; ADC1_CH9; FSPICS0 | TOUCH10; ADC1_CH9; FSPICS0 | io |
| J1.17 | GPIO11 | TOUCH11; ADC2_CH0; FSPID | TOUCH11; ADC2_CH0; FSPID | io |
| J1.18 | GPIO12 | TOUCH12; ADC2_CH1; FSPICLK | TOUCH12; ADC2_CH1; FSPICLK | io |
| J1.19 | GPIO13 | TOUCH13; ADC2_CH2; FSPIQ | TOUCH13; ADC2_CH2; FSPIQ | io |
| J1.20 | GPIO14 | TOUCH14; ADC2_CH3; FSPIWP | TOUCH14; ADC2_CH3; FSPIWP | io |
| J1.21 | 5V | 5 V supply (USB 5 V behind Schottky diodes, or input) | 5-V-Versorgung (USB-5 V hinter Schottky-Dioden, oder Eingang) | power |
| J1.22 | GND | Ground (silk G) | Masse (Aufdruck G) | ground |
| J3.1 | GND | Ground (silk G) | Masse (Aufdruck G) | ground |
| J3.2 | GPIO43 | U0TXD, to the USB-UART bridge (silk TX) | U0TXD, zur USB-UART-Brücke (Aufdruck TX) | io |
| J3.3 | GPIO44 | U0RXD, from the USB-UART bridge (silk RX) | U0RXD, von der USB-UART-Brücke (Aufdruck RX) | io |
| J3.4 | GPIO1 | TOUCH1; ADC1_CH0 | TOUCH1; ADC1_CH0 | io |
| J3.5 | GPIO2 | TOUCH2; ADC1_CH1 | TOUCH2; ADC1_CH1 | io |
| J3.6 | GPIO42 | JTAG MTMS | JTAG MTMS | io |
| J3.7 | GPIO41 | JTAG MTDI; CLK_OUT1 | JTAG MTDI; CLK_OUT1 | io |
| J3.8 | GPIO40 | JTAG MTDO; CLK_OUT2 | JTAG MTDO; CLK_OUT2 | io |
| J3.9 | GPIO39 | JTAG MTCK; CLK_OUT3 | JTAG MTCK; CLK_OUT3 | io |
| J3.10 | GPIO38 | RGB LED data (v1.1); FSPIWP | RGB-LED-Daten (v1.1); FSPIWP | io |
| J3.11 | GPIO37 | SPIDQS; used by the Octal flash or PSRAM on the variants sold now | SPIDQS; bei den heute verkauften Varianten vom Octal-Flash oder -PSRAM belegt | data |
| J3.12 | GPIO36 | SPIIO7; used by the Octal flash or PSRAM on the variants sold now | SPIIO7; bei den heute verkauften Varianten vom Octal-Flash oder -PSRAM belegt | data |
| J3.13 | GPIO35 | SPIIO6; used by the Octal flash or PSRAM on the variants sold now | SPIIO6; bei den heute verkauften Varianten vom Octal-Flash oder -PSRAM belegt | data |
| J3.14 | GPIO0 | Strapping pin (boot mode); BOOT button | Strapping-Pin (Boot-Modus); BOOT-Taster | io |
| J3.15 | GPIO45 | Strapping pin (VDD_SPI voltage) | Strapping-Pin (VDD_SPI-Spannung) | io |
| J3.16 | GPIO48 | SPICLK_N | SPICLK_N | io |
| J3.17 | GPIO47 | SPICLK_P | SPICLK_P | io |
| J3.18 | GPIO21 | GPIO (RTC_GPIO21) | GPIO (RTC_GPIO21) | io |
| J3.19 | GPIO20 | USB_D+ of the chip, wired to the USB socket; U1CTS; ADC2_CH9 | USB_D+ des Chips, an der USB-Buchse; U1CTS; ADC2_CH9 | io |
| J3.20 | GPIO19 | USB_D- of the chip, wired to the USB socket; U1RTS; ADC2_CH8 | USB_D- des Chips, an der USB-Buchse; U1RTS; ADC2_CH8 | io |
| J3.21 | GND | Ground (silk G) | Masse (Aufdruck G) | ground |
| J3.22 | GND | Ground (silk G) | Masse (Aufdruck G) | ground |

Notes for the item: All GPIOs are 3.3 V logic (ESP32-S3 datasheet: VIH max VDD + 0.3 V), except as below. Strapping pins GPIO0, GPIO3, GPIO45 and GPIO46 are read at reset (datasheet section 3): GPIO0/GPIO46 set the boot mode (BOOT button on GPIO0), GPIO45 sets the flash supply voltage VDD_SPI, GPIO3 the JTAG source — keep circuits on them from forcing levels at power-up. GPIO35–GPIO37 are not available on boards with Octal flash/PSRAM (WROOM-1/1U with Octal memory, all WROOM-2), i.e. the current N16R8 and N32R16V variants (Espressif guide note). GPIO47/GPIO48: the datasheet says that on ESP32-S3R8V/R16V chips (VDD_SPI 1.8 V) these two pins work at 1.8 V; the N32R16V variant carries a 1.8 V-memory module (WROOM-2-N32R16V) — treat GPIO47/48 as 1.8 V there. GPIO19/GPIO20 also go to the second Micro-USB socket (ESP32-S3 USB OTG/JTAG). RGB LED on GPIO38 on v1.1; the initial version used GPIO48. Power: the USB sockets, the 5V pin or the 3V3 pins — three mutually exclusive ways (Espressif); each USB VBUS reaches the 5V rail through its own 1N5819 diode.

## Sources
1. Espressif, ESP32-S3-DevKitC-1 v1.1 User Guide (esp-dev-kits, latest; source docs/en/esp32-s3-devkitc-1/user_guide_v1.1.rst, last changed 2026-10-14), https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32s3/esp32-s3-devkitc-1/user_guide.html — key components (RGB LED on GPIO38), Octal note on GPIO35–37, power options, header tables J1/J3, v1.0 vs v1.1 note; read via the GitHub source; accessed 2026-11-24
2. Espressif, ordering-info.inc of that guide — variants N32R16V (WROOM-2-N32R16V), N16R8 (WROOM-1), 1U-N16R8, EOL N8/N8R2; accessed 2026-11-24
3. Espressif, pin layout figure v1.1, https://raw.githubusercontent.com/espressif/esp-dev-kits/master/docs/_static/esp32-s3-devkitc-1/ESP32-S3_DevKitC-1_pinlayout_v1.1.jpg — looked at; accessed 2026-11-24
4. Espressif, annotated photo "ESP32-S3-DevKitC-1 - front", https://raw.githubusercontent.com/espressif/esp-dev-kits/master/docs/_static/esp32-s3-devkitc-1/ESP32-S3-DevKitC-1_v2-annotated-photo.png — silk of both rows (zoomed), socket positions; accessed 2026-11-24
5. Espressif, schematic ESP32-S3-DevKitC-1 V1.1 (sheet 2 dated Wednesday, November 30, 2022), https://dl.espressif.com/dl/schematics/SCH_ESP32-S3-DevKitC-1_V1.1_20221130.pdf — J1, J3 (CON22X1), USB-to-UART and ESP USB power paths (D1, D7 1N5819HW), ESP USB D-/D+ = GPIO19/GPIO20; rendered and zoomed; accessed 2026-11-24
6. Espressif, ESP32-S3 Series Datasheet Version 2.2, https://documentation.espressif.com/esp32-s3_datasheet_en.pdf — section 3 boot configurations (strapping pins, table 3-1), pin note 4 on GPIO47/48 at 1.8 V, table 5-4 DC characteristics; read as text; accessed 2026-11-24

## Cross-check
Guide tables (source 1) against schematic (source 5), pin by pin: J1 1/2 ESP_3V3, 3 CHIP_PU, 4–20 GPIO4, 5, 6, 7, 15, 16, 17, 18, 8, 3, 46, 9, 10, 11, 12, 13, 14, 21 VCC_5V, 22 GND; J3 1 GND, 2 U0TXD, 3 U0RXD, 4–20 GPIO1, 2, 42, 41, 40, 39, 38, 37, 36, 35, 0, 45, 48, 47, 21, 20, 19, 21/22 GND. Match on all 44. Figure (source 3) and photo silk (source 4) carry the same order in both columns.

Differences found:
- The guide's tables mark no strapping pins; the figure marks GPIO0, GPIO3, GPIO45, GPIO46 as STRAP and the datasheet lists exactly these four. Taken from figure + datasheet.
- The guide links two schematic file names (…_20220413 in the header section, …_20221130 in Related Documents). I read the 20221130 file; its J1/J3 match the guide tables.
- The figure labels the supply pin 5V0, the silk 5V; ground G on the silk, GND in the figure. Names follow GND/5V.

## Choices
- Catalog item: the lists are Espressif's turned by 180 degrees (a rotation, not a mirror) so that the USB sockets are at the top like the ESP32-DevKitC (`connector: top`); left J3.22 to J3.1, right J1.22 to J1.1. The four strapping pins carry a pin note; GPIO35 to GPIO37 are type `data` with a note. The UNVERIFIED point of this record (GPIO47 and GPIO48 at 1.8 V on one variant) is left out of the data: the `function` of J3.16 and J3.17 names only the chip signals.
- Names `GPIOnn` for every GPIO, silk TX/RX in `function` (as in the esp32_devkitc record).
- GPIO35–37 type `data`: like GPIO6–11 on the ESP32-DevKitC, they are memory bus lines on the current variants and must not look like free pins; the guide's I/O/T type is kept in words.
- GPIO47/48 stay `io` with the 1.8 V caution in `function` and notes.
- Lists given in Espressif's orientation (USB down) and as a 180° turn for `connector: top`.

## Spot checks
1. J1.3 = RST (EN/CHIP_PU) — guide J1 row 3 "RST I EN"; schematic J1 pin 3 CHIP_PU; figure "RST"; photo "RST".
2. J1.14 = GPIO46 strapping — guide J1 row 14 "46 GPIO46"; schematic J1 pin 14 GPIO46; figure "GPIO46 LOG" (STRAP colour); datasheet "GPIO0, GPIO3, GPIO45, GPIO46 : Strapping pins".
3. J1.21 = 5V — guide J1 row 21 "5V P"; schematic J1 pin 21 VCC_5V; figure "5V0"; photo "5V".
4. J3.2 = GPIO43 / TX — guide J3 row 2 "TX U0TXD, GPIO43"; schematic J3 pin 2 U0TXD; photo "TX".
5. J3.10 = GPIO38, RGB LED — guide J3 row 10 "GPIO38 … RGB LED" and key components; schematic J3 pin 10 GPIO38, RGB LED fed from GPIO38; figure "GPIO38 … RGB_LED".
6. J3.14 = GPIO0 BOOT — guide J3 row 14; schematic J3 pin 14 GPIO0, SW1 to GPIO0; figure "GPIO0 BOOT".
7. J3.20 = GPIO19 / USB_D- — guide J3 row 20 "… USB_D-"; schematic J3 pin 20 GPIO19 and J4 D- = GPIO19; figure "USB_D- GPIO19".

UNVERIFIED: whether the WROOM-2-N32R16V module uses an ESP32-S3R16V-class chip whose GPIO47/48 run at 1.8 V (the module datasheet was not read; the caution is derived from the chip datasheet note and the "V" variant name). All 44 pin positions and names are verified.
