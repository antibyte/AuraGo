# Chibi Sensei P3 (test and compare) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship phase P3 of Chibi Sensei: for a succeeded run the user opens step 4 "Test", reads the report card and the evaluation examples, chats with the tuned model and compares it side by side with the base model, both served by a local llama.cpp runtime that AuraGo installs once and supervises.

**Architecture:** The shared llama.cpp pins and helpers move from `internal/embeddings` into the new `internal/llamacpp`. A supervisor in `internal/chibisensei/chatrt` installs the pinned `llama-server` (build b9994) into its own cache, starts at most two loopback child processes and streams chat completions from them. The domain package gains the example views, the model lookup, the report-seen flag and a smoke seam in the orchestrator's `verifying` state. The HTTP layer adds the runtime routes, the streamed chat and the test view under `/api/desktop/chibi-sensei`; the UI adds the Test step with two modules (test view, chat).

**Tech Stack:** Go (net/http, SSE over a POST answer, os/exec child processes, modernc SQLite), llama.cpp `llama-server` b9994 (downloaded release asset, sha256-pinned), vanilla JS classic scripts with a `fetch` stream reader, CSS with `--vd-mat-*` tokens, 16 locale JSON files.

Design spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`. Earlier plans: `docs/superpowers/plans/2026-11-22-chibi-sensei-p1.md`, `docs/superpowers/plans/2026-11-23-chibi-sensei-p2.md`.

This plan has three files:

- this index: conventions, impact analysis, batch order, the coordinator contract (decisions, Go API between batches, HTTP contract additions, UI behaviour, amendments);
- `2026-11-27-chibi-sensei-p3-backend.md`: batches B1 to B4;
- `2026-11-27-chibi-sensei-p3-frontend.md`: batches F1 to F3.

Each half starts with its own "Contract changes" list. The index's "Amendments" section collects the ones that bind other batches; where a half and the amendments differ, the amendments win.

---

## Conventions for every batch

**Where to work.** Only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\xenodochial-blackwell-7e567d` (branch `claude/xenodochial-blackwell-7e567d`). Never read or write the main checkout at `C:\Users\Andi\Documents\repo\AuraGo` outside that folder: other sessions work there.

**Who commits.** Implementers do not commit, stage, stash, rebase or switch branches. The coordinating session reviews each batch and commits its paths.

**Ignored files.** `.gitignore` ignores `AGENTS.md`, `docs/` and `*.lock`, among others. Before a commit the coordinator runs `git status --short --ignored -- <paths>` and force-adds every new file that shows as `!!` (`git add -f`), then checks with `git ls-files`.

**One batch, one dispatch** where the batch fits. One combined review per pair of batches reports only Critical and Important issues; minor items are collected and not fixed per batch.

**Tests.** Write the tests the task names, plus one test for each bug you fix on the way. No property, mutation or stress suites, no `-race` runs. No test needs a GPU, a network or a real `llama-server`, except the one gated real smoke test.

**Go commands** (Git Bash, from the worktree root):

