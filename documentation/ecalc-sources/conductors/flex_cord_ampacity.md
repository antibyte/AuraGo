# flex_cord_ampacity — Current-carrying capacity of flexible cords and cables / Belastbarkeit flexibler Leitungen

Kind: transcribed standard (DIN VDE 0298-4:2013-06 Tabelle 11; rated voltage up to 1000 V and heat-resistant cords)
Guide-value note needed: yes

Reference conditions: copper, ambient 30 °C, permissible conductor temperature as for the cable type (e.g. 60 °C rubber, 70 °C PVC, higher for heat-resistant types), continuous operation. Which cable types belong to which column is listed in DIN VDE 0298-4 (Tabelle 1/Tabelle 11; not transcribed here). Heavy rubber cords for industrial use (H07RN-F, A07RN-F) have their own table (DIN VDE 0298-4 Tabelle 13, excerpted in Lapp T12 Tabelle 12-9; one source only, not included).
Bezugsbedingungen: Kupfer, Umgebung 30 °C, zulässige Leitertemperatur nach Leitungsbauart, Dauerbetrieb.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal conductor cross-section | Leiternennquerschnitt | mm² | yes |
| a_single_1 | Single-core cords (rubber, PVC, TPE, heat-resistant), in free air spaced ≥ d from surfaces; 1 loaded conductor | Einadrige Leitungen (Gummi, PVC, TPE, wärmebeständig), frei in Luft mit Abstand ≥ d; 1 belastete Ader | A | yes |
| b_household_2 | Multi-core cords for household and hand-held appliances, on or at surfaces; 2 loaded | Mehradrige Leitungen für Haus- und Handgeräte, auf oder an Flächen; 2 belastete Adern | A | yes |
| b_household_3 | Same, 3 loaded | dto., 3 belastete Adern | A | yes |
| c_multicore_2or3 | Multi-core cords other than for household/hand-held appliances (rubber, PVC, TPE, heat-resistant), on or at surfaces; 2 or 3 loaded | Mehradrige Leitungen außer für Haus- und Handgeräte, auf oder an Flächen; 2 oder 3 belastete Adern | A | yes |
| d_rubber_3 | Multi-core rubber-sheathed cables ≥ 0.6/1 kV; 3 loaded | Mehradrige Gummischlauchleitungen mind. 0,6/1 kV; 3 belastete Adern | A | yes |
| d_single_rubber_1 | Single-core special rubber-insulated cables 0.6/1 or 1.8/3 kV; 1 loaded | Einadrige Sondergummiaderleitungen 0,6/1 oder 1,8/3 kV; 1 belastete Ader | A | yes |

## Rows
```csv
size_mm2,a_single_1,b_household_2,b_household_3,c_multicore_2or3,d_rubber_3,d_single_rubber_1
0.5,12,3,3,9,,
0.75,15,6,6,12,,
1,19,10,10,15,,
1.5,24,16,16,18,23,30
2.5,32,25,20,26,30,41
4,42,32,25,34,41,55
6,54,40,,44,,
10,73,63,,61,,
16,98,,,82,,
25,129,,,108,,
35,158,,,135,,
50,198,,,168,,
70,245,,,207,,
95,292,,,250,,
120,344,,,292,,
150,391,,,335,,
185,448,,,382,,
240,528,,,453,,
300,608,,,523,,
400,726,,,,,
500,830,,,,,
```

Status:
- 0.5 mm² a_single_1 (12) and c_multicore_2or3 (9): not in DIN VDE 0298-4 Tabelle 11 (Bayka lists "–"); Lapp marks them as an extended range "in Anlehnung an" Tabelle 11; an older catalogue table gives the same numbers. Treat as extended values, flag in the UI.
- UNVERIFIED (one source): b_household_2 at 6 mm² (40) and 10 mm² (63); a_single_1 at 500 mm² (830); all d_rubber_3 and d_single_rubber_1 values.

## Sources
1. Lapp, "T12 Technische Tabellen — Belastbarkeit", Tabelle 12-1 "Strombelastbarkeit" (categories A–D, 0.08–4 mm²), "in vereinfachter Form der DIN VDE 0298 Teil 4, 2013-06, Tabelle 11 und 15 … entnommen": https://contentmedia.lappcdn.com/e/lapp/fuk2Jf1cPlV6EesaZZFavA~~ — accessed 2026-11-02.
2. Bayerische Kabelwerke AG (Bayka), TD06DE "Strombelastbarkeit flexible Kupferkabel und -leitungen nach DIN VDE 0298-4", 07/2022, "empfohlene Werte nach VDE 0298-4 Tabelle 11" (columns 2–5, 0.5–500 mm²): https://www.bayka.de/wp-content/uploads/2022/03/TD06DE-Strombelastbarkeit-flexible-Kupferkabel-und-Leitungen.pdf — accessed 2026-11-02.
3. Unattributed cable catalogue page "X 28", "Strombelastbarkeit (allgemein) für flexible Leitungen …", values "in vereinfachter Form der DIN VDE 0298 Teil 4 und DIN VDE 0100 Teil 430" (older Gruppe 1/2/3 layout; PDF dated 2008), hosted at https://www.zeuthen.desy.de/~sulanke/Projects/CTA/Dig_trigger/NectarCam/connectors/max_current_vs_mm2.pdf — Gruppe 3 (single-core in free air, spaced) and Gruppe 2 (multi-core cords) used as cross-check; accessed 2026-11-02.

## Cross-check
- a_single_1, 0.75–4 mm²: sources 1, 2, 3 agree; 6–400 mm²: sources 2 and 3 (Gruppe 3) agree; 500 mm²: source 2 only → UNVERIFIED.
- b_household_2 / b_household_3, 0.5–4 mm²: sources 1 and 2 agree; b_household_2 6 and 10 mm²: source 2 only → UNVERIFIED.
- c_multicore_2or3, 0.75–4 mm²: sources 1, 2, 3 agree; 6–300 mm²: sources 2 and 3 (Gruppe 2) agree.
- 0.5 mm²: source 2 gives no value for columns a and c; sources 1 (marked "Erweiterter Bereich … in Anlehnung an Tabelle 11") and 3 give 12 A / 9 A.
- d columns: source 1 only → UNVERIFIED.
- Source 3 is an old-layout table (pre-2003 "Gruppe" scheme); it agrees with the current Tabelle 11 columns wherever compared. The DIN VDE 0298-4 edition 2023-06 was not checked.
- Not included: Lapp 0.08–0.34 mm² values (taken from DIN VDE 0891-1, a telecom standard, not Tabelle 11).

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Correction: ambient temperature (cable_ambient_temp_factors, column by the cord's permissible conductor temperature), grouping (cable_grouping_factors) and multi-core cords (cable_multicore_factors). Cords on reels: factor 0.80 for one layer (DIN VDE 0298-4 Tabelle 27).
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. Umrechnung für Umgebungstemperatur, Häufung und vieladrige Leitungen; aufgewickelte Leitungen: Faktor 0,80 bei einer Lage (DIN VDE 0298-4 Tabelle 27).

## Suggested finder / lookups
Smallest size_mm2 in the chosen column with value × corrections ≥ load current.
