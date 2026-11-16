# EasyDrag W1c "Header and one Stop" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the EasyDrag editor a calm header that says where the flow stands and offers its next step, show exactly one Stop, and let the quick-add say what a new step goes after and suggest what usually comes next.

**Architecture:**
- **Part A.** Three server facts reach the editor as top-level fields of the flow API:
  - `next_run`, from one shared `flows.Service.NextRun` that Mission Control also calls;
  - `tested`, a durable marker in the existing `flows_meta` table (no migration);
  - `changes`, from the publish diff's own counting.
- **Part B.** The header becomes Zurück, Name, Pill, Stop slot, Testen, primary, Undo, Redo and "⋯".
  - The pill (`easydrag-pill.js`) says where the flow stands. Its popover (`ED.kit.popover`) holds what left the header.
  - The primary button follows the state: Testen → "Veröffentlichen · N Änderungen" → none.
  - The facts live on `ed.flow` (`easydrag-flow-state.js`).
  - The header budget is enforced at 8.
- **Part C.** `ED.runStop` paints the one Stop in the reserved slot, for the run in focus. The banner and drawer Stops go.
- **Part D.** The quick-add gets:
  - the header "Schritte nach: X";
  - suggestions: the user's own history first, then a curated list per category from the Go catalog (`suggest`);
  - "Alle anzeigen", which opens the palette scoped to the source step.

**Tech Stack:**
- vanilla JS classic scripts on `window.EasyDrag`; CSS custom properties;
- Go (`internal/flows`, `internal/tools`, `internal/server`);
- Node test scripts (`scripts/test-easydrag*.mjs`), Go tests in `ui` (source lints and go-rod browser tests);
- 16 locale files in `ui/lang/easydrag/`, changed only through the key files in `keys/`.

**Spec:** [`plans/2026-11-16-easydrag-w1c-spec.md`](../2026-11-16-easydrag-w1c-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) §6 Wave 1.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-a-server.md](part-a-server.md) | §3 | A.0 preamble · A.1 `Service.NextRun`, `ScheduleNexter`, `Store.NextTimerAfter` · A.2 Mission Control and the server share one function (`tools.FlowNextRunner`) · A.3 the "tested" marker · A.4 `CountChanges` (shared `countDiff`) · A.5 the API fields · A.6 the fixture fields, a Node test, the `internal/flows/AGENTS.md` note |
| [part-b-header.md](part-b-header.md) | §4 | B.0 preamble · B.1 facts on `ed.flow` (`easydrag-flow-state.js`) · B.2 `ED.kit.popover` · B.3 pill and popover replace the chips and Runs, Run now and Active · B.4 primary button and the secondary Test · B.5 header row, Stop slot, narrow layouts · B.6 budget at 8, browser tests, notes |
| [part-c-stop.md](part-c-stop.md) | §5 | C.1 `ED.runStop` and the header Stop for the run in focus · C.2 banner and drawer Stops removed, tests moved · C.3 browser tests for the one Stop · C.4 (below, this file) manual chapter 24 describes the new header |
| [part-d-quickadd.md](part-d-quickadd.md) | §6 | D.1 `suggest` in the Go catalog · D.2 quick-add header · D.3 history and suggestions · D.4 "Alle anzeigen" and the scoped palette · D.5 browser acceptance |
| [keys/](keys/) | | the locale key files (see "Shared files") |

## Order, worktrees and reviews

| Part | Worktree | Branch | Starts | Lands |
|---|---|---|---|---|
| A | `.worktrees/easydrag-w1c-server` | `feat/easydrag-w1c-server` | now | first |
| B, then C | `.worktrees/easydrag-w1c-header` | `feat/easydrag-w1c-header` | now, in parallel | after A, as one branch |
| D | `.worktrees/easydrag-w1c-quickadd` | `feat/easydrag-w1c-quickadd` | now, in parallel | last, after B+C |

