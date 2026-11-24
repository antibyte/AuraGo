# arduino_mega — Arduino Mega 2560 Rev3 headers / Arduino Mega 2560 Rev3 Stiftleisten

Kind: authoritative maker document (Arduino full pinout + Arduino schematic + Arduino datasheet), board Arduino Mega 2560 Rev3 (SKU A000067, schematic MEGA2560_Rev3e)
Pins: 92; types: power 7, ground 6, signal 3, data 3, io 72, nc 1

## View
Board from the component side, portrait, USB-B socket and DC jack at the top (`connector: top`). Basis: Arduino's MEGA 2560 REV3 full pinout, page 1, rendered and looked at: the board is drawn portrait with the DC jack at the upper left and the grey USB-B socket at the upper right; the POWER/analog headers run down the left edge, the digital/communication headers down the right edge, and the 2x18 double row lies across the bottom edge. This is the usual landscape photo (USB at the left) turned 90° clockwise.

Page 3 of the same file draws the double row alone in landscape (thumbnail captioned TOP, USB at the left): +5V/+5V at the top end, GND/GND at the bottom end, the column nearer the board's middle carries D22, D24 … D52, the column at the board edge D23, D25 … D53. Turned 90° clockwise into the portrait view of page 1: the +5V end goes to the right, the GND end to the left, the even column becomes the upper row, the odd column (board edge) the lower row.

Positions along the edges, measured on page 1 (pitch 16.5 px at the render scale; right column top = 0):
- right: SCL, SDA, AREF, GND, D13 … D8 at 0 … 9; D7 … D0 at 10.6 … 17.6 (the 1.6-pitch step like the Uno); D14 … D21 at 19.6 … 26.6.
- left: NC … VIN at 3.6 … 10.6 (VIN level with D7); A0 … A7 at 12.6 … 19.6 (A7 level with D14); A8 … A15 at 21.6 … 28.6.
- the double row lies under both columns across the bottom edge.

Proposed lists in whole pitches (left/lower-right groups stay aligned; the SCL … D8 group sits 0.4 pitch higher than to scale, the same compromise as the Uno record):
- left (top to bottom): null, null, null, null, POWER.1 … POWER.8, null, ADCL.1 … ADCL.8, null, ADCH.1 … ADCH.8
- right (top to bottom): JP6.1 … JP6.10, null, PWML.8 … PWML.1, null, COMM.8 … COMM.1
- double row (left to right in this view), best as an `inner` block "XIO" placed at the bottom:
  - upper row: XIO.35, XIO.33, XIO.31, XIO.29, XIO.27, XIO.25, XIO.23, XIO.21, XIO.19, XIO.17, XIO.15, XIO.13, XIO.11, XIO.9, XIO.7, XIO.5, XIO.3, XIO.1
  - lower row (board edge): XIO.36, XIO.34, XIO.32, XIO.30, XIO.28, XIO.26, XIO.24, XIO.22, XIO.20, XIO.18, XIO.16, XIO.14, XIO.12, XIO.10, XIO.8, XIO.6, XIO.4, XIO.2
- ICSP (ATmega2560) as `inner`: upper row ICSP.5, ICSP.3, ICSP.1; lower row ICSP.6, ICSP.4, ICSP.2. Basis: pinout page 4 draws the block in the middle of the board with numbered circles 5 3 1 above and 6 4 2 below (zoomed render checked), pin 1 upper right, exactly as on the Uno.

