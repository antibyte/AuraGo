# schematic_symbols — Schematic symbols, IEC vs ANSI / Schaltzeichen, IEC und ANSI im Vergleich

Kind: transcribed standard (graphical symbols) + secondary references for the US side
Guide-value note needed: yes (shipped as `guide: true`: the IEC side is the 1978-1983 edition via IS 12032, the ANSI side is read through NEETS)

Purpose: a reference table of the 25 symbols in ECalc's own symbol library (`ui/js/desktop/apps/ecalc/ecalc-symbols.js`),
each shown in its IEC drawing and its ANSI drawing side by side, with name, what the part does, and where the two
standards differ. The record also lists which of ECalc's current drawings are wrong or miss a difference.

What was read, in short:
- **IEC side — primary.** The IEC 60617 database (library.iec.ch) could not be opened: every page answers with an AWS
  bot challenge (not bypassed). The 1978–1983 IEC parts were read instead in their **identical Indian adoptions**
  IS 12032 (Parts 2, 4, 5, 6, 7, 8, 13), published by Public.Resource.Org on archive.org (page scans viewed). Their
  symbol numbers (e.g. 04-01-01) are the numbers the IEC 60617 database still cites as "earlier published in IEC
  60617-x". Current database S-numbers (e.g. S00555) were **not** read.
- **ANSI side — no primary.** IEEE Std 315-1975 / ANSI Y32.2-1975 is not legitimately available free of charge (the
  copies found online carry an IHS "not for resale" licence stamp; not used). Read instead: (1) US Navy NEETS
  Module 3, Appendix III, which reproduces pages laid out as IEEE 315 (section titles, the "IEC" marker on symbols
  that agree with IEC, cross-reference to ANSI C37.2-1970) — public release; (2) NEETS Modules 7 and 8 (figures);
  (3) Wikimedia Commons redrawings named after IEEE 315 item numbers with the item text copied into the file
  description (secondary); (4) English/German Wikipedia captions (secondary). IEEE 315 clause numbers below are
  **as named on Commons**, not seen in the standard.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id = ECalc symbol key | Zeilen-Id = ECalc-Symbolschlüssel | – | no |
| name | name "en\|de" | Bezeichnung "en\|de" | – | no |
| function | what the part does, one line "en\|de" | Funktion in einer Zeile "en\|de" | – | no |
| iec_ref | IEC symbol number as read (IEC 617 part + number, via IS 12032) | IEC-Symbolnummer wie gelesen | – | no |
| ieee_ref | IEEE 315 item / page as reported by the secondary source | IEEE-315-Nummer/Seite laut Sekundärquelle | – | no |
| differs | yes / no / style (same symbol allowed in both, typical drawings differ) | ja / nein / Stil | – | no |
| difference | where the standards differ "en\|de" ("same in both" when they agree) | Unterschied "en\|de" | – | no |
| ecalc | state of ECalc's current drawings: ok / change IEC / change ANSI / optional | Zustand der ECalc-Zeichnungen | – | no |
| check | cross-check record | Quellenabgleich | – | no |

