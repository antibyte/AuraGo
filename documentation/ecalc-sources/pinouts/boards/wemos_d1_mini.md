# wemos_d1_mini — WEMOS/LOLIN D1 mini (ESP8266) / WEMOS/LOLIN D1 mini (ESP8266)

Kind: authoritative maker document (WEMOS/LOLIN docs: board pages V4.0.0 and V3.1.0, schematics V4.0.0 and V3.0.0, pinout figure, board photo) plus Espressif's ESP8266EX datasheet for the chip's pin functions
Pins: 16; types: power 2; ground 1; signal 2; data 0; io 11; nc 0
Status: complete (header positions identical on V3.x and V4.0.0; the 5V pin differs, see notes)

## View
Board from the component side (ESP8266EX/module, flash, USB bridge visible), portrait. WEMOS draws it with the PCB antenna at the top and the USB socket at the bottom. Basis:
- WEMOS pinout figure `d1_mini_v4.0.0_5_16x9.png` (rendered and looked at): antenna up, USB-C down; left column top to bottom RST, A0, 16, 14 (SCK), 12 (MISO), 13 (MOSI), 15, 3.3V; right column TX, RX, 5 (SCL), 4 (SDA), 0, 2 (LED), GND, VBUS; reset button at the lower left, a 4-pin I2C socket at the lower right (GND, 4, 5, 3.3V).
- WEMOS board photo `d1_mini_v3.1.0_1_16x16.jpg` (looked at): antenna up, Micro-USB down, silk on the left RST, A0, D0, D5, D6, D7, D8, 3V3; on the right TX, RX, D1, D2, D3, D4, GND, 5V.
- Same pin at every position in both: the V3.1.0 pin table maps D0 = GPIO16, D5 = GPIO14, D6 = GPIO12, D7 = GPIO13, D8 = GPIO15, D1 = GPIO5, D2 = GPIO4, D3 = GPIO0, D4 = GPIO2.

Lists as WEMOS draws it (`connector: bottom`):
- left (top to bottom): J2.1, J2.2, J2.3, J2.4, J2.5, J2.6, J2.7, J2.8
- right (top to bottom): J3.1, J3.2, J3.3, J3.4, J3.5, J3.6, J3.7, J3.8
If the catalog wants USB at the top (`connector: top`), turn by 180°: left J3.8 … J3.1, right J2.8 … J2.1 (a rotation, not a mirror; the component side stays up).

## Header numbers
V4.0.0 schematic (KiCad, file D1_mini.kicad_sch, PDF dated 2021-11-09): J2 (P_01x08) 1 RST, 2 A0, 3 IO16, 4 IO14 (SCK), 5 IO12 (MISO), 6 IO13 (MOSI), 7 IO15, 8 +3V3; J3 (P_01x08) 1 TX, 2 RX, 3 IO5 (SCL), 4 IO4 (SDA), 5 IO0, 6 IO2, 7 GND, 8 VBUS. Pin 1 of both is at the antenna end.
V3.0.0 schematic (EAGLE, 2018-01-23, REV V3.0.0): one 16-pin symbol P1 counted around the board: 1 RST, 2 A0, 3 D0 (GPIO16), 4 D5/SCK (GPIO14), 5 D6/MISO (GPIO12), 6 D7/MOSI (GPIO13), 7 D8 (GPIO15), 8 3V3, 9 5V (+5V), 10 GND, 11 D4 (GPIO2), 12 D3 (GPIO0), 13 SDA/D2 (GPIO4), 14 SCL/D1 (GPIO5), 15 RX, 16 TX. So P1.1–8 = J2.1–8 and P1.16 … P1.9 = J3.1 … J3.8.

