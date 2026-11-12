[← Plan index](README.md) · Spec: [plans/2026-11-12-easydrag-s0c-spec.md](../2026-11-12-easydrag-s0c-spec.md)

## Part C: Living documentation (FD-16)

Spec: `plans/2026-11-12-easydrag-s0c-spec.md` §5 (design), §7 rows 12 and 14, §8, §10. Tasks run in the order C.1 to C.5. C.1 to C.4 start after Part A has landed on main. C.5 starts after Part B has landed on main.

**Shell.** Git Bash. Every command starts with `cd <worktree> && ...` because the working directory resets between calls. Set `MSYS_NO_PATHCONV=1` for any argument that starts with `//`. `WT` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs`.

**Worktree.** Create it only after the coordinator says Part A landed on main.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0c-docs -b feat/easydrag-s0c-docs main
mkdir -p C:/disposable/easydrag-s0c D:/tmp/go-build
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git log -1 --format=%h && git status --short
```

Expected: the worktree is created from `main`; `git status --short` prints nothing.

**Baseline.** Run it once before C.1. A failure that is already on main goes into the report to the coordinator and is not fixed here.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... -count=1 -timeout 15m 2>&1 | tail -20
```

Expected: `ok  	aurago/internal/flows` (and one `ok` or `no test files` line per sub-package). Copy any `FAIL` line into the report.

**What this part depends on.**
- Part A (landed): the manual text in C.3 describes start failures as failed runs, the stop hint, the file.write card word, the schedule help and the `card_label` option. C.3's first step reads the real locale words and says what to do when they differ from the words used here.
- Part B (landed before C.5 only): `scripts/test-easydrag-t0.mjs` and its `GO_STEPS` array.

**Conventions of this part.**
- Go temp dir: `GOTMPDIR='D:\tmp\go-build'` for `cmd/...` and `internal/flows`. `internal/server` is not touched here.
- No JS, CSS or locale file changes in this part, so the `node scripts/test-easydrag*.mjs` gates do not apply. `ui/lang/easydrag/*.json` are read, never written (Part A owns them).
- Gates before every Go commit: `go vet` on the touched packages and `gofmt -l` on the touched files (it prints nothing).
- Commit: one per task. `git add <exact paths>`, then `git commit -F C:/disposable/easydrag-s0c/<message file>`. Write each message file with the Write tool. Every message ends with the GitNexus line, a blank line and the Co-Authored-By line; the implementer fills in the model line.
- GitNexus cannot index a worktree on this machine. IMPACT runs from the main checkout path; HIGH or CRITICAL is reported, UNKNOWN is confirmed with `grep -rn`.
- Escapes: the Write and Edit tools turn backslash-u sequences into characters. This part has none; keep it that way.
- Line ends: all three documents are LF today (`git ls-files --eol` shows `i/lf`), and `.gitattributes` forces LF. The generator refuses a file with CR.

**Facts the tasks rely on (read from main at b52103d51).**
- `flows.RegisterCatalog(reg *Registry, env CatalogEnv) error` (`internal/flows/catalog.go:17`). `CatalogEnv` is `interface{ ToolAvailability(tool string) Availability }` (`catalog_tools.go:12`). `StaticEnv` lists tools and reports unlisted ones as needing setup, so the generator uses its own `docEnv` where every tool is available. Generic `tool.*` types are never registered by `RegisterCatalog` (they exist only in a live registry), so the reference lists the 42 curated types: 14 triggers and 28 steps.
- `flows.DescribeNodeTypes(reg, tr)` (`catalog.go:154`) returns `[]NodeTypeInfo` with `Type`, `Category`, `Label`, `Description`, `Trigger`, `Effects []Effect` and `Simulatable`. It translates through the `tr` function and falls back to the key itself when `tr` returns "". The generator treats a fallback as an error.
- Labels and descriptions: `easydrag.node.<type with dots as underscores>.label` and `.description` (44 of each in `en.json` and `de.json`). Categories: `easydrag.category.<category>` (`trigger`, `logic`, `ai`, `web`, `documents`, `notify`, `smart_home`, `planner`). Effects: `easydrag.ui.effect_<effect>` for the six `Effect` constants in `node.go:20-27` (`sends_message`, `writes_files`, `controls_devices`, `runs_code`, `deletes`, `system_change`). The locale files are flat string maps and are not sorted.
- `flows.CategoryOrder` (`catalog.go:14`) is the palette order. The generated tables use it, then the localized label, then the type id.

---

### Task C.1: One function for the Simulation rule

**Where the rule lives today.** Two places decide it, and neither is a function the generator can call.
1. `Service.Simulate` (`internal/flows/simulation.go:87`) decides per step whether it runs or is replaced. Line 156 is the rule, inline inside a loop: `pure := simulationPureType(def.Type) || def.Type == blockCallType || def.Type == blockEachType`. A pure step runs its own `Execute`; every other non-trigger step returns `FLOW_SIMULATION_FIXTURE_REQUIRED` unless the request has a fixture for it. Triggers are skipped by the loop (they supply the trigger data).
2. `simulatableType` (`simulation.go:253`, unexported, cached) says whether `Simulate` accepts a draft that contains the type: it must exist in the simulation registry (no `tool.*`) and must not be durable. The catalog sends it as `NodeTypeInfo.Simulatable` and the test dialog reads it in `canSimulate` (`easydrag-testing.js:102`).

The test dialog's default (`easydrag-testing.js:112-115`, `simulationDefault`) is a different question: whether the dialog opens in Simulation. It stays in JS and is documented by hand in C.3.

**The extraction.** One exported function, `SimulationBehaviorOf(typ string, trigger bool) SimulationBehavior`, in `simulation.go`. It has four outcomes: runs locally, needs a sample output, cannot be simulated, supplies the test data (a trigger). `Simulate` calls it instead of the inline expression, and the generator calls it for the "In Simulation" column. Behaviour of `Simulate` does not change: `pure` stays true for exactly `logic.if`, `logic.switch`, `logic.merge`, `logic.set`, `logic.stop`, `component.call`, `component.each`. For the durable types `pure` was false and stays false (`Simulate` refuses a draft with a durable step before the loop).

**Covering tests today.** `TestSimulationIsIsolated` (`simulation_test.go:139`) pins that `http.request` needs a sample (`FLOW_SIMULATION_FIXTURE_REQUIRED`) and that `logic.set` runs without one. `TestDescribeNodeTypesSimulatable` (`catalog_test.go:102`) pins `Simulatable`. Neither pins the other five pure types, `logic.wait`, the block types or the trigger outcome, and the new function is public API that the generator and the manual now depend on. The task therefore adds one small table test, `TestSimulationBehaviorOf`, and re-runs both covering tests.

**Files:**
- Modify: `WT/internal/flows/simulation.go:70-76` (add the type and the function next to `simulationPureType`), `:156` (call it)
- Test (create): `WT/internal/flows/simulation_behavior_test.go`

- [ ] **Step 1: IMPACT on the function that changes and the one it wraps.**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Simulate" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "simulationPureType" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Expected: `Simulate` is called by the server's simulate handler (`internal/server/flows_handlers_runs.go:130`) and by tests; `simulationPureType` only by `Simulate` and tests. Report the risk the tool prints. HIGH or CRITICAL on `Simulate` is expected to be a function of its callers' count, not of this change (one line changes, with the same result); say so in the report. If a result is UNKNOWN, confirm with `grep -rn "simulationPureType\|\.Simulate(" internal --include=*.go`: there must be exactly one non-test use of `simulationPureType` (line 156 and its own definition) and one server caller of `Simulate`.

- [ ] **Step 2: Confirm the helpers the test uses.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && grep -n "^func catalogRegistry\|^func fullEnv" internal/flows/*_test.go && grep -n "^package" internal/flows/simulation_test.go
```

Expected: one line each for `catalogRegistry` and `fullEnv` (both in `catalog_test.go` or a sibling), and `package flows`. If either helper is gone, build the registry in the test with `reg := NewRegistry(); if err := RegisterCatalog(reg, nil); err != nil { t.Fatal(err) }` and drop `fullEnv()`.

- [ ] **Step 3: Write the failing test.** Create `WT/internal/flows/simulation_behavior_test.go`:

```go
package flows

import "testing"

// SimulationBehaviorOf is the one rule for what Simulation does with a step. The manuals
// are generated from it, so the whole table is pinned, not only the types that
// TestSimulationIsIsolated happens to use.
func TestSimulationBehaviorOf(t *testing.T) {
	cases := []struct {
		typ     string
		trigger bool
		want    SimulationBehavior
	}{
		{TypeIf, false, SimulationRunsLocally},
		{TypeSwitch, false, SimulationRunsLocally},
		{TypeMerge, false, SimulationRunsLocally},
		{TypeSet, false, SimulationRunsLocally},
		{TypeStop, false, SimulationRunsLocally},
		{blockCallType, false, SimulationRunsLocally},
		{blockEachType, false, SimulationRunsLocally},
		{TypeWait, false, SimulationNeedsSample},
		{TypeHTTPRequest, false, SimulationNeedsSample},
		{TypeAIStep, false, SimulationNeedsSample},
		{TypeTelegram, false, SimulationNeedsSample},
		{TypeFileWrite, false, SimulationNeedsSample},
		{durableWaitType, false, SimulationUnsupported},
		{durableApprovalType, false, SimulationUnsupported},
		{GenericTypePrefix + "docker", false, SimulationUnsupported},
		{TypeTriggerManual, true, SimulationSuppliesData},
		{TypeTriggerSchedule, true, SimulationSuppliesData},
	}
	for _, c := range cases {
		if got := SimulationBehaviorOf(c.typ, c.trigger); got != c.want {
			t.Errorf("%s (trigger %v): got %q, want %q", c.typ, c.trigger, got, c.want)
		}
	}

	// The rule and the catalog flag agree on what Simulate accepts.
	for _, info := range DescribeNodeTypes(catalogRegistry(t, fullEnv()), nil) {
		if info.Trigger {
			continue
		}
		if unsupported := SimulationBehaviorOf(info.Type, false) == SimulationUnsupported; unsupported == info.Simulatable {
			t.Errorf("%s: behaviour unsupported=%v but the catalog says simulatable=%v", info.Type, unsupported, info.Simulatable)
		}
	}
}
```

- [ ] **Step 4: Run it and see it fail for the stated reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run TestSimulationBehaviorOf -count=1 2>&1 | head -12
```

Expected: a build failure, `undefined: SimulationBehaviorOf`, `undefined: SimulationRunsLocally` and the other three constants.

- [ ] **Step 5: Implement.** In `WT/internal/flows/simulation.go`, replace the function `simulationPureType` (lines 70-76) with itself plus the new type and function. Old text:

```go
func simulationPureType(typ string) bool {
	switch typ {
	case TypeIf, TypeSwitch, TypeMerge, TypeSet, TypeStop:
		return true
	}
	return false
}
```

New text:

```go
func simulationPureType(typ string) bool {
	switch typ {
	case TypeIf, TypeSwitch, TypeMerge, TypeSet, TypeStop:
		return true
	}
	return false
}

// SimulationBehavior says what Simulation does with one step.
type SimulationBehavior string

const (
	// SimulationRunsLocally: pure logic. The step runs as in a real run, without side effects.
	SimulationRunsLocally SimulationBehavior = "runs_locally"
	// SimulationNeedsSample: the step is replaced by the sample output of the test dialog.
	// Without one the simulation stops with FLOW_SIMULATION_FIXTURE_REQUIRED.
	SimulationNeedsSample SimulationBehavior = "needs_sample"
	// SimulationUnsupported: Simulate refuses a draft that contains the step (durable steps,
	// generic tool steps).
	SimulationUnsupported SimulationBehavior = "unsupported"
	// SimulationSuppliesData: a trigger. It starts the run with the test data.
	SimulationSuppliesData SimulationBehavior = "supplies_data"
)

// SimulationBehaviorOf is the one rule for what Simulation does with a step of this type.
// Service.Simulate runs a step for real only when it answers SimulationRunsLocally, and
// cmd/flowsdoc prints the same answer in the manuals. trigger is NodeDef.Trigger.
func SimulationBehaviorOf(typ string, trigger bool) SimulationBehavior {
	switch {
	case trigger:
		return SimulationSuppliesData
	case !simulatableType(typ):
		return SimulationUnsupported
	case simulationPureType(typ) || typ == blockCallType || typ == blockEachType:
		return SimulationRunsLocally
	}
	return SimulationNeedsSample
}
```

Then at line 156 (now a few lines lower), replace

```go
		pure := simulationPureType(def.Type) || def.Type == blockCallType || def.Type == blockEachType
```

with

```go
		pure := SimulationBehaviorOf(def.Type, false) == SimulationRunsLocally
```

- [ ] **Step 6: Run the new test and both covering tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run '(?i)(TestSimulationBehaviorOf|TestSimulation|TestDescribeNodeTypesSimulatable)' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)" 
```

Expected: `--- PASS` lines for `TestSimulationBehaviorOf`, `TestSimulationFixturesAndTemplate`, `TestSimulationFileFixtures`, `TestSimulationIsIsolated`, `TestSimulationRejectsUnknownNodesBeforeExecution`, `TestDescribeNodeTypesSimulatable`, then `ok  	aurago/internal/flows`. Count them: six PASS lines is the evidence that behaviour did not change.

- [ ] **Step 7: Gates and the wider package.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && gofmt -l internal/flows/simulation.go internal/flows/simulation_behavior_test.go && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -count=1 2>&1 | tail -3
```

Expected: `go vet` prints nothing, `gofmt -l` prints nothing, `ok  	aurago/internal/flows`.

- [ ] **Step 8: Commit.** Write `C:/disposable/easydrag-s0c/c1-msg.txt` with the Write tool:

```
refactor(flows): one exported rule for what Simulation does with a step

Service.Simulate decided inline whether a step runs or needs a sample output, and
simulatableType decided whether the draft is accepted at all. SimulationBehaviorOf
now answers both (runs locally, needs a sample, unsupported, supplies the test data
for a trigger). Simulate calls it; the manuals generator will print the same answer.
Behaviour is unchanged: the same seven types run locally. TestSimulationBehaviorOf
pins the whole table; TestSimulationIsIsolated and
TestDescribeNodeTypesSimulatable still pass.

IMPACT: Simulate (one server caller, tests), simulationPureType (Simulate only).
GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git add internal/flows/simulation.go internal/flows/simulation_behavior_test.go && git commit -F C:/disposable/easydrag-s0c/c1-msg.txt && git status --short
```

Expected: one commit; `git status --short` prints nothing.

---

### Task C.2: The generator, the drift test and the marker blocks

**Design.**
- `cmd/flowsdoc/render.go` holds everything testable: the target list, the locale reader, the catalog description, the table and list renderers and the marker replacement. `cmd/flowsdoc/main.go` finds the repository root (the first parent directory with a `go.mod`, so `go run` and `go test` both work), runs `apply` for each target and writes a file only when its content changed.
- One code path serves the generator and the test: `source.apply(target, doc)` returns the document with every block rewritten, or an error when a marker pair is missing. The test compares `apply`'s result with the file, so the generator and the test cannot disagree.
- The tables follow spec §5.1: name, category, what it does, effects, in Simulation. Rows sort by palette category, localized label, type id. Effects are the words of `easydrag.ui.effect_*`; "None" / "Keine" when a block names none (the column header says "default settings", because `http.request` and `home.assistant` change with their parameters; the chapter says so in the intro). The Simulation words are fixed in the generator, per language (below), keyed by `SimulationBehavior` from C.1.
- The agent manual gets a list instead of a table, English only, with the type id first: ``- `trigger.schedule` (Schedule): Starts at regular times, ...``.
- A missing label, description, category word or effect word fails the generator with the file, the key and the locale. Part A owns the locale files; a failure after Part A has landed means a type has no text and goes into the report.

**Words (generator constants).**

| Behaviour | en | de |
|---|---|---|
| `SimulationRunsLocally` | Runs locally | Läuft lokal |
| `SimulationNeedsSample` | Replaced by your sample output | Wird durch deine Beispielausgabe ersetzt |
| `SimulationUnsupported` | Cannot be simulated | Lässt sich nicht simulieren |
| `SimulationSuppliesData` | Supplies the test data | Liefert die Testdaten |
| no effects | None | Keine |

Headers en: `Name | Category | What it does | Effects (default settings) | In Simulation`. Headers de: `Name | Kategorie | Was er tut | Wirkungen (Standardeinstellungen) | In der Simulation`.

**Files:**
- Create: `WT/cmd/flowsdoc/main.go`, `WT/cmd/flowsdoc/render.go`
- Test (create): `WT/cmd/flowsdoc/flowsdoc_test.go`
- Modify: `WT/documentation/manuals/en/24-easydrag.md` (TOC lines 9-26 area, new section before the closing `---` at line ~548), `WT/documentation/manuals/de/24-easydrag.md` (same), `WT/prompts/tools_manuals/manage_missions.md` (new section before `## Examples`, line 25)

- [ ] **Step 1: Confirm the facts the generator reads.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && node -e "
for (const l of ['en','de']) { const d = require('./ui/lang/easydrag/' + l + '.json');
  const need = ['trigger','logic','ai','web','documents','notify','smart_home','planner'].map(c => 'easydrag.category.' + c)
    .concat(['sends_message','writes_files','controls_devices','runs_code','deletes','system_change'].map(e => 'easydrag.ui.effect_' + e));
  console.log(l, 'missing:', need.filter(k => typeof d[k] !== 'string' || !d[k]).join(',') || 'none'); }"
ls cmd/flowsdoc 2>&1 | head -2; head -3 go.mod
```

Expected: `en missing: none`, `de missing: none`, `ls: cannot access 'cmd/flowsdoc'` and `go 1.27.3` on the third line of the module header. If a key is missing, Part A's locale work removed or renamed it: stop and report (do not add keys here).

- [ ] **Step 2: Write the failing test.** Create `WT/cmd/flowsdoc/flowsdoc_test.go`:

```go
package main

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

const regenerate = "run `go run ./cmd/flowsdoc` from the repository root and commit the result"

// The manuals' reference blocks are generated from the node catalog. This test renders
// every block in memory and compares it with the file, so a changed label, description or
// effect (or a deleted marker) cannot reach main without the regenerated documents.
func TestFlowsDocBlocksMatchTheCatalog(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	src, err := loadSource(root)
	if err != nil {
		t.Fatal(err)
	}
	read := func(tg target) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tg.path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	t.Run("every generated block is current", func(t *testing.T) {
		for _, tg := range targets {
			doc := read(tg)
			next, err := src.apply(tg, doc)
			if err != nil {
				t.Errorf("%v; restore the marker pair, then %s", err, regenerate)
				continue
			}
			if next != doc {
				t.Errorf("%s: the generated blocks differ from the catalog; %s", tg.path, regenerate)
			}
		}
	})

	t.Run("a document without a marker pair fails", func(t *testing.T) {
		for name, doc := range map[string]string{
			"no markers":            "# Chapter\n\nText.\n",
			"a begin without an end": "<!-- BEGIN GENERATED: flows-triggers -->\n<!-- BEGIN GENERATED: flows-steps -->\n<!-- END GENERATED: flows-steps -->\n",
		} {
			if _, err := src.apply(targets[0], doc); err == nil {
				t.Errorf("%s: apply accepted the document", name)
			}
		}
	})

	t.Run("a changed catalog label is detected", func(t *testing.T) {
		tg := targets[0]
		current := read(tg)
		locales := maps.Clone(src.locales)
		en := maps.Clone(src.locales["en"])
		en["easydrag.node.trigger_manual.label"] += " (renamed)"
		locales["en"] = en
		next, err := (&source{reg: src.reg, locales: locales}).apply(tg, current)
		if err != nil {
			t.Fatal(err)
		}
		if next == current {
			t.Fatal("renaming a label did not change the generated block, so drift would go unnoticed")
		}
	})
}
```

- [ ] **Step 3: Run it and see it fail for the stated reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 2>&1 | head -10
```

Expected: a build failure, `undefined: repoRoot`, `undefined: loadSource`, `undefined: target`, `undefined: targets`, `undefined: source`.

- [ ] **Step 4: Write `WT/cmd/flowsdoc/main.go`.**

```go
// Command flowsdoc rewrites the generated blocks of the EasyDrag manuals and of the
// manage_missions tool manual from the node catalog.
//
// Run it from anywhere inside the repository after a label, a description or an effect of
// a building block changed:
//
//	go run ./cmd/flowsdoc
//
// It rewrites only the text between "<!-- BEGIN GENERATED: name -->" and
// "<!-- END GENERATED: name -->". Its output has no dates, a fixed order and LF line ends.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "flowsdoc:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	src, err := loadSource(root)
	if err != nil {
		return err
	}
	for _, tg := range targets {
		path := filepath.Join(root, filepath.FromSlash(tg.path))
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next, err := src.apply(tg, string(old))
		if err != nil {
			return err
		}
		if next == string(old) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(next), info.Mode().Perm()); err != nil {
			return err
		}
		fmt.Println("updated", tg.path)
	}
	return nil
}

// repoRoot is the first directory, from the working directory upwards, that holds go.mod.
// go run starts in the module root, go test in the package directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}
```

- [ ] **Step 5: Write `WT/cmd/flowsdoc/render.go`.**

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/flows"
)

const (
	blockTriggers = "flows-triggers"
	blockSteps    = "flows-steps"
)

// target is one document with the generated blocks it carries.
type target struct {
	path   string // slash-separated, relative to the repository root
	lang   string // language of the labels and of the generator's own words
	agent  bool   // the agent manual: a list with type ids instead of tables
	blocks []string
}

// targets[0] is the English chapter; the test relies on it.
var targets = []target{
	{path: "documentation/manuals/en/24-easydrag.md", lang: "en", blocks: []string{blockTriggers, blockSteps}},
	{path: "documentation/manuals/de/24-easydrag.md", lang: "de", blocks: []string{blockTriggers, blockSteps}},
	{path: "prompts/tools_manuals/manage_missions.md", lang: "en", agent: true, blocks: []string{blockTriggers}},
}

type words struct {
	headers [5]string
	none    string
	sim     map[flows.SimulationBehavior]string
}

var wordsByLang = map[string]words{
	"en": {
		headers: [5]string{"Name", "Category", "What it does", "Effects (default settings)", "In Simulation"},
		none:    "None",
		sim: map[flows.SimulationBehavior]string{
			flows.SimulationRunsLocally:   "Runs locally",
			flows.SimulationNeedsSample:   "Replaced by your sample output",
			flows.SimulationUnsupported:   "Cannot be simulated",
			flows.SimulationSuppliesData:  "Supplies the test data",
		},
	},
	"de": {
		headers: [5]string{"Name", "Kategorie", "Was er tut", "Wirkungen (Standardeinstellungen)", "In der Simulation"},
		none:    "Keine",
		sim: map[flows.SimulationBehavior]string{
			flows.SimulationRunsLocally:   "Läuft lokal",
			flows.SimulationNeedsSample:   "Wird durch deine Beispielausgabe ersetzt",
			flows.SimulationUnsupported:   "Lässt sich nicht simulieren",
			flows.SimulationSuppliesData:  "Liefert die Testdaten",
		},
	},
}

// docEnv is the catalog environment of the manuals: every tool is available, so no node
// type is hidden behind a missing setup.
type docEnv struct{}

func (docEnv) ToolAvailability(string) flows.Availability {
	return flows.Availability{State: flows.AvailableState}
}

// source is the catalog and the locale texts the blocks are rendered from.
type source struct {
	reg     *flows.Registry
	locales map[string]map[string]string
}

func loadSource(root string) (*source, error) {
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, docEnv{}); err != nil {
		return nil, fmt.Errorf("register the catalog: %w", err)
	}
	src := &source{reg: reg, locales: map[string]map[string]string{}}
	for lang := range wordsByLang {
		raw, err := os.ReadFile(filepath.Join(root, "ui", "lang", "easydrag", lang+".json"))
		if err != nil {
			return nil, err
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			return nil, fmt.Errorf("ui/lang/easydrag/%s.json: %w", lang, err)
		}
		src.locales[lang] = texts
	}
	return src, nil
}

// apply returns doc with every block of tg rewritten. A missing marker pair is an error.
func (s *source) apply(tg target, doc string) (string, error) {
	if strings.Contains(doc, "\r") {
		return "", fmt.Errorf("%s has CR line ends; the generator writes LF only", tg.path)
	}
	for _, name := range tg.blocks {
		body, err := s.renderBlock(tg, name)
		if err != nil {
			return "", fmt.Errorf("%s: %w", tg.path, err)
		}
		if doc, err = replaceBlock(doc, name, body); err != nil {
			return "", fmt.Errorf("%s: %w", tg.path, err)
		}
	}
	return doc, nil
}

// replaceBlock replaces the text between the two marker lines of name. body ends with a
// line feed, so the end marker stays on its own line.
func replaceBlock(doc, name, body string) (string, error) {
	begin := "<!-- BEGIN GENERATED: " + name + " -->"
	end := "<!-- END GENERATED: " + name + " -->"
	b, e := strings.Index(doc, begin), strings.Index(doc, end)
	if b < 0 || e < 0 || e < b || strings.Count(doc, begin) != 1 || strings.Count(doc, end) != 1 {
		return "", fmt.Errorf("block %q needs exactly one line %s followed by one line %s", name, begin, end)
	}
	return doc[:b+len(begin)] + "\n" + body + doc[e:], nil
}

func (s *source) renderBlock(tg target, name string) (string, error) {
	switch {
	case tg.agent && name == blockTriggers:
		return s.renderAgentList(tg.lang)
	case name == blockTriggers || name == blockSteps:
		return s.renderTable(tg.lang, name == blockTriggers)
	}
	return "", fmt.Errorf("unknown block %q", name)
}

// describe returns the catalog in the language, sorted by palette category, label and type.
// A type without a label or a description is an error: the manuals never show a key.
func (s *source) describe(lang string) ([]flows.NodeTypeInfo, error) {
	texts := s.locales[lang]
	infos := flows.DescribeNodeTypes(s.reg, func(key string) string { return texts[key] })
	for _, info := range infos {
		if info.Label == info.Type || info.Description == "" ||
			strings.HasPrefix(info.Label, "easydrag.") || strings.HasPrefix(info.Description, "easydrag.") {
			return nil, fmt.Errorf("%s has no label or no description in ui/lang/easydrag/%s.json", info.Type, lang)
		}
	}
	sort.SliceStable(infos, func(i, j int) bool {
		a, b := infos[i], infos[j]
		if ra, rb := categoryRank(a.Category), categoryRank(b.Category); ra != rb {
			return ra < rb
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if la, lb := strings.ToLower(a.Label), strings.ToLower(b.Label); la != lb {
			return la < lb
		}
		return a.Type < b.Type
	})
	return infos, nil
}

func categoryRank(category string) int {
	for i, c := range flows.CategoryOrder {
		if c == category {
			return i
		}
	}
	return len(flows.CategoryOrder)
}

func (s *source) renderTable(lang string, triggers bool) (string, error) {
	infos, err := s.describe(lang)
	if err != nil {
		return "", err
	}
	texts, w := s.locales[lang], wordsByLang[lang]
	var b strings.Builder
	b.WriteString("| " + strings.Join(w.headers[:], " | ") + " |\n|---|---|---|---|---|\n")
	for _, info := range infos {
		if info.Trigger != triggers {
			continue
		}
		category, err := word(texts, lang, "easydrag.category."+info.Category)
		if err != nil {
			return "", err
		}
		effects := []string{}
		for _, e := range info.Effects {
			text, err := word(texts, lang, "easydrag.ui.effect_"+string(e))
			if err != nil {
				return "", err
			}
			effects = append(effects, text)
		}
		effectText := w.none
		if len(effects) > 0 {
			effectText = strings.Join(effects, ", ")
		}
		simText := w.sim[flows.SimulationBehaviorOf(info.Type, info.Trigger)]
		b.WriteString("| " + strings.Join([]string{cell(info.Label), cell(category), cell(info.Description), cell(effectText), cell(simText)}, " | ") + " |\n")
	}
	return b.String(), nil
}

// renderAgentList is the trigger list of the agent manual: the type id first, because the
// agent names types.
func (s *source) renderAgentList(lang string) (string, error) {
	infos, err := s.describe(lang)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, info := range infos {
		if info.Trigger {
			fmt.Fprintf(&b, "- `%s` (%s): %s\n", info.Type, oneLine(info.Label), oneLine(info.Description))
		}
	}
	return b.String(), nil
}

func word(texts map[string]string, lang, key string) (string, error) {
	if texts[key] == "" {
		return "", fmt.Errorf("ui/lang/easydrag/%s.json has no key %s", lang, key)
	}
	return texts[key], nil
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// cell is oneLine for a table cell: a pipe would end it.
func cell(s string) string { return strings.ReplaceAll(oneLine(s), "|", "\\|") }
```

- [ ] **Step 6: Format and run the test; see the right failure.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && gofmt -w cmd/flowsdoc/*.go && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 -v 2>&1 | grep -E "^(\s*--- |ok|FAIL)|needs exactly"
```

Expected: `--- FAIL: TestFlowsDocBlocksMatchTheCatalog`, a failing `every_generated_block_is_current` with three lines `block "flows-triggers" needs exactly one line <!-- BEGIN GENERATED: flows-triggers --> followed by one line <!-- END ...` (one per file) and the advice `run `go run ./cmd/flowsdoc``; `a_document_without_a_marker_pair_fails` passes; `a_changed_catalog_label_is_detected` fails with the same marker error. This is the red state: the documents have no markers yet. If the failure is instead `has no label or no description` or `has no key`, a catalog text is missing: report it and stop.

- [ ] **Step 7: Add the marker blocks to the English chapter.** In `WT/documentation/manuals/en/24-easydrag.md`:

1. TOC. After the line `   - [Visual groups](#visual-groups)` add nothing here (C.3 adds the schedule entry). After the line `15. [Reusable Local Blocks](#reusable-local-blocks)` add:

```
16. [Reference: triggers and steps](#reference-triggers-and-steps)
```

2. The section. Insert it directly after the keyboard shortcut table (the line `| Esc | Close or clear the selection; in a run view, back to the draft |`) and before the closing `---` line that precedes `**See also:**`. Leave one blank line before and after it:

```
## Reference: triggers and steps

<!-- The two tables below are generated from the building-block catalog. Run `go run ./cmd/flowsdoc` to update them; a test fails when they no longer match. -->

Names, descriptions and effects come from the building-block catalog. *Effects* are what a block does with its default settings. What a flow really does is decided when you publish, from the actual settings of each step (see [Publishing](#publishing)); an HTTP request with POST, for example, counts as sending. Generic tool steps ("any permitted tool") depend on the tool and are not listed. *In Simulation* says what the Simulation of the test dialog does with the block (see [Testing](#testing)).

### Triggers

<!-- BEGIN GENERATED: flows-triggers -->
<!-- END GENERATED: flows-triggers -->

### Steps

<!-- BEGIN GENERATED: flows-steps -->
<!-- END GENERATED: flows-steps -->
```

- [ ] **Step 8: Add the marker blocks to the German chapter.** In `WT/documentation/manuals/de/24-easydrag.md`:

1. TOC. After the line `15. [Wiederverwendbare lokale Bausteine](#wiederverwendbare-lokale-bausteine)` add:

```
16. [Referenz: Auslöser und Schritte](#referenz-auslöser-und-schritte)
```

2. The section, after the last row of the Tastenkürzel table and before the closing `---` that precedes `**Siehe auch:**`:

```
## Referenz: Auslöser und Schritte

<!-- Die beiden Tabellen unten werden aus dem Baustein-Katalog erzeugt. `go run ./cmd/flowsdoc` aktualisiert sie; ein Test schlägt fehl, wenn sie nicht mehr zum Katalog passen. -->

Namen, Beschreibungen und Wirkungen stammen aus dem Baustein-Katalog. *Wirkungen* nennt, was ein Baustein mit seinen Standardeinstellungen tut. Was ein Flow wirklich tut, entscheidet sich beim Veröffentlichen aus den tatsächlichen Einstellungen jedes Schritts (siehe [Veröffentlichen](#veröffentlichen)); eine HTTP-Anfrage mit POST zählt zum Beispiel als Senden. Generische Werkzeug-Schritte („jedes freigegebene Werkzeug“) hängen vom Werkzeug ab und stehen nicht in der Liste. *In der Simulation* sagt, was die Simulation des Testdialogs mit dem Baustein macht (siehe [Testen](#testen)).

### Auslöser

<!-- BEGIN GENERATED: flows-triggers -->
<!-- END GENERATED: flows-triggers -->

### Schritte

<!-- BEGIN GENERATED: flows-steps -->
<!-- END GENERATED: flows-steps -->
```

- [ ] **Step 9: Add the marker block to the agent manual.** In `WT/prompts/tools_manuals/manage_missions.md`, insert before the line `## Examples` (line 25; lines 22-23 stay as they are):

```
## Flow trigger types

A flow starts from its trigger nodes. These are the types (type id, name, what starts the flow). The triggers of a flow are set in EasyDrag; this tool cannot add or change them.

<!-- BEGIN GENERATED: flows-triggers -->
<!-- END GENERATED: flows-triggers -->

```

(The block keeps one blank line before `## Examples`.) Line 20 (the `triggered` mission events) and line 21 (the flow paragraph, with `read_only`) are not edited: see "Deviations", item 1.

- [ ] **Step 10: Generate, then check that a second run changes nothing.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go run ./cmd/flowsdoc && echo "--- second run" && GOTMPDIR='D:\tmp\go-build' go run ./cmd/flowsdoc && echo "(no output above means no change)" && git status --short
```

Expected: three `updated ...` lines on the first run (`documentation/manuals/en/24-easydrag.md`, `documentation/manuals/de/24-easydrag.md`, `prompts/tools_manuals/manage_missions.md`), nothing after `--- second run`, and `git status --short` listing the three documents plus `?? cmd/flowsdoc/`.

- [ ] **Step 11: Look at the output and count the rows.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && for f in documentation/manuals/en/24-easydrag.md documentation/manuals/de/24-easydrag.md; do for b in flows-triggers flows-steps; do printf "%s %s rows: " "$f" "$b"; sed -n "/BEGIN GENERATED: $b/,/END GENERATED: $b/p" "$f" | grep -c '^| '; done; done; sed -n '/BEGIN GENERATED: flows-triggers/,/END GENERATED: flows-triggers/p' prompts/tools_manuals/manage_missions.md | cut -c1-110; sed -n '/BEGIN GENERATED: flows-steps/,/END GENERATED: flows-steps/p' documentation/manuals/en/24-easydrag.md | sed -n 1,8p | cut -c1-200; git diff --check
```

Expected: 15 rows per trigger table (header plus 14) and 29 per steps table (header plus 28) on today's catalog; the agent list shows 14 lines in the form ``- `trigger.manual` (Manual start): Starts with “Run now” and with test runs.`` and, sorted by palette order then label, the first table row is a logic step; `git diff --check` prints nothing. Read the English steps table once: `logic.stop`, `logic.if` and `logic.set` say "Runs locally", `notify.telegram` says "Sends messages" and "Replaced by your sample output", `logic.durable_wait` says "Cannot be simulated". If a row looks wrong, the cause is in the catalog or the locale, not in the generator; report it.

- [ ] **Step 12: Run the test and the gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 -v 2>&1 | grep -E "^(\s*--- |ok|FAIL)" && GOTMPDIR='D:\tmp\go-build' go vet ./cmd/flowsdoc && gofmt -l cmd/flowsdoc
```

Expected: `--- PASS: TestFlowsDocBlocksMatchTheCatalog` with three `--- PASS` subtests, `ok  	aurago/cmd/flowsdoc`; `go vet` and `gofmt -l` print nothing.

- [ ] **Step 13: Prove the test fails on real drift, then restore.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && sed -i 's/"easydrag.node.trigger_manual.label": "Manual start"/"easydrag.node.trigger_manual.label": "Manual starter"/' ui/lang/easydrag/en.json && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 2>&1 | grep -E "differ from the catalog|^(ok|FAIL)"; git checkout -- ui/lang/easydrag/en.json && git status --short ui/lang
```

Expected: `documentation/manuals/en/24-easydrag.md: the generated blocks differ from the catalog; run `go run ./cmd/flowsdoc` from the repository root and commit the result` (the agent manual shows the same line, because its list carries the label) and `FAIL`; after the checkout, `git status --short ui/lang` prints nothing. If the `sed` changed nothing (the line differs), edit the label by hand for this check and restore it the same way.

- [ ] **Step 14: Commit.** Write `C:/disposable/easydrag-s0c/c2-msg.txt`:

```
docs(flows): generate the trigger and step reference from the node catalog

cmd/flowsdoc builds the catalog with RegisterCatalog (every tool available) and
rewrites only the text between BEGIN/END GENERATED markers: flows-triggers and
flows-steps in chapter 24 (en and de, new closing reference section) and
flows-triggers in the manage_missions tool manual. Columns: name, category, what it
does, effects, and what Simulation does (SimulationBehaviorOf). Output is
deterministic (palette order, then label, LF, no dates).

TestFlowsDocBlocksMatchTheCatalog renders every block in memory, compares it with the
files, names `go run ./cmd/flowsdoc` on drift, and fails when a marker pair is missing
or a label change would go unnoticed.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git add cmd/flowsdoc/main.go cmd/flowsdoc/render.go cmd/flowsdoc/flowsdoc_test.go documentation/manuals/en/24-easydrag.md documentation/manuals/de/24-easydrag.md prompts/tools_manuals/manage_missions.md && git commit -F C:/disposable/easydrag-s0c/c2-msg.txt && git status --short
```

Expected: one commit; `git status --short` prints nothing.

---

### Task C.3: The hand-written manual fixes (en and de)

Five fixes in each chapter, spec §5.3. Every edit is an Edit with the exact old string (taken from main at b52103d51) or an insertion before an exact anchor. The generated blocks are not touched. German uses lowercase "du". The new text matches each chapter's voice: the English chapter states facts in plain sentences with *italic* UI words; the German chapter does the same with „…“ for quoted text.

**Files:**
- Modify: `WT/documentation/manuals/en/24-easydrag.md` (TOC; "The Editor" after "Collection expressions"; "Testing" bullets; "Runs and Notifications" bullets; "When a Step Fails" intro)
- Modify: `WT/documentation/manuals/de/24-easydrag.md` (the same places)

- [ ] **Step 1: Read what Part A landed, and fix the quoted UI words.** The manual quotes UI words; they must match the locale files as they are on main now.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git log --oneline -12 && node -e "
const en = require('./ui/lang/easydrag/en.json'), de = require('./ui/lang/easydrag/de.json');
const keys = Object.keys(en).filter(k => /run_stopped_by|error_flow_start_failed|step_effect_uncertain\$|file_write.*(if_exists|card|summary)|if_exists_(overwrite|unique|fail)|if_exists.*card|schedule_(minutes|hours|day).*help|schedule_.*\.help|run_failed\$/.test(k));
for (const k of keys) console.log(k, '|', en[k], '|', de[k]);" && grep -n "FLOW_START_FAILED" internal/flows/*.go | head -3 && grep -n "card_label" internal/flows/*.go | head -3
```

Expected: the log shows Part A's commits (start failure, stop hint, file.write card, schedule help); the node script prints at least `easydrag.ui.run_stopped_by`, `easydrag.ui.error_flow_start_failed`, `easydrag.ui.step_effect_uncertain`, the three file.write card keys and the schedule help keys; the greps print at least one line each. If Part A is missing (no `FLOW_START_FAILED`, no `run_stopped_by`), stop: C.3 is blocked, tell the coordinator.

The text below uses these words; compare them with the output and replace only the quoted UI word where the locale differs (nothing else in the text changes):

| Quoted in the text | en | de | locale key |
|---|---|---|---|
| stop hint | Ended by Stop: *name of the stop step* | Durch Stopp beendet: *Name des Stopp-Schritts* | `easydrag.ui.run_stopped_by` |
| cut-off step | Effect may have happened | Wirkung möglicherweise erfolgt | `easydrag.ui.step_effect_uncertain` |
| file.write card word, overwrite | overwrites | überschreibt | file_write `if_exists` overwrite card key |
| file.write card word, unique | new name | neuer Name | `if_exists` unique card key |
| file.write card word, fail | aborts | bricht ab | `if_exists` fail card key |
| failed run status | Run failed | Lauf fehlgeschlagen | `easydrag.ui.run_failed` |

The spec calls the cut-off marker "unklar". The UI word is the one in the table (`step_effect_uncertain` reads "Wirkung möglicherweise erfolgt"; "unklar" appears nowhere in the step chip). The manual quotes the real chip text. If Part A introduced a separate short word "unklar" for the run view, add it in parentheses after the chip text in fix 2.

- [ ] **Step 2: Fix 1, the Simulation default (en).** In `WT/documentation/manuals/en/24-easydrag.md`, "Testing", insert a new bullet directly after the bullet that starts `- **Test** saves the draft first;` (a single line, the first bullet of that list):

```
- The dialog opens in **Simulation** when the steps it would run have effects outside AuraGo (named ones, or a risky step whose effects cannot be named) and every step of the flow can be simulated. A flow without such steps opens as a **Real test**, and so does a flow with a step that cannot be simulated: a generic tool step, *Persistent wait* or *Request approval*. The check covers the whole flow, or, with *Test step*, the steps that run. The mode you pick stays: EasyDrag never switches a test you chose, or one already in Simulation, back to a real test.
```

- [ ] **Step 3: Fix 1, the Simulation default (de).** In `WT/documentation/manuals/de/24-easydrag.md`, „Testen“, insert directly after the line that starts `- **Testen** speichert zuerst den Entwurf;` (line 353):

```
- Der Dialog öffnet in der **Simulation**, wenn die Schritte, die er ausführen würde, Auswirkungen außerhalb von AuraGo haben (benannte oder ein riskanter Schritt, dessen Auswirkungen sich nicht benennen lassen) und sich jeder Schritt des Flows simulieren lässt. Ein Flow ohne solche Schritte öffnet als **Echter Test**, ebenso ein Flow mit einem Schritt, der sich nicht simulieren lässt: einem generischen Werkzeug-Schritt, *Fortsetzbare Wartezeit* oder *Freigabe anfordern*. Geprüft wird der ganze Flow, bei *Schritt testen* die Schritte, die laufen. Eine Auswahl von dir bleibt bestehen: EasyDrag schaltet einen gewählten oder bereits simulierten Test nie auf einen echten Test zurück.
```

- [ ] **Step 4: Fix 2, "durch Stopp beendet" (en and de).** Replace the whole bullet.

English, old text (one line, "Runs and Notifications"):

```
- When a *Stop* step ends a run while other steps still run, they get up to 30 seconds to finish; then they are stopped and show *Effect may have happened*: what they send or change may already have happened. If one of them fails after a stop with *Success*, the run counts as failed; after a stop with *Error*, the stop's message stays.
```

New text:

```
- When a *Stop* step ends a run with *Success*, the run stays successful and says so: the run view and the test result show *Ended by Stop: <name of the stop step>*. Steps that were still running get up to 30 seconds to finish; then they are stopped and, if they could have sent or changed something, show *Effect may have happened*: what they send or change may already have happened. Steps the stop never reached show as skipped. If one of the running steps fails after a stop with *Success*, the run counts as failed and shows no stop hint; after a stop with *Error*, the run fails with the stop's message.
```

German, old text (line 408):

```
- Beendet ein *Stopp*-Schritt einen Lauf, während andere Schritte noch laufen, haben diese bis zu 30 Sekunden Zeit; danach werden sie gestoppt und zeigen *Wirkung möglicherweise erfolgt*: Was sie senden oder ändern, kann schon passiert sein. Scheitert einer davon nach einem Stopp mit *Erfolg*, gilt der Lauf als fehlgeschlagen; nach einem Stopp mit *Fehler* bleibt dessen Meldung.
```

New text:

```
- Beendet ein *Stopp*-Schritt einen Lauf mit *Erfolg*, bleibt der Lauf erfolgreich und sagt das: Die Laufansicht und das Testergebnis zeigen *Durch Stopp beendet: <Name des Stopp-Schritts>*. Schritte, die noch liefen, haben bis zu 30 Sekunden Zeit; danach werden sie gestoppt und zeigen, wenn sie etwas gesendet oder geändert haben könnten, *Wirkung möglicherweise erfolgt*: Was sie senden oder ändern, kann schon passiert sein. Schritte, die der Stopp nie erreicht hat, gelten als übersprungen. Scheitert einer der laufenden Schritte nach einem Stopp mit *Erfolg*, gilt der Lauf als fehlgeschlagen und zeigt keinen Stopp-Hinweis; nach einem Stopp mit *Fehler* scheitert der Lauf mit dessen Meldung.
```

- [ ] **Step 5: Fix 3, file.write overwrites (en and de).** Insert a new paragraph at the end of the intro of "When a Step Fails" / „Wenn ein Schritt scheitert“, after the paragraph that ends with the sentence about the filesystem tool's permissions, and before the first bullet of the section (leave a blank line on both sides).

English, after `The filesystem tool's permissions and protected paths still apply.`:

```
**Write file** replaces a file that already exists. This is the default of *If the file exists* (*Overwrite*); the other choices are *Keep both (new name)* and *Stop with an error*. The step's card shows the choice after the path, for example `reports/daily.txt · overwrites`, so a flow that replaces its file says so at a glance. A retried step writes the same file again.
```

German, after `Die Berechtigungen und geschützten Pfade des Dateisystem-Werkzeugs gelten weiterhin.`:

```
**Datei schreiben** ersetzt eine Datei, die schon existiert. Das ist die Voreinstellung von *Wenn die Datei existiert* (*Überschreiben*); die anderen Möglichkeiten sind *Beide behalten (neuer Name)* und *Mit Fehler abbrechen*. Die Karte des Schritts zeigt die Wahl hinter dem Pfad, zum Beispiel `reports/daily.txt · überschreibt`; ein Flow, der seine Datei ersetzt, sagt das also auf einen Blick. Ein wiederholter Schritt schreibt dieselbe Datei erneut.
```

(Last sentence: the code comment at `catalog_actions_docs.go:444-451` names the retry as the reason the default stays "overwrite"; with "new name" a retry would create a second file.)

- [ ] **Step 6: Fix 4, start failures are failed runs (en and de).** Insert a new bullet directly after the bullet that starts `- A failed live run notifies as the flow settings say` (en) and `- Ein fehlgeschlagener Live-Lauf benachrichtigt so` (de, line 406).

English:

```
- A trigger that cannot start the flow is a failed run too. When a trigger fires and the flow is not published, has no trigger that is turned on, or cannot start for another reason (for example a version that no longer passes the checks), EasyDrag records a run with the status *Run failed*, the code `FLOW_START_FAILED` and the reason in plain words, notifies as the flow settings say, and notes the failure on the mission in Mission Control. The same reason several times in a row is reported once, so a scheduled flow that is not published does not notify every minute. A different reason, or a start that works in between, begins a new report; after AuraGo restarts, the first failure is reported again. Refusals are not failures and record no run: paused flows, a full queue and EasyDrag being switched off are only noted on the mission.
```

German:

```
- Ein Auslöser, der den Flow nicht starten kann, ist ebenfalls ein fehlgeschlagener Lauf. Feuert ein Auslöser und der Flow ist nicht veröffentlicht, hat keinen eingeschalteten Auslöser oder kann aus einem anderen Grund nicht starten (zum Beispiel eine Version, die die Prüfungen nicht mehr besteht), legt EasyDrag einen Lauf mit dem Status *Lauf fehlgeschlagen*, dem Code `FLOW_START_FAILED` und dem Grund in Klartext an, benachrichtigt so, wie es die Flow-Einstellungen sagen, und vermerkt den Fehler an der Mission in Mission Control. Derselbe Grund mehrmals hintereinander wird nur einmal gemeldet; ein geplanter Flow, der nicht veröffentlicht ist, benachrichtigt also nicht jede Minute. Ein anderer Grund oder ein dazwischen geglückter Start beginnt eine neue Meldung; nach einem Neustart von AuraGo wird der erste Fehler wieder gemeldet. Abweisungen sind keine Fehler und legen keinen Lauf an: pausierte Flows, eine volle Warteschlange und abgeschaltetes EasyDrag werden nur an der Mission vermerkt.
```

- [ ] **Step 7: Fix 5, the schedule modes (en).** Add a TOC sub-entry after the line `   - [Visual groups](#visual-groups)`:

```
   - [Schedule trigger](#schedule-trigger)
```

Insert the section directly before the line `## Saving` (after the "Collection expressions" section), with one blank line on both sides:

```
### Schedule trigger

The **Schedule** trigger starts the flow at fixed times. *Repeat* offers seven modes:

| Mode | What you enter | When it runs |
|---|---|---|
| Every few minutes | *Minutes*: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 or 30 | every n minutes from the full hour (with 15: at :00, :15, :30 and :45) |
| Every few hours | *Hours*: 1, 2, 3, 4, 6, 8 or 12 | at minute 0, every n hours from 00:00 (with 6: at 00:00, 06:00, 12:00 and 18:00) |
| Daily | *Time* (HH:MM) | every day at that time |
| Weekdays (Mon–Fri) | *Time* | Monday to Friday at that time |
| Weekly | *Time* and *Weekdays* | on the chosen days |
| Monthly | *Time* and *Day of month* (1–31) | on that day of the month |
| Cron expression | 5 or 6 fields | as the expression says, at most once a minute |

An interval has to divide the hour (60 minutes) or the day (24 hours) evenly, or the gaps would be uneven: every 7 minutes would run at :56 and again at :00. EasyDrag therefore refuses other values, and the field names the valid ones.

A month without the chosen day skips that run. *Day of month* 29, 30 or 31 does not move to the last day of the month: day 31 runs only in January, March, May, July, August, October and December, day 30 never in February, and day 29 in February only in leap years.
```

- [ ] **Step 8: Fix 5, the schedule modes (de).** TOC entry after `   - [Visuelle Gruppen](#visuelle-gruppen)`:

```
   - [Zeitplan-Auslöser](#zeitplan-auslöser)
```

Section, directly before the line `## Speichern`:

```
### Zeitplan-Auslöser

Der Auslöser **Zeitplan** startet den Flow zu festen Zeiten. *Wiederholung* bietet sieben Arten:

| Art | Was du eingibst | Wann er läuft |
|---|---|---|
| Alle paar Minuten | *Minuten*: 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 oder 30 | alle n Minuten ab der vollen Stunde (bei 15: um :00, :15, :30 und :45) |
| Alle paar Stunden | *Stunden*: 1, 2, 3, 4, 6, 8 oder 12 | in Minute 0, alle n Stunden ab 00:00 Uhr (bei 6: um 00:00, 06:00, 12:00 und 18:00 Uhr) |
| Täglich | *Uhrzeit* (HH:MM) | jeden Tag zu dieser Zeit |
| Werktags (Mo–Fr) | *Uhrzeit* | Montag bis Freitag zu dieser Zeit |
| Wöchentlich | *Uhrzeit* und *Wochentage* | an den gewählten Tagen |
| Monatlich | *Uhrzeit* und *Tag im Monat* (1–31) | an diesem Tag des Monats |
| Cron-Ausdruck | 5 oder 6 Felder | wie der Ausdruck es sagt, höchstens einmal pro Minute |

Ein Intervall muss die Stunde (60 Minuten) oder den Tag (24 Stunden) ohne Rest teilen, sonst wären die Abstände ungleich: Alle 7 Minuten liefe um :56 und gleich wieder um :00. EasyDrag lehnt andere Werte deshalb ab, und das Feld nennt die gültigen.

Hat ein Monat den gewählten Tag nicht, entfällt der Lauf in diesem Monat. *Tag im Monat* 29, 30 oder 31 rückt nicht auf den letzten Tag des Monats vor: Tag 31 läuft nur im Januar, März, Mai, Juli, August, Oktober und Dezember, Tag 30 nie im Februar und Tag 29 im Februar nur in Schaltjahren.
```

- [ ] **Step 9: Check the edits.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git diff --stat && git diff --check && grep -c "Schedule trigger" documentation/manuals/en/24-easydrag.md && grep -c "Zeitplan-Auslöser" documentation/manuals/de/24-easydrag.md && grep -n "FLOW_START_FAILED" documentation/manuals/*/24-easydrag.md | cut -c1-90 && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 2>&1 | tail -2
```

Expected: `git diff --stat` lists only the two chapters (about 30 lines added in each, 1 replaced); `git diff --check` prints nothing; each `grep -c` prints `2` (the TOC entry and the heading); two `FLOW_START_FAILED` lines (one per language); `ok  	aurago/cmd/flowsdoc` (the drift test still passes, because the generated blocks did not change).

- [ ] **Step 10: Read the German and English text once against the UI words.** Re-run the node script from step 1 and compare each quoted word in the new text (`Ended by Stop`, `Effect may have happened`, `overwrites`, `Run failed` and the German forms) with its locale value. Where one differs, edit the manual to the locale's word. This is the only edit allowed in this step.

- [ ] **Step 11: Commit.** Write `C:/disposable/easydrag-s0c/c3-msg.txt`:

```
docs(easydrag): chapter 24 describes the Simulation default, schedules, stops, file.write and start failures

