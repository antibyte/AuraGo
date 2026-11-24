# arduino_leonardo — Arduino Leonardo headers / Arduino Leonardo Stiftleisten

Kind: authoritative maker document (Arduino full pinout + Arduino schematic), board Arduino Leonardo (SKU A000057 with headers / A000052 without; schematic Leonardo_Rev3d)
Pins: 38; types: power 5, ground 4, signal 3, data 3, io 22, nc 1

## View
Board from the component side, portrait, Micro-USB socket and DC jack at the top (`connector: top`). Basis: Arduino's LEONARDO full pinout page 1, rendered and looked at: DC jack upper left, grey Micro-USB socket upper right, power/analog headers down the left edge, digital headers down the right edge, ICSP 2x3 at the bottom middle — the same layout and orientation as the Uno R3 figure.

Positions measured on page 1 (pitch 16.5 px; right column top = 0): right SCL, SDA, AREF, GND, D13 … D8 at 0 … 9; D7 … D0 at 10.6 … 17.6. Left NC … VIN at 3.6 … 10.6 (VIN level with D7), A0 … A5 at 12.6 … 17.6 (A5 level with D0). Identical to the Uno R3 figure, so the Uno's lists fit:
- left (top to bottom): null, null, null, null, J3.1 … J3.8, null, J4.1 … J4.6
- right (top to bottom): JP1.10 … JP1.1, null, J2.8 … J2.1
- inner ICSP: upper row ICSP.5, ICSP.3, ICSP.1; lower row ICSP.6, ICSP.4, ICSP.2 (pinout page 3: numbered circles 5 3 1 above, 6 4 2 below; pin 1 upper right).

