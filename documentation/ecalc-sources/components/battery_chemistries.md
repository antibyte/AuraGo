# battery_chemistries — Battery chemistries: cell voltages and energy density / Batteriechemien: Zellspannungen und Energiedichte

Kind: typical values
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| chem | chemistry | Chemie | – | no (en/de text) |
| iec_letter | IEC letter code of the electrochemical system (IEC 60086 / 61951 / 61960 naming) | IEC-Kennbuchstabe des Systems | – | no |
| rechargeable | rechargeable? | wiederaufladbar? | – | no (yes/no) |
| v_nom | nominal voltage per cell | Nennspannung je Zelle | V | yes |
| v_charge | end-of-charge (CV) voltage per cell; empty = not charged with a voltage limit | Ladeschlussspannung (CV) je Zelle; leer = keine Spannungsgrenze | V | yes |
| v_cutoff | typical discharge cut-off (end) voltage per cell | typische Entladeschlussspannung je Zelle | V | yes |
| wh_kg_typ | typical specific energy range from the handbook source (text "min–max") | typische spezifische Energie laut Handbuchquelle (Text „min–max“) | Wh/kg | no (range text) |
| wh_kg_example | example cell: V_nom × capacity / mass from the cited datasheet | Beispielzelle: U_nenn × Kapazität / Masse laut Datenblatt | Wh/kg | yes |
| example_cell | the datasheet cell used for wh_kg_example | Datenblattzelle für wh_kg_example | – | no |
| note | remarks | Bemerkungen | – | no (en/de text) |

## Rows
```csv
chem,iec_letter,rechargeable,v_nom,v_charge,v_cutoff,wh_kg_typ,wh_kg_example,example_cell,note
"Alkaline (Zn/MnO2)|Alkali-Mangan (Zn/MnO2)",L,no,1.5,,0.8,,163,"Energizer E91 AA (23.0 g; 2500 mAh at 100 mA to 0.8 V per handbook)","Fresh open-circuit voltage about 1.6 V; devices often stop at 0.9 V; example uses V_nom, so it is an upper bound|Leerlaufspannung neu etwa 1,6 V; Geräte schalten oft bei 0,9 V ab; Beispiel mit U_nenn gerechnet, daher Obergrenze"
"NiMH|NiMH",H,yes,1.2,,1.0,,89,"Panasonic eneloop BK-3MCC AA (2000 mAh typ., 27 g)","Charged with constant current and −ΔV or dT/dt termination, not a fixed voltage|Laden mit Konstantstrom und −ΔU- bzw. dT/dt-Abschaltung, nicht mit fester Spannung"
"NiCd|NiCd",K,yes,1.2,,1.0,,45,"Saft VRE AA 700 (780 mAh typ., 21 g)","Charged like NiMH (−ΔV or dT/dt end of charge)|Laden wie NiMH (−ΔU- bzw. dT/dt-Ladeende)"
"Li-ion (NMC/NCA)|Li-Ion (NMC/NCA)",IN,yes,3.6,4.2,2.5,150–220,256,"Samsung INR21700-50E (4900 mAh min., 3.63 V, 69.5 g max.)","Nominal 3.6–3.7 V; many applications cut off at 3.0 V; Panasonic NCR18650B states 243 Wh/kg|Nenn 3,6–3,7 V; viele Anwendungen schalten bei 3,0 V ab; Panasonic NCR18650B nennt 243 Wh/kg"
"LiFePO4 (LFP)|LiFePO4 (LFP)",IF,yes,3.3,3.6,2.5,90–120,109,"A123 ANR26650M1-B (2.5 Ah, 76 g)","Nominal 3.2–3.3 V; charge limit 3.6–3.65 V; cut-off 2.0–2.7 V in the sources|Nenn 3,2–3,3 V; Ladegrenze 3,6–3,65 V; Entladeschluss in den Quellen 2,0–2,7 V"
"Li-polymer (LiPo, pouch)|Lithium-Polymer (LiPo, Pouch)",IC,yes,3.7,4.2,3.0,,150,"Renata ICP402035 (195 mAh, 4.8 g)","Same electrochemistry as Li-ion with pouch/polymer construction; IEC 61960 name of the example: ICP052136|Gleiche Elektrochemie wie Li-Ion, Pouch-/Polymer-Bauweise; IEC-61960-Bezeichnung des Beispiels: ICP052136"
"Lead-acid (VRLA/AGM)|Blei-Säure (VRLA/AGM)",,yes,2.0,2.42,1.75,,38,"Yuasa NP7-12L 12 V 7 Ah (20 h), 2.2 kg","Float charge 2.275 V/cell, cyclic charge 2.42 V/cell at 20 °C; 1.75 V/cell is the end voltage of the 20-hour rating|Erhaltungsladung 2,275 V/Zelle, Zyklenladung 2,42 V/Zelle bei 20 °C; 1,75 V/Zelle ist die Endspannung der 20-h-Kapazität"
"Lithium primary (Li/MnO2, CR)|Lithium-Primär (Li/MnO2, CR)",C,no,3.0,,2.0,,241,"Panasonic CR2032 (225 mAh at 0.2 mA, 2.8 g)","Cut-off 2.0 V is UNVERIFIED for CR2032 (not in the Panasonic CR2032 datasheet)|Entladeschluss 2,0 V ist für CR2032 UNVERIFIED (nicht im Panasonic-CR2032-Datenblatt)"
```

