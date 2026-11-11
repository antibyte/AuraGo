[← Plan index](README.md) · Spec: [plans/2026-11-10-easydrag-s0-spec.md](../2026-11-10-easydrag-s0-spec.md)

## Part 4: Small defects

Spec §6 (S0.3). It has three tasks, with one commit each, in the order 4.1, 4.2, 4.3. All work happens in `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0` on `feat/easydrag-s0`, and every command runs from that folder. Line numbers are those of main 3da58db83.

**Dependency on Part 1.** Part 1 moves the test-start code and the test dialog (`storedEffects`, `confirmedEffects`, `confirmEffects`, `startTest`, `openTest` and the dialog body, runs.js :382-614) into `ui/js/desktop/apps/easydrag-run-start.js`. Tasks 4.1 and 4.3 edit that post-move file. Function names and local names (`openTest`, `o`, `real`, `trigger`, `prefilled`, `shown`, `simulation`, `select`) stay the same. The anchors below are the moved lines; runs.js line numbers are given for orientation only. If Part 1 renamed the revision getter in the `ED.testing.decorate(ed, dialog, { ... })` env, keep its name and add only what is shown.

**Commit form (every task).** Stage with `git add <paths>` and commit with `git commit -m "<subject>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"`. Git joins the two `-m` parts with a blank line. Before each commit, run `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0`. A `partial` or `truncated` result is not clean, so run it again.

**Browser tests** run only with `$env:AURAGO_RUN_BROWSER_SMOKE='1'` (`requirePrecisionBrowserSmoke`, `ui/precision_workspace_browser_test.go:13`). The fixture page loads the merged `de.json` words (`startEasyDragSmoke`, `ui/desktop_easydrag_browser_test.go:366-386`), so `t()` returns German there. If Chrome cannot start from the test binary, follow the recipe in the memory file `reference_chrome_launch_from_agent.md` (GOTMPDIR on D:).

---

