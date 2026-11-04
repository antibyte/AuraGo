# xlr — XLR 3-pin / XLR 3-polig

Kind: cross-checked reproductions
Pins: 3; types: ground 1, signal 2

## View
Written for the male connector, looking at its mating face (`drawn: plug`), rotated so that pin 3 is straight below the centre.

Angles (clockwise from 12 o'clock, male face): **pin 1 = 270°, pin 2 = 90°, pin 3 = 180°, radius 0.5; key = 0° (the latch).** The female face follows from the mirror rule: pin 1 = 90°, pin 2 = 270°, key = 0°. The polarising rib sits at 45° on the male face and at 315° on the female face.

Derivation:
- Neutrik drawing ST-NC3MAH, front view of the male receptacle: 1 at the left, 2 at the right, 3 below the centre, with the latch cut-out at 12 o'clock. Measured on the rendered drawing from the centre: pin 1 is 104 px left, pin 2 is 103 px right, pin 3 is 100 px below, all on the level of the centre lines. So the pattern is a T with equal radii, not a 120-degree triangle.
- Neutrik drawing 3102 St 18 19, front view of the female NC3FAV1, "PUSH" latch on top: 2 at the left, 1 at the right, 3 below. This is exactly the mirror image. The same sheet's rear view shows 1 left, 2 right, and its "PCB-Layout (component side)" shows 2 and "1+G".
- Amphenol, "AX Series Contact Arrangements (Front view of pin inserts)": 1 left, 2 right, 3 below, all three at the same distance from the centre.
- ITT Cannon, the originator of the XLR, "Contact Arrangement Configuration (Engaging View Pin Insert)": the same pattern, without readable numbers at this print quality.
- Key: Amphenol and ITT Cannon draw a small rib on the rim of the pin insert at the upper right. Measured at 45° on both Amphenol's figure (95 px right, 95 px up) and Neutrik's male drawing (143 px, 145 px). Neutrik's female drawing has it at the upper left, at 315°.
- Radius: 104 px of an inner radius of about 200 px on ST-NC3MAH, 70 of 137 px on Amphenol's figure, so 0.5.

## Sources
1. Neutrik, drawing ST-NC3MAH, "XLR Chassis Connectors NC3MAH", copy at https://pdf2.datasheet.su/neutrik/nc3mah.pdf — front view of the male connector, pin numbers and key position, accessed 2026-11-04
2. Neutrik, product page and drawing 3102 St 18 19, NC3FAV1, copy at https://pdf2.datasheet.su/neutrik/nc3fav1.pdf — front and rear view of the female connector, PCB layout, latch position, accessed 2026-11-04
3. Amphenol Australia, Entertainment Interconnect Catalogue, "Standard Data AX Series Cable Connectors" (page 7, Rev 2, 11/2009), copy at https://allchip.ru/content/pdfs/142/4573652.pdf — front view of pin inserts with numbers and key, accessed 2026-11-04
4. ITT Cannon, Audio XL catalogue, "XLR Series" (page 8), copy at https://radel.ru/upload/iblock/e44/8o3rfdvnh0bl7u3qrdvbzn6o0edrp67t.pdf — engaging view of the pin insert, pattern and key, no readable numbers, accessed 2026-11-04
5. Rane Corporation, Rane Note 102, "Analog I/O Standards", https://www.ranecommercial.com/legacy/pdf/old/note102.pdf — pin functions: "Pin 1 Ground (Shield, screen, etc.)", "Pin 2 Positive", "Pin 3 Negative", per IEC 268, with AES14-1992 agreeing, accessed 2026-11-04
6. Shure, SM58 user guide, https://pubs.shure.com/view/guide/SM58/en-US.pdf — three-pin XLR male connector, positive pressure gives positive voltage on pin 2 with respect to pin 3, accessed 2026-11-04
7. Mackie, 1620 manual, appendix B "Connections", https://www.manualslib.com/manual/97163/Mackie-1620.html?page=23 — "Pin 1 = Shield", "Pin 2 = Hot (+)", "Pin 3 = Cold", accessed 2026-11-04

## Cross-check
Functions: Rane, Shure and Mackie agree. Geometry: Neutrik and Amphenol agree, and Neutrik's male and female drawings are mirror images of each other.

AES14 itself was not read: the AES preview page answered 403, and the standard is not free. Its content here comes only through Rane's statement. Rane calls "European" and "American" wiring a myth, but also says many makers do not follow the standard, so the notes tell the reader to check old equipment.

**A correction worth keeping:** I first expected 2 at the left and 1 at the right on the male face, and an equal triangle. That is the female face. The drawings decide.

## Choices
- Only one angle fits `key`. The catalog gives the latch at 12 o'clock (0°), the feature people orient the connector by. The polarising rib (45° on the male face) is named in `notes`.
- `Cold (-)` uses a plain hyphen, as a pin name people type.
- `signal` for pins 2 and 3: analogue audio, as in the catalog schema.
- No `wire`: cable colours are not fixed by these sources.
