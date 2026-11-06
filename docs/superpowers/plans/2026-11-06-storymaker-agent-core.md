# Storymaker Agent Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The agent roster in the Setup section applies on top of that skill.

**Goal:** A user says in the chat or in the Storymaker app "make me an explainer video about X", and an agent writes a valid story from the prepared catalog, also on a weak model. The user can then revise it by instruction, open it and export it.

**Architecture:** Studio tools `storymaker_story`, `storymaker_catalog` and `storymaker_check` live as testable domain code in `internal/storymaker` and reach the model through `ExecutionHooks.HandleTool`, bound to one job and one project. A job runner in `internal/server` uses a third `render.Manager` for its queue and runs one agent loop per phase: outline, cast, one per scene to fill, check. Each loop has its own prompt, tools and context packet, in a native and a text-mode form. A short compile in Chrome reports real scene lengths and motion lint. Narration runs at the end, so "finished" means exportable. A chat tool `storymaker` and an app panel start and follow the jobs.

**Tech Stack:** Go, the existing agent loop in `internal/agent`, plain classic-script JavaScript with no framework (`scripts/build-ui-bundles.js` concatenates the parts), Chrome headless for the compile check. No new dependency.

This is the first slice of the spec's phase 3. It does not contain `storymaker_code`, the asset generator, stills, `checks.js`, music in agent jobs, or the other items of [design section 1](2026-11-06-storymaker-agent-core/design.md). Layout stays unchecked, so an outcome reads `ready_unverified` at best, and the user is told so.

## Documents

| Document | Role |
| --- | --- |
| [Design spec](../specs/2026-11-01-storymaker-design.md) | What and why of Storymaker. Task F2 records what this plan changed in it. |
| [Design of the agent core](2026-11-06-storymaker-agent-core/design.md) | Scope, decisions, interfaces, prompt texts. Later sections win over earlier ones: 11 over 10, 10 over 9, 9 over 7 and 8, those over 1 to 6. The part files hold the final code. |
| [Code maps](2026-11-06-storymaker-agent-core/code-map.md) | Where the code of each area lives, from a read-only analysis on 2026-11-06. Verify a line before relying on it. |
| `part-A.md` to `part-F.md` in the same folder | The 47 tasks. |
| `evidence/baseline.md` in the same folder | Written by task B0: the tests that already fail on main. |
| [evidence/dry-run.md](2026-11-06-storymaker-agent-core/evidence/dry-run.md), `evidence/dry.js` | The plan applied by script to a copy of main and tested at each batch end, what that proves and what not. Rerun `dry.js --check` if main has moved far before a batch starts. |

## Execution order

Run the tasks in exactly this order. It is checked by a dry run, see below.

`B0; A1 to A7, A7a, A8, A8a, A9, A10; B1 to B5; C1 to C4, C4a, C5, C6, C7a, C7, C7b, C8 to C10; D1 to D6, D6a, D7; E2, E3, E3a, E4; F1 to F4.`

| Batch | Part | Tasks | Delivers | Visible to the user |
| --- | --- | --- | --- | --- |
| A | [part-A.md](2026-11-06-storymaker-agent-core/part-A.md) | B0, A1 to A10 | Config keys, Studio session and tools as Go code, word check, compile check, outcome rules, `make-silent` logic | No |
| B | [part-B.md](2026-11-06-storymaker-agent-core/part-B.md) | B1 to B5 | Job tool schemas, private route and scope, call repair, recovery nudge, output trust | No |
| C | [part-C.md](2026-11-06-storymaker-agent-core/part-C.md) | C1 to C10 | Runner, events, profiles, revise, scripted-model tests | No |
| D | [part-D.md](2026-11-06-storymaker-agent-core/part-D.md) | D1 to D7 | Routes, write guards, project launch, agent panel, start card, locales, notification | Yes: the app can start the agent |
| E | [part-E.md](2026-11-06-storymaker-agent-core/part-E.md) | E2 to E4 | Chat tool with its whole registration chain and manual | Yes: the chat can start the agent |
| F | [part-F.md](2026-11-06-storymaker-agent-core/part-F.md) | F1 to F4 | Config page, docs, final matrix, landing, the user's acceptance recipe | Yes |

Each batch lands alone and leaves main green. Batch A starts with B0. Task D7 stands after D6a. The numbers E1 and C7 of older notes: E1 no longer exists, its work is C7a.

## Setup for the executor

