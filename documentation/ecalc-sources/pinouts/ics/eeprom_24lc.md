# eeprom_24lc — Microchip 24LC32A / 24LC64 / 24LC256 I²C serial EEPROM (DIP-8) / Microchip 24LC32A / 24LC64 / 24LC256 serielles I²C-EEPROM (DIP-8)

Kind: authoritative maker documents (Microchip, one datasheet per part, all three read)
Pins: 8; types: power 1, ground 1, signal 4, data 2

## Decision: one item
The three PDIP pinouts agree pin by pin: A0 1, A1 2, A2 3, VSS 4, SDA 5, SCL 6, WP 7, VCC 8, with no NC pin in the PDIP of any of the three. Checked in each datasheet's Table 2-1 "Pin Function Table", PDIP column, and in each page-1 "Package Types" drawing that includes PDIP. So one item covers all three.

The only NC case in these sheets is outside the DIP: on the 24XX256 in MSOP, pins A0 and A1 are not connected ("Pins A0 and A1 are no connects for the MSOP package only"), and the SOT-23 versions of all three have no address pins. Neither affects the DIP item.

## Part
24LC32A-I/P, 24LC64-I/P and 24LC256-I/P: I²C serial EEPROMs with 32 Kbit (4K × 8), 64 Kbit (8K × 8) and 256 Kbit (32K × 8), in the 8-lead PDIP (Microchip package code "P"). Display name `24LC32A / 24LC64 / 24LC256 (DIP-8)`, the same in all locales; suggested id `eeprom_24lc`.

Members that share this pinout, as far as a datasheet says so:
- 24AA32A and 24LC32A (DS20001713P): same Table 2-1 for both; PDIP listed for both.
- 24AA64, 24FC64 and 24LC64 (DS20001189V): one table; PDIP listed for all three.
- 24AA256, 24LC256 and 24FC256 (DS20001203Y): one table; PDIP listed for all three.
- In each sheet the MSOP, SOIC, SOIJ and TSSOP columns of Table 2-1 have the same numbers as PDIP (except the 24XX256 MSOP, which leaves A0 and A1 unconnected). DFN/TDFN also use 1 to 8 the same way.
- Not this pinout: SOT-23 (5 leads), CSP (balls), X-rotated TSSOP (WP on 1, VCC on 2, …).
- Not claimed: other sizes (24LC01B … 24LC16B, 24LC128, 24LC512) and other makers' 24C parts; not read.

## Pin table
| No. | Name (Microchip) | 24LC32A / 24LC64 / 24LC256 | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | A0 | same in all three | Chip address input A0: with A1 and A2 selects one of eight chips on the bus; tie to VCC or VSS | Chip-Adresseingang A0: wählt mit A1 und A2 einen von acht Bausteinen am Bus; an VCC oder VSS legen | signal |
| 2 | A1 | same in all three | Chip address input A1; tie to VCC or VSS | Chip-Adresseingang A1; an VCC oder VSS legen | signal |
| 3 | A2 | same in all three | Chip address input A2; tie to VCC or VSS | Chip-Adresseingang A2; an VCC oder VSS legen | signal |
| 4 | VSS | same in all three | Ground | Masse | ground |
| 5 | SDA | same in all three | I²C data, bidirectional, open drain: needs a pull-up resistor to VCC | I²C-Daten, bidirektional, Open Drain: braucht einen Pull-up-Widerstand nach VCC | data |
| 6 | SCL | same in all three | I²C clock input | I²C-Takteingang | data |
| 7 | WP | same in all three | Write protect: VSS allows writing, VCC blocks writing (reading still works); do not leave open | Schreibschutz: VSS erlaubt Schreiben, VCC sperrt es (Lesen geht weiter); nicht offen lassen | signal |
| 8 | VCC | same in all three | Supply, 2.5 V to 5.5 V (24AA and 24FC versions from 1.7 V) | Versorgung, 2,5 V bis 5,5 V (24AA- und 24FC-Versionen ab 1,7 V) | power |

Microchip prints `VSS` and `VCC` with "SS" and "CC" in small capitals. No pin carries an overbar.

"0x50" is derived, not printed: control code 1010 plus A2 A1 A0 = 000 gives 1010000 binary = 0x50 (and 0x57 with all three at VCC). The sheets show the control byte as "1 0 1 0 A2 A1 A0 R/W".

