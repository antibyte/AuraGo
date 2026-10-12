# ONNX Runtime layer

## Purpose

Shared ONNX Runtime layer of `internal/embeddings` and `internal/persondetect`: a CGO-free purego binding
of the ONNX Runtime C API, a verified cache for pinned runtime and model downloads, the pinned runtime
manifest, and the helpers that start native worker processes. Design: spec
`docs/superpowers/specs/2026-10-11-network-cameras-person-detection-design.md` and plans
`docs/superpowers/plans/2026-10-11-person-detection-*.md` (contract C1; local, git-ignored).

## Ownership

`internal/onnxrt` owns `Library`/`Runtime`/`Session` (`library_*.go`, `runtime.go`, `session.go`), the
`AssetCache` with archive extraction (`assets.go`, `wheel.go`), the pinned ORT assets (`runtimes.go`) and
`ConfigureWorkerCommand`/`AccessibleRenderNode` (`process*.go`). It must not import `internal/embeddings` or
`internal/persondetect`. The callers own model manifests, tokenizers, tensor layouts, worker protocols and
what a session result means (`internal/embeddings/ort.go` adapts Granite, `internal/persondetect/engine.go`
adapts D-FINE). The audit `NetworkClientInventory` lists `internal/onnxrt/` as the pinned HTTPS download client.

## Local Contracts

### Native code and the C API

- Native code runs only in worker subprocesses (`--embedding-worker`, `--person-detect-worker`), never in
  the server process, and only one ORT library per process: two builds export the same symbols and
  interpose. Tests that load a real runtime in-process say so and skip when another build is loaded.
- C API version 26 (`APIVersion`) is served by the ORT 1.26.0 CPU/CUDA bundles and the 1.27.0 WebGPU wheels.
  The `struct OrtApi` indices in `runtime.go` count members from 0 in `onnxruntime_c_api.h` of the pinned
  release; verify any new index there before adding it.
- `SessionOptionsAppendExecutionProvider` gets `"WebGPU"` for `ProviderWebGPU` and the canonical names for
  CUDA and CoreML, exactly what embeddings always passed (ORT 1.26's generic append does not register CUDA
  under either spelling, an existing embeddings limit). `ProviderCPU` is `""`; `"CPUExecutionProvider"` is
  treated as CPU and `ProviderOptions` need a non-CPU provider. `NewSession` fails with the available list
  when `Runtime.Providers()` lacks the requested provider.
- `FindLibrary` returns an absolute path below the runtime root (versioned names such as
  `libonnxruntime.so.1.27.0` match) and `Load` always opens that path: Windows ships its own
  `System32\onnxruntime.dll`, which the loader search path must never pick.
- `Runtime` shares one environment (warning log level) and CPU memory info with its sessions and counts open
  sessions. `Runtime.Close` fails and unloads nothing while a session is open, so close sessions before the
  runtime; closing a session of an already closed runtime is an error; closing twice is safe.
  `NewSession`/`Providers` may run concurrently but not with `Close`; `Session.Run` is concurrent-safe;
  `Session.Close` and `EndProfiling` must not overlap any other call on that session. A profile that was
  never ended is ended and deleted by `Close`.
- Session inputs are Go memory: each input is pinned with `runtime.Pinner` and stays reachable until every
  output is copied and released (an output may alias an input). Outputs are always copied into Go memory.
  `Tensor` holds float32 or int64 data; inputs need rank >= 1 and positive dimensions, outputs may be rank 0
  or zero-size. Strings with NUL bytes are rejected before any native call.
