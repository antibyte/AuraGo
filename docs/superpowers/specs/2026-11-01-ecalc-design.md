# ECalc desktop app — design

Date: 2026-11-01. Status: approved by the user section by section (all four content areas in v1, agent with compute and
open, layout C, declarative catalog with two evaluators, four phases).

## Goal

ECalc is a formula collection and reference for electronics hobbyists, built into the AuraGo virtual desktop. It is modelled
on the Android app ElectroDroid and has four areas: interactive calculator pages, reference tables (for example the current
capacity of cables and PCB traces), component code decoders and graphical pinouts. A calculator page shows the schematic
with its input fields placed in it, the formula typeset with the live values, and the results calculated automatically as
the user types. The AuraGo agent uses the same catalog and formulas to compute values and to open a page pre-filled with
values.

## Decisions taken with the user

| Topic | Decision |
| --- | --- |
| Scope v1 | All four areas: calculators, reference tables, component codes, pinouts. Delivered in four phases (see Phases). |
| Agent | Compute and open. The `ecalc` tool uses the same catalog and formulas and can open ECalc on a page with values pre-filled. |
| Languages | UI chrome, categories and item names in all 16 desktop locales. Long explanations and table notes in German and English; other locales fall back to English. |
| Standards | EU/DE primary: mm² with DIN VDE 0298-4 and IEC 60364-5-52 installation methods. US (AWG/NEC) as a switchable second view. PCB traces per IPC-2221. IPC-2152 only if a published, citable approximation can be verified; otherwise omitted with a note. |
| Calculator layout | "C, combined". Input fields sit on the schematic next to the components. Beside it, the typeset formula with live substituted values. Below it, the nearest E-series value with its deviation and the derived quantities (for example cross current and dissipated power of a divider). Calculators without a schematic (for example dB) show the formula alone. |
| Architecture | Approach 1: a declarative catalog in Go (embedded YAML), a small safe expression language implemented twice (Go for the agent, JavaScript for instant in-browser calculation), and shared test vectors run against both. The rejected options are described under Approach. |

## Constraints that shape the design

- The repo's calculator tests enforce a "no eval" rule for desktop calculator code. ECalc keeps the same rule: the JavaScript
  evaluator uses neither `eval` nor `Function`.
- Shipped web assets load nothing external: no internet fonts, libraries or analytics. `TestShippedWebAssetsLoadNothingExternal`
  guards this.
- Every colour comes from the `--vd-mat-*` and `--ds-color-*` tokens, so the app follows all 14 desktop skins.
- The Go module has no embedded JavaScript VM today, and a change in the browser must not need a server round trip.
- In-app shortcuts must not reach the desktop's global Spotlight shortcut. This is the same lesson as the Ctrl+K handling in
  Docker Manager.

## Approach: declarative catalog with two evaluators

Every calculator, table, code and pinout is data in an embedded YAML catalog. Go owns the catalog, the expression language,
units, E-series, codes and the solver. The desktop app runs a JavaScript mirror of the same logic, so a value typed into a
field updates the results at once, and the page keeps working over remote access without a request per keystroke.

Rejected:

- **Go-only, with an API round trip per keystroke.** It is sluggish over remote access, and the interactive widgets (schematic
  inputs, colour bands, drag interaction) still need JavaScript.
- **JavaScript only, with an embedded JS VM (goja) in Go.** A large new dependency, and the repo has none.

The cost is that the expression language and the solver exist twice. Shared test vectors in
`internal/ecalc/testdata/vectors/*.json` run against both the Go and the JavaScript implementation, so any divergence fails
the tests.

## Architecture