## Sources
1. Energizer Brands, "Alkaline Manganese Dioxide – Handbook and Application Manual" (©2018), https://data.energizer.com/pdfs/alkaline_appman.pdf — minimum cut-off 0.8 V per cell, typical device range 1.6 V to 0.9 V, OCV example 1.6 V, AA/LR6 2500 mAh at 100 mA to 0.8 V; accessed 2026-11-02.
2. Energizer, "Product datasheet Energizer E91" (AA), copy https://cdn.connectec.uk/Files/Products/Batteries/energizer_batteries/e91_aa_battery_datasheet.pdf — Zn/MnO2, ANSI-15A, IEC-LR6, 1.5 V, 23.0 g; accessed 2026-11-02.
3. Panasonic, "Cell Type BK-3MCC Specifications" (eneloop AA NiMH), copy https://www.peak-system.com/produktcd/Tools/PCAN-Diag%20FD/Documentation/Addendum%20Batteries/Panasonic_Cell%20Type%20BK-3MCC.pdf — 1.2 V, 2000 mAh typ./1900 mAh min. (discharge to E.V. = 1.0 V), approx. 27 g, charge −ΔV = 10 mV; accessed 2026-11-02.
4. Saft, "VRE AA 700, Standard Series Nickel-Cadmium", Doc. N° 11091-2-0604, copy https://static.datasheets.com/doc/17856711-saft-789056-ds.pdf — 1.2 V, 780 mAh typ., 700 mAh IEC min., IEC KRMR 15/51, 21 g, end of fast charge by −dV or dT/dt; accessed 2026-11-02.
5. Panasonic (SANYO), "NCR18650B Lithium Ion" datasheet, Version 13.11 R1, copy https://www.batteryspace.com/prod-specs/NCR18650B.pdf — 3.6 V nominal, CC-CV 4.20 V, discharge to 2.50 V, 48.5 g max., 243 Wh/kg; accessed 2026-11-02.
6. Samsung SDI, "Specification of Product INR21700-50E", Spec. No. INR21700-50E, Version V1.0, copy https://shop.aceongroup.com/cdn/shop/files/RLII-CSI-1618_Samsung_50E_DATASHEET.pdf — 4.2 V charge, 3.63 V nominal, 2.5 V discharge cut-off, min. 4900 mAh, 69.5 g max.; accessed 2026-11-02.
7. A123 Systems, "Nanophosphate High Power Lithium Ion Cell ANR26650M1-B", MD100113-02 (©2012), copy https://bto.pl/files/Karty%20katalogowe/a123%20datasheet_26650m1b.pdf — 3.3 V nominal, 2.5 A to 3.6 V CCCV, 2.5 Ah, 76 g, 26 × 65 mm; accessed 2026-11-02.
8. Renata SA, "ICP402035 Rechargeable Lithium Ion Polymer Battery Pack 3.7 V", Rev. Li Ion ICP402035-11.02, copy https://static.rapidonline.com/pdf/12-8371_v1.pdf — 3.7 V, 195 mAh, 4.2 V CV, cut-off 3.0 V, ~4.8 g, IEC designation ICP052136; accessed 2026-11-02.
9. Yuasa, "Yuasa NP7-12L Industrial VRLA Battery, Technical Data Sheet", published 26/05/2025, copy https://docs.rs-online.com/408f/A700000015456382.pdf — 12 V, 7 Ah (20 h to 1.75 V/cell), 2.2 kg, float 13.65 V/block = 2.275 V/cell, cyclic 14.5 V/block = 2.42 V/cell; accessed 2026-11-02.
10. Panasonic Energy, "CR2032 CR Coin-type Lithium Battery", as of February 2026, https://energy.panasonic.com/dam/master/pdf/en/datasheet/lithium/CR2032_Datasheet_EN.pdf — 3 V, 225 mAh (0.2 mA), Ø 20.0 × 3.2 mm max., approx. 2.8 g; accessed 2026-11-02.
11. Battery University (Cadex), "BU-205: Types of Lithium-ion", last updated 8-Dec-2023, https://batteryuniversity.com/article/bu-205-types-of-lithium-ion — NMC 3.60/3.70 V, 4.20 V, 2.50 V cut-off, 150–220 Wh/kg; LFP 3.20/3.30 V, 3.65 V, 2.50 V cut-off, 90–120 Wh/kg; accessed 2026-11-02.
12. Battery University (Cadex), "BU-501: Basics about Discharging", last updated 27-Oct-2021, https://batteryuniversity.com/article/bu-501-basics-about-discharging — end-of-discharge voltages: lead acid 1.75 V/cell, NiCd/NiMH 1.00 V/cell, Li-phosphate 2.70 V/cell (normal load), most Li-ion 3.0 V/cell; accessed 2026-11-02.
13. Wikipedia, "Battery nomenclature", https://en.wikipedia.org/wiki/Battery_nomenclature — IEC letters L (alkaline), C (Li/MnO2), H (NiMH), K (NiCd); IEC 61960 scheme (I = lithium ion, C/N/M/V/T = positive electrode, R/P = shape); accessed 2026-11-02.

