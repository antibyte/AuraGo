# EasyDrag W1a "Foundations" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundations of EasyDrag Wave 1. Every status renders through one map as a disc with a glyph and a word. One token set covers type, radius, space and motion, and a small kit replaces ad-hoc components and native disclosures. Counts and dates follow the language. The rail is labelled and every icon has one meaning. Nothing moves while idle. A lab checks all of it in 16 theme contexts.

**Architecture:**
- **Part A.** `easydrag-status.js` holds 12 values (10 from the roadmap plus run-level `stopped` and `cancelled`). About 15 surfaces move onto it, and a lint forbids status classes and words outside it.
- **Part B.** `easydrag-kit.js` provides disclosure, callout, chip, toast and toolbar. `modal()` becomes a bottom sheet under 600 px. Part B also migrates all `<details>` and all font sizes onto six tokens.
- **Part C.** `easydrag-format.js` provides plurals through `Intl.PluralRules`, `Intl` formatters, "vN", and a pseudo-locale behind a developer switch.
- **Part D.** `easydrag-icons.js` maps meanings to glyphs, and the rail gets captions under its icons. `easydrag-delight.js` is one motion gate with the "Lebendigkeit" setting.
- **Part E.** `TestEasyDragLabBrowser` runs six scenes in 16 theme contexts and checks contrast, distance, targets, clipping and accessible names.

**Tech Stack:** vanilla JS classic scripts on `window.EasyDrag`; CSS custom properties; Node test scripts (`scripts/test-easydrag*.mjs`); Go tests in `ui` (source lints and go-rod browser tests); 16 locale files in `ui/lang/easydrag/`.

**Spec:** [`plans/2026-11-13-easydrag-w1a-spec.md`](../2026-11-13-easydrag-w1a-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) §6 Wave 1, E2.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-a-status.md](part-a-status.md) | §3 | A.1 tokens and status colour test · A.2 `easydrag-status.js` · A.3–A.5 surface migration · A.6 old keys deleted · A.7 status lint and full run |
| [part-b-kit.md](part-b-kit.md) | §4 | B.1 `easydrag-kit.js` · B.2 sheet · B.3 undo toast · B.4 `<details>` migration · B.5 type scale · B.6 ratchets |
| [part-c-language.md](part-c-language.md) | §5 | C.1 `easydrag-format.js` · C.2 formatter call sites · C.3a/b plural forms and count sites · C.4 "vN" · C.5 pseudo-locale |
| [part-d-rail.md](part-d-rail.md) | §6 | D.1 `easydrag-icons.js` · D.2 rail captions · D.3 `easydrag-delight.js` and "Lebendigkeit" · D.4 motion gate and idle test |
| [part-e-lab.md](part-e-lab.md) | §7 | E.1 lab scaffolding · E.2 scenes 1–2 · E.3 scenes 3–5 · E.4 pseudo-locale run · E.5 fix what the lab finds |

## Order, worktrees and reviews

| Part | Worktree | Branch | Starts | Lands |
|---|---|---|---|---|
| A | `.worktrees/easydrag-w1a-status` | `feat/easydrag-w1a-status` | now | first |
| C | `.worktrees/easydrag-w1a-language` | `feat/easydrag-w1a-language` | now, in parallel with A | after A or before it |
| D | `.worktrees/easydrag-w1a-rail` | `feat/easydrag-w1a-rail` | now, in parallel with A | after A or before it |
| B | `.worktrees/easydrag-w1a-kit` | `feat/easydrag-w1a-kit` | after A has landed | after A |
| E | `.worktrees/easydrag-w1a-lab` | `feat/easydrag-w1a-lab` | after A, B, C and D have landed | last |

- Each part gets one combined review by `superpowers:code-reviewer` before it lands. The review reports Critical and Important findings only, and there are no re-review rounds.
- Before landing, each part rebases on main and re-runs its checks.
- The parts share some files. The rules for them are in "Shared files" below.

## Conventions that override the parts

The five parts were written in parallel from one brief (`C:/disposable/easydrag-w1a-plan/BRIEF.md`). Where they differ, this section decides.

