# Release checksum signatures

The Linux, PowerShell and batch release builders sign the completed `SHA256SUMS`
and verify that exact manifest before publishing. Stale signature files are
removed before signing. A failure stops before Git push or release creation.
Local `make_deploy.sh --no-publish` builds may remain unsigned.

## Automatic installation and updates

Official native installers and updaters verify signatures automatically. Users do
not install Cosign, distribute keys, configure environment variables or sign in.
The initial script is obtained over HTTPS from the official repository/release;
it is the installation's initial trust boundary. Application payloads are not run
to verify themselves.

`scripts/release-bootstrap.sh` is embedded identically into `install.sh`, `update.sh`
and `repair-update.sh` by `scripts/build-release-verifier.py`. It obtains a small
CGO-free verifier with its exact version, byte size and SHA-256 already pinned in
the trusted script. It checks those pins before every execution, including cache
hits. Files are downloaded with bounded output, into private temporary files and
installed atomically under `.aurago-update/tools`. A healthy cache works without
another tool download. Interrupted or corrupt downloads never replace a valid tool.

The native helper uses Go's ECDSA/SHA-256/PKIX implementation to verify the existing
detached Cosign signature against the reviewed public root in
`internal/releaseverify/release.pub`. No private signing material is distributed.
Missing, damaged or oversized signatures stop official installations and updates
before payload installation or service shutdown. `AURAGO_ALLOW_UNSIGNED_RELEASES`
does not bypass this consumer policy. Unsigned local development builds are separate.

Historical keyless releases use automatically provisioned, pinned Cosign v3.1.4,
retaining the GitHub Actions issuer and repository identity checks. The ordinary
keyed path does not download Cosign. An explicit `AURAGO_RELEASE_PUBLIC_KEY` remains
an operator/fork override; it never falls back to a certificate or the AuraGo root.
Unsupported custom key types are checked by pinned Cosign. Legacy unsigned forks
retain the previous optional policy unless strict verification or a key is set.

### Bootstrap publication and reproducibility

`python scripts/build-release-verifier.py --check` rebuilds Linux/macOS/Windows
amd64/arm64 helpers with pinned Go 1.27.2, CGO disabled, trimpath, no VCS stamping and
no build ID. A temporary stdlib-only module file keeps the bootstrap compiler
independent of application Go updates. Every binary must match
`scripts/release-verifier.json`, and every generated bootstrap block must match
its template. Both Windows builders and the
Linux builder include these checks, helpers and the repair script in release output.
Security Gates checks reproducibility on Linux and Windows and actual Cosign/native
interoperability with disposable keys.
Publishing application builders also require the public bootstrap's six GitHub
asset digests/sizes to match these pins (`--require-published`); unavailable helpers
stop publication before users receive an unusable installer.
They verify the finished application signature against consumer trust before
pushing or publishing; generating an unrelated maintainer key cannot silently
produce a release that automatic installers reject.

Publish the bootstrap component once by pushing its reviewed `release-verifier-v1`
tag (or dispatching `.github/workflows/release-verifier.yml`)
before publishing an application installer that references it. The workflow checks
all pins/tests, signs checksums, uploads to a draft and publishes a **prerelease**
without changing `latest`. Keep `release-verifier-v1` while any supported installer
references it. Never replace its bytes; changes require a new version/tag, reviewed
pins (`--pin`) and regenerated scripts. Application release retention excludes
these prereleases. `--scripts-only` checks generated source without compiling.

### Key rotation

Keep the original root protected. To authorize successor release keys, publish
`release-keyring.json` with `{"version":1,"keys":["PEM public key..."]}` and its
detached `release-keyring.json.sig`, signed by that root. Only after verifying this
proof may the helper use a successor key. Keyrings are bounded to 64 KiB/16 keys;
unknown fields, trailing JSON and self-signed transitions are rejected. A new
keyring must retain the keys needed by supported releases. Do not fetch and trust
a release's public key without this root signature. Replacing/revoking the root
itself requires an authenticated installer/bootstrap update.
Store the reviewed public keyring/proof in `internal/releaseverify/`; builders stage
and publish the pair and validate successor-signed manifests through the same helper.

