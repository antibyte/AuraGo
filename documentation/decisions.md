# Decisions

Decisions is an optional shared service for decisions from text and images,
using either a managed local runtime or a saved external provider connection.
It is off by default. Consumers include EasyDrag's **Decisions** node and the
optional [LLM router](llm-router.md). The router uses this service for unclear
tasks when both features are enabled; its **Use Decisions** switch defaults on.
Routing never starts or installs a stopped local service. External classification
sends bounded task text and may incur cost, booked once under `routing`.

See [model versions, image preparation and evaluation](decisions-validation.md)
for the shared availability contract, shipping limits and opt-in evaluation.

## Setup and permissions

For local processing, open **Configuration → Decisions**, select **On the
AuraGo server**, and click **Set up Decisions**. This enables the local feature,
saves its deployment and explicit CPU permission, installs the runtime and model,
starts the service, and runs a fixed text probe. Other unsaved settings are kept.
The model and image projector need approximately 2 GB; runtime packages and
temporary extraction space are additional. No compiler or CUDA SDK is required
on the user's machine. Compatible graphics drivers remain a prerequisite.

The phases show system/runtime checks, runtime download, model download,
verification, loading/warmup, and a function check. Download and hashing phases
show bytes when measurable; loading uses an activity indicator. **Decisions is
ready** appears only after actual device attestation and a valid score response.
Low confidence in that probe does not indicate an installation failure.

**Cancel** stops the whole setup. Closing the page leaves the server-owned
operation running, and reopening it restores progress from status. Duplicate
clicks do not start another installation. Disabling Decisions, revoking CPU
permission, changing its saved deployment or Docker permissions, and server
shutdown cancel the current operation and prevent a delayed start. Verified
downloads remain reusable. Setup repairs missing or damaged files.

Under **Run mode and CPU permission**, `auto` selects a native process for a
native AuraGo installation and a managed container when AuraGo itself runs in
Docker. Containers require existing Docker enable/write permissions. The Docker
socket proxy keeps `BUILD=0`: setup pulls published digest-pinned images and
does not build them. GPU/iGPU is preferred; CPU and mixed operation require the
separate explicitly selected, saved permission. A missing accelerator opens
that setting and explains the choice.

When AuraGo runs in Docker and its data uses a named volume, Decisions needs
Docker Engine 26 or newer (API 1.45) to mount only its model and credential
subdirectories. Older Engines report `docker_api_too_old` before installation
or launch. Upgrade the Engine or explicitly configure a suitable bind mount;
AuraGo never falls back to exposing the whole data volume. Bind-mounted data
does not require this volume-subpath capability.

After setup, **Test decision** runs another explicit text test; merely opening
settings only reads status. **Stop** stops the process or container. Advanced
options retain the independent check and start actions for diagnosis. Setup
already downloads the model; no separate download action is needed. A failed
setup or prerequisite check reveals the setup guide. Installation through the
legacy API alone leaves the service installed; it does not grant readiness.
Reinstallation stops a serving generation before
replacing files. Evaluation failures never silently replay the request or change
the active backend.

Runtime downloads are authorized exclusively by the catalog compiled into
AuraGo: fixed release URLs, archive sizes/hashes, manifest hashes and Docker
digests. Versioned packages are fully verified, including all bundled libraries,
before execution. New runtime versions have separate directories. File hashing
and extraction observe cancellation, and Windows file replacement uses the shared
retry handling. User requests cannot choose download URLs or runtime paths.

