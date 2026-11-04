# avr_isp_6 — AVR ISP (6-pin) / AVR ISP (6-polig)

Kind: authoritative maker document
Pins: 6; types: power 1, ground 1, signal 1, data 3

## View
Written for the header on the target board seen from above, looking at its pins (the mating face): `drawn: plug`, rows [1 2], [3 4], [5 6], odd pins left.

Basis: AVR910 caption "Figure 1-2. Recommended In-System Programming Interface Connector Layout (Top View)", rendered and read: MISO 1 / 2 VCC, SCK 3 / 4 MOSI, RESET 5 / 6 GND, on a "2 x 3 pin header contact, with pin spacing of 100 mils". AN2519 figure 4-1 and Atmel-ICE figure 4-10 draw the same layout with "1 2" on top.

`key: left` is derived, not drawn. The Atmel-ICE guide (page 15) puts pin 1 of its 6-pin 100-mil connector "to the right of the keying", seen from the cable. Looking at the socket from the cable is the same direction as looking down on the header. With the key turned to the top, pin 1 is to its right only if the key lies on the side of pins 1, 3 and 5, which in the drawn view is the left side.

## Sources
1. Atmel/Microchip, AVR910: In-System Programming, Atmel-0943F, 11/2016, https://ww1.microchip.com/downloads/en/AppNotes/Atmel-0943-In-System-Programming_ApplicationNote_AVR910.pdf — page 3, figures 1-1 and 1-2; signal table in section 2. Accessed 2026-11-04.
2. Microchip, AN2519 AVR Microcontroller Hardware Design Considerations, DS00002519B, https://ww1.microchip.com/downloads/en/Appnotes/AN2519-AVR-Microcontroller-Hardware-Design-Considerations-00002519B.pdf — page 9, figure 4-1, VTG supply text. Accessed 2026-11-04.
3. Atmel-ICE User Guide, Atmel-42330C, 10/2016, https://ww1.microchip.com/downloads/en/DeviceDoc/Atmel-ICE_UserGuide.pdf — page 15 (keying), page 38 (figure 4-10), table 3-6. Accessed 2026-11-04.
4. Microchip, MPLAB ICE 4 In-Circuit Emulator User's Guide, section 3.3.4.2.2, table 3-7, https://onlinedocs.microchip.com/oxy/GUID-E9773D3D-A940-4268-9E35-189A30B5467E-en-US-1/GUID-A86E8194-0702-42A1-99BC-DC4BC6CDB518.html — the same mapping in a current tool guide. Accessed 2026-11-04.

## Cross-check
All four agree on every pin. Spelling only:
- Pin 2: VCC (AVR910, Atmel-ICE figure), VTG (AN2519, table 3-6, MPLAB ICE 4). Chose VCC, VTG is in the function.
- Pin 5: RESET with overbar (AVR910), RST (AN2519), /RESET (Atmel-ICE). Chose RESET, the low level is in the function.
- Pins 1 and 4: PDO/MISO and PDI/MOSI in Atmel-ICE figure 4-10, because that header is also used for PDI on older parts. Chose MISO and MOSI.

Unverified: nothing about the pins. No document draws the notch for this header; see the derivation above.

## Choices
- `key: left` although many boards (for example bare 2 × 3 headers) have no shroud. The note says so.
- "No crossover" is taken from AVR910 figure 1-1, which joins MISO to MISO and MOSI to MOSI.
- The 10-way ISP header is not part of this item. It is drawn in AN2519 figure 4-1 with another layout.
- The reset line is `signal`, SCK, MISO and MOSI are `data` by the catalog schema.
