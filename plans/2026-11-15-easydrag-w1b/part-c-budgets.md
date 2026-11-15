## Part C: Control budgets

Part C adds two Go tests and no product code. It lands first, so that W1c (header) and W1f (home) have a number to hold.

- **Shell:** Git Bash. Every command starts with `cd <worktree> && ...`.
- **Worktree:** `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets`, branch `feat/easydrag-w1b-budgets`, created from main at the start of Task C.1.
- **Baseline (Task C.1, Step 1):** the two guard tests that this part touches by adding a `*easydrag*_browser_test.go` file are green before the first edit.
- **Depends on:** nothing. W1a is complete on main. No locale work, so there are no key files in this part.
- **IMPACT:** nothing existing is changed. Both tasks create new files and new symbols (`easyDragControlBudget`, `easyDragControlBudgets`, `easyDragControlCount`, `easyDragMenuCount`, `countControls`, `countMenuEntries`, `easyDragBudgetProblems`). The one grep in C.1 Step 2 proves that none of these names exists today.
- **Go env:** every Go command is prefixed with `GOTOOLCHAIN=go1.27.4`, and `GOTMPDIR='D:\tmp\go-build'` for `./ui`.
- **Browser runs:** `AURAGO_RUN_BROWSER_SMOKE=1`, `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots`, Bash sandbox **disabled** (`dangerouslyDisableSandbox: true`). Repo-built executables cannot start Chrome inside the sandbox.

### What the code looks like today (verified 2026-11-15, main at 065b6fca8)

| Region | Root selector | Where it is built |
|---|---|---|
| Editor header | `.ed-editor > .ed-head` | `easydrag-editor.js:53-65`; the undo/redo buttons are added into `[data-ed-history-host]` by `easydrag-header.js` |
| Editor footer | `.ed-editor > .ed-foot` | `easydrag-editor.js:79-83`: `last-run` and `issues` buttons |
| "⋯" menu | `.vd-context-menu`, opened by `[data-ed-cmd="more"]` | `moreMenu`, `easydrag-editor.js:428-460`; entries are `button.vd-context-item` or, for a submenu parent (Lebendigkeit), `.vd-context-submenu > button.vd-context-item`; separators are `.vd-context-separator` (`menus-and-routing.js:424-474`) |
| Home chrome | `.ed-home`, skipping `.ed-flow-card` and `.ed-template-card` | `easydrag-home.js:88-132` (hero, search, filters, sort, view, favourites, saved-filters head, organisation head, template search and category) |
| One published flow card | the first `.ed-home .ed-flow-card` | `easydrag-home-grid.js:162-170`: select checkbox, star, active switch, "⋯" button |

- The seeded fixture flow is **already published and enabled** (`addFlow(briefing(), true, true)`, `ui/testdata/easydrag-fixture.js:185`), so no publish step is needed to get a published idle flow. Other tests publish through the UI only because they edit first.
- The window is 1440x900 by default (`newEasyDragPage`).
- The counting rule is spec §5.1. The scanner `checkTargets` (`ui/desktop_easydrag_targets_browser_test.go:68`) is **not reusable as is**: it is a size check with a fixed selector list that fails the test and returns no count. The helper in Task C.1 uses the same visibility rules (display none, visibility hidden, zero-size, `.ed-sr-only`), adds `[hidden]` and `[inert]` ancestors, the wider role list and the group rule, and does not touch the scanner file.

---

### Task C.1: The counting helper and the budget browser test

**Files:**
- Create: `ui/desktop_easydrag_budget_browser_test.go`
- Test: `go test ./ui -run TestEasyDragControlBudgetsBrowser` (browser)

- [ ] **Step 1: Create the worktree and check the baseline**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1b-budgets -b feat/easydrag-w1b-budgets main
```
Expected: `Preparing worktree (new branch 'feat/easydrag-w1b-budgets')` and `HEAD is now at <sha> ...`.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragBrowserTestsUseTheSmokeHelper|TestEasyDragAssetsLoadInDependencyOrder)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: two `--- PASS` lines and `ok  	github.com/.../ui`.

- [ ] **Step 2: Check that no name collides**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && grep -rn "easyDragControlBudget\|easyDragControlCount\|easyDragMenuCount\|countControls\|countMenuEntries\|easyDragBudgetProblems" ui/*.go
```
Expected: no output. If a name exists, rename the new symbol (and its uses in C.2) and say so in the commit body.

