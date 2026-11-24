# arduino_micro — Arduino Micro headers / Arduino Micro Stiftleisten

Kind: authoritative maker document (Arduino full pinout + Arduino schematic), board Arduino Micro (SKU A000053 with headers / A000093 without)
Pins: 40; types: power 4, ground 3, signal 4, data 3, io 24, nc 2

## View
Board from the component side, portrait, Micro-USB socket at the top (`connector: top`). Basis: Arduino's MICRO full pinout page 1 (and page 2), rendered and zoomed: USB at the top edge, the ATmega32U4 in the middle, the reset button and the 2x3 ICSP block at the bottom, 17 pins down each long edge. Page 3 draws the same top view with the ICSP numbers and, beside it, a view captioned "BOTTOM".

Both columns are straight 17-pin rows at the same height (no offsets), so:
- left (top to bottom): J6.17, J6.16, J6.15, J6.14, J6.13, J6.12, J6.11, J6.10, J6.9, J6.8, J6.7, J6.6, J6.5, J6.4, J6.3, J6.2, J6.1
- right (top to bottom): J5.17, J5.16, J5.15, J5.14, J5.13, J5.12, J5.11, J5.10, J5.9, J5.8, J5.7, J5.6, J5.5, J5.4, J5.3, J5.2, J5.1
- inner ICSP (lower left, between the left column and the reset button), 3 rows of 2: [ICSP.1, ICSP.2], [ICSP.3, ICSP.4], [ICSP.5, ICSP.6] — pinout page 3 draws numbered circles 1 2 / 3 4 / 5 6, odd numbers on the left, pin 1 at the top.

The pinout draws the pad at the bottom right (D16/COPI) square: that is J5.1, pin 1 of the schematic's J5.

