# Chibi Sensei: small-model training studio

Status: design approved by the user on 2026-11-22 and checked against the code that day. P1 (dataset studio), P2 (training on Kaggle) and P3 (test and compare, native runtime only; the Docker sidecar, the provider column and the LLM judge are follow-ups) are implemented; the plans under `docs/superpowers/plans/` record what was built and where it differs from this design. Each phase gets its own implementation plan; the user asked for the P1 plan and its implementation to follow without a further gate. Values marked UNVERIFIED stay unproven until the run named under "Risks" passes.

## Goal

Chibi Sensei is a first-party desktop app that walks one person through fine-tuning a small LLM:

- define the goal;
- build and check a dataset, with help from the AuraGo agent and Hugging Face imports;
- train on the user's own Kaggle GPU quota (later also on a local NVIDIA GPU);
- test and compare the result locally;
- take the model into use.

The app automates every step it can. Every automatic choice is shown and can be overridden. It never widens a permission silently, and training data leaves the machine only after a confirmation that names where the data goes.

## Decisions the user made (do not re-ask)

1. Launch base models: Gemma 3 270M, Qwen3.5-0.8B and LFM2.5-1.2B. The catalog stays extensible.
2. Launch recipes:
   - Assistant with style and knowledge.
   - Specialist for fixed tasks: classify, extract, rewrite, text to JSON.
   - Free training: raw text or the user's own prompt format.
   - No tool-calling recipe.
3. Local GPU training runs through a managed NVIDIA Docker container in a later phase; Kaggle comes first. Local testing works on CPU, Vulkan or NVIDIA.
4. The design below is approved as a whole: the five-step path, one own training script for every backend, and the phases P1 to P5 with Hugging Face import in P4.
5. The proof runs of all three model profiles happen in P2, not P3 (see "Phases"). They use the user's Kaggle account on the test server and are asked for separately before P2 starts.
6. Defaults that were put to the user and accepted:
   - `chibi_sensei.enabled` defaults to true.
   - A run deletes its Kaggle dataset and notebook afterwards when deletion is allowed.
   - The comparison base GGUF comes from the training run, cached per profile revision.
   - Gemma comes from the ungated mirror with the license consent recorded in AuraGo; Kaggle Models is the fallback.
   - "Serve as provider" stays in P4, admin-only and explicitly pinned.

Background to decision 1: Liquid released the LFM3 family on 2026-10-27. It starts at 1.7B, so LFM2.5-1.2B stays the choice for the 1B tier and `LFM3-1.7B` is a later catalog candidate.

## Vocabulary

- **Project**: one training goal. It holds a recipe, a base model profile, a dataset, runs and a goal text.
- **Recipe**: `assistant`, `specialist` (with `task_kind`: `classify | extract | rewrite | to_json`) or `free`.
- **Model profile**: the data-driven description of one base model (source, template, training defaults, GGUF facts). Its status is `experimental` or `verified`.
- **Row**: one training example. The canonical forms are `{"messages":[{"role","content"}]}` and `{"text"}` (free recipe only).
- **Check**: an automatic dataset test. Its result level is `ok | info | warn | block`.
- **Preset**: `quick | balanced | thorough`. It maps dataset size and profile to hyperparameters.
- **Plan**: the computed hyperparameters, split, time estimate and destinations for a run, plus a `plan_token`.
- **Run**: one training attempt. It moves through a persisted state machine and runs on the `kaggle` backend (later `local_nvidia`).
- **Artifact**: a registered output file (tuned GGUF, base GGUF, adapter, metrics, eval samples, manifest, status, log tail).
- **Report card**: the evaluation view, built from `eval_samples.jsonl` without local compute.
- **Runtime instance**: one local llama.cpp chat server serving one GGUF. At most two run at once.
- **Sensei**: the helper panel that runs an app-scoped agent loop with `chibi_`-prefixed tools.
- **Proposal**: a costly or destructive action that the agent suggests and only the user can start.

## User journey

Each project has a five-step rail. Each step shows a readiness state (`todo | in_progress | ready | done | blocked`) and has one primary action. The Sensei panel and the mascot are always visible. An "Advanced" disclosure in each step shows every automatic decision and lets the user override it.

1. **Goal** (primary action: "Continue to data")
   - The user picks a recipe, writes the task in one paragraph and, for the specialist recipe, picks `task_kind` and the labels.
   - The app suggests a profile. The rule prefers verified profiles:
     - specialist with short inputs → Gemma 3 270M;
     - assistant in a language outside LFM's eight → Qwen3.5-0.8B;
     - assistant in LFM's languages → LFM2.5-1.2B;
     - otherwise Qwen3.5-0.8B.
   - The reason is shown, and the user can override it.
   - Ready when the recipe, profile and goal text are set.
2. **Data** (primary action: "Check dataset", which becomes "Continue to training" once nothing blocks)
   - Sources: table editor, workspace file import, agent generation, agent cleaning, Hugging Face import (P4).
   - The checks panel lists findings with one-click fixes.
   - The split preview shows the train, validation and evaluation counts.
   - "Export JSONL" writes to the workspace.
   - Ready when no check is `block`.
3. **Training** (primary action: "Start training")
   - The preflight lists missing permissions, with guided enabling for admins.
   - The preset cards show the computed hyperparameters, estimated GPU time and quota.
   - A confirmation dialog names the Kaggle dataset and notebook, the byte count and the base model license.
   - The run monitor shows the stage timeline, a live or post-run loss chart and log highlights.
   - Done when a run is `succeeded`.
4. **Test** (primary action: "Chat with your model")
   - Report card, chat with the tuned model, side-by-side with the base model, and optionally a configured AuraGo provider as a reference.
   - Done once the report card of a succeeded run has been opened.
5. **Use** (primary action: "Export model")
   - Export the GGUF and a model card to the workspace.
   - Admins can pin the model as an AuraGo provider (P4).
   - Done after an export or a pin.

Mascot moods are local SVG built with `createElementNS`: `idle`, `thinking` (jobs running), `happy` (step ready), `worried` (block or failure), `sleeping` (runtime stopped), `celebrating` (run succeeded).

## Architecture overview

Components and boundaries:

- **Desktop app** (`ui/js/desktop/apps/chibi-sensei/`): classic-script modules. It talks only to `/api/desktop/chibi-sensei`.
- **HTTP layer** (`internal/server/chibi_sensei_*.go`):
  - gates (desktop scope and operation, feature flag, read-only), CSRF and bounded bodies;
  - adapters to Kaggle, the LLM routes, the Vault, the desktop workspace and the agent loop.
  - It is the only place that touches `*Server`.
- **Domain package** (`internal/chibisensei`): store, rows, checks, split, token estimate, imports, profiles, presets, plan, job spec, notebook generation, run state machine, log parser, artifacts, report scoring.
  - It never imports `internal/server`. Kaggle, the LLMs and the workspace reach it through interfaces.
- **Trainer** (`internal/chibisensei/trainer/chibi_train.py`, embedded): one script for Kaggle and the later local container.
- **Chat runtime** (`internal/chibisensei/chatrt`): supervises at most two llama-server instances. It uses the shared llama.cpp pins from the new `internal/llamacpp`.
- **Kaggle** (`internal/kaggle`): unchanged ownership. It gains three small extensions: a machine-shape allow-list, `ReadKernelLog` and `RemoveFile`.

```
 Desktop window (chibi-sensei/*.js) ── fetch/SSE ──> /api/desktop/chibi-sensei (internal/server)
        │                                                │
        │                  ┌─────────────────────────────┼──────────────────────────────┐
        │                  v                             v                              v
        │        internal/chibisensei           chibiLLM (one-shot)              Sensei agent loop
        │        store (data/chibi_sensei.db)    flowProviderEntry,               agent.ExecuteAgentLoop,
        │        checks · plan · runs             budget "chibi_sensei"           hooks for chibi_* tools
        │                  │
        │      KaggleDriver│(interface)               ChatRuntime (chatrt)
        │                  v                                │
        │        kaggle.Service.Execute ── Kaggle API        ├─ native llama-server (llamacpp pins)
        │        ReadKernelLog · RemoveFile                  └─ Docker sidecar when AuraGo runs in Docker
        │                  │
        │                  v
        │        Kaggle notebook: wrapper → chibi_train.py → /kaggle/working/chibi-*
        v
   data/chibi_sensei/{runs,models,base}/   desktop workspace (imports, exports)
```

## Naming and registration

These names follow existing conventions:

- Built-in IDs are hyphenated (`cyd-creator`, `game-maker-studio`, `video-studio`).
- Config sections use underscores (`cyd_creator`), and capabilities use the section name.
- Launcher keys replace hyphens with underscores (`desktop.app_cyd_creator`, `ui/lang/desktop/en.json:925`).

| Item | Value |
|---|---|
| App ID / entry | `chibi-sensei` / `builtin://chibi-sensei` |
| Go package | `internal/chibisensei` (sub-packages `trainer`, `chatrt`, `localtrain` in P5); shared `internal/llamacpp` |
| Config section | `chibi_sensei` (`ChibiSensei ChibiSenseiConfig` in `config_types.go` next to `Storymaker`, :2212) |
| Capability | `chibi_sensei` |
| API base | `/api/desktop/chibi-sensei` (same pattern as `/api/desktop/video-studio`, `server_routes.go:529-531`) |
| JS globals | `window.ChibiSenseiApp = {render, dispose, open}`; parts namespace `window.ChibiSenseiParts` |
| CSS | `ui/css/desktop-app-chibi-sensei.css`, class prefix `csn-` (no match in `ui/css`; no `cc-`, no `ad`) |
| i18n | section `chibi_sensei` (`ui/lang/chibi_sensei/<16 locales>.json`, keys `chibi_sensei.*`); launcher `desktop.app_chibi_sensei` |
| Icon | `ui/img/desktop-icons/chibi-sensei.svg` |
| SQLite | `config.ChibiSenseiDBFilename = "chibi_sensei.db"` in `internal/config/sqlite_paths.go` (pattern :13-27) |
| Agent tool prefix | `chibi_` |
| Budget category | `chibi_sensei` (categories are free-form, `budget/tracker.go:385`) |
| Docker owner / names | `dockerutil.ChibiSenseiOwner = "chibi-sensei"`, containers `aurago-chibi-llm-<10 hex>`, `aurago-chibi-serve` |
| Kaggle slugs | dataset `chibi-data-<run12>`, notebook `chibi-<project8>-<run12>` |