- [ ] **Step 3: Write the test file**

Create `ui/desktop_easydrag_budget_browser_test.go`. The ceilings in the table are the spec's estimates; Step 5 replaces them with the measured numbers.

```go
package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// easyDragControlBudget is one row of the control budget (W1b spec section 5).
//
//   - Ceiling is what the region may show today. The test fails above it and asks to lower it when the
//     count is below.
//   - Target is the number a later sub-project must reach; 0 means the region has no target.
//   - Gate names the sub-project that enforces the target ("" when there is no target).
//   - Enforced is flipped by that sub-project when it lands; from then on Ceiling equals Target.
type easyDragControlBudget struct {
	Region   string
	Ceiling  int
	Target   int
	Gate     string
	Enforced bool
}

// easyDragControlBudgets is the table. Regions are measured at 1440x900 with the standard fixture
// and a published, idle, enabled flow (see TestEasyDragControlBudgetsBrowser).
var easyDragControlBudgets = []easyDragControlBudget{
	// The editor header (.ed-head). W1c rebuilds it and must reach 8.
	{Region: "header", Ceiling: 10, Target: 8, Gate: "W1c"},
	// The editor footer (.ed-foot). No target: it must not grow.
	{Region: "footer", Ceiling: 2},
	// The entries of the header "..." menu, separators not counted. No target: it must not grow, so
	// W1c cannot reach its header target by moving controls into the menu.
	{Region: "more-menu", Ceiling: 12},
	// Every control on the start page outside the flow cards and template cards (radio groups count
	// as one, a collapsed disclosure as its head). W1f rebuilds the page and must reach 6.
	{Region: "home", Ceiling: 13, Target: 6, Gate: "W1f"},
	// One published flow card, the card element itself not counted. W1f must reach 3.
	{Region: "card", Ceiling: 4, Target: 3, Gate: "W1f"},
}

// easyDragControlCount counts the controls under a root by the rule of spec section 5.1 and lists
// them, so a changed ceiling can be reviewed. Arguments: the root selector and a selector for
// descendants to skip ("" for none). It returns JSON {count, items}; count is -1 when the root is
// missing.
//
// Counted: button, a[href], input (not hidden), select, textarea and the roles button, switch,
// checkbox, tab, link, option, menuitem and radio, and tabindex="0". Disabled controls count.
// Counted as one: a radio group (role=radiogroup, or inputs with the same name). A collapsed
// disclosure shows only its head, because its body carries [hidden]. Not counted: [hidden] and
// [inert] ancestors, display none, visibility hidden, a zero-size box, the screen-reader-only list,
// and the root itself.
const easyDragControlCount = `(rootSel,skipSel)=>{
	const root=document.querySelector(rootSel);
	if(!root)return JSON.stringify({count:-1,items:[]});
	const picks='button,a[href],input,select,textarea,[role="button"],[role="switch"],[role="checkbox"],[role="tab"],[role="link"],[role="option"],[role="menuitem"],[role="radio"],[tabindex="0"]';
	const seen=new Set();
	const items=[];
	root.querySelectorAll(picks).forEach(el=>{
		if(el.matches('input[type="hidden"]'))return;
		if(skipSel&&el.closest(skipSel))return;
		if(el.closest('[hidden],[inert],.ed-sr-only'))return;
		const cs=getComputedStyle(el),r=el.getBoundingClientRect();
		if(cs.display==='none'||cs.visibility==='hidden'||(r.width===0&&r.height===0))return;
		let key=el;
		if(el.matches('input[type="radio"][name]'))key='radio:'+el.name;
		else if(el.matches('[role="radio"]')){const g=el.closest('[role="radiogroup"]');if(g)key=g;}
		if(seen.has(key))return;
		seen.add(key);
		const label=(el.getAttribute('aria-label')||el.textContent||'').replace(/\s+/g,' ').trim().slice(0,24);
		const data=[...el.attributes].map(a=>a.name).find(n=>n.startsWith('data-ed'))||'';
		items.push(el.tagName.toLowerCase()+(data?'['+data+']':'')+(label?' "'+label+'"':''));
	});
	return JSON.stringify({count:items.length,items:items});
}`

// easyDragMenuCount counts the entries of the open desktop context menu: a button, or the parent
// button of a submenu (its entries are not counted). Separators are no entries. Disabled entries
// count.
const easyDragMenuCount = `()=>{
	const menu=document.querySelector('.vd-context-menu');
	if(!menu)return JSON.stringify({count:-1,items:[]});
	const items=[...menu.querySelectorAll(':scope > .vd-context-item, :scope > .vd-context-submenu > .vd-context-item')]
		.map(b=>(b.querySelector('.vd-context-label')||b).textContent.replace(/\s+/g,' ').trim());
	return JSON.stringify({count:items.length,items:items});
}`

type easyDragControlReading struct {
	Count int      `json:"count"`
	Items []string `json:"items"`
}

func (s *easyDragSmoke) readControls(region, script string, args ...interface{}) int {
	s.t.Helper()
	var reading easyDragControlReading
	if err := json.Unmarshal([]byte(s.page.MustEval(script, args...).Str()), &reading); err != nil {
		s.t.Fatalf("%s: read the control count: %v", region, err)
	}
	s.t.Logf("CONTROLS region=%s count=%d: %s", region, reading.Count, strings.Join(reading.Items, " | "))
	return reading.Count
}

// countControls counts the controls under root, skipping the descendants that match skip.
func (s *easyDragSmoke) countControls(region, root, skip string) int {
	s.t.Helper()
	return s.readControls(region, easyDragControlCount, root, skip)
}

// countMenuEntries counts the entries of the open context menu.
func (s *easyDragSmoke) countMenuEntries(region string) int {
	s.t.Helper()
	return s.readControls(region, easyDragMenuCount)
}

// TestEasyDragControlBudgetsBrowser counts the controls of the header, the footer, the "..." menu,
// the start page and one published flow card at 1440x900 and compares each count with its ceiling in
// easyDragControlBudgets. A count above the ceiling fails; a count below it logs "lower the ceiling".
// Every region logs one BUDGET line.
func TestEasyDragControlBudgetsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	page := s.page
	counts := map[string]int{}

	// The start page with the seeded, published and enabled flow and the three templates.
	s.wait(`()=>document.querySelectorAll('.ed-home .ed-flow-card').length===1 && document.querySelectorAll('.ed-home .ed-template-card').length===3`)
	counts["home"] = s.countControls("home", ".ed-home", ".ed-flow-card, .ed-template-card")
	counts["card"] = s.countControls("card", ".ed-home .ed-flow-card", "")

	// The editor of that flow: published, idle and switched on, so Run now is shown.
	page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.wait(`()=>{const ed=edFixture.editor();const run=document.querySelector('[data-ed-cmd="run-now"]');return !!ed && ed.flowEnabled===true && !!run && !run.hidden}`)
	counts["header"] = s.countControls("header", ".ed-editor > .ed-head", "")
	counts["footer"] = s.countControls("footer", ".ed-editor > .ed-foot", "")

	// The "..." menu: open it, count its entries, close it again.
	page.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-context-menu .vd-context-item')`)
	counts["more-menu"] = s.countMenuEntries("more-menu")
	page.MustEval(`()=>aurora.closeContextMenu()`)
	s.wait(`()=>!document.querySelector('.vd-context-menu')`)

	for _, b := range easyDragControlBudgets {
		count, ok := counts[b.Region]
		if !ok {
			t.Errorf("budget region %q is not measured by this test", b.Region)
			continue
		}
		if count <= 0 {
			t.Errorf("budget region %q: found %d controls; the region selector no longer matches", b.Region, count)
			continue
		}
		target, gate := "-", "-"
		if b.Target > 0 {
			target = strconv.Itoa(b.Target)
		}
		if b.Gate != "" {
			gate = b.Gate
		}
		t.Logf("BUDGET region=%s count=%d ceiling=%d target=%s gate=%s", b.Region, count, b.Ceiling, target, gate)
		if count > b.Ceiling {
			t.Errorf("budget region %q: %d controls, ceiling %d (target %s, gate %s). A control was added; remove it or move it out of this region", b.Region, count, b.Ceiling, target, gate)
		}
		if count < b.Ceiling {
			t.Logf("BUDGET region=%s: lower the ceiling to %d", b.Region, count)
		}
	}
	s.failOnPageErrors("control budgets")
}
```

- [ ] **Step 4: Run it and read the numbers**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && gofmt -l ui/desktop_easydrag_budget_browser_test.go
```
Expected: no output.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^TestEasyDragControlBudgetsBrowser$' -count=1 -v -timeout 10m 2>&1 | grep -E 'CONTROLS|BUDGET|^(--- |ok|FAIL)|budget region'
```
(Bash sandbox disabled.) Expected: five `CONTROLS region=... count=N: <list>` lines, five `BUDGET region=... count=N ceiling=C ...` lines, and either `--- PASS` (every count is at or below the estimate; `lower the ceiling to N` lines name the corrections) or `--- FAIL` with `budget region "x": N controls, ceiling C` (a count above the estimate). Both outcomes are expected here: the spec's numbers are estimates (header 10, footer 2, "⋯" 12, home about 13, card 4).

Read each `CONTROLS` list before trusting the number:
- **header:** expect back, name input, runs, test, publish, run now, active switch, more, undo, redo. Stop (`hidden`) and the run-view banner buttons must not be listed. If the state chip or the revision is listed as a control, it is a real control and counts.
- **footer:** expect `last-run` and `issues`.
- **more-menu:** the list is the real entry labels. `repair_mission` appears only with a Mission Control problem, so it is absent here, and "Mit E-Mail teilen" counts even when disabled.
- **home:** the hero buttons (import, pack import, approval inbox, authoring, new), the block library link, the search, the five filter radios as one, the sort select, the two view radios as one, favourites, the saved-filters head, the organisation head if present, the template search and category select, and whatever the approvals, pack, durable and credential helpers mount. If a list shows an item that does not belong to the page chrome (for example inside a banner), it is still counted; note it in the commit body.
- **card:** expect select checkbox, star, active switch, menu (4).
- If a region reports `-1` or `0`, the root selector is wrong: fix it with the table above, not by lowering a ceiling.

- [ ] **Step 5: Turn the measured numbers into the ceilings**

For every region, set `Ceiling` in `easyDragControlBudgets` to the **count the run printed**, whatever it is (higher or lower than the estimate). Targets and gates stay as in Step 3: header 8 (W1c), home 6 (W1f), card 3 (W1f); footer and "more-menu" none. `Enforced` stays false on every row. If a measured count is below its target (not expected), keep the target, set the ceiling to the count, and C.2 will fail the row; stop and raise it under "Decisions needed" instead of changing the target.

Example (edit only the numbers that differ from the printed counts):

```go
	{Region: "header", Ceiling: 10, Target: 8, Gate: "W1c"},
	{Region: "footer", Ceiling: 2},
	{Region: "more-menu", Ceiling: 12},
	{Region: "home", Ceiling: 13, Target: 6, Gate: "W1f"},
	{Region: "card", Ceiling: 4, Target: 3, Gate: "W1f"},
