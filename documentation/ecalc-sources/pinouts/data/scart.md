# scart — SCART (EN 50049-1, 21-pin) / SCART (EN 50049-1, 21-polig)

Kind: cross-checked reproductions (the standard EN 50049-1 / IEC 60933-1 is paywalled and was not read)
Connector: SCART (Péritel, Euro-AV), 21 positions in two staggered rows: 10 even positions in the row on the wide side of the shell, 11 positions (the odd pins 1–19 plus 21) in the row on the narrow side. Drawn side: the female socket on a TV, VCR or receiver (`drawn: socket`). Pin 21 is a contact in the socket and the metal shell on the plug.
Pins: 21; types: signal 11, ground 8, data 2

## Pins
Directions are given for a TV. Each device has its outputs on 1, 3, 19 and its inputs on 2, 6, 20; a SCART cable crosses 1↔2, 3↔6 and 19↔20 and runs the other pins straight, so RGB (7, 11, 15), pin 8 and pin 16 are inputs at a TV and outputs at a source (player, receiver, console).

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | Audio Out R | Audio output, right | Audioausgang rechts | signal |
| 2 | Audio In R | Audio input, right | Audioeingang rechts | signal |
| 3 | Audio Out L | Audio output, left (mono) | Audioausgang links (Mono) | signal |
| 4 | Audio GND | Audio ground | Audiomasse | ground |
| 5 | Blue GND | Ground of blue (pin 7) | Masse für Blau (Pin 7) | ground |
| 6 | Audio In L | Audio input, left (mono) | Audioeingang links (Mono) | signal |
| 7 | Blue | Blue video (RGB), input at a TV | Video Blau (RGB), Eingang am Fernseher | signal |
| 8 | Function Switch | Slow switching / status: 0–2 V TV, 5–8 V AV 16:9, 9.5–12 V AV 4:3 | Langsame Umschaltung / Status: 0–2 V TV, 5–8 V AV 16:9, 9,5–12 V AV 4:3 | signal |
| 9 | Green GND | Ground of green (pin 11) | Masse für Grün (Pin 11) | ground |
| 10 | Data 2 | Data line 2 / control bus (AV.link), often not connected | Datenleitung 2 / Steuerbus (AV.link), oft nicht belegt | data |
| 11 | Green | Green video (RGB), input at a TV | Video Grün (RGB), Eingang am Fernseher | signal |
| 12 | Data 1 | Data line 1, often not connected | Datenleitung 1, oft nicht belegt | data |
| 13 | Red GND | Ground of red (pin 15) | Masse für Rot (Pin 15) | ground |
| 14 | GND | Ground; devices use it for data, blanking or not at all | Masse; Geräte nutzen sie für Daten, Austastung oder gar nicht | ground |
| 15 | Red / C | Red video (RGB), or S-Video chroma, input at a TV | Video Rot (RGB) oder S-Video-Chroma, Eingang am Fernseher | signal |
| 16 | Blanking | Fast blanking / RGB switch: 0–0.4 V composite, 1–3 V RGB, 75 Ω | Schnelle Austastung / RGB-Umschaltung: 0–0,4 V FBAS, 1–3 V RGB, 75 Ω | signal |
| 17 | Video GND | Ground of composite video (pins 19, 20) | Masse für FBAS-Video (Pins 19, 20) | ground |
| 18 | Blanking GND | Ground of pin 16 | Masse für Pin 16 | ground |
| 19 | Video Out | Composite video output (or S-Video Y) | FBAS-Videoausgang (oder S-Video Y) | signal |
| 20 | Video In | Composite video input (or S-Video Y) | FBAS-Videoeingang (oder S-Video Y) | signal |
| 21 | Shield | Common ground and shield; the plug's shell | Gemeinsame Masse und Schirm; das Steckergehäuse | ground |

Type count: signal 1, 2, 3, 6, 7, 8, 11, 15, 16, 19, 20 = 11; ground 4, 5, 9, 13, 14, 17, 18, 21 = 8; data 10, 12 = 2.