## Pins
| position | label | function en | function de | type |
|---|---|---|---|---|
| J2.1 | RST | Reset, active low; reset button | Reset, low-aktiv; Reset-Taster | signal |
| J2.2 | A0 | Analog input 0–3.2 V (220 k/100 k divider to the ADC TOUT) | Analogeingang 0–3,2 V (Teiler 220 k/100 k zum ADC TOUT) | signal |
| J2.3 | D0 | GPIO16; no interrupt, PWM, I2C or one-wire; deep-sleep wake (jumper to RST) | GPIO16; kein Interrupt, PWM, I2C oder One-Wire; Deep-Sleep-Wecken (Brücke zu RST) | io |
| J2.4 | D5 | GPIO14; SPI SCK | GPIO14; SPI SCK | io |
| J2.5 | D6 | GPIO12; SPI MISO | GPIO12; SPI MISO | io |
| J2.6 | D7 | GPIO13; SPI MOSI | GPIO13; SPI MOSI | io |
| J2.7 | D8 | GPIO15; SPI SS; 12 kΩ pull-down on the board; boot-mode pin | GPIO15; SPI SS; 12-kΩ-Pull-down auf der Platine; Boot-Modus-Pin | io |
| J2.8 | 3V3 | 3.3 V output of the on-board regulator | 3,3-V-Ausgang des Reglers auf der Platine | power |
| J3.1 | TX | UART0 TX (GPIO1); to the USB-serial chip | UART0 TX (GPIO1); zum USB-Seriell-Chip | io |
| J3.2 | RX | UART0 RX (GPIO3); from the USB-serial chip | UART0 RX (GPIO3); vom USB-Seriell-Chip | io |
| J3.3 | D1 | GPIO5; I2C SCL | GPIO5; I2C SCL | io |
| J3.4 | D2 | GPIO4; I2C SDA | GPIO4; I2C SDA | io |
| J3.5 | D3 | GPIO0; 12 kΩ pull-up on the board; boot-mode pin | GPIO0; 12-kΩ-Pull-up auf der Platine; Boot-Modus-Pin | io |
| J3.6 | D4 | GPIO2; 12 kΩ pull-up; on-board LED (active low); boot-mode pin | GPIO2; 12-kΩ-Pull-up; LED auf der Platine (low-aktiv); Boot-Modus-Pin | io |
| J3.7 | GND | Ground | Masse | ground |
| J3.8 | 5V | USB 5 V (V4.0.0: VBUS, direct; V3: through diode and 0.5 A fuse) | USB-5 V (V4.0.0: VBUS, direkt; V3: über Diode und 0,5-A-Sicherung) | power |

Notes for the item: All I/O pins run at 3.3 V and are not 5 V tolerant (WEMOS: "All of the IO pins run at 3.3V"; Espressif: VIH max 3.6 V, I/O IMAX 12 mA). A0 takes 0–3.2 V (WEMOS), the chip's TOUT 0–1.0 V behind the divider. GPIO0, GPIO2 and GPIO15 (D3, D4, D8) select the boot mode at reset (Espressif: "GPIO2, GPIO0, and MTDO are used to select booting mode"), so keep circuits on them from pulling against the board's pull resistors at power-up; Espressif also warns that U0TXD (TX) must not be pulled low externally during power-up. D0 (GPIO16) has no interrupt/PWM/I2C/one-wire (WEMOS) and wakes the chip from deep sleep when linked to RST (solder jumper on the board). 5V pin: on V3.x it is "5V" behind a B5819W diode and a 0.5 A fuse from USB; on V4.0.0 WEMOS labels it VBUS and the schematic ties it straight to the USB-C VBUS — feeding 5 V into it while USB is plugged in drives the USB port. V4.0.0 also has a 4-pin I2C socket (SH1.0: 1 GND, 2 SDA = GPIO4, 3 SCL = GPIO5, 4 3V3), not listed as pins. Clones exist with other parts; check the silk.

## Sources
1. WEMOS/LOLIN, "D1 mini" board page (current V4.0.0), https://www.wemos.cc/en/latest/d1/d1_mini.html — features ("11 digital IO, interrupt/pwm/I2C/one-wire supported(except D0)", "1 analog input(3.2V max input)"), tech specs (3.3 V), "All of the IO pins run at 3.3V", links to the figure and schematic; accessed 2026-11-24
2. WEMOS/LOLIN, pinout figure V4.0.0, https://www.wemos.cc/en/latest/_static/boards/d1_mini_v4.0.0_5_16x9.png — looked at; accessed 2026-11-24
3. WEMOS/LOLIN, schematic V4.0.0, https://www.wemos.cc/en/latest/_static/files/sch_d1_mini_v4.0.0.pdf (KiCad, PDF 2021-11-09) — J2, J3, P1 (I2C), pull resistors R9/R10/R11 (12 k), A0 divider R12/R13, LED1 on IO2, JP1 RST–IO16, USB VBUS net; rendered and zoomed; accessed 2026-11-24
4. WEMOS/LOLIN, "D1 mini V3.1.0" board page, https://www.wemos.cc/en/latest/d1/d1_mini_3.1.0.html — pin table (Pin / Function / ESP-8266 Pin: TX TXD, RX RXD, A0 "Analog input, max 3.2V", D0 GPIO16, D1 "IO, SCL" GPIO5, D2 "IO, SDA" GPIO4, D3 "IO, 10k Pull-up" GPIO0, D4 "IO, 10k Pull-up, BUILTIN_LED" GPIO2, D5 SCK GPIO14, D6 MISO GPIO12, D7 MOSI GPIO13, D8 "IO, 10k Pull-down, SS" GPIO15, G, 5V, 3V3, RST); accessed 2026-11-24
5. WEMOS/LOLIN, board photo V3.1.0, https://www.wemos.cc/en/latest/_static/boards/d1_mini_v3.1.0_1_16x16.jpg — silk labels and positions; looked at; accessed 2026-11-24
6. WEMOS, schematic "WEMOS D1 mini" REV V3.0.0, 2018-01-23 (linked from the V3.1.0 page), https://www.wemos.cc/en/latest/_static/files/sch_d1_mini_v3.0.0.pdf — P1 1–16, F1/D2 on the 5 V path; rendered and zoomed; accessed 2026-11-24
7. Espressif, ESP8266EX Datasheet Version 7.1 (footer 2025.11, NRND), https://documentation.espressif.com/0a-esp8266ex_datasheet_en.pdf — section 2 pin definitions and note on boot-mode pins and U0TXD, ADC section (TOUT 0–1.0 V), electrical table (VIH, IMAX 12 mA); read as text; accessed 2026-11-24

