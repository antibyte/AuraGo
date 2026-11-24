# arduino_pro_mini — Arduino Pro Mini (SparkFun v14 layout) / Arduino Pro Mini (Layout v14 von SparkFun)

Kind: authoritative maker document. Arduino's retired product page says "The Arduino Pro Mini was designed and is manufactured by SparkFun Electronics"; the board maker's documents are SparkFun's schematic (also hosted by Arduino), SparkFun's Eagle board file and SparkFun's graphical datasheets. Board: Arduino Pro Mini 328, PCB v14, the same PCB for 5 V/16 MHz (DEV-11113) and 3.3 V/8 MHz (DEV-11114), Arduino SKU E000025.
Pins: 34; types: power 3, ground 4, signal 5, data 2, io 20

## View
Board from the component side (ATmega328P, reset button and silk labels on top), portrait, the 6-pin FTDI header at the top (drawn as an `inner` group, see Choices). Basis:
- SparkFun's graphical datasheets "Arduino Pro Mini (DEV-11113) 16MHz / 5V" and "(DEV-11114) 8MHz / 3.3V", rendered and looked at: board photo with the FTDI header at the top edge, left column TX0, RXI, RST, GND, 2 … 9 from top to bottom, right column RAW, GND, RST, VCC, A3, A2, A1, A0, 13, 12, 11, 10; the FTDI row reads (board side, "To target board") GND, GND, VCC, RXI, TXO, DTR from left to right, with BLK at the left end and GRN at the right end; A5/A4 and A7/A6 are listed as separate inner pads.
- SparkFun's Eagle board file Arduino-Pro-Mini.brd (Eagle 6.5, v14 on the bottom silk), read as XML: outline 17.78 x 33.02 mm; JP7 pads at x = 1.27 mm (pad 1 at y = 1.27 … pad 12 at y = 29.21), JP6 pads at x = 16.51 mm (pad 1 at y = 29.21 … pad 12 at y = 1.27), JP1 pads along y = 31.75 mm (pad 1 at x = 15.24 … pad 6 at x = 2.54), JP2 pads at x = 13.589 (pad 1 y = 17.907, pad 2 y = 20.447), JP3 pads at x = 13.589 (pad 1 y = 7.493, pad 2 y = 10.033). Top-silk (layer 21) texts sit beside the pads: TXO/RXI/RST/GND/2…9 along the left, RAW/GND/RST/VCC/A3…A0/13…10 along the right, BLK/GND/VCC/RXI/TXO/GRN along the top; A4/A5/A6/A7 are bottom-silk (layer 22) texts next to JP2/JP3. Eagle's view is the top view, so these coordinates are the drawn view (y up = towards the FTDI header).
- Signal nets of the board file give each pad's net (listed under Header numbers).

Lists in this view:
- left (top to bottom): JP7.12, JP7.11, JP7.10, JP7.9, JP7.8, JP7.7, JP7.6, JP7.5, JP7.4, JP7.3, JP7.2, JP7.1
- right (top to bottom): JP6.1, JP6.2, JP6.3, JP6.4, JP6.5, JP6.6, JP6.7, JP6.8, JP6.9, JP6.10, JP6.11, JP6.12
- inner, one column 2.9 mm inside the right column: JP2 (A5 above A4) level with rows 3.45 and 4.45 of the right column (between VCC/A3 and A3/A2), JP3 (A7 above A6) level with rows 7.55 and 8.55 (between A1/A0 and A0/13). Suggested `inner` blocks: "A4/A5" rows [JP2.2], [JP2.1]; "A6/A7" rows [JP3.2], [JP3.1]. They are off the 0.1" grid of the edge rows.