```
internal/ecalc/                 Go package, no server dependency
  catalog/{calculators,tables,codes,pinouts}/*.yaml   (go:embed)
  catalog.go   schema, loading, strict validation at startup and in tests
  expr.go      expression language: parser -> AST -> float64 evaluation, fixed function whitelist
  units.go     dimensions, SI prefixes, parsing ("4k7", "2u2", "10 mA", "1e-6", comma decimal), formatting
               (engineering notation, configurable significant digits)
  eseries.go   E3…E192, nearest standard value + deviation, two-resistor combination finder
  solve.go     rule propagation solver
  codes.go     resistor colour bands, SMD resistor codes (3/4-digit, R notation, EIA-96), capacitor and inductor codes
  testdata/vectors/*.json   shared test vectors for Go and JS

internal/server   GET /api/ecalc/catalog?lang=xx — localized catalog (names/docs resolved for the language with English
                  fallback), cached, ETag; plus the audit route contract entry in internal/audit/audit_manifests.go
internal/tools    agent tool `ecalc`
ui/js/desktop/apps/ecalc/   JS mirror of expr/units/eseries/solve/codes, schematic symbol library, schematic specs,
                  page renderers (calculator, table, code, pinout), entry ecalc.js exporting
                  window.ECalcApp = { render(hostEl, windowId, ctx), dispose(windowId) }
ui/css/desktop-app-ecalc.css   theme-native via --vd-mat-* and --ds-color-* tokens only
```

The JS evaluator must not use `eval` or `Function`, in line with the calculator rule above.

### `internal/ecalc` (new): catalog, expressions, solver

- The package does not import the server. The HTTP route, the agent tool and the tests all use it directly.
- The catalog is embedded with `go:embed` and validated strictly, both at startup and in tests.
- Each file has one job. `catalog.go` holds the schema, loading and validation. `expr.go` holds the parser, the AST and the
  evaluation with a fixed function whitelist. `units.go` holds dimensions, prefixes, parsing and formatting. `eseries.go`
  holds the standard values and the two-resistor combination finder. `solve.go` holds the rule propagation. `codes.go`
  holds the code decoders and encoders.

### Server route and audit contract

`GET /api/ecalc/catalog?lang=xx` returns the catalog for one language. Names and documents are resolved for that language,
with English as the fallback. The response is cached and carries an ETag, so the app can revalidate it cheaply. The route
also gets an entry in the audit route contract in `internal/audit/audit_manifests.go`.

### Agent tool

The `ecalc` tool in `internal/tools` calls the Go package directly. Its actions are described under Agent tool `ecalc`.

### Desktop app (`ui/js/desktop/apps/ecalc/`)

The app folder holds the JavaScript mirror (`expr`, `units`, `eseries`, `solve`, `codes`), the schematic symbol library and
specs, and the renderers for calculator, table, code and pinout pages. The entry file `ecalc.js` exports
`window.ECalcApp = { render(hostEl, windowId, ctx), dispose(windowId) }`. The stylesheet is `ui/css/desktop-app-ecalc.css`.

```
ECalc app (JavaScript)                              Agent tool ecalc (Go)
  JS mirror: expr, units, eseries,                    search, describe, calculate,
  solve, codes                                        table, decode, encode, pinout
  schematics, page renderers                          open (desktop event, see below)
        |                                                   |
        | GET /api/ecalc/catalog?lang=xx                    | direct Go calls
        v                                                   v
  internal/server  ------------------------------->  internal/ecalc
                                                      catalog, expr, units, eseries,
                                                      solve, codes (no server dependency)
```

## Catalog schema

### Calculator items

The approved shape of a calculator item:

```yaml
id: voltage_divider
category: basics
name: {en: Voltage divider, de: Spannungsteiler, …}   # all 16 desktop locales required
doc:  {en: "…", de: "…"}                             # other locales fall back to English
tags: [divider, teiler, potentiometer]
vars:
  vin:  {dim: voltage, label: "V_in", default: 12}
  r1:   {dim: resistance, default: 4.7k, eseries: true}
  r2:   {dim: resistance, default: 2.2k, eseries: true}
  vout: {dim: voltage, label: "V_out"}
dof: 3                              # this many known values determine the rest
rules:                              # every direction explicit, no symbolic algebra
  vout: vin*r2/(r1+r2)
  r2:   vout*r1/(vin-vout)
  r1:   r2*(vin-vout)/vout
  vin:  vout*(r1+r2)/r2
derived:
  i: {dim: current, expr: vin/(r1+r2)}
  p: {dim: power, expr: vin*i}
checks:
  - {when: vout >= vin, warn: vout_ge_vin}
formula: vout = vin * r2 / (r1 + r2)    # display formula, parsed by the same parser
schematic: voltage_divider
source: "…"                             # standard, datasheet or textbook (required)
```