## Header numbers
Schematic header names: J5 (HEAD17-NOSS) and J6 (HEAD17-NOSS-1), ICSP is J4 (3x2M). Pin 1 of J5 and J6 is at the bottom (the end away from USB).
- J5.1 … J5.17 = MOSI, RXLED/SS, D1/TX, D0/RX, RESET, GND, D2/SDA, D3/SCL, D4, D5, D6, D7, IO8, IO9, IO10, IO11, IO12.
- J6.1 … J6.17 = SCK, MISO, VIN, GND, RESET, +5V, (no net), (no net), A5, A4, A3, A2, A1, A0, AREF, 3V3, IO13.

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| J6.17 | D13 | PWM; on-board LED (PC7) | PWM; LED auf der Platine (PC7) | io |
| J6.16 | 3V3 | 3.3 V output, 50 mA at most | 3,3-V-Ausgang, höchstens 50 mA | power |
| J6.15 | AREF | Reference voltage of the ADC | Referenzspannung des ADC | signal |
| J6.14 | A0 | Analog input ADC7; digital D18; JTAG TDI (PF7) | Analogeingang ADC7; digital D18; JTAG TDI (PF7) | io |
| J6.13 | A1 | Analog input ADC6; digital D19; JTAG TDO (PF6) | Analogeingang ADC6; digital D19; JTAG TDO (PF6) | io |
| J6.12 | A2 | Analog input ADC5; digital D20; JTAG TMS (PF5) | Analogeingang ADC5; digital D20; JTAG TMS (PF5) | io |
| J6.11 | A3 | Analog input ADC4; digital D21; JTAG TCK (PF4) | Analogeingang ADC4; digital D21; JTAG TCK (PF4) | io |
| J6.10 | A4 | Analog input ADC1; digital D22 (PF1) | Analogeingang ADC1; digital D22 (PF1) | io |
| J6.9 | A5 | Analog input ADC0; digital D23 (PF0) | Analogeingang ADC0; digital D23 (PF0) | io |
| J6.8 | NC | Not connected | Nicht belegt | nc |
| J6.7 | NC | Not connected | Nicht belegt | nc |
| J6.6 | 5V | 5 V rail | 5-V-Schiene | power |
| J6.5 | RESET | Reset; pull low to restart the ATmega32U4 | Reset; low startet den ATmega32U4 neu | signal |
| J6.4 | GND | Ground | Masse | ground |
| J6.3 | VIN | Supply input, 6–9 V per Arduino's pinout, feeds the 5 V regulator | Versorgungseingang, 6–9 V laut Arduino-Pinout, speist den 5-V-Regler | power |
| J6.2 | D14 | SPI CIPO, formerly MISO (PB3); same line as ICSP.1 | SPI CIPO, früher MISO (PB3); dieselbe Leitung wie ICSP.1 | io |
| J6.1 | D15 | SPI SCK (PB1); same line as ICSP.3 | SPI SCK (PB1); dieselbe Leitung wie ICSP.3 | io |
| J5.17 | D12 | Digital pin; analog input A11 (ADC9) (PD6) | Digitaler Pin; Analogeingang A11 (ADC9) (PD6) | io |
| J5.16 | D11 | PWM (PB7) | PWM (PB7) | io |
| J5.15 | D10 | PWM; analog input A10 (ADC13) (PB6) | PWM; Analogeingang A10 (ADC13) (PB6) | io |
| J5.14 | D9 | PWM; analog input A9 (ADC12) (PB5) | PWM; Analogeingang A9 (ADC12) (PB5) | io |
| J5.13 | D8 | Digital pin; analog input A8 (ADC11) (PB4) | Digitaler Pin; Analogeingang A8 (ADC11) (PB4) | io |
| J5.12 | D7 | Digital pin; interrupt INT6 (PE6) | Digitaler Pin; Interrupt INT6 (PE6) | io |
| J5.11 | D6 | PWM; analog input A7 (ADC10) (PD7) | PWM; Analogeingang A7 (ADC10) (PD7) | io |
| J5.10 | D5 | PWM (PC6) | PWM (PC6) | io |
| J5.9 | D4 | Digital pin; analog input A6 (ADC8) (PD4) | Digitaler Pin; Analogeingang A6 (ADC8) (PD4) | io |
| J5.8 | D3 | PWM; I2C SCL; interrupt INT0 (PD0) | PWM; I2C SCL; Interrupt INT0 (PD0) | io |
| J5.7 | D2 | I2C SDA; interrupt INT1 (PD1) | I2C SDA; Interrupt INT1 (PD1) | io |
| J5.6 | GND | Ground | Masse | ground |
| J5.5 | RESET | Reset; same line as J6.5 | Reset; dieselbe Leitung wie J6.5 | signal |
| J5.4 | D0 | UART RX of Serial1; interrupt INT2 (PD2) | UART RX von Serial1; Interrupt INT2 (PD2) | io |
| J5.3 | D1 | UART TX of Serial1; interrupt INT3 (PD3) | UART TX von Serial1; Interrupt INT3 (PD3) | io |
| J5.2 | D17 | SPI SS (PB0); also drives the RX LED | SPI SS (PB0); steuert auch die RX-LED | io |
| J5.1 | D16 | SPI COPI, formerly MOSI (PB2); same line as ICSP.4 | SPI COPI, früher MOSI (PB2); dieselbe Leitung wie ICSP.4 | io |
| ICSP.1 | CIPO | SPI CIPO, formerly MISO (D14, PB3) | SPI CIPO, früher MISO (D14, PB3) | data |
| ICSP.2 | 5V | 5 V rail | 5-V-Schiene | power |
| ICSP.3 | SCK | SPI clock (D15, PB1) | SPI-Takt (D15, PB1) | data |
| ICSP.4 | COPI | SPI COPI, formerly MOSI (D16, PB2) | SPI COPI, früher MOSI (D16, PB2) | data |
| ICSP.5 | RESET | Reset of the ATmega32U4 | Reset des ATmega32U4 | signal |
| ICSP.6 | GND | Ground | Masse | ground |