- **Shell and IMPACT.**
  - Git Bash. Every command starts with `cd <worktree> && …`.
  - Run IMPACT before changing a function: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`. Warn on HIGH or CRITICAL, and confirm UNKNOWN with grep.
- **DETECT.** Every commit body ends with `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, placed before the Co-Authored-By line.
- **Commits.**
  - One per task: exact paths, then `git commit -F <message file>`. Message files go into `C:/disposable/easydrag-w1a/`.
  - The last line is the Co-Authored-By line for the model the implementer runs as.
  - No amend or push. The only rebase is the pre-landing one.
- **Go.**
  - Use `GOTMPDIR='D:\tmp\go-build'` for `ui`, `internal/flows`, `internal/tools` and `cmd/...`, and none for `internal/server`.
  - Prefix every Go command with `GOTOOLCHAIN=go1.27.4`, because the go1.27.3 toolchain on this machine loses `vet.exe`.
- **Browser tests.**
  - Set `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots`, and run with the Bash sandbox disabled.
  - Open pages only through `startEasyDragSmokeWith` / `startEasyDragSmoke`.
  - Re-run a failing test alone before counting it.
- **Escapes.** Never put a backslash-u sequence in plan code. In Go, use `"\x5c" + "u…"` or runes. In JS, use `String.fromCodePoint`.
- **Gates before each commit.**
  - Go: `go vet` and `gofmt -l`.
  - JS and CSS: `node scripts/test-easydrag.mjs` and `node scripts/test-easydrag-render-baseline.mjs`.
  - T0 (`npm run test:easydrag:t0`) at the end of each part.
- **Line ratchet.**
  - `easydrag-canvas.js` stays at 928 lines.
  - `easydrag-model.js`, `-template.js`, `-editor.js`, `-home.js` and `-detail.js` change existing lines only.
- **Tests.** Essential tests only. No external assets; `TestShippedWebAssetsLoadNothingExternal` stays green.

## Shared files

- **Locale files** (`ui/lang/easydrag/*.json`).
  - Change them only through key files and the helpers:
    - `C:/disposable/easydrag-w1a/locale-apply.mjs`, for keys with a text in every locale;
    - `C:/disposable/easydrag-w1a/locale-apply-forms.mjs` (Part C), for plural forms that only some locales need.
  - Key files live in `C:/disposable/easydrag-w1a/keys/`.
  - On a rebase conflict, take main's locale files and re-run the part's key files in task order. Both helpers are idempotent.
- **Module registration.** `ui/js/desktop/core/module-loader.js`, the Go list `easyDragModules` and the Node harness `MODULES` lists all take new modules from A, C and D. On a conflict, keep both entries.
- **New Go tests go into new files**, one per concern, so `ui/desktop_easydrag_test.go` changes only in its module list and its locale family map.
- **CSS.**
  - A owns the token block and the status rules.
  - D owns the rail rules and the motion gate rule.
  - B (after A) owns the kit rules and every `font-size`.
  - A conflict between A and D is merged by hand; the two parts touch different rules.

## Amendments found while planning

Each amendment is applied in the named part. The coordinator copies them into the spec when the part lands. Spec §3.4 and §4.1 were updated before this plan was committed, for amendments 1 and 13.

**Part A, status and tokens**
1. **Radius naming (A.1).**
   - The 12px token is `--ed-radius-xl`.
   - `--ed-radius` (10px) and `--ed-radius-lg` (14px, 16px in Fruity) keep their values.
2. **Motion tokens (A.1).** They read the desktop's `--ds-motion-*` durations, with 120 and 200 ms fallbacks, because Amiga sets them to 0. `--ed-fast`, `--ed-base` and `--ed-ease` become aliases.
3. **The grey (A.1).**
   - `--ed-status-neutral` is `color-mix(in srgb, var(--ed-text) 60%, var(--ed-panel-strong))`. It reaches 3:1 on canvas, card and well in all 16 contexts; the worst case is 3.32:1 (voxel_world, windows_xp).
   - The glyph on a grey disc uses the panel colour, not white.
   - The grey is checked for contrast only. It is not checked for distance to the category tiles (coordinator decision): the logic and tool tiles are grey by design, and grey statuses carry their meaning through the glyph.