The keys have these meanings:

| Key | Meaning |
| --- | --- |
| `vars` | Variables with a dimension (`dim`), an optional display `label` (`V_in` is typeset with a subscript), a `default` and, where the E-series suggestion applies, `eseries: true`. |
| `dof` | How many known values determine the others. The example has four variables and `dof: 3`. |
| `rules` | One explicit expression per direction, so each computable variable has a rule for its own target. The author writes every inversion; the engine does no symbolic algebra. |
| `derived` | Quantities computed after solving, each with a dimension and an expression. |
| `checks` | Conditions evaluated after solving. When `when` is true, the warning named by `warn` is shown. |
| `formula` | The display formula. It is parsed by the same parser as the rules, so a formula that does not parse fails validation. |
| `schematic` | The id of a schematic spec on the UI side. `internal/ecalc` never reads UI files; a Go test in the `ui/` package cross-checks every catalog schematic id against the UI schematic manifest. |
| `source` | The standard, datasheet or textbook behind the formula. |

### List-typed variables

A variable can hold a list, for example series or parallel resistors entered as `4k7, 10k, 2k2`. The functions `sum()` and
`par()` reduce such a list.

### Alternative rules

A target can list several alternative expressions as a YAML list. Each alternative uses its own variables. The first
alternative whose variables are all known is applied.

### Other item kinds

| Kind | Content |
| --- | --- |
| `calculator` | The fields described above: `vars`, `dof`, `rules`, `derived`, `checks`, `formula`, `schematic`, `source`. |
| `table` | Columns, each with a dimension and a unit; rows; `notes` with `en` and `de`; `source`. |
| `code` | A code kind plus its data, for example a colour mapped to digit, multiplier, tolerance and temperature coefficient. |
| `pinout` | A connector shape spec and the pins. Each pin has `number`, `name`, `function`, `type` (`power`, `ground`, `signal`, `data`, `io` or `nc`) and `notes`. |

Item ids are unique across all four kinds.

### Localization and sources

- `name` requires all 16 desktop locales. The validator enforces this.
- `doc` and `notes` fields require English and German. Other locales fall back to English when the server resolves the
  catalog for a language.
- Every item carries a `source`.
- The `warn` value of a check is a message key. Its text is written in the catalog and follows the same English and German
  rule as the explanations.

## Expression language and units

### Expression language

`expr.go` parses a string into an AST and evaluates it to a `float64`. The JavaScript mirror does the same.

- Operators: `+ - * / ^`, parentheses and comparisons.
- Functions, from a fixed whitelist: `sqrt ln log10 exp sin cos tan atan abs min max round`. The constant is `pi`.
- Conditional and aggregate: `if(c,a,b)`, `sum(list)`, `par(list)`.
- Table functions: `lookup(table, column, key)` and `interp(table, xcol, ycol, x)` read catalog tables, for example a derating
  factor by ambient temperature.
- The language has no assignments and no loops. Nothing outside the whitelist is callable, and anything outside the grammar
  is a parse error. Rules and formulas are parsed at startup, so a broken expression fails validation rather than failing at
  runtime.

### Units, numbers and formatting

Internally every value is in SI base units. Each dimension has display units, and the user picks one per field.