Registration:

- **Manifest.** Add this entry to `BuiltinApps()` after Storymaker (`internal/desktop/types.go:859`):
  `{ID: "chibi-sensei", Name: "Chibi Sensei", Version: "1.0.0", Icon: "chibi-sensei", Entry: "builtin://chibi-sensei", Runtime: BuiltinRuntime, Category: "ai", Description: "Train a small AI model on your own examples, guided step by step, and test it locally.", Permissions: []string{"files:read", "files:write", "notifications"}, Requires: []string{"chibi_sensei"}, Metadata: map[string]string{"open_maximized": "true", "logo_path": "/img/desktop-icons/chibi-sensei.svg"}}`.
- **Capability.** `Requires` is evaluated by `AppRequirementsMet` (`capabilities.go:24-31`) against `desktopCapabilities.HasCapability`, which is a switch in `internal/server/bluetooth_live.go:14-39`. Add `case "chibi_sensei": return c.s.chibiSenseiAvailable()`. That function is `cfg.ChibiSensei.Enabled && cfg.VirtualDesktop.Enabled`, the same as `storymakerAvailable` (`storymaker_service.go:407-410`). A disabled app hides; it is never deleted.
- **Routes.** `mux.HandleFunc("/api/desktop/chibi-sensei/", handleChibiSensei(s))` in `server_routes.go` next to :531. The gate helper `chibiSenseiAuthorize` copies `storymakerAuthorize` (`storymaker_handlers.go:72-105`): `requireDesktopOperation`, the feature flag (403 `chibi_disabled`), desktop enabled (503 `desktop_disabled`) and runtime availability. Optionally it resolves the desktop service for workspace I/O.
- **Boot.** `s.initChibiSensei(serverCtx)` runs right after `ensureKaggleServices` (`server.go:950`), with a deferred close. It opens the store, starts the run reconciler (only when non-terminal runs exist) and starts the chat runtime reaper.
- **Shell wiring.** Every app needs these points:
  - `ui/js/desktop/core/module-loader.js`: a `DESKTOP_APP_ASSETS` entry (styles via `appStyles()`, scripts with the composition root last) and `APP_I18N_SECTIONS['chibi-sensei'] = ['chibi_sensei']`;
  - `core/desktop-foundation.js`, `core/menus-and-routing.js`;
  - `core/window-shell-runtime.js`: category `ai`, size 1280×820, min 900×600, multi-instance;
  - `core/sdk-events-bootstrap.js`: none in P1; in P4 the `open_app` launch payload passes `project_id` only if it matches the UUID pattern;
  - then rebuild `ui/js/desktop/bundles/main.bundle.js` (`npm run build:ui`, `npm run check:ui`).

## Data model and storage

Decision: one dedicated SQLite file, `data/chibi_sensei.db`, for projects, rows, runs and metrics. Large files go under `data/chibi_sensei/`. Workspace files are used only for explicit imports and exports. Reasons:

- Paging, filtering, dedupe and bulk edits over up to 50,000 rows need indexes and transactions. Storymaker's model of one project document in the desktop workspace suits whole-document editing, not row-level work.
- `data/kaggle.db` belongs to `internal/kaggle` and pins schema version 1 (`store.go:59-75`). Adding tables there breaks ownership.
- `desktop_store.db` holds app installs.
- GGUF files up to about 1.3 GB stay out of the workspace: users edit, sync and browse it. The chat runtime mounts the model directory read-only.

The store follows `kaggle.OpenStore`: modernc sqlite, `journal_mode=WAL`, `busy_timeout=5000`, `SetMaxOpenConns(1)` and a `schema_version` table starting at 1. Tables:

```sql
settings(key TEXT PRIMARY KEY, value TEXT NOT NULL)            -- kaggle_project_id, serve_pin
projects(id TEXT PRIMARY KEY, name TEXT, recipe TEXT, task_kind TEXT, labels_json TEXT,
  goal_text TEXT, profile_id TEXT, preset TEXT, overrides_json TEXT, split_seed INTEGER,
  data_rev INTEGER NOT NULL DEFAULT 0, rev INTEGER NOT NULL DEFAULT 1, created TEXT, updated TEXT, archived INTEGER DEFAULT 0)
rows(id INTEGER PRIMARY KEY, project_id TEXT NOT NULL, seq INTEGER NOT NULL, kind TEXT NOT NULL, -- chat|text
  messages_json TEXT, text TEXT, label TEXT, split TEXT NOT NULL DEFAULT 'auto', -- auto|train|eval|exclude
  source TEXT NOT NULL, source_ref TEXT, batch_id TEXT, tags TEXT, content_hash TEXT NOT NULL,
  prompt_hash TEXT, est_tokens INTEGER NOT NULL, flags INTEGER NOT NULL DEFAULT 0, rev INTEGER NOT NULL DEFAULT 1,
  created TEXT, updated TEXT)
  -- indexes: (project_id, seq), (project_id, content_hash), (project_id, prompt_hash), (project_id, split), (project_id, batch_id)
row_history(id INTEGER PRIMARY KEY, project_id TEXT, row_id INTEGER, batch_id TEXT, before_json TEXT, created TEXT)
documents(id TEXT PRIMARY KEY, project_id TEXT, name TEXT, workspace_path TEXT, sha256 TEXT, text TEXT, chars INTEGER, created TEXT)
checks(project_id TEXT PRIMARY KEY, data_rev INTEGER, result_json TEXT, computed_at TEXT)
jobs(id TEXT PRIMARY KEY, project_id TEXT, kind TEXT, state TEXT, params_json TEXT, progress_json TEXT,
  result_json TEXT, error_code TEXT, error TEXT, created TEXT, updated TEXT) -- generate|clean|import_file|import_hf|judge|export
consents(profile_id TEXT, license_id TEXT, license_version TEXT, accepted_at TEXT, PRIMARY KEY(profile_id, license_id, license_version))
runs(id TEXT PRIMARY KEY, project_id TEXT, backend TEXT, state TEXT, step_attempt INTEGER, profile_id TEXT,
  profile_sha TEXT, preset TEXT, hyper_json TEXT, job_json TEXT, data_sha TEXT, remote_json TEXT,
  deadline TEXT, cancel_requested INTEGER DEFAULT 0, error_code TEXT, error TEXT, leftovers_json TEXT, created TEXT, updated TEXT)
run_events(id INTEGER PRIMARY KEY, run_id TEXT, ts TEXT, kind TEXT, data_json TEXT)  -- at most 5,000 per run
metrics(run_id TEXT, step INTEGER, kind TEXT, epoch REAL, loss REAL, lr REAL, grad_norm REAL, tok_s REAL, ts TEXT,
  PRIMARY KEY(run_id, step, kind))                                                     -- kind: train|eval
artifacts(id TEXT PRIMARY KEY, run_id TEXT, project_id TEXT, kind TEXT, rel_path TEXT, bytes INTEGER, sha256 TEXT, created TEXT)
eval_reports(run_id TEXT, kind TEXT, result_json TEXT, created TEXT, PRIMARY KEY(run_id, kind))  -- auto|judge
base_gguf(profile_id TEXT, base_revision TEXT, converter TEXT, rel_path TEXT, sha256 TEXT, PRIMARY KEY(profile_id, base_revision, converter))
```

Additional rules:

- **Revisions.** Every row mutation increments `projects.data_rev` in the same transaction. Checks and plans name the `data_rev` they were computed for.
- **Hashes.** `content_hash` is the sha256 of the normalized JSON: roles plus contents, NFC, trimmed, whitespace runs collapsed, lower-cased. `prompt_hash` covers everything before the last assistant message.
- **Files.**
  - `data/chibi_sensei/runs/<run_id>/data/{train,val,eval}.jsonl`: the frozen snapshot of a run.
  - `data/chibi_sensei/runs/<run_id>/out/`: status, metrics, eval samples, manifest, log tail.
  - `data/chibi_sensei/models/<run_id>/model-q8_0.gguf` and `adapter/`.
  - `data/chibi_sensei/base/<profile>-<rev12>-<converter>.gguf`.
- **Storage limit.** The total is capped by `chibi_sensei.storage.max_gb`. A run refuses to start when free disk space or the cap cannot hold the profile's estimated artifact size times 1.2.

## Model profiles

Profiles are embedded JSON files, `internal/chibisensei/profiles/<id>.json`, one per model, loaded and validated at startup. `ProfileSHA` is the sha256 of the canonical JSON and is recorded in each run. Model-specific behaviour lives only here and in the trainer branches that read these fields.

```go
type Profile struct {
    ID, DisplayName string; Schema int; Status string // experimental|verified
    Verified *struct{ Date, AuraGoVersion, KaggleRef, LlamaCpp string } `json:"verified"`
    Unverified []string        `json:"unverified"` // JSON paths of values not yet proven; empty when verified
    License struct{ ID, Name, URL, Version string; RequiresConsent bool; NoteKey string }
    Base struct{ Source, Repo, Revision, AutoClass string; Gated bool }  // Source: hf
    Arch struct{ LlamaCppArch string; ContextMax int; ParamsB float64; TextOnly bool; Languages []string }
    Chat struct{ SystemRole string; Stop []string }                     // SystemRole: native|merge_into_first_user
    Training struct{
        MethodDefault string; Methods []string; DType string            // float32 only at launch
        LoRA struct{ R, Alpha int; Dropout float64; TargetModules []string }
        LR map[string]float64; MaxSeqCap, BatchTokens int; GradCheckpointing bool
        T4TokensPerSecond, T4GenTokensPerSecond float64; MinGPUCapability string; MinVRAMGB float64
        ExtraPip []string
    }
    Sampling struct{ Temperature, TopP, RepeatPenalty, PresencePenalty float64; TopK int }
    Eval struct{ MaxNewTokensAssistant, MaxNewTokensSpecialist int }
    GGUF struct{ OutType, ExpectedArch string; EstimatedMB int }
}
```

