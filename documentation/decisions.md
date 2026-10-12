# Decisions

Decisions is an optional shared local service for decisions from text and images.
It is off by default and is independent of normal chat, helper LLMs, and agents.
EasyDrag's **Decisions** node is its first consumer.

## Setup and permissions

Open **Configuration → Decisions**, enable the service, select its deployment,
and save. `auto` selects a native process for a native AuraGo installation and a
managed container when AuraGo itself runs in Docker. Container management also
requires the existing Docker enable/write permissions.

The **Install** action verifies the runtime bundle and downloads the model plus
image projector with progress. **Start** loads one resident model for all callers.
**Test decision** runs a small explicit text test; merely opening settings only
reads status. **Stop** stops the process or container. Disabling the feature
cancels requests and stops it while preserving verified downloads.
Reinstalling stops a loaded runtime before replacing its files. A runtime or
device failure retires the active generation and cancels queued work; **Start**
performs device selection again. The failed evaluation is never silently replayed.

Runtime bundles must be built and packaged from the pinned native source under
[`decisions_runtime`](../decisions_runtime/). Local native bundles are staged in
`bin/decisions-runtime/<os>-<arch>/<backend>/` under the trusted AuraGo install
directory. The manager provisions verified bundles into its data directory.
Docker requires a locally built, verified image; a mutable tag alone never
establishes a trusted running image. There are no fabricated remote release URLs
or automatic pulls of an unpinned runtime. A missing bundle is a setup error.
See the runtime README for build and packaging commands.

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
  deployment: auto       # auto, native, docker
  cpu_fallback_allowed: false
```

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
| POST | `/api/decisions/install` | Start managed installation |
| POST | `/api/decisions/start` | Start the installed service |
| POST | `/api/decisions/stop` | Cancel and stop the service |
| POST | `/api/decisions/test` | Explicit text evaluation |
| POST | `/api/decisions/evaluate` | Evaluate authorized input |

Example body:

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
model and supports cancellation. Each question starts a separate prompt/context;
there is no cross-request conversational memory.

Prompts follow Jeff's published **Task / State / Options / Decision** format.
Candidate codes must each be exactly one token in the actual prompt context.
The private authenticated native service reads every candidate logit directly.
Stable softmax is computed only over those candidates, retaining an entry even
for very unlikely options. `confidence` is the largest resulting probability.
Missing, duplicate, non-finite, or inconsistent scores are errors.

The response includes the exact model revision, full answer distributions, actual
runtime/device information, and token counts. It explicitly says
`calibration: "uncalibrated"`. The original Intern-Decision temperature is not
transferred to Jeff. A high probability is a relative preference among the given
options, not a calibrated guarantee of correctness.

## Limits

- 16 ordered questions per evaluation, 2–62 options per choice/rating question;
  yes/no uses the fixed stable IDs `no` and `yes`.
- Eight images, at most 12 MiB each and 24 MiB combined, at most 32 million pixels
  per image. Supported public inputs are PNG, JPEG, GIF, and WebP; WebP is
  normalized to PNG within the same byte limits. Source files and normalized
  images each have a separate 24 MiB total cap. Animated inputs use one frame.
- 8192 input tokens **including image tokens across the evaluation**. Independent
  questions repeat their shared state and images, so those tokens count again.
  Each question is tokenized against the remaining budget before inference.
  If a later question exceeds it, the whole evaluation fails without a partial
  result. Inputs are never silently truncated.
- The HTTP JSON envelope is limited to 1 MiB; state is limited to 256 KiB and
  each question's instructions and each option label to 16 KiB before tokenization.
  Question and option IDs are at most 128 bytes (EasyDrag uses its stricter
  32-character lowercase identifier format for stable ports).

## Pins and acceptance

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
