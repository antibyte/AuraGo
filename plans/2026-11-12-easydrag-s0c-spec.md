# EasyDrag S0c "Hygiene track": design spec

**Status:** design spec, 2026-11-12. The owner approved the design in the brainstorming session the same day. Implementation plan: [`plans/2026-11-12-easydrag-s0c/README.md`](2026-11-12-easydrag-s0c/README.md). Planning amended some points of this spec; until the coordinator copies them in here as each part lands, the README's "Amendments found while planning" take precedence.
**Roadmap:** `plans/2026-11-09-easydrag-next-roadmap.md`, E1 items FD-12 (rest), FD-15, FD-16 and FD-20 (lines 289-310, acceptance 321-322), Wave 0 (§6), gates of Waves 2 and 3.
**Predecessors:** S0 landed at 79c7c601a (`plans/2026-11-10-easydrag-s0-spec.md`). S0b landed at c4370ae91 (Part A) and 53bd11eff (Part B) (`plans/2026-11-11-easydrag-s0b-spec.md`). Nothing is pushed.

## 1. Goals and owner decisions

S0c closes Wave 0. The roadmap lets it run until the Wave 2 exit; the owner chose to finish it now, as a short mini-wave before Wave 1, so that Wave 1 starts with fast tests, a shared test browser and touch tests.

- **Part A, defect tail (FD-20 and the rest of FD-12).** Every listed defect ends as fixed with a test, closed with evidence, or a task in a named wave. The ledger in §7 records each outcome.
- **Part B, test tiers (FD-15).** One fast command (T0) and one shared-browser smoke run (T1). Every browser test sets reduced motion through one helper.
- **Part C, living documentation (FD-16).** Reference blocks in the manuals are generated from the node catalog, and a test fails when they drift.
- **Part D, touch audit (FD-15).** Five gestures at two viewports under CDP touch emulation.

Owner decisions (2026-11-12):

| # | Question | Decision |
|---|---|---|
| 1 | logic.stop semantics (roadmap decision 13, defect E3) | **Keep today's behaviour and make it visible.** A run that a stop node ends with "success" stays successful and shows "durch Stopp beendet". Steps cut off by the stop show "unklar" when they may have had an effect. No engine change. |
| 2 | file.write default (roadmap decision 4) | **Keep "overwrite" and show it on the card.** A test pins the default. |
| 3 | Trigger start failures | **Record them as a failed run.** A run-history entry with the reason and the normal "Lauf fehlgeschlagen" notification. Identical consecutive failures are reported once. OP-01 (Wave 3) and PE-16 (Wave 5) see them with no extra path. |
| 4 | Touch audit timing | **Now, with design findings deferred.** Five gesture tests on today's UI. Small findings are fixed now; findings that need design work (for example box select by finger) become Wave 1 tasks. |
| 5 | Approach | **Compact now.** A and B in parallel, then C and D in parallel. |

## 2. Scope

**In S0c**
- Part A: start failures as runs (A1), the stop hint (A2), the file.write card (A3), schedule help (A4), target sizes (A5), plural forms (A6), evidence closings (A7), the ledger (§7).
- Part B: the T0 command and its CI job, the shared T1 browser, the reduced-motion helper and its guard.
- Part C: the generator, the drift test, the manual fixes, the agent manual's generated trigger list.
- Part D: the touch helper, five gestures at 1024×768 and 390×844, findings fixed or handed to Wave 1.
- The roadmap's Wave 0 status (§6 of the roadmap) records S0c as landed.

**Not in S0c**
- The status map (FD-08, Wave 1). A2 hands it the "stopped" entry (§3.2).
- The FLOW_* error-class registry (FD-19, Wave 2). A1 adds its code the way codes are translated today (§3.1).
- The canvas.js split (Wave 2 entry). No S0c change adds a line to canvas.js.
- New phone layouts. D records what fails at 390×844; the phone work stays in its waves.
- Engine changes to logic.stop (owner decision 1).

## 3. Part A: defect tail

