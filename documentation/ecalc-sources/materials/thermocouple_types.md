# thermocouple_types — Thermocouple types (IEC 60584) / Thermoelement-Typen (IEC 60584)

Kind: transcribed standard (IEC 60584-1:2013 legs, ranges, tolerance classes; IEC 60584-3 and ANSI MC96.1 colours) plus formula-derived sensitivities (from the reference functions / NIST ITS-90 tables)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| type | letter designation | Typbezeichnung (Kennbuchstabe) | – | no |
| pos_leg | positive conductor (nominal composition) | positiver Schenkel (Nennzusammensetzung) | – (en\|de) | no |
| neg_leg | negative conductor | negativer Schenkel | – (en\|de) | no |
| code | short material code | Kurzzeichen der Werkstoffpaarung | – (code) | no |
| ref_t_min | lower limit of the IEC 60584-1 reference function | untere Grenze der Referenzfunktion nach IEC 60584-1 | °C | yes |
| ref_t_max | upper limit of the IEC 60584-1 reference function | obere Grenze der Referenzfunktion nach IEC 60584-1 | °C | yes |
| c1_t_min | tolerance class 1: lower limit of the range | Toleranzklasse 1: untere Bereichsgrenze | °C | yes |
| c1_t_max | tolerance class 1: upper limit | Toleranzklasse 1: obere Bereichsgrenze | °C | yes |
| c1_tol | class 1 tolerance, fixed part (the larger of c1_tol and c1_tol_frac·\|t\| applies) | Klasse-1-Grenzabweichung, fester Anteil (der größere von c1_tol und c1_tol_frac·\|t\| gilt) | °C | yes |
| c1_tol_frac | class 1 tolerance, part proportional to \|t\| | Klasse-1-Grenzabweichung, zu \|t\| proportionaler Anteil | 1 (fraction of t in °C) | yes |
| c2_t_min | tolerance class 2: lower limit | Toleranzklasse 2: untere Bereichsgrenze | °C | yes |
| c2_t_max | tolerance class 2: upper limit | Toleranzklasse 2: obere Bereichsgrenze | °C | yes |
| c2_tol | class 2 tolerance, fixed part | Klasse-2-Grenzabweichung, fester Anteil | °C | yes |
| c2_tol_frac | class 2 tolerance, part proportional to \|t\| | Klasse-2-Grenzabweichung, zu \|t\| proportionaler Anteil | 1 | yes |
| emf_100 | thermoelectric voltage at 100 °C, reference junction 0 °C | Thermospannung bei 100 °C, Vergleichsstelle 0 °C | mV | yes |
| sens_0_100 | mean sensitivity 0–100 °C = emf_100 / 100 K | mittlere Empfindlichkeit 0–100 °C = emf_100 / 100 K | µV/K | yes |
| emf_1000 | thermoelectric voltage at 1000 °C, reference junction 0 °C (empty when outside the range) | Thermospannung bei 1000 °C, Vergleichsstelle 0 °C (leer außerhalb des Bereichs) | mV | yes |
| seebeck_1000 | local sensitivity at 1000 °C = (E(1010) − E(1000)) / 10 K | örtliche Empfindlichkeit bei 1000 °C = (E(1010) − E(1000)) / 10 K | µV/K | yes |
| iec_pos | IEC 60584-3 colour of the positive core | Farbe der Plus-Ader nach IEC 60584-3 | – (en\|de) | no |
| iec_neg | IEC 60584-3 colour of the negative core | Farbe der Minus-Ader nach IEC 60584-3 | – (en\|de) | no |
| iec_sheath | IEC 60584-3 sheath colour (non-intrinsically-safe circuits) | Mantelfarbe nach IEC 60584-3 (nicht eigensichere Kreise) | – (en\|de) | no |
| ansi_pos | ANSI MC96.1 colour of the positive core | Farbe der Plus-Ader nach ANSI MC96.1 | – (en\|de) | no |
| ansi_neg | ANSI MC96.1 colour of the negative core | Farbe der Minus-Ader nach ANSI MC96.1 | – (en\|de) | no |
| ansi_ext_jacket | ANSI MC96.1 jacket, extension grade | ANSI-MC96.1-Mantel, Ausgleichs-/Verlängerungsleitung | – (en\|de) | no |
| ansi_tc_jacket | ANSI MC96.1 jacket, thermocouple grade (empty = not established) | ANSI-MC96.1-Mantel, Thermoleitung (leer = nicht festgelegt) | – (en\|de) | no |
| flags | implementer-only remarks (not user-facing) | nur für die Umsetzung (nicht anzeigen) | – (English only) | no |
| note | user-facing remark | Hinweis für den Nutzer | – (en\|de) | no |

