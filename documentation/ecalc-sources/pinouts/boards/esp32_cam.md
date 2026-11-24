# esp32_cam — Ai-Thinker ESP32-CAM headers / Ai-Thinker ESP32-CAM Stiftleisten

Kind: fallback route (maker documents not reachable). Ai-Thinker's own ESP32-CAM pages and files no longer load: docs.ai-thinker.com/en/esp32-cam is now a navigation hub without ESP32 products, and the schematic/spec links that other sites cite (docs.ai-thinker.com/_media/esp32/docs/esp32_cam_sch.pdf, …/ov2640_ds_1.8_.pdf) return that hub page instead of a PDF; vdoc.ai-thinker.com has an expired certificate; web.archive.org is not fetchable here. So, as for a clone: signal functions from Espressif (ESP32 datasheet, Espressif's camera pin map for "AI_THINKER"), header nets from a copy of the board schematic "ESP32_CAM" Rev 1.6, and the header layout from two independent publishers.
Board: ESP32-CAM with the Ai-Thinker ESP-32S module, OV2640 camera on a 24-pin FPC, microSD socket, PSRAM, 2x8 pins.
Pins: 16; types: power 3; ground 3; signal 1; data 1; io 8; nc 0
Status: complete via the fallback route (positions and nets agree in all sources read); Ai-Thinker documents not opened

## View
The side with the camera, the microSD socket and the flash LED, which is also the side with the pin labels printed beside the pins; microSD socket at the top, camera FPC connector at the bottom. There is no USB socket on the board (`connector: none`; or `top` if the catalog names the microSD end). Basis:
- Random Nerd Tutorials pinout figure `ESP32-CAM-pinout-new.png` (looked at): left column top to bottom 5V, GND, GPIO12, GPIO13, GPIO15, GPIO14, GPIO2, GPIO4; right column 3.3V, GPIO16, GPIO0, GND, VCC (3.3V/5V), GPIO3 (U0RXD), GPIO1 (U0TXD), GND; the drawn silk beside the pads reads 5V, GND, IO12, IO13, IO15, IO14, IO2, IO4 and 3V3, IO16, IO0, GND, VCC, U0R, U0T, GND.
- Last Minute Engineers pinout figure `ESP32-CAM-Pinout.png` (own drawing, looked at): the same side, the same orientation and the same order in both columns.
- The SunFounder figure (`esp32_cam_pinout.png`, "the ESP32-CAM pinout (AI-Thinker module)") was also looked at; it is the Random Nerd Tutorials graphic without the watermark, so it is not counted as an independent source.
- The Zephyr photo shows the other side (ESP-32S module, antenna at the top, RST button lower left); no labels readable, used only to confirm which side carries the module.

Lists in this view:
- left (top to bottom): P2.8, P2.7, P2.6, P2.5, P2.4, P2.3, P2.2, P2.1
- right (top to bottom): P1.8, P1.7, P1.6, P1.5, P1.4, P1.3, P1.2, P1.1
Seen from the module side the columns swap (mirror image); the catalog draws only the labelled side.

