# attiny85 — ATtiny85 microcontroller (DIP-8) / ATtiny85 Mikrocontroller (DIP-8)

Kind: authoritative maker document (Atmel datasheet, served by Microchip, which bought Atmel; no second maker exists). Part: ATtiny85 8-bit AVR microcontroller in the 8-pin PDIP, 8 pins. Pin-compatible relatives in the same figure: Figure 1-1 is titled "Pinout ATtiny25/45/85" and drawn for "PDIP/SOIC/TSSOP", so the ATtiny25, ATtiny45 and the V (low-voltage) grades share it. Exception read in section 1.1.3: on the ATtiny25 only, PB3 and PB4 (pins 2 and 3) are exchanged in ATtiny15 Compatibility Mode.
Pins: 8; types: power 1, ground 1, signal 1, io 5

## Pins
Names follow the `atmega328p` item: the name is the port bit; the function lists the main alternate functions from Figure 1-1 and Table 10-3. The reset pin is named like `PC6/RESET` there. Atmel prints overbars on `RESET` (pin 1), on `OC1B` at pin 2 and on `OC1A` at pin 5 (the inverted timer-1 outputs); they are dropped here and the function text says "active low" or "inverted".

| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | `PB5/RESET` | Reset input, active low; ADC0 (analog input 0), PCINT5, dW (debugWIRE) | Reset-Eingang, aktiv low; ADC0 (Analogeingang 0), PCINT5, dW (debugWIRE) | signal |
| 2 | `PB3` | Port B bit 3; XTAL1/CLKI (crystal or clock input), ADC3, OC1B inverted, PCINT3 | Port B Bit 3; XTAL1/CLKI (Quarz- oder Takteingang), ADC3, OC1B invertiert, PCINT3 | io |
| 3 | `PB4` | Port B bit 4; XTAL2 (crystal output), CLKO, ADC2, OC1B, PCINT4 | Port B Bit 4; XTAL2 (Quarzausgang), CLKO, ADC2, OC1B, PCINT4 | io |
| 4 | `GND` | Ground | Masse | ground |
| 5 | `PB0` | Port B bit 0; MOSI (SPI and ISP), DI, SDA (USI two-wire), AIN0, OC0A, OC1A inverted, AREF, PCINT0 | Port B Bit 0; MOSI (SPI und ISP), DI, SDA (USI-Zweidraht), AIN0, OC0A, OC1A invertiert, AREF, PCINT0 | io |
| 6 | `PB1` | Port B bit 1; MISO (SPI and ISP), DO, AIN1, OC0B, OC1A, PCINT1 | Port B Bit 1; MISO (SPI und ISP), DO, AIN1, OC0B, OC1A, PCINT1 | io |
| 7 | `PB2` | Port B bit 2; SCK (SPI and ISP), USCK, SCL (USI two-wire), ADC1, T0, INT0, PCINT2 | Port B Bit 2; SCK (SPI und ISP), USCK, SCL (USI-Zweidraht), ADC1, T0, INT0, PCINT2 | io |
| 8 | `VCC` | Supply, 2.7 V to 5.5 V (ATtiny85V: 1.8 V to 5.5 V) | Versorgung, 2,7 V bis 5,5 V (ATtiny85V: 1,8 V bis 5,5 V) | power |

Suggested pin note for 1 (read, section 1.1.4, section 10.3.1 "Port B, Bit 5" and Table 20-4): "With the RSTDISBL fuse programmed the pin becomes the weak I/O bit PB5; after that the chip can only be programmed in high-voltage serial mode." / "Mit programmierter RSTDISBL-Fuse wird der Pin zum schwachen I/O-Bit PB5; danach lässt sich der Chip nur noch im Hochvolt-Seriellmodus programmieren."

Suggested pin note for 2 and 3 (read, section 10.3.1): "Taken by the crystal when the fuses select one; free as I/O with the internal RC oscillator." / "Vom Quarz belegt, wenn die Fuses einen wählen; mit dem internen RC-Oszillator frei als I/O."

Suggested item note (read, Table 20-10 and section 10.3.1): in-system programming uses MOSI = PB0 (pin 5), MISO = PB1 (pin 6), SCK = PB2 (pin 7) and RESET (pin 1). SDA and SCL belong to the USI (Universal Serial Interface) in two-wire mode. Speed grades (page 1): ATtiny85 0–10 MHz at 2.7–5.5 V and 0–20 MHz at 4.5–5.5 V; ATtiny85V 0–4 MHz at 1.8–5.5 V and 0–10 MHz at 2.7–5.5 V. / Die Programmierung im System nutzt MOSI = PB0 (Pin 5), MISO = PB1 (Pin 6), SCK = PB2 (Pin 7) und RESET (Pin 1). SDA und SCL gehören zur USI im Zweidrahtmodus. Taktgrenzen (Seite 1): ATtiny85 0–10 MHz bei 2,7–5,5 V und 0–20 MHz bei 4,5–5,5 V; ATtiny85V 0–4 MHz bei 1,8–5,5 V und 0–10 MHz bei 2,7–5,5 V.