## Header numbers
The schematic (MEGA2560_Rev3e) prints the names XIO (the 2x18 "18x2F-H8.5"), JP6 (10 pins), ADCH and ICSP. The power header, the A0–A7 header, the D0–D7 header and the D14–D21 header carry pin numbers but no printed name; I call them POWER, ADCL, PWML and COMM (see Choices). Pin numbers are the schematic's:
- POWER.1 NC … POWER.8 VIN, pin 1 at the top (USB end).
- ADCL.1 = A0 … ADCL.8 = A7; ADCH.1 = A8 … ADCH.8 = A15; pin 1 at the top.
- JP6.1 = SCL … JP6.10 = D8, pin 1 at the top (note: the Uno's IOH counts the other way).
- PWML.1 = D0 … PWML.8 = D7, pin 1 at the bottom.
- COMM.1 = D21 … COMM.8 = D14, pin 1 at the bottom.
- XIO.1/XIO.2 = +5V, XIO.3 = D22, XIO.4 = D23 … XIO.34 = D53, XIO.35/XIO.36 = GND (odd = even D numbers).

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| POWER.1 | NC | Not connected | Nicht belegt | nc |
| POWER.2 | IOREF | Tells shields the logic voltage; tied to 5 V | Zeigt Shields die Logikspannung; liegt an 5 V | power |
| POWER.3 | RESET | Reset; pull low to restart the ATmega2560 | Reset; low startet den ATmega2560 neu | signal |
| POWER.4 | 3V3 | 3.3 V output, 50 mA at most | 3,3-V-Ausgang, höchstens 50 mA | power |
| POWER.5 | 5V | 5 V rail | 5-V-Schiene | power |
| POWER.6 | GND | Ground | Masse | ground |
| POWER.7 | GND | Ground | Masse | ground |
| POWER.8 | VIN | Supply input, feeds the 5 V regulator (pinout: 6–20 V; datasheet: 7–12 V recommended) | Versorgungseingang, speist den 5-V-Regler (Pinout: 6–20 V; Datenblatt: 7–12 V empfohlen) | power |
| ADCL.1 | A0 | Analog input ADC0; digital D54 (PF0) | Analogeingang ADC0; digital D54 (PF0) | io |
| ADCL.2 | A1 | Analog input ADC1; digital D55 (PF1) | Analogeingang ADC1; digital D55 (PF1) | io |
| ADCL.3 | A2 | Analog input ADC2; digital D56 (PF2) | Analogeingang ADC2; digital D56 (PF2) | io |
| ADCL.4 | A3 | Analog input ADC3; digital D57 (PF3) | Analogeingang ADC3; digital D57 (PF3) | io |
| ADCL.5 | A4 | Analog input ADC4; digital D58; JTAG TCK (PF4) | Analogeingang ADC4; digital D58; JTAG TCK (PF4) | io |
| ADCL.6 | A5 | Analog input ADC5; digital D59; JTAG TMS (PF5) | Analogeingang ADC5; digital D59; JTAG TMS (PF5) | io |
| ADCL.7 | A6 | Analog input ADC6; digital D60; JTAG TDO (PF6) | Analogeingang ADC6; digital D60; JTAG TDO (PF6) | io |
| ADCL.8 | A7 | Analog input ADC7; digital D61; JTAG TDI (PF7) | Analogeingang ADC7; digital D61; JTAG TDI (PF7) | io |
| ADCH.1 | A8 | Analog input ADC8; digital D62; PCINT16 (PK0) | Analogeingang ADC8; digital D62; PCINT16 (PK0) | io |
| ADCH.2 | A9 | Analog input ADC9; digital D63; PCINT17 (PK1) | Analogeingang ADC9; digital D63; PCINT17 (PK1) | io |
| ADCH.3 | A10 | Analog input ADC10; digital D64; PCINT18 (PK2) | Analogeingang ADC10; digital D64; PCINT18 (PK2) | io |
| ADCH.4 | A11 | Analog input ADC11; digital D65; PCINT19 (PK3) | Analogeingang ADC11; digital D65; PCINT19 (PK3) | io |
| ADCH.5 | A12 | Analog input ADC12; digital D66; PCINT20 (PK4) | Analogeingang ADC12; digital D66; PCINT20 (PK4) | io |
| ADCH.6 | A13 | Analog input ADC13; digital D67; PCINT21 (PK5) | Analogeingang ADC13; digital D67; PCINT21 (PK5) | io |
| ADCH.7 | A14 | Analog input ADC14; digital D68; PCINT22 (PK6) | Analogeingang ADC14; digital D68; PCINT22 (PK6) | io |
| ADCH.8 | A15 | Analog input ADC15; digital D69; PCINT23 (PK7) | Analogeingang ADC15; digital D69; PCINT23 (PK7) | io |
| JP6.1 | SCL | I2C SCL; same line as D21 | I2C SCL; dieselbe Leitung wie D21 | io |
| JP6.2 | SDA | I2C SDA; same line as D20 | I2C SDA; dieselbe Leitung wie D20 | io |
| JP6.3 | AREF | Reference voltage of the ADC | Referenzspannung des ADC | signal |
| JP6.4 | GND | Ground | Masse | ground |
| JP6.5 | D13 | PWM (OC0A/OC1C); on-board LED (PB7) | PWM (OC0A/OC1C); LED auf der Platine (PB7) | io |
| JP6.6 | D12 | PWM (OC1B) (PB6) | PWM (OC1B) (PB6) | io |
| JP6.7 | D11 | PWM (OC1A) (PB5) | PWM (OC1A) (PB5) | io |
| JP6.8 | D10 | PWM (OC2A) (PB4) | PWM (OC2A) (PB4) | io |
| JP6.9 | D9 | PWM (OC2B) (PH6) | PWM (OC2B) (PH6) | io |
| JP6.10 | D8 | PWM (OC4C) (PH5) | PWM (OC4C) (PH5) | io |
| PWML.8 | D7 | PWM (OC4B) (PH4) | PWM (OC4B) (PH4) | io |
| PWML.7 | D6 | PWM (OC4A) (PH3) | PWM (OC4A) (PH3) | io |
| PWML.6 | D5 | PWM (OC3A); AIN1 (PE3) | PWM (OC3A); AIN1 (PE3) | io |
| PWML.5 | D4 | PWM (OC0B) (PG5) | PWM (OC0B) (PG5) | io |
| PWML.4 | D3 | PWM (OC3C); interrupt INT5 (PE5) | PWM (OC3C); Interrupt INT5 (PE5) | io |
| PWML.3 | D2 | PWM (OC3B); interrupt INT4 (PE4) | PWM (OC3B); Interrupt INT4 (PE4) | io |
| PWML.2 | D1 | UART0 TX (PE1); linked to the USB bridge | UART0 TX (PE1); mit der USB-Brücke verbunden | io |
| PWML.1 | D0 | UART0 RX (PE0); linked to the USB bridge | UART0 RX (PE0); mit der USB-Brücke verbunden | io |
| COMM.8 | D14 | UART3 TX (PJ1) | UART3 TX (PJ1) | io |
| COMM.7 | D15 | UART3 RX (PJ0) | UART3 RX (PJ0) | io |
| COMM.6 | D16 | UART2 TX (PH1) | UART2 TX (PH1) | io |
| COMM.5 | D17 | UART2 RX (PH0) | UART2 RX (PH0) | io |
| COMM.4 | D18 | UART1 TX; interrupt INT3 (PD3) | UART1 TX; Interrupt INT3 (PD3) | io |
| COMM.3 | D19 | UART1 RX; interrupt INT2 (PD2) | UART1 RX; Interrupt INT2 (PD2) | io |
| COMM.2 | D20 | I2C SDA; interrupt INT1 (PD1) | I2C SDA; Interrupt INT1 (PD1) | io |
| COMM.1 | D21 | I2C SCL; interrupt INT0 (PD0) | I2C SCL; Interrupt INT0 (PD0) | io |
| XIO.1 | 5V | 5 V rail | 5-V-Schiene | power |
| XIO.2 | 5V | 5 V rail | 5-V-Schiene | power |
| XIO.3 | D22 | Digital pin (PA0) | Digitaler Pin (PA0) | io |
| XIO.4 | D23 | Digital pin (PA1) | Digitaler Pin (PA1) | io |
| XIO.5 | D24 | Digital pin (PA2) | Digitaler Pin (PA2) | io |
| XIO.6 | D25 | Digital pin (PA3) | Digitaler Pin (PA3) | io |
| XIO.7 | D26 | Digital pin (PA4) | Digitaler Pin (PA4) | io |
| XIO.8 | D27 | Digital pin (PA5) | Digitaler Pin (PA5) | io |
| XIO.9 | D28 | Digital pin (PA6) | Digitaler Pin (PA6) | io |
| XIO.10 | D29 | Digital pin (PA7) | Digitaler Pin (PA7) | io |
| XIO.11 | D30 | Digital pin (PC7) | Digitaler Pin (PC7) | io |
| XIO.12 | D31 | Digital pin (PC6) | Digitaler Pin (PC6) | io |
| XIO.13 | D32 | Digital pin (PC5) | Digitaler Pin (PC5) | io |
| XIO.14 | D33 | Digital pin (PC4) | Digitaler Pin (PC4) | io |
| XIO.15 | D34 | Digital pin (PC3) | Digitaler Pin (PC3) | io |
| XIO.16 | D35 | Digital pin (PC2) | Digitaler Pin (PC2) | io |
| XIO.17 | D36 | Digital pin (PC1) | Digitaler Pin (PC1) | io |
| XIO.18 | D37 | Digital pin (PC0) | Digitaler Pin (PC0) | io |
| XIO.19 | D38 | Digital pin (PD7) | Digitaler Pin (PD7) | io |
| XIO.20 | D39 | Digital pin (PG2) | Digitaler Pin (PG2) | io |
| XIO.21 | D40 | Digital pin (PG1) | Digitaler Pin (PG1) | io |
| XIO.22 | D41 | Digital pin (PG0) | Digitaler Pin (PG0) | io |
| XIO.23 | D42 | Digital pin (PL7) | Digitaler Pin (PL7) | io |
| XIO.24 | D43 | Digital pin (PL6) | Digitaler Pin (PL6) | io |
| XIO.25 | D44 | PWM (PL5) | PWM (PL5) | io |
| XIO.26 | D45 | PWM (PL4) | PWM (PL4) | io |
| XIO.27 | D46 | PWM (PL3) | PWM (PL3) | io |
| XIO.28 | D47 | Digital pin (PL2) | Digitaler Pin (PL2) | io |
| XIO.29 | D48 | Digital pin (PL1) | Digitaler Pin (PL1) | io |
| XIO.30 | D49 | Digital pin (PL0) | Digitaler Pin (PL0) | io |
| XIO.31 | D50 | SPI CIPO, formerly MISO (PB3) | SPI CIPO, früher MISO (PB3) | io |
| XIO.32 | D51 | SPI COPI, formerly MOSI (PB2) | SPI COPI, früher MOSI (PB2) | io |
| XIO.33 | D52 | SPI SCK (PB1) | SPI SCK (PB1) | io |
| XIO.34 | D53 | SPI SS (PB0) | SPI SS (PB0) | io |
| XIO.35 | GND | Ground | Masse | ground |
| XIO.36 | GND | Ground | Masse | ground |
| ICSP.1 | CIPO | SPI CIPO, formerly MISO (D50, PB3) | SPI CIPO, früher MISO (D50, PB3) | data |
| ICSP.2 | 5V | 5 V rail | 5-V-Schiene | power |
| ICSP.3 | SCK | SPI clock (D52, PB1) | SPI-Takt (D52, PB1) | data |
| ICSP.4 | COPI | SPI COPI, formerly MOSI (D51, PB2) | SPI COPI, früher MOSI (D51, PB2) | data |
| ICSP.5 | RESET | Reset of the ATmega2560 | Reset des ATmega2560 | signal |
| ICSP.6 | GND | Ground | Masse | ground |

Item notes (en, as in the catalog): Logic level 5 V; each I/O pin carries 20 mA at most, the 3V3 pin 50 mA. VIN: Arduino's pinout says 6–20 V, the datasheet recommends 7–12 V; stay within 7–12 V. PWM is on D2 to D13 and D44 to D46 (15 pins) per the datasheet and the analogWrite reference; the pinout figure also marks D0 and D1 with a tilde, which the other documents do not support. I2C is on D20 (SDA) and D21 (SCL), also on the SDA and SCL pins beside AREF, not on A4 and A5 as on the Uno. SPI is on D50 to D53 and the ICSP block, not on D11 to D13. D0 and D1 also reach the USB bridge. The pinout also prints external-memory names (A8 to A15 on D37 to D30); those are address lines, not the analog inputs A8 to A15. ICSP1 of the ATmega16U2 near the USB socket and the 2×2 JP5 pads are not listed; the RESET-EN solder jumper disables auto-reset.

## Sources
1. Arduino, MEGA 2560 REV3 full pinout, SKU A000067, "Last update: 16/12/2020", https://docs.arduino.cc/resources/pinouts/A000067-full-pinout.pdf — pages 1 (board, all edge headers), 2 (alternate functions: timer OCnx, INTn, PCINTn, JTAG, UARTs), 3 (D22–D53 double row, landscape, TOP/USB thumbnail), 4 (ICSP, ICSP1, JP5), each rendered and zoomed; accessed 2026-11-24
2. Arduino, schematic MEGA2560_Rev3e (Arturo Guadalupi, 23/01/19, sheet 1/2), https://docs.arduino.cc/resources/schematics/A000067-schematics.pdf — page 1: XIO (18x2F-H8.5), JP6, ADCH, the unnamed POWER/ADCL/PWML/COMM 8-pin headers, ICSP; rendered and zoomed; accessed 2026-11-24
3. Arduino, Mega 2560 Rev3 user manual (datasheet), SKU A000067, "Modified: 19/11/2026", https://docs.arduino.cc/resources/datasheets/A000067-datasheet.pdf — Features ("54 Digital, 16 Analog, 15 PWM Output"), 2.1 ratings, 5.1–5.6 connector tables; read as text; accessed 2026-11-24
4. Arduino, analogWrite() reference source (arduino/reference-en, Language/Functions/Analog IO/analogWrite.adoc), https://raw.githubusercontent.com/arduino/reference-en/master/Language/Functions/Analog%20IO/analogWrite.adoc — PWM table row "Mega | 2 - 13, 44 - 46 | 490 Hz (pins 4 and 13: 980 Hz)"; accessed 2026-11-24
5. Arduino, Mega 2560 product page, https://docs.arduino.cc/hardware/mega-2560/ — "54 digital input/output pins (of which 15 can be used as PWM outputs)", "16 analog inputs"; accessed 2026-11-24

## Cross-check
Schematic (source 2) against full pinout (source 1), pin by pin:
- POWER 1–8: NC (pin 1 unconnected), pin 2 tied to +5V (= IOREF), RESET, +3V3, +5V, GND, GND, VIN — same order as page 1. Match.
- ADCL 1–8 = ADC0–ADC7, ADCH 1–8 = ADC8–ADC15 — page 1/2 A0–A15 on PF0–PF7 and PK0–PK7. Match.
- JP6 1–10 = SCL, SDA, AREF, GND, 13, 12, 11, 10, 9 (PH6), 8 (PH5) — page 1 D21/SCL, D20/SDA, AREF, GND, ~D13 PB7 … ~D8 PH5. Match.
- PWML 1–8 = 0 PE0, 1 PE1, 2 PE4, 3 PE5, 4 PG5, 5 PE3, 6 PH3, 7 PH4 — page 1 identical ports. Match.
- COMM 1–8 = 21, 20, 19 RXD1, 18 TXD1, 17 RXD2, 16 TXD2, 15 RXD3, 14 TXD3 — page 1 D21/SCL PD0 … D14/TX3 PJ1. Match.
- XIO 1–36: +5V, +5V, then PA0 22, PA1 23 … PA7 29, PC7 30 … PC0 37, PD7 38, PG2 39, PG1 40, PG0 41, PL7 42 … PL0 49, PB3 50 (MISO), PB2 51 (MOSI), PB1 52 (SCK), PB0 53 (SS), GND, GND — page 3 lists exactly these ports beside D22–D53 and +5V/GND at the ends. Match on all 36.
- ICSP 1–6 = MISO PB3, +5V, SCK PB1, MOSI PB2, RESET, GND — page 4 table CIPO PB3, +5V, SCK PB1, COPI PB2, RESET, GND. Match.
Datasheet (source 3) section 5: Analog table 1–24 = NC, IOREF, Reset, +3V3, +5V, GND, GND, VIN, A0–A15 (same order); Digital table 1–26 = D21/SCL, D20/SDA, AREF, GND, D13 … D0, D14 … D21; tables 5.5/5.6 LHS = +5V, D22, D24 … D52, GND and RHS = +5V, D23 … D53, GND. Same order everywhere.

Differences found:
- PWM on D0/D1: pinout pages 1–2 print "~D1/TX0" and "~D0/RX0", and the schematic writes "pwm" beside PE0/PE1. But page 2's timer column gives no OCnx for PE0/PE1 (only TXD0/RXD0), the datasheet and product page count 15 PWM outputs, and Arduino's analogWrite reference lists "2 - 13, 44 - 46" (12 + 3 = 15). I treat D0/D1 as non-PWM and D2–D13, D44–D46 as PWM.
- D44–D46: page 3 marks them with "~"; no timer names printed for them on page 3. PWM from the tilde and source 4. Their timer channels (OC5C/OC5B/OC5A) are NOT written into `function` because I did not read them in an Arduino document.
- Datasheet typos: table 5.2 row 18 "D0/Tx1" (pinout: D0/RX0); table 5.1 "Type" and "Description" columns are shifted in the text layer.
- VIN: pinout legend "VIN 6-20 V input to the board"; datasheet table 2.1 gives 7 / 7.0 / 12 V (min/typ/max) as recommended. Both in `function`.
- The double row's on-board orientation is drawn only on page 3 (landscape); page 1 shows the header without labels. The portrait assignment is a rotation of page 3 (see View) and is consistent with the datasheet's LHS/RHS split.

## Choices
- Catalog item: `board` shape, `connector: top`, top view. Left list: four gaps, POWER.1 to POWER.8, a gap, ADCL.1 to ADCL.8, a gap, ADCH.1 to ADCH.8; right list: JP6.1 to JP6.10, a gap, PWML.8 to PWML.1, a gap, COMM.8 to COMM.1 (the rows of the record's View). The 2×18 double row is one `inner` group "XIO" of two rows of 18 (upper XIO.35 to XIO.1 odd, lower XIO.36 to XIO.2 even), followed by the ICSP group; the drawing centres inner groups in the board, so the double row is not drawn at the bottom edge as on the real board, and the item doc says so. A side list was not used because a side list has one pin per row of the board edge and would make the board 36 rows tall.
- Header ids: JP6, ADCH, XIO and ICSP are the schematic's own names; POWER, ADCL, PWML and COMM are ours (the schematic prints only pin numbers on these four 8-pin headers). The item doc says which is which.
- Where the maker's documents disagree the datasheet and the reference text win over the figure: PWM on D2 to D13 and D44 to D46 (15 pins, product page and datasheet "15 PWM Output", analogWrite reference), not on D0 and D1 as the pinout figure and the schematic mark. Both D0 (PWML.1) and D1 (PWML.2) carry a pin note, and the item note says so.
- VIN: the function text names both values (pinout 6 to 20 V, datasheet 7 to 12 V); the item note uses the narrower, conservative 7 to 12 V.
- Header ids POWER, ADCL, PWML, COMM are mine: the schematic prints only pin numbers for these four 8-pin headers. JP6, ADCH, XIO, ICSP are the schematic's names.
- Names: `3V3`, `5V` for `+3V3`, `+5V`; D-numbers without "~" or "/TXn"; UART, PWM and I2C roles in `function`.
- JP6.1/JP6.2 named `SCL`/`SDA` (type io), same nets as D21/D20 per schematic net labels SCL/SDA.
- AREF `signal`, IOREF `power`, NC `nc`, ICSP SPI lines `data`, RESET `signal`.
- The double row as one `inner` block (two rows of 18) at the bottom rather than splitting it into `bottom` + `inner`.
- ICSP1 (16U2) and JP5 only in the notes, as on the Uno.

## Spot-check
1. JP6.1 = SCL, top of the right column — schematic JP6 pin 1 "SCL"; pinout p.1 first right label "D21/SCL" region, p.2 "SCL" beside the top pin.
2. JP6.5 = D13 (PB7, LED_BUILTIN) — schematic JP6 pin 5 "13"; pinout p.1 "~D13 PB7", p.2 "PB7 LED_BUILTIN".
3. PWML.3 = D2 (PE4, INT4) — schematic PWML pin 3 "PE4 pwm 2"; pinout p.2 "~D2 PE4 OC3B INT[4]".
4. COMM.1 = D21 (PD0, SCL, INT0) — schematic comm header pin 1 "21"; pinout p.2 bottom "D21/SCL PD0 SCL INT[0]".
5. XIO.3 = D22 (PA0), XIO.4 = D23 (PA1) — schematic XIO pins 3/4 "PA0 22" / "PA1 23"; pinout p.3 first row "PA0 D22" / "D23 PA1"; datasheet 5.5/5.6 row 2.
6. XIO.34 = D53 (PB0, SS), XIO.35/36 = GND — schematic XIO "PB0 (SS) 53" pin 34, GND on 35/36; pinout p.3 "D53 PB0 SS", "GND GND".
7. POWER.8 = VIN, POWER.1 = NC — schematic power header pins 8/1; pinout p.1 left column; datasheet 5.1 rows 1 and 8.
8. ICSP.1 = CIPO (PB3), pin 1 upper right — schematic ICSP pin 1 "PB3 (MISO)"; pinout p.4 circles "5 3 1 / 6 4 2", table "1 CIPO PB3".
9. ADCH.8 = A15 (PK7, PCINT23) — schematic ADCH pin 8 "ADC15"; pinout p.2 "PCINT[23] ADC[15] PK7 A15 D69".

UNVERIFIED: the timer channels of D44–D46 (not read in an Arduino document; only "PWM" is stated). The printed names of the four unnamed 8-pin headers (ids chosen by me). Everything else read.
