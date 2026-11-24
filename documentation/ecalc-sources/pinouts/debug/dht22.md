# dht22 — DHT22 / AM2302 (4-pin) / DHT22 / AM2302 (4-polig)

Kind: authoritative maker document (two editions of the Aosong datasheet)
Pins: 4; types: power 1, ground 1, data 1, nc 1

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | VDD | Supply, 3.3 to 6 V DC (edition A) or 3.3 to 5.5 V DC (edition B) | Versorgung, 3,3 bis 6 V DC (Ausgabe A) bzw. 3,3 bis 5,5 V DC (Ausgabe B) | power |
| 2 | DATA | Single-bus data line, needs a pull-up to VDD (1 kΩ in Aosong's diagram) | Eindraht-Datenleitung, braucht einen Pull-up nach VDD (1 kΩ im Aosong-Schaltbild) | data |
| 3 | NC | Not connected, leave free | Nicht belegt, frei lassen | nc |
| 4 | GND | Ground | Masse | ground |

Wired AM2302 ("big case and wires", edition B only): red = power supply, black = GND, yellow = data output. These colours belong to the wired version, not to the 4-pin part, so no `wire` is set on the pins; the item note names them.

## View
Written for the sensor seen from the front (perforated grille facing you), pins pointing down: one row, pins 1, 2, 3, 4 from left to right. `drawn: plug` (the sensor carries the pins).

Basis: edition A, page 4, dimension drawing of the "Small size" sensor (front view with grille and mounting hole, pins down) with the caption "Pin sequence number: 1 2 3 4 (from left to right direction)." Edition B, page 2, the same drawing and caption for the "Standard AM2302". The other view (mirror) is the back of the sensor: 4, 3, 2, 1 from left to right.

## Sources
1. Edition A: Aosong Electronics Co., Ltd, "Digital-output relative humidity & temperature sensor/module DHT22 (DHT22 also named as AM2302)", 10 pages, undated, copy hosted by SparkFun: https://cdn.sparkfun.com/assets/f/7/d/9/c/DHT22.pdf (redirect from https://www.sparkfun.com/datasheets/Sensors/Temperature/DHT22.pdf) — page 2 (technical specification: power supply 3.3-6V DC), page 4 (drawing, caption, pin table "1 VDD----power supply / 2 DATA--signal / 3 NULL / 4 GND"), page 5 (connection diagram VDD to 1Pin, DATA to 2Pin, GND to 4Pin; "3Pin---NC"; 100 nF between VDD and GND), page 9 (electrical characteristics 3.3 / 5 / 6 V). Rendered and read. Accessed 2026-11-24.
2. Edition B: Aosong (header "Your specialist in innovating humidity & temperature sensors"), "Digital relative humidity & temperature sensor AM2302/DHT22", 5 pages, undated, copy hosted by Adafruit: https://cdn-shop.adafruit.com/datasheets/Digital+humidity+and+temperature+sensor+AM2302.pdf — page 1 (power supply 3.3-5.5V DC), page 2 (drawing, caption "from left to right direction", pin table, wire colours of the big case, connection diagram with 1K pull-up from DATA to VDD; pins 1, 2, 4 used), page 5 (electrical characteristics 3.3 / 5 / 6 V). Rendered and read. Accessed 2026-11-24.

Not opened: the current datasheet on aosong.com (not found by search); copy at enimages.ofweek.com (HTTP 503).

## Cross-check
- Pins 1, 2 and 4: both editions agree (VDD, DATA, GND), and both connection diagrams wire only 1Pin, 2Pin and 4Pin.
- Pin 3: edition A table "NULL" and diagram note "3Pin---NC". Edition B's table prints "GND" for pin 3, rendered with a line through or under it at this print quality, while its diagram leaves pin 3 out. The catalog follows edition A and both diagrams: NC. A search summary (not counted) of later copies also says "NC".
- Supply: edition A "3.3-6V DC" (page 2) and edition B "3.3-5.5V DC" (page 1, and "Power's voltage should be 3.3-5.5V DC"); both electrical tables give min 3.3, typ 5, max 6 V. The pin text quotes both, the note recommends staying within 3.3 to 5.5 V.
- Pull-up: edition B's diagram shows "1K" between DATA and VDD; edition A's diagram on page 5 does not show a resistor value. Text: "Data-bus's free status is high voltage level".
- Both editions describe a single-bus protocol that differs from the Maxim/Dallas 1-Wire bus (edition B's wording).

## Choices
- Catalog item: `rows` with one row ["1", "2", "3", "4"], `outline: rect`, `contact: square`, `drawn: plug`, front view (grille side). Pin 3 is `nc` with "leave free" and a pin note that explains the one datasheet edition (B) that prints GND at pin 3: its table entry is marked up and print quality is poor, its own connection diagram leaves pin 3 out, and edition A says NULL / NC, so it is resolved as NC. No source says pin 3 may be tied to GND.
- Pin 2 `data`: a bidirectional single-wire data line.
- Notes: "Wait at least 1 s after power-up before the first request and 2 s between readings" (edition A page 5 and page 9, edition B page 2 and page 5); "not compatible with Dallas/Maxim 1-Wire".

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | VDD | power | Aosong DHT22 (SparkFun copy) page 4 table "1 VDD----power supply"; AM2302 (Adafruit copy) page 2 table "1 VDD—power supply" |
| 2 | DATA | data | page 4 table "2 DATA--signal"; Adafruit copy page 2 diagram "DATA 2Pin" with "1K" to VDD |
| 3 | NC | nc | SparkFun copy page 4 table "3 NULL", page 5 "3Pin---NC" |
| 4 | GND | ground | SparkFun copy page 4 table "4 GND"; Adafruit copy page 2 table "4 GND", diagram "4Pin" to GND |

UNVERIFIED: pin 3 in edition B's table (printed "GND", visibly marked); resolved as NC from edition A and both diagrams. Neither edition carries a version number or date. Breakout boards with 3 pins (sensor on a small PCB) have their own order and are not covered.
