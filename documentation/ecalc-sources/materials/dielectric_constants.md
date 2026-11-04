# dielectric_constants — Relative permittivity and loss tangent of PCB and cable dielectrics / Permittivitätszahl und Verlustfaktor von Leiterplatten- und Kabelisolierstoffen

Kind: typical values
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id | Zeilen-ID | – | no |
| material | material name | Werkstoffname | – (en\|de) | no |
| product | grade, product or maker designation | Sorte, Produkt oder Herstellerbezeichnung | – (names, not translated) | no |
| use | typical use | typische Verwendung | – (en\|de) | no |
| eps_r | relative permittivity εr (typical value to use) | Permittivitätszahl εr (typischer Rechenwert) | 1 (dimensionless) | yes |
| eps_r_min | lower end of the range given by the sources (empty = single value) | untere Grenze des Bereichs in den Quellen (leer = Einzelwert) | 1 | yes |
| eps_r_max | upper end of that range | obere Grenze dieses Bereichs | 1 | yes |
| eps_r_design | maker's "design Dk" for circuit simulation (Rogers only) | „Design-Dk" des Herstellers für die Schaltungssimulation (nur Rogers) | 1 | yes |
| tan_d | loss tangent tan δ (typical value to use) | Verlustfaktor tan δ (typischer Rechenwert) | 1 | yes |
| tan_d_min | lower end of the tan δ range | untere Grenze des tan-δ-Bereichs | 1 | yes |
| tan_d_max | upper end of the tan δ range | obere Grenze des tan-δ-Bereichs | 1 | yes |
| freq | test frequency or frequency range of the values | Messfrequenz oder Frequenzbereich der Werte | – (text, e.g. "10 GHz") | no |
| freq_hz | representative test frequency for sorting/filtering | repräsentative Messfrequenz zum Sortieren/Filtern | Hz | yes |
| flags | implementer-only: single-source or disputed cells (not user-facing) | nur für die Umsetzung: Einzelquelle oder strittig (nicht anzeigen) | – (English only) | no |
| note | user-facing remark | Hinweis für den Nutzer | – (en\|de) | no |

