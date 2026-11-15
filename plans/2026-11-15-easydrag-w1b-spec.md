# EasyDrag W1b "Voice and actions": design spec

- **Status:** design spec, 2026-11-15. The owner approved the design in the brainstorming session on 2026-11-14/15. Implementation plan: [`plans/2026-11-15-easydrag-w1b/README.md`](2026-11-15-easydrag-w1b/README.md). Its "Amendments found while planning" take precedence until they are copied in here.
- **Roadmap:** `plans/2026-11-09-easydrag-next-roadmap.md`:
  - §6, Wave 1 "Readable and honest" (lines 1412-1449);
  - E2 "One visual language" (lines 350-416): the action registry (line 371), UX-18 and CS-20 (line 377), CS-17 (line 380);
  - §10 decision 1 (lowercase "du", line 1940);
  - §9: CS-17 is a gate only for the header and home control counts (line 1933).
- **Predecessor:** W1a "Foundations", complete and in origin/main (spec `plans/2026-11-13-easydrag-w1a-spec.md`, §2 lists the Wave 1 split). W1b builds on its status map, kit, icon map and formatting.

## 1. Goals and owner decisions

W1b gives every action one name, one shortcut and one reason for being unavailable; gives EasyDrag one voice; and puts a number on how many controls the header and the home may show.

Owner decisions (2026-11-14/15):

| # | Question | Decision |
|---|---|---|
| 1 | Notifications about the user's own actions | **Only where the result is invisible.** A visual notification goes when the screen already shows the result. Where the result is invisible (the clipboard), a short confirmation appears at the control itself. Screen-reader announcements stay, and so does the delete toast with "Rückgängig". |
| 2 | Limits | **Shown from 90 %.** Counters appear at 90 % of the limit. Rule hints shown in advance become the error message when an input breaks the rule. Where an input is capped silently today, a short "Höchstens 120 Zeichen." appears when the cap is reached. |
| 3 | Control budgets | **A ratchet now plus fixed targets.** The test counts today's controls and lets the numbers only fall. The targets that W1c and W1f must reach: header ≤ 8, home ≤ 6, published flow card ≤ 3 (§5.2). |
| 4 | Depth of the action registry | **Full: description and logic.** One table for every action: label, icon, shortcut, availability with a reason, risk, and the execution itself. Every surface lists action ids. |
| 5 | "Baustein" means two things | **The palette is "Schritte".** "Baustein" (en "block") means only the reusable sub-flow (§4.4). |
| 6 | How much copy W1b rewrites | **The rules plus the error texts.** W1b writes the rules and lints, converts the capital "Du" forms and rewrites the about 140 error and recovery texts. W1c, W1d and W1f rewrite the header, test-loop and home texts when they rebuild those surfaces. |
| 7 | How the registry is built | **A table plus logic bound by the modules.** `easydrag-actions.js` holds label, icon, shortcut, risk and the shared checks with their reasons. The module that owns an action binds its execution with `ED.actions.bind(id, …)`. |
| 8 | How to cut W1b | **Three parts.** C (budgets) lands first; A (registry) and B (voice) run in parallel (§7). |

## 2. Scope

**In W1b:**
- Part A, the action registry (§3);
- Part B, the voice: the rules, the "du" lint, the error texts, the concept ledger, own-action notifications and limits (§4);
- Part C, the control budgets (§5).

**Not in W1b:**
- the header rebuild, the one Stop and the primary-action states (W1c). W1c renders the header through the registry and enforces the header target;
- the selection bar on the canvas (E4). It reads the registry when it lands;
- Ctrl+K command mode and ">" commands (UX-04, Wave 4) and the Spotlight flows source (OP-11, Wave 6a). Both read the registry;
- header, test-loop and home copy (W1c, W1d, W1f), written against the §4.1 rules;
- the home rebuild (W1f). W1f enforces the home and card targets;
- the home hero buttons and the home bulk bar. They are not menus today; W1f adds registry rows when it builds "Neu ▾" and "Importieren…";
- retiring the rail (roadmap decision 2).

