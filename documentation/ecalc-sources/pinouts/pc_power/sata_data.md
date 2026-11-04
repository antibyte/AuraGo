# sata_data — SATA data, 7-pin / SATA-Daten, 7-polig

Kind: free official specification
Pins: 7; types: ground 3, data 4

## View
Written for the cable connector, looking at its mating face (`drawn: plug`), with the L-shaped key at the bottom. Read left to right the row is S1 to S7, and the key is at the S7 end.

Here `plug` means the cable end, as in the other PC power items. The SATA documents use the words the other way round: their "device plug" is on the drive and their "receptacle" on the cable. The captions say both.

Basis:
- SATA-IO TP 037, table 1, lists the row along the connector: signal segment key, S1 to S7, signal segment "L", central connector gap, power segment "L", P1 to P15, power segment key. "All pins are in a single row". So the two L keys face each other at S7 and P1.
- Intel ATX figure 5-2, "Pin-side view", shows the power cable connector with "Pin 1" at the top and the L opening to the left at that end. Turned 90 degrees clockwise and then by 180 degrees (rotations, no mirror) it has P1 at the left and the L going down.
- Gigabyte B450M K, page 17, draws the data connectors on the board from above: 1 at the left, 7 at the right, the L rising at the 7 end. That is the mating face of the plug on the board. Its mirror image, the cable side, turned by 180 degrees, has S1 at the left and the L going down at S7. The Intel figure and the Gigabyte drawing are consistent with each other under this reasoning.

The catalog schema's other view (x to W - x, step staying at the bottom) gives the drive or board side with its L at the bottom. Seen on a real drive the whole row can be the other way up, which is a rotation of the same thing. No source states this view in words for the data connector; it is derived from the two drawings and the row order.

## Sources
1. SATA-IO, Technical Proposal TP 037, "SATA TechProp C110" v1.1 (October 2011), https://sata-io.org/sites/default/files/TP_037_SATA_TechProp_C110v1_1.pdf — table 1: names S1 to S7, row order, keys; the document changes the pin table of the SATA specification and repeats it, accessed 2026-11-04
2. Seagate, "BarraCuda Product Manual", 100817550 Rev. L, December 2020, https://www.seagate.com/www-content/product-content/barracuda-fam/barracuda-new/en-us/docs/100817550L.pdf — table 5, page 18: S1 to S7; section 3.3: either end of the signal cable goes to drive or host; figure 2, accessed 2026-11-04
3. Gigabyte, B450M K motherboard user manual, https://download.gigabyte.com/FileList/Manual/mb_manual_b450m-k_e.pdf — page 17: drawing with 1 and 7; table 1 GND, 2 TXP, 3 TXN, 4 GND, 5 RXN, 6 RXP, 7 GND, accessed 2026-11-04

## Cross-check
TP 037, Seagate and Gigabyte agree on the pins. Gigabyte writes the host's view (TXP, RXP), the others A and B. No disagreement.

The SATA specification itself was not read; it is not free. TP 037 is SATA-IO's own document and repeats this table. Molex 67492 and other connector drawings were not reached.

## Choices
- Names `A+`, `A-`, `B-`, `B+` as in the SATA documents. Note the order: B- comes before B+.
- The TX and RX words in the functions hold for a board connector, which is what Gigabyte's table covers. I do not state the drive's direction in the item, as no source I read says it in words.
- `drawn: plug` with the L at the bottom fits the `sata_data` outline of the app (step at the lower right). If the outline changes its step, `rows` has to follow.
- No `wire`: the cable is a shielded pair cable, and no source names colours.
