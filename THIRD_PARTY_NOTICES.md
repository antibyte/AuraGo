# Third-party notices for local embeddings

AuraGo does not embed the following native runtimes or model weights in its source tree or primary binary. When `embeddings.provider` is `local-granite`, it downloads pinned artifacts on demand and verifies their exact size and SHA-256 before use.

| Component | Use | License |
|---|---|---|
| IBM Granite Embedding 97M Multilingual R2 | Base embedding model, ONNX conversion, and GGUF quantization | Apache License 2.0 |
| ONNX Runtime 1.26.0 | Native ONNX inference runtime | MIT License |
| llama.cpp b9994 | GGUF embedding server and published Docker sidecars | MIT License |
| Ebitengine PureGo | CGO-free dynamic calls from Go into ONNX Runtime | Apache License 2.0 |
| dlclark/regexp2 | Go tokenizer regular-expression compatibility | MIT License |

The upstream projects and their complete license texts remain authoritative:

- https://huggingface.co/ibm-granite/granite-embedding-97m-multilingual-r2
- https://github.com/microsoft/onnxruntime
- https://github.com/ggml-org/llama.cpp
- https://github.com/ebitengine/purego
- https://github.com/dlclark/regexp2

AuraGo's own license remains the MIT License in [LICENSE](LICENSE).
