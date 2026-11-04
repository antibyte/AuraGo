# smd_chip_sizes — SMD chip package sizes (resistors/capacitors) / SMD-Chip-Bauformen (Widerstände/Kondensatoren)

Kind: typical values (package dimensions are maker data that agree closely; resistor ratings are maker-specific)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| imperial | EIA size code (inch-based, 0.01 in units) | EIA-Baugrößencode (Zoll, Einheit 0,01 in) | – | no (text, keep leading zeros) |
| metric | metric size code (L×W in 0.1 mm) | metrischer Baugrößencode (L×B in 0,1 mm) | – | no (text, keep leading zeros) |
| l_mm | nominal body length | Nennlänge des Körpers | mm | yes |
| w_mm | nominal body width | Nennbreite des Körpers | mm | yes |
| p_typ_w | typical rated power at 70 °C (consensus of the makers below, thick-film general purpose) | typische Nennleistung bei 70 °C (übereinstimmender Wert der Hersteller unten, Dickschicht Standard) | W | yes |
| v_typ | typical max. working voltage / limiting element voltage (consensus) | typische max. Betriebsspannung / Grenzspannung (übereinstimmender Wert) | V | yes |
| yageo_rc_p_w | Yageo RC_L series rated power at 70 °C (standard type) | Yageo RC_L Nennleistung bei 70 °C (Standardtyp) | W | yes |
| yageo_rc_v | Yageo RC_L max. working voltage | Yageo RC_L max. Betriebsspannung | V | yes |
| vishay_crcw_p_w | Vishay D/CRCW e3 rated dissipation P70 (standard operation mode) | Vishay D/CRCW e3 Nennverlustleistung P70 (Standardbetrieb) | W | yes |
| vishay_crcw_v | Vishay D/CRCW e3 operating voltage Umax AC RMS/DC (standard mode) | Vishay D/CRCW e3 Betriebsspannung Umax AC eff./DC (Standardbetrieb) | V | yes |
| panasonic_erj_p_w | Panasonic ERJ (precision thick film, ±1 %) rated power at 70 °C | Panasonic ERJ (Präzisions-Dickschicht, ±1 %) Nennleistung bei 70 °C | W | yes |
| panasonic_erj_v | Panasonic ERJ limiting element voltage | Panasonic ERJ Grenzspannung (limiting element voltage) | V | yes |
| note | remarks | Bemerkungen | – | no (en/de text) |

Empty cell = that maker's cited series has no part in this size (or the value was not read).

