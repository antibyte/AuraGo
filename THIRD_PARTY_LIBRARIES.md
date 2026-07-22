# Third-Party Libraries and Assets

Inventory of external libraries, runtimes, fonts, icons, and other third-party assets used by **AuraGo**.

- **AuraGo project license:** [MIT](LICENSE)
- **Generated:** 2026-07-19
- **Sources:** `go.mod`, root `package.json`, `browser_automation_sidecar/package.json`, `ui/js/vendor/`, `ui/fonts/`, `ui/img/`, `THIRD_PARTY_NOTICES.md`

> **Note:** Upstream license texts are authoritative. This document is an attribution inventory. Versions may drift; check `go.mod` / `package.json` / vendored file headers for the exact pin in your checkout.
>
> Transitive Go modules are resolved via `go.sum` and are not listed exhaustively. Direct / first-party-used components are listed below.

---

## Table of contents

1. [Go — direct dependencies](#1-go--direct-dependencies)
2. [Go — notable indirect / platform](#2-go--notable-indirect--platform)
3. [Frontend — vendored JS/CSS/WASM](#3-frontend--vendored-jscsswasm)
4. [Frontend — npm packages (build / vendor sources)](#4-frontend--npm-packages-build--vendor-sources)
5. [Browser automation sidecar](#5-browser-automation-sidecar)
6. [Fonts](#6-fonts)
7. [Icon themes and UI assets](#7-icon-themes-and-ui-assets)
8. [On-demand / optional runtimes](#8-on-demand--optional-runtimes)
9. [Copyleft components (attention)](#9-copyleft-components-attention)

---

## 1. Go — direct dependencies

| Package | Version (from `go.mod`) | License | Project |
|---|---|---|---|
| `gopkg.in/yaml.v3` | v3.0.1 | MIT and Apache-2.0 | https://github.com/go-yaml/yaml |
| `github.com/sashabaranov/go-openai` | v1.41.2 | Apache-2.0 | https://github.com/sashabaranov/go-openai |
| `codeberg.org/readeck/go-readability/v2` | v2.1.2 | MIT | https://codeberg.org/readeck/go-readability |
| `github.com/BurntSushi/toml` | v1.6.0 | MIT | https://github.com/BurntSushi/toml |
| `github.com/JohannesKaufmann/html-to-markdown/v2` | v2.5.2 | MIT | https://github.com/JohannesKaufmann/html-to-markdown |
| `github.com/PuerkitoBio/goquery` | v1.12.0 | BSD-2-Clause | https://github.com/PuerkitoBio/goquery |
| `github.com/SherClockHolmes/webpush-go` | v1.4.0 | MIT | https://github.com/SherClockHolmes/webpush-go |
| `github.com/a2aproject/a2a-go/v2` | v2.3.1 | Apache-2.0 | https://github.com/a2aproject/a2a-go |
| `github.com/beevik/etree` | v1.7.0 | BSD-2-Clause | https://github.com/beevik/etree |
| `github.com/bwmarrin/discordgo` | v0.29.0 | BSD-3-Clause | https://github.com/bwmarrin/discordgo |
| `github.com/danielthedm/promptsec` | v0.1.0 | Apache-2.0 | https://github.com/danielthedm/promptsec |
| `github.com/emiago/diago` | v0.31.0 | MPL-2.0 | https://github.com/emiago/diago |
| `github.com/emiago/sipgo` | v1.4.3 | BSD-2-Clause | https://github.com/emiago/sipgo |
| `github.com/dlclark/regexp2` | v1.12.0 | MIT | https://github.com/dlclark/regexp2 |
| `github.com/ebitengine/purego` | v0.10.1 | Apache-2.0 | https://github.com/ebitengine/purego |
| `github.com/eclipse/paho.mqtt.golang` | v1.5.1 | EPL-2.0 | https://github.com/eclipse/paho.mqtt.golang |
| `github.com/go-ldap/ldap/v3` | v3.4.14 | MIT | https://github.com/go-ldap/ldap |
| `github.com/go-rod/rod` | v0.116.2 | MIT | https://github.com/go-rod/rod |
| `github.com/go-sql-driver/mysql` | v1.10.0 | MPL-2.0 | https://github.com/go-sql-driver/mysql |
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | v5.5.1 | MIT | https://github.com/go-telegram-bot-api/telegram-bot-api |
| `github.com/gocolly/colly/v2` | v2.3.0 | Apache-2.0 | https://github.com/gocolly/colly |
| `github.com/gofrs/flock` | v0.13.0 | BSD-3-Clause | https://github.com/gofrs/flock |
| `github.com/gorilla/websocket` | v1.5.4 (pseudo) | BSD-2-Clause | https://github.com/gorilla/websocket |
| `github.com/huin/goupnp` | v1.3.0 | BSD-2-Clause | https://github.com/huin/goupnp |
| `github.com/hajimehoshi/go-mp3` | v0.3.4 | Apache-2.0 | https://github.com/hajimehoshi/go-mp3 |
| `github.com/johnfercher/maroto/v2` | v2.4.0 | MIT | https://github.com/johnfercher/maroto |
| `github.com/ledongthuc/pdf` | 20250511 | BSD-style / MIT-compatible | https://github.com/ledongthuc/pdf |
| `github.com/lib/pq` | v1.12.3 | MIT | https://github.com/lib/pq |
| `github.com/miekg/dns` | v1.1.72 | BSD-3-Clause | https://github.com/miekg/dns |
| `github.com/minio/minio-go/v7` | v7.2.1 | Apache-2.0 | https://github.com/minio/minio-go |
| `github.com/pdfcpu/pdfcpu` | v0.13.0 | Apache-2.0 | https://github.com/pdfcpu/pdfcpu |
| `github.com/philippgille/chromem-go` | v0.7.0 | MPL-2.0 | https://github.com/philippgille/chromem-go |
| `github.com/pkg/sftp` | v1.13.11 | BSD-2-Clause | https://github.com/pkg/sftp |
| `github.com/pkoukk/tiktoken-go` | v0.1.8 | MIT | https://github.com/pkoukk/tiktoken-go |
| `github.com/prometheus-community/pro-bing` | v0.9.1 | MIT | https://github.com/prometheus-community/pro-bing |
| `github.com/robfig/cron/v3` | v3.0.1 | MIT | https://github.com/robfig/cron |
| `github.com/shirou/gopsutil/v4` | v4.26.6 | BSD-3-Clause | https://github.com/shirou/gopsutil |
| `github.com/tailscale/go-winio` | 20231025 | MIT | https://github.com/tailscale/go-winio |
| `github.com/tidwall/gjson` | v1.19.0 | MIT | https://github.com/tidwall/gjson |
| `github.com/tidwall/sjson` | v1.2.5 | MIT | https://github.com/tidwall/sjson |
| `github.com/vishen/go-chromecast` | v0.3.4 | Apache-2.0 | https://github.com/vishen/go-chromecast |
| `github.com/xeipuuv/gojsonschema` | v1.2.0 | Apache-2.0 | https://github.com/xeipuuv/gojsonschema |
| `github.com/xuri/excelize/v2` | v2.11.0 | BSD-3-Clause | https://github.com/qax-os/excelize |
| `golang.org/x/crypto` | v0.54.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/crypto |
| `golang.org/x/image` | v0.44.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/image |
| `golang.org/x/net` | v0.57.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/net |
| `golang.org/x/sync` | v0.22.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/sync |
| `golang.org/x/sys` | v0.47.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/sys |
| `google.golang.org/grpc` | v1.82.1 | Apache-2.0 | https://github.com/grpc/grpc-go |
| `modernc.org/sqlite` | v1.54.0 | BSD-3-Clause | https://gitlab.com/cznic/sqlite |
| `tailscale.com` | v1.100.0 | BSD-3-Clause | https://github.com/tailscale/tailscale |

---

## 2. Go — notable indirect / platform

Selected transitive modules that matter operationally (not exhaustive):

| Package | License | Project |
|---|---|---|
| `github.com/aws/aws-sdk-go-v2` (+ config/smithy) | Apache-2.0 | https://github.com/aws/aws-sdk-go-v2 |
| `github.com/golang-jwt/jwt/v5` | MIT | https://github.com/golang-jwt/jwt |
| `github.com/google/uuid` | BSD-3-Clause | https://github.com/google/uuid |
| `github.com/klauspost/compress` | Apache-2.0 / BSD / MIT (multi) | https://github.com/klauspost/compress |
| `github.com/phpdave11/gofpdf` | MIT | https://github.com/phpdave11/gofpdf |
| `github.com/ysmood/*` (rod ecosystem) | MIT | https://github.com/ysmood |
| `golang.org/x/oauth2` | BSD-3-Clause | https://pkg.go.dev/golang.org/x/oauth2 |
| `golang.org/x/text` | BSD-3-Clause | https://pkg.go.dev/golang.org/x/text |
| `google.golang.org/protobuf` | BSD-3-Clause | https://github.com/protocolbuffers/protobuf-go |
| `gvisor.dev/gvisor` | Apache-2.0 | https://github.com/google/gvisor |
| `modernc.org/libc` / `mathutil` / `memory` | BSD-3-Clause | https://gitlab.com/cznic |
| WireGuard / wintun related (`golang.zx2c4.com/*`, Tailscale forks) | MIT / BSD-style | https://www.wireguard.com/ |

Full module graph: `go list -m all` / `go.sum`.

---

## 3. Frontend — vendored JS/CSS/WASM

Embedded under `ui/` (served via `go:embed`). Paths relative to repo root.

### Core UI libraries

| Asset / component | Path | License | Project |
|---|---|---|---|
| Chart.js v4.5.1 | `ui/chart.min.js` | MIT | https://www.chartjs.org / https://github.com/chartjs/Chart.js |
| Tailwind CSS (Play CDN / standalone) | `ui/tailwind.min.js` | MIT | https://tailwindcss.com / https://github.com/tailwindlabs/tailwindcss |
| markdown-it 14.0.0 | `ui/js/vendor/markdown-it.min.js` | MIT | https://github.com/markdown-it/markdown-it |
| marked | `ui/js/vendor/marked.min.js` | MIT | https://github.com/markedjs/marked |
| highlight.js | `ui/js/vendor/highlight.min.js` | BSD-3-Clause | https://github.com/highlightjs/highlight.js |
| highlight.js GitHub themes | `ui/css/hljs-github.min.css`, `ui/css/hljs-github-dark.min.css` | BSD-3-Clause (theme lineage) | https://github.com/highlightjs/highlight.js |
| DOMPurify 3.4.11 | `ui/js/vendor/purify.min.js` | Apache-2.0 **or** MPL-2.0 | https://github.com/cure53/DOMPurify |
| Mermaid | `ui/js/vendor/mermaid.min.js` | MIT | https://github.com/mermaid-js/mermaid |
| Quill 2.x | `ui/js/vendor/quill.js`, `ui/css/quill.snow.css` | BSD-3-Clause | https://quilljs.com / https://github.com/slab/quill |
| CodeMirror 6 (bundled) | `ui/js/vendor/codemirror-bundle.esm.js` | MIT | https://codemirror.net / https://github.com/codemirror/dev |
| xterm.js | `ui/js/vendor/xterm.min.js`, `ui/css/xterm.css` | MIT | https://github.com/xtermjs/xterm.js |
| xterm-addon-fit | `ui/js/vendor/xterm-addon-fit.min.js` | MIT | https://github.com/xtermjs/xterm.js |
| PDF.js | `ui/js/vendor/pdf.min.js`, `ui/js/vendor/pdf.worker.min.js` | Apache-2.0 | https://github.com/mozilla/pdf.js |
| hls.js v1.5.20 | `ui/js/vendor/hls.min.js` | Apache-2.0 | https://github.com/video-dev/hls.js |
| force-graph | `ui/js/vendor/force-graph.min.js` | MIT | https://github.com/vasturiano/force-graph |
| qrcode | `ui/js/vendor/qrcode.min.js` | MIT | https://github.com/davidshimjs/qrcodejs (common embed lineage) |
| noVNC | `ui/js/vendor/novnc.min.js` | MPL-2.0 | https://github.com/novnc/noVNC |

### 3D / graphics

| Asset / component | Path | License | Project |
|---|---|---|---|
| three.js (r128 lineage in header) | `ui/js/vendor/three.min.js` | MIT | https://threejs.org / https://github.com/mrdoob/three.js |
| GLTFLoader | `ui/js/vendor/GLTFLoader.min.js` | MIT | three.js examples |
| STLLoader | `ui/js/vendor/STLLoader.min.js` | MIT | three.js examples |
| OrbitControls | `ui/js/vendor/OrbitControls.min.js` | MIT | three.js examples |
| DRACOLoader | `ui/js/vendor/DRACOLoader.min.js` | MIT | three.js examples |
| Google Draco decoder | `ui/js/vendor/draco/*` | Apache-2.0 | https://github.com/google/draco |

### Chess / games

| Asset / component | Path | License | Project |
|---|---|---|---|
| chess.js + cm-chessboard vendor bundle | `ui/js/vendor/chess-vendor.esm.js` | BSD-2-Clause (chess.js) + MIT (cm-chessboard) | https://github.com/jhlywa/chess.js · https://github.com/shaack/cm-chessboard |
| cm-chessboard CSS | `ui/css/cm-chessboard.css` | MIT | https://github.com/shaack/cm-chessboard |
| Stockfish 18 lite (WASM engine) | `ui/js/vendor/stockfish/*` | **GPL-3.0** | https://stockfishchess.org / https://github.com/official-stockfish/Stockfish · npm `stockfish` |

### Desktop media player

| Asset / component | Path | License | Project |
|---|---|---|---|
| Webamp | `ui/js/vendor/webamp/*` | MIT | https://github.com/captbaritone/webamp |

### Realtime speech (browser VAD)

See also `documentation/third_party/realtime_speech_browser_runtime.md`.

| Asset / component | Path | License | Project |
|---|---|---|---|
| ONNX Runtime Web 1.27.0 | `ui/js/realtime-speech/vendor/ort*.js`, `*.wasm`, `*.mjs` | MIT | https://github.com/microsoft/onnxruntime |
| Silero VAD v6.2.1 | `ui/js/realtime-speech/vendor/silero_vad_v6.2.1.onnx` | MIT | https://github.com/snakers4/silero-vad |

---

## 4. Frontend — npm packages (build / vendor sources)

Root `package.json` (`aurago-web-assets`) — used to build/vendor UI assets; not all are runtime CDN loads.

| Package | Declared version | License | Project |
|---|---|---|---|
| `@codemirror/*` (autocomplete, commands, language packs, state, view, lint, search, theme-one-dark, …) | ^6.x | MIT | https://github.com/codemirror/dev |
| `chess.js` | 1.4.0 | BSD-2-Clause | https://github.com/jhlywa/chess.js |
| `cm-chessboard` | 8.12.12 | MIT | https://github.com/shaack/cm-chessboard |
| `headroom-ai` | ^0.22.4 | Apache-2.0 | https://www.npmjs.com/package/headroom-ai |
| `onnxruntime-web` | 1.27.0 | MIT | https://github.com/microsoft/onnxruntime |
| `quill` | 2.0.2 | BSD-3-Clause | https://github.com/slab/quill |
| `stockfish` | 18.0.8 | **GPL-3.0** | https://www.npmjs.com/package/stockfish |
| `rollup` | ^4.62.2 | MIT | https://rollupjs.org |
| `@rollup/plugin-node-resolve` | ^16.0.3 | MIT | https://github.com/rollup/plugins |

---

## 5. Browser automation sidecar

`browser_automation_sidecar/package.json`:

| Package | Declared version | License | Project |
|---|---|---|---|
| `playwright-core` | ^1.61.1 | Apache-2.0 | https://github.com/microsoft/playwright |
| `cloakbrowser` | ^0.4.11 | MIT | https://www.npmjs.com/package/cloakbrowser |

---

## 6. Fonts

Self-hosted under `ui/fonts/` (see `ui/fonts/fonts.css`).

| Family | Typical license | Project / source |
|---|---|---|
| Geist | SIL Open Font License 1.1 | https://vercel.com/font · https://github.com/vercel/geist-font |
| Geist Mono | SIL Open Font License 1.1 | https://github.com/vercel/geist-font |
| Inter | SIL Open Font License 1.1 | https://github.com/rsms/inter |
| Darker Grotesque | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Darker+Grotesque |
| Exo 2 | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Exo+2 |
| Oxanium | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Oxanium |
| Special Elite | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Special+Elite |
| Schoolbell | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Schoolbell |
| Shadows Into Light Two | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Shadows+Into+Light+Two |
| Press Start 2P | SIL Open Font License 1.1 | https://fonts.google.com/specimen/Press+Start+2P |
| Datatype | Check upstream (self-hosted specialty face) | Vendor/original foundry for the Datatype files in `ui/fonts/` |

---

## 7. Icon themes and UI assets

### Third-party icon themes (GPL)

| Theme | Path | License | Project |
|---|---|---|---|
| Papirus Icon Theme | `ui/img/papirus/` (+ `LICENSE-Papirus.txt`) | **GPL-3.0** | https://github.com/PapirusDevelopmentTeam/papirus-icon-theme |
| WhiteSur Icon Theme | `ui/img/whitesur/` (+ `LICENSE-WhiteSur.txt`) | **GPL-3.0** | https://github.com/vinceliuice/WhiteSur-icon-theme |

Manifests: `ui/img/papirus/manifest.json`, `ui/img/whitesur/manifest.json`.

### First-party / project assets (AuraGo)

Not third-party, listed for completeness:

| Asset class | Path examples | Notes |
|---|---|---|
| Logos, favicons | `ui/aurago_logo*.png`, `ui/favicon*`, `ui/web-app-manifest-*.png` | AuraGo branding |
| Chat / persona / tool icons | `ui/img/chat-ui-icons/`, `ui/img/persona*`, sprites | Project artwork |
| Wallpapers | `ui/img/wallpapers/` | Project media |
| 3D models | `ui/3d/*.glb` | Project models |
| Sample media | `assets/media_samples/`, `assets/*_samples/` | Bundled samples |

---

## 8. On-demand / optional runtimes

Documented in `THIRD_PARTY_NOTICES.md`. **Not** embedded in the default source tree/binary; downloaded when `embeddings.provider` is `local-granite` (verified by size + SHA-256).

| Component | Use | License | Project |
|---|---|---|---|
| IBM Granite Embedding 97M Multilingual R2 | Local embeddings model | Apache-2.0 | https://huggingface.co/ibm-granite/granite-embedding-97m-multilingual-r2 |
| ONNX Runtime 1.26.0 (native) | ONNX inference | MIT | https://github.com/microsoft/onnxruntime |
| llama.cpp (pinned build) | GGUF embedding server / sidecars | MIT | https://github.com/ggml-org/llama.cpp |
| Ebitengine PureGo | CGO-free native calls | Apache-2.0 | https://github.com/ebitengine/purego |
| dlclark/regexp2 | Tokenizer regex | MIT | https://github.com/dlclark/regexp2 |

---

## 9. Copyleft components (attention)

These use strong or network-relevant copyleft licenses. Redistributors should review obligations carefully:

| Component | License | Where used |
|---|---|---|
| Stockfish | GPL-3.0 | Desktop chess engine (`ui/js/vendor/stockfish/`) |
| Papirus icons | GPL-3.0 | Virtual Desktop icon theme |
| WhiteSur icons | GPL-3.0 | Virtual Desktop icon theme (fruity theme) |
| DOMPurify | Apache-2.0 **or** MPL-2.0 (dual) | HTML sanitization |
| Diago | MPL-2.0 | Native SIP endpoint and RTP media handling |
| go-sql-driver/mysql | MPL-2.0 | MySQL client |
| chromem-go | MPL-2.0 | Embedded vector DB |
| Eclipse Paho MQTT | EPL-2.0 | MQTT client |
| noVNC | MPL-2.0 | VNC in virtual computers UI |

---

## How to regenerate / audit

```bash
# Go modules
go list -m all
go list -m -json all   # machine-readable, includes path

# Optional license scan (if go-licenses is installed)
# go install github.com/google/go-licenses@latest
# go-licenses report ./...

# Frontend declared deps
npm ls --all
# Sidecar
cd browser_automation_sidecar && npm ls --all
```

Related docs:

- `LICENSE` — AuraGo MIT license
- `THIRD_PARTY_NOTICES.md` — local embedding runtime attributions
- `documentation/third_party/realtime_speech_browser_runtime.md` — Silero + ORT web pins

---

## Disclaimer

License classifications above are based on license files in the module cache, package metadata, and file headers at generation time. Always verify against the license file shipped with the exact version you distribute. This is not legal advice.
