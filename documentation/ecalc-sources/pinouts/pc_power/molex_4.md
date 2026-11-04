# molex_4 — Peripheral power, 4-pin (Molex) / Laufwerksstrom, 4-polig (Molex)

Kind: free official specification
Pins: 4; types: power 2, ground 2

## View
Written for the cable connector from the power supply, looking at its mating face (`drawn: plug`), with the two cut corners at the bottom. Left to right: 4, 3, 2, 1.

Basis: Maxtor DiamondMax 17, figure 3-5, shows the drive connector in a rear view: "4-Pin DC Power (J1 Section A)", labels 4 3 2 1 from left to right, with the cut corners at the top. The manual adds "Labels indicate the pin numbers on the connector". Then:
- Drive connector, cut corners on top: 4 3 2 1 (as drawn by Maxtor).
- Cable connector, cut corners on top: 1 2 3 4 (mirror image, as the faces look at each other).
- Cable connector, cut corners at the bottom: 4 3 2 1 (turned by 180 degrees). This is `rows`.

Intel ATX figure 5-2 ("Pin-side view") shows pin 1 at the top and pin 4 at the bottom of a vertical connector, which fits a cable connector turned by 90 degrees, but its print is too coarse to confirm on which long side the cut corners are.

The cut-corner side therefore rests on **one maker's drawing plus the mirror rule**. The notes give the order in words for both positions of the corners.

## Sources
1. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-2 pins and colours, section 5.2.2.2 connector parts, table 5-3 floppy connector for comparison, figure 5-2, accessed 2026-11-04
2. Maxtor (Seagate site), DiamondMax 17 80-160GB AT product manual, https://www.seagate.com/staticfiles/maxtor/en_us/documentation/manuals/diamondmax_17_product_manual_pata.pdf — figure 3-5 rear view, table 3-3 pins and mating connector AMP 1-480424-0, accessed 2026-11-04
3. Wikipedia, "Molex connector", https://en.wikipedia.org/wiki/Molex_connector — pinout, colours and the AMP and Molex part names, accessed 2026-11-04

## Cross-check
Intel, Maxtor and Wikipedia agree on the pins. Intel and Wikipedia agree on the colours.

Not reached: TE's customer drawing of 1-480424-0 (HTTP 403 at te.com; the mirror holds only a product page without a drawing) and the Molex drawings (molex.com timed out or reset). They would be the best check of the cut-corner side.

## Choices
- `rows` is written for the `molex` outline of the app, which cuts the two **lower** corners. The catalog schema does not fix this. If the outline changes to cut corners on top, `rows` has to become 1, 2, 3, 4.
- One row, as a connector of this shape has its long side horizontal. So no names beside the contacts; the table carries them.
- `+12V1`, `+5V`: Intel's names without DC. `COM` as Intel writes it.
- `round` contacts: the pins and sockets of this family are round.