Both languages. The test dialog's Simulation default; a new "Schedule trigger" section
with the seven modes, the valid minute and hour intervals and the rule that days 29-31
skip months without that day; "Ended by Stop" and "Effect may have happened" for runs
a Stop step ends; file.write overwrites by default and shows it on the card; a trigger
that cannot start the flow is a failed run (FLOW_START_FAILED), reported once per
streak. The generated reference blocks are untouched.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git add documentation/manuals/en/24-easydrag.md documentation/manuals/de/24-easydrag.md && git commit -F C:/disposable/easydrag-s0c/c3-msg.txt && git status --short
```

Expected: one commit; `git status --short` prints nothing.

---

### Task C.4: The agent manual, closing the read_only / allow_publish point

The only new thing in `prompts/tools_manuals/manage_missions.md` is the generated trigger block (C.2, step 9). Spec §5.4 closes roadmap acceptance point 321 with evidence. This task checks the evidence against main as it is now. It commits nothing unless a check fails.

**Files:**
- Read: `WT/prompts/tools_manuals/manage_missions.md:20-21`, `WT/internal/config/flows.go:25-33`, `WT/internal/agent/agent_dispatch_comm.go:1405-1418`, `WT/internal/agent/manage_missions_flow_read_only_test.go`
- Modify (only on a mismatch): `WT/prompts/tools_manuals/manage_missions.md:21`

- [ ] **Step 1: Print the three places and compare them.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && sed -n 21p prompts/tools_manuals/manage_missions.md | cut -c560-960 && sed -n 25,33p internal/config/flows.go && sed -n 1409,1415p internal/agent/agent_dispatch_comm.go
```