```

Write the measured numbers and the difference from the spec's estimates into the commit body (Step 7).

- [ ] **Step 6: Run again and see it pass without "lower" lines**

Same command as Step 4. Expected: `--- PASS: TestEasyDragControlBudgetsBrowser`, five lines `BUDGET region=<r> count=N ceiling=N target=<8|-|6|3> gate=<W1c|-|W1f>` with count equal to ceiling, and **no** `lower the ceiling` line.

Then the two guards:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_budget_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragBrowserTestsUseTheSmokeHelper|TestEasyDragControlBudgetsBrowser)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: no vet output, no gofmt output, two `--- PASS` lines (the browser test skips without the env var: `--- SKIP` is correct for that one here), `ok`.

- [ ] **Step 7: Commit**

Write `C:/disposable/easydrag-w1b/c1-commit.txt` with the Write tool (fill the measured counts for `N`):

```
test(easydrag): count the controls of the header, home and card

TestEasyDragControlBudgetsBrowser counts the visible controls of the
editor header, the editor footer, the "..." menu, the start page outside
its cards and one published flow card at 1440x900 with the standard
fixture, and compares each with its ceiling in the table
easyDragControlBudgets. Above the ceiling it fails; below it logs "lower
the ceiling to N". Every region logs a BUDGET line.

