# usb3_header — Internal USB 3.x header, 19-pin (2x10) / Interner USB-3.x-Anschluss, 19-polig (2x10)

Kind: free official specification (Intel), cross-checked with two board makers
Pins: 19 listed on 20 positions (pin 20 removed); types: power 2, data 12, ground 4, signal 1

## View
Written for the header on the board, seen from above (pins pointing at the reader), key slot of the shroud on top (`key: top`), pin 1 at the top left (`drawn: socket`). In the `pc_power` convention (`fan_4` and the others: `plug` = cable end, `socket` = board header) this is the `socket` view, so record and group agree. Rows (top to bottom, each left to right):
- top row: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
- bottom row: gap at position 20, then 19, 18, 17, 16, 15, 14, 13, 12, 11

Position 20 (bottom left, under pin 1) has no pin: a gap (`null`) in the bottom row, and the pin list has no entry for 20. Each bottom pin sits under a top pin: 19 under 2, 11 under 10. The cable plug (looking into its holes) is the mirror image with the key still on top; the catalog derives it.

Basis (read):
- Intel USB 3.0 Internal Connector and Cable Specification, figure 3-1 "Header interface dimensions" (page 17, zoomed): top view of the header with leaders "PIN 1" (top row, left end), "PIN 2", "PIN 10" (top row, right end), "PIN 11" (bottom row, right end), "PIN 20 REMOVED" (bottom row, left end); the shroud's long wall on the pin 1 side has the key slot at the centre line. The "RECOMMENDED P.C.B. LAYOUT" repeats PIN 1 top left, PIN 10 top right, PIN 19 bottom (second from left), PIN 11 bottom right.
- Same specification, figure 2-1 "USB3 ICC pin numbering" (page 8, zoomed): 1 at top left to 10 at top right, 19 at bottom (under pin 2) to 11 at bottom right, no pin under pin 1, "Key" on the top edge; Vbus pairs 1/19, GND 4/16 and 7/13 marked.
- MSI JUSB3 drawing (page 34): 1 top left, 10 top right, 20 bottom left, 11 bottom right, the gap of the key slot in the top wall: identical orientation.
- ASRock USB3_5_6 drawing (page 29): the same layout turned 90 degrees clockwise (a column with Vbus at the top next to the empty key corner). Its names are consistent with Intel's numbering position by position.

## Sources
1. Intel, "USB 3.0 Internal Connector and Cable Specification", Revision 1.0, August 20, 2010, https://www.intel.com.au/content/dam/www/public/us/en/documents/technical-specifications/usb3-internal-connector-cable-specification.pdf — table 2-1 "USB3 ICC A Pin Assignment and Description" (page 9), figure 2-1 (page 8), table 2-3 (page 10), table 2-4 current rating (page 16), figure 3-1 (page 17); accessed 2026-11-24
2. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 34, "JUSB3: USB 3.1 Gen1 Connector": 1 Power, 2 USB3_RX_DN, 3 USB3_RX_DP, 4 Ground, 5 USB3_TX_C_DN, 6 USB3_TX_C_DP, 7 Ground, 8 USB2.0-, 9 USB2.0+, 10 NC, 11 USB2.0+, 12 USB2.0-, 13 Ground, 14 USB3_TX_C_DP, 15 USB3_TX_C_DN, 16 Ground, 17 USB3_RX_DP, 18 USB3_RX_DN, 19 Power, 20 No Pin; accessed 2026-11-24
3. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 29 (PDF page 35), "USB 3.2 Gen1 Header (19-pin USB3_5_6)": drawing with names; accessed 2026-11-24

## Cross-check
Intel and MSI agree on every pin (MSI writes "NC" for pin 10 and "No Pin" for 20). ASRock's names agree position by position with Intel (Vbus next to the key corner; second Vbus beside IntA_PB_SSRX-; "Dummy" at the ID position).

Pin-1 mark differs on ASRock: it prints "1" and a square outline at the "Dummy" pin, which is Intel's pin 10 (the corner diagonal to the key corner). The pin names in its drawing still match Intel's positions, so only the numbering mark disagrees. The catalog follows Intel and MSI, and the item note warns that some manuals mark the other end.

Intel table 2-1 names the motherboard side ("USB3 ICC A"); the daughter-card side ("ICC B", table 2-2) numbers the same wires in reverse (table 2-3). Only the board header is this item.

## Choices
- Catalog item: `rows` with two rows of ten (bottom row with a gap at 20), `outline: rect`, `contact: square`, `drawn: socket`, `key: top`, as the record says (the outline centres the slot, which matches the centred slot of the shroud).
- Pin 10 follows Intel's specification for the function: `ID`, over-current protection, type `signal`. The board-maker variants (MSI NC, ASRock dummy) are in the pin note and the function text ("many boards leave it unconnected").
- Intel's signal names (the longest is 13 characters). SuperSpeed and USB 2.0 lines `data`, GND `ground`, Vbus `power`. RX/TX are seen from the host (Intel). Current rating at least 0.5 A per signal pin and 1.5 A per Vbus pin (table 2-4, 30 °C rise).
- No `wire`: the specification assigns wire types (shielded pairs, drain, discrete) but no colours.

## Spot-check list
- Pin 1 = Vbus, pin 19 = Vbus: Intel table 2-1, page 9.
- Pin 4 = GND, pin 7 = GND, pin 13 = GND, pin 16 = GND: Intel table 2-1, page 9.
- Pin 10 = ID (Over Current Protection); MSI NC: Intel table 2-1, page 9; MSI page 34.
- Pin 11 = IntA_P2_D+: Intel table 2-1, page 9; MSI 11 = USB2.0+.
- PIN 1 top left, PIN 20 REMOVED bottom left, PIN 11 bottom right in the top view: Intel figure 3-1, page 17.

UNVERIFIED: none for pins. ASRock's pin-1 mark is a documented disagreement, not an open question.