## Rows
```csv
imperial,metric,l_mm,w_mm,p_typ_w,v_typ,yageo_rc_p_w,yageo_rc_v,vishay_crcw_p_w,vishay_crcw_v,panasonic_erj_p_w,panasonic_erj_v,note
01005,0402,0.4,0.2,0.031,15,0.03125,15,,,0.031,15,"Yageo calls this size 0100 (RC0100 = 1/32 W)|Yageo nennt diese Größe 0100 (RC0100 = 1/32 W)"
0201,0603,0.6,0.3,0.05,25,0.05,25,,,0.05,25,"Vishay CRCW0201 is a separate datasheet, not read|Vishay CRCW0201 steht in einem eigenen Datenblatt, nicht gelesen"
0402,1005,1.0,0.5,0.0625,50,0.0625,50,0.063,50,0.1,50,"Power varies by maker: 1/16 W (Yageo, Vishay standard) or 0.1 W (Panasonic ERJ-2R, Vishay extended mode 0.10 W/75 V); Yageo also offers a 1/8 W type|Leistung je Hersteller: 1/16 W (Yageo, Vishay Standard) oder 0,1 W (Panasonic ERJ-2R, Vishay erweitert 0,10 W/75 V); Yageo hat auch einen 1/8-W-Typ"
0603,1608,1.6,0.8,0.1,75,0.1,75,0.1,75,0.1,75,"Vishay extended mode 0.125 W; Yageo also 1/5 W type; Vishay body 1.55 x 0.85 mm|Vishay erweitert 0,125 W; Yageo auch 1/5-W-Typ; Vishay-Körper 1,55 x 0,85 mm"
0805,2012,2.0,1.25,0.125,150,0.125,150,0.125,150,0.125,150,"Vishay extended mode 0.25 W; Yageo also 1/4 W type|Vishay erweitert 0,25 W; Yageo auch 1/4-W-Typ"
1206,3216,3.2,1.6,0.25,200,0.25,200,0.25,200,0.25,200,"Yageo body 3.10 mm long; Yageo also 1/2 W type; Panasonic ERJ-8EN is NRFND|Yageo-Körper 3,10 mm lang; Yageo auch 1/2-W-Typ; Panasonic ERJ-8EN ist NRFND"
1210,3225,3.2,2.5,0.5,200,0.5,200,0.5,200,0.5,200,"Yageo body 3.10 x 2.60 mm; Panasonic ERJ-14N is NRFND|Yageo-Körper 3,10 x 2,60 mm; Panasonic ERJ-14N ist NRFND"
1812,4532,4.5,3.2,0.75,200,,,,,0.75,200,"Common MLCC size; resistor rating from Panasonic ERJ-12N only (NRFND)|Übliche MLCC-Größe; Widerstandswerte nur von Panasonic ERJ-12N (NRFND)"
2010,5025,5.0,2.5,0.75,200,0.75,200,0.75,400,0.75,200,"Vishay allows 400 V; Panasonic ERJ-12S is NRFND|Vishay erlaubt 400 V; Panasonic ERJ-12S ist NRFND"
2512,6332,6.3,3.2,1.0,200,1.0,200,1.0,500,1.0,200,"Maker body sizes differ: 6.35 x 3.10 (Yageo), 6.3 x 3.15 (Vishay), 6.40 x 3.20 (Panasonic); Vishay allows 500 V; Yageo also 2 W type|Herstellermaße weichen ab: 6,35 x 3,10 (Yageo), 6,3 x 3,15 (Vishay), 6,40 x 3,20 (Panasonic); Vishay erlaubt 500 V; Yageo auch 2-W-Typ"
```

## Sources
1. Yageo, "Chip Resistor Surface Mount RC_L series 0075 to 2512", Product specification, V.11 (pages dated Jul. 7, 2020 / May 15, 2020), mirror of the Yageo PDF at https://static.chipdip.ru/lib/770/DOC034770119.pdf (yageo.com/yageogroup.com link returned 404) — Table 1 dimensions, Table 2 electrical characteristics, power ratings per type; accessed 2026-11-02.
2. Vishay Dale, "D/CRCW e3 Standard Thick Film Chip Resistors", Document Number 20035, Revision 14-Apr-2026, https://www.vishay.com/docs/20035/dcrcwe3.pdf — technical specifications (imperial and metric size codes RR1005M…RR6332M, P70, Umax), "Maximum resistance change at rated dissipation" table (standard vs extended mode for 0402/0603/0805), dimensions table; accessed 2026-11-02.
3. Panasonic Industry, "Precision Thick Film Chip Resistors, ERJ type (ERJ XG, 1G, 1R, 2R, 3R, 6R, 3E, 6E, 8E, 14, 12, 1T series)", pages dated 29-May-25, https://industrial.panasonic.com/cdbs/www-data/pdf/RDA0000/AOA0000C304.pdf — ratings (rated power, limiting element voltage, max overload voltage) and dimensions; accessed 2026-11-02.
4. KEMET (YAGEO Group), "Surface Mount Multilayer Ceramic Chip Capacitors (SMD MLCCs), C0G Dielectric, 10–250 VDC (Commercial Grade)", C1003_C0G, 2/20/2025, https://content.kemet.com/datasheets/KEM_C1003_C0G_SMD.pdf — EIA/metric size codes and L×W (0201/0603 … 2225/5664); accessed 2026-11-02.
5. KEMET (YAGEO Group), "… X7R Dielectric, 6.3–250 VDC (Commercial Grade)", C1002_X7R (2026-09-01), https://content.kemet.com/datasheets/KEM_C1002_X7R_SMD.pdf — EIA/metric size codes and L×W (0402/1005 … 2225/5664); accessed 2026-11-02.
6. TDK Corporation of America, "MLCC Product Guide" (A13), mirror at https://storage.sea.com.ua/tech_info/tdk/MLCC%20Product%20Guide%20A13.pdf — case size code table (metric 0402 = 0.40 × 0.20 mm, case sizes 01005–2220); accessed 2026-11-02.