- **Branches.** Cut every worktree from main after this plan's commit, so `plans/2026-11-16-easydrag-w1c/` and `keys/` are in it.
- **B waits for A once.** Before B.6 (its browser tests), rebase `feat/easydrag-w1c-header` onto main with Part A landed. If A has not landed when B reaches B.6, stop and wait. B.1 never runs `b1-fixture.mjs` (decision 22).
- **C runs in B's worktree**, after B.6.
- **Reviews.** There are three combined reviews by `superpowers:code-reviewer`: A, B+C together, and D. Each reports Critical and Important findings only, with no re-review rounds.
- **Rebase before landing.** Before landing, each branch rebases on main and re-runs its checks.

## Conventions that override the parts

The parts were written in parallel from one brief (`C:/disposable/easydrag-w1c-plan/BRIEF.md`). Where they differ, this section decides.

- **Shell and IMPACT.**
  - Git Bash. Every command starts with `cd <worktree> && …`. Never edit, stage or commit in the main checkout.
  - Run IMPACT before changing a function: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`. Warn on HIGH or CRITICAL. Confirm UNKNOWN with grep (the index reports `behind`).
- **DETECT.** Every commit body ends with `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, then a blank line and the Co-Authored-By line.
- **Commits.**
  - One per task, as each part prints it.
  - Message files go into `C:/disposable/easydrag-w1c/`; commit with `git commit -F <file>`.
  - The last line is the Co-Authored-By line for the model the implementer runs as.
  - No amend or push. The only rebases are B's one before B.6 and the pre-landing ones.