Validation at load:

- every method in `Methods` has an `LR`;
- `DType == "float32"`;
- `MaxSeqCap` is 256 to 4096;
- `Status == "verified"` requires an empty `Unverified` and a non-nil `Verified`;
- `Base.Gated` must be false. A gated base would need a token on Kaggle, and the design avoids that.

Launch profiles. Every value is UNVERIFIED unless the brief states it as a vendor fact. All three ship as `experimental`.

Proof runs on 2026-11-25 replaced these assumptions with measured values and flipped all three profiles to `verified`; the files under `internal/chibisensei/profiles/` are authoritative, and the execution record of the P2 plan lists the differences to the table below.

| Field | `gemma3-270m` | `qwen35-0p8b` | `lfm25-1p2b` |
|---|---|---|---|
| Base repo | `unsloth/gemma-3-270m-it` (ungated mirror, UNVERIFIED that it stays ungated) | `Qwen/Qwen3.5-0.8B` | `LiquidAI/LFM2.5-1.2B-Instruct` |
| Revision | pinned commit sha, recorded at proof run | same | same |
| AutoClass | `AutoModelForCausalLM` | `AutoModelForCausalLM`, text-only load UNVERIFIED (vision tower) | `AutoModelForCausalLM` |
| License | Gemma Terms of Use, consent required | Apache 2.0, consent recorded | LFM Open License v1.0 (free under $10M revenue), consent required |
| llama.cpp arch | `gemma3` | `qwen35` UNVERIFIED name | `lfm2` |
| Context max | 32768 | 262144 | 32768 |
| System role | `merge_into_first_user` | `native` (thinking off) | `native` |
| Method default | `full` (`lora` also allowed, experimental) | `lora` | `lora` |
| LoRA r/alpha/dropout | 16/32/0.05 | 16/16/0.05 (vendor) | 16/32/0.05 (vendor) |
| Target modules | q,k,v,o,gate,up,down_proj | same, DeltaNet names UNVERIFIED | same, conv-block names UNVERIFIED |
| LR | full 5e-5, lora 2e-4 | lora 2e-4 | lora 2e-4 |
| MaxSeqCap | 2048 | 2048 | 2048 |
| T4 train tok/s | 6000 UNVERIFIED | 2500 UNVERIFIED | 1800 UNVERIFIED |
| Sampling | temp 1.0, top_k 64, top_p 0.95 | temp 0.7, top_p 0.8, top_k 20, presence 1.5 | temp 0.1, top_k 50, repeat 1.05 |
| GGUF Q8_0 est. | 300 MB | 900 MB | 1250 MB |
| Min GPU | capability 7.0, 12 GB | 7.0, 14 GB | 7.0, 15 GB |
| Extra pip | none | `flash-linear-attention` pin UNVERIFIED (Triton on T4) | none |

**270M method.** Full fine-tuning is the default. Google publishes a full fine-tune guide for this model next to its QLoRA guide; the guide was not re-read on 2026-11-22, so the hyperparameters here are our own and UNVERIFIED. In fp32 the model, gradients and AdamW state take about 4.3 GB plus activations, which fits a 16 GB T4. A full fine-tune also skips the merge step, removing one point of failure. The trainer reads `method` from `job.json` in one place: the `full` path skips adapter save and merge.

**Presets.** Pure Go in `internal/chibisensei/presets.go`. Inputs are N_train, the p95 token estimate P, the profile and the recipe.

- `max_seq_len` = the smallest bucket in {256, 512, 1024, 2048, 4096} that is ≥ 1.1·P, capped at `MaxSeqCap`. Rows over it are truncated, and a check reports them.
- Base epochs: N < 300 → 3; N < 3000 → 2; otherwise 1.
  - quick = 1 epoch, LoRA r/2 (alpha r), eval samples 30, training budget at most 30 min;
  - balanced = base epochs, profile LoRA, eval samples 100;
  - thorough = min(base+1, 4) epochs, LoRA 2r/2·alpha, eval samples 200.
- LR = `profile.LR[method]` for every preset. Fixed for all presets: cosine scheduler, warmup ratio 0.03, weight decay 0, seed 42.
- `per_device_batch` = max(1, floor(BatchTokens / max_seq_len)), capped at 8. `grad_accum` = ceil(16 / per_device_batch), but the effective batch is at most N_train/10.
- Loss on assistant tokens for `assistant` and `specialist`, on all tokens for `free`.
- Estimate in minutes = 8 (environment and install) + 3 (base download and load) + training tokens·epochs / `T4TokensPerSecond` / 60 + eval samples·2·max_new / `T4GenTokensPerSecond` / 60 + 6 (merge and GGUF) + 3 if a base GGUF is needed.
- Session timeout = clamp(1.5·estimate + 15, 30, `kaggle.remote_timeout_minutes`) minutes.
- `max_train_s` = session timeout − (non-training estimate·1.5).

## Training contract

**Transport.** The training data is uploaded as files of a private Kaggle dataset. The script and `job.json` are written into the notebook as base64 strings. Reasons:

- A Kaggle request is capped at 20 MiB (`catalog.go:99-101`, `client.go:71-73`), which a 50,000-row dataset can exceed.
- Code stays versioned with AuraGo, and no secret is ever in either place.

Dataset files:

- `train.jsonl`, `val.jsonl`, `eval.jsonl`;
- the marker `chibi-data-<run_id>.json`: `{"v":1,"run_id","files":{"train":{"name","sha256","rows"},...}}`.

The script finds the marker by walking `/kaggle/input` to depth 4, so the mount layout does not matter.

**Notebook** (`internal/chibisensei/notebook.go`, golden-file tested). nbformat 4 with two cells:

1. A markdown cell: "Generated by AuraGo Chibi Sensei for run <run12>. Do not edit."
2. One Python cell that:
   - decodes `JOB_B64` and `SCRIPT_B64` into `/tmp/chibi/`;
   - prints the job JSON and the script's sha256;
   - runs `subprocess.call([sys.executable, "-u", script, "--job", job, "--out", "/kaggle/working"])`;
   - catches every exception, prints `@@chibi {"v":1,"t":"exit","code":rc}` and never raises.

The session therefore ends `COMPLETE` and its outputs stay downloadable. Success is decided from `chibi-status.json`, not from the Kaggle state.

**`job.json`** (schema v1, built by `jobspec.go`):

```json
{"v":1,"run_id":"<uuid>","created":"<RFC3339>","aurago_version":"<ver>",
 "profile":{"…full profile snapshot…"},"profile_sha":"<hex>",
 "recipe":"assistant|specialist|free","task_kind":"classify|extract|rewrite|to_json|null","labels":["…"],
 "method":"lora|full","data":{"marker":"chibi-data-<run_id>.json","format":"messages|text"},
 "hyper":{"epochs":2,"learning_rate":0.0002,"lr_scheduler":"cosine","warmup_ratio":0.03,"weight_decay":0,
   "per_device_batch":4,"grad_accum":4,"max_seq_len":1024,"seed":42,"gradient_checkpointing":true,"loss_on":"assistant",
   "lora":{"r":16,"alpha":32,"dropout":0.05,"target_modules":["q_proj","…"]}},
 "budget":{"session_timeout_s":5400,"max_train_s":3600,"eval_max_samples":100,"max_new_tokens":256},
 "outputs":{"emit_base_gguf":true,"gguf_outtype":"q8_0","prefix":"chibi-"},
 "deps":{"pip":["peft==<pin>","accelerate==<pin>","sentencepiece==<pin>"],"transformers":"<pin>",
   "llama_cpp":{"tag":"b9994","url":"https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/b9994","sha256":"<pin>"}},
 "log":{"prefix":"@@chibi","metric_every_steps":10}}
```

Pins live in the embedded `trainer/requirements.lock` and are recorded at the proof run. `transformers` is reinstalled only when the preinstalled version differs from the pin; torch is never reinstalled. The llama.cpp tag matches the local runtime build `b9994` (`embeddings/manifest.go:13`), so the converter and the runtime agree.

**Stages** (`chibi_train.py`, stdlib at module top, heavy imports inside stages):

1. `boot`: parse arguments, validate `job.json`.
2. `env_check` (fail fast, before any install):
   - Python ≥ 3.10;
   - CUDA available;
   - GPU capability ≥ `MinGPUCapability` (error code `env_unsupported_gpu`, naming the GPU, for example a P100);
   - VRAM ≥ `MinVRAMGB`;
   - free disk ≥ 3× the model size;
   - pypi.org and huggingface.co reachable within 10 s (`env_no_internet`, with a hint about phone verification).
3. `install`: pip install the pins; import checks; record versions.
4. `data`: find the marker, verify sha256 values (`data_mismatch`), parse and validate rows.
5. `load_base`: download the base at the pinned revision, anonymously; load tokenizer and model in fp32 on `cuda:0`. Use one GPU: set `CUDA_VISIBLE_DEVICES=0`, even on T4×2.
6. `base_eval`: base eval loss; greedy base answers for the eval samples.
7. `train`: `transformers.Trainer` plus `peft` for LoRA, with a callback emitting metrics. It stops cleanly at `max_train_s` (`stop_reason: time_budget`) or on a NaN loss (`train_diverged`). CUDA out-of-memory maps to `oom`.
8. `tuned_eval`: tuned eval loss and greedy answers.
9. `save`: adapter to `chibi-adapter/` (LoRA only); merge (LoRA) or take the trained weights (full); save to `/tmp/chibi/merged`.
10. `gguf`:
    - fetch and verify the llama.cpp tarball;
    - `pip install ./gguf-py`;
    - `convert_hf_to_gguf.py --outtype q8_0` to `chibi-model-q8_0.gguf`, and the base to `chibi-base-q8_0.gguf` when `emit_base_gguf`;
    - verify with `gguf.GGUFReader` that the architecture equals `ExpectedArch` and tensors exist (`gguf_arch_mismatch`).
11. `manifest`: sha256 of every output into `chibi-manifest.json`; final `chibi-status.json`.

