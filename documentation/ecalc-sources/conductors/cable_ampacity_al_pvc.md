# cable_ampacity_al_pvc — Current-carrying capacity, aluminium, PVC (70 °C) / Strombelastbarkeit, Aluminium, PVC (70 °C)

Kind: transcribed standard (IEC 60364-5-52:2009 Annex B, Tables B.52.2 / B.52.4 / B.52.11)
Guide-value note needed: yes

Reference conditions: aluminium conductor, PVC insulation, 70 °C conductor, 30 °C ambient air, one circuit. Subset 16–240 mm² (IEC also lists 2.5–10 mm² Al; DIN VDE 0100-520 practice uses Al only from 16 mm²).
Bezugsbedingungen: Aluminiumleiter, PVC, 70 °C Leiter, 30 °C Umgebungsluft, ein Stromkreis. Auswahl 16–240 mm².

## Columns
Same ids and meanings as `cable_ampacity_cu_pvc` (size_mm2 in mm², a1_2 … f_3 in A).

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal conductor cross-section | Leiternennquerschnitt | mm² | yes |
| a1_2 … f_3 | Method (A1, A2, B1, B2, C, E, F) and loaded conductors (2 / 3; F: 2 flat, 3 trefoil) — see cable_ampacity_cu_pvc | Verlegeart und Anzahl belasteter Adern — siehe cable_ampacity_cu_pvc | A | yes |

## Rows
```csv
size_mm2,a1_2,a1_3,a2_2,a2_3,b1_2,b1_3,b2_2,b2_3,c_2,c_3,e_2,e_3,f_2,f_3
16,48,43,44,41,60,53,54,48,66,59,73,61,,
25,63,57,58,53,79,70,71,62,83,73,89,78,98,84
35,77,70,71,65,97,86,86,77,103,90,111,96,122,105
50,93,84,86,78,118,104,104,92,125,110,135,117,149,128
70,118,107,108,98,150,133,131,116,160,140,173,150,192,166
95,142,129,130,118,181,161,157,139,195,170,210,183,235,203
120,164,149,150,135,210,186,181,160,226,197,244,212,273,237
150,189,170,172,155,234,204,201,176,261,227,282,245,316,274
185,215,194,195,176,266,230,230,199,298,259,322,280,363,315
240,252,227,229,207,312,269,269,232,352,305,380,330,430,375
```

Verification status per column: a1_3, a2_3, b1_3, b2_3, c_3 = verified (2 independent sources). a1_2, a2_2, b1_2, b2_2, c_2, e_2, e_3, f_2, f_3 = UNVERIFIED (one publisher only).

## Sources
1. ABB SACE, "Electrical installation handbook — Protection, control and electrical devices", 6th edition 2010, Table 8 (pp. 288–291): https://content.voltimum.com/au/145/abb-electrical-installation-handbook.pdf — all cells; accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2", 4th edition 2006, 1SDC010001D0204, Table 8: https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — same publisher; accessed 2026-11-02.
3. Schneider Electric, Electrical Installation Guide (wiki), "General method for cable sizing", Fig. G20 = IEC 60364-5-52 Table B.52.4 (PVC, 3 loaded, Cu and Al): https://www.electrical-installation.org/enwiki/General_method_for_cable_sizing — accessed 2026-11-02.

## Cross-check
- 3-loaded columns A1, A2, B1, B2, C: ABB (1) and Schneider EIG (3) agree on all 50 cells.
- 2-loaded columns and E/F columns: only ABB (two editions of the same handbook, mutually consistent) → UNVERIFIED.
- Follow-up search (2026-11-02) for a second Al PVC source, without result: the Schneider Electrical Installation Guide wiki (all chapter-G pages) reproduces only Table B.52.4 (PVC, 3 loaded — already used above) and the simplified Table C.52.1; its C.52.1 columns merge several method/loading cases and are not exact for the 2-loaded cells, so they were not used. Prysmian Spain Al datasheets cover only XLPE (AL XZ1/AL RZ1). Helukabel technical annex (URL 404), Kabelwerk Eupen technical data (404), DIN VDE 0298-4 preview (404), BS 7671 aluminium tables (no reproduction found) were not reachable.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Aluminium conductors need terminals approved for aluminium.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Aluminiumleiter nur an dafür zugelassenen Klemmen anschließen.

## Suggested finder / lookups
As `cable_ampacity_cu_pvc`. The finder should show an "unverified" marker when it lands on a 2-loaded, E or F aluminium cell.
