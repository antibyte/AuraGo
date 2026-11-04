# pcb_copper_weight — PCB copper weight vs. thickness / Kupferauflage (oz/ft²) und Schichtdicke

Kind: formula-derived (area weight from unit definitions; thickness = area weight / density) + typical values (nominal thickness designations per IPC-4562 and industry practice)
Guide-value note needed: no

One "ounce" copper = 1 avoirdupois ounce of copper spread over 1 ft². The area weight follows from the unit definitions; the thickness depends on the copper density assumed, so several "1 oz" thicknesses are in use.
„1 oz" Kupfer = 1 Unze Kupfer auf 1 Quadratfuß verteilt. Das Flächengewicht folgt aus den Einheiten, die Dicke hängt von der angenommenen Dichte ab.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| weight_oz | Copper weight | Kupferauflage | oz/ft² | yes |
| area_weight_g_m2 | Area weight (oz × 305.1517 g/m²) | Flächengewicht (oz × 305,1517 g/m²) | g/m² | yes |
| t_physical_um | Thickness = area weight ÷ copper density 8.935 g/cm³ (20 °C) | Dicke = Flächengewicht ÷ Kupferdichte 8,935 g/cm³ (20 °C) | µm | yes |
| t_ipc4562_um | Nominal thickness per IPC-4562A as quoted by Siemens (Hargin) | Nenndicke nach IPC-4562A (zitiert von Siemens/Hargin) | µm | yes |
| t_ipc4562_mil | Same, in mil | dto. in mil | mil | yes |
| t_designation_um | Commercial thickness designation (IPC-4562 Table 1-1 designation as printed by Denkai; "35 µm per oz") | Handelsübliche Dickenbezeichnung („35 µm je oz") | µm | yes |
| t_137mil_um | Thickness by the common rule "1 oz = 1.37 mil" | Dicke nach der verbreiteten Regel „1 oz = 1,37 mil" | µm | yes |
| t_137mil_mil | Same, in mil | dto. in mil | mil | yes |

## Rows
```csv
weight_oz,area_weight_g_m2,t_physical_um,t_ipc4562_um,t_ipc4562_mil,t_designation_um,t_137mil_um,t_137mil_mil
0.25,76.29,8.54,,,9,8.70,0.3425
0.375,114.43,12.81,,,12,13.05,0.51375
0.5,152.58,17.08,17.1,0.68,18,17.40,0.685
1,305.15,34.15,34.3,1.35,35,34.80,1.37
2,610.30,68.30,68.6,2.7,70,69.60,2.74
3,915.46,102.46,,,103,104.39,4.11
4,1220.61,136.61,,,,139.19,5.48
```

## Sources
1. NIST, "NIST Guide to the SI, Appendix B.9: Factors for units listed by kind of quantity or field of science": ounce (avoirdupois) per square foot → kilogram per square metre = 3.051 517 E-01; foot → metre = 3.048 E-01 (exact); square foot → square metre = 9.290 304 E-02 (exact): https://www.nist.gov/pml/special-publication-811/nist-guide-si-appendix-b-conversion-factors/nist-guide-si-appendix-b9 — accessed 2026-11-02.
2. Wikipedia, "Copper", infobox: density near room temperature (20 °C) 8.935 g/cm³: https://en.wikipedia.org/wiki/Copper — accessed 2026-11-02 (typical value; used only for t_physical_um).
3. B. Hargin, Siemens EDA blog, "Copper thickness: Closing the knowledge gap to design success" (2025-08-13), Table 1, citing IPC-4562A: 1/2 oz = 0.68 mil (17.1 µm), 1 oz = 1.35 mil (34.3 µm), 2 oz = 2.7 mil (68.6 µm); after fabrication 1 oz ≈ 1.2 mil (30 µm); notes that some sources use 35 µm: https://blogs.sw.siemens.com/electronic-systems-design/2025/08/13/copper-thickness/ — accessed 2026-11-02.
4. Denkai America, Technical Data Sheet TOB-III (Rev. 2020-05-04), "Thickness Designation / Nominal Thickness / Area Weight" referencing "IPC-4562, 1.2.5.1 Table 1-1": Q (1/4 oz) 9 µm 75.9 g/m², T (3/8 oz) 12 µm 106.8 g/m², H (1/2 oz) 18 µm 152.5 g/m², 1 oz 35 µm 305 g/m², 2 oz 70 µm 610 g/m², 3 oz 103 µm 915 g/m²: https://insulectro.com/wp-content/uploads/2021/09/Denkai-America-TOB-III-TDS.pdf — accessed 2026-11-02.
5. JLCPCB blog, "How Copper Foil Shapes PCB Performance and Manufacturing Quality": 1 oz ≈ 34.8 µm (1.37 mil), "typically rounded to 35 µm"; table 0.5 oz 17.4 µm … 4 oz 140 µm: https://jlcpcb.com/blog/pcb-copper-foil — accessed 2026-11-02.
6. AIVON blog, "Copper thickness in mils: a comparison of different PCB standards (IPC vs others)" (2026-02-19): nominal 0.5 oz ≈ 0.70 mil, 1 oz ≈ 1.37 mil, 2 oz ≈ 2.74 mil, 3 oz ≈ 4.11 mil: https://www.aivon.com/blog/pcb-knowledge/copper-thickness-in-mils-a-comparison-of-different-pcb-standards-ipc-vs-others — accessed 2026-11-02.

## Cross-check
- area_weight: arithmetic with source 1; matches source 4 for 1/2, 1, 2, 3 oz (152.5 / 305 / 610 / 915 g/m², rounded). Source 4's 1/4 oz (75.9 g/m²) and 3/8 oz (106.8 g/m²) are foil-grade values, not exact fractions of 305.15 g/m² (76.3 / 114.4) — the designations Q and T are names, not exact weights.
- Density implied by each convention (a helper script that is not part of the repository): 34.15 µm ↔ 8.935 g/cm³ (source 2); 34.3 µm (IPC-4562A, source 3) ↔ 8.90 g/cm³; 34.8 µm (1.37 mil, sources 5 and 6) ↔ 8.77 g/cm³; 35 µm (source 4 designation; ECalc's `oz` length unit = 35e-6 m) ↔ 8.72 g/cm³.
- 1.37-mil rule: sources 5 and 6 agree (34.8 µm / 1.37 mil per oz; 2.74 mil for 2 oz). Rows 0.25, 0.375 and 4 oz are arithmetic extensions of the rule.
- t_ipc4562 for 1/4, 3/8, 3 and 4 oz and t_designation for 4 oz: not given in the sources found → empty.

## Notes (en / de)
- en: "1 oz" is an area weight, not an exact thickness: 34.2 µm (pure-copper density), 34.3 µm (IPC-4562A nominal), 34.8 µm (1.37 mil rule) and 35 µm (designation) are all in use. ECalc's unit `oz` uses 35 µm. Finished copper after processing is thinner (about 30 µm for 1 oz per source 3).
- de: „1 oz" ist ein Flächengewicht, keine exakte Dicke: 34,2 µm (Dichte von reinem Kupfer), 34,3 µm (IPC-4562A), 34,8 µm (Regel 1,37 mil) und 35 µm (Handelsbezeichnung) sind gebräuchlich. ECalc rechnet mit 35 µm. Nach der Fertigung ist das Kupfer dünner (bei 1 oz etwa 30 µm).

## Suggested finder / lookups
Converter oz ↔ µm/mil with a convention selector (35 µm default, 34.3 µm IPC-4562A, 1.37 mil); feeds the IPC-2221 trace-width formula (computed by ECalc).
