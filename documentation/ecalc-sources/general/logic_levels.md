# logic_levels — Logic levels of common logic families / Logikpegel gängiger Logikfamilien

Kind: transcribed standard (datasheet limits)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | row id | Zeilen-Id | – | no |
| family | logic family / standard "en\|de" | Logikfamilie / Standard "en\|de" | – | no |
| vcc | supply range the limits apply to | Versorgungsbereich, für den die Grenzen gelten | V | no (range text) |
| vcc_nom | nominal supply used for numeric examples | Nennversorgung für Zahlenbeispiele | V | yes |
| vih_min | minimum input voltage recognised as HIGH (formula rows: value at vcc_nom in brackets) | minimale Eingangsspannung für HIGH | V | partly |
| vil_max | maximum input voltage recognised as LOW | maximale Eingangsspannung für LOW | V | partly |
| voh_min | minimum HIGH output voltage; "a / b" = values for the two load currents in voh_cond | minimale HIGH-Ausgangsspannung; "a / b" = Werte zu den zwei Lastströmen | V | partly |
| voh_cond | load current(s) for voh_min | Laststrom/-ströme zu voh_min | – | no |
| vol_max | maximum LOW output voltage; "a / b" as above | maximale LOW-Ausgangsspannung | V | partly |
| vol_cond | load current(s) for vol_max | Laststrom/-ströme zu vol_max | – | no |
| vt_typ | approximate switching threshold (TI Logic Guide figures) | ungefähre Schaltschwelle (TI Logic Guide) | V | yes |
| note | remark "en\|de" | Hinweis "en\|de" | – | no |
| check | cross-check record | Quellenabgleich | – | no |

