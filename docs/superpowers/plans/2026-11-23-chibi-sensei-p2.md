# Chibi Sensei P2 (Kaggle training) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship phase P2 of Chibi Sensei: the user starts a training run from the Training step, AuraGo trains the chosen small model on the user's own Kaggle GPU quota, shows progress, downloads and verifies the result, and registers a GGUF model with basic report numbers.

**Architecture:** One Python script (`internal/chibisensei/trainer/chibi_train.py`, embedded) does the training on Kaggle from a `job.json`; AuraGo wraps both into a generated notebook. A persisted state machine in `internal/chibisensei` (`reconcile.go`) drives a run through upload, dataset creation, notebook start, tracking, download, verification and cleanup, and talks to Kaggle only through the `KaggleDriver` interface, which `internal/server` implements over `internal/kaggle`. The HTTP layer adds preflight, consent, prepare/start with a plan token and the run routes under `/api/desktop/chibi-sensei`; the UI adds the Training step (preflight, start, run monitor, loss chart).

**Tech Stack:** Go (modernc SQLite, net/http, SSE), Python 3.10+ (PyTorch, transformers, peft, llama.cpp converter; on Kaggle only), vanilla JS classic scripts, CSS with `--vd-mat-*` tokens, 16 locale JSON files.

Design spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`. P1 plan: `docs/superpowers/plans/2026-11-22-chibi-sensei-p1.md`.

This plan has three files:

- this index: conventions, impact analysis, batch order, the coordinator contract (decisions, Go API between batches, HTTP contract additions, trainer contract, amendments, proof runs);
- `2026-11-23-chibi-sensei-p2-backend.md`: tasks B1 to B5 (B2 in two parts: B2a the Go side, B2b the trainer script);
- `2026-11-23-chibi-sensei-p2-frontend.md`: tasks F1 to F4.

Each part of the backend file and the frontend file starts with its own "Contract changes" list. The index's "Amendments" section collects the ones that bind other batches; where a part and the amendments differ, the amendments win.

---

## Conventions for every batch

**Where to work.** Only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\trusting-roentgen-94e148` (branch `claude/trusting-roentgen-94e148`). Never read or write the main checkout at `C:\Users\Andi\Documents\repo\AuraGo` outside that folder: other sessions work there.

**Who commits.** Implementers do not commit, stage, stash, rebase or switch branches. The coordinating session reviews each batch and commits its paths.

**Ignored files.** `.gitignore` ignores `AGENTS.md`, `docs/` and `*.lock`, among others. Before a commit the coordinator runs `git status --short --ignored -- <paths>` and force-adds every new file that shows as `!!` (`git add -f`), then checks with `git ls-files`.

**One batch, one dispatch** where the batch fits; a large batch is split into consecutive dispatches along its task numbers. Each batch gets one combined review that reports only Critical and Important issues.

**Tests.** Write the tests the task names, plus one test for each bug you fix on the way. No property, mutation or stress suites, no `-race` runs.

**Go commands** (Git Bash, from the worktree root):