## Header numbers
Schematic/board names (SparkFun Arduino-Pro-Mini v14): JP1 "FTDI Basic", JP6 and JP7 (1x12), JP2 and JP3 (1x02, "Off Grid Breakout headers").
- JP1 nets (board file): 1 DTR, 2 TXO, 3 RXI, 4 VCC, 5 GND, 6 GND.
- JP6: 1 RAW, 2 GND, 3 RESET, 4 VCC, 5 A3, 6 A2, 7 A1, 8 A0, 9 SCK (D13), 10 MISO (D12), 11 MOSI (D11), 12 D10.
- JP7: 1 D9, 2 D8, 3 D7, 4 D6, 5 D5, 6 D4, 7 D3, 8 D2, 9 GND, 10 RESET, 11 RXI (D0), 12 TXO (D1).
- JP2: 1 A4, 2 A5. JP3: 1 A6, 2 A7.

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| JP1.6 | GND | Ground (FTDI pin marked BLK) | Masse (FTDI-Pin mit BLK markiert) | ground |
| JP1.5 | GND | Ground (CTS of the adapter) | Masse (CTS des Adapters) | ground |
| JP1.4 | VCC | Board supply from the adapter: 5 V or 3.3 V by model | Versorgung vom Adapter: 5 V oder 3,3 V je nach Modell | power |
| JP1.3 | RXI | Serial input from the adapter; same line as D0 | Serieller Eingang vom Adapter; dieselbe Leitung wie D0 | data |
| JP1.2 | TXO | Serial output to the adapter; same line as D1 | Serieller Ausgang zum Adapter; dieselbe Leitung wie D1 | data |
| JP1.1 | DTR | Auto-reset: to RESET through 0.1 µF (pin marked GRN) | Auto-Reset: über 0,1 µF an RESET (Pin mit GRN markiert) | signal |
| JP7.12 | D1 | UART TX (PD1); silk TXO | UART TX (PD1); Aufdruck TXO | io |
| JP7.11 | D0 | UART RX (PD0); silk RXI | UART RX (PD0); Aufdruck RXI | io |
| JP7.10 | RST | Reset; pull low to restart the ATmega328P | Reset; low startet den ATmega328P neu | signal |
| JP7.9 | GND | Ground | Masse | ground |
| JP7.8 | D2 | Digital pin; interrupt INT0 (PD2) | Digitaler Pin; Interrupt INT0 (PD2) | io |
| JP7.7 | D3 | PWM (OC2B); interrupt INT1 (PD3) | PWM (OC2B); Interrupt INT1 (PD3) | io |
| JP7.6 | D4 | Digital pin (PD4) | Digitaler Pin (PD4) | io |
| JP7.5 | D5 | PWM (OC0B) (PD5) | PWM (OC0B) (PD5) | io |
| JP7.4 | D6 | PWM (OC0A) (PD6) | PWM (OC0A) (PD6) | io |
| JP7.3 | D7 | Digital pin (PD7) | Digitaler Pin (PD7) | io |
| JP7.2 | D8 | Digital pin (PB0) | Digitaler Pin (PB0) | io |
| JP7.1 | D9 | PWM (OC1A) (PB1) | PWM (OC1A) (PB1) | io |
| JP6.1 | RAW | Unregulated supply input to the regulator (see notes) | Ungeregelter Versorgungseingang zum Regler (siehe Hinweise) | power |
| JP6.2 | GND | Ground | Masse | ground |
| JP6.3 | RST | Reset; same line as the other RST pin | Reset; dieselbe Leitung wie der andere RST-Pin | signal |
| JP6.4 | VCC | Regulated 5 V or 3.3 V by model; output, or input for a regulated supply | Geregelte 5 V oder 3,3 V je nach Modell; Ausgang oder Eingang für geregelte Versorgung | power |
| JP6.5 | A3 | Analog input ADC3; digital D17 (PC3) | Analogeingang ADC3; digital D17 (PC3) | io |
| JP6.6 | A2 | Analog input ADC2; digital D16 (PC2) | Analogeingang ADC2; digital D16 (PC2) | io |
| JP6.7 | A1 | Analog input ADC1; digital D15 (PC1) | Analogeingang ADC1; digital D15 (PC1) | io |
| JP6.8 | A0 | Analog input ADC0; digital D14 (PC0) | Analogeingang ADC0; digital D14 (PC0) | io |
| JP6.9 | D13 | SPI SCK; on-board LED (PB5) | SPI SCK; LED auf der Platine (PB5) | io |
| JP6.10 | D12 | SPI CIPO, formerly MISO (PB4) | SPI CIPO, früher MISO (PB4) | io |
| JP6.11 | D11 | PWM (OC2A); SPI COPI, formerly MOSI (PB3) | PWM (OC2A); SPI COPI, früher MOSI (PB3) | io |
| JP6.12 | D10 | PWM (OC1B); SPI SS (PB2) | PWM (OC1B); SPI SS (PB2) | io |
| JP2.1 | A4 | Analog input ADC4; digital D18; I2C SDA (PC4); inner pad | Analogeingang ADC4; digital D18; I2C SDA (PC4); innenliegendes Pad | io |
| JP2.2 | A5 | Analog input ADC5; digital D19; I2C SCL (PC5); inner pad | Analogeingang ADC5; digital D19; I2C SCL (PC5); innenliegendes Pad | io |
| JP3.1 | A6 | Analog input ADC6 only, no digital use; inner pad | Nur Analogeingang ADC6, nicht digital nutzbar; innenliegendes Pad | signal |
| JP3.2 | A7 | Analog input ADC7 only, no digital use; inner pad | Nur Analogeingang ADC7, nicht digital nutzbar; innenliegendes Pad | signal |

