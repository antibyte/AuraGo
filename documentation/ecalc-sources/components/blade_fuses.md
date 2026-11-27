# blade_fuses — Automotive blade fuses: rated current and housing colour / Kfz-Flachsicherungen: Nennstrom und Gehäusefarbe

Kind: transcribed industry colour code (maker catalogues; ISO 8820-3 itself not read)
Guide-value note needed: yes (identification aid; the marked rating counts)

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id (`std_10` = 10 A in the standard code, `maxi_40` = 40 A MAXI); key | Zeilen-Id; Schlüssel | – | no |
| family | colour-code family | Farbcode-Familie | – | no (en/de text) |
| rating | rated current | Nennstrom | A | yes |
| colour | housing colour | Gehäusefarbe | – | no (en/de text) |
| formats | formats in which at least two makers offer this rating (micro2, micro3, LP mini, mini, regular, MAXI) | Bauformen, in denen mindestens zwei Hersteller diesen Nennstrom anbieten | – | no (en/de text) |
| remark | remark | Hinweis | – | no (en/de text) |

No colour-swatch (hex) column: the tables show colours as text only (`wire_colours` has `colour` with en/de names). The Littelfuse swatch RGB values read from the current datasheets are listed under Cross-check for reference only.

## Rows
```csv
id,family,rating,colour,formats,remark
std_1,"standard code|Standard-Farbcode",1,"black|schwarz",regular,"Bussmann also lists a 1 A Mini (ATM-1); only one maker, so not listed as Mini|Bussmann führt auch eine 1-A-Mini (ATM-1); nur ein Hersteller, daher nicht als Mini aufgeführt"
std_2,"standard code|Standard-Farbcode",2,"grey|grau","LP mini, mini, regular",
std_3,"standard code|Standard-Farbcode",3,"violet|violett","LP mini, mini, regular","Littelfuse also offers 3 A as Micro2 (327) and Micro3 (337); only one maker, so not listed|Littelfuse bietet 3 A auch als Micro2 (327) und Micro3 (337) an; nur ein Hersteller, daher nicht aufgeführt"
std_4,"standard code|Standard-Farbcode",4,"pink|rosa","LP mini, mini, regular",
std_5,"standard code|Standard-Farbcode",5,"tan (beige)|beige (hellbraun)","micro2, micro3, LP mini, mini, regular","HELLA and MTA name the colour beige|HELLA und MTA nennen die Farbe beige"
std_7_5,"standard code|Standard-Farbcode",7.5,"brown|braun","micro2, micro3, LP mini, mini, regular",
std_10,"standard code|Standard-Farbcode",10,"red|rot","micro2, micro3, LP mini, mini, regular",
std_15,"standard code|Standard-Farbcode",15,"blue|blau","micro2, micro3, LP mini, mini, regular",
std_20,"standard code|Standard-Farbcode",20,"yellow|gelb","micro2, LP mini, mini, regular",
std_25,"standard code|Standard-Farbcode",25,"natural (clear/white)|farblos (natur/weiß)","micro2, LP mini, mini, regular","Makers name it natural (Littelfuse, MTA), clear (Bussmann, OptiFuse) or white (HELLA)|Die Hersteller nennen sie natur (Littelfuse, MTA), klar (Bussmann, OptiFuse) oder weiß (HELLA)"
std_30,"standard code|Standard-Farbcode",30,"green|grün","micro2, LP mini, mini, regular",
std_35,"standard code|Standard-Farbcode",35,"blue-green|blaugrün","mini, regular","HELLA calls it dark green; not in the MTA standard range|HELLA nennt sie dunkelgrün; nicht im Standard-Sortiment von MTA"
std_40,"standard code|Standard-Farbcode",40,"orange|orange",regular,
maxi_20,"MAXI|Maxi",20,"yellow|gelb",MAXI,
maxi_25,"MAXI|Maxi",25,"grey|grau",MAXI,
maxi_30,"MAXI|Maxi",30,"green|grün",MAXI,
maxi_35,"MAXI|Maxi",35,"brown|braun",MAXI,"Not 70 A: brown is 35 A in the MAXI code|Nicht 70 A: Braun ist im Maxi-Code 35 A"
maxi_40,"MAXI|Maxi",40,"orange|orange",MAXI,
maxi_50,"MAXI|Maxi",50,"red|rot",MAXI,
maxi_60,"MAXI|Maxi",60,"blue|blau",MAXI,
maxi_70,"MAXI|Maxi",70,"tan (beige)|beige (hellbraun)",MAXI,"HELLA lists brown for 70 A; four other makers give tan/beige|HELLA nennt für 70 A braun; vier andere Hersteller nennen beige/hellbraun"
maxi_80,"MAXI|Maxi",80,"natural (clear)|farblos (natur)",MAXI,"HELLA names it white|HELLA nennt sie weiß"
maxi_100,"MAXI|Maxi",100,"violet|violett",MAXI,"Not offered by Littelfuse (299 ends at 80 A) or HELLA; Bussmann only as indicating type MAX-100ID|Nicht bei Littelfuse (299 endet bei 80 A) und HELLA; Bussmann nur als Ausführung mit Anzeige MAX-100ID"
```