- `go build ./...` must stay green after every batch.
- Run only the packages a task names, for example `go test ./internal/chibisensei/... -count=1`. For `-run`, group patterns as `(?i)(a|b)`.
- If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`.
- Do not set `GOTMPDIR` or `GOCACHE` yourself.

**Python.** The developer machine has `python` (3.14), not `python3`. The trainer's Go test looks for both and skips when neither runs.

**UI commands.** `node --check <file>` for every changed JS file, `node scripts/test-chibi-sensei-model.mjs`, `npm run build:ui` after touching a bundled source (the module loader), `npm run check:ui` before reporting done.

**Text encoding.** Write real UTF-8 characters (umlauts, CJK) into source and JSON files. Never write `\u` escape sequences through the file-writing tool: it decodes them. Files under `internal/chibisensei/trainer/` and `internal/chibisensei/testdata/` use LF.

**UI rules.** `textContent` only (no `innerHTML`, no template strings, no `eval`, no storage, no browser dialogs), no external assets, CSS classes start with `csn-`, each JS module stays under 1100 lines, every user-visible string is a translated key, `fetch` and `EventSource` only in `chibi-sensei-api.js`.

**Scratch files** go to `C:\disposable\chibi-sensei\`, never into the repository.

**Processes.** Stop only processes you started, by PID. Never kill by image name.

**Code intelligence.** The coordinating session runs the impact analysis for existing symbols before a batch starts and the change analysis before each commit; implementers do not run GitNexus.

**Report format.** End with: files created, files modified, the exact test commands run with their PASS/FAIL lines, deviations from the plan with the reason, and anything left open.

## Impact analysis of existing symbols (2026-11-23)

| Symbol | Risk | Change |
|---|---|---|
| `config.NormalizeKaggleConfig` | CRITICAL, 23 impacted, reaches `main` and `Start` through the config load | normalizes one new list |
| `kaggle.WithConfigSnapshot` | HIGH (tools, agent, server) | copies one new slice |
| `desktopRequestOperation` (internal/server) | HIGH, 156 impacted (every desktop route) | one new path rule: run cancel is a stop operation |
| `config.load` | hub | defaults of `chibi_sensei.training` |
| `kaggle.CheckGrants`, `kaggle.intersectConfig`, `(*kaggle.Service).download` | LOW | machine-shape rule; list intersection; redirect helper extracted |
| `chibisensei.ComputePlan`, `computeSteps`, `ValidateProfile` | LOW | new plan fields; training step state; stricter validation |
| `handleChibiSensei`, `openChibiRuntime`, `handleChibiEvents` (internal/server) | LOW | new routes; orchestrator wiring; new events |
| `(*chibisensei.Store).DeleteProject` | UNKNOWN in the index; one caller by text search (`internal/server/chibi_sensei_projects.go`) | refuses a project with an active run |

Because of the CRITICAL and HIGH entries, B1's verification runs the whole `internal/config` and `internal/kaggle` packages plus the Kaggle tests of `internal/server`, `internal/tools` and `internal/agent`; B5's verification runs the desktop-operation tests and every Chibi Sensei and Kaggle test of `internal/server`.

## Batch order

1. B1 (Kaggle extensions and config).
2. Then two chains in parallel, with strict path ownership:
   - backend B2a → B2b → B3 → B4 → B5 owns `internal/**`, `config_template.yaml`, `documentation/**`, `prompts/**`, `.gitattributes`, the root `AGENTS.md`;
   - frontend F1 → F2 → F3 → F4 owns `ui/**` and `scripts/**`.
3. Proof runs (coordinator): they start as soon as B1 and B2 are in, first as CPU smoke runs of the trainer in a container, then on Kaggle through a scratch AuraGo instance on the test server (see "Proof runs").
4. Integration: one end-to-end run through the real orchestrator, one browser smoke, one whole-branch review, rebase onto `main`, land.

`go test ./ui` has two locale tests that fail between F2 and F4 by design (the 14 other locales follow in F4).

---
## Coordinator contract

Written by the coordinating session before the plan parts, and amended as they came back. It fixes what the parts share. It covers decisions, batch order, path ownership, the Go API between batches, the HTTP contract additions and the trainer contract. Where this file and the spec differ, this file wins; each difference is listed under "Decisions".

Sources to read first:

- Spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md` (sections "Model profiles", "Training contract", "Kaggle orchestration", "Live progress", "Config, permissions and preflight", "Security and privacy", "Testing", "Phases" P2, "Risks and unverified assumptions").
- P1 plan index (conventions, HTTP contract): `docs/superpowers/plans/2026-11-22-chibi-sensei-p1.md`.
- Package guide: `internal/chibisensei/AGENTS.md`; Kaggle guide: `internal/kaggle/AGENTS.md`.
- Fact sheets gathered on 2026-11-23 (code as it is now, with line numbers), all in `C:\disposable\chibi-sensei\p2\`:
  - `facts-kaggle.md` (config, policy, service, transfers, remote jobs, client, operations, server, test fixtures, config UI);
  - `facts-p1-backend.md` (schema, store, profiles, plan, jobs, events, steps, server, config, config update path, backup, docs);
  - `facts-p1-frontend.md` (modules, core and api, training step, composition root, dialogs, CSS, locales, loader, tests, config UI, guides);
  - `web-qwen35.md`, `web-lfm25.md`, `web-gemma3.md`, `web-llamacpp-b9994.md`, `web-kaggle-runtime.md`, `web-module-names.md` (web research on the three base models, the converter and the Kaggle image).

The fact sheets were written by reader agents. Check a line number against the file before you put it into a step.

### Decisions

User decisions of 2026-11-23 (do not re-ask):

- U1. The proof runs of all three profiles may use the user's Kaggle account and GPU quota. The account is phone-verified.
- U2. The runs go through a scratch AuraGo instance on `aurago-test`, built from this branch, with a copy of the live vault and config. The user enters the Kaggle credentials in the live instance; grants are set only in the scratch config.
- U3. Grants for the scratch instance: enabled, not read-only, `allow_create`, `allow_remote_execute`, `allow_gpu`, `allow_internet`, `allow_download`, machine shape `NvidiaTeslaT4`, plus `allow_delete`.

Coordinator decisions:

- D1. **No schema version bump.** `schemaVersion` stays 1 and `OpenStore` keeps its strict equality check. The run tables of P1 (`runs`, `run_events`, `metrics`, `artifacts`, `eval_reports`, `base_gguf`, `consents`, `settings`) are used as they are. Data that has no column goes into the JSON columns (`runs.remote_json`, `runs.job_json`, `runs.hyper_json`, `runs.leftovers_json`). New indexes may be added to `schemaSQL` with `CREATE INDEX IF NOT EXISTS` (the script runs at every open).
- D2. **Manual training loop.** The trainer uses plain PyTorch (AdamW, cosine schedule with warmup, gradient clipping 1.0) instead of `transformers.Trainer`. Reason: the Trainer arguments change between transformers 5.x releases (for example `warmup_ratio`), and a manual loop gives exact control of the time budget, NaN and out-of-memory handling. `accelerate` is not needed. The spec's hyperparameters, metrics and stop reasons are unchanged.
- D3. **Discovery logging.** Every value the profiles mark as unverified is measured and logged by the trainer: GPU name, capability and VRAM; Python, torch, transformers and peft versions; the class that loaded the base; the histogram of `nn.Linear` leaf names; the resolved LoRA targets; training and generation tokens per second; peak VRAM; the sha256 of the llama.cpp tarball; the converter environment used; the GGUF architecture and tensor count. All of it is written into `chibi-status.json` (`discovery` object) and as `@@chibi` lines.
- D4. **LoRA targets are resolved at run time.** The trainer intersects the profile's `target_modules` with the leaf names of the `nn.Linear` modules of the loaded text model (never the LM head, the embeddings or a vision tower). A profile name without a match gives a `warn` line. If nothing matches, the trainer falls back to every text `nn.Linear` leaf name and warns. `status.discovery.lora_targets` records the result.
- D5. **Base loading chain.** `base.auto_class` names the class to try first; the new optional profile field `base.auto_class_fallbacks` lists further class names. The trainer tries them in order with `getattr(transformers, name)` and records the one that worked.
- D6. **Profile corrections from research** (all three stay `experimental`; `unverified` lists stay until the proof runs):
  - `gemma3-270m`: `base.revision` = `23cf460f6bb16954176b3ddcc8d4f250501458a9`.
  - `qwen35-0p8b`: `base.revision` = `2fc06364715b967f1860aea9cf38778875588b17`; `base.auto_class` = `Qwen3_5ForCausalLM`, `base.auto_class_fallbacks` = `["AutoModelForCausalLM", "AutoModelForMultimodalLM", "AutoModelForImageTextToText"]`; `training.lora.target_modules` = `["q_proj","k_proj","v_proj","o_proj","gate_proj","up_proj","down_proj","in_proj_qkv","in_proj_z","out_proj"]`; `training.extra_pip` = `[]` (transformers falls back to plain PyTorch ops without `fla` and `causal_conv1d`; the proof run measures the speed).
  - `lfm25-1p2b`: `base.revision` = `b9d816ab510f9a28dca37cb182e9da8b0226e13c`; `training.lora.target_modules` = `["q_proj","k_proj","v_proj","out_proj","w1","w2","w3","in_proj"]`.
- D7. **Training text.** For chat rows the loss covers only the last assistant message. `prompt = tokenizer.apply_chat_template(messages[:-1], add_generation_prompt=True, tokenize=False)`; `train_text = prompt + answer + profile.chat.stop[0]`; both parts are tokenized with `add_special_tokens=False` (the template carries the BOS token) and the prompt tokens get label -100. For `system_role == "merge_into_first_user"` the trainer merges a leading system message into the first user message (`system + "\n\n" + user`) before templating. Text rows (free recipe) are `bos + text + eos` with loss on all tokens. A sequence over `max_seq_len` keeps its first token and the last `max_seq_len - 1` tokens; `status.train.truncated_rows` counts them.
- D8. **Tokenizer files of the merged model** are copied from the downloaded base snapshot, not re-saved, so the converter sees the files the vendor published.
- D9. **Converter environment.** The converter first runs in the notebook's own Python environment. If it fails, the trainer builds a virtual environment from the tarball's `requirements/requirements-convert_hf_to_gguf.txt` and runs it again. `status.discovery.converter_env` is `system` or `pinned`.
- D10. **Adapter files are flat**: `chibi-adapter-model.safetensors` and `chibi-adapter-config.json` in `/kaggle/working` (no sub-directory), because a download names one flat file.
- D11. **Empty split files are not written.** A split with zero rows (usually `val`) has no file and no marker entry.
- D12. **Dataset file names.** The names inside the Kaggle dataset equal the staged project file names (`upload` overwrites `fileName` with the project file name; verify in `internal/kaggle/transfers.go`). Staged and dataset names: `r<run12>-train.jsonl`, `r<run12>-val.jsonl`, `r<run12>-eval.jsonl`, `r<run12>-marker.json`. `job.json` carries `data.marker` = the marker's exact file name. `<run12>` is the first 12 hex characters of the run ID without dashes.
- D13. **One active run per installation.** A second start answers 409 `run_active`.
- D14. **`prepare` then `start`.** The confirmation data and the plan token come from `POST /projects/{id}/runs/prepare`, not from `GET /plan`. `GET /plan` stays cheap and free of Kaggle calls.
- D15. **Admin enabling goes through a Chibi route** that accepts flag names only and builds the config patch on the server (`applyConfigPatch`, `internal/server/config_patch.go`). The client never sends config values.
- D16. **Run monitor is its own UI module** (`chibi-sensei-run.js`) next to `chibi-sensei-training.js` and `chibi-sensei-chart.js`, to keep every module under 1100 lines.
- D17. **No machine-shape values beyond the two known names**: `NvidiaTeslaT4`, `NvidiaL4`.
- D18. **Backups.** `data/chibi_sensei/` (snapshots, models, base GGUFs) is not part of AuraGo backups today (`facts-p1-backend.md`, section 10); only `chibi_sensei.db` is. P2 keeps that and documents it. No backup code changes.
- D19. **Python on the developer machine** is `python` (3.14), not `python3`. The selftest Go test looks for `python3`, then `python`, and skips when neither runs.

### Batches and path ownership

Order: B1 → B2 → proof runs start (coordinator) while B3 → B4 → B5 continue; the frontend chain F1 → F2 → F3 → F4 starts after B1.

| Batch | Owner paths | Content |
|---|---|---|
| B1 | `internal/config/**`, `internal/kaggle/**`, `config_template.yaml`, `documentation/**`, `prompts/**` | `kaggle.allowed_machine_shapes`, policy, `ReadKernelLog`, `RemoveFile`; `chibi_sensei.training` config section |
| B2 | `internal/chibisensei/**` (new files below, `plan.go`, `profiles/*.json`, `profiles.go`) | trainer, lock file, job spec, notebook, snapshot, log and output parsers, golden files, proof data and bundle generator |
| B3 | `internal/chibisensei/**` (`runs.go`, `plantoken.go`, `preflight.go`, `report.go`, `driver.go`, `projects.go`) | run store, consents, settings, plan token, preflight evaluation, report scoring, driver interface and the fake driver |
| B4 | `internal/chibisensei/**` (`reconcile.go`, `artifacts.go`, `gguf.go`) | state machine, reconciler, downloads, artifact registry, cleanup, cancel, deadline; the fake-Kaggle suite |
| B5 | `internal/server/**`, `documentation/**`, `AGENTS.md` files | driver adapter, routes, preflight and enable, boot, SSE, docs and DOX |
| F1 | `ui/cfg/**`, `ui/lang/kaggle/**`, `ui/lang/config/chibi_sensei/**`, `ui/js/config/**` | Kaggle machine-shape checkboxes, Chibi training settings |
| F2 | `ui/js/desktop/apps/chibi-sensei/**`, `ui/css/desktop-app-chibi-sensei.css`, `ui/lang/chibi_sensei/{en,de}.json` | api methods, preflight, enable dialog, preset cards, overrides, consent, confirmation |
| F3 | same as F2 | run monitor, chart, result card, artifacts, leftovers, history, SSE and poll wiring |
| F4 | `ui/lang/chibi_sensei/*.json` (14 other locales), `ui/*_test.go`, `scripts/**`, `ui/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `ui/js/desktop/core/module-loader.js`, bundle | translations, test updates, loader entry, bundle rebuild, guides |

The backend chain never edits `ui/**` or `scripts/**`; the frontend chain never edits `internal/**`. Exception: F4 may touch `ui/*_test.go` only.

### Go API between batches

Package `chibisensei` unless stated. Names below are binding; a plan half may add helpers.

#### B1 provides (package `kaggle` and `config`)

```go
// internal/config/kaggle.go
type KaggleConfig struct { /* ... */ AllowedMachineShapes []string `yaml:"allowed_machine_shapes" json:"allowed_machine_shapes"` }
var KaggleMachineShapes = []string{"NvidiaTeslaT4", "NvidiaL4"} // known names; NormalizeKaggleConfig keeps only these, trimmed, deduped, in this order