## View
Shape `dip`, 8 pins: package from above, notch at the top, pin 1 top left, numbers counter-clockwise. Basis: Figure 1-1 "Pinout ATtiny25/45/85", drawing labelled "PDIP/SOIC/TSSOP" (note: "TSSOP only for ATtiny45/V"), shows the notch on top, 1–4 down the left and 8–5 down the right. The drawing has no "Top View" caption; the notch and the counter-clockwise numbering are the usual top view. I rendered PDF page 2 to an image (3750 pixels wide) and looked at a crop of the figure, including the overbars.

## Sources
1. Atmel (now Microchip Technology), "Atmel 8-bit AVR Microcontroller with 2/4/8K Bytes In-System Programmable Flash, ATtiny25/V / ATtiny45/V / ATtiny85/V" datasheet, 2586Q–AVR–08/2013 (revision Q, August 2013). Page 1: features (operating voltage, speed grade, packages); page 2: Figure 1-1; pages 2–3: section 1.1 Pin Descriptions; pages 60–62: section 10.3.1, Table 10-3 "Port B Pins Alternate Functions" and the per-bit descriptions; page 148: Table 20-4 Fuse High Byte (RSTDISBL and its notes); page 151: Table 20-10 "Pin Mapping Serial Programming". https://ww1.microchip.com/downloads/en/DeviceDoc/Atmel-2586-AVR-8-bit-Microcontroller-ATtiny25-ATtiny45-ATtiny85_Datasheet.pdf — accessed 2026-11-24.

## Cross-check
One maker document is enough: Atmel/Microchip is the only maker. Inside the document, Figure 1-1, section 1.1, Table 10-3 with the per-bit descriptions, and Table 20-10 agree: PB5 = RESET/dW/ADC0/PCINT5 (pin 1), PB3 = XTAL1/CLKI/ADC3/OC1B inverted/PCINT3 (pin 2), PB4 = XTAL2/CLKO/ADC2/OC1B/PCINT4 (pin 3), GND (pin 4), PB0 = MOSI/DI/SDA/AIN0/OC0A/OC1A inverted/AREF/PCINT0 (pin 5), PB1 = MISO/DO/AIN1/OC0B/OC1A/PCINT1 (pin 6), PB2 = SCK/USCK/SCL/ADC1/T0/INT0/PCINT2 (pin 7), VCC (pin 8). No disagreement.

## Choices
- Catalog item: names follow the `atmega328p` item (`PB5/RESET` as `PC6/RESET`); overbars are dropped, the function text says "active low" for RESET and "inverted" for OC1B (pin 2) and OC1A (pin 5); pins 1 to 3 carry the RSTDISBL and crystal notes; the QFN/MLF package is left out.
- Naming and typing copy the `atmega328p` item: port pins `io` with the port bit as name; the reset pin `signal` with name `PB5/RESET` (as `PC6/RESET`), because RESET is its default function (RSTDISBL unprogrammed by default, Table 20-4).
- Section 1.1.4 says "The reset pin can also be used as a (weak) I/O pin"; the note says "weak I/O" accordingly.
- Overbars: Figure 1-1 has them on RESET, on OC1B at PB3 and on OC1A at PB0; section 10.3.1 describes those two as "Inverted Output Compare Match output". The function text writes "inverted".
- "SPI and ISP" for MOSI/MISO/SCK: section 10.3.1 describes them as SPI channel pins; Table 20-10 maps them for serial programming.
- Supply: page 1, "1.8 - 5.5V for ATtiny25V/45V/85V" and "2.7 - 5.5V for ATtiny25/45/85".
- QFN/MLF (20 pads, DNC pins) left out.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | `PB5/RESET` | signal | Atmel 2586Q, Figure 1-1 (page 2); section 1.1.4 (page 3); Table 10-3 (page 60) |
| 2 | `PB3` | io | Atmel 2586Q, Figure 1-1 (page 2); Table 10-3 (page 60) |
| 4 | `GND` | ground | Atmel 2586Q, Figure 1-1 (page 2); section 1.1.2 (page 2) |
| 5 | `PB0` | io | Atmel 2586Q, Figure 1-1 (page 2); section 10.3.1 "Port B, Bit 0" (page 62); Table 20-10 (page 151) |
| 7 | `PB2` | io | Atmel 2586Q, Figure 1-1 (page 2); section 10.3.1 "Port B, Bit 2" (page 61); Table 20-10 (page 151) |
| 8 | `VCC` | power | Atmel 2586Q, Figure 1-1 (page 2); section 1.1.1 (page 2) |

Counts for the vector: power 1, ground 1, signal 1, io 5.

UNVERIFIED: whether Microchip has a revision newer than 2586Q (August 2013); the file read is the one at the Microchip URL above, and Microchip's product pages answered HTTP 403 to the fetch tool, so the current revision could not be checked. Every pin number, name and function above was read in revision Q.
