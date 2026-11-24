# microbit — BBC micro:bit V2 edge connector / BBC micro:bit V2 Randstecker

Kind: authoritative maker document (Micro:bit Educational Foundation: tech site edge-connector page and figure, V2.2.1 schematic, schematic page pin map), board BBC micro:bit V2 (V2.00 and V2.2x share the edge connector)
Pins: 25; types: power 3, ground 3, signal 3, data 2, io 14

## View
Front of the board (the side with the LED matrix and buttons A/B, where the rings are labelled 0, 1, 2, 3V, GND), upright, edge connector along the bottom edge, read from left to right. The USB socket is at the top edge on the back. Suggested shape: `board`, `connector: top` (USB, back side) with the 25 contacts as the `bottom` list; or a `rows` shape with one row if the drawing should show only the connector. Basis:
- The tech site's V2 figure `edge-connector-2.svg` (rendered and looked at) draws the edge connector turned 90° counter-clockwise: ring 0 at the bottom, GND at the top, contacts pointing right, ring numbers and "GND" upright when the figure is turned back. Turned back, ring 0 is at the left and GND at the right of the front view; the single strip outside ring 0 (LED Col 3 = P3) is the leftmost contact, the guard strip after GND the rightmost.
- The tech site's V2 table lists the contacts in connector order (one table row per 1.27 mm position): P3, RING0 (4 rows), P4, P5, P6, P7, RING1 (4), P8, P9, P10, P11, P12, RING2 (4), P13, P14, P15, P16, +V_TGT (6 rows: strip, ring, strip), P19, P20, GND (6 rows: strip, ring, strip).
- The V2.2.1 schematic, sheet "Expansion connector", draws J1 with the same order and numbers the 40 front positions: 1 P3, 2 ring P0, 6 P4 … 9 P7, 10 ring P1, 14 P8 … 18 P12, 19 ring P2, 23 P13 … 26 P16, 27 3V strip, 28 3V ring, 32 3V strip, 33 P19, 34 P20, 35 GND strip, 36 GND ring, 40 GND strip.

Proposed `bottom` list (left to right, one entry per contact, numbers = J1 pins of the V2.2.1 schematic): 1, 2, 6, 7, 8, 9, 10, 14, 15, 16, 17, 18, 19, 23, 24, 25, 26, 27, 28, 32, 33, 34, 35, 36, 40.
To scale, each ring is four strip widths wide (J1 positions 2–5, 10–13, 19–22, 28–31, 36–39 at 1.27 mm); the catalog grid has no wide pad, so the compact list draws every ring as one position. If the drawing should show the ring widths, three `null` after each ring number would do it (40 positions).

Back side: "Only the pins on the front are connected to signals. The back rings are connected to the front rings, but the back small strips are unconnected." The record lists only the front contacts.

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | P3 | GPIO/analog AIN7 (P0.31); shared with LED column 3 | GPIO/Analog AIN7 (P0.31); mit LED-Spalte 3 geteilt | io |
| 2 | 0 | Ring P0: GPIO, analog AIN0, touch, PWM (P0.02); 10 MΩ pull-up | Ring P0: GPIO, Analog AIN0, Touch, PWM (P0.02); 10-MΩ-Pull-up | io |
| 6 | P4 | GPIO/analog AIN4 (P0.28); shared with LED column 1 | GPIO/Analog AIN4 (P0.28); mit LED-Spalte 1 geteilt | io |
| 7 | P5 | Button A (P0.14); 10 kΩ pull-up on the board | Taster A (P0.14); 10-kΩ-Pull-up auf der Platine | signal |
| 8 | P6 | GPIO (P1.05); shared with LED column 4 | GPIO (P1.05); mit LED-Spalte 4 geteilt | io |
| 9 | P7 | GPIO (P0.11); shared with LED column 2 | GPIO (P0.11); mit LED-Spalte 2 geteilt | io |
| 10 | 1 | Ring P1: GPIO, analog AIN1, touch, PWM (P0.03); 10 MΩ pull-up | Ring P1: GPIO, Analog AIN1, Touch, PWM (P0.03); 10-MΩ-Pull-up | io |
| 14 | P8 | GPIO (P0.10); NFC2, off by default | GPIO (P0.10); NFC2, standardmäßig aus | io |
| 15 | P9 | GPIO (P0.09); NFC1, off by default | GPIO (P0.09); NFC1, standardmäßig aus | io |
| 16 | P10 | GPIO/analog AIN6 (P0.30); shared with LED column 5 | GPIO/Analog AIN6 (P0.30); mit LED-Spalte 5 geteilt | io |
| 17 | P11 | Button B (P0.23); 10 kΩ pull-up on the board | Taster B (P0.23); 10-kΩ-Pull-up auf der Platine | signal |
| 18 | P12 | Reserved for accessibility (P0.12); do not use | Für Barrierefreiheit reserviert (P0.12); nicht verwenden | signal |
| 19 | 2 | Ring P2: GPIO, analog AIN2, touch, PWM (P0.04); 10 MΩ pull-up | Ring P2: GPIO, Analog AIN2, Touch, PWM (P0.04); 10-MΩ-Pull-up | io |
| 23 | P13 | GPIO; SPI SCK (P0.17) | GPIO; SPI SCK (P0.17) | io |
| 24 | P14 | GPIO; SPI MISO (P0.01) | GPIO; SPI MISO (P0.01) | io |
| 25 | P15 | GPIO; SPI MOSI (P0.13) | GPIO; SPI MOSI (P0.13) | io |
| 26 | P16 | GPIO (P1.02) | GPIO (P1.02) | io |
| 27 | 3V | 3 V supply strip beside the 3V ring | 3-V-Versorgungsstreifen neben dem 3V-Ring | power |
| 28 | 3V | Ring 3V: 3 V supply, output or input | Ring 3V: 3-V-Versorgung, Ausgang oder Eingang | power |
| 32 | 3V | 3 V supply strip beside the 3V ring | 3-V-Versorgungsstreifen neben dem 3V-Ring | power |
| 33 | P19 | External I2C SCL (P0.26); 4.7 kΩ pull-up | Externes I2C SCL (P0.26); 4,7-kΩ-Pull-up | data |
| 34 | P20 | External I2C SDA (P1.00); 4.7 kΩ pull-up | Externes I2C SDA (P1.00); 4,7-kΩ-Pull-up | data |
| 35 | GND | Ground strip beside the GND ring | Massestreifen neben dem GND-Ring | ground |
| 36 | GND | Ring GND: ground | Ring GND: Masse | ground |
| 40 | GND | Ground strip beside the GND ring | Massestreifen neben dem GND-Ring | ground |

