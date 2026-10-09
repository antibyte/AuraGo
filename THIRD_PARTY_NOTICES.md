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
| klauspost/compress v1.20.1 (`zstd`) | Zstandard decompression of Local Wikipedia ZIM clusters | BSD 3-Clause License (`zstd/internal/xxhash`: MIT License) |
| ulikunitz/xz v0.5.17 | XZ (LZMA2) decompression of older Local Wikipedia ZIM clusters | BSD 3-Clause License |
| blevesearch/snowballstem v0.9.0 | Snowball word stemming that matches the Xapian full-text index inside Local Wikipedia ZIM files | BSD 3-Clause License |

The upstream projects and their complete license texts remain authoritative:

- https://huggingface.co/ibm-granite/granite-embedding-97m-multilingual-r2
- https://github.com/microsoft/onnxruntime
- https://github.com/ggml-org/llama.cpp
- https://github.com/emiago/diago
- https://github.com/emiago/sipgo
- https://github.com/hajimehoshi/go-mp3
- https://github.com/ebitengine/purego
- https://github.com/dlclark/regexp2
- https://github.com/klauspost/compress
- https://github.com/ulikunitz/xz
- https://github.com/blevesearch/snowballstem

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

## Local Wikipedia content and formats

The Local Wikipedia integration downloads one Wikipedia edition packaged by
[Kiwix/openZIM](https://kiwix.org) as a ZIM file when an administrator clicks
**Install**. AuraGo does not ship a Wikipedia edition in its source tree, binary
or web resources.

- Wikipedia text is licensed under the
  [Creative Commons Attribution-ShareAlike 4.0 License](https://creativecommons.org/licenses/by-sa/4.0/)
  (CC BY-SA 4.0; older revisions also under the GFDL). Images and other media in
  "with media" editions carry their own licenses, listed on their Wikimedia
  Commons pages. Reusing text from a local edition requires attribution and the
  same license.
- Wikipedia is a trademark of the Wikimedia Foundation. AuraGo is not affiliated
  with or endorsed by the Wikimedia Foundation or Kiwix.
- AuraGo's ZIM and Xapian index readers are original MIT-licensed code. The
  GPL-licensed libzim and Xapian sources served only as format references; no
  code was copied.
- The test archives under `internal/zim/testdata/` and the golden files under
  `internal/zim/xapian/testdata/` are generated by `scripts/localwiki/fixtures/`
  from self-authored text and contain no Wikipedia articles (only the
  `real_fixture_libzim.json` golden records article paths and hit counts of the
  real archive below). The optional real-archive test downloads
  `wikipedia_en_climate_change_mini_2024-06.zim` (CC BY-SA 4.0) into the user
  cache only when `AURAGO_ZIM_REAL_FIXTURE=1`; it is never committed.
- The only Wikipedia text committed to the source tree are the two trimmed
  article pages `render_en_okjokull.html` (English Wikipedia, "Okjökull") and
  `render_de_bielefeld.html` (German Wikipedia, "Bielefeld") in
  `internal/localwiki/testdata/`, with their Markdown results. They are inputs
  of the article renderer tests (text CC BY-SA 4.0, Wikipedia contributors; the
  revisions and the author lists are named in the file headers and on the
  Wikipedia history pages). They are test data only and are never compiled into
  the binary or shipped with a release.