**Structured log line.** One line of at most 4 KiB: `@@chibi {"v":1,"seq":N,"ts":"…","t":"stage|metric|eval|warn|error|done|exit",…}`. Metric fields: `step, max_steps, epoch, loss, eval_loss, lr, grad_norm, tok_s, elapsed_s, eta_s, vram_gb`. Stage fields: `stage, state` (`start|done|failed`). Error fields: `stage, code, message`.

**Output files** (flat in `/kaggle/working`, prefix `chibi-`):

- `chibi-metrics.jsonl`: the metric objects without the prefix.
- `chibi-status.json`: `{"v":1,"run_id","state":"succeeded|failed","stage","code","message","started","finished","gpu":{"name","capability","vram_gb"},"versions":{"python","torch","transformers","peft","llama_cpp"},"train":{"steps","epochs","train_loss","eval_loss_base","eval_loss_tuned","stop_reason"}}`.
- `chibi-eval_samples.jsonl`: `{"id","prompt_messages":[…]|"prompt_text","expected","base_output","tuned_output","base_ms","tuned_ms","truncated"}`. For the free recipe, the prompt is the first half of an eval text.
- `chibi-adapter/` (LoRA), `chibi-model-q8_0.gguf`, optional `chibi-base-q8_0.gguf`.
- `chibi-manifest.json`: `{"v":1,"run_id","profile_id","profile_sha","files":[{"path","kind","bytes","sha256"}]}`.

**Selftest.** `python chibi_train.py --selftest` needs no GPU and no network. It checks the `job.json` schema, the marker and sha256 logic, the log-line encoder, the assistant-token masking (with a character-level stub tokenizer) and the status and manifest writers.

`--dry-run --job … --out …` with `CHIBI_FAKE_ENV=1` runs every stage with stubs and writes real-shaped outputs. Go then parses those outputs with the production parser.

## Kaggle orchestration

**In-process access.** `internal/server/chibi_sensei_kaggle.go` implements `chibisensei.KaggleDriver` over the `*kaggle.Service` from `s.ensureKaggleServices()` (`kaggle_handlers.go:39-67`). It always passes `s.kaggleSnapshot().Access` (`:26-38`).

```go
type KaggleDriver interface {
    Account(ctx context.Context) (string, error)                       // Service.Test → "account"
    Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error)
    Job(ctx context.Context, id string) (kaggle.Job, error)            // Store.Get
    ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error)       // Store.List, filtered like checkComputeSlots
    EnsureProject(ctx context.Context) (string, error)                 // one "Chibi Sensei" project, id in settings
    WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error)
    OpenFile(project, name string) (*os.File, error)
    RemoveFile(project, name string) error
    ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error)
    Cancel(ctx context.Context, jobID string) error
}
```

`Execute` is asynchronous: it admits a durable job and returns at once (`service.go:119-240`). Chibi supplies a deterministic request ID: the first 32 hex characters of `sha256(run_id+":"+step+":"+attempt)`. A replay with the same parameters returns the original receipt instead of mutating again (`store.go:110-125`, `service.go:209-211`), which makes every step resumable.

**Staging.** Uploads read a file from a Kaggle project and need the Vault (`transfers.go:320-330`). Downloads must name a project and create a new file (`:274-318`, expected `"*"` at :312). Downloaded bytes count against `kaggle.max_storage_mb` (`:225-244`).

Chibi therefore keeps one Kaggle project titled "Chibi Sensei" and names files `r<run12>-<name>`. Each file is copied to Chibi storage, its sha256 is checked against the manifest, and it is then removed.

**Kaggle extensions** (smallest safe changes, each with tests in `internal/kaggle`):

1. **Machine-shape allow-list.**
   - Add `AllowedMachineShapes []string `yaml:"allowed_machine_shapes" json:"allowed_machine_shapes"`` to `KaggleConfig` (`config/kaggle.go:6-30`). The default is empty.
   - `NormalizeKaggleConfig` trims and dedupes the list and keeps only known names: `NvidiaTeslaT4` and `NvidiaL4`.
   - `intersectConfig` (`policy.go:18-42`) intersects the list, and `WithConfigSnapshot` (:14-17) copies it.
   - `CheckGrants` (:154-156): `dockerImage` stays refused. A non-empty `machineShape` must be in the list, with `enableGpu == true` and `AllowGPU`.
   - The Kaggle config form (`ui/cfg/kaggle.js`, catalog `ui/js/config/catalog.js`) gets a checkbox list.
2. **`ReadKernelLog`.** A new `internal/kaggle/logs.go`: `func (s *Service) ReadKernelLog(ctx, access Access, owner, slug string, version int, maxBytes int64) ([]byte, error)`.
   - It applies `CheckGrants` for `get_kernel_session_logs_stream` (grant `download`, `catalog.go:57`, so it needs `AllowDownload` and write access).
   - It requires `owner` to be the authenticated account.
   - It calls the operation with `{userName, kernelSlug, versionLabel, waitForLogsUrlSeconds: 10}` (`contracts.json:2221-2241`) and follows a storage redirect through `storageFactory`.
   - It reads at most 16 MiB and returns the last `maxBytes`. There is no durable receipt and no project file: a read is not a mutation, and one receipt per poll would flood the Kaggle job list.
3. **`RemoveFile`** in `transfers.go`: `validFile`, `projectRoot`, `Lstat` must be a regular file, remove under `filesMu`.

**Run state machine** (`internal/chibisensei/runs.go`). The state persists in `runs.state` and `remote_json`; every transition is a store transaction plus a `run_events` row and an SSE event.

| State | Action | Next | Recovery after restart |
|---|---|---|---|
| `queued` | freeze the snapshot files and `job.json`; compute `data_sha` | `packaging` | redo (local, idempotent) |
| `packaging` | `EnsureProject`; write data files and marker as `r<run12>-*` | `uploading` | rewrite files that are missing |
| `uploading` | `upload_dataset_file` per file (deterministic IDs), wait for completion | `creating_dataset` | completed → next; failed → `failed`; `unknown` → new attempt (upload receipts are never replayed, so this means a new upload and new IDs) |
| `creating_dataset` | `create_dataset {ownerSlug, slug, title, isPrivate:true, licenseName:"unknown", files:[{token:"upload:<id>"}…]}`; remove staged files | `waiting_dataset` | `unknown` → `get_dataset` read: exists → next; 404 → new attempt |
| `waiting_dataset` | poll `get_dataset_status` every 15 s, up to 15 min | `starting` | resume polling |
| `starting` | `save_kernel {slug, newTitle, text, kernelType:"notebook", language:"python", kernelExecutionType:"SAVE_AND_RUN_ALL", isPrivate:true, enableGpu:true, enableInternet:true, machineShape, datasetDataSources:[…], sessionTimeoutSeconds}` | `running` once the Kaggle job is `remote_running` | `unknown` → `get_kernel_session_status` read of the slug: running or complete → `running` with `remote_json.untracked=true`; 404 → `failed` (`kaggle_submit_unknown`, nothing replayed) |
| `running` | watch the Kaggle job (`pollRemote` every 10 s, `remote_jobs.go:12-141`), or poll status itself when untracked; `ReadKernelLog` every 60 s | `downloading` on `completed`/`failed`/`cancelled` | resume (`remote_running` survives a restart, `service.go:80-84`) |
| `downloading` | download `chibi-status.json` and `chibi-manifest.json`, then every manifest file; verify sha256; copy to Chibi storage; `RemoveFile` | `verifying`, or `failed` when the status says failed | redo missing files (attempt+1, at most 3) |
| `verifying` | GGUF header check in Go; smoke generation through chatrt when available (P3) | `cleaning` | redo |
| `cleaning` | when `delete_remote_after_run` and `AllowDelete`: `delete_dataset`, then `delete_kernel`; otherwise record leftovers | `succeeded` | redo; failures are recorded as leftovers |

Terminal states: `succeeded`, `failed`, `cancelled`.

**Timeouts.**
- The run deadline is the remote start plus `sessionTimeoutSeconds` plus 20 min. `pollRemote` enforces timeouts only for benchmarks (`remote_jobs.go:41-49`), so Chibi enforces its own deadline.
- Past the deadline, one final status read decides the outcome; if the session is still running, the run fails with `remote_timeout` and a link to Kaggle.

**Quota and slots.**
- Before planning and again at start, Chibi reads `get_accelerator_quota_statistics` (a read operation). It blocks when the parsed remaining hours are below 1.2× the estimate, and warns when the response cannot be parsed.
- Chibi refuses to start while any compute job is active, `unknown` included. `checkComputeSlots` counts `unknown` (`service.go:420`), and `MaxRemoteJobs` defaults to 1.
- For `unknown` jobs the preflight names the admin action: "release in the Kaggle app" (`job_resolution.go:13-36`, never an agent tool).

**Cancel semantics** (stated plainly in the UI):
- Before `starting`: the run stops locally and the upload and dataset are cleaned up.
- After submission: Chibi calls `Service.Cancel`. SaveKernel returns `ref`, `versionNumber` and `kernelId` but no `kernelSessionId` (`remote_lifecycle_test.go:82`), and the Kaggle contract forbids inferring one (`internal/kaggle/AGENTS.md:19`, `remote_jobs.go:213-215`). Remote cancel therefore normally fails.
- When it fails, the run shows "Stop it on Kaggle" with a link to `https://www.kaggle.com/code/<owner>/<slug>`, keeps tracking until a terminal state and then ends `cancelled`.
- The promise in the UI (corrected in P2 to match the rule above): "AuraGo will not download a model from this run. If the notebook is already running, GPU time ends when you stop it on Kaggle, and at the latest at the run's time limit (<n> min). AuraGo keeps watching the notebook until it has ended and then tidies up; a new run can start after that."
- A stop that arrives while the run is already tidying up (`cleaning`) changes nothing: the run's end state is decided.

**Kaggle app job list.** Chibi's operations show up as ordinary job rows (`create_dataset`, `upload_dataset_file`, `save_kernel`, `download_kernel_output`, `delete_*`), and the `chibi-` slugs make them recognizable. The Kaggle app is not changed. Chibi's run view lists the Kaggle job IDs of each step.

## Live progress

The code allows live progress, with one gap. `get_kernel_session_logs_stream` accepts `userName/kernelSlug/versionLabel` and a `waitForLogsUrlSeconds` (`contracts.json:2221-2241`). Through `Execute`, though, every call would create a durable job and a project file (`service.go:276-277`, `transfers.go:312`). `ReadKernelLog` is the non-durable path.

