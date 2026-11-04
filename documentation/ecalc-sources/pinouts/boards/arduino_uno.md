# arduino_uno — Arduino Uno R3 headers / Arduino Uno R3 Stiftleisten

Kind: authoritative maker document
Pins: 38; types: power 5; ground 4; signal 3; data 3; io 22; nc 1

## View
Board from the component side, portrait, USB at the top, as `connector: top`. Basis: Arduino's UNO R3 Full Pinout (A000066) page 1 is captioned "TOP VIEW" and I rendered and looked at it: USB and DC jack at the top edge, header POWER/AD on the left, IOH/IOL on the right, from top to bottom NC … VIN, gap, A0 … A5 and D19/SCL … D8, gap, D7 … D0/RX. The YAML lists are read straight from that figure.

ICSP block: same figure family, page 4, also "TOP VIEW", with numbered circles around the header at the bottom of the board: upper row 5 3 1, lower row 6 4 2 (zoomed render checked). So pin 1 is at the upper right in this orientation, which is what the `inner` rows state.

Position along the edge: in the figure both columns end level at the bottom (A5 and D0/RX), and the left column starts about 3.6 pitches lower than the right one, with NC about level with D13. So the left list starts with four `null`, which puts NC beside D13 and A5 beside D0.

Header numbers: the schematic's header names POWER, AD, IOH, IOL, ICSP plus the position printed there. POWER and AD count from the top of the left side; IOH and IOL have pin 1 at the D8 and D0 end, which is why the right list runs 10…1 and 8…1. The label at each schematic position (for example IOH pin 6 = 13, IOL pin 1 = 0) ties the numbers to the figure's names.

## Sources
1. Arduino, UNO R3 Full Pinout, SKU A000066, last update 6 Oct 2022, https://docs.arduino.cc/resources/pinouts/A000066-full-pinout.pdf — pages 1, 3 (advanced functions, read as text) and 4 (ICSP) rendered and read, accessed 2026-11-04
2. Arduino, UNO-TH_Rev3e schematic (Arduino(TM) UNO Rev3, dated 06/03/19), https://docs.arduino.cc/resources/schematics/A000066-schematics.pdf — header names, positions and nets, rendered and zoomed, accessed 2026-11-04
3. Arduino, UNO R3 datasheet (user manual), SKU A000066, modified 03/11/2026, https://docs.arduino.cc/resources/datasheets/A000066-datasheet.pdf — sections 2.2, 5.1, 5.2, accessed 2026-11-04

## Cross-check
Schematic and full pinout agree on every pin. Differences:
- The datasheet's section 5 numbers the pins differently (JANALOG 1–14, JDIGITAL 1–18). Its D-numbers match, but it writes "A4/SD4" and "A5/SD5" for pins 17 and 18 (typos for SDA/SCL). I use the schematic's header names as planned.
- The datasheet types +3V3, +5V and VIN as "Power", and IOREF as its own type. The catalog schema has no such type; I use `power` because the schematic ties it to +5V.
- The datasheet's section 3.2 says "up to 20 MHz" for the processor; not used here.
- The schematic calls the SPI pins MISO/MOSI, the full pinout CIPO/COPI and says these are the former names. Name follows the full pinout; the old names are in `function` and in the tags.
- The VIN range 6–20 V stands in both the full pinout's legend and the datasheet's table 2.2. No UNVERIFIED pin.

## Choices
- Names: `3V3` and `5V` for the pinout's `+3V3` and `+5V`, as planned; the plain D-numbers for `D1/TX`, `~D3` and so on, with TX, RX and PWM in `function`.
- IOH.9 and IOH.10 are named `SDA` and `SCL`, type `io`: the full pinout calls them D18/SDA and D19/SCL and they are the same nets as A4 and A5.
- AREF is `signal`, IOREF is `power`, NC is `nc`.
- The gaps: one `null` between VIN and A0 and one between D8 and D7. In the figure the D8/D7 spacing is about 1.6 pitches and the VIN/A0 spacing about 2; the catalog schema only knows whole pitches, so D7 … D0 sit about half a pitch lower in the drawing than to scale.
- Only the ICSP header of the ATmega328P is an `inner` block, as planned. ICSP1 of the ATmega16U2 is named in the notes, not drawn, because its pins would add six more rows for a header that very few people need.
- The optional 2x2 JP2 pads are left out.