// internal/config/chibi_sensei.go
type ChibiSenseiTrainingConfig struct {
    MachineShape         string `yaml:"machine_shape" json:"machine_shape"`                     // default NvidiaTeslaT4; must be a known name
    DeleteRemoteAfterRun bool   `yaml:"delete_remote_after_run" json:"delete_remote_after_run"` // default true
}
// ChibiSenseiConfig gains: Training ChibiSenseiTrainingConfig `yaml:"training" json:"training"`

// internal/kaggle/logs.go
// ReadKernelLog reads the session log of one notebook version without a durable job and without a
// project file. It needs the download grant, filesystem write access and network access, and owner
// must be the authenticated account. It returns at most the last maxBytes bytes (the read itself is
// capped at 16 MiB). An empty result with a nil error means "no log yet".
func (s *Service) ReadKernelLog(ctx context.Context, access Access, owner, slug string, version int, maxBytes int64) ([]byte, error)

// internal/kaggle/transfers.go
// RemoveFile deletes one regular file of a project. A missing file is not an error.
func (s *Service) RemoveFile(project, name string) error
```

`DeleteRemoteAfterRun` defaults to true. A YAML bool cannot tell "absent" from false, so the loader sets the default before unmarshalling (the way `ChibiSensei.Enabled` gets its default in `config.go`).

#### B2 provides

```go
// plan.go: Plan gains these fields (additive JSON), set by ComputePlan
type Plan struct { /* existing fields */
    EvalSamples         int  `json:"eval_samples"`
    MaxNewTokens        int  `json:"max_new_tokens"`         // profile.Eval by recipe
    MaxTrainSeconds     int  `json:"max_train_seconds"`      // session timeout minus 1.5 x the non-training estimate, at least 300
    NeedBaseGGUF        bool `json:"need_base_gguf"`
    ArtifactsEstimatedMB int `json:"artifacts_estimated_mb"` // tuned GGUF, plus the base GGUF when needed, plus 50
}
// ComputePlan sets Trainable = true and ReasonKey = "" when it succeeds.

// profiles.go: ProfileBase gains AutoClassFallbacks []string `json:"auto_class_fallbacks,omitempty"`

// trainer/embed.go (package trainer)
func Script() []byte          // embedded chibi_train.py
func ScriptSHA256() string
type Lock struct {
    Pip          []string // "name==version" lines
    Transformers string   // "" keeps the preinstalled version
    LlamaCppTag, LlamaCppURL, LlamaCppSHA256 string // sha256 "" = not pinned yet: the trainer logs the measured value and warns
}
func LoadLock() (Lock, error) // parses the embedded requirements.lock

// jobspec.go
type JobSpec struct { /* fields of the spec's job.json schema v1, in that order */ }
type JobInput struct {
    RunID         string
    Created       time.Time
    AuraGoVersion string
    Project       Project
    Profile       *Profile
    ProfileSHA    string
    Plan          Plan
    Marker        string // file name of the dataset marker
    Format        string // messages | text
    Lock          trainer.Lock
}
func BuildJobSpec(in JobInput) (JobSpec, error)
func (j JobSpec) JSON() ([]byte, error) // deterministic: struct order, no HTML escaping, one trailing newline

// notebook.go
func BuildNotebook(runID string, job, script []byte) ([]byte, error) // nbformat 4, two cells, deterministic bytes

// snapshot.go
type SnapshotFile struct {
    Name   string `json:"name"`
    SHA256 string `json:"sha256"`
    Rows   int    `json:"rows"`
    Bytes  int64  `json:"bytes"`
}
type Snapshot struct {
    RunID   string
    Dir     string
    Files   map[string]SnapshotFile // keys train, val, eval; a split with zero rows is absent
    Marker  SnapshotFile            // Rows is 0
    Format  string                  // messages | text
    DataRev int64
    DataSHA string                  // sha256 over the split names and file hashes in the order train, val, eval
    Bytes   int64                   // sum of all four files
}
// FreezeSnapshot writes the split files and the marker into dir and returns their facts. It fails
// with ErrDataChanged when the rows changed while it read them. Excluded rows are skipped.
func (s *Store) FreezeSnapshot(ctx context.Context, projectID, runID, dir string) (Snapshot, error)
func Run12(runID string) string // first 12 hex characters, dashes removed