**The Wave 1 gates that W1b closes:**
- the header and home control budgets exist as a test (W1c and W1f enforce their targets);
- for every action, the context menus and the header "⋯" show the same label, shortcut and disabled reason (E2 acceptance, roadmap line 390). The selection bar joins when it exists.

## 3. Part A: the action registry

### 3.1 Today

Eleven surfaces build their own entries (research 2026-11-14):
- the window menus (`easydrag-editor.js` `setMenus`, lines 502-561);
- the header "⋯" (`easydrag-editor.js` `moreMenu`, 428-460);
- the card context menu and the empty-canvas menu (`easydrag-interact.js` `openNodeMenu` 526-553, `openCanvasMenu` 555-568);
- the card tools (`easydrag-canvas.js` 478-498);
- the home card menu (`easydrag-home.js` `cardMenu`, 677-694);
- the Lebendigkeit submenu (`easydrag-delight.js` `menuItems`, 79-92);
- the shortcut table (`easydrag-dialogs.js` `SHORTCUTS`, 9-39);
- the keyboard handlers (`easydrag-editor.js` `onKeyDown` 721-752, `easydrag-interact.js` `handleKey` 664-697).

Each re-derives its labels and checks inline. The same action carries different labels ("Test", "Test draft", "Test this step"), different checks (Copy is refused in read-only mode on the Edit menu and allowed on the card menu) and different shortcut spellings ("Del" in the menu, "Delete" in the table). Only "Run now" says why it is unavailable. No EasyDrag entry uses the desktop's `danger` styling.

### 3.2 The table

New module `ui/js/desktop/apps/easydrag-actions.js`, `ED.actions`. One row per action:

| Field | Meaning |
|---|---|
| `id` | `group.name`, for example `edit.duplicate` |
| `label` | the one label key (under `easydrag.ui.`) |
| `icon` | an alias from the W1a icon map (`ED.icons`) |
| `shortcut` | the canonical shortcut, for example `Ctrl+D`, `Delete`, `Shift+1`; or none |
| `scope` | `editor` (anywhere in the editor window except text fields) or `canvas` (only while the canvas or a card has focus; single-letter keys and Delete) |
| `risk` | `safe`, `undoable` or `confirm` (§3.5) |
| `needs` | the shared checks (§3.3) |

The actions:

| Group | Actions |
|---|---|
| Flow | `flow.home`, `flow.save` (Ctrl+S), `flow.test` (Ctrl+Enter), `flow.run_now`, `flow.stop`, `flow.publish`, `flow.active`, `flow.versions`, `flow.settings`, `flow.organize`, `flow.describe`, `flow.repair_mission`, `flow.duplicate`, `flow.export`, `flow.export_pack`, `flow.export_email`, `flow.mission_control`, `flow.delete`, `flow.open` |
| Edit | `edit.undo` (Ctrl+Z), `edit.redo` (Ctrl+Shift+Z), `edit.cut` (Ctrl+X), `edit.copy` (Ctrl+C), `edit.paste` (Ctrl+V), `edit.duplicate` (Ctrl+D), `edit.delete` (Delete), `edit.select_all` (Ctrl+A), `edit.extract`, `edit.auto_layout` (Alt+Shift+F), `edit.group`, `edit.add_step` |
| Step | `step.open` (Enter), `step.test`, `step.toggle` (D), `step.connect` (C), `step.add_next`, `step.align_horizontal`, `step.align_vertical` |
| View | `view.zoom_in` (+), `view.zoom_out` (−), `view.zoom_fit` (Shift+1), `view.zoom_reset` (Ctrl+0), `view.find_step` (Ctrl+F), `view.search` (Ctrl+K), `view.palette`, `view.runs`, `view.shortcuts` (?), `view.liveliness` (submenu: `view.liveliness_normal`, `view.liveliness_calm`) |

