# Storymaker package contract

## Purpose

Go core of the Storymaker desktop app (explainer videos from a story document): the `storymaker/1` story model, strict
decoding, validation, defaults, the catalog index, code-scene bundling and the project storage layout. No server
dependency. Spec: `docs/superpowers/specs/2026-11-01-storymaker-design.md`. The player that renders the story is
JavaScript (`ui/js/storymaker/`); Go never recomputes the timeline.

## Ownership

- `model.go` types and JSON (polymorphic `Position` and `Duration` keep an unreadable value as `Invalid` so Validate reports
  it at the right path); `decode.go` `DecodeStory` and the field hints; `validate.go` + `issues.go` `Validate`,
  `CapIssues`; `defaults.go` `ApplyDefaults`; `catalog.go` the `storymaker-catalog/1` index; `codebundle.go`
  `BundleCodeScene`; `project.go` + `memstore.go` + `slug.go` storage, `MemStore` (test helper), `Slugify`.
- `testdata/stories/*.json`, `testdata/scenes/*` (with the golden `s3.bundle.js`) and `testdata/catalog-mini/catalog.json`
  are shared fixtures: the player tests read the same files. `catalog-mini` has only the manifest; the files it
  references belong to the player part.
- The render job, listener and browser live in `render/`; HTTP handlers in `internal/server/storymaker_*.go`. Config is
  `internal/config/storymaker.go`.

## Local Contracts

- Issue shape `{severity, scene_id, path, code, message, at_seconds, fix}`; `path` is a JSON Pointer. `error` blocks a
  render, `warning` does not, `minor` is `not_supported_yet` and `canonicalised`. Codes are the constants in `model.go`;
  the app localises them as `storymaker.issue.<code>`. Model-facing answers carry at most `MaxModelIssues` (`CapIssues`).
- `DecodeStory` fails with a `*DecodeError` that always carries issues. Unknown fields are reported with the path, a
  hint for common mistakes and the allowed fields; key matching is exact. Wrong types become `invalid_json` with the path.
- `Validate` canonicalises prefixed names in place (`pose.typing` becomes `typing`, with a `canonicalised` note), knows the
  closed pose/gesture/expression/transition/ease sets, reports names the catalog lacks as `not_supported_yet` and unknown
  names as `unknown_name` with at most 5 alternatives. A nil catalog skips asset, spot and catalog-name checks.
- The phase 1 support matrix lives in `model.go` (`phase1Beats`, `phase1Transitions`) and in `validate.go`; move a beat or
  transition out of "not supported yet" there when the player gains it.
- `ApplyDefaults` fills only the static defaults of the spec and the show-beat instance id; timing (`delay`, `duration`,
  `ease`) is never defaulted: the player's compile step owns it. It is idempotent.
- `LoadCatalog` checks the manifest only; `VerifyFiles` checks that referenced files exist. The catalog is immutable after
  loading. Search is term scoring over id, name and tags (default 6 results, cap 12).
- `BundleCodeScene`: esbuild `Transform` (IIFE, global `__smSceneModule`, ES2020, no legal comments) plus the appended
  `self.__smScene=__smSceneModule.default;`. The pre-check is a tokenizer over the author's source and is defence in depth
  only; the CSP and the worker realm are the barriers. A slash after `)`, `}`, `++` or `--` is read both ways. Positions
  are 1-based and count characters. The golden bundle is reproduced byte for byte by Node esbuild 0.28.2 in the player tests.
- `Store` is the only file access of `ProjectStore`. `Remove` behaves like `os.RemoveAll`; etags are
  `desktop.NoteVersion` (`ETag`); a failed precondition is `ErrConflict`. Project ids are lowercase UUID v4, scene ids slugs;
  every path is built from validated ids only.
- `SaveCodeScene` checks the precondition itself, removes the old bundle, and only then writes the source and the new
  bundle. A save that fails half way therefore leaves the old source with its own bundle or the new source without one, never a
  bundle built from older source: a render loads the bundle of every scene whose status is ok.
- Slugs for file names: ASCII letters, digits and dashes, umlauts transliterated, Windows device names refused, 80 characters.

## Work Guidance

- Keep one responsibility per file. New issue codes or fields need the model, the validator, the hint text
  and the app's `storymaker.issue.<code>` locale keys in step.
- Do not edit the `catalog-mini` fixture or the story fixtures without updating the player's Node tests that read them.
- Regenerate the golden bundle only for an intended esbuild change: `STORYMAKER_UPDATE_GOLDEN=1 go test ./internal/storymaker -run TestBundleGolden`.

## Verification

- `go test ./internal/storymaker/... -count=1` and `go test ./internal/config/... -count=1`.

## Render and server (part B)

- `render/` turns a story into an MP4 (job manager, listener, ingest, browser, egress guard, FFmpeg); its contract is `render/AGENTS.md`. The package has no dependency on `internal/server`; the handlers are `internal/server/storymaker_*.go`.
- The server implements `Store` and `render.Publisher` over `*desktop.Service` (`storymakerStore`), passes `Limits.MaxDurationSeconds = storymaker.render.max_duration_seconds` to the validator and gives every request its own `Story` copy (`Validate` and `ApplyDefaults` mutate).
- Catalog version, canvas sizes (1920x1080, 1080x1920, 1080x1080), the support matrix and the error codes of a job (`invalid_story`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `busy`, `code_error`, `interrupted`, `storymaker_disabled`) are shared with the app through `GET /status` and the job document.

## Child DOX Index

- `render/AGENTS.md`: the render pipeline contract (job manager, listener, ingest, browser, egress guard, FFmpeg).
