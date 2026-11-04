# line_interface_levels — Signal levels of RS-232, RS-485 and LVDS / Signalpegel von RS-232, RS-485 und LVDS

Kind: transcribed standard
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| interface | interface and governing standard | Schnittstelle und Norm | – | no |
| parameter | parameter "en\|de" | Parameter "en\|de" | – | no |
| min | minimum value (empty = not specified) | Minimalwert | see `unit` | yes |
| typ | typical / nominal value | typischer / Nennwert | see `unit` | yes |
| max | maximum value | Maximalwert | see `unit` | yes |
| unit | unit of min/typ/max | Einheit von min/typ/max | – | no |
| condition | test condition / remark "en\|de" | Bedingung / Hinweis "en\|de" | – | no |
| check | cross-check record; "(UNVERIFIED)" = single source | Quellenabgleich; "(UNVERIFIED)" = nur eine Quelle | – | no |

## Rows
```csv
interface,parameter,min,typ,max,unit,condition,check
RS-232 (TIA-232-F),"driver output, logic 0 / space / ON|Treiberausgang, logisch 0 / Space / EIN",5,,15,V,load 3–7 kΩ|Last 3–7 kΩ,TI SLLA037A = Wikipedia (+5…+15 V)
RS-232 (TIA-232-F),"driver output, logic 1 / mark / OFF|Treiberausgang, logisch 1 / Mark / AUS",-15,,-5,V,load 3–7 kΩ|Last 3–7 kΩ,TI SLLA037A = Wikipedia (−5…−15 V)
RS-232 (TIA-232-F),receiver input recognised as logic 0 (space)|Empfängereingang als logisch 0 (Space) erkannt,3,,15,V,,TI SLLA037A = Wikipedia
RS-232 (TIA-232-F),receiver input recognised as logic 1 (mark)|Empfängereingang als logisch 1 (Mark) erkannt,-15,,-3,V,,TI SLLA037A = Wikipedia
RS-232 (TIA-232-F),transition region (undefined)|Übergangsbereich (undefiniert),-3,,3,V,,TI SLLA037A = Wikipedia
RS-232 (TIA-232-F),maximum voltage at connector pins|maximale Spannung an den Steckerkontakten,-25,,25,V,,TI SLLA037A = Wikipedia
RS-232 (TIA-232-F),receiver input impedance|Eingangswiderstand des Empfängers,3,,7,kΩ,,TI SLLA037A only (UNVERIFIED)
RS-232 (TIA-232-F),driver slew rate|Flankensteilheit des Treibers,,,30,V/µs,,TI SLLA037A only (UNVERIFIED)
RS-485 (TIA/EIA-485-A),"driver differential output voltage, magnitude of VOD|Betrag der differenziellen Treiberausgangsspannung VOD",1.5,,,V,RL = 54 Ω|RL = 54 Ω,TI SLLA272D = Wikipedia = TI SN65HVD72 (VOD ≥ 1.5 V at 54 Ω)
RS-485 (TIA/EIA-485-A),receiver differential threshold|Differenzielle Empfängerschwelle,-200,,200,mV,a standard receiver detects a differential input of 200 mV or more|ein Standardempfänger erkennt Differenzspannungen ab 200 mV,TI SLLA272D = Wikipedia
RS-485 (TIA/EIA-485-A),common-mode voltage range|Gleichtaktspannungsbereich,-7,,12,V,,TI SLLA272D = Wikipedia = TI SN65HVD72 (bus input −7…12 V)
RS-485 (TIA/EIA-485-A),unit loads per bus (standard drivers)|Lasteinheiten je Bus (Standardtreiber),,,32,UL,1 UL ≈ 12 kΩ|1 UL ≈ 12 kΩ,32 UL: TI SLLA272D = Wikipedia; 12 kΩ: TI only (UNVERIFIED)
LVDS (TIA/EIA-644-A),"driver differential output voltage, magnitude of VOD|Betrag der differenziellen Treiberausgangsspannung VOD",247,350,454,mV,RL = 100 Ω|RL = 100 Ω,typ 350 mV: TI LVDS Owner's Manual = Wikipedia; 247/454 mV: TI SN65LVDS31 only (UNVERIFIED); datasheet typ 340 mV
LVDS (TIA/EIA-644-A),driver offset (common-mode) voltage VOS|Offset- (Gleichtakt-)spannung VOS des Treibers,1.125,1.2,1.375,V,,typ 1.2 V: TI SN65LVDS31 = Wikipedia; 1.125/1.375 V: TI SN65LVDS31 only (UNVERIFIED)
LVDS (TIA/EIA-644-A),receiver differential threshold|Differenzielle Empfängerschwelle,-100,,100,mV,,TI LVDS Owner's Manual = Frontgrade AN-LVDS-010-01
LVDS (TIA/EIA-644-A),receiver input voltage range|Eingangsspannungsbereich des Empfängers,0,,2.4,V,,TI LVDS Owner's Manual = Frontgrade AN-LVDS-010-01
LVDS (TIA/EIA-644-A),driver current (into the termination)|Treiberstrom (in den Abschluss),,3.5,,mA,nominal|nominal,TI LVDS Owner's Manual = Wikipedia
LVDS (TIA/EIA-644-A),termination resistor|Abschlusswiderstand,,100,,Ω,at the receiver|am Empfänger,TI LVDS Owner's Manual = TI SN65LVDS31 (RL = 100 Ω); Wikipedia: about 100–120 Ω
```

