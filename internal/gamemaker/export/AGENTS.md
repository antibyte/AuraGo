# Game Maker export, revisions and receipts

## Purpose

Revision blobs, publication fingerprints, filesystem receipts and ZIP export.

## Ownership

This block belongs to the Game Maker contract. The Studio contract, the shared
game lifecycle and the sprite/model library stay in `internal/gamemaker/AGENTS.md`.

## Local Contracts

- Revision blobs are SHA-256 addressed and deduplicated. Restore creates a new revision; export excludes tokens, staging, revision metadata, and AuraGo state while including source, output, local runtimes, assets, and third-party notices.
- Revision listings compare file hashes with each revision's parent, include
  added/changed/deleted project-relative paths in sorted order (at most 40),
  and expose the full changed count plus a truncation flag.
- Publication holds the source/build locks and rechecks the validation fingerprint
  over source, assets, runtimes, output and plan before creating a revision.
  Generated asset writes share the source lock. Changed or missing fingerprints
  reject publication while preserving the working copy and playable revision.
  A failed revision commit must retain staged blob work for retry or explicit cleanup;
  rollback removes only records and blobs created by that failed attempt.
- Publication and deletion write a private filesystem receipt before moving
  project directories. On restart, recover only when the project revision and
  expected files establish the outcome; retain ambiguous receipts for review.
  Incomplete temporary receipt writes may be removed without changing projects.
- ZIP export reads one published revision from the blob store, verifies sizes/hashes and required entry files, and retains that revision's runtime files. Never export mutable workspace edits alongside old compiled output. Finish a temporary archive before committing HTTP download headers; export failures must not become successful partial ZIPs. Include standalone HTTP-server instructions; file:// is not a supported launch path. Check extracted 2D/3D games without the preview boot/driver, including subdirectory hosting, imported assets and audio. `TestNightlyExportBrowser` does that for one unmodified platformer export and one exploration export that uses the guided default catalog models, with no LLM, when `GAMEMAKER_NIGHTLY_BROWSER=1`. The larger `GAMEMAKER_*_BROWSER` suites stay opt-in.

## Work Guidance

Keep these rules binding for every Game Maker change in this area. A rule that
applies to the whole Studio belongs in the parent contract.

## Verification

`TestNightlyExportBrowser` (`GAMEMAKER_NIGHTLY_BROWSER=1`), the revision and
publication fingerprint tests, and `TestFilesystemReceiptPublicationAcceptsRelativeWorkspace`.

## Child DOX Index

None.
