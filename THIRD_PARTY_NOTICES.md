# Third-party notices

## Gradium Phonon speech resources

The separate local speech package contains Gradium's Phonon `ptts` 0.4.0 runtime
(MIT/Apache-2.0), the Apache-2.0 `gradium/phonon-voices` catalog and the unchanged
q8 weights of `gradium/phonon-40m-multilingual` (CC BY-NC 4.0, noncommercial only).
Attribution, source URLs, exact revisions and checksums are in
[`documentation/phonon.md`](documentation/phonon.md). Packages carry that notice,
the original runtime MIT text and the Apache-2.0 and CC BY-NC 4.0 license texts.
These component licenses do not change AuraGo's own MIT license.

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
| ISNet general-use (DIS), ONNX file `isnet-general-use.onnx` from the rembg release `v0.0.0` (178,648,008 bytes) | Pixel background removal, "AI standard" engine; downloaded only when a user clicks "Download model" | Apache License 2.0 (upstream project: DIS, https://github.com/xuebinqin/DIS) |
| BiRefNet general lite (Swin-tiny backbone), ONNX file `BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx` from the rembg release `v0.0.0` (224,005,088 bytes) | Pixel background removal, "AI precise" engine; downloaded only when a user clicks "Download model" | MIT License (upstream project: BiRefNet, https://github.com/ZhengPeng7/BiRefNet) |
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
- https://github.com/danielgatis/rembg/releases/tag/v0.0.0
- https://github.com/xuebinqin/DIS
- https://github.com/ZhengPeng7/BiRefNet
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

## Pixel AI background removal models

The Pixel image editor (`internal/pixelmatte`, Cutout panel) can remove backgrounds with two ONNX
matting models. AuraGo does not ship them. They are downloaded only after a user clicks
**Download model**, from the HTTPS URLs pinned in `internal/pixelmatte/model.go`. Those URLs point
to the `v0.0.0` release of the rembg project on GitHub
(https://github.com/danielgatis/rembg/releases/tag/v0.0.0). Each download is checked by its exact
size and SHA-256 before it is used. AuraGo loads the files unmodified and does not redistribute them.

- **ISNet general-use.** File `isnet-general-use.onnx`, 178,648,008 bytes, SHA-256
  `60920e99c45464f2ba57bee2ad08c919a52bbf852739e96947fbb4358c0d964a`. The upstream project is
  DIS (dichotomous image segmentation), https://github.com/xuebinqin/DIS, licensed under the
  Apache License 2.0. It is the "AI standard" engine.
- **BiRefNet general lite.** File `BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx`, 224,005,088
  bytes, SHA-256 `5600024376f572a557870a5eb0afb1e5961636bef4e1e22132025467d0f03333`. The upstream
  project is BiRefNet, https://github.com/ZhengPeng7/BiRefNet, licensed under the MIT License. It
  is the "AI precise" engine.
- Both models run in an isolated worker process on the CPU ONNX Runtime listed above. Images go
  to the AuraGo server only; no external service receives them.

## Shared QR codes

- `github.com/makiuchi-d/gozxing` v0.1.1: Copyright (c) 2018 Daisuke MAKIUCHI (MakKi; makki_d), MIT. Its original ZXing portions are Copyright 2007-2018 ZXing authors, Apache License 2.0. [Source](https://github.com/makiuchi-d/gozxing/tree/v0.1.1).
- `github.com/go-text/typesetting` v0.3.5: Copyright 2021 The go-text authors, Unlicense OR BSD-3-Clause. [Source](https://github.com/go-text/typesetting/tree/v0.3.5). The HarfBuzz translation retains the following MIT copyright lines:

```text
Copyright (c) 2021 B. KUGLER
Copyright © 2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020  Google, Inc.
Copyright © 2018,2019,2020  Ebrahim Byagowi
Copyright © 2019,2020  Facebook, Inc.
Copyright © 2012  Mozilla Foundation
Copyright © 2011  Codethink Limited
Copyright © 2008,2010  Nokia Corporation and/or its subsidiary(-ies)
Copyright © 2009  Keith Stribley
Copyright © 2009  Martin Hosken and SIL International
Copyright © 2007  Chris Wilson
Copyright © 2006  Behdad Esfahbod
Copyright © 2005  David Turner
Copyright © 2004,2007,2008,2009,2010  Red Hat, Inc.
Copyright © 1998-2004  David Turner and Werner Lemberg
```

The complete MIT, BSD-3-Clause, Unlicense and Apache-2.0 texts are retained under `licences/texts/`.

The verified external QR font set contains Noto Sans Arabic and Devanagari (Copyright 2022 The Noto Project Authors), Noto Sans CJK SC, Noto Emoji (Copyright 2013 Google LLC), and Liberation Serif (digitised data Copyright 2010 Google Corporation, reserved names Arimo/Tinos/Cousine; Copyright 2012 Red Hat, Inc., reserved name Liberation). All retain their original OFL-1.1 texts alongside the font bytes. Exact font sources, immutable revisions, hashes and lengths are in [`ui/fonts/qr/manifest.json`](ui/fonts/qr/manifest.json). Go Sans/Mono are supplied by the already pinned `golang.org/x/image/font/gofont` dependency.

Go Sans/Mono: Copyright (c) 2016 Bigelow & Holmes Inc., all rights reserved. The original BSD-style font terms (including the Google/contributor endorsement clause) are preserved in `ui/fonts/qr/Go-fonts-LICENSE.txt` and accompany the resource set.

The independent browser test uses `jsqr` 1.4.0, Apache-2.0 ([source](https://github.com/cozmo/jsQR)); it is a development dependency and is not a production QR runtime. [Mini QR](https://github.com/lyqht/mini-qr) is the functional reference; AuraGo's implementation/presets contain no copied Mini QR source or assets.

## Ziro Designer (Software Store)

ZiroEDA and contributors; portions derived from KiCad, copyright The KiCad Developers.
GPL-3.0-or-later; KiCad libraries CC-BY-SA-4.0 with the KiCad library exception.
The separate Store image includes `/LICENSE`, `/NOTICE.md` and `/source.tar.gz`
with the pinned upstream source and AuraGo adapter, served by the image web server.
Upstream: https://github.com/ZiroEDA/ziro-designer/tree/7b4597005e4801d4fb19661fcac6fc3380547cf2.

## Additional Desktop icon themes

Tela (Vince Liuice), Candy (Eliver Lara) and Fluent (Vince Liuice) artwork is
distributed under GPL-3.0. Tela/Fluent preserve Numix, Papirus and Paper source
credits in their AUTHORS.txt. Buuf For Many Desktops (eudaimon/Nestort), based on
Paul Davey/Mattahan with OxayotlTheGreat and robson-66 remixes, is CC BY-NC-SA 2.5
and is included for noncommercial use. The imported Buuf assets are not MIT.

Pinned revisions, per-motif source paths, hashes and changes are documented in
`documentation/desktop-icon-themes.md` and `ui/img/<theme>/provenance.json`.
Each generated theme directory ships LICENSE.txt and NOTICE.txt; Buuf also ships
its original README/index declarations. Original AuraGo supplements are separately
marked MIT with their own license copy. All notices accompany the offline web
resource package.
