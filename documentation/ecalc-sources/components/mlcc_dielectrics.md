# mlcc_dielectrics — Ceramic capacitor dielectrics (EIA codes) / Keramikkondensator-Dielektrika (EIA-Codes)

Kind: definitional (EIA RS-198 / EIA-198 code meanings) with maker confirmation; class-2 behaviour notes are typical
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| code | EIA temperature-characteristic code | EIA-Temperaturkennlinien-Code | – | no |
| alias | other common name | übliche andere Bezeichnung | – | no |
| eia_class | EIA class as stated by the maker (I, II, III) | EIA-Klasse laut Hersteller (I, II, III) | – | no |
| iec_code | IEC 60384-8/-9 equivalent (approx. where marked ≈) | IEC-60384-8/-9-Entsprechung (≈ = nur ungefähr) | – | no |
| t_min_c | lower temperature limit | untere Temperaturgrenze | °C | yes |
| t_max_c | upper temperature limit | obere Temperaturgrenze | °C | yes |
| tc_ppm_k | temperature coefficient (class I only) | Temperaturkoeffizient (nur Klasse I) | ppm/K | yes |
| tc_tol_ppm_k | tolerance of the temperature coefficient (± value, class I only) | Toleranz des TK (±, nur Klasse I) | ppm/K | yes |
| dc_min_pct | max. capacitance change over the range, lower bound (class II/III) | max. Kapazitätsänderung im Bereich, untere Grenze (Klasse II/III) | % | yes |
| dc_max_pct | max. capacitance change over the range, upper bound (class II/III) | max. Kapazitätsänderung im Bereich, obere Grenze (Klasse II/III) | % | yes |
| use | typical use | typische Anwendung | – | no (en/de text) |

## Rows
```csv
code,alias,eia_class,iec_code,t_min_c,t_max_c,tc_ppm_k,tc_tol_ppm_k,dc_min_pct,dc_max_pct,use
C0G,NP0,I,CG,-55,125,0,30,,,"Resonant circuits, filters, timing, where Q and capacitance stability matter; no ageing, no DC-bias effect|Schwingkreise, Filter, Zeitglieder, wo Güte und Kapazitätsstabilität zählen; keine Alterung, kein DC-Bias-Effekt"
X8R,,II,,-55,150,,,-15,15,"Decoupling/bypass at high temperature (e.g. automotive)|Abblocken/Entkoppeln bei hoher Temperatur (z. B. Automobil)"
X7R,,II,2X1,-55,125,,,-15,15,"General-purpose bypass, decoupling and filtering where Q and stability are not critical|Allgemeines Abblocken, Entkoppeln und Filtern, wo Güte und Stabilität unkritisch sind"
X7S,,II,≈2C1,-55,125,,,-22,22,"Decoupling with higher capacitance per size than X7R, wider tolerance|Entkoppeln mit mehr Kapazität je Baugröße als X7R, weitere Toleranz"
X7T,,II,,-55,125,,,-33,22,"High-capacitance decoupling; large negative change at temperature extremes|Entkoppeln mit hoher Kapazität; starke negative Änderung an den Temperaturgrenzen"
X6S,,II,,-55,105,,,-22,22,"High-capacitance decoupling up to 105 °C|Entkoppeln mit hoher Kapazität bis 105 °C"
X5R,,II,,-55,85,,,-15,15,"High-capacitance decoupling and bypass up to 85 °C|Entkoppeln und Abblocken mit hoher Kapazität bis 85 °C"
Y5V,,III (KEMET),≈2F4,-30,85,,,-82,22,"Low-cost bypass/decoupling where stability does not matter; avoid where temperature varies|Billiges Abblocken/Entkoppeln, wo Stabilität egal ist; bei schwankender Temperatur meiden"
```