### 3.1 A1: trigger start failures become failed runs

**Facts on main**
- `Service.TriggerFromMission` (`internal/flows/service_runs.go:235-252`) calls `startLive` with `gated=true`. `startLive` (`:269-292`) returns `ErrNotPublished` or `ErrNoTrigger` before the runner sees the request, or whatever `Runner.StartContext` returns. In none of these cases is a run recorded.
- The bridge drops the outcome: `flowMissionHooks.StartFlowRun` (`internal/server/flows_bridge.go:351-354`) calls `StartFlowRunReported` and returns only the error.
- The mission side (`internal/tools/missions_v2_flows.go:866-890`, `startFlowRun`) notes the three refusals (`ErrFlowRunsPaused`, `ErrFlowsUnavailable`, `ErrFlowQueueFull`) on the mission and sends every other error only to `slog.Warn`. Callers: `:811`, `:847`, `:1154`, and the planner (`missions_v2_planner.go:141`). A reported path exists (`missions_v2.go:1360`, `:2034`, `startFlowRunReported`).
- `MissionBridge.FlowRunFinished` (`internal/server/flows_bridge.go:263-293`) handles only started runs (`info.Started`). For them it calls `flowIssue` and `notifyFlowFailure` (`flows_bridge_notify.go:47`, `:105`). A run that never started only refreshes open editors, because that path runs inside `DeleteFlow`, `CancelMissionRuns` and `Runner.Shutdown` and must stay free of database and network work.
- Codes are translated on the client through `easydrag.ui.error_<code in lower case>` (`easydrag-core.js:418-423`).

**Which failures count**
- Counted: `ErrNotPublished`, `ErrNoTrigger`, and any error from `Runner.StartContext` that is not listed below (for example a document that fails validation at start).
- Not counted, because they are deliberate refusals or not a flow fault:
  - `ErrFlowRunsPaused`, `ErrFlowsUnavailable`, `ErrFlowQueueFull` and the runner's gate refusal (`RunsPausedCode`); they stay mission notes as today;
  - the runner closing during shutdown (`ErrRunnerClosed`);
  - `ErrNotFound`, a flow deleted between the mission's decision and the start.
- Only gated live starts count (`TriggerFromMission`). Run now and test runs answer their caller directly, as today.

**What happens**
1. `startLive` (or `TriggerFromMission` around it) classifies the error. A counted failure goes to a new `Service.recordStartFailure(rec, nodeID, triggerType, err)`.
2. **One record per streak.** The service keeps an in-memory map from flow id to the reason of its current streak: `not_published`, `no_trigger`, or the start error's own FLOW code. When the flow already has a streak with the same reason, nothing is recorded; a different reason records again and replaces it. A successful start (`StartStarted` or `StartQueued`) deletes the flow's entry. After a restart, the first failure records again. Deleting a flow removes its entry.
3. **The record.** A `RunRecord` with mode live, the flow's live revision (0 when there is none), the trigger node and type, status `error`, `ErrorCode` `FLOW_START_FAILED`, an `ErrorMessage` that names the reason in plain words (for example "the flow is not published"), cut to the usual 1,000 runes and scrubbed like any run error, `StartedAt` and `FinishedAt` both now, and no steps. Trigger data is not stored; a start that failed never ran on it.
4. **The report.** The service calls a new optional bridge interface:

   ```go
   // StartFailureReporter is an optional MissionBridge extension. FlowStartFailed is called
   // off the caller's goroutine, once per failure streak, after the failed run was recorded.
   type StartFailureReporter interface {
       FlowStartFailed(info RunFinishedInfo)
   }
   ```

   `info` carries the record, the flow name, the mission id and `NotifyOnError`, with `Started` false. The server's bridge implements it with `flowIssue(info, true, message)`, `notifyFlowFailure(info, message)` and `broadcastFlowsChanged(flowID, "run_finished")`. It does not call `mm.FlowRunFinishedAtDepth`, because no running slot was taken.