Item notes (en, as in the catalog): Logic level 5 V. Current per I/O pin: the Micro pinout's legend says 40 mA at most, the Leonardo pinout 20 mA and the tech specs 10 mA; stay at 10 mA. VIN: the pinout says 6–9 V, the tech specs 7–12 V nominal; stay within 7–9 V, where both agree. The USB serial port is native (Serial); D0 and D1 are the separate hardware UART Serial1. I2C is on D2 (SDA) and D3 (SCL); SPI is on D14 to D17 and the ICSP block. PWM is on D3, D5, D6, D9, D10, D11 and D13. A6 to A11 are extra analog inputs on D4, D6, D8, D9, D10 and D12. D17 (SS) shares its pin with the RX LED.

## Sources
1. Arduino, MICRO full pinout, SKU A000053, "Last update: 10/11/2020", https://docs.arduino.cc/resources/pinouts/A000053-full-pinout.pdf — page 1 (top view, all 34 header pins), page 2 (alternate functions), page 3 (ICSP numbering + BOTTOM view); rendered and zoomed; accessed 2026-11-24
2. Arduino, Arduino MICRO schematic, https://docs.arduino.cc/resources/schematics/A000053-schematics.pdf — page 1: J5, J6 (HEAD17-NOSS), J4 ICSP (3x2M), U1 ATmega32U4 net labels; rendered and zoomed (no date legible in the title area); accessed 2026-11-24
3. Arduino docs-content, Micro tech-specs.yml and features.md, https://github.com/arduino/docs-content/tree/main/content/hardware/hero/boards/micro — "Digital I/O Pins: 20", "PWM pins: 7", "Analog input pins: 12", "I/O Voltage: 5V", "Input voltage (nominal): 7-12V", "DC Current per I/O Pin: 10 mA"; accessed 2026-11-24
4. Arduino docs-content, "Getting Started with the Arduino Leonardo, Leonardo ETH and Micro", https://docs.arduino.cc/retired/getting-started-guides/ArduinoLeonardoMicro/ (read via GitHub source) — Serial vs Serial1 on pins 0/1; accessed 2026-11-24
5. Arduino, ArduinoCore-avr variants/leonardo/pins_arduino.h (the Micro uses the same variant), https://raw.githubusercontent.com/arduino/ArduinoCore-avr/master/variants/leonardo/pins_arduino.h — SS 17, MOSI 16, MISO 14, SCK 15, SDA 2, SCL 3, LED_BUILTIN_RX 17, PWM 3/5/6/9/10/11/13; accessed 2026-11-24
6. Arduino, analogWrite() reference source, https://raw.githubusercontent.com/arduino/reference-en/master/Language/Functions/Analog%20IO/analogWrite.adoc — "Leonardo, Micro, Yún | 3, 5, 6, 9, 10, 11, 13"; accessed 2026-11-24

## Cross-check
Schematic (source 2) against full pinout (source 1), pin by pin, reading J5/J6 from pin 17 (top) to pin 1 (bottom):
- J6: IO13, 3V3, AREF, A0, A1, A2, A3, A4, A5, —, —, +5V, RESET, GND, VIN, MISO, SCK — pinout left column D13~ PC7, +3V3, AREF, A0 … A5, NC, NC, +5V, RESET, GND, VIN, CIPO/D14 PB3, SCK/D15 PB1. Match on all 17.
- J5: IO12, IO11, IO10, IO9, IO8, D7, D6, D5, D4, D3/SCL, D2/SDA, GND, RESET, D0/RX, D1/TX, RXLED/SS, MOSI — pinout right column D12 A11, ~D11, ~D10 A10, ~D9 A9, D8 A8, D7, ~D6 A7, ~D5, D4 A6, ~D3/SCL, D2/SDA, GND, RESET, D0/RX, D1/TX, D17/SS PB0, D16/COPI PB2. Match on all 17.
- ICSP J4 1–6 = MISO, +5V, SCK, MOSI, RESET, GND — pinout p.3 1 CIPO PB3, 2 +5V, 3 SCK PB1, 4 COPI PB2, 5 RESET, 6 GND. Match.