Expected, line by line:
1. The manual says: "With `flows.agent.read_only` on, `run` and `run_now` on a flow mission are refused, while `list` and `history` still work. The switch only stops you from starting flows directly: flows that the owner's own triggers start (for example when a mission completes) still run. The agent cannot publish flows."
2. `FlowsAgentConfig`'s comment says the same: read-only means list and history yes, `run` and `run_now` on a flow mission refused, read on every call (no restart), no effect on the owner's own triggers such as `mission_completed`; "AllowPublish has no effect: the agent cannot publish flows."
3. The dispatch enforces exactly that: `if targetIsFlow && cfg.Flows.Agent.ReadOnly && (req.Operation == "run" || req.Operation == "run_now")` answers `flow_read_only`; `list` and `history` are not in the condition.

- [ ] **Step 2: Run the test that pins the refusal, and the manual's own tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./internal/agent -run '(?i)(ManageMissionsFlow|FlowReadOnly)' -count=1 2>&1 | tail -3 && GOTMPDIR='D:\tmp\go-build' go test ./internal/config -run '(?i)flows' -count=1 2>&1 | tail -2
```

Expected: `ok  	aurago/internal/agent` (it includes `TestManageMissionsFlowReadOnlyRefusesRuns`) and `ok  	aurago/internal/config`. The agent package is large; if it takes longer than 10 minutes, run only `-run TestManageMissionsFlowReadOnlyRefusesRuns`.

- [ ] **Step 3: Outcome.**
  - All three agree (expected): no edit, no commit. Put this evidence into the report: `prompts/tools_manuals/manage_missions.md:21` (read_only behaviour, owner triggers still run, agent cannot publish), `internal/config/flows.go:25-33` (the struct comment; `AllowPublish` has no effect), `internal/agent/agent_dispatch_comm.go:1409-1415` (enforcement), `TestManageMissionsFlowReadOnlyRefusesRuns` (pin). Note that line 21 names `flows.agent.read_only` and says the agent cannot publish, but never names `allow_publish`; that is accurate, because the key does nothing and the agent never sets it.
  - A check disagrees (for example the dispatch now also refuses `list`): edit only the sentence in line 21 that is wrong, using the wording of the struct comment, run `go test ./cmd/flowsdoc -count=1` (the generated block must not change), commit with message `docs(agent): manage_missions describes flows.agent.read_only as it behaves` and the GitNexus and Co-Authored-By lines, and report the mismatch.

---

### Task C.5: Put the drift test into T0

Runs only after Part B has landed on main. Part B defines `scripts/test-easydrag-t0.mjs` with one array at its top, `GO_STEPS`, each entry `{ label, args, env }`.

**Files:**
- Modify: `WT/scripts/test-easydrag-t0.mjs` (the `GO_STEPS` array)

- [ ] **Step 1: Check that Part B is on main, and bring the worktree up to date.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git show main:scripts/test-easydrag-t0.mjs > /dev/null && echo "t0 on main" && git log main --oneline -3 -- scripts/test-easydrag-t0.mjs
```

