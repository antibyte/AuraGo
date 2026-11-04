# ceramic_temp — Ceramic capacitor temperature-characteristic codes (EIA-198) / Temperaturkennlinien-Codes für Keramikkondensatoren (EIA-198)

Kind: definitional (EIA-198 code letters, as reproduced by makers)
Guide-value note needed: no

- **Class 1** (temperature compensating, e.g. C0G): letter = significant figure of the temperature coefficient,
  digit = multiplier, letter = tolerance → TC = sig × mult ± tol in ppm/K (10⁻⁶/K).
- **Class 2** (e.g. X7R): letter = lower category temperature, digit = upper category temperature,
  letter = maximum capacitance change over that range (relative to 25 °C).

## Columns

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| position | code position (c1_sig, c1_mult, c1_tol, c2_low, c2_high, c2_change) | Code-Position | – | no |
| symbol | letter or digit | Buchstabe oder Ziffer | – | no |
| value | meaning (number) | Bedeutung (Zahl) | ppm/K, factor, °C or % (see position) | yes |
| value_minus | for asymmetric changes: lower limit | bei unsymmetrischer Änderung: untere Grenze | % | yes |
| status | verification | Prüfstatus | – | no |

Positions and units: c1_sig = significant figure (ppm/K before multiplier); c1_mult = factor (signed);
c1_tol = ± ppm/K; c2_low = °C; c2_high = °C; c2_change = ± % (or +value/−value_minus %).

## Rows

```csv
position,symbol,value,value_minus,status
c1_sig,C,0.0,,"verified: Vishay 23140, Wikipedia"
c1_sig,B,0.3,,"UNVERIFIED: Wikipedia only"
c1_sig,L,0.8,,"UNVERIFIED: Wikipedia only"
c1_sig,A,0.9,,"UNVERIFIED: Wikipedia only"
c1_sig,M,1.0,,"verified: Vishay 23140, Wikipedia"
c1_sig,P,1.5,,"verified: Vishay 23140, Wikipedia"
c1_sig,R,2.2,,"verified: Vishay 23140, Wikipedia"
c1_sig,S,3.3,,"verified: Vishay 23140, Wikipedia"
c1_sig,T,4.7,,"verified: Vishay 23140, Wikipedia"
c1_sig,V,5.6,,"UNVERIFIED: Wikipedia only"
c1_sig,U,7.5,,"verified: Vishay 23140, Wikipedia; Murata U2J = −750 ppm/K"
c1_mult,0,-1,,"verified: Vishay 23140, Wikipedia"
c1_mult,1,-10,,"verified: Vishay 23140, Wikipedia"
c1_mult,2,-100,,"verified: Vishay 23140, Wikipedia"
c1_mult,3,-1000,,"verified: Vishay 23140, Wikipedia"
c1_mult,5,1,,"verified: Vishay 23140, Wikipedia"
c1_mult,6,10,,"verified: Vishay 23140, Wikipedia"
c1_mult,7,100,,"verified: Vishay 23140, Wikipedia"
c1_mult,8,1000,,"verified: Vishay 23140, Wikipedia"
c1_tol,G,30,,"verified: Vishay 23140, Wikipedia; Murata C0G ±30"
c1_tol,H,60,,"verified: Vishay 23140, Wikipedia"
c1_tol,J,120,,"verified: Vishay 23140, Wikipedia; Murata U2J ±120"
c1_tol,K,250,,"verified: Vishay 23140, Wikipedia"
c1_tol,L,500,,"verified: Vishay 23140, Wikipedia"
c1_tol,M,1000,,"verified: Vishay 23140, Wikipedia"
c1_tol,N,2500,,"verified: Vishay 23140, Wikipedia"
c2_low,X,-55,,"verified: Vishay 23140, Wikipedia, Murata (X5R/X7R ranges)"
c2_low,Y,-30,,"verified: Vishay 23140, Wikipedia"
c2_low,Z,10,,"verified: Vishay 23140, Wikipedia; KEMET C320 (Z5U +10…+85 °C)"
c2_high,2,45,,"UNVERIFIED: Vishay 23140 only"
c2_high,4,65,,"verified: Vishay 23140, Wikipedia"
c2_high,5,85,,"verified: Vishay 23140, Wikipedia, Murata X5R"
c2_high,6,105,,"verified: Vishay 23140, Wikipedia, Murata X6S"
c2_high,7,125,,"verified: Vishay 23140, Wikipedia, Murata X7R"
c2_high,8,150,,"verified (indirect): Wikipedia; Murata X8G −55…150 °C; TDK lists X8R"
c2_high,9,200,,"UNVERIFIED: Wikipedia only"
c2_change,A,1.0,,"UNVERIFIED: Vishay 23140 only"
c2_change,B,1.5,,"UNVERIFIED: Vishay 23140 only"
c2_change,C,2.2,,"UNVERIFIED: Vishay 23140 only"
c2_change,D,3.3,,"UNVERIFIED: Vishay 23140 only"
c2_change,E,4.7,,"UNVERIFIED: Vishay 23140 only"
c2_change,F,7.5,,"UNVERIFIED: Vishay 23140 only (X5F is a Vishay product code)"
c2_change,P,10,,"verified: Vishay 23140, Wikipedia"
c2_change,R,15,,"verified: Vishay 23140, Wikipedia, Murata, KEMET X7R"
c2_change,S,22,,"verified: Vishay 23140, Wikipedia, Murata X7S/X6S"
c2_change,T,22,33,"verified: Vishay 23140, Wikipedia, Murata X7T"
c2_change,U,22,56,"verified: Vishay 23140, Wikipedia, Murata X7U, KEMET C320 Z5U"
c2_change,V,22,82,"verified: Vishay 23140, Wikipedia, TDK C series Y5V"
```

