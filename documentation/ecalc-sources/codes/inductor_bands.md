# inductor_bands — Inductor colour code / Induktivitäts-Farbcode (Spulen)

Kind: definitional (maker / military-spec convention — IEC 60062 defines no inductor colour code)
Guide-value note needed: no

Two layouts exist and must be kept apart:
- **EIA style** (4 bands): digit, digit, multiplier, tolerance; value in µH; gold = ×0.1, silver = ×0.01 as multiplier.
- **MIL style** (MIL-PRF-15305, 5 bands): a wide silver MIL identifier band first, then three value bands and one
  tolerance band; value in µH; **gold = decimal point** (not a multiplier) when it is band 1 or 2 of the three value
  bands (values < 10 µH).

## Columns

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| colour_id | colour (same ids and names as resistor_bands) | Farbe (gleiche Kennungen/Namen wie resistor_bands) | – | no |
| digit | significant figure | Ziffer | – | yes |
| eia_multiplier | multiplier, EIA style | Multiplikator, EIA-Art | factor (×), value in µH | yes |
| eia_tol_pct | tolerance, EIA style, ± | Toleranz EIA-Art ± | % | yes |
| mil_multiplier | multiplier, MIL style (3rd value band, values ≥ 10 µH) | Multiplikator, MIL-Art (3. Wertring, Werte ≥ 10 µH) | factor (×) | yes |
| mil_role | special role in MIL style | Sonderrolle in MIL-Art | – | no (en\|de) |
| mil_tol_pct | tolerance, MIL style, ± | Toleranz MIL-Art ± | % | yes |

## Rows

```csv
colour_id,digit,eia_multiplier,eia_tol_pct,mil_multiplier,mil_role,mil_tol_pct
black,0,1,,1,,
brown,1,10,,10,,1
red,2,100,,100,,2
orange,3,1000,,1000,,3
yellow,4,,,,,
green,5,,,,,
blue,6,,,,,
violet,7,,,,,
grey,8,,,,,
white,9,,,,,
gold,,0.1,5,,"decimal point (band 1 or 2 of the 3 value bands, values < 10 µH)|Dezimalpunkt (Ring 1 oder 2 der 3 Wertringe, Werte < 10 µH)",5
silver,,0.01,10,,"MIL identifier when double width and first|MIL-Kennring, wenn doppelt breit und zuerst",10
none,,,,,,20
```

Layouts:

```csv
layout_id,bands,sequence
eia4,4,"digit 1, digit 2, multiplier, tolerance (value in µH)|Ziffer 1, Ziffer 2, Multiplikator, Toleranz (Wert in µH)"
mil5_ge10,5,"wide silver MIL identifier, digit 1, digit 2, multiplier, tolerance (value ≥ 10 µH)|breiter silberner MIL-Kennring, Ziffer 1, Ziffer 2, Multiplikator, Toleranz (Wert ≥ 10 µH)"
mil5_lt10,5,"wide silver MIL identifier, then three value bands of which band 1 or 2 is gold = decimal point and the other two are digits, then tolerance (value < 10 µH)|breiter silberner MIL-Kennring, dann drei Wertringe, davon Ring 1 oder 2 Gold = Dezimalpunkt, die anderen zwei Ziffern, dann Toleranz (Wert < 10 µH)"
```

Empty cell = not assigned in the source.

## Sources
1. Bourns, *Color Code Marking (MIL STD)* and *Color Code Marking (EIA STD)*, inductive components technical library
   (PDF created 2006-10-24; MIL table for series 9130, 9210, 9220, 9230, 9250, 9310, 8250; EIA table for series 5300,
   77F, 78F, 79F, 8230), https://www.bourns.com/docs/technical-documents/technical-library/inductive-components/publications/ColorCodeMarkings.pdf?sfvrsn=0 — accessed 2026-11-02.
2. MIL-PRF-15305F, *Coils, Fixed and Variable, Radio Frequency, General Specification for*, paragraph 3.22.3 and
   Table II, as reproduced at https://coilstransformers.emilspec.com/MIL-PRF-15305/page5.html and
   https://coilstransformers.emilspec.com/MIL-PRF-15305/page6.html — accessed 2026-11-02.
3. Token Electronics, *Inductor Color Code* (web page; "EIA Standard & Military Standard"; 270 µH ±5 % example; gold
   multiplier moves the decimal point one place left, silver two places), https://token.com.tw/inductor-color-code.htm — accessed 2026-11-02.
4. Bourns, *9230 Series – Molded Axial Inductor* datasheet (embeds the MIL colour table), https://www.bourns.com/docs/Product-Datasheets/9230_series.pdf — accessed 2026-11-02.
5. IEC 60062:2016 clause 1 Scope ("colour coding for resistors" only) — iTeh preview, see resistor_bands.md source 1.

