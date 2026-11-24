# esp32_c3_devkitm1 — ESP32-C3-DevKitM-1 headers / ESP32-C3-DevKitM-1 Stiftleisten

Kind: authoritative maker document (Espressif user guide, pin layout figure, annotated photo, schematic; ESP32-C3 datasheet for chip limits), board ESP32-C3-DevKitM-1 V1 (with ESP32-C3-MINI-1 or -MINI-1U), the only hardware revision ("No previous versions available")
Pins: 30; types: power 4; ground 10; signal 1; data 0; io 15; nc 0
Status: complete

## View
Board from the component side, portrait. Espressif's pin layout figure draws it with the module (antenna) at the top and the Micro-USB socket at the bottom: left column J1.1 (GND, top) … J1.15 (GND, bottom), right column J3.1 (GND, top) … J3.15 (GND, bottom). Basis:
- Pin layout figure `esp32-c3-devkitm-1-v1-pinout.png` (rendered and looked at): left GND, 3V3, 3V3, GPIO2, GPIO3, GND, RST, GND, GPIO0, GPIO1, GPIO10, GND, 5V0, 5V0, GND; right GND, GPIO21 (U0TXD), GPIO20 (U0RXD), GND, GPIO9 (BOOT), GPIO8 (LOG, RGB_LED), GND, GPIO7, GPIO6, GPIO5, GPIO4, GND, GPIO18 (USB_D-), GPIO19 (USB_D+), GND; BOOT button lower left, RST button lower right.
- Annotated photo `esp32-c3-devkitm-1-v1-annotated-photo.png` ("front", looked at): board landscape, module left, USB right; silk of the upper row from the module end GND, TX, RX, GND, 9, 8, GND, 7, 6, 5, 4, GND, 18, 19, GND; lower row GND, 3V3, 3V3, 2, 3, GND, RST, GND, 0, 1, 10, GND, 5V, 5V, GND. Turned 90° clockwise (module up) the upper row becomes the right column and the lower row the left one — the same as the figure.

Lists as Espressif draws it (`connector: bottom`):
- left (top to bottom): J1.1 … J1.15
- right (top to bottom): J3.1 … J3.15
For `connector: top` (USB up, as in the esp32_devkitc record), turn by 180°: left J3.15 … J3.1, right J1.15 … J1.1 (a rotation, not a mirror).

