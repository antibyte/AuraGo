# mikrobus — mikroBUS socket (2 x 1x8) / mikroBUS-Sockel (2 x 1x8)

Kind: authoritative maker document (positions and names); pin numbers cross-checked in two board guides of a second maker
Pins: 16; types: power 2, ground 2, signal 5, data 7

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | AN | Analog input | Analogeingang | signal |
| 2 | RST | Reset | Reset | signal |
| 3 | CS | SPI chip select | SPI-Chipauswahl | signal |
| 4 | SCK | SPI clock | SPI-Takt | data |
| 5 | MISO | SPI data, master in slave out (add-on board marks it SDO) | SPI-Daten, Master-Eingang (auf dem Modul als SDO beschriftet) | data |
| 6 | MOSI | SPI data, master out slave in (add-on board marks it SDI) | SPI-Daten, Master-Ausgang (auf dem Modul als SDI beschriftet) | data |
| 7 | +3.3V | 3.3 V supply | 3,3-V-Versorgung | power |
| 8 | GND | Reference ground | Bezugsmasse | ground |
| 9 | GND | Reference ground | Bezugsmasse | ground |
| 10 | +5V | 5 V supply | 5-V-Versorgung | power |
| 11 | SDA | I2C data | I2C-Daten | data |
| 12 | SCL | I2C clock | I2C-Takt | data |
| 13 | TX | UART transmit of the mainboard (add-on board marks this pin RX) | UART-Senden der Hauptplatine (auf dem Modul als RX beschriftet) | data |
| 14 | RX | UART receive of the mainboard (add-on board marks this pin TX) | UART-Empfang der Hauptplatine (auf dem Modul als TX beschriftet) | data |
| 15 | INT | Hardware interrupt | Hardware-Interrupt | signal |
| 16 | PWM | PWM output | PWM-Ausgang | signal |

## View
Written for the mainboard socket seen from above (the side the add-on board is plugged into), mikroBUS logo on top, notch at the lower right: `drawn: socket`.

Rows (left pin, right pin), top to bottom:
- [1 AN, 16 PWM]
- [2 RST, 15 INT]
- [3 CS, 14 RX]
- [4 SCK, 13 TX]
- [5 MISO, 12 SCL]
- [6 MOSI, 11 SDA]
- [7 +3.3V, 10 +5V]
- [8 GND, 9 GND]

The other view (mirror) is the add-on board's pin side seen from below (pin tips): PWM top left, AN top right. The add-on board's top side, seen from above, has the same layout as the socket (AN top left, notch bottom right), because it plugs in face up.

Basis:
- mikroBUS standard, page 6, "Pinout specification" figure ("Pin functions/silkscreen markings for the socket"): left 1x8 header from the top AN, RST, CS, SCK, MISO, MOSI, +3.3V, GND; right header from the top PWM, INT, RX, TX, SCL, SDA, +5V, GND; square pads at AN and PWM, the diagonal notch at the lower right, logo on top. Text: "+3.3V and GND on the left, and 5V and GND on the right 1x8 header".
- Page 7, "Full mikroBUS silkscreen markings": the same layout is labelled "Front view"; the "Rear view" beside it shows PWM/RC0 top left and RA2/AN top right, the mirror image. Item 6: "A diagonal notch below the right-hand side pinout (serves as a guideline for add-on board insertion)".
- Page 12, add-on board front view: AN column at the left, PWM column at the right, notch at the bottom right corner ("make sure to retain the notch on the bottom right corner of the board"), so board top and socket top coincide.
- Page 9 dimensions: columns 900 mil (22.86 mm) apart, 100 mil pitch.

The standard draws no pin numbers. Numbers 1 to 16 come from Microchip's board guides (see Sources) and run counter-clockwise: down the left column, then up the right column, so pins 8 and 9 are the two GND pins at the bottom. The item note says this.

