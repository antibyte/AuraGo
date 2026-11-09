# Storymaker Output Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The agent roster in the Setup section applies on top of that skill.

**Goal:** A film the in-app Storymaker agent makes looks and sounds like a deliberate explainer, also on a weak author model: varied scenes, readable composition, a check the agent can act on, and a result the user can look at (a contact sheet).

**Architecture:** Two stages from the [design spec](../specs/2026-11-09-storymaker-quality-design.md). Stage 1 puts the craft into the system: film-level lints with fix texts, a per-scene direction in the fill packet, composition v2 in the player (a caption-safe floor, a hero prop of a quarter of the frame height, a slow push-in) for projects the agent creates, an umlaut repair for German text, and a browser that is made ready so the compile check actually runs. Stage 2 lets the system see: the player measures every scene's layout during the compile run, Go turns the boxes into model-fixable findings, a film with nothing open can be `ready`, and one still per scene becomes a contact sheet on the job card.

**Tech Stack:** Go, plain classic-script JavaScript with no framework (`scripts/build-ui-bundles.js` concatenates the parts), Chrome headless for the compile run. No new dependency (`golang.org/x/image` is already a direct dependency).

This plan changes what the spec said in a few places, because the code differed from the spec's reading of it. [design.md](2026-11-09-storymaker-quality/design.md) records each decision and its reason. The one change a reader must know first: **the vision review (spec 2.3) is not part of this plan.** The agent loop can carry an image, but the policy that refuses local images for Agnes AI (the provider of the five real runs) would stop it there.

## Documents

| Document | Role |
| --- | --- |
| [Design spec](../specs/2026-11-09-storymaker-quality-design.md) | What and why, with the evidence of five real agent runs. The last task of batch E (E7) brings it in line with what was built. |
| [design.md](2026-11-09-storymaker-quality/design.md) | The plan's decisions where the spec had a gap or a conflict. They win over the spec. |
| `part-A.md` to `part-E.md` in the same folder | The tasks, A0 to E7. |
| [evidence/firewall-run.story.json](2026-11-09-storymaker-quality/evidence/firewall-run.story.json) | The real story an agent wrote on aurago-test on 2026-11-08. Task A1 turns it into a test fixture. |
| `evidence/baseline.md` in the same folder | Written by task A0: the tests that already fail on main. |

## Execution order

Batches A, B and C are independent. D needs A and B (its tests use the fixtures and helpers of A). E needs D. They overlap in a few files (`agentjob.go`, `studio_check.go`, `storymaker-agent.js`, the locale files, `beats.js`), so the order below keeps the rebases small: land A, B and C first, in any order, then D, then E. Within a batch, run the tasks in exactly this order:

`A0; A1, A2, A6, A7, A3, A4, A5, A8, A9, A10; B1 to B6; C1 to C6; D1 to D5; E1 to E7.`

(In batch A the tasks A6 and A7, the environment guide and the direction plan, come before A3, the lint that uses them.)

| Batch | Part | Tasks | Delivers | Visible to the user |
| --- | --- | --- | --- | --- |
| A | [part-A.md](2026-11-09-storymaker-quality/part-A.md) | A0 to A10 | Four film-level lints that never block, the direction packet and a worked example built from it, prompts v3 (and the fix of three entry styles the validator refuses), the fixtures | After the next agent run: varied scenes; the job card names open craft checks |
| B | [part-B.md](2026-11-09-storymaker-quality/part-B.md) | B1 to B6 | `style.compose`, `Beat.scale`, composition v2 in the player (caption-safe floor at 74 percent, hero prop, push-in), agent projects get v2 | Films the agent makes look different; old films do not change |
| C | [part-C.md](2026-11-09-storymaker-quality/part-C.md) | C1 to C6 | German umlaut repair, browser made ready before the check, the reason of an unavailable compile in the log, the event and the job card | The compile check runs on a box that never exported; the card says why when it cannot |
| D | [part-D.md](2026-11-09-storymaker-quality/part-D.md) | D1 to D5 | The layout run, five layout findings, outcome `ready` | A checked film can end `ready` |
| E | [part-E.md](2026-11-09-storymaker-quality/part-E.md) | E1 to E7 | Stills, contact sheet, `scenes_look_alike`, the link on the job card, the real-browser acceptance and the spec brought up to date | The contact sheet |

