# capacitor — Capacitor value, tolerance and rated-voltage codes / Kondensator-Codes für Wert, Toleranz und Nennspannung

Kind: definitional (value code: IEC 60062 + maker part-numbering systems; voltage code: maker part-numbering systems)
Guide-value note needed: no

Three sub-tables: (A) value code rules, (B) tolerance letters, (C) two-character rated-voltage codes.

## Columns

Table A — value code families

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| family | code family id | Code-Familie | – | no |
| pattern | pattern | Muster | – | no |
| rule | decoding rule | Dekodierregel | – | no (en\|de) |
| base_unit | unit of the decoded number | Einheit des Ergebnisses | pF or µF | no |

Table B — tolerance letters

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| letter | tolerance letter | Toleranzbuchstabe | – | no |
| minus | lower deviation (absolute value) | untere Abweichung (Betrag) | unit in `unit` | yes |
| plus | upper deviation | obere Abweichung | unit in `unit` | yes |
| unit | pF (absolute) or % (relative) | pF (absolut) oder % (relativ) | – | no |
| applies | where the letter is used | Anwendungsbereich | – | no (en\|de) |
| status | sources / verification | Quellen / Prüfstatus | – | no |

Table C — rated-voltage codes

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| code | two-character voltage code | zweistelliger Spannungscode | – | no |
| volts | rated DC voltage | Nenngleichspannung | V | yes |
| sources | T = TDK, N = Nichicon, P = Panasonic, M = Murata | Quellen | – | no |
| status | verification status | Prüfstatus | – | no |

## Rows

Table A — value code families

```csv
family,pattern,rule,base_unit
pf3,"DDM (3 digits)","value = DD × 10^M pF; M = 0…6 (common). Example 104 = 10 × 10⁴ pF = 100 nF.|Wert = DD × 10^M pF; M = 0…6 (üblich). Beispiel 104 = 10 × 10⁴ pF = 100 nF.",pF
pf3_89,"DD8 / DD9","KEMET part numbers: last digit 9 = ×0.1, 8 = ×0.01 (229 = 2.2 pF, 508 = 0.5 pF). Digits 6 and 7 are not used as multipliers in this scheme (UNVERIFIED as a general rule).|KEMET-Bestellnummern: letzte Ziffer 9 = ×0,1, 8 = ×0,01 (229 = 2,2 pF, 508 = 0,5 pF). 6 und 7 werden in diesem Schema nicht als Multiplikator benutzt (als allgemeine Regel UNGEPRÜFT).",pF
pf_r,"D R D, R D D","R = decimal point, all digits significant: 0R5 = 0.5 pF, R50 = 0.50 pF, 1R0 = 1.0 pF, 2R2 = 2.2 pF.|R = Komma, alle Ziffern signifikant: 0R5 = 0,5 pF, R50 = 0,50 pF, 1R0 = 1,0 pF, 2R2 = 2,2 pF.",pF
rkm,"D p D, D n D, D µ D (u), D m D, D F D","IEC 60062 RKM for capacitors: lowercase p = 10⁻¹², n = 10⁻⁹, µ (or u) = 10⁻⁶, m = 10⁻³, F = 1 — letter = decimal point and unit prefix: 4p7 = 4.7 pF, 2n2 = 2.2 nF, 4µ7 = 4.7 µF, 1p0 = 1.0 pF.|IEC-60062-RKM für Kondensatoren: kleines p = 10⁻¹², n = 10⁻⁹, µ (oder u) = 10⁻⁶, m = 10⁻³, F = 1 — Buchstabe = Komma und Vorsatz: 4p7 = 4,7 pF, 2n2 = 2,2 nF, 4µ7 = 4,7 µF, 1p0 = 1,0 pF.",F
uf3,"DDM / DRD in aluminium electrolytic part numbers","Same digits but base unit µF: 010 = 1 µF, 2R2 = 2.2 µF, R47 = 0.47 µF, 220 = 22 µF, 221 = 220 µF, 222 = 2200 µF (Nichicon); EEE-FK1E101 → 101 = 100 µF (Panasonic).|Gleiche Ziffern, aber Basis µF: 010 = 1 µF, 2R2 = 2,2 µF, R47 = 0,47 µF, 220 = 22 µF, 221 = 220 µF, 222 = 2200 µF (Nichicon); EEE-FK1E101 → 101 = 100 µF (Panasonic).",µF
```

Table B — tolerance letters (C = nominal capacitance)

