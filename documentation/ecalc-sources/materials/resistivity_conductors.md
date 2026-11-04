# resistivity_conductors — Resistivity and temperature coefficient of conductor and resistance materials / Spezifischer Widerstand und Temperaturkoeffizient von Leiter- und Widerstandswerkstoffen

Kind: typical values (the two rows `copper_iec60028` and `aluminium_iec60889` are standard values; everything else is handbook or maker data)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id | Zeilen-ID | – | no |
| material | material name | Werkstoffname | – (en\|de) | no |
| designation | alloy designation, material number, standard, trade names | Kurzzeichen, Werkstoffnummer, Norm, Handelsnamen | – (codes, not translated) | no |
| category | material group | Werkstoffgruppe | – (en\|de) | no |
| rho20 | resistivity at 20 °C (value to use) | spezifischer Widerstand bei 20 °C (Rechenwert) | Ω·mm²/m (ECalc unit `ohmmm2m`; 1 Ω·mm²/m = 1 µΩ·m = 100 µΩ·cm) | yes |
| rho20_min | lower end of the spread found in the sources (empty = no spread worth showing) | untere Grenze der Streuung in den Quellen (leer = keine nennenswerte Streuung) | Ω·mm²/m | yes |
| rho20_max | upper end of that spread | obere Grenze dieser Streuung | Ω·mm²/m | yes |
| alpha | temperature coefficient of resistance near 20 °C (value to use; empty for precision alloys whose α is specified only as a band) | Temperaturkoeffizient des Widerstands um 20 °C (Rechenwert; leer bei Präzisionslegierungen mit nur einem Toleranzband) | 1/K | yes |
| alpha_min | lower limit of the specified α band | untere Grenze des spezifizierten α-Bands | 1/K | yes |
| alpha_max | upper limit of the specified α band | obere Grenze des spezifizierten α-Bands | 1/K | yes |
| alpha_basis | how/where α applies | Gültigkeit/Bezug von α | – (en\|de) | no |
| flags | implementer-only: unverified or disputed cells (not user-facing) | nur für die Umsetzung: unbestätigte oder strittige Zellen (nicht anzeigen) | – (English only) | no |
| note | user-facing remark | Hinweis für den Nutzer | – (en\|de) | no |

