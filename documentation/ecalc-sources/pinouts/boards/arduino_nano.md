# arduino_nano — Arduino Nano headers / Arduino Nano Stiftleisten

Kind: authoritative maker document
Pins: 36; types: power 4; ground 3; signal 6; data 3; io 20

## View
Board from the component side, portrait, USB at the top, as `connector: top`. Basis: Arduino's Nano Full Pinout (A000005) page 1, rendered and read: the Mini-B connector is at the top edge and the MCU and reset button are visible, so it is the component side (page 3 draws the other side separately and captions it "BOTTOM"). Left column top to bottom: D13, +3V3, AREF, A0 … A7, +5V, RESET, GND, VIN. Right column: D12, ~D11 … D2, GND, RESET, D0/RX, D1/TX. The two square pads in the figure are at VIN and D1/TX.

Numbers: the schematic's J1 runs 1 D1/TX, 2 D0/RX, 3 RESET, 4 GND, 5 D2 … 15 D12/MISO; J2 runs 1 VIN, 2 GND, 3 RESET, 4 +5V, 5 A7 … 12 A0, 13 AREF, 14 3V3, 15 D13/SCK. Pin 1 of each is the square pad of the figure, which fixes the lists as J2.15…J2.1 on the left and J1.15…J1.1 on the right.

ICSP block: page 3, left picture (component side, USB at the top), zoom checked: upper row 2 4 6, lower row 1 3 5.

## Sources
1. Arduino, Nano Full Pinout, SKU A000005, last update 30/06/2021, https://docs.arduino.cc/resources/pinouts/A000005-full-pinout.pdf — pages 1 to 3 rendered and read, accessed 2026-11-04
2. Arduino, NanoV3.3 schematic (dated 23/01/19), https://docs.arduino.cc/resources/schematics/A000005-schematics.pdf — J1, J2, J4 (ICSP), rendered and zoomed, accessed 2026-11-04
3. Arduino, Nano datasheet, SKU A000005, modified 03/11/2026, https://docs.arduino.cc/resources/datasheets/A000005-datasheet.pdf — features and section 5, accessed 2026-11-04

## Cross-check
Schematic and full pinout agree on every pin. Differences, all in the datasheet:
- Section 5.1 lists only 13 analog-side pins (no D13, no AREF) and numbers two of them 12; section 5.2 lists D1 … D13, Reset, GND. Not usable for numbering, so J1 and J2 come from the schematic.
- Section 5.1 calls A6 and A7 "Analog input … /GPIO". The full pinout draws them as analog only with no digital number, and the schematic wires them to ADC6 and ADC7 of the MCU. I chose analog only, type `signal`.
- Its features say "7-15V unregulated external power supply (pin 30)", the full pinout says "VIN 7-12 V input to the board". `function` uses the narrower 7–12 V; the notes give both.
- Its "pin 27" and "pin 30" are the old count around the board. Both match J2.4 and J2.1 if that count continues from J1 (1–15) back along J2 (30 = J2.1); I state only these two in the notes because they are what the document names.
No UNVERIFIED pin.

## Choices
- Classic Nano with ATmega328 (schematic NanoV3.3). Clones are not covered; nothing in the sources speaks about them.
- A6 and A7 are `signal`, not `io`: the catalog schema lists A0–A5 as `io`, and these two can only be read as analog.
- `ICSP.n` for the schematic's J4 (titled ICSP there), so that the label in the drawing and the number agree.
- Names as on the pinout, shortened as planned (`D1` for `D1/TX`, `3V3` for `+3V3`); TX and RX stand in `function`.
- J2.14 "output of the FT232RL": the schematic's 3V3 net comes from pin 17 (3V3OUT) of IC1. The 50 mA limit is in the pinout's legend.