- **Helpers.** In `C:/disposable/easydrag-w1c/`:
  - `lib-patch.mjs` (`edit`, `replaceBlock`; keeps line endings);
  - `apply-plan-edits.mjs` (edit groups `<!-- edits:NAME -->`, run with the part file's path in the worktree);
  - `extract.mjs`.
  - Patch scripts that a part prints go into `C:/disposable/easydrag-w1c/patches/`. B's start with `b`, C's with `c-`; they never share a name.
- **Go.** Prefix every Go command with `GOTOOLCHAIN=go1.27.4`. Use `GOTMPDIR='D:\tmp\go-build'` for `ui`, `internal/flows`, `internal/tools` and `cmd/...`, and none for `internal/server`.
- **Browser tests.**
  - Set `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1c-shots`, and run with the Bash sandbox disabled.
  - Open pages only through `startEasyDragSmokeWith` / `startEasyDragSmoke`.
  - Re-run a failing test alone before counting it.
  - Before a full run, check for leftover headless Chromes (`Temp\rod\user-data\…`, see the memory note). Ask the owner before stopping any process you did not start.
- **Escapes.** Never put a backslash-u sequence in plan code. In Go use `"\x5c" + "u…"` or runes; in JS use `String.fromCodePoint`.
- **Gates before each commit.**
  - Go: `go vet` and `gofmt -l`.
  - JS and CSS: `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs`. The render baseline should take about 15–25 s; a slower one is a finding.
  - After a JS change, also the static ui tests: `GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short`. They hold the `TestEasyDragSplitSeams` pins, the menu lint, the plural lists and the literal ratchets.
  - T0 (`npm run test:easydrag:t0`) at the end of each part.
- **Line rule.**
  - `easydrag-editor.js` (896), `-interact.js` (753), `-home.js` (889) and `-dialogs.js` (309) may only get shorter. After B the editor has 847 lines, and C removes about 7 more.
  - `easydrag-canvas.js` stays at 928 lines or fewer.
  - `easydrag-runs.js` stays at 400 or fewer (`easyDragRunsLines`).
  - `easydrag-model.js`, `-template.js` and `-detail.js` change existing lines only.
  - `easydrag-palette.js` has no ceiling; D ends near 653 lines.
- **Tests.** Essential tests only. No external assets; `TestShippedWebAssetsLoadNothingExternal` stays green.

## Shared files

- **Locale files** (`ui/lang/easydrag/*.json`).
  - Change them only through the key files in `plans/2026-11-16-easydrag-w1c/keys/`:

    | Part | Key files, in task order |
    |---|---|
    | B | `b-pill.json` (B.3), `b-primary.json` (B.4) |
    | C | `c-1-stopping.json` (C.1), `c-2-drawer.json` (C.2) |
    | D | `d-2.json` (D.2), `d-3.json` (D.3), `d-4.json` (D.4) |

  - Plain keys go through `node C:/disposable/easydrag-w1a/locale-apply.mjs <file>`. The plural key `publish_with_changes` (`b-primary.json`) goes through `node C:/disposable/easydrag-w1a/locale-apply-forms.mjs <file>`.
  - Both helpers are idempotent. On a rebase conflict, take main's locale files and re-run the part's key files in task order.
  - The 15 new keys do not clash: no key is in two files, and the only existing keys touched are `run_stopping` (changed, de "Wird gestoppt…") and `run_stop_label` (deleted).
- **The browser fixture** (`ui/testdata/easydrag-fixture.js`) belongs to Part A (A.6).
  - Its defaults are neutral: `tested: false`, `changes: null`, `nextRun: null` (decision 9).
  - `changes` appears only for a flow with a live version, and `next_run` only on GET while `state.nextRun` is set.
  - B, C and D set these values in their own tests. They add other fixture state only under new names.
- **`easydrag-editor.js` and `easydrag-header.js`** belong to B and C (one branch, in order). D does not touch them; quick-add and palette talk over the bus event `palette-scope`.
- **Module registration.** Only B adds modules (`easydrag-flow-state.js`, `easydrag-pill.js`): `ui/js/desktop/core/module-loader.js`, the Go list `easyDragModules` and the Node harness lists.
- **Files that may conflict on a rebase.** `ui/css/desktop-app-easydrag.css` (B, C, D; CRLF), `scripts/test-easydrag.mjs` (each part adds a runner line) and `ui/desktop_easydrag_test.go` (B's module list and pins, C's pins). On a conflict keep both edits. CSS stays on tokens (`TestEasyDragLiteralRatchets`).
- **`internal/flows`.** A and D change different files: A the run, store, simulation and diff files; D `catalog.go` and the new `catalog_suggest.go`.
- **AGENTS.md files.**
  - Each file has its own rule:

    | File | Who edits it | State in the main checkout |
    |---|---|---|
    | `internal/flows/AGENTS.md` | A.6 | clean |
    | `ui/js/desktop/apps/AGENTS.md` | B.6 (one line, the old narrow-header rule) | carries another session's uncommitted edits |

  - The Part C line (`ED.runStop`) and the Part D lines (`palette-scope`, `aurago.easydrag.after`, `NodeTypeInfo.Suggest`) are added by the coordinator in each part's records step.
  - If the apps file still carries foreign edits when B+C lands, `git merge --ff-only` refuses. The coordinator then asks the owner before using the careful landing of W1b (`C:/disposable/easydrag-w1b/land/land-a.sh`). That approval was for W1b only.

## Coordinator decisions and amendments found while planning

Each item is applied in the named part. The coordinator copies them into the spec when the part lands.

**Part A, server data**
1. **Mission Control switches through an optional interface.** `nextFlowRun` delegates to the shared function when the hooks implement `tools.FlowNextRunner`, as AuraGo's real hooks do. The old cron plus timer combination stays for test doubles and for a server without flows, so the existing `internal/tools` tests stay green.
2. **The equality test** ("Mission Control equals `NextRun`") lives in `internal/server`, the only package that sees both services. `internal/tools` gets a delegation test.
3. **The three fields are top-level** in the answers, beside `enabled` and `draft_revision`, not inside `flow` (accepted). The editor copies them onto `ed.flow`.
4. **Blocks answer `tested` and `changes`** at `/api/desktop/flow-blocks/{id}` (GET and PUT), never `next_run`. This settles Part B's decision 4: the block adapter keeps the top-level fields (`easydrag-blocks.js:12`, `normalize`), so B's `ED.flowState.take` reads them with no client change. A block is tested after a reload.
5. **The save answer carries `next_run`** too; B takes it when present.
6. **"Tested" is durable** (coordinator decision, because the owner chose "Erfolgreich, dauerhaft").
   - A successful simulation and a test run that ends with success (`Service.onRunFinished`) both write the revision into `flows_meta` under `tested:<flow id>`. Only a higher revision replaces it.
   - `IsTested` keeps the `flow_runs` lookup as a second source.
   - Pruning old runs no longer makes a draft untested; `TestTestedSurvivesPruningOfTheTestRun` checks this.
7. **A partial test or simulation** (one step) marks the whole draft (accepted). This matches the publish dialog's evidence today; telling them apart would need a schema change.
8. **`DiffFlows` is split** into the count (`countDiff`) and the details, with the same result. Measured: 0.8 ms per save at 26 steps, 22 ms at 500 steps.
9. **Neutral fixture defaults** (coordinator decision, see "Shared files"). A.6 sets the values in its own Node test.

**Part B, header**
10. **B.3 holds the pill and its popover** together with the removal of Runs, Run now and Active, so no commit lacks those controls. B.4 is the primary button. That makes B.0–B.6 instead of the brief's B.1–B.7.
11. **The Stop slot** is a fixed-width box. Its button keeps `hidden` while idle, so every existing Stop check still reads `hidden`. C sets the width (`--ed-stop-slot-w`) from the real labels.
12. **"Testen" is the registry's `flow.test` label** ("Entwurf testen" / "Test draft"); blocks keep "Baustein simulieren".
13. **The pill's order** puts a viewed past run right after a run. A viewed run that is still running says "Läuft".
14. **A partial publish keeps Publish as the primary** (accepted). The popover also offers "Veröffentlichen abschließen".
15. **The "⋯" menu** no longer gains Runs and Test in a narrow editor (`headerNarrow` goes). Both stay reachable without it.
16. **`[data-ed-revisions]` moves onto the pill.** `revision_live` and `revision_ahead` become unused (follow-up).
17. **`ED.kit.popover`** returns `{el, close, update, isOpen}`.
18. **The count is a plural key** (`publish_with_changes`) applied with `locale-apply-forms.mjs`.
19. **The popover reads the flow again when it opens**, so a schedule changed since loading shows the right next run.
20. **The next run is shown in the browser's local time** (`ED.format.nextRun`); UX-13 settles the display with the DST note.
21. **Narrow header** (coordinator decision). At 560 px and below the right group may take two rows. Undo and Redo stay in the header and do not move into "⋯". C.3's position test compares states at one width, not widths.
22. **No fixture edit in B** (coordinator decision). B.0 step 0.1's "Part A has not landed" branch is void: B.1 never runs `b1-fixture.mjs`. B rebases onto main with A before B.6, as stated in "Order".
23. **B's decision 1** (the manual's header text) becomes Task C.4 below, after C.

