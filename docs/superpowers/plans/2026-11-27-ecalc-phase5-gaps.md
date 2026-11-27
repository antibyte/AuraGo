# ECalc phase 5 (`ecalc-p5`): design and execution plan

Date 2026-11-27 · Base: `main` @ aec34ec39 · Binding rules: `internal/ecalc/AGENTS.md` and the `## ECalc` section of `ui/js/desktop/apps/AGENTS.md`. This document does not repeat those rules. It points to them, and A.3 lists the per-item checklist they imply.

Scope: groups 1 and 2 of the comparison with circflow.com/tools (user choice 2026-11-27). Group 1 is the calculators, tables and decoders that ECalc lacks; group 2 is the missing IC pinouts and the pin-compatible parts that the catalog has but cannot find by name. The digital calculators that need bit operations (group 3) and the tools left out on purpose (group 4) are out of scope (A.4). Only the list of Circflow's tools and their one-line descriptions were read. No data is taken from Circflow; every value comes from our own research in primary sources, as in phases 1–4.

---

# PART A — Design

## A.0 Findings that shape the design

- **Calculators without numeric inputs are refused by the loader only.** `catalog.go:756-758` fails `dof < 1`. `Calculator.Solve` (`solve.go:119`) already treats the constants of the selected choice options as known before it applies the rules, so a calculator whose only inputs are choices needs: the loader, `SolvableSets` for zero inputs (one empty set), the ECMAScript twin `ecalc-solve.js`, a page with no input fields (`ecalc-calc-page.js`), and the tool's `compute` with choices only. Nothing in the expression language changes.
- **Derived-only calculators exist.** `mosfet_losses` (`calculators/semiconductors.yaml`) has dof 7, no rules, and every result is `derived`. That is the model for the MOSFET logic-level check.
- **Closed-form impedance rules exist.** `microstrip_impedance` (`calculators/rf.yaml`) has one closed-form rule for every input, `t` in `oz`, a `w/h` derived ratio and validity checks. Stripline and the differential lines follow it.
- **Choice constants**: `decibel_ratio` (`calculators/basics.yaml`) is the template: `choices.<id>.options.<id>.consts`.
- **Tables** (`tables.go`): a column's `dim` is a unit dimension or `text` (`tables.go:19-20`, checked at 414-419). Groups are fixed: `conductors, components, materials, general` (`tables.go:29`). The table page tells numeric columns by `dim !== 'text'` (`ecalc-table-page.js:56`); `lookup` and `interp` refuse text columns (`ecalc-tables.js:57,81`, Go twin `tablefuncs.go`). Generators (`tablegen.go`) are Go-only and not needed in this phase.
- **Symbols**: `ecalc-symbols.js` draws 25 symbols (resistor capacitor polarized inductor diode led zener crystal fuse switch pot source acsource isource battery meter ground antenna npn pnp nmos pmos opamp transformer ic) in two standards: `svg(spec, {standard: 'iec' | 'ansi'})`, chosen by the setting `store.prefs().symbols`. Only a few symbols branch on the standard today (resistor, polarised capacitor, potentiometer). A symbol reference table therefore costs one new column kind and the research into where IEC 60617 and IEEE 315 really differ.
- **Code kinds** are registered in `codes.go:131-138` behind the `codeCodec` interface (`decode`, `encode`, `codes.go:123`). The list of kinds is pinned in:
  - `codes_test.go:153`, `:166`, `:225` and `:485`;
  - `query_codes.go:51`;
  - the JS registry `ecalc-codes.js:1102`;
  - the page `CONTROLLERS` (`ecalc-code-page.js:834`), itself pinned by `ui/desktop_ecalc_codes_browser_test.go:908`;
  - `KINDS` in `scripts/test-ecalc-codes.mjs:34`;
  - the code-kind table of `prompts/tools_manuals/ecalc.md:127`.

  The tool schema takes its enum from `ecalc.CodeKindIDs()`. **The schema is at 799 of 800 tokens** (`TestECalcSchemaIsStrictCompactAndNamedSafely`, `internal/agent/ecalc_tool_test.go:49`, budget at 102-106), so a new kind word needs a trim elsewhere in the schema text.
- **Pinouts need no engine change.** DIP-8 to DIP-28, `to92` variants and the `sot223` examples exist. The IC files are `pinouts/ics.yaml` (ne555 lm317 l78xx lm358 lm324 sn74hc595 uln2003 atmega328p to92 pc817 irf540n tip120 bt136 lm1117 sot23 sot223), `ics_logic.yaml` (sn74hc00 sn74hc04 sn74hc14 sn74hc138 sn74hc165 sn74hc245 cd4017 cd4051) and `ics_analog.yaml` (lm393 lm339 lm386 max232 l293d attiny85 mcp3008 mcp23017). Records live in `documentation/ecalc-sources/pinouts/ics/<id>.md`.
- **ULN2803 was dropped once.** The extension's research could open only onsemi's sheet, not TI's (`C:/disposable/ecalc-ext/research/parts/uln2803.md`, status PARTIAL). This phase tries again with TI SLRS049 or a second current maker document; if both makers cannot be read, it stays out again.

## A.1 Final item tables

Totals after this phase: calculators 113 → 119, tables 44 → 49, code items 6 → 7 (code kinds 6 → 7), pinouts 111 → 127. Every count assumes that the research verifies the item; A.4 says what happens otherwise.

### Wave 1 — six calculators

| # | id | cat | name en / de | vars (dim) | dof | rules / results | notes |
|---|---|---|---|---|---|---|---|
| 1 | stripline_impedance | rf | Stripline impedance (symmetric) / Stripline-Wellenwiderstand (symmetrisch) | er #, b m (plane-to-plane spacing), w m, t m (unit oz), z0 Ω | 4 | z0, er, b, w, t, each closed form | IPC-2141 symmetric stripline. The research fixes the exact form (IPC-2141A, Wadell) and its validity range; checks outside it, as in `microstrip_impedance`. |
| 2 | diff_microstrip_impedance | rf | Differential microstrip impedance / Differenzielle Mikrostreifenleitung | er, h, w, t, s m (edge-to-edge spacing), zdiff Ω | 5 | zdiff, s (closed form; w only if the research gives a clean inverse) | Z_diff = 2·Z_0·(1 − 0.48·e^(−0.96·s/h)) with Z_0 of the microstrip formula; derived z0 (single-ended) and z_odd = zdiff/2. Form and constants to be confirmed by two sources. |
| 3 | diff_stripline_impedance | rf | Differential stripline impedance / Differenzielle Stripline | er, b, w, t, s, zdiff | 5 | zdiff, s | Z_diff = 2·Z_0·(1 − 0.347·e^(−2.9·s/b)), same confirmation rule. |
| 4 | logic_level_check | digital | Logic level compatibility / Logikpegel-Verträglichkeit | targets nmh V, nml V (`input: false`) | **0** | nmh = voh − vih, nml = vil − vol | Needs **E1**. Choice `driver` (8 families) sets voh, vol; choice `receiver` (8 families) sets vih, vil, vin_max. Families at nominal supply, as in the `logic_levels` table; VOH at light load (CMOS input, ≤ 20 µA) and said so in the doc. Derived voh, vol, vih, vil for display. Checks: nmh < 0 (HIGH not recognised), nml < 0 (LOW not recognised), voh > vin_max (over-voltage on the receiver input). |
| 5 | mosfet_logic_switch | semiconductors | MOSFET as a logic-level switch / MOSFET als Logikpegel-Schalter | vgs V (drive), vth V (V_GS(th) max), vrds V (gate voltage of the R_DS(on) rating), rds Ω, i_d A | 5 | derived only: p_cond = i_d²·rds, vds_on = i_d·rds, margin = vgs − vth | Checks: vgs ≤ vth (does not turn on), vgs < vrds (R_DS(on) not guaranteed at this drive; the real resistance is higher). Doc: read vth and vrds from the datasheet table, not from the typical curve. |
| 6 | bits_range | digital | Bits and value range / Bits und Wertebereich | n int (bits), vmax int | 1 | vmax = 2^n − 1; n = ln(vmax + 1)/ln(2) | Derived: count 2^n, smin = −2^(n−1), smax = 2^(n−1) − 1, bits needed = ceil(n − 1e-9). Checks: n > 53 (beyond exact integers in Go and JS), n not whole (a value of this size needs ceil(n) bits). Round trip stays exact because the inverse is real-valued. |

Rail placement: no new categories. Schematics: #1–#3 get one each in `ecalc-schematics-rf.js` (cross-sections drawn with existing symbols only if they read clearly; otherwise none, as for `microstrip_impedance` today). #4–#6 have none.

### Wave 2 — five tables and one code kind

