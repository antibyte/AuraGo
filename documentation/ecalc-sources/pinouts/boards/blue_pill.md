# blue_pill — STM32F103C8 "Blue Pill" headers / STM32F103C8 „Blue Pill“ Stiftleisten

Kind: no official board maker (clone board). Pin functions and 5 V tolerance from STMicroelectronics' STM32F103x8/xB datasheet; header layout from two independent sources (stm32-base.org board page with its top-view photo, and the Zephyr Project's Blue Pill pinout figure); a schematic credited to VCC-GND Studio (www.vcc-gnd.com, the commonly named originator) as a third, consistent source. No single maker document exists for the board itself.
Pins: 44; types: power 5; ground 4; signal 1; data 2; io 32; nc 0
Status: complete via the clone route (layout from two independent sources that agree on all 44 positions)

## View
Board from the component side (MCU, crystal, reset button, BOOT jumpers), portrait, Micro-USB socket at the top (`connector: top`), the 4-pin SWD header at the bottom edge. Basis:
- Zephyr's figure `stm32_min_dev_pinout_blue.jpg` ("Pinout for STM32 Minimum Development Blue Pill Board", "The Generic STM32F103 Pinout Diagram"), enlarged and read: USB up, component side; left column top to bottom PB12, PB13, PB14, PB15, PA8, PA9, PA10, PA11, PA12, PA15, PB3, PB4, PB5, PB6, PB7, PB8, PB9, 5V, GND, 3.3V; right column GND, GND, 3.3V, NRST, PB11, PB10, PB1, PB0, PA7 … PA0, PC15, PC14, PC13, VBAT; SWD header at the bottom, left to right 3.3V, SWDIO (PA13), SWCLK (PA14), GND. LQFP48 pin numbers are printed beside each pin and match ST's table (e.g. PB12 = 25, PA15 = 38, PB8 = 45, PA0 = 10, PC13 = 2).
- stm32-base.org photo "Blue Pill: Top view" (looked at, SWD corner zoomed): component side with USB at the left; silk of the upper row from the USB end G, G, 3.3, R, B11, B10, B1, B0, A7 … A0, C15, C14, C13, VB; lower row B12, B13, B14, B15, A8, A9, A10, A11, A12, A15, B3, B4, B5, B6, B7, B8, B9, 5V, G, 3.3; SWD header at the right end with GND (top) … 3V3 (bottom); square pads at VB, 3.3 (lower row) and the SWD 3V3. Turned 90° clockwise (USB up) this is exactly the Zephyr figure.

Lists (USB at the top):
- left (top to bottom): H2.20, H2.19, H2.18, H2.17, H2.16, H2.15, H2.14, H2.13, H2.12, H2.11, H2.10, H2.9, H2.8, H2.7, H2.6, H2.5, H2.4, H2.3, H2.2, H2.1
- right (top to bottom): H1.20, H1.19, H1.18, H1.17, H1.16, H1.15, H1.14, H1.13, H1.12, H1.11, H1.10, H1.9, H1.8, H1.7, H1.6, H1.5, H1.4, H1.3, H1.2, H1.1
- bottom (left to right): SWD.1, SWD.2, SWD.3, SWD.4

## Header numbers
stm32-base.org numbers the headers "Header 1" (H1, pin 1 = VB), "Header 2" (H2, pin 1 = 3.3) and the "SWD debug header" (1 3V3, 2 DIO, 3 CLK, 4 GND); pin 1 of each is at the SWD end (square pads in the photo). The VCC-GND schematic calls the same headers P3, P4 and P2 with the same pin numbers (P3.1 VBAT … P3.20 GND; P4.1 VCC3V3, P4.2 GND, P4.3 5V, P4.4 PB9 … P4.20 PB12; P2.1 VCC3V3, P2.2 SWDIO, P2.3 SWDCLK, P2.4 GND).

## Pins
FT = "five-volt tolerant" in ST's table 5. Pins without FT take at most 4.0 V (absolute maximum) — treat them as 3.3 V only.