The bundled catalog pins [runtime 1.0.0](https://github.com/antibyte/AuraGo/releases/tag/decisions-runtime-v1.0.0):
15 native packages and three Docker images for amd64 and arm64. Every native
archive and both architectures of every image passed anonymous download/pull
verification. See [runtime publication and system requirements](decisions-runtime-release.md).
Locally built, verified developer bundles and existing local Docker images
remain usable for development; normal setup downloads the published packages.

Each runtime launch has a five-minute readiness budget after AuraGo's own file
verification. This includes the runtime's independent hashes, loading and first
device warmup. `runtime_start_timeout` stops the attempt without interpreting
slowness as device failure or trying another device/CPU mode. Pin failures and
invalid startup configuration also stop retries; Docker reports these early
exits directly. Real device failures still follow the accelerator policy below.
Older runtime packages remain compatible but need rebuilding to emit load and
warmup phases. No router timeout or confidence threshold is changed by this
startup budget.

### Accelerator policy

CUDA is preferred for suitable NVIDIA devices, Metal on Apple Silicon, and
Vulkan for other supported GPUs/iGPUs. Dedicated GPUs are preferred to integrated
devices. A failed accelerator attempt leads to another suitable accelerator.
Flags and hardware detection do not establish readiness: the runtime must
confirm actual model, projector, and KV placement and computation.

**CPU fallback and mixed GPU/CPU operation are denied by default.** The separate
CPU permission switch explicitly permits both and persists across restarts.
Even with permission, full acceleration is tried first. CPU participation and
its reason are visible in status. Revoking permission immediately cancels any
CPU-assisted inference; if complete acceleration is unavailable the service
remains unavailable. Ordinary tokenization, image file handling, and reading the
selected logits on the host are not CPU model inference.

```yaml
decisions:
  enabled: false
  backend: local         # local or provider; omitted values preserve local operation
  deployment: auto       # auto, native, docker
  cpu_fallback_allowed: false
```

### External processing with OpenRouter

In **Providers**, create an OpenRouter connection with purpose **Decisions**, or
reuse an existing general OpenRouter connection. Store its API key through the
provider editor, or authorize an OAuth connection there; AuraGo keeps both kinds
of credentials in the encrypted Vault. Decisions picks up renewed OAuth access
tokens and cancels requests using revoked or replaced credentials. A dedicated Decisions
connection does not require a chat model and is excluded from chat, helper and
media provider selectors.

In **Decisions**, select the external backend and the saved connection, explicitly
load its decision models, select a model, and save. The model belongs to Decisions
configuration, independently of the connection's chat model. **Test decision**
sends a fixed small example and may incur provider charges. Opening the settings
or reading status never runs an inference. A configured connection can be used
without running the test first; readiness and the last successful request are
shown separately from configuration completeness.

```yaml
providers:
  - id: decisions_cloud
    type: openrouter
    usage: decisions
    name: Decision models
    base_url: https://openrouter.ai/api/v1
decisions:
  enabled: true
  backend: provider
  provider: decisions_cloud
  model: microsoft/microsoft-decision-1
```

The adapter uses OpenRouter's **`POST /api/alpha/decisions`**, not chat completions.
It discovers models through **`GET /api/v1/models?output_modalities=decisions`**
and verifies advertised capabilities before evaluation. Models advertising image input can accept
authorized images within the adapter's limits. An unavailable model or unsupported
image input fails explicitly. There is no automatic fallback between local and
external backends, models, or ordinary chat APIs.

External processing sends the supplied state, questions and authorized images
to OpenRouter and its executing provider. Selecting the external backend is the
operator's choice to allow that transfer. Images use in-memory data URLs; AuraGo
does not upload arbitrary URL references or expose filesystem paths. Only the
canonical OpenRouter HTTPS origin is accepted, redirects are refused, and no
credentials or provider response bodies are included in errors.

Local install/start/stop, accelerator attestation and CPU consent do not apply to
external inference. Existing local settings and downloads are preserved when
switching backends. Disabling Decisions, changing its configuration or provider,
or shutting down cancels active work and rejects late results. Remote cancellation
cannot guarantee that the provider stopped processing or charging. An uncertain
POST outcome is never automatically retried, including by EasyDrag node retries.

Reported USD costs are recorded once in the existing Decisions budget category.
An absent cost remains unknown, not zero; a timed-out request may still be charged
upstream without an amount AuraGo can record. Budget enforcement blocks a new
request once the configured limit has been reached; it cannot reserve an exact
price for an in-flight request.

See the [OpenRouter Decisions API](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request)
and [model discovery documentation](https://openrouter.ai/docs/guides/overview/models).

## EasyDrag

| Mode | Questions | Outputs |
| --- | --- | --- |
| Decide and branch | One selection or yes/no question | One port per stable answer ID, plus Uncertain |
| Evaluate several questions | Selection, yes/no, or numeric rating | Result or Uncertain, with all answers |

Question and option IDs remain stable when labels are renamed or reordered.
Deleting an option leaves existing connections visible as validation errors.
The editor supports the existing variable syntax for state, question data, and
image references. References can identify registered images (`media:<id>` or an
existing `/files/…` registry URL) or a file already permitted by AuraGo's workspace
and document attachment policy. An arbitrary URL or path grants no new access.

The default minimum confidence is **0.70**. A result exactly at the threshold
is accepted. In multi-question mode, any answer below it selects Uncertain and
retains every answer. Technical errors use **Stop** or the existing **Error
output**; they never select an answer port or masquerade as uncertainty.

`answers.<question-id>` contains `decision`, `type`, `probabilities`, and
`confidence`. Selection questions additionally contain the selected `choice`
label; yes/no (`noul`) questions contain `noul`, the probability of **yes**; rating
questions contain `score`, the probability-weighted numeric expectation. Omitted
rating values default to their zero-based option positions. Explicit values
must be finite and distinct. Use explicit values when ratings must retain their
meaning after reordering.

## API and scoring

The authenticated administrator API provides:

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/decisions/status` | Passive status and actual runtime/device metadata |
| GET | `/api/decisions/models?provider=<saved-id>` | Explicit external decision-model catalog lookup |
| POST | `/api/decisions/setup` | Save local Decisions settings, install, start, and verify; returns 202 with operation ID |
| POST | `/api/decisions/install` | Start managed installation |
| POST | `/api/decisions/check` | Read local installation prerequisites; no download, start or inference |
| POST | `/api/decisions/start` | Start the installed service |
| POST | `/api/decisions/stop` | Cancel and stop the service |
| POST | `/api/decisions/test` | Explicit text evaluation |
| POST | `/api/decisions/evaluate` | Evaluate authorized input |

Setup accepts `{ "deployment": "auto", "cpu_fallback_allowed": false }`. CPU permission must be explicitly present. Its response contains `setup_id` and the saved `decisions` section; status adds `setup_id`, `setup_active`, and phase/byte progress. The legacy endpoints remain supported.

Evaluation example:

```json
{
  "state": {"alarm": "active", "temperature": 38},
  "questions": [
    {"id": "intervene", "type": "noul", "instructions": "Is intervention needed?"},
    {"id": "urgency", "type": "score", "instructions": "Rate urgency.",
     "options": [
       {"id": "low", "label": "Low", "value": 1},
       {"id": "high", "label": "High", "value": 5}
     ]}
  ],
  "images": []
}
```

The Go `Evaluate(ctx, Request)` API is independent of EasyDrag. Its image values
contain already authorized bytes; the public HTTP/flow adapters resolve file
references first. A bounded queue serializes evaluations against the resident
model and supports cancellation. Each local question starts a separate prompt/context;
there is no cross-request conversational memory.

Local prompts follow Jeff's published **Task / State / Options / Decision** format.
Candidate codes must each be exactly one token in the actual prompt context.
The private authenticated native service reads every candidate logit directly.
Stable softmax is computed only over those candidates, retaining an entry even
for very unlikely options. `confidence` is the largest resulting probability.
Missing, duplicate, non-finite, or inconsistent scores are errors.

The local response includes the exact model revision, full answer distributions, actual
runtime/device information, and token counts. It explicitly says
`calibration: "uncalibrated"`. The original Intern-Decision temperature is not
transferred to Jeff. A high probability is a relative preference among the given
options, not a calibrated guarantee of correctness.

External results preserve those answer semantics: `decision` is the original
option ID, `choice` its label, `confidence` the selected option's probability,
and `score` uses the caller's numeric option values. OpenRouter's separate
confidence measure is exposed as `provider_confidence` without replacing
`confidence`. `execution` identifies the configured connection and executing
provider; `usage.cost_usd` is present only when reported. External results remain
`uncalibrated` and never claim local device attestation.

The external adapter accepts JPEG and PNG (including server-normalized WebP),
with additional conservative limits: four images, at most
300 KiB each, a 3 MiB serialized request, and the model's advertised context
budget. Known model-specific limits, including Clef's text truncation boundary,
are checked before sending rather than silently dropping input. Exceeding a
limit requires a smaller request or another compatible model.

## Limits

The public input limits below apply to both backends. External evaluation also
enforces the smaller image/request limits and advertised model context described
above; the 8192-token scoring budget applies only to the local runtime.

- 16 ordered questions per evaluation, 2–62 options per choice/rating question;
  yes/no uses the fixed stable IDs `no` and `yes`.
- Eight images, at most 12 MiB each and 24 MiB combined, at most 32 million pixels
  per image. Supported public inputs are PNG, JPEG, GIF, and WebP; GIF and WebP
  are decoded to one frame and re-encoded as PNG within the same byte limits.
  PNG and JPEG stay as detected. Source files and normalized images each have a
  separate 24 MiB total cap.
- 8192 input tokens **including image tokens across the evaluation**. Independent
  questions repeat their shared state and images, so those tokens count again.
  Each question is tokenized against the remaining budget before inference.
  If a later question exceeds it, the whole evaluation fails without a partial
  result. Inputs are never silently truncated.
- The HTTP JSON envelope is limited to 1 MiB; state is limited to 256 KiB and
  each question's instructions and each option label to 16 KiB before tokenization.
  Question and option IDs are at most 128 bytes (EasyDrag uses its stricter
  32-character lowercase identifier format for stable ports).

## Local runtime pins and acceptance

Both model files come from
[jersobh/jeff-2b-gguf](https://huggingface.co/jersobh/jeff-2b-gguf/tree/263ac27c59688a360d2db255c4f7e57f665bcca4),
revision `263ac27c59688a360d2db255c4f7e57f665bcca4`:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `jeff-2B.Q4_K_M.gguf` | 1274396448 | `d4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b` |
| `jeff-2B.mmproj.gguf` | 671372608 | `1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68` |

The base model is [Intern-Decision-2B](https://huggingface.co/internlm/Intern-Decision-2B).
Runtime package manifests record exact source/build pins and binary hashes.

Unit, protocol, and browser tests establish lifecycle, input, and UI behavior;
they do not establish hardware compatibility or model quality. Release acceptance
requires real CUDA, Metal, and Vulkan GPU/iGPU trials, including out-of-memory,
device failure, alternate devices, full projector offload, approved mixed/CPU
execution, revocation during inference, text/image examples, and rare-candidate
distributions. Jeff's image quality and confidence behavior require separate
evaluation. Publishing runtime releases and deployment are separate operations.

For a real local native acceptance run, first build the matching accelerator
bundle and place the verified model files on the same volume as the test's
temporary directory. Set `AURAGO_DECISIONS_LIVE_MODELS` to that model directory
and `AURAGO_DECISIONS_LIVE_RUNTIME` to the AuraGo source/install root, then run
`go test ./internal/decisions -run '^TestDecisionsLiveNative$' -v -count=1 -timeout 15m`.
Setting `AURAGO_DECISIONS_LIVE_IGPU=1` additionally requires an integrated GPU.
The test keeps CPU consent disabled, checks dedicated-device priority, full
offload attestation, text/image/multi-question results, all 62 candidate codes,
token-budget rejection, and shutdown. Its small semantic examples are smoke
tests, not a model-quality or confidence-calibration benchmark.
