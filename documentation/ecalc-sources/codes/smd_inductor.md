# smd_inductor — SMD inductor value codes (3-digit µH code, R and N notation, tolerance letters) / SMD-Induktivitätscodes (dreistelliger µH-Code, R- und N-Schreibweise, Toleranzbuchstaben)

Kind: definitional (maker part-numbering rules and maker part tables; no standard read)
Guide-value note needed: yes (tolerance letters differ between makers, see Cross-check)

What the code is: the three-character inductance code that SMD inductor makers put into the part number and, on
larger parts, print on the body. Small chip inductors (0402 and smaller, multilayer RF parts) usually carry no value
print at all: Bourns CE1005Q shows only a dark band labelled "MARKING" on the body (page 3 drawing), not a code.

Three notations, all in the same three-character field:
1. **Three digits = µH**: two significant digits + number of zeros (101 = 10 × 10¹ µH = 100 µH).
2. **R = decimal point, value in µH**: all characters are significant (4R7 = 4.7 µH, R47 = 0.47 µH).
3. **N = decimal point, value in nH**: all characters are significant (4N7 = 4.7 nH, 47N = 47 nH).

## Which maker states which rule

| Rule | Murata (stated) | TDK FAQ (stated; read on a DigiKey mirror) | Vishay (stated) | Bourns (from part tables) |
|---|---|---|---|---|
| 3 digits, unit µH, 2 significant + number of zeros | yes: "The unit is micro-henry (µH). The first and second figures are significant digits, and the third figure expresses the number of zeros" | yes: first two digits significant, third a multiplier; 100 = 10 µH, 101 = 100 µH | not stated (all IHLP2020AB codes use R) | SRR1260 table: 100M = 10.0 µH, 101M = 100.0 µH, 102K = 1000.0 µH; drawing of the body printed "100" |
| R = decimal point, µH, all figures significant | yes: "If there is a decimal point, it is expressed by the capital letter R. In this case, all figures are significant digits." | yes: R is the decimal point; 4R7 = 4.7 µH | yes: "4R7 = 4.7 µH"; table R10 = 0.1 µH, R47 = 0.47 µH, 1R0 = 1 µH | SRR1260: 1R0Y = 1.0 µH … 8R2Y = 8.2 µH; CE1005Q: R10 = 100 nH, R12 = 120 nH, R15 = 150 nH |
| N = decimal point, unit nH | yes: below 0.1 µH "two figures and the capital letter N … unit nH … N indicates the unit of nH, and also expresses a decimal point" | not in the text read | not used | CE1005Q table (L column in nH): 0N3 = 0.3, 1N2 = 1.2, 4N7 = 4.7, 10N = 10, 47N = 47, 82N = 82 nH, then R10 = 100 nH |
| N notation only below 0.1 µH, R from 0.1 µH up | yes (stated) | – | – | CE1005Q: last N code 82N (82 nH), first R code R10 (100 nH) — consistent |

So: the 3-digit µH rule is stated by two makers (Murata, TDK) and confirmed by a third maker's table (Bourns SRR1260);
the R rule is stated by three (Murata, TDK, Vishay) and confirmed by Bourns tables; the N rule is stated by one maker
(Murata) and confirmed by a second maker's table (Bourns CE1005Q), which gives each code with its value in nH.

## Tolerance letters

| Letter | Murata LQ (stated) | Bourns CE1005Q (stated) | Bourns SRR1260 (table pairing) | Vishay IHLP2020AB-A1 (stated) | Status |
|---|---|---|---|---|---|
| W | ±0.05 nH | – | – | – | single source (Murata) |
| B | ±0.1 nH | – | – | – | single source (Murata) |
| P | – | ±0.1 nH | – | – | single source (Bourns) |
| C | ±0.2 nH | ±0.2 nH | – | – | cross-checked |
| S | ±0.3 nH | – | – | – | single source (Murata) |
| D | **±0.5 nH** | **±0.3 nH** | – | – | **CONFLICT** |
| F | ±1 % | – | – | – | single source (Murata) |
| G | ±2 % | ±2 % | – | – | cross-checked |
| H | ±3 % | ±3 % | – | – | cross-checked |
| J | ±5 % | ±5 % | – | – | cross-checked |
| K | ±10 % | – | ±10 % (121K … 102K) | – | cross-checked (Bourns by table only) |
| M | ±20 % | – | ±20 % (100M … 101M) | ±20 % | cross-checked |
| N | – | – | – | ±30 % | single source (Vishay) |
| Y | – | – | ±30 % (1R0Y … 8R2Y) | – | single source, inferred from the table |

