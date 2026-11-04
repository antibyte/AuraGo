# fuse_prearc_iec60127_2 — Pre-arcing time limits of 5×20 mm fuses, IEC 60127-2 sheets 2 (F) and 3 (T) / Schmelzzeitgrenzen von 5×20-mm-Sicherungen, IEC 60127-2 Normblatt 2 (F) und 3 (T)

Kind: transcribed standard (values as reproduced in two makers' datasheets)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| sheet | IEC 60127-2 standard sheet | IEC-60127-2-Normblatt | – | yes |
| char | characteristic (F quick-acting, T time-lag), low breaking capacity (L) | Kennlinie (F flink, T träge), niedriges Schaltvermögen (L) | – | no |
| in_min_a | rated current range, lower end | Nennstrombereich, untere Grenze | A | yes |
| in_max_a | rated current range, upper end | Nennstrombereich, obere Grenze | A | yes |
| multiple | test current as multiple of rated current In | Prüfstrom als Vielfaches des Nennstroms In | × In | yes |
| t_min_s | minimum pre-arcing time (fuse must not open earlier); empty = no limit | minimale Schmelzzeit (darf nicht früher auslösen); leer = keine Grenze | s (converted exactly from min/s/ms) | yes |
| t_max_s | maximum pre-arcing time (fuse must open by then); empty = no limit | maximale Schmelzzeit (muss bis dahin auslösen); leer = keine Grenze | s (converted exactly) | yes |
| as_printed | the limits exactly as printed in the datasheets | Grenzen genau wie im Datenblatt gedruckt | – | no |

## Rows
```csv
sheet,char,in_min_a,in_max_a,multiple,t_min_s,t_max_s,as_printed
2,F,0.032,0.1,1.5,3600,,"60 min min."
2,F,0.032,0.1,2.1,,1800,"30 min max."
2,F,0.032,0.1,2.75,0.01,0.5,"10 ms – 500 ms"
2,F,0.032,0.1,4,0.003,0.1,"3 ms – 100 ms"
2,F,0.032,0.1,10,,0.02,"20 ms max."
2,F,0.125,6.3,1.5,3600,,"60 min min."
2,F,0.125,6.3,2.1,,1800,"30 min max."
2,F,0.125,6.3,2.75,0.05,2,"50 ms – 2 s"
2,F,0.125,6.3,4,0.01,0.3,"10 ms – 300 ms"
2,F,0.125,6.3,10,,0.02,"20 ms max."
2,F,8,10,1.5,1800,,"30 min min."
2,F,8,10,2.1,,1800,"30 min max."
2,F,8,10,2.75,0.05,2,"50 ms – 2 s"
2,F,8,10,4,0.01,0.4,"10 ms – 400 ms"
2,F,8,10,10,,0.04,"40 ms max."
3,T,0.032,0.1,1.5,3600,,"60 min min."
3,T,0.032,0.1,2.1,,120,"120 s max."
3,T,0.032,0.1,2.75,0.2,10,"200 ms – 10 s"
3,T,0.032,0.1,4,0.04,3,"40 ms – 3 s"
3,T,0.032,0.1,10,0.01,0.3,"10 ms – 300 ms"
3,T,0.125,6.3,1.5,3600,,"60 min min."
3,T,0.125,6.3,2.1,,120,"120 s max."
3,T,0.125,6.3,2.75,0.6,10,"600 ms – 10 s"
3,T,0.125,6.3,4,0.15,3,"150 ms – 3 s"
3,T,0.125,6.3,10,0.02,0.3,"20 ms – 300 ms"
3,T,8,16,1.5,1800,,"30 min min."
3,T,8,16,2.1,,120,"120 s max."
3,T,8,16,2.75,0.6,10,"600 ms – 10 s"
3,T,8,16,4,0.15,3,"150 ms – 3 s"
3,T,8,16,10,0.02,0.3,"20 ms – 300 ms"
```

## Sources
1. SCHURTER, "FSF 5x20 — Miniature Fuse, 5 x 20 mm, Quick-Acting F, L, 250 VAC", designed to IEC 60127-2/2, https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_FSF_5x20.pdf — "Pre-Arcing Time" table (rows 0.032–0.1 A, 0.125–6.3 A, 8–10 A); accessed 2026-11-02.
2. Littelfuse, "217 Series, 5×20 mm, Fast-acting Fuse", Revised GD.10/30/20, "meets the IEC 60127-2, Sheet 2 specification", copy at https://static.chipdip.ru/lib/405/DOC042405444.pdf — "Electrical Characteristics" table (150 %, 210 %, 275 %, 400 %, 1000 % of rating; ranges 0.032–0.100 A, 0.125–6.3 A, 8–15 A); accessed 2026-11-02.
3. SCHURTER, "FST 5x20 — Miniature Fuse, 5 x 20 mm, Time-Lag T, L, 250 VAC", designed to IEC 60127-2/3, https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_FST_5x20.pdf — "Pre-Arcing Time" table (rows 0.05–0.1 A, 0.125–6.3 A, 8–20 A); accessed 2026-11-02.
4. Littelfuse, "218 Series, 5×20 mm, Time-Lag Fuse", Revised BA.08/26/20, "meets the IEC 60127-2, Sheet 3 specification", copy at https://sy-dep-epc-lpc.web.cern.ch/components/datasheets/epc-lpc%20(converters)/0218080MXP_Fuse_(Littelfuse).pdf — "Electrical Characteristics" table (ranges 0.032–0.100 A, 0.125–6.3 A, 8–16 A); accessed 2026-11-02.
(The standard IEC 60127-2 itself was not available; both makers state conformity with the cited sheet.)

## Cross-check
- Sheet 2 (F): Schurter FSF (1) and Littelfuse 217 (2) agree on every cell for 0.032–0.1 A and 0.125–6.3 A. For high currents Schurter's row is 8–10 A, Littelfuse's 8–15 A; all values agree, so the table uses the overlap 8–10 A.
- Sheet 3 (T): Schurter FST (3) and Littelfuse 218 (4) agree on every cell. Low range: Schurter's row starts at 0.05 A (its product range), Littelfuse at 0.032 A — same values; table uses 0.032 A. High range: Schurter 8–20 A, Littelfuse 8–16 A — same values; table uses the overlap 8–16 A.
- The PDF text layers of both Littelfuse datasheets are column-shifted; values were mapped row by row and every mapped value matches the Schurter table, which has a clean layout.
- No UNVERIFIED cells. Not covered: sheets 1 and 5 (high breaking capacity F/T), sheets for 6.3×32 mm and IEC 60127-3/-4 types.

## Notes (en / de)
- en: Limits are pre-arcing (melting) times at room temperature for 5×20 mm glass fuses with low breaking capacity. A fuse that meets the standard opens between t_min and t_max at the given multiple. Ratings of 8 A and above extend beyond the classic standard-sheet range; check the actual datasheet.
- de: Grenzen sind Schmelzzeiten bei Raumtemperatur für 5×20-mm-Glasrohrsicherungen mit niedrigem Schaltvermögen. Eine normgerechte Sicherung löst beim angegebenen Vielfachen zwischen t_min und t_max aus. Nennströme ab 8 A gehen über den klassischen Normblattbereich hinaus; das konkrete Datenblatt prüfen.

## Suggested finder / lookups
- Given sheet/char, In and a fault current I: compute k = I / In, interpolate log(t) vs log(k) between the listed multiples for min/max bounds, report "opens between t_min and t_max" (state that interpolation is an approximation).
- Below 1.5 × In: "will not open (≥ 60 min)" for In ≤ 6.3 A.
