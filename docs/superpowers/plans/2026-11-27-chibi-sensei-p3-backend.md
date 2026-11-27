# Chibi Sensei P3 (test and compare): backend plan (B1 to B4)

Index with conventions, the coordinator contract and its amendments: `2026-11-27-chibi-sensei-p3.md`. Where this file and the amendments differ, the amendments win.

Chibi Sensei P3 (test and compare), backend half: batches B1 to B4. The coordinator contract `C:\disposable\chibi-sensei\p3\p3-contract.md` and the conventions of the P2 index apply. Those conventions are: work only in the worktree, implementers do not commit, Git Bash from the worktree root, `go build ./...` green after every batch, `-run` patterns grouped as `(?i)(a|b)`, no `-race`, real UTF-8, scratch files in `C:\disposable\chibi-sensei\p3\`, and the report format (files created and modified, commands run with their PASS/FAIL lines, deviations, open items). Every line number below was checked in the worktree on 2026-11-27.

## Contract changes

1. **`internal/llamacpp` additions.** Every contract signature works as written. These are added:
   - `func DetectHardwareWith(ctx context.Context, augment func(context.Context, *HardwareInfo) string) HardwareInfo`. Embeddings must keep its Docker probe (`augmentDockerHardware`, `llama_docker.go:281`) inside the hardware fingerprint at the same position. The fingerprint is byte-identical only when the extra evidence line comes before the `os=`/`arch=` lines. `DetectHardware(ctx)` is `DetectHardwareWith(ctx, nil)`.
   - `func Runtimes() map[string]AssetSpec` (a copy of the manifest map). `embeddings/manifest_test.go:19` ranges over `runtimeManifest` as a map.
   - `const DockerCPUImage, DockerCUDAImage, DockerVulkanImage` and `const WindowsCUDARuntimeID = "llama-windows-amd64-cuda-runtime"`.
2. **`chatrt` additions** (additive; contract names unchanged):
   - `ErrStopped`, `ErrClosed`;
   - `type DownloadStatus struct{ Bytes, DoneBytes int64; Host, ErrorCode string }`;
   - `const MaxInstances = 2`;
   - `func (m *Manager) BeginInstall(ctx) (<-chan struct{}, error)`. The 202 install route needs the state `installing` registered before it answers. `Install` is `BeginInstall` plus the wait.
   - `func (m *Manager) Admit(keys ...string) error`, the pre-stream check that gives the 409/503 answers;
   - `func (i *Instance) Backend() string`;
   - the shapes of `Launcher`, `Process` and `LaunchSpec` (B2 below);
   - a test helper package `internal/chibisensei/chatrt/chatrttest` (exported fake launcher used by the server tests and the browser smoke).
3. **B2 also edits `internal/audit/audit_manifests.go`.** It adds one row to `NetworkClientInventory` (`:236`) for `internal/chibisensei/chatrt/`. `TestNetworkClientInventoryCoversProductionHTTPClients` (`audit_manifests_test.go:164`) fails for any production file with `&http.Client{` outside a listed prefix. No batch owns `internal/audit`.
4. **B4 also edits three licence records.** It changes the usage column of the llama.cpp rows in `licences/THIRD_PARTY_LIBRARIES.md:362`, `licences/THIRD_PARTY_NOTICES.md:191` and `licences/UEBERSICHT.md:239`, adding the Chibi Sensei test chat. B4 also edits `internal/onnxrt/AGENTS.md`, which is already covered by "AGENTS.md files under `internal/`".
5. **`internal/llamacpp/AGENTS.md` is written in B4, not B1.** The contract puts all DOX in B4.
6. **B3 exports four extra names:**
   - `ErrRunNotSucceeded`, `ErrSamplesMissing`;
   - `const SamplePageMax = 50`;
   - `func RunTask(run Run) (recipe, taskKind string, labels []string, err error)`, which reads the run's `job.json` facts so the server can call `ReadSamples`.
7. **`POST /runs/{id}/report/seen` uses the read-class desktop operation**, so it also works on a read-only desktop. The flag is UI state, and D10 keeps the report card usable when the desktop is read-only. Without this, opening the report card would answer 403 there.
8. **The smoke has no read-only gate.** It starts a local process and changes no user data. The adapter maps `ErrUnsupported` and `ErrNotInstalled` to `skipped/runtime_not_installed`, `ErrDisabled` and `ErrClosed` to `skipped/runtime_disabled`, and `ErrBusy` and `ErrInstalling` to `skipped/runtime_busy`. A smoke aborted by a cancel records nothing.
9. **Three chat details:**
   - `POST /runtime/chat` 422 issues all carry the single message key `chibi_sensei.chat.invalid` with field paths such as `targets[1].messages`. F2 may show the generic validation text instead of adding the key.
   - `GET /runtime` `instances[].kind` comes from the key prefix.
   - "Stop models" during an answer ends that target with the stream code `cancelled`.
10. **`Status.Fallback` is sticky.** After a GPU start failed, later instances start directly on CPU until the next `Install`/`BeginInstall` or a change of `runtime.backend`. This avoids a slow failing GPU attempt per model.
11. **The `different` filter** matches samples where both answers are present and differ after `normalizeAnswer` (`report.go:125`).

---

## B1: `internal/llamacpp` extraction and `chibi_sensei.runtime` config

**Goal:** a minimal shared llama.cpp package, with embeddings left on thin wrappers or aliases so that no `internal/embeddings/*_test.go` changes. Plus the runtime config section.

**Create:**
- `internal/llamacpp/build.go`
- `internal/llamacpp/manifest.go`
- `internal/llamacpp/server.go`
- `internal/llamacpp/hardware.go`
- `internal/llamacpp/vulkan_loader_windows.go`
- `internal/llamacpp/vulkan_loader_other.go`
- `internal/llamacpp/manifest_test.go`
- `internal/llamacpp/server_test.go`
- `internal/llamacpp/hardware_test.go`

**Modify:**
- `internal/embeddings/manifest.go`, `candidates.go`, `llama_embedder.go`, `llama_docker.go`
- delete `internal/embeddings/vulkan_loader_windows.go` and `vulkan_loader_other.go`
- `internal/config/chibi_sensei.go`, `internal/config/config.go`, `internal/config/chibi_sensei_test.go`
- `config_template.yaml`

### Task B1.1: the package (moves, with the wrapper that stays)

- [ ] `internal/llamacpp/build.go` (package doc: "Package llamacpp holds the pinned llama.cpp build shared by local embeddings and the Chibi Sensei chat runtime: release assets, Docker images, server discovery, loopback helpers, GPU evidence and hardware detection. It imports only internal/onnxrt."):

```go
const Build = "b9994"
const (
	DockerCPUImage    = "ghcr.io/ggml-org/llama.cpp:server-b9994@sha256:823b6f019cafbee8878dfdd0d4750eae4f81dfafb60dc1fbefb66794a59903c8"
	DockerCUDAImage   = "ghcr.io/ggml-org/llama.cpp:server-cuda-b9994@sha256:b57dce073940d6f347d59230d4d28fc947db8948f2eb162326008380b07bab77"
	DockerVulkanImage = "ghcr.io/ggml-org/llama.cpp:server-vulkan-b9994@sha256:5c9c880045f1d6131aa7c90bf2695dbb3e9c9bf2e3f4513eb0672f1500318e02"
)
func DockerImage(backend string) (string, bool) // body of embeddings/llama_docker.go:340-351
```

- [ ] `manifest.go`:
  - `type AssetSpec = onnxrt.AssetSpec`
  - `const WindowsCUDARuntimeID = "llama-windows-amd64-cuda-runtime"`
  - the unexported `var runtimeManifest = map[string]AssetSpec{…}`: the 13 entries of `embeddings/manifest.go:64-117` byte for byte, with `onnxrt.AssetTarGzip`/`onnxrt.AssetZip` as kinds.
  - `func Runtimes() map[string]AssetSpec { return maps.Clone(runtimeManifest) }`
  - `func RuntimeAsset(goos, goarch, backend string) (AssetSpec, bool)`: body of `embeddings/manifest.go:175-184`.
  - `SupplementalAssets`:

```go
// SupplementalAssets lists the extra runtime archives a backend needs: on windows/amd64 the CUDA runtime libraries
// (unpacked as their own runtime root and put on the library path); nil everywhere else.
func SupplementalAssets(goos, goarch, backend string) []AssetSpec {
	if goos == "windows" && goarch == "amd64" && backend == "cuda" {
		return []AssetSpec{runtimeManifest[WindowsCUDARuntimeID]}
	}
	return nil
}
```

- [ ] `server.go`. Bodies and error texts stay unchanged:
  - `FindServer(root string) (string, error)` from `embeddings/llama_embedder.go:344-367`
  - `GPUVerified(backend, logs string) bool` from `:369-379`
  - `LoopbackPort() (int, error)` from `:381-392`
  - `RandomAPIKey() (string, error)` from `:394-400`
- [ ] `hardware.go`:
  - `type HardwareInfo struct{ NVIDIA bool; NVIDIAReason string; Vulkan bool; VulkanReason string; Fingerprint string }`
  - `DetectHardware(ctx)` and `DetectHardwareWith(ctx, augment)`. The body is `embeddings/candidates.go:91-126`, with the call `augmentDockerHardware(ctx, &info)` (`:119-121`) replaced by `if augment != nil { if line := augment(ctx, &info); line != "" { evidence = append(evidence, line) } }`.
  - the unexported `shortCommand`, moved from `candidates.go:129-139`.
- [ ] `vulkan_loader_windows.go` / `vulkan_loader_other.go`: move `embeddings/vulkan_loader_windows.go:1-26` and `vulkan_loader_other.go` (`//go:build !windows`, returns `""`) with the package line changed.

### Task B1.2: embeddings wrappers (no embeddings test file changes)

| Moved symbol (current place) | What stays in `internal/embeddings` |
|---|---|
| `llamaRuntimeBuild` `manifest.go:13` | `llamaRuntimeBuild = llamacpp.Build` (same const block) |
| `llamaDocker{CPU,CUDA,Vulkan}Image` `manifest.go:22-24` | `llamaDockerCPUImage = llamacpp.DockerCPUImage` (likewise CUDA, Vulkan) |
| `runtimeManifest` `manifest.go:64-117` | `var runtimeManifest = llamacpp.Runtimes()` |
| `llamaRuntimeAsset` `manifest.go:175-184` | `func llamaRuntimeAsset(goos, goarch, backend string) (assetSpec, bool) { return llamacpp.RuntimeAsset(goos, goarch, backend) }` |
| `hardwareInfo` `candidates.go:82-88` | `type hardwareInfo = llamacpp.HardwareInfo` |
| `detectHardware` `candidates.go:90-127` | `func detectHardware(ctx context.Context) hardwareInfo { return llamacpp.DetectHardwareWith(ctx, augmentDockerHardware) }` |
| `shortCommand` `candidates.go:129-139` | removed (only caller moved) |
| `vulkanLoaderEvidence` (both files) | files deleted (only caller moved) |
| `findLlamaServer` `llama_embedder.go:344` | one-line wrapper over `llamacpp.FindServer` |
| `llamaGPUVerified` `:369` | wrapper over `llamacpp.GPUVerified` |
| `availableLoopbackPort` `:381` | wrapper over `llamacpp.LoopbackPort` |
| `randomAPIKey` `:394` | wrapper over `llamacpp.RandomAPIKey` |
| `llamaDockerImage` `llama_docker.go:340` | wrapper over `llamacpp.DockerImage` |

- [ ] Remove the imports that become unused (`crypto/sha256`, `encoding/hex`, `os/exec`, `time`, `net`, `crypto/rand`, `encoding/base64` where they apply).
- [ ] Leave these unchanged:
  - `manager.go` (`:612` and `:959` keep indexing `runtimeManifest`);
  - `selectDockerModelMount` and the whole sidecar;
  - `assets.go`;
  - `llama_embedder.go:76` and `:158`. That file keeps its `&http.Client{` and its `Authorization` header, because `credentialedRedirectPending` in `internal/audit/credentialed_redirect_guard_test.go:26` is a ratchet that fails when the file no longer matches.
- [ ] `augmentDockerHardware(ctx, *hardwareInfo) string` (`llama_docker.go:281`) already has the `augment` signature through the alias.

### Task B1.3: config

- [ ] `internal/config/chibi_sensei.go`:
  - add `Runtime ChibiSenseiRuntimeConfig \`yaml:"runtime" json:"runtime"\`` after `Training` (`:18`);
  - add the struct of the contract;
  - update the doc comment (`:8-11`): "Enabled, Training.DeleteRemoteAfterRun and Runtime.Enabled default to true in the loader; a later phase adds Agent";
  - add `var ChibiSenseiRuntimeBackends = []string{"auto", "cpu", "vulkan", "cuda"}`.
- [ ] `NormalizeChibiSenseiConfig` (`:57`):
  - before the int table: `c.Runtime.Backend = strings.ToLower(strings.TrimSpace(c.Runtime.Backend))`; `""` becomes `"auto"`; a value outside the list returns `fmt.Errorf("chibi_sensei: runtime.backend must be one of %s", strings.Join(ChibiSenseiRuntimeBackends, ", "))`;
  - add two table rows: `{&c.Runtime.IdleMinutes, 10, 1, 120, "runtime.idle_minutes"}` and `{&c.Runtime.ContextSize, 4096, 1024, 32768, "runtime.context_size"}`;
  - it does not touch `Runtime.Enabled`.
- [ ] `internal/config/config.go` in `load` (`:398`), after `:608`: `cfg.ChibiSensei.Runtime.Enabled = true` (the comment notes the local chat runtime downloads nothing until an administrator confirms).
- [ ] `config_template.yaml` after `:2446`:

```yaml
    runtime:
        enabled: true               # local llama.cpp chat in the Test step; nothing is downloaded until an administrator confirms
        backend: "auto"             # auto, cpu, vulkan or cuda; auto tries CUDA, then Vulkan, then CPU
        idle_minutes: 10            # a test model nobody used for this long is stopped (1-120)
        context_size: 4096          # tokens one test chat can hold (1024-32768)
```

### Tests

`internal/llamacpp`:
- `TestRuntimeManifestPinsBuild`: every key equals its ID, every URL contains `/b9994/`, every kind is zip or tar.gz, every spec passes `onnxrt.ValidateAssetSpec`, and `Runtimes()` returns a copy (changing it leaves `RuntimeAsset` unchanged).
- `TestRuntimeAssetAndSupplements`: `RuntimeAsset("darwin","arm64","cpu")` returns the metal archive; `("windows","arm64","cuda")` returns false; `SupplementalAssets` gives one archive only for windows/amd64/cuda.
- `TestDockerImageDigestPinned`: cpu, cuda and vulkan contain `@sha256:`; metal returns false.
- `TestServerHelpers`: `FindServer` finds `nested/llama-server[.exe]`, case-insensitive, and returns the "not found below" error otherwise; `LoopbackPort` > 0; `RandomAPIKey` is 43 characters, two calls differ; `GPUVerified` uses the two cases of `embeddings_test.go:336-343` plus metal.
- `TestDetectHardwareWithAugment`: a stable fingerprint across two calls; an augment that sets `NVIDIA` and returns `"docker=x"` changes the fingerprint and sets the flag; an augment returning `""` gives the same fingerprint as `DetectHardware`.

`internal/config`:
- `TestChibiSenseiDefaults` (`chibi_sensei_test.go:22`): `want` gains `Runtime: ChibiSenseiRuntimeConfig{Enabled: true, Backend: "auto", IdleMinutes: 10, ContextSize: 4096}`. Add the case `"null runtime": "chibi_sensei:\n  runtime:\n"`.
- New `TestChibiSenseiRuntimeConfig`:
  - an explicit `enabled: false` survives `Load`;
  - `" CUDA "` becomes `cuda`;
  - `metal` and `rocm` are rejected with the message above;
  - the idle bounds 1 and 120 are accepted, 0 becomes 10, 121 is rejected;
  - the context bounds 1024 and 32768 are accepted, 1023 is rejected.
- `TestChibiSenseiTemplateMatchesLoadDefaults` passes unchanged (template and loader agree).

### Commands

```
go build ./...
go vet ./internal/llamacpp/... ./internal/embeddings/... ./internal/config/...
GOOS=linux GOARCH=amd64 go vet ./internal/llamacpp/ ./internal/embeddings/
GOOS=darwin GOARCH=arm64 go vet ./internal/llamacpp/ ./internal/embeddings/
go test ./internal/llamacpp/... ./internal/embeddings/... ./internal/config/... -count=1
git status --short -- internal/embeddings/ | grep _test.go    # must print nothing
go test ./internal/audit/ -count=1 -run '(?i)(NetworkClientInventory|CredentialedHTTPClients|SubprocessLaunches|PersonDetection)'
go test ./internal/server -count=1 -timeout 10m -run '(?i)(ChibiSensei|Embeddings)'
```

**Audit scan for B1:**
- `internal/llamacpp` builds no HTTP client, so it needs no inventory row.
- `exec.CommandContext` in `shortCommand` keeps `onnxrt.ConfigureWorkerCommand`, which passes `TestSubprocessLaunchesDoNotInheritRawProcessEnvironment` (`audit_manifests_test.go:284`).
- No audit test scans pins or download hosts outside person detection.

**Done when:** every command above passes, no embeddings test file changed, and `internal/llamacpp` imports only the standard library and `internal/onnxrt`.

---

## B2: `internal/chibisensei/chatrt`

**Goal:** the chat runtime manager with a launcher seam, the native launcher, the streaming chat client and message preparation, all tested without a real `llama-server` except the gated smoke. `chatrt` imports `internal/llamacpp`, `internal/onnxrt`, `internal/security` and `internal/fileutil`, never `internal/chibisensei`.

**Create (each under 600 lines):**

| File | Content |
|---|---|
| `errors.go` | the contract errors plus `ErrStopped`, `ErrClosed` |
| `types.go` | Config, ModelRef, Message, Sampling, ChatRequest, ChatStats, InstanceStatus, Status, DownloadStatus |
| `launcher.go` | Launcher, LaunchSpec, Process |
| `manager.go` | NewManager, Start, Status, StopAll, Close, the reaper |
| `acquire.go` | Admit, Acquire, Instance, Release |
| `install.go` | Installed, Install, BeginInstall |
| `backend.go` | resolution and fallback |
| `messages.go` | PrepareMessages |
| `client.go` | request body and HTTP stream |
| `sse.go` | the stream parser |
| `native_launcher.go` | NativeLauncher |
| `native_process.go` | start, health, logs, stop |
| `process_windows.go` | job object |
| `process_other.go` | plain process |
| `logbuf.go` | bounded log buffer |
| `chatrttest/fake.go` | exported fake |

Tests: `fake_test.go`, `manager_test.go`, `install_test.go`, `backend_test.go`, `messages_test.go`, `sse_test.go`, `client_test.go`, `native_test.go`, `real_smoke_test.go`, and `testdata/stream_basic.txt`, `testdata/stream_tolerant.txt`, `testdata/context_error.json`, `testdata/context_error_stream.txt`.

**Modify:** `internal/audit/audit_manifests.go:259-260` region. Add after the embeddings row:
`{Path: "internal/chibisensei/chatrt/", Classification: "pinned-llama-cpp-runtime-download-and-authenticated-loopback-chat-child", AllowsLocalNet: true, Credentialed: true},`

### Task B2.1 (first): discovery with the real binary

- [ ] Write `real_smoke_test.go` (package `chatrt`) with `TestRealLlamaServerChat`:
  - It skips unless `AURAGO_RUN_LLAMA_SMOKE=1`, and skips when `CHIBI_SMOKE_GGUF` is empty.
  - The cache is `AURAGO_LLAMA_SMOKE_CACHE`, or `t.TempDir()`.
  - The optional `CHIBI_SMOKE_RECORD=<dir>` writes every raw output to files there.
- [ ] Phases, in order, each logging raw output with `t.Logf`:
  1. `NativeLauncher().Install` of `Assets("cpu")`, logging progress.
  2. `llama-server --version`, then `--help`, filtered by `(?m)^.*(--reasoning|--jinja|--no-webui|--webui|--api-key|--threads|--ctx-size|--parallel|--n-gpu-layers|--no-slots|--slots).*$`. Both run through `exec.CommandContext` with `onnxrt.ConfigureWorkerCommand(cmd, runtimeRoot)` and a 30 s timeout. Record `version.txt` and `help.txt`.
  3. `Launch` with `ContextSize: 1024`, `Backend: "cpu"`, `Key: "tuned:smoke"`.
  4. Raw stream: POST the exact body of `buildChatBody` (B2.4) for "Say hello in five words." with `MaxTokens: 32`. Log every line as it arrives; record `stream.txt`.
  5. Parsed path: `proc.Chat` with the same request. It asserts non-empty text, `FinishReason` ∈ {stop, length} and `CompletionTokens > 0`.
  6. Over-long prompt: one user message of `strings.Repeat("word ", 3000)`. Log the status, `Content-Type` and the first 4 KiB of the body twice: `stream:true` (record `overlong_stream.txt`) and `stream:false` (record `overlong_plain.txt`). Then `proc.Chat` must return `errors.Is(err, ErrContext)`.
  7. Cancel: `Chat` with `MaxTokens: 512`; cancel the context at the first delta. A following short `Chat` must finish within 20 s, which proves the single slot was freed upstream.
  8. `Stop`; `Done()` closes within 5 s.
- [ ] Run it once with a GGUF the coordinator names (any run's `model-q8_0.gguf`, or a small public chat GGUF):
  `AURAGO_RUN_LLAMA_SMOKE=1 CHIBI_SMOKE_GGUF=<path> CHIBI_SMOKE_RECORD=C:/disposable/chibi-sensei/p3/discovery go test ./internal/chibisensei/chatrt -run '^TestRealLlamaServerChat$' -count=1 -v -timeout 20m`
- [ ] Copy into `testdata/`: `stream.txt` → `stream_basic.txt`; `overlong_plain.txt` body → `context_error.json`; `overlong_stream.txt` → `context_error_stream.txt` (only when the stream form carried the error).
- [ ] If no GGUF is available during the dispatch, write provisional fixtures in OpenAI framing, mark them `PROVISIONAL` in the report, and leave the run to the coordinator. The parser below must not depend on the details either way.

### Task B2.2: types and seam

The contract types are copied verbatim into `types.go`, plus:

```go
type DownloadStatus struct {
	Bytes, DoneBytes int64  // total of the install set; bytes done so far
	Host             string // host of the asset URLs, e.g. "github.com"
	ErrorCode        string // "" | download_failed | checksum_mismatch | disk_full
}
const MaxInstances = 2
```

`launcher.go`:

```go
// Launcher is the seam between the manager and the operating system. NativeLauncher is the real one.
type Launcher interface {
	Platform() (goos, goarch string)
	Hardware(ctx context.Context) llamacpp.HardwareInfo
	// Assets is the install set of a backend: the CPU archive first, then the backend's archive and its
	// supplements, without duplicate IDs. ok is false when the platform has no CPU archive.
	Assets(backend string) (assets []llamacpp.AssetSpec, ok bool)
	// Verify reports, without network access, whether every asset is unpacked intact below cacheDir.
	Verify(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec) bool
	// Install downloads and unpacks the assets in order; progress gets the asset ID and its bytes done.
	Install(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec, progress func(assetID string, done int64)) error
	// Launch starts one server and returns when /health answers 200, or fails. It never downloads.
	Launch(ctx context.Context, spec LaunchSpec) (Process, error)
}
type LaunchSpec struct {
	Key         string               // ModelRef.Key (logs, fakes)
	CacheDir    string
	Assets      []llamacpp.AssetSpec // runtime roots of this start: [cpu] or [gpu, supplements...]
	Backend     string               // cpu | vulkan | cuda | metal
	ModelPath   string
	ContextSize int
	Threads     int
}
type Process interface {
	Chat(ctx context.Context, req ChatRequest, onDelta func(text string)) (ChatStats, error) // Backend left empty
	GPUVerified() bool        // GPU evidence in the server log; false for cpu
	Done() <-chan struct{}    // closed when the process exited
	Stop() error              // kill by handle and wait; idempotent
}
```

### Task B2.3: manager (rules are binding)

**State:**
- `Manager{cfg func() Config; launcher; log; now func() time.Time; reapEvery (30 s); busyWait (30 s); life context.Context; lifeCancel; mu sync.Mutex; res *resolution; installed *bool; verifyMu sync.Mutex; inst *installOp; lastErrCode string; gpuFailed bool; failedFor string (backend the sticky flag belongs to); instances map[string]*instance; closed bool; reaperDone chan struct{}}`.
- `instance{key, backend string; gpuVerified bool; proc Process; started, lastUsed time.Time; ready chan struct{}; startErr error; slot chan struct{} (cap 1); waiters int; stopping bool; cancelStart context.CancelFunc}`.
- `Instance` is a lease: `{m *Manager; in *instance; once sync.Once}`.

**Lock rule:** `mu` is never held across `Hardware`, `Verify`, `Install`, `Launch`, `Chat` or `Stop`. Those run with copies taken under the lock.

**resolve(ctx)**, cached per configured backend:
- `requested == "auto"`: darwin/arm64 → `metal`; else NVIDIA and a `cuda` asset → `cuda`; else Vulkan and a `vulkan` asset → `vulkan`; else `cpu`.
- `cpu`: cpu.
- `cuda` / `vulkan`: the value when `RuntimeAsset` exists for the platform, else `cpu`.
- `Hardware` runs only for `auto`.
- The install set is `Assets(selected)`. No CPU archive → unsupported.
- A change of `cfg().Backend` drops the cache, `installed` and the sticky fallback.

**Installed():** returns the cached verify result. The first call (or after a reset) runs `launcher.Verify` under `verifyMu` with a 2-minute background context.

**State order of `Status`:** disabled (`!cfg.Enabled`) > unsupported > installing > failed (`lastErrCode != ""`) > ready (installed) > not_installed.
- `Download.Bytes` is the sum of the install set's sizes; `Host` is `url.Parse(assets[0].URL).Host`.
- `Instances` are sorted by key.
- `IdleSeconds` is `now-lastUsed` for an idle ready instance, else 0.
- `Started` is RFC 3339 UTC.

**BeginInstall(ctx):**
- ErrDisabled / ErrUnsupported / ErrClosed when they apply.
- Already installed: a closed channel.
- While installing: the running op's channel (one download only).
- Otherwise:
  1. register `installOp{done, doneBytes map[assetID]int64}`, clear `lastErrCode`, `gpuFailed` and `installed`;
  2. check free disk: `fileutil.FreeDiskBytes(filepath.Dir(cacheDir))` must be ≥ 4×Bytes + 64 MiB, else end with `disk_full`;
  3. run `launcher.Install` in a goroutine with `ctx2, cancel := context.WithTimeout(m.life, 60*time.Minute)` plus `context.AfterFunc(ctx, cancel)`;
  4. on success set `installed = true`; on failure set `lastErrCode = installErrorCode(err)`.
- `Install(ctx)` = `BeginInstall`, then wait for `done` or `ctx`, then return the result error.

`installErrorCode` copies the pattern of `persondetect/images.go:339-354` (`isNoSpace`: ENOSPC/EDQUOT, Windows 39/112) for `disk_full`. `checksum_mismatch` covers errors whose text contains `SHA-256`, `content length` or `received` (the fixed onnxrt messages at `onnxrt/assets.go:296,319,324`). Everything else is `download_failed`. The raw error goes to the log through `security.Scrub` only.

**Admit(keys...):** returns ErrClosed, ErrDisabled, ErrUnsupported, ErrInstalling or ErrNotInstalled (the same precedence as `Status`). Otherwise `need` = number of keys without an instance and `freeable` = instances outside `keys` with a free slot, `waiters == 0` and not starting. It answers ErrBusy when `len(instances) - freeable + need > MaxInstances`.

**Acquire(ctx, ref):**
1. Run the state checks of `Admit` (it never downloads).
2. `os.Stat(ref.Path)` must be a regular file of `ref.Bytes`, else ErrModelMissing.
3. Under `mu`:
   - **Existing instance:** `waiters++`, unlock, wait for `ready` (only `ctx` bounds this wait), then take the slot within `busyWait` (`select` on `slot <- struct{}{}`, a timer, `ctx`). Then `waiters--`. A dead, stopping or failed instance is dropped and the loop retries once. A timeout returns ErrBusy.
   - **No instance, `len < MaxInstances`:** create one in `starting`.
   - **No instance, full:** evict the least recently used idle instance (free slot, `waiters == 0`, ready). Mark it `stopping`, delete it from the map and `Stop` it after the unlock. With no idle instance return ErrBusy at once, without waiting.
   - Then start in a goroutine with `startCtx = context.WithTimeout(m.life, 6*time.Minute)` (the instance's `cancelStart`). The caller waits for `ready` as an ordinary waiter, so a caller that gives up leaves a warm instance behind.
4. **launchWithFallback:**
   - `backend = selected`, or `cpu` when the sticky `gpuFailed` holds for this backend.
   - For a GPU backend: `Launch` with `Assets = [gpu asset] + SupplementalAssets`. On success with `GPUVerified()` it is done. A successful start without evidence is `Stop()`ped.
   - Unless `startCtx` ended: log `slog.Warn("Chibi Sensei chat runtime fell back to CPU", "backend", b, "error", scrubbed)`, set `gpuFailed = true`, then `Launch` on CPU with `Assets = [cpu asset]`.
   - `Threads = max(1, runtime.NumCPU()/2)`; `ContextSize = cfg.ContextSize`.
   - A start failure removes the instance from the map and wraps the cause. The cause is not cached, so the next Acquire tries again.

**Instance.Chat(ctx, req, onDelta):**
- Requires the lease; otherwise `errors.New("chat runtime: instance not held")`.
- Calls `proc.Chat` and fills `stats.Backend`.
- When `ctx.Err() != nil` it returns `ctx.Err()`.
- When the instance was stopped by `StopAll` or `Close` (`stopping`) it returns ErrStopped.
- When `proc.Done()` is closed (crash) it removes the instance and returns the wrapped error.
- `ErrContext` passes through.

**Release:** once per lease. Sets `lastUsed = now`, frees the slot, and is safe after `Close`.

**reapOnce:**
- With `!cfg.Enabled`, it stops every instance.
- Otherwise it stops each instance that is ready, has a free slot, has `waiters == 0` and `now - lastUsed ≥ IdleMinutes`.
- It drops instances whose `Done()` is closed.
- `Start(ctx)` runs `reapOnce` every `reapEvery` until `ctx` or `Close`. It is idempotent.

**StopAll(ctx):** marks every instance `stopping`, cancels starts, empties the map, then `Stop()`s each one outside the lock, so busy chats end with ErrStopped. It is idempotent.

**Close:** sets `closed`, then `lifeCancel()` (ends install and starts), `StopAll`, and waits for the reaper (at most 5 s). Afterwards `Acquire`, `Admit` and `BeginInstall` answer ErrClosed.

`PrepareMessages(systemRole, in)` (`messages.go`): returns a copy. With `systemRole == "merge_into_first_user"`, a leading `system` message is removed and its non-empty content becomes `system + "\n\n" + firstUser.Content`. It never mutates `in`.

### Task B2.4: client and parser

`client.go`. The HTTP client, in the same file as the `Authorization` header so the redirect guard (`credentialed_redirect_guard_test.go:72`) is satisfied:

```go
func newLoopbackClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Minute, MaxIdleConnsPerHost: 2},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
type chatBody struct {
	Model              string          `json:"model"` // "chibi" (the --alias)
	Messages           []Message       `json:"messages"`
	Stream             bool            `json:"stream"` // true
	StreamOptions      map[string]bool `json:"stream_options"` // {"include_usage":true}
	Temperature        float64         `json:"temperature"`
	TopP               float64         `json:"top_p"`
	TopK               int             `json:"top_k"`
	RepeatPenalty      float64         `json:"repeat_penalty"`
	PresencePenalty    float64         `json:"presence_penalty"`
	MaxTokens          int             `json:"max_tokens"`
	Seed               *int64          `json:"seed,omitempty"` // only when Sampling.Seed > 0
	Stop               []string        `json:"stop,omitempty"`
	ChatTemplateKwargs map[string]any  `json:"chat_template_kwargs"` // {"enable_thinking": false} (D4)
	CachePrompt        bool            `json:"cache_prompt"` // true
}
func buildChatBody(req ChatRequest) chatBody
// streamChat POSTs <baseURL>/v1/chat/completions with "Authorization: Bearer <key>" and Accept text/event-stream.
// A non-200 answer: read at most 16 KiB, decode {"error":{code,message,type}}, classifyUpstream. Cancelling ctx
// closes the connection, which ends the generation upstream.
func streamChat(ctx context.Context, c *http.Client, baseURL, key string, req ChatRequest, onDelta func(string)) (ChatStats, error)
```

**Stats:**
- `MS` = wall time.
- `PromptTokens` = usage.prompt_tokens, else timings.prompt_n.
- `CompletionTokens` = usage.completion_tokens, else timings.predicted_n, else the number of deltas.
- `TokensPerSecond` = timings.predicted_per_second, else completion tokens divided by the seconds from the first delta to the end.
- `FinishReason` = the stream's value; `"stop"` when only `[DONE]` came.

`sse.go`. Tolerant; the fake-based tests never depend on it:

```go
const (maxStreamLine = 1 << 20; maxStreamBytes = 8 << 20; maxBadLines = 16)
type streamChunk struct {
	Choices []struct {
		Delta        struct{ Content *string `json:"content"` } `json:"delta"` // reasoning fields are not decoded
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage   *struct{ PromptTokens int `json:"prompt_tokens"`; CompletionTokens int `json:"completion_tokens"` } `json:"usage"`
	Timings *struct{ PromptN int `json:"prompt_n"`; PredictedN int `json:"predicted_n"`; PredictedPerSecond float64 `json:"predicted_per_second"` } `json:"timings"`
	Error   *upstreamError `json:"error"`
}
type upstreamError struct{ Code int `json:"code"`; Message string `json:"message"`; Type string `json:"type"` }
type streamResult struct{ FinishReason string; PromptTokens, CompletionTokens, Deltas int; PerSecond float64; Done bool }
func parseStream(r io.Reader, onDelta func(string)) (streamResult, error)
var contextPattern = regexp.MustCompile(`(?i)(exceed\w*|too long|larger than|longer than)[^.]{0,40}context|context[^.]{0,40}(exceed\w*|too (long|small)|size)`)
func classifyUpstream(status int, e upstreamError) error // ErrContext when Type contains "context" or contextPattern matches Message
```

**Line rules:**
- strip `\r`;
- empty lines, `:` comments, `event:`, `id:` and `retry:` lines are skipped;
- `data:` with or without a space carries the payload, and a bare line starting with `{` counts as a payload;
- `[DONE]` ends the stream;
- payloads that do not decode are skipped, and more than `maxBadLines` is an error;
- a `chunk.Error` returns `classifyUpstream(0, *chunk.Error)`;
- only `choices[0].delta.content` (non-empty) reaches `onDelta`;
- the last `finish_reason`, usage and timings win;
- EOF without `[DONE]` is accepted when a `finish_reason` came, otherwise it returns `errStreamCut`;
- a line above `maxStreamLine` is an error.

### Task B2.5: native launcher and process

**`Assets(backend)`:** pure `installSet(goos, goarch, backend)`: `[RuntimeAsset(cpu)] + [RuntimeAsset(backend)] + SupplementalAssets`, de-duplicated by ID (darwin/arm64 cpu and metal are one archive). It returns false without a CPU archive.

**`Verify`:** `onnxrt.NewAssetCache(cacheDir, "AuraGo/chibi-sensei", nil).VerifyRuntime` for each asset.

**`Install`:** `onnxrt.NewAssetCache(cacheDir, "AuraGo/chibi-sensei", func(s onnxrt.DownloadStatus){ progress(s.Asset, s.Downloaded) })` and `EnsureRuntime` for each asset. The cache root is `CacheDir` (`<data>/chibi_sensei/runtime`, D2), never `<data>/embeddings`.

**`Launch`:**
1. `VerifyRuntime` for each `spec.Assets`, which gives the runtime roots and never downloads.
2. `llamacpp.FindServer(roots[0])`; `key := llamacpp.RandomAPIKey()`; `security.RegisterSensitive(key)` before the start (D15).
3. `port := llamacpp.LoopbackPort()`.
4. `cmd := exec.Command(server, serverArgs(spec, port)...)`; `onnxrt.ConfigureWorkerCommand(cmd, roots...)`; `cmd.Env = serverEnv(cmd.Env, key)`.
5. stdout and stderr go to `newLogBuffer(64 << 10)` (copy of `embeddings/onnx_worker.go:621-645`).
6. `startProcess(cmd)` (per OS, below).
7. `waitHealthy`. If the process exits before health and its log contains `bind` or `address already in use`, retry once with a new port.
8. Errors carry `security.Scrub(log tail)` and never the key.

```go
func serverArgs(spec LaunchSpec, port int) []string {
	ngl := "0"
	if spec.Backend != "cpu" { ngl = "999" }
	t := strconv.Itoa(max(1, spec.Threads))
	return []string{"--model", spec.ModelPath, "--alias", "chibi", "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"--ctx-size", strconv.Itoa(spec.ContextSize), "--parallel", "1", "--n-gpu-layers", ngl,
		"--threads", t, "--threads-batch", t, "--jinja", "--reasoning", "off", "--no-webui", "--no-slots"}
}
// serverEnv drops every inherited LLAMA_* variable (LLAMA_ARG_* would change flags we do not pass) and adds the key.
func serverEnv(env []string, key string) []string // filtered copy + "LLAMA_API_KEY="+key (never cmd.Env = append(os.Environ(), …))
```

**`waitHealthy(ctx, done, baseURL)`:**
- `GET /health` every 250 ms with a 2 s per-request timeout and no key (public endpoint);
- 200 means ready; 503 means "loading", keep waiting;
- `done` closed means `fmt.Errorf("llama-server exited: %s", scrubbed log)`;
- the overall limit is 3 minutes.

**`GPUVerified()`:** `spec.Backend != "cpu" && llamacpp.GPUVerified(backend, logs.String())`, evaluated once right after health.

**Process lifetime and AuraGo's exit:**
- `onnxrt.ConfigureWorkerCommand` only filters the environment, extends the library paths and hides the console window on Windows (`onnxrt/process.go:24-31`, `process_windows.go:15-21`). It sets nothing about child lifetime.
- Embeddings starts `llama-server` with a plain `exec.Command` and stops it with `Process.Kill` plus `Wait` (`llama_embedder.go:227-236, 330-342`). On every OS its child outlives a crashed AuraGo.
- `chatrt` does better on Windows:
  - `process_windows.go` copies `internal/ruview/process_windows.go:13-45`: a job object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` that keeps the `SysProcAttr` `ConfigureWorkerCommand` set and assigns the process after `Start`. `Stop` closes the job handle and waits.
  - When AuraGo exits or crashes, Windows closes the handle and kills `llama-server`.
- `process_other.go` (`!windows`) does what embeddings does: plain `Start`, stop by `Process.Kill` and `Wait`. There is no `Setpgid`, so Ctrl+C in a terminal also reaches the child, and no `Pdeathsig`, which fires when the creating OS thread ends.
  - Under systemd (`KillMode=control-group` default, see `update.sh:2346`) and in Docker (PID 1 exit) the child dies with AuraGo.
  - A `kill -9` of a hand-started AuraGo leaves an orphan bound to loopback with an unknown key. This is listed under Open risks.
- The manager stops only processes it started, by their handle.

**`Chat`:** `streamChat` with the process's client, URL and key. When `Done()` is closed it returns an error that wraps `errProcessExited`.

### Task B2.6: `chatrttest` (exported fake for the server)

```go
package chatrttest
type Launcher struct { // all fields under mu
	Replies    map[string]string // by key prefix ("tuned:", "base:"); default "Fake answer."
	FailLaunch map[string]error  // by key prefix
	ChatErr    map[string]error  // by key prefix, returned after the reply was streamed
	Hold       chan struct{}     // non-nil: Chat waits for it or ctx before answering
}
func New(installed bool) *Launcher
func (l *Launcher) SetInstalled(bool)
func (l *Launcher) Launches() []chatrt.LaunchSpec
func (l *Launcher) Requests(prefix string) []chatrt.ChatRequest
func (l *Launcher) Stopped() int
func (l *Launcher) Cancelled() <-chan string // key of a Chat that saw ctx end (buffer 8)
```

- `Platform` is linux/amd64 and `Hardware` is zero.
- `Assets` returns one CPU spec: ID `fake-cpu`, URL `https://github.com/ggml-org/llama.cpp/releases/download/b9994/fake.tar.gz`, Size 15_857_307, a 64-hex SHA, kind tar.gz.
- `Verify` returns the installed flag. `Install` reports progress and sets it.
- `Chat` streams the reply word by word, keeping the spaces, and returns `{FinishReason: "stop", PromptTokens: 10*len(messages), CompletionTokens: words, MS: 5, TokensPerSecond: 20}`.

### Tests (fake process in `fake_test.go`, clock and intervals set on the unexported fields)

- `TestManagerRefusesBeforeInstall`: disabled → ErrDisabled; no CPU archive → ErrUnsupported with state `unsupported`; not verified → ErrNotInstalled with state `not_installed`, `Download.Bytes` > 0 and host `github.com`; `Acquire` never calls `Install`.
- `TestManagerInstallProgressAndJoin`: a blocking fake install; `Status` shows `installing` and `DoneBytes`; a second `BeginInstall` returns the same channel and `Install` runs once; `Acquire` during it → ErrInstalling; afterwards the state is `ready`.
- `TestManagerInstallErrorCodes`: a checksum text, wrapped `syscall.ENOSPC` and a plain error give their three codes and state `failed`; the next `BeginInstall` clears the code. Free disk below the rule gives `disk_full` without calling `Install`.
- `TestManagerReusesInstanceAndWaitsWhileBusy`: the same key launches once; a second `Acquire` blocks until `Release`; with `busyWait` set to 50 ms and no release → ErrBusy.
- `TestManagerEvictsLeastRecentlyUsedIdle`: A, B, A again (A newer), then C stops B; the instances are A and C; three launches.
- `TestManagerBusyWhenNoIdleSlot`: A and B held; C → ErrBusy faster than `busyWait`; `Admit("c")` → ErrBusy, `Admit("a")` → nil.
- `TestManagerIdleReaper`: `IdleMinutes` 1 and the clock 61 s later stop an idle instance but keep a held one; a disabled config stops all.
- `TestManagerGPUFallback`:
  - auto with NVIDIA and a cuda asset: a cuda start without evidence is stopped, then a cpu start follows; `Backend()` is cpu and `Status.Fallback` true;
  - the next key starts directly on cpu;
  - a cuda `Launch` error falls back too;
  - `BeginInstall` and a backend config change clear the flag;
  - windows/amd64 cuda launch specs carry the supplement.
- `TestManagerChatCancelStopsGeneration`: cancelling during the fake `Chat` returns `context.Canceled` within 1 s, and the instance can be acquired again.
- `TestManagerStopAllAndClose`: a busy chat returns ErrStopped and `Status` lists no instance; `StopAll` twice is safe; after `Close`, `Acquire` returns ErrClosed, the reaper has ended, and `Release` after `Close` does not panic.
- `TestManagerModelMissing` (missing file, wrong size, no launch) and `TestManagerStartFailureNotCached`.
- `TestManagerLaunchSpec`: Threads `max(1, NumCPU/2)`, `ContextSize` from the config, `Key`.
- `TestResolveBackend`: a table over goos, arch, requested and hardware.
- `TestPrepareMessages`: merge, native copy, no system, empty system, input unchanged.
- `TestParseStream`:
  - `stream_basic.txt` gives text, finish reason and usage;
  - `stream_tolerant.txt` (synthetic) covers no space after `data:`, comments, `event:` lines, `reasoning_content` dropped, `\r\n`, a bad line skipped, timings only;
  - an error chunk with a context message gives ErrContext;
  - a cut stream gives an error;
  - finish without `[DONE]` is OK;
  - an overlong line gives an error.
- `TestStreamChatRequest` (httptest):
  - the body has `model` chibi, `stream` true, `chat_template_kwargs.enable_thinking` false, `stop`, `max_tokens`, the sampling values, `seed` only when > 0;
  - the request carries `Authorization: Bearer <key>`;
  - a 302 is not followed;
  - a 400 from `context_error.json` gives ErrContext; another 400 or a 500 gives a non-context error; the body is capped at 16 KiB.
- `TestStreamChatCancelClosesUpstream`: a slow httptest handler sees `r.Context().Done()` within 2 s of the client cancel.
- `TestServerArgsAndEnv`: the exact `serverArgs` for cpu and cuda; no `--api-key` and no key anywhere in argv; the environment holds exactly one `LLAMA_API_KEY=` and no inherited `LLAMA_ARG_HOST`.
- `TestInstallSet`: linux/amd64 cuda → [cpu, cuda]; windows/amd64 cuda → [cpu, cuda, cuda-runtime]; darwin/arm64 metal → one; windows/arm64 vulkan → [cpu].
- `TestWaitHealthy`: 503 then 200 → ok; `done` closed → an error containing the scrubbed log.
- `TestLogBufferBounded`.

### Commands

```
go build ./...
go vet ./internal/chibisensei/chatrt/...
GOOS=linux GOARCH=amd64 go vet ./internal/chibisensei/chatrt/...
GOOS=darwin GOARCH=arm64 go vet ./internal/chibisensei/chatrt/...
go test ./internal/chibisensei/chatrt/... -count=1
go test ./internal/audit/ -count=1 -run '(?i)(NetworkClientInventory|CredentialedHTTPClients|SubprocessLaunches)'
```

**Done when:** all of these pass, the discovery output is in `C:\disposable\chibi-sensei\p3\discovery\` (or the fixtures are flagged provisional), and `chatrt` imports no `internal/chibisensei` package.

---

## B3: domain additions in `internal/chibisensei`

**Create:**
- `samples.go`, `testview.go`, `reconcile_smoke.go`
- `samples_test.go`, `testview_test.go`, `reconcile_smoke_test.go`, `runtime_pin_test.go`

**Modify:** `runs.go`, `reconcile.go`, `reconcile_outputs.go`, `projects.go`, `profiles.go`, `runs_test.go` (one case).

### Task B3.1: run fields and views

- [ ] `RunRemote` (`runs.go:153-219`) gains, after `GGUF`:
  - `Smoke *SmokeResult \`json:"smoke,omitempty"\``
  - `ReportSeen bool \`json:"report_seen,omitempty"\``
- [ ] `RunView` (`:321-350`) gains, after `Result`:
  - `Smoke *SmokeResult \`json:"smoke"\``
  - `ReportSeen bool \`json:"report_seen"\``
- [ ] `Run.view` (`:355`) copies them (`Smoke` as a copy of the pointed-to value). `RunSummary` gets them through the type conversion.

### Task B3.2: `samples.go`

- [ ] The contract's `SampleView` and `SamplePage`, plus:

```go
var ErrSamplesMissing = errors.New("chibisensei: the run has no evaluation examples")
const SamplePageMax = 50
func ReadSamples(dataDir string, run Run, recipe, taskKind string, labels []string, offset, limit int, filter string) (SamplePage, error)
```

- [ ] Rules:
  - `ValidID(run.ID)`, and `filter` ∈ {all, different, wrong} (otherwise a plain error; the server validates first).
  - Clamp `limit` to 1..50 and `offset` to ≥ 0.
  - Open `os.OpenRoot(ChibiDir(dataDir))`, then `root.Open(runRel(run.ID, outSubdir, "eval_samples.jsonl"))`. `fs.ErrNotExist` from either gives ErrSamplesMissing.
  - Parse with `ParseEvalSamples(f, run.Remote.Plan.EvalSamples)` (0 when `Plan` is nil).
  - `Index` is the 0-based position in the file.
  - `PromptMessages` is a copy, or nil with `PromptText` set for text prompts.
  - `sampleMatch(recipe, taskKind, labels, expected, answer *string) *bool`:
    - nil answer → nil;
    - specialist extract/rewrite/to_json → `answerMatches` (`report.go:162`);
    - specialist classify → nil when `matchLabel(labels, expected) < 0`, otherwise label equality (`report.go:213`);
    - every other recipe → nil.
  - `different` and `wrong` follow Contract changes 11 and the contract.
  - `Total` counts after the filter; `Samples` is never nil.

### Task B3.3: `testview.go`

- [ ] Declarations:

```go
var ErrRunNotSucceeded = errors.New("chibisensei: the training run did not succeed")
type ModelFile struct { /* contract */ }
func RunTask(run Run) (recipe, taskKind string, labels []string, err error) // parseJobFacts(run.JobJSON)
func RunModels(ctx context.Context, st *Store, dataDir string, run Run) (tuned, base ModelFile, err error)
func (s *Store) MarkReportSeen(ctx context.Context, runID string) error
```

- [ ] `RunModels` follows D6:
  - `run.State != RunSucceeded` → ErrRunNotSucceeded.
  - `wantArch` is `run.Remote.GGUF.Architecture` when it is set.
  - `checkModel(root, rel, wantBytes, wantArch, key)`, in order:
    1. `!validArtifactPath(rel)` → invalid;
    2. `root.Lstat(rel)`: not found → missing; not a regular file (a symlink included) → invalid;
    3. `wantBytes >= 0` and the size differs → missing;
    4. `readGGUFInRoot` fails or the architecture differs → invalid;
    5. otherwise `Available`, `Bytes`, `Path = filepath.Join(ChibiDir(dataDir), filepath.FromSlash(rel))`, `Key`.
  - Tuned: the `gguf_tuned` artifact with its `Bytes`, key `tuned:<run_id>`. No row → missing.
  - Base:
    - the run's `gguf_base` artifact with its `Bytes`;
    - otherwise `GetBaseGGUF(run.ProfileID, facts.Profile.Base.Revision, facts.converter())` with any size (`wantBytes -1`; the table has no size);
    - otherwise, or when `job.json` does not parse, `skipped`;
    - key `base:<rel>`.
  - A missing `chibi_sensei` directory gives missing/skipped, never an error.
- [ ] `MarkReportSeen`:
  - `GetRun` (ErrNotFound passes through);
  - not succeeded → ErrRunNotSucceeded;
  - already seen → nil;
  - otherwise `UpdateRun(ctx, id, RunEvent{}, fn)`, which writes no event; `fn` checks the state again and sets `Remote.ReportSeen = true`.

### Task B3.4: step rule

- [ ] `projectSelect` (`projects.go:186-189`): append a column after the `succeeded` EXISTS:
  `EXISTS(SELECT 1 FROM runs x WHERE x.project_id=p.id AND x.state='succeeded' AND json_extract(x.remote_json,'$.report_seen')=1)`
- [ ] `scanProject` (`:159-184`): scans it into `seen int` and passes `projectRuns{…, ReportSeen: seen != 0}`.
- [ ] `projectRuns` (`:227`) gains `ReportSeen bool`.
- [ ] `computeSteps` (`:232`), after the training switch:

```go
switch {
case runs.Succeeded && runs.ReportSeen:
	st.Test = "done"
case runs.Succeeded:
	st.Test = "ready"
}
```

  `Use` stays `todo`.

### Task B3.5: profile summary for `/meta`

- [ ] `ProfileSummary` (`profiles.go:116`) gains:
  - `Sampling ProfileSampling \`json:"sampling"\``
  - `MaxNewTokens struct{ Assistant int \`json:"assistant"\`; Specialist int \`json:"specialist"\` } \`json:"max_new_tokens"\``