## Cross-check
- Dimensions: 0402, 0603, 0805, 1206, 1210, 2010 agree within ≤0.1 mm across Yageo, Vishay, Panasonic and KEMET (Yageo 1206/1210 length 3.10 mm vs 3.20 mm at the others; Vishay 0603 1.55 × 0.85 mm vs 1.60 × 0.80 mm). 01005: Yageo, Panasonic, TDK all 0.40 × 0.20. 0201: Yageo, Panasonic, KEMET all 0.60 × 0.30. 1812: Panasonic and KEMET 4.50 × 3.20.
- 2512: no two makers agree exactly (6.35 × 3.10 Yageo, 6.3 × 3.15 Vishay, 6.40 × 3.20 Panasonic). Chosen nominal 6.3 × 3.2 mm (matches the metric code 6332 used by Vishay); spread is in the note.
- Metric codes: 0402→1005, 0603→1608, 0805→2012, 1206→3216, 1210→3225 confirmed by Vishay and KEMET; 0201→0603 by KEMET (C0G table); 1812→4532 by KEMET and TDK; 01005→0402 by TDK (metric 0402 = 0.40 × 0.20 mm) plus the matching 0.40 × 0.20 bodies at Yageo/Panasonic. 2010→5025 and 2512→6332 only from Vishay (RR5025M, RR6332M) — single source, consistent with all three makers' body sizes for 2010; for 2512 Panasonic's body (6.4 mm) would suggest 6432 (some makers use that code).
- Power ratings agree for 0603, 0805, 1206, 1210, 2010, 2512 (all three makers) and 01005, 0201 (Yageo + Panasonic). 0402 disagrees: 1/16 W (Yageo, Vishay standard) vs 0.1 W (Panasonic) → typical 1/16 W chosen (two makers), note added. 1812 power is Panasonic-only (single source).
- Working voltage agrees up to 1210 (all makers). 2010/2512: Yageo and Panasonic 200 V vs Vishay 400 V / 500 V → typical 200 V chosen (two makers), Vishay value kept in its own column.
- Vishay 0402/0603/0805 headline values on page 1 are the "extended" mode (0.10 W/75 V, 0.125 W/75 V, 0.25 W/150 V); the table uses the "standard" mode values from the same datasheet so the rows are comparable.

## Notes (en / de)
- en: Ratings are for general-purpose thick-film chip resistors at 70 °C ambient; above 70 °C derate linearly to zero at the maximum film temperature (155 °C for most series, 125 °C for Yageo RC0075–RC0201 and Panasonic ERJ-XG/1G). The continuous working voltage is the lower of √(P·R) and the maximum working voltage. Capacitor (MLCC) bodies use the same size codes, but thickness and tolerances are maker-specific. Some sizes/parts are marked NRFND by Panasonic.
- de: Die Werte gelten für Standard-Dickschicht-Chipwiderstände bei 70 °C Umgebung; darüber linear bis null bei der maximalen Schichttemperatur abwerten (155 °C bei den meisten Reihen, 125 °C bei Yageo RC0075–RC0201 und Panasonic ERJ-XG/1G). Die Dauerbetriebsspannung ist der kleinere Wert aus √(P·R) und der maximalen Betriebsspannung. MLCC-Körper verwenden dieselben Baugrößencodes, Dicke und Toleranzen sind aber herstellerabhängig. Einige Größen/Typen sind bei Panasonic als NRFND markiert.

## Suggested finder / lookups
- Smallest size whose p_typ_w ≥ required power (with derating margin) and v_typ ≥ applied voltage.
- Critical resistance R_crit = v_typ² / p_typ_w: below R_crit power limits, above it voltage limits (e.g. 0603: 75² / 0.1 = 56.25 kΩ; 1206: 160 kΩ).
- Convert imperial ↔ metric code.