For T/U/V `value` is the upper (+) and `value_minus` the lower (−) limit, e.g. U = +22 % / −56 %.

Common codes (decoded, for display and tests):

```csv
code,class,meaning,source
C0G,1,"0 ± 30 ppm/K (NP0)|0 ± 30 ppm/K (NP0)","Murata C02E21 (−55…125 °C, ref. 25 °C); KEMET C0G datasheet; Vishay industry code NP0 = C0G"
U2J,1,"−750 ± 120 ppm/K (N750)|−750 ± 120 ppm/K (N750)","Murata C02E21; Vishay N750 = U2J"
P3K,1,"−1500 ± 250 ppm/K (N1500)|−1500 ± 250 ppm/K (N1500)","Vishay N1500 = P3K; Wikipedia"
X5R,2,"−55…+85 °C, ±15 %|−55…+85 °C, ±15 %","Murata C02E21; Vishay"
X6S,2,"−55…+105 °C, ±22 %|−55…+105 °C, ±22 %",Murata C02E21
X7R,2,"−55…+125 °C, ±15 %|−55…+125 °C, ±15 %","Murata C02E21; KEMET C1002_X7R; Vishay"
X7S,2,"−55…+125 °C, ±22 %|−55…+125 °C, ±22 %","Murata C02E21; Vishay"
X7T,2,"−55…+125 °C, +22/−33 %|−55…+125 °C, +22/−33 %",Murata C02E21
X7U,2,"−55…+125 °C, +22/−56 %|−55…+125 °C, +22/−56 %",Murata C02E21
X8R,2,"−55…+150 °C, ±15 %|−55…+150 °C, ±15 %","rule (8 = +150 °C indirect); TDK names X8R"
Y5V,2,"−30…+85 °C, +22/−82 %|−30…+85 °C, +22/−82 %","TDK C series (Y5V +22, −82 %); Vishay"
Z5U,2,"+10…+85 °C, +22/−56 %|+10…+85 °C, +22/−56 %","KEMET C320; Vishay"
```

