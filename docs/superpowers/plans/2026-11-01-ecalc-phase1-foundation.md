# ECalc Phase 1 (Foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the foundation of the ECalc desktop app: the `internal/ecalc` core (expression language, units, E-series,
solver, validated YAML catalog) with the 11 basics calculators, the catalog API, the app registration, and a polished
calculator UI in layout C (fields on the schematic, typeset formula with live values), with Go/JS parity tests.

**Architecture:** A declarative YAML catalog embedded in the Go package `internal/ecalc` is validated at load time and
served localized by `GET /api/ecalc/catalog?lang=xx`. The same small expression language, unit parser/formatter,
E-series logic and rule-propagation solver exist in Go (for the server, later the agent tool) and as a DOM-free JS mirror
in `ui/js/desktop/apps/ecalc/` (instant in-browser calculation); shared vector files in `internal/ecalc/testdata/vectors/`
and a generated catalog snapshot keep both implementations identical.

**Tech Stack:** Go 1.27 (`gopkg.in/yaml.v3`, `go:embed`, `math/big`), vanilla JS desktop app modules loaded by the desktop
module loader, Node test scripts (`scripts/test-*.mjs`), rod browser tests gated by `AURAGO_RUN_BROWSER_SMOKE=1`.

**Spec:** `docs/superpowers/specs/2026-11-01-ecalc-design.md` (commit a323eae0a). This plan covers spec phase 1 only.

---

## How this plan was made and verified

The plan has three parts, drafted in parallel against one shared contract (appendix) and each implemented in its own
scratch worktree at base commit `1fb47da49`:

- **Part A** (Go core, tasks A1–A10): every code block was generated from the verified worktree commits; a mechanical
  replay of the plan text on a fresh checkout of the base commit reproduced the worktree byte for byte (all expected-FAIL
  runs failed, all PASS runs passed). `go test ./internal/ecalc/` green (24 tests + 11 calculator subtests).
- **Part B** (route, audit, registration, i18n, manual, tasks B1–B6): every old→new edit was re-applied to the base commit
  and compared byte for byte with the worktree (all 16 locale files included). Focused `internal/server`, `internal/audit`,
  `internal/desktop` and `./ui` tests green; full `internal/server` green.
- **Part C** (desktop UI, tasks C1–C6): code blocks generated from the verified worktree files, edits checked byte for byte
  against the base commit. Re-verified with Part A's and Part B's real files: Node parity test 686 cases ok, schematics
  cross-check against `ecalc.Default()`, `npm run build:ui -- --check` clean, guard tests green, browser test green.

Each part starts with its own **Contract deviations** list. Where a part's deviation list and the appendix disagree, the
part wins.

## Execution order

One branch (for example `ecalc-p1` in `.worktrees/ecalc-p1`, created from `main`). Run the tasks in this order:

1. **A1 → A10** (Go core and content; no dependency).
2. **B1 → B6** (B1/B2 need Part A; B3 precedes B4 so `TestDesktopBuiltinAppNamesExistInAllLanguages` never goes red).
3. **C1 → C6** (C1/C2 need Part A; C3's last test, C5 and C6 need Part B).

The plan was written against base `1fb47da49`; `main` has moved since. Line numbers in "Modify" entries refer to the
base commit — anchor on the quoted text, not on the number. After rebasing, rebuild the desktop bundle
(`npm run build:ui`, then `npm run build:ui -- --check`) because `main.bundle.js` is generated.

## Scope decisions inside phase 1 (relative to the spec)

- Phase 1 ships 11 basics calculators (the spec says "about ten"); the standard-value combiner moves to phase 2.
- `lookup` / `interp` move to phase 3, together with the tables they read.
- English-name search fetches `?lang=en` once in the background (Part C deviation 3). Follow-up for phase 2: add a
  `name_en` field to the catalog JSON and drop the second request.
- The app icon is app-owned artwork via `metadata.logo_path` (Part B deviation 1), not a themed icon.

## Known failures unrelated to this plan (present at the base commit)

- `go test ./ui`: `TestChatFrontend_IntegrationsDrawerRemainsWired`, `TestFrontend_StaticI18nKeysExistInEnglishBundle`
  (FindIt keys).
- `go test ./internal/audit` (whole package): `TestUpdateConfigFallbackOnlyTrustedTransactionBackups` on Windows.
- `internal/server` Video Studio test with `GOTMPDIR` on `D:` and the checkout on `C:` (cross-drive `filepath.Rel`).

## Landing notes (shared checkout with other sessions)

- Other sessions keep uncommitted edits in the main checkout. At plan time `AGENTS.md` (root, a GitNexus rewrite) and
  `ui/js/desktop/apps/AGENTS.md` were dirty there and are also touched by this plan (A10, C5). Before landing run
  `git -C <main checkout> status --short -- <paths of the branch diff>` and use the keep-foreign landing procedure.
- `.gitignore` ignores every `AGENTS.md` except the root one: the new `internal/ecalc/AGENTS.md` needs `git add -f`
  (Task A10 says so). The tracked `internal/server/AGENTS.md`, `internal/desktop/AGENTS.md` and
  `ui/js/desktop/apps/AGENTS.md` are added normally.
- Root `AGENTS.md` may carry the assume-unchanged flag in the main checkout (`git ls-files -v AGENTS.md` shows `h`);
  Task A10 explains how to clear it.
- Before every commit run the GitNexus change check required by `CLAUDE.md`
  (`node .gitnexus/run.cjs detect-changes --scope all --repo .`).

---

## Part A — Go core (`internal/ecalc`)

Contract deviations:

1. `roundSig` (CONTRACT §4/§5) rounds *exact* decimal ties away from zero instead of calling `strconv.FormatFloat` and `ParseFloat`, which round ties to even (12345 to 4 digits is 12340 with `strconv` but 12350 with JavaScript's `Number(x.toPrecision(4))`). It works on the exact binary value, so every value that is not an exact tie gives the same result as the contract formula. Checked against Node on 120 054 random values and ties: 0 mismatches. The JavaScript mirror keeps `Number(x.toPrecision(n))` unchanged.
2. Two extra files beyond CONTRACT §1: `internal/ecalc/yamlutil.go` (strict-YAML helpers, no exported API) and `internal/ecalc/AGENTS.md` (the DOX guide, Task A10, which also adds one row to the root `AGENTS.md` Child DOX Index).
3. `Load` is stricter than §2 spells out: every var and derived quantity needs a non-empty `label`; `tags` and `formula` need at least one entry; a choice has at least two options and every option defines the same consts; a variable that cannot be an input must be a rule target; defaults must be finite and greater than zero for positive dimensions.
4. Additions the contract left open: the vector file formats (below), and a `eseries_tables` section in `core.json` so the JavaScript mirror can verify its copy of the E-series tables.

GitNexus: Part A creates a new package and modifies no existing repository symbol, so no upstream impact analysis is needed. Before every commit run `node .gitnexus/run.cjs detect-changes --scope all --repo .` and expect only new files under `internal/ecalc/`.

### How to run Part A

All paths are relative to the worktree root; every Go command runs from there. Before the first Go command in a PowerShell session (CONTRACT §0):

```powershell
$env:GOTMPDIR = 'C:\tmp\go-build-ecalc'
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
```

Every code block below is the exact content of the scratch-worktree file it creates, or the exact text it replaces. After each task `gofmt -l internal/ecalc` printed nothing, `go vet ./internal/ecalc/` was clean and `go test ./internal/ecalc/ -count=1` passed, so every task leaves the package green. Run `gofmt -l internal/ecalc` and `go vet ./internal/ecalc/` before each commit.

### What Parts B and C can rely on

Exported Go API of package `aurago/internal/ecalc`:

```go
func Default() (*Catalog, error)                  // embedded catalog, loaded and validated once (sync.Once)
func Load(fsys fs.FS) (*Catalog, error)           // units.yaml, categories.yaml, calculators/*.yaml at the root of fsys
func (c *Catalog) LocalizedJSON(lang string) (body []byte, etag string, err error)
func (c *Catalog) Calculator(id string) *Calculator
func (c *Calculator) Solve(in Inputs) Result      // Inputs{Values, Lists map; Choices map[string]string}, SI units
func (c *Calculator) SolvableSets() [][]string
func (c *Calculator) Var(id string) *Var
func ParseExpr(src string) (*Node, error)
func ParseFormula(src string) (*Node, error)
func Eval(n *Node, env Env) (float64, error)      // Env{Vals map[string]float64; Lists map[string][]float64}
func Deps(n *Node) []string
func (n *Node) Sexp() string
func (u *Units) Parse(input, dimID, unitID string) (float64, error)
func (u *Units) ParseList(input, dimID, unitID string) ([]float64, error)
func (u *Units) Format(v float64, dimID, unitID string, sig int) string
func (u *Units) Dim(id string) *Dim
func Nearest(series string, v float64) (value, devPct float64, err error)
func ESeriesValues(series string) ([]float64, bool)
var ESeriesIDs []string                           // E3 E6 E12 E24 E48 E96 E192
var Locales []string                              // the 16 desktop locales
const ErrUndetermined, ErrNonFinite, ErrRange     // "undetermined", "nonfinite", "range"
```

`LocalizedJSON` normalizes the language first (case and region dropped, so `de-DE`, `DE_de` and ` de ` are `de`; `nb` and `nn` are `no`; anything that is not one of the 16 locales, including the empty string, is `en`) and caches by the *normalized* language, so `?lang=<anything>` can never grow the cache beyond 16 entries. The body and ETag for `en`, `xx` and `""` are byte-identical, and the `lang` field echoes the normalized language. Do not modify the returned body.

Catalog JSON quirks (CONTRACT §7 holds; these are the details it leaves open):

- The body is compact JSON without a trailing newline, and `<` and `>` are not escaped (`"when":"vout >= vin"`). The ETag is `"ecalc-<16 hex>"` including the quotes.
- Top-level keys are `version`, `lang`, `units`, `categories`, `items`. Every collection is an array and never `null`; only a variable's `default` and `default_list` are `null` when there is none (a list variable has `default: null` and a `default_list`).
- `units.prefixes[]` is `{id, factor}`; `units.dims[]` is `{id, symbol, aliases, prefixes, positive, units}`; an explicit unit is `{id, symbol, factor, to, from}`: `factor > 0` and empty `to`/`from` for a linear unit, `factor: 0` and the two expressions (variable `x`) for a non-linear one. A `""` entry in `prefixes` means the bare base unit. Go writes small numbers like `0.000001` for 1e-6.
- `consts` is always an object, `schematic` is `""` when a calculator has none, and `sets` lists the solvable input sets in declaration order.

Semantics the JavaScript mirror must reproduce (all pinned by the shared vectors):

- `round(x)` is half away from zero: use `Math.sign(x) * Math.round(Math.abs(x))` in JavaScript (`Math.round(-2.5)` is `-2`). `if(c, a, b)` takes `b` when `c` is `0` or NaN, and evaluates only the branch it takes. Comparisons give `1` or `0`; NaN compares false. `log10` of an exact power of ten is exact (JavaScript `Math.log10` already is). `sum`/`par` take a list variable only; `par([5, 0])` is `0`.
- `Format` stepping, `roundSig` and `fmtNum` as in `units.go` and `eseries.go` below; the number text is `String(x)`-compatible, including `1e-7` and `1e+21`.
- `Parse` multiplies by the prefix factor (`4.7 * 1e3`), so compare parse results with a relative tolerance of 1e-12. `ParseList` splits on commas, semicolons and white space, so an item with a separate unit such as `4.7 kΩ` is not supported inside a list.
- `Solve` ignores inputs that are not primary variables, falls back to the choice's default option for an unknown option id, reports `undetermined` for every unknown primary variable, and turns a non-finite derived value into NaN without an error code.

Shared vector formats (both under `internal/ecalc/testdata/vectors/`):

- `core.json`: top-level sections `expr`, `eval_error`, `parse`, `parse_error`, `formula`, `formula_error`, `deps`, `round_sig`, `eseries`, `eseries_error`, `eseries_tables`, `units_parse`, `units_parse_error`, `units_list`, `units_list_error`, `units_format`. Each section has a `_doc_<section>` string next to it that states what a vector means. A numeric `want` may be the string `"nan"`, `"inf"` or `"-inf"`. Numbers match at relative tolerance 1e-12 (absolute 1e-12 when the expected value is 0).
- `calculators/<id>.json`: `{"calc", "sets", "solve"}`. A *set* is a hand-checked consistent solution (`values` for every variable and derived quantity, arrays for list variables, optional `choices`, a `ref`): every rule alternative must reproduce its target from the set's values, and every solvable input set must reproduce the whole set through `Solve` (relative tolerance 1e-9). A *solve* vector is one `Solve` call: `inputs` (SI numbers, arrays for lists), optional `choices`, `expect` (a subset of values), the exact `errors` map and the exact `warnings` list of check indexes.

### Task A1: Expression language (parser, AST, evaluator)

**Files:**
- Create: `internal/ecalc/doc.go`
- Create: `internal/ecalc/expr.go`
- Test: `internal/ecalc/helpers_test.go`
- Test: `internal/ecalc/expr_test.go`
- Test: `internal/ecalc/testdata/vectors/core.json`

- [ ] **Step 1: Write the failing test and the first shared vectors**

The vectors cover precedence, right-associative `^`, unary minus against `^`, division by zero, the lazy `if`, `sum`/`par`, evaluation errors, parse errors with every kind of malformed input, formulas and `Deps`. The helper file is used by every later test: `readJSON` loads a vector file, `vecNum` reads a number that may be `"nan"`/`"inf"`, `near` is the tolerance comparison.

Create `internal/ecalc/helpers_test.go` with exactly this content:

```go
package ecalc

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// readJSON loads a JSON file below the package directory (go test runs in it).
func readJSON(t *testing.T, rel string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
}

// vecNum converts a vector number: a JSON number, or one of the strings "nan", "inf" and "-inf".
func vecNum(t *testing.T, v any) float64 {
	t.Helper()
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "nan":
			return math.NaN()
		case "inf":
			return math.Inf(1)
		case "-inf":
			return math.Inf(-1)
		}
	}
	t.Fatalf("bad vector number %#v", v)
	return 0
}

// near reports whether got matches want: NaN equals NaN, infinities must be identical, a zero want allows an
// absolute error of 1e-12, everything else must agree within the relative error rel.
func near(got, want, rel float64) bool {
	switch {
	case math.IsNaN(want):
		return math.IsNaN(got)
	case math.IsInf(want, 0):
		return got == want
	case want == 0:
		return math.Abs(got) <= 1e-12
	}
	return math.Abs(got-want) <= rel*math.Abs(want)
}
```

Create `internal/ecalc/expr_test.go` with exactly this content:

```go
package ecalc

import (
	"reflect"
	"strings"
	"testing"
)

type exprVector struct {
	Src   string               `json:"src"`
	Env   map[string]float64   `json:"env"`
	Lists map[string][]float64 `json:"lists"`
	Want  any                  `json:"want"`
}

type astVector struct {
	Src string `json:"src"`
	AST string `json:"ast"`
}

type depsVector struct {
	Src  string   `json:"src"`
	Deps []string `json:"deps"`
}

func TestExprVectors(t *testing.T) {
	var f struct {
		Expr         []exprVector `json:"expr"`
		EvalError    []exprVector `json:"eval_error"`
		Parse        []astVector  `json:"parse"`
		ParseError   []string     `json:"parse_error"`
		Formula      []astVector  `json:"formula"`
		FormulaError []string     `json:"formula_error"`
		Deps         []depsVector `json:"deps"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Expr) == 0 || len(f.Parse) == 0 || len(f.Deps) == 0 {
		t.Fatal("core.json lacks the expression sections")
	}

	for _, v := range f.Expr {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("expr %q: parse: %v", v.Src, err)
			continue
		}
		got, err := Eval(n, Env{Vals: v.Env, Lists: v.Lists})
		if err != nil {
			t.Errorf("expr %q: eval: %v", v.Src, err)
			continue
		}
		if want := vecNum(t, v.Want); !near(got, want, 1e-12) {
			t.Errorf("expr %q = %v, want %v", v.Src, got, want)
		}
	}
	for _, v := range f.EvalError {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("eval_error %q: parse: %v", v.Src, err)
			continue
		}
		if got, err := Eval(n, Env{Vals: v.Env, Lists: v.Lists}); err == nil {
			t.Errorf("eval_error %q: evaluated to %v, want an error", v.Src, got)
		}
	}
	for _, v := range f.Parse {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("parse %q: %v", v.Src, err)
			continue
		}
		if got := n.Sexp(); got != v.AST {
			t.Errorf("parse %q = %s, want %s", v.Src, got, v.AST)
		}
	}
	for _, src := range f.ParseError {
		if n, err := ParseExpr(src); err == nil {
			t.Errorf("parse_error %q: parsed as %s, want an error", src, n.Sexp())
		}
	}
	for _, v := range f.Formula {
		n, err := ParseFormula(v.Src)
		if err != nil {
			t.Errorf("formula %q: %v", v.Src, err)
			continue
		}
		if got := n.Sexp(); got != v.AST {
			t.Errorf("formula %q = %s, want %s", v.Src, got, v.AST)
		}
	}
	for _, src := range f.FormulaError {
		if n, err := ParseFormula(src); err == nil {
			t.Errorf("formula_error %q: parsed as %s, want an error", src, n.Sexp())
		}
	}
	for _, v := range f.Deps {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("deps %q: %v", v.Src, err)
			continue
		}
		got := Deps(n)
		if got == nil {
			got = []string{}
		}
		if !reflect.DeepEqual(got, v.Deps) {
			t.Errorf("deps %q = %v, want %v", v.Src, got, v.Deps)
		}
	}
}

func TestParseErrorsNamePosition(t *testing.T) {
	if _, err := ParseExpr("1 + $"); err == nil || !strings.Contains(err.Error(), "position 4") {
		t.Errorf("lexer error = %v, want a message naming position 4", err)
	}
	if _, err := ParseExpr("sqrt(1, 2)"); err == nil || !strings.Contains(err.Error(), `"sqrt"`) {
		t.Errorf("arity error = %v, want a message naming sqrt", err)
	}
	if _, err := ParseExpr("foo(1)"); err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("unknown function error = %v", err)
	}
}
```

Create `internal/ecalc/testdata/vectors/core.json` with exactly this content:

```json
{
  "_doc": [
    "Shared ECalc vectors. The Go tests (internal/ecalc) and the JavaScript parity test run the same file.",
    "A numeric want is a JSON number or one of the strings \"nan\", \"inf\", \"-inf\".",
    "Numbers match when the relative error is at most 1e-12; a zero want allows an absolute error of 1e-12.",
    "Keys starting with _doc are documentation. Every other top-level key is a list of vectors."
  ],
  "_doc_expr": "expr: parse src, evaluate it with env (name -> number) and lists (name -> array of numbers); the result must equal want.",
  "expr": [
    {"src": "1+2*3", "want": 7},
    {"src": "(1+2)*3", "want": 9},
    {"src": "10-4-3", "want": 3},
    {"src": "100/10/5", "want": 2},
    {"src": "8/2*4", "want": 16},
    {"src": "2^3^2", "want": 512},
    {"src": "2*3^2", "want": 18},
    {"src": "-2^2", "want": -4},
    {"src": "-3^2+1", "want": -8},
    {"src": "(-2)^2", "want": 4},
    {"src": "2^-1", "want": 0.5},
    {"src": "2*-3", "want": -6},
    {"src": "--3", "want": 3},
    {"src": "+4", "want": 4},
    {"src": "-(2+3)*2", "want": -10},
    {"src": "10 ^ ( -3 )", "want": 0.001},
    {"src": "10^(20/20)", "want": 10},
    {"src": "2^0.5", "want": 1.4142135623730951},
    {"src": ".5+1e-3", "want": 0.501},
    {"src": "1E3", "want": 1000},
    {"src": "2e+2", "want": 200},
    {"src": "1.5e3/3", "want": 500},
    {"src": "1e3*2^10", "want": 1024000},
    {"src": "1/0", "want": "inf"},
    {"src": "-1/0", "want": "-inf"},
    {"src": "0/0", "want": "nan"},
    {"src": "sqrt(16)", "want": 4},
    {"src": "sqrt(-1)", "want": "nan"},
    {"src": "ln(1)", "want": 0},
    {"src": "ln(exp(2))", "want": 2},
    {"src": "ln(0)", "want": "-inf"},
    {"src": "log10(1000)", "want": 3},
    {"src": "log10(0.001)", "want": -3},
    {"src": "log10(2)", "want": 0.3010299956639812},
    {"src": "exp(0)", "want": 1},
    {"src": "exp(1)", "want": 2.718281828459045},
    {"src": "sin(pi/2)", "want": 1},
    {"src": "sin(pi)", "want": 0},
    {"src": "cos(0)", "want": 1},
    {"src": "tan(0)", "want": 0},
    {"src": "atan(1)*4", "want": 3.141592653589793},
    {"src": "pi", "want": 3.141592653589793},
    {"src": "abs(-3.5)", "want": 3.5},
    {"src": "round(2.5)", "want": 3},
    {"src": "round(-2.5)", "want": -3},
    {"src": "round(2.4)", "want": 2},
    {"src": "round(-2.4)", "want": -2},
    {"src": "min(3,1,2)", "want": 1},
    {"src": "max(3,1,2)", "want": 3},
    {"src": "max(-1,-2)", "want": -1},
    {"src": "if(1<2,10,20)", "want": 10},
    {"src": "if(2<1,10,20)", "want": 20},
    {"src": "if(1,5,nosuch)", "want": 5},
    {"src": "if(0,nosuch,6)", "want": 6},
    {"src": "if(0/0,1,2)", "want": 2},
    {"src": "1<2", "want": 1},
    {"src": "2<=2", "want": 1},
    {"src": "3<2", "want": 0},
    {"src": "2>=3", "want": 0},
    {"src": "3>2", "want": 1},
    {"src": "2==2", "want": 1},
    {"src": "2!=2", "want": 0},
    {"src": "1+1==2", "want": 1},
    {"src": "0/0==0/0", "want": 0},
    {"src": "x*y+z", "env": {"x": 2, "y": 3, "z": 4}, "want": 10},
    {"src": "abs(x-y)/max(x,y)", "env": {"x": 3, "y": 5}, "want": 0.4},
    {"src": "sum(a)", "lists": {"a": [1, 2, 3.5]}, "want": 6.5},
    {"src": "par(a)", "lists": {"a": [100, 100]}, "want": 50},
    {"src": "par(a)", "lists": {"a": [3, 6]}, "want": 2},
    {"src": "par(a)", "lists": {"a": [5, 0]}, "want": 0},
    {"src": "par(a)+sum(a)", "lists": {"a": [3, 6]}, "want": 11},
    {"src": "sum(a)*k", "env": {"k": 2}, "lists": {"a": [1, 2]}, "want": 6}
  ],
  "_doc_eval_error": "eval_error: src parses, but evaluating it with env and lists must fail (unknown identifier, list outside sum/par, sum/par argument that is not a list).",
  "eval_error": [
    {"src": "x+1", "env": {}},
    {"src": "x+1", "env": {"y": 1}},
    {"src": "a+1", "lists": {"a": [1, 2]}},
    {"src": "sum(x)", "env": {"x": 3}},
    {"src": "sum(1+a)", "lists": {"a": [1]}},
    {"src": "par(a)", "lists": {}}
  ],
  "_doc_parse": "parse: ParseExpr(src) must succeed and print as ast. Prefix notation: (op l r) for binary operators, (neg a), (pos a), (fn arg ...) for calls, numbers exactly as written, identifiers by name. Parentheses leave no node.",
  "parse": [
    {"src": "1+2*3", "ast": "(+ 1 (* 2 3))"},
    {"src": "(1+2)*3", "ast": "(* (+ 1 2) 3)"},
    {"src": "((a))", "ast": "a"},
    {"src": "a-b-c", "ast": "(- (- a b) c)"},
    {"src": "a/b/c", "ast": "(/ (/ a b) c)"},
    {"src": "a-(b-c)", "ast": "(- a (- b c))"},
    {"src": "a^b^c", "ast": "(^ a (^ b c))"},
    {"src": "-a^b", "ast": "(neg (^ a b))"},
    {"src": "a^-b", "ast": "(^ a (neg b))"},
    {"src": "-a*b", "ast": "(* (neg a) b)"},
    {"src": "a*-b", "ast": "(* a (neg b))"},
    {"src": "+a", "ast": "(pos a)"},
    {"src": "2*(a+b)^2", "ast": "(* 2 (^ (+ a b) 2))"},
    {"src": "a<b+1", "ast": "(< a (+ b 1))"},
    {"src": "a<=b", "ast": "(<= a b)"},
    {"src": "a>b", "ast": "(> a b)"},
    {"src": "a>=b", "ast": "(>= a b)"},
    {"src": "a==b", "ast": "(== a b)"},
    {"src": "a!=b", "ast": "(!= a b)"},
    {"src": "sqrt(a*b)", "ast": "(sqrt (* a b))"},
    {"src": "min(a, b, 3)", "ast": "(min a b 3)"},
    {"src": "if(a>=1, b, c)", "ast": "(if (>= a 1) b c)"},
    {"src": "sum(rs)", "ast": "(sum rs)"},
    {"src": "1e-6", "ast": "1e-6"},
    {"src": ".5", "ast": ".5"},
    {"src": "1E3", "ast": "1E3"},
    {"src": "0.25", "ast": "0.25"},
    {"src": "pi", "ast": "pi"}
  ],
  "_doc_parse_error": "parse_error: ParseExpr(src) must fail.",
  "parse_error": ["", "   ", "1+", "+", "(1+2", "1+2)", "foo(1)", "frobnicate(1)", "sqrt()", "sqrt(1,2)", "min(1)", "if(1,2)", "1 2", "a b", "1..2", "3.", "a=b", "1<2<3", "1 $ 2", "2e", "max(1,)", "(,)", "sqrt 4", "pi()", "x^", "*3", "1,2"],
  "_doc_formula": "formula: ParseFormula(src) must succeed and print as ast. The top node is (= part part ...), also for a formula without '='.",
  "formula": [
    {"src": "a = b * c", "ast": "(= a (* b c))"},
    {"src": "vout = vin * r2 / (r1 + r2)", "ast": "(= vout (/ (* vin r2) (+ r1 r2)))"},
    {"src": "x = y = z", "ast": "(= x y z)"},
    {"src": "a + b", "ast": "(= (+ a b))"},
    {"src": "g = k * log10(a2 / a1)", "ast": "(= g (* k (log10 (/ a2 a1))))"},
    {"src": "rser = sum(rs)", "ast": "(= rser (sum rs))"}
  ],
  "_doc_formula_error": "formula_error: ParseFormula(src) must fail.",
  "formula_error": ["a =", "= a", "a = = b", ""],
  "_doc_deps": "deps: the sorted unique identifiers of ParseExpr(src), without function names and without pi.",
  "deps": [
    {"src": "x*sin(y)+pi+x", "deps": ["x", "y"]},
    {"src": "sum(rs)", "deps": ["rs"]},
    {"src": "min(a,b)>c", "deps": ["a", "b", "c"]},
    {"src": "2+3", "deps": []},
    {"src": "if(a, b, c)", "deps": ["a", "b", "c"]},
    {"src": "vout*r1/(vin-vout)", "deps": ["r1", "vin", "vout"]}
  ]
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run 'TestExpr|TestParseErrors' -count=1
```

Expected: **FAIL** — the package has no `ParseExpr`, `Eval` or `Env` yet, so the test binary does not build:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\expr_test.go:42:13: undefined: ParseExpr
internal\ecalc\expr_test.go:47:15: undefined: Eval
internal\ecalc\expr_test.go:47:23: undefined: Env
internal\ecalc\expr_test.go:57:13: undefined: ParseExpr
internal\ecalc\expr_test.go:62:18: undefined: Eval
```

- [ ] **Step 3: Implement the expression language**

`ParseExpr` and `ParseFormula` are a hand-written lexer plus recursive descent (`cmp > add > mul > unary > pow > atom`; `pow` takes a `unary` on its right, which gives right associativity and `-2^2 = -4`). Function names and arities are checked while parsing, so an unknown function is a parse error. `Eval` follows IEEE-754: division by zero is `+-Inf` or NaN, never an error.

Create `internal/ecalc/doc.go` with exactly this content:

```go
// Package ecalc is the engine behind the ECalc desktop app: a declarative
// electronics catalog (embedded YAML), a small safe expression language, units
// and number parsing/formatting, E-series lookup and a rule propagation solver.
//
// The package has no server dependency. The HTTP route, the agent tool and the
// tests all use it directly. The expression language, the unit parser and
// formatter, the E-series lookup and the solver are mirrored in JavaScript under
// ui/js/desktop/apps/ecalc; both implementations run the shared vectors in
// testdata/vectors.
package ecalc
```

Create `internal/ecalc/expr.go` with exactly this content:

```go
package ecalc

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is one node of a parsed expression. T selects the shape:
//
//	"num"  V, Raw (the literal as written)
//	"id"   Name
//	"neg"  A       "pos"  A
//	"bin"  Op, L, R (Op is + - * / ^ < <= > >= == !=)
//	"call" Name, Args
//	"eq"   Parts (formulas only: expr = expr = ...)
type Node struct {
	T     string
	V     float64
	Raw   string
	Name  string
	Op    string
	A     *Node
	L, R  *Node
	Args  []*Node
	Parts []*Node
}

// Env holds the values an expression may reference: scalars by name and lists (for sum and par) by name.
type Env struct {
	Vals  map[string]float64
	Lists map[string][]float64
}

// funcArity is the fixed function whitelist: name -> {min, max} argument count (max -1 = unbounded).
var funcArity = map[string][2]int{
	"sqrt": {1, 1}, "ln": {1, 1}, "log10": {1, 1}, "exp": {1, 1}, "sin": {1, 1}, "cos": {1, 1},
	"tan": {1, 1}, "atan": {1, 1}, "abs": {1, 1}, "round": {1, 1},
	"min": {2, -1}, "max": {2, -1}, "if": {3, 3}, "sum": {1, 1}, "par": {1, 1},
}

func isFunc(name string) bool {
	_, ok := funcArity[name]
	return ok
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokNum
	tokIdent
	tokOp
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// scanNumber returns the end of the number literal starting at s[i]:
// digits ['.' digits] [('e'|'E') ['+'|'-'] digits]  or  '.' digits ...
func scanNumber(s string, i int) int {
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j+1 < len(s) && s[j] == '.' && isDigit(s[j+1]) {
		j++
		for j < len(s) && isDigit(s[j]) {
			j++
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	return j
}

func lex(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case isDigit(c) || (c == '.' && i+1 < len(src) && isDigit(src[i+1])):
			j := scanNumber(src, i)
			toks = append(toks, token{tokNum, src[i:j], i})
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			toks = append(toks, token{tokIdent, src[i:j], i})
			i = j
		default:
			if i+1 < len(src) {
				switch two := src[i : i+2]; two {
				case "<=", ">=", "==", "!=":
					toks = append(toks, token{tokOp, two, i})
					i += 2
					continue
				}
			}
			if strings.IndexByte("+-*/^(),<>=", c) < 0 {
				r, _ := utf8.DecodeRuneInString(src[i:])
				return nil, fmt.Errorf("unexpected character %q at position %d", r, i)
			}
			toks = append(toks, token{tokOp, src[i : i+1], i})
			i++
		}
	}
	toks = append(toks, token{tokEOF, "", len(src)})
	return toks, nil
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

func (p *parser) isOp(s string) bool {
	t := p.peek()
	return t.kind == tokOp && t.text == s
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of input"
	}
	return fmt.Sprintf("%q", t.text)
}

func (p *parser) unexpected() error {
	t := p.peek()
	return fmt.Errorf("unexpected %s at position %d", t, t.pos)
}

// ParseExpr parses one expression (no '=').
func ParseExpr(src string) (*Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, err := p.parseCmp()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, p.unexpected()
	}
	return n, nil
}

// ParseFormula parses a display formula: expr ('=' expr)*. The result is an "eq" node with one part per expression.
func ParseFormula(src string) (*Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	var parts []*Node
	for {
		n, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		parts = append(parts, n)
		if !p.isOp("=") {
			break
		}
		p.next()
	}
	if p.peek().kind != tokEOF {
		return nil, p.unexpected()
	}
	return &Node{T: "eq", Parts: parts}, nil
}

// cmp := add (('<' | '<=' | '>' | '>=' | '==' | '!=') add)?
func (p *parser) parseCmp() (*Node, error) {
	l, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind == tokOp {
		switch t.text {
		case "<", "<=", ">", ">=", "==", "!=":
			p.next()
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return &Node{T: "bin", Op: t.text, L: l, R: r}, nil
		}
	}
	return l, nil
}

func (p *parser) parseAdd() (*Node, error) {
	l, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.isOp("+") || p.isOp("-") {
		op := p.next().text
		r, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		l = &Node{T: "bin", Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseMul() (*Node, error) {
	l, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isOp("*") || p.isOp("/") {
		op := p.next().text
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l = &Node{T: "bin", Op: op, L: l, R: r}
	}
	return l, nil
}

// unary := ('-' | '+') unary | pow
func (p *parser) parseUnary() (*Node, error) {
	if p.isOp("-") || p.isOp("+") {
		t := "neg"
		if p.next().text == "+" {
			t = "pos"
		}
		a, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{T: t, A: a}, nil
	}
	return p.parsePow()
}

// pow := atom ('^' unary)?  (right-associative; -2^2 = -(2^2); 2^-1 is fine)
func (p *parser) parsePow() (*Node, error) {
	base, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.isOp("^") {
		p.next()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{T: "bin", Op: "^", L: base, R: exp}, nil
	}
	return base, nil
}

func (p *parser) parseAtom() (*Node, error) {
	t := p.peek()
	switch {
	case t.kind == tokNum:
		p.next()
		v, err := strconv.ParseFloat(t.text, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, fmt.Errorf("bad number %q at position %d", t.text, t.pos)
		}
		return &Node{T: "num", V: v, Raw: t.text}, nil
	case t.kind == tokIdent:
		p.next()
		if !p.isOp("(") {
			return &Node{T: "id", Name: t.text}, nil
		}
		return p.parseCall(t)
	case t.kind == tokOp && t.text == "(":
		p.next()
		n, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		if !p.isOp(")") {
			return nil, fmt.Errorf("expected ')' but found %s at position %d", p.peek(), p.peek().pos)
		}
		p.next()
		return n, nil
	}
	return nil, p.unexpected()
}

func (p *parser) parseCall(name token) (*Node, error) {
	arity, ok := funcArity[name.text]
	if !ok {
		return nil, fmt.Errorf("unknown function %q at position %d", name.text, name.pos)
	}
	p.next() // '('
	var args []*Node
	if !p.isOp(")") {
		for {
			a, err := p.parseCmp()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if !p.isOp(",") {
				break
			}
			p.next()
		}
	}
	if !p.isOp(")") {
		return nil, fmt.Errorf("expected ')' but found %s at position %d", p.peek(), p.peek().pos)
	}
	p.next()
	if len(args) < arity[0] || (arity[1] >= 0 && len(args) > arity[1]) {
		return nil, fmt.Errorf("wrong number of arguments (%d) for function %q at position %d", len(args), name.text, name.pos)
	}
	return &Node{T: "call", Name: name.text, Args: args}, nil
}

// Sexp prints the tree in prefix notation: (op l r), (neg a), (pos a), (fn arg ...), (= part ...); numbers as
// written, identifiers by name. The shared vectors compare trees in this form.
func (n *Node) Sexp() string {
	switch n.T {
	case "num":
		return n.Raw
	case "id":
		return n.Name
	case "neg", "pos":
		return "(" + n.T + " " + n.A.Sexp() + ")"
	case "bin":
		return "(" + n.Op + " " + n.L.Sexp() + " " + n.R.Sexp() + ")"
	case "call":
		return "(" + n.Name + sexpList(n.Args) + ")"
	case "eq":
		return "(=" + sexpList(n.Parts) + ")"
	}
	return "?"
}

func sexpList(nodes []*Node) string {
	var b strings.Builder
	for _, a := range nodes {
		b.WriteByte(' ')
		b.WriteString(a.Sexp())
	}
	return b.String()
}

// Deps returns the sorted, unique identifiers an expression uses, without function names and without pi.
func Deps(n *Node) []string {
	seen := map[string]bool{}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.T == "id" && n.Name != "pi" && !isFunc(n.Name) {
			seen[n.Name] = true
		}
		walk(n.A)
		walk(n.L)
		walk(n.R)
		for _, a := range n.Args {
			walk(a)
		}
		for _, a := range n.Parts {
			walk(a)
		}
	}
	walk(n)
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Eval evaluates the tree with IEEE-754 double semantics: division by zero gives +-Inf or NaN, never an error.
// Unknown identifiers, lists used outside sum/par, and a sum/par argument that is not a list are errors.
func Eval(n *Node, env Env) (float64, error) {
	switch n.T {
	case "num":
		return n.V, nil
	case "id":
		if n.Name == "pi" {
			return math.Pi, nil
		}
		if v, ok := env.Vals[n.Name]; ok {
			return v, nil
		}
		if _, ok := env.Lists[n.Name]; ok {
			return 0, fmt.Errorf("list %q used outside sum() or par()", n.Name)
		}
		return 0, fmt.Errorf("unknown identifier %q", n.Name)
	case "neg", "pos":
		a, err := Eval(n.A, env)
		if err != nil {
			return 0, err
		}
		if n.T == "neg" {
			return -a, nil
		}
		return a, nil
	case "bin":
		l, err := Eval(n.L, env)
		if err != nil {
			return 0, err
		}
		r, err := Eval(n.R, env)
		if err != nil {
			return 0, err
		}
		return evalBin(n.Op, l, r), nil
	case "call":
		return evalCall(n, env)
	}
	return 0, fmt.Errorf("cannot evaluate a %q node", n.T)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func evalBin(op string, l, r float64) float64 {
	switch op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		return l / r
	case "^":
		return math.Pow(l, r)
	case "<":
		return b2f(l < r)
	case "<=":
		return b2f(l <= r)
	case ">":
		return b2f(l > r)
	case ">=":
		return b2f(l >= r)
	case "==":
		return b2f(l == r)
	case "!=":
		return b2f(l != r)
	}
	return math.NaN()
}

// log10 is math.Log10 made exact for exact powers of ten, like the JavaScript Math.log10.
func log10(x float64) float64 {
	r := math.Log10(x)
	if n := math.Round(r); math.Abs(r-n) < 1e-9 && math.Abs(n) <= 300 && x == math.Pow10(int(n)) {
		return n
	}
	return r
}

func evalCall(n *Node, env Env) (float64, error) {
	switch n.Name {
	case "if":
		c, err := Eval(n.Args[0], env)
		if err != nil {
			return 0, err
		}
		if c != 0 && !math.IsNaN(c) {
			return Eval(n.Args[1], env)
		}
		return Eval(n.Args[2], env)
	case "sum", "par":
		id := n.Args[0]
		list, ok := env.Lists[id.Name]
		if id.T != "id" || !ok {
			return 0, fmt.Errorf("%s() needs a list variable", n.Name)
		}
		s := 0.0
		for _, x := range list {
			if n.Name == "sum" {
				s += x
			} else {
				s += 1 / x
			}
		}
		if n.Name == "par" {
			return 1 / s, nil
		}
		return s, nil
	}
	args := make([]float64, len(n.Args))
	for i, a := range n.Args {
		v, err := Eval(a, env)
		if err != nil {
			return 0, err
		}
		args[i] = v
	}
	switch n.Name {
	case "sqrt":
		return math.Sqrt(args[0]), nil
	case "ln":
		return math.Log(args[0]), nil
	case "log10":
		return log10(args[0]), nil
	case "exp":
		return math.Exp(args[0]), nil
	case "sin":
		return math.Sin(args[0]), nil
	case "cos":
		return math.Cos(args[0]), nil
	case "tan":
		return math.Tan(args[0]), nil
	case "atan":
		return math.Atan(args[0]), nil
	case "abs":
		return math.Abs(args[0]), nil
	case "round":
		return math.Round(args[0]), nil
	case "min", "max":
		m := args[0]
		for _, a := range args[1:] {
			if n.Name == "min" {
				m = math.Min(m, a)
			} else {
				m = math.Max(m, a)
			}
		}
		return m, nil
	}
	return 0, fmt.Errorf("unknown function %q", n.Name)
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -run 'TestExpr|TestParseErrors' -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/doc.go internal/ecalc/expr.go internal/ecalc/expr_test.go internal/ecalc/helpers_test.go internal/ecalc/testdata/vectors/core.json
git commit -m "feat(ecalc): expression parser, AST and evaluator with shared vectors" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A2: E-series tables, nearest value and significant-digit rounding

**Files:**
- Create: `internal/ecalc/eseries.go`
- Test: `internal/ecalc/eseries_test.go`
- Modify: `internal/ecalc/testdata/vectors/core.json`

- [ ] **Step 1: Write the failing test and the E-series vectors**

`roundSig` is tested here because `Nearest` and the unit formatter of Task A3 both depend on it; its vectors include exact ties (12345 to 4 digits is 12350, 0.125 to 2 digits is 0.13) and the doubles just above a tie (999.95 is 1000). The `Nearest` vectors were worked out by hand from the decade tables, including both decade edges (999 gives 1000, 9.6 gives 10, 1.014 gives 1.01). The table test checks counts, order, the subset chain E3 ⊂ E6 ⊂ E12 ⊂ E24 and E48 ⊂ E96 ⊂ E192, the eight hand-picked IEC 60063 values, and that E48/E96/E192 equal 10^(k/N) rounded to three digits.

Create `internal/ecalc/eseries_test.go` with exactly this content:

```go
package ecalc

import (
	"math"
	"sort"
	"strconv"
	"testing"
)

func TestESeriesVectors(t *testing.T) {
	var f struct {
		RoundSig []struct {
			X    float64 `json:"x"`
			N    int     `json:"n"`
			Want float64 `json:"want"`
		} `json:"round_sig"`
		Nearest []struct {
			Series string  `json:"series"`
			V      float64 `json:"v"`
			Want   float64 `json:"want"`
			Dev    float64 `json:"dev"`
		} `json:"eseries"`
		NearestError []struct {
			Series string `json:"series"`
			V      any    `json:"v"`
		} `json:"eseries_error"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.RoundSig) == 0 || len(f.Nearest) == 0 {
		t.Fatal("core.json lacks the round_sig / eseries sections")
	}
	for _, v := range f.RoundSig {
		if got := roundSig(v.X, v.N); got != v.Want {
			t.Errorf("roundSig(%v, %d) = %v, want %v", v.X, v.N, got, v.Want)
		}
	}
	for _, v := range f.Nearest {
		got, dev, err := Nearest(v.Series, v.V)
		if err != nil {
			t.Errorf("Nearest(%s, %v): %v", v.Series, v.V, err)
			continue
		}
		if !near(got, v.Want, 1e-12) || !near(dev, v.Dev, 1e-9) {
			t.Errorf("Nearest(%s, %v) = %v (%v %%), want %v (%v %%)", v.Series, v.V, got, dev, v.Want, v.Dev)
		}
	}
	for _, v := range f.NearestError {
		if got, _, err := Nearest(v.Series, vecNum(t, v.V)); err == nil {
			t.Errorf("Nearest(%s, %v) = %v, want an error", v.Series, v.V, got)
		}
	}
}

func TestESeriesTables(t *testing.T) {
	wantLen := map[string]int{"E3": 3, "E6": 6, "E12": 12, "E24": 24, "E48": 48, "E96": 96, "E192": 192}
	for _, id := range ESeriesIDs {
		vals, ok := ESeriesValues(id)
		if !ok || len(vals) != wantLen[id] {
			t.Fatalf("%s: ok=%v len=%d, want %d values", id, ok, len(vals), wantLen[id])
		}
		if !sort.Float64sAreSorted(vals) || vals[0] != 1 || vals[len(vals)-1] >= 10 {
			t.Errorf("%s is not an ascending decade starting at 1 below 10", id)
		}
	}
	subset := func(small, big string) {
		bigVals, _ := ESeriesValues(big)
		in := map[float64]bool{}
		for _, v := range bigVals {
			in[v] = true
		}
		smallVals, _ := ESeriesValues(small)
		for _, v := range smallVals {
			if !in[v] {
				t.Errorf("%s value %v is missing from %s", small, v, big)
			}
		}
	}
	subset("E3", "E6")
	subset("E6", "E12")
	subset("E12", "E24")
	subset("E48", "E96")
	subset("E96", "E192")

	// The hand-picked IEC 60063 values that differ from the rounded progression.
	e24, _ := ESeriesValues("E24")
	have := map[float64]bool{}
	for _, v := range e24 {
		have[v] = true
	}
	for _, v := range []float64{2.7, 3.0, 3.3, 3.6, 3.9, 4.3, 4.7, 8.2} {
		if !have[v] {
			t.Errorf("E24 lacks %v", v)
		}
	}

	// E48, E96 and E192 are 10^(k/N) rounded to three digits (E192 index 185 is 9.20 in the standard, 9.19 rounded).
	for _, n := range []int{48, 96, 192} {
		vals, _ := ESeriesValues("E" + strconv.Itoa(n))
		for k, v := range vals {
			want := roundSig(math.Pow(10, float64(k)/float64(n)), 3)
			if n == 192 && k == 185 {
				want = 9.20
			}
			if v != want {
				t.Errorf("E%d[%d] = %v, want %v", n, k, v, want)
			}
		}
	}
}
```

In `internal/ecalc/testdata/vectors/core.json`, replace this exact text (it occurs once):

```json
    {"src": "vout*r1/(vin-vout)", "deps": ["r1", "vin", "vout"]}
  ]
}
```

with:

```json
    {"src": "vout*r1/(vin-vout)", "deps": ["r1", "vin", "vout"]}
  ],
  "_doc_round_sig": "round_sig: round x to n significant digits and return the nearest double of that decimal. Exact decimal ties round away from zero (the behaviour of JavaScript's Number(x.toPrecision(n))); the comparison is exact.",
  "round_sig": [
    {"x": 12345, "n": 4, "want": 12350},
    {"x": -12345, "n": 4, "want": -12350},
    {"x": 2.5, "n": 1, "want": 3},
    {"x": 0.125, "n": 2, "want": 0.13},
    {"x": 0.375, "n": 2, "want": 0.38},
    {"x": 1000.5, "n": 4, "want": 1001},
    {"x": 0.99995, "n": 4, "want": 1},
    {"x": 999.95, "n": 4, "want": 1000},
    {"x": 999.94, "n": 4, "want": 999.9},
    {"x": 9.9996, "n": 4, "want": 10},
    {"x": 0.00012345, "n": 3, "want": 0.000123},
    {"x": 1234567, "n": 3, "want": 1230000},
    {"x": 4700.000000000001, "n": 12, "want": 4700},
    {"x": 1e-7, "n": 3, "want": 1e-7},
    {"x": 0, "n": 4, "want": 0}
  ],
  "_doc_eseries": "eseries: Nearest(series, v) returns the standard value want (relative deviation, a tie goes to the lower value) and dev = (want - v) / v * 100.",
  "eseries": [
    {"series": "E24", "v": 4800, "want": 4700, "dev": -2.0833333333333335},
    {"series": "E24", "v": 4700, "want": 4700, "dev": 0},
    {"series": "E24", "v": 1, "want": 1, "dev": 0},
    {"series": "E24", "v": 1000, "want": 1000, "dev": 0},
    {"series": "E24", "v": 999, "want": 1000, "dev": 0.1001001001001001},
    {"series": "E24", "v": 9.5, "want": 9.1, "dev": -4.2105263157894735},
    {"series": "E24", "v": 9.6, "want": 10, "dev": 4.166666666666667},
    {"series": "E24", "v": 0.0047, "want": 0.0047, "dev": 0},
    {"series": "E24", "v": 2.2e-12, "want": 2.2e-12, "dev": 0},
    {"series": "E12", "v": 0.5, "want": 0.47, "dev": -6},
    {"series": "E6", "v": 2000, "want": 2200, "dev": 10},
    {"series": "E3", "v": 3, "want": 2.2, "dev": -26.666666666666668},
    {"series": "E3", "v": 10, "want": 10, "dev": 0},
    {"series": "E48", "v": 2, "want": 1.96, "dev": -2},
    {"series": "E96", "v": 1234, "want": 1240, "dev": 0.4862236628849271},
    {"series": "E96", "v": 976, "want": 976, "dev": 0},
    {"series": "E96", "v": 990, "want": 1000, "dev": 1.0101010101010102},
    {"series": "E192", "v": 1.014, "want": 1.01, "dev": -0.39447731755424065},
    {"series": "E192", "v": 9.2, "want": 9.2, "dev": 0},
    {"series": "E192", "v": 9.19, "want": 9.2, "dev": 0.1088139281828074}
  ],
  "_doc_eseries_error": "eseries_error: Nearest(series, v) must fail (unknown series, v not finite or not greater than zero).",
  "eseries_error": [
    {"series": "E24", "v": 0},
    {"series": "E24", "v": -5},
    {"series": "E24", "v": "inf"},
    {"series": "E24", "v": "nan"},
    {"series": "E5", "v": 10}
  ]
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run 'TestESeries' -count=1
```

Expected: **FAIL** — `roundSig`, `Nearest`, `ESeriesIDs` and `ESeriesValues` do not exist yet:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\eseries_test.go:33:13: undefined: roundSig
internal\ecalc\eseries_test.go:38:20: undefined: Nearest
internal\ecalc\eseries_test.go:48:21: undefined: Nearest
internal\ecalc\eseries_test.go:56:21: undefined: ESeriesIDs
internal\ecalc\eseries_test.go:57:15: undefined: ESeriesValues
internal\ecalc\eseries_test.go:66:17: undefined: ESeriesValues
internal\ecalc\eseries_test.go:71:19: undefined: ESeriesValues
internal\ecalc\eseries_test.go:85:12: undefined: ESeriesValues
internal\ecalc\eseries_test.go:98:14: undefined: ESeriesValues
internal\ecalc\eseries_test.go:100:12: undefined: roundSig
internal\ecalc\eseries_test.go:100:12: too many errors
```

- [ ] **Step 3: Implement the E-series package code**

The E3 to E24 tables are the hand-picked IEC 60063 values. E48 and E96 are 10^(k/N) rounded to three digits and E192 is the same except for 9.20 at index 185 (the standard lists 9.20, the rounded progression gives 9.19). `roundSig` uses `math/big` on the exact binary value of `x`, so it rounds ties the way `toPrecision` does.

Create `internal/ecalc/eseries.go` with exactly this content:

```go
package ecalc

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// IEC 60063 preferred-number tables: one decade of mantissas in [1, 10) per series. E3 to E24 are the
// hand-picked two-digit values (2.7, 3.0, 3.3, 3.6, 3.9, 4.3, 4.7 and 8.2 differ from the rounded geometric
// progression). E48 and E96 are 10^(k/N) rounded to three digits; E192 is the same except for 9.20 at index 185,
// where the standard lists 9.20 although 10^(185/192) rounds to 9.19.

var e3 = []float64{
	1.0, 2.2, 4.7,
}

var e6 = []float64{
	1.0, 1.5, 2.2, 3.3, 4.7, 6.8,
}

var e12 = []float64{
	1.0, 1.2, 1.5, 1.8, 2.2, 2.7, 3.3, 3.9, 4.7, 5.6, 6.8, 8.2,
}

var e24 = []float64{
	1.0, 1.1, 1.2, 1.3, 1.5, 1.6, 1.8, 2.0, 2.2, 2.4, 2.7, 3.0,
	3.3, 3.6, 3.9, 4.3, 4.7, 5.1, 5.6, 6.2, 6.8, 7.5, 8.2, 9.1,
}

var e48 = []float64{
	1.00, 1.05, 1.10, 1.15, 1.21, 1.27, 1.33, 1.40, 1.47, 1.54, 1.62, 1.69,
	1.78, 1.87, 1.96, 2.05, 2.15, 2.26, 2.37, 2.49, 2.61, 2.74, 2.87, 3.01,
	3.16, 3.32, 3.48, 3.65, 3.83, 4.02, 4.22, 4.42, 4.64, 4.87, 5.11, 5.36,
	5.62, 5.90, 6.19, 6.49, 6.81, 7.15, 7.50, 7.87, 8.25, 8.66, 9.09, 9.53,
}

var e96 = []float64{
	1.00, 1.02, 1.05, 1.07, 1.10, 1.13, 1.15, 1.18, 1.21, 1.24, 1.27, 1.30,
	1.33, 1.37, 1.40, 1.43, 1.47, 1.50, 1.54, 1.58, 1.62, 1.65, 1.69, 1.74,
	1.78, 1.82, 1.87, 1.91, 1.96, 2.00, 2.05, 2.10, 2.15, 2.21, 2.26, 2.32,
	2.37, 2.43, 2.49, 2.55, 2.61, 2.67, 2.74, 2.80, 2.87, 2.94, 3.01, 3.09,
	3.16, 3.24, 3.32, 3.40, 3.48, 3.57, 3.65, 3.74, 3.83, 3.92, 4.02, 4.12,
	4.22, 4.32, 4.42, 4.53, 4.64, 4.75, 4.87, 4.99, 5.11, 5.23, 5.36, 5.49,
	5.62, 5.76, 5.90, 6.04, 6.19, 6.34, 6.49, 6.65, 6.81, 6.98, 7.15, 7.32,
	7.50, 7.68, 7.87, 8.06, 8.25, 8.45, 8.66, 8.87, 9.09, 9.31, 9.53, 9.76,
}

var e192 = []float64{
	1.00, 1.01, 1.02, 1.04, 1.05, 1.06, 1.07, 1.09, 1.10, 1.11, 1.13, 1.14,
	1.15, 1.17, 1.18, 1.20, 1.21, 1.23, 1.24, 1.26, 1.27, 1.29, 1.30, 1.32,
	1.33, 1.35, 1.37, 1.38, 1.40, 1.42, 1.43, 1.45, 1.47, 1.49, 1.50, 1.52,
	1.54, 1.56, 1.58, 1.60, 1.62, 1.64, 1.65, 1.67, 1.69, 1.72, 1.74, 1.76,
	1.78, 1.80, 1.82, 1.84, 1.87, 1.89, 1.91, 1.93, 1.96, 1.98, 2.00, 2.03,
	2.05, 2.08, 2.10, 2.13, 2.15, 2.18, 2.21, 2.23, 2.26, 2.29, 2.32, 2.34,
	2.37, 2.40, 2.43, 2.46, 2.49, 2.52, 2.55, 2.58, 2.61, 2.64, 2.67, 2.71,
	2.74, 2.77, 2.80, 2.84, 2.87, 2.91, 2.94, 2.98, 3.01, 3.05, 3.09, 3.12,
	3.16, 3.20, 3.24, 3.28, 3.32, 3.36, 3.40, 3.44, 3.48, 3.52, 3.57, 3.61,
	3.65, 3.70, 3.74, 3.79, 3.83, 3.88, 3.92, 3.97, 4.02, 4.07, 4.12, 4.17,
	4.22, 4.27, 4.32, 4.37, 4.42, 4.48, 4.53, 4.59, 4.64, 4.70, 4.75, 4.81,
	4.87, 4.93, 4.99, 5.05, 5.11, 5.17, 5.23, 5.30, 5.36, 5.42, 5.49, 5.56,
	5.62, 5.69, 5.76, 5.83, 5.90, 5.97, 6.04, 6.12, 6.19, 6.26, 6.34, 6.42,
	6.49, 6.57, 6.65, 6.73, 6.81, 6.90, 6.98, 7.06, 7.15, 7.23, 7.32, 7.41,
	7.50, 7.59, 7.68, 7.77, 7.87, 7.96, 8.06, 8.16, 8.25, 8.35, 8.45, 8.56,
	8.66, 8.76, 8.87, 8.98, 9.09, 9.20, 9.31, 9.42, 9.53, 9.65, 9.76, 9.88,
}

var eseriesTables = map[string][]float64{
	"E3": e3, "E6": e6, "E12": e12, "E24": e24, "E48": e48, "E96": e96, "E192": e192,
}

// ESeriesIDs lists the supported series from coarse to fine.
var ESeriesIDs = []string{"E3", "E6", "E12", "E24", "E48", "E96", "E192"}

// ESeriesValues returns a copy of the mantissas (one decade, 1 <= m < 10) of a series.
func ESeriesValues(series string) ([]float64, bool) {
	tbl, ok := eseriesTables[series]
	if !ok {
		return nil, false
	}
	return append([]float64(nil), tbl...), true
}

// Nearest returns the standard value of the series closest to v (smallest relative deviation, a tie goes to the
// lower value) and its deviation (value-v)/v in percent. v must be finite and greater than zero.
func Nearest(series string, v float64) (value, devPct float64, err error) {
	tbl, ok := eseriesTables[series]
	if !ok {
		return 0, 0, fmt.Errorf("unknown E-series %q", series)
	}
	if !(v > 0) || math.IsInf(v, 0) {
		return 0, 0, fmt.Errorf("E-series lookup needs a finite value greater than zero, got %v", v)
	}
	d := math.Pow10(int(math.Floor(log10(v))))
	best, bestDev := 0.0, math.Inf(1)
	try := func(c float64) {
		if dev := math.Abs(c-v) / v; dev < bestDev {
			best, bestDev = c, dev
		}
	}
	for _, m := range tbl {
		try(m * d)
	}
	try(10 * d)
	best = roundSig(best, 12)
	return best, (best - v) / v * 100, nil
}

func bigPow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// roundSig rounds x to n significant digits and returns the double nearest to that decimal. Exact decimal ties are
// rounded away from zero, which is what JavaScript's Number(x.toPrecision(n)) does (Go's strconv would round them
// to even: 12345 to 4 digits is 12350 here, 12340 with strconv). It works on the exact binary value of x, so
// 999.95, which lies a hair above the tie, rounds up to 1000 in both languages.
func roundSig(x float64, n int) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	if n < 1 {
		n = 1
	}
	ax := math.Abs(x)
	exact := new(big.Rat).SetFloat64(ax)
	lo := new(big.Rat).SetInt(bigPow10(n - 1))
	hi := new(big.Rat).SetInt(bigPow10(n))
	e := int(math.Floor(log10(ax)))
	for {
		p := e - n + 1 // q = ax / 10^p must lie in [10^(n-1), 10^n)
		q := new(big.Rat)
		if p >= 0 {
			q.Quo(exact, new(big.Rat).SetInt(bigPow10(p)))
		} else {
			q.Mul(exact, new(big.Rat).SetInt(bigPow10(-p)))
		}
		if q.Cmp(lo) < 0 {
			e--
			continue
		}
		if q.Cmp(hi) >= 0 {
			e++
			continue
		}
		num := new(big.Int).Lsh(q.Num(), 1)
		num.Add(num, q.Denom())
		m := num.Quo(num, new(big.Int).Lsh(q.Denom(), 1)) // floor(q + 1/2)
		r, _ := strconv.ParseFloat(m.String()+"e"+strconv.Itoa(p), 64)
		if x < 0 {
			r = -r
		}
		return r
	}
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/eseries.go internal/ecalc/eseries_test.go internal/ecalc/testdata/vectors/core.json
git commit -m "feat(ecalc): E-series tables, nearest value and tie-exact significant-digit rounding" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A3: Units model, number parsing and formatting

**Files:**
- Create: `internal/ecalc/yamlutil.go`
- Create: `internal/ecalc/units.go`
- Create: `internal/ecalc/catalog/units.yaml`
- Test: `internal/ecalc/units_test.go`
- Modify: `internal/ecalc/testdata/vectors/core.json`

- [ ] **Step 1: Write the failing test and the unit vectors**

The vectors cover `4k7`, `4R7`, `47R`, `2u2`, `10mA`, `"10 mA"`, comma decimals (`1,5k`), `-10dBm`, `0dBu`, `2Ah`, a prefix that the dimension does not allow, an explicit unit that is selected while the input has no unit suffix, list parsing, and formatting: prefix stepping (999.95 k gives `1 MΩ`), zero, unitless dimensions, the explicit units dBm and Ah, and non-finite input. Expected values were worked out by hand or with exact decimal arithmetic. The Go test also loads deliberately broken unit definitions and folds the Greek mu and the ohm sign onto the micro sign and the Greek omega.

Create `internal/ecalc/units_test.go` with exactly this content:

```go
package ecalc

import (
	"os"
	"strings"
	"testing"
)

func testUnits(t *testing.T) *Units {
	t.Helper()
	data, err := os.ReadFile("catalog/units.yaml")
	if err != nil {
		t.Fatal(err)
	}
	u, err := loadUnits(data)
	if err != nil {
		t.Fatalf("units.yaml: %v", err)
	}
	return u
}

func TestUnitsVectors(t *testing.T) {
	var f struct {
		Parse []struct {
			In   string  `json:"in"`
			Dim  string  `json:"dim"`
			Unit string  `json:"unit"`
			Want float64 `json:"want"`
		} `json:"units_parse"`
		ParseError []struct {
			In   string `json:"in"`
			Dim  string `json:"dim"`
			Unit string `json:"unit"`
		} `json:"units_parse_error"`
		List []struct {
			In   string    `json:"in"`
			Dim  string    `json:"dim"`
			Want []float64 `json:"want"`
		} `json:"units_list"`
		ListError []struct {
			In  string `json:"in"`
			Dim string `json:"dim"`
		} `json:"units_list_error"`
		Format []struct {
			V    any    `json:"v"`
			Dim  string `json:"dim"`
			Unit string `json:"unit"`
			Sig  int    `json:"sig"`
			Want string `json:"want"`
		} `json:"units_format"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Parse) == 0 || len(f.Format) == 0 || len(f.List) == 0 {
		t.Fatal("core.json lacks the units sections")
	}
	u := testUnits(t)

	for _, v := range f.Parse {
		got, err := u.Parse(v.In, v.Dim, v.Unit)
		if err != nil {
			t.Errorf("Parse(%q, %s, %q): %v", v.In, v.Dim, v.Unit, err)
		} else if !near(got, v.Want, 1e-12) {
			t.Errorf("Parse(%q, %s, %q) = %v, want %v", v.In, v.Dim, v.Unit, got, v.Want)
		}
	}
	for _, v := range f.ParseError {
		if got, err := u.Parse(v.In, v.Dim, v.Unit); err == nil {
			t.Errorf("Parse(%q, %s, %q) = %v, want an error", v.In, v.Dim, v.Unit, got)
		}
	}
	for _, v := range f.List {
		got, err := u.ParseList(v.In, v.Dim, "")
		if err != nil || len(got) != len(v.Want) {
			t.Errorf("ParseList(%q) = %v, %v, want %v", v.In, got, err, v.Want)
			continue
		}
		for i := range got {
			if !near(got[i], v.Want[i], 1e-12) {
				t.Errorf("ParseList(%q)[%d] = %v, want %v", v.In, i, got[i], v.Want[i])
			}
		}
	}
	for _, v := range f.ListError {
		if got, err := u.ParseList(v.In, v.Dim, ""); err == nil {
			t.Errorf("ParseList(%q) = %v, want an error", v.In, got)
		}
	}
	for _, v := range f.Format {
		if got := u.Format(vecNum(t, v.V), v.Dim, v.Unit, v.Sig); got != v.Want {
			t.Errorf("Format(%v, %s, %q, %d) = %q, want %q", v.V, v.Dim, v.Unit, v.Sig, got, v.Want)
		}
	}
}

// The Greek mu (U+03BC) and the ohm sign (U+2126) look like the micro sign and the Greek omega; input folds them.
func TestUnitsFoldLookAlikes(t *testing.T) {
	u := testUnits(t)
	greekMu, ohmSign := string(rune(0x3BC)), string(rune(0x2126))
	if got, err := u.Parse("2"+greekMu+"2", "current", ""); err != nil || !near(got, 2.2e-6, 1e-12) {
		t.Errorf("greek mu: %v, %v", got, err)
	}
	if got, err := u.Parse("4.7k"+ohmSign, "resistance", ""); err != nil || !near(got, 4700, 1e-12) {
		t.Errorf("ohm sign: %v, %v", got, err)
	}
}

func TestLoadUnitsRejectsBrokenDefinitions(t *testing.T) {
	cases := []struct{ name, yaml, want string }{
		{"unknown field", "prefixes: []\ndims:\n  a: {symbol: A, colour: red}\n", "colour"},
		{"unknown prefix", "prefixes: [{id: k, factor: 1e3}]\ndims:\n  a: {symbol: A, prefixes: [k, M]}\n", `unknown prefix "M"`},
		{"prefixes descending", "prefixes: [{id: k, factor: 1e3}, {id: m, factor: 1e-3}]\ndims: {}\n", "ascending"},
		{"unit without conversion", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u}]}\n", "factor"},
		{"conversion with a foreign variable", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, to: \"y*2\", from: \"x/2\"}]}\n", "only x"},
		{"conversion that does not parse", "prefixes: []\ndims:\n  a: {symbol: A, units: [{id: u, symbol: u, to: \"x*\", from: \"x/2\"}]}\n", "unit \"u\""},
		{"duplicate dim", "prefixes: []\ndims:\n  a: {symbol: A}\n  a: {symbol: B}\n", "duplicate key"},
	}
	for _, c := range cases {
		_, err := loadUnits([]byte(c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}
```

In `internal/ecalc/testdata/vectors/core.json`, replace this exact text (it occurs once):

```json
    {"series": "E5", "v": 10}
  ]
}
```

with:

```json
    {"series": "E5", "v": 10}
  ],
  "_doc_units_parse": "units_parse: Parse(in, dim, unit) returns want in the SI base unit. unit is the selected explicit unit id (absent or empty = none).",
  "units_parse": [
    {"in": "4k7", "dim": "resistance", "want": 4700},
    {"in": "4K7", "dim": "resistance", "want": 4700},
    {"in": "4R7", "dim": "resistance", "want": 4.7},
    {"in": "47R", "dim": "resistance", "want": 47},
    {"in": "47", "dim": "resistance", "want": 47},
    {"in": "47Ω", "dim": "resistance", "want": 47},
    {"in": "4.7kΩ", "dim": "resistance", "want": 4700},
    {"in": "4k7Ω", "dim": "resistance", "want": 4700},
    {"in": "47 kΩ", "dim": "resistance", "want": 47000},
    {"in": "4.7 k", "dim": "resistance", "want": 4700},
    {"in": "4,7k", "dim": "resistance", "want": 4700},
    {"in": "1,5k", "dim": "resistance", "want": 1500},
    {"in": "2M2", "dim": "resistance", "want": 2200000},
    {"in": "2G2", "dim": "resistance", "want": 2200000000},
    {"in": "100ohm", "dim": "resistance", "want": 100},
    {"in": "100 Ohm", "dim": "resistance", "want": 100},
    {"in": "1e3", "dim": "resistance", "want": 1000},
    {"in": "10m", "dim": "resistance", "want": 0.01},
    {"in": "0.5", "dim": "resistance", "want": 0.5},
    {"in": "-5", "dim": "resistance", "want": -5},
    {"in": " 330 ", "dim": "resistance", "want": 330},
    {"in": "10mA", "dim": "current", "want": 0.01},
    {"in": "10 mA", "dim": "current", "want": 0.01},
    {"in": "10m", "dim": "current", "want": 0.01},
    {"in": "2u2", "dim": "current", "want": 2.2e-6},
    {"in": "2µ2", "dim": "current", "want": 2.2e-6},
    {"in": "500n", "dim": "current", "want": 5e-7},
    {"in": "3p3", "dim": "current", "want": 3.3e-12},
    {"in": "1A", "dim": "current", "want": 1},
    {"in": "1k5", "dim": "current", "want": 1500},
    {"in": "1.5e-3A", "dim": "current", "want": 0.0015},
    {"in": "12 V", "dim": "voltage", "want": 12},
    {"in": "5mV", "dim": "voltage", "want": 0.005},
    {"in": "1 kV", "dim": "voltage", "want": 1000},
    {"in": "230", "dim": "voltage", "want": 230},
    {"in": "1e-3", "dim": "voltage", "want": 0.001},
    {"in": "20dBV", "dim": "voltage", "want": 10},
    {"in": "-10dBV", "dim": "voltage", "want": 0.31622776601683794},
    {"in": "0 dBu", "dim": "voltage", "want": 0.7745966692414834},
    {"in": "6dBu", "dim": "voltage", "want": 1.5455235434379457},
    {"in": "0", "dim": "voltage", "unit": "dBV", "want": 1},
    {"in": "20", "dim": "voltage", "unit": "dBV", "want": 10},
    {"in": "-6", "dim": "voltage", "unit": "dBu", "want": 0.3882179618340383},
    {"in": "5 V", "dim": "voltage", "unit": "dBV", "want": 5},
    {"in": "5 mV", "dim": "voltage", "unit": "dBV", "want": 0.005},
    {"in": "0dBu", "dim": "voltage", "unit": "dBV", "want": 0.7745966692414834},
    {"in": "0dBm", "dim": "power", "want": 0.001},
    {"in": "-10dBm", "dim": "power", "want": 0.0001},
    {"in": "30 dBm", "dim": "power", "want": 1},
    {"in": "10dBm", "dim": "power", "want": 0.01},
    {"in": "250mW", "dim": "power", "want": 0.25},
    {"in": "1.5kW", "dim": "power", "want": 1500},
    {"in": "10", "dim": "power", "unit": "dBm", "want": 0.01},
    {"in": "2 W", "dim": "power", "unit": "dBm", "want": 2},
    {"in": "2Ah", "dim": "charge", "want": 7200},
    {"in": "2 Ah", "dim": "charge", "want": 7200},
    {"in": "500mAh", "dim": "charge", "want": 1800},
    {"in": "2.5Ah", "dim": "charge", "want": 9000},
    {"in": "1C", "dim": "charge", "want": 1},
    {"in": "10mC", "dim": "charge", "want": 0.01},
    {"in": "2", "dim": "charge", "want": 2},
    {"in": "2", "dim": "charge", "unit": "Ah", "want": 7200},
    {"in": "500", "dim": "charge", "unit": "mAh", "want": 1800},
    {"in": "2Ah", "dim": "charge", "unit": "mAh", "want": 7200},
    {"in": "7.4Wh", "dim": "energy", "want": 26640},
    {"in": "1kWh", "dim": "energy", "want": 3600000},
    {"in": "100mWh", "dim": "energy", "want": 360},
    {"in": "5J", "dim": "energy", "want": 5},
    {"in": "1kJ", "dim": "energy", "want": 1000},
    {"in": "60", "dim": "energy", "unit": "Wh", "want": 216000},
    {"in": "4,5", "dim": "number", "want": 4.5},
    {"in": "1e3", "dim": "number", "want": 1000},
    {"in": "-2", "dim": "number", "want": -2},
    {"in": "20dB", "dim": "gain", "want": 20},
    {"in": "-3 dB", "dim": "gain", "want": -3},
    {"in": "6", "dim": "gain", "want": 6}
  ],
  "_doc_units_parse_error": "units_parse_error: Parse(in, dim, unit) must fail.",
  "units_parse_error": [
    {"in": "", "dim": "resistance"},
    {"in": "   ", "dim": "resistance"},
    {"in": "abc", "dim": "resistance"},
    {"in": "R", "dim": "resistance"},
    {"in": "k", "dim": "resistance"},
    {"in": "Ω", "dim": "resistance"},
    {"in": "4.7.3", "dim": "resistance"},
    {"in": "--5", "dim": "resistance"},
    {"in": "1e", "dim": "resistance"},
    {"in": "5u", "dim": "resistance"},
    {"in": "4k7k", "dim": "resistance"},
    {"in": "2.2uF", "dim": "resistance"},
    {"in": "3V3", "dim": "voltage"},
    {"in": "10mA", "dim": "voltage"},
    {"in": "1k", "dim": "voltage", "unit": "dBV"},
    {"in": "4k7", "dim": "voltage", "unit": "dBV"},
    {"in": "1m", "dim": "charge", "unit": "mAh"},
    {"in": "4k7", "dim": "number"},
    {"in": "5%", "dim": "number"},
    {"in": "3k", "dim": "gain"},
    {"in": "1", "dim": "bogus"},
    {"in": "1", "dim": "voltage", "unit": "bogus"}
  ],
  "_doc_units_list": "units_list: ParseList(in, dim, unit) splits on commas, semicolons and white space, parses every item like Parse and skips empty items.",
  "units_list": [
    {"in": "4k7, 10k, 2k2", "dim": "resistance", "want": [4700, 10000, 2200]},
    {"in": "100 200;300", "dim": "resistance", "want": [100, 200, 300]},
    {"in": "1k", "dim": "resistance", "want": [1000]},
    {"in": "4k7,,10k", "dim": "resistance", "want": [4700, 10000]},
    {"in": "47R 4R7", "dim": "resistance", "want": [47, 4.7]},
    {"in": "10, 20", "dim": "voltage", "want": [10, 20]}
  ],
  "_doc_units_list_error": "units_list_error: ParseList(in, dim) must fail.",
  "units_list_error": [
    {"in": "", "dim": "resistance"},
    {"in": " , ", "dim": "resistance"},
    {"in": "4k7, abc", "dim": "resistance"},
    {"in": "5u, 10k", "dim": "resistance"}
  ],
  "_doc_units_format": "units_format: Format(v, dim, unit, sig) returns want. v is a number or \"nan\"/\"inf\"/\"-inf\" (result \"\"); unit is the selected explicit unit id (absent = automatic prefix); sig defaults to 4.",
  "units_format": [
    {"v": 4700, "dim": "resistance", "want": "4.7 kΩ"},
    {"v": 0, "dim": "resistance", "want": "0 Ω"},
    {"v": 1, "dim": "resistance", "want": "1 Ω"},
    {"v": 470, "dim": "resistance", "want": "470 Ω"},
    {"v": 0.47, "dim": "resistance", "want": "470 mΩ"},
    {"v": 0.0005, "dim": "resistance", "want": "0.5 mΩ"},
    {"v": 2200000, "dim": "resistance", "want": "2.2 MΩ"},
    {"v": 999.95, "dim": "resistance", "want": "1 kΩ"},
    {"v": 999940, "dim": "resistance", "want": "999.9 kΩ"},
    {"v": 999950, "dim": "resistance", "want": "1 MΩ"},
    {"v": 123456, "dim": "resistance", "want": "123.5 kΩ"},
    {"v": 4750, "dim": "resistance", "sig": 2, "want": "4.8 kΩ"},
    {"v": 4700, "dim": "resistance", "sig": 2, "want": "4.7 kΩ"},
    {"v": 4701.5, "dim": "resistance", "sig": 6, "want": "4.7015 kΩ"},
    {"v": -4700, "dim": "resistance", "want": "-4.7 kΩ"},
    {"v": 1e12, "dim": "resistance", "want": "1000 GΩ"},
    {"v": 1e-9, "dim": "resistance", "want": "0.000001 mΩ"},
    {"v": 1e-10, "dim": "resistance", "want": "1e-7 mΩ"},
    {"v": 12, "dim": "voltage", "want": "12 V"},
    {"v": 0.005, "dim": "voltage", "want": "5 mV"},
    {"v": 3.8260869565217392, "dim": "voltage", "want": "3.826 V"},
    {"v": 1e-7, "dim": "voltage", "want": "100 nV"},
    {"v": 1e-10, "dim": "voltage", "want": "0.1 nV"},
    {"v": 1500, "dim": "voltage", "want": "1.5 kV"},
    {"v": -0.0123, "dim": "voltage", "want": "-12.3 mV"},
    {"v": 1, "dim": "voltage", "unit": "dBV", "want": "0 dBV"},
    {"v": 10, "dim": "voltage", "unit": "dBV", "want": "20 dBV"},
    {"v": 0.1, "dim": "voltage", "unit": "dBV", "want": "-20 dBV"},
    {"v": 0.7745966692414834, "dim": "voltage", "unit": "dBu", "want": "0 dBu"},
    {"v": 0, "dim": "voltage", "unit": "dBV", "want": ""},
    {"v": -1, "dim": "voltage", "unit": "dBV", "want": ""},
    {"v": 0.25, "dim": "power", "want": "250 mW"},
    {"v": 1500, "dim": "power", "want": "1.5 kW"},
    {"v": 0.0000015, "dim": "power", "want": "1.5 µW"},
    {"v": 0, "dim": "power", "want": "0 W"},
    {"v": 0.001, "dim": "power", "unit": "dBm", "want": "0 dBm"},
    {"v": 1, "dim": "power", "unit": "dBm", "want": "30 dBm"},
    {"v": 0.1, "dim": "power", "unit": "dBm", "want": "20 dBm"},
    {"v": 0.01, "dim": "power", "unit": "dBm", "want": "10 dBm"},
    {"v": 2, "dim": "power", "unit": "dBm", "want": "33.01 dBm"},
    {"v": 0, "dim": "power", "unit": "dBm", "want": ""},
    {"v": 0.0017391304347826088, "dim": "current", "want": "1.739 mA"},
    {"v": 0.02, "dim": "current", "want": "20 mA"},
    {"v": 2.2e-6, "dim": "current", "want": "2.2 µA"},
    {"v": 5e-13, "dim": "current", "want": "0.5 pA"},
    {"v": 7200, "dim": "charge", "unit": "Ah", "want": "2 Ah"},
    {"v": 1800, "dim": "charge", "unit": "mAh", "want": "500 mAh"},
    {"v": 3.6, "dim": "charge", "unit": "mAh", "want": "1 mAh"},
    {"v": 7200, "dim": "charge", "want": "7200 C"},
    {"v": 0.5, "dim": "charge", "want": "500 mC"},
    {"v": 26640, "dim": "energy", "unit": "Wh", "want": "7.4 Wh"},
    {"v": 3600000, "dim": "energy", "unit": "kWh", "want": "1 kWh"},
    {"v": 26640, "dim": "energy", "want": "26.64 kJ"},
    {"v": 12345, "dim": "number", "want": "12350"},
    {"v": 0, "dim": "number", "want": "0"},
    {"v": 0.5, "dim": "number", "want": "0.5"},
    {"v": 0.000001, "dim": "number", "want": "0.000001"},
    {"v": 1e-7, "dim": "number", "want": "1e-7"},
    {"v": 1234567, "dim": "number", "want": "1235000"},
    {"v": 1e21, "dim": "number", "want": "1e+21"},
    {"v": -2.5, "dim": "number", "sig": 1, "want": "-3"},
    {"v": 20, "dim": "gain", "want": "20 dB"},
    {"v": 3.010299956639812, "dim": "gain", "want": "3.01 dB"},
    {"v": 0, "dim": "gain", "want": "0 dB"},
    {"v": -6.020599913279624, "dim": "gain", "want": "-6.021 dB"},
    {"v": "nan", "dim": "resistance", "want": ""},
    {"v": "inf", "dim": "resistance", "want": ""},
    {"v": "-inf", "dim": "resistance", "want": ""},
    {"v": 1, "dim": "bogus", "want": ""}
  ]
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run 'TestUnits|TestLoadUnits' -count=1
```

Expected: **FAIL** — `Units` and `loadUnits` do not exist yet:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\units_test.go:9:31: undefined: Units
internal\ecalc\units_test.go:15:12: undefined: loadUnits
internal\ecalc\units_test.go:118:13: undefined: loadUnits
FAIL	aurago/internal/ecalc [build failed]
FAIL
```

- [ ] **Step 3: Implement the YAML helpers, the units file and the units model**

`units.yaml` is exactly the structure of CONTRACT §2 (it uses the micro sign U+00B5 as the prefix id and the Greek capital omega U+03A9 as the resistance symbol). `yamlutil.go` holds three helpers that `catalog.go` reuses: strict decoding that rejects unknown fields, the same for a single `yaml.Node`, and `orderedMap`, which returns a mapping in file order and rejects duplicate keys. `units.go` implements CONTRACT §4: `Parse`, `ParseList`, `Format`, the non-linear units (`to`/`from` are expressions in the language of Task A1) and `fmtNum`, which writes numbers the way JavaScript's `String(x)` does.

Create `internal/ecalc/yamlutil.go` with exactly this content:

```go
package ecalc

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// decodeStrict decodes the first YAML document of data into v and rejects unknown fields.
func decodeStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}

// decodeNode decodes one YAML node into v and rejects unknown fields (yaml.Node.Decode alone would not).
func decodeNode(n *yaml.Node, v any) error {
	data, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return decodeStrict(data, v)
}

type mapEntry struct {
	Key string
	Val *yaml.Node
}

// orderedMap returns the entries of a YAML mapping in file order. An absent or null node is an empty mapping;
// anything else that is not a mapping, a non-scalar key or a duplicate key is an error.
func orderedMap(n *yaml.Node, what string) ([]mapEntry, error) {
	if n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null") {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping (line %d)", what, n.Line)
	}
	seen := map[string]bool{}
	out := make([]mapEntry, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: keys must be plain names (line %d)", what, k.Line)
		}
		if seen[k.Value] {
			return nil, fmt.Errorf("%s: duplicate key %q (line %d)", what, k.Value, k.Line)
		}
		seen[k.Value] = true
		out = append(out, mapEntry{Key: k.Value, Val: n.Content[i+1]})
	}
	return out, nil
}
```

Create `internal/ecalc/catalog/units.yaml` with exactly this content:

```yaml
# Prefixes and dimensions of the ECalc catalog. Every value is stored in SI base units.
#
# prefixes: ascending by factor. Input also accepts u for the micro sign and K for k.
# dims:     a mapping, order significant. A dimension has a base symbol, optional aliases that are accepted as a
#           suffix on input, the prefixes it may be written with ("" = the bare base unit), positive (values must
#           be greater than zero) and optional explicit units. An explicit unit is either linear (factor: base =
#           x * factor) or non-linear (to/from expressions in the expression language, x = the value in the unit).
prefixes:
  - {id: p, factor: 1e-12}
  - {id: n, factor: 1e-9}
  - {id: µ, factor: 1e-6}
  - {id: m, factor: 1e-3}
  - {id: k, factor: 1e3}
  - {id: M, factor: 1e6}
  - {id: G, factor: 1e9}
dims:
  voltage: {symbol: V, prefixes: [n, µ, m, "", k, M], units: [{id: dBV, symbol: dBV, to: "10^(x/20)", from: "20*log10(x)"}, {id: dBu, symbol: dBu, to: "0.7745966692414834*10^(x/20)", from: "20*log10(x/0.7745966692414834)"}]}
  current: {symbol: A, prefixes: [p, n, µ, m, "", k]}
  resistance: {symbol: Ω, aliases: [ohm, Ohm, R], prefixes: [m, "", k, M, G], positive: true}
  power: {symbol: W, prefixes: [n, µ, m, "", k, M], units: [{id: dBm, symbol: dBm, to: "10^((x-30)/10)", from: "10*log10(x)+30"}]}
  charge: {symbol: C, prefixes: [µ, m, ""], units: [{id: mAh, symbol: mAh, factor: 3.6}, {id: Ah, symbol: Ah, factor: 3600}]}
  energy: {symbol: J, prefixes: [m, "", k, M], units: [{id: mWh, symbol: mWh, factor: 3.6}, {id: Wh, symbol: Wh, factor: 3600}, {id: kWh, symbol: kWh, factor: 3.6e6}]}
  number: {symbol: "", prefixes: [], positive: true}
  gain: {symbol: dB, prefixes: []}
```

Create `internal/ecalc/units.go` with exactly this content:

```go
package ecalc

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// The micro sign (U+00B5) is the prefix id for 1e-6; the Greek mu (U+03BC) and the ohm sign (U+2126) are folded
// onto it and onto the Greek capital omega (U+03A9, the resistance symbol) when input is parsed.
var (
	micro     = string(rune(0xB5))
	ohm       = string(rune(0x3A9))
	inputFold = strings.NewReplacer(string(rune(0x3BC)), micro, string(rune(0x2126)), ohm)
)

var (
	plainNumRe    = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	prefixedNumRe = regexp.MustCompile(`^([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)([pnu` + micro + `mkKMG])$`)
	infixNumRe    = regexp.MustCompile(`^(\d+)([pnu` + micro + `mkKMG])(\d+)$`)
	infixRRe      = regexp.MustCompile(`^(\d+)R(\d+)$`)
)

// Prefix is an SI prefix: id (p n µ m k M G) and factor.
type Prefix struct {
	ID     string  `yaml:"id" json:"id"`
	Factor float64 `yaml:"factor" json:"factor"`
}

// Unit is an explicit display unit of a dimension. A linear unit has a Factor (base = x * Factor); a non-linear one
// has To and From expressions in the expression language, where x is the value in this unit (To) or in the base
// unit (From).
type Unit struct {
	ID     string  `json:"id"`
	Symbol string  `json:"symbol"`
	Factor float64 `json:"factor"`
	To     string  `json:"to"`
	From   string  `json:"from"`

	to, from *Node
}

// Dim is a physical dimension (voltage, resistance, ...). Values are stored in its SI base unit.
type Dim struct {
	ID       string   `json:"id"`
	Symbol   string   `json:"symbol"`
	Aliases  []string `json:"aliases"`
	Prefixes []string `json:"prefixes"`
	Positive bool     `json:"positive"`
	Units    []Unit   `json:"units"`

	pfx      []Prefix // Prefixes resolved, ascending by factor
	suffixes []string // base symbol and aliases, longest first
	exUnits  []*Unit  // explicit units, longest symbol first
}

// Units is the unit model loaded from catalog/units.yaml.
type Units struct {
	Prefixes []Prefix `json:"prefixes"`
	Dims     []Dim    `json:"dims"`

	prefix map[string]float64
	byDim  map[string]*Dim
}

type rawUnit struct {
	ID     string  `yaml:"id"`
	Symbol string  `yaml:"symbol"`
	Factor float64 `yaml:"factor"`
	To     string  `yaml:"to"`
	From   string  `yaml:"from"`
}

type rawDim struct {
	Symbol   string    `yaml:"symbol"`
	Aliases  []string  `yaml:"aliases"`
	Prefixes []string  `yaml:"prefixes"`
	Positive bool      `yaml:"positive"`
	Units    []rawUnit `yaml:"units"`
}

func loadUnits(data []byte) (*Units, error) {
	var raw struct {
		Prefixes []Prefix  `yaml:"prefixes"`
		Dims     yaml.Node `yaml:"dims"`
	}
	if err := decodeStrict(data, &raw); err != nil {
		return nil, fmt.Errorf("units: %w", err)
	}
	u := &Units{Prefixes: raw.Prefixes, prefix: map[string]float64{"": 1}, byDim: map[string]*Dim{}}
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf("units: "+format, args...)) }

	last := 0.0
	for _, p := range u.Prefixes {
		if p.ID == "" || p.Factor <= last {
			fail("prefix %q: ids must be set and factors positive and ascending", p.ID)
		}
		if _, dup := u.prefix[p.ID]; dup {
			fail("prefix %q is defined twice", p.ID)
		}
		u.prefix[p.ID] = p.Factor
		last = p.Factor
	}

	entries, err := orderedMap(&raw.Dims, "dims")
	if err != nil {
		return nil, fmt.Errorf("units: %w", err)
	}
	for _, e := range entries {
		var rd rawDim
		if err := decodeNode(e.Val, &rd); err != nil {
			fail("dim %q: %v", e.Key, err)
			continue
		}
		d := Dim{ID: e.Key, Symbol: rd.Symbol, Aliases: rd.Aliases, Prefixes: rd.Prefixes, Positive: rd.Positive}
		if d.Aliases == nil {
			d.Aliases = []string{}
		}
		if d.Prefixes == nil {
			d.Prefixes = []string{}
		}
		for _, id := range d.Prefixes {
			f, ok := u.prefix[id]
			if !ok {
				fail("dim %q: unknown prefix %q", d.ID, id)
				continue
			}
			d.pfx = append(d.pfx, Prefix{ID: id, Factor: f})
		}
		sort.Slice(d.pfx, func(i, j int) bool { return d.pfx[i].Factor < d.pfx[j].Factor })
		d.Units = make([]Unit, 0, len(rd.Units))
		seen := map[string]bool{}
		for _, ru := range rd.Units {
			un := Unit{ID: ru.ID, Symbol: ru.Symbol, Factor: ru.Factor, To: ru.To, From: ru.From}
			switch {
			case un.ID == "" || un.Symbol == "" || seen[un.ID]:
				fail("dim %q: unit %q needs a unique id and a symbol", d.ID, un.ID)
			case un.Factor > 0 && un.To == "" && un.From == "":
			case un.Factor == 0 && un.To != "" && un.From != "":
				un.to, un.from = parseUnitExpr(&errs, d.ID, un.ID, un.To), parseUnitExpr(&errs, d.ID, un.ID, un.From)
			default:
				fail("dim %q: unit %q needs either a positive factor or both to and from", d.ID, un.ID)
			}
			seen[un.ID] = true
			d.Units = append(d.Units, un)
		}
		u.Dims = append(u.Dims, d)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	for i := range u.Dims {
		d := &u.Dims[i]
		u.byDim[d.ID] = d
		for _, s := range append([]string{d.Symbol}, d.Aliases...) {
			if s != "" {
				d.suffixes = append(d.suffixes, s)
			}
		}
		sort.SliceStable(d.suffixes, func(a, b int) bool { return len(d.suffixes[a]) > len(d.suffixes[b]) })
		for j := range d.Units {
			d.exUnits = append(d.exUnits, &d.Units[j])
		}
		sort.SliceStable(d.exUnits, func(a, b int) bool { return len(d.exUnits[a].Symbol) > len(d.exUnits[b].Symbol) })
	}
	return u, nil
}

// parseUnitExpr parses a unit conversion expression, whose only variable is x.
func parseUnitExpr(errs *[]error, dim, unit, src string) *Node {
	n, err := ParseExpr(src)
	if err == nil {
		for _, dep := range Deps(n) {
			if dep != "x" {
				err = fmt.Errorf("unknown variable %q (only x is allowed)", dep)
			}
		}
	}
	if err != nil {
		*errs = append(*errs, fmt.Errorf("units: dim %q unit %q: %q: %v", dim, unit, src, err))
	}
	return n
}

// Dim returns the dimension with the given id, or nil.
func (u *Units) Dim(id string) *Dim { return u.byDim[id] }

// Unit returns the explicit unit with the given id, or nil.
func (d *Dim) Unit(id string) *Unit {
	for i := range d.Units {
		if d.Units[i].ID == id {
			return &d.Units[i]
		}
	}
	return nil
}

func (d *Dim) hasPrefix(id string) bool {
	for _, p := range d.pfx {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (d *Dim) hasAlias(a string) bool {
	for _, x := range d.Aliases {
		if x == a {
			return true
		}
	}
	return false
}

func evalUnitExpr(n *Node, x float64) float64 {
	v, err := Eval(n, Env{Vals: map[string]float64{"x": x}})
	if err != nil {
		return math.NaN()
	}
	return v
}

// ToBase converts a value in this unit to the SI base unit.
func (un *Unit) ToBase(x float64) float64 {
	if un.to != nil {
		return evalUnitExpr(un.to, x)
	}
	return x * un.Factor
}

// FromBase converts a value in the SI base unit to this unit.
func (un *Unit) FromBase(v float64) float64 {
	if un.from != nil {
		return evalUnitExpr(un.from, v)
	}
	return v / un.Factor
}

func canonPrefix(s string) string {
	switch s {
	case "u":
		return micro
	case "K":
		return "k"
	}
	return s
}

// Parse reads a number typed by the user and returns it in the SI base unit of the dimension. unitID selects an
// explicit unit ("" = none): a plain number without any unit suffix is then a value in that unit.
//
// Accepted: 4700, 4k7, 4R7 (resistance), 47R, 2u2, 10mA, "10 mA", 1e-6, 1,5k (comma as decimal point), -10dBm.
func (u *Units) Parse(input, dimID, unitID string) (float64, error) {
	d := u.byDim[dimID]
	if d == nil {
		return 0, fmt.Errorf("unknown dimension %q", dimID)
	}
	s := inputFold.Replace(strings.Join(strings.Fields(input), ""))
	return u.parseNormalized(d, strings.ReplaceAll(s, ",", "."), unitID)
}

// ParseList reads a list typed as "4k7, 10k, 2k2": items are separated by commas, semicolons or white space, and
// every item is parsed like Parse (so there is no decimal comma in lists).
func (u *Units) ParseList(input, dimID, unitID string) ([]float64, error) {
	d := u.byDim[dimID]
	if d == nil {
		return nil, fmt.Errorf("unknown dimension %q", dimID)
	}
	fields := strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) })
	if len(fields) == 0 {
		return nil, errors.New("empty list")
	}
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := u.parseNormalized(d, inputFold.Replace(f), unitID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (u *Units) parseNormalized(d *Dim, s, unitID string) (float64, error) {
	if s == "" {
		return 0, errors.New("empty value")
	}
	var sel *Unit
	if unitID != "" {
		if sel = d.Unit(unitID); sel == nil {
			return 0, fmt.Errorf("unknown unit %q for %s", unitID, d.ID)
		}
	}
	orig := s
	bad := func() (float64, error) { return 0, fmt.Errorf("cannot read %q as a %s", orig, d.ID) }

	// An explicit unit written after the number (10dBm, 2Ah).
	for _, un := range d.exUnits {
		if rest, ok := strings.CutSuffix(s, un.Symbol); ok && plainNumRe.MatchString(rest) {
			x, _ := strconv.ParseFloat(rest, 64)
			return un.ToBase(x), nil
		}
	}
	// The base symbol or an alias (V, Ω, ohm, R), possibly behind a prefix (kΩ, mA).
	hadSuffix := false
	for _, suf := range d.suffixes {
		if rest, ok := strings.CutSuffix(s, suf); ok {
			s, hadSuffix = rest, true
			break
		}
	}
	if s == "" {
		return bad()
	}

	var x float64
	prefix := ""
	if plainNumRe.MatchString(s) {
		x, _ = strconv.ParseFloat(s, 64)
	} else if m := prefixedNumRe.FindStringSubmatch(s); m != nil {
		x, _ = strconv.ParseFloat(m[1], 64)
		prefix = canonPrefix(m[2])
	} else if m := infixNumRe.FindStringSubmatch(s); m != nil {
		x, _ = strconv.ParseFloat(m[1]+"."+m[3], 64)
		prefix = canonPrefix(m[2])
	} else if m := infixRRe.FindStringSubmatch(s); m != nil && d.hasAlias("R") {
		x, _ = strconv.ParseFloat(m[1]+"."+m[2], 64)
	} else {
		return bad()
	}

	if prefix != "" {
		if !d.hasPrefix(prefix) {
			return 0, fmt.Errorf("the prefix %q is not allowed for %s", prefix, d.ID)
		}
		if sel != nil && !hadSuffix {
			return 0, fmt.Errorf("a prefix cannot be combined with the selected unit %s", sel.ID)
		}
		return x * u.prefix[prefix], nil
	}
	if sel != nil && !hadSuffix {
		return sel.ToBase(x), nil
	}
	return x, nil
}

// fmtNum writes x as JavaScript's String(x) does: the shortest decimal that reads back as the same double, without
// an exponent for 1e-6 <= |x| < 1e21, otherwise like 1e-7 or 1.5e+21.
func fmtNum(x float64) string {
	if x == 0 {
		return "0"
	}
	if ax := math.Abs(x); ax >= 1e-6 && ax < 1e21 {
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	s := strconv.FormatFloat(x, 'e', -1, 64)
	i := strings.IndexByte(s, 'e')
	exp := strings.TrimLeft(s[i+2:], "0")
	if exp == "" {
		exp = "0"
	}
	return s[:i+2] + exp
}

func fmtSig(x float64, sig int) string { return fmtNum(roundSig(x, sig)) }

func withUnit(num, unit string) string {
	if unit == "" {
		return num
	}
	return num + " " + unit
}

// Format writes v (SI base unit) with sig significant digits (default 4). With an explicit unit selected the value
// is converted into it; otherwise the SI prefix is chosen automatically (4.7 kΩ, 250 mW). Values that are not
// finite give "".
func (u *Units) Format(v float64, dimID, unitID string, sig int) string {
	d := u.byDim[dimID]
	if d == nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if sig < 1 {
		sig = 4
	}
	if un := d.Unit(unitID); un != nil {
		x := un.FromBase(v)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ""
		}
		return fmtSig(x, sig) + " " + un.Symbol
	}
	if len(d.pfx) == 0 {
		return withUnit(fmtSig(v, sig), d.Symbol)
	}
	if v == 0 {
		return withUnit("0", d.Symbol)
	}
	av := math.Abs(v)
	idx := 0
	for i, p := range d.pfx {
		if p.Factor <= av {
			idx = i
		}
	}
	m := roundSig(v/d.pfx[idx].Factor, sig)
	if math.Abs(m) >= 1000 && idx+1 < len(d.pfx) {
		idx++
		m = roundSig(v/d.pfx[idx].Factor, sig)
	}
	return withUnit(fmtNum(m), d.pfx[idx].ID+d.Symbol)
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/yamlutil.go internal/ecalc/units.go internal/ecalc/units_test.go internal/ecalc/catalog/units.yaml internal/ecalc/testdata/vectors/core.json
git commit -m "feat(ecalc): units model with number parsing and engineering-notation formatting" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A4: Catalog schema, strict loader and validation

**Files:**
- Create: `internal/ecalc/catalog.go`
- Create: `internal/ecalc/catalog/categories.yaml`
- Test: `internal/ecalc/catalog_test.go`

- [ ] **Step 1: Write the failing test**

The test builds a small valid calculator (`baseCalc`) in an in-memory `fstest.MapFS` next to the real `units.yaml` and `categories.yaml`, checks that it loads with the declaration order kept, then mutates the YAML one defect at a time and expects `Load` to reject it with a specific message: a missing locale, an unknown language key, a missing German doc, a missing source, an unknown category or kind, an unknown YAML field, an unknown dimension or display unit, bad defaults, a wrong number of defaults, rules and formulas that do not parse, unknown names, a rule that uses its own target, a reserved variable name, a list variable outside `sum`/`par`, and a choice const that collides with a variable. Duplicate ids across files and empty files are checked separately. `TestDefaultCatalogLoads` checks the 13 categories in rail order, all 16 names for each, and that `Default()` returns one shared catalog.

Create `internal/ecalc/catalog_test.go` with exactly this content:

```go
package ecalc

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// flow16 writes {cs: "v", da: "v", ...} for all 16 locales.
func flow16(v string) string {
	parts := make([]string, len(Locales))
	for i, l := range Locales {
		parts[i] = l + `: "` + v + `"`
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// baseCalc is a small valid calculator: current = voltage / resistance with two defaults.
var baseCalc = strings.ReplaceAll(`- id: t_div
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    a: {dim: voltage, label: A, name: {en: "A", de: "A"}, default: "10 V"}
    b: {dim: resistance, label: B, name: {en: "B", de: "B"}, default: "100"}
    c: {dim: current, label: C, name: {en: "C", de: "C"}}
  dof: 2
  rules:
    c: ["a/b"]
    a: ["c*b"]
    b: ["a/c"]
  derived:
    p: {dim: power, label: P, name: {en: "P", de: "P"}, expr: "a*c"}
  checks:
    - {when: "c > 1", warn: {en: "Too much.", de: "Zu viel."}}
  formula: ["c = a / b"]
`, "NAME16", flow16("T"))

func fixtureFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	units, err := os.ReadFile("catalog/units.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cats, err := os.ReadFile("catalog/categories.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"units.yaml": {Data: units}, "categories.yaml": {Data: cats}}
	for name, data := range files {
		fsys["calculators/"+name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestDefaultCatalogLoads(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	var ids []string
	for _, c := range cat.Categories {
		ids = append(ids, c.ID)
		if c.Name["en"] == "" || c.Name["de"] == "" || len(c.Name) != 16 {
			t.Errorf("category %s: names %v", c.ID, c.Name)
		}
	}
	want := "basics passive filters semiconductors power opamps timers rf digital wiring tables codes pinouts"
	if strings.Join(ids, " ") != want {
		t.Errorf("categories = %v, want %s", ids, want)
	}
	for _, dim := range []string{"voltage", "current", "resistance", "power", "charge", "energy", "number", "gain"} {
		if cat.Units.Dim(dim) == nil {
			t.Errorf("units.yaml lacks the dimension %s", dim)
		}
	}
	if again, _ := Default(); again != cat {
		t.Error("Default must return the same catalog every time")
	}
}

func TestLoadFixtureBuildsTheCalculator(t *testing.T) {
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": baseCalc}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cat.Calculator("t_div")
	if c == nil || c.DOF != 2 || len(c.Vars) != 3 || len(c.Rules) != 3 || len(c.Derived) != 1 || len(c.Checks) != 1 {
		t.Fatalf("unexpected calculator %+v", c)
	}
	if c.Vars[0].ID != "a" || c.Vars[2].ID != "c" || c.Rules[0].Target != "c" {
		t.Errorf("declaration order was not kept: %v / %v", c.Vars, c.Rules)
	}
	if a := c.Var("a"); a.Default == nil || *a.Default != 10 || !a.Input {
		t.Errorf("var a = %+v, want default 10 and input", a)
	}
	if b := c.Var("b"); b.Default == nil || *b.Default != 100 {
		t.Errorf("var b = %+v, want default 100", b)
	}
	if c.Var("c").Default != nil {
		t.Error("var c has no default")
	}
	if c.Checks[0].When.Src != "c > 1" || c.FormulaAST[0].Sexp() != "(= c (/ a b))" {
		t.Errorf("check/formula not parsed: %+v %s", c.Checks[0], c.FormulaAST[0].Sexp())
	}
}

func TestLoadRejectsBrokenCalculators(t *testing.T) {
	cases := []struct{ name, old, new, want string }{
		{"missing locale", `, zh: "T"}`, `}`, `missing "zh" text`},
		{"unknown language key", `de: "Doku."`, `de: "Doku.", xx: "Y"`, `unknown language key "xx"`},
		{"missing german doc", `doc: {en: "Doc.", de: "Doku."}`, `doc: {en: "Doc."}`, `doc: missing "de" text`},
		{"missing source", `source: "Textbook"`, `source: ""`, "source is required"},
		{"unknown category", `category: basics`, `category: nowhere`, `unknown category "nowhere"`},
		{"unknown kind", `kind: calculator`, `kind: table`, "kind must be calculator"},
		{"unknown field", `label: A,`, `label: A, colour: red,`, "colour"},
		{"unknown dimension", `dim: current`, `dim: speed`, `unknown dimension "speed"`},
		{"unknown display unit", `default: "10 V"}`, `default: "10 V", unit: dBm}`, `unit "dBm" is not an explicit unit`},
		{"unparsable default", `default: "10 V"`, `default: "ten"`, "default"},
		{"zero default of a positive dimension", `default: "100"`, `default: "0"`, "greater than zero"},
		{"wrong number of defaults", `, default: "100"}`, `}`, "exactly 2 input variables need a default, found 1"},
		{"rule that does not parse", `c: ["a/b"]`, `c: ["a/"]`, "rule c"},
		{"rule with an unknown name", `c: ["a/b"]`, `c: ["a/zz"]`, `unknown name "zz"`},
		{"rule using its own target", `a: ["c*b"]`, `a: ["c*a"]`, "uses its own target"},
		{"rule target that is not a variable", `b: ["a/c"]`, `q: ["a/c"]`, `rule target "q" is not a variable`},
		{"variable named like a function", `    c: {dim: current,`, `    min: {dim: current,`, `var "min" is reserved`},
		{"list variable outside sum", `default: "100"}`, `default: ["100", "200"], list: true}`, "used outside sum() or par()"},
		{"check with an unknown name", `when: "c > 1"`, `when: "zz > 1"`, `unknown name "zz"`},
		{"formula that does not parse", `formula: ["c = a / b"]`, `formula: ["c = a / "]`, "formula"},
		{"choice const colliding with a variable", `  dof: 2`,
			"  choices:\n    q: {name: " + flow16("Q") + ", default: o, options: {o: {name: " + flow16("O") + ", consts: {a: 1}}, p: {name: " + flow16("P") + ", consts: {a: 2}}}}\n  dof: 2",
			`choice const "a" collides with the var`},
	}
	for _, c := range cases {
		if !strings.Contains(baseCalc, c.old) {
			t.Fatalf("%s: fixture does not contain %q", c.name, c.old)
		}
		broken := strings.Replace(baseCalc, c.old, c.new, 1)
		_, err := Load(fixtureFS(t, map[string]string{"t.yaml": broken}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

func TestLoadRejectsDuplicateIDsAndEmptyFiles(t *testing.T) {
	_, err := Load(fixtureFS(t, map[string]string{"a.yaml": baseCalc, "b.yaml": baseCalc}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in calculators/a.yaml") {
		t.Errorf("duplicate id: error = %v", err)
	}
	_, err = Load(fixtureFS(t, map[string]string{"a.yaml": ""}))
	if err == nil || !strings.Contains(err.Error(), "the file is empty") {
		t.Errorf("empty file: error = %v", err)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run 'TestDefaultCatalog|TestLoad' -count=1
```

Expected: **FAIL** — `Locales`, `Default` and `Load` do not exist yet:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\catalog_test.go:12:30: undefined: Locales
internal\ecalc\catalog_test.go:13:20: undefined: Locales
internal\ecalc\catalog_test.go:61:14: undefined: Default
internal\ecalc\catalog_test.go:81:17: undefined: Default
internal\ecalc\catalog_test.go:87:14: undefined: Load
internal\ecalc\catalog_test.go:143:13: undefined: Load
internal\ecalc\catalog_test.go:151:12: undefined: Load
```

- [ ] **Step 3: Implement the catalog and the category rail**

`catalog.go` embeds `catalog/` with `go:embed`, decodes with `KnownFields(true)`, keeps `vars`, `choices`, `rules` and `derived` in file order (through `yaml.Node` and `orderedMap`), parses every expression and formula, and collects *all* problems with `errors.Join` so an author sees every defect at once. The solvable-set rules of CONTRACT §6 arrive with the solver in Task A5. `categories.yaml` holds the 13 categories of CONTRACT §2 in rail order with real names in all 16 locales.

Create `internal/ecalc/catalog.go` with exactly this content:

```go
package ecalc

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed catalog
var catalogFS embed.FS

// Locales are the 16 desktop locales. Every name in the catalog is written in all of them; docs and warnings need
// English and German and fall back to English for the rest.
var Locales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

var docLocales = []string{"en", "de"}

var (
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	schematicRe = regexp.MustCompile(`^[a-z0-9_]*$`)
)

// Text is a localized string: language -> text.
type Text map[string]string

// Category is one entry of the category rail.
type Category struct {
	ID   string
	Name Text
}

// Expr is one parsed expression with the names it uses.
type Expr struct {
	Src  string
	Node *Node
	Deps []string
}

// Var is a primary variable of a calculator.
type Var struct {
	ID          string
	Dim         string
	Label       string
	Name        Text
	Default     *float64  // SI value, nil when the variable has no default
	DefaultList []float64 // SI values of a list variable's default
	Unit        string    // default display unit id, "" = automatic
	ESeries     bool
	List        bool
	Input       bool // can be an input (false: only ever computed)

	dim *Dim
}

// ChoiceOption is one option of a choice; its consts become known names while it is selected.
type ChoiceOption struct {
	ID     string
	Name   Text
	Consts map[string]float64
}

// Choice is a selector (for example the waveform) whose selected option supplies constants to the rules.
type Choice struct {
	ID      string
	Name    Text
	Default string
	Options []ChoiceOption
}

// Rule computes one variable. Alts are alternatives; the first one whose names are all known is applied.
type Rule struct {
	Target string
	Alts   []Expr
}

// Derived is a quantity computed after (and, through the rules, during) solving.
type Derived struct {
	ID    string
	Dim   string
	Label string
	Name  Text
	Alts  []Expr
	Unit  string // fixed display unit id, "" = automatic

	dim *Dim
}

// Check shows Warn when When evaluates to a non-zero number.
type Check struct {
	When Expr
	Warn Text
}

// Calculator is a catalog item of kind "calculator".
type Calculator struct {
	ID         string
	Kind       string
	Category   string
	Name       Text
	Doc        Text
	Tags       []string
	Source     string
	Vars       []Var
	Choices    []Choice
	DOF        int
	Rules      []Rule
	Derived    []Derived
	Checks     []Check
	Formula    []string
	FormulaAST []*Node
	Schematic  string

	varByID map[string]*Var
}

// Var returns the primary variable with the given id, or nil.
func (c *Calculator) Var(id string) *Var { return c.varByID[id] }

// Catalog is the validated ECalc catalog.
type Catalog struct {
	Units       *Units
	Categories  []Category
	Calculators []*Calculator

	byID map[string]*Calculator
}

// Calculator returns the calculator with the given id, or nil.
func (c *Catalog) Calculator(id string) *Calculator { return c.byID[id] }

var (
	defaultOnce sync.Once
	defaultCat  *Catalog
	defaultErr  error
)

// Default returns the embedded catalog, loaded and validated once.
func Default() (*Catalog, error) {
	defaultOnce.Do(func() {
		sub, err := fs.Sub(catalogFS, "catalog")
		if err != nil {
			defaultErr = err
			return
		}
		defaultCat, defaultErr = Load(sub)
	})
	return defaultCat, defaultErr
}

type rawCheck struct {
	When string `yaml:"when"`
	Warn Text   `yaml:"warn"`
}

type rawCalc struct {
	ID        string     `yaml:"id"`
	Kind      string     `yaml:"kind"`
	Category  string     `yaml:"category"`
	Name      Text       `yaml:"name"`
	Doc       Text       `yaml:"doc"`
	Tags      []string   `yaml:"tags"`
	Source    string     `yaml:"source"`
	Vars      yaml.Node  `yaml:"vars"`
	Choices   yaml.Node  `yaml:"choices"`
	DOF       int        `yaml:"dof"`
	Rules     yaml.Node  `yaml:"rules"`
	Derived   yaml.Node  `yaml:"derived"`
	Checks    []rawCheck `yaml:"checks"`
	Formula   []string   `yaml:"formula"`
	Schematic string     `yaml:"schematic"`
}

type rawVar struct {
	Dim     string    `yaml:"dim"`
	Label   string    `yaml:"label"`
	Name    Text      `yaml:"name"`
	Default yaml.Node `yaml:"default"`
	Unit    string    `yaml:"unit"`
	ESeries bool      `yaml:"eseries"`
	List    bool      `yaml:"list"`
	Input   *bool     `yaml:"input"`
}

type rawChoice struct {
	Name    Text      `yaml:"name"`
	Default string    `yaml:"default"`
	Options yaml.Node `yaml:"options"`
}

type rawOption struct {
	Name   Text               `yaml:"name"`
	Consts map[string]float64 `yaml:"consts"`
}

type rawDerived struct {
	Dim   string    `yaml:"dim"`
	Label string    `yaml:"label"`
	Name  Text      `yaml:"name"`
	Expr  yaml.Node `yaml:"expr"`
	Unit  string    `yaml:"unit"`
}

type loader struct {
	units *Units
	cats  map[string]bool
	ids   map[string]string // item id -> file that defines it
	errs  []error
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

// Load reads and validates a catalog: units.yaml, categories.yaml and calculators/*.yaml at the root of fsys. All
// problems found are reported together.
func Load(fsys fs.FS) (*Catalog, error) {
	unitsData, err := fs.ReadFile(fsys, "units.yaml")
	if err != nil {
		return nil, fmt.Errorf("ecalc catalog: %w", err)
	}
	units, err := loadUnits(unitsData)
	if err != nil {
		return nil, err
	}
	l := &loader{units: units, cats: map[string]bool{}, ids: map[string]string{}}
	cat := &Catalog{Units: units, byID: map[string]*Calculator{}}

	catData, err := fs.ReadFile(fsys, "categories.yaml")
	if err != nil {
		return nil, fmt.Errorf("ecalc catalog: %w", err)
	}
	cat.Categories = l.categories(catData)

	files, err := fs.Glob(fsys, "calculators/*.yaml")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			l.fail("%s: %v", file, err)
			continue
		}
		var raws []rawCalc
		if err := decodeStrict(data, &raws); err != nil {
			if errors.Is(err, io.EOF) {
				err = errors.New("the file is empty")
			}
			l.fail("%s: %v", file, err)
			continue
		}
		for i := range raws {
			if c := l.calculator(file, &raws[i]); c != nil {
				cat.Calculators = append(cat.Calculators, c)
				cat.byID[c.ID] = c
			}
		}
	}
	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return cat, nil
}

func (l *loader) categories(data []byte) []Category {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		l.fail("categories.yaml: %v", err)
		return nil
	}
	if len(doc.Content) == 0 {
		l.fail("categories.yaml: the file is empty")
		return nil
	}
	entries, err := orderedMap(doc.Content[0], "categories.yaml")
	if err != nil {
		l.fail("%v", err)
		return nil
	}
	var out []Category
	for _, e := range entries {
		var rc struct {
			Name Text `yaml:"name"`
		}
		if err := decodeNode(e.Val, &rc); err != nil {
			l.fail("categories.yaml: %s: %v", e.Key, err)
			continue
		}
		if !idRe.MatchString(e.Key) {
			l.fail("categories.yaml: category id %q must match %s", e.Key, idRe)
		}
		l.checkText("categories.yaml: "+e.Key+": name", rc.Name, Locales)
		l.cats[e.Key] = true
		out = append(out, Category{ID: e.Key, Name: rc.Name})
	}
	return out
}

// checkText reports missing required languages, empty texts and keys that are not desktop locales.
func (l *loader) checkText(what string, t Text, required []string) {
	for _, lang := range required {
		if strings.TrimSpace(t[lang]) == "" {
			l.fail("%s: missing %q text", what, lang)
		}
	}
	langs := make([]string, 0, len(t))
	for lang := range t {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		if !isLocale(lang) {
			l.fail("%s: unknown language key %q", what, lang)
		} else if strings.TrimSpace(t[lang]) == "" && !contains(required, lang) {
			l.fail("%s: empty %q text", what, lang)
		}
	}
}

func isLocale(lang string) bool { return contains(Locales, lang) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func newExpr(src string) (Expr, error) {
	n, err := ParseExpr(src)
	if err != nil {
		return Expr{}, fmt.Errorf("%q: %w", src, err)
	}
	return Expr{Src: src, Node: n, Deps: Deps(n)}, nil
}

// exprSources reads a YAML node that is one expression or a list of alternative expressions.
func exprSources(n *yaml.Node) ([]string, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(n.Content))
		for _, it := range n.Content {
			if it.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("alternatives must be expression strings (line %d)", it.Line)
			}
			out = append(out, it.Value)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("the list of alternatives is empty (line %d)", n.Line)
		}
		return out, nil
	}
	return nil, fmt.Errorf("an expression or a list of expressions is required (line %d)", n.Line)
}

// scalarValue reads a default: a string is parsed like user input (with the variable's display unit selected), a
// number is already in SI units.
func (l *loader) scalarValue(n *yaml.Node, dim *Dim, unit string) (float64, error) {
	if n.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("a number or a string is required (line %d)", n.Line)
	}
	if n.Tag == "!!str" {
		return l.units.Parse(n.Value, dim.ID, unit)
	}
	return strconv.ParseFloat(n.Value, 64)
}

// calculator builds and validates one calculator; it returns nil when something is wrong.
func (l *loader) calculator(file string, r *rawCalc) *Calculator {
	before := len(l.errs)
	label := r.ID
	if label == "" {
		label = "(item without id)"
	}
	fail := func(format string, args ...any) {
		l.errs = append(l.errs, fmt.Errorf("%s: %s: %s", file, label, fmt.Sprintf(format, args...)))
	}
	text := func(what string, t Text, required []string) {
		l.checkText(fmt.Sprintf("%s: %s: %s", file, label, what), t, required)
	}

	c := &Calculator{
		ID: r.ID, Kind: r.Kind, Category: r.Category, Name: r.Name, Doc: r.Doc, Tags: r.Tags,
		Source: r.Source, DOF: r.DOF, Formula: r.Formula, Schematic: r.Schematic,
		varByID: map[string]*Var{},
	}
	switch {
	case !idRe.MatchString(r.ID):
		fail("id must match %s", idRe)
	case l.ids[r.ID] != "":
		fail("id is already used in %s", l.ids[r.ID])
	default:
		l.ids[r.ID] = file
	}
	if r.Kind != "calculator" {
		fail("kind must be calculator, got %q", r.Kind)
	}
	if !l.cats[r.Category] {
		fail("unknown category %q", r.Category)
	}
	text("name", r.Name, Locales)
	text("doc", r.Doc, docLocales)
	if strings.TrimSpace(r.Source) == "" {
		fail("source is required (a standard, datasheet or textbook)")
	}
	if len(r.Tags) == 0 {
		fail("at least one tag is required")
	}
	for _, tag := range r.Tags {
		if strings.TrimSpace(tag) == "" {
			fail("tags must not be empty")
		}
	}
	if !schematicRe.MatchString(r.Schematic) {
		fail("schematic %q must match %s", r.Schematic, schematicRe)
	}

	// names maps every name an expression may use to what it is.
	names := map[string]string{}
	claim := func(name, what string) bool {
		switch {
		case !idRe.MatchString(name):
			fail("%s %q must match %s", what, name, idRe)
		case name == "pi" || isFunc(name):
			fail("%s %q is reserved (constant or function name)", what, name)
		case names[name] != "":
			fail("%s %q collides with the %s of the same name", what, name, names[name])
		default:
			names[name] = what
			return true
		}
		return false
	}

	// Variables.
	varEntries, err := orderedMap(&r.Vars, "vars")
	if err != nil {
		fail("%v", err)
	}
	if len(varEntries) == 0 {
		fail("vars are required")
	}
	lists := map[string]bool{}
	c.Vars = make([]Var, 0, len(varEntries))
	for _, e := range varEntries {
		var rv rawVar
		if err := decodeNode(e.Val, &rv); err != nil {
			fail("var %s: %v", e.Key, err)
			continue
		}
		v := Var{ID: e.Key, Dim: rv.Dim, Label: rv.Label, Name: rv.Name, Unit: rv.Unit, ESeries: rv.ESeries,
			List: rv.List, Input: rv.Input == nil || *rv.Input}
		claim(v.ID, "var")
		v.dim = l.units.Dim(v.Dim)
		if v.dim == nil {
			fail("var %s: unknown dimension %q", v.ID, v.Dim)
		} else {
			if v.Unit != "" && v.dim.Unit(v.Unit) == nil {
				fail("var %s: unit %q is not an explicit unit of %s", v.ID, v.Unit, v.Dim)
			}
			l.defaults(fail, &v, &rv.Default)
		}
		if strings.TrimSpace(v.Label) == "" {
			fail("var %s: label is required", v.ID)
		}
		text("var "+v.ID+": name", v.Name, docLocales)
		if v.List {
			lists[v.ID] = true
		}
		c.Vars = append(c.Vars, v)
	}
	for i := range c.Vars {
		c.varByID[c.Vars[i].ID] = &c.Vars[i]
	}

	// Choices: their consts are known names whatever option is selected.
	choiceEntries, err := orderedMap(&r.Choices, "choices")
	if err != nil {
		fail("%v", err)
	}
	for _, e := range choiceEntries {
		var rc rawChoice
		if err := decodeNode(e.Val, &rc); err != nil {
			fail("choice %s: %v", e.Key, err)
			continue
		}
		if !idRe.MatchString(e.Key) {
			fail("choice id %q must match %s", e.Key, idRe)
		}
		ch := Choice{ID: e.Key, Name: rc.Name, Default: rc.Default}
		text("choice "+ch.ID+": name", ch.Name, Locales)
		optEntries, err := orderedMap(&rc.Options, "choice "+ch.ID+": options")
		if err != nil {
			fail("%v", err)
		}
		if len(optEntries) < 2 {
			fail("choice %s needs at least two options", ch.ID)
		}
		var constNames []string
		for i, oe := range optEntries {
			var ro rawOption
			if err := decodeNode(oe.Val, &ro); err != nil {
				fail("choice %s option %s: %v", ch.ID, oe.Key, err)
				continue
			}
			opt := ChoiceOption{ID: oe.Key, Name: ro.Name, Consts: ro.Consts}
			text("choice "+ch.ID+" option "+opt.ID+": name", opt.Name, Locales)
			keys := make([]string, 0, len(opt.Consts))
			for k := range opt.Consts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if i == 0 {
				constNames = keys
				for _, k := range keys {
					claim(k, "choice const")
				}
			} else if strings.Join(keys, ",") != strings.Join(constNames, ",") {
				fail("choice %s: option %s defines different consts than %s", ch.ID, opt.ID, ch.Options[0].ID)
			}
			ch.Options = append(ch.Options, opt)
		}
		if !hasOption(ch, ch.Default) {
			fail("choice %s: default %q is not one of its options", ch.ID, ch.Default)
		}
		c.Choices = append(c.Choices, ch)
	}

	// Derived quantities.
	derivedEntries, err := orderedMap(&r.Derived, "derived")
	if err != nil {
		fail("%v", err)
	}
	for _, e := range derivedEntries {
		var rd rawDerived
		if err := decodeNode(e.Val, &rd); err != nil {
			fail("derived %s: %v", e.Key, err)
			continue
		}
		d := Derived{ID: e.Key, Dim: rd.Dim, Label: rd.Label, Name: rd.Name, Unit: rd.Unit}
		claim(d.ID, "derived")
		if d.dim = l.units.Dim(d.Dim); d.dim == nil {
			fail("derived %s: unknown dimension %q", d.ID, d.Dim)
		} else if d.Unit != "" && d.dim.Unit(d.Unit) == nil {
			fail("derived %s: unit %q is not an explicit unit of %s", d.ID, d.Unit, d.Dim)
		}
		if strings.TrimSpace(d.Label) == "" {
			fail("derived %s: label is required", d.ID)
		}
		text("derived "+d.ID+": name", d.Name, docLocales)
		d.Alts = l.alternatives(fail, "derived "+d.ID, &rd.Expr)
		c.Derived = append(c.Derived, d)
	}

	// Rules.
	ruleEntries, err := orderedMap(&r.Rules, "rules")
	if err != nil {
		fail("%v", err)
	}
	isTarget := map[string]bool{}
	for _, e := range ruleEntries {
		v := c.Var(e.Key)
		switch {
		case v == nil:
			fail("rule target %q is not a variable", e.Key)
		case v.List:
			fail("rule target %q is a list variable", e.Key)
		}
		isTarget[e.Key] = true
		rule := Rule{Target: e.Key, Alts: l.alternatives(fail, "rule "+e.Key, e.Val)}
		for _, alt := range rule.Alts {
			if contains(alt.Deps, e.Key) {
				fail("rule %s uses its own target", e.Key)
			}
		}
		c.Rules = append(c.Rules, rule)
	}

	// Every expression may only use known names; lists only inside sum() and par().
	known := func(what string, ex Expr) {
		for _, dep := range ex.Deps {
			if names[dep] == "" {
				fail("%s: unknown name %q in %q", what, dep, ex.Src)
			}
		}
		checkListUse(ex.Node, lists, func(msg string) { fail("%s: %s in %q", what, msg, ex.Src) })
	}
	for _, rule := range c.Rules {
		for _, alt := range rule.Alts {
			known("rule "+rule.Target, alt)
		}
	}
	for _, d := range c.Derived {
		for _, alt := range d.Alts {
			known("derived "+d.ID, alt)
		}
	}

	// Checks.
	for i, rc := range r.Checks {
		ex, err := newExpr(rc.When)
		if err != nil {
			fail("check %d: %v", i+1, err)
			continue
		}
		known(fmt.Sprintf("check %d", i+1), ex)
		text(fmt.Sprintf("check %d: warn", i+1), rc.Warn, docLocales)
		c.Checks = append(c.Checks, Check{When: ex, Warn: rc.Warn})
	}

	// Display formula.
	if len(r.Formula) == 0 {
		fail("formula is required")
	}
	for _, line := range r.Formula {
		n, err := ParseFormula(line)
		if err != nil {
			fail("formula %q: %v", line, err)
			continue
		}
		c.FormulaAST = append(c.FormulaAST, n)
		for _, part := range n.Parts {
			known("formula", Expr{Src: line, Node: part, Deps: Deps(part)})
		}
	}

	// Inputs and defaults: exactly dof input-capable variables carry a default, and every variable can be known.
	inputs, withDefault := 0, 0
	for _, v := range c.Vars {
		if v.Input {
			inputs++
			if v.Default != nil || v.DefaultList != nil {
				withDefault++
			}
		} else if v.Default != nil || v.DefaultList != nil {
			fail("var %s is not an input and must not have a default", v.ID)
		}
		if !v.Input && !isTarget[v.ID] {
			fail("var %s is neither an input nor the target of a rule", v.ID)
		}
	}
	switch {
	case c.DOF < 1:
		fail("dof must be at least 1")
	case inputs < c.DOF:
		fail("dof is %d but only %d variables can be inputs", c.DOF, inputs)
	case withDefault != c.DOF:
		fail("dof is %d, so exactly %d input variables need a default, found %d", c.DOF, c.DOF, withDefault)
	}

	if len(l.errs) > before {
		return nil
	}
	return c
}

func hasOption(ch Choice, id string) bool {
	for _, o := range ch.Options {
		if o.ID == id {
			return true
		}
	}
	return false
}

// alternatives parses the expression or list of alternatives of a rule or derived quantity.
func (l *loader) alternatives(fail func(string, ...any), what string, n *yaml.Node) []Expr {
	if n.Kind == 0 {
		fail("%s: an expression is required", what)
		return nil
	}
	srcs, err := exprSources(n)
	if err != nil {
		fail("%s: %v", what, err)
		return nil
	}
	var out []Expr
	for _, src := range srcs {
		ex, err := newExpr(src)
		if err != nil {
			fail("%s: %v", what, err)
			continue
		}
		out = append(out, ex)
	}
	return out
}

// defaults reads and checks the default of a variable.
func (l *loader) defaults(fail func(string, ...any), v *Var, n *yaml.Node) {
	if n.Kind == 0 {
		return
	}
	check := func(x float64) bool {
		switch {
		case math.IsNaN(x) || math.IsInf(x, 0):
			fail("var %s: the default must be finite", v.ID)
		case v.dim.Positive && x <= 0:
			fail("var %s: the default must be greater than zero", v.ID)
		default:
			return true
		}
		return false
	}
	if v.List {
		if n.Kind != yaml.SequenceNode {
			fail("var %s: the default of a list variable must be a list", v.ID)
			return
		}
		list := make([]float64, 0, len(n.Content))
		for _, it := range n.Content {
			x, err := l.scalarValue(it, v.dim, v.Unit)
			if err != nil {
				fail("var %s: default: %v", v.ID, err)
				return
			}
			if !check(x) {
				return
			}
			list = append(list, x)
		}
		v.DefaultList = list
		return
	}
	x, err := l.scalarValue(n, v.dim, v.Unit)
	if err != nil {
		fail("var %s: default: %v", v.ID, err)
		return
	}
	if check(x) {
		v.Default = &x
	}
}

// checkListUse reports a list variable used outside sum()/par() and a sum()/par() argument that is not a list.
func checkListUse(n *Node, lists map[string]bool, report func(string)) {
	if n == nil {
		return
	}
	if n.T == "call" && (n.Name == "sum" || n.Name == "par") {
		if a := n.Args[0]; a.T != "id" || !lists[a.Name] {
			report(n.Name + "() needs a list variable")
		}
		return
	}
	if n.T == "id" && lists[n.Name] {
		report(fmt.Sprintf("list variable %q is used outside sum() or par()", n.Name))
	}
	checkListUse(n.A, lists, report)
	checkListUse(n.L, lists, report)
	checkListUse(n.R, lists, report)
	for _, a := range n.Args {
		checkListUse(a, lists, report)
	}
	for _, a := range n.Parts {
		checkListUse(a, lists, report)
	}
}
```

Create `internal/ecalc/catalog/categories.yaml` with exactly this content:

```yaml
# The category rail of the ECalc app, in display order. Every name is written in all 16 desktop locales.
basics:
  name: {cs: "Základy", da: "Grundlæggende", de: "Grundlagen", el: "Βασικά", en: "Basics", es: "Fundamentos", fr: "Bases", hi: "बुनियादी", it: "Fondamenti", ja: "基礎", nl: "Basis", no: "Grunnleggende", pl: "Podstawy", pt: "Fundamentos", sv: "Grunder", zh: "基础"}
passive:
  name: {cs: "Pasivní součástky", da: "Passive komponenter", de: "Passive Bauteile", el: "Παθητικά εξαρτήματα", en: "Passive", es: "Componentes pasivos", fr: "Composants passifs", hi: "निष्क्रिय घटक", it: "Componenti passivi", ja: "受動部品", nl: "Passieve componenten", no: "Passive komponenter", pl: "Elementy bierne", pt: "Componentes passivos", sv: "Passiva komponenter", zh: "无源元件"}
filters:
  name: {cs: "Filtry", da: "Filtre", de: "Filter", el: "Φίλτρα", en: "Filters", es: "Filtros", fr: "Filtres", hi: "फ़िल्टर", it: "Filtri", ja: "フィルタ", nl: "Filters", no: "Filtre", pl: "Filtry", pt: "Filtros", sv: "Filter", zh: "滤波器"}
semiconductors:
  name: {cs: "Polovodiče", da: "Halvledere", de: "Halbleiter", el: "Ημιαγωγοί", en: "Semiconductors", es: "Semiconductores", fr: "Semi-conducteurs", hi: "अर्धचालक", it: "Semiconduttori", ja: "半導体", nl: "Halfgeleiders", no: "Halvledere", pl: "Półprzewodniki", pt: "Semicondutores", sv: "Halvledare", zh: "半导体"}
power:
  name: {cs: "Napájení", da: "Strømforsyning", de: "Stromversorgung", el: "Τροφοδοσία", en: "Power supply", es: "Fuentes de alimentación", fr: "Alimentation", hi: "बिजली आपूर्ति", it: "Alimentazione", ja: "電源", nl: "Voeding", no: "Strømforsyning", pl: "Zasilanie", pt: "Fontes de alimentação", sv: "Strömförsörjning", zh: "电源"}
opamps:
  name: {cs: "Operační zesilovače", da: "Operationsforstærkere", de: "OpAmps", el: "Τελεστικοί ενισχυτές", en: "Op-amps", es: "Amplificadores operacionales", fr: "Amplificateurs opérationnels", hi: "ऑप-एम्प", it: "Amplificatori operazionali", ja: "オペアンプ", nl: "Operationele versterkers", no: "Operasjonsforsterkere", pl: "Wzmacniacze operacyjne", pt: "Amplificadores operacionais", sv: "Operationsförstärkare", zh: "运算放大器"}
timers:
  name: {cs: "Časovače", da: "Timere", de: "Timer", el: "Χρονιστές", en: "Timers", es: "Temporizadores", fr: "Temporisateurs", hi: "टाइमर", it: "Temporizzatori", ja: "タイマー", nl: "Timers", no: "Timere", pl: "Timery", pt: "Temporizadores", sv: "Timers", zh: "定时器"}
rf:
  name: {cs: "Vysoké frekvence", da: "Højfrekvens", de: "HF", el: "Ραδιοσυχνότητες (RF)", en: "RF", es: "Radiofrecuencia", fr: "Radiofréquence", hi: "आरएफ", it: "Radiofrequenza", ja: "高周波", nl: "Radiofrequentie", no: "Høyfrekvens", pl: "Wysoka częstotliwość", pt: "Radiofrequência", sv: "Radiofrekvens", zh: "射频"}
digital:
  name: {cs: "Digitální technika", da: "Digital", de: "Digital", el: "Ψηφιακά", en: "Digital", es: "Digital", fr: "Numérique", hi: "डिजिटल", it: "Digitale", ja: "デジタル", nl: "Digitaal", no: "Digital", pl: "Cyfrowe", pt: "Digital", sv: "Digital", zh: "数字电路"}
wiring:
  name: {cs: "Vodiče a DPS", da: "Ledninger og print", de: "Leitungen & PCB", el: "Καλωδίωση & PCB", en: "Wiring & PCB", es: "Cableado y PCB", fr: "Câblage et PCB", hi: "वायरिंग और पीसीबी", it: "Cablaggio e PCB", ja: "配線とプリント基板", nl: "Bedrading en pcb", no: "Ledninger og kretskort", pl: "Okablowanie i PCB", pt: "Cablagem e PCB", sv: "Kablage och kretskort", zh: "布线与PCB"}
tables:
  name: {cs: "Tabulky", da: "Tabeller", de: "Tabellen", el: "Πίνακες", en: "Tables", es: "Tablas", fr: "Tableaux", hi: "तालिकाएँ", it: "Tabelle", ja: "表", nl: "Tabellen", no: "Tabeller", pl: "Tabele", pt: "Tabelas", sv: "Tabeller", zh: "表格"}
codes:
  name: {cs: "Kódy součástek", da: "Komponentkoder", de: "Bauteilcodes", el: "Κωδικοί εξαρτημάτων", en: "Codes", es: "Códigos de componentes", fr: "Codes des composants", hi: "घटक कोड", it: "Codici dei componenti", ja: "部品コード", nl: "Componentcodes", no: "Komponentkoder", pl: "Kody elementów", pt: "Códigos de componentes", sv: "Komponentkoder", zh: "元件代码"}
pinouts:
  name: {cs: "Zapojení pinů", da: "Benforbindelser", de: "Pinbelegungen", el: "Αντιστοίχιση ακροδεκτών", en: "Pinouts", es: "Distribución de pines", fr: "Brochages", hi: "पिनआउट", it: "Piedinature", ja: "ピン配置", nl: "Pinbezetting", no: "Pinnekonfigurasjon", pl: "Rozmieszczenie wyprowadzeń", pt: "Pinagens", sv: "Stiftkonfiguration", zh: "引脚定义"}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/catalog.go internal/ecalc/catalog_test.go internal/ecalc/catalog/categories.yaml
git commit -m "feat(ecalc): catalog schema, strict loader and validation with the category rail" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A5: Rule propagation solver and solvable input sets

**Files:**
- Create: `internal/ecalc/solve.go`
- Modify: `internal/ecalc/catalog.go`
- Test: `internal/ecalc/solve_test.go`

- [ ] **Step 1: Write the failing test**

`mixCalc` has a choice (constant `f`), a list variable and derived quantities that reduce the list; together with the base calculator of Task A4 it covers propagation order, computed names, warnings, `undetermined`, `range` and `nonfinite` errors, the default option for an unknown choice id, list elements, and the solvable sets. The last test loads a calculator with an input variable nobody can compute and expects the three set errors.

Create `internal/ecalc/solve_test.go` with exactly this content:

```go
package ecalc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// mixCalc has a choice (constant f), a list variable and derived quantities that reduce the list.
var mixCalc = strings.ReplaceAll(`- id: t_mix
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    x: {dim: number, label: X, name: {en: "X", de: "X"}, default: 3}
    rs: {dim: resistance, label: R, name: {en: "R", de: "R"}, list: true, default: ["100", "300"]}
    y: {dim: number, label: Y, name: {en: "Y", de: "Y"}}
  choices:
    m: {name: NAME16, default: dbl, options: {dbl: {name: NAME16, consts: {f: 2}}, tri: {name: NAME16, consts: {f: 3}}}}
  dof: 2
  rules:
    y: ["x*f"]
    x: ["y/f"]
  derived:
    rsum: {dim: resistance, label: R_S, name: {en: "Sum", de: "Summe"}, expr: "sum(rs)"}
    rpar: {dim: resistance, label: R_P, name: {en: "Par", de: "Par"}, expr: "par(rs)"}
  formula: ["y = x * f"]
`, "NAME16", flow16("T"))

func loadOne(t *testing.T, calcYAML string) *Calculator {
	t.Helper()
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": calcYAML}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cat.Calculators) != 1 {
		t.Fatalf("got %d calculators", len(cat.Calculators))
	}
	return cat.Calculators[0]
}

func TestSolvePropagatesAndFlagsProblems(t *testing.T) {
	c := loadOne(t, baseCalc)

	res := c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 100}})
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Errorf("errors %v warnings %v, want none", res.Errors, res.Warnings)
	}
	if !near(res.Values["c"], 0.1, 1e-12) || !near(res.Values["p"], 1, 1e-12) {
		t.Errorf("values = %v, want c=0.1 p=1", res.Values)
	}
	if !reflect.DeepEqual(res.Computed, []string{"c", "p"}) {
		t.Errorf("computed = %v, want [c p]", res.Computed)
	}

	// A check fires when its condition holds: 10 V / 5 ohm = 2 A.
	if res := c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 5}}); !reflect.DeepEqual(res.Warnings, []int{0}) {
		t.Errorf("warnings = %v, want [0]", res.Warnings)
	}

	// Under-determined: nothing can be computed from one value.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10}})
	want := map[string]string{"b": ErrUndetermined, "c": ErrUndetermined}
	if !reflect.DeepEqual(res.Errors, want) || !math.IsNaN(res.Values["b"]) || !math.IsNaN(res.Values["p"]) {
		t.Errorf("under-determined: errors %v values %v", res.Errors, res.Values)
	}

	// A zero resistance is out of range and makes the current infinite; an infinite derived value is NaN, no error.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 0}})
	want = map[string]string{"b": ErrRange, "c": ErrNonFinite}
	if !reflect.DeepEqual(res.Errors, want) || !math.IsNaN(res.Values["p"]) || len(res.Warnings) != 0 {
		t.Errorf("zero resistance: errors %v values %v warnings %v", res.Errors, res.Values, res.Warnings)
	}

	// Inputs that are not variables (a derived quantity, an unknown name) are ignored.
	res = c.Solve(Inputs{Values: map[string]float64{"a": 10, "b": 100, "zz": 5, "p": 7}})
	if _, ok := res.Values["zz"]; ok || !near(res.Values["p"], 1, 1e-12) {
		t.Errorf("values = %v, want p=1 computed and no zz", res.Values)
	}
}

func TestSolveChoicesAndLists(t *testing.T) {
	c := loadOne(t, mixCalc)
	in := Inputs{
		Values: map[string]float64{"x": 3},
		Lists:  map[string][]float64{"rs": {100, 300}},
	}

	res := c.Solve(in) // default option dbl: f = 2
	if !near(res.Values["y"], 6, 1e-12) || !near(res.Values["rsum"], 400, 1e-12) || !near(res.Values["rpar"], 75, 1e-12) {
		t.Errorf("dbl: values = %v, want y=6 rsum=400 rpar=75", res.Values)
	}
	in.Choices = map[string]string{"m": "tri"}
	if res := c.Solve(in); !near(res.Values["y"], 9, 1e-12) {
		t.Errorf("tri: y = %v, want 9", res.Values["y"])
	}
	in.Choices = map[string]string{"m": "nonsense"}
	if res := c.Solve(in); !near(res.Values["y"], 6, 1e-12) {
		t.Errorf("unknown option: y = %v, want the default option (6)", res.Values["y"])
	}

	// A zero element of a positive list is out of range; the parallel value is then 0, not an error.
	in.Lists = map[string][]float64{"rs": {100, 0}}
	res = c.Solve(in)
	if !reflect.DeepEqual(res.Errors, map[string]string{"rs": ErrRange}) || !near(res.Values["rpar"], 0, 1e-12) {
		t.Errorf("zero element: errors %v values %v", res.Errors, res.Values)
	}

	// Without the list the variable is undetermined and the derived values stay NaN.
	res = c.Solve(Inputs{Values: map[string]float64{"x": 3}})
	if !reflect.DeepEqual(res.Errors, map[string]string{"rs": ErrUndetermined}) || !math.IsNaN(res.Values["rsum"]) {
		t.Errorf("no list: errors %v values %v", res.Errors, res.Values)
	}
}

func TestSolvableSets(t *testing.T) {
	if got := loadOne(t, baseCalc).Sets; !reflect.DeepEqual(got, [][]string{{"a", "b"}, {"a", "c"}, {"b", "c"}}) {
		t.Errorf("t_div sets = %v, want all three pairs", got)
	}
	// {x, rs} and {rs, y} work; {x, y} cannot know the list.
	if got := loadOne(t, mixCalc).Sets; !reflect.DeepEqual(got, [][]string{{"x", "rs"}, {"rs", "y"}}) {
		t.Errorf("t_mix sets = %v, want [x rs] and [rs y]", got)
	}
}

func TestLoadRejectsUnsolvableCalculators(t *testing.T) {
	// d is an input without any rule, and c can never be an input: neither default nor input sets work out.
	broken := strings.Replace(baseCalc, "    c: {dim: current, label: C, name: {en: \"C\", de: \"C\"}}\n",
		"    c: {dim: current, label: C, name: {en: \"C\", de: \"C\"}}\n    d: {dim: current, label: D, name: {en: \"D\", de: \"D\"}}\n", 1)
	_, err := Load(fixtureFS(t, map[string]string{"t.yaml": broken}))
	if err == nil {
		t.Fatal("a calculator with an input variable nobody can compute must be rejected")
	}
	for _, want := range []string{"do not determine every variable", "var c is an input but belongs to no solvable input set", "var d is an input but belongs to no solvable input set"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run 'TestSolve|TestSolvableSets|TestLoadRejectsUnsolvable' -count=1
```

Expected: **FAIL** — `Calculator.Solve`, `Inputs` and the error codes do not exist yet:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\solve_test.go:49:11: c.Solve undefined (type *Calculator has no field or method Solve)
internal\ecalc\solve_test.go:49:17: undefined: Inputs
internal\ecalc\solve_test.go:61:14: c.Solve undefined (type *Calculator has no field or method Solve)
internal\ecalc\solve_test.go:61:20: undefined: Inputs
internal\ecalc\solve_test.go:66:10: c.Solve undefined (type *Calculator has no field or method Solve)
internal\ecalc\solve_test.go:66:16: undefined: Inputs
internal\ecalc\solve_test.go:67:33: undefined: ErrUndetermined
```

- [ ] **Step 3: Implement the solver and wire the set checks into validation**

`Solve` is CONTRACT §6: the consts of the selected choice options and the inputs are known; `propagate` applies, as a fixpoint, the first alternative of every unknown target whose names are all known (rule targets in declaration order, then derived quantities); then errors, derived values and checks are computed. `SolvableSets` runs the same `propagate` on names only, for every combination of `dof` input-capable variables. `Load` stores the sets on the calculator and rejects a calculator whose initial inputs are not one of them or that has an input variable in none.

Create `internal/ecalc/solve.go` with exactly this content:

```go
package ecalc

import (
	"math"
	"strings"
)

// Error codes of Result.Errors.
const (
	ErrUndetermined = "undetermined" // not enough values to compute the variable
	ErrNonFinite    = "nonfinite"    // NaN or infinite
	ErrRange        = "range"        // zero or negative for a dimension that must be positive
)

// Inputs are the known values for Solve, all in SI base units.
type Inputs struct {
	Values  map[string]float64   // scalar variables
	Lists   map[string][]float64 // list variables
	Choices map[string]string    // choice id -> option id (missing or unknown = the choice's default)
}

// Result is what Solve found.
type Result struct {
	Values   map[string]float64 // every variable and derived quantity; NaN when unknown
	Computed []string           // names computed from rules, in the order they were computed
	Errors   map[string]string  // variable -> error code
	Warnings []int              // indexes of the checks that fired
}

type target struct {
	name string
	alts []Expr
}

// targets lists what propagation can compute: the rule targets in declaration order, then the derived quantities.
func (c *Calculator) targets() []target {
	ts := make([]target, 0, len(c.Rules)+len(c.Derived))
	for _, r := range c.Rules {
		ts = append(ts, target{r.Target, r.Alts})
	}
	for _, d := range c.Derived {
		ts = append(ts, target{d.ID, d.Alts})
	}
	return ts
}

// propagate applies rules until nothing changes: every unknown target whose first applicable alternative has all
// its names known becomes known, and apply is called for it.
func (c *Calculator) propagate(known map[string]bool, apply func(t target, alt Expr)) {
	ts := c.targets()
	for changed := true; changed; {
		changed = false
		for _, t := range ts {
			if known[t.name] {
				continue
			}
			for _, alt := range t.alts {
				if allKnown(alt.Deps, known) {
					apply(t, alt)
					known[t.name] = true
					changed = true
					break
				}
			}
		}
	}
}

func allKnown(deps []string, known map[string]bool) bool {
	for _, d := range deps {
		if !known[d] {
			return false
		}
	}
	return true
}

func (ch *Choice) option(id string) *ChoiceOption {
	for _, want := range []string{id, ch.Default} {
		for i := range ch.Options {
			if ch.Options[i].ID == want {
				return &ch.Options[i]
			}
		}
	}
	return &ch.Options[0]
}

func valueError(d *Dim, x float64) string {
	switch {
	case math.IsNaN(x) || math.IsInf(x, 0):
		return ErrNonFinite
	case d.Positive && x <= 0:
		return ErrRange
	}
	return ""
}

// Solve computes every variable that the inputs determine: the constants of the selected choice options and the
// inputs are known; rules and derived quantities are applied as a fixpoint; then errors and checks are evaluated.
func (c *Calculator) Solve(in Inputs) Result {
	env := Env{Vals: map[string]float64{}, Lists: map[string][]float64{}}
	known := map[string]bool{}
	for i := range c.Choices {
		ch := &c.Choices[i]
		for k, v := range ch.option(in.Choices[ch.ID]).Consts {
			env.Vals[k] = v
			known[k] = true
		}
	}
	for _, v := range c.Vars {
		if v.List {
			if list, ok := in.Lists[v.ID]; ok {
				env.Lists[v.ID] = list
				known[v.ID] = true
			}
		} else if x, ok := in.Values[v.ID]; ok {
			env.Vals[v.ID] = x
			known[v.ID] = true
		}
	}

	res := Result{Values: map[string]float64{}, Computed: []string{}, Errors: map[string]string{}, Warnings: []int{}}
	c.propagate(known, func(t target, alt Expr) {
		x, err := Eval(alt.Node, env)
		if err != nil {
			x = math.NaN()
		}
		env.Vals[t.name] = x
		res.Computed = append(res.Computed, t.name)
	})

	for i := range c.Vars {
		v := &c.Vars[i]
		switch {
		case !known[v.ID]:
			res.Errors[v.ID] = ErrUndetermined
			if !v.List {
				res.Values[v.ID] = math.NaN()
			}
		case v.List:
			for _, x := range env.Lists[v.ID] {
				if code := valueError(v.dim, x); code != "" {
					res.Errors[v.ID] = code
					break
				}
			}
		default:
			x := env.Vals[v.ID]
			res.Values[v.ID] = x
			if code := valueError(v.dim, x); code != "" {
				res.Errors[v.ID] = code
			}
		}
	}
	for _, d := range c.Derived {
		x := math.NaN()
		if known[d.ID] && !math.IsInf(env.Vals[d.ID], 0) {
			x = env.Vals[d.ID]
		}
		res.Values[d.ID] = x
	}
	for i, ch := range c.Checks {
		if !finiteDeps(ch.When.Deps, known, env) {
			continue
		}
		if x, err := Eval(ch.When.Node, env); err == nil && !math.IsNaN(x) && x != 0 {
			res.Warnings = append(res.Warnings, i)
		}
	}
	return res
}

func finiteDeps(deps []string, known map[string]bool, env Env) bool {
	for _, d := range deps {
		if !known[d] {
			return false
		}
		if list, ok := env.Lists[d]; ok {
			for _, x := range list {
				if math.IsNaN(x) || math.IsInf(x, 0) {
					return false
				}
			}
		} else if x := env.Vals[d]; math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

// SolvableSets returns every combination of dof input variables (in declaration order) from which propagation can
// compute all variables. It looks at the structure of the rules only: any alternative whose names are known
// applies, and the constants of every choice count as known.
func (c *Calculator) SolvableSets() [][]string {
	var inputs []string
	for _, v := range c.Vars {
		if v.Input {
			inputs = append(inputs, v.ID)
		}
	}
	consts := map[string]bool{}
	for _, ch := range c.Choices {
		for _, o := range ch.Options {
			for k := range o.Consts {
				consts[k] = true
			}
		}
	}
	var out [][]string
	if c.DOF < 1 || c.DOF > len(inputs) {
		return out
	}
	idx := make([]int, c.DOF)
	var pick func(pos, start int)
	pick = func(pos, start int) {
		if pos == c.DOF {
			set := make([]string, c.DOF)
			known := map[string]bool{}
			for k := range consts {
				known[k] = true
			}
			for i, ix := range idx {
				set[i] = inputs[ix]
				known[set[i]] = true
			}
			c.propagate(known, func(target, Expr) {})
			for _, v := range c.Vars {
				if !known[v.ID] {
					return
				}
			}
			out = append(out, set)
			return
		}
		for i := start; i < len(inputs); i++ {
			idx[pos] = i
			pick(pos+1, i+1)
		}
	}
	pick(0, 0)
	return out
}

// solvableSets stores the solvable input sets on the calculator and checks that the initial inputs are one of them
// and that every input variable belongs to at least one.
func (l *loader) solvableSets(fail func(string, ...any), c *Calculator) {
	c.Sets = c.SolvableSets()
	var defaults []string
	for _, v := range c.Vars {
		if v.Input && (v.Default != nil || v.DefaultList != nil) {
			defaults = append(defaults, v.ID)
		}
	}
	isDefault := false
	inSet := map[string]bool{}
	for _, set := range c.Sets {
		if strings.Join(set, ",") == strings.Join(defaults, ",") {
			isDefault = true
		}
		for _, id := range set {
			inSet[id] = true
		}
	}
	if !isDefault {
		fail("the default inputs (%s) do not determine every variable", strings.Join(defaults, ", "))
	}
	for _, v := range c.Vars {
		if v.Input && !inSet[v.ID] {
			fail("var %s is an input but belongs to no solvable input set", v.ID)
		}
	}
}
```

In `internal/ecalc/catalog.go`, replace this exact text (it occurs once):

```go
	Schematic  string

	varByID map[string]*Var
}
```

with:

```go
	Schematic  string
	// Sets lists every combination of dof input variables from which all variables can be computed, in declaration
	// order (see SolvableSets). Load fills it in.
	Sets [][]string

	varByID map[string]*Var
}
```

In `internal/ecalc/catalog.go`, replace this exact text (it occurs once):

```go
	if len(l.errs) > before {
		return nil
	}
	return c
}
```

with:

```go
	if len(l.errs) == before {
		l.solvableSets(fail, c)
	}
	if len(l.errs) > before {
		return nil
	}
	return c
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/solve.go internal/ecalc/solve_test.go internal/ecalc/catalog.go
git commit -m "feat(ecalc): rule propagation solver and graph-based solvable input sets" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A6: Ohm's law, dividers and series/parallel (first five calculators) with hand-checked vectors

**Files:**
- Create: `internal/ecalc/catalog/calculators/basics.yaml`
- Test: `internal/ecalc/calculators_test.go`
- Test: `internal/ecalc/basics_sets_test.go`
- Test: `internal/ecalc/testdata/vectors/calculators/ohms_law.json`
- Test: `internal/ecalc/testdata/vectors/calculators/voltage_divider.json`
- Test: `internal/ecalc/testdata/vectors/calculators/loaded_divider.json`
- Test: `internal/ecalc/testdata/vectors/calculators/current_divider.json`
- Test: `internal/ecalc/testdata/vectors/calculators/series_parallel.json`

- [ ] **Step 1: Write the failing tests and the vectors**

`calculators_test.go` is generic and covers every calculator of the catalog, so Task A7 adds calculators without touching it: for every set it checks that *each rule alternative and each derived alternative* reproduces its target (this also exercises alternatives that propagation can never reach, such as `p/i^2` for `r` in Ohm's law, whose earlier alternatives are always applicable first), that every solvable input set solves back to the whole set, and that every option of every choice has at least two sets; then it runs the solve vectors. `TestRoundTripAllCalculators` solves the defaults and then every solvable input set taken out of that solution, for every option of every choice (CONTRACT §8). `basics_sets_test.go` pins the exact solvable sets; `{i, i1, i2}` is absent for the current divider because the three currents cannot fix the resistors.

Every set and solve vector carries a `ref`; the expected numbers were computed by hand or with exact fractions (for example 12 V · 2200 / 6900 = 88/23 V), never taken from the solver.

Create `internal/ecalc/calculators_test.go` with exactly this content:

```go
package ecalc

import (
	"fmt"
	"reflect"
	"testing"
)

const solveTol = 1e-9

// calcVectors is the format of testdata/vectors/calculators/<id>.json.
//
// A set is a hand-checked consistent solution: a value for every variable and derived quantity (a JSON array for a
// list variable), optionally for one option of each choice. Every rule alternative and every derived alternative
// must reproduce its target from the set's other values, and every solvable input set must reproduce the whole set
// through Solve. A set is therefore one vector for each rule direction; every option of every choice needs two.
//
// A solve vector is a single Solve call: inputs (SI), the expected values (a subset), the exact error map and the
// exact list of warning (check) indexes.
type calcVectors struct {
	Calc string `json:"calc"`
	Sets []struct {
		Choices map[string]string `json:"choices"`
		Values  map[string]any    `json:"values"`
		Ref     string            `json:"ref"`
	} `json:"sets"`
	Solve []struct {
		Inputs   map[string]any    `json:"inputs"`
		Choices  map[string]string `json:"choices"`
		Expect   map[string]any    `json:"expect"`
		Errors   map[string]string `json:"errors"`
		Warnings []int             `json:"warnings"`
		Ref      string            `json:"ref"`
	} `json:"solve"`
}

// split separates a vector's values into scalars and lists.
func split(t *testing.T, in map[string]any) (map[string]float64, map[string][]float64) {
	t.Helper()
	vals, lists := map[string]float64{}, map[string][]float64{}
	for k, v := range in {
		if arr, ok := v.([]any); ok {
			for _, x := range arr {
				lists[k] = append(lists[k], vecNum(t, x))
			}
			continue
		}
		vals[k] = vecNum(t, v)
	}
	return vals, lists
}

func loadVectors(t *testing.T, id string) calcVectors {
	t.Helper()
	var v calcVectors
	readJSON(t, "testdata/vectors/calculators/"+id+".json", &v)
	if v.Calc != id {
		t.Fatalf("%s.json declares calc %q", id, v.Calc)
	}
	return v
}

// selectedConsts returns the constants of the selected option of every choice.
func selectedConsts(c *Calculator, choices map[string]string) map[string]float64 {
	out := map[string]float64{}
	for i := range c.Choices {
		ch := &c.Choices[i]
		for name, x := range ch.option(choices[ch.ID]).Consts {
			out[name] = x
		}
	}
	return out
}

func TestCalculatorVectors(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.Calculators {
		t.Run(c.ID, func(t *testing.T) {
			v := loadVectors(t, c.ID)
			if len(v.Solve) == 0 {
				t.Error("no solve vectors")
			}

			// Every option of every choice has at least two sets (two in total when there is no choice).
			if len(c.Choices) == 0 && len(v.Sets) < 2 {
				t.Errorf("%d sets, every rule direction needs at least 2 vectors", len(v.Sets))
			}
			for i := range c.Choices {
				ch := &c.Choices[i]
				for _, o := range ch.Options {
					n := 0
					for _, s := range v.Sets {
						if ch.option(s.Choices[ch.ID]).ID == o.ID {
							n++
						}
					}
					if n < 2 {
						t.Errorf("choice %s option %s has %d sets, every rule direction needs at least 2 vectors", ch.ID, o.ID, n)
					}
				}
			}

			for i, s := range v.Sets {
				if s.Ref == "" {
					t.Errorf("set %d has no ref", i)
				}
				vals, lists := split(t, s.Values)
				for _, vr := range c.Vars {
					_, isScalar := vals[vr.ID]
					_, isList := lists[vr.ID]
					if (vr.List && !isList) || (!vr.List && !isScalar) {
						t.Errorf("set %d lacks the %s variable %s", i, map[bool]string{true: "list", false: "scalar"}[vr.List], vr.ID)
					}
				}
				for _, d := range c.Derived {
					if _, ok := vals[d.ID]; !ok {
						t.Errorf("set %d lacks the derived quantity %s", i, d.ID)
					}
				}

				// (a) every alternative of every rule and derived quantity reproduces its target.
				env := Env{Vals: selectedConsts(c, s.Choices), Lists: lists}
				for k, x := range vals {
					env.Vals[k] = x
				}
				for _, tg := range c.targets() {
					for j, alt := range tg.alts {
						got, err := Eval(alt.Node, env)
						if err != nil || !near(got, vals[tg.name], solveTol) {
							t.Errorf("set %d (%s): %s alternative %d (%s) = %v (%v), want %v", i, s.Ref, tg.name, j, alt.Src, got, err, vals[tg.name])
						}
					}
				}

				// (b) every solvable input set reproduces the whole solution.
				for _, set := range c.Sets {
					in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: s.Choices}
					for _, name := range set {
						if c.Var(name).List {
							in.Lists[name] = lists[name]
						} else {
							in.Values[name] = vals[name]
						}
					}
					res := c.Solve(in)
					if len(res.Errors) != 0 {
						t.Errorf("set %d from %v: errors %v", i, set, res.Errors)
					}
					for name, want := range vals {
						if got := res.Values[name]; !near(got, want, solveTol) {
							t.Errorf("set %d from %v: %s = %v, want %v", i, set, name, got, want)
						}
					}
				}
			}

			for i, sv := range v.Solve {
				if sv.Ref == "" {
					t.Errorf("solve vector %d has no ref", i)
				}
				vals, lists := split(t, sv.Inputs)
				res := c.Solve(Inputs{Values: vals, Lists: lists, Choices: sv.Choices})
				for name, want := range sv.Expect {
					if got := res.Values[name]; !near(got, vecNum(t, want), solveTol) {
						t.Errorf("solve %d (%s): %s = %v, want %v", i, sv.Ref, name, got, want)
					}
				}
				wantErrors := sv.Errors
				if wantErrors == nil {
					wantErrors = map[string]string{}
				}
				if !reflect.DeepEqual(res.Errors, wantErrors) {
					t.Errorf("solve %d (%s): errors %v, want %v", i, sv.Ref, res.Errors, wantErrors)
				}
				wantWarnings := sv.Warnings
				if wantWarnings == nil {
					wantWarnings = []int{}
				}
				if !reflect.DeepEqual(res.Warnings, wantWarnings) {
					t.Errorf("solve %d (%s): warnings %v, want %v", i, sv.Ref, res.Warnings, wantWarnings)
				}
			}
		})
	}
}

// defaultInputs returns the initial inputs of a calculator: the variables that carry a default.
func defaultInputs(c *Calculator, choices map[string]string) Inputs {
	in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: choices}
	for _, v := range c.Vars {
		switch {
		case v.List && v.DefaultList != nil:
			in.Lists[v.ID] = v.DefaultList
		case v.Default != nil:
			in.Values[v.ID] = *v.Default
		}
	}
	return in
}

// TestRoundTripAllCalculators solves the defaults, then solves again from every solvable input set taken out of
// that solution: everything must come back. Every option of every choice is tried (the others at their default).
func TestRoundTripAllCalculators(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Calculators) == 0 {
		t.Fatal("the catalog has no calculators")
	}
	for _, c := range cat.Calculators {
		selections := []map[string]string{{}}
		for i := range c.Choices {
			for _, o := range c.Choices[i].Options {
				selections = append(selections, map[string]string{c.Choices[i].ID: o.ID})
			}
		}
		for _, choices := range selections {
			name := fmt.Sprintf("%s %v", c.ID, choices)
			defaults := defaultInputs(c, choices)
			base := c.Solve(defaults)
			if len(base.Errors) != 0 {
				t.Errorf("%s: the defaults give errors %v", name, base.Errors)
				continue
			}
			for _, set := range c.Sets {
				in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: choices}
				for _, id := range set {
					if c.Var(id).List {
						in.Lists[id] = defaults.Lists[id]
					} else {
						in.Values[id] = base.Values[id]
					}
				}
				res := c.Solve(in)
				if len(res.Errors) != 0 {
					t.Errorf("%s from %v: errors %v", name, set, res.Errors)
				}
				for k, want := range base.Values {
					if got := res.Values[k]; !near(got, want, solveTol) {
						t.Errorf("%s from %v: %s = %v, want %v", name, set, k, got, want)
					}
				}
			}
		}
	}
}
```

Create `internal/ecalc/basics_sets_test.go` with exactly this content:

```go
package ecalc

import (
	"reflect"
	"testing"
)

// TestSolvableSetsOfBasics pins the input sets that determine each calculator. Sets are listed in the order of the
// variable declarations. A missing set is a missing rule direction; an extra one would be a rule that cannot work.
func TestSolvableSetsOfBasics(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id   string
		sets [][]string
	}{
		{"ohms_law", [][]string{{"u", "i"}, {"u", "r"}, {"u", "p"}, {"i", "r"}, {"i", "p"}, {"r", "p"}}},
		{"voltage_divider", [][]string{{"vin", "r1", "r2"}, {"vin", "r1", "vout"}, {"vin", "r2", "vout"}, {"r1", "r2", "vout"}}},
		{"loaded_divider", [][]string{
			{"vin", "r1", "r2", "rl"}, {"vin", "r1", "r2", "vout"}, {"vin", "r1", "rl", "vout"},
			{"vin", "r2", "rl", "vout"}, {"r1", "r2", "rl", "vout"},
		}},
		// The three currents {i, i1, i2} cannot fix the resistors, only their ratio, so that set is absent.
		{"current_divider", [][]string{
			{"i", "r1", "r2"}, {"i", "r1", "i1"}, {"i", "r1", "i2"}, {"i", "r2", "i1"}, {"i", "r2", "i2"},
			{"r1", "r2", "i1"}, {"r1", "r2", "i2"}, {"r1", "i1", "i2"}, {"r2", "i1", "i2"},
		}},
		{"series_parallel", [][]string{{"rs"}}},
	}
	for _, w := range want {
		c := cat.Calculator(w.id)
		if c == nil {
			t.Errorf("calculator %s is missing", w.id)
			continue
		}
		if !reflect.DeepEqual(c.Sets, w.sets) {
			t.Errorf("%s: solvable sets\n got %v\nwant %v", w.id, c.Sets, w.sets)
		}
	}
}
```

Create `internal/ecalc/testdata/vectors/calculators/ohms_law.json` with exactly this content:

```json
{
  "calc": "ohms_law",
  "sets": [
    {"values": {"u": 12, "i": 0.02, "r": 600, "p": 0.24}, "ref": "hand: 12 V / 20 mA = 600 ohm; 12 V * 20 mA = 240 mW"},
    {"values": {"u": 5, "i": 0.1, "r": 50, "p": 0.5}, "ref": "hand: 5 V / 100 mA = 50 ohm; 5 V * 100 mA = 500 mW"},
    {"values": {"u": 230, "i": 2, "r": 115, "p": 460}, "ref": "hand: 230 V / 2 A = 115 ohm; 230 V * 2 A = 460 W"}
  ],
  "solve": [
    {"inputs": {"u": 12, "i": 0.02}, "expect": {"r": 600, "p": 0.24}, "ref": "hand: R = 12 V / 0.02 A = 600 ohm; P = 12 V * 0.02 A = 0.24 W"},
    {"inputs": {"p": 0.5, "r": 50}, "expect": {"u": 5, "i": 0.1}, "ref": "hand: U = sqrt(0.5 W * 50 ohm) = sqrt(25) = 5 V; I = 5 V / 50 ohm = 0.1 A"},
    {"inputs": {"u": 12}, "errors": {"i": "undetermined", "r": "undetermined", "p": "undetermined"}, "ref": "one value is not enough for two degrees of freedom"},
    {"inputs": {"u": 12, "r": 0}, "errors": {"r": "range", "i": "nonfinite", "p": "nonfinite"}, "ref": "hand: a zero resistance is out of range and 12 V / 0 ohm is infinite"},
    {"inputs": {"u": -12, "i": 0.02}, "expect": {"r": -600}, "errors": {"r": "range"}, "ref": "hand: -12 V / 20 mA = -600 ohm, a negative resistance is out of range"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/voltage_divider.json` with exactly this content:

```json
{
  "calc": "voltage_divider",
  "sets": [
    {"values": {"vin": 10, "r1": 1000, "r2": 4000, "vout": 8, "i": 0.002, "p": 0.02}, "ref": "hand: 10 V * 4k / (1k + 4k) = 8 V; I = 10 V / 5k = 2 mA; P = 10 V * 2 mA = 20 mW"},
    {"values": {"vin": 9, "r1": 2000, "r2": 1000, "vout": 3, "i": 0.003, "p": 0.027}, "ref": "hand: 9 V * 1k / 3k = 3 V; I = 9 V / 3k = 3 mA; P = 9 V * 3 mA = 27 mW"},
    {"values": {"vin": 12, "r1": 4700, "r2": 2200, "vout": 3.82608695652174, "i": 0.00173913043478261, "p": 0.0208695652173913}, "ref": "hand: V_out = 12 V * 2200 / 6900 = 88/23 V = 3.8260869565 V; I = 12 V / 6900 ohm = 1/575 A; P = 12/575 W"}
  ],
  "solve": [
    {"inputs": {"vin": 12, "r1": 4700, "r2": 2200}, "expect": {"vout": 3.82608695652174, "i": 0.00173913043478261, "p": 0.0208695652173913}, "ref": "hand: 12 V * 2200 / 6900 = 88/23 V; I = 1/575 A; P = 12/575 W"},
    {"inputs": {"r1": 1000, "r2": 4000, "vout": 8}, "expect": {"vin": 10}, "ref": "hand: V_in = 8 V * (1k + 4k) / 4k = 10 V"},
    {"inputs": {"vin": 5, "r1": 1000, "vout": 6}, "errors": {"r2": "range"}, "warnings": [0], "ref": "hand: V_out above V_in gives R2 = 6 V * 1k / (5 V - 6 V) = -6000 ohm, out of range; the V_out >= V_in check fires"},
    {"inputs": {"vin": 5, "r1": 1000, "vout": 5}, "errors": {"r2": "nonfinite"}, "warnings": [0], "ref": "hand: V_out = V_in needs R2 = 5 V * 1k / 0 V = infinity; the check fires"},
    {"inputs": {"vin": 12, "r1": 4700}, "errors": {"r2": "undetermined", "vout": "undetermined"}, "ref": "two of the three inputs are missing"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/loaded_divider.json` with exactly this content:

```json
{
  "calc": "loaded_divider",
  "sets": [
    {"values": {"vin": 12, "r1": 10000, "r2": 10000, "rl": 10000, "vout": 4, "rp": 5000, "vnl": 6, "i": 0.0008}, "ref": "hand: R_P = 10k || 10k = 5k; V_out = 12 V * 5k / 15k = 4 V; unloaded 12 V * 10k / 20k = 6 V; I = 12 V / 15k = 0.8 mA"},
    {"values": {"vin": 9, "r1": 1000, "r2": 3000, "rl": 6000, "vout": 6, "rp": 2000, "vnl": 6.75, "i": 0.003}, "ref": "hand: R_P = 3k * 6k / 9k = 2k; V_out = 9 V * 2k / 3k = 6 V; unloaded 9 V * 3k / 4k = 6.75 V; I = 9 V / 3k = 3 mA"},
    {"values": {"vin": 5, "r1": 2200, "r2": 3300, "rl": 3300, "vout": 2.142857142857143, "rp": 1650, "vnl": 3, "i": 0.0012987012987013}, "ref": "hand: R_P = 3k3 || 3k3 = 1650 ohm; V_out = 5 V * 1650 / 3850 = 15/7 V; unloaded 5 V * 3300 / 5500 = 3 V; I = 5 V / 3850 ohm = 1/770 A"}
  ],
  "solve": [
    {"inputs": {"vin": 12, "r1": 10000, "r2": 10000, "rl": 10000}, "expect": {"vout": 4, "vnl": 6, "rp": 5000, "i": 0.0008}, "ref": "hand: R_P = 5k; V_out = 12 V * 5k / 15k = 4 V; unloaded 6 V; I = 0.8 mA"},
    {"inputs": {"vin": 9, "vout": 6, "r1": 1000, "r2": 3000}, "expect": {"rl": 6000, "rp": 2000}, "ref": "hand: R_P = 6 V * 1k / (9 V - 6 V) = 2k; R_L = 2k * 3k / (3k - 2k) = 6k"},
    {"inputs": {"vin": 12, "vout": 8, "r1": 10000, "r2": 10000}, "errors": {"rl": "range"}, "warnings": [1], "ref": "hand: R_P = 8 V * 10k / 4 V = 20k is not below R2 = 10k, so R_L = 20k * 10k / (10k - 20k) = -20k is out of range; the R_P >= R2 check (index 1) fires"},
    {"inputs": {"vin": 12, "r1": 10000, "r2": 10000}, "errors": {"rl": "undetermined", "vout": "undetermined"}, "ref": "neither the load nor the output voltage is known"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/current_divider.json` with exactly this content:

```json
{
  "calc": "current_divider",
  "sets": [
    {"values": {"i": 0.1, "r1": 100, "r2": 220, "i1": 0.06875, "i2": 0.03125, "u": 6.875}, "ref": "hand: I_1 = 0.1 A * 220 / 320 = 68.75 mA; I_2 = 0.1 A * 100 / 320 = 31.25 mA; U = 68.75 mA * 100 ohm = 6.875 V"},
    {"values": {"i": 0.3, "r1": 100, "r2": 200, "i1": 0.2, "i2": 0.1, "u": 20}, "ref": "hand: I_1 = 0.3 A * 200 / 300 = 0.2 A; I_2 = 0.3 A * 100 / 300 = 0.1 A; U = 0.2 A * 100 ohm = 20 V = 0.1 A * 200 ohm"},
    {"values": {"i": 0.5, "r1": 300, "r2": 100, "i1": 0.125, "i2": 0.375, "u": 37.5}, "ref": "hand: I_1 = 0.5 A * 100 / 400 = 0.125 A; I_2 = 0.5 A * 300 / 400 = 0.375 A; U = 0.125 A * 300 ohm = 37.5 V = 0.375 A * 100 ohm"}
  ],
  "solve": [
    {"inputs": {"i": 0.1, "r1": 100, "r2": 220}, "expect": {"i1": 0.06875, "i2": 0.03125, "u": 6.875}, "ref": "hand: see the first set"},
    {"inputs": {"i1": 0.2, "i2": 0.1, "r1": 100}, "expect": {"r2": 200, "i": 0.3, "u": 20}, "ref": "hand: R2 = I_1 * R1 / I_2 = 0.2 * 100 / 0.1 = 200 ohm; I = I_1 + I_2 = 0.3 A; U = 20 V"},
    {"inputs": {"i": 0.3, "i1": 0.2, "i2": 0.1}, "errors": {"r1": "undetermined", "r2": "undetermined"}, "ref": "the currents alone only fix the ratio R1 : R2, not the resistors"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/series_parallel.json` with exactly this content:

```json
{
  "calc": "series_parallel",
  "sets": [
    {"values": {"rs": [1000, 1000], "rser": 2000, "rpar": 500}, "ref": "hand: 1k + 1k = 2k; 1k || 1k = 500 ohm"},
    {"values": {"rs": [60, 30, 20], "rser": 110, "rpar": 10}, "ref": "hand: 60 + 30 + 20 = 110 ohm; 1 / (1/60 + 1/30 + 1/20) = 1 / (6/60) = 10 ohm"},
    {"values": {"rs": [2000, 3000], "rser": 5000, "rpar": 1200}, "ref": "hand: 2k + 3k = 5k; 2k * 3k / 5k = 1200 ohm"},
    {"values": {"rs": [100, 100, 100, 100], "rser": 400, "rpar": 25}, "ref": "hand: 4 * 100 = 400 ohm; 100 / 4 = 25 ohm"}
  ],
  "solve": [
    {"inputs": {"rs": [4700, 10000, 2200]}, "expect": {"rser": 16900, "rpar": 1303.2518275775145}, "ref": "hand: 4700 + 10000 + 2200 = 16900; 1/(1/4700 + 1/10000 + 1/2200) = 5170000/3967 = 1303.2518275775 ohm"},
    {"inputs": {"rs": [5000]}, "expect": {"rser": 5000, "rpar": 5000}, "ref": "hand: a single resistor is its own series and parallel value"},
    {"inputs": {"rs": [1000, 0]}, "expect": {"rser": 1000, "rpar": 0}, "errors": {"rs": "range"}, "ref": "hand: a zero element is out of range for a resistance; a short in parallel gives 0 ohm, in series it adds nothing"},
    {"inputs": {}, "errors": {"rs": "undetermined"}, "ref": "the list is the only input"}
  ]
}
```

- [ ] **Step 2: Run the tests and watch them fail**

```bash
go test ./internal/ecalc/ -run 'TestCalculatorVectors|TestRoundTrip|TestSolvableSetsOfBasics' -count=1
```

Expected: **FAIL** — the catalog has no calculators yet (`TestCalculatorVectors` passes vacuously):

```text
--- FAIL: TestSolvableSetsOfBasics (0.00s)
    basics_sets_test.go:35: calculator ohms_law is missing
    basics_sets_test.go:35: calculator voltage_divider is missing
    basics_sets_test.go:35: calculator loaded_divider is missing
    basics_sets_test.go:35: calculator current_divider is missing
    basics_sets_test.go:35: calculator series_parallel is missing
--- FAIL: TestRoundTripAllCalculators (0.00s)
    calculators_test.go:212: the catalog has no calculators
FAIL
FAIL	aurago/internal/ecalc	0.030s
FAIL
```

- [ ] **Step 3: Write the first five calculators**

Each calculator names itself in all 16 locales and documents itself in English and German; variable names, derived names and warnings are English and German. The English and German names are exactly those of the manual chapter (Part B). Every `rules` entry is an explicit expression per direction, in the order the solver tries them. `source` cites a standard or a well-known textbook at book or chapter level only.

Create `internal/ecalc/catalog/calculators/basics.yaml` with exactly this content:

```yaml
# Basics calculators. Every name is written in all 16 desktop locales; docs, variable names and warnings in English
# and German (other languages fall back to English). Rules are written out for every direction, in the order the
# solver tries them: the first alternative whose variables are all known wins.

- id: ohms_law
  kind: calculator
  category: basics
  name: {cs: "Ohmův zákon a výkon", da: "Ohms lov og effekt", de: "Ohmsches Gesetz und Leistung", el: "Νόμος του Ωμ και ισχύς", en: "Ohm's law and power", es: "Ley de Ohm y potencia", fr: "Loi d'Ohm et puissance", hi: "ओम का नियम और शक्ति", it: "Legge di Ohm e potenza", ja: "オームの法則と電力", nl: "Wet van Ohm en vermogen", no: "Ohms lov og effekt", pl: "Prawo Ohma i moc", pt: "Lei de Ohm e potência", sv: "Ohms lag och effekt", zh: "欧姆定律与功率"}
  doc:
    en: "Ohm's law relates voltage, current and resistance (U = R × I); the electrical power is P = U × I. Any two of the four values determine the other two."
    de: "Das Ohmsche Gesetz verknüpft Spannung, Strom und Widerstand (U = R · I); die elektrische Leistung ist P = U · I. Je zwei der vier Größen bestimmen die beiden anderen."
  tags: [ohm, ohms law, voltage, current, resistance, power, watt, ohmsches gesetz, spannung, strom, widerstand, leistung]
  source: "Ohm's law and Joule's law (P = U·I); P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1"
  vars:
    u: {dim: voltage, label: U, name: {en: "Voltage", de: "Spannung"}, default: "12 V"}
    i: {dim: current, label: I, name: {en: "Current", de: "Strom"}, default: "20 mA"}
    r: {dim: resistance, label: R, name: {en: "Resistance", de: "Widerstand"}, eseries: true}
    p: {dim: power, label: P, name: {en: "Power", de: "Leistung"}}
  dof: 2
  rules:
    u: ["i*r", "p/i", "sqrt(p*r)"]
    i: ["u/r", "p/u", "sqrt(p/r)"]
    r: ["u/i", "u^2/p", "p/i^2"]
    p: ["u*i", "i^2*r", "u^2/r"]
  formula: ["u = r * i", "p = u * i"]
  schematic: ohms_law

- id: voltage_divider
  kind: calculator
  category: basics
  name: {cs: "Napěťový dělič", da: "Spændingsdeler", de: "Spannungsteiler", el: "Διαιρέτης τάσης", en: "Voltage divider", es: "Divisor de tensión", fr: "Diviseur de tension", hi: "वोल्टेज विभाजक", it: "Partitore di tensione", ja: "分圧回路", nl: "Spanningsdeler", no: "Spenningsdeler", pl: "Dzielnik napięcia", pt: "Divisor de tensão", sv: "Spänningsdelare", zh: "分压器"}
  doc:
    en: "Two resistors in series divide the input voltage: V_out = V_in × R2 / (R1 + R2). The output is unloaded; for a divider that drives a load use the loaded voltage divider. The current and power are those of the divider chain itself."
    de: "Zwei in Reihe geschaltete Widerstände teilen die Eingangsspannung: V_out = V_in · R2 / (R1 + R2). Der Ausgang ist unbelastet; für einen Teiler mit Last den belasteten Spannungsteiler verwenden. Strom und Leistung sind die der Teilerkette selbst."
  tags: [divider, voltage divider, potentiometer, attenuator, teiler, spannungsteiler, widerstand]
  source: "Voltage divider rule; P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1; C. K. Alexander, M. N. O. Sadiku: Fundamentals of Electric Circuits, ch. 2"
  vars:
    vin: {dim: voltage, label: V_in, name: {en: "Input voltage", de: "Eingangsspannung"}, default: "12 V"}
    r1: {dim: resistance, label: R_1, name: {en: "Upper resistor", de: "Oberer Widerstand"}, default: "4k7", eseries: true}
    r2: {dim: resistance, label: R_2, name: {en: "Lower resistor", de: "Unterer Widerstand"}, default: "2k2", eseries: true}
    vout: {dim: voltage, label: V_out, name: {en: "Output voltage", de: "Ausgangsspannung"}}
  dof: 3
  rules:
    vout: ["vin*r2/(r1+r2)"]
    r2: ["vout*r1/(vin-vout)"]
    r1: ["r2*(vin-vout)/vout"]
    vin: ["vout*(r1+r2)/r2"]
  derived:
    i: {dim: current, label: I, name: {en: "Divider current", de: "Querstrom"}, expr: "vin/(r1+r2)"}
    p: {dim: power, label: P, name: {en: "Divider power", de: "Verlustleistung"}, expr: "vin*i"}
  checks:
    - {when: "vout >= vin", warn: {en: "V_out is not below V_in: a passive divider can only attenuate.", de: "V_out liegt nicht unter V_in: Ein passiver Teiler kann nur abschwächen."}}
  formula: ["vout = vin * r2 / (r1 + r2)"]
  schematic: voltage_divider

- id: loaded_divider
  kind: calculator
  category: basics
  name: {cs: "Zatížený napěťový dělič", da: "Belastet spændingsdeler", de: "Belasteter Spannungsteiler", el: "Φορτισμένος διαιρέτης τάσης", en: "Loaded voltage divider", es: "Divisor de tensión con carga", fr: "Diviseur de tension chargé", hi: "भारित वोल्टेज विभाजक", it: "Partitore di tensione con carico", ja: "負荷付き分圧回路", nl: "Belaste spanningsdeler", no: "Belastet spenningsdeler", pl: "Obciążony dzielnik napięcia", pt: "Divisor de tensão com carga", sv: "Belastad spänningsdelare", zh: "带负载分压器"}
  doc:
    en: "A voltage divider that drives a load R_L. The load is in parallel with R2, so V_out = V_in × R_P / (R1 + R_P) with R_P = R2 ∥ R_L. The unloaded output V_nl shows how much the load pulls the voltage down."
    de: "Ein Spannungsteiler, der eine Last R_L speist. Die Last liegt parallel zu R2, daher gilt V_out = V_in · R_P / (R1 + R_P) mit R_P = R2 ∥ R_L. Die Leerlaufspannung V_nl zeigt, wie stark die Last die Spannung herunterzieht."
  tags: [divider, loaded divider, load, voltage divider, teiler, belasteter spannungsteiler, last]
  source: "Voltage divider with a load (R2 in parallel with R_L); P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1"
  vars:
    vin: {dim: voltage, label: V_in, name: {en: "Input voltage", de: "Eingangsspannung"}, default: "12 V"}
    r1: {dim: resistance, label: R_1, name: {en: "Upper resistor", de: "Oberer Widerstand"}, default: "10k", eseries: true}
    r2: {dim: resistance, label: R_2, name: {en: "Lower resistor", de: "Unterer Widerstand"}, default: "10k", eseries: true}
    rl: {dim: resistance, label: R_L, name: {en: "Load resistance", de: "Lastwiderstand"}, default: "10k", eseries: true}
    vout: {dim: voltage, label: V_out, name: {en: "Output voltage under load", de: "Ausgangsspannung unter Last"}}
  dof: 4
  rules:
    vout: ["vin*rp/(r1+rp)"]
    vin: ["vout*(r1+rp)/rp"]
    r1: ["rp*(vin-vout)/vout"]
    r2: ["rp*rl/(rl-rp)"]
    rl: ["rp*r2/(r2-rp)"]
  derived:
    rp: {dim: resistance, label: R_P, name: {en: "R2 ∥ RL", de: "R2 ∥ RL"}, expr: ["r2*rl/(r2+rl)", "vout*r1/(vin-vout)"]}
    vnl: {dim: voltage, label: V_nl, name: {en: "Unloaded output voltage", de: "Leerlaufspannung"}, expr: "vin*r2/(r1+r2)"}
    i: {dim: current, label: I, name: {en: "Input current", de: "Eingangsstrom"}, expr: "vin/(r1+rp)"}
  checks:
    - {when: "vout >= vin", warn: {en: "V_out is not below V_in: a passive divider can only attenuate.", de: "V_out liegt nicht unter V_in: Ein passiver Teiler kann nur abschwächen."}}
    - {when: "rp >= r2", warn: {en: "R_P must be smaller than R2; these values do not fit together (a load always pulls the lower resistance down).", de: "R_P muss kleiner als R2 sein; diese Werte passen nicht zusammen (eine Last verkleinert den unteren Widerstand immer)."}}
  formula: ["vout = vin * rp / (r1 + rp)", "rp = r2 * rl / (r2 + rl)"]
  schematic: loaded_divider

- id: current_divider
  kind: calculator
  category: basics
  name: {cs: "Proudový dělič", da: "Strømdeler", de: "Stromteiler", el: "Διαιρέτης ρεύματος", en: "Current divider", es: "Divisor de corriente", fr: "Diviseur de courant", hi: "धारा विभाजक", it: "Partitore di corrente", ja: "分流回路", nl: "Stroomdeler", no: "Strømdeler", pl: "Dzielnik prądu", pt: "Divisor de corrente", sv: "Strömdelare", zh: "分流电路"}
  doc:
    en: "Two resistors in parallel share the total current in inverse proportion to their resistance: I_1 = I × R2 / (R1 + R2) and I_2 = I × R1 / (R1 + R2). The voltage across both is U = I_1 × R1. The two branch currents alone cannot fix the resistors, only their ratio."
    de: "Zwei parallele Widerstände teilen den Gesamtstrom umgekehrt proportional zu ihrem Widerstand: I_1 = I · R2 / (R1 + R2) und I_2 = I · R1 / (R1 + R2). Die Spannung an beiden beträgt U = I_1 · R1. Die beiden Zweigströme allein legen die Widerstände nicht fest, nur ihr Verhältnis."
  tags: [current divider, parallel, shunt, branch current, stromteiler, parallelschaltung, strom, zweigstrom]
  source: "Current divider rule (Kirchhoff's current law with Ohm's law); C. K. Alexander, M. N. O. Sadiku: Fundamentals of Electric Circuits, ch. 2"
  vars:
    i: {dim: current, label: I, name: {en: "Total current", de: "Gesamtstrom"}, default: "100 mA"}
    r1: {dim: resistance, label: R_1, name: {en: "Resistor 1", de: "Widerstand 1"}, default: "100", eseries: true}
    r2: {dim: resistance, label: R_2, name: {en: "Resistor 2", de: "Widerstand 2"}, default: "220", eseries: true}
    i1: {dim: current, label: I_1, name: {en: "Current through R1", de: "Strom durch R1"}}
    i2: {dim: current, label: I_2, name: {en: "Current through R2", de: "Strom durch R2"}}
  dof: 3
  rules:
    i1: ["i*r2/(r1+r2)", "i-i2", "i2*r2/r1"]
    i2: ["i*r1/(r1+r2)", "i-i1", "i1*r1/r2"]
    i: ["i1+i2", "i1*(r1+r2)/r2", "i2*(r1+r2)/r1"]
    r1: ["i2*r2/i1"]
    r2: ["i1*r1/i2"]
  derived:
    u: {dim: voltage, label: U, name: {en: "Voltage across both", de: "Spannung an beiden"}, expr: "i1*r1"}
  formula: ["i1 = i * r2 / (r1 + r2)", "i2 = i * r1 / (r1 + r2)"]
  schematic: current_divider

- id: series_parallel
  kind: calculator
  category: basics
  name: {cs: "Sériové a paralelní zapojení rezistorů", da: "Serie- og parallelforbundne modstande", de: "Reihen- und Parallelschaltung", el: "Σύνδεση αντιστάσεων σε σειρά και παράλληλα", en: "Series and parallel resistors", es: "Resistencias en serie y en paralelo", fr: "Résistances en série et en parallèle", hi: "श्रेणी और समानांतर प्रतिरोधक", it: "Resistori in serie e in parallelo", ja: "抵抗の直列・並列接続", nl: "Weerstanden in serie en parallel", no: "Seriekoblede og parallellkoblede motstander", pl: "Rezystory połączone szeregowo i równolegle", pt: "Resistências em série e em paralelo", sv: "Seriekopplade och parallellkopplade resistorer", zh: "电阻串联与并联"}
  doc:
    en: "Resistors in series add up: R_S = R_1 + R_2 + …. Resistors in parallel combine as 1/R_P = 1/R_1 + 1/R_2 + …. Enter the values as a list, for example 4k7, 10k, 2k2."
    de: "In Reihe geschaltete Widerstände addieren sich: R_S = R_1 + R_2 + …. Parallel geschaltete Widerstände ergeben 1/R_P = 1/R_1 + 1/R_2 + …. Die Werte als Liste eingeben, zum Beispiel 4k7, 10k, 2k2."
  tags: [series, parallel, resistors, combination, reihenschaltung, parallelschaltung, widerstände, ersatzwiderstand]
  source: "Series and parallel resistor combination; P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1"
  vars:
    rs: {dim: resistance, label: R, name: {en: "Resistors", de: "Widerstände"}, list: true, default: ["4k7", "10k", "2k2"]}
  dof: 1
  derived:
    rser: {dim: resistance, label: R_S, name: {en: "Series resistance", de: "Reihenwiderstand"}, expr: "sum(rs)"}
    rpar: {dim: resistance, label: R_P, name: {en: "Parallel resistance", de: "Parallelwiderstand"}, expr: "par(rs)"}
  formula: ["rser = sum(rs)", "rpar = par(rs)"]
  schematic: series_parallel
```

- [ ] **Step 4: Run the tests and watch them pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/catalog/calculators/basics.yaml internal/ecalc/calculators_test.go internal/ecalc/basics_sets_test.go internal/ecalc/testdata/vectors/calculators/ohms_law.json internal/ecalc/testdata/vectors/calculators/voltage_divider.json internal/ecalc/testdata/vectors/calculators/loaded_divider.json internal/ecalc/testdata/vectors/calculators/current_divider.json internal/ecalc/testdata/vectors/calculators/series_parallel.json
git commit -m "feat(ecalc): Ohm's law, dividers and series/parallel calculators with hand-checked vectors" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A7: The remaining six basics calculators (decibel, dB levels, bridge, star-delta, waveform, charge)

**Files:**
- Modify: `internal/ecalc/catalog/calculators/basics.yaml`
- Modify: `internal/ecalc/basics_sets_test.go`
- Test: `internal/ecalc/basics_test.go`
- Test: `internal/ecalc/testdata/vectors/calculators/decibel_ratio.json`
- Test: `internal/ecalc/testdata/vectors/calculators/db_levels.json`
- Test: `internal/ecalc/testdata/vectors/calculators/wheatstone.json`
- Test: `internal/ecalc/testdata/vectors/calculators/star_delta.json`
- Test: `internal/ecalc/testdata/vectors/calculators/rms_waveform.json`
- Test: `internal/ecalc/testdata/vectors/calculators/charge_energy.json`

- [ ] **Step 1: Write the failing tests**

`basics_test.go` pins the eleven calculators of CONTRACT §9: order, degrees of freedom and schematic ids (`""` for the four without a schematic). The edit to `basics_sets_test.go` adds the solvable sets of the new calculators; `star_delta` must come out as exactly the delta set and the star set.

Create `internal/ecalc/basics_test.go` with exactly this content:

```go
package ecalc

import "testing"

// TestPhaseOneCalculators pins the eleven phase 1 calculators: order, degrees of freedom and schematic ids. The UI
// draws a schematic for every calculator that names one (the ui/ package cross-checks the ids).
func TestPhaseOneCalculators(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id, schematic string
		dof           int
	}{
		{"ohms_law", "ohms_law", 2},
		{"voltage_divider", "voltage_divider", 3},
		{"loaded_divider", "loaded_divider", 4},
		{"current_divider", "current_divider", 3},
		{"series_parallel", "series_parallel", 1},
		{"decibel_ratio", "", 2},
		{"db_levels", "", 2},
		{"wheatstone", "wheatstone", 5},
		{"star_delta", "star_delta", 3},
		{"rms_waveform", "", 1},
		{"charge_energy", "", 2},
	}
	if len(cat.Calculators) != len(want) {
		t.Fatalf("catalog has %d calculators, want %d", len(cat.Calculators), len(want))
	}
	for i, w := range want {
		c := cat.Calculators[i]
		if c.ID != w.id || c.Category != "basics" || c.Schematic != w.schematic || c.DOF != w.dof {
			t.Errorf("calculator %d = {%s %s schematic %q dof %d}, want {%s basics schematic %q dof %d}",
				i, c.ID, c.Category, c.Schematic, c.DOF, w.id, w.schematic, w.dof)
		}
	}
}
```

In `internal/ecalc/basics_sets_test.go`, replace this exact text (it occurs once):

```go
		{"series_parallel", [][]string{{"rs"}}},
	}
```

with:

```go
		{"series_parallel", [][]string{{"rs"}}},
		{"decibel_ratio", [][]string{{"a1", "a2"}, {"a1", "g"}, {"a2", "g"}}},
		{"db_levels", [][]string{{"v", "r"}, {"v", "p"}, {"r", "p"}}},
		{"wheatstone", [][]string{
			{"vs", "r1", "r2", "r3", "r4"}, {"vs", "r1", "r2", "r3", "vd"}, {"vs", "r1", "r2", "r4", "vd"},
			{"vs", "r1", "r3", "r4", "vd"}, {"vs", "r2", "r3", "r4", "vd"}, {"r1", "r2", "r3", "r4", "vd"},
		}},
		// Exactly the delta set and the star set: a mix such as {rab, rbc, ra} has no explicit rule path.
		{"star_delta", [][]string{{"rab", "rbc", "rca"}, {"ra", "rb", "rc"}}},
		{"rms_waveform", [][]string{{"vrms"}, {"vpk"}, {"vpp"}, {"vavg"}}},
		{"charge_energy", [][]string{{"q", "v"}, {"q", "e"}, {"v", "e"}}},
	}
```

- [ ] **Step 2: Run the tests and watch them fail**

```bash
go test ./internal/ecalc/ -run 'TestPhaseOne|TestSolvableSetsOfBasics' -count=1
```

Expected: **FAIL** — only five calculators exist:

```text
--- FAIL: TestSolvableSetsOfBasics (0.01s)
    basics_sets_test.go:45: calculator decibel_ratio is missing
    basics_sets_test.go:45: calculator db_levels is missing
    basics_sets_test.go:45: calculator wheatstone is missing
    basics_sets_test.go:45: calculator star_delta is missing
    basics_sets_test.go:45: calculator rms_waveform is missing
    basics_sets_test.go:45: calculator charge_energy is missing
--- FAIL: TestPhaseOneCalculators (0.00s)
    basics_test.go:29: catalog has 5 calculators, want 11
FAIL
FAIL	aurago/internal/ecalc	0.022s
FAIL
```

- [ ] **Step 3: Write the vectors and the six calculators**

Worked-out reference values: `decibel_ratio` uses 20·log10 for voltage and 10·log10 for power with log10 2 = 0.30102999566398120; `db_levels` uses the definition 0 dBu = sqrt(0.6) V = 0.7745966692414834 V, which dissipates exactly 1 mW in 600 Ω; the Wheatstone sets are exact fractions (V_D = 12 V · (2/3 − 1/4) = 5 V); `rms_waveform` has two sets for each of sine (2/π, 1/√2), square and triangle (1/√3, 1/2); `charge_energy` uses 1 Ah = 3600 C and 1 Wh = 3600 J.

Create `internal/ecalc/testdata/vectors/calculators/decibel_ratio.json` with exactly this content:

```json
{
  "calc": "decibel_ratio",
  "sets": [
    {"choices": {"qty": "voltage"}, "values": {"a1": 1, "a2": 10, "g": 20, "f": 10}, "ref": "definition: G = 20 * log10(10 / 1) = 20 dB"},
    {"choices": {"qty": "voltage"}, "values": {"a1": 0.5, "a2": 50, "g": 40, "f": 100}, "ref": "definition: G = 20 * log10(50 / 0.5) = 20 * log10(100) = 40 dB"},
    {"choices": {"qty": "voltage"}, "values": {"a1": 2, "a2": 1, "g": -6.020599913279624, "f": 0.5}, "ref": "definition: G = 20 * log10(1 / 2) = -20 * 0.30102999566398120 = -6.0206 dB (log10 2 = 0.3010299956639812)"},
    {"choices": {"qty": "power"}, "values": {"a1": 2, "a2": 200, "g": 20, "f": 100}, "ref": "definition: G = 10 * log10(200 / 2) = 10 * log10(100) = 20 dB"},
    {"choices": {"qty": "power"}, "values": {"a1": 1, "a2": 1000, "g": 30, "f": 1000}, "ref": "definition: G = 10 * log10(1000) = 30 dB"},
    {"choices": {"qty": "power"}, "values": {"a1": 1, "a2": 2, "g": 3.010299956639812, "f": 2}, "ref": "definition: G = 10 * log10(2) = 3.0103 dB (log10 2 = 0.3010299956639812)"}
  ],
  "solve": [
    {"inputs": {"a1": 1, "a2": 10}, "expect": {"g": 20, "f": 10}, "ref": "definition: the default option is voltage / current, G = 20 * log10(10) = 20 dB"},
    {"inputs": {"a1": 1, "a2": 10}, "choices": {"qty": "power"}, "expect": {"g": 10, "f": 10}, "ref": "definition: for power G = 10 * log10(10) = 10 dB"},
    {"inputs": {"a1": 1, "a2": 10}, "choices": {"qty": "bogus"}, "expect": {"g": 20}, "ref": "an unknown option falls back to the default option (voltage / current)"},
    {"inputs": {"a1": 1, "a2": 0.1}, "expect": {"g": -20, "f": 0.1}, "ref": "definition: G = 20 * log10(0.1) = -20 dB"},
    {"inputs": {"a1": 1, "g": 6}, "choices": {"qty": "power"}, "expect": {"a2": 3.9810717055349727, "f": 3.9810717055349727}, "ref": "definition: A_2 = A_1 * 10^(6 / 10) = 10^0.6 = 3.98107170553497"},
    {"inputs": {"a2": 100, "g": 40}, "expect": {"a1": 1, "f": 100}, "ref": "definition: A_1 = 100 / 10^(40 / 20) = 100 / 100 = 1"},
    {"inputs": {"a1": 1, "a2": 0}, "expect": {"f": 0}, "errors": {"a2": "range", "g": "nonfinite"}, "ref": "hand: A_2 = 0 is out of range and 20 * log10(0) is minus infinity"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/db_levels.json` with exactly this content:

```json
{
  "calc": "db_levels",
  "sets": [
    {"values": {"v": 0.7745966692414834, "r": 600, "p": 0.001, "p_dbm": 0.001, "v_dbv": 0.7745966692414834, "v_dbu": 0.7745966692414834}, "ref": "definition: 0 dBu = sqrt(0.6) V = 0.7745966692414834 V dissipates 0.6 V^2 / 600 ohm = 1 mW"},
    {"values": {"v": 10, "r": 50, "p": 2, "p_dbm": 2, "v_dbv": 10, "v_dbu": 10}, "ref": "hand: 10 V across 50 ohm: 100 / 50 = 2 W"},
    {"values": {"v": 1, "r": 600, "p": 0.0016666666666666668, "p_dbm": 0.0016666666666666668, "v_dbv": 1, "v_dbu": 1}, "ref": "hand: 1 V across 600 ohm: 1 / 600 = 1.6667 mW"}
  ],
  "solve": [
    {"inputs": {"v": 0.7746, "r": 600}, "expect": {"p": 0.0010000086}, "ref": "hand: 0.7746^2 / 600 = 0.60000516 / 600 = 1.0000086 mW"},
    {"inputs": {"p": 2, "r": 50}, "expect": {"v": 10}, "ref": "hand: V = sqrt(2 W * 50 ohm) = sqrt(100) = 10 V"},
    {"inputs": {"v": 10, "p": 2}, "expect": {"r": 50}, "ref": "hand: R = 10^2 / 2 = 50 ohm"},
    {"inputs": {"v": 10, "r": 0}, "errors": {"r": "range", "p": "nonfinite"}, "ref": "hand: a zero resistance is out of range and 100 / 0 is infinite"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/wheatstone.json` with exactly this content:

```json
{
  "calc": "wheatstone",
  "sets": [
    {"values": {"vs": 12, "r1": 1000, "r2": 2000, "r3": 3000, "r4": 1000, "vd": 5, "r4bal": 6000}, "ref": "hand: V_D = 12 V * (2000/3000 - 1000/4000) = 12 V * (2/3 - 1/4) = 12 V * 5/12 = 5 V; balance R4 = 2000 * 3000 / 1000 = 6000 ohm"},
    {"values": {"vs": 10, "r1": 1000, "r2": 1000, "r3": 1000, "r4": 3000, "vd": -2.5, "r4bal": 1000}, "ref": "hand: V_D = 10 V * (1/2 - 3/4) = -2.5 V; balance R4 = 1000 * 1000 / 1000 = 1000 ohm"},
    {"values": {"vs": 5, "r1": 1000, "r2": 1000, "r3": 1000, "r4": 1100, "vd": -0.11904761904761904, "r4bal": 1000}, "ref": "hand: V_D = 5 V * (1/2 - 1100/2100) = 5 V * (1/2 - 11/21) = 5 V * (-1/42) = -5/42 V = -0.119047619 V; balance R4 = 1000 ohm"}
  ],
  "solve": [
    {"inputs": {"vs": 5, "r1": 1000, "r2": 1000, "r3": 1000, "r4": 1100}, "expect": {"vd": -0.11904761904761904, "r4bal": 1000}, "ref": "hand: V_D = -5/42 V (see the third set)"},
    {"inputs": {"vs": 5, "r1": 1000, "r2": 1000, "r3": 1000, "r4": 1000}, "expect": {"vd": 0, "r4bal": 1000}, "ref": "hand: R1 * R4 = R2 * R3, so the bridge is balanced and V_D = 0"},
    {"inputs": {"vs": 12, "vd": 5, "r1": 1000, "r2": 2000, "r3": 3000}, "expect": {"r4": 1000, "r4bal": 6000}, "ref": "hand: x = 2/3 - 5/12 = 1/4; R4 = 3000 * (1/4) / (3/4) = 1000 ohm; balance R4 = 6000 ohm"},
    {"inputs": {"vd": 0, "r1": 1000, "r2": 1000, "r3": 1000, "r4": 1000}, "errors": {"vs": "nonfinite"}, "ref": "hand: a balanced bridge (V_D = 0) does not determine V_S: 0 / 0"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/star_delta.json` with exactly this content:

```json
{
  "calc": "star_delta",
  "sets": [
    {"values": {"rab": 30, "rbc": 60, "rca": 90, "ra": 15, "rb": 10, "rc": 30}, "ref": "hand: sum = 180; R_A = 30 * 90 / 180 = 15; R_B = 30 * 60 / 180 = 10; R_C = 60 * 90 / 180 = 30"},
    {"values": {"rab": 100, "rbc": 100, "rca": 100, "ra": 33.333333333333336, "rb": 33.333333333333336, "rc": 33.333333333333336}, "ref": "hand: a balanced delta of 100 ohm is a balanced star of 100 / 3 = 33.333 ohm"},
    {"values": {"rab": 20, "rbc": 30, "rca": 50, "ra": 10, "rb": 6, "rc": 15}, "ref": "hand: sum = 100; R_A = 20 * 50 / 100 = 10; R_B = 20 * 30 / 100 = 6; R_C = 30 * 50 / 100 = 15"}
  ],
  "solve": [
    {"inputs": {"rab": 30, "rbc": 60, "rca": 90}, "expect": {"ra": 15, "rb": 10, "rc": 30}, "ref": "hand: see the first set"},
    {"inputs": {"ra": 15, "rb": 10, "rc": 30}, "expect": {"rab": 30, "rbc": 60, "rca": 90}, "ref": "hand: (15*10 + 10*30 + 30*15) = 900; R_AB = 900 / 30 = 30; R_BC = 900 / 15 = 60; R_CA = 900 / 10 = 90"},
    {"inputs": {"rab": 30, "rbc": 60, "ra": 15}, "errors": {"rca": "undetermined", "rb": "undetermined", "rc": "undetermined"}, "ref": "a mix of delta and star values has no explicit rule"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/rms_waveform.json` with exactly this content:

```json
{
  "calc": "rms_waveform",
  "sets": [
    {"choices": {"wave": "sine"}, "values": {"vrms": 7.0710678118654755, "vpk": 10, "vpp": 20, "vavg": 6.366197723675813}, "ref": "definition: sine with V_pk = 10 V: V_rms = 10 / sqrt(2) = 7.0710678 V, V_pp = 20 V, rectified average 2 * 10 / pi = 6.3661977 V"},
    {"choices": {"wave": "sine"}, "values": {"vrms": 0.7071067811865476, "vpk": 1, "vpp": 2, "vavg": 0.6366197723675814}, "ref": "definition: sine with V_pk = 1 V: V_rms = 1 / sqrt(2) = 0.70710678 V, V_pp = 2 V, rectified average 2 / pi = 0.63661977 V"},
    {"choices": {"wave": "square"}, "values": {"vrms": 5, "vpk": 5, "vpp": 10, "vavg": 5}, "ref": "definition: a symmetric square wave of +-5 V has V_rms = V_pk = rectified average = 5 V"},
    {"choices": {"wave": "square"}, "values": {"vrms": 12, "vpk": 12, "vpp": 24, "vavg": 12}, "ref": "definition: a symmetric square wave of +-12 V has V_rms = V_pk = rectified average = 12 V"},
    {"choices": {"wave": "triangle"}, "values": {"vrms": 1.7320508075688772, "vpk": 3, "vpp": 6, "vavg": 1.5}, "ref": "definition: triangle with V_pk = 3 V: V_rms = 3 / sqrt(3) = sqrt(3) V, V_pp = 6 V, rectified average = V_pk / 2 = 1.5 V"},
    {"choices": {"wave": "triangle"}, "values": {"vrms": 5.773502691896257, "vpk": 10, "vpp": 20, "vavg": 5}, "ref": "definition: triangle with V_pk = 10 V: V_rms = 10 / sqrt(3) = 5.7735027 V, V_pp = 20 V, rectified average = 5 V"}
  ],
  "solve": [
    {"inputs": {"vrms": 230}, "expect": {"vpk": 325.2691193458119, "vpp": 650.5382386916237, "vavg": 207.0727527161344}, "ref": "hand: sine: V_pk = 230 * sqrt(2) = 325.2691 V; V_pp = 650.5382 V; rectified average = 230 * 2 * sqrt(2) / pi = 207.0728 V"},
    {"inputs": {"vpp": 2}, "choices": {"wave": "square"}, "expect": {"vrms": 1, "vpk": 1, "vavg": 1}, "ref": "definition: square wave, V_pp = 2 V means +-1 V and V_rms = V_pk = average = 1 V"},
    {"inputs": {"vavg": 1.5}, "choices": {"wave": "triangle"}, "expect": {"vpk": 3, "vrms": 1.7320508075688772, "vpp": 6}, "ref": "definition: triangle, rectified average = V_pk / 2 so V_pk = 3 V; V_rms = sqrt(3) V; V_pp = 6 V"}
  ]
}
```

Create `internal/ecalc/testdata/vectors/calculators/charge_energy.json` with exactly this content:

```json
{
  "calc": "charge_energy",
  "sets": [
    {"values": {"q": 7200, "v": 3.7, "e": 26640}, "ref": "hand: 2 Ah = 7200 C; 7200 C * 3.7 V = 26640 J = 7.4 Wh"},
    {"values": {"q": 3.6, "v": 5, "e": 18}, "ref": "hand: 1 mAh = 3.6 C; 3.6 C * 5 V = 18 J = 5 mWh"},
    {"values": {"q": 18000, "v": 12, "e": 216000}, "ref": "hand: 5 Ah = 18000 C; 18000 C * 12 V = 216000 J = 60 Wh"}
  ],
  "solve": [
    {"inputs": {"q": 7200, "v": 3.7}, "expect": {"e": 26640}, "ref": "hand: 7200 C * 3.7 V = 26640 J"},
    {"inputs": {"e": 26640, "v": 3.7}, "expect": {"q": 7200}, "ref": "hand: 26640 J / 3.7 V = 7200 C = 2 Ah"},
    {"inputs": {"e": 216000, "q": 18000}, "expect": {"v": 12}, "ref": "hand: 216000 J / 18000 C = 12 V"},
    {"inputs": {"e": 100, "q": 0}, "errors": {"v": "nonfinite"}, "ref": "hand: 100 J / 0 C is infinite"}
  ]
}
```

In `internal/ecalc/catalog/calculators/basics.yaml`, replace this exact text (it occurs once):

```yaml
  schematic: series_parallel
```

with:

```yaml
  schematic: series_parallel

- id: decibel_ratio
  kind: calculator
  category: basics
  name: {cs: "Decibel (poměr)", da: "Decibel (forhold)", de: "Dezibel-Verhältnis", el: "Ντεσιμπέλ (λόγος)", en: "Decibel ratio", es: "Decibelios (relación)", fr: "Décibels (rapport)", hi: "डेसिबल (अनुपात)", it: "Decibel (rapporto)", ja: "デシベル（比）", nl: "Decibel (verhouding)", no: "Desibel (forhold)", pl: "Decybele (stosunek)", pt: "Decibéis (razão)", sv: "Decibel (förhållande)", zh: "分贝（比值）"}
  doc:
    en: "The decibel expresses a ratio on a logarithmic scale: G = 20 × log10(A_2 / A_1) for voltages and currents (field quantities) and G = 10 × log10(A_2 / A_1) for power. Choose the quantity first: +20 dB is a factor of 10 for voltages but a factor of 100 for power."
    de: "Das Dezibel gibt ein Verhältnis logarithmisch an: G = 20 · log10(A_2 / A_1) für Spannungen und Ströme (Feldgrößen) und G = 10 · log10(A_2 / A_1) für Leistungen. Zuerst die Größe wählen: +20 dB bedeuten bei Spannungen den Faktor 10, bei Leistungen den Faktor 100."
  tags: [decibel, db, gain, ratio, attenuation, logarithm, dezibel, verstärkung, dämpfung, verhältnis, pegel]
  source: "Definition of the decibel (field quantities 20·log10, power quantities 10·log10), cf. IEC 60027-3 (logarithmic quantities and their units)"
  vars:
    a1: {dim: number, label: A_1, name: {en: "Reference value", de: "Bezugswert"}, default: 1}
    a2: {dim: number, label: A_2, name: {en: "Compared value", de: "Vergleichswert"}, default: 10}
    g: {dim: gain, label: G, name: {en: "Level difference", de: "Pegeldifferenz"}}
  choices:
    qty:
      name: {cs: "Veličina", da: "Størrelse", de: "Größe", el: "Μέγεθος", en: "Quantity", es: "Magnitud", fr: "Grandeur", hi: "राशि", it: "Grandezza", ja: "量", nl: "Grootheid", no: "Størrelse", pl: "Wielkość", pt: "Grandeza", sv: "Storhet", zh: "量类型"}
      default: voltage
      options:
        voltage:
          name: {cs: "Napětí / proud", da: "Spænding / strøm", de: "Spannung / Strom", el: "Τάση / ρεύμα", en: "Voltage / current", es: "Tensión / corriente", fr: "Tension / courant", hi: "वोल्टेज / धारा", it: "Tensione / corrente", ja: "電圧／電流", nl: "Spanning / stroom", no: "Spenning / strøm", pl: "Napięcie / prąd", pt: "Tensão / corrente", sv: "Spänning / ström", zh: "电压/电流"}
          consts: {k: 20}
        power:
          name: {cs: "Výkon", da: "Effekt", de: "Leistung", el: "Ισχύς", en: "Power", es: "Potencia", fr: "Puissance", hi: "शक्ति", it: "Potenza", ja: "電力", nl: "Vermogen", no: "Effekt", pl: "Moc", pt: "Potência", sv: "Effekt", zh: "功率"}
          consts: {k: 10}
  dof: 2
  rules:
    g: ["k*log10(a2/a1)"]
    a2: ["a1*10^(g/k)"]
    a1: ["a2/10^(g/k)"]
  derived:
    f: {dim: number, label: F, name: {en: "Ratio A_2 / A_1", de: "Verhältnis A_2 / A_1"}, expr: "a2/a1"}
  formula: ["g = k * log10(a2 / a1)"]

- id: db_levels
  kind: calculator
  category: basics
  name: {cs: "Úrovně v dB (dBm, dBV, dBu)", da: "dB-niveauer (dBm, dBV, dBu)", de: "Pegel in dB", el: "Στάθμες dB (dBm, dBV, dBu)", en: "dB levels", es: "Niveles en dB (dBm, dBV, dBu)", fr: "Niveaux en dB (dBm, dBV, dBu)", hi: "dB स्तर (dBm, dBV, dBu)", it: "Livelli in dB (dBm, dBV, dBu)", ja: "dBレベル（dBm、dBV、dBu）", nl: "dB-niveaus (dBm, dBV, dBu)", no: "dB-nivåer (dBm, dBV, dBu)", pl: "Poziomy w dB (dBm, dBV, dBu)", pt: "Níveis em dB (dBm, dBV, dBu)", sv: "dB-nivåer (dBm, dBV, dBu)", zh: "dB电平（dBm、dBV、dBu）"}
  doc:
    en: "Absolute levels in decibels: dBm is relative to 1 mW, dBV to 1 V and dBu to 0.7746 V, the voltage that dissipates 1 mW in 600 Ω. From a voltage and a load resistance the power follows; the derived values are shown as dBm, dBV and dBu."
    de: "Absolute Pegel in Dezibel: dBm bezieht sich auf 1 mW, dBV auf 1 V und dBu auf 0,7746 V, die Spannung, die an 600 Ω eine Leistung von 1 mW umsetzt. Aus Spannung und Lastwiderstand folgt die Leistung; die abgeleiteten Werte werden als dBm, dBV und dBu angezeigt."
  tags: [dbm, dbv, dbu, level, audio, milliwatt, 600 ohm, pegel]
  source: "Definitions: dBm = 10·log10(P / 1 mW), dBV = 20·log10(U / 1 V), dBu = 20·log10(U / 0.7746 V) with 0.7746 V = sqrt(0.6) V (1 mW in 600 Ω); decibel per IEC 60027-3"
  vars:
    v: {dim: voltage, label: V, name: {en: "Voltage", de: "Spannung"}, default: "0.7746 V"}
    r: {dim: resistance, label: R, name: {en: "Load resistance", de: "Lastwiderstand"}, default: "600"}
    p: {dim: power, label: P, name: {en: "Power", de: "Leistung"}}
  dof: 2
  rules:
    p: ["v^2/r"]
    v: ["sqrt(p*r)"]
    r: ["v^2/p"]
  derived:
    p_dbm: {dim: power, label: P_dBm, name: {en: "Power level", de: "Leistungspegel"}, expr: "p", unit: dBm}
    v_dbv: {dim: voltage, label: V_dBV, name: {en: "Voltage level re 1 V", de: "Spannungspegel bezogen auf 1 V"}, expr: "v", unit: dBV}
    v_dbu: {dim: voltage, label: V_dBu, name: {en: "Voltage level re 0.7746 V", de: "Spannungspegel bezogen auf 0,7746 V"}, expr: "v", unit: dBu}
  formula: ["p = v^2 / r"]

- id: wheatstone
  kind: calculator
  category: basics
  name: {cs: "Wheatstoneův můstek", da: "Wheatstone-bro", de: "Wheatstone-Brücke", el: "Γέφυρα Wheatstone", en: "Wheatstone bridge", es: "Puente de Wheatstone", fr: "Pont de Wheatstone", hi: "व्हीटस्टोन सेतु", it: "Ponte di Wheatstone", ja: "ホイートストンブリッジ", nl: "Brug van Wheatstone", no: "Wheatstone-bro", pl: "Mostek Wheatstone'a", pt: "Ponte de Wheatstone", sv: "Wheatstones brygga", zh: "惠斯通电桥"}
  doc:
    en: "A Wheatstone bridge is two voltage dividers side by side: R1 over R2 on the left and R3 over R4 on the right. The bridge voltage V_D between the two midpoints is V_D = V_S × (R2 / (R1 + R2) − R4 / (R3 + R4)). It is zero when R1 × R4 = R2 × R3; R_4b shows the R4 that balances the bridge."
    de: "Eine Wheatstone-Brücke besteht aus zwei nebeneinander liegenden Spannungsteilern: links R1 über R2, rechts R3 über R4. Die Brückenspannung V_D zwischen den beiden Mittelpunkten ist V_D = V_S · (R2 / (R1 + R2) − R4 / (R3 + R4)). Sie ist null bei R1 · R4 = R2 · R3; R_4b zeigt den R4, der die Brücke abgleicht."
  tags: [bridge, wheatstone, balance, sensor, strain gauge, brücke, abgleich, messbrücke, dehnungsmessstreifen]
  source: "Wheatstone bridge as two voltage dividers with the balance condition R1·R4 = R2·R3; e.g. C. K. Alexander, M. N. O. Sadiku: Fundamentals of Electric Circuits (resistance measurement with the Wheatstone bridge)"
  vars:
    vs: {dim: voltage, label: V_S, name: {en: "Supply voltage", de: "Speisespannung"}, default: "5 V"}
    r1: {dim: resistance, label: R_1, name: {en: "Upper left resistor", de: "Linker oberer Widerstand"}, default: "1k", eseries: true}
    r2: {dim: resistance, label: R_2, name: {en: "Lower left resistor", de: "Linker unterer Widerstand"}, default: "1k", eseries: true}
    r3: {dim: resistance, label: R_3, name: {en: "Upper right resistor", de: "Rechter oberer Widerstand"}, default: "1k", eseries: true}
    r4: {dim: resistance, label: R_4, name: {en: "Lower right resistor", de: "Rechter unterer Widerstand"}, default: "1k1", eseries: true}
    vd: {dim: voltage, label: V_D, name: {en: "Bridge voltage", de: "Brückenspannung"}}
  dof: 5
  rules:
    vd: ["vs*(r2/(r1+r2)-r4/(r3+r4))"]
    vs: ["vd/(r2/(r1+r2)-r4/(r3+r4))"]
    r1: ["r2*(1-(r4/(r3+r4)+vd/vs))/(r4/(r3+r4)+vd/vs)"]
    r2: ["r1*(r4/(r3+r4)+vd/vs)/(1-(r4/(r3+r4)+vd/vs))"]
    r3: ["r4*(1-(r2/(r1+r2)-vd/vs))/(r2/(r1+r2)-vd/vs)"]
    r4: ["r3*(r2/(r1+r2)-vd/vs)/(1-(r2/(r1+r2)-vd/vs))"]
  derived:
    r4bal: {dim: resistance, label: R_4b, name: {en: "R4 for balance", de: "R4 für Abgleich"}, expr: "r2*r3/r1"}
  formula: ["vd = vs * (r2 / (r1 + r2) - r4 / (r3 + r4))", "r4bal = r2 * r3 / r1"]
  schematic: wheatstone

- id: star_delta
  kind: calculator
  category: basics
  name: {cs: "Převod hvězda–trojúhelník", da: "Stjerne-trekant-omregning", de: "Stern-Dreieck-Umwandlung", el: "Μετασχηματισμός αστέρα–τριγώνου", en: "Star-delta transform", es: "Transformación estrella-triángulo", fr: "Transformation étoile-triangle", hi: "स्टार-डेल्टा रूपांतरण", it: "Trasformazione stella-triangolo", ja: "スター・デルタ変換", nl: "Ster-driehoektransformatie", no: "Stjerne-trekant-omforming", pl: "Przekształcenie gwiazda–trójkąt", pt: "Transformação estrela-triângulo", sv: "Stjärn-triangelomvandling", zh: "星形-三角形变换"}
  doc:
    en: "Converts between a star (Y) network of three resistors R_A, R_B, R_C and the equivalent delta (Δ) network R_AB, R_BC, R_CA. Delta to star: R_A = R_AB × R_CA / (R_AB + R_BC + R_CA), and cyclically. Star to delta: R_AB = (R_A R_B + R_B R_C + R_C R_A) / R_C, and cyclically. Enter either the three delta or the three star resistors."
    de: "Rechnet zwischen einem Sternnetz (Y) aus drei Widerständen R_A, R_B, R_C und dem gleichwertigen Dreiecksnetz (Δ) R_AB, R_BC, R_CA um. Dreieck nach Stern: R_A = R_AB · R_CA / (R_AB + R_BC + R_CA), und zyklisch. Stern nach Dreieck: R_AB = (R_A R_B + R_B R_C + R_C R_A) / R_C, und zyklisch. Entweder die drei Dreiecks- oder die drei Sternwiderstände eingeben."
  tags: [star delta, wye delta, y-delta, delta, star, wye, transformation, stern dreieck, dreieck, stern, umwandlung]
  source: "Wye-delta (star-delta) transformation; C. K. Alexander, M. N. O. Sadiku: Fundamentals of Electric Circuits, ch. 2"
  vars:
    rab: {dim: resistance, label: R_AB, name: {en: "Delta resistor A-B", de: "Dreieckswiderstand A-B"}, default: "30"}
    rbc: {dim: resistance, label: R_BC, name: {en: "Delta resistor B-C", de: "Dreieckswiderstand B-C"}, default: "60"}
    rca: {dim: resistance, label: R_CA, name: {en: "Delta resistor C-A", de: "Dreieckswiderstand C-A"}, default: "90"}
    ra: {dim: resistance, label: R_A, name: {en: "Star resistor at A", de: "Sternwiderstand an A"}}
    rb: {dim: resistance, label: R_B, name: {en: "Star resistor at B", de: "Sternwiderstand an B"}}
    rc: {dim: resistance, label: R_C, name: {en: "Star resistor at C", de: "Sternwiderstand an C"}}
  dof: 3
  rules:
    ra: ["rab*rca/(rab+rbc+rca)"]
    rb: ["rab*rbc/(rab+rbc+rca)"]
    rc: ["rbc*rca/(rab+rbc+rca)"]
    rab: ["(ra*rb+rb*rc+rc*ra)/rc"]
    rbc: ["(ra*rb+rb*rc+rc*ra)/ra"]
    rca: ["(ra*rb+rb*rc+rc*ra)/rb"]
  formula: ["ra = rab * rca / (rab + rbc + rca)", "rab = (ra * rb + rb * rc + rc * ra) / rc"]
  schematic: star_delta

- id: rms_waveform
  kind: calculator
  category: basics
  name: {cs: "Efektivní, špičková a střední hodnota", da: "Effektiv-, spids- og middelværdi", de: "Effektiv-, Spitzen- und Mittelwert", el: "Ενεργός, μέγιστη και μέση τιμή", en: "RMS, peak and average", es: "Valor eficaz, de pico y medio", fr: "Valeur efficace, crête et moyenne", hi: "आरएमएस, शिखर और औसत मान", it: "Valore efficace, di picco e medio", ja: "実効値・ピーク値・平均値", nl: "Effectieve waarde, piek en gemiddelde", no: "Effektiv-, topp- og middelverdi", pl: "Wartość skuteczna, szczytowa i średnia", pt: "Valor eficaz, de pico e médio", sv: "Effektivvärde, toppvärde och medelvärde", zh: "有效值、峰值与平均值"}
  doc:
    en: "Converts between the RMS value, the peak value, the peak-to-peak value and the average of the rectified waveform for a sine, square or triangle wave. The crest factor is V_pk / V_rms and the form factor is V_rms / V_avg; for a sine wave they are √2 and π / (2√2) ≈ 1.1107. The average is the mean of the absolute value (the rectified average), not the mean of the signal, which is zero for a symmetric wave."
    de: "Rechnet zwischen Effektivwert, Spitzenwert, Spitze-Spitze-Wert und dem Gleichrichtwert (Mittelwert des Betrags) für Sinus-, Rechteck- und Dreieckschwingungen um. Der Scheitelfaktor ist V_pk / V_rms, der Formfaktor V_rms / V_avg; beim Sinus sind es √2 und π / (2√2) ≈ 1,1107. Der Mittelwert ist der Gleichrichtwert (Mittelwert des Betrags), nicht der Mittelwert des Signals, der bei symmetrischen Kurven null ist."
  tags: [rms, peak, average, crest factor, form factor, sine, square, triangle, effektivwert, spitzenwert, mittelwert, scheitelfaktor, formfaktor, gleichrichtwert]
  source: "Standard waveform relations: sine rms = peak/√2 and rectified average = 2·peak/π; square rms = rectified average = peak; triangle rms = peak/√3 and rectified average = peak/2 (integration of the waveform); P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1 (signals)"
  vars:
    vrms: {dim: voltage, label: V_rms, name: {en: "RMS value", de: "Effektivwert"}, default: "230 V"}
    vpk: {dim: voltage, label: V_pk, name: {en: "Peak value", de: "Spitzenwert"}}
    vpp: {dim: voltage, label: V_pp, name: {en: "Peak-to-peak value", de: "Spitze-Spitze-Wert"}}
    vavg: {dim: voltage, label: V_avg, name: {en: "Average of the rectified wave", de: "Gleichrichtwert"}}
  choices:
    wave:
      name: {cs: "Průběh", da: "Kurveform", de: "Kurvenform", el: "Κυματομορφή", en: "Waveform", es: "Forma de onda", fr: "Forme d'onde", hi: "तरंग रूप", it: "Forma d'onda", ja: "波形", nl: "Golfvorm", no: "Signalform", pl: "Kształt przebiegu", pt: "Forma de onda", sv: "Kurvform", zh: "波形"}
      default: sine
      options:
        sine:
          name: {cs: "Sinus", da: "Sinus", de: "Sinus", el: "Ημίτονο", en: "Sine", es: "Senoidal", fr: "Sinusoïdale", hi: "ज्या तरंग", it: "Sinusoidale", ja: "正弦波", nl: "Sinus", no: "Sinus", pl: "Sinusoida", pt: "Senoidal", sv: "Sinus", zh: "正弦波"}
          consts: {cf: 1.4142135623730951, ff: 1.1107207345395915}
        square:
          name: {cs: "Obdélník", da: "Firkant", de: "Rechteck", el: "Τετραγωνική", en: "Square", es: "Cuadrada", fr: "Carrée", hi: "वर्ग तरंग", it: "Quadra", ja: "矩形波", nl: "Blokgolf", no: "Firkant", pl: "Prostokąt", pt: "Quadrada", sv: "Fyrkant", zh: "方波"}
          consts: {cf: 1, ff: 1}
        triangle:
          name: {cs: "Trojúhelník", da: "Trekant", de: "Dreieck", el: "Τριγωνική", en: "Triangle", es: "Triangular", fr: "Triangulaire", hi: "त्रिकोणीय तरंग", it: "Triangolare", ja: "三角波", nl: "Driehoek", no: "Trekant", pl: "Trójkąt", pt: "Triangular", sv: "Triangel", zh: "三角波"}
          consts: {cf: 1.7320508075688772, ff: 1.1547005383792515}
  dof: 1
  rules:
    vrms: ["vpk/cf", "vpp/(2*cf)", "vavg*ff"]
    vpk: ["vrms*cf", "vpp/2", "vavg*ff*cf"]
    vpp: ["2*vpk", "2*vrms*cf", "2*vavg*ff*cf"]
    vavg: ["vrms/ff", "vpk/(cf*ff)", "vpp/(2*cf*ff)"]
  formula: ["vpk = vrms * cf", "vrms = vavg * ff"]

- id: charge_energy
  kind: calculator
  category: basics
  name: {cs: "Náboj a energie", da: "Ladning og energi", de: "Ladung und Energie", el: "Φορτίο και ενέργεια", en: "Charge and energy", es: "Carga y energía", fr: "Charge et énergie", hi: "आवेश और ऊर्जा", it: "Carica ed energia", ja: "電荷とエネルギー", nl: "Lading en energie", no: "Ladning og energi", pl: "Ładunek i energia", pt: "Carga e energia", sv: "Laddning och energi", zh: "电荷与能量"}
  doc:
    en: "Charge Q in ampere-hours (1 Ah = 3600 C) and energy E in watt-hours (1 Wh = 3600 J) are related by the voltage: E = Q × U. A 2 Ah cell at 3.7 V holds 7.4 Wh. The nominal voltage is only an average; real batteries deliver less energy at high load."
    de: "Ladung Q in Amperestunden (1 Ah = 3600 C) und Energie E in Wattstunden (1 Wh = 3600 J) hängen über die Spannung zusammen: E = Q · U. Eine Zelle mit 2 Ah bei 3,7 V speichert 7,4 Wh. Die Nennspannung ist nur ein Mittelwert; reale Akkus liefern bei hoher Last weniger Energie."
  tags: [charge, energy, battery, capacity, mah, ah, wh, joule, coulomb, ladung, energie, akku, kapazität]
  source: "Definitions of charge and energy: E = Q·U (1 C·V = 1 J), 1 Ah = 3600 C, 1 Wh = 3600 J; The International System of Units (SI Brochure, BIPM)"
  vars:
    q: {dim: charge, label: Q, name: {en: "Charge", de: "Ladung"}, default: "2 Ah", unit: Ah}
    v: {dim: voltage, label: U, name: {en: "Voltage", de: "Spannung"}, default: "3.7 V"}
    e: {dim: energy, label: E, name: {en: "Energy", de: "Energie"}, unit: Wh}
  dof: 2
  rules:
    e: ["q*v"]
    q: ["e/v"]
    v: ["e/q"]
  formula: ["e = q * v"]
```

- [ ] **Step 4: Run the tests and watch them pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Commit**

```bash
git add internal/ecalc/catalog/calculators/basics.yaml internal/ecalc/basics_sets_test.go internal/ecalc/basics_test.go internal/ecalc/testdata/vectors/calculators/decibel_ratio.json internal/ecalc/testdata/vectors/calculators/db_levels.json internal/ecalc/testdata/vectors/calculators/wheatstone.json internal/ecalc/testdata/vectors/calculators/star_delta.json internal/ecalc/testdata/vectors/calculators/rms_waveform.json internal/ecalc/testdata/vectors/calculators/charge_energy.json
git commit -m "feat(ecalc): decibel, dB levels, bridge, star-delta, waveform and charge calculators" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A8: Localized catalog JSON, ETag cache and snapshot

**Files:**
- Create: `internal/ecalc/localize.go`
- Modify: `internal/ecalc/catalog.go`
- Test: `internal/ecalc/localize_test.go`
- Create (generated): `internal/ecalc/testdata/catalog.en.json`

- [ ] **Step 1: Write the failing test**

`-update` is a test flag defined in this file: `go test ./internal/ecalc/ -run TestCatalogSnapshot -update` rewrites `testdata/catalog.en.json` (the English catalog, pretty-printed with two-space indentation and a final newline); without it the test fails when the file is stale or missing. The other tests check English and German strings, the English fallback for French docs and variable names, unknown/empty/region-tagged languages (the body *and* the ETag equal the English ones, `lang` is `en`), the ETag format and stability, the cache bound after 200 junk languages, that no collection is ever `null`, and the defaults and sets of two calculators.

Create `internal/ecalc/localize_test.go` with exactly this content:

```go
package ecalc

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/catalog.en.json from the embedded catalog")

// catalogDoc is the part of the catalog JSON the tests look at.
type catalogDoc struct {
	Version    int    `json:"version"`
	Lang       string `json:"lang"`
	Categories []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Doc  string `json:"doc"`
		Vars []struct {
			ID          string    `json:"id"`
			Name        string    `json:"name"`
			Default     *float64  `json:"default"`
			DefaultList []float64 `json:"default_list"`
		} `json:"vars"`
		Checks []struct {
			Warn string `json:"warn"`
		} `json:"checks"`
		Sets [][]string `json:"sets"`
	} `json:"items"`
}

func fetch(t *testing.T, lang string) (catalogDoc, []byte, string) {
	t.Helper()
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	body, etag, err := cat.LocalizedJSON(lang)
	if err != nil {
		t.Fatal(err)
	}
	var doc catalogDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("LocalizedJSON(%q) is not valid JSON: %v", lang, err)
	}
	return doc, body, etag
}

func TestCatalogSnapshot(t *testing.T) {
	_, body, _ := fetch(t, "en")
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	const path = "testdata/catalog.en.json"
	if *update {
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (create it with: go test ./internal/ecalc/ -run TestCatalogSnapshot -update)", err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Errorf("%s is stale; regenerate it with: go test ./internal/ecalc/ -run TestCatalogSnapshot -update", path)
	}
}

func TestLocalizedJSONLanguagesAndFallback(t *testing.T) {
	en, _, _ := fetch(t, "en")
	if en.Version != 1 || en.Lang != "en" || len(en.Items) != 11 || len(en.Categories) != 13 {
		t.Fatalf("en: version %d lang %q items %d categories %d", en.Version, en.Lang, len(en.Items), len(en.Categories))
	}
	find := func(d catalogDoc, id string) int {
		for i, it := range d.Items {
			if it.ID == id {
				return i
			}
		}
		t.Fatalf("no item %s", id)
		return -1
	}
	vd := find(en, "voltage_divider")
	if en.Items[vd].Name != "Voltage divider" || en.Categories[0].Name != "Basics" || en.Items[vd].Vars[0].Name != "Input voltage" {
		t.Errorf("en strings: %q %q %q", en.Items[vd].Name, en.Categories[0].Name, en.Items[vd].Vars[0].Name)
	}

	de, _, _ := fetch(t, "de")
	if de.Lang != "de" || de.Items[vd].Name != "Spannungsteiler" || de.Categories[0].Name != "Grundlagen" ||
		de.Items[vd].Vars[0].Name != "Eingangsspannung" || !strings.HasPrefix(de.Items[vd].Doc, "Zwei in Reihe") ||
		!strings.HasPrefix(de.Items[vd].Checks[0].Warn, "V_out liegt nicht unter") {
		t.Errorf("de strings: %+v", de.Items[vd])
	}

	// French has a name but no doc or variable names: those fall back to English.
	fr, _, _ := fetch(t, "fr")
	if fr.Items[vd].Name != "Diviseur de tension" || fr.Items[vd].Doc != en.Items[vd].Doc || fr.Items[vd].Vars[0].Name != "Input voltage" {
		t.Errorf("fr fallback: %+v", fr.Items[vd])
	}

	// Unknown or empty languages are English; region tags and aliases are normalized.
	_, enBody, enTag := fetch(t, "en")
	for _, lang := range []string{"", "xx", "tlh", "en-US"} {
		if doc, body, tag := fetch(t, lang); !bytes.Equal(body, enBody) || tag != enTag || doc.Lang != "en" {
			t.Errorf("LocalizedJSON(%q) is not the English catalog (lang %q, etag %s, want %s)", lang, doc.Lang, tag, enTag)
		}
	}
	_, deBody, _ := fetch(t, "de")
	_, noBody, _ := fetch(t, "no")
	for lang, want := range map[string][]byte{"de-DE": deBody, "DE_de": deBody, " de ": deBody, "nb": noBody} {
		if _, body, _ := fetch(t, lang); !bytes.Equal(body, want) {
			t.Errorf("LocalizedJSON(%q) was not normalized", lang)
		}
	}
}

func TestLocalizedJSONETagAndCache(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	body, etag, _ := cat.LocalizedJSON("en")
	again, etag2, _ := cat.LocalizedJSON("en")
	_, deTag, _ := cat.LocalizedJSON("de")
	if etag != etag2 || etag == deTag || &body[0] != &again[0] {
		t.Errorf("the ETag must be stable per language, differ between languages and the body must be cached (%s %s %s)", etag, etag2, deTag)
	}
	if !regexp.MustCompile(`^"ecalc-[0-9a-f]{16}"$`).MatchString(etag) {
		t.Errorf("etag = %s, want \"ecalc-\" and 16 hex digits in quotes", etag)
	}

	// The cache is keyed by the normalized language, so arbitrary ?lang= values cannot make it grow.
	for i := 0; i < 200; i++ {
		if _, _, err := cat.LocalizedJSON(fmt.Sprintf("zz%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	cat.locMu.Lock()
	cached := len(cat.loc)
	cat.locMu.Unlock()
	if cached > len(Locales) {
		t.Errorf("the cache holds %d languages after 200 junk requests, want at most %d", cached, len(Locales))
	}
}

func TestLocalizedJSONNeverHasNullCollections(t *testing.T) {
	_, body, _ := fetch(t, "en")
	var doc struct {
		Units struct {
			Prefixes []any            `json:"prefixes"`
			Dims     []map[string]any `json:"dims"`
		} `json:"units"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Units.Prefixes == nil || len(doc.Units.Dims) == 0 {
		t.Fatal("units are missing")
	}
	for _, d := range doc.Units.Dims {
		for _, key := range []string{"aliases", "prefixes", "units"} {
			if _, ok := d[key].([]any); !ok {
				t.Errorf("dim %v: %s = %#v, want an array", d["id"], key, d[key])
			}
		}
	}
	for _, it := range doc.Items {
		for _, key := range []string{"tags", "vars", "choices", "rules", "derived", "checks", "formula", "sets"} {
			if _, ok := it[key].([]any); !ok {
				t.Errorf("item %v: %s = %#v, want an array", it["id"], key, it[key])
			}
		}
		for _, v := range it["vars"].([]any) {
			vm := v.(map[string]any)
			for _, key := range []string{"id", "dim", "label", "name", "default", "default_list", "unit", "eseries", "list", "input"} {
				if _, ok := vm[key]; !ok {
					t.Errorf("item %v var %v lacks the key %s", it["id"], vm["id"], key)
				}
			}
		}
	}
}

func TestLocalizedJSONDefaultsAndSets(t *testing.T) {
	en, _, _ := fetch(t, "en")
	byID := map[string]int{}
	for i, it := range en.Items {
		byID[it.ID] = i
	}
	vd := en.Items[byID["voltage_divider"]]
	if vd.Vars[0].Default == nil || *vd.Vars[0].Default != 12 || vd.Vars[3].Default != nil || vd.Vars[0].DefaultList != nil || len(vd.Sets) != 4 {
		t.Errorf("voltage_divider vars/sets: %+v", vd)
	}
	sp := en.Items[byID["series_parallel"]]
	if sp.Vars[0].Default != nil || len(sp.Vars[0].DefaultList) != 3 || sp.Vars[0].DefaultList[0] != 4700 {
		t.Errorf("series_parallel list default: %+v", sp.Vars[0])
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

```bash
go test ./internal/ecalc/ -run 'TestCatalogSnapshot|TestLocalizedJSON' -count=1
```

Expected: **FAIL** — `LocalizedJSON` does not exist yet:

```text
# aurago/internal/ecalc [aurago/internal/ecalc.test]
internal\ecalc\localize_test.go:47:25: cat.LocalizedJSON undefined (type *Catalog has no field or method LocalizedJSON)
internal\ecalc\localize_test.go:135:23: cat.LocalizedJSON undefined (type *Catalog has no field or method LocalizedJSON)
internal\ecalc\localize_test.go:136:25: cat.LocalizedJSON undefined (type *Catalog has no field or method LocalizedJSON)
internal\ecalc\localize_test.go:137:21: cat.LocalizedJSON undefined (type *Catalog has no field or method LocalizedJSON)
internal\ecalc\localize_test.go:147:23: cat.LocalizedJSON undefined (type *Catalog has no field or method LocalizedJSON)
```

- [ ] **Step 3: Implement the localized JSON**

The JSON structs are the shapes of CONTRACT §7. `Text.in` resolves a language with English as the fallback, `render` builds the document with `SetEscapeHTML(false)`, and `LocalizedJSON` caches body and ETag per normalized language. The `Catalog` struct gets the mutex and the cache map.

Create `internal/ecalc/localize.go` with exactly this content:

```go
package ecalc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// The JSON shapes below are the catalog API (GET /api/ecalc/catalog?lang=xx). Every ordered collection is an
// array, and optional fields are written with their zero value so that clients never test for existence; only a
// variable's default and default_list are null when there is none.
type catalogJSON struct {
	Version    int            `json:"version"`
	Lang       string         `json:"lang"`
	Units      unitsJSON      `json:"units"`
	Categories []categoryJSON `json:"categories"`
	Items      []itemJSON     `json:"items"`
}

type unitsJSON struct {
	Prefixes []Prefix `json:"prefixes"`
	Dims     []Dim    `json:"dims"`
}

type categoryJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type varJSON struct {
	ID          string    `json:"id"`
	Dim         string    `json:"dim"`
	Label       string    `json:"label"`
	Name        string    `json:"name"`
	Default     *float64  `json:"default"`
	DefaultList []float64 `json:"default_list"`
	Unit        string    `json:"unit"`
	ESeries     bool      `json:"eseries"`
	List        bool      `json:"list"`
	Input       bool      `json:"input"`
}

type optionJSON struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	Consts map[string]float64 `json:"consts"`
}

type choiceJSON struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Default string       `json:"default"`
	Options []optionJSON `json:"options"`
}

type ruleJSON struct {
	Target string   `json:"target"`
	Alts   []string `json:"alts"`
}

type derivedJSON struct {
	ID    string   `json:"id"`
	Dim   string   `json:"dim"`
	Label string   `json:"label"`
	Name  string   `json:"name"`
	Alts  []string `json:"alts"`
	Unit  string   `json:"unit"`
}

type checkJSON struct {
	When string `json:"when"`
	Warn string `json:"warn"`
}

// itemJSON is a calculator. Phase 3 adds the other item kinds next to it.
type itemJSON struct {
	Kind      string        `json:"kind"`
	ID        string        `json:"id"`
	Category  string        `json:"category"`
	Name      string        `json:"name"`
	Doc       string        `json:"doc"`
	Tags      []string      `json:"tags"`
	Source    string        `json:"source"`
	Vars      []varJSON     `json:"vars"`
	Choices   []choiceJSON  `json:"choices"`
	DOF       int           `json:"dof"`
	Rules     []ruleJSON    `json:"rules"`
	Derived   []derivedJSON `json:"derived"`
	Checks    []checkJSON   `json:"checks"`
	Formula   []string      `json:"formula"`
	Schematic string        `json:"schematic"`
	Sets      [][]string    `json:"sets"`
}

type localized struct {
	body []byte
	etag string
}

// in returns the text for lang, or the English text when there is none.
func (t Text) in(lang string) string {
	if s := t[lang]; s != "" {
		return s
	}
	return t["en"]
}

// normalizeLang maps a language tag onto one of the 16 desktop locales: case and region are dropped (de-DE, de_DE
// and DE are German), the Norwegian variants nb and nn are no, anything else is English.
func normalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(l, "-_"); i > 0 {
		l = l[:i]
	}
	switch {
	case l == "nb" || l == "nn":
		return "no"
	case isLocale(l):
		return l
	}
	return "en"
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func sources(alts []Expr) []string {
	out := make([]string, len(alts))
	for i, a := range alts {
		out[i] = a.Src
	}
	return out
}

// LocalizedJSON returns the catalog as JSON with every text resolved for lang (English where a language has no
// text), together with its ETag: "ecalc-" and the first 16 hex digits of the SHA-256 of the body, in quotes. The
// result is cached per normalized language; callers must not modify the returned body.
func (c *Catalog) LocalizedJSON(lang string) (body []byte, etag string, err error) {
	lang = normalizeLang(lang)
	c.locMu.Lock()
	defer c.locMu.Unlock()
	if e, ok := c.loc[lang]; ok {
		return e.body, e.etag, nil
	}
	if body, err = c.render(lang); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	e := localized{body: body, etag: `"ecalc-` + hex.EncodeToString(sum[:8]) + `"`}
	if c.loc == nil {
		c.loc = map[string]localized{}
	}
	c.loc[lang] = e
	return e.body, e.etag, nil
}

func (c *Catalog) render(lang string) ([]byte, error) {
	out := catalogJSON{
		Version:    1,
		Lang:       lang,
		Units:      unitsJSON{Prefixes: orEmpty(c.Units.Prefixes), Dims: orEmpty(c.Units.Dims)},
		Categories: make([]categoryJSON, 0, len(c.Categories)),
		Items:      make([]itemJSON, 0, len(c.Calculators)),
	}
	for _, cat := range c.Categories {
		out.Categories = append(out.Categories, categoryJSON{ID: cat.ID, Name: cat.Name.in(lang)})
	}
	for _, calc := range c.Calculators {
		it := itemJSON{
			Kind: calc.Kind, ID: calc.ID, Category: calc.Category, Name: calc.Name.in(lang), Doc: calc.Doc.in(lang),
			Tags: orEmpty(calc.Tags), Source: calc.Source, DOF: calc.DOF, Formula: orEmpty(calc.Formula),
			Schematic: calc.Schematic, Sets: orEmpty(calc.Sets),
			Vars: []varJSON{}, Choices: []choiceJSON{}, Rules: []ruleJSON{}, Derived: []derivedJSON{}, Checks: []checkJSON{},
		}
		for _, v := range calc.Vars {
			it.Vars = append(it.Vars, varJSON{
				ID: v.ID, Dim: v.Dim, Label: v.Label, Name: v.Name.in(lang), Default: v.Default,
				DefaultList: v.DefaultList, Unit: v.Unit, ESeries: v.ESeries, List: v.List, Input: v.Input,
			})
		}
		for _, ch := range calc.Choices {
			cj := choiceJSON{ID: ch.ID, Name: ch.Name.in(lang), Default: ch.Default, Options: []optionJSON{}}
			for _, o := range ch.Options {
				consts := o.Consts
				if consts == nil {
					consts = map[string]float64{}
				}
				cj.Options = append(cj.Options, optionJSON{ID: o.ID, Name: o.Name.in(lang), Consts: consts})
			}
			it.Choices = append(it.Choices, cj)
		}
		for _, r := range calc.Rules {
			it.Rules = append(it.Rules, ruleJSON{Target: r.Target, Alts: sources(r.Alts)})
		}
		for _, d := range calc.Derived {
			it.Derived = append(it.Derived, derivedJSON{
				ID: d.ID, Dim: d.Dim, Label: d.Label, Name: d.Name.in(lang), Alts: sources(d.Alts), Unit: d.Unit,
			})
		}
		for _, ch := range calc.Checks {
			it.Checks = append(it.Checks, checkJSON{When: ch.When.Src, Warn: ch.Warn.in(lang)})
		}
		out.Items = append(out.Items, it)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // formulas contain < and >
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
```

In `internal/ecalc/catalog.go`, replace this exact text (it occurs once):

```go
	byID map[string]*Calculator
}
```

with:

```go
	byID map[string]*Calculator

	locMu sync.Mutex
	loc   map[string]localized // LocalizedJSON cache, by normalized language
}
```

- [ ] **Step 4: Generate the snapshot**

```bash
go test ./internal/ecalc/ -run TestCatalogSnapshot -update -count=1
```

Expected: **PASS**, and `internal/ecalc/testdata/catalog.en.json` now exists (1876 lines, 41 822 bytes with LF line endings at the time of writing,beginning with `{`, `  "version": 1,` and `  "lang": "en",`). Look through it once: units first, then the 13 categories, then the 11 items with their `sets`.

- [ ] **Step 5: Run the whole package and watch it pass**

```bash
go test ./internal/ecalc/ -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 6: Commit**

```bash
git add internal/ecalc/localize.go internal/ecalc/localize_test.go internal/ecalc/catalog.go internal/ecalc/testdata/catalog.en.json
git commit -m "feat(ecalc): localized catalog JSON with English fallback, ETag cache and snapshot" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A9: Publish the E-series tables in the shared vectors

**Files:**
- Modify: `internal/ecalc/eseries_test.go`
- Modify: `internal/ecalc/testdata/vectors/core.json`

- [ ] **Step 1: Extend the test**

The JavaScript mirror needs the same seven tables. Publishing them in `core.json` lets the JavaScript parity test compare its copy, and the Go test below guarantees that the published tables are the Go tables.

In `internal/ecalc/eseries_test.go`, replace this exact text (it occurs once):

```go
import (
	"math"
	"sort"
	"strconv"
	"testing"
)
```

with:

```go
import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"testing"
)
```

In `internal/ecalc/eseries_test.go`, replace this exact text (it occurs once):

```go
		NearestError []struct {
			Series string `json:"series"`
			V      any    `json:"v"`
		} `json:"eseries_error"`
	}
```

with:

```go
		NearestError []struct {
			Series string `json:"series"`
			V      any    `json:"v"`
		} `json:"eseries_error"`
		Tables map[string][]float64 `json:"eseries_tables"`
	}
```

In `internal/ecalc/eseries_test.go`, replace this exact text (it occurs once):

```go
			t.Errorf("Nearest(%s, %v) = %v, want an error", v.Series, v.V, got)
		}
	}
}
```

with:

```go
			t.Errorf("Nearest(%s, %v) = %v, want an error", v.Series, v.V, got)
		}
	}
	if len(f.Tables) != len(ESeriesIDs) {
		t.Errorf("core.json carries %d eseries_tables, want %d", len(f.Tables), len(ESeriesIDs))
	}
	for id, want := range f.Tables {
		if got, _ := ESeriesValues(id); !reflect.DeepEqual(got, want) {
			t.Errorf("eseries_tables[%s] differs from the Go table", id)
		}
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
go test ./internal/ecalc/ -run TestESeriesVectors -count=1
```

Expected: **FAIL** — `core.json` has no `eseries_tables` section yet:

```text
--- FAIL: TestESeriesVectors (0.00s)
    eseries_test.go:55: core.json carries 0 eseries_tables, want 7
FAIL
FAIL	aurago/internal/ecalc	0.080s
FAIL
```

- [ ] **Step 3: Add the tables to the vectors**

In `internal/ecalc/testdata/vectors/core.json`, replace this exact text (it occurs once):

```json
    {"v": 1, "dim": "bogus", "want": ""}
  ]
}
```

with:

```json
    {"v": 1, "dim": "bogus", "want": ""}
  ],
  "_doc_eseries_tables": "eseries_tables: the mantissas of one decade (1 <= m < 10) of every series, as listed by IEC 60063. The JavaScript mirror must carry exactly these tables (E192 has 9.20 at index 185).",
  "eseries_tables": {
    "E3": [1.0, 2.2, 4.7],
    "E6": [1.0, 1.5, 2.2, 3.3, 4.7, 6.8],
    "E12": [1.0, 1.2, 1.5, 1.8, 2.2, 2.7, 3.3, 3.9, 4.7, 5.6, 6.8, 8.2],
    "E24": [
      1.0, 1.1, 1.2, 1.3, 1.5, 1.6, 1.8, 2.0, 2.2, 2.4, 2.7, 3.0,
      3.3, 3.6, 3.9, 4.3, 4.7, 5.1, 5.6, 6.2, 6.8, 7.5, 8.2, 9.1
    ],
    "E48": [
      1.00, 1.05, 1.10, 1.15, 1.21, 1.27, 1.33, 1.40, 1.47, 1.54, 1.62, 1.69,
      1.78, 1.87, 1.96, 2.05, 2.15, 2.26, 2.37, 2.49, 2.61, 2.74, 2.87, 3.01,
      3.16, 3.32, 3.48, 3.65, 3.83, 4.02, 4.22, 4.42, 4.64, 4.87, 5.11, 5.36,
      5.62, 5.90, 6.19, 6.49, 6.81, 7.15, 7.50, 7.87, 8.25, 8.66, 9.09, 9.53
    ],
    "E96": [
      1.00, 1.02, 1.05, 1.07, 1.10, 1.13, 1.15, 1.18, 1.21, 1.24, 1.27, 1.30,
      1.33, 1.37, 1.40, 1.43, 1.47, 1.50, 1.54, 1.58, 1.62, 1.65, 1.69, 1.74,
      1.78, 1.82, 1.87, 1.91, 1.96, 2.00, 2.05, 2.10, 2.15, 2.21, 2.26, 2.32,
      2.37, 2.43, 2.49, 2.55, 2.61, 2.67, 2.74, 2.80, 2.87, 2.94, 3.01, 3.09,
      3.16, 3.24, 3.32, 3.40, 3.48, 3.57, 3.65, 3.74, 3.83, 3.92, 4.02, 4.12,
      4.22, 4.32, 4.42, 4.53, 4.64, 4.75, 4.87, 4.99, 5.11, 5.23, 5.36, 5.49,
      5.62, 5.76, 5.90, 6.04, 6.19, 6.34, 6.49, 6.65, 6.81, 6.98, 7.15, 7.32,
      7.50, 7.68, 7.87, 8.06, 8.25, 8.45, 8.66, 8.87, 9.09, 9.31, 9.53, 9.76
    ],
    "E192": [
      1.00, 1.01, 1.02, 1.04, 1.05, 1.06, 1.07, 1.09, 1.10, 1.11, 1.13, 1.14,
      1.15, 1.17, 1.18, 1.20, 1.21, 1.23, 1.24, 1.26, 1.27, 1.29, 1.30, 1.32,
      1.33, 1.35, 1.37, 1.38, 1.40, 1.42, 1.43, 1.45, 1.47, 1.49, 1.50, 1.52,
      1.54, 1.56, 1.58, 1.60, 1.62, 1.64, 1.65, 1.67, 1.69, 1.72, 1.74, 1.76,
      1.78, 1.80, 1.82, 1.84, 1.87, 1.89, 1.91, 1.93, 1.96, 1.98, 2.00, 2.03,
      2.05, 2.08, 2.10, 2.13, 2.15, 2.18, 2.21, 2.23, 2.26, 2.29, 2.32, 2.34,
      2.37, 2.40, 2.43, 2.46, 2.49, 2.52, 2.55, 2.58, 2.61, 2.64, 2.67, 2.71,
      2.74, 2.77, 2.80, 2.84, 2.87, 2.91, 2.94, 2.98, 3.01, 3.05, 3.09, 3.12,
      3.16, 3.20, 3.24, 3.28, 3.32, 3.36, 3.40, 3.44, 3.48, 3.52, 3.57, 3.61,
      3.65, 3.70, 3.74, 3.79, 3.83, 3.88, 3.92, 3.97, 4.02, 4.07, 4.12, 4.17,
      4.22, 4.27, 4.32, 4.37, 4.42, 4.48, 4.53, 4.59, 4.64, 4.70, 4.75, 4.81,
      4.87, 4.93, 4.99, 5.05, 5.11, 5.17, 5.23, 5.30, 5.36, 5.42, 5.49, 5.56,
      5.62, 5.69, 5.76, 5.83, 5.90, 5.97, 6.04, 6.12, 6.19, 6.26, 6.34, 6.42,
      6.49, 6.57, 6.65, 6.73, 6.81, 6.90, 6.98, 7.06, 7.15, 7.23, 7.32, 7.41,
      7.50, 7.59, 7.68, 7.77, 7.87, 7.96, 8.06, 8.16, 8.25, 8.35, 8.45, 8.56,
      8.66, 8.76, 8.87, 8.98, 9.09, 9.20, 9.31, 9.42, 9.53, 9.65, 9.76, 9.88
    ]
  }
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
go test ./internal/ecalc/ -run TestESeriesVectors -count=1
```

Expected: **PASS** (`ok  <TAB>aurago/internal/ecalc`, the time varies).

- [ ] **Step 5: Verify the Go package**

```bash
gofmt -l internal/ecalc
go vet ./internal/ecalc/
go test ./internal/ecalc/ -count=1 -v
```

Expected: `gofmt -l` prints nothing, `go vet` prints nothing, and the test run ends with `PASS` and `ok  <TAB>aurago/internal/ecalc` (24 top-level tests; `TestCalculatorVectors` has one subtest per calculator).

- [ ] **Step 6: Commit**

```bash
git add internal/ecalc/eseries_test.go internal/ecalc/testdata/vectors/core.json
git commit -m "test(ecalc): publish the E-series tables in the shared vectors for the JavaScript mirror" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A10: DOX guide for `internal/ecalc` and its route in the root Child DOX Index

**Files:**
- Create: `internal/ecalc/AGENTS.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Create the package guide**

The repository's DOX rules (root `AGENTS.md`, section "DOX: read, edit, close out") require a child guide at a durable boundary, in the order Purpose, Ownership, Local Contracts, Work Guidance, Verification, Child DOX Index, and an index update when a guide is created. This guide follows the `internal/findit/AGENTS.md` shape: file map, the catalog/vector/snapshot workflow, the Go/JS parity rule (it points at `ui/js/desktop/apps/ecalc/` and `scripts/test-ecalc-core.mjs` from Part C) and the content-honesty rules.

`AGENTS.md` files are git-ignored by pattern (`.gitignore` line 142 ignores every `AGENTS.md` except the root one), so this file only shows up in `git status` after it is staged with `git add -f`.

Create `internal/ecalc/AGENTS.md` with exactly this content:

```markdown
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
```

- [ ] **Step 2: Route the guide in the root Child DOX Index**

The root `AGENTS.md` table "Child DOX Index / cross-component routing" lists every feature guide, and the existing audit tests read it (`TestLocalWikipediaContractIsRouted`, `TestPersonDetectionContractsAreRouted`); no test requires an ECalc row, but the DOX rule does. The row goes after the Docker Manager rows (the table is ordered by owner path). It names the server guide for the route and `ui/js/desktop/apps/AGENTS.md` for the app, the guides Parts B and C extend.

Make the edit in the execution worktree, which has the committed root `AGENTS.md`. If `git status` does not list `AGENTS.md` as modified afterwards, run `git ls-files -v AGENTS.md`: a leading `h` means the file is flagged assume-unchanged, and `git update-index --no-assume-unchanged AGENTS.md` clears it.

In `AGENTS.md`, replace this exact text (it occurs once):

```markdown
| Managed RustFS S3 | `internal/rustfs/AGENTS.md`
```

with:

```markdown
| ECalc | `internal/ecalc/AGENTS.md`, `internal/server/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Electronics formula catalog, expression language, units, E-series and solver with shared Go/JS parity vectors; `GET /api/ecalc/catalog`; Desktop app and manual. |
| Managed RustFS S3 | `internal/rustfs/AGENTS.md`
```

- [ ] **Step 3: Check the diff and the routing tests**

```bash
git diff --check
go test ./internal/audit/ -run 'TestLocalWikipediaContractIsRouted|TestPersonDetectionContractsAreRouted' -count=1
```

Expected: **PASS** (`git diff --check` prints nothing; the audit tests print `ok  <TAB>aurago/internal/audit`).

- [ ] **Step 4: Commit**

```bash
git add -f internal/ecalc/AGENTS.md AGENTS.md
git commit -m "docs(ecalc): DOX guide for internal/ecalc and its Child DOX Index route" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


---

## Part B — Server route, registration, i18n, manual

Contract deviations:

1. **Icon (task B4).** Contract section 1 lists "BuiltinApps entry + icon" without fixing the icon, and the spec allows "a name in `desktopPreferredIconNames` or artwork through `metadata.logo_path`". Part B uses the second option: `Icon: "ecalc"` plus `Metadata["logo_path"] = "/img/desktop-icons/ecalc.svg"`, with a new hand-written SVG `ui/img/desktop-icons/ecalc.svg` (a colour-banded resistor on a log scale, teal tile, in the style of the four existing app logos there). Reason: no existing Papirus icon fits except `calculator`, which the Calculator app already uses (two identical icons in the start menu and dock), and a new themed icon would have to be added to seven icon sets (four of them are importer outputs with provenance records). This is the Synth Studio pattern. Consequence for Part C: do **not** add an `appIconKeys` entry for `ecalc` and do not touch the icon theme manifests or `desktopPreferredIconNames`; a themed `ecalc` icon would replace the logo.
2. **Two extra test files, owned by B**: `ui/desktop_ecalc_i18n_test.go` (`TestDesktopECalcLocaleKeysExistInAllLanguages`, package var `ecalcLocaleKeys`) and `ui/desktop_ecalc_registration_test.go` (`TestDesktopECalcBackendRegistration`). Part C must not reuse those identifiers in `ui/desktop_ecalc_test.go` and friends.
3. **Server and desktop AGENTS.md** each get one short entry (`internal/server/AGENTS.md`, `internal/desktop/AGENTS.md`), following the repo's DOX convention. Not listed in contract section 1.

Order inside this part: B1 and B2 depend on Part A (`internal/ecalc`). B3, B4 and B5 do not touch Go code from Part A. B3 (locale keys) is before B4 (app registration) so that `TestDesktopBuiltinAppNamesExistInAllLanguages` is never red between commits.

### Task B1: Catalog route `GET /api/ecalc/catalog`

**Runs after Part A.** It needs the package `internal/ecalc` with exactly the contract API `Default() (*Catalog, error)` and `(*Catalog).LocalizedJSON(lang string) (body []byte, etag string, err error)`. The handler and its test rely only on that API and on the section 7 JSON shape (`version`, `lang`, `units`, `categories`, a non-empty `items` array; an unknown or empty `lang` comes back as `"en"`; the same body and ETag for `en`, `xx` and an empty `lang`).

**Environment for every Go command in this part:** run from the repository root with `GOTMPDIR` set to a short backslash path (contract section 0), for example in PowerShell `$env:GOTMPDIR='C:\tmp\go-build-ecalc'`.

**Pattern followed:** the Pixel routes (`internal/server/pixel_handlers.go`, registered in `server_routes.go`): the generic `/api/` session auth in `authMiddleware`, then `requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead)` in the handler, then a method check. Tests build a `&Server{Cfg, Logger}` and call the handler, as `pixel_handlers_test.go` does; ours goes through a `http.ServeMux` so the registered path is tested too.

**GitNexus impact (contract section 0):** the only existing function touched is `(*Server).run` in `internal/server/server_routes.go` (one added call). `impact "run" --file internal/server/server_routes.go --direction upstream`: 1 direct caller (`Start`), risk LOW. `registerECalcRoutes` is a new symbol.

**Files:**
- Create: `internal/server/ecalc_handlers.go`
- Create: `internal/server/ecalc_handlers_test.go`
- Modify: `internal/server/server_routes.go` — func `(*Server).run`, the "Pixel image editor endpoints" block (lines 664-676 at the base commit)
- Modify: `internal/server/AGENTS.md` — new section "ECalc catalog API" before "### Code Studio" (line 335 at the base commit)

- [ ] **Step 1: Write the failing test**

Create `internal/server/ecalc_handlers_test.go`:

```go
package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func newECalcTestServer(authEnabled bool) *Server {
	cfg := &config.Config{}
	if authEnabled {
		cfg.Auth.Enabled = true
		cfg.Auth.SessionSecret = "ecalc-test-secret"
	}
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func serveECalc(s *Server, method, target string, header map[string]string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerECalcRoutes(mux, s)
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeECalcCatalog(t *testing.T, rr *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("catalog is not a JSON object: %v\n%s", err, rr.Body.String())
	}
	return doc
}

func ecalcLang(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var lang string
	if err := json.Unmarshal(decodeECalcCatalog(t, rr)["lang"], &lang); err != nil {
		t.Fatalf("catalog lang: %v", err)
	}
	return lang
}

func TestECalcCatalogServesLocalizedJSONWithETag(t *testing.T) {
	rr := serveECalc(newECalcTestServer(false), http.MethodGet, "/api/ecalc/catalog?lang=de", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if etag := rr.Header().Get("ETag"); !strings.HasPrefix(etag, `"ecalc-`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("ETag = %q, want a quoted ecalc- tag", etag)
	}
	doc := decodeECalcCatalog(t, rr)
	for _, key := range []string{"version", "lang", "units", "categories", "items"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("catalog is missing %q: %s", key, rr.Body.String())
		}
	}
	if got := ecalcLang(t, rr); got != "de" {
		t.Fatalf("lang = %q, want de", got)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(doc["items"], &items); err != nil || len(items) == 0 {
		t.Fatalf("items must be a non-empty array: %v (%s)", err, doc["items"])
	}
}

func TestECalcCatalogFallsBackToEnglish(t *testing.T) {
	s := newECalcTestServer(false)
	english := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=en", nil)
	for _, target := range []string{"/api/ecalc/catalog", "/api/ecalc/catalog?lang=", "/api/ecalc/catalog?lang=xx"} {
		rr := serveECalc(s, http.MethodGet, target, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", target, rr.Code)
		}
		if got := ecalcLang(t, rr); got != "en" {
			t.Fatalf("%s: lang = %q, want en", target, got)
		}
		if rr.Header().Get("ETag") != english.Header().Get("ETag") {
			t.Fatalf("%s: ETag %q differs from the English one %q", target, rr.Header().Get("ETag"), english.Header().Get("ETag"))
		}
	}
}

func TestECalcCatalogAnswers304ForMatchingETag(t *testing.T) {
	s := newECalcTestServer(false)
	first := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first answer carries no ETag")
	}
	for _, match := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		rr := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", map[string]string{"If-None-Match": match})
		if rr.Code != http.StatusNotModified {
			t.Fatalf("If-None-Match %q: status = %d, want 304", match, rr.Code)
		}
		if rr.Body.Len() != 0 {
			t.Fatalf("If-None-Match %q: a 304 must not carry a body, got %d bytes", match, rr.Body.Len())
		}
		if got := rr.Header().Get("ETag"); got != etag {
			t.Fatalf("If-None-Match %q: 304 ETag = %q, want %q", match, got, etag)
		}
	}
	stale := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", map[string]string{"If-None-Match": `"ecalc-0000000000000000"`})
	if stale.Code != http.StatusOK || stale.Body.Len() == 0 {
		t.Fatalf("a stale If-None-Match must get the full catalog, got status %d with %d bytes", stale.Code, stale.Body.Len())
	}
}

func TestECalcCatalogIsGetOnly(t *testing.T) {
	s := newECalcTestServer(false)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		rr := serveECalc(s, method, "/api/ecalc/catalog", nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: status = %d, want 405", method, rr.Code)
		}
		if got := rr.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s: Allow = %q, want GET", method, got)
		}
	}
}

func TestECalcCatalogRequiresASession(t *testing.T) {
	rr := serveECalc(newECalcTestServer(true), http.MethodGet, "/api/ecalc/catalog", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a session; body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "items") {
		t.Fatalf("an unauthenticated answer must not carry the catalog: %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run:

```
go test ./internal/server -run 'TestECalcCatalog' -count=1
```

Expected: FAIL at build time, `internal\server\ecalc_handlers_test.go:26:2: undefined: registerECalcRoutes`.

- [ ] **Step 3: Write the handler**

Create `internal/server/ecalc_handlers.go`:

```go
package server

import (
	"net/http"
	"strings"

	"aurago/internal/ecalc"
)

const ecalcCatalogPath = "/api/ecalc/catalog"

func registerECalcRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc(ecalcCatalogPath, handleECalcCatalog(s))
}

// handleECalcCatalog serves GET /api/ecalc/catalog?lang=xx: the embedded ECalc catalog
// with names and documents resolved for one language (unknown or empty means English).
// The route is read-only and has no state or config switch. It is gated like the other
// desktop app routes (session, desktop read scope). The ETag is the one LocalizedJSON
// computed for the body, and a matching If-None-Match answers 304 without a body.
func handleECalcCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		catalog, err := ecalc.Default()
		if err != nil {
			s.Logger.Error("The ECalc catalog could not be loaded", "error", err)
			http.Error(w, "ECalc catalog unavailable", http.StatusInternalServerError)
			return
		}
		body, etag, err := catalog.LocalizedJSON(r.URL.Query().Get("lang"))
		if err != nil {
			s.Logger.Error("The ECalc catalog could not be localized", "error", err)
			http.Error(w, "ECalc catalog unavailable", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("ETag", etag)
		h.Set("Cache-Control", "no-cache")
		if ecalcETagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// ecalcETagMatches reports whether an If-None-Match header names etag. It compares
// weakly, as RFC 9110 prescribes for If-None-Match (a W/ prefix on either side is
// ignored), and "*" matches any tag.
func ecalcETagMatches(header, etag string) bool {
	if strings.TrimSpace(header) == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Register the route**

In `internal/server/server_routes.go` (func (s *Server) run, the "Pixel image editor endpoints" block, lines 664-676 at the base commit), replace this exact text:

```go
	mux.HandleFunc("/api/pixel/cutout", handlePixelCutout(s))

	// Galaxa Deluxe highscore endpoints
```

with:

```go
	mux.HandleFunc("/api/pixel/cutout", handlePixelCutout(s))

	// ECalc formula catalog (read-only, embedded in the binary)
	registerECalcRoutes(mux, s)

	// Galaxa Deluxe highscore endpoints
```

Indentation inside the Go blocks is tabs.

- [ ] **Step 5: Run the tests and confirm they pass**

Run:

```
go test ./internal/server -run 'TestECalcCatalog' -count=1 -v
```

Expected: five `--- PASS` lines (`TestECalcCatalogServesLocalizedJSONWithETag`, `TestECalcCatalogFallsBackToEnglish`, `TestECalcCatalogAnswers304ForMatchingETag`, `TestECalcCatalogIsGetOnly`, `TestECalcCatalogRequiresASession`) and `ok  	aurago/internal/server`.

- [ ] **Step 6: Document the route**

In `internal/server/AGENTS.md` (end of the "Docker Manager API" section, line 333 at the base commit, directly before "### Code Studio" (line 335)), replace this exact text:

```markdown
Stack behaviour is in `../dockerstacks/AGENTS.md`, hosts and Engine reads in `../dockerhost/AGENTS.md`.

### Code Studio
```

with:

```markdown
Stack behaviour is in `../dockerstacks/AGENTS.md`, hosts and Engine reads in `../dockerhost/AGENTS.md`.

### ECalc catalog API

- `GET /api/ecalc/catalog?lang=xx` (`ecalc_handlers.go`, wired by `registerECalcRoutes`) serves the embedded `internal/ecalc` catalog with names and documents resolved for one language; an unknown or empty `lang` is English. It has no state and no config switch. The gate is the one of the Pixel routes: the generic `/api/` session auth, then `requireDesktopOperation` with the desktop read scope. Only GET is allowed (405 with `Allow: GET`). The body, its `ETag` and the per-language cache come from `ecalc.Catalog.LocalizedJSON`; the handler adds `Cache-Control: no-cache` and answers 304 without a body when `If-None-Match` names the tag (weak comparison, `*` matches). A catalog that cannot load or localize is a logged 500 with a generic message. The route has its own `RouteContractManifest` entry (`session`, GET, `application/json`). Verify `go test ./internal/server -run TestECalcCatalog` and `go test ./internal/audit -run Manifest`. The catalog itself (schema, expression language, solver) lives in `internal/ecalc`, which imports nothing from the server.

### Code Studio
```

- [ ] **Step 7: Format and vet**

Run:

```
gofmt -l internal/server/ecalc_handlers.go internal/server/ecalc_handlers_test.go internal/server/server_routes.go
go vet ./internal/server
```

Expected: `gofmt -l` prints nothing, `go vet` prints nothing.

- [ ] **Step 8: Commit**

```
git add internal/server/ecalc_handlers.go internal/server/ecalc_handlers_test.go internal/server/server_routes.go internal/server/AGENTS.md
git commit -m "feat(ecalc): serve the localized catalog at GET /api/ecalc/catalog

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B2: Audit route contract entry

**Runs after B1** (it pins the route B1 registers; it does not need Part A by itself).

**GitNexus impact:** `RouteContractManifest` (`internal/audit/audit_manifests.go`) reports risk UNKNOWN with no callers in the graph, which is not an all-clear. A text search confirms the callers: `TestRouteContractManifestCoversRegisteredServerRoutes` (`audit_manifests_test.go`), `TestLocalWikipediaDesktopRoutesHaveReadOnlyContract` and `TestLocalWikipediaRoutesHaveAdminContract` (`local_wikipedia_manifest_test.go`). All are tests. The change is one added data entry. `/api/ecalc/catalog` is already covered by the generic `/api/` entry, so the coverage check would pass without this task; the explicit entry and the new assertion pin the contract (GET only, session).

**Files:**
- Modify: `internal/audit/audit_manifests_test.go` — func `TestRouteContractManifestCoversRegisteredServerRoutes` (lines 135-139 at the base commit)
- Modify: `internal/audit/audit_manifests.go` — func `RouteContractManifest`, between the `/api/virtual-computers/` and `/api/daemons` entries (lines 169-170 at the base commit)

- [ ] **Step 1: Write the failing assertion**

In `internal/audit/audit_manifests_test.go` (func TestRouteContractManifestCoversRegisteredServerRoutes, lines 135-139 at the base commit), replace this exact text:

```go
		t.Fatal("/api/health/discord must have an explicit session route contract")
	}
```

with:

```go
		t.Fatal("/api/health/discord must have an explicit session route contract")
	}
	if !routeContractExists(contracts, "/api/ecalc/catalog", "session") {
		t.Fatal("/api/ecalc/catalog must have an explicit session route contract")
	}
	for _, contract := range contracts {
		if contract.Pattern == "/api/ecalc/catalog" && !slices.Equal(contract.Methods, []string{"GET"}) {
			t.Fatalf("/api/ecalc/catalog contract %+v must be GET only", contract)
		}
	}
```

Indentation inside the Go blocks is tabs.

- [ ] **Step 2: Run the test and confirm it fails**

Run:

```
go test ./internal/audit -run 'TestRouteContractManifestCoversRegisteredServerRoutes' -count=1
```

Expected: FAIL with `audit_manifests_test.go:139: /api/ecalc/catalog must have an explicit session route contract`.

- [ ] **Step 3: Add the contract entry**

In `internal/audit/audit_manifests.go` (func RouteContractManifest, between the "/api/virtual-computers/" entry (line 169) and the "/api/daemons" entry (line 170) at the base commit), replace this exact text:

```go
		{Pattern: "/api/virtual-computers/", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session-or-desktop-token", Category: "desktop", ContentTypes: []string{"application/json", "websocket", "proxy"}},
		{Pattern: "/api/daemons", Methods: []string{"GET", "POST"}, Auth: "session", Category: "skills"},
```

with:

```go
		{Pattern: "/api/virtual-computers/", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session-or-desktop-token", Category: "desktop", ContentTypes: []string{"application/json", "websocket", "proxy"}},
		{Pattern: "/api/ecalc/catalog", Methods: []string{"GET"}, Auth: "session", Category: "desktop", ContentTypes: []string{"application/json"}},
		{Pattern: "/api/daemons", Methods: []string{"GET", "POST"}, Auth: "session", Category: "skills"},
```

Indentation inside the Go blocks is tabs.

- [ ] **Step 4: Run the audit manifest tests and confirm they pass**

Run:

```
go test ./internal/audit -run 'Manifest' -count=1
gofmt -l internal/audit/audit_manifests.go internal/audit/audit_manifests_test.go
```

Expected: `ok  	aurago/internal/audit`, and `gofmt -l` prints nothing. (The whole `./internal/audit` package has one failure that is unrelated and already present at the base commit on Windows: `TestUpdateConfigFallbackOnlyTrustedTransactionBackups`. Run the package with `-run Manifest` as `internal/server/AGENTS.md` prescribes.)

- [ ] **Step 5: Commit**

```
git add internal/audit/audit_manifests.go internal/audit/audit_manifests_test.go
git commit -m "feat(audit): add the ECalc catalog route to the route contract

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B3: Locale keys for all 16 desktop languages

No Go symbol is touched (JSON data and one test file). The keys are the 43 of contract section 10: `desktop.app_ecalc` plus 42 `desktop.ecalc_*`. Placeholders use the single-brace form `{series}`; the shared `t()` replaces both `{x}` and `{{x}}`.

**Placement convention:** `ui/lang/desktop/*.json` are in one common order in all 16 files and the main region is alphabetical by key (compare the `desktop.calc_*` block). So `desktop.app_ecalc` goes between `desktop.app_delete_permanent` and `desktop.app_editor`, and the 42 `desktop.ecalc_*` keys go in one alphabetical block between `desktop.done` and `desktop.edit`. Anchors are matched by their text; the line numbers are those of the base commit (they are the same in all 16 files, and the second anchor moves down by one line after the first insertion).

**English meaning of each string** (for review of the translations): back "Back"; computed "Computed"; copied "Copied"; copy_results "Copy results"; derived "Derived values"; dim_* are the dimension names Voltage, Current, Resistance, Power, Charge, Energy, Number, Gain; eseries_nearest "Nearest {series} value"; eseries_series "E series"; lock "Lock this value"; unlock "Unlock this value"; unit_auto "Auto"; symbols_iec "IEC (rectangular resistor)"; symbols_ansi "ANSI (zigzag resistor)". The complete English strings are in the `en` block below.

**Files:**
- Create: `ui/desktop_ecalc_i18n_test.go`
- Modify: `ui/lang/desktop/cs.json`, `da.json`, `de.json`, `el.json`, `en.json`, `es.json`, `fr.json`, `hi.json`, `it.json`, `ja.json`, `nl.json`, `no.json`, `pl.json`, `pt.json`, `sv.json`, `zh.json` (two insertions each: line 750 and line 1127 at the base commit)

- [ ] **Step 1: Write the failing test**

Create `ui/desktop_ecalc_i18n_test.go`:

```go
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ecalcLocaleKeys lists every desktop.* string of the ECalc app (ECalc phase 1 contract, section 10).
var ecalcLocaleKeys = []string{
	"desktop.app_ecalc",
	"desktop.ecalc_search_placeholder", "desktop.ecalc_favorites", "desktop.ecalc_recent",
	"desktop.ecalc_empty_favorites", "desktop.ecalc_empty_recent", "desktop.ecalc_no_results",
	"desktop.ecalc_favorite_add", "desktop.ecalc_favorite_remove", "desktop.ecalc_copy_results",
	"desktop.ecalc_copied", "desktop.ecalc_reset", "desktop.ecalc_explanation", "desktop.ecalc_source",
	"desktop.ecalc_derived", "desktop.ecalc_eseries_nearest", "desktop.ecalc_eseries_series",
	"desktop.ecalc_lock", "desktop.ecalc_unlock", "desktop.ecalc_unit_auto",
	"desktop.ecalc_error_undetermined", "desktop.ecalc_error_nonfinite", "desktop.ecalc_error_range",
	"desktop.ecalc_error_parse", "desktop.ecalc_settings", "desktop.ecalc_settings_digits",
	"desktop.ecalc_settings_symbols", "desktop.ecalc_symbols_iec", "desktop.ecalc_symbols_ansi",
	"desktop.ecalc_back", "desktop.ecalc_load_failed", "desktop.ecalc_retry", "desktop.ecalc_warnings",
	"desktop.ecalc_inputs", "desktop.ecalc_computed",
	"desktop.ecalc_dim_voltage", "desktop.ecalc_dim_current", "desktop.ecalc_dim_resistance",
	"desktop.ecalc_dim_power", "desktop.ecalc_dim_charge", "desktop.ecalc_dim_energy",
	"desktop.ecalc_dim_number", "desktop.ecalc_dim_gain",
}

func TestDesktopECalcLocaleKeysExistInAllLanguages(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range ecalcLocaleKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s is missing %s", path, key)
			}
		}
		if !strings.Contains(values["desktop.ecalc_eseries_nearest"], "{series}") {
			t.Fatalf("%s: desktop.ecalc_eseries_nearest must keep the {series} placeholder: %q", path, values["desktop.ecalc_eseries_nearest"])
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run:

```
go test ./ui -run 'TestDesktopECalcLocaleKeysExistInAllLanguages' -count=1
```

Expected: FAIL with `lang\desktop\cs.json is missing desktop.app_ecalc`.

- [ ] **Step 3: Insert the keys into the 16 files**

For each language below, make the two insertions. Every inserted line is indented by two spaces and ends in a comma, and each insertion is followed by an existing key, so the files stay valid JSON.

**cs** (`ui/lang/desktop/cs.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Trvale smazat",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Hotovo",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Zpět",
  "desktop.ecalc_computed": "Vypočteno",
  "desktop.ecalc_copied": "Zkopírováno",
  "desktop.ecalc_copy_results": "Kopírovat výsledky",
  "desktop.ecalc_derived": "Odvozené hodnoty",
  "desktop.ecalc_dim_charge": "Náboj",
  "desktop.ecalc_dim_current": "Proud",
  "desktop.ecalc_dim_energy": "Energie",
  "desktop.ecalc_dim_gain": "Zesílení",
  "desktop.ecalc_dim_number": "Číslo",
  "desktop.ecalc_dim_power": "Výkon",
  "desktop.ecalc_dim_resistance": "Odpor",
  "desktop.ecalc_dim_voltage": "Napětí",
  "desktop.ecalc_empty_favorites": "Zatím žádné oblíbené. Označte kalkulačku jako oblíbenou, aby se zobrazila zde.",
  "desktop.ecalc_empty_recent": "Zatím nic otevřeno. Naposledy použité kalkulačky se zobrazí zde.",
  "desktop.ecalc_error_nonfinite": "Pro tyto hodnoty neexistuje konečný výsledek.",
  "desktop.ecalc_error_parse": "Hodnotu se nepodařilo přečíst. Příklady: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Hodnota musí být větší než nula.",
  "desktop.ecalc_error_undetermined": "Pro výpočet zadejte další hodnoty.",
  "desktop.ecalc_eseries_nearest": "Nejbližší hodnota řady {series}",
  "desktop.ecalc_eseries_series": "Řada E",
  "desktop.ecalc_explanation": "Vysvětlení",
  "desktop.ecalc_favorite_add": "Přidat do oblíbených",
  "desktop.ecalc_favorite_remove": "Odebrat z oblíbených",
  "desktop.ecalc_favorites": "Oblíbené",
  "desktop.ecalc_inputs": "Vstupy",
  "desktop.ecalc_load_failed": "Katalog ECalc se nepodařilo načíst.",
  "desktop.ecalc_lock": "Zamknout hodnotu",
  "desktop.ecalc_no_results": "Nic nenalezeno.",
  "desktop.ecalc_recent": "Naposledy použité",
  "desktop.ecalc_reset": "Obnovit výchozí hodnoty",
  "desktop.ecalc_retry": "Zkusit znovu",
  "desktop.ecalc_search_placeholder": "Hledat podle názvu nebo klíčového slova…",
  "desktop.ecalc_settings": "Nastavení",
  "desktop.ecalc_settings_digits": "Platné číslice",
  "desktop.ecalc_settings_symbols": "Schematické značky",
  "desktop.ecalc_source": "Zdroj",
  "desktop.ecalc_symbols_ansi": "ANSI (klikatý rezistor)",
  "desktop.ecalc_symbols_iec": "IEC (obdélníkový rezistor)",
  "desktop.ecalc_unit_auto": "Automaticky",
  "desktop.ecalc_unlock": "Odemknout hodnotu",
  "desktop.ecalc_warnings": "Upozornění",
```

**da** (`ui/lang/desktop/da.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Slet permanent",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Færdig",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Tilbage",
  "desktop.ecalc_computed": "Beregnet",
  "desktop.ecalc_copied": "Kopieret",
  "desktop.ecalc_copy_results": "Kopiér resultater",
  "desktop.ecalc_derived": "Afledte værdier",
  "desktop.ecalc_dim_charge": "Ladning",
  "desktop.ecalc_dim_current": "Strøm",
  "desktop.ecalc_dim_energy": "Energi",
  "desktop.ecalc_dim_gain": "Forstærkning",
  "desktop.ecalc_dim_number": "Tal",
  "desktop.ecalc_dim_power": "Effekt",
  "desktop.ecalc_dim_resistance": "Modstand",
  "desktop.ecalc_dim_voltage": "Spænding",
  "desktop.ecalc_empty_favorites": "Ingen favoritter endnu. Markér en lommeregner som favorit for at finde den her.",
  "desktop.ecalc_empty_recent": "Intet åbnet endnu. Senest brugte lommeregnere vises her.",
  "desktop.ecalc_error_nonfinite": "Disse værdier giver intet endeligt resultat.",
  "desktop.ecalc_error_parse": "Værdien kunne ikke læses. Eksempler: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Værdien skal være større end nul.",
  "desktop.ecalc_error_undetermined": "Angiv flere værdier for at beregne dette.",
  "desktop.ecalc_eseries_nearest": "Nærmeste {series}-værdi",
  "desktop.ecalc_eseries_series": "E-serie",
  "desktop.ecalc_explanation": "Forklaring",
  "desktop.ecalc_favorite_add": "Føj til favoritter",
  "desktop.ecalc_favorite_remove": "Fjern fra favoritter",
  "desktop.ecalc_favorites": "Favoritter",
  "desktop.ecalc_inputs": "Inputværdier",
  "desktop.ecalc_load_failed": "ECalc-kataloget kunne ikke indlæses.",
  "desktop.ecalc_lock": "Lås værdi",
  "desktop.ecalc_no_results": "Ingen resultater.",
  "desktop.ecalc_recent": "Senest brugt",
  "desktop.ecalc_reset": "Nulstil til standardværdier",
  "desktop.ecalc_retry": "Prøv igen",
  "desktop.ecalc_search_placeholder": "Søg efter navn eller nøgleord…",
  "desktop.ecalc_settings": "Indstillinger",
  "desktop.ecalc_settings_digits": "Betydende cifre",
  "desktop.ecalc_settings_symbols": "Kredsløbssymboler",
  "desktop.ecalc_source": "Kilde",
  "desktop.ecalc_symbols_ansi": "ANSI (zigzag-modstand)",
  "desktop.ecalc_symbols_iec": "IEC (rektangulær modstand)",
  "desktop.ecalc_unit_auto": "Automatisk",
  "desktop.ecalc_unlock": "Lås værdi op",
  "desktop.ecalc_warnings": "Advarsler",
```

**de** (`ui/lang/desktop/de.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Dauerhaft löschen",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Fertig",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Zurück",
  "desktop.ecalc_computed": "Berechnet",
  "desktop.ecalc_copied": "Kopiert",
  "desktop.ecalc_copy_results": "Ergebnisse kopieren",
  "desktop.ecalc_derived": "Abgeleitete Werte",
  "desktop.ecalc_dim_charge": "Ladung",
  "desktop.ecalc_dim_current": "Strom",
  "desktop.ecalc_dim_energy": "Energie",
  "desktop.ecalc_dim_gain": "Verstärkung",
  "desktop.ecalc_dim_number": "Zahl",
  "desktop.ecalc_dim_power": "Leistung",
  "desktop.ecalc_dim_resistance": "Widerstand",
  "desktop.ecalc_dim_voltage": "Spannung",
  "desktop.ecalc_empty_favorites": "Noch keine Favoriten. Markiere einen Rechner als Favorit, um ihn hier zu finden.",
  "desktop.ecalc_empty_recent": "Noch nichts geöffnet. Zuletzt benutzte Rechner erscheinen hier.",
  "desktop.ecalc_error_nonfinite": "Mit diesen Werten gibt es kein endliches Ergebnis.",
  "desktop.ecalc_error_parse": "Dieser Wert ist nicht lesbar. Beispiele: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Dieser Wert muss größer als null sein.",
  "desktop.ecalc_error_undetermined": "Gib weitere Werte ein, um das zu berechnen.",
  "desktop.ecalc_eseries_nearest": "Nächster {series}-Wert",
  "desktop.ecalc_eseries_series": "E-Reihe",
  "desktop.ecalc_explanation": "Erklärung",
  "desktop.ecalc_favorite_add": "Zu Favoriten hinzufügen",
  "desktop.ecalc_favorite_remove": "Aus Favoriten entfernen",
  "desktop.ecalc_favorites": "Favoriten",
  "desktop.ecalc_inputs": "Eingaben",
  "desktop.ecalc_load_failed": "Der ECalc-Katalog konnte nicht geladen werden.",
  "desktop.ecalc_lock": "Wert sperren",
  "desktop.ecalc_no_results": "Keine Treffer.",
  "desktop.ecalc_recent": "Zuletzt verwendet",
  "desktop.ecalc_reset": "Auf Standardwerte zurücksetzen",
  "desktop.ecalc_retry": "Erneut versuchen",
  "desktop.ecalc_search_placeholder": "Nach Name oder Stichwort suchen…",
  "desktop.ecalc_settings": "Einstellungen",
  "desktop.ecalc_settings_digits": "Signifikante Stellen",
  "desktop.ecalc_settings_symbols": "Schaltsymbole",
  "desktop.ecalc_source": "Quelle",
  "desktop.ecalc_symbols_ansi": "ANSI (Zickzack-Widerstand)",
  "desktop.ecalc_symbols_iec": "IEC (rechteckiger Widerstand)",
  "desktop.ecalc_unit_auto": "Automatisch",
  "desktop.ecalc_unlock": "Wert entsperren",
  "desktop.ecalc_warnings": "Warnungen",
```

**el** (`ui/lang/desktop/el.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Μόνιμη διαγραφή",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Ολοκληρώθηκε",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Πίσω",
  "desktop.ecalc_computed": "Υπολογισμένο",
  "desktop.ecalc_copied": "Αντιγράφηκε",
  "desktop.ecalc_copy_results": "Αντιγραφή αποτελεσμάτων",
  "desktop.ecalc_derived": "Παράγωγες τιμές",
  "desktop.ecalc_dim_charge": "Φορτίο",
  "desktop.ecalc_dim_current": "Ρεύμα",
  "desktop.ecalc_dim_energy": "Ενέργεια",
  "desktop.ecalc_dim_gain": "Κέρδος",
  "desktop.ecalc_dim_number": "Αριθμός",
  "desktop.ecalc_dim_power": "Ισχύς",
  "desktop.ecalc_dim_resistance": "Αντίσταση",
  "desktop.ecalc_dim_voltage": "Τάση",
  "desktop.ecalc_empty_favorites": "Δεν υπάρχουν αγαπημένα ακόμη. Σημειώστε μια αριθμομηχανή ως αγαπημένη για να τη βρίσκετε εδώ.",
  "desktop.ecalc_empty_recent": "Δεν έχει ανοιχτεί τίποτα ακόμη. Οι αριθμομηχανές που χρησιμοποιήσατε πρόσφατα εμφανίζονται εδώ.",
  "desktop.ecalc_error_nonfinite": "Αυτές οι τιμές δεν δίνουν πεπερασμένο αποτέλεσμα.",
  "desktop.ecalc_error_parse": "Η τιμή δεν μπόρεσε να διαβαστεί. Παραδείγματα: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Η τιμή πρέπει να είναι μεγαλύτερη από μηδέν.",
  "desktop.ecalc_error_undetermined": "Εισαγάγετε περισσότερες τιμές για να υπολογιστεί αυτό.",
  "desktop.ecalc_eseries_nearest": "Πλησιέστερη τιμή της σειράς {series}",
  "desktop.ecalc_eseries_series": "Σειρά E",
  "desktop.ecalc_explanation": "Επεξήγηση",
  "desktop.ecalc_favorite_add": "Προσθήκη στα αγαπημένα",
  "desktop.ecalc_favorite_remove": "Αφαίρεση από τα αγαπημένα",
  "desktop.ecalc_favorites": "Αγαπημένα",
  "desktop.ecalc_inputs": "Είσοδοι",
  "desktop.ecalc_load_failed": "Δεν ήταν δυνατή η φόρτωση του καταλόγου ECalc.",
  "desktop.ecalc_lock": "Κλείδωμα τιμής",
  "desktop.ecalc_no_results": "Δεν βρέθηκαν αποτελέσματα.",
  "desktop.ecalc_recent": "Πρόσφατα",
  "desktop.ecalc_reset": "Επαναφορά προεπιλογών",
  "desktop.ecalc_retry": "Δοκιμάστε ξανά",
  "desktop.ecalc_search_placeholder": "Αναζήτηση με όνομα ή λέξη-κλειδί…",
  "desktop.ecalc_settings": "Ρυθμίσεις",
  "desktop.ecalc_settings_digits": "Σημαντικά ψηφία",
  "desktop.ecalc_settings_symbols": "Σύμβολα κυκλωμάτων",
  "desktop.ecalc_source": "Πηγή",
  "desktop.ecalc_symbols_ansi": "ANSI (αντίσταση ζιγκ-ζαγκ)",
  "desktop.ecalc_symbols_iec": "IEC (ορθογώνια αντίσταση)",
  "desktop.ecalc_unit_auto": "Αυτόματα",
  "desktop.ecalc_unlock": "Ξεκλείδωμα τιμής",
  "desktop.ecalc_warnings": "Προειδοποιήσεις",
```

**en** (`ui/lang/desktop/en.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Delete permanently",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Done",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Back",
  "desktop.ecalc_computed": "Computed",
  "desktop.ecalc_copied": "Copied",
  "desktop.ecalc_copy_results": "Copy results",
  "desktop.ecalc_derived": "Derived values",
  "desktop.ecalc_dim_charge": "Charge",
  "desktop.ecalc_dim_current": "Current",
  "desktop.ecalc_dim_energy": "Energy",
  "desktop.ecalc_dim_gain": "Gain",
  "desktop.ecalc_dim_number": "Number",
  "desktop.ecalc_dim_power": "Power",
  "desktop.ecalc_dim_resistance": "Resistance",
  "desktop.ecalc_dim_voltage": "Voltage",
  "desktop.ecalc_empty_favorites": "No favorites yet. Mark a calculator as a favorite to find it here.",
  "desktop.ecalc_empty_recent": "Nothing opened yet. Recently used calculators will show up here.",
  "desktop.ecalc_error_nonfinite": "These values give no finite result.",
  "desktop.ecalc_error_parse": "Could not read this value. Examples: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "This value must be greater than zero.",
  "desktop.ecalc_error_undetermined": "Enter more values to calculate this.",
  "desktop.ecalc_eseries_nearest": "Nearest {series} value",
  "desktop.ecalc_eseries_series": "E series",
  "desktop.ecalc_explanation": "Explanation",
  "desktop.ecalc_favorite_add": "Add to favorites",
  "desktop.ecalc_favorite_remove": "Remove from favorites",
  "desktop.ecalc_favorites": "Favorites",
  "desktop.ecalc_inputs": "Inputs",
  "desktop.ecalc_load_failed": "The ECalc catalog could not be loaded.",
  "desktop.ecalc_lock": "Lock this value",
  "desktop.ecalc_no_results": "No matches found.",
  "desktop.ecalc_recent": "Recent",
  "desktop.ecalc_reset": "Reset to defaults",
  "desktop.ecalc_retry": "Try again",
  "desktop.ecalc_search_placeholder": "Search by name or keyword…",
  "desktop.ecalc_settings": "Settings",
  "desktop.ecalc_settings_digits": "Significant digits",
  "desktop.ecalc_settings_symbols": "Schematic symbols",
  "desktop.ecalc_source": "Source",
  "desktop.ecalc_symbols_ansi": "ANSI (zigzag resistor)",
  "desktop.ecalc_symbols_iec": "IEC (rectangular resistor)",
  "desktop.ecalc_unit_auto": "Auto",
  "desktop.ecalc_unlock": "Unlock this value",
  "desktop.ecalc_warnings": "Warnings",
```

**es** (`ui/lang/desktop/es.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Eliminar permanentemente",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Hecho",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Atrás",
  "desktop.ecalc_computed": "Calculado",
  "desktop.ecalc_copied": "Copiado",
  "desktop.ecalc_copy_results": "Copiar resultados",
  "desktop.ecalc_derived": "Valores derivados",
  "desktop.ecalc_dim_charge": "Carga",
  "desktop.ecalc_dim_current": "Corriente",
  "desktop.ecalc_dim_energy": "Energía",
  "desktop.ecalc_dim_gain": "Ganancia",
  "desktop.ecalc_dim_number": "Número",
  "desktop.ecalc_dim_power": "Potencia",
  "desktop.ecalc_dim_resistance": "Resistencia",
  "desktop.ecalc_dim_voltage": "Tensión",
  "desktop.ecalc_empty_favorites": "Aún no hay favoritos. Marca una calculadora como favorita para encontrarla aquí.",
  "desktop.ecalc_empty_recent": "Todavía no se ha abierto nada. Las calculadoras usadas recientemente aparecerán aquí.",
  "desktop.ecalc_error_nonfinite": "Estos valores no dan un resultado finito.",
  "desktop.ecalc_error_parse": "No se pudo leer este valor. Ejemplos: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Este valor debe ser mayor que cero.",
  "desktop.ecalc_error_undetermined": "Introduce más valores para calcular esto.",
  "desktop.ecalc_eseries_nearest": "Valor más cercano de la serie {series}",
  "desktop.ecalc_eseries_series": "Serie E",
  "desktop.ecalc_explanation": "Explicación",
  "desktop.ecalc_favorite_add": "Añadir a favoritos",
  "desktop.ecalc_favorite_remove": "Quitar de favoritos",
  "desktop.ecalc_favorites": "Favoritos",
  "desktop.ecalc_inputs": "Entradas",
  "desktop.ecalc_load_failed": "No se pudo cargar el catálogo de ECalc.",
  "desktop.ecalc_lock": "Bloquear valor",
  "desktop.ecalc_no_results": "No se encontró nada.",
  "desktop.ecalc_recent": "Recientes",
  "desktop.ecalc_reset": "Restablecer valores predeterminados",
  "desktop.ecalc_retry": "Reintentar",
  "desktop.ecalc_search_placeholder": "Buscar por nombre o palabra clave…",
  "desktop.ecalc_settings": "Ajustes",
  "desktop.ecalc_settings_digits": "Cifras significativas",
  "desktop.ecalc_settings_symbols": "Símbolos de esquemas",
  "desktop.ecalc_source": "Fuente",
  "desktop.ecalc_symbols_ansi": "ANSI (resistencia en zigzag)",
  "desktop.ecalc_symbols_iec": "IEC (resistencia rectangular)",
  "desktop.ecalc_unit_auto": "Automático",
  "desktop.ecalc_unlock": "Desbloquear valor",
  "desktop.ecalc_warnings": "Avisos",
```

**fr** (`ui/lang/desktop/fr.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Supprimer définitivement",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Terminé",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Retour",
  "desktop.ecalc_computed": "Calculé",
  "desktop.ecalc_copied": "Copié",
  "desktop.ecalc_copy_results": "Copier les résultats",
  "desktop.ecalc_derived": "Valeurs dérivées",
  "desktop.ecalc_dim_charge": "Charge",
  "desktop.ecalc_dim_current": "Courant",
  "desktop.ecalc_dim_energy": "Énergie",
  "desktop.ecalc_dim_gain": "Gain",
  "desktop.ecalc_dim_number": "Nombre",
  "desktop.ecalc_dim_power": "Puissance",
  "desktop.ecalc_dim_resistance": "Résistance",
  "desktop.ecalc_dim_voltage": "Tension",
  "desktop.ecalc_empty_favorites": "Aucun favori pour l’instant. Marque une calculatrice comme favorite pour la retrouver ici.",
  "desktop.ecalc_empty_recent": "Rien d’ouvert pour l’instant. Les calculatrices utilisées récemment apparaissent ici.",
  "desktop.ecalc_error_nonfinite": "Ces valeurs ne donnent pas de résultat fini.",
  "desktop.ecalc_error_parse": "Impossible de lire cette valeur. Exemples : 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Cette valeur doit être supérieure à zéro.",
  "desktop.ecalc_error_undetermined": "Saisis d’autres valeurs pour calculer ceci.",
  "desktop.ecalc_eseries_nearest": "Valeur la plus proche de la série {series}",
  "desktop.ecalc_eseries_series": "Série E",
  "desktop.ecalc_explanation": "Explication",
  "desktop.ecalc_favorite_add": "Ajouter aux favoris",
  "desktop.ecalc_favorite_remove": "Retirer des favoris",
  "desktop.ecalc_favorites": "Favoris",
  "desktop.ecalc_inputs": "Entrées",
  "desktop.ecalc_load_failed": "Impossible de charger le catalogue ECalc.",
  "desktop.ecalc_lock": "Verrouiller la valeur",
  "desktop.ecalc_no_results": "Aucun résultat.",
  "desktop.ecalc_recent": "Récents",
  "desktop.ecalc_reset": "Rétablir les valeurs par défaut",
  "desktop.ecalc_retry": "Réessayer",
  "desktop.ecalc_search_placeholder": "Rechercher par nom ou mot-clé…",
  "desktop.ecalc_settings": "Paramètres",
  "desktop.ecalc_settings_digits": "Chiffres significatifs",
  "desktop.ecalc_settings_symbols": "Symboles des schémas",
  "desktop.ecalc_source": "Source",
  "desktop.ecalc_symbols_ansi": "ANSI (résistance en zigzag)",
  "desktop.ecalc_symbols_iec": "IEC (résistance rectangulaire)",
  "desktop.ecalc_unit_auto": "Auto",
  "desktop.ecalc_unlock": "Déverrouiller la valeur",
  "desktop.ecalc_warnings": "Avertissements",
```

**hi** (`ui/lang/desktop/hi.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "स्थायी रूप से हटाएँ",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "पूरा",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "वापस",
  "desktop.ecalc_computed": "परिकलित",
  "desktop.ecalc_copied": "कॉपी हो गया",
  "desktop.ecalc_copy_results": "परिणाम कॉपी करें",
  "desktop.ecalc_derived": "व्युत्पन्न मान",
  "desktop.ecalc_dim_charge": "आवेश",
  "desktop.ecalc_dim_current": "धारा",
  "desktop.ecalc_dim_energy": "ऊर्जा",
  "desktop.ecalc_dim_gain": "गेन",
  "desktop.ecalc_dim_number": "संख्या",
  "desktop.ecalc_dim_power": "शक्ति",
  "desktop.ecalc_dim_resistance": "प्रतिरोध",
  "desktop.ecalc_dim_voltage": "वोल्टेज",
  "desktop.ecalc_empty_favorites": "अभी कोई पसंदीदा नहीं है। किसी कैलकुलेटर को पसंदीदा के रूप में चिह्नित करें, वह यहाँ दिखाई देगा।",
  "desktop.ecalc_empty_recent": "अभी कुछ नहीं खोला गया है। हाल में इस्तेमाल किए गए कैलकुलेटर यहाँ दिखाई देंगे।",
  "desktop.ecalc_error_nonfinite": "इन मानों से कोई सीमित परिणाम नहीं मिलता।",
  "desktop.ecalc_error_parse": "यह मान पढ़ा नहीं जा सका। उदाहरण: 4k7, 10 mA, 1e-6।",
  "desktop.ecalc_error_range": "यह मान शून्य से बड़ा होना चाहिए।",
  "desktop.ecalc_error_undetermined": "इसकी गणना के लिए और मान दर्ज करें।",
  "desktop.ecalc_eseries_nearest": "निकटतम {series} मान",
  "desktop.ecalc_eseries_series": "E-श्रृंखला",
  "desktop.ecalc_explanation": "व्याख्या",
  "desktop.ecalc_favorite_add": "पसंदीदा में जोड़ें",
  "desktop.ecalc_favorite_remove": "पसंदीदा से हटाएँ",
  "desktop.ecalc_favorites": "पसंदीदा",
  "desktop.ecalc_inputs": "इनपुट",
  "desktop.ecalc_load_failed": "ECalc कैटलॉग लोड नहीं हो सका।",
  "desktop.ecalc_lock": "मान लॉक करें",
  "desktop.ecalc_no_results": "कोई परिणाम नहीं मिला।",
  "desktop.ecalc_recent": "हाल के",
  "desktop.ecalc_reset": "डिफ़ॉल्ट पर रीसेट करें",
  "desktop.ecalc_retry": "फिर कोशिश करें",
  "desktop.ecalc_search_placeholder": "नाम या कीवर्ड से खोजें…",
  "desktop.ecalc_settings": "सेटिंग्स",
  "desktop.ecalc_settings_digits": "सार्थक अंक",
  "desktop.ecalc_settings_symbols": "सर्किट प्रतीक",
  "desktop.ecalc_source": "स्रोत",
  "desktop.ecalc_symbols_ansi": "ANSI (ज़िगज़ैग प्रतिरोधक)",
  "desktop.ecalc_symbols_iec": "IEC (आयताकार प्रतिरोधक)",
  "desktop.ecalc_unit_auto": "स्वचालित",
  "desktop.ecalc_unlock": "मान अनलॉक करें",
  "desktop.ecalc_warnings": "चेतावनियाँ",
```

**it** (`ui/lang/desktop/it.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Elimina definitivamente",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Fatto",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Indietro",
  "desktop.ecalc_computed": "Calcolato",
  "desktop.ecalc_copied": "Copiato",
  "desktop.ecalc_copy_results": "Copia i risultati",
  "desktop.ecalc_derived": "Valori derivati",
  "desktop.ecalc_dim_charge": "Carica",
  "desktop.ecalc_dim_current": "Corrente",
  "desktop.ecalc_dim_energy": "Energia",
  "desktop.ecalc_dim_gain": "Guadagno",
  "desktop.ecalc_dim_number": "Numero",
  "desktop.ecalc_dim_power": "Potenza",
  "desktop.ecalc_dim_resistance": "Resistenza",
  "desktop.ecalc_dim_voltage": "Tensione",
  "desktop.ecalc_empty_favorites": "Ancora nessun preferito. Contrassegna una calcolatrice come preferita per ritrovarla qui.",
  "desktop.ecalc_empty_recent": "Ancora nulla di aperto. Le calcolatrici usate di recente compaiono qui.",
  "desktop.ecalc_error_nonfinite": "Questi valori non danno un risultato finito.",
  "desktop.ecalc_error_parse": "Impossibile leggere questo valore. Esempi: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Questo valore deve essere maggiore di zero.",
  "desktop.ecalc_error_undetermined": "Inserisci altri valori per calcolarlo.",
  "desktop.ecalc_eseries_nearest": "Valore più vicino della serie {series}",
  "desktop.ecalc_eseries_series": "Serie E",
  "desktop.ecalc_explanation": "Spiegazione",
  "desktop.ecalc_favorite_add": "Aggiungi ai preferiti",
  "desktop.ecalc_favorite_remove": "Rimuovi dai preferiti",
  "desktop.ecalc_favorites": "Preferiti",
  "desktop.ecalc_inputs": "Ingressi",
  "desktop.ecalc_load_failed": "Impossibile caricare il catalogo di ECalc.",
  "desktop.ecalc_lock": "Blocca il valore",
  "desktop.ecalc_no_results": "Nessun risultato.",
  "desktop.ecalc_recent": "Recenti",
  "desktop.ecalc_reset": "Ripristina i valori predefiniti",
  "desktop.ecalc_retry": "Riprova",
  "desktop.ecalc_search_placeholder": "Cerca per nome o parola chiave…",
  "desktop.ecalc_settings": "Impostazioni",
  "desktop.ecalc_settings_digits": "Cifre significative",
  "desktop.ecalc_settings_symbols": "Simboli degli schemi",
  "desktop.ecalc_source": "Fonte",
  "desktop.ecalc_symbols_ansi": "ANSI (resistore a zigzag)",
  "desktop.ecalc_symbols_iec": "IEC (resistore rettangolare)",
  "desktop.ecalc_unit_auto": "Auto",
  "desktop.ecalc_unlock": "Sblocca il valore",
  "desktop.ecalc_warnings": "Avvisi",
```

**ja** (`ui/lang/desktop/ja.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "完全に削除",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "完了",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "戻る",
  "desktop.ecalc_computed": "計算値",
  "desktop.ecalc_copied": "コピーしました",
  "desktop.ecalc_copy_results": "結果をコピー",
  "desktop.ecalc_derived": "派生値",
  "desktop.ecalc_dim_charge": "電荷",
  "desktop.ecalc_dim_current": "電流",
  "desktop.ecalc_dim_energy": "エネルギー",
  "desktop.ecalc_dim_gain": "利得",
  "desktop.ecalc_dim_number": "数値",
  "desktop.ecalc_dim_power": "電力",
  "desktop.ecalc_dim_resistance": "抵抗",
  "desktop.ecalc_dim_voltage": "電圧",
  "desktop.ecalc_empty_favorites": "お気に入りはまだありません。計算機をお気に入りに登録すると、ここに表示されます。",
  "desktop.ecalc_empty_recent": "まだ何も開いていません。最近使った計算機がここに表示されます。",
  "desktop.ecalc_error_nonfinite": "この値では有限の結果になりません。",
  "desktop.ecalc_error_parse": "この値を読み取れません。例: 4k7、10 mA、1e-6",
  "desktop.ecalc_error_range": "この値は 0 より大きくする必要があります。",
  "desktop.ecalc_error_undetermined": "これを計算するには、さらに値を入力してください。",
  "desktop.ecalc_eseries_nearest": "最も近い {series} の値",
  "desktop.ecalc_eseries_series": "E系列",
  "desktop.ecalc_explanation": "解説",
  "desktop.ecalc_favorite_add": "お気に入りに追加",
  "desktop.ecalc_favorite_remove": "お気に入りから削除",
  "desktop.ecalc_favorites": "お気に入り",
  "desktop.ecalc_inputs": "入力値",
  "desktop.ecalc_load_failed": "ECalc のカタログを読み込めませんでした。",
  "desktop.ecalc_lock": "値を固定",
  "desktop.ecalc_no_results": "一致する項目はありません。",
  "desktop.ecalc_recent": "最近使用",
  "desktop.ecalc_reset": "初期値に戻す",
  "desktop.ecalc_retry": "再試行",
  "desktop.ecalc_search_placeholder": "名前またはキーワードで検索…",
  "desktop.ecalc_settings": "設定",
  "desktop.ecalc_settings_digits": "有効桁数",
  "desktop.ecalc_settings_symbols": "回路図記号",
  "desktop.ecalc_source": "出典",
  "desktop.ecalc_symbols_ansi": "ANSI（ジグザグ抵抗）",
  "desktop.ecalc_symbols_iec": "IEC（長方形の抵抗）",
  "desktop.ecalc_unit_auto": "自動",
  "desktop.ecalc_unlock": "固定を解除",
  "desktop.ecalc_warnings": "警告",
```

**nl** (`ui/lang/desktop/nl.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Permanent verwijderen",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Klaar",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Terug",
  "desktop.ecalc_computed": "Berekend",
  "desktop.ecalc_copied": "Gekopieerd",
  "desktop.ecalc_copy_results": "Resultaten kopiëren",
  "desktop.ecalc_derived": "Afgeleide waarden",
  "desktop.ecalc_dim_charge": "Lading",
  "desktop.ecalc_dim_current": "Stroom",
  "desktop.ecalc_dim_energy": "Energie",
  "desktop.ecalc_dim_gain": "Versterking",
  "desktop.ecalc_dim_number": "Getal",
  "desktop.ecalc_dim_power": "Vermogen",
  "desktop.ecalc_dim_resistance": "Weerstand",
  "desktop.ecalc_dim_voltage": "Spanning",
  "desktop.ecalc_empty_favorites": "Nog geen favorieten. Markeer een rekenmachine als favoriet om deze hier terug te vinden.",
  "desktop.ecalc_empty_recent": "Nog niets geopend. Onlangs gebruikte rekenmachines verschijnen hier.",
  "desktop.ecalc_error_nonfinite": "Deze waarden geven geen eindig resultaat.",
  "desktop.ecalc_error_parse": "Deze waarde kan niet worden gelezen. Voorbeelden: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Deze waarde moet groter zijn dan nul.",
  "desktop.ecalc_error_undetermined": "Voer meer waarden in om dit te berekenen.",
  "desktop.ecalc_eseries_nearest": "Dichtstbijzijnde {series}-waarde",
  "desktop.ecalc_eseries_series": "E-reeks",
  "desktop.ecalc_explanation": "Uitleg",
  "desktop.ecalc_favorite_add": "Aan favorieten toevoegen",
  "desktop.ecalc_favorite_remove": "Uit favorieten verwijderen",
  "desktop.ecalc_favorites": "Favorieten",
  "desktop.ecalc_inputs": "Invoer",
  "desktop.ecalc_load_failed": "De ECalc-catalogus kon niet worden geladen.",
  "desktop.ecalc_lock": "Waarde vastzetten",
  "desktop.ecalc_no_results": "Geen resultaten gevonden.",
  "desktop.ecalc_recent": "Recent",
  "desktop.ecalc_reset": "Standaardwaarden herstellen",
  "desktop.ecalc_retry": "Opnieuw proberen",
  "desktop.ecalc_search_placeholder": "Zoeken op naam of trefwoord…",
  "desktop.ecalc_settings": "Instellingen",
  "desktop.ecalc_settings_digits": "Significante cijfers",
  "desktop.ecalc_settings_symbols": "Schemasymbolen",
  "desktop.ecalc_source": "Bron",
  "desktop.ecalc_symbols_ansi": "ANSI (zigzagweerstand)",
  "desktop.ecalc_symbols_iec": "IEC (rechthoekige weerstand)",
  "desktop.ecalc_unit_auto": "Automatisch",
  "desktop.ecalc_unlock": "Waarde losmaken",
  "desktop.ecalc_warnings": "Waarschuwingen",
```

**no** (`ui/lang/desktop/no.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Slett permanent",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Ferdig",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Tilbake",
  "desktop.ecalc_computed": "Beregnet",
  "desktop.ecalc_copied": "Kopiert",
  "desktop.ecalc_copy_results": "Kopier resultatene",
  "desktop.ecalc_derived": "Avledede verdier",
  "desktop.ecalc_dim_charge": "Ladning",
  "desktop.ecalc_dim_current": "Strøm",
  "desktop.ecalc_dim_energy": "Energi",
  "desktop.ecalc_dim_gain": "Forsterkning",
  "desktop.ecalc_dim_number": "Tall",
  "desktop.ecalc_dim_power": "Effekt",
  "desktop.ecalc_dim_resistance": "Motstand",
  "desktop.ecalc_dim_voltage": "Spenning",
  "desktop.ecalc_empty_favorites": "Ingen favoritter ennå. Merk en kalkulator som favoritt for å finne den her.",
  "desktop.ecalc_empty_recent": "Ingenting åpnet ennå. Nylig brukte kalkulatorer vises her.",
  "desktop.ecalc_error_nonfinite": "Disse verdiene gir ikke noe endelig resultat.",
  "desktop.ecalc_error_parse": "Verdien kunne ikke leses. Eksempler: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Verdien må være større enn null.",
  "desktop.ecalc_error_undetermined": "Skriv inn flere verdier for å beregne dette.",
  "desktop.ecalc_eseries_nearest": "Nærmeste {series}-verdi",
  "desktop.ecalc_eseries_series": "E-serie",
  "desktop.ecalc_explanation": "Forklaring",
  "desktop.ecalc_favorite_add": "Legg til i favoritter",
  "desktop.ecalc_favorite_remove": "Fjern fra favoritter",
  "desktop.ecalc_favorites": "Favoritter",
  "desktop.ecalc_inputs": "Inndata",
  "desktop.ecalc_load_failed": "ECalc-katalogen kunne ikke lastes.",
  "desktop.ecalc_lock": "Lås verdien",
  "desktop.ecalc_no_results": "Ingen treff.",
  "desktop.ecalc_recent": "Nylig brukt",
  "desktop.ecalc_reset": "Tilbakestill til standardverdier",
  "desktop.ecalc_retry": "Prøv igjen",
  "desktop.ecalc_search_placeholder": "Søk etter navn eller nøkkelord…",
  "desktop.ecalc_settings": "Innstillinger",
  "desktop.ecalc_settings_digits": "Gjeldende siffer",
  "desktop.ecalc_settings_symbols": "Kretssymboler",
  "desktop.ecalc_source": "Kilde",
  "desktop.ecalc_symbols_ansi": "ANSI (sikksakkmotstand)",
  "desktop.ecalc_symbols_iec": "IEC (rektangulær motstand)",
  "desktop.ecalc_unit_auto": "Automatisk",
  "desktop.ecalc_unlock": "Lås opp verdien",
  "desktop.ecalc_warnings": "Advarsler",
```

**pl** (`ui/lang/desktop/pl.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Usuń trwale",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Gotowe",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Wstecz",
  "desktop.ecalc_computed": "Obliczone",
  "desktop.ecalc_copied": "Skopiowano",
  "desktop.ecalc_copy_results": "Kopiuj wyniki",
  "desktop.ecalc_derived": "Wartości pochodne",
  "desktop.ecalc_dim_charge": "Ładunek",
  "desktop.ecalc_dim_current": "Prąd",
  "desktop.ecalc_dim_energy": "Energia",
  "desktop.ecalc_dim_gain": "Wzmocnienie",
  "desktop.ecalc_dim_number": "Liczba",
  "desktop.ecalc_dim_power": "Moc",
  "desktop.ecalc_dim_resistance": "Rezystancja",
  "desktop.ecalc_dim_voltage": "Napięcie",
  "desktop.ecalc_empty_favorites": "Brak ulubionych. Oznacz kalkulator jako ulubiony, aby znaleźć go tutaj.",
  "desktop.ecalc_empty_recent": "Nic jeszcze nie otwarto. Ostatnio używane kalkulatory pojawią się tutaj.",
  "desktop.ecalc_error_nonfinite": "Dla tych wartości nie ma skończonego wyniku.",
  "desktop.ecalc_error_parse": "Nie można odczytać tej wartości. Przykłady: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Ta wartość musi być większa od zera.",
  "desktop.ecalc_error_undetermined": "Wpisz więcej wartości, aby to obliczyć.",
  "desktop.ecalc_eseries_nearest": "Najbliższa wartość z szeregu {series}",
  "desktop.ecalc_eseries_series": "Szereg E",
  "desktop.ecalc_explanation": "Objaśnienie",
  "desktop.ecalc_favorite_add": "Dodaj do ulubionych",
  "desktop.ecalc_favorite_remove": "Usuń z ulubionych",
  "desktop.ecalc_favorites": "Ulubione",
  "desktop.ecalc_inputs": "Dane wejściowe",
  "desktop.ecalc_load_failed": "Nie udało się wczytać katalogu ECalc.",
  "desktop.ecalc_lock": "Zablokuj wartość",
  "desktop.ecalc_no_results": "Nic nie znaleziono.",
  "desktop.ecalc_recent": "Ostatnie",
  "desktop.ecalc_reset": "Przywróć wartości domyślne",
  "desktop.ecalc_retry": "Spróbuj ponownie",
  "desktop.ecalc_search_placeholder": "Szukaj według nazwy lub słowa kluczowego…",
  "desktop.ecalc_settings": "Ustawienia",
  "desktop.ecalc_settings_digits": "Cyfry znaczące",
  "desktop.ecalc_settings_symbols": "Symbole schematów",
  "desktop.ecalc_source": "Źródło",
  "desktop.ecalc_symbols_ansi": "ANSI (rezystor zygzakowaty)",
  "desktop.ecalc_symbols_iec": "IEC (rezystor prostokątny)",
  "desktop.ecalc_unit_auto": "Automatycznie",
  "desktop.ecalc_unlock": "Odblokuj wartość",
  "desktop.ecalc_warnings": "Ostrzeżenia",
```

**pt** (`ui/lang/desktop/pt.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Eliminar permanentemente",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Concluído",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Voltar",
  "desktop.ecalc_computed": "Calculado",
  "desktop.ecalc_copied": "Copiado",
  "desktop.ecalc_copy_results": "Copiar resultados",
  "desktop.ecalc_derived": "Valores derivados",
  "desktop.ecalc_dim_charge": "Carga",
  "desktop.ecalc_dim_current": "Corrente",
  "desktop.ecalc_dim_energy": "Energia",
  "desktop.ecalc_dim_gain": "Ganho",
  "desktop.ecalc_dim_number": "Número",
  "desktop.ecalc_dim_power": "Potência",
  "desktop.ecalc_dim_resistance": "Resistência",
  "desktop.ecalc_dim_voltage": "Tensão",
  "desktop.ecalc_empty_favorites": "Ainda não há favoritos. Marque uma calculadora como favorita para encontrá-la aqui.",
  "desktop.ecalc_empty_recent": "Nada aberto ainda. As calculadoras usadas recentemente aparecem aqui.",
  "desktop.ecalc_error_nonfinite": "Esses valores não dão um resultado finito.",
  "desktop.ecalc_error_parse": "Não foi possível ler este valor. Exemplos: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Este valor deve ser maior que zero.",
  "desktop.ecalc_error_undetermined": "Informe mais valores para calcular isto.",
  "desktop.ecalc_eseries_nearest": "Valor mais próximo da série {series}",
  "desktop.ecalc_eseries_series": "Série E",
  "desktop.ecalc_explanation": "Explicação",
  "desktop.ecalc_favorite_add": "Adicionar aos favoritos",
  "desktop.ecalc_favorite_remove": "Remover dos favoritos",
  "desktop.ecalc_favorites": "Favoritos",
  "desktop.ecalc_inputs": "Entradas",
  "desktop.ecalc_load_failed": "Não foi possível carregar o catálogo do ECalc.",
  "desktop.ecalc_lock": "Travar valor",
  "desktop.ecalc_no_results": "Nenhum resultado.",
  "desktop.ecalc_recent": "Recentes",
  "desktop.ecalc_reset": "Restaurar padrões",
  "desktop.ecalc_retry": "Tentar novamente",
  "desktop.ecalc_search_placeholder": "Pesquisar por nome ou palavra-chave…",
  "desktop.ecalc_settings": "Configurações",
  "desktop.ecalc_settings_digits": "Algarismos significativos",
  "desktop.ecalc_settings_symbols": "Símbolos de esquemas",
  "desktop.ecalc_source": "Fonte",
  "desktop.ecalc_symbols_ansi": "ANSI (resistor em zigue-zague)",
  "desktop.ecalc_symbols_iec": "IEC (resistor retangular)",
  "desktop.ecalc_unit_auto": "Automático",
  "desktop.ecalc_unlock": "Destravar valor",
  "desktop.ecalc_warnings": "Avisos",
```

**sv** (`ui/lang/desktop/sv.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "Radera permanent",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "Klart",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "Tillbaka",
  "desktop.ecalc_computed": "Beräknat",
  "desktop.ecalc_copied": "Kopierat",
  "desktop.ecalc_copy_results": "Kopiera resultat",
  "desktop.ecalc_derived": "Härledda värden",
  "desktop.ecalc_dim_charge": "Laddning",
  "desktop.ecalc_dim_current": "Ström",
  "desktop.ecalc_dim_energy": "Energi",
  "desktop.ecalc_dim_gain": "Förstärkning",
  "desktop.ecalc_dim_number": "Tal",
  "desktop.ecalc_dim_power": "Effekt",
  "desktop.ecalc_dim_resistance": "Resistans",
  "desktop.ecalc_dim_voltage": "Spänning",
  "desktop.ecalc_empty_favorites": "Inga favoriter än. Markera en kalkylator som favorit för att hitta den här.",
  "desktop.ecalc_empty_recent": "Inget har öppnats än. Nyligen använda kalkylatorer visas här.",
  "desktop.ecalc_error_nonfinite": "De här värdena ger inget ändligt resultat.",
  "desktop.ecalc_error_parse": "Värdet kunde inte läsas. Exempel: 4k7, 10 mA, 1e-6.",
  "desktop.ecalc_error_range": "Värdet måste vara större än noll.",
  "desktop.ecalc_error_undetermined": "Ange fler värden för att beräkna detta.",
  "desktop.ecalc_eseries_nearest": "Närmaste {series}-värde",
  "desktop.ecalc_eseries_series": "E-serie",
  "desktop.ecalc_explanation": "Förklaring",
  "desktop.ecalc_favorite_add": "Lägg till i favoriter",
  "desktop.ecalc_favorite_remove": "Ta bort från favoriter",
  "desktop.ecalc_favorites": "Favoriter",
  "desktop.ecalc_inputs": "Indata",
  "desktop.ecalc_load_failed": "ECalc-katalogen kunde inte läsas in.",
  "desktop.ecalc_lock": "Lås värdet",
  "desktop.ecalc_no_results": "Inga träffar.",
  "desktop.ecalc_recent": "Senaste",
  "desktop.ecalc_reset": "Återställ till standardvärden",
  "desktop.ecalc_retry": "Försök igen",
  "desktop.ecalc_search_placeholder": "Sök efter namn eller nyckelord…",
  "desktop.ecalc_settings": "Inställningar",
  "desktop.ecalc_settings_digits": "Signifikanta siffror",
  "desktop.ecalc_settings_symbols": "Kopplingssymboler",
  "desktop.ecalc_source": "Källa",
  "desktop.ecalc_symbols_ansi": "ANSI (sicksackmotstånd)",
  "desktop.ecalc_symbols_iec": "IEC (rektangulärt motstånd)",
  "desktop.ecalc_unit_auto": "Automatiskt",
  "desktop.ecalc_unlock": "Lås upp värdet",
  "desktop.ecalc_warnings": "Varningar",
```

**zh** (`ui/lang/desktop/zh.json`)

Anchor 1 (line 750 at the base commit), the existing line:

```json
  "desktop.app_delete_permanent": "永久删除",
```

Insert directly after it this line:

```json
  "desktop.app_ecalc": "ECalc",
```

Anchor 2 (line 1127 at the base commit), the existing line:

```json
  "desktop.done": "完成",
```

Insert directly after it these 42 lines, in this order:

```json
  "desktop.ecalc_back": "返回",
  "desktop.ecalc_computed": "计算值",
  "desktop.ecalc_copied": "已复制",
  "desktop.ecalc_copy_results": "复制结果",
  "desktop.ecalc_derived": "派生值",
  "desktop.ecalc_dim_charge": "电荷",
  "desktop.ecalc_dim_current": "电流",
  "desktop.ecalc_dim_energy": "能量",
  "desktop.ecalc_dim_gain": "增益",
  "desktop.ecalc_dim_number": "数值",
  "desktop.ecalc_dim_power": "功率",
  "desktop.ecalc_dim_resistance": "电阻",
  "desktop.ecalc_dim_voltage": "电压",
  "desktop.ecalc_empty_favorites": "暂无收藏。将计算器标记为收藏后，可在此处找到。",
  "desktop.ecalc_empty_recent": "尚未打开任何内容。最近使用的计算器会显示在这里。",
  "desktop.ecalc_error_nonfinite": "这些数值无法得出有限的结果。",
  "desktop.ecalc_error_parse": "无法识别此数值。示例：4k7、10 mA、1e-6。",
  "desktop.ecalc_error_range": "此值必须大于零。",
  "desktop.ecalc_error_undetermined": "请再输入更多数值以计算此项。",
  "desktop.ecalc_eseries_nearest": "最接近的 {series} 标称值",
  "desktop.ecalc_eseries_series": "E 系列",
  "desktop.ecalc_explanation": "说明",
  "desktop.ecalc_favorite_add": "添加到收藏",
  "desktop.ecalc_favorite_remove": "从收藏中移除",
  "desktop.ecalc_favorites": "收藏",
  "desktop.ecalc_inputs": "输入",
  "desktop.ecalc_load_failed": "无法加载 ECalc 目录。",
  "desktop.ecalc_lock": "锁定此值",
  "desktop.ecalc_no_results": "未找到匹配项。",
  "desktop.ecalc_recent": "最近",
  "desktop.ecalc_reset": "恢复默认值",
  "desktop.ecalc_retry": "重试",
  "desktop.ecalc_search_placeholder": "按名称或关键词搜索…",
  "desktop.ecalc_settings": "设置",
  "desktop.ecalc_settings_digits": "有效数字",
  "desktop.ecalc_settings_symbols": "电路图符号",
  "desktop.ecalc_source": "来源",
  "desktop.ecalc_symbols_ansi": "ANSI（锯齿形电阻）",
  "desktop.ecalc_symbols_iec": "IEC（矩形电阻）",
  "desktop.ecalc_unit_auto": "自动",
  "desktop.ecalc_unlock": "解锁此值",
  "desktop.ecalc_warnings": "警告",
```

- [ ] **Step 4: Run the i18n tests and confirm they pass**

Run:

```
go test ./ui -run '(?i)(TestDesktopECalcLocaleKeys|TestDesktopBuiltinAppNames|TestDesktopUsedI18n|TestTranslations_|TestDesktopDynamicI18n)' -count=1
git diff --stat -- ui/lang/desktop
```

Expected: `ok  	aurago/ui`. The diffstat lists 16 files with 43 insertions each and 688 insertions in total, no deletions.

(`TestFrontend_StaticI18nKeysExistInEnglishBundle` and `TestChatFrontend_IntegrationsDrawerRemainsWired` fail at the base commit on unrelated FindIt and chat markers. Do not run them as evidence for this part.)

- [ ] **Step 5: Commit**

```
git add ui/desktop_ecalc_i18n_test.go ui/lang/desktop/cs.json ui/lang/desktop/da.json ui/lang/desktop/de.json ui/lang/desktop/el.json ui/lang/desktop/en.json ui/lang/desktop/es.json ui/lang/desktop/fr.json ui/lang/desktop/hi.json ui/lang/desktop/it.json ui/lang/desktop/ja.json ui/lang/desktop/nl.json ui/lang/desktop/no.json ui/lang/desktop/pl.json ui/lang/desktop/pt.json ui/lang/desktop/sv.json ui/lang/desktop/zh.json
git commit -m "feat(ecalc): add the ECalc strings to all 16 desktop locales

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B4: Desktop app registration and logo

**GitNexus impact:** `BuiltinApps` (`internal/desktop/types.go`), upstream, uid `Function:internal/desktop/types.go:BuiltinApps`: risk **HIGH** (12 impacted, lower bound), direct callers `Service.Bootstrap`, `DeleteApp`, `findApp` (`internal/desktop`) and `disabledDesktopBootstrap` (`internal/server/desktop_handlers.go`), indirect `handleDesktopWS` and `handleDesktopBootstrap`; affected process `run`. HIGH is expected for a function this central, and the change is purely additive: one more literal in the returned slice, no signature or behaviour change, no other entry touched. The direct callers in `internal/desktop` are covered by `go test ./internal/desktop`, and the server side (`disabledDesktopBootstrap`, `handleDesktopBootstrap`) by the bootstrap tests run in Step 5.

**Icon decision (see deviation 1):** `Icon: "ecalc"` is not in `desktopPreferredIconNames`, nor in any of the seven icon-theme manifests (Papirus, WhiteSur, Windows XP, Tela, Candy, Fluent, Buuf), so `themeBackedAppIconKey` finds no themed art and `iconForApp` falls through to `appLogoIconKey`, which renders `Metadata["logo_path"]`. The existing icon tests (`TestDesktopPapirusManifestMatchesBackendIconCatalog` and the additional-icon tests) are therefore untouched and stay green.

**Files:**
- Create: `internal/desktop/ecalc_app_test.go`
- Create: `ui/desktop_ecalc_registration_test.go`
- Create: `ui/img/desktop-icons/ecalc.svg`
- Modify: `internal/desktop/types.go` — func `BuiltinApps`, between the `cyd-creator` and `homepage-studio` entries (lines 847-848 at the base commit)
- Modify: `internal/desktop/AGENTS.md` — "Local Contracts", after the FindIt bullet (line 13 at the base commit)

- [ ] **Step 1: Write the failing tests**

Create `internal/desktop/ecalc_app_test.go`:

```go
package desktop

import "testing"

// ECalc is a plain builtin developer app: no capability gate, it is always listed. Its icon is
// app-owned artwork (logo_path), like Synth Studio's, so it needs no entry in the themed icon sets.
func TestBuiltinECalcAppIsAnAlwaysVisibleDeveloperApp(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "ecalc")
	if app.Entry != "builtin://ecalc" || app.Name != "ECalc" || app.Category != "dev" || app.Runtime != BuiltinRuntime {
		t.Fatalf("ecalc manifest = %+v", app)
	}
	if app.Icon != "ecalc" || app.Metadata["logo_path"] != "/img/desktop-icons/ecalc.svg" {
		t.Fatalf("ecalc icon = %q, metadata = %v; want the app-owned logo /img/desktop-icons/ecalc.svg", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 0 {
		t.Fatalf("ecalc must not require a capability, requires = %v", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("ecalc must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 1 {
		t.Fatalf("ecalc must show without any capability: %+v", got)
	}
}
```

Create `ui/desktop_ecalc_registration_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopECalcBackendRegistration(t *testing.T) {
	t.Parallel()

	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "ecalc" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("ecalc is not a builtin app")
	}
	if app.Name != "ECalc" || app.Category != "dev" || app.Entry != "builtin://ecalc" || len(app.Requires) != 0 || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	logo := app.Metadata["logo_path"]
	if logo != "/img/desktop-icons/ecalc.svg" {
		t.Fatalf("logo_path = %q", logo)
	}
	svg, err := Content.ReadFile(strings.TrimPrefix(logo, "/"))
	if err != nil {
		t.Fatalf("the ECalc logo is not embedded: %v", err)
	}
	text := string(svg)
	if !strings.HasPrefix(text, "<svg ") || strings.Contains(text, "href") || strings.Contains(text, "<script") {
		t.Fatalf("the ECalc logo must be a self-contained SVG: %.120s", text)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run:

```
go test ./internal/desktop -run 'TestBuiltinECalcApp' -count=1
go test ./ui -run 'TestDesktopECalcBackendRegistration' -count=1
```

Expected: both FAIL. The first prints `app "ecalc" not found in [...]` (a long list follows on the same line), the second `ecalc is not a builtin app`.

- [ ] **Step 3: Create the logo**

Create `ui/img/desktop-icons/ecalc.svg`. It is 128 x 128, self-contained (no fonts, no external references), and draws a teal rounded tile with a white resistor that has amber, red and teal bands, a logarithmic scale below it and a small yellow sparkle:

```xml
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" role="img" aria-label="ECalc">
  <defs>
    <linearGradient id="g" x1="18" y1="15" x2="108" y2="115" gradientUnits="userSpaceOnUse">
      <stop stop-color="#14b8a6"/>
      <stop offset="1" stop-color="#0e5a78"/>
    </linearGradient>
  </defs>
  <rect x="8" y="8" width="112" height="112" rx="27" fill="url(#g)"/>
  <path d="M19 58h21M88 58h21" stroke="#ffffff" stroke-width="6" stroke-linecap="round"/>
  <rect x="36" y="43" width="56" height="30" rx="13" fill="#fff7e6"/>
  <rect x="49" y="43" width="7" height="30" fill="#f59e0b"/>
  <rect x="61" y="43" width="7" height="30" fill="#e11d48"/>
  <rect x="73" y="43" width="7" height="30" fill="#0e5a78"/>
  <path d="M28 94v12M49.7 94v12M78.3 94v12M100 94v12" stroke="#ffffff" stroke-width="3.5" stroke-linecap="round"/>
  <path d="M40.7 94v7M62.4 94v7M71.4 94v7M84 94v7M88.9 94v7M93 94v7M96.7 94v7" stroke="#d6f7f2" stroke-width="3" stroke-linecap="round"/>
  <path d="m98 20 2.8 7.4 7.4 2.8-7.4 2.8L98 40.4l-2.8-7.4-7.4-2.8 7.4-2.8L98 20Z" fill="#fff4a8"/>
</svg>
```

- [ ] **Step 4: Register the app**

In `internal/desktop/types.go` (func BuiltinApps, between the "cyd-creator" entry (line 847) and the "homepage-studio" entry (line 848) at the base commit), replace this exact text:

```go
Requires: []string{"cyd_creator"}, Metadata: map[string]string{"open_maximized": "true"}},
		{ID: "homepage-studio",
```

with:

```go
Requires: []string{"cyd_creator"}, Metadata: map[string]string{"open_maximized": "true"}},
		{ID: "ecalc", Name: "ECalc", Version: "1.0.0", Icon: "ecalc", Entry: "builtin://ecalc", Runtime: BuiltinRuntime, Category: "dev", Description: "Electronics formula collection: interactive calculators with schematics, live results and standard resistor values.", Metadata: map[string]string{"logo_path": "/img/desktop-icons/ecalc.svg"}},
		{ID: "homepage-studio",
```

Indentation inside the Go blocks is tabs.

- [ ] **Step 5: Run the tests and confirm they pass**

Run:

```
go test ./internal/desktop -count=1
go test ./ui -run '(?i)(TestDesktopECalc|TestDesktopBuiltinAppNames|TestDesktopPapirus|TestDesktopAdditionalIcon|TestDesktopRadioApps|TestShippedWebAssets|TestDesktopThemeRegistry)' -count=1
go test ./internal/server -run '(?i)(Bootstrap|DesktopApps|DesktopStatus)' -count=1
gofmt -l internal/desktop/types.go internal/desktop/ecalc_app_test.go ui/desktop_ecalc_registration_test.go ui/desktop_ecalc_i18n_test.go
go vet ./internal/desktop ./ui
```

Expected: the three `go test` runs print `ok`, `gofmt -l` prints nothing, `go vet` prints nothing. `go test ./internal/desktop` takes about 40 seconds.

- [ ] **Step 6: Document the app**

In `internal/desktop/AGENTS.md` ("Local Contracts", directly after the FindIt bullet (line 13 at the base commit)), replace this exact text:

```markdown
its large component atlas never changes the global Desktop sprite grid.

- Desktop cloud transfers use
```

with:

```markdown
its large component atlas never changes the global Desktop sprite grid.

- ECalc (`ecalc`, category `dev`) is a built-in electronics formula app with no capability gate. Its icon is app-owned artwork, `logo_path` `/img/desktop-icons/ecalc.svg`, and its `Icon` value `ecalc` is deliberately absent from `desktopPreferredIconNames` and every icon theme (as for Synth Studio), so every theme shows the logo; adding a themed `ecalc` icon would replace it. Its catalog route is `GET /api/ecalc/catalog` (`../server/AGENTS.md`). Verify `TestBuiltinECalcApp*` and `go test ./ui -run TestDesktopECalc`.

- Desktop cloud transfers use
```

- [ ] **Step 7: Commit**

```
git add internal/desktop/types.go internal/desktop/ecalc_app_test.go internal/desktop/AGENTS.md ui/desktop_ecalc_registration_test.go ui/img/desktop-icons/ecalc.svg
git commit -m "feat(ecalc): register the ECalc desktop app with its own logo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B5: Manual chapter 32 (skeleton) and index rows

Documentation only. The chapter describes exactly what phase 1 delivers (11 basics calculators, layout C, entering values, E series, settings, the browser-side storage key) and says that tables, codes, pinouts, more calculators and the agent tool follow in later phases. The previous-chapter footer matches `31-pixel.md`; chapter 31 has no forward link, so it needs no edit. The calculator names in the chapter's table are the English and German names the catalog should carry (`name.en` and `name.de` in `internal/ecalc/catalog/calculators/basics.yaml`). If Part A's YAML names differ, adjust the table in this task to the YAML.

No test in the repository reads chapter 32 or the new index rows (the doc tests in `internal/audit` read chapters 13 and 21 and the Local Wikipedia chapters). The check here is that the unrelated doc tests still pass and that every link in the new files resolves.

**Files:**
- Create: `documentation/manuals/en/32-ecalc.md`
- Create: `documentation/manuals/de/32-ecalc.md`
- Modify: `documentation/manuals/README.md` — chapter table (line 74) and the closing paragraph (line 83 at the base commit)
- Modify: `documentation/manuals/en/README.md` — "Part 6", after line 132 at the base commit
- Modify: `documentation/manuals/de/README.md` — "Teil 6", after line 132 at the base commit
- Modify: `documentation/manuals/en/04-webui.md` — "Included Apps" table, after line 265 at the base commit
- Modify: `documentation/manuals/de/04-webui.md` — "Enthaltene Apps" table, after line 265 at the base commit

- [ ] **Step 1: Create the English chapter**

Create `documentation/manuals/en/32-ecalc.md`:

```markdown
# Chapter 32: ECalc – electronics formulas in the Virtual Desktop

ECalc is a formula collection for electronics in the Virtual Desktop. A calculator page shows the circuit, the formula and the
results side by side, and it recalculates while you type. You enter the values you know and ECalc works out the rest. For
resistors it also suggests the nearest standard value from an E series.

This is the first version. It has 11 calculators in the **Basics** category. Tables, component codes, pinouts and an agent tool
follow in later phases (see [What Comes Next](#what-comes-next)). ECalc runs in your browser. It needs no provider and has no
switch in **Config**.

## Table of Contents
1. [Opening ECalc](#opening-ecalc)
2. [Finding a Calculator](#finding-a-calculator)
3. [The Calculator Page](#the-calculator-page)
4. [Entering Values](#entering-values)
5. [Standard Values (E Series)](#standard-values-e-series)
6. [Settings](#settings)
7. [The Calculators](#the-calculators)
8. [What Comes Next](#what-comes-next)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening ECalc

Open **ECalc** from the start menu, in the category **Development**. The window opens at about 1100 × 760 pixels and you can
resize it. ECalc reopens on the calculator you used last.

The window has three columns: a **category rail** on the left, a **list** of calculators in the middle and the **calculator
page** on the right. In a narrow window (below about 720 pixels) the three become one stack. Choose a category, then a
calculator, and use **Back** to return to the previous level.

## Finding a Calculator

- **Category rail:** **Favorites** and **Recent** come first, followed by the calculator categories: Basics, Passive, Filters,
  Semiconductors, Power supply, Op-amps, Timers, RF, Digital and Wiring & PCB. In this version only **Basics** holds
  calculators.
- **Search:** type in the search box at the top to find a calculator by its name in your language, by its English name or by a
  keyword such as "divider".
- **Favorites:** the favorite button on a calculator page adds it to **Favorites**. Click it again to remove it.
- **Recent:** ECalc remembers the last 12 calculators you opened.

## The Calculator Page

A calculator page has these parts:

- **Title bar:** the name of the calculator and the buttons **Add to favorites**, **Copy results** and **Reset to defaults**.
- **Schematic:** the circuit, with an input field next to each component it belongs to. A calculator without a schematic shows
  its fields in a plain list.
- **Formula:** the formula in typeset form, and below it a live line with your values filled in, for example
  *V_out = 12 V · 2.2 kΩ / (4.7 kΩ + 2.2 kΩ) = 3.83 V*.
- **Nearest standard values:** the E-series suggestions for the resistors (see [Standard Values (E Series)](#standard-values-e-series)).
- **Derived values:** further quantities that follow from the result, for example the current through a divider and its power.
- **Warnings:** a note when the values do not make sense, for example an output voltage that is not below the input voltage.
- **Explanation:** a collapsible text about the calculator, with its **Source**: the standard, datasheet or textbook behind the
  formula.

**Copy results** puts the values of the page on the clipboard. **Reset to defaults** restores the starting values of the
calculator.

## Entering Values

Every calculator needs a fixed number of known values. Ohm's law, for example, needs two of the four quantities voltage,
current, resistance and power. The fields you fill in are **inputs**. ECalc fills the other fields with **computed** values and
tints them.

- **Change what is known:** type into a computed field. It becomes an input, and the input you edited longest ago becomes
  computed. This way you can work from whichever value you have at hand.
- **Lock a value:** the lock icon on a field pins it. A locked field stays an input until you unlock it, even when you edit
  other fields.
- **Number formats:** you can type `4k7` (4.7 kΩ), `2u2` (2.2 µ), `10 mA`, `1e-6` or `0,5`. A decimal comma and a decimal point
  both work. A unit after the number is optional.
- **Units:** the dropdown next to a field chooses the unit. **Auto** picks a suitable prefix, such as mA or kΩ. Some fields offer
  special units, such as dBm, dBV and dBu for levels, or mAh, Ah and Wh for charge and energy.
- **Lists:** the series and parallel calculator takes a list such as `4k7, 10k, 2k2`. Separate the entries with commas,
  semicolons or spaces. Lists use a decimal point only.
- **Display:** the results show a decimal comma in German, Czech, Danish, Greek, Spanish, French, Italian, Dutch, Norwegian,
  Polish, Portuguese and Swedish, and a decimal point in the other languages.

When something is wrong, the field shows a message instead of a number:

| Message | Meaning |
|---------|---------|
| "Enter more values to calculate this." | The known values do not determine this field yet. |
| "These values give no finite result." | The calculation divides by zero or leaves the range of numbers. |
| "This value must be greater than zero." | Resistance, for example, cannot be zero or negative. |
| "Could not read this value. Examples: 4k7, 10 mA, 1e-6." | The text is not a number with an optional prefix and unit. Your previous values stay as they were. |

## Standard Values (E Series)

Resistors come in standard values. Every resistor field of a calculator shows **Nearest E24 value**: the closest standard
value and how far it is from the calculated one, in percent. For example, 4.7 kΩ is in the E24 series, and a calculated 4.6 kΩ
would show 4.7 kΩ with a deviation of about +2.2 %.

Choose another series (E6 to E192) in the **E series** selector. The default is E24. Series with more values, such as E96, give
a smaller deviation. ECalc remembers your choice.

## Settings

The settings menu has two options:

- **Significant digits:** how many digits the results show, from 3 to 6. The default is 4.
- **Schematic symbols:** **IEC** draws the resistor as a rectangle (the default). **ANSI** draws the zigzag resistor.

ECalc keeps its settings, your favorites, the recent list, the units you chose and the last values of each calculator in the
browser, under the key `aurago.ecalc.v1`. They stay in that browser. Nothing is stored on the AuraGo server, and clearing the
browser data resets them. ECalc works without browser storage too, it just forgets everything when you close the window.

## The Calculators

All calculators of this version are in the **Basics** category.

| Calculator | What it does |
|------------|--------------|
| Ohm's law and power | Voltage, current, resistance and power. Enter any two and get the other two. |
| Voltage divider | The output voltage of a divider of two resistors. It also finds a resistor or the input voltage for a target output, and shows current and power. |
| Loaded voltage divider | The same divider with a load resistor on the output. It shows the unloaded output for comparison and warns when the load makes the target impossible. |
| Current divider | How a current splits between two parallel resistors. |
| Series and parallel resistors | The total resistance of a list of resistors, in series and in parallel. |
| Decibel ratio | Converts between a ratio and decibels. Choose voltage (20 log) or power (10 log). |
| dB levels | Voltage, resistance and power with the levels in dBm, dBV and dBu. |
| Wheatstone bridge | The bridge voltage from the supply and four resistors, or the resistor that balances the bridge. |
| Star-delta transform | Converts three delta resistors to the equivalent star, and back. |
| RMS, peak and average | Converts between the RMS, peak, peak-to-peak and average value of a sine, square or triangle wave. |
| Charge and energy | Converts between charge (for example the mAh of a battery), voltage and energy (Wh or J). |

## What Comes Next

The catalog grows in later phases:

- **More calculators** in the categories Passive, Filters, Semiconductors, Power supply, Op-amps, Timers, RF, Digital and
  Wiring & PCB.
- **Reference tables**, for example the current capacity of cables and PCB traces.
- **Component codes**, for example the resistor colour code and SMD codes.
- **Pinouts** of connectors and ICs.
- **The `ecalc` agent tool.** The AuraGo agent will use the same formulas to calculate values, and it will be able to open
  ECalc on a page with values already filled in.

This chapter grows with them.

## Privacy and Safety

- **On your device.** The calculations run in your browser. ECalc loads its catalog from your AuraGo server and sends none of
  your values anywhere. It loads no external fonts, libraries or services.
- **Read-only.** ECalc only reads. A read-only Virtual Desktop does not change how it works.
- **Check what matters.** Each calculator names its source under **Explanation**. The results are only as good as the formula
  and your inputs. Check a design that matters against the datasheets of your parts. Mains voltage is dangerous, and ECalc is
  no substitute for qualified electrical work.

## When Something Goes Wrong

| Problem | What helps |
|---------|------------|
| "The ECalc catalog could not be loaded." | Click **Try again**. If it keeps failing, check that AuraGo is running and that you are still signed in, then reload the Virtual Desktop. |
| A field shows a message instead of a value | See the table in [Entering Values](#entering-values). |
| The favorites or the last values are gone | They live in the browser. A different browser, a private window or cleared browser data starts with empty ones. |
| The E-series value is far from the calculated one | Choose a series with more values, such as E96 or E192. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 31: Pixel](./31-pixel.md)
```

- [ ] **Step 2: Create the German chapter**

Create `documentation/manuals/de/32-ecalc.md`:

```markdown
# Kapitel 32: ECalc – Elektronik-Formeln im Virtual Desktop

ECalc ist eine Formelsammlung für Elektronik im Virtual Desktop. Eine Rechenseite zeigt die Schaltung, die Formel und die
Ergebnisse nebeneinander und rechnet schon während du tippst. Du gibst die Werte ein, die du kennst, und ECalc berechnet den
Rest. Bei Widerständen schlägt es außerdem den nächsten Normwert aus einer E-Reihe vor.

Das ist die erste Version. Sie enthält 11 Rechner in der Kategorie **Grundlagen**. Tabellen, Bauteilcodes, Pinbelegungen und ein
Agent-Tool folgen in späteren Phasen (siehe [Wie es weitergeht](#wie-es-weitergeht)). ECalc läuft in deinem Browser. Es braucht
keinen Provider und hat keinen Schalter unter **Konfiguration**.

## Inhalt
1. [ECalc öffnen](#ecalc-öffnen)
2. [Einen Rechner finden](#einen-rechner-finden)
3. [Die Rechenseite](#die-rechenseite)
4. [Werte eingeben](#werte-eingeben)
5. [Normwerte (E-Reihen)](#normwerte-e-reihen)
6. [Einstellungen](#einstellungen)
7. [Die Rechner](#die-rechner)
8. [Wie es weitergeht](#wie-es-weitergeht)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## ECalc öffnen

Öffne **ECalc** aus dem Startmenü, in der Kategorie **Entwicklung**. Das Fenster öffnet sich mit etwa 1100 × 760 Pixeln und lässt
sich in der Größe ändern. ECalc öffnet wieder den Rechner, den du zuletzt benutzt hast.

Das Fenster hat drei Spalten: links die **Kategorieleiste**, in der Mitte die **Liste** der Rechner und rechts die
**Rechenseite**. In einem schmalen Fenster (unter etwa 720 Pixeln) wird daraus ein einziger Stapel. Wähle eine Kategorie,
dann einen Rechner, und gehe mit **Zurück** eine Ebene nach oben.

## Einen Rechner finden

- **Kategorieleiste:** Zuerst kommen **Favoriten** und **Zuletzt verwendet**, danach die Rechner-Kategorien: Grundlagen, Passive
  Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer, HF, Digital und Leitungen & PCB. In dieser Version enthält nur
  **Grundlagen** Rechner.
- **Suche:** Tippe oben ins Suchfeld, um einen Rechner über seinen Namen in deiner Sprache, über den englischen Namen oder über
  ein Stichwort wie „Teiler“ zu finden.
- **Favoriten:** Die Favoriten-Schaltfläche auf einer Rechenseite nimmt den Rechner in die **Favoriten** auf. Ein weiterer Klick
  entfernt ihn wieder.
- **Zuletzt verwendet:** ECalc merkt sich die letzten 12 Rechner, die du geöffnet hast.

## Die Rechenseite

Eine Rechenseite besteht aus diesen Teilen:

- **Titelzeile:** der Name des Rechners und die Schaltflächen **Zu Favoriten hinzufügen**, **Ergebnisse kopieren** und **Auf
  Standardwerte zurücksetzen**.
- **Schaltplan:** die Schaltung, mit einem Eingabefeld neben jedem Bauteil, zu dem es gehört. Ein Rechner ohne Schaltplan zeigt
  seine Felder in einer einfachen Liste.
- **Formel:** die gesetzte Formel und darunter eine Live-Zeile mit deinen eingesetzten Werten, zum Beispiel
  *V_out = 12 V · 2,2 kΩ / (4,7 kΩ + 2,2 kΩ) = 3,83 V*.
- **Nächste Normwerte:** die E-Reihen-Vorschläge für die Widerstände (siehe [Normwerte (E-Reihen)](#normwerte-e-reihen)).
- **Abgeleitete Werte:** weitere Größen, die sich aus dem Ergebnis ergeben, zum Beispiel der Strom durch einen Spannungsteiler
  und seine Leistung.
- **Warnungen:** ein Hinweis, wenn die Werte keinen Sinn ergeben, zum Beispiel eine Ausgangsspannung, die nicht unter der
  Eingangsspannung liegt.
- **Erklärung:** ein einklappbarer Text zum Rechner mit der **Quelle**: der Norm, dem Datenblatt oder dem Lehrbuch hinter der
  Formel.

**Ergebnisse kopieren** legt die Werte der Seite in die Zwischenablage. **Auf Standardwerte zurücksetzen** stellt die
Startwerte des Rechners wieder her.

## Werte eingeben

Jeder Rechner braucht eine feste Anzahl bekannter Werte. Das Ohmsche Gesetz zum Beispiel braucht zwei der vier Größen Spannung,
Strom, Widerstand und Leistung. Die Felder, die du ausfüllst, sind **Eingaben**. ECalc füllt die übrigen Felder mit
**berechneten** Werten und färbt sie ein.

- **Ändern, was bekannt ist:** Tippe in ein berechnetes Feld. Es wird zur Eingabe, und die Eingabe, die du am längsten nicht
  mehr angefasst hast, wird berechnet. So kannst du von dem Wert ausgehen, den du gerade zur Hand hast.
- **Wert sperren:** Das Schlosssymbol an einem Feld hält es fest. Ein gesperrtes Feld bleibt eine Eingabe, bis du es wieder
  entsperrst, auch wenn du andere Felder änderst.
- **Zahlenformate:** Du kannst `4k7` (4,7 kΩ), `2u2` (2,2 µ), `10 mA`, `1e-6` oder `0,5` tippen. Dezimalkomma und Dezimalpunkt
  funktionieren beide. Eine Einheit hinter der Zahl ist optional.
- **Einheiten:** Das Auswahlfeld neben einem Feld wählt die Einheit. **Automatisch** wählt ein passendes Präfix, etwa mA oder kΩ.
  Manche Felder bieten besondere Einheiten, etwa dBm, dBV und dBu für Pegel oder mAh, Ah und Wh für Ladung und Energie.
- **Listen:** Der Rechner für Reihen- und Parallelschaltung nimmt eine Liste wie `4k7, 10k, 2k2`. Trenne die Einträge mit
  Kommas, Semikolons oder Leerzeichen. In Listen gilt nur der Dezimalpunkt.
- **Anzeige:** Die Ergebnisse zeigen in Deutsch, Tschechisch, Dänisch, Griechisch, Spanisch, Französisch, Italienisch,
  Niederländisch, Norwegisch, Polnisch, Portugiesisch und Schwedisch ein Dezimalkomma, in den anderen Sprachen einen
  Dezimalpunkt.

Wenn etwas nicht stimmt, zeigt das Feld eine Meldung statt einer Zahl:

| Meldung | Bedeutung |
|---------|-----------|
| „Gib weitere Werte ein, um das zu berechnen.“ | Die bekannten Werte bestimmen dieses Feld noch nicht. |
| „Mit diesen Werten gibt es kein endliches Ergebnis.“ | Die Rechnung teilt durch null oder verlässt den Zahlenbereich. |
| „Dieser Wert muss größer als null sein.“ | Ein Widerstand zum Beispiel kann nicht null oder negativ sein. |
| „Dieser Wert ist nicht lesbar. Beispiele: 4k7, 10 mA, 1e-6.“ | Der Text ist keine Zahl mit optionalem Präfix und optionaler Einheit. Deine bisherigen Werte bleiben unverändert. |

## Normwerte (E-Reihen)

Widerstände gibt es in Normwerten. Jedes Widerstandsfeld eines Rechners zeigt **Nächster E24-Wert**: den nächstliegenden Normwert
und wie weit er vom berechneten Wert abweicht, in Prozent. 4,7 kΩ zum Beispiel gehört zur E24-Reihe, und ein berechneter Wert von
4,6 kΩ würde 4,7 kΩ mit einer Abweichung von etwa +2,2 % zeigen.

Eine andere Reihe (E6 bis E192) wählst du im Auswahlfeld **E-Reihe**. Voreingestellt ist E24. Reihen mit mehr Werten, etwa E96,
ergeben eine kleinere Abweichung. ECalc merkt sich deine Wahl.

## Einstellungen

Das Einstellungsmenü hat zwei Optionen:

- **Signifikante Stellen:** wie viele Stellen die Ergebnisse zeigen, von 3 bis 6. Voreingestellt sind 4.
- **Schaltsymbole:** **IEC** zeichnet den Widerstand als Rechteck (Voreinstellung). **ANSI** zeichnet den Zickzack-Widerstand.

ECalc speichert seine Einstellungen, deine Favoriten, die Liste der zuletzt verwendeten Rechner, die gewählten Einheiten und die
letzten Werte jedes Rechners im Browser unter dem Schlüssel `aurago.ecalc.v1`. Sie bleiben in diesem Browser. Auf dem
AuraGo-Server wird nichts gespeichert, und wer die Browserdaten löscht, setzt sie zurück. ECalc funktioniert auch ohne
Browser-Speicher, vergisst dann aber alles, sobald du das Fenster schließt.

## Die Rechner

Alle Rechner dieser Version gehören zur Kategorie **Grundlagen**.

| Rechner | Was er macht |
|---------|--------------|
| Ohmsches Gesetz und Leistung | Spannung, Strom, Widerstand und Leistung. Gib zwei beliebige Größen ein und erhalte die beiden anderen. |
| Spannungsteiler | Die Ausgangsspannung eines Teilers aus zwei Widerständen. Er findet auch einen Widerstand oder die Eingangsspannung für eine gewünschte Ausgangsspannung und zeigt Strom und Leistung. |
| Belasteter Spannungsteiler | Derselbe Teiler mit einem Lastwiderstand am Ausgang. Er zeigt zum Vergleich die unbelastete Ausgangsspannung und warnt, wenn die Last das Ziel unmöglich macht. |
| Stromteiler | Wie sich ein Strom auf zwei parallele Widerstände verteilt. |
| Reihen- und Parallelschaltung | Der Gesamtwiderstand einer Liste von Widerständen, in Reihe und parallel. |
| Dezibel-Verhältnis | Rechnet zwischen einem Verhältnis und Dezibel um. Wähle Spannung (20 log) oder Leistung (10 log). |
| Pegel in dB | Spannung, Widerstand und Leistung mit den Pegeln in dBm, dBV und dBu. |
| Wheatstone-Brücke | Die Brückenspannung aus der Versorgung und vier Widerständen, oder der Widerstand, der die Brücke abgleicht. |
| Stern-Dreieck-Umwandlung | Wandelt drei Dreieck-Widerstände in den gleichwertigen Stern um und umgekehrt. |
| Effektiv-, Spitzen- und Mittelwert | Rechnet zwischen Effektivwert, Spitzenwert, Spitze-Spitze-Wert und Mittelwert einer Sinus-, Rechteck- oder Dreieckschwingung um. |
| Ladung und Energie | Rechnet zwischen Ladung (zum Beispiel den mAh eines Akkus), Spannung und Energie (Wh oder J) um. |

## Wie es weitergeht

Der Katalog wächst in späteren Phasen:

- **Weitere Rechner** in den Kategorien Passive Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer, HF, Digital und
  Leitungen & PCB.
- **Nachschlagetabellen**, zum Beispiel die Strombelastbarkeit von Kabeln und Leiterbahnen.
- **Bauteilcodes**, zum Beispiel der Widerstandsfarbcode und SMD-Codes.
- **Pinbelegungen** von Steckverbindern und ICs.
- **Das Agent-Tool `ecalc`.** Der AuraGo-Agent soll dieselben Formeln zum Rechnen nutzen und ECalc auf einer Seite mit bereits
  eingetragenen Werten öffnen können.

Dieses Kapitel wächst mit.

## Datenschutz und Sicherheit

- **Auf deinem Gerät.** Die Berechnungen laufen in deinem Browser. ECalc lädt seinen Katalog von deinem AuraGo-Server und
  schickt keinen deiner Werte irgendwohin. Es lädt keine externen Schriften, Bibliotheken oder Dienste.
- **Schreibgeschützt.** ECalc liest nur. Ein schreibgeschützter Virtual Desktop ändert nichts an seiner Funktion.
- **Prüfe, worauf es ankommt.** Jeder Rechner nennt seine Quelle unter **Erklärung**. Die Ergebnisse sind nur so gut wie die
  Formel und deine Eingaben. Prüfe einen wichtigen Entwurf anhand der Datenblätter deiner Bauteile. Netzspannung ist
  gefährlich, und ECalc ersetzt keine Arbeit durch eine Elektrofachkraft.

## Wenn etwas schiefgeht

| Problem | Was hilft |
|---------|-----------|
| „Der ECalc-Katalog konnte nicht geladen werden.“ | Klicke auf **Erneut versuchen**. Falls es weiter scheitert, prüfe, ob AuraGo läuft und du noch angemeldet bist, und lade den Virtual Desktop neu. |
| Ein Feld zeigt eine Meldung statt eines Werts | Siehe die Tabelle unter [Werte eingeben](#werte-eingeben). |
| Favoriten oder letzte Werte sind weg | Sie liegen im Browser. Ein anderer Browser, ein privates Fenster oder gelöschte Browserdaten beginnen mit leeren. |
| Der E-Reihen-Wert weicht stark vom berechneten ab | Wähle eine Reihe mit mehr Werten, etwa E96 oder E192. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 31: Pixel](./31-pixel.md)
```

- [ ] **Step 3: Add the chapter to the indexes**

The handbook hub, chapter table row:

In `documentation/manuals/README.md` (the chapter table, line 74 at the base commit), replace this exact text:

```markdown
| 31 · Pixel | [Pixel](de/31-pixel.md) | [Pixel](en/31-pixel.md) | Layered image editor, filters, generators, AI background removal |
```

with:

```markdown
| 31 · Pixel | [Pixel](de/31-pixel.md) | [Pixel](en/31-pixel.md) | Layered image editor, filters, generators, AI background removal |
| 32 · ECalc | [ECalc](de/32-ecalc.md) | [ECalc](en/32-ecalc.md) | Electronics formula collection: calculators, live results, E series |
```

The handbook hub, closing paragraph (new date and chapter count):

In `documentation/manuals/README.md` (the last paragraph, line 83 at the base commit (date and chapter count)), replace this exact text:

```markdown
**Stand: 22. Oktober 2026.** Both languages cover the same 31 chapters. German uses *du*.
```

with:

```markdown
**Stand: 1. November 2026.** Both languages cover the same 32 chapters. German uses *du*.
```

The English overview, "Part 6":

In `documentation/manuals/en/README.md` ("Part 6 — Studio and Desktop Apps", line 132 at the base commit), replace this exact text:

```markdown
31. [Pixel](31-pixel.md) — Layered image editor with masks, 80 filters, 43 generators, AI background removal and OpenRaster projects
```

with:

```markdown
31. [Pixel](31-pixel.md) — Layered image editor with masks, 80 filters, 43 generators, AI background removal and OpenRaster projects
32. [ECalc](32-ecalc.md) — Electronics formula collection: calculators with schematics, live results and E-series values for resistors
```

The German overview, "Teil 6":

In `documentation/manuals/de/README.md` ("Teil 6 — Studio- und Desktop-Apps", line 132 at the base commit), replace this exact text:

```markdown
31. [Pixel](31-pixel.md) — Ebenen-Bildeditor mit Masken, 80 Filtern, 43 Generatoren, KI-Freistellen und OpenRaster-Projekten
```

with:

```markdown
31. [Pixel](31-pixel.md) — Ebenen-Bildeditor mit Masken, 80 Filtern, 43 Generatoren, KI-Freistellen und OpenRaster-Projekten
32. [ECalc](32-ecalc.md) — Elektronik-Formelsammlung: Rechner mit Schaltplänen, Live-Ergebnissen und E-Reihen-Werten für Widerstände
```

The English Web UI chapter, app table:

In `documentation/manuals/en/04-webui.md` ("Included Apps" table, line 265 at the base commit), replace this exact text:

```markdown
| **Calculator / Settings / System Info / Log Viewer** | Calculator, desktop settings, system diagnostics, and a live log tail |
```

with:

```markdown
| **Calculator / Settings / System Info / Log Viewer** | Calculator, desktop settings, system diagnostics, and a live log tail |
| **ECalc** | Electronics formula collection: interactive calculators with schematics, typeset formulas and live results, plus E-series values for resistors. Runs in the browser and needs no setup ([Chapter 32](32-ecalc.md)) |
```

The German Web UI chapter, app table:

In `documentation/manuals/de/04-webui.md` ("Enthaltene Apps" table, line 265 at the base commit), replace this exact text:

```markdown
| **Calculator / Settings / System Info / Log Viewer** | Rechner, Desktop-Einstellungen, Systemdiagnose und Live-Logtail |
```

with:

```markdown
| **Calculator / Settings / System Info / Log Viewer** | Rechner, Desktop-Einstellungen, Systemdiagnose und Live-Logtail |
| **ECalc** | Elektronik-Formelsammlung: interaktive Rechner mit Schaltplänen, gesetzten Formeln und Live-Ergebnissen, dazu E-Reihen-Werte für Widerstände. Läuft im Browser und braucht keine Einrichtung ([Kapitel 32](32-ecalc.md)) |
```

- [ ] **Step 4: Check the doc tests and the links**

Run:

```
go test ./internal/audit -run '(?i)(TestDashboardDocs|TestLocalWikipediaManual|TestLocalWikipediaGuide|TestLocalWikipediaReadmeMention)' -count=1
git grep -n "32-ecalc" -- documentation/manuals
```

Expected: `ok  	aurago/internal/audit`, and `git grep` lists five lines, one in each of `documentation/manuals/README.md`, `documentation/manuals/de/04-webui.md`, `documentation/manuals/de/README.md`, `documentation/manuals/en/04-webui.md` and `documentation/manuals/en/README.md`.

Then run `git status --short documentation`. Expected: exactly `M documentation/manuals/README.md`, `M documentation/manuals/de/04-webui.md`, `M documentation/manuals/de/README.md`, `M documentation/manuals/en/04-webui.md`, `M documentation/manuals/en/README.md`, `?? documentation/manuals/de/32-ecalc.md` and `?? documentation/manuals/en/32-ecalc.md`.

Link check: every relative link and in-page anchor of the two new chapters and of the three README files must resolve. A quick way is to open the chapters in the editor's markdown preview and click the table of contents; the repository has no link checker for the manuals.

- [ ] **Step 5: Commit**

```
git add documentation/manuals/en/32-ecalc.md documentation/manuals/de/32-ecalc.md documentation/manuals/README.md documentation/manuals/en/README.md documentation/manuals/de/README.md documentation/manuals/en/04-webui.md documentation/manuals/de/04-webui.md
git commit -m "docs(ecalc): manual chapter 32 skeleton and index rows

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B6: Part B verification sweep

No new files and no commit. Run after B1-B5 (and after Part A) to confirm the whole part together.

- [ ] **Step 1: Vet and format**

Run:

```
go vet ./internal/server ./internal/audit ./internal/desktop ./ui
gofmt -l internal/server/ecalc_handlers.go internal/server/ecalc_handlers_test.go internal/server/server_routes.go internal/audit/audit_manifests.go internal/audit/audit_manifests_test.go internal/desktop/types.go internal/desktop/ecalc_app_test.go ui/desktop_ecalc_i18n_test.go ui/desktop_ecalc_registration_test.go
```

Expected: no output from either command.

- [ ] **Step 2: Run the focused tests**

Run:

```
go test ./internal/server -run '(?i)(TestECalcCatalog|Bootstrap|DesktopApps|DesktopStatus)' -count=1
go test ./internal/audit -run 'Manifest' -count=1
go test ./internal/desktop -count=1
go test ./ui -run '(?i)(TestDesktopECalc|TestDesktopBuiltinAppNames|TestTranslations_|TestDesktopUsedI18n|TestDesktopDynamicI18n|TestDesktopPapirus|TestDesktopAdditionalIcon|TestDesktopRadioApps|TestShippedWebAssets)' -count=1
```

Expected: four `ok` lines.

- [ ] **Step 3: Know the failures that are not yours**

At the base commit these tests already fail on this box, independent of Part B: `go test ./ui` fails `TestChatFrontend_IntegrationsDrawerRemainsWired` (marker `function speechLabBrowserURL()`) and `TestFrontend_StaticI18nKeysExistInEnglishBundle` (seven keys used by `findit-dialogs.js` and `findit-model.js`); `go test ./internal/audit` fails `TestUpdateConfigFallbackOnlyTrustedTransactionBackups`. Anything else red in those packages is new.


---

## Part C — Desktop UI

Contract deviations:

1. **Core semantics follow Part A's Go code where contract §3–§6 are looser.** The JS mirror is checked against Part A's real vectors, so these are the exact rules: `Solve` takes only primary variables as inputs (any other name is ignored); every unknown primary variable is `undetermined` (not only input-capable or rule-target ones); a check whose value is NaN does not fire; `if(c,a,b)` takes `b` when `c` is 0 or NaN; `Format` steps up to the next prefix at most once; `Parse` with an unknown unit id fails; the Greek mu (U+03BC) and the ohm sign (U+2126) are folded to µ (U+00B5) and Ω (U+03A9) while parsing. For every item of the phase 1 catalog the results are identical to the contract text.
2. **UI-only additions to the core files (no new file):** a unit id `prefix:<id>` (for example `prefix:m`, and `prefix:` for the plain unit) lets a field show a fixed prefix; `ECalcCore.solve.solve(calc, inputs, choices, units)` takes the units API as a fourth argument (it needs `dim(id).positive`); and `ecalc-solve.js` also holds the DOM-free input bookkeeping of a page (`defaultState`, `restoreState`, `readInputs`, `applyEdit`, `toggleLock`) next to `chooseInputs`, so that it is covered by the Node test. The file list of contract §1 is unchanged: eleven scripts in `ui/js/desktop/apps/ecalc/`.
3. **English names for the search.** The catalog JSON of §7 carries the name in the requested language only, but the spec searches the English name as well. When the desktop language is not `en` the app fetches `GET /api/ecalc/catalog?lang=en` once in the background and uses only its item names; a failure only narrows the search. Open point for A/B: a `name_en` field per item would remove that second request.
4. **Stored unit choices.** `units{calc.var}` stores `auto` for the automatic prefix (an absent entry means "the default unit of the variable", for example Ah for the charge of `charge_energy`); the stored value is ignored when the catalog no longer offers it.
5. **`api()` and `version`.** The desktop `api()` helper overwrites `body.version` of a JSON object with the ETag header when there is one. The app therefore never reads `catalog.version`.
6. **Vector formats.** Contract §8 names the vector files but not their JSON; Part C's parity test reads the format Part A published (core.json sections `expr`, `eval_error`, `parse`, `parse_error`, `formula`, `formula_error`, `deps`, `round_sig`, `eseries`, `eseries_error`, `eseries_tables`, `units_parse`, `units_parse_error`, `units_list`, `units_list_error`, `units_format`; calculators/`<id>`.json with `sets` and `solve`; see the header of `scripts/test-ecalc-core.mjs`).

**Dependencies on the other parts (read before starting).**

- **Part A** (`internal/ecalc/**`, including `testdata/catalog.en.json` and `testdata/vectors/**`) is needed by: Task C1 (the Node parity test reads the vectors and the catalog), Task C2 (the schematics Go test calls `ecalc.Default()` and `(*Catalog).LocalizedJSON("en")`) and Task C6 (the browser test serves the real catalog). Land Part A first, or have its files in the worktree.
- **Part B** (the 16 locale files `ui/lang/desktop/*.json` with `desktop.app_ecalc` and the 42 `desktop.ecalc_*` keys, the `BuiltinApps()` entry `ecalc` with its logo `ui/img/desktop-icons/ecalc.svg`, the route `GET /api/ecalc/catalog`) is needed by: the last test of Task C3 (`TestECalcUsesOnlyItsOwnTranslationKeys` checks the dimension names that the unit dropdown looks up dynamically), Task C5 (`TestDesktopUsedI18nKeysExistInAllLanguages`, `TestDesktopBuiltinAppNamesExistInAllLanguages`) and Task C6 (the browser test opens the app through `desktop.BuiltinApps()` and shows the German strings). Do not add `appIconKeys['ecalc']`, do not touch icon theme manifests or `desktopPreferredIconNames`: a themed icon would replace B's logo.
- Tasks C1 and C2 need no Part B file. Order: C1 → C2 → C3 → C4 → C5 → C6; C1/C2 can run before B lands, C3's last test and C5/C6 need B.

**Environment.** Windows box. Run Go commands in PowerShell with a short backslash `GOTMPDIR`; the browser test needs Chrome started from a test binary, so run it with the sandbox disabled (contract §0).

```powershell
$env:GOTMPDIR = 'C:\tmp\go-build-ecalc'          # create the folder once: New-Item -ItemType Directory -Force C:\tmp\go-build-ecalc
```

(Git Bash equivalent: `export GOTMPDIR='C:\tmp\go-build-ecalc'`; `export AURAGO_RUN_BROWSER_SMOKE=1`.) Commit messages end with the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

### Task C1: DOM-free core, typesetter and the parity test

**Files:**
- Create: `ui/js/desktop/apps/ecalc/ecalc-expr.js`, `ui/js/desktop/apps/ecalc/ecalc-units.js`, `ui/js/desktop/apps/ecalc/ecalc-eseries.js`, `ui/js/desktop/apps/ecalc/ecalc-solve.js`, `ui/js/desktop/apps/ecalc/ecalc-typeset.js`
- Create: `scripts/test-ecalc-core.mjs`
- Modify: `package.json` (base 1fb47da49: after line 24, `"test:docker-manager": ...`, in `scripts`)

The four core files publish `window.ECalcCore.{expr,units,eseries,solve}` and run in a Node vm without a document; `ecalc-typeset.js` publishes `window.ECalcTypeset` (also DOM-free). None of them uses `eval` or `Function`.

- [ ] **Step 1: Write the failing parity test and the npm script**

Create `scripts/test-ecalc-core.mjs`:

```javascript
// ECalc desktop app: parity test of the JavaScript core against the shared vectors of the Go package.
// The DOM-free files ui/js/desktop/apps/ecalc/ecalc-{expr,units,eseries,solve,typeset}.js run in a Node vm (no document),
// the calculators come from internal/ecalc/testdata/catalog.en.json (the catalog JSON of the server route), and every case
// of internal/ecalc/testdata/vectors/core.json and vectors/calculators/*.json must give the same result as Go: numbers match
// at a relative error of 1e-12 (calculator vectors 1e-9), a zero want allows an absolute error of 1e-12, and the strings
// "nan", "inf" and "-inf" stand for the non-finite doubles. The second half checks what exists on the UI side only:
// chooseInputs, the input bookkeeping of a page, the display units and the escaping of the typesetter.
// Run: node scripts/test-ecalc-core.mjs   (npm run test:ecalc)
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const files = ['ecalc-expr.js', 'ecalc-units.js', 'ecalc-eseries.js', 'ecalc-solve.js', 'ecalc-typeset.js'];
const plain = value => JSON.parse(JSON.stringify(value));
// The core runs in its own vm realm, so what it returns is compared as plain JSON.
const eq = (actual, expected, message) => assert.deepStrictEqual(plain(actual), plain(expected), message);

const window = {};
vm.runInNewContext(files.map(file => read('ui/js/desktop/apps/ecalc/' + file)).join('\n'), { window });
const core = window.ECalcCore, Typeset = window.ECalcTypeset;
assert.ok(core && core.expr && core.units && core.eseries && core.solve && Typeset, 'the core files publish window.ECalcCore and window.ECalcTypeset');
const catalog = JSON.parse(read('internal/ecalc/testdata/catalog.en.json'));
const units = core.units.create(catalog.units);
const calcOf = id => {
    const calc = catalog.items.find(item => item.id === id);
    assert.ok(calc, 'catalog has ' + id);
    return calc;
};
const calculators = catalog.items.filter(item => item.kind === 'calculator');

const decode = value => {
    if (typeof value !== 'string') return value;
    const word = value.toLowerCase();
    if (word === 'nan') return NaN;
    if (word === 'inf' || word === '+inf') return Infinity;
    if (word === '-inf') return -Infinity;
    return value;
};
function same(actual, expected, label, tolerance = 1e-12) {
    const want = decode(expected);
    if (Number.isNaN(want)) { assert.ok(Number.isNaN(actual), `${label}: want NaN, got ${actual}`); return; }
    if (!Number.isFinite(want)) { assert.equal(actual, want, label); return; }
    assert.ok(Number.isFinite(actual), `${label}: want ${want}, got ${actual}`);
    const allowed = want === 0 ? 1e-12 : tolerance * Math.abs(want);
    assert.ok(Math.abs(actual - want) <= allowed, `${label}: want ${want}, got ${actual}`);
}
// The prefix notation of the vectors: (op l r), (neg a), (pos a), (fn args), (= parts), numbers as written.
function sexp(node) {
    switch (node.t) {
        case 'num': return node.raw;
        case 'id': return node.name;
        case 'neg': case 'pos': return `(${node.t} ${sexp(node.a)})`;
        case 'bin': return `(${node.op} ${sexp(node.l)} ${sexp(node.r)})`;
        case 'call': return `(${node.name} ${node.args.map(sexp).join(' ')})`;
        case 'eq': return `(= ${node.parts.map(sexp).join(' ')})`;
        default: throw new Error('unknown node ' + node.t);
    }
}
const throwsCode = (fn, code, label) => assert.throws(fn, error => error.code === code, label);
let count = 0;

// ---- core.json: expressions, units, E-series --------------------------------------------------------------------------
const vectors = JSON.parse(read('internal/ecalc/testdata/vectors/core.json'));
const sections = ['expr', 'eval_error', 'parse', 'parse_error', 'formula', 'formula_error', 'deps', 'round_sig', 'eseries', 'eseries_error', 'units_parse', 'units_parse_error', 'units_list', 'units_list_error', 'units_format'];
for (const section of sections) assert.ok(Array.isArray(vectors[section]) && vectors[section].length > 0, `core.json has no ${section} vectors`);
const expr = core.expr;
const environment = v => Object.assign({}, v.env || {}, v.lists || {});
for (const v of vectors.expr) { same(expr.evaluate(expr.parse(v.src), environment(v)), v.want, 'expr ' + v.src); count += 1; }
for (const v of vectors.eval_error) { throwsCode(() => expr.evaluate(expr.parse(v.src), environment(v)), 'eval', 'eval error: ' + v.src); count += 1; }
for (const v of vectors.parse) { assert.equal(sexp(expr.parse(v.src)), v.ast, 'parse ' + v.src); count += 1; }
for (const src of vectors.parse_error) { throwsCode(() => expr.parse(src), 'parse', 'parse error: ' + JSON.stringify(src)); count += 1; }
for (const v of vectors.formula) { assert.equal(sexp(expr.parseFormula(v.src)), v.ast, 'formula ' + v.src); count += 1; }
for (const src of vectors.formula_error) { throwsCode(() => expr.parseFormula(src), 'parse', 'formula error: ' + JSON.stringify(src)); count += 1; }
for (const v of vectors.deps) { eq(expr.deps(expr.parse(v.src)), v.deps, 'deps ' + v.src); count += 1; }
for (const v of vectors.round_sig) { assert.equal(core.units.roundSig(v.x, v.n), v.want, `roundSig(${v.x}, ${v.n})`); count += 1; }
for (const v of vectors.eseries) {
    const found = core.eseries.nearest(v.series, decode(v.v));
    assert.ok(found, `eseries ${v.series} ${v.v} has a value`);
    same(found.value, v.want, `eseries ${v.series} ${v.v}`);
    same(found.dev, v.dev, `eseries ${v.series} ${v.v} deviation`);
    count += 1;
}
for (const v of vectors.eseries_error) { assert.equal(core.eseries.nearest(v.series, decode(v.v)), null, `eseries error ${v.series} ${v.v}`); count += 1; }
assert.ok(vectors.eseries_tables, 'core.json carries the E-series tables');
for (const [name, table] of Object.entries(vectors.eseries_tables)) { eq(core.eseries.TABLES[name], table, 'table ' + name); count += 1; }
eq(Object.keys(core.eseries.TABLES), Object.keys(vectors.eseries_tables), 'the same series');
for (const v of vectors.units_parse) { same(units.parse(v.in, v.dim, v.unit || ''), v.want, `parse ${JSON.stringify(v.in)} as ${v.dim} ${v.unit || ''}`); count += 1; }
for (const v of vectors.units_parse_error) { throwsCode(() => units.parse(v.in, v.dim, v.unit || ''), 'parse', `parse error ${JSON.stringify(v.in)} as ${v.dim} ${v.unit || ''}`); count += 1; }
for (const v of vectors.units_list) { eq(units.parseList(v.in, v.dim, v.unit || '').map((x, i) => (Math.abs(x - v.want[i]) <= 1e-12 * Math.abs(v.want[i]) ? v.want[i] : x)), v.want, `list ${JSON.stringify(v.in)}`); count += 1; }
for (const v of vectors.units_list_error) { throwsCode(() => units.parseList(v.in, v.dim, v.unit || ''), 'parse', `list error ${JSON.stringify(v.in)}`); count += 1; }
for (const v of vectors.units_format) { assert.equal(units.format(decode(v.v), v.dim, v.unit || '', v.sig || 0), v.want, `format ${v.v} as ${v.dim} ${v.unit || ''}`); count += 1; }

// ---- calculators/*.json: consistent solutions and single solves -----------------------------------------------------
const split = values => {
    const env = {};
    for (const [name, value] of Object.entries(values)) env[name] = Array.isArray(value) ? value.map(decode) : decode(value);
    return env;
};
const SOLVE_TOLERANCE = 1e-9;
for (const item of calculators) {
    const file = path.join(root, 'internal/ecalc/testdata/vectors/calculators', item.id + '.json');
    assert.ok(fs.existsSync(file), `no vectors for the calculator ${item.id}`);
    const set = JSON.parse(fs.readFileSync(file, 'utf8'));
    assert.equal(set.calc, item.id);
    assert.ok(set.solve.length > 0, `${item.id} has no solve vectors`);
    assert.ok(set.sets.length >= 2, `${item.id} needs at least two consistent solutions`);
    set.sets.forEach((entry, index) => {
        const label = `${item.id} set ${index} (${entry.ref})`;
        const values = split(entry.values);
        // (a) every alternative of every rule and derived quantity reproduces its target
        const env = Object.assign({}, core.solve.choiceConsts(item, entry.choices || {}), values);
        for (const target of item.rules.map(rule => ({ name: rule.target, alts: rule.alts })).concat(item.derived.map(d => ({ name: d.id, alts: d.alts })))) {
            target.alts.forEach((alt, altIndex) => same(expr.evaluate(expr.parseCached(alt), env), values[target.name], `${label}: ${target.name} alternative ${altIndex} (${alt})`, SOLVE_TOLERANCE));
        }
        // (b) every solvable input set reproduces the whole solution
        for (const names of item.sets) {
            const inputs = {};
            for (const name of names) inputs[name] = values[name];
            const result = core.solve.solve(item, inputs, entry.choices || {}, units);
            eq(result.errors, {}, `${label} from ${names}: no errors`);
            for (const [name, want] of Object.entries(values)) {
                if (Array.isArray(want)) eq(result.values[name], want, `${label} from ${names}: ${name}`);
                else same(result.values[name], want, `${label} from ${names}: ${name}`, SOLVE_TOLERANCE);
            }
            count += 1;
        }
    });
    for (const [index, v] of set.solve.entries()) {
        const label = `${item.id} solve ${index} (${v.ref})`;
        const result = core.solve.solve(item, split(v.inputs), v.choices || {}, units);
        for (const [name, want] of Object.entries(v.expect || {})) same(result.values[name], want, `${label}: ${name}`, SOLVE_TOLERANCE);
        eq(result.errors, v.errors || {}, `${label}: errors`);
        eq(result.warnings, v.warnings || [], `${label}: warnings`);
        count += 1;
    }
}

// ---- the defaults of every calculator solve, and every solvable set reproduces them (every choice option) ---------------
for (const item of calculators) {
    const selections = [{}];
    for (const choice of item.choices) for (const option of choice.options) selections.push({ [choice.id]: option.id });
    for (const choices of selections) {
        const auto = () => '';
        const state = core.solve.defaultState(item, units, auto);
        state.choices = Object.assign({}, state.choices, choices);
        const base = core.solve.solve(item, core.solve.readInputs(item, state, units, auto).values, state.choices, units);
        eq(base.errors, {}, `${item.id} ${JSON.stringify(choices)}: the defaults solve without errors`);
        for (const names of item.sets) {
            const inputs = {};
            for (const name of names) inputs[name] = base.values[name];
            const again = core.solve.solve(item, inputs, state.choices, units);
            eq(again.errors, {}, `${item.id} from ${names}: no errors`);
            for (const v of item.vars) {
                if (Array.isArray(base.values[v.id])) eq(again.values[v.id], base.values[v.id]);
                else same(again.values[v.id], base.values[v.id], `${item.id} ${JSON.stringify(choices)} from ${names}: ${v.id}`, SOLVE_TOLERANCE);
            }
            count += 1;
        }
    }
}

// ---- chooseInputs: which variables stay inputs after the user typed into one ------------------------------------
{
    const choose = core.solve.chooseInputs;
    const divider = [['vin', 'r1', 'r2'], ['vin', 'r1', 'vout'], ['vin', 'r2', 'vout'], ['r1', 'r2', 'vout']];
    eq(calcOf('voltage_divider').sets, divider, 'the catalog lists the divider sets in declaration order');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', []), { set: ['r1', 'r2', 'vout'], recency: ['r1', 'r2', 'vout'] }, 'the oldest input yields');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'r1', []), { set: ['vin', 'r1', 'r2'], recency: ['vin', 'r2', 'r1'] }, 'editing an input keeps the set and makes it the newest');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin']), { set: ['vin', 'r2', 'vout'], recency: ['vin', 'r2', 'vout'] }, 'a locked input never yields');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin', 'r1']), { set: ['vin', 'r1', 'vout'], recency: ['vin', 'r1', 'vout'] }, 'two locks');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'vout', ['vin', 'r1', 'r2']).set, ['r1', 'r2', 'vout'], 'locks that no set can honour are ignored');
    eq(choose(divider, ['vin', 'r1', 'r2'], 'nothing', []), null, 'a variable in no set');
    eq(choose([['x', 'b'], ['x', 'c']], ['a'], 'x', []).set, ['x', 'b'], 'a tie goes to the first set');
    const starDelta = [['rab', 'rbc', 'rca'], ['ra', 'rb', 'rc']];
    eq(calcOf('star_delta').sets, starDelta, 'the catalog lists exactly the delta set and the star set');
    eq(choose(starDelta, ['rab', 'rbc', 'rca'], 'ra', []), { set: ['ra', 'rb', 'rc'], recency: ['rb', 'rc', 'ra'] }, 'star-delta switches the whole set');
    eq(choose(starDelta, ['rb', 'rc', 'ra'], 'rbc', []), { set: ['rab', 'rbc', 'rca'], recency: ['rab', 'rca', 'rbc'] }, 'and back');
    eq(choose(calcOf('ohms_law').sets, ['u', 'i'], 'p', []).set, ['i', 'p'], 'ohms law: typing a power keeps the newer current');
    count += 12;
}

// ---- the input bookkeeping of a page ------------------------------------------------------------------------------
{
    const S = core.solve, auto = () => '';
    const divider = calcOf('voltage_divider');
    let state = S.defaultState(divider, units, auto);
    eq(state, { inputs: { vin: '12 V', r1: '4.7 kΩ', r2: '2.2 kΩ' }, recency: ['vin', 'r1', 'r2'], locked: [], choices: {} });
    let read = S.readInputs(divider, state, units, auto);
    eq(read.values, { vin: 12, r1: 4700, r2: 2200 });
    let shown = S.solve(divider, read.values, state.choices, units).values;
    same(shown.vout, 3.826086956521739, 'divider default output');

    state = S.applyEdit(divider, state, 'vout', '4 V', units, auto, shown);
    eq(state, { inputs: { r1: '4.7 kΩ', r2: '2.2 kΩ', vout: '4 V' }, recency: ['r1', 'r2', 'vout'], locked: [], choices: {} }, 'typing into the output makes the oldest input (vin) computed');
    read = S.readInputs(divider, state, units, auto);
    shown = S.solve(divider, read.values, state.choices, units).values;
    same(shown.vin, 4 * 6900 / 2200, 'divider input from the output');

    state = S.toggleLock(state, 'r1');
    eq(state.locked, ['r1']);
    state = S.applyEdit(divider, state, 'vin', '9 V', units, auto, shown);
    eq(state.inputs, { vin: '9 V', r1: '4.7 kΩ', vout: '4 V' }, 'the locked r1 stays an input, the oldest unlocked r2 yields');
    eq(S.toggleLock(state, 'r1').locked, [], 'unlocking');

    eq(S.readInputs(divider, { inputs: { vin: 'abc', r1: '1k', r2: '1k' } }, units, auto).errors, { vin: 'parse' }, 'a text that does not parse is reported');
    eq(S.restoreState(divider, { inputs: { vin: '5 V', r1: '1k', r2: '1k' }, recency: ['r2', 'vin', 'r1'], locked: ['r1', 'ghost'], choices: {} }, units, auto),
        { inputs: { vin: '5 V', r1: '1k', r2: '1k' }, recency: ['r2', 'vin', 'r1'], locked: ['r1'], choices: {} }, 'a stored state is restored');
    eq(S.restoreState(divider, { inputs: { vin: '5 V', r1: '1k' } }, units, auto), S.defaultState(divider, units, auto), 'a stored state that is no solvable set falls back to the defaults');
    eq(S.restoreState(divider, 'junk', units, auto), S.defaultState(divider, units, auto));

    // A computed variable that becomes an input keeps the value it showed.
    const star = calcOf('star_delta');
    let starState = S.defaultState(star, units, auto);
    const starShown = S.solve(star, S.readInputs(star, starState, units, auto).values, starState.choices, units).values;
    starState = S.applyEdit(star, starState, 'ra', '12', units, auto, starShown);
    eq(starState, { inputs: { ra: '12', rb: '10 Ω', rc: '30 Ω' }, recency: ['rb', 'rc', 'ra'], locked: [], choices: {} }, 'star-delta: rb and rc keep their computed values');
    same(S.solve(star, S.readInputs(star, starState, units, auto).values, {}, units).values.rab, (12 * 10 + 10 * 30 + 30 * 12) / 30, 'star-delta result after the switch');

    const list = calcOf('series_parallel');
    const listState = S.defaultState(list, units, auto);
    eq(listState.inputs, { rs: '4.7kΩ, 10kΩ, 2.2kΩ' }, 'list items are written without blanks');
    eq(S.readInputs(list, listState, units, auto).values, { rs: [4700, 10000, 2200] });

    const ratio = calcOf('decibel_ratio');
    eq(S.defaultState(ratio, units, auto).choices, { qty: 'voltage' });
    eq(S.restoreState(ratio, { inputs: { a1: '1', a2: '10' }, recency: [], locked: [], choices: { qty: 'power' } }, units, auto).choices, { qty: 'power' });
    eq(S.restoreState(ratio, { inputs: { a1: '1', a2: '10' }, recency: [], locked: [], choices: { qty: 'bogus' } }, units, auto).choices, { qty: 'voltage' });
    count += 14;
}

// ---- display units of the pages (fixed prefixes) and the folded input characters ----------------------------------------
{
    assert.equal(units.format(0.0038, 'voltage', 'prefix:m', 4), '3.8 mV');
    assert.equal(units.format(0.0038, 'voltage', 'prefix:', 4), '0.0038 V');
    assert.equal(units.format(12, 'voltage', 'prefix:k', 4), '0.012 kV');
    same(units.parse('5', 'voltage', 'prefix:m'), 0.005, 'a plain number is in the chosen prefix unit');
    same(units.parse('5V', 'voltage', 'prefix:m'), 5, 'a typed unit wins');
    same(units.parse('5k', 'voltage', 'prefix:m'), 5000, 'a typed prefix wins');
    throwsCode(() => units.parse('5', 'voltage', 'prefix:x'), 'parse', 'an unknown fixed prefix');
    eq(units.options('voltage').map(option => option.id), ['', 'prefix:n', 'prefix:µ', 'prefix:m', 'prefix:', 'prefix:k', 'prefix:M', 'dBV', 'dBu']);
    eq(units.options('number').map(option => option.id), ['']);
    eq(units.options('resistance').map(option => option.label), ['', 'mΩ', 'Ω', 'kΩ', 'MΩ', 'GΩ']);
    same(units.parse('4.7kΩ', 'resistance', ''), 4700, 'the ohm sign');
    same(units.parse('3μA', 'current', ''), 3e-6, 'the Greek mu');
    count += 12;
}

// ---- the typesetter: structure and escaping ---------------------------------------------------------------------------
{
    const ctx = { labelOf: name => ({ vout: 'V_out', vin: 'V_in', r2: 'R_2', r1: 'R_1' }[name] || ''), constOf: name => (name === 'k' ? '20' : undefined) };
    const html = Typeset.formulaHTML(expr.parseFormula('vout = vin * r2 / (r1 + r2)'), ctx);
    assert.ok(html.includes('<span class="ec-frac">') && html.includes('V<sub>out</sub>') && html.includes('·') && !html.includes('((') , 'a fraction with subscripts and a centred dot');
    assert.ok(Typeset.formulaHTML(expr.parse('x^2 + sqrt(y)'), ctx).includes('<sup class="ec-sup">') && Typeset.formulaHTML(expr.parse('sqrt(y)'), ctx).includes('ec-sqrt'), 'superscript and radical');
    assert.ok(Typeset.formulaHTML(expr.parse('k * pi'), ctx).includes('20') && Typeset.formulaHTML(expr.parse('k * pi'), ctx).includes('π'), 'a choice const as its value, pi as the Greek letter');
    const hostile = Typeset.formulaHTML(expr.parse('a_b + c'), { labelOf: () => '<img src=x onerror=alert(1)>_<b>' });
    assert.ok(!hostile.includes('<img') && !hostile.includes('<b>') && hostile.includes('&lt;img'), 'labels from the catalog are escaped');
    const live = Typeset.liveHTML(expr.parseFormula('vout = vin / r2'), Object.assign({}, ctx, { valueOf: name => (name === 'vin' ? { html: '12 V', prec: 3 } : name === 'vout' ? { html: '3.8 V', prec: 3 } : null) }));
    assert.ok(live.includes('12 V') && live.includes('3.8 V') && live.includes('ec-u'), 'the live line shows values, the unknown variable keeps its label');
    count += 5;
}

console.log(`ecalc core tests: ${count} cases ok`);
```

In `package.json`, after the `test:docker-manager` line (base line 24) add the `test:ecalc` script:

Replace:

```json
    "test:docker-manager": "node scripts/test-docker-manager-ui.mjs",
```

with:

```json
    "test:docker-manager": "node scripts/test-docker-manager-ui.mjs",
    "test:ecalc": "node scripts/test-ecalc-core.mjs",
```

- [ ] **Step 2: Run it and see it fail**

Part A's `internal/ecalc/testdata/` must be in the worktree.

```powershell
node scripts/test-ecalc-core.mjs
```

Expected: FAIL with `Error: ENOENT: no such file or directory, open '...\ui\js\desktop\apps\ecalc\ecalc-expr.js'`.

- [ ] **Step 3: Create the five core files**

`ui/js/desktop/apps/ecalc/ecalc-expr.js`:

```javascript
/* ECalc expression language: lexer, parser, AST, evaluation and dependency lists. A DOM-free classic script that
   publishes window.ECalcCore.expr, so scripts/test-ecalc-core.mjs can run it in a Node vm against the shared vectors of
   internal/ecalc/testdata/vectors. The grammar below is the whole language (it mirrors internal/ecalc/expr.go): no
   assignment, no loops, a fixed function whitelist, and no dynamic code evaluation of any kind.

     expr   := cmp
     cmp    := add (('<' | '<=' | '>' | '>=' | '==' | '!=') add)?
     add    := mul (('+' | '-') mul)*
     mul    := unary (('*' | '/') unary)*
     unary  := ('-' | '+') unary | pow
     pow    := atom ('^' unary)?
     atom   := number | ident '(' args? ')' | ident | '(' expr ')'

   AST nodes: {t:'num',v,raw} {t:'id',name} {t:'neg',a} {t:'pos',a} {t:'bin',op,l,r} {t:'call',name,args}; a formula is
   {t:'eq',parts}. Errors carry code 'parse' or 'eval'. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});

    // Exact arity per function; a negative number means "at least that many".
    const ARITY = { sqrt: 1, ln: 1, log10: 1, exp: 1, sin: 1, cos: 1, tan: 1, atan: 1, abs: 1, round: 1, min: -2, max: -2, if: 3, sum: 1, par: 1 };
    const COMPARISONS = { '<': true, '<=': true, '>': true, '>=': true, '==': true, '!=': true };
    const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    const isDigit = char => char >= '0' && char <= '9';
    const isIdentStart = char => (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char === '_';
    const isIdentPart = char => isIdentStart(char) || isDigit(char);

    function fail(message, code) {
        const error = new Error(message);
        error.code = code || 'parse';
        return error;
    }

    function tokenize(source) {
        const text = String(source == null ? '' : source);
        const tokens = [];
        let i = 0;
        while (i < text.length) {
            const char = text[i];
            if (char === ' ' || char === '\t' || char === '\n' || char === '\r') { i += 1; continue; }
            if (isDigit(char) || (char === '.' && isDigit(text[i + 1]))) {
                let j = i;
                while (isDigit(text[j])) j += 1;
                if (text[j] === '.') {
                    if (!isDigit(text[j + 1])) throw fail('digits expected after the decimal point at ' + j);
                    j += 1;
                    while (isDigit(text[j])) j += 1;
                }
                if (text[j] === 'e' || text[j] === 'E') {
                    let k = j + 1;
                    if (text[k] === '+' || text[k] === '-') k += 1;
                    if (isDigit(text[k])) {
                        while (isDigit(text[k])) k += 1;
                        j = k;
                    }
                }
                const raw = text.slice(i, j);
                tokens.push({ k: 'num', v: Number(raw), raw });
                i = j;
                continue;
            }
            if (isIdentStart(char)) {
                let j = i + 1;
                while (j < text.length && isIdentPart(text[j])) j += 1;
                tokens.push({ k: 'id', v: text.slice(i, j) });
                i = j;
                continue;
            }
            const pair = text.slice(i, i + 2);
            if (pair === '<=' || pair === '>=' || pair === '==' || pair === '!=') {
                tokens.push({ k: 'op', v: pair });
                i += 2;
                continue;
            }
            if ('+-*/^(),<>='.indexOf(char) >= 0) {
                tokens.push({ k: 'op', v: char });
                i += 1;
                continue;
            }
            throw fail('unexpected character ' + JSON.stringify(char) + ' at ' + i);
        }
        tokens.push({ k: 'eof', v: '' });
        return tokens;
    }

    function parseTokens(tokens, formula) {
        let pos = 0;
        const peek = () => tokens[pos];
        const isOp = value => peek().k === 'op' && peek().v === value;
        const take = () => tokens[pos++];

        function parseCmp() {
            const left = parseAdd();
            const token = peek();
            if (token.k === 'op' && has(COMPARISONS, token.v)) {
                take();
                return { t: 'bin', op: token.v, l: left, r: parseAdd() };
            }
            return left;
        }
        function parseAdd() {
            let left = parseMul();
            while (isOp('+') || isOp('-')) {
                const op = take().v;
                left = { t: 'bin', op, l: left, r: parseMul() };
            }
            return left;
        }
        function parseMul() {
            let left = parseUnary();
            while (isOp('*') || isOp('/')) {
                const op = take().v;
                left = { t: 'bin', op, l: left, r: parseUnary() };
            }
            return left;
        }
        function parseUnary() {
            if (isOp('-')) { take(); return { t: 'neg', a: parseUnary() }; }
            if (isOp('+')) { take(); return { t: 'pos', a: parseUnary() }; }
            return parsePow();
        }
        function parsePow() {
            const base = parseAtom();
            if (isOp('^')) {
                take();
                return { t: 'bin', op: '^', l: base, r: parseUnary() };
            }
            return base;
        }
        function parseAtom() {
            const token = take();
            if (token.k === 'num') return { t: 'num', v: token.v, raw: token.raw };
            if (token.k === 'id') {
                if (!isOp('(')) return { t: 'id', name: token.v };
                take();
                const args = [];
                if (!isOp(')')) {
                    args.push(parseCmp());
                    while (isOp(',')) { take(); args.push(parseCmp()); }
                }
                if (!isOp(')')) throw fail('")" expected after the arguments of ' + token.v);
                take();
                if (!has(ARITY, token.v)) throw fail('unknown function ' + token.v);
                const arity = ARITY[token.v];
                if (arity >= 0 ? args.length !== arity : args.length < -arity) throw fail('wrong number of arguments for ' + token.v);
                return { t: 'call', name: token.v, args };
            }
            if (token.k === 'op' && token.v === '(') {
                const inner = parseCmp();
                if (!isOp(')')) throw fail('")" expected');
                take();
                return inner;
            }
            throw fail('unexpected ' + (token.k === 'eof' ? 'end of input' : JSON.stringify(token.v)));
        }

        const first = parseCmp();
        let result = first;
        if (formula) {
            const parts = [first];
            while (isOp('=')) { take(); parts.push(parseCmp()); }
            result = { t: 'eq', parts };
        }
        if (peek().k !== 'eof') throw fail('unexpected ' + JSON.stringify(peek().v));
        return result;
    }

    const parse = source => parseTokens(tokenize(source), false);
    const parseFormula = source => parseTokens(tokenize(source), true);

    // IEEE-754 power with the special cases of the Go standard library (math.Pow): 1^y and x^0 are 1 for every y,
    // including NaN, and (-1)^(+-Inf) is 1. Math.pow yields NaN for those.
    function power(base, exponent) {
        if (base === 1 || exponent === 0) return 1;
        if (base === -1 && (exponent === Infinity || exponent === -Infinity)) return 1;
        return Math.pow(base, exponent);
    }

    // Round half away from zero (math.Round).
    function roundHalfAway(x) {
        const magnitude = Math.abs(x);
        let rounded = Math.floor(magnitude);
        if (magnitude - rounded >= 0.5) rounded += 1;
        return Math.sign(x) * rounded;
    }

    function listArgument(node, env, name) {
        if (node.t === 'id' && has(env, node.name) && Array.isArray(env[node.name])) return env[node.name];
        throw fail(name + '() needs a list variable', 'eval');
    }

    function evaluate(node, env) {
        switch (node.t) {
            case 'num': return node.v;
            case 'id': {
                if (node.name === 'pi') return Math.PI;
                if (!has(env, node.name)) throw fail('unknown identifier ' + node.name, 'eval');
                const value = env[node.name];
                if (Array.isArray(value)) throw fail('list ' + node.name + ' used outside sum() or par()', 'eval');
                return value;
            }
            case 'neg': return -evaluate(node.a, env);
            case 'pos': return evaluate(node.a, env);
            case 'bin': {
                const l = evaluate(node.l, env), r = evaluate(node.r, env);
                switch (node.op) {
                    case '+': return l + r;
                    case '-': return l - r;
                    case '*': return l * r;
                    case '/': return l / r;
                    case '^': return power(l, r);
                    case '<': return l < r ? 1 : 0;
                    case '<=': return l <= r ? 1 : 0;
                    case '>': return l > r ? 1 : 0;
                    case '>=': return l >= r ? 1 : 0;
                    case '==': return l === r ? 1 : 0;
                    case '!=': return l !== r ? 1 : 0;
                    default: throw fail('unknown operator ' + node.op, 'eval');
                }
            }
            case 'call': {
                const args = node.args;
                if (node.name === 'if') {
                    const condition = evaluate(args[0], env);
                    return condition !== 0 && !Number.isNaN(condition) ? evaluate(args[1], env) : evaluate(args[2], env);
                }
                if (node.name === 'sum') return listArgument(args[0], env, 'sum').reduce((total, x) => total + x, 0);
                if (node.name === 'par') return 1 / listArgument(args[0], env, 'par').reduce((total, x) => total + 1 / x, 0);
                const x = args.map(arg => evaluate(arg, env));
                switch (node.name) {
                    case 'sqrt': return Math.sqrt(x[0]);
                    case 'ln': return Math.log(x[0]);
                    case 'log10': return Math.log10(x[0]);
                    case 'exp': return Math.exp(x[0]);
                    case 'sin': return Math.sin(x[0]);
                    case 'cos': return Math.cos(x[0]);
                    case 'tan': return Math.tan(x[0]);
                    case 'atan': return Math.atan(x[0]);
                    case 'abs': return Math.abs(x[0]);
                    case 'round': return roundHalfAway(x[0]);
                    case 'min': return Math.min(...x);
                    case 'max': return Math.max(...x);
                    default: throw fail('unknown function ' + node.name, 'eval');
                }
            }
            default:
                throw fail('cannot evaluate a ' + node.t + ' node', 'eval');
        }
    }

    // Sorted unique identifiers that are neither function names nor the constant pi.
    function deps(node) {
        const found = new Set();
        (function walk(n) {
            switch (n.t) {
                case 'id': if (n.name !== 'pi' && !has(ARITY, n.name)) found.add(n.name); break;
                case 'neg': case 'pos': walk(n.a); break;
                case 'bin': walk(n.l); walk(n.r); break;
                case 'call': n.args.forEach(walk); break;
                case 'eq': n.parts.forEach(walk); break;
                default: break;
            }
        })(node);
        return [...found].sort();
    }

    // Parsed expressions are cached by source text: the solver evaluates the same rules on every keystroke.
    const cache = new Map();
    function parseCached(source) {
        let ast = cache.get(source);
        if (!ast) {
            ast = parse(source);
            cache.set(source, ast);
        }
        return ast;
    }

    core.expr = { ARITY, parse, parseFormula, parseCached, evaluate, deps, power, roundHalfAway };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-units.js`:

```javascript
/* ECalc units: dimensions, SI prefixes, input parsing ("4k7", "2u2", "10 mA", "1e-6", comma decimals) and output
   formatting (engineering notation with a configurable number of significant digits). A DOM-free classic script that
   publishes window.ECalcCore.units; it mirrors internal/ecalc/units.go and is checked against the same vectors.

   ECalcCore.units.create(catalog.units) returns the API used by the solver and the pages:
     parse(input, dim, unit) -> base value        parseList(input, dim, unit) -> [base value]
     format(value, dim, unit, sig) -> text        dim(id) -> {symbol, positive, ...}     options(dim) -> unit choices
   `unit` is '' (automatic prefix), the id of an explicit unit (dBm, Ah ...) or, as a display choice of the pages only,
   'prefix:<id>' for a fixed prefix (prefix:m shows milli-units, prefix: the plain unit). Every value is in SI base units. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const expr = core.expr;

    const NUMBER = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;
    const NUMBER_PREFIX = /^([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)([pnuµmkKMG])$/;
    const INFIX = /^(\d+)([pnuµmkKMG])(\d+)$/;
    const INFIX_R = /^(\d+)R(\d+)$/;
    const PREFIX_UNIT = 'prefix:';
    // U+03BC (Greek mu) is typed for the micro sign U+00B5, U+2126 (ohm sign) for the Greek capital omega U+03A9.
    const fold = text => text.split(String.fromCharCode(0x3bc)).join(String.fromCharCode(0xb5)).split(String.fromCharCode(0x2126)).join(String.fromCharCode(0x3a9));

    function fail(message) {
        const error = new Error(message);
        error.code = 'parse';
        return error;
    }

    // Round to n significant digits; fmtNum prints the shortest round-trip decimal (JS switches to an exponent below
    // 1e-6 and from 1e21, the same limits the contract sets for the Go side).
    const roundSig = (x, n) => Number(x.toPrecision(n));
    const fmtNum = x => String(x);
    const fmtSig = (x, n) => fmtNum(roundSig(x, n));

    function create(catalogUnits) {
        const source = catalogUnits || {};
        const prefixFactor = new Map([['', 1]]);
        for (const prefix of source.prefixes || []) prefixFactor.set(prefix.id, prefix.factor);
        // The letter u stands for the micro prefix, whatever character the catalog uses for it.
        let microId = 'µ';
        for (const [id, factor] of prefixFactor) if (factor === 1e-6) microId = id;
        const inputPrefix = letter => (letter === 'u' ? microId : letter === 'K' ? 'k' : letter);

        const dims = new Map();
        for (const raw of source.dims || []) {
            const prefixes = (raw.prefixes || []).map(id => ({ id, factor: prefixFactor.has(id) ? prefixFactor.get(id) : 1 })).sort((a, b) => a.factor - b.factor);
            const units = (raw.units || []).map(unit => {
                const linear = !unit.to;
                const toAst = linear ? null : expr.parseCached(unit.to), fromAst = linear ? null : expr.parseCached(unit.from);
                return {
                    id: unit.id, symbol: unit.symbol,
                    toBase: x => (linear ? x * unit.factor : expr.evaluate(toAst, { x })),
                    fromBase: v => (linear ? v / unit.factor : expr.evaluate(fromAst, { x: v }))
                };
            });
            dims.set(raw.id, {
                id: raw.id, symbol: raw.symbol || '', aliases: raw.aliases || [], positive: !!raw.positive, prefixes, units,
                unitById: new Map(units.map(unit => [unit.id, unit])),
                bySymbolLength: units.slice().sort((a, b) => b.symbol.length - a.symbol.length),
                names: [raw.symbol || ''].concat(raw.aliases || []).filter(Boolean).sort((a, b) => b.length - a.length)
            });
        }

        function dim(id) {
            const found = dims.get(id);
            if (!found) throw fail('unknown dimension ' + id);
            return found;
        }

        function parse(input, dimId, unitId) {
            const d = dim(dimId);
            if (unitId && !d.unitById.has(unitId) && !(unitId.startsWith(PREFIX_UNIT) && prefixFactor.has(unitId.slice(PREFIX_UNIT.length)))) throw fail('unknown unit ' + unitId + ' for ' + dimId);
            // The Greek mu and the ohm sign are folded into the micro sign and the Greek capital omega.
            let text = fold(String(input == null ? '' : input).trim().replace(/\s+/g, ''));
            if (text === '') throw fail('empty input');
            text = text.replace(/,/g, '.');

            // An explicit unit typed behind the number wins over everything else (longest symbol first).
            for (const unit of d.bySymbolLength) {
                if (text.endsWith(unit.symbol)) {
                    const rest = text.slice(0, text.length - unit.symbol.length);
                    if (NUMBER.test(rest)) return finite(unit.toBase(Number(rest)));
                }
            }
            // The base symbol or an alias may follow (4.7kΩ, 10mA, 100R); a prefix in front of it stays in the text.
            let suffixed = false;
            for (const name of d.names) {
                if (text.endsWith(name)) {
                    text = text.slice(0, text.length - name.length);
                    suffixed = true;
                    break;
                }
            }
            let value = NaN, prefix = '';
            let match;
            if (NUMBER.test(text)) value = Number(text);
            else if ((match = NUMBER_PREFIX.exec(text))) { value = Number(match[1]); prefix = inputPrefix(match[2]); }
            else if ((match = INFIX.exec(text))) { value = Number(match[1] + '.' + match[3]); prefix = inputPrefix(match[2]); }
            else if (d.id === 'resistance' && (match = INFIX_R.exec(text))) value = Number(match[1] + '.' + match[2]);
            if (Number.isNaN(value)) throw fail('cannot read ' + JSON.stringify(input) + ' as a ' + dimId + ' value');
            if (prefix !== '' && !d.prefixes.some(entry => entry.id === prefix)) throw fail('the prefix ' + prefix + ' does not fit ' + dimId);

            const unit = unitId ? d.unitById.get(unitId) : null;
            if (unit && !suffixed) {
                if (prefix !== '') throw fail('a prefix needs a unit symbol in a ' + unit.symbol + ' field');
                return finite(unit.toBase(value));
            }
            if (!unit && unitId && unitId.startsWith(PREFIX_UNIT) && !suffixed && prefix === '') {
                return finite(value * prefixFactor.get(unitId.slice(PREFIX_UNIT.length)));
            }
            return finite(value * prefixFactor.get(prefix));
        }

        function finite(value) {
            if (!Number.isFinite(value)) throw fail('the value is out of range');
            return value;
        }

        // List fields: split on commas, semicolons and white space, then parse every item.
        function parseList(input, dimId, unitId) {
            const items = String(input == null ? '' : input).trim().split(/[,;\s]+/).filter(Boolean);
            if (!items.length) throw fail('empty list');
            return items.map(item => parse(item, dimId, unitId));
        }

        function format(value, dimId, unitId, sig) {
            const d = dims.get(dimId);
            if (!d || !Number.isFinite(value)) return '';
            const digits = sig || 4;
            const unit = unitId ? d.unitById.get(unitId) : null;
            if (unit) {
                const x = unit.fromBase(value);
                return Number.isFinite(x) ? fmtSig(x, digits) + ' ' + unit.symbol : '';
            }
            if (unitId && unitId.startsWith(PREFIX_UNIT)) {
                const id = unitId.slice(PREFIX_UNIT.length);
                if (prefixFactor.has(id)) return (fmtSig(value / prefixFactor.get(id), digits) + ' ' + id + d.symbol).trim();
            }
            if (!d.prefixes.length) return fmtSig(value, digits) + (d.symbol ? ' ' + d.symbol : '');
            if (value === 0) return '0' + (d.symbol ? ' ' + d.symbol : '');
            let index = -1;
            d.prefixes.forEach((entry, i) => { if (entry.factor <= Math.abs(value)) index = i; });
            if (index < 0) index = 0;
            let mantissa = roundSig(value / d.prefixes[index].factor, digits);
            if (Math.abs(mantissa) >= 1000 && index + 1 < d.prefixes.length) {
                index += 1;
                mantissa = roundSig(value / d.prefixes[index].factor, digits);
            }
            return (fmtNum(mantissa) + ' ' + d.prefixes[index].id + d.symbol).trim();
        }

        // The choices of a unit dropdown: automatic, the fixed prefixes, then the explicit units of the dimension.
        function options(dimId) {
            const d = dim(dimId);
            const list = [{ id: '', label: '', kind: 'auto' }];
            for (const entry of d.prefixes) list.push({ id: PREFIX_UNIT + entry.id, label: entry.id + d.symbol, kind: 'prefix' });
            for (const unit of d.units) list.push({ id: unit.id, label: unit.symbol, kind: 'explicit' });
            return d.prefixes.length || d.units.length ? list : [list[0]];
        }

        return { dim, parse, parseList, format, options, roundSig, fmtNum, fmtSig };
    }

    core.units = { create, roundSig, fmtNum, fmtSig, PREFIX_UNIT };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-eseries.js`:

```javascript
/* ECalc E-series: the IEC 60063 value tables E3 to E192 and the nearest standard value with its deviation. A DOM-free
   classic script that publishes window.ECalcCore.eseries; it mirrors internal/ecalc/eseries.go and is checked against
   the same vectors. The decade values are written out because E3 to E24 are not the rounded roots (2.7 is not 2.6). */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});

    const words = text => text.split(' ').map(Number);
    const TABLES = {
        E3: words('1.0 2.2 4.7'),
        E6: words('1.0 1.5 2.2 3.3 4.7 6.8'),
        E12: words('1.0 1.2 1.5 1.8 2.2 2.7 3.3 3.9 4.7 5.6 6.8 8.2'),
        E24: words('1.0 1.1 1.2 1.3 1.5 1.6 1.8 2.0 2.2 2.4 2.7 3.0 3.3 3.6 3.9 4.3 4.7 5.1 5.6 6.2 6.8 7.5 8.2 9.1'),
        E48: words('1.00 1.05 1.10 1.15 1.21 1.27 1.33 1.40 1.47 1.54 1.62 1.69 1.78 1.87 1.96 2.05 2.15 2.26 2.37 2.49 2.61 2.74 2.87 3.01 3.16 3.32 3.48 3.65 3.83 4.02 4.22 4.42 4.64 4.87 5.11 5.36 5.62 5.90 6.19 6.49 6.81 7.15 7.50 7.87 8.25 8.66 9.09 9.53'),
        E96: words('1.00 1.02 1.05 1.07 1.10 1.13 1.15 1.18 1.21 1.24 1.27 1.30 1.33 1.37 1.40 1.43 1.47 1.50 1.54 1.58 1.62 1.65 1.69 1.74 1.78 1.82 1.87 1.91 1.96 2.00 2.05 2.10 2.15 2.21 2.26 2.32 2.37 2.43 2.49 2.55 2.61 2.67 2.74 2.80 2.87 2.94 3.01 3.09 3.16 3.24 3.32 3.40 3.48 3.57 3.65 3.74 3.83 3.92 4.02 4.12 4.22 4.32 4.42 4.53 4.64 4.75 4.87 4.99 5.11 5.23 5.36 5.49 5.62 5.76 5.90 6.04 6.19 6.34 6.49 6.65 6.81 6.98 7.15 7.32 7.50 7.68 7.87 8.06 8.25 8.45 8.66 8.87 9.09 9.31 9.53 9.76'),
        E192: words('1.00 1.01 1.02 1.04 1.05 1.06 1.07 1.09 1.10 1.11 1.13 1.14 1.15 1.17 1.18 1.20 1.21 1.23 1.24 1.26 1.27 1.29 1.30 1.32 1.33 1.35 1.37 1.38 1.40 1.42 1.43 1.45 1.47 1.49 1.50 1.52 1.54 1.56 1.58 1.60 1.62 1.64 1.65 1.67 1.69 1.72 1.74 1.76 1.78 1.80 1.82 1.84 1.87 1.89 1.91 1.93 1.96 1.98 2.00 2.03 2.05 2.08 2.10 2.13 2.15 2.18 2.21 2.23 2.26 2.29 2.32 2.34 2.37 2.40 2.43 2.46 2.49 2.52 2.55 2.58 2.61 2.64 2.67 2.71 2.74 2.77 2.80 2.84 2.87 2.91 2.94 2.98 3.01 3.05 3.09 3.12 3.16 3.20 3.24 3.28 3.32 3.36 3.40 3.44 3.48 3.52 3.57 3.61 3.65 3.70 3.74 3.79 3.83 3.88 3.92 3.97 4.02 4.07 4.12 4.17 4.22 4.27 4.32 4.37 4.42 4.48 4.53 4.59 4.64 4.70 4.75 4.81 4.87 4.93 4.99 5.05 5.11 5.17 5.23 5.30 5.36 5.42 5.49 5.56 5.62 5.69 5.76 5.83 5.90 5.97 6.04 6.12 6.19 6.26 6.34 6.42 6.49 6.57 6.65 6.73 6.81 6.90 6.98 7.06 7.15 7.23 7.32 7.41 7.50 7.59 7.68 7.77 7.87 7.96 8.06 8.16 8.25 8.35 8.45 8.56 8.66 8.76 8.87 8.98 9.09 9.20 9.31 9.42 9.53 9.65 9.76 9.88')
    };
    // The series the pages offer (E3 is too coarse to be useful as a suggestion).
    const SERIES = ['E6', 'E12', 'E24', 'E48', 'E96', 'E192'];

    // Nearest standard value of a series, by relative distance, a tie going to the lower value. The candidates of
    // the decade of v include the first value of the next decade, so a v just below a power of ten is handled.
    // Returns {value, dev} with dev in percent ((value - v) / v * 100), or null for v <= 0, a non-finite v or an
    // unknown series.
    function nearest(series, v) {
        if (!Object.prototype.hasOwnProperty.call(TABLES, series) || !Number.isFinite(v) || v <= 0) return null;
        const decade = Math.pow(10, Math.floor(Math.log10(v)));
        let best = 0, bestError = Infinity;
        for (const entry of TABLES[series].concat([10])) {
            const candidate = entry * decade;
            const error = Math.abs(candidate - v) / v;
            if (error < bestError) { best = candidate; bestError = error; }
        }
        best = Number(best.toPrecision(12));
        return { value: best, dev: (best - v) / v * 100 };
    }

    core.eseries = { TABLES, SERIES, nearest };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-solve.js`:

```javascript
/* ECalc solver: rule propagation over a catalog calculator, plus the input bookkeeping of the pages (which variables
   are inputs, which yield when the user types into a computed one). A DOM-free classic script that publishes
   window.ECalcCore.solve; solve() mirrors internal/ecalc/solve.go and is checked against the same vectors, while
   chooseInputs() and the state helpers exist on the UI side only.

   A calculator is the item of the catalog JSON: vars[], choices[], dof, rules[{target, alts}], derived[{id, alts}],
   checks[{when, warn}], sets[[var ids]]. Inputs are base-unit numbers, or arrays of them for list variables. */
(function () {
    'use strict';

    const core = (window.ECalcCore = window.ECalcCore || {});
    const expr = core.expr;

    const isFiniteValue = value => (Array.isArray(value) ? value.every(Number.isFinite) : Number.isFinite(value));

    // The consts of the selected option of every choice (an unknown or missing selection falls back to the default).
    function choiceConsts(calc, choices) {
        const consts = {};
        for (const choice of calc.choices || []) {
            const wanted = choices && choices[choice.id];
            const option = choice.options.find(entry => entry.id === wanted) || choice.options.find(entry => entry.id === choice.default) || choice.options[0];
            if (option) Object.assign(consts, option.consts || {});
        }
        return consts;
    }

    // Solve(calc, inputs, choices, units) -> {values, computed, errors, warnings}. `units` supplies dim(id).positive.
    function solve(calc, inputs, choices, units) {
        const env = Object.create(null), known = new Set();
        const consts = choiceConsts(calc, choices);
        for (const name of Object.keys(consts)) { env[name] = consts[name]; known.add(name); }
        // Only primary variables can be inputs; any other name in `inputs` is ignored.
        for (const v of calc.vars) {
            if (!inputs || !Object.prototype.hasOwnProperty.call(inputs, v.id) || inputs[v.id] === undefined) continue;
            env[v.id] = inputs[v.id];
            known.add(v.id);
        }

        const order = calc.rules.map(rule => ({ target: rule.target, alts: rule.alts })).concat(calc.derived.map(item => ({ target: item.id, alts: item.alts })));
        const computed = [];
        for (let changed = true; changed;) {
            changed = false;
            for (const item of order) {
                if (known.has(item.target)) continue;
                for (const alt of item.alts) {
                    const ast = expr.parseCached(alt);
                    if (!expr.deps(ast).every(name => known.has(name))) continue;
                    let value;
                    try { value = expr.evaluate(ast, env); } catch (error) { value = NaN; }
                    env[item.target] = value;
                    known.add(item.target);
                    computed.push(item.target);
                    changed = true;
                    break;
                }
            }
        }

        const values = {}, errors = {};
        for (const v of calc.vars) {
            if (!known.has(v.id)) {
                values[v.id] = NaN;
                errors[v.id] = 'undetermined';
                continue;
            }
            const value = env[v.id];
            const positive = !!(units && units.dim(v.dim).positive);
            values[v.id] = Array.isArray(value) ? value.slice() : value;
            if (!isFiniteValue(value)) errors[v.id] = 'nonfinite';
            else if (positive && (Array.isArray(value) ? value.some(x => x <= 0) : value <= 0)) errors[v.id] = 'range';
        }
        for (const item of calc.derived) {
            values[item.id] = known.has(item.id) && Number.isFinite(env[item.id]) ? env[item.id] : NaN;
        }

        const warnings = [];
        (calc.checks || []).forEach((check, index) => {
            const ast = expr.parseCached(check.when);
            if (!expr.deps(ast).every(name => known.has(name) && isFiniteValue(env[name]))) return;
            let result;
            try { result = expr.evaluate(ast, env); } catch (error) { return; }
            if (result !== 0 && !Number.isNaN(result)) warnings.push(index);
        });
        return { values, computed, errors, warnings };
    }

    // The user typed into `edited`. Among the solvable sets that contain it (and every locked variable, when any
    // set does) the set wins that keeps the most recently edited variables as inputs; a tie goes to the first set.
    // recency lists the current inputs from the oldest edit to the newest. Returns {set, recency} or null when no
    // set contains `edited`.
    function chooseInputs(sets, recency, edited, locked) {
        const withEdited = sets.filter(set => set.includes(edited));
        if (!withEdited.length) return null;
        const pinned = (locked || []).filter(Boolean);
        const withLocks = withEdited.filter(set => pinned.every(name => set.includes(name)));
        const candidates = withLocks.length ? withLocks : withEdited;
        let best = null, bestScore = -1;
        for (const set of candidates) {
            let score = 0;
            recency.forEach((name, index) => { if (name !== edited && set.includes(name)) score += Math.pow(2, index); });
            if (score > bestScore) { best = set; bestScore = score; }
        }
        const kept = recency.filter(name => name !== edited && best.includes(name));
        const added = best.filter(name => name !== edited && !recency.includes(name));
        return { set: best.slice(), recency: added.concat(kept, [edited]) };
    }

    // ---- input bookkeeping of a page -----------------------------------------------------------------------------
    // A page state is {inputs: {var: raw text}, recency: [var], locked: [var], choices: {choice: option}}; it is what
    // the store persists. `unitOf(varId)` names the display unit of a variable ('' = automatic).

    const hasDefault = v => v.default != null || v.default_list != null;
    const SIG_EXACT = 12;

    // Text for a base-unit value that parses back to the same number: the symbol is part of the text, so the unit
    // dropdown cannot change its meaning. List items lose their blanks to survive the list splitting.
    function exactText(v, value, units, unitOf) {
        const one = x => units.format(x, v.dim, unitOf(v.id), SIG_EXACT);
        return Array.isArray(value) ? value.map(x => one(x).replace(/\s+/g, '')).join(', ') : one(value);
    }

    function defaultState(calc, units, unitOf) {
        const inputs = {}, recency = [];
        for (const v of calc.vars) {
            if (v.input === false || !hasDefault(v)) continue;
            inputs[v.id] = exactText(v, v.list ? v.default_list : v.default, units, unitOf);
            recency.push(v.id);
        }
        const choices = {};
        for (const choice of calc.choices || []) choices[choice.id] = choice.default;
        return { inputs, recency, locked: [], choices };
    }

    // A stored state is only used when its inputs are one of the solvable sets; anything else falls back to the defaults.
    function restoreState(calc, stored, units, unitOf) {
        const base = defaultState(calc, units, unitOf);
        if (!stored || typeof stored !== 'object' || !stored.inputs || typeof stored.inputs !== 'object') return base;
        const names = Object.keys(stored.inputs);
        const sameSet = calc.sets.some(set => set.length === names.length && set.every(name => names.includes(name)));
        if (!sameSet || !names.every(name => typeof stored.inputs[name] === 'string')) return base;
        const known = new Set(calc.vars.map(v => v.id));
        const recency = (Array.isArray(stored.recency) ? stored.recency : []).filter(name => names.includes(name));
        for (const name of names) if (!recency.includes(name)) recency.unshift(name);
        const choices = Object.assign({}, base.choices);
        for (const choice of calc.choices || []) {
            const wanted = stored.choices && stored.choices[choice.id];
            if (choice.options.some(option => option.id === wanted)) choices[choice.id] = wanted;
        }
        return {
            inputs: Object.assign({}, stored.inputs),
            recency,
            locked: (Array.isArray(stored.locked) ? stored.locked : []).filter(name => known.has(name)),
            choices
        };
    }

    // Parse the raw texts of a state. Returns {values, errors}; errors maps a variable to 'parse'.
    function readInputs(calc, state, units, unitOf) {
        const values = {}, errors = {};
        for (const v of calc.vars) {
            if (!Object.prototype.hasOwnProperty.call(state.inputs, v.id)) continue;
            try {
                values[v.id] = v.list ? units.parseList(state.inputs[v.id], v.dim, unitOf(v.id)) : units.parse(state.inputs[v.id], v.dim, unitOf(v.id));
            } catch (error) {
                errors[v.id] = 'parse';
            }
        }
        return { values, errors };
    }

    // The user entered `raw` into `edited`. `lastValues` are the base-unit results shown so far; a computed variable
    // that becomes an input keeps the value it showed. Returns the new state (the argument is not changed).
    function applyEdit(calc, state, edited, raw, units, unitOf, lastValues) {
        const chosen = chooseInputs(calc.sets, state.recency, edited, state.locked);
        if (!chosen) return state;
        const inputs = {};
        for (const name of chosen.set) {
            if (name === edited) { inputs[name] = raw; continue; }
            if (Object.prototype.hasOwnProperty.call(state.inputs, name)) { inputs[name] = state.inputs[name]; continue; }
            const v = calc.vars.find(entry => entry.id === name);
            const shown = lastValues && lastValues[name];
            if (v && shown != null && isFiniteValue(shown)) inputs[name] = exactText(v, shown, units, unitOf);
            else if (v && hasDefault(v)) inputs[name] = exactText(v, v.list ? v.default_list : v.default, units, unitOf);
        }
        return { inputs, recency: chosen.recency, locked: state.locked.slice(), choices: Object.assign({}, state.choices) };
    }

    function toggleLock(state, name) {
        const locked = state.locked.includes(name) ? state.locked.filter(entry => entry !== name) : state.locked.concat([name]);
        return Object.assign({}, state, { locked });
    }

    core.solve = { solve, chooseInputs, choiceConsts, defaultState, restoreState, readInputs, applyEdit, toggleLock, exactText };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-typeset.js`:

```javascript
/* ECalc typesetting: turns the AST of a catalog formula into HTML (fractions for /, superscripts for ^, a radical for
   sqrt, a centred dot for *, pi as the Greek letter, labels such as V_in with a subscript) and renders the live line,
   the same AST with the current values substituted. DOM-free classic script that publishes window.ECalcTypeset.
   Everything that comes from the catalog or from a value is escaped; the only markup is built here.

   The context object of both renderers:
     labelOf(name)  -> display label of a variable (V_in), or '' for an unknown name
     constOf(name)  -> number of a choice const (k, cf), or undefined
     valueOf(name)  -> live line only: {html, prec} of the formatted value of a variable, or null when it is unknown
   prec is the binding strength of the html: 6 atom, 3 a product such as "12 V", 2 a sum or a negative number. */
(function () {
    'use strict';

    const SIGNS = { '<': '&lt;', '<=': '≤', '>': '&gt;', '>=': '≥', '==': '=', '!=': '≠' };
    const escape = text => String(text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const span = (name, html) => '<span class="ec-' + name + '">' + html + '</span>';

    // V_in -> V<sub>in</sub>: everything behind the first underscore is the subscript.
    function labelHTML(label) {
        const text = String(label == null ? '' : label), at = text.indexOf('_');
        if (at <= 0) return escape(text);
        return escape(text.slice(0, at)) + '<sub>' + escape(text.slice(at + 1)) + '</sub>';
    }

    function render(node, ctx, live) {
        const wrap = (child, minimum) => {
            const part = go(child);
            return part.prec < minimum ? '(' + part.html + ')' : part.html;
        };
        const go = child => render(child, ctx, live);
        switch (node.t) {
            case 'num': return { html: span('n', escape(node.raw)), prec: 6 };
            case 'id': {
                if (node.name === 'pi') return { html: span('n', 'π'), prec: 6 };
                const constant = ctx.constOf ? ctx.constOf(node.name) : undefined;
                if (constant !== undefined) return { html: span('n', escape(constant)), prec: 6 };
                if (live && ctx.valueOf) {
                    const value = ctx.valueOf(node.name);
                    if (value) return { html: span('v', value.html), prec: value.prec };
                }
                const label = (ctx.labelOf && ctx.labelOf(node.name)) || node.name;
                return { html: span(live ? 'u' : 'i', labelHTML(label)), prec: 6 };
            }
            case 'neg': return { html: '−' + wrap(node.a, 4), prec: 4 };
            case 'pos': return { html: '+' + wrap(node.a, 4), prec: 4 };
            case 'bin': {
                const op = node.op;
                if (op === '/') return { html: span('frac', span('fnum', go(node.l).html) + span('fden', go(node.r).html)), prec: 5 };
                if (op === '^') return { html: wrap(node.l, 6) + '<sup class="ec-sup">' + go(node.r).html + '</sup>', prec: 5 };
                if (op === '*') return { html: wrap(node.l, 3) + span('op', '·') + wrap(node.r, 3), prec: 3 };
                if (op === '+') return { html: wrap(node.l, 2) + span('op', '+') + wrap(node.r, 2), prec: 2 };
                if (op === '-') return { html: wrap(node.l, 2) + span('op', '−') + wrap(node.r, 3), prec: 2 };
                return { html: wrap(node.l, 2) + span('op', SIGNS[op] || escape(op)) + wrap(node.r, 2), prec: 1 };
            }
            case 'call': {
                const args = node.args.map(arg => go(arg).html);
                if (node.name === 'sqrt') return { html: span('sqrt', span('rad', '√') + span('radb', args[0])), prec: 5 };
                if (node.name === 'abs') return { html: '|' + args[0] + '|', prec: 6 };
                const name = node.name === 'log10' ? 'log<sub>10</sub>' : escape(node.name);
                return { html: span('fn', name) + '(' + args.join(', ') + ')', prec: 6 };
            }
            case 'eq': return { html: node.parts.map(part => go(part).html).join(span('eq', '=')), prec: 0 };
            default: return { html: escape(node.t), prec: 6 };
        }
    }

    const formulaHTML = (ast, ctx) => render(ast, ctx || {}, false).html;

    // The live line: "label = substituted right side = value". The first part of an equation that is a bare variable
    // keeps its label, and its own value is appended at the end.
    function liveHTML(ast, ctx) {
        const context = ctx || {};
        if (ast.t !== 'eq') return render(ast, context, true).html;
        const lead = ast.parts[0], target = lead.t === 'id' ? lead.name : '';
        const left = render(lead, context, !target).html;
        const rest = ast.parts.slice(1).map(part => render(part, context, true).html);
        const own = target && context.valueOf ? context.valueOf(target) : null;
        return [left].concat(rest, own ? [span('v', own.html)] : []).join(span('eq', '='));
    }

    window.ECalcTypeset = { escape, labelHTML, formulaHTML, liveHTML };
})();
```

- [ ] **Step 4: Run the parity test and see it pass**

```powershell
npm run test:ecalc
```

Expected: `ecalc core tests: 686 cases ok` (the number is Part A's vector count at its commit 24e420cbe; a different count is fine, an `AssertionError` is not: its message names the vector, for example `expr round(2.5): want 3, got 2`).

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/apps/ecalc/ecalc-expr.js ui/js/desktop/apps/ecalc/ecalc-units.js ui/js/desktop/apps/ecalc/ecalc-eseries.js ui/js/desktop/apps/ecalc/ecalc-solve.js ui/js/desktop/apps/ecalc/ecalc-typeset.js scripts/test-ecalc-core.mjs package.json
git commit -m "feat(ecalc): JavaScript mirror of the expression language, units, E-series and solver with parity tests

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C2: Schematic symbol library, the schematics and their catalog cross-check

**Files:**
- Create: `ui/js/desktop/apps/ecalc/ecalc-symbols.js`, `ui/js/desktop/apps/ecalc/ecalc-schematics.js`
- Create: `ui/desktop_ecalc_schematics_test.go`

`ecalc-schematics.js` must be exactly `window.ECalcSchematics = {JSON};` (the Go test parses the JSON body, strictly: an unknown key fails). The grid is 20 px per unit; `anchors` place the field chips (`side` is the side of the point on which the chip sits). Part A's catalog marks `ohms_law`, `voltage_divider`, `loaded_divider`, `current_divider`, `series_parallel`, `wheatstone` and `star_delta` with a schematic id; the other four phase 1 calculators have `""`.

- [ ] **Step 1: Write the failing test**

Create `ui/desktop_ecalc_schematics_test.go`:

```go
package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/ecalc"
)

const (
	ecalcSchematicsPath = "js/desktop/apps/ecalc/ecalc-schematics.js"
	ecalcSymbolsPath    = "js/desktop/apps/ecalc/ecalc-symbols.js"
)

// ecalcSchematic is the schema of one entry of window.ECalcSchematics; unknown fields fail the decode, so a typo in a
// key cannot silently drop part of a drawing.
type ecalcSchematic struct {
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Parts []struct {
		Sym  string  `json:"sym"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Rot  int     `json:"rot"`
		Flip bool    `json:"flip"`
		Text string  `json:"text"`
		W    float64 `json:"w"`
		H    float64 `json:"h"`
		Pins []struct {
			N string  `json:"n"`
			S string  `json:"s"`
			Y float64 `json:"y"`
		} `json:"pins"`
	} `json:"parts"`
	Wires     [][][2]float64 `json:"wires"`
	Dots      [][2]float64   `json:"dots"`
	Terminals [][2]float64   `json:"terminals"`
	Arrows    []struct {
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
		Dir int     `json:"dir"`
	} `json:"arrows"`
	Texts []struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Text   string  `json:"text"`
		Anchor string  `json:"anchor"`
	} `json:"texts"`
	Anchors []struct {
		Var  string  `json:"var"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Side string  `json:"side"`
	} `json:"anchors"`
}

// ecalcCatalogItem is the part of the catalog JSON of the server route (contract section 7) the cross-check reads.
type ecalcCatalogItem struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Schematic string `json:"schematic"`
	Vars      []struct {
		ID string `json:"id"`
	} `json:"vars"`
}

func loadECalcSchematics(t *testing.T) map[string]ecalcSchematic {
	t.Helper()
	text := strings.TrimSpace(readDesktopAssetText(t, ecalcSchematicsPath))
	const prefix = "window.ECalcSchematics = {"
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, "};") {
		t.Fatalf("%s must be exactly `window.ECalcSchematics = {JSON};`", ecalcSchematicsPath)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(text, "window.ECalcSchematics = "), ";")
	decoder := json.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.DisallowUnknownFields()
	var specs map[string]ecalcSchematic
	if err := decoder.Decode(&specs); err != nil {
		t.Fatalf("%s is not strict JSON of the schematic schema: %v", ecalcSchematicsPath, err)
	}
	return specs
}

func loadECalcCatalogItems(t *testing.T) []ecalcCatalogItem {
	t.Helper()
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatalf("ecalc.Default: %v", err)
	}
	body, _, err := catalog.LocalizedJSON("en")
	if err != nil {
		t.Fatalf("LocalizedJSON: %v", err)
	}
	var parsed struct {
		Items []ecalcCatalogItem `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("catalog JSON: %v", err)
	}
	return parsed.Items
}

// Every schematic belongs to a calculator, every calculator that names a schematic has one, and every field anchor
// names a variable of its calculator; the drawing itself stays inside its grid and uses symbols the library has.
func TestECalcSchematicsMatchTheCatalog(t *testing.T) {
	t.Parallel()
	specs := loadECalcSchematics(t)
	symbols := readDesktopAssetText(t, ecalcSymbolsPath)
	users := map[string][]ecalcCatalogItem{}
	for _, item := range loadECalcCatalogItems(t) {
		if item.Kind == "calculator" && item.Schematic != "" {
			users[item.Schematic] = append(users[item.Schematic], item)
		}
	}
	for id := range users {
		if _, ok := specs[id]; !ok {
			t.Errorf("the catalog names the schematic %q but %s has no entry for it", id, ecalcSchematicsPath)
		}
	}
	for id, spec := range specs {
		owners := users[id]
		if len(owners) == 0 {
			t.Errorf("schematic %q belongs to no calculator of the catalog", id)
			continue
		}
		inside := func(what string, x, y float64) {
			if x < 0 || y < 0 || x > spec.W || y > spec.H {
				t.Errorf("schematic %q: %s at (%v, %v) lies outside the %vx%v grid", id, what, x, y, spec.W, spec.H)
			}
		}
		for _, part := range spec.Parts {
			if !strings.Contains(symbols, "        "+part.Sym+": {") {
				t.Errorf("schematic %q uses the unknown symbol %q", id, part.Sym)
			}
			inside("part "+part.Sym, part.X, part.Y)
		}
		for _, wire := range spec.Wires {
			if len(wire) < 2 {
				t.Errorf("schematic %q has a wire with fewer than two points", id)
			}
			for _, point := range wire {
				inside("wire point", point[0], point[1])
			}
		}
		seen := map[string]bool{}
		for _, anchor := range spec.Anchors {
			if seen[anchor.Var] {
				t.Errorf("schematic %q anchors %q twice", id, anchor.Var)
			}
			seen[anchor.Var] = true
			if anchor.Side != "left" && anchor.Side != "right" && anchor.Side != "top" && anchor.Side != "bottom" {
				t.Errorf("schematic %q anchor %q has the side %q", id, anchor.Var, anchor.Side)
			}
			inside("anchor "+anchor.Var, anchor.X, anchor.Y)
			for _, owner := range owners {
				found := false
				for _, v := range owner.Vars {
					found = found || v.ID == anchor.Var
				}
				if !found {
					t.Errorf("schematic %q anchors %q, which is no variable of the calculator %q", id, anchor.Var, owner.ID)
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

```powershell
go test ./ui -run TestECalcSchematicsMatchTheCatalog -count=1
```

Expected: FAIL, `read js/desktop/apps/ecalc/ecalc-schematics.js: open ...: no such file or directory`.

- [ ] **Step 3: Create the symbol library and the schematics**

`ui/js/desktop/apps/ecalc/ecalc-symbols.js` (resistor in IEC box and ANSI zigzag, capacitor, polarized capacitor, inductor, diode, LED, Zener, crystal, fuse, DC and AC voltage source, current source, battery, meter, ground, antenna, NPN, PNP, N- and P-MOSFET, op-amp, transformer, generic IC box with pin names; all strokes `currentColor`):

```javascript
/* ECalc schematic symbols: the IEC (default) and ANSI symbol library and the renderer that turns a schematic spec into
   inline SVG. DOM-free classic script that publishes window.ECalcSymbols. Every stroke is currentColor, so a schematic
   follows the theme.

   Geometry: one grid unit is 20 px. A symbol is drawn in its own frame with pin 1 at the origin; a part places it with
   {sym, x, y, rot, flip} (x and y in grid units, rot in degrees clockwise, flip mirrors it across its own x axis
   before the rotation). Two-terminal symbols run from pin a (0,0) to pin b (3,0); a is the positive side of sources,
   batteries and polarised capacitors and the anode of diodes, so rot 90 stands them upright with a on top.

   A schematic spec is {w, h, parts, wires, dots, terminals, arrows, texts, anchors}: w and h in grid units, wires as
   polylines of [x, y] points, dots (junctions), terminals (open circles), arrows ({x, y, dir} with dir in degrees,
   0 = right, 90 = down) and texts ({x, y, text, anchor}). `anchors` ({var, x, y, side}) is read by the calculator
   page, not drawn here. */
(function () {
    'use strict';

    const G = 20;
    const escape = text => String(text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const num = x => String(Math.round(x * 100) / 100);
    const path = d => '<path d="' + d + '"/>';
    const solid = d => '<path d="' + d + '" fill="currentColor" stroke-width="1"/>';

    // A filled arrow head whose tip is at (x, y) and that points along (dx, dy).
    function head(x, y, dx, dy, length, half) {
        const size = Math.hypot(dx, dy), ux = dx / size, uy = dy / size, bx = x - length * ux, by = y - length * uy;
        return solid('M' + num(x) + ' ' + num(y) + 'L' + num(bx - half * uy) + ' ' + num(by + half * ux) + 'L' + num(bx + half * uy) + ' ' + num(by - half * ux) + 'Z');
    }

    const twoPins = () => ({ a: [0, 0], b: [3, 0] });
    const threePins = () => ({ b: [0, 0], c: [3, -2], e: [3, 2] });
    const fetPins = () => ({ g: [0, 0], d: [3, -2], s: [3, 2] });
    const diodeBase = () => path('M0 0H22M38 0H60') + path('M22 -10L38 0L22 10Z') + path('M38 -10V10');
    const plus = (x, y) => path('M' + (x - 3) + ' ' + y + 'h6M' + x + ' ' + (y - 3) + 'v6');
    // Markup centred on (x, y) that stays upright when the part is rotated or flipped (the signs of a source, a letter).
    const upright = (part, x, y, inner) => '<g transform="translate(' + x + ' ' + y + ')' + (part.rot ? ' rotate(' + -part.rot + ')' : '') + (part.flip ? ' scale(1 -1)' : '') + '">' + inner + '</g>';

    const SYMBOLS = {
        resistor: {
            pins: twoPins,
            draw: standard => (standard === 'ansi'
                ? path('M0 0H10L14 -8L22 8L30 -8L38 8L46 -8L50 0H60')
                : path('M0 0H10M50 0H60') + '<rect x="10" y="-7" width="40" height="14"/>')
        },
        capacitor: { pins: twoPins, draw: () => path('M0 0H27M33 0H60') + path('M27 -12V12M33 -12V12') },
        polarized: {
            pins: twoPins,
            draw: standard => path('M0 0H27M37 0H60') + path('M27 -12V12') + (standard === 'ansi' ? path('M33 -12Q41 0 33 12') : '<rect x="33" y="-12" width="4" height="24"/>') + plus(19, -9)
        },
        inductor: { pins: twoPins, draw: () => path('M0 0H10M50 0H60') + path('M10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0') },
        diode: { pins: twoPins, draw: diodeBase },
        led: { pins: twoPins, draw: () => diodeBase() + path('M26 -14L36 -26') + head(36, -26, 10, -12, 7, 3) + path('M35 -10L45 -22') + head(45, -22, 10, -12, 7, 3) },
        zener: { pins: twoPins, draw: () => path('M0 0H22M38 0H60') + path('M22 -10L38 0L22 10Z') + path('M42 -14L38 -10V10L34 14') },
        crystal: { pins: twoPins, draw: () => path('M0 0H22M38 0H60') + path('M22 -12V12M38 -12V12') + '<rect x="26" y="-8" width="8" height="16"/>' },
        fuse: { pins: twoPins, draw: () => path('M0 0H10M50 0H60') + '<rect x="10" y="-7" width="40" height="14"/>' + path('M10 0H50') },
        source: { pins: twoPins, draw: (standard, part) => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + upright(part, 22, 0, plus(0, 0)) + upright(part, 38, 0, path('M-3 0h6')) },
        acsource: { pins: twoPins, draw: () => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + path('M21 0Q25.5 -9 30 0T39 0') },
        isource: { pins: twoPins, draw: () => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + path('M21 0H38') + head(40, 0, 1, 0, 7, 4) },
        battery: { pins: twoPins, draw: () => path('M0 0H20M40 0H60') + path('M20 -12V12M26 -7V7M34 -12V12M40 -7V7') },
        meter: { pins: twoPins, draw: (standard, part) => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + upright(part, 30, 0, '<text x="0" y="4.5" text-anchor="middle" class="ec-sch-text" stroke="none" fill="currentColor">' + escape(part.text || 'V') + '</text>') },
        ground: { pins: () => ({ g: [0, 0] }), draw: () => path('M0 0V10M-12 10H12M-8 15H8M-4 20H4') },
        antenna: { pins: () => ({ a: [0, 0] }), draw: () => path('M0 0V-26M-13 -44L0 -26L13 -44') },
        npn: {
            pins: threePins,
            draw: () => path('M0 0H34M34 -16V16M34 -8L60 -24V-40M34 8L60 24V40') + head(56, 21.5, 26, 16, 8, 3.5)
        },
        pnp: {
            pins: threePins,
            draw: () => path('M0 0H34M34 -16V16M34 -8L60 -24V-40M34 8L60 24V40') + head(40, 12.3, -26, -16, 8, 3.5)
        },
        nmos: {
            pins: fetPins,
            draw: () => path('M0 0H24M24 -14V14M30 -16V-9M30 -4V4M30 9V16M30 -12H60V-40M30 12H60V40M30 0H46V12') + head(32, 0, -1, 0, 7, 3.5)
        },
        pmos: {
            pins: fetPins,
            draw: () => path('M0 0H24M24 -14V14M30 -16V-9M30 -4V4M30 9V16M30 -12H60V-40M30 12H60V40M30 0H46V12') + head(44, 0, 1, 0, 7, 3.5)
        },
        opamp: {
            pins: () => ({ inn: [0, -1], inp: [0, 1], out: [5, 0] }),
            draw: () => path('M20 -40L20 40L100 0Z') + path('M0 -20H20M0 20H20') + path('M26 -20h8') + plus(30, 20)
        },
        transformer: {
            pins: () => ({ p1: [0, 0], p2: [0, 3], s1: [4, 0], s2: [4, 3] }),
            draw: () => path('M0 0H22M0 60H22M80 0H58M80 60H58') + path('M22 0a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15')
                + path('M58 0a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15') + path('M36 0V60M44 0V60')
        },
        // A generic IC box: part.w and part.h in grid units (the box without its pin stubs), part.pins [{n, s: 'l'|'r', y}]
        // with y in grid units from the top edge, part.text the title. Pin points sit at the outer ends of the stubs.
        ic: {
            pins: part => {
                const out = {};
                for (const pin of part.pins || []) out[pin.n] = [pin.s === 'r' ? part.w + 2 : 0, pin.y];
                return out;
            },
            draw: (standard, part) => {
                let markup = '<rect x="' + G + '" y="0" width="' + part.w * G + '" height="' + part.h * G + '"/>';
                for (const pin of part.pins || []) {
                    const right = pin.s === 'r', y = pin.y * G;
                    markup += path(right ? 'M' + (part.w + 1) * G + ' ' + y + 'H' + (part.w + 2) * G : 'M0 ' + y + 'H' + G);
                    markup += '<text x="' + num(right ? (part.w + 1) * G - 4 : G + 4) + '" y="' + num(y + 3.5) + '" text-anchor="' + (right ? 'end' : 'start') + '" class="ec-sch-pin" stroke="none" fill="currentColor">' + escape(pin.n) + '</text>';
                }
                if (part.text) markup += '<text x="' + num((part.w + 2) * G / 2) + '" y="-6" text-anchor="middle" class="ec-sch-text" stroke="none" fill="currentColor">' + escape(part.text) + '</text>';
                return markup;
            }
        }
    };

    // Label text with a subscript behind the first underscore: V_out -> V<tspan>out</tspan>.
    function textMarkup(text) {
        const label = String(text), at = label.indexOf('_');
        if (at <= 0) return escape(label);
        return escape(label.slice(0, at)) + '<tspan class="ec-sch-sub" dy="3">' + escape(label.slice(at + 1)) + '</tspan>';
    }

    function partMarkup(part, standard) {
        const symbol = SYMBOLS[part.sym];
        if (!symbol) return '';
        const transform = 'translate(' + num(part.x * G) + ' ' + num(part.y * G) + ')' + (part.rot ? ' rotate(' + part.rot + ')' : '') + (part.flip ? ' scale(1 -1)' : '');
        return '<g transform="' + transform + '">' + symbol.draw(standard, part) + '</g>';
    }

    // The grid position of a pin of a placed part (rotation and flip applied); used by tests and by schematic authors.
    function pinPoint(part, name) {
        const symbol = SYMBOLS[part.sym], pin = symbol && symbol.pins(part)[name];
        if (!pin) return null;
        const radians = (part.rot || 0) * Math.PI / 180, x = pin[0], y = part.flip ? -pin[1] : pin[1];
        const round = value => Math.round(value * 1e6) / 1e6;
        return [round(part.x + x * Math.cos(radians) - y * Math.sin(radians)), round(part.y + x * Math.sin(radians) + y * Math.cos(radians))];
    }

    // Inline SVG of a schematic spec. `standard` is 'iec' (default) or 'ansi'; `label` becomes the accessible name.
    function svg(spec, options) {
        const standard = options && options.standard === 'ansi' ? 'ansi' : 'iec', label = (options && options.label) || '';
        const width = spec.w * G, height = spec.h * G;
        const point = p => num(p[0] * G) + ' ' + num(p[1] * G);
        let body = '';
        for (const wire of spec.wires || []) body += path('M' + wire.map(point).join('L'));
        for (const part of spec.parts || []) body += partMarkup(part, standard);
        for (const dot of spec.dots || []) body += '<circle class="ec-dot" cx="' + num(dot[0] * G) + '" cy="' + num(dot[1] * G) + '" r="3.2" fill="currentColor" stroke="none"/>';
        for (const end of spec.terminals || []) body += '<circle class="ec-term" cx="' + num(end[0] * G) + '" cy="' + num(end[1] * G) + '" r="4"/>';
        for (const arrow of spec.arrows || []) {
            const radians = (arrow.dir || 0) * Math.PI / 180;
            body += head(arrow.x * G + 4.5 * Math.cos(radians), arrow.y * G + 4.5 * Math.sin(radians), Math.cos(radians), Math.sin(radians), 9, 4);
        }
        for (const text of spec.texts || []) {
            body += '<text class="ec-sch-text" x="' + num(text.x * G) + '" y="' + num(text.y * G) + '" text-anchor="' + escape(text.anchor || 'middle') + '" stroke="none" fill="currentColor">' + textMarkup(text.text) + '</text>';
        }
        return '<svg class="ec-sch" viewBox="0 0 ' + width + ' ' + height + '" role="img" aria-label="' + escape(label) + '" focusable="false" preserveAspectRatio="xMidYMid meet" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';
    }

    window.ECalcSymbols = { GRID: G, names: () => Object.keys(SYMBOLS), pins: (sym, part) => (SYMBOLS[sym] ? SYMBOLS[sym].pins(part || {}) : null), pinPoint, symbolMarkup: (part, standard) => partMarkup(part, standard), svg };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-schematics.js`:

```javascript
window.ECalcSchematics = {
 "ohms_law": {
  "w": 22, "h": 10,
  "parts": [
   {"sym": "source", "x": 8, "y": 5, "rot": 90},
   {"sym": "resistor", "x": 12, "y": 4}
  ],
  "wires": [[[8, 5], [8, 4], [12, 4]], [[15, 4], [19, 4], [19, 9]], [[8, 8], [8, 9], [19, 9]]],
  "arrows": [
   {"x": 17, "y": 4, "dir": 0}
  ],
  "anchors": [
   {"var": "u", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "r", "x": 13.5, "y": 3.3, "side": "top"},
   {"var": "i", "x": 13.5, "y": 5, "side": "bottom"}
  ]
 },
 "voltage_divider": {
  "w": 27, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 5, "rot": 90},
   {"sym": "ground", "x": 12, "y": 9}
  ],
  "wires": [[[8, 3], [8, 2], [16, 2]], [[8, 6], [8, 9], [16, 9]], [[16, 8], [16, 9]], [[16, 5], [19, 5]], [[16, 9], [19, 9]]],
  "dots": [[16, 5], [12, 9], [16, 9]],
  "terminals": [[19, 5], [19, 9]],
  "anchors": [
   {"var": "vin", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "r1", "x": 15.6, "y": 3.65, "side": "left"},
   {"var": "r2", "x": 15.6, "y": 6.6, "side": "left"},
   {"var": "vout", "x": 19.8, "y": 7, "side": "right"}
  ]
 },
 "loaded_divider": {
  "w": 32, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 5, "rot": 90},
   {"sym": "resistor", "x": 20, "y": 5, "rot": 90},
   {"sym": "ground", "x": 12, "y": 9}
  ],
  "wires": [[[8, 3], [8, 2], [16, 2]], [[8, 6], [8, 9], [16, 9]], [[16, 8], [16, 9]], [[16, 5], [24, 5]], [[16, 9], [24, 9]], [[20, 8], [20, 9]]],
  "dots": [[16, 5], [20, 5], [12, 9], [16, 9], [20, 9]],
  "terminals": [[24, 5], [24, 9]],
  "anchors": [
   {"var": "vin", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "r1", "x": 15.6, "y": 3.65, "side": "left"},
   {"var": "r2", "x": 15.6, "y": 6.6, "side": "left"},
   {"var": "rl", "x": 20, "y": 4.6, "side": "top"},
   {"var": "vout", "x": 24.8, "y": 7, "side": "right"}
  ]
 },
 "current_divider": {
  "w": 25, "h": 11,
  "parts": [
   {"sym": "isource", "x": 8, "y": 6, "rot": 270},
   {"sym": "resistor", "x": 16, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 24, "y": 3, "rot": 90}
  ],
  "wires": [[[8, 3], [8, 2], [24, 2], [24, 3]], [[16, 2], [16, 3]], [[8, 6], [8, 9], [24, 9], [24, 6]], [[16, 6], [16, 9]]],
  "dots": [[16, 2], [16, 9]],
  "arrows": [
   {"x": 12, "y": 2, "dir": 0},
   {"x": 16, "y": 7.5, "dir": 90},
   {"x": 24, "y": 7.5, "dir": 90}
  ],
  "anchors": [
   {"var": "i", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "r1", "x": 15.6, "y": 4.5, "side": "left"},
   {"var": "i1", "x": 15.6, "y": 7.5, "side": "left"},
   {"var": "r2", "x": 23.6, "y": 4.5, "side": "left"},
   {"var": "i2", "x": 23.6, "y": 7.5, "side": "left"}
  ]
 },
 "series_parallel": {
  "w": 23, "h": 14,
  "parts": [
   {"sym": "resistor", "x": 5, "y": 5},
   {"sym": "resistor", "x": 10, "y": 5},
   {"sym": "resistor", "x": 15, "y": 5},
   {"sym": "resistor", "x": 9, "y": 9},
   {"sym": "resistor", "x": 9, "y": 11},
   {"sym": "resistor", "x": 9, "y": 13}
  ],
  "wires": [[[3, 5], [5, 5]], [[8, 5], [10, 5]], [[13, 5], [15, 5]], [[18, 5], [20, 5]], [[5, 11], [7, 11]], [[7, 9], [7, 13]], [[7, 9], [9, 9]], [[7, 13], [9, 13]], [[7, 11], [9, 11]], [[12, 9], [14, 9]], [[12, 13], [14, 13]], [[12, 11], [14, 11]], [[14, 9], [14, 13]], [[14, 11], [16, 11]]],
  "dots": [[7, 11], [14, 11]],
  "terminals": [[3, 5], [20, 5], [5, 11], [16, 11]],
  "texts": [
   {"x": 20.8, "y": 5.35, "text": "R_S", "anchor": "start"},
   {"x": 16.8, "y": 11.35, "text": "R_P", "anchor": "start"}
  ],
  "anchors": [
   {"var": "rs", "x": 11.5, "y": 3.6, "side": "top"}
  ]
 },
 "wheatstone": {
  "w": 32, "h": 11,
  "parts": [
   {"sym": "source", "x": 8, "y": 3, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 16, "y": 5, "rot": 90},
   {"sym": "resistor", "x": 24, "y": 2, "rot": 90},
   {"sym": "resistor", "x": 24, "y": 5, "rot": 90},
   {"sym": "meter", "x": 18.5, "y": 5, "text": "V"}
  ],
  "wires": [[[8, 3], [8, 2], [24, 2]], [[8, 6], [8, 9], [24, 9]], [[16, 8], [16, 9]], [[24, 8], [24, 9]], [[16, 5], [18.5, 5]], [[21.5, 5], [24, 5]]],
  "dots": [[16, 5], [24, 5]],
  "anchors": [
   {"var": "vs", "x": 7.4, "y": 4.5, "side": "left"},
   {"var": "r1", "x": 15.6, "y": 3.65, "side": "left"},
   {"var": "r2", "x": 15.6, "y": 6.6, "side": "left"},
   {"var": "r3", "x": 24.6, "y": 3.5, "side": "right"},
   {"var": "r4", "x": 24.6, "y": 6.5, "side": "right"},
   {"var": "vd", "x": 20, "y": 5.8, "side": "bottom"}
  ]
 },
 "star_delta": {
  "w": 22, "h": 23,
  "parts": [
   {"sym": "resistor", "x": 10, "y": 4},
   {"sym": "resistor", "x": 10, "y": 9},
   {"sym": "resistor", "x": 8, "y": 5, "rot": 90},
   {"sym": "resistor", "x": 13, "y": 15, "rot": 90},
   {"sym": "resistor", "x": 8, "y": 19},
   {"sym": "resistor", "x": 15, "y": 19}
  ],
  "wires": [[[8, 4], [10, 4]], [[13, 4], [16, 4], [16, 6.5]], [[8, 9], [10, 9]], [[13, 9], [16, 9], [16, 6.5]], [[8, 4], [8, 5]], [[8, 8], [8, 9]], [[8, 4], [5, 4]], [[8, 9], [5, 9]], [[16, 6.5], [19, 6.5]], [[13, 14], [13, 15]], [[13, 18], [13, 19]], [[11, 19], [13, 19]], [[8, 19], [6, 19]], [[13, 19], [15, 19]], [[18, 19], [20, 19]]],
  "dots": [[8, 4], [8, 9], [16, 6.5], [13, 19]],
  "terminals": [[5, 4], [5, 9], [19, 6.5], [13, 14], [6, 19], [20, 19]],
  "texts": [
   {"x": 4.4, "y": 4.35, "text": "A", "anchor": "end"},
   {"x": 4.4, "y": 9.35, "text": "B", "anchor": "end"},
   {"x": 19.8, "y": 6.85, "text": "C", "anchor": "start"},
   {"x": 12.4, "y": 14.2, "text": "A", "anchor": "end"},
   {"x": 5.4, "y": 19.35, "text": "B", "anchor": "end"},
   {"x": 20.8, "y": 19.35, "text": "C", "anchor": "start"},
   {"x": 19.5, "y": 1.6, "text": "Δ", "anchor": "middle"},
   {"x": 19.5, "y": 14.2, "text": "Y", "anchor": "middle"}
  ],
  "anchors": [
   {"var": "rab", "x": 7.4, "y": 6.5, "side": "left"},
   {"var": "rca", "x": 11.5, "y": 3.6, "side": "top"},
   {"var": "rbc", "x": 11.5, "y": 9.4, "side": "bottom"},
   {"var": "ra", "x": 13.6, "y": 16.5, "side": "right"},
   {"var": "rb", "x": 9.2, "y": 18.6, "side": "top"},
   {"var": "rc", "x": 16.5, "y": 19.4, "side": "bottom"}
  ]
 }
};
```

- [ ] **Step 4: Run the test and see it pass**

```powershell
go test ./ui -run TestECalcSchematicsMatchTheCatalog -count=1
```

Expected: `ok  	aurago/ui`.

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/apps/ecalc/ecalc-symbols.js ui/js/desktop/apps/ecalc/ecalc-schematics.js ui/desktop_ecalc_schematics_test.go
git commit -m "feat(ecalc): IEC and ANSI symbol library and the schematics of the basics calculators

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C3: Store, calculator page, shell and composition root

**Files:**
- Create: `ui/js/desktop/apps/ecalc/ecalc-store.js`, `ui/js/desktop/apps/ecalc/ecalc-calc-page.js`, `ui/js/desktop/apps/ecalc/ecalc-shell.js`, `ui/js/desktop/apps/ecalc/ecalc.js`
- Create: `ui/desktop_ecalc_test.go` (this task: the helpers and four tests; Tasks C4 and C5 append one test each)

Behaviour that the files below implement (all of it is exercised by the browser test of Task C6):

- A field is an input (the user's value), computed (success tint), in error (danger tint with a message: parse, nonfinite, range) or pending (not enough values), and an input can be locked. Typing parses after 60 ms; a text that does not parse changes nothing and the field falls back to the last good value when it loses the focus. Focus shows the raw text of an input, blur the value formatted with the chosen significant digits and unit.
- The page lays itself out for the width of its host (the shell observes the host: a page must not observe itself, its layout changes its own height and would loop the `ResizeObserver`): the fields sit on the schematic only when the page is at least as wide as the drawing plus the stage padding, otherwise they are listed under it; the formula goes beside the stage only when there is room; under 760 px the action buttons show their icon only.
- The shell is one stack (rail → list → page, back button in the top bar) below 720 px; Ctrl/Cmd+K focuses the search and stops propagation, so the desktop Spotlight does not open; lists keep the decimal point in every language (the comma separates their items), single values use the decimal comma in the 12 locales of contract §4.

- [ ] **Step 1: Write the failing source-marker tests**

Create `ui/desktop_ecalc_test.go` with the helpers and the first four tests (Tasks C4 and C5 append `TestECalcStylesheetUsesOnlyTokens` and `TestECalcIsWiredIntoTheDesktopCore`):

```go
package ui

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// The ECalc app is a set of classic scripts in js/desktop/apps/ecalc/ plus css/desktop-app-ecalc.css. The tests here
// are source markers in the style of desktop_calculator_test.go; the behaviour is covered by scripts/test-ecalc-core.mjs
// (the core) and desktop_ecalc_browser_test.go (the page).

const ecalcDir = "js/desktop/apps/ecalc"

func ecalcScripts(t *testing.T) map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(Content, ecalcDir)
	if err != nil {
		t.Fatalf("read %s: %v", ecalcDir, err)
	}
	scripts := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			scripts[entry.Name()] = readDesktopAssetText(t, ecalcDir+"/"+entry.Name())
		}
	}
	if len(scripts) != 11 {
		t.Fatalf("%s holds %d scripts, want the 11 of the plan", ecalcDir, len(scripts))
	}
	return scripts
}

func assertECalcMarkersInOrder(t *testing.T, source string, markers []string) {
	t.Helper()
	offset := 0
	for _, marker := range markers {
		index := strings.Index(source[offset:], marker)
		if index < 0 {
			t.Fatalf("marker %q missing after offset %d", marker, offset)
		}
		offset += index + len(marker)
	}
}

func TestECalcDoesNotUseDynamicEvaluation(t *testing.T) {
	t.Parallel()
	forbidden := map[string]string{
		"Function call":           `\bFunction\s*\(`,
		"new Function":            `\bnew\s+Function\b`,
		"eval":                    `\beval\s*\(`,
		"globalThis.Function":     `\bglobalThis\s*\.\s*Function\b`,
		"window.Function":         `\bwindow\s*\.\s*Function\b`,
		"window bracket Function": `\bwindow\s*\[\s*['"]Function['"]\s*\]`,
		"constructor constructor": `\bconstructor\s*\.\s*constructor\b`,
		"string timer":            `\bset(?:Timeout|Interval)\s*\(\s*['"]`,
	}
	for name, script := range ecalcScripts(t) {
		for label, pattern := range forbidden {
			if regexp.MustCompile(pattern).MatchString(script) {
				t.Errorf("%s must not use %s", name, label)
			}
		}
	}
	expr := ecalcScripts(t)["ecalc-expr.js"]
	for _, marker := range []string{"function tokenize(", "function parseTokens(", "function evaluate(", "case 'sqrt': return Math.sqrt(x[0]);", "const ARITY = {", "throw fail('unknown function '"} {
		if !strings.Contains(expr, marker) {
			t.Errorf("ecalc-expr.js is missing the safe parser marker %q", marker)
		}
	}
}

// Every storage access sits in a try block, so a browser that blocks storage still gets a working app.
func TestECalcStorageIsGuarded(t *testing.T) {
	t.Parallel()
	for name, script := range ecalcScripts(t) {
		if name != "ecalc-store.js" && strings.Contains(script, "localStorage") {
			t.Errorf("%s touches localStorage; only ecalc-store.js may", name)
		}
	}
	store := ecalcScripts(t)["ecalc-store.js"]
	if !strings.Contains(store, "const KEY = 'aurago.ecalc.v1';") {
		t.Error("ecalc-store.js must keep its data under aurago.ecalc.v1")
	}
	lines := strings.Split(store, "\n")
	guarded := 0
	for i, line := range lines {
		if !strings.Contains(line, "backend()") || strings.Contains(line, "const backend") {
			continue
		}
		guarded++
		window := strings.Join(lines[max(0, i-2):i+1], "\n")
		if !strings.Contains(window, "try {") {
			t.Errorf("ecalc-store.js line %d reads or writes storage outside a try block: %s", i+1, strings.TrimSpace(line))
		}
	}
	if guarded != 2 {
		t.Errorf("ecalc-store.js has %d storage accesses, want the read and the write", guarded)
	}
	if strings.Count(store, "catch (error)") < 2 {
		t.Error("ecalc-store.js must catch the storage errors")
	}
}

// Ctrl/Cmd+K focuses the in-app search and must not reach the desktop's Spotlight shortcut on the document.
func TestECalcSearchShortcutStopsPropagation(t *testing.T) {
	t.Parallel()
	root := ecalcScripts(t)["ecalc.js"]
	assertECalcMarkersInOrder(t, root, []string{
		"root.addEventListener('keydown', event => {",
		"(event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k'",
		"event.preventDefault();",
		"event.stopPropagation();",
		"shell.focusSearch()",
	})
	for _, marker := range []string{"function dispose(id)", "window.ECalcApp = { render, dispose };", "ctx.api(catalogURL(lang))", "desktop.ecalc_load_failed", "desktop.ecalc_retry"} {
		if !strings.Contains(root, marker) {
			t.Errorf("ecalc.js is missing %q", marker)
		}
	}
	shell := ecalcScripts(t)["ecalc-shell.js"]
	for _, marker := range []string{"const COMPACT_BELOW = 720;", "ui.compact", "data-action=\"back\""} {
		if !strings.Contains(shell, marker) {
			t.Errorf("ecalc-shell.js is missing %q", marker)
		}
	}
}

// The app only speaks through desktop.ecalc_* and desktop.app_ecalc keys, and the dimension names that are looked up
// dynamically exist in every language.
func TestECalcUsesOnlyItsOwnTranslationKeys(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{}
	for _, key := range strings.Fields(`app_ecalc ecalc_search_placeholder ecalc_favorites ecalc_recent ecalc_empty_favorites ecalc_empty_recent
		ecalc_no_results ecalc_favorite_add ecalc_favorite_remove ecalc_copy_results ecalc_copied ecalc_reset ecalc_explanation ecalc_source
		ecalc_derived ecalc_eseries_nearest ecalc_eseries_series ecalc_lock ecalc_unlock ecalc_unit_auto ecalc_error_undetermined
		ecalc_error_nonfinite ecalc_error_range ecalc_error_parse ecalc_settings ecalc_settings_digits ecalc_settings_symbols ecalc_symbols_iec
		ecalc_symbols_ansi ecalc_back ecalc_load_failed ecalc_retry ecalc_warnings ecalc_inputs ecalc_computed ecalc_dim_voltage ecalc_dim_current
		ecalc_dim_resistance ecalc_dim_power ecalc_dim_charge ecalc_dim_energy ecalc_dim_number ecalc_dim_gain`) {
		allowed["desktop."+key] = true
	}
	keyPattern := regexp.MustCompile(`['"](desktop\.[A-Za-z0-9_.]+)['"]`)
	used := 0
	for name, script := range ecalcScripts(t) {
		for _, match := range keyPattern.FindAllStringSubmatch(script, -1) {
			used++
			key := match[1]
			if key == "desktop.ecalc_dim_" {
				continue // the prefix of the dimension names
			}
			if !allowed[key] {
				t.Errorf("%s uses the translation key %s, which is not one of the ECalc keys", name, key)
			}
		}
	}
	if used < 30 {
		t.Fatalf("found only %d translation keys in the ECalc scripts, the pattern no longer matches", used)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readMergedLangMap(t, lang)
		for _, dim := range []string{"voltage", "current", "resistance", "power", "charge", "energy", "number", "gain"} {
			if strings.TrimSpace(words["desktop.ecalc_dim_"+dim]) == "" {
				t.Errorf("%s misses desktop.ecalc_dim_%s, which the unit dropdown looks up dynamically", lang, dim)
			}
		}
	}
}
```

- [ ] **Step 2: Run them and see them fail**

```powershell
go test ./ui -run 'TestECalc(DoesNotUseDynamicEvaluation|StorageIsGuarded|SearchShortcutStopsPropagation|UsesOnlyItsOwnTranslationKeys)' -count=1
```

Expected: FAIL, `js/desktop/apps/ecalc holds 7 scripts, want the 11 of the plan` (the core, the typesetter, the symbols and the schematics exist, the four files of this task do not).

- [ ] **Step 3: Create the four files**

`ui/js/desktop/apps/ecalc/ecalc-store.js`:

```javascript
/* ECalc preferences: the last page, favorites, recents, unit choices, significant digits, the E-series, the symbol
   standard and the inputs of every calculator, kept in localStorage under aurago.ecalc.v1. Every storage access is
   wrapped in try/catch: with blocked or full storage the app works, it just forgets. Publishes window.ECalcStore.
   Persisted shape: {last, favorites[], recents[] (at most 12), units{"calc.var": unitId}, sig (3..6), eseries,
   symbols ("iec"|"ansi"), values{calcId: {inputs{var: raw text}, recency[], locked[], choices{}}}}. */
(function () {
    'use strict';

    const KEY = 'aurago.ecalc.v1';
    const SERIES = ['E6', 'E12', 'E24', 'E48', 'E96', 'E192'];
    const MAX_RECENTS = 12, MAX_FAVORITES = 200, MAX_CALCS = 200;

    const isText = value => typeof value === 'string';
    const textList = (value, limit) => [...new Set((Array.isArray(value) ? value : []).filter(isText))].slice(0, limit);
    const textMap = value => {
        const out = {};
        if (value && typeof value === 'object' && !Array.isArray(value)) for (const [key, entry] of Object.entries(value)) if (isText(entry)) out[key] = entry;
        return out;
    };

    function normalize(raw) {
        const source = raw && typeof raw === 'object' ? raw : {};
        const sig = Number(source.sig);
        const values = {};
        if (source.values && typeof source.values === 'object') {
            for (const [id, entry] of Object.entries(source.values).slice(0, MAX_CALCS)) {
                if (!entry || typeof entry !== 'object') continue;
                values[id] = { inputs: textMap(entry.inputs), recency: textList(entry.recency, 50), locked: textList(entry.locked, 50), choices: textMap(entry.choices) };
            }
        }
        return {
            last: isText(source.last) ? source.last : '',
            favorites: textList(source.favorites, MAX_FAVORITES),
            recents: textList(source.recents, MAX_RECENTS),
            units: textMap(source.units),
            sig: Number.isInteger(sig) && sig >= 3 && sig <= 6 ? sig : 4,
            eseries: SERIES.includes(source.eseries) ? source.eseries : 'E24',
            symbols: source.symbols === 'ansi' ? 'ansi' : 'iec',
            values
        };
    }

    // `storage` is injectable for tests; the default is window.localStorage, looked up inside the try block because
    // merely touching it can throw when the browser blocks storage.
    function create(storage) {
        const backend = () => storage || window.localStorage;
        let prefs;
        try {
            const raw = backend().getItem(KEY);
            prefs = normalize(raw ? JSON.parse(raw) : {});
        } catch (error) {
            prefs = normalize({});
        }
        let timer = 0;
        function save() {
            try { backend().setItem(KEY, JSON.stringify(prefs)); } catch (error) { /* blocked or full storage: keep going in memory */ }
        }
        // Keystrokes persist after a short pause; everything else saves at once.
        function saveSoon() {
            clearTimeout(timer);
            timer = setTimeout(() => { timer = 0; save(); }, 250);
        }
        function flush() {
            if (timer) { clearTimeout(timer); timer = 0; save(); }
        }
        return {
            prefs: () => prefs,
            set(key, value) {
                prefs = normalize(Object.assign({}, prefs, { [key]: value }));
                save();
            },
            isFavorite: id => prefs.favorites.includes(id),
            toggleFavorite(id) {
                const on = !prefs.favorites.includes(id);
                prefs.favorites = on ? textList(prefs.favorites.concat([id]), MAX_FAVORITES) : prefs.favorites.filter(entry => entry !== id);
                save();
                return on;
            },
            pushRecent(id) {
                prefs.recents = [id].concat(prefs.recents.filter(entry => entry !== id)).slice(0, MAX_RECENTS);
                prefs.last = id;
                save();
            },
            unitOf: (calcId, varId) => prefs.units[calcId + '.' + varId],
            setUnit(calcId, varId, unit) {
                if (unit) prefs.units[calcId + '.' + varId] = unit; else delete prefs.units[calcId + '.' + varId];
                save();
            },
            calcState: id => prefs.values[id] || null,
            saveCalc(id, state) {
                prefs.values[id] = { inputs: textMap(state.inputs), recency: textList(state.recency, 50), locked: textList(state.locked, 50), choices: textMap(state.choices) };
                saveSoon();
            },
            flush
        };
    }

    window.ECalcStore = { KEY, SERIES, create, normalize };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-calc-page.js`:

```javascript
/* ECalc calculator page (layout C): the schematic with the input fields placed on it, the typeset formula with its live
   line, the nearest E-series values, the derived quantities, warnings, the collapsible explanation with its source and
   the actions (favorite, copy results, reset). A field is an input (the user's value), computed (success tint), in
   error (danger tint with a message) and, for inputs, lockable. Typing parses after a short pause; a text that does not
   parse changes nothing. The state that decides which variables are inputs lives in ECalcCore.solve (see there);
   this file only draws it and feeds the typed texts into it. Publishes window.ECalcCalcPage. */
(function () {
    'use strict';

    const core = window.ECalcCore, Typeset = window.ECalcTypeset, Symbols = window.ECalcSymbols;
    const S = core.solve, esc = Typeset.escape;
    const has = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    const TYPING_PAUSE_MS = 60;
    const FORMULA_COLUMN = 324; // the formula column of the split layout plus its gap
    const STAGE_INSET = 26; // padding and border of the stage around the drawing
    const NARROW_BELOW = 760; // under this width the action buttons show their icon only

    // options: {calc, units, store, t, icon, uid, decimalComma, schematics, onFavorite}
    function create(options) {
        const { calc, units, store, t, icon, uid } = options;
        const varById = new Map(calc.vars.map(v => [v.id, v]));
        const derivedById = new Map(calc.derived.map(item => [item.id, item]));
        const spec = calc.schematic && options.schematics && options.schematics[calc.schematic] ? options.schematics[calc.schematic] : null;
        const anchors = new Map();
        if (spec) for (const anchor of spec.anchors || []) if (varById.has(anchor.var)) anchors.set(anchor.var, anchor);
        const formulas = calc.formula.map(line => core.expr.parseFormula(line));

        // The unit of a field: the stored choice when this catalog still offers it ('auto' is the automatic prefix),
        // else the default unit of the variable.
        const unitOf = id => {
            const v = varById.get(id), stored = store.unitOf(calc.id, id);
            const choice = stored === 'auto' ? '' : stored;
            if (v && stored !== undefined && units.options(v.dim).some(option => option.id === choice)) return choice;
            return v && v.unit ? v.unit : '';
        };
        const sig = () => store.prefs().sig;
        const comma = text => (options.decimalComma ? text.replace(/(\d)\.(\d)/g, '$1,$2') : text);
        // Lists keep the decimal point (the comma separates their items), single values follow the language.
        const display = (entry, value, unitId) => (Array.isArray(value)
            ? value.map(x => units.format(x, entry.dim, unitId, sig()).replace(/\s+/g, '')).join(', ')
            : comma(units.format(value, entry.dim, unitId, sig())));
        const finite = value => (Array.isArray(value) ? value.length > 0 && value.every(Number.isFinite) : Number.isFinite(value));

        let state = S.restoreState(calc, store.calcState(calc.id), units, unitOf);
        let result = null, parseErrors = {}, typing = {};
        const timers = new Map(), fields = new Map();
        let copyTimer = 0;

        function errorText(code) {
            switch (code) {
                case 'parse': return t('desktop.ecalc_error_parse');
                case 'nonfinite': return t('desktop.ecalc_error_nonfinite');
                case 'range': return t('desktop.ecalc_error_range');
                case 'undetermined': return t('desktop.ecalc_error_undetermined');
                default: return '';
            }
        }
        const persist = () => store.saveCalc(calc.id, state);

        // ---- markup --------------------------------------------------------------------------------------------

        function fieldMarkup(v) {
            const id = uid + '-' + v.id;
            const choices = units.options(v.dim);
            let unit = '';
            if (choices.length > 1) {
                unit = '<span class="ec-unit-wrap"><select class="ec-unit" aria-label="' + esc(t('desktop.ecalc_dim_' + v.dim)) + '" title="' + esc(t('desktop.ecalc_dim_' + v.dim)) + '">'
                    + choices.map(option => '<option value="' + esc(option.id) + '">' + esc(option.kind === 'auto' ? t('desktop.ecalc_unit_auto') : option.label) + '</option>').join('') + '</select></span>';
            } else {
                const symbol = units.dim(v.dim).symbol;
                if (symbol) unit = '<span class="ec-unit-static">' + esc(symbol) + '</span>';
            }
            return '<div class="ec-field" role="group" data-var="' + esc(v.id) + '" data-state="input" data-dim="' + esc(v.dim) + '"' + (v.list ? ' data-list="true"' : '') + ' aria-labelledby="' + id + '-label">'
                + '<div class="ec-field-head"><label class="ec-field-label" id="' + id + '-label" for="' + id + '" title="' + esc(v.name || '') + '">' + Typeset.labelHTML(v.label || v.id) + '</label>'
                + '<button type="button" class="ec-lock" aria-pressed="false" data-action="lock">' + icon('lock') + '</button></div>'
                + '<div class="ec-field-row"><input class="ec-input" id="' + id + '" type="text" inputmode="decimal" autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="' + id + '-msg">' + unit + '</div>'
                + '<div class="ec-field-msg" id="' + id + '-msg" aria-live="polite"></div></div>';
        }

        const choiceMarkup = choice => '<fieldset class="ec-choice" data-choice="' + esc(choice.id) + '"><legend>' + esc(choice.name) + '</legend><div class="ec-seg">'
            + choice.options.map(option => '<label><input type="radio" name="' + uid + '-' + esc(choice.id) + '" value="' + esc(option.id) + '"><span>' + esc(option.name) + '</span></label>').join('') + '</div></fieldset>';

        const docMarkup = String(calc.doc || '').split(/\n{2,}/).filter(part => part.trim()).map(part => '<p>' + esc(part.trim()) + '</p>').join('');
        const favoriteLabel = () => (store.isFavorite(calc.id) ? t('desktop.ecalc_favorite_remove') : t('desktop.ecalc_favorite_add'));

        const el = document.createElement('article');
        el.className = 'ec-page';
        el.dataset.calc = calc.id;
        el.dataset.layout = 'stack';
        el.dataset.chips = spec ? 'listed' : 'none';
        el.setAttribute('aria-labelledby', uid + '-title');
        el.innerHTML = '<header class="ec-page-head"><h2 class="ec-title" id="' + uid + '-title">' + esc(calc.name) + '</h2><div class="ec-actions">'
            + '<button type="button" class="ec-btn" data-action="favorite" aria-pressed="false">' + icon('star') + '<span class="ec-btn-label" data-label="favorite"></span></button>'
            + '<button type="button" class="ec-btn" data-action="copy" title="' + esc(t('desktop.ecalc_copy_results')) + '" aria-label="' + esc(t('desktop.ecalc_copy_results')) + '">' + icon('copy') + '<span class="ec-btn-label" data-label="copy">' + esc(t('desktop.ecalc_copy_results')) + '</span></button>'
            + '<button type="button" class="ec-btn" data-action="reset" title="' + esc(t('desktop.ecalc_reset')) + '" aria-label="' + esc(t('desktop.ecalc_reset')) + '">' + icon('reset') + '<span class="ec-btn-label">' + esc(t('desktop.ecalc_reset')) + '</span></button></div></header>'
            + ((calc.choices || []).length ? '<div class="ec-choices">' + calc.choices.map(choiceMarkup).join('') + '</div>' : '')
            + '<div class="ec-main"><section class="ec-stage" aria-label="' + esc(t('desktop.ecalc_inputs')) + '">'
            + (spec ? '<div class="ec-sch-wrap" style="--ec-w:' + spec.w + ';--ec-ar:' + spec.w + '/' + spec.h + '"><div class="ec-sch-art"></div><div class="ec-chips"></div></div>' : '')
            + '<div class="ec-fields"></div></section>'
            + '<section class="ec-math-card"><div class="ec-formulas">' + formulas.map((_, index) => '<div class="ec-formula" data-line="' + index + '"><div class="ec-math" data-math="formula"></div><div class="ec-math ec-live" data-math="live"></div></div>').join('') + '</div></section></div>'
            + '<div class="ec-below"><section class="ec-block" data-block="eseries"><h3 data-label="eseries"></h3><ul class="ec-rows" data-rows="eseries"></ul></section>'
            + '<section class="ec-block" data-block="derived"><h3>' + esc(t('desktop.ecalc_derived')) + '</h3><ul class="ec-rows" data-rows="derived"></ul></section></div>'
            + '<ul class="ec-warnings" aria-label="' + esc(t('desktop.ecalc_warnings')) + '"></ul>'
            + '<details class="ec-doc"><summary>' + esc(t('desktop.ecalc_explanation')) + '</summary><div class="ec-doc-body">' + docMarkup + '<p class="ec-source"><strong>' + esc(t('desktop.ecalc_source')) + ':</strong> ' + esc(calc.source || '') + '</p></div></details>';

        const chipsEl = el.querySelector('.ec-chips'), fieldsEl = el.querySelector('.ec-fields'), artEl = el.querySelector('.ec-sch-art');
        for (const v of calc.vars) {
            const holder = document.createElement('div');
            holder.innerHTML = fieldMarkup(v);
            const root = holder.firstElementChild, anchor = anchors.get(v.id);
            if (anchor && chipsEl) {
                root.dataset.side = anchor.side || 'right';
                root.style.setProperty('--ax', String(Math.round(anchor.x / spec.w * 10000) / 100));
                root.style.setProperty('--ay', String(Math.round(anchor.y / spec.h * 10000) / 100));
                chipsEl.appendChild(root);
            } else {
                fieldsEl.appendChild(root);
            }
            const refs = { v, root, input: root.querySelector('.ec-input'), select: root.querySelector('.ec-unit'), lock: root.querySelector('.ec-lock'), msg: root.querySelector('.ec-field-msg') };
            if (refs.select) refs.select.value = unitOf(v.id);
            fields.set(v.id, refs);
        }
        if (fieldsEl && !fieldsEl.children.length) fieldsEl.remove();
        if (spec && !chipsEl.children.length) chipsEl.remove();
        function drawSchematic() {
            if (spec && artEl) artEl.innerHTML = Symbols.svg(spec, { standard: store.prefs().symbols, label: calc.name });
        }
        drawSchematic();

        // ---- painting ------------------------------------------------------------------------------------------

        function compute() {
            const read = S.readInputs(calc, state, units, unitOf);
            parseErrors = read.errors;
            result = S.solve(calc, read.values, state.choices, units);
        }

        function paintField(refs, keep) {
            const v = refs.v, isInput = has(state.inputs, v.id) && v.input !== false, locked = state.locked.includes(v.id);
            const value = result.values[v.id];
            const code = typing[v.id] || parseErrors[v.id] || result.errors[v.id] || '';
            refs.root.dataset.state = code === 'undetermined' ? 'pending' : code ? 'error' : isInput ? 'input' : 'computed';
            refs.root.dataset.locked = locked ? 'true' : 'false';
            refs.lock.hidden = !isInput;
            refs.lock.setAttribute('aria-pressed', locked ? 'true' : 'false');
            const lockLabel = locked ? t('desktop.ecalc_unlock') : t('desktop.ecalc_lock');
            refs.lock.setAttribute('aria-label', lockLabel);
            refs.lock.title = lockLabel;
            refs.input.readOnly = v.input === false;
            refs.input.setAttribute('aria-invalid', code && code !== 'undetermined' ? 'true' : 'false');
            refs.msg.textContent = errorText(code);
            const focused = refs.input.ownerDocument.activeElement === refs.input;
            if (focused && keep) return;
            let text;
            if (focused) text = isInput ? state.inputs[v.id] : finite(value) ? display(v, value, unitOf(v.id)) : '';
            else if (isInput && !parseErrors[v.id]) text = display(v, value, unitOf(v.id));
            else if (isInput) text = state.inputs[v.id];
            else text = finite(value) ? display(v, value, unitOf(v.id)) : '';
            if (refs.input.value !== text) refs.input.value = text;
        }

        const labelOf = name => {
            const entry = varById.get(name) || derivedById.get(name);
            return entry ? entry.label || name : '';
        };
        function typesetContext(live) {
            const consts = S.choiceConsts(calc, state.choices);
            const ctx = { labelOf, constOf: name => (has(consts, name) ? comma(units.fmtSig(consts[name], 4)) : undefined) };
            if (live) {
                ctx.valueOf = name => {
                    const entry = varById.get(name) || derivedById.get(name);
                    const value = entry && result ? result.values[name] : NaN;
                    if (!entry || !finite(value)) return null;
                    const text = display(entry, value, derivedById.has(name) ? entry.unit || '' : unitOf(name));
                    return { html: esc(text), prec: text.startsWith('-') ? 2 : /\s/.test(text) ? 3 : 6 };
                };
            }
            return ctx;
        }
        function paintFormulas(withFormula) {
            const lines = el.querySelectorAll('.ec-formula');
            formulas.forEach((ast, index) => {
                if (withFormula) lines[index].querySelector('[data-math="formula"]').innerHTML = Typeset.formulaHTML(ast, typesetContext(false));
                lines[index].querySelector('[data-math="live"]').innerHTML = Typeset.liveHTML(ast, typesetContext(true));
            });
        }

        function percent(dev) {
            const size = Math.abs(dev);
            if (size < 0.05) return comma('0.0 %');
            return comma((dev > 0 ? '+' : '−') + (size < 10 ? size.toFixed(1) : size.toFixed(0)) + ' %');
        }
        function paintResults() {
            const series = store.prefs().eseries;
            const eRows = calc.vars.filter(v => v.eseries).map(v => {
                const value = result.values[v.id], near = Number.isFinite(value) && value > 0 && !Array.isArray(value) ? core.eseries.nearest(series, value) : null;
                if (!near) return '';
                const tone = Math.abs(near.dev) < 1 ? 'ok' : Math.abs(near.dev) >= 5 ? 'warn' : 'mid';
                return '<li><span class="ec-row-name">' + Typeset.labelHTML(v.label || v.id) + '</span><span class="ec-row-val">' + esc(display(v, near.value, unitOf(v.id))) + '</span><span class="ec-row-dev" data-tone="' + tone + '">' + esc(percent(near.dev)) + '</span></li>';
            }).join('');
            el.querySelector('[data-block="eseries"]').hidden = !eRows;
            el.querySelector('[data-rows="eseries"]').innerHTML = eRows;
            el.querySelector('[data-label="eseries"]').textContent = t('desktop.ecalc_eseries_nearest', { series });
            const dRows = calc.derived.map(item => {
                const value = result.values[item.id];
                return '<li><span class="ec-row-name" title="' + esc(item.name || '') + '">' + Typeset.labelHTML(item.label || item.id) + '</span><span class="ec-row-val">'
                    + (finite(value) ? esc(display(item, value, item.unit || '')) : '–') + '</span></li>';
            }).join('');
            el.querySelector('[data-block="derived"]').hidden = !dRows;
            el.querySelector('[data-rows="derived"]').innerHTML = dRows;
            el.querySelector('.ec-warnings').innerHTML = result.warnings.map(index => '<li>' + icon('warning') + '<span>' + esc(calc.checks[index].warn) + '</span></li>').join('');
        }

        function paintChoices() {
            for (const choice of calc.choices || []) {
                for (const radio of el.querySelectorAll('[data-choice="' + choice.id + '"] input')) radio.checked = radio.value === state.choices[choice.id];
            }
        }
        function paintActions() {
            const favorite = store.isFavorite(calc.id), button = el.querySelector('[data-action="favorite"]');
            button.setAttribute('aria-pressed', favorite ? 'true' : 'false');
            button.querySelector('[data-label]').textContent = favoriteLabel();
            button.title = favoriteLabel();
            button.setAttribute('aria-label', favoriteLabel());
        }

        function paint(editing) {
            compute();
            for (const refs of fields.values()) paintField(refs, refs.v.id === editing);
            paintFormulas(false);
            paintResults();
        }

        // ---- events --------------------------------------------------------------------------------------------

        function readText(v, text, unitId) {
            return v.list ? units.parseList(text, v.dim, unitId) : units.parse(text, v.dim, unitId);
        }
        function commitText(refs) {
            const v = refs.v, text = refs.input.value;
            clearTimeout(timers.get(v.id));
            timers.delete(v.id);
            try {
                readText(v, text, unitOf(v.id));
            } catch (error) {
                typing[v.id] = 'parse';
                paintField(refs, true);
                return;
            }
            delete typing[v.id];
            if (has(state.inputs, v.id) && state.inputs[v.id] === text) { paintField(refs, true); return; }
            state = S.applyEdit(calc, state, v.id, text, units, unitOf, result.values);
            persist();
            paint(v.id);
        }

        el.addEventListener('input', event => {
            const refs = event.target.classList.contains('ec-input') ? fields.get(event.target.closest('.ec-field').dataset.var) : null;
            if (!refs || refs.v.input === false) return;
            clearTimeout(timers.get(refs.v.id));
            timers.set(refs.v.id, setTimeout(() => commitText(refs), TYPING_PAUSE_MS));
        });
        el.addEventListener('focusin', event => {
            if (!event.target.classList.contains('ec-input')) return;
            const refs = fields.get(event.target.closest('.ec-field').dataset.var);
            if (has(state.inputs, refs.v.id) && !typing[refs.v.id]) refs.input.value = state.inputs[refs.v.id];
            refs.input.select();
        });
        el.addEventListener('focusout', event => {
            if (!event.target.classList.contains('ec-input')) return;
            const refs = fields.get(event.target.closest('.ec-field').dataset.var);
            if (timers.has(refs.v.id)) commitText(refs);
            delete typing[refs.v.id];
            paintField(refs, false);
        });
        el.addEventListener('keydown', event => {
            if (event.key === 'Enter' && event.target.classList.contains('ec-input')) event.target.blur();
        });
        el.addEventListener('change', event => {
            const target = event.target;
            if (target.classList.contains('ec-unit')) {
                const refs = fields.get(target.closest('.ec-field').dataset.var), v = refs.v;
                let exact = null;
                if (has(state.inputs, v.id)) {
                    try { exact = readText(v, state.inputs[v.id], unitOf(v.id)); } catch (error) { exact = null; }
                }
                store.setUnit(calc.id, v.id, target.value || 'auto');
                if (exact !== null) state = Object.assign({}, state, { inputs: Object.assign({}, state.inputs, { [v.id]: S.exactText(v, exact, units, unitOf) }) });
                persist();
                paint();
            } else if (target.type === 'radio' && target.closest('.ec-choice')) {
                const id = target.closest('.ec-choice').dataset.choice;
                state = Object.assign({}, state, { choices: Object.assign({}, state.choices, { [id]: target.value }) });
                persist();
                paintFormulas(true);
                paint();
            }
        });

        function plainText() {
            const lines = [calc.name];
            for (const v of calc.vars) {
                const value = result.values[v.id];
                if (finite(value)) lines.push((v.label || v.id) + ' = ' + display(v, value, unitOf(v.id)));
            }
            for (const item of calc.derived) {
                const value = result.values[item.id];
                if (finite(value)) lines.push((item.label || item.id) + ' = ' + display(item, value, item.unit || ''));
            }
            return lines.join('\n');
        }
        function copyText(text) {
            const fallback = () => {
                const area = document.createElement('textarea');
                area.value = text;
                area.setAttribute('readonly', '');
                area.style.position = 'fixed';
                area.style.opacity = '0';
                document.body.appendChild(area);
                area.select();
                try { document.execCommand('copy'); } catch (error) { /* nothing more to try */ }
                area.remove();
            };
            if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).catch(fallback);
            else fallback();
        }

        el.addEventListener('click', event => {
            const button = event.target.closest('[data-action]');
            if (!button || !el.contains(button)) return;
            const action = button.dataset.action;
            if (action === 'lock') {
                state = S.toggleLock(state, button.closest('.ec-field').dataset.var);
                persist();
                paint();
            } else if (action === 'favorite') {
                store.toggleFavorite(calc.id);
                paintActions();
                if (options.onFavorite) options.onFavorite(calc.id);
            } else if (action === 'reset') {
                state = S.defaultState(calc, units, unitOf);
                typing = {};
                persist();
                paintChoices();
                paintFormulas(true);
                paint();
            } else if (action === 'copy') {
                copyText(plainText());
                const label = button.querySelector('[data-label]');
                button.dataset.done = 'true';
                label.textContent = t('desktop.ecalc_copied');
                button.title = t('desktop.ecalc_copied');
                clearTimeout(copyTimer);
                copyTimer = setTimeout(() => {
                    delete button.dataset.done;
                    label.textContent = t('desktop.ecalc_copy_results');
                    button.title = t('desktop.ecalc_copy_results');
                }, 1600);
            }
        });

        // The split layout needs room for the formula next to the stage; the fields sit on the schematic only when the
        // stage is at least as wide as the drawing (so the chips never crowd), otherwise they are listed under it. The
        // shell calls this whenever the page host changes its size (a page must not observe itself: its own layout
        // changes its height).
        function layout() {
            const width = el.clientWidth;
            if (!width) return;
            const need = spec ? spec.w * Symbols.GRID + STAGE_INSET : 0;
            el.dataset.layout = (spec ? width >= need + FORMULA_COLUMN : width >= NARROW_BELOW) ? 'split' : 'stack';
            el.dataset.narrow = width < NARROW_BELOW ? 'true' : 'false';
            if (spec) el.dataset.chips = width >= need ? 'anchored' : 'listed';
        }

        paintChoices();
        paintActions();
        paintFormulas(true);
        paint();

        return {
            el,
            layout,
            // The settings changed (digits, E-series, symbol standard): draw everything again.
            refresh() {
                drawSchematic();
                paintActions();
                paintFormulas(true);
                paint();
            },
            focusFirst() {
                const first = [...fields.values()].find(refs => has(state.inputs, refs.v.id));
                if (first) first.input.focus();
            },
            dispose() {
                for (const timer of timers.values()) clearTimeout(timer);
                timers.clear();
                clearTimeout(copyTimer);
                store.flush();
            }
        };
    }

    window.ECalcCalcPage = { create };
})();
```

`ui/js/desktop/apps/ecalc/ecalc-shell.js`:

```javascript
/* ECalc shell: the top bar with the search box and the settings popover, the category rail (Favorites, Recent, then the
   categories that have items), the item list and the page host. Below 720 px of width the three columns collapse into
   one stack (rail, list, page) with a back button. Search covers the name in the current language, the English name and
   the catalog tags. Publishes window.ECalcShell; ecalc.js owns the data and the window lifecycle. */
(function () {
    'use strict';

    const core = window.ECalcCore, Typeset = window.ECalcTypeset;
    const esc = Typeset.escape;
    const COMPACT_BELOW = 720;

    const ICONS = {
        search: '<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4.5 4.5"/>',
        settings: '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>',
        back: '<path d="M19 12H6M11 6l-6 6 6 6"/>',
        star: '<path d="M12 3.8l2.5 5.2 5.7.8-4.1 4 1 5.7-5.1-2.7-5.1 2.7 1-5.7-4.1-4 5.7-.8z"/>',
        clock: '<circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/>',
        copy: '<rect x="8" y="8" width="11" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H7a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h1"/>',
        reset: '<path d="M5 12a7 7 0 1 0 2.3-5.2"/><path d="M5 4.5V8h3.5"/>',
        lock: '<rect x="6" y="11" width="12" height="9" rx="2"/><path class="ec-shackle" d="M8.5 11V8a3.5 3.5 0 0 1 7 0v3"/>',
        warning: '<path d="M12 4l9 16H3z"/><path d="M12 10v4M12 17v.5"/>'
    };
    const icon = name => '<svg class="ec-glyph" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + (ICONS[name] || '') + '</svg>';

    // options: {root, catalog, t, store, schematics, decimalComma, uid}
    function create(options) {
        const { root, catalog, t, store, schematics } = options;
        const units = core.units.create(catalog.units);
        const items = catalog.items;
        const itemById = new Map(items.map(item => [item.id, item]));
        const categories = (catalog.categories || []).filter(category => items.some(item => item.category === category.id));
        const categoryName = new Map((catalog.categories || []).map(category => [category.id, category.name]));
        let englishNames = new Map();
        const ui = { rail: '', query: '', selected: '', view: 'page', compact: false };
        let page = null, popoverOpen = false, observer = null, hostObserver = null;
        const mac = /mac|iphone|ipad/i.test(String(navigator.platform || ''));

        root.innerHTML = '<header class="ec-top">'
            + '<button type="button" class="ec-icon-btn ec-back" data-action="back" hidden aria-label="' + esc(t('desktop.ecalc_back')) + '" title="' + esc(t('desktop.ecalc_back')) + '">' + icon('back') + '</button>'
            + '<label class="ec-search">' + icon('search') + '<input class="ec-search-input" type="search" autocomplete="off" spellcheck="false" placeholder="' + esc(t('desktop.ecalc_search_placeholder')) + '" aria-label="' + esc(t('desktop.ecalc_search_placeholder')) + '"><kbd class="ec-kbd">' + (mac ? '⌘K' : 'Ctrl K') + '</kbd></label>'
            + '<div class="ec-settings-wrap"><button type="button" class="ec-icon-btn" data-action="settings" aria-haspopup="dialog" aria-expanded="false" aria-label="' + esc(t('desktop.ecalc_settings')) + '" title="' + esc(t('desktop.ecalc_settings')) + '">' + icon('settings') + '</button>'
            + '<div class="ec-pop" role="dialog" aria-label="' + esc(t('desktop.ecalc_settings')) + '" hidden>'
            + '<label class="ec-set"><span>' + esc(t('desktop.ecalc_settings_digits')) + '</span><select data-set="sig">' + [3, 4, 5, 6].map(n => '<option value="' + n + '">' + n + '</option>').join('') + '</select></label>'
            + '<label class="ec-set"><span>' + esc(t('desktop.ecalc_eseries_series')) + '</span><select data-set="eseries">' + core.eseries.SERIES.map(name => '<option value="' + name + '">' + name + '</option>').join('') + '</select></label>'
            + '<fieldset class="ec-set"><legend>' + esc(t('desktop.ecalc_settings_symbols')) + '</legend><div class="ec-seg"><label><input type="radio" name="' + options.uid + '-symbols" value="iec"><span>' + esc(t('desktop.ecalc_symbols_iec')) + '</span></label>'
            + '<label><input type="radio" name="' + options.uid + '-symbols" value="ansi"><span>' + esc(t('desktop.ecalc_symbols_ansi')) + '</span></label></div></fieldset></div></div></header>'
            + '<div class="ec-body"><nav class="ec-rail"></nav>'
            + '<section class="ec-list"><h2 class="ec-list-title"></h2><ul class="ec-items"></ul><p class="ec-empty" hidden></p></section>'
            + '<main class="ec-page-host"></main></div>';

        const q = selector => root.querySelector(selector);
        const searchEl = q('.ec-search-input'), railEl = q('.ec-rail'), listEl = q('.ec-list'), itemsEl = q('.ec-items'), emptyEl = q('.ec-empty');
        const titleEl = q('.ec-list-title'), hostEl = q('.ec-page-host'), backEl = q('.ec-back'), popEl = q('.ec-pop'), settingsEl = q('[data-action="settings"]');

        // ---- data ----------------------------------------------------------------------------------------------

        function listed(ids) {
            return ids.map(id => itemById.get(id)).filter(Boolean);
        }
        function search(query) {
            const words = query.toLowerCase().split(/\s+/).filter(Boolean), found = [];
            items.forEach((item, index) => {
                const haystack = [item.name, englishNames.get(item.id) || '', (item.tags || []).join(' '), item.id.replace(/_/g, ' ')].join(' ').toLowerCase();
                if (!words.every(word => haystack.includes(word))) return;
                const name = item.name.toLowerCase();
                found.push({ item, index, rank: name.startsWith(words[0]) ? 0 : name.includes(words[0]) ? 1 : 2 });
            });
            return found.sort((a, b) => a.rank - b.rank || a.index - b.index).map(entry => entry.item);
        }
        function currentItems() {
            if (ui.query.trim()) return search(ui.query);
            if (ui.rail === 'favorites') return listed(store.prefs().favorites);
            if (ui.rail === 'recent') return listed(store.prefs().recents);
            return items.filter(item => item.category === ui.rail);
        }

        // ---- painting ------------------------------------------------------------------------------------------

        function railRows() {
            const rows = [
                { id: 'favorites', label: t('desktop.ecalc_favorites'), count: listed(store.prefs().favorites).length, glyph: 'star' },
                { id: 'recent', label: t('desktop.ecalc_recent'), count: listed(store.prefs().recents).length, glyph: 'clock' }
            ];
            return rows.concat(categories.map(category => ({ id: category.id, label: category.name, count: items.filter(item => item.category === category.id).length, glyph: '' })));
        }
        function paintRail() {
            const searching = !!ui.query.trim();
            railEl.innerHTML = railRows().map((row, index) => (index === 2 ? '<div class="ec-rail-sep" role="separator"></div>' : '')
                + '<button type="button" class="ec-rail-btn" data-rail="' + esc(row.id) + '" title="' + esc(row.label) + '"' + (!searching && ui.rail === row.id ? ' aria-current="true"' : '') + '>'
                + (row.glyph ? icon(row.glyph) : '<span class="ec-glyph ec-glyph-dot"></span>') + '<span class="ec-rail-label">' + esc(row.label) + '</span><span class="ec-count">' + row.count + '</span></button>').join('');
        }
        function paintList() {
            const searching = !!ui.query.trim(), rows = currentItems();
            const row = railRows().find(entry => entry.id === ui.rail);
            titleEl.textContent = searching ? '' : row ? row.label : '';
            titleEl.hidden = searching;
            itemsEl.innerHTML = rows.map(item => '<li><button type="button" class="ec-item" data-id="' + esc(item.id) + '"' + (ui.selected === item.id ? ' aria-current="true"' : '') + '>'
                + '<span class="ec-item-text"><span class="ec-item-name">' + esc(item.name) + '</span>'
                + (searching || ui.rail === 'favorites' || ui.rail === 'recent' ? '<span class="ec-item-sub">' + esc(categoryName.get(item.category) || '') + '</span>' : '') + '</span>'
                + (store.isFavorite(item.id) ? '<span class="ec-item-fav">' + icon('star') + '</span>' : '') + '</button></li>').join('');
            emptyEl.hidden = rows.length > 0;
            emptyEl.textContent = rows.length ? '' : searching ? t('desktop.ecalc_no_results') : ui.rail === 'favorites' ? t('desktop.ecalc_empty_favorites') : ui.rail === 'recent' ? t('desktop.ecalc_empty_recent') : t('desktop.ecalc_no_results');
        }
        function paintSettings() {
            popEl.querySelector('[data-set="sig"]').value = String(store.prefs().sig);
            popEl.querySelector('[data-set="eseries"]').value = store.prefs().eseries;
            for (const radio of popEl.querySelectorAll('input[type="radio"]')) radio.checked = radio.value === store.prefs().symbols;
        }

        function setView(view) {
            ui.view = view;
            root.dataset.view = view;
            backEl.hidden = !(ui.compact && view !== 'rail');
        }
        function setCompact(compact) {
            ui.compact = compact;
            root.dataset.compact = compact ? 'true' : 'false';
            setView(ui.view);
        }

        function mountPage(item) {
            if (page) page.dispose();
            page = window.ECalcCalcPage.create({
                calc: item, units, store, t, icon, schematics, uid: options.uid + '-' + item.id, decimalComma: options.decimalComma,
                onFavorite: () => { paintRail(); paintList(); }
            });
            hostEl.replaceChildren(page.el);
            hostEl.scrollTop = 0;
            page.layout();
        }
        function openItem(id) {
            const item = itemById.get(id);
            if (!item) return;
            ui.selected = id;
            store.pushRecent(id);
            if (!ui.query.trim()) ui.rail = ui.rail === 'favorites' || ui.rail === 'recent' ? ui.rail : item.category;
            mountPage(item);
            paintRail();
            paintList();
            if (ui.compact) setView('page');
        }
        function pickRail(id) {
            ui.rail = id;
            ui.query = '';
            searchEl.value = '';
            paintRail();
            paintList();
            if (ui.compact) setView('list');
        }

        // ---- events --------------------------------------------------------------------------------------------

        function closePopover() {
            if (!popoverOpen) return;
            popoverOpen = false;
            popEl.hidden = true;
            settingsEl.setAttribute('aria-expanded', 'false');
            document.removeEventListener('pointerdown', onOutside, true);
        }
        function onOutside(event) {
            if (!popEl.contains(event.target) && !settingsEl.contains(event.target)) closePopover();
        }
        function openPopover() {
            popoverOpen = true;
            paintSettings();
            popEl.hidden = false;
            settingsEl.setAttribute('aria-expanded', 'true');
            document.addEventListener('pointerdown', onOutside, true);
        }

        root.addEventListener('click', event => {
            const action = event.target.closest('[data-action]');
            if (action && action.dataset.action === 'settings') { if (popoverOpen) closePopover(); else openPopover(); return; }
            if (action && action.dataset.action === 'back') {
                if (ui.view === 'page') setView('list');
                else { ui.query = ''; searchEl.value = ''; paintRail(); paintList(); setView('rail'); }
                return;
            }
            const rail = event.target.closest('[data-rail]');
            if (rail && railEl.contains(rail)) { pickRail(rail.dataset.rail); return; }
            const item = event.target.closest('.ec-item');
            if (item && itemsEl.contains(item)) openItem(item.dataset.id);
        });
        popEl.addEventListener('change', event => {
            const target = event.target;
            if (target.dataset.set === 'sig') store.set('sig', Number(target.value));
            else if (target.dataset.set === 'eseries') store.set('eseries', target.value);
            else if (target.type === 'radio') store.set('symbols', target.value);
            else return;
            if (page) page.refresh();
        });
        searchEl.addEventListener('input', () => {
            ui.query = searchEl.value;
            paintRail();
            paintList();
            if (ui.compact && ui.query.trim() && ui.view === 'rail') setView('list');
        });
        searchEl.addEventListener('keydown', event => {
            if (event.key === 'Escape' && searchEl.value) {
                event.preventDefault();
                event.stopPropagation();
                searchEl.value = '';
                ui.query = '';
                paintRail();
                paintList();
            } else if (event.key === 'Enter') {
                const first = currentItems()[0];
                if (first) openItem(first.id);
            } else if (event.key === 'ArrowDown') {
                const first = itemsEl.querySelector('.ec-item');
                if (first) { event.preventDefault(); first.focus(); }
            }
        });
        // Up and Down move through the rail and the list; the buttons stay ordinary tab stops.
        function arrowKeys(container, selector) {
            container.addEventListener('keydown', event => {
                const keys = { ArrowDown: 1, ArrowUp: -1, Home: 'first', End: 'last' };
                if (!(event.key in keys) || !event.target.matches(selector)) return;
                const buttons = [...container.querySelectorAll(selector)], index = buttons.indexOf(event.target);
                const next = keys[event.key] === 'first' ? 0 : keys[event.key] === 'last' ? buttons.length - 1 : Math.max(0, Math.min(buttons.length - 1, index + keys[event.key]));
                event.preventDefault();
                buttons[next].focus();
            });
        }
        arrowKeys(railEl, '.ec-rail-btn');
        arrowKeys(itemsEl, '.ec-item');
        root.addEventListener('keydown', event => {
            if (event.key === 'Escape' && popoverOpen) { event.preventDefault(); event.stopPropagation(); closePopover(); settingsEl.focus(); }
        });

        const measure = width => setCompact(width > 0 && width < COMPACT_BELOW);
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(entries => measure(entries[entries.length - 1].contentRect.width));
            observer.observe(root);
        } else {
            measure(root.clientWidth);
        }
        // The page lays itself out for the width of its host, which is not changed by that layout.
        if (typeof ResizeObserver === 'function') {
            hostObserver = new ResizeObserver(() => { if (page) page.layout(); });
            hostObserver.observe(hostEl);
        }

        // ---- start ---------------------------------------------------------------------------------------------

        const remembered = itemById.get(store.prefs().last) || null;
        const first = remembered || items[0] || null;
        ui.rail = first ? first.category : categories.length ? categories[0].id : 'favorites';
        ui.view = remembered ? 'page' : 'rail';
        paintRail();
        paintList();
        setView(ui.view);
        if (first) {
            ui.selected = first.id;
            mountPage(first);
            paintList();
        }

        return {
            openItem,
            focusSearch() { searchEl.focus(); searchEl.select(); },
            setEnglishNames(names) {
                englishNames = names;
                if (ui.query.trim()) paintList();
            },
            dispose() {
                closePopover();
                if (observer) observer.disconnect();
                if (hostObserver) hostObserver.disconnect();
                if (page) page.dispose();
                page = null;
                store.flush();
            }
        };
    }

    window.ECalcShell = { create, icon };
})();
```

`ui/js/desktop/apps/ecalc/ecalc.js`:

```javascript
/* ECalc app: the composition root. It fetches the catalog of the desktop language once per open window, shows a retry
   screen when that fails, hands the data to the shell (rail, list, page) and cleans up when the window closes. The
   math lives in ecalc-expr/units/eseries/solve.js, the page in ecalc-calc-page.js, the shell in ecalc-shell.js.
   Ctrl/Cmd+K belongs to the app while the focus is inside it: the desktop's Spotlight listens on the document, so the
   event is stopped here. Publishes window.ECalcApp = { render(hostEl, windowId, ctx), dispose(windowId) }. */
(function () {
    'use strict';

    const instances = new Map();
    // Languages that write the decimal comma. The core always uses the point; this only changes what is displayed.
    const COMMA_LANGUAGES = ['cs', 'da', 'de', 'el', 'es', 'fr', 'it', 'nl', 'no', 'pl', 'pt', 'sv'];
    const catalogURL = lang => '/api/ecalc/catalog?lang=' + encodeURIComponent(lang);

    function dispose(id) {
        const stop = instances.get(id);
        instances.delete(id);
        if (stop) stop();
    }

    // The house translator inserts {name} and {{name}}; a stand-in translator may only know one of them.
    function fill(text, vars) {
        let out = String(text);
        for (const [name, value] of Object.entries(vars || {})) out = out.split('{{' + name + '}}').join(String(value)).split('{' + name + '}').join(String(value));
        return out;
    }

    function render(host, id, ctx) {
        dispose(id);
        const lang = String(ctx.lang || window.SYSTEM_LANG || document.documentElement.lang || 'en').slice(0, 2).toLowerCase();
        const uid = 'ec-' + String(id).replace(/[^\w-]/g, '-');
        const t = (key, vars) => fill(ctx.t(key, vars), vars);
        const esc = ctx.esc || window.ECalcTypeset.escape;
        const store = window.ECalcStore.create();
        const root = document.createElement('div');
        root.className = 'ec-app';
        root.dataset.compact = 'false';
        root.dataset.view = 'page';
        host.replaceChildren(root);
        let disposed = false, shell = null;

        // English names make the search find "voltage divider" in every language; the catalog of the language
        // itself has the translated names only. A failure here only narrows the search.
        function loadEnglishNames() {
            if (lang === 'en') return;
            Promise.resolve(ctx.api(catalogURL('en'))).then(english => {
                if (disposed || !shell || !english || !Array.isArray(english.items)) return;
                shell.setEnglishNames(new Map(english.items.map(item => [item.id, String(item.name || '')])));
            }).catch(() => {});
        }

        async function load() {
            root.dataset.state = 'loading';
            root.innerHTML = '<div class="ec-status" role="status" aria-busy="true"><span class="ec-spinner"></span></div>';
            try {
                const catalog = await ctx.api(catalogURL(lang));
                if (disposed) return;
                if (!catalog || !Array.isArray(catalog.items) || !catalog.units) throw new Error('unexpected catalog');
                root.dataset.state = 'ready';
                shell = window.ECalcShell.create({ root, catalog, t, store, schematics: window.ECalcSchematics || {}, decimalComma: COMMA_LANGUAGES.includes(lang), uid });
                loadEnglishNames();
            } catch (error) {
                if (disposed) return;
                shell = null;
                root.dataset.state = 'failed';
                root.innerHTML = '<div class="ec-fail" role="alert"><p>' + esc(t('desktop.ecalc_load_failed')) + '</p><button type="button" class="ec-btn" data-action="retry">' + esc(t('desktop.ecalc_retry')) + '</button></div>';
            }
        }

        root.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k') {
                event.preventDefault();
                event.stopPropagation();
                if (shell) shell.focusSearch();
            }
        });
        root.addEventListener('click', event => {
            const retry = event.target.closest('[data-action="retry"]');
            if (retry && root.contains(retry) && !shell) void load();
        });

        instances.set(id, () => {
            if (disposed) return;
            disposed = true;
            if (shell) shell.dispose();
            shell = null;
            store.flush();
            root.replaceChildren();
        });
        void load();
    }

    window.ECalcApp = { render, dispose };
})();
```

- [ ] **Step 4: Run the tests and see them pass**

```powershell
go test ./ui -run 'TestECalc(DoesNotUseDynamicEvaluation|StorageIsGuarded|SearchShortcutStopsPropagation|UsesOnlyItsOwnTranslationKeys)' -count=1
node scripts/test-ecalc-core.mjs
```

Expected: `ok  	aurago/ui` and `ecalc core tests: 686 cases ok`. (`TestECalcUsesOnlyItsOwnTranslationKeys` needs Part B's locale files: without them it fails with `en misses desktop.ecalc_dim_voltage ...`; the other three pass.)

- [ ] **Step 5: Commit**

```powershell
git add ui/js/desktop/apps/ecalc/ecalc-store.js ui/js/desktop/apps/ecalc/ecalc-calc-page.js ui/js/desktop/apps/ecalc/ecalc-shell.js ui/js/desktop/apps/ecalc/ecalc.js ui/desktop_ecalc_test.go
git commit -m "feat(ecalc): app shell, calculator page with schematic fields, store and composition root

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C4: Stylesheet

**Files:**
- Create: `ui/css/desktop-app-ecalc.css`
- Modify: `ui/desktop_ecalc_test.go` (append `TestECalcStylesheetUsesOnlyTokens`)
- Modify: `ui/desktop_theme_consistency_test.go` (base 1fb47da49: lines 74–75, in `TestDesktopEverydayAppsUseThemeBridge`, func at line 59)

Only `--vd-mat-*`, `--vd-*` and `--ds-*` tokens (with the old shell tokens as fallbacks, like the other app stylesheets), so the app follows all 14 skins: the stage uses the well material, the cards the card material, the fields the field material, the selection the selected material; computed and error tints are `color-mix()` of the status tokens over the field material; transitions and animations sit inside `@media (prefers-reduced-motion: no-preference)` on selectors that start with `body:not([data-animations="false"])`.

- [ ] **Step 1: Write the failing tests**

Append to `ui/desktop_ecalc_test.go` (at the end of the file, after one blank line):

```go
// The stylesheet takes every colour from tokens and gates its motion.
func TestECalcStylesheetUsesOnlyTokens(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-ecalc.css")
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	for label, pattern := range map[string]string{
		"hex colour":          `#[0-9a-fA-F]{3,8}\b`,
		"colour function":     `\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]`,
		"named colour":        `(?i)[:\s(,](?:white|black|red|green|blue|gray|grey|orange|yellow|purple)\b\s*[;,)]`,
		"external url":        `url\(`,
		"import":              `@import`,
		"theme id":            `data-theme=`,
		"cookie banner class": `\.cc-`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("css/desktop-app-ecalc.css has a %s: %q", label, match)
		}
	}
	if strings.Count(body, "!important") != 1 || !strings.Contains(body, ".ec-app [hidden] { display: none !important; }") {
		t.Error("the only !important allowed is the [hidden] rule")
	}
	for _, marker := range []string{"var(--vd-mat-canvas", "var(--vd-mat-bar", "var(--vd-mat-side", "var(--vd-mat-card", "var(--vd-mat-well", "var(--vd-mat-field", "var(--vd-mat-btn", "var(--vd-mat-selected", "var(--ds-color-success-fg)", "var(--ds-color-danger-fg)", "var(--ds-color-border-focus)", `.ec-app[data-compact="true"]`, `.ec-field[data-state="computed"]`, `.ec-field[data-state="error"]`} {
		if !strings.Contains(body, marker) {
			t.Errorf("css/desktop-app-ecalc.css is missing %q", marker)
		}
	}
	gate := strings.Index(body, "@media (prefers-reduced-motion: no-preference)")
	if gate < 0 {
		t.Fatal("css/desktop-app-ecalc.css has no reduced-motion gate")
	}
	for _, motion := range regexp.MustCompile(`\b(?:transition|animation)\s*:`).FindAllStringIndex(body, -1) {
		if motion[0] < gate {
			t.Errorf("a transition or animation at offset %d sits outside @media (prefers-reduced-motion: no-preference)", motion[0])
		}
	}
}
```

In `ui/desktop_theme_consistency_test.go` (base lines 74–75) add three rows to the table of `TestDesktopEverydayAppsUseThemeBridge`:

Replace:

```go
		{"css/desktop-app-calculator.css", ".vd-calc-prog-display"},
		{"css/desktop-app-calculator.css", "background: var(--vd-mat-well, var(--vd-theme-panel-bg))"},
```

with:

```go
		{"css/desktop-app-calculator.css", ".vd-calc-prog-display"},
		{"css/desktop-app-calculator.css", "background: var(--vd-mat-well, var(--vd-theme-panel-bg))"},
		{"css/desktop-app-ecalc.css", ".ec-app"},
		{"css/desktop-app-ecalc.css", "background: var(--vd-mat-canvas, var(--vd-theme-app-bg))"},
		{"css/desktop-app-ecalc.css", "background: var(--vd-mat-well, var(--ec-card))"},
```

- [ ] **Step 2: Run them and see them fail**

```powershell
go test ./ui -run 'TestECalcStylesheetUsesOnlyTokens|TestDesktopEverydayAppsUseThemeBridge' -count=1
```

Expected: FAIL, `read css/desktop-app-ecalc.css: open ...: no such file or directory`.

- [ ] **Step 3: Create the stylesheet**

`ui/css/desktop-app-ecalc.css`:

```css
/* ECalc: category rail, item list and the calculator page with its schematic, field chips and typeset formulas. Tokens
   only (--vd-mat-*, --vd-*, --ds-*): no hex or literal colour functions outside color-mix(), so the app follows every
   desktop skin. Every transition and animation sits inside @media (prefers-reduced-motion: no-preference) on selectors
   that start with body:not([data-animations="false"]). */
.ec-app { --ec-line: var(--vd-mat-divider, var(--vd-theme-border)); --ec-card: var(--vd-mat-card-color, var(--vd-theme-panel-bg)); --ec-ok: var(--ds-color-success-fg); --ec-warn: var(--ds-color-warning-fg); --ec-bad: var(--ds-color-danger-fg); --ec-ok-text: light-dark(color-mix(in srgb, var(--ec-ok) 30%, var(--vd-text)), var(--ec-ok)); --ec-warn-text: light-dark(color-mix(in srgb, var(--ec-warn) 30%, var(--vd-text)), var(--ec-warn)); --ec-bad-text: light-dark(color-mix(in srgb, var(--ec-bad) 34%, var(--vd-text)), color-mix(in srgb, var(--ec-bad) 88%, var(--vd-text))); --ec-live: color-mix(in srgb, var(--vd-accent) 55%, var(--vd-text)); --ec-mono: var(--vd-mat-mono-font, ui-monospace, SFMono-Regular, Consolas, monospace); --ec-math-font: "Cambria Math", "STIX Two Math", "Latin Modern Math", "Times New Roman", Georgia, serif; position: relative; display: grid; grid-template-rows: auto minmax(0, 1fr); width: 100%; height: 100%; min-width: 0; min-height: 360px; overflow: hidden; isolation: isolate; color: var(--vd-text); background: var(--vd-mat-canvas, var(--vd-theme-app-bg)); font: 14px/1.45 var(--vd-font-ui, system-ui, sans-serif); }
.ec-app * { box-sizing: border-box; }
.ec-app :where(button, input, select) { font: inherit; color: inherit; }
.ec-app [hidden] { display: none !important; }
.ec-app :is(button, input, select, summary):focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset); }
.ec-app button:disabled { opacity: .5; cursor: not-allowed; }
.ec-glyph { width: 16px; height: 16px; flex: 0 0 auto; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.ec-glyph-dot { display: inline-block; width: 6px; height: 6px; margin: 0 5px; border-radius: 50%; background: currentColor; opacity: .45; }

/* Loading and failure */
.ec-status, .ec-fail { display: grid; place-content: center; justify-items: center; gap: 12px; height: 100%; grid-row: 1 / -1; padding: 24px; text-align: center; }
.ec-fail p { margin: 0; max-width: 36ch; }
.ec-spinner { width: 22px; height: 22px; border: 2px solid var(--ec-line); border-top-color: var(--vd-accent); border-radius: 50%; }

/* Buttons */
.ec-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 32px; padding: 0 12px; border: 1px solid var(--vd-mat-btn-border, var(--vd-theme-border)); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); color: var(--vd-mat-btn-text, inherit); font-weight: 600; white-space: nowrap; cursor: pointer; }
.ec-btn:hover { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
.ec-btn[data-done="true"] { border-color: var(--ec-ok); color: var(--ec-ok-text); }
.ec-btn[aria-pressed="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); }
.ec-btn[data-action="favorite"][aria-pressed="true"] .ec-glyph { fill: currentColor; }
.ec-icon-btn { display: inline-grid; place-items: center; flex: 0 0 auto; width: 32px; height: 32px; padding: 0; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; color: var(--vd-text); cursor: pointer; }
.ec-icon-btn:hover, .ec-icon-btn[aria-expanded="true"] { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }

/* Top bar: search and settings */
.ec-top { display: flex; align-items: center; gap: var(--ds-space-2); min-height: 52px; padding: var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--ec-line); background: var(--vd-mat-bar, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-bar-edge, none); }
.ec-search { display: flex; flex: 1 1 auto; align-items: center; gap: var(--ds-space-2); max-width: 440px; height: 34px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-mat-field-border, var(--vd-theme-border)); border-radius: var(--vd-mat-field-radius, var(--ds-radius-md)); background: var(--vd-mat-field, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-field-shadow, none); color: var(--vd-theme-muted); }
.ec-search:focus-within { border-color: var(--ds-color-border-focus); box-shadow: 0 0 0 1px var(--ds-color-border-focus); }
.ec-app .ec-search-input, .ec-app .ec-search-input:focus-visible { flex: 1; min-width: 0; height: 100%; border: 0; outline: 0; box-shadow: none; background: transparent; color: var(--vd-text); -webkit-appearance: none; appearance: none; }
.ec-search-input::-webkit-search-cancel-button { -webkit-appearance: none; }
.ec-kbd { padding: 1px 6px; border: 1px solid var(--ec-line); border-radius: var(--ds-radius-xs, 4px); color: var(--vd-theme-muted); font: 12px/1.5 var(--ec-mono); white-space: nowrap; }
.ec-settings-wrap { position: relative; margin-left: auto; }
.ec-pop { position: absolute; right: 0; top: calc(100% + 8px); z-index: 30; display: grid; gap: var(--ds-space-3); width: 248px; padding: var(--ds-space-3); border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, 0 12px 32px color-mix(in srgb, var(--vd-text) 22%, transparent)); }
.ec-set { display: grid; gap: 4px; margin: 0; padding: 0; border: 0; min-width: 0; }
.ec-set > span, .ec-set > legend { padding: 0; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; font-weight: 600; }
.ec-set select { height: 32px; padding: 0 8px; border: 1px solid var(--vd-mat-field-border, var(--vd-theme-border)); border-radius: var(--vd-mat-field-radius, var(--ds-radius-sm)); background: var(--vd-mat-field, var(--vd-theme-control-bg)); }

/* Segmented control (symbol standard, choices) */
.ec-seg { display: inline-flex; max-width: 100%; border: 1px solid var(--vd-mat-btn-border, var(--vd-theme-border)); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); overflow: hidden; }
.ec-seg label { position: relative; display: block; min-width: 0; cursor: pointer; }
.ec-seg label + label { border-left: 1px solid var(--ec-line); }
.ec-seg input { position: absolute; inset: 0; width: 100%; height: 100%; margin: 0; opacity: 0; cursor: pointer; }
.ec-seg span { display: block; padding: 5px 12px; overflow: hidden; color: var(--vd-mat-btn-text, inherit); text-overflow: ellipsis; white-space: nowrap; }
.ec-seg label:hover span { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
.ec-seg input:checked + span { background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); font-weight: 600; }
.ec-seg input:focus-visible + span { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: -2px; }

/* Three columns: rail, list, page */
.ec-body { display: grid; grid-template-columns: 176px 196px minmax(0, 1fr); min-height: 0; }
.ec-rail { padding: 6px; overflow: auto; border-right: 1px solid var(--ec-line); background: var(--vd-mat-side, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-side-edge, none); }
.ec-rail-sep { height: 1px; margin: var(--ds-space-2) var(--ds-space-2); background: var(--ec-line); }
.ec-rail-btn { display: flex; align-items: center; gap: var(--ds-space-2); width: 100%; min-height: 36px; padding: 5px 6px; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; font-weight: 500; line-height: 1.25; text-align: left; cursor: pointer; }
.ec-rail-btn:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.ec-rail-btn[aria-current="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); font-weight: 600; }
.ec-rail-label { flex: 1 1 auto; min-width: 0; overflow-wrap: break-word; hyphens: auto; }
.ec-count { flex: 0 0 auto; margin-left: auto; padding: 0 6px; border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); color: var(--vd-theme-muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.ec-rail-btn[aria-current="true"] .ec-count { background: transparent; color: inherit; }
.ec-list { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: auto; border-right: 1px solid var(--ec-line); }
.ec-list-title { margin: 0; padding: var(--ds-space-3) var(--ds-space-3) var(--ds-space-1); color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 15px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }
.ec-items { display: grid; gap: 2px; margin: 0; padding: var(--ds-space-2); list-style: none; }
.ec-item { display: flex; align-items: center; gap: var(--ds-space-2); width: 100%; min-height: 40px; padding: 4px var(--ds-space-2); border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.ec-item:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.ec-item[aria-current="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); }
.ec-item-text { display: grid; flex: 1; min-width: 0; }
.ec-item-name { font-weight: 600; line-height: 1.25; overflow-wrap: break-word; hyphens: auto; }
.ec-item-sub { overflow: hidden; color: var(--vd-theme-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.ec-item[aria-current="true"] .ec-item-sub { color: inherit; opacity: .8; }
.ec-item-fav { display: inline-grid; color: var(--ec-warn-text); }
.ec-item-fav .ec-glyph { width: 13px; height: 13px; fill: currentColor; }
.ec-empty { margin: 0; padding: var(--ds-space-4) var(--ds-space-3); color: var(--vd-theme-muted); }
.ec-page-host { min-width: 0; min-height: 0; padding: 16px 16px 28px; overflow: auto; }

/* Calculator page */
.ec-page { display: grid; align-content: start; gap: 16px; max-width: 1180px; margin: 0 auto; }
.ec-page-head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; }
.ec-title { flex: 1 1 220px; min-width: 0; margin: 0; color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 20px; font-weight: var(--vd-mat-heading-weight, 600); line-height: 1.25; letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }
.ec-actions { display: flex; flex-wrap: wrap; gap: 6px; }
.ec-page[data-narrow="true"] .ec-btn { width: 34px; padding: 0; }
.ec-page[data-narrow="true"] .ec-btn-label { display: none; }
.ec-choices { display: flex; flex-wrap: wrap; gap: 8px 20px; }
.ec-choice { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; min-width: 0; margin: 0; padding: 0; border: 0; }
.ec-choice legend { float: left; padding: 0; margin-right: 10px; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 13px; font-weight: 600; }
.ec-main { display: grid; grid-template-columns: minmax(0, 1fr); gap: 16px; }
.ec-page[data-layout="split"] .ec-main { grid-template-columns: minmax(0, 1fr) 300px; }
.ec-stage { --ec-stage-bg: var(--vd-mat-well-color, var(--vd-theme-app-bg)); display: grid; align-content: start; gap: 12px; min-width: 0; padding: 12px; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-well, var(--ec-card)); box-shadow: var(--vd-mat-well-shadow, none); }
.ec-math-card, .ec-block { min-width: 0; padding: 14px 16px; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }

/* Schematic: strokes are currentColor, the field chips sit at the anchors of the drawing */
.ec-sch-wrap { position: relative; width: 100%; max-width: calc(var(--ec-w) * 30px); margin: 0 auto; aspect-ratio: var(--ec-ar); }
.ec-sch-art { position: absolute; inset: 0; }
.ec-sch { display: block; width: 100%; height: 100%; color: var(--vd-text); overflow: visible; }
.ec-sch .ec-term { fill: var(--ec-stage-bg); }
.ec-sch-text { font: 12px/1 var(--vd-font-ui, system-ui, sans-serif); }
.ec-sch-pin { font: 9px/1 var(--vd-font-ui, system-ui, sans-serif); }
.ec-sch-sub { font-size: 8.5px; }
.ec-chips { position: absolute; inset: 0; pointer-events: none; }
.ec-chips > .ec-field { position: absolute; left: calc(var(--ax) * 1%); top: calc(var(--ay) * 1%); width: 128px; pointer-events: auto; }
.ec-chips > .ec-field[data-list] { width: 224px; }
.ec-chips > [data-side="left"] { transform: translate(-100%, -50%); margin-left: -6px; }
.ec-chips > [data-side="right"] { transform: translate(0, -50%); margin-left: 6px; }
.ec-chips > [data-side="top"] { transform: translate(-50%, -100%); margin-top: -6px; }
.ec-chips > [data-side="bottom"] { transform: translate(-50%, 0); margin-top: 6px; }
.ec-page[data-chips="listed"] .ec-sch-wrap { display: grid; gap: 12px; max-width: none; aspect-ratio: auto; }
.ec-page[data-chips="listed"] .ec-sch-art { position: static; width: 100%; max-width: 460px; margin: 0 auto; }
.ec-page[data-chips="listed"] .ec-sch { height: auto; }
.ec-page[data-chips="listed"] .ec-chips { position: static; display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 10px; pointer-events: auto; }
.ec-page[data-chips="listed"] .ec-chips > .ec-field { position: static; width: auto; transform: none; margin: 0; }
.ec-fields { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 10px; }
.ec-fields:empty { display: none; }

/* Field chip: input, computed, error, pending and locked states */
.ec-field { --ec-tint: transparent; --ec-edge: var(--vd-mat-field-border, var(--vd-theme-border)); display: grid; gap: 1px; min-width: 0; padding: 3px 6px 4px; border: 1px solid var(--ec-edge); border-radius: var(--vd-mat-field-radius, var(--ds-radius-md)); background: linear-gradient(var(--ec-tint), var(--ec-tint)), var(--vd-mat-field, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-field-shadow, none); }
.ec-field[data-state="computed"] { --ec-tint: color-mix(in srgb, var(--ec-ok) 15%, transparent); --ec-edge: color-mix(in srgb, var(--ec-ok) 48%, var(--vd-mat-field-border, var(--vd-theme-border))); }
.ec-field[data-state="error"] { --ec-tint: color-mix(in srgb, var(--ec-bad) 15%, transparent); --ec-edge: var(--ec-bad); }
.ec-field[data-state="pending"] { --ec-tint: color-mix(in srgb, var(--ec-warn) 10%, transparent); border-style: dashed; }
.ec-field[data-locked="true"] { --ec-edge: color-mix(in srgb, var(--vd-accent) 70%, var(--vd-mat-field-border, var(--vd-theme-border))); }
.ec-field:focus-within { --ec-edge: var(--ds-color-border-focus); }
.ec-field-head { display: flex; align-items: center; justify-content: space-between; gap: 4px; min-height: 20px; }
.ec-field-label { min-width: 0; overflow: hidden; color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12.5px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.ec-field-label sub, .ec-title sub, .ec-row-name sub, .ec-math sub { font-size: .75em; line-height: 0; }
.ec-lock { display: inline-grid; place-items: center; width: 22px; height: 22px; padding: 0; border: 0; border-radius: var(--ds-radius-xs, 4px); background: transparent; color: var(--vd-theme-muted); opacity: .8; cursor: pointer; }
.ec-lock:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); opacity: 1; }
.ec-lock .ec-glyph { width: 14px; height: 14px; }
.ec-lock .ec-shackle { transform-origin: 8.5px 11px; }
.ec-lock[aria-pressed="false"] .ec-shackle { transform: rotate(-26deg); }
.ec-lock[aria-pressed="true"] { color: var(--vd-accent); opacity: 1; }
.ec-field-row { display: flex; align-items: center; gap: 4px; min-width: 0; }
.ec-input { flex: 1 1 auto; width: 100%; min-width: 0; height: 26px; padding: 0 5px; border: 1px solid transparent; border-radius: var(--ds-radius-xs, 4px); background: transparent; color: var(--vd-text); font-family: var(--ec-mono); font-size: 13px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; }
.ec-input:hover:not(:focus) { border-color: var(--ec-line); }
.ec-input:focus-visible { outline-offset: 0; }
.ec-input[readonly] { cursor: default; }
.ec-unit-wrap { position: relative; flex: 0 0 auto; width: 24px; height: 26px; }
.ec-unit-wrap::after { content: ""; position: absolute; right: 8px; top: 50%; width: 5px; height: 5px; border-right: 1.5px solid var(--vd-theme-muted); border-bottom: 1.5px solid var(--vd-theme-muted); transform: translateY(-70%) rotate(45deg); pointer-events: none; }
.ec-unit { width: 100%; height: 100%; padding: 0; border: 1px solid var(--vd-mat-btn-border, var(--vd-theme-border)); border-radius: var(--ds-radius-xs, 4px); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: transparent; font-size: 12px; cursor: pointer; -webkit-appearance: none; appearance: none; }
.ec-unit:hover { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
.ec-unit option { background-color: var(--vd-theme-panel-bg-strong, var(--vd-theme-panel-bg)); color: var(--vd-text); }
.ec-unit-static { flex: 0 0 auto; padding: 0 4px; color: var(--vd-theme-muted); font-size: 12px; }
.ec-field-msg { color: var(--ec-bad-text); font-size: 12px; line-height: 1.3; }
.ec-field[data-state="pending"] .ec-field-msg { color: var(--ec-warn-text); }
.ec-field-msg:empty { display: none; }
.ec-chips > .ec-field .ec-field-msg { position: absolute; left: -1px; right: -1px; top: calc(100% + 4px); z-index: 3; padding: 4px 8px; border: 1px solid var(--ec-edge); border-radius: var(--ds-radius-sm); background: linear-gradient(var(--ec-tint), var(--ec-tint)), var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }
.ec-page[data-chips="listed"] .ec-chips > .ec-field .ec-field-msg { position: static; padding: 0; border: 0; background: none; box-shadow: none; }

/* Typeset formula and live line */
.ec-formula + .ec-formula { margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--ec-line); }
.ec-math { overflow-x: auto; overflow-y: hidden; padding: 6px 2px; font-family: var(--ec-math-font); font-size: 18px; line-height: 1.5; }
.ec-live { margin-top: 2px; font-size: 16px; color: var(--vd-theme-muted); }
.ec-live .ec-v { color: var(--ec-live); }
.ec-live .ec-u { opacity: .7; }
.ec-i { font-style: italic; }
.ec-n, .ec-fn { font-style: normal; }
.ec-op { margin: 0 .24em; }
.ec-eq { margin: 0 .42em; }
.ec-frac { display: inline-flex; flex-direction: column; margin: 0 .12em; vertical-align: middle; text-align: center; }
.ec-fnum { padding: 0 .3em .06em; border-bottom: 1px solid currentColor; }
.ec-fden { padding: .06em .3em 0; }
.ec-sup { font-size: .72em; line-height: 0; vertical-align: .6em; }
.ec-sqrt { display: inline-flex; align-items: stretch; }
.ec-rad { margin-right: 1px; }
.ec-radb { margin-top: .08em; padding: 0 .18em; border-top: 1px solid currentColor; }

/* E-series, derived quantities, warnings, explanation */
.ec-below { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px; }
.ec-block h3 { margin: 0 0 8px; color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 13.5px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }
.ec-rows { display: grid; gap: 2px; margin: 0; padding: 0; list-style: none; }
.ec-rows li { display: grid; grid-template-columns: minmax(44px, auto) minmax(0, 1fr) auto; align-items: baseline; gap: 12px; padding: 4px 0; border-bottom: 1px solid color-mix(in srgb, var(--ec-line) 60%, transparent); }
.ec-rows li:last-child { border-bottom: 0; }
.ec-row-name { color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-weight: 600; }
.ec-row-val { font-family: var(--ec-mono); font-variant-numeric: tabular-nums; }
.ec-row-dev { font-family: var(--ec-mono); font-size: 12.5px; font-variant-numeric: tabular-nums; color: var(--vd-theme-muted); }
.ec-row-dev[data-tone="ok"] { color: var(--ec-ok-text); }
.ec-row-dev[data-tone="warn"] { color: var(--ec-warn-text); }
.ec-warnings { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
.ec-warnings:empty { display: none; }
.ec-warnings li { display: flex; align-items: flex-start; gap: 8px; padding: 9px 12px; border: 1px solid color-mix(in srgb, var(--ec-warn) 55%, var(--ec-line)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: linear-gradient(color-mix(in srgb, var(--ec-warn) 13%, transparent), color-mix(in srgb, var(--ec-warn) 13%, transparent)), var(--vd-mat-card, var(--vd-theme-panel-bg)); }
.ec-warnings .ec-glyph { margin-top: 2px; color: var(--ec-warn-text); }
.ec-doc { padding: 0 2px; }
.ec-doc summary { width: fit-content; padding: 4px 0; color: var(--vd-mat-heading-color, var(--vd-text)); font-weight: 600; cursor: pointer; }
.ec-doc-body { max-width: 72ch; padding-top: 6px; color: var(--vd-theme-muted); }
.ec-doc-body p { margin: 0 0 8px; }
.ec-source { font-size: 13px; }
.ec-source strong { color: var(--vd-text); }

/* One stack below 720 px: rail, list or page, with the back button in the top bar */
.ec-app[data-compact="true"] .ec-body { grid-template-columns: minmax(0, 1fr); }
.ec-app[data-compact="true"] :is(.ec-rail, .ec-list, .ec-page-host) { display: none; border-right: 0; }
.ec-app[data-compact="true"][data-view="rail"] .ec-rail { display: block; }
.ec-app[data-compact="true"][data-view="list"] .ec-list { display: flex; }
.ec-app[data-compact="true"][data-view="page"] .ec-page-host { display: block; padding: 12px 12px 24px; }
.ec-app[data-compact="true"] .ec-search { max-width: none; }
.ec-app[data-compact="true"] .ec-kbd { display: none; }
.ec-app[data-compact="true"] .ec-settings-wrap { margin-left: 0; }
@media (pointer: coarse) { .ec-btn, .ec-icon-btn, .ec-rail-btn, .ec-item, .ec-seg span, .ec-unit, .ec-input { min-height: 44px; } .ec-lock { width: 32px; height: 32px; } }

@media (prefers-reduced-motion: no-preference) {
    body:not([data-animations="false"]) .ec-btn, body:not([data-animations="false"]) .ec-rail-btn, body:not([data-animations="false"]) .ec-item, body:not([data-animations="false"]) .ec-icon-btn, body:not([data-animations="false"]) .ec-field { transition: background-color .12s ease, border-color .12s ease, color .12s ease; }
    body:not([data-animations="false"]) .ec-lock .ec-shackle { transition: transform .14s ease; }
    body:not([data-animations="false"]) .ec-spinner { animation: ec-spin .8s linear infinite; }
    @keyframes ec-spin { to { transform: rotate(360deg); } }
}
```

- [ ] **Step 4: Run the tests and the style guards and see them pass**

```powershell
go test ./ui -run '^(TestECalcStylesheetUsesOnlyTokens|TestDesktopEverydayAppsUseThemeBridge|TestDesktopAppMaterials|TestDesktopFruity|TestDesktopCSS|TestDesktopThemesShare)' -count=1
```

Expected: `ok  	aurago/ui`.

- [ ] **Step 5: Commit**

```powershell
git add ui/css/desktop-app-ecalc.css ui/desktop_ecalc_test.go ui/desktop_theme_consistency_test.go
git commit -m "feat(ecalc): theme-native stylesheet for the ECalc app

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C5: Register the app in the desktop core and rebuild the bundle

**Files:**
- Modify: `ui/js/desktop/core/module-loader.js` (base 1fb47da49: before line 29, `'minigolf': {`, inside `DESKTOP_APP_ASSETS`, const at line 22)
- Modify: `ui/js/desktop/core/menus-and-routing.js` (before line 1762, `if (appId === 'tresor') {`, inside `renderAppContent`, function at line 1553)
- Modify: `ui/js/desktop/core/desktop-foundation.js` (after line 604, `findit: 'FindItApp',`, inside `appGlobalName`, function at line 586)
- Modify: `ui/js/desktop/core/window-shell-runtime.js` (after line 1321, `findit: { width: 1320, height: 820 },` in `appWindowSize`, function at line 1296; after line 1363, `if (appId === 'findit') return ...` in `appWindowMinSize`, function at line 1358)
- Modify (generated): `ui/js/desktop/bundles/main.bundle.js` (by `npm run build:ui`)
- Modify: `ui/desktop_ecalc_test.go` (append `TestECalcIsWiredIntoTheDesktopCore`)
- Modify: `ui/frontend_optimization_test.go` (base: lines 717–719, in `TestDesktopAppAssetsRegistryCoversLazyCSSForAllBuiltinApps`, func at line 694)
- Modify: `ui/js/desktop/apps/AGENTS.md` (base: before line 30, `## Email exports`)

**GitNexus impact (contract §0), run from the main checkout on 2026-11-01 at base 1fb47da49:** `appGlobalName` LOW (1 direct caller, `disposeAppWindow`), `renderAppContent` LOW (2 direct callers, `openDesktopFileDropInWindow` and `openApp`), `appWindowSize` LOW (1, `openApp`), `appWindowMinSize` LOW (2, `windowLayoutLimits` and `openApp`); every edit adds one table entry or one `if` branch for the new id and changes nothing for the existing apps. No HIGH/CRITICAL, no UNKNOWN. Re-run before editing:

```powershell
cd C:\Users\Andi\Documents\repo\AuraGo
foreach ($s in 'appGlobalName','renderAppContent','appWindowSize','appWindowMinSize') { node .gitnexus/run.cjs impact $s --direction upstream --repo . }
```

Expected: `"risk": "LOW"` for all four.

- [ ] **Step 1: Write the failing wiring test**

Append to `ui/desktop_ecalc_test.go` (at the end of the file, after one blank line):

```go
// ECalc is registered in the loader, the router, the global-name table and the window presets, and the bundle holds them.
func TestECalcIsWiredIntoTheDesktopCore(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        ecalc: {")
	if start < 0 {
		t.Fatal("module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	if !strings.Contains(entry, "appStyles('/css/desktop-app-ecalc.css')") {
		t.Error("the ecalc loader entry must load css/desktop-app-ecalc.css")
	}
	order := []string{"ecalc-expr.js", "ecalc-units.js", "ecalc-eseries.js", "ecalc-solve.js", "ecalc-typeset.js", "ecalc-symbols.js", "ecalc-schematics.js", "ecalc-store.js", "ecalc-calc-page.js", "ecalc-shell.js", "/ecalc.js'"}
	offset := 0
	for _, script := range order {
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the ecalc loader entry lists %s out of order or not at all (the core first, the composition root last)", script)
		}
		offset += index + len(script)
	}
	for _, check := range []struct{ file, marker string }{
		{"js/desktop/core/menus-and-routing.js", "if (appId === 'ecalc') {"},
		{"js/desktop/core/menus-and-routing.js", "window.ECalcApp.render(contentEl(id), id,"},
		{"js/desktop/core/desktop-foundation.js", "ecalc: 'ECalcApp',"},
		{"js/desktop/core/window-shell-runtime.js", "ecalc: { width: 1100, height: 760 },"},
		{"js/desktop/core/window-shell-runtime.js", "if (appId === 'ecalc') return { width: 640, height: 480 };"},
		{"js/desktop/bundles/main.bundle.js", "ecalc: 'ECalcApp',"},
		{"js/desktop/bundles/main.bundle.js", "window.ECalcApp.render(contentEl(id), id,"},
		{"js/desktop/bundles/main.bundle.js", "if (appId === 'ecalc') return { width: 640, height: 480 };"},
	} {
		if !strings.Contains(readDesktopAssetText(t, check.file), check.marker) {
			t.Errorf("%s is missing %q", check.file, check.marker)
		}
	}
}
```

In `ui/frontend_optimization_test.go` (base lines 717–719) require the stylesheet and the loader entry:

Replace:

```go
		"'/css/desktop-app-terminal.css'",
		"'settings'",
		"'calculator'",
```

with:

```go
		"'/css/desktop-app-terminal.css'",
		"'/css/desktop-app-ecalc.css'",
		"'settings'",
		"'calculator'",
		"ecalc: {",
```

- [ ] **Step 2: Run them and see them fail**

```powershell
go test ./ui -run '^(TestECalcIsWiredIntoTheDesktopCore|TestDesktopAppAssetsRegistryCoversLazyCSSForAllBuiltinApps)$' -count=1
```

Expected: FAIL, `module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS` and `desktop app asset registry missing lazy CSS marker "'/css/desktop-app-ecalc.css'"`.

- [ ] **Step 3: Register the app (exact edits)**

`ui/js/desktop/core/module-loader.js`, the entry goes in front of `'minigolf'`; the composition root `ecalc.js` is last:

Replace:

```javascript
        'minigolf': {
            styles: appStyles('/css/desktop-app-minigolf.css'),
```

with:

```javascript
        ecalc: {
            styles: appStyles('/css/desktop-app-ecalc.css'),
            scripts: ['/js/desktop/apps/ecalc/ecalc-expr.js', '/js/desktop/apps/ecalc/ecalc-units.js', '/js/desktop/apps/ecalc/ecalc-eseries.js', '/js/desktop/apps/ecalc/ecalc-solve.js', '/js/desktop/apps/ecalc/ecalc-typeset.js', '/js/desktop/apps/ecalc/ecalc-symbols.js', '/js/desktop/apps/ecalc/ecalc-schematics.js', '/js/desktop/apps/ecalc/ecalc-store.js', '/js/desktop/apps/ecalc/ecalc-calc-page.js', '/js/desktop/apps/ecalc/ecalc-shell.js', '/js/desktop/apps/ecalc/ecalc.js']
        },
        'minigolf': {
            styles: appStyles('/css/desktop-app-minigolf.css'),
```

`ui/js/desktop/core/menus-and-routing.js`, the dispatch goes in front of the `tresor` branch:

Replace:

```javascript
        if (appId === 'tresor') {
            if (!window.TresorApp) {
```

with:

```javascript
        if (appId === 'ecalc') {
            if (!window.ECalcApp) {
                window.AuraDesktopModules.loadAppScript('ecalc').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.ECalcApp.render === 'function') {
                return window.ECalcApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t }));
            }
        }
        if (appId === 'tresor') {
            if (!window.TresorApp) {
```

`ui/js/desktop/core/desktop-foundation.js`, the global name makes `dispose(windowId)` run when the window closes:

Replace:

```javascript
            findit: 'FindItApp',
            tresor: 'TresorApp',
```

with:

```javascript
            findit: 'FindItApp',
            ecalc: 'ECalcApp',
            tresor: 'TresorApp',
```

`ui/js/desktop/core/window-shell-runtime.js`, the default size 1100×760 (the window is resizable: only `calculator` is not) and the minimum 640×480:

Replace:

```javascript
            findit: { width: 1320, height: 820 },
            tresor: { width: 1060, height: 700 },
```

with:

```javascript
            findit: { width: 1320, height: 820 },
            ecalc: { width: 1100, height: 760 },
            tresor: { width: 1060, height: 700 },
```

Replace:

```javascript
        if (appId === 'findit') return { width: 360, height: 480 };
        if (appId === 'radio')
```

with:

```javascript
        if (appId === 'findit') return { width: 360, height: 480 };
        if (appId === 'ecalc') return { width: 640, height: 480 };
        if (appId === 'radio')
```

Developer notes: insert this section into `ui/js/desktop/apps/AGENTS.md` in front of `## Email exports` (base line 30):

```markdown
## ECalc

- `ecalc/` holds eleven classic scripts in this load order (module-loader entry `ecalc`, pinned by `TestECalcIsWiredIntoTheDesktopCore`): the DOM-free core `ecalc-expr.js`, `ecalc-units.js`, `ecalc-eseries.js`, `ecalc-solve.js` (`window.ECalcCore`, a mirror of `internal/ecalc`; no `eval`, no `Function`), `ecalc-typeset.js`, `ecalc-symbols.js`, `ecalc-schematics.js` (exactly `window.ECalcSchematics = {JSON};`), `ecalc-store.js`, `ecalc-calc-page.js`, `ecalc-shell.js`, then the composition root `ecalc.js` (`window.ECalcApp`). Style: `css/desktop-app-ecalc.css`, tokens only, motion gated. The catalog comes from `GET /api/ecalc/catalog?lang=` once per window; every string is a `desktop.ecalc_*` key.
- The core must give the results of Go. `scripts/test-ecalc-core.mjs` (`npm run test:ecalc`) runs `internal/ecalc/testdata/vectors/**` and `testdata/catalog.en.json` against it: change an expression rule, a unit or the solver in Go, in JavaScript and in the vectors together. Inputs persist as raw texts under `aurago.ecalc.v1`; a computed value that becomes an input keeps its value with 12 significant digits. Lists keep the decimal point in every language because the comma separates their items.
- The page never observes itself (its layout changes its own height and would loop the ResizeObserver); the shell observes the page host and the root. Field chips sit at the anchors of the schematic only when the page is as wide as the drawing plus the stage padding, otherwise they are listed under it; below 720 px the shell is one stack with a back button. Ctrl/Cmd+K focuses the search and stops propagation so the desktop Spotlight stays closed.
- A schematic is a grid spec (20 px units) keyed by the calculator id. `TestECalcSchematicsMatchTheCatalog` checks ids, anchors and symbols against the catalog; `TestDesktopECalcBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`) checks that the chips of every schematic stay apart (and those of the divider in every desktop skin), plus live solving, locks, the star-delta set switch, the compact stack and Ctrl+K.
```

- [ ] **Step 4: Rebuild the bundle and check it**

```powershell
npm run build:ui
npm run build:ui -- --check
```

Expected: `Built ui/js/desktop/bundles/main.bundle.js (59 parts)` and, for the check, a `Checked ...` line per bundle and no `is stale` message. `git diff --stat ui/js/desktop/bundles/main.bundle.js` shows 12 inserted lines (the three core edits).

- [ ] **Step 5: Run the tests and the registration guards and see them pass**

```powershell
go test ./ui -run '^(TestECalc|TestDesktopAppAssets|TestDesktopAppMaterials|TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopBuiltinAppNamesExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal|TestUIAvoidsCookieConsentClassPrefix|TestDesktopThemeBundle|TestDesktopInitialLoad)' -count=1
```

Expected: `ok  	aurago/ui` (needs Part B's locale files and `BuiltinApps()` entry for the two i18n tests; `TestDesktopAppMaterialsAdoptedByApps` now also reads `css/desktop-app-ecalc.css` through the loader and finds `--vd-mat-`).

- [ ] **Step 6: Commit**

```powershell
git add ui/js/desktop/core/module-loader.js ui/js/desktop/core/menus-and-routing.js ui/js/desktop/core/desktop-foundation.js ui/js/desktop/core/window-shell-runtime.js ui/js/desktop/bundles/main.bundle.js ui/desktop_ecalc_test.go ui/frontend_optimization_test.go ui/js/desktop/apps/AGENTS.md
git commit -m "feat(ecalc): register the ECalc app in the desktop loader, router and window presets

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task C6: Browser test and final verification

**Files:**
- Create: `ui/desktop_ecalc_browser_test.go`

One browser test, gated like the other smokes (`AURAGO_RUN_BROWSER_SMOKE=1`). It runs the real desktop shell with the real catalog (served from `ecalc.Default()` in the language the app asks for; the German `de.json` makes the desktop German, so numbers appear with a decimal comma): the failing-catalog retry, the rail and list, the German page title, list entry and category (the harness must serve the requested language, otherwise an English catalog hides in a German desktop), that no rail label of any of the 16 locales is cut off (they wrap), the divider defaults, live solving (R2 changes V_out; typing V_out makes the oldest input, R2, computed), an unreadable text, a lock, reset, the unit dropdown, the favorite, the star-delta set switch, the field chips of all seven schematics (inside the drawing, apart from each other) and of the voltage divider in every desktop skin of the registry, the decibel choice, Ctrl+K (focus on a button, so the desktop would open its Spotlight if the event were not stopped), the search by tag, by English name (the background `?lang=en` catalog; the harness renames one English item to a word no German name or tag contains) and no result, and the compact stack at 390×667 (back button, listed fields, no sideways scroll). It saves screenshots when `AURAGO_BROWSER_ARTIFACT_DIR` is set.

- [ ] **Step 1: Write the browser test**

Create `ui/desktop_ecalc_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/ecalc"
	"github.com/go-rod/rod/lib/input"
)

// TestDesktopECalcBrowser drives ECalc inside the real desktop shell with the catalog of the server route: live solving
// on the voltage divider (the oldest input yields, a lock pins a field), the star-delta set switch, the field chips on
// every schematic, the compact stack and the Ctrl+K shortcut that must not open the Spotlight. The shell runs in
// German: numbers appear with a decimal comma and the app must show the German catalog (names and categories), while
// the search still finds items by their English name. Set AURAGO_BROWSER_ARTIFACT_DIR to keep screenshots.
func TestDesktopECalcBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	// names reads the item and category names of one language from the same JSON the route serves.
	names := func(lang string) (items, categories map[string]string, categoryNames []string) {
		t.Helper()
		body, _, err := catalog.LocalizedJSON(lang)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Items      []struct{ ID, Name string } `json:"items"`
			Categories []struct{ ID, Name string } `json:"categories"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatal(err)
		}
		items, categories = map[string]string{}, map[string]string{}
		for _, item := range parsed.Items {
			items[item.ID] = item.Name
		}
		for _, category := range parsed.Categories {
			categories[category.ID] = category.Name
			categoryNames = append(categoryNames, category.Name)
		}
		return items, categories, categoryNames
	}
	deItems, deCategories, _ := names("de")
	enItems, enCategories, _ := names("en")
	if deItems["voltage_divider"] == "" || deItems["voltage_divider"] == enItems["voltage_divider"] || deCategories["basics"] == enCategories["basics"] {
		t.Fatalf("the German and the English catalog must differ: %q / %q and %q / %q", deItems["voltage_divider"], enItems["voltage_divider"], deCategories["basics"], enCategories["basics"])
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/ecalc-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/ecalc-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/ecalc-catalog.json", func(w http.ResponseWriter, r *http.Request) {
		body, _, err := catalog.LocalizedJSON(r.URL.Query().Get("lang"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustSetViewport(1366, 900, 1, false)
	// The fixture answers every /api/ request with an empty object; the catalog route gets the real catalog of the
	// requested language, or a failure while window.ecalcCatalogDown is set. The English catalog (the background request
	// that feeds the search) names the voltage divider "Zzyzx divider", a word no German name or tag contains.
	page.MustEval(`async()=>{
		await fixtureReady;
		window.ecalcCatalogRequests=[];
		const base=window.fetch;
		window.fetch=(url,options)=>{
			const text=String(url);
			if(!text.startsWith('/api/ecalc/catalog'))return base(url,options);
			window.ecalcCatalogRequests.push(text);
			if(window.ecalcCatalogDown)return Promise.resolve(new Response('{"error":"down"}',{status:500,headers:{'Content-Type':'application/json'}}));
			const reply=base('/ecalc-catalog.json'+text.slice('/api/ecalc/catalog'.length));
			if(!text.includes('lang=en'))return reply;
			return reply.then(r=>r.json()).then(j=>{
				j.items.find(i=>i.id==='voltage_divider').name='Zzyzx divider';
				return new Response(JSON.stringify(j),{headers:{'Content-Type':'application/json'}});
			});
		};
	}`)

	shot := func(name string) {
		t.Helper()
		dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(dir, name+".png"))
	}
	wait := func(expr string) {
		t.Helper()
		waitForJSBool(t, page, expr)
	}
	field := func(name string) string { return `.ec-field[data-var="` + name + `"]` }
	value := func(name string) string {
		return page.MustEval(`n=>document.querySelector('.ec-field[data-var="'+n+'"] .ec-input').value`, name).Str()
	}
	state := func(name string) string {
		return page.MustEval(`n=>document.querySelector('.ec-field[data-var="'+n+'"]').dataset.state`, name).Str()
	}
	typeInto := func(name, text string) {
		t.Helper()
		el := page.MustElement(field(name) + " .ec-input")
		el.MustClick()
		el.MustSelectAllText()
		el.MustInput(text)
	}
	setUnit := func(name, unit string) {
		t.Helper()
		page.MustEval(`(n,u)=>{const s=document.querySelector('.ec-field[data-var="'+n+'"] .ec-unit');s.value=u;s.dispatchEvent(new Event('change',{bubbles:true}));}`, name, unit)
	}
	visible := func(selector string) bool {
		return page.MustEval(`s=>{const e=document.querySelector(s);return !!e&&getComputedStyle(e).display!=='none'&&e.getClientRects().length>0}`, selector).Bool()
	}
	openApp := func() {
		t.Helper()
		page.MustEval(`async()=>{await fixtureOpen('ecalc');}`)
		wait(`()=>!!document.querySelector('.ec-page[data-calc]')`)
	}
	openCalc := func(id string) {
		t.Helper()
		page.MustElement(`.ec-item[data-id="` + id + `"]`).MustClick()
		wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='` + id + `'`)
	}
	// The field chips of a schematic must stay inside the drawing and apart from each other.
	chipIssues := `()=>{
		const wrap=document.querySelector('.ec-sch-wrap');
		if(!wrap)return '';
		const box=wrap.getBoundingClientRect(),issues=[];
		const chips=[...wrap.querySelectorAll('.ec-chips > .ec-field')].map(c=>({id:c.dataset.var,r:c.getBoundingClientRect()}));
		chips.forEach((a,i)=>{
			if(a.r.left<box.left-1||a.r.right>box.right+1||a.r.top<box.top-1||a.r.bottom>box.bottom+1)issues.push(a.id+' sticks out of the drawing');
			chips.slice(i+1).forEach(b=>{if(a.r.left<b.r.right&&b.r.left<a.r.right&&a.r.top<b.r.bottom&&b.r.top<a.r.bottom)issues.push(a.id+' overlaps '+b.id);});
		});
		return issues.join('; ');
	}`

	// ---- opening: the rail, the list and the first calculators -------------------------------------------------
	page.MustEval(`()=>fixtureTheme('standard')`)
	// A failing catalog shows the failure and a retry button that loads it.
	page.MustEval(`async()=>{window.ecalcCatalogDown=true;await fixtureOpen('ecalc');}`)
	wait(`()=>!!document.querySelector('.ec-fail [data-action="retry"]') && document.querySelector('.ec-fail p').textContent.length>0`)
	page.MustEval(`()=>{window.ecalcCatalogDown=false;}`)
	page.MustElement(`.ec-fail [data-action="retry"]`).MustClick()
	wait(`()=>!!document.querySelector('.ec-page[data-calc]')`)
	wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="favorites"]') && !!document.querySelector('.ec-rail-btn[data-rail="basics"]')`)
	if got := page.MustEval(`()=>document.querySelectorAll('.ec-items .ec-item').length`).Int(); got != 11 {
		t.Fatalf("the basics category lists %d calculators, want 11", got)
	}
	openCalc("voltage_divider")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value!==''`)
	// The desktop language is German: the page, the list and the rail show the German catalog.
	if got := page.MustEval(`()=>document.querySelector('.ec-title').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the page title is %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-item[data-id="voltage_divider"] .ec-item-name').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the list shows %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="basics"] .ec-rail-label').textContent`).Str(); got != deCategories["basics"] {
		t.Fatalf("the rail shows %q, want the German category %q", got, deCategories["basics"])
	}
	if got := page.MustEval(`()=>JSON.stringify(window.ecalcCatalogRequests.filter(u=>!u.includes('lang=de')&&!u.includes('lang=en')))`).Str(); got != "[]" {
		t.Fatalf("the app asked for catalogs in languages other than the desktop's: %s", got)
	}
	// Every rail label of every language wraps instead of being cut off.
	labels := map[string][]string{}
	for _, lang := range ecalc.Locales {
		words := readMergedLangMap(t, lang)
		_, _, categoryNames := names(lang)
		labels[lang] = append([]string{words["desktop.ecalc_favorites"], words["desktop.ecalc_recent"]}, categoryNames...)
	}
	if got := page.MustEval(`labels=>{
		const rail=document.querySelector('.ec-rail'),button=rail.querySelector('.ec-rail-btn[data-rail="recent"]').cloneNode(true);
		rail.appendChild(button);
		const label=button.querySelector('.ec-rail-label'),issues=[],line=parseFloat(getComputedStyle(label).lineHeight)||18;
		for(const [lang,texts] of Object.entries(labels)){
			button.lang=lang;
			for(const text of texts){
				label.textContent=text;
				if(label.scrollWidth>label.clientWidth+1)issues.push(lang+': "'+text+'" is cut off');
				if(label.clientHeight>line*3+2)issues.push(lang+': "'+text+'" needs more than three lines');
			}
		}
		button.remove();
		return issues.join('; ');
	}`, labels).Str(); got != "" {
		t.Fatalf("rail labels: %s", got)
	}
	if got := []string{value("vin"), value("r1"), value("r2"), value("vout")}; got[0] != "12 V" || got[1] != "4,7 kΩ" || got[2] != "2,2 kΩ" || got[3] != "3,826 V" {
		t.Fatalf("divider defaults %v, want 12 V, 4,7 kΩ, 2,2 kΩ and 3,826 V", got)
	}
	if state("vin") != "input" || state("vout") != "computed" {
		t.Fatalf("states vin=%s vout=%s, want input and computed", state("vin"), state("vout"))
	}
	if got := page.MustEval(chipIssues).Str(); got != "" {
		t.Fatalf("divider chips: %s", got)
	}
	if !page.MustEval(`()=>!!document.querySelector('.ec-frac') && !!document.querySelector('.ec-live .ec-v') && !!document.querySelector('.ec-sch path')`).Bool() {
		t.Fatal("the divider page must show the typeset fraction, the live line and the schematic")
	}
	shot("voltage-divider-standard")

	// ---- every desktop skin keeps the chips apart; three are kept as screenshots ---------------------------------------
	for _, theme := range parseDesktopThemeRegistry(t) {
		names := []string{theme.ID}
		if theme.ID == "fruity" {
			names = []string{"fruity-light", "fruity-dark"}
		}
		for _, name := range names {
			page.MustEval(`id=>fixtureTheme(id)`, name)
			if got := page.MustEval(chipIssues).Str(); got != "" {
				t.Fatalf("skin %s: divider chips: %s", name, got)
			}
			if name == "fruity-light" || name == "minimal" || name == "cyberpunk" {
				shot("voltage-divider-" + name)
			}
		}
	}
	page.MustEval(`()=>fixtureTheme('standard')`)

	// ---- live solving: R2 changes V_out; typing V_out makes the oldest input (R2) computed ----------------------
	typeInto("r2", "3k3")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='4,95 V'`)
	typeInto("vin", "10")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='4,125 V'`)
	typeInto("r1", "5k")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3,976 V'`)
	typeInto("vout", "3")
	wait(`()=>document.querySelector('.ec-field[data-var="r2"]').dataset.state==='computed' && document.querySelector('.ec-field[data-var="r2"] .ec-input').value==='2,143 kΩ'`)
	if state("vout") != "input" || state("vin") != "input" || state("r1") != "input" {
		t.Fatalf("after typing V_out the states are vin=%s r1=%s vout=%s, want inputs", state("vin"), state("r1"), state("vout"))
	}
	// An unreadable text changes nothing and says so; leaving the field restores the last good value.
	typeInto("r1", "abc")
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='error' && document.querySelector('.ec-field[data-var="r1"] .ec-field-msg').textContent.length>0`)
	page.MustElement(field("vin") + " .ec-input").MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='input' && document.querySelector('.ec-field[data-var="r1"] .ec-input').value==='5 kΩ'`)

	// ---- lock: a locked input never yields -----------------------------------------------------------------------
	page.MustElement(field("vin") + " .ec-lock").MustClick()
	if page.MustEval(`()=>document.querySelector('.ec-field[data-var="vin"] .ec-lock').getAttribute('aria-pressed')`).Str() != "true" {
		t.Fatal("the lock button must be pressed")
	}
	typeInto("r2", "2k")
	wait(`()=>document.querySelector('.ec-field[data-var="r1"]').dataset.state==='computed'`)
	if state("vin") != "input" || state("r2") != "input" || state("vout") != "input" {
		t.Fatalf("with V_in locked the states are vin=%s r2=%s vout=%s, want inputs", state("vin"), state("r2"), state("vout"))
	}
	page.MustElement(`[data-action="reset"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="r2"] .ec-input').value==='2,2 kΩ' && document.querySelector('.ec-field[data-var="vin"] .ec-lock').getAttribute('aria-pressed')==='false'`)

	// ---- unit dropdown and the favorite -------------------------------------------------------------------------
	setUnit("vout", "prefix:m")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3826 mV'`)
	setUnit("vout", "")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='3,826 V'`)
	page.MustElement(`[data-action="favorite"]`).MustClick()
	wait(`()=>document.querySelector('.ec-rail-btn[data-rail="favorites"] .ec-count').textContent==='1'`)

	// ---- star-delta: typing a star value switches the whole set -----------------------------------------------
	openCalc("star_delta")
	wait(`()=>document.querySelector('.ec-field[data-var="ra"] .ec-input').value==='15 Ω'`)
	if state("rab") != "input" || state("ra") != "computed" {
		t.Fatalf("star-delta starts with rab=%s ra=%s, want input and computed", state("rab"), state("ra"))
	}
	shot("star-delta-standard")
	typeInto("ra", "12")
	wait(`()=>document.querySelector('.ec-field[data-var="rab"]').dataset.state==='computed' && document.querySelector('.ec-field[data-var="ra"]').dataset.state==='input'`)
	if state("rb") != "input" || state("rc") != "input" || value("rb") != "10 Ω" || value("rc") != "30 Ω" {
		t.Fatalf("after the switch rb=%s %q rc=%s %q, want the computed values kept as inputs", state("rb"), value("rb"), state("rc"), value("rc"))
	}
	wait(`()=>document.querySelector('.ec-field[data-var="rab"] .ec-input').value==='26 Ω'`)

	// ---- every schematic keeps its field chips apart and inside the drawing ---------------------------------------------
	for _, id := range []string{"ohms_law", "voltage_divider", "loaded_divider", "current_divider", "series_parallel", "wheatstone", "star_delta"} {
		openCalc(id)
		shot("schematic-" + id)
		if got := page.MustEval(`()=>document.querySelector('.ec-page').dataset.chips`).Str(); got != "anchored" {
			t.Fatalf("%s: the fields are %q at the default window size, want anchored on the schematic", id, got)
		}
		if got := page.MustEval(chipIssues).Str(); got != "" {
			t.Fatalf("%s chips: %s", id, got)
		}
		if id == "series_parallel" {
			if got := value("rs"); got != "4.7kΩ, 10kΩ, 2.2kΩ" {
				t.Fatalf("the resistor list shows %q, want decimal points (the comma separates the items)", got)
			}
		}
	}
	openCalc("decibel_ratio")
	page.MustElement(`.ec-choice input[value="power"]`).MustClick()
	wait(`()=>document.querySelector('.ec-field[data-var="g"] .ec-input').value==='10 dB'`)

	// ---- Ctrl+K belongs to the app: the search gets the focus, the Spotlight stays closed ----------------------------
	// The desktop ignores keys typed into fields, so the focus sits on a button when the shortcut is pressed.
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustFocus()
	page.KeyActions().Press(input.ControlLeft).Type('k').MustDo()
	wait(`()=>document.activeElement&&document.activeElement.classList.contains('ec-search-input')`)
	if page.MustEval(`()=>!!document.getElementById('vd-spotlight-backdrop')`).Bool() {
		t.Fatal("Ctrl+K inside ECalc must not open the desktop Spotlight")
	}
	search := page.MustElement(".ec-search-input")
	search.MustInput("teiler")
	wait(`()=>[...document.querySelectorAll('.ec-items .ec-item')].some(b=>b.dataset.id==='voltage_divider')`)
	// The English name finds the item although the catalog and the page are German (the background English request).
	search.MustSelectAllText().MustInput("zzyzx")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===1 && document.querySelector('.ec-items .ec-item').dataset.id==='voltage_divider'`)
	if got := page.MustEval(`()=>document.querySelector('.ec-items .ec-item .ec-item-name').textContent`).Str(); got != deItems["voltage_divider"] {
		t.Fatalf("the English-name hit shows %q, want the German name %q", got, deItems["voltage_divider"])
	}
	if got := page.MustEval(`()=>window.ecalcCatalogRequests.some(u=>u.includes('lang=en'))`).Bool(); !got {
		t.Fatal("the app never asked for the English catalog that feeds the search")
	}
	search.MustSelectAllText().MustInput("star delta")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===1 && document.querySelector('.ec-items .ec-item').dataset.id==='star_delta'`)
	search.MustSelectAllText().MustInput("zzzz")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===0 && !document.querySelector('.ec-empty').hidden`)
	search.MustSelectAllText().MustInput(" ")
	wait(`()=>document.querySelectorAll('.ec-items .ec-item').length===11`)
	openCalc("voltage_divider")

	// ---- compact: one stack with a back button -------------------------------------------------------------------
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	page.MustSetViewport(390, 667, 1, false)
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	openApp()
	wait(`()=>document.querySelector('.ec-app').dataset.compact==='true'`)
	if !visible(".ec-page-host") || visible(".ec-rail") || visible(".ec-list") || !visible(".ec-back") {
		t.Fatal("compact mode must start on the page of the last calculator with the back button shown")
	}
	if got := page.MustEval(`()=>document.querySelector('.ec-page').dataset.chips`).Str(); got != "listed" {
		t.Fatalf("on a phone the fields are %q, want listed under the schematic", got)
	}
	shot("compact-page")
	if got := page.MustEval(`()=>{const p=document.querySelector('.ec-page-host');return p.scrollWidth>p.clientWidth+1?'the page scrolls sideways':'';}`).Str(); got != "" {
		t.Fatal(got)
	}
	page.MustElement(".ec-back").MustClick()
	if !visible(".ec-list") || visible(".ec-page-host") || visible(".ec-rail") {
		t.Fatal("back from the page must show the list")
	}
	shot("compact-list")
	page.MustElement(".ec-back").MustClick()
	if !visible(".ec-rail") || visible(".ec-list") || visible(".ec-back") {
		t.Fatal("back from the list must show the rail and hide the back button")
	}
	page.MustElement(`.ec-rail-btn[data-rail="basics"]`).MustClick()
	page.MustElement(`.ec-item[data-id="voltage_divider"]`).MustClick()
	wait(`()=>document.querySelector('.ec-page')?.dataset.calc==='voltage_divider'`)
	if !visible(".ec-page-host") {
		t.Fatal("opening a calculator must show its page")
	}
	typeInto("r2", "10k")
	wait(`()=>document.querySelector('.ec-field[data-var="vout"] .ec-input').value==='8,163 V'`)

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
```

- [ ] **Step 2: Run it**

The test exercises code that already exists, so it is expected to pass at once; a failure names the broken step (the message is the JavaScript condition that never became true, or the field state that was wrong). Run it with the sandbox disabled:

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
$env:AURAGO_BROWSER_ARTIFACT_DIR = 'C:\tmp\ecalc-shots'   # optional: screenshots of the divider in three skins, the schematics and the compact stack
go test ./ui -run TestDesktopECalcBrowser -count=1 -v
```

Expected: `--- PASS: TestDesktopECalcBrowser` and `ok  	aurago/ui`.

- [ ] **Step 3: Final verification of the whole part**

```powershell
Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE, Env:AURAGO_BROWSER_ARTIFACT_DIR -ErrorAction SilentlyContinue   # the broad run below must not start every other browser smoke
npm run test:ecalc
npm run build:ui -- --check
gofmt -l ui/desktop_ecalc_test.go ui/desktop_ecalc_schematics_test.go ui/desktop_ecalc_browser_test.go
go vet ./ui
go test ./ui -run '^(TestECalc|TestDesktopECalc|TestDesktopAppMaterials|TestDesktopCSS|TestDesktopFruity|TestDesktopThemes|TestDesktopEverydayApps|TestDesktopAppAssets|TestDesktopUsedI18n|TestDesktopBuiltinAppNames|TestShippedWeb|TestUIAvoids|TestDesktopThemeBundle|TestDesktopInitialLoad)' -count=1
```

Expected: `ecalc core tests: ... cases ok`; the bundle check clean; `gofmt -l` and `go vet` print nothing; `ok  	aurago/ui`. (A full `go test ./ui` run in the verified worktree also reported `TestChatFrontend_IntegrationsDrawerRemainsWired` and `TestFrontend_StaticI18nKeysExistInEnglishBundle` (FindIt keys); both read files that this part does not touch and are unrelated to ECalc.)

- [ ] **Step 4: Commit**

```powershell
git add ui/desktop_ecalc_browser_test.go
git commit -m "test(ecalc): browser test for live solving, schematic fields, skins, compact stack and Ctrl+K

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


---

## Appendix: shared contract given to the drafters

The parts above reference it as "contract §N". Each part's "Contract deviations" list overrides it.

## ECalc phase 1 — shared contract for the three plan drafters

Spec (approved, read it first): `docs/superpowers/specs/2026-11-01-ecalc-design.md` (commit a323eae0a).
Phase 1 = foundation: `internal/ecalc` core + 11 basics calculators, JS mirror with parity tests, catalog API,
app shell + calculator page (layout C), schematic symbol library, registration, icon, i18n, manual skeleton.
NOT in phase 1: agent tool, tables, codes, pinouts, `lookup`/`interp` (phase 3), standard-value combiner (phase 2).

Parts: **A** Go core + catalog content + vectors (owner of `internal/ecalc/**`). **B** server route, audit contract,
desktop registration (Go side), icon, i18n keys, manual skeleton. **C** all UI (JS/CSS), core shell wiring, bundle,
UI tests, node parity test, browser test. Everything below is binding for all three; if you must deviate, write the
deviation at the top of your plan part under "Contract deviations" and keep it minimal.

### 0. Repo facts

- Go module `aurago`, Go 1.27, YAML lib `gopkg.in/yaml.v3` (use `yaml.Node` where order matters).
- 16 desktop locales: cs da de el en es fr hi it ja nl no pl pt sv zh.
- Node test scripts live in `scripts/test-*.mjs` and are wired as `npm run test:<name>` in `package.json`.
- Core shell files are bundled into `ui/js/desktop/bundles/main.bundle.js`: after editing them run `npm run build:ui`
  and `npm run build:ui -- --check`.
- Windows box. Go: set `GOTMPDIR` to a short BACKSLASH path (e.g. `C:\tmp\go-build-ecalc`), never a forward-slash or
  scratchpad path. If D: is full (`GOCACHE=D:\dev-cache\go-build`), use a per-agent GOCACHE under your scratch dir.
- Browser tests (`AURAGO_RUN_BROWSER_SMOKE=1`) need Chrome started from a test binary: run them with the PowerShell
  tool and `dangerouslyDisableSandbox: true`; binaries built under the repo cannot start Chrome (crashpad access denied).
- The Write/Edit tools decode backslash-u escapes; in Go source write the backslash as `\x5c` if you ever need one.
- GitNexus rule of this repo: before modifying an existing function, run impact analysis from the MAIN checkout
  (`cd C:\Users\Andi\Documents\repo\AuraGo && node .gitnexus/run.cjs impact "<symbol>" --direction upstream --repo .`)
  and note callers/risk in the plan task. Warn on HIGH/CRITICAL; treat UNKNOWN as unresolved (confirm by grep).
- Plan execution will happen in a git worktree on its own branch: commit steps in the plan use plain
  `git add <explicit paths>` + `git commit -m "..."` (never `git add -A`). Commit messages end with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

### 1. File layout (phase 1)

```
internal/ecalc/                         (A)
  doc.go                package doc
  expr.go               lexer, parser, AST, Eval, Deps
  units.go              Units model, Parse, Format, prefix table
  eseries.go            E3…E192 tables, Nearest, roundSig
  solve.go              Solve, SolvableSets (graph-based)
  catalog.go            YAML schema, Load, Default, validation, Item lookup
  localize.go           LocalizedJSON(lang) + ETag + per-lang cache
  catalog/units.yaml
  catalog/categories.yaml
  catalog/calculators/basics.yaml       the 11 calculators below
  testdata/vectors/core.json            expr/parse/format/eseries vectors
  testdata/vectors/calculators/<id>.json   solve vectors per calculator
  testdata/catalog.en.json              generated snapshot (Go test with -update regenerates; fails when stale)
  *_test.go
internal/server/ecalc_handlers.go (+ test)            (B)
internal/audit/audit_manifests.go  route contract entry   (B)
internal/desktop/types.go  BuiltinApps entry + icon      (B)   + internal/desktop/ecalc_app_test.go
ui/lang/desktop/*.json  (16)   desktop.app_ecalc + desktop.ecalc_* keys (B writes all 16; C uses them)
documentation/manuals/{en,de}/32-ecalc.md + index rows   (B)
ui/js/desktop/apps/ecalc/                (C)
  ecalc-expr.js  ecalc-units.js  ecalc-eseries.js  ecalc-solve.js     DOM-free core (node-testable)
  ecalc-typeset.js     AST -> HTML (formula + live line)
  ecalc-symbols.js     IEC/ANSI symbol library -> SVG
  ecalc-schematics.js  `window.ECalcSchematics = {STRICT JSON};` (exactly this prefix, JSON body, trailing `;`)
  ecalc-store.js       localStorage prefs (try/catch everywhere)
  ecalc-calc-page.js   calculator page
  ecalc-shell.js       rail, list, search, favorites/recent, compact stack
  ecalc.js             composition root: window.ECalcApp = { render(hostEl, windowId, ctx), dispose(windowId) }
ui/css/desktop-app-ecalc.css             (C)  --vd-mat-* / --ds-color-* only
scripts/test-ecalc-core.mjs  (+ npm script test:ecalc)  (C)
ui/desktop_ecalc_test.go, ui/desktop_ecalc_browser_test.go, ui/desktop_ecalc_schematics_test.go   (C)
core wiring (C): ui/js/desktop/core/module-loader.js, menus-and-routing.js, desktop-foundation.js,
  window-shell-runtime.js, bundles; guard lists in ui/frontend_optimization_test.go, ui/desktop_theme_consistency_test.go
```

### 2. YAML catalog schema

#### units.yaml
```yaml
prefixes:            # order = ascending factor; input also accepts u for µ and K for k
  - {id: p, factor: 1e-12}
  - {id: n, factor: 1e-9}
  - {id: µ, factor: 1e-6}
  - {id: m, factor: 1e-3}
  - {id: k, factor: 1e3}
  - {id: M, factor: 1e6}
  - {id: G, factor: 1e9}
dims:                # mapping, order significant
  voltage:    {symbol: V, prefixes: [n, µ, m, "", k, M],
               units: [{id: dBV, symbol: dBV, to: "10^(x/20)", from: "20*log10(x)"},
                       {id: dBu, symbol: dBu, to: "0.7745966692414834*10^(x/20)", from: "20*log10(x/0.7745966692414834)"}]}
  current:    {symbol: A, prefixes: [p, n, µ, m, "", k]}
  resistance: {symbol: Ω, aliases: [ohm, Ohm, R], prefixes: [m, "", k, M, G], positive: true}
  power:      {symbol: W, prefixes: [n, µ, m, "", k, M],
               units: [{id: dBm, symbol: dBm, to: "10^((x-30)/10)", from: "10*log10(x)+30"}]}
  charge:     {symbol: C, prefixes: [µ, m, ""], units: [{id: mAh, symbol: mAh, factor: 3.6}, {id: Ah, symbol: Ah, factor: 3600}]}
  energy:     {symbol: J, prefixes: [m, "", k, M], units: [{id: mWh, symbol: mWh, factor: 3.6}, {id: Wh, symbol: Wh, factor: 3600}, {id: kWh, symbol: kWh, factor: 3.6e6}]}
  number:     {symbol: "", prefixes: [], positive: true}
  gain:       {symbol: dB, prefixes: []}
```
- `""` in `prefixes` = no prefix. A unit has either `factor` (linear: base = x*factor) or `to`/`from` expressions in
  the expression language with variable `x` (to: display->base, from: base->display).
- `positive: true` = values must be > 0 (inputs and results), else error code `range`.

#### categories.yaml
Mapping of the 13 category ids, in rail order, each `{name: {16 locales}}`:
basics, passive, filters, semiconductors, power, opamps, timers, rf, digital, wiring, tables, codes, pinouts.
English names: Basics, Passive, Filters, Semiconductors, Power supply, Op-amps, Timers, RF, Digital, Wiring & PCB,
Tables, Codes, Pinouts. German: Grundlagen, Passive Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer,
HF, Digital, Leitungen & PCB, Tabellen, Bauteilcodes, Pinbelegungen.

#### calculators/*.yaml — a YAML sequence of items
```yaml
- id: voltage_divider              # [a-z0-9_]+, unique across all kinds
  kind: calculator
  category: basics
  name: {cs: …, da: …, de: Spannungsteiler, …, en: Voltage divider, …}     # all 16 required
  doc:  {en: "…", de: "…"}         # en+de required, others optional (fallback en)
  tags: [divider, teiler]
  source: "…"                      # required, honest (see §8)
  vars:                            # mapping, ORDER SIGNIFICANT
    vin:  {dim: voltage, label: V_in, name: {en: Input voltage, de: Eingangsspannung}, default: "12 V"}
    r1:   {dim: resistance, label: R_1, name: {…}, default: "4k7", eseries: true}
    r2:   {dim: resistance, label: R_2, name: {…}, default: "2k2", eseries: true}
    vout: {dim: voltage, label: V_out, name: {…}}
  choices:                         # optional mapping; each option's consts become known names
    qty: {name: {16}, default: voltage, options: {voltage: {name: {16}, consts: {k: 20}}, power: {name: {16}, consts: {k: 10}}}}
  dof: 3
  rules:                           # mapping target -> expr string OR list of alternative expr strings; order significant
    vout: vin*r2/(r1+r2)
    r2:   vout*r1/(vin-vout)
  derived:                         # mapping, order significant; may be referenced by rules (propagation includes them)
    i: {dim: current, label: I, name: {…}, expr: vin/(r1+r2)}          # expr: string or list of alternatives
    p_dbm: {dim: power, label: P, name: {…}, expr: p, unit: dBm}       # optional fixed display unit
  checks:
    - {when: "vout >= vin", warn: {en: "…", de: "…"}}
  formula: ["vout = vin * r2 / (r1 + r2)"]   # list of display lines, parsed with ParseFormula
  schematic: voltage_divider       # "" or omitted = no schematic
```
Var fields: `dim` (required), `label` (display symbol, `_` starts a subscript), `name` {en,de required},
`default` (string parsed with the dim's parser, or number = SI), `unit` (default display unit id, "" = auto),
`eseries` (bool), `list` (bool; default given as YAML list of strings/numbers), `input` (bool, default true).
Exactly `dof` input-capable vars carry a `default`; they are the initial inputs.

### 3. Expression language (identical in Go and JS)

```
formula  := expr ('=' expr)*                 (display only; ParseFormula)
expr     := cmp
cmp      := add (('<' | '<=' | '>' | '>=' | '==' | '!=') add)?
add      := mul (('+' | '-') mul)*
mul      := unary (('*' | '/') unary)*
unary    := ('-' | '+') unary | pow
pow      := atom ('^' unary)?                (right-assoc; -2^2 = -(2^2); 2^-1 ok)
atom     := number | ident '(' args? ')' | ident | '(' expr ')'
number   := digits ['.' digits] [('e'|'E') ['+'|'-'] digits]   |  '.' digits ...   (no SI prefixes in expressions)
ident    := [A-Za-z_][A-Za-z0-9_]*
```
- Whitespace ignored. Comparisons yield 1 or 0. Constant: `pi`. No assignment, no loops.
- Functions (exact arity): sqrt(1) ln(1) log10(1) exp(1) sin(1) cos(1) tan(1) atan(1) abs(1) round(1) min(≥2) max(≥2)
  if(3, lazy) sum(1 list) par(1 list; = 1/Σ(1/x)). Unknown function or wrong arity = parse error.
- Float semantics: IEEE-754 doubles; division by zero yields ±Inf / NaN (no error). `round` = round half away from zero.
  Evaluating an unknown identifier = eval error. A list var used outside sum/par = eval error.
- AST node shapes (JSON-like, used by C's typesetter, Go mirrors them in structs):
  `{t:"num", v:12, raw:"12"}` `{t:"id", name:"vin"}` `{t:"neg", a:…}` `{t:"pos", a:…}`
  `{t:"bin", op:"+|-|*|/|^|<|<=|>|>=|==|!=", l:…, r:…}` `{t:"call", name:"sqrt", args:[…]}`
  ParseFormula returns `{t:"eq", parts:[…]}`.
- `Deps` = sorted unique identifiers that are not function names and not `pi`.

### 4. Units: parse and format (identical in Go and JS)

Parse(input, dimId, unitId) -> base value (float) or error:
1. Trim. Remove all spaces. Empty -> error.
2. If the dim has explicit units (from `units:`), try them longest-symbol-first (case-sensitive): if input ends with
   the symbol and the rest is a plain number (`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`, comma already replaced),
   return toBase(rest).
3. Replace `,` by `.`.  (Do step 3 before step 2's number test — i.e. normalise commas first.)
4. Strip a trailing base symbol or alias (case-sensitive) if present (e.g. `Ω`, `ohm`, `R`, `V`). Also accept a prefix
   immediately before it (e.g. `kΩ`, `mA`) — that prefix is then handled by step 5.
5. Remaining must match one of: plain number; plain number + one prefix letter (`p n u µ m k K M G`); infix form
   `^(\d+)([pnuµmkKMG])(\d+)$` (4k7 = 4.7k); for dim `resistance` also infix `R` (4R7 = 4.7) and trailing `R`.
   Prefix letters allowed only if listed in the dim's `prefixes` (u→µ, K→k). Else error.
6. If unitId names an explicit unit and the input had no unit suffix: the plain number (no prefix allowed) is in that
   unit -> toBase. Else value*prefixFactor.
List fields: split on `,` `;` and whitespace, parse each item with steps 1–6 (no decimal comma in list fields).

Format(v, dimId, unitId, sig) -> string (sig default 4):
- NaN/±Inf -> "". If unitId is an explicit unit: x = fromBase(v), return `fmtSig(x, sig) + " " + symbol`.
- Auto: if dim has no prefixes: `fmtSig(v, sig)` + (symbol ? " "+symbol : ""). If v == 0: "0" + (" "+symbol).
  Else choose the largest prefix factor f (from the dim's prefixes, "" = 1) with f <= |v|; if none, the smallest;
  m = roundSig(v/f, sig); if |m| >= 1000 and a larger prefix exists, step up and recompute.
  Result `fmtNum(m) + " " + prefix + symbol` (symbol may be "" -> no trailing space, i.e. trim).
- roundSig(x, n): round to n significant digits (Go: strconv.FormatFloat(x,'e',n-1,64) then ParseFloat;
  JS: Number(x.toPrecision(n))). fmtNum: shortest round-trip decimal without exponent for 1e-6 <= |x| < 1e21
  (Go strconv 'f' -1; JS String(x)); vectors cover the exact strings. Decimal point is always "." in core;
  the UI swaps to "," for locales de, cs, da, el, es, fr, it, nl, no, pl, pt, sv only at display time.

### 5. E-series

`Nearest(series, v)` -> (value, devPct). Series ids E3 E6 E12 E24 E48 E96 E192 with the IEC 60063 tables (E3..E24
two significant digits incl. 2.7 3.0 3.3 3.6 3.9 4.3 4.7 8.2; E48..E192 three digits). For v <= 0 or non-finite:
error / null. d = 10^floor(log10 v); candidates = each table value*d plus 10*d; pick min |c-v|/v, tie -> lower;
c = roundSig(c, 12); devPct = (c-v)/v*100.

### 6. Solver (identical in Go and JS)

Solve(calc, inputs, choices) -> Result:
1. env = consts of the selected option of every choice (unknown/missing choice -> its default) + inputs.
   known = names in env.
2. Fixpoint: repeat { for each target in [rules targets in declaration order, then derived in declaration order]:
   if target not known: for each alternative in order: if Deps(alt) ⊆ known: env[target] = Eval(alt) (eval error ->
   NaN); known += target; break } until a full pass changes nothing.
3. errors: every input-capable or rule-target primary var still unknown -> "undetermined". Every known primary value
   (inputs included) that is NaN/±Inf -> "nonfinite"; else dim.positive && v <= 0 -> "range". For list inputs each
   element is checked (positive dims). Derived that stay unknown are NaN without error; derived non-finite -> NaN.
4. checks: evaluated in order when all deps are known and finite; result != 0 -> its index goes into `warnings`.
5. Result: `values` (primary + derived; NaN for unknown), `computed` (names computed in step 2), `errors`
   (name -> code), `warnings` (check indexes). JSON vectors write NaN as the string "nan".

SolvableSets(calc) (Go only; graph-based, no evaluation): all dof-sized combinations of input-capable primary vars,
in combination order of declaration indices, for which step 2's propagation (treating every alternative's deps,
consts always known) makes every primary var known. Validation errors: default set not solvable; an input-capable
var that is in no solvable set; dof > number of input-capable vars.

UI-only (JS) `chooseInputs(sets, recency, edited, locked)`: candidates = sets containing `edited` and every locked var
(if none, ignore locks). score(S) = Σ over v in S∩recency, v ≠ edited, of 2^(index of v in recency) (recency is
oldest→newest). Pick max score, tie -> first in `sets` order. New recency = vars of S∖{edited} that were in the old
recency (old order), preceded by the new ones (set order), then `edited` last.

### 7. Catalog JSON (GET /api/ecalc/catalog?lang=xx and testdata/catalog.en.json)

```json
{ "version": 1, "lang": "en",
  "units": {"prefixes":[{"id":"p","factor":1e-12}],
            "dims":[{"id":"voltage","symbol":"V","aliases":[],"prefixes":["n","µ","m","","k","M"],"positive":false,
                     "units":[{"id":"dBV","symbol":"dBV","factor":0,"to":"10^(x/20)","from":"20*log10(x)"}]}]},
  "categories": [{"id":"basics","name":"Basics"}],
  "items": [{"kind":"calculator","id":"voltage_divider","category":"basics","name":"Voltage divider","doc":"…",
     "tags":["divider"],"source":"…",
     "vars":[{"id":"vin","dim":"voltage","label":"V_in","name":"Input voltage","default":12,"default_list":null,
              "unit":"","eseries":false,"list":false,"input":true}],
     "choices":[{"id":"qty","name":"…","default":"voltage","options":[{"id":"voltage","name":"…","consts":{"k":20}}]}],
     "dof":3,
     "rules":[{"target":"vout","alts":["vin*r2/(r1+r2)"]}],
     "derived":[{"id":"i","dim":"current","label":"I","name":"…","alts":["vin/(r1+r2)"],"unit":""}],
     "checks":[{"when":"vout >= vin","warn":"…"}],
     "formula":["vout = vin * r2 / (r1 + r2)"],
     "schematic":"voltage_divider",
     "sets":[["vin","r1","r2"],["vin","r1","vout"]]}] }
```
- All ordered collections are arrays. Strings localized for `lang` with English fallback. Unknown/empty lang -> "en".
- Defaults are SI numbers (list defaults in `default_list`, `default` then null). Absent optional fields are emitted
  with their zero value (as shown) so JS never needs existence checks. `choices`/`checks`/`derived` are `[]` if none.
- Go API for B: `ecalc.Default() (*ecalc.Catalog, error)` (embedded, validated, sync.Once) and
  `(*Catalog).LocalizedJSON(lang string) (body []byte, etag string, err error)` (cached per normalized lang;
  etag = `"ecalc-<first 16 hex of sha256(body)>"`).
- HTTP (B): GET only (405 otherwise); same auth/middleware as the other desktop app routes; headers
  `Content-Type: application/json; charset=utf-8`, `ETag`, `Cache-Control: no-cache`; `If-None-Match` equal -> 304.
  500 with a generic message if the catalog failed to load (log the error).

### 8. Content honesty rules (A)

- `source` must be true: cite well-known works only at book/chapter level or a standard by number (e.g.
  "Ohm's law; P. Horowitz, W. Hill: The Art of Electronics, 3rd ed., ch. 1"). Never invent page numbers, URLs, or
  app-note numbers you cannot verify.
- Every rule direction has ≥ 2 solve vectors. Each vector has a `ref` string saying where the expected value comes
  from: an exact hand calculation written out (e.g. "hand: 12 V / 0.02 A = 600 Ω") or a definition
  ("definition: 0 dBu = sqrt(0.6) V"). Expected values must be computed independently of the implementation
  (exact arithmetic by hand / known constants), not by running the solver and pasting its output.
- Round-trip test over all calculators: solve defaults -> for every solvable set S, solve with S's values -> all
  primary values equal the default solution (rel 1e-9).

### 9. Phase 1 calculators (all category `basics`; A writes them, C draws schematics for those marked ⊞)

1. ⊞ `ohms_law` Ohm's law & power — vars u (voltage, U, "12 V"), i (current, I, "20 mA"), r (resistance, R, eseries),
   p (power, P); dof 2; rules u:[i*r, p/i, sqrt(p*r)] i:[u/r, p/u, sqrt(p/r)] r:[u/i, u^2/p, p/i^2] p:[u*i, i^2*r, u^2/r];
   formula ["u = r * i", "p = u * i"].
2. ⊞ `voltage_divider` — as in §2 (vin "12 V", r1 "4k7", r2 "2k2", vout); derived i = vin/(r1+r2), p = vin*i;
   check vout >= vin.
3. ⊞ `loaded_divider` — vin "12 V", r1 "10k", r2 "10k", rl "10k" (label R_L), vout; dof 4; derived rp (label R_P,
   name "R2 ∥ RL") expr [r2*rl/(r2+rl), vout*r1/(vin-vout)]; rules vout: vin*rp/(r1+rp), vin: vout*(r1+rp)/rp,
   r1: rp*(vin-vout)/vout, r2: rp*rl/(rl-rp), rl: rp*r2/(r2-rp); derived vnl (unloaded output) = vin*r2/(r1+r2),
   i = vin/(r1+rp); checks vout >= vin, rp >= r2 (impossible load).
4. ⊞ `current_divider` — i "100 mA", r1 "100", r2 "220", i1, i2; dof 3; rules i1:[i*r2/(r1+r2), i-i2, i2*r2/r1]
   i2:[i*r1/(r1+r2), i-i1, i1*r1/r2] i:[i1+i2, i1*(r1+r2)/r2, i2*(r1+r2)/r1] r1:[i2*r2/i1] r2:[i1*r1/i2];
   derived u = i1*r1. (Note: rules map order = vars order i, r1, r2, i1, i2 is fine; propagation loops anyway.)
5. ⊞ `series_parallel` — rs (resistance, list, label R, default ["4k7","10k","2k2"]), dof 1, no rules;
   derived rser = sum(rs) (label R_S), rpar = par(rs) (label R_P); formula ["rser = sum(rs)", "rpar = par(rs)"].
6. `decibel_ratio` — choice qty {voltage: k 20, power: k 10}; a1 (number, A_1, 1), a2 (number, A_2, 10), g (gain, G);
   dof 2; rules g: k*log10(a2/a1), a2: a1*10^(g/k), a1: a2/10^(g/k); derived f = a2/a1 (number, label A_2/A_1 →
   use label "F"); formula ["g = k * log10(a2 / a1)"].
7. `db_levels` — v (voltage, V, "0.7746 V"), r (resistance, R, "600"), p (power, P); dof 2; rules p: v^2/r,
   v: sqrt(p*r), r: v^2/p; derived p_dbm (power, expr p, unit dBm), v_dbv (voltage, expr v, unit dBV),
   v_dbu (voltage, expr v, unit dBu); formula ["p = v^2 / r"].
8. ⊞ `wheatstone` — vs "5 V", r1 "1k", r2 "1k", r3 "1k", r4 "1k1", vd (label V_D); dof 5; left divider r1 top/r2
   bottom, right divider r3 top/r4 bottom; vd = vs*(r2/(r1+r2) - r4/(r3+r4)); inverse rules for vs, r1..r4 written out
   explicitly (A derives them, e.g. r4 = x*r3/(1-x) with x = r2/(r1+r2) - vd/vs written inline);
   derived r4bal = r2*r3/r1 (label R_4,bal → "R_4b").
9. ⊞ `star_delta` — rab "30", rbc "60", rca "90" (labels R_AB …), ra, rb, rc; dof 3; ra: rab*rca/(rab+rbc+rca),
   rb: rab*rbc/(…), rc: rbc*rca/(…), rab: (ra*rb+rb*rc+rc*ra)/rc, rbc: (…)/ra, rca: (…)/rb.
   Solvable sets must come out as exactly the delta set and the star set.
10. `rms_waveform` — choice wave {sine: cf sqrt(2) = 1.4142135623730951, ff = 1.1107207345395915;
    square: cf 1, ff 1; triangle: cf 1.7320508075688772, ff 1.1547005383792515}; vrms "230 V", vpk, vpp, vavg;
    dof 1; vrms:[vpk/cf, vpp/(2*cf), vavg*ff] vpk:[vrms*cf, vpp/2, vavg*ff*cf] vpp:[2*vpk, 2*vrms*cf, 2*vavg*ff*cf]
    vavg:[vrms/ff, vpk/(cf*ff), vpp/(2*cf*ff)]; formula ["vpk = vrms * cf", "vrms = vavg * ff"].
11. `charge_energy` — q (charge, Q, "2 Ah", unit Ah), v (voltage, U, "3.7 V"), e (energy, E, unit Wh); dof 2;
    e: q*v, q: e/v, v: e/q; formula ["e = q * v"].

### 10. UI contract (C), i18n keys (B writes, C uses)

- Start menu category `dev`. App id `ecalc`, `Entry: builtin://ecalc`, window default 1100×760, min 640×480,
  resizable. Compact layout (single stack with back button) below 720 px content width.
- Catalog fetched once per open window via the desktop's existing API helper conventions from
  `/api/ecalc/catalog?lang=<desktop lang>`; failure shows `ecalc_load_failed` + retry button.
- localStorage key `aurago.ecalc.v1`: {last, favorites[], recents[] (max 12), units{calc.var: unitId}, sig (3..6,
  default 4), eseries ("E24"), symbols ("iec"|"ansi"), values{calcId: {inputs:{var: raw string}, recency:[…],
  locked:[…], choices:{…}}}}.
- Calculator page = layout C (spec). Field states: input (user value), computed (success tint), error (danger tint +
  message), locked (lock icon pressed). Typing parses on input (debounce ≤ 80 ms); parse error shows `ecalc_error_parse`
  and does not change inputs.
- i18n keys (prefix `desktop.`), all 16 locales:
  app_ecalc ("ECalc"), ecalc_search_placeholder, ecalc_favorites, ecalc_recent, ecalc_empty_favorites,
  ecalc_empty_recent, ecalc_no_results, ecalc_favorite_add, ecalc_favorite_remove, ecalc_copy_results, ecalc_copied,
  ecalc_reset, ecalc_explanation, ecalc_source, ecalc_derived, ecalc_eseries_nearest ("Nearest {series} value"),
  ecalc_eseries_series, ecalc_lock, ecalc_unlock, ecalc_unit_auto, ecalc_error_undetermined, ecalc_error_nonfinite,
  ecalc_error_range, ecalc_error_parse, ecalc_settings, ecalc_settings_digits, ecalc_settings_symbols,
  ecalc_symbols_iec, ecalc_symbols_ansi, ecalc_back, ecalc_load_failed, ecalc_retry, ecalc_warnings, ecalc_inputs,
  ecalc_computed, ecalc_dim_voltage, ecalc_dim_current, ecalc_dim_resistance, ecalc_dim_power, ecalc_dim_charge,
  ecalc_dim_energy, ecalc_dim_number, ecalc_dim_gain.
  Placeholders use `{name}` syntax like the existing desktop keys (check en.json for the house style).
