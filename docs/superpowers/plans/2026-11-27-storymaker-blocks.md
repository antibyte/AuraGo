# Storymaker blocks and reliable code scenes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Storymaker agent makes better explainer films: free-mode code scenes finish reliably and are reviewable, and both modes get explainer blocks (icons, diagrams, charts, kinetic text) from one renderer.

**Architecture:** Stage 1 widens the output budget for free-mode code phases, fills scenes in two passes, gives the free fill packet code_rev, safe_box and an API card, and lets code scenes reach the stills and contact sheet. Stage 2 adds a player module (ui/js/storymaker/blocks.js, blocks-draw.js, blocks-plans.js) that draws Lucide icons, diagrams, charts, headlines and lists on Canvas2D as a pure function of time. Storyboard beats (beats-blocks.js) and code scenes (api.icon/diagram/chart/headline/list) create the same block entities, the layout check measures them, and a new env.board environment and board areas place them. Batch F teaches the agent to use them and replaces unfilled scenes with a fallback board.

**Tech Stack:** Go (internal/storymaker, internal/server, internal/agent), plain-JS player (ui/js/storymaker, bundled by npm run build:ui), Node test scripts (npm run test:storymaker, node --test), Lucide icons (lucide-static 1.82.0, ISC/MIT), headless Chrome for the browser test.

---

## Documents

- Spec: docs/superpowers/specs/2026-11-27-storymaker-blocks-design.md (approved by the user 2026-11-27).
- design.md (this folder): the binding seams (names, types, JSON shapes, limits, file ownership).
- design.md rule: where a part file and design.md disagree, the part file wins. Part files are generated from tested commits.
- Deviations made while drafting are recorded in the commit bodies and in "Deviations from design.md" below.
- part-A.md through part-F.md (this folder): the tasks, generated from the drafted commits.

## How this plan was made

- Each batch was drafted as real code, one commit per task.
- Commit subject format: `smb <X><n>: ...`. The body has Test, Red and Green lines.
- Drafting ran in scratch worktrees on base 70e65ce41.
- Branch chain: smb-a, then smb-b, smb-c, smb-d, smb-e, smb-f.
- Each batch got one combined review. Review fixes are their own tasks (A13-A15, B7, C11-C14, D10-D12, E8, F11-F13).
- The part files were generated from the commits and replayed on the base in memory: 144 files, 0 differences from the tip.
- Branch tips (for cherry-picking instead of retyping):
  - smb-a f8bc11664
  - smb-b f9eb9c256
  - smb-c 0831f87f9
  - smb-d 152499ce2
  - smb-e 105ea7ca0
  - smb-f 4a780758f
- The branches live in the local repo. The worktrees are under C:\disposable\smb\.

## Batches

| Part | Tasks | Content | Depends on |
|------|-------|---------|------------|
| A | 15 (A1-A15) | Stage 1: output budget, two-pass fill, closing markers, safe box and api.safe(), free packet code_rev/safe_box/api_card, profile, references, code-scene layout blocks; review fixes | base |
| B | 7 (B1-B7) | Icons: converter, build script with integrity check, 516 icons, licence and NOTICE, catalog delivery, Go loading, search incl. free mode | A (only for the shared chain) |
| C | 14 (C1-C14) | Player blocks, icon/diagram/chart/text painting, measurement, render/raster integration, env.board, board areas; review fixes (text fit, narrow areas, payload hardening, even draw-on) | B |
| D | 12 (D1-D12) | Storyboard entry: model, Go validation, node highlight, node cue_phrase, beats, layout measurement, Go layout rules, agent schema, app codes and locales; review fixes | C |
| E | 8 (E1-E8) | Code entry: api.icon/diagram/chart/headline/list, API card, references and free example, board colour tokens; review fix | D |
| F | 13 (F1-F13) | Browser fixture film, free outlines on env.board, suggested_block, block fill example, advice findings, fallback board, job end, profiles v7, manual, AGENTS.md; review fixes (suggested_block word rules, fallback skipped on a user stop, guidance wording) | E |

- Total: 69 tasks.
- Land C-F together. The profiles only teach blocks in F.

## Before you start

- Lucide package (needed by B2 and any icon rebuild):
  - Run `npm pack lucide-static@1.82.0` once in a scratch folder outside the repo.
  - Check integrity: `sha512-Xzzpfkx06hHXjXoLWOk1RWg52OhfaRcIOIK72hCbRty8Jq29WQUV4J4Z9GvjAKDoPxGf7tlcqBbJX4qrr1U8+w==` with `node scripts/build-storymaker-icons.mjs --verify <tgz>`.
  - Unpack it outside the repo and pass the unpacked `package` folder to `--package`.
- The package is data. Never run its code.
- Only the generated JSON, the allow-list and the licence files are committed.
- Go environment on this Windows box:
  - GOTMPDIR must be a short backslash path, for example C:\tmp\go-build-smb.
  - If `no such tool "vet"` appears, set GOTOOLCHAIN=go1.27.4.
