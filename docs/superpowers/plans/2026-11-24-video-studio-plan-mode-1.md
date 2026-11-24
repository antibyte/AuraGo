# Video Studio plan mode — slice 1 (planning) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Video Studio agent plans a complete clip as a validated storyboard (scenes, prompts, durations, transitions, keyframe sources, narration and music prompts) that the user reviews and edits in a new plan panel, all within what the active video generation API can do, without spending money yet.

**Architecture:** A pure-Go plan model and validator (`internal/videostudio/storyplan`) checks every plan against a capability profile derived from the configured video provider (`internal/server/video_studio_generation_profile.go`, which also takes over the manual generation rules). `plan.json` lives in the project folder with ETag concurrency and is served by new `/projects/{id}/plan` routes. A server-side agent job (modelled on the Storymaker agent core) writes the plan through five job-private tools in three phases (outline → details → check) or one revise loop. The Video Studio UI gains an "Edit | Plan" switch and a storyboard board that edits the plan with debounced saves.

**Tech Stack:** Go (`internal/videostudio/storyplan`, `internal/server`, `internal/agent`, `internal/config`, `internal/tools`), the existing agent loop (`agent.ExecuteAgentLoop`, prepared prompt profiles, execution hooks), vanilla JS classic scripts (`window.VideoStudioPlan`, `window.VideoStudioPlanBoard`), CSS, 16 desktop locale files, rod browser tests.

**Spec:** `docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md` (slice 1 = §13 item 1).

---

## 1. Documents of this plan

The plan is split into parts that were drafted in parallel and then reconciled. Later documents win on conflicts:

1. `2026-11-24-video-studio-plan-mode-1/contracts.md` — binding names, types, JSON shapes, routes, seams (the drafting
   brief) plus the reconcile decisions at its end.
2. `2026-11-24-video-studio-plan-mode-1/part-A.md` — storyplan core, capability profile, config (tasks A1–A8).
3. `2026-11-24-video-studio-plan-mode-1/part-B.md` — plan storage and HTTP API (tasks B1–B4).
4. `2026-11-24-video-studio-plan-mode-1/part-C.md` — planning agent: tools, studio, prompts, job (tasks C1–C4).
5. `2026-11-24-video-studio-plan-mode-1/part-D.md` — storyboard panel UI, strings, browser tour, contract (tasks D1–D4).

Each part starts with its prerequisites and ends with "Contract notes" (what the drafter found in the real code). The
notes are explanations; the task text is what gets executed.

## 2. Execution order (fixed)

| Batch | Tasks | What lands |
|---|---|---|
| 1 Core | A1, A2, A3, A4 | storyplan types, Decode/Marshal/Normalize, word budget, validator; `tools.MiniMaxVideoAPIModel` |
| 2 Profile and config | A5, A6, A7, A8 | capability profile + status `generation.profile`; generate admission on the profile; `video_studio.plan` config; config page section |
| 3 Storage and agent tools | B1, B2, C1, C2 | plan.json store, availability + status `plan`; server-field merge; job-private tool schemas; plan studio |
| 4 Agent job and routes | C3, C4, B3, B4 | prompt profiles; agent manager and job; GET/PUT plan; agent start/stop routes |
| 5 UI | D1, D2, D3, D4 | board + header switch + CSS + script lists; strings in 16 locales; browser tour; AGENTS.md contract |

B3/B4 come after C4 because the routes call the agent manager that C4 declares. Do not reorder.

## 3. Setup

- Work in a worktree: `git worktree add .worktrees/vs-plan-1 -b feat/vs-plan-mode-1 main` from the main checkout
  (`C:\Users\Andi\Documents\repo\AuraGo`). Never start a job in the same message as the `git worktree add` it needs.
- `node_modules`: copy, do not junction (a junction breaks the Storymaker vendor check of `npm run check:ui`):
  `robocopy C:\Users\Andi\Documents\repo\AuraGo\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`.
- Go environment: `GOCACHE=D:\dev-cache\go-build`; `GOTMPDIR` short and in backslash form (`D:\tmp\go-build`, or
  `C:\tmp\go-build` for `internal/server` tests that compare paths across drives). If `go vet` reports
  `no such tool "vet"`, prefix with `GOTOOLCHAIN=go1.27.4`. If D: is full, trim cache entries older than 3 days.
- Other sessions commit in the main checkout concurrently. Never `git add -A`; stage only the task's files. Landing
  is `git merge --ff-only feat/vs-plan-mode-1` from the main checkout after a rebase, and refuses when another
  session has uncommitted edits in a file this branch changes (check with `git -C <main> diff HEAD --name-only`).
- CLAUDE.md GitNexus rules: run impact before editing an existing function (MCP `impact` with
  `repo: C:\Users\Andi\Documents\repo\AuraGo`; the index is stale for recent Video Studio work, so confirm callers with
  grep and say so), and `detect_changes` before each commit. HIGH/CRITICAL risk → stop and report to the controller.
  UNKNOWN is unresolved, never safe.
- Line endings: Go and locale JSON are LF (`.gitattributes` eol=lf). Never convert files. Run `gofmt -w` on new or
  changed Go files.

## 4. Agents (CLAUDE.md roster, no model overrides)

- maker-agent implements tasks; small consecutive tasks of one batch may go to one maker in one dispatch.
- test-agent runs the batch-end matrix (foreground commands only, full logs, no piping through `tail`).
- reader-agent does one combined spec+quality review per batch (Critical/Important only; minors go to a list for
  the final review). security-agent reviews batches 3 and 4 (routes, scopes, read-only, job-private tools, prompt
  data wrapping, plan.json size and path handling).
- docs-agent writes D4 (AGENTS.md).
- Lean tests: the tasks' own tests plus a test per real bug fix; no extra suites, no re-review rounds after small
  fixes (the controller checks the diff and reruns the tests).

## 5. Batch-end matrix

Run after each batch (only the lines whose packages the batch touched, all lines after batch 5):

```
go build ./...
go vet ./internal/videostudio/storyplan/... ./internal/server ./internal/agent ./internal/config ./internal/tools
go test ./internal/videostudio/storyplan/... -count=1
go test ./internal/config -count=1
go test ./cmd/config-merger -count=1
go test ./internal/tools -run '(?i)(MiniMax|VideoGeneration)' -count=1
go test ./internal/agent -run '(?i)(VideoStudioPlan|NativeTool|Strict|ActionPolicy)' -count=1
go test ./internal/security -run '(?i)ToolOutput' -count=1
go test ./internal/server -run '(?i)(VideoStudio|Storymaker)' -count=1 -timeout 30m
go test ./internal/videostudio -count=1
npm run check:ui
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)(DesktopVideoStudio|VideoStudioNavigationLoadOrder|VideoStudioCountStrings)' -count=1 -timeout 30m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '(?i)VideoStudioBrowser' -count=1 -timeout 30m
```

Known environment noise (not regressions): `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` fails
when GOTMPDIR is on another drive than the checkout; Windows animation settings flip `prefers-reduced-motion` in
browser tests. Check any other red test on an untouched `main` before treating it as caused by this branch.

## 6. Done when

- All tasks committed on `feat/vs-plan-mode-1`, batch-5 matrix green (except the known environment noise), the
  final reader-agent review has no open Critical/Important item.
- Manual check by the controller with a scratch instance or the browser tour: the switch appears only with an open
  project and `video_studio.plan.enabled`; a provider without plan support (Agnes) shows the reason; a plan can be
  created (real or scripted model), edited, revised per scene, and survives a reload.
- Landing on `main` and pushing only on the user's word. Real-model acceptance on aurago-test is the user's.

## 7. Out of scope (later slices)

Preview step (keyframes, narration TTS), provider lane, plan runner, still chaining, bridge execution, timeline
assembly, stop/resume/regenerate of generations, music, levels, chat tool `video_studio_plan`, manuals and
`documentation/*` updates (spec §13 items 2–4).
