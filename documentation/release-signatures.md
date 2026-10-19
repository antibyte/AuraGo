# Release checksum signatures

The Linux, PowerShell and batch release builders sign the completed `SHA256SUMS`
and verify that exact manifest before publishing. Stale signature files are
removed before signing. A failure stops before Git push or release creation.
Local `make_deploy.sh --no-publish` builds may remain unsigned.

For an offline, operator-managed signing key, install Cosign independently and
set `AURAGO_RELEASE_SIGNING_KEY` to its private-key path and
`AURAGO_RELEASE_PUBLIC_KEY` to the corresponding trusted public-key path.
Provide its password using Cosign's normal `COSIGN_PASSWORD` mechanism. Keep
private keys and passwords outside the repository. Both builders use keyed
Cosign signing without transparency-log upload, then verify using the supplied
public key. GitHub Actions also supports the existing keyless OIDC path.

Consumers using the keyed path must configure `AURAGO_RELEASE_PUBLIC_KEY` from
a separately trusted source before running `install.sh` or `update.sh`. Never
trust a public key downloaded alongside an untrusted release. Keyless releases
continue to require their GitHub certificate and issuer verification.

An existing invalid signature, incomplete signature/certificate pair, or a
missing verifier for a signed release is always fatal. The legacy
`AURAGO_STRICT_RELEASE_VERIFY=1` setting also rejects absent signatures. A
configured trusted public key also makes missing signatures fatal. The
existing installer/updater default remains unchanged until the real Linux and
Windows signing fixture has passed in CI and the operator has distributed the
trusted key. Builders require explicit `AURAGO_ALLOW_UNSIGNED_RELEASES=1` to
publish a legacy unsigned release. That switch never accepts an invalid
signature.

Run `pwsh -NoProfile -File scripts/test-release-signing.ps1` with Cosign installed
to exercise actual cryptographic signing, verification and tamper rejection.
It creates and removes its own disposable fixture key; it never contacts a
model provider, publishes artifacts or changes operator keys. Security Gates
runs it on Linux and Windows. This CI proof and real signed release acceptance
are separate from local script/fixture validation. Local acceptance with the
verified Cosign 3.1.3 Windows binary passes both the PowerShell fixture and the
shell signer under Git Bash, including tamper rejection. Signers explicitly
select the separate signature format consumed by existing installers; a future
bundle migration must update signers and consumers together.

Security Gates also runs complete `go vet ./...`, agent race tests, Windows
replacement/cancellation contracts, and deterministic UI bundle drift checks.
The small offline behavior fixtures live in
`internal/agent/runtime_boundaries_test.go` and
`internal/agent/codequality_batch_test.go`: whole-answer fallback, truncated
completion, cancellation, unknown side effects, approval boundaries and
declared-call/result ordering. They use scripted responses and streams; no
provider credentials are required. Real provider smoke tests are a separate,
explicitly manual operator action and must never run implicitly in these jobs.
Volatile upstream catalog refresh checks remain in the separate scheduled or
manual, nonblocking `model-catalog-drift.yml` workflow.

The Cosign flags follow the upstream [blob signing documentation](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/)
and [verification command reference](https://github.com/sigstore/cosign/blob/main/doc/cosign_verify-blob.md).