Item notes (en, as in the catalog): One PCB for two models, 5 V / 16 MHz and 3.3 V / 8 MHz: the logic level of every I/O pin is the VCC of the model, so do not drive the pins of the 3.3 V model with 5 V. RAW feeds the on-board regulator (150 mA): Arduino's tech specs give 5–12 V for the 5 V model and 3.35–12 V for the 3.3 V model, SparkFun up to 16 V; stay within 12 V and feed unregulated power to RAW, not to VCC. Current per I/O pin: 40 mA at most (Arduino and SparkFun), 20 mA recommended by SparkFun; use 20 mA. The FTDI header takes a 3.3 V or 5 V FTDI cable or breakout; JP1.6 (marked BLK) is the ground end and JP1.1 (marked GRN) is DTR. TXO and RXI are the board's own TX and RX, which the adapter crosses. The board has no USB. A6 and A7 are analog inputs only.

## Sources
1. Arduino docs-content, "Arduino Pro Mini" retired product page (content/retired/01.boards/arduino-pro-mini/content.md), https://docs.arduino.cc/retired/boards/arduino-pro-mini/ — designer/maker SparkFun, power pins RAW/VCC/GND, PWM 3, 5, 6, 9, 10, 11, interrupts 2 and 3, SPI 10–13, LED 13, I2C A4/A5, "40 mA", tech specs; read via GitHub source; accessed 2026-11-24
2. SparkFun Electronics, schematic Arduino-Pro-Mini, REV 14, dated 4/9/2014 ("Original Arduino Mini Design by Team Arduino / Arduino Pro Mini Design by Spark Fun Electronics"), hosted by Arduino at https://www.arduino.cc/en/uploads/Main/Arduino-Pro-Mini-schematic.pdf — JP1, JP6, JP7, JP2, JP3, regulator note; rendered and zoomed; accessed 2026-11-24
3. SparkFun Electronics, Eagle board file Hardware/Arduino-Pro-Mini.brd, repository sparkfun/Arduino_Pro_Mini_328 (last commit 2019-03-29), https://github.com/sparkfun/Arduino_Pro_Mini_328/blob/master/Hardware/Arduino-Pro-Mini.brd — element placement, package pads, signal nets of every header pad, top and bottom silk texts; read as XML; accessed 2026-11-24
4. SparkFun Electronics, graphical datasheet "Arduino Pro Mini (DEV-11113) 16MHz / 5V", https://github.com/sparkfun/Arduino_Pro_Mini_328/blob/master/Documentation/ProMini16MHzv2.pdf — board picture with all pin labels, port and alternate functions, power and current box; rendered and zoomed; accessed 2026-11-24
5. SparkFun Electronics, graphical datasheet "Arduino Pro Mini (DEV-11114) 8MHz / 3.3V", https://github.com/sparkfun/Arduino_Pro_Mini_328/blob/master/Documentation/ProMini8MHzv2.pdf — same layout and labels; power box "Raw: 3.3V-16V (4V-12V recommended)", "VCC:3.3V"; read as text; accessed 2026-11-24
Not opened: SparkFun's hookup guide (learn.sparkfun.com returned HTTP 403).

## Cross-check
Schematic (source 2) against board file nets (source 3), pad by pad: JP6.1–12 RAW, GND, RESET, VCC, A3, A2, A1, A0, SCK, MISO, MOSI, D10; JP7.1–12 D9, D8, D7, D6, D5, D4, D3, D2, GND, RESET, RXI, TXO; JP2 A4, A5; JP3 A6, A7. Match on all 32. JP1: the schematic symbol lists GND, CTS, VCC, TXO, RXI, DTR (the adapter's pin names) wired to nets GND, GND, VCC, RXI, TXO, DTR; the board file gives pads 1 DTR, 2 TXO, 3 RXI, 4 VCC, 5 GND, 6 GND. Consistent (the TXO/RXI crossing is in the symbol's naming, not on the board).

Board file positions (source 3) against SparkFun's graphic (source 4/5): left column top to bottom TX0, RXI, RST, GND, 2 … 9 = JP7.12 … JP7.1 by y coordinate; right column RAW, GND, RST, VCC, A3, A2, A1, A0, 13, 12, 11, 10 = JP6.1 … JP6.12; FTDI row left to right GND, GND, VCC, RXI, TXO, DTR = pads 6, 5, 4, 3, 2, 1 by x coordinate, with the top-silk texts BLK (x 1.27) and GRN (x 17.27) at the ends. Match.

