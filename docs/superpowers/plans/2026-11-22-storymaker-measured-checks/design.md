# Storymaker measured checks: seams between the parts

Binding for every part. The spec is `docs/superpowers/specs/2026-11-22-storymaker-measured-checks-design.md`; where this file
is more precise, this file wins. Every name below is final: do not rename, do not add a second spelling.

## Parts, order and file ownership

| Part | Content | Branch / worktree | Starts from |
|---|---|---|---|
| A | Issue codes, classification, layout text checks (`text_offframe`, `text_unsafe`), locale keys of all new issue codes | `smc-a`, `C:\disposable\smc\a` | main e24358452 |
| B | Text contrast on the stills | `smc-b`, `C:\disposable\smc\b` | A's tip |
| C | Frame probe, `checks.js` rules (`flash`, `flash_unverified`, code-scene `dead_stretch` / `first_frame_empty`), `slow_hook`, 9:16 caption | `smc-c`, `C:\disposable\smc\c` | A's tip |
| D | Cue by phrase (Studio resolver, beat schema, prompts, reference, tools manual) | `smc-d`, `C:\disposable\smc\d` | main e24358452 |
| E | Audio: piece faults + retry, mix warnings, narration faults on the export and the agent job card | `smc-e`, `C:\disposable\smc\e` | main e24358452 |
| F | Docs (manuals, documentation/storymaker.md, contract), final matrix | after A-E are chained | |

The plan applies the parts in the order A, B, D, E, C, F (C was drafted last and goes last; the generated "replace ... with"
blocks are exact for this order only). A part only edits the files listed for it here; a file not listed needed the
controller's OK while drafting. As drafted, E also edits `internal/server/storymaker_narrate.go` and
`internal/server/storymaker_agent.go`, and C also edits `ui/js/storymaker/schedule.js` (`settledFrame`) and
`scripts/storymaker-tests/*.mjs`. Shared files and who edits which region:

- `internal/storymaker/model.go`: A only (all new issue codes).
- `internal/storymaker/issues.go`: A only.
- `internal/storymaker/studio_check.go`: A (`StoryLayoutOptions`, classification of `flash_unverified`), B (one line that
  appends `ContrastIssues(summary.Stills)` right after `LookAlikeIssues(summary.Stills)`).