### Task 4.1: Simulation is preselected, the mode has its own label, and Enter never starts a real test

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-testing.js` (:84-92 add after; :113-137 `decorate`; :248 export)
- Modify: `ui/js/desktop/apps/easydrag-run-start.js` (after Part 1; was runs.js :567-583, the decorate env plus the new Enter guard)
- Modify: `ui/desktop_easydrag_browser_test.go` (:142-143)
- Modify: `scripts/test-easydrag-extra4.mjs` (:164), plus any other test the suite run in step 9 shows that clicks Run and expects a real POST on a flow with effects
- Create: `scripts/test-easydrag-test-mode.mjs`
- Create: `ui/desktop_easydrag_test_mode_browser_test.go`
- Modify: all 16 `ui/lang/easydrag/*.json` (new key `easydrag.ui.test_mode`, after `"easydrag.ui.test_title"`, en/de :907)

- [ ] **Step 1: Impact checks from the main checkout.**
  ```
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "decorate" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
  node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "openTest" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
  ```
  - Expected callers: `decorate` is called only from openTest (runs.js:567 today) and from Node tests. `openTest` is called only from `startTest`.
  - If the result is UNKNOWN, confirm with a grep for `testing.decorate(` and `openTest(` in `ui/js` and `scripts`.
  - Warn on HIGH or CRITICAL risk.

- [ ] **Step 2: Write the failing Node test `scripts/test-easydrag-test-mode.mjs`.**
  ```js
  #!/usr/bin/env node
  // Test opens in Simulation when the steps it runs have outward effects (named or unknown);
  // flows without effects and durable flows keep the real test.
  import assert from 'node:assert/strict';
  import fs from 'node:fs';
  import vm from 'node:vm';

  const box = vm.createContext({ window: {}, console, Intl, Date, Map, Set, JSON });
  const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
  vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
  const ED = box.window.EasyDrag;

  const infos = new Map([
      ['trigger.manual', { type: 'trigger.manual', trigger: true }],
      ['notify.telegram', { type: 'notify.telegram', effects: ['sends_message'] }],
      ['logic.set', { type: 'logic.set' }],
      ['tool.mystery', { type: 'tool.mystery', risky: true }],
      ['logic.durable_wait', { type: 'logic.durable_wait' }],
      ['logic.durable_approval', { type: 'logic.durable_approval' }]
  ]);
  // edFor builds a chain n0 -> n1 -> ... of the given types.
  const edFor = types => {
      const nodes = types.map((type, i) => ({ id: 'n' + i, type, label: type, settings: {} }));
      const edges = nodes.slice(1).map((n, i) => ({ id: 'e' + i, source: { node: nodes[i].id, port: 'out' }, target: { node: n.id, port: 'in' } }));
      return { model: { doc: { nodes, edges }, info: type => infos.get(type), node: id => nodes.find(n => n.id === id) } };
  };
  const pick = (types, onlyNode, real) => ED.testing.simulationDefault(edFor(types), onlyNode || null, 'n0', real || null);

  assert.equal(pick(['trigger.manual', 'notify.telegram']), true, 'a named effect selects Simulation');
  assert.equal(pick(['trigger.manual', 'logic.set']), false, 'no effect keeps the real test');
  assert.equal(pick(['trigger.manual', 'tool.mystery']), true, 'an unknown effect selects Simulation');
  assert.equal(pick(['trigger.manual', 'logic.set', 'notify.telegram'], 'n1'), false, 'a step test counts only what it runs');
  assert.equal(pick(['trigger.manual', 'logic.set'], null, new Map([['n1', ['sends_message']]])), true, 'effects of the real settings count');
  assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_wait']), false, 'a durable wait keeps the real test');
  assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_approval']), false, 'a durable approval keeps the real test');
  console.log('All EasyDrag test-mode checks passed.');
  ```

- [ ] **Step 3: Run it.** `node scripts/test-easydrag-test-mode.mjs`. Expected failure: `TypeError: ED.testing.simulationDefault is not a function`.

- [ ] **Step 4: Write the failing browser test `ui/desktop_easydrag_test_mode_browser_test.go`.**
  ```go
  package ui

  import (
  	"testing"
  	"time"

  	"github.com/go-rod/rod/lib/input"
  	"github.com/go-rod/rod/lib/proto"
  )

  // TestDesktopEasyDragTestModeBrowser: a flow that sends a Telegram message opens Test in
  // Simulation, and the mode control is named "Testart" (the fixture speaks German). In a real
  // test, Enter on the select, on a checkbox of the dialog and on the run button starts nothing
  // and confirms nothing; Space on the run button still starts the real test.
  func TestDesktopEasyDragTestModeBrowser(t *testing.T) {
  	requirePrecisionBrowserSmoke(t)
  	s := startEasyDragSmoke(t)
  	p := s.page
  	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
  		t.Fatal(err)
  	}
  	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
  	p.MustElement(`.ed-flow-card h3`).MustClick()
  	s.wait(`()=>!!edFixture.editor() && !!document.querySelector('[data-ed-cmd="test"]')`)
  	p.MustEval(`()=>{const ed=edFixture.editor();ed.model.addNode('notify.telegram',{x:1280,y:120},{message:'Hallo'},{node:'n_pdf',port:'out'});}`)
  	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.some(n=>n.type==='notify.telegram')}`)
  	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
  	s.wait(`()=>{const m=document.querySelector('.ed-modal [data-ed-test-mode]');return !!m && m.value==='simulation' && !document.querySelector('.ed-modal .ed-simulation-fields').hidden}`)
  	p.MustEval(`()=>{if(document.activeElement && document.activeElement.matches('[data-ed-action="run"]'))throw Error('the run button has the default focus')}`)

  	// The accessible name, read from the browser's accessibility tree.
  	if err := (proto.AccessibilityEnable{}).Call(p); err != nil {
  		t.Fatal(err)
  	}
  	desc, err := p.MustElement(`.ed-modal [data-ed-test-mode]`).Describe(0, false)
  	if err != nil {
  		t.Fatal(err)
  	}
  	tree, err := proto.AccessibilityGetPartialAXTree{BackendNodeID: desc.BackendNodeID}.Call(p)
  	if err != nil {
  		t.Fatal(err)
  	}
  	name := ""
  	for _, n := range tree.Nodes {
  		if n.BackendDOMNodeID == desc.BackendNodeID && n.Name != nil {
  			name = n.Name.Value.Str()
  		}
  	}
  	if name != "Testart" {
  		t.Fatalf("test mode name = %q, want Testart", name)
  	}

  	// Real mode: Enter starts nothing and confirms nothing.
  	p.MustEval(`()=>{window.__edRequests=edFixture.state.requests.length;const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
  	for _, selector := range []string{`.ed-modal [data-ed-test-mode]`, `.ed-modal [data-ed-test-remember]`, `.ed-modal [data-ed-action="run"]`} {
  		p.MustElement(selector).MustFocus()
  		p.Keyboard.MustType(input.Enter)
  	}
  	time.Sleep(500 * time.Millisecond)
  	p.MustEval(`()=>{const posts=edFixture.state.requests.slice(window.__edRequests).filter(r=>/^POST \S+\/test$/.test(r));const c=edFixture.editor().effectsConfirmed;if(posts.length || (c instanceof Set && c.size) || !document.querySelector('.ed-modal [data-ed-action="run"]'))throw Error('Enter started a real test: '+posts.join());}`)

  	// Space on the run button still starts the real test.
  	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustFocus()
  	p.Keyboard.MustType(input.Space)
  	s.wait(`()=>edFixture.state.requests.slice(window.__edRequests).some(r=>/^POST \S+\/test$/.test(r))`)
  	s.noRawKeys("test mode")
  	s.failOnPageErrors("test mode")
  }
  ```

- [ ] **Step 5: Run it.** `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDragTestMode)' -count=1 -timeout 30m`. Expected failure: the first `s.wait` for `m.value==='simulation'` times out, because the select still reads `real`.

- [ ] **Step 6: Implement in `easydrag-testing.js`.**

  (a) Insert after `unknownEffects` (after :92, before the blank line and `parseSample`):
  ```js
      // Service.Simulate refuses a draft with a durable step (internal/flows/simulation.go,
      // ErrDurableInvalidGraph), so such a flow keeps the real test and its effect confirmation.
      const DURABLE_TYPES = new Set(['logic.durable_wait', 'logic.durable_approval']);
      let modeIds = 0;

      // simulationDefault: a test whose steps have outward effects, named (catalog or publish
      // preview, see effects) or unknown (a risky step without named effects), opens in
      // Simulation. A flow without effects, or with a durable step, opens as a real test.
      function simulationDefault(ed, onlyNode, triggerId, real) {
          if (ed.model.doc.nodes.some(n => DURABLE_TYPES.has(n.type))) return false;
          return effects(ed, onlyNode, triggerId, real).size > 0 || unknownEffects(ed, onlyNode, triggerId);
      }
  ```

  (b) In `decorate`. Before (:116-117):
  ```js
          const controller = new AbortController();
          const mode = core.el('<label class="ed-label">' + esc(t('easydrag.ui.test_title')) + '<select class="ed-input" data-ed-test-mode><option value="real">' + esc(t('easydrag.ui.real_test')) + '</option><option value="simulation">' + esc(t('easydrag.ui.simulation_title')) + '</option></select></label>');
  ```
  After:
  ```js
          const controller = new AbortController();
          // The select is named by its own label only (an embedded select would add its option text).
          const modeLabel = 'ed-test-mode-' + (++modeIds);
          const mode = core.el('<label class="ed-label"><span id="' + modeLabel + '">' + esc(t('easydrag.ui.test_mode')) + '</span><select class="ed-input" data-ed-test-mode aria-labelledby="' + modeLabel + '"><option value="real">' + esc(t('easydrag.ui.real_test')) + '</option><option value="simulation">' + esc(t('easydrag.ui.simulation_title')) + '</option></select></label>');
  ```

  (c) Before (:136-137):
  ```js
          select.addEventListener('change', syncMode);
          dialog.done.then(() => { closed = true; controller.abort(); });
  ```
  After:
  ```js
          select.addEventListener('change', syncMode);
          // A test of steps with outward effects opens in Simulation (UX-07). The scope is the
          // one openTest checks: the tested step (env.onlyNode) and the chosen trigger.
          const triggerNow = typeof env.trigger === 'function' ? env.trigger() : undefined;
          if (simulationDefault(ed, env.onlyNode, triggerNow, typeof env.real === 'function' ? env.real() : null)) { select.value = 'simulation'; syncMode(); }
          dialog.done.then(() => { closed = true; controller.abort(); });
  ```

  (d) Export, before (:248):
  ```js
      ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects };
  ```
  After:
  ```js
      ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects, simulationDefault };
  ```

- [ ] **Step 7: Implement in `easydrag-run-start.js` (post-move `openTest`).**

  (a) Pass the preview effects to decorate. Before (was runs.js:569):
  ```js
                  onlyNode: o.onlyNode, revision: baseRevision,
  ```
  After:
  ```js
                  onlyNode: o.onlyNode, revision: baseRevision, real: () => real,
  ```

  (b) Add the Enter guard after the decorate call. Insert directly before the line
  `            // showChecks shows the hint and the effects for a trigger again (another trigger, a newer`
  this code:
  ```js
              // Enter never starts a real test. core.modal has no form and only type="button"
              // buttons, so the one Enter path is a focused button's activation; while the mode is
              // real, Enter does nothing in this dialog except a new line in the sample text. A run
              // starts from a click or Space on the run button.
              dialog.el.addEventListener('keydown', event => {
                  if (event.key !== 'Enter' || (event.target && event.target.tagName === 'TEXTAREA')) return;
                  if (simulation && simulation.isSelected()) return;
                  event.preventDefault();
                  event.stopPropagation();
              }, true);
  ```
  `confirmEffects` itself stays as it is (see coverage). The dialog's default focus already goes to the sample textarea: `core.modal` `focusFirst` picks `.ed-modal-body textarea` before `.ed-btn--primary` (`easydrag-core.js:457`). The browser test pins this.

- [ ] **Step 8: Add the key to all 16 locales.** Insert it after the `"easydrag.ui.test_title"` line (en/de :907) of each file:
  - en `"easydrag.ui.test_mode": "Test mode",`
  - de `"easydrag.ui.test_mode": "Testart",`
  - cs "Režim testu", da "Testtilstand", el "Λειτουργία δοκιμής", es "Modo de prueba", fr "Mode de test", hi "परीक्षण मोड", it "Modalità di test", ja "テストモード", nl "Testmodus", no "Testmodus", pl "Tryb testu", pt "Modo de teste", sv "Testläge", zh "测试模式"

  Confirm the 14 other locales with `Get-ChildItem ui/lang/easydrag`. Every file present needs a real translation.

- [ ] **Step 9: Update the tests that relied on `real` being the default.**

  (a) `ui/desktop_easydrag_browser_test.go`. Before (:142-143):
  ```go
  	page.MustEval(`()=>{edFixture.state.delay=450;edFixture.state.resyncAfter=3}`)
  	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
  ```
  After:
  ```go
  	// The flow sends a message, so Test opens in Simulation; this journey runs the real test.
  	page.MustEval(`()=>{const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
  	page.MustEval(`()=>{edFixture.state.delay=450;edFixture.state.resyncAfter=3}`)
  	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
  ```

  (b) `scripts/test-easydrag-extra4.mjs`. Before (:164):
  ```js
          step.el.querySelector('[data-ed-action="run"]').fire('click');
  ```
  After:
  ```js
          const stepMode = step.body.querySelector('[data-ed-test-mode]');
          if (stepMode) { stepMode.value = 'real'; stepMode.fire('change'); }
          step.el.querySelector('[data-ed-action="run"]').fire('click');
  ```

  (c) Run the suites in step 10. Some test may click Run on a flow with effects and expect a POST to `/test` (failure shapes: 0 posts, or `/simulate` requested). For each one, add the same explicit `real` selection right before its Run click: in Node, `mode.value = 'real'; mode.fire('change')`; in a browser test, the `MustEval` from (a). Change nothing else.

- [ ] **Step 10: Run the tests.** Expected: all pass.
  ```
  node scripts/test-easydrag-test-mode.mjs
  node scripts/test-easydrag.mjs; node scripts/test-easydrag-extra2.mjs; node scripts/test-easydrag-extra3.mjs; node scripts/test-easydrag-extra4.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m
  ```
  - The plain `go test` runs the static and i18n tests; the browser tests skip there.
  - Expected Node output includes "All EasyDrag test-mode checks passed."

- [ ] **Step 11: Commit.**
  ```
  git add ui/js/desktop/apps/easydrag-testing.js ui/js/desktop/apps/easydrag-run-start.js ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_test_mode_browser_test.go scripts/test-easydrag-test-mode.mjs scripts/test-easydrag-extra4.mjs ui/lang/easydrag/*.json
  git commit -m "fix(easydrag): Test opens in Simulation for flows with effects, the mode has its own label, and Enter never starts a real test" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```
  Also add any other test file changed in step 9(c).

---

### Task 4.2: Goal counter in the AI authoring dialog

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-authoring.js` (:4, :37-43, :75-85, export at the end of the file)
- Modify: `ui/css/desktop-app-easydrag.css` (after :124 `.ed-hint`)
- Modify: `scripts/test-easydrag-authoring.mjs` (before :45)
- Modify: `ui/desktop_easydrag_test_mode_browser_test.go` (add one test function)
- Modify: all 16 `ui/lang/easydrag/*.json` (`easydrag.ui.goal_counter`, after `"easydrag.ui.authoring_invalid"`, en/de :1017)

No Go test is added. `TestFlowAIProposalGoalUnicodeLimit` (`internal/server/flows_ai_proposal_test.go:21-42`) already sends 4,000 and 4,001 emoji and expects `FLOW_REVISION_CONFLICT` (that is, the goal was accepted) and `FLOW_BAD_REQUEST`, against `flows_ai_proposal.go:30-31`.

- [ ] **Step 1: Impact check.** Run `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "create" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`. Pick the result for easydrag-authoring.js. The name is generic, so if the result is UNKNOWN or ambiguous, grep for `authoring.create(` in `ui/js` (home.js and editor.js are expected). Report the risk.

- [ ] **Step 2: Write the failing Node test.** In `scripts/test-easydrag-authoring.mjs`, insert before :45 (`console.log('All EasyDrag proposal checks passed.');`):
  ```js
  // The goal counter counts code points of the trimmed goal, as the server does (4,000 runes).
  const counterT = (key, args) => key === 'easydrag.ui.goal_counter' ? args.count + ' / ' + args.max : key;
  const emoji = String.fromCodePoint(0x1F600);
  let goal = ED.authoring.goalCounter(emoji.repeat(4000), counterT, 'de-DE');
  assert.deepEqual([goal.count, goal.over, goal.text], [4000, false, '4.000 / 4.000']);
  goal = ED.authoring.goalCounter(emoji.repeat(4001), counterT, 'de-DE');
  assert.deepEqual([goal.count, goal.over, goal.text], [4001, true, '4.001 / 4.000']);
  goal = ED.authoring.goalCounter('  ab  ', counterT, 'en-US');
  assert.deepEqual([goal.count, goal.over, goal.text], [2, false, '2 / 4,000']);
  ```

- [ ] **Step 3: Run it.** `node scripts/test-easydrag-authoring.mjs`. Expected failure: `TypeError: ED.authoring.goalCounter is not a function`.

- [ ] **Step 4: Add the failing browser test.** Append to `ui/desktop_easydrag_test_mode_browser_test.go`:
  ```go
  // TestDesktopEasyDragGoalCounterBrowser: the AI goal has no maxlength, a counter linked by
  // aria-describedby counts code points, and above 4,000 it turns to the warn ink and Generate
  // is disabled.
  func TestDesktopEasyDragGoalCounterBrowser(t *testing.T) {
  	requirePrecisionBrowserSmoke(t)
  	s := startEasyDragSmoke(t)
  	p := s.page
  	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
  		t.Fatal(err)
  	}
  	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
  	p.MustElement(`.ed-flow-card h3`).MustClick()
  	s.wait(`()=>!!edFixture.editor()`)
  	p.MustEval(`()=>{window.__edAuthoring=EasyDrag.authoring.create(edFixture.editor(),null);window.__edAuthoring.open();}`)
  	s.wait(`()=>!!document.querySelector('[data-ed-authoring-goal]') && !!document.querySelector('[data-ed-goal-counter]')`)
  	p.MustEval(`()=>{const a=document.querySelector('[data-ed-authoring-goal]'),c=document.querySelector('[data-ed-goal-counter]');if(a.hasAttribute('maxlength'))throw Error('maxlength is still set');if(!c.id || a.getAttribute('aria-describedby')!==c.id)throw Error('the counter is not linked');a.value=String.fromCodePoint(0x1F600).repeat(4001);a.dispatchEvent(new Event('input',{bubbles:true}));}`)
  	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]'),nf=new Intl.NumberFormat(document.documentElement.lang||undefined);return c.classList.contains('is-over') && g.disabled && c.textContent.includes(nf.format(4001)) && c.textContent.includes(nf.format(4000));}`)
  	p.MustEval(`()=>{const a=document.querySelector('[data-ed-authoring-goal]');a.value=String.fromCodePoint(0x1F600).repeat(4000);a.dispatchEvent(new Event('input',{bubbles:true}));}`)
  	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]');return !c.classList.contains('is-over') && !g.disabled;}`)
  	s.noRawKeys("goal counter")
  	s.failOnPageErrors("goal counter")
  }
  ```
  Run `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDragGoalCounter)' -count=1 -timeout 30m`. Expected failure: the counter wait times out, because `[data-ed-goal-counter]` does not exist yet.

- [ ] **Step 5: Implement in `easydrag-authoring.js`.**

  (a) Before (:4):
  ```js
      const ED = window.EasyDrag = window.EasyDrag || {};
  ```
  After:
  ```js
      const ED = window.EasyDrag = window.EasyDrag || {};
      const GOAL_MAX = 4000;
      let goalCounters = 0;

      // goalCounter counts the goal the way the server does (flows_ai_proposal.go: trimmed, in
      // code points, at most 4,000) and formats "count / max" in the UI language.
      function goalCounter(value, t, locale) {
          const count = Array.from(String(value || '').trim()).length;
          let nf;
          try { nf = new Intl.NumberFormat(locale || undefined); } catch (err) { nf = new Intl.NumberFormat(); }
          return { count, over: count > GOAL_MAX, text: t('easydrag.ui.goal_counter', { count: nf.format(count), max: nf.format(GOAL_MAX) }) };
      }
  ```

  (b) Before (:37):
  ```js
              const requestController = controller;
  ```
  After:
  ```js
              const requestController = controller;
              const counterId = 'ed-goal-counter-' + (++goalCounters);
              // syncGoal is set once the dialog exists; an action re-applies it, because the
              // dialog's setBusy(false) enables every action button again.
              let syncGoal = () => {};
  ```

  (c) Body, before (:41):
  ```js
                  body: '<p class="ed-callout">' + esc(t('easydrag.ui.authoring_notice')) + '</p><label class="ed-label">' + esc(t('easydrag.ui.authoring_goal')) + '<textarea class="ed-input" rows="5" maxlength="8000" data-ed-authoring-goal></textarea></label><p class="ed-error" role="alert" hidden></p><section data-ed-authoring-preview hidden></section>',
  ```
  After:
  ```js
                  body: '<p class="ed-callout">' + esc(t('easydrag.ui.authoring_notice')) + '</p><label class="ed-label">' + esc(t('easydrag.ui.authoring_goal')) + '<textarea class="ed-input" rows="5" aria-describedby="' + counterId + '" data-ed-authoring-goal></textarea></label><p class="ed-goal-counter" id="' + counterId + '" data-ed-goal-counter></p><p class="ed-error" role="alert" hidden></p><section data-ed-authoring-preview hidden></section>',
  ```

  (d) Before (:43):
  ```js
                  onAction: async (action, view) => {
  ```
  After:
  ```js
                  onAction: (action, view) => (async () => {
  ```

  (e) Before (:75-78):
  ```js
                      return true;
                  }
              });
              const owned = dialog;
  ```
  After:
  ```js
                      return true;
                  })().finally(() => setTimeout(syncGoal, 0))
              });
              const owned = dialog;
  ```

  (f) Before (:79-85):
  ```js
              owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
              owned.body.querySelector('[data-ed-authoring-goal]').addEventListener('input', () => {
                  proposal = null;
                  owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
                  owned.el.querySelector('[data-ed-action="generate"]').hidden = false;
                  owned.body.querySelector('[data-ed-authoring-preview]').hidden = true;
              });
  ```
  After:
  ```js
              owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
              const goalInput = owned.body.querySelector('[data-ed-authoring-goal]');
              const counter = owned.body.querySelector('[data-ed-goal-counter]');
              const locale = (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || undefined;
              // Above the limit the counter marks a state (warn ink) and Generate is disabled; the
              // length check in onAction stays as a backstop.
              syncGoal = () => {
                  const state = goalCounter(goalInput.value, t, locale);
                  counter.textContent = state.text;
                  if (state.over) counter.classList.add('is-over'); else counter.classList.remove('is-over');
                  owned.el.querySelector('[data-ed-action="generate"]').disabled = state.over;
              };
              syncGoal();
              goalInput.addEventListener('input', () => {
                  proposal = null;
                  owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
                  owned.el.querySelector('[data-ed-action="generate"]').hidden = false;
                  owned.body.querySelector('[data-ed-authoring-preview]').hidden = true;
                  syncGoal();
              });
  ```
  The check at :50, `Array.from(goal).length > 4000`, stays unchanged.

  (g) At the end of the file, add `goalCounter` to the `ED.authoring = { ... }` export, after the names it already lists (today it holds at least `create` and `applyProposal`). For example, `ED.authoring = { create, applyProposal };` becomes `ED.authoring = { create, applyProposal, goalCounter };`.

- [ ] **Step 6: CSS.** In `ui/css/desktop-app-easydrag.css`, insert after :124 (`.ed-hint { ... }`):
  ```css
  .ed-goal-counter { margin: 4px 0 0; color: var(--ed-muted); font-size: 12px; font-variant-numeric: tabular-nums; text-align: end; }
  .ed-goal-counter.is-over { color: var(--ed-warn-ink); font-weight: 600; }
  ```

- [ ] **Step 7: Add the key to all 16 locales,** after `"easydrag.ui.authoring_invalid"` (en/de :1017).
  - en and de: `"easydrag.ui.goal_counter": "{count} / {max}",`
  - All other locales get the same value `{count} / {max}`. The numbers come from Intl.NumberFormat, so the value is a format and needs no words.
  - If an "untranslated value" check in the EasyDrag i18n tests flags it, add the key to that check's list of language-neutral keys.

- [ ] **Step 8: Run the tests.** Expected: all pass.
  ```
  node scripts/test-easydrag-authoring.mjs
  node scripts/test-easydrag-extra3.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m
  ```
  - extra3 loads authoring.js with the fake DOM.
  - Expected Node output includes "All EasyDrag proposal checks passed."

- [ ] **Step 9: Commit.**
  ```
  git add ui/js/desktop/apps/easydrag-authoring.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-authoring.mjs ui/desktop_easydrag_test_mode_browser_test.go ui/lang/easydrag/*.json
  git commit -m "fix(easydrag): the AI goal shows a code-point counter and disables Generate above 4,000" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 4.3: Trigger samples in the UI language

**Files:**
- Modify: `internal/flows/catalog_triggers.go` (:107-121 `newTriggerType`; :139 add after; sample literals :290, :306, :358, :376, :390, :391)
- Modify: `internal/flows/node.go` (:259-260 `NodeDef`)
- Modify: `internal/flows/catalog.go` (:83 `NodeTypeInfo`; :329-335 trigger branch of the describe function)
- Modify: `internal/flows/templates_builtin.go` (:359-365 in `CatalogI18nKeys`)
- Modify: `internal/flows/service_runs.go` (:105-131 and the rest of `triggerSample`)
- Modify: `internal/server/flows_handlers_runs.go` (:229-234)
- Modify: `internal/flows/catalog_triggers_test.go` (:157), `internal/flows/catalog_triggers_params_test.go` (:237)
- Create: `internal/flows/catalog_triggers_sample_i18n_test.go`, `internal/server/flows_trigger_sample_source_test.go`, `scripts/test-easydrag-samples.mjs`
- Modify: `ui/js/desktop/apps/easydrag-testing.js` (new `localSample`, export)
- Modify: `ui/js/desktop/apps/easydrag-run-start.js` (post-move; was runs.js :501-505, :537-557, :603-607)
- Modify: all 16 `ui/lang/easydrag/*.json` (8 `easydrag.sample.*` keys)

- [ ] **Step 1: Impact checks** (main checkout; same `run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo` form). Run them for `newTriggerType`, `TriggerSampleData`, `triggerSample`, `CatalogI18nKeys` and `openTest`. Then:
  - Find the describe function (the function that holds `catalog.go:329`, `if def.Trigger {`) and the test-data handler (the function that holds `flows_handlers_runs.go:229`). Grep for `^func ` in both files and take the nearest one above those lines. Run impact on both.
  - `triggerSample` is also used by the test start when the client sends no data. Report it and any HIGH or CRITICAL risk.

- [ ] **Step 2: Write the failing Go catalog test `internal/flows/catalog_triggers_sample_i18n_test.go`.**
  ```go
  package flows

  import (
  	"fmt"
  	"strconv"
  	"strings"
  	"testing"
  )

  // samplePath reads a dotted path ("zones.0" is the first zone) from a JSON-like value.
  func samplePath(v any, path string) any {
  	for _, part := range strings.Split(path, ".") {
  		switch m := v.(type) {
  		case map[string]any:
  			v = m[part]
  		case []any:
  			i, err := strconv.Atoi(part)
  			if err != nil || i < 0 || i >= len(m) {
  				return nil
  			}
  			v = m[i]
  		default:
  			return nil
  		}
  	}
  	return v
  }

  // TestTriggerSampleTextKeys: every named text field is a text field of its built-in sample,
  // the definition carries the keys for its catalog entry, and CatalogI18nKeys lists them, so
  // the 16-locale test (catalog_i18n_test.go) covers them. No built-in sample is German.
  func TestTriggerSampleTextKeys(t *testing.T) {
  	reg := NewRegistry()
  	if err := RegisterCatalog(reg, nil); err != nil {
  		t.Fatal(err)
  	}
  	listed := map[string]bool{}
  	for _, k := range CatalogI18nKeys(reg) {
  		listed[k] = true
  	}
  	if len(sampleTextKeys) == 0 {
  		t.Fatal("no sample text keys")
  	}
  	for typ, byEvent := range sampleTextKeys {
  		tt, ok := triggerTypes()[typ]
  		if !ok || len(tt.def.SampleTextKeys) != len(byEvent) {
  			t.Errorf("%s: definition does not carry its sample text keys", typ)
  		}
  		for event, fields := range byEvent {
  			params := map[string]any{}
  			if event != "" {
  				params["event"] = event
  			}
  			sample := TriggerSample(&Node{Type: typ, Params: params})
  			for path, key := range fields {
  				if _, ok := samplePath(sample, path).(string); !ok {
  					t.Errorf("%s %q: %s is not a text field of %v", typ, event, path, sample)
  				}
  				if !listed[key] {
  					t.Errorf("CatalogI18nKeys misses %s", key)
  				}
  			}
  		}
  	}
  	for typ := range triggerTypes() {
  		for _, event := range []string{"", "disconnected", "todo_overdue"} {
  			text := fmt.Sprint(TriggerSample(&Node{Type: typ, Params: map[string]any{"event": event}}))
  			for _, word := range []string{"Hallo", "Rechnung", "Anruf", "Zahnarzt", "Einfahrt"} {
  				if strings.Contains(text, word) {
  					t.Errorf("%s %q: built-in sample is German (%s): %s", typ, event, word, text)
  				}
  			}
  		}
  	}
  }
  ```

- [ ] **Step 3: Write the failing server test `internal/server/flows_trigger_sample_source_test.go`.** First read `flows_handlers_runs.go:235-250` (the PUT case). If its body needs more than `{"data": ...}`, add those fields to the PUT body below.
  ```go
  package server

  import (
  	"net/http"
  	"strings"
  	"testing"
  )

  // TestFlowTriggerSampleSource: the test-data answer says whether the sample is the built-in
  // default (which the editor shows in the UI language) or a saved one (user data).
  func TestFlowTriggerSampleSource(t *testing.T) {
  	s, token := newFlowsTestServer(t)
  	rec := createTestFlow(t, s, greetFlowJSON)
  	trigger := ""
  	for _, n := range rec.Draft.Nodes {
  		if strings.HasPrefix(n.Type, "trigger.") {
  			trigger = n.ID
  			break
  		}
  	}
  	if trigger == "" {
  		t.Fatal("the greet flow has no trigger")
  	}
  	path := "/api/desktop/flows/" + rec.ID + "/test-data/" + trigger
  	if got := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))["source"]; got != "builtin" {
  		t.Fatalf("source before saving = %v, want builtin", got)
  	}
  	if w := flowsCall(t, s, http.MethodPut, path, token, `{"data":{"text":"mine"}}`); w.Code != http.StatusOK {
  		t.Fatalf("save = %d %s", w.Code, w.Body.String())
  	}
  	body := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))
  	data, _ := body["data"].(map[string]any)
  	if body["source"] != "saved" || data["text"] != "mine" {
  		t.Fatalf("after saving = %v, want source saved with the saved data", body)
  	}
  }
  ```

- [ ] **Step 4: Write the failing Node test `scripts/test-easydrag-samples.mjs`.**
  ```js
  #!/usr/bin/env node
  // A built-in trigger sample shows its text fields in the UI language; a saved one is user data.
  import assert from 'node:assert/strict';
  import fs from 'node:fs';
  import vm from 'node:vm';

  const box = vm.createContext({ window: {}, console, Intl, Date, Map, Set, JSON });
  const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
  vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
  const ED = box.window.EasyDrag;
  const words = lang => JSON.parse(fs.readFileSync(new URL('../ui/lang/easydrag/' + lang + '.json', import.meta.url), 'utf8'));
  const infos = new Map([
      ['trigger.email', { type: 'trigger.email', trigger: true, sample_text_keys: { '': { subject: 'easydrag.sample.mail_subject', body: 'easydrag.sample.mail_body' } } }],
      ['trigger.planner', { type: 'trigger.planner', trigger: true, sample_text_keys: { '': { title: 'easydrag.sample.planner_appointment_title' }, todo_overdue: { title: 'easydrag.sample.planner_todo_title' } } }],
      ['trigger.person_detected', { type: 'trigger.person_detected', trigger: true, sample_text_keys: { '': { camera_name: 'easydrag.sample.person_camera', 'zones.0': 'easydrag.sample.person_camera' } } }]
  ]);
  const editor = lang => { const dict = words(lang); return { t: key => dict[key] || key, model: { info: type => infos.get(type) } }; };
  const mail = { id: 'n_mail', type: 'trigger.email', params: {} };
  const builtinMail = () => ({ source: 'builtin', data: { subject: 'October invoice', from: 'shop@example.com', body: 'Hello, please find the invoice attached.' } });

  assert.equal(ED.testing.localSample(editor('en'), mail, builtinMail()).subject, 'October invoice');
  const de = ED.testing.localSample(editor('de'), mail, builtinMail());
  assert.deepEqual([de.subject, de.body, de.from], ['Rechnung Oktober', 'Hallo, anbei die Rechnung.', 'shop@example.com']);
  assert.equal(ED.testing.localSample(editor('de'), mail, { source: 'saved', data: { subject: 'Mine' } }).subject, 'Mine', 'a saved sample stays');
  assert.equal(ED.testing.localSample(editor('de'), mail, { data: { subject: 'October invoice' } }).subject, 'October invoice', 'no source: left alone');
  const todo = ED.testing.localSample(editor('de'), { id: 'n_p', type: 'trigger.planner', params: { event: 'todo_overdue' } }, { source: 'builtin', data: { title: 'Pay the invoice' } });
  assert.equal(todo.title, 'Rechnung bezahlen', 'the event variant picks its own keys');
  const person = ED.testing.localSample(editor('de'), { id: 'n_c', type: 'trigger.person_detected', params: {} }, { source: 'builtin', data: { camera_name: 'Driveway', zones: ['Driveway'] } });
  assert.deepEqual([person.camera_name, person.zones[0]], ['Einfahrt', 'Einfahrt'], 'paths reach into lists');
  console.log('All EasyDrag sample checks passed.');
  ```

- [ ] **Step 5: Run the three tests.**
  ```
  go test ./internal/flows/ -run '(?i)(TestTriggerSampleTextKeys)' -count=1
  go test ./internal/server/ -run '(?i)(TestFlowTriggerSampleSource)' -count=1
  node scripts/test-easydrag-samples.mjs
  ```
  Expected failures:
  - the flows build fails with `undefined: sampleTextKeys` (and `tt.def.SampleTextKeys undefined`);
  - the server test fails with `source before saving = <nil>`;
  - the Node test fails with `ED.testing.localSample is not a function`.

- [ ] **Step 6: Server: English defaults plus the text keys.**

  (a) `catalog_triggers.go`: change the sample literals (before, then after).
  - :290
    - before: `}, map[string]any{"raw": `{"text":"Hallo"}`, "payload": map[string]any{"text": "Hallo"}}),`
    - after: `}, map[string]any{"raw": `{"text":"Hello"}`, "payload": map[string]any{"text": "Hello"}}),`
  - :306
    - before: `}, map[string]any{"subject": "Rechnung Oktober", "from": "shop@example.com", "body": "Hallo, anbei die Rechnung."}),`
    - after: `}, map[string]any{"subject": "October invoice", "from": "shop@example.com", "body": "Hello, please find the invoice attached."}),`
  - :358
    - before: `}, map[string]any{"event_id": 42, "camera_id": "driveway", "camera_name": "Einfahrt", "zones": []any{"Einfahrt"},`
    - after: `}, map[string]any{"event_id": 42, "camera_id": "driveway", "camera_name": "Driveway", "zones": []any{"Driveway"},`
  - :376
    - before: `}, map[string]any{"call_type": "call", "summary": "Anruf von 030 1234567", "time": sampleTime}),`
    - after: `}, map[string]any{"call_type": "call", "summary": "Call from 030 1234567", "time": sampleTime}),`
  - :390
    - before: `}, map[string]any{"appointment_id": "apt-1", "title": "Zahnarzt", "date_time": "2026-10-05T09:00:00Z", "time": sampleTime}),`
    - after: `}, map[string]any{"appointment_id": "apt-1", "title": "Dentist", "date_time": "2026-10-05T09:00:00Z", "time": sampleTime}),`
  - :391
    - before: `map[string]map[string]any{"todo_overdue": {"todo_id": "todo-1", "title": "Rechnung bezahlen", "due_date": "2026-10-02T17:00:00Z", "time": sampleTime}}),`
    - after: `map[string]map[string]any{"todo_overdue": {"todo_id": "todo-1", "title": "Pay the invoice", "due_date": "2026-10-02T17:00:00Z", "time": sampleTime}}),`

  (b) `catalog_triggers.go`: insert after `staticSample` (after :141):
  ```go
  // sampleTextKeys names the translatable text fields of the built-in trigger samples: trigger
  // type -> event variant ("" is the default sample, as in withEventSamples) -> dotted field
  // path ("zones.0" is the first zone) -> i18n key. The server keeps the English defaults; the
  // editor shows these fields in the UI language when it seeds a built-in sample
  // (TriggerSampleDataSource answers SampleSourceBuiltin). A saved sample is never changed.
  var sampleTextKeys = map[string]map[string]map[string]string{
  	TypeTriggerWebhook:        {"": {"payload.text": "easydrag.sample.webhook_text", "raw": "easydrag.sample.webhook_raw"}},
  	TypeTriggerEmail:          {"": {"subject": "easydrag.sample.mail_subject", "body": "easydrag.sample.mail_body"}},
  	TypeTriggerPersonDetected: {"": {"camera_name": "easydrag.sample.person_camera", "zones.0": "easydrag.sample.person_camera"}},
  	TypeTriggerFritzBox:       {"": {"summary": "easydrag.sample.call_summary"}},
  	TypeTriggerPlanner: {
  		"":             {"title": "easydrag.sample.planner_appointment_title"},
  		"todo_overdue": {"title": "easydrag.sample.planner_todo_title"},
  	},
  }

  // cloneSampleTextKeys copies a sample-text-key table (nil when empty).
  func cloneSampleTextKeys(in map[string]map[string]string) map[string]map[string]string {
  	if len(in) == 0 {
  		return nil
  	}
  	out := make(map[string]map[string]string, len(in))
  	for event, fields := range in {
  		m := make(map[string]string, len(fields))
  		for path, key := range fields {
  			m[path] = key
  		}
  		out[event] = m
  	}
  	return out
  }
  ```
  The webhook constant is used at the webhook `missionTriggerType(...)` call just above :282. Confirm its name with a grep for `missionTriggerType(TypeTrigger` and use the name found.

  (c) `newTriggerType`. Before (:116):
  ```go
  		SummaryKey:     "easydrag.node." + key + ".summary",
  ```
  After:
  ```go
  		SummaryKey:     "easydrag.node." + key + ".summary",
  		SampleTextKeys: cloneSampleTextKeys(sampleTextKeys[typ]),
  ```

  (d) `node.go`. Before (:259-260):
  ```go
  	Effects          []Effect
  	EffectsFunc      func(n *Node) []Effect
  ```
  After:
  ```go
  	Effects          []Effect
  	EffectsFunc      func(n *Node) []Effect
  	// SampleTextKeys names the translatable text fields of a trigger's built-in sample:
  	// event variant ("" for the default sample) -> dotted field path -> i18n key.
  	SampleTextKeys map[string]map[string]string
  ```

  (e) `catalog.go`. Before (:83):
  ```go
  	Sample         map[string]any `json:"sample,omitempty"`
  ```
  After:
  ```go
  	Sample         map[string]any `json:"sample,omitempty"`
  	// SampleTextKeys: see NodeDef.SampleTextKeys. The editor localizes those fields of a
  	// built-in sample.
  	SampleTextKeys map[string]map[string]string `json:"sample_text_keys,omitempty"`
  ```
  Before (:332-335):
  ```go
  		if describeHook(def, "TriggerSample", func() { data = cloneJSONMap(sample(sampleNode(def))) }) {
  			info.Sample = data
  		}
  	}
  ```
  After:
  ```go
  		if describeHook(def, "TriggerSample", func() { data = cloneJSONMap(sample(sampleNode(def))) }) {
  			info.Sample = data
  		}
  		info.SampleTextKeys = cloneSampleTextKeys(def.SampleTextKeys)
  	}
  ```

  (f) `templates_builtin.go` (`CatalogI18nKeys`). Before (:360-365):
  ```go
  		if describeHook(def, "OutputFieldsFunc", func() { fields = def.FieldsOf(sampleNode(def)) }) {
  			for _, f := range fields {
  				add(f.DescriptionKey)
  			}
  		}
  	}
  ```
  After:
  ```go
  		if describeHook(def, "OutputFieldsFunc", func() { fields = def.FieldsOf(sampleNode(def)) }) {
  			for _, f := range fields {
  				add(f.DescriptionKey)
  			}
  		}
  		for _, byField := range def.SampleTextKeys {
  			for _, k := range byField {
  				add(k)
  			}
  		}
  	}
  ```

  (g) `service_runs.go`. Replace :105-127, from the `// TriggerSampleData returns` comment through the `if ok { return data, nil }` block of `triggerSample`, with:
  ```go
  // Sample sources in the test-data answer. A saved sample is user data. The built-in one is
  // the trigger type's English default, which the editor shows in the UI language.
  const (
  	SampleSourceSaved   = "saved"
  	SampleSourceBuiltin = "builtin"
  )

  // TriggerSampleData returns the remembered sample data of a trigger node, or its built-in
  // sample. A node that is not an enabled trigger of the draft is ErrNoTrigger (wrapped,
  // bounded id), like in SaveTriggerSample.
  func (s *Service) TriggerSampleData(ctx context.Context, flowID, nodeID string) (map[string]any, error) {
  	data, _, err := s.TriggerSampleDataSource(ctx, flowID, nodeID)
  	return data, err
  }

  // TriggerSampleDataSource is TriggerSampleData and also names the sample's source
  // (SampleSourceSaved or SampleSourceBuiltin).
  func (s *Service) TriggerSampleDataSource(ctx context.Context, flowID, nodeID string) (map[string]any, string, error) {
  	rec, err := s.requireFlow(ctx, flowID)
  	if err != nil {
  		return nil, "", err
  	}
  	if err := s.requireTrigger(rec, nodeID); err != nil {
  		return nil, "", err
  	}
  	data, saved, err := s.triggerSampleSource(ctx, rec, nodeID)
  	if err != nil {
  		return nil, "", err
  	}
  	if saved {
  		return data, SampleSourceSaved, nil
  	}
  	return data, SampleSourceBuiltin, nil
  }

  // triggerSample is TriggerSampleData for a trigger node of rec's draft.
  func (s *Service) triggerSample(ctx context.Context, rec *FlowRecord, nodeID string) (map[string]any, error) {
  	data, _, err := s.triggerSampleSource(ctx, rec, nodeID)
  	return data, err
  }

  // triggerSampleSource is triggerSample; saved is true for a remembered sample.
  func (s *Service) triggerSampleSource(ctx context.Context, rec *FlowRecord, nodeID string) (map[string]any, bool, error) {
  	data, ok, err := s.store.GetTestData(ctx, rec.ID, nodeID, TestDataTriggerSample)
  	if err != nil {
  		return nil, false, err
  	}
  	if ok {
  		return data, true, nil
  	}
  ```
  - The rest of the old `triggerSample` body (from `n := rec.Draft.NodeByID(nodeID)` at :128 to its closing brace) stays as it is, with one mechanical change: every `return X, nil` becomes `return X, false, nil`, and every `return nil, err` becomes `return nil, false, err`. For example, :130 becomes `return map[string]any{}, false, nil`.
  - Run `gofmt -w internal/flows/service_runs.go internal/flows/node.go internal/flows/catalog.go internal/flows/catalog_triggers.go internal/flows/templates_builtin.go`.

  (h) `internal/server/flows_handlers_runs.go`. Before (:229-234):
  ```go
  			data, err := s.Flows.TriggerSampleData(ctx, id, node)
  			if err != nil {
  				s.flowsErrorFrom(w, r, err)
  				return true
  			}
  			s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"data": data})
  ```
  After:
  ```go
  			data, source, err := s.Flows.TriggerSampleDataSource(ctx, id, node)
  			if err != nil {
  				s.flowsErrorFrom(w, r, err)
  				return true
  			}
  			s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"data": data, "source": source})
  ```

  (i) Update the two tests that pin the old German defaults.
  - `catalog_triggers_test.go:157`: `if TriggerSample(email)["subject"] != "Rechnung Oktober" {` becomes `if TriggerSample(email)["subject"] != "October invoice" {`.
  - `catalog_triggers_params_test.go:237`: `s["title"] == "Zahnarzt"` becomes `s["title"] == "Dentist"`.

- [ ] **Step 7: Client.**

  (a) `easydrag-testing.js`: insert after `simulationDefault` (added in 4.1):
  ```js
      const UNSAFE_PATH = new Set(['__proto__', 'constructor', 'prototype']);

      // localSample shows a built-in trigger sample in the UI language. The catalog names the
      // text fields of each sample (sample_text_keys: event variant -> dotted path -> key; ""
      // is the default sample, as in withEventSamples on the server), and those fields take
      // t(key). A saved sample, or an answer without a source, is left as it is.
      function localSample(ed, node, answer) {
          const data = answer && answer.data && typeof answer.data === 'object' && !Array.isArray(answer.data) ? answer.data : {};
          if (!answer || answer.source !== 'builtin' || !node) return data;
          const byEvent = (ed.model.info(node.type) || {}).sample_text_keys;
          if (!byEvent || typeof byEvent !== 'object') return data;
          const event = String((node.params || {}).event == null ? '' : node.params.event).trim();
          const keys = event && Object.prototype.hasOwnProperty.call(byEvent, event) ? byEvent[event] : byEvent[''];
          if (!keys || typeof keys !== 'object') return data;
          Object.keys(keys).forEach(path => {
              const parts = path.split('.');
              if (parts.some(part => UNSAFE_PATH.has(part)) || typeof keys[path] !== 'string') return;
              let holder = data;
              for (let i = 0; i < parts.length - 1 && holder; i++) holder = typeof holder === 'object' ? holder[parts[i]] : null;
              const last = parts[parts.length - 1];
              if (!holder || typeof holder !== 'object' || typeof holder[last] !== 'string') return;
              const text = ed.t(keys[path]);
              if (typeof text === 'string' && text && text !== keys[path]) holder[last] = text;
          });
          return data;
      }
  ```
  The export becomes:
  ```js
      ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects, simulationDefault, localSample };
  ```

  (b) `easydrag-run-start.js` (post-move `openTest`).

  Before (was runs.js:501-506):
  ```js
              let prefilled = !!(o.data && typeof o.data === 'object' && !Array.isArray(o.data));
              let sample = {};
              if (prefilled) sample = o.data;
              else {
                  try { sample = (await ed.api.testData(ed.flow.id, trigger.id)).data || {}; } catch (err) { sample = {}; }
              }
  ```
  After:
  ```js
              let prefilled = !!(o.data && typeof o.data === 'object' && !Array.isArray(o.data));
              // seeded: the shown sample is a built-in one localized to the UI language. It runs as
              // shown (sent as trigger_data), but only a real change is remembered.
              let seeded = false;
              const localized = (answer, node) => {
                  const data = (answer && answer.data) || {};
                  const before = JSON.stringify(data);
                  const out = ED.testing && ED.testing.localSample ? ED.testing.localSample(ed, node, answer) : data;
                  return { data: out, seeded: JSON.stringify(out) !== before };
              };
              let sample = {};
              if (prefilled) sample = o.data;
              else {
                  try { const got = localized(await ed.api.testData(ed.flow.id, trigger.id), trigger); sample = got.data; seeded = got.seeded; } catch (err) { sample = {}; }
              }
  ```

  Before (was runs.js:538):
  ```js
                      const edited = text !== shown || prefilled;
  ```
  After:
  ```js
                      const changed = text !== shown || prefilled;
                      const edited = changed || seeded;
  ```

  Before (was runs.js:557):
  ```js
                      const remember = edited && d.body.querySelector('[data-ed-test-remember]').checked;
  ```
  After:
  ```js
                      const remember = changed && d.body.querySelector('[data-ed-test-remember]').checked;
  ```

  Before (was runs.js:603-607):
  ```js
                      let next = {};
                      try { next = (await ed.api.testData(ed.flow.id, id)).data || {}; } catch (err) { next = {}; }
                      if (select.value !== id || area.value !== shown) return;
                      shown = JSON.stringify(next, null, 2);
                      area.value = shown;
  ```
  After:
  ```js
                      let next = {}, nextSeeded = false;
                      try { const got = localized(await ed.api.testData(ed.flow.id, id), ed.model.node(id)); next = got.data; nextSeeded = got.seeded; } catch (err) { next = {}; }
                      if (select.value !== id || area.value !== shown) return;
                      shown = JSON.stringify(next, null, 2);
                      area.value = shown;
                      seeded = nextSeeded;
  ```
  The edited, prefilled and placeholder paths behave as before, because `changed` is the old `edited`.

  (c) Check that the catalog reaches `ed.model.info` with all its fields: grep for `output_fields` in `ui/js/desktop/apps/easydrag-*.js`. If a catalog normaliser copies the fields one by one, add `sample_text_keys` to it in the same way.

- [ ] **Step 8: Add the 8 sample keys to all 16 locales.**
  - Insert them directly before the `"easydrag.ui.test_title"` line, at the same place in every file.
  - `webhook_raw` must stay the JSON text `{"text":"…"}`, escaped in the file as `"{\"text\":\"…\"}"`. Only the word is translated, and it equals that locale's `webhook_text`.

  English, then German:
  - `easydrag.sample.webhook_text`: "Hello" / "Hallo"
  - `easydrag.sample.webhook_raw`: "{\"text\":\"Hello\"}" / "{\"text\":\"Hallo\"}"
  - `easydrag.sample.mail_subject`: "October invoice" / "Rechnung Oktober"
  - `easydrag.sample.mail_body`: "Hello, please find the invoice attached." / "Hallo, anbei die Rechnung."
  - `easydrag.sample.person_camera`: "Driveway" / "Einfahrt"
  - `easydrag.sample.call_summary`: "Call from 030 1234567" / "Anruf von 030 1234567"
  - `easydrag.sample.planner_appointment_title`: "Dentist" / "Zahnarzt"
  - `easydrag.sample.planner_todo_title`: "Pay the invoice" / "Rechnung bezahlen"

  The other 14 locales, in the key order webhook_text; mail_subject; mail_body; person_camera; call_summary; planner_appointment_title; planner_todo_title (webhook_raw wraps that locale's webhook_text):
  - cs: Ahoj; Faktura za říjen; Dobrý den, v příloze posílám fakturu.; Příjezdová cesta; Hovor z 030 1234567; Zubař; Zaplatit fakturu
  - da: Hej; Faktura for oktober; Hej, vedhæftet er fakturaen.; Indkørsel; Opkald fra 030 1234567; Tandlæge; Betal fakturaen
  - el: Γεια σου; Τιμολόγιο Οκτωβρίου; Γεια σας, επισυνάπτεται το τιμολόγιο.; Είσοδος οχημάτων; Κλήση από 030 1234567; Οδοντίατρος; Πληρωμή τιμολογίου
  - es: Hola; Factura de octubre; Hola, adjunto la factura.; Entrada de vehículos; Llamada de 030 1234567; Dentista; Pagar la factura
  - fr: Bonjour; Facture d'octobre; Bonjour, veuillez trouver la facture en pièce jointe.; Allée; Appel de 030 1234567; Dentiste; Payer la facture
  - hi: नमस्ते; अक्टूबर का बिल; नमस्ते, बिल संलग्न है।; ड्राइववे; 030 1234567 से कॉल; दंत चिकित्सक; बिल का भुगतान करें
  - it: Ciao; Fattura di ottobre; Ciao, in allegato la fattura.; Vialetto; Chiamata da 030 1234567; Dentista; Pagare la fattura
  - ja: こんにちは; 10月の請求書; こんにちは。請求書を添付します。; 車寄せ; 030 1234567 からの着信; 歯医者; 請求書を支払う
  - nl: Hallo; Factuur oktober; Hallo, bijgaand de factuur.; Oprit; Oproep van 030 1234567; Tandarts; Factuur betalen
  - no: Hei; Faktura for oktober; Hei, vedlagt er fakturaen.; Innkjørsel; Anrop fra 030 1234567; Tannlege; Betal fakturaen
  - pl: Cześć; Faktura za październik; Dzień dobry, w załączniku przesyłam fakturę.; Podjazd; Połączenie z 030 1234567; Dentysta; Zapłacić fakturę
  - pt: Olá; Fatura de outubro; Olá, segue a fatura em anexo.; Entrada da garagem; Chamada de 030 1234567; Dentista; Pagar a fatura
  - sv: Hej; Faktura för oktober; Hej, bifogat finns fakturan.; Uppfart; Samtal från 030 1234567; Tandläkare; Betala fakturan
  - zh: 你好; 十月发票; 你好，附件是发票。; 车道; 来自 030 1234567 的来电; 牙医; 支付发票

  If the folder holds a different set of locales, translate for the files present. All 16 files need all 8 keys.

- [ ] **Step 9: Run the tests.** Expected: all pass.
  ```
  go test ./internal/flows/ -count=1
  go test ./internal/server/ -run '(?i)(Flow)' -count=1
  node scripts/test-easydrag-samples.mjs
  node scripts/test-easydrag.mjs; node scripts/test-easydrag-extra2.mjs; node scripts/test-easydrag-extra3.mjs; node scripts/test-easydrag-extra4.mjs
  go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  $env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m
  ```
  - The flows run includes `TestTriggerSampleTextKeys`, `TestCatalogI18nKeys` and the 16-locale test in `catalog_i18n_test.go`.
  - If another flows test pins an old German default literal (for example an exact sample map in `catalog_hardening_sample_test.go`), swap only that literal for its English default.
  - Expected Node output includes "All EasyDrag sample checks passed."

- [ ] **Step 10: Commit.**
  ```
  git add internal/flows/catalog_triggers.go internal/flows/node.go internal/flows/catalog.go internal/flows/templates_builtin.go internal/flows/service_runs.go internal/flows/catalog_triggers_test.go internal/flows/catalog_triggers_params_test.go internal/flows/catalog_triggers_sample_i18n_test.go internal/server/flows_handlers_runs.go internal/server/flows_trigger_sample_source_test.go ui/js/desktop/apps/easydrag-testing.js ui/js/desktop/apps/easydrag-run-start.js scripts/test-easydrag-samples.mjs ui/lang/easydrag/*.json
  git commit -m "fix(easydrag): built-in trigger samples default to English and show in the UI language" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```
  Also add any test file changed in step 9 and any catalog normaliser changed in step 7(c).

---

### Part 4 coverage

| Spec item | Task |
|---|---|
| §6.1 Simulation preselected when the scope has named or unknown effects | 4.1 steps 6-7 (`simulationDefault`, env `real`) |
| §6.1 Flows with no effect keep `real` | 4.1 (Node test) |
| §6.1 Label key `easydrag.ui.test_mode` ("Test mode"/"Testart"); accessible name "Testart" | 4.1 steps 6b, 8; the browser test reads the name from the AX tree |
| §6.1 Enter never starts a real test; a run starts from click or Space | 4.1 step 7b; browser test (Enter on select, checkbox and run button gives no POST; Space gives a POST) |
| §6.1 confirmEffects has no Enter binding and no default-focus confirm button | 4.1 (see the answer below); the browser test checks the default focus |
| §6.2 maxlength removed, code-point counter, Intl.NumberFormat, `easydrag.ui.goal_counter`, aria-describedby, warn ink and disabled submit above 4,000, :50 kept | 4.2 |
| §6.2 Go 4,000/4,001 test | Already covered (see the answer below); no new test |
| §6.2 Node or browser: 4,001 emoji shows over the limit and disables submit | 4.2 (Node `goalCounter`; browser test checks the DOM state) |
| §6.3 English server defaults | 4.3 step 6a |
| §6.3 `SampleTextKeys` per type and event, exported with the catalog entry | 4.3 steps 6b-6e (`sample_text_keys`) |
| §6.3 The client replaces fields with `t(key)` for built-in samples only | 4.3 step 7 (`localSample`, both openTest fetch sites) |
| §6.3 `CatalogI18nKeys` lists the sample keys, so the 16-locale test covers them | 4.3 step 6f, Go test |
| §6.3 Node test: English shows the English subject, German the German one | 4.3 `scripts/test-easydrag-samples.mjs` |
| §14 "notify.telegram opens with Simulation, named Testart, Enter safe" / "counter matches server test" / "samples follow UI language" / "every new key in 16 locales, lowercase du" | 4.1 / 4.2 / 4.3; keys in steps 4.1-8, 4.2-7, 4.3-8 (no new German string addresses the user) |

**PLAN-CHECK answers.**
- **Simulation for durable flows: not allowed.** `Service.Simulate` (`internal/flows/simulation.go:94-98`) returns `ErrDurableInvalidGraph` for any draft node of type `logic.durable_wait` or `logic.durable_approval` (`durable_checkpoint_graph.go:4-5`). The locale text `durable_live_only` agrees. Durable flows therefore keep `real` and the effect confirmation (spec §13 decision 3). `ED.durable.requires` (durable.js:51) checks only `durable_wait`, so `simulationDefault` checks both types itself.
- **Built-in vs saved sample: the server did not say.** `TriggerSampleData` returned only the map, and the handler answered `{data}` (`flows_handlers_runs.go:229-234`). 4.3 adds `source: "builtin" | "saved"` through `TriggerSampleDataSource`. A client without a source leaves the sample alone.
- **The spec's `realEffects(ed, onlyNode, triggerId)` is really `effects(...)`.** In code, `realEffects(preview)` (testing.js:13) only turns the publish preview into a map. The rule is implemented as `effects(ed, onlyNode, trigger, real).size > 0 || unknownEffects(ed, onlyNode, trigger)`, so catalog effects and preview effects both count. A failed preview by itself does not select Simulation; this follows the spec formula, and catalog effects still count.
- **confirmEffects (runs.js:400) is not a dialog.** It records the effects of the dialog after a test started. The confirmation is the test dialog, and its Run button is the confirm button. `core.modal` has no `<form>` and only `type="button"` buttons, so no implicit submit exists. Its default focus goes to the sample textarea, not to `.ed-btn--primary` (`easydrag-core.js:457`). The only Enter path was native button activation, which the capture-phase guard blocks while the mode is `real`.
- **Go test for 4,000/4,001: it exists.** `TestFlowAIProposalGoalUnicodeLimit` (`flows_ai_proposal_test.go:21-42`).

### Notes for the other parts

- **Part 1 (run-start split).**
  - Move `openTest` verbatim. Keep the local names (`real`, `prefilled`, `shown`, `simulation`, `select`, `trigger`) and the env object passed to `ED.testing.decorate`.
  - Tasks 4.1 and 4.3 anchor on those exact lines in `easydrag-run-start.js`. If Part 1 renames `baseRevision` (for example to `core.baseRevision`), 4.1 step 7a keeps that name.
  - The new Node scripts `scripts/test-easydrag-test-mode.mjs` and `scripts/test-easydrag-samples.mjs` load only `easydrag-testing.js`, so the split leaves them alone. If a Go test or `package.json` lists EasyDrag Node scripts by name (the Part 1 PLAN-CHECK grep), add both scripts there.
- **Existing tests changed by 4.1:** `ui/desktop_easydrag_browser_test.go:142` and `scripts/test-easydrag-extra4.mjs:164` now select `real` explicitly. The main journey's `test-dialog` screenshot now shows Simulation, which affects the S0.5 contact sheet and the comparison screenshots, not the S0.0 canvas counters.
- **S0.5 (colour/lint).** `.ed-goal-counter.is-over` uses `--ed-warn-ink` as a state colour, which is allowed. The token and lint tests should accept it.
- **Not changed, for the UX-07 follow-up and the owner:**
  - The Ctrl+Enter quick test (`o.quick`) still runs a real test with no dialog when every effect is already confirmed (remembered) and the preview checked. This is unchanged by S0.
  - Durable flows still offer the Simulation option, which the server refuses.
- **Review.** Task 4.3 closes S0.3, so the combined S0.3 review covers 4.1-4.3 together (Critical and Important findings only).