| position | label | function en | function de | type |
|---|---|---|---|---|
| H1.1 | VBAT | Backup supply for RTC/backup registers (VB) | Pufferversorgung für RTC/Backup-Register (VB) | power |
| H1.2 | PC13 | GPIO; on-board LED, lights when low; 3 mA max, 2 MHz max; 3.3 V only | GPIO; LED auf der Platine, leuchtet bei Low; max. 3 mA, max. 2 MHz; nur 3,3 V | io |
| H1.3 | PC14 | GPIO; OSC32_IN (32.768 kHz crystal on board); 3 mA max; 3.3 V only | GPIO; OSC32_IN (32,768-kHz-Quarz auf der Platine); max. 3 mA; nur 3,3 V | io |
| H1.4 | PC15 | GPIO; OSC32_OUT (32.768 kHz crystal on board); 3 mA max; 3.3 V only | GPIO; OSC32_OUT (32,768-kHz-Quarz auf der Platine); max. 3 mA; nur 3,3 V | io |
| H1.5 | PA0 | ADC12_IN0; WKUP; USART2_CTS; TIM2_CH1_ETR; 3.3 V only | ADC12_IN0; WKUP; USART2_CTS; TIM2_CH1_ETR; nur 3,3 V | io |
| H1.6 | PA1 | ADC12_IN1; USART2_RTS; TIM2_CH2; 3.3 V only | ADC12_IN1; USART2_RTS; TIM2_CH2; nur 3,3 V | io |
| H1.7 | PA2 | ADC12_IN2; USART2_TX; TIM2_CH3; 3.3 V only | ADC12_IN2; USART2_TX; TIM2_CH3; nur 3,3 V | io |
| H1.8 | PA3 | ADC12_IN3; USART2_RX; TIM2_CH4; 3.3 V only | ADC12_IN3; USART2_RX; TIM2_CH4; nur 3,3 V | io |
| H1.9 | PA4 | ADC12_IN4; SPI1_NSS; USART2_CK; 3.3 V only | ADC12_IN4; SPI1_NSS; USART2_CK; nur 3,3 V | io |
| H1.10 | PA5 | ADC12_IN5; SPI1_SCK; 3.3 V only | ADC12_IN5; SPI1_SCK; nur 3,3 V | io |
| H1.11 | PA6 | ADC12_IN6; SPI1_MISO; TIM3_CH1; 3.3 V only | ADC12_IN6; SPI1_MISO; TIM3_CH1; nur 3,3 V | io |
| H1.12 | PA7 | ADC12_IN7; SPI1_MOSI; TIM3_CH2; 3.3 V only | ADC12_IN7; SPI1_MOSI; TIM3_CH2; nur 3,3 V | io |
| H1.13 | PB0 | ADC12_IN8; TIM3_CH3; 3.3 V only | ADC12_IN8; TIM3_CH3; nur 3,3 V | io |
| H1.14 | PB1 | ADC12_IN9; TIM3_CH4; 3.3 V only | ADC12_IN9; TIM3_CH4; nur 3,3 V | io |
| H1.15 | PB10 | I2C2_SCL; USART3_TX; 5 V tolerant | I2C2_SCL; USART3_TX; 5-V-tolerant | io |
| H1.16 | PB11 | I2C2_SDA; USART3_RX; 5 V tolerant | I2C2_SDA; USART3_RX; 5-V-tolerant | io |
| H1.17 | NRST | Reset, active low; reset button (silk R) | Reset, low-aktiv; Reset-Taster (Aufdruck R) | signal |
| H1.18 | 3V3 | 3.3 V rail (regulator output) | 3,3-V-Schiene (Reglerausgang) | power |
| H1.19 | GND | Ground | Masse | ground |
| H1.20 | GND | Ground | Masse | ground |
| H2.1 | 3V3 | 3.3 V rail (regulator output) | 3,3-V-Schiene (Reglerausgang) | power |
| H2.2 | GND | Ground | Masse | ground |
| H2.3 | 5V | 5 V rail, tied to USB VBUS, no protection | 5-V-Schiene, direkt an USB-VBUS, ohne Schutz | power |
| H2.4 | PB9 | TIM4_CH4; remap I2C1_SDA, CANTX; 5 V tolerant | TIM4_CH4; Remap I2C1_SDA, CANTX; 5-V-tolerant | io |
| H2.5 | PB8 | TIM4_CH3; remap I2C1_SCL, CANRX; 5 V tolerant | TIM4_CH3; Remap I2C1_SCL, CANRX; 5-V-tolerant | io |
| H2.6 | PB7 | I2C1_SDA; TIM4_CH2; remap USART1_RX; 5 V tolerant | I2C1_SDA; TIM4_CH2; Remap USART1_RX; 5-V-tolerant | io |
| H2.7 | PB6 | I2C1_SCL; TIM4_CH1; remap USART1_TX; 5 V tolerant | I2C1_SCL; TIM4_CH1; Remap USART1_TX; 5-V-tolerant | io |
| H2.8 | PB5 | I2C1_SMBA; remap TIM3_CH2, SPI1_MOSI; 3.3 V only (not FT) | I2C1_SMBA; Remap TIM3_CH2, SPI1_MOSI; nur 3,3 V (nicht FT) | io |
| H2.9 | PB4 | JTAG NJTRST after reset; remap TIM3_CH1, SPI1_MISO; 5 V tolerant | Nach Reset JTAG NJTRST; Remap TIM3_CH1, SPI1_MISO; 5-V-tolerant | io |
| H2.10 | PB3 | JTAG JTDO/TRACESWO after reset; remap TIM2_CH2, SPI1_SCK; 5 V tolerant | Nach Reset JTAG JTDO/TRACESWO; Remap TIM2_CH2, SPI1_SCK; 5-V-tolerant | io |
| H2.11 | PA15 | JTAG JTDI after reset; remap TIM2_CH1_ETR, SPI1_NSS; 5 V tolerant | Nach Reset JTAG JTDI; Remap TIM2_CH1_ETR, SPI1_NSS; 5-V-tolerant | io |
| H2.12 | PA12 | USB D+ (wired to the USB socket, pull-up on board); CANTX; USART1_RTS; 5 V tolerant | USB D+ (an der USB-Buchse, Pull-up auf der Platine); CANTX; USART1_RTS; 5-V-tolerant | io |
| H2.13 | PA11 | USB D- (wired to the USB socket); CANRX; USART1_CTS; 5 V tolerant | USB D- (an der USB-Buchse); CANRX; USART1_CTS; 5-V-tolerant | io |
| H2.14 | PA10 | USART1_RX; TIM1_CH3; 5 V tolerant | USART1_RX; TIM1_CH3; 5-V-tolerant | io |
| H2.15 | PA9 | USART1_TX; TIM1_CH2; 5 V tolerant | USART1_TX; TIM1_CH2; 5-V-tolerant | io |
| H2.16 | PA8 | TIM1_CH1; MCO; USART1_CK; 5 V tolerant | TIM1_CH1; MCO; USART1_CK; 5-V-tolerant | io |
| H2.17 | PB15 | SPI2_MOSI; TIM1_CH3N; 5 V tolerant | SPI2_MOSI; TIM1_CH3N; 5-V-tolerant | io |
| H2.18 | PB14 | SPI2_MISO; USART3_RTS; TIM1_CH2N; 5 V tolerant | SPI2_MISO; USART3_RTS; TIM1_CH2N; 5-V-tolerant | io |
| H2.19 | PB13 | SPI2_SCK; USART3_CTS; TIM1_CH1N; 5 V tolerant | SPI2_SCK; USART3_CTS; TIM1_CH1N; 5-V-tolerant | io |
| H2.20 | PB12 | SPI2_NSS; I2C2_SMBA; USART3_CK; TIM1_BKIN; 5 V tolerant | SPI2_NSS; I2C2_SMBA; USART3_CK; TIM1_BKIN; 5-V-tolerant | io |
| SWD.1 | 3V3 | 3.3 V rail | 3,3-V-Schiene | power |
| SWD.2 | SWDIO | SWD data, PA13 (JTMS); 5 V tolerant (silk DIO) | SWD-Daten, PA13 (JTMS); 5-V-tolerant (Aufdruck DIO) | data |
| SWD.3 | SWCLK | SWD clock, PA14 (JTCK); 5 V tolerant (silk CLK) | SWD-Takt, PA14 (JTCK); 5-V-tolerant (Aufdruck CLK) | data |
| SWD.4 | GND | Ground | Masse | ground |

