# s_video — S-Video (4-pin mini-DIN) / S-Video (4-polig Mini-DIN)

Kind: cross-checked reproductions (no standard text for the Y/C connector was read; the IEC part usually cited for it, IEC 60933-5, is paywalled and its number is not verified here)
Connector: 4-pin mini-DIN, circular, keyed. Devices have the female socket, cables the male plug. Drawn side: the socket on the device (`drawn: socket`).
Pins: 4; types: ground 2, signal 2 (plus the shield, not numbered)

## Pins

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | GND | Ground (analog) | Masse (analog) | ground |
| 2 | GND | Ground (analog) | Masse (analog) | ground |
| 3 | Y | Luminance (brightness and sync) | Luminanz (Helligkeit und Sync) | signal |
| 4 | C | Chrominance (colour) | Chrominanz (Farbe) | signal |

Both sources name pins 1 and 2 only "GND" / "Analog GND". The common statement "1 = ground of Y, 2 = ground of C" was not read in a reputable source; leave it out or put it in `notes` as unverified.

Apple's 7-pin S-video socket accepts a 4-pin plug in positions 1–4 (its extra pins: 5 composite video, 6 and 7 unused) — item `notes` may mention that a 7-pin socket takes the 4-pin plug, since Apple's figure shows positions 1–4 where the 4-pin layout has them.

## View
Written for the **female socket on the device, looking at its mating face**, shell notch at 12 o'clock and the rectangular plastic key at 6 o'clock (`drawn: socket`, `key: 0`):
- 4 upper left, 3 upper right,
- 2 left, 1 right (slightly below the horizontal centre line).

Positions for `type: circular` (angles clockwise from 12 o'clock, socket face; radius as a fraction of the receptacle opening radius, measured on Kycon's 4-position front view):
- pin 4: angle 310, radius 0.65
- pin 3: angle 50, radius 0.65
- pin 2: angle 263, radius 0.63
- pin 1: angle 97, radius 0.63

Plug (cable end), looking at the tip of its pins: the mirror image — 3 upper left, 4 upper right, 1 left, 2 right (angles: 3 = 310, 4 = 50, 1 = 263, 2 = 97), key at 6 o'clock, shell notch feature at 12.

Basis:
- Euresys Picolo handbook, "S-VIDEO Connector", figure captioned "S-VIDEO connector (Bracket viewed from outside)" (a female right-angle PCB socket on a capture card, i.e. the socket face): looked at the image (109 × 109 px). Key rectangle on the left, shell notch on the right; going clockwise from the top: 2, 4, (notch), 3, 1, (key). Turned by 90° so that the notch is at the top: 4 upper left, 3 upper right, 2 left, 1 right, key at the bottom.
- Apple, Mac hardware video documentation, Figure 5 "S-video connector" (the 7-pin socket on the computer), image and alt text "Top row left to right is numbered 4, 7, and three. Bottom row left to right is numbered 2, 6, 5, and 1": looked at the image: notch at the top, rectangular key at the bottom. The 4-pin positions are 4 upper left, 3 upper right, 2 left, 1 right — the same as Euresys.
- Kycon drawing KMDGX-xS-BS, KMDGX-4S-BS front view: notch at the top, four contacts upper left/right and left/right, a horizontal rectangular key at the bottom. Angles measured on a render. **Kycon numbers these contacts 8, 6, 5, 3 (its own scheme), not S-Video 4, 3, 2, 1.** Only the geometry is taken from Kycon.

**For the drawing:** a `circular` outline with the notch at 0° and, if the outline can show it, the key rectangle at 180°.

## Sources
1. Euresys, MultiCam 6.7 documentation, Picolo handbook, Connectors, "S-VIDEO Connector", https://documentation.euresys.com/Products/MULTICAM/MULTICAM/Content/MultiCam_6_7_HTML_Documentation/6748.htm, figure minidin4connector.png — "4-pin female receptacle, right-angled PCB-mount, shielded Mini-DIN connector", pin table 1 GND, 2 GND, 3 LUMA (Y), 4 CHROMA (C). Accessed 2026-11-24.
2. Apple, "Video Product-Specific Details" (Hardware Technologies: Video, archived developer documentation), Power Mac G5 section, Figure 5 "S-video connector" (art/050836001705_10.gif) and Table 31 "Pin assignments for the S-video output connector" (1 Analog GND, 2 Analog GND, 3 Video Y (luminance), 4 Video C (chroma), 5 composite video, 6 and 7 unused), https://developer.apple.com/library/archive/documentation/Hardware/Conceptual/HWTech_Video/Articles/Video_implementation.html. Accessed 2026-11-24.
3. Kycon, drawing KMDGX-xS-BS rev C6 (03/05/18), https://cloud.kycon.com/Pub_Eng_Draw/KMDGX-xS-BS%20Series.pdf — 4-position socket front view (geometry only). Accessed 2026-11-24.

## Cross-check
Euresys and Apple agree on all four pins (1 GND, 2 GND, 3 Y, 4 C) and on the positions relative to the notch and key (4 and 3 next to the notch, 2 and 1 at the sides, key opposite the notch). Euresys draws the socket turned by 90°; it is a rotation, not a mirror (the clockwise order 4, 3, 1, 2 is the same in both). Kycon's 4-position socket has the same geometry. No disagreement found.

## Choices
- Catalog item: `data_av.yaml`, group `data`, shape `circular`, `contact: round`, `drawn: socket`, `key: 0`, the four angles and radii above; pins 1 and 2 are both named `GND` with a pin note that no source says which belongs to Y and which to C; no standard text was read.
- `drawn: socket`, `key: 0` (notch at 12 o'clock), like `ps2` and `midi`.
- Y and C typed `signal` (analogue video).
- Grounds not split into "Y ground" / "C ground" because neither source says so.

## Spot-check list
- Pin 3 = Y (luminance) — Euresys table; Apple Table 31.
- Pin 4 = C (chroma) — Euresys table; Apple Table 31.
- Pins 1 and 2 = ground — Euresys table; Apple Table 31.
- Socket face: 4 upper left, 3 upper right, 2 left, 1 right, notch at the top — Apple Figure 5; Euresys figure turned by 90°.

UNVERIFIED: no standard text read (IEC 60933-5 as the S-Video standard is an unverified citation). Which ground pin belongs to Y and which to C. The exact angles come from Kycon's generic mini-DIN drawing.