The letters that encode ±0.1 nH, ±0.3 nH and ±30 % differ between makers, and D means ±0.5 nH at Murata but ±0.3 nH at
Bourns. A decoder must show the tolerance letter as maker-dependent, or name the maker with each reading.

## Columns (suggested data shape)

Table A — notation rules

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| pattern | code pattern | Code-Muster | – | no |
| unit | unit of the decoded value | Einheit des decodierten Werts | µH or nH | no |
| rule | how to read it | Leseregel | – | no (en\|de) |
| status | verification status | Prüfstatus | – | no (en\|de) |

Table B — tolerance letters

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| letter | tolerance letter | Toleranzbuchstabe | – | no |
| tolerance | tolerance | Toleranz | % or nH as in source | no (keep "±0.3 nH" / "±5 %") |
| makers | makers that state it | Hersteller, die ihn angeben | – | no |
| status | verification status | Prüfstatus | – | no (en\|de) |

## Rows

Table A

```csv
pattern,unit,rule,status
DDD,µH,"two significant digits, third digit = number of zeros (101 = 100 µH)|zwei gültige Ziffern, dritte Ziffer = Anzahl Nullen (101 = 100 µH)","stated by Murata and TDK; Bourns SRR1260 table agrees|von Murata und TDK angegeben; Tabelle Bourns SRR1260 stimmt überein"
xRx / Rxx / xxR,µH,"R is the decimal point, all digits significant (4R7 = 4.7 µH, R47 = 0.47 µH)|R ist das Komma, alle Ziffern gültig (4R7 = 4,7 µH, R47 = 0,47 µH)","stated by Murata, TDK, Vishay; Bourns tables agree|von Murata, TDK, Vishay angegeben; Bourns-Tabellen stimmen überein"
xNx / xxN,nH,"N is the decimal point and the unit is nH, all digits significant (4N7 = 4.7 nH, 47N = 47 nH)|N ist das Komma und die Einheit nH, alle Ziffern gültig (4N7 = 4,7 nH, 47N = 47 nH)","stated by Murata; Bourns CE1005Q table agrees|von Murata angegeben; Tabelle Bourns CE1005Q stimmt überein"
```

Table B

```csv
letter,tolerance,makers,status
W,±0.05 nH,Murata,"single source|eine Quelle"
B,±0.1 nH,Murata,"single source|eine Quelle"
P,±0.1 nH,Bourns,"single source|eine Quelle"
C,±0.2 nH,"Murata, Bourns","verified|geprüft"
S,±0.3 nH,Murata,"single source|eine Quelle"
D,"±0.5 nH (Murata) / ±0.3 nH (Bourns)","Murata, Bourns","CONFLICT — maker-dependent|WIDERSPRUCH — herstellerabhängig"
F,±1 %,Murata,"single source|eine Quelle"
G,±2 %,"Murata, Bourns","verified|geprüft"
H,±3 %,"Murata, Bourns","verified|geprüft"
J,±5 %,"Murata, Bourns","verified|geprüft"
K,±10 %,"Murata; Bourns (table)","verified|geprüft"
M,±20 %,"Murata, Vishay; Bourns (table)","verified|geprüft"
N,±30 %,Vishay,"single source|eine Quelle"
Y,±30 %,Bourns (table),"single source, inferred|eine Quelle, abgeleitet"
```

## Sources
1. Murata, "Part Numbering — RF Inductors" (LQ chip inductors: LQG, LQP, LQW, LQH), one page, no document number or
   date printed; PDF metadata created 2025-03-13. Fields ⑥ Inductance (rule text quoted above) and ⑦ Inductance
   Tolerance (B, C, D, F, G, H, J, K, M, S, W). Example part number in the field box: LQG15HN1N0S02D.
   https://pim.murata.com/asset/pim4/inductor/PARTNUMBERING_LQ3E_PDF_INDUCTOR — read the rendered page, accessed 2026-11-24.