Item notes (en, as in the catalog): All signal contacts are 3.3 V logic and not 5 V tolerant: the pin voltage must stay between −0.3 V and VDD + 0.3 V, and the board supply is 1.7 to 3.6 V. Drive is 1 to 4 mA standard and 6 to 15 mA high. The edge connector may supply 190 mA (300 mA regulator minus the on-board budget); the edge-connector page says 270 mA and marks it as to be confirmed, so use 190 mA. Only three pins can do PWM at the same time. P3, P4, P6, P7 and P10 belong to the LED display: switch the display off in software before using them otherwise. P5 and P11 are the buttons A and B, P12 is reserved for the accessibility mode. P19 and P20 are the external I2C bus with 4.7 kΩ pull-ups; on V2 the motion sensor is on a separate internal I2C bus. Do not connect speakers, motors or piezos directly. To power the board through the 3V ring use a regulated supply and a protection diode.

## Sources
1. Micro:bit Educational Foundation, "Edge connector and pinout", https://tech.microbit.org/hardware/edgeconnector/ (source hardware/edge-connector.mdx in github.com/microbit-foundation/dev-docs, last changed 2026-10-09) — overview text, V2 table (m:b ring, mod, schem, MCU, s/w, functions, dir, pull), notes 1–7, "Uncoupling default functionality", nRF52 GPIO table, NOTE 4 current; read in full; accessed 2026-11-24
2. Micro:bit Educational Foundation, V2 edge connector figure, https://tech.microbit.org/assets/images/edge-connector-2-53c4bc0ed7948cca05d99e7bdac6ba13.svg (source hardware/assets/edge-connector-2.svg) — rendered and looked at; LED-column labels and ring labels readable, the white pin-name labels on the coloured boxes did not render; accessed 2026-11-24
3. Micro:bit Educational Foundation, micro:bit V2.2.1 schematic (MicroBit_V2.2.1_nRF52820 schematic.PDF, Altium, file date 2021-11-29), https://github.com/microbit-foundation/microbit-v2-hardware/blob/main/V2.21/MicroBit_V2.2.1_nRF52820%20schematic.PDF — sheet 2 block "Expansion connector", J1 with pins 1–40 and nets; rendered and zoomed; accessed 2026-11-24
4. Micro:bit Educational Foundation, "Schematic" page, https://tech.microbit.org/hardware/schematic/ (source hardware/schematic.md) — V2 pinmap (nRF52833 GPIO → allocation → edge connector name), regulator 300 mA / 190 mA for accessories, 10 MΩ pull-ups on P0–P2, NFC on P0.09/P0.10; accessed 2026-11-24
5. Micro:bit Educational Foundation, "Power supply" page, https://tech.microbit.org/hardware/powersupply/ (source hardware/power-supply.md) — 3V ring powering precautions, V2 300 mA regulator, 270 mA / 190 mA figures; accessed 2026-11-24