## Rows
```csv
type,pos_leg,neg_leg,code,ref_t_min,ref_t_max,c1_t_min,c1_t_max,c1_tol,c1_tol_frac,c2_t_min,c2_t_max,c2_tol,c2_tol_frac,emf_100,sens_0_100,emf_1000,seebeck_1000,iec_pos,iec_neg,iec_sheath,ansi_pos,ansi_neg,ansi_ext_jacket,ansi_tc_jacket,flags,note
J,Iron|Eisen,Copper-nickel (constantan)|Kupfer-Nickel (Konstantan),Fe-CuNi,-210,1200,-40,750,1.5,0.004,-40,750,2.5,0.0075,5.269,52.7,57.953,59.2,black|schwarz,white|weiß,black|schwarz,white|weiß,red|rot,black|schwarz,brown|braun,,Above 760 °C the iron leg changes; the lower range may then no longer stay within tolerance.|Oberhalb 760 °C verändert sich der Eisenschenkel; der untere Bereich hält danach die Toleranz eventuell nicht mehr ein.
K,Nickel-chromium|Nickel-Chrom,Nickel-aluminium|Nickel-Aluminium,NiCr-NiAl (NiCr-Ni),-270,1300,-40,1000,1.5,0.004,-40,1200,2.5,0.0075,4.096,41.0,41.276,38.9,green|grün,white|weiß,green|grün,yellow|gelb,red|rot,yellow|gelb,brown|braun,ref_t_max: IEC 60584-1:2013 restricts the K function to 1300 °C; NIST ITS-90 tables go to 1372 °C,Most common general-purpose type.|Häufigster Universaltyp.
T,Copper|Kupfer,Copper-nickel (constantan)|Kupfer-Nickel (Konstantan),Cu-CuNi,-270,400,-40,350,0.5,0.004,-40,350,1.0,0.0075,4.279,42.8,,,brown|braun,white|weiß,brown|braun,blue|blau,red|rot,blue|blau,brown|braun,seebeck_1000 empty: range ends at 400 °C (local value at 200 °C is 53.4 µV/K),Good for low temperatures; tightest class 1 tolerance.|Gut für tiefe Temperaturen; engste Klasse-1-Toleranz.
E,Nickel-chromium|Nickel-Chrom,Copper-nickel (constantan)|Kupfer-Nickel (Konstantan),NiCr-CuNi,-270,1000,-40,800,1.5,0.004,-40,900,2.5,0.0075,6.319,63.2,76.373,,violet|violett,white|weiß,violet|violett,purple|violett,red|rot,purple|violett,brown|braun,"seebeck_1000 empty: 1000 °C is the end of the range; IEC colour name is violet, makers write purple; one source (SAB) shows pink for EX — treated as an error (three other sources say violet/purple)",Highest output of the standard types.|Höchste Thermospannung der Normtypen.
N,Nickel-chromium-silicon (Nicrosil)|Nickel-Chrom-Silizium (Nicrosil),Nickel-silicon (Nisil)|Nickel-Silizium (Nisil),NiCrSi-NiSi,-270,1300,-40,1000,1.5,0.004,-40,1200,2.5,0.0075,2.774,27.7,36.256,38.5,pink|rosa,white|weiß,pink|rosa,orange|orange,red|rot,orange|orange,brown|braun,,More stable than type K at high temperature.|Bei hohen Temperaturen stabiler als Typ K.
R,Platinum-13 % rhodium|Platin-13 % Rhodium,Platinum|Platin,PtRh13-Pt,-50,1768.1,0,1600,1.0,,0,1600,1.5,0.0025,0.647,6.5,10.506,13.2,orange|orange,white|weiß,orange|orange,black|schwarz,red|rot,green|grün,,"c1: above 1100 °C the class 1 tolerance is ±[1 + 0.003·(t − 1100)] °C (c1_tol_frac not applicable); ANSI colours are for the compensating (SX/RX) cable",Noble-metal type for high temperatures; low output.|Edelmetalltyp für hohe Temperaturen; geringe Thermospannung.
S,Platinum-10 % rhodium|Platin-10 % Rhodium,Platinum|Platin,PtRh10-Pt,-50,1768.1,0,1600,1.0,,0,1600,1.5,0.0025,0.646,6.5,9.587,11.6,orange|orange,white|weiß,orange|orange,black|schwarz,red|rot,green|grün,,"c1: above 1100 °C ±[1 + 0.003·(t − 1100)] °C; ANSI colours are for the compensating cable",Noble-metal type for high temperatures; low output.|Edelmetalltyp für hohe Temperaturen; geringe Thermospannung.
B,Platinum-30 % rhodium|Platin-30 % Rhodium,Platinum-6 % rhodium|Platin-6 % Rhodium,PtRh30-PtRh6,0,1820,,,,,600,1700,1.5,0.0025,0.033,0.3,4.834,9.2,grey|grau,white|weiß,grey|grau,grey|grau,red|rot,grey|grau,,"class 1 not defined for B; sens_0_100 is meaningless for B (output near zero below about 50 °C, so no reference-junction compensation is needed in practice); IEC grey/white/grey from EPIC Sensors chart + Letronic (search excerpt); older DIN IEC 584 tables (SAB) show grey/red and Omega's chart says no IEC code",Output is almost zero near room temperature; use above 600 °C.|Thermospannung nahe Raumtemperatur fast null; Einsatz oberhalb 600 °C.
```

