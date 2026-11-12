# EasyDrag S0c "Hygiene track" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close Wave 0 of EasyDrag. Every open defect from FD-20 and FD-12 ends as fixed with a test, closed with evidence, or a task in a named wave. EasyDrag gets a fast test command (T0), one shared test browser with reduced motion (T1), manual reference tables generated from the node catalog, and touch tests for five gestures.

**Architecture:**
- **Part A** fixes the defect tail in product code:
  - A trigger start that fails becomes a failed run, recorded and reported once per failure streak. The work runs off the caller's goroutine, through a new optional bridge interface `StartFailureReporter`.
  - The client gets the remaining fixes:
    - the hint "durch Stopp beendet";
    - the word "überschreibt" on the file.write card;
    - help text for the schedule fields;
    - targets of at least 24 px;
    - singular forms for counts of one;
    - a start time in the "Vergangener Lauf" tooltip.
- **Part B** adds the test tiers:
  - `scripts/test-easydrag-t0.mjs`, with its CI step;
  - one Chrome per `go test` process, with an incognito context per test;
  - reduced motion set in one helper, with a source test that guards it.
- **Part C** adds `cmd/flowsdoc`. It rewrites marked blocks in the manuals from the node catalog, and a drift test in T0 fails when they fall behind.
- **Part D** drives five gestures through CDP touch emulation at 1024×768 and 390×844.

**Tech Stack:** Go 1.27 (`internal/flows`, `internal/server`, `internal/tools`, `ui`, `cmd/flowsdoc`); go-rod v0.116.2; vanilla JS classic scripts with `window.EasyDrag`; Node test scripts (`scripts/test-easydrag*.mjs`); 16 locale files per area under `ui/lang/`.

**Spec:** [`plans/2026-11-12-easydrag-s0c-spec.md`](../2026-11-12-easydrag-s0c-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) E1 (FD-12 rest, FD-15, FD-16, FD-20), §6 Wave 0.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-a1-start-failures.md](part-a1-start-failures.md) | §3.1 | A1.1 classify, streak, record, report (`internal/flows`) · A1.2 bridge reporter and end-to-end test (`internal/server`) · A1.3 mission side and locales |
| [part-a2-tail.md](part-a2-tail.md) | §3.2–§3.7 | A2 stop hint · A3 file.write card · A4 schedule help · A5 target sizes (CSS) · A6 plurals · A7.1 guide and scrub comments · A7.2 tooltip · A7.3 ledger evidence |
| [part-b-tiers.md](part-b-tiers.md) | §4, §3.5 test | B.1 trim the slow Node check · B.2 T0 · B.3 CI · B.4 shared browser, `TestMain`, guard · B.5 reduced motion · B.6 target-size test · B.7 report |
| [part-c-docs.md](part-c-docs.md) | §5 | C.1 Simulation rule in one function · C.2 generator, drift test, marker blocks · C.3 hand-written manual fixes · C.4 agent manual evidence · C.5 drift test into T0 |
| [part-d-touch.md](part-d-touch.md) | §6 | D.1 touch helper · D.2 insert on a wire (fix F1) · D.3 delete a wire · D.4 card tools and long press · D.5 mapping chip · D.6 box select (handed to Wave 1) · D.7 report |

Every task commits once, except A7.3 and C.4 (verification only) and B.7 and D.7 (reports).

## Order, worktrees and reviews

| Part | Worktree | Branch | Starts | Lands |
|---|---|---|---|---|
| A (A1 then A2–A7) | `.worktrees/easydrag-s0c-defects` | `feat/easydrag-s0c-defects` | now | first |
| B | `.worktrees/easydrag-s0c-tiers` | `feat/easydrag-s0c-tiers` | now, in parallel with A | after A, because B.6 needs A5's CSS |
| C | `.worktrees/easydrag-s0c-docs` | `feat/easydrag-s0c-docs` | after A has landed | C.5 waits for B |
| D | `.worktrees/easydrag-s0c-touch` | `feat/easydrag-s0c-touch` | after A and B have landed, in parallel with C | last or with C |

- **B.6** is the A5 target-size test. It starts with `git rebase main` in B's worktree once A has landed. Until then B stops after B.5.
- **C.5** adds the drift test to T0. It rebases C's branch on main once B has landed.
- These two rebases are the only rebases inside a part.

**Review batches.** Each part gets one combined review, run by `superpowers:code-reviewer` before the part lands. The review reports only Critical and Important findings, and there are no re-review rounds. The review of A also checks A1's goroutine and lock design against `service_runs.go` (the `DeleteFlow` path) and `Shutdown`.

| Batch | Covers | Review point |
|---|---|---|
| A | A1.1–A7.2 | after A7.2 |
| B | B.1–B.6 | after B.6 |
| C | C.1–C.5 | after C.5 |
| D | D.1–D.6 | after D.6, before D.7's report |

---

## Conventions that override the parts

The five parts were written in parallel from one brief. Where they differ, this section decides.

- **Shell.** Git Bash.
  - Every command starts with `cd <worktree> && …`, because the working directory resets between calls.
  - Set `MSYS_NO_PATHCONV=1` for an argument that starts with `//`.
- **IMPACT.** Before changing or moving a function, run `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`.
  - Warn on HIGH or CRITICAL. Expect `runNowTitle` (A7.2) to come out HIGH; its signature does not change.
  - UNKNOWN is unresolved: confirm the callers with `grep -rn`.
