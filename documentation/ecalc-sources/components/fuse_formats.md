# fuse_formats — Common miniature fuse form factors / Gängige Bauformen von Feinsicherungen

Kind: typical values (maker dimensions, cross-checked between two makers)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| format | common name of the form factor | gängiger Name der Bauform | – | no |
| aliases | other names | andere Namen | – | no |
| standard | standard the cited parts are designed to | Norm, nach der die zitierten Typen gebaut sind | – | no |
| body_d_mm | body / cap diameter (nominal, or max where marked) | Körper-/Kappendurchmesser (nominal bzw. max.) | mm | yes |
| length_mm | overall length (cartridge) | Gesamtlänge (Patrone) | mm | yes |
| pin_pitch_mm | lead/pin spacing (radial types) | Raster der Anschlussdrähte (radiale Typen) | mm | yes |
| mounting | mounting style | Montageart | – | no (en/de text) |
| note | remarks | Bemerkungen | – | no (en/de text) |

## Rows
```csv
format,aliases,standard,body_d_mm,length_mm,pin_pitch_mm,mounting,note
5×20 mm,"G-fuse, 5x20",IEC 60127-2,5.2,20,,"cartridge in fuse holder (pigtail versions exist)|Patrone im Sicherungshalter (auch mit Drahtanschlüssen)","Diameter 5.2 +0.1/−0.2 mm, length 20 ±0.5 mm; most common IEC fuse in equipment|Durchmesser 5,2 +0,1/−0,2 mm, Länge 20 ±0,5 mm; häufigste IEC-Gerätesicherung"
6.3×32 mm,"1/4 x 1-1/4 in, 3AG/3AB",UL 248-14 (cited parts),6.35,31.75,,"cartridge in fuse holder|Patrone im Sicherungshalter","Littelfuse 312: 6.35 x 31.75 mm; Schurter SPT 6.3x32: Ø6.35 ±0.1 x 31.8 ±0.8 mm|Littelfuse 312: 6,35 x 31,75 mm; Schurter SPT 6.3x32: Ø6,35 ±0,1 x 31,8 ±0,8 mm"
TR5,"subminiature radial fuse, 8.5 mm",IEC 60127-3,8.5,,5.08,"radial, soldered into PCB or plugged into socket|radial, eingelötet oder in Fassung gesteckt","Body Ø 8.5 mm (max.), pins ~0.6 mm; height not reliably extracted (UNVERIFIED)|Körper Ø 8,5 mm (max.), Stifte ~0,6 mm; Höhe nicht sicher entnommen (UNVERIFIED)"
```

## Sources
1. SCHURTER, "FST 5x20 — Miniature Fuse, 5 x 20 mm, Time-Lag T, L, 250 VAC", https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_FST_5x20.pdf — dimension drawing: 20 ±0.5, Ø 5.2 +0.1 −0.2, cap length 5.1 ±0.6; accessed 2026-11-02.
2. Littelfuse, "218 Series, 5×20 mm, Time-Lag Fuse", Revised BA.08/26/20, copy https://sy-dep-epc-lpc.web.cern.ch/components/datasheets/epc-lpc%20(converters)/0218080MXP_Fuse_(Littelfuse).pdf — dimensions 20 (±0.5), Ø 5.2 +0.1/−0.2, caps 5.1; accessed 2026-11-02.
3. SCHURTER, "SPT 6.3x32 — Miniature Fuse, 6.3 x 32 mm, Time-Lag T, Sand, 250 VAC", https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_SPT_6.3x32.pdf — Ø 6.35 ±0.1, length 31.8 ±0.8; accessed 2026-11-02.
4. Littelfuse, "318 Series – 3AG Fast-Acting Axial Leaded Glass Body Fuse" datasheet incl. 312 series dimensions (©2008), copy https://docs.rs-online.com/8587/0900766b813f06e5.pdf — 312 Series 6.35 (.25") × 31.75 (1.25"); accessed 2026-11-02.
5. SCHURTER, "MST 250 — Subminiature Fuse, 8.5 mm, Time-Lag T, 250 VAC, 63 VDC" (IEC 60127-3/4), copy https://www.electrokit.com/upload/product/41015/41015115/mst250.pdf — Ø 8.5, pitch 5.08, pin Ø 0.64; accessed 2026-11-02.
6. Littelfuse, "372 Series, TR5®, Time-Lag Fuse" (designed in accordance to IEC 60127-3), Revised 12/19/13, copy https://static.chipdip.ru/lib/869/DOC003869221.pdf — Ø 8.5, pins Ø 0.6, pitch 5.08, short leads 4.3 mm; accessed 2026-11-02.

## Cross-check
- 5×20: Schurter (1) and Littelfuse (2) agree (Ø 5.2 +0.1/−0.2, 20 ±0.5 mm).
- 6.3×32: Schurter (3) Ø 6.35 ±0.1 × 31.8 ±0.8 and Littelfuse (4) 6.35 × 31.75 agree within tolerance; chosen 6.35 × 31.75 (= 1/4 × 1-1/4 in).
- TR5: Schurter MST 250 (5) and Littelfuse 372 (6) agree on Ø 8.5 mm and 5.08 mm pitch; pin Ø 0.64 vs 0.6 mm. Body height: Littelfuse text shows "max. 8", Schurter drawing values are ambiguous in the text layer (4.3 / 8.5 appear next to lead lengths) → height left empty, UNVERIFIED. "TR5" is a Littelfuse (Wickmann) trademark; Schurter's equivalent is named "Subminiature Fuse, 8.5 mm".
- Not included (no second source read): TE5, 2AG (≈4.5 × 14.5 mm), SMD chip fuses, automotive blade fuses (ISO 8820-3).

## Notes (en / de)
- en: Dimensions only — the time-current class (F, T …), rated voltage and breaking capacity are separate properties; fuses of the same size are not interchangeable unless all of them match. 6.3×32 mm fuses are mainly North-American (UL 248-14) types.
- de: Nur Abmessungen — Kennlinie (F, T …), Nennspannung und Schaltvermögen sind eigene Eigenschaften; gleich große Sicherungen sind nur austauschbar, wenn alles übereinstimmt. 6,3×32-mm-Sicherungen sind überwiegend nordamerikanische Typen (UL 248-14).

## Suggested finder / lookups
- Identify a fuse from measured diameter × length (nearest match within ±0.5 mm).
