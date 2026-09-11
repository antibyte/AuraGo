# Managed ACE-Step runtime

## Purpose and ownership

This package owns local music setup, hardware qualification, the private worker,
model/cache volumes, Vault authentication and serial generation.

## Local contracts

- `aurago-acestep-local` is a music-only provider. All callers use the shared
  music path, quantity limit and media registry; local music has zero cloud cost.
- Keep Docker resource and Python secret-export protections. Browser and agent
  responses never contain the runtime key. Cancel/timeout stops actual inference.
- `release.json` pins images, models and effective Python model code; upstream
  replaces Hugging Face Python copies during loading. Downloads verify size/hash.
- Restore the previous qualified container after failed/interrupted replacement.
  No silent CPU/cloud fallback and no profile change during generation.
- Vulkan is an additional Linux/amd64 backend, implemented by pinned
  `acestep.cpp`/GGML and separate GGUF model pins in `release.json.vulkan`.
  The standard-library adapter exposes only the existing authenticated private
  API; native port 8002 stays container-loopback. Select the real PCI/render node,
  verify GGML Vulkan matrix multiplication, and force that backend for inference.
  The build patch loads the selected models before native health and constrains
  LM audio-code generation to the requested duration at 5Hz. Qualification
  requires an actual ten-second MP3. Restart loads models but reuses the
  image/model/hardware-bound qualification. Never accept software Vulkan as a GPU.

## Verification

- `go test ./internal/acestep` and `runtime/test_runtime.py` cover the contracts.
- `python internal/acestep/runtime/test_vulkan_runtime.py` checks the native
  adapter without weights. Build it with `runtime/Dockerfile.vulkan` and validate
  real GPU audio separately; the CPU/PyTorch runtime is not a Vulkan substitute.
- Image builds and CPU tests do not establish GPU acceptance. Follow
  `documentation/ace-step-local.md` for hardware, UI and release checks.
