# Update artifact upkeep

## Purpose

`internal/upkeep` owns installation-scoped preview, guarded deletion, transaction
recovery and the `--update-maintenance` CLI used by `update.sh`.

## Ownership

Keep the current executable and resource set plus two verified rollback versions.
The updater owns transaction manifests; this package verifies them before collection.

## Local Contracts

- Do not execute archived binaries to identify their web-asset pins. Read Go build
  metadata when available. For `-trimpath` binaries, match linked 64-character
  IDs only against installed resource-set directory names; reject ambiguous
  matches before any deletion. Keep explicitly retained binaries with no
  installed resource-set match without blocking cleanup of verified update
  backups; still reject unreadable binaries.
- Preview is the default. Applying cleanup requires locks, verified resources,
  resolved transactions and running-version readiness. Keep unknown, damaged or
  unreadable artifacts rather than guessing their provenance.
- Keep cleanup results sanitized in `data/update_cleanup_status.json`; private
  backups may contain credentials.
- `update.sh` shows a compact cleanup summary, including removed/kept counts
  and freed space. Keep the maintenance CLI's default JSON inventory for
  diagnostics; `--summary` must preserve failure exit codes and visible errors.
  Older rollback binaries without `--summary` get a generic success line.
- Source builds with Go do not resolve the AuraGo application release. Optional
  Phonon provisioning uses the source-pinned resource package independently.
  Reject missing/invalid current-binary resource pins before shutdown and explain that
  recovery requires rebuilding with generated asset flags and `--check-assets`;
  packaging alone cannot repair an unpinned executable. Never guess an asset set.
  Release installs pin the tag and checksum manifest before shutdown and reuse
  that pair throughout the update. Retry transient downloads at most three
  times; retain mandatory checksums and optional/strict signature policy.
  Required artifact failures after shutdown use `abort_update`, preserving
  rollback and unresolved-transaction safeguards.
- Supplied release signatures are never optional: invalid or incomplete
  signature metadata and a missing Cosign verifier fail closed. Keyed offline
  verification trusts only the operator's `AURAGO_RELEASE_PUBLIC_KEY`, never a
  key fetched with the release. Builders sign and verify the finished checksum
  manifest before publication; explicit legacy unsigned publication uses
  `AURAGO_ALLOW_UNSIGNED_RELEASES=1`. Keep the current unsigned-install default
  until the Linux/Windows signing fixture and trusted-key distribution are
  confirmed. See `documentation/release-signatures.md`.
- Windows maintainer setup pins Cosign and atomically creates the user-only
  `%LOCALAPPDATA%\AuraGo\release-signing` profile with an encrypted private key
  and DPAPI-protected password. Preserve existing keys. Both Windows builders
  preflight signing before cleanup/build; explicit environment configuration and
  CI retain precedence. Restore the caller's environment after profile signing.
  Operator trust on target hosts remains explicit; never fetch a release's own key.
- Publish Windows release drafts only after every asset upload succeeds. A failed
  upload or publication must stop before retention deletes anything. Retain three
  stable AuraGo releases without counting drafts or integration prereleases.
  `AURAGO_KEEP_RELEASES=1` skips retention and preserves every existing release/tag.

## Work Guidance

Keep `update.sh`, the maintenance CLI and the release build flags aligned.

## Verification

Run `go test ./internal/upkeep`, `go test ./internal/audit -run 'Update|ReleaseSignatureDownloads'`
and `bash -n update.sh` after relevant changes.

## Child DOX Index

None.