## Sources
1. IEC 60584-1:2013 "Thermocouples – Part 1: EMF specifications and tolerances", Ed. 3.0 — preview pages from iTeh: Table 1 (types and positive/negative conductors), Tables 2–9 (reference-function temperature ranges: R and S −50 to 1768.1 °C, B 0 to 1820 °C, J −210 to 1200 °C, T −270 to 400 °C, E −270 to 1000 °C, K −270 to 1300 °C, N −270 to 1300 °C), foreword (merges IEC 60584-1:1995 and IEC 60584-2:1982; K function restricted to 1300 °C; "IEC 60584-3:2007 remains valid"); Table 12 (tolerances) is not in the preview — https://cdn.standards.iteh.ai/samples/18205/94ad5627a504499e94f8a247c744934d/IEC-60584-1-2013.pdf — accessed 2026-11-02.
2. NIST Standard Reference Database 60, ITS-90 Thermocouple Database (NIST Monograph 175), https://its90.nist.gov/ — pages /Table (thermoelement compositions), /RefFunctions (ranges and coefficients; NIST K range 0 to 1372 °C), and EMF tables https://its90.nist.gov/downloadFiles/type_b.tab.txt … type_t.tab.txt (rows used: 0, 100, 1000 °C and neighbours) — accessed 2026-11-02. The IEC 60584-1 coefficients in source 1 equal the NIST ones (e.g. type J a1 = 50.381 µV/K vs 5.04E-02 mV/K; type S a1 = 5.4031 vs 5.40E-03).
3. SAB Bröckskes, "Thermocouples – Limit Deviation and Tolerances" (DIN EN 60584; classes 1/2/3 for J, K, N, E, T, R, S, B) — https://sab-cable.com/cables-wires-harnessing-temperature-measurement/technical-data/temperature-measurement-technical-data-instructions/mineral-insulated-thermocouples/tolerances-of-thermocouples.html — accessed 2026-11-02.
4. TC Direct (TC Ltd), "How accurate is a thermocouple?" (IEC 60584-1 classes for K, N, T, J, R, S) — https://tcdirect.co.uk/thermocouple-sensors/thermocouple-accuracies.aspx — accessed 2026-11-02.
5. Thermosense, "Thermocouple technical reference" data sheet (IEC 60584.1/.2 classes incl. E and B) — https://www.thermosensedirect.com/media/thermoattachment/products/attachments/thermocouple_technical_reference.pdf — accessed 2026-11-02.
6. TC Direct, "What are thermocouple colour codes?" (IEC 60584-3 for K, T, J, N, R/S) — https://www.tcdirect.co.uk/thermocouple-sensors/thermocouple-colour-codes.aspx — accessed 2026-11-02.
7. Tempco, "International Color Codes for TC and Extension Grade Wires" (IEC 584-3 for J, K, E, T, N, RX, SX incl. intrinsically-safe blue jacket; read from image alt text) — https://www.tempco.com/Tempco/Resources/Engineering-Data/International-Color-Codes-for-TC-and-Extension-Grade-Wires.htm — accessed 2026-11-02.
8. EPIC Sensors (LAPP group), data sheet "Compensating cables for thermocouple sensors" with colour code chart IEC 60584-3 / DIN 43714 / ANSI MC 96.1 (incl. B grey/white/grey IEC and grey/red/grey ANSI) — https://t3.lappcdn.com/fileadmin/DAM/Epic_Sensors/PDF/2019/datasheets/EN-data/Compensating-cables.pdf — accessed 2026-11-02.
9. Thermometrics Corporation, "Thermocouple color codes" (ANSI/ASTM positive/negative, extension and thermocouple-grade jackets) — https://thermometricscorp.com/thermocouple-color-codes.html — accessed 2026-11-02.
10. Omega Engineering chart "International Thermocouple Colour Codes" as mirrored at https://www.thierry-lequeu.fr/data/th-color.htm — colours only in image file names (read via file names, not the pictures) — accessed 2026-11-02.
11. WIKA Instruments, "International Thermocouple Color Code Chart" (ANSI MC96.1 thermocouple/extension grade, IEC 584-3 incl. intrinsically safe, BS, DIN, JIS, NF) — https://blog.wika.com/us//files/2018/02/international-thermocouple-color-charts.pdf — accessed 2026-11-02 (text extraction is scrambled; used only as a supporting reference for "brown" ANSI thermocouple-grade jackets and the blue IS sheath).
12. Wikipedia, "Thermocouple", section "Types" (approximate sensitivities: K ≈ 41 µV/°C, J ≈ 50, T ≈ 43, E 68, N ≈ 39 at 900 °C, B/R/S ≈ 10) — https://en.wikipedia.org/wiki/Thermocouple — accessed 2026-11-02.
13. SAB Bröckskes, "Thermocouples compensating and extension cables – types, materials, identifications and colours" (colour column headed "DIN IEC 587"; shows B grey/red) — https://sab-cable.com/cables-wires-harnessing-temperature-measurement/technical-data/temperature-measurement-technical-data-instructions/mineral-insulated-thermocouples/thermocouples-compensating-and-extension-cables-thermocouple-types-materials-identifications-and-colours.html — accessed 2026-11-02.
14. Letronic, data sheet "Silicone BC-SSP-O 2x0,50mm²" (type BC compensating cable, grey (+) / white (−), IEC 60584-3) — seen only as web-search excerpt (page returned 404 when opened): https://letronic.de/en/produkte/datenblatt/1846/silicone-bc-ssp-o-2x050mm2-1800c-thermal-compensating-line