// logparse.go
type LogEvent struct {
    Seq   int64
    TS    string
    T     string          // stage | metric | eval | warn | error | done | exit | info
    Raw   json.RawMessage // the whole object
}
// ParseKernelLog accepts a JSON array of {stream_name, time, data} entries or plain text, extracts
// the "@@chibi " lines, drops malformed ones, dedupes by seq and returns them in seq order.
func ParseKernelLog(body []byte) []LogEvent
type Metric struct {
    Step, MaxSteps int
    Kind           string // train | eval
    Epoch, Loss, LR, GradNorm, TokS, ElapsedS, EtaS, VRAMGB float64
    TS             string
}
func (e LogEvent) Metric() (Metric, bool)
type StageEvent struct{ Stage, State string } // state: start | done | failed
func (e LogEvent) Stage() (StageEvent, bool)
type ErrorEvent struct{ Stage, Code, Message string }
func (e LogEvent) Error() (ErrorEvent, bool)

// outputs.go
type Status struct { /* chibi-status.json of the spec, plus Discovery map[string]any `json:"discovery"` and Train.TruncatedRows */ }
func ParseStatus(b []byte) (Status, error)
type ManifestFile struct{ Path, Kind string; Bytes int64; SHA256 string }
type Manifest struct{ V int; RunID, ProfileID, ProfileSHA string; Files []ManifestFile }
func ParseManifest(b []byte) (Manifest, error)       // rejects a path with a separator, "..", or without the "chibi-" prefix
func ParseMetricsFile(r io.Reader) ([]Metric, error)
type EvalSample struct { /* chibi-eval_samples.jsonl of the spec */ }
func ParseEvalSamples(r io.Reader, max int) ([]EvalSample, error)
```

Manifest `kind` values: `gguf_tuned`, `gguf_base`, `adapter`, `adapter_config`, `metrics`, `eval_samples`, `status`.

#### B3 provides

```go
// runs.go
const ( RunQueued = "queued"; RunPackaging = "packaging"; RunUploading = "uploading"; RunCreatingDataset = "creating_dataset";
    RunWaitingDataset = "waiting_dataset"; RunStarting = "starting"; RunRunning = "running"; RunDownloading = "downloading";
    RunVerifying = "verifying"; RunCleaning = "cleaning"; RunSucceeded = "succeeded"; RunFailed = "failed"; RunCancelled = "cancelled" )
func RunTerminal(state string) bool
type Run struct { /* JSON = the API shape "Run" below; RunRemote holds everything kept in remote_json */ }
type RunRemote struct { /* owner, dataset and notebook slugs, kernel version, per-step Kaggle job ids and upload refs, remote status,
    untracked flag, timestamps, latest stage, live flag, result summary, warning code, split counts, estimate, method */ }
func (s *Store) CreateRun(ctx context.Context, r Run) error                       // ErrRunActive when another run is not terminal
func (s *Store) GetRun(ctx context.Context, id string) (Run, error)               // ErrNotFound
func (s *Store) ListRuns(ctx context.Context, projectID string, limit int) ([]Run, error)
func (s *Store) ActiveRuns(ctx context.Context) ([]Run, error)
// UpdateRun loads the run, calls fn, stores it and appends one run_events row in one transaction.
// It publishes nothing; the caller publishes after the commit.
func (s *Store) UpdateRun(ctx context.Context, id string, ev RunEvent, fn func(*Run) error) (Run, error)
func (s *Store) DeleteRun(ctx context.Context, id string) error                   // ErrRunNotTerminal
type RunEvent struct { ID int64 `json:"id"`; TS string `json:"ts"`; Kind string `json:"kind"`; Data json.RawMessage `json:"data"` }
func (s *Store) AddRunEvent(ctx context.Context, runID string, ev RunEvent) error // keeps the newest 5,000 per run
func (s *Store) ListRunEvents(ctx context.Context, runID string, afterID int64, limit int) ([]RunEvent, error)
func (s *Store) UpsertMetrics(ctx context.Context, runID string, m []Metric) error
func (s *Store) ReplaceMetrics(ctx context.Context, runID string, m []Metric) error
func (s *Store) ListMetrics(ctx context.Context, runID string, sinceStep int) ([]Metric, error)
type Artifact struct { ID, RunID, ProjectID, Kind, Name, RelPath string; Bytes int64; SHA256, Created string }
func (s *Store) AddArtifact(ctx context.Context, a Artifact) error
func (s *Store) ListArtifacts(ctx context.Context, runID string) ([]Artifact, error)
func (s *Store) GetArtifact(ctx context.Context, id string) (Artifact, error)
func (s *Store) SaveReport(ctx context.Context, runID, kind string, result any) error
func (s *Store) GetReport(ctx context.Context, runID, kind string) (json.RawMessage, bool, error)
func (s *Store) RecordConsent(ctx context.Context, profileID, licenseID, version string) (Consent, error) // idempotent
func (s *Store) HasConsent(ctx context.Context, profileID, licenseID, version string) (bool, error)
func (s *Store) ListConsents(ctx context.Context) ([]Consent, error)
func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error)
func (s *Store) SetSetting(ctx context.Context, key, value string) error

// plantoken.go (copy of the DeleteTokens pattern in confirm.go)
const PlanTokenTTL = 15 * time.Minute
type PlanTicket struct { ProjectID, RunID, Preset, Hash string; Expires time.Time }
type PlanTokens struct{ /* in memory, at most 64 */ }
func NewPlanTokens(now func() time.Time) *PlanTokens
func (p *PlanTokens) Issue(t PlanTicket) (token string, ttl time.Duration)
func (p *PlanTokens) Consume(token, projectID string) (PlanTicket, bool) // always removes the token
// PlanHash is the sha256 over the canonical plan JSON, data_rev, profile SHA, script SHA, owner, machine shape and delete flag.
func PlanHash(plan Plan, profileSHA, scriptSHA, owner, machineShape string, deleteAfterRun bool) string

// preflight.go (pure; no I/O)
type PreflightInput struct { /* plain values the server collects: Kaggle flags and limits, access, credentials, vault,
    connection result, quota, active compute jobs, storage, consent, check state, plan, profile, machine shape, admin */ }
type PreflightItem struct {
    ID         string         `json:"id"`
    Group      string         `json:"group"`  // kaggle | account | quota | data | license | storage
    OK         bool           `json:"ok"`
    Level      string         `json:"level"`  // block | warn
    MessageKey string         `json:"message_key"`
    Args       map[string]any `json:"args,omitempty"`
    Flag       string         `json:"flag,omitempty"` // a name the enable route accepts
    Link       string         `json:"link,omitempty"`
}
type Preflight struct { Ready bool `json:"ready"`; Items []PreflightItem `json:"items"` }
func EvaluatePreflight(in PreflightInput) Preflight
var PreflightFlags = []string{ /* the flag names below */ }

// report.go
type Report struct { /* API shape "Report" below */ }
func ScoreReport(recipe, taskKind string, labels []string, st Status, samples []EvalSample) Report

// driver.go
type KaggleDriver interface { /* the ten methods of the spec, section "Kaggle orchestration" */ }
// fakekaggle_test.go: fakeKaggle, an in-memory KaggleDriver with scripted outcomes per operation
// (completed, failed, unknown, remote states, log bodies, output files, checksum faults).
```

#### B4 provides

```go
// reconcile.go
type OrchestratorConfig struct {
    MachineShape         string
    DeleteRemoteAfterRun bool
    AllowDelete          bool
    RemoteTimeoutMinutes int
    AuraGoVersion        string
}
type Orchestrator struct{ /* store, hub, driver, catalog, data dir, config func, clock, intervals, logger */ }
func NewOrchestrator(st *Store, hub *Hub, cat *Catalog, drv KaggleDriver, dataDir string, cfg func() OrchestratorConfig) *Orchestrator
func (o *Orchestrator) Start(ctx context.Context)                                   // reconcile loop; resumes every non-terminal run
func (o *Orchestrator) Submit(ctx context.Context, in SubmitInput) (Run, error)      // creates the run in state queued and wakes the loop
func (o *Orchestrator) Cancel(ctx context.Context, runID string) (Run, error)
func (o *Orchestrator) Step(ctx context.Context, runID string) (changed bool, err error) // exactly one transition; tests drive restarts with it
func (o *Orchestrator) Close()
type SubmitInput struct { RunID string; Project Project; Profile *Profile; ProfileSHA string; Plan Plan; Owner string }