```csv
letter,minus,plus,unit,applies,status
W,0.05,0.05,pF,"C < 10 pF|C < 10 pF","verified: Murata, TDK"
B,0.1,0.1,pF,"C < 10 pF|C < 10 pF","verified: KEMET, TDK, Murata"
C,0.25,0.25,pF,"C < 10 pF|C < 10 pF","verified: KEMET, TDK, Murata, Vishay"
D,0.5,0.5,pF,"C < 10 pF|C < 10 pF","verified: KEMET, TDK, Murata, Vishay"
D,0.5,0.5,%,"C ≥ 10 pF (and resistors)|C ≥ 10 pF (und Widerstände)","Murata (capacitors); TT TN004 (resistors)"
E,0.2,0.2,pF,"C < 10 pF, TDK only|C < 10 pF, nur TDK","UNVERIFIED: TDK 2013 guide only"
F,1,1,%,"C ≥ 10 pF (and resistors)|C ≥ 10 pF (und Widerstände)","verified: KEMET, TDK, Murata"
F,1,1,pF,"C < 10 pF|C < 10 pF","UNVERIFIED: Wikipedia only"
G,2,2,%,"C ≥ 10 pF (and resistors)|C ≥ 10 pF (und Widerstände)","verified: KEMET, TDK, Murata"
G,2,2,pF,"C < 10 pF|C < 10 pF","UNVERIFIED: Wikipedia only"
H,3,3,%,"general|allgemein","UNVERIFIED: Wikipedia; Token (inductors)"
J,5,5,%,"general|allgemein","verified: KEMET, TDK, Murata, Vishay"
K,10,10,%,"general|allgemein","verified: KEMET, TDK, Murata, Vishay, Nichicon"
M,20,20,%,"general|allgemein","verified: KEMET, TDK, Murata, Vishay, Nichicon"
N,30,30,%,"general|allgemein","UNVERIFIED: Wikipedia only"
Q,10,30,%,"asymmetric (electrolytics)|asymmetrisch (Elkos)","verified: Nichicon, Wikipedia"
T,10,50,%,"asymmetric (electrolytics)|asymmetrisch (Elkos)","verified: Nichicon, Wikipedia"
S,20,50,%,"asymmetric|asymmetrisch","UNVERIFIED: Wikipedia only; Vishay uses Y for −20/+50"
Y,20,50,%,"asymmetric, Vishay Cera-Mite|asymmetrisch, Vishay Cera-Mite","maker-specific: Vishay 23140"
Z,20,80,%,"asymmetric (Y5V/Z5U ceramics)|asymmetrisch (Y5V/Z5U-Keramik)","verified: Vishay, KEMET (Y5V datasheet), TDK C series, Wikipedia"
P,0,100,%,"asymmetric, Vishay Cera-Mite|asymmetrisch, Vishay Cera-Mite","UNVERIFIED: Vishay only (P is ±0.02 % for resistors per Wikipedia)"
```

Table C — two-character rated-voltage codes (digit = decade, letter = value)

```csv
code,volts,sources,status
0E,2.5,"N M",verified
0G,4,"T N P M",verified
0L,5.5,N,"single source (Nichicon)"
0J,6.3,"T N P M",verified
1A,10,"T N P M",verified
1C,16,"T N P M",verified
1D,20,N,"single source (Nichicon)"
1E,25,"T N P M",verified
1V,35,"T N P","verified (Murata uses YA for 35 V)"
1H,50,"T N P M",verified
1J,63,"N P M",verified
1K,80,"N P M",verified
2A,100,"T N P M",verified
2Q,110,N,"single source (Nichicon)"
2B,125,N,"single source (Nichicon)"
2C,160,"N P",verified
2Z,180,N,"single source (Nichicon)"
2D,200,"T N P M",verified
2P,220,N,"single source (Nichicon)"
2E,250,"T N P M",verified
2F,315,N,"single source (Nichicon)"
2V,350,"N P",verified
2G,400,"N P",verified
2W,450,"T N P M",verified
2H,500,"N M",verified
2L,550,N,"single source (Nichicon)"
2X,600,N,"single source (Nichicon)"
2J,630,"T M",verified
3A,1000,"T M",verified
3D,2000,"T M",verified
3F,3150,"T M","verified code; Murata states 3.15 kV, TDK labels it 3 kV"
```

Maker-specific codes (do not decode as general codes): Murata **YA = DC 35 V** (plus Murata safety/AC codes BB, E2,
GB, GD, GF — not transcribed); Nichicon **W6 = 420 V**; KEMET MLCC part numbers use their own single-character voltage
digit (9 = 6.3 V, 8 = 10 V, 4 = 16 V, 3 = 25 V, 6 = 35 V, 5 = 50 V, 1 = 100 V, 2 = 200 V, A = 250 V — KEMET X7R/C0G
datasheets), unrelated to the two-character code.

