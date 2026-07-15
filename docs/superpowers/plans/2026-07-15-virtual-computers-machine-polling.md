# Virtual Computers Machine Polling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refresh each visible Virtual Computers machine list every five seconds while preserving active viewers and avoiding redraws when the server response is unchanged.

**Architecture:** Add one per-window chained timeout that calls only the existing machines endpoint. Store a serialized baseline whenever the normal refresh accepts machines, compare each background response with that baseline, and reuse the existing selection/VNC/terminal reconciliation functions only when the data changes.

**Tech Stack:** Vanilla JavaScript, Node.js `vm` regression tests, Go embedded-UI contract tests, GitNexus CLI.

## Global Constraints

- Poll every exactly 5,000 milliseconds with chained `setTimeout` calls; do not use `setInterval`.
- Do not redraw when the complete ordered machine array is unchanged.
- Pause requests while the browser tab is hidden or the desktop app window is minimized or invisible.
- Preserve an eligible active VNC or terminal session; close only sessions invalidated by machine deletion or a display-capability change.
- Background failures retain the last machine list and do not notify the user.
- Add no backend endpoint, configuration field, credential, database migration, dashboard entry, dependency, or translation key.
- Do not modify Quick Connect, the VNC controller, or the terminal controller.
- Preserve the existing user-owned `AGENTS.md` modification and stage only feature files.
- Run GitNexus impact before symbol edits and `detect-changes --scope compare --base-ref main` before committing.

---

## File Structure

- Modify `ui/js/desktop/apps/virtual-computers.js`: own the per-window timer, baseline, visibility check, machine-only request, reconciliation, and cleanup.
- Modify `scripts/test-ui-regressions.mjs`: execute the polling helpers with mocked requests, timers, visibility, render calls, and late responses.
- Modify `ui/desktop_virtual_computers_control_center_test.go`: lock the source-level lifecycle contract into the focused Go UI suite.
- Do not create a separate polling module: the implementation depends on private app state and existing private reconciliation helpers, so keeping it in the app closure is the smallest coherent boundary.

### Task 1: Add change-aware machine polling

**Files:**
- Modify: `ui/desktop_virtual_computers_control_center_test.go:41`
- Modify: `scripts/test-ui-regressions.mjs:643-718`
- Modify: `ui/js/desktop/apps/virtual-computers.js:4-900`

**Interfaces:**
- Consumes: `request(path, options)`, `reconcileSelection(state)`, `reconcileVNC(state)`, `reconcileTerminal(state)`, and `draw(state)` from `virtual-computers.js`.
- Produces: private `normalizeMachineList(body) -> Array`, `storeMachines(state, machines) -> void`, `isMachinePollingVisible(state) -> boolean`, `pollMachines(state) -> Promise<void>`, and `scheduleMachineRefresh(state) -> void` helpers.
- State additions: `machineSnapshot: string`, `machinePollTimer: number|null`, and `machinePollInFlight: boolean` on each app-window state object.

- [ ] **Step 1: Re-run impact analysis immediately before editing the four existing symbols**

Run:

```powershell
node .gitnexus/run.cjs impact "render" --direction upstream --repo AuraGo --file ui/js/desktop/apps/virtual-computers.js --include-tests
node .gitnexus/run.cjs impact "applyResourceResult" --direction upstream --repo AuraGo --file ui/js/desktop/apps/virtual-computers.js --include-tests
node .gitnexus/run.cjs impact "refresh" --direction upstream --repo AuraGo --file ui/js/desktop/apps/virtual-computers.js --include-tests
node .gitnexus/run.cjs impact "dispose" --direction upstream --repo AuraGo --file ui/js/desktop/apps/virtual-computers.js --include-tests
```

Expected: all four reports remain `LOW`; `applyResourceResult` and `refresh` affect only the existing Virtual Computers refresh/mutation callers, while `dispose` is called by `render`. If any result becomes `HIGH` or `CRITICAL`, stop and report the changed blast radius before editing.

- [ ] **Step 2: Add the failing Go lifecycle contract test**

Insert this test after `TestVirtualComputersDesktopRendersControlCenter` in `ui/desktop_virtual_computers_control_center_test.go`:

```go
func TestVirtualComputersMachineListPollingContract(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, marker := range []string{
		`const machinePollIntervalMs = 5000;`,
		`machineSnapshot: JSON.stringify([])`,
		`machinePollTimer: null`,
		`machinePollInFlight: false`,
		`function normalizeMachineList(body)`,
		`function storeMachines(state, machines)`,
		`function isMachinePollingVisible(state)`,
		`async function pollMachines(state)`,
		`function scheduleMachineRefresh(state)`,
		`request('/api/virtual-computers/machines')`,
		`document.visibilityState`,
		`closest('.vd-window')`,
		`scheduleMachineRefresh(state);`,
		`clearTimeout(state.machinePollTimer)`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("virtual computers machine polling missing %q", marker)
		}
	}

	applyStart := strings.Index(app, `function applyResourceResult`)
	applyEnd := strings.Index(app, `async function refresh`, applyStart)
	if applyStart < 0 || applyEnd <= applyStart || !strings.Contains(app[applyStart:applyEnd], `storeMachines(state, normalizeMachineList(body));`) {
		t.Fatal("normal machine refresh must update the polling snapshot")
	}

	disposeStart := strings.Index(app, `function dispose(windowId)`)
	disposeEnd := strings.Index(app, `window.VirtualComputersApp`, disposeStart)
	if disposeStart < 0 || disposeEnd <= disposeStart || !strings.Contains(app[disposeStart:disposeEnd], `clearTimeout(state.machinePollTimer)`) {
		t.Fatal("disposing a Virtual Computers window must clear machine polling")
	}
}
```

- [ ] **Step 3: Add the failing runtime regression test**

Add these helpers and the test before `testVirtualComputersAgentTaskFeedbackAndPolling` in `scripts/test-ui-regressions.mjs`:

```js
function loadVirtualComputersMachinePollingRuntime() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function normalizeMachineList', 'function hasActiveTasks');
  const requests = [];
  const scheduled = [];
  const cleared = [];
  const draws = [];
  const reconciled = [];
  let requestImpl = () => Promise.resolve({ machines: [] });
  const context = {
    Array,
    JSON,
    request(path) {
      requests.push(path);
      return requestImpl(path);
    },
    setTimeout(callback, delay) {
      scheduled.push({ callback, delay });
      return scheduled.length;
    },
    clearTimeout(id) { cleared.push(id); },
    reconcileSelection() { reconciled.push('selection'); },
    reconcileVNC() { reconciled.push('vnc'); },
    reconcileTerminal() { reconciled.push('terminal'); },
    draw(state) { draws.push(state.machines.map(machine => machine.id)); }
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.storeMachines = storeMachines; ` +
    'globalThis.pollMachines = pollMachines; globalThis.scheduleMachineRefresh = scheduleMachineRefresh;',
    context
  );
  return {
    context,
    requests,
    scheduled,
    cleared,
    draws,
    reconciled,
    setRequestImpl(impl) { requestImpl = impl; }
  };
}

function newVirtualComputersPollingState() {
  const appWindow = { hidden: false, style: { display: '' } };
  const ownerDocument = { visibilityState: 'visible' };
  const host = {
    ownerDocument,
    closest(selector) { return selector === '.vd-window' ? appWindow : null; },
    getClientRects() { return [{}]; }
  };
  return {
    state: {
      host,
      machines: [],
      machineSnapshot: JSON.stringify([]),
      machinePollTimer: null,
      machinePollInFlight: false,
      resourceLoading: { machines: false },
      resourceErrors: { machines: 'old error' },
      refreshGeneration: 1,
      disposed: false
    },
    appWindow,
    ownerDocument
  };
}

async function testVirtualComputersMachinePollingLifecycle() {
  const runtime = loadVirtualComputersMachinePollingRuntime();
  const { state, appWindow, ownerDocument } = newVirtualComputersPollingState();

  runtime.context.scheduleMachineRefresh(state);
  assert.equal(runtime.scheduled.length, 1);
  assert.equal(runtime.scheduled[0].delay, 5000);

  runtime.setRequestImpl(() => Promise.resolve({ machines: [] }));
  await runtime.scheduled.shift().callback();
  assert.deepEqual(runtime.requests, ['/api/virtual-computers/machines']);
  assert.equal(runtime.draws.length, 0, 'unchanged machines must not redraw');
  assert.equal(runtime.scheduled.length, 1, 'each completed poll must schedule its successor');

  ownerDocument.visibilityState = 'hidden';
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'hidden tabs must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  ownerDocument.visibilityState = 'visible';
  appWindow.style.display = 'none';
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'minimized app windows must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  appWindow.style.display = '';
  state.host.getClientRects = () => [];
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'invisible app hosts must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  state.host.getClientRects = () => [{}];
  runtime.setRequestImpl(() => Promise.resolve({ machines: [{ id: 'vm-1', display: false }] }));
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 2);
  assert.deepEqual(runtime.draws, [['vm-1']]);
  assert.deepEqual(runtime.reconciled, ['selection', 'vnc', 'terminal']);
  assert.equal(state.resourceErrors.machines, '');
  assert.equal(runtime.scheduled.length, 1);

  runtime.setRequestImpl(() => Promise.reject(new Error('offline')));
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.draws.length, 1, 'background failures must retain the rendered list');
  assert.equal(runtime.scheduled.length, 1, 'background failures must retry on the next cycle');

  runtime.context.storeMachines(state, [{ id: 'vm-1', display: false }]);
  runtime.setRequestImpl(() => Promise.resolve({ machines: [{ id: 'vm-1', display: false }] }));
  await runtime.context.pollMachines(state);
  assert.equal(runtime.draws.length, 1, 'a full-refresh baseline must prevent a redundant redraw');

  let releaseSlowRequest;
  runtime.setRequestImpl(() => new Promise(resolve => { releaseSlowRequest = resolve; }));
  const firstPoll = runtime.context.pollMachines(state);
  const secondPoll = runtime.context.pollMachines(state);
  assert.equal(runtime.requests.length, 5, 'an in-flight poll must suppress overlap');
  releaseSlowRequest({ machines: [{ id: 'vm-2', display: false }] });
  await Promise.all([firstPoll, secondPoll]);
  assert.deepEqual(state.machines.map(machine => machine.id), ['vm-2']);

  let releaseLateRequest;
  runtime.setRequestImpl(() => new Promise(resolve => { releaseLateRequest = resolve; }));
  const latePoll = runtime.context.pollMachines(state);
  state.disposed = true;
  releaseLateRequest({ machines: [{ id: 'vm-late', display: false }] });
  await latePoll;
  assert.deepEqual(state.machines.map(machine => machine.id), ['vm-2'], 'late responses after dispose must be ignored');
}

