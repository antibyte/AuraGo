# cable_ambient_temp_factors — Correction factors for ambient air temperature / Umrechnungsfaktoren für abweichende Umgebungstemperatur (Luft)

Kind: transcribed standard (IEC 60364-5-52:2009 Table B.52.14; DIN VDE 0298-4:2013-06 Tabelle 17)
Guide-value note needed: yes

Applies to cables in air; reference ambient 30 °C. Multiply the tabulated current-carrying capacity by k.
Gilt für Verlegung in Luft; Bezugstemperatur 30 °C. Tabellierte Strombelastbarkeit mit k multiplizieren.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| ambient_c | Ambient air temperature | Umgebungstemperatur (Luft) | °C | yes |
| k_60 | Factor for 60 °C max. conductor temperature (e.g. rubber-insulated cords, NR/SR) — DIN VDE 0298-4 only | Faktor für 60 °C zulässige Leitertemperatur (z. B. Gummileitungen) — nur DIN VDE 0298-4 | 1 (factor) | yes |
| k_70 | Factor for 70 °C conductor (PVC) | Faktor für 70 °C Leitertemperatur (PVC) | 1 | yes |
| k_90 | Factor for 90 °C conductor (XLPE, EPR) | Faktor für 90 °C Leitertemperatur (VPE, EPR) | 1 | yes |

## Rows
```csv
ambient_c,k_60,k_70,k_90
10,1.29,1.22,1.15
15,1.22,1.17,1.12
20,1.15,1.12,1.08
25,1.08,1.06,1.04
30,1.00,1.00,1.00
35,0.91,0.94,0.96
40,0.82,0.87,0.91
45,0.71,0.79,0.87
50,0.58,0.71,0.82
55,0.41,0.61,0.76
60,,0.50,0.71
65,,0.35,0.65
70,,,0.58
75,,,0.50
80,,,0.41
85,,,0.29
```

Status: k_70 65 °C (0.35) and k_90 85 °C (0.29) are DIN VDE 0298-4 additions not in IEC B.52.14. k_60 10–30 °C, k_90 85 °C: single source → UNVERIFIED (formula-consistent, see Cross-check).

## Sources
1. Schneider Electric, Electrical Installation Guide (wiki), "General method for cable sizing", Fig. G12 "Correction factors for ambient air temperatures other than 30 °C … (from table B.52.14 of IEC 60364-5-52)", PVC and XLPE/EPR, 10–80 °C: https://www.electrical-installation.org/enwiki/General_method_for_cable_sizing — accessed 2026-11-02.
2. ABB SACE, "Electrical installation handbook Volume 2", 4th edition 2006, 1SDC010001D0204, Table 4 "Correction factor for ambient air temperature other than 30 °C" (PVC, XLPE/EPR, mineral): https://library.e.abb.com/public/561a28a17753c444c125714f00360967/1SDC010001D0204.pdf — accessed 2026-11-02.
3. ABB STOTZ-KONTAKT, 2CDC400027D0104 Rev. D (02/2018), Tabelle 5 "Umrechnungsfaktoren für von 30 °C abweichende Umgebungstemperaturen" (60/70/80/90 °C, 10–85 °C), excerpt of DIN VDE 0298-4:2013-06 (Tabelle 17/18): https://content.voltimum.com/de/118048/2cdc400027d0104_revd_02-2018_72dpi_0.pdf — accessed 2026-11-02.
4. Lapp, "T12 Technische Tabellen — Belastbarkeit", Tabelle 12-2 "Umrechnungsfaktoren" (60/70/80/85/90 °C; 30–80 °C), "in vereinfachter Form der DIN VDE 0298 Teil 4, 2013-06, Tabelle 17 entnommen": https://contentmedia.lappcdn.com/e/lapp/fuk2Jf1cPlV6EesaZZFavA~~ — accessed 2026-11-02.
5. Unattributed cable catalogue page "X 28", "Strombelastbarkeit (allgemein) für flexible Leitungen …" (PDF dated 2008; Gummi 60 °C and PVC 70 °C factors above 30 °C), hosted at https://www.zeuthen.desy.de/~sulanke/Projects/CTA/Dig_trigger/NectarCam/connectors/max_current_vs_mm2.pdf — accessed 2026-11-02.

## Cross-check
- k_70 and k_90, 10–80 °C: IEC sources 1 and 2 agree on every value; the DIN sources 3 (all) and 4 (40/50/60/70/80 °C) give the same numbers.
- k_70 at 65 °C = 0.35: sources 3 and 5 (DIN additions; IEC stops at 60 °C).
- k_90 at 85 °C = 0.29: source 3 only → UNVERIFIED.
- k_60: 35–55 °C confirmed by sources 3 and 5 (and 40/50 °C by 4); 10–25 °C only source 3 → UNVERIFIED.
- Every value equals sqrt((θc − θa)/(θc − 30)) rounded to 2 decimals (θc = 60/70/90 °C), e.g. k_60(10 °C) = 1.291, k_90(85 °C) = 0.289 (a helper script that is not part of the repository). This supports, but does not replace, the second source.
- The DIN 80 °C column (source 3) and the 85 °C column (source 4) were left out (no ECalc table uses them).

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. For cables in the ground other factors apply (reference 20 °C, IEC B.52.15). In Germany a 25 °C ambient is common practice; then k_70 = 1.06.
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Für Verlegung in Erde gelten andere Faktoren (Bezug 20 °C). Bei 25 °C Umgebungstemperatur (in Deutschland üblich) ist k_70 = 1,06.

## Suggested finder / lookups
k = value at the next listed temperature at or above the actual ambient (conservative), or linear interpolation between rows; choose the column from the insulation (PVC → k_70, XLPE/EPR → k_90, rubber cords → k_60). Empty cell = not permitted at that ambient.
