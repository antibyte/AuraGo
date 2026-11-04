# Conductors group — summary


## Tables

| table_id | kind | rows × value cols | sources | status |
|---|---|---|---|---|
| cable_ampacity_cu_pvc | transcribed standard (IEC 60364-5-52 B.52.2/B.52.4/B.52.10 = DIN VDE 0298-4 Tab. 3) | 15 × 14 (1.5–240 mm²; A1, A2, B1, B2, C, E × 2/3 loaded; F from 25 mm²) | 6 | all verified; DIN variant noted (1.5 mm² A1/A2 2-loaded = 15.5 A) |
| cable_ampacity_cu_xlpe | transcribed standard (B.52.3/B.52.5/B.52.12) | 15 × 14 | 3 | 4 UNVERIFIED (2.5 mm² A1/B1) |
| cable_ampacity_al_pvc | transcribed standard (B.52.2/B.52.4/B.52.11) | 10 × 14 (16–240 mm²) | 3 | 3-loaded A1–C verified (50 cells); 88 UNVERIFIED (2-loaded, E, F) |
| cable_ampacity_al_xlpe | transcribed standard (B.52.3/B.52.5/B.52.13) | 10 × 14 | 4 (ABB + Prysmian Spain) | F 25–240 (18) and 14 B1 cells verified; 106 UNVERIFIED (A1, A2, B2, C, E, 6 B1 cells ±1 A) |
| cable_ambient_temp_factors | transcribed standard (IEC B.52.14 / DIN Tab. 17) | 16 × 3 (k_60, k_70, k_90; 10–85 °C) | 5 | 5 UNVERIFIED (k_60 10–25 °C, k_90 85 °C); all match the square-root formula |
| cable_grouping_factors | transcribed standard (IEC B.52.17 / DIN Tab. 21) | 7 arrangements × 14 n-columns | 4 | 9 UNVERIFIED (DIN-only cells) |
| cable_multicore_factors | transcribed standard (DIN Tab. 26) | 8 × 2 | 3 | 9 UNVERIFIED (k_air 19/40/61; all k_ground) |
| flex_cord_ampacity | transcribed standard (DIN VDE 0298-4 Tab. 11) | 21 × 6 (0.5–500 mm²) | 3 | 9 UNVERIFIED + 2 "extended" 0.5 mm² values |
| nec_ampacity_awg | transcribed standard (NEC 2023 Table 310.16) | 20 × 6 (18 AWG – 500 kcmil, Cu & Al, 60/75/90 °C) | 4 | all verified |
| iec60228_cross_sections | transcribed standard (IEC 60228) | 30 × 4 (0.5–2500 mm²; class 2 Cu/Al, class 5 Cu plain/coated R20) | 3 | 10 UNVERIFIED (class 2, 1400–2500 mm²) |
| ipc2221b_conductor_spacing | transcribed standard (IPC-2221B Table 6-1, B1–B4, A5–A7) | 10 × 7 | 4 | all verified; 1 disagreement resolved (B2 251–300 V = 1.25 mm) |
| ipc2221c_conductor_spacing | transcribed standard (IPC-2221C 2023 Table 6-1, B1–B5, A6–A8) | 10 × 8 | 2 | values verified; per-column wording from one source |
| pcb_copper_weight | formula-derived + typical | 7 × 7 (0.25–4 oz) | 6 | n/a |

Skipped as instructed: the IPC-2221 width × copper weight × ΔT grid and AWG diameter/area (ECalc computes both).

## Open problems

1. Aluminium ampacities: Al XLPE methods F and B1 are now confirmed by Prysmian Spain datasheets (F exact at 30 °C;
   B1 via the stated 40 °C factor 0.91, 6 cells only within ±1 A). Still single-source (ABB): Al PVC 2-loaded/E/F and
   Al XLPE A1/A2/B2/C/E. The Schneider EIG wiki has no further Al tables (only B.52.4 and the simplified C.52.1);
   Helukabel/Eupen/DIN preview URLs returned 404. Needs a licensed IEC 60364-5-52 / DIN VDE 0298-4 Tab. 3/4 or a maker
   table. Either ship flagged "unverified" or ship only the verified columns.