- [ ] `(*Profile).Summary` (`:210`) fills them from `p.Sampling` and `p.Eval`.

### Task B3.6: the smoke seam (`reconcile_smoke.go`)

- [ ] The contract's `SmokeInput`, `SmokeResult` and `Smoker`, plus `const smokeTimeout = 3 * time.Minute`.
- [ ] The `Orchestrator` struct (`reconcile.go:83-100`) gains `smokeMu sync.Mutex; smoker Smoker; smokeRun string; smokeCancel context.CancelFunc; smokeAborted bool`.
- [ ] Functions:
  - `SetSmoker(s)`: under `smokeMu`; call it before `Start`.
  - `abortSmoke(runID)`: under `smokeMu`; when `smokeRun == runID`, set `smokeAborted` and call `smokeCancel()`.
  - `runSmoke(ctx, r, prof, rel string, bytes int64) (res *SmokeResult, aborted bool, err error)`:
    1. With no smoker → `nil, false, nil`.
    2. `sctx, cancel := context.WithTimeout(ctx, smokeTimeout)`; register run and cancel.
    3. `out := smoker.Smoke(sctx, SmokeInput{RunID: r.ID, ModelPath: filepath.Join(ChibiDir(o.dataDir), filepath.FromSlash(rel)), ModelKey: "tuned:" + r.ID, ModelBytes: bytes, Profile: prof})`.
    4. Unregister and read `smokeAborted`.
    5. `ctx.Err() != nil` → `nil, false, ctx.Err()` (shutdown: the step runs again after the restart).
    6. Aborted → `nil, true, nil`.
    7. Normalize: a state outside {ok, failed, skipped} becomes `failed/generation_failed`; `Code` is cut to 40 characters; `MS ≥ 0`; `Text = scrubText(out.Text, 200)` (`reconcile.go:510`).
