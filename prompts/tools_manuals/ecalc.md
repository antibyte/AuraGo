# ECalc (electronics calculators, reference tables and component codes)

Use `ecalc` for electronics formulas: Ohm's law, voltage dividers, filters, timers, op-amp stages, power supplies, RF, standard resistor values and more. It computes with the same catalog and formulas as the ECalc desktop app, exactly and with units; use it instead of calculating by hand. The catalog also holds reference **tables** (AWG wire sizes, PCB trace width after IPC-2221, E-series values and more) that you read with `table`, and **component codes** (resistor and inductor colour bands, SMD markings, capacitor and ceramic codes) that you read with `decode` and write with `encode`. It reads nothing from the network or from files.

## Workflow

1. `search` with a few words (English or German, at most 256 characters): a topic, a part or a formula. A hit has a `kind`: `calculator`, `table` or `code`. An empty `query` lists the categories.
2. `describe` the item you picked. A calculator: its variables with units and defaults, the choices, how many values are needed (`dof`), which input combinations work (`sets`), the computed outputs (`derived`), the checks and the formula. A table: its columns, key, row count, notes and source. A code: its `code_kind` and the forms it reads and writes.
3. `calculate` with the values you know (any complete input set works, so you can solve for whichever quantity is missing), `table` to read rows or one value of a table, `decode` to read a code, `encode` to write one.
4. `open` only when the user wants to see or play with the item; it opens the ECalc app on the desktop (pre-filled for a calculator).

```json
{"action":"ecalc","operation":"search","query":"voltage divider"}
```

```json
{"action":"ecalc","operation":"describe","id":"voltage_divider"}
```

```json
{"action":"ecalc","operation":"calculate","id":"voltage_divider","inputs":[{"name":"vout","value":"3.3 V"},{"name":"vin","value":"12 V"},{"name":"r1","value":"10k"}],"eseries":"E96"}
```

```json
{"action":"ecalc","operation":"calculate","id":"eseries_combiner","inputs":[{"name":"target","value":"5k"}],"eseries":"E24"}
```

```json
{"action":"ecalc","operation":"table","id":"awg","filter":"1/0"}
```

```json
{"action":"ecalc","operation":"table","id":"awg","lookup":{"column":"awg","value":"12","output":"diameter"}}
```

```json
{"action":"ecalc","operation":"table","id":"ipc2221","lookup":{"mode":"interp","column":"current","value":"1.25 A","output":"ext_1oz_10"}}
```

```json
{"action":"ecalc","operation":"decode","kind":"resistor_bands","code":"brown black red gold"}
```

```json
{"action":"ecalc","operation":"encode","kind":"capacitor","value":"100n","tolerance":"K","voltage":"50V"}
```

```json
{"action":"ecalc","operation":"open","id":"rms_waveform","inputs":[{"name":"vrms","value":"230 V"}],"choices":[{"name":"wave","option":"triangle"}]}
```

## Operations

- `search`: `query`, optional `limit` (1-25, default 10). Results hold `id`, `kind`, `name`, `category` and a one-sentence `summary`, best match first; `total` is the number of matches and `truncated: true` means more exist than were returned.
- `describe`: `id` of a calculator, table or code. Calculator vars have `id`, `label`, `name`, `dim`, `unit` (the unit a bare number is read in; empty means the SI unit), `units` (units a value may carry), `input`, `eseries` and the `default`. A table has `columns` (`id`, `name`, `dim`, `unit`, `units`, `key`), `key`, `row_count`, `notes`, `source`, `guide`/`mains` with a `caution`, and a `finder` when a calculator selects its rows. A code has `code_kind` (the `kind` for decode and encode), `decode` (the forms it reads), `encode` (the request fields) and `fields`.
- `calculate`: `id`, `inputs`, optional `choices` and `eseries` (E3, E6, E12, E24, E48, E96 or E192; default E24).
- `table`: `id` and optionally `filter`, `offset`, `limit` and `lookup` (see Tables).
- `decode`: `kind` and `code` (see Codes).
- `encode`: `kind`, `value` and optionally `bands`, `tolerance`, `tempco`, `voltage` and `class` (which of them a kind reads: see Codes).
- `open`: `id` of any item (optional), optional `inputs` and `choices` (calculators only). Without `id` it only opens the app. The values are not checked against each other; the app shows them as typed.

## Values

