# Realtime Speech browser runtime attributions

AuraGo vendors the following pinned browser-side artifacts for local voice activity detection. They are shipped in the verified external Web UI resource set and do not load code or models from a CDN at runtime.

## Silero VAD

- Project: [Silero VAD](https://github.com/snakers4/silero-vad)
- Version: `v6.2.1`
- Artifact: `ui/js/realtime-speech/vendor/silero_vad_v6.2.1.onnx`
- SHA-256: `1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3`
- License: MIT, Copyright (c) 2020-present Silero Team
- Bundled license: `ui/js/realtime-speech/vendor/LICENSE-SILERO.txt`

## ONNX Runtime Web

- Project: [Microsoft ONNX Runtime](https://github.com/microsoft/onnxruntime)
- npm package: `onnxruntime-web`
- Version: `1.30.0`
- License: MIT, Copyright (c) Microsoft Corporation
- Bundled license: `ui/js/realtime-speech/vendor/LICENSE-ONNXRUNTIME.txt`

| Artifact | SHA-256 |
|---|---|
| `ort.wasm.min.js` | `faece07e84faa9001f5104bbe4e5c70861d06cb2376a0dea1fa0ad33a67adc81` |
| `ort-wasm-simd-threaded.mjs` | `e13f7f94fc51b4ca72b12faeb1ee95f4ace6dfbc8939bc718aabdc0a27c4299b` |
| `ort-wasm-simd-threaded.wasm` | `3398c10d07d229bd91b364548e130e0e51a8e5704b88c7c083ebbeb78842dee2` |

Run `npm run build:realtime-speech-vendor -- --check` to verify every vendored checksum without modifying the files.
