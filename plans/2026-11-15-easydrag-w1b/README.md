# EasyDrag W1b "Voice and actions" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every EasyDrag action one name, one shortcut and one reason for being unavailable; give EasyDrag one voice; and put a number on how many controls the header and the home may show.

**Architecture:**
- **Part C.** `TestEasyDragControlBudgetsBrowser` counts the controls of the editor header, the footer, the "⋯" menu, the home and one published card with one counting rule, and holds a table of ceilings (today's counts) and targets (W1c and W1f). `TestEasyDragControlBudgetTable` keeps the table consistent in T0.
- **Part A.** `easydrag-actions.js` holds one row per action (label, icon, shortcut, scope, risk, shared checks with fixed reasons). The owning modules bind the execution with `ED.actions.bind`. The window menus, the "⋯", the card and canvas menus, the card tools, the home card menu, the Lebendigkeit submenu, the header buttons, the shortcut table and the keyboard render or run through it.
- **Part B.** A developer guide (`documentation/easydrag/voice.md`) with seven rules and the concept ledger; two lints (lowercase "du", ledger words); about 140 error texts rewritten outcome-first in 16 locales; the palette renamed to "Schritte"; the own-action notifications removed or moved to the control; `ED.kit.limit` for every capped input.

**Tech Stack:** vanilla JS classic scripts on `window.EasyDrag`; CSS custom properties; Node test scripts (`scripts/test-easydrag*.mjs`); Go tests in `ui` (source lints and go-rod browser tests); 16 locale files in `ui/lang/easydrag/`, changed only through the key files in `keys/`.

**Spec:** [`plans/2026-11-15-easydrag-w1b-spec.md`](../2026-11-15-easydrag-w1b-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) §6 Wave 1, E2.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-c-budgets.md](part-c-budgets.md) | §5 | C.1 counting helper and budget browser test · C.2 table check (T0) |
| [part-a-actions.md](part-a-actions.md) | §3 | A.0 preamble · A.1 `easydrag-actions.js`, Node test, registration, reason keys · A.2 card tools, card and canvas menus; edit, step and zoom actions bound in `interact` · A.3 window menus, "⋯", Lebendigkeit; flow and view actions bound in the editor · A.4 home card menu and start-page window menu · A.5 `ED.actions.key` and the generated shortcut table · A.6 header buttons, Undo and Redo · A.7 duplicate keys retired, lint `TestEasyDragMenusUseTheActionRegistry` · A.8 `TestEasyDragActionsAgreeBrowser` |
| [part-b-voice.md](part-b-voice.md) | §4 | B.1 guide, "du" lint, 29 keys · B.2 "Schritte" rename and ledger lint · B.3–B.5 error texts (below) · B.6 own-action notifications · B.7 `ED.kit.limit`, capped inputs, rule hints · B.8 manuals (amendment 25) |
| [part-b-errors-1.md](part-b-errors-1.md) | §4.3 | B.3 error texts, list 1 (47 keys) |
| [part-b-errors-2.md](part-b-errors-2.md) | §4.3 | B.4 error texts, list 2 (47 keys) |
| [part-b-errors-3.md](part-b-errors-3.md) | §4.3 | B.5 error texts, list 3 (46 keys) |
| [keys/](keys/) | | the locale key files, applied with `node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/<file>.json` from the worktree |

## Order, worktrees and reviews

| Part | Worktree | Branch | Starts | Lands |
|---|---|---|---|---|
| C | `.worktrees/easydrag-w1b-budgets` | `feat/easydrag-w1b-budgets` | now | first |
| A | `.worktrees/easydrag-w1b-actions` | `feat/easydrag-w1b-actions` | now, in parallel | after C |
| B | `.worktrees/easydrag-w1b-voice` | `feat/easydrag-w1b-voice` | now, in parallel | after C, before or after A |

- Each part gets one combined review by `superpowers:code-reviewer` before it lands. The review reports Critical and Important findings only, and there are no re-review rounds.
- Before landing, each part rebases on main and re-runs its checks.
- The parts share some files. The rules for them are in "Shared files" below.

## Conventions that override the parts

The parts were written in parallel from one brief (`C:/disposable/easydrag-w1b-plan/BRIEF.md`). Where they differ, this section decides.

- **Shell and IMPACT.**
  - Git Bash. Every command starts with `cd <worktree> && …`.
  - Run IMPACT before changing a function: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`. Warn on HIGH or CRITICAL, and confirm UNKNOWN with grep.
- **DETECT.** Every commit body ends with `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, placed before the Co-Authored-By line.
- **Commits.**
  - One per task: exact paths, then `git commit -F <message file>`. Message files go into `C:/disposable/easydrag-w1b/`.
  - The last line is the Co-Authored-By line for the model the implementer runs as.
  - No amend or push. The only rebase is the pre-landing one.
