# amateur_bands_de — Amateur radio bands in Germany (AFuV Annex 1) / Amateurfunkbänder in Deutschland (AFuV Anlage 1)

Kind: transcribed standard (German regulation)
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| nr | running number ("Lfd. Nr.") of AFuV Annex 1 part A | laufende Nummer der AFuV Anlage 1 Buchstabe A | – | yes |
| band | conventional band name by wavelength (not part of the regulation) | übliche Bandbezeichnung nach Wellenlänge (nicht Teil der Verordnung) | – | no |
| f_low | lower band edge | untere Bandgrenze | see `unit` | yes |
| f_high | upper band edge (empty for "> 275 GHz") | obere Bandgrenze | see `unit` | yes |
| unit | frequency unit of this row exactly as in the regulation (kHz, MHz, GHz) | Frequenzeinheit wie in der Verordnung | – | no |
| status | `P` = amateur service is primary, `S` = secondary | `P` = primärer, `S` = sekundärer Funkdienst | – | no |
| class_a | maximum power for licence class A (PEP = peak envelope power, ERP = effective radiated power) | Höchstleistung Klasse A (PEP = Spitzenleistung, ERP = effektive Strahlungsleistung) | W | partly (value + PEP/ERP) |
| class_e | maximum power for class E (empty = band not available to class E) | Höchstleistung Klasse E (leer = Band für Klasse E nicht freigegeben) | W | partly |
| class_n | maximum power for class N (entry class since 2024; empty = not available) | Höchstleistung Klasse N (Einsteigerklasse seit 2024; leer = nicht freigegeben) | W | partly |
| b_notes | numbers of the additional usage conditions in part B ("Zusätzliche Nutzungsbestimmungen"), see notes | Nummern der zusätzlichen Nutzungsbestimmungen (Buchstabe B) | – | no |

## Rows
```csv
nr,band,f_low,f_high,unit,status,class_a,class_e,class_n,b_notes
1,2200 m,135.7,137.8,kHz,S,1 W ERP,,,"1,2,10"
2,630 m,472,479,kHz,S,1 W ERP,,,1
3,160 m,1810,1850,kHz,P,750 W PEP,100 W PEP,,3
4,160 m,1850,1890,kHz,S,75 W PEP,75 W PEP,,"3,10,12,15"
5,160 m,1890,2000,kHz,S,10 W PEP,10 W PEP,,"3,10,15"
6,80 m,3500,3800,kHz,P,750 W PEP,100 W PEP,,3
7,60 m,5351.5,5366.5,kHz,S,9.14 W ERP,,,3
8,40 m,7000,7200,kHz,P,750 W PEP,,,"3,13"
9,30 m,10100,10150,kHz,S,150 W PEP,,,"1,10,12"
10,20 m,14000,14350,kHz,P,750 W PEP,,,"3,13"
11,17 m,18068,18168,kHz,P,750 W PEP,,,"3,13"
12,15 m,21000,21450,kHz,P,750 W PEP,100 W PEP,,"3,13"
13,12 m,24890,24990,kHz,P,750 W PEP,,,"3,13"
14,10 m,28,29.7,MHz,P,750 W PEP,100 W PEP,10 W ERP,"4,13"
15,6 m,50,50.4,MHz,S,750 W PEP,,,"5,16"
16,6 m,50.4,52,MHz,S,25 W PEP,,,"5,16"
17,2 m,144,146,MHz,P,750 W PEP,75 W PEP,6.1 W ERP,"6,13"
18,70 cm,430,440,MHz,P,750 W PEP,75 W PEP,6.1 W ERP,"7,13"
19,23 cm,1240,1250,MHz,S,750 W PEP,75 W PEP,,"8,11,17"
20,23 cm,1250,1260,MHz,S,750 W PEP,75 W PEP,,"8,11,17"
21,23 cm,1260,1300,MHz,S,750 W PEP,75 W PEP,,"8,11,13,17"
22,13 cm,2320,2400,MHz,S,75 W PEP,5 W PEP,,"9,17"
23,13 cm,2400,2450,MHz,S,75 W PEP,5 W PEP,,"9,13,17"
24,9 cm,3400,3475,MHz,S,75 W PEP,5 W PEP,,"9,17"
25,6 cm,5650,5670,MHz,S,75 W PEP,5 W PEP,,"9,13,17"
26,6 cm,5670,5725,MHz,S,75 W PEP,5 W PEP,,"9,17"
27,6 cm,5725,5755,MHz,S,75 W PEP,5 W PEP,,"9,17"
28,6 cm,5755,5830,MHz,S,75 W PEP,5 W PEP,,"9,17"
29,6 cm,5830,5850,MHz,S,75 W PEP,5 W PEP,,"9,17"
30,3 cm,10,10.4,GHz,S,75 W PEP,5 W PEP,,"9,17"
31,3 cm,10.4,10.45,GHz,S,75 W PEP,5 W PEP,,"9,17"
32,3 cm,10.45,10.5,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
33,1.2 cm,24,24.05,GHz,P,75 W PEP,5 W PEP,,"13,17"
34,1.2 cm,24.05,24.25,GHz,S,75 W PEP,5 W PEP,,"9,17"
35,6 mm,47,47.2,GHz,P,75 W PEP,5 W PEP,,"13,17"
36,4 mm,76,77.5,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
37,4 mm,77.5,78,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
38,4 mm,78,79,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
39,4 mm,79,81,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
40,2.5 mm,122.25,123,GHz,S,75 W PEP,5 W PEP,,"9,17"
41,2 mm,134,136,GHz,P,75 W PEP,5 W PEP,,"9,13,17"
42,2 mm,136,141,GHz,S,75 W PEP,5 W PEP,,"9,13,17"
43,1.2 mm,241,248,GHz,S,75 W PEP,5 W PEP,,"13,17"
44,1.2 mm,248,250,GHz,P,75 W PEP,5 W PEP,,"13,17"
45,,>275,,GHz,,,,,"14,17"
```