2. Cu XLPE 2.5 mm² A1/B1: ABB only (the BS 7671 reproduction omits that row); a Prysmian Cu datasheet supports
   b1_3 = 28 only within ±1 A — still UNVERIFIED.
3. DIN vs IEC vs BS: DIN VDE 0298-4 (2013) 1.5 mm² A1/A2 2-loaded = 15.5 A vs IEC 14.5/14 A; BS 7671 2.5 mm² Method A
   2-cable = 20 A vs IEC/DIN 19.5 A (chose 19.5). DIN VDE 0298-4:2023-06 (current edition) not checked.
4. IPC-2221C column wording (B4, A6–A8) from a third-party code library; verify against IPC-2221C §6.3.
5. NEC 2026 first-draft FR-8257 adds 10 A at 60 °C for 16 AWG Cu and 14 AWG CCA; final text not checked, table stays NEC
   2023.
6. Flexible cords: column D (rubber cables ≥ 0.6/1 kV) only from Lapp (1.5–4 mm²); H07RN-F table (DIN Tab. 13)
   single-source, left out.
7. Subsets chosen: Cu 1.5–240 mm², Al 16–240 mm²; methods D1/D2 and G not included; tray tables B.52.20/B.52.21 not
   included (sources exist: Schneider EIG Fig. G17, ABB EIH Tables 6–7, ABB DE Tab. 8, Lapp 12-7/12-8).

## Finder (a) — cable cross-section

Inputs: Ib [A]; optional device rating In [A]; material (Cu | Al); insulation (PVC 70 °C | XLPE/EPR 90 °C); method
(A1 | A2 | B1 | B2 | C | E | F); circuit: single-phase/DC → 2 loaded, three-phase → 3 loaded; ambient air temperature
[°C]; grouping: arrangement + number of circuits n; optional loaded cores N > 3 (multi-core cables ≤ 10 mm²); optional
rule set (IEC | DIN).

Columns used:
1. `cable_ampacity_<cu|al>_<pvc|xlpe>`: `size_mm2` + exactly one value column `<method>_<n>` (`a1_2` … `e_3`; F: `f_2` =
   2 single-core touching flat, `f_3` = 3 single-core trefoil; F only from 25 mm²).
2. `cable_ambient_temp_factors`: `k_70` (PVC) or `k_90` (XLPE); first `ambient_c` ≥ actual ambient, or interpolate;
   empty cell = insulation not permitted at that ambient.
3. `cable_grouping_factors`: row by `arrangement_id` (defaults A1/A2/B1/B2 → `bunched`, C → `wall_touching`, E/F →
   `perforated_tray`), column `n<k>` with the smallest listed k ≥ n.
4. `cable_multicore_factors.k_air`: first row with `loaded_cores` ≥ N, only when N > 3 and size ≤ 10 mm².
5. DIN rule set: at 1.5 mm² Cu PVC, `a1_2` and `a2_2` become 15.5 A.

Algorithm: for sizes ascending (non-empty cells only): Iz = I_table × k_temp × k_group × k_cores; return the first size
with Iz ≥ max(Ib, In). Show I_table, factors, Iz and the guide-value note; badge "unverified" if the chosen cell is
UNVERIFIED; "exceeds table range" if none qualifies.

## Finder (b) — PCB spacing by voltage (IPC-2221 Table 6-1)

Inputs: V (DC or AC peak; AC RMS → V_peak = V_rms × √2 for a sine, say so); revision C (default) or B; column (C: b1,
b2, b3, b4, b5, a6, a7, a8; B: b1, b2, b3, b4, a5, a6, a7).
Columns used: `v_min`, `v_max`, the chosen column, and `per_volt` (from `ipc2221c_conductor_spacing` or
`ipc2221b_conductor_spacing`).
Algorithm: V ≤ 500 → row with `v_min` ≤ ceil(V) ≤ `v_max` (between bands → next band up); V > 500 → cell of the 301–500
row + (V − 500) × cell of the `per_volt` row. Output mm and mil (mil = mm ÷ 0.0254), with the note that IPC spacing is
functional spacing, not safety isolation (use IEC 60664-1 / IEC 62368-1 for that).
