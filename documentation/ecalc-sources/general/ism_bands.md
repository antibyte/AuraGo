# ism_bands — ISM bands (ITU) and EU short-range-device bands / ISM-Bänder (ITU) und EU-SRD-Bänder

Kind: transcribed standard
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id | Zeilen-Id | – | no |
| f_low | lower band edge | untere Bandgrenze | MHz | yes |
| f_high | upper band edge | obere Bandgrenze | MHz | yes |
| f_centre | ISM centre frequency (ITU rows only) | ISM-Mittenfrequenz (nur ITU-Zeilen) | MHz | yes |
| basis | legal basis: ITU Radio Regulations footnote, or EU decision + annex row | Rechtsgrundlage | – | no |
| region | where the band applies "en\|de" | Geltungsbereich "en\|de" | – | no |
| power | maximum radiated power for EU SRD rows (e.r.p. or e.i.r.p. as written in the source) | maximale Strahlungsleistung (EU-SRD-Zeilen) | mW | partly (text with e.r.p./e.i.r.p.) |
| duty_cycle | maximum duty cycle if used instead of LBT/AFA mitigation (EU SRD rows) | maximaler Tastgrad, falls statt LBT/AFA verwendet | % | partly |
| note | typical use / remark "en\|de" | typische Nutzung / Hinweis "en\|de" | – | no |
| check | cross-check record | Quellenabgleich | – | no |