## Rows
```csv
id,material,product,use,eps_r,eps_r_min,eps_r_max,eps_r_design,tan_d,tan_d_min,tan_d_max,freq,freq_hz,flags,note
fr4_generic,FR-4 (glass-epoxy)|FR-4 (Glas-Epoxid),generic FR-4,standard PCB laminate|Standard-Leiterplattenbasismaterial,4.4,3.9,4.7,,0.02,0.017,0.03,1 MHz – 1 GHz (generic data),1e9,"generic values from Wikipedia (citing Altera 2003, Atmel 2003, Avago 2005 app notes); no single maker",Depends on resin content and glass style; ask the board maker for the stack-up value.|Hängt von Harzanteil und Glasgewebe ab; den Wert für den Lagenaufbau beim Leiterplattenhersteller erfragen.
fr4_370hr_1ghz,FR-4 high-Tg (example)|FR-4 High-Tg (Beispiel),Isola 370HR,multilayer PCB laminate|Multilayer-Basismaterial,4.17,,,,0.0161,,,1 GHz,1e9,,"Same laminate at 100 MHz: εr 4.24 / tan δ 0.0150.|Gleiches Material bei 100 MHz: εr 4,24 / tan δ 0,0150."
fr4_370hr_10ghz,FR-4 high-Tg (example)|FR-4 High-Tg (Beispiel),Isola 370HR,multilayer PCB laminate|Multilayer-Basismaterial,3.92,,,,0.0250,,,10 GHz,1e10,,"FR-4 losses rise with frequency (2 GHz: εr 4.04 / tan δ 0.0210).|Die Verluste von FR-4 steigen mit der Frequenz (2 GHz: εr 4,04 / tan δ 0,0210)."
ro4003c,Hydrocarbon/ceramic laminate|Kohlenwasserstoff-Keramik-Laminat,Rogers RO4003C,RF and microwave PCB|HF- und Mikrowellen-Leiterplatte,3.38,3.33,3.43,3.55,0.0027,,,10 GHz (tan δ 0.0021 at 2.5 GHz),1e10,,"εr 3.38 ± 0.05 is the process (clamped stripline) value; use the design Dk 3.55 (8–40 GHz) for line impedance.|εr 3,38 ± 0,05 ist der Prozesswert (Clamped Stripline); für Leitungsimpedanzen den Design-Dk 3,55 (8–40 GHz) verwenden."
ro4350b,Hydrocarbon/ceramic laminate|Kohlenwasserstoff-Keramik-Laminat,Rogers RO4350B,RF and microwave PCB|HF- und Mikrowellen-Leiterplatte,3.48,3.43,3.53,3.66,0.0037,,,10 GHz (tan δ 0.0031 at 2.5 GHz),1e10,,"εr 3.48 ± 0.05 process value; design Dk 3.66 (8–40 GHz).|εr 3,48 ± 0,05 Prozesswert; Design-Dk 3,66 (8–40 GHz)."
rt_duroid_5880,PTFE/glass microfibre laminate|PTFE-Glasmikrofaser-Laminat,Rogers RT/duroid 5880,low-loss microwave PCB|verlustarme Mikrowellen-Leiterplatte,2.20,2.18,2.22,2.20,0.0009,0.0004,0.0009,10 GHz (tan δ 0.0004 at 1 MHz),1e10,,"εr 2.20 ± 0.02 at 10 GHz.|εr 2,20 ± 0,02 bei 10 GHz."
ptfe,PTFE (pure)|PTFE (rein),"Teflon PTFE (DuPont/Chemours), generic PTFE",cable insulation and RF parts|Kabelisolierung und HF-Teile,2.1,,,,0.0001,0.0001,0.0004,60 Hz – 2 GHz,1e6,"tan_d: DuPont table <0.0001 (60 Hz–2 GHz), DuPont text <0.0004 up to 100 MHz, myElectrical 0.0003 (50 Hz)",εr is almost independent of frequency and temperature.|εr ist nahezu unabhängig von Frequenz und Temperatur.
polyimide_kapton_hn,Polyimide film|Polyimidfolie,DuPont Kapton HN,flex PCB and insulation film|flexible Leiterplatte und Isolierfolie,3.4,3.4,3.5,,0.0018,0.0018,0.0026,1 kHz,1e3,,"3.4 for 25–50 µm and 3.5 for 75–125 µm film; tan δ rises from 0.0018 (25 µm) to 0.0026 (125 µm).|3,4 bei 25–50 µm und 3,5 bei 75–125 µm Folie; tan δ steigt von 0,0018 (25 µm) auf 0,0026 (125 µm)."
pe,Polyethylene (LDPE/HDPE)|Polyethylen (LDPE/HDPE),"PE, VDE code 2Y",coax and data cable insulation|Isolierung von Koax- und Datenkabeln,2.3,2.2,2.4,,0.0002,0.0002,0.0003,50 Hz,50,tan_d single source (myElectrical); eps_r agrees across Lapp/myElectrical/Wikipedia/Engineering ToolBox,"Very low loss; solid-PE coax has a velocity factor of about 0.66.|Sehr verlustarm; Koaxkabel mit Voll-PE haben einen Verkürzungsfaktor von etwa 0,66."
pvc,PVC|PVC,"PVC, VDE code Y",mains and control cable insulation|Isolierung von Netz- und Steuerleitungen,4.0,3.6,6.5,,0.05,0.04,0.1,50 Hz,50,"tan_d single source (myElectrical, 0.04–0.1); eps_r: Lapp 4.0 (heat-resistant PVC 3.5), myElectrical 3.6–6.5; tan_d = 0.05 is a representative pick inside the source range",High loss; not suitable as an RF dielectric.|Hohe Verluste; als HF-Dielektrikum ungeeignet.
air,Air (dry)|Luft (trocken),–,reference|Bezug,1.00059,1.000536,1.00059,,0,,,"0.9 MHz at STP (1.00059); 20 °C (1.000536)",9e5,"tan_d: no value in the sources, treated as 0",For most calculations εr = 1.|Für die meisten Rechnungen εr = 1.
glass_borosilicate,Borosilicate glass|Borosilikatglas,SCHOTT BOROFLOAT 33,glass substrates and windows|Glassubstrate und Fenster,4.6,4.3,5.0,,0.0037,,,1 MHz,1e6,tan_d single source (SCHOTT); eps_r range = Engineering ToolBox Pyrex 4.3–5.0,"Other glasses range from about 3.7 to 10.|Andere Gläser liegen etwa zwischen 3,7 und 10."
alumina_96,Alumina ceramic 96 %|Aluminiumoxidkeramik 96 %,"Al2O3 96 % (CoorsTek AD-96, Kyocera A476)",thick-film substrates and hybrids|Dickschichtsubstrate und Hybride,9.0,9.0,9.4,,0.0002,0.0002,0.0004,1 MHz,1e6,Kyocera values (9.4 / 0.0004) seen only as web-search excerpt,
alumina_995,"Alumina ceramic 99.5 %|Aluminiumoxidkeramik 99,5 %",Al2O3 99.5 % (CoorsTek AD-995),thin-film microwave substrates|Dünnschicht-Mikrowellensubstrate,9.7,,,,0.0001,,,1 MHz,1e6,single maker (CoorsTek); Engineering ToolBox gives alumina 9.3–11.5 (unspecified),
```

