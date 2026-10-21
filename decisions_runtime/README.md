# AuraGo Decisions native runtime

This small C++ service computes next-token logits for the pinned Jeff 2B GGUF model. Go owns prompt construction, option normalization, probability calculation, policy, and lifecycle. The native service does not sample and does not return generated confidence values.

## Immutable dependencies and artifacts

- llama.cpp source: `ggml-org/llama.cpp` commit `d81235049384534c167caea52b85a694f6103d14`, with the exact local safety patch in `patches/strict-offload-preflight.patch` (SHA-256 `a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790`). The build refuses any unrecognized local edits to the pinned source tree.
- Jeff model and projector: Hugging Face repository `jersobh/jeff-2b-gguf`, revision `263ac27c59688a360d2db255c4f7e57f665bcca4`.
- Model `jeff-2B.Q4_K_M.gguf`: SHA-256 `d4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b`, size `1274396448`.
- Projector `jeff-2B.mmproj.gguf`: SHA-256 `1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68`, size `671372608`.

Weights stay in the app-managed model store and are not included in source packages or images. A locally built runtime bundle contains `aurago-decisions-runtime[.exe]` and `release-manifest.json`; the manifest hashes the produced binary and includes the llama.cpp commit and safety-patch hash plus the model/projector source pins. Published v2 packages also inventory every redistributable library and license notice. The compiled application catalog pins their archives, manifests and GHCR image digests; local developer images do not substitute for published digest pins.

End users use **Set up Decisions** in Configuration. The commands below are for
runtime development and release builders. See [runtime publication](../documentation/decisions-runtime-release.md)
for the six-platform build matrix, anonymous verification and the required
generated catalog. Automatic installation remains unavailable until that release
gate has been completed.

## Build locally

Select the hardware backend explicitly when its SDK/toolchain is available: Windows/Linux Vulkan with `scripts/build-decisions-runtime.ps1 -Backend vulkan` or `scripts/build-decisions-runtime.sh --backend vulkan`; NVIDIA CUDA with `-Backend cuda` / `--backend cuda`; and Apple Metal with `-Backend metal` / `--backend metal`. CPU is an explicit fallback build (`-Backend cpu` / `--backend cpu`) and does not authorize CPU inference: the manager still requires saved CPU-fallback consent before starting it. Every build uses the pinned llama.cpp commit and maintained scheduler preflight patch. The script runs `--self-test-protocol`, verifies `--discover`, and writes the release manifest next to the executable.

The local `--self-test-protocol` first runs a synthetic two-input ggml graph: the scheduler preflight denial must abort before changing a sentinel output, and the allowed path must compute the expected sum. It then attaches the production routes to a loopback server and checks bearer rejection before body transfer, health and capabilities, malformed and deeply nested JSON, request-ID echo, 40 MiB score and 4 KiB cancel limits (including chunked cancellation), and active/early cancellation. Neither test loads model weights. Production startup always initializes and validates the pinned models before serving. `--discover [--model PATH]` reports real llama.cpp backend devices; with a model path it parses layer metadata without allocating model weights or running inference.

Windows ARM64 developer builds require Visual Studio's Clang tools (`ClangCL`).
The build script selects them for a fresh ARM64 build so packages do not inherit
the build machine's optional instruction set.

## Launch modes

The manager passes immutable model/projector paths, revisions and SHA-256 values, an exact enumerated device id, loopback bind address, port, and a private token file.

- Strict GPU: `--device DEVICE_ID`; the service requests all layers on that device, forces all model weight buffers to its buffer type, and refuses before warmup/inference if graph preflight assigns any real compute operation to another or unknown device. Pure view/reshape/permute aliases and host input staging are not model compute.
- Consented hybrid: `--device DEVICE_ID --allow-cpu-offload --gpu-layers N`. The manager owns the bounded retry ladder and may use this only after saved CPU-fallback consent. The runtime reports actual CPU and device graph work.
- Consented CPU: `--allow-cpu` with a CPU backend build. The manager may start it only after saved consent.

There is no native fallback between modes. Failed strict startup exits; the manager chooses whether an explicitly consented retry is allowed. Input context is capped at 8192 tokens. `/score` accepts the manager’s remaining `max_input_tokens`, counts text and image tokens without truncation, and rejects before inference if the request exceeds that budget. Requests allow at most 8 images, 12 MiB each, 24 MiB total, and 32 Mi pixels per decoded image. The native decoder accepts JPEG, PNG, GIF, and BMP; AuraGo normalizes WebP to PNG before this boundary.

