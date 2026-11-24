# ps2 — PS/2 keyboard/mouse (6-pin mini-DIN) / PS/2 Tastatur/Maus (6-polig Mini-DIN)

Kind: maker's own document (IBM, originator of the PS/2 port) cross-checked with a computer maker (Dell, two documents) and a connector maker's geometry (Kycon)
Connector: 6-pin mini-DIN, circular, keyed. Computer side: female socket; cable side: male plug. Drawn side: the socket on the computer (`drawn: socket`).
Pins: 6; types: data 2, ground 1, power 1, nc 2 (plus the shield, not numbered)

## Pins (keyboard and mouse ports are the same)

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | Data | Keyboard or mouse data, bidirectional, open collector with pull-up to +5 V | Tastatur- oder Mausdaten, bidirektional, Open Collector mit Pull-up auf +5 V | data |
| 2 | NC | Reserved, not connected | Reserviert, nicht belegt | nc |
| 3 | GND | Ground | Masse | ground |
| 4 | +5V | +5 V supply from the computer (fused) | +5-V-Versorgung vom Computer (abgesichert) | power |
| 5 | Clock | Keyboard or mouse clock, bidirectional, open collector with pull-up to +5 V | Tastatur- oder Maustakt, bidirektional, Open Collector mit Pull-up auf +5 V | data |
| 6 | NC | Reserved, not connected | Reserviert, nicht belegt | nc |

Shield / shell: frame (chassis) ground (IBM "Shield — Frame Ground"; Dell "Shell — chassis ground"). Not numbered; for `notes`.

Variant to warn about in `notes` (not a separate variant): IBM ThinkPad 560 has one combined keyboard/mouse port with Mouse Data on 1, **Keyboard Data on 2**, Ground 3, +5 V dc 4, Mouse Clock 5, **Keyboard Clock on 6** (for a Y-splitter). So pins 2 and 6 are not always free; do not use them for anything else.

## View
Written for the **female socket on the computer, looking at its mating face**, key (notch in the shell, plastic key between the two top contacts) at 12 o'clock (`drawn: socket`, `key: 0`):
- 6 upper left, 5 upper right,
- 4 left, 3 right (slightly below the horizontal centre line),
- 2 lower left, 1 lower right (close to the bottom).

Positions for `type: circular` (angles clockwise from 12 o'clock, socket face; radius as a fraction of the receptacle opening radius, measured on Kycon's 6-position front view):
- pin 6: angle 310, radius 0.65
- pin 5: angle 50, radius 0.65
- pin 4: angle 263, radius 0.63
- pin 3: angle 97, radius 0.63
- pin 2: angle 203, radius 0.63
- pin 1: angle 157, radius 0.63

Plug (cable end), looking at the tip of its pins: the mirror image — 5 upper left, 6 upper right, 3 left, 4 right, 1 lower left, 2 lower right, key at 12 o'clock (angles 360 − a: 5 = 310, 6 = 50, 3 = 263, 4 = 97, 1 = 203, 2 = 157).