- **DETECT.** GitNexus cannot index a worktree on this machine. Every commit body therefore ends with `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, placed before the Co-Authored-By line.
- **Commits.**
  - One per task: stage exact paths, then `git commit -F <message file>`. Message files go into `C:/disposable/easydrag-s0c/`.
  - The last line is the Co-Authored-By line for the model the implementer actually runs as.
  - Never amend or push. Rebase only in B.6 and C.5.
- **No part edits the spec or the roadmap.** Measured numbers, ledger evidence and amendments go into each part's final report. The coordinator records them on main (see "Finishing S0c").
- **Go temporary directory.**
  - `GOTMPDIR='D:\tmp\go-build'` for `internal/flows`, `internal/tools`, `cmd/...` and `ui`.
  - No `GOTMPDIR` for `internal/server`, because a temp directory on D: breaks a cross-drive `filepath.Rel` there.
  - The T0 script sets none and passes the caller's environment through.
- **Browser tests.**
  - Gated by `AURAGO_RUN_BROWSER_SMOKE=1`, with `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots`.
  - Run with `GOTMPDIR='D:\tmp\go-build'` and the Bash sandbox disabled (memory note `reference_chrome_launch_from_agent`).
  - Re-run a single failing test alone before treating it as a regression.
- **Escapes.** The Write and Edit tools turn backslash-u sequences into characters.
  - Go: `"\x5c" + "u003c"` or numeric runes.
  - JS: `String.fromCodePoint(...)`.
- **Locales.**
  - Every new key goes into all 16 files of its `ui/lang/<area>/` folder, with the exact texts the task gives. German uses lowercase "du".
  - The files are canonical JSON: `JSON.stringify(obj, null, 2) + '\n'` reproduces them byte for byte.
  - New keys go next to their siblings, through the helper script that Part A2 defines (A2, Step "locale helper"). A1.3 uses its own small insert script.
  - `TestCatalogTranslationsAreComplete` is the guard.
- **Line ratchet.**
  - `easydrag-canvas.js` (928 lines) must not change length. A3 changes one existing line; D.2 puts its fix elsewhere.
  - `easydrag-model.js`, `easydrag-template.js`, `easydrag-editor.js`, `easydrag-home.js` and `easydrag-detail.js` take at most a change inside an existing line.
- **Gates before each commit.**
  - Go changes: `go vet` on the touched packages and `gofmt -l` on the touched files.
  - JS or CSS changes: `node scripts/test-easydrag.mjs` (`all EasyDrag checks passed`) and `node scripts/test-easydrag-render-baseline.mjs` (`PASS render baseline (unchanged)`).
  - A3 may change a file.write card's second line in the baseline. Update the baseline with `AURAGO_EASYDRAG_RENDER_BASELINE=update` only when that line is the sole difference, and say so in the commit.
- **Tests.** Essential tests only (the owner's standing preference). No external assets; `TestShippedWebAssetsLoadNothingExternal` stays green.
- **aurago-test.** S0c does not use it.

---

## Amendments found while planning

Each amendment is applied in the named task. The coordinator copies them into the spec when the part lands.

**Part A1, start failures (§3.1)**
1. **No runner fake (A1.1).** `Service` holds the concrete `*Runner`, so `TestStartFailureStreak` drives the real runner with real start errors:
   - `ErrNoTrigger` from an unknown trigger node id;
   - `ErrNotPublished` from an unpublished flow;
   - `ErrRunnerClosed` after `Shutdown`.
   Other refusals are covered by a classification table.
2. **The gate refusal (A1.1).** `RunsPausedCode` never reaches a caller as a start error; it is only the code of a queued run that the gate stopped. The Service asks its own `RunGate` and records nothing while the gate is closed.
3. **Not counted, beyond the spec's list (A1.1):** `ErrQueueFull`, `ErrFlowDisabled`, `ErrFlowMissionMissing`, `ErrMissionControlUnavailable`, and a cancelled or expired context.
4. **The streak reason for other errors (A1.1).** It is `"start_error: "` plus the error text, cut to 200 runes, because the Service has no error-to-code function. `recordStartFailure` takes the reason as an argument.
5. **Timers count (A1.1, coordinator decision).**
   - The spec leaves out Run now and test runs because they answer their caller directly. A Date/Time timer has no caller, so `onTimerFired` calls `afterTriggerStart` with the same rules.
   - It runs on the timer goroutine, outside any flow lock. A retrying timer stays one streak.
   - Mission Control's own Run goes through `TriggerFromMission`. A user's Run on an unpublished flow therefore also records one failed run.
6. **The mission note stays English (A1.3).** It is a constant like `flowRunsPausedOutput`, stored as the mission's `LastOutput`, and no `ui/lang` folder holds mission notes. The translated text lives in the run history and the notification (`easydrag.ui.error_flow_start_failed`).
7. **Mission Control recognises a start failure (A1.3)** through a wrapped `tools.FlowStartFailedError`. The server's hook applies it via `flows.IsStartFailure`. A plain error such as "store down" still only warns.

**Part A2, the tail (§3.2–§3.7)**

8. **Where the stop hint shows (A2).** It shows in the run view the drawer opens (its banner) and in the test result panel. It does not show in the drawer's list rows, because they carry no steps.
9. **`stoppedBy` also requires the run's status to be `success` (A2).** The engine records the stop step as `success` even for a stop with status "error". The function returns `{ step, node, label }`.
10. **Cut-off steps.** The spec's "unklar" is the existing chip "Wirkung möglicherweise erfolgt" / "Effect may have happened" (`easydrag.ui.step_effect_uncertain`). Manuals and the spec quote the real text.
11. **`if_exists` values (A3).** They are `overwrite`, `unique` and `fail`. The card keys are `easydrag.option.if_exists_{overwrite,unique,fail}.card`, and a second test pins `card_label` (`TestFileWriteOptionsCarryCardLabels`).
12. **More plural keys (A6).** A6 adds five `_one` keys through a new `core.pluralKey`:
    - `versions_stats` holds two counts, so it gets `versions_stats_one` and `versions_stats_edge_one`;
    - the group header gets `group_member_count_one`, `group_edges_count_one` and `group_errors_one`.
13. **Tooltip (A7.2).** It reads "Lauf vom {time}" and falls back to today's text when the run has no start time.
14. **No DOM test for the banner hint (A2).** The Node harness has no `logic.stop` type. `stoppedBy` is tested; the two call sites are thin.
15. **Further targets under 24 px (B.6).** B.6 fixes any control under 24 px beyond A5's two in the CSS, in its own commit, because A has landed by then. Planning found one: the palette setup link `a.ed-chip.ed-palette-setup` (72×20), fixed with `.ed-palette-setup { min-height: 24px }`.

**Part C, living documentation (§5)**

16. **The Simulation column has four outcomes (C.1):** runs locally, replaced by your sample output, cannot be simulated, supplies the test data (triggers).
    - C.1 extracts `SimulationBehaviorOf` from the inline rule in `Service.Simulate`.
    - `logic.wait` keeps today's behaviour (replaced by the sample output).
17. **The agent manual (C.2).** `manage_missions.md` lines 20–21 are the mission-trigger enum, and no hand-written flow-trigger list exists. The generated `flows-triggers` block gets its own section, "Flow trigger types", before `## Examples`. The two marker comment lines stay in the prompt text.
18. **Order and scope of the tables (C.2).**
    - Categories follow the palette order (`flows.CategoryOrder`), then label, then type id.
    - Generic `tool.*` steps are not listed, because `RegisterCatalog` does not register them. The reference has 14 triggers and 28 steps.
