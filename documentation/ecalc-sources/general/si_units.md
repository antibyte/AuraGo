# si_units — SI base units and common derived units / SI-Basiseinheiten und gebräuchliche abgeleitete Einheiten

Kind: definitional
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| kind | `base` (SI Brochure Table 2), `special` (one of the 22 units with special names, Table 4), `derived` (example coherent derived unit, Tables 5/6) | `base` = Basiseinheit, `special` = abgeleitete Einheit mit besonderem Namen, `derived` = sonstige kohärente abgeleitete Einheit | – | no |
| quantity | quantity name "en\|de" | Größe "en\|de" | – | no |
| qty_symbol | typical quantity symbol (only where the brochure gives one; empty otherwise) | typisches Formelzeichen (nur wo die Broschüre eines nennt) | – | no |
| unit | unit name "en\|de" | Einheitenname "en\|de" | – | no |
| symbol | unit symbol | Einheitenzeichen | – | no |
| base_expr | the unit expressed in SI base units (order of symbols as in the 9th edition) | Einheit in SI-Basiseinheiten | – | no |
| other_expr | the unit expressed in other SI units (Table 4 last column; empty if none) | Einheit in anderen SI-Einheiten | – | no |

## Rows
```csv
kind,quantity,qty_symbol,unit,symbol,base_expr,other_expr
base,time|Zeit,t,second|Sekunde,s,s,
base,length|Länge,"l, x, r",metre|Meter,m,m,
base,mass|Masse,m,kilogram|Kilogramm,kg,kg,
base,electric current|elektrische Stromstärke,"I, i",ampere|Ampere,A,A,
base,thermodynamic temperature|thermodynamische Temperatur,T,kelvin|Kelvin,K,K,
base,amount of substance|Stoffmenge,n,mole|Mol,mol,mol,
base,luminous intensity|Lichtstärke,Iv,candela|Candela,cd,cd,
special,plane angle|ebener Winkel,,radian|Radiant,rad,m/m (see note),1
special,solid angle|Raumwinkel,,steradian|Steradiant,sr,m²/m² (see note),1
special,frequency|Frequenz,,hertz|Hertz,Hz,s⁻¹,
special,force|Kraft,,newton|Newton,N,kg·m·s⁻²,
special,"pressure, stress|Druck, mechanische Spannung",,pascal|Pascal,Pa,kg·m⁻¹·s⁻²,N/m²
special,"energy, work, amount of heat|Energie, Arbeit, Wärmemenge",,joule|Joule,J,kg·m²·s⁻²,N·m
special,"power, radiant flux|Leistung, Strahlungsleistung",,watt|Watt,W,kg·m²·s⁻³,J/s
special,electric charge|elektrische Ladung,,coulomb|Coulomb,C,A·s,
special,"electric potential difference (voltage)|elektrische Spannung, Potentialdifferenz",,volt|Volt,V,kg·m²·s⁻³·A⁻¹,W/A
special,capacitance|elektrische Kapazität,,farad|Farad,F,kg⁻¹·m⁻²·s⁴·A²,C/V
special,electric resistance|elektrischer Widerstand,,ohm|Ohm,Ω,kg·m²·s⁻³·A⁻²,V/A
special,electric conductance|elektrischer Leitwert,,siemens|Siemens,S,kg⁻¹·m⁻²·s³·A²,A/V
special,magnetic flux|magnetischer Fluss,,weber|Weber,Wb,kg·m²·s⁻²·A⁻¹,V·s
special,magnetic flux density|magnetische Flussdichte,,tesla|Tesla,T,kg·s⁻²·A⁻¹,Wb/m²
special,inductance|Induktivität,,henry|Henry,H,kg·m²·s⁻²·A⁻²,Wb/A
special,Celsius temperature|Celsius-Temperatur,,degree Celsius|Grad Celsius,°C,K,
special,luminous flux|Lichtstrom,,lumen|Lumen,lm,cd·sr,cd·sr
special,illuminance|Beleuchtungsstärke,,lux|Lux,lx,cd·sr·m⁻²,lm/m²
special,activity referred to a radionuclide|Aktivität einer radioaktiven Substanz,,becquerel|Becquerel,Bq,s⁻¹,
special,"absorbed dose, kerma|Energiedosis, Kerma",,gray|Gray,Gy,m²·s⁻²,J/kg
special,dose equivalent|Äquivalentdosis,,sievert|Sievert,Sv,m²·s⁻²,J/kg
special,catalytic activity|katalytische Aktivität,,katal|Katal,kat,mol·s⁻¹,
derived,area|Fläche,A,square metre|Quadratmeter,m²,m²,
derived,volume|Volumen,V,cubic metre|Kubikmeter,m³,m³,
derived,"speed, velocity|Geschwindigkeit",v,metre per second|Meter durch Sekunde,m/s,m·s⁻¹,
derived,acceleration|Beschleunigung,a,metre per second squared|Meter durch Sekunde zum Quadrat,m/s²,m·s⁻²,
derived,current density|elektrische Stromdichte,j,ampere per square metre|Ampere durch Quadratmeter,A/m²,A·m⁻²,
derived,magnetic field strength|magnetische Feldstärke,H,ampere per metre|Ampere durch Meter,A/m,A·m⁻¹,
derived,"moment of force, torque|Drehmoment",,newton metre|Newtonmeter,N·m,kg·m²·s⁻²,
derived,"angular velocity, angular frequency|Winkelgeschwindigkeit, Kreisfrequenz",,radian per second|Radiant durch Sekunde,rad/s,s⁻¹,
derived,"heat flux density, irradiance|Wärmestromdichte, Bestrahlungsstärke",,watt per square metre|Watt durch Quadratmeter,W/m²,kg·s⁻³,
derived,"heat capacity, entropy|Wärmekapazität, Entropie",,joule per kelvin|Joule durch Kelvin,J/K,kg·m²·s⁻²·K⁻¹,
derived,thermal conductivity|Wärmeleitfähigkeit,,watt per metre kelvin|Watt durch Meter und Kelvin,W/(m·K),kg·m·s⁻³·K⁻¹,
derived,energy density|Energiedichte,,joule per cubic metre|Joule durch Kubikmeter,J/m³,kg·m⁻¹·s⁻²,
derived,electric field strength|elektrische Feldstärke,,volt per metre|Volt durch Meter,V/m,kg·m·s⁻³·A⁻¹,
derived,electric charge density|Raumladungsdichte,,coulomb per cubic metre|Coulomb durch Kubikmeter,C/m³,A·s·m⁻³,
derived,surface charge density|Flächenladungsdichte,,coulomb per square metre|Coulomb durch Quadratmeter,C/m²,A·s·m⁻²,
derived,"electric flux density, electric displacement|elektrische Flussdichte, Verschiebungsdichte",,coulomb per square metre|Coulomb durch Quadratmeter,C/m²,A·s·m⁻²,
derived,permittivity|Permittivität,,farad per metre|Farad durch Meter,F/m,kg⁻¹·m⁻³·s⁴·A²,
derived,permeability|Permeabilität,,henry per metre|Henry durch Meter,H/m,kg·m·s⁻²·A⁻²,
```