- **Worktrees.** One per batch: `.worktrees/storymaker-agent-a` to `.worktrees/storymaker-agent-f`, each on a branch of the same name, each started from the main of that moment. `node_modules` in a worktree is a junction to the main checkout's folder. Remove it as a link only: PowerShell `(Get-Item -LiteralPath $p -Force).LinkType -eq 'Junction'`, then `[System.IO.Directory]::Delete($p,$false)`. Never delete through it.
- **Agents.** The user's roster, with no model overrides. maker-agent writes and edits code. test-agent runs tests and validation. reader-agent does one combined review per batch, Critical and Important findings only. debug-agent investigates failures. Fixes go back to maker-agent. security-agent reviews batches B, C and E. docs-agent writes docs. No re-review after a fix: the controller checks the diff and reruns the tests the fix touched.
- **Tests.** The user's rule is "reduziere tests auf ein minimum". Only the tests a task names run per task, and no red run. The batch matrix below runs at each batch's end.
- **GitNexus.** Before editing an existing symbol, from the main checkout: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`. Report callers, processes and risk, and warn the user on HIGH or CRITICAL. Before each commit, alone, from the worktree root: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo`. Do not gate on its exit code. A symbol created by an earlier task is not in the index, so `UNKNOWN` or PARTIAL is expected there: confirm callers with a text search and say so in the commit body. Never re-index.
- **Go on this box.** `GOCACHE='D:\dev-cache\go-build'` and `GOTMPDIR='D:\tmp\go-build'` in every shell, in that backslash form. If a build fails for lack of space, confirm no go or test process of concern runs, trim entries older than 3 days in `D:\dev-cache\go-build`, and tell the user.
- **Browser tests.** Only with `AURAGO_RUN_BROWSER_SMOKE=1`, set on that line alone. Chrome starts only from test binaries under `D:\tmp\go-build`, with the shell sandbox off, so run them through `go test` with `dangerouslyDisableSandbox: true`, one foreground command at a time. Export lines also set `AURAGO_TEST_FFMPEG_DIR` to the local FFmpeg bin folder.
- **Line endings.** `core.autocrlf` is true. Files with `eol=lf` (Go sources, `config_template.yaml` and more) stay LF. The JS, CSS and JSON files of the app may be CRLF in a worktree and stay so. The plan's Old and New blocks show LF: match them with the file's own endings, by a small node script when a multi-line Old is involved. Before each commit, `git ls-files --eol | grep 'attr/text eol=lf' | grep w/crlf` must print nothing.
- **Formatting.** Run `gofmt -w` on each new or changed Go file before its commit. Two files in the plan's text are known to need it: `internal/storymaker/agentjob.go` and `internal/agent/native_tools_storymaker_chat.go`. Do not reformat files this plan does not change: some are unformatted on main.
- **Bundles.** After a JS change, `npm run build:ui` and `npm run check:ui`. A rebuilt `ui/js/desktop/bundles/main.bundle.js` is committed with the task that changed its sources.
- **Commits.** One per task, named paths only, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. `docs/` is git-ignored, so those paths and a new nested `AGENTS.md` need `git add -f`. Never push unless the user asks. No real TTS, music or LLM calls in tests. Never use a bare `git stash`, never kill a process by image name, and stop only your own PIDs.
- **Other sessions** commit in the same main checkout and keep uncommitted edits there. Never stage in its index, and never commit, stash or restore their edits.

## Batch matrix

At the end of each batch, from its worktree, each line in the foreground, one at a time, on a quiet machine:

1. `go build ./...`, and `GOOS=linux go build ./...`
2. `go vet` on the touched packages
3. Tests of the touched packages. For `internal/server`: `-run Storymaker`. Batches B and E also run `./internal/agent`, `./internal/prompts`, `./internal/security`; batch E also `./internal/audit`.
4. `go test ./ui -count=1 -run '(?i)(storymaker|config|i18n|registration|TestShippedWebAssetsLoadNothingExternal)'`
5. `npm run check:ui` when JS changed
6. From batch D on, the browser tour: `go test ./internal/server -v -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 9m`. A skip counts as failed.

A batch passes when its failures are a subset of `evidence/baseline.md`.

## Landing a batch

In this order, in the batch's worktree:

1. `git status --short` is empty.
2. `git rebase main`. For a conflict in `main.bundle.js`, rebuild instead of merging.
3. `npm run build:ui`. If the status is not empty, run detect-changes, stage each listed bundle by name and commit `build(ui): rebuild bundles`.
4. The batch matrix again when main's new commits overlap the batch's files. Otherwise only the lines covering changed packages.
5. Any failure not in the baseline is confirmed on a clean main before it counts as inherited.
6. Checks on the main checkout: `git branch --show-current` is `main`; `git merge-base --is-ancestor main <branch>`; `git diff HEAD --name-only` and `git diff main...<branch> --name-only` share no file; `git ls-files -v AGENTS.md` starts with an uppercase `H`; `.git/index.lock` does not exist. If main moved, rebase and rebuild. If another session holds an overlapping file, wait and poll, and ask the user if it runs long. A stale index lock is removed only with the user's OK.
7. `git -C <main> merge --ff-only <branch>`, as its own step, never chained after the checks.

The user decides each landing. Report the landed SHA, each failure confirmed on clean main, and what stays open, and ask before starting the next batch.

## Decisions the user may still overrule

Taken with the design's recommendations, since they change little:

1. Narration runs automatically at the end of a job, including cloud voices, under the existing daily and per-job caps.
2. The agent is on by default (`storymaker.agent.enabled: true`). The chat tool is intent-triggered, so other chats pay nothing.
3. No music in this slice. The Music tab stays for the user.
4. "Export without voice" is a visible button when no voice is ready, and it can be undone with "Restore speech".

## Open after this plan

- The user-run acceptance on aurago-test, recipe in task F4. It also measures text-mode parsing on real GLM replies, the `{n}` cue markers on weak models, and whether 2.3 words per second reads as rushed in a short film.
- Asset generator, `storymaker_code`, stills, `checks.js`, directions, rhythm plan, typed parameters, comments: a later plan.
- From the cinecode review, for that later plan: a six-question jury with at most three fixes, beat types with their pace, pixel checks on the finished film (real action per scene, one-frame flashes, repeated pictures), and a table of motion devices by job.
