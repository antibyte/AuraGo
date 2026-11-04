# ipc2221c_conductor_spacing — PCB electrical conductor spacing, IPC-2221C Table 6-1 / Leiterabstände auf Leiterplatten, IPC-2221C Tabelle 6-1

Kind: transcribed standard (IPC-2221C, December 2023, Table 6-1 "Electrical Conductor Spacing", p. 58)
Guide-value note needed: no

Current revision of the table. Eight columns: B1–B5 (conductors, creepage on same layer, internal or external) and A6–A8 (component lead, corona effect). Note 4 of the table: column B5 was formerly designated A5.
Aktuelle Revision; Spalten B1–B5 (Leiter) und A6–A8 (Bauteilanschlüsse).

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| v_min | Lower bound of voltage band (DC or AC peak) | Untere Grenze (DC oder AC-Scheitelwert) | V | yes |
| v_max | Upper bound (empty = ">500" row) | Obere Grenze (leer = Zeile „>500") | V | yes |
| b1 | B1 — internal conductors | B1 — Innenlagen | mm (mm/V) | yes |
| b2 | B2 — external conductors, uncoated, sea level to 3050 m | B2 — Außenlagen, unbeschichtet, bis 3050 m | mm (mm/V) | yes |
| b3 | B3 — external conductors, uncoated, over 3050 m or vacuum | B3 — Außenlagen, unbeschichtet, über 3050 m oder Vakuum | mm (mm/V) | yes |
| b4 | B4 — external conductors with polymer (solder-mask-type) coating, any elevation (wording per pcb-toolkit; verify) | B4 — Außenlagen mit Polymerbeschichtung, jede Höhe (Wortlaut prüfen) | mm (mm/V) | yes |
| b5 | B5 (formerly A5) — external conductors with conformal coating, any elevation | B5 (früher A5) — Außenlagen mit Schutzlack, jede Höhe | mm (mm/V) | yes |
| a6 | A6 — external component lead/termination, coated, any elevation (wording per pcb-toolkit; verify) | A6 — Bauteilanschluss außen, beschichtet (Wortlaut prüfen) | mm (mm/V) | yes |
| a7 | A7 — external component lead/termination, uncoated, sea level to 3050 m (per pcb-toolkit; verify) | A7 — Bauteilanschluss außen, unbeschichtet, bis 3050 m (prüfen) | mm (mm/V) | yes |
| a8 | A8 — external component lead/termination, uncoated, over 3050 m or vacuum (per pcb-toolkit; verify) | A8 — Bauteilanschluss außen, unbeschichtet, über 3050 m (prüfen) | mm (mm/V) | yes |
| per_volt | 1 for the ">500 V" row (mm per volt), else 0 | 1 für Zeile „>500 V" (mm je Volt), sonst 0 | — | yes |

## Rows
```csv
v_min,v_max,b1,b2,b3,b4,b5,a6,a7,a8,per_volt
0,15,0.05,0.1,0.1,0.075,0.075,0.13,0.13,0.13,0
16,30,0.05,0.1,0.1,0.075,0.075,0.13,0.25,0.25,0
31,50,0.1,0.64,0.64,0.3,0.13,0.13,0.4,0.8,0
51,100,0.1,0.64,1.5,0.3,0.13,0.13,0.5,1.0,0
101,150,0.2,0.64,3.2,0.8,0.4,0.4,0.8,1.6,0
151,170,0.2,1.25,3.2,0.8,0.4,0.4,0.8,1.6,0
171,250,0.2,1.25,6.4,0.8,0.4,0.4,0.8,1.6,0
251,300,0.2,1.25,12.5,0.8,0.4,0.4,0.8,1.6,0
301,500,0.25,2.5,12.5,1.6,0.8,0.8,1.5,3.0,0
500,,0.0025,0.005,0.025,0.00305,0.00305,0.00305,0.00305,0.0061,1
```

## Sources
1. S. V. Chavez, Siemens EDA blog, "PCB high voltage spacing: What every engineer should know" (2025-04-29), Figure 1: image of "Table 6-1 Electrical Conductor Spacing" from IPC-2221C (2023-December, p. 58, "used with permission of IPC International, Inc."), including table notes 1–5: https://blogs.sw.siemens.com/electronic-systems-design/2025/04/29/pcb-high-voltage-spacing-what-every-engineer-should-know/ (image: https://blogs.sw.siemens.com/wp-content/uploads/sites/65/2025/04/IPC-table.png) — read from the image; accessed 2026-11-02.
2. pcb-toolkit 0.1.5 (Rust crate), `src/spacing.rs`, constant `TABLE_MM` and `SLOPE_MM_PER_V` "IPC-2221C (2023) Table 6-1, p.58", plus `DeviceType` doc comments for the column meanings: https://docs.rs/crate/pcb-toolkit/latest/source/src/spacing.rs — accessed 2026-11-02.

## Cross-check
- Sources 1 and 2 agree on all 80 cells (9 voltage bands × 8 columns + per-volt row).
- Column meanings: source 1 (image) shows only the column groups "Conductors (creepage on same layer, internal or external)" and "Component lead (corona effect)" and note 4 (B5 = former A5); the per-column wording above comes from source 2 only → wording to be verified against IPC-2221C §6.3 before release (values are verified).
- Differences to IPC-2221B (see ipc2221b_conductor_spacing): B2 31–150 V and B3 31–50 V 0.6 → 0.64 mm; B4 changed (e.g. 0–30 V 0.05 → 0.075 mm, 31–100 V 0.13 → 0.3 mm, 101–300 V 0.4 → 0.8 mm, 301–500 V 0.8 → 1.6 mm); B5 (old A5) 0–30 V 0.13 → 0.075 mm; the old A6 values now appear in A7, the old A7 values (except 251–300 V: 0.8 → 0.4 mm) in A6; new column A8.

## Notes (en / de)
- en: Functional spacing for printed boards (IPC); not a safety-isolation table — use IEC 60664-1 / IEC 62368-1 for mains and SELV separation. Above 500 V: spacing = value(301–500 V) + (V − 500 V) × per-volt value (IPC-2221C para. 6.3, as applied in both sources).
- de: Funktionsabstände für Leiterplatten (IPC); keine Tabelle für Schutztrennung — dafür IEC 60664-1 / IEC 62368-1. Über 500 V: Abstand = Wert(301–500 V) + (V − 500 V) × Wert je Volt.

## Suggested finder / lookups
Same as ipc2221b_conductor_spacing; offer a revision switch (B / C) with C as default.