- [ ] `stepVerifying` (`reconcile_outputs.go:455-496`). Insert between the end of the base-GGUF block (`:492`) and the `return o.transition(…)` (`:493`):

```go
	var tunedBytes int64
	if out := findKind(r.Remote.Outputs, "gguf_tuned"); out != nil {
		tunedBytes = out.Bytes
	}
	smoke, aborted, err := o.runSmoke(ctx, r, prof, rel, tunedBytes)
	if err != nil {
		return stepResult{}, err // the loop's context ended; verifying runs again after a restart
	}
	if aborted {
		return stepResult{progressed: true}, nil // Cancel arrived during the smoke; the next Step ends the run cancelled (reconcile.go:334)
	}
	if smoke != nil && smoke.State == "failed" {
		if err := o.st.AddRunEvent(ctx, r.ID, NewWarnRunEvent("smoke_failed", "The trained model did not answer a short test prompt.")); err != nil {
			return o.localError(ctx, r, err)
		}
		if warning == "" {
			warning = "smoke_failed"
		}
	}
```

  and change the transition to carry it: the data map gets `"smoke": smoke` and the mutation sets `x.Remote.Smoke = smoke`.
- [ ] **Restart and cancel behaviour** (the doc comment of `stepVerifying` says so):
  - The smoke result is committed only with the transition to `cleaning`. A restart before that commit repeats the header checks and the smoke, which is harmless: the instance key is the same, so the warm instance is reused.
  - A cancel during the smoke: `Cancel` (`reconcile.go:237`) calls `o.abortSmoke(runID)` after its `UpdateRun` commit succeeded, right before `o.publishRun(next)` (`:256`). The smoke context ends, `stepVerifying` returns without committing, and the next `Step` sees `CancelRequested` in `verifying`. `handleCancel` (`:334`) ends the run `cancelled` at once.
  - A cancel stored after the smoke returned, but before the transition commit, is still caught by `cancelWins` (`:280-283`).
  - The step mutex (`:269`) is held during the smoke. `Cancel` does not take it.

