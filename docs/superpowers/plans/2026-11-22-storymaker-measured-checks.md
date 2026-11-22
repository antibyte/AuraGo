# Storymaker Measured Checks and Cue by Phrase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Storymaker's agent check and export measurements that need no vision model (text off the frame, safe zones, contrast, flashing, frozen code scenes, a slow first scene, speech dropouts and clipping, mix balance), and let the agent cue beats by a phrase of the narration instead of numbered `{n}` markers.

**Architecture:** Measure where the data already is. Go checks the layout boxes and the stills that the browser compile check already takes. A new frame probe in the check's browser page renders every frame at 64 px within 20 s, and `checks.js` turns the luminance series into findings. Go checks synthesized speech pieces at narration time and the stems in the export mix's third pass. Cue phrases are resolved by the Studio into ordinary `{n}` markers, so the story format, the player and the editor do not change. Hard findings (`text_offframe`, `flash`) are warnings that block only the agent outcome. Everything else is advice or an export warning.

**Tech Stack:** Go (internal/storymaker, internal/storymaker/render, internal/server, internal/agent), the classic-script Storymaker player in `ui/js/storymaker/` (headless Chrome via rod), Node test runner (`npm run test:storymaker`), FFmpeg for the export tests.

---

## Documents

- Spec (user-approved 2026-11-22): `docs/superpowers/specs/2026-11-22-storymaker-measured-checks-design.md`
- Seams between the parts (binding; names, types, thresholds, file ownership): `2026-11-22-storymaker-measured-checks/design.md`
- Parts, applied in this order:

| Batch | Part file | Content | Tasks | Needs |
|---|---|---|---|---|
| 1 | `2026-11-22-storymaker-measured-checks/part-A.md` | Issue codes, classification, `text_offframe`, `text_unsafe`, safe zones, locale keys, editor titles | A1-A7 | main |
| 2 | `2026-11-22-storymaker-measured-checks/part-B.md` | `text_contrast` on the stills | B1-B5 | A |
| 3 | `2026-11-22-storymaker-measured-checks/part-D.md` | Cue by phrase: resolver, Studio wiring, beat schema, packet example, prompts | D1-D7 | main |
| 4 | `2026-11-22-storymaker-measured-checks/part-E.md` | Piece dropouts and clipping with one retry, envelope faults, mix warnings, export and job-card warnings | E1-E11 | main |
| 5 | `2026-11-22-storymaker-measured-checks/part-C.md` | Frame probe, `checks.js` (`flash`, `flash_unverified`, code-scene `dead_stretch` / `first_frame_empty`), `slow_hook`, 9:16 caption | C2-C14 (C1 is evidence) | A |
| 6 | `2026-11-22-storymaker-measured-checks/part-F.md` | Manuals, documentation, contract, AGENTS.md rules | F1 | A-E |

Batches 2 to 5 do not depend on each other's code. Apply them in the table's order anyway: every "replace ... with" block was generated against exactly this order.

## How this plan was made