// artifacts.go
func RunDir(dataDir, runID string) string   // <data>/chibi_sensei/runs/<run_id>
func ModelDir(dataDir, runID string) string // <data>/chibi_sensei/models/<run_id>
func BaseDir(dataDir string) string         // <data>/chibi_sensei/base
func StorageBytes(dataDir string) int64     // bytes under <data>/chibi_sensei

// gguf.go
type GGUFInfo struct { Version uint32; Tensors uint64; Architecture string }
func ReadGGUFInfo(path string) (GGUFInfo, error) // header and metadata only
```

Hub events the orchestrator publishes on the project stream: `run` (data: Run summary), `metric` (data: `{"run_id","points":[Metric]}`), `stage` (data: `{"run_id","stage","state"}`).

### HTTP contract additions

Base path `/api/desktop/chibi-sensei`; P1 conventions apply (flat errors, same-origin writes, RFC 3339 UTC times).

#### Meta

`GET /meta` gains:

```json
"training":{"backend":"kaggle","machine_shape":"NvidiaTeslaT4","delete_remote_after_run":true},
"run_states":["queued","packaging","uploading","creating_dataset","waiting_dataset","starting","running","downloading","verifying","cleaning","succeeded","failed","cancelled"]
```

Each profile's `license` object gains `version` and `accepted` (bool).

#### Plan

`GET /projects/{id}/plan?preset=` gains the fields `eval_samples`, `max_new_tokens`, `max_train_seconds`, `need_base_gguf`, `artifacts_estimated_mb`; `trainable` is true and `reason_key` is empty when the plan computes.

#### Preflight

- `GET /projects/{id}/preflight?preset=quick|balanced|thorough` →

```json
{"ready":false,"admin":true,"preset":"balanced","checked_at":"…",
 "items":[{"id":"kaggle_allow_gpu","group":"kaggle","ok":false,"level":"block",
           "message_key":"chibi_sensei.preflight.kaggle_allow_gpu","args":{},"flag":"kaggle.allow_gpu","link":""}],
 "fix":{"flags":["kaggle.enabled","kaggle.allow_gpu"]}}
```

`fix` is null when no failing item has a flag. Item IDs, in this order:

| ID | Group | Level | Flag | OK when |
|---|---|---|---|---|
| `dataset_ready` | data | block | | the latest check result is for the current `data_rev` and ready |
| `run_active` | data | block | | no run is active |
| `kaggle_enabled` | kaggle | block | `kaggle.enabled` | `kaggle.enabled` |
| `kaggle_not_readonly` | kaggle | block | `kaggle.read_only` | not read-only |
| `kaggle_allow_create` | kaggle | block | `kaggle.allow_create` | granted |
| `kaggle_allow_remote_execute` | kaggle | block | `kaggle.allow_remote_execute` | granted |
| `kaggle_allow_gpu` | kaggle | block | `kaggle.allow_gpu` | granted |
| `kaggle_allow_internet` | kaggle | block | `kaggle.allow_internet` | granted |
| `kaggle_allow_download` | kaggle | block | `kaggle.allow_download` | granted |
| `kaggle_machine_shape` | kaggle | block | `kaggle.allowed_machine_shapes` | the list contains `chibi_sensei.training.machine_shape` |
| `kaggle_allow_delete` | kaggle | warn | `kaggle.allow_delete` | granted, or `delete_remote_after_run` is false |
| `network_access` | account | block | | `Access.Network` |
| `filesystem_write` | account | block | | `Access.Write` |
| `kaggle_vault` | account | block | | the Vault is available |
| `kaggle_credentials` | account | block | | credentials are stored (link `/config#kaggle`) |
| `kaggle_connection` | account | block | | `Service.Test` succeeds (skipped, and reported not OK, while an earlier Kaggle item fails) |
| `kaggle_timeout` | quota | warn | | `remote_timeout_minutes` ≥ 1.5 × estimate + 15 (args: `minutes`, `needed`) |
| `kaggle_quota` | quota | block / warn | | remaining GPU hours ≥ 1.2 × estimate; warn when the answer cannot be parsed |
| `kaggle_slot` | quota | block | | no active or `unknown` Kaggle compute job (args: `jobs`) |
| `kaggle_transfer` | storage | block | | `max_transfer_mb` and the free Kaggle project storage hold the largest artifact |
| `disk_space` | storage | block | | free disk and `chibi_sensei.storage.max_gb` hold 1.2 × `artifacts_estimated_mb` |
| `license_consent` | license | block | | the profile needs no consent, or consent for its license version is recorded |

- `POST /preflight/enable` `{"flags":["kaggle.enabled","kaggle.allow_gpu"]}` → `{"applied":["kaggle.enabled","kaggle.allow_gpu"]}`. Admin only (403 `admin_required`). A flag outside `PreflightFlags` answers 422. The server maps a flag to its value itself: `kaggle.enabled` → true, `kaggle.read_only` → false, `kaggle.allow_*` → true, `kaggle.allowed_machine_shapes` → the list plus the configured machine shape. It then applies one config patch through `applyConfigPatch`.

#### License consent

- `POST /consents` `{"profile_id":"gemma3-270m"}` → 201 `{"consent":{"profile_id","license_id","license_version","accepted_at"}}`. Idempotent. 422 `profile_unknown`.

#### Runs

- `POST /projects/{id}/runs/prepare` `{"preset":"quick|balanced|thorough"}` → 200

```json
{"plan_token":"…","expires_in_s":900,"run_id":"<uuid>","plan":Plan,
 "destinations":{"owner":"andi","dataset":"andi/chibi-data-0123456789ab","notebook":"andi/chibi-1a2b3c4d-0123456789ab",
                 "machine_shape":"NvidiaTeslaT4","private":true},
 "data":{"rows":{"train":371,"val":21,"eval":20},"bytes":183422},
 "license":{"id":"gemma","name":"Gemma Terms of Use","url":"…","version":"1.0","requires_consent":true,"accepted":true},
 "time":{"estimate_minutes":34,"session_timeout_minutes":66,"quota_hours_remaining":27.5},
 "cleanup":{"delete_remote_after_run":true,"allow_delete":true}}
```

  It recomputes the preflight; a blocking item answers 409 `preflight_failed` with `missing: ["<item id>", …]`. `quota_hours_remaining` is null when unknown. `data.bytes` is the size of the canonical JSONL of the three splits.
