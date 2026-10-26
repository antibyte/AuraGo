# Decisions runtime contributor notes

- Keep the runtime native and small. Its only inference dependency is the pinned llama.cpp/mtmd source in `CMakeLists.txt` plus the maintained patch under `patches/`.
- Never download, commit, or bundle Jeff model weights here. The manager validates the exact Hugging Face model and projector pins before launch.
- Do not claim CPU/GPU placement from launch flags. Attestation must come from allocated GGML graph backend assignments before compute, actual model/projector weight buffers, and KV buffers. Projector scratch-buffer byte counts are diagnostic capacity only; projector compute placement comes from the scheduler preflight assignments, not those allocation sizes.
- Strict GPU mode must reject graph assignments before execution. CPU and hybrid execution are available only through explicit manager consent flags.
- Keep `/score`, `/cancel`, `/healthz`, `/capabilities`, and `--discover` compatible with `README.md`. Do not log prompts, images, bearer tokens, or request bodies.
- Authenticate before reading bodies. Keep score/cancel limits at 40 MiB/4 KiB,
  including chunked transfers, and parse JSON once with at most 32 container levels.
- The protocol self-test attaches production routes to an uninitialized Runtime;
  production main always validates and initializes models before serving.
- Run the real loopback protocol self-test and native build before reporting a runtime build as verified. A CPU build does not prove image/model inference quality or GPU support.
