# pcie_6 — PCIe power, 6-pin (2x3) / PCIe-Strom, 6-polig (2x3)

Kind: free official specification
Pins: 6; types: power 3, ground 2, signal 1

## View
Written for the cable plug from the power supply, looking at its mating face (`drawn: plug`), latch on top: the contact holes face the reader.

Basis, four sources that fit together:
- PCI-SIG CEM Rev. 4.0 names the two sides. Figure 58, "2 x 4 Auxiliary Power Connector Plug Side Pin-out", shows the latch on top, PIN 5 6 7 8 in the latch row and PIN 1 2 3 4 below, each left to right. Figure 59, "Receptacle Side Pin-out", shows the mirror image: PIN 8 7 6 5 over PIN 4 3 2 1. Figure 60 shows the 2 x 3 plug the same way: PIN 4 5 6 over PIN 1 2 3.
- Intel ATX Rev. 2.1a, figure 5-2, is captioned "Connectors (Pin-side view, not to Scale)" and shows the harness connectors with pin 1 at top left, pin 13 at top right and the latch on the right beside pins 13 to 24 (the 4-pin +12V2 connector likewise: 1 and 3 on top, latch on the +12 V side). Turned 90 degrees so that the latch is on top, this is the same handedness as CEM figure 58, so "pin side" means the mating side of the cable plug. smps.us says the same in words: "the view from the pin side rather than from the wire side".
- NZXT, for the 24-pin plug: hold it "with its clip facing up and the pin holes facing you"; PS_ON# is then the 4th contact from the left in the clip row, with the 4-pin section on the right. That is pin 13 at the left end of the top row.
- The board side is the mirror image. ASRock (B450 Steel Legend, B450M-HDV) and Gigabyte (B450M K) draw the 24-pin header with 12 and 24 at the top, 1 and 13 at the bottom and the latch next to the column 13 to 24. ASRock draws the 8-pin header as 8 ... 5 over 4 ... 1 with the latch mark over the top row, as in CEM figure 59.

The catalog schema's other view (x to W - x, latch staying on top) gives the header view of CEM figure 59.

## Sources
1. PCI-SIG, "PCI Express Card Electromechanical Specification", Revision 4.0, Version 1.0, August 7, 2019. A members' document, read in a third-party copy at https://billxiang.github.io/BillXiang-BookMarks/./书签工具栏/设备与驱动/PCI/PCIe/PCIe_CEM_SPEC_R4_V1_0_08072019_NCB.pdf — figure 60 (2 x 3 pin-out, latch on top), table 53 (2 x 3 plug to 2 x 4 receptacle mapping), accessed 2026-11-04
2. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-4 pins, colours and sense note, accessed 2026-11-04
3. Moddiy, "Power Supply Connectors & Pinouts", https://moddiy.com/pages/Power-Supply-Connectors-&-Pinouts.html — clip side and colours, accessed 2026-11-04
4. Hardware Secrets, "Everything You Need to Know About Power Supplies", page 14, https://www.hardwaresecrets.com/everything-you-need-to-know-about-power-supplies/14/ — pin table, see the disagreement below, accessed 2026-11-04

## Cross-check
Intel table 5-4 and CEM table 53 agree: 1 to 3 +12 V, 4 ground, 5 sense, 6 ground.

**Disagreement:** Hardware Secrets gives pin 5 as ground and pin 6 as Sense0, and says pin 2 must be left unconnected. Moddiy lists pins 4 to 6 as plain ground and pin 2 as "yellow or not connected". I follow the PCI-SIG and Intel documents. CEM table 53 also explains why: plug pin 5 meets receptacle pin 6 (Sense0), and plug pin 6 meets receptacle pin 7 (ground). With Hardware Secrets' numbering the plug would not line up with the 8-pin header.

Pin 2: the two current documents have +12 V. Two secondary sources report cables with pin 2 unconnected, so the note mentions it.

## Choices
- Rows, not two columns. the `minifit` outline of the app draws the latch on top, in the middle of the upper edge. Two columns would put it on a short side, where no real connector has one, and `key` is not available on `minifit`. The rows above are exact as drawn. The price is that names do not stand beside the contacts (the catalog schema shows them only for rows of at most two contacts); the pin table carries them.
- `plug` is the cable end and `socket` the board end, as PCI-SIG uses the words. The contacts inside the plug are sockets and those of the header are pins; the captions say so.
- `+12V` as in CEM; Intel's "+12V3/V4" names rails of its own supply model. `COM` and `Sense` as Intel writes them. The sense wire is black, per Intel.
- `signal` for Sense, as the catalog schema says.
- Neither document calls this housing Mini-Fit; both give the 4.2 mm pitch, so the doc text says that. The outline `minifit` and the tag follow the plan, and the geometry is that of the family.