Notes for the item: The STM32F103 runs at 2.0–3.6 V; pins marked 5 V tolerant (FT) take up to VDD + 4.0 V (absolute maximum), all others at most 4.0 V — PA0–PA7, PB0, PB1, PB5 and PC13–PC15 must never see 5 V. A 5 V signal on an FT pin also needs the internal pull-up/pull-down disabled (ST footnote). Each I/O pin sinks or sources 25 mA at most (absolute maximum); PC13–PC15 only 3 mA and 2 MHz. PB3, PB4 and PA15 start as JTAG pins and must be remapped before GPIO use; PA13/PA14 are the SWD pins. The 5V pin connects straight to the USB 5 V with no protection: do not feed it while USB is plugged in (stm32-base). The USB D+ pull-up on many Blue Pills has the wrong value (10 kΩ or 4.7 kΩ instead of 1.5 kΩ; stm32-base, VCC-GND schematic R10 4.7 kΩ). BOOT0 and BOOT1 (PB2) are jumpers, not header pins. Parts on clones vary (3.3 V regulator RT9193-33 in the VCC-GND schematic, TX6211B per stm32-base; some boards carry other STM32F103 variants or compatible chips).

## Sources
1. STMicroelectronics, STM32F103x8 STM32F103xB datasheet, DocID13587 Rev 16 (August 2013), copy of ST's PDF from RS Components, https://nonprod.docs.rs-online.com/49e6/0900766b812ffd87.pdf (ST's own URL https://www.st.com/resource/en/datasheet/stm32f103c8.pdf timed out twice) — table 5 pin definitions (pages 28–33, rendered and read: LQFP48 column, type, I/O level FT, main and alternate functions, footnotes 2 and 5), table 6 voltage characteristics, table 7 current characteristics; accessed 2026-11-24
2. stm32-base.org, "STM32F103C8T6 Blue Pill" board page, https://stm32-base.org/boards/STM32F103C8T6-Blue-Pill.html — Header 1, Header 2 and SWD header tables, power notes (5V tied to USB, no protection; regulator), USB D+ resistor note, LED on PC13, BOOT jumpers; photo "Blue Pill: Top view" https://stm32-base.org/assets/img/boards/STM32F103C8T6_Blue_Pill-2.jpg (looked at, zoomed); accessed 2026-11-24
3. Zephyr Project, "STM32 Minimum Development Board" (stm32_min_dev), https://docs.zephyrproject.org/latest/boards/others/stm32_min_dev/doc/index.html — Blue Pill LED on PC13, ST-LINK wiring G/CLK/IO/V3; pinout figure https://docs.zephyrproject.org/latest/_images/stm32_min_dev_pinout_blue.jpg ("The Generic STM32F103 Pinout Diagram", looked at, enlarged); accessed 2026-11-24
4. VCC-GND Studio (源地工作室, www.vcc-gnd.com), schematic "STM32F103C8T6核心板原理图", dated 2016/1/26, copy hosted by stm32-base.org, https://stm32-base.org/assets/pdf/boards/original-schematic-STM32F103C8T6-Blue_Pill.pdf — P3, P4, P2 (SWD), LEDs D1/D2, USB circuit R9/R10/R11, regulator; rendered and read; accessed 2026-11-24

