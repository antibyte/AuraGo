# materials — summary

| table_id | title (en / de) | kind | rows | sources |
|---|---|---|---|---|
| resistivity_conductors | Resistivity and temperature coefficient of conductor and resistance materials / Spezifischer Widerstand und Temperaturkoeffizient von Leiter- und Widerstandswerkstoffen | typical values (2 standard rows: IEC 60028 Cu, IEC 60889 Al) | 22 | 22 |
| dielectric_constants | Relative permittivity and loss tangent of PCB and cable dielectrics / Permittivitätszahl und Verlustfaktor von Leiterplatten- und Kabelisolierstoffen | typical values | 14 | 15 |
| thermocouple_types | Thermocouple types (IEC 60584) / Thermoelement-Typen (IEC 60584) | transcribed standard + formula-derived sensitivities | 8 | 14 |
| solder_alloys | Solder alloys / Lotlegierungen | typical values (+ transcribed ISO 9453 numbers) | 7 | 12 |

## Open problems

### resistivity_conductors
- UNVERIFIED α: gold (0.0034 vs 0.0037), nickel (0.0056–0.0064), iron (0.0050–0.0065; 0.0055 chosen),
  steel_low_carbon (single source, 0.0066), stainless_1_4301 (single source), nichrome_8020 (makers ≈ 0.0001 /K,
  textbooks 0.0004 /K; maker value used).
- Tin ρ only good to 2 digits (0.109–0.115).
- Resistivity disagreements settled in favour of the CRC 84th edition: gold, lead, tungsten.
- Seen only as search excerpts: the CuSn6 α value and the second manganin maker.
- No α for the solders.
- IEC 60028 / IEC 60889 not re-read (preview 403); catalog values kept (aluminium pair verified by the controller via
  HD 532 S1 and GB/T 17048-2017).
- Extra rows beyond the brief: copper_pure, aluminium_pure, stainless_1_4301.

### dielectric_constants
- tan δ single source: PE, PVC, Borofloat 33, Kapton, Al₂O₃ 99.5 %.
- Kyocera alumina values seen only as a search excerpt.
- Generic FR-4 row: Wikipedia (citing chip-vendor app notes) + Isola 370HR datasheet.
- PVC tan δ 0.05 is a representative pick within 0.04–0.1.

### thermocouple_types
- Type B IEC colours weakly sourced (one maker chart + a search excerpt; older tables conflict).
- Colours not checked against IEC 60584-3:2021 (IEC 60584-3:2007 used).
- Tolerance classes from three agreeing maker transcriptions; IEC table itself not read.
- Type K upper limit: IEC 1300 °C used (NIST 1372 °C).
- One source shows type E as pink; violet used.

### solder_alloys
- SAC405 liquidus UNVERIFIED (217/219/225 °C; 219 °C chosen); no ISO number found.
- Sn96.5Ag3.5 ISO number 703 UNVERIFIED.
- ISO numbers 401 and 301 from a single source (Stannol).
- J-STD-006 designations not found, omitted.
- Bi58Sn42 melting point 138 °C used (Stannol: 139 °C).

## Not done
Nothing skipped. 