# protection_classes — Protection classes against electric shock (IEC 61140) / Schutzklassen (DIN EN 61140, VDE 0140-1)

Kind: transcribed standard
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| class | protection class (0, I, II, III) | Schutzklasse | – | no |
| principle | how protection against electric shock is achieved "en\|de" | Schutzprinzip "en\|de" | – | no |
| pe | protective-earth connection: `yes` / `no` | Schutzleiteranschluss: `yes` / `no` | – | no |
| symbol | marking symbol "en\|de" (with IEC 60417 reference number) | Kennzeichnungssymbol "en\|de" (mit Nummer nach IEC 60417) | – | no |
| iec60417 | IEC 60417 graphical symbol number (empty = no symbol) | Nummer des Bildzeichens nach IEC 60417 (leer = kein Symbol) | – | no |
| remark | restrictions / remarks "en\|de" | Einschränkungen / Hinweise "en\|de" | – | no |

## Rows
```csv
class,principle,pe,symbol,iec60417,remark
0,"basic insulation only, no connection to the protective conductor; protection relies on the environment|nur Basisisolierung, kein Anschluss an den Schutzleiter; Schutz muss durch die Umgebung sichergestellt werden",no,no symbol (marking not provided for)|kein Symbol (Kennzeichnung nicht vorgesehen),,"not permitted in Germany; sale prohibited in many countries (e.g. UK, New Zealand)|in Deutschland nicht mehr zulässig; in vielen Ländern verboten (z. B. Vereinigtes Königreich, Neuseeland)"
I,"basic insulation plus connection of all accessible conductive parts to the protective earth conductor; a fault trips the fuse, circuit breaker or RCD|Basisisolierung und Anschluss aller berührbaren leitfähigen Teile an den Schutzleiter; ein Fehler löst Sicherung, Leitungsschutzschalter oder FI-Schutzschalter (RCD) aus",yes,protective earth symbol (circle with earth sign) on the PE terminal|Schutzleiter-Symbol (Erdungszeichen im Kreis) an der PE-Klemme,5019,the symbol marks the protective-conductor terminal; there is no separate class-I device symbol|das Symbol kennzeichnet die Schutzleiterklemme; ein eigenes Gerätesymbol für Klasse I gibt es nicht
II,double or reinforced insulation between live and accessible parts; no protective-earth connection|doppelte oder verstärkte Isolierung zwischen aktiven und berührbaren Teilen; kein Schutzleiteranschluss,no,two concentric squares|zwei ineinanderliegende Quadrate,5172,"a variant with functional earth (FE, for EMC only) exists, symbol IEC 60417-6092|Variante mit Funktionserde (FE, nur für EMV) möglich, Symbol IEC 60417-6092"
III,supply only from safety extra-low voltage (SELV) or protective extra-low voltage (PELV) sources|Betrieb nur mit Sicherheitskleinspannung (SELV) oder Schutzkleinspannung (PELV),no,roman numeral III in a diamond|römische III in einer Raute,5180,"connect only to SELV/PELV sources such as a safety transformer or battery|Anschluss nur an SELV-/PELV-Quellen, z. B. Sicherheitstransformator oder Batterie"
```

## Sources
1. en.wikipedia, "Appliance classes", https://en.wikipedia.org/wiki/Appliance_classes — classes 0, 0I, I, II, IIFE, III per IEC 61140; symbol numbers IEC 60417-5172 (II), -6092 (II with FE), -5180 (III); class 0 prohibited in e.g. UK and New Zealand; accessed 2026-11-02.
2. de.wikipedia, "Schutzklasse (Elektrotechnik)", https://de.wikipedia.org/wiki/Schutzklasse_(Elektrotechnik) — classes 0–III per DIN EN 61140 (VDE 0140-1):2016-11; symbol IEC 60417-5019 for the PE terminal (class I), -5172 (II), -5180 (III); class 0 "in Deutschland normativ nicht mehr zulässig"; class III with SELV or PELV; accessed 2026-11-02.
3. FRIWO Gerätebau, "IP-Schutzarten nach DIN 60529" (2016), https://www.friwo.com/wp-content/uploads/Downloads/IP_Schutzklassen_220216.pdf — distinguishes IP degrees from the protection classes I, II, III (context only); accessed 2026-11-02.

## Cross-check
- Sources 1 and 2 agree on the protection principle of every class, on symbol numbers 5172 (II) and 5180 (III), and on class III = SELV supply. Source 2 additionally names PELV for class III and 5019 for the class-I PE terminal; source 1 shows the earth symbol for class I without a number → 5019 is single-source (source 2) but is the well-known protective-earth symbol; mark "5019" as checked by one source only.
- The current edition is IEC 61140:2016 (DIN EN 61140 VDE 0140-1:2016-11 per source 2); I could not read the standard itself (paywalled).
- Class 0I (class 0 with an earth terminal) is mentioned only in source 1 and is left out.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Errichtungsbestimmungen.
- en: Protection classes describe protection against electric shock; IP codes describe protection against foreign objects and water — they are independent.
- de: Schutzklassen beschreiben den Schutz gegen elektrischen Schlag, IP-Schutzarten den Schutz gegen Fremdkörper und Wasser – beides ist unabhängig voneinander.

## Suggested finder / lookups
- Class → principle and symbol (static reference table, no calculation).