## Cross-check
- Typical data → one maker datasheet per value group is cited; where a handbook also gives a value it is listed:
  - v_cutoff NiMH/NiCd 1.0 V: eneloop (3) + BU-501 (12). Lead-acid 1.75 V: Yuasa (9) + BU-501. Alkaline 0.8 V: Energizer handbook (1) only.
  - Li-ion: 4.2 V / 2.5 V: Panasonic (5), Samsung (6), BU-205 (11) agree; BU-501 says most Li-ion stop at 3.0 V (application cut-off) → note.
  - LFP: charge 3.6 V (A123) vs 3.65 V (BU-205); cut-off: A123 gives none (curves end at 2.0 V), BU-205 2.50 V, BU-501 2.70 V (normal load) → chosen 2.5 V, disagreement in the note.
  - wh_kg_typ ranges only exist for NMC and LFP (BU-205); the BU-107 comparison table is an image and could not be read, so other chemistries have no typical range here.
  - wh_kg_example values were computed with a helper script that is not part of the repository from the datasheet numbers (alkaline 163.0, NiMH 88.9, NiCd 44.6, NMC 255.9, LFP 108.6, LiPo 150.3, lead-acid 38.2, CR2032 241.1 Wh/kg).
- iec_letter: from Wikipedia's IEC nomenclature summary (13) and maker designations (E91 "IEC-LR6", Saft "KRMR 15/51", Renata "ICP052136", Samsung "INR…"). Lead-acid has no IEC 60086/61951/61960 letter. "IF" for LiFePO4: the commercial prefix (IFR) is common, but Wikipedia's IEC 61960 list of positive-electrode letters (C, N, M, V, T) has no F → UNVERIFIED.
- UNVERIFIED: CR (Li/MnO2) cut-off 2.0 V; LFP iec_letter "IF".

## Notes (en / de)
- en: Typical values; always use the datasheet of the actual cell. Specific energy depends on cell size and design (high-power cells have less). Li-ion charging needs a CC/CV charger with the exact end voltage; never charge primary cells (alkaline, CR). LiPo and Li-ion differ in construction, not in cell voltage.
- de: Typische Werte; immer das Datenblatt der konkreten Zelle verwenden. Die spezifische Energie hängt von Zellgröße und Bauart ab (Hochstromzellen haben weniger). Li-Ionen-Zellen nur mit CC/CV-Lader und exakter Ladeschlussspannung laden; Primärzellen (Alkali, CR) nie laden. LiPo und Li-Ion unterscheiden sich in der Bauweise, nicht in der Zellspannung.

## Suggested finder / lookups
- Pack voltage = n × v_nom; pack charge voltage = n × v_charge; pack cut-off = n × v_cutoff.
- Number of cells in series for a target voltage: ceil(V_target / v_nom).