## Sources
1. Verordnung zum Gesetz über den Amateurfunk (Amateurfunkverordnung – AFuV), **Anlage 1** "Nutzungsbedingungen für die im Frequenzplan für den Amateurfunkdienst und den Amateurfunkdienst über Satelliten ausgewiesenen Frequenzbereiche" (Fundstelle: BGBl. 2024 I Nr. 175, S. 1–4), https://www.gesetze-im-internet.de/afuv_2005/anlage_1.html — all 45 rows, footnotes 1 (P/S) and 2 (PEP/ERP), part B notes 1–17; accessed 2026-11-02.
2. buzer.de, "Anlage 1 AFuV" (independent transcription of the same legal text), https://www.buzer.de/Anlage_1_AFuV.htm — all 45 rows; accessed 2026-11-02.
3. DARC e.V., "Amateurfunkverordnung – Änderungen zum 24.06.2024" (Schnellübersicht), https://www.darc.de/fileadmin/filemounts/gs/redaktion/DARC-Portal/2024/2406/AFuV2024-Schnelluebersicht.pdf — class N on 28–29.7 MHz (10 W ERP), 144–146 and 430–440 MHz (10 W EIRP / 6.1 W ERP); 50–50.4 MHz class A 750 W PEP, 50.4–52 MHz 25 W PEP; class E no longer allowed on 6 m; 160 m weekend rule; accessed 2026-11-02.

