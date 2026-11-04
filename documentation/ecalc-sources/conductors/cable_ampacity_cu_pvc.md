# cable_ampacity_cu_pvc — Current-carrying capacity, copper, PVC (70 °C) / Strombelastbarkeit, Kupfer, PVC (70 °C)

Kind: transcribed standard (IEC 60364-5-52:2009 Annex B, Tables B.52.2 / B.52.4 / B.52.10; equal to DIN VDE 0298-4:2013-06 Tabelle 3 except the two DIN cells listed under Cross-check)
Guide-value note needed: yes

Reference conditions: copper conductor, PVC insulation, max. conductor temperature 70 °C, ambient air 30 °C, one circuit (no grouping), continuous operation, AC 50/60 Hz or DC.
Bezugsbedingungen: Kupferleiter, PVC-Isolierung, zulässige Leitertemperatur 70 °C, Umgebungsluft 30 °C, ein Stromkreis (keine Häufung), Dauerbetrieb.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal conductor cross-section | Leiternennquerschnitt | mm² | yes |
| a1_2 | Method A1 — insulated conductors / single-core cables in conduit in a thermally insulated wall; 2 loaded conductors | Verlegeart A1 — Aderleitungen / einadrige Kabel im Elektroinstallationsrohr in wärmegedämmter Wand; 2 belastete Adern | A | yes |
| a1_3 | Method A1; 3 loaded conductors | Verlegeart A1; 3 belastete Adern | A | yes |
| a2_2 | Method A2 — multi-core cable in conduit in a thermally insulated wall; 2 loaded | Verlegeart A2 — mehradriges Kabel / Mantelleitung im Rohr in wärmegedämmter Wand; 2 belastete Adern | A | yes |
| a2_3 | Method A2; 3 loaded | Verlegeart A2; 3 belastete Adern | A | yes |
| b1_2 | Method B1 — insulated conductors / single-core cables in conduit on (or in) a wall; 2 loaded | Verlegeart B1 — Aderleitungen / einadrige Kabel im Elektroinstallationsrohr auf oder in der Wand; 2 belastete Adern | A | yes |
| b1_3 | Method B1; 3 loaded | Verlegeart B1; 3 belastete Adern | A | yes |
| b2_2 | Method B2 — multi-core cable in conduit on (or in) a wall; 2 loaded | Verlegeart B2 — mehradriges Kabel / Mantelleitung im Rohr auf oder in der Wand; 2 belastete Adern | A | yes |
| b2_3 | Method B2; 3 loaded | Verlegeart B2; 3 belastete Adern | A | yes |
| c_2 | Method C — cable fixed directly on/in a wall or on an unperforated tray; 2 loaded | Verlegeart C — direkt auf/in der Wand oder auf ungelochter Kabelrinne; 2 belastete Adern | A | yes |
| c_3 | Method C; 3 loaded | Verlegeart C; 3 belastete Adern | A | yes |
| e_2 | Method E — multi-core cable in free air / on perforated tray (≥ 0.3 d from the wall); 2 loaded | Verlegeart E — mehradriges Kabel frei in Luft / gelochte Kabelrinne (Wandabstand ≥ 0,3 d); 2 belastete Adern | A | yes |
| e_3 | Method E; 3 loaded | Verlegeart E; 3 belastete Adern | A | yes |
| f_2 | Method F — single-core cables touching, in free air; 2 loaded (flat) | Verlegeart F — einadrige Kabel mit Berührung frei in Luft; 2 belastete Leiter (flach) | A | yes |
| f_3 | Method F; 3 loaded, trefoil | Verlegeart F; 3 belastete Leiter im Dreieck | A | yes |

Empty cell = no value in the standard (F only from 25 mm²). / Leere Zelle = kein Wert in der Norm (F erst ab 25 mm²).

## Rows
```csv
size_mm2,a1_2,a1_3,a2_2,a2_3,b1_2,b1_3,b2_2,b2_3,c_2,c_3,e_2,e_3,f_2,f_3
1.5,14.5,13.5,14,13,17.5,15.5,16.5,15,19.5,17.5,22,18.5,,
2.5,19.5,18,18.5,17.5,24,21,23,20,27,24,30,25,,
4,26,24,25,23,32,28,30,27,36,32,40,34,,
6,34,31,32,29,41,36,38,34,46,41,51,43,,
10,46,42,43,39,57,50,52,46,63,57,70,60,,
16,61,56,57,52,76,68,69,62,85,76,94,80,,
25,80,73,75,68,101,89,90,80,112,96,119,101,131,110
35,99,89,92,83,125,110,111,99,138,119,148,126,162,137
50,119,108,110,99,151,134,133,118,168,144,180,153,196,167
70,151,136,139,125,192,171,168,149,213,184,232,196,251,216
95,182,164,167,150,232,207,201,179,258,223,282,238,304,264
120,210,188,192,172,269,239,232,206,299,259,328,276,352,308
150,240,216,219,196,300,262,258,225,344,299,379,319,406,356
185,273,245,248,223,341,296,294,255,392,341,434,364,463,409
240,321,286,291,261,400,346,344,297,461,403,514,430,546,485
```

DIN VDE 0298-4 variant (only differences): size 1.5 → a1_2 = 15.5, a2_2 = 15.5.