## Rows
```csv
id,name,function,iec_ref,ieee_ref,differs,difference,ecalc,check
resistor,"Resistor|Widerstand","Limits current; U = R·I|Begrenzt den Strom; U = R·I","IEC 617-4 04-01-01 (preferred: rectangle), 04-01-02 (other form: zigzag)","2.1.1.a (zigzag), 2.1.1.b (rectangle with identification inside)",yes,"IEC: rectangle. ANSI: zigzag (IEEE 315 also has a rectangle form that must carry an identification)|IEC: Rechteck. ANSI: Zickzacklinie (IEEE 315 kennt auch ein Rechteck, das dann eine Kennzeichnung tragen muss)",ok,"IS 12032-4 p. 3 = Wikipedia IEC/ANSI resistor images = Commons IEEE 2.1.1"
capacitor,"Capacitor|Kondensator","Stores charge; blocks DC, passes AC|Speichert Ladung; sperrt Gleichstrom, lässt Wechselstrom durch","IEC 617-4 04-02-01 (preferred: two straight plates), 04-02-02 (other form: one plate curved)","Fundamental Items: capacitor general style 1 (two straight plates), style 2 (one plate curved)",no,"Same in both: two parallel plates. Both also allow one curved plate (outer or low-potential electrode)|In beiden gleich: zwei parallele Platten. Beide erlauben auch eine gebogene Platte (Außen- bzw. Niederpotenzial-Elektrode)",ok,"IS 12032-4 p. 4 = Commons IEEE FI 32/33 = Wikipedia caption"
polarized,"Polarised (electrolytic) capacitor|Gepolter Kondensator (Elko)","Large capacitance; must be connected with the right polarity|Große Kapazität; muss richtig gepolt angeschlossen werden","IEC 617-4 04-02-05 (preferred: two straight plates + '+'), 04-02-06 (other form: negative plate curved + '+')","Fundamental Items: polarized capacitor style 1 (straight + '+'), style 2 (curved + '+')",style,"IEC: two straight plates, '+' at the positive plate. ANSI: usually the negative plate curved, '+' at the straight plate (IEEE 315 also allows the IEC form)|IEC: zwei gerade Platten, „+“ an der Plusplatte. ANSI: meist Minusplatte gebogen, „+“ an der geraden Platte (IEEE 315 erlaubt auch die IEC-Form)","change IEC, change ANSI","IS 12032-4 p. 4 = Wikipedia 'Polarized capacitor (IEC 60617 style)' / '(old alternate IEEE 315 style)' = Commons FI 35/36"
inductor,"Inductor (coil)|Spule (Induktivität)","Stores energy in a magnetic field; opposes changes of current|Speichert Energie im Magnetfeld; wirkt Stromänderungen entgegen","IEC 617-4 04-03-01 (preferred: half-circles), 04-03-02 (other form: filled rectangle); core line 04-03-03","not seen in IEEE 315 sources read",no,"Same in both: a row of half-circles (US drawings also use loops). IEC also allows a filled rectangle|In beiden gleich: Reihe von Halbkreisen (US-Zeichnungen auch mit Schleifen). IEC erlaubt zusätzlich ein gefülltes Rechteck",ok,"IS 12032-4 = Wikipedia 'IEC inductor' / 'series of loops'; IEEE item not seen"
diode,"Diode|Diode","Conducts in one direction only (anode → cathode)|Leitet nur in einer Richtung (Anode → Katode)","IEC 617-5 05-03-01","8.5.1",no,"Same in both: triangle pointing to a bar (cathode). US drawings often fill the triangle|In beiden gleich: Dreieck mit Strich (Katode). US-Zeichnungen füllen das Dreieck oft",ok,"IS 12032-5 p. 3 = Commons IEEE 8.5.1 = NEETS M7"
led,"Light-emitting diode (LED)|Leuchtdiode (LED)","Diode that emits light when forward-biased|Diode, die in Durchlassrichtung Licht abgibt","IEC 617-5 05-03-02","not numbered in sources read",no,"Same in both: diode with two arrows pointing away|In beiden gleich: Diode mit zwei nach außen zeigenden Pfeilen",ok,"IS 12032-5 p. 3 = NEETS M7 (text with Fig. 3-27) = Wikipedia LED symbol"
zener,"Zener diode|Z-Diode","Conducts in reverse above its breakdown voltage; voltage reference/regulator|Leitet in Sperrrichtung ab der Durchbruchspannung; Spannungsreferenz/-begrenzer","IEC 617-5 05-03-06 (cathode bar with one short stub, qualifier 05-02-03)","8.5.6.1 (Commons: bar bent at both ends); 8.3.1 breakdown indicator",yes,"IEC: cathode bar with one short stub at one end, bent back towards the anode. ANSI: bar bent at both ends in opposite directions (Z shape); US documents also show other variants|IEC: Katodenstrich mit einem kurzen Haken an einem Ende, zur Anode hin abgewinkelt. ANSI: Strich an beiden Enden gegensinnig abgewinkelt (Z-Form); US-Unterlagen zeigen auch andere Varianten",change IEC,"IEC: IS 12032-5 p. 4 = de.wikipedia Z-Diode; ANSI: Commons IEEE 8.5.6.1.c = Wikipedia 'Zener diode'; NEETS M7 Fig. 3-4 shows five variants"
crystal,"Quartz crystal|Schwingquarz","Resonator with a very stable frequency|Resonator mit sehr stabiler Frequenz","IEC 617-4 04-07-01","Fundamental Items: piezoelectric crystal unit",no,"Same in both: rectangle between two electrode plates|In beiden gleich: Rechteck zwischen zwei Elektrodenplatten",ok,"IS 12032-4 p. 9 = Commons IEEE FI 113"
fuse,"Fuse|Sicherung","Melts and opens the circuit on overcurrent|Schmilzt bei Überstrom und trennt den Stromkreis","IEC 617-7 07-21-01 (rectangle, line through it lengthwise)","Circuit Protectors, 'Fuse … General': IEC form OR cartridge form OR S-shaped curve (NEETS M3 p. AIII-8)",style,"IEC: rectangle with a line through it. ANSI: the same symbol is allowed; typical US drawings use an S-shaped curve|IEC: Rechteck mit durchgehender Linie. ANSI: dasselbe Zeichen ist erlaubt; US-Zeichnungen verwenden meist eine S-förmige Linie",optional ANSI,"IS 12032-7 p. 31 = NEETS M3 p. AIII-8 (IEC-marked form) = de.wikipedia; S-curve: NEETS M3 Fig. 2-1 and p. AIII-8 = Commons 'Fuces.svg' description"
switch,"Switch (make contact)|Schalter (Schließer)","Opens or closes a circuit|Öffnet oder schließt einen Stromkreis","IEC 617-7 07-02-01 (form 1, no circles), 07-02-02 (form 2, hinge circle); note I.2","Contacts, Switches: 'Single-throw, general' marked IEC (NEETS M3 p. AIII-3); fixed contact may be an open circle (p. AIII-2)",style,"Same symbol in both (inclined blade). IEC draws no circles (a hinge circle is allowed); US drawings often show open circles at both contacts|Gleiches Zeichen (schräger Schaltarm). IEC ohne Kreise (Drehpunktkreis erlaubt); US-Zeichnungen zeigen oft offene Kreise an beiden Kontakten",optional IEC,"IS 12032-7 pp. 3, 5 = NEETS M3 p. AIII-3 (IEC marker); circles: NEETS p. AIII-2 = Wikipedia SPST"
pot,"Potentiometer|Potentiometer","Adjustable voltage divider; the wiper taps the resistance|Einstellbarer Spannungsteiler; der Schleifer greift den Widerstand ab","IEC 617-4 04-01-07 (rectangle, wiper arrow)","2.1.3 (adjustable contact; Commons shows 2.1.3.1.b), Wikipedia ANSI pot",yes,"IEC: rectangle with the wiper arrow. ANSI: zigzag with the wiper arrow|IEC: Rechteck mit Schleiferpfeil. ANSI: Zickzacklinie mit Schleiferpfeil",ok,"IS 12032-4 p. 3 = Wikipedia IEC/ANSI resistor images"
source,"DC voltage source|Gleichspannungsquelle","Ideal source of constant voltage|Ideale Quelle konstanter Spannung","IEC 617-2 02-16-02 ideal voltage source (circle, line through it along the conductor)","IEEE 315 'Battery – generalized direct-current source' (Commons FI 91); circle with + and − not seen in IEEE 315",yes,"IEC: circle crossed by the conductor (line along the leads); polarity added outside. US textbooks: circle with + and − inside|IEC: Kreis, durch den die Leitung hindurchgeht (Linie in Leitungsrichtung); Polarität außen angeben. US-Lehrbücher: Kreis mit + und − innen",change IEC,"IEC: IS 12032-2 p. 21 = de.wikipedia Spannungsquelle (DIN EN 60617-2); US form: en.wikipedia 'DC voltage source' only (UNVERIFIED as a standard symbol)"
acsource,"AC voltage source|Wechselspannungsquelle","Source of sinusoidal alternating voltage|Quelle sinusförmiger Wechselspannung","not found in the IEC parts read","Fundamental Items: 'Oscillator. Generalized alternating-current source' (circle with sine)",no,"Same in both: circle with a sine wave (IEC form not read)|In beiden gleich: Kreis mit Sinuslinie (IEC-Form nicht gelesen)",ok,"Commons IEEE FI 105 = en.wikipedia 'AC voltage source'; IEC UNVERIFIED"
isource,"Current source|Stromquelle","Ideal source of constant current|Ideale Quelle konstanten Stroms","IEC 617-2 02-16-01 ideal current source (circle, line across the conductor)","not seen in IEEE 315 sources; US form per de.wikipedia 'US-amerikanisch'",yes,"IEC: circle with a line across, perpendicular to the leads. US: circle with an arrow inside in the current direction|IEC: Kreis mit Querstrich senkrecht zu den Anschlüssen. US: Kreis mit Pfeil in Stromrichtung",change IEC,"IEC: IS 12032-2 p. 21 = de.wikipedia Stromquelle (IEC 60617-2:1996); US: de.wikipedia 'US-amerikanisch' = en.wikipedia 'Current source'"
battery,"Battery|Batterie","Chemical DC source; the long line is the positive pole|Chemische Gleichspannungsquelle; der lange Strich ist der Pluspol","IEC 617-6 06-15-01 (cell), 06-15-02 (battery, form 1)","Fundamental Items: battery one cell / multicell, 'the long line is always positive'",no,"Same in both: long thin line = +, short line = − (IEC: the short line may be thicker)|In beiden gleich: langer Strich = +, kurzer Strich = − (IEC: kurzer Strich darf dicker sein)",ok,"IS 12032-6 p. 18 = Commons IEEE FI 90/92"
meter,"Meter (voltmeter, ammeter)|Messgerät (Spannungs-, Strommesser)","Indicates a measured quantity; the letter names it (V, A, Ω …)|Zeigt eine Messgröße an; der Buchstabe nennt sie (V, A, Ω …)","IEC 617-8 08-01-01 (indicating instrument), 08-02-01 (voltmeter)","Readout Devices: 'Meter, Instrument' with letter (V voltmeter, A ammeter, both marked IEC; NEETS M3 p. AIII-10)",no,"Same in both: circle with the unit letter|In beiden gleich: Kreis mit dem Einheitenbuchstaben",ok,"IS 12032-8 pp. 4–5 = NEETS M3 p. AIII-10"
ground,"Ground (earth)|Masse (Erde)","Common reference potential, 0 V|Gemeinsames Bezugspotenzial, 0 V","IEC 617-2 02-15-01 earth, general symbol","Transmission Path: 'Circuit return – ground, general'",no,"Same in both: stem with three lines getting shorter|In beiden gleich: Stiel mit drei kürzer werdenden Strichen",ok,"IS 12032-2 p. 20 = Commons IEEE TP 75 (Wikipedia: 'IEC-style')"
antenna,"Antenna|Antenne","Radiates or receives radio waves|Strahlt Funkwellen ab oder empfängt sie","IS 2032-16:1972 clause 2.1 (based on IEC 117-10:1968): trident; current IEC form not read","Fundamental Items: 'Antenna – general' (V on a stem; also triangle on a stem)",no,"Same in ECalc: a V on a stem (older IEC drawings show a trident)|In ECalc gleich: V auf einem Stiel (ältere IEC-Zeichnungen zeigen einen Dreizack)",ok,"Commons IEEE FI 55 = Wikipedia 'General antenna (IEC-style)'; IS 2032-16 differs → UNVERIFIED"
npn,"NPN transistor|NPN-Transistor","Bipolar transistor; a small base current controls the collector current|Bipolartransistor; ein kleiner Basisstrom steuert den Kollektorstrom","IEC 617-5 05-05-02 (NPN, shown with envelope), base symbol 05-05-01 without circle","8.6.2 (with envelope circle)",style,"Same symbol: emitter arrow points outward. US drawings usually add the envelope circle (optional in ANSI Y32.2 A4.11); IEC usually without|Gleiches Zeichen: Emitterpfeil zeigt nach außen. US-Zeichnungen meist mit Gehäusekreis (in ANSI Y32.2 A4.11 optional); IEC meist ohne",ok (optional circle),"IS 12032-5 p. 5 = Commons IEEE 8.6.2; envelope rule: en.wikipedia citing ANSI Y32.2 A4.11"
pnp,"PNP transistor|PNP-Transistor","Bipolar transistor with reversed polarities; emitter arrow points in|Bipolartransistor mit umgekehrter Polarität; Emitterpfeil zeigt hinein","IEC 617-5 05-05-01","8.6.1 (with envelope circle)",style,"Same symbol: emitter arrow points towards the base. Envelope circle as for NPN|Gleiches Zeichen: Emitterpfeil zeigt zur Basis. Gehäusekreis wie beim NPN",ok (optional circle),"IS 12032-5 p. 5 = Commons IEEE 8.6.1"
nmos,"N-channel MOSFET (enhancement)|N-Kanal-MOSFET (Anreicherungstyp)","Voltage-controlled switch/amplifier; conducts when the gate is positive|Spannungsgesteuerter Schalter/Verstärker; leitet bei positiver Gatespannung","IEC 617-5 05-05-12 / 05-05-14 (broken channel, substrate arrow inward, substrate tied to source)","8.6.10.5 (N-channel enhancement, with envelope)",no,"Same in both: broken channel line, substrate arrow pointing to the channel|In beiden gleich: unterbrochene Kanallinie, Substratpfeil zeigt zum Kanal",ok,"IS 12032-5 pp. 6–7 = Commons IEEE 8.6.10.5"
pmos,"P-channel MOSFET (enhancement)|P-Kanal-MOSFET (Anreicherungstyp)","As the N-channel type with reversed polarities; conducts when the gate is negative|Wie der N-Kanal-Typ mit umgekehrter Polarität; leitet bei negativer Gatespannung","IEC 617-5 05-05-11 / 05-05-13 (substrate arrow outward)","8.6.11.5 (P-channel enhancement, with envelope)",no,"Same in both: as N-channel, substrate arrow pointing away from the channel|In beiden gleich: wie N-Kanal, Substratpfeil zeigt vom Kanal weg",ok,"IS 12032-5 p. 6 = Commons IEEE 8.6.11.5"
opamp,"Operational amplifier|Operationsverstärker","Very high-gain differential amplifier; the circuit around it sets the function|Differenzverstärker mit sehr hoher Verstärkung; die Beschaltung bestimmt die Funktion","IEC 617-13 13-04-02 (rectangle with ▷∞, inputs − and +)","not seen in IEEE 315 sources; NEETS M8 Fig. 3-10 (triangle)",yes,"IEC: rectangle with the amplifier sign ▷ and ∞ inside, inputs marked − and +. ANSI/US: triangle pointing to the output|IEC: Rechteck mit Verstärkerzeichen ▷ und ∞, Eingänge mit − und + bezeichnet. ANSI/US: Dreieck mit der Spitze zum Ausgang",change IEC,"IEC: IS 12032-13 p. 4 = de.wikipedia ('Schaltsymbol nach DIN EN 60617 Teil 13'); triangle: NEETS M8 Fig. 3-10 = en.wikipedia"
transformer,"Transformer|Transformator","Transfers AC energy between windings; turns ratio sets the voltage ratio|Überträgt Wechselstromenergie zwischen Wicklungen; das Windungsverhältnis bestimmt das Spannungsverhältnis","IEC 617-6 06-09-02 (form 2: windings as half-circles; core per 04-03-03), 06-09-01 (form 1: two circles)","not numbered in sources read",no,"Same in both: two windings with core lines between. IEC also allows two overlapping circles (single-line diagrams)|In beiden gleich: zwei Wicklungen mit Kernstrichen dazwischen. IEC erlaubt zusätzlich zwei überlappende Kreise (einpolige Darstellung)",ok,"IS 12032-6 p. 10 = Wikipedia 'Transformer'"
ic,"Integrated circuit (generic box)|Integrierter Schaltkreis (allgemeiner Block)","Any IC drawn as a box with numbered/named pins|Beliebiger IC als Kasten mit nummerierten/benannten Anschlüssen","not a standard component symbol (IEC 60617-12 qualifiers not used)","–",no,"Same in both: a rectangle with pin stubs and names (drawing convention, not a standardised symbol)|In beiden gleich: Rechteck mit Anschlussstrichen und Namen (Zeichenkonvention, kein genormtes Schaltzeichen)",ok,"no source needed; ECalc convention"
```

## Per symbol: what ECalc draws now (read from the SVG path data) and the verdict
Grid: 1 unit = 20 px; two-terminal parts run from pin a (0,0) to pin b (60,0); a = positive side / anode.

- **resistor** — IEC: leads 0–10 and 50–60, rectangle x 10–50, y −7…7. ANSI: zigzag `M0 0H10L14 -8 … L50 0H60`.
  Matches IEC 04-01-01 / IEEE 2.1.1.a. OK. (IEC 617-4:1983 still listed the zigzag as "other form" 04-01-02; whether
  the current database keeps it was not read.)
- **capacitor** — both: plates at x 27 and 33 (±12), leads to the plates. Matches IEC 04-02-01 / IEEE style 1. OK.
- **polarized** — IEC: straight plate at x 27, negative plate drawn as a **hollow rectangle** x 33–37 (±12), "+" at
  (19,−9). **Not an IEC drawing**: IEC 04-02-05 has two straight plates with "+"; Wikipedia files the hollow-rectangle
  plate under "Obsolete capacitor". ANSI: plate at 27 and the curve `M33 -12Q41 0 33 12`, whose middle lies at x≈37,
  i.e. it bulges **away** from the straight plate ("`)`"). IEEE style 2 (Commons FI 36) and IEC 04-02-06 both draw the
  curved plate bulging **towards** the straight plate ("`(`"), with the lead attached at its apex. **Both drawings
  change** (see list below).
- **inductor** — both: four half-circles of radius 5 from x 10 to 50. Matches IEC 04-03-01 (preferred). OK.
- **diode** — both: open triangle 22–38, bar at 38 (±10), leads stop at the triangle. IEC 05-03-01 draws the
  conductor through the triangle; IEEE 8.5.1 (Commons) fills the triangle. Cosmetic only; OK.
- **led** — diode + two arrows outward from near the bar. Matches IEC 05-03-02. OK.
- **zener** — both: triangle + bar `M42 -14L38 -10V10L34 14`, i.e. bent at **both** ends (top towards b, bottom
  towards a). Matches the ANSI Z form (IEEE 8.5.6.1.c on Commons has the same orientation, with square stubs).
  **IEC is wrong**: IEC 05-03-06 has one stub only, at one end, pointing back towards the anode.
- **crystal** — plates at 22 and 38 (±12), rectangle 26–34 (±8). Matches IEC 04-07-01 / IEEE crystal unit. OK.
- **fuse** — both: rectangle 10–50 (±7) with the line 10–50 through it. Matches IEC 07-21-01; the IEEE 315 page
  reproduced in NEETS (p. AIII-8) marks this same form "IEC" as one of three accepted forms. OK; optional ANSI
  S-curve.
- **switch** — both: leads 0–15 and 45–60, blade (15,0)→(42,−15), open circles r 2.5 at 15 and 45. IEC 07-02-01 has
  no circles (a hinge circle at the pivot is allowed, 07-02-02 / note I.2); IEEE allows open-circle fixed contacts.
  OK for ANSI; optional for IEC (drop the circle at the fixed contact b, keep or drop the hinge circle).