Informative — JIS codes seen on Japanese parts (single maker source: Murata C02E21, reference temperature 20 °C):
CH 0 ± 60 ppm/K, CJ 0 ± 120 ppm/K, CK 0 ± 250 ppm/K, UJ −750 ± 120 ppm/K, SL +350 … −1000 ppm/K,
B ±10 % (−25…+85 °C), R ±15 % (−55…+125 °C).

## Sources
1. Vishay Cera-Mite, *Ceramic DC Disc, RFI, and Safety Capacitors – General Information*, document 23140,
   rev. 11-Sep-2026, page 4 "Class 1 ceramics according to EIA-198-1, -2, -3" and "Class 2 ceramics according to
   EIA-198-1, -2, -3"; page 3 industry-code table, https://www.vishay.com/docs/23140/geninfodcdiscrfisafe.pdf — accessed 2026-11-02.
2. Wikipedia, *Ceramic capacitor* (secondary; EIA RS-198 class 1 and class 2 code tables, IEC/EN 60384-8/21 class 1
   table), https://en.wikipedia.org/wiki/Ceramic_capacitor (raw wikitext) — accessed 2026-11-02.
3. Murata, *Chip Multilayer Ceramic Capacitors for General* catalogue C02E21, "Part Numbering – Temperature
   Characteristics" (EIA and JIS codes with ranges and limits), copy at https://static.chipdip.ru/lib/811/DOC021811649.pdf — accessed 2026-11-02.