## Cross-check
- Sources 1 and 2 were both parsed into tables and compared row by row with a helper script that is not part of the repository: **all 45 rows identical** (frequency ranges, status, powers for A/E/N, part-B note numbers); the only difference is the dash glyph in row 45.
- Source 3 (DARC, independent of the legal-text publishers) agrees for every value it mentions: class N 28–29.7 MHz 10 W ERP, 144–146 / 430–440 MHz 6.1 W ERP; 50–50.4 MHz 750 W PEP and 50.4–52 MHz 25 W PEP for class A only; weekend 160 m powers (part B note 15).
- The legal text is the authority; the table reflects the version published in BGBl. 2024 I Nr. 175 (in force 24 June 2024 per source 3). Temporary toleration rules (e.g. 70 MHz band by BNetzA Verfügung) are **not** part of Annex 1 and are not in the rows.
- Band names (`band`) are conventional and not from the regulation.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Only the current AFuV and the Bundesnetzagentur's decisions are binding.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Regeln. Verbindlich sind nur die aktuelle AFuV und die Verfügungen der Bundesnetzagentur.
- en: Part B notes (summary of source 1): 1 max. occupied bandwidth 800 Hz; 2 sites must be notified to BNetzA; 3 max. bandwidth 2.7 kHz; 4 max. bandwidth 7 kHz below 29 MHz, 40 kHz above; 5 no interference to broadcasting/wind-profiler radars, horizontal polarisation only, fixed stations only (50–52 MHz); 6 max. bandwidth 40 kHz; 7 max. 2 MHz (AM TV 7 MHz); 8 max. 2 MHz (AM/digital TV 7 MHz, FM TV 18 MHz); 9 max. 10 MHz (TV 20 MHz); 10 no remote-controlled stations except remote operation, no contests; 11 1247–1263 MHz max. 3.05 W ERP, no remote/automatic stations; 12 automatic stations max. 50 W ERP; 13 also usable for the amateur-satellite service (see source for primary/secondary details); 14 bands above 275 GHz usable without protection; 15 weekends on 1850–2000 kHz: class A 750 W PEP, class E 100 W PEP, contests allowed; 16 max. bandwidth 12 kHz; 17 links of remote/automatic stations up to 1000 W ERP in justified cases.
- de: Zusätzliche Nutzungsbestimmungen (Zusammenfassung von Quelle 1): 1 max. belegte Bandbreite 800 Hz; 2 Betriebsorte der BNetzA anzeigen; 3 max. Bandbreite 2,7 kHz; 4 max. Bandbreite unter 29 MHz 7 kHz, darüber 40 kHz; 5 keine Störung von Rundfunk/Windprofilradaren, nur horizontale Polarisation, nur ortsfeste Stationen (50–52 MHz); 6 max. Bandbreite 40 kHz; 7 max. 2 MHz (AM-Fernsehen 7 MHz); 8 max. 2 MHz (AM/digitales Fernsehen 7 MHz, FM-Fernsehen 18 MHz); 9 max. 10 MHz (Fernsehen 20 MHz); 10 keine fernbedienten Stationen außer Remote-Betrieb, kein Contestbetrieb; 11 1247–1263 MHz max. 3,05 W ERP, keine fernbedienten/automatischen Stationen; 12 automatische Stationen max. 50 W ERP; 13 auch für den Amateurfunkdienst über Satelliten nutzbar (Details zu primär/sekundär in der Quelle); 14 Bereiche über 275 GHz ohne Schutzanspruch nutzbar; 15 an Wochenenden auf 1850–2000 kHz: Klasse A 750 W PEP, Klasse E 100 W PEP, Contests erlaubt; 16 max. Bandbreite 12 kHz; 17 Linkstrecken fernbedienter/automatischer Stationen bis 1000 W ERP in begründeten Fällen.
- en: General rule (Annex 1 (1)): remote-controlled and automatic terrestrial stations above 30 MHz max. 50 W ERP (except remote operation).
- de: Allgemein (Anlage 1 Abs. 1): fernbediente und automatisch arbeitende terrestrische Stationen oberhalb 30 MHz max. 50 W ERP (ausgenommen Remote-Betrieb).

## Suggested finder / lookups
- Frequency + licence class → allowed? and max. power (find row with f_low ≤ f ≤ f_high in the row's unit; empty class cell = not allowed).