## Header numbers
The schematic (Leonardo_Rev3d, 28/01/19) names the headers JP1 (10x1F-H8.5), J2 (8x1F-H8.5), J3 (8x1F-H8.5), J4 (6x1F-H8.5) and ICSP (3x2M). Note that these differ from the Uno schematic's POWER/AD/IOH/IOL names; the positions are the same.
- J3.1 … J3.8 = RFU/NC, IOREF, RESET, +3V3, +5V, GND, GND, VIN (pin 1 at the top).
- J4.1 … J4.6 = A0 … A5 (pin 1 at the top).
- JP1.1 … JP1.10 = IO8, IO9, IO10, IO11, IO12, IO13, GND, AREF, D2/SDA, D3/SCL (pin 1 at D8, so pin 10 is the top).
- J2.1 … J2.8 = D0/RX, D1/TX, D2/SDA, D3/SCL, D4, D5, D6, D7 (pin 1 at the bottom).

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| J3.1 | NC | Not connected (schematic: RFU, reserved) | Nicht belegt (Schaltplan: RFU, reserviert) | nc |
| J3.2 | IOREF | Tells shields the logic voltage; tied to 5 V | Zeigt Shields die Logikspannung; liegt an 5 V | power |
| J3.3 | RESET | Reset; pull low to restart the ATmega32U4 | Reset; low startet den ATmega32U4 neu | signal |
| J3.4 | 3V3 | 3.3 V output, 50 mA at most | 3,3-V-Ausgang, höchstens 50 mA | power |
| J3.5 | 5V | 5 V rail | 5-V-Schiene | power |
| J3.6 | GND | Ground | Masse | ground |
| J3.7 | GND | Ground | Masse | ground |
| J3.8 | VIN | Supply input, feeds the 5 V regulator (pinout: 6–20 V; tech specs: 7–12 V nominal) | Versorgungseingang, speist den 5-V-Regler (Pinout: 6–20 V; Tech Specs: 7–12 V nominal) | power |
| J4.1 | A0 | Analog input ADC7; digital D18; JTAG TDI (PF7) | Analogeingang ADC7; digital D18; JTAG TDI (PF7) | io |
| J4.2 | A1 | Analog input ADC6; digital D19; JTAG TDO (PF6) | Analogeingang ADC6; digital D19; JTAG TDO (PF6) | io |
| J4.3 | A2 | Analog input ADC5; digital D20; JTAG TMS (PF5) | Analogeingang ADC5; digital D20; JTAG TMS (PF5) | io |
| J4.4 | A3 | Analog input ADC4; digital D21; JTAG TCK (PF4) | Analogeingang ADC4; digital D21; JTAG TCK (PF4) | io |
| J4.5 | A4 | Analog input ADC1; digital D22 (PF1); not I2C | Analogeingang ADC1; digital D22 (PF1); kein I2C | io |
| J4.6 | A5 | Analog input ADC0; digital D23 (PF0); not I2C | Analogeingang ADC0; digital D23 (PF0); kein I2C | io |
| JP1.10 | SCL | I2C SCL; same line as D3 | I2C SCL; dieselbe Leitung wie D3 | io |
| JP1.9 | SDA | I2C SDA; same line as D2 | I2C SDA; dieselbe Leitung wie D2 | io |
| JP1.8 | AREF | Reference voltage of the ADC | Referenzspannung des ADC | signal |
| JP1.7 | GND | Ground | Masse | ground |
| JP1.6 | D13 | PWM; on-board LED (PC7) | PWM; LED auf der Platine (PC7) | io |
| JP1.5 | D12 | Digital pin; analog input A11 (ADC9) (PD6) | Digitaler Pin; Analogeingang A11 (ADC9) (PD6) | io |
| JP1.4 | D11 | PWM (PB7); not SPI | PWM (PB7); kein SPI | io |
| JP1.3 | D10 | PWM; analog input A10 (ADC13) (PB6) | PWM; Analogeingang A10 (ADC13) (PB6) | io |
| JP1.2 | D9 | PWM; analog input A9 (ADC12) (PB5) | PWM; Analogeingang A9 (ADC12) (PB5) | io |
| JP1.1 | D8 | Digital pin; analog input A8 (ADC11) (PB4) | Digitaler Pin; Analogeingang A8 (ADC11) (PB4) | io |
| J2.8 | D7 | Digital pin; interrupt INT6 (PE6) | Digitaler Pin; Interrupt INT6 (PE6) | io |
| J2.7 | D6 | PWM; analog input A7 (ADC10) (PD7) | PWM; Analogeingang A7 (ADC10) (PD7) | io |
| J2.6 | D5 | PWM (PC6) | PWM (PC6) | io |
| J2.5 | D4 | Digital pin; analog input A6 (ADC8) (PD4) | Digitaler Pin; Analogeingang A6 (ADC8) (PD4) | io |
| J2.4 | D3 | PWM; I2C SCL; interrupt INT0 (PD0) | PWM; I2C SCL; Interrupt INT0 (PD0) | io |
| J2.3 | D2 | I2C SDA; interrupt INT1 (PD1) | I2C SDA; Interrupt INT1 (PD1) | io |
| J2.2 | D1 | UART TX of Serial1; interrupt INT3 (PD3) | UART TX von Serial1; Interrupt INT3 (PD3) | io |
| J2.1 | D0 | UART RX of Serial1; interrupt INT2 (PD2) | UART RX von Serial1; Interrupt INT2 (PD2) | io |
| ICSP.1 | CIPO | SPI CIPO, formerly MISO (PB3); SPI only here | SPI CIPO, früher MISO (PB3); SPI nur hier | data |
| ICSP.2 | 5V | 5 V rail | 5-V-Schiene | power |
| ICSP.3 | SCK | SPI clock (PB1) | SPI-Takt (PB1) | data |
| ICSP.4 | COPI | SPI COPI, formerly MOSI (PB2) | SPI COPI, früher MOSI (PB2) | data |
| ICSP.5 | RESET | Reset of the ATmega32U4 | Reset des ATmega32U4 | signal |
| ICSP.6 | GND | Ground | Masse | ground |