## Cross-check
Header layout, the two independent sources: stm32-base's tables and photo (source 2) against the Zephyr figure (source 3). Right column (H1 from pin 20 down to pin 1): G, G, 3.3, R, B11, B10, B1, B0, A7, A6, A5, A4, A3, A2, A1, A0, C15, C14, C13, VB — Zephyr: GND, GND, 3.3V, NRST, PB11, PB10, PB1, PB0, PA7 … PA0, PC15, PC14, PC13, VBAT. Left column (H2 from 20 to 1): B12, B13, B14, B15, A8, A9, A10, A11, A12, A15, B3, B4, B5, B6, B7, B8, B9, 5V, G, 3.3 — Zephyr: PB12 … PB9, 5V, GND, 3.3V. SWD left to right (USB up): 3V3, DIO, CLK, GND — Zephyr: 3.3V, SWDIO, SWCLK, GND. Match on all 44.
The VCC-GND schematic (source 4) numbers P3/P4/P2 exactly like stm32-base's Header 1/2/SWD.
Chip functions against ST (source 1): every port name on the headers exists on the LQFP48 (pins 2–4, 7, 10–19, 21, 22, 25–34, 37–43, 45, 46); the LQFP48 numbers printed in the Zephyr figure equal ST's LQFP48 column.