| Dimension | Base | Display units (examples) |
| --- | --- | --- |
| length | m | m, mm, µm, mil, inch |
| temperature | K | °C, °F, K |
| area | m² | mm², kcmil |
| voltage, current, resistance, capacitance, inductance, frequency, power | V, A, Ω, F, H, Hz, W | the SI prefixes as needed (mV, V, kV; µA, mA, A; kΩ, MΩ; pF, nF, µF; µH, mH; kHz, MHz; mW, kW) |

Non-linear units are defined by to-base and from-base expressions in the same language. The variable `x` is the value in the
display unit.

```
dBm   to base   W  = 10^((x - 30) / 10)              from base   x = 10 * log10(W) + 30
°F    to base   K  = (x - 32) * 5 / 9 + 273.15       from base   x = (K - 273.15) * 9 / 5 + 32
AWG   to base   m  = 0.127e-3 * 92^((36 - x) / 39)   from base   x = 36 - 39 * ln(m / 0.127e-3) / ln(92)
```

AWG exists as a unit of two dimensions: diameter (length, as above) and cross-section (area, via A = pi * d^2 / 4 on top
of the diameter formula), so both wire diameter and cross-section fields can switch to AWG.

Input parsing (`units.go`) accepts `4k7` (4.7 k, the letter is both decimal point and prefix), `2u2`, `10 mA`, `1e-6`, and a
comma as decimal separator. Output uses engineering notation with a configurable number of significant digits.

### Formula typesetting

The `formula` string of a calculator is parsed to an AST and rendered as HTML. Fractions are drawn for `/`, superscripts for
`^`, a radical for `sqrt`, and subscripts come from labels such as `V_in`. The live line renders the same AST with the
current values substituted, for example `V_out = 12 V * 2.2k / (4.7k + 2.2k) = 3.83 V`.

## Solver

### Known values

- Exactly `dof` fields are inputs at any time; all other fields are computed. The validator requires exactly `dof`
  variables with a `default`, and on first open those are the inputs.
- The most recently edited fields count as known. When the user types into a computed field, that field becomes an input and
  the oldest input becomes computed. This is the "oldest input yields" rule.
- A lock icon pins a field so it never yields. A locked field stays an input until the user unlocks it.

### Propagation

1. The known set starts with the inputs.
2. Rules are applied as a fixpoint: repeatedly, every rule whose target is unknown and whose variables are all known is
   applied, until nothing changes. A rule target with several alternatives uses the first applicable one.
3. The `derived` quantities are computed, and then the `checks` are evaluated. Triggered checks are shown as warnings.

Every direction is an explicit rule, so no step needs symbolic algebra. Each dimension states whether it must be positive.

### Under-determined, errors and contradictions

- Under-determined: when fields are still unknown, they show an "enter more values" hint.
- Errors: a NaN, an infinite result, or a wrong-sign result for a dimension that must be positive shows as an error state on
  the field. It is never shown as a number.
- Contradictions: when the agent or a test supplies more values than `dof`, each extra value is checked against the value
  computed from the others. A value that disagrees beyond tolerance is reported as a contradiction.

## App UI

### Window and layout

- The default window is about 1100 × 760 and resizable. The minimum size is set through `appWindowMinSize`, and the default
  size through `appWindowSize`.
- Three columns: a category rail, an item list and the page.
- The category rail lists Favorites and Recent, then the ten calculator categories (Basics, Passive, Filters, Semiconductors,
  Power supply, Op-amps, Timers, RF, Digital, Wiring & PCB), then Tables, Codes and Pinouts.
- On a compact viewport the rail, the list and the page collapse into one stack. Each level can go back to the previous one.
- The start menu category is `dev`.

### Search

- The search box at the top searches all four kinds by the current-language name, the English name and the catalog tags.
  Tags also match values such as "USB-C".
- The in-app search shortcut (Ctrl+K, as in Docker Manager) calls `stopPropagation`, so the desktop's global Spotlight shortcut
  does not open as well.

### Calculator page (layout C)