- The plan may add a row when it finds an action this list missed. It may not leave an action that a menu shows outside the registry.
- The home card menu uses the `flow.*` rows with the card's flow as context.
- The panel width menu (`easydrag-workspace.js` 98-101) offers pixel sizes, not actions, and stays outside. The lint (§3.7) names it as its one exception.

### 3.3 Checks and reasons

Each check in `needs` has a fixed set of causes, and each cause has exactly one reason text. That makes the reasons identical on every surface.

| Check | Cause | Reason (de) |
|---|---|---|
| `writable` | a past run is open (run view) | "Im Lauf-Rückblick kann nichts geändert werden." |
| | read-only mode is on | "Der Nur-Lese-Modus ist an." |
| | the agent permissions lock the flows (home `createOff`) | "Durch die Agenten-Rechte gesperrt." (the existing `blockReason` text) |
| `free` | a dialog or overlay is open (`overlayOpen`) | "Schließe zuerst den offenen Dialog." |
| | a version is being restored (`restoring`) | "Eine Version wird gerade wiederhergestellt." |
| `selection` | nothing is selected | "Wähle zuerst einen Schritt." |
| `published` | the flow has no published version | "Erst nach dem Veröffentlichen möglich." |
| `active` | the flow is paused | "Der Flow ist pausiert." |
| `idle` | a run is in progress | "Ein Lauf läuft gerade." |
| `steps2` | fewer than two steps | "Braucht mindestens zwei Schritte." |

- The plan maps each cause to the state it reads today and writes the reason keys in all 16 locales. The wording follows §4.1.
- A bound action may add its own check with its own reason (§3.4), for example "Nichts zum Einfügen." for paste, "Nichts rückgängig zu machen." for undo, or the pending run-now of a home card.
- Copying changes nothing, so `edit.copy` has no `writable` check and works in read-only mode on every surface.

### 3.4 Binding and output

- **`ED.actions.bind(id, { run, check, label })`.** The owning module binds the execution. `check(ctx)` returns a reason key or nothing. `label(ctx)` is allowed only where the context names a different object: `flow.test` reads "Baustein simulieren" in the block editor, and `step.toggle` reads "Einschalten" or "Ausschalten" by the step's state.
- **`ED.actions.state(id, ctx)`** returns `{ enabled, reason }`. It runs the shared checks in `needs` order, then `check`.
- **`ED.actions.run(id, ctx)`** checks the state again and runs only when enabled. Every surface and every shortcut runs through it.
- **`ED.actions.items(ids, ctx)`** returns desktop menu entries (`showContextMenu` and `setWindowMenus` format): label, icon, the shortcut for display, `disabled`, `disabledHint` with the reason, `danger` for risky actions, separators for `'-'`, submenus for `view.liveliness`. An action whose `bind` is missing in this editor (for example `flow.repair_mission` without a Mission Control problem, `edit.extract` in the block editor) is left out.
- **`ED.actions.button(id, ctx)`** returns the attributes for icon buttons (card tools, header buttons): `aria-label`, a `title` of the label plus the shortcut, and when unavailable `aria-disabled="true"` with the reason in the title and in `aria-description`. Buttons use `aria-disabled`, not `disabled`, so keyboard users can still focus them and learn why.
- **`ED.actions.key(event, ctx)`** is the one keyboard path for registry shortcuts. It matches the canonical shortcut in its scope and calls `run`. Each shortcut is dispatched by exactly one path: the plan decides, per shortcut, whether the desktop window-menu dispatch (`shortcut` field, `menus-and-routing.js:1531`) or `ED.actions.key` fires it, and the other path only displays it (`shortcutHint`).
- **Text fields and dialogs keep today's behaviour.** Today `easydrag-editor.js` `onKeyDown` (721-752) has per-key rules: Mod+S is always kept from the browser, Ctrl+Enter, Ctrl+K and Ctrl+F are held back under an open dialog, text fields keep native undo and IME, and keys without a modifier never act from a text field. The plan records these rules per shortcut and `ED.actions.key` keeps them; a shortcut that changes behaviour is a design decision for the owner.
- **Display.** Shortcuts are displayed through `core.shortcut()` (⌘ and ⇧ on macOS) and localized key names (de "Entf" for Delete).
- **The shortcut table** ("Tasten") is generated from the registry, grouped as today (canvas, edit, flow). Gestures that are not actions get their own rows from a short list in `easydrag-dialogs.js`: arrows (navigate), Shift+arrows (nudge), Space (pan), Esc.