## View
Written for the **female socket on a device, looking at its mating face**, with the wide side of the shell at the top: the right side of the shell is straight, the left side slants inward toward the bottom (the cut corner is the lower left) (`drawn: socket`).
- Top row (wide side), 10 positions: 20 18 16 14 12 10 8 6 4 2, left to right.
- Bottom row (narrow side), 11 positions: 21 19 17 15 13 11 9 7 5 3 1, left to right.
- Stagger: each top-row position sits halfway between two bottom-row positions (20 between 21 and 19, 2 between 3 and 1). Pitch 3.81 mm within a row, rows offset 1.905 mm sideways (Kycon). So the bottom row is the full row and the top row has `offset: 0.5`.

The plug (cable end), looking at its mating face, is the mirror image: top row 2 4 … 20 left to right, bottom row 1 3 … 19 left to right, the slanted side on the right. The plug has 20 pins; position 21 is its metal shell, which touches the socket's contact 21.

Basis:
- Kycon catalogue page 134, K-SCART-021 (single 21-position right-angle socket): front view rendered and looked at. Labels "20" above the left end and "2" at the right end of the 10-position top row, "21" below the left end and "1" at the right end of the 11-position bottom row; 1.905 mm column offset. Traced the shell outline in the render: the right edge is vertical, the left edge runs from x = 52 to x = 107 (render pixels) between the top and the bottom, so the shell is narrower at the bottom and the cut corner is the lower left. It is a front view: the side view placed to its right shows the face plate on the left, the body and PCB legs on the right, and the photo on the same page shows the same face.
- Pioneer KRP-600A operating instructions, page "Additional information" (SCART pin assignment): figure labels "1 3 5 7 9 11 13 15 17 19 21" in one row and "2 4 6 … 20" in the other. That is Kycon's front view turned by 180° (a rotation, no mirror), i.e. the socket as mounted on that device. Consistent.
- Wikipedia figure SCART_Connector_Pinout_Color.svg, captioned "Female connector seen from the front": shell outline path read from the SVG file: straight right side, left side slanting inward toward the bottom. Pin labels there are paths, not text, so its numbers were not read. Further check only.
- RND Connect (Distrelec) RND 205-01033 SCART male plug drawing: numbered view with 19 17 … 1 in one row and 20 18 … 2 in the other, the even row starting half a pitch further left. That is the plug's mating face turned by 180° (or its rear flipped; the drawing does not say which), consistent with the mirror rule. Not used for the direction.

**For the drawing:** the view holds only if the `scart` outline has its wide side at the top and the slanted side on the left in the socket view.

## Sources
1. Kycon, Inc., catalogue page 134 "SCART Connectors, K-SCART Series" (K-SCART-021, -028A, -022, -024; dimensions in mm), copy at https://allchip.ru/content/pdfs/283/9080335.pdf — front view of the K-SCART-021 with contact numbers 1, 2, 20, 21. Accessed 2026-11-24.
2. Pioneer, KRP-600A (Kuro plasma) operating instructions, "Additional information", SCART pin assignment for INPUT 1, 2 and 3, read at https://www.manualslib.de/manual/539217/Pioneer-Krp-600A.html?page=107 — pin tables and row figure. Accessed 2026-11-24.
3. Daewoo, 2898ST service manual, "21 PIN EURO-SCART" table, read at https://www.manualslib.com/manual/931143/Daewoo-2898st.html?page=3 — pins 1–21 with designations and levels. Accessed 2026-11-24.
4. LG Electronics, 32LC2D service manual, "Table 1 Scart Arrangement 1 (Full Scart)" and "Table 2 (Half Scart)", read at https://www.manualslib.com/manual/1249135/Lg-32lc2d-B.html?page=11 and ?page=12 — pins, pin 8 and pin 16 levels. Accessed 2026-11-24.
5. AOC, L19WA91 service manual, "Full Scart Connector Pin Assignments", read at https://www.manualslib.com/manual/828423/Aoc-L19wa91.html?page=17. Accessed 2026-11-24.
6. RND Connect (Distrelec), RND 205-01033 "Video Connector SCART" (male) data sheet, copy at https://static.chipdip.ru/lib/621/DOC041621319.pdf — product drawing. Accessed 2026-11-24.
7. Wikipedia, SCART, https://en.wikipedia.org/wiki/SCART, with https://upload.wikimedia.org/wikipedia/commons/e/e3/SCART_Connector_Pinout_Color.svg — pin list, levels, figure. Further check only. Accessed 2026-11-24.