- `internal/storymaker/stills.go`: B only. `internal/storymaker/layout.go`: A only.
- `internal/storymaker/render/compilestills.go`: B only. `render/compilecheck.go`, `render/report.go`: C only.
- `render/mix.go`, `render/jobs.go`, `render/render.go`: E only.
- `internal/storymaker/envelope.go`: E only. `internal/storymaker/agentjob.go`: E only (A needs no change there).
- `internal/storymaker/studio_story_scenes.go`, `internal/storymaker/studio_packet.go`: D only.
- `ui/js/desktop/apps/storymaker-core.js` (`ISSUE_CODES`, the editor's titled codes) and its guard test
  `ui/desktop_storymaker_guards_test.go`: A only.
- `ui/lang/storymaker/*.json` (16 files): A adds the six `storymaker.issue.*` keys right after the
  `storymaker.issue.glyph_missing` line; E adds `storymaker.agent.narration_faults` right after the last existing
  `storymaker.agent.*` key. Each value is a real translation (the locale tests refuse a copy of the English text).
- `ui/js/storymaker/*.js`: C only. `ui/js/desktop/apps/storymaker-agent.js`: E only.
- Generated files (`ui/js/storymaker/player.bundle.js`, `host.bundle.js`, `ui/js/desktop/main.bundle.js` and any other
  bundle `npm run build:ui` writes) are rebuilt with `npm run build:ui`, committed with the task that changes their sources,
  and never edited by hand.

## Issue codes (A, `internal/storymaker/model.go`)

```go
CodeTextOffframe    = "text_offframe"    // hard: a text sticks out of the frame (layout)
CodeTextUnsafe      = "text_unsafe"      // advice: a text or the caption lies in the aspect's safe zone (layout)
CodeTextContrast    = "text_contrast"    // advice: contrast below 3:1 on the still (B)
CodeFlash           = "flash"            // hard: more than 3 flashes in a second (C, page probe)
CodeFlashUnverified = "flash_unverified" // open check: the probe did not reach every frame (C)
CodeSlowHook        = "slow_hook"        // advice: scene 1 is silent or lands after 0.5 s (C, player lint)
```

Classification (A, `issues.go`, `studio_check.go`):
- `IssueModelFixable` returns true for `CodeTextOffframe` and `CodeFlash`. They are not craft or layout codes, so
  `issueNeverBlocks` is false for them and `DecideOutcome` blocks on a left one (`incomplete`). Their severity is
  `SeverityWarning` everywhere, so `HasErrors` stays false and a person's export is never refused.
- `IssueAdvisory` gains `CodeTextContrast`, `CodeTextUnsafe`, `CodeSlowHook` (`CodeDeadStretch` and `CodeFirstFrameEmpty`
  are already there and cover the code-scene versions).
- `flash_unverified` goes to `CheckReport.Environment` in the classification switch of the check (an explicit case before
  the others). With severity warning it is listed by `DecideOutcome` and never blocks: `ready_unverified`.
- `IssueLayout` does not change: `text_offframe` and `text_unsafe` are NOT layout codes.

## Layout text checks (A, `layout.go`)

```go
// LayoutZone is the safe zone of an aspect: fractions of the frame measured from each edge.
type LayoutZone struct{ Top, Right, Bottom, Left float64 }

// SafeZone returns the zone of an aspect; any value other than "9:16" gets the 5 % margin.
func SafeZone(aspect string) LayoutZone
```

Values: `9:16` = `{Top: 0.115, Right: 0.130, Bottom: 0.219, Left: 0.056}`; every other aspect `{0.05, 0.05, 0.05, 0.05}`.
`LayoutOptions` gains `Aspect string`; `StoryLayoutOptions` sets it from `story.Format.Aspect`.
- Texts: share outside the frame `> layoutTextOutside` (0.01) gives `text_offframe` (it replaces today's `offscreen`
  finding for texts; actors and props keep `offscreen`). Message names the text id and the scene, the fix says to shorten
  the text or show it at "center".
- Texts and the caption box: share of the box area inside the zone `> layoutZoneShare` (new constant 0.02) gives
  `text_unsafe`, except for a text that already has `text_offframe`. The caption is checked in every aspect.
- Both carry `SeverityWarning`, `SceneID`, and a `Path` of `/scenes/<i>` like the other layout findings.

## Contrast (B)

```go
// TextContrast is the measured contrast of one text box on a still; ID is the text id or "caption".
type TextContrast struct {
	ID    string  `json:"id"`
	Ratio float64 `json:"ratio"`
}
// StillInfo gains: Texts []TextContrast

// MeasureStillContrast decodes one still (through stillConfig/decodeStill) and measures every text box and the caption
// box of its layout scene. Boxes are fractions of the frame, as in LayoutScene.
func MeasureStillContrast(data []byte, scene LayoutScene) ([]TextContrast, error)

// ContrastIssues turns the measured ratios into text_contrast warnings: 1.15 <= ratio < 3.
func ContrastIssues(stills []StillInfo) []Issue
```

New file `internal/storymaker/contrast.go`; constants `contrastMin = 3.0`, `contrastMasked = 1.15`,
`contrastMaxSamples = 20000`. Relative luminance from sRGB (linearise each channel, 0.2126 R + 0.7152 G + 0.0722 B);
ratio `(L97 + 0.05) / (L3 + 0.05)` over the 3rd and 97th percentiles of a sample grid inside the box. `captureStills`
(render) calls `MeasureStillContrast` for each still with the layout scene of the same scene id before `BuildStills`, and
puts the result into the matching `StillInfo.Texts`. A measuring error drops the ratios of that still only.

## Frame probe (C)

- Page side: after the layout run, `host-render.js` runs the probe within `PROBE_DEADLINE_MS = 20000`. Each frame is
  rendered at 64 px width (the player's `opts.width` path; code scenes from their own frames) and reduced to two numbers:
  mean relative luminance `y` and mean absolute difference per colour channel to the previous frame `d` (0..1). The first
  frame after the entry transition of every code scene also yields `uniform` (every pixel within 1 % of full scale of the
  mean).
- `checks.js` (`SM.checks`) applies the rules and returns findings. Exported pure functions, testable in node:
  `SM.checks.flashes(ySeries, fps)` returns the list of flash start frames; `SM.checks.run(probe, timeline)` returns the
  issues. The compile report's existing `checks` list carries them.
- Flash rule: a transition is a monotone run of `y` with total change `>= 0.10` whose darker end is `< 0.80`; a flash is two
  consecutive transitions in opposite directions; more than 3 flashes starting inside any window of `fps` frames gives one
  `flash` issue per scene (first window start as `at_seconds`).
- Code scenes: more than 3 s of frames with `d < 0.01` gives `dead_stretch`; `uniform` on the first frame after the entry
  transition gives `first_frame_empty`. Storyboard scenes keep their timeline lint for both codes.
- Frames the probe did not reach give one `flash_unverified` issue (scene of the first unreached frame, its time).
- Issue shape in the report, as everywhere: `{severity: "warning", scene_id, path: "/scenes/<i>", code, message, at_seconds,
  fix}`.
- Go side: `compileCheckReport` (render) gains `Checks []storymaker.Issue `json:"checks"``; `compileInBrowser` appends them
  to the summary's issues, so the Studio check sees them through `checkCompileIssues` unchanged.
- `slow_hook`: `lint.js` film lint, warning on scene 1 when its narration is empty or its first show beat lands after
  0.5 s; it reaches Go as a compile issue like `first_frame_empty`.
- 9:16 caption (`render.js` `captionLayout`): bottom at 0.78 of the height and maximum width from the zone (centre in the
  band 0.056 .. 0.870 of the width). The zone numbers are duplicated in JS with a comment naming `layout.go SafeZone`.

## Cue by phrase (D)

```go
// cuePhraseRefusal is a refused cue_phrase; Code is cue_phrase_not_found, cue_phrase_conflict or invalid_value.
type cuePhraseRefusal struct{ Code, Message, Next string }

// resolveCuePhrases rewrites fields in place: every beat with cue_phrase gets a {n} marker in fields["narration"] and
// cue: n, and loses cue_phrase. notes say when a phrase occurred more than once.
func resolveCuePhrases(fields map[string]any) (notes []string, refusal *cuePhraseRefusal)
```

New file `internal/storymaker/cuephrase.go`. `writeScene` and `dryRunScene` call it right after `repairSceneFields`
(dry run: if `dryRunScene` has no such call, right after its fields are built). A refusal returns
`sceneWriteRefused(op, sceneID, refusal.Code, refusal.Message, refusal.Next, nil)` and counts as a refused write. Notes are
appended to the answer's `next_action` text. Beat schema property `cue_phrase` (string, maxLength 60) in
`internal/agent/native_tools_storymaker.go`; the 900-token cap test must pass.

## Audio (E)

```go
// Envelope gains: Faults []string `json:"faults,omitempty"` with the values "clipped" and "dropout", sorted.

// CheckPiece reports the faults of one synthesized piece: "clipped", "dropout" (sorted, each at most once).
func CheckPiece(audio DecodedAudio, env Envelope) []string
```

New file `internal/storymaker/piececheck.go`. Render job warning codes in `render/jobs.go`:

```go
warnBedTooLoud       = "bed_too_loud"
warnBedGap           = "bed_gap"
warnMixClipped       = "mix_clipped"
warnNarrationDropout = "narration_dropout"
warnNarrationClipped = "narration_clipped"
```

`MixResult` gains `Warnings []string` (codes in the order bed_too_loud, bed_gap, mix_clipped, each at most once), measured
in a new `render/mixcheck.go` on the pass-3 stem blocks; `render.go` warns each like `warnMusicSilent`. Narration faults
reach the export as `narration_dropout` / `narration_clipped` (one warning each when any used piece envelope has the
fault) and the agent job card through the job record field `NarrationFaults []string `json:"narration_faults,omitempty"``
and the locale key `storymaker.agent.narration_faults`.

## Rules for every drafting agent

- Work only in your worktree and branch. Never touch the main checkout `C:\Users\Andi\Documents\repo\AuraGo`, never push,
  never merge, never rebase onto main.
- One commit per plan task, test first: write the test, run it and keep the failing line, implement, run it green, commit
  test and code together. Commit message:

  ```
  smc <Part><n>: <task title in plain words>

  Test: <exact command>
  Red: <the failing line before the implementation>
  Green: <the PASS line after>
  ```

  A task without a meaningful red state (docs, prompts, generated files) writes `Red: none (<why>)`.
- Tasks are small: one behaviour each, 3-10 tasks per part. Keep diffs minimal and in the style of the surrounding code
  (comments are full sentences, as in the existing files). Read the nearest `AGENTS.md` first
  (`internal/storymaker/AGENTS.md`, `internal/storymaker/render/AGENTS.md`, `ui/js/storymaker/AGENTS.md`).
- Never convert `eol=lf` files to CRLF (check `git ls-files --eol <file>`); write LF.
- Go: set `GOTMPDIR` to a short backslash path, e.g. `C:\tmp\go-build-smc-<part>` (create it); keep the default GOCACHE.
  Run targeted tests (`go test ./internal/storymaker/... -run '<pattern>' -count=1`), not the whole repo. Group patterns as
  `(?i)(a|b)` and count PASS lines.
- JS: `npm run build:ui` after source edits, `npm run check:ui`; node tests with `node --test <file>` or the existing
  `npm run test:storymaker` script. `node_modules` is a real copy in worktrees A, C and E; do not replace it with a junction.
- Browser tests run only with `AURAGO_RUN_BROWSER_SMOKE=1` and need the sandbox off; run at most one per part.
- Leftover processes: stop only the PIDs you started, never by image name.
- At the end report: the commit list (hash, subject), the test commands with their PASS counts, anything you could not
  make green and why, and every place where you had to deviate from this file.
