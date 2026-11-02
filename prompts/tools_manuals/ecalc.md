# ECalc (electronics calculators)

Use `ecalc` for electronics formulas: Ohm's law, voltage dividers, filters, timers, op-amp stages, power supplies, RF, standard resistor values and more. It computes with the same catalog and formulas as the ECalc desktop app, exactly and with units; use it instead of calculating by hand. It reads nothing from the network or from files.

## Workflow

1. `search` with a few words (English or German): a topic, a part or a formula. An empty `query` lists the categories.
2. `describe` the calculator you picked: its variables with units and defaults, the choices, how many values are needed (`dof`), which input combinations work (`sets`), the computed outputs (`derived`), the checks and the formula.
3. `calculate` with the values you know. Any complete input set works, so you can solve for whichever quantity is missing.
4. `open` only when the user wants to see or play with the calculator; it opens the ECalc app on the desktop, pre-filled with your values.

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
{"action":"ecalc","operation":"open","id":"rms_waveform","inputs":[{"name":"vrms","value":"230 V"}],"choices":[{"name":"wave","option":"triangle"}]}
```

## Operations

- `search`: `query`, optional `limit` (1-25, default 10). Results hold `id`, `name`, `category` and a one-sentence `summary`, best match first; `total` is the number of matches and `truncated: true` means more exist than were returned.
- `describe`: `id`. Vars have `id`, `label`, `name`, `dim`, `unit` (the unit a bare number is read in; empty means the SI unit), `units` (units a value may carry), `input`, `eseries` and the `default`.
- `calculate`: `id`, `inputs`, optional `choices` and `eseries` (E3, E6, E12, E24, E48, E96 or E192; default E24).
- `open`: `id` (optional), optional `inputs` and `choices`. Without `id` it only opens the app. The values are not checked against each other; the app shows them as typed.

## Values

- Write values the way a person types them: `4700`, `4.7k`, `4k7`, `4R7` (resistance), `2u2`, `10mA`, `12 V`, `25 °C`, `-10 dBm`. A decimal comma works too.
- A bare number is in the variable's default unit from `describe` (`unit`; the SI unit when it is empty). `2` for a capacity shown in Ah is 2 Ah; write `2000 mAh` to be explicit.
- A list variable takes `"4k7, 10k, 2k2"`: separate the items with a comma and no space between a number and its unit.
- `name` is a variable id from `describe`; case does not matter and the label (`V_in`) works too. Choices (a waveform, a material) are `{name, option}` with ids from `describe`; a choice you leave out uses its default.
- A number system (`dim: integer`, units `dec`, `hex`, `bin`, `oct`) is read in the unit of its variable: a `hex` variable takes hex digits with or without `0x`, a `bin` variable binary digits with or without `0b`; the result is shown the same way (`0xFF`). Whole numbers up to 2^53 - 1 only.
- Give at least `dof` values forming one of the `sets`. With more values the first solvable combination is used and the others are checked against the result.

## Reading the result

- `quantities` lists every variable and computed output with `si` (SI base units), `formatted` (4 significant digits in the variable's display unit) and `computed` (false for what you gave). Report `formatted` to the user.
- `eseries` on a computed resistor is the nearest standard value of the chosen series with its deviation in percent; offer that value when the user wants to buy or build the circuit.
- A choice option may ignore variables (`ignore` in `describe`, `ignored: true` in the result): you need not give them, and when the values you give do not make a solvable set their defaults complete it (`defaulted: true`).
- `combos` (the standard-value combiner `eseries_combiner`, finder `combine`) lists the nearest single standard value and the best series and parallel pairs of the chosen series for the target resistance: `kind` single, series or parallel, `r1`, `r2`, the combined `value` and `dev_pct`, the deviation from the target in percent.
- `warnings` are the calculator's own checks (for example a divider output above its input); mention them.
- `contradictions` lists a redundant value that differs from the computed one by more than 1 %; tell the user their numbers do not agree and which one you computed from (`basis`).
- A quantity with `error` (`range`, `nonfinite`) could not be computed from these values, for example a division by zero; the answer has `status: error` and `code: calculation_failed` then.

## Errors

- `unknown_id`: the message suggests close matches; search first.
- `unknown_variable`, `not_an_input` (a computed output cannot be given), `unknown_choice`, `unknown_option`, `unknown_eseries`, `duplicate_name`, `too_many_values`: the message lists the valid names; fix the call.
- `parse_error`: the text of one value could not be read with the unit of its variable; the message names the variable and its dimension.
- `underdetermined`: too few values, or no combination of them is a solvable set; `sets` lists the input sets closest first and the message says what to add.
- `invalid_operation`: use `search`, `describe`, `calculate` or `open`.
- `policy_denied` with `desktop_unavailable` or `desktop_agent_control_disabled` (open only): the desktop is off or the user has not allowed the agent to control it. Give the numbers from `calculate` instead and say that the user can open ECalc themselves.
- Answers are shortened to fit the output limit; `truncated: true` marks it. Search again with more specific words or fewer results rather than asking for more.
