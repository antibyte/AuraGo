# iec60228_cross_sections — Standard conductor cross-sections and max. resistance (IEC 60228) / Normquerschnitte und max. Leiterwiderstand (IEC 60228)

Kind: transcribed standard (IEC 60228:2004 and IEC 60228:2023 / BS EN IEC 60228:2024 — conductor tables for classes 2 and 5; table numbers differ between editions)
Guide-value note needed: no

Nominal cross-sections of the series, with the maximum DC resistance at 20 °C (Ω/km) for the most used conductor classes.
Nennquerschnitte der Reihe mit dem höchstzulässigen Gleichstromwiderstand bei 20 °C (Ω/km).

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size_mm2 | Nominal cross-sectional area | Nennquerschnitt | mm² | yes |
| preferred | yes = preferred size; no = non-preferred (1400, 1800) | ja = Vorzugsgröße; nein = nicht bevorzugt (1400, 1800) | — | no |
| r_cu_class2 | Max. resistance at 20 °C, class 2 (stranded) plain copper | Max. Widerstand bei 20 °C, Klasse 2 (mehrdrähtig), blankes Kupfer | Ω/km | yes |
| r_cu_class5 | Max. resistance at 20 °C, class 5 (flexible) plain copper | Max. Widerstand bei 20 °C, Klasse 5 (feindrähtig), blankes Kupfer | Ω/km | yes |
| r_cu_class5_coated | Max. resistance at 20 °C, class 5, metal-coated (tinned) copper | Max. Widerstand bei 20 °C, Klasse 5, metallumhüllt (verzinnt) | Ω/km | yes |
| r_al_class2 | Max. resistance at 20 °C, class 2 aluminium | Max. Widerstand bei 20 °C, Klasse 2, Aluminium | Ω/km | yes |

## Rows
```csv
size_mm2,preferred,r_cu_class2,r_cu_class5,r_cu_class5_coated,r_al_class2
0.5,yes|ja,36.0,39.0,40.1,
0.75,yes|ja,24.5,26.0,26.7,
1,yes|ja,18.1,19.5,20.0,
1.5,yes|ja,12.1,13.3,13.7,
2.5,yes|ja,7.41,7.98,8.21,
4,yes|ja,4.61,4.95,5.09,
6,yes|ja,3.08,3.30,3.39,
10,yes|ja,1.83,1.91,1.95,3.08
16,yes|ja,1.15,1.21,1.24,1.91
25,yes|ja,0.727,0.780,0.795,1.20
35,yes|ja,0.524,0.554,0.565,0.868
50,yes|ja,0.387,0.386,0.393,0.641
70,yes|ja,0.268,0.272,0.277,0.443
95,yes|ja,0.193,0.206,0.210,0.320
120,yes|ja,0.153,0.161,0.164,0.253
150,yes|ja,0.124,0.129,0.132,0.206
185,yes|ja,0.0991,0.106,0.108,0.164
240,yes|ja,0.0754,0.0801,0.0817,0.125
300,yes|ja,0.0601,0.0641,0.0654,0.100
400,yes|ja,0.0470,0.0486,0.0495,0.0778
500,yes|ja,0.0366,0.0384,0.0391,0.0605
630,yes|ja,0.0283,0.0287,0.0292,0.0469
800,yes|ja,0.0221,,,0.0367
1000,yes|ja,0.0176,,,0.0291
1200,yes|ja,0.0151,,,0.0247
1400,no|nein,0.0129,,,0.0212
1600,yes|ja,0.0113,,,0.0186
1800,no|nein,0.0101,,,0.0165
2000,yes|ja,0.0090,,,0.0149
2500,yes|ja,0.0072,,,0.0127
```

Status: r_al_class2 and r_cu_class2 for 1400–2500 mm² rest on one source (Cembre) → UNVERIFIED; all other cells verified.

## Sources
1. Cembre, General Catalogue, pages "IEC 60228 : 2004 — 11 Conductor tables" (Class 1 and Class 2: copper plain / metal-coated / aluminium; Class 5 and Class 6: copper plain / metal-coated): https://storage.electrika.com/flips/8940-cembre-16/files/assets/basic-html/page-211.html and https://storage.electrika.com/flips/8940-cembre-16/files/assets/basic-html/page-212.html — accessed 2026-11-02.
2. myCableEngineering, "Conductor Resistance: DC, AC, Temperature and IEC 60228 Values" (cites BS EN IEC 60228:2024, identical to IEC 60228:2023; classes 1/2 Cu and Al up to 1200 mm², classes 5/6 Cu up to 630 mm²): https://mycableengineering.com/conductor-resistance/ — accessed 2026-11-02.
3. Wikipedia, "IEC 60228" (list of nominal cross-sections 0.5–2500 mm², 1400 and 1800 marked non-preferred): https://en.wikipedia.org/wiki/IEC_60228 — accessed 2026-11-02.

## Cross-check
- Size list: sources 1 and 3 identical (0.5 … 2500 mm², 1400/1800 non-preferred); source 2 agrees up to 1200 mm².
- Resistances: sources 1 and 2 agree on every common cell (class 2 Cu 0.5–1200, class 2 Al 10–1200, class 5 Cu plain and metal-coated 0.5–630). Source 2 rounds class 5 25 mm² to 0.78; source 1 prints 0.780 — same value.
- Class 2 values 1400–2500 mm² (Cu and Al): source 1 only → UNVERIFIED.
- Edition: source 1 reproduces the 2004 edition, source 2 the 2023 edition; the values agree, so no change between editions is visible for these cells.
- Note: Cembre's web text mislabels the class-1/class-2 column heads; the column order was taken from the values (aluminium starts at 10 mm² with 3.08 Ω/km, matching source 2).

## Notes (en / de)
- en: Class 1 = solid, class 2 = stranded (fixed installation), class 5 = flexible, class 6 = very flexible. Class 1 copper resistances differ from class 2 from 185 mm² upwards (e.g. 185 mm²: 0.101 vs 0.0991 Ω/km); class 6 shares the class 5 values up to 300 mm².
- de: Klasse 1 = eindrähtig, Klasse 2 = mehrdrähtig (feste Verlegung), Klasse 5 = feindrähtig, Klasse 6 = feinstdrähtig. Klasse-1-Kupfer weicht ab 185 mm² von Klasse 2 ab; Klasse 6 hat bis 300 mm² die Werte von Klasse 5.

## Suggested finder / lookups
Standard-size snapping for every cable calculator: round a computed cross-section up to the next size_mm2 (skip non-preferred unless asked). Resistance lookup for voltage-drop / loop-impedance calculators.