## Pins
| position | label | function en | function de | type |
|---|---|---|---|---|
| J1.1 | GND | Ground | Masse | ground |
| J1.2 | 3V3 | 3.3 V supply (LDO output, or input) | 3,3-V-Versorgung (LDO-Ausgang oder Eingang) | power |
| J1.3 | 3V3 | 3.3 V supply (LDO output, or input) | 3,3-V-Versorgung (LDO-Ausgang oder Eingang) | power |
| J1.4 | GPIO2 | ADC1_CH2; FSPIQ; strapping pin (silk 2) | ADC1_CH2; FSPIQ; Strapping-Pin (Aufdruck 2) | io |
| J1.5 | GPIO3 | ADC1_CH3 (silk 3) | ADC1_CH3 (Aufdruck 3) | io |
| J1.6 | GND | Ground | Masse | ground |
| J1.7 | RST | CHIP_PU (chip enable); low resets; RST button | CHIP_PU (Chip-Freigabe); low setzt zurück; RST-Taster | signal |
| J1.8 | GND | Ground | Masse | ground |
| J1.9 | GPIO0 | ADC1_CH0; XTAL_32K_P (silk 0) | ADC1_CH0; XTAL_32K_P (Aufdruck 0) | io |
| J1.10 | GPIO1 | ADC1_CH1; XTAL_32K_N (silk 1) | ADC1_CH1; XTAL_32K_N (Aufdruck 1) | io |
| J1.11 | GPIO10 | FSPICS0 (silk 10) | FSPICS0 (Aufdruck 10) | io |
| J1.12 | GND | Ground | Masse | ground |
| J1.13 | 5V | 5 V supply (USB 5 V behind a Schottky diode, or input) | 5-V-Versorgung (USB-5 V hinter Schottky-Diode, oder Eingang) | power |
| J1.14 | 5V | 5 V supply (USB 5 V behind a Schottky diode, or input) | 5-V-Versorgung (USB-5 V hinter Schottky-Diode, oder Eingang) | power |
| J1.15 | GND | Ground | Masse | ground |
| J3.1 | GND | Ground | Masse | ground |
| J3.2 | GPIO21 | U0TXD, to the USB-UART bridge (silk TX) | U0TXD, zur USB-UART-Brücke (Aufdruck TX) | io |
| J3.3 | GPIO20 | U0RXD, from the USB-UART bridge (silk RX) | U0RXD, von der USB-UART-Brücke (Aufdruck RX) | io |
| J3.4 | GND | Ground | Masse | ground |
| J3.5 | GPIO9 | Strapping pin; BOOT button to GND (silk 9) | Strapping-Pin; BOOT-Taster gegen GND (Aufdruck 9) | io |
| J3.6 | GPIO8 | Strapping pin; RGB LED data, 10 kΩ pull-up (silk 8) | Strapping-Pin; RGB-LED-Daten, 10-kΩ-Pull-up (Aufdruck 8) | io |
| J3.7 | GND | Ground | Masse | ground |
| J3.8 | GPIO7 | FSPID; JTAG MTDO (silk 7) | FSPID; JTAG MTDO (Aufdruck 7) | io |
| J3.9 | GPIO6 | FSPICLK; JTAG MTCK (silk 6) | FSPICLK; JTAG MTCK (Aufdruck 6) | io |
| J3.10 | GPIO5 | ADC2_CH0; FSPIWP; JTAG MTDI (silk 5) | ADC2_CH0; FSPIWP; JTAG MTDI (Aufdruck 5) | io |
| J3.11 | GPIO4 | ADC1_CH4; FSPIHD; JTAG MTMS (silk 4) | ADC1_CH4; FSPIHD; JTAG MTMS (Aufdruck 4) | io |
| J3.12 | GND | Ground | Masse | ground |
| J3.13 | GPIO18 | USB_D- of the chip's USB Serial/JTAG (silk 18) | USB_D- des USB-Serial/JTAG im Chip (Aufdruck 18) | io |
| J3.14 | GPIO19 | USB_D+ of the chip's USB Serial/JTAG (silk 19) | USB_D+ des USB-Serial/JTAG im Chip (Aufdruck 19) | io |
| J3.15 | GND | Ground | Masse | ground |

Notes for the item: All GPIOs are 3.3 V only: the ESP32-C3 datasheet gives VIH max VDD + 0.3 V and 3.6 V absolute maximum on the supply pins. GPIO2, GPIO8 and GPIO9 are strapping pins read at power-up/reset (Espressif guide footnote 2); download mode needs GPIO9 low with GPIO8 high (datasheet table 3-3), which is what the BOOT button does; GPIO8 also drives the addressable RGB LED. GPIO18/GPIO19 are the chip's own USB pins (USB Serial/JTAG by default per the datasheet); the Micro-USB socket of this board goes to the USB-UART bridge on GPIO20/21, not to GPIO18/19. The SPI flash bus is inside the module and not on the headers. Power: Micro-USB, the 5V pins or the 3V3 pins — "three mutually exclusive ways" (Espressif); use only one. The 5V pins sit behind a 1N5819 Schottky diode from USB VBUS (schematic).

## Sources
1. Espressif, ESP32-C3-DevKitM-1 User Guide (esp-dev-kits, latest; source docs/en/esp32-c3-devkitm-1/user_guide.rst, last changed 2026-10-14), https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32c3/esp32-c3-devkitm-1/user_guide.html — key components, power supply options, header tables J1 and J3 with footnotes, revision note; read via the GitHub source; accessed 2026-11-24
2. Espressif, pin layout figure of that guide, https://raw.githubusercontent.com/espressif/esp-dev-kits/master/docs/_static/esp32-c3-devkitm-1/esp32-c3-devkitm-1-v1-pinout.png — looked at; accessed 2026-11-24
3. Espressif, annotated photo "ESP32-C3-DevKitM-1 (with the ESP32-C3-MINI-1 module) - front", https://raw.githubusercontent.com/espressif/esp-dev-kits/master/docs/_static/esp32-c3-devkitm-1/esp32-c3-devkitm-1-v1-annotated-photo.png — silk and orientation; looked at; accessed 2026-11-24
4. Espressif, schematic ESP32-C3-DevKitM-1 V1.0 (sheet 2 dated Thursday, May 20, 2021; linked as SCH_ESP32-C3-DEVKITM-1_V1_20200915A.pdf), https://dl.espressif.com/dl/schematics/SCH_ESP32-C3-DEVKITM-1_V1_20200915A.pdf — J1, J3 (CON15X1_2P54), D1 VBUS→VCC_5V, R20 on GPIO8, SW1/SW2, RGB LED; rendered and zoomed; accessed 2026-11-24
5. Espressif, ESP32-C3 Series Datasheet Version 2.5, https://documentation.espressif.com/esp32-c3_datasheet_en.pdf — section 3 boot configurations (tables 3-1, 3-3), pin overview notes on strapping and USB_D+/-, table 5-1 absolute maximum ratings, table 5-4 DC characteristics; read as text; accessed 2026-11-24

