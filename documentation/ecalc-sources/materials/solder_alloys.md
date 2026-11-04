# solder_alloys — Solder alloys / Lotlegierungen

Kind: typical values (melting data from maker datasheets and the NIST database; ISO 9453 alloy numbers transcribed from the standard and makers' documents)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id | Zeilen-ID | – | no |
| alloy | alloy name as usually written (mass %) | übliche Legierungsbezeichnung (Massen-%) | – (code) | no |
| common_name | trade/short name | Handels-/Kurzname | – (code) | no |
| iso9453_no | ISO 9453 alloy number (empty = not found) | ISO-9453-Legierungsnummer (leer = nicht gefunden) | – (code) | no |
| iso9453_name | ISO 9453 designation | ISO-9453-Kurzzeichen | – (code) | no |
| solidus | solidus temperature | Solidustemperatur | °C | yes |
| liquidus | liquidus temperature (= solidus for eutectic alloys) | Liquidustemperatur (= Solidus bei eutektischen Legierungen) | °C | yes |
| eutectic | eutectic (single melting point)? | eutektisch (ein Schmelzpunkt)? | – (yes\|no → en\|de) | no |
| lead_free | lead-free? | bleifrei? | – (en\|de) | no |
| density | density (typical) | Dichte (typisch) | g/cm³ | yes |
| use | typical use | typische Verwendung | – (en\|de) | no |
| flags | implementer-only: single-source or disputed cells (not user-facing) | nur für die Umsetzung: Einzelquelle oder strittig (nicht anzeigen) | – (English only) | no |
| note | user-facing remark | Hinweis für den Nutzer | – (en\|de) | no |

## Rows
```csv
id,alloy,common_name,iso9453_no,iso9453_name,solidus,liquidus,eutectic,lead_free,density,use,flags,note
sn63pb37,Sn63Pb37,63/37,101,S-Sn63Pb37,183,183,yes|ja,no|nein,8.4,"Hand, wave and reflow soldering of electronics (leaded)|Hand-, Wellen- und Reflowlöten in der Elektronik (bleihaltig)",ISO 102 = S-Sn63Pb37E (same alloy with tighter impurity limits; Stannol uses 102),Lead-containing: in the EU allowed only where a RoHS exemption applies.|Bleihaltig: in der EU nur zulässig wo eine RoHS-Ausnahme gilt.
sn60pb40,Sn60Pb40,60/40,103,S-Sn60Pb40,183,190,no|nein,no|nein,8.5,General-purpose solder wire for hand soldering|Universal-Lötdraht fürs Handlöten,"liquidus: ISO 9453, Kester, Stannol 190; Indium directory 191",Small melting range (pasty between 183 and 190 °C).|Kleiner Schmelzbereich (teigig zwischen 183 und 190 °C).
sac305,Sn96.5Ag3.0Cu0.5,SAC305,711,S-Sn96Ag3Cu1,217,220,no|nein,yes|ja,7.4,"Standard lead-free alloy for reflow, wave and hand soldering|Bleifreie Standardlegierung für Reflow-, Wellen- und Handlöten","Indium PDS density 7.35, Indium directory and Kester 7.40",About 35 °C higher melting than Sn63Pb37.|Schmilzt etwa 35 °C höher als Sn63Pb37.
sac405,Sn95.5Ag4.0Cu0.5,SAC405,,,217,219,no|nein,yes|ja,7.4,Lead-free reflow soldering and BGA solder spheres|Bleifreies Reflowlöten und BGA-Lotkugeln,"iso9453_no not found; liquidus UNVERIFIED: Kapp Alloy and Caplinq 219 (search excerpts only; pages 404/522), Kester chart 217 (single value), Indium directory R2 225; density single source (Indium); use (BGA spheres) from Kapp/Caplinq search excerpts",Close to the Sn-Ag-Cu eutectic (217 °C).|Nahe am Sn-Ag-Cu-Eutektikum (217 °C).
sn99cu07,Sn99.3Cu0.7,SnCu0.7 (Stannol: TC),401,S-Sn99Cu1,227,227,yes|ja,yes|ja,7.31,Low-cost lead-free alloy for wave and solder-bath soldering|Kostengünstige bleifreie Legierung für Wellen- und Tauchlöten,iso9453_no single source (Stannol); designation S-Sn99Cu1 also in Felder SDS; SN100C (Ni-stabilised) is ISO alloy 403 per DKL — different alloy,"ISO designation Sn99Cu1 although the composition is Sn99.3Cu0.7.|ISO-Kurzzeichen Sn99Cu1 obwohl die Zusammensetzung Sn99,3Cu0,7 ist."
sn965ag35,Sn96.5Ag3.5,96.5/3.5 (tin-silver),703,,221,221,yes|ja,yes|ja,7.4,Lead-free higher-temperature soldering; good thermal-fatigue resistance|Bleifreies Löten bei höherer Temperatur; gute Temperaturwechselfestigkeit,"iso9453_no UNVERIFIED (single source MakeItFrom, search excerpt only); iso9453_name not found; density: Kester 7.36, Indium 7.50",
sn42bi58,Sn42Bi58,Bi58Sn42 (tin-bismuth),301,S-Bi58Sn42,138,138,yes|ja,yes|ja,8.56,Low-temperature soldering of heat-sensitive parts and step soldering|Niedertemperaturlöten wärmeempfindlicher Teile und Stufenlöten,"iso9453_no single source (Stannol); melting: Indium 138 (eutectic), NIST 138; Stannol 139; density single source (Indium)",Melts about 45 °C below Sn63Pb37.|Schmilzt etwa 45 °C unter Sn63Pb37.
```

## Sources
1. ISO 9453:2006 "Soft solder alloys — Chemical compositions and forms", preview (iTeh) — Table 2: alloy 101 S-Sn63Pb37 183 °C, 102 S-Sn63Pb37E 183 °C, 103 S-Sn60Pb40 183/190 °C, 104 S-Sn60Pb40E; Table 3 (lead-free) not in the preview — https://cdn.standards.iteh.ai/samples/41923/8bea747c92a348408a23d8d531e1e089/ISO-9453-2006.pdf — accessed 2026-11-02.
2. Stannol, solder bar catalogue, table "Composition of solders" (alloy names and numbers according to ISO EN 9453:2006: 401 S-Sn99Cu1 = Sn99.3Cu0.7 227 °C; 701 S-Sn96Ag4 221 °C; 711 S-Sn96Ag3Cu1 = Sn96.5Ag3.0Cu0.5 217–220 °C; 713 S-Sn95Ag4Cu1 = Sn95.5Ag3.8Cu0.7 217 °C; 301 S-Bi58Sn42 139 °C; 102 S-Sn63Pb37E 183 °C; 103 S-Sn60Pb40 183–190 °C; RoHS lead-free / lead-containing) — copy at https://www.synergyelectronics.co.nz/catalogues/Stannol%20Solder%20Bar.pdf — accessed 2026-11-02.
3. DKL Metals, "E-Qual Lead Free Solid Solder Wire" data sheet: SAC305 = ISO 9453:2020(E) Alloy 711, solidus 217 °C, liquidus 220 °C, density 7.4; SN100C = Alloy 403, 227 °C — https://dklmetals.co.uk/wp-content/uploads/2023/09/E-Qual-Solid-Solder-Wire.pdf — accessed 2026-11-02.
4. Felder Löttechnik, technical data sheet "FELDER-ISO-Tin bleifrei, Sn96,5Ag3Cu0,5" (Stand 02/21): DIN EN ISO 9453:2014, melting range 217–220 °C — https://static.mercateo.com/79/4ec93a8d8fb44f73b3366f501184b2d8/pdf/Technisches-Datenblatt_Stangenzinn_Sn965Ag30Cu05.pdf — accessed 2026-11-02.
5. Felder Löttechnik, safety data sheet "Weichlotlegierung bleifrei, SnAg, SnCu, SnAgCu" (trade-name list with DIN EN ISO 9453 designations, e.g. Sn96,5Ag3,0Cu0,5 = Sn96Ag3Cu1, Sn99,3Cu0,7 = Sn99Cu1) — https://media.contorion.de/content/pdf/felder/at/S_4000872734_DE_20180215_de.pdf — accessed 2026-11-02.
6. Kester, "Alloy Temperature Chart" (Sn63Pb37 183; Sn60Pb40 183–190; Sn96.5Ag3.5 221; Sn99.3Cu0.7 227; Sn96.5Ag3Cu0.5 217–220; Sn95.5Ag4Cu0.5 217; Sn42Bi57Ag1 138) — https://www.kester.com/Portals/0/Documents/Knowledge%20Base/Alloy%20Temperature%20Chart.pdf — accessed 2026-11-02.
7. Kester, "Technical Data on Soft Solders" (solidus/liquidus, specific gravity: Sn63Pb37 183 e 8.41; Sn60Pb40 183/190 8.51; Sn99.3Cu0.7 227 e 7.31; Sn96.5Ag3.5 221 e 7.36; Sn96.5Ag3Cu0.5 217/220 7.40) — https://www.kester.com/Portals/0/Documents/Knowledge%20Base/Alloy%20Properties.pdf — accessed 2026-11-02.
8. Indium Corporation, Solder Alloy Directory (brochure 97720 R2): Indalloy 106 63Sn37Pb 183E 8.40; 109 60Sn40Pb 191/183 8.50; 256 SAC305 220/217 7.40 ("Preferred alloy to recommend"); 246 95.5Sn4.0Ag0.5Cu 225/217 7.40; 121 96.5Sn3.5Ag 221E 7.50 ("Lead-free high-temperature solder. Excellent thermal fatigue properties"); 244 99.3Sn0.7Cu 227E 7.31; 281 58Bi42Sn 138E 8.56 ("Makes a good low-temperature solder paste for electronics assembly") — copy at https://xdevs.com/doc/_Materials/Chemistry/solder_alloy_directory_brochure_97720_r2.pdf — accessed 2026-11-02.
9. Indium Corporation, PDS "Indalloy 256 (SAC305) for Preforms", Form No. 100346 R0: liquidus 220 °C, solidus 217 °C, density 7.35 g/cc — https://www.indium.com/wp-content/uploads/2025/03/Indalloy256-for-Preforms-PDS-100346-R0-1.pdf — accessed 2026-11-02.
10. NIST / Colorado School of Mines, "Database for Solder Properties with Emphasis on New Lead-free Solders", Release 4.0 (Feb 2002), Table 1.18 (Sn-3.5Ag 221 °C; Sn-37Pb 183 °C) and Table 2.2.19 (Sn-37Pb 183; Sn-3.5Ag 221; Sn-0.7Cu 227; Sn-58Bi 138 °C) — https://www.msed.nist.gov/solder/NIST_LeadfreeSolder_v4.pdf — accessed 2026-11-02.
11. R. Pandher (Cookson Electronics), "Step 3: Solder Materials: Melting and Freezing Characteristics of Common Lead-free Alloys", I-Connect007/SMT007 (Sn-Ag-Cu eutectic ≈ Sn3.7Ag0.9Cu at 217 °C; SAC305 and SAC405 close to it) — https://iconnect007.com//article/43869/step-3-solder-materialsbrmelting-and-freezing-characteristics-of-common-leadfree-alloys/43872/smt — accessed 2026-11-02.
12. Seen only as web-search excerpts (pages not opened or not reachable): MakeItFrom "ISO Solder Alloy 703 IEC A35 Sn96.5Ag3.5" (https://www.makeitfrom.com/material-properties/ISO-Solder-Alloy-703-IEC-A35-Sn96.5Ag3.5); Kapp Alloy SAC405 TDS "217–219 °C" (https://www.kappalloy.com/solder-alloy/sac-solders/sac405/, HTTP 429/404); Caplinq SAC405 "Solidus/Liquidus 217/219 °C" (https://storefront.caplinq.com/products/sac405, HTTP 522).

## Cross-check
| row | solidus/liquidus sources | ISO 9453 sources | verdict |
|---|---|---|---|
| sn63pb37 | ISO 9453 183; Kester 183; Indium 183E; Stannol 183; NIST 183 | ISO 9453:2006 preview (101; 102 = E-grade); Stannol (102) | agree |
| sn60pb40 | ISO 9453 183/190; Kester 183–190; Stannol 183–190; Indium 183/191 | ISO preview (103); Stannol (103) | 183/190 chosen (standard + 2 makers); Indium 1 °C higher |
| sac305 | Kester 217/220; Indium 217/220 (directory and PDS); Stannol 217–220; Felder 217–220; DKL 217/220 | Stannol 711 (2006 ed.); DKL 711 (2020 ed.); Felder SDS designation S-Sn96Ag3Cu1 | agree |
| sac405 | solidus 217: Kester, Indium, Kapp/Caplinq excerpts; liquidus: 217 (Kester), 219 (Kapp, Caplinq — excerpts), 225 (Indium R2) | none found | liquidus UNVERIFIED; 219 chosen as the value two vendors quote, range 217–225 |
| sn99cu07 | Kester 227; Indium 227E; Stannol 227; NIST 227 | Stannol 401 (number); Felder SDS designation Sn99Cu1 | melting agrees; number single source |
| sn965ag35 | Kester 221 e; Indium 221E; NIST 221 (two tables) | MakeItFrom 703 only (excerpt) | melting agrees; ISO number UNVERIFIED |
| sn42bi58 | Indium 138E; NIST 138 (two tables); Stannol 139 | Stannol 301 only | 138 chosen (eutectic per Indium + NIST); ISO number single source |

Lead-free yes/no follows from the composition (no Pb) and matches Stannol's RoHS column.
J-STD-006 designations: not found in any source read — not included.

## Notes (en / de)
- en: Typical values from maker datasheets. Non-eutectic alloys are pasty between solidus and liquidus; eutectic alloys melt at one temperature. Use the maker's datasheet for the soldering profile.
- de: Typische Werte aus Herstellerdatenblättern. Nicht-eutektische Legierungen sind zwischen Solidus und Liquidus teigig; eutektische Legierungen schmelzen bei einer Temperatur. Für das Lötprofil das Herstellerdatenblatt verwenden.
- en: Lead-containing solders (Sn63Pb37, Sn60Pb40) are restricted by RoHS in the EU; use them only where an exemption applies (e.g. repairs of old equipment).
- de: Bleihaltige Lote (Sn63Pb37, Sn60Pb40) sind in der EU durch RoHS beschränkt; nur verwenden, wo eine Ausnahme gilt (z. B. Reparatur alter Geräte).

## Suggested finder / lookups
- "Which alloys melt below X °C?" → liquidus ≤ X; "lead-free only" filter on `lead_free`.
- Show melting range as "solidus–liquidus" or a single value when eutectic.