- **Go.**
  - Use `GOTMPDIR='D:\tmp\go-build'` for `ui`, `internal/flows`, `internal/tools` and `cmd/...`, and none for `internal/server`.
  - Prefix every Go command with `GOTOOLCHAIN=go1.27.4`.
- **Browser tests.**
  - Set `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots`, and run with the Bash sandbox disabled.
  - Open pages only through `startEasyDragSmokeWith` / `startEasyDragSmoke`.
  - Re-run a failing test alone before counting it. The full run takes about 17 minutes since the W1a lab.
- **Escapes.** Never put a backslash-u sequence in plan code. In Go, use `"\x5c" + "u…"` or runes. In JS, use `String.fromCodePoint`.
- **Gates before each commit.**
  - Go: `go vet` and `gofmt -l`.
  - JS and CSS: `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs`.
  - T0 (`npm run test:easydrag:t0`) at the end of each part.
- **Line rule.**
  - `easydrag-editor.js` (896), `-interact.js` (753), `-home.js` (889) and `-dialogs.js` (309) may only get shorter.
  - `easydrag-canvas.js` stays at 928 lines or fewer.
  - `easydrag-model.js`, `-template.js` and `-detail.js` change existing lines only.
- **Tests.** Essential tests only. No external assets; `TestShippedWebAssetsLoadNothingExternal` stays green.

## Shared files

- **Locale files** (`ui/lang/easydrag/*.json`).
  - Change them only through the key files in `plans/2026-11-15-easydrag-w1b/keys/` and the helper `C:/disposable/easydrag-w1a/locale-apply.mjs`.
  - On a rebase conflict, take main's locale files and re-run the part's key files in task order. The helper is idempotent.
  - **Order matters inside Part B.** Eleven keys sit in two key files: eight error keys are in `b-du.json` and in an error file, and three (`error_flow_node_unavailable`, `error_node_type_unknown`, `error_flow_pack_unsupported`) are in `b-rename.json` and in an error file. Run the files in task order (`b-du`, `b-rename`, `b-errors-1`, `b-errors-2`, `b-errors-3`, `b-notify`, `b-limit`); the error file's text is final, and it keeps both the lowercase "du" and the ledger words.
- **Module registration.** Only Part A adds a module (`easydrag-actions.js`): `ui/js/desktop/core/module-loader.js`, the Go list `easyDragModules` and the Node harness lists.
- **`easydrag-editor.js`, `-home.js`, `-dialogs.js`** are touched by A (menus, keyboard, shortcut table) and B (notification call sites, limits, hint texts), in different functions. On a conflict keep both edits.
- **`easydrag-kit.js`** belongs to B (`limit`, `flash`).
- **New Go tests go into new files**, one per concern, so `ui/desktop_easydrag_test.go` changes only in its module list.
- **CSS.** B adds the `.ed-limit` rules; A adds rules only if its part says so. The file has CRLF line endings; Part B's edit script keeps them.

## Amendments found while planning

Each amendment is applied in the named part. The coordinator copies them into the spec when the part lands.

**Part C, control budgets**
1. **The scanner.** The counting helper copies the visibility rules of `checkTargets` (not its code) and adds `[hidden]` and `[inert]` ancestors, the §5.1 roles and the group rule, because `checkTargets` returns no count.
2. **No target** is `Target: 0` with `Gate: ""`.
3. **The "⋯" menu is counted in the wide window** (1440 px), without the phone-only entries. There is no phone row (coordinator decision).
4. **The block library link counts** toward the home budget (coordinator decision): it is page chrome, and W1f must place it deliberately.
5. **A measured count that is already at or below its target** (not expected): the row is enforced at once, with ceiling and target equal to the target (coordinator decision). The region already meets its goal and must not regress.