- `POST /projects/{id}/runs` `{"plan_token":"…"}` → 202 `{"run":Run}`. The server consumes the token, recomputes plan and preflight, and answers 409 `plan_expired` when the token is unknown, expired or the hash differs. For a profile without required consent the start records the consent row.
- `GET /projects/{id}/runs?limit=20` → `{"runs":[RunSummary]}` newest first.
- `GET /runs/{runId}` → `{"run":Run}`
- `POST /runs/{runId}/cancel` → `{"run":Run}`. Classified as a stop operation (like job cancel), so a read-only desktop may cancel.
- `GET /runs/{runId}/metrics?since_step=0` → `{"metrics":[{"step":10,"kind":"train","epoch":0.1,"loss":2.1,"lr":0.0002,"grad_norm":1.2,"tok_s":2400,"ts":"…"}],"max_steps":480,"live":"yes|no|unknown"}`
- `GET /runs/{runId}/events?after=0&limit=200` → `{"events":[{"id":12,"ts":"…","kind":"state|stage|warn|error|kaggle|log","data":{…}}]}`
- `GET /runs/{runId}/artifacts` → `{"artifacts":[{"id":"…","kind":"gguf_tuned","name":"model-q8_0.gguf","bytes":912345678,"sha256":"…","created":"…"}]}`
- `GET /runs/{runId}/artifacts/{artifactId}/file` → the file as an attachment (`Content-Disposition` with the server-generated name). Follow the repository's precedent for large downloads through `internal/httpstream`; if none fits a 1.3 GB file, say so under "Contract changes" and leave the route out.
- `GET /runs/{runId}/report` → `{"report":Report|null}`
- `GET /runs/{runId}/log` → `{"tail":"…","bytes":65536}` (last 64 KiB, scrubbed)
- `POST /runs/{runId}/delete` → 204. Terminal runs only (409 `run_not_terminal`). Removes the run's local files, artifacts and rows. It does not touch Kaggle.

```json
Run = {"id":"…","project_id":"…","backend":"kaggle",
  "state":"queued|packaging|uploading|creating_dataset|waiting_dataset|starting|running|downloading|verifying|cleaning|succeeded|failed|cancelled",
  "profile_id":"qwen35-0p8b","profile_status":"experimental","preset":"balanced","method":"lora",
  "hyper":{…},"split":{"train":371,"val":21,"eval":20},
  "stage":{"name":"train","state":"start","step":120,"max_steps":480,"epoch":0.5,"eta_s":700},
  "live":"yes|no|unknown",
  "estimate_minutes":34,"session_timeout_minutes":66,"deadline":"…","remote_started":"…","finished":"…",
  "cancel_requested":false,
  "cancel":{"remote_stopped":false,"stop_url":"https://www.kaggle.com/code/andi/chibi-1a2b3c4d-0123456789ab","time_limit_minutes":66},
  "remote":{"owner":"andi","dataset":"andi/chibi-data-0123456789ab","notebook":"andi/chibi-1a2b3c4d-0123456789ab",
            "notebook_url":"https://www.kaggle.com/code/andi/chibi-1a2b3c4d-0123456789ab","status":"RUNNING","untracked":false,
            "kaggle_jobs":[{"step":"upload_train","job_id":"…"}]},
  "error_code":"","error":"","warning_code":"",
  "result":{"steps":480,"epochs":2,"train_loss":0.92,"eval_loss_base":2.31,"eval_loss_tuned":1.12,"stop_reason":"done",
            "gpu":"Tesla T4","versions":{"python":"3.13","torch":"…","transformers":"…","peft":"…","llama_cpp":"b9994"}},
  "leftovers":[{"kind":"dataset","ref":"andi/chibi-data-0123456789ab","url":"https://www.kaggle.com/datasets/andi/chibi-data-0123456789ab"}],
  "created":"…","updated":"…"}
RunSummary = Run without hyper, remote.kaggle_jobs and result.versions
Report = {"run_id":"…","kind":"auto","samples":20,"small_sample":true,
  "loss":{"base":2.31,"tuned":1.12},
  "exact_match":{"base":0.2,"tuned":0.85,"n":20},
  "label_accuracy":{"base":0.4,"tuned":0.9,"n":20,"labels":["pos","neg"],"confusion":{"base":[[4,6],[6,4]],"tuned":[[9,1],[1,9]]}},
  "perplexity":{"base":10.1,"tuned":3.1}}
```

`stage`, `cancel`, `result` and each Report part are null when they do not apply: `exact_match` only for `extract`, `rewrite` and `to_json`; `label_accuracy` only for `classify`; `perplexity` only for `free`. `cancel` is non-null once `cancel_requested` is true.

#### Project steps

`steps.training` is `todo` while the data step is not ready, `ready` when it is and no run exists, `in_progress` while a run is active, `done` after a succeeded run, `blocked` when the latest run failed or was cancelled and none succeeded. `test` and `use` stay `todo`.

#### Events

`GET /projects/{id}/events` gains the events `run`, `metric` and `stage` (payloads under "B4 provides"). After the first `rows` event the stream sends the active run of the project, if any, as a `run` event.

#### New error codes

`preflight_failed` 409 (with `missing`), `plan_expired` 409, `run_active` 409, `run_not_terminal` 409, `run_not_found` 404, `artifact_not_found` 404, `admin_required` 403, `kaggle_unavailable` 503, `license_consent_required` 409. Run failure codes (in `Run.error_code`, never as an HTTP error) are the list in the spec, section "Error handling", plus `lora_targets_missing`, `cancelled_by_user` and `desktop_readonly`.

#### Config

```yaml
chibi_sensei:
  training:
    machine_shape: "NvidiaTeslaT4"
    delete_remote_after_run: true
kaggle:
  allowed_machine_shapes: []
```

### Trainer contract additions

On top of the spec's "Training contract":

- Arguments: `--job <file> --out <dir>`, `--selftest`, `--dry-run`. Standard library only at module top; `torch`, `transformers`, `peft`, `huggingface_hub` and `gguf` are imported inside the stages.
- `env_check` logs the GPU facts before anything is installed, and fails fast as the spec says.
- `install`: `pip install` the lock's `pip` lines; reinstall `transformers` only when the lock names a version and it differs; never touch `torch`. Then import checks and a `versions` log line.
- `load_base`: `huggingface_hub.snapshot_download(repo, revision)` without a token (`HF_TOKEN` and `HUGGING_FACE_HUB_TOKEN` are removed from the environment first), then the loading chain of D5 with `dtype=torch.float32` (falling back to the `torch_dtype` keyword), `.to("cuda:0")`, `CUDA_VISIBLE_DEVICES=0` set before `torch` is imported.
- `base_eval` and `tuned_eval`: mean loss over `eval` with the masking of D7, then greedy answers (`do_sample=False`, `max_new_tokens` from the job) for the first `eval_max_samples` rows, stopping at the tokenizer's EOS and at `chat.stop`.
- `train`: the manual loop of D2. Dynamic padding, `per_device_batch`, `grad_accum`, optional gradient checkpointing, a metric line every `metric_every_steps` steps, an `eval` metric on `val` at the end of each epoch when `val` exists. Stops: `time_budget` at `max_train_s`, `train_diverged` on a NaN or infinite loss, `oom` on a CUDA out-of-memory error.
- `save`: LoRA writes the flat adapter files (D10), then `merge_and_unload()`; full takes the trained weights. `save_pretrained(merged, safe_serialization=True)`, then the tokenizer files of D8.
- `gguf`: download the tarball with `urllib`, verify or log its sha256, extract, `pip install ./gguf-py`, run `convert_hf_to_gguf.py <merged> --outtype q8_0 --outfile <out>/chibi-model-q8_0.gguf`, and for the base the snapshot directory to `chibi-base-q8_0.gguf` when `emit_base_gguf`. Environment fallback of D9. Verify each file with `gguf.GGUFReader`.
- The base GGUF is optional output: when less than 6 minutes of the session budget are left before it, the trainer skips it with a `warn` line and `warning_code` `base_gguf_skipped`.
- `chibi-status.json` is written in a `finally` block, on success and on failure.
- `--dry-run` with `CHIBI_FAKE_ENV=1` imports none of the heavy libraries, runs every stage with stubs, and writes real-shaped outputs including a minimal valid GGUF v3 file (magic, version, one tensor, the key `general.architecture`) that `ReadGGUFInfo` accepts.
- The script never reads a secret and never prints the environment.