### Tests

- `TestReadSamplesFiltersAndPages`:
  - specialist extract with 5 samples: matches as expected;
  - `different` and `wrong` filters;
  - offset/limit paging and the limit clamp;
  - a text-prompt sample has `PromptText` set and `PromptMessages` nil;
  - classify uses labels, and a free recipe gives null matches;
  - a missing file gives ErrSamplesMissing.
- `TestRunModels`:
  - tuned available, with an absolute path under `ChibiDir` and its key;
  - wrong size → missing; bad header → invalid; a symlink → invalid (skip on Windows when no symlink can be made);
  - base from the artifact; base from the shared row; no base → skipped; base file gone → missing; base architecture ≠ tuned → invalid;
  - a failed run → ErrRunNotSucceeded.
  - Uses `fakeGGUF` (`fakekaggle_test.go:1239`).
- `TestMarkReportSeen`: not succeeded → ErrRunNotSucceeded; succeeded → seen with no new run event; idempotent; unknown → ErrNotFound; `GetProject` shows `steps.test` `ready` before and `done` after.
- `runs_test.go:398` table (`computeSteps`): add the cases todo, ready and done for `Test`.
- `TestRunViewSmokeAndReportSeen`: the JSON of `View()` and `Summary()` holds `"smoke":null` and `"report_seen":false`, then the values once set.
- `TestProfileSummarySamplingAndTokens`: qwen35 gives 0.7/0.8/20/1.0/1.5 and 256/64.
- `TestRuntimePinMatchesTrainerLock` (D14): `trainer.LoadLock()`'s `LlamaCppTag == llamacpp.Build` (a test-only import).
- `reconcile_smoke_test.go`, with `newRunFixture` (`reconcile_test.go:51`) and `before: func(fx *runFixture){ fx.o.SetSmoker(sm) }` (a scripted fake smoker that records inputs and contexts):
  - `TestOrchestratorSmokeOK`:
    - succeeded with `Remote.Smoke` `{ok, 812, "Hello"}` and no warning;
    - the input carries `ModelKey "tuned:<id>"`, `ModelBytes` equal to the GGUF size, a path under `ChibiDir` and the profile ID;
    - the last `state` event to `cleaning` holds `"smoke"`.
  - `TestOrchestratorSmokeFailedIsAWarning`:
    - succeeded, `warning_code smoke_failed`, exactly one `warn` event with that code;
    - with `withBaseGGUF(o, "llama")` (`reconcile_fixes_test.go:20`), the warning is `base_gguf_invalid` and the `smoke_failed` warn event still exists.
  - `TestOrchestratorSmokeSkippedRecorded`: `{skipped, runtime_not_installed}` is stored, with no warning.
  - `TestOrchestratorWithoutSmoker`: `Smoke` nil.
  - `TestOrchestratorCancelDuringSmoke`:
    - drive to `verifying`;
    - the smoker blocks until its context ends and signals that it started;
    - a goroutine calls `fx.o.Cancel`;
    - the `Step` returns progressed, the smoker saw `context.Canceled`;
    - `drive` ends `cancelled` with `ErrorCode` `cancelled_by_user` (the `codeCancelled` value) and `Smoke` nil.
  - `TestOrchestratorRestartDuringSmoke`:
    - `Step` with a context cancelled during the smoke returns an error and the state stays `verifying`;
    - `fx.restart()` and `SetSmoker` again;
    - `drive` gives succeeded, and the smoker ran twice.
  - `TestOrchestratorSmokeTextCut`: 500 characters are stored as 200 runes.