Units: `rating` in A. `formats` lists only what at least two makers offer.

## Sources
1. Littelfuse, current datasheets (read in the browser from littelfuse.com; colours appear only as swatches, without names). Used for the rating ranges per format and as a swatch cross-check; accessed 2026-11-27.
   - "ATOF® Series Blade Fuses – Rated 32V", revised 02/04/2025, page 2 "Ratings" (0287001 … 0287040, 1–40 A incl. 35 A), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-287-atof?assetguid=43dcdce8-8ca2-426f-8998-7e566f048d40
   - MINI® 32V (297 series), revised 09/27/2024, page 2 "Ratings" (2, 3, 4, 5, 7.5, 10, 15, 20, 25, 30 A + shunt), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-297-mini32v?assetguid=42c9dd21-a88e-4328-8e67-2f832444faf1
   - LP-MINI (891 series), revised 09/27/2024, page 2 "Ratings" (2, 5, 7.5, 10, 15, 20, 25, 30 A), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-891-lp-mini?assetguid=748f2a1b-6fd1-43cb-b900-7178f45585a0
   - MICRO2™ (327 series), revised 07/01/2024, page 2 "Ratings" (3, 5, 7.5, 10, 15, 20, 25, 30 A + shunt), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-327-micro2?assetguid=a43b3ed7-5f7d-40ea-888f-4a2d42aeebe8
   - MICRO3™ (337 series), revised 01/06/2025, page 2 "Ratings" (3, 5, 7.5, 10, 15 A + shunt), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-337-micro3?assetguid=a65b4c69-57d2-42b8-8c9b-13c982b9f65a
   - MAXI™ Slo-Blo® 32V (299 series), revised 01/06/2025, page 2 "Ratings" (20, 25, 30, 35, 40, 50, 60, 70, 80 A), https://www.littelfuse.com/assetdocs/littelfuse-datasheet-299-maxi32v?assetguid=68b8bbe8-3e51-4c7b-bbda-3c71667f75f3