**Part C, the one Stop**
24. **The parked own run is a third case of "in focus"** (accepted). While a past run is open, the editor's own running test is parked (`rc.parked.record`), and the header Stop stops it.
25. **`flow.stop`'s check is unchanged.** `runs.cancel()` already does nothing without a run in focus.
26. **A new helper file** `ui/desktop_easydrag_stop_helpers_browser_test.go` holds `stopsVisible`, `waitStop`, `clickStop` and `wantStops`.
27. **`extra4` tests call `stopRun`** with the run a row names (accepted). That harness has no header; the header path is covered elsewhere.
28. **Focus hand-over** (accepted). When the Stop disappears under the focus, the focus moves to "Zurück zum Entwurf" in a review, or to the canvas.
29. **A terminal own durable run** with a checkpoint cleanup is reachable only through its review (accepted).
30. **`run_stopping`** changes in German only ("Wird gestoppt…"); it also serves the durable status text.
31. **C is re-anchored on B.** Its patch scripts were re-verified on a scratch copy with B.1–B.6 applied (`b1-fixture.mjs` skipped).
    - The Node suite passes with 1256 `ok` lines, and the render baseline is unchanged.
    - The editor has 847 lines after B and 840 after C.
    - C sets `--ed-stop-slot-w` to 10.5rem, an estimate from the longest "Stopping" label (hi, 15 characters), and keeps the Stop on one line. C.3's label test measures it in a browser.
    - If an anchor still misses, C.1 step 1 says how to re-read B's header.

