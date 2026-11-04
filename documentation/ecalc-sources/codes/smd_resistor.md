# smd_resistor — SMD resistor marking codes (3-/4-digit, R/L notation, EIA-96) / SMD-Widerstandscodes (3-/4-stellig, R/L-Schreibweise, EIA-96)

Kind: definitional
Guide-value note needed: no

Three code families, all used on chip resistors:
1. **3-character code** (IEC 60062 4.2.2): two significant figures + exponent digit (E3…E24 values, typically ±5 %);
   values below 10 Ω (below 10 mΩ with L) use R (or L) as decimal point.
2. **4-character code** (IEC 60062 4.2.3): three significant figures + exponent digit (E48/E96/E192, ±1 % and better);
   values below 100 Ω use R (or L) as decimal point.
3. **EIA-96 code** (IEC 60062 Annex A "special three-character code"; Yageo/Bourns/Venkel/TT "EIA-96"): two digits =
   index into the E96 series + one multiplier letter; mainly 0603 ±1 %.

RKM letters (IEC 60062 4.2.1): **L = 10⁻³ (milli), R = 1, K = 10³, M = 10⁶, G = 10⁹** — the letter is both the decimal
point and the multiplier (e.g. 4K7 = 4.7 kΩ, 1L5 = 1.5 mΩ).

## Columns

Table A — EIA-96 index

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| code | 2-digit EIA-96 code | zweistelliger EIA-96-Code | – | no (keep leading zero) |
| value | E96 significant figures (base 100…976) | E96-Ziffernfolge (Basis 100…976) | Ω before multiplier | yes |

Table B — EIA-96 multiplier letters

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| letter | multiplier letter | Multiplikator-Buchstabe | – | no |
| factor | factor applied to the 3-figure value | Faktor auf den dreistelligen Wert | × | yes |
| status | verification status | Prüfstatus | – | no (en\|de) |

Table C — RKM letters

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| letter | RKM letter (decimal point + multiplier) | RKM-Buchstabe (Komma + Multiplikator) | – | no |
| factor | multiplier of the value in ohm | Multiplikator des Ohm-Werts | × | yes |

## Rows

Table A — EIA-96 code → E96 value (agrees with IEC 60063 E96; checked against Yageo, TT, Venkel, Bourns)

```csv
code,value
01,100
02,102
03,105
04,107
05,110
06,113
07,115
08,118
09,121
10,124
11,127
12,130
13,133
14,137
15,140
16,143
17,147
18,150
19,154
20,158
21,162
22,165
23,169
24,174
25,178
26,182
27,187
28,191
29,196
30,200
31,205
32,210
33,215
34,221
35,226
36,232
37,237
38,243
39,249
40,255
41,261
42,267
43,274
44,280
45,287
46,294
47,301
48,309
49,316
50,324
51,332
52,340
53,348
54,357
55,365
56,374
57,383
58,392
59,402
60,412
61,422
62,432
63,442
64,453
65,464
66,475
67,487
68,499
69,511
70,523
71,536
72,549
73,562
74,576
75,590
76,604
77,619
78,634
79,649
80,665
81,681
82,698
83,715
84,732
85,750
86,768
87,787
88,806
89,825
90,845
91,866
92,887
93,909
94,931
95,953
96,976
```

Table B — EIA-96 multiplier letters

```csv
letter,factor,status
Z,0.001,"two sources (Venkel table; electronics-notes) — rare|zwei Quellen (Venkel-Tabelle; electronics-notes) — selten"
Y,0.01,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
X,0.1,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
A,1,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
B,10,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
C,100,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
D,1000,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
E,10000,"verified (Yageo, Bourns, Venkel, TT)|geprüft (Yageo, Bourns, Venkel, TT)"
F,100000,"verified (Yageo, Bourns, Venkel)|geprüft (Yageo, Bourns, Venkel)"
S,,"UNVERIFIED alias — TT: S = Y (×0.01); electronics-notes: S = X (×0.1)|UNGEPRÜFTER Alias — TT: S = Y (×0,01); electronics-notes: S = X (×0,1)"
R,,"UNVERIFIED alias — TT: R = X (×0.1); electronics-notes: R = Y (×0.01)|UNGEPRÜFTER Alias — TT: R = X (×0,1); electronics-notes: R = Y (×0,01)"
H,,"UNVERIFIED alias — TT: H = B (×10); Venkel: H = ×10⁷|UNGEPRÜFTER Alias — TT: H = B (×10); Venkel: H = ×10⁷"
G,1000000,"single source (Venkel) — not used in practice|eine Quelle (Venkel) — praktisch ungenutzt"
```

Table C — RKM letters (IEC 60062 4.2.1)

```csv
letter,factor
L,0.001
R,1
K,1000
M,1000000
G,1000000000
```

## Sources
1. IEC 60062:2016 (ed. 6.0), clause 4.2 (RKM code, Tables 2–4: L/R/K/M/G letters, fixed-length 4-character RKM,
   4-significant-figure coding) and contents (Annex A "Special three-character code system for resistors", Tables A.1
   "Coding of the significant numerals of the E96 series" and A.2 "Coding of the multiplier") — iTeh preview,
   https://cdn.standards.iteh.ai/samples/20940/4ae014b83a7c49adb888d27fd4b35b6b/IEC-60062-2016.pdf — accessed 2026-11-02.
   (Annex A itself is not in the preview.)