19. **The guide line (C.2).** C.2 adds this bullet to `internal/flows/AGENTS.md` (Part A has landed, so the file is free): "After changing a node label or description in `ui/lang/easydrag/en.json` or `de.json`, run `go run ./cmd/flowsdoc`; T0's drift test `TestFlowsDocBlocksMatchTheCatalog` fails otherwise."
20. **Start failures in the manual (C.3).** The manual's sentence on start failures names Date/Time schedules too (amendment 5).

**Part D, touch audit (§6)**

21. **Wire selection is a small finding (D.2).** It is fixed now as finding F1, not handed to Wave 1:
    - a tapped wire is selected, and its actions stay open;
    - the hit stroke and the buttons are 44 px under a coarse pointer.

    Only box select (F2) goes to Wave 1.
22. **Six test functions instead of five (D.1, D.4).**
    - `TestEasyDragTouchHelper` checks the helper first, so a failing gesture points at the product.
    - D.4 also covers the long-press menu.
23. **Test fixtures (D.5, D.1).** The mapping test taps the PDF step's `content` field. Tests at 390 px use only what the automatic view shows.

**Part B, test tiers (§4)**

24. **T0 runs the Go steps at the same time as the Node scripts (B.2).**
    - Measured on 16 CPUs with a warm build cache:

      | Run | Time |
      |---|---|
      | one after another | 94 s (Node 21 s, Go 73 s) |
      | together, tests forced (`GOFLAGS=-count=1`) | 37–39 s |
      | together, Go results cached | 26 s |
      | cold (fresh worktree), Go steps alone | 162 s |

    - The 60 s target is met. The spec records both warm numbers, and the fallback (moving `internal/flows` to T1) is not needed.
    - The script still stops at the first failure and keeps `GO_STEPS` as one ordered array.
25. **All 35 Node scripts pass on main (B.1).** None is deleted.
    - 23 of them only export `run()` for `test-easydrag.mjs`.
    - The one problem is time. `test-easydrag.mjs` takes 223–249 s, 201 s of it in the F6 run-event check of `test-easydrag-ux-batch5.mjs` (300 steps, 1,000 events). B.1 trims that check to 40 steps and 160 events (3.7 s), and the script then runs in 28 s.
26. **No `-count=1` in `GO_STEPS` (B.2, C.5).** Go's test cache makes an unchanged tree cheap, and a caller forces a full run with `GOFLAGS=-count=1`. C.5's entry follows this.
27. **Helpers beyond the brief (B.4).**
    - `newSmokeBrowserWith` in `ui/destructive_flow_browser_test.go` is split into `launchSmokeBrowser` plus a wrapper; its messages and cleanup order are unchanged.
    - `newEasyDragPage` gives the durable config test, which opens `/config` and not the fixture, a page in the shared browser.
    - The guard lives in `ui/desktop_easydrag_helper_guard_test.go`. It marks the helper section with `// easydrag-helpers:begin` and `// easydrag-helpers:end`, and its exception map starts empty.