## Sources
1. Texas Instruments, *Interface Circuits for TIA/EIA-232-F — Design Notes*, SLLA037A (September 2002), https://www.ti.com/lit/an/slla037a/slla037a.pdf — section "TIA/EIA-232-F Electrical Specification" and Figure 3 (±25 V max, 3–7 kΩ load, logic 0 = +5…+15 V, logic 1 = −5…−15 V at the driver; +3…+15 V / −3…−15 V at the receiver; ±3 V transition region; 30 V/µs slew rate; < 2500 pF); accessed 2026-11-02.
2. en.wikipedia, "RS-232", https://en.wikipedia.org/wiki/RS-232 — valid levels ±3…±15 V, ±3 V invalid, 25 V maximum, logic 1 negative on data lines; accessed 2026-11-02.
3. Texas Instruments, *The RS-485 Design Guide*, SLLA272D (Feb 2008, rev. May 2021), https://www.ti.com/lit/an/slla272d/slla272d.pdf — driver ≥ 1.5 V into 54 Ω, receiver 200 mV, −7…+12 V common mode, 32 unit loads, unit load ≈ 12 kΩ; accessed 2026-11-02.
4. en.wikipedia, "RS-485", https://en.wikipedia.org/wiki/RS-485 — 1.5 V across 54 Ω, 200 mV, −7…+12 V, at least 32 unit loads; accessed 2026-11-02.
5. Texas Instruments, SN65HVD72/75/78 data sheet, SLLSE11H (rev. March 2019), https://www.ti.com/lit/ds/symlink/sn65hvd72.pdf — |VOD| ≥ 1.5 V at RL = 54 Ω, bus input −7…12 V; accessed 2026-11-02.
6. Texas Instruments, SN65LVDS31 (and family) data sheet, SLLS261N (rev. April 2021), https://www.ti.com/lit/ds/symlink/sn65lvds31.pdf — "meet or exceed ANSI TIA/EIA-644"; |VOD| 247 / 340 / 454 mV (min/typ/max) at RL = 100 Ω; VOC(SS) 1.125 / 1.2 / 1.375 V; accessed 2026-11-02.
7. Texas Instruments (National Semiconductor), *LVDS Owner's Manual*, 4th edition 2008, SNLA187, https://www.ti.com/lit/ug/snla187/snla187.pdf — nominal 3.5 mA, 100 Ω, 350 mV, receiver threshold ≤ 100 mV over 0…2.4 V common mode, ANSI/TIA/EIA-644A; accessed 2026-11-02.
8. Frontgrade (formerly Aeroflex/Cobham), application note AN-LVDS-010-01 "LVDS Receiver Input Thresholds", v1.0.0 (2 June 2011), https://www.frontgrade.com/sites/default/files/documents/app-note-lvds-receiverinputthresholdsr1.0.0.pdf — ±100 mV guaranteed receiver thresholds over 0…2.4 V; accessed 2026-11-02.
9. en.wikipedia, "Low-voltage differential signaling", https://en.wikipedia.org/wiki/Low-voltage_differential_signaling — 3.5 mA, about 350 mV, about 1.2 V common mode, about 100–120 Ω termination; accessed 2026-11-02.

## Cross-check
- The TIA standards themselves (TIA-232-F, TIA/EIA-485-A, TIA/EIA-644-A) are paywalled and were not read; values come from maker documents that cite them plus Wikipedia.
- RS-232: driver ±5…±15 V, receiver ±3 V thresholds, ±25 V maximum — sources 1 and 2 agree. 3–7 kΩ load and 30 V/µs slew rate only in source 1 → **UNVERIFIED**.
- RS-485: 1.5 V / 54 Ω, ±200 mV, −7…+12 V, 32 UL — sources 3 and 4 agree (source 5 agrees on 1.5 V and −7…12 V). Unit load ≈ 12 kΩ only in source 3 → **UNVERIFIED**.
- LVDS: ±100 mV threshold and 0…2.4 V input range — sources 7 and 8 agree. 350 mV / 3.5 mA / 1.2 V / 100 Ω — sources 7 and 9 agree (source 6 typ 340 mV). VOD 247…454 mV and VOS 1.125…1.375 V only from source 6 (a part that "meets TIA/EIA-644") → **UNVERIFIED** as standard limits; a Fairchild app note quoted in a web search gives 250…450 mV (rounded), which I could not open.

## Notes (en / de)
- en: RS-232 uses negative logic on data lines: logic 1 (mark, idle) is negative, logic 0 (space) positive. Control lines are active when positive.
- de: RS-232 nutzt auf den Datenleitungen negative Logik: logisch 1 (Mark, Ruhezustand) ist negativ, logisch 0 (Space) positiv. Steuerleitungen sind bei positiver Spannung aktiv.
- en: RS-485 and LVDS are differential: the receiver evaluates VA − VB; terminate the line with its characteristic impedance (RS-485 typically 120 Ω at both ends, LVDS 100 Ω at the receiver).
- de: RS-485 und LVDS sind differenziell: Der Empfänger wertet VA − VB aus; die Leitung wird mit ihrem Wellenwiderstand abgeschlossen (RS-485 typisch 120 Ω an beiden Enden, LVDS 100 Ω am Empfänger).
- en: Noise margin RS-485 = 1.5 V − 0.2 V = 1.3 V; RS-232 = 5 V − 3 V = 2 V (source 1: "2-V minimum noise margin").
- de: Störabstand RS-485 = 1,5 V − 0,2 V = 1,3 V; RS-232 = 5 V − 3 V = 2 V (Quelle 1: "2-V minimum noise margin").

## Suggested finder / lookups
- Interface → level summary; RS-485 bus loading helper: number of transceivers = 32 / (unit-load rating of the part, e.g. 1/8 UL → 256).
