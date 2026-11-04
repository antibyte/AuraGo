# cable_ampacity_cu_xlpe — Current-carrying capacity, copper, XLPE/EPR (90 °C) / Strombelastbarkeit, Kupfer, VPE/EPR (90 °C)

Kind: transcribed standard (IEC 60364-5-52:2009 Annex B, Tables B.52.3 / B.52.5 / B.52.12)
Guide-value note needed: yes

Reference conditions: copper conductor, XLPE or EPR insulation, max. conductor temperature 90 °C, ambient air 30 °C, one circuit, continuous operation.
Bezugsbedingungen: Kupferleiter, VPE- oder EPR-Isolierung, zulässige Leitertemperatur 90 °C, Umgebungsluft 30 °C, ein Stromkreis, Dauerbetrieb.

## Columns
Same column ids and meanings as `cable_ampacity_cu_pvc` (size_mm2 in mm²; a1_2 … f_3 in A, method + number of loaded conductors).

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal conductor cross-section | Leiternennquerschnitt | mm² | yes |
| a1_2, a1_3 | Method A1 (single-core in conduit in thermally insulated wall), 2 / 3 loaded | Verlegeart A1 (einadrig im Rohr in wärmegedämmter Wand), 2 / 3 belastete Adern | A | yes |
| a2_2, a2_3 | Method A2 (multi-core in conduit in thermally insulated wall), 2 / 3 loaded | Verlegeart A2 (mehradrig im Rohr in wärmegedämmter Wand), 2 / 3 belastete Adern | A | yes |
| b1_2, b1_3 | Method B1 (single-core in conduit on a wall), 2 / 3 loaded | Verlegeart B1 (einadrig im Rohr auf der Wand), 2 / 3 belastete Adern | A | yes |
| b2_2, b2_3 | Method B2 (multi-core in conduit on a wall), 2 / 3 loaded | Verlegeart B2 (mehradrig im Rohr auf der Wand), 2 / 3 belastete Adern | A | yes |
| c_2, c_3 | Method C (direct on wall / unperforated tray), 2 / 3 loaded | Verlegeart C (direkt auf der Wand / ungelochte Rinne), 2 / 3 belastete Adern | A | yes |
| e_2, e_3 | Method E (multi-core in free air / perforated tray), 2 / 3 loaded | Verlegeart E (mehradrig frei in Luft / gelochte Rinne), 2 / 3 belastete Adern | A | yes |
| f_2, f_3 | Method F (single-core touching in free air), 2 loaded flat / 3 loaded trefoil | Verlegeart F (einadrig mit Berührung frei in Luft), 2 Leiter flach / 3 Leiter im Dreieck | A | yes |

## Rows
```csv
size_mm2,a1_2,a1_3,a2_2,a2_3,b1_2,b1_3,b2_2,b2_3,c_2,c_3,e_2,e_3,f_2,f_3
1.5,19,17,18.5,16.5,23,20,22,19.5,24,22,26,23,,
2.5,26,23,25,22,31,28,30,26,33,30,36,32,,
4,35,31,33,30,42,37,40,35,45,40,49,42,,
6,45,40,42,38,54,48,51,44,58,52,63,54,,
10,61,54,57,51,75,66,69,60,80,71,86,75,,
16,81,73,76,68,100,88,91,80,107,96,115,100,,
25,106,95,99,89,133,117,119,105,138,119,149,127,161,135
35,131,117,121,109,164,144,146,128,171,147,185,158,200,169
50,158,141,145,130,198,175,175,154,209,179,225,192,242,207
70,200,179,183,164,253,222,221,194,269,229,289,246,310,268
95,241,216,220,197,306,269,265,233,328,278,352,298,377,328
120,278,249,253,227,354,312,305,268,382,322,410,346,437,383
150,318,285,290,259,393,342,334,300,441,371,473,399,504,444
185,362,324,329,295,449,384,384,340,506,424,542,456,575,510
240,424,380,386,346,528,450,459,398,599,500,641,538,679,607
```

UNVERIFIED cells: 2.5 mm² a1_2 (26), a1_3 (23), b1_2 (31), b1_3 (28).

## Sources
1. ABB SACE, "Electrical installation handbook — Protection, control and electrical devices", 6th edition 2010, Table 8 (pp. 288–291): https://content.voltimum.com/au/145/abb-electrical-installation-handbook.pdf — all cells; accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2", 4th edition 2006, 1SDC010001D0204, Table 8: https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — same publisher (consistency only); accessed 2026-11-02.
3. BS 7671:2018 Appendix 4, Table 4E1A (single-core 90 °C thermosetting, copper; Ref. Methods A, B, C, F) and Table 4E2A (multicore 90 °C thermosetting, copper; Ref. Methods A, B, C, E), "Reproduced from BS7671:2018", Eland Cables: https://www.elandcables.com/media/ehkk3rg2/table-4e1a-and-4e1b.pdf and https://www.elandcables.com/media/ksllepjp/table-4e2a-and-4e2b.pdf — accessed 2026-11-02.

## Cross-check
- Source 1 (ABB) and source 3 (BS 7671) agree on every cell they share: A1/B1/F against 4E1A, A2/B2/C/E against 4E2A, 1.5–240 mm². Checked by a helper script that is not part of the repository.
- The Eland reproduction of Table 4E1A has no 2.5 mm² row (it jumps from 1.5 to 4 mm²), so the four 2.5 mm² A1/B1 cells rest on ABB only → UNVERIFIED (values look consistent with the neighbouring rows).
- Follow-up search (2026-11-02): the Schneider Electrical Installation Guide wiki has no XLPE B-table (only B.52.4 PVC and the simplified C.52.1, whose merged columns are not exact for these cells). Prysmian Spain "EXZHELLENT Compact 1000 V (AS) — RZ1-K" (Cu/XLPE, https://es.prysmian.com/sites/es.prysmian.com/files/media/products/Exzhellent-Compact-1000V-RZ1-K-Cca-s1b-d1-a1-OCT23.pdf) gives 25 A for 1×2.5 mm² "bajo tubo" (B1, B.52.5, 3 loaded) at 40 °C; 28 × 0.91 = 25.5 → 25, but 27 × 0.91 = 24.6 → 25 as well, so it supports b1_3 = 28 only within ±1 A. No source found for a1_2, a1_3, b1_2. All four cells stay UNVERIFIED.
- B1/B2 150–240 mm² are not in the 2006 ABB edition (B1/B2 stop at 120 there); they are in the 2010 edition and in BS 7671 (4E1A/4E2A), which agree.
- No DIN source for the 90 °C table was reachable (DIN VDE 0298-4 Tabelle 4 not excerpted by the sources found).

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. 90 °C values may only be used if terminals, devices and accessories are rated for 90 °C; in building installations (DIN VDE 0298-4) the conductor temperature is usually limited to 70 °C, i.e. use the PVC table.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. 90-°C-Werte nur verwenden, wenn Klemmen, Geräte und Zubehör für 90 °C geeignet sind; in der Gebäudeinstallation (DIN VDE 0298-4) ist die Leitertemperatur meist auf 70 °C zu begrenzen, d. h. PVC-Tabelle verwenden.

## Suggested finder / lookups
As `cable_ampacity_cu_pvc`; use the k_90 column of `cable_ambient_temp_factors`.
