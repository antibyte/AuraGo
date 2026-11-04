# components — summary

All CSV blocks parse and match the declared columns; arithmetic checked with a helper script that is not part of the repository.
The standards themselves (IEC 60127, JEITA RC-5320A,
EIA-198, IEC 60086, IEC 61960) were not read; catalogue abstracts, previews and two makers' reproductions were used.

| table_id | kind | rows | sources |
|---|---|---|---|
| smd_chip_sizes | typical (maker data) | 10 | 6 |
| mlcc_dielectrics | definitional + maker | 8 | 6 |
| fuse_speed_classes | definitional | 5 | 5 |
| fuse_prearc_iec60127_2 | transcribed standard (2 makers agree on every cell) | 30 | 4 |
| fuse_formats | typical | 3 | 6 |
| battery_chemistries | typical | 8 | 13 |
| battery_formats | typical | 10 | 9 |
| dc_barrel_connectors | transcribed standard + typical | 9 | 7 |

## UNVERIFIED cells
- Barrel connectors: EIAJ-05 inner diameter (4.4 vs Lumberg 4.3 mm); the 2.5×0.7 row (single source).
- Battery chemistries: CR cut-off 2.0 V; LFP IEC letter "IF".
- Battery formats: 14500 dimensions; all ANSI codes except AA (Wikipedia only).
- Fuse formats: TR5 body height (left empty).
- MLCC dielectrics: IEC 60384 equivalents (Wikipedia only); use texts for X8R, X7S, X7T, X6S are editorial.
- Fuse speed classes: meaning texts editorial; FF, M, TT have no sourced 10×In example (left empty).

## Disagreements resolved (details in each file)
0402 power (1/16 W vs 0.1 W); 2010/2512 voltage (200 V vs Vishay 400/500 V); 2512 size; Y5V class (KEMET III vs IEC
class 2); EIAJ-04 inner diameter (3.3 chosen over 3.4 mm); LFP charge voltage 3.6 vs 3.65 V and cut-off (2.5 V chosen,
sources 2.0–2.7 V); 9V designation 6LR61 vs 6LP3146.

## Subsets chosen
- SMD sizes: the 9 requested + 1812.
- Fuse pre-arcing: IEC 60127-2 sheets 2 and 3 only (sheets 1 and 5 not covered).
- Fuse formats: 5×20, 6.3×32, TR5 (TE5, 2AG, SMD, blade left out: no second source read).
- Battery formats: AAA, AA, C, D, 9V, CR2032, 14500, 18650, 21700, 26650 (other coin cells, LR44, CR123A left out:
  single source).
- Barrel connectors: RC-5320A types 1–5 + 5.5×2.1, 5.5×2.5, 3.5×1.35, 2.5×0.7.

## Unreachable sources
Littelfuse.com, TDK site and Mouser returned 403/timeouts — named mirrors of the same maker PDFs used. Battery
University BU-107 comparison table is an image and could not be read.