5. **Off the caller's goroutine.** `TriggerFromMission` can run inside `FlowRunFinished` of another flow's delete, with that flow's lock held (`service_runs.go:254-268`). The record and the report therefore run on a goroutine of their own. The streak map bounds them to one goroutine per flow per streak.
6. **The mission side.** `startFlowRun` and the planner call the reported path. A start failure is noted on the mission like the refusals (`noteFlowRefusalLocked`), with a new note kind "Start fehlgeschlagen". A `StartSkipped` outcome gets a quiet note only when the mission's previous note was not a skip.
7. **Translations.** `easydrag.ui.error_flow_start_failed` ("Der Flow konnte nicht starten.") and the note text, in all 16 `ui/lang/easydrag/*.json` files, with real translations and lowercase "du" in German. The FD-19 registry (Wave 2) picks the code up from there.

**Tests (Go):**
- `internal/flows`, the streak: with a runner fake, the start errors fail, fail, success, fail record two failed runs; fail (not published) then fail (no trigger) records two; a refusal (runs paused) and `ErrRunnerClosed` record none.
- `internal/server`, end to end: a flow mission whose flow is not published fires its trigger twice. The first fire records exactly one failed run with `FLOW_START_FAILED`, one desktop notification and one planner issue; the second records nothing.

### 3.2 A2: the stop hint (decision 13)

**Today's behaviour, recorded** (`internal/flows/engine_state.go`, `nodes_logic.go:521-545`):
- A logic.stop with status "success" ends the run. Steps not yet reached are closed as skipped (`closeOpenNodes`, `:780-799`). Steps still running get the abandon grace of 30 s (`engine.go:38`, `engine_state.go:361-366`) and are then cancelled. A cancelled step that has effects is marked `EffectUncertain` (`:623-627`, `:575`), which the UI already shows as "unklar" (`easydrag.ui.step_effect_uncertain`).
- The run ends as `success`, unless a step still running after the stop fails inside the grace; that failure is not hidden and the run ends as `error` (`conclude`, `:801-823`).
- A logic.stop with status "error" ends the run as `error` with `FLOW_STOPPED` and its message. This is already visible and needs nothing.

**Change (client only, no engine change):**
- A pure function `stoppedBy(detail)` returns the stop step of a successful run, or null. A step counts when its node in `detail.doc` has type `logic.stop` and its status is `success`. When `detail.doc` is missing (a pruned version), it falls back to the editor's model node with the same id and type; otherwise it returns null and no hint is shown.
- The run drawer (`easydrag-run-drawer.js`) and the test result panel (`easydrag-testing.js`) show the hint "durch Stopp beendet", naming the stop step's label. Both modules are far below the 900-line warning.
- New keys in all 16 locales.
- **Status map.** Roadmap decision 13 asks for a status-map entry. The status map arrives with FD-08 in Wave 1; the ledger hands it a task: a run-level reason "stopped" whose word is "durch Stopp beendet", rendered by every surface that shows a run status.

**Test (Node):** `stoppedBy` against four details: a stop run (hint), a stop run with a failed step after the stop (no hint, the run is an error), an ordinary run (no hint), and a stop run whose doc is missing (falls back to the model, or no hint).

### 3.3 A3: file.write shows "überschreibt" (decision 4)

- The card's second line comes from the node type's summary template (`easydrag-canvas.js:376-379`), and an enum value already renders as its option label (`:349-352`). The control's labels ("Überschreiben", "Beide behalten (neuer Name)", "Mit Fehler abbrechen") stay, because they read well as choices. The card gets its own short words:
  - `Option` gets an optional card label key, resolved by the node-types API like the label and sent as `card_label`. Only file.write's `if_exists` sets them: "überschreibt", "neuer Name", "bricht ab", with matching short forms in the other 15 locales.
  - `displayValue` prefers `opt.card_label` over `opt.label`. That changes the existing line at `easydrag-canvas.js:351` and adds none, so canvas.js does not grow.
  - `easydrag.node.file_write.summary` becomes `"{path} · {if_exists}"` in all 16 locales. A step without `if_exists` shows the default, because `displayValue` already falls back to it (`:347`).