2. Bourns, "CE1005Q Series Multilayer Chip Inductors", REV. 02/26. Pages 1–2 Electrical Specifications (part number
   with L in nH, 0N3 … 82N, R10, R12, R15; note "Underscore indicates Inductance Tolerance Code: P = ±0.1 nH, C = ±0.2 nH,
   D = ±0.3 nH, G = ±2 %, H = ±3 %, J = ±5 %"); page 3 "How to Order" (CE1005Q - 1N2 D: Series, Value Code,
   Tolerance) and the dimension drawing with "MARKING".
   https://www.bourns.com/docs/Product-Datasheets/CE1005Q.pdf — read pages 1–3, accessed 2026-11-24.
3. Bourns, "SRR1260 Series - Shielded SMD Power Inductors", REV. 03/26. Page 1 Electrical Specifications (part number
   with inductance in µH and Tol. (%): 1R0Y … 8R2Y ±30 %, 100M … 101M ±20 %, 121K … 102K ±10 %), product photo with "470"
   on the body, Product Dimensions drawing with "100" printed on the top. Page 2 has no ordering key.
   https://www.bourns.com/docs/Product-Datasheets/SRR1260.pdf — read pages 1–2 rendered, accessed 2026-11-24.
4. Vishay Dale, "IHLP® Automotive Inductors, High Saturation Series — IHLP2020AB-A1", Document Number 34711, Revision
   03-Feb-2026. Page 1 Standard Electrical Specifications (IHLP2020AB∗∗R10MA1 0.1 µH, R47 0.47, 1R0 1, 2R2 2.2, 3R3 3.3,
   4R7 4.7); page 2 Description / Global Part Number ("4R7 = 4.7 µH", "M = ± 20 %", "N = ± 30 %").
   https://www.vishay.com/docs/34711/ihlp2020ab-a1.pdf — read pages 1–2 as text, accessed 2026-11-24.
5. TDK, FAQ "Why are the characters such as "4R7" or "100" printed on some products?",
   https://product.tdk.com/en/contact/faq/inductors-0032.html — **not opened** (HTTP 403 to the fetch). Read instead as
   the copy posted by DigiKey, "TDK Inductors/Coils FAQ: Part Markings", https://forum.digikey.com/t/tdk-inductors-coils-faq-part-markings/4332
   (secondary; it links the TDK page as its source): printed numbers are the inductance in µH, first two digits
   significant, third a multiplier, R a decimal point; 4R7 = 4.7 µH, 100 = 10 µH, 101 = 100 µH. Accessed 2026-11-24.

Not opened: Coilcraft FAQ and "power inductors color coding" pages (HTTP 403); TDK catalogs on product.tdk.com
(HTTP 403); Murata LQ catalog copy on mouser.com (timeout); Taiyo Yuden — no official yuden.co.jp document found, only
distributor copies (not used). Würth Elektronik not searched (its order codes are numeric, not value codes).

## Cross-check
- 3-digit µH code: Murata and TDK state it; Bourns SRR1260 lists 100M = 10.0 µH, 101M = 100.0 µH, 102K = 1000.0 µH,
  which fits. Agreed.
- R notation: Murata, TDK and Vishay state it; Vishay's table and both Bourns tables fit. Agreed.
- N notation: stated by Murata only; Bourns CE1005Q pairs every code with its nH value (0N3 = 0.3 nH, 1N2 = 1.2 nH,
  10N = 10 nH, 47N = 47 nH) and so confirms it independently. Agreed.
- Where N stops and R starts: Murata says N is used below 0.1 µH; Bourns CE1005Q goes 82N → R10. Agreed.
- Tolerance letters: C, G, H, J agree between Murata and Bourns; K and M agree between Murata and Bourns' table; M also
  Vishay. **D disagrees** (Murata ±0.5 nH, Bourns ±0.3 nH). ±0.1 nH is B at Murata and P at Bourns; ±0.3 nH is S at
  Murata and D at Bourns; ±30 % is N at Vishay and Y at Bourns (table).

