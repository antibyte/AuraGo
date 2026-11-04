# raspberry_pi_pico — Raspberry Pi Pico and Pico W pins / Raspberry Pi Pico und Pico W Pins

Kind: authoritative maker document
Pins: 43; types: power 3; ground 9; signal 3; data 2; io 26

## View
Board from the component side, portrait, USB at the top, as `connector: top`. Basis:
- Raspberry Pi Pico Datasheet, figure 4, "The pin numbering of the Raspberry Pi Pico Rev3 board" (page rendered and looked at): USB at the top, 1 … 20 down the left side, 40 … 21 down the right side, pads D1, D2, D3 from left to right on the bottom edge. The figure's text "TP1-6 ARE ON REVERSE SIDE" shows it is drawn from the component side.
- The pinout diagram `Pico-R3-A4-Pinout.pdf` (figure 2 of the datasheet; rendered and read), with the RP2040 and BOOTSEL visible, gives the same positions and the signal names, and shows the debug pads SWCLK, GND, SWDIO from left to right.
- Pico W: datasheet figure 4 and `PicoW-A4-Pinout.pdf` give the same 1 … 40. Its debug pads are at a different place: inside the board, level with pins 13 and 28, labelled SWCLK, GND, SWDIO from left to right.

## Sources
1. Raspberry Pi, Raspberry Pi Pico Datasheet, https://datasheets.raspberrypi.com/pico/pico-datasheet.pdf — figures 2 and 4, sections 2.1 and 4.2, accessed 2026-11-04
2. Raspberry Pi, Pico pinout diagram, https://datasheets.raspberrypi.com/pico/Pico-R3-A4-Pinout.pdf — alternative functions and defaults, accessed 2026-11-04
3. Raspberry Pi, Raspberry Pi Pico W Datasheet, https://datasheets.raspberrypi.com/picow/pico-w-datasheet.pdf — figures 3 and 4, section 2.1 (internal GPIOs, LED), accessed 2026-11-04
4. Raspberry Pi, Pico W pinout diagram, https://datasheets.raspberrypi.com/picow/PicoW-A4-Pinout.pdf — same pins, debug pad position, LED, accessed 2026-11-04

## Cross-check
The two datasheets and the two diagrams agree on pins 1 to 40. Details:
- Debug pads differ between the boards, as above. The task puts them at the bottom edge, which is the Pico; the notes state the Pico W position. This is the one place where the shared drawing is not physically right for the Pico W.
- The D1, D2, D3 numbers are in the Pico figure 4. The Pico W figure 4 prints only the signal names at these pads. I use the Pico's numbers for both.
- Pin 36: the diagram writes 3V3(OUT), the datasheet's list PIN36 3V3. Name follows the diagram because it says the direction.
- Pin 33: the diagram shows GND and AGND, the datasheet's list AGND. Name is `AGND`.
- "(default)" is read from the colour legend of the diagrams (darker tag = default): GP0/GP1 UART0, GP4/GP5 I2C0, GP16–GP19 SPI0. Checked on a zoomed render of the Pico diagram.
No UNVERIFIED pin.

## Choices
- One item for both boards, with no variants, as planned.
- Pico 2 and Pico 2 W are not covered: I did not read their documents.
- ADC_VREF is `signal`, like AREF on the Arduino boards, although it also feeds the ADC; 3V3_EN and RUN are `signal`.
- AGND and the debug pad D2 count as `ground`.
- SWCLK and SWDIO are `data` (a serial line and its clock).
- Pad numbers D1–D3 can be mistaken for Arduino D pins; the names beside them make clear what they are.
- The test points TP1–TP6 on the back are left out.