## Cross-check
V4.0.0 figure (source 2) against V4.0.0 schematic (source 3): left RST, A0, 16, 14, 12, 13, 15, 3.3V = J2.1–8 (RST, A0, IO16, IO14, IO12, IO13, IO15, +3V3); right TX, RX, 5, 4, 0, 2, GND, VBUS = J3.1–8. Match on all 16. SCK/MISO/MOSI beside 14/12/13 and SCL/SDA beside 5/4 in both.
V3.1.0 photo (source 5) against V3.0.0 schematic (source 6) and V3.1.0 table (source 4): silk RST, A0, D0, D5, D6, D7, D8, 3V3 / TX, RX, D1, D2, D3, D4, GND, 5V = P1.1–8 and P1.16–9 with the same GPIOs as the table. Match on all 16.
V3 against V4: every position carries the same GPIO; only the 5V/VBUS pin's circuit differs.

Differences found:
- Pull resistors: the V3.1.0 pin table says 10 k (D3, D4 pull-up, D8 pull-down); both schematics (V3.0.0 R10/R11/R5, V4.0.0 R9/R10/R11) show 12 k. `function` gives no value.
- 5V pin: "5V" on the V3 photo and table, "VBUS" on the V4 figure and schematic; V3 schematic has diode D2 (B5819W) and fuse F1 0.5 A between USB VBUS and +5V, V4 schematic has neither. Stated in the pin and the notes.
- LED series resistor 1 k (V3) vs 2 k (V4); not used.
- The V3.1.0 page links a schematic named v3.0.0; I treat it as the V3.x schematic (same pins as the V3.1.0 table and photo).

## Choices
- Catalog item: the lists are the maker's turned by 180 degrees (a rotation, not a mirror) so that the USB socket is at the top like the other ESP boards (`connector: top`); left J3.8 to J3.1, right J2.8 to J2.1. D3, D4 and D8 say 12 kΩ in `function` (both schematics) and the notes mention the V3.1.0 table's "10k". The V4 VBUS difference is a pin note of J3.8 and in the item notes. The I2C socket of the V4 stays out.
- Names are the D-labels of the V3 silk and pin table (D0 … D8), with the GPIO number first in `function`, as the task asks for D-labels and GPIO numbers; RST, A0, TX, RX, 3V3, GND, 5V as printed on V3. The V4 figure prints GPIO numbers instead; same pins.
- Position ids follow the current V4.0.0 schematic (J2, J3), with the V3 P1 numbering in Header numbers.
- RST and A0 are `signal` (A0 is an ADC input only), as in the NodeMCU record; TX/RX `io`.
- J3.8 is named `5V` (V3 silk) with the V4 VBUS difference in `function` and notes.
- The V4 I2C socket is in the notes, not drawn.

## Spot checks
1. J2.3 = D0 = GPIO16 — V4 schematic J2 pin 3 "IO16"; V4 figure third left "16"; V3 table "D0 IO GPIO16"; V3 photo third left "D0".
2. J2.7 = D8 = GPIO15, pull-down — V4 schematic J2 pin 7 "IO15", R11 12k to GND; V3 table "D8 IO, 10k Pull-down, SS GPIO15".
3. J2.8 = 3V3 — V4 schematic J2 pin 8 "+3V3"; figure "3.3V" bottom left; photo "3V3".
4. J3.3 = D1 = GPIO5 (SCL) — V4 schematic J3 pin 3 "IO5" with "SCL"; figure "5 SCL"; V3 table "D1 IO, SCL GPIO5".
5. J3.6 = D4 = GPIO2 (LED) — V4 schematic J3 pin 6 "IO2", LED1 to IO2; figure "2 LED"; V3 table "D4 … BUILTIN_LED GPIO2".
6. J3.8 = 5V/VBUS — V4 schematic J3 pin 8 "VBUS"; figure "VBUS"; V3 schematic P1 pin 9 "+5V" after F1; photo "5V".
7. J3.1 = TX — V4 schematic J3 pin 1 "TX" (U0TXD through R8 470 Ω); figure and photo "TX" top right.

UNVERIFIED: none of the 16 pins. Not read: a current rating for the 3V3 output (left out of `function`).
