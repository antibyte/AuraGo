# dvi_i — DVI-I (dual link) / DVI-I (Dual Link)

Kind: free official specification (DDWG DVI 1.0), cross-checked with two device makers' tables
Connector: DVI combined (analog + digital) connector, 24 contacts in three rows of eight plus the analog block C1–C5 (29 contacts) and the shell. Drawn side: the receptacle on the graphics card or monitor (`drawn: socket`).
Pins: 29; types: data 16, ground 6, power 1, signal 6

## Pins (combined connector = DVI-I, DDWG Table 5-2)

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | TMDS Data2- | TMDS data channel 2 (red), negative | TMDS-Datenkanal 2 (Rot), negativ | data |
| 2 | TMDS Data2+ | TMDS data channel 2 (red), positive | TMDS-Datenkanal 2 (Rot), positiv | data |
| 3 | TMDS Data2/4 Shield | Shield of TMDS data channels 2 and 4 | Abschirmung der TMDS-Datenkanäle 2 und 4 | ground |
| 4 | TMDS Data4- | TMDS data channel 4 (second link, green), negative | TMDS-Datenkanal 4 (zweiter Link, Grün), negativ | data |
| 5 | TMDS Data4+ | TMDS data channel 4 (second link, green), positive | TMDS-Datenkanal 4 (zweiter Link, Grün), positiv | data |
| 6 | DDC Clock | DDC clock (I²C) | DDC-Takt (I²C) | data |
| 7 | DDC Data | DDC data (I²C) | DDC-Daten (I²C) | data |
| 8 | Analog Vertical Sync | Analog vertical sync | Analoger Vertikal-Sync | signal |
| 9 | TMDS Data1- | TMDS data channel 1 (green), negative | TMDS-Datenkanal 1 (Grün), negativ | data |
| 10 | TMDS Data1+ | TMDS data channel 1 (green), positive | TMDS-Datenkanal 1 (Grün), positiv | data |
| 11 | TMDS Data1/3 Shield | Shield of TMDS data channels 1 and 3 | Abschirmung der TMDS-Datenkanäle 1 und 3 | ground |
| 12 | TMDS Data3- | TMDS data channel 3 (second link, blue), negative | TMDS-Datenkanal 3 (zweiter Link, Blau), negativ | data |
| 13 | TMDS Data3+ | TMDS data channel 3 (second link, blue), positive | TMDS-Datenkanal 3 (zweiter Link, Blau), positiv | data |
| 14 | +5V Power | +5 V from the host for the monitor's DDC/EDID, at least 55 mA available | +5 V vom Host für DDC/EDID des Monitors, mindestens 55 mA verfügbar | power |
| 15 | Ground | Ground, return for +5 V, HSync and VSync | Masse, Rückleitung für +5 V, HSync und VSync | ground |
| 16 | Hot Plug Detect | Hot plug detect, driven by the monitor (above 2.4 V when EDID is readable) | Hot-Plug-Erkennung, vom Monitor getrieben (über 2,4 V, wenn EDID lesbar ist) | signal |
| 17 | TMDS Data0- | TMDS data channel 0 (blue), negative | TMDS-Datenkanal 0 (Blau), negativ | data |
| 18 | TMDS Data0+ | TMDS data channel 0 (blue), positive | TMDS-Datenkanal 0 (Blau), positiv | data |
| 19 | TMDS Data0/5 Shield | Shield of TMDS data channels 0 and 5 | Abschirmung der TMDS-Datenkanäle 0 und 5 | ground |
| 20 | TMDS Data5- | TMDS data channel 5 (second link, red), negative | TMDS-Datenkanal 5 (zweiter Link, Rot), negativ | data |
| 21 | TMDS Data5+ | TMDS data channel 5 (second link, red), positive | TMDS-Datenkanal 5 (zweiter Link, Rot), positiv | data |
| 22 | TMDS Clock Shield | Shield of the TMDS clock | Abschirmung des TMDS-Takts | ground |
| 23 | TMDS Clock+ | TMDS clock, positive | TMDS-Takt, positiv | data |
| 24 | TMDS Clock- | TMDS clock, negative | TMDS-Takt, negativ | data |
| C1 | Analog Red | Analog red video | Analoges Video Rot | signal |
| C2 | Analog Green | Analog green video | Analoges Video Grün | signal |
| C3 | Analog Blue | Analog blue video | Analoges Video Blau | signal |
| C4 | Analog HSync | Analog horizontal sync | Analoger Horizontal-Sync | signal |
| C5 | Analog Ground | Analog ground, return for red, green and blue | Analoge Masse, Rückleitung für Rot, Grün und Blau | ground |

Shell: connector shell, mates first (Table 5-3). Not a numbered contact; for `notes`.