## Sources
1. Rogers Corporation, "RO4000® Series High Frequency Circuit Materials" data sheet (RO4003C, RO4350B), Revised 1592 080322, PUB# 92-004: process Dk 3.38 ± 0.05 / 3.48 ± 0.05 (10 GHz/23 °C, IPC-TM-650 2.5.5.5 clamped stripline), design Dk 3.55 / 3.66 (8–40 GHz, differential phase length), Df 0.0027 / 0.0037 at 10 GHz and 0.0021 / 0.0031 at 2.5 GHz, TCDk +40 / +50 ppm/°C — https://www.rogerscorp.com/-/media/project/rogerscorp/documents/advanced-electronics-solutions/english/data-sheets/ro4000-laminates-ro4003c-and-ro4350b---data-sheet.pdf — accessed 2026-11-02.
2. Rogers Corporation, RO4350B product page (3.48 ± 0.05 process, 3.66 design, Df 0.0037) — https://www.rogerscorp.com/advanced-electronics-solutions/ro4000-series-laminates/ro4350b-laminates — accessed 2026-11-02.
3. Rogers Corporation, "RT/duroid® 5870/5880 High Frequency Laminates" data sheet, Revised 1603 080822, Publication #92-101: 5880 Dk 2.20 (1 MHz, IPC-TM-650 2.5.5.3) and 2.20 ± 0.02 (10 GHz, 2.5.5.5), design Dk 2.20 (8–40 GHz), Df 0.0004 (1 MHz) / 0.0009 (10 GHz) — https://www.rogerscorp.com/-/media/project/rogerscorp/documents/advanced-electronics-solutions/english/data-sheets/rt-duroid-5870---5880-data-sheet.pdf — accessed 2026-11-02.
4. Atlas PCB, "Rogers RO4003C" material page (RO4003C 3.38 / 0.0027 at 10 GHz; RO4350B 3.48 / 0.0037; RT/duroid 5880 2.20 / 0.0009) — https://www.atlaspcb.com/materials/rogers-ro4003c/ — accessed 2026-11-02.
5. Isola Group, 370HR data sheet, Revision B, last updated 14 Sep 2018: Dk 4.24 / 4.17 / 4.04 / 3.92 / 3.92 and Df 0.0150 / 0.0161 / 0.0210 / 0.0250 / 0.0250 at 100 MHz / 1 / 2 / 5 / 10 GHz (IPC-TM-650 2.5.5.3, 2.5.5.9, Bereskin stripline, 2.5.5.5) — copy at https://www.epectec.com/downloads/Isola-370HR-Data-Sheet.pdf — accessed 2026-11-02.
6. Wikipedia, "FR-4": εr 4.4 (A and D24/23), 3.9–4.7 (Altera 2003), 4.4 at 1 GHz (Isola; Atmel 2003); dissipation factor 0.017/0.018, loss tangent 0.02–0.03 (Altera), 0.030 at 1 GHz (Avago 2005; Atmel) — https://en.wikipedia.org/wiki/FR-4 — accessed 2026-11-02.
7. DuPont Fluoroproducts, "Teflon® PTFE fluoropolymer resin — Properties Handbook" (edition year not printed in the extracted text): dielectric constant 2.1 and dissipation factor < 0.0001 for 60 Hz to 2×10⁹ Hz (ASTM D150); text: "below 0.0004 over a frequency range up to 10⁸ Hz" — copy at https://cdn.toxicdocs.org/K6/K6RD4NNj8dVar4pb7aORgak4X/K6RD4NNj8dVar4pb7aORgak4X.pdf — accessed 2026-11-02.
8. DuPont, "Kapton® HN polyimide film" technical data sheet K-15345-2 (12/16): dielectric constant 3.4 (25 and 50 µm), 3.5 (75 and 125 µm) at 1 kHz; dissipation factor 0.0018 / 0.0020 / 0.0020 / 0.0026 at 1 kHz — copy at https://www.epectec.com/downloads/DuPont-Kapton-HN-Polyimide-Film.pdf — accessed 2026-11-02.
9. Lapp, ÖLFLEX catalogue appendix T15 "Properties of insulating and sheathing materials": dielectric constant PVC 4.0, heat-resistant PVC 3.5, LDPE 2.3, HDPE 2.3, PTFE 2.1, FEP 2.1 — https://t3.lappcdn.com/fileadmin/DAM/Miltronic_Denmark/4_servicecenter/T15.pdf — accessed 2026-11-02.
10. S. McFadyen, "Cable Insulation Properties", myElectrical (15 May 2012): εr at 50 Hz/20 °C and tan δ — PVC 3.6–6.5 / 0.04–0.1, LDPE 2.3 / 2×10⁻⁴, HDPE 2.3 / 3×10⁻⁴, PTFE 2.1 / 3×10⁻⁴ — https://myelectrical.com/notes/entryid/178/cable-insulation-properties — accessed 2026-11-02. (Its XLPE row "4–6" looks like a mis-transcription and was not used.)
11. Wikipedia, "Relative permittivity", table "Relative permittivities of some materials at room temperature under 1 kHz": air 1.00058986 ± 0.00000050 (STP, 900 kHz; Hector & Schultz 1936), PTFE 2.1, polyethylene 2.25, polyimide 3.4, glass (Pyrex) 4.7 (3.7–10), sapphire 8.9–11.1 — https://en.wikipedia.org/wiki/Relative_permittivity — accessed 2026-11-02.
12. The Engineering ToolBox, "Relative Permittivity – the Dielectric Constant": air (dry, 68 °F) 1.000536, alumina 9.3–11.5, glass 3.7–10, Pyrex 4.3–5.0, polyethylene/XLPE 2.2–2.4, PTFE 2.1, polyimide 3.4 (no frequencies given) — https://www.engineeringtoolbox.com/relative-permittivity-d_1660.html — accessed 2026-11-02.
13. SCHOTT, "BOROFLOAT® 33 – Technical Data": εr 4.6 and tan δ 37×10⁻⁴ at 1 MHz and 25 °C — copy at https://www.edmundoptics.com/ViewDocument/schott-borofloat-technical-data-sheet-english.pdf — accessed 2026-11-02.
14. CoorsTek, "Ceramic Properties Standard" chart, © 2008, F0801 8510-1042 Rev. J: dielectric constant / tan δ at 1 MHz, 25 °C (ASTM D150) — AD-94 9.1 / 0.0004, AD-96 9.0 / 0.0002, AD-995 9.7 / 0.0001, AD-998 9.8 / 0.0001 — copy at https://psec.uchicago.edu/ceramics/CoorsTek%208510-1042_Ceramic_Material_Properties.pdf — accessed 2026-11-02.
15. Kyocera A476 (≥ 96 % alumina): εr 9.4 and tan δ 4×10⁻⁴ at 1 MHz — seen only in a web-search result summary of https://global.kyocera.com/prdct/semicon/search_material/detail/alumina.html and the Kyocera Fineceramics Europe AO476O data sheet; pages not opened.