- **pot** — IEC: rectangle 10–70 + wiper from (40,−40) to the arrow tip (40,−8). ANSI: zigzag + same wiper. Matches
  IEC 04-01-07 / ANSI zigzag pot. OK.
- **source** — both: circle r 14 at (30,0), leads 0–16 and 44–60, "+" at (22,0) and "−" at (38,0) inside, kept
  upright. ANSI-style only. **IEC is wrong**: IEC 02-16-02 is a circle with the conductor drawn straight through it.
- **acsource** — circle + sine `M21 0Q25.5 -9 30 0T39 0`. Matches IEEE "generalized AC source"; IEC form not read. OK.
- **isource** — circle + arrow inside (21→40, head at 40). ANSI/US style only. **IEC is wrong**: IEC 02-16-01 is a
  circle with a line across it, perpendicular to the leads.
- **battery** — long lines at 20 and 34 (±12), short at 26 and 40 (±7), pin a on a long line = "+". Matches IEC
  06-15-02 form 1 and IEEE "long line positive". OK. (IEC note: the short line may be thickened.)
- **meter** — circle + letter (default V). Matches IEC 08-02-01 / IEEE meter. OK.
- **ground** — stem 0–10 and lines ±12, ±8, ±4. Matches IEC 02-15-01 / IEEE ground. OK.
- **antenna** — stem 0 to −26, V to (±13,−44). Matches IEEE "antenna – general" (Commons FI 55); see UNVERIFIED.
- **npn / pnp** — no envelope circle, emitter arrow outward (npn) / towards the base (pnp). Matches IEC 05-05-01/02
  and IEEE 8.6.1/8.6.2 apart from the optional envelope. OK.
- **nmos / pmos** — gate line, broken channel (three segments), substrate tied to source, arrow towards the channel
  (nmos) / away (pmos). Matches IEC 05-05-14 / 05-05-11…13 and IEEE 8.6.10.5 / 8.6.11.5. OK.
- **opamp** — both: triangle (20,−40)(20,40)(100,0), inputs at (0,∓20), "−" at y −20, "+" at y 20. Correct for ANSI.
  **IEC is wrong**: IEC 13-04-02 is a rectangle with ▷ and ∞.
- **transformer** — two coils of four half-circles (r 7.5) at x 22 and 58, core lines at 36 and 44. Matches IEC
  06-09-02 form 2 with core. OK.
- **ic** — box with stubs and labels. Not a standard symbol. OK.

## Drawings to change (same 20-px grid; pins stay where they are)
Items 1 to 5 were applied to `ecalc-symbols.js` (commit `fix(ecalc): IEC and ANSI drawings of polarised capacitor, zener and sources`). Item 6 (opamp, IEC) and the optional items 7 were not applied: the decision to switch all op-amp schematics is left to the user, so ECalc shows the triangle in both modes.
1. **polarized, IEC** — replace the hollow rectangle by a second straight plate:
   `path('M0 0H27M33 0H60') + path('M27 -12V12M33 -12V12') + plus(19, -9)`
   (= the capacitor plus the "+" at the positive plate; IEC 617-4 04-02-05).
2. **polarized, ANSI** — turn the curve round so that it bulges towards the straight plate and the lead meets its apex:
   `path('M0 0H27M33 0H60') + path('M27 -12V12') + path('M37 -12Q29 0 37 12') + plus(19, -9)`
   (apex at x = 33, ends at x = 37; IEEE 315 polarized capacitor style 2, IEC 04-02-06).
3. **zener, IEC** — one stub only, at the lower end, pointing back to the anode:
   `path('M0 0H22M38 0H60') + path('M22 -10L38 0L22 10Z') + path('M38 -10V10H33')`
   (IEC 617-5 05-03-06 with qualifier 05-02-03). Keep the current two-ended Z for ANSI.