**Part A, action registry**
6. **Icons.** Rows carry `icon` (the desktop menu icon the menus draw today) and `glyph` (the `core.ICONS` name for in-app icon buttons), not an alias from the W1a icon map: that map has 16 one-to-one meanings and cannot name the card tools' glyphs. `ED.icons` is untouched.
7. **A `menu` field** keeps each action's window-menu item id (`run`, `delete-sel`, `select-all` …), which the desktop uses as the action key and about twenty tests use to find items.
8. **Four more rows:** `flow.open_draft` (the block editor's "Open current draft") and `flow.new`, `flow.import`, `flow.reload` (the start page's window menu in `easydrag.js`). The home hero buttons and the bulk bar stay outside, as the spec says.
9. **Two more checks:** `unlocked` (read-only and locked, but not the run view: duplicating, deleting and exporting a flow work in a past run today) and `quiet` (quick-add is open: edits wait for it, Save and Test do not).
10. **More helpers** than the contract (`paint`, `attrString`, `label`, `display`, `table`, `observe`, `TOOLS`, `REASONS`, `CHECKS`), and `bind(id, impl, host)` takes the host as a third argument, so several editor windows never share bindings. `impl` may carry `show`, `checked` and `accepts` besides `run`, `check` and `label`.
11. **Two reason texts are more accurate than the spec's** (coordinator decision, rule 7): `reason_locked` "Durch die Agenten-Rechte gesperrt oder bis die Liste neu geladen ist." (`createOff()` is also true for a stale list), and `reason_restoring` "Beantworte zuerst das Wiederherstellungs-Angebot." (the editor's `restoring` flag is the emergency-copy offer, not a version restore).
12. **Task order:** `interact` (A.2) before the editor's menus (A.3), because the edit actions' closures live in `interact`. **The line rule is checked at the end of each part,** not after every task: `easydrag-interact.js` is 765 lines after A.2 and 750 after A.5.
13. **Shortcut scopes follow today's code** (coordinator decision): the Edit shortcuts (Ctrl+Z, Ctrl+C, Delete …) stay `canvas` scope, acting only while the canvas, a card or the page has the focus. Only Ctrl+S, Ctrl+Enter, Ctrl+K, Ctrl+F and Alt+Shift+F are editor-wide. **Dispatch:** the desktop shell fires Ctrl+S, Ctrl+Enter and Ctrl+K (`DESKTOP_KEYS`, menus carry `shortcut`); `ED.actions.key` fires every other registry shortcut, and the menus only draw it (`shortcutHint`).
14. **"⋯" has no row;** it opens a menu and is not an action.
15. **Red only in context menus** (coordinator decision). The desktop's window menus (`normalizeWindowMenuItems`) do not support `danger`; red in the menu bar is a follow-up desktop change.
16. **Where surfaces disagreed, the stricter condition wins** (spec rule), except Copy:
    - Ctrl+X in read-only mode no longer copies (the Edit menu already refused Cut).
    - The Flow menu's Test is unavailable during a run (the "⋯" and the header already said so).
    - Find step is unavailable in a flow without steps on every surface.
    - Flow-menu items are unavailable with a reason while a dialog or the restore offer is open (they did nothing silently).
    - Paste is unavailable only when nothing can be pasted (no stored fragment and no clipboard API); in a browser it stays available (coordinator decision: keep the check).
    - Run now gives a reason in read-only mode, in a past run and for a never-published flow; paused uses `reason_paused` instead of `error_flow_disabled`.
    - "Versions" in the block editor's Flow menu opens the block's versions, as its "⋯" did.
    - A key the registry takes and the action refuses is still prevented.
17. **One label per action:** Test reads "Entwurf testen" / "Test draft" on every surface (the Flow menu said "Testen"; the precise label wins, because it tests the draft), a block's delete reads "Baustein löschen", Paste reads "Einfügen", the shortcut table uses the menu labels, Delete is drawn "Entf" / "Del". The key `easydrag.ui.test` is retired with ten other duplicates (`keys/a-7.json`).
18. **Test expectations change** where a test read a surface the registry now writes differently; each task names the tests and the cause. No test is weakened.

**Both parts**
19. **CSS.** Part A adds `[aria-disabled="true"]` rules and Part B the `.ed-limit` rules. If both add them at the same place, a rebase conflict keeps both.