Each batch lands alone and leaves main green.

## Setup for the executor

- **Worktrees.** One per batch: `.worktrees/storymaker-quality-a` to `.worktrees/storymaker-quality-e`, each on a branch of the same name, each started from the main of that moment. `node_modules` in a worktree is a junction to the main checkout's folder. Remove it as a link only: PowerShell `(Get-Item -LiteralPath $p -Force).LinkType -eq 'Junction'`, then `[System.IO.Directory]::Delete($p,$false)`. Never delete through it.
- **Agents.** The user's roster, with no model overrides. maker-agent writes and edits code. test-agent runs tests and validation. reader-agent does one combined review per batch, Critical and Important findings only. debug-agent investigates failures; fixes go back to maker-agent. security-agent reviews batch C (the download gate) and batch E (the new route and the image handling). docs-agent writes docs. No re-review after a fix: the controller checks the diff and reruns the tests the fix touched.
- **Tests.** The user's rule is a minimum of tests. Only the tests a task names run per task. The batch matrix below runs at each batch's end. A red run before the code is written is not required.
- **GitNexus.** Before editing an existing symbol, from the main checkout: `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`. Report callers, processes and risk, and warn the user on HIGH or CRITICAL. Before each commit, alone, from the worktree root: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo`. Do not gate on its exit code. A symbol created by an earlier task is not in the index, so `UNKNOWN` or PARTIAL is expected there: confirm callers with a text search and say so in the commit body. Never re-index.
- **Go on this box.** `GOCACHE='D:\dev-cache\go-build'` and `GOTMPDIR='D:\tmp\go-build'` in every shell, in that backslash form. If a build fails for lack of space, confirm that no go or test process of concern runs, trim entries older than 3 days in `D:\dev-cache\go-build`, and tell the user.
- **`-run` patterns.** Go splits a top-level `|` before the regex flags: group alternatives, `'^(A|B)$'` or `'(?i)(a|b)'`, and count the PASS lines when a run is evidence.
- **Browser tests.** Only with `AURAGO_RUN_BROWSER_SMOKE=1`, set on that line alone. Chrome starts only from test binaries under `D:\tmp\go-build`, with the shell sandbox off, so run them through `go test` with `dangerouslyDisableSandbox: true`, one foreground command at a time. Tests that build the real fixture also set `AURAGO_TEST_FFMPEG_DIR` to the local FFmpeg bin folder. A skipped browser test counts as not run.
- **Scratch folder and the JS edit helper.** Edit scripts, the locale scripts and the docs commit script live in a folder outside the repo, for example `D:\tmp\storymaker-quality\`, and are run from the worktree root (`node D:/tmp/storymaker-quality/<name>.mjs`). The player parts, `storymaker-agent.js`, `storymaker-api.js` and the CSS file are CRLF, so a multi-line edit with the Edit tool would mix endings. Write this helper once as `D:\tmp\storymaker-quality\crlf-edit.mjs` with the Write tool; every JS edit script of the parts imports it:

```js
import fs from 'node:fs';

