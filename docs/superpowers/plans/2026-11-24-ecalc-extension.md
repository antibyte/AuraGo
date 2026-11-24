# ECalc content extension (`ecalc-ext`): design and execution plan

Date 2026-11-24 · Base: `main` @ e70911c1c · Binding rules: `internal/ecalc/AGENTS.md` and the `## ECalc` section of `ui/js/desktop/apps/AGENTS.md`. This document does not repeat those rules. It points to them.

---

# PART A — Design

## A.0 Findings that shape the design

- **Expression language** (`expr.go`): `sqrt ln log10 exp sin cos tan atan abs round floor ceil min max if sum par`, plus `pi` and `^`. There is no `acos` or `asin`, so φ from cos φ is written `atan(sqrt(1-pf^2)/pf)`. Lists only work inside `sum` and `par`, so no running product is possible. Names may not collide with a function or with `pi`, so no variable can be called `if`, `min`, `exp` and so on.
- **Solver and tests** (`calculators_test.go`):
  - Every alternative of every rule must reproduce its target from a vector set, within 1e-9.
  - Every solvable input set must reproduce the whole set.
  - The round trip solves from the defaults for every choice option and then from every set.
  - So a rule that rounds (`round()`) breaks the round trip unless it is forward-only. The precedent is `uart_baud_error`, `digital.yaml:130-160`: the rounded value is a *derived* quantity and there are no inverse rules.
- **Dimensions** (`units.yaml`, pinned by `TestUnitsCatalogDimensions`, `units_test.go:162-209`) have no VA, var, V/s, N·m, rpm, bytes or bit/s, and `ratio` has no ppm. A variable with an empty `units` list offers *every* unit of its dimension. Adding VA/var to `power` would therefore leak into all existing power dropdowns, so new dimensions are the clean solution. Each dimension needs a `desktop.ecalc_dim_<id>` key in all 16 `ui/lang/desktop/*.json` (checked by `ui/desktop_ecalc_test.go:160-177`).
- **Categories** live in `catalog/categories.yaml` and are pinned in two places:
  - the order string at `catalog_test.go:74`;
  - the count 13 at `localize_test.go:91`.

  The rail is data-driven (`ecalc-shell.js:119`), so it needs no code change.
- **Schematics** are optional; 47 of 79 calculators have one. There is one file per calculator category, listed in four places:
  - `ui/desktop_ecalc_schematics_test.go:21` (`ecalcSchematicCategories`, plus the "ten" texts);
  - the order list at `ui/desktop_ecalc_test.go:235`;
  - `ui/js/desktop/core/module-loader.js:31` (not bundled; `npm run check:ui` confirms);
  - the UI AGENTS entry ("29 classic scripts", "ten schematics files").

  Only the symbols in `ecalc-symbols.js` may be used: resistor capacitor polarized inductor diode led zener crystal fuse switch pot source acsource isource battery meter ground antenna npn pnp nmos pmos opamp transformer ic.
- **Pinout `to` shape** (`pinouts.go:35,551-571`): packages `to92`/`to220`, 2–5 pins left to right, drawn `front`. Two places assume TO-style ids:
  - `query_pinout.go:51,98` builds the package name with `strings.ToUpper(s.Package[:2])+"-"+s.Package[2:]`, which would turn `sot23` into "SO-T23";
  - `ecalc-pinout-draw.js:474 layoutTo` only knows TO bodies.
- **Pinned lists**:
  - outlines: `PinoutOutlines` (16; `pinouts_test.go:112`, `ecalc-pinout-draw.js:50 OUTLINES`, `scripts/test-ecalc-pinouts.mjs:26`, `pinoutOutlineNames` at `query_pinout.go:24`);
  - groups: `PinoutGroups` (`pinouts_test.go:109,182`, `ecalc-shell.js:20 PINOUT_GROUPS` checked by `ui/desktop_ecalc_pinouts_test.go:86-95`, the key list at `ui/desktop_ecalc_pinouts_test.go:17`, `ui/desktop_ecalc_pinouts_browser_test.go:228`);
  - packages: `pinouts_test.go:239` error text, `query_pinout_test.go:311-312`, `scripts/ecalc-pinout-fixtures.js:88-89`, `ui/desktop_ecalc_pinouts_test.go:261`.
