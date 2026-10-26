# Native release verification

## Purpose

Verify keyed Cosign checksum manifests offline without an end-user Cosign install.

## Ownership

This package and `cmd/release-verifier` own bounded verification and the embedded
public trust root. `scripts/build-release-verifier.py` owns reproducible bootstrap
artifacts, their pins and generated installer/updater blocks. Update transactions
and signature policy remain owned by `internal/upkeep/AGENTS.md`.

## Local Contracts

- Use standard-library ECDSA/SHA-256/PKIX verification, never custom cryptography.
- Never obtain a trust root from the release being verified. Operator key overrides
  remain explicit. Successor keyrings require the embedded root's detached signature.
- Bootstrap executables must match the size and SHA-256 embedded in trusted scripts
  before execution, including cache hits. Publish immutable bootstrap components as
  prereleases so application release retention never removes them.
- A repeated publication succeeds only after every public bootstrap asset matches
  its reviewed size/digest; never overwrite an existing bootstrap or accept drafts.
- Native helpers cannot verify Fulcio certificates; keyless releases retain Cosign's
  identity/issuer verification through an automatically provisioned, pinned binary.
- Never bundle private keys, passwords or maintainer signing profiles.

## Work Guidance

Regenerate bootstrap blocks through their generator; do not hand-edit generated pins.
Keep bootstrap source, compiler and build flags stable for reproducibility.

## Verification

`go test ./internal/releaseverify ./cmd/release-verifier` and
`go test ./internal/audit -run 'ReleaseSignature|ReleaseBootstrap|ReleaseVerifier'`.
`python scripts/build-release-verifier.py --check` rebuilds and checks all six targets.

## Child DOX Index

None.