### Commands

```
go build ./...
go vet ./internal/chibisensei/
go test ./internal/chibisensei/... -count=1
go test ./internal/chibisensei/ -count=1 -run '(?i)(Orchestrator|Smoke|Samples|RunModels|ReportSeen|Steps|RuntimePin|ProfileSummary)'
```

**Done when:** all of these pass, `chibisensei` imports neither `chatrt` nor `internal/server` in production code, and the goldens are unchanged (`job.json` and the notebook do not depend on the new fields).

---

## B4: server routes, wiring, tests, browser smoke, docs and DOX

**Create:**

| File | Content |
|---|---|
| `internal/server/chibi_sensei_runtime.go` | wiring helpers, runtime view, `GET /runtime`, install, stop, `chibiRuntimeError` |
| `internal/server/chibi_sensei_chat.go` | the chat stream handler |
| `internal/server/chibi_sensei_chat_request.go` | decode and validation |
| `internal/server/chibi_sensei_test_view.go` | test view, samples, report seen |
| `internal/server/chibi_sensei_smoke.go` | `chibiSmoker` |
| `internal/server/chibi_sensei_runtime_test.go` | |
| `internal/server/chibi_sensei_chat_test.go` | |
| `internal/server/chibi_sensei_test_view_test.go` | |

New git-ignored files the coordinator force-adds: `internal/chibisensei/chatrt/AGENTS.md`, `internal/llamacpp/AGENTS.md`.