**Part B, voice**
20. **`secret_deleted` stays** (coordinator decision). It appears only when flows still use the deleted secret and warns of a consequence the list does not show ("Flows that use it fail until the secret is set again"), so it is a partial result in the sense of rule 6. Eight notifications change, not nine.
21. **The goal limit is 4,000 code points** (`GOAL_MAX`, `easydrag-authoring.js`, and the server), not the spec's 2,000; 2,000 is the flow description.
22. **`ED.kit.limit` takes two more options**, `t` (the caller's translator) and `after` (the element the note follows). **`ED.kit.flash(button, text, ms)`** shows the 2-second "Kopiert" without motion.
23. **The limit note does not speak on every keystroke** (coordinator decision, replacing Part B's decision 3). The visible note updates on every input; the input points to it with `aria-describedby`, so a screen reader reads the current count on focus. The polite live region speaks only when the state changes: when the counter appears (90 %), when the cap is reached, and when the note disappears. B.7 implements this in `ED.kit.limit` and its Node test checks that typing inside the 90–99 % band does not change the live region's text.
24. **The lints are stricter than the spec:** the "du" lint knows the endings (Deine, Deinem, Deinen, Deiner, Deines); the ledger lint also bans plurals and the German compounds (Blockversion, Blockkopie, Blockinhalt, Blockeingabe, Blockausgabe), and keeps the 12 step texts free of "Baustein" and "block".
25. **The manuals follow the ledger and the "du" rule** (coordinator decision, replacing Part B's decision 1). Task B.8, after B.7, applies them to `documentation/manuals/de/24-easydrag.md` and `documentation/manuals/en/24-easydrag.md` (see below).
26. **The rename touches 20 keys,** including `canvas_role` and `authoring_notice` ("node"), the German pack texts ("Blöcke", "Blockversionen", …), `panel_resize_palette` (de) and `rail_palette` (it, pt, ja; kept, so the rail and the palette use the same word).
27. **`flow_max_run_hint` loses its second sentence;** the rule moves into the existing error `flow_max_run_invalid`. `goalCounter()` and the key `goal_counter` stay for their code-point test.
28. **One more browser test,** `TestDesktopEasyDragHomeRunNowBrowser`, for the new "Läuft" line on the home card.
29. **Edits by script.** B.2, B.6 and B.7 apply their edits with `C:/disposable/easydrag-w1b/apply-plan-edits.mjs` from the edit groups in `part-b-voice.md` (exact old and new text, one match each, line endings kept).

### Task B.8: the manuals follow the ledger and the "du" rule

**Files:**
- Modify: `documentation/manuals/de/24-easydrag.md`, `documentation/manuals/en/24-easydrag.md`

Do this task after B.7, in the worktree `.worktrees/easydrag-w1b-voice`.

- [ ] **Step 1: List the hits.**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && grep -n -E 'Baustein|Bausteine|Bausteinen' documentation/manuals/de/24-easydrag.md; grep -n -E '(^|[^.!?:] )(Du|Dich|Dir|Dein[a-z]*)\b' documentation/manuals/de/24-easydrag.md; grep -n -i -E 'building blocks?|\bnodes?\b|\bworkflows?\b' documentation/manuals/en/24-easydrag.md
```

Expected: a list of lines. The generated reference tables (between the `flows-triggers` and `flows-steps` markers of S0c) come from the catalog; do not edit inside them.

- [ ] **Step 2: Edit by hand, outside the generated blocks.**
  - de: "Baustein(e)" in the sense of a step or step type becomes "Schritt(e)"; the reusable sub-flow keeps "Baustein". A capital "Du", "Dich", "Dir" or "Dein…" inside a sentence becomes lowercase.
  - en: "building block(s)" in the step sense becomes "step(s)"; "node(s)" becomes "step(s)"; "block" keeps the reusable sense.

- [ ] **Step 3: Check.** Re-run the greps of Step 1: only reusable-block "Baustein" lines, sentence-initial capitals and generated-block lines remain. Then:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && node scripts/test-easydrag-ux-batch5.mjs 2>&1 | grep -E 'U18|FAIL' ; GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./cmd/flowsdoc -run 'TestFlowsDocBlocksMatchTheCatalog' -count=1
```

Expected: the `b5 U18 manual chapter 24` check passes and no FAIL line appears (`scripts/test-easydrag-ux-batch5.mjs:966` requires words such as "Schritt hier hinzufügen" and "Add step here", which stay), and `ok  ... cmd/flowsdoc` (the generated tables still match the catalog).

- [ ] **Step 4: Commit.**

```
docs(easydrag): manuals say "Schritt" for steps and lowercase "du" (W1b B.8)

The German manual called steps "Bausteine", the English one "building
blocks", and the German text used a capital "Deine" inside sentences. Both
manuals now follow the W1b ledger: a step is "Schritt"/"step", and
"Baustein"/"block" is only the reusable sub-flow. The generated reference
tables are unchanged.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

## Not covered by W1b (follow-ups)

- **Native-speaker check** of the 14 other locales: the error texts (B.3–B.5), the rename (B.2), the reason texts (A) and the limit and notification keys. In the Wave 1 sessions, as for W1a.
- **`goalCounter()` and `goal_counter`** can go once another test covers the code-point count.
- **W1c to W1f** (spec §10).

---

## Finishing W1b

1. **Rebase and re-check.** Rebase the part's branch onto the current main in its worktree. If locale files conflict, re-apply the key files in task order.
2. **Run the part's checks:**
   - T0;
   - one full gated EasyDrag browser run, including the W1a lab (zero findings);
   - for C, the measured ceilings in the commit body.
3. **Check the main checkout.**
   - It must be on `main`: `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current`.
   - Compare its uncommitted files with the branch's files.
4. **Land** with `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only <branch>`. Do not push.
5. **Record.** After each part, the coordinator records on main, through a temporary index:
   - the amendments above into the spec;
   - the landed range into the roadmap's Wave 1 status;
   - the EasyDrag project memory.
6. **Clean up.** Remove the worktree and delete the merged branch.