Expected: `t0 on main` and at least one commit. If `git show` fails (`exists on disk, but not in 'main'`), Part B has not landed: stop and tell the coordinator; do not create the script.

Then, in the worktree (the one place this part rebases; it pushes nothing and rewrites only this part's own unpushed commits):

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git status --short && git rebase main && ls scripts/test-easydrag-t0.mjs && git log --oneline -6
```

Expected: `git status --short` prints nothing before the rebase; the rebase ends with `Successfully rebased and updated refs/heads/feat/easydrag-s0c-docs` (a conflict can only be in a manual if main changed chapter 24 meanwhile: resolve by keeping both sides' text, run `go run ./cmd/flowsdoc`, `git add` the file and `git rebase --continue`); the script exists; the log shows this part's commits on top of Part B's.

- [ ] **Step 2: Re-run the drift test after the rebase.** Part A or B may have changed a label or a manual meanwhile.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go run ./cmd/flowsdoc; GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 2>&1 | tail -2; git status --short
```

Expected: `go run` prints nothing, the test prints `ok`, `git status --short` prints nothing. If `go run` printed `updated ...`, a catalog text changed on main since C.2: add the regenerated files to this task's commit (`git add` them) and say so in the commit body.

- [ ] **Step 3: Read the array.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && grep -n "GO_STEPS" -A22 scripts/test-easydrag-t0.mjs | head -40
```

Expected: an array of entries shaped `{ label: '...', args: ['test', ...], env: {} }`, one for `./internal/flows/...`, one for `./internal/server -run '(?i)flow'`, one for `./internal/tools -run '(?i)flow'` and one for `./ui -run '(?i)easydrag'`. If an entry's shape differs from the brief (for example `env` is missing), follow the shape that is there.

- [ ] **Step 4: Add the step.** Append this entry after the last element of `GO_STEPS` (keep the array's comma style; the entry needs a trailing comma when it is not the last element, and the previous last element needs one now):

```js
    { label: 'flowsdoc drift', args: ['test', './cmd/flowsdoc'], env: {} },
```

No `-count=1`: like Part B's other entries, the step relies on Go's test cache, and a caller forces a full run with `GOFLAGS=-count=1`. The test reads the manuals and the locale files, and Go's test cache tracks opened files, so a changed manual or label invalidates the cached result and the cache cannot hide a drift.

- [ ] **Step 5: Check and run.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && node --check scripts/test-easydrag-t0.mjs && grep -n "flowsdoc" scripts/test-easydrag-t0.mjs && GOTMPDIR='D:\tmp\go-build' npm run test:easydrag:t0 2>&1 | tail -15
```

Expected: `node --check` prints nothing; the grep shows the new entry; the T0 run lists the `flowsdoc drift` step as passed (the exact wording is Part B's) and ends in its success line. Note the wall time of the whole run and of the `flowsdoc drift` step for the report (the step should add about a second: the packages it imports are already built by the `internal/flows` step). T0 takes minutes on a cold cache; the spec's 60 s target is for a warm cache and Part B's measurement owns that number.

- [ ] **Step 6: Commit.** Write `C:/disposable/easydrag-s0c/c5-msg.txt`:

```
test(easydrag): T0 runs the manuals drift test

Adds `go test ./cmd/flowsdoc` to GO_STEPS of scripts/test-easydrag-t0.mjs, so a catalog
label, description or effect that changes without `go run ./cmd/flowsdoc` fails the
fast tier. CI already runs the test in the Go job (go test ./...).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && git add scripts/test-easydrag-t0.mjs && git commit -F C:/disposable/easydrag-s0c/c5-msg.txt && git status --short
```

Expected: one commit; `git status --short` prints nothing.

- [ ] **Step 7: Final checks and the report.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-docs && GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc ./internal/flows/... -count=1 2>&1 | tail -6 && GOTMPDIR='D:\tmp\go-build' go vet ./cmd/flowsdoc ./internal/flows && gofmt -l cmd/flowsdoc internal/flows && git log --oneline main..HEAD
```

Expected: `ok` for `aurago/cmd/flowsdoc` and `aurago/internal/flows`, nothing from vet or gofmt, and four commits (C.1, C.2, C.3, C.5; C.4 only if it needed an edit).

This part edits neither the spec nor the roadmap. The report to the coordinator carries: the commit hashes; the Simulation rule's location and the new function; the agent-manual evidence from C.4; the T0 times from step 5; the deviations below; and ledger evidence for the coordinator to record on main: row 12 (FD-12 manual drift fixes) is `TestFlowsDocBlocksMatchTheCatalog` plus the hand-written sections of C.3, and row 14 (agent manual) is `manage_missions.md:21`, `internal/config/flows.go:25-33`, `agent_dispatch_comm.go:1409-1415` and `TestManageMissionsFlowReadOnlyRefusesRuns`.

---

## Coverage

| Spec item | Task |
|---|---|
| §5.1 the Simulation column comes from one rule, extracted if needed | C.1 (`SimulationBehaviorOf`), C.2 (the column) |
| §5.1 generator `cmd/flowsdoc`, markers, deterministic output | C.2 |
| §5.1 `flows-triggers` and `flows-steps` in chapter 24 en/de, new closing section | C.2 steps 7, 8 |
| §5.1 `flows-triggers` in `manage_missions.md` with type ids | C.2 step 9 |
| §5.2 `TestFlowsDocBlocksMatchTheCatalog`, names the command, fails without a marker pair | C.2 steps 2, 3, 6, 12, 13 |
| §5.3 Simulation default in the test dialog | C.3 steps 2, 3 |
| §5.3 schedule modes, valid intervals, days 29-31 | C.3 steps 7, 8 |
| §5.3 "durch Stopp beendet" and the cut-off steps | C.3 step 4 |
| §5.3 file.write overwrites and shows it on the card | C.3 step 5 |
| §5.3 start failures are failed runs, one notification per streak | C.3 step 6 |
| §5.4 `read_only` / `allow_publish` closed with evidence | C.4 |
| §8 drift test in T0 | C.5 |
| §7 row 12 (FD-12 manual drift fixes) | C.2 + C.3 |
| §7 row 14 (agent manual) | C.4 |
| §10 "the drift test is green, and fails when a catalog label changes without regenerating" | C.2 steps 12, 13 |

## Deviations from the spec

1. **`manage_missions.md` lines 20-21 are not replaced.** Line 20 is the list of event kinds of `triggered` missions (webhook, mission completed, email, MQTT, system startup, invasion, device, Fritz!Box, budget, Home Assistant, Planner reminders). That is the mission trigger enum, not the flow trigger nodes (it has "invasion" and no "schedule" or "manual"), and replacing it would remove true information. Line 21 only says "Its triggers are the flow's trigger nodes". No hand-written flow trigger list exists there. The generated block therefore gets its own section, "Flow trigger types", between the flow paragraph and "Examples".
2. **The Simulation column has four outcomes, not two.** The spec says "runs for real, or simulated". The code has more: pure steps run locally; other steps are replaced by a sample output; durable steps (and, in the UI, generic tool steps) cannot be simulated; triggers supply the test data. `SimulationBehaviorOf` returns all four, and the manual says so.
3. **Category order is the palette order, not alphabetical by category name.** The spec says "sorted by category, then label". The tables use `flows.CategoryOrder` (trigger, logic, ai, web, documents, notify, smart home, planner) as the editor's palette does, then the localized label, then the type id.
4. **"unklar" is quoted as the chip text.** The spec (§3.2) and the brief call the cut-off marker "unklar". The UI's `step_effect_uncertain` reads "Wirkung möglicherweise erfolgt" / "Effect may have happened", and "unklar" appears in no step chip. The manual quotes the real chip text (C.3 step 1 tells the implementer to add "unklar" only if Part A introduced it).
5. **The test has two subtests beyond "files match".** "A document without a marker pair fails" and "a changed catalog label is detected" implement the two acceptance sentences in §5.2 and §10 directly, in the same test function; no other test is added. C.1 adds `TestSimulationBehaviorOf` because the existing covering tests (`TestSimulationIsIsolated`, `TestDescribeNodeTypesSimulatable`) reach only two of the four outcomes and one of the seven local types.
6. **Generic `tool.*` steps are not in the reference.** `RegisterCatalog` never registers them (they depend on the live tool list); the chapter's intro says so.
7. **The `if_exists` option values are `overwrite`, `unique`, `fail`** (the brief says `rename` in one place). The manual names them by their UI labels and card words, so it is not affected; Part A's card keys follow the real values.
8. **C.5 rebases.** The conventions forbid a rebase inside a task; the task list for this part asks C.5 to rebase onto main after Part B. It is the only rebase, in this part's own unpushed branch.

## Points that need a decision

1. **Agent manual markers.** The two marker comment lines end up in the agent's prompt text. They cost a few tokens and are harmless; the alternative (a generator that finds the block by heading instead of markers) would break the spec's marker contract. Recommendation: keep the markers.
2. **Label edits by other sessions now fail T0 until `go run ./cmd/flowsdoc` is run.** That is the intent of FD-16, but a session that edits `en.json` or `de.json` labels or descriptions of a node type without knowing it will see `TestFlowsDocBlocksMatchTheCatalog` fail, with the command in the message. Recommendation: the coordinator adds one line to `internal/flows/AGENTS.md` (Part A's file, so not done here): "after changing a node label, description or effect, run `go run ./cmd/flowsdoc`".
3. **`logic.wait` in Simulation.** It is not in the pure list, so the column says "Replaced by your sample output" for it. The chapter's existing text ("*Wait* frees its run slot") does not contradict that, but a Wait that simulates instantly may be wanted later. Not planned here; it would change `simulationPureType`, which this task keeps behaviour-neutral.