Arithmetic: a helper script that is not part of the repository.

## Cross-check
- Legs/compositions: IEC 60584-1:2013 Table 1 and NIST /Table agree for all eight types (NIST gives nominal % for the base-metal alloys; IEC notes that base-metal compositions are standardised only for N).
- Reference-function ranges: IEC 60584-1:2013 and NIST agree for J, T, E, N, R, S, B. K: IEC 1300 °C vs NIST 1372 °C — IEC chosen (the standard the table is named after); flagged in the row.
- EMF values / sensitivities: computed from NIST table rows (formula-derived; the IEC reference functions are identical). Second view: Wikipedia's approximate sensitivities (K 41, T 43, J ≈ 50 vs 52.7 mean 0–100 °C, E 68 vs 63.2 mean 0–100 °C — Wikipedia's figure is nearer the 100–200 °C slope of 67.9) — consistent.
- Tolerance classes: SAB (all types) and Thermosense (all types) agree on every range and tolerance; TC Direct agrees for K, N, T, J, R, S (E and B not listed there). Class 3 (−200…+40 °C for K, N, E, T; 600…1700 °C for B) exists but is not tabulated here.
- IEC 60584-3 colours: J, K, T, N, R, S agree across TC Direct, Tempco, EPIC Sensors and Omega. E violet/purple: Tempco, EPIC, Omega agree; SAB shows pink for EX — disagreement, the three-source value chosen. B: EPIC Sensors chart (fetched) and Letronic (search excerpt) give grey/white/grey; SAB (older DIN IEC 584) grey/red; Omega's chart (old IEC 584-3:1989) "no standard" — grey/white/grey chosen as the IEC 60584-3:2007 value, flagged.
- ANSI MC96.1 colours: Thermometrics and Omega agree on all types (positive, negative red, extension jacket); EPIC agrees for J, K, R, S, B, E. Thermocouple-grade jacket brown: Thermometrics (text) and WIKA (scrambled text); "not established" for R, S, B per Thermometrics.
- Edition caveat: IEC 60584-3:2007 has been replaced by IEC 60584-3:2021 (BS EN IEC 60584-3:2021 supersedes BS EN 60584-3:2008, per BSI listing seen as search excerpt; the 2021 edition adds types C and A). The 2021 colour table itself was not available; the colours above for J…B are not known to have changed, but this is NOT verified against the 2021 text.