## Sources
1. KEMET (YAGEO Group), "SMD MLCCs, C0G Dielectric, 10–250 VDC (Commercial Grade)", C1003_C0G, 2/20/2025, https://content.kemet.com/datasheets/KEM_C1003_C0G_SMD.pdf — "EIA characterizes C0G dielectric as a Class I material"; ±30 ppm/°C from −55 °C to +125 °C; no change with time and voltage; resonant-circuit use; accessed 2026-11-02.
2. KEMET, "… X7R Dielectric, 6.3–250 VDC", C1002_X7R (2026-09-01), https://content.kemet.com/datasheets/KEM_C1002_X7R_SMD.pdf — Class II; ±15 % from −55 °C to +125 °C; bypass/decoupling use; predictable change with time and voltage; accessed 2026-11-02.
3. KEMET, "… X5R Dielectric, 4–100 VDC", C1006_X5R (2026-09-07), https://content.kemet.com/datasheets/KEM_C1006_X5R_SMD.pdf — Class II; ±15 % from −55 °C to +85 °C; accessed 2026-11-02.
4. KEMET, "… Y5V Dielectric, 6.3–50 VDC", C1005_Y5V_SMD, 2/20/2025, https://content.kemet.com/datasheets/KEM_C1005_Y5V_SMD.pdf — "EIA characterizes Y5V dielectric as a Class III material"; +22 %/−82 % from −30 °C to +85 °C; accessed 2026-11-02.
5. TDK Corporation of America, "MLCC Product Guide" (A13), mirror https://storage.sea.com.ua/tech_info/tdk/MLCC%20Product%20Guide%20A13.pdf — part-number decoding table of temperature characteristics: C0G −55…+125 °C 0±30 ppm/°C, X5R −55…+85 ±15 %, X6S −55…+105 ±22 %, X7R −55…+125 ±15 %, X7S −55…+125 ±22 %, X7T −55…+125 +22/−33 %, X8R −55…+150 ±15 %; accessed 2026-11-02.
6. Wikipedia, "Ceramic capacitor", https://en.wikipedia.org/wiki/Ceramic_capacitor — EIA RS-198 code tables (class 1: C=0.0, 0=×−1, G=±30 ppm/K; class 2: X=−55, Y=−30, Z=+10 °C; 5=+85, 6=+105, 7=+125, 8=+150 °C; R=±15 %, S=±22 %, T=+22/−33 %, V=+22/−82 %) and IEC 60384-8/-9 equivalents (C0G→CG; X7R→2X1; X7S≈2C1; Y5V≈2F4); class 3 discussion; accessed 2026-11-02.

## Cross-check
- C0G: KEMET (1) and TDK (5) agree: −55…+125 °C, 0 ± 30 ppm/K; Wikipedia code decode agrees.
- X7R, X5R: KEMET (2, 3) and TDK (5) agree (±15 %, −55…+125 / −55…+85 °C); Wikipedia decode agrees.
- X7S, X6S, X7T, X8R: TDK (5) values agree with the Wikipedia EIA code decode (6) (S = ±22 %, T = +22/−33 %, 6 = +105 °C, 8 = +150 °C). Note: the TDK PDF's text layer is column-shifted; values were mapped by row order and each one matches the code decode.
- Y5V: KEMET (4) +22/−82 %, −30…+85 °C; Wikipedia decode (Y = −30, 5 = +85, V = +22/−82 %) agrees.
- Disagreement — class of Y5V: KEMET calls it EIA Class III; Wikipedia treats Y5V/Z5U as EIA class 2 codes and says class 3 (barrier-layer) is no longer standardized by the IEC. Chosen: "III (KEMET)" with the note below.
- use column: C0G, X7R, X5R, Y5V texts paraphrase the KEMET overviews (1–4). The texts for X8R, X7S, X7T, X6S are editorial (derived from the code meaning, not quoted from a source) — the implementer may shorten them to "class II decoupling".
- IEC codes: only Wikipedia (single source) gives the IEC 60384-8/-9 equivalents (C0G→CG, X7R→2X1, X7S≈2C1, Y5V≈2F4); no IEC code was found for X8R, X7T, X6S, X5R → treat iec_code as informative only (UNVERIFIED as a set, single source).

## Notes (en / de)
- en: Code reading (class II/III): 1st letter = lower temperature (X −55 °C, Y −30 °C, Z +10 °C), digit = upper temperature (5 +85, 6 +105, 7 +125, 8 +150 °C), last letter = max. capacitance change (R ±15 %, S ±22 %, T +22/−33 %, V +22/−82 %). Class II/III capacitors also lose capacitance with applied DC voltage and with age (ageing); the temperature code says nothing about DC bias. Some makers (e.g. KEMET) call Y5V "Class III"; in IEC terms it is a class 2 dielectric.
- de: Code-Lesart (Klasse II/III): 1. Buchstabe = untere Temperatur (X −55 °C, Y −30 °C, Z +10 °C), Ziffer = obere Temperatur (5 +85, 6 +105, 7 +125, 8 +150 °C), letzter Buchstabe = max. Kapazitätsänderung (R ±15 %, S ±22 %, T +22/−33 %, V +22/−82 %). Klasse-II/III-Kondensatoren verlieren zusätzlich Kapazität mit angelegter Gleichspannung und mit der Zeit (Alterung); der Temperaturcode sagt nichts über den DC-Bias. Manche Hersteller (z. B. KEMET) nennen Y5V „Klasse III“; nach IEC ist es ein Klasse-2-Dielektrikum.

## Suggested finder / lookups
- Filter: dielectrics whose [t_min_c, t_max_c] covers the required operating range; rank by |dc_min_pct| (class I first).
- Worst-case capacitance over temperature: C·(1 + dc_min_pct/100) … C·(1 + dc_max_pct/100); for class I: C·(1 ± tc_tol_ppm_k·1e-6·|T − 25 °C|).