The page follows layout C, "combined". The input fields sit on the schematic next to the components they belong to. Beside
the schematic, the typeset formula and its live line are shown. Below them, the nearest E-series values and the derived
quantities are listed.

```
Voltage divider                                                  [favorite] [copy] [reset]

  +-----------------------------+    V_out = V_in * R2 / (R1 + R2)
  | schematic, fields on it     |    V_out = 12 V * 2.2k / (4.7k + 2.2k)
  |                             |    V_out = 3.83 V
  +-----------------------------+

  E24 nearest:  R1 4.7k (0.0 %)   R2 2.2k (0.0 %)
  derived:      i = 1.74 mA       p = 20.9 mW
```

Within the page:

- Each field has a unit dropdown.
- Each `eseries` variable gets an E-series suggestion. The series is selectable from E6 to E192, and the default is E24.
- Warnings from the `checks` are shown on the page.
- The explanation is collapsible and shows the source.
- The actions are copy results, reset to defaults, and favorite.
- A calculator without a schematic shows the formula alone.

### Table page

- Columns are sortable and filterable. A unit toggle switches between views, for example mm² and AWG.
- An optional finder header sits above the table. The cable ampacity finder takes an installation method, an ambient
  temperature and a grouping, and returns the required cross-section with the matching row highlighted.
- A finder is an ordinary calculator that uses `lookup` and `interp`.
- Transcribed standard tables carry the guide-value marking described under Content correctness.

### Code page

- A graphical resistor with 4, 5 or 6 bands works in both directions. Clicking a band changes the value. Typing a value shows
  the bands of the nearest standard value.
- An SMD part is drawn for a code input.
- Capacitor and inductor codes are handled on the code pages as well.

### Pinout page

- The connector SVG is generated from the data and has a plug and socket view toggle.
- Pins are coloured by type: power, ground, signal, data, io and nc.
- Hovering over or clicking a pin shows its details. A pin table sits beside the drawing.

### Schematics

- The JavaScript symbol library holds: R, C, polarized C, L, diode, LED, Zener, NPN and PNP transistors, N- and P-channel
  MOSFETs, op-amp, source, battery, ground, crystal, transformer, fuse, antenna, and IC boxes for the 555 and LM317 with pin
  names.
- IEC symbols are the default. A setting switches to ANSI, which draws the zigzag resistor.
- Each schematic is a compact grid spec with symbols, wires and field anchors. It is rendered to inline SVG that uses
  `currentColor`, so it follows every theme.
- Schematic specs live on the UI side and are keyed by calculator id. The agent does not need them.
- A test checks that every field anchor matches a variable of its calculator.

### Theming

- Only `--vd-mat-*` and `--ds-color-*` tokens are used, so the app works in all 14 desktop skins.
- No external assets are loaded, which `TestShippedWebAssetsLoadNothingExternal` enforces.

### Persistence

- Settings live in the browser's localStorage under the key `aurago.ecalc.v1`. They hold the last page, favorites, recents,
  unit choices, significant digits, the E-series and the last values per calculator.
- All storage access is wrapped in try/catch. The app works fully without storage.
- Server-side storage of favorites and values is out of scope for v1.

### Strings

- UI chrome strings are `desktop.ecalc_*` keys in the 16 files `ui/lang/desktop/*.json`. They are covered by the existing
  `TestDesktopUsedI18nKeysExistInAllLanguages`.
- The app name key is `desktop.app_ecalc`.

## Content v1

The v1 content is about 70 calculators in 10 categories, about 30 tables, 5 code pages and about 35 pinouts. Every item
carries a source, as described under Content correctness.

### Calculators (about 70 in 10 categories)

- **Basics:** Ohm's law and power; voltage divider (unloaded and loaded); current divider; series and parallel with list
  input; standard-value combiner (two E-series resistors for a target); dB (voltage, power, dBm, dBV, dBu); Wheatstone
  bridge; star-delta transform; RMS, peak and average; charge and energy (mAh, Wh, J).
