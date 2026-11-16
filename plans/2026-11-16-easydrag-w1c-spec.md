# EasyDrag W1c "Header and one Stop": design spec

- **Status:** W1c COMPLETE. All four parts landed on local main on 2026-11-16 and are not pushed: A 690fdacbd..0da8b7357, B+C b07de91d3..1a313bf33, D c675a4298..b8c94fe91. What implementation and review changed is in §12. The owner approved the design in the brainstorming session on 2026-11-16. Implementation plan: [`plans/2026-11-16-easydrag-w1c/`](2026-11-16-easydrag-w1c/README.md) (2026-11-16; four parts A–D, 41 coordinator decisions and amendments in its README).
- **Roadmap:** `plans/2026-11-09-easydrag-next-roadmap.md`:
  - E4 "Calm chrome, confident publishing" (lines 496-559): UI-14 with CS-07, the one Stop, the F.2 next-fire accessor;
  - E9 "Build with real things" (lines 811-857): the UX-04 quick-add header slice;
  - §2 principle 3 "One next action" (line 96);
  - §6 Wave 1 gates: exactly one visible Stop per window, equal control positions before and after a run at 390, 900 and 1440 px, the header control budget.
- **Predecessors:** W1a (status map, kit, format, icons, lab) and W1b (action registry `ED.actions`, voice, control budgets), both complete and pushed (origin main 7e1ecc6c3). W1b's budget test holds the header target of 8 for W1c to enforce.

## 1. Goals and owner decisions

W1c gives the editor a calm header: one pill that says where the flow stands, one button for the next step, and one Stop that never moves. It adds the read-only "next run" that later surfaces read, and a quick-add that knows what it adds after.

Owner decisions (2026-11-16):