### 3.5 Risk

| Risk | Actions | Effect |
|---|---|---|
| `safe` | everything that changes nothing for good | none |
| `undoable` | `edit.delete`, `edit.cut` | red text (`danger`); the delete toast with "Rückgängig" stays |
| `confirm` | `flow.delete` (and the block editor's delete) | red text; the existing confirmation dialogs stay |

### 3.6 Surfaces that move to the registry

- The window menus, the "⋯", the card menu, the canvas menu, the home card menu and the Lebendigkeit submenu list action ids and render through `ED.actions.items`.
- The card tools render through `ED.actions.button`. A card tool selects its card first and then runs the same `edit.*` or `step.*` action as the card menu.
- The header buttons (Test, Stop, Publish, Run now, the active switch, "⋯") take label, title and reason from `ED.actions.button`, but keep their markup, because W1c rebuilds the header.
- The keyboard handlers route registry shortcuts through `ED.actions.key`. Navigation keys stay in `handleKey`.
- One label per action: the duplicate keys (`menu_copy`, `menu_paste`, `node_delete`, `node_duplicate`, `menu_select_all`, …) leave all 16 locales when no code uses them. "Einfügen" replaces "Hier einfügen"; the canvas menu pastes at the pointer, the Edit menu at the view centre, as today.
- **Line rule.** `easydrag-editor.js` (896 lines), `-interact.js` (753), `-home.js` (889) and `-dialogs.js` (309) may only get shorter. `easydrag-canvas.js` stays at 928 lines or fewer.

### 3.7 Tests (Part A)

- **T0, node** (`scripts/test-easydrag.mjs` or a new script in T0):
  - every row has a label key in all 16 locales, an icon in the icon map, a valid risk and scope, and a shortcut that no other row in the same scope uses;
  - every check cause has a reason key in all 16 locales;
  - `state` refuses and `run` does nothing when a check fails; `items` marks `confirm` and `undoable` rows `danger`.
- **T0, Go lint** (`ui/desktop_easydrag_actions_test.go`): no EasyDrag module outside `easydrag-actions.js` builds a menu entry with its own `label:` or `labelKey:`, or gives a menu entry a `shortcut` or `shortcutHint` (the panel width menu is the one exception).
- **Browser** (`TestEasyDragActionsAgreeBrowser`): in four states (normal, run view, run in progress, nothing selected) the test opens every surface and checks that each action shows the same label, displayed shortcut, availability and reason on every surface that lists it; that every unavailable entry carries a reason; and that pressing each registry shortcut once runs its action once.

## 4. Part B: the voice

### 4.1 The guide and the rules

New developer guide `documentation/easydrag/voice.md`, with en and de examples. It holds the rules and the concept ledger (§4.4).

1. **Outcome first.** A text says first what it means for the user, then what to do. Mechanism and log hints come last.
2. **A next step.** Every error ends with what to do, or says that nothing needs doing.
3. **One form of address.** German uses lowercase "du", "dich", "dir", "dein…"; a capital only at the start of a sentence (roadmap decision 1).
4. **One word per concept** (the ledger).
5. **Limits.** Always-visible text names no limit. Counters appear from 90 %. A rule hint becomes the error message when an input breaks the rule. Help that the user opens may name limits.
6. **The user's own actions.** No visual notification when the screen already shows the result. Where the result is invisible, a short confirmation appears at the control. Screen-reader announcements stay.
7. **Only what the code guarantees.** A text says "nothing has changed" or "nothing was sent" only when the error code means exactly that.

### 4.2 "du"

- The 29 keys in `ui/lang/easydrag/de.json` with a capital "Du", "Dich", "Dir" or "Dein…" inside a sentence are converted (examples: `error_flow_mission_missing`, `simulation_hint`, `authoring_title`, `transform_preview_hint`, `metadata_reload_confirm`, `group_readonly`, `extraction_stale`).
- **Lint** (`ui/desktop_easydrag_voice_test.go`): it fails on a capital form that is not at the start of the value or after ". ", "! ", "? ", ": " or a line break. Placeholders (`{name}`) are removed first.

### 4.3 The error texts

- **Keys:** the 121 `easydrag.ui.error_*` keys, the 8 `durable_reason_*`, the 10 `approval_blocked_*` and `load_failed`, about 140 in all.
- **Form:** what happened to the user, then what to do. Example: "Die Flow-Ausführung ist unerwartet fehlgeschlagen. Details stehen im AuraGo-Log." becomes "Der Lauf ist abgebrochen. Starte ihn noch einmal; Details stehen im AuraGo-Log."
- **Accuracy** (rule 7): the plan reads the code behind each error code before it rewrites the text.
- **Languages:** en and de first. The other 14 locales are translated from the new en through key files and `locale-apply.mjs`. Placeholders stay as they are; the catalog test checks them.
- Texts that only W1c, W1d or W1f surfaces show (header, test loop, home) are left to them.

### 4.4 The concept ledger and the rename

The ledger is a table in the guide: concept, de word, en word, meaning, where it appears.

| Concept | de | en | Meaning |
|---|---|---|---|
| Flow | Flow | flow | one automation |
| Step | Schritt | step | one card on the canvas, and the palette of step types |
| Building block | Baustein | block | a reusable published sub-flow |
| Trigger | Auslöser | trigger | the step a flow starts from |
| Draft | Entwurf | draft | the editable state |
| Published state | vN | vN | one published state (W1a CS-01) |
| Run | Lauf | run | one execution |
| Simulation | Simulation | simulation | a test run with sample outputs and no effects |
| Template | Vorlage | template | a flow to start from |
| Pack | Pack | pack | a local export or import bundle |

**Open in the ledger:** "Vorlage" next to "Rezept" (the Wave 3 recipe cards decide); "Simulation" next to a real test run (W1d); "Testbank" is not yet a UI word.

**The rename (decision 5):**
- de: the palette and step-type texts change "Baustein" to "Schritt" (`palette_title` "Schritte", `palette_search` "Schritte suchen", `palette_no_results` "Kein Schritt gefunden.", `badge_unknown`, `error_node_type_unknown`, `issue_node_type_unknown`, `panel_resize_palette`, …). The reusable sense keeps "Baustein" (`blocks_title`, `block_*`). `pack_blocks` "Lokale Blöcke" becomes "Lokale Bausteine".
- en: "building block" (11 texts) becomes "step"; "block" keeps the reusable sense.
- Other locales: the renamed keys are translated again from the new en.
- The plan lists every renamed key by name, so the reusable sense is not caught by mistake.

**Lint** (`ui/desktop_easydrag_voice_test.go`), on de and en only, after removing placeholders:
- de bans "Workflow", "Knoten", "Node", "Block", "Blöcke", "Blocks", "Revision" and "Paket";
- en bans "workflow", "node", "building block", "revision" and "package".

### 4.5 Notifications about the user's own actions

| Notification (key) | Call site | Change |
|---|---|---|
| `publish_done` | `easydrag-publish.js:236` | removed: the header shows the new vN |
| `active_on`, `active_off` | `publish.js:254` | removed: the switch shows it |
| `versions_restored` | `publish.js:320`, `easydrag-blocks.js:183` | removed: the draft and the header show it |
| `run_started` | `easydrag-home.js:659` | removed: the card's last-run line shows "Läuft" with the `running` disc until the next list load. The home has no status disc today, so W1b adds this one line |
| `repair_mission_done` | `easydrag-editor.js:477` | removed: the issue disappears |
| `extraction_success` | `easydrag-extraction.js:81` | removed: the canvas shows the new building block |
| `authoring_applied` | `easydrag-authoring.js:93` | removed: the draft shows the change |
| `secret_deleted` | `easydrag-fields.js:578` | removed: the list shows it |
| `copied_json` | `easydrag-detail.js:780` | becomes "Kopiert" on the button for 2 seconds (no motion), then the button label returns |

- Every removed notification is replaced by a polite screen-reader announcement with the same text, unless one already exists.
- Notifications about failures, partial results and skips stay (`publish_partial`, `run_now_skipped`, every `type: 'error'`).
- The existing aria-live echoes (add, delete, undo, copy, connect, saved, selected) stay.

### 4.6 Limits

- **`ED.kit.limit(input, { max, counter })`**, a new kit helper:
  - `counter: true` (multi-line text): "1.812 / 2.000" appears from 90 % of `max`, in `ED.format` numbers;
  - `counter: false` (single line): "Höchstens 120 Zeichen." appears when the input reaches `max`;
  - both sit under the field in a polite live region, and both disappear when the text is shorter again.
- **Where:** every capped text input in EasyDrag. Known today: the description goal (authoring, 2000, counter; it keeps its warning colour and the disabled Generate button above the limit), the flow name (120) and description (2000, counter), the building-block name (120), the key name (40). The step label is capped at 80 code points, with an input `maxlength` of 160 UTF-16 units for surrogate pairs (`ui/AGENTS.md`); its message counts code points. The plan finds the rest by `maxlength`.
- **Rule hints:** `block_fields_hint` ("up to 40 lowercase letters…") and `flow_max_run_hint` ("from 1 second to 24 hours") leave the always-visible text and become the error shown when an input breaks the rule. Collapsed help (`group_limit`, `transform_limit`, `block_sample_limit`, `batch_limit_hint`) stays.
- Not limits: the preview character count (`preview_text`) describes a value and stays.

### 4.7 Tests (Part B)

- **T0, Go** (`ui/desktop_easydrag_voice_test.go`): the "du" lint and the ledger lint.
- **T0, node:** `kit.limit` hides the counter at 50 %, shows it at 90 %, shows the cap message at 100 % and hides it again below.
- **Browser** (in an existing journey test): publishing and switching the flow on send no desktop notification and do send an announcement; "JSON kopieren" shows "Kopiert" on the button.
- The existing catalog tests (`internal/flows/catalog_i18n_test.go`, `ui/desktop_easydrag_test.go` `TestEasyDragUIKeysExistInAllLocales`) keep the 16 locales complete.

## 5. Part C: the control budgets

### 5.1 The counting rule

One helper, built on the target scanner of `ui/desktop_easydrag_targets_browser_test.go` (`checkTargets`, line 68):
- **Counted:** every visible control: `button`, `a[href]`, `input` (not hidden), `select`, `textarea`, and elements with `role` button, switch, checkbox, tab, link, option or menuitem, or `tabindex="0"`. Unavailable controls count, because the user sees them.
- **Counted as one:** a radio group (same `name`), a collapsed disclosure (its head), a menu button (its entries are not counted).
- **Not counted:** hidden, inert or zero-size elements, and the flow card itself.
- **State:** the standard fixture, a published idle flow, 1440 px wide.

### 5.2 Regions, ceilings and targets

| Region | Today | Target | Enforced by |
|---|---|---|---|
| Editor header (the title bar) | 10 | ≤ 8 | W1c |
| Editor footer | 2 | no growth | — |
| "⋯" menu (entries, separators not counted) | 12 | no growth, so W1c cannot meet its target by moving controls into "⋯" | — |
| Home: every control outside the flow cards | about 13 (radio groups count as one) | ≤ 6 | W1f |
| One published flow card | 4 | ≤ 3 | W1f |

"Today" is the research count of 2026-11-14. Part C's first task measures the real numbers with the helper and checks them in as the ceilings.

### 5.3 Mechanics

- The test (`ui/desktop_easydrag_budget_browser_test.go`) holds a table: region, ceiling, target, enforcing sub-project, enforced (yes or no).
- It fails when a count is above its ceiling. When a count is below, it logs "lower the ceiling to N".
- With "enforced: yes", the ceiling equals the target. W1c switches the header row, W1f the home and card rows. At the Wave 1 exit every row with a target is enforced.
- Every run logs `BUDGET region= count= ceiling= target= gate=`.

### 5.4 Tests (Part C)

The budget test itself, plus a T0 check that the table is well formed (every target ≤ its ceiling, every target row names its sub-project).

## 6. Testing

- **Per commit:** as in W1a: `go vet` and `gofmt -l` for Go; `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs` for JS and CSS.
- **Per batch:** T0 (`npm run test:easydrag:t0`).
- **On landing:** the full browser run, `go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v`, including the W1a lab (16 contexts), which must stay at zero findings.
- **Review:** one combined review per part, Critical and Important only.

## 7. Order and parallel work

- **C first.** It is small and only adds a test.
- **A and B in parallel**, each in its own worktree.
- **Landing:** by rebase, then fast-forward. A and B both write locale keys. The second to land re-applies its key files after the rebase (the key files are idempotent) instead of merging JSON by hand.
- Files: A owns `easydrag-actions.js` and the menu and keyboard code; B owns the locales' rewritten keys, the notification call sites, `kit.limit` and the guide. Both touch `easydrag-editor.js`, `-home.js` and `-dialogs.js`, in different functions.

## 8. Acceptance checklist

- [ ] Every action that a menu, card tool, header button or shortcut offers is a registry row, and the lint finds no menu entry with its own label.
- [ ] In the four states, each action shows the same label, shortcut, availability and reason on every surface; every unavailable entry says why; each shortcut fires once.
- [ ] Delete and cut are red; deleting a flow is red and asks first.
- [ ] Copy works in read-only mode on every surface.
- [ ] No capital "Du/Dich/Dir/Dein…" inside a German sentence; the lint passes.
- [ ] The about 140 error texts follow rules 1, 2 and 7 in en and de and are translated in the other 14 locales.
- [ ] The palette says "Schritte"; "Baustein" means only the reusable block; the ledger lint passes.
- [ ] The nine own-action notifications of §4.5 are gone or moved to the control; screen readers still hear each outcome.
- [ ] Counters appear from 90 %; capped inputs say so when reached; no always-visible text names a limit.
- [ ] The budget test runs with today's ceilings and the targets of §5.2.
- [ ] The full browser run and T0 are green; the lab has zero findings.

## 9. Risks

- **Shortcuts that fire twice or not at all** after the move to one path. The browser test presses every registry shortcut once.
- **`aria-disabled` buttons still react.** `ED.actions.run` checks the state before every run.
- **The rename catches the reusable sense of "Baustein".** The plan lists every renamed key.
- **Translation quality** of about 170 changed or new keys in 14 locales (the error texts, the renamed keys, the reason texts). Key files are reviewed; a native-speaker check is a human follow-up, as for W1a.
- **Line headroom.** `easydrag-editor.js` is at 896 of 900 lines; the registry only shortens it.
- **Other sessions in the main checkout.** Implementation runs in worktrees only.

## 10. Left for later sub-projects

- W1c: renders the header from the registry, adds the one Stop, enforces the header target, rewrites header copy.
- W1d: rewrites test-loop copy and settles "Simulation" versus a real test in the ledger.
- W1f: enforces the home and card targets, adds the hero actions ("Neu ▾", "Importieren…") as registry rows, rewrites home copy.
- E4: the canvas selection bar reads the registry.
- Wave 4: Ctrl+K command mode (">" commands) reads the registry.