- **Test (Go):** `fileWriteDef`'s `if_exists` default is `"overwrite"` (`catalog_actions_docs.go:392`), and a file.write node without `if_exists` overwrites an existing file. The retry comment at `:444-451` stays as the reason.

### 3.4 A4: schedule help (N20, days 29–31)

- `catalog_schedule.go:36-45` gets help keys:
  - `minutes`: valid values 1, 2, 3, 4, 5, 6, 10, 12, 15, 20 and 30, because the interval must divide the hour evenly (`scheduleInterval`, `:137-157`);
  - `hours`: valid values 1, 2, 3, 4, 6, 8 and 12;
  - `day`: a month without that day (29, 30 or 31) skips that run; it does not move to the last day.
- New keys in all 16 locales. The server's English error stays as the detail behind the field's issue.
- **Test:** the existing catalog i18n test (`catalog_i18n_test.go`) covers the new keys; no new test.

### 3.5 A5: target sizes (U9, V4)

- `ui/css/desktop-app-easydrag.css`:
  - `.ed-seg--small button` (line 356): min-height 22 px → 24 px;
  - `.ed-port-add` (line 754): 22×22 → 24×24.
- Already sufficient, recorded as exceptions with the reason:
  - `.ed-switch--small` (line 371): the visible switch is 30×17, its `::before` hit area is 34×25;
  - `.ed-flow-select input` (line 469): the 20×20 box sits in a 44 px label that is the target.
- **Test (T1 browser):** in the main editor fixture at 1440×900, every visible `button`, `a[href]`, `input`, `select` and `[role=button]` inside the EasyDrag window measures at least 24×24 CSS px, or matches a selector on the documented exception list, each with its reason. Inline links inside running text are exempt (WCAG 2.5.8).

### 3.6 A6: "1 Schritte"

- `group_member_count` (`de.json:1293`, used at `easydrag-groups.js:34`) and `versions_stats` (used at `easydrag-publish.js:288`) get `_one` variants in all 16 locales and go through the same selection as `stepsText` (`easydrag-core.js:144-145`).
- **Test (Node):** count 1 and count 2 give the singular and the plural.

### 3.7 A7: evidence closings
- `internal/flows/AGENTS.md:831` names no template count any more, because `templates_builtin.go` (12 templates) changes more often than the guide.
- The double scrub in `credentials.go:669-684` and `local_pack.go:421`, `:447-448` stays as defence in depth. A comment at each site names the other.
- The "Vergangener Lauf" residual tooltip (`easydrag-header.js:52`, `:124-127`): the header pill and its tooltip no longer both say "Vergangener Lauf"; the tooltip names the run's start time instead.
- The items already fixed by earlier work are closed with file, line and test in the ledger (§7). Where an item has code but no test, A7 adds the smallest test that pins it.

## 4. Part B: test tiers (FD-15)

### 4.1 T0
- New `scripts/test-easydrag-t0.mjs` and the npm script `test:easydrag:t0`. It runs, and fails on the first failure:
  1. every `scripts/test-easydrag*.mjs` (35 today) in one Node process per script, in parallel up to the CPU count;
  2. `go test ./internal/flows/...`;
  3. `go test ./internal/server -run '(?i)flow'`;
  4. `go test ./internal/tools -run '(?i)flow'`;
  5. `go test ./ui -run '(?i)easydrag'`, where the browser tests skip because `AURAGO_RUN_BROWSER_SMOKE` is unset.
- **Stale scripts.** Each of the 35 scripts runs once before T0 exists. A script that fails on main is fixed, or deleted when what it tests no longer exists. The plan records each one.
- **Target:** under 60 s with a warm build cache, on the owner's machine. The plan measures it and writes the number into this spec. If T0 is above 60 s, the slowest Go step moves to T1 and the spec records that, rather than missing the target silently.
- **CI.** The Node job of `.github/workflows/security-gates.yml` (line 52) runs the Node scripts of T0 in addition to `npm run test:ui-regressions`. The Go steps already run in the Go job (`go test ./... -p 1`).
- Go temp-directory settings stay with the caller's environment; the script sets none.

