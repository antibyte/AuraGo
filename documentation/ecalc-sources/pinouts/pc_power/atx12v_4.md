# atx12v_4 — ATX12V CPU power, 4-pin / ATX12V-CPU-Strom, 4-polig

Kind: free official specification
Pins: 4; types: ground 2, power 2

## View
Written for the cable plug from the power supply, looking at its mating face (`drawn: plug`), latch on top: the contact holes face the reader.

Rows (top to bottom, each left to right):
- top row (latch side): 3, 4
- bottom row: 1, 2

The header on the board (`socket`) is the mirror image x to W - x with the latch staying on top: 4, 3 over 2, 1.

Basis (read):
- Intel 336521 Rev. 2.1a, figure 5-2 "Connectors (Pin-side view, not to Scale)", the drawing "+12V2 DC Connector" (rendered page 54 and zoomed): numbers 1 and 3 above the two columns; COM beside the left column (pins 1 and 2), +12V2 DC beside the right column (pins 3 and 4); the latch is drawn on the right, next to the +12V2 column. So in the figure: 1 top left, 3 top right, 2 bottom left, 4 bottom right, latch at the right.
- "Pin-side view" is the mating face of the cable plug, the basis already used for `atx_24` and `eps_8` (CEM 4.0 figures 58/59 for the Mini-Fit family, NZXT, smps.us).
- Turning the figure 90 degrees counter-clockwise puts the latch on top: the right column (3, 4) becomes the top row read 3, 4 and the left column (1, 2) the bottom row read 1, 2. Same handedness as the `eps_8` plug (5 6 7 8 over 1 2 3 4), as it must be: ASRock says a 4-pin plug goes onto the 8-pin header "along Pin 1 and Pin 5", so 4-pin pins 1, 2, 3, 4 meet 8-pin pins 1, 2, 5, 6, which keeps the +12 V row at the latch in both.

## Sources
1. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-8 "+12 V Power 4 pin Connector Pin-out" (page 67): 1 COM Black, 2 COM Black, 3 +12V2 DC Yellow, 4 +12V2 DC Yellow, "Note: 18 AWG wire."; section 5.2.2.5 (page 66): housing Molex 0039012040, contact Molex 44476-1112, mating board connector Molex 39-29-9042; figure 5-2 (page 54) view and latch side; accessed 2026-11-24
2. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 31 (PDF page 37), "8-pin ATX12V1": "To use a 4-pin ATX power supply, please plug it along Pin 1 and Pin 5."; accessed 2026-11-24
3. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 33, CPU_PWR1 table: 1 to 4 Ground, 5 to 8 +12V (the 8-pin; for the relation only); accessed 2026-11-24
4. Record `eps_8.md` (view basis of the Mini-Fit family: Intel figure 5-2, PCI-SIG CEM 4.0 figures 58/59, NZXT, smps.us, ASRock/Gigabyte header drawings).

## Cross-check
Intel's table 5-8 and the labels of figure 5-2 agree (COM on 1 and 2, +12V2 on 3 and 4). ASRock's "along Pin 1 and Pin 5" and MSI's 8-pin table (1 to 4 ground, 5 to 8 +12 V) are consistent with it: the 4-pin plug covers 8-pin positions 1, 2, 5, 6, so ground meets ground and +12 V meets +12 V.

Not read: a board maker's own 4-pin ATX12V header drawing (none of the manuals opened has a 4-pin header) and the Molex drawings of 39-01-2040 / 39-29-9042.

## Choices
- Catalog item: `rows` with the latch row 3, 4 over 1, 2, `outline: minifit`, `contact: square`, `drawn: plug`, as `eps_8`. `plug` = cable end, `socket` = board header.
- Names `COM` and `+12V2` as Intel writes them (without "DC", as in `eps_8`). Wire colours from Intel table 5-8 (black, yellow).
- The note carries the relation to the 8-pin header (along pins 1 and 5, meeting 1, 2, 5, 6) and the 18 AWG wire.

## Spot-check list
- Pin 1 = COM, black: Intel 336521 table 5-8, page 67.
- Pin 3 = +12V2 DC, yellow: Intel 336521 table 5-8, page 67.
- Pin 4 = +12V2 DC, yellow: Intel 336521 table 5-8, page 67.
- Latch beside the +12V2 column, 1 and 3 on the same side: Intel 336521 figure 5-2, page 54.
- 4-pin plug on an 8-pin header "along Pin 1 and Pin 5": ASRock B450 Steel Legend manual, page 31.

UNVERIFIED: none for pins; the view rests on Intel figure 5-2 plus the family convention of `eps_8` (no maker drawing of a 4-pin board header was read).