Basis:
- IBM, Keyboards (101- and 102-Key) technical reference, Figure 22: the "System Connector" (the plug at the end of the keyboard cable) drawn from its tip, rendered and looked at: notch/key at the top of the shell, a vertical plastic key in the centre, labels 5 (upper left) and 6 (upper right), 3 and 4 below them, 1 and 2 at the bottom. That is the plug face.
- Dell PowerEdge 1600SC Installation and Troubleshooting Guide, Figure B-4 "Keyboard and Mouse Connector Pin Numbers" (the port on the server): 6 upper left, 5 upper right, 4 left, 3 right, 2 lower left, 1 lower right, key at the top — exactly the mirror of IBM's plug. Socket face.
- Dell PowerEdge 8450 User's Guide, Figure 4 "Pin Numbers for the Keyboard Connector": the same socket turned by 90° (key to the right); going clockwise the labels read 3, 1, 2, 4, 6, 5, which is the cyclic order of the 1600SC figure. Consistent (a rotation, not a mirror).
- Kycon drawing KMDGX-xS-BS (rev C6, 03/05/18), KMDGX-6S-BS front view: notch at the top, contacts upper left/right, left/right, lower left/right, a vertical rectangular key in the centre; the angles above were measured on a render of this view (contact centres relative to the opening's centre). **Kycon numbers the six contacts 8, 6, 5, 3, 2, 1 (its own position scheme shared with the 8-pin part); those are not PS/2 pin numbers.** Only the geometry is taken from Kycon.

**For the drawing:** a `circular` outline with the key at 0° (12 o'clock) in the socket view.

## Sources
1. IBM, "Hardware Interface Technical Reference — Keyboards (101- and 102-Key)", October 1990, section "Cables and Connectors", Figure 22 "Keyboard Connectors Signal and Voltage Assignments" (page 49 of the section), scanned copy at https://ardent-tool.com/docs/pdf/ibm_hitrc11.pdf. Accessed 2026-11-24.
2. Dell, PowerEdge 1600SC Systems Installation and Troubleshooting Guide, appendix B "Jumpers, Switches, and Connectors": Figure B-4 and Table B-4 "Keyboard and Mouse Connector Pin Assignments", copy at https://cs.uwaterloo.ca/~brecht/servers/docs/PowerEdge-2600/en/Pe1600SC/IT/0r626ab0.htm, figure 0r626ac3.jpg. Accessed 2026-11-24.
3. Dell, PowerEdge 8450 Systems User's Guide, "I/O Ports and Connectors": Figure 4/Table 3 (keyboard), Figure 5/Table 4 (mouse), copy at https://cs.uwaterloo.ca/~brecht/servers/docs/PowerEdge-2600/en/Pe8450/UG/io.htm, figure io-4.gif. Accessed 2026-11-24.
4. IBM, ThinkPad 560/560E technical information, "Keyboard/Mouse Connector", Figure 2-2 and pin table, read at https://www.manualsdir.com/manuals/128462/ibm-560.html?page=26 — open-collector drivers pulled to 5 V; combined-port variant. Accessed 2026-11-24.
5. Kycon, drawing KMDGX-xS-BS "Circular Mini-DIN Sockets, Au Plated Contact Areas", rev C6 (03/05/18), https://cloud.kycon.com/Pub_Eng_Draw/KMDGX-xS-BS%20Series.pdf — front views of the 3- to 8-position sockets. Accessed 2026-11-24.

## Cross-check
Functions: IBM Figure 22 (1 +KBD DATA, 2 Reserved, 3 Ground, 4 +5.0 Vdc, 5 +KBD CLK, 6 Reserved, Shield Frame Ground) and Dell 8450 (1 KBDATA / MFDATA, 2 NC, 3 GND, 4 FVcc fused supply voltage, 5 KBCLK / MFCLK, 6 NC) and Dell 1600SC (same, shell chassis ground) agree on every pin. The ThinkPad 560 combined port differs on 2 and 6 (see above) — a different port, not a conflict.

Positions: IBM (plug) and Dell 1600SC (socket) are mirror images, as they must be; Dell 8450 is the socket rotated by 90°. Kycon gives the same six-position geometry with the notch at the top.

## Choices
- Catalog item: `data_av.yaml`, group `data`, shape `circular`, `contact: round`, `drawn: socket`, `key: 0`, the six angles and radii above; plug and socket views as `midi`; the ThinkPad 560 combined-port variation is in `notes` and in the notes of pins 2 and 6, no colours and no current limit.
- `drawn: socket`, `key: 0` (notch at 12 o'clock), as the item `midi` does.
- Data and clock typed `data` (bidirectional serial lines, as DDC in `vga`); 2 and 6 `nc` with a note about combined ports.
- Names "Data", "Clock" without KBD/MOUSE, because both ports use the same pins; the item `doc` can say keyboard purple, mouse green only if a source is found (PC 99 colours were not verified here, so leave colours out).
- +5 V current: IBM ThinkPad 560 gives 0.5 A for its port; Dell says "fused". No general figure is verified — leave the current out.

## Spot-check list
- Pin 1 = data — IBM Figure 22; Dell 8450 Table 3; Dell 1600SC Table B-4.
- Pin 3 = ground, pin 4 = +5 V — IBM Figure 22; Dell 8450 Table 3.
- Pin 5 = clock — IBM Figure 22; Dell 8450 Tables 3 and 4.
- Pins 2 and 6 reserved / no connection — IBM Figure 22; Dell 1600SC Table B-4.
- Socket face: 6 upper left, 5 upper right, 2 lower left, 1 lower right, key at the top — Dell 1600SC Figure B-4; mirror of IBM Figure 22.

UNVERIFIED: the exact angles are measured on Kycon's generic mini-DIN drawing, not on a PS/2-specific document (IBM and Dell figures agree in arrangement but are not dimensioned). PC 99 port colours not checked. No current limit for pin 4 stated.
