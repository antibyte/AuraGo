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
  times; retain mandatory checksums and fail-closed official release signatures.
  Required artifact failures after shutdown use `abort_update`, preserving
  rollback and unresolved-transaction safeguards.
- Official releases require signatures without user setup. `internal/releaseverify`
  owns the embedded root and offline keyed verifier. The generated bootstrap
  provisions a size/SHA-256-pinned helper into the private installation's
  `.aurago-update/tools` cache; validate every cache hit before execution. Keyless
  and custom-key fallback provisions pinned Cosign with existing issuer/identity
  checks. Invalid, incomplete or oversized metadata fails closed; an explicit
  operator key never falls back to the embedded root or a release certificate.
  Legacy unsigned fork releases retain their existing optional/strict policy.
  Builders still sign and verify the finished manifest before publication;
  `AURAGO_ALLOW_UNSIGNED_RELEASES=1` only controls explicit legacy publication.
  See `internal/releaseverify/AGENTS.md` and `documentation/release-signatures.md`.
- `repair-update.sh` migrates blocked old script copies under the existing update
  lock. Verify the manifest and updater before atomic replacement; preserve config,
  master keys, binaries and running services. Save the old script privately. An
  optional `--run` releases the repair lock before invoking the repaired updater.
  Old binaries cannot discover this migration retroactively; operators deploy it
  through existing management access, without end-user crypto setup.
- Windows entrypoints default to `scripts/start-release-ci.ps1`: publish committed
  `main`, dispatch `release.yml` with the expected SHA and wait for its exit status.
  Local drafts remain outside the build. CI checks out the exact source commit,
  requires its complete Security Gates run, builds all native assets and signs
  with GitHub OIDC. Keep local profiles and the embedded trust root unchanged.
  CI retains all releases/tags and explicitly dispatches Docker publication after
  the native draft is fully uploaded, verified and published.
- Optional local Windows maintainer setup pins Cosign and atomically creates the
  user-only
  `%LOCALAPPDATA%\AuraGo\release-signing` profile with an encrypted private key
  and DPAPI-protected password. Preserve existing keys. `make_release.ps1 -Local`
  or explicit key configuration retains local builds. Both actual Windows builders
  preflight signing before cleanup/build; explicit environment configuration and
  CI retain precedence. Restore the caller's environment after profile signing.
  End-user trust comes from the reviewed embedded public root; never trust a
  release's self-declared key. Root-signed keyrings authorize successor release keys.
- Publish Windows release drafts only after every asset upload succeeds. A failed
  upload or publication must stop before retention deletes anything. Retain three
  stable AuraGo releases without counting drafts or integration prereleases.
  `AURAGO_KEEP_RELEASES=1` skips retention and preserves every existing release/tag.

## Work Guidance

Keep `update.sh`, the maintenance CLI and the release build flags aligned.
Already-current updates may repair the service user's Chromium AppArmor profile;
they report that AuraGo was not rebuilt or restarted, rather than claiming no files changed.
Both Windows builders must reject dirty source trees and link the reviewed commit
and generated UI pins into main binaries; pass linker flags as one native argument.
The local dispatcher pushes reviewed HEAD even when already committed. CI never
pushes an older checkout back to main. The PowerShell publisher pins the release
target and passes a flat asset list plus a BOM-free notes file.

## Verification

Run `go test ./internal/upkeep ./internal/releaseverify`,
`go test ./internal/audit -run 'Update|ReleaseSignature|ReleaseBootstrap|ReleaseVerifier'`
and `bash -n update.sh install.sh repair-update.sh` after relevant changes.

## Child DOX Index

None.