## Cross-check
Tech-site table (source 1) against the V2.2.1 schematic J1 (source 3), contact by contact in connector order: P3/COLR3, ring P0/RING0, P4/COLR1, P5/BTN_A, P6/COLR4, P7/COLR2, ring P1/RING1, P8/GPIO1, P9/GPIO2, P10/COLR5, P11/BTN_B, P12/GPIO4, ring P2/RING2, P13/SPI_EXT_SCK, P14/SPI_EXT_MISO, P15/SPI_EXT_MOSI, P16/GPIO3, 3V strip, 3V ring, 3V strip, P19/I2C_EXT_SCL, P20/I2C_EXT_SDA, GND strip, GND ring, GND strip. Match on all 25 (schematic net names equal the table's "schem" column).
MCU pins: the edge table (source 1) and the schematic page's pinmap (source 4) agree on all 19 signal contacts (P0 P0.02, P1 P0.03, P2 P0.04, P3 P0.31, P4 P0.28, P5 P0.14, P6 P1.05, P7 P0.11, P8 P0.10, P9 P0.09, P10 P0.30, P11 P0.23, P12 P0.12, P13 P0.17, P14 P0.01, P15 P0.13, P16 P1.02, P19 P0.26, P20 P1.00).
Direction: figure (source 2) turned back gives P3 at the left end and the GND guard strip at the right end, the same order as the table and J1 pins 1 → 40.

Differences found:
- Edge-connector current: source 1 NOTE 4 "270mA" (TBC); sources 4 and 5 "190mA" for accessories with a 300 mA regulator. I use 190 mA.
- P19/P20: source 1 says the I2C bus is "used by both the on-board motion sensor" (V1 wording); on V2 the schematic page lists the motion sensor on I2C_INT (P0.08/P0.16) and P19/P20 as I2C_EXT. The record calls P19/P20 the external I2C bus.
- The V1 table (same page) has different MCU pins; not used. V1 P9 was also an LED column; on V2 it is not.

## Choices
- Catalog item: `board` shape, `connector: top`, front view: the 25 contacts are the `bottom` list, left to right, numbered by the J1 pins of the V2.2.1 schematic (1, 2, 6 to 10, 14 to 19, 23 to 28, 32 to 36, 40), each ring one position (the grid has no wide pad, so the three nulls per ring of the record's alternative are not used). The numbers are at most two characters, so the 24-unit pitch of a side list holds them. `connector: top` draws the USB socket as an orientation aid although it is on the back; the item doc says so.
- Wording follows V2: the motion sensor is on the internal I2C bus, P19 and P20 are the external bus (pin notes on 33 and 34). The V1 wording of the tech site page is not used.
- Edge-connector current: the item note uses 190 mA (schematic and power pages) and names the 270 mA (to be confirmed) of the edge-connector page.
- Position ids are the J1 pin numbers of the V2.2.1 schematic (rings: 2, 10, 19, 28, 36; guard strips 27, 32, 35, 40), because the board has no printed numbers for the small strips and these numbers are unique and ordered left to right.
- Names: ring labels as printed (0, 1, 2, 3V, GND); small strips by their software names P3 … P20; the guard strips 3V/GND.
- Types: P5/P11/P12 `signal` (buttons and the accessibility pin, not free GPIO); P19/P20 `data` (I2C bus with fixed pull-ups); the LED-column pins stay `io` with the sharing in `function`.
- Only the front side is listed; the back rings are the same nets, the back strips are unconnected.

## Spot-check
1. Position 1 = P3 (leftmost contact, LED column 3) — schematic J1 pin 1 "P3" net COLR3; table first row "21 COLR3 P0.31/AIN7 P3"; figure: strip below ring 0 labelled "LED Col 3".
2. Position 2 = ring 0 (P0.02) — schematic J1 pin 2 ring "P0" net RING0; table "0 18 RING0 P0.02/AIN0 P0".
3. Position 7 = P5 button A — schematic J1 pin 7 BTN_A; table "37 BTN_A P0.14 P5".
4. Position 18 = P12 accessibility — schematic J1 pin 18 GPIO4, note "GPIO4 reserved for accessibility"; table "40 GPIO4 P0.12 P12 … ACCESSIBILITY".
5. Position 28 = 3V ring, input or output — schematic J1 pin 28 to VREG, note "Edge connector 3V may be input or output"; table "3V … +V_TGT".
6. Position 33 = P19 SCL, 34 = P20 SDA — schematic J1 pins 33/34 I2C_EXT_SCL/I2C_EXT_SDA; table "17 I2C EXT SCL P0.26 P19", "16 I2C EXT SDA P1.00 P20".
7. Position 40 = GND guard strip (rightmost) — schematic J1 pin 40 GND; table last row "GND … PSU(GND)"; figure: strip above the GND ring.

UNVERIFIED: none of the 25 contacts. The text of the coloured pin-name boxes in the V2 figure did not render (direction taken from the rendered ring labels and LED-column labels).