### 4.2 T1: one shared browser
- `startEasyDragSmoke` (`ui/desktop_easydrag_browser_test.go:349-424`) gets its browser from `sharedEasyDragBrowser()`: one Chrome per `go test` process, started on first use through a `sync.Once`. Each test gets a fresh incognito browser context and closes it in `t.Cleanup`.
- A new `TestMain` in the `ui` package closes the shared browser after `m.Run()`. The package has none today.
- A test that needs its own Chrome (for example to set launch flags) says so with a helper option and gets a private browser.
- The 22 EasyDrag browser files keep their tests; only the start helper changes. The durable test file also launches a browser directly; it moves to the helper or gets the option above.

### 4.3 Reduced motion
- `startEasyDragSmoke` sets `prefers-reduced-motion: reduce` through `Emulation.setEmulatedMedia` on every page it opens. A test that checks motion opts in with an option and then sets both values explicitly.
- **Guard (Go source test, T0):** every `ui/*easydrag*_browser_test.go` opens pages only through `startEasyDragSmoke`, or carries a listed exception with a reason. The test reads the files and fails on `launcher.New`, `rod.New` or `newSmokeBrowser` outside the helper.

## 5. Part C: living documentation (FD-16)

### 5.1 The generator
- New `cmd/flowsdoc`, run as `go run ./cmd/flowsdoc`.
- It builds the catalog with `flows.RegisterCatalog` and a static environment in which every node type is available, and reads the labels from `ui/lang/easydrag/en.json` and `de.json`.
- It rewrites the text between markers and nothing else:

  ```
  <!-- BEGIN GENERATED: flows-steps -->
  …
  <!-- END GENERATED: flows-steps -->
  ```

- **Blocks:**
  - `flows-triggers` and `flows-steps` in `documentation/manuals/en/24-easydrag.md` and `de/24-easydrag.md`, in a new closing section "Reference: triggers and steps" / "Referenz: Auslöser und Schritte". One table per block, sorted by category, then by label. Columns: name (localized label), category, what it does (localized description), effects (from `NodeDef.Effects`, in words), and in Simulation (runs for real, or simulated). The Simulation column comes from the same rule the Simulation mode applies; if that rule is not one function today, the plan extracts it into one, and both use it.
  - `flows-triggers` in `prompts/tools_manuals/manage_missions.md`, replacing the hand-written trigger list next to lines 20-21. English, with the trigger type id next to the label, because the agent names types.
- Output is deterministic: no dates, stable sort, LF line ends.

### 5.2 The drift test
- `TestFlowsDocBlocksMatchTheCatalog` (in `cmd/flowsdoc`, T0) renders every block in memory and compares it with the file. On a difference it fails and names the command: `go run ./cmd/flowsdoc`.
- A file that lacks a marker pair fails the test too, so a block cannot be deleted by accident.

### 5.3 Manual fixes by hand (after Part A)
In both `en` and `de` chapter 24:
- the Simulation default in the test dialog (`easydrag-testing.js:112-115`);
- the schedule modes, with the valid intervals and the days 29–31 rule (A4);
- "durch Stopp beendet" and "unklar" (A2);
- file.write overwrites by default and shows it on the card (A3);
- trigger start failures appear as failed runs with one notification per streak (A1).

### 5.4 The agent manual
`manage_missions.md:20-21` already describes `read_only` and `allow_publish` as they behave (`internal/config/flows.go:25-33`). The ledger closes that roadmap acceptance point with evidence. Only the generated trigger list is new.

## 6. Part D: touch audit

### 6.1 The touch helper
In the T1 test helpers:
- `Emulation.setDeviceMetricsOverride` with `mobile: true` and `Emulation.setTouchEmulationEnabled` with 5 touch points;
- `touchTap(x, y)`, `touchDrag(from, to, steps)` and `touchLongPress(x, y, ms)`, each built on `Input.dispatchTouchEvent`.