## Cross-check
Agree in Pioneer (input 1), Daewoo, LG and Wikipedia: 1 audio out right, 2 audio in right, 3 audio out left, 4 audio ground, 5 blue ground, 6 audio in left, 7 blue, 8 function/AV control, 9 green ground, 11 green, 13 red ground, 15 red, 16 RGB/blanking control, 19 video out, 20 video in, 21 shield/common ground. (Daewoo names no left/right on 1, 2, 3, 6, only "linked with"; LG and Pioneer do. AOC has the same, but labels pin 8 "Switch Signal Input (Fast Blanking)", which contradicts the others; not used for 8.)

Chroma on 15 and luma on 20: Daewoo ("Red In, C In", "Video In, Y In") and Pioneer inputs 2/3 ("Chroma S-Video input", "Video input/S-Video input") agree.

Levels: pin 8 — LG "Low (0–2V) TV Mode, Mid (5–8V) Wide Screen, High (9.5–12V) AV Mode", Daewoo "TV: 0–2V, PERI: 9.5–12V", Wikipedia list the same bands (its design section says 4.5–7 V for 16:9). Pin 16 — LG "High (1–3V) RGB, Low (0–0.4V) Composite", Daewoo "Logic 0: 0–0.4V, Logic 1: 1–3V, Imp 75", Wikipedia the same.

Disagreements (real device differences, not errors):
- Pin 10: Daewoo NC, LG "Comms Data 2", Pioneer "Not used" (input 1) / "AV link control" (inputs 2, 3), Wikipedia "Clock / Data 2", "Control bus (AV.link)".
- Pin 12: Daewoo NC, LG "Comms Data 1", Pioneer "Not used", Wikipedia "Reserved / Data 1".
- Pin 14: Daewoo NC, LG and AOC "Ground (Blanking)", Pioneer "Earth for Red/Green/Blue control", Wikipedia "Usually data signal ground (pins 8, 10 & 12)".
- Pin 17: LG and AOC "Ground (Video input & Output)", Daewoo "Video Earth", Pioneer "Earth for Video output".
- Pin 18: LG and AOC "Ground (RGB Switching Control)", Daewoo "Rapid Blanking Earth", Wikipedia "Blanking signal ground (pin 16)"; Pioneer "Earth for Video input".
Chosen: 17 = video ground, 18 = ground of pin 16 (three device tables and Wikipedia), 14 = plain ground with the variation in its note; 10 and 12 data lines with "often not connected".

## Choices
- Catalog item: `data_av.yaml`, group `data`, outline `scart`, `contact: round`, `drawn: socket`, top row 20..2 with `offset: 0.5`, bottom row 21..1, pin 21 in the list as `ground`; pins 10, 12, 14, 17, 18 as chosen here, the device variations are in their pin notes; the standard EN 50049-1 was not read and the item notes say so.
- `drawn: socket`: devices carry the socket, and the maker drawing is of the socket.
- Pin 21 is in the pin list as `ground` and in the rows at the left end of the bottom row (it is a contact in the socket); `notes` says it is the shell on the plug.
- Pin 8 and 16 are `signal` (switching voltages); 10 and 12 `data`; RGB and audio `signal`.
- Directions in the functions are for a TV, with the cable crossing in the item `notes`.
- Item names: "SCART (21-pin)" in every locale; Péritel/Euro-AV as tags.

## Spot-check list
- Pin 1 = audio output right, pin 2 = audio input right — Pioneer table (input 1); LG Table 1.
- Pin 8 levels 0–2 / 5–8 / 9.5–12 V — LG Table 1; Daewoo (0–2 V and 9.5–12 V).
- Pin 16 = RGB/blanking switch, 0–0.4 V composite, 1–3 V RGB — LG Table 1; Daewoo.
- Pin 19 = video output, pin 20 = video input — Pioneer; Daewoo; LG Table 2.
- Pin 21 = shield / common earth — Pioneer "Plug shield"; Daewoo "Common Earth"; LG "Common ground (Shield)".
- Position: 20 top-left, 2 top-right, 21 bottom-left, 1 bottom-right of the socket face — Kycon K-SCART-021 front view.

UNVERIFIED: EN 50049-1 / IEC 60933-1 not read (paywalled). The use of pins 10, 12 and 14 varies; their wording in the standard is not known to me. The 4.5–7 V vs 5–8 V band for 16:9 differs between sources (5–8 V chosen: LG and the Wikipedia list).