**Part D, quick-add**
32. **Three key files** (`d-2`, `d-3`, `d-4`), so each text lands with its use. "Abbrechen", "Schritt hinzufügen" and "Auslöser wählen" reuse existing keys.
33. **Nine curated rows**, including `planner` and `tool` (every `tool:*` step), at most 5 each. A step's own type is never offered after itself, and `logic.stop` gets none. The owner may re-order the `trigger` row later; the Go test keeps any edit valid.
34. **Tab in the quick-add search moves to "Alle anzeigen"** (accepted). Shift+Tab, Escape and Tab past the link close as before.
35. **Enter in the palette search adds the first match only while a scope is active** (accepted).
36. **"Alle anzeigen" is on every entry.** Without a source, it opens the palette with its search focused and no scope.
37. **A palette drag is not scoped**, but a successful drop ends an active scope.
38. **The scope banner's dot** is a separate muted span, not part of the translated text.
39. **The browser test sets `suggest` on the editor's copy** of the catalog, not in the fixture.
40. **History limits:** 8 types per source and 40 sources in `aurago.easydrag.after`.
41. **One placement function** (`placeStep`) serves quick-add and the scoped palette. The point is frozen when "Alle anzeigen" is clicked.

### Task C.4: manual chapter 24 describes the new header

**Files:**
- Modify: `documentation/manuals/en/24-easydrag.md`, `documentation/manuals/de/24-easydrag.md`

Do this task after C.3, in the worktree `.worktrees/easydrag-w1c-header`, with docs-agent. C.2 has already rewritten the places that named the banner and the list as Stops; this task covers the header around them.

- [ ] **Step 1: List the hits.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-header && grep -n -E '^- The header shows|\*\*Active\*\* switch|under \*\*Runs\*\*|\*\*Runs\*\* initially|\*\*Run now\*\* \(|Unpublished changes' documentation/manuals/en/24-easydrag.md | cut -c1-160; grep -n -E '^- Der Kopf zeigt|Schalter \*\*Aktiv\*\*|unter \*\*Läufe\*\*|\*\*Läufe\*\* zeigt|\*\*Jetzt ausführen\*\* \(|Unveröffentlichte' documentation/manuals/de/24-easydrag.md | cut -c1-160
```

Expected: the header bullet (en about line 221, de about 251), the "follows changes made elsewhere" bullet, the read-only bullet, the Runs bullet, the Run now bullet and the publish-dialog bullet that names the **Active** switch.

- [ ] **Step 2: Edit by hand, outside the generated blocks** (between the `flows-triggers` and `flows-steps` markers; do not edit inside them). Use the words the UI shows (read them from `ui/lang/easydrag/en.json` and `de.json` after B and C). The German text uses the lowercase "du" and the ledger words (`documentation/easydrag/voice.md`).
  - **The header bullet** names, in order:
    - **Back**;
    - the name;
    - the state pill. Name its states as the UI writes them: draft, live with version, live with a changed draft, paused, a missing link, a run, a viewed run. Say that clicking it opens a panel with **Active**, the next run, **Run now**, **Runs**, and, after an incomplete publish, the action that finishes it;
    - **Stop**, shown only while a run can be stopped, always at the same place;
    - **Test draft**;
    - the primary button: **Test draft** until the exact draft passed a test or simulation, then "Publish · N changes", and none on a published flow without changes;
    - **Undo**, **Redo** and **⋯**.
  - **Other bullets** that say the **Active** switch, **Runs** or *Run now* are "in the header" or "in the ⋯ menu" now say they are in the pill's panel. *Run now* also stays in the *Flow* menu and on the start-page card.
  - **Keep these phrases**, which `b5 U18` requires: `560 px wide or narrower` / `560 px oder schmaler`, `**Undo**` / `**Rückgängig**`. The narrow-width sentence now says that Runs and Run now are in the pill's panel, not in "⋯".

- [ ] **Step 3: Check.** Re-run the greps of Step 1. No line may describe Runs, Active or Run now as header buttons. Then run:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-header && node scripts/test-easydrag-ux-batch5.mjs 2>&1 | grep -E 'U18|FAIL' ; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -count=1 2>&1 | tail -2 ; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short 2>&1 | tail -2
```