Differences found:
- PWM marks: the schematic puts "*" on D4 (J5.9) and IO12 (J5.17) as well; the pinout has no tilde there, page 2 shows no timer output for PD4 (only ICP1) and only the inverted T1/OC4D for PD6; Arduino's reference and pins_arduino.h list PWM on 3, 5, 6, 9, 10, 11, 13. I follow the pinout and the reference: D4 and D12 are not PWM.
- Current per I/O: 40 mA (Micro pinout legend) vs 20 mA (Leonardo pinout) vs 10 mA (tech specs).
- VIN: 6–9 V (pinout legend) vs 7–12 V nominal (tech specs). Function uses the pinout; the note gives both.
- J6.7 and J6.8 have no net in the schematic; the pinout calls them NC. Same meaning.

## Choices
- Catalog item: `board` shape, `connector: top`, top view: left J6.17 down to J6.1, right J5.17 down to J5.1 (both straight 17-pin rows, no gaps), and the `inner` group ICSP in three rows of two (ICSP.1 ICSP.2, ICSP.3 ICSP.4, ICSP.5 ICSP.6; pin 1 upper left as on the pinout's page 3). The schematic's J4 is named ICSP, as on the other Arduino items, because J4 is not printed on the board.
- Per-pin current and VIN: the item note uses the conservative values (10 mA from the tech specs; 7 to 9 V, the overlap of the pinout's 6 to 9 V and the tech specs' 7 to 12 V) and names the others; the function of VIN gives the pinout's 6 to 9 V.
- D4 (J5.9) and D12 (J5.17) carry a pin note: the schematic marks them PWM, the pinout and the analogWrite reference do not (PWM on 3, 5, 6, 9, 10, 11, 13).
- Header ids J5, J6, ICSP (the schematic's J4 renamed ICSP as in the other Arduino records, because J4 is not printed on the board; say so in the doc text or keep `J4.n` — maker's call).
- Names: D14/D15/D16/D17 (pins_arduino.h numbers) for the pinout's CIPO/D14, SCK/D15, D16/COPI, D17/SS, SPI role in `function`, type io (they are usable GPIO). The ICSP lines stay type data as on the Uno.
- `3V3`, `5V` for `+3V3`, `+5V`; RESET appears twice (J6.5, J5.5), both type signal.

## Spot-check
1. J6.17 = D13 (PC7, LED) top left — schematic J6 pin 17 "IO13*"; pinout p.1 top left "PC7 D13~".
2. J6.3 = VIN — schematic J6 pin 3 to VIN; pinout p.1 "VIN" third from the bottom on the left.
3. J6.2 = D14 / CIPO (PB3) — schematic J6 pin 2 "MISO"; pinout p.1 "CIPO PB3 CIPO/D14".
4. J5.1 = D16 / COPI (PB2), square pad bottom right — schematic J5 pin 1 "MOSI"; pinout p.1 "D16/COPI PB2 COPI".
5. J5.2 = D17 / SS (PB0) = RX LED — schematic J5 pin 2 "RXLED/SS"; pinout p.1 "D17/SS PB0 SS" and "RX LED PB0".
6. J5.8 = D3 / SCL (PD0) — schematic J5 pin 8 "D3/SCL"; pinout p.2 "~D3/SCL PD0 SCL OC0B INT[0]".
7. J5.4 = D0/RX (PD2), above J5.3 = D1/TX — schematic J5 pins 4/3; pinout p.1 D0/RX above D1/TX.
8. ICSP.1 upper left — pinout p.3 circles 1 2 / 3 4 / 5 6.

UNVERIFIED: none of the pins. The schematic's revision/date (not legible on the page) is not stated.
