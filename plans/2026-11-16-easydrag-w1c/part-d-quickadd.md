## Part D: The quick-add (header, suggestions, "Show all")

Spec: `plans/2026-11-16-easydrag-w1c-spec.md` §6, owner decision 5 ("your own history first, then a curated list per category, at most 5"). Roadmap: E9 "Build with real things", the UX-04 quick-add header slice. Part D gives the quick-add a real header that says what the new step goes after, offers the steps that usually come next, and hands over to the full palette without losing the source step.

**Shell and worktree.** Git Bash. Worktree `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-quickadd`, branch `feat/easydrag-w1c-quickadd`, cut from main when the part starts; `W` below means that path. Every command starts with `cd W && …` (or `cd C:/Users/Andi/Documents/repo/AuraGo` for the worktree setup). Never edit, stage or commit in the main checkout. Go commands carry `GOTOOLCHAIN=go1.27.4`; `GOTMPDIR='D:\tmp\go-build'` for `internal/flows`, `internal/tools`, `cmd/...` and `ui`, none for `internal/server`. Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots`, `GOTMPDIR='D:\tmp\go-build'` and the Bash sandbox off.

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1c-quickadd -b feat/easydrag-w1c-quickadd main
mkdir -p C:/disposable/easydrag-w1c D:/tmp/go-build
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-quickadd && ls plans/2026-11-16-easydrag-w1c/keys/ C:/disposable/easydrag-w1c/apply-plan-edits.mjs
```