28. **Reduced motion (B.5).** The helper replaces 7 inline settings in 6 files, and `TestEasyDragSmokeSetsReducedMotion` pins it.
29. **The target-size measure (B.6).**
    - The scope is `.ed-app`.
    - The size is the larger of the bounding box and the layout box, because `.ed-port-add` is scaled to 0.6 until hover.
    - The two documented exceptions are measured by their real target (the `::before` area, the 44 px label), not skipped.
30. **`GOTMPDIR` and `internal/server`.** On this machine `GOTMPDIR` is set globally, and the server step passed with it. T0 passes the caller's environment through. The manual rule above (no `GOTMPDIR` for `internal/server`) stays for hand-run commands until a real failure shows which is right.
31. **Browser timings (B.4).** The gated EasyDrag run takes 259 s instead of 300 s. It ends at 36 PASS: the baseline's 33, plus the guard, the motion test and the target-size test.

---

## Ledger additions (spec §7)

The coordinator adds these rows when the parts land. Rows 1–16 keep their outcome, and the coordinator fills in the evidence with the commit SHA.

| # | Item | Outcome | Evidence or task |
|---|---|---|---|
| 17 | F1: a wire cannot be selected by touch; its Insert/Delete actions vanish (D.2) | fixed | `TestEasyDragTouchInsertOnWire`, `TestEasyDragTouchDeleteWire` |
| 18 | F2: box select by finger; a finger on empty canvas always pans (D.6) | handed to Wave 1 | a select mode for touch; the skipped test turns on with it |
| 19 | `.ed-port-add` overlaps the first third of an outgoing wire at 390 px (D.7 watch item) | handed to Wave 1 | `pointer-events: none` until its card is selected |
| 20 | "1 …" plurals in other features: `folder_flows`, `cred_purge_versions`, `cred_purge_pinned` (found in A6) | handed to Wave 1 | `core.pluralKey`, about three lines each |
| 21 | Status-map entry "stopped" (decision 13, A2) | handed to Wave 1, FD-08 | a run-level reason "stopped" whose word is "durch Stopp beendet", rendered on every run-status surface |

D.7's report may change rows 17–19, depending on what the gesture tests show.

## Not covered by S0c (follow-ups)
- The rows handed to Wave 1 above.
- The FLOW_* code registry (FD-19, Wave 2). It picks `FLOW_START_FAILED` up from the locale key.
- The canvas.js split (Wave 2 entry).

---

## Finishing S0c

A lands first, then B, then C and D.

1. **Rebase and re-check.** In the part's worktree, rebase its branch onto the current main, then run the part's checks again:
   - **A:** `go test ./internal/flows/... ./internal/tools -count=1` (with GOTMPDIR), `go test ./internal/server -run '(?i)flow' -count=1` (without), `node scripts/test-easydrag.mjs` and the render baseline.
   - **B:** `npm run test:easydrag:t0`, plus one full gated EasyDrag browser run.
   - **C:** `go test ./cmd/flowsdoc ./internal/flows/... -count=1` and `npm run test:easydrag:t0`.
   - **D:** the touch tests and one full gated EasyDrag browser run.
2. **Check the main checkout.** Other sessions work in it.
   - Check `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current`.
   - Compare the uncommitted files there with the branch's files.
3. **Land.** `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only <branch>`. Do not push; the owner decides when to push and deploy.
4. **Record.** The coordinator records each part on main in one docs commit. It stages only its own paths through a temporary index, never the shared index.
   - In the spec:
     - the amendments above;
     - the ledger evidence with SHAs and test names;
     - T0's measured time in §4.1 (after B);
     - rows 17+ from D.7.
   - In the roadmap, after the last part:
     - §6 Wave 0 status: S0c landed;
     - the Wave 2 entry "SSE redaction verified" and the Wave 0/2/3 gate "StartResult surfaced" point at the ledger.
   - The EasyDrag project memory.
5. **Clean up.** Remove the worktree and delete the merged branch.
