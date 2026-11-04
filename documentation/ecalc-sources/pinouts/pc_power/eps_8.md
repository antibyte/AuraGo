# eps_8 — EPS12V CPU power, 8-pin / EPS12V-CPU-Strom, 8-polig

Kind: free official specification
Pins: 8; types: ground 4, power 4

## View
Written for the cable plug from the power supply, looking at its mating face (`drawn: plug`), latch on top: the contact holes face the reader.

Basis, four sources that fit together:
- PCI-SIG CEM Rev. 4.0 names the two sides. Figure 58, "2 x 4 Auxiliary Power Connector Plug Side Pin-out", shows the latch on top, PIN 5 6 7 8 in the latch row and PIN 1 2 3 4 below, each left to right. Figure 59, "Receptacle Side Pin-out", shows the mirror image: PIN 8 7 6 5 over PIN 4 3 2 1. Figure 60 shows the 2 x 3 plug the same way: PIN 4 5 6 over PIN 1 2 3.
- Intel ATX Rev. 2.1a, figure 5-2, is captioned "Connectors (Pin-side view, not to Scale)" and shows the harness connectors with pin 1 at top left, pin 13 at top right and the latch on the right beside pins 13 to 24 (the 4-pin +12V2 connector likewise: 1 and 3 on top, latch on the +12 V side). Turned 90 degrees so that the latch is on top, this is the same handedness as CEM figure 58, so "pin side" means the mating side of the cable plug. smps.us says the same in words: "the view from the pin side rather than from the wire side".
- NZXT, for the 24-pin plug: hold it "with its clip facing up and the pin holes facing you"; PS_ON# is then the 4th contact from the left in the clip row, with the 4-pin section on the right. That is pin 13 at the left end of the top row.
- The board side is the mirror image. ASRock (B450 Steel Legend, B450M-HDV) and Gigabyte (B450M K) draw the 24-pin header with 12 and 24 at the top, 1 and 13 at the bottom and the latch next to the column 13 to 24. ASRock draws the 8-pin header as 8 ... 5 over 4 ... 1 with the latch mark over the top row, as in CEM figure 59.

The catalog schema's other view (x to W - x, latch staying on top) gives the header view of CEM figure 59.

For this connector also: Moddiy, "An 8 pin 12 volt cable has yellow wires on the same side as the connector clip", and Intel's 4-pin figure shows the +12 V side at the latch. So pins 5 to 8 are in the latch row. No source draws the 8-pin EPS plug with a stated view of its own: the view is derived from the family (Intel figure 5-2 and CEM figure 58) and from ASRock's 8-pin header drawing, which is its mirror.

## Sources
1. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-9 pins and colours; figure 5-2 shows the 4-pin version; its note says the 8-pin 12V2 connector is not drawn, accessed 2026-11-04
2. SSI, "EPS12V Power Supply Design Guide", Version 2.91, copy at https://xDevs.com/doc/xDevs.com/Neutron/pdf/eps_designguide_2.91.pdf — table 8 pins and colours, housing Molex 39-01-2080, accessed 2026-11-04
3. PCI-SIG, "PCI Express Card Electromechanical Specification", Revision 4.0, Version 1.0, August 7, 2019. A members' document, read in a third-party copy at https://billxiang.github.io/BillXiang-BookMarks/./书签工具栏/设备与驱动/PCI/PCIe/PCIe_CEM_SPEC_R4_V1_0_08072019_NCB.pdf — figures 58 and 59: numbering of plug and receptacle in the 2 x 4 housing, accessed 2026-11-04
4. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 31: 8 ... 5 over 4 ... 1, latch mark over the top row; a 4-pin supply goes along pins 1 and 5, accessed 2026-11-04
5. Gigabyte, B450M K motherboard user manual, https://download.gigabyte.com/FileList/Manual/mb_manual_b450m-k_e.pdf — page 16: table and drawing, see the disagreement below, accessed 2026-11-04
6. Moddiy, "Power Supply Connectors & Pinouts", https://moddiy.com/pages/Power-Supply-Connectors-&-Pinouts.html — clip side of the EPS and PCIe plugs, accessed 2026-11-04
7. Molex, Mini-Fit Jr. receptacle housing sheet, copy at https://static.chipdip.ru/lib/739/DOC038739126.pdf — lists 39-01-2080 in its ordering table, so the housing SSI names belongs to the family, accessed 2026-11-04

## Cross-check
Pins: Intel, SSI and Gigabyte's table agree (1 to 4 ground, 5 to 8 +12 V).

Rails and colours: Intel names +12V2 on all four, yellow. SSI divides them into +12 V1 (5, 6, yellow/black stripe) and +12 V2 (7, 8, yellow), all plain yellow when 240 VA limiting is not needed. I use Intel.

**Drawing disagreement:** Gigabyte's ATX_12V drawing shows 4 ... 1 over 8 ... 5 with the latch beside the lower row. Turned so that the latch is on top, it reads 5 ... 8 over 1 ... 4, which is the plug's handedness on a drawing of a board header, and the opposite of Gigabyte's own 24-pin drawing on the same page. ASRock's header drawing and CEM figure 59 have the other handedness, so I treat Gigabyte's drawing as the outlier. Both agree that the latch is next to pins 5 to 8.

Not reached: Molex drawing of 39-01-2080.

## Choices
- Rows, not two columns. the `minifit` outline of the app draws the latch on top, in the middle of the upper edge. Two columns would put it on a short side, where no real connector has one, and `key` is not available on `minifit`. The rows above are exact as drawn. The price is that names do not stand beside the contacts (the catalog schema shows them only for rows of at most two contacts); the pin table carries them.
- `plug` is the cable end and `socket` the board end, as PCI-SIG uses the words. The contacts inside the plug are sockets and those of the header are pins; the captions say so.
- `+12V2` is Intel's name in table 5-9. `COM` as Intel writes it.
- The note warns about the PCIe 8-pin: both sources name the rows, and Intel says the PCIe housing has its own keying.
