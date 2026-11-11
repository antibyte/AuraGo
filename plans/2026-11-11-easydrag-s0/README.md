# EasyDrag S0 "Room, trust and legibility" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement S0, the first sub-project of the EasyDrag roadmap. It makes room in the full modules, measures rendering first, closes four trust gaps and three small defects, and makes the canvas readable with colour that means status.

**Architecture:**
- **Front end.** The work keeps EasyDrag's vanilla classic scripts (shared `window.EasyDrag`) and its deterministic Go engine. Split modules keep their facades (`ED.runs`, `ED.home`, `ED.fields`). A render-counter baseline guards every later change.
- **Engine and server.** Trust fixes sit at the engine, decisions and agent-dispatch boundaries.
- **Visual changes.** They are CSS tokens, guarded by a pure-Go token test and a lint.

**Tech Stack:** Go 1.x (`internal/flows`, `internal/decisions`, `internal/agent`, `internal/server`, `ui` tests), vanilla JS classic scripts, CSS, Node test harnesses (`scripts/test-easydrag*.mjs`), go-rod browser tests.

**Spec:** [`plans/2026-11-10-easydrag-s0-spec.md`](../2026-11-10-easydrag-s0-spec.md). **Roadmap:** [`plans/2026-11-09-easydrag-next-roadmap.md`](../2026-11-09-easydrag-next-roadmap.md) §7.

---

## Files of this plan

| File | Spec | Tasks |
|---|---|---|
| [part-1-counters-and-runs.md](part-1-counters-and-runs.md) | §3 S0.0, §4 runs.js | 1.1 counters and baseline · 1.2 run core and run-stream · 1.3 run-start · 1.4 run-stop · 1.5 run-drawer |
| [part-2-splits.md](part-2-splits.md) | §4 editor, home, fields, ratchet | 2.1 header · 2.2 home grid · 2.3 template field and `ED.fieldsShared` · 2.4 fields-conditions · 2.5 headroom ratchet |
| [part-3-trust.md](part-3-trust.md) | §5 S0.2 | 3.1 decisions encoder · 3.2 per-leaf fence, guard and size check · 3.3 notes, Cf rule and scrub · 3.4 agent switches |
| [part-4-defects.md](part-4-defects.md) | §6 S0.3 | 4.1 Simulation default, label and safe Enter · 4.2 goal counter · 4.3 trigger samples |
| [part-5-type-and-colour.md](part-5-type-and-colour.md) | §7 S0.4, §8 S0.5 | 5.1 card type · 5.2 tokens, bands, groups, lint and token test · 5.3 identity-mark usage · 5.4 contact sheet |

That is 21 tasks with one commit each, except 5.4, which commits nothing. Run them in this order: 1.1 → 1.5, 2.1 → 2.5, 3.1 → 3.4, 4.1 → 4.3, 5.1 → 5.4.

**Review batches** (spec §10): there is one combined review per batch, reporting Critical and Important findings only, and no re-review rounds.

| Batch | Covers | Review point |
|---|---|---|
| A | Part 1 and Part 2 (S0.0 and S0.1) | after Task 2.5 |
| B | Part 3 (S0.2) | after Task 3.4 |
| C | Part 4 (S0.3) | after Task 4.3 |
| D | Part 5 (S0.4 and S0.5) | after Task 5.3 |

Task 5.4 produces evidence for the owner and needs no review.

---

## Part 0: Set up the worktree

Other sessions commit in the main checkout, so S0 runs in its own worktree.

- [ ] **Step 1: Check the starting point.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo && git branch --show-current && git log --oneline -1 && git check-ignore -v .worktrees/easydrag-s0
  ```
  Expected:
  - the branch is `main`;
  - HEAD contains 3da58db83 (`git merge-base --is-ancestor 3da58db83 HEAD` exits 0);
  - `.worktrees` is ignored.
- [ ] **Step 2: Create the worktree.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0 -b feat/easydrag-s0 main
  ```
- [ ] **Step 3: Check that the baseline is green before any change.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0 && node scripts/test-easydrag.mjs && go test ./ui/ -run '(?i)(EasyDrag)' -count=1
  ```
  Expected: `all EasyDrag checks passed` and `ok  aurago/ui`. If something fails here, stop and report: it is a failure on main and does not belong to S0.
- [ ] **Step 4: Settle the detect-changes form once.**
  ```bash
  cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0 && node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .
  ```
  - If this runs (an empty diff is fine), the `DETECT` command below is valid in every part.
  - If it fails with "Repository not found", try `--repo C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0`.
  - If neither works, every commit body ends with the line `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`

---

## Conventions that override the parts

The five parts were planned in parallel. Where they differ, this section decides.

- **Shells.** Each part names its shell: Part 2 uses PowerShell, Parts 1, 3 and 5 use Git Bash, and Part 4 gives both forms where they differ. Use the shell the part names. Every command starts by changing to the worktree root, because the working directory resets between calls.
- **DETECT.** Use the form settled in Part 0 Step 4 for every commit in every part. A `partial` or `truncated` result is not clean: run it again, or fall back to the commit-body line above.
- **IMPACT.** Run `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo` before changing or moving a function.
  - `UNKNOWN` is unresolved: confirm the callers with a text search.
  - Warn on HIGH or CRITICAL before editing.
  - Closure-local JS functions usually come back `UNKNOWN`. That is expected, and the grep in each task confirms them.
- **Commits.** Stage exact paths and run `git commit`. The worktree has its own index, so the temporary-index recipe needed in the main checkout is not used here. Every message ends with a blank line and `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Baseline.** From Task 1.1 on, every task runs `node scripts/test-easydrag-render-baseline.mjs`. Expected: `PASS render baseline (unchanged)`. Never run it with `--update` in S0.
- **Browser runs.**
  - Set `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots`, so no run rewrites `reports/easydrag/*.png`.
  - Use `-run '(?i)(EasyDrag)' -timeout 30m`.
  - Every new browser test sets reduced motion explicitly.
  - If a single check fails, re-run that test alone before treating it as a regression: Windows motion settings can flip reduced motion. If Chrome cannot start from the test binary, follow `reference_chrome_launch_from_agent` (GOTMPDIR on D:).
