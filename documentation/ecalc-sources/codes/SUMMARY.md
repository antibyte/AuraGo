# codes — summary

All CSV blocks parse; computed test vectors checked with a helper script that is not part of the repository (`python -I`). | table | rows | sources | main source |
|---|---|---|---|
| resistor_bands.md | 14 colours, 4 layouts, colour names in 16 locales | 7 | IEC 60062:2016, Table 1 |
| inductor_bands.md | 13 colours, 2 layouts (EIA, MIL) | 5 | Bourns colour-code sheet, MIL-PRF-15305F |
| smd_resistor.md | 96 EIA-96 codes, 13 letters, 5 RKM letters | 9 | Yageo, TT, Venkel, Bourns, ROHM |
| capacitor.md | 5 value-code families, 21 tolerance rows, 31 voltage codes | 9 | Vishay, KEMET, TDK, Murata, Nichicon, Panasonic |
| ceramic_temp.md | 48 code symbols, 12 common codes | 6 | Vishay 23140 (EIA-198-1), Murata |

Edition followed: IEC 60062:2016 with the 2016 corrigendum and the 2019 amendment (Table 1 identical in the 2016 and
2019 previews read).

## Riskiest values / open problems

- Grey tolerance band: IEC 2016 gives ±0.01 % (orange ±0.05 %, yellow ±0.02 %); Panasonic (2025) and TT (2024) still
  print grey ±0.05 % and no tolerance for orange/yellow. Both values are in the file; a decoder should show both.
- EIA-96 letters R, S, H: sources contradict → UNVERIFIED; recommendation: do not decode them. Letters Y X A B C D E F
  agree across four makers.
- Capacitor voltage codes: IEC 60062 has no voltage code; the table is a consensus of four makers' part-numbering
  systems. Murata uses YA for 35 V; nine codes have only one source (Nichicon).
- UNVERIFIED: resistor tolerance letters W, P, L (Wikipedia only); some MIL inductor tolerances (single, badly
  extracted source); capacitor tolerance letters E, F/G in pF, H, N, S, P (one source or conflicting); rare ceramic
  letters/digits (class 1: B, L, A, V; class 2: digits 2 and 9, change letters A–F) one source each.
- Not covered (possible future tables): TCR letter codes, date codes, the two-character capacitance marking (KEMET
  "KA8").
- Colour names are the agent's own translations; the Hindi ones need a native-speaker check.