function testVirtualComputersMachinePollingDisposeClearsTimer() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const disposeSource = sourceBetween(app, 'function dispose(windowId)', 'window.VirtualComputersApp');
  const cleared = [];
  const state = {
    disposed: false,
    refreshGeneration: 2,
    taskRefreshTimer: 7,
    machinePollTimer: 8,
    clickHandler: null,
    changeHandler: null,
    keyHandler: null,
    host: {}
  };
  const context = {
    instanceMap: new Map([['vc-1', state]]),
    clearTimeout(id) { cleared.push(id); },
    disconnectRemoteSessions() {}
  };
  vm.createContext(context);
  vm.runInContext(
    `const instances = globalThis.instanceMap; ${disposeSource}; globalThis.disposeApp = dispose;`,
    context
  );
  context.disposeApp('vc-1');
  assert.deepEqual(cleared, [7, 8]);
  assert.equal(state.disposed, true);
  assert.equal(state.refreshGeneration, 3);
  assert.equal(context.instanceMap.has('vc-1'), false);
}
```

Add both tests to the `tests` array:

```js
  ['Virtual Computers polls machines only when visible and changed', testVirtualComputersMachinePollingLifecycle],
  ['Virtual Computers clears machine polling on dispose', testVirtualComputersMachinePollingDisposeClearsTimer],