## HTTP protocol v1

Before opening the HTTP port, the runtime verifies both pinned files, loads the
model and projector, and performs the device-attestation warmup. The manager
allows five minutes for this startup, with individually bounded readiness
probes. A timeout stops the attempt without trying another device or CPU mode;
explicit device failures still follow the manager's consent policy.

Startup writes constant, newline-terminated diagnostics to stderr:
`AURAGO_DECISIONS_STARTUP phase=<phase>` for `verifying_model`,
`verifying_projector`, `loading_model`, `loading_projector`, and `warming_up`.
Failures use `AURAGO_DECISIONS_STARTUP_ERROR code=<stable-code>`. Native and Docker
managers accept only bounded allowlisted lines, never arbitrary log text.
Older verified bundles without phase messages remain usable but cannot display
these phases; rebuild the bundle/image to include them. Neither hashes nor
device attestation are skipped to shorten startup.

All routes require `Authorization: Bearer <private token>` before reading request bodies and return JSON. Score bodies are limited to 40 MiB, cancellation bodies to 4 KiB including chunked transfers, and JSON to 32 container levels. Failed authorization is HTTP 401 `{"error":{"code":"unauthorized"}}`.

- `GET /healthz` and `GET /capabilities`: protocol, readiness, exact model/projector pins, runtime build hash, model layer count, selected device, backend assignment and buffer attestations, image marker, and limits. Graph preflight proves where real model/projector operators would execute before the graph runs; model/projector weight-buffer and KV maps are checked separately. Reported projector compute-buffer byte counts describe allocated scratch capacity and are diagnostic only, not CPU/GPU execution evidence.
- `POST /score`: JSON `{ "request_id": "...", "prompt": "...", "candidates": [{"code":"public-option-id","token_text":"A"}], "images": [{"mime_type":"image/png","data_base64":"..."}], "max_input_tokens": 8192 }`. Candidate token text must be exactly one of `A..Z`, `a..z`, or `0..9`, and tokenize to exactly one suffix token after the prompt. The response echoes `request_id`, includes per-candidate `{code, token_id, logit}`, the same health/capability metadata, and `usage:{prompt_tokens,image_tokens,context_tokens}` where context is their sum.
- `POST /cancel`: JSON `{ "request_id": "..." }`. Active requests are marked cancelled; an early cancellation creates a bounded 30-second tombstone so request arrival order does not lose it. The idempotent response is HTTP 200 `{ "request_id":"...", "cancelled":true, "error":null }`. llama.cpp’s pinned CPU and Metal backends accept an abort callback; CUDA/Vulkan kernels do not expose a preemption callback at this pin, so an already-running kernel can finish before the graph/chunk cancellation checkpoint. Queued work and later chunks are stopped.

Request IDs contain 16–128 ASCII letters, digits, `-`, or `_`. Stable score errors include `invalid_request`, `input_token_budget_exceeded`, `candidate_not_single_token`, `request_cancelled`, `runtime_not_ready`, `runtime_busy` (HTTP 503 while inference is occupied), and `device_attestation_changed`. Score errors echo a valid `request_id` whenever it could be extracted.

## Local Docker source build

`Dockerfile` builds from a caller-provided, locally present base image referenced by immutable `name@sha256:<64 lowercase hex>` digest. The base image must contain a compiler toolchain, CMake, Git, Python, and the SDK/libraries for the requested backend. `scripts/build-decisions-runtime-docker.ps1` checks for the exact base image locally and never requests `docker pull`; it creates the development tag `aurago-decisions-runtime:<backend>` and labels it with protocol, source pin, patch hash, and backend. Metal is a host/macOS backend and is not supported by this Linux container path. The development tag and image ID are local build evidence, not release pins.

The native bundle disables OpenMP and uses static MSVC CRT linkage on Windows so the CPU executable does not silently depend on optional compiler/OpenMP DLLs. ggml statically links CUDA runtime support; Linux uses static cuBLAS where available, while Windows still requires CUDA cuBLAS DLLs because the toolkit does not provide static cuBLAS there. All CUDA builds require a matching NVIDIA driver. Vulkan uses the system Vulkan loader and compatible driver; Metal uses the operating system framework. The local manifest names these required system components.
