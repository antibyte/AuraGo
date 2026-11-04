# battery_formats — Battery sizes and designations / Batteriegrößen und Bezeichnungen

Kind: typical values (maximum dimensions from maker datasheets, cross-checked with a size list)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| format | common name | gängiger Name | – | no |
| de_name | traditional German name | traditioneller deutscher Name | – | no |
| iec | IEC designation (alkaline version; other chemistries in note) | IEC-Bezeichnung (Alkali-Ausführung; andere Chemien in note) | – | no |
| ansi | ANSI designation (alkaline) | ANSI-Bezeichnung (Alkali) | – | no |
| shape | round (cylindrical/coin) or prismatic | rund (zylindrisch/Knopf) oder prismatisch | – | no (en/de text) |
| d_mm | diameter (max.) or width for prismatic | Durchmesser (max.) bzw. Breite bei prismatisch | mm | yes |
| h_mm | height/length (max.) | Höhe/Länge (max.) | mm | yes |
| t_mm | thickness (prismatic only) | Dicke (nur prismatisch) | mm | yes |
| v_nom | nominal voltage of the common chemistry | Nennspannung der üblichen Chemie | V | yes |
| note | remarks | Bemerkungen | – | no (en/de text) |

## Rows
```csv
format,de_name,iec,ansi,shape,d_mm,h_mm,t_mm,v_nom,note
AAA,Micro,LR03,24A,"round|rund",10.5,44.5,,1.5,"NiMH: HR03 (1.2 V)|NiMH: HR03 (1,2 V)"
AA,Mignon,LR6,15A,"round|rund",14.5,50.5,,1.5,"Energizer E91: Ø 13.50–14.50, h 49.50–50.50 mm; NiMH HR6 1.2 V (eneloop 14.35 x 50.4 mm)|Energizer E91: Ø 13,50–14,50, h 49,50–50,50 mm; NiMH HR6 1,2 V (eneloop 14,35 x 50,4 mm)"
C,Baby,LR14,14A,"round|rund",26.2,50,,1.5,"NiMH: HR14|NiMH: HR14"
D,Mono,LR20,13A,"round|rund",34.2,61.5,,1.5,"NiMH: HR20|NiMH: HR20"
9V,E-Block,6LR61,1604A,"prismatic|prismatisch",26.5,48.5,17.5,9,"Varta designates its 9 V alkaline 6LP3146 (same outline); NiMH 6HR61 is 7.2–9.6 V|Varta bezeichnet seinen 9-V-Alkali-Block als 6LP3146 (gleiche Außenmaße); NiMH 6HR61 hat 7,2–9,6 V"
CR2032,,CR2032,5004LC,"coin|Knopfzelle",20.0,3.2,,3.0,"Name = chemistry C + diameter 20 mm + height 3.2 mm|Name = Chemie C + Durchmesser 20 mm + Höhe 3,2 mm"
14500,,,,"round|rund",14,50,,3.7,"Li-ion in AA size — NOT interchangeable with 1.5 V AA; dimensions single source (UNVERIFIED)|Li-Ion in AA-Größe — NICHT gegen 1,5-V-AA tauschbar; Maße nur eine Quelle (UNVERIFIED)"
18650,,,,"round|rund",18.5,65.3,,3.6,"Size code ≈ nominal Ø 18 mm and length 65 mm; max. with tube per Panasonic NCR18650B|Größencode ≈ Nenn-Ø 18 mm und Länge 65 mm; max. mit Hülle laut Panasonic NCR18650B"
21700,,,,"round|rund",21.25,70.8,,3.6,"Max. with sleeve per Samsung INR21700-50E (3.63 V nominal)|Max. mit Hülle laut Samsung INR21700-50E (3,63 V nenn)"
26650,,,,"round|rund",26,65,,3.3,"Nominal size (A123 LiFePO4 ANR26650M1-B, 3.3 V); Li-ion versions 3.6–3.7 V|Nenngröße (A123 LiFePO4 ANR26650M1-B, 3,3 V); Li-Ion-Ausführungen 3,6–3,7 V"
```

