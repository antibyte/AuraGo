# AuraGo-Qwen managed local test model

AuraGo-Qwen is a small Qwen 3.5 model trained specifically for AuraGo tool-call shapes. It is an optional test and fallback runtime. It is **not** intended to replace a capable large local model or a cloud model.

The current Q4 evaluation reached about 94.0% schema-contract accuracy and 95.0% exact-case accuracy. The original 97% acceptance target was not reached. This limitation is shown in both Setup and Config so operators can make an informed choice.

## Practical hardware requirements

The v1 supported host is Linux AMD64 with Docker. For useful performance, use a discrete NVIDIA, Intel Arc, or Vulkan 1.2+ GPU with at least 8 GB VRAM.

A modern integrated GPU may work, but generation can be unsatisfactorily slow. Explicit CPU mode is also experimental and is never selected automatically. Integrated-GPU and CPU profiles require an acknowledgement bound to the detected hardware fingerprint.

## Configuration

```yaml
local_llm:
  enabled: false
  backend: auto
  model_variant: q4_k_m
  mtp: off
  context_size: 8192
  idle_timeout_minutes: 15
  listen_port: 18081
```

The provider ID `aurago-qwen-local` is reserved. Do not add it to `providers`. AuraGo resolves its model alias, private endpoint, and runtime credential through the manager.

Supported roles are:

- `test_only`: the regular provider remains primary and AuraGo-Qwen is reached only by explicit tests.
- `fallback`: the selected regular provider is primary and AuraGo-Qwen replaces the configured fallback.
- `primary`: AuraGo-Qwen is primary and exactly one regular provider remains its fallback.

Primary or fallback routing is activated only after health, native tool-call, memory-profile, and GPU/KV-offload checks succeed. A regular fallback is mandatory when AuraGo-Qwen is primary.

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

`off` uses the normal GGUF and is the default. `mtp2` requires the exact same-quantization target/sidecar pair and fails visibly if that profile does not pass. `auto` compares the MTP target without and with its sidecar using one warm-up and three measured 2K tool-call runs.

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