## Cross-check
| row | εr sources | tan δ sources | verdict |
|---|---|---|---|
| fr4_generic | Wikipedia 4.4 (3.9–4.7); Isola 370HR 4.17 (1 GHz) | Wikipedia 0.017–0.03; Isola 0.0161 (1 GHz) | typical, wide range is honest |
| fr4_370hr_* | Isola data sheet only (maker) | Isola only | maker data per brief rule 4 |
| ro4003c / ro4350b | Rogers data sheet + Rogers web page + Atlas PCB | same | agree |
| rt_duroid_5880 | Rogers data sheet + Atlas PCB | same | agree |
| ptfe | DuPont 2.1; Lapp 2.1; myElectrical 2.1; Wikipedia 2.1; Engineering ToolBox 2.1 | DuPont < 0.0001 (table) / < 0.0004 (text); myElectrical 0.0003 | εr agrees; tan δ range kept |
| polyimide_kapton_hn | DuPont 3.4/3.5; Wikipedia 3.4; Engineering ToolBox 3.4 | DuPont 0.0018–0.0026 only | εr agrees; tan δ maker only |
| pe | Lapp 2.3; myElectrical 2.3; Wikipedia 2.25; Engineering ToolBox 2.2–2.4 | myElectrical 0.0002–0.0003 only | εr agrees; tan δ single source |
| pvc | Lapp 4.0 (3.5 heat-resistant); myElectrical 3.6–6.5 | myElectrical 0.04–0.1 only | typical range; tan δ single source |
| air | Wikipedia 1.00059 (STP, 0 °C); Engineering ToolBox 1.000536 (20 °C) | – | both right for their temperature |
| glass_borosilicate | SCHOTT 4.6; Wikipedia Pyrex 4.7; Engineering ToolBox Pyrex 4.3–5.0 | SCHOTT 0.0037 only (a PGO table with 0.0048 was seen in a search excerpt only) | εr agrees |
| alumina_96 | CoorsTek AD-96 9.0; Kyocera A476 9.4 (search excerpt); Engineering ToolBox 9.3–11.5 | CoorsTek 0.0002; Kyocera 0.0004 | within 5 %; range kept |
| alumina_995 | CoorsTek 9.7 only; Wikipedia sapphire 8.9–11.1 (single crystal, anisotropic) | CoorsTek 0.0001 only | maker data |