- **Passive:** reactance X_C and X_L; impedance of RC, RL and RLC, series and parallel; RC time constant and charge curve;
  C and L in series and parallel; capacitor energy; air-core coil (Wheeler); toroid turns from the AL value; wire resistance
  and temperature coefficient.
- **Filters:** RC low-pass and high-pass; LC resonance; RLC Q and bandwidth; Sallen-Key second order; twin-T notch; pi and T
  attenuators.
- **Semiconductors:** LED series resistor (single LED, string, parallel strings); Zener regulator; transistor as a switch;
  voltage-divider bias; MOSFET losses; heat sink (thermal resistance chain to junction temperature).
- **Power supply:** LM317 voltage and current source; linear regulator dissipation; buck and boost (duty, inductor, ripple);
  smoothing capacitor; battery runtime and charge time; battery pack in series and parallel; cable voltage drop (copper and
  aluminium, DC and AC).
- **Op-amps:** inverting; non-inverting; difference; summing; integrator; Schmitt trigger; instrumentation amplifier.
- **Timers:** NE555 astable and monostable; crystal load capacitance; frequency, period and duty; PWM average.
- **RF:** frequency ↔ wavelength; dipole and λ/4 with velocity factor; microstrip and coax impedance; SWR, return loss and Γ;
  dBm ↔ W ↔ V at 50 Ω; free-space path loss; skin depth.
- **Digital:** ADC and DAC (LSB, output); I²C pull-up (minimum and maximum); level-shift divider 5 V → 3.3 V; UART baud error;
  number systems hex, dec and bin.
- **Wiring & PCB:** trace width per IPC-2221 (external and internal layers, oz and µm copper, ΔT); trace resistance and voltage
  drop; via current and resistance; trace spacing by voltage; cable cross-section finder per VDE with derating; AWG ↔ mm² ↔
  diameter.

### Tables (about 30)

- **Conductors:** cable ampacity per DIN VDE 0298-4 and IEC 60364-5-52 (installation methods A1 to F, copper and aluminium,
  PVC and XLPE, 2 and 3 loaded cores); ambient temperature and grouping factors; flexible cords; AWG with NEC ampacity (US
  view); standard cross-sections; IPC-2221 grid (width × copper weight × ΔT); clearance and spacing; oz ↔ µm.
- **Components:** E3 to E192; SMD package sizes (dimensions, power, voltage); ceramic dielectrics C0G, X7R, X5R and Y5V; fuse
  characteristics and form factors; battery chemistries and formats; barrel connector sizes.
- **Materials:** resistivity and temperature coefficient α for copper, aluminium, silver, brass, constantan, manganin,
  nichrome, solder and others; dielectric constants (FR4, PTFE, Rogers and others); thermocouple types with IEC colours;
  solder alloys.
- **General:** SI prefixes and units; physical constants; Greek alphabet; IP codes and protection classes; mains voltages and
  plug types worldwide; frequency bands (ISM, LoRa, Wi-Fi channels, amateur radio DE); logic levels (TTL, CMOS 5 V, 3.3 V and
  1.8 V, RS-232, RS-485, LVDS); ASCII; wire colour codes (VDE and IEC).

### Component codes (5)

- Resistor colour code per IEC 60062: 4, 5 and 6 bands, in both directions.
- Inductor colour code.
- SMD resistor codes: 3-digit and 4-digit, R notation and EIA-96.
- Capacitor codes: the pF code, tolerance letters, and EIA voltage codes such as `1H` = 50 V.
- Ceramic temperature codes.

### Pinouts (about 35)

- **PC and consumer:** USB-A, USB-B, Mini and Micro; USB-C, including the CC resistor values for PD and default current; HDMI-A,
  DisplayPort, VGA; RJ45 T568A and T568B with PoE pairs; RS-232 DB9; audio jack TRS and TRRS (CTIA and OMTP); XLR; MIDI;
  ATX-24, EPS-8, PCIe 6-pin and 8-pin, SATA data and power, Molex; fan 3-pin and 4-pin.
