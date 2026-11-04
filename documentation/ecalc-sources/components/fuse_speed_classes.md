# fuse_speed_classes — Miniature fuse time-current characteristics (IEC 60127) / Kennlinien von Geräteschutzsicherungen (IEC 60127)

Kind: definitional (code letters and names)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| code | characteristic symbol marked on the fuse | Kennbuchstabe auf der Sicherung | – | no |
| name | official name | offizielle Bezeichnung | – | no (en/de text) |
| speed_rank | 1 = fastest … 5 = slowest | 1 = am schnellsten … 5 = am langsamsten | – | yes |
| meaning | what the class means | Bedeutung der Kennlinie | – | no (en/de text) |
| example_10x_max_ms | example: max. pre-arcing time at 10 × In for a 5×20 mm low-breaking fuse 0.125–6.3 A (IEC 60127-2 sheet 2 for F, sheet 3 for T); empty where not sourced | Beispiel: max. Schmelzzeit bei 10 × In für 5×20-mm-Sicherung, niedriges Schaltvermögen, 0,125–6,3 A (IEC 60127-2 Normblatt 2 für F, 3 für T); leer, wo nicht belegt | ms | yes |

## Rows
```csv
code,name,speed_rank,meaning,example_10x_max_ms
FF,"very quick-acting|superflink",1,"Interrupts overcurrents fastest; for protecting sensitive semiconductors|Schaltet Überströme am schnellsten ab; zum Schutz empfindlicher Halbleiter",
F,"quick-acting|flink",2,"Fast response to overload and short circuit; general protection of electronics without inrush|Schnelle Reaktion auf Überlast und Kurzschluss; allgemeiner Elektronikschutz ohne Einschaltstrom",20
M,"medium time-lag|mittelträge",3,"Between quick-acting and time-lag; tolerates small inrush peaks|Zwischen flink und träge; verträgt kleine Einschaltspitzen",
T,"time-lag|träge",4,"Tolerates inrush currents (transformers, motors, capacitor charging) but still clears sustained overload|Verträgt Einschaltströme (Trafos, Motoren, Kondensatorladen), löst bei Dauerüberlast aber aus",300
TT,"long time-lag|superträge",5,"Slowest class; for very high or long inrush currents|Langsamste Klasse; für sehr hohe oder lange Einschaltströme",
```

## Sources
1. DIN EN 60127-4:2013-12 (VDE 0820-4), "Miniature fuses – Part 4: Universal modular fuse-links (UMF) – Through-hole and surface mount types", DIN Media catalogue page, https://www.dinmedia.de/en/standard/din-en-60127-4/191105992 (English abstract: "very quick acting (FF), quick acting (F), time-lag (T) and long time-lag (TT)") and German page https://www.dinmedia.de/de/norm/din-en-60127-4/191105992 (Einführungsbeitrag: „superflink (FF), flink (F), träge (T) und superträge (TT)“); accessed 2026-11-02.
2. DIN 41571-2:1984-06, "Miniature fuses; cartridge fuse links 250 V, interchangeable, medium time-lag (M)" / „Geräteschutzsicherungen; G-Sicherungseinsätze 250 V, verwechselbar, mittelträge (M)“, DIN Media catalogue page, https://dinmedia.de/en/standard/din-41571-2/1125168 — name of class M in English and German; accessed 2026-11-02.
3. SCHURTER, "FST 5x20 — Miniature Fuse, 5 x 20 mm, Time-Lag T, L, 250 VAC" (IEC 60127-2/3), https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_FST_5x20.pdf and "FSF 5x20 — Miniature Fuse, 5 x 20 mm, Quick-Acting F, L, 250 VAC" (IEC 60127-2/2), https://www.schurter.com/bundles/snceschurter/epim/_ProdPool_/newDS/en/typ_FSF_5x20.pdf — names "Quick-Acting F", "Time-Lag T" and the 10 × In example values; accessed 2026-11-02.
4. Littelfuse, "218 Series, 5×20 mm, Time-Lag Fuse" (Revised BA.08/26/20, IEC 60127-2 Sheet 3) and "217 Series, 5×20 mm, Fast-acting Fuse" (Revised GD.10/30/20, IEC 60127-2 Sheet 2), copies at https://sy-dep-epc-lpc.web.cern.ch/components/datasheets/epc-lpc%20(converters)/0218080MXP_Fuse_(Littelfuse).pdf and https://static.chipdip.ru/lib/405/DOC042405444.pdf — 10 × In values (cross-check of 3); accessed 2026-11-02.
5. IEC 60127-1:2023 (Edition 3.0, 2023-10), "Miniature fuses – Part 1: Definitions for miniature fuses and general requirements for miniature fuse-links", publisher preview https://assets.vde-verlag.de/iec-normen/preview-pdf/info_iec60127-1%7Bed3.0%7Db.pdf — confirms the structure (9.2 time/current characteristic, Annex A colour coding, Table C.1 summary of IEC 60127-2 standard sheets); the definitions clause itself is not in the free preview; accessed 2026-11-02.

## Cross-check
- Names FF/F/T/TT: DIN EN 60127-4 abstract (1) in English and German. F and T names also on Schurter (3) and Littelfuse (4) datasheets ("Quick-Acting F"/"Fast-acting", "Time-Lag T"). M: DIN 41571-2 title (2) only — a national standard, consistent with the IEC letter. Definitional → one authoritative source per name is met; the IEC 60127-1 definitions text itself could not be read (paywalled).
- example_10x_max_ms: F = 20 ms and T = 300 ms agree between Schurter and Littelfuse (see table fuse_prearc_iec60127_2). No sourced 10 × In example was found for FF, M and TT → left empty (not guessed).
- meaning column: the short explanations are editorial wording (not quoted); they restate the class order. The inrush examples (transformers, motors, capacitor charging) are general engineering guidance, not taken from a cited source.

## Notes (en / de)
- en: The letter is printed on the end cap of the fuse together with rated current, rated voltage and breaking capacity (e.g. L = low, H = high, as used by Schurter). The exact pre-arcing limits depend on the standard sheet (size, breaking capacity) — see fuse_prearc_iec60127_2. Never replace a fuse with a slower class or higher rating.
- de: Der Buchstabe steht mit Nennstrom, Nennspannung und Schaltvermögen (z. B. L = niedrig, H = hoch, wie bei Schurter) auf der Kappe. Die genauen Schmelzzeitgrenzen hängen vom Normblatt (Bauform, Schaltvermögen) ab — siehe fuse_prearc_iec60127_2. Eine Sicherung nie durch eine trägere Klasse oder einen höheren Nennstrom ersetzen.

## Suggested finder / lookups
- Lookup by code letter → name and rank; "is class A slower than class B" via speed_rank.