Expected:
- the `b5 U18 manual chapter 24` check passes and no FAIL line appears;
- `ok … cmd/flowsdoc`: the generated tables still match the catalog;
- `ok … aurago/ui`: the "du" and ledger lints read locale files, not manuals, but they stay green.

- [ ] **Step 4: Commit.** Write `C:/disposable/easydrag-w1c/msg-c4.txt`:

```
docs(easydrag): manual chapter 24 describes the W1c header (W1c C.4)

The English and German chapters described the old header: state chips,
Runs, Run now and the Active switch as header buttons, and Runs and Test in
the "⋯" menu on a phone. They now describe the header W1c built: Back, the
name, the state pill whose panel holds Active, the next run, Run now, Runs
and the action that finishes an incomplete publish, the one Stop at a fixed
place, Test draft, the primary button that follows the state, Undo, Redo
and "⋯". The generated reference tables are unchanged.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-header && git add documentation/manuals/en/24-easydrag.md documentation/manuals/de/24-easydrag.md && git commit -F C:/disposable/easydrag-w1c/msg-c4.txt && git log --oneline -1
```

## Not covered by W1c (follow-ups)

- **Native-speaker check** of the 14 other locales for the W1c keys (pill, primary count, Stop, quick-add), together with W1a's and W1b's.
- **Unused keys** `revision_live` and `revision_ahead`. Delete them with their lint entries in a cleanup.
- **Red in the window menu bar** (desktop core), `goalCounter()`, and the header name-limit test (carried over from W1b).
- **The curated lists** can be re-ordered by the owner at any time.
- **W1d to W1f** (spec §10).

---

## Finishing W1c

1. **Rebase and re-check.** Rebase the branch onto the current main in its worktree. If locale files conflict, re-apply the part's key files in task order.
2. **Run the part's checks.**
   - T0.
   - For B+C and D: one full gated EasyDrag browser run, `go test ./ui -run '(?i)easydrag' -count=1 -timeout 40m -v`, including the W1a lab (zero findings). The run takes about 17 minutes; check for leftover rod Chromes first.
   - For A: the suites of its A.6 step 5, and T0.
3. **Check the main checkout.**
   - It must be on `main`: `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current`.
   - Compare its uncommitted files with the branch's files (see "AGENTS.md files").
4. **Land** with `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only <branch>`. Do not push without the owner's OK.
5. **Record.** After each part, the coordinator records on main through a temporary index:
   - the decisions above into the spec;
   - the landed range into the roadmap's Wave 1 status;
   - the AGENTS.md lines named in "Shared files";
   - the EasyDrag project memory.
6. **Clean up.** Remove the worktree and delete the merged branch.