## View
Shape `dip`, 8 pins: package from above, notch at the top, pin 1 top left, numbers counter-clockwise. Basis: the page-1 "Package Types" drawing of each sheet ("MSOP/PDIP/SOIC/SOIJ/TSSOP" in DS20001713P and DS20001189V, "MSOP/PDIP" in DS20001203Y) shows a notch at the top, A0, A1, A2, VSS on pins 1–4 down the left and VCC, WP, SCL, SDA on pins 8–5 down the right. I rendered page 1 and the Table 2-1 page of each of the three PDFs to images and looked at them.

## Sources
1. Microchip, 24AA32A/24LC32A "32-Kbit I2C Serial EEPROM", DS20001713P (© 2002–2023). Page 1: Device Selection Table (24LC32A 2.5 V–5.5 V, 400 kHz, packages incl. P), Package Types; page 5: Table 2-1 "Pin Function Table" and sections 2.1–2.4; page 7: control code "1010". https://ww1.microchip.com/downloads/aemDocuments/documents/MPD/ProductDocuments/DataSheets/24AA32A-24LC32A-32-Kbit-I2C-Serial-EEPROM-DS20001713.pdf — read pages 1, 5 and 7, accessed 2026-11-27.
2. Microchip, 24AA64/24FC64/24LC64 "64-Kbit I2C Serial EEPROM", DS20001189V (© 2003–2025). Page 1: Device Selection Table (24LC64 2.5 V–5.5 V, 400 kHz, packages incl. P), Package Types; page 6: Table 2-1 and sections 2.1–2.4; page 8: control code "1010". https://ww1.microchip.com/downloads/aemDocuments/documents/MPD/ProductDocuments/DataSheets/24AA64-24FC64-24LC64-64-Kbit-I2C-Serial-EEPROM-DS20001189.pdf — read pages 1, 6 and 8, accessed 2026-11-27.
3. Microchip, 24AA256/24LC256/24FC256 "256-Kbit I2C Serial EEPROM", DS20001203Y (© 1998–2022). Page 1: Device Selection Table (24LC256 2.5 V–5.5 V, 400 kHz, packages incl. P), Package Types with the MSOP note; page 7: Table 2-1 and sections 2.1–2.4; page 10: control code "1010". https://ww1.microchip.com/downloads/aemDocuments/documents/MPD/ProductDocuments/DataSheets/24AA256-24LC256-24FC256-256K-I2C-Serial-EEPROM-DS20001203.pdf — read pages 1, 7 and 10, accessed 2026-11-27.

## Cross-check
All three sheets agree on every PDIP pin (Table 2-1 PDIP column and page-1 drawing). Inside each sheet the drawing and the table agree. All three say in 2.4 that WP "must be connected to either VSS or VCC" (VSS: writes enabled; VCC: writes inhibited, reads not affected), and in 2.1 that A0 to A2 "must be connected to either VCC or VSS".

Differences, none of them in the PDIP pin numbers:
- 24XX256 MSOP: A0 and A1 not connected (other two sheets: MSOP like PDIP).
- Table 2-1 "Description"/"Function" wording: 24XX32A and 24XX64 say "Chip Address Input" for A0–A2; 24XX256 says "User Configurable Chip Select".
- Page buffer: 32 bytes (24XX32A, 24XX64), 64 bytes (24XX256). Clock: 400 kHz for all 24LC; 1 MHz only for the 24FC versions.
- Supply: all 24LC versions 2.5 V to 5.5 V; 24AA and 24FC versions 1.7 V to 5.5 V.

This is a single-maker item by nature (Microchip's own parts); no second maker was sought.

## Choices
- One item for all three, because the PDIP pinouts are identical and none has an NC pin in PDIP.
- Names follow Microchip (`A0`, `A1`, `A2`, `VSS`, `SDA`, `SCL`, `WP`, `VCC`).
- Types: address inputs and WP `signal` (static levels, like enables elsewhere); SDA and SCL `data` (as the shipped `sn74hc595` types its data and clock pins `data`); VSS `ground`; VCC `power`.
- Pin 8 names the 24LC range (2.5 V to 5.5 V) first, because the item is named after the 24LC parts.

## Spot-check
| No. | Name | Type | Ref |
| --- | --- | --- | --- |
| 1 | A0 | signal | DS20001713P / DS20001189V / DS20001203Y, Table 2-1, PDIP column |
| 4 | VSS | ground | same three tables |
| 5 | SDA | data | same three tables; section 2.2 (open drain) |
| 6 | SCL | data | same three tables |
| 7 | WP | signal | same three tables; section 2.4 |
| 8 | VCC | power | same three tables; page-1 Device Selection Tables |

UNVERIFIED: none of the pin data. The 7-bit address "0x50" in the note is derived from the printed control byte, not printed itself. Every pin number, name and function above was read in all three sheets.