### Not cross-checked (only one source)
- Tolerance letters W (±0.05 nH), B (±0.1 nH), S (±0.3 nH), F (±1 %) — Murata only.
- Tolerance letter P (±0.1 nH) — Bourns only.
- Tolerance letter N (±30 %) — Vishay only.
- Tolerance letter Y (±30 %) — Bourns SRR1260, inferred from the table, no key printed in the pages read.
- Murata's sentence that values specified with three significant digits "may be indicated using the closest two
  digits instead" — Murata only.
- That the code is printed on the body (not only used in the part number): TDK FAQ (secondary copy) and the Bourns
  SRR1260 drawing/photo. Murata and Vishay documents read describe part numbers only.

## Notes (en / de)
- en: 3 digits are microhenry: the last digit is the number of zeros, so 100 = 10 µH and 101 = 100 µH. de: 3 Ziffern sind
  Mikrohenry: die letzte Ziffer ist die Anzahl Nullen, also 100 = 10 µH und 101 = 100 µH.
- en: R marks the decimal point in µH (4R7 = 4.7 µH, R47 = 0.47 µH); N marks the decimal point in nH (4N7 = 4.7 nH,
  47N = 47 nH). de: R markiert das Komma in µH (4R7 = 4,7 µH, R47 = 0,47 µH); N markiert das Komma in nH (4N7 = 4,7 nH,
  47N = 47 nH).
- en: Tolerance letters are not the same at every maker: D is ±0.5 nH at Murata but ±0.3 nH at Bourns; check the
  datasheet. de: Toleranzbuchstaben sind nicht bei jedem Hersteller gleich: D ist bei Murata ±0,5 nH, bei Bourns
  ±0,3 nH; Datenblatt prüfen.
- en: Small chip inductors are often not printed with a value at all. de: Kleine Chip-Induktivitäten tragen oft gar
  keinen aufgedruckten Wert.

## Decode / encode rules
1. Normalise: trim, uppercase.
2. Take the first three characters as the value code; a fourth letter, if present, is the tolerance letter
   (Bourns "1N2D", "100M"; Vishay "4R7M"). Do not read a trailing N after a full 3-character value as nH: at Vishay
   N is the ±30 % tolerance ("4R7N" would be 4.7 µH ±30 %).
3. Exactly one N among digits → nH, N is the decimal point (47N = 47 nH, 4N7 = 4.7 nH, 0N3 = 0.3 nH).
4. Exactly one R among digits → µH, R is the decimal point (4R7 = 4.7 µH, R47 = 0.47 µH, R10 = 0.10 µH = 100 nH).
5. Three digits → µH = first two digits × 10^(third digit) (101 = 100 µH, 102 = 1000 µH = 1 mH, 470 = 47 µH).
6. Encoder: below 0.1 µH use N in nH (two significant figures: 4N7, 47N, 82N); from 0.1 µH to below 10 µH use R
   (R10, R47, 4R7, 8R2); from 10 µH use three digits (100, 470, 101, 102).

## Edge cases
- "100" is 10 µH, not 100 µH (Bourns SRR1260-100M = 10.0 µH; TDK: 100 = 10 µH).
- "R10" is 0.10 µH = 100 nH (Vishay R10 = 0.1 µH; Bourns CE1005Q-R10 = 100 nH): the same value, two units.
- "0N3" (Bourns CE1005Q) = 0.3 nH: a leading zero is allowed in the N notation.
- Tolerance letter D: maker-dependent (see table).
- UNVERIFIED caution, not a rule: a web-search summary says Coilcraft reads its 3-digit part-number code in **nH**
  (e.g. -501 = 500 nH). The Coilcraft page could not be opened (HTTP 403). If true, "102" in a Coilcraft part number
  would be 1 µH, not 1 mH. Do not ship this; at most say "some makers (Coilcraft) use nH — check the datasheet" once it
  is read.

