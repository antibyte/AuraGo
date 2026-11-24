# irf540n — IRF540N N-channel MOSFET (TO-220) / IRF540N N-Kanal-MOSFET (TO-220)

Kind: authoritative maker document (Infineon, formerly International Rectifier); IRLZ44N checked in its own Infineon sheet; Vishay IRF540 (a related, not identical part) as an outside cross-check
Pins: 3; types: signal 3

## Part
IRF540NPbF, 100 V N-channel HEXFET power MOSFET in TO-220 (Infineon package group PG-TO220-3-U05). Suggested display
name `IRF540N (TO-220)`.

Members that share this pinout, as far as a datasheet says so:
- **IRLZ44N (IRLZ44NPbF)**: its own Infineon sheet (Rev. 2.1, 2026-08-12) has the identical page-1 figure: "Gate Pin 1",
  "Drain Pin 2, Tab", "Source Pin 3", TO-220 picture with 1 2 3 from left to right, and the same G / D / S legend.
  **Yes: same G-D-S order, tab = drain.**
- Vishay IRF540 (without N, a different part of the same family): page-1 TO-220AB picture with G, D, S on the leads from
  left to right. Agrees.

## Pin table
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | G | Gate | Gate | signal |
| 2 | D | Drain, also on the tab | Drain, auch an der Metallfahne | signal |
| 3 | S | Source | Source | signal |

Suggested item note (en/de): "The metal tab is connected to the drain (pin 2): a heat sink on the tab carries the
drain voltage, so insulate it from metal at any other potential. The IRLZ44N (logic-level) has the same pinout." /
"Die Metallfahne ist mit dem Drain (Pin 2) verbunden: Ein Kühlkörper an der Fahne führt die Drain-Spannung, also gegen
Metall mit anderem Potenzial isolieren. Der IRLZ44N (Logikpegel) hat dieselbe Belegung."
(The insulation advice is my conclusion from "Drain Pin 2, Tab", as in the `lm317` record, not a datasheet sentence.)

## View
Shape `to`, package `to220`: label side towards the viewer, leads pointing down, pins 1, 2, 3 left to right (same as
`lm317`). Basis: Infineon page 1, right-hand figure "TO-220": a picture of the part with the Infineon logo on the
plastic facing the viewer, tab with hole at the top, leads pointing down-left, labelled "1 2 3" from left to right, and
the tab labelled "tab". That is the catalog view directly. The page-8 "TO-220 Package Outline" front drawing also numbers
the leads 1, 2, 3 from left to right. I rendered pages 1 and 8 to images and looked at them.

## Sources
1. Infineon Technologies, "Datasheet IRF540NPbF", Rev. 2.1, 2026-07-24 (revision history: "Update datasheet to Infineon
   format"), page 1 (symbol "Gate Pin 1 / Drain Pin 2, Tab / Source Pin 3", TO-220 picture "1 2 3", "tab", table
   G Gate / D Drain / S Source) and page 8 "TO-220 Package Outline" (PG-TO220-3-U05).
   https://www.infineon.com/assets/row/public/documents/24/49/infineon-irf540npbf-datasheet-en.pdf — read pages 1 and 8
   rendered, accessed 2026-11-24.
2. Infineon Technologies, "Datasheet IRLZ44NPbF", Rev. 2.1, 2026-08-12, page 1 (same figure and legend as source 1).
   https://www.infineon.com/assets/row/public/documents/24/49/infineon-irlz44npbf-datasheet-en.pdf — read page 1
   rendered, accessed 2026-11-24.
3. Vishay Siliconix, "IRF540 Power MOSFET", Document Number 91021, S21-0819-Rev. C, 02-Aug-2021, page 1 (TO-220AB
   picture with G, D, S). https://www.vishay.com/docs/91021/irf540.pdf — read page 1 rendered, accessed 2026-11-24.

## Cross-check
- Inside Infineon's IRF540N sheet: symbol, picture and outline agree (1 G, 2 D + tab, 3 S).
- IRLZ44N (own Infineon sheet): identical, pin by pin, including tab = drain.
- Vishay IRF540 (related part, other maker): G, D, S left to right as well; Vishay's page 1 does not state the tab
  connection.
- IRF540N is an International Rectifier/Infineon part; no second maker's **IRF540N** was read.

## Choices
- Catalog item: one item named `IRF540N / IRLZ44N (TO-220)` so that both parts are found; tags carry both names and the family (`irf540`, `irlz44`). The IRLZ44N claim rests on its own Infineon sheet (same figure, G-D-S, tab = drain). Names `G`, `D`, `S` typed `signal`; the tab is in the function of pin 2 and the note.
- Names `G`, `D`, `S` as in Infineon's legend; words in the function. Type `signal` as for the transistors in `to92`.
- Tab connection goes into the function of pin 2 and the item note, as in `lm317`.

## Spot-check
| No. | Name | Type | Ref |
| --- | --- | --- | --- |
| 1 | G | signal | Infineon IRF540NPbF Rev. 2.1 page 1 "Gate Pin 1"; picture lead 1 left |
| 2 | D | signal | Infineon IRF540NPbF Rev. 2.1 page 1 "Drain Pin 2, Tab"; picture lead 2 middle, "tab" |
| 3 | S | signal | Infineon IRF540NPbF Rev. 2.1 page 1 "Source Pin 3"; picture lead 3 right |
| IRLZ44N 1/2/3/tab | G/D/S/D | – | Infineon IRLZ44NPbF Rev. 2.1 page 1, same figure |

UNVERIFIED: no second maker's IRF540N read (the part is Infineon/IR's own; Vishay's IRF540 is a related part). Every
pin above was read.