### Amendments (after the first plan parts came back)

From the B1 plan (`plan-b1.md`, "Contract changes"), binding for B4 and B5:

- A1. `ReadKernelLog(ctx, access, owner, slug, version, maxBytes)`: `slug` is the bare kernel slug without the owner; `version` ≥ 1. It returns `nil, nil` when no log exists yet (Kaggle 404, storage 404, 204 or an empty body). The text is already scrubbed. A 200 body that is a JSON object is refused with an error (it might carry a signed URL); a JSON array or plain text is returned. `maxBytes <= 0` or above 16 MiB means 16 MiB. Each call makes two Kaggle requests (identity, then the log).
- A2. The orchestrator calls `ReadKernelLog` with `maxBytes = 16 << 20` for parsing, because a tail cut out of a JSON-array log is not valid JSON. It keeps only the last 64 KiB of the joined text for display and for the log-tail artifact.
- A3. `RemoveFile(project, name)` applies no policy gate (it is in-process cleanup of files Chibi staged). The driver adapter in B5 decides when to call it.
- A4. Helpers: `config.CanonicalKaggleMachineShape(name) (string, bool)`, `config.DefaultChibiSenseiMachineShape = "NvidiaTeslaT4"`. `kaggle.allowed_machine_shapes` is never nil after load.
- A5. `CheckGrants` refusal texts: `custom Kaggle compute images are not supported`, `a Kaggle machine shape requires a GPU run and the GPU permission`, `Kaggle machine shape is not in kaggle.allowed_machine_shapes`.

From the coordinator:

- A6. The lock file pins the llama.cpp source tarball: sha256 `8e8873031f05ec06ebf464a66fa4295edcc449d3068f3e94c6be88be7f632390` (35,736,748 bytes, downloaded 2026-11-23). The GGUF architecture strings at that tag are `gemma3`, `lfm2` and `qwen35` (read in `gguf-py/gguf/constants.py`).
- A7. The trainer accepts `CHIBI_ALLOW_CPU=1` for local smoke runs: `env_check` then does not require CUDA, the device is `cpu`, the GPU facts in the status are `{"name":"cpu","capability":"0.0","vram_gb":0}`, and a `warn` line says the run is a CPU smoke run. Nothing in AuraGo sets this variable; it exists so the coordinator can run the real pipeline in a container before a Kaggle run.
- A8. `agotrain` needs no reconcile: `machineShape` does not occur in `../agotrain/training/operation_contracts.json` or `tool_tiers.json` (checked 2026-11-23).

From the B2a plan (`plan-b2a.md`, "Contract changes"), binding for the trainer (B2b), B3, B4 and B5:

- A9. Marker file `r<run12>-marker.json`: `{"v":1,"run_id","format","data_rev","files":{"<split>":{"name","sha256","rows","bytes"}}}`; a split with zero rows has no entry.
- A10. Metric lines and the lines of `chibi-metrics.jsonl` carry `kind` (`train` or `eval`); an eval metric puts the validation loss in `loss`. A metric line needs `step` and a loss.
- A11. The wrapper's exit line is `@@chibi {"v":1,"t":"exit","code":rc}` without `seq`; every other line needs a `seq`.
- A12. Stage names are the fixed list `boot, env_check, install, data, load_base, base_eval, train, tuned_eval, save, gguf, manifest` (`TrainerStages`); the parsers reject other names.
- A13. Manifest files are fixed per kind, each at most once: `gguf_tuned` = `chibi-model-q8_0.gguf`, `gguf_base` = `chibi-base-q8_0.gguf`, `adapter` = `chibi-adapter-model.safetensors`, `adapter_config` = `chibi-adapter-config.json`, `metrics` = `chibi-metrics.jsonl`, `eval_samples` = `chibi-eval_samples.jsonl`, `status` = `chibi-status.json` (optional). The manifest never lists itself.
- A14. An eval sample's `id` is the 0-based line index in the eval file (string or number).
- A15. Snapshot lines are `{"messages":[…]}` or `{"text":"…"}` per row; a free project may contain both. The trainer handles each line by its shape; `hyper.loss_on` (`assistant` | `all`) decides the masking for chat lines.
- A16. `hyper.lora` is omitted for `full`. `deps.pip` is the lock's pip lines plus the profile's `extra_pip`.
- A17. `MaxTrainSeconds` for preset `quick` is capped at 1800.
- A18. Helpers to reuse: `SnapshotFileName`, `MarkerFileName`, `DatasetSlug(runID)` = `chibi-data-<run12>`, `NotebookSlug(projectID, runID)` = `chibi-<first 8 hex of the project id>-<run12>`, `FormatFor`, `ExitSeq`, `TrainerStages`, `ManifestKindFile`, the `Out*` file-name constants, `LogEvent.ExitCode`, `FlexString`, `ValidManifestPath`, `trainer.ParseLock`, `trainer.ValidPin`.
- A19. `.gitattributes` keeps LF for `internal/chibisensei/trainer/**` and `internal/chibisensei/testdata/**`.
- A20. B5 records in `internal/chibisensei/AGENTS.md`: the trainer embed and the lock format, the golden and proof-data `-update` commands, the `.gitattributes` lines, `TestProofBundle`, the stage list and the output names.

From the B3 plan (`plan-b3.md`, "Contract changes"), binding for B4, B5 and the frontend:

- A21. Store files: `runs.go` (model, states, JSON view), `runstore.go` (SQL), `consents.go` (consents, settings), `quota.go`, `preflight.go`, `report.go`, `plantoken.go`, `driver.go`. `schema.go` gains four `CREATE INDEX IF NOT EXISTS` lines.
- A22. `UpdateRun` with an empty event kind writes no event.
- A23. `Run.deadline`, `remote_started` and `finished` are `""` when absent, never null.
- A24. Report: `loss` and `perplexity` values may each be null; `label_accuracy.confusion.base|tuned` has one row per label and `len(labels)+1` columns, the last column counting answers that match no label; a sample whose expected answer is no project label is left out of `label_accuracy`.
- A25. `GET /runs/{id}/metrics?since_step=N` is inclusive (`step >= N`); the client replaces points with the same `(step, kind)`. A NaN or infinite loss is not stored.
- A26. `DeleteProject` refuses a project with a non-terminal run (`ErrRunActive` → 409 `run_active`); `ErrRunNotTerminal` → 409 `run_not_terminal`. B5 maps both in `chibiDomainError`.
- A27. `RemoveRunFiles(dataDir, runID)` (B4, `artifacts.go`) removes the run and model directories; B5 calls it after `DeleteRun`, and for a project delete after `DeleteProject` for every ID from `ListRunIDs`.
- A28. The Kaggle project id lives in the Chibi setting `kaggle_project_id` (`SettingKaggleProjectID`); the adapter recreates the project when the id no longer resolves.
- A29. Artifact JSON hides `run_id`, `project_id` and `rel_path`; `name` is the base name of `rel_path`.
- A30. Preflight item args: `run_active` `{run_id, project_id}`; `kaggle_machine_shape` `{shape}`; `kaggle_connection` `{account}` or `{skipped:true}`; `kaggle_timeout` `{minutes, needed}`; `kaggle_quota` `{hours, needed}`, `{unknown:true}` or `{skipped:true}`; `kaggle_slot` `{jobs, unknown}` or `{skipped:true}`; `kaggle_transfer` `{needed_mb, max_transfer_mb, free_mb}`; `disk_space` `{needed_mb, free_mb, cap_free_mb}`; `license_consent` `{license_id, license_version}`. A skipped quota is `warn`; an unreadable compute-job list makes `kaggle_slot` `block`.
- A31. `DatasetSlug`, `NotebookSlug` and `Run12` are defined once, in `snapshot.go` (B2a). B3 uses them and does not define them again.

