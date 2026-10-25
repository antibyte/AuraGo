# Release checksum signatures

The Linux, PowerShell and batch release builders sign the completed `SHA256SUMS`
and verify that exact manifest before publishing. Stale signature files are
removed before signing. A failure stops before Git push or release creation.
Local `make_deploy.sh --no-publish` builds may remain unsigned.

## Windows maintainer setup

Run once under the Windows account used to build releases:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup-release-signing.ps1
```

The setup verifies Cosign v3.1.4 Windows amd64 against its pinned release size
and SHA-256, generates a password-encrypted signing key, and protects the
random password with Windows DPAPI. The completed profile is installed atomically
under `%LOCALAPPDATA%\AuraGo\release-signing` only after real signing and
verification succeed. Its ACL grants access to the current user and SYSTEM.
Private keys, passwords and the maintainer tool never enter the checkout or
release assets. Repeating setup retains existing keys; incomplete profiles fail
without replacement or rotation.

Both Windows builders automatically discover `cosign.exe`, `release.key`,
`release.pub` and `password.dpapi` in that profile. They check readiness before
cleanup and compilation, including in an already-open terminal with no Cosign on
`PATH`. The signer only unlocks the password for signing and restores the caller's
environment afterward. Explicit key/password environment settings retain priority;
GitHub Actions keeps its existing OIDC behavior.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/sign-release-checksums.ps1 -CheckPrerequisites
```

Back up the encrypted private key and retain a separately protected recovery copy
of its password before migrating or reinstalling Windows. DPAPI data is bound to
the Windows user; copying `password.dpapi` alone to another account is insufficient.
Distribute `release.pub` to consumers through a separately trusted channel and
configure their `AURAGO_RELEASE_PUBLIC_KEY`; setup does not change remote hosts.
The public key itself is safe to share. Never share `release.key` or
`password.dpapi` with a release.

## Manual keys and verification

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
verified Cosign 3.1.4 Windows binary passes both the PowerShell fixture and the
shell signer under Git Bash, including tamper rejection. The Windows profile
fixture also covers discovery without a PATH entry, environment cleanup,
explicit overrides, invalid DPAPI data, incomplete profiles and repeated setup.
Signers probe the compatibility flag directly because Cosign v3.1.4 hides it
from help output. Signers explicitly
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