Colour words (red/green/blue per channel) come from DDWG Figure 3-2/3-3 (channel 0 = BLU, 1 = GRN, 2 = RED; channels 3, 4, 5 = BLU2, GRN2, RED2 of the second link). They describe the pixel data each encoder carries, not a wire colour; keep or drop them from the functions as you like, they are verified.

### DVI-D (digital-only connector, DDWG Table 5-1)
Same 24 contacts, same numbers and names, with two differences:
- Pin 8 is **No Connect** (DDWG: "No Connect").
- Pin 15 is "Ground (for +5V)".
- No C1–C4 and no C5 contact. The digital-only receptacle (Figure 5-1) has a single flat slot where the combined one has the cross; the digital plug (Figure 5-5) has a flat blade there with no C number (a key, not a listed contact).

### Single link vs dual link
DDWG 3.1.5 and 3.2.1: "A dual-link transmitter incorporates an additional three data channels"; Figure 3-3 maps the second link to channels 3, 4, 5. So a **single link** uses only data channels 0, 1, 2 and the clock:
- Used by DVI-D single link: 1, 2, 3, 6, 7, 9, 10, 11, 14, 15, 16, 17, 18, 19, 22, 23, 24 (pin 8 no connect).
- Second link only: 4, 5 (Data4), 12, 13 (Data3), 20, 21 (Data5). The shields 3, 11, 19 are shared between a first-link and a second-link pair and stay in use.
- DVI-I single link: the single-link set above plus 8 and C1–C5.
Euresys marks pins 4, 5, 12, 13, 20, 21 "(link 2)", which agrees. The DVI spec itself does not define a "single link connector" that omits those six contacts; cables that leave them out are a market practice that I did not verify, so state only "unused on single link".

## View
Written for the **receptacle (on the graphics card or monitor), looking at its mating face**, wide side of the shell at the top, the two chamfered corners at the bottom (`drawn: socket`):
- Top row: 1 2 3 4 5 6 7 8, left to right.
- Middle row: 9 … 16, left to right, directly below.
- Bottom row: 17 … 24, left to right.
- Analog block to the **right** of pins 8/16/24: C1 upper left, C2 upper right, C3 lower left, C4 lower right, C5 the cross-shaped ground blade in the middle (flat horizontal blade with the cross bar on the receptacle).

The plug (cable end), looking at its mating face, is the mirror image: 8 … 1 in the top row (pin 1 at the upper right), 16 … 9, 24 … 17, analog block on the **left** with C2 upper left, C1 upper right, C4 lower left, C3 lower right, C5 a flat horizontal blade between them.

