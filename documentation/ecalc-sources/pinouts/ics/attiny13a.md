# attiny13a — ATtiny13A microcontroller (DIP-8) / ATtiny13A Mikrocontroller (DIP-8)

Kind: authoritative maker document (Microchip, the only maker of the part)
Pins: 8; types: power 1, ground 1, signal 1, io 5

## Part
ATtiny13A-PU: 8-bit AVR microcontroller (1 KB flash, 64 B SRAM, 64 B EEPROM) in the 8-pin PDIP. Display name `ATtiny13A (DIP-8)`, the same in all locales. Item style as the shipped `attiny85`: the name is the port bit, the function lists what else the pin can do.

Members that share this pinout, as far as a datasheet says so:
- ATtiny13A in 8-pin SOIC and SOIJ: Figure 1-1 has one drawing "8-PDIP/SOIJ/SOIC".
- Not this pinout: the 20-pad WQFN and 10-pad VDFN of the same chip (other numbers, DNC pads).
- Not claimed: the older ATtiny13 (without "A"); its own datasheet was not read.

## Pin table
| No. | Name (Microchip) | Alternate functions printed in Figure 1-1 | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | PB5 | PCINT5/RESET/ADC0/dW (RESET with overbar) | Reset input, active low; ADC0 (analogue input 0), PCINT5, dW (debugWIRE) | Reset-Eingang, aktiv low; ADC0 (Analogeingang 0), PCINT5, dW (debugWIRE) | signal |
| 2 | PB3 | PCINT3/CLKI/ADC3 | Port B bit 3; CLKI (external clock input), ADC3, PCINT3 | Port B Bit 3; CLKI (Eingang für externen Takt), ADC3, PCINT3 | io |
| 3 | PB4 | PCINT4/ADC2 | Port B bit 4; ADC2, PCINT4 | Port B Bit 4; ADC2, PCINT4 | io |
| 4 | GND | — | Ground | Masse | ground |
| 5 | PB0 | MOSI/AIN0/OC0A/PCINT0 | Port B bit 0; MOSI (in-system programming), AIN0, OC0A, PCINT0 | Port B Bit 0; MOSI (Programmierung im System), AIN0, OC0A, PCINT0 | io |
| 6 | PB1 | MISO/AIN1/OC0B/INT0/PCINT1 | Port B bit 1; MISO (in-system programming), AIN1, OC0B, INT0, PCINT1 | Port B Bit 1; MISO (Programmierung im System), AIN1, OC0B, INT0, PCINT1 | io |
| 7 | PB2 | SCK/ADC1/T0/PCINT2 | Port B bit 2; SCK (in-system programming), ADC1, T0, PCINT2 | Port B Bit 2; SCK (Programmierung im System), ADC1, T0, PCINT2 | io |
| 8 | VCC | — | Supply, 1.8 V to 5.5 V | Versorgung, 1,8 V bis 5,5 V | power |

Glossary used in the function texts (Table 10-3 "Port B Pins Alternate Functions"): ADC0–ADC3 ADC input channels; PCINT0–5 pin change interrupt sources; dW debugWIRE I/O; CLKI external clock input; AIN0/AIN1 analogue comparator positive/negative input; OC0A/OC0B Timer/Counter0 compare match outputs; INT0 external interrupt 0; T0 Timer/Counter0 clock source; MOSI/MISO/SCK the serial (SPI) programming pins (Table 17-7 "Pin Mapping Serial Programming": MOSI PB0, MISO PB1, SCK PB2). German "Analogeingang" is used as in the shipped `attiny85` item.

Pin-level note for 1 (section 1.1.4; Table 17-3 note 4): en "With the RSTDISBL fuse programmed the pin becomes the weak I/O bit PB5; after that the chip can only be programmed in high-voltage serial mode." / de "Mit programmierter RSTDISBL-Fuse wird der Pin zum schwachen I/O-Bit PB5; danach lässt sich der Chip nur noch im Hochvolt-Seriellmodus programmieren." (The same text as the `attiny85` item; both statements are read in the ATtiny13A sheet: "The reset pin can also be used as a (weak) I/O pin" and "When programming the RSTDISBL fuse, High-voltage Serial programming has to be used to change fuses to perform further programming.")

## Comparison with the shipped `attiny85` item (`internal/ecalc/catalog/pinouts/ics_analog.yaml`)
Pin numbers and port bits are the same on all 8 pins (PB5/RESET 1, PB3 2, PB4 3, GND 4, PB0 5, PB1 6, PB2 7, VCC 8). ADC0–ADC3, PCINT0–5, AIN0/AIN1, OC0A/OC0B, T0, dW and the ISP pins sit on the same pins. The alternate functions differ:

| No. | ATtiny85 item has | ATtiny13A (Figure 1-1, Table 10-3) |
| --- | --- | --- |
| 1 | RESET, ADC0, PCINT5, dW | same |
| 2 | XTAL1/CLKI, ADC3, OC1B inverted, PCINT3 | CLKI, ADC3, PCINT3 — no XTAL1, no OC1B |
| 3 | XTAL2, CLKO, ADC2, OC1B, PCINT4 | ADC2, PCINT4 — no XTAL2, no CLKO, no OC1B |
| 5 | MOSI, DI, SDA, AIN0, OC0A, OC1A inverted, AREF, PCINT0 | MOSI, AIN0, OC0A, PCINT0 — no USI (DI, SDA), no OC1A, no AREF |
| 6 | MISO, DO, AIN1, OC0B, OC1A, PCINT1 | MISO, AIN1, OC0B, INT0, PCINT1 — INT0 is here; no DO, no OC1A |
| 7 | SCK, USCK, SCL, ADC1, T0, INT0, PCINT2 | SCK, ADC1, T0, PCINT2 — no USCK/SCL, no INT0 |
| 8 | 2.7–5.5 V (ATtiny85V 1.8–5.5 V) | 1.8–5.5 V |