The ceilings are the measured counts: header N, footer N, "..." menu N,
home N, card N (the W1b spec estimated 10, 2, 12, about 13 and 4). The
targets are header 8 (W1c), home 6 and card 3 (W1f); footer and menu have
no target. No row is enforced yet.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && git add ui/desktop_easydrag_budget_browser_test.go && git commit -F C:/disposable/easydrag-w1b/c1-commit.txt
```
Expected: `1 file changed`.

---

### Task C.2: The table check (T0)

**Files:**
- Create: `ui/desktop_easydrag_budget_test.go`
- Test: `go test ./ui -run TestEasyDragControlBudgetTable`

- [ ] **Step 1: Write the test**

Create `ui/desktop_easydrag_budget_test.go`. `easyDragBudgetProblems` is the whole check; the test runs it on the real table and on a deliberately broken sample, so a check that stopped checking would fail.

```go
package ui

import (
	"strings"
	"testing"
)

// easyDragBudgetProblems lists what is wrong with a budget table. A well-formed table has:
//   - a region name that is unique and a ceiling above zero;
//   - a target that is not negative and not above its ceiling (a target of 0 means "none");
//   - a gate on every row with a target, and no gate on a row without one;
//   - for an enforced row, a target and a ceiling equal to it.
func easyDragBudgetProblems(rows []easyDragControlBudget) []string {
	var problems []string
	seen := map[string]bool{}
	for _, b := range rows {
		switch {
		case strings.TrimSpace(b.Region) == "":
			problems = append(problems, "a row has no region name")
			continue
		case seen[b.Region]:
			problems = append(problems, b.Region+": the region appears twice")
		}
		seen[b.Region] = true
		if b.Ceiling <= 0 {
			problems = append(problems, b.Region+": the ceiling must be above 0")
		}
		if b.Target < 0 {
			problems = append(problems, b.Region+": the target must not be negative")
		}
		if b.Target > b.Ceiling {
			problems = append(problems, b.Region+": the target is above the ceiling")
		}
		if b.Target > 0 && strings.TrimSpace(b.Gate) == "" {
			problems = append(problems, b.Region+": a row with a target must name the sub-project that enforces it")
		}
		if b.Target == 0 && b.Gate != "" {
			problems = append(problems, b.Region+": a row without a target must not name a gate")
		}
		if b.Enforced && (b.Target == 0 || b.Ceiling != b.Target) {
			problems = append(problems, b.Region+": an enforced row needs a target and a ceiling equal to it")
		}
	}
	return problems
}