// edit(file, oldText, newText): replaces exactly one occurrence. The texts use \n; the file's own line endings are used.
export function edit(file, oldText, newText) {
  const src = fs.readFileSync(file, 'utf8');
  const eol = src.includes('\r\n') ? '\r\n' : '\n';
  const o = oldText.replace(/\r?\n/g, eol), n = newText.replace(/\r?\n/g, eol);
  const at = src.indexOf(o);
  if (at < 0) throw new Error(file + ': old text not found:\n' + oldText);
  if (src.indexOf(o, at + 1) >= 0) throw new Error(file + ': old text is not unique:\n' + oldText);
  fs.writeFileSync(file, src.slice(0, at) + n + src.slice(at + o.length));
}
```
  When an `Old` block in a part does not match because another batch changed the neighbouring lines, read that region and match the lines that are really there; the plan names the one line that must be unique.
- **Old and New blocks.** They show Go sources with tabs. If an Edit fails on whitespace, match the single most distinctive line of the block and put the rest of the change in `New`.
- **Line endings.** `core.autocrlf` is true. Files with `eol=lf` (Go sources, `config_template.yaml`) stay LF. The JS, CSS and JSON files of the app may be CRLF in a worktree and stay so; the locale JSON files are LF. Before each commit, `git ls-files --eol | grep 'attr/text eol=lf' | grep w/crlf` must print nothing, and `git diff --check` must be clean.
- **Formatting.** Run `gofmt -w` on each new or changed Go file before its commit. Do not reformat files this plan does not change.
- **Bundles.** After a change to a player part or to an app module that a bundle holds, `npm run build:ui` and `npm run check:ui`. A rebuilt `player.bundle.js`, `host.bundle.js` or `main.bundle.js` is committed with the task that changed its sources.
- **Commits.** One per task, named paths only, trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` (or the attribution line that the executing session's own rule gives). `docs/` is git-ignored: commit a change under it through the temporary index with the script below. Never push unless the user asks. No real TTS, music or LLM calls in tests. Never use a bare `git stash`, never kill a process by image name, stop only your own PIDs.
- **Docs commit script.** Write `D:\tmp\storymaker-quality\commit-docs.sh` once, and run it from the main checkout on `main` as `sh D:/tmp/storymaker-quality/commit-docs.sh <message-file> <path>...`:

```sh
#!/bin/sh
# Commits git-ignored files to main through a temporary index, without touching the shared index or the working tree of other sessions.
set -e
MSG="$1"; shift
cd "$(git rev-parse --show-toplevel)"
[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || { echo "not on main"; exit 1; }
IDX="${TMPDIR:-/tmp}/idx-storymaker-docs"
i=0
while [ $i -lt 4 ]; do
  i=$((i+1))
  OLD=$(git rev-parse HEAD)
  export GIT_INDEX_FILE="$IDX"
  rm -f "$GIT_INDEX_FILE"
  git read-tree "$OLD"
  for F in "$@"; do BLOB=$(git hash-object -w -- "$F"); git update-index --add --cacheinfo "100644,$BLOB,$F"; done
  TREE=$(git write-tree)
  NEW=$(git commit-tree "$TREE" -p "$OLD" -F "$MSG")
  unset GIT_INDEX_FILE
  if git update-ref refs/heads/main "$NEW" "$OLD"; then echo "committed $NEW"; break; fi
  echo "main moved, retry $i"
done
if [ ! -e .git/index.lock ]; then for F in "$@"; do git reset -q -- "$F" || true; done; fi
git log --oneline -2
```
  The message file holds the commit message and its trailer.
- **Other sessions** commit in the same main checkout and keep uncommitted edits there. Never stage in its index, and never commit, stash or restore their edits.

## Batch matrix

At the end of each batch, from its worktree, each line in the foreground, one at a time, on a quiet machine:

1. `go build ./...`, and `GOOS=linux go build ./...`
2. `go vet ./internal/storymaker/... ./internal/server ./internal/agent` (the packages the batch touched)
3. Tests of the touched packages: `go test ./internal/storymaker/... -count=1`; `go test ./internal/server -run Storymaker -count=1 -timeout 20m`. Batches B and E also run `go test ./internal/agent -run Storymaker -count=1`, `./internal/prompts` and `./internal/security`; batch C also `./internal/config`.
4. `go test ./ui -count=1 -timeout 20m -run '(?i)(storymaker|config|i18n|registration|TestShippedWebAssetsLoadNothingExternal)'`
5. `npm run test:storymaker` and `npm run check:ui` when JavaScript or a locale file changed
6. From batch D on, the browser lines: `TestCompileCheckBrowser` (D, E) and `TestCompileCheckStillsOfTheRealRuns` (E), each as the browser-test rule above says. Batch E also runs the app tour of the agent core: `go test ./internal/server -v -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 9m`.

A batch passes when its failures are a subset of `evidence/baseline.md`.

## Landing a batch

In this order, in the batch's worktree:

1. `git status --short` is empty.
2. `git rebase main`. For a conflict in a bundle, rebuild it instead of merging. For a conflict in a locale file, take both sides' keys: rerun the batch's key script (it refuses to add a key twice, so remove the batch's own lines first).
3. `npm run build:ui`. If the status is not empty, run detect-changes, stage each listed bundle by name and commit `build(ui): rebuild bundles`.
4. The batch matrix again when main's new commits overlap the batch's files. Otherwise only the lines covering changed packages.
5. Any failure not in the baseline is confirmed on a clean main before it counts as inherited.
6. Checks on the main checkout: `git branch --show-current` is `main`; `git merge-base --is-ancestor main <branch>`; `git diff HEAD --name-only` and `git diff main...<branch> --name-only` share no file; `.git/index.lock` does not exist. If main moved, rebase and rebuild. If another session holds an overlapping file, wait and poll, and ask the user if it runs long. A stale index lock is removed only with the user's OK.
7. `git -C <main> merge --ff-only <branch>`, as its own step, never chained after the checks.

The user decides each landing. Report the landed SHA, each failure confirmed on clean main, and what stays open, and ask before starting the next batch.

## Decisions the user may still overrule

Taken with the recommendations of [design.md](2026-11-09-storymaker-quality/design.md), since they change little:

1. **Vision review dropped** (see the first page). Reinstating it means: a setting `storymaker.agent.vision_review`, one `ExecuteMinimalLoop` call with the contact sheet through the job's route, and either a provider that accepts a local image or a public URL. About 5 to 8 files.
2. **Composition v2 is for the agent's projects only.** Editor projects and every existing film keep their look. A switch in the editor is not part of this plan.
3. **Craft and layout findings never make a job `incomplete`.** What is left after the check rounds becomes an open check on a `ready_unverified` job.
4. **A revise job gets no craft lint.** It makes the smallest change the user asked for.
5. **The browser download needs no new setting.** The agent calls the same `Ensure` the export calls, unless the desktop is read-only. A first agent run on a box that never exported therefore downloads the pinned Chromium (about 190 MB), with `browser` progress events.
6. **The caption-safe feet line is at 74 percent**, not 76 as the spec said: the push-in moves the feet down on screen by up to 2 percent.

## Acceptance by the user on aurago-test

No agent deploys to aurago-test, and agents read results there only when asked. After the user has deployed the batches they want, the recipe is:

1. In the Storymaker app start a new film with the request `was ist eine firewall`, about 60 seconds, language German, with the model they used before (agnes-3.0-flash).
2. Wait for the job card. Note: the first run may spend time downloading Chromium; the card's step list stays on "check" while it does.
3. Judge the film and the job card against the success criteria:
   - At least three different environments, and both cast members on stage where the story has two.
   - A readable prop in each scene, no caption over a foot, no scene with the same picture as its neighbours.
   - The narration has its umlauts ("prüft", "während", "persönlichen").
   - The outcome is `ready` or `ready_unverified`; if `ready_unverified`, every open check is named. A `compile_unverified` check names its reason.
   - The contact sheet link opens a picture of seven scenes.
4. Read the evidence when asked (read-only, `ssh aurago-test`): `/home/aurago/aurago/data/storymaker/agent/jobs.json` (the job's `agent` block: `open_checks` with `detail`, `review`, `repairs`), `agent/events/<job>.jsonl` (the `check` events with `compile_reason`, the `browser` events) and the project folder under `agent_workspace/virtual_desktop/Documents/Storymaker/<project>/` (`story.json`, `review/contact-sheet.png`).
5. Compare with the five runs of 2026-11-07/08 (seven identical scenes, `ready_unverified`, `compile: unavailable` every time). Those runs predate commit `e6ff84295` (the author packets and stage lints), so a clean run only shows the effect of both together.

## Open after this plan

- The acceptance run above, and with it the answer to what `e6ff84295` already improved.
- The vision review (decision 1).
- Sound: the export loudness is about -17 LUFS where the web target is about -14, and the density of automatic sound effects is slightly above the reference. Not changed here; the user decides.
- Words per second: the film planned for 55 s ran 42.2 s with the chosen voice (2.3 words per second is too slow for that voice). Not changed here.
- The layout of code scenes is never measured (`layout_unverified` stays for them).
- Speech and thought bubbles are not part of the layout run.
- A switch for composition v2 in the editor.