- **Scratch files** go in `C:/disposable/easydrag-s0/`. Create them with the Write tool, never with heredocs.
- **Escapes.** The Write and Edit tools turn backslash-u sequences into characters. Go code uses `"\x5c" + "u003c"` or numeric runes such as `rune(0x202E)`; JS uses `String.fromCodePoint(...)`.
- **Locales.** Every new key goes in all 16 files with a real translation. German uses lowercase "du". Part 3 inserts its keys right after `easydrag.ui.issue_group_invalid`; Parts 4 and 5 add theirs elsewhere.
- **Tests.** Write essential tests only (the owner's standing preference). Add no extra property, mutation or stress suites.

---

## Amendments to the spec found while planning

Each amendment is applied in the named task. The spec and roadmap stay the reference for intent.

1. **Header context (Task 2.1).** The header receives `ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner, isDisposed(), lastRecord()`; `t`, `esc` and `ctx` come from `ed`. `draftModel` is not needed. `renderHeader` calls `renderBanner`, which stays in the editor.
2. **Home grid (Task 2.2).** The grid gets the callbacks that `render()` really calls: the library, organization and selection controls, `createOff`, `blockReason`, `metadataOf` and `metadataMarkup`. It reads the page state through a `state()` snapshot. The click-handler targets that the spec named stay in home.js.
3. **Fields (Tasks 2.3 and 2.4).**
   - `ED.fieldsShared` holds `nameAttrs`, `withChange`, `composing` and one shared id counter, `nextFieldId`.
   - `objects` lives in fields-conditions and is exported for fields.js.
   - `ED.fields` keeps its export line byte for byte.
4. **Run core (Part 1).** The four run parts register under `ED.runParts`. runs.js ends at about 146 lines (the limit is 400). The `extra5` content pins on runs.js strings move with the code.
5. **Fence API (Task 3.2).** `resolveParams` takes `map[string]fenceMode` (`fenceWhole` or `fenceLeaves`). New exports: `decisions.MaxStateBytes`, `decisions.MaxInstructionBytes` and `decisions.EncodeStateText`.
6. **Step-note scrub (Task 3.3).** `local_pack.go:421` scrubs step notes a second time. The scrub is idempotent, so the line stays; removing it is a later cleanup.
7. **Neutral group colour (Task 5.2).** `--ed-muted` grey cannot meet the 0.20 OKLab distance: it is about 0.14 from info and success in Standard. The token test therefore exempts the neutral group token from the distance rule. It still checks that no group token reads a status token, and that every coloured group token keeps 0.20. Spec §8 is amended to match.
8. **Theme survey (Task 5.4).** The registry has 12 themes besides Standard and Fruity. The survey shoots all 12, plus Minimal dark.
9. **Blue group fallback (Task 5.2).** The fallback list was computed by hand. Crazy (0.21) and Minimal dark (0.22) are close to the limit. The token test decides; follow its message.

## Not covered by S0 (follow-ups)

- **Ctrl+Enter quick test.** It still runs a real test with no dialog when every effect is already confirmed and remembered. Roadmap principle 4 forbids any shortcut that starts a real test, so this belongs to the rest of UX-07 in Wave 1.
- **Durable flows.** They still offer the Simulation option, which the server refuses. Hiding it, or making Simulation work for durable flows, is a Wave 1 or Wave 2 item.
- **The redundant step-note scrub** in `local_pack.go:421` (amendment 6).

---

## Finishing S0

After Task 5.4 and the owner's review of the contact sheet:
1. **Rebase.** Rebase `feat/easydrag-s0` onto the current `main` inside the worktree. Re-run `node scripts/test-easydrag.mjs`, the static Go tests, the touched Go packages (`go test ./internal/flows/ ./internal/decisions/ ./internal/agent/ ./internal/server/ -count=1`) and one browser run.
2. **Check the main checkout.** Before landing, check `git -C C:/Users/Andi/Documents/repo/AuraGo branch --show-current` and `git diff HEAD --name-only` for overlap with the branch.
3. **Land.** Land with `git -C C:/Users/Andi/Documents/repo/AuraGo merge --ff-only feat/easydrag-s0`. Do not push: the owner decides when to push and deploy.
4. **Record.** Update the roadmap's §6 Wave 0 entry and the EasyDrag project memory with the landed SHA.