// TestEasyDragControlBudgetTable is the T0 check of the control budget (W1b spec section 5.4): the
// table that TestEasyDragControlBudgetsBrowser reads is well formed, and it still names every region
// the browser test measures.
func TestEasyDragControlBudgetTable(t *testing.T) {
	for _, problem := range easyDragBudgetProblems(easyDragControlBudgets) {
		t.Error(problem)
	}
	have := map[string]bool{}
	for _, b := range easyDragControlBudgets {
		have[b.Region] = true
	}
	for _, region := range []string{"header", "footer", "more-menu", "home", "card"} {
		if !have[region] {
			t.Errorf("the table has no row for %q", region)
		}
	}

	broken := []easyDragControlBudget{
		{Region: "a", Ceiling: 5, Target: 6, Gate: "W1c"},
		{Region: "b", Ceiling: 5, Target: 3},
		{Region: "c", Ceiling: 5, Gate: "W1f"},
		{Region: "d", Ceiling: 5, Target: 3, Gate: "W1c", Enforced: true},
		{Region: "d", Ceiling: 0},
	}
	if got := len(easyDragBudgetProblems(broken)); got != 6 {
		t.Errorf("the check found %d problems in a table with 6, so it no longer checks", got)
	}
}
```

Count in the sample: a, target above ceiling (1); b, target without gate (2); c, gate without target (3); d, enforced with ceiling 5 above target 3 (4); second d, duplicate region (5) and ceiling 0 (6). Total 6.

- [ ] **Step 2: Run it and see it pass**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && gofmt -l ui/desktop_easydrag_budget_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^TestEasyDragControlBudgetTable$' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)|budget|table'
```
Expected: no gofmt output, `--- PASS: TestEasyDragControlBudgetTable`, `ok`.

- [ ] **Step 3: See it fail for the stated reason, then restore**