- `go build ./...` must stay green after every batch.
- Run only the packages a task names. For `-run`, group patterns as `(?i)(a|b)`.
- If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`.
- Do not set `GOTMPDIR` or `GOCACHE` yourself.

**UI commands.** `node --check <file>` for every changed JS file, `node scripts/test-chibi-sensei-model.mjs`, `npm run check:ui` before reporting done. The Chibi Sensei modules are classic scripts loaded one by one; a new module needs no bundle rebuild.

**Text encoding.** Write real UTF-8 characters (umlauts, CJK) into source and JSON files. Never write `\u` escape sequences through the file-writing tool: it decodes them. LF line endings, no BOM.

**UI rules.** `textContent` only (no `innerHTML`, no template strings, no `eval`, no storage, no browser dialogs), no external assets, CSS classes start with `csn-`, each JS module stays under 1100 lines, every user-visible string is a translated key, `fetch`, `EventSource` and stream-reader code only in `chibi-sensei-api.js`.

**Scratch files** go to `C:\disposable\chibi-sensei\p3\`, never into the repository.

**Processes.** Stop only processes you started, by PID or handle. Never kill by image name.

**Code intelligence.** The coordinating session runs the impact analysis for existing symbols before a batch starts and the change analysis before each commit; implementers do not run GitNexus. A worktree cannot be indexed, so the analysis runs against the index of the main checkout.

**Report format.** End with: files created, files modified, the exact test commands run with their PASS/FAIL lines, deviations from the plan with the reason, and anything left open.

## Impact analysis of existing symbols (2026-11-27)

Run against the index of the main checkout (at 018b68005, seven commits behind main's tip; none of those commits touches the symbols below).

| Symbol | Risk | Change |
|---|---|---|
| `config.NormalizeChibiSenseiConfig` | CRITICAL, 76 impacted, reaches `main` and `Start` through the config load and the config save route | normalizes four new fields of `chibi_sensei.runtime` |
| `desktopRequestOperation` (internal/server) | HIGH, 166 impacted (every desktop route) | one new path rule: `POST …/chibi-sensei/runtime/stop` is a stop operation |
| `llamaRuntimeAsset`, `detectHardware` and the other moved helpers of `internal/embeddings` | LOW, embeddings only (4 and 9 impacted) | bodies move to `internal/llamacpp`; thin wrappers stay |
| `chibisensei.computeSteps` | LOW, 18 impacted (Chibi only) | rule for `steps.test` |
| `(*Orchestrator).stepVerifying` | LOW, 4 impacted | optional smoke generation before `cleaning` |
| `(*Server).openChibiRuntime` | LOW, 3 impacted, part of `Start` | builds the chat runtime manager |
| `primaryFor`, `runPrimary` (Chibi UI) | LOW (28 and 3 impacted, Chibi modules only) | Test step primary action |
| `createApi` (Chibi UI) | UNKNOWN in the index (no caller edge: it is read as `P.createApi`); one caller by text search, the composition root | seven new methods |

Because of the CRITICAL and HIGH entries, B1's verification runs the whole `internal/config` and `internal/embeddings` packages, and B4's verification runs the desktop-operation tests, the config-save tests and every Chibi Sensei test of `internal/server`.

## Batch order

1. B1 (llama.cpp extraction and config), with F1 in parallel.
2. B2 and B3 in parallel (different directories), F2 in parallel.
3. B4, F3.
4. Integration: one combined review per pair of batches plus one security pass, the browser smoke with the test-and-compare path, the real runtime proof, rebase onto `main`. Landing, pushing and deploying need the user's yes.

`go test ./ui` has locale tests that fail between F2 and F3 by design (the 14 other locales follow in F3).

---
## Coordinator contract

Written by the coordinating session on 2026-11-27, before the plan parts. It fixes what the parts share: decisions, batches and path ownership, the Go API between batches, the HTTP contract additions and the UI behaviour. Where this file and the spec differ, this file wins; each difference is listed under "Decisions".

Sources to read first:

- Spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`, sections "Test and compare", "Local chat runtime", "Security and privacy", "Frontend structure", "Error handling", "Phases" (P3).
- P2 plan index (conventions, HTTP contract style, execution record): `docs/superpowers/plans/2026-11-23-chibi-sensei-p2.md`.
- Guides: `internal/chibisensei/AGENTS.md`, `internal/embeddings/AGENTS.md`, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` (Chibi Sensei part), `documentation/chibi-sensei.md`.
- Fact sheets of the code as of 2026-11-27 (written by reader agents; check a line number against the file before you put it into a step), all in `C:\disposable\chibi-sensei\p3\`:
  - `facts-llama.md`: the llama.cpp pins, asset cache, native `llama-server` start, hardware detection, Docker sidecar, tests, Dockerfile;
  - `facts-llama-server.md`: flags and HTTP API of `llama-server` at tag b9994, read from its README at that tag;
  - `facts-backend.md`: artifacts, GGUF reader, runs, report, eval samples, `verifying`, profiles, steps, hub, jobs, server structure, config, tests;
  - `facts-frontend.md`: modules, core and api helpers, composition root, steps, run module, CSS, locales, tests, loader, model test, browser smoke, config UI.

### What P3 delivers

Step 4 "Test" of a project works: for a succeeded run the user sees the report card and the evaluation examples (expected, base answer, tuned answer), chats with the tuned model, and compares it side by side with the base model. Both models run in a local `llama-server` (llama.cpp build b9994) that AuraGo downloads once and supervises as loopback child processes. After a run's results are checked, the orchestrator tries one short generation with the tuned model when the runtime is already installed.

### Decisions

Facts checked on 2026-11-27 that the decisions rest on:

- Every reusable piece of `internal/embeddings` (pins, asset table, server discovery, port, key, GPU evidence, hardware detection) is unexported; `internal/llamacpp` does not exist.
- The Docker sidecar of the embeddings is used only when AuraGo itself runs in a container (`dockerEmbeddingSidecarsAvailable`: `DOCKER_HOST` plus `/.dockerenv`). The test server `aurago-test` runs AuraGo natively as a systemd service (x86_64, 12 cores, no NVIDIA GPU), so the native path is the one that can be accepted there.
- A succeeded run keeps `models/<run>/model-q8_0.gguf` (artifact kind `gguf_tuned`). The base GGUF is the shared file `base/<profile>-<rev12>-<converter>.gguf` (artifact kind `gguf_base`, table `base_gguf`). A succeeded run can exist without a base GGUF (trainer warning `base_gguf_skipped`, a base dropped as `base_gguf_invalid`, or a file removed later).
- No route serves the evaluation examples as JSON; `test` and `use` are always `todo`; `chibi_sensei.runtime` does not exist in the config; the UI has no helper for a streamed POST answer.
- `llama-server` at b9994: `--jinja` is on by default, `--no-webui` exists, `-rea, --reasoning [on|off|auto]` exists (default `auto`), the API key can come from the environment variable `LLAMA_API_KEY`, `/health` answers 503 while the model loads and is public, per-request `chat_template_kwargs` exists. The stream framing of `/v1/chat/completions` and the answer to an over-long prompt are not in the README: the first chatrt task reads them from the real binary.

User decisions of 2026-11-27 (asked in a question dialog, do not re-ask):

- U1. **Runtime download.** Nothing is downloaded until an administrator confirms once in the Test step; the dialog names the source, the backend and the size.
- U2. **AuraGo in Docker.** P3 ships the native runtime only (inside a container the same native binary runs in AuraGo's container, as the embeddings fallback does; not verified there); the hardened sidecar mode is a follow-up.
- U3. **Scope.** P3 ships report card, examples, chat and comparison with the base model; the LLM judge and a configured provider as a third chat column are a follow-up after the user's acceptance.

Coordinator decisions:

- D1. **`internal/llamacpp` is extracted, minimal.** It receives the build tag, the native runtime asset table with its lookup, the Windows CUDA supplement, the Docker image lookup, server discovery, the loopback port helper, the API key helper, the GPU evidence check and hardware detection. `internal/embeddings` keeps its unexported names as thin wrappers or aliases, so no embeddings test changes. `selectDockerModelMount` and the sidecar code stay in `internal/embeddings` (they move only when a sidecar mode is built).
- D2. **Own asset cache root** `<data>/chibi_sensei/runtime`, not `<data>/embeddings`: the embeddings manager removes llama GPU runtimes it does not use (`cleanupUnusedGPURuntimes`), and a Chibi runtime must not disappear under a running chat. Cost: a second copy of a 16 to 33 MB archive on hosts that also run local embeddings.
- D3. **Install set.** Installing fetches the CPU runtime always, and the GPU runtime of the resolved backend in addition (plus the Windows CUDA supplement). The CPU runtime is the fallback when a GPU start fails. `GET /runtime` reports the total bytes before anything is downloaded.
- D4. **Reasoning is always off.** Every instance starts with `--reasoning off`, and every chat request sends `chat_template_kwargs: {"enable_thinking": false}`. The trainer trained all three profiles on the non-thinking prompt, so the runtime must use it too. No profile field is added (a profile change would change `ProfileSHA`, `job.json` and the goldens).
- D5. **Training format is reproduced.** For a profile with `chat.system_role == "merge_into_first_user"` the server merges a leading system message into the first user message as `system + "\n\n" + user` before the request (`chatrt.PrepareMessages`), exactly as the trainer does. The profile's `chat.stop` strings are sent as `stop`; the profile's sampling values are the defaults.
- D6. **Model files come only from the artifact registry.** The client names a run and a target kind, never a path. The server resolves the artifact row, checks the relative path with the existing validator, opens it through `os.OpenRoot(ChibiDir)`, requires a regular file of the recorded size and reads the GGUF header again before the first load. The base model of a run is its `gguf_base` artifact; without one, the shared `base_gguf` row for the run's profile, base revision and converter is used when its file exists.
- D7. **At most two instances**, one request per instance. A third model stops the least recently used idle instance; without an idle one the answer is `runtime_busy`. The idle reaper runs every 30 s and stops an instance idle for `runtime.idle_minutes`. `Pin` and `Unpin` of the spec belong to P4 and are not built.
- D8. **Two targets generate at the same time**, each instance with half of the CPU cores (`--threads`, at least 1), so a comparison does not oversubscribe the CPU.
- D9. **GPU fallback.** A GPU start that fails, or that shows no GPU evidence in the server log, is stopped and started again on CPU; the instance and the chat `done` event say which backend answered, and the status carries the notice code `gpu_fallback`.
- D10. **Chat is a write-class request.** `POST /runtime/chat` keeps the default classification (a read-only desktop refuses it with 403 `desktop_readonly`, and a desktop that turns read-only cancels a running answer through the request context). `POST /runtime/stop` is a stop operation (one new path rule in `desktopRequestOperation`). The report card and the examples stay readable on a read-only desktop.
- D11. **The test step.** `steps.test` is `todo` while the project has no succeeded run, `ready` when it has one, `done` once the report card of a succeeded run has been opened (`POST /runs/{id}/report/seen`). `steps.use` stays `todo` (P4). No schema bump: the flag lives in `runs.remote_json` or `settings`.
- D12. **Smoke generation in `verifying`** is a warning, never a failure: with an installed runtime the orchestrator asks the tuned model for 16 tokens; a failure sets the warning `smoke_failed`, a missing or busy runtime is `skipped`. It never triggers a download. The instance it started stays warm for the user's first chat until the idle reaper stops it.
- D13. **Config.** `chibi_sensei.runtime` has `enabled` (default true), `backend` (`auto | cpu | vulkan | cuda`, default `auto`), `idle_minutes` (default 10, 1 to 120) and `context_size` (default 4096, 1024 to 32768). The spec's `memory_mb` (sidecar only) and `serve_port` (P4) are added with their phases.
- D14. **Converter and runtime agree.** A test asserts that the trainer lock's `llama_cpp_tag` equals `llamacpp.Build`.
- D15. **No secret in argv.** The instance's random API key is passed in the child's environment (`LLAMA_API_KEY`), registered with `security.RegisterSensitive` before the process starts, and never logged. The server binds `127.0.0.1` only and runs with `--no-webui`.
- D16. **Two UI modules**, to stay under 1100 lines each: `chibi-sensei-test.js` (step view, run picker, report card, examples) and `chibi-sensei-chat.js` (runtime strip, install dialog, chat, comparison). The report card code of `chibi-sensei-run.js` stays where it is; the test module reuses its exported helpers.
- D17. **Not taken along:** the training time estimate's fixed parts and the Kaggle client's dropped error text (both reported in P2) stay follow-ups; neither is cheap enough to ride on this phase (the estimate needs new measurements per stage, the error text changes the Kaggle integration's public errors).

### Batches and path ownership

| Batch | Owner paths | Content |
|---|---|---|
| B1 | `internal/llamacpp/**`, `internal/embeddings/**`, `internal/config/**`, `config_template.yaml` | extraction of the shared llama.cpp pieces with embeddings wrappers; `chibi_sensei.runtime` config |
| B2 | `internal/chibisensei/chatrt/**` | manager, launcher interface, native launcher, install, chat client, message preparation, limits, reaper, GPU fallback; fake-launcher tests; gated real smoke test |
| B3 | `internal/chibisensei/*.go` | example views with match flags, model availability, report-seen flag, step rule, smoke seam in the orchestrator |
| B4 | `internal/server/**`, `documentation/**`, `prompts/**`, `AGENTS.md` files under `internal/` and the root | runtime routes, chat stream, test view, examples, wiring, meta, operation rule, tests, browser smoke path, docs and DOX |
| F1 | `ui/cfg/**`, `ui/js/config/**`, `ui/lang/config/chibi_sensei/**`, `ui/lang/meta.json`, `ui/js/desktop/apps/chibi-sensei/chibi-sensei-{core,api,steps}.js`, `scripts/**` | config form group, api methods with the stream reader, constants, step primary |
| F2 | `ui/js/desktop/apps/chibi-sensei/**`, `ui/css/desktop-app-chibi-sensei.css`, `ui/lang/chibi_sensei/{en,de}.json` | test module, chat module, root routing, CSS; English and German |
| F3 | `ui/lang/chibi_sensei/*.json` (14 other locales), `ui/*_test.go`, `ui/js/desktop/core/module-loader.js`, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | translations, test lists, loader entry, guides |

Order: B1, then B2 and B3 (different directories, may run in parallel), then B4. The frontend chain F1 → F2 → F3 starts with B1. The backend chain never edits `ui/**` or `scripts/**`; the frontend chain never edits `internal/**`. Exception: the browser smoke file `internal/server/chibi_sensei_app_browser_test.go` belongs to B4.

`go test ./ui` has locale tests that fail between F2 and F3 by design.

### Go API between batches

Names are binding; a plan part may add helpers. Each part states under "Contract changes" what it had to change.

#### B1 provides

```go
// package llamacpp (internal/llamacpp)
const Build = "b9994"
type AssetSpec = onnxrt.AssetSpec
func RuntimeAsset(goos, goarch, backend string) (AssetSpec, bool)   // backend: cpu | vulkan | cuda | metal
func SupplementalAssets(goos, goarch, backend string) []AssetSpec    // windows/amd64/cuda: the CUDA runtime archive; otherwise nil
func DockerImage(backend string) (string, bool)
func FindServer(root string) (string, error)
func LoopbackPort() (int, error)
func RandomAPIKey() (string, error)
func GPUVerified(backend, logs string) bool
type HardwareInfo struct{ NVIDIA bool; NVIDIAReason string; Vulkan bool; VulkanReason string; Fingerprint string }
func DetectHardware(ctx context.Context) HardwareInfo
```

The exact signatures follow the moved functions (check them in the code); if one differs from the list above, say so under "Contract changes".

```go
// package config
type ChibiSenseiRuntimeConfig struct {
    Enabled     bool   `yaml:"enabled" json:"enabled"`           // default true (set before unmarshalling, like Training.DeleteRemoteAfterRun)
    Backend     string `yaml:"backend" json:"backend"`           // auto | cpu | vulkan | cuda; default auto
    IdleMinutes int    `yaml:"idle_minutes" json:"idle_minutes"` // default 10, 1..120
    ContextSize int    `yaml:"context_size" json:"context_size"` // default 4096, 1024..32768
}
// ChibiSenseiConfig gains: Runtime ChibiSenseiRuntimeConfig `yaml:"runtime" json:"runtime"`
```

#### B2 provides (package `chatrt`, `internal/chibisensei/chatrt`)

```go
type Config struct {
    Enabled     bool
    Backend     string // auto | cpu | vulkan | cuda
    IdleMinutes int
    ContextSize int
    CacheDir    string // <data>/chibi_sensei/runtime
}

var (
    ErrDisabled     = errors.New("chat runtime disabled")
    ErrUnsupported  = errors.New("chat runtime unsupported on this platform")
    ErrNotInstalled = errors.New("chat runtime not installed")
    ErrInstalling   = errors.New("chat runtime is being installed")
    ErrBusy         = errors.New("chat runtime busy")
    ErrModelMissing = errors.New("model file missing")
    ErrContext      = errors.New("conversation exceeds the context size")
)

type ModelRef struct {
    Key   string // stable identity of the file: "tuned:<run_id>" or "base:<rel_path of the shared file>"
    Path  string // absolute path of a GGUF the caller has verified
    Bytes int64
}
type Message struct {
    Role    string `json:"role"`    // system | user | assistant
    Content string `json:"content"`
}
type Sampling struct {
    Temperature, TopP, RepeatPenalty, PresencePenalty float64
    TopK, MaxTokens                                   int
    Seed                                              int64 // 0 = random
}
type ChatRequest struct {
    Messages []Message // already prepared with PrepareMessages
    Sampling Sampling
    Stop     []string
}
type ChatStats struct {
    FinishReason     string  // stop | length
    PromptTokens     int
    CompletionTokens int
    MS               int64
    TokensPerSecond  float64
    Backend          string  // cpu | vulkan | cuda | metal
}
type InstanceStatus struct {
    Key         string `json:"key"`
    State       string `json:"state"`        // starting | ready | busy | stopping
    Backend     string `json:"backend"`
    GPUVerified bool   `json:"gpu_verified"`
    IdleSeconds int    `json:"idle_s"`
    Started     string `json:"started"`
}
type Status struct {
    State     string           // disabled | unsupported | not_installed | installing | ready | failed
    Build     string
    Requested string           // configured backend
    Selected  string           // resolved backend; "" while unknown
    Fallback  bool             // a GPU start fell back to CPU since the last install or start
    Download  DownloadStatus   // Bytes (total of the install set), DoneBytes, Host, ErrorCode
    Instances []InstanceStatus
}

func NewManager(cfg func() Config, launcher Launcher, log *slog.Logger) *Manager
func NativeLauncher() Launcher
func (m *Manager) Start(ctx context.Context)                       // idle reaper
func (m *Manager) Status(ctx context.Context) Status
func (m *Manager) Installed() bool                                 // the install set is complete on disk; no network
func (m *Manager) Install(ctx context.Context) error               // starts or joins the download; returns when it ends
func (m *Manager) Acquire(ctx context.Context, ref ModelRef) (*Instance, error)
func (i *Instance) Chat(ctx context.Context, req ChatRequest, onDelta func(text string)) (ChatStats, error)
func (i *Instance) Release()
func (m *Manager) StopAll(ctx context.Context)
func (m *Manager) Close()
func PrepareMessages(systemRole string, in []Message) []Message   // merge_into_first_user rule of D5; otherwise a copy
```

`Launcher` is the seam for tests (a fake process that answers scripted deltas); its shape is the plan part's choice. `Acquire` waits up to 30 s for a busy instance of the same key, then answers `ErrBusy`. A cancelled `ctx` in `Chat` stops the generation at once (the upstream request is closed).

#### B3 provides (package `chibisensei`)

```go
// samples.go
type SampleView struct {
    Index          int       `json:"index"`
    PromptMessages []Message `json:"prompt_messages"` // null for text prompts
    PromptText     *string   `json:"prompt_text"`
    Expected       string    `json:"expected"`
    BaseOutput     *string   `json:"base_output"`
    TunedOutput    *string   `json:"tuned_output"`
    BaseMS         *int64    `json:"base_ms"`
    TunedMS        *int64    `json:"tuned_ms"`
    Truncated      bool      `json:"truncated"`
    BaseMatch      *bool     `json:"base_match"`  // null when the recipe has no match rule or the answer is missing
    TunedMatch     *bool     `json:"tuned_match"`
}
type SamplePage struct {
    Samples []SampleView `json:"samples"`
    Total   int          `json:"total"`   // after the filter
    Offset  int          `json:"offset"`
    Limit   int          `json:"limit"`
}
// ReadSamples reads the run's eval samples through os.Root and applies the report's own match rules.
// filter: all | different (base and tuned answers differ) | wrong (tuned_match is false).
func ReadSamples(dataDir string, run Run, recipe, taskKind string, labels []string, offset, limit int, filter string) (SamplePage, error)

// testview.go
type ModelFile struct {
    Available bool   `json:"available"`
    Bytes     int64  `json:"bytes"`
    Reason    string `json:"reason"` // "" | missing | skipped | invalid
    Key       string `json:"-"`      // chatrt.ModelRef.Key
    Path      string `json:"-"`      // absolute path, set only when Available
}
// RunModels resolves the tuned and the base GGUF of a succeeded run by the rules of D6.
func RunModels(ctx context.Context, st *Store, dataDir string, run Run) (tuned, base ModelFile, err error)
func (s *Store) MarkReportSeen(ctx context.Context, runID string) error // succeeded runs only; idempotent
// computeSteps follows D11.

// smoke seam (reconcile*.go)
type SmokeInput struct{ RunID, ModelPath, ModelKey string; ModelBytes int64; Profile *Profile }
type SmokeResult struct {
    State string `json:"state"` // ok | failed | skipped
    Code  string `json:"code,omitempty"`  // runtime_not_installed | runtime_busy | runtime_disabled | start_failed | generation_failed
    MS    int64  `json:"ms,omitempty"`
    Text  string `json:"text,omitempty"`  // at most 200 characters, scrubbed
}
type Smoker interface{ Smoke(ctx context.Context, in SmokeInput) SmokeResult }
func (o *Orchestrator) SetSmoker(s Smoker) // nil = no smoke (state skipped is not recorded)
```

`RunView` gains `"smoke": SmokeResult | null` and `"report_seen": bool`. `chibisensei` must not import `chatrt` (the server adapts `chatrt.Manager` to `Smoker`); `chatrt` must not import `chibisensei`.

### HTTP contract additions

Base path `/api/desktop/chibi-sensei`; P1 and P2 conventions apply (flat errors, same-origin writes, RFC 3339 UTC times).

#### Meta

`GET /meta` gains:

```json
"runtime":{"enabled":true,"backend":"auto","build":"b9994"},
"chat":{"max_targets":2,"max_turns":20,"max_chars":16000,"max_tokens_cap":1024}
```

Each profile gains `"sampling":{"temperature":0.7,"top_p":0.8,"top_k":20,"repeat_penalty":1.0,"presence_penalty":1.5}` and `"max_new_tokens":{"assistant":256,"specialist":64}`.

#### Runtime

- `GET /runtime` → `{"runtime":Runtime}`
- `POST /runtime/install` → 202 `{"runtime":Runtime}`. Admin only (403 `admin_required`). Starts the download in the background and answers at once; the client polls `GET /runtime` once a second while `state` is `installing`. A second call while installing answers the same. 503 `runtime_unavailable` when the runtime is disabled or the platform has no asset.
- `POST /runtime/stop` → `{"runtime":Runtime}`. Stops every instance. A stop operation (allowed on a read-only desktop).

```json
Runtime = {"state":"disabled|unsupported|not_installed|installing|ready|failed","admin":true,"build":"b9994",
  "backend":{"requested":"auto","selected":"cpu","fallback":false},
  "download":{"bytes":15857307,"done_bytes":0,"host":"github.com","error_code":""},
  "instances":[{"key":"tuned:<run_id>","run_id":"…","kind":"tuned|base","state":"starting|ready|busy|stopping",
                "backend":"cpu","gpu_verified":false,"idle_s":12,"started":"…"}],
  "limits":{"max_instances":2,"idle_minutes":10,"context_size":4096},
  "notice_code":""}
```

`notice_code` is `gpu_fallback` or empty. `download.error_code` is `download_failed`, `checksum_mismatch`, `disk_full` or empty; `state` is `failed` after a failed install until the next install call. `run_id` of a base instance is the run that loaded it last.

#### Test view

- `GET /runs/{runId}/test` →

```json
{"run":RunSummary,"report":Report|null,"report_seen":false,"samples_total":20,
 "models":{"tuned":{"available":true,"bytes":912345678,"reason":""},
           "base":{"available":false,"bytes":0,"reason":"skipped"}},
 "chat":{"system_role":"native","sampling":{"temperature":0.7,"top_p":0.8,"top_k":20,"repeat_penalty":1.0,"presence_penalty":1.5},
         "max_tokens":256}}
```

  409 `run_not_succeeded` for a run in another state. `reason`: `missing` (the file is gone or its size differs), `skipped` (the run has no base model), `invalid` (the header check failed). `chat.max_tokens` is the profile's `eval.max_new_tokens_*` for the project's recipe (specialist value for `specialist`, assistant value otherwise).
- `GET /runs/{runId}/samples?offset=0&limit=20&filter=all|different|wrong` → `SamplePage` (limit 1 to 50, default 20). 404 `samples_missing` when the run has no examples file.
- `POST /runs/{runId}/report/seen` → 204. Succeeded runs only (409 `run_not_succeeded`); idempotent. The next project read shows `steps.test: "done"`.

#### Chat

`POST /runtime/chat`

```json
{"run_id":"…",
 "targets":[{"kind":"tuned","messages":[{"role":"system","content":"…"},{"role":"user","content":"…"}]},
            {"kind":"base","messages":[{"role":"user","content":"…"}]}],
 "sampling":{"temperature":0.7,"top_p":0.8,"top_k":20,"max_tokens":256,"seed":0}}
```

- Each target carries its own conversation, because the two models' earlier answers differ. Every conversation ends with a `user` message.
- `sampling` is optional; a missing field takes the profile's value. Ranges: temperature 0 to 2, top_p 0 to 1 (above 0), top_k 0 to 200, max_tokens 1 to 1024, seed ≥ 0.
- Limits: 1 or 2 targets, each kind at most once; per conversation at most 20 user messages, 64 messages and 16,000 characters in total; roles as in a dataset row (optional single leading `system`, then alternating `user` and `assistant`).
- Before the stream starts, errors are ordinary JSON answers: 422 `validation_failed`, 404 `run_not_found`, 409 `run_not_succeeded`, 404 `model_missing` (a requested target has no usable file; the message names the kind), 409 `runtime_not_installed`, 503 `runtime_unavailable` (disabled or unsupported), 409 `runtime_busy` (an install is running, or both slots are in use by other models).
- Otherwise the answer is `200 text/event-stream` (`Cache-Control: no-store`, `X-Accel-Buffering: no`), one event per line pair as in the project event stream, a `: ping` comment every 15 s:

```
event: status  data: {"target":"tuned","state":"loading|generating"}
event: delta   data: {"target":"tuned","text":"…"}
event: done    data: {"target":"tuned","finish_reason":"stop|length","prompt_tokens":41,"completion_tokens":87,"ms":3120,"tok_s":27.9,"backend":"cpu"}
event: error   data: {"target":"base","code":"start_failed|generation_failed|context_exceeded|runtime_busy|cancelled","message":"…"}
event: end     data: {}
```

- Every target ends with exactly one `done` or `error`; `end` closes the stream. One target's error does not stop the other. Closing the request stops both generations.
- Only answer text is forwarded (`choices[0].delta.content`); reasoning fields are dropped. The text is model output and untrusted: the client shows it with `textContent`.

#### Runs and steps

- `Run` and `RunSummary` gain `"report_seen": bool` and `"smoke": {"state":"ok|failed|skipped","code":"…","ms":812,"text":"…"} | null`.
- New run warning code `smoke_failed` (in `Run.warning_code` only when no other warning is set, always as a `warn` run event).
- `steps.test` follows D11.

#### New error codes

`runtime_unavailable` 503, `runtime_not_installed` 409, `runtime_busy` 409, `model_missing` 404, `run_not_succeeded` 409, `samples_missing` 404. Stream error codes: `start_failed`, `generation_failed`, `context_exceeded`, `runtime_busy`, `cancelled`.

#### Config

```yaml
chibi_sensei:
  runtime:
    enabled: true
    backend: "auto"      # auto | cpu | vulkan | cuda
    idle_minutes: 10     # 1..120
    context_size: 4096   # 1024..32768
```

### UI behaviour (binding for F1 to F3 and for the browser smoke in B4)

- The Test step replaces the placeholder of `P.training.createLater` for `test` (the `use` placeholder stays). Without a succeeded run it shows an empty state that points to the Training step.
- Layout: a run picker (succeeded runs, newest first, shown only when there is more than one), then a segmented switch with three views: report card, examples, chat. Opening the report card of a succeeded run calls `report/seen` once and refreshes the project.
- Report card: the numbers of the report (the run module's helpers), the base-model availability, the smoke result as one line, a "small sample" note.
- Examples: a paged list (20 per page) with the prompt, the expected answer and the base and tuned answers side by side, match marks where the API gives them, the filter of the API, and "Try in chat", which puts the example's prompt into the composer.
- Chat: a runtime strip (state, backend, GPU fallback notice, "Stop models"); when the runtime is not installed, a card with the download size and host and the button "Install" for an administrator (confirmation dialog first) or the text whom to ask; an optional system prompt field; the composer with Send and Stop; the switch "Compare with base model" (off and disabled with the reason when the base model is not available); one or two columns of turns with a stats line per answer (tokens, tokens per second, backend); "New chat"; an advanced disclosure with temperature, top_p, top_k and max tokens and "Reset to model defaults".
- The primary action of the Test step is "Chat with your model" (switches to the chat view); it is disabled with a reason while no succeeded run exists.
- Selectors the browser smoke uses (`data-csn`): `test` (step root), `test-view` with `data-view="report|samples|chat"` on the switch buttons, `report-card`, `sample` (one per example), `runtime-strip` with `data-state`, `runtime-install`, `chat-input`, `chat-send`, `chat-stop`, `chat-compare`, `chat-column` with `data-target="tuned|base"`, `chat-answer` with `data-target` and `data-state="streaming|done|error"`.
- Read-only desktop: report card and examples work; the chat view shows the read-only note and no composer.

### Verification of the phase

- `go build ./...`; `go vet` on the touched packages.
- `go test ./internal/llamacpp/... ./internal/embeddings/... ./internal/config/... ./internal/chibisensei/... -count=1`; the Chibi handler tests and the desktop-operation tests of `internal/server`; the Chibi tests of `./ui`; `node scripts/test-chibi-sensei-model.mjs`; `npm run check:ui`.
- Browser smoke `TestChibiSenseiAppRealServerBrowser` with one test-and-compare path: a seeded succeeded run with example file and both model files, a fake launcher behind a server test override, report card, examples, one compared chat turn.
- Real runtime: the gated test `AURAGO_RUN_LLAMA_SMOKE=1 CHIBI_SMOKE_GGUF=<path> go test ./internal/chibisensei/chatrt -run TestRealLlamaServerChat` downloads the pinned CPU runtime into a temp cache, loads the GGUF and streams one answer; then one chat through a scratch AuraGo instance with a seeded succeeded run.

### Amendments

From the frontend plan ("Contract changes"), accepted by the coordinator:

- A1. The loader entry and the module-list tests (`module-loader.js`, the script count in `desktop_chibi_sensei_guards_test.go`, the module list in `desktop_chibi_sensei_registration_test.go`, the helper rows in `i18n_lint_test.go`) move from F3 to F2: the guard test stops at the script count before it checks the new modules.
- A2. F1 also edits `ui/config_chibi_sensei_test.go`.
- A3. F1 writes nine keys into all 16 `ui/lang/chibi_sensei/*.json` files (the six new error texts, `primary.no_succeeded_run`, `tip.test_ready`, `tip.test_done`).
- A4. F1 edits `recompute` and `runPrimary` of the composition root; F2 edits the root afterwards.
- A5. `scripts/test-chibi-sensei-model.mjs` is extended by F1 (stream parser, primary, tips) and by F2 (chat reducer, test helpers).
- A6. `chat-answer` gains `data-state="stopped"` and, while streaming, `data-phase="waiting|loading|generating"`. Further selectors the smoke may use: `test-run`, `test-panel`, `test-empty`, `samples-filter`, `samples-pager`, `sample-try`, `runtime-stop`, `runtime-card`, `runtime-progress`, `chat-system`, `chat-new`, `chat-error`, `chat-live`, `chat-readonly`.
- A7. The run module's report code is lifted to module scope (`fillReport`, `confusionBox`) and exported; its output does not change.
- A8. Loader order: `… run, chat, test, root`.
- A9. The UI never sends `report/seen` from a read-only desktop; the Test step stays `ready` there until a writable visit.

From the coordinator:

- A10. Conversation length is counted in Unicode code points on both sides (the server counts runes, never bytes or UTF-16 units).
- A11. The server sends the chat stream exactly as the contract shows it (`event:` line, `data:` line, empty line; `: ping` comments); the client drops unknown events.

From the backend plan ("Contract changes"):

- A12. Accepted: the additions to `internal/llamacpp` (`DetectHardwareWith`, `Runtimes`, the Docker image constants, `WindowsCUDARuntimeID`) and to `chatrt` (`ErrStopped`, `ErrClosed`, `DownloadStatus`, `MaxInstances`, `BeginInstall`, `Admit`, `Instance.Backend`, the launcher shapes, the test helper package `chatrt/chatrttest`); B2 adds one row to `internal/audit/audit_manifests.go`; B4 edits the usage column of the llama.cpp rows in three licence records and writes `internal/llamacpp/AGENTS.md`; B3 exports `ErrRunNotSucceeded`, `ErrSamplesMissing`, `SamplePageMax` and `RunTask`; the smoke has no read-only gate; `Status.Fallback` is sticky until the next install or a change of `runtime.backend`; the `different` filter compares normalized answers that are both present; a 422 of the chat route carries the message key `chibi_sensei.chat.invalid`; "Stop models" during an answer ends the target with the stream code `cancelled`.
- A13. Not taken: the backend plan's change 7. `POST /runs/{id}/report/seen` keeps the default write classification (no second rule in `desktopRequestOperation`, no state change on a read-only desktop). A9 stays: the UI does not send it from a read-only desktop.

## Execution record

What happened when the plan was carried out (2026-11-27), where it differs from the parts as written. The parts are kept as planned; this list and the amendments say what is true.

- **Order.** F1 and B1 ran in parallel, then F2 with B2 and B3, then B4 with the documentation. The 14 other locales were translated by three implementers through one merge script that checks key sets and placeholders. Every batch was committed by the coordinator after its own re-run of the batch's checks.
- **Reviews.** One read-only review of the frontend (F1 and F2), one of the backend (B2 and B3) and one security pass over the runtime and the routes. Fixed with a test each, unless noted:
  - frontend: the run picker had no change listener (only the newest run could be tested); "Try in chat" lost the example while the runtime status was loading or the runtime was not installed; a message refused before the stream was lost when the user had typed again; stale runtime answers after install and stop; events after `end`; no retry for a failed first page of examples;
  - backend: the idle reaper freed a slot before the child had stopped, so three children could run; the backend probe could be cached from a cancelled request; a stalled answer had no idle limit (now two minutes, `ErrStalled`); upstream error text was not scrubbed; a stop stored just before the smoke was registered did not end the run at once;
  - security: the model file is checked again with `Lstat` right before a start; one API key per launcher instead of one per start; at most four chat requests at the same time.
- **Backend labels.** The plan's texts put the backend label into three different sentences; a label with an article or a case ending fits only one. All 16 locales now use a neutral noun phrase after a colon or in parentheses.
- **Discovery with the real binary** (B2, windows/amd64 CPU, a Qwen3.5 4B GGUF): every planned flag exists at b9994; `/v1/chat/completions` streams `data:` chunks that end with `[DONE]`, with `usage` and `timings` in a last chunk when `stream_options.include_usage` is set; an over-long prompt is HTTP 400 with the type `exceed_context_size_error`, with and without `stream`; a closed client request frees the slot at once; `--reasoning off` plus `enable_thinking: false` gives answers without reasoning text. A reused connection failed once with EOF, so the loopback client opens a fresh connection per request.
- **Linux.** The gated runtime test, cross-compiled and run on the test server (Ubuntu, glibc 2.43), failed at first: the Linux archive ships the library names the loader asks for (`libllama-common.so.0` and others) as symbolic links, and the shared extractor of `internal/onnxrt` skips links. The chat runtime now recreates the archive's link entries in its own directory (`ensureArchiveLinks`); `internal/onnxrt` is unchanged. After the fix the test passes there: start in 9 s, chat, context error, cancel and stop by handle, 7 to 10 tokens per second with a 4B Q4 model on 3 threads. The native llama.cpp path of the embeddings uses the same archive and extractor and very likely has the same defect on Linux; it was not changed here.
- **Threads.** `runtime.NumCPU()/2` per instance meant 16 threads on 8 physical cores for a compared chat; it is a quarter of the logical processors now (1 to 8).
- **Real end-to-end runs** (gated tests `TestChibiSenseiRealRuntimeChat` and `TestChibiSenseiAppRealRuntimeBrowser`, windows/amd64): the install route downloaded the pinned CPU runtime (18,254,390 bytes from github.com) into the test's data directory; a seeded succeeded run whose tuned and base files are the same real Qwen3.5 4B GGUF answered both targets through `POST /runtime/chat` and through the UI in Chrome; a second turn used the warm instance; an over-long conversation ended with `context_exceeded`; Stop models left no process. The machine was at 100 % CPU load from other work during these runs, so their speed (below one token per second) says nothing about the runtime.
- **Not done, by decision:** the Docker sidecar mode, the LLM judge and a provider as third chat column (user decisions U2 and U3); `Pin` and `Unpin` (P4); the training time estimate and the Kaggle client's error text (D17).
- **Not verified:** a tuned GGUF from a real training run next to its real base GGUF (the proof files of P2 were deleted, so both targets used the same model file); macOS; Vulkan and CUDA starts and the GPU fallback; the native binary inside AuraGo's Docker image; the install click with its confirmation dialog in a browser (the install route itself ran for real); two instances side by side on a quiet machine.
- **Open points for later:** `llama-server` parses a model file from a training run as native code without an operating-system sandbox (security review, medium); the child gets the repository-wide filtered environment, not an allow-list; the health probe does not check the listener's identity; evaluation examples are parsed on every request; no parent-death signal on Linux and macOS (systemd and Docker end the child with AuraGo).
- **Red on main, not from this phase:** `TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget` (two `aura-drift` files above the line budget).
