# Storymaker sprite engine

## Accepted implementation contract

- PixiJS 8.22.0 and GSAP 3.15.1, locally bundled, deterministic explicit frame evaluation.
- Existing `storymaker/1` documents and asset IDs remain usable; new artwork ships as `storymaker-catalog/2`.
- Twelve fixed illustrated characters, twelve day/night environments, eighty props. See `assets/storymaker/lib/raster-catalog.mjs` for the exact IDs and prop packs.
- ImageGen creates original masters during development; ordinary builds use the retained masters offline. Production references and prompts belong to `assets/storymaker/production.json`.
- Thirteen-bone rigs with three views, all existing poses/gestures/expressions/visemes, facial and hand swaps, calibrated joints and ground contact.
- Strict opaque-origin iframe and isolated code-scene workers remain mandatory. Text layout, captions, speech and audio timing retain their existing contracts.
- GSAP's current no-charge licence and FAQ distinguish niche visual tools from direct Webflow competitors. Storymaker produces explainer films; preserve the actual terms and notices rather than labelling GSAP MIT.

## Implemented delivery

- The installed v2 library replaces the vector artwork with twelve characters, twelve day/night environments and eighty props. Production retains 122 ImageGen originals, exact prompts, references and hashes. The deterministic catalog stays below 256 MiB.
- The persistent Pixi scene graph and paused GSAP tracks are shared by preview, thumbnails, isolated code scenes and export. Known software GPU adapters select Canvas; hardware WebGL remains preferred. Library clocks and tickers are inert.
- Facial and hand swaps, seated lower-body drawings, calibrated attachment boxes, ankle pivots and planted two-bone walking extend the shared skeleton. Labels remain measured, translatable runtime text.
- Go startup, browser texture decoding and export snapshots validate the same bounded inventory. Each worker owns its transfer buffers, fonts, textures and timelines. Cancellation and project changes release them, including late asynchronous results.
- Export concurrency reserves encoded images, simultaneous decoded CPU/GPU copies and shared Go/FFmpeg headroom. Unknown available memory selects one raster worker. Byte-backed frame POSTs let Chromium enforce bounded DevTools payload caches.
- All sixteen catalog name tables, agent search tags, thumbnail previews, licence inventories and owning DOX contracts are updated. No rig editor, Spine dependency or runtime image generation is included.

## Verification

- `npm run test:storymaker`: 215 checks, including original compiler/audio/subtitle behavior, real GSAP sampling, provenance, malformed textures, deterministic planted feet and bounded lifecycle.
- `node --test assets/storymaker/test/*.test.mjs`: 44 production-data, deterministic-build, font and audio checks.
- `go test ./internal/storymaker/... -count=1`, `go test ./internal/server -run Storymaker -count=1`, `go test ./ui -run Storymaker -count=1`.
- Native browser checks require installed Chrome and `AURAGO_RUN_BROWSER_SMOKE=1`. Run parallel parity/encoder-restart tests, UI preview/export parity, app/Video Studio handoff and `TestRasterReferenceExportBrowser`.
- `node scripts/test-storymaker-raster-browser.mjs --canvas`, `--webgl --loss` and `--software` exercise real renderer paths, arbitrary/reverse seeks, fresh reloads and context recovery. Windows Chrome 156 and Linux Chrome 157 passed the three-format storyboard/code matrix with sandboxing enabled and no physical GPU.
- `--matrix --canvas` writes 72 contact sheets and animated scene documents: twelve characters, three views, two facings and 36 states each. Contact sheets render at 512 pixels; format/export checks use full output resolution. `--environments --canvas` checks all 72 day/night/aspect combinations.
- `node scripts/build-storymaker-catalog.mjs --check`, `npm run check:ui`, then the normal `cmd/assetpack` packaging check.

## Production and implementation entrypoints

- `assets/storymaker/AGENTS.md` owns the artwork workflow.
- `scripts/build-storymaker-vendor.mjs` builds/checks pinned runtime and notices.
- `scripts/build-storymaker-raster.mjs --prototype` retains the isolated technical prototype workflow; without it, the canonical v2 catalog builder runs with complete production validation.
- `ui/js/storymaker/raster.js` draws persistent sprites and `textures.js` owns bitmap integrity/metadata checks; the existing workers and host carry the new texture dependency keys.
- `ui/img/storymaker` contains the installed catalog. Shared font and sound-effect provenance remains in the existing inventories.

## Review artifacts and limits

Ignored `ui/reports/storymaker-raster/` holds contact sheets, full-resolution captures, browser logs, a real 900-frame MP4 with SRT/VTT, library-copy evidence and measured process memory/export throughput. The reference soundtrack is a calibration tone; it does not assess voice quality.

Textured graphics use more memory and export more slowly than the former vectors. Compare both on the same fixture and machine; software rendering does not establish physical-GPU performance. Hardware performance remains a separate check. A current installed Chrome is preferable to the older provisioned Chromium 128; long CPU renders may need a larger `render.timeout_seconds`.