## Sources
1. MikroElektronika, "mikroBUS Standard specifications", revision 2.00, May 2015 (cover "June, 2015"), https://download.mikroe.com/documents/standards/mikrobus/mikrobus-standard-specification-v200.pdf — page 6 (pinout figure and function list), page 7 (front and rear view, notch), page 9 (dimensions), page 11 (add-on board marking: "sockets RX TX MISO MOSI / boards TX RX SDO SDI", "UART markings are criss-crossed because receivers are matched to transmiters"), page 12 (board shape, notch). Rendered and read. Accessed 2026-11-24.
2. Microchip, PIC32CK SG01/GC01 Curiosity Ultra User's Guide, section 2.4 "mikroBUS Socket", table 2-3, https://onlinedocs.microchip.com/oxy/GUID-44D0ED0E-9CC5-4BBF-A3CE-D03D8049B2B0-en-US-3/GUID-1831C52D-1188-4C9A-A936-7BDDDA9AAF5F.html — pin numbers 1 to 16 with names. Accessed 2026-11-24.
3. Microchip, PIC32 WFI32E Curiosity Board User's Guide, section 3.5 "mikroBUS Socket (J200)", table 3-5, https://onlinedocs.microchip.com/oxy/GUID-B2A07637-605D-4BC2-B21D-BA645340D9E6-en-US-3/GUID-ACC879C1-4121-48FC-8E99-5B7F4191AD18.html — the same 16 numbers and names. Accessed 2026-11-24.

## Cross-check
- Names: the standard (page 6) and both Microchip tables agree on all 16 names. Microchip writes "SPI Host In Client Out" for MISO; the WFI32E table writes "SPI Host Input Client Input" for MOSI, an obvious typo in that table (the Curiosity Ultra guide has "SPI Host Out Client In").
- Numbers: both Microchip tables: 1 AN, 2 RST, 3 CS, 4 SCK, 5 MISO, 6 MOSI, 7 +3.3V, 8 GND, 9 GND, 10 +5V, 11 SDA, 12 SCL, 13 TX, 14 RX, 15 INT, 16 PWM. Mapped onto the standard's positions this is exactly the counter-clockwise order. Both are Microchip documents, so the numbering has one publisher; the standard itself defines no numbers.
- Supply: the standard names +3.3V and +5V only; no current limit is given.

## Choices
- Catalog item: `rows` with two pins per row (left, right), `outline: rect`, `contact: square`, `drawn: socket`, no `key` (the diagonal notch at the lower right is no rect key; the note names it). The 900 mil gap between the columns is not drawn; the note says so. The note states that the standard defines positions and names and that the numbers 1 to 16 are Microchip's.
- UART: names follow the socket (mainboard) side. The function text says that the add-on board's silkscreen is crossed, because a board labelled TX sits on the socket's RX pin. This is the most common wiring mistake with these boards.
- CS, RST, INT, PWM and AN are `signal`; SCK, MISO, MOSI, SDA, SCL, TX, RX are `data`, following the avr_isp_6 precedent.
- MISO/MOSI keep the standard's names; the standard itself shows SDI/SDO as the vertical short form (page 8) and on boards (page 11).

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | AN | signal | mikroBUS standard 2.00, page 6 figure, left header top (square pad); Microchip PIC32CK guide table 2-3 pin 1 |
| 7 | +3.3V | power | standard page 6, left header 7th from top; text "+3.3V and GND on the left"; Microchip table 2-3 pin 7 |
| 10 | +5V | power | standard page 6, right header 7th from top; Microchip table 2-3 pin 10, WFI32E table 3-5 pin 10 |
| 14 | RX | data | standard page 6, right header 3rd from top; page 11 table "sockets RX / boards TX"; Microchip table 2-3 pin 14 |
| 16 | PWM | signal | standard page 6, right header top (square pad); Microchip tables pin 16 |

UNVERIFIED: no MikroElektronika document read that numbers the pins 1 to 16 (numbers rest on two Microchip guides). Supply current limits: not stated in the standard, left out.