The pin-2 and pin-3 notes of the `attiny85` item ("Taken by the crystal when the fuses select one") must NOT be copied: Table 6-1 of the ATtiny13A offers only External Clock, the calibrated internal 4.8/9.6 MHz oscillator and the internal 128 kHz oscillator.

## View
Shape `dip`, 8 pins: package from above, notch at the top, pin 1 top left, numbers counter-clockwise. Basis: Figure 1-1 "Pinout of ATtiny13A", drawing "8-PDIP/SOIJ/SOIC", shows a notch at the top, PB5, PB3, PB4, GND on pins 1–4 down the left and VCC, PB2, PB1, PB0 on pins 8–5 down the right, each with its alternate functions in brackets. The drawing has no "top view" caption; the notch and the counter-clockwise numbering are the usual top view, the same as the shipped `attiny85` item. I rendered PDF pages 8, 62 and 63 to images and looked at them.

## Sources
1. Microchip, "ATtiny13A" data sheet ("Data Sheet Complete"), DS40002307A (© 2021). Page 2: features (8-pin PDIP/SOIC/SOIJ with six programmable I/O lines, operating voltage 1.8–5.5 V, speed grades 0–4 MHz at 1.8–5.5 V, 0–10 MHz at 2.7–5.5 V, 0–20 MHz at 4.5–5.5 V); page 8: Figure 1-1 "Pinout of ATtiny13A" (8-PDIP/SOIJ/SOIC, 20-WQFN, 10-VDFN); page 9: section 1.1 Pin Description (VCC, GND, Port B, RESET "can also be used as a (weak) I/O pin"); page 30: Table 6-1 "Device Clocking Options Select"; page 62: Table 10-3 "Port B Pins Alternate Functions" and Table 10-4; page 63: Table 10-5; page 110: fuse high byte with note 4 on RSTDISBL; page 112: Table 17-7 "Pin Mapping Serial Programming"; section 14.6.2 (ADC reference: VCC or internal 1.1 V). https://ww1.microchip.com/downloads/aemDocuments/documents/MCU08/ProductDocuments/DataSheets/ATtiny13A-Data-Sheet-DS40002307A.pdf — read the pages named, accessed 2026-11-27.

## Cross-check
Inside the datasheet, Figure 1-1 (8-PDIP) and Table 10-3 agree on every alternate function of every pin. Table 10-4 (PB5/RESET/ADC0/PCINT5, PB4/ADC2/PCINT4, PB3/ADC3/CLKI/PCINT3) and the PB2 and PB1 columns of Table 10-5 agree as well.

Contradiction inside the datasheet: the PB0 column heading of Table 10-5 reads "PB0/MOSI/AIN0/AREF/OC0A/PCINT0", with AREF, while Figure 1-1 and Table 10-3 list no AREF on PB0, and section 14.6.2 says the ADC reference "can be selected as either VCC, or internal 1.1V reference" (no external reference). AREF is left out; the heading looks like a leftover from the ATtiny25/45/85 family.

No second maker exists (the ATtiny13A is made only by Microchip, formerly Atmel); the shipped `attiny85` item also rests on a single maker.

## Choices
- Names and pin 1 notation follow the `attiny85` item (`PB5/RESET`; port bit as the name, alternate functions in the text).
- Types follow the `attiny85` item: pin 1 `signal` (reset by default), PB0–PB4 `io`, GND `ground`, VCC `power`.
- "MOSI/MISO/SCK (in-system programming)" instead of "(SPI and ISP)": the ATtiny13A has no SPI or USI peripheral; Table 10-3 names these pins "SPI Host Data Output / Client Data Input" and so on, and Table 17-7 lists them as the serial programming pins.
- RESET carries an overbar in Figure 1-1; ECalc drops it and the function says "active low".

## Spot-check
| No. | Name | Type | Ref |
| --- | --- | --- | --- |
| 1 | PB5/RESET | signal | DS40002307A, Figure 1-1 (page 8); Table 10-3 (page 62) |
| 2 | PB3 | io | DS40002307A, Figure 1-1 (page 8); Table 10-3 (page 62) |
| 4 | GND | ground | DS40002307A, Figure 1-1 (page 8) |
| 6 | PB1 | io | DS40002307A, Figure 1-1 (page 8); Table 10-3 INT0 on PB1 (page 62) |
| 7 | PB2 | io | DS40002307A, Figure 1-1 (page 8); Table 17-7 SCK (page 112) |
| 8 | VCC | power | DS40002307A, Figure 1-1 (page 8); features (page 2) |

UNVERIFIED: the AREF entry in the Table 10-5 heading (left out, see Cross-check). The older ATtiny13 (non-A) was not checked. Every pin number, name and function above was read.
