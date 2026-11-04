# ip_codes — IP code (degrees of protection, IEC 60529) / IP-Schutzarten (DIN EN 60529)

Kind: transcribed standard
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| position | `first` = first characteristic numeral (solid foreign objects, access), `second` = second numeral (water) | `first` = erste Kennziffer (Fremdkörper, Berührung), `second` = zweite Kennziffer (Wasser) | – | no |
| code | the numeral (0–6, 0–9, X; "9K" is from ISO 20653, see notes) | Kennziffer | – | no |
| short | short description "en\|de" | Kurzbeschreibung "en\|de" | – | no |
| definition | what the degree means "en\|de" | Bedeutung des Schutzgrads "en\|de" | – | no |
| test_hint | test conditions as summarised by Wikipedia — **whole column UNVERIFIED (single source)**, for information only | Prüfbedingungen laut Wikipedia — **ganze Spalte UNVERIFIED (eine Quelle)** | as written | no |

## Rows
```csv
position,code,short,definition,test_hint
first,0,not protected|nicht geschützt,no special protection|kein besonderer Schutz,
first,1,protected against solid foreign objects of 50 mm Ø and greater|geschützt gegen feste Fremdkörper 50 mm Durchmesser und größer,"the object probe, sphere of 50 mm Ø, shall not fully penetrate (protection against access with the back of a hand)|die Objektsonde, Kugel 50 mm Durchmesser, darf nicht voll eindringen (Schutz gegen Zugang mit dem Handrücken)",
first,2,"protected against solid foreign objects of 12.5 mm Ø and greater|geschützt gegen feste Fremdkörper 12,5 mm Durchmesser und größer","the object probe, sphere of 12.5 mm Ø, shall not fully penetrate (protection against access with a finger)|die Objektsonde, Kugel 12,5 mm Durchmesser, darf nicht voll eindringen (Schutz gegen Zugang mit dem Finger)",
first,3,"protected against solid foreign objects of 2.5 mm Ø and greater|geschützt gegen feste Fremdkörper 2,5 mm Durchmesser und größer","the object probe, sphere of 2.5 mm Ø, shall not penetrate at all (tools, thick wires)|die Objektsonde, Kugel 2,5 mm Durchmesser, darf überhaupt nicht eindringen (Werkzeuge, dicke Drähte)",
first,4,"protected against solid foreign objects of 1.0 mm Ø and greater|geschützt gegen feste Fremdkörper 1,0 mm Durchmesser und größer","the object probe, sphere of 1.0 mm Ø, shall not penetrate at all (wires)|die Objektsonde, Kugel 1,0 mm Durchmesser, darf überhaupt nicht eindringen (Drähte)",
first,5,dust-protected|staubgeschützt,"ingress of dust is not totally prevented, but dust shall not penetrate in a quantity that interferes with satisfactory operation or safety|Eindringen von Staub ist nicht vollständig verhindert, aber Staub darf nicht in einer Menge eindringen, die das zufriedenstellende Arbeiten oder die Sicherheit beeinträchtigt",
first,6,dust-tight|staubdicht,no ingress of dust|kein Eindringen von Staub,dust chamber with vacuum; test up to 8 h depending on airflow
first,X,not specified|nicht angegeben,numeral not specified (not tested or not required)|Kennziffer nicht angegeben (nicht geprüft oder nicht gefordert),
second,0,not protected|nicht geschützt,no special protection|kein besonderer Schutz,
second,1,protected against vertically falling water drops|geschützt gegen Tropfwasser,vertically falling drops shall have no harmful effects|senkrecht fallende Tropfen dürfen keine schädlichen Wirkungen haben,"1 mm/min rainfall, 10 min, turntable 1 rpm"
second,2,"protected against vertically falling water drops when the enclosure is tilted up to 15°|geschützt gegen Tropfwasser, wenn das Gehäuse bis zu 15° geneigt ist","vertically falling drops shall have no harmful effects when the enclosure is tilted at any angle up to 15° on either side of the vertical|senkrecht fallende Tropfen dürfen keine schädlichen Wirkungen haben, wenn das Gehäuse um einen Winkel bis zu 15° beiderseits der Senkrechten geneigt ist","3 mm/min rainfall, 4 positions × 2.5 min (10 min)"
second,3,protected against spraying water|geschützt gegen Sprühwasser,"water sprayed at an angle up to 60° on either side of the vertical shall have no harmful effects|Wasser, das in einem Winkel bis zu 60° beiderseits der Senkrechten gesprüht wird, darf keine schädlichen Wirkungen haben","oscillating tube 10 min, or spray nozzle 10 L/min at 50–150 kPa, ≥ 5 min"
second,4,protected against splashing water|geschützt gegen Spritzwasser,"water splashed against the enclosure from any direction shall have no harmful effects|Wasser, das aus jeder Richtung gegen das Gehäuse spritzt, darf keine schädlichen Wirkungen haben","oscillating tube 10 min, or spray nozzle without shield ≥ 5 min"
second,5,protected against water jets|geschützt gegen Strahlwasser,"water projected in jets against the enclosure from any direction shall have no harmful effects|Wasser, das aus jeder Richtung als Strahl gegen das Gehäuse gerichtet ist, darf keine schädlichen Wirkungen haben","6.3 mm nozzle, 12.5 L/min, 30 kPa at 3 m, ≥ 3 min"
second,6,protected against powerful water jets|geschützt gegen starkes Strahlwasser,"water projected in powerful jets against the enclosure from any direction shall have no harmful effects|Wasser, das aus jeder Richtung als starker Strahl gegen das Gehäuse gerichtet ist, darf keine schädlichen Wirkungen haben","12.5 mm nozzle, 100 L/min, 100 kPa at 3 m, ≥ 3 min"
second,7,protected against the effects of temporary immersion in water|geschützt gegen die Wirkungen beim zeitweiligen Untertauchen in Wasser,"ingress of water in harmful quantity shall not be possible when the enclosure is temporarily immersed under standardised conditions of pressure and time|Wasser darf nicht in schädlicher Menge eintreten, wenn das Gehäuse unter genormten Druck- und Zeitbedingungen zeitweilig untergetaucht ist","30 min; lowest point 1 m below the surface or highest point 0.15 m below, whichever is deeper"
second,8,protected against the effects of continuous immersion in water|geschützt gegen die Wirkungen beim dauernden Untertauchen in Wasser,"ingress of water in harmful quantity shall not be possible when continuously immersed under conditions agreed between manufacturer and user, more severe than for numeral 7|Wasser darf nicht in schädlicher Menge eintreten, wenn das Gehäuse dauernd untergetaucht ist, unter zwischen Hersteller und Anwender vereinbarten Bedingungen, die schwieriger sein müssen als für Kennziffer 7","conditions by agreement, generally up to 3 m"
second,9,protected against high-pressure and high-temperature water jets|geschützt gegen Hochdruck und hohe Strahlwassertemperaturen,"water directed at high pressure and high temperature against the enclosure from any direction shall have no harmful effects|Wasser, das bei hohem Druck und hohen Temperaturen aus allen Richtungen gegen das Gehäuse gerichtet ist, darf keine schädlichen Wirkungen haben","80 °C water, 8–10 MPa, 14–16 L/min; turntable 4 angles × 30 s, or freehand ≥ 3 min at 0.15–0.2 m"
second,X,not specified|nicht angegeben,numeral not specified (not tested or not required)|Kennziffer nicht angegeben (nicht geprüft oder nicht gefordert),
second,9K,"high-pressure / steam-jet cleaning (ISO 20653, road vehicles; not IEC 60529)|Hochdruck-/Dampfstrahlreinigung (ISO 20653, Straßenfahrzeuge; nicht IEC 60529)","defined in ISO 20653 (formerly DIN 40050-9), not in IEC 60529|definiert in ISO 20653 (früher DIN 40050-9), nicht in IEC 60529",
```