## Header numbers
Schematic "ESP32_CAM", Revision 1.6, dated 2018/1/30, file ESP32_CAM_V1.6.SchDoc (no company in the title block): P1 "Header 8": 1 GND, 2 U0TXD, 3 U0RXD, 4 VCC (to 3V3 through R1 0R or to 5V through R2 0R), 5 GND, 6 CSI_MCLK, 7 U2RXD, 8 3V3. P2 "Header 8": 1 HS2_DATA1, 2 HS2_DATA0, 3 HS2_CLK, 4 HS2_CMD, 5 HS2_DATA3, 6 HS2_DATA2, 7 GND, 8 5V. Module M1 (ESP-32S) nets: HS2_DATA1 = GPIO4, HS2_DATA0 = IO2, HS2_CLK = GPIO14, HS2_CMD = IO15, HS2_DATA3 = IO13, HS2_DATA2 = GPIO12, CSI_MCLK = GPIO0 (R19 10 k to 3V3), U2RXD = GPIO16 (also PSRAM CS#, R5 10 k to 3V3), U0TXD/U0RXD = GPIO1/GPIO3.

## Pins
| position | label | function en | function de | type |
|---|---|---|---|---|
| P2.8 | 5V | 5 V supply input to the 3.3 V regulator (AMS1117) | 5-V-Versorgungseingang zum 3,3-V-Regler (AMS1117) | power |
| P2.7 | GND | Ground | Masse | ground |
| P2.6 | GPIO12 | microSD DATA2 (HS2_DATA2); strapping pin MTDI (flash voltage) — keep low at boot | microSD DATA2 (HS2_DATA2); Strapping-Pin MTDI (Flash-Spannung) — beim Start low halten | io |
| P2.5 | GPIO13 | microSD DATA3 (HS2_DATA3) | microSD DATA3 (HS2_DATA3) | io |
| P2.4 | GPIO15 | microSD CMD (HS2_CMD); strapping pin MTDO | microSD CMD (HS2_CMD); Strapping-Pin MTDO | io |
| P2.3 | GPIO14 | microSD CLK (HS2_CLK) | microSD CLK (HS2_CLK) | io |
| P2.2 | GPIO2 | microSD DATA0 (HS2_DATA0); strapping pin (boot mode) | microSD DATA0 (HS2_DATA0); Strapping-Pin (Boot-Modus) | io |
| P2.1 | GPIO4 | microSD DATA1 (HS2_DATA1); drives the white flash LED | microSD DATA1 (HS2_DATA1); steuert die weiße Blitz-LED | io |
| P1.8 | 3V3 | 3.3 V rail (regulator output) | 3,3-V-Schiene (Reglerausgang) | power |
| P1.7 | GPIO16 | PSRAM chip select; U2RXD — not free while PSRAM is used | PSRAM-Chipauswahl; U2RXD — nicht frei, solange PSRAM genutzt wird | data |
| P1.6 | GPIO0 | Camera XCLK; boot-mode strapping pin, 10 kΩ pull-up — low at reset = flash mode | Kamera-XCLK; Boot-Strapping-Pin, 10-kΩ-Pull-up — low beim Reset = Flash-Modus | signal |
| P1.5 | GND | Ground | Masse | ground |
| P1.4 | VCC | Supply output, 3.3 V (default) or 5 V, set by a 0 Ω link | Versorgungsausgang, 3,3 V (Standard) oder 5 V, per 0-Ω-Brücke | power |
| P1.3 | GPIO3 | UART0 RX (U0RXD), for programming (silk U0R) | UART0 RX (U0RXD), zum Programmieren (Aufdruck U0R) | io |
| P1.2 | GPIO1 | UART0 TX (U0TXD), for programming (silk U0T) | UART0 TX (U0TXD), zum Programmieren (Aufdruck U0T) | io |
| P1.1 | GND | Ground | Masse | ground |

Camera and board GPIOs (not on the headers), for the notes: OV2640 D0–D7 (Y2–Y9) = GPIO5, 18, 19, 21, 36, 39, 34, 35; XCLK = GPIO0; PCLK = GPIO22; VSYNC = GPIO25; HREF = GPIO23; SIOD (SCCB SDA) = GPIO26; SIOC (SCCB SCL) = GPIO27; PWDN = GPIO32; camera reset not connected (−1). Red LED on GPIO33 (active low, schematic LED1 from 3V3 through 1 k to GPIO33); white flash LED switched by GPIO4 through a transistor; PSRAM on GPIO16 (CS) and GPIO17 (CLK).

Notes for the item: All GPIOs are 3.3 V logic (ESP32 datasheet: VIH max VDD + 0.3 V); feed the board through 5V (the 3V3 pin is the regulator output). VCC is an output, 3.3 V by default, 5 V if the 0 Ω link is moved — not a supply input. The six P2 GPIOs belong to the microSD card while it is used (1-bit SD mode frees GPIO12/13 per RNT). GPIO0, GPIO2, GPIO12 and GPIO15 are ESP32 strapping pins (datasheet section 3): GPIO0/GPIO2 select the boot mode, GPIO12 (MTDI) the flash supply voltage — a high level there at reset can stop the board from booting. Programming needs an external USB-serial adapter on U0R/U0T with GPIO0 pulled to GND at reset.

## Sources
1. Espressif, ESP32 Series Datasheet Version 5.3, https://documentation.espressif.com/esp32_datasheet_en.pdf — section 3 boot configurations (strapping GPIO0, GPIO2, MTDI, MTDO, GPIO5; table 3-1), input-only GPIO34–39, table 5-3 DC characteristics; read as text; accessed 2026-11-24
2. Espressif, arduino-esp32 `libraries/ESP32/examples/Camera/CameraWebServer/camera_pins.h`, block `CAMERA_MODEL_AI_THINKER`, https://raw.githubusercontent.com/espressif/arduino-esp32/master/libraries/ESP32/examples/Camera/CameraWebServer/camera_pins.h — PWDN 32, RESET −1, XCLK 0, SIOD 26, SIOC 27, Y2–Y9 5/18/19/21/36/39/34/35, VSYNC 25, HREF 23, PCLK 22, LED_GPIO_NUM 4 ("4 for flash led or 33 for normal led"); accessed 2026-11-24
3. Schematic "ESP32_CAM", Revision 1.6, 2018/1/30 (ESP32_CAM_V1.6.SchDoc), copy hosted by Seeed Studio, https://github.com/SeeedDocument/forum_doc/blob/master/reg/ESP32_CAM_V1.6.pdf (linked by Random Nerd Tutorials as the ESP32-CAM schematic) — P1, P2, M1 nets, PSRAM U5, LEDs, regulator, VCC links R1/R2; rendered and zoomed; authorship not stated in the title block; accessed 2026-11-24
4. Random Nerd Tutorials, "ESP32-CAM AI-Thinker Pinout Guide: GPIOs Usage Explained", https://randomnerdtutorials.com/esp32-cam-ai-thinker-pinout/ — pinout figure https://i0.wp.com/randomnerdtutorials.com/wp-content/uploads/2020/03/ESP32-CAM-pinout-new.png (looked at), power pins, GPIO0 pull-up, GPIO16/PSRAM, microSD table, GPIO4 flash, GPIO33 red LED, camera table; page date not shown; accessed 2026-11-24
5. Last Minute Engineers (Amrit Prabhu), "ESP32-CAM Pinout Reference", last updated Jan 20, 2026, https://lastminuteengineers.com/esp32-cam-pinout-reference/ — pinout figure https://lastminuteengineers.com/wp-content/uploads/iot/ESP32-CAM-Pinout.png (looked at), VCC note, GPIO0, GPIO4, GPIO33; accessed 2026-11-24
6. SunFounder, Galaxy RVR docs "ESP32 CAM", https://docs.sunfounder.com/projects/galaxy-rvr/en/latest/hardware/cpn_esp_32_cam.html — states its data come from Ai-Thinker; microSD table, GPIO32 camera power, power "at least 5V 2A"; its pinout figure is the RNT graphic (not independent); accessed 2026-11-24
7. Zephyr Project, "Ai-Thinker ESP32-CAM" board page, https://docs.zephyrproject.org/latest/boards/aithinker/esp32_cam/doc/index.html — GPIO4 shared by microSD data and flash LED, serial wiring (GPIO0 "Must be low at boot"); board photo (module side); accessed 2026-11-24
Not opened: Ai-Thinker ESP32-CAM page and files (see Kind). A retailer-hosted "ESP32-Cam Datasheet, AI-Thinker Model" (cdn.shopify.com, Google-Docs export without publisher) was read but not used as a source.

## Cross-check
Header layout, the two independent figures (sources 4 and 5): left 5V, GND, GPIO12, GPIO13, GPIO15, GPIO14, GPIO2, GPIO4; right 3.3V, GPIO16, GPIO0, GND, VCC, GPIO3, GPIO1, GND. Identical on all 16 positions.
Figures against the schematic copy (source 3): right column top to bottom = P1.8 3V3, P1.7 U2RXD (GPIO16), P1.6 CSI_MCLK (GPIO0), P1.5 GND, P1.4 VCC, P1.3 U0RXD, P1.2 U0TXD, P1.1 GND; left column = P2.8 5V, P2.7 GND, P2.6 HS2_DATA2 (GPIO12), P2.5 HS2_DATA3 (GPIO13), P2.4 HS2_CMD (GPIO15), P2.3 HS2_CLK (GPIO14), P2.2 HS2_DATA0 (GPIO2), P2.1 HS2_DATA1 (GPIO4). Match on all 16; header numbering runs in one direction along each column.
Camera GPIOs: Espressif's camera_pins.h (source 2), the schematic copy's camera nets (CSI_D0–D7 on GPIO5, 18, 19, 21, 36 (SENSOR_VP), 39 (SENSOR_VN), 34, 35; CSI_MCLK GPIO0; CSI_PCLK GPIO22; CSI_VSYNC GPIO25; CSI_HSYNC GPIO23; TWI_SDA GPIO26; TWI_SCK GPIO27; CAM_PWR GPIO32) and RNT's table agree.
microSD: schematic copy, RNT and SunFounder agree (CLK 14, CMD 15, D0 2, D1 4, D2 12, D3 13).

Differences found:
- Last Minute Engineers' figure labels the SD signals differently (GPIO12 "Data 1", GPIO13 "Data 0", GPIO15 "CLK", GPIO14 "CMD", GPIO2 "Data 3", GPIO4 "Data 2"); the schematic copy, RNT and SunFounder agree on the mapping used here. LME's positions, ADC/touch/HSPI labels match.
- SunFounder's figure is the RNT graphic, so only RNT and LME count as independent layout sources.
- RNT says GPIO16 is on the PSRAM "on many boards"; the schematic copy shows it as PSRAM CS# (and U2RXD on P1.7).

## Choices
- Catalog item: drawn from the labelled camera side with the microSD socket at the top, `connector: none` (no USB socket); left P2.8 to P2.1, right P1.8 to P1.1, as the record's lists. The item notes and the source line say that no maker document was opened and the layout rests on two independent sources; the camera and microSD GPIOs and the PSRAM chip select are in the notes, GPIO16 also as a pin note.
- Header ids P1/P2 from the schematic copy, with that caveat stated; names `GPIOnn`, `5V`, `3V3`, `VCC`, `GND`; the silk U0R/U0T in `function`.
- GPIO16 type `data` (PSRAM bus line, as the flash lines on the ESP32-DevKitC); GPIO0 type `signal` (camera clock and boot strap, not a free pin); the SD-card GPIOs stay `io` with the sharing in `function`.
- VCC type `power` with "output" spelled out in `function` (people mistake it for an input).
- Drawn from the labelled camera side; `connector: none` suggested.

## Spot checks
1. Left top = 5V (P2.8) — schematic P2 pin 8 to 5V; RNT and LME figures "5V" top left.
2. Left 3rd = GPIO12 (HS2_DATA2, strapping MTDI) — schematic P2 pin 6 HS2_DATA2 = M1 GPIO12; both figures "GPIO12"; ESP32 datasheet 3: MTDI sets VDD_SDIO.
3. Left bottom = GPIO4 (flash LED, SD DATA1) — schematic P2 pin 1 HS2_DATA1 = GPIO4, Q1 base via R13 from HS2_DATA1; camera_pins.h LED_GPIO_NUM 4; both figures "GPIO4".
4. Right 2nd = GPIO16 (PSRAM CS) — schematic P1 pin 7 U2RXD, M1 pin 27 GPIO16 "IO16", U5 CS# on IO16; both figures "GPIO16".
5. Right 3rd = GPIO0 (XCLK, boot) — schematic P1 pin 6 CSI_MCLK = GPIO0 with R19 10 k to 3V3; camera_pins.h XCLK 0; both figures "GPIO0".
6. Right 5th = VCC output — schematic P1 pin 4 via R1/R2 0R to 3V3/5V; RNT "3.3V/5V", LME "VCC" and text "outputs 3.3V".
7. Right 7th = GPIO1 (U0TXD) — schematic P1 pin 2 U0TXD; figures "GPIO1 U0TXD".

UNVERIFIED: that the Rev 1.6 schematic copy is Ai-Thinker's current document (no company in the title block; Ai-Thinker's own link is dead). The pin layout itself rests on two independent tutorial publishers plus that schematic; no Ai-Thinker document was read. Clones may differ.