## Sources
1. ABB SACE, "Electrical installation handbook — Protection, control and electrical devices", Technical guide 6th edition 2010, Table 8 "Current carrying capacity of cables with PVC or EPR/XLPE insulation" (methods A–C and E–F–G), pp. 288–291; based on IEC 60364-5-52. PDF hosted by Voltimum: https://content.voltimum.com/au/145/abb-electrical-installation-handbook.pdf — all columns, all sizes; accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2 — Electrical devices", 4th edition March 2006, 1SDC010001D0204, Table 8: https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — same values as source 1 (same publisher, not counted as independent); in this edition B1/B2 stop at 120 mm²; accessed 2026-11-02.
3. BS 7671:2018 Appendix 4, Table 4D1A (single-core 70 °C thermoplastic, copper) and Table 4D2A (multicore 70 °C thermoplastic, copper), "Reproduced from BS7671:2018", Eland Cables PDFs: https://elandcables.com/media/vyrp3kxq/table-4d1a-and-4d1b.pdf and https://elandcables.com/media/ek1mri4h/table-4d2a-and-4d2b.pdf — Ref. Method A/B/C/E/F; accessed 2026-11-02. (IET Wiring Matters, Autumn 2008, M. Coles, "Appendix 4 of BS 7671", states that most BS 7671 ratings are taken from IEC 60364-5-52: https://electrical.theiet.org/media/1617/appendix-4-of-bs-7671.pdf.)
4. Schneider Electric, Electrical Installation Guide (wiki), "General method for cable sizing", Fig. G20 = IEC 60364-5-52 Table B.52.4 (PVC, 3 loaded, Cu/Al, methods A1–D2): https://www.electrical-installation.org/enwiki/General_method_for_cable_sizing — all 3-loaded columns A1, A2, B1, B2, C; accessed 2026-11-02 (read in the built-in browser; plain fetch returns HTTP 403).
5. ABB STOTZ-KONTAKT, "Technische Information: Kabel- und Leitungsdimensionierung und Auswahl von Überstrom-Schutzeinrichtungen", 2CDC400027D0104 Rev. D (02/2018), Tabelle 2 "Strombelastbarkeit bei fester Verlegung und Dauerbetrieb, Umgebungstemperatur 30 °C" — excerpt of DIN VDE 0298-4:2013-06 reproduced with DIN/VDE permission (Cu, PVC, A1–C, E, F, G, 1.5–120 mm²): https://content.voltimum.com/de/118048/2cdc400027d0104_revd_02-2018_72dpi_0.pdf — accessed 2026-11-02.
6. Engineering ToolBox, "Electric Cable Size vs. Current Rating" (Cu PVC, A1/A2/B1/B2, 2 and 3 loaded): https://www.engineeringtoolbox.com/cables-current-rating-a1-a2-b1-b2-d_1875.html — rows 1.5–120 mm² used; its rows ≥ 150 mm² came out column-shifted in extraction and were not used; accessed 2026-11-02.

## Cross-check
- Every published cell is confirmed by at least two independent owners: ABB (source 1) and BS 7671 (source 3) for all columns; Schneider EIG (source 4) additionally for all 3-loaded A1/A2/B1/B2/C cells; DIN excerpts (sources 5, 6) additionally for 1.5–120 mm². Script: a helper script that is not part of the repository (886 cells parsed, 0 unexplained mismatches).
- Disagreement 1 (national deviation, not an error): DIN VDE 0298-4 (sources 5, 6) gives 15.5 A for 1.5 mm² A1 and A2 with 2 loaded conductors (footnote: "Bewertungsunterschiede, siehe DIN VDE 0298-4 Anhang C.3.3"); IEC/BS (sources 1, 3) give 14.5 A (A1) and 14 A (A2). Chosen: IEC values in the table, DIN values listed as variant above.
- Disagreement 2: BS 7671 Table 4D1A gives 20 A for 2.5 mm² Method A, 2 cables; ABB (IEC) and both DIN sources give 19.5 A. Chosen 19.5 A (3 sources, matches IEC and DIN).
- BS 7671 Table 4D1A Method C (single-core clipped direct) has its own higher values (e.g. 25 mm²: 114 A vs IEC 112 A); the c_* columns here follow IEC (= BS 7671 Table 4D2A multicore Method C). Not used.
- In the 2006 ABB edition (source 2) B1/B2 end at 120 mm²; the 150–240 mm² B1/B2 values come from the 2010 ABB edition (IEC 60364-5-52:2009) and are confirmed by BS 7671 (2-loaded and 3-loaded) and Schneider EIG (3-loaded).
- DIN VDE 0298-4 current edition is 2023-06; the DIN sources reproduce the 2013-06 edition. Changes in 2023 were not checked (standard not accessible).
- No UNVERIFIED cells.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Values apply to one circuit at 30 °C ambient air; multiply by the ambient-temperature factor (cable_ambient_temp_factors) and the grouping factor (cable_grouping_factors). In German building installations DIN VDE 0298-4 limits the conductor temperature to 70 °C even for cables rated higher, and gives 15.5 A for 1.5 mm² A1/A2 with 2 loaded conductors.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Die Werte gelten für einen Stromkreis bei 30 °C Umgebungsluft; mit dem Umrechnungsfaktor für die Umgebungstemperatur (cable_ambient_temp_factors) und dem Häufungsfaktor (cable_grouping_factors) multiplizieren. Nach DIN VDE 0298-4 gilt für 1,5 mm² in A1/A2 mit 2 belasteten Adern 15,5 A.

## Suggested finder / lookups
Column = method + loaded conductors (e.g. `b2_3`). Iz = table value × k_temp × k_group (× k_multicore). Smallest size_mm2 with Iz ≥ Ib (or ≥ In of the protective device). See SUMMARY.md, finder (a).