## Rows
```csv
id,f_low,f_high,f_centre,basis,region,power,duty_cycle,note,check
ism_6m78,6.765,6.795,6.78,ITU RR 5.138,"worldwide, subject to local acceptance|weltweit, vorbehaltlich nationaler Zulassung",,,ISM use needs special authorization by the administration|ISM-Nutzung nur mit besonderer Genehmigung der Verwaltung,FCC 18.301 6.78 MHz ±15 kHz = WP
ism_13m56,13.553,13.567,13.56,ITU RR 5.150,worldwide|weltweit,,,"NFC/RFID (13.56 MHz)|NFC/RFID (13,56 MHz)",FCC 13.56 MHz ±7 kHz = WP
ism_27m12,26.957,27.283,27.12,ITU RR 5.150,worldwide|weltweit,,,overlaps the CB radio band|überlappt den CB-Funk-Bereich,FCC 27.12 MHz ±163 kHz = WP
ism_40m68,40.66,40.70,40.68,ITU RR 5.150,worldwide|weltweit,,,,FCC 40.68 MHz ±20 kHz = WP
ism_433,433.05,434.79,433.92,ITU RR 5.138,ITU Region 1 only (countries of RR 5.280)|nur ITU-Region 1 (Länder nach RR 5.280),,,LPD433; also amateur 70 cm band|LPD433; zugleich 70-cm-Amateurfunkband,WP (not in FCC 18.301: Region 2 has no 433 MHz ISM band)
ism_915,902,928,915,ITU RR 5.150,ITU Region 2 only (Americas)|nur ITU-Region 2 (Amerika),,,US 915 MHz band (LoRaWAN US902-928)|US-915-MHz-Band (LoRaWAN US902-928),FCC 915 MHz ±13 MHz = WP
ism_2g45,2400,2500,2450,ITU RR 5.150,worldwide|weltweit,,,"Wi-Fi, Bluetooth, microwave ovens; RLAN/SRD use in Europe limited to 2400–2483.5 MHz|WLAN, Bluetooth, Mikrowellenherde; in Europa SRD/WLAN nur 2400–2483,5 MHz",FCC 2450 MHz ±50 MHz = WP
ism_5g8,5725,5875,5800,ITU RR 5.150,worldwide|weltweit,,,Wi-Fi channels 149–165 (US); SRD in Europe|WLAN-Kanäle 149–165 (USA); SRD in Europa,FCC 5800 MHz ±75 MHz = WP
ism_24g125,24000,24250,24125,ITU RR 5.150,worldwide|weltweit,,,"radar sensors, motion detectors|Radarsensoren, Bewegungsmelder",FCC 24.125 GHz ±125 MHz = WP
ism_61g25,61000,61500,61250,ITU RR 5.138,"worldwide, subject to local acceptance|weltweit, vorbehaltlich nationaler Zulassung",,,,FCC 61.25 GHz ±250 MHz = WP
ism_122g5,122000,123000,122500,ITU RR 5.138,"worldwide, subject to local acceptance|weltweit, vorbehaltlich nationaler Zulassung",,,,FCC 122.5 GHz ±500 MHz = WP
ism_245g,244000,246000,245000,ITU RR 5.138,"worldwide, subject to local acceptance|weltweit, vorbehaltlich nationaler Zulassung",,,,FCC 245 GHz ±1 GHz = WP
srd_433,433.05,434.79,,"EU 2006/771/EC as amended by (EU) 2022/180, row 44b",EU|EU,10 mW e.r.p.,10 %,"non-specific SRD (remote controls, sensors); row 44a: 1 mW without duty-cycle limit|nicht-spezifische SRD (Fernbedienungen, Sensoren); Zeile 44a: 1 mW ohne Duty-Cycle-Grenze",single source (EU legal text); WP SRD lists the band only
srd_865,865,868,,"EU (EU) 2022/180, row 47",EU|EU,25 mW e.r.p.,1 %,non-specific SRD|nicht-spezifische SRD,EU text = WP SRD
srd_868_0,868.0,868.6,,"EU (EU) 2022/180, row 48",EU|EU,25 mW e.r.p.,1 %,"LoRaWAN EU868 default channels 868.1 / 868.3 / 868.5 MHz lie here|hier liegen die LoRaWAN-EU868-Standardkanäle 868,1 / 868,3 / 868,5 MHz",EU text = WP SRD
srd_868_7,868.7,869.2,,"EU (EU) 2022/180, row 50",EU|EU,25 mW e.r.p.,0.1 %,non-specific SRD|nicht-spezifische SRD,EU text = WP SRD
srd_869_4,869.4,869.65,,"EU (EU) 2022/180, row 54",EU|EU,500 mW e.r.p.,10 %,"LoRaWAN EU868 RX2 / beacon 869.525 MHz lies here|hier liegt LoRaWAN-EU868-RX2/Beacon 869,525 MHz",EU text = WP SRD
srd_869_7,869.7,870.0,,"EU (EU) 2022/180, row 56b",EU|EU,25 mW e.r.p.,1 %,row 56a allows 5 mW without duty-cycle limit (voice)|Zeile 56a erlaubt 5 mW ohne Duty-Cycle-Grenze (Sprache),EU text = WP SRD
srd_2g4,2400,2483.5,,"EU (EU) 2022/180, row 57a",EU|EU,10 mW e.i.r.p.,,"non-specific SRD; wideband data (Wi-Fi/Bluetooth, row 57c) up to 100 mW e.i.r.p.|nicht-spezifische SRD; Breitband-Daten (WLAN/Bluetooth, Zeile 57c) bis 100 mW e.i.r.p.",single source for power (EU legal text); band edges = WP SRD
srd_5g8,5725,5875,,"EU (EU) 2022/180, row 61",EU|EU,25 mW e.i.r.p.,,non-specific SRD|nicht-spezifische SRD,single source for power (EU legal text); band edges = WP SRD
```

