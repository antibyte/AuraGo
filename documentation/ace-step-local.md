# Local ACE-Step music generation

Select **ACE-Step 1.5 — local** under Music Generation, keep Docker enabled with
write access, and save. AuraGo pulls a pinned runtime image, probes the Docker
host, downloads only the selected models, loads them, and generates a short
qualification clip. The service stays available until disabled or stopped.
Stopping preserves model and cache volumes. Start resumes it; Recheck performs
hardware/profile qualification again. Connection Test only checks authentication
and loaded-model readiness.

## Hardware

Linux/amd64 Docker engines support CUDA, ROCm and Intel XPU when the actual
container probe can execute on the device. Windows Docker Desktop/WSL2 supports
NVIDIA CUDA. Apple GPU acceleration, AMD/Intel on Windows, audio editing and
training are outside this integration. CPU requires explicit selection; there is
no automatic CPU or cloud fallback. Drivers and Docker device support must
already be installed on the Docker host.

Automatic selection prefers the verified device with the largest free memory
budget. The default VRAM reserve is 1 GiB and can be changed, including to zero.
The balanced profile uses 2B Turbo, considering XL Turbo at 20 GiB usable VRAM.
The pinned ACE-Step adaptive profile controls language-model size, offloading,
quantization and duration limits. AMD and Intel use the PyTorch LM backend.
A failed memory qualification permits one smaller profile attempt. Other
containers are never stopped to reclaim memory.

## Generation

Agent `generate_music`, Noisemaker and Game Maker share the provider path, media
registry and daily quantity limit. Local music has zero provider cost and is
exempt from the cloud music budget. Noisemaker's optional cover generation
continues to use the separately configured image provider.

Duration defaults to 120 seconds and is limited by the active profile, never
above 600 seconds. BPM is 30–300. Vocal language accepts codes such as `de`, `en`
and `ja`. An omitted seed is random; zero is a valid deterministic seed. Without
a local language model, provide lyrics or select instrumental. Local mode never
silently invokes the chat/cloud LLM to write lyrics.

One request owns the worker at a time; others receive `acestep_busy`. The native
API is polled every two seconds. The default deadline is 1800 seconds. Canceling
or timing out stops the dedicated container to terminate actual inference,
then prepares it again. Changing the profile waits for a running generation;
disabling or explicitly stopping cancels it.

## Storage and access

- Runtime: `aurago-acestep`, with temporary owned probe/rollback containers.
- Volumes: `aurago_acestep_models`, `aurago_acestep_cache`.
- Runtime API: loopback port 18083 for native AuraGo, otherwise private
  `aurago-app` Docker networking on port 8001. The container has no Docker socket.
- The independent `acestep_runtime_key` lives in the encrypted Vault. It is never
  returned to browsers or agents. Generic agent Docker access to these resources
  and Python secret export are blocked.
- Successful MP3 files move atomically into `data/audio` and enter the existing
  media registry once. Audio redirects and foreign file/download paths fail closed.
- Model files use pinned revisions, sizes and SHA-256 hashes. Incomplete files
  remain `.part`; complete cached models work without network access.

Admin routes use `/api/music-generation/local/status` (GET), `/probe` (POST),
and `/action` (POST with `start`, `stop` or `recheck`). Mutations obey the normal
admin, CSRF and Docker write gates. Noisemaker only uses Desktop endpoints.
Hardware Probe checks devices without loading models or running an audio sample;
it preserves a manually stopped runtime. Disable local music and wait for it to
stop before switching Docker to read-only or disabling Docker access.

## Building and qualifying releases

`internal/acestep/release.json` pins upstream commit
`ca1e85fe9430179831e6bc6be790c332190a3866`, model revisions and file checksums.
`python scripts/generate-acestep-manifest.py --write` refreshes model metadata;
review the resulting pins before publishing. The default invocation checks them.

The `ACE-Step runtime images` workflow builds CPU, CUDA/cu128, ROCm/7.1 and XPU
for Linux/amd64, runs the offline adapter contract tests, and publishes signed
images with provenance and SBOMs. Download its `acestep-release-manifest`
artifact, verify anonymous access to every digest, and copy its content into the
embedded release manifest before packaging AuraGo. An absent digest blocks setup;
there is no unpinned-image or local-build fallback. The Docker proxy retains
`BUILD=0`.

Run focused Go tests for ACE-Step, provider switching, Noisemaker and Docker
protection. Set `AURAGO_RUN_BROWSER_SMOKE=1` for `go test ./ui -run TestLocalMusic`.
Run `node scripts/build-ui-bundles.js --check`. Package the versioned web assets
and matching binary using the normal assetpack workflow.

An image build is not hardware acceptance. Each release still needs actual
instrumental/vocal generation, seed-zero, timeout and cancellation checks on
NVIDIA, AMD, Intel and Windows/NVIDIA. Record driver, selected profile, image
digest and model fingerprint for those checks. Never report GPU acceptance from
the CPU or mocked profile tests.

Upstream references: [API](https://github.com/ace-step/ACE-Step-1.5/blob/ca1e85fe9430179831e6bc6be790c332190a3866/docs/en/API.md),
[GPU profiles](https://github.com/ace-step/ACE-Step-1.5/blob/ca1e85fe9430179831e6bc6be790c332190a3866/docs/en/GPU_COMPATIBILITY.md),
[Docker Desktop GPU support](https://docs.docker.com/desktop/features/gpu/).