```

Change the runner so asynchronous polling assertions are awaited:

```js
for (const [name, test] of tests) {
  try {
    await test();
    console.log(`PASS ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`FAIL ${name}: ${error.message}`);
  }
}
```

- [ ] **Step 4: Run the new tests and verify the red phase**

Run:

```powershell
go test ./ui -run 'TestVirtualComputersMachineListPollingContract' -count=1
npm run test:ui-regressions
```

Expected: the Go test fails because the polling markers are absent, and the Node suite reports the new polling tests as failures because `normalizeMachineList` does not exist. Existing tests must not acquire unrelated failures.

- [ ] **Step 5: Add per-window polling state and start the scheduler**

In `ui/js/desktop/apps/virtual-computers.js`, define the interval beside `instances`:

```js
    const instances = new Map();
    const machinePollIntervalMs = 5000;
```

Add these fields to the state object created by `render`:

```js
            machineSnapshot: JSON.stringify([]),
            machinePollTimer: null,
            machinePollInFlight: false,
```

Start the loop after the initial full refresh has been launched:

```js
        bindActions(state);
        draw(state);
        refresh(state);
        scheduleMachineRefresh(state);
```

- [ ] **Step 6: Centralize accepted machine responses**

In `applyResourceResult`, replace the direct machine assignment with:

```js
            if (resource === 'machines') storeMachines(state, normalizeMachineList(body));
```

In the `!status.enabled` branch of `refresh`, replace `state.machines = [];` with:

```js
                storeMachines(state, []);
```

This makes initial refreshes, manual refreshes, mutation refreshes, and disabled-state refreshes all establish the background comparison baseline.

- [ ] **Step 7: Implement visibility, comparison, request, and rescheduling helpers**

Insert these helpers after `notify` and before `hasActiveTasks`:

```js
    function normalizeMachineList(body) {
        return body && Array.isArray(body.machines) ? body.machines : [];
    }

    function storeMachines(state, machines) {
        const normalized = Array.isArray(machines) ? machines : [];
        state.machines = normalized;
        state.machineSnapshot = JSON.stringify(normalized);
    }

    function isMachinePollingVisible(state) {
        if (!state.host) return false;
        const document = state.host.ownerDocument;
        if (document && document.visibilityState === 'hidden') return false;
        const appWindow = typeof state.host.closest === 'function' ? state.host.closest('.vd-window') : null;
        if (appWindow && (appWindow.hidden || (appWindow.style && appWindow.style.display === 'none'))) return false;
        return typeof state.host.getClientRects !== 'function' || state.host.getClientRects().length > 0;
    }

    async function pollMachines(state) {
        if (state.disposed || state.machinePollInFlight || state.resourceLoading.machines || !isMachinePollingVisible(state)) return;
        const generation = state.refreshGeneration;
        state.machinePollInFlight = true;
        try {
            const body = await request('/api/virtual-computers/machines');
            if (state.disposed || generation !== state.refreshGeneration) return;
            const machines = normalizeMachineList(body);
            const snapshot = JSON.stringify(machines);
            if (snapshot === state.machineSnapshot) return;
            storeMachines(state, machines);
            state.resourceErrors.machines = '';
            reconcileSelection(state);
            reconcileVNC(state);
            reconcileTerminal(state);
            draw(state);
        } catch (_) {
            // Background polling is best-effort; explicit refresh reports errors.
        } finally {
            state.machinePollInFlight = false;
        }
    }

    function scheduleMachineRefresh(state) {
        if (state.machinePollTimer) {
            clearTimeout(state.machinePollTimer);
            state.machinePollTimer = null;
        }
        if (state.disposed) return;
        state.machinePollTimer = setTimeout(async () => {
            state.machinePollTimer = null;
            await pollMachines(state);
            scheduleMachineRefresh(state);
        }, machinePollIntervalMs);
    }
```

The local `document` variable is intentionally derived from `state.host.ownerDocument`; this keeps multiple documents/test contexts independent and still exposes the explicit `document.visibilityState` contract.

- [ ] **Step 8: Dispose the machine timer**

Extend `dispose` immediately after task-timer cleanup:

```js
        if (state.taskRefreshTimer) clearTimeout(state.taskRefreshTimer);
        if (state.machinePollTimer) clearTimeout(state.machinePollTimer);
        state.machinePollTimer = null;
```

The existing `state.disposed = true` and `state.refreshGeneration++` lines make any already-running response harmless; `pollMachines` clears its in-flight flag in `finally`.

- [ ] **Step 9: Run focused tests and verify the green phase**

Run:

```powershell
go test ./ui -run 'TestVirtualComputers(MachineListPollingContract|DesktopRendersControlCenter|TerminalLifecycleAndMachineGating|VNCLifecycleAndPermissionGating)' -count=1
npm run test:ui-regressions
```

Expected: all selected Go tests pass; the Node suite prints `PASS Virtual Computers polls machines only when visible and changed` and `PASS Virtual Computers clears machine polling on dispose`, then exits successfully.

- [ ] **Step 10: Run the complete verification matrix**

Run each command separately so a failure is attributable:

```powershell
go test ./internal/server -run 'TestVirtualComputers' -count=1
go test ./ui -run 'TestVirtualComputers' -count=1
npm run check:ui
npm run test:ui-regressions
go test ./...
```

Expected: every command exits with status 0. No generated UI bundle changes should appear because the modified app is loaded as a standalone embedded asset.

- [ ] **Step 11: Review scope, run GitNexus change detection, and commit only feature files**

Run:

```powershell
git diff --check
git status --short
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo AuraGo
git add -- ui/js/desktop/apps/virtual-computers.js scripts/test-ui-regressions.mjs ui/desktop_virtual_computers_control_center_test.go
git diff --cached --check
git diff --cached
git status --short
git commit -m "feat(virtual-computers): refresh machine list automatically"
```

Expected: GitNexus reports only the intended Virtual Computers refresh/lifecycle symbols and their existing callers. The staged diff contains exactly the three files above, no secret-like values, no locale files, and no `AGENTS.md` change.

- [ ] **Step 12: Fast-forward the approved feature into local `main` without touching `AGENTS.md`**

From the primary repository directory, preserve and compare the user-owned file hash around the merge:

```powershell
$main = 'C:\Users\Andi\Documents\repo\AuraGo'
$agentsHash = git -C $main hash-object AGENTS.md
git -C $main merge --ff-only feat/virtual-computers-headless-terminal
if ((git -C $main hash-object AGENTS.md) -ne $agentsHash) { throw 'AGENTS.md changed during merge' }
git -C $main status --short --branch
git -C $main log -3 --oneline
```

Expected: `main` fast-forwards through the design, plan, and implementation commits; `AGENTS.md` remains the only pre-existing working-tree modification and is neither staged nor committed.
