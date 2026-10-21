# Third-party notices

## Optional Indra OCR service

Indra OCR 1.1.2 is Apache-2.0 code by Datalab, pinned to
[`0e96b93c6810f17cf8352acbb4487cc9e45b66d5`](https://github.com/datalab-to/indra/tree/0e96b93c6810f17cf8352acbb4487cc9e45b66d5).
Its default models (without the optional math model) are the unmodified v1.1.0
snapshot [`bd02167fdf2005a19962cc15188f8496a2c87d3b`](https://huggingface.co/datalab-to/indra-ocr/tree/bd02167fdf2005a19962cc15188f8496a2c87d3b).
They carry separate **AI Pubs OpenRAIL-M License (Modified)** terms, including
use/redistribution restrictions, attribution and share-alike provisions for
models and outputs. The complete upstream text is retained verbatim in
[`licences/texts/Indra-OCR-models.txt`](licences/texts/Indra-OCR-models.txt).

AuraGo's adapter remains MIT. The optional image includes LICENSE-CODE.txt
(Apache-2.0), LICENSE-MODELS.txt, LICENSE-AURAGO.txt and NOTICE.txt; Python
distribution licenses remain with the installed packages. JSON and ZIP exports
include the model terms and attribution; text/Markdown exports include attribution
and the pinned license link. Model sizes/hashes and Python package hashes are in
`internal/ocr/runtime/models.json` and `requirements.txt`; immutable image
digests are generated in `internal/ocr/release.go`. See `documentation/ocr.md`.

## Native code graph

`github.com/odvcencio/gotreesitter v0.56.0` is MIT licensed,
Copyright (c) 2026 Oscar Villavicencio. AuraGo links only its Go, JavaScript,
TypeScript/TSX and Python grammar packages. The upstream grammar license audit
in that exact module records these MIT works:

- tree-sitter-go, `2346a3ab1bb3857b48b29d779a1ef9799a248cd7`:
  Copyright (c) 2014 Max Brunsfeld.
- tree-sitter-javascript, `58404d8cf191d69f2674a8fd507bd5776f46cb11`:
  Copyright (c) 2014 Max Brunsfeld.
- tree-sitter-typescript (including TSX), `75b3874edb2dc714fb1fd77a32013d0f8699989f`:
  Copyright (c) 2017 Max Brunsfeld.
- tree-sitter-python, `26855eabccb19c6abf499fbc5b8dc7cc9ab8bc64`:
  Copyright (c) 2016 Max Brunsfeld.

The complete MIT terms are retained in `licences/texts/MIT.txt`.
Source and pinned grammar provenance:
[gotreesitter v0.56.0](https://github.com/odvcencio/gotreesitter/tree/v0.56.0),
[`licenses/grammars.json`](https://github.com/odvcencio/gotreesitter/blob/v0.56.0/licenses/grammars.json).


AuraGo links the listed Go modules or retrieves the listed optional model/runtime artifacts. When `embeddings.provider` is `local-granite`, native runtimes and model weights are downloaded on demand and verified by exact size and SHA-256 rather than embedded in the primary binary. The person-detection mode of the Network Cameras app downloads its detector model and ONNX Runtime the same way (see "Person detection models and runtime" below).

| Component | Use | License |
|---|---|---|
| IBM Granite Embedding 97M Multilingual R2 | Base embedding model, ONNX conversion, and GGUF quantization | Apache License 2.0 |
| ONNX Runtime 1.26.0 | Native ONNX inference runtime | MIT License |
| ONNX Runtime WebGPU 1.27.0 (`onnxruntime-webgpu` wheel from PyPI) | GPU/iGPU inference runtime of the person detector (linux/amd64, windows/amd64, darwin/arm64); only its runtime libraries are extracted | MIT License (the wheel also lists Dawn: BSD 3-Clause License, and LLVM-derived code: University of Illinois/NCSA Open Source License) |
| DirectX Shader Compiler `dxcompiler.dll` 1.9.0.1 and `dxil.dll` 1.8.2502.11 (inside the Windows WebGPU wheel) | Shader compilation of the WebGPU provider on Windows (Direct3D 12) | University of Illinois/NCSA Open Source License (upstream project); Microsoft-signed binaries, see "Person detection models and runtime" |
| D-FINE-N, ONNX export `AnnotateIt/dfine-nano-coco-onnx` of `ustc-community/dfine-nano-coco` (revision 571d3971bfb4) | Person detector, "standard" model | Apache License 2.0 |
| D-FINE-S, ONNX conversion `onnx-community/dfine_s_coco-ONNX` (revision a3cf03147a9b) of `ustc-community/dfine-small-coco` | Person detector, "accurate" model | Apache License 2.0 (the original weights; the conversion repository itself declares no license) |
| "Pedestrians using crosswalk (Unsplash)" by Peter Miranda | Embedded self-test photo of the person detector (`internal/persondetect/selftest_reference.jpg`) | CC0 1.0 Universal |
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
- https://pypi.org/project/onnxruntime-webgpu/
- https://github.com/microsoft/DirectXShaderCompiler
- https://dawn.googlesource.com/dawn
- https://huggingface.co/AnnotateIt/dfine-nano-coco-onnx
- https://huggingface.co/ustc-community/dfine-nano-coco
- https://huggingface.co/onnx-community/dfine_s_coco-ONNX
- https://huggingface.co/ustc-community/dfine-small-coco
- https://arxiv.org/abs/2410.13842
- https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg
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

## CYD Creator builder image

CYD Creator compiles firmware in the separately published image
`ghcr.io/antibyte/aurago-cyd-builder`, which AuraGo pulls on demand; it is not
part of the AuraGo binary. Besides MIT and other permissive components, the
image ships GPL and LGPL software (Espressif's Xtensa GCC toolchains, esptool,
the Arduino-ESP32 core, Adafruit NeoPixel, Debian 12 packages) and MPL-2.0
certifi.

- The image carries `/usr/share/doc/aurago-cyd-builder/NOTICE`, generated from
  [`deploy/docker/cyd-builder/NOTICE`](deploy/docker/cyd-builder/NOTICE). It
  lists each GPL, LGPL and MPL component with its exact version, where its
  licence text is in the image, and its upstream source. It also lists every
  installed Debian package with its source package version, and says how to
  request the corresponding source.
- All components of the image and their licences:
  [THIRD_PARTY_LIBRARIES.md, section 8](THIRD_PARTY_LIBRARIES.md#8-on-demand--optional-runtimes).

## Person detection models and runtime

The person-detection mode of the Network Cameras app (`internal/persondetect`,
`internal/onnxrt`) runs a D-FINE object detector in an isolated worker process.
AuraGo does not ship the models or the ONNX Runtime libraries in its source tree,
binary or web resources. They are downloaded on demand from the HTTPS URLs pinned
in `internal/persondetect/model.go` and `internal/onnxrt/runtimes.go` and are
verified by exact size and SHA-256 before use.

- **Models.** D-FINE (arXiv 2410.13842) is Apache License 2.0 software. The
  "standard" model is the ONNX export `AnnotateIt/dfine-nano-coco-onnx` (pinned
  revision `571d3971bfb47dc5ddd3746134dd464954bc9df0`) of
  `ustc-community/dfine-nano-coco`; the export repository
  (`AnnotateIt/dfine-nano-coco-onnx`) declares Apache-2.0 and ships a `LICENSE` file,
  and the base repository `ustc-community/dfine-nano-coco` declares Apache-2.0. The
  "accurate" model is the ONNX conversion
  `onnx-community/dfine_s_coco-ONNX` (pinned revision
  `a3cf03147a9b86c78475139115c8ac142577352d`) of the Apache-2.0 weights
  `ustc-community/dfine-small-coco`; the conversion repository has no license
  field of its own, so AuraGo relies on the license of the converted weights.
  Both were trained on COCO (the Hugging Face model tags name the dataset). On
  disk AuraGo only derives a local WebGPU copy (`model.webgpu-1.27.0.onnx`: constant
  folding and one reshaped weight, see `internal/persondetect/gpuprep.go`); it is
  never redistributed.
- **ONNX Runtime WebGPU 1.27.0.** The `onnxruntime-webgpu` wheel for the platform is
  downloaded from PyPI, kept unmodified in the downloads cache, and only the
  runtime libraries are extracted from it (linux/amd64: `libonnxruntime.so.1.27.0`,
  `libonnxruntime_providers_shared.so`; windows/amd64: `onnxruntime.dll`,
  `onnxruntime_providers_shared.dll`, `dxcompiler.dll`, `dxil.dll`; darwin/arm64:
  `libonnxruntime.1.27.0.dylib`). The Windows wheel's `onnxruntime/LICENSE` is the MIT
  License, "Copyright (c) Microsoft Corporation". Its `onnxruntime/ThirdPartyNotices.txt`
  lists, among many others, Dawn (BSD 3-Clause License, "Copyright 2017-2023 The Dawn
  & Tint Authors") and The LLVM Compiler Infrastructure (University of Illinois/NCSA
  Open Source License, "Copyright (c) 2003-2017 University of Illinois at
  Urbana-Champaign"). The CPU path uses the ONNX Runtime 1.26.0 bundles listed above.
- **DirectX Shader Compiler (Windows only).** `dxcompiler.dll` (file version
  1.9.0.1) and `dxil.dll` (file version 1.8.2502.11) are Microsoft-signed binaries
  whose version information reads "DirectX Compiler - Out Of Band", "(c) Microsoft
  Corporation. All rights reserved." AuraGo does not redistribute them: they are
  extracted on the user's machine from the PyPI wheel, unmodified, and loaded by the
  WebGPU provider. The upstream project (https://github.com/microsoft/DirectXShaderCompiler)
  states that it is distributed under the University of Illinois/NCSA Open Source
  License (its `LICENSE.TXT` and `README.md`), and the wheel's `ThirdPartyNotices.txt`
  carries that license text under the LLVM entry; the wheel has no separate
  DirectX Shader Compiler entry, and neither the wheel nor the upstream README or
  `LICENSE.TXT` states redistribution terms specific to the signed `dxil.dll` binary.
  The upstream repository root lists only `LICENSE.TXT` and `ThirdPartyNotices.txt` (no
  separate Microsoft license file) and the notes of its latest release (v1.9.2609)
  carry no license terms; the release archives themselves were not inspected. The
  upstream licenses remain authoritative.
- **Self-test photo.** `internal/persondetect/selftest_reference.jpg` is
  "Pedestrians using crosswalk (Unsplash).jpg" by Peter Miranda, released under
  CC0 1.0 Universal (public domain dedication) on Wikimedia Commons
  (https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg).
  AuraGo embeds a 1280 px thumbnail resized to 800 px wide (LANCZOS, JPEG quality
  85) to check every detector start. Attribution is not required by CC0 and is given
  here for provenance.