## Sources
1. IEC 60062:2016 (ed. 6.0) contents: 4.3.1 multiplier code system for capacitors (Tables 7–9), 4.3.2 three-character
   code systems for capacitors (Tables 10–11), clause 5 tolerance letters (Tables 12–14); clause 6 covers only the
   dielectric material of film capacitors — **no rated-voltage code**. iTeh preview (URL in resistor_bands.md source 1) —
   tables themselves not in the preview. Accessed 2026-11-02.
2. Vishay Cera-Mite, *Ceramic DC Disc, RFI, and Safety Capacitors – General Information*, document 23140, rev. 11-Sep-2026
   ("Capacitance code system according to IEC 60062": 1p0…9p1, 100…910, 101…911 …; tolerance codes C D J K M Y Z P),
   https://www.vishay.com/docs/23140/geninfodcdiscrfisafe.pdf — accessed 2026-11-02.
3. KEMET (YAGEO Group), *Surface Mount MLCC C0G Dielectric, 10–250 VDC (Commercial Grade)*, C1003_C0G, 2/20/2025
   (ordering information: "Two significant digits and number of zeros. Use 9 for 1.0–9.9 pF, Use 8 for 0.5–.99 pF",
   examples 229, 508; tolerances B C D F G J K M); X7R C1002_X7R (2026-09-01)
   and Y5V (Z = +80 %/−20 %) datasheets — accessed 2026-11-02.
4. TDK, *Ceramic Capacitor Product Guide* (2013) — part-number legend: tolerance W B C D E F G J K M, rated-voltage code
   matrix, examples 0R2 = 0.2 pF, 103 = 10 000 pF, 105 = 1 µF — accessed 2026-11-02.
5. Murata, *Chip Multilayer Ceramic Capacitors for General* catalogue C02E21 (2017-11-27, mod. 2020-02-12), "Part
   Numbering": rated voltage codes, capacitance code ("R50 = 0.50 pF, 1R0 = 1.0 pF, 100 = 10 pF, 103 = 10 000 pF"),
   tolerance W B C D F G J K M, copy at https://static.chipdip.ru/lib/811/DOC021811649.pdf — accessed 2026-11-02.
6. Nichicon, *Aluminum Electrolytic Capacitors – Type numbering system*, CAT.8100M (PDF 2024-09-05/2025-05-30),
   rated-voltage code list 2.5…600 V, capacitance code in µF, tolerance K M Q T A,
   https://www.nichicon.com/getmedia/89511a38-f5ea-4ee9-a981-597080f9dea0/e-partnumbering-1.pdf — accessed 2026-11-02.
7. Panasonic, *Aluminum Electrolytic Capacitors (SMD Type) – Explanation of part numbers* (10-Nov-23), voltage codes
   0G…2W and the 12-figure abbreviation rule, https://industrial.panasonic.com/cdbs/www-data/pdf/RDE0000/DME0000COL90.pdf — accessed 2026-11-02.
8. Wikipedia, *RKM code* (secondary; tolerance letter table attributed to IEC 60062, lowercase p/n/µ/m/F for capacitors),
   https://en.wikipedia.org/wiki/RKM_code — accessed 2026-11-02.
9. TT Electronics TN004 (resistor tolerance letters, see resistor_bands.md) and Token Electronics *Inductor Color Code*
   (H = ±3 %) — accessed 2026-11-02.

## Cross-check
- **3-digit pF code**: Vishay (IEC 60062 table), TDK, Murata, KEMET agree (DD × 10^M pF).
- **R notation for pF**: TDK and Murata agree (0R2, R50, 1R0). **8/9 multipliers**: KEMET only (single maker), noted as
  KEMET-specific. **p/n/µ RKM**: Vishay "1p0…9p1 according to IEC 60062" + Wikipedia; IEC Tables 7–9 not readable.
- **Tolerance letters**: see `status`. Conflicts: S vs Y for −20/+50 % (Wikipedia vs Vishay); P = +100/−0 % (Vishay
  capacitors) vs P = ±0.02 % (Wikipedia, resistors); E = ±0.2 pF appears only in TDK. F/G as ±1/±2 pF below 10 pF only
  in Wikipedia; all maker tables read use F/G as %.
- **Voltage codes**: 4 maker tables (TDK 2013, Nichicon CAT.8100M, Panasonic 10-Nov-23, Murata C02E21) agree on every
  code they share; no value conflict. Coverage gaps are listed in `status`. Murata replaces 1V by **YA** for 35 V.
  IEC 60062 contains no rated-voltage code (contents of sources 1), so the scheme is a de-facto maker/JIS convention;
  I found no standard text for it → the table is a maker-consensus table, not "the standard".