## Rows
```csv
id,material,designation,category,rho20,rho20_min,rho20_max,alpha,alpha_min,alpha_max,alpha_basis,flags,note
silver,Silver|Silber,Ag (pure),pure metal|reines Metall,0.01587,,,0.0038,,,local value at 20 °C|lokaler Wert bei 20 °C,,Lowest resistivity of all metals.|Niedrigster spezifischer Widerstand aller Metalle.
copper_iec60028,"Copper, annealed (standard)|Kupfer, weichgeglüht (Normwert)","IEC 60028 (International Annealed Copper Standard, 100 % IACS = 58 MS/m)",conductor standard|Leiter-Normwert,0.017241,,,0.00393,,,at 20 °C per IEC 60028|bei 20 °C nach IEC 60028,,Exactly 1/58 Ω·mm²/m; the reference for copper cables and windings. Already used by the ECalc wire calculators.|Genau 1/58 Ω·mm²/m; Bezugswert für Kupferkabel und Wicklungen. Wird bereits von den ECalc-Leiterrechnern verwendet.
copper_pure,"Copper, pure (handbook)|Kupfer, rein (Tabellenwert)",Cu (high-purity),pure metal|reines Metall,0.01678,,,0.0040,,,local value at 20 °C|lokaler Wert bei 20 °C,alpha: HyperPhysics/Giancoli lists 0.00386,"Purer than the IACS reference; commercial copper is closer to 0.01724.|Reiner als der IACS-Bezug; handelsübliches Kupfer liegt näher an 0,01724."
gold,Gold|Gold,Au (pure),pure metal|reines Metall,0.02214,0.0221,0.0244,0.0037,,,local value at 20 °C|lokaler Wert bei 20 °C,"rho: Wikipedia/Serway 0.0244 vs CRC 0.02214 (chosen); alpha UNVERIFIED: CRC-derived 0.0037 vs Wikipedia/Serway 0.0034",Value for the pure metal; plated gold layers can differ.|Wert für das reine Metall; galvanische Goldschichten können abweichen.
aluminium_iec60889,"Aluminium, hard-drawn wire (standard)|Aluminium, hartgezogener Draht (Normwert)","IEC 60889 (hard-drawn aluminium wire for overhead line conductors, 61 % IACS)",conductor standard|Leiter-Normwert,0.028264,,,0.00403,,,at 20 °C per IEC 60889|bei 20 °C nach IEC 60889,,Value for aluminium conductors; already used by the ECalc wire calculators.|Wert für Aluminiumleiter; wird bereits von den ECalc-Leiterrechnern verwendet.
aluminium_pure,"Aluminium, pure (handbook)|Aluminium, rein (Tabellenwert)",Al (high-purity),pure metal|reines Metall,0.02650,,,0.0043,,,local value at 20 °C|lokaler Wert bei 20 °C,alpha: Engineering ToolBox lists 0.0038,Pure metal; conductor aluminium has the higher IEC 60889 value.|Reines Metall; Leiteraluminium hat den höheren Wert nach IEC 60889.
brass_cuzn37,Brass|Messing,"CuZn37 (CW508L, UNS C27200, Ms63)",copper alloy|Kupferlegierung,0.066,0.062,0.071,0.0017,,,mean 0–100 °C|Mittelwert 0–100 °C,alpha: Engineering ToolBox (brass 63 % Cu) 0.0015,Depends on zinc content and temper; soft temper conducts best.|Abhängig von Zinkgehalt und Zustand; weich geglüht leitet am besten.
bronze_cusn6,Phosphor bronze|Phosphorbronze,"CuSn6 (CW452K, UNS C51900)",copper alloy|Kupferlegierung,0.111,0.111,0.112,0.00065,,,mean 0–100 °C|Mittelwert 0–100 °C,"alpha single source (DKI sheet, seen only as search excerpt); mean 20–200 °C from the DKI/Bikar rho values is 0.00085",Spring and contact material.|Feder- und Kontaktwerkstoff.
nickel,Nickel|Nickel,Ni (pure),pure metal|reines Metall,0.0693,0.0685,0.0699,0.0060,,,near 20 °C|um 20 °C,"alpha UNVERIFIED: Wikipedia 0.0060, Engineering ToolBox 0.0064, CRC-derived local 0.0056","Technical nickel (Ni 99.2) is higher: about 0.09.|Technisches Nickel (Ni 99,2) liegt höher: etwa 0,09."
iron,Iron|Eisen,Fe (pure),pure metal|reines Metall,0.0961,0.0961,0.0971,0.0055,,,local value at 20 °C (rises strongly with temperature)|lokaler Wert bei 20 °C (steigt mit der Temperatur stark an),"alpha UNVERIFIED: CRC-derived local 0.0054, Wikipedia 0.0050, HyperPhysics 0.00651, Engineering ToolBox 0.00641",Pure iron; steels are much higher.|Reines Eisen; Stähle liegen deutlich höher.
steel_low_carbon,"Steel, unalloyed low-carbon|Stahl, unlegiert, kohlenstoffarm","e.g. AISI 1010 / C10",steel|Stahl,0.143,0.14,0.15,0.0066,,,near 20 °C|um 20 °C,"alpha UNVERIFIED (single source: Engineering ToolBox, mild steel); rho: Elgin datasheet and Wikipedia may share one origin; MakeItFrom 12 % IACS = 0.144 seen only as search excerpt",Varies with carbon and alloy content.|Schwankt mit Kohlenstoff- und Legierungsgehalt.
stainless_1_4301,"Stainless steel 1.4301|Edelstahl 1.4301","1.4301 (X5CrNi18-10, AISI 304)",steel|Stahl,0.73,0.69,0.75,0.00094,,,near 20 °C|um 20 °C,"alpha UNVERIFIED (single source: Wikipedia, generic stainless steel); rho: Wikipedia 0.69 (generic), Outokumpu 0.75 seen only as search excerpt",Austenitic stainless steel; about 40 × copper.|Austenitischer Edelstahl; etwa 40 × Kupfer.
tin,Tin|Zinn,Sn (pure),pure metal|reines Metall,0.11,0.109,0.115,0.0045,,,near 20 °C|um 20 °C,rho UNVERIFIED to 2 digits: Wikipedia 0.109 and Engineering ToolBox 0.110 (20 °C) vs CRC/Kaye & Laby/NIST 0.115 (temperature not stated); alpha: Engineering ToolBox 0.0042,
lead,Lead|Blei,Pb (pure),pure metal|reines Metall,0.208,0.206,0.22,0.0039,,,near 20 °C|um 20 °C,rho: Wikipedia/HyperPhysics 0.22 (textbook) vs CRC 0.208 (chosen),
tungsten,Tungsten|Wolfram,W (pure),pure metal|reines Metall,0.0528,0.0528,0.0565,0.0045,,,near 20 °C|um 20 °C,rho: textbooks 0.056–0.0565 vs CRC 0.0528 (chosen),Lamp filaments; hot resistance is many times the cold value.|Glühfäden; der Heißwiderstand ist ein Vielfaches des Kaltwerts.
platinum,Platinum|Platin,Pt (pure),pure metal|reines Metall,0.105,,,0.00392,,,near 20 °C (pure platinum)|um 20 °C (reines Platin),CRC-derived alpha (0.0043) is imprecise because CRC gives only 2–3 digits; textbook value 0.00392 chosen,"Pt100 sensors to IEC 60751 use α = 0.00385 /K (mean 0–100 °C).|Pt100-Sensoren nach IEC 60751 verwenden α = 0,00385 /K (Mittel 0–100 °C)."
zinc,Zinc|Zink,Zn (pure),pure metal|reines Metall,0.0590,,,0.0037,,,near 20 °C|um 20 °C,,
constantan,Constantan|Konstantan,"CuNi44 (2.0842; ISOTAN, Cuprothal 49)",resistance alloy|Widerstandslegierung,0.49,0.44,0.54,,-0.00008,0.00004,"band 20–105 °C (DIN 17471 limits, Isabellenhütte); Kanthal: ±0.00006 from −55 to 150 °C|Band 20–105 °C (Grenzen DIN 17471, Isabellenhütte); Kanthal: ±0,00006 von −55 bis 150 °C","rho_min/max = maker tolerance ±10 %; alpha nominally ~0, use the band",Nearly temperature-independent; high thermo-EMF against copper (about −40 µV/K) so not for precision resistors.|Nahezu temperaturunabhängig; hohe Thermospannung gegen Kupfer (etwa −40 µV/K) und daher nicht für Präzisionswiderstände.
manganin,Manganin|Manganin,"CuMn12Ni (2.1362; MANGANIN)",resistance alloy|Widerstandslegierung,0.43,0.41,0.45,,-0.00001,0.00001,band 20–50 °C (parabolic R(T) curve)|Band 20–50 °C (parabelförmige R(T)-Kurve),"rho_min/max = maker tolerance ±5 %; textbooks (Giancoli/Serway) give 0.482 and 2e-6 for an older composition",Material for shunts and precision resistors; low thermo-EMF against copper.|Werkstoff für Shunts und Präzisionswiderstände; geringe Thermospannung gegen Kupfer.
nichrome_8020,Nichrome 80/20|Nickelchrom 80/20,"NiCr 80/20 (NiCr8020; Nikrothal 80, ISA-CHROM 80)",heating alloy|Heizleiterlegierung,1.10,1.08,1.12,0.0001,,,"mean 20–100 °C, approximate|Mittelwert 20–100 °C, Näherung","alpha UNVERIFIED: maker tables give about 0.00011–0.00013 (Isabellenhütte 112→113 µΩ·cm, Kanthal Ct(100 °C) = 1.01), textbooks give 0.0004",Heating-wire alloy; resistivity depends slightly on cooling rate and wire size.|Heizdrahtlegierung; der spezifische Widerstand hängt etwas von Abkühlrate und Drahtdurchmesser ab.
solder_sn63pb37,Solder Sn63Pb37|Lot Sn63Pb37,Sn63Pb37,solder|Lot,0.150,0.145,0.150,,,,,"no alpha found; NIST database gives 0.145 (14.5 µΩ·cm), Kester and Indium 11.5 % IACS = 0.150",Bulk solder; about 9 × copper.|Massives Lot; etwa 9 × Kupfer.
solder_sac305,Solder SAC305|Lot SAC305,Sn96.5Ag3.0Cu0.5,solder|Lot,0.132,0.130,0.132,,,,,"no alpha found; NIST lists 13 µΩ·cm for Sn-3.5/3.8Ag-0.7Cu (not exactly SAC305)",Lead-free standard solder.|Bleifreies Standardlot.
```