Functions: PWM 3, 5, 6, 9, 10, 11 (Arduino page) match the OCnx labels in SparkFun's graphic (OC2B on D3, OC0B D5, OC0A D6, OC1A D9, OC1B D10, OC2A D11). SPI 10 SS, 11 MOSI, 12 MISO, 13 SCK and I2C A4 SDA / A5 SCL agree between Arduino's page and SparkFun's graphic.

Differences found:
- Arduino's page says "8 analog inputs … Four of them are on the headers on the edge of the board; two (inputs 4 and 5) on holes in the interior", while its tech-spec table says 6 analog inputs; the v14 board has A0–A3 on the edge and A4–A7 on inner holes (board file, schematic, SparkFun graphic). I follow the board file.
- RAW range: SparkFun graphic 5–16 V / 3.3–16 V (6–12 / 4–12 V recommended), schematic note "Max Voltage Input: 16VDC", Arduino tech specs 5–12 V / 3.35–12 V. Both in the notes; `function` gives no number.
- SparkFun's graphic labels D7 "IN1" (the ATmega328P's AIN1); not used.

## Choices
- Catalog item: `board` shape, `connector: none` (the board has no USB socket), top view: left JP7.12 to JP7.1, right JP6.1 to JP6.12. The six FTDI pins (JP1.6 to JP1.1, BLK end at the left) and the A4 to A7 pads that lie inside the board off the 0.1 inch grid are `inner` groups: "FTDI" (one row), "A4/A5" (rows JP2.2, JP2.1) and "A6/A7" (rows JP3.2, JP3.1). The FTDI header is not a `top` list because every contact of a board is as wide as its longest number (JP7.12) and a `top` list has a pitch of 24 units, so its contacts would overlap; the drawing centres inner groups, so none of them sits at its real place (the doc says so). Layout of the SparkFun/Arduino v14 PCB only; clones are not covered.
- RAW range and per-pin current: the item note uses the conservative values (Arduino's 5 to 12 V and 3.35 to 12 V, not SparkFun's up to 16 V; 20 mA, not 40 mA) and names the other.
- One item for both models (same PCB), voltage differences in the notes and in VCC's function.
- Header ids are the SparkFun schematic's JP1, JP6, JP7, JP2, JP3.
- Names: D-numbers for the digital pins (D1/D0 with silk TXO/RXI in `function`), `RST` as on the silk, `RAW`, `VCC`; the FTDI pins keep their silk names TXO/RXI/DTR and are type `data`/`signal` (bus to the programming adapter, like ICSP lines); the edge-row D0/D1 are `io`.
- A6/A7 type `signal` (analog only), as in the Nano record.

## Spot-check
1. JP7.12 = D1/TXO, top of the left column — schematic JP7 pin 12 "TXO"; board file JP7 pad 12 net TXO at (1.27, 29.21) with top silk "TXO" at y 28.19; SparkFun graphic first left row "TX0 D1 PD1 TXD".
2. JP7.9 = GND — schematic JP7 pin 9 "GND"; board net GND; graphic fourth left row "GND".
3. JP6.1 = RAW, top of the right column — schematic JP6 pin 1 "RAW"; board pad 1 at (16.51, 29.21), silk "RAW"; graphic first right row "RAW".
4. JP6.9 = D13 (SCK, LED) — schematic JP6 pin 9 "SCK"; board pad 9 at y 8.89, silk "13"; graphic "13 D13 PB5 SCK PCINT5 LED".
5. JP6.12 = D10 (SS, PWM), bottom of the right column — schematic JP6 pin 12 "D10"; board pad 12 at y 1.27, silk "10"; graphic "10 D10 PB2 8-bit SS OC1B".
6. JP1.1 = DTR at the GRN end, JP1.6 = GND at the BLK end — board file JP1 pad 1 net DTR at x 15.24, pad 6 net GND at x 2.54, silk GRN at x 17.27, BLK at x 1.27; graphic "GND GND VCC RXI TXO DTR" with BLK left, GRN right.
7. JP2.1 = A4 (SDA), JP2.2 = A5 (SCL) — schematic JP2; board pads at y 17.907/20.447 with bottom silk A4/A5; graphic "A4/D18 PC4 ADC4 SDA", "A5/D19 PC5 ADC5 SCL".

UNVERIFIED: none of the v14 pins. Layouts of clone "Pro Mini" boards are not covered and must not be assumed to match.