- **Maker:** Arduino Uno and Nano; ESP32 DevKitC; ESP8266 NodeMCU; Raspberry Pi 40-pin and Pico; ARM JTAG-20, SWD-10 and
  AVR ISP-6; FTDI-6; Qwiic and STEMMA QT; Grove; servo; OBD-II.
- **ICs:** NE555, LM317, 78xx, LM358, LM324, 74HC595, ULN2003, ATmega328P, and TO-92 examples. The pinout of a TO-92 part varies
  by part, and the item says so.

## Content correctness

These rules are binding for every item:

- Every item carries a `source`: a standard, a datasheet or a textbook.
- Every rule direction has at least two test vectors from independent references (datasheet examples, TI application notes,
  textbook examples).
- Every calculator has an automatic round-trip test. The test computes forward, solves backward, and the original inputs come
  back within tolerance.
- Formula-derived tables (AWG and the IPC-2221 grid) are generated by code and checked against published values.
- Transcribed standard tables (for example the VDE ampacity values) are cross-checked against two independent sources. The app
  marks them "guide values — the standard is authoritative".
- Pages related to mains voltage show a safety note.
- IPC-2152 is used only with a verifiable published approximation. Otherwise the app uses IPC-2221 with a note.

## Agent tool `ecalc`

The tool is read-only, except for `open`, which opens a window on the desktop. It uses the same catalog and the same formulas
as the app, computed by the Go implementation.

| Action | Input | Output |
| --- | --- | --- |
| `search` | `query` | Calculators, tables, codes and pinouts: id, kind, name and a short description. |
| `describe` | `id` | Variables with units, `dof`, the solvable directions and the checks. For tables, the columns. |
| `calculate` | `id`, `values`, optional `eseries` | All values in SI and formatted, which values were computed, E-series suggestions and warnings. A clear error when the input is under-determined or contradictory. |
| `table` | `id`, optional `filter`, optional `lookup` | Rows, paged. Supports lookup and interpolation. |
| `decode` / `encode` | `kind` and item-specific fields | Colour bands, SMD codes and capacitor codes, in both directions. |
| `pinout` | `id`, optional `pin` | Pins with their function and type. |
| `open` | `id`, optional `values` | Opens ECalc on the desktop at that page, pre-filled with the values. |

Details:

- Values are strings with units and prefixes, for example `"4k7"` or `"12 V"`.
- Large outputs, such as tables, are paged so that they fit the inline output budget.
- The tool is found through tool search and is not part of the core prompt. It is registered in `internal/agent/tool_categories.go`,
  in the tool search terms under `internal/agent/tool_search_terms/*.yaml`, and in the tool role policy
  (`internal/agent/tool_role_policy.go`).
- Whether the tool gets its own config switch follows the pattern of comparable read-only tools. The plan for phase 2 fixes it.
- `open` uses the existing `virtual_desktop_apps` open_app event (`internal/tools/virtual_desktop.go`, handled in
  `ui/js/desktop/core/sdk-events-bootstrap.js`) with a launch payload that carries the id and the values. The exact bridge is
  fixed in the phase 2 plan.
- The agent does not need the schematic specs, so they are not part of the tool's output.

## Integration points in the existing desktop

- [ ] `internal/desktop/types.go`: a `BuiltinApps()` entry with ID `ecalc`, Category `dev` and Entry `builtin://ecalc`.
- [ ] Icon: either a name in `desktopPreferredIconNames`, which must also exist in the Papirus set (`ui/desktop_papirus_assets_test.go`
  requires both directions), or artwork through `metadata.logo_path`.
- [ ] `ui/js/desktop/core/module-loader.js`: a `DESKTOP_APP_ASSETS` entry with the CSS and the scripts. The composition root is
  loaded last.
