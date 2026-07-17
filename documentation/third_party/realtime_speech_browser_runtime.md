# Realtime Speech browser runtime attributions

AuraGo vendors the following pinned browser-side artifacts for local voice activity detection. They are embedded into the single AuraGo binary and do not load code or models from a CDN at runtime.

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
- Version: `1.27.0`
- License: MIT, Copyright (c) Microsoft Corporation
- Bundled license: `ui/js/realtime-speech/vendor/LICENSE-ONNXRUNTIME.txt`

| Artifact | SHA-256 |
|---|---|
| `ort.wasm.min.js` | `ea3a767b15df7dbe3d695ec9c182ca0f15b2ce7750156c6b70276e11c28997f0` |
| `ort-wasm-simd-threaded.mjs` | `0a1e718d99c41b22c21f2520ff4f9e883a6b5533856e398d21816ee8eb8185d3` |
| `ort-wasm-simd-threaded.wasm` | `d1ab1b94b16a65b29d710d0b587b29e7bed336827577623913479b8afe8113e6` |

Run `npm run build:realtime-speech-vendor -- --check` to verify every vendored checksum without modifying the files.