## Worked examples (test vectors)
| code | expected | source |
|---|---|---|
| 101 | 100 µH | Bourns SRR1260-101M = 100.0 µH (page 1); TDK FAQ copy: 101 = 100 µH |
| 100 | 10 µH | Bourns SRR1260-100M = 10.0 µH (page 1, body drawing printed "100"); TDK FAQ copy |
| 470 | 47 µH | Bourns SRR1260-470M = 47.0 µH (page 1; product photo shows "470") |
| 151 | 150 µH | Bourns SRR1260-151K = 150.0 µH ±10 % (page 1) |
| 102 | 1000 µH = 1 mH | Bourns SRR1260-102K = 1000.0 µH (page 1) |
| 4R7 | 4.7 µH | Vishay 34711 page 2 ("4R7 = 4.7 µH"); Bourns SRR1260-4R7Y = 4.7 µH; TDK FAQ copy |
| 2R2 | 2.2 µH | Vishay IHLP2020AB∗∗2R2MA1 = 2.2 µH (page 1); Bourns SRR1260-2R2Y = 2.2 µH |
| 1R0 | 1.0 µH | Vishay IHLP2020AB∗∗1R0MA1 = 1 µH (page 1); Bourns SRR1260-1R0Y = 1.0 µH |
| R47 | 0.47 µH | Vishay IHLP2020AB∗∗R47MA1 = 0.47 µH (page 1) |
| R10 | 0.10 µH = 100 nH | Vishay IHLP2020AB∗∗R10MA1 = 0.1 µH (page 1); Bourns CE1005Q-R10 = 100 nH (page 2) |
| R12 | 120 nH | Bourns CE1005Q-R12 = 120 nH (page 2) |
| 47N | 47 nH | Bourns CE1005Q-47N = 47 nH (page 2) |
| 10N | 10 nH | Bourns CE1005Q-10N = 10 nH (page 2) |
| 4N7 | 4.7 nH | Bourns CE1005Q-4N7 = 4.7 nH (page 2) |
| 1N2 | 1.2 nH | Bourns CE1005Q-1N2 = 1.2 nH (page 1; How to Order example "CE1005Q - 1N2 D") |
| 0N3 | 0.3 nH | Bourns CE1005Q-0N3 = 0.3 nH (page 1) |
| 1N0 | 1.0 nH | computed by Murata's rule; code from Murata's example part number LQG15HN1N0S02D |
| 1N2D | 1.2 nH ±0.3 nH (Bourns) | Bourns CE1005Q How to Order; with Murata's letters D would read ±0.5 nH |
| 100M | 10 µH ±20 % | Bourns SRR1260-100M (page 1) |
| 4R7M | 4.7 µH ±20 % | Vishay 34711 page 2 |

## What the engine ships (catalog/codes/smd_inductor.yaml, codes_smd_inductor.go, ecalc-codes.js)
- Value: the three notations above. Three characters, one of `ddd`, `ddR`, `dRd`, `Rdd`, `ddN`, `dNd`, `Ndd`; the value
  zero is rejected. Encoder: 0.1 µH up to below 10 µH with R, 10 µH up with three digits (exponent 0 to 9), below 0.1 µH
  with N (0.3 nH as `0N3`, 1.0 nH to 9.9 nH as `1N0` … `9N9`, 10 nH to 99 nH as `10N` … `99N`); no series snapping (the
  record gives none), a value with more than two significant digits is refused.
- Tolerance letters decoded (stated identically by two makers): C ±0.2 nH (Murata, Bourns CE1005Q), G ±2 %, H ±3 %,
  J ±5 % (Murata, Bourns CE1005Q), K ±10 % (Murata; Bourns SRR1260 by table pairing only), M ±20 % (Murata, Vishay;
  Bourns SRR1260 by table pairing).
- Tolerance letters in `not_cross_checked` (decoding answers `unsupported`, "not cross-checked"): D (conflict: ±0.5 nH
  Murata, ±0.3 nH Bourns), W, B, S, F (Murata only), P (Bourns only), N (Vishay only), Y (Bourns table only, inferred).
- The unverified Coilcraft remark (3-digit code in nH) is not shipped.

## Suggested finder / lookups
- One input box: 3 characters (+ optional tolerance letter); show which rule was used (digits / R / N).
- Show the tolerance letter with the maker name when it is one of the conflicting or single-source letters.
- Reverse: value → code (rule 6).

UNVERIFIED: TDK's own FAQ page not opened (HTTP 403), read only on DigiKey's copy; no TDK catalog read. Coilcraft
(possible nH-based 3-digit codes) not opened. Taiyo Yuden and Würth not read. Bourns' ±30 % letter Y and ±10 % letter K
are read from the table pairing (part number next to "Tol. (%)"), not from a printed key. No standard (IEC 60062) was
read for inductors. Every rule and example above was read in the documents listed.
