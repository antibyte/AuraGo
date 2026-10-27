# Game Maker validation and repairable checks

## Purpose

Validation plan disclosure, repairable checks, tool validation evidence and the
browser evidence boundary.

## Ownership

This block belongs to the Game Maker contract. The Studio contract, the shared
game lifecycle and the sprite/model library stay in `internal/gamemaker/AGENTS.md`.

## Local Contracts

- `validationScenarios` in `validation_plan.go` is the single source of the
  scenarios a build drives. `ValidationPlan` and `BaseChecks` disclose exactly
  those checks; never describe a check the build does not execute. Planning
  context and `inspect` carry `base_checks`; building and repair carry
  `validation_plan`. Both are untrusted context after the prompt prefix and
  never change a profile fingerprint.
- `CheckResult.Repairable` and `BuildResult.Repairable` mark an `unavailable`
  check whose cause lies in the game: an absent or unreachable target, an
  inactive or unsupported control, or sampled state without the expected
  consequence. Such a result enters the bounded repair loop and shares its
  budget with failed checks. Missing browser feedback, an absent measurement or
  an unavailable runtime stays non-repairable and ends validation as before.
  Repairable never turns a check into `passed` and never permits publication.
- Diagnostic texts explain, they never change a status or gate. `CheckResult.Expected`
  prints no value for `increased`/`decreased`/`changed` (they compare against the
  before value, so a trailing `0` read like a threshold). Each `target=<role> <reason>`
  token keeps its raw reason code and gains a plain-language note after the
  closing parenthesis (`targetReasonNote`); the blind-input sentence is likewise
  extended, not replaced. `boundedPreviewDiagnostics` adds an `Asset handle:` hint
  to `reading 'released'` runtime errors (undefined `loadAsset` result passed to
  `createInstance`/`createInstances`). In Studio runs a missing `read`/`search`
  file also lists up to twelve existing `src/` text files (3D helpers are not
  named `common.ts`).
- A target run stopped because the Studio window was hidden or in the background (`document.hidden`
  or the parent's `aurago:game:active` flag) reports `host_hidden`: `unavailable`, never repairable,
  so it spends no repair pass and the job ends with code `preview`. `inactive` stays the game's
  own pause/end state (`view.active`) and remains repairable.
- `ValidationNextAction` names the next step for every validation answer and
  must never claim success for unobserved gameplay. Tool answers also return
  the remaining repair passes.
- Invalid scopes/check IDs and unsupported free-code 3D gameplay requests
  return a `request` diagnostic with correction guidance. They preserve the
  current preview and last validation, consume no repair pass and do not end
  the agent round. Check IDs are resolved before building. Free-code `three`
  needs valid 3D scene data for gameplay; declared scenarios alone do not enable it.
- `game_maker_file` `search` without `path` scans `game.json` and `src/**` text
  only: at most 64 files of 512 KiB each and twelve matches in total. Managed
  runtimes, compiled output, assets and `.aurago` stay excluded. `path` is
  therefore optional in every phase schema; other operations still reject a
  missing path.
- File errors are `*fs.PathError` values carrying the project-relative path, so
  `os.IsNotExist` and `errors.Is` keep working. `RedactHostPaths` is the last
  boundary before text reaches a model, a client or the job ledger; route new
  error paths through it or through `fileError`.
- Phase context includes `budget`: tool calls, remaining seconds rounded to
  30 s and phase guidance. Keep it coarse; exact values defeat prompt reuse.

- Game Maker `BuildJob` compiles without waiting; `ValidateJob` additionally
  requires a build-bound browser startup check through the authenticated Studio
  parent. Runtime errors feed the bounded repair loop; missing feedback prevents
  publication. Diagnostics stay bounded, untrusted data and never become trusted
  prompt instructions. Only the server boot's visible-canvas report establishes
  readiness; engine console errors also fail validation. Procedural SVG assets
  return `.svg` paths, never SVG bytes mislabeled as PNG. A passed startup check
  does not certify all gameplay.

- Browser gameplay evidence is a cooperative quality check, not independent
  attestation of hostile game code: the driver and project share one JavaScript
  realm. Preserve sandbox/API isolation; do not claim that window/channel checks
  authenticate observations independently of the generated game.

## Work Guidance

Keep these rules binding for every Game Maker change in this area. A rule that
applies to the whole Studio belongs in the parent contract.

## Verification

`TestValidationPlanMatchesExecutedChecks`, `TestBaseChecksNameTargetsPerBase`,
`TestValidationNextActionNeverClaimsUnobservedSuccess`, `TestRepairableCheckClassification`
and `TestMissingTargetStartsBoundedRepair`.

## Child DOX Index

None.