2. ROHM, Application Note 67AN061E Rev.001 (December 2024), *Resistor: List of Nominal Resistance Values*, section 5-3
   (RKM, three-character and four-character code systems per IEC 60062 Ed. 6.0, Tables 5–7),
   https://fscdn.rohm.com/en/products/databook/applinote/passive/resistor/common/list_of_nominal_resistance_values_an-e.pdf — accessed 2026-11-02.
3. Yageo (Phycomp), *Data Sheet – Chip Resistors – Marking*, product specification V.1, Apr 02, 2008 (Table 1 resistance
   value indication, Table 2 "EIA-96 marking rule", letters X Y A B C D E F, jumper "0", examples), copy at
   https://www.rom.by/files/PYu-R_Marking_1.pdf — accessed 2026-11-02.
4. Bourns, *CRP0603 Series – Precision Chip Resistors* (PDF dated 2004-08-30), "EIA-96 Marking for CRP0603",
   letters Y X A B C D E F, https://www.bourns.com/pdfs/CRP0603.pdf — accessed 2026-11-02.
5. Venkel, *Marking codes for associated decade resistance values – 0603, ±1 %*, Rev 08/2020-A (letters A–H, X, Y, Z;
   examples 01A, 25C, 93D), https://data.venkel.com/documents/0603-series-resistor-marking-codes?force=1 — accessed 2026-11-02.
6. TT Electronics, *Technical Note TN004 – Methods for Marking Values on Resistors* (09.24), page 2 (3-/4-digit code,
   R notation, EIA-96 table with column heads "S or Y", "R or X", A, "H or B", C, D, E), URL as in resistor_bands.md source 4 — accessed 2026-11-02.
7. Electronics Notes, *Surface Mount Resistor: codes & markings*,
   https://www.electronics-notes.com/articles/electronic_components/resistors/smt-smd-resistor-codes-markings.php
   (secondary; letter table Z, Y or R, X or S, A, B or H, C…F; example 68X = 49.9 Ω) — accessed 2026-11-02.
8. Vishay, *D/CRCW e3 Standard Thick Film Chip Resistors*, document 20035, rev. 14-Apr-2026, part-number legend
   ("0000 = jumper", "0R0 = jumper", tolerance "Z = jumper") — file already/vishay-crcw — accessed 2026-11-02.
9. Wikipedia, *Resistor* (secondary; "000 and 0000 sometimes appear as values on surface-mount zero-ohm links"),
   https://en.wikipedia.org/wiki/Resistor — accessed 2026-11-02.

## Cross-check
- **E96 index (Table A)**: Yageo Table 2, TT TN004 table, Venkel chart: all 96 codes parsed and identical to the E96
  list (IEC 60063); Bourns CRP0603: 90 of 96 parsed (layout), all identical. Script: a helper script that is not part of the repository.
  (A Chinese distributor sheet, sowei.com.cn, writes the same table as 10.0…97.6 with letters shifted one decade —
  Y = 0.1, X = 1, A = 10 … — giving the same resistances; not used.)
- **Letters Y X A B C D E** agree in Yageo, Bourns, Venkel and TT; **F** in Yageo, Bourns, Venkel.
- **Z = ×0.001**: Venkel and electronics-notes agree; Yageo/Bourns/TT do not list it.
- **Aliases R, S, H — disagreement**: TT (maker) pairs S↔Y (×0.01), R↔X (×0.1), H↔B (×10); electronics-notes
  pairs R↔Y (×0.01), S↔X (×0.1), H↔B (×10); Venkel defines H = ×10⁷ and G = ×10⁶. IEC 60062 Table A.2 would settle
  it but is not accessible. → **R, S, H UNVERIFIED**; recommend the decoder rejects them (or shows both readings) —
  note also that "10R" is a valid RKM/TT notation for 10 Ω, so accepting R as an EIA-96 letter makes input ambiguous.
- **3-/4-digit codes and R notation**: IEC 60062 (Tables 2–4), ROHM 67AN061E Tables 5–7, Yageo marking sheet and TT
  TN004 agree (e.g. 472 = 4.7 kΩ, 1001 = 1.00 kΩ, 4R7 = 4.7 Ω, 31R6 = 31.6 Ω).

## Notes (en / de)
- en: The two EIA-96 digits are an index, not the value: 01 means 100, 68 means 499. de: Die zwei EIA-96-Ziffern sind ein
  Index, nicht der Wert: 01 bedeutet 100, 68 bedeutet 499.
- en: 3-digit codes: the last digit is the number of zeros, so 100 = 10 Ω, not 100 Ω. de: Dreistellige Codes: die letzte
  Ziffer ist die Anzahl Nullen, 100 = 10 Ω, nicht 100 Ω.