## Sources
1. IEC, "Ingress Protection (IP) ratings", https://www.iec.ch/ip-ratings — confirms the structure (first numeral 0 "no protection" to 6 "no ingress of dust"; second numeral 0 to 9 "high-pressure hot water from different angles") and refers to IEC 60529 for full descriptions; read via the in-app browser (direct fetch is blocked), accessed 2026-11-02.
2. IEC Webstore, "IEC 60529:1989+AMD1:1999+AMD2:2013 CSV — Degrees of protection provided by enclosures (IP Code)", edition 2.2, published 2013-08-29, https://webstore.iec.ch/en/publication/2452 — identifies the current consolidated edition (not the text itself; paywalled); accessed 2026-11-02.
3. Häwa GmbH (enclosure maker), "Schutzarten nach DIN EN 60529" (PDF data sheet), https://www.haewa.de/fileadmin/Assets/de/Download-Center/Schutzart_de.pdf — German short description and definition for every numeral 0–6 and 0–9 (wording follows the standard's table); accessed 2026-11-02.
4. en.wikipedia, "IP code", https://en.wikipedia.org/wiki/IP_code — English descriptions of 0–6, 0–9, test details, and that 6K/9K are from DIN 40050-9 / ISO 20653, not IEC 60529; accessed 2026-11-02.
5. BGN (Berufsgenossenschaft Nahrungsmittel und Gastgewerbe), ASI 3.10 "Elektrischer Strom – Gefahren und Schutzmaßnahmen", section 4 "Schutzarten (IP-Code)", https://vorschriften.bgn-branchenwissen.de/daten/asi/a3_10_0/4.htm — German short descriptions 0–6, 0–8 and 9K; states IPX9K is only in ISO 20653, not EN 60529; accessed 2026-11-02.
6. FRIWO Gerätebau, "IP-Schutzarten nach DIN 60529" (2016), https://www.friwo.com/wp-content/uploads/Downloads/IP_Schutzklassen_220216.pdf — third maker table for 0–6 and 0–8 (probe diameters 50 / 12.5 / 2.5 / 1.0 mm, 15° and 60°); accessed 2026-11-02.

