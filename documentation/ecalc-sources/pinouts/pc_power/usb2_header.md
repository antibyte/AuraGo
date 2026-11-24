# usb2_header — Internal USB 2.0 header, 2x5 / Interner USB-2.0-Anschluss, 2x5

Kind: free official specification (Intel), cross-checked with two board makers
Pins: 9 listed on 10 positions; types: power 2, data 4, ground 2, nc 1

## View
Written for the header on the board, seen from above (pins pointing at the reader), pin 1 at the bottom left, odd pins in the bottom row (`drawn: socket`). In the `pc_power` convention (`fan_4` and the others: `plug` = cable end, `socket` = board header) this is the `socket` view, so record and group agree. Rows (top to bottom, each left to right):
- top row: 2, 4, 6, 8, 10
- bottom row: 1, 3, 5, 7, and a gap at position 9

Position 9 has no pin. It is a gap (`null`) in the bottom row and the pin list has no entry for 9. The cable plug (looking into its holes; the hole at 9 is usually blocked) is the mirror image: 10, 8, 6, 4, 2 over gap, 7, 5, 3, 1; the catalog derives it.

Basis (read):
- Intel 2018 figure 3-1 "Front Panel USB Connector" (page 20): 9 and 1 at the ends of the top row, 10 and 2 at the bottom row; pin 1 square pad top right; top row has no pin at 9. Intel 2004 figure 7 "Dual Port USB Header (Top View)" (page 28): pin 1 square top right, odd row on top.
- MSI JUSB1~2 drawing (page 34): 2 ... 10 above, 1 ... 9 below, pin 1 square bottom left, position 9 empty. ASRock USB_1_2 drawing (page 29): pin 1 marked bottom left. Both are Intel's top view turned by 180 degrees.

## Sources
1. Intel, "Front Panel I/O Connectivity Design Guide", document 600569, Revision 1.1, July 2018, https://cdrdv2-public.intel.com/600569/600569-fpio-dg-rev1p1.pdf — section 3.2, figure 3-1 and table 3-1 "Front Panel USB Connector" (page 20); accessed 2026-11-24
2. Intel, "Front Panel I/O Connectivity Design Guide", Version 1.2, July 2004, A29286-004, copy at https://ftpmirror.infania.net/sites/intel/Motherboards/Workstation/Intel%20OR840/Docs/datasheets/A29286-004_frontPanelIO.pdf — section 3.2.3, figure 7 (Top View) and table 8 (page 28); accessed 2026-11-24
3. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 34, "JUSB1~2: USB 2.0 Connectors": 1 VCC, 2 VCC, 3 USB0-, 4 USB1-, 5 USB0+, 6 USB1+, 7 Ground, 8 Ground, 9 No Pin, 10 NC; accessed 2026-11-24
4. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 29 (PDF page 35), "USB 2.0 Headers (9-pin USB_1_2)": drawing with names; accessed 2026-11-24

## Cross-check
Intel 2018 / Intel 2004 / MSI / ASRock:
- 1, 2: VREG_FP_USBPWR0 / VREG_FP_USBPWR / VCC / USB_PWR — agree
- 3: USB_FP_P0- / USB_FP_P0- / USB0- / P- — agree
- 4: USB_FP_P1- / USB_FP_P1- / USB1- / P- — agree
- 5: USB_FP_P0+ / USB_FP_P0+ / USB0+ / P+ — agree
- 6: USB_FP_P1+ / USB_FP_P1+ / USB1+ / P+ — agree
- 7, 8: Ground in all four — agree
- 9: Key / Key / No Pin / no pin drawn — agree
- 10: USB_FP_OC0 / Key / NC / DUMMY — differs

## Choices
- Catalog item: `rows` with two rows of five, `outline: rect`, `contact: square`, `drawn: socket`, no `key`; position 9 is a gap, the pin list has nine entries.
- Pin 10: Intel 2018 says USB_FP_OC0 (a front panel over-current signal), Intel 2004 says key, MSI says NC, ASRock says dummy. The function follows the safe reading for wiring, `nc` "leave free", and the pin note names the Intel 2018 signal and the maker variants.
- Names `+5V`, `D0-`, `D1-`, `D0+`, `D1+` (short, language neutral); Intel's VREG_FP_USBPWR0 and USB_FP_P0- are in the vector refs. Power note: "+5 V Dual" is Intel 2004 table 8 note; the fuse and the "not for devices outside the case" caution are Intel 2.1.1.
- No `wire`: no source names colours.

## Spot-check list
- Pin 1 = VREG_FP_USBPWR0, front panel USB power: Intel 600569 table 3-1, page 20.
- Pin 3 = USB_FP_P0-, pin 5 = USB_FP_P0+: Intel 600569 table 3-1, page 20.
- Pin 9 = Key: Intel 600569 table 3-1, page 20; MSI page 34 "No Pin".
- Pin 10 = NC on MSI, USB_FP_OC0 on Intel 2018: MSI page 34; Intel 600569 page 20.
- Pin 1 at the top right of the top view: Intel A29286-004 figure 7, page 28.

UNVERIFIED: none.