4. KEMET (YAGEO Group), X7R datasheet C1002_X7R (2026-09-01) ("EIA characterizes X7R … capacitance change limited to
   ±15 % from −55 °C to +125 °C") and C0G datasheet C1003_C0G — accessed 2026-11-02.
5. KEMET, *C320 … ceramic capacitors* (radial, CERN copy): Class I/II/III descriptions, Z5U +22 %/−56 %,
   https://sy-dep-epc-lpc.web.cern.ch/components/datasheets/epc-lpc%20(converters)/C320%20CERAMIC%20CAPACITOR%20(KEMET).pdf — accessed 2026-11-02.
6. TDK, *Ceramic Capacitors C Series For General Use* (001-01 / 20011024), Y5V (+22, −82 %); TDK *Ceramic Capacitor
   Product Guide* 2013 (lists X8R) — accessed 2026-11-02.

## Cross-check
- Core letters and digits (class 1: C M P R S T U; 0 1 2 3 5 6 7 8; G…N — class 2: X Y Z; 4 5 6 7; P R S T U V) agree
  between Vishay 23140 and Wikipedia; Murata/KEMET/TDK products confirm the commonly used combinations.
- **Only in Wikipedia** (UNVERIFIED): class 1 letters B 0.3, L 0.8, A 0.9, V 5.6; class 2 digit 9 = +200 °C;
  change letter L (±15 %, +15/−40 % above 125 °C, AVX X8L — maker-specific).
- **Only in Vishay** (UNVERIFIED): class 2 digit 2 = +45 °C; change letters A…F (±1 … ±7.5 %).
- **Not in any source read**: class 1 multiplier digits 4 (−10 000) and 9 (+10 000) → leave out.
- **Industry-name conflicts**: P100 → Vishay "C0K" vs Wikipedia "M7G" (M7G = +100 ± 30 matches the letter rules, C0K
  would mean 0 ± 250); N1000 → Vishay "M3K" (rule-consistent) vs Wikipedia "Q3K" (Q is not a significant-figure
  letter); Wikipedia's N33 → "H2G" also uses a non-table letter. Only NP0 = C0G, N750 = U2J, N1500 = P3K are listed as
  verified aliases.
- Class naming: KEMET C320 calls Z5U "Class III" (older EIA grouping: changes beyond ±15 %), Vishay and Murata put
  Y5V/Z5U in class 2. The code letters are the same; the UI should say "class 2 (formerly class III for Y5V/Z5U)".
- Reference temperature: Vishay says class 1 TC values are defined from the change between +20 °C and +85 °C; Murata
  gives 25 °C reference and 25…125 °C for C0G. Shown as a note, not as data.

## Notes (en / de)
- en: C0G (NP0) is the most stable class 1 dielectric: 0 ± 30 ppm/K. de: C0G (NP0) ist das stabilste Klasse-1-Dielektrikum:
  0 ± 30 ppm/K.
- en: Class 2 codes describe only the temperature range and the change over temperature — DC bias and ageing reduce the
  capacitance further. de: Klasse-2-Codes beschreiben nur Temperaturbereich und Änderung über die Temperatur —
  Gleichspannung und Alterung verringern die Kapazität zusätzlich.
- en: X7R: −55 … +125 °C, ±15 %; Y5V: −30 … +85 °C, +22/−82 %. de: X7R: −55 … +125 °C, ±15 %; Y5V: −30 … +85 °C, +22/−82 %.
- en: Rare letters are shown with a caution where the sources disagree or only one source lists them. de: Seltene
  Buchstaben werden mit Hinweis angezeigt, wenn die Quellen abweichen oder nur eine Quelle sie nennt.

## Decode / encode rules
1. Normalise: uppercase; accept "NP0"/"NPO" as alias for C0G (and the letter O typed instead of zero in "C0G"/"COG").
2. If code[0] ∈ {X,Y,Z} and code[1] is a digit 2…9 and code[2] is a change letter → class 2.
   Otherwise if code[0] ∈ class-1 significant letters, code[1] ∈ {0,1,2,3,5,6,7,8} and code[2] ∈ {G…N} → class 1.
   (No overlap: class 1 never starts with X/Y/Z.)
3. Class 1: TC = sig × mult ppm/K, tolerance ± tol ppm/K. Example U2J: 7.5 × (−100) = −750 ± 120 ppm/K.
4. Class 2: range = [low, high] °C; change = ± value % or + value / − value_minus %.
5. Encode class 1 from TC and tolerance: find sig, mult with sig × mult = TC (prefer verified letters), tol letter
   whose value equals the tolerance; otherwise "no EIA code".

## Edge cases
- "C0G" with letter O ("COG") is common in catalogues → accept.
- Class 1 code with digit 4 or 9, or class 2 code with digit 2 or 9 / change A…F / L → decode with "rare/unverified" warning.
- Murata "X8G" and "X8L" (AVX) are maker codes outside the EIA tables (Murata marks X8G as its own code).

## Worked examples (test vectors)
| code | expected | source |
|---|---|---|
| C0G | 0 ± 30 ppm/K | Murata C02E21; KEMET C0G |
| U2J | −750 ± 120 ppm/K | Murata C02E21 |
| P3K | −1500 ± 250 ppm/K | Vishay (N1500 = P3K) |
| M3K | −1000 ± 250 ppm/K | Vishay (N1000 = M3K) |
| T3M | −4700 ± 1000 ppm/K | Vishay (N4700 = T3M) |
| S3N | −3300 ± 2500 ppm/K | Vishay (N3300 = S3N) |
| X7R | −55 … +125 °C, ±15 % | Murata; KEMET |
| X5R | −55 … +85 °C, ±15 % | Murata |
| X6S | −55 … +105 °C, ±22 % | Murata |
| X7T | −55 … +125 °C, +22/−33 % | Murata |
| Y5V | −30 … +85 °C, +22/−82 % | TDK C series |
| Z5U | +10 … +85 °C, +22/−56 % | KEMET C320 |
| Y5U | −30 … +85 °C, +22/−56 % | rule (Vishay lists Y5U) |

Computed class 1 values checked with a helper script that is not part of the repository.

## Suggested finder / lookups
- Decoder for any 3-character code; reverse finder "which class 2 codes cover −40 … +105 °C with ≤ ±15 %"
  (= X6R/X7R/X8R … from the rule; flag non-catalogue combinations as "valid code, rarely made").
