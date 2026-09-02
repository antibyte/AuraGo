# AuraGo managed local test models

AuraGo manages one local model at a time: **AuraGo-Qwen** or **AuraGo-Ling**.
Config and Setup select the family; the existing manager owns downloads,
Docker lifecycle, authentication, hardware checks and provider routing.
Switching families uses the regular restart/recreate flow and invalidates
model/engine verification and RAM prompt caches. Downloaded Qwen files remain.

## AuraGo-Ling

Ling uses `antibyte/AuraGo-Ling`, file `AuraGo-Ling-3.0-tiny-Q4_K_L.gguf`
(5,096,544,352 bytes; SHA256
`4c25f349d6ea6872907c6fbd827d4b90abfd420320394a8cf420ce9b60abee68`).
It is an experimental tool-call fine-tune of the MIT-licensed Ling-3.0-tiny.
The existing export scored 68/76 corrective cases and 49/49 replay cases;
general framework knowledge remains unreliable. These synthetic results do
not establish live agent reliability or qualify the new engine.

```yaml
local_llm:
  model_family: ling
  model_variant: q4_k_l
  mtp: off
  context_size: 16384
```

The API model alias is `aurago-ling`; the internal provider ID remains
`aurago-qwen-local` for compatibility. Missing `model_family` selects Qwen.
Ling rejects other quantizations, draft modes and 32K until separately qualified.
Both UIs choose valid dependent settings when changing the family.

Ling pins `antibyte/llama-wackMall-hybrid` at
`f37a34cd4e502284ca297e141a6c4013bd151b18`; Qwen retains its existing engine/images.
CUDA uses prompt/decode batches 2048/64, Q8 KV, flash attention and full GPU
offload. SM75 kernel tuning applies only to detected compute capability 7.5.
SYCL/Vulkan use batch 512, F16 KV and automatic flash attention without CUDA
tuning. All Ling profiles use the GGUF chat template, Thinking off, one slot,
no MTP/DFlash, no KVFlash eviction and no `--fit` context reduction.
The complete 16K context must remain available.

The Intel Arc B580 (`0xe20b`) Vulkan profile disables F16 compute kernels with
`GGML_VK_DISABLE_F16=1`; KV storage stays F16. Default F16 compute reproduced
truncated slash-only responses in both tested engines. The workaround is
restricted to Ling/Vulkan/B580 and does not qualify the Linux backend.

The release manifest stays closed until the public HF commit and all three
image digests are verified. An image without native Linux GPU qualification
remains experimental and cannot be selected automatically. Windows/WSL tests
do not qualify Linux. The historical report of over 100 tokens/s is not a
benchmark for this Q4_K_L export. The engine's Apache-2.0 license is separate
from the model's MIT license.

## AuraGo-Qwen

AuraGo-Qwen is a small Qwen 3.5 model trained specifically for AuraGo tool-call shapes. It is an optional test and fallback runtime. It is **not** intended to replace a capable large local model or a cloud model.

The current Q4 evaluation reached about 94.0% schema-contract accuracy and 95.0% exact-case accuracy. The original 97% acceptance target was not reached. This limitation is shown in both Setup and Config so operators can make an informed choice.

## Practical hardware requirements

The v1 supported host is Linux AMD64 with Docker. For useful performance, use a discrete NVIDIA, Intel Arc, or Vulkan 1.2+ GPU with at least 8 GB VRAM.

A modern integrated GPU may work, but generation can be unsatisfactorily slow. Explicit CPU mode is also experimental and is never selected automatically. Integrated-GPU and CPU profiles require an acknowledgement bound to the detected hardware fingerprint.

## Configuration

```yaml
local_llm:
  enabled: false
  model_family: qwen
  backend: auto
  model_variant: q4_k_m
  mtp: off
  context_size: 16384
  idle_timeout_minutes: 15
  listen_port: 18081
```

The selectable context sizes are 16K and 32K. Existing 2K and 8K settings are
migrated to 16K because AuraGo's normal system prompt and native tool schemas
do not fit usefully into those smaller windows. Selecting 32K remains
fail-closed: startup must still pass the configured memory and GPU/KV-offload
attestation, with no silent reduction to 16K.

The provider ID `aurago-qwen-local` is reserved. Do not add it to `providers`. AuraGo resolves its model alias, private endpoint, and runtime credential through the manager.

Supported roles are:

- `test_only`: the regular provider remains primary and AuraGo-Qwen is reached only by explicit tests.
- `fallback`: the selected regular provider is primary and AuraGo-Qwen replaces the configured fallback.
- `primary`: AuraGo-Qwen is primary and exactly one regular provider remains its fallback.

Primary or fallback routing is activated only after health, native tool-call, memory-profile, and GPU/KV-offload checks succeed. A regular fallback is mandatory when AuraGo-Qwen is primary.

## Stable tools and RAM prefix cache

AuraGo-Qwen uses a deterministic core tool profile with at most 16 direct
tools and 4,096 tool-schema tokens. Other permitted tools remain available
through `discover_tools` followed by `invoke_tool`. Restricted runtimes such as
SIP, missions, Game Maker, and co-agents keep their narrower allowlists; the
indirect tool path never widens them.