2. Littelfuse, catalogue section "Blade Terminal & Special Purpose Fuses" (PDF created 2005-05-17), pages 450 (ATO® 257 series, "Body Color Code" column), 451 (MINI® 297), 453 (MAXI™ 299), colour names in text; copy at https://www.farnell.com/datasheets/237771.pdf; accessed 2026-11-27. (An older ATO page with the same table, page 86, PDF 2001: https://www.farnell.com/datasheets/5428.pdf.)
3. Littelfuse, aftermarket catalogue page "FUSES 5" (ATO®, MINI®, MAXI™ colour code lists; PDF created 1998, modified 2006-10-13), https://www.farnell.com/datasheets/82316.pdf; accessed 2026-11-27.
4. Eaton, "Bussmann series full line catalog 1007 — October 2018", section 2 "Low voltage supplemental fuses", page 2-12 (ATM / ATM-LP and ATR micro blade fuse tables with "Color code") and page 2-13 (MAX (MAXI) and ATC tables with "Color code"); RS copy https://docs.rs-online.com/74b4/A700000014064745.pdf; accessed 2026-11-27. (Also seen: Eaton product specification sheet ATM-5, © 2023, "Color code tan", https://assets.rs-online.com/v1678201146/Datasheets/e7481879692804b022b7a7d377c2572d.pdf.)
5. OptiFuse (San Diego, USA) datasheets, https://www.optifuse.com/optifuse_ecommerce_tools/datasheets/…, each with a "Body Color" column; accessed 2026-11-27:
   - APR "Automotive Blade Fuse Standard", Rev B 08/2025 (`APR.pdf`), 1–40 A incl. 12 A "Light Pink" and 35 A "Blue Green"
   - APM "Miniature Blade Fuse", Rev K 03/2025 (`APM.pdf`), 2–35 A; APM-UL Rev B 06/2018 (`APM-UL.pdf`), 2–30 A
   - APS "Automotive Blade Fuse Low Profile Miniature", Rev B 06/2018 (`APS.pdf`), 2–30 A
   - APT "Tiny (Micro2 Style) Blade Fuse", Rev C 03/2025 (`APT.pdf`), 5–30 A
   - ANK "MICRO3 Style Blade Fuse", Rev B 04/2024 (`ANK.pdf`), 5, 7.5, 10, 15 A; ANK58 Rev B 03/2025 (`ANK58.pdf`) same colours
   - APX "Large (Maxi Style) Blade Fuse", Rev E 03/2025 (`APX.pdf`), 20, 30, 40, 50, 60, 70, 80, 100, 120 A
6. HELLA (FORVIA), "Brief information — Blade fuses" (`BI_Fuses_2026`), product range lists STANDARD 1–40 A, MINI 2–35 A, MINI LP 2–30 A, MAXI 20–80 A with colour names, https://www.hella.com/forvia-us/assets/documents/BI_Fuses_2026.pdf; accessed 2026-11-27.
7. MTA S.p.A., "Automotive Fuses 2018" (v1.1), page 6 (miniVAL® 2–30 A, uniVAL® 1–40 A without 35 A) and page 9 (maxiVAL® 20–100 A) with colour names; copy on a retailer's CDN https://cdn.shopify.com/s/files/1/0029/2827/9610/files/MTA_Automotive_Fuses_2018_v1.1.pdf; accessed 2026-11-27.

Not read: ISO 8820-3 (paywalled). The makers state conformity (Littelfuse ATOF "Comply With: SAE J1284 and ISO 8820-3", OptiFuse "Designed to meet … ISO 8820-3", HELLA "ISO 8820-3"); SAE J1284/J2077/J1888 not read.

