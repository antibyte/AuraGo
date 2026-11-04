# cable_grouping_factors — Reduction factors for grouped circuits / Umrechnungsfaktoren für Häufung

Kind: transcribed standard (IEC 60364-5-52:2009 Table B.52.17; DIN VDE 0298-4:2013-06 Tabelle 21)
Guide-value note needed: yes

Factor for n circuits (one multi-core cable, or one group of 2 or 3 single-core cables, counts as one circuit), cables touching unless stated, all equally loaded. Applies to the A–F ampacity tables.
Faktor für n Stromkreise bzw. mehradrige Kabel (2 oder 3 einadrige Kabel eines Stromkreises zählen als ein Stromkreis), gleich belastet; anzuwenden auf die Strombelastbarkeit der Verlegearten A–F.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| arrangement_id | Arrangement key | Schlüssel der Verlegeanordnung | — | no |
| arrangement | Arrangement (text) | Verlegeanordnung (Text) | — | no |
| methods | Reference methods the row applies to | Gültig für Referenzverlegearten | — | no |
| n1 … n20 | Factor for 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 14, 16, 20 circuits or multi-core cables | Faktor für 1 … 20 Stromkreise bzw. mehradrige Kabel | 1 (factor) | yes |

## Rows
```csv
arrangement_id,arrangement,methods,n1,n2,n3,n4,n5,n6,n7,n8,n9,n10,n12,n14,n16,n20
bunched,"Bunched in air, on a surface, embedded or enclosed (conduit, trunking)|Gebündelt in Luft, auf/in der Wand, im Elektroinstallationsrohr oder -kanal",A1-F,1.00,0.80,0.70,0.65,0.60,0.57,0.54,0.52,0.50,0.48,0.45,0.43,0.41,0.38
wall_touching,"Single layer on wall, floor or unperforated tray, touching|Einlagig auf der Wand, dem Fußboden oder ungelochter Kabelrinne, mit Berührung",C,1.00,0.85,0.79,0.75,0.73,0.72,0.72,0.71,0.70,0.70,0.70,0.70,0.70,0.70
ceiling_touching,"Single layer fixed directly under a (wooden) ceiling, touching|Einlagig direkt unter der Decke, mit Berührung",C,0.95,0.81,0.72,0.68,0.66,0.64,0.63,0.62,0.61,0.61,0.61,0.61,0.61,0.61
perforated_tray,"Single layer on a perforated horizontal or vertical tray, touching|Einlagig auf gelochter waagerechter oder senkrechter Kabelrinne, mit Berührung",E/F,1.00,0.88,0.82,0.77,0.75,0.73,0.73,0.72,0.72,0.72,0.72,0.72,0.72,0.72
ladder,"Single layer on ladder supports or cleats, touching|Einlagig auf Kabelleiter oder Kabelschellen, mit Berührung",E/F,1.00,0.87,0.82,0.80,0.80,0.79,0.79,0.78,0.78,0.78,0.78,0.78,0.78,0.78
wall_spaced,"Single layer on wall or floor, spaced by one cable diameter d (DIN VDE 0298-4)|Einlagig auf der Wand oder dem Fußboden, Zwischenraum gleich Durchmesser d (DIN VDE 0298-4)",C,1.00,0.94,0.90,0.90,0.90,0.90,,0.90,,0.90,0.90,0.90,,
ceiling_spaced,"Single layer under the ceiling, spaced by one cable diameter d (DIN VDE 0298-4)|Einlagig unter der Decke, Zwischenraum gleich Außendurchmesser d (DIN VDE 0298-4)",C,0.95,0.85,0.85,0.85,0.85,0.85,,0.85,,0.85,0.85,0.85,,
```

Status: bunched n14 (0.43) UNVERIFIED (one source). wall_spaced and ceiling_spaced n5, n8, n12, n14 UNVERIFIED (one source). Values for n > 9 in rows wall_touching … ladder are filled from the standard's rule "no further reduction factor for more than nine circuits" (IEC) and confirmed by DIN for n = 10/12/14 where DIN lists them (wall_touching, ceiling_touching).

## Sources
1. Schneider Electric, Electrical Installation Guide (wiki), "General method for cable sizing", Fig. G16 "Reduction factors for unburied cables … (table B.52.17 of IEC 60364-5-52)", n = 1–9, 12, 16, 20: https://www.electrical-installation.org/enwiki/General_method_for_cable_sizing — accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2", 4th edition 2006, 1SDC010001D0204, Table 5 "Reduction factor for grouped cables" (same content as source 1): https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — accessed 2026-11-02.
3. ABB STOTZ-KONTAKT, 2CDC400027D0104 Rev. D (02/2018), Tabelle 7 "Umrechnungsfaktoren für Häufung von Kabeln und Leitungen mit Nennlast im Dauerbetrieb" (DIN VDE 0298-4:2013-06 excerpt), n = 1, 2, 3, 4, 5, 6, 8, 10, 12, 14: https://content.voltimum.com/de/118048/2cdc400027d0104_revd_02-2018_72dpi_0.pdf — accessed 2026-11-02.
4. Lapp, "T12 Technische Tabellen — Belastbarkeit", Tabelle 12-6 (from DIN VDE 0298-4:2013-06 Tabelle 21), n = 1, 2, 3, 4, 6, 10: https://contentmedia.lappcdn.com/e/lapp/fuk2Jf1cPlV6EesaZZFavA~~ — accessed 2026-11-02.

## Cross-check
- IEC rows (bunched, wall_touching, ceiling_touching, perforated_tray, ladder), n = 1–9, 12, 16, 20: sources 1 and 2 identical (2 is ABB, 1 is Schneider → independent).
- DIN rows and DIN columns: sources 3 and 4 agree on every common cell (n = 1, 2, 3, 4, 6, 10); DIN agrees with IEC on all common cells (bunched n = 1–6, 8, 12; wall_touching n = 1–6, 8; ceiling_touching n = 1–6, 8).
- bunched n10 = 0.48: sources 3 and 4. bunched n14 = 0.43: source 3 only → UNVERIFIED.
- DIN rows wall_spaced/ceiling_spaced: n = 5, 8, 12, 14 only in source 3 → UNVERIFIED; n7 and n9 not given by DIN (left empty).
- Not included (sources found, could be added later): tray tables IEC B.52.20/B.52.21 (multi-core / single-core on several trays; Schneider EIG Fig. G17, ABB EIH Tables 6–7, ABB DE Tabelle 8, Lapp 12-7/12-8) and buried-cable tables B.52.18/B.52.19.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Factors apply to groups of similar, equally loaded cables whose sizes differ by at most three adjacent standard sizes (IEC) / one size step (DIN). No factor is needed when the horizontal clearance between cables exceeds twice their diameter, or for a cable loaded below 30 % of its grouped rating.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Gilt für gleichartige, gleich belastete Kabel (DIN: Querschnitte höchstens eine Stufe verschieden). Kein Faktor nötig, wenn der lichte Abstand größer als der doppelte Außendurchmesser ist oder ein Leiter mit höchstens 30 % seiner Belastbarkeit belastet wird.

## Suggested finder / lookups
Pick the row from the arrangement, the column from the number of circuits; if n is not listed, use the next larger listed n (conservative). For rows wall_touching … ladder all n > 9 share the n9 value.
