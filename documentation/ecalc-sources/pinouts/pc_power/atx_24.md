# atx_24 — ATX main power, 24-pin / ATX-Hauptstrom, 24-polig

Kind: free official specification
Pins: 24; types: power 13, ground 8, signal 2, nc 1

## View
Written for the cable plug from the power supply, looking at its mating face (`drawn: plug`), latch on top: the contact holes face the reader.

Basis, four sources that fit together:
- PCI-SIG CEM Rev. 4.0 names the two sides. Figure 58, "2 x 4 Auxiliary Power Connector Plug Side Pin-out", shows the latch on top, PIN 5 6 7 8 in the latch row and PIN 1 2 3 4 below, each left to right. Figure 59, "Receptacle Side Pin-out", shows the mirror image: PIN 8 7 6 5 over PIN 4 3 2 1. Figure 60 shows the 2 x 3 plug the same way: PIN 4 5 6 over PIN 1 2 3.
- Intel ATX Rev. 2.1a, figure 5-2, is captioned "Connectors (Pin-side view, not to Scale)" and shows the harness connectors with pin 1 at top left, pin 13 at top right and the latch on the right beside pins 13 to 24 (the 4-pin +12V2 connector likewise: 1 and 3 on top, latch on the +12 V side). Turned 90 degrees so that the latch is on top, this is the same handedness as CEM figure 58, so "pin side" means the mating side of the cable plug. smps.us says the same in words: "the view from the pin side rather than from the wire side".
- NZXT, for the 24-pin plug: hold it "with its clip facing up and the pin holes facing you"; PS_ON# is then the 4th contact from the left in the clip row, with the 4-pin section on the right. That is pin 13 at the left end of the top row.
- The board side is the mirror image. ASRock (B450 Steel Legend, B450M-HDV) and Gigabyte (B450M K) draw the 24-pin header with 12 and 24 at the top, 1 and 13 at the bottom and the latch next to the column 13 to 24. ASRock draws the 8-pin header as 8 ... 5 over 4 ... 1 with the latch mark over the top row, as in CEM figure 59.

The catalog schema's other view (x to W - x, latch staying on top) gives the header view of CEM figure 59.

## Sources
1. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-1 pins and colours, figure 5-2 view, section 4.2.3 remote sensing, housing Molex 39-01-2240, header 44206-0007, accessed 2026-11-04
2. PCI-SIG, "PCI Express Card Electromechanical Specification", Revision 4.0, Version 1.0, August 7, 2019. A members' document, read in a third-party copy at https://billxiang.github.io/BillXiang-BookMarks/./书签工具栏/设备与驱动/PCI/PCIe/PCIe_CEM_SPEC_R4_V1_0_08072019_NCB.pdf — figures 58 and 59: which side shows which numbering in this connector family, accessed 2026-11-04
3. NZXT, "PSU paperclip test", https://nzxt.com/blogs/news/psu-paperclip-test — holding position, clip row, count from the left, accessed 2026-11-04
4. Lazar Rozenblat, "Computer power supply connectors and pinouts", https://smps.us/power-connectors.html — meaning of "pin side", accessed 2026-11-04
5. Gigabyte, B450M K motherboard user manual, https://download.gigabyte.com/FileList/Manual/mb_manual_b450m-k_e.pdf — page 16: header drawing and table, accessed 2026-11-04
6. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 31: header drawing; 20-pin supply goes along pins 1 and 13, accessed 2026-11-04
7. SSI, "EPS12V Power Supply Design Guide", Version 2.91, copy at https://xDevs.com/doc/xDevs.com/Neutron/pdf/eps_designguide_2.91.pdf — table 7, for comparison, accessed 2026-11-04

## Cross-check
The Intel table is authoritative for pins and colours, and figure 5-2 repeats the pins. Gigabyte's table and Moddiy's list agree.

Pin 13 sense: **sources differ.** Intel table 5-1 and section 4.2.3 ("The default sense should be connected to pin 13") put the sense wire on pin 13. SSI EPS12V V2.91, table 7, note 1 attaches the "3.3V remote sense signal double crimped with 3.3V contact" to pin 1. I follow Intel, as this is the ATX item; a reader of a server supply should check.

Pin 20: Intel "Reserved", "NC". SSI: "Reserved (-5 V in ATX)". Older supplies carried -5 V there; the pin note says so.

Pins 10 and 11: SSI names a yellow/blue stripe on +12 V3 for 240 VA limiting; Intel names yellow. I use Intel.

Not reached: the Molex drawings of 39-01-2240 and 44206-0007 (molex.com timed out or reset, also through a mirror). The view rests on the sources above.

## Choices
- Rows, not two columns. the `minifit` outline of the app draws the latch on top, in the middle of the upper edge. Two columns would put it on a short side, where no real connector has one, and `key` is not available on `minifit`. The rows above are exact as drawn. The price is that names do not stand beside the contacts (the catalog schema shows them only for rows of at most two contacts); the pin table carries them.
- `plug` is the cable end and `socket` the board end, as PCI-SIG uses the words. The contacts inside the plug are sockets and those of the header are pins; the captions say so.
- If the outline later gets a side latch, the view of Intel figure 5-2 can be used instead, with names beside the contacts: `rows` [1, 13], [2, 14], ... [12, 24] from top to bottom, latch on the right, still `drawn: plug`.
- `+3.3V`, `+5V`, `+12V1`, `-12V`: Intel's names without the "DC". `COM` as Intel writes it.
- Pin 20 is `nc` without `wire`. Pin 13's `wire` is the orange supply wire; the brown sense wire is in the pin note because a pin has one `wire`.