- Write values the way a person types them: `4700`, `4.7k`, `4k7`, `4R7` (resistance), `2u2`, `10mA`, `12 V`, `25 °C`, `-10 dBm`. A decimal comma works too.
- A bare number is in the variable's default unit from `describe` (`unit`; the SI unit when it is empty). `2` for a capacity shown in Ah is 2 Ah; write `2000 mAh` to be explicit.
- A list variable takes `"4k7, 10k, 2k2"`: separate the items with a comma and no space between a number and its unit.
- `name` is a variable id from `describe`; case does not matter and the label (`V_in`) works too. Choices (a waveform, a material) are `{name, option}` with ids from `describe`; a choice you leave out uses its default.
- A number system (`dim: integer`, units `dec`, `hex`, `bin`, `oct`) is read in the unit of its variable: a `hex` variable takes hex digits with or without `0x`, a `bin` variable binary digits with or without `0b`; the result is shown the same way (`0xFF`). Whole numbers up to 2^53 - 1 only.
- Give at least `dof` values forming one of the `sets`. With more values the first solvable combination is used and the others are checked against the result.

## Reading the result of calculate

- `quantities` lists every variable and computed output with `si` (SI base units), `formatted` (4 significant digits in the variable's display unit) and `computed` (false for what you gave). Report `formatted` to the user.
- `eseries` on a computed resistor is the nearest standard value of the chosen series with its deviation in percent; offer that value when the user wants to buy or build the circuit.
- A choice option may ignore variables (`ignore` in `describe`, `ignored: true` in the result): you need not give them, and when the values you give do not make a solvable set their defaults complete it (`defaulted: true`).
- `combos` (the standard-value combiner `eseries_combiner`, finder `combine`) lists the nearest single standard value and the best series and parallel pairs of the chosen series for the target resistance: `kind` single, series or parallel, `r1`, `r2`, the combined `value` and `dev_pct`, the deviation from the target in percent.
- `warnings` are the calculator's own checks (for example a divider output above its input); mention them.
- `contradictions` lists a redundant value that differs from the computed one by more than 1 %; tell the user their numbers do not agree and which one you computed from (`basis`).
- A quantity with `error` (`range`, `nonfinite`) could not be computed from these values, for example a division by zero; the answer has `status: error` and `code: calculation_failed` then.

## Tables

- `table` lists rows: `columns` names the columns (`id`, `name`, `dim`, `unit`, `key`) and every row in `rows` is an array of cells in that order. A cell is text with its unit (`"2.5 mm²"`, `"19.5 A"`) or `null` for an empty cell; a column whose `unit` is empty shows each number with its own prefix. `si` repeats the numbers in SI base units (m, m², Ω, A, K) for further calculation; it is `null` for text and empty cells. A table transcribed from a standard shows its numbers with every digit they were written with (`434.79 MHz`, `2.4835 GHz`); the computed tables (AWG, IPC-2221 widths, E-series) show 4 significant digits.
- `total` counts the rows that match the `filter`, `returned` the rows listed from `offset`. `truncated: true` with `next_offset` means more rows follow: call again with `offset` set to it. `limit` is 1-50, default 20.
- `filter` keeps the rows with a cell that contains the text, ignoring case: numbers as they are written in the rows, text cells in English or German (`"PVC"`, `"1/0"`).
- `lookup` is `{column, value, mode, output}`. `value` is typed like an input (`"12"`, `"2.5 mm2"`, `"70 °C"`); a bare number is in the unit of the column. `mode` is `exact` (default: the row where `column` equals `value`; without `column` the key column; a text column compares text), `ge` (the first row in table order whose `column` is at least `value`) or `interp` (linear between the neighbouring rows; `column` must rise or fall steadily and `output` is required). `output` names the column to return; without it exact and ge answer with the whole row.
- The `lookup` answer has `found`, `value` (as it was understood, with its unit), `row` (formatted, in the order of `columns`), `result` and `si` of the `output` column, and a `message` when nothing was found (a lookup that finds nothing is an answer, not an error). `ge` takes the first row in table order: a table that runs from large to small (AWG from 4/0 to 40) answers with its first row whenever that qualifies, so read the rows when you need the smallest fit.
- With a `lookup` and no `filter`, `offset` or `limit` only the lookup is answered and no rows are listed.
- `guide: true` marks values transcribed from a standard as a guide: the standard itself is authoritative. `mains: true` marks installation data: the national rules and a qualified person decide. Pass the `caution` on to the user and name the `source` when they rely on a value. `notes` explain conditions and assumptions of the table.
- Answers are shortened to fit the output limit and the `note` says what was left out: first `si`, then fewer rows are listed (`truncated`, continue at `next_offset`); with a very small limit also the column names, the notes and the source, and as the last resort the rows themselves (`returned: 0`: read single cells with a `lookup` that names its `output` column). A table page is shortened, never refused for its size. Use `filter` or `lookup` instead of asking for more rows.

## Codes

- `decode` reads a code, `encode` writes one. `kind` is the `code_kind` from `describe` of a code item (the id of the code item works too).
- `offset`, `limit` and `bands` are numbers.
- Write band colours as English colour names separated by blanks, commas or dashes (`brown black red gold`; case does not matter) and markings as printed: `103`, `4R7`, `01C`, `104K 1H`, `4p7`, `X7R`, `C0G`.
- `encode` takes the value the way a person types it (`4k7`, `100n`, `6.8u`) and chooses the nearest standard value of the series that the tolerance implies (20 % E6, 10 % E12, 5 % E24, 2 % E48, 1 % E96, 0.5 % and tighter E192); a changed value is named in `warnings`. Without a tolerance 5 % is assumed (a warning says so). `tolerance`, `tempco` (`"50ppm"`), `voltage` (`"50V"`) and `class` (`"1"` or `"2"`) are written as text; a number is read as typed.
- The kinds, what `decode` reads, what `encode` takes and the ids of the `fields`:

| `kind` | `decode` | `encode` | `fields` |
| --- | --- | --- | --- |
| `resistor_bands` | 1 to 6 colours (`blue grey red` = 6.8 kΩ ±20 %) | `value`, `tolerance`, `tempco` (6 bands), `bands` 3 to 6 | `tolerance`, `tcr` |
| `inductor_bands` | 4 colours (EIA) or 5 (MIL, silver first) | `value`, `tolerance` 5 or 10 %, `bands` 4 or 5 | `layout`, `tolerance` |
| `smd_resistor` | `472`, `4R7`, `01C` (EIA-96), `1L5`, `000` | `value`, `tolerance` (picks the 3-digit, 4-digit or EIA-96 marking) | `family`; encode adds `code_3digit`, `code_4digit`, `code_eia96` |
| `capacitor` | up to three parts: `104`, `4p7`, `K`, `1H` (`104K 1H` = 100 nF ±10 %, 50 V) | `value`, `tolerance` (letter or %), `voltage` | `family`, `tolerance`, `voltage`; encode adds `value_code`, `rkm` |
| `ceramic_temp` | `X7R`, `C0G`, `NP0`, `U2J` | `class` (required); class 1: `tempco`, `tolerance`; class 2: `value` as a range (`-55..125`), `tolerance` (`15%`) | class 1: `class`, `tc`, `tolerance`, `industry_code`; class 2: `class`, `temp_low`, `temp_high`, `change` |

- The answer has `kind`, `code` (the canonical text), `bands` (colour ids), `value` (SI) with `formatted`, `dim`, `fields` (`id`, `name`, `text` and `value` where it is a number) and `warnings`. A code without a value (a class 2 ceramic code) has none of `value`, `dim` and `formatted`. Pass the `warnings` on: they say that an older tolerance reading exists, that voltage codes are a maker convention (check the datasheet), or that a symbol is ambiguous between makers.

## Errors

- `unknown_id`: the message suggests close matches; search first. `not_a_calculator`, `not_a_table` and `not_a_code`: the id names an item of another kind; the message says which operation reads it.
- `unknown_variable`, `not_an_input` (a computed output cannot be given), `unknown_choice`, `unknown_option`, `unknown_eseries`, `duplicate_name`, `too_many_values`: the message lists the valid names; fix the call.
- `parse_error`: the text of one value could not be read with the unit of its variable or column; the message names the variable or column and its dimension.
- `underdetermined`: too few values, or no combination of them is a solvable set; `sets` lists the input sets closest first and the message says what to add.
- `unknown_column`, `invalid_lookup` (bad mode, no value, missing output, a text column in ge or interp, a column that is not steady for interp), `invalid_filter` (more than 128 characters): the message lists the columns or the valid forms.
- `unknown_code_kind`: the message lists the kinds. `invalid_code`: the code to decode could not be read (the message says why: an unknown colour, a wrong number of bands, more than 64 characters). `invalid_value`: a field of an encode request could not be read or contradicts another (a value that is no number, `bands` out of range, nothing to encode). `unsupported`: well formed, but not decodable or not codable, for example a symbol that makers disagree on or a tolerance without a band colour.
- `invalid_operation`: use `search`, `describe`, `calculate`, `table`, `decode`, `encode` or `open`.
- `policy_denied` with `desktop_unavailable` or `desktop_agent_control_disabled` (open only): the desktop is off or the user has not allowed the agent to control it. Give the numbers from `calculate` or `table` instead and say that the user can open ECalc themselves.
- `output_budget_exceeded`: a complete calculation cannot fit the available response budget. Do not interpret it as a partial numeric result. Describe/search responses may omit optional explanatory detail.
- Answers are shortened to fit the output limit; `truncated: true` marks it. Search again with more specific words or fewer results rather than asking for more.
