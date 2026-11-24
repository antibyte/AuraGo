# rgb_header_12v — RGB LED header, 4-pin 12 V / RGB-LED-Anschluss, 4-polig 12 V

Kind: two board makers' manuals (no free specification exists; the header is a maker convention)
Pins: 4; types: power 1, signal 3

## View
Written for the header on the board, seen from above (pins pointing at the reader), pin 1 at the left (`drawn: socket`). In the `pc_power` convention (`fan_4` and the others: `plug` = cable end, `socket` = board header) this is the `socket` view. One row, left to right: 1, 2, 3, 4. The cable plug (looking into its holes) is the mirror image: 4, 3, 2, 1; the catalog derives it.

Basis (read):
- ASRock RGB_LED1 drawing (B450 Steel Legend manual, page 32 and again page 52): "1" at the left end with the square pin-1 outline, labels under the pins "12V G R B" from left to right.
- MSI JRGB drawing (B450 TOMAHAWK page 38; MAG B550 TOMAHAWK MAX WIFI page 27): square pin-1 pad at the left with "1" below it; table 1 +12V, 2 G, 3 R, 4 B.
- Both drawings are board top views.

## Sources
1. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 32 (PDF page 38), "RGB LED Header (4-pin RGB_LED1)" drawing "12V G R B"; page 52 (PDF page 58), section 3.4: "standard 5050 RGB LED strip (12V/G/R/B), with a maximum power rating of 3A (12V) and length within 2 meters"; accessed 2026-11-24
2. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 38, "JRGB1~2: RGB LED connectors": table 1 +12V, 2 G, 3 R, 4 B; "up to 2 meters continuous 5050 RGB LED strips (12V/G/R/B) with the maximum power rating of 3A (12V)"; accessed 2026-11-24
3. MSI, MAG B550 TOMAHAWK MAX WIFI user guide, https://download-2.msi.com/archive/mnu_exe/mb/MAGB550TOMAHAWKMAXWIFI.pdf — page 27 (PDF page 39), "JRGB1~2": same table and rating; page 28: the 5 V / 12 V caution; accessed 2026-11-24

## Cross-check
ASRock and MSI (two independent makers) agree pin by pin: 1 +12V (ASRock "12V"), 2 G, 3 R, 4 B, pin 1 at the left of the drawn header; both give 3 A at 12 V and 2 m. MSI's caution: the JRGB (12 V) and JRAINBOW (5 V addressable) connectors "provide different voltages, and connecting the 5V LED strip to the JRGB connector will result in damage to the LED strip."

Not read: an Asus or Gigabyte manual (Gigabyte LED_C ratings appeared only in search snippets and do not count). How the board switches the channels (low side or high side) is not stated, so the functions only name the colours.

## Choices
- Catalog item: one `rows` row of four, `outline: rect`, `contact: square`, `drawn: socket`, no `key`. No standard exists, and the doc text says the order is a maker convention.
- `+12V` as MSI writes it (ASRock "12V"). G, R, B `signal` (PWM-controlled LED channels), `power` for +12V; this header has no ground pin.
- The order G before R is in the item note, as is the 5 V strip warning.
- No `wire`: no source names cable colours.

## Spot-check list
- Pin 1 = +12V: MSI B450 TOMAHAWK page 38; ASRock page 32 ("12V" under pin 1).
- Pin 2 = G: MSI page 38; ASRock page 32.
- Pin 3 = R: MSI page 38; ASRock page 32.
- Pin 4 = B: MSI page 38; ASRock page 32.
- 3 A at 12 V, within 2 m: ASRock page 52; MSI page 38.

UNVERIFIED: none for pins; whether the channels are switched to ground is not stated and is left out.
