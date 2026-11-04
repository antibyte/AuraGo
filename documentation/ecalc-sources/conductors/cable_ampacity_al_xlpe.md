# cable_ampacity_al_xlpe — Current-carrying capacity, aluminium, XLPE/EPR (90 °C) / Strombelastbarkeit, Aluminium, VPE/EPR (90 °C)

Kind: transcribed standard (IEC 60364-5-52:2009 Annex B, Tables B.52.3 / B.52.5 / B.52.13)
Guide-value note needed: yes

Reference conditions: aluminium conductor, XLPE or EPR insulation, 90 °C conductor, 30 °C ambient air, one circuit. Subset 16–240 mm².
Bezugsbedingungen: Aluminiumleiter, VPE/EPR, 90 °C Leiter, 30 °C Umgebungsluft, ein Stromkreis. Auswahl 16–240 mm².

## Columns
Same ids and meanings as `cable_ampacity_cu_pvc` (size_mm2 in mm², a1_2 … f_3 in A).

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal conductor cross-section | Leiternennquerschnitt | mm² | yes |
| a1_2 … f_3 | Method (A1, A2, B1, B2, C, E, F) and loaded conductors — see cable_ampacity_cu_pvc | Verlegeart und Anzahl belasteter Adern — siehe cable_ampacity_cu_pvc | A | yes |

## Rows
```csv
size_mm2,a1_2,a1_3,a2_2,a2_3,b1_2,b1_3,b2_2,b2_3,c_2,c_3,e_2,e_3,f_2,f_3
16,64,58,60,55,79,71,72,64,84,76,91,77,,
25,84,76,78,71,105,93,94,84,101,90,108,97,121,103
35,103,94,96,87,130,116,115,103,126,112,135,120,150,129
50,125,113,115,104,157,140,138,124,154,136,164,146,184,159
70,158,142,145,131,200,179,175,156,198,174,211,187,237,206
95,191,171,175,157,242,217,210,188,241,211,257,227,289,253
120,220,197,201,180,281,251,242,216,280,245,300,263,337,296
150,253,226,230,206,307,267,261,240,324,283,346,304,389,343
185,288,256,262,233,351,300,300,272,371,323,397,347,447,395
240,338,300,307,273,412,351,358,318,439,382,470,409,530,471
```

Status:
- Verified (2 independent publishers): f_2 and f_3 for 25–240 mm² (18 cells); b1_2 at 16, 35, 50, 70, 95, 120, 150, 240 mm² and b1_3 at 16, 25, 50, 70, 150, 185 mm² (14 cells).
- UNVERIFIED (one publisher): every a1_*, a2_*, b2_*, c_*, e_* cell; b1_2 at 25 and 185 mm² and b1_3 at 35, 95, 120, 240 mm² (second source consistent only within ±1 A, see Cross-check). 106 cells in total.

## Sources
1. ABB SACE, "Electrical installation handbook — Protection, control and electrical devices", 6th edition 2010, Table 8: https://content.voltimum.com/au/145/abb-electrical-installation-handbook.pdf — accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2", 4th edition 2006, 1SDC010001D0204, Table 8: https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — same publisher; accessed 2026-11-02.
3. Prysmian Cables Spain, datasheet "HARMOHNY Class S — AL XZ1 (S) Eca" (V1.MAY.26), single-core Al/XLPE; column "Intensidad máxima de corriente al aire (30 °C)", 2 and 3 cables touching, footnote (1): "Instalación tipo F, tabla B.52.13 de UNE-HD 60364-5-52 y IEC 60364-5-52": https://es.prysmian.com/sites/es.prysmian.com/files/media/products/HarmOHny-Class-S-AL-XZ1-S-Eca-MAY26.pdf — method F at 30 °C; accessed 2026-11-02.
4. Prysmian Cables Spain, datasheet "AL VOLTALENE FLAMEX CPRO (S) — AL XZ1 (S) Eca" (V1.MAY.26), single-core Al/XLPE; column "bajo tubo o canal protectora (40 °C)", footnote (3): "Tabla B.52.5. Trifásica (T). Tabla B.52.3. Monofásica o continua (M). Método B1 … Para temperatura ambiente de 30 °C, multiplicar las intensidades por 1,1": https://es.prysmian.com/sites/es.prysmian.com/files/media/products/Al-Voltalene-Flamex-CPRO-S-AL-XZ1-S-Eca-MAY26.pdf — method B1 at 40 °C; accessed 2026-11-02. (The sister datasheets "Al Afumex Class AS — AL RZ1 (AS)" and "Al Voltalene e-SenS CPRO (S)" print the same current columns; same publisher, not counted twice.)

## Cross-check
- Method F (f_2, f_3), 25–240 mm²: source 3 prints the 30 °C values directly; all 18 cells equal ABB exactly. (Source 3 also gives 16 mm²: 95/76 A; ABB has no F value at 16 mm², so not added.)
- Method B1 (b1_2, b1_3), 16–240 mm²: source 4 prints 40 °C values. For every cell, round(ABB × 0.91) (0.91 = IEC B.52.14 factor for XLPE at 40 °C, as stated by Prysmian) equals the Prysmian value. Where only one integer 30 °C value maps to the printed 40 °C value, the cell counts as verified (14 cells). Where two neighbouring integers map to it, the cell stays UNVERIFIED (b1_2: 25 mm² 105/106, 185 mm² 350/351; b1_3: 35 mm² 116/117, 95 mm² 216/217, 120 mm² 250/251, 240 mm² 350/351 — ABB value is one of the two). Script: a helper script that is not part of the repository.
- Methods A1, A2, B2, C, E: still only ABB (two editions, consistent). The Schneider Electrical Installation Guide wiki (chapter G, all pages listed in its category "Chapter – Sizing and protection of conductors") reproduces only Table B.52.4 (PVC, 3 loaded) and the simplified Table C.52.1, so it gives no Al XLPE values. Helukabel's technical annex (helu.com URL 404), Kabelwerk Eupen technical data (404) and DIN VDE 0298-4 previews (404) were not reachable; HES Kablo datasheets found are medium-voltage only.
- Plausibility (not a source): the values sit between the Al PVC and Cu XLPE tables for every cell; note the source's own oddity that c_2/c_3 at 25 mm² (101/90 A) are lower than b1_2/b1_3 (105/93 A). IEC notes that values ≥ 25 mm² refer to shaped conductors; this is reproduced as printed and should be checked against IEC 60364-5-52 Table B.52.3/B.52.5 before release.
- Suggested follow-up: check against a licensed copy of IEC 60364-5-52 (or DIN VDE 0298-4 Tabelle 4) or a maker table (e.g. Kabelwerk Eupen "Technical Data for Low Voltage Power and Control cable 0,6/1 kV" — URL returned 404 on 2026-11-02).

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Partly unverified: only the B1 and F columns are confirmed by a second publisher.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Teilweise nicht gegengeprüft: nur die Spalten B1 und F sind durch eine zweite Quelle bestätigt.

## Suggested finder / lookups
As `cable_ampacity_cu_pvc`. Flag results from UNVERIFIED cells (A1, A2, B2, C, E and the six ±1 A B1 cells) as unverified.