## Existing installations

Once the new updater is installed, CLI, Web UI and agent updates need no crypto
setup. Configuration, master keys, model resources and service settings are preserved.
An already distributed updater that fails before refreshing itself cannot learn
the new bootstrap just because a release was published.

For those old copies, operators run `repair-update.sh --root /path/to/aurago` once
through their existing management access. Obtain this script over HTTPS from the
official current release/repository, as with the installer. It provisions its own
verifier, pins the current release, verifies its manifest and `update.sh`, checks
that the new updater includes automatic verification, then atomically replaces
only that script under the update lock. It saves the previous script as
`.aurago-update/update.sh.before-repair`. It does not stop/reinstall AuraGo or change
config, secrets or data. `--run` continues with the normal verified update after
releasing the repair lock. No end-user tool, key or environment setup is needed.
Concurrent repairs/updates and symlinked state or updater paths are rejected.

## Default Windows release publication

The Windows entrypoints dispatch the native GitHub Actions release workflow:

```powershell
.\make_release.bat v2026.10.30
# Or:
.\make_release.ps1 -Version v2026.10.30
```

Run from the reviewed `main` branch with Git and an authenticated GitHub CLI.
The dispatcher pushes committed HEAD, passes its exact SHA and waits for the
workflow's result. Uncommitted drafts remain local. If main advances between push
and dispatch, the workflow stops rather than releasing a different commit.

The workflow requires the complete Security Gates run for that source commit.
When the push also triggers Training Dataset Gates, that run must succeed too.
It checks out clean source, prepares the manifest-pinned Phonon resources and
builds all six native platforms through the existing PowerShell builder. Cosign
uses the short-lived GitHub Actions identity; no local private key, DPAPI password
or signing secret is required. The finished checksum manifest is verified against
the same repository identity and issuer used by installers before the completed
draft becomes public. Existing bootstrap pins and the embedded key stay unchanged.

CI preserves existing releases and tags. Docker publication is dispatched explicitly
for the published tag because events created by `GITHUB_TOKEN` do not trigger tag
push workflows. Docker publication continues in its own workflow.

`make_release.ps1 -Local` or an explicit `AURAGO_RELEASE_SIGNING_KEY` selects the
existing local build path instead. Local builds need the Go, Node.js, Python, tar and Cosign prerequisites
and a clean source checkout. The default dispatcher needs none of those build tools.

## Optional local Windows maintainer setup

For local keyed signing, run once under the Windows account used to sign:

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

Local profile signing (`make_release.ps1 -Local`) automatically discovers
`cosign.exe`, `release.key`, `release.pub` and `password.dpapi` in that profile.
Local builds check readiness before
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
Review/embed the public root before publishing installers; the automatic consumer
path distributes that trust without operator environment setup. Maintainer setup
does not change remote hosts. The public key is safe to share. Never share `release.key` or
`password.dpapi` with a release.

## Manual keys and verification

For an offline, operator-managed signing key, install Cosign independently and
set `AURAGO_RELEASE_SIGNING_KEY` to its private-key path and
`AURAGO_RELEASE_PUBLIC_KEY` to the corresponding trusted public-key path.
Provide its password using Cosign's normal `COSIGN_PASSWORD` mechanism. Keep
private keys and passwords outside the repository. Both builders use keyed
Cosign signing without transparency-log upload, then verify using the supplied
public key. GitHub Actions also supports the existing keyless OIDC path.

Consumers normally use the embedded AuraGo root. Fork operators may explicitly
configure `AURAGO_RELEASE_PUBLIC_KEY` from a separately trusted source. Keyless
releases still require certificate/issuer verification. Builders require explicit
`AURAGO_ALLOW_UNSIGNED_RELEASES=1` to publish legacy unsigned output; official
consumer scripts deliberately refuse it.

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
