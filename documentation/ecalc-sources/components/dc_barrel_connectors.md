# dc_barrel_connectors — DC barrel (coaxial power) connector sizes / Hohlstecker-Größen (DC-Koaxialstecker)

Kind: transcribed standard (JEITA RC-5320A rows) + typical values (common non-standard sizes)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| name | size name / standard type | Größenbezeichnung / Normtyp | – | no |
| standard | standard or "common" | Norm oder „verbreitet“ | – | no |
| od_mm | plug outer diameter (barrel) | Außendurchmesser des Steckers (Hülse) | mm | yes |
| id_mm | plug inner diameter (hole for the jack's centre pin) | Innendurchmesser des Steckers (Loch für den Mittelstift der Buchse) | mm | yes |
| pin_mm | plug's own centre pin diameter (RC-5320A types 4/5 only) | Mittelstift im Stecker (nur RC-5320A Typ 4/5) | mm | yes |
| v_min | lower end of the voltage class | untere Grenze der Spannungsklasse | V | yes |
| v_max | rated voltage (upper end of class) | Bemessungsspannung (obere Grenze) | V | yes |
| i_max_a | rated current of the cited plugs | Bemessungsstrom der zitierten Stecker | A | yes |
| use | typical use / remarks | typische Verwendung / Bemerkungen | – | no (en/de text) |

## Rows
```csv
name,standard,od_mm,id_mm,pin_mm,v_min,v_max,i_max_a,use
EIAJ-01 (JSAP1),JEITA RC-5320A type 1,2.35,0.7,,0,3.15,2,"Voltage class up to 3.15 V|Spannungsklasse bis 3,15 V"
EIAJ-02 (JSAP2),JEITA RC-5320A type 2,4.0,1.7,,3.15,6.3,2,"Voltage class 3.15–6.3 V; = the common 4.0 × 1.7 plug (e.g. Sony PSP charger)|Spannungsklasse 3,15–6,3 V; = der verbreitete 4,0 × 1,7 (z. B. Sony-PSP-Ladegerät)"
EIAJ-03 (JSAP3),JEITA RC-5320A type 3,4.75,1.7,,6.3,10.5,2,"Voltage class 6.3–10.5 V|Spannungsklasse 6,3–10,5 V"
EIAJ-04 (JSBP4),JEITA RC-5320A type 4,5.5,3.3,1.0,10.5,13.5,2,"Voltage class 10.5–13.5 V; plug has its own centre pin|Spannungsklasse 10,5–13,5 V; Stecker hat eigenen Mittelstift"
EIAJ-05 (JSBP5),JEITA RC-5320A type 5,6.5,4.4,1.4,13.5,18,2,"Voltage class 13.5–18 V; plug has its own centre pin; ID UNVERIFIED (4.4 vs 4.3 mm)|Spannungsklasse 13,5–18 V; Stecker hat eigenen Mittelstift; ID UNVERIFIED (4,4 bzw. 4,3 mm)"
5.5 × 2.1,common (IEC 60130-10 type A per Wikipedia),5.5,2.1,,,,0.5,"Very common (e.g. Arduino adapters, guitar effects pedals); current rating depends on the part (Lumberg NES/J 210: 0.5 A, 12 V)|Sehr verbreitet (z. B. Arduino-Netzteile, Gitarren-Effektgeräte); Strombelastbarkeit je nach Typ (Lumberg NES/J 210: 0,5 A, 12 V)"
5.5 × 2.5,common (IEC 60130-10 type A per Wikipedia),5.5,2.5,,,,0.5,"Looks like 5.5 × 2.1 but needs a 2.5 mm jack pin; a 2.1 plug does not fit a 2.5 jack pin, a 2.5 plug fits loosely on 2.1; Schurter 4840.5210: 0.5 A, 12 V|Sieht aus wie 5,5 × 2,1, braucht aber 2,5-mm-Stift; 2,1er-Stecker passt nicht auf 2,5er-Stift, 2,5er sitzt locker auf 2,1er; Schurter 4840.5210: 0,5 A, 12 V"
3.5 × 1.35,common,3.5,1.35,,,,0.5,"Small devices and chargers (e.g. Canon CA-570, 8.4 V); Lumberg sells the size as 3.4 × 1.4 mm (NES/J 135: 0.5 A, 12 V)|Kleingeräte und Ladegeräte (z. B. Canon CA-570, 8,4 V); Lumberg führt die Größe als 3,4 × 1,4 mm (NES/J 135: 0,5 A, 12 V)"
2.5 × 0.7,common,2.5,0.7,,,,,"Small/older portable devices (e.g. Canon ACK-800, old Motorola phones); single source, close to EIAJ-01 (2.35 × 0.7) — UNVERIFIED|Kleine/ältere Mobilgeräte (z. B. Canon ACK-800, alte Motorola-Handys); nur eine Quelle, nahe an EIAJ-01 (2,35 × 0,7) — UNVERIFIED"
```

## Sources
1. Lumberg, data sheets 16 1636 01 (08/2023, obsolete), 1636 02, 1636 03, 1636 04, 1636 05 (10/2025): "Power supply plug … acc. to JEITA (EIAJ) RC-5320A type 1 JSAP1 / type 2 JSAP2 / type 3 JSAP3 / type 4 JSBP4 / type 5 JSBP5", https://downloads.lumberg.com/datenblaetter/en/1636_01.pdf … /1636_05.pdf — outer/inner/centre-pin diameters, rated current 2 A, rated voltage 3.15 / 6.3 / 10.5 / 13.5 / 18 V DC; accessed 2026-11-02.
2. Wikipedia, "EIAJ connector", https://en.wikipedia.org/wiki/EIAJ_connector — RC-5320A table (min./rated voltage, 2 A, plug OD/ID, pin, shaft length); cites JEITA ET-2502A; accessed 2026-11-02.
3. GlobTek, "EIAJ RC 5320A Connectors", https://www.globtek.com/en/cordsets/EIAJ-RC-5320A-Connectors — same table as (2) (EIAJ-04 ID 3.4 mm); accessed 2026-11-02.
4. Wikipedia, "Coaxial power connector", https://en.wikipedia.org/wiki/Coaxial_power_connector — common sizes (2.5 × 0.7, 3.5 × 1.35, 4.0 × 1.7, 5.5 × 2.1, 5.5 × 2.5) with example uses, IEC 60130-10 type A note, polarity (centre-positive most common); its RC-5320A table differs (see cross-check); accessed 2026-11-02.
5. Lumberg, data sheet "NES/J 210 — Power supply plug, 5.5 mm x 2.1 mm" (10/2025), https://downloads.lumberg.com/datenblaetter/en/nes_j210.pdf — 5.5/2.1 mm, 0.5 A, 12 V DC; accessed 2026-11-02.
6. Lumberg, data sheet "NES/J 135 — Power supply plug, 3.4 mm x 1.4 mm" (10/2025), https://downloads.lumberg.com/datenblaetter/en/nes_j135.pdf — 3.4/1.4 mm, 0.5 A, 12 V DC; accessed 2026-11-02.
7. SCHURTER, "4840.5210 DC Connector, Plug, 5.5x2.5 mm, 2-pole, straight, with cable" (05.04.2013), copy https://komplekt-a.ru/image/catalog/pdf/typ_4840.5210.pdf — 5.5 mm / 2.5 mm, 0.5 A / 12 VDC; accessed 2026-11-02.
(The JEITA RC-5320A standard itself was not available.)

## Cross-check
- RC-5320A types 1–3: Lumberg (1) and Wikipedia "EIAJ connector" (2) agree: 2.35/0.7, 4.0/1.7, 4.75/1.7 mm; voltages 3.15/6.3/10.5 V; 2 A.
- Type 4: OD 5.5, pin 1.0 agree (1, 2, 3); ID 3.3 mm in Lumberg and Wikipedia EIAJ, 3.4 mm in GlobTek and Wikipedia "Coaxial power connector" → chosen 3.3 mm (maker drawing + one more source).
- Type 5: OD 6.5, pin 1.4 agree; ID 4.4 mm in Wikipedia EIAJ and GlobTek, 4.3 mm in Lumberg → UNVERIFIED (GlobTek's table looks copied from Wikipedia, so the two 4.4 sources may not be independent); 4.4 kept as listed value.
- Wikipedia "Coaxial power connector" (4) gives a divergent RC-5320A table (EIAJ-01 2.5 × 0.7, EIAJ-03 5.0 × 1.7, EIAJ-04 5.5 × 3.4); not used because the maker drawings (1) and the dedicated article (2) agree on 2.35 and 4.75 mm.
- 5.5 × 2.1: Wikipedia (4) + Lumberg NES/J 210 (5) agree. 5.5 × 2.5: Wikipedia + Schurter 4840.5210 (7) agree. 3.5 × 1.35: Wikipedia names it 3.5 × 1.35; Lumberg's plug for that size is specified 3.4 × 1.4 → nominal kept, Lumberg value in the use column. 2.5 × 0.7: Wikipedia only → UNVERIFIED.
- i_max_a for common sizes is that of the cited plugs only (0.5 A); higher-rated plugs exist (e.g. Same Sky PP3-002B "DC power plug" rated 7 A, 24 Vdc — its drawing dimensions were not readable, so it is not used as a dimension source).
- "IEC 60130-10 type A" is from Wikipedia only (single source).
- use texts for the common sizes come from Wikipedia's examples; the fit remark for 2.1 vs 2.5 is editorial (follows from the diameters).

## Notes (en / de)
- en: Barrel size does not define voltage, current or polarity except for the JEITA RC-5320A types, whose size encodes the voltage class so that a higher-voltage plug does not fit a lower-voltage jack. Centre-positive is the most common polarity, but check the symbol on the device. Inner diameter = diameter of the jack's centre pin.
- de: Die Hohlsteckergröße legt Spannung, Strom und Polarität nicht fest — außer bei den JEITA-RC-5320A-Typen, deren Größe die Spannungsklasse kodiert, damit ein Stecker höherer Spannung nicht in eine Buchse niedrigerer Spannung passt. Plus innen ist am verbreitetsten, das Symbol am Gerät aber immer prüfen. Innendurchmesser = Durchmesser des Mittelstifts der Buchse.

## Suggested finder / lookups
- Identify a plug from measured OD × ID (nearest row within ±0.15 mm).
- RC-5320A: pick the type whose [v_min, v_max] contains the supply voltage.