**Modify:**
- `chibi_sensei_service.go`, `chibi_sensei_handlers.go`, `desktop_operation.go`, `server.go`
- `desktop_audit_lifetime_test.go`, `chibi_sensei_app_browser_test.go`
- `documentation/chibi-sensei.md`
- the guides listed in Task B4.6
- the three licence files of Contract changes 4
- `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md:3` (git-ignored, already tracked)

### Task B4.1: wiring

- [ ] `server.go:407` area: `// chibiSenseiLauncherOverride replaces the chat runtime launcher of the next opened runtime in tests (read in openChibiRuntime).` and `chibiSenseiLauncherOverride chatrt.Launcher`.
- [ ] `chibiSenseiRuntime` (`chibi_sensei_service.go:34`) gains `chat *chatrt.Manager` and `chatMu sync.Mutex; chatRuns map[string]string` (instance key → the run that used it last).
- [ ] `openChibiRuntime` (`:72`), after the orchestrator is built (`:95-98`):
  - `launcher := chatrt.NativeLauncher()`, or the override;
  - `chat := chatrt.NewManager(s.chibiChatConfig(dataDir), launcher, s.Logger)` (nil logger → `slog.Default()`);
  - `orch.SetSmoker(chibiSmoker{chat: chat})`;
  - `chat.Start(ctx)` with `s.integrationCtx`, falling back to `context.Background()`;
  - store `chat` and an empty `chatRuns` on `rt`.
- [ ] `chibiChatConfig(dataDir)`:
  - returns a func that normalizes a copy of `ConfigSnapshot().ChibiSensei` (ignoring the error, like `chibiOrchestratorConfig` `:131-149`);
  - `Enabled = cs.Enabled && cs.Runtime.Enabled && cfg.VirtualDesktop.Enabled`;
  - copies backend, idle minutes and context size;
  - `CacheDir = filepath.Join(chibisensei.ChibiDir(dataDir), "runtime")`.
- [ ] `finishChibiRetire` (`:235`): `rt.orch.Close()`, then `rt.chat.Close()`, then `rt.jobs.Close()`. The orchestrator closes first because it ends a smoke; shutdown (`closeChibiSensei` `:260`) goes through the same function.
- [ ] `chibiDomainError` (`:433`) gains `ErrRunNotSucceeded` → 409 `run_not_succeeded` and `ErrSamplesMissing` → 404 `samples_missing`.
- [ ] `handleChibiMeta` (`:516`) adds:

```go
"runtime": map[string]any{"enabled": acc.cfg.ChibiSensei.Runtime.Enabled, "backend": acc.cfg.ChibiSensei.Runtime.Backend, "build": llamacpp.Build},
"chat":    map[string]any{"max_targets": chibiChatMaxTargets, "max_turns": chibiChatMaxTurns, "max_chars": chibiChatMaxChars, "max_tokens_cap": chibiChatMaxTokensCap},
```

  Profiles already carry `sampling` and `max_new_tokens` (B3.5).
- [ ] `chibi_sensei_smoke.go` (`chibiSmoker`):
  1. `Admit(in.ModelKey)` gives the skipped codes of Contract changes 8.
  2. `Acquire` maps the same way; any other error → `failed/start_failed`; then `defer Release`.
  3. `Chat` with `PrepareMessages(prof.Chat.SystemRole, [{user, "Hello! Please introduce yourself in one sentence."}])`, `chibiProfileSampling(prof, 16)` and `Stop: prof.Chat.Stop`; text collected up to 2000 bytes.
  4. An error → `failed/generation_failed`, logged with `security.Scrub`.
  5. Otherwise `ok` with `MS` and `Text`.
- [ ] `chibiProfileSampling(p *chibisensei.Profile, maxTokens int) chatrt.Sampling` copies the five values.

### Task B4.2: routes and operation rule

- [ ] `handleChibiSensei` (`chibi_sensei_handlers.go:21-82`), before `default`: `case len(parts) == 2 && parts[0] == "runtime":`, then a switch on `parts[1]`:
  - `""`: not reachable;
  - `chat` → `handleChibiChat`;
  - `install` → `handleChibiRuntimeInstall`;
  - `stop` → `handleChibiRuntimeStop`.
- [ ] Plus `case len(parts) == 1 && parts[0] == "runtime": handleChibiRuntime`.
- [ ] `handleChibiRunRoute` (`:168-200`):
  - `rest[0] == "test"` → `handleChibiRunTest`;
  - `"samples"` → `handleChibiRunSamples`;
  - `case len(rest) == 2 && rest[0] == "report" && rest[1] == "seen": handleChibiReportSeen`.
- [ ] `desktopRequestOperation` (`desktop_operation.go:39`): add `|| path == "/api/desktop/chibi-sensei/runtime/stop"` to the `desktopStop` list next to `path == "/api/desktop/rtl-sdr/stop"` (`:49`). It is the only new rule.

**Handlers:**

| Route | Authorize | Notes |
|---|---|---|
| `GET /runtime` | read scope, `desktopRead` | `{"runtime": chibiRuntimeView(…)}` |
| `POST /runtime/install` | write scope, `desktopWrite`, `chibiSameOrigin`, then `chibiRequestIsAdmin` (403 `admin_required`) | `ctx, cancel, err := s.beginDesktopBackgroundRun(chibiRuntimeInstallTimeout /* 60 min */)`; `done, err := rt.chat.BeginInstall(ctx)`; an error → `cancel()` and `chibiRuntimeError`; otherwise `go func(){ <-done; cancel() }()` and 202 with the view. The download outlives the request, a read-only switch or shutdown cancels it, and the runtime's `Close` cancels it. |
| `POST /runtime/stop` | write scope, `desktopStop`, same-origin | `rt.chat.StopAll(r.Context())`, 200 with the view |
| `GET /runs/{id}/test` | read | load the run; not succeeded → 409; profile from the catalog (missing → `chibiProfileUnknown`); `report` via `GetReport` (null when absent); `samples_total` from `ReadSamples(…, 0, 1, "all").Total` (0 on ErrSamplesMissing); `models` from `RunModels`; `chat = {system_role, sampling (profile), max_tokens: Eval.MaxNewTokensSpecialist when the recipe from RunTask is specialist, else MaxNewTokensAssistant}`; `run` = summary; `report_seen`. A `RunTask` error gives recipe assistant. |
| `GET /runs/{id}/samples` | read | `offset` ≥ 0; `limit` 1..50, default 20, otherwise 422 `validation_failed` via `chibiValidation(w, "limit", "chat.invalid", …)`; `filter` ∈ all/different/wrong, otherwise 422; recipe, task and labels from `RunTask`; answers `SamplePage` |
| `POST /runs/{id}/report/seen` | write scope, `desktopRead` (Contract changes 7), same-origin | `MarkReportSeen`; 204; ErrNotFound → `chibiRunNotFound` |

`chibiRuntimeView(s, r, acc)` builds the contract's `Runtime` JSON from `rt.chat.Status(ctx)`:
- `admin` = `chibiRequestIsAdmin`;
- `backend.requested` = `cfg.Backend`;
- each instance gets `kind` (`tuned`/`base` from the prefix) and `run_id` (`tuned:` suffix, or `rt.chatRuns[key]`);
- `limits` = `{max_instances: chatrt.MaxInstances, idle_minutes, context_size}`;
- `notice_code` = `"gpu_fallback"` when `Fallback`.

`chibiRuntimeError(w, err)`:
- ErrDisabled, ErrUnsupported, ErrClosed → 503 `runtime_unavailable`;
- ErrNotInstalled → 409 `runtime_not_installed`;
- ErrInstalling, ErrBusy → 409 `runtime_busy`;
- ErrModelMissing → 404 `model_missing`;
- anything else → 503 `runtime_unavailable` (logged, scrubbed).

### Task B4.3: the chat stream (`chibi_sensei_chat.go`, pattern `handleChibiEvents` `chibi_sensei_jobs.go:90-192`)

**Constants:**

```go
const (
	chibiChatMaxTargets   = 2
	chibiChatMaxTurns     = 20     // user messages per conversation
	chibiChatMaxMessages  = 64
	chibiChatMaxChars     = 16000  // runes per conversation, system included
	chibiChatMaxTokensCap = 1024
)
```