## Sources
1. IEC 60028:1925 "International standard of resistance for copper" — 1/58 Ω·mm²/m and α = 0.00393 /K at 20 °C. Not re-read here (the IEC webstore preview https://webstore.ansi.org/preview-pages/IEC/preview_iec60028%7Bed2.0%7Db.img.pdf returned HTTP 403); values as already cited in the ECalc catalog (passive.yaml, power.yaml, wiring.yaml). Consistent with sources 3–5 and with https://en.wikipedia.org/wiki/International_Annealed_Copper_Standard ("exactly 58×10^6 S/m at 20 °C"). Accessed 2026-11-02.
2. IEC 60889 (hard-drawn aluminium wire for overhead line conductors) — 0.028264 Ω·mm²/m, α 0.00403 /K. Already verified per the brief and cited in the ECalc catalog; not re-read. Wikipedia (source 3) lists aluminium 2.82×10⁻⁸ Ω·m, which is the same 61 % IACS value.
3. Wikipedia, "Electrical resistivity and conductivity", table of resistivity, conductivity and temperature coefficient at 20 °C (cites Serway, Griffiths, Giancoli) — https://en.wikipedia.org/wiki/Electrical_resistivity_and_conductivity — accessed 2026-11-02.
4. HyperPhysics (C. R. Nave, Georgia State University), "Resistivity and Temperature Coefficient at 20 C" (cites Giancoli, Physics 4th ed. 1995; CRC Handbook 64th ed. for annealed copper) — https://hyperphysics.gsu.edu/hbase/Tables/rstiv.html — accessed 2026-11-02.
5. The Engineering ToolBox (2003), "Resistivity and Conductivity – Temperature Coefficients Common Materials" — https://www.engineeringtoolbox.com/resistivity-conductivity-d_418.html — accessed 2026-11-02.
6. Wikipedia, "Electrical resistivities of the elements (data page)", CRC column = CRC Handbook of Chemistry and Physics, 84th ed. (D. R. Lide, 2003), section 12 "Electrical Resistivity of Pure Metals", values at 273/293/300 K — https://en.wikipedia.org/wiki/Electrical_resistivities_of_the_elements_(data_page) — accessed 2026-11-02. Used for ρ20 of the pure metals and to derive α (see Cross-check).
7. Isabellenhütte Heusler, data sheet MANGANIN (2.1362, CuMn12Ni): 43 µΩ·cm ±5 %, TCR −10…+10 ppm/K (20–50 °C), thermo-EMF vs Cu −0.6 µV/K — copy at https://hallaweb.jlab.org/tech/Detectors/public_html/manuals/data_sheets-manuals/G-I/I/ISABELLENHUETTE/MANGANIN-ISABELLENHUETTE-R.pdf — accessed 2026-11-02.
8. Isabellenhütte Heusler, data sheet ISOTAN (2.0842, CuNi44, "also known as Konstantan"): 49 µΩ·cm ±10 %, TCR −80…+40 ppm/K (20–105 °C, DIN 17471), thermo-EMF vs Cu −40 µV/K — https://www.isabellenhuette.com/hubfs/Files/Data-sheets/LE/Resistor/ISOTAN.pdf (file metadata 2022-10) — accessed 2026-11-02.
9. Isabellenhütte Heusler, "Technical Information" (English), Table 1 "Dependence of Resistivity in µΩ·cm on Temperature for Various Alloys" (ISA-CHROM 80 = NiCr8020: 112 µΩ·cm at 20 °C, 113 at 100 °C, 108 after rapid cooling; ISOTAN 49; MANGANIN 43; pure nickel 8; Ni 99.2 9) — https://www.isabellenhuette.com/hubfs/Files/Whitepaper/alloys/Tech_Informationen_engl_01.pdf — accessed 2026-11-02.
10. Kanthal, material datasheet Nikrothal 80 (ribbon): Ni bal., Cr 19–21 %, 1.09 Ω·mm²/m at 20 °C, Ct = 1.01 at 100 °C — https://www.kanthal.com/products/datasheets/material-datasheets/ribbon-flat-wire/nikrothal-80/ — accessed 2026-11-02.
11. Kanthal, "Physical and mechanical properties – Cuprothal": Cuprothal 49 (Ni 44 %, Cu bal.) 0.49 Ω·mm²/m at 20 °C, temperature factor ±20/±60 (−55…150 °C) — https://prodshop.kanthal.com/globalassets/kanthal-global/downloads/physical-and-mechanical-prop-cuprothal.pdf — accessed 2026-11-02.
12. Deutsches Kupferinstitut, Werkstoff-Datenblatt CuZn37 (DKI CuZn37:2005): 15 MS/m, 0.066 Ω·mm²/m at 20 °C, 0.083 at 200 °C, α = 0.0017 K⁻¹ valid 0–100 °C — copy at https://cdn.shopify.com/s/files/1/0652/7241/4476/files/Datenblatt_CuZn37.pdf — accessed 2026-11-02.
13. Aurubis, Material Datasheet CuZn37 (PNA 243, EN_2026_01; UNS C27200, EN CW508L): 16 MS/m = 28 % IACS (soft temper) — https://aurubis.com/dam/jcr:c81305d1-7d1b-4f12-9eaf-376cff4ada3c/PNA%20243_CuZn37_C27200-E.N.pdf — accessed 2026-11-02.
14. Bikar Metalle, product page CW452K / CuSn6: 0.111 Ω·mm²/m at 20 °C, 0.128 at 200 °C, 9 / 7.8 m/(Ω·mm²) — https://my.bikar.com/de_DE/produkte/421_020-421-020-1 — accessed 2026-11-02. The DKI CuSn6 sheet (α = 0.00065 K⁻¹, 0–100 °C) was seen only as a web-search excerpt, not opened.
15. Aurubis, Material Datasheet CuSn6 (PNA 282, EN_2026_01; UNS C51900, EN CW452K): 9 MS/m = 16 % IACS — https://www.aurubis.com/dam/jcr:6eaf4a8d-ba49-4d2b-ae4f-72c36374a8fa/PNA%20282_CuSn6_C51900-EN.pdf — accessed 2026-11-02.
16. Elgin Fasteners, "Carbon Steel Grade 1010 Data Sheet" (AISI 1010 cold drawn): electrical resistivity 0.0000143 Ω·cm — https://www.mwcomponents.com/uploads/Resource-Center/Elgin-Material-Sheets/Carbon-Steel-Grade-1010-Data-Sheet_Elgin-Fasteners.pdf — accessed 2026-11-02.
17. Lamineries Matthey, data sheet Stainless Steel 1.4301 (v22E): electrical resistivity 73 µΩ·cm, conductivity 1.4 MS/m — https://www.matthey.ch/fileadmin/user_upload/downloads/fichetechnique/EN/1.4301_v22E.pdf — accessed 2026-11-02.
18. NIST / Colorado School of Mines, "Database for Solder Properties with Emphasis on New Lead-free Solders", Release 4.0 (Feb 2002), Table 1.18 (Sn-37Pb 14.5 µΩ·cm; Sn-3.5Ag-0.7Cu and Sn-3.8Ag-0.7Cu 13 µΩ·cm) and Table 2.2.19 (Sn 11.5, Sn-37Pb 14.5 µΩ·cm) — https://www.msed.nist.gov/solder/NIST_LeadfreeSolder_v4.pdf — accessed 2026-11-02.
19. Kester, "Technical Data on Soft Solders" (Alloy Properties): Sn63Pb37 electrical conductivity 11.5 % of Cu — https://www.kester.com/Portals/0/Documents/Knowledge%20Base/Alloy%20Properties.pdf — accessed 2026-11-02.
20. Indium Corporation, Solder Alloy Directory (brochure 97720 R2): Indalloy 106 (63Sn37Pb) 11.5 % IACS — copy at https://xdevs.com/doc/_Materials/Chemistry/solder_alloy_directory_brochure_97720_r2.pdf — accessed 2026-11-02.
21. Indium Corporation, Product Data Sheet "Indalloy 256 (SAC305) for Preforms", Form No. 100346 R0 (2024): electrical resistivity 13.2 µΩ·cm — https://www.indium.com/wp-content/uploads/2025/03/Indalloy256-for-Preforms-PDS-100346-R0-1.pdf — accessed 2026-11-02.
22. ICS Schneider Messtechnik, "Pt100 characteristic curve according to IEC 60751: do not confuse α = 0.00385 …" (seen as web-search excerpt only) — https://www.ics-schneider.de/pt100-kennlinie-nach-iec-60751-%ce%b1-000385-und-abweichende-kennlinien-nicht-verwechseln/?lang=en — for the Pt100 note only.

Arithmetic (unit conversions, α derivations): a helper script that is not part of the repository.

## Cross-check
Method: ρ20 of pure metals = CRC 84th ed. value at 293 K (source 6); α of pure metals = textbook value (sources 3–5) checked against a local α derived from the CRC data, α ≈ (ρ293 − ρ273)/(20 K · ρ293) (second estimate with ρ300 gives the same to ±0.0001). Agreement = within about 5 %.

| row | ρ20 sources | α sources | verdict |
|---|---|---|---|
| silver | CRC 0.01587; Wiki/HP/ETB 0.0159 | Wiki 0.0038, HP 0.0038, ETB 0.00382, CRC-derived 0.00378 | agree |
| copper_iec60028 | IEC 60028 1/58; HP (CRC 64th) 0.0172; Wiki 0.0172; ETB 0.01724 | IEC 0.00393; HP 0.00393; Wiki 0.00393; ETB 0.00393 | agree |
| copper_pure | CRC 0.01678; Wiki/HP 0.0168 | Wiki 0.00404, CRC-derived 0.00402; HP 0.00386 disagrees | chosen 0.0040 |
| gold | CRC 0.02214; ETB 0.0224; Wiki/Serway 0.0244 disagrees | CRC-derived 0.0037; Wiki/Serway 0.0034 | ρ: CRC chosen (also used in rf.yaml); α UNVERIFIED (two values 8 % apart) |
| aluminium_iec60889 | IEC 60889 0.028264; Wiki 0.0282 | IEC 0.00403; Wiki 0.0039 | standard value kept (brief) |
| aluminium_pure | CRC 0.02650; HP 0.0265; ETB 0.0265 | HP 0.00429, CRC-derived 0.0044; ETB 0.0038 disagrees | chosen 0.0043 |
| brass_cuzn37 | DKI 0.066 (15 MS/m); Aurubis ≥16 MS/m = 0.0625; ETB (63 % Cu) 0.071 | DKI 0.0017 (0–100 °C); ETB 0.0015; DKI ρ(200 °C) gives mean 0.0014 | typical, spread shown |
| bronze_cusn6 | Bikar/DKI 0.111; Aurubis 9 MS/m = 0.111; Wiki (phosphor bronze 5 % Sn) 0.112 | DKI 0.00065 only (search excerpt); a Schlenk sheet quoted in a search excerpt as "0.065·10⁻³/K" — looks like a factor-10 typo, not opened | ρ agrees; α single source |
| nickel | CRC 0.0693; Wiki 0.0699; ETB 0.0685 | Wiki 0.0060; ETB 0.00641; CRC-derived 0.0056 | ρ agrees; α UNVERIFIED (spread 0.0056–0.0064), 0.0060 chosen |
| iron | CRC 0.0961; Wiki 0.0970; HP/ETB 0.0971 | CRC-derived 0.0054; Wiki 0.0050; HP 0.00651; ETB 0.00641 | ρ agrees (1 %); α UNVERIFIED, 0.0055 chosen as the CRC local slope |
| steel_low_carbon | Elgin 1010 0.143; Wiki 1010 0.143; ETB mild steel 0.15 | ETB 0.0066 only | ρ agrees (Elgin and Wikipedia may both trace to MatWeb); α UNVERIFIED |
| stainless_1_4301 | Matthey 0.73; Wiki 0.69 (generic); Total Materia 0.72–0.73 and Outokumpu 0.75 (search excerpts) | Wiki 0.00094 only | ρ 0.73 chosen; α UNVERIFIED |
| tin | Wiki 0.109; ETB 0.110; CRC/Kaye & Laby/NIST 0.115 (no temperature stated; one wiki says Kaye & Laby values are for 300 K) | Wiki 0.0045; ETB 0.0042 | ρ UNVERIFIED beyond 2 digits, 0.11 chosen; α approx |
| lead | CRC 0.208; ETB 0.206; Wiki/HP 0.22 | Wiki 0.0039; CRC-derived 0.0038 | CRC chosen; α agrees |
| tungsten | CRC 0.0528; Wiki 0.0560; HP 0.056; ETB 0.0565 | Wiki/HP/ETB 0.0045; CRC-derived 0.0044 | ρ disagreement 6 %: CRC chosen (modern handbook); α agrees |
| platinum | CRC 0.105; ETB 0.105; Wiki/HP 0.106 | Wiki 0.00392; HP 0.003927; ETB 0.00393 | agree (CRC-derived 0.0043 not usable: CRC 273 K value has only 2 digits) |
| zinc | CRC 0.0590; Wiki 0.0590; ETB 0.0592 | Wiki 0.0037; ETB 0.0037; CRC-derived 0.0037 | agree |
| constantan | Isabellenhütte 0.49 ±10 %; Kanthal Cuprothal 49 0.49; Wiki/HP/ETB 0.49 | Isabellenhütte −80…+40 ppm/K (DIN 17471, 20–105 °C); Kanthal ±60 ppm/K (special ±20); Wiki 8 ppm/K; ETB 30 ppm/K | ρ agrees; α given as maker band |
| manganin | Isabellenhütte 0.43 ±5 %; Isabellenhütte Tech-Info 0.43 (same maker); Wieland CuMn12Ni2 0.43 ±5 % and Goodfellow 0.43–0.48 seen only as search excerpts; Wiki/HP 0.482 (older Cu84Mn12Ni4) | Isabellenhütte ±10 ppm/K (20–50 °C); Rhopoint ±10 ppm/K (search excerpt); Wiki 2 ppm/K | maker value chosen; textbook value is a different composition |
| nichrome_8020 | Kanthal 1.09; Isabellenhütte 1.12 (1.08 rapid-cooled); Wiki 1.10; HP 1.00 ("Ni, Fe, Cr alloy"); ETB 1.00–1.50 | maker-derived 0.00011 (Isabellenhütte) and ~0.00013 (Kanthal Ct, coarse); Wiki/HP/ETB 0.0004 | ρ 1.10 chosen between the two makers; α UNVERIFIED — textbook 0.0004 likely refers to Ni-Fe-Cr nichrome, maker data for NiCr 80/20 used |
| solder_sn63pb37 | Kester 11.5 % Cu = 0.150; Indium 11.5 % IACS = 0.150; ETB "solder" 0.15; NIST 0.145 (14.5 µΩ·cm, 11.9 % IACS) | none | 0.150 chosen (two makers); NIST 3 % lower |
| solder_sac305 | Indium PDS 13.2 µΩ·cm; NIST 13 µΩ·cm for near-SAC alloys; Caplinq 13.1 (search excerpt) | none | agree within 2 % |

Discarded: NIST solder database Table 2.2.8 (Smithells 6th ed. pure-metal resistivities) — its rows are visibly inconsistent (e.g. Ag 1.47 µΩ·cm at "300 K", Pt TCR 3.0×10⁻³), probably mis-aligned columns; not used.

## Notes (en / de)
- en: Typical values at 20 °C. Real conductors differ with purity, alloy content and temper (cold work raises the resistivity). The linear model ρ(T) = ρ20 · (1 + α · (T − 20 °C)) holds only in a moderate range around 20 °C; for iron, nickel and tungsten the error grows quickly with temperature.
- de: Typische Werte bei 20 °C. Reale Leiter weichen je nach Reinheit, Legierungsgehalt und Zustand ab (Kaltverformung erhöht den spezifischen Widerstand). Das lineare Modell ρ(T) = ρ20 · (1 + α · (T − 20 °C)) gilt nur in einem mäßigen Bereich um 20 °C; bei Eisen, Nickel und Wolfram wächst der Fehler mit der Temperatur schnell.
- en: For precision alloys (manganin, constantan) the makers specify a band for α instead of a single value; inside that band the resistance is practically constant.
- de: Für Präzisionslegierungen (Manganin, Konstantan) geben die Hersteller statt eines Einzelwerts ein Band für α an; innerhalb dieses Bands ist der Widerstand praktisch konstant.
- en: 1 Ω·mm²/m = 1 µΩ·m = 100 µΩ·cm = 10⁻⁶ Ω·m; % IACS = 1.7241 µΩ·cm / ρ × 100.
- de: 1 Ω·mm²/m = 1 µΩ·m = 100 µΩ·cm = 10⁻⁶ Ω·m; % IACS = 1,7241 µΩ·cm / ρ × 100.

## Suggested finder / lookups
- Material picker for the existing "wire resistance" and "voltage drop" calculators: feed `rho20` and `alpha` (ECalc dims `resistivity` and `tempco`, unit `ohmmm2m` and 1/K). The `copper_iec60028` and `aluminium_iec60889` rows reproduce the constants already in passive.yaml / power.yaml exactly.
- Resistance at temperature: R(T) = R20 · (1 + α · (T − 20 °C)); for rows with only a band, show R(T) as a range using `alpha_min` / `alpha_max`.
- Reverse lookup: "which material has ρ closest to X" (sort by |rho20 − X|) or "conductivity in % IACS" = 0.017241 / rho20 × 100 (rho20 in Ω·mm²/m).