## Sources
1. ITU Radio Regulations, Article 5, footnotes **No. 5.138** (bands designated for ISM, use subject to special authorization) and **No. 5.150** (bands designated for ISM) — **not read directly** (the ITU RR PDF is behind a download form); the band list is taken from sources 2 and 3, which both reference these footnotes.
2. U.S. eCFR, 47 CFR § 18.301 "Operating frequencies" (ISM frequency and tolerance table), https://www.ecfr.gov/current/title-47/chapter-I/subchapter-A/part-18/subpart-C/section-18.301 — 6.78 MHz ±15 kHz … 245 GHz ±1 GHz; accessed 2026-11-02.
3. en.wikipedia, "ISM radio band" (table with type A = RR 5.138, type B = RR 5.150), https://en.wikipedia.org/wiki/ISM_radio_band — accessed 2026-11-02.
4. Commission Implementing Decision (EU) 2022/180 of 8 February 2022 amending Decision 2006/771/EC on harmonisation of the radio spectrum for use by short-range devices, Annex (rows 44a/44b, 47, 48, 50, 54, 56a/56b, 57a/57c, 61), https://eur-lex.europa.eu/legal-content/EN/TXT/HTML/?uri=CELEX:32022D0180 (OJ L 29, 10.2.2022) — read via the in-app browser (direct fetch blocked); accessed 2026-11-02.
5. en.wikipedia, "Short-range device", https://en.wikipedia.org/wiki/Short-range_device — EU 863–870 MHz sub-band table (duty cycle, ERP) and SRD band list; accessed 2026-11-02.

## Cross-check
- **ITU ISM rows:** every band edge equals FCC centre ± tolerance (source 2) and the Wikipedia table (source 3); verified arithmetically in a helper script that is not part of the repository (all 11 FCC rows). 433.05–434.79 MHz is a Region-1-only band and therefore not in the FCC table; it is backed by source 3 (and by source 4/5 as EU SRD band). The RR footnote numbers (5.138/5.150) come from source 3 only, because the RR text was not read.
- Wikipedia (source 3) also lists 169.4–169.8125 MHz as an EU-only "ISM" band; it is not an ITU ISM band and is left out.
- **EU SRD rows 863–870 MHz:** power and duty cycle agree between the EU decision (source 4) and the Wikipedia SRD table (source 5) for 865–868, 868.0–868.6, 868.7–869.2, 869.4–869.65 and 869.7–870 MHz. Minor difference: source 5 shows 863.0–865.6 MHz for the 0.1 % sub-band, source 4 row 46a 863–865 MHz (not included in the rows).
- **Single-source values (EU legal text only):** 433.05–434.79 MHz 10 mW e.r.p./10 % (row 44b), 2400–2483.5 MHz 10 mW e.i.r.p. (57a) and 5725–5875 MHz 25 mW e.i.r.p. (61). Source 5 lists these bands but not the power. They come from the binding legal text, so I consider them reliable, but by the brief's rule they are single-source.
- Decision (EU) 2022/180 is the version I could read; later amendments of 2006/771/EC may exist — the implementer should check EUR-Lex for a newer consolidated text before release.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Radio use always needs the national frequency assignment (in Germany: Bundesnetzagentur general assignments).
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Regeln. Funkbetrieb braucht immer die nationale Frequenzzuteilung (in Deutschland: Allgemeinzuteilungen der Bundesnetzagentur).
- en: ISM bands are designated for industrial, scientific and medical RF energy (e.g. microwave ovens); radio services there must accept interference from ISM equipment. Short-range devices (SRD) are licence-exempt only under the EU/national conditions (power, duty cycle or LBT/AFA).
- de: ISM-Bänder sind für industrielle, wissenschaftliche und medizinische HF-Anwendungen (z. B. Mikrowellenherde) vorgesehen; Funkdienste dort müssen Störungen durch ISM-Geräte hinnehmen. SRD sind nur unter den EU-/nationalen Bedingungen (Leistung, Tastgrad oder LBT/AFA) anmeldefrei.
- en: Duty cycle = share of transmit time, e.g. 1 % = 36 s per hour. LBT/AFA = listen-before-talk with adaptive frequency agility (alternative to the duty-cycle limit).
- de: Tastgrad (Duty Cycle) = Anteil der Sendezeit, z. B. 1 % = 36 s pro Stunde. LBT/AFA = "Listen before talk" mit adaptivem Frequenzwechsel (Alternative zur Tastgrad-Grenze).

## Suggested finder / lookups
- Frequency → which ISM/SRD band(s) contain it (f_low ≤ f ≤ f_high).
- Duty-cycle helper: allowed airtime per hour = duty_cycle × 3600 s.