**Validation** (`chibi_sensei_chat_request.go`). The request is decoded with `chibiDecode(w, r, &req, chibiJSONLimit)` into:
- `{run_id string; targets []{kind string; messages []chibisensei.Message}; sampling *{temperature *float64; top_p *float64; top_k *int; max_tokens *int; seed *int64}}`

All failures answer 422 `validation_failed` with key `chibi_sensei.chat.invalid`:
- `run_id` must pass `ValidID`.
- 1 or 2 targets; kind ∈ {tuned, base}, each at most once.
- Roles: an optional single leading system message, then user and assistant alternating, starting and ending with user.
- Every content is non-empty after trimming and contains no NUL.
- ≤ 20 user messages, ≤ 64 messages, ≤ 16,000 runes in total.
- temperature 0..2; top_p in (0, 1]; top_k 0..200; max_tokens 1..1024; seed ≥ 0.

**Before the stream** (ordinary JSON errors):
1. `chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)` (D10; the request context is a desktop run, so turning the desktop read-only cancels it) and `chibiSameOrigin`;
2. decode and validate;
3. `chibiLoadRun` (404 `run_not_found`);
4. succeeded, otherwise 409 `run_not_succeeded`;
5. the profile from the catalog;
6. `RunModels`: a requested kind that is not available → 404 `model_missing` with the message "The <kind> model file is not available.";
7. `rt.chat.Admit(keys...)` → `chibiRuntimeError`.

Then record `rt.chatRuns[baseKey] = runID` under `chatMu`.

**Stream** (`200 text/event-stream; charset=utf-8`, `Cache-Control: no-store`, `X-Accel-Buffering: no`). Skeleton (binding shape):

```go
rc := http.NewResponseController(w)
_ = rc.SetReadDeadline(time.Time{})
arm := func() { _ = rc.SetWriteDeadline(time.Now().Add(chibiEventWriteTimeout)) }
genCtx, cancelGen := context.WithCancel(r.Context()) // client close or read-only switch end both generations
defer cancelGen()
events := make(chan chibiChatEvent, 64) // {name string; data any}
quit := make(chan struct{})               // closed when the writer returns; emit never blocks after that
emit := func(e chibiChatEvent) { select { case events <- e: case <-quit: } }
var wg sync.WaitGroup
for _, t := range targets { wg.Add(1); go func(t chibiChatTarget) { defer wg.Done(); s.runChibiChatTarget(genCtx, acc.rt, t, emit) }(t) }
go func() { wg.Wait(); close(events) }()
defer func() { close(quit); cancelGen(); waitTimeout(&wg, 10*time.Second) }()
// single writer: ": chibi-sensei chat" comment, then loop:
//   ev, open := <-events → !open: send "end" {} and return; else send it, drain up to 31 more without blocking, flush once
//   heartbeat (chibiEventHeartbeat) → ": ping\n\n" + flush
//   rt.retiring or serverDone → cancelGen() and keep reading, so the targets' error events and "end" still go out
//   any write or flush error → return
```

Each write is `fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, json)` after `arm()`.

**`runChibiChatTarget`**, every target ending with exactly one `done` or `error`:
1. `emit status {target, state: "loading"}`.
2. `Acquire(genCtx, ModelRef{Key, Path, Bytes})`. ErrBusy or ErrInstalling → `runtime_busy`; a context error → `cancelled`; anything else → `start_failed`.
3. `defer Release`; `emit status generating`.
4. `msgs := chatrt.PrepareMessages(profile.Chat.SystemRole, target.messages)`.
5. `Sampling` = profile values with the request overrides; `MaxTokens` = override or `chat.max_tokens`, capped at 1024; `Stop` = `profile.Chat.Stop`.
6. `inst.Chat(genCtx, req, func(t string){ emit delta {target, text: t} })`.
7. Errors: ErrContext → `context_exceeded`; a context error or ErrStopped → `cancelled`; anything else → `generation_failed`. The raw error is logged scrubbed.
8. Success: `done {target, finish_reason, prompt_tokens, completion_tokens, ms, tok_s: math.Round(x*10)/10, backend}`.

Error messages are fixed English sentences. Model text is forwarded unchanged and is untrusted; the UI renders it with `textContent`.

### Task B4.4: tests (`internal/server`)

`chibiSucceededTestRun(t, rt, pid, profileID string, withBase bool, samples int) string` (in `chibi_sensei_test_view_test.go`) is modelled on `chibiStoredRun` (`chibi_sensei_runs_test.go:136-179`) with these differences:
- `JobJSON` is `{"recipe":"assistant","task_kind":null,"labels":[],"profile":{"base":{"revision":"<40 hex>"}},"deps":{"llama_cpp":{"tag":"b9994"}}}`;
- `Remote.GGUF` is `{3, 1, arch}`;
- the tuned and base files are written with `chibiTinyGGUF(arch)` (a copy of `fakeGGUF`, `fakekaggle_test.go:1239-1263`), and the base as `base/<profile>-<rev12>-llamacpp-b9994.gguf` with its artifact;
- `runs/<id>/out/eval_samples.jsonl` holds `samples` lines (`expected`, base and tuned answers, one different);
- a report.

All tests use `s.chibiSenseiLauncherOverride = chatrttest.New(true)`, set right after `chibiAPIFixture` and before the first request. Admin requests use token `""` with `Origin: http://example.com` (`chibi_sensei_preflight_test.go:162`).

- `TestChibiSenseiRuntimeRoutes`:
  - not installed: state `not_installed`, `download.bytes` 15857307, host `github.com`, `limits`, `build`;
  - install as the write token → 403 `admin_required`; foreign origin → 403 `csrf_check_failed`;
  - as admin → 202 `installing`, then polled to `ready`;
  - on a read-only desktop: stop → 200, install → 403 `desktop_readonly`;
  - `runtime.enabled: false`: GET state `disabled`, install → 503 `runtime_unavailable`.
- `TestChibiSenseiTestView`:
  - a failed run → 409 `run_not_succeeded`;
  - with base: both models available with bytes; without base: base `skipped`;
  - `chat.max_tokens` 256, `samples_total`, report, `report_seen` false;
  - the JSON has no `path` and no `key`.
- `TestChibiSenseiSamplesRoute`: `limit=2` gives two samples with `total`; `filter=different`; limit 0 and 51 → 422; `filter=x` → 422; no file → 404 `samples_missing`.
- `TestChibiSenseiReportSeen`: 204; the project then shows `steps.test` `done`; idempotent; works on a read-only desktop; a failed run → 409.
- `TestChibiSenseiChatValidation`: a table over every rule of B4.3 → 422 `validation_failed`; an unknown field → 400 `invalid_request`.
- `TestChibiSenseiChatPreStreamErrors`:
  - 404 `run_not_found`; 409 `run_not_succeeded`;
  - base without a file → 404 `model_missing`;
  - not installed → 409 `runtime_not_installed`; disabled → 503 `runtime_unavailable`;
  - two other models held with `Hold` → 409 `runtime_busy`;
  - read-only → 403 `desktop_readonly`.
- `TestChibiSenseiChatStreamsBothTargets`:
  - headers;
  - per target: `status loading`, `status generating`, deltas joining to "Tuned answer." and "Base answer.", one `done` with `backend` cpu;
  - `end` last;
  - launches keyed `tuned:<id>` and `base:base/…`;
  - a gemma3 run's system message arrives merged in the fake's request (`Requests("tuned:")`).
- `TestChibiSenseiChatOneTargetFails`: `FailLaunch["base:"]` → base `error start_failed`, tuned `done`, then `end`.
- `TestChibiSenseiChatContextExceeded`: `ChatErr["tuned:"] = chatrt.ErrContext` → `context_exceeded`.
- `TestChibiSenseiChatClientCloseCancels`: `httptest.NewServer(handleChibiSensei(s))`; the client closes after the first `status` event; `Cancelled()` receives the key within 2 s.
- `TestChibiSenseiChatRetireStopsInstances`: `closeChibiSensei` → `Stopped()` ≥ 1.
- `TestChibiSenseiMetaRuntimeAndChat`: `runtime.build` b9994, `chat.max_targets` 2, profile `sampling.top_k` 20.
- `TestChibiSenseiSmokerAdapter`: not installed → `skipped runtime_not_installed`; installed → `ok` with text; busy → `skipped runtime_busy`; `ChatErr` → `failed generation_failed`.
- `TestDesktopReadonlyCleanupRoutes` (`desktop_audit_lifetime_test.go:12`): add `/api/desktop/chibi-sensei/runtime/stop` to the stop list (`:32`), and `/runtime/chat`, `/runtime/install`, `/runtime/stop/extra` to the not-stop list (`:37`).

**Browser smoke** (`TestChibiSenseiAppRealServerBrowser`, `chibi_sensei_app_browser_test.go:159`):
- Set `s.chibiSenseiLauncherOverride = chatrttest.New(true)` after `chibiAPIFixture` (`:163`).
- After step 9 (`:252`), add:
  10. Seed: `rt, release := s.acquireChibiSensei()`; `CreateProject("Test project", assistant, qwen35-0p8b)`; `runID := chibiSucceededTestRun(t, rt, pid2, "qwen35-0p8b", true, 3)`; `release()`.
  11. Click `[data-csn="back"]`; wait for `[data-csn="project-card"][data-id="<pid2>"]`; click it; wait for `[data-csn="project-name"]` with the text "Test project".
  12. Wait for `[data-csn="step"][data-step="test"][data-state="ready"]`; click it; wait for `[data-csn="test"]`.
  13. Click `[data-csn="test-view"][data-view="report"]`; wait for `[data-csn="report-card"]`.
  14. Click `[data-csn="test-view"][data-view="samples"]`; wait for exactly 3 `[data-csn="sample"]`.
  15. Click `[data-csn="test-view"][data-view="chat"]`; wait for `[data-csn="runtime-strip"][data-state="ready"]` and an enabled `[data-csn="chat-compare"]`.
  16. Click `[data-csn="chat-compare"]`; wait for `[data-csn="chat-column"][data-target="tuned"]` and `[data-target="base"]`.
  17. `csnbType` "Hello there" into `[data-csn="chat-input"]`; click `[data-csn="chat-send"]`.
  18. Wait for `[data-csn="chat-answer"][data-target="tuned"][data-state="done"]` with text containing "Tuned answer.", and the same for base with "Base answer.".
  19. Run the error and `failedChibi` checks of step 7 again.
  20. API: `GET /runs/<runID>` has `"report_seen":true`; `GET /projects/<pid2>` has `"test":"done"`; `Launches()` has 2 entries.

### Task B4.5: documentation

- [ ] `documentation/chibi-sensei.md`: add `## Testing your model` after the `### When a run fails` block (before `## Storage`, `:195`), with these parts:
  - report card (numbers, small-sample note, base availability, the one-line smoke result);
  - examples (filters, match marks, "Try in chat");
  - chat and comparison (separate conversations, profile defaults, advanced values, stats line, reasoning always off, the training prompt format reproduced);
  - the local runtime:
    - the one-time administrator download: source GitHub `ggml-org/llama.cpp` release b9994 (CUDA on Linux: `hybridgroup/llama-cpp-builder`), the sizes from the table in `facts-llama.md`;
    - stored in `data/chibi_sensei/runtime/`, which counts toward `storage.max_gb`;
    - backends and auto order; GPU fallback notice;
    - two models at once, idle stop, "Stop models";
    - loopback only, a random key in the environment;
  - read-only desktop behaviour;
  - the automatic check after a run (`smoke_failed` warning, never a failure, never a download);
  - a table of the new HTTP and stream error codes with what to do.