## Rows
```csv
id,family,vcc,vcc_nom,vih_min,vil_max,voh_min,voh_cond,vol_max,vol_cond,vt_typ,note,check
ttl,"TTL (74xx, e.g. SN7400)|TTL (74xx, z. B. SN7400)",4.75–5.25,5,2.0,0.8,2.4,IOH = −0.4 mA,0.4,IOL = 16 mA,1.5,standard bipolar TTL|Standard-TTL (bipolar),inputs: TI SN7400 = TI Logic Guide = Wikipedia; outputs: TI SN7400 = TI Logic Guide (both TI)
ls,"74LS (Low-power Schottky TTL, e.g. SN74LS00)|74LS (Low-Power-Schottky-TTL, z. B. SN74LS00)",4.75–5.25,5,2.0,0.8,2.5,IOH = −0.4 mA,0.4 / 0.5,IOL = 4 mA / 8 mA,,"VOH/VOL from one datasheet only (UNVERIFIED); older data books give VOH 2.7 V for 74LS|VOH/VOL nur aus einem Datenblatt (UNVERIFIED); ältere Datenbücher nennen VOH 2,7 V für 74LS",inputs: TI SN74LS00 = Wikipedia (TTL); outputs: TI only
hc,"74HC (CMOS) at VCC = 4.5 V|74HC (CMOS) bei VCC = 4,5 V",2–6 (table at 4.5),4.5,3.15,1.35,4.4 / 3.84,IOH = −20 µA / −4 mA,0.1 / 0.33,IOL = 20 µA / 4 mA,,VOH/VOL limits for −40…85 °C; HC inputs are not TTL-compatible|VOH/VOL-Grenzen für −40…85 °C; HC-Eingänge sind nicht TTL-kompatibel,TI SN74HC00 = Diodes 74HC00 (all values)
hct,"74HCT (CMOS, TTL-compatible inputs)|74HCT (CMOS, TTL-kompatible Eingänge)",4.5–5.5,5,2.0,0.8,4.4 / 3.84,IOH = −20 µA / −4 mA (VCC 4.5 V),0.1 / 0.33,IOL = 20 µA / 4 mA (VCC 4.5 V),1.5,drives CMOS inputs from TTL outputs; −40…85 °C limits|zum Ansteuern von CMOS-Logik durch TTL-Ausgänge; Grenzen für −40…85 °C,TI SN74HCT00 = Diodes 74HCT00; Vt 1.5 V: TI Logic Guide (TTL-compatible CMOS)
cmos5,"CMOS 5 V generic (0.7/0.3 VCC rule), e.g. 74LVC at 4.5–5.5 V|CMOS 5 V allgemein (0,7/0,3-VCC-Regel), z. B. 74LVC bei 4,5–5,5 V",4.5–5.5,5,0.7·VCC (3.5),0.3·VCC (1.5),VCC − 0.1 / 3.8,IOH = −100 µA / −32 mA (VCC 4.5 V),0.1 / 0.55,IOL = 100 µA / 32 mA (VCC 4.5 V),2.5,outputs depend on the family; values shown for 74LVC1G00|Ausgangswerte sind familienabhängig; Werte für 74LVC1G00,inputs: TI Logic Guide (3.5/1.5) = Wikipedia (0.7/0.3 VDD) = TI and Diodes LVC1G00; outputs: TI = Diodes LVC1G00
lvcmos33,"LVCMOS / LVTTL 3.3 V (e.g. 74LVC at 3.0–3.6 V)|LVCMOS / LVTTL 3,3 V (z. B. 74LVC bei 3,0–3,6 V)",3.0–3.6,3.3,2.0,0.8,VCC − 0.1 / 2.4,IOH = −100 µA / −16 mA (VCC 3.0 V),0.1 / 0.4,IOL = 100 µA / 16 mA (VCC 3.0 V),1.5,"LVTTL output levels per TI Logic Guide: VOH 2.4 V, VOL 0.4 V|LVTTL-Ausgangspegel laut TI Logic Guide: VOH 2,4 V, VOL 0,4 V",TI SN74LVC1G00 = Diodes 74LVC1G00 = TI Logic Guide (3.3 V LVTTL)
lvcmos25,"LVCMOS 2.5 V (e.g. 74LVC at 2.3–2.7 V)|LVCMOS 2,5 V (z. B. 74LVC bei 2,3–2,7 V)",2.3–2.7,2.5,1.7,0.7,VCC − 0.1 / 1.9,IOH = −100 µA / −8 mA (VCC 2.3 V),0.1 / 0.3,IOL = 100 µA / 8 mA (VCC 2.3 V),1.2,"TI Logic Guide shows VOH 2.0 V / VOL 0.4 V for 2.5 V CMOS (load not stated)|TI Logic Guide nennt VOH 2,0 V / VOL 0,4 V für 2,5-V-CMOS (Last nicht angegeben)",TI SN74LVC1G00 = Diodes 74LVC1G00; inputs also = TI Logic Guide
lvcmos18,"LVCMOS 1.8 V (e.g. 74LVC at 1.65–1.95 V)|LVCMOS 1,8 V (z. B. 74LVC bei 1,65–1,95 V)",1.65–1.95,1.8,0.65·VCC (1.17),0.35·VCC (0.63),VCC − 0.1 / 1.2,IOH = −100 µA / −4 mA (VCC 1.65 V),0.1 / 0.45,IOL = 100 µA / 4 mA (VCC 1.65 V),0.9,"TI Logic Guide: VOH 1.35 V (= VCC − 0.45 V), VOL 0.45 V|TI Logic Guide: VOH 1,35 V (= VCC − 0,45 V), VOL 0,45 V",TI SN74LVC1G00 = Diodes 74LVC1G00; inputs at 1.8 V also = TI Logic Guide (1.17/0.63)
```

## Sources
1. Texas Instruments, *Logic Guide*, SDYU001AC (June 2015, revised November 2025), Figures 2-2 "5V Logic I/O Standards" and 2-3 "CMOS Logic I/O Standards", https://www.ti.com/lit/sg/sdyu001ab/sdyu001ab.pdf (URL still carries the older "ab" name, content is revision AC) — VIH/VIL/Vt/VOH/VOL for TTL, TTL-compatible CMOS, 5 V CMOS, 3.3 V LVTTL, 2.5 V, 1.8 V CMOS; accessed 2026-11-02.
2. Texas Instruments, SN5400/SN7400, SN54LS00/SN74LS00 data sheet, SDLS025D (rev. May 2017), https://www.ti.com/lit/ds/symlink/sn7400.pdf — TTL and LS input/output levels; accessed 2026-11-02.
3. Texas Instruments, SN74HC00 data sheet, SCLS181H (rev. August 2021), https://www.ti.com/lit/ds/symlink/sn74hc00.pdf; SN74HCT00 data sheet, SCLS062F (rev. October 2022), https://www.ti.com/lit/ds/symlink/sn74hct00.pdf; accessed 2026-11-02.
4. Diodes Incorporated, 74HC00 data sheet DS35319 Rev. 3-2, https://www.diodes.com/assets/Datasheets/74HC00.pdf; 74HCT00 data sheet DS35329 Rev. 3-2, https://www.diodes.com/assets/Datasheets/74HCT00.pdf; accessed 2026-11-02.
5. Texas Instruments, SN74LVC1G00 data sheet, SCES212AC (rev. August 2026), https://www.ti.com/lit/ds/symlink/sn74lvc1g00.pdf — VIH/VIL per VCC range, VOH/VOL per load; accessed 2026-11-02.
6. Diodes Incorporated, 74LVC1G00 data sheet DS32196 Rev. 11-2, https://www.diodes.com/assets/Datasheets/74LVC1G00.pdf — same parameters; accessed 2026-11-02.
7. en.wikipedia, "Logic level", https://en.wikipedia.org/wiki/Logic_level — CMOS 0.3/0.7 VDD rule, TTL VIL 0.8 V / VIH 2 V, 7400 at 5 V ±5 %; accessed 2026-11-02.

