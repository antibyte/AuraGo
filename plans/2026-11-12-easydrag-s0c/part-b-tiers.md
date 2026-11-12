[← Plan index](README.md) · Spec: [plans/2026-11-12-easydrag-s0c-spec.md](../2026-11-12-easydrag-s0c-spec.md)

## Part B: test tiers (FD-15)

Goal: one fast command (T0), one shared Chrome for the EasyDrag browser tests (T1), reduced motion set in one place with a guard, and the target-size test of A5.

**Shell and paths.** Git Bash. Every command starts with `cd <worktree> && ...`, because the working directory resets between calls. Browser tests need `GOTMPDIR='D:\tmp\go-build'`, `AURAGO_RUN_BROWSER_SMOKE=1`, `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots` and the Bash sandbox off (Chrome cannot start from a test binary under the repo with the sandbox on; see memory note `reference_chrome_launch_from_agent.md`). Never set `GOTMPDIR` for `internal/server`. This machine has `GOTMPDIR=D:\tmp\go-build` set globally, so a command that must run without it uses `env -u GOTMPDIR`.

**Worktree.** Part B starts now, in parallel with Part A:

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0c-tiers -b feat/easydrag-s0c-tiers main
mkdir -p C:/disposable/easydrag-s0c D:/tmp/go-build D:/tmp/easydrag-s0c-shots
```

Do not edit the main checkout. Every commit body ends with the fallback line (GitNexus cannot index a worktree here), then the `Co-Authored-By:` line. Never amend, rebase or push inside a task; Task B.6 starts with the one `git rebase main` this part needs, after the coordinator says Part A landed.

**What this part depends on.** Tasks B.1 to B.5 depend on nothing. Task B.6 needs Part A on main (its CSS for A5). Part C appends `./cmd/flowsdoc` to `GO_STEPS` (Task B.2) and Part D uses `startEasyDragSmokeWith` (Task B.4).

**Locale keys.** This part adds none.

### Baseline (measured while planning, main at b52103d51, 16 CPUs, Node 24.11, Go 1.27.3)

Run these once in the new worktree before Task B.1 and keep the numbers for comparison:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && time node scripts/test-easydrag.mjs
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee C:/disposable/easydrag-s0c/browser-base.log | grep -E '^(--- (FAIL|SKIP)|ok|FAIL)'
```

Expected and measured:

| Command | Result |
|---|---|
| `node scripts/test-easydrag.mjs` | ends `all EasyDrag checks passed`, exit 0, **223 to 249 s** (201 s of it is one check, see Task B.1) |
| `go test ./ui -run '(?i)easydrag' -count=1` (no browser) | `ok  	aurago/ui	6.0s`, the browser tests skip |
| the gated run | `--- SKIP: TestDesktopEasyDragContactSheetBrowser (0.00s)` (it needs its own `AURAGO_EASYDRAG_CONTACT_SHEET=1`), `ok  	aurago/ui	299.7s`, 33 `--- PASS` lines, 0 FAIL; wrapper time **306 s** |

The 33 PASS lines include the pure source tests that match `(?i)easydrag`. After each task the gated count changes only by the tests the task adds: B.4 adds the guard (34), B.5 adds the motion test (35), B.6 adds the target-size test (36).

### How the 22 EasyDrag browser files open their page today

Spec §4.2 asks for this list. Every file uses `startEasyDragSmoke(t, fixtures...)` (22 of 22), which launches **its own Chrome** through `newSmokeBrowser(t)` and closes it in `t.Cleanup`. Two exceptions:

| File | Page opened by | Notes |
|---|---|---|
| `desktop_easydrag_browser_test.go` | defines the helper (the only `newSmokeBrowser` call, in `startEasyDragSmoke`) | 1 test uses it |
| `desktop_easydrag_durable_browser_test.go` | `startEasyDragSmoke` twice, **and `newSmokeBrowser(t).MustPage()` directly** in `TestConfigEasyDragDurableSwitchBrowser` | that test opens `/config`, not the EasyDrag fixture; it needs no launch flag, so it moves to the helper (`newEasyDragPage`) |
| `desktop_easydrag_approval_`, `batches_`, `blocks_`, `boundary_`, `child_history_`, `colour_`, `connections_`, `contact_sheet_`, `credentials_`, `credentials_purge_`, `decision_`, `extraction_`, `groups_`, `legibility_`, `library_pause_`, `metadata_`, `nested_`, `pack_`, `test_mode_`, `ux_` `browser_test.go` | `startEasyDragSmoke`, one Chrome per call | `test_mode` calls it twice |

Seven files set reduced motion themselves after the page opened: `credentials`, `credentials_purge` and `test_mode` (twice) with an inline `proto.EmulationSetEmulatedMedia`, `colour`, `contact_sheet` and `legibility` with `setReducedMotion(t, s.page, "reduce")`. Task B.5 removes all of them, because the helper does it for every page. No file needs launch flags or browser-wide state, so no file uses `PrivateBrowser` today and the exception map of the guard (Task B.4) starts empty.

### Measured while planning (the T0 numbers)

Throwaway detached worktree, 16 CPUs. "Warm" means the Go build cache was warm and `-count=1` forced the tests to run.

| Step | Cold (first run in a fresh worktree, shared build cache) | Warm, `-count=1` (2 runs) | Warm, cached result |
|---|---|---|---|
| `go test ./internal/flows/...` | 48.6 s | 38.4 / 36.3 s | 1.7 s |
| `go test ./internal/server -run '(?i)flow'` | 69.8 s | 17.8 / 17.7 s | 4.3 s |
| `go test ./internal/tools -run '(?i)flow'` | 22.9 s | 13.1 / 12.9 s | 1.8 s |
| `go test ./ui -run '(?i)easydrag'` | 21.0 s | 5.9 / 6.4 s | 2.5 s |
| Go total, one after another | 162 s | **73 s** | 10 s |
| 35 Node scripts, parallel, after Task B.1 | | 21 s (17.8 s is `test-easydrag.mjs`) | |
| **T0 as planned (Node and Go together)** | | **37.4 and 39.0 s** | **26.4 s** |
| T0 if the Go steps ran after the Node scripts | | 21 + 73 = 94 s, above the 60 s target | |

The flows package alone is 1,080 tests that run one after another (36.4 s in `go test -json`); no single test is above 3 s, so there is no slow test to cut. The spec's fallback (move the slowest Go step, internal/flows, to T1) would leave 21 s of Node plus 17.7 + 12.9 + 6.4 s of Go, about 58 s: two seconds under the target, with no margin, and the flows tests out of T0. Running the four Go steps and the Node scripts at the same time reaches 37 to 39 s with every step in T0. That is the plan (see Deviations, point 1).

### Stale scripts (spec §4.1, task B.1)

Each of the 35 `scripts/test-easydrag*.mjs` ran once in a clean detached worktree of main with a 120 s timeout. **All 35 exit 0.** Silent means the script prints nothing on success.

| Script | Result | Time |
|---|---|---|
| test-easydrag-approvals, authoring, compare, credentials-move, credentials-purge, credentials, notes, samples, test-mode, transform | pass, one summary line | 0.3 to 0.5 s |
| test-easydrag-render-baseline | pass, `PASS render baseline (unchanged)` | 3.7 s |
| test-easydrag-batches, blocks, child-history, durable, extra, extra2 to extra7, extraction, flagship, groups-save, groups, kv-editor, library, live-state, metadata, nested, pack, scenarios, ux-batch5 | pass, silent (exit 0): each only exports `run()`, which `scripts/test-easydrag.mjs` calls with its harness; alone it proves that the module imports and parses | 0.3 to 0.5 s |
| **test-easydrag** | **pass, but 223 to 249 s**; timed out at the 120 s limit on the first loop | 223 to 249 s |