Differences found:
- PB5: ST's table gives no FT for PB5 (all neighbours PB3, PB4, PB6–PB15 are FT). Some community diagrams mark PB5 tolerant; I follow ST.
- Regulator: RT9193-33 in the VCC-GND schematic, TX6211B per stm32-base — clones differ; not used in `function`.
- USB D+ pull-up: schematic R10 4.7 kΩ; stm32-base "10kΩ or 4.7kΩ" instead of 1.5 kΩ.
- ST datasheet revision: Rev 16 (2013) read; newer revisions (DS5319 Rev 18–20) exist but st.com did not respond. Pin definitions of this long-produced part are not expected to change, but that was not checked.

## Choices
- Catalog item: USB at the top as in the Zephyr figure (`connector: top`); right column H1.20 to H1.1, left column H2.20 to H2.1, and the 4-pin SWD header as an inner group SWD.1 to SWD.4 (left to right, USB up), drawn in the middle of the board because a `bottom` list would give each pad the width of the longest pin number of the board (H2.20), which does not fit four pads at the 24-unit pitch. Names are the port names, not the silk short forms. PB5 not being 5 V tolerant is a pin note of H2.8 and in the item notes; the differences between clones (regulator, USB D+ pull-up) are in the notes and as a pin note of PA12.
- Ids H1/H2/SWD after stm32-base's "Header 1/2" (the board carries no printed header names); the VCC-GND P3/P4/P2 numbers are equal and named in Header numbers.
- Names are the port names (PA0 … PC15) instead of the silk short forms (A0, B12, C13); `NRST` for silk R, `VBAT` for VB, `3V3` for 3.3, `GND` for G.
- "3.3 V only" / "5 V tolerant" written into each GPIO's `function`, from ST's FT column.
- SWDIO/SWCLK type `data` (debug bus), NRST `signal`, VBAT `power`.
- USB at the top as in the Zephyr figure; the stm32-base photo is that view turned by 90°.

## Spot checks
1. H2.20 = PB12 (top left) — stm32-base Header 2 row 20 "B12 PB12"; Zephyr figure "PB12 25" top left; ST LQFP48 pin 25 PB12, FT, SPI2_NSS.
2. H2.8 = PB5, not 5 V tolerant — stm32-base Header 2 row 8 "B5"; Zephyr "PB5 41"; ST table page 32: PB5 I/O level column empty (no FT).
3. H2.3 = 5V — stm32-base Header 2 row 3 "5V +5V rail"; Zephyr "5V" below PB9; VCC-GND P4.3 5V.
4. H1.1 = VBAT (bottom right, square pad) — stm32-base Header 1 row 1 "VB VBAT"; Zephyr "1 VBAT" bottom right; photo square pad at VB.
5. H1.17 = NRST — stm32-base Header 1 row 17 "R NRST"; Zephyr "7 NRST RESET BUTTON"; ST LQFP48 pin 7 NRST.
6. H1.5 = PA0 (ADC12_IN0, 3.3 V only) — stm32-base Header 1 row 5; Zephyr "10 PA0 ADC0"; ST LQFP48 pin 10, no FT.
7. SWD.2 = SWDIO (PA13) — stm32-base SWD row 2 "DIO SWDIO PA13"; Zephyr "34 SWDIO PA13"; ST LQFP48 pin 34 PA13 JTMS/SWDIO, FT.

UNVERIFIED: no maker document exists for the board; clones may differ from this layout (check the silk). Whether a given board carries a genuine STM32F103C8 (pin functions and FT marks are ST's) is outside this record.