4. **Two extra tokens (A.1).**
   - `--ed-status-edge` is a 1px grey ring on every disc. Without it, the coloured discs fail 3:1 on the light surfaces of fruity-light, amiga, windows_xp and voxel_world.
   - `--ed-status-ink` gives the glyph ink per band. Lollipop and minimal-light use white, because their status colours are dark.
   - The status colours themselves stay fixed (E2 guardrail). The owner judges the ring on the lab screenshots.
5. **`stopped` (A.4).**
   - The status word replaces S0c's hint line.
   - The stop step's name moves into the `title` tooltip of the `stopped` disc and label, wherever `stoppedBy` gives it (coordinator decision).
6. **Allow-list (A.7).** The card's state ring `.ed-node.status-<raw>` stays, with its reason: canvas.js may not grow, and many browser tests select it.
7. **One word for uncertain (A.6).** `easydrag.ui.port_uncertain` takes the same word as `easydrag.status.uncertain`, and a test checks that they match in every locale.
8. **Extra API (A.2–A.4).** `ED.status.ofStep`, `table`, `bind(t)`, `disc(…, { t, title, hidden })` and `label(…, wordOverride)`; `ED.durable.value`; `ED.testing.statusOf` and `statusLabel`.
9. **No pulse on the running disc.** `@keyframes ed-pulse` stays; Part D owns animation rules.

**Part B, kit and type scale**

10. **`notify` stays (B.3).**
    - The ~81 `notify` calls keep the desktop-wide `vd-toast`, which has no action slot.
    - Only the undo toast moves onto `ED.kit.toast`, and its test hook becomes `data-ed-toast-action`.
    - Restyling the desktop toast is a later desktop task.
11. **Info callouts (B.1).** `callout` also takes `status: 'info'`, with its own small "i" disc. `info` is not a status value, and the status lint must not flag it.
12. **Kit API (B.1).** It adds `disclosureMarkup`, `attach`, `isOpen`, `setOpen`, `setTitle`, `setCount` and `body`. The string-template call sites use `disclosureMarkup`, and one delegated listener on `.ed-app` toggles them.
13. **Type scale (B.5), owner decision of 2026-11-13.**
    - The scale is **11/12/14/16/18/24**, not 11/12/13/15/18/24. Principle 1 needs card names at 13.8px or more and parameter lines at 12px or more at fit zoom 0.86.
    - Touch input fields use 16 (card), so iOS does not zoom into a focused field.
    - The thresholds of `TestDesktopEasyDragLegibilityBrowser` stay unchanged.
14. **Ratchet boundary (B.6).** The boundary is the property name: `--ed-*` declarations are ignored. It is not a marker comment, because the tokens sit in four rules.
15. **Render baseline (B.5).** It is expected to stay unchanged, because it counts render work on a stub DOM, not pixels.
16. **The sheet's grab bar (B.2)** is decorative and has no swipe gesture.

16a. **Kit selectors and the status lint (B.1 meets A.7).**
   - The kit CSS addresses a disc inside a callout or a toast as `[role="img"]`, never as `.ed-status-disc`. It adds two token aliases, `--ed-kit-ink` and `--ed-kit-neutral`, so no kit rule spells an `ed-status-` class.
   - The `info` callout uses `.ed-callout-disc` and `data-status="info"`, with no status class and no status word key.
   - B.1 reads Part A's lint before it writes CSS, and uses `--ed-radius-xl` for 12px (amendment 1).

**Part C, language and numbers**

17. **`duration` keeps today's unit style ("400 ms"),** which `scripts/test-easydrag-ux-batch5.mjs` pins. Demo 3.1's "0,4 s" is for W1d to decide.
18. **The template engine's `en-GB` date filter stays** (`easydrag-template.js:407`, with a comment), because the server evaluates the same expressions.
19. **Count keys (C.3).**
    - 25 of the 34 `{count}` keys are number-neutral, because the count follows a colon or sits in brackets. They get no forms.
    - `home_bulk_selected` (es, fr, it, pt, sv) and `batch_result_summary` (el, es, fr, it, pt, sv) are reworded to the colon form.
    - A Go test sorts every `{count}` key into one of the two lists.
