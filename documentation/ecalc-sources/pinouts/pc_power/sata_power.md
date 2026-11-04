# sata_power — SATA power, 15-pin / SATA-Strom, 15-polig

Kind: free official specification
Pins: 15; types: power 9, ground 5, signal 1

## View
Written for the cable connector, looking at its mating face (`drawn: plug`), with the L-shaped key at the bottom. Read left to right the row is P1 to P15, and the key is at the P1 end.

Here `plug` means the cable end, as in the other PC power items. The SATA documents use the words the other way round: their "device plug" is on the drive and their "receptacle" on the cable. The captions say both.

Basis:
- SATA-IO TP 037, table 1, lists the row along the connector: signal segment key, S1 to S7, signal segment "L", central connector gap, power segment "L", P1 to P15, power segment key. "All pins are in a single row". So the two L keys face each other at S7 and P1.
- Intel ATX figure 5-2, "Pin-side view", shows the power cable connector with "Pin 1" at the top and the L opening to the left at that end. Turned 90 degrees clockwise and then by 180 degrees (rotations, no mirror) it has P1 at the left and the L going down.
- Gigabyte B450M K, page 17, draws the data connectors on the board from above: 1 at the left, 7 at the right, the L rising at the 7 end. That is the mating face of the plug on the board. Its mirror image, the cable side, turned by 180 degrees, has S1 at the left and the L going down at S7. The Intel figure and the Gigabyte drawing are consistent with each other under this reasoning.

The catalog schema's other view (x to W - x, step staying at the bottom) gives the drive or board side with its L at the bottom. Seen on a real drive the whole row can be the other way up, which is a rotation of the same thing. No source states this view in words for the data connector; it is derived from the two drawings and the row order.

## Sources
1. SATA-IO, Technical Proposal TP 037, "SATA TechProp C110" v1.1 (October 2011), https://sata-io.org/sites/default/files/TP_037_SATA_TechProp_C110v1_1.pdf — table 1 with notes 1 and 5: P1 to P15, retired 3.3 V, P11 grounded on the cable side, single row, accessed 2026-11-04
2. Seagate, "BarraCuda Product Manual", 100817550 Rev. L, December 2020, https://www.seagate.com/www-content/product-content/barracuda-fam/barracuda-new/en-us/docs/100817550L.pdf — table 5, page 18: pins, pre-charge, mate order on a backplane, P11 "If grounded, drive does not use deferred spin", accessed 2026-11-04
3. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-10 with notes: wires 5 to 1 and colours, "Connector pin numbers and wire numbers are not 1:1"; figure 5-2 view; +3.3 V recommended for backward compatibility, accessed 2026-11-04
4. Western Digital, tech brief "Power Disable Feature", https://documents.westerndigital.com/content/dam/doc-library/en_us/assets/public/western-digital/collateral/tech-brief/tech-brief-western-digital-power-disable-pin.pdf — page 1: legacy supplies with 3.3 V on P3 hold a drive with Power Disable in hard reset, accessed 2026-11-04

## Cross-check
TP 037 and Seagate agree on all voltages and grounds.

**Differences:**
- P11: TP 037 names it `DAS/DSS` (device activity signal / disable staggered spinup), Seagate "Ground or LED signal". The same pin, seen from two sides. I use TP 037's name and describe both in the function and note.
- +3.3 V: TP 037 retires P1 to P3, so "Retired pins ... should be left not connected"; Seagate's Rev. L still lists V33; Intel says it was removed but recommends it for backward compatibility; Western Digital warns about P3. All four statements are true together. The pins stay `power`, because a real cable may be live there, and the item notes the conflict.

Colours per contact are worked out from Intel table 5-10 and figure 5-2: the figure gives the function of each contact, the table the colour of each function. P11 is black as the cable side is grounded (TP 037 note 1, Intel figure 5-2 "COM").

## Choices
- Names `V33`, `V5`, `V12`, `GND`, as in the SATA documents; TP 037's "V33Retired" is shortened, with "(retired)" in the function.
- `signal` for P11: one defined signal on the drive, ground on the cable.
- Mate order is only noted for P4 and P12, as an aid. Seagate's values hold for backplanes only.
- `drawn: plug` with the L at the bottom fits the `sata_power` outline of the app (step at the lower left). If the outline changes its step, `rows` has to follow.
