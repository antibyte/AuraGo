# argb_header_5v — Addressable RGB header, 3-pin 5 V (4 positions) / ARGB-Anschluss, 3-polig 5 V (4 Positionen)

Kind: two board makers' manuals (no free specification exists; the header is a maker convention)
Pins: 3 listed on 4 positions; types: power 1, data 1, ground 1

## View
Written for the header on the board, seen from above (pins pointing at the reader), pin 1 at the left (`drawn: socket`). In the `pc_power` convention (`fan_4` and the others: `plug` = cable end, `socket` = board header) this is the `socket` view. One row, left to right: 1, 2, gap, 4. Position 3 has no pin: a gap (`null`) in the row and no entry in the pin list. The cable plug (looking into its holes) is the mirror image: 4, gap, 2, 1; the catalog derives it.

Basis (read):
- ASRock ADDR_LED1 drawing (B450 Steel Legend manual, page 32 and page 53): "1" at the left end with the square pin-1 outline; pins at positions 1, 2 and 4, position 3 drawn empty; labels VOUT (1), DO_ADDR (2), GND (4).
- MSI JRAINBOW drawing (MAG B550 TOMAHAWK MAX WIFI user guide, page 28): "1" above the left end, three square pads with a gap at position 3; table 1 +5V, 2 Data, 3 No Pin, 4 Ground.
- Both drawings are board top views.

## Sources
1. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 32 (PDF page 38), "Addressable LED Header (3-pin ADDR_LED1)": VOUT, DO_ADDR, (no pin), GND; page 53 (PDF page 59), "Connecting the Addressable RGB LED Strip": rating note; accessed 2026-11-24
2. MSI, MAG B550 TOMAHAWK MAX WIFI user guide, https://download-2.msi.com/archive/mnu_exe/mb/MAGB550TOMAHAWKMAXWIFI.pdf — page 28 (PDF page 40), "JRAINBOW1~2: Addressable RGB LED connectors": table 1 +5V, 2 Data, 3 No Pin, 4 Ground; caution and rating; accessed 2026-11-24
3. MSI, MAG B550 TOMAHAWK quick start manual, copy at https://manualslib.com/manual/2272645/Msi-Mag-B550-Tomahawk.html?page=38 — same JRAINBOW table and rating (text extraction only, a repeat, not a separate maker); accessed 2026-11-24

## Cross-check
ASRock and MSI agree pin by pin: 1 VOUT / +5V, 2 DO_ADDR / Data, 3 no pin / No Pin, 4 GND / Ground. Both give 3 A at 5 V. ASRock: "WS2812B addressable RGB LED strip (5V/Data/GND), with a maximum power rating of 3A (5V) and length within 2 meters". MSI: "up to 75 LEDs WS2812B ... with the maximum power rating of 3A (5V). In the case of 20% brightness, the connector supports up to 200 LEDs." MSI caution: the 12 V RGB header and this 5 V header provide different voltages, and connecting the 5 V strip to the JRGB connector damages the strip.

Not read: Asus (ADD_GEN2) and Gigabyte (D_LED) manuals; their tables appeared only in search snippets.

## Choices
- Catalog item: one `rows` row of four positions with a gap at 3, `outline: rect`, `contact: square`, `drawn: socket`, no `key` (the missing pin itself is the key, and the gap shows it). The doc text says there is no standard.
- Names `+5V`, `Data`, `GND` (MSI's words; ASRock VOUT, DO_ADDR, noted in the vector refs). `data` for the data line (a serial LED protocol), `power`, `ground`.
- The plug has three wires in a 4-position housing with one hole blocked (the item note says so). That is a reading of the drawings, not a statement of either maker.
- No `wire`: no source names cable colours.

## Spot-check list
- Pin 1 = +5V (ASRock VOUT): MSI MAG B550 TOMAHAWK MAX WIFI page 28; ASRock page 32.
- Pin 2 = Data (ASRock DO_ADDR): MSI page 28; ASRock page 32.
- Position 3 = No Pin: MSI page 28; ASRock page 32 (empty position).
- Pin 4 = Ground: MSI page 28; ASRock page 32.
- 3 A at 5 V: ASRock page 53; MSI page 28.

UNVERIFIED: none.