Temporarily change the header row in `ui/desktop_easydrag_budget_browser_test.go` to `Target: 12` and run the same command. Expected: `--- FAIL`, `header: the target is above the ceiling`. Then `Target: 8, Gate: ""` for the header: expected `header: a row with a target must name the sub-project that enforces it`. Restore the row to its C.1 content and check with:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && git diff --stat -- ui/desktop_easydrag_budget_browser_test.go
```
Expected: no output (the file is back to the C.1 commit).

- [ ] **Step 4: Gates and T0**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_budget_test.go ui/desktop_easydrag_budget_browser_test.go
```
Expected: no output.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
```
Expected: `ok  	.../ui` (browser tests skip without the env var; this proves the new T0 test is picked up by the `(?i)easydrag` filter and nothing else broke). If the worktree has `node_modules` (copy, not a junction) also run `npm run test:easydrag:t0` and expect it to end green; otherwise leave the full T0 batch run to the coordinator.

- [ ] **Step 5: Commit**

Write `C:/disposable/easydrag-w1b/c2-commit.txt` with the Write tool:

```
test(easydrag): check that the control budget table is well formed

TestEasyDragControlBudgetTable (T0, no browser) fails when a target is
above its ceiling, a row with a target names no gate, a row without a
target names one, an enforced row's ceiling differs from its target, a
region is repeated, or the table lacks a region the browser test
measures. It also runs the check on a broken sample so that the check
cannot silently stop checking.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-budgets && git add ui/desktop_easydrag_budget_test.go && git commit -F C:/disposable/easydrag-w1b/c2-commit.txt
```
Expected: `1 file changed`.

---

### Coverage

| Spec item | Task |
|---|---|
| §5.1 counting rule (visible controls, roles, radio group = 1, collapsed disclosure = head, menu = one button, hidden/inert/zero-size not counted, card itself not counted, disabled counted) | C.1 (`easyDragControlCount`, `easyDragMenuCount`) |
| §5.1 state: standard fixture, published idle flow, 1440 px | C.1 (`startEasyDragSmoke`, seeded flow, wait on `flowEnabled` and Run now) |
| §5.2 five regions and ceilings measured, not estimated | C.1 Steps 4-5 |
| §5.2 targets 8 / 6 / 3 with gates W1c / W1f / W1f; footer and "⋯" none | C.1 table |
| §5.3 table: region, ceiling, target, gate, enforced | C.1 `easyDragControlBudget` |
| §5.3 fail above, "lower the ceiling to N" below, BUDGET line every run | C.1 test body |
| §5.3 enforced means ceiling equals target | C.2 |
| §5.4 T0 table check: target <= ceiling, target rows name their gate | C.2 |
| §1 decision 3 (ratchet now, fixed targets later) | C.1 + C.2 |

### Deviations from the spec

1. **The scanner `checkTargets` is not reused.** It returns no count, is size-based with a fixed selector list, and fails the test. The helper copies its visibility rules (not its code) and adds `[hidden]` and `[inert]` ancestors, the roles of §5.1 and the group rule. Reason: reuse would have meant changing a shipped test file.
2. **"No target" is `Target: 0` and `Gate: ""`**, not a separate flag (the brief's struct has no flag). C.2 enforces the pairing.
3. **No publish step in the test.** The seeded fixture flow is already published and enabled, so the "published idle flow" needs no UI publish.
4. **The "⋯" menu is counted in the wide window**, so the phone-only "Runs" and "Test" entries (`headerNarrow()`) are not in the number. The ceiling therefore guards the wide menu only. A narrow-window row is not in the spec.
5. **Measured ceilings may differ from the spec's estimates.** C.1 Step 5 records them in the commit body; nothing in the spec or roadmap is edited.

### Decisions needed

1. **Is the block library link on the home a control the W1f budget should count?** `.ed-block-library-link` is inserted next to the toolbar when blocks are available and is counted by the rule as written. Recommendation: keep it counted. It is page chrome, and the target of 6 then forces W1f to place it deliberately.
2. **Should the "⋯" menu also be budgeted on a phone?** Recommendation: no. W1c rebuilds the header, and a second row would double the table before the numbers matter.
3. **If a measured count is already below its target** (not expected; the card has four controls today), the target must stay and the ceiling is set to the count. C.2 would then fail only if the target is above the ceiling. Recommendation: in that case raise the question to the owner instead of weakening the target.
