# front_panel — Front panel header (power switch, reset, LEDs), 2x5 / Frontpanel-Anschluss (Ein-Taster, Reset, LEDs), 2x5

Kind: free official specification (Intel), cross-checked with two board makers
Pins: 9 listed on 10 positions; types: signal 6 (1, 2, 3, 4, 6, 7), ground 2 (5, 8), nc 1 (9)

## View
Written for the header on the board, seen from above (pins pointing at the reader), pin 1 at the bottom left, odd pins in the bottom row (`drawn: socket`). In the `pc_power` convention, set by `fan_4` and the other items (`plug` = cable end, `socket` = board header), this is the `socket` view, so the record and the group agree and nothing is mirrored for the catalog. Rows (top to bottom, each left to right):
- top row: 2, 4, 6, 8, and a gap at position 10
- bottom row: 1, 3, 5, 7, 9

Position 10 has no pin (key). It is a gap (`null`) in the top row, not a pin, and the pin list has no entry for 10. The cable plug (looking into its holes) is the mirror image: gap, 8, 6, 4, 2 over 9, 7, 5, 3, 1; the catalog derives it.

Basis (read):
- Intel FPIO 2004 (A29286-004) figure 1 "Front Panel Switch/LED and IR Headers (Top View)" (page 17): 9 and 1 on the top row ends, 10 and 2 on the bottom row ends: pin 1 top right, odd row on top, pin 10 bottom left drawn empty. The same drawing is figure 2-1 of the 2018 revision 1.1 (page 15; pin 1 square pad top right, pin 9 drawn hollow, pin 10 position empty).
- Turning Intel's top view by 180 degrees gives exactly the drawings of MSI (JFP1: 2 ... 10 on top, 1 ... 9 below, square pin 1 bottom left) and ASRock (PANEL1: 1 at the bottom left, square pad). A rotation, not a mirror, so all three are the same top view.
- Pin positions by function also agree: Intel item C = pins 1 and 3 (HDD LED), D = 2 and 4 (power LED), B = 5 and 7 (reset), E = 6 and 8 (power switch).

## Sources
1. Intel, "Front Panel I/O Connectivity Design Guide", document 600569, Revision 1.1, July 2018, https://cdrdv2-public.intel.com/600569/600569-fpio-dg-rev1p1.pdf — section 2.2.2 (pin pairs per function), table 2-4 "Switch/LED Front Panel Electrical Connection" (page 16), figure 2-1 (page 15), tables 2-1/2-2 LED states; accessed 2026-11-24
2. Intel, "Front Panel I/O Connectivity Design Guide", Version 1.2, July 2004, order number A29286-004, copy at https://ftpmirror.infania.net/sites/intel/Motherboards/Workstation/Intel%20OR840/Docs/datasheets/A29286-004_frontPanelIO.pdf — table 5 (page 18) with notes 1 to 3, figure 1 "(Top View)" (page 17); accessed 2026-11-24
3. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 32, "JFP1, JFP2: Front Panel Connectors": 1 HDD LED +, 2 Power LED +, 3 HDD LED -, 4 Power LED -, 5 Reset Switch, 6 Power Switch, 7 Reset Switch, 8 Power Switch, 9 Reserved, 10 No Pin; accessed 2026-11-24
4. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 28 (PDF page 34), "System Panel Header (9-pin PANEL1)": drawing with names, pin 1 marked; accessed 2026-11-24

## Cross-check
Pin by pin, Intel 2018 / Intel 2004 / MSI / ASRock:
- 1: HD_LED_P / HD_LED_P / HDD LED + / HDLED+ — agree
- 2: FP PWR/SLP / FP PWR/SLP / Power LED + / PLED+ — agree
- 3: HD_LED_N / HD_LED_N / HDD LED - / HDLED- — agree
- 4: FP PWR/SLP / FP PWR/SLP / Power LED - / PLED- — agree
- 5: RST_SW_N (100 Ω pull-down to GND) / RST_SW_N "tied to GND" / Reset Switch / GND — agree
- 6: PWR_SW_P (pull-up) / PWR_SW_P (pull-up) / Power Switch / PWRBTN# — agree
- 7: RST_SW_P (pull-up) / RST_SW_P (pull-up) / Reset Switch / RESET# — agree
- 8: PWR_SW_N (100 Ω pull-down to GND) / PWR_SW_N "tied to GND" / Power Switch / GND — agree
- 9: RSVD_DNU / RSVD_DNU (+5 V allowed via 0 Ω for legacy 2x8) / Reserved / GND — differs
- 10: not listed / not listed / No Pin / no pin drawn — agree

## Choices
- Catalog item: `rows` with two rows of five, `outline: rect`, `contact: square`, `drawn: socket`, no `key`; position 10 is a gap, the other nine are pins. Names in the makers' style (`HDD_LED+`, `PLED+`, `PWR_SW`, `RESET`) because Intel gives pins 2 and 4 the same name and its `_P`/`_N` names hide the LED polarity; Intel's names are in the item note.
- Pin 9 follows Intel's specification for the function: reserved, do not use, `nc`. The board-maker variants (ASRock draws ground, Intel 2004 allows +5 V through 0 Ω for a legacy 2x8 connector, MSI reserved) are in the pin note. Pin 10 is a gap.
- Pins 5 and 8 `ground`, the other side of each switch `signal` (the pulled-up side is what the board senses). Power switch "at least 50 ms" is Intel 2.2.2.4.
- LED polarity of pins 2/4 comes from the board makers (Intel defines a dual-colour LED on them without plus and minus); the pin note says so. Pull-up values differ between Intel revisions (10 kΩ to +5 V in 2018, 2.2 kΩ to +3.3 V standby in 2004) and are board detail, left out.
- No `wire`: no source names cable colours.

## Spot-check list
- Pin 1 = HD_LED_P, 330 Ω pull-up to +5 V: Intel 600569 table 2-4, page 16.
- Pin 6 = PWR_SW_P, pin 8 = PWR_SW_N: Intel 600569 table 2-4, page 16; figure 2-1, page 15.
- Pin 2 = Power LED +, pin 4 = Power LED -: MSI B450 TOMAHAWK, page 32.
- Pin 9 = GND on ASRock, Reserved on MSI, RSVD_DNU on Intel: ASRock page 28; MSI page 32; Intel 600569 page 16.
- Pin 1 top right in Intel's top view: Intel A29286-004 figure 1, page 17.

UNVERIFIED: none for positions and functions; the LED polarity of pins 2/4 comes from the board makers.