## Cross-check
- Standard code, colour names: every colour from 1 A to 40 A is given in text by at least four independent makers (Littelfuse 2005/aftermarket, Bussmann 2018, OptiFuse 2018–2025, HELLA 2026, MTA 2018), and they agree except for naming variants: 5 A tan = beige (HELLA, MTA); 25 A natural = clear = white; 35 A blue-green (Littelfuse, Bussmann, OptiFuse) vs. "dark green" (HELLA).
- MAXI colours: 20 yellow, 30 green, 40 orange, 50 red, 60 blue agree in all five makers' documents; 25 grey and 35 brown only exist at Littelfuse and Bussmann (both agree); 70 A: tan (Littelfuse, Bussmann, OptiFuse) = beige (MTA) vs. brown (HELLA, the only dissent); 80 A natural/clear at the other four makers, white at HELLA; 100 A violet (MTA, OptiFuse) = purple (Bussmann).
- "All six sources" means the six documents of sources 2–7 (Littelfuse 2005 catalogue, Littelfuse aftermarket page, Bussmann, OptiFuse, HELLA, MTA), i.e. five makers.
- Current Littelfuse datasheets (source 1) carry no colour names, only swatches; the swatch fill colours sit at the y position of each table row and match the names above: grey #A7A9AC, violet #A860A7, pink #F49AC2, tan #D0924D, brown #A45E48, red #ED1C24, blue #0094C9, yellow #FFF200, natural #FFEBCB (MINI/ATOF/MAXI 25/80 A) or #FFFFFF (LP-MINI, MICRO2 25 A), green #008752, ATOF 30 A #53A986 vs. 35 A #008752 (the darker blue-green), orange #F58220, MAXI 25 A grey #BCBEC0. The 1 A ATOF swatch (black) was not found; not needed, the name is in five other sources.
- Formats: a format is listed for a rating only if at least two makers offer it. Micro2: Littelfuse 327 + Bussmann ATR + OptiFuse APT agree on 5–30 A. Micro3: Littelfuse 337 + OptiFuse ANK on 5–15 A. LP mini: Bussmann ATM-LP, OptiFuse APS and HELLA 2–30 A; Littelfuse 891 has 2, 5, 7.5–30 A. Mini: 2–30 A at all makers, 35 A at OptiFuse APM and HELLA. Regular: 1–40 A at Littelfuse, Bussmann, OptiFuse, HELLA (MTA without 35 A).

## Deliberate deviations
- One colour code row per rating for the five small formats instead of one row per format: the makers use the same colours across micro2, micro3, LP mini, mini and regular (Bussmann even prints one colour column for ATM and ATM-LP); the `formats` cell says where the rating exists.
- Colour names: the English cell uses the majority name with the variant in brackets (tan (beige), natural (clear/white)); German names are translations, not quoted from a German maker document (the HELLA and MTA documents read were in English).
- The MAXI time characteristic (Littelfuse "Slo-Blo", HELLA "Slow-blow", Bussmann catalogue headline "fast-acting") is not part of this table.
- The research-only cross-check column is not shipped; its content is this section.

## UNVERIFIED (not shipped)
- 12 A, "light pink", regular: only OptiFuse APR lists it.
- 120 A MAXI "pink": only OptiFuse APX lists it.
- 1 A as MINI (Bussmann ATM-1 only), 3 A as MICRO2 and MICRO3 (Littelfuse only): left out of `formats`, named in `remark`.
- 25 A and 35 A MAXI exist only at Littelfuse and Bussmann; two makers, so they ship.
- Snippet-only claims not used: a search snippet called Bussmann ATC-40 "amber" (the Bussmann catalogue read says Orange); not counted.

## Notes (en / de)
- en: The colour only helps to identify the rating; what counts is the rating marked on the fuse. Always replace a fuse with one of the same rating and type; never fit a higher rating, the fuse protects the wiring. MAXI fuses use a different colour code: a green MAXI is 30 A like a green mini fuse, but a brown MAXI is 35 A while a brown mini fuse is 7.5 A.
- de: Die Farbe dient nur zum Erkennen des Nennstroms; maßgeblich ist der auf der Sicherung aufgedruckte Wert. Eine Sicherung immer durch eine gleichen Nennstroms und gleicher Bauform ersetzen, nie durch eine stärkere; die Sicherung schützt die Leitung. Maxi-Sicherungen haben einen eigenen Farbcode: Grün ist bei Maxi wie bei Mini 30 A, Braun aber bei Maxi 35 A und bei Mini 7,5 A.
- en: The formats differ mechanically (micro2 has two blades, micro3 three blades with two fuse elements, LP mini a lower body than mini); the same colour does not make them interchangeable.
- de: Die Bauformen sind mechanisch verschieden (Micro2 mit zwei Kontaktmessern, Micro3 mit drei Kontaktmessern und zwei Schmelzleitern, Mini Low Profile flacher als Mini); gleiche Farbe heißt nicht austauschbar.

## Suggested finder / lookups
- Colour → rating (filter by family standard / MAXI); rating → colour; "does format X come in Y A" via `formats`.