- en: R (and L for milliohm) marks the decimal point: 4R7 = 4.7 Ω, R47 = 0.47 Ω, 1L5 = 1.5 mΩ. de: R (und L für Milliohm)
  markiert das Komma: 4R7 = 4,7 Ω, R47 = 0,47 Ω, 1L5 = 1,5 mΩ.
- en: Very small chips (0402 and smaller) are usually unmarked. de: Sehr kleine Chips (0402 und kleiner) sind meist
  unbeschriftet.
- en: Some makers use R, S or H as extra EIA-96 letters with conflicting meanings — check the datasheet. de: Manche
  Hersteller verwenden R, S oder H als zusätzliche EIA-96-Buchstaben mit widersprüchlicher Bedeutung — Datenblatt prüfen.

## Decode / encode rules
1. Normalise: trim, uppercase (Venkel: letters may be lowercase), remove spaces/bars (Yageo underlines some 3-digit
   1 % codes; the bar has no value meaning).
2. If the code is exactly 2 digits + 1 letter from {Z,Y,X,A,B,C,D,E,F} → EIA-96: value = TableA[code] × TableB[letter].
   Codes 00 and 97–99 are invalid.
3. Else if it contains one of R/L (once) and otherwise only digits → RKM decimal point: replace R by "." (Ω) or L by "."
   (mΩ). All digits are significant. Examples 4R7, R47, 0R22, 31R6, 1L5, R020.
4. Else if all digits, length 3 → 2 significant figures + exponent; length 4 → 3 significant figures + exponent.
5. All-zero codes "0", "00", "000", "0000" and "0R0" → 0 Ω jumper.
6. Encoder: pick the family from the tolerance (≥ 2 % → 3-digit, ≤ 1 % → 4-digit, 0603 ±1 % → EIA-96 when the value is
   in E96); R notation when the value < 10 Ω (3-digit) or < 100 Ω (4-digit); EIA-96 only for E96 values between
   1.00 Ω (01Y; 0.100 Ω with the rarely used Z) and 97.6 MΩ (96F).
7. The 4-digit code cannot express values < 100 Ω without R; the 3-digit code cannot express < 10 Ω without R.

## Edge cases
- "000"/"0000"/"0": jumper (Yageo: "Jumper = 0"; Wikipedia: 000 and 0000). Part-number notation "0R0", "0000", tolerance
  letter "Z" (Vishay CRCW) also mean jumper.
- "100" is 10 Ω (3-digit), "1000" is 100 Ω (4-digit), "10R0" is 10.0 Ω. TT: "330 = 33 ohms = 33R (N.B. not 330R)".
- Yageo RL0603 low-ohm codes R01 … R91 mean milliohm values (R01 = 10 mΩ) in their RXX scheme — that is ordinary R
  notation (0.01 Ω = 10 mΩ), not a separate code.
- Yageo PR series "1m5" (lowercase m with a top bar) = 1.5 mΩ: maker-specific, IEC uses L.
- E24 values in 1 % parts may carry a 3-digit code (with a bar below on Yageo RC0603) — the code itself is ordinary.

## Worked examples (test vectors)
| code | expected | source |
|---|---|---|
| 01C | 10.0 kΩ | computed from Yageo Table 2 (100 × 100) |
| 09B | 1.21 kΩ | computed (121 × 10) |
| 07B | 1.15 kΩ | TT TN004 ("07B" = 1K15) |
| 88A | 806 Ω | Yageo marking sheet Fig. 12 |
| 68X | 49.9 Ω | electronics-notes |
| 25C | 17.8 kΩ | Venkel example |
| 93D | 909 kΩ | Venkel example |
| 01A | 100 Ω | Venkel example |
| 20E | 1.58 MΩ | computed (158 × 10⁴) |
| 01Y | 1.00 Ω | computed (100 × 0.01) |
| 472 | 4.7 kΩ | TT TN004 |
| 244 | 240 kΩ | Yageo marking sheet |
| 100 | 10 Ω | IEC/ROHM rule (10 × 10⁰) |
| 1331 | 1.33 kΩ | TT TN004 |
| 3160 | 316 Ω | Yageo marking sheet |
| 1002 | 10.0 kΩ | Yageo marking sheet Fig. 16 |
| 4R7 | 4.7 Ω | TT TN004 |
| 34R8 | 34.8 Ω | TT TN004 |
| 31R6 | 31.6 Ω | Yageo marking sheet |
| R20 | 0.20 Ω = 200 mΩ | Yageo marking sheet Fig. 14 ("R20 = 220 mΩ" printed — see note) |
| R020 | 20 mΩ | Yageo marking sheet Fig. 22 |
| 1L5 | 1.5 mΩ | IEC 60062 Table 2 |
| 000 | 0 Ω jumper | Wikipedia; Yageo "0" |

Note on R20: the Yageo sheet text reads "R20 = 220 mΩ" for Fig. 14, but its own Table 3 lists R20 = 200 mΩ; by the rule
R20 = 0.20 Ω. Use **200 mΩ**; the figure caption is a typo in the source.

## Suggested finder / lookups
- Single input box: auto-detect the family (rules 2–5) and show which rule was used.
- Reverse: value + tolerance → all valid markings (3-digit, 4-digit, EIA-96 if E96).