## Notes (en / de)
- en: The thermoelectric voltage is defined with the reference (cold) junction at 0 °C; at another reference temperature subtract E(t_ref) of the same type. Sensitivity changes with temperature — use the reference function or table, not a constant, for accurate work.
- de: Die Thermospannung ist für eine Vergleichsstelle bei 0 °C festgelegt; bei anderer Vergleichsstellentemperatur E(t_ref) desselben Typs abziehen. Die Empfindlichkeit ändert sich mit der Temperatur – für genaue Messungen die Referenzfunktion oder Tabelle verwenden, nicht einen konstanten Wert.
- en: Tolerance: the larger of the fixed value (°C) and the proportional value (fraction × |t|) applies. IEC: positive core in the type colour, negative core white; intrinsically safe circuits use a blue sheath. ANSI: negative core always red.
- de: Grenzabweichung: Es gilt der größere Wert aus festem Wert (°C) und proportionalem Wert (Faktor × |t|). IEC: Plus-Ader in der Typfarbe, Minus-Ader weiß; eigensichere Kreise haben einen blauen Mantel. ANSI: Minus-Ader immer rot.
- en: Older national colour codes (DIN 43714, BS 1843, JIS, NF) differ — check the colours of an existing installation before connecting.
- de: Ältere nationale Farbcodes (DIN 43714, BS 1843, JIS, NF) weichen ab – vor dem Anschließen die Farben einer bestehenden Installation prüfen.

## Suggested finder / lookups
- Type picker for a thermocouple calculator: use `ref_t_min`/`ref_t_max` as the input range; voltage ≈ sens × ΔT only as a rough estimate (state that it is approximate).
- "Which types cover temperature t?" → rows with c2_t_min ≤ t ≤ c2_t_max.
- Tolerance at t for class c: max(c_tol, c_tol_frac · |t|) (R/S class 1 above 1100 °C: 1 + 0.003·(t − 1100)).
- Colour identification: lookup by (iec_pos, iec_sheath) or (ansi_pos, ansi_ext_jacket) → type.