| id | kind / group | content | source basis to verify |
|---|---|---|---|
| ethernet_categories | table / general | Cat 5, 5e, 6, 6A, 7, 7A, 8.1, 8.2: bandwidth (MHz), ISO/IEC 11801 class, usual shielding, and the Ethernet speeds each supports with the channel reach (100BASE-TX, 1000BASE-T, 2.5G/5GBASE-T, 10GBASE-T, 25G/40GBASE-T) | IEEE 802.3 (free through IEEE GET) for speeds and reach; TIA-568 and ISO/IEC 11801 are paywalled, so category bandwidths need two independent maker documents. |
| usb_versions | table / general | USB 1.1 to USB4 Version 2.0: specification name, earlier names (3.0 = 3.1 Gen 1 = 3.2 Gen 1, …), the USB-IF marketing name, signalling rate, connectors | USB-IF specifications and the USB-IF Language Usage Guidelines (free). USB PD power levels as a note only. |
| blade_fuses | table / components | Automotive blade fuses: rating ↔ colour for the mini/regular/low-profile family and for MAXI (its colours differ), and which formats carry which ratings (incl. micro2, micro3) | ISO 8820-3 is paywalled: two maker catalogues (for example Littelfuse and Eaton Bussmann) must agree cell by cell. |
| level_shifters | table / components | Ways to connect logic of two voltages: resistor divider, series resistor, BSS138/2N7002 bidirectional MOSFET, PCA9306, TXS0108E, TXB0108, SN74LVC8T245/74LVC1T45 (dual supply, DIR), 74AHCT125/74HCT (3.3 V → 5 V), 74LVC with 5-V-tolerant inputs (5 V → 3.3 V). Columns: direction, open-drain (I²C) yes/no, push-pull, max data rate, supply ranges, DIR pin, remark | TI voltage-translation guides and the datasheets, NXP AN10441 for the MOSFET circuit. This replaces Circflow's "selector": the agent and the user read the rows; there is no recommendation engine. |
| schematic_symbols | table / general | One row per symbol of `ecalc-symbols.js` (25): the drawing in IEC and in ANSI side by side, name, what the part does, and where the standards differ | IEC 60617 (database) and IEEE 315 / ANSI Y32.2. Needs **E2**. A row may say "same in both". |
| date_code | code kind `date_code` (new) | 4-digit date codes read as YYWW **and** WWYY; both readings are shown when both are valid. Each reading gives the year, the ISO 8601 week and its Monday-to-Sunday dates, and refuses week 53 in a year that has only 52. Encode: a date gives the YYWW code. If the research confirms it, also the two-character year/month code of IEC 60062 | ISO 8601 for the week rules; maker documents for YYWW and WWYY; IEC 60062:2016 for the letter code. Needs **E3**. |

Two-digit years read as 2000–2099. The decoder has no notion of "today", so its results do not change with the date. A note says that a part may be from the 1900s.

### Wave 3 — IC pinouts (16 new, 5 edited)

All new parts use `dip`, with the same name in all 16 locales.

| batch / file | id | name | pins | flag |
|---|---|---|---|---|
| P1 `ics_logic.yaml` | sn74hc02 | 74HC02 (DIP-14) | 14 | NOR: outputs come first in each gate, unlike 74HC00 |
| | sn74hc08, sn74hc32, sn74hc86 | 74HC08 / 74HC32 / 74HC86 (DIP-14) | 14 | three items; the layout of 74HC00, different gates |
| | sn74hc74 | 74HC74 (DIP-14) | 14 | dual D flip-flop |
| | sn74hc139 | 74HC139 (DIP-16) | 16 | dual 2-to-4 decoder |
| | sn74hc161 | 74HC161 / 74HC163 (DIP-16) | 16 | one item if TI's figures are identical (asynchronous vs synchronous clear in the notes), otherwise two |
| | cd4011, cd4013, cd4511 | CD4011B / CD4013B (DIP-14), CD4511B (DIP-16) | 14/16 | |
| P2 `ics_analog.yaml` | ne556 | NE556 (DIP-14) | 14 | dual 555 |
| | lm741 | LM741 (DIP-8) | 8 | offset-null pins |
| | uln2803 | ULN2803A (DIP-18) | 18 | second attempt, see A.0 |
| | sn75176 | SN75176B (DIP-8) | 8 | RS-485 transceiver |
| | eeprom_24lc | 24LC32A / 24LC64 / 24LC256 (DIP-8) | 8 | one item if Microchip's figures agree pin by pin |
| | attiny13a | ATtiny13A (DIP-8) | 8 | own item: same pin numbers as ATtiny85, other alternate functions |
| | max232 (edit) | tag `max3232` + note | – | only if a maker document says pin-compatible |
| P3 `ics.yaml` (edits) | ne555 | tag `tlc555` + note | – | same rule |
| | lm358 | tags `tl072`, `ne5532` + note | – | each part's own datasheet must show the same pins |
| | sot223 | tag `ams1117` on the `lm1117` variant + note | – | AMS datasheet |
| | to92 | new variant `tmp36` | 3 | Analog Devices datasheet; a variant has its own pin list |

An edit updates the item's `notes`, its `tags`, its record ("Members that share this pinout") and, for `to92`, its vector. Nothing is renamed.

## A.2 Engine and UI changes (exact files)