## Cross-check
- **EIA style**: Bourns EIA table (source 1: silver ×0.01 ±10 %, gold ×0.1 ±5 %, black…orange ×1…×1000, digits 0–9) and
  Token (source 3: gold multiplier = one decimal place left, silver = two; gold tolerance ±5 %; example 270 µH ±5 % =
  red-violet-brown-gold) agree.
- **MIL style**: Bourns MIL table (source 1, also source 4) and MIL-PRF-15305F §3.22.3 (source 2) agree on: double-width
  silver MIL identifier near one end, three value bands in µH, fourth band tolerance; gold in band 1 or 2 = decimal
  point for values < 10; for ≥ 10 bands 1–2 digits, band 3 multiplier; multipliers black 1, brown 10, red 100,
  orange 1000; gold ±5 %, silver ±10 %.
- **MIL tolerances brown ±1 %, red ±2 %, orange ±3 %, none ±20 %**: only MIL-PRF-15305F Table II (source 2); the web
  copy's table text is garbled, the column assignment is my best reading → treat as **UNVERIFIED (single source,
  layout uncertain)**. Bourns lists only gold and silver tolerances.
- **EIA style "no tolerance band = ±20 %"** and EIA tolerance colours other than gold/silver: not in Bourns' EIA table →
  **UNVERIFIED**; the decoder should report "tolerance not coded" instead of guessing.
- Multipliers yellow…white are not defined for inductors in either source (values above 9.9 mH are not colour coded).

## Notes (en / de)
- en: Inductor colour codes are maker/military conventions; IEC 60062 only covers resistors. Values are in µH.
  de: Spulen-Farbcodes sind Hersteller-/Militärkonventionen; IEC 60062 gilt nur für Widerstände. Werte in µH.
- en: A wide silver first band marks a military (MIL-PRF-15305) coil; then gold inside the value bands is the decimal point.
  de: Ein breiter silberner erster Ring kennzeichnet eine Militärspule (MIL-PRF-15305); Gold in den Wertringen ist dann der
  Dezimalpunkt.
- en: Without the wide silver band (EIA style), gold in the 3rd band means ×0.1 and silver ×0.01.
  de: Ohne breiten Silberring (EIA-Art) bedeutet Gold im 3. Ring ×0,1 und Silber ×0,01.

## Decode / encode rules
1. Ask (or detect) the layout: 5 bands with a wide silver first band → MIL; 4 bands → EIA.
2. EIA: L = (10·d1 + d2) × multiplier µH; multiplier band black/brown/red/orange/gold/silver; tolerance gold ±5 %,
   silver ±10 %. Range 0.10 µH (10 × 0.01) … 99 000 µH (99 × 1000).
3. MIL ≥ 10 µH: L = (10·d1 + d2) × multiplier (black…orange). MIL < 10 µH: exactly one of value bands 1 or 2 is gold;
   gold in band 1 → L = 0.d2d3 µH; gold in band 2 → L = d1.d3 µH. Gold in band 3 is invalid in MIL style.
4. Encoder: EIA — choose multiplier so that the value is a 2-digit integer × {0.01, 0.1, 1, 10, 100, 1000}.
   MIL — < 1 µH: gold, d, d; 1…9.9 µH: d, gold, d; ≥ 10 µH: d, d, multiplier.

## Edge cases
- 6.8 µH is blue-grey-gold (EIA, gold = ×0.1) but silver(wide)-blue-gold-grey (MIL, gold = point): same colours, different order.
- Values with 3 significant figures cannot be coded; SMD/molded inductors use a numeric code instead (Token: "100" =
  10 µH, "4R7" = 4.7 µH, "R10" = 0.1 µH, "1N0" = 1 nH) — related but separate.

## Worked examples (test vectors)
| layout | bands | expected | source |
|---|---|---|---|
| MIL | silver(wide), blue, gold, grey, silver | 6.8 µH ±10 % | Bourns MIL table example |
| MIL | silver(wide), red, violet, brown, gold | 270 µH ±5 % | Bourns MIL table example |
| EIA | blue, grey, gold, silver | 6.8 µH ±10 % | Bourns EIA table example |
| EIA | red, violet, brown, gold | 270 µH ±5 % | Bourns EIA example; Token example |
| EIA | brown, black, black, silver | 10 µH ±10 % | computed |
| EIA | yellow, violet, silver, gold | 0.47 µH ±5 % | computed |
| MIL | silver(wide), gold, yellow, violet, silver | 0.47 µH ±10 % | computed (gold in band 1 = decimal point) |
| MIL | silver(wide), brown, black, red, gold | 1000 µH ±5 % | computed |
| EIA | brown, black, orange, silver | 10 000 µH = 10 mH ±10 % | computed |

## Suggested finder / lookups
- Layout toggle EIA / MIL in the calculator; decoder + encoder; show "µH" and the equivalent in nH/mH.
- Colour names: reuse Table C of resistor_bands.md.