### 6.2 The gestures
Each gesture runs at 1024×768 and at 390×844 on the main editor fixture:

| Gesture | Code today | Passes when |
|---|---|---|
| Insert a step on a wire | `easydrag-wires.js:30`, `easydrag-palette.js:59-91` (`insertOnEdge`) | a tap on a wire offers insertion, and the chosen step lands between the two cards |
| Delete a wire | `easydrag-wires.js:219` | a wire can be selected and removed by touch |
| Card tools | `easydrag-canvas.js:478-504`, CSS `:hover`/`:focus-within` at line 772 | the card's tools become reachable by a tap, not only by hover |
| Mapping chip into a field | `easydrag-mapping.js:202-225`, `easydrag-forms.js:434-459` | the tap fallback inserts the chip, because HTML5 drag cannot be started by touch |
| Box select | `easydrag-interact.js:391-403` | several cards can be selected by touch |

### 6.3 Findings
- A small finding (a missing `touch-action`, a hover-only control that needs a focus or tap path, a target below 24 px) is fixed now, with its gesture test as the test.
- A finding that needs design (likely: box select by finger, wire selection at 390 px) becomes a Wave 1 task in the ledger. Its gesture test is committed with `t.Skip` and the ledger reference, so it turns on when Wave 1 builds the fix.

## 7. FD-20 ledger

Outcome kinds: **fixed** (with a test), **closed** (already fixed; evidence given), **handed** (a task in a named wave). The plan fills in the final evidence column as each part lands.

| # | Item (roadmap line) | Outcome | Evidence or task |
|---|---|---|---|
| 1 | logic.stop run-level semantics, defect E3, decision 13 (297) | fixed (A2) | `stoppedBy` Node test; the status-map entry is handed to Wave 1, FD-08 |
| 2 | N20 help for divisors (298) | fixed (A4) | `catalog_i18n_test.go` covers the keys |
| 3 | `internal/flows/AGENTS.md` says "six" templates (299; now line 831) | fixed (A7) | the count is removed |
| 4 | mission_update debounce (301) | closed | `internal/server/mission_update_debounce.go:11` (250 ms); `TestFB4MissionUpdatesAreDebounced` |
| 5 | SSE redaction per (run, seq) with a per-client cap (302); the Wave 2 entry "SSE redaction verified" (309) | closed | `flows_sse_cache.go:35-65`; `flowStreamsPerClient=8` at `flows_handlers_runs.go:381`; `TestFB4SSECacheScrubsAnEventOnce`, `TestFB4FlowStreamsAreCappedPerClient` |
| 6 | start outcome discarded in `StartFlowRun` (303); "StartResult surfaced" for OP-01 and PE-16 (308) | fixed (A1) | the streak test in §3.1 |
| 7 | LastRun missing omitzero (304) | closed | `internal/tools/missions_v2.go:157`; `TestFB4MissionLastRunIsLeftOutWhenZero` |
| 8 | docked palette not restored after the run view below 900 px (305) | closed | `easydrag-editor.js:245-246`, `easydrag-palette.js:257-260`; the plan names the covering test in `scripts/test-easydrag-ux-batch5.mjs` or adds one (A7) |
| 9 | duplicated "Vergangener Lauf" (306) | fixed (A7) | `de.json:884`; the header tooltip residual (`easydrag-header.js:52`, `:124-127`) |
| 10 | U9/V4 targets under 24 px (307) | fixed (A5) | the T1 target-size test |
| 11 | FD-12: help for schedule days 29–31 (291) | fixed (A4) | as item 2 |
| 12 | FD-12: manual drift fixes (291) | fixed (C3) | the drift test plus the hand-written sections of §5.3 |
| 13 | FD-12: file.write default, decision 4 (291) | fixed (A3) | the default test in §3.3 |
| 14 | FD-16: the agent manual describes read_only and allow_publish as they behave (321) | closed | `prompts/tools_manuals/manage_missions.md:20-21`, `internal/config/flows.go:25-33` |
| 15 | Found while planning: "1 Schritte" plurals | fixed (A6) | the plural Node test |
| 16 | Found while planning: double scrub in packs | closed (A7) | comments at both sites; defence in depth |
| 17+ | Touch findings (Part D) | fixed or handed to Wave 1 | one row per finding, added as D runs |