Findings:
- Nothing fails, so nothing is deleted and nothing needs a product-code change. **The coordinator has nothing to take over.**
- `test-easydrag.mjs` runs most of the others through `import`, so T0 runs them twice; the standalone runs cost under 0.5 s each in parallel and also catch a module that no longer imports.
- The one real problem is time. A CPU profile of the run puts 90 percent into the test's own mini DOM (`matchCompound` 69.6 s self, `matches` 56.9 s, the lambda at `:907` 38.4 s, `querySelector` 12.3 s), called from one check in `scripts/test-easydrag-ux-batch5.mjs`: `b5 F6 run events repaint only the steps they touch: 300 steps, 1000 events`, which prints `info b5 F6 1000 events complete in 201100 ms`. The cost grows with steps times events. The product does not do this work; a real DOM answers these queries in microseconds. Task B.1 shrinks that check to 40 steps and 160 events (3.7 s) and keeps every assertion, which are counts and do not depend on the size.

### Task B.1: Make `test-easydrag.mjs` fit in T0 (spec B1.1)

**Files:**
- Modify: `scripts/test-easydrag-ux-batch5.mjs:810-835` (the F6 check)
- Test: `scripts/test-easydrag.mjs`, `scripts/test-easydrag-ux-batch5.mjs`

- [ ] **Step 1: Confirm the cost and the lines.** The run prints the info line near the end.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && grep -n "F6 run events repaint\|chainDoc(300)\|k < 500\|k % 299\|1000 events\|<= 2000\|nodes\[299\]\|\[\[0, 1000, true\], 298" scripts/test-easydrag-ux-batch5.mjs
```

Expected: 8 lines between 812 and 831 (`812` the name, `814` `chainDoc(300)`, `823` the loop, `824` `k % 299`, `827` the info line, `829` `<= 2000`, `831` `nodes[299]`, `832` the expected array). If the line numbers differ by a few, use the text.

- [ ] **Step 2: Change the size, with the expected numbers.** With 40 steps the chain has nodes 1 to 39 that events touch (`1 + k % 39`), 80 loops send 160 events (two per loop), one paint per event gives 160, at most two wires per event gives 320, and every finished step except the last has its pill, so 38. Apply these seven replacements with the Edit tool (each `old_string` is unique in the file):

| old | new |
|---|---|
| `run events repaint only the steps they touch: 300 steps, 1000 events` | `run events repaint only the steps they touch: 40 steps, 160 events` |
| `const chain = chainDoc(300);` | `const chain = chainDoc(40);` |
| `for (let k = 0; k < 500; k++) {` | `for (let k = 0; k < 80; k++) {` |
| `chain.nodes[1 + (k % 299)].id` | `chain.nodes[1 + (k % 39)].id` |
| `'info b5 F6 1000 events complete in '` | `'info b5 F6 160 events complete in '` |
| `stats.wires - before.wires <= 2000` | `stats.wires - before.wires <= 320` |
| `s.card(chain.nodes[299].id)` | `s.card(chain.nodes[39].id)` |
| `[[0, 1000, true], 298, true, true, []]` | `[[0, 160, true], 38, true, true, []]` |

Add one comment line above the `await guardAsync('b5 F6 run events repaint ...` line: `// The checks are counts, so they hold at any size. The test's mini DOM scans every card per event, so 300 steps took 200 s; 40 steps take 4 s.`

- [ ] **Step 3: Run it.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && time node scripts/test-easydrag.mjs 2>&1 | grep -E "b5 F6|all EasyDrag|failure"
```

Expected (the times are the measured ones, give or take a few seconds):

```
info b5 F6 160 events complete in 3713 ms
ok   b5 F6 no card is re-rendered, one paint per event, at most two wires per event, every finished step's wire has its pill
ok   b5 F6 renderStats carries the S0.0 counters
ok   b5 F6 incoming, outgoing and edge after disconnect, undo, connect and replaceDoc; a caller's change to a list stays its own; walks over 3000
all EasyDrag checks passed
real	0m28s
```

If the pill count or the paint count differs, the arithmetic above is the place to look: paints = 2 x loops, pills = steps - 2.

- [ ] **Step 4: Gates.** `node scripts/test-easydrag-render-baseline.mjs` prints `PASS render baseline (unchanged)`.

- [ ] **Step 5: Commit.** Write `C:/disposable/easydrag-s0c/b1.txt` with the Write tool:

```
test(easydrag): run the F6 event-cost check at 40 steps

scripts/test-easydrag.mjs took 223 to 249 s, 201 s of it in one check
that sent 1000 run events over a 300-step chain: the test's mini DOM
scans every card for every event. The check asserts counts (no card
re-rendered, one paint per event, two wires per event at most, a pill
per finished step), which hold at any size, so it now runs 160 events
over 40 steps (3.7 s) and the whole script takes 28 s. All 35 EasyDrag
scripts pass on main; none is stale. This makes T0 fit its 60 s target.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add scripts/test-easydrag-ux-batch5.mjs && git commit -F C:/disposable/easydrag-s0c/b1.txt
```

### Task B.2: The T0 script and `npm run test:easydrag:t0` (spec B1.2)

**Files:**
- Create: `scripts/test-easydrag-t0.mjs`
- Modify: `package.json:27` (one line after `"test:easydrag"`)
- Test: the script itself

Design, as measured above: every `scripts/test-easydrag*.mjs` except the T0 script itself runs in its own process, at most `availableParallelism() - GO_STEPS.length` at a time (12 here); the four Go steps start at once, one process each; the first failure kills every other child (`taskkill /T /F` on Windows, a process-group kill elsewhere) and prints its last 60 lines; the total time is printed. `--node-only` skips the Go steps. The script sets no environment variable: every child gets `process.env` plus the step's own `env` (empty today), so `GOTMPDIR`, `GOFLAGS` and the rest come from the caller. `GO_STEPS` is one array at the top of the file, each entry `{ label, args, env }`; Part C appends `./cmd/flowsdoc`.

- [ ] **Step 1: Write the script.** Create `scripts/test-easydrag-t0.mjs` with exactly this content (it was run against main in a throwaway worktree, with Node 22.22 and 24.11):

```js
// T0: the fast EasyDrag check. One command, no browser.
//
//   node scripts/test-easydrag-t0.mjs              Node scripts and Go steps
//   node scripts/test-easydrag-t0.mjs --node-only  Node scripts only (the CI Node job)
//
// Every scripts/test-easydrag*.mjs runs in a process of its own, at most one per CPU. The Go steps in
// GO_STEPS start together with them, one process each, because the slowest Go package (internal/flows,
// about 35 s) would otherwise be added to the Node time. The first failure stops everything that is
// still running and prints its output. The script sets no Go temporary directory: the caller's
// environment, GOTMPDIR included, reaches every child unchanged.
//
// Several scripts (extra*.mjs, flagship, pack, ux-batch5, render-baseline, ...) only export a run()
// that scripts/test-easydrag.mjs calls with its harness. Alone they check that the module imports and
// parses, and finish in well under a second.
import { spawn, spawnSync } from 'node:child_process';
import { readdirSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SELF = path.basename(fileURLToPath(import.meta.url));

// Part C appends './cmd/flowsdoc' here. Each entry: { label, args, env } (env is merged over the caller's).
export const GO_STEPS = [
    { label: 'go internal/flows', args: ['test', './internal/flows/...'], env: {} },
    { label: 'go internal/server (flow)', args: ['test', './internal/server', '-run', '(?i)flow'], env: {} },
    { label: 'go internal/tools (flow)', args: ['test', './internal/tools', '-run', '(?i)flow'], env: {} },
    { label: 'go ui (easydrag)', args: ['test', './ui', '-run', '(?i)easydrag'], env: {} },
];

const TAIL_LINES = 60;

function nodeJobs() {
    return readdirSync(path.join(ROOT, 'scripts'))
        .filter(name => /^test-easydrag.*\.mjs$/.test(name) && name !== SELF)
        .sort()
        .map(name => ({ label: 'node scripts/' + name, cmd: process.execPath, args: ['scripts/' + name], env: {} }));
}

function goJobs() {
    return GO_STEPS.map(step => ({ label: step.label, cmd: 'go', args: step.args, env: step.env || {} }));
}

function kill(child) {
    if (child.exitCode !== null || child.pid === undefined) return;
    if (process.platform === 'win32') {
        spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
        try { process.kill(-child.pid, 'SIGKILL'); } catch { child.kill('SIGKILL'); }
    }
}

function seconds(ms) { return (ms / 1000).toFixed(1) + ' s'; }

export async function main(argv) {
    const nodeOnly = argv.includes('--node-only');
    const started = Date.now();
    const queue = nodeJobs();
    const goQueue = nodeOnly ? [] : goJobs();
    const total = queue.length + goQueue.length;
    const running = new Set();
    let failed = null;
    let done = 0;

    const launch = job => new Promise(resolve => {
        const t0 = Date.now();
        const child = spawn(job.cmd, job.args, {
            cwd: ROOT,
            env: { ...process.env, ...job.env },
            detached: process.platform !== 'win32',
            windowsHide: true,
        });
        running.add(child);
        const chunks = [];
        child.stdout.on('data', d => chunks.push(d));
        child.stderr.on('data', d => chunks.push(d));
        child.on('error', err => { chunks.push(Buffer.from(String(err))); finish(-1); });
        child.on('close', code => finish(code));
        let over = false;
        function finish(code) {
            if (over) return;
            over = true;
            running.delete(child);
            if (failed) return resolve();
            done++;
            if (code === 0) {
                console.log('ok   ' + job.label + '  (' + seconds(Date.now() - t0) + ', ' + done + '/' + total + ')');
            } else {
                failed = { job, code, output: Buffer.concat(chunks).toString('utf8') };
                for (const other of running) kill(other);
            }
            resolve();
        }
    });

    // Go steps start at once; the Node scripts fill the remaining CPUs.
    const goRuns = goQueue.map(launch);
    const width = Math.max(1, os.availableParallelism() - goQueue.length);
    const worker = async () => {
        while (queue.length && !failed) await launch(queue.shift());
    };
    await Promise.all([...goRuns, ...Array.from({ length: width }, worker)]);

    const elapsed = seconds(Date.now() - started);
    if (failed) {
        const lines = failed.output.trimEnd().split('\n');
        console.log('\nFAIL ' + failed.job.label + ' (exit ' + failed.code + ')');
        console.log(lines.slice(-TAIL_LINES).join('\n'));
        console.log('\nT0 FAILED after ' + elapsed + ': ' + failed.job.label + ' (' + done + ' of ' + total + ' passed first)');
        return 1;
    }
    console.log('\nT0 passed: ' + total + ' steps' + (nodeOnly ? ' (Node only)' : '') + ' in ' + elapsed);
    return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
    process.exit(await main(process.argv.slice(2)));
}
```

- [ ] **Step 2: Add the npm script.** In `package.json`, directly after the `"test:easydrag": "node scripts/test-easydrag.mjs",` line, add:

```
    "test:easydrag:t0": "node scripts/test-easydrag-t0.mjs",
```

Check the file is still valid JSON: `node -e "JSON.parse(require('fs').readFileSync('package.json','utf8'))"` prints nothing.

- [ ] **Step 3: Run the Node half.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && node scripts/test-easydrag-t0.mjs --node-only | tail -3
```

Expected (times within a few seconds; the order of the `ok` lines varies):

```
ok   node scripts/test-easydrag-render-baseline.mjs  (2.2 s, 34/35)
ok   node scripts/test-easydrag.mjs  (17.8 s, 35/35)

T0 passed: 35 steps (Node only) in 21.1 s
```

The count is 35, not 36: the script leaves itself out.

- [ ] **Step 4: Run all of T0, forced and cached.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOFLAGS=-count=1 npm run test:easydrag:t0 2>&1 | tail -4
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && npm run test:easydrag:t0 2>&1 | tail -2
```

Expected: the first ends `ok   go internal/flows  (39.0 s, 39/39)` and `T0 passed: 39 steps in 39.0 s` (37 to 39 s; `GOFLAGS=-count=1` reaches the Go children through the caller's environment and defeats the test-result cache). The second, with the Go results cached, ends `T0 passed: 39 steps in 26.4 s`. Write both numbers down for the final report. If the forced run is above 60 s, stop and tell the coordinator: it means the machine is busy or the Go steps grew; the spec's fallback is then the decision (Deviations, point 1).

- [ ] **Step 5: See it fail the right way.** Make a script that fails, run T0, and remove it:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && echo "console.log('boom'); process.exit(3)" > scripts/test-easydrag-zz-fail.mjs && node scripts/test-easydrag-t0.mjs --node-only | tail -6; echo "exit ${PIPESTATUS[0]}"; rm scripts/test-easydrag-zz-fail.mjs
```

Expected: `FAIL node scripts/test-easydrag-zz-fail.mjs (exit 3)`, then `boom`, then `T0 FAILED after a few seconds: node scripts/test-easydrag-zz-fail.mjs (N of 36 passed first)`, then `exit 1`. `git status --short` shows only the new script and `package.json` again.

- [ ] **Step 6: Under Node 22 (what CI runs).**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && C:/Users/Andi/AppData/Local/nvm/v22.22.3/node.exe scripts/test-easydrag-t0.mjs --node-only | tail -2
```

Expected: `T0 passed: 35 steps (Node only) in 21.4 s`. If that Node is not installed, `nvm list` shows which 22.x is; any 22.x will do.

- [ ] **Step 7: Commit.** Write `C:/disposable/easydrag-s0c/b2.txt`:

```
test(easydrag): T0, one fast command for every non-browser check

scripts/test-easydrag-t0.mjs runs every scripts/test-easydrag*.mjs in
a process of its own (up to the CPU count) and the Go steps of the
GO_STEPS array (internal/flows, internal/server and internal/tools
with -run flow, ui with -run easydrag) at the same time, stops at the
first failure and prints its output and the total time. --node-only
runs the scripts only (for CI). It sets no environment variable.
Measured on 16 CPUs: 37 to 39 s with the Go cache warm and the tests
forced, 26 s with the Go results cached; run one after another the
Go steps alone are 73 s, so they run beside the Node scripts.
npm run test:easydrag:t0.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add scripts/test-easydrag-t0.mjs package.json && git commit -F C:/disposable/easydrag-s0c/b2.txt
```

### Task B.3: T0's Node scripts in the CI Node job (spec B1.3)

**Files:**
- Modify: `.github/workflows/security-gates.yml:52-53` (after the step `Test UI delivery regressions`)

Facts checked: the job `js-security` sets up Node `22` with `cache: npm` and runs `npm ci` first; the scripts import only `node:` modules and repository files (a grep of every `import` and `require` in `scripts/test-easydrag*.mjs` finds no bare package name); every file they read is tracked (a clean worktree of main passes); and the node-only run passed under Node 22.22 in Task B.2. The Go steps already run in the `go-security` job (`go test ./... -p 1`).

- [ ] **Step 1: Add the step.** Directly after

```
      - name: Test UI delivery regressions
        run: npm run test:ui-regressions
```

insert (two-space steps, the same indentation as its neighbours):

```
      - name: Test EasyDrag scripts (T0, Node only)
        run: node scripts/test-easydrag-t0.mjs --node-only
```

- [ ] **Step 2: Validate.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && node -e "const y=require('fs').readFileSync('.github/workflows/security-gates.yml','utf8');const i=y.indexOf('Test EasyDrag scripts');console.log(y.slice(i-120,i+120))" && git diff --stat
```

Expected: the printed text shows the new step directly below `npm run test:ui-regressions`; `git diff --stat` shows one file, 3 insertions (the step and its line, plus a blank line). No YAML parser is installed in the repo; if `python -c "import yaml"` works on your machine, `python -c "import yaml,sys;yaml.safe_load(open('.github/workflows/security-gates.yml'))"` must print nothing.

- [ ] **Step 3: Commit.** Write `C:/disposable/easydrag-s0c/b3.txt`:

```
ci(easydrag): run the EasyDrag Node scripts in the JS security job

The Node job ran only the UI delivery regressions. It now also runs
node scripts/test-easydrag-t0.mjs --node-only (35 scripts, about 21 s
on 16 CPUs). The scripts need nothing beyond Node 22 and the
checkout; the Go steps of T0 already run in the Go job.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add .github/workflows/security-gates.yml && git commit -F C:/disposable/easydrag-s0c/b3.txt
```

### Task B.4: The shared browser, `TestMain` and the guard (spec B2, B3 guard)

**Files:**
- Modify: `ui/destructive_flow_browser_test.go:3-15`, `:56-95` (split `newSmokeBrowserWith`)
- Modify: `ui/desktop_easydrag_browser_test.go:3-20`, `:349-424` (the helpers)
- Modify: `ui/desktop_easydrag_durable_browser_test.go:65-66`
- Create: `ui/main_test.go`, `ui/desktop_easydrag_helper_guard_test.go`
- Test: `go test ./ui -run '(?i)easydrag'` (T0 form) and the full gated run

Facts checked: `package ui` in all 769 test files (no `package ui_test`), no `TestMain` and no `t.Parallel` in the package's EasyDrag tests. rod v0.116.2: `func (b *Browser) Incognito() (*Browser, error)` (`browser.go:85`; it copies the browser and sets `BrowserContextID`); `Browser.Close()` on such a copy disposes the context only (`browser.go:178-183`); `Browser.Page` creates the tab inside the context (`:186`). The launcher's 180 s context bounds only the start (`Launch` does `defer l.ctxCancel()` and starts Chrome with a plain `exec.Command`), so a browser that lives for the whole process is safe. `newSmokeBrowser` has 160 files of callers in `ui/` and `newSmokeBrowserWith` 15; this task changes neither signature.

- [ ] **Step 1: IMPACT.**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "newSmokeBrowserWith" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Warn the coordinator on HIGH or CRITICAL (expected: many callers, all browser tests). Treat UNKNOWN as unresolved and confirm with `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers/ui && grep -ln "newSmokeBrowserWith(" *_test.go`, which lists the 15 files: `desktop_audio_player_`, `desktop_audit_`, `desktop_aura_drift_frame_`, `desktop_fritzbox_widget_`, `desktop_fruity_dock_`, `desktop_minigolf_`, `desktop_pinball_audio_render_`, `desktop_pinball_`, `desktop_pinball_survey_`, `desktop_theme_neutrality_`, `desktop_theme_sweep_`, `destructive_flow_`, `galaxa_expansion_`, `go2rtc_viewer_sound_`, `webgl_smoke_` `browser_test.go`. The refactor in Step 4 keeps the function's behaviour, messages and cleanup order, and Step 9 re-runs two of those callers.

- [ ] **Step 2: Write the guard first (red).** Create `ui/desktop_easydrag_helper_guard_test.go` with this content. It is the B3 source test: it runs without a browser, so T0 runs it.

```go
package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// easyDragDirectLaunchExceptions lists the EasyDrag browser test files that may launch a browser
// themselves, each with its reason. It is empty: every test opens its page through
// startEasyDragSmokeWith (or newEasyDragPage). Adding a file here needs a reason that the helper
// options (PrivateBrowser, Motion) cannot give.
var easyDragDirectLaunchExceptions = map[string]string{}

var easyDragDirectLaunch = regexp.MustCompile(`\blauncher\.New\b|\brod\.New\b|\bnewSmokeBrowser(?:With)?\b`)

// easyDragHelperSection is the part of desktop_easydrag_browser_test.go between the helper markers.
var easyDragHelperSection = regexp.MustCompile(`(?s)// easydrag-helpers:begin.*?// easydrag-helpers:end`)

// easyDragDirectLaunches returns the lines of src (comments cut) that launch a browser directly.
func easyDragDirectLaunches(src string) []string {
	var found []string
	for i, line := range strings.Split(src, "\n") {
		if cut := strings.Index(line, "//"); cut >= 0 {
			line = line[:cut]
		}
		if easyDragDirectLaunch.MatchString(line) {
			found = append(found, strings.TrimSpace(line)+" (line "+strconv.Itoa(i+1)+")")
		}
	}
	return found
}

// TestEasyDragBrowserTestsUseTheSmokeHelper keeps every EasyDrag browser test on the shared
// Chrome, with reduced motion set by the helper: no such file launches a browser itself, apart
// from the helper section of desktop_easydrag_browser_test.go and the listed exceptions. It reads
// source only, so it runs without a browser (T0).
func TestEasyDragBrowserTestsUseTheSmokeHelper(t *testing.T) {
	// The scanner itself.
	if got := easyDragDirectLaunches("p := newSmokeBrowser(t).MustPage()\nb := rod.New()\nl := launcher.New() // fine\n// rod.New() in a comment\nx := 1"); len(got) != 3 {
		t.Fatalf("scanner found %d launches in the sample, want 3: %v", len(got), got)
	}

	files, err := filepath.Glob("*easydrag*_browser_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no EasyDrag browser test files found (%v)", err)
	}
	sort.Strings(files)
	for file, reason := range easyDragDirectLaunchExceptions {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("exception %s has no reason", file)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("exception %s names a file that does not exist", file)
		}
	}
	const helperFile = "desktop_easydrag_browser_test.go"
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		if file == helperFile {
			if len(easyDragHelperSection.FindAllString(src, -1)) != 1 {
				t.Fatalf("%s must hold exactly one easydrag-helpers:begin ... easydrag-helpers:end section", file)
			}
			src = easyDragHelperSection.ReplaceAllString(src, "")
		}
		if _, listed := easyDragDirectLaunchExceptions[file]; listed {
			continue
		}
		for _, hit := range easyDragDirectLaunches(src) {
			t.Errorf("%s launches a browser itself: %s; use startEasyDragSmokeWith (PrivateBrowser: true for an own Chrome)", file, hit)
		}
	}
}
```

The file name does not match `*easydrag*_browser_test.go`, so the guard does not scan itself.

- [ ] **Step 3: Run it and see it fail for the stated reason.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragBrowserTestsUseTheSmokeHelper' -count=1 2>&1 | tail -5
```

Expected: `FAIL`, `desktop_easydrag_helper_guard_test.go:..: desktop_easydrag_browser_test.go must hold exactly one easydrag-helpers:begin ... easydrag-helpers:end section`. (After the markers exist, the guard would name `desktop_easydrag_durable_browser_test.go`: `launches a browser itself: p := newSmokeBrowser(t).MustPage() ...`.)

- [ ] **Step 4: Split `newSmokeBrowserWith` so the browser no longer belongs to a test.** In `ui/destructive_flow_browser_test.go`, add `"fmt"` to the imports (after `"context"`), and replace the whole body of `newSmokeBrowserWith` (from `func newSmokeBrowserWith` to its closing brace, `newSmokeBrowser` above it stays) with:

```go
func newSmokeBrowserWith(t *testing.T, tweak func(*launcher.Launcher)) *rod.Browser {
	t.Helper()

	browser, stop, failure := launchSmokeBrowser(tweak)
	if failure != nil {
		if failure.unavailable {
			browserSmokeUnavailable(t, "%s", failure.message)
		}
		t.Fatal(failure.message)
	}
	t.Cleanup(stop)
	return browser
}

// smokeBrowserFailure says why launchSmokeBrowser gave no browser. unavailable means Chrome is
// missing or would not start: a test skips then, unless AURAGO_RUN_BROWSER_SMOKE=1 demands it.
type smokeBrowserFailure struct {
	message     string
	unavailable bool
}

// launchSmokeBrowser starts a headless Chrome the way every smoke test does and returns it with a
// stop function (close the browser, kill the process, cancel the launch context). It ties nothing
// to a *testing.T, so the shared EasyDrag browser can outlive the test that started it.
func launchSmokeBrowser(tweak func(*launcher.Launcher)) (*rod.Browser, func(), *smokeBrowserFailure) {
	bin, ok := browserExecutable()
	if !ok {
		return nil, nil, &smokeBrowserFailure{message: "headless browser smoke test requires Chrome or Edge (set CHROME_BIN)", unavailable: true}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	l := launcher.New().
		Context(ctx).
		Bin(bin).
		Headless(true).
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage")
	kill := func() {}
	if runtime.GOOS == "windows" {
		// Rod's leakless helper can hang before publishing the child PID on Windows.
		l.Leakless(false)
		kill = l.Kill
	}
	if os.Getenv("AURAGO_BROWSER_MUTE_AUDIO") == "1" {
		l.Set("mute-audio")
	}
	if tweak != nil {
		tweak(l)
	}
	u, err := l.Launch()
	if err != nil {
		kill()
		cancel()
		return nil, nil, &smokeBrowserFailure{message: fmt.Sprintf("headless browser launch failed: %v", err), unavailable: true}
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		kill()
		cancel()
		return nil, nil, &smokeBrowserFailure{message: fmt.Sprintf("connect headless browser for UI smoke test: %v", err)}
	}
	return browser, func() { _ = browser.Close(); kill(); cancel() }, nil
}
```

The messages, the skip-or-fail rule and the order of the cleanups (close, kill, cancel) are the old ones. This file is not an EasyDrag file, so the guard does not read it.

- [ ] **Step 5: The helpers.** In `ui/desktop_easydrag_browser_test.go`, add `"sync"` to the imports (between `"strings"` and `"testing"`); `proto`, `rod` and `time` are imported already. Then, with the Edit tool, replace this block (the struct and the first lines of `startEasyDragSmoke`):

```go
// easyDragSmoke holds the page and the screenshot folder of TestDesktopEasyDragBrowser.
type easyDragSmoke struct {
	t    *testing.T
	page *rod.Page
	dir  string
}

// startEasyDragSmoke serves the desktop shell with the aurora and EasyDrag fixtures and opens
// EasyDrag in a 1440×900 page.
func startEasyDragSmoke(t *testing.T, extraFixtures ...string) *easyDragSmoke {
	t.Helper()
```

with the following. The line after it in the file, `html := regexp.MustCompile(...)`, stays as it is. (Motion is declared but still unused: Task B.5 gives it its meaning.)

```go
// easydrag-helpers:begin (TestEasyDragBrowserTestsUseTheSmokeHelper allows browser launches in this section only)

// easyDragSmoke holds the page and the screenshot folder of TestDesktopEasyDragBrowser.
type easyDragSmoke struct {
	t    *testing.T
	page *rod.Page
	dir  string
}

// easyDragSmokeOptions changes how startEasyDragSmokeWith opens the editor.
type easyDragSmokeOptions struct {
	Fixtures       []string // extra fixture scripts, as startEasyDragSmoke's variadic argument
	PrivateBrowser bool     // an own Chrome instead of the shared one (launch flags, browser-wide state)
	Motion         bool     // leave prefers-reduced-motion alone; the test sets it itself
	Width, Height  int      // the viewport; 0 means 1440×900
}

// sharedEasyDragChrome is the one Chrome of the go test process. TestMain closes it.
var sharedEasyDragChrome struct {
	once    sync.Once
	browser *rod.Browser
	stop    func()
	failure *smokeBrowserFailure
}

// sharedEasyDragBrowser returns the Chrome that every EasyDrag browser test shares, started on
// first use. Tests take a fresh incognito context of it (newEasyDragPage) and never close it.
func sharedEasyDragBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	c := &sharedEasyDragChrome
	c.once.Do(func() { c.browser, c.stop, c.failure = launchSmokeBrowser(nil) })
	if c.failure != nil {
		if c.failure.unavailable {
			browserSmokeUnavailable(t, "%s", c.failure.message)
		}
		t.Fatal(c.failure.message)
	}
	return c.browser
}

// closeSharedEasyDragBrowser is TestMain's end of sharedEasyDragBrowser; it does nothing when no
// test started the browser.
func closeSharedEasyDragBrowser() {
	if sharedEasyDragChrome.stop != nil {
		sharedEasyDragChrome.stop()
	}
}

// newEasyDragPage opens a blank page in a fresh incognito context of the shared browser (or in an
// own Chrome with PrivateBrowser) and sets the viewport. The page and its context close in
// t.Cleanup, the page first.
func newEasyDragPage(t *testing.T, opts easyDragSmokeOptions) *rod.Page {
	t.Helper()
	var browser *rod.Browser
	if opts.PrivateBrowser {
		browser = newSmokeBrowser(t)
	} else {
		incognito, err := sharedEasyDragBrowser(t).Incognito()
		if err != nil {
			t.Fatalf("open an incognito context in the shared browser: %v", err)
		}
		t.Cleanup(func() { _ = incognito.Close() })
		browser = incognito
	}
	page := browser.MustPage().Timeout(150 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	width, height := opts.Width, opts.Height
	if width == 0 {
		width = 1440
	}
	if height == 0 {
		height = 900
	}
	page.MustSetViewport(width, height, 1, false)
	return page
}

// startEasyDragSmoke serves the desktop shell with the aurora and EasyDrag fixtures and opens
// EasyDrag in a 1440×900 page.
func startEasyDragSmoke(t *testing.T, extraFixtures ...string) *easyDragSmoke {
	t.Helper()
	return startEasyDragSmokeWith(t, easyDragSmokeOptions{Fixtures: extraFixtures})
}

// startEasyDragSmokeWith is startEasyDragSmoke with options.
func startEasyDragSmokeWith(t *testing.T, opts easyDragSmokeOptions) *easyDragSmoke {
	t.Helper()
	extraFixtures := opts.Fixtures
```

Then replace these four lines at the end of the function (the old launch, viewport and navigation) and close the helper section after the function:

```go
	page := newSmokeBrowser(t).MustPage().Timeout(150 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1440, 900, 1, false)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('easydrag');}`)
	return &easyDragSmoke{t: t, page: page, dir: dir}
}
```

with

```go
	page := newEasyDragPage(t, opts)
	page.MustNavigate(srv.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('easydrag');}`)
	return &easyDragSmoke{t: t, page: page, dir: dir}
}

// easydrag-helpers:end
```

