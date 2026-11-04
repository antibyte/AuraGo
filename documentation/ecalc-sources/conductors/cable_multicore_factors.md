# cable_multicore_factors — Factors for multi-core cables with more than 3 loaded cores / Umrechnungsfaktoren für vieladrige Kabel und Leitungen

Kind: transcribed standard (DIN VDE 0298-4:2013-06 Tabelle 26)
Guide-value note needed: yes

For multi-core cables and cords with conductor cross-sections up to 10 mm² and more than three loaded cores; applied to the 3-loaded (or, for flexible cords, the listed) current-carrying capacity.
Für vieladrige Kabel und Leitungen mit Leiternennquerschnitten bis 10 mm² und mehr als drei belasteten Adern.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| loaded_cores | Number of loaded cores | Anzahl der belasteten Adern | 1 (count) | yes |
| k_air | Factor, installation in air | Umrechnungsfaktor für Verlegung in Luft | 1 (factor) | yes |
| k_ground | Factor, installation in the ground | Umrechnungsfaktor für Verlegung in Erde | 1 (factor) | yes |

## Rows
```csv
loaded_cores,k_air,k_ground
5,0.75,0.70
7,0.65,0.60
10,0.55,0.50
14,0.50,0.45
19,0.45,
24,0.40,0.35
40,0.35,
61,0.30,
```

Status: k_air 19, 40, 61 UNVERIFIED (one source). k_ground: all UNVERIFIED (one source).

## Sources
1. Lapp, "T12 Technische Tabellen — Belastbarkeit", Tabelle 12-3 (from DIN VDE 0298-4:2013-06 Tabelle 26), 5/7/10/14/24 cores, air and earth: https://contentmedia.lappcdn.com/e/lapp/fuk2Jf1cPlV6EesaZZFavA~~ — accessed 2026-11-02.
2. Bayerische Kabelwerke AG (Bayka), TD06DE "Strombelastbarkeit flexible Kupferkabel und -leitungen nach DIN VDE 0298-4", 07/2022, page 3 "Umrechnungsfaktoren für vieladrige Kabel und Leitungen … nach DIN VDE 0298-4 Tabelle 26" (in air, 5–61 cores): https://www.bayka.de/wp-content/uploads/2022/03/TD06DE-Strombelastbarkeit-flexible-Kupferkabel-und-Leitungen.pdf — accessed 2026-11-02.
3. ABB STOTZ-KONTAKT, 2CDC400027D0104 Rev. D (02/2018), Tabelle 9 (5/7/10/14 cores): https://content.voltimum.com/de/118048/2cdc400027d0104_revd_02-2018_72dpi_0.pdf — accessed 2026-11-02.

## Cross-check
- k_air 5, 7, 10, 14: sources 1, 2, 3 agree; 24: sources 1 and 2 agree; 19, 40, 61: source 2 only → UNVERIFIED.
- k_ground: source 1 only → UNVERIFIED.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Count only cores that carry load current (not PE; a neutral only if it carries current, e.g. harmonics).
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Nur stromführende Adern zählen (nicht PE; Neutralleiter nur, wenn er Strom führt, z. B. Oberschwingungen).

## Suggested finder / lookups
For a cable with N loaded cores (N > 3, ≤ 10 mm²) use the next listed row with loaded_cores ≥ N.