Every task was first written as working code, test first, on scratch branches (`smc-a` ... `smc-f` in
`C:\disposable\smc\`), one commit per task. Each commit recorded its test command, the failing line before the change and
the passing line after it. The branches were then chained in the batch order above, and the part files were generated from
those commits. A replay script applied the generated parts to main e24358452 in memory and compared every touched file with
the chain's tip: identical, with no old text matching zero times or more than once.

Matrix on the final chain (all 45 tasks, including the review fixes; scratch branch `smc-final`, tip 34e590195):
- `go build ./internal/... ./cmd/...` ok.
- `go vet` on storymaker, agent and server ok.
- `go test ./internal/storymaker/... -count=1` ok (3 packages).
- `go test ./internal/agent -run '(?i)storymaker'` ok.
- `go test ./internal/server -run '(?i)storymaker'` ok.
- `go test ./ui -run '(?i)storymaker'` ok.
- `npm run test:storymaker` 264 passed, 0 failed.
- `npm run check:ui` ok.
- Browser runs with `AURAGO_RUN_BROWSER_SMOKE=1` (drafting of Part C): `TestCompileCheckFlashBrowser` and `TestCompileCheckBrowser` PASS.

One combined review (Critical/Important only) found no Critical issue and four Important ones:
- A probe failure passed the flash check silently. Fixed by C13.
- A released second take did not give its characters back to the job. Fixed by E11.
- The 9:16 caption test did not test the zone. Fixed by C14.
- The cut-window fallback for long films is missing. Not needed: the C1 measurement probes a 90 s storyboard film in 7 to 13 s, well inside the 20 s deadline.

## Execution rules

- Work in one worktree per batch (or one for all) under `.worktrees/storymaker-checks-<batch>`, branched from the current
  main. Batch 1 must be on main, or in the chain, before batches 2 and 5.
- Copy `node_modules` into the worktree with robocopy (`robocopy <main>\node_modules <worktree>\node_modules /E /MT:16`).
  A junction breaks `npm run check:ui`.
- Set a short backslash `GOTMPDIR`, for example `C:\tmp\go-build-smc`. Keep the default GOCACHE, and check the free space on
  D: first.
- Project CLAUDE.md: run GitNexus `impact` on every function you edit, and `detect_changes` before each commit. On
  2026-11-22 the index could not be opened (storage version 49 vs. engine 47; `gitnexus analyze --force` rebuilds it). If it
  is still unusable, say so in the task report and confirm callers with a text search; never treat an empty result as safe.
- Apply each task's steps in order. Run the test, see the red line, apply the implementation, run it green, then commit
  with the task's message. Bundles are never edited by hand: run `npm run build:ui` where a task says so and commit the
  regenerated files with that task.
- Never convert `eol=lf` files to CRLF. `docs/` is git-ignored: stage its files with `git add -f`.
- Other sessions share the main checkout. Land with `git merge --ff-only` from the main checkout, only when none of the
  batch's files are dirty there. Stage only your own paths. Never push or deploy without the user's word.
- Test scope follows the user's standing wish for lean runs: each task's own test, then the batch-end matrix below. One
  browser line per batch at most. No mutation, stress or repeated runs.

## Batch-end matrix

Run in the worktree after each batch, with the same lines every time:

```bash
go build ./internal/... ./cmd/...
go vet ./internal/storymaker/... ./internal/agent/... ./internal/server/...
go test ./internal/storymaker/... -count=1
go test ./internal/agent -run '(?i)storymaker' -count=1
go test ./internal/server -run '(?i)storymaker' -count=1
go test ./ui -run '(?i)storymaker' -count=1
npm run test:storymaker
npm run check:ui
```

After batch 5, also run once with the sandbox off:
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '(?i)TestCompileCheck(Flash)?Browser' -count=1 -v`.
Expected: `TestCompileCheckFlashBrowser` and `TestCompileCheckBrowser` PASS.

Known red outside this plan: none on main e24358452 for these lines. If one turns red on a newer main, rerun it on an
untouched main before blaming the batch.

## Decisions made while drafting

These choices fill gaps in the spec. A reviewer should not re-open them without a reason.
- `bed_too_loud` compares the music and the narration RMS only over the speaking 10 ms steps of each 6 s window (narration
  above -45 dBFS). Over the whole window, a normal mix with 50 % speech would warn falsely. `bed_gap` ignores the last 1.5 s
  (the fade-out) and beds below -90 dB. `MixResult.WarningAt` carries the time for the warning message.
- A dropout needs speech after the silence. Speech that simply ends in digital silence is not a dropout. `CheckPiece`
  measures its own unclamped 10 ms power, because the envelope's level byte stops at -60 dBFS.
- A cue-phrase refusal counts as one scene write, so a model cannot loop on bad phrases. Numbers already used by other beats
  count as taken. A refusal leaves the fields untouched.
- `AudioBlock.Faults` is a Go-only field (`json:"-"`), so the page, `/narration` and `boot.audio_info` keep their JSON.
- The probe runs only when `boot.layout` is set (the agent's check), never in exports. A frame error ends the probe without
  failing the check, and the page reports it as `probe_error`, which Go turns into `flash_unverified` (C13).
- `slow_hook` counts narration only, so a first scene with dialogue but no narration is flagged, as advice. The real
  firewall fixture now raises `slow_hook` (its first show lands at 2.1 s), and its node test allows it.
- `mix_clipped` has no end-to-end fixture: the master-gain bounds make a real clamp nearly unreachable. It is tested through
  `mixWarnings`.
- The editor titles the new codes. A7 extends `ISSUE_CODES` in `storymaker-core.js` and the pinned guard list.
- The spec's cheat fixtures are the tasks' own red-then-green tests: each new code has a test that makes it fire, and each was seen red before its code existed. The clean cases are the calm story of the browser test (C12), the clean mix fixture (E6), and the "none" cases in B3, C3 and C13. There is no separate table of cheats.

Known limits, reported and not fixed:
- `bed_gap` does not exclude impact dips of 12 dB or more. No catalog SFX has one today.
- A standing `{0}` marker is read as "no marker".
- A single word wider than the 9:16 band can still overflow the zone.
- On slow machines, films with many code scenes may end with `flash_unverified` for their tail. Code-scene frames cost about 30 to 40 ms each, against 3 to 5 ms for storyboard frames.

## Acceptance by the user on aurago-test

Agents never deploy to aurago-test and never write there. After the user deploys:
1. Start an agent job in 9:16 with a long title text. Expect `text_offframe` fixed by the agent, or the job `incomplete`. The editor shows the localized title.
2. Make a code scene that blinks full-screen black and white. The agent check reports `flash`, and the agent slows it down or the job ends `incomplete`.
3. Export a 9:16 film with captions. The caption sits above the bottom overlay band (about 78 % of the height).
4. Ask for a film with grey text on a grey environment. Expect the advice `text_contrast` in the job's advice list.
5. Watch an agent job's scene writes in the events: beats use `cue_phrase`, and the saved story holds `{n}` markers at the right words.
6. Export with a loud music track. If the music swells, expect `bed_too_loud` in the export warnings, and no refusal.
7. Listen to the narration of one job; a reported `narration_dropout` should match an audible gap.