Then `gofmt -w ui/desktop_easydrag_browser_test.go` (the Edit tool can drop the space after `html :=`; gofmt restores it) and check `gofmt -l ui/desktop_easydrag_browser_test.go` prints nothing.

- [ ] **Step 6: The config test of the durable file takes the helper's page.** In `ui/desktop_easydrag_durable_browser_test.go`, `TestConfigEasyDragDurableSwitchBrowser` opens `/config`, not the EasyDrag fixture, and sets no launch flag, so it takes a page of the shared browser. Replace

```go
	p := newSmokeBrowser(t).MustPage()
	defer p.MustClose()
```

with

```go
	p := newEasyDragPage(t, easyDragSmokeOptions{})
```

(the helper closes the page in `t.Cleanup`; the test sets its own viewport afterwards, as before).

- [ ] **Step 7: `TestMain`.** Create `ui/main_test.go`:

```go
package ui

import (
	"os"
	"testing"
)

// TestMain runs the package's tests unchanged and then closes the Chrome that the EasyDrag browser
// tests share (sharedEasyDragBrowser). It does nothing when no test started that browser.
func TestMain(m *testing.M) {
	code := m.Run()
	closeSharedEasyDragBrowser()
	os.Exit(code)
}
```

- [ ] **Step 8: Build and run the source test and the T0 form of the package.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/main_test.go ui/destructive_flow_browser_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_durable_browser_test.go ui/desktop_easydrag_helper_guard_test.go && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
```

Expected: `go vet` and `gofmt -l` print nothing, then `ok  	aurago/ui	2.4s` (the browser tests skip; the guard passes). `gofmt -l ui/*.go` also lists four files that were already unformatted on main (`desktop_aura_drift_test.go`, `desktop_media_gallery_assets_test.go`, `desktop_video_studio_keyframes_test.go`, `desktop_video_studio_navigation_test.go`); leave them.

- [ ] **Step 9: Run the gated suite and two unrelated browser callers.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run 'TestContainersDestructiveDeleteFlowBrowserSmoke|TestWebGL' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee C:/disposable/easydrag-s0c/browser-shared.log | grep -E '^(--- (FAIL|SKIP)|ok|FAIL|panic)'
```

Expected: the first prints `--- PASS: TestContainersDestructiveDeleteFlowBrowserSmoke (2.3s)` and `ok` (the `TestWebGL` pattern matches nothing today; the Containers test goes through the refactored `newSmokeBrowser`). The second prints the one SKIP line of the baseline and `ok  	aurago/ui	259.3s` (measured: 259 s against the baseline's 300 s; each EasyDrag test saves one to four seconds of Chrome start). Then `grep -c '^--- PASS' C:/disposable/easydrag-s0c/browser-shared.log` prints `34`: the 33 of the baseline plus the guard. No `--- FAIL`. A test that fails here and passes alone leaks state between tests: re-run it alone first (`-run` with the test's exact name, anchored with `^` and a closing dollar sign); if it only fails in the suite, pass `PrivateBrowser: true` to `startEasyDragSmokeWith` for it (that goes through the helper, so the guard needs no exception) and say why in a comment.

- [ ] **Step 10: Commit.** Write `C:/disposable/easydrag-s0c/b4.txt`:

```
test(easydrag): one shared Chrome for the EasyDrag browser tests

startEasyDragSmoke now takes its page from sharedEasyDragBrowser (one
Chrome per go test process, started on first use) in a fresh incognito
context that closes in t.Cleanup, instead of launching Chrome per test.
startEasyDragSmokeWith takes options (fixtures, PrivateBrowser, Motion,
viewport). launchSmokeBrowser is newSmokeBrowserWith without the
per-test cleanup, and newSmokeBrowserWith calls it with the same
messages and cleanup order. A new TestMain closes the shared browser
after m.Run and changes nothing else. The durable config test opens its
page through the helper. TestEasyDragBrowserTestsUseTheSmokeHelper
reads every ui/*easydrag*_browser_test.go and fails on launcher.New,
rod.New or newSmokeBrowser outside the helper section. The gated
EasyDrag run takes 259 s instead of 300 s.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add ui/main_test.go ui/destructive_flow_browser_test.go ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_durable_browser_test.go ui/desktop_easydrag_helper_guard_test.go && git commit -F C:/disposable/easydrag-s0c/b4.txt
```

### Task B.5: Reduced motion through one helper (spec B3)

**Files:**
- Modify: `ui/desktop_easydrag_browser_test.go` (the helper section from Task B.4)
- Modify: `ui/desktop_easydrag_credentials_browser_test.go`, `ui/desktop_easydrag_credentials_purge_browser_test.go`, `ui/desktop_easydrag_test_mode_browser_test.go`, `ui/desktop_easydrag_colour_browser_test.go`, `ui/desktop_easydrag_contact_sheet_browser_test.go`, `ui/desktop_easydrag_legibility_browser_test.go`
- Create: `ui/desktop_easydrag_motion_browser_test.go`

rod v0.116.2 type names, checked in `lib/proto/emulation.go`: `proto.EmulationSetEmulatedMedia{Media string, Features []*proto.EmulationMediaFeature}` (line 528) with `proto.EmulationMediaFeature{Name, Value string}` (line 83); `.Call(page)` works because `*rod.Page` is a `proto.Client`, as the existing inline code shows.

- [ ] **Step 1: The failing test.** Create `ui/desktop_easydrag_motion_browser_test.go`:

```go
package ui

import "testing"

// TestEasyDragSmokeSetsReducedMotion: a page that startEasyDragSmoke opens reports
// prefers-reduced-motion: reduce, so no test depends on the machine's animation setting.
func TestEasyDragSmokeSetsReducedMotion(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	if !s.page.MustEval(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`).Bool() {
		t.Fatal("the page does not report prefers-reduced-motion: reduce")
	}
}
```

- [ ] **Step 2: See it fail.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run 'TestEasyDragSmokeSetsReducedMotion' -count=1 -v 2>&1 | grep -E '^(---|\s+desktop_easydrag_motion|ok|FAIL)'
```

Expected: `the page does not report prefers-reduced-motion: reduce`, `--- FAIL`. (Windows' "Show animations" setting maps to this media query, so a machine with animations switched off passes without the helper; then go on, the helper makes the test independent of it.)

- [ ] **Step 2b: The helper.** In the helper section of `ui/desktop_easydrag_browser_test.go`, add before `newEasyDragPage`:

```go
// setEasyDragReducedMotion makes the page report prefers-reduced-motion: reduce, so canvas
// animations end at once and screenshots do not depend on timing.
func setEasyDragReducedMotion(t *testing.T, page *rod.Page) {
	t.Helper()
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "reduce"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(page); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
}
```

and in `newEasyDragPage`, directly after `page.MustSetViewport(width, height, 1, false)`:

```go
	if !opts.Motion {
		setEasyDragReducedMotion(t, page)
	}
```

Update the doc comment of `newEasyDragPage` to end `... sets the viewport and, unless Motion is set, reduced motion.` A test that checks motion passes `Motion: true` and then sets both values itself (none does today).

- [ ] **Step 3: Remove the seven inline settings.** Each is now redundant. With the Edit tool (use `replace_all` where a block occurs twice):
  - In `desktop_easydrag_credentials_browser_test.go`, `desktop_easydrag_credentials_purge_browser_test.go` and `desktop_easydrag_test_mode_browser_test.go` (twice, `replace_all: true`), delete exactly these three lines (tab-indented):

    ```go
    	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
    		t.Fatal(err)
    	}
    ```

    The credentials and purge files then no longer use `proto`: replace their import block (`import (` `"testing"` blank line `"github.com/go-rod/rod/lib/proto"` `)`) with `import "testing"`. `test_mode` still uses `proto.Accessibility...`; leave its imports. The purge test keeps its `s.wait(matchMedia ... reduce)` line: it now asserts the helper's work.
  - In `desktop_easydrag_colour_browser_test.go`, `desktop_easydrag_contact_sheet_browser_test.go` and `desktop_easydrag_legibility_browser_test.go`, delete the line `	setReducedMotion(t, s.page, "reduce")` (the function itself stays, the screensaver tests use it).

- [ ] **Step 4: Build, then run the seven touched tests and the new one.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' go vet ./ui && gofmt -l ui/desktop_easydrag_*.go && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(?i)(EasyDragSmokeSetsReducedMotion|EasyDragCredentialMoveBrowser|EasyDragCredentialsPurgeBrowser|EasyDragTestModeBrowser|EasyDragGoalCounterBrowser|EasyDragColourRolesBrowser|EasyDragLegibilityBrowser)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: `go vet` and `gofmt -l` print nothing; 7 `--- PASS` lines (the test-mode file holds two tests) and `ok`. The contact-sheet test is skipped by its own gate and is checked by the full run.

- [ ] **Step 5: The full gated run, and compare with the baseline.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee C:/disposable/easydrag-s0c/browser-motion.log | grep -E '^(--- (FAIL|SKIP)|ok|FAIL|panic)'; grep -c '^--- PASS' C:/disposable/easydrag-s0c/browser-motion.log
```

Expected: one SKIP (the contact sheet), `ok  	aurago/ui	~260s`, no FAIL, and `35` PASS lines. A test that now fails and passed before is a test that relied on motion; re-run it alone, and if it only passes with motion, give it `Motion: true` through `startEasyDragSmokeWith` and set the values it needs inside the test.

- [ ] **Step 6: The guard still passes in T0 form.** `cd .../easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1` prints `ok`.

- [ ] **Step 7: Commit.** Write `C:/disposable/easydrag-s0c/b5.txt`:

```
test(easydrag): set reduced motion once, in the smoke helper

startEasyDragSmokeWith calls setEasyDragReducedMotion
(Emulation.setEmulatedMedia, prefers-reduced-motion: reduce) on every
page unless the test passes Motion: true. The seven tests that set it
by hand (credentials, credentials purge, test mode twice, colour
roles, contact sheet, legibility) lose their own call.
TestEasyDragSmokeSetsReducedMotion pins the default.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_motion_browser_test.go ui/desktop_easydrag_credentials_browser_test.go ui/desktop_easydrag_credentials_purge_browser_test.go ui/desktop_easydrag_test_mode_browser_test.go ui/desktop_easydrag_colour_browser_test.go ui/desktop_easydrag_contact_sheet_browser_test.go ui/desktop_easydrag_legibility_browser_test.go && git commit -F C:/disposable/easydrag-s0c/b5.txt
```

### Task B.6: The A5 target-size test (spec §3.5), after Part A has landed

**Start only after the coordinator says Part A is on main.** Then:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git status --short && git rebase main
```

`git status --short` must print nothing (everything of B.1 to B.5 is committed). The rebase replays B.1 to B.5 on top of Part A; the files do not overlap (Part A owns the CSS, the JS and the locales; this part owns scripts, `package.json`, the workflow and `ui/*_test.go`). If a conflict appears, stop and tell the coordinator.

**Files:**
- Create: `ui/desktop_easydrag_targets_browser_test.go`
- Modify: `ui/css/desktop-app-easydrag.css` (only the offenders the test names beyond Part A's two; measured below)
- Test: `TestEasyDragTargetSizes` (gated)

**What the test found on main (measured while planning).** Run against main with the scan below, before any CSS change, it reports 12 offenders in two states, at 1440 by 900:

| State | Offender | Size | Cause |
|---|---|---|---|
| start page | `button[data-ed-filter]` x5 and `button[data-ed-library-view]` x2 | height 22 | `.ed-seg--small button { min-height: 22px }` (A5, `desktop-app-easydrag.css:356`) |
| editor | `button.ed-port-add` x4 | measured 11.4 by 11.4 | `.ed-port-add` is 22 by 22 (A5, `:754`) and scaled to 0.6 until hover; the scan uses the layout box |
| editor | `a.ed-chip.ed-chip--warn.ed-palette-setup[data-ed-setup]` | 72 by 20 | **a third offender, not in A5**: the setup link next to a palette entry (`:1554`, the chip's `min-height: 20px` at `:385`) |

With Part A's two fixes applied the start page is clean (the scan measures `.ed-switch--small` through its `::before`, 34 by 25, and `.ed-flow-select input` through its 44 px label, one each) and the editor lists only the palette setup link. So this task adds one CSS rule. Coordinator: if Part A already carries the third rule (recommended, tell Part A before it lands), Step 4 finds nothing to fix and only the test is committed.

- [ ] **Step 1: Write the test.** Create `ui/desktop_easydrag_targets_browser_test.go`. The scan lives in a Go constant so a failing run names selector, class list and size; it opens its page through `startEasyDragSmoke`, so the guard of Task B.4 stays green.

```go
package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// easyDragTargetScan measures every pointer target inside the EasyDrag windows (WCAG 2.5.8, 24 CSS
// px). Two kinds of control are measured by their real target instead of their own box, and the
// reason sits next to each:
//   - .ed-switch--small is drawn 30x17, and its ::before widens the target to 34x25;
//   - .ed-flow-select input is a 20x20 box inside a 44 px label, and the label is the target.
//
// Skipped: hidden controls, controls with pointer-events:none, and a link that sits inline in
// running text (the text around it is not part of the link).
const easyDragTargetScan = `()=>{
	const MIN=24;
	const exceptions=[
		{selector:'.ed-switch--small',how:'before'},
		{selector:'.ed-flow-select input',how:'label'}
	];
	const squash=s=>s.replace(/\s+/g,' ').trim();
	const offenders=[];
	const viaTarget={};
	const seen=new Set();
	document.querySelectorAll('.ed-app').forEach(app=>{
		app.querySelectorAll('button,a[href],input,select,[role="button"]').forEach(el=>{
			if(seen.has(el))return;
			seen.add(el);
			if(el.matches('input[type="hidden"]'))return;
			const cs=getComputedStyle(el),r=el.getBoundingClientRect();
			if(cs.display==='none'||cs.visibility==='hidden'||cs.pointerEvents==='none'||(r.width===0&&r.height===0))return;
			if(el.matches('a[href]')&&cs.display==='inline'){
				const p=el.parentElement;
				if(p&&squash(p.textContent)!==squash(el.textContent))return;
			}
			// offsetWidth/offsetHeight are the layout box: the hover reveal of .ed-port-add scales it to 0.6 until the pointer arrives.
			let w=Math.max(r.width,el.offsetWidth),h=Math.max(r.height,el.offsetHeight);
			const ex=exceptions.find(e=>el.matches(e.selector));
			if(ex)viaTarget[ex.selector]=(viaTarget[ex.selector]||0)+1;
			if(ex&&ex.how==='before'){
				const b=getComputedStyle(el,'::before');
				w=parseFloat(b.width)||0;
				h=parseFloat(b.height)||0;
			}
			if(ex&&ex.how==='label'){
				const label=el.closest('label');
				if(label){const lr=label.getBoundingClientRect();w=lr.width;h=lr.height;}
			}
			if(w>=MIN-0.01&&h>=MIN-0.01)return;
			const data=[...el.attributes].map(a=>a.name).find(n=>n.startsWith('data-ed'))||'';
			const classes=typeof el.className==='string'?el.className.trim():'';
			offenders.push({
				selector:el.tagName.toLowerCase()+(classes?'.'+classes.split(/\s+/).join('.'):'')+(data?'['+data+']':''),
				classes:classes,
				width:Math.round(w*10)/10,
				height:Math.round(h*10)/10
			});
		});
	});
	return JSON.stringify({scanned:seen.size,viaTarget:viaTarget,offenders:offenders});
}`

// checkTargets fails the test when a visible target in an EasyDrag window is under 24x24 CSS px.
func (s *easyDragSmoke) checkTargets(step string) {
	s.t.Helper()
	var result struct {
		Scanned   int            `json:"scanned"`
		ViaTarget map[string]int `json:"viaTarget"`
		Offenders []struct {
			Selector string  `json:"selector"`
			Classes  string  `json:"classes"`
			Width    float64 `json:"width"`
			Height   float64 `json:"height"`
		} `json:"offenders"`
	}
	if err := json.Unmarshal([]byte(s.page.MustEval(easyDragTargetScan).Str()), &result); err != nil {
		s.t.Fatalf("%s: read the target scan: %v", step, err)
	}
	if result.Scanned < 10 {
		s.t.Fatalf("%s: the scan found only %d controls; it no longer sees the EasyDrag window", step, result.Scanned)
	}
	s.t.Logf("%s: %d controls scanned, measured by their real target: %v", step, result.Scanned, result.ViaTarget)
	var report []string
	for _, o := range result.Offenders {
		report = append(report, o.Selector+" is "+formatTargetSize(o.Width, o.Height)+" (classes: "+o.Classes+")")
	}
	if len(report) > 0 {
		s.t.Errorf("%s: %d of %d controls are under 24x24 CSS px:\n  %s", step, len(report), result.Scanned, strings.Join(report, "\n  "))
	}
}

func formatTargetSize(w, h float64) string {
	return strconv.FormatFloat(w, 'f', -1, 64) + "x" + strconv.FormatFloat(h, 'f', -1, 64)
}

// TestEasyDragTargetSizes is the A5 check (U9, V4): on the start page and in the editor of the
// seeded flow at 1440x900, every button, link, input, select and role=button in the EasyDrag window
// is at least 24x24 CSS px, apart from the two documented exceptions in easyDragTargetScan.
func TestEasyDragTargetSizes(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && document.querySelectorAll('.ed-template-card').length===3`)
	s.checkTargets("start page")
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.checkTargets("editor")
	s.failOnPageErrors("target sizes")
}
```

- [ ] **Step 2: Run it.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && gofmt -l ui/desktop_easydrag_targets_browser_test.go && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run 'TestEasyDragTargetSizes' -count=1 -v 2>&1 | tail -14 | cut -c1-220
```

Expected with Part A's two fixes on main: the start page logs `35 controls scanned, measured by their real target: map[.ed-flow-select input:1 .ed-switch--small:1]` and passes; the editor logs `89 controls scanned, measured by their real target: map[]` and fails with exactly one line, `a.ed-chip.ed-chip--warn.ed-palette-setup[data-ed-setup] is 72x20 (classes: ed-chip ed-chip--warn ed-palette-setup)`.

If the output lists the `.ed-seg--small` buttons or `.ed-port-add` too, Part A's A5 CSS is not on main: stop and tell the coordinator, do not patch it here. If it lists an offender that is not in the table above, treat it like the palette link in Step 4: one CSS fix, listed in the commit message.

- [ ] **Step 3: IMPACT.** Not needed: this task changes a CSS rule and adds a test, no function. Note the selectors of the offenders for the report.

- [ ] **Step 4: Fix each offender in the CSS, here, in this commit.** For the palette setup link, in `ui/css/desktop-app-easydrag.css` change

```css
.ed-palette-setup { margin-right: 6px; text-decoration: none; }
```

to

```css
.ed-palette-setup { margin-right: 6px; min-height: 24px; text-decoration: none; }
```

The chip is `display: inline-flex; align-items: center`, so the text stays centred and the chip is 4 px taller in the palette row (the row already aligns its children to the centre). Another offender gets the same kind of rule at the selector the test names, using `min-width` or `min-height` of 24 px, never a change that makes the visible control larger than it needs to be (a pseudo-element hit area, like `.ed-switch--small::before`, is the pattern for a control that must keep its look).

- [ ] **Step 5: Run again, then the CSS and render gates.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(TestEasyDragTargetSizes|TestDesktopEasyDragBrowser$|TestDesktopEasyDragLegibilityBrowser|TestEasyDragStylesheet)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && node scripts/test-easydrag-render-baseline.mjs
```

Expected: 4 `--- PASS` (the target test, `TestDesktopEasyDragBrowser` (34 s), the legibility test and the stylesheet-token test), `ok`, and `PASS render baseline (unchanged)`. Open `D:/tmp/easydrag-s0c-shots/editor-standard.png` and look at the palette: the setup chip is a few pixels taller and still sits on the row's centre.

- [ ] **Step 6: Whole gated run and T0.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee C:/disposable/easydrag-s0c/browser-final.log | grep -E '^(--- (FAIL|SKIP)|ok|FAIL|panic)'; grep -c '^--- PASS' C:/disposable/easydrag-s0c/browser-final.log
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && npm run test:easydrag:t0 2>&1 | tail -2
```

Expected: one SKIP, `ok`, 36 PASS lines, then `T0 passed: N steps in NN s` (N is 39 plus what Parts A and C added).

- [ ] **Step 7: Commit.** Write `C:/disposable/easydrag-s0c/b6.txt` (list the offenders the run really found, with sizes; the text below is the expected one):

```
test(easydrag): target sizes of the EasyDrag window (A5, U9/V4)

TestEasyDragTargetSizes (gated) scans the start page and the editor of
the seeded flow at 1440x900: every visible button, a[href], input,
select and [role=button] inside the EasyDrag window is at least 24x24
CSS px. The two documented exceptions are measured by their real
target: .ed-switch--small by its ::before hit area (34x25) and
.ed-flow-select input by its 44 px label. Links inline in running text
and hidden or pointer-events:none controls are skipped. On main it
found 12 offenders: .ed-seg--small button (22 px high, 7 on the start
page), .ed-port-add (22x22, 4 in the editor), both fixed by A5, and
one more, the palette's setup link a.ed-palette-setup (72x20), fixed
here with min-height: 24px.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <model line>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-tiers && git add ui/desktop_easydrag_targets_browser_test.go ui/css/desktop-app-easydrag.css && git commit -F C:/disposable/easydrag-s0c/b6.txt
```

### Task B.7: The report to the coordinator (no commit)

No part edits the spec or the roadmap; the coordinator records these on main when the part lands. Send, in the final report of Part B:

- the T0 numbers measured in Task B.2 Step 4 (forced and cached) and the Go-step times of the table above, for the spec's "target under 60 s" line;
- the stale-script table (all 35 pass, the F6 trim, no product-code item);
- the gated EasyDrag run: 300 s before, 259 s after Task B.4, 36 PASS and 1 SKIP at the end;
- the target-size findings of Task B.6 (12 offenders, three CSS rules) for ledger row 10;
- the deviations below and the decisions the coordinator took.

### Coverage

| Spec item | Task |
|---|---|
| §4.1 each stale script runs once, fix or delete | planning run (table), B.1 |
| §4.1 T0 script, `test:easydrag:t0`, `--node-only`, first failure, total time, no Go temp dir | B.2 |
| §4.1 target under 60 s, measured | table above, B.2 Step 4 |
| §4.1 CI Node job | B.3 |
| §4.2 shared browser, incognito context per test, `TestMain`, private-browser option, durable file | B.4 |
| §4.3 reduced motion on every page, opt-in `Motion` | B.5 |
| §4.3 guard test (source, T0) | B.4 |
| §3.5 / §8 target-size test, exceptions with reasons | B.6 |
| Brief contract: `easyDragSmokeOptions`, `startEasyDragSmokeWith`, `sharedEasyDragBrowser`, `setEasyDragReducedMotion`, `GO_STEPS` | B.4, B.5, B.2 |

### Deviations from the spec

1. **T0 runs the Go steps at the same time as the Node scripts** (spec §4.1 and the brief: Node first, then the Go steps in order). Reason: measured one after the other, the warm total is 94 s (21 s Node plus 73 s Go), above the 60 s target; the spec's fallback (move the slowest Go step, `internal/flows`, to T1) would give about 58 s, two seconds under the target, and would take the A1 and A3 flows tests out of T0, which the contracts between the parts rely on. Run together, with every step in T0, the measured total is 37 to 39 s forced and 26 s cached. The script still reports the first failure and keeps `GO_STEPS` as one ordered array. Decision for the coordinator, below.
2. **An extra change to a script that passes** (`test-easydrag-ux-batch5.mjs`, Task B.1): the spec only plans fixes for failing scripts, but `test-easydrag.mjs` takes 223 to 249 s, so no T0 can meet 60 s without it.
3. **A refactor outside the EasyDrag files** (`newSmokeBrowserWith` in `ui/destructive_flow_browser_test.go` splits into `launchSmokeBrowser` and a wrapper), because the shared browser must not hang its cleanup on the test that starts it. The wrapper keeps messages, skip rule and cleanup order for the 15 callers.
4. **One helper beyond the contract**, `newEasyDragPage(t, opts)`, because `TestConfigEasyDragDurableSwitchBrowser` needs a shared-browser page that is not the EasyDrag fixture. The contract's four names and fields are exactly as the brief gives them.
5. **The guard's helper section is marked by two comment lines** (`// easydrag-helpers:begin` and `:end`) in `desktop_easydrag_browser_test.go`; the exception map starts empty.
6. **Target-size scope and measure:** "inside the EasyDrag window" is read as inside `.ed-app` (the shell's own window buttons and context menus are not EasyDrag CSS); the size is the larger of the box and the layout box, because `.ed-port-add` is scaled to 0.6 until hover; the two exceptions are checked by their real target instead of being skipped blindly. Beyond the two A5 offenders the test found a third (`.ed-palette-setup`), fixed in Task B.6.
7. **No `-count=1` in `GO_STEPS`:** the Go result cache makes an unchanged Go tree cost about 10 s; the caller forces a full run with `GOFLAGS=-count=1`.

### Decisions I could not make alone (recommendation first)

- **Concurrent T0 (deviation 1).** Recommend concurrent, as planned. If the owner wants the spec's literal order, the change is one loop in `main()`, and T0 then reads 94 s warm; the spec's number would have to be changed to match.
- **What "warm" means for the 60 s line.** Recommend recording both numbers (37 to 39 s with tests forced, 26 s with Go results cached) and calling the target met.
- **The third A5 rule.** Recommend telling Part A to add `.ed-palette-setup { min-height: 24px }` with its two A5 rules; then Task B.6 only commits the test. Either way the task works.
- **`GOTMPDIR` for the `internal/server` step.** The memory note says a temp dir on D: breaks a cross-drive `filepath.Rel` there. On this machine `GOTMPDIR=D:\tmp\go-build` is set globally and `go test ./internal/server -run '(?i)flow'` passed with it (cold run), so T0 passes the caller's environment through unchanged. If the owner's shell ever shows that failure, the caller runs T0 with `GOTMPDIR` unset (`env -u GOTMPDIR npm run test:easydrag:t0`), which costs the other Go steps their D: temp dir; the alternative is to let `GO_STEPS` entries carry `env: { GOTMPDIR: null }` meaning "remove", a three-line change in `launch`. Recommend waiting for a real failure.