The local transport asks llama.cpp to retain the identical rendered prefix in
RAM. This prefix contains only the model/template settings, canonical tool
schemas, and the static system prompt before `# TURN CONTEXT`. User messages,
history, current memories, RAG results, tool output, paths, and secrets are not
part of the reusable seed. Cache state is not written to disk. A restart
therefore needs one cold warm-up; an idle restart warms the retained in-process
seed again.

The prefix cache saves prompt-processing time only. It does **not** reduce the
number of tokens occupying the selected 16K or 32K context window. If template
rendering, warm-up, or cache verification fails, the request continues
uncached and the status reports a sanitized degraded or rejected state.

Qwen on AMD Vulkan devices uses the tested `vulkan-amd-fast-v1` profile. It enables the
RADV `nogttspill` optimization, full GPU offload, batch/uBatch 2048/512, eight
prompt/decode threads, flash attention, one slot, F16 KV caches, and a bounded
2,048 MiB prompt cache on integrated/shared-memory GPUs. These parameters are
attested through the container startup manifest and are not free-form YAML
settings. Other Vulkan devices never inherit the AMD-only RADV option.

## Installation and release gate

Downloads happen only after an explicit administrator action. Requests never trigger model downloads. Each GGUF is downloaded through a resumable `.part` file and is published only after exact size and SHA-256 validation.

The built-in release manifest is fail-closed. A releasable manifest must contain:

- a public, exact Hugging Face commit for every artifact;
- the recorded file size and SHA-256;
- a digest-pinned CUDA, SYCL, and Vulkan image;
- a successful native Linux GPU smoke-test marker for every selectable backend;
- llama.cpp commit `555881ebc8b0fc0402b30e09258a32a7bfd13c52`.

The public model artifacts are pinned to immutable Hugging Face commits:

- GGUF: `37e44d3534c05447be9e486cadca5d1da9838539`;
- MTP target and sidecar: `abf7f625cc52c019ef5a14afa0c56713d5183818`.

The CUDA, SYCL, and Vulkan GHCR packages and the model artifacts are public and digest-pinned. Candidate backends may be installed only after an explicit backend choice and experimental-hardware acknowledgement. Automatic backend selection remains unavailable until an image and matching hardware profile pass the required native Linux GPU smoke test. AuraGo does not accept Hugging Face or GHCR credentials for this feature.

### WSL2 development tests

WSL2 is not treated as a native Linux release qualification. Intel Arc GPUs can
nevertheless run the SYCL image experimentally through `/dev/dxg` when the
Windows WSL driver libraries are mounted read-only from `/usr/lib/wsl`. The
managed v1 runtime currently accepts native DRM render nodes only, so a
successful manual WSL2 test must not mark a backend `validated-linux` or make
it available to automatic backend selection.

## Runtime security

Native AuraGo publishes the sidecar only on `127.0.0.1`. Containerized AuraGo uses the private `aurago-app` network without a host port. The sidecar runs as `65532:65532` with a read-only root filesystem, no Linux capabilities, `no-new-privileges`, a PID limit, and a bounded `/tmp`.

For a native installation, `listen_port` must be unused on the loopback
interface. AuraGo checks the configured binding before container creation and
reports `listen_port_unavailable` instead of selecting another port silently.
Choose a free loopback port, save the configuration, and retry the requested
action.

The random API key is stored only as `local_llm_runtime_api_key` in the Vault. At runtime it is materialized as a mode-0600 file, mounted read-only, and removed on stop and AuraGo startup. It is never placed in YAML, container environment variables, command arguments, logs, or API responses.

The model and runtime-key volumes are protected from AuraGo's agent-facing Docker tool. That permission boundary is independent of the internal manager's Docker transport.

## MTP

`off` uses the normal GGUF and is the default. `mtp2` requires the exact same-quantization target/sidecar pair and fails visibly if that profile does not pass. `auto` compares the MTP target without and with its sidecar using one warm-up and three measured 8K tool-call runs. Prompt caching is disabled during this comparison so the target and MTP paths are measured fairly.

Automatic selection requires semantically identical tool calls, no OOM/offload error, at least 80% draft acceptance, at least 10% higher median generation performance, and no more than 25% median TTFT regression. The decision is cached by the full desired-state and hardware fingerprint.

## Administrator API

- `GET /api/local-llm/status`
- `POST /api/local-llm/probe`
- `POST /api/local-llm/install`
- `POST /api/local-llm/action`
- `POST /api/local-llm/role`
- `POST /api/local-llm/acknowledgement`

The `smoke_test` action starts an installed runtime on demand before sending the
exact tool-call probe. Experimental backends require the current
hardware-fingerprint acknowledgement for install, start, recreate, smoke test,
and benchmark operations.

All routes are same-origin protected and administrator-only. Status responses contain sanitized codes and resolved parameters, not raw logs, host paths, or credentials.