## Cross-check
- 74HC and 74HCT: TI (source 3) and Diodes (source 4) agree on every value used (VIH 3.15 / VIL 1.35 at 4.5 V for HC; VIH 2.0 / VIL 0.8 for HCT; VOH 4.4 V at −20 µA and 3.84 V at −4 mA; VOL 0.1 V at 20 µA and 0.33 V at 4 mA, −40…85 °C columns). At 25 °C TI also gives 3.98 V / 0.26 V; Diodes' wider temperature column (−40…125 °C) gives 3.7 V / 0.44 V (HC) and 3.70 V / 0.4 V (HCT) — the table uses the common −40…85 °C limits.
- 74LVC at 1.8/2.5/3.3/5 V: TI (source 5) and Diodes (source 6) agree on all VIH/VIL limits and on VOH/VOL for −40…85 °C. (Diodes' −40…125 °C column has lower VOH, e.g. 0.95 V at 1.65 V/−4 mA; not used.)
- TTL inputs (2.0/0.8 V): sources 1, 2, 7 agree. TTL outputs (2.4 V / 0.4 V): sources 1 and 2 agree, but both are TI documents → effectively one maker.
- **UNVERIFIED:** 74LS VOH 2.5 V and VOL 0.4/0.5 V are from one datasheet only (source 2); classic data books state VOH ≥ 2.7 V for SN74LS00 (and 2.5 V for SN54LS00) — I could not read a second source. JEDEC JESD8 standards (JESD8C.01 3.3 V, JESD8-5 2.5 V, JESD8-7 1.8 V) were **not** accessible (registration required); the values here are datasheet limits of real parts, which is what a designer checks anyway.
- `vt_typ` values are read from TI Logic Guide figures (source 1) and are approximate.

## Notes (en / de)
- en: Interfacing rule: a driver fits a receiver when VOH(min, driver) ≥ VIH(min, receiver) and VOL(max, driver) ≤ VIL(max, receiver). Example: 3.3 V LVCMOS output (VOH ≥ 2.4 V) drives 74HCT (VIH 2.0 V) but not 74HC at 5 V (VIH 3.15 V at 4.5 V).
- de: Verbindungsregel: Ein Treiber passt zu einem Empfänger, wenn VOH(min, Treiber) ≥ VIH(min, Empfänger) und VOL(max, Treiber) ≤ VIL(max, Empfänger). Beispiel: 3,3-V-LVCMOS-Ausgang (VOH ≥ 2,4 V) treibt 74HCT (VIH 2,0 V), aber nicht 74HC an 5 V (VIH 3,15 V bei 4,5 V).
- en: Output levels depend strongly on load current and temperature; always check the datasheet of the actual part.
- de: Ausgangspegel hängen stark von Laststrom und Temperatur ab; maßgeblich ist das Datenblatt des konkreten Bausteins.
- en: Noise margin high = VOH(min) − VIH(min); noise margin low = VIL(max) − VOL(max).
- de: Störabstand HIGH = VOH(min) − VIH(min); Störabstand LOW = VIL(max) − VOL(max).

## Suggested finder / lookups
- Compatibility checker: choose driver family and receiver family → compare VOH/VIH and VOL/VIL, show both noise margins (use the larger-current VOH/VOL value as worst case).
