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

## Humanizer writing guidance

The Writer specialist's compact additional prompt in `internal/config/config.go`
and `config_template.yaml` adapts concepts from
[Humanizer 3.0.0](https://github.com/blader/humanizer/blob/main/SKILL.md),
with AuraGo-specific multilingual, factual-integrity, and output-format guidance.
The upstream license is reproduced below.

MIT License

Copyright (c) 2025 Siqi Chen

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Detective PDF fonts

Detective embeds Go Regular and Go Bold from `golang.org/x/image v0.45.0`
(`font/gofont`), created by Bigelow & Holmes. The font license follows.
The existing `github.com/phpdave11/gofpdf v1.4.3` dependency (MIT) renders PDFs.

```text
Copyright (c) 2016 Bigelow & Holmes Inc.. All rights reserved.

Distribution of this font is governed by the following license. If you do not
agree to this license, including the disclaimer, do not distribute or modify
this font.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

	* Redistributions of source code must retain the above copyright notice,
	  this list of conditions and the following disclaimer.

	* Redistributions in binary form must reproduce the above copyright notice,
	  this list of conditions and the following disclaimer in the documentation
	  and/or other materials provided with the distribution.

	* Neither the name of Google Inc. nor the names of its contributors may be
	  used to endorse or promote products derived from this software without
	  specific prior written permission.

DISCLAIMER: THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO,
THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

```