20. **The versions row (C.3b).**
    - `versions_stats`, `versions_stats_one` and `versions_stats_edge_one` are deleted. The row composes `group_member_count` and the new `versions_connections`.
    - `scripts/test-easydrag-plurals.mjs` (S0c A6) is deleted, because the format test and the Go tests cover it.
21. **"vN" (C.4).**
    - 13 keys change. "Revision" appears in 7 German and 9 English values; the survey's "46 German lines" was wrong.
    - `run_view_pruned` keeps a noun before the number in pl, hi, ja and zh, where a bare "v3" lacks a subject.
22. **The UI locale** is `window.SYSTEM_LANG`, with `document.documentElement.lang` and then `en` as fallbacks. The pseudo wrapper sits where EasyDrag receives `t` (`easydrag.js`, `t: ctx.t`).

**Part D, rail, icons and motion**

23. **The rail at 390px** stays a 64px column, because the CSS only reorders it. The caption test measures it there too.
24. **The motion gate (D.4).**
    - It is one negative rule: no animation inside `.ed-app` unless `data-motion="normal"`.
    - The audit found no idle animation: all 15 run during a run, save, drag or load, or once on entry. So none is removed.
    - The browser test is the proof.
25. **"Lebendigkeit" (D.3).** It sits in the editor's "⋯" menu and in the block editor's menu. The home page gets it with W1f.
26. **Icons (D.1).** Start uses the existing `home` glyph. Gruppen gets a new `group-frame` glyph in the `ICONS` style, from the owner-approved mock. Part E shows it next to `folder` before sign-off.
27. **The test fixture already runs calm.** `ui/testdata/aurora-fixture.js` sets `windows.animations: false`. The motion test sets `data-animations` itself and counts only animations with `playState === 'running'`.

**Part E, lab**

28. **16 contexts:** the 14 themes plus the dark schemes of Fruity and Minimal, as in the colour test.
29. **The lab sits beside the contact sheet.** It reuses the sheet's run-done seed, and the sheet stays the owner's before/after view. Each theme gets its own page.
30. **Scenes.** Scenes 1 and 2 are panels injected with the real classes, on three surfaces (panel, canvas, well). The distance check uses `ok`, `failed`, `running` and `uncertain` only.
31. **`labKnown` (E.5).** A finding that needs a design decision may land as known, but only with the owner's word and a named reason. It is logged on every run.
32. **Time budget (E.4).** If the lab takes more than 240 s, the pseudo-locale run splits into `TestEasyDragLabPseudoBrowser`.

## Not covered by W1a (follow-ups)

- **Native-speaker check:**
  - the short rail captions (ja, hi, el, da);
  - the cs, pl, el and hi plural texts.

  Both happen in the Wave 1 sessions.
- `sizeText` in `easydrag-credentials-purge.js` still uses `toFixed(1)`. This is a one-line follow-up.
- **The desktop-wide toast look** (amendment 10). This is a desktop task.
- **A run the server still reports as queued or running** keeps its running state. W1c's status work decides how to show it.
- **W1b to W1f** (spec §2).

---

## Finishing W1a

1. **Rebase and re-check.** Rebase the part's branch onto the current main in its worktree. If locale files conflict, re-apply the key files.
2. **Run the part's checks:**
   - T0;
   - one full gated EasyDrag browser run;
   - for E, the lab itself.
3. **Check the main checkout.**
   - It must be on `main`: `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current`.
   - Compare its uncommitted files with the branch's files.
4. **Land** with `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only <branch>`. Do not push.
5. **Record.** After each part, the coordinator records on main, through a temporary index:
   - the amendments above into the spec;
   - the landed range into the roadmap's Wave 1 status;
   - the EasyDrag project memory.

   After the last part, the roadmap also gets:
   - in §10, decision 13 recorded as taken (S0c);
   - the six-part split of Wave 1;
   - S0c rows 23 and 24 pointing at the W1a spec.
6. **Clean up.** Remove the worktree and delete the merged branch.