From the frontend plan (`plan-frontend.md`, "Contract changes"), binding for B3, B4 and B5:

- A32. `GET /meta` profiles gain `methods` (string array) and `max_seq_cap` (int): two new fields of `ProfileSummary` in `profiles.go`, set in `(*Profile).Summary()`. B5 adds them.
- A33. `Run` and `RunSummary` gain `last_state`: the last non-terminal state the run was in (equal to `state` while the run is active). It lives in `RunRemote` (`remote_json`); the orchestrator sets it on every transition into a non-terminal state, and `Submit` sets it to `queued`.
- A34. `run_events.data` shapes, exactly: kind `state` → `{"to":"<state>"}`; `stage` → `{"stage":"<name>","state":"start|done|failed"}`; `warn` → `{"code":"…","message":"…"}`; `error` → `{"stage":"…","code":"…","message":"…"}`; `kaggle` → `{"step":"…","job_id":"…"}`; `log` → `{"text":"…"}`.
- A35. Artifact downloads in the UI are plain same-origin anchors to `GET /runs/{id}/artifacts/{aid}/file`; B5 must say clearly whether the route ships.

### Proof runs (coordinator)

1. After B1 and B2: build the branch for Linux with the asset set, start the scratch instance on `aurago-test` (own port and data directory, a dead `DOCKER_HOST`, the copied vault and config, grants of U3).
2. Generate the proof bundle per profile with the gated test of B2 (`CHIBI_PROOF_OUT`, `CHIBI_PROOF_PROFILE`): data files, marker, `job.json`, notebook.
3. Push by hand through the scratch instance's Kaggle API: create a project, write the four files, `upload_dataset_file` each, `create_dataset`, poll `get_dataset_status`, `save_kernel` with the notebook and `machineShape`, watch the job, try `ReadKernelLog` while it runs, `list_kernel_session_output`, `download_kernel_output` for every manifest file, verify the hashes.
4. Load each GGUF with the pinned llama.cpp `b9994` build on `aurago-test` and ask three prompts.
5. Record per profile: pins, GPU name, measured tokens per second, the class that loaded the base, the LoRA targets, the Kaggle reference, the real response shapes of the status and quota operations. Flip the profile to `verified` and empty `unverified`, or report the failure with the evidence and a replacement candidate to the user.
6. Order: Gemma 3 270M (specialist, classify), then Qwen3.5-0.8B (assistant), then LFM2.5-1.2B (assistant). The Gemma run also answers the Kaggle questions of the risk table (machine shape, live log, output download, `licenseName`).
7. After B5: one end-to-end run through the real orchestrator on the scratch instance with the smallest profile.

## Execution record

What happened when the plan was carried out, where it differs from the parts as written. The parts are kept as planned; this list and the amendments say what is true.

- **Mechanical assembly.** The trainer script (`chibi_train.py`), its Go test and the six orchestrator files (`reconcile*.go`) were assembled from the fenced code blocks of parts B2b and B4 by a script, not retyped. Both compiled against the code that the other batches had produced.
- **CPU smoke runs before any Kaggle run.** The generated notebook cell was run as is in a CPU container (`CHIBI_ALLOW_CPU=1`, amendment A7) for each profile. They found three defects that a GPU run would have paid for:
  - the converter flag `--no-mtp` was passed for every architecture, and the converter accepts it only for Qwen3.5 (the Gemma base conversion failed in both converter environments);
  - the eval prompt was cut to `max_seq_len - max_new_tokens` tokens, which is 8 for the assistant recipe, so the sample answers ignored the question;
  - two placeholder URLs in the trainer's selftest were outside the allow-list of the payload security test.
- **Stage list.** The trainer part moved `data` before `install` and added the stage `converter` after `load_base`; `TrainerStages` and the UI stage list follow it.
- **Eval samples.** An answer that was not generated is JSON null (`EvalSample` uses pointer fields); `ScoreReport` leaves such samples out and reports the counts it scored.
- **`last_state`.** Set on every transition into a non-terminal state, except into `cleaning` when the run is ending in failure or cancel; such a run keeps the state it stopped in. State events carry `to`, `state` and `from`.
- **Notebook wrapper.** The cell pipes the trainer's output and prints it line by line instead of calling `subprocess.call`, so the log does not depend on the kernel capturing a child's file descriptors.
- **Lock.** `peft==0.22.0` (the version the CPU smoke runs used with transformers 5.24.0) and the measured hash of the llama.cpp `b9994` source tarball.
- **Ignored files.** `.gitignore` ignores `*.lock` as well as `AGENTS.md` and `docs/`; `trainer/requirements.lock` had to be force-added.
- **Translations.** The 14 other locales were produced as one JSON file per language and merged by a script that checks key sets and placeholders; six single words that are the same in a language and in English are listed in the locale test.
- **Stop texts.** The spec's promise "AuraGo stops waiting at once" did not match the rule that a run keeps tracking its notebook after submission. The rule stayed; the texts (16 locales) and the spec now say that AuraGo keeps watching the notebook until it has ended.
- **Reviews after B4.** Three read-only reviews (orchestrator against the spec, domain logic, security) found no critical defect and these important ones, all fixed with a test each:
  - the log scan was quadratic on a body of repeated `"stream_name"` keys and dropped the plain lines of a plain-text log that contained one stream object; it is linear now (bounded decode window, at most 200000 entries) and keeps both;
  - a run could replace or delete the base GGUF that other runs share; a stored file that matches its recorded hash is authoritative now, a manifest that declares another hash fails with `checksum_mismatch`, and a run removes a base file only if it stored it and nobody else refers to it;
  - a stop in `cleaning` was shown as accepted while the run ended `succeeded`; it changes nothing now, and a stop stored before `cleaning` wins (files already downloaded stay with the run);
  - a crash between the local copy of a result and the removal of its staged file caused a second download; the copy is committed first;
  - staged files that could not be removed were reported as cleaned up; removal is retried five times and then named in the warning `staged_left`;
  - a read-only desktop or a switched-off Kaggle setting held the single run slot for up to six hours; a run that has not reached Kaggle fails at once (`desktop_readonly`, new code `kaggle_refused`), a submitted run keeps waiting and says why;
  - one failed status read after the deadline ended a healthy run with `remote_timeout`; the run waits 30 minutes for a successful read first;
  - a submit that Kaggle refused was listed as a leftover notebook; leftovers need evidence that Kaggle has the object (or an unknown outcome);
  - the trainer's output error used the stage name `finalize`, which the parser drops; it is `manifest`;
  - a score with no scored samples was reported as 0; it is null;
  - a lock without the llama.cpp tarball hash was accepted; the hash is mandatory.
- **Decided not to change:** the deadline compares wall-clock timestamps (a clock jump is accepted); `logReadMaxBytes` stays 16 MiB (the spec said 4 MiB).
- **Waiting notes.** The warn event of a waiting run carries only a code; the UI had no text for `kaggle_unavailable` and `desktop_readonly` and showed the generic note. Both, `kaggle_refused` and `staged_left` have texts in 16 locales.
- **B5.3 detail.** `min(...)` in the artifact download window is written as an `if`, because a test file of package `server` defines its own `min`.