Whether Kaggle serves the log while the session runs is UNVERIFIED.

Design:
- While `running`, the reconciler calls `ReadKernelLog(max 16 MiB)` every 60 s (P2 raised the cap from 4 MiB: Kaggle wraps every output line in a JSON entry, and the trainer's lines are size-capped).
- `logparse.go` handles two inputs. It first tries a JSON array of `{stream_name, time, data}` entries and joins their `data`; otherwise it treats the body as plain text. It then extracts `@@chibi ` lines and dedupes them by `seq`.
- Metrics are upserted into `metrics`; stages and warnings go to `run_events`.
- SSE pushes `run`, `metric` and `stage` events to the window. The UI also polls `GET runs/{id}` every 2.5 s as a safety net, as Storymaker does.

Fallback:
- When the log is empty or refused during a run, the monitor shows "Running on Kaggle", the elapsed time against the estimate, the remote status and the note "Live numbers are not available for this run; the chart appears when the run ends".
- After the run, `chibi-metrics.jsonl` replaces the live metrics, and the last 64 KiB of the log tail is stored as an artifact.

## Dataset studio

**Row schema in the API:**

```json
{"id":123,"seq":5,"kind":"chat","messages":[{"role":"system|user|assistant","content":"…"}],"text":null,
 "label":"positive","split":"auto|train|eval|exclude","source":"manual|file|generated|cleaned|hf|sensei",
 "source_ref":"Imports/data.csv#L12","tags":["…"],"est_tokens":180,"flags":["duplicate","too_long"],"rev":3}
```

Structure rules:
- Optional single leading `system`, then alternating `user`/`assistant`, ending with `assistant`.
- `label` is derived from the last assistant content for `classify`.
- `text` rows are allowed only in the free recipe.

Limits:
- 50,000 rows per project (`chibi_sensei.dataset.max_rows`);
- 32,000 characters per message;
- 64 messages per row.

**Table editor.**
- Server-paged with `offset` and `limit` (50–200). Filters: `q` (LIKE over content), `split`, `source`, `flag`, `label`, `batch`; sort by `seq` or `est_tokens`.
- Inline editing saves with `If-Match: <row rev>` and answers 412 on conflict.
- Bulk actions on selected IDs or on the current filter: set split, add or remove a tag, exclude, delete.
- Deleting more than 50 rows or deleting by filter needs `POST rows/delete-preview` first. It returns `{count, token}` (valid 5 min), and `POST rows/delete {token}` executes it.
- Every bulk change writes `row_history` under a `batch_id`, so "Undo" restores it.

**Imports from the workspace.** Files are read through `desktop.Service.ReadFileBytes` (`service_files.go:307`), which keeps workspace path safety and the size limit, capped at `chibi_sensei.import.max_file_mb`.
- JSONL: `messages`, `conversations` (`from/value`), `instruction/input/output`, `prompt/completion`, `text`, `text+label`.
- CSV: column mapping with a preview of 5 mapped rows.
- TXT and Markdown are stored as source documents for generation, never as rows; the free recipe can opt in to "split into text rows" by paragraphs of at most 2,000 characters.
- Imports run as `import_file` jobs with progress. Invalid rows are reported with line numbers and skipped.

**Exports.** JSONL of the chosen split, or of all rows, in canonical form. Written with `WriteFileStreamConditional` (`service_file_stream.go:19`) to a workspace path the user picks.

**Split.** A stable split from `sha256(split_seed‖row_id)`. Pinned splits are honored.
- eval = clamp(round(0.05·N), 10, 200) when N ≥ 100; otherwise max(5, round(0.1·N)).
- val = 0 when N < 200; otherwise clamp(round(0.05·N), 10, 500).
- `classify` stratifies by label.

**Token estimate.** `est_tokens` = ceil(UTF-8 bytes / 3.6) for non-CJK text, plus 1 per CJK rune, plus 4 per message. Labelled "estimate" everywhere.

**Checks** (`checks.go`, run on demand; a result is stale when `data_rev` changes):

| ID | Rule | Level |
|---|---|---|
| `count_min` | rows < minimum (assistant 30, specialist 30, free 20 rows and 20k tokens) / below recommended (200 / 100 / 200) | block / warn |
| `structure` | invalid role order, empty assistant, `text` row outside free | block (row-level) |
| `short_answer` | assistant < 2 characters (specialist) or < 10 (assistant) | warn |
| `duplicates` | equal `content_hash` | warn, fix "remove duplicates (keep first)" |
| `conflicts` | equal `prompt_hash` with different answers (specialist) | warn |
| `too_long` | `est_tokens` > planned `max_seq_len` | warn, with count and share (≥ 20% says "strongly consider shortening") |
| `label_balance` | classify: single label → block; a label with < 10 rows or a max/min ratio > 5 → warn | block / warn |
| `json_valid` | to_json: the assistant answer must parse as JSON | block (row-level) |
| `split_sizes` | eval < 10 → warn; train below minimum → block | warn / block |
| `language_fit` | detected script outside `profile.Arch.Languages` (cheap script heuristic) | info |

Check result schema:

```json
{"project_id":"…","data_rev":42,"computed_at":"…","summary":{"rows":1200,"train":1080,"val":60,"eval":60,
 "est_tokens":{"p50":140,"p95":610,"max":2400}},
 "checks":[{"id":"duplicates","level":"warn","value":14,"threshold":0,"message_key":"chibi_sensei.check.duplicates",
   "args":{"count":14},"row_ids":[12,40],"row_count":14,"fix":"dedupe"}],
 "ready":true}
```

`row_ids` lists at most 50 IDs; the filter `flag=<id>` shows them all.

**Agent generation and cleaning** (one-shot jobs, P1):
- **Generation.**
  - Input: the goal text, up to 10 seed rows, and optional documents chunked to at most 6,000 characters.
  - Each call asks for 10 rows through a JSON schema `{rows:[{messages:[…]}]}` (or `{text}`). Rows are validated and deduped against existing hashes, then inserted with `source=generated` and a `batch_id`.
  - Progress events come per batch. Cancel stops after the current call.
- **Cleaning.**
  - Covers at most 500 selected rows in batches of 10. The model answers `{id, action: keep|fix|drop, messages?, reason}`.
  - The UI shows a review diff. Nothing is applied until the user accepts all or some of it. Drops are never automatic.
- **Limits.** `generator.max_rows_per_job` (500) and `generator.max_daily_rows` (5,000).

## Hugging Face import

- **Client.** Chibi calls `internal/huggingface.Client` directly (`client.go:158-202`) with:
  - `HTTPClient: security.NewSSRFProtectedHTTPClient(timeout)` (`ssrf.go:218`), because the default client is a plain `http.Client` (`client.go:184-187`); `NewClient` keeps `SameOriginRedirect` on the injected client;
  - base URLs from `HuggingFaceConfig`;
  - `MaxResultBytes: 8 MiB`, because rows of long conversations exceed the 512 KiB default (`:177-179`, `:842-849`);
  - `Token` read from the Vault (`huggingface_token`) only for gated datasets, used only in this process and never sent to Kaggle.
- **No Parquet.** There is no Parquet reader in `go.mod`, so imports use the Dataset Viewer `/rows` endpoint.
- **Policy.** Chibi requires `huggingface.enabled`. It runs `tools.EvaluateHuggingFacePolicy` for each named read (`huggingface.go:55-150`), which enforces `allowed_repos` and namespaces when configured.
- **Caps.** Page size = min(100, `huggingface.max_dataset_rows`). That config key bounds a single request (`huggingface.go:77-84`, `client.go:795-811`). The total import is capped separately by `chibi_sensei.import.max_rows` (default 20,000, range 100–50,000) and by the project's free row budget.
- **Pacing.** One request per 700 ms. A 429 honors `Retry-After`, and five consecutive failures abort with `hf_rate_limited`. The partial import is kept and labelled.
- **Curated list.** `internal/chibisensei/hfcatalog.json` (embedded): `{repo, config, split, recipe, task_kind, mapping, languages, rows_hint, description_key}`. Initial candidates are checked live at implementation time: `HuggingFaceH4/no_robots`, `databricks/databricks-dolly-15k`, `fancyzhx/ag_news`, `dair-ai/emotion`. Search uses `SearchDatasets`.
- **Flow.**
  1. `GetDataset` returns the license (`cardData.license`, or "unknown — check the card") and the gated flag. The license is shown before import and stored in each row's `source_ref`.
  2. `DatasetSplits`, then the first rows with features.
  3. Automatic mapping, the same detectors as the file import, plus ClassLabel names for `text+label`. The mapping is editable, with a preview of 5 rows.
  4. An `import_hf` job with progress.

## Agent help

**One-shot client** (`internal/server/chibi_sensei_llm.go`). `flowLLM.Step` is tied to flows types, the budget category `flows` and `FLOW_*` codes (`flows_llm.go:44`, `:98-145`), so Chibi has a sibling `chibiLLM.Complete(ctx, route, system, user, schema)` that reuses:

- `flowProviderEntry` (:495-522) and `flowCredentialScope` (:415-431) for routes;
- `flowAIResponseFormat` (:241-251) for strict JSON schema with the `json_object` retry (`flowAIFormatRejected`, :286-309);
- `budget.CompleteChat` with category `chibi_sensei`;
- `security.GateScanComposedPrompt` (`composed_prompt.go:27`) on the composed prompt.

Dataset and document text is wrapped with `security.IsolateExternalData` (`guardian.go:750`) under a guard instruction stating that wrapped text is material, never instructions.

**Model slots:**
- `chibi_sensei.generator.provider`/`.model`: generation and cleaning. Empty means the main route.
- `chibi_sensei.agent.provider`/`.model`: the Sensei panel. Empty means the main route, as `storymakerAgentResolveRoute` does (`storymaker_agent.go:44-84`).
- The judge uses `llm.ResolveHelperBackedClient(cfg, generatorClient, generatorModel)` (`helper_resolver.go:55-64`): the helper model when configured, otherwise the generator route. The report card names the judge model.
- Both provider keys are added to `provider_references.go:74-75` (roles `chibi_sensei_generator`, `chibi_sensei_agent`) and to the list in `decisions.go:158-159`.

**Sensei panel** (P4). It copies the Storymaker template:
- `runConfig` (`storymaker_agent.go:256-290`): `buildDesktopRunConfigForSession(…, "chibi_sensei")`, an ephemeral history, no long-term memory or knowledge graph, `AllowedTools` taken from the profile, `IsMission`, `SuppressTurnSideEffects`.
- `hooks` (:238-253): routes `chibi_` calls to the app.
- `runLoop` (:318-367): `ExecuteAgentLoop` plus a broker.

Additional rules:
- The user message carries the project packet in `<external_data>` (:390).
- The profile `chibi_sensei/v1/panel` is built with `agent.NewPreparedPromptProfile`. Its schemas come from a new `internal/agent/native_tools_chibi_sensei.go`, normalized like `StorymakerPhaseToolSchemas` (`native_tools_storymaker.go:23-46`).
- Required change: the private-tool route at `internal/agent/action_policy.go:336` lists only the `game_maker_`, `cyd_creator_` and `storymaker_` prefixes. `chibi_` must be added, or an enabled action policy refuses Sensei's tools.
- Arguments follow Storymaker's `operation` plus `data` object, so no argument shares a name with a typed `agent.ToolCall` field.

Tools:

| Tool | Operations | Limits |
|---|---|---|
| `chibi_project` | `get` (goal, recipe, profile, readiness), `set_goal_text` | — |
| `chibi_rows` | `list` (filter, page of ≤ 20, each text ≤ 600 characters, wrapped as external data), `get`, `add` (≤ 20 per call), `update` (≤ 20 per call) | ≤ 100 written rows per message; writes carry `source=sensei` and a per-message `batch_id` with an "Undo this answer" action |
| `chibi_checks` | `run`, `get` | — |
| `chibi_plan` | `estimate` (preset → hyperparameters, minutes) | read only |
| `chibi_propose` | `generate_rows`, `clean_rows`, `delete_rows`, `start_run` | creates a proposal card only |
| `chibi_report` | `get` (report card summary) | read only |

Run limits: `ToolCallLimit` 12 and `IterationLimit` 16 per message; timeout `agent.job_timeout_seconds` (default 300); `agent.max_daily_turns` (100).

**Confirmations.** The agent never starts costly or destructive work. A proposal (kept in memory per project, valid 30 min) shows the exact effect: row count, model, estimated tokens or GPU minutes. "Accept" calls the normal endpoint, and a GPU run still opens the same confirmation dialog and plan-token check as a manual start.

## Test and compare

- **Automatic report card** (`report.go`, computed when a run is registered):
  - eval loss base vs tuned (all recipes);
  - exact match for `extract | rewrite`: normalized string equality;
  - exact match for `to_json`: semantic JSON equality;
  - label accuracy plus a confusion table for `classify`;
  - for `free`: perplexity base vs tuned and 20 continuation samples;
  - a sample browser with expected, base and tuned side by side.
  - Every number carries the sample size and a "small sample" note when n < 30.
- **LLM judge** (optional, user-started, assistant recipe):
  - at most 100 samples;
  - rubric scores 1–5 for base and tuned against the expected answer, plus a preference;
  - shown as "LLM judge (<model>), indicative only";
  - stored in `eval_reports(kind=judge)`;
  - budget category `chibi_sensei`.
- **Chat.**
  - Endpoint: `POST runtime/chat {targets:[{kind:"tuned",run_id}|{kind:"base",run_id}|{kind:"provider",provider_id}], messages, sampling}`. It answers with an SSE stream of `delta`, `done` and `error` per target.
  - Limits: at most 3 targets, 20 turns and 16,000 characters.
  - The base GGUF comes from the `base_gguf` cache (produced in-run on the first run per profile revision). Profile sampling defaults apply, with advanced overrides.
  - The provider reference uses `chibiLLM` with any provider that allows general chat.

## Local chat runtime

The code forces a change here.
- **Why not the embeddings sidecar.** The embeddings Docker sidecar is used only when AuraGo itself runs in a container: `dockerEmbeddingSidecarsAvailable` (`llama_docker.go:228-236`) needs `DOCKER_HOST` plus `/.dockerenv`. It mounts the model by inspecting AuraGo's own container (`:598-701`).
- **Primary path: native.** The same package ships a pinned native llama-server b9994 for Linux, Windows and macOS on CPU, Vulkan and CUDA (`manifest.go:64-117`). It is downloaded through `onnxrt.AssetCache` under `<data>/embeddings` (`assets.go:9-18`) and run as a loopback child process (`llama_embedder.go:191-243`).
- **Docker sidecar only inside a container.** Inside a container, the sidecar is preferred and native is the fallback, mirroring `embeddings/manager.go:584-635`.
- **AuraGo-Qwen runtime not reused.** `internal/localllm` is a singleton AuraGo-Qwen runtime with its own image and key volume.

Decision: extract the shared pieces, then build a sibling supervisor.

1. **New package `internal/llamacpp`** (behaviour-preserving move; embeddings keeps thin aliases). It receives:
   - `Build`, `DockerImage(backend)` (from `manifest.go:13,22-24`);
   - `RuntimeAsset(goos, goarch, backend)` with the Windows CUDA supplemental archive (`manifest.go:64-117,175-184`, `manager.go:611-621`);
   - `FindServer`, `LoopbackPort`, `RandomAPIKey`, `GPUVerified` (`llama_embedder.go:344-400`);
   - `HardwareInfo` and `DetectHardware` (`candidates.go:82-90`).
   - `selectDockerModelMount` and `containerPathRelative` (`llama_docker.go:636-701,1058-1072`) move to `internal/dockerutil/mounts.go` as `SelectModelMount`.
2. **`internal/chibisensei/chatrt`.** API:
   - `Manager.Acquire(ctx, ModelRef{Key, Path, SHA256, Ctx, Sampling}) (*Instance, error)`;
   - `Instance.Chat(ctx, req, onDelta)`;
   - `Status()`, `StopAll(ctx)`, `Pin(key)`, `Unpin(key)`.
3. **Native mode.** Command: `llama-server --model <path> --alias chibi --host 127.0.0.1 --port <free> --api-key <random> --ctx-size <runtime.context_size> --parallel 1 --n-gpu-layers 0|999 --jinja --no-webui`, using `onnxrt.ConfigureWorkerCommand`. Health comes from `/health`; chat goes to `/v1/chat/completions` with streaming.
4. **Docker mode** (AuraGo in a container):
   - image `llamacpp.DockerImage(backend)`;
   - a private internal network per instance with AuraGo connected (pattern `llama_docker.go:703-742`);
   - a read-only model mount via `SelectModelMount`;
   - hardening `ReadonlyRootfs`, `CapDrop ALL`, `no-new-privileges`, `Memory: runtime.memory_mb`, `PidsLimit 128`;
   - GPU via `DeviceRequests` (NVIDIA) or `/dev/dri` plus group IDs (Vulkan), as in `:836-852`;
   - labels `dockerutil.ManagedLabels("chibi-sensei","chat-runtime","runtime",sha12)` (`managed_resources.go:93-104`).
   - Add `IsChibiSenseiContainerName` to `managed_resources.go` and to the protection lists (`internal/server/containers_protection.go`, the reserved-name checks in `internal/tools/docker*.go`). Remove stale labelled containers at boot.
5. **Backend selection.** `runtime.backend` is `auto | cpu | vulkan | cuda`. `auto` uses `DetectHardware` in the order CUDA, Vulkan, CPU. A GPU start that fails, or that shows no `GPUVerified` evidence, restarts on CPU, and the UI shows a notice.
6. **Limits.**
   - At most 2 instances (tuned and base). Acquiring a third stops the least recently used idle instance, or answers 409 `runtime_busy`.
   - The idle reaper runs every 30 s and stops instances idle ≥ `runtime.idle_minutes`. Pinned instances are exempt but count toward the 2.
   - One request per instance at a time (`--parallel 1`); further requests queue for up to 30 s.
7. **Without a runtime.** When the native runtime fails to start (for example an unsupported platform) and Docker is not available, chat shows "Local testing is not available on this host". The report card still works.

## Config, permissions and preflight

```yaml
chibi_sensei:
  enabled: true              # default true (local dataset studio; never widens other permissions)
  storage: {max_gb: 20}                                   # 1..500
  dataset: {max_rows: 50000}                              # 1000..50000
  import:  {max_rows: 20000, max_file_mb: 50}             # 100..50000; 1..500
  generator: {provider: "", model: "", max_rows_per_job: 500, max_daily_rows: 5000}
  agent:   {enabled: true, provider: "", model: "", job_timeout_seconds: 300, max_daily_turns: 100}
  training: {machine_shape: "NvidiaTeslaT4", delete_remote_after_run: true}
  runtime: {backend: "auto", idle_minutes: 10, memory_mb: 4096, context_size: 4096, serve_port: 0}
kaggle:
  allowed_machine_shapes: []  # new; known values NvidiaTeslaT4, NvidiaL4
```

Config plumbing:
- `internal/config/chibi_sensei.go`: `NormalizeChibiSenseiConfig` returns range errors, in the style of `config/storymaker.go:75-144`.
- Defaults are set in `config.go` next to :598-599, and normalization is called next to :1130.
- Config UI: a new `ui/cfg/chibi_sensei.js` plus an entry in `ui/js/config/catalog.js`.

**Preflight** (`GET preflight?project=`, `preflight.go`). It returns `{"ready":bool,"items":[{"id","ok","level":"block|warn","message_key","admin_fix":{"config_patch":{…}}|null,"link"}]}`. Items:

- Kaggle: enabled; not read-only; `allow_create`; `allow_remote_execute`; `allow_gpu`; `allow_internet`; `allow_download`; `allowed_machine_shapes` contains `training.machine_shape`; credentials present in the Vault; connection (`Service.Test`); Vault available for uploads.
- Network and filesystem write access (`kaggleSnapshot().Access`).
- `remote_timeout_minutes` ≥ the estimate (warn otherwise, suggest "quick").
- Free Kaggle storage and `max_transfer_mb` ≥ the GGUF estimate.
- Quota remaining; no active or `unknown` compute job.
- License consent recorded.
- `allow_delete` (optional, warn: "uploaded data stays on Kaggle").

An admin sees "Enable for Chibi Sensei". It applies exactly the listed `config_patch` through the normal config update path, after a dialog that lists every flag. Non-admins see what to ask for. Nothing is enabled implicitly.

## Security and privacy

- **Data egress.** Training data leaves the machine only to the user's private Kaggle dataset, and only after the start dialog. It names `<owner>/chibi-data-<run12>`, `<owner>/chibi-<…>`, the row count and the byte size, and needs a valid `plan_token`.
  - The token is the sha256 of the canonical plan plus `data_rev`, valid 15 min, kept in memory.
  - The server recomputes the plan at start and answers 409 `plan_expired` on any difference.
  - `create_dataset` and `save_kernel` force `isPrivate:true`, which is also Kaggle's default (`service.go:158-160,180-183`).
- **Secrets.**
  - No secret enters `job.json`, the notebook, logs or events.
  - Gated bases are refused by profile validation, so no Hugging Face token is ever needed on Kaggle.
  - The Kaggle token stays in the Vault behind `kaggle.Service`.
  - The runtime API key is random per instance and registered with `security.RegisterSensitive`.
  - Log text passes through `security.Scrub` before storage.
- **Outbound HTTP.**
  - Hugging Face uses the SSRF-protected client.
  - Kaggle uses its own SSRF clients (`client.go:60`, storage client :155).
  - The trainer's download URLs (llama.cpp tarball, pip, Hugging Face) are fixed in Go code; none comes from user input.
- **Paths.**
  - Imports and exports go only through `desktop.Service`, which keeps workspace containment and no-symlink rules.
  - Artifacts use server-generated names under `data/chibi_sensei/`, opened with `os.OpenRoot`.
  - Kaggle file names pass `validFile`.
- **Prompt injection.**
  - Dataset, document, Hugging Face and eval text is untrusted. It is wrapped with `IsolateExternalData` in every prompt and scanned with `GateScanComposedPrompt`.
  - The Sensei rules say row text is data.
  - Tool results return row text only inside JSON fields, never as instructions.
- **Licenses.** Consent per base model license and version is recorded in `consents` before the first run. Licenses requiring consent (Gemma, LFM) are shown in full name with a link. Hugging Face dataset licenses are shown before import and kept in `source_ref`.
- **Repository rules.** The UI uses `textContent` only, no external assets, and gates mutations with CSRF and same-origin checks.

## Frontend structure

Classic scripts in `ui/js/desktop/apps/chibi-sensei/`, one IIFE each, each under 1100 lines. Load order (composition root last):

1. `chibi-sensei-core.js` (≈450): namespace, `S.el` with no HTML option, icons, bus, error normalization, formatters, local `t` helper.
2. `chibi-sensei-api.js` (≈450): fetch wrapper with one `AbortController` per window, flat errors `{error, code, message}`, the reopening `EventSource` (copied pattern, `storymaker-api.js:30-64`).
3. `chibi-sensei-mascot.js` (≈350): SVG moods via `createElementNS`.
4. `chibi-sensei-projects.js` (≈600): project list, create, archive, the Goal step form, profile cards with the experimental badge.
5. `chibi-sensei-steps.js` (≈350): step rail, readiness, primary-action slot.
6. `chibi-sensei-table.js` (≈900): paged table, inline editor, selection, bulk bar, undo.
7. `chibi-sensei-data.js` (≈800): imports, documents, checks panel, split preview, export.
8. `chibi-sensei-generate.js` (≈600): generation and cleaning dialogs, progress, review diff.
9. `chibi-sensei-training.js` (≈950): preflight, preset cards, advanced overrides, consent, confirmation dialog, run monitor.
10. `chibi-sensei-chart.js` (≈300): SVG loss chart.
11. `chibi-sensei-test.js` (≈900): report card, sample browser, chat, side-by-side (P3).
12. `chibi-sensei-hf.js` (≈700): curated list, search, mapping, import (P4).
13. `chibi-sensei-panel.js` (≈700): Sensei panel, proposal cards (P4).
14. `chibi-sensei-use.js` (≈400): export and pin (P4).
15. `chibi-sensei.js` (≈700): instance map per window, layout, routing, SSE fan-out, disposal; publishes `window.ChibiSenseiApp`.

Additional rules:
- CSS reads `--vd-mat-*` roles with fallbacks (canvas, bar, side, card, well, field, btn, primary, selected), uses container queries on the window and gates motion (`TestDesktopAppMaterialsAdoptedByApps`).
- The modules translate through a local helper, so the app is registered in `desktopLocalI18nHelpers` (`ui/i18n_lint_test.go`) under `chibi_sensei.`.
- Copy lives in 16 locales with real translations; German uses "Du".
- Phases add modules; the composition root loads only modules that exist, so P1 ships modules 1–10 (training shows the plan only).

## Error handling

- **Envelope.** Flat `{error, code, message}`, plus `issues` for 422 and `missing` for preflight-related refusals.
- **Bodies.** Bounded through `decodeJSONRequest` (`json_request.go:32`): 1 MiB by default, 16 MiB for row batches.
- **HTTP codes:**
  - `chibi_disabled` 403, `desktop_disabled` 503, `desktop_readonly` 403, `not_found` 404, `conflict` 412;
  - `validation_failed` 422, `dataset_full` 409, `import_failed` 422;
  - `kaggle_unavailable` 503, `kaggle_permission_missing` 409, `kaggle_quota_low` 409, `kaggle_slot_busy` 409;
  - `run_active` 409, `plan_expired` 409, `license_consent_required` 409;
  - `runtime_unavailable` 503, `runtime_busy` 409, `model_missing` 404;
  - `budget_exceeded` 429, `provider_unavailable` 503;
  - `hf_disabled` 403, `hf_gated` 403, `hf_rate_limited` 429.
- **Run failure codes.** These come from `chibi-status.json` or the orchestrator. Each maps to a message key and a suggested fix:
  - `env_no_gpu`, `env_unsupported_gpu` (suggest the machine shape), `env_no_internet` (phone verification);
  - `install_failed`, `data_mismatch`, `base_download_failed`;
  - `oom` (suggest preset quick or a lower `max_seq_len`), `train_diverged` (lower LR), `time_budget` (partial result kept; succeeded with a warning);
  - `merge_failed`, `gguf_failed`, `gguf_arch_mismatch`;
  - `kaggle_submit_unknown`, `remote_timeout`, `status_missing` (log tail shown), `download_failed`, `checksum_mismatch`.
- Errors shown in the UI never contain host paths (`kaggle.PublicError` style) or secrets.

## Testing

No test needs a GPU or a network.

- **Go `internal/config`.** Defaults, ranges, the Kaggle `allowed_machine_shapes` normalization, provider references.
- **Go `internal/kaggle`:**
  - `CheckGrants` with allowed and refused shapes (no `AllowGPU`, `enableGpu` false, unknown shape, `dockerImage` still refused);
  - `intersectConfig` intersects the list;
  - `ReadKernelLog` against a fixture client (grant checks, owner check, redirect, tail cap, no job row);
  - `RemoveFile` path safety.
- **Go `internal/chibisensei`:**
  - profile validation (`verified` needs empty `Unverified`, gated refused);
  - preset tables;
  - split determinism and stratification;
  - token estimate;
  - every check, with fixtures;
  - dedupe and history undo;
  - import mappers (JSONL, CSV, Hugging Face feature mapping);
  - `job.json` and notebook golden files per profile (`testdata/*.golden`);
  - log parser on JSON-array and plain-text fixtures;
  - report scoring;
  - the run state machine against `fakeKaggle`, an in-memory `KaggleDriver`: happy path, every `unknown` branch, restart at each state, timeout, cancel before and after submit, AllowDelete off leaving leftovers, checksum mismatch.
- **Trainer.** `trainer/selftest_test.go` runs `--selftest` and `--dry-run` when `python3` is on PATH (skip otherwise) and parses the outputs with the production parsers.
- **Go `internal/server`.** Handlers and gates (disabled, read-only, CSRF, scope), plan-token expiry, bulk delete token, preflight items, capability gating, LLM jobs against a fake `ChatClient`, the Sensei hooks routing only `chibi_` calls, and a test that `action_policy` admits `chibi_` under an enabled policy.
- **chatrt.** A fake process launcher for LRU, idle and pin behaviour; a Docker payload golden; an optional real smoke behind `AURAGO_RUN_LLAMA_SMOKE=1` with a tiny GGUF.
- **UI (usual set):**
  - `ui/desktop_chibi_sensei_registration_test.go` (loader order, shell markers, bundle);
  - `internal/desktop/chibi_sensei_app_test.go` (`Requires`);
  - `ui/desktop_chibi_sensei_i18n_test.go` (16 locales, key parity);
  - `_css_test.go` (materials, `csn-` prefix);
  - `_guards_test.go` (no innerHTML or template HTML, no eval, no storage, line budget);
  - `scripts/test-chibi-sensei-model.mjs` (readiness and paging state in Node);
  - the browser smoke `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestChibiSenseiApp` against a fixture backend.
- **Manual proof run per profile.**
  - Steps: quick preset with `internal/chibisensei/testdata/proof/<recipe>.jsonl` (200 rows) on a real Kaggle account; run succeeds; artifacts verify; the GGUF loads in the local b9994 runtime and answers 3 prompts.
  - Record pins, measured tok/s, GPU name and the Kaggle ref in the profile. Flip `status` to `verified` and empty `Unverified`.
  - Until P3 lands, the load check uses `llama-cli` from the pinned b9994 release.

## Phases

**P1: Foundation and dataset studio**
- Scope:
  - config section, app registration, capability, routes, store;
  - projects and the Goal step with profile cards (all experimental);
  - rows table, file imports, documents, dedupe, checks, split, JSONL export;
  - plan display (read-only estimate);
  - one-shot generation and cleaning;
  - mascot, step rail, 16 locales, DOX.
- The user can create a project, build and check a dataset with or without the agent, and export JSONL.
- Acceptance:
  - 50,000-row project pages under 300 ms per page on the test server;
  - dedupe and bulk delete with undo;
  - every check fires on fixtures;
  - generation is capped and cancellable;
  - read-only desktop refuses writes;
  - the app hides when disabled;
  - all UI tests are green.
- Files:
  - `internal/config/{chibi_sensei.go,chibi_sensei_test.go,config.go,config_types.go,sqlite_paths.go,provider_references.go,decisions.go}`;
  - `internal/desktop/{types.go,chibi_sensei_app_test.go}`;
  - `internal/server/{bluetooth_live.go,server_routes.go,server.go,chibi_sensei_handlers.go,chibi_sensei_service.go,chibi_sensei_rows.go,chibi_sensei_imports.go,chibi_sensei_jobs.go,chibi_sensei_llm.go,*_test.go}`;
  - `internal/chibisensei/{store.go,schema.go,rows.go,history.go,checks.go,split.go,tokens.go,imports.go,export.go,profiles.go,profiles/*.json,presets.go,plan.go,jobs.go,events.go,AGENTS.md,*_test.go}`;
  - `ui/js/desktop/apps/chibi-sensei/` modules 1–10 and 15;
  - `ui/css/desktop-app-chibi-sensei.css`, `ui/lang/chibi_sensei/*.json`, `ui/lang/desktop/*.json`, `ui/img/desktop-icons/chibi-sensei.svg`;
  - the shell files listed under "Naming and registration", plus `ui/js/desktop/bundles/main.bundle.js`;
  - `ui/cfg/chibi_sensei.js`, `ui/js/config/catalog.js`, `ui/i18n_lint_test.go`;
  - the UI tests, `ui/js/desktop/apps/AGENTS.md`.

**P2: Kaggle training**
- Scope: Kaggle extensions, trainer and selftest, `job.json` and notebook generation, preflight with admin enabling, consent, confirmation and plan token, run state machine and reconciler, live progress and fallback, downloads, artifact registry, cleanup, basic report numbers (loss, exact match, label accuracy), proof runs of all three profiles.
  - The proof runs come early in P2, as soon as the trainer and a manual notebook push work, because the two newer models carry the largest risk. A profile that cannot be made to pass is reported to the user with the evidence and a replacement candidate before more of P2 is built on it.
  - The local load check of each GGUF uses `llama-cli` from the pinned b9994 release until P3 lands.
- The user can train on Kaggle and download a GGUF.
- Acceptance:
  - fake-Kaggle state-machine suite green, including restart at every state;
  - each of the three proof runs passes end to end and its profile is flipped, or the failure is documented and the user has decided what replaces the profile;
  - cancel copy matches behaviour;
  - no secret in the notebook (golden diff);
  - `allow_delete` off leaves listed leftovers.
- Files:
  - `internal/kaggle/{policy.go,logs.go,transfers.go,AGENTS.md,*_test.go}`, `internal/config/kaggle.go`, `ui/cfg/kaggle.js`;
  - `internal/chibisensei/{trainer/chibi_train.py,trainer/requirements.lock,trainer/embed.go,trainer/selftest_test.go,jobspec.go,notebook.go,runs.go,reconcile.go,logparse.go,artifacts.go,preflight.go,report.go,testdata/**}`;
  - `internal/server/{chibi_sensei_kaggle.go,chibi_sensei_runs.go,chibi_sensei_preflight.go}`;
  - UI modules 9–10, extended from the plan-only state of P1.

**P3: Test and compare**
- Scope: `internal/llamacpp` extraction, `dockerutil` mounts and names, chatrt, chat and side-by-side, provider reference, judge, automatic smoke in `verifying`.
- The user can chat with the tuned model and compare it with the base model and a provider on CPU, Vulkan or NVIDIA.
- Acceptance:
  - embeddings tests unchanged and green after the move;
  - two-instance limit and idle stop;
  - GPU fallback to CPU;
  - in-container sidecar works on the test server;
  - report card with judge labelled.
- Files:
  - `internal/llamacpp/*`, `internal/embeddings/{manifest.go,llama_embedder.go,llama_docker.go,candidates.go,AGENTS.md}`;
  - `internal/dockerutil/{mounts.go,managed_resources.go}`, `internal/server/containers_protection.go`, `internal/tools/docker*.go`;
  - `internal/chibisensei/chatrt/*`, `internal/server/{chibi_sensei_runtime.go,chibi_sensei_report.go}`;
  - UI module 11.

**P4: Hugging Face import, Sensei panel, use in AuraGo**
- Scope:
  - curated list and search; import job with mapping and license;
  - Sensei agent with tools, proposals and undo;
  - export of GGUF plus model card;
  - admin "serve as provider": a pinned instance on `runtime.serve_port` (native) or `aurago-chibi-serve` (Docker), a `custom` provider entry added through a config patch with its key in the Vault as `provider_<id>_api_key`, and unpin refused while other config references the provider.
- The user can import Hugging Face datasets, get guided help and use the model in AuraGo.
- Acceptance:
  - 20,000-row import paced without a 429 storm;
  - agent cannot start runs or bulk-delete;
  - `action_policy` test green;
  - the provider survives an AuraGo restart.
- Files:
  - `internal/chibisensei/{hfimport.go,hfcatalog.json,proposals.go}`;
  - `internal/agent/{native_tools_chibi_sensei.go,action_policy.go,*_test.go}`;
  - `internal/server/{chibi_sensei_hf.go,chibi_sensei_agent.go,chibi_sensei_agent_profile.go,chibi_sensei_use.go}`;
  - `ui/js/desktop/core/sdk-events-bootstrap.js`;
  - UI modules 12–14.

**P5: Local NVIDIA training**
- Scope: backend `local_nvidia` in `internal/chibisensei/localtrain`.
  - A Docker container from a digest-pinned PyTorch CUDA image (chosen and pinned in the P5 plan), with the GPU via `DeviceRequests` (pattern `internal/acestep/docker.go:121-132`).
  - The run directory is mounted, the same script and `job.json` run with `--out /work/out`, and network access is used for pip and Hugging Face.
  - Live logs come from the container log stream. Cancel is a container stop, which really cancels.
  - States skip upload and dataset; preflight checks Docker not read-only, the NVIDIA runtime and VRAM.
- The user can train without Kaggle.
- Acceptance: one profile trained locally on the test server or a dev GPU; cancel frees the GPU in under 30 s.

## Risks and unverified assumptions

| Assumption | How and when it gets verified |
|---|---|
| Kaggle accepts `machineShape: "NvidiaTeslaT4"` through SaveKernel and grants a T4 | P2 Gemma proof run: `env_check` logs the GPU name |
| Default GPU without a shape is an unsupported P100 | Same run with the shape omitted once (manual); the result is recorded in the profile notes |
| The session log is readable while running | P2 proof run: `ReadKernelLog` returns `@@chibi` lines before completion; otherwise the fallback stays the documented behaviour |
| Status strings of `get_kernel_session_status` and `get_dataset_status` | P2 proof run; parser tests updated from real responses |
| Outputs in `/kaggle/working` survive a `COMPLETE` session and download by flat name | P2 proof run |
| `licenseName:"unknown"` accepted for private datasets | P2 proof run (fallback `CC0-1.0`, chosen by the user) |
| Quota response shape | P2: parsed against a real response; unparsable responses warn |
| Library pins load Qwen3.5 text-only, LFM2.5 and Gemma 3 in fp32; LoRA target names; Triton compile time on T4 | P2 proof runs of each profile |
| `convert_hf_to_gguf.py` at b9994 converts fine-tuned LFM2.5 and Qwen3.5, and b9994 loads `lfm2` and Qwen3.5 GGUFs | P2 proof runs with `llama-cli` b9994; again through chatrt in P3 |
| b9994 is new enough for all three architectures. If it is not, the shared llama.cpp pin must be raised, which also changes the embeddings runtime | Decided at the P2 proof runs; a pin change gets its own task with the embeddings tests |
| `--jinja` and `--no-webui` exist in b9994 | P3 first chatrt task runs `llama-server --help` from the pinned asset |
| `unsloth/gemma-3-270m-it` stays ungated and matches Google's weights | P2 start: check the repo card and config hash; fallback is Kaggle Models via `modelDataSources` after the user accepts the license on kaggle.com |
| fp32 memory for 1.2B at seq 2048 on a T4 | P2 LFM proof run, repeated once with preset balanced |
| Throughput figures in profiles | Each proof run measures and records them |
| Phone-verified account needed for internet | `env_no_internet` message checked in P2 with an unverified test account when available |
| Backup handling of large files under `data/chibi_sensei/` | P2 plan task: read the backup code and exclude `models/` and `base/` if they are included |
| Hub search parameters | P4 first task against the live API |

## Out of scope

- Hugging Face Hub upload, Kaggle Models publishing, public datasets or notebooks.
- Multi-GPU, TPU, hyperparameter search, resuming training across sessions, checkpoints.
- Tool-calling and vision fine-tuning; DPO or RLHF; quantization other than Q8_0 (no `llama-quantize`).
- Ollama registration (Qwen3.5 GGUFs do not run in Ollama); serving more than one pinned model.
- Near-duplicate detection, tokenizer-exact counting, a Parquet reader, gated base models.
- Changes to the Kaggle app's job list; a generic shared job manager.

## Settled questions

These came up during the design and are settled (see decision 6). The reasons are kept here.

1. `chibi_sensei.enabled` defaults to true, like Storymaker. P1 is purely local, and every remote step is gated by Kaggle permissions that stay off by default.
2. A run deletes its Kaggle dataset and notebook when `allow_delete` is granted. Training data should not linger in the cloud; leftovers are listed with links when deletion is not allowed.
3. The base GGUF for comparison comes from the training run, cached per profile revision. It needs no extra client and guarantees the same converter and template as the tuned model.
4. The Gemma base comes from the `unsloth` mirror with consent recorded in AuraGo. Kaggle Models (license accepted on kaggle.com, no Hugging Face download) is the documented fallback if the mirror becomes gated.
5. "Serve as provider" stays in P4, admin-only and explicitly pinned. Without it, "use in AuraGo" is only a file export.