Expected: the key files `d-2.json  d-3.json  d-4.json` (the coordinator commits them to main before this part starts, together with the other parts' key files) and the edit script. If a key file is missing, stop and ask the coordinator; do not write it by hand. The node_modules of a worktree: `scripts/test-easydrag*.mjs` need none.

**Depends on.** W1a and W1b on main. Part D touches the palette (`easydrag-palette.js`), the catalog Go files (`internal/flows/catalog.go` and a new file), the locale files (through key files), the CSS file and tests. It does **not** touch `easydrag-editor.js`, `-interact.js`, `-canvas.js`, `-kit.js` or the fixture `ui/testdata/easydrag-fixture.js` (Part A's). Parts A, B and C may land first or later; on a rebase conflict in the locale files take main's files and run the key files again in task order (`d-2`, `d-3`, `d-4`). Part D changes no API field of Part A.

**Baseline (before any change).**

```
cd W && npm run test:easydrag:t0
cd W && wc -l ui/js/desktop/apps/easydrag-palette.js && grep -c "" ui/css/desktop-app-easydrag.css
```

Expected: T0 ends without a FAIL line (note the time, about 90-200 s); the palette has 504 lines. If main has moved, the new count is the baseline; Part D ends at the baseline plus about 150 lines (see "Size of easydrag-palette.js" below).

**How the edits are made.** The JS, CSS and Go files are CRLF in a Windows checkout; the Write and Edit tools mangle some escapes. Edits to existing files are therefore *edit groups* in this file, the blocks between `<!-- edits:NAME -->` and `<!-- /edits -->`, each with the exact old and new text. Apply one with the edit script of the W1b plans (already at `C:/disposable/easydrag-w1c/apply-plan-edits.mjs`; it keeps each file's line endings, requires each old text to occur exactly once and writes nothing unless every edit of the group matches):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md <GROUP>
```

`--check` only verifies. A new file is written with the Write tool as the step says. Locale files change only through the key files.

**IMPACT** (done while planning, on main d87930dfc; the index is stale by about one line per function and does not index worktrees, so the greps are the check on the worktree):

| Symbol | Result |
|---|---|
| `openQuickAdd` (`easydrag-palette.js:358`) | LOW; 7 impacted, 4 direct, 2 processes. Callers by grep: `easydrag-editor.js:286` (Ctrl+K with one step), `:579` (the `quick-add` bus event: card tools, port "+", pulled wire, canvas menu, empty-canvas button, inspector Insert via `requestInsert`), tests |
| `addByClick` (`:220`, inside `createPanel`) | LOW; 7 impacted, 2 direct |
| `DescribeNodeTypes` (`internal/flows/catalog.go:156`) | LOW; 4 impacted: `flowNodeTypesBody` (`internal/server/flows_handlers_catalog.go:182`), `Service.BlockCatalog` (`blocks_nodes.go:113`), `cmd/flowsdoc` (`render.go:140`) |
| `place` (`:60`) | UNKNOWN: the index finds 52 symbols named `place` and none for this file. Confirmed by `grep -rn "palette\.place\|[^.]place(" ui/js scripts`: callers are `startDrag` (drop), `addByClick`, `openQuickAdd.choose`, and `scripts/test-easydrag-durable.mjs:55` |

No HIGH or CRITICAL. The signature of `place` does not change; it only records one more fact. To re-run one (the log lines about unusable registry entries are noise):

```
cd C:/Users/Andi/Documents/repo/AuraGo && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "openQuickAdd" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -v '"level"'
```

Re-run the checks in Step 1 of D.1 and D.2 before editing: if a caller list differs (a new caller of `openQuickAdd` or `place`), read it before you go on.

**Escapes.** No code or key file below contains a backslash-u sequence; the middle dot of the scope banner is `String.fromCodePoint(0xB7)`. Keep it so.

**Every commit body ends with** `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. The message files are written with the Write tool to `C:/disposable/easydrag-w1c/` and committed with `git commit -F`. One commit per task; never amend, rebase or push inside a task.

### The design in one page

**Header (D.2).** `.ed-quick` gets a first row `.ed-quick-head` ([`data-ed-quick-head`]). Its text, which is also the dialog's `aria-label`:

| Entry | Request | Header (en / de) |
|---|---|---|
| a step's "+", a card's Add tool, Ctrl+K with one step selected, a wire pulled from a port | `from` | "Steps after: {name}" / "Schritte nach: {name}" |
| Insert on a wire (wire button, inspector row) | `edge` | "Insert after: {name}" / "Einfügen nach: {name}" (the wire's source step) |
| a click on the empty canvas, the canvas menu | none | "Add step" / "Schritt hinzufügen" (existing `quick_title`) |
| the empty-canvas trigger button | `filter: 'trigger'` | "Choose a trigger" / "Auslöser wählen" (existing `empty_action`) |

`{name}` is the step's label, else its type's label. The row is set with `textContent`, the `aria-label` is escaped. A source that is gone by the time the popover opens falls back to the plain "Add step". `position()` uses the measured height (`offsetHeight`, capped at the 380 px of the CSS) and the order `render(); position();`.

**History (D.3).** `localStorage['aurago.easydrag.after']` is `{sourceType: {addedType: count}}`. `place()` counts a step that was added after a source: after `opts.from` (not for a trigger, which is never connected), or inserted on `opts.edge` (the wire's source step; not when `insertOnEdge` refused and the step went beside the wire). A source keeps its 8 most used types, the book the 40 sources used last. A damaged value reads as empty.

**Suggestions (D.3).** Only when the request has a source step: at most five entries in a group `[data-ed-quick-suggested]` ("Suggestions" / "Vorschläge") above everything else: first the history of that source type, highest count first, then the catalog's `suggest` list of that type, no duplicates, only types that are candidates of the request (no trigger; for a wire also a step with an output). Types that are not set up stay in the list and are greyed (`is-unavailable`) like in the palette. The "recent" group follows without the suggested ones, then the categories without them. Each suggested item carries `data-ed-suggested`. A search lists matches only. The first entry is the active one, so Enter takes the best suggestion.

**Curated list (D.1).** A short list per source category in Go, served as `suggest` (array of type ids) on every `NodeTypeInfo` that has outputs. The list of the category of the step, without the step's own type, without a type the registry does not hold (the block registry is smaller). Generic tool steps (`tool:<name>` categories) use the group `tool`. The lists (best first; `trigger` stands for all 14 triggers, `notify` is "messages"):

| Source category | Steps in it | Suggested next steps (type ids, in order) | Why |
|---|---|---|---|
| `trigger` | 14 triggers | `ai.step`, `web.search`, `http.request`, `notify.telegram`, `logic.if` | what most flows do first: think, fetch, call, tell, branch |
| `ai` | `ai.step`, `ai.decision` | `logic.if`, `notify.telegram`, `notify.email`, `doc.pdf_create`, `logic.set` | use the answer: branch, send, write down, reshape |
| `web` | `web.search`, `web.read`, `http.request` | `ai.step`, `logic.if`, `notify.telegram`, `doc.pdf_create`, `logic.set` | summarise or check what was fetched, then send or store |
| `documents` | `doc.pdf_create`, `doc.pdf_read`, `file.read`, `file.write` | `ai.step`, `notify.email`, `notify.telegram`, `logic.if`, `doc.pdf_create` | read it with the AI, mail or send the file |
| `notify` ("messages") | `notify.telegram`, `.whatsapp`, `.email`, `.push`, `.discord` | `logic.if`, `logic.wait`, `planner.todo_add`, `http.request`, `ai.step` | follow up: branch, wait, remember, call |
| `smart_home` | `home.assistant`, `mqtt.publish` | `logic.if`, `notify.telegram`, `notify.push`, `logic.wait`, `ai.step` | check the result, tell, wait |
| `logic` | `logic.if`, `.switch`, `.merge`, `.wait`, `.set`, `.durable_*`, `component.*` (`logic.stop` has no outputs) | `ai.step`, `notify.telegram`, `http.request`, `logic.set`, `logic.wait` | the generic continuations |
| `planner` | `planner.appointment_add`, `planner.todo_add` | `notify.telegram`, `notify.push`, `logic.if`, `ai.step`, `notify.email` | confirm what was planned |
| `tool` (every `tool:<name>`) | generic tool steps | `ai.step`, `logic.if`, `notify.telegram`, `logic.set`, `http.request` | the generic continuations |

The editor's own copy of the list is the catalog entry (`info.suggest`); the client only ever reads it.

**"Show all" (D.4).** A link `[data-ed-quick-all]` at the foot of the quick-add. It closes the quick-add without giving the focus back and emits `palette-scope` on the editor's bus; the palette panel (the only listener, registered in `createPanel`) shows a banner `[data-ed-palette-scope]` "Adds after {name}" / "Fügt nach {name} ein" (a text, a middle dot, a Cancel button), focuses its search and keeps a scope `{from, edge, name, world}`. The next step chosen in the palette (a click, Enter on an entry, or Enter in the search with a query) goes through the same function the quick-add uses, `placeStep(ed, canvas, scope, type)`, so it lands exactly where the quick-add would have put it (after the source, or on the wire). The scope ends with that one step, Cancel, Escape (taken, focus to the canvas) or hiding the palette. A request without a source (empty canvas, trigger button) emits `palette-scope` with `null`: the palette opens with its search focused and no scope. Keyboard: Tab in the quick-add's search moves to the link, Shift+Tab goes back, Tab past it and Escape close the popover as they did.

**Size of `easydrag-palette.js`.** 504 lines now, 653 after Part D (the check is in D.4). It has no hard limit and the new code is cohesive (header text, history, suggestions, placement, scope), so no new module; a new module would have to be registered in `module-loader.js`, `easyDragModules` and every Node harness list. If a rebase brings more lines into the file and it passes about 700, split `stepName`, `sourceStep`, `headerText`, `afterBook`, `rememberAfter` and `suggestedFor` into a new `easydrag-quick.js` loaded before the palette, registered in the three places.

**Shared files.** `ui/lang/easydrag/*.json` (key files only), `scripts/test-easydrag.mjs` (one call added after the kit's), `ui/css/desktop-app-easydrag.css` (quick-add rules at the `.ed-quick` block and the palette rules at `.ed-palette-list`). Nothing else is shared with Parts A, B and C.

---

### Task D.1: Curated suggestions in the Go catalog

**Files:**
- Create: `internal/flows/catalog_suggest.go`
- Create (test): `internal/flows/catalog_suggest_test.go`
- Modify: `internal/flows/catalog.go:60-90` (`NodeTypeInfo`), `:164-168` (`DescribeNodeTypes`)

- [ ] **Step 1: Verify main**

```
cd W && git log --oneline -3 && git status --short | head -5
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestRegisterCatalogCounts' -count=1 -v 2>&1 | tail -4
cd W && grep -n 'CategoryOrder = ' internal/flows/catalog.go
```

Expected: `TestRegisterCatalogCounts` PASS (42 node types, 14 triggers); `CategoryOrder` is `trigger, logic, ai, web, documents, notify, smart_home, planner`; every constant above exists (`TypeAIStep` is declared as `const TypeAIStep = "ai.step"`). For each outcome:
  - the catalog has more or fewer than 42 types, or a category is new: read `internal/flows/catalog*.go`, add the new category to the table of "The design in one page" and to `curatedSuggestions` below (the second test names the gap), and tell the coordinator;
  - a constant is missing: take the type id from the type's definition (`grep -n 'Type: ' internal/flows/catalog_*.go`) and use a string literal.

- [ ] **Step 2: Write the failing test**

Write `W/internal/flows/catalog_suggest_test.go` with the Write tool:

```go
package flows

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every curated list names real steps the editor can add after another step: a catalog type that
// is not a trigger, has an input, and appears once; a list is short and never empty.
func TestCuratedSuggestionsAreValid(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for category, list := range curatedSuggestions {
		if len(list) == 0 || len(list) > maxSuggestions {
			t.Errorf("%s: %d suggestions, want 1 to %d", category, len(list), maxSuggestions)
		}
		seen := map[string]bool{}
		for _, typ := range list {
			def, ok := reg.Lookup(typ)
			switch {
			case !ok:
				t.Errorf("%s: %q is not a catalog type", category, typ)
			case def.Trigger:
				t.Errorf("%s: %q is a trigger and cannot follow a step", category, typ)
			case len(def.InputPorts()) == 0:
				t.Errorf("%s: %q has no input", category, typ)
			}
			if seen[typ] {
				t.Errorf("%s: %q is listed twice", category, typ)
			}
			seen[typ] = true
		}
	}
}

// Every curated category and the generic tool group has a list, and the catalog serves it on each
// step that has an output (and on no other), never with the step's own type in it.
func TestEveryStepWithOutputsGetsSuggestions(t *testing.T) {
	for _, category := range append(append([]string{}, CategoryOrder...), "tool") {
		if len(curatedSuggestions[category]) == 0 {
			t.Errorf("category %q has no curated suggestions", category)
		}
	}
	for _, info := range DescribeNodeTypes(catalogRegistry(t, fullEnv()), nil) {
		hasOutputs := len(info.Outputs) > 0 || info.DynamicOutputs != ""
		if hasOutputs && len(info.Suggest) == 0 {
			t.Errorf("%s has outputs and no suggestions", info.Type)
		}
		if !hasOutputs && info.Suggest != nil {
			t.Errorf("%s has no outputs and offers %v", info.Type, info.Suggest)
		}
		for _, typ := range info.Suggest {
			if typ == info.Type {
				t.Errorf("%s suggests itself", info.Type)
			}
		}
	}
}

// A generic tool step ("tool:<name>" category) takes the list of the "tool" group, a type that the
// registry lacks is dropped, and the field reaches the catalog JSON as "suggest" only when it is set.
func TestSuggestionsForToolsRegistryAndJSON(t *testing.T) {
	present := map[string]bool{TypeAIStep: true, TypeIf: true}
	got := suggestionsFor("tool.file_reader", "tool:files", true, present)
	if strings.Join(got, ",") != TypeAIStep+","+TypeIf {
		t.Fatalf("tool suggestions = %v, want the listed types the registry has", got)
	}
	if got := suggestionsFor(TypeStop, "logic", false, present); got != nil {
		t.Fatalf("a step without outputs got %v", got)
	}
	reg := catalogRegistry(t, fullEnv())
	raw, err := json.Marshal(DescribeNodeTypes(reg, nil))
	if err != nil {
		t.Fatal(err)
	}
	var infos []map[string]any
	if err := json.Unmarshal(raw, &infos); err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		_, has := info["suggest"]
		if want := info["type"] != TypeStop; has != want {
			t.Errorf("%v: suggest present = %v, want %v", info["type"], has, want)
		}
	}
}

// The block catalog uses a smaller registry: its steps are offered only types that registry holds.
func TestBlockCatalogSuggestionsStayInsideItsRegistry(t *testing.T) {
	reg := blockRegistry()
	for _, info := range DescribeNodeTypes(reg, nil) {
		for _, typ := range info.Suggest {
			if _, ok := reg.Lookup(typ); !ok {
				t.Errorf("block step %s suggests %s, which the block registry lacks", info.Type, typ)
			}
		}
	}
}
```

- [ ] **Step 3: Run the test and see it fail**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCuratedSuggestions|TestEveryStepWithOutputs|TestSuggestionsForTools|TestBlockCatalogSuggestions' -count=1 2>&1 | head -12
```

Expected: the package does not build: `# aurago/internal/flows [aurago/internal/flows.test]`, then `catalog_suggest_test.go:13:30: undefined: curatedSuggestions`, `undefined: maxSuggestions`, `undefined: suggestionsFor`, `info.Suggest undefined (type NodeTypeInfo has no field or method Suggest)` and `too many errors`.

- [ ] **Step 4: Create the list**

Write `W/internal/flows/catalog_suggest.go` with the Write tool:

```go
package flows

import "strings"

// curatedSuggestions is the short list of next steps the quick-add offers after a step of a
// category. The key is the category of the step the quick-add was opened for; every category of
// CategoryOrder that has steps with outputs has a list, and "tool" serves the generic tool
// categories ("tool:<name>"). A list holds at most maxSuggestions type ids, best first. Every id is a
// step of RegisterCatalog that is not a trigger and has an input; TestCuratedSuggestionsAreValid
// pins that. The list is a hint for the editor (NodeTypeInfo.Suggest): a type that is not available
// is still listed, the editor greys it out.
var curatedSuggestions = map[string][]string{
	"trigger":    {TypeAIStep, TypeWebSearch, TypeHTTPRequest, TypeTelegram, TypeIf},
	"ai":         {TypeIf, TypeTelegram, TypeEmail, TypePDFCreate, TypeSet},
	"web":        {TypeAIStep, TypeIf, TypeTelegram, TypePDFCreate, TypeSet},
	"documents":  {TypeAIStep, TypeEmail, TypeTelegram, TypeIf, TypePDFCreate},
	"notify":     {TypeIf, TypeWait, TypeTodoAdd, TypeHTTPRequest, TypeAIStep},
	"smart_home": {TypeIf, TypeTelegram, TypePush, TypeWait, TypeAIStep},
	"logic":      {TypeAIStep, TypeTelegram, TypeHTTPRequest, TypeSet, TypeWait},
	"planner":    {TypeTelegram, TypePush, TypeIf, TypeAIStep, TypeEmail},
	"tool":       {TypeAIStep, TypeIf, TypeTelegram, TypeSet, TypeHTTPRequest},
}

// maxSuggestions is the most entries a list may have; the editor shows at most five.
const maxSuggestions = 5

// suggestionCategory maps the category of a step to the key of curatedSuggestions.
func suggestionCategory(category string) string {
	if strings.HasPrefix(category, "tool:") {
		return "tool"
	}
	return category
}

// suggestionsFor returns the curated next steps after a step of type typ in category: the list of
// its category, without the step's own type and without a type that is not in present (a
// registry may hold fewer types than the full catalog, as the block registry does). A step
// without any output has no next step and gets nil.
func suggestionsFor(typ, category string, hasOutputs bool, present map[string]bool) []string {
	if !hasOutputs {
		return nil
	}
	var out []string
	for _, candidate := range curatedSuggestions[suggestionCategory(category)] {
		if candidate != typ && present[candidate] {
			out = append(out, candidate)
		}
	}
	return out
}
```

- [ ] **Step 5: Serve it on every node type info**

Group `d1-catalog`: the new field of `NodeTypeInfo` and the loop of `DescribeNodeTypes`.

<!-- edits:d1-catalog -->

**Edit** `internal/flows/catalog.go`

Old:

```text
	// SampleTextKeys: see NodeDef.SampleTextKeys. The editor localizes those fields of a
	// built-in sample.
	SampleTextKeys map[string]map[string]string `json:"sample_text_keys,omitempty"`
}
```

New:

```text
	// SampleTextKeys: see NodeDef.SampleTextKeys. The editor localizes those fields of a
	// built-in sample.
	SampleTextKeys map[string]map[string]string `json:"sample_text_keys,omitempty"`
	// Suggest lists the type ids the quick-add offers first after a step of this type: a curated
	// list per category (curatedSuggestions), without the type itself. Absent for a step without outputs.
	Suggest []string `json:"suggest,omitempty"`
}
```

**Edit** `internal/flows/catalog.go`

Old:

```text
	out := make([]NodeTypeInfo, 0, len(defs))
	for _, def := range defs {
		out = append(out, describeNodeType(def, tr, TriggerSample))
	}
```

New:

```text
	present := make(map[string]bool, len(defs))
	for _, def := range defs {
		present[def.Type] = true
	}
	out := make([]NodeTypeInfo, 0, len(defs))
	for _, def := range defs {
		info := describeNodeType(def, tr, TriggerSample)
		info.Suggest = suggestionsFor(info.Type, info.Category, len(info.Outputs) > 0 || info.DynamicOutputs != "", present)
		out = append(out, info)
	}
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d1-catalog
cd W && grep -n "Suggest \[\]string\|suggestionsFor(" internal/flows/catalog.go
```

Expected: `applied 2 edit(s) of group d1-catalog in 1 file(s).`; two grep lines (the field, the call). `describeNodeType` is untouched on purpose: tests call it directly, and the registry-wide `present` set only exists in `DescribeNodeTypes`.

- [ ] **Step 6: Run the tests and see them pass**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCuratedSuggestions|TestEveryStepWithOutputs|TestSuggestionsForTools|TestBlockCatalogSuggestions' -count=1 -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"
```

Expected:
```
--- PASS: TestCuratedSuggestionsAreValid (0.00s)
--- PASS: TestEveryStepWithOutputsGetsSuggestions (0.00s)
--- PASS: TestSuggestionsForToolsRegistryAndJSON (0.00s)
--- PASS: TestBlockCatalogSuggestionsStayInsideItsRegistry (0.00s)
PASS
ok  	aurago/internal/flows	…
```

- [ ] **Step 7: Run the neighbours**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... ./cmd/flowsdoc -count=1 2>&1 | tail -4
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -3
```

Expected: `ok  	aurago/internal/flows` (about 150 s), `ok  	aurago/cmd/flowsdoc`, `ok  	aurago/internal/server`. The generated flows documentation (`cmd/flowsdoc`) does not print `Suggest`, so no document changes. If `cmd/flowsdoc` fails with a diff, run `go run ./cmd/flowsdoc` as the repository documents and add the generated files to the commit.

- [ ] **Step 8: Gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && gofmt -l internal/flows/catalog.go internal/flows/catalog_suggest.go internal/flows/catalog_suggest_test.go
```

Expected: no output from either (other files of the package may already be listed by `gofmt -l ./internal/flows`; they are not yours).

Write `C:/disposable/easydrag-w1c/msg-d1.txt`:

```text
feat(flows): curated next steps per category in the node catalog

NodeTypeInfo gains "suggest": the type ids the quick-add offers first
after a step of that type. The lists are curated per category (and one
for the generic tool steps), at most five, never the step's own type,
only types the registry holds (the block catalog has fewer), and none
for a step without outputs.

Tests: every listed type exists, is not a trigger and has an input; every
category has a list and every step with outputs gets one; the JSON field
is present exactly when it is set; the block catalog stays inside its own
registry.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add internal/flows/catalog.go internal/flows/catalog_suggest.go internal/flows/catalog_suggest_test.go && git commit -F C:/disposable/easydrag-w1c/msg-d1.txt && git status --short | head -3
```

Expected: one commit with three files; no other change in the worktree.

---

### Task D.2: The quick-add header

**Files:**
- Create: `scripts/test-easydrag-quickadd.mjs` (the Node checks of D.2 to D.4), `C:/disposable/easydrag-w1c/quickadd-run.mjs` (a helper outside the repository)
- Modify: `ui/js/desktop/apps/easydrag-palette.js:354-415, 493` (helpers before `openQuickAdd`, the markup, `position()`, the call order), `ui/css/desktop-app-easydrag.css` (`.ed-quick-head`, after line 927), `scripts/test-easydrag.mjs` (one call after the kit's, line 1144)
- Locale: `ui/lang/easydrag/*.json` through `plans/2026-11-16-easydrag-w1c/keys/d-2.json`

The two new texts (en / de; the other 14 locales are in the key file):

| Key | en | de |
|---|---|---|
| `easydrag.ui.quick_head_after` | Steps after: {name} | Schritte nach: {name} |
| `easydrag.ui.quick_head_insert` | Insert after: {name} | Einfügen nach: {name} |

The canvas text reuses `easydrag.ui.quick_title` ("Add step" / "Schritt hinzufügen") and the trigger text `easydrag.ui.empty_action` ("Choose a trigger" / "Auslöser wählen"); the voice guide's ledger words hold (Schritt, Auslöser). The keys sort into the files by the helper (no `_after`).

- [ ] **Step 1: Verify the code you are about to change**

```
cd W && grep -n "function openQuickAdd\|function closeQuickAdd\|const h = 380\|position();$" ui/js/desktop/apps/easydrag-palette.js
cd W && grep -rn "ED.palette.openQuickAdd\|palette.openQuickAdd\|'quick-add'" ui/js --include=*.js | grep -v vendor | cut -c1-150
cd W && grep -n "easydrag-kit.mjs" scripts/test-easydrag.mjs
cd W && ls plans/2026-11-16-easydrag-w1c/keys/d-2.json
```

Expected: `function openQuickAdd` at 358, `function closeQuickAdd` at about 499, `const h = 380;` once, `position();` once; callers `easydrag-editor.js:286` and `:579`, `easydrag-interact.js:150, 392, 454, 692, 706`, `easydrag-canvas.js:872`, `easydrag-dialogs.js:119`; one `easydrag-kit.mjs` line in `test-easydrag.mjs`, and the key file. If a caller passes a field the table of "The design in one page" does not know, read it before you go on.

- [ ] **Step 2: The helper for the inner loop**

Write `C:/disposable/easydrag-w1c/quickadd-run.mjs` with the Write tool. It runs only `scripts/test-easydrag-quickadd.mjs`, in about a second; the full `node scripts/test-easydrag.mjs` stays the gate:

```js
// Runs only the quick-add checks (scripts/test-easydrag-quickadd.mjs) in about a second, for the inner loop of Part D.
//   cd <worktree> && node C:/disposable/easydrag-w1c/quickadd-run.mjs
// It takes miniDom from scripts/test-easydrag.mjs, so it needs no copy of the stub. The full run stays the gate.
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
const root = path.resolve(process.argv[2] || process.cwd());
const src = fs.readFileSync(path.join(root, 'scripts', 'test-easydrag.mjs'), 'utf8');
const start = src.indexOf('function miniDom() {');
const end = src.indexOf('// modalHarness loads core');
const miniDom = new Function(src.slice(start, end) + '\nreturn miniDom;')();
let failures = 0;
const check = (name, cond, detail) => { if (cond) console.log('ok   ' + name); else { failures++; console.log('FAIL ' + name + (detail ? ' - ' + detail : '')); } };
const eq = (name, got, want) => check(name, JSON.stringify(got) === JSON.stringify(want), 'got ' + JSON.stringify(got) + ' want ' + JSON.stringify(want));
const guardAsync = async (name, fn) => { try { await fn(); } catch (err) { failures++; console.log('FAIL ' + name + ' threw: ' + (err && err.stack || err)); } };
const t = (key, params) => key.replace('easydrag.ui.', '') + (params ? ':' + Object.values(params).join(',') : '');
const apps = path.join(root, 'ui', 'js', 'desktop', 'apps');
const mod = await import(pathToFileURL(path.join(root, 'scripts', 'test-easydrag-quickadd.mjs')).href);
await mod.run({ apps, miniDom, t, check, eq, guardAsync });
console.log(failures ? failures + ' failure(s)' : 'quickadd checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 3: Write the failing test**

Write `W/scripts/test-easydrag-quickadd.mjs` with the Write tool. It runs the real palette, model and geometry on the `miniDom` stub, with a small canvas stub; D.3 and D.4 add their checks to the same file:

```js
// W1c Part D: the quick-add. D.2 checks its header text for every entry path, D.3 the suggestions,
// D.4 "Show all" and the scoped palette. It runs on the miniDom stub of test-easydrag.mjs, which calls
// run(env); alone this file only has to import. The palette and the model are the real modules; the
// canvas is a small stub (the quick-add reads clientToWorld, nodeRect and the canvas element).
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const MODULES = ['easydrag-core.js', 'easydrag-format.js', 'easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-palette.js'];
const TRG = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const GAMMA = 'n_cccccccc';
const AFTER_KEY = 'aurago.easydrag.after';

// docOf is the flow start -> alpha -> beta, plus an unlabelled step gamma (it goes by its type's label).
const docOf = () => ({
    schema: 1, name: 'Quick',
    nodes: [
        { id: TRG, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
        { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: {}, settings: {} },
        { id: B, key: 'beta', type: 'ai.step', label: 'Beta', position: { x: 600, y: 0 }, params: {}, settings: {} },
        { id: GAMMA, key: 'gamma', type: 'web.search', label: '', position: { x: 600, y: 300 }, params: {}, settings: {} }
    ],
    edges: [
        { id: 'e1', source: { node: TRG, port: 'out' }, target: { node: A, port: 'in' } },
        { id: 'e2', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }
    ]
});

const infoOf = (type, label, category, extra) => Object.assign({ type, version: 1, label, category, icon: 'search', inputs: ['in'], outputs: ['out'], params: [], availability: { state: 'available' } }, extra || {});
// The catalog the checks use: web.search and the manual trigger carry curated suggestions the way the
// server sends them (the trigger's list has six entries, one more than the quick-add shows).
const catalogOf = () => [
    infoOf('trigger.manual', 'Start', 'trigger', { trigger: true, inputs: [], suggest: ['ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'notify.email'] }),
    infoOf('ai.step', 'KI-Schritt', 'ai'),
    infoOf('web.search', 'Websuche', 'web', { suggest: ['notify.email', 'ai.step'] }),
    infoOf('http.request', 'HTTP-Anfrage', 'web'),
    infoOf('notify.telegram', 'Telegram', 'notify'),
    infoOf('notify.email', 'E-Mail', 'notify', { availability: { state: 'needs_setup', config_section: 'email' } }),
    infoOf('logic.if', 'Wenn', 'logic', { outputs: ['true', 'false'] }),
    infoOf('logic.stop', 'Stopp', 'logic', { outputs: [] })
];
const CATEGORIES = [{ id: 'trigger', label: 'Trigger' }, { id: 'ai', label: 'KI' }, { id: 'web', label: 'Web' }, { id: 'notify', label: 'Benachrichtigen' }, { id: 'logic', label: 'Logik' }];

// load runs the real palette, model and geometry on miniDom. h.open(req) opens the quick-add, h.types(quick)
// lists its entries, h.said holds the announcements, h.runTimers() fires the pending timeouts.
function load(env) {
    const { apps, miniDom, t } = env;
    const dom = miniDom();
    const proto = dom.El.prototype;
    // The search fields are read at render: an input answers with its typed value, else its attribute.
    Object.defineProperty(proto, 'value', {
        configurable: true,
        get() { return this.typed !== undefined ? this.typed : (this.attrs.get('value') || ''); },
        set(v) { this.typed = String(v); }
    });
    proto.scrollIntoView = function () {};
    proto.select = function () {};
    proto.removeAttribute = function (name) { this.attrs.delete(name); };
    const docListeners = {};
    Object.assign(dom.document, {
        addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); },
        removeEventListener(type, fn) { const list = docListeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
    });
    const store = new Map();
    const timers = [];
    const logged = [];
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document,
        console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
        localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
        setTimeout: fn => { timers.push(fn); return timers.length; }, clearTimeout() {},
        requestAnimationFrame: fn => { fn(); return 1; },
        matchMedia: () => ({ matches: false }), CSS: { escape: value => String(value) }
    });
    for (const file of MODULES) vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), box, { filename: file });
    const ED = box.window.EasyDrag;
    const G = ED.geometry;
    const list = catalogOf();
    const types = new Map(list.map(info => [info.type, info]));
    const model = ED.model.create(docOf(), { types });
    const said = [];
    const root = new dom.El('div', { class: 'ed-editor' });
    const canvasEl = root.appendChild(new dom.El('div', { class: 'ed-canvas' }));
    const canvas = {
        el: canvasEl,
        nodeRect: id => { const n = model.node(id); return n ? G.nodeRect(n.position, 1) : null; },
        clientToWorld: (x, y) => ({ x, y }),
        centerOn() {}, size: () => ({ w: 1000, h: 600 })
    };
    const ed = {
        t, esc: ED.core.esc, windowId: 'w1', readonly: false, runView: null, ctx: { notify() {} },
        selection: new Set(), view: { x: 0, y: 0, zoom: 1 }, bus: ED.core.emitter(), catalog: { types, list, categories: CATEGORIES },
        model, root, announce: text => said.push(text), quickAdd: null
    };
    return {
        ED, ed, canvas, model, dom, root, store, said, logged,
        open: req => ED.palette.openQuickAdd(ed, canvas, req),
        types: quick => quick.el.querySelectorAll('[data-ed-index]').map(n => n.getAttribute('data-ed-type')),
        runTimers() { while (timers.length) timers.shift()(); }
    };
}

async function headerChecks(env) {
    const { eq, check, guardAsync } = env;

    await guardAsync('w1c D.2 quick-add header', async () => {
        const h = load(env);
        // head opens a quick-add and returns the dialog's accessible name and its header row text.
        const head = req => {
            const quick = h.open(req);
            const out = [quick.el.getAttribute('aria-label'), quick.el.querySelector('[data-ed-quick-head]').textContent, quick.el.children[0].attrs.has('data-ed-quick-head')];
            quick.close();
            return out;
        };
        const same = text => [text, text, true];
        eq('w1c D.2 a step\'s "+", a card tool, Ctrl+K and a wire pulled from a port head "Steps after: <step>"',
            [head({ x: 10, y: 10, from: { node: A, port: 'out' } }), head({ x: 10, y: 10, from: { node: TRG, port: 'out' } })],
            [same('quick_head_after:Alpha'), same('quick_head_after:Start')]);
        eq('w1c D.2 a step without a label goes by its type\'s label', head({ x: 10, y: 10, from: { node: GAMMA, port: 'out' } }), same('quick_head_after:Websuche'));
        eq('w1c D.2 Insert on a wire heads "Insert after: <the wire\'s source step>"', head({ x: 10, y: 10, edge: 'e2' }), same('quick_head_insert:Alpha'));
        eq('w1c D.2 the empty canvas and the canvas menu head "Add step", the trigger button "Choose a trigger"',
            [head({ x: 10, y: 10 }), head({ x: 10, y: 10, filter: 'trigger' })], [same('quick_title'), same('empty_action')]);
        eq('w1c D.2 a source that is gone falls back to the plain heading',
            [head({ x: 10, y: 10, from: { node: 'n_gone0000', port: 'out' } }), head({ x: 10, y: 10, edge: 'e_gone' })], [same('quick_title'), same('quick_title')]);

        // A label with markup stays text: the attribute is escaped, the row is set as text.
        h.model.setLabel(A, 'A<b>&"x');
        const quick = h.open({ x: 10, y: 10, from: { node: A, port: 'out' } });
        eq('w1c D.2 the step name is escaped in the accessible name and plain text in the row',
            [/[<>"]/.test(quick.el.getAttribute('aria-label')), quick.el.querySelector('[data-ed-quick-head]').textContent], [false, 'quick_head_after:A<b>&"x']);
        quick.close();
        eq('w1c D.2 the header checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.2 quick-add position uses the real height', async () => {
        const h = load(env);
        // The editor is 1000 x 300 px; a popover that is 200 px tall must fit below y = 280 at 92 px
        // (300 - 200 - 8), where the fixed 380 px of before pushed it above the editor.
        h.root.getBoundingClientRect = () => ({ left: 0, top: 0, right: 1000, bottom: 300, width: 1000, height: 300 });
        Object.defineProperty(h.dom.El.prototype, 'offsetHeight', { configurable: true, get() { return this.className.includes('ed-quick') ? 200 : 0; } });
        const quick = h.open({ x: 100, y: 280 });
        check('w1c D.2 position() clamps with the measured height', /,92px\)$/.test(quick.el.style.transform), quick.el.style.transform);
        quick.close();
        delete h.dom.El.prototype.offsetHeight;
        const fallback = h.open({ x: 100, y: 280 });
        check('w1c D.2 without a measured height the popover keeps the 380 px it had', /,-88px\)$/.test(fallback.el.style.transform), fallback.el.style.transform);
        fallback.close();
    });
}

export async function run(env) {
    await headerChecks(env);
}
```

Hook it into the main runner (group `d2-hook`):

<!-- edits:d2-hook -->

**Edit** `scripts/test-easydrag.mjs`

Old:

```text
await (await import('./test-easydrag-kit.mjs')).run({ apps, miniDom, eq });
```

New:

```text
await (await import('./test-easydrag-kit.mjs')).run({ apps, miniDom, eq });
// ── W1c D: the quick-add (test-easydrag-quickadd.mjs) ──
await (await import('./test-easydrag-quickadd.mjs')).run({ apps, miniDom, t, check, eq, guardAsync });
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d2-hook
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs
```

Expected: `FAIL w1c D.2 quick-add header threw: TypeError: Cannot read properties of null (reading 'textContent')` (there is no header row yet), `FAIL w1c D.2 position() clamps with the measured height - translate(100px,-88px)` (the fixed 380 px), and `2 failure(s)`.

- [ ] **Step 4: Locale keys**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/d-2.json
cd W && git diff --stat -- ui/lang | tail -1 && grep -c "quick_head_" ui/lang/easydrag/de.json
```

Expected: `plans/2026-11-16-easydrag-w1c/keys/d-2.json: 2 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`; `16 files changed, 32 insertions(+)`; `2`.

- [ ] **Step 5: Implement the header**

Groups `d2-palette` (the palette) and `d2-css` (the style). The first edit adds the helpers `stepName`, `sourceStep` and `headerText` (D.3 and D.4 use them again), the second the header row and the accessible name, the third and fourth the measured height:

<!-- edits:d2-palette -->

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
    // openQuickAdd shows the quick-add popover at client point (x, y).
    // req: {x, y, from?: {node, port}, edge?: edgeId, filter?: "trigger", restoreFocus?: () => void}
```

New:

```text
    // stepName is the name a step goes by in a text: its label, else the label of its type.
    function stepName(ed, nodeId) {
        const node = ed.model.node(nodeId);
        if (!node) return '';
        const info = ed.catalog.types.get(node.type);
        return node.label || (info && info.label) || node.type;
    }

    // sourceStep is the step a quick-add request goes after: the step it was opened from, or the source
    // of the wire it inserts into. It is null when the request names none, or when that step is gone.
    function sourceStep(ed, req) {
        if (req.edge) {
            const wire = ed.model.edge(req.edge);
            return (wire && ed.model.node(wire.source.node)) || null;
        }
        return (req.from && ed.model.node(req.from.node)) || null;
    }

    // headerText is the title row of the quick-add; the dialog's accessible name is the same text.
    function headerText(ed, req) {
        const source = sourceStep(ed, req);
        if (source) return ed.t(req.edge ? 'easydrag.ui.quick_head_insert' : 'easydrag.ui.quick_head_after', { name: stepName(ed, source.id) });
        return ed.t(req.filter === 'trigger' ? 'easydrag.ui.empty_action' : 'easydrag.ui.quick_title');
    }

    // openQuickAdd shows the quick-add popover at client point (x, y).
    // req: {x, y, from?: {node, port}, edge?: edgeId, filter?: "trigger", restoreFocus?: () => void}
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        const el = core.el('<div class="ed-quick" role="dialog" aria-label="' + esc(t('easydrag.ui.quick_title')) + '">' +
            '<div class="ed-search">' + core.icon('search') + '<input type="text" class="ed-quick-search" role="combobox" aria-expanded="true" aria-autocomplete="list" placeholder="' +
            esc(req.filter === 'trigger' ? t('easydrag.ui.quick_search_trigger') : t('easydrag.ui.quick_search')) + '" enterkeyhint="go" autocomplete="off"></div>' +
            '<ul class="ed-quick-list" role="listbox"></ul></div>');
        ed.root.appendChild(el);
```

New:

```text
        const head = headerText(ed, req);
        const el = core.el('<div class="ed-quick" role="dialog" aria-label="' + esc(head) + '">' +
            '<div class="ed-quick-head" data-ed-quick-head></div>' +
            '<div class="ed-search">' + core.icon('search') + '<input type="text" class="ed-quick-search" role="combobox" aria-expanded="true" aria-autocomplete="list" placeholder="' +
            esc(req.filter === 'trigger' ? t('easydrag.ui.quick_search_trigger') : t('easydrag.ui.quick_search')) + '" enterkeyhint="go" autocomplete="off"></div>' +
            '<ul class="ed-quick-list" role="listbox"></ul></div>');
        el.querySelector('[data-ed-quick-head]').textContent = head;
        ed.root.appendChild(el);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            const h = 380;
```

New:

```text
            const h = Math.min(380, el.offsetHeight || 380);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        position();
        render();
        input.focus({ preventScroll: true });
```

New:

```text
        render();
        position();
        input.focus({ preventScroll: true });
```

<!-- /edits -->

<!-- edits:d2-css -->

**Edit** `ui/css/desktop-app-easydrag.css`

Old:

```text
.ed-quick > .ed-search > .ed-icon { left: 20px; }
```

New:

```text
.ed-quick > .ed-search > .ed-icon { left: 20px; }
.ed-quick-head { flex: none; padding: 12px 14px 0; font-size: var(--ed-font-label); font-weight: 700; color: var(--ed-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ed-quick > .ed-search { flex: none; }
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d2-palette
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d2-css
cd W && grep -n "quick_title" ui/js/desktop/apps/easydrag-palette.js; grep -n "const h = 380" ui/js/desktop/apps/easydrag-palette.js; grep -c "quick-head" ui/css/desktop-app-easydrag.css
```

Expected: `applied 4 edit(s) of group d2-palette in 1 file(s).`, `applied 1 edit(s) of group d2-css in 1 file(s).`; `quick_title` is read once, in `headerText` (the old `aria-label` literal is gone); no `const h = 380` line (the new line is `Math.min(380, …)`); `1` CSS line.

- [ ] **Step 6: Run the checks and see them pass**

```
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs | tail -4
```

Expected: nine `ok   w1c D.2 …` lines and `quickadd checks passed`.

- [ ] **Step 7: Gates**

```
cd W && node scripts/test-easydrag.mjs > D:/tmp/ed.txt 2>&1; grep -c '^FAIL' D:/tmp/ed.txt; tail -1 D:/tmp/ed.txt
cd W && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short 2>&1 | tail -3
```

Expected: `0` FAIL lines and `all EasyDrag checks passed` (about 100-200 s); `PASS render baseline (unchanged)` and `exit` 0 (note the wall time, about 15-25 s); `ok  	aurago/ui` (the static ui tests include `TestEasyDragUIKeysExistInAllLocales`, the voice lints and the module list; the browser tests skip without the environment variable). Existing quick-add checks (`c1d05 quick-add …`, `a1023b U2 Insert on the wire …`) need **no change**: the header is a new first child, and without a source or a curated list the entry order is the one they assert.

- [ ] **Step 8: Commit**

Write `C:/disposable/easydrag-w1c/msg-d2.txt`:

```text
feat(easydrag): the quick-add has a header that names what it adds after

A real title row at the top of the quick-add: "Steps after: <step>" from
a step's "+", a card tool, Ctrl+K or a pulled wire; "Insert after: <step>"
on a wire (the wire's source step); "Add step" on the canvas; "Choose a
trigger" from the empty-canvas button. The dialog's accessible name is the
same text. The popover position uses the measured height. Texts in all 16
locales (key file d-2). Node checks for every entry path, the escaping of
a step name and the position clamp.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add scripts/test-easydrag-quickadd.mjs scripts/test-easydrag.mjs ui/js/desktop/apps/easydrag-palette.js ui/css/desktop-app-easydrag.css ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-w1c/msg-d2.txt && git status --short | head -3
```

Expected: one commit with 20 files (the new check file, `test-easydrag.mjs`, the palette, the CSS and 16 locale files; `git show --stat HEAD | tail -1` prints `20 files changed`); nothing else changed. The runner under `C:/disposable/easydrag-w1c/` is not part of the repository.

---

### Task D.3: History and suggestions

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-palette.js:7-8` (constants), `:59-88` (`place`), `:354-406` (`suggestedFor`, the list order, the group), `scripts/test-easydrag-quickadd.mjs`
- Locale: `ui/lang/easydrag/*.json` through `plans/2026-11-16-easydrag-w1c/keys/d-3.json`

| Key | en | de |
|---|---|---|
| `easydrag.ui.quick_suggested` | Suggestions | Vorschläge |

- [ ] **Step 1: Verify**

```
cd W && git log --oneline -3 && grep -n "const recent = \|const rest = \|rememberType(type);\|function place(" ui/js/desktop/apps/easydrag-palette.js
cd W && grep -rn "aurago.easydrag.after" ui scripts --include=*.js --include=*.mjs --include=*.go | head -3
```

Expected: the D.2 commit on top; `const recent`, `const rest`, one `rememberType(type);` and `function place(` once each; nothing uses `aurago.easydrag.after` yet (if something does, another session started this feature: stop and tell the coordinator).

- [ ] **Step 2: Write the failing checks**

Group `d3-test` adds `suggestionChecks` to `scripts/test-easydrag-quickadd.mjs` (the catalog of its `load()` lists curated suggestions for the manual trigger, six of them, and for `web.search`, the e-mail step first, which is not set up):

<!-- edits:d3-test -->

**Edit** `scripts/test-easydrag-quickadd.mjs`

Old:

```text
export async function run(env) {
    await headerChecks(env);
}
```

New:

```text
export async function run(env) {
    await headerChecks(env);
    await suggestionChecks(env);
}
```

**Append** `scripts/test-easydrag-quickadd.mjs`

```text
// suggestionChecks: the entries the quick-add offers first (D.3). The catalog of load() lists curated
// suggestions for the manual trigger (six, one over the limit) and for web.search (the e-mail step first,
// which is not set up).
async function suggestionChecks(env) {
    const { eq, check, guardAsync } = env;
    const FROM_ALPHA = { x: 10, y: 10, from: { node: A, port: 'out' } };

    await guardAsync('w1c D.3 quick-add suggestions', async () => {
        const h = load(env);
        const marked = quick => quick.el.querySelectorAll('[data-ed-suggested]').map(n => n.getAttribute('data-ed-type'));
        const groups = quick => quick.el.querySelectorAll('[data-ed-quick-suggested]').length;
        const listed = (req, history) => {
            if (history !== undefined) h.store.set(AFTER_KEY, JSON.stringify(history)); else h.store.delete(AFTER_KEY);
            const quick = h.open(req);
            const out = { all: h.types(quick), marked: marked(quick), groups: groups(quick), greyed: quick.el.querySelectorAll('.is-unavailable').map(n => n.getAttribute('data-ed-type')) };
            quick.close();
            return out;
        };

        const curated = listed(FROM_ALPHA);
        eq('w1c D.3 without a history the curated list of the source comes first, a type that is not set up greyed, the rest follows without it',
            [curated.all, curated.marked, curated.groups, curated.greyed],
            [['notify.email', 'ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'logic.stop'], ['notify.email', 'ai.step'], 1, ['notify.email']]);

        const history = listed(FROM_ALPHA, { 'web.search': { 'notify.telegram': 3, 'logic.if': 1, 'ai.step': 1 } });
        eq('w1c D.3 the own history comes before the curated list, highest count first, and no type appears twice',
            [history.all, history.marked], [['notify.telegram', 'logic.if', 'ai.step', 'notify.email', 'web.search', 'http.request', 'logic.stop'], ['notify.telegram', 'logic.if', 'ai.step', 'notify.email']]);

        const capped = listed({ x: 10, y: 10, from: { node: TRG, port: 'out' } });
        eq('w1c D.3 at most five are suggested; the sixth curated type is listed with the rest',
            [capped.all, capped.marked.length], [['ai.step', 'web.search', 'http.request', 'notify.telegram', 'logic.if', 'notify.email', 'logic.stop'], 5]);

        const mixed = listed({ x: 10, y: 10, from: { node: TRG, port: 'out' } }, { 'trigger.manual': { 'trigger.manual': 9, 'gone.type': 5, 'logic.if': 2 } });
        eq('w1c D.3 history entries that are triggers, gone from the catalog or not candidates are skipped',
            mixed.marked, ['logic.if', 'ai.step', 'web.search', 'http.request', 'notify.telegram']);

        const damaged = ['x', '[1,2]', 'null', '{"web.search":5}', '{"web.search":{"ai.step":"many","gone":-2}}'].map(raw => {
            h.store.set(AFTER_KEY, raw);
            const quick = h.open(FROM_ALPHA);
            const out = marked(quick);
            quick.close();
            return out;
        });
        eq('w1c D.3 a damaged history changes nothing', damaged, damaged.map(() => ['notify.email', 'ai.step']));

        h.store.delete(AFTER_KEY);
        h.store.set('aurago.easydrag.recent', JSON.stringify(['logic.stop', 'ai.step']));
        const recent = listed(FROM_ALPHA);
        eq('w1c D.3 the recently used steps follow the suggestions, without the ones already suggested',
            recent.all, ['notify.email', 'ai.step', 'logic.stop', 'web.search', 'http.request', 'notify.telegram', 'logic.if']);
        h.store.delete('aurago.easydrag.recent');

        const wire = listed({ x: 10, y: 10, edge: 'e2' });
        eq('w1c D.3 Insert on a wire suggests after the wire\'s source step, only steps that have an output',
            [wire.marked, wire.all.includes('logic.stop')], [['notify.email', 'ai.step'], false]);

        const bare = listed({ x: 10, y: 10 });
        const trigger = listed({ x: 10, y: 10, filter: 'trigger' });
        eq('w1c D.3 the empty canvas and the trigger button have no source and no suggestions',
            [bare.groups, bare.marked, trigger.groups, trigger.all], [0, [], 0, ['trigger.manual']]);

        const quick = h.open(FROM_ALPHA);
        const search = quick.el.querySelector('.ed-quick-search');
        search.value = 'Telegram';
        search.fire('input');
        eq('w1c D.3 a search lists matches only, without a suggestions group',
            [h.types(quick), quick.el.querySelectorAll('[data-ed-quick-suggested]').length, marked(quick)], [['notify.telegram'], 0, []]);
        quick.close();
        eq('w1c D.3 the suggestion checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.3 history of what was added after what', async () => {
        const h = load(env);
        const book = () => JSON.parse(h.store.get(AFTER_KEY) || 'null');
        const pick = (req, type) => {
            const quick = h.open(req);
            const search = quick.el.querySelector('.ed-quick-search');
            search.value = type;
            search.fire('input');
            search.fire('keydown', { key: 'Enter' });
        };
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'Telegram');
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'Telegram');
        pick({ x: 10, y: 10, from: { node: A, port: 'out' } }, 'HTTP');
        eq('w1c D.3 a step added after a source is counted under the source\'s type, highest first',
            book(), { 'web.search': { 'notify.telegram': 2, 'http.request': 1 } });

        pick({ x: 10, y: 10, edge: 'e1' }, 'Wenn');
        eq('w1c D.3 a step inserted on a wire counts after the wire\'s source step',
            book()['trigger.manual'], { 'logic.if': 1 });

        const before = JSON.stringify(book());
        pick({ x: 10, y: 10 }, 'Wenn');
        h.ED.palette.place(h.ed, 'logic.stop', { x: 900, y: 0 }, { edge: 'e2' });
        h.ED.palette.place(h.ed, 'trigger.manual', { x: 900, y: 200 }, { from: { node: A, port: 'out' } });
        eq('w1c D.3 a step added on the empty canvas, beside a wire it could not cut, or a trigger counts nothing', JSON.stringify(book()), before);

        for (let i = 0; i < 3; i++) pick({ x: 10, y: 10, from: { node: B, port: 'out' } }, 'KI-Schritt');
        eq('w1c D.3 the history is per source type', Object.keys(book()), ['web.search', 'trigger.manual', 'ai.step']);
        check('w1c D.3 the book keeps only objects of counts', Object.values(book()).every(row => Object.values(row).every(n => Number.isInteger(n) && n > 0)));
        eq('w1c D.3 the history checks log no errors', h.logged, []);
    });
}
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d3-test
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs | grep -c "^FAIL"
```

Expected: `9` FAIL lines (seven listing checks, the first history check, and the one that throws because no history was written) and a last line `9 failure(s)`; the D.2 lines stay `ok`.

- [ ] **Step 3: Locale key**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/d-3.json
```

Expected: `…/d-3.json: 1 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`.

- [ ] **Step 4: Implement**

Group `d3-palette`: the constants, `afterBook` and `rememberAfter`, the recording in `place()`, `suggestedFor`, and the new order and group heading in `openQuickAdd`:

<!-- edits:d3-palette -->

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
    const OPEN_KEY = 'aurago.easydrag.palette.open';
```

New:

```text
    const OPEN_KEY = 'aurago.easydrag.palette.open';
    const AFTER_KEY = 'aurago.easydrag.after'; // the quick-add's own history: {sourceType: {addedType: count}}
    const MAX_SUGGESTED = 5;
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
    // place adds a node of type at a world position, optionally connected or inserted.
```

New:

```text
    // afterBook reads the history of what was added after what; anything that is not an object reads as empty.
    function afterBook() {
        const book = ED.core.storage.get(AFTER_KEY, {});
        return book && typeof book === 'object' && !Array.isArray(book) ? book : {};
    }

    // rememberAfter counts a step of type added after a step of sourceType (8 types per source, 40 sources).
    function rememberAfter(sourceType, type) {
        const book = afterBook();
        const row = book[sourceType] && typeof book[sourceType] === 'object' ? book[sourceType] : {};
        row[type] = (Number(row[type]) || 0) + 1;
        delete book[sourceType];
        book[sourceType] = Object.fromEntries(Object.entries(row).sort((a, b) => b[1] - a[1]).slice(0, 8));
        const sources = Object.keys(book);
        sources.slice(0, Math.max(0, sources.length - 40)).forEach(key => { delete book[key]; });
        ED.core.storage.set(AFTER_KEY, book);
    }

    // place adds a node of type at a world position, optionally connected or inserted.
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        let id = null;
        if (opts && opts.edge && ed.model.edge(opts.edge) && (info.inputs || []).length) {
            id = ed.model.insertOnEdge(opts.edge, type, pos);
        }
        // insertOnEdge refuses a step without outputs (it would cut the wire off): that step is added beside the wire.
        if (!id && opts && opts.from && !info.trigger) {
            id = ed.model.addNode(type, pos, null, opts.from);
        } else if (!id) {
            id = ed.model.addNode(type, pos);
        }
        if (id) {
            ed.selection.clear();
```

New:

```text
        let id = null;
        let after = null; // the step the new one follows, for the quick-add's history
        if (opts && opts.edge && ed.model.edge(opts.edge) && (info.inputs || []).length) {
            after = ed.model.node(ed.model.edge(opts.edge).source.node);
            id = ed.model.insertOnEdge(opts.edge, type, pos);
        }
        // insertOnEdge refuses a step without outputs (it would cut the wire off): that step is added beside the wire.
        if (!id && opts && opts.from && !info.trigger) {
            id = ed.model.addNode(type, pos, null, opts.from);
            after = id ? ed.model.node(opts.from.node) : null;
        } else if (!id) {
            id = ed.model.addNode(type, pos);
            after = null;
        }
        if (id) {
            if (after) rememberAfter(after.type, type);
            ed.selection.clear();
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
    // openQuickAdd shows the quick-add popover at client point (x, y).
    // req: {x, y, from?: {node, port}, edge?: edgeId, filter?: "trigger", restoreFocus?: () => void}
```

New:

```text
    // suggestedFor lists the entries the quick-add offers first after a step of sourceType, at most five and
    // only among the candidates: the types the user added after it most often on this device, then the
    // catalog's curated list for that step (info.suggest). No entry is listed twice.
    function suggestedFor(ed, sourceType, candidates) {
        const out = [];
        const add = type => {
            const info = candidates.find(i => i.type === type);
            if (info && !out.includes(info) && out.length < MAX_SUGGESTED) out.push(info);
        };
        const row = afterBook()[sourceType];
        const counts = row && typeof row === 'object' ? row : {};
        Object.keys(counts).filter(type => Number(counts[type]) > 0).sort((a, b) => Number(counts[b]) - Number(counts[a])).forEach(add);
        const source = ed.catalog.types.get(sourceType);
        ((source && Array.isArray(source.suggest)) ? source.suggest : []).forEach(add);
        return out;
    }

    // openQuickAdd shows the quick-add popover at client point (x, y).
    // req: {x, y, from?: {node, port}, edge?: edgeId, filter?: "trigger", restoreFocus?: () => void}
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        const recent = core.storage.get(RECENT_KEY, []).map(type => candidates.find(i => i.type === type)).filter(Boolean);
```

New:

```text
        const source = sourceStep(ed, req);
        const suggested = source ? suggestedFor(ed, source.type, candidates) : [];
        const recent = core.storage.get(RECENT_KEY, []).map(type => candidates.find(i => i.type === type)).filter(Boolean).filter(i => !suggested.includes(i));
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
                const rest = candidates.filter(i => !recent.includes(i));
                items = recent.concat(rest);
```

New:

```text
                const rest = candidates.filter(i => !suggested.includes(i) && !recent.includes(i));
                items = suggested.concat(recent, rest);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
                const cat = !q.trim() && idx < recent.length ? 'recent' : (q.trim() ? '' : info.category);
                if (cat && cat !== lastCat) {
                    const label = cat === 'recent' ? t('easydrag.ui.quick_recent') : ((ed.catalog.categories.find(c => c.id === cat) || {}).label || cat);
                    head = '<li class="ed-quick-group" role="presentation">' + esc(label) + '</li>';
                    lastCat = cat;
                }
                return head + itemMarkup(core, t, esc, info, 'li').replace('<li class="ed-palette-item', '<li id="' + listId + '-' + idx + '" aria-selected="' + (idx === active) + '" data-ed-index="' + idx + '" class="ed-palette-item' + (idx === active ? ' is-active' : ''));
```

New:

```text
                const searching = !!q.trim();
                const isSuggested = !searching && idx < suggested.length;
                const cat = searching ? '' : (isSuggested ? 'suggested' : (idx < suggested.length + recent.length ? 'recent' : info.category));
                if (cat && cat !== lastCat) {
                    const label = cat === 'suggested' ? t('easydrag.ui.quick_suggested') : cat === 'recent' ? t('easydrag.ui.quick_recent') : ((ed.catalog.categories.find(c => c.id === cat) || {}).label || cat);
                    head = '<li class="ed-quick-group" role="presentation"' + (cat === 'suggested' ? ' data-ed-quick-suggested' : '') + '>' + esc(label) + '</li>';
                    lastCat = cat;
                }
                return head + itemMarkup(core, t, esc, info, 'li').replace('<li class="ed-palette-item', '<li id="' + listId + '-' + idx + '" aria-selected="' + (idx === active) + '" data-ed-index="' + idx + '"' + (isSuggested ? ' data-ed-suggested' : '') + ' class="ed-palette-item' + (idx === active ? ' is-active' : ''));
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d3-palette
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs | tail -3
```

Expected: `applied 7 edit(s) of group d3-palette in 1 file(s).`; then `ok   w1c D.3 the history checks log no errors` and `quickadd checks passed` (25 `ok` lines in all: 9 of D.2, 16 of D.3).

- [ ] **Step 5: Gates**

```
cd W && node scripts/test-easydrag.mjs > D:/tmp/ed.txt 2>&1; grep -c '^FAIL' D:/tmp/ed.txt; tail -1 D:/tmp/ed.txt
cd W && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short 2>&1 | tail -3
```

Expected: `0`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok  	aurago/ui`.

- [ ] **Step 6: Commit**

Write `C:/disposable/easydrag-w1c/msg-d3.txt`:

```text
feat(easydrag): the quick-add suggests your own history, then the curated list

After a step the quick-add lists up to five "Suggestions" first: the steps
you added after that kind of step most often on this device
(aurago.easydrag.after, counted by place() for a step added after a source
or inserted on a wire), then the catalog's curated list, no duplicates,
only steps the request can add. A step that is not set up stays in the
list, greyed. The recent group and the categories follow without the
suggested ones; a search lists matches only. A damaged history reads as
empty. Text in all 16 locales (key file d-3).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add scripts/test-easydrag-quickadd.mjs ui/js/desktop/apps/easydrag-palette.js ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-w1c/msg-d3.txt && git status --short | head -3
```

Expected: one commit, 18 files (`git show --stat HEAD | tail -1`).

---

### Task D.4: "Show all" and the scoped palette

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-palette.js:92-96` (`placeStep` before `requestInsert`), `:98-352` (`createPanel`: banner, scope, `addByClick`, drop, `setOpen`, listeners, `focusSearch`), `:358-497` (`openQuickAdd`: the link, `choose`, `showAll`, keys), `ui/css/desktop-app-easydrag.css` (`.ed-quick-foot`, `.ed-palette-scope*`), `scripts/test-easydrag-quickadd.mjs`
- Locale: `ui/lang/easydrag/*.json` through `plans/2026-11-16-easydrag-w1c/keys/d-4.json`

| Key | en | de |
|---|---|---|
| `easydrag.ui.quick_all` | Show all | Alle anzeigen |
| `easydrag.ui.palette_scope` | Adds after {name} | Fügt nach {name} ein |

The scope banner's Cancel reuses `easydrag.ui.cancel` ("Cancel" / "Abbrechen").

- [ ] **Step 1: Verify**

```
cd W && git log --oneline -3 && grep -n "function addByClick\|function choose\|focusSearch()\|function setOpen\|function requestInsert\|const G = ED.geometry" ui/js/desktop/apps/easydrag-palette.js
cd W && grep -rn "focusSearch\|palette-scope" ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-interact.js | cut -c1-140
```

Expected: the D.3 commit on top; each function once; `const G = ED.geometry` in `place`, `createPanel` and `openQuickAdd`; `focusSearch` is called only at `easydrag-editor.js:290` (the palette's return object still offers it, so that call keeps working); `palette-scope` nowhere. The editor is not edited: the quick-add and the panel talk over the existing bus.

- [ ] **Step 2: Write the failing checks**

Group `d4-test` adds `scopeChecks` (it adds the real palette panel to `load()`'s editor; the Escape, Cancel, Enter, source-gone, read-only, dispose and keyboard cases and the "same spot as the quick-add" comparison for a pulled wire and for an inserted wire):

<!-- edits:d4-test -->

**Edit** `scripts/test-easydrag-quickadd.mjs`

Old:

```text
    await suggestionChecks(env);
}
```

New:

```text
    await suggestionChecks(env);
    await scopeChecks(env);
}
```

**Append** `scripts/test-easydrag-quickadd.mjs`

```text
// scopeChecks: "Show all" and the scoped palette (D.4). panelOf adds the real palette panel to load()'s editor.
function panelOf(h) {
    const panel = h.ED.palette.createPanel(h.ed, h.canvas);
    h.root.appendChild(panel.el);
    const banner = panel.el.querySelector('[data-ed-palette-scope]');
    return {
        panel, banner,
        search: panel.el.querySelector('.ed-palette-search'),
        item: type => panel.el.querySelectorAll('[data-ed-type]').find(n => n.getAttribute('data-ed-type') === type),
        scoped: () => !banner.attrs.has('hidden'),
        text: () => banner.querySelector('.ed-palette-scope-text').textContent,
        // choose presses Enter on a palette entry, as a keyboard user does; a click ends in the same function.
        choose(type) { this.item(type).fire('keydown', { key: 'Enter' }); },
        // showAll opens the quick-add for req and clicks its "Show all" link.
        showAll(req) {
            const quick = h.open(req);
            quick.el.querySelector('[data-ed-quick-all]').fire('click');
            return quick;
        }
    };
}

async function scopeChecks(env) {
    const { eq, check, guardAsync } = env;
    const FROM_ALPHA = { x: 10, y: 10, from: { node: A, port: 'out' } };
    const added = h => h.model.doc.nodes.filter(n => ![TRG, A, B, GAMMA].includes(n.id));

    await guardAsync('w1c D.4 Show all hands the source to the palette', async () => {
        const h = load(env);
        const p = panelOf(h);
        const quick = p.showAll(FROM_ALPHA);
        eq('w1c D.4 the link closes the quick-add and focuses the palette search, not the canvas',
            [h.ed.quickAdd, h.root.querySelectorAll('.ed-quick').length, h.dom.document.activeElement === p.search, h.dom.document.activeElement === h.canvas.el], [null, 0, true, false]);
        eq('w1c D.4 the palette shows "Adds after <step>" with a Cancel button', [p.scoped(), p.text(), !!p.banner.querySelector('[data-ed-palette-scope-cancel]')], [true, 'palette_scope:Alpha', true]);
        check('w1c D.4 the quick-add is gone for good (no handler left that would close it twice)', quick.el.parentNode === null);

        p.choose('http.request');
        const first = added(h)[0];
        eq('w1c D.4 the next step chosen in the palette goes after the source and ends the scope',
            [added(h).length, first.type, h.model.incoming(first.id).map(e => e.source.node), p.scoped(), h.model.node(Array.from(h.ed.selection)[0]).id === first.id], [1, 'http.request', [A], false, true]);
        h.ed.selection.clear();
        p.choose('notify.telegram');
        const second = added(h)[1];
        eq('w1c D.4 the step after that is a plain palette add again (no selection, no source)', [added(h).length, h.model.incoming(second.id).length], [2, 0]);
        eq('w1c D.4 the scoped add counts in the history like a quick-add', JSON.parse(h.store.get(AFTER_KEY)), { 'web.search': { 'http.request': 1 } });
        eq('w1c D.4 the scope checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 the palette places the step where the quick-add would', async () => {
        const req = { x: 900, y: 120, from: { node: A, port: 'out' } };
        const viaQuick = load(env);
        const q = viaQuick.open(req);
        const search = q.el.querySelector('.ed-quick-search');
        search.value = 'HTTP';
        search.fire('input');
        search.fire('keydown', { key: 'Enter' });
        const viaPalette = load(env);
        const p = panelOf(viaPalette);
        p.showAll(req);
        p.choose('http.request');
        const a = added(viaQuick)[0].position;
        const b = added(viaPalette)[0].position;
        eq('w1c D.4 a wire pulled from a port and dropped far away puts the step at the drop point either way', [a, b.x === a.x && b.y === a.y, a.x > 600], [a, true, true]);

        const wireQuick = load(env);
        const wq = wireQuick.open({ x: 10, y: 10, edge: 'e2' });
        const wqSearch = wq.el.querySelector('.ed-quick-search');
        wqSearch.value = 'HTTP';
        wqSearch.fire('input');
        wqSearch.fire('keydown', { key: 'Enter' });
        const wirePalette = load(env);
        const wp = panelOf(wirePalette);
        wp.showAll({ x: 10, y: 10, edge: 'e2' });
        eq('w1c D.4 the scope of an inserted wire says the wire\'s source step', wp.text(), 'palette_scope:Alpha');
        wp.choose('http.request');
        const ends = h => { const n = added(h)[0]; return [h.model.incoming(B).map(e => e.source.node)[0] === n.id, h.model.incoming(n.id).map(e => e.source.node), h.model.doc.edges.some(e => e.id === 'e2'), n.position]; };
        eq('w1c D.4 a wire scope inserts between the two steps exactly as the quick-add does', [ends(wirePalette), JSON.stringify(ends(wirePalette)) === JSON.stringify(ends(wireQuick))], [[true, [A], false, ends(wirePalette)[3]], true]);
    });

    await guardAsync('w1c D.4 ending the scope', async () => {
        const h = load(env);
        const p = panelOf(h);
        p.showAll(FROM_ALPHA);
        p.banner.querySelector('[data-ed-palette-scope-cancel]').fire('click');
        eq('w1c D.4 Cancel ends the scope and leaves the focus in the search', [p.scoped(), h.dom.document.activeElement === p.search], [false, true]);
        p.choose('http.request');
        eq('w1c D.4 a step chosen after Cancel is not connected', [added(h).length, h.model.incoming(added(h)[0].id).length], [1, 0]);

        p.showAll(FROM_ALPHA);
        const unscoped = p.panel.el.fire('keydown', { key: 'Escape' });
        eq('w1c D.4 Escape ends the scope, is taken (the editor does not see it) and puts the focus on the canvas',
            [p.scoped(), unscoped.stopped, unscoped.defaultPrevented, h.dom.document.activeElement === h.canvas.el], [false, true, true, true]);
        const plain = p.panel.el.fire('keydown', { key: 'Escape' });
        eq('w1c D.4 without a scope Escape is left to the editor', [plain.stopped, plain.defaultPrevented], [false, false]);

        p.showAll(FROM_ALPHA);
        p.panel.setOpen(false);
        eq('w1c D.4 hiding the palette ends the scope', p.scoped(), false);

        p.showAll(FROM_ALPHA);
        h.model.removeNodes([A]);
        const count = h.model.doc.nodes.length;
        p.choose('notify.telegram');
        eq('w1c D.4 a source that is gone by then adds nothing, says so and ends the scope',
            [h.model.doc.nodes.length === count, h.said.includes('quick_target_gone'), p.scoped()], [true, true, false]);
        eq('w1c D.4 the ending checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 Enter in the scoped search', async () => {
        const h = load(env);
        const p = panelOf(h);
        const type = text => { p.search.value = text; p.search.fire('input'); h.runTimers(); };
        type('Telegram');
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.4 without a scope Enter in the search adds nothing, as before', added(h).length, 0);
        p.showAll(FROM_ALPHA);
        p.search.value = '';
        p.search.fire('keydown', { key: 'Enter' });
        eq('w1c D.4 with a scope but no query Enter adds nothing', [added(h).length, p.scoped()], [0, true]);
        type('Telegram');
        p.search.fire('keydown', { key: 'Enter' });
        const step = added(h)[0];
        eq('w1c D.4 with a scope Enter adds the first match after the source', [step.type, h.model.incoming(step.id).map(e => e.source.node), p.scoped()], ['notify.telegram', [A], false]);
    });

    await guardAsync('w1c D.4 entries without a source and guards', async () => {
        const h = load(env);
        const p = panelOf(h);
        p.showAll({ x: 10, y: 10 });
        eq('w1c D.4 from the empty canvas Show all opens the palette with the search focused and no scope', [p.scoped(), h.dom.document.activeElement === p.search, h.ed.quickAdd], [false, true, null]);
        p.showAll({ x: 10, y: 10, filter: 'trigger' });
        eq('w1c D.4 the trigger button\'s link does the same', [p.scoped(), h.dom.document.activeElement === p.search], [false, true]);

        h.ed.readonly = true;
        h.ed.bus.emit('palette-scope', { from: { node: A, port: 'out' }, edge: undefined, name: 'Alpha', world: () => ({ x: 0, y: 0 }) });
        eq('w1c D.4 a read-only editor takes no scope', p.scoped(), false);
        h.ed.readonly = false;

        p.panel.dispose();
        h.ed.bus.emit('palette-scope', { from: { node: A, port: 'out' }, name: 'Alpha', world: () => ({ x: 0, y: 0 }) });
        eq('w1c D.4 a disposed palette no longer listens', p.scoped(), false);
        eq('w1c D.4 the guard checks log no errors', h.logged, []);
    });

    await guardAsync('w1c D.4 keyboard path of the quick-add', async () => {
        const h = load(env);
        const p = panelOf(h);
        const quick = h.open(FROM_ALPHA);
        const search = quick.el.querySelector('.ed-quick-search');
        const link = quick.el.querySelector('[data-ed-quick-all]');
        const tab = search.fire('keydown', { key: 'Tab' });
        eq('w1c D.4 Tab in the search moves to Show all and keeps the quick-add open', [h.dom.document.activeElement === link, !!h.ed.quickAdd, tab.defaultPrevented], [true, true, true]);
        link.fire('keydown', { key: 'Tab', shiftKey: true });
        eq('w1c D.4 Shift+Tab on the link goes back to the search', [h.dom.document.activeElement === search, !!h.ed.quickAdd], [true, true]);
        search.fire('keydown', { key: 'Tab' });
        link.fire('keydown', { key: 'Tab' });
        eq('w1c D.4 Tab on the link closes the quick-add and gives the focus back to the canvas', [h.ed.quickAdd, h.dom.document.activeElement === h.canvas.el], [null, true]);
        const again = h.open(FROM_ALPHA);
        again.el.querySelector('.ed-quick-search').fire('keydown', { key: 'Tab', shiftKey: true });
        eq('w1c D.4 Shift+Tab in the search and Escape close it as before', [h.ed.quickAdd, p.scoped()], [null, false]);
        const third = h.open(FROM_ALPHA);
        third.el.querySelector('[data-ed-quick-all]').fire('keydown', { key: 'Escape' });
        eq('w1c D.4 Escape on the link closes it too', h.ed.quickAdd, null);
        eq('w1c D.4 the keyboard checks log no errors', h.logged, []);
    });
}
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d4-test
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs | grep "^FAIL" | cut -c1-120
```

Expected: six `FAIL w1c D.4 … threw: TypeError: Cannot read properties of null (reading 'fire')` lines (no link yet), and a last line `6 failure(s)`.

- [ ] **Step 3: Locale keys**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-16-easydrag-w1c/keys/d-4.json
```

Expected: `…/d-4.json: 2 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`.

- [ ] **Step 4: Implement**

Group `d4-palette` (13 edits) and `d4-css`. The first edit adds `placeStep`, the shared placement of the quick-add and the scoped palette (the body of the old `choose`, with `scope.world()` instead of `canvas.clientToWorld(req.x, req.y)`, and the free-spot rule for a step that cannot go on a wire); `choose` becomes a short call to it:

<!-- edits:d4-palette -->

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
    // requestInsert asks the editor for the quick-add popover that puts a step into a connection. The
```

New:

```text
    // placeStep adds a step of type where a quick-add request puts it: after scope.from, on the wire scope.edge,
    // or at the point scope.world() names. The quick-add and the palette's scope both call it, so a step lands in
    // the same spot either way. It returns {gone: true} when the step or wire is gone by now, else {id} (null: refused).
    function placeStep(ed, canvas, scope, type) {
        const G = ED.geometry;
        if ((scope.from && !ed.model.node(scope.from.node)) || (scope.edge && !ed.model.edge(scope.edge))) return { gone: true };
        let at;
        if (scope.from) {
            const src = ed.model.node(scope.from.node);
            const outs = ed.model.outputs(src);
            const port = G.portPoint(src.position, 'out', Math.max(0, outs.indexOf(scope.from.port)), outs.length);
            const dropped = scope.world();
            at = dropped.x > port.x + 40 ? { x: dropped.x, y: dropped.y - G.NODE_H / 2 } : { x: src.position.x + G.NODE_W + 96, y: port.y - G.NODE_H / 2 };
        } else if (scope.edge) {
            const e = ed.model.edge(scope.edge);
            const src = ed.model.node(e.source.node);
            const dst = ed.model.node(e.target.node);
            at = { x: (src.position.x + dst.position.x) / 2, y: (src.position.y + dst.position.y) / 2 + 24 };
        } else {
            const p = scope.world();
            at = { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 };
        }
        const info = ed.catalog.types.get(type) || {};
        const onWire = !!scope.edge && (info.inputs || []).length > 0 && (info.outputs || []).length > 0; // a wire takes only steps with both
        const rects = ed.model.doc.nodes.map(n => canvas.nodeRect(n.id));
        return { id: place(ed, type, onWire ? at : G.freeSpot(at, rects), { from: scope.from, edge: scope.edge }) };
    }

    // requestInsert asks the editor for the quick-add popover that puts a step into a connection. The
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            '<div class="ed-palette-list"></div>' +
```

New:

```text
            '<div class="ed-palette-scope" data-ed-palette-scope role="status" hidden><span class="ed-palette-scope-text"></span>' +
            '<span class="ed-palette-scope-dot" aria-hidden="true">' + String.fromCodePoint(0xB7) + '</span>' +
            '<button type="button" class="ed-btn ed-btn--ghost" data-ed-palette-scope-cancel>' + esc(t('easydrag.ui.cancel')) + '</button></div>' +
            '<div class="ed-palette-list"></div>' +
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        const input = el.querySelector('.ed-palette-search');

        function groups() {
```

New:

```text
        const input = el.querySelector('.ed-palette-search');

        // scope is where the next step chosen here goes ({from?, edge?, name, world}); "Show all" of the
        // quick-add sets it. One added step, Cancel, Escape and hiding the panel end it.
        const scopeEl = el.querySelector('[data-ed-palette-scope]');
        let scope = null;
        function setScope(next) {
            scope = next;
            scopeEl.toggleAttribute('hidden', !next);
            scopeEl.querySelector('.ed-palette-scope-text').textContent = next ? t('easydrag.ui.palette_scope', { name: next.name }) : '';
        }
        function endScope() { if (scope) setScope(null); }

        function groups() {
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            if (!open && el.contains(document.activeElement)) canvas.el.focus({ preventScroll: true });
            el.classList.toggle('is-collapsed', !open);
```

New:

```text
            if (!open && el.contains(document.activeElement)) canvas.el.focus({ preventScroll: true });
            if (!open) endScope();
            el.classList.toggle('is-collapsed', !open);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
                place(ed, type, { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 }, { edge: overEdge });
                closeOverlay();
```

New:

```text
                if (place(ed, type, { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 }, { edge: overEdge })) endScope();
                closeOverlay();
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        function addByClick(type) {
            const info = ed.catalog.types.get(type);
```

New:

```text
        function addByClick(type) {
            const info = ed.catalog.types.get(type);
            if (scope) {
                // One step ends the scope, also when its source is gone by now (nothing is added then).
                const there = scope;
                endScope();
                const result = placeStep(ed, canvas, there, type);
                closeOverlay();
                if (result.gone) core.say(ed, t('easydrag.ui.quick_target_gone'));
                else if (result.id) canvas.centerOn(result.id, { animate: true });
                return;
            }
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        render();
        const savedOpen = core.storage.get(OPEN_KEY, null);
```

New:

```text
        function focusSearch() { setOpen(true); input.focus(); input.select(); }

        // "Show all" of the quick-add hands over its source (or null: no source, only the search).
        bag.add(ed.bus.on('palette-scope', next => {
            if (ed.readonly || ed.runView) return;
            setScope(next || null);
            focusSearch();
        }));
        bag.listen(el.querySelector('[data-ed-palette-scope-cancel]'), 'click', () => { endScope(); focusSearch(); });
        bag.listen(el, 'keydown', event => {
            if (event.key !== 'Escape' || !scope) return;
            event.preventDefault();
            event.stopPropagation();
            endScope();
            canvas.el.focus({ preventScroll: true });
        });
        // With a scope, Enter in the search adds the first match, so the search that Show all focused is enough.
        bag.listen(input, 'keydown', event => {
            if (event.key !== 'Enter' || !scope || event.isComposing || !input.value.trim()) return;
            const first = list.querySelector('[data-ed-type]');
            if (first) { event.preventDefault(); addByClick(first.dataset.edType); }
        });

        render();
        const savedOpen = core.storage.get(OPEN_KEY, null);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            focusSearch() { setOpen(true); input.focus(); input.select(); },
```

New:

```text
            focusSearch,
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        const core = ED.core;
        const G = ED.geometry;
        const { t, esc } = ed;
        if (ed.readonly || ed.runView) return null;
        closeQuickAdd(ed);
```

New:

```text
        const core = ED.core;
        const { t, esc } = ed;
        if (ed.readonly || ed.runView) return null;
        closeQuickAdd(ed);
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            '<ul class="ed-quick-list" role="listbox"></ul></div>');
        el.querySelector('[data-ed-quick-head]').textContent = head;
```

New:

```text
            '<ul class="ed-quick-list" role="listbox"></ul>' +
            '<div class="ed-quick-foot"><button type="button" class="ed-btn ed-btn--ghost" data-ed-quick-all>' + esc(t('easydrag.ui.quick_all')) + '</button></div></div>');
        el.querySelector('[data-ed-quick-head]').textContent = head;
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
        const listEl = el.querySelector('ul');
```

New:

```text
        const listEl = el.querySelector('ul');
        const allBtn = el.querySelector('[data-ed-quick-all]');
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            const info = items[idx];
            if (!info) return;
            // The wire or the step quick-add was opened for can be gone by now (an undo, a delete
            // elsewhere): nothing is added, the popover closes and says why.
            if ((req.from && !ed.model.node(req.from.node)) || (req.edge && !ed.model.edge(req.edge))) {
                close();
                giveFocusBack();
                ED.core.say(ed, ed.t('easydrag.ui.quick_target_gone'));
                return;
            }
            let at;
            if (req.from) {
                const src = ed.model.node(req.from.node);
                const outs = ed.model.outputs(src);
                const port = G.portPoint(src.position, 'out', Math.max(0, outs.indexOf(req.from.port)), outs.length);
                const dropped = canvas.clientToWorld(req.x, req.y);
                at = dropped.x > port.x + 40 ? { x: dropped.x, y: dropped.y - G.NODE_H / 2 } : { x: src.position.x + G.NODE_W + 96, y: port.y - G.NODE_H / 2 };
            } else if (req.edge) {
                const e = ed.model.edge(req.edge);
                const src = ed.model.node(e.source.node);
                const dst = ed.model.node(e.target.node);
                at = { x: (src.position.x + dst.position.x) / 2, y: (src.position.y + dst.position.y) / 2 + 24 };
            } else {
                const p = canvas.clientToWorld(req.x, req.y);
                at = { x: p.x - G.NODE_W / 2, y: p.y - G.NODE_H / 2 };
            }
            const rects = ed.model.doc.nodes.map(n => canvas.nodeRect(n.id));
            const spot = req.edge ? at : G.freeSpot(at, rects);
            const id = place(ed, info.type, spot, { from: req.from, edge: req.edge });
            close();
            if (id) giveFocusBack();
        }
```

New:

```text
            const info = items[idx];
            if (!info) return;
            const result = placeStep(ed, canvas, { from: req.from, edge: req.edge, world: () => canvas.clientToWorld(req.x, req.y) }, info.type);
            close();
            // The wire or the step quick-add was opened for can be gone by now (an undo, a delete
            // elsewhere): nothing is added, the popover closes and says why.
            if (result.gone) {
                giveFocusBack();
                ED.core.say(ed, ed.t('easydrag.ui.quick_target_gone'));
                return;
            }
            if (result.id) giveFocusBack();
        }

        // showAll closes the quick-add without taking the focus back; the palette puts its next step where this popover would.
        function showAll() {
            const source = sourceStep(ed, req);
            const point = source && !req.edge ? canvas.clientToWorld(req.x, req.y) : null;
            const scope = source ? { from: req.from, edge: req.edge, name: stepName(ed, source.id), world: () => point } : null;
            close();
            ed.bus.emit('palette-scope', scope);
        }
```

**Edit** `ui/js/desktop/apps/easydrag-palette.js`

Old:

```text
            else if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); close(); giveFocusBack(); }
            event.stopPropagation();
        });
```

New:

```text
            else if (event.key === 'Tab' && !event.shiftKey) { event.preventDefault(); allBtn.focus(); }
            else if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); close(); giveFocusBack(); }
            event.stopPropagation();
        });
        // Tab leads from the search to "Show all", Shift+Tab goes back; Tab past it and Escape close.
        allBtn.addEventListener('click', showAll);
        allBtn.addEventListener('keydown', (event) => {
            if (event.key === 'Tab' && event.shiftKey) { event.preventDefault(); input.focus({ preventScroll: true }); }
            else if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); close(); giveFocusBack(); }
            event.stopPropagation();
        });
```

<!-- /edits -->

<!-- edits:d4-css -->

**Edit** `ui/css/desktop-app-easydrag.css`

Old:

```text
.ed-quick-empty { padding: 18px; text-align: center; color: var(--ed-muted); list-style: none; }
```

New:

```text
.ed-quick-empty { padding: 18px; text-align: center; color: var(--ed-muted); list-style: none; }
.ed-quick-foot { flex: none; display: flex; justify-content: flex-end; padding: 4px 8px 8px; border-top: 1px solid var(--ed-border); }
```

**Edit** `ui/css/desktop-app-easydrag.css`

Old:

```text
.ed-palette-list { flex: 1; overflow: auto; padding: 0 8px 8px; }
```

New:

```text
.ed-palette-list { flex: 1; overflow: auto; padding: 0 8px 8px; }
.ed-palette-scope { display: flex; align-items: center; gap: 6px; margin: 0 10px 8px; padding: 4px 4px 4px 10px; border-radius: var(--ed-radius-md); background: color-mix(in srgb, var(--ed-accent) 14%, transparent); font-size: var(--ed-font-label); color: var(--ed-text); }
.ed-palette-scope[hidden] { display: none; }
.ed-palette-scope-text { min-width: 0; overflow-wrap: anywhere; }
.ed-palette-scope-dot { flex: none; color: var(--ed-muted); }
.ed-palette-scope .ed-btn { flex: none; margin-left: auto; }
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d4-palette
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-d-quickadd.md d4-css
cd W && grep -n "const G = ED.geometry" ui/js/desktop/apps/easydrag-palette.js
cd W && wc -l ui/js/desktop/apps/easydrag-palette.js
```

Expected: `applied 13 edit(s) of group d4-palette in 1 file(s).` and `applied 2 edit(s) of group d4-css in 1 file(s).`; `const G = ED.geometry` is declared in `place`, `placeStep` and `createPanel` (three lines) and no longer in `openQuickAdd`, whose `choose` no longer places by hand; `653` lines (a few lines off is fine; above 700 read "Size of easydrag-palette.js" above).

- [ ] **Step 5: Run the checks and see them pass**

```
cd W && node C:/disposable/easydrag-w1c/quickadd-run.mjs | tail -3 && node C:/disposable/easydrag-w1c/quickadd-run.mjs | grep -c "^ok"
```

Expected: `quickadd checks passed` and `56` `ok` lines (9 + 16 + 31).

- [ ] **Step 6: Gates**

```
cd W && node scripts/test-easydrag.mjs > D:/tmp/ed.txt 2>&1; grep -c '^FAIL' D:/tmp/ed.txt; tail -1 D:/tmp/ed.txt
cd W && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short 2>&1 | tail -3
```

Expected: `0`, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok  	aurago/ui`. The CSS lines are covered by the existing static CSS lints (`TestShippedWebAssetsLoadNothingExternal` stays green).

- [ ] **Step 7: Commit**

Write `C:/disposable/easydrag-w1c/msg-d4.txt`:

```text
feat(easydrag): Show all opens the full palette scoped to the quick-add's source

A link at the foot of the quick-add closes it (focus stays out of the
canvas) and opens the palette with its search focused. When the quick-add
had a source step or wire, the palette shows "Adds after <step>" with a
Cancel button, and the next step chosen there (click, Enter on an entry,
Enter in the search) goes through the same placeStep() the quick-add uses,
so it lands exactly where the quick-add would have put it. Cancel, Escape,
hiding the palette or one added step end the scope. Tab in the quick-add's
search now leads to the link; Tab past it and Escape close as before. The
quick-add and the panel talk over a new "palette-scope" bus event; the
editor is not touched. Texts in all 16 locales (key file d-4).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add scripts/test-easydrag-quickadd.mjs ui/js/desktop/apps/easydrag-palette.js ui/css/desktop-app-easydrag.css ui/lang/easydrag/*.json && git commit -F C:/disposable/easydrag-w1c/msg-d4.txt && git status --short | head -3
```

Expected: one commit, 19 files.

---

### Task D.5: The browser acceptance (roadmap E9)

**Files:**
- Create: `ui/desktop_easydrag_quickadd_browser_test.go`

The test uses real pointer events (`MustHover`, `MustClick`) with motion on (`Motion: true`, as `TestEasyDragActionsMenuClicksBrowser`): the "+" reveals itself on hover and the popover animates in. It does not edit the fixture: the catalog of `ui/testdata/easydrag-fixture.js` has no `suggest` field (Part A owns that file), so the test sets one on the editor's copy of the schedule trigger, the way `/node-types` would deliver it (the Go test of D.1 pins the server side).

- [ ] **Step 1: Verify**

```
cd W && git log --oneline -4 && grep -n "func requirePrecisionBrowserSmoke\|func startEasyDragSmokeWith\|func (s \*easyDragSmoke) \(wait\|settle\|shot\|failOnPageErrors\)" ui/*_test.go | cut -c1-120
cd W && grep -n "data-ed-port-add" ui/js/desktop/apps/easydrag-canvas.js | cut -c1-110 && grep -n "n_zeitplan\|Werktags 7 Uhr" ui/testdata/easydrag-fixture.js | head -3
```

Expected: the D.4 commit on top; all five helpers found; the port "+" button is drawn on each card; the seeded flow has the trigger `n_zeitplan` labelled "Werktags 7 Uhr" and three more steps (4 nodes, 3 wires). If Part A changed the fixture's seed, adapt the three literals (`n_zeitplan`, the label, the node count) in the test.

- [ ] **Step 2: Write the test**

Write `W/ui/desktop_easydrag_quickadd_browser_test.go` with the Write tool:

```go
package ui

import "testing"

// TestEasyDragQuickAddSuggestionsBrowser is the E9 acceptance of the quick-add (W1c Part D): from a
// trigger's "+" the header names the trigger, the suggestions come first, "Show all" opens the full
// palette scoped to that trigger, and the next step chosen there lands after it. It runs with
// motion on and uses real pointer events: the "+" button reveals itself on hover, and the
// popover animates in and out.
func TestEasyDragQuickAddSuggestionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()

	// The server sends the curated list as "suggest" on each step type (internal/flows, TestEveryStepWithOutputsGetsSuggestions);
	// the fixture catalog has none, so the test puts one on the editor's copy of the schedule trigger. The fifth entry is a
	// step that is not set up; a sixth shows that the list stops at five.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.catalog.types.get('trigger.schedule').suggest=['ai.prompt','web.search','notify.telegram','smart_home.service','http.request','logic.if'];window.__ids=ed.model.doc.nodes.map(n=>n.id);}`)

	// A real pointer: hover the trigger's card (the "+" scales up from 0.6 on hover), then click it.
	p.MustElement(`.ed-node[data-node-id="n_zeitplan"]`).MustHover()
	p.MustElement(`.ed-node[data-node-id="n_zeitplan"] [data-ed-port-add]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-quick.is-open') && document.activeElement && document.activeElement.classList.contains('ed-quick-search')`)
	s.settle()

	// The header reads "<Steps after>: <the trigger's label>" and is the dialog's accessible name.
	if got := p.MustEval(`()=>{const q=document.querySelector('.ed-quick');const head=q.querySelector('.ed-quick-head');const want=window.t('easydrag.ui.quick_head_after',{name:'Werktags 7 Uhr'});return head.textContent===want && q.getAttribute('aria-label')===want && want.includes('Werktags 7 Uhr') && !want.includes('{')?'':'header "'+head.textContent+'", name "'+q.getAttribute('aria-label')+'", want "'+want+'"';}`).Str(); got != "" {
		t.Fatalf("the quick-add header is wrong: %s", got)
	}

	// The suggestions come first: five entries, the curated order, the one that is not set up greyed, then the rest.
	if got := p.MustEval(`()=>{const q=document.querySelector('.ed-quick');const items=Array.from(q.querySelectorAll('[data-ed-index]'));const marked=items.filter(i=>i.hasAttribute('data-ed-suggested')).map(i=>i.dataset.edType);const first=items.slice(0,5).map(i=>i.dataset.edType);const group=q.querySelector('.ed-quick-list > [data-ed-quick-suggested]');const grey=items.filter(i=>i.classList.contains('is-unavailable')&&i.hasAttribute('data-ed-suggested')).map(i=>i.dataset.edType);const rest=items.slice(5).map(i=>i.dataset.edType);return JSON.stringify({marked,first,grey,group:group&&group.textContent===window.t('easydrag.ui.quick_suggested'),groupFirst:q.querySelector('.ed-quick-list').firstElementChild===group,restHasSixth:rest.includes('logic.if'),restRepeats:rest.some(x=>marked.includes(x))});}`).Str(); got != `{"marked":["ai.prompt","web.search","notify.telegram","smart_home.service","http.request"],"first":["ai.prompt","web.search","notify.telegram","smart_home.service","http.request"],"grey":["smart_home.service"],"group":true,"groupFirst":true,"restHasSixth":true,"restRepeats":false}` {
		t.Fatalf("the suggestions are wrong: %s", got)
	}
	// The popover is whole inside the editor.
	if !p.MustEval(`()=>{const q=document.querySelector('.ed-quick').getBoundingClientRect();const e=document.querySelector('.ed-editor').getBoundingClientRect();return q.left>=e.left-0.5&&q.right<=e.right+0.5&&q.top>=e.top-0.5&&q.bottom<=e.bottom+0.5;}`).Bool() {
		t.Fatal("the quick-add reaches outside the editor")
	}
	s.shot("quickadd-header-suggestions")

	// "Show all" closes the popover and opens the full palette with its search focused and the scope banner.
	p.MustElement(`.ed-quick [data-ed-quick-all]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-quick') && !!document.querySelector('[data-ed-palette-scope]:not([hidden])') && document.activeElement===document.querySelector('.ed-palette-search')`)
	if got := p.MustEval(`()=>{const b=document.querySelector('[data-ed-palette-scope]');const want=window.t('easydrag.ui.palette_scope',{name:'Werktags 7 Uhr'});return b.querySelector('.ed-palette-scope-text').textContent===want && want.includes('Werktags 7 Uhr') && !!b.querySelector('[data-ed-palette-scope-cancel]')?'':'banner "'+b.textContent+'", want "'+want+'"';}`).Str(); got != "" {
		t.Fatalf("the palette scope banner is wrong: %s", got)
	}
	s.shot("quickadd-show-all-scope")

	// The next click on a palette step places it after the trigger, exactly where the quick-add would have.
	p.MustElement(".ed-palette-search").MustInput("Telegram")
	s.wait(`()=>document.querySelectorAll('.ed-palette-item[data-ed-type="notify.telegram"]').length===1`)
	p.MustElement(`.ed-palette-item[data-ed-type="notify.telegram"]`).MustClick()
	s.wait(`()=>{const m=edFixture.editor().model;const added=m.doc.nodes.find(n=>!window.__ids.includes(n.id));return m.doc.nodes.length===5 && !!added && added.type==='notify.telegram' && m.incoming(added.id).map(e=>e.source.node).join()==='n_zeitplan' && document.querySelector('[data-ed-palette-scope]').hidden}`)
	s.settle()
	s.failOnPageErrors("quick-add suggestions")
}
```

- [ ] **Step 3: Run it**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^TestEasyDragQuickAddSuggestionsBrowser$' -count=1 -v -timeout 10m 2>&1 | tail -6
```

Run it with the Bash sandbox off. Expected: `--- PASS: TestEasyDragQuickAddSuggestionsBrowser`, `PASS`, `ok  	aurago/ui`, and two screenshots in `D:/tmp/easydrag-w1c-shots` (`quickadd-header-suggestions…png`, `quickadd-show-all-scope…png`). Open both once: the header row sits on top of the search field, the first five entries are under "Vorschläge", the Home Assistant entry is greyed, the foot has "Alle anzeigen", and in the second shot the palette shows "Fügt nach Werktags 7 Uhr ein · Abbrechen" above its list. For each failure:
  - `the quick-add header is wrong` or `the suggestions are wrong`: the message prints what the page had; compare with the Node checks of D.2 and D.3, which cover the same logic;
  - a timeout at the "+" click: the button did not become clickable after the hover; check `.ed-port-add` in the CSS for its hover rule and `pointer-events`, and click it with `p.Mouse.MustMoveTo` to its centre first;
  - `the quick-add reaches outside the editor`: `position()` is wrong for the real height; print `getBoundingClientRect` of `.ed-quick` and `.ed-editor`.

- [ ] **Step 4: Run the other browser tests that open the quick-add**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestEasyDragTouchInsertOnWire|TestEasyDragActionsMenuClicksBrowser|TestDesktopEasyDragUXBrowser|TestDesktopEasyDragBrowser)$' -count=1 -v -timeout 30m 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"
```

Expected: four `--- PASS` lines and `ok  	aurago/ui` (about 2 minutes; `TestDesktopEasyDragBrowser` and `TestDesktopEasyDragUXBrowser` are the long ones, about 50 s each). They keep passing because the fixture has no curated list (so nothing is suggested) and their Insert and Enter steps pick the first listed type, which is the same.

- [ ] **Step 5: The part's gate**

```
cd W && npm run test:easydrag:t0
```

Expected: no FAIL line. Then the coordinator's pre-landing run: the full EasyDrag browser run (about 10-17 minutes, `AURAGO_RUN_BROWSER_SMOKE=1 … go test ./ui -run '(?i)easydrag' -count=1 -timeout 40m`) after a check for leftover headless rod Chromes, and one look at the lab (zero findings). The quick-add, the palette and the new banner are covered by the lab's palette and quick-add scenes if it has them; if the lab reports a finding on `.ed-quick-foot` or `.ed-palette-scope`, fix the CSS (the cause is a token or a contrast value), not the lab.

- [ ] **Step 6: Commit**

Write `C:/disposable/easydrag-w1c/msg-d5.txt`:

```text
test(easydrag): browser acceptance of the quick-add header, suggestions and Show all

From a trigger's "+" (a real hover and click, motion on) the header reads
"Steps after: <the trigger's label>" and is the dialog's name, the five
suggestions come first with the one that is not set up greyed, the popover
stays inside the editor, and Show all opens the full palette with its
search focused and the scope banner; the next click on a palette step
places it after the trigger.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/desktop_easydrag_quickadd_browser_test.go && git commit -F C:/disposable/easydrag-w1c/msg-d5.txt && git status --short | head -3 && git log --oneline -6
```

Expected: one commit with one file; the log shows the five Part D commits (D.1 to D.5) on top of main.

---

## Coverage of the spec

| Spec item | Task |
|---|---|
| §6.2 a real header row at the top of `.ed-quick` | D.2 |
| §6.2 header text per entry (step "+", card tool, Ctrl+K, pulled wire; Insert on a wire; empty canvas and canvas menu; trigger button) | D.2 (`headerText`; the table in "The design in one page") |
| §6.2 `{name}` is the label or the type's label, escaped | D.2 (`stepName`, `textContent`, `esc` in the `aria-label`; check "escaped") |
| §6.2 the dialog's `aria-label` follows the header | D.2 |
| §6.2 the height grows by the header row, `position()` uses the real height | D.2 (`offsetHeight`, `render(); position();`; the two position checks) |
| §6.3 the group "Vorschläge", at most 5, only with a source step | D.3 |
| §6.3 own history in `localStorage['aurago.easydrag.after']`, written by `place()`, highest count first | D.3 (`rememberAfter`, `suggestedFor`) |
| §6.3 curated list per source category in the Go catalog, served with the catalog; a Go test that every listed type exists and is not a trigger | D.1 (`curatedSuggestions`, `Suggest`, `TestCuratedSuggestionsAreValid`) |
| §6.3 no entry twice, the rest follows without the suggested ones | D.3 |
| §6.3 unavailable types greyed as in the palette | D.3 (`itemMarkup` keeps `is-unavailable`; checks "greyed" and, in the browser, the Home Assistant entry) |
| §6.4 the link at the bottom; closes the quick-add without giving the focus back; opens the palette with the search focused | D.4 (`showAll`, `palette-scope`, `focusSearch`) |
| §6.4 the palette receives the source (`from` or `edge`), the banner "Fügt nach {name} ein · Abbrechen" | D.4 (`[data-ed-palette-scope]`, `setScope`) |
| §6.4 the next step (click or Enter) is placed after the source exactly as the quick-add places it | D.4 (`placeStep`, shared; the "same spot" checks for a pulled wire and for an inserted wire) |
| §6.4 Cancel, Escape or adding one step end the scope | D.4 (checks "ending the scope") |
| §6.5 Node tests: header text per entry; suggestion order; Show all scoped palette | D.2, D.3, D.4 (`scripts/test-easydrag-quickadd.mjs`) |
| §6.5 Go test: the catalog suggestion list is valid | D.1 |
| §6.5 Browser test (E9 acceptance): trigger "+", header, suggestions first, Show all opens the palette; one check with `Motion: true` | D.5 |
| §7 per commit: `go vet`, `gofmt -l`; `test-easydrag.mjs`; render baseline | every task's gates |
| §7 per part: T0 | D.5 Step 5 |
| §9 "Quick-add shows its header, suggestions first, and Show all opens the scoped palette" | D.2 to D.5 |
| §8 shared files: Part D touches only the palette, the catalog and its keys | held: no edit to `easydrag-editor.js`, the fixture or `kit.js` |

## Findings while planning

1. **The categories are eight**, not the seven the brief listed: `trigger, logic, ai, web, documents, notify, smart_home, planner` (`CategoryOrder`). `planner` has two steps and gets a list. Generic tool steps (`tool:<name>`) are registered elsewhere (not by `RegisterCatalog`, which holds 42 types) and have per-tool categories; they share one list (`tool`).
2. **`logic.stop` has no outputs**, `logic.switch` and `ai.decision` have dynamic outputs: the rule "a step gets suggestions when it has outputs or dynamic outputs" covers them; `logic.stop` gets none (a test pins it).
3. **The node-type answer is cached by registry generation** (`flowNodeTypesCache`) and the list is static, so the cache needs no change; `ETag`s change once when the field appears.
4. **`miniDom` drops text nodes**, so Node checks cannot read `<li>` or button text from markup. The header text is therefore set with `textContent` (a plain property in the stub, safe text in a browser) and checked together with the escaped `aria-label`; group headings are checked by their hook attributes and, for their text, in the browser test.
5. **The quick-add's `choose()` and the scoped palette needed one placement code.** `choose()` computed the spot from the request point; the scope has to freeze that point when "Show all" is clicked (the user may pan before choosing). `placeStep(ed, canvas, scope, type)` takes `scope.world()` for it; the quick-add passes a function that reads `canvas.clientToWorld(req.x, req.y)` at the moment of choosing, as before.
6. **The editor needs no edit.** The panel is created in `easydrag-editor.js:106` and the quick-add is opened there too; they now talk over the bus event `palette-scope` that `createPanel` registers (and disposes with its bag). Part B+C keep `easydrag-editor.js` to themselves.
7. **A static ratchet counts literal radii.** `TestEasyDragLiteralRatchets` (`ui/desktop_easydrag_ratchet_test.go`) fails on a new `border-radius: 10px` (the first scratch run failed there); the scope banner uses `var(--ed-radius-md)`. Keep the CSS of D.2 and D.4 to tokens.
8. **The existing Node and browser quick-add checks need no change, none weakened** (full run in the scratch copy: `all EasyDrag checks passed`, render baseline unchanged). Checked by reading and by the run: `scripts/test-easydrag-extra.mjs` (`c1d04` Add and the pulled wire at :236 and :345-360, `c1d05` quick-add at :1080-1111), `-extra5.mjs:884-895` (Ctrl+K opens quick-add with the first output), `-extra7.mjs:149-152` and `:349-362` (Insert on a wire picks entry 0), `test-easydrag-actions.mjs`, `-transform.mjs`, `-ux-batch5.mjs`, and the Go browser tests `desktop_easydrag_browser_test.go:68-73, 275-281`, `_touch_browser_test.go:253-255`, `_actions_browser_test.go:382-391`, `_ux_browser_test.go:186`. The cases that pick `[data-ed-index="0"]` after an Insert or a "+" run on catalogs without `suggest` fields and without history, so the first entry is the same; the new header is a new first child and none of them counts children.

## Deviations from the spec

1. **Key files: three, not one** (`d-2`, `d-3`, `d-4`), so each text lands with the commit that uses it and a rebase conflict can be repaired task by task. The texts are the ones the spec names; "Abbrechen" reuses `cancel`, "Schritt hinzufügen" `quick_title`, "Auslöser wählen" `empty_action`.
2. **The curated lists have nine rows**, not the eight the brief's examples suggest: `planner` and `tool` are added so that "every category with outputs has a list" holds for the real catalog and for generic tool steps. The step's own type is never listed after itself (after a Telegram message the list would otherwise offer Telegram again).
3. **Tab in the quick-add** now leads from the search to "Show all" (it used to close the popover). Without it a keyboard user could not reach the link, which the spec places in the popover. Shift+Tab in the search, Escape and Tab past the link close as before, and the focus goes back to the canvas as before.
4. **Enter in the palette's search adds the first match, only while a scope is active.** After "Show all" the search is focused; without this a keyboard user would have to tab to an entry. Outside a scope nothing changes (Enter in the search did nothing before).
5. **"Show all" is on every entry**, also the canvas entry and the trigger button, where it opens the palette with its search focused and no scope (the spec describes only the scoped case).
6. **A palette drag** (drop on the canvas) is not scoped: the user chooses the spot. A successful drop does end an active scope ("adding one step ends the scope").
7. **The scope banner's dot** is a separate muted span (`String.fromCodePoint(0xB7)`) between the text and the Cancel button, not part of the translated text; the text is "Adds after {name}" / "Fügt nach {name} ein".
8. **The browser test does not edit the fixture.** It sets `suggest` on the editor's copy of the schedule trigger, because the fixture file is Part A's; the server side is pinned by the Go test.
9. **`easydrag-palette.js` ends at 653 lines.** About three over the "keep it near 650" mark of the brief; no new module (see "Size of easydrag-palette.js").
10. **History limits** are my addition (8 types per source, 40 sources) so the stored value stays small; the spec only says "pairs counted".

## Decisions needed

1. **Tab leads to "Show all" (Deviation 3).** Recommended: keep; the alternative is a shortcut (for example Ctrl+Enter) that nobody would find. If the owner wants Tab to close as before, delete the Tab branch of the search's `keydown` and the link's own `keydown` handler (D.4), and "Show all" stays a pointer and touch control.
2. **Enter in the scoped search (Deviation 4).** Recommended: scoped only. A general "Enter adds the first match" is a palette behaviour change that belongs to UX-04 (Wave 4: ">" commands, synonyms).
3. **The curated lists (the table in "The design in one page").** They are a content decision. Recommended: ship them as they are; the Go test keeps any later edit valid, and the history takes over for each user after a few flows. The owner may want to re-order the `trigger` row (the first thing most flows do).
4. **Native review of the 14 translated locales** of the five new keys (the voice guide asks for it); the English and German texts are final.
5. **An `AGENTS.md` line** (`ui/js/desktop/apps/AGENTS.md` and `internal/flows/AGENTS.md`: the `palette-scope` event, the `aurago.easydrag.after` key, `NodeTypeInfo.Suggest`). Not done here because the AGENTS.md files are edited by other sessions and the main checkout keeps local, unstaged changes in them; recommended: the coordinator adds one line each after the part lands.

## What was verified while planning, and what was not

Verified on a scratch copy of main d87930dfc (`C:/disposable/easydrag-w1c-plan/scratch-d` and `scratch-d2`, the touched files converted to CRLF like the worktree):
- Every edit group applied with `apply-plan-edits.mjs` in task order, on CRLF files, each old text matching exactly once (`seq.mjs` rebuilds the copy and applies D.2, D.3 and D.4 with a red run before and a green run after each).
- The Go tests of D.1 (red by build failure, green after the edit), `go vet ./internal/flows ./cmd/flowsdoc`, `go test ./internal/flows/... ./cmd/flowsdoc` green; `gofmt -l` clean for the three files.
- `node scripts/test-easydrag.mjs`: `all EasyDrag checks passed` with the 56 new `w1c D.x` lines and no change to an existing check; `node scripts/test-easydrag-render-baseline.mjs`: `PASS render baseline (unchanged)`.
- The key files: JSON parses, 16 non-empty texts per key, placeholders equal to English, no backslash-u; `locale-apply.mjs` applies them (2 + 1 + 2 keys in 16 files).
- `go test ./ui -run '(?i)easydrag' -count=1 -short`: `ok  	aurago/ui`. It failed once on the first draft of the CSS (`TestEasyDragLiteralRatchets`, a literal `border-radius: 10px`; fixed, see Finding 7).
- The browser test of D.5 ran against a real Chrome in the scratch copy (Bash sandbox off, `GOTMPDIR='D:\tmp\go-build'`): `--- PASS: TestEasyDragQuickAddSuggestionsBrowser (5.0s)`. I looked at both screenshots and fixed two flaws they showed: the header row was squeezed to half its height by the flex column (now `flex: none` on the header, the search row and the foot), and the scope banner cut its text with an ellipsis at 280 px (now the text wraps, the Cancel button sits right).
- `go test ./internal/server -run '(?i)flow' -count=1` in the scratch copy: `ok  	aurago/internal/server	28.0s`.
- The four other browser tests of D.5 Step 4 ran green in the scratch copy: `TestEasyDragActionsMenuClicksBrowser`, `TestDesktopEasyDragBrowser`, `TestEasyDragTouchInsertOnWire`, `TestDesktopEasyDragUXBrowser` (`ok  	aurago/ui  115.6s`).

Not verified: the full EasyDrag browser run, T0 as one command, and the lab (the coordinator's landing step); the 14 machine-translated locales have had no native review; the Go `internal/tools` and `internal/server` packages were not run beyond the `flow` pattern above.