- `SessionOptions` zero values: `OptimizationAll` (ORT level 99) and ORT's default threads. A model saved with
  `OptimizedModelPath` at `OptimizationAll`/`OptimizationExtended` may bake in hardware-specific kernels;
  persist only `OptimizationBasic` results for use on other machines. `DisableSpinning` sets
  `session.intra_op.allow_spinning=0` (ORT's CPU spin-wait otherwise keeps every intra-op thread busy).

### Worker processes

- `ConfigureWorkerCommand(cmd, roots...)` replaces `cmd.Env` with `sandbox.FilterEnv(os.Environ())` (never
  nil: a nil `Env` would inherit everything, secrets included) and prepends every directory below the
  roots to `PATH` (plus `LD_LIBRARY_PATH` on Linux, `DYLD_LIBRARY_PATH` on macOS). Roots are made absolute;
  empty, missing or non-directory roots contribute nothing and without any directory the filtered
  environment stays unchanged, so the working directory never lands on a library search path. Callers add
  their own variables afterwards. On Windows it sets `HideWindow` and `CREATE_NO_WINDOW` on top of any
  `SysProcAttr` the caller set.
- `AccessibleRenderNode` error texts are part of the embeddings hardware fingerprint (`dri=unavailable:<error>`);
  changing them forces a re-benchmark on every Linux install.

### Asset cache

- Layout, shared by embeddings and person detection and stable on disk: `<root>/.download.lock`,
  `<root>/downloads/<id>{.zip,.tar.gz,.whl,.bin}` (`ArchivePath`), `<root>/runtimes/<id>/` with
  `.complete.json` (version 1). Both features use `<data>/embeddings` as the runtime root; embeddings'
  GPU cleanup removes only its own candidates' asset IDs (`TestCleanupUnusedGPURuntimesKeepsForeignRuntimes`).
- One process lock per root (`flock`, waits honour the context) serializes every download and extraction,
  across AuraGo processes and features; a long embeddings download therefore delays a detector start-up.
  `EnsureRuntime` re-hashes the cached archive each call.
- `ValidateAssetSpec` runs before any network access: ID matching `^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`
  (a safe single path component, no trailing dot), absolute HTTPS URL, size > 0, a 64-digit hex SHA-256,
  `Members` only for `AssetWheel`. `EnsureRuntime` accepts zip, tar.gz and wheel only.
- Downloads: HTTPS only, redirects included (at most 10 requests, i.e. 9 redirects); a known
  `Content-Length` must equal the pinned size (an unknown or chunked length is accepted), and the received
  bytes must equal it either way; the `.part` file is checked for size and SHA-256 and removed on any
  failure; a cached file that matches is reused without network access.
- Extraction goes to `<id>.tmp-<random>` and is renamed into place only when complete. The marker lists
  every file with size and SHA-256; a missing, changed, extra or symlinked file, a marker for another
  asset/size/SHA, or (wheels) a file list that differs from the flattened `Members` triggers a fresh
  extraction from the verified archive. Entries are validated against traversal and absolute names, zip
  symlinks are refused, tar symlinks/hardlinks are skipped and never materialized (`FindLibrary` finds the
  versioned file instead), modes become 0640 (0750 for executables).
- Wheels (zip): only the listed members are extracted, flattened to their base names; each must exist exactly
  once as a regular file. Member paths are clean forward-slash relative paths whose base names are portable
  (`^[A-Za-z0-9][A-Za-z0-9._+-]*[A-Za-z0-9]$`), unique ignoring case and never `.complete.json`.
- Pins live in `runtimes.go`: CPU/CUDA bundles of ORT `CPURuntimeVersion` (1.26.0) and the
  `onnxruntime-webgpu` wheels of `WebGPURuntimeVersion` (1.27.0) for linux/amd64, windows/amd64 and
  darwin/arm64 (`WebGPURuntimeAsset`). The Windows wheel also extracts `dxcompiler.dll` and `dxil.dll`;
  `onnxruntime_providers_shared` is listed but the WebGPU path does not load it (observed in the spike).
  Change a pin only together with size, SHA-256, members and a passing integration run. When a pin or its
  members change, update `THIRD_PARTY_NOTICES.md` and `THIRD_PARTY_LIBRARIES.md` (the audit test
  `TestPersonDetectionThirdPartyRecords` checks versions and members).

## Work Guidance

- Add no dependency on a feature package; keep behavior shared by both features here and covered by this
  package's tests.
- Embeddings output must stay byte-identical across changes here (`go test ./internal/embeddings/ -count=1`).

## Verification

- `go test ./internal/onnxrt/ -count=1`, `go vet` for windows/amd64, linux/amd64, linux/arm64 and
  darwin/arm64, `go test ./internal/embeddings/ -count=1`, and the notice and routing checks
  `go test ./internal/audit/ -run '(?i)(TestPersonDetection)' -count=1`.
- Opt-in real runtimes download from the pinned URLs and need `-timeout 30m`. Set
  `AURAGO_PERSONDETECT_INTEGRATION=1`; the cache root defaults to `disposable/person-detection/cache`
  (override with `AURAGO_PERSONDETECT_CACHE`, runtimes land in `<cache>/embeddings`). Two ORT builds must not
  meet in one process, so run the CPU bundle and the WebGPU wheel in separate invocations:
  `-run '^(TestCPURuntimeIntegration|TestCPURuntimeIntegrationOutputs)$'`, and with
  `AURAGO_PERSONDETECT_GPU=1` `-run '^TestWebGPURuntimeIntegration$'` (needs a usable GPU).
- The Linux GPU path is verified on aurago-test in a throw-away container; recipe in
  `internal/persondetect/AGENTS.md`.

## Child DOX Index

None.
