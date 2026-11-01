# ECalc Domain

## Purpose

`internal/ecalc` is the engine of the ECalc Desktop app, an electronics formula collection: an embedded declarative catalog (YAML), a small safe expression language, unit parsing and formatting, E-series lookup, a rule propagation solver and the localized catalog JSON. It has no server dependency; the HTTP route, the Desktop app and the later agent tool all use it directly.

## Ownership

- Own `internal/ecalc/` and its tests. File map:
  - `expr.go`: lexer, parser, AST (`Node`, `Sexp`), `Eval`, `Deps`, `ParseFormula`.
  - `units.go` + `catalog/units.yaml`: prefixes, dimensions, explicit and non-linear units, `Parse`, `ParseList`, `Format`.
  - `eseries.go`: IEC 60063 tables, `Nearest`, `roundSig`.
  - `solve.go`: `Solve`, `SolvableSets`.
  - `catalog.go` + `catalog/categories.yaml` + `catalog/calculators/*.yaml`: schema, strict `Load`, `Default()` (embedded, validated once).
  - `localize.go`: `LocalizedJSON(lang)`, ETag, per-language cache.
  - `yamlutil.go`: strict YAML decoding helpers.
  - `testdata/vectors/core.json`, `testdata/vectors/calculators/<id>.json`: shared vectors; `testdata/catalog.en.json`: generated snapshot.
- Owned elsewhere: `GET /api/ecalc/catalog` and its audit contract (`internal/server`, `internal/audit`), the Desktop registration (`internal/desktop`), the app, schematics and the JavaScript mirror (`ui/js/desktop/apps/ecalc/`, `ui/css/desktop-app-ecalc.css`, `scripts/test-ecalc-core.mjs`), the manual chapter (`documentation/manuals/*/32-ecalc.md`) and the translated app strings (`ui/lang/desktop/*.json`). Tables, codes, pinouts, `lookup`/`interp` and the agent tool are later phases of `docs/superpowers/specs/2026-11-01-ecalc-design.md`.

## Local Contracts

- Every value is stored and solved in SI base units. Display units and prefixes exist only in `units.yaml`, `Parse` and `Format`.
- A calculator is declared, never coded: `vars` (ordered), optional `choices`, `dof`, one explicit `rules` expression per direction (a list gives alternatives; the first whose names are all known wins), `derived`, `checks`, `formula`, optional `schematic`. The engine does no symbolic algebra. Exactly `dof` input-capable variables carry a default, and the default inputs must be one of the solvable input sets.
- `Load` is strict (unknown YAML fields are errors) and reports every problem at once. Item ids are unique across kinds, `name` is written in all 16 desktop locales (`Locales`), `doc`, variable and derived names and check warnings in English and German (other languages fall back to English), and every item has a `source`.
- The expression language is `expr.go`: a fixed function whitelist, no assignment or loops, IEEE-754 doubles (division by zero gives `+-Inf`/NaN, never an error). Adding or changing a function, operator or unit rule is a change to the Go code, the JavaScript mirror and the shared vectors together.
- Go/JS parity: the expression language, units, E-series and solver exist twice, here and in `ui/js/desktop/apps/ecalc/` (`ecalc-expr.js`, `ecalc-units.js`, `ecalc-eseries.js`, `ecalc-solve.js`). Both run the same vectors; `scripts/test-ecalc-core.mjs` (`npm run test:ecalc`) runs them against the JavaScript side. Behavior that can differ between the languages is pinned by a vector: `roundSig` rounds exact ties away from zero like `Number(x.toPrecision(n))`, `round` is half away from zero, `if` takes the else branch for NaN, `fmtNum` writes numbers like `String(x)`.
- `LocalizedJSON` normalizes the language (unknown or empty means `en`) and caches by the normalized language, so the cache never exceeds the 16 locales; the body and ETag of `en`, `xx` and `""` are identical. Collections in the JSON are never `null`.
- Content honesty: `source` names a standard by number or a well-known work at book or chapter level only, never an invented page, URL or application-note number. Every rule direction is covered by at least two hand-checked sets (two per option of every choice); expected values come from exact arithmetic, a definition or a published constant, never from running the solver. Every calculator must pass the round-trip test (defaults, then every solvable input set, for every choice option).
- E-series: E3 to E24 are the IEC 60063 hand-picked values; E48 and E96 are 10^(k/N) rounded to three digits; E192 is the same except 9.20 at index 185. `eseries_tables` in `core.json` publishes them for the JavaScript mirror.

## Work Guidance

- To add or change a calculator: edit `catalog/calculators/*.yaml` (quote every expression and text string; use the micro sign U+00B5 and the Greek omega U+03A9, never the look-alike code points), add or update `testdata/vectors/calculators/<id>.json`, run the tests, regenerate the snapshot with `go test ./internal/ecalc -run TestCatalogSnapshot -update` and review its diff, then update the UI schematic id and the manual when the change is visible to users.
- `testdata/catalog.en.json` is generated; never edit it by hand. A stale snapshot fails `TestCatalogSnapshot`.
- Run `gofmt` on Go files before closeout.

## Verification

- `go test ./internal/ecalc -count=1` runs the unit tests, the shared vectors, the round trip over all calculators and the snapshot check.
- `go vet ./internal/ecalc/` and `gofmt -l internal/ecalc` must print nothing.
- `npm run test:ecalc` runs the shared vectors against the JavaScript mirror.

## Child DOX Index

None.