- Worktrees need a real copy of node_modules. A junction breaks the Storymaker vendor check.
- Never convert eol=lf files to CRLF.
- When writing JS or Go with an editor tool, check that regex backslashes survived. This cost two fixes while drafting.
- Never use git stash in a worktree. The stash list is shared with the main checkout.
- Under machine load the server storymaker tests and internal/storymaker/render can pass Go's default 10-minute timeout. Use -timeout 25m.

## Batch-end checks

Run these after each batch:

```
go build ./...
go vet ./internal/storymaker/... ./internal/server/ ./internal/agent/
go test ./internal/storymaker/... -count=1
go test ./internal/server -run "(?i)storymaker" -count=1 -timeout 25m
go test ./internal/agent -run "(?i)storymaker" -count=1
go test ./ui -run "(?i)storymaker" -count=1
npm run test:storymaker
npm run build:ui
npm run check:ui
node scripts/build-storymaker-ranges.mjs --check
```

- Plus for B and C:
  - `node --test assets/storymaker/test/icons.test.mjs assets/storymaker/test/catalog.test.mjs assets/storymaker/test/environments.test.mjs`
  - `node scripts/build-storymaker-catalog.mjs --check`
  - `node scripts/build-storymaker-icons.mjs --check --package <dir>`
- After F, run once:
  - `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run "(?i)(TestCompileCheckBlocksBrowser|TestCompileCheckBrowser|TestCompileCheckStillsOfTheRealRuns)" -count=1 -timeout 30m`
  - This needs a local Chrome. Set AURAGO_STILLS_OUT=<dir> to keep the contact sheets.
  - `go test ./internal/audit/... -count=1`

## Baseline

- On the base 70e65ce41 every check above is green except one.
- Failing test: internal/audit TestLinkedGoModulesHaveGPL3CompatibleLicenses.
- Cause: a stale licences/linked-go-modules.tsv row for github.com/gobwas/pool.
- This plan did not cause it.

## Deviations from design.md (decided while drafting)

- API card bound: 1800 bytes after A, 2400 after E. The card is 2382 bytes. The E text was shortened to fit.
- Icon path character set is [MmLlHhVvCcSsQqTtAaZz0-9 .,eE-].
- An exact bare icon name scores like an exact tag, so `icon.lock` leads the icons for "lock". Catalog entries still win ties.
- Free catalog tool offers search (kind forced to icon) and reference in fill and revise. Check keeps reference only.
- Board rects were measured on the raster masters:
  - env.office {809,121,642,311} (night tone dark)
  - env.classroom {515,104,1020,338}
- The board area also clips to the visible frame and follows the stage focus.
- Extra SM.blocks exports: layout, plan, cacheKey, itemCount, lines, pathLength, kit.
- SM.scene.base is exported.
- blocks-plans.js holds the pie and list plans to keep it under the 600-line guard.
- Compact story schema stays at or below 900 tokens. It has no chart title, no compare points, no diagram flow switch and no descriptions. Guidance does not promise those in compact phases.
- Go reports errors exactly where the player's check() refuses a block. Ids are counted alike in Go and JS, legacy chart beats included.
- block_too_small judges diagrams and charts by box height. Text blocks are judged by label size.
- Code scenes get no caption-zone or camera advice. The look-alike fix text speaks code for code scenes.
- Code scenes on env.board read the board's colour tokens.
- Fallback headlines are cut to 8 words and 80 characters. Characters are counted as UTF-16 units, like the validator.
- The fallback board is applied when the job deadline ends the run, not when the user stops the job.
- The free fill example uses fixed seconds. It calls api.cue(n) only for markers {n} that exist in the narration.
- The fill example and the block beat example are German only for language base de, English otherwise.
- FallbackBoards clears the check and compile caches.
- Free outlines seed env.board only for agent jobs. Projects made in the app keep env.plain-stage for new scenes.

## Acceptance by the user on aurago-test

Agents never deploy. The user checks these after landing and deploying.

1. Free mode, MiniMax-M3.1-Flash (or the usual model). Request: "Was ist ein Agentic Harness".
   - Expect all 7 scenes filled or replaced by a fallback board. No empty curtain stage.
   - No model call ends at exactly 20480 completion tokens.
   - The contact sheet shows the code scenes.
   - Diagrams and headlines appear in several scenes.
2. Catalog mode. Request: "Wie funktioniert ein LLM".
   - Expect at least two scenes with blocks (diagram, chart, icon card or headline) at the board or on env.board.
   - A character points at the board.
   - No `{/1}` in captions.
3. Look at one export in 9:16.
   - Blocks stay inside the frame.
   - Captions stay free.

Evidence:
- data/storymaker/agent/jobs.json
- agent/events/<job>.jsonl
- The project folders under agent_workspace/virtual_desktop/Documents/Storymaker/

## Known limits and follow-ups

- Hub nodes and compare points are small in 16:9 at full size. Enlarge them after the first real films.
- Determinism inside onFrame is documented, not enforced. This was already the case before this plan.
- A film with code scenes stays layout_unverified. Raw g drawing is never measured. Block boxes are measured.
- Icon search is English only (Lucide tags).
- F13 is text only. Its tests were written after the edits, so no red state was observed.
