# Storymaker asset contract

## Sources and ownership

AGENTS.md owns production. production.json records retained ImageGen environments/props, prompts, references, dimensions, sizes and hashes. masters/previews contains real player renders. Human bitmap characters were externally archived with byte/hash verification before removal. Archived reference names document historical inputs and are not build dependencies.

models/ contains separate derivatives of ui/3d/pets/{hoodie,professor,punk}. Never overwrite the source pets. Per-character provenance records original/derived hashes, copied notices, facial landmarks and generator hashes. V1 geometry generators remain for shared stage/font/audio definitions, motion vocabulary and compatibility fixtures.

## Delivery schema

- Projects remain storymaker/1; delivery remains storymaker-catalog/2. V1 fixtures still work.
- catalog.json.files lists every dependency except the manifest: relative path, MIME type, exact bytes, lowercase SHA-256 and image width/height. Fonts retain their shared UI inventory. Version is the existing twelve-character sorted-content hash; text uses LF, binary bytes remain unchanged.
- Three canonical characters: char.hoodie, char.professor, char.punk. Their aliases resolve all twelve retired IDs in Go, the compiler and the app. Searches/pickers expose only canonical entries.
- A storymaker-character-3d/1 descriptor names model (embedded GLB) and motions (storymaker-model-motions/1); both appear in resources.
- Motion packs contain rest transforms, fifteen authored poses (ten public poses plus gesture targets) and selected original clips. Nine gestures approach canonical arm targets and return to the base pose. Every frame resets transforms before sampling explicit scene time. Walking blends in/out; actual skinned sole vertices determine ground height.
- Six speech morphs plus implicit rest cover seven mouth names. Ten expressions include implicit neutral; localized blink/brow/lip morphs supply other states. Speech blends neighboring stored mouth samples. No frame-dependent head translation or head replacement is permitted.
- Three.js renders articulated models into transparent Pixi layers. View/facing rotates the model; existing pose/look controls still own perspective. No raw Three.js or new story field is exposed to code scenes.
- Twelve day/night environments retain layered parallax and stage spots; eighty props retain old IDs. Labels, captions and bubbles remain measured runtime text.

## Bounds and trust

- Delivery: 256 MiB, 2000 files, 8 MiB per file. Images: 2048 squared atlases/foreground, 3840 x 2160 backgrounds, 256 squared thumbnails.
- Models: glTF 2 binary, one embedded buffer, no external URI/required extensions, JSON <=1 MiB, <=128 nodes, 16 meshes, 4 skins, 72 joints per skin, 60,000 triangles and 16 morph targets per primitive. At most eight embedded PNG/JPEG textures, each <=1024 squared. Accessor/view counts and allocation sizes are bounded before parsing.
- Go verifies lengths/hashes and encoded images/GLB structure before startup. Export snapshots all listed non-WAV files with the same hashes. Browser resolvers accept only listed relative paths; texture:<path> and model:<path> carry bounded ArrayBuffers.
- Workers verify model bytes before GLTFLoader. A local loader plugin decodes embedded images without URL fetching, checks decoded dimensions and discards cancelled loads. Opaque-origin CSP and worker isolation remain active.
- Cache original bytes; copy independently for each worker. Evict inactive models/textures. Close skeletons, geometries, materials, bitmaps, contexts, canvases, fonts and paused timelines on reset/disposal.
- Pixi may use Canvas on software adapters; characters require WebGL2 (hardware or software). Headless acceptance uses SwiftShader without a physical GPU. Initialization failure uses the normal player readiness error, with no sprite substitute.
- Export reserves 1280 MiB plus encoded catalog bytes per worker and 768 MiB shared headroom; unknown memory selects one worker. Active background/prop decoded images remain capped at 512 MiB; model textures have the bounds above.

## Production and verification

1. node scripts/build-storymaker-models.mjs produces derivatives/motions/provenance offline; --check compares exact output.
2. node scripts/build-storymaker-catalog.mjs packs models and image masters. npm run build:ui bundles local PixiJS 8.22.0, GSAP 3.15.1 and Three.js 0.186.1.
3. node scripts/test-storymaker-raster-browser.mjs --previews --canvas renders library thumbnails. Rebuild the catalog, then run --check. Commit sources, provenance and delivery together.
4. npm run test:storymaker, asset tests and Go catalog checks cover compatibility, integrity, rigid skulls and seek parity.
5. Browser --canvas / --webgl --loss cover three formats, code scenes, seeks/reloads and context recovery. --matrix --character char.hoodie --canvas covers poses/gestures/expressions/mouths in both facings; repeat for the other pets. --speech records mouth/blink sequences; --combinations checks composed arm gestures. Inspect native-size frames and movement; reports stay in ignored ui/reports/storymaker-raster.
6. AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run TestRasterReferenceExportBrowser verifies a real MP4, subtitles and library copy. Audio is a technical calibration tone; AURAGO_STORYMAKER_REVIEW_DIR retains artifacts.

## Rights

Copy each pet's NOTICE, CC0 motion and MIT pipeline licenses verbatim. CC0 motion provenance does not relicense geometry. Derivative generation code follows the repository license. ImageGen originals retain their production records; shared font/audio notices remain in README.md and ../../licences/THIRD_PARTY_LIBRARIES.md. PixiJS/Three.js are MIT; GSAP's Standard No Charge License remains reproduced beside the local bundle.
