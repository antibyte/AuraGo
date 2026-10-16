# Publishing Decisions runtimes

The application does not manufacture release pins. The first public runtime
release and its generated catalog are mandatory before shipping one-click setup.
Publishing runtimes is separate from releasing or deploying AuraGo.

## Build and publication

1. Review the runtime source pin and patch, then push the reviewed workflow commit
   with explicit publication authorization. Dispatch `decisions-runtime.yml`
   with a new numeric version, initially with `publish=false` to exercise the
   complete matrix. Never overwrite a version.
2. The matrix uses native Windows, Linux and macOS runners for amd64 and arm64.
   It builds 15 native packages: CUDA/Vulkan/CPU on Windows amd64 and both Linux
   architectures, Vulkan/CPU on Windows arm64, and Metal/CPU on both macOS
   architectures. Linux arm64 CUDA is SBSA, not Jetson. Windows uses a static
   MSVC runtime; other required redistributables and license notices are bundled.
3. Each job runs the real loopback protocol self-test, device discovery, and
   those checks again from the packaged directory with SDK search paths removed.
   Empty accelerator discovery on a runner without a GPU is valid. These checks
   do not replace scoring/model or real GPU acceptance.
4. Dispatch with `publish=true` only after review. The workflow rejects an
   existing release, creates `decisions-runtime-v<version>` as a prerelease with
   `latest=false`, and publishes three multi-architecture GHCR images. The usual
   AuraGo cleanup scripts retain only `v[0-9]...` application releases and never
   delete Decisions releases.
5. The GHCR package `antibyte/aurago-decisions-runtime` must be public. A new
   private package requires its owner to change package visibility. Anonymous
   verification deliberately fails until public downloads/pulls work.
6. The workflow downloads every release archive anonymously and checks its size
   and digest. It inspects and pulls both architectures of each image using an
   empty Docker credential directory. Only then does it generate the
   `decisions-verified-catalog` artifact. Copy its
   `runtime_catalog_generated.go` into `internal/decisions/`, review the exact
   pins, and run the application tests before the application commit/release.

Partial publication is not a verified release. Missing or defective artifacts
require a new version; never replace an archive or reuse an image version tag.
If all artifacts were published and only verification failed, dispatch the same
workflow with `verify_existing=true` and `publish=false`. This read-only job
downloads the existing release, resolves its versioned image tags, and repeats
all anonymous checks before generating a catalog. It never builds, uploads or
replaces runtime packages or images. Pulls use each architecture's manifest
digest from the pinned index, avoiding classic Docker image-store conflicts.
The generated JSON catalog can be retained as release evidence. The generator's
`--write` option requires `--verify-published`; it cannot generate unchecked pins.

## System requirements and trust

The first matrix targets Windows 10/11 or Server 2022+, Linux glibc 2.39+
(Ubuntu 24.04 or compatible), and macOS 14+. Windows arm64 requires Windows 11.
Packages record these requirements. CUDA uses the pinned 12.9 toolkit on the
builder; end users need a compatible NVIDIA driver. Vulkan needs an installed
device driver, Metal a supported Apple device. No installer changes host drivers.

Native archives are flat ZIPs with a v2 manifest listing the executable,
libraries and license notices by exact size and SHA-256. The manifest also pins
both model files and the llama.cpp source and patch. Archives reject traversal,
symlinks, duplicate portable names, unlisted members and mismatched pins.
Docker uses the same verified native package and immutable image digests, keeps
the 8 GiB host-memory cap, and runs with the existing private transport and GPU
device policy. `BUILD=0` remains supported.

## Verification

```text
python scripts/decisions/test_release_catalog.py
go test ./internal/decisions ./internal/onnxrt ./internal/dockerutil ./internal/embeddings ./internal/persondetect
go test ./internal/server ./internal/config -run Decisions
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run Decisions
node scripts/build-ui-bundles.js --check
```

Follow the repository's Windows test wrapper instructions when applicable.
After publication, test setup from a missing runtime through a valid text probe
on the AMD/Vulkan test server. Confirm actual model, projector and KV placement,
not merely a visible render node. Separately qualify CUDA and Metal on hardware.
Exercise interrupted downloads, low disk space, Windows locks, cancellation,
saved consent revocation and server shutdown. Publication, deployment and GPU
qualification must each be recorded separately.