**E1 — calculators whose only inputs are choices (Wave 1, before item #4)**
- `internal/ecalc/catalog.go:756-758`: allow `dof: 0` when the calculator has at least one choice and no input variable; keep `dof must be at least 1` for every other case, with a new message for a dof-0 calculator that has inputs.
- `internal/ecalc/solve.go` `solvableSets` / `SolvableSets`: zero inputs give one empty set; `Solve` needs no change (A.0).
- `ui/js/desktop/apps/ecalc/ecalc-solve.js`: the same in the twin.
- `ui/js/desktop/apps/ecalc/ecalc-calc-page.js`: a calculator without input fields shows its choices, results, derived values and checks; no lock pins, no "oldest input yields".
- `internal/tools/ecalc.go`: `compute` accepts `inputs` that are absent or empty when the calculator has dof 0; the same for `open`.
- Tests:
  - a loader case in `catalog_test.go` (accepted with choices only, refused with an input);
  - a solve case;
  - a node parity case through `testdata/vectors/core.json` or the calculator vector of #4;
  - a tool case in `internal/tools/ecalc_test.go`.
- Docs: one sentence in `prompts/tools_manuals/ecalc.md` and in `internal/ecalc/AGENTS.md`.
- GitNexus `impact` before editing: `(*loader).calculator`, `SolvableSets`, `(*Calculator).Solve`, `executeECalc`, and the JS `solve` of `ecalc-solve.js`.

**E2 — symbol column (Wave 2, before `schematic_symbols`)**
- New `internal/ecalc/symbols.go`: `SymbolIDs`, the 25 names of `ecalc-symbols.js` in its order. A ui test compares the list with `ECalcSymbols.names()` (pattern: the existing tests that read ECalc JS files from Go).
- `internal/ecalc/tables.go`: dimension `symbol` beside `text`. A cell is a symbol id, checked against `SymbolIDs`. `Numeric()` is false; `lookup` and `interp` refuse it like text.
- Table JSON: unchanged shape (the id travels as text).
- `ecalc-table-page.js`: a `symbol` column draws the cell twice, IEC and ANSI, through `ECalcSymbols.symbolMarkup` in a small `svg` per drawing, with the symbol name as the accessible label.
- `internal/ecalc/query_table.go`: the agent sees the symbol id as text (no change expected; confirm with a test).
- Tests:
  - a loader case in `internal/ecalc/tables_test.go` beside the text-column check at line 89 (a known id loads, an unknown id fails);
  - a fixture table with a `symbol` column next to `fx_plain` in `ui/desktop_ecalc_tables_browser_test.go:77`, checked by `TestDesktopECalcTablesBrowser` (two drawings per cell, accessible labels);
  - the ui test that compares `SymbolIDs` with `ECalcSymbols.names()`.
- `ecalc-symbols.js`: an ANSI or IEC drawing changes **only** where the research record of that symbol shows a difference. Every change also changes the calculator schematics in that standard, so `TestDesktopECalcBrowser` (chip check) runs in both standards in W2-Z.
- 16 `ui/lang/desktop/*.json`: the column captions "IEC" and "ANSI" are proper names; no key unless the page needs a new label.

**E3 — date code kind (Wave 2)**
- New `internal/ecalc/codes_date.go` (decode, encode), registered in `codes.go:131-138` and in the kind doc at `codes.go:14`.
- Kind lists in `codes_test.go:153,166,225,485`; `query_codes.go:51`.
- `testdata/vectors/codes.json`: decode cases (both readings, ISO week edges: week 1 that starts in December, week 53, an invalid week 54 or 00), encode cases and error cases.
- `ecalc-codes.js`: mirror and registry entry (`:1102`).
- `ecalc-code-page.js` `CONTROLLERS` (`:834`) gets `date_code: dateController`: one code field and the list of readings, with no drawing. Add the kind to `ui/desktop_ecalc_codes_browser_test.go:908` and to `KINDS` in `scripts/test-ecalc-codes.mjs:34`.
- New result fields get `desktop.ecalc_code_field_<id>` keys in 16 `ui/lang/desktop/*.json` and the page's key list.
- Content in the same task: `catalog/codes/date_code.yaml`, `documentation/ecalc-sources/codes/date_code.md`, a row in `codes/SUMMARY.md`.
- Agent: the enum grows by `date_code`. Trim the schema text so that `TestECalcSchemaIsStrictCompactAndNamedSafely` (`internal/agent/ecalc_tool_test.go:105`) stays at most 800 tokens. Move the removed words into the tool manual. Add a `date_code` row to `prompts/tools_manuals/ecalc.md:127`.

**Snapshot:** `go test ./internal/ecalc -run TestCatalogSnapshot -update -count=1`, only in each wave's closing task (and in E-tasks whose change alters the catalog JSON, which the task then says). Bundles: none; confirm with `npm run check:ui`.

**Docs:**
- `documentation/manuals/{en,de}/32-ecalc.md`: counts, the new rows per section, choice-only calculators, the symbol table, the date code.
- `internal/ecalc/AGENTS.md`:
  - dof 0;
  - the `symbol` dimension;
  - seven code kinds and their file map;
  - IC files unchanged.
- UI AGENTS `## ECalc`: the calc page without fields, the symbol column, `CONTROLLERS`.
- `documentation/ecalc-sources/{general,components,codes,pinouts}/SUMMARY.md`.

**Tests that pin counts or lists:**
- `codes_test.go:153,166,225,485`, `ui/desktop_ecalc_codes_browser_test.go:908`, `scripts/test-ecalc-codes.mjs:34`, `internal/agent/ecalc_tool_test.go:105`;
- `internal/ecalc/tables_test.go:89` and the fixture tables of `ui/desktop_ecalc_tables_browser_test.go:51-79`;
- `TestDefaultCatalogLoads`, `TestCalculateEveryCalculatorFromItsDefaults` and the round trip, which now meet a dof-0 calculator.

## A.3 Per-item content checklist (from the two AGENTS files)

**Every item:**
- [ ] id `^[a-z][a-z0-9_]*$`, ≤ 63 characters, unique across kinds. Variable, choice and option ids `^[a-z][a-z0-9_]{0,31}$`; no function names or `pi`.
- [ ] `name` in all 16 `Locales`; `doc` in en + de; every expression and text quoted; micro sign U+00B5 and omega U+03A9 written literally (the Write tool decodes backslash-u escapes).
- [ ] `source` names a standard by number, a well-known work at book or chapter level, or the maker's datasheet. Never an invented page, URL or application-note number.
- [ ] Tags: at least one, English plus German search words.

**Calculator:**
- [ ] vars in order; exactly `dof` input variables carry a default and form a solvable set; every non-input is a rule target.
- [ ] One explicit rule per direction; checks, derived values and the formula declared.
- [ ] `testdata/vectors/calculators/<id>.json` with at least 2 hand-checked `sets` per rule direction and 2 per option of every choice, each with a `ref` showing the arithmetic. Expected values come from exact arithmetic, a definition or a published constant, never from the solver. `logic_level_check` therefore needs 2 sets per driver and per receiver option. Add `solve` vectors for errors and warnings.
- [ ] The round trip passes for every option; `TestCalculateEveryCalculatorFromItsDefaults` passes.
- [ ] Schematic (if any): the category's file, ≤ 32 units, existing symbols only, chips clear of wires.

**Table:**
- [ ] Only cross-checked values ship; an UNVERIFIED cell is `null` (empty columns are hidden and counted on the page).
- [ ] `documentation/ecalc-sources/<group>/<id>.md` in the structure of `general/logic_levels.md`: sources with URLs, cross-check, deliberate deviations. Add a row in that group's `SUMMARY.md`.
- [ ] `guide: true` where the standard is authoritative and we quote makers; `mains` stays false for all five.

**Pinout:**
- [ ] Pins from the maker's document, or two independent sources. An unverified detail stays out.
- [ ] `testdata/vectors/pinouts/<id>.json`: exact `counts`, at least 3 pins checked by number, name and type, each with a `ref`.
- [ ] Record `documentation/ecalc-sources/pinouts/ics/<id>.md` in the `ne555.md` structure. A changed value updates record, YAML and vector together.

**Code:**
- [ ] Tables live in data; a single-source symbol goes into `not_cross_checked`. Go, `ecalc-codes.js` and `codes.json` change together.

## A.4 Risks and out of scope

**Risks:**
1. **E1 touches both solvers.** Mitigation: parity vectors for the dof-0 calculator, a tool test without inputs, and the browser smoke of the calc page. If E1 grows beyond the files listed in A.2, `logic_level_check` falls back to a four-input noise-margin calculator (voh, vol, vih, vil with nmh, nml), and the family values stay in the `logic_levels` table.
2. **Symbol changes reach every schematic.** A corrected ANSI or IEC drawing changes all calculator drawings in that standard. W2-Z runs the schematic chip check in both standards and looks at one contact sheet per standard.
3. **Schema budget.** One enum word costs tokens; the trim must not remove information the model needs. Put the removed words into the tool manual.
4. **Paywalled standards.** TIA-568, ISO/IEC 11801 and ISO 8820-3 cannot be read. Cells need two independent maker documents; otherwise they stay `null` and the record says why.
5. **Research gaps.** An item whose record is missing or UNVERIFIED is dropped and reported, never guessed. This applies to ULN2803, 74HC161/163 as one item, 24LCxx as one item, the IEC 60062 date code and every compatibility tag.
6. **`Load` is all-or-nothing.** Content batches own disjoint files, add one item at a time, run `go test` with `-skip TestCatalogSnapshot`, and never touch a file they do not own. At most three content agents run at once.
7. **Hot files at landing.** `ui/lang/desktop/*.json`, `module-loader.js` and the AGENTS files are edited by other sessions in the shared checkout. Rebase before landing; land with the keep-foreign script (`project_ecalc` memory, phase 4 lesson).

**Deliberately out of scope:**
- the digital calculators that need bit operations (two's complement, Gray, BCD, parity, bitwise, binary add/subtract, IEEE 754, CRC, Hamming), which come with expression-language work;
- the logic teaching tools, BOM and selection matrix, cross-reference links and PCB fab tiers;
- finders for the new tables;
- new symbols (only the 25 of the library get rows);
- new pinout shapes or packages;
- push and deploy.

---

# PART B — Execution plan

## B.0 Setup (orchestrator, serial)

1. Worktree from current `main`:
   ```
   git -C C:/Users/Andi/Documents/repo/AuraGo worktree add -b ecalc-p5 .worktrees/ecalc-p5 main
   ```
   All paths below are relative to `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/ecalc-p5`.
2. Copy a real `node_modules` into the worktree (a junction breaks a vendor check). Use the usual GOCACHE/GOTMPDIR on D:.
3. Baseline, and record any pre-existing failures in the task log:
   ```
   go test ./internal/ecalc -count=1
   npm run test:ecalc
   go test ./ui -count=1 -run '(?i)^TestECalc'
   go test ./internal/tools ./internal/agent -count=1 -run '(?i)ecalc'
   ```
4. Research folder `C:/disposable/ecalc-p5/research/{calc,tables,codes,parts}/`, outside the repo and out of reach of other sessions' worktrees.

**Rules for every task:**
- Commit only your own paths (`git add <paths>; git commit -m ...`). If `index.lock` exists, wait and retry; never delete it.
- `gofmt` your Go files.
- Before editing a function, run GitNexus `impact` from the main checkout and hand the result to the implementer (the worktree cannot be indexed).
- Expected test result: `ok  aurago/...` with no FAIL. Count the PASS lines of `-v -run` as evidence where named.
- Commits carry the line "GitNexus detect-changes unavailable in the worktree; judged by symbols and tests."

## Wave 0 — research (before the content of each wave)

Research agents with web access write **one record per item** to the research folder as they go. Lesson: Fable limits cut several agents at once; per-item records let SendMessage resume them. The controller saves each batch's `SUMMARY.md` from the agent's report, because the harness refuses report files from research agents. At most three run at once. Expect 400–600k tokens per agent.

| batch | items | record format | must answer |
|---|---|---|---|
| R1 `calc/` | stripline_impedance, diff_microstrip_impedance, diff_stripline_impedance, mosfet_logic_switch, bits_range | `C:/disposable/ecalc-ext/research/calc/avr_timer.md` | exact formulas with validity ranges from two sources (IPC-2141A, Wadell, maker application notes such as TI or Analog Devices); clean inverses; 2 hand-worked vectors per direction; the datasheet terms V_GS(th) and the R_DS(on) test condition from two MOSFET datasheets (e.g. IRLZ44N, IRF540N) |
| R2 `calc/` | logic_level_check | same | per driver family: VOH min at light load and VOL max; per receiver family: VIH min, VIL max and the input voltage limit (absolute maximum, 5-V tolerance of 74LVC) at nominal supply. Reuse `documentation/ecalc-sources/general/logic_levels.md` and name every added datasheet |
| R3 `tables/` | ethernet_categories, usb_versions | `documentation/ecalc-sources/general/logic_levels.md` | every cell with two sources or `UNVERIFIED`; which columns stay empty |
| R4 `tables/` | blade_fuses, level_shifters | same | two maker catalogues per colour; for each level-shifter row the datasheet that states direction, open-drain support and data rate |
| R5 `tables/` + `codes/` | schematic_symbols, date_code | symbols: one section per library symbol; date code: `documentation/ecalc-sources/codes/smd_inductor.md` | IEC 60617 vs IEEE 315 drawing per symbol (with the figure or clause that was actually seen); YYWW/WWYY maker examples, ISO 8601 week rules, IEC 60062 letter code yes/no |
| R6 `parts/` | P1 items | `C:/disposable/ecalc-ext/research/parts/sn74hc00.md` | pins from TI (and a second maker), shared-pinout claims only where a datasheet says so |
| R7 `parts/` | P2 and P3 items and edits | same | ULN2803A from TI or a second current maker; each compatibility tag backed by that part's own datasheet |

Order: R1 ∥ R2 ∥ R6 first (Wave 1 and Wave 3 can start from them), then R3 ∥ R4 ∥ R5, then R7.

## Wave 1 — calculators

**W1-E1 Choice-only calculators** · serial, first
- Owns: `internal/ecalc/catalog.go`, `internal/ecalc/solve.go`, their tests, `ui/js/desktop/apps/ecalc/ecalc-solve.js`, `ui/js/desktop/apps/ecalc/ecalc-calc-page.js`, `internal/tools/ecalc.go`, `internal/tools/ecalc_test.go`.
- Input: A.2 E1.
- Steps:
  1. Write the failing tests: the loader case, the solve case, and the tool case with a fixture calculator.
  2. Loader and `SolvableSets`.
  3. JS twin.
  4. Calc page without fields.
  5. Tool `compute` and `open` without inputs.
- Verify:
  ```
  go test ./internal/ecalc -count=1
  npm run test:ecalc
  go test ./internal/tools -count=1 -run '(?i)ecalc'
  go test ./ui -count=1 -run '(?i)^TestECalc'
  ```
  All pass.

**W1-C1 rf** ∥ **W1-C2 digital and semiconductors** · C1 may start before E1; C2 item #4 waits for E1
| task | owns | items |
|---|---|---|
| C1 | `calculators/rf.yaml`, `ecalc-schematics-rf.js`, vectors of #1–#3 | #1–#3 |
| C2 | `calculators/digital.yaml`, `calculators/semiconductors.yaml`, vectors of #4–#6 | #4–#6 |

- Input: `research/calc/<id>.md` and the A.1 row.
- Steps per item: vectors first (sets for every direction and option, with refs), then the YAML entry at the end of its file, then the schematic if planned; test; commit after every 1–2 green items.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -skip TestCatalogSnapshot
  go test ./internal/ecalc -count=1 -v -run '(?i)(TestCalculatorVectors|TestRoundTripAllCalculators)/<id>'
  go test ./ui -count=1 -run '(?i)^TestECalcSchematicsMatchTheCatalog$'
  ```

**W1-Z Closing** · serial
1. Snapshot with `-update`; review that the diff holds only the six items.
2. Checks:
   ```
   go test ./internal/ecalc -count=1
   go vet ./internal/ecalc/ ./internal/tools/
   gofmt -l internal/ecalc internal/tools      # empty
   npm run test:ecalc
   go test ./ui -count=1 -run '(?i)^TestECalc'
   go test ./internal/tools ./internal/agent ./internal/server -count=1 -run '(?i)ecalc'
   npm run check:ui
   ```
3. Browser smoke (Chrome from a binary outside the repo, `D:/tmp/go-build`, sandbox off):
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^TestDesktopECalcBrowser$'
   ```
   Look at `logic_level_check` through `AURAGO_BROWSER_ARTIFACT_DIR`: a page without fields must not look broken.
4. One combined review of the wave diff (Critical/Important only): the solver change in Go and JS, the honesty of sources and refs, and the physics of the formulas. Fix, then re-run step 2 only.
5. Docs (one docs agent): manual en/de, `internal/ecalc/AGENTS.md` (dof 0), UI AGENTS (calc page without fields), tool manual sentence.
6. Commits: `feat(ecalc): calculators whose inputs are choices`, `feat(ecalc): stripline and differential impedance`, `feat(ecalc): logic level, MOSFET switch and bit range calculators`, snapshot, docs.

## Wave 2 — tables and the date code

**W2-E2 Symbol column** ∥ **W2-E3 Date code kind** · disjoint files (A.2); the AGENTS files are left to W2-Z
- Before editing, run GitNexus impact on the table loader (`(*loader).columns`, `readCell`), the table page renderer, `CONTROLLERS`, the code kind registry and `CodeKindIDs`.
- E2 verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestTable|TestQueryTable|TestDefaultCatalogLoads)'
  go test ./ui -count=1 -run '(?i)^TestECalc(Table|Symbol)'
  ```
- E3 input: `research/codes/date_code.md`. Verify:
  ```
  go test ./internal/ecalc -count=1 -run '(?i)(TestCode|TestQuery(Decode|Encode))'
  npm run test:ecalc
  go test ./ui -count=1 -run '(?i)^TestECalcCode'
  go test ./internal/agent ./internal/tools -count=1 -run '(?i)ecalc'
  ```
  All pass, including the 800-token schema test.

**W2-T1 general tables** (after E2) ∥ **W2-T2 component tables**
| task | owns | items |
|---|---|---|
| T1 | `catalog/tables/general.yaml`, records in `documentation/ecalc-sources/general/`, its `SUMMARY.md`; `ecalc-symbols.js` drawings that R5 says are wrong | ethernet_categories, usb_versions, schematic_symbols |
| T2 | `catalog/tables/components.yaml`, records in `documentation/ecalc-sources/components/`, its `SUMMARY.md` | blade_fuses, level_shifters |

- Steps per table: record, then YAML, then the table test; commit per table.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -skip TestCatalogSnapshot -run '(?i)(TestTable|TestDefaultCatalogLoads)'
  ```

**W2-Z Closing**
1. Snapshot and the W1-Z checks.
2. Browser smoke, schematics in both standards:
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^(TestDesktopECalcBrowser|TestDesktopECalcTablesBrowser|TestDesktopECalcCodesMarkingsBrowser|TestDesktopECalcCodesSkinsBrowser)$'
   ```
   If the existing smoke draws schematics in one standard only, add a one-off run with the setting set to ANSI (harness: the phase 4 survey approach, `background-position` crops in one HTML sheet).
3. Review: table cells against their records, symbol drawings against R5, Go/JS parity of the date code.
4. Docs:
   - manual (five tables, the date code);
   - tool manual row;
   - `internal/ecalc/AGENTS.md` (the `symbol` dimension, seven code kinds);
   - UI AGENTS (symbol column, CONTROLLERS);
   - `codes/SUMMARY.md`.
5. Commits: `feat(ecalc): symbol columns in tables`, `feat(ecalc): component date codes`, one `feat(ecalc): <table> table` per table, snapshot, docs.

## Wave 3 — IC pinouts

**W3-P1 ∥ P2 ∥ P3** · any time after R6/R7; no engine task
| task | owns |
|---|---|
| P1 | `catalog/pinouts/ics_logic.yaml`, its new vectors and records |
| P2 | `catalog/pinouts/ics_analog.yaml` (incl. the `max232` edit), its new vectors and records, `max232.md` |
| P3 | `catalog/pinouts/ics.yaml` edits, `vectors/pinouts/to92.json`, records `ne555.md`, `lm358.md`, `sot223.md`, `to92.md` |

- Steps per item: record, then YAML, then vector; test; commit every 3 items.
- Verify:
  ```
  go test ./internal/ecalc -count=1 -skip TestCatalogSnapshot -run '(?i)(TestPinout|TestDefaultCatalogLoads)'
  ```

**W3-Z Closing**
1. Snapshot and the W1-Z checks.
2. Browser smoke:
   ```
   AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -count=1 -timeout 30m -run '(?i)^(TestDesktopECalcPinoutRealBrowser|TestDesktopECalcPinoutSkinsBrowser)$'
   ```
3. Review: pins against the records; compatibility claims against the named datasheets.
4. Docs: manual (pinout count and ICs table), `documentation/ecalc-sources/pinouts/SUMMARY.md`.
5. Rebase `ecalc-p5` on `main`. Land locally only with the user's OK, using the keep-foreign script. No push.

## B.1 Templates — copy from these (do not write new examples)

- **Closed-form calculator with inverse rules:** `calculators/rf.yaml` `microstrip_impedance`, vectors `testdata/vectors/calculators/microstrip_impedance.json`.
- **Derived-only calculator:** `calculators/semiconductors.yaml` `mosfet_losses`.
- **Choice consts:** `calculators/basics.yaml` `decibel_ratio`.
- **Forward-only with rounding and integers:** `calculators/digital.yaml` `uart_baud_error`.
- **Transcribed table, text and numeric columns:** `tables/general.yaml` `logic_levels`, record `documentation/ecalc-sources/general/logic_levels.md`; component table `tables/components.yaml` `fuse_speed_classes`.
- **Code kind:** `internal/ecalc/codes_smd_inductor.go`, the `smd_inductor` part of `ecalc-codes.js`, `catalog/codes/smd_inductor.yaml`, `documentation/ecalc-sources/codes/smd_inductor.md`, the `smd_inductor` cases in `testdata/vectors/codes.json`.
- **Pinout `dip`:** `pinouts/ics_logic.yaml` `sn74hc00`, with `vectors/pinouts/sn74hc00.json` and `documentation/ecalc-sources/pinouts/ics/sn74hc00.md`.
- **`to` with variants:** `pinouts/ics.yaml` `to92`, with `to92.json` and `to92.md`.

---

## GitNexus in the worktree (settled by the repo's earlier practice)

CLAUDE.md requires `impact` before edits and `detect_changes` before every commit, but the worktree cannot be indexed. The orchestrator therefore runs `impact` against the main checkout's index before each engine edit and hands the result to the implementer. Commits in the worktree carry the line above. `detect_changes` with `scope: compare` runs once from the main checkout after the branch has landed.

## Assumptions I made instead of asking

- **The logic-level check is a calculator whose inputs are choices (E1).** Families at nominal supply and light load, like the `logic_levels` table. If E1 turns out bigger than planned, the fallback is a four-voltage noise-margin calculator (A.4).
- **The level-shifter "selector" is a table.** ECalc has no recommendation engine, and a table with direction, open-drain support and data rate answers the same question for the user and the agent.
- **No finders** for the Ethernet and USB tables; the agent reads the rows.
- **Schematic symbols are the 25 of the library.** No new symbols; a drawing changes only where the research shows that our IEC or ANSI version is wrong.
- **Date codes**: two-digit years are read as 2000–2099 with a note; YYWW and WWYY are both shown when both are valid; the IEC 60062 letter code only if it is verified.
- **Pin-compatible parts are tags and notes** on the existing item. A new item only where pins or functions differ (ATtiny13A). TMP36 becomes a `to92` variant.
- **One item where the pinout is identical:** 74HC161/163 and 24LC32A/64/256, if the makers' figures agree pin by pin.
- **No new categories, groups or table groups.**
- **No push or deploy.** The work lands on local `main` only, with the user's OK.