Item notes (en, as in the catalog): Logic level 5 V. Current per I/O pin: Arduino's pinout says 20 mA at most, the tech specs say 10 mA; stay at 10 mA. The 3V3 pin gives 50 mA at most. VIN: the pinout says 6–20 V, the tech specs 7–12 V nominal; stay within 7–12 V. I2C is on D2 (SDA) and D3 (SCL) and on the two pins beside AREF, not on A4 and A5. SPI is only on the ICSP block, not on D11 to D13. The USB serial port is native (Serial); D0 and D1 are a separate hardware UART (Serial1), not linked to USB, so a circuit on them does not disturb uploads. A6 to A11 are extra analog inputs on D4, D6, D8, D9, D10 and D12. PWM is on D3, D5, D6, D9, D10, D11 and D13. There is no ATmega16U2 and no second ICSP header on this board.

## Sources
1. Arduino, LEONARDO full pinout, SKU A000057, "Last update: 25/11/2025", https://docs.arduino.cc/resources/pinouts/A000057-full-pinout.pdf — page 1 (board, all headers), page 2 (alternate functions: ADC numbers, timers, INTn, PCINTn, JTAG on A0–A3, UART/I2C), page 3 (ICSP numbering and table); rendered and zoomed; accessed 2026-11-24
2. Arduino, schematic Leonardo_Rev3d (Arturo Guadalupi, 28/01/19), https://docs.arduino.cc/resources/schematics/A000057-schematics.pdf — page 1: JP1, J2, J3, J4, ICSP and the ATmega32U4 net labels; rendered and zoomed; accessed 2026-11-24
3. Arduino docs-content, Leonardo tech-specs.yml and features.md, https://github.com/arduino/docs-content/tree/main/content/hardware/hero/boards/leonardo — "Digital I/O Pins: 20", "PWM pins: 7", "Analog input pins: 12", "I/O Voltage: 5V", "Input voltage (nominal): 7-12V", "DC Current per I/O Pin: 10 mA"; accessed 2026-11-24
4. Arduino docs-content, "Getting Started with the Arduino Leonardo, Leonardo ETH and Micro" (content/retired/06.getting-started-guides/ArduinoLeonardoMicro/ArduinoLeonardoMicro.md), https://docs.arduino.cc/retired/getting-started-guides/ArduinoLeonardoMicro/ — Serial is USB, hardware serial on pins 0 and 1 is Serial1; read via the GitHub source; accessed 2026-11-24
5. Arduino, ArduinoCore-avr variants/leonardo/pins_arduino.h, https://raw.githubusercontent.com/arduino/ArduinoCore-avr/master/variants/leonardo/pins_arduino.h — SDA = 2, SCL = 3, SPI SS 17 / MOSI 16 / MISO 14 / SCK 15, A0 = 18 … A11 = 29 (D12), digitalPinHasPWM = 3, 5, 6, 9, 10, 11, 13; accessed 2026-11-24
6. Arduino, analogWrite() reference source, https://raw.githubusercontent.com/arduino/reference-en/master/Language/Functions/Analog%20IO/analogWrite.adoc — "Leonardo, Micro, Yún | 3, 5, 6, 9, 10, 11, 13"; accessed 2026-11-24

## Cross-check
Schematic (source 2) against full pinout (source 1), pin by pin:
- J3.1–8: pin 1 unconnected ("RFU"), pin 2 IOREF tied to +5V, RESET, +3V3, +5V, GND, GND, VIN — pinout page 1 NC, IOREF, RESET, +3V3, +5V, GND, GND, VIN. Match (only the name RFU vs NC differs).
- J4.1–6 = A0 … A5 — pinout A0 PF7 … A5 PF0, same order. Match.
- JP1.1–10 = IO8, IO9*, IO10*, IO11*, IO12, IO13*, GND, AREF, D2/SDA, D3/SCL — pinout from the top ~D3/SCL, D2/SDA, AREF, GND, ~D13, D12, ~D11, ~D10, ~D9, D8. Match.
- J2.1–8 = D0/RX, D1/TX, D2/SDA, D3/SCL, D4, D5*, D6*, D7 — pinout bottom-up D0/RX PD2, D1/TX PD3, D2 PD1, ~D3 PD0, D4 PD4, ~D5 PC6, ~D6 PD7, D7 PE6. Match.
- ICSP 1–6 = MISO, +5V, SCK, MOSI, RESET, GND (zoomed: pin 1 line from the MISO label, pin 3 from SCK) — pinout page 3: 1 CIPO PB3, 2 +5V, 3 SCK PB1, 4 COPI PB2, 5 RESET, 6 GND. Match.
- PWM: schematic asterisks on IO9, IO10, IO11, IO13, D5, D6 (D3/SCL has none on the header label); pinout tildes on D3, D5, D6, D9, D10, D11, D13; reference and pins_arduino.h agree on 3, 5, 6, 9, 10, 11, 13. The missing asterisk on D3 in the schematic is cosmetic (pinout p.2 shows OC0B on PD0).
- Port and ADC numbers in `function` are from pinout page 2 and agree with the ATmega32U4 net labels on the schematic (e.g. PB6 = "ADC13/OC1B/OC4B/PCINT6" on U1 pin 30, net IO10).