- Observation (not used as data): every code fits "digit = power of ten, letter = R10-like mantissa" (A 1.0, B 1.25,
  C 1.6, D 2.0, E 2.5, F 3.15, G 4.0, H 5.0, J 6.3, K 8.0, plus Q 1.1, Z 1.8, P 2.2, V 3.5, W 4.5, L 5.5, X 6.0) — checked
  for all 31 rows with a helper script that is not part of the repository. Decode from the table, not from this pattern.

## Notes (en / de)
- en: Ceramic and film capacitors: the code is in picofarads — 104 = 100 000 pF = 100 nF. de: Keramik- und
  Folienkondensatoren: der Code ist in Pikofarad — 104 = 100 000 pF = 100 nF.
- en: Aluminium electrolytic part numbers use microfarads — 101 = 100 µF. de: Aluminium-Elko-Bestellnummern verwenden
  Mikrofarad — 101 = 100 µF.
- en: Below 10 pF tolerance letters mean absolute pF (C = ±0.25 pF), from 10 pF up they mean percent. de: Unter 10 pF
  bedeuten Toleranzbuchstaben absolute pF (C = ±0,25 pF), ab 10 pF Prozent.
- en: Voltage codes come from maker part-numbering systems; some makers deviate (Murata YA = 35 V). de: Spannungscodes
  stammen aus Hersteller-Nummernsystemen; manche weichen ab (Murata YA = 35 V).

## Decode / encode rules
1. Value: if the code contains p/n/µ/u/m/F (lowercase except F) → RKM (letter = decimal point × prefix). Else if it
   contains R → decimal point in the base unit (pF for ceramics/film, µF for electrolytic part numbers). Else 3 digits →
   DD × 10^M (M 0…6; 8 → ×0.01 and 9 → ×0.1 only if the user selects the KEMET scheme).
2. Tolerance: if C < 10 pF and letter ∈ {W,B,C,D} → absolute pF; else percent. Letter E only in TDK mode.
3. Voltage: exact lookup in Table C; unknown code → "not a standard code, check maker (e.g. Murata YA = 35 V)".
   Panasonic SMD electrolytics may drop the leading digit when the part number exceeds 12 characters (0J → J, 1A → A,
   1C → C, 1E → E, 1V → V, 1H → H) — accept a single letter only in "Panasonic" mode.
4. Encode value: pF → choose DD and M with DD ∈ 10…99; if pF < 10 use R notation (e.g. 4R7); show RKM (4n7) as an
   alternative.

## Edge cases
- "100" = 10 pF, not 100 pF; "010" = 1 µF in Nichicon µF codes; "0R5" and "R50" both = 0.5 pF.
- 3-digit code cannot express 3 significant figures (e.g. 1.5 nF ±1 % C0G written 152F is fine; 1.54 nF not codable).
- Murata: any letter other than R in the capacitance field means a non-standard part (catalogue note).
- KEMET voltage digits (3 = 25 V, 5 = 50 V …) look like numbers but are codes — do not confuse with two-character codes.

## Worked examples (test vectors)
| input | expected | source |
|---|---|---|
| 104 | 100 000 pF = 100 nF | rule (TDK/Murata/Vishay) |
| 103 | 10 000 pF = 10 nF | TDK guide; Murata |
| 105 | 1 000 000 pF = 1 µF | TDK guide |
| 100 | 10 pF | Murata |
| R50 | 0.50 pF | Murata |
| 0R2 | 0.2 pF | TDK guide |
| 229 (KEMET) | 2.2 pF | KEMET C0G ordering info |
| 508 (KEMET) | 0.5 pF | KEMET C0G ordering info |
| 4p7 | 4.7 pF | IEC RKM (Vishay 1p0 style) |
| 2n2 | 2.2 nF | IEC RKM rule |
| 221 (Nichicon, µF) | 220 µF | Nichicon CAT.8100M |
| C (C = 4.7 pF) | ±0.25 pF | KEMET/TDK/Murata/Vishay |
| K | ±10 % | all |
| Z | −20 % / +80 % | Vishay; KEMET Y5V |
| 1H | 50 V | TDK, Nichicon, Panasonic, Murata |
| 2A | 100 V | all four |
| 1V | 35 V | TDK, Nichicon, Panasonic |
| 2J | 630 V | TDK, Murata |
| YA (Murata) | 35 V | Murata (maker-specific) |
| part no. GRM188B11H102KA01D | 1H = 50 V, 102 = 1000 pF, K = ±10 % | Murata catalogue example (code fields) |
| part no. CGA5L3X7R1H105K160AE | X7R, 1H = 50 V, 105 = 1 µF, K = ±10 % | TDK guide example |

## Suggested finder / lookups
- One decoder with a "family" selector (Ceramic/Film pF · Electrolytic µF · KEMET 8/9) plus tolerance and voltage lookups.
- Reverse voltage lookup: "smallest standard code with volts ≥ V" for choosing a rating.