- **Code kinds** are pinned at `codes_test.go:153,165,484`. The page picks a controller by kind (`ecalc-code-page.js:828 CONTROLLERS`). The agent schema takes its enum from `ecalc.CodeKindIDs()` (`native_tools_ecalc.go:65`), so the schema grows by one enum word; there is no schema-size test for `ecalc`.
- **Loading and file names**: `Load` fails the *whole* catalog on any error. Nothing ties a pinout file name to its group, so a group may span several files. This is what makes parallel pinout batches possible (see A.2 and A.3).
- **Calculator research records**: the AGENTS rules only require records in `documentation/ecalc-sources/` for tables, codes and pinouts. A calculator carries its `source` plus the vector `ref`s. The research notes in `C:\disposable\ecalc-ext\research\calc\` are inputs only and are not committed.

## A.1 Final item tables

### Wave 1 — 34 calculators (79 → 113; categories 10 → 12 calculator categories)

Dimension abbreviations:
- existing: V voltage, A current, Ω resistance, W power, J energy, # number, dB gain, F, H, Hz, s time, m length, K temperature, r ratio, ∠ angle, int integer;
- new (E1): VA `apparent_power`, var `reactive_power`, V/s `slew_rate`, N·m `torque`, rpm `rotational_speed`, bit `datasize`, bit/s `datarate`.

`in:f` means `input: false` (a computed-only rule target).

| # | id | cat | name en / de | vars (dim) | dof | rule targets (directions) | schematic | notes / workable form |
|---|---|---|---|---|---|---|---|---|
| 1 | power_triangle | power | Power triangle (P, Q, S, cos φ) / Leistungsdreieck (P, Q, S, cos φ) | p W, q var, s VA, pf r | 2 | s [sqrt(p²+q²), p/pf, q/sqrt(1−pf²)]; p [s·pf, sqrt(s²−q²), q·pf/sqrt(1−pf²)]; q [p·sqrt(1−pf²)/pf, sqrt(s²−p²), s·sqrt(1−pf²)]; pf [p/s, sqrt(1−(q/s)²)] | – | derived φ ∠ = atan(q/p). Magnitudes only; inductive/capacitive in doc. Check pf>1. |
| 2 | three_phase_power | power | Three-phase power / Drehstromleistung | v V (line-to-line), i A, pf r, p W | 3 | p, i, v, pf | – | derived s VA = √3·v·i, q var, v_ph = v/√3 |
| 3 | pf_correction | power | Power-factor correction capacitor / Blindleistungskompensation (Kondensator) | p W, pf1 r, pf2 r, v V, f Hz, c F | 5 | c, pf2, p | power | choice conn: `single_star` (k=1, per phase at line voltage) / `delta` (k=3); tan = sqrt(1−pf²)/pf; derived qc var; checks pf2≤pf1, pf>1 |
| 4 | energy_cost | power | Energy consumption and cost / Energieverbrauch und Kosten | p W, hd s (unit h), days #, price # (per kWh, any currency), e J (unit kWh), cost # | 4 | e, p, cost, price | – | currency = plain number; check hd > 24 h |
| 5 | led_strip_power | power | LED strip supply (WS2812) / LED-Streifen-Versorgung (WS2812) | n #, iled A, v V, rs Ω (both rails, whole strip), vmin V, i A | 5 | i, n, iled | – | choice feed: `one_end` (kf=1/2) / `both_ends` (kf=1/8) stands in for "injection"; derived p W, dv = kf·i·rs, vend = v−dv; check vend < vmin (vmin from the LED datasheet, not a rule of thumb) |
| 6 | supercap_backup | power | Supercapacitor hold-up time / Überbrückungszeit mit Superkondensator | c F, v1 V, v2 V, i A, p W, t s | 5 | t, c (both with `if(cp,…)`) | power | choice load: `current` (cp=0, ignore p) / `power` (cp=1, ignore i); derived usable e J; ESR ignored (doc); check v2≥v1 |
| 7 | inverting_buck_boost | power | Inverting buck-boost converter / Invertierender Hoch-Tiefsetzsteller | vin V, vout V (magnitude), iout A, f Hz, l H, dil A | 5 | dil, l, f, vin (= k·vout/(vout−k), k=dil·f·l) | power | mirror `buck_converter` (power.yaml:93); derived D, I_L = iout·(vin+vout)/vin, I_pk; DCM check |
| 8 | transformer_ratio | passive | Transformer turns ratio / Transformator-Übersetzung | np #, ns #, vp V, vs V, ip A, is A | 4 | vs, vp, ns, np, is, ip | passive (transformer) | ideal; derived n, impedance ratio n² |
| 9 | ntc_beta | sensors | NTC thermistor (B equation) / NTC-Thermistor (B-Gleichung) | r0 Ω, t0 K (25 °C), beta # ("B in kelvin"), t K, r Ω | 4 | r, t, beta, r0 | – | derived α = −beta/t² (tempco); no NTC symbol, so no schematic |
| 10 | ntc_divider | sensors | NTC voltage divider / NTC-Spannungsteiler | vcc V, rfix Ω, r0 Ω, t0 K, beta #, t K, vout V, rntc Ω in:f | 6 | rntc [B-eq, divider inverse], t, vout, rfix | sensors | choice pos: `low`/`high` (NTC to ground / to supply) via `if(low,…)`; derived self-heating P |
| 11 | rtd_pt100 | sensors | Pt100/Pt1000 resistance thermometer (IEC 60751) / Pt100/Pt1000-Widerstandsthermometer (IEC 60751) | t K, r Ω | 1 | r: Callendar–Van Dusen with `if(tc<0, C-term, 0)`, tc=t−273.15; t: quadratic inverse | – | choice element `pt100` (r0=100) / `pt1000` (r0=1000). **Workable form:** the inverse is exact only from 0 to 850 °C; vector *sets* only ≥0 °C (below 0 °C appears only in solve vectors with input t); checks t<0 °C ("T from R ignores the C term") and outside −200…850 °C |
| 12 | current_loop_4_20ma | sensors | 4–20 mA current loop / 4–20-mA-Stromschleife | i A, lo r, hi r, pv r, r Ω (250 Ω), v V | 4 | pv, i [from pv, v/r], v | sensors (isource, resistor) | scale ends in `ratio` (may be 0 or negative); checks from NAMUR NE 43 limits per research |
| 13 | shunt_current_sense | sensors | Shunt current measurement / Strommessung mit Shunt | i A, r Ω, gain #, v V, vout V | 3 | v [i·r, vout/gain], i, r, vout, gain | sensors | derived P = i²r |
| 14 | stepper_steps_per_mm | drives | Stepper motor steps per mm / Schrittmotor: Schritte pro mm | spr #, ms #, pitch m, teeth #, spmm # | 4 | spmm, pitch, ms (all with `if(belt,teeth,1)`) | – | choice drive `screw` (ignore teeth) / `belt`; **no inverse for teeth** (it is not used with a screw, which would break the round trip); derived travel per microstep |
| 15 | motor_torque_power | drives | Motor torque, speed and power / Motor: Drehmoment, Drehzahl, Leistung | tq N·m, n rpm, p W | 2 | p = 2π·n·tq, tq, n | – | n stored in rev/s (SI of the new dimension) |
| 16 | lc_lowpass | filters | LC low-pass, 2nd-order Butterworth / LC-Tiefpass 2. Ordnung (Butterworth) | r Ω, f Hz, l H, c F | 2 | l, c, f, r (each with 2–3 alternatives incl. the l–c pair) | filters | choice term `double` (kl=kc=√2) / `load_only` (kl=√2, kc=1/√2) |
| 17 | sallen_key_highpass | filters | Sallen-Key high-pass (unity gain) / Sallen-Key-Hochpass (Verstärkung 1) | c F (both), r1 Ω (feedback), r2 Ω (to ground), f0 Hz, q # | 3 | f0, q, r1, r2, c | filters | mirror `sallen_key_lowpass` (filters.yaml:122-147); derived r2 for Q=0.707 |
| 18 | mfb_bandpass | filters | MFB band-pass / MFB-Bandpass (Mehrfachgegenkopplung) | c F, r1, r2, r3 Ω, f0 Hz, q #, g # | 4 | f0, q, g (forward); r2, r1, r3, c (design) | filters | check 2q² ≤ g (no positive r3) |
| 19 | speaker_crossover | filters | 2-way speaker crossover, 2nd order / 2-Wege-Frequenzweiche 2. Ordnung | r Ω, f Hz, l H, c F | 2 | l, c, f, r | filters | choice `butterworth` (kl=√2/(2π), kc=1/(2√2π)) / `linkwitz_riley` (kl=1/π, kc=1/(4π)); same L and C for both sections |
| 20 | l_pad | filters | L-pad attenuator / L-Pad-Abschwächer | z Ω, a dB, rs Ω, rp Ω | 2 | rs, rp, a [from rs,z; from rp,z], z [3 alts incl. (rs+sqrt(rs²+4·rs·rp))/2] | filters | |
| 21 | opamp_differentiator | opamps | Op-amp differentiator / OPV-Differenzierer | r Ω, c F, f Hz, a #, r1 Ω (series) | 4 | a = 2πfRC, r, c, f | opamps | mirror `opamp_integrator` (opamps.yaml:124); derived τ, f_HF = 1/(2π·r1·c); check f > f_HF |
| 22 | opamp_gbw_bandwidth | opamps | Closed-loop bandwidth from GBW / Bandbreite aus dem GBW | gbw Hz, g #, f Hz | 2 | f, g, gbw | – | choice config `noninv` (inv=0) / `inv` (inv=1): noise gain = g+inv |
| 23 | slew_rate_fpbw | opamps | Slew rate and full-power bandwidth / Slew-Rate und Leistungsbandbreite | sr V/s (unit V/µs), vp V, f Hz | 2 | f, sr, vp | – | derived Vpp |
| 24 | thermal_noise | rf | Thermal (Johnson) noise / Thermisches Rauschen (Johnson) | r Ω, t K (290 K), b Hz, vn V | 3 | vn, r, t, b | – | k = 1.380649e-23 (exact SI); derived e_n as "voltage in 1 Hz" (V), kTB in W/dBm |
| 25 | l_match | rf | L-network matching / L-Anpassnetzwerk | rs Ω, rl Ω, f Hz, l H in:f, c F in:f | 3 | l, c (forward only, max/min inline) | rf (first rf schematic) | choice `lowpass`/`highpass`; derived q; check rs = rl |
| 26 | eirp | rf | EIRP and ERP / EIRP und ERP | p W (dBm), loss dB, g dB, eirp W (dBm) | 3 | eirp, p, g, loss | – | derived ERP = EIRP / 10^0.215 |
| 27 | coax_stub | rf | Coax stub / electrical length / Koax-Stichleitung / elektrische Länge | f Hz, vf r, deg ∠ (90°), l m | 3 | l, deg, f, vf | – | derived λ₀, λ in line |
| 28 | noise_figure_cascade | rf | Cascaded noise figure (Friis) / Gesamtrauschzahl (Friis) | nf1, g1, nf2, g2, nf3, nft (all dB) | 5 | nft, nf1, g1 (closed forms) | – | **Workable form:** a fixed 3-stage chain (lists cannot form products); nf3 = 0 dB removes stage 3; derived Te = 290·(F−1) |
| 29 | optocoupler_resistor | semiconductors | Optocoupler resistors (CTR) / Optokoppler-Widerstände (CTR) | vin V, vf V, iled A, r1 Ω, ctr r (%), vcc V, vcesat V, ic A in:f, rl Ω in:f | 6 | r1, iled, ic, rl (smallest pull-up for saturation) | semis (led + npn) | checks from the part datasheet (research) |
| 30 | mosfet_gate_drive | semiconductors | MOSFET gate drive / MOSFET-Gate-Ansteuerung | qg C, vdrv V, vpl V, rg Ω, t s, f Hz | 5 | t, rg, qg | semis (nmos) | constant-current plateau approximation (doc); derived ig, ipk, driver P = qg·vdrv·f |
| 31 | crystal_drift | timers | Crystal drift (ppm ↔ s/day) / Quarzabweichung (ppm ↔ s/Tag) | ppm r (unit ppm, units [ppm, pct, ""]), dd s, f Hz (32768) | 2 | dd, ppm | – | magnitude (time is positive); derived s/30 d, s/365 d, Δf |
| 32 | avr_timer | timers | AVR timer compare value (CTC) / AVR-Timer-Vergleichswert (CTC) | fclk Hz, f Hz | 2 | **none** (forward-only) | – | **Workable form** like `uart_baud_error`: choices prescaler `auto`/1/8/64/256/1024 (const n, 0 = auto), mode `toggle` (k=2) / `interrupt` (k=1), width `bits8` (top 255) / `bits16` (top 65535). Derived: nsel = `if(n,n,if(fclk/(k*f)<=top+1,1,if(…8…,8,…1024)))`, ocr int = `max(round(fclk/(k*nsel*f))-1,0)`, f_act, err %. Check ocr > top. ≥12 sets (2 per prescaler option). |
| 33 | rc_debounce | digital | RC debounce / RC-Entprellung | r Ω, c F, vdd V, vth V, t s | 4 | t, r, c, vth | digital (switch, R, C) | choice edge `rise` (ln(vdd/(vdd−vth))) / `fall` (ln(vdd/vth)) |
| 34 | data_transfer_time | digital | Data transfer time / Übertragungsdauer | size bit, rate bit/s, t s | 2 | t, size, rate | – | choice frame `raw` (1) / `uart_8n1` (10/8) / `uart_11bit` (8N2, 8E1: 11/8) |

New categories (16-locale names written in E2):
- `sensors` — "Sensors" / "Sensoren";
- `drives` — "Drives & motors" / "Antriebe & Motoren".

Rail order: appended after `wiring`, before `tables`.

### Wave 2 — components (pinouts 47 → 71, codes 5 → 6)

All 18 DIP parts use `dip` (ne555 pattern, ics.yaml:4). A name is a proper name and identical in all 16 locales; generic examples items get translated "examples" names like `to92`.

| batch / file | id | name (all locales) | pins | shape |
|---|---|---|---|---|
| P1 `ics_logic.yaml` | sn74hc00, sn74hc04, sn74hc14 | 74HC00 / 74HC04 / 74HC14 (DIP-14) | 14 | dip |
| | sn74hc138, sn74hc165 | 74HC138 / 74HC165 (DIP-16) | 16 | dip |
| | sn74hc245 | 74HC245 (DIP-20) | 20 | dip |
| | cd4017, cd4051 | CD4017B / CD4051B (DIP-16) | 16 | dip |
| P2 `ics_analog.yaml` | lm393, lm386, attiny85 | LM393 / LM386 / ATtiny85 (DIP-8) | 8 | dip |
| | lm339 | LM339 (DIP-14) | 14 | dip |
| | max232, l293d, mcp3008 | MAX232 / L293D / MCP3008 (DIP-16) | 16 | dip |
| | uln2803 | ULN2803A (DIP-18) | 18 | dip |
| | mcp23017 | MCP23017 (DIP-28) | 28 | dip |
| P3 `ics.yaml` (existing) | pc817 | PC817 (DIP-4) | 4 | dip |
| | irf540n | IRF540N / IRLZ44N (TO-220) — one item, identical G-D-S | 3 | to/to220 |
| | tip120, bt136, lm1117 | TIP120 / BT136 / LM1117 (TO-220) | 3 | to/to220 |
| | to92 (edit) | +5 variants: bc337, bc557, q2n3906, bs170, ds18b20 | 3 | to/to92 |
| | sot23 (NEW package) | SOT-23: examples / SOT-23: Beispiele — variants e.g. bc847, q2n7002 (research picks 2–4) | 3 | to/**sot23** |
| | sot223 (NEW package) | SOT-223: examples / SOT-223: Beispiele — variants lm1117 (ADJ/GND, OUT, IN, tab OUT), bcp56 | 4 | to/**sot223** |
| E5 `codes/smd_inductor.yaml` | smd_inductor_code | SMD inductor marking (16 locales) | – | new kind `smd_inductor` |

**Flags:**
- SOT-23 has no single "standard" pinout, so it is an examples item with variants, like `to92`.
- If the research cannot find a current TI document for LM1117 in TO-220, P3 drops that package and keeps LM1117 only as the `sot223` variant.

### Wave 3 — connectors (pinouts 71 → 119, groups 6 → 7)

Group `vehicle_mains` — "Vehicle and mains" / "Fahrzeug und Netz".

| batch / file (group) | id | shape / outline | flag |
|---|---|---|---|
| Q1 `data_av.yaml` (data) | dvi_i | rows/**dvi** (NEW) | 29 contacts; C5 in row 2 after a gap, blade drawn by the outline; DVI-D and single link in notes |
| | scart | rows/**scart** (NEW) | 2 staggered rows (offset 0.5) |
| | ps2, s_video | circular (mini-DIN 6 / 4) | key angle, plug and socket views |
| | dsub25 | rows/dsub | variants `parallel` (IEEE 1284) / `rs232` (same numbers) |
| | rj11 | rows/**rj11** (NEW, see below) | variants `rj11` (6P4C, outer positions nc) / `rj12` (6P6C) |
| | can_db9 | rows/dsub | |
| | sd_card, microsd_card | rows/**card** (NEW) | two items (9 vs 8 pins); variants `sd` / `spi` each |
| Q2 `pc_internal.yaml` (pc_power) | atx12v_4 | rows/minifit | |
| | pcie_12vhpwr | rows/minifit + sideband row with gaps | approximation: sideband contacts drawn at power size; 12V-2x6 in notes |
| | front_panel, usb2_header, hd_audio_header | rows/rect 2×5 with gap | |
| | usb3_header | rows/rect 2×10, key | |
| | rgb_header_12v, argb_header_5v | rows/rect 1×4 (ARGB with nc) | two items |
| Q3 `vehicle_mains.yaml` (**vehicle_mains**) | trailer_7, trailer_13 | circular (ISO 1724 / ISO 11446) | |
| | cee_3p, cee_5p | circular (IEC 60309, 2P+E / 3P+N+E) | 16 A and 32 A share the arrangement |
| | type2 | circular (IEC 62196-2) | flat top of the housing not drawn |
| | iso10487_a, iso10487_b | rows/rect 2×4 | two housings, so two items |
| Q4 `audio_pro.yaml` (audio) | jack_635 | jack (T,R,S) | variants `balanced` / `stereo` / `insert`; TS mono in notes |
| | rca | circular, 2 positions (centre r=0, shell contact at the rim) | approximation; `jack` would draw a phone plug |
| | xlr5_dmx, speakon_nl4 | circular | |
| | eurorack_power | rows/rect 2×8, key | 10-pin header in notes |
| Q5 `debug_modules.yaml` (debug) | esp01, nrf24l01 | rows/rect 2×4 | |
| | hc05 | rows/rect 1×6 | |
| | hc_sr04, dht22 | rows/rect 1×4 | |
| | i2c_display | rows/rect 1×4 | variants `gnd_first` / `vcc_first` |
| | jst_xh_balance | rows/jst 1×7 | one item drawn for 6S; "an n-cell pack uses pins 1…n+1 from the black lead" |
| | mikrobus | board, connector none (left 8 / right 8) | |
| | pmod | rows/rect 2×6 | |
| Q6 `boards_arduino.yaml` (boards) | arduino_mega | board + `inner` 2×18 | drawing size risk |
| | arduino_pro_mini, arduino_micro | board | |
| | arduino_leonardo | board (Uno form) | Leonardo and Micro are two items |
| | microbit | board, bottom edge 25 pads | big and small pads drawn alike |
| Q7 `boards_mcu.yaml` (boards) | wemos_d1_mini, esp32_c3_devkitm1, esp32_s3_devkitc1, esp32_cam, blue_pill | board | |

Shape verification:
- Everything not marked NEW fits the existing `rows` outlines, `circular`, `dip`, `to`, `jack` and `board`.
- **The one deviation from the approved scope: RJ11/RJ12 needs a 4th outline `rj11`.** It shares the `rj45` builder. The reason is text, not drawing: `pinoutConnector` would otherwise tell the agent "RJ45 connector, 6 pins".
- RCA, 12VHPWR and Type 2 are approximations, as flagged in the table.

## A.2 Engine and UI changes (exact files)

**E1 — units (Wave 1)**
- `internal/ecalc/catalog/units.yaml`: append after `integer`:
  - `apparent_power` {VA, prefixes m "" k M, positive false};
  - `reactive_power` {var, m "" k M, positive false};
  - `slew_rate` {V/s, "" k M, positive true, units [{id: Vus, symbol: V/µs, aliases [V/us], factor 1e6}]};
  - `torque` {N·m, aliases [Nm], m "", positive true, units N·cm 0.01, kgf·cm 0.0980665, ozf·in 0.00706155181422604375 (= 0.028349523125 × 9.80665 × 0.0254)};
  - `rotational_speed` {1/s, "", positive true, units [{id: rpm, symbol: rpm, aliases [U/min], factor 0.016666666666666666}]};
  - `datasize` {bit, "" k M G, positive, units B 8, kB 8e3, MB 8e6, GB 8e9, KiB 8192, MiB 8388608, GiB 8589934592};
  - `datarate` {bit/s, aliases [bps], "" k M G, positive, units B/s 8, kB/s 8e3, MB/s 8e6};
  - add `{id: ppm, symbol: ppm, factor: 1e-6}` to `ratio`.
- `internal/ecalc/units_test.go:164-188`: extend `want`.
- `internal/ecalc/testdata/vectors/core.json`: a few parse/format/round-trip unit vectors per new dimension (`2.5 kVA`, `1 V/µs`, `3000 rpm`, `1 MiB`, `12 kgf·cm`, `50 ppm`). JS parity then runs through `npm run test:ecalc`.
- `ui/lang/desktop/*.json` (16): `desktop.ecalc_dim_{apparent_power,reactive_power,slew_rate,torque,rotational_speed,datasize,datarate}`.
- Translation-key lists: `ui/desktop_ecalc_i18n_test.go:25-32` and `ui/desktop_ecalc_test.go:139-143`.

**E2 — categories and schematic files (Wave 1)**
- `catalog/categories.yaml`: `sensors` and `drives` after `wiring` (16 locales each).
- Pinned category tests: `internal/ecalc/catalog_test.go:74` order string; `internal/ecalc/localize_test.go:89-91` count 13 → 15.
- New `ui/js/desktop/apps/ecalc/ecalc-schematics-{drives,sensors}.js`, each exactly `window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {});`.
- Register them in load order (basics, digital, **drives**, filters, opamps, passive, power, rf, semiconductors, **sensors**, timers, wiring):
  - `ui/js/desktop/core/module-loader.js:31`;
  - `ui/desktop_ecalc_test.go:235`;
  - `ui/desktop_ecalc_schematics_test.go:21` plus the "ten" comments and the message at line 226.
- Do **not** create the category YAML files here; C2 creates them with content.

**E4 — SOT packages (Wave 2)**
- `internal/ecalc/pinouts.go`:
  - `pinoutPackages` → to92, to220, sot23, sot223;
  - pin count per package (sot23 exactly 3, sot223 exactly 4; TO stays 2–5);
  - `Drawn = "top"` for SOT;
  - doc comment on the pin order: sot23 = [lower left, lower right, top centre], sot223 = [1, 2, 3 bottom left to right, tab top].
- `internal/ecalc/query_pinout.go`: a `pinoutPackageNames` map (TO-92, TO-220, SOT-23, SOT-223) used in `pinoutConnector`/`pinoutView`, plus a SOT view sentence ("SOT-23 seen from above, marking side, pin 1 at the lower left …").
- Tests: `pinouts_test.go:239-241` (message plus new count cases); `query_pinout_test.go:311` (two SOT rows).
- `ui/js/desktop/apps/ecalc/ecalc-pinout-draw.js` `layoutTo`, a SOT branch drawn in the existing materials:
  - `epoxy` body, `tin` gull-wing leads as filled paths;
  - pads as contacts, pin-1 dot as a `recess` layer;
  - SOT-223 tab in `tin`;
  - staggered names below and above;
  - header comment updated.
- Fixtures: `scripts/ecalc-pinout-fixtures.js` `fx_pin_sot23`, `fx_pin_sot223` (plus the header list).
- `ui/desktop_ecalc_pinouts_test.go:261`: add `"package: 'sot23'"` and `"package: 'sot223'"`.

**E5 — SMD inductor code kind (Wave 2)**
- New `internal/ecalc/codes_smd_inductor.go`, registered in `init`. Rules:
  - 3-digit µH code (`101` = 100 µH, `100` = 10 µH);
  - R and N decimal markers (`4R7` = 4.7 µH, `R47`, `47N` = 47 nH, `4N7`);
  - optional tolerance letter J/K/M.
- Dimension `inductance`; letters and tolerances live in data.
- `codes.go`: kind list and doc. `codes_test.go:153,165,484`: six kinds.
- `query_codes.go`: if it lists kinds. `testdata/vectors/codes.json`: decode, encode and error cases.
- `ui/js/desktop/apps/ecalc/ecalc-codes.js`: mirror.
- `ecalc-code-page.js:828`: an `smd_inductor` controller, reusing the SMD controller's `chipSVG` drawing with the `choke` material.
- New result fields get `desktop.ecalc_code_field_<id>` keys in 16 `ui/lang` files and the page's key list.
- Content in the same task: `catalog/codes/smd_inductor.yaml`, `documentation/ecalc-sources/codes/smd_inductor.md`, and a row in `codes/SUMMARY.md`.
- `scripts/test-ecalc-codes.mjs`: only if it lists kinds.

**E6a — group `vehicle_mains` (Wave 3)**
- `pinouts.go:22`; `pinouts_test.go:109,182`; `ecalc-shell.js:20`.
- `ui/lang/desktop/*.json` (16): `desktop.ecalc_group_vehicle_mains`.
- `ui/desktop_ecalc_pinouts_test.go:17` and its "six sections" comment at line 53; `ui/desktop_ecalc_pinouts_browser_test.go:228`.

**E6b — outlines `card`, `dvi`, `rj11`, `scart` (Wave 3)**
- `pinouts.go:25` (sorted, 20); `pinouts_test.go:112` (16 → 20); `query_pinout.go:24` names.
- `ecalc-pinout-draw.js`:
  - `OUTLINES`, `ROW_STYLE`, `SHELLS` builders: dvi = steel D shell, wide side up, `pbt` insulator, blade layer; scart = `pbt` with one cut corner; card = `pbt` body with the corner notch and `gold` contacts; `rj11: SHELLS.rj45`;
  - header "sixteen" → twenty;
  - no colour literals.
- Fixtures: `scripts/ecalc-pinout-fixtures.js`, one fixture per new outline.
- `scripts/test-ecalc-pinouts.mjs:26` (20).
- The orientation of each new outline is fixed by the research records for dvi/scart/sd and written into the AGENTS orientation list.

**Schematics (Wave 1 content):** the file of the calculator's category. At most 32 units wide, existing symbols only, chips clear of wires. The planned schematics are marked in the table above (17).

**Agent tool and manuals:**
- `internal/agent/native_tools_ecalc.go` needs no change (enum is automatic).
- `prompts/tools_manuals/ecalc.md`: add an `smd_inductor` row to the code-kind table (line 126). Nothing else lists categories or groups.

**Snapshot:** `go test ./internal/ecalc -run TestCatalogSnapshot -update -count=1`, only in each wave's closing task. Bundles: none (ECalc scripts load through `module-loader.js`); confirm with `npm run check:ui`.

**Docs:**
- `documentation/manuals/{en,de}/32-ecalc.md`: counts (113 calculators in 12 categories, 6 codes, 119 pinouts in 7 groups), Sensors and Drives sections, new rows per section, SMD inductor, the vehicle group.
- `internal/ecalc/AGENTS.md`:
  - dimension list;
  - code kinds (six, file map);
  - groups;
  - `to` packages and SOT order;
  - outline list and orientations;
  - pinout files `catalog/pinouts/<group>[_<part>].yaml`.
- UI AGENTS `## ECalc`: 31 scripts, twelve schematic files in order, CONTROLLERS, pinout groups, twenty outlines.
- `documentation/ecalc-sources/pinouts/SUMMARY.md` (counts, groups, weakest points).

**Tests that pin counts or lists:**
- `TestUnitsCatalogDimensions`, `TestDefaultCatalogLoads` (catalog_test.go:74), the localize test count (localize_test.go:91);
- `ecalcSchematicCategories` / `TestECalcSchematicFilesAndNewSymbols`, `TestECalcIsWiredIntoTheDesktopCore`;
- i18n key lists (desktop_ecalc_i18n_test.go, desktop_ecalc_test.go:139);
- `pinouts_test.go:109,112,182,239`, `query_pinout_test.go:311`;
- `TestECalcPinoutPagesAreWired` (PINOUT_GROUPS), `TestECalcPinoutDrawIsADomFreeStringBuilder` (OUTLINES), `TestECalcPinoutFixtureCoversEveryShape`, `test-ecalc-pinouts.mjs:26`, `desktop_ecalc_pinouts_browser_test.go:228`;
- `codes_test.go:153,165,484`.

## A.3 Per-item content checklist (from the two AGENTS files)

**Every item:**
- [ ] id `^[a-z][a-z0-9_]*$`, ≤ 63 characters, unique across kinds. Variable, choice and option ids `^[a-z][a-z0-9_]{0,31}$`; no function names or `pi`.
- [ ] `name` in all 16 `Locales`; `doc` in en + de; every expression and text quoted; micro sign U+00B5 and omega U+03A9. The Write tool decodes `\u` escapes, so write the characters literally.
- [ ] `source` names a standard by number or a well-known work at book or chapter level, or the maker's datasheet. Never an invented page, URL or application-note number.
- [ ] Tags: at least one, English plus German search words (repo practice).

**Calculator:**
- [ ] vars in order. Exactly `dof` input variables carry a default, and the defaults form a solvable set. Every input belongs to a set. A non-input is a rule target.
- [ ] One explicit rule per direction (a list gives alternatives, first applicable wins); no symbolic algebra; checks, derived values and the formula declared.
- [ ] Variable/derived names and check warnings in en + de; a `units` whitelist where the default list would be odd (ppm, kWh, V/µs, rpm).
- [ ] `testdata/vectors/calculators/<id>.json` with at least 2 hand-checked `sets` per rule direction and 2 per option of every choice, each with a `ref` showing the arithmetic. Expected values come from exact arithmetic, a definition or a published constant, never from the solver. Plus `solve` vectors for errors and warnings.
- [ ] The round trip passes for every option; `TestCalculateEveryCalculatorFromItsDefaults` passes.
- [ ] Schematic (if any): the category's file, ≤ 32 units, existing symbols only, anchors name variables, chips clear of wires.

**Pinout:**
- [ ] Pins come from the maker's document (own part) or a free official specification, otherwise two independent sources. An unverified detail stays out. `wire` only where a source names the colour.
- [ ] `rows`/`positions`/board lists written in the stated view, against the outline orientation; the other view is the mirror image, never a second list; `views` only for rows and circular.
- [ ] Pin number 1–8 characters (unique), name 1–24, function en + de, type from the six.
- [ ] `testdata/vectors/pinouts/<id>.json`: exact `counts` per variant, at least 3 pins (all if fewer) checked by number, name and type, each with a `ref`.
- [ ] `documentation/ecalc-sources/pinouts/<group>/<id>.md` in the ne555.md structure: Kind, Pins, View with its basis, Sources with URLs, Cross-check, Choices. A changed value updates record, YAML and vector together.

**Code:**
- [ ] Tables live in data. A single-source or UNVERIFIED symbol goes into `not_cross_checked`. Go, `ecalc-codes.js` and `codes.json` change together.

## A.4 Risks and out of scope

**Risks:**
1. **`Load` is all-or-nothing.** One broken YAML from a parallel batch reds everyone's tests. Mitigation:
   - batches own disjoint files and add one item at a time (vectors + YAML + schematic together);
   - they run `go test` with `-skip TestCatalogSnapshot`;
   - on a failure in a file they do not own, wait and retry; never touch it;
   - at most three content agents at once.
2. **Merge conflicts at landing.** `ui/lang/desktop/*.json` and the AGENTS files are hot in `main` (other sessions). Rebase `ecalc-ext` on `main` before merging; resolve JSON key order by hand.
3. **New dimensions.** JS parsing of symbols with `/` and `·` is covered by the core.json vectors. Rail length: 15 categories plus 2 fixed rows in the 640×480 minimum window must scroll; check in the Wave 1 smoke screenshot.
4. **Pinout drawings.**
   - Mega, Pro Mini and micro:bit boards are large, and the name-overlap check in `TestDesktopECalcPinoutRealBrowser` may fire.
   - New shells must stay 2 units clear of contacts (`test-ecalc-pinouts.mjs:121-142`).
   - Skin legibility: `ec-shell-edge` ≥ 3.0.
5. **Safety content.** CEE, Type 2 and trailer sockets carry mains or vehicle power. Docs and notes must say so; a wrong pin can injure, not just destroy.
6. **Research gaps.** An item whose record is missing or UNVERIFIED is dropped and reported, never guessed.
7. **Two hard calculators.**
   - `rtd_pt100`: the inverse is not exact below 0 °C.
   - `avr_timer`: 12 or more vector sets.

**Deliberately out of scope:**
- new expression functions (acos, atan2) or list products;
- new shape types; SOT-23-5/6 and SOT-89;
- colour-dot inductor codes; new tables;
- rail or design changes;
- schematics for calculators marked "–";
- push and deploy.

---

# PART B — Execution plan

## B.0 Setup (orchestrator, serial)

1. The worktree exists (created 2026-11-24 from `main` @ e70911c1c, the repo's usual place for worktrees):
   ```
   git -C C:/Users/Andi/Documents/repo/AuraGo worktree add -b ecalc-ext .worktrees/ecalc-ext main
   ```
   All paths below are relative to `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/ecalc-ext`.
2. Copy a real `node_modules` into the worktree (a junction breaks a vendor check). Use the usual GOCACHE/GOTMPDIR on D:.
3. Baseline, and record any pre-existing failures in the task log:
   ```
   go test ./internal/ecalc -count=1
   npm run test:ecalc
   go test ./ui -count=1 -run '(?i)^TestECalc'
   ```
4. Check that the inputs exist: `C:\disposable\ecalc-ext\research\{calc,parts,connectors}\<id>.md` for every item of the wave. List the missing ones before starting the wave.

**Rules for every task:**
- Commit only your own paths (`git add <paths>; git commit -m ...`). If `index.lock` exists, wait and retry; never delete it.
- `gofmt` your Go files.
- Before editing a function, run GitNexus `impact` from the main checkout: the worktree cannot be indexed, and the symbols are unchanged at the branch base. See the open question about `detect_changes`.
- Expected test result: `ok  aurago/...` with no FAIL. Count the PASS lines of `-v -run` as evidence where named.

## Wave 1 — calculators

**W1-E1 Units and dimensions** · serial, first
- Owns: `internal/ecalc/catalog/units.yaml`, `internal/ecalc/units_test.go`, `internal/ecalc/testdata/vectors/core.json`, `ui/lang/desktop/*.json` (16), `ui/desktop_ecalc_i18n_test.go`, `ui/desktop_ecalc_test.go`.
- Input: A.2 E1.
- Steps:
  1. Add the 7 dimensions and ppm; extend `want` in `units_test.go`.
  2. Add unit vectors to core.json, modelled on the existing `K/W` and `mm2` cases.
  3. Add 7 keys × 16 languages; extend both key lists.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestUnits|TestDefaultCatalogLoads|TestCatalogSnapshot)'
  ```
  The snapshot will differ because the units are part of the catalog JSON. Regenerate it here with `-update`, review that the diff only touches units, and add `internal/ecalc/testdata/catalog.en.json` to this task's paths. Then:
  ```
  npm run test:ecalc
  go test ./ui -count=1 -run '(?i)^TestECalc'
  ```
  All pass.

**W1-E2 Categories and schematic files** · serial, after E1 (shares `ui/desktop_ecalc_test.go`)
- Owns: `internal/ecalc/catalog/categories.yaml`, `internal/ecalc/catalog_test.go`, `internal/ecalc/localize_test.go`, `ui/js/desktop/apps/ecalc/ecalc-schematics-{drives,sensors}.js`, `ui/js/desktop/core/module-loader.js`, `ui/desktop_ecalc_test.go` (line 235), `ui/desktop_ecalc_schematics_test.go`, snapshot.
- Steps: as in A.2 E2.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -run TestCatalogSnapshot -update
  go test ./internal/ecalc -count=1
  go test ./ui -count=1 -run '(?i)^TestECalc(IsWired|Schematic)'
  npm run check:ui
  ```
  All pass.

**W1-C1 … C5 Content** · after E2. Run C1 ∥ C2 ∥ C3 first, then C4 ∥ C5.

Each batch owns its category YAML(s), the matching `ecalc-schematics-<cat>.js`, and `testdata/vectors/calculators/<id>.json` for its own ids.

| task | owns | items |
|---|---|---|
| C1 | `calculators/power.yaml`, `calculators/passive.yaml`, `ecalc-schematics-power.js`, `ecalc-schematics-passive.js` | #1–8 |
| C2 | new `calculators/sensors.yaml`, new `calculators/drives.yaml`, `ecalc-schematics-sensors.js` (drives stays `{}`) | #9–15 |
| C3 | `filters.yaml`, `opamps.yaml`, their two schematic files | #16–23 |
| C4 | `rf.yaml`, `semiconductors.yaml`, their two schematic files | #24–30 |
| C5 | `timers.yaml`, `digital.yaml`, `ecalc-schematics-digital.js` | #31–34 |

- Input: research `calc/<id>.md` and the A.1 row.
- Steps per item:
  1. Write the vectors (sets for every direction and option, with refs).
  2. Write the YAML entry (append at the end of the file, keep the file's header comment).
  3. Write the schematic stub if planned and set `schematic: <id>`.
  4. Run the tests; commit after every 2–3 green items.
- Verify (expected `ok`):
  ```
  go test ./internal/ecalc -count=1 -skip TestCatalogSnapshot
  go test ./internal/ecalc -count=1 -v -run '(?i)(TestCalculatorVectors|TestRoundTripAllCalculators)/<id>'
  go test ./ui -count=1 -run '(?i)^TestECalcSchematicsMatchTheCatalog$'
  ```

**W1-Z Closing** · serial
1. Snapshot: `go test ./internal/ecalc -run TestCatalogSnapshot -update -count=1`. Review the diff: only new items, categories and units.
2. Checks:
   ```
   go test ./internal/ecalc -count=1
   go vet ./internal/ecalc/
   gofmt -l internal/ecalc                 # empty
   npm run test:ecalc
   go test ./ui -count=1 -run '(?i)^TestECalc'
   go test ./internal/tools ./internal/agent ./internal/server -count=1 -run '(?i)ecalc'
   npm run check:ui
   ```
3. One browser smoke:
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^TestDesktopECalcBrowser$'
   ```
   It checks the chips of every schematic. Chrome must be started from a binary outside the repo (`D:/tmp/go-build`), sandbox off. Use `AURAGO_BROWSER_ARTIFACT_DIR` to look at the rail.
4. One combined review of `git diff main...HEAD` for this wave: Critical/Important only, focused on honesty of sources and refs, physics of the rules, and schematic anchors. Fix, then re-run step 2 only.
5. Docs (one docs agent): manual en/de (calculators, Sensors and Drives sections, counts); `internal/ecalc/AGENTS.md` dimension list; UI AGENTS (31 scripts, twelve schematic files).
6. Commits: `feat(ecalc): seven dimensions and ppm` (E1), `feat(ecalc): sensors and drives categories` (E2), one `feat(ecalc): <category> calculators` per batch, `chore(ecalc): regenerate catalog snapshot`, `docs(ecalc): wave 1 calculators`.

## Wave 2 — components

**W2-E4 SOT packages** ∥ **W2-E5 SMD inductor kind**
- They own disjoint files (A.2 E4 vs E5); the AGENTS files are left to W2-Z.
- Before editing, run GitNexus impact on `pinoutConnector`, `pinoutView`, the shape parser in `pinouts.go`, `layoutTo` (E4), and `CONTROLLERS`/`smdController` and the code kind registry (E5).
- E4 verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestPinout|TestQueryPinout)'
  npm run test:ecalc
  go test ./ui -count=1 -run '(?i)^TestECalcPinout'
  ```
  All pass. The geometry script draws `fx_pin_sot23` and `fx_pin_sot223`.
- E5 input: `research/parts/smd_inductor.md`. Verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestCode|TestQuery(Decode|Encode))'
  npm run test:ecalc
  go test ./ui -count=1 -run '(?i)^TestECalcCode'
  go test ./internal/agent ./internal/tools -count=1 -run '(?i)ecalc'
  ```
  All pass.

**W2-P1 ∥ P2 ∥ P3 Content** · after E4 and E5

| task | owns |
|---|---|
| P1 | new `catalog/pinouts/ics_logic.yaml`, its vectors and records |
| P2 | new `catalog/pinouts/ics_analog.yaml`, its vectors and records |
| P3 | `catalog/pinouts/ics.yaml`, `vectors/pinouts/to92.json`, `documentation/ecalc-sources/pinouts/ics/to92.md`, plus new vectors and records for its items |

- Input: `research/parts/<id>.md`.
- Steps per item: record, then YAML, then vector; test; commit every 3 items.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -skip TestCatalogSnapshot -run '(?i)(TestPinout|TestDefaultCatalogLoads)'
  ```
  Expected `ok`.

**W2-Z Closing**
1. Snapshot regeneration and the same checks as W1-Z step 2.
2. Browser smoke:
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^(TestDesktopECalcPinoutRealBrowser|TestDesktopECalcPinoutSkinsBrowser|TestDesktopECalcCodesMarkingsBrowser)$'
   ```
3. Combined review: pin data against the records, SOT drawing direction, Go/JS parity of the code kind.
4. Docs:
   - manual (pinouts 71, ICs table, SMD inductor);
   - tools manual row;
   - `internal/ecalc/AGENTS.md` (code kinds, file map, packages, `<group>[_<part>].yaml`);
   - UI AGENTS (CONTROLLERS, the SOT branch of the draw description);
   - pinouts and codes `SUMMARY.md`.
5. Commits: `feat(ecalc): SOT-23 and SOT-223 packages`, `feat(ecalc): SMD inductor marking`, one `feat(ecalc): <file> pinouts` per batch, snapshot, docs.

## Wave 3 — connectors

**W3-E6a Group `vehicle_mains`** → **W3-E6b Outlines** · serial (both edit `pinouts.go` and `pinouts_test.go`)
- E6b input: `research/connectors/{dvi_i,scart,sd_card,microsd_card,rj11}.md`, for the orientation of each outline.
- E6a verify:
  ```
  go test ./internal/ecalc -count=1 -run TestPinout
  go test ./ui -count=1 -run '(?i)^TestECalcPinout'
  ```
- E6b verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestPinout|TestQueryPinout)'
  npm run test:ecalc
  go test ./ui -count=1 -run '(?i)^TestECalcPinout'
  ```
  All pass; `test-ecalc-pinouts.mjs` reports 20 outlines, each drawn by a fixture.

**W3-Q1 … Q7 Content** · after E6b. Run at most 3–4 in parallel: Q1 ∥ Q2 ∥ Q3, then Q4 ∥ Q5 ∥ Q6 ∥ Q7. Each batch owns the new YAML file named in A.1 (Wave 3), its vectors and its records.
- Input: `research/connectors/<id>.md`.
- Same steps and verification as W2-P*.

**W3-Z Closing**
1. Snapshot regeneration and the W1-Z checks.
2. Browser smoke:
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^(TestDesktopECalcPinoutRealBrowser|TestDesktopECalcPinoutDrawingsBrowser|TestDesktopECalcPinoutSkinsBrowser)$'
   ```
3. Review: views and orientation, mains safety notes, approximations stated in the docs.
4. Docs:
   - manual (pinouts 119, the seven groups, the vehicle section);
   - both AGENTS files (groups, twenty outlines, their orientations);
   - pinouts `SUMMARY.md`.
5. Commits, then rebase `ecalc-ext` on `main`, merge locally. No push.

## B.1 Templates — copy from these (do not write new examples)

- **Calculator, no choice, rules in every direction:** `internal/ecalc/catalog/calculators/power.yaml:5-32` (`lm317_voltage`). Vectors: `internal/ecalc/testdata/vectors/calculators/buck_converter.json:1-18` (format).
- **Calculator with choice consts:** `basics.yaml:133` (`decibel_ratio`, consts at 153/156).
- **Forward-only calculator with `round()` and integer** (for `avr_timer`): `digital.yaml:130-160` (`uart_baud_error`).
- **Mirror models:** `filters.yaml:122-147` (`sallen_key_lowpass`), `opamps.yaml:124` (`opamp_integrator`), `power.yaml:93` (`buck_converter`).
- **Schematic stubs:** `ui/js/desktop/apps/ecalc/ecalc-schematics-filters.js:82`, `ecalc-schematics-power.js:25`, `ecalc-schematics-opamps.js:92`.
- **Pinout `dip`:** `catalog/pinouts/ics.yaml:4-24` (`ne555`), with `testdata/vectors/pinouts/ne555.json` and `documentation/ecalc-sources/pinouts/ics/ne555.md`.
- **`to` with variants:** `ics.yaml:214-269` (`to92`), with `to92.json` and `to92.md`.
- **Board:** `boards.yaml:4-59` (`arduino_uno`, with `inner`).
- **Rows:**
  - rect: `debug.yaml:4` (`arm_jtag_20`), `pc_power.yaml:227` (`fan_3`);
  - dsub: `data.yaml:293` (`vga`);
  - minifit: `pc_power.yaml:50` (`eps_8`);
  - jst: `debug.yaml:153` (`qwiic`).
- **Circular:** `audio.yaml:49` (`xlr`), `:75` (`midi`).
- **Jack:** `audio.yaml:4` (`audio_trs`); with variants `audio.yaml:21` (`audio_trrs`).
- **Code item:** `catalog/codes/smd_resistor.yaml:1-36`, with `documentation/ecalc-sources/codes/smd_resistor.md` and the `smd_resistor` cases in `testdata/vectors/codes.json`.

---

## GitNexus in the worktree (settled by the repo's earlier practice)

CLAUDE.md requires `impact` before edits and `detect_changes` before every commit, but the worktree cannot be indexed (the indexer runs out of memory). As in the earlier worktree executions: the orchestrator runs `impact` against the main checkout's index before each engine edit and hands the result to the implementer; commits in the worktree carry the line "GitNexus detect-changes unavailable in the worktree; judged by symbols and tests."; `detect_changes` with `scope: compare` runs once from the main checkout after the branch has landed.

## Assumptions I made instead of asking

- **Seven new dimensions plus ppm instead of mislabelled W/number units.** If you prefer fewer: drop `datasize`/`datarate` by putting the size in `number` with a unit choice. The others have no honest substitute.
- **RJ11 gets a 4th outline `rj11`** that shares the `rj45` builder. It changes text, not drawing.
- **More items than approved, from splitting** (Wave 3 ≈ 48 items instead of 40): a variant needs identical pin numbers, and two housings are two items. Split cases: SD/microSD, CEE 3/5-pin, ISO 10487 A/B, RGB/ARGB, Leonardo/Micro, mikroBUS/Pmod.
- **One item each where the pinout is identical:** IRF540N and IRLZ44N share an item. The new TO-92 parts become variants of `to92`.
- **SOT-23 and SOT-223 are examples items with variants**, because SOT-23 has no single standard pinout.
- **JST-XH balance is one 6S item** with the n+1 rule in the notes. Eurorack is one 16-pin item with the 10-pin header in the notes.
- **Pinout files may be split per group** (`ics_logic.yaml`, …) so batches can run in parallel. AGENTS is updated to `<group>[_<part>].yaml`.
- **Rail placement:** sensors and drives are appended after `wiring`. Modules go to `debug`, PC headers to `pc_power`. The group labels are unchanged; renaming them to "Interfaces and modules" or "PC power and headers" would be a locale-only follow-up.
- **No push or deploy.** The work lands on local `main` only.