## 8. Testing

Essential tests only (owner's standing preference).
- **Go:** the start-failure streak test and the end-to-end test (A1); the file.write default test (A3); the browser-source guard (B3); the drift test (C2).
- **Node:** `stoppedBy` (A2); the plurals (A6).
- **Browser (T1, gated by `AURAGO_RUN_BROWSER_SMOKE=1`):** the target-size test (A5); the five gestures at two viewports (D).
- Every browser test sets reduced motion through the helper (B3). T0 runs every non-browser test above.

## 9. Order and parallel work

- **Part A** in one worktree: A1, A2, A3, A4, A5 (CSS only; its test lands with B), A6, A7.
- **Part B** in a second worktree, at the same time: B1 (stale scripts, then T0, then CI), B2 (shared browser and `TestMain`), B3 (reduced-motion helper and guard), then the A5 target-size test once A5's CSS is on main.
- **Part C** after A lands: C1 generator, C2 drift test, C3 manual fixes, C4 agent manual block.
- **Part D** after A and B land, at the same time as C: the touch helper, then one commit per gesture with its fixes.
- One combined review per part (Critical and Important findings only). Worktree commits carry the fallback line for GitNexus.
- Files do not overlap between parallel parts. A owns the locale, CSS and product-code changes of §3. B touches only test helpers, scripts and CI. C touches only the manuals, the agent manual and `cmd/flowsdoc`. D adds CSS and JS only for its touch fixes.

## 10. Acceptance checklist
- [ ] Every row of the ledger (§7) has an outcome, and each "fixed" row names its test.
- [ ] A trigger start that fails records one failed run with `FLOW_START_FAILED`, one notification and one planner issue per streak; refusals record nothing.
- [ ] A run ended by a "success" stop shows "durch Stopp beendet"; cut-off steps with effects show "unklar".
- [ ] A file.write card shows "überschreibt" by default, and a test pins the default.
- [ ] `npm run test:easydrag:t0` passes, and its time is recorded in this spec (target under 60 s warm).
- [ ] The CI Node job runs the EasyDrag Node scripts.
- [ ] All EasyDrag browser tests share one Chrome per process and set reduced motion through the helper; the guard test is green.
- [ ] The target-size test is green.
- [ ] The drift test is green, and fails when a catalog label changes without regenerating.
- [ ] The five touch gestures are green at both viewports, or skipped with a Wave 1 ledger row.
- [ ] Every new key is in all 16 locales; canvas.js has not grown; `TestShippedWebAssetsLoadNothingExternal` stays green.
- [ ] The roadmap's Wave 0 status records S0c as landed, and its Wave 2 entry "SSE redaction verified" and the Wave 0/2/3 gate "StartResult surfaced" point at the ledger.

## 11. Risks
- **T0 above 60 s.** `internal/server` compiles slowly. The plan measures first; the fallback (§4.1) moves the slowest step to T1 and records it.
- **The shared browser leaks state between tests.** Incognito contexts isolate cookies and storage; a test that changes browser-wide state (permissions, downloads) uses the private-browser option.
- **Start-failure noise.** A flow on a one-minute schedule that is unpublished would fail every minute. The streak rule records once per streak, and the existing flood rule (`flowFailureNotifier`) bounds notifications further.
- **Touch emulation is not a real finger.** CDP touch events skip browser gesture heuristics such as scroll intervention. The audit finds missing paths, not feel; the Wave 1 human sessions judge feel.
- **Generated tables grow the manual.** The steps table has one row per node type. It sits in a closing reference section, so the reading chapters stay short.