| # | Question | Decision |
|---|---|---|
| 1 | What "tested" means for the primary button | **A successful test or simulation of exactly the current draft revision, remembered by the server.** It survives a reload. A failed test keeps "Testen" as the primary action. |
| 2 | Where "N Änderungen" comes from | **From the server, after each save,** computed by the same diff as the publish dialog, so the button and the dialog always agree. |
| 3 | Header layout | **Zurück, Name, Pill, Stop slot, "Testen", primary button, Rückgängig, Wiederholen, "⋯".** "Aktiv", "Jetzt ausführen", "Läufe ansehen" and the next run move into the pill popover. Idle: 7 controls, 8 with "Veröffentlichen". The "⋯" menu does not grow. |
| 4 | The other Stop buttons | **Only the header slot.** The run-review banner and the runs list lose their Stop; a running row opens its run, which the header Stop then stops. |
| 5 | Quick-add suggestions | **Your own history first, then a curated list per category,** at most 5. |
| 6 | The next-run accessor | **One shared function** over both sources (cron schedules and date-and-time timers). Mission Control calls the same function. |
| 7 | How to cut W1c | **Four parts:** A (server data), B (header), C (the one Stop, after B, in B's worktree), D (quick-add). A, B and D start in parallel; A lands first (§8). |

## 2. Scope

**In W1c:** §3 to §6.

**Not in W1c:**
- the publish review content (UX-13, Wave 3: next-run line, change marks on the canvas, schedule diff, DST note). W1c only provides `next_run`;
- the bottom-bar Stop below 560 px (E16, Wave 4). Until then the header slot serves every width;
- the selection bar (E4, Wave 3), the inspector form (UI-16, Wave 2);
- quick-add synonyms, favourites first and ">" commands (UX-04, Wave 4);
- the home page and its card targets (W1f), run marks (OP-02), the attention endpoint (Wave 3).

**The Wave 1 gates that W1c closes:**
- exactly one visible Stop per window;
- equal control positions before and after a run at 390, 900 and 1440 px;
- the header control budget (W1b's budget row "header" becomes enforced at 8).

## 3. Part A: server data (F.2, "tested", change count)

### 3.1 Today

- **Next run.** Mission Control's `MissionManagerV2.NextRun` (`internal/tools/missions_v2.go:2453`) calls `nextFlowRun` (`internal/tools/missions_v2_flow_runs.go:418-459`), the minimum of two sources: schedule triggers from the cron engine (`cron.NextRun(flowCronJobID)`, runtime jobs from `missions_v2_flows.go:609-648`) and date-and-time triggers from the flow store (`hooks.NextFlowRun` → `Service.NextTimer`, `internal/flows/service_missions.go:92-102`, store `NextTimerAt`, `store_misc.go:160-177`). A disabled flow has no next run. `NextTimerAt` returns the raw stored `fire_at`, which can lie in the past (a fire in retry). The cron engine returns nothing before it has started. Both use `time.Local` today, by coincidence rather than a shared setting. The flow API (`GET /api/desktop/flows`, `GET /api/desktop/flows/{id}`, `internal/server/flows_handlers.go:611, 692`) returns no next run.
- **Tested.** Only in memory: `ed.testEvidence` (`easydrag-run-stream.js:324` for a final test-mode run, `easydrag-run-start.js:216` for a simulation, `easydrag-blocks.js:154` for blocks), lost on reload, failed runs included. Its one reader is the publish dialog's `exactTest` (`easydrag-publish.js:186-188`).
- **Change count.** None. The client has `has_unpublished_changes` (home) and the revisions; the real diff (`added_nodes`, `changed_nodes`, `removed_nodes`, `changed_edges`) comes only from the publish preview (`ed.api.preview()`, `easydrag-publish.js:150`).

### 3.2 The next-run function (F.2)

- **`flows.Service.NextRun(flowID string) (NextRun, bool)`** with `NextRun{At time.Time, NodeID string}`.
  - **Sources:** the date-and-time timers of the flow store, and the schedule triggers through a narrow interface `ScheduleNexter` (one method: the next fire of a flow's schedule jobs) that the server injects from the cron side. The flows package does not import the cron package.
  - **Definition of "next":** the earliest fire time **after now** of the **live version** while the flow is **active**. A paused flow, a flow without a live version and a flow without time-based triggers give nothing. A stored fire time that has passed (in retry or not yet processed) is not "next".
  - **Locking:** read-only; no flow lock (the bridge may call it synchronously); the store read is lock-free.
- **Mission Control** replaces the body of `nextFlowRun` with a call to this function, so Mission Control and EasyDrag show the same time by construction.
- **API:** `GET /api/desktop/flows/{id}` returns `"next_run": {"at": "<RFC 3339 in the server's location>", "node": "<node id>"}` or no field. The list endpoint does not get it in W1c (UI-04, W1f).

### 3.3 The "tested" marker

- **Rule:** a test or simulation of the flow's draft that finishes **successfully** for revision R marks R as tested. Any new draft revision is untested. A failed test or simulation does not mark.
- **Where it shows:** `"tested": true|false` in `GET /api/desktop/flows/{id}` and in the save response (the response that carries the new draft revision).
- **Storage:** the plan's first step checks whether stored test runs carry the draft revision and whether simulations are stored.
  - If test runs carry it, "tested" for real tests is derived from the newest successful test run of revision R (no new storage).
  - For simulations that are not stored today, the plan uses the smallest record that fits the existing tables.
  - **No schema migration in W1c** (the schema roadmap places them in Waves 3, 5 and 7b). If only a migration fits, the plan stops and asks the owner.
- **Blocks:** the block editor's simulation marks the same way.

### 3.4 The change count

- **`"changes": {"steps": N, "connections": M}`** in `GET /api/desktop/flows/{id}` and in the save response, for a flow with a live version. `steps` = added + changed + removed steps; `connections` = changed connections. Computed by the same function the publish preview uses for its diff, so the button and the dialog agree.
- A flow that was never published has no `changes` field.
- **Cost:** one diff per save. The plan measures it on the largest fixture flow; flows are small.

### 3.5 Tests (Part A)

- **Go, `internal/flows`:** `NextRun` with a date-and-time trigger, with a schedule trigger (fake `ScheduleNexter`), with both (earliest wins), paused, unpublished, a stored fire in the past (skipped), no time-based trigger.
- **Go, `internal/tools`:** Mission Control's next run equals `NextRun` for the same flow (the existing `missions_v2_flow_runs` tests stay green).
- **Go, `internal/server` or `internal/flows`:** the marker (success marks R; failure does not; a save makes it false; a simulation marks), and the count (equals the publish preview's diff for the same draft).

## 4. Part B: the header

### 4.1 Today

- **Markup** (`easydrag-editor.js:52-84`): back, `.ed-title` (name input, state chip, revision text, save chip), Undo/Redo, then `.ed-head-actions`: runs, test, cancel (hidden), publish, run now (hidden unless live), active switch, more. Run-view banner (67-71), footer (80-83).
- **Renderer** (`easydrag-header.js`, 200 lines): `stateOf` (20-28), extra chips paused, block, live-outdated ("Republish needed"), timers-off (93-94, 68-83), revision text (31-38), `paintActions` (43-49).
- **CSS** (`desktop-app-easydrag.css`): `.ed-head` 618-621, container queries at 900 px (1407-1421) and 560 px (1433-1451).

### 4.2 Layout

Left: Zurück, Name, Pill. Right, a right-aligned group: Stop slot, "Testen", primary button, Rückgängig, Wiederholen, "⋯". The footer (last run, issues) stays.

- The Stop slot has a fixed reserved width (the wider of "Stoppen" and "Wird gestoppt…" at the header font) and is invisible but space-taking when idle.
- The pill has a maximum width and truncates with an ellipsis and a tooltip; the name input keeps its W1a sizing. Nothing on the left can push the right group.
- Narrow widths keep the same order. Labels may hide below 900 px as today, except the Stop label (§5) and the primary button's label.

### 4.3 The pill

One button with a W1a status disc and one short text. A fixed order picks the text when several apply: run, then problem, then pause, then version state.

| State | Disc | Text (de) |
|---|---|---|
| a run of this flow is in progress (own or viewed) | running | "Läuft" |
| publishing incomplete (`FLOW_PUBLISH_INCOMPLETE`) | blocked (amber) | "Veröffentlichen unvollständig" |
| Mission Control link missing (`FLOW_MISSION_MISSING`) or live issues | blocked | "Verknüpfung fehlt" / "Neu veröffentlichen nötig" |
| date-and-time triggers not running | uncertain (amber) | "Zeit-Auslöser laufen nicht" |
| live, switched off | paused (grey) | "Pausiert · v2" |
| live, draft changed | ok | "Live v2 · Entwurf geändert" |
| live, unchanged | ok | "Live v3" |
| never published | not_run (grey) | "Entwurf" |
| past run open | — | "Vergangener Lauf" |

The texts come from status-map words and new keys; the version reads "vN" (W1a CS-01).

### 4.4 The pill popover

A small anchored panel (new kit piece `ED.kit.popover(anchor, content)`: focus moves in, Escape and an outside click close it, focus returns to the pill). Contents, all through the action registry:

- **Aktiv** (`flow.active`, a switch);
- **Nächster Lauf:** "Mo, 07:00" from `next_run` via `ED.format`; "Kein geplanter Lauf" when there is none; "Pausiert" when switched off;
- **Jetzt ausführen** (`flow.run_now`) and **Läufe ansehen** (`view.runs`);
- **the action that completes a partial publish:** "Veröffentlichen abschließen" (opens the publish dialog) for `FLOW_PUBLISH_INCOMPLETE` and live issues, "Verknüpfung reparieren" (`flow.repair_mission`) for `FLOW_MISSION_MISSING`.

Blocks have no Aktiv, next run or run now; their popover shows the version state and, when relevant, the completing action.

### 4.5 The primary button

| State | Primary (filled) | Beside it |
|---|---|---|
| changes not tested (or never tested) | **Testen** | — |
| tested, changes present | **Veröffentlichen · 3 Änderungen** ("Veröffentlichen" when never published or the count is missing) | Testen |
| clean live flow | — | Testen |
| read-only, past run | — | Testen (`aria-disabled` with its reason) |

- "Testen" is `flow.test` (the block editor: "Baustein simulieren"); the primary "Veröffentlichen" is `flow.publish` and always opens the full publish dialog, never publishes directly (E4 guardrail).
- At most one filled button per screen (principle 3).
- The count is the sum `steps + connections` from §3.4; the label uses the W1a plural forms.

### 4.6 Budget

W1b's budget row "header" switches to enforced with ceiling = target = 8 (`ui/desktop_easydrag_budget_browser_test.go`, table `easyDragControlBudgets`). The "⋯" row keeps its ceiling of 12.

### 4.7 Tests (Part B)

- **Node:** the pill text and disc for every row of §4.3; the primary button for every row of §4.5 (fixture `tested` and `changes`); the popover contents per state.
- **Browser:** the popover opens and closes by keyboard; "Aktiv" and "Jetzt ausführen" in the popover work and show the registry reasons; the budget test passes with the header enforced at 8.

## 5. Part C: the one Stop

### 5.1 Today

Three kinds of Stop: the header `[data-ed-cmd=cancel]` (`easydrag-editor.js:61`, shown while `runs.isRunning()`), the run-review banner `stop-viewed` (`easydrag-editor.js:69`, `renderBanner` 151-173), and one per active row in the runs drawer `data-ed-run-stop` (`easydrag-run-drawer.js:33-36`). They can be visible together. `easydrag-runs.js:54-61` (`stopButtons`) and `easydrag-run-stop.js` (`setStopPending` 98-121, `confirmStop` 244-251) serve them.

### 5.2 The component

- `easydrag-run-stop.js` owns one Stop button in the header's reserved slot. It keeps `data-ed-cmd="cancel"`.
- **The run in focus:** in the run review, the viewed run when it is still active (durable runs included, as `stop-viewed` did); otherwise the editor's own running run. With no run in focus the slot is empty (and keeps its width).
- **Label:** always visible at every width, also 320 px: "Stoppen", while stopping "Wird gestoppt…" (`aria-busy`).
- **Confirmation:** live and durable runs keep the existing confirmation; a test run stops at once, as today.
- **Registry:** `flow.stop` covers both cases; its reason when nothing can be stopped stays `reason_stop_unavailable`.

### 5.3 What goes

- The banner's `stop-viewed` button. The banner keeps "Mit diesen Daten erneut testen" and "Zurück zum Entwurf".
- The runs drawer's row Stops. A running row opens its run in the review; the header Stop then stops it.
- `stopButtons` collects only the one button.
- Tests that click `stop-viewed` or row Stops (`ui/desktop_easydrag_browser_test.go:719-740`, `ui/desktop_easydrag_durable_browser_test.go:34, 49-57`, `scripts/test-easydrag-durable.mjs`, `-extra4.mjs`, `-extra5.mjs:713`, `-extra6.mjs:725, 745`) move to the header Stop; none is weakened.

### 5.4 Tests (Part C)

- **Exactly one visible Stop** (Go browser): with the own test running; with a running run open in the review; with the runs drawer open while a run is going.
- **Equal positions:** the x position of every header control is the same idle, during a run and after it, at 390, 900 and 1440 px (wait for fonts and layout to settle before measuring).
- **Label:** the Stop's text is visible (not clipped, not hidden) at 320, 760 and 1440 px.

## 6. Part D: the quick-add

### 6.1 Today

`openQuickAdd(ed, canvas, req)` (`easydrag-palette.js:358`), request `{x, y, from?, edge?, filter?, restoreFocus?}`. No header element (the title is an `aria-label`); list ordering is "recent" (global, `aurago.easydrag.recent`, max 6) then catalog order; fixed size 320 × 380 (`:408-415`). No suggestion data exists (`internal/flows/catalog.go:60-90`). The full palette's `addByClick` (`:220-236`) attaches to the selection only.

### 6.2 Header

A real header row at the top of `.ed-quick`:

| Entry | Header |
|---|---|
| from a step's "+", a card's add tool, Ctrl+K with one step, a wire pulled from a port | "Schritte nach: {name}" |
| Insert on a wire | "Einfügen nach: {name}" (the wire's source step) |
| a click on the empty canvas, the canvas menu | "Schritt hinzufügen" |
| the empty-canvas trigger button | "Auslöser wählen" |

`{name}` is the step's label (or its type's label), escaped. The dialog's `aria-label` follows the header text. The height grows by the header row; `position()` uses the real height.

### 6.3 Suggestions

A "Vorschläge" group of at most 5 entries above the rest, only when the quick-add has a source step:

1. **Your history on this device:** pairs (source type → added type) counted in `localStorage['aurago.easydrag.after']`, written by `place()` when a step is added after a source. Highest count first.
2. **Curated:** a short list per source category in the Go catalog (for example after a trigger: KI-Schritt, Websuche, HTTP-Anfrage, Telegram senden), served with the catalog. A Go test checks that every listed type exists and is not a trigger.

No entry appears twice; the rest of the list follows as today without the suggested ones. Unavailable types (not set up) appear greyed out as in the palette (W1b grayscale).

### 6.4 "Alle anzeigen"

- A link at the bottom of the quick-add. It closes the quick-add (without returning focus to the canvas) and opens the full palette with the search focused.
- **Scope:** the palette receives the quick-add's source (`from` or `edge`) and shows a small banner "Fügt nach {name} ein · Abbrechen". The next step chosen in the palette (click or Enter) is placed after that source exactly as the quick-add would place it. Cancel, Escape or adding one step ends the scope.

### 6.5 Tests (Part D)

- **Node:** the header text per entry; the suggestion order (history before curated, at most 5, no duplicates, unavailable greyed); "Alle anzeigen" opens the scoped palette and the next click attaches after the source.
- **Go:** the catalog suggestion list is valid.
- **Browser** (roadmap acceptance, E9): from a trigger's "+", the header reads "Schritte nach: <trigger label>" and suggestions come first; "Alle anzeigen" opens the full palette.

## 7. Testing

- **Per commit:** `go vet` and `gofmt -l` for Go; `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs` for JS and CSS (the render baseline's wall time is watched, not only its counters).
- **Per part:** T0 (`npm run test:easydrag:t0`); Part A also `go test ./internal/flows/... ./internal/tools -run '(?i)mission|flow' ./internal/server -run '(?i)flow'`.
- **On landing:** the full EasyDrag browser run including the W1a lab (zero findings); before it, a check for leftover headless rod Chromes.
- **Review:** one combined review per part, Critical and Important only. The review of B+C must include a check with desktop animations on.

## 8. Order and parallel work

- **Worktrees:** A (server), B+C (header, then Stop, one worktree, C's tasks after B's), D (quick-add).
- **Start:** A, B and D in parallel. B builds against the API contract of §3 with fixture data.
- **Landing:** A first (B needs its fields), then B+C, then D (D touches only the palette, the catalog and its keys, and may land earlier).
- **Shared files:** locale files through key files; `easydrag-editor.js` is touched by B and C only; `easydrag-palette.js` by D only; `internal/server/flows_handlers.go` by A only.

## 9. Acceptance checklist

- [x] `next_run` in the flow API equals Mission Control's next run for the same flow; schedules and date-and-time triggers both count; paused and unpublished flows have none.
- [x] `tested` turns true after a successful test or simulation of the current draft revision, survives a reload, and turns false on the next save; a failed test leaves it false.
- [x] `changes` equals the publish preview's diff counts.
- [x] The header shows Zurück, Name, Pill, Stop slot, Testen, primary, Rückgängig, Wiederholen, "⋯"; the budget test enforces 8.
- [x] The pill shows the states of §4.3; its popover holds Aktiv, the next run, Jetzt ausführen, Läufe ansehen and the completing action.
- [x] The primary button follows §4.5; a clean live flow shows no filled button.
- [x] Exactly one Stop is visible per window in the three situations of §5.4; its label shows at 320, 760 and 1440 px.
- [x] No header control moves when a run starts or ends at 390, 900 and 1440 px.
- [x] Quick-add shows its header, suggestions first, and "Alle anzeigen" opens the scoped palette.
- [x] T0 and the full browser run are green; the lab has zero findings.

## 10. Risks

- **Simulations and the "tested" marker.** If simulations leave no stored record, the plan must find a fitting existing record; a schema migration needs the owner's decision.
- **Mission Control switching to the shared function** touches `internal/tools`. IMPACT first; its tests stay green.
- **Tests that click the removed Stops** are listed in §5.3 and move to the header Stop.
- **Position test flakiness** (fonts, late layout): measure after fonts and layout have settled.
- **Cost of the change count** on every save: measured; if too slow on large flows, the save response leaves it out and the editor asks for it after a pause instead (same function).
- **Line rule** (W1b limits): `easydrag-editor.js` ≤ 896 (859 now), `-interact.js` ≤ 753, `-home.js` ≤ 889, `-dialogs.js` ≤ 309, `-canvas.js` ≤ 928.

## 11. Left for later sub-projects

- UX-13 (Wave 3): the publish review shows `next_run`, the schedule diff and the DST note.
- E16 (Wave 4): the bottom-bar Stop below 560 px.
- UI-04 and W1f: `next_run` on the home cards.
- UX-04 (Wave 4): synonyms, favourites first, ">" commands in the quick-add.

## 12. Changes from implementation and review

**Planning.**
- The plan README records 41 coordinator decisions and amendments. Among them:
  - "tested" is a marker in `flows_meta` (`tested:<id>`), written by successful simulations and by test runs that end with success, so pruning old runs does not make a draft untested (no migration);
  - a one-step test counts for the whole draft;
  - the three facts are top-level fields, for blocks too (without `next_run`);
  - the fixture defaults are neutral;
  - the phone header takes two rows at 560 px and below;
  - the Stop slot is 10.5rem wide;
  - the parked own run is a case of "in focus";
  - Tab in the quick-add leads to "Alle anzeigen";
  - nine curated rows, including `planner` and `tool`.

**Implementation.**
- The pill's class is `ed-head-pill`: `ed-status-pill` is on the status map's retired list.
- The pill's background is `--ed-panel-strong`, because the Voxel World button material gave its disc 1.03:1 contrast.
- The phone header's right group takes exactly two rows: Stop and Test, then the primary, Undo, Redo and "⋯".
- Removing the Publish dot lowered the radius ratchet from 79 to 78 and dropped one colour pin.
- The actions test counts `flow.run_now` on 2 surfaces. The header lost Run now; the Node checks pin the popover's reasons.

**Review A.** No Critical or Important findings.

**Review B+C.** Seven Important findings, all fixed:
1. The change count is gated on the revision it was read for. `take` keeps only `next_run` from an answer for another revision.
2. After a publish, the focus returns to Test when Publish is hidden.
3. Tab past the popover's last entry, or Shift+Tab before its first, closes it and returns to the pill.
4. The pending Stop is `aria-busy`/`aria-disabled` and never `disabled`, so the focus hand-over works from the keyboard.
5. A parked own run makes the pill say "Läuft".
6. **Owner decision of 2026-11-16, "always visible", amending §4.5:**
   - "Veröffentlichen · N Änderungen" stays visible in its place whenever there is something to publish (never published, a changed draft, an incomplete publish), and is hidden only on a clean live flow or read-only.
   - Before a passed test, Test is the filled button; after it, Publish is.
   - Publishing an untested draft from the header stays possible, and the dialog shows the evidence.
   - Nothing in the header moves when a test ends. `TestEasyDragStopPositionsPublishBrowser` checks this at 390, 900 and 1440 px.
   - The budget test counts this fullest state: 8 of 8.
7. The UX test checks the counted Publish at every width again.

**Review D.** Three Important findings, all fixed:
- the history keeps learning past 8 types: the type just used is always kept;
- scoped Enter and clicks add only steps that can follow the source: the check is `fits`, and a refused step keeps the scope;
- "Alle anzeigen" passes on the opener's `restoreFocus`.

**Landing evidence.**
- **A:** T0 passed with 49 steps; the flows, tools and server suites were ok after two rebases.
- **B+C:**
  - The full EasyDrag browser run had 137 PASS, 0 FAIL and 2 SKIP, with 0 lab findings, after the rebase on main that brought another session's session-action code into `easydrag-editor.js` (878 lines, limit 896).
  - T0 passed with 51 steps. One earlier T0 failed once in `internal/flows` under load; the package passed alone, and the full T0 passed again.
- **D:** the full run had 138 PASS, 0 FAIL and 2 SKIP, with 0 lab findings, and T0 passed with 52 steps.

**Open.**
- A native-speaker check of the 14 other locales for the W1c keys.
- The unused keys `revision_live` and `revision_ahead`.
- The scoped palette's refusal reuses `connect_port` ("These ports cannot be connected."); a more specific text needs a key.
- The 390 px header is tall: the left part stacks name, pill and save state.
- `TestEasyDragActionsAgreeBrowser` failed once under load and then passed 3 of 3 runs alone.
