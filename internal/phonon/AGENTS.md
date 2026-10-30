# Phonon local speech

## Purpose
Own the pinned native CPU runtime, immutable multilingual checkpoint, voice catalog,
verified optional voice downloads and cancellable WAV runner.

## Ownership
`internal/tools` owns cached/transient dispatch and the sanoTTS fallback. Config
owns language/provider/manual voice settings. Server owns protected voice routes;
UI owns the state-backed picker and explicit download action.

## Local Contracts
- Keep `manifest.json`, `scripts/prepare-phonon.py`, licensing and `documentation/phonon.md` aligned.
- `--from-release latest` uses the manifest-pinned model package URL, size and
  SHA-256, not the latest AuraGo binary release. Explicit tags retain release
  checksum verification; both paths verify the individual model/voice files.
- Never bundle an HF token or download gated weights on a user's first utterance.
  Release builders require the prepared model and ten bundled voice embeddings.
- Empty manual voice means a deterministic language/persona default. Unknown,
  neutral and custom personas default to a female voice. Reject unsupported
  languages instead of silently converting them to English before provider selection.
- Run only the checksum-pinned local CLI/model/voice paths, with filtered environment,
  option-safe text, cancellation, bounded WAV output and private temporary cleanup.
- Voice downloads accept catalog IDs only, pinned revisions/hashes and bounded
  responses. Verify before atomic publication. Reads never provision resources.
- Version bundled resources beside the executable; additional voices remain in
  the persistent data directory. Do not place immutable assets under a Docker data mount.
- Runtime/voice licenses and the model's noncommercial restriction travel with the bundle.

## Work Guidance
Use the standard-library preparation helper. Do not add Python or heavy ML runtime
requirements to Phonon inference; the packaging helper is maintainer-only.
Keep prepared `deploy/phonon` inputs through release cleanup; clean worktrees must
receive these ignored inputs before running a release builder.
The native release workflow prepares all runtime targets and the model with
`--from-release latest` before invoking the existing Windows builder.
The AuraGo Docker job in Security Gates prepares the manifest-verified
`linux_amd64` resources with `--from-release latest` before building the image;
the Dockerfile still performs its own offline `--check`.

## Verification
`go test ./internal/phonon` covers catalog defaults, download integrity/cancellation
and the native runner protocol. `AURAGO_PHONON_SMOKE_DIR` enables real ten-voice
offline CPU synthesis. Also run focused TTS config/tools/agent/server/browser tests,
the preparation helper checks and relevant installer/release/Docker contracts.
`TestPhononCPUSmokePaths` verifies cached/transient speech and telephone PCM decoding
for all ten bundled voices; run it in the runtime image with networking disabled.

## Child DOX Index
None.
