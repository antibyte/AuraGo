# sanoTTS runtime

## Purpose and Ownership
- Own the shared CPU/WAV runner, supported voice selection, and the bundled upstream
  wheel and license. `internal/tools/sanotts.go` owns provisioning and serialization;
  CYD owns its short English notifications and 8 kHz conversion.

## Local Contracts
- Keep wheel, revision, SHA-256 test and `documentation/sanotts.md` synchronized.
- Rebuild with `python scripts/build-sanotts-runtime.py`; temporary sources stay in
  `disposable/`. Retain the upstream MIT license and the wheel's G2P notices.
- Prefer the smallest published Python voice. Only actual Python packages count as
  supported; browser-only formats fall back to English. Do not claim translation.
- Keep cancellation, private temporary WAV cleanup, bounded output, filtered child
  environment and option-safe text arguments. No GPU, Docker or heavy ML runtime.

## Verification
- `go test ./internal/sanotts ./internal/cyd` checks packaged voices and PCM handling.
- `AURAGO_SANOTTS_SMOKE_DIR` opts into `TestSanoTTSCPUSmoke` in `internal/tools`.

## Child DOX Index
- None.