## Notes (en / de)
- en: Typical values. εr and tan δ depend on frequency, temperature, humidity and — for laminates — on resin content and glass weave; always state the frequency with the value. For impedance design use the board maker's stack-up data (Rogers: the "design Dk").
- de: Typische Werte. εr und tan δ hängen von Frequenz, Temperatur, Feuchte und – bei Laminaten – von Harzanteil und Glasgewebe ab; den Wert immer mit der Frequenz angeben. Für den Impedanzentwurf die Lagenaufbau-Daten des Leiterplattenherstellers verwenden (Rogers: den „Design-Dk").
- en: tan δ is the ratio of the loss to the stored energy per cycle; the dielectric attenuation of a line grows with √εr · tan δ · f.
- de: tan δ ist das Verhältnis von Verlust- zu gespeicherter Energie je Periode; die dielektrische Dämpfung einer Leitung wächst mit √εr · tan δ · f.

## Suggested finder / lookups
- Feed `eps_r_design` (else `eps_r`) into the microstrip / stripline / coax calculators as a material picker.
- Velocity factor of a TEM line in a homogeneous dielectric: VF = 1/√εr (e.g. PE 2.3 → 0.66, PTFE 2.1 → 0.69).
- Dielectric loss of a TEM line: α_d ≈ 27.3 · √εr · tan δ / λ0 dB per unit length (λ0 free-space wavelength) — formula for an implementer, not part of the table.