- [ ] `ui/js/desktop/core/menus-and-routing.js`: `renderAppContent` dispatches to `window.ECalcApp`.
- [ ] `ui/js/desktop/core/desktop-foundation.js`: an `appGlobalName` mapping, so that `dispose` runs when the window closes;
  an `appIconKeys` entry if the icon is themed.
- [ ] `ui/js/desktop/core/window-shell-runtime.js`: `appWindowSize` and `appWindowMinSize`.
- [ ] Bundle: the core shell files are bundled into `ui/js/desktop/bundles/main.bundle.js`. After editing them, run
  `npm run build:ui` and then `npm run build:ui -- --check`.
- [ ] Guards to extend: the app CSS list in `ui/frontend_optimization_test.go`; `ui/desktop_app_materials_test.go` (the stylesheet
  must use `--vd-mat-`); the table in `ui/desktop_theme_consistency_test.go`; `internal/desktop/app_categories_test.go`; and
  `TestDesktopBuiltinAppNamesExistInAllLanguages`, which requires `desktop.app_ecalc` in all 16 locales.
- [ ] Server route `GET /api/ecalc/catalog` in `internal/server`, with its entry in the audit route contract.
- [ ] Manuals: chapter 32 in `documentation/manuals/en/32-ecalc.md` and `documentation/manuals/de/32-ecalc.md`, with the header
  `# Chapter 32: ECalc – <tagline>` and a previous-chapter footer like the one in `31-pixel.md`. Rows in
  `documentation/manuals/README.md`, `documentation/manuals/en/README.md`, `documentation/manuals/en/04-webui.md` and
  `documentation/manuals/de/04-webui.md`.

## Testing

### Go

- Catalog validation test: every expression parses; every variable is reachable from some set of `dof` inputs; all 16 names
  are present; English and German docs are present; a source is present; exactly `dof` variables carry a default.
- Cross-check test in the `ui/` package: every catalog schematic id has an entry in the UI schematic manifest, and every
  manifest entry belongs to a calculator.
- Unit tests for `expr`, `units`, `eseries`, `codes` and `solve`.
- Shared vectors from `internal/ecalc/testdata/vectors/*.json`, run against the Go code.
- A round-trip property test over all calculators.
- A server route test and the audit contract.
- Tests for the agent tool.

### JavaScript

- The same vectors run against the JavaScript evaluators. The runner is Node if the repo has a JavaScript test runner for
  this area. Otherwise it is a browser test, gated like the existing browser smokes by `AURAGO_RUN_BROWSER_SMOKE=1`.
- Every schematic anchor matches a variable of its calculator.

### Desktop

- The registration guards listed under Integration points.
- Source-marker tests on the app JavaScript, in the style of `ui/desktop_calculator_test.go`.
- Browser tests for layout C rendering, live solving, the colour-code round trip, pinout drawing and the compact viewport.
- A final screenshot survey across all 14 skins with the existing desktop preview harness.

## Phases

Each phase gets its own implementation plan and lands separately.

1. **Foundation.** `internal/ecalc` core and the JavaScript mirror, with parity tests. Catalog API. App shell with navigation,
   search, favorites and recent. Calculator page with formula typesetting. Schematic symbol library. About ten basics
   calculators. Registration and the icon. Manual chapter 32 skeleton.
2. **Agent and calculator library.** The `ecalc` tool with `search`, `describe`, `calculate` and `open`, including the pre-fill
   bridge. The remaining calculators (about 60) with schematics.
3. **Tables and codes.** The table page with finders and all tables. The colour band, SMD and capacitor code pages. The tool
   actions `table`, `decode` and `encode`.
4. **Pinouts.** The connector renderer and all pinouts. The tool action `pinout`. The manual completed. Acceptance across all
   skins.

## Out of scope for v1

- SMD marking-code database for diodes and transistors.
- User-defined custom formulas.
- Server-side storage of favorites and values.
- Charts beyond the RC charge curve.
- IPC-2152 without a verifiable source.