4. **source, IEC** — the conductor runs straight through the circle; polarity outside, next to pin a:
   `path('M0 0H60') + '<circle cx="30" cy="0" r="14"/>' + upright(part, 8, -8, plus(0, 0))`
   (IEC 617-2 02-16-02; the "+" keeps ECalc's "pin a is positive" convention). Keep "+"/"−" inside for ANSI.
5. **isource, IEC** — circle with a line across, perpendicular to the leads; direction as an arrow on the lead:
   `path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + path('M30 -14V14') + head(54, 0, 1, 0, 7, 4)`
   (IEC 617-2 02-16-01; the outside arrow is ECalc's reference direction a → b, not part of the IEC symbol). Keep the
   arrow-in-circle for ANSI.
6. **opamp, IEC** — rectangle instead of the triangle; same pins:
   `'<rect x="20" y="-40" width="60" height="80"/>' + path('M0 -20H20M0 20H20M80 0H100') + path('M34 -32V-18L46 -25Z')`
   `+ upright(part, 58, -25, '<text x="0" y="4.5" text-anchor="middle" class="ec-sch-text" stroke="none" fill="currentColor">∞</text>')`
   `+ path('M24 -20h8') + plus(28, 20)`
   (box 60 × 80 px from x 20 to 80, output stub 80–100 to the unchanged pin `out` (5,0); ▷ drawn as a small open
   triangle at the top left, ∞ beside it, the input signs inside the box next to the input stubs)
   (IEC 617-13 13-04-02: amplifier sign ▷ and ∞ at the top, "−"/"+" written inside the box at the inputs; the
   example also marks the output "+", optional). Keep the triangle for ANSI.
7. Optional, not required by either standard:
   - **fuse, ANSI** — S-curve to make the difference visible: `path('M0 0H18A6 6 0 0 1 30 0A6 6 0 0 0 42 0H60')`
     (NEETS M3 Fig. 2-1 and p. AIII-8; the current IEC form is also valid ANSI).
   - **switch, IEC** — drop the circle at the fixed contact b (`<circle cx="45" …>`); the hinge circle at a may stay.
   - **npn/pnp/nmos/pmos, ANSI** — envelope circle (IEEE 315 drawings show it; ANSI Y32.2 A4.11 lets it be omitted).

## Sources
1. IS 12032 (Part 2):1987, *Graphical symbols for diagrams in the field of electrotechnology, Part 2: Symbol elements,
   qualifying symbols and other symbols having general application*, identical with IEC Pub 617-2 (1983). Read: p. 20
   Section 15 (02-15-01 earth, general symbol), p. 21 Section 16 (02-16-01 ideal current source, 02-16-02 ideal voltage
   source). https://archive.org/details/gov.in.is.12032.2.1987 — page scans viewed, accessed 2026-11-27.
2. IS 12032 (Part 4):1987 (reaffirmed 2004), *… Part 4: Passive components*, national foreword: "identical with IEC
   Pub 617-4 (1983)". Read: p. 3 Section 1 resistors (04-01-01 preferred form, 04-01-02 other form, 04-01-07
   potentiometer with sliding contact), p. 4 Section 2 capacitors (2.1 "Two forms are shown, but both should not be
   used on the same diagram"; 04-02-01/02, 04-02-05/06 polarized capacitor), Section 3 inductors (04-03-01 preferred,
   04-03-02 other form, notes 1–2, 04-03-03 core), p. 9 Section 7 (04-07-01 piezoelectric crystal with two
   electrodes). https://archive.org/details/gov.in.is.12032.4.1987 — accessed 2026-11-27.
3. IS 12032 (Part 5):1993, *… Part 5: Semiconductors and electron tubes*, identical with IEC Pub 617-5 (1983). Read:
   p. 3 Section 2 (05-02-03 unidirectional breakdown effect) and Section 3 (05-03-01 diode, 05-03-02 LED), p. 4
   (05-03-06 breakdown diode, unidirectional; voltage regulator diode), p. 5 Section 5 (05-05-01 PNP transistor,
   05-05-02 NPN with collector connected to the envelope), pp. 6–7 (05-05-11 … 05-05-17 IGFETs).
   https://archive.org/details/gov.in.is.12032.5.1993 — accessed 2026-11-27.
4. IS 12032 (Part 6):1987, identical with IEC Pub 617-6 (1983). Read: p. 10 Chapter III notes III.1–III.4 and Section 9
   (06-09-01 form 1, 06-09-02 form 2, transformer with two windings), p. 18 Section 15 (06-15-01 primary cell — "The
   longer line represents the positive pole … The short line may be thickened for emphasis"; 06-15-02/03 battery).
   https://archive.org/details/gov.in.is.12032.6.1987 — accessed 2026-11-27.
5. IS 12032 (Part 7):1987, identical with IEC Pub 617-7 (1983). Read: p. 3 Chapter I note I.2 ("A small circle, open
   or filled in, representing the hinge-point may be added …"), p. 5 Section 2 (07-02-01 make contact form 1, "also
   used as the general symbol for a switch"; 07-02-02 form 2), p. 31 Section 21 (07-21-01 fuse, general symbol).
   https://archive.org/details/gov.in.is.12032.7.1987 — accessed 2026-11-27.
6. IS 12032 (Part 8):1987, identical with IEC Pub 617-8 (1983). Read: p. 4 (08-01-01 indicating instrument), p. 5
   (08-02-01 voltmeter). https://archive.org/details/gov.in.is.12032.8.1987 — accessed 2026-11-27.
7. IS 12032 (Part 13):1992, identical with IEC Pub 617-13 (1978). Read: p. 4 Section Four (13-04-02 "High gain
   differential amplifier (operational amplifier)", rectangle with ▷ ∞, inputs − and +; note on the ∞ sign).
   https://archive.org/details/gov.in.is.12032.13.1992 — accessed 2026-11-27.
8. IS 2032 (Part XVI):1972, *Graphical symbols used in electrotechnology, Part XVI: Aerials*, foreword 0.4 "assistance
   has been derived from IEC Pub 117-10 (1968)". Read: p. 3 clause 2.1 aerial (trident on a stem).
   https://archive.org/details/gov.in.is.2032.16.1972 — accessed 2026-11-27.
9. US Navy, *NEETS Module 3 – Introduction to Circuit Protection, Control, and Measurement*, NAVEDTRA 14175, September
   1998 ("Distribution Statement A: Approved for public release"). Read: p. 2-4 Figure 2-1 "Typical fuses and schematic
   symbols" (cartridge form and S-curve); Appendix III "Schematic Symbols", pp. AIII-2 (fixed contact: IEC open circle
   or arrow), AIII-3 ("Single-throw, general", marked IEC), AIII-8 ("Fuse … General": IEC rectangle-with-line OR
   cartridge OR S-curve; "Fuse, supply side indicated by a thick line"), AIII-10 ("Meter, Instrument": letters A, V
   marked IEC). The appendix pages carry IEEE 315's section titles and "IEC" markers; the module does not name its
   source. https://archive.org/details/NEETSModules (file "NEETS Module 03- Introduction to Circuit Protection, Control,
   and Measurement") — page scans viewed, accessed 2026-11-27.
10. US Navy, *NEETS Module 7 – Introduction to Solid-State Devices and Power Supplies*, NAVEDTRA 14179, September 1998.
    Read: pp. 3-6/3-7 Figures 3-4A–E "Schematic symbols for Zener diodes" (five variants), text with Figure 3-27 ("a
    standard diode symbol with two arrows pointing away"). Same archive item — accessed 2026-11-27.
11. US Navy, *NEETS Module 8 – Introduction to Amplifiers*, NAVEDTRA 14180. Read: p. 3-16 Figures 3-10A/B "Schematic
    symbols of an operational amplifier" (triangle, inputs − and +). Same archive item — accessed 2026-11-27.
12. Wikimedia Commons, files named after IEEE Std 315-1975 (1993) items, with the item text in the file description
    (secondary redrawings): "IEEE 315-1975 (1993) 2.1.1.a/.b" (resistor general: zigzag / rectangle "Always add
    identification within or adjacent to the rectangle"), "… 8.5.1" (diode), "… 8.3.1" (breakdown indicator),
    "… 8.5.6.1.a/.c" (breakdown diode, unidirectional), "… 8.6.1/8.6.2" (PNP/NPN, with envelope), "… 8.6.10.5/8.6.11.5"
    (N-/P-channel enhancement IGFET), "IEEE 315 Fundamental Items Symbols (32)–(36)" (capacitor general style 1/2,
    polarized style 1/2), "(55)/(56)" (antenna – general), "(90)–(92)" (battery), "(105)" (generalized AC source),
    "(113)" (piezoelectric crystal unit), "IEEE 315 Transmission Path Symbols (75)" (ground, general), "IEEE 315
    Contacts, Switches … (49)" (single-throw switch, general), "Fuces.svg" (description: "IEC-style fuse (a), other
    symbols (b,c) (IEEE Std 315-1975)"). https://commons.wikimedia.org/ — 598 file descriptions listed via the API,
    thumbnails viewed, accessed 2026-11-27.
13. en.wikipedia, *Electronic symbol*, https://en.wikipedia.org/wiki/Electronic_symbol (secondary): captions "IEC
    style / ANSI style" resistors, "Polarized capacitor (IEC 60617 style)" / "(old alternate IEEE 315 style)", the
    "Obsolete capacitor" gallery (incl. the hollow-rectangle plate), "Zener diode", "DC voltage source", "Current
    source", "AC voltage source", "Operational amplifier", "General antenna (IEC-style)", NPN/PNP "may include a
    circle" citing ANSI Y32.2-1975 A4.11; fuse caption "(b) IEC, (a,c) IEEE/ANSI" — contradicts the file's own
    description and source 5 (see Cross-check). Accessed 2026-11-27.
14. de.wikipedia (secondary): *Z-Diode* ("Schaltzeichen einer Z-Diode", one stub), *Spannungsquelle* ("Schaltzeichen
    gemäß DIN EN 60617-2 für eine ideale Gleichspannungsquelle"), *Stromquelle* (left symbol "gemäß DIN 40700 …
    Normung … inzwischen international in IEC 60617-2:1996 und DIN EN 60617-2:1997", right symbol "US-amerikanisch"),
    *Operationsverstärker* ("Schaltsymbol nach DIN EN 60617 Teil 13", file Normsymbol_OPV.svg = rectangle with ▷∞),
    *Schmelzsicherung* (Fuse symbol Europe = rectangle with line). Accessed 2026-11-27.

Not opened / not used: IEC 60617 database pages (https://library.iec.ch/grasymb/…, e.g. S00555): AWS WAF bot
challenge, not bypassed — so no S-numbers and no current-edition drawings were read. IEEE Std 315-1975 PDFs found
online (e.g. on documents.epfl.ch) carry an IHS licence stamp — not a legitimate free copy, not used; the ANSI
Y32.2-1975 copy on a private site that en.wikipedia cites (julesbartow.com) was not used for the same reason. IEC 60617-4:1996
and EN 60617-4:1996 previews: none found. DOE-HDBK-1016/1-93: page did not expose the PDF link; not read.

## Cross-check
- **resistor**: IEC rectangle — IS 12032-4 04-01-01 = Wikipedia "IEC resistors". ANSI zigzag — Commons IEEE 2.1.1.a
  = Wikipedia ANSI image. Agreed. Note: both standards also list the other form (IEC 617-4:1983 04-01-02 zigzag as
  "other form"; IEEE 2.1.1.b rectangle with identification).
- **polarized**: IEC two straight plates + "+" — IS 12032-4 04-02-05 = Wikipedia "(IEC 60617 style)" = Commons FI 35
  (IEEE style 1). Curved negative plate — IS 12032-4 04-02-06 (IEC other form) = Commons FI 36 (IEEE style 2,
  "old alternate IEEE 315 style"). The hollow-rectangle plate ECalc uses appears in no standard read; Wikipedia lists it
  as obsolete. The curve direction ("(" towards the straight plate) is the same in IS 12032-4 04-02-02/06 and Commons
  FI 33/36. Agreed.
- **zener**: IEC one stub — IS 12032-5 05-03-06 = de.wikipedia Z-Diode. ANSI bar bent at both ends — Commons IEEE
  8.5.6.1.c = en.wikipedia "Zener diode" (mirror-image orientations). Agreed that the forms differ; NEETS M7 shows
  five US variants, so the US side is a family, not one shape.
- **source / isource**: IEC forms — IS 12032-2 02-16-01/02 = de.wikipedia (DIN EN 60617-2 / IEC 60617-2:1996). US
  arrow-in-circle current source — de.wikipedia "US-amerikanisch" = en.wikipedia. US +/− voltage source — en.wikipedia
  only; IEEE 315 (Commons FI 91) uses the battery as "generalized direct-current source".
- **opamp**: IEC rectangle — IS 12032-13 13-04-02 = de.wikipedia DIN EN 60617-13 image. Triangle — NEETS M8 Fig. 3-10
  = en.wikipedia. Agreed.
- **fuse**: IEC rectangle-with-line — IS 12032-7 07-21-01 = de.wikipedia = NEETS AIII-8 ("IEC" marker). US S-curve —
  NEETS Fig. 2-1 = NEETS AIII-8 = Commons "Fuces.svg" description. **Conflict**: the en.wikipedia caption of the same
  image says "(b) IEC, (a,c) IEEE/ANSI", while the file description says "IEC-style fuse (a), other symbols (b,c)";
  the standard (IS 12032-7) and NEETS side with the file description.
- **switch**: IS 12032-7 07-02-01 (no circles) = NEETS AIII-3 ("Single-throw, general", IEC marker) = Commons CS 49.
  Circles: IS 12032-7 note I.2 (hinge circle) and NEETS AIII-2 (open-circle fixed contact) — both optional.
- **same-in-both rows** (capacitor, inductor, diode, LED, crystal, battery, meter, ground, transistors, MOSFETs,
  transformer): each IEC number was read in IS 12032 and the US counterpart seen in Commons/NEETS as listed in `check`.

## Notes (en / de)
- en: IEC 60617 is the international standard (in Germany DIN EN 60617); IEEE 315 / ANSI Y32.2 was the US standard.
  de: IEC 60617 ist die internationale Norm (in Deutschland DIN EN 60617); IEEE 315 / ANSI Y32.2 war die US-Norm.
  (The statement that IEEE 315 is inactive since 2019 had no source in this research and is not shipped.)
- en: Both standards accept several forms for some parts; never mix two forms of the same part in one diagram. de: Beide
  Normen lassen für manche Bauteile mehrere Formen zu; im selben Schaltplan nie zwei Formen desselben Bauteils
  mischen.
- en: Drawings in datasheets often mix styles (zigzag resistor next to IEC symbols). de: Datenblätter mischen die Stile
  oft (Zickzack-Widerstand neben IEC-Zeichen).

## Suggested finder / lookups
- Table with two drawing columns (IEC, ANSI) rendered from `ECalcSymbols.symbolMarkup({sym, x, y}, 'iec'|'ansi')`,
  then name, function and difference; filter "only rows that differ" (`differs` = yes).

UNVERIFIED:
- **IEC 60617 database S-numbers and current drawings**: not read (bot challenge). All IEC statements are from the
  1978–1983 parts (identical Indian adoptions). If the database changed a drawing since, this record does not know.
- **IEEE 315 clause numbers**: taken from Commons file names, not from the standard; Fundamental Items (capacitor,
  battery, crystal, antenna, AC source) have no clause number in the sources read.
- **antenna, IEC**: IS 2032-16:1972 (from IEC 117-10:1968) shows a trident; Wikipedia calls the V-on-a-stem "IEC
  style" — current IEC form not read. Keep ECalc's V.
- **acsource, IEC**: no IEC AC-source symbol found in the parts read. Keep ECalc's circle with sine.
- **source, ANSI**: circle with + and − is a US textbook convention (en.wikipedia only); IEEE 315 itself (as read on
  Commons) uses the battery as the generalized DC source. Keep ECalc's drawing.
- **zener, ANSI orientation**: the two-ended form appears in both mirror orientations; NEETS shows five variants.
  ECalc's current Z is one accepted form.

## Shipped table (deviations from the research record)
- Columns shipped: `name` (the key column, so that row headers and the accessible names of the drawings are the part names in the page language), `symbol` (dim `symbol`: the library id, the page draws IEC and ANSI side by side), `function`, `iec_ref` (IEC 617 item number without the "IEC 617-x" prefix), `difference`. The research columns `ieee_ref` (clause numbers taken only from Wikimedia file names, one non-authoritative source), `differs`, `ecalc` and `check` are not shipped; the cross-check is in this record.
- `iec_ref` is null (UNVERIFIED) for `acsource` (no IEC form read), `antenna` (IS 2032-16:1972 shows a trident, current form not read) and `ic` (no standard symbol).
- The `difference` text of `fuse`, `switch`, `source`, `isource`, `polarized`, `antenna` and `opamp` says what ECalc draws. The opamp row says that IEC 60617 draws a rectangle with the amplifier sign and the infinity sign while ECalc shows the triangle in both modes.
- The `difference` of `polarized` reads "curved towards the straight plate" because the ANSI drawing was corrected (see below).

## Drawing changes made in ecalc-symbols.js and why
1. `polarized`, IEC: two straight plates (`M0 0H27M33 0H60`, `M27 -12V12M33 -12V12`) plus "+" (IEC 617-4 04-02-05) instead of a hollow rectangle plate, which appears in no standard read.
2. `polarized`, ANSI: the curved plate `M37 -12Q29 0 37 12` bulges towards the straight plate, lead from the apex at x 33 (IEEE style 2, IEC 04-02-06); before, it bulged away.
3. `zener`, IEC: a single stub at the lower end pointing back to the anode (`M38 -10V10H33`, IEC 617-5 05-03-06); ANSI keeps the two-ended Z.
4. `source`, IEC: the conductor runs straight through the circle (`M0 0H60`), "+" outside next to pin a (IEC 617-2 02-16-02); ANSI keeps "+" and "−" inside.
5. `isource`, IEC: circle with a line across, perpendicular to the leads (`M30 -14V14`, IEC 617-2 02-16-01), reference direction a to b as an arrow on the lead (`head(54, 0, 1, 0, 7, 4)`); ANSI keeps the arrow inside.
Pins and the symbol boxes did not change: every new stroke lies inside the old footprint.
