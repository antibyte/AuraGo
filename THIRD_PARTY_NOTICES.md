# Third-party notices

AuraGo links the listed Go modules or retrieves the listed optional model/runtime artifacts. When `embeddings.provider` is `local-granite`, native runtimes and model weights are downloaded on demand and verified by exact size and SHA-256 rather than embedded in the primary binary.

| Component | Use | License |
|---|---|---|
| IBM Granite Embedding 97M Multilingual R2 | Base embedding model, ONNX conversion, and GGUF quantization | Apache License 2.0 |
| ONNX Runtime 1.26.0 | Native ONNX inference runtime | MIT License |
| llama.cpp b9994 | GGUF embedding server and published Docker sidecars | MIT License |
| Ebitengine PureGo | CGO-free dynamic calls from Go into ONNX Runtime | Apache License 2.0 |
| dlclark/regexp2 | Go tokenizer regular-expression compatibility | MIT License |
| emiago/diago v0.31.0 | Native SIP endpoint and RTP media handling | Mozilla Public License 2.0 |
| emiago/sipgo v1.4.3 | SIP transport and message processing | BSD 2-Clause License |
| hajimehoshi/go-mp3 v0.3.4 | Pure-Go MP3 decoding for telephone TTS audio | Apache License 2.0 |

The upstream projects and their complete license texts remain authoritative:

- https://huggingface.co/ibm-granite/granite-embedding-97m-multilingual-r2
- https://github.com/microsoft/onnxruntime
- https://github.com/ggml-org/llama.cpp
- https://github.com/emiago/diago
- https://github.com/emiago/sipgo
- https://github.com/hajimehoshi/go-mp3
- https://github.com/ebitengine/purego
- https://github.com/dlclark/regexp2

AuraGo's own license remains the MIT License in [LICENSE](LICENSE).