Basis:
- DDWG DVI 1.0, Figure 5-3 "Combined Receptacle Connector Mating Interface Dimensions" (page 46): rendered and looked at. Callouts "PIN 1" at the upper-left cell and "PIN 24" at the lower-right cell of the 3 × 8 field; row-end labels 1, 9, 17 on the left and 8, 16, 24 on the right; C1, C2 above, C3, C4, C5 below the cross, which is right of the field. Mounting flange, threads and board tails visible (a front view: the side view next to it is a right-angle part with the mating face to the left), PCB tails at the bottom, chamfered shell corners at the bottom. Figure 5-1 (digital-only receptacle) has the same pin-1 position with a single slot instead of the cross.
- DDWG Figure 5-4 "Recommended Combined Receptacle Connector PCB Layout": CKT 1 upper left, CKT 24 lower right, C1/C2/C3/C4/C5 to the right, PCB edge at the bottom. Seen from above with the board edge toward the viewer, left and right are the same as on the front face, so this agrees with reading Figure 5-3 as the front.
- DDWG Figure 5-6 "Analog Plug Connector Mating Interface Dimensions" (page 47): "PIN 1" callout at the upper right, top row labelled 8 7 6 5 4 3 2 1, rows 16 … 9 and 24 … 17, block on the left labelled C2 C1 / C5 / C4 C3. This is exactly the mirror of Figure 5-3, as the two faces require. Figure 5-5 (digital plug) the same without C1–C4.
- Apple, Xserve G4 developer note, Figure 3-6 "DVI-I connector" (a computer's port): 1 … 8, 9 … 16, 17 … 24 left to right, C1 C2 / C3 C4 right of the field, C5 below the cross, chamfered corners at the bottom. No view is stated, but it is the port on the computer and the same as DDWG Figure 5-3.

**For the drawing:** the view holds only if the `dvi` outline has its wide side at the top (chamfered corners at the bottom) and the analog block on the right in the socket view.

## Sources
1. Digital Display Working Group (DDWG), "Digital Visual Interface DVI", Revision 1.0, 02 April 1999, copy at https://www.monitortests.com/download/pdf/dvi_10.pdf (76 pages) — section 5.2.1 Table 5-1 "Digital-Only Connector Pin Assignments" and Table 5-2 "Combined Analog and Digital Connector Pin Assignments" (page 43–44), Table 5-3 "Mating Contact Sequence", Figures 5-1, 5-3, 5-4, 5-5, 5-6 (pages 45–47), sections 2.2.6 (DDC +5 V, 55 mA), 2.2.9.2 (HPD > +2.4 V), 3.1.5/3.2.1 and Figure 3-3 (dual link channel map). Accessed 2026-11-24. (A Molex-hosted copy at ttiinc.com returned HTTP 403.)
2. Apple, "Xserve G4 Developer Note", Input and Output Devices, DVI connector: Table 3-9 "Main signals on the DVI-I connector", Table 3-10 "MicroCross signals on the DVI-I connector", Figure 3-6 "DVI-I connector", https://developer.apple.com/library/archive/documentation/Hardware/Developer_Notes/Servers/XserveG4/3Input-Output/chapter_4_section_8.html and figure https://developer.apple.com/library/archive/documentation/Hardware/Developer_Notes/Servers/XserveG4/art/q28_12.gif. Accessed 2026-11-24.
3. Euresys, Picolo HD documentation, "DVI-I Input connector" (DVI-I female socket on a capture card), https://documentation.euresys.com/Products/Picolo_HD/Picolo_HD/en-us/Content/03_Hardware/meca/connectors/DVI_I_Input.htm — pin/signal/usage table incl. C1–C5 and shell, link-2 marks. Accessed 2026-11-24.

## Cross-check
Pins 1–24 and C1–C5: DDWG Table 5-2, Apple Tables 3-9/3-10 and Euresys agree pin by pin on every signal (names differ only in spelling: "DDC Clock" vs SCL, "Analog Common Ground Return" vs AGND).

Differences, none a conflict:
- Pin 14: Euresys "-" (empty). Euresys is a capture input (a sink), which does not supply the +5 V; DDWG and Apple: +5V Power. Use the spec.
- Pin 15: DDWG combined "Ground (return for +5V, HSync, and VSync)", digital-only "Ground (for +5V)", Apple "Ground for +5V Power", Euresys "Ground".
- Pin 8: DDWG digital-only "No Connect" vs combined "Analog Vertical Sync" — the DVI-D/DVI-I difference, not a conflict.
- View: DDWG Figure 5-3 and Apple Figure 3-6 agree; DDWG Figure 5-6 is the mirror (plug).

## Choices
- Catalog item: `data_av.yaml`, outline `dvi`, `contact: round`, `drawn: socket`, rows 1..8 / 9..16 / 17..24 with a gap (`null`) before the analog block (`C1 C2` / `C5` / `C3 C4`); 29 pins in one list; DVI-D (pin 8 no connect, no C1–C5) and the single-link rule are in `notes`, the second-link pins carry a pin note.
- Item = DVI-I, because the dual-link combined connector contains every contact; DVI-D (pin 8 no connect, no C1–C5) goes into `notes`, or as a variant if the app wants one (variant `dvi_d` with 24 pins; pin 8 `nc`).
- Second-link pins keep type `data`; "unused on single link" goes into their `notes`.
- Shields and C5 are `ground`; HPD and the analog lines `signal`; DDC lines `data` (as `hdmi_a` and `vga`).
- Names shortened from "T.M.D.S." to "TMDS" as in `hdmi_a`; all names ≤ 24 characters.
- `drawn: socket`: the part on the device and the part with the spec's numbered front view.

## Spot-check list
- Pin 1 = T.M.D.S. Data2- — DDWG Table 5-2, page 44; Apple Table 3-9.
- Pin 8 = Analog Vertical Sync (combined) / No Connect (digital-only) — DDWG Tables 5-2 and 5-1.
- Pin 14 = +5V Power, pin 16 = Hot Plug Detect — DDWG Table 5-2; Apple Table 3-9.
- Pins 23/24 = T.M.D.S. Clock+ / Clock- — DDWG Table 5-2; Apple Table 3-9; Euresys.
- C1 = Analog Red, C4 = Analog Horizontal Sync, C5 = Analog Ground — DDWG Table 5-2; Apple Table 3-10.
- Pin 1 at the upper left of the receptacle face, analog block right — DDWG Figure 5-3 ("PIN 1" callout); Apple Figure 3-6.
- Pin 1 at the upper right of the plug face, C2 C1 / C4 C3 — DDWG Figure 5-6.

UNVERIFIED: the DVI-D single-link cable or plug that physically omits contacts 4, 5, 12, 13, 20, 21 (not in DDWG 1.0; not stated). Later DVI revisions (1.0 errata) were not read. The colour words for the second-link channels (Data3 blue, Data4 green, Data5 red) rest on DDWG Figure 3-3 only (Euresys agrees).