## Cross-check
Guide tables (source 1) against schematic (source 4), pin by pin: J1 1 GND, 2/3 VCC_3V3, 4 GPIO2, 5 GPIO3, 6 GND, 7 CHIP_PU, 8 GND, 9 GPIO0, 10 GPIO1, 11 GPIO10, 12 GND, 13/14 VCC_5V, 15 GND; J3 1 GND, 2 U0TXD, 3 U0RXD, 4 GND, 5 GPIO9, 6 GPIO8, 7 GND, 8 GPIO7, 9 GPIO6, 10 GPIO5, 11 GPIO4, 12 GND, 13 GPIO18, 14 GPIO19, 15 GND. Match on all 30. The pin layout figure (source 2) and the photo silk (source 3) carry the same order in both columns.

Differences found:
- The figure marks STRAP functions only on GPIO9 ("BOOT") and GPIO8 ("LOG"); the guide footnote and the datasheet list GPIO2 as a strapping pin too. I mark all three (datasheet: GPIO2 "does not determine" SPI vs download boot but "it is recommended to pull this pin up").
- The schematic link is named …_20200915A but the PDF's title block says V1.0 and its sheet 2 date is May 20, 2021 (design name …_20210520A). Same headers either way.
- The figure labels the supply pins 5V0, the silk and guide 5V. Name follows guide and silk.

## Choices
- Catalog item: the lists are Espressif's turned by 180 degrees (a rotation, not a mirror) so that the USB socket is at the top like the ESP32-DevKitC (`connector: top`); left J3.15 to J3.1, right J1.15 to J1.1. The strapping pins GPIO2, GPIO8 and GPIO9 carry a pin note; GPIO18 and GPIO19 stay `io`.
- Names `GPIOnn` for every GPIO, as in the esp32_devkitc record (GPIO21 instead of silk TX, GPIO20 instead of RX); silk text in `function`.
- `RST` type `signal`; GPIO18/19 stay `io` (Espressif lists them I/O/T), with the USB role in `function`.
- Lists given in Espressif's orientation (USB down) and as a 180° turn for `connector: top`; the integrator picks one and states it in `doc`.

## Spot checks
1. J1.7 = RST (CHIP_PU) — guide J1 row 7 "RST I CHIP_PU"; schematic J1 pin 7 CHIP_PU; figure left 7th "RST"; photo lower row 7th "RST".
2. J1.13 = 5V — guide J1 row 13 "5V P"; schematic J1 pin 13 VCC_5V; figure "5V0"; photo "5V".
3. J1.4 = GPIO2 strapping — guide J1 row 4 "GPIO2 [2], ADC1_CH2, FSPIQ"; schematic J1 pin 4 GPIO2; datasheet 3 "Strapping pins: GPIO2, GPIO8, and GPIO9".
4. J3.2 = GPIO21 / TX — guide J3 row 2 "TX GPIO21, U0TXD"; schematic J3 pin 2 U0TXD; figure "GPIO21 U0TXD"; photo upper row 2nd "TX".
5. J3.6 = GPIO8, RGB LED — guide J3 row 6 "GPIO8 [2], RGB LED"; schematic J3 pin 6 GPIO8, GPIO8→R17→RGB_CTRL; figure "GPIO8 LOG RGB_LED".
6. J3.14 = GPIO19 / USB_D+ — guide J3 row 14 "GPIO19, USB_D+"; schematic J3 pin 14 GPIO19; figure "GPIO19 USB_D+".
7. J3.15 = GND (bottom right) — guide J3 row 15; schematic J3 pin 15 to GND; figure last right "GND".

UNVERIFIED: none of the 30 pins. The 3V3 output current is not stated in the sources read and is left out.