- [ ] Also in that file:
  - add a `smoke_failed` row to the failure and warning table of `### When a run fails`;
  - add the four `runtime` keys to `## Settings` (`:199`);
  - mention `runtime/` in `## Storage`;
  - add to `## Privacy and security`: chat text never leaves the machine, model files come only from the run's registered artifacts.
- [ ] `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md:3`: "P1 (dataset studio), P2 (training on Kaggle) and P3 (test and compare, native runtime only; the Docker sidecar, the provider column and the LLM judge are follow-ups) are implemented".

### Task B4.6: DOX

- [ ] **New `internal/llamacpp/AGENTS.md`** (Purpose, Ownership, Local Contracts, Work Guidance, Verification, Child DOX Index):
  - Purpose: the shared pinned llama.cpp build.
  - Ownership: the B1 files. Consumers are `internal/embeddings` (wrappers, Docker sidecar stays there) and `internal/chibisensei/chatrt`. It imports only onnxrt.
  - Local Contracts:
    - a pin change updates build, all asset sizes, SHAs and images together, plus the trainer lock (`TestRuntimePinMatchesTrainerLock`) and the licence rows;
    - error texts and evidence order are part of the embeddings fingerprint (`DetectHardwareWith`);
    - `SupplementalAssets` is the only source of the Windows CUDA archive.
  - Verification: the B1 commands.
  - Child DOX Index: None.
- [ ] **New `internal/chibisensei/chatrt/AGENTS.md`**, same sections:
  - Ownership: the B2 files and `chatrttest`. It never imports chibisensei.
  - Local Contracts:
    - nothing downloads before `BeginInstall`;
    - the own cache root;
    - the install set;
    - argv without secrets, key in `LLAMA_API_KEY`, `LLAMA_*` stripped;
    - `127.0.0.1`, `--no-webui`, `--no-slots`, `--reasoning off` plus `enable_thinking:false`;
    - two instances, LRU idle eviction, the busy and wait rules, the 30 s reaper;
    - sticky GPU fallback;
    - the lock rule;
    - child lifetime per OS;
    - the tolerant stream parser with fixtures from discovery;
    - the audit inventory row.
  - Verification: the B2 commands plus the gated smoke with its environment variables and `-timeout 20m`.
- [ ] `internal/chibisensei/AGENTS.md`:
  - Purpose and Ownership gain `samples.go`, `testview.go`, `reconcile_smoke.go` and "never imports `chatrt`";
  - Local Contracts gain the `report_seen` flag in `remote_json`, the `steps.test` rule, the smoke rules (warning only, aborted by cancel, repeated after a restart, at most 200 scrubbed characters, `smoke_failed` warn event) and D6 for `RunModels`;
  - Child DOX Index: `chatrt/AGENTS.md`.
- [ ] `internal/embeddings/AGENTS.md`, Ownership: the pins, asset table, server discovery, loopback, key, GPU evidence and hardware detection live in `../llamacpp`. Embeddings keeps unexported wrappers so its tests are unchanged, and keeps the Docker sidecar and `selectDockerModelMount`.
- [ ] `internal/onnxrt/AGENTS.md`, Asset cache bullet: add "Chibi Sensei's chat runtime uses its own root `<data>/chibi_sensei/runtime` (no shared GPU cleanup)". Keep the strings `TestPersonDetectionContractsAreRouted` requires (`person_detection_docs_test.go:29-31`).
- [ ] `internal/server/AGENTS.md`, Chibi Sensei section (`:314-321`): one P3 bullet covering the routes, D10 classes, the report-seen read class, the stream writer and deadlines, `chibiSenseiLauncherOverride`, retire order, `chibiSmoker`. Extend the Verify line with `go test ./internal/chibisensei/chatrt/...`.
- [ ] Root `AGENTS.md`:
  - The Chibi Sensei routing row gains `internal/chibisensei/chatrt/AGENTS.md`, `internal/llamacpp/AGENTS.md` and the scope words "local llama.cpp test chat (report card, examples, compare), smoke check".
  - New row: `| Shared llama.cpp build | \`internal/llamacpp/AGENTS.md\`, \`internal/embeddings/AGENTS.md\`, \`internal/chibisensei/chatrt/AGENTS.md\` | b9994 pins, release assets, discovery, GPU evidence, hardware detection shared by embeddings and the Chibi chat runtime. |`
- [ ] Intentionally unchanged: `prompts/**` (no agent tool in P3), `internal/config` has no guide, `ui/**` guides (frontend chain).

### Commands

```
git diff --check
go build ./...
go vet ./internal/server/
go test ./internal/server -run '(?i)(ChibiSensei|DesktopReadonlyCleanupRoutes)' -count=1 -timeout 10m
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestChibiSenseiAppRealServerBrowser$' -count=1 -timeout 15m   # needs F1-F3 in the tree
go test ./internal/llamacpp/... ./internal/embeddings/... ./internal/config/... ./internal/chibisensei/... -count=1
go test ./internal/audit/ -count=1 -run '(?i)(PersonDetection|NetworkClientInventory|CredentialedHTTPClients|SubprocessLaunches)'
```

**Done when:**
- all of these pass, the browser smoke included once F1 to F3 are in;
- no handler logs or returns a model path, key or raw upstream error;
- the two new `AGENTS.md` files are listed for force-add.

---

## Existing symbols changed

| Symbol | file:line | Batch | Change |
|---|---|---|---|
| `llamaRuntimeBuild`, `llamaDocker{CPU,CUDA,Vulkan}Image` | `internal/embeddings/manifest.go:13,22-24` | B1 | alias consts of `llamacpp` |
| `runtimeManifest` | `internal/embeddings/manifest.go:64` | B1 | `= llamacpp.Runtimes()` |
| `llamaRuntimeAsset` | `internal/embeddings/manifest.go:175` | B1 | wrapper |
| `hardwareInfo` | `internal/embeddings/candidates.go:82` | B1 | type alias |
| `detectHardware` | `internal/embeddings/candidates.go:90` | B1 | wrapper over `DetectHardwareWith(ctx, augmentDockerHardware)` |
| `shortCommand` | `internal/embeddings/candidates.go:129` | B1 | moved to `llamacpp` |
| `vulkanLoaderEvidence` | `internal/embeddings/vulkan_loader_windows.go:12`, `vulkan_loader_other.go:5` | B1 | moved (files deleted) |
| `findLlamaServer`, `llamaGPUVerified`, `availableLoopbackPort`, `randomAPIKey` | `internal/embeddings/llama_embedder.go:344,369,381,394` | B1 | one-line wrappers |
| `llamaDockerImage` | `internal/embeddings/llama_docker.go:340` | B1 | wrapper |
| `ChibiSenseiConfig` | `internal/config/chibi_sensei.go:12` | B1 | new `Runtime` field |
| `NormalizeChibiSenseiConfig` | `internal/config/chibi_sensei.go:57` | B1 | backend and two ranges |
| `load` | `internal/config/config.go:398` (defaults at `:606-608`) | B1 | `Runtime.Enabled = true` |
| `TestChibiSenseiDefaults` | `internal/config/chibi_sensei_test.go:22` | B1 | `want` gains `Runtime` |
| `NetworkClientInventory` | `internal/audit/audit_manifests.go:236` | B2 | one row |
| `RunRemote` | `internal/chibisensei/runs.go:153` | B3 | `Smoke`, `ReportSeen` |
| `RunView`, `Run.view` | `internal/chibisensei/runs.go:321,355` | B3 | `smoke`, `report_seen` |
| `Orchestrator` (struct) | `internal/chibisensei/reconcile.go:83` | B3 | smoker and smoke-cancel fields |
| `(*Orchestrator).Cancel` | `internal/chibisensei/reconcile.go:237` | B3 | `abortSmoke` after the commit |
| `(*Orchestrator).stepVerifying` | `internal/chibisensei/reconcile_outputs.go:455` | B3 | smoke between `:492` and `:493` |
| `scanProject`, `projectSelect`, `projectRuns`, `computeSteps` | `internal/chibisensei/projects.go:159,186,227,232` | B3 | report-seen column and `test` rule |
| `ProfileSummary`, `(*Profile).Summary` | `internal/chibisensei/profiles.go:116,210` | B3 | `sampling`, `max_new_tokens` |
| `computeSteps` test table | `internal/chibisensei/runs_test.go:398` | B3 | `Test` cases |
| `Server` (struct fields) | `internal/server/server.go:404-408` | B4 | `chibiSenseiLauncherOverride` |
| `chibiSenseiRuntime` | `internal/server/chibi_sensei_service.go:34` | B4 | `chat`, `chatMu`, `chatRuns` |
| `openChibiRuntime` | `internal/server/chibi_sensei_service.go:72` | B4 | manager, smoker, reaper start |
| `finishChibiRetire` | `internal/server/chibi_sensei_service.go:235` | B4 | `chat.Close()` between orchestrator and runner |
| `chibiDomainError` | `internal/server/chibi_sensei_service.go:433` | B4 | two error cases |
| `handleChibiMeta` | `internal/server/chibi_sensei_service.go:516` | B4 | `runtime`, `chat` |
| `handleChibiSensei` | `internal/server/chibi_sensei_handlers.go:21` | B4 | `runtime` routes |
| `handleChibiRunRoute` | `internal/server/chibi_sensei_handlers.go:168` | B4 | `test`, `samples`, `report/seen` |
| `desktopRequestOperation` | `internal/server/desktop_operation.go:39` | B4 | `runtime/stop` is `desktopStop` |
| `TestDesktopReadonlyCleanupRoutes` | `internal/server/desktop_audit_lifetime_test.go:12` | B4 | new paths |
| `TestChibiSenseiAppRealServerBrowser` | `internal/server/chibi_sensei_app_browser_test.go:159` | B4 | test-and-compare path |

## Open risks (only the real binary or a real install can settle)

1. **The stream framing of `/v1/chat/completions` at b9994** is unconfirmed: whether `usage` and `timings` arrive in the stream with `stream_options.include_usage`, and whether `max_tokens` is honoured on this endpoint. B2.1 records it; until then the fixtures are provisional.
2. **The over-long-prompt answer is unconfirmed:** an HTTP 400 before the stream or a `data: {"error":…}` event, and its `type` and message. `classifyUpstream` matches tolerantly, but the exact text decides whether `context_exceeded` or `generation_failed` shows.
3. **Whether the upstream generation stops when the client closes the connection** with `--parallel 1` (B2.1 phase 7). If it does not, a cancelled answer holds the slot until `max_tokens`.
4. **`--reasoning off` with `--jinja` on the Qwen3.5 template** must give the non-thinking prompt. `--no-slots` and `--threads-batch` must exist at b9994. The `--help` capture confirms the flags, not the template behaviour.
5. **GPU evidence strings** (`offloaded`, `using device`) on Vulkan and CUDA at b9994, and whether `--fit` (default on) changes `-ngl 999`. Only a GPU host can show this; `aurago-test` has no NVIDIA card.
6. **The glibc and library needs of the downloaded Linux binaries** on `aurago-test` and inside AuraGo's Debian bookworm image (no `libgomp`/`libvulkan` named in `Dockerfile:64-70`).
7. **Orphaned children:** a `kill -9` of a hand-started AuraGo on Linux or macOS leaves `llama-server` running on loopback until someone stops it. systemd and Docker clean up.
8. **The disk rule** (4× archive bytes + 64 MiB) and the runtime's share of `storage.max_gb` (the CUDA set on Windows is about 640 MB) need a check against real extracted sizes.
9. **Model load time and memory** of two 1.2 GB instances side by side on a 12-core CPU host, and whether half the cores each gives acceptable speed. This needs the test-server acceptance run.