## Sources
1. VARTA Consumer Batteries, "General Purpose | Assortment 2016/17" — VARTA Energy pages 18 and 112–113 (type 4103/4106/4114/4120/4122: height, diameter, width/depth, weight, IEC, other names Micro/Mignon/Baby/Mono/E-Block), copy https://static.mercateo.com/78/6996b3ad28464d939bc17e037ed63c99/pdf/varta-energy_eng_tds.pdf?v=323; accessed 2026-11-02.
2. Energizer, "Product datasheet Energizer E91" (AA), copy https://cdn.connectec.uk/Files/Products/Batteries/energizer_batteries/e91_aa_battery_datasheet.pdf — ANSI-15A, IEC-LR6, Ø 14.50/13.50, height 50.50/49.50 mm; accessed 2026-11-02.
3. Wikipedia, "List of battery sizes", https://en.wikipedia.org/wiki/List_of_battery_sizes — IEC/ANSI designations and nominal dimensions (AAA 10.5 × 44.5, AA 14.5 × 50.5, C 26.2 × 50, D 34.2 × 61.5, 9V 48.5 × 26.5 × 17.5, CR2032 20 × 3.2, 14500 14 × 50, 18650 18 × 65, 21700 21 × 70, 26650 26 × 65); accessed 2026-11-02.
4. Panasonic Energy, "CR2032 CR Coin-type Lithium Battery", as of February 2026, https://energy.panasonic.com/dam/master/pdf/en/datasheet/lithium/CR2032_Datasheet_EN.pdf — Ø 20.0 mm max., height 3.2 mm max., 3 V; accessed 2026-11-02.
5. Panasonic (SANYO), "NCR18650B" datasheet, Version 13.11 R1, copy https://www.batteryspace.com/prod-specs/NCR18650B.pdf — max. 18.5 mm × max. 65.3 mm with tube, 3.6 V; accessed 2026-11-02.
6. Samsung SDI, "Specification of Product INR21700-50E", Version V1.0, copy https://shop.aceongroup.com/cdn/shop/files/RLII-CSI-1618_Samsung_50E_DATASHEET.pdf — top diameter max. 21.25 mm, height max. 70.80 mm (with sleeve), 3.63 V; accessed 2026-11-02.
7. A123 Systems, "ANR26650M1-B" datasheet MD100113-02, copy https://bto.pl/files/Karty%20katalogowe/a123%20datasheet_26650m1b.pdf — cell dimensions 26 × 65 mm, 3.3 V; accessed 2026-11-02.
8. Panasonic, "Cell Type BK-3MCC" (eneloop AA), copy https://www.peak-system.com/produktcd/Tools/PCAN-Diag%20FD/Documentation/Addendum%20Batteries/Panasonic_Cell%20Type%20BK-3MCC.pdf — 14.35 × 50.4 mm with tube, 1.2 V; accessed 2026-11-02.
9. Wikipedia, "Battery nomenclature", https://en.wikipedia.org/wiki/Battery_nomenclature — IEC 60086-1 naming (chemistry letter, R = round, digits = diameter in mm then height in 0.1 mm for coin/button cells) and IEC 61960 naming; accessed 2026-11-02.

## Cross-check
- AAA, C, D, 9V: Varta (1) and Wikipedia (3) agree on all dimensions and IEC designations (except 9V, see below). AA: Varta, Energizer (2) and Wikipedia agree (14.5 × 50.5 mm max.).
- 9V IEC: Wikipedia 6LR61 vs Varta 6LP3146 — both are IEC designations for the same outline; chosen 6LR61 (the common one), Varta's in the note.
- ANSI codes: AA 15A from Energizer + Wikipedia; AAA 24A, C 14A, D 13A, 9V 1604A, CR2032 5004LC from Wikipedia only (single source).
- CR2032: Panasonic (4) and Wikipedia agree (20 × 3.2 mm, 3 V).
- 18650, 21700: maker maxima (5, 6) vs Wikipedia nominal sizes (18 × 65, 21 × 70) — consistent (size code = nominal, maker = maximum incl. sleeve). Table uses the maker maxima.
- 26650: A123 (7) and Wikipedia agree on 26 × 65 (nominal; no maximum was read).
- 14500: Wikipedia only (14 × 50, 3.7 V) → UNVERIFIED.
- de_name values (Micro, Mignon, Baby, Mono, E-Block) are from Varta (1).
- Not included (single source only): AAAA, CR2025, CR2016, CR1220, CR123A, LR44, 4.5 V flat pack, 10440, 16340.

## Notes (en / de)
- en: Dimensions are maximum outline values; real cells are slightly smaller. Li-ion sizes (14500, 18650, 21700, 26650) are named after their nominal diameter and length in mm; they say nothing about chemistry or voltage. A 14500 Li-ion cell fits an AA holder but has 3.6–3.7 V and can destroy 1.5 V devices.
- de: Maße sind maximale Außenmaße; reale Zellen sind etwas kleiner. Li-Ion-Größen (14500, 18650, 21700, 26650) sind nach Nenndurchmesser und -länge in mm benannt; sie sagen nichts über Chemie oder Spannung. Eine 14500-Li-Ion-Zelle passt in einen AA-Halter, hat aber 3,6–3,7 V und kann 1,5-V-Geräte zerstören.

## Suggested finder / lookups
- Identify a cell from measured diameter and length (nearest row within ±1 mm).
- Decode an IEC coin-cell name (e.g. CR2450: Ø 24 mm, 5.0 mm).