## Sources
1. BIPM, *The International System of Units (SI)*, 9th edition 2019, version V4.01 (June 2026), Table 2 (base units), Table 4 "The 22 SI units with special names and symbols" incl. footnotes (a)–(i), Table 5 and Table 6 (examples of coherent derived units), https://www.bipm.org/documents/20126/41483022/SI-Brochure-9-EN.pdf — all English names, symbols and base-unit expressions; accessed 2026-11-02.
2. Einheitenverordnung (EinhV), Anlage 1 (alphabetical list of legal units with German unit names and German quantity names), https://www.gesetze-im-internet.de/einhv/anlage_1.html — German unit names and German quantity names for the base and special units; accessed 2026-11-02.
3. de.wikipedia, "Liste physikalischer Größen", https://de.wikipedia.org/wiki/Liste_physikalischer_Größen — German quantity names for some `derived` rows (Permittivität, Permeabilität, Flächenladungsdichte …); accessed 2026-11-02.

## Cross-check
Definitional (single authoritative source, BIPM). The Table 4 text was extracted twice (layout and raw PDF text order) because the layout extraction jumbled the right-hand column; the raw extraction gives the clean row order used here.
German names: base and special units follow EinhV Anlage 1 (which still uses the old spelling "Flußdichte"/"Fluß" — modernised here to "Flussdichte"/"Fluss"). The German names of the `derived` rows (e.g. "Meter durch Sekunde", "Wärmestromdichte", "Raumladungsdichte") are standard German usage written by me, not transcribed from a German standard — the implementer may want a German reviewer to confirm them.

## Notes (en / de)
- en: The radian and steradian are the coherent units for plane and solid angle; the brochure warns that "rad = m/m" is "not intrinsic and may be misleading" — they are dimensionless (1) but keep their names for clarity. The hertz is only for periodic phenomena, the becquerel only for stochastic processes (radioactive decay).
- de: Radiant und Steradiant sind die kohärenten Einheiten für ebenen und räumlichen Winkel; sie sind dimensionslos (1), werden aber zur Klarheit benannt. Hertz nur für periodische Vorgänge, Becquerel nur für radioaktiven Zerfall.
- en: "Electric potential difference" is also called "voltage" (footnote e). A temperature difference has the same numerical value in K and °C (footnote f).
- de: Die elektrische Potentialdifferenz heißt auch "Spannung". Eine Temperaturdifferenz hat in K und °C denselben Zahlenwert.
- en: The order of base-unit symbols follows the 9th edition (e.g. newton = kg m s⁻²), which differs from the 8th edition.
- de: Die Reihenfolge der Basiseinheiten folgt der 9. Auflage (z. B. Newton = kg m s⁻²).

## Suggested finder / lookups
- Symbol → base-unit expression (e.g. "F" → kg⁻¹·m⁻²·s⁴·A²); useful for a "dimension check" helper.
- Quantity search by name in en or de.
