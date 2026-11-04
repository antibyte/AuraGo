# ipc2221b_conductor_spacing — PCB electrical conductor spacing, IPC-2221B Table 6-1 / Leiterabstände auf Leiterplatten, IPC-2221B Tabelle 6-1

Kind: transcribed standard (IPC-2221B, Table 6-1 "Electrical Conductor Spacing"; same table in IPC-2221A)
Guide-value note needed: no

Minimum spacing between conductors by voltage between them (DC or AC peak). Rows are voltage bands; the last row gives a per-volt increment for voltages above 500 V.
Mindestabstand zwischen Leitern abhängig von der Spannung zwischen ihnen (DC oder AC-Scheitelwert).

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| v_min | Lower bound of the voltage band (DC or AC peak) | Untere Grenze des Spannungsbereichs (DC oder AC-Scheitelwert) | V | yes |
| v_max | Upper bound (empty = open-ended, row ">500") | Obere Grenze (leer = offen, Zeile „>500") | V | yes |
| b1 | B1 — internal conductors | B1 — Innenlagen | mm (last row: mm/V) | yes |
| b2 | B2 — external conductors, uncoated, sea level to 3050 m | B2 — Außenlagen, unbeschichtet, bis 3050 m Höhe | mm (mm/V) | yes |
| b3 | B3 — external conductors, uncoated, over 3050 m | B3 — Außenlagen, unbeschichtet, über 3050 m Höhe | mm (mm/V) | yes |
| b4 | B4 — external conductors with permanent polymer coating (any elevation) | B4 — Außenlagen mit dauerhafter Polymerbeschichtung (jede Höhe) | mm (mm/V) | yes |
| a5 | A5 — external conductors with conformal coating over assembly (any elevation) | A5 — Außenlagen mit Schutzlack über der bestückten Baugruppe (jede Höhe) | mm (mm/V) | yes |
| a6 | A6 — external component lead/termination, uncoated, sea level to 3050 m | A6 — Bauteilanschluss außen, unbeschichtet, bis 3050 m | mm (mm/V) | yes |
| a7 | A7 — external component lead/termination, with conformal coating (any elevation) | A7 — Bauteilanschluss außen, mit Schutzlack (jede Höhe) | mm (mm/V) | yes |
| per_volt | 1 for the ">500 V" row (values are mm per volt), else 0 | 1 für Zeile „>500 V" (Werte in mm je Volt), sonst 0 | — | yes |

## Rows
```csv
v_min,v_max,b1,b2,b3,b4,a5,a6,a7,per_volt
0,15,0.05,0.1,0.1,0.05,0.13,0.13,0.13,0
16,30,0.05,0.1,0.1,0.05,0.13,0.25,0.13,0
31,50,0.1,0.6,0.6,0.13,0.13,0.4,0.13,0
51,100,0.1,0.6,1.5,0.13,0.13,0.5,0.13,0
101,150,0.2,0.6,3.2,0.4,0.4,0.8,0.4,0
151,170,0.2,1.25,3.2,0.4,0.4,0.8,0.4,0
171,250,0.2,1.25,6.4,0.4,0.4,0.8,0.4,0
251,300,0.2,1.25,12.5,0.4,0.4,0.8,0.8,0
301,500,0.25,2.5,12.5,0.8,0.8,1.5,0.8,0
500,,0.0025,0.005,0.025,0.00305,0.00305,0.00305,0.00305,1
```

## Sources
1. Altium Resources, Z. Peterson, "Using an IPC-2221 Calculator for High Voltage Design" (created 2020-01-17, updated 2025), figure "IPC-2221 Table 6.1" — image of Table 6-1 with mm and inch values and the B1–A7 legend: https://resources.altium.com/p/using-an-ipc-2221-calculator-for-high-voltage-design (image: https://files.resources.altium.com/sites/default/files/styles/max_width_1300/public/inline-images/ipc2221-table-6-1.png) — read from the image in the built-in browser; accessed 2026-11-02.
2. P. McGaw, "Creepage and clearance" (2023-02-21), HTML table "IPC-2221B Conductor spacing requirements" with B1–A7 legend: https://philipmcgaw.com/creepage-and-clearance/ — accessed 2026-11-02.
3. smps.us, "PCB trace spacing" (text remark: IPC gives 1.25 mm for 300 V and 2.5 mm for 301 V), citing Table 6.1 of IPC-2221B: https://www.smps.us/pcbtracespacing.html — accessed 2026-11-02.
4. S. V. Chavez, Siemens EDA blog, "PCB high voltage spacing: What every engineer should know" (2025-04-29) — worked example: B1 at 600 V = 0.25 mm + 100 V × 0.0025 mm/V = 0.50 mm: https://blogs.sw.siemens.com/electronic-systems-design/2025/04/29/pcb-high-voltage-spacing-what-every-engineer-should-know/ — accessed 2026-11-02.

## Cross-check
- Sources 1 and 2 agree on 69 of 70 cells. Disagreement: B2, 251–300 V — source 2 gives 2.5 mm, source 1 gives 1.25 mm. Source 3 (1.25 mm at 300 V, 2.5 mm at 301 V) and IPC-2221C (1.25 mm, see ipc2221c_conductor_spacing) support 1.25 mm → chosen 1.25 mm.
- Above 500 V: source 1 says "> 500 — see para. 6.3 for calc."; source 4 applies spacing = value(301–500 V) + (V − 500) × per-volt value. Source 2 writes the row as "V × 0.0025" etc., which would give much larger values; chosen the source-1/4 reading (also used by pcb-toolkit for Rev C).
- Inch values in source 1 (e.g. 0.6 mm = 0.024 in, 1.25 mm = 0.0492 in) are conversions printed by IPC; ECalc converts itself.

## Notes (en / de)
- en: IPC-2221 spacing is functional spacing for printed boards; for mains or safety isolation use IEC 60664-1 / IEC 62368-1 creepage and clearance. Solder resist does not count as a coating for B2/B3. The rows use whole volts (e.g. 16–30 V); a voltage between bands (e.g. 15.5 V) belongs to the next band up. IPC-2221C (2023) revised this table — see ipc2221c_conductor_spacing.
- de: IPC-2221 gibt Funktionsabstände für Leiterplatten an; für Netz- bzw. Schutztrennung gelten Kriech- und Luftstrecken nach IEC 60664-1 / IEC 62368-1. Lötstopplack zählt nicht als Beschichtung (B2/B3). Spannungen zwischen den Bändern gehören zum nächsthöheren Band. IPC-2221C (2023) hat die Tabelle überarbeitet.

## Suggested finder / lookups
Finder (b), see SUMMARY.md: input peak voltage V and column; if V ≤ 500 pick the row with v_min ≤ ceil(V) ≤ v_max; if V > 500: spacing = row(301–500) + (V − 500) × row(>500).