## Cross-check
- `short` and `definition`: sources 3 (Häwa, de) and 4 (Wikipedia, en) agree on every numeral 0–6 and 0–9 (probe diameters 50 / 12.5 / 2.5 / 1.0 mm; 15° tilt; 60° spray; "any direction"; temporary vs continuous immersion; high pressure + high temperature for 9). Source 6 agrees for 0–6 and 0–8; source 5 agrees for 0–6 and 0–8.
- Disagreement: source 5 (BGN, 2015) gives "> 12 mm" for first numeral 2; sources 3, 4, 6 give 12.5 mm. **Chosen 12.5 mm** (three sources, and the IEC wording "12.5 mm"); 12 mm is the older rounding.
- The English `definition` texts are my translation of the Häwa wording, checked for sense against Wikipedia; they are not a verbatim copy of IEC 60529 (which I could not read).
- `test_hint` is from Wikipedia only → UNVERIFIED (no second source with test conditions was reachable; the standard is paywalled).
- Numeral 9 exists in IEC 60529 since AMD2:2013 (DIN EN 60529:2014-09). 9K is ISO 20653 and is kept as an informational row.
- The additional letters (A, B, C, D: protection of persons against access with back of hand, finger, tool, wire) and supplementary letters (H high-voltage apparatus, M/S moving/stationary during water test, W weather conditions) exist in IEC 60529; only Wikipedia (source 4) lists H/M/S/W, so they are left out of the rows (UNVERIFIED).

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Errichtungsbestimmungen.
- en: The two numerals are independent: IPX7 does not imply IPX5/IPX6 (immersion is not a jet test). A product rated for both is marked e.g. "IPX6/IPX7".
- de: Die beiden Kennziffern sind unabhängig: IPX7 schließt IPX5/IPX6 nicht ein (Untertauchen ist keine Strahlwasserprüfung). Geräte mit beiden Prüfungen werden z. B. "IPX6/IPX7" gekennzeichnet.
- en: The IP code says nothing about protection against electric shock class (see protection classes, IEC 61140) or mechanical impact (IK code, IEC 62262).
- de: Der IP-Code sagt nichts über die Schutzklasse (IEC 61140) oder die Stoßfestigkeit (IK-Code, IEC 62262) aus.

## Suggested finder / lookups
- Decoder: input "IP54" → first row (first,5) + second row (second,4).
- Requirement helper: "needs at least dust-protected and splash-proof" → minimal code IP54.