Differences found: J3.1 is "RFU" in the schematic, "NC" in the pinout; per-pin current 20 mA (pinout) vs 10 mA (tech specs); VIN 6–20 V (pinout legend) vs 7–12 V nominal (tech specs). No pin-position difference.

## Choices
- Catalog item: `board` shape, `connector: top`, top view, the same lists as `arduino_uno` (left: four gaps, J3.1 to J3.8, a gap, J4.1 to J4.6; right: JP1.10 to JP1.1, a gap, J2.8 to J2.1) because the figure has the Uno's positions, plus the `inner` group ICSP (upper row ICSP.5, ICSP.3, ICSP.1, lower row ICSP.6, ICSP.4, ICSP.2).
- Per-pin current and VIN: the item note uses the conservative value (10 mA from the tech specs, 7 to 12 V from the tech specs) and names the other (20 mA and 6 to 20 V in the pinout); the function of VIN names both.
- Header ids follow the Leonardo schematic (J3, J4, JP1, J2, ICSP), not the Uno's names, because the Leonardo schematic is the one that applies; positions are the same as the Uno.
- Name `NC` for J3.1 (pinout), with "RFU" in `function`.
- JP1.9/JP1.10 named `SDA`/`SCL`, type io, same nets as D2/D3 (schematic labels D2/SDA and D3/SCL on both headers).
- D11 and D13 carry "not SPI" only where a Uno user would expect it (D11); D12/D13 functions as read.
- The A6–A11 alternates go into `function` of D4, D6, D8, D9, D10, D12; names stay D-numbers.

## Spot-check
1. JP1.10 = SCL (D3, PD0) at the top right — schematic JP1 pin 10 "D3/SCL"; pinout p.1/p.2 first right label "~D3/SCL" beside "SCL".
2. J2.4 = D3 (PD0, OC0B, INT0) — schematic J2 pin 4 "D3/SCL"; pinout p.2 "~D3 PD0 SCL OC0B INT[0]".
3. J2.1 = D0/RX (PD2, RXD1) — schematic J2 pin 1 "D0/RX"; pinout p.2 "D0/RX PD2 RXD1 INT[2]"; guide: pins 0 and 1 are Serial1.
4. JP1.5 = D12 / A11 (PD6, ADC9) — schematic JP1 pin 5 "IO12"; pinout p.2 "D12 A11 PD6 ADC[9]"; pins_arduino.h "A11 = PIN_A11 // D12".
5. J4.5 = A4 (PF1), not I2C — schematic J4 pin 5 "A4"; pinout p.1 "ADC[1] PF1 A4 D22" with no SDA label.
6. ICSP.4 = COPI (PB2) — schematic ICSP pin 4 "MOSI"; pinout p.3 "4 COPI PB2".
7. J3.8 = VIN, J3.2 = IOREF on +5V — schematic J3 pins 8 and 2; pinout p.1 left column.

UNVERIFIED: none of the pins. Not verified: which of the two per-pin current figures Arduino intends as the limit (both noted).
