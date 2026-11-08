# Storymaker asset contract

## Ownership and inputs

`AGENTS.md` owns production. `production.json` records the 122 ImageGen originals, exact prompts, reference relationships, dimensions, sizes and SHA-256 values. Originals stay in `masters/characters`, `masters/props` and `masters/environments`. `masters/previews` contains previews rendered from the assembled rig, with its renderer/source-catalog manifest. Image generation is never part of a normal build.

The retained v1 geometry generators supply the compatible skeleton, motion, stage, glyph and audio definitions and legacy test fixtures. The delivered graphics are exclusively bitmap images. Fixed character identities do not expose the former interchangeable vector clothing/hair colours.

## Delivery schema

- Project documents remain `storymaker/1`. Catalogs use `storymaker-catalog/2`; runtime and Go readers retain v1 fixture support.
- `catalog.json.files` inventories every catalog dependency except itself: relative `path`, MIME `type`, exact `bytes`, lowercase `sha256`, and image `width`/`height`. Fonts remain in the shared UI font inventory, referenced by `catalog.fonts` and `glyphs.json`.
- `version` is the first twelve hex characters of the existing sorted-content catalog hash. JSON/text use LF; PNG/WAV bytes are hashed unchanged.
- A character entry names a `storymaker-character/1` JSON descriptor and `resources`. It defines `front`, `three_quarter` and `profile` attachment arrays, calibrated `box`, `bone`, optional `pivot` and `upright`, draw order, facial/hand `variants` and view-specific `seated` replacements. Views face right; the actor transform mirrors them to face left.
- Every `image` names a listed PNG and a bounded integer `rect: [x,y,width,height]`. Crops retain transparent overlap around joints. Facial cells preserve a common neck pivot; shoes pivot at the ankle and stay upright during planted walking. Sitting uses replacement lower-body drawings.
- `lib/raster-attachments.mjs` measures opaque joint cross-sections. Its calibrated boxes place both segment anchors on the shared bones; optional finite `shear` (absolute value below one) straightens the source centerline in attachment-local coordinates after pivot/upright placement. Adjacent knee/elbow widths match. Hand swaps use connected-subject crops, a common wrist anchor and uniform scaling within body-specific hand lengths. Near arms draw above the torso; front and angled views retain separate orders. Keep retained master pixels unchanged during calibration.
- Environment descriptors contain separate background and foreground layers, per-layer parallax, day/night variants, local colour tokens and the existing aspect-specific stage/spot definitions. Labels, captions and speech bubbles remain measured, translatable runtime text.
- The library contains exactly twelve characters, twelve environments and eighty props, including all previous IDs. Pose, gesture, expression, mouth and movement names retain their meaning. `lib/raster-catalog.mjs` owns the explicit ID lists; `lib/raster-names.mjs` adds genuine names in all sixteen locales.

## Bounds and trust

- Total delivery budget: 256 MiB / 2000 dependencies. Per dependency: 8 MiB. Atlases and foreground sheets: at most 2048²; environment backgrounds: 3840 × 2160; thumbnails: 256².
- Go startup verifies exact length, hash, encoded type and dimensions. The render snapshot retains the checked startup manifest, copies listed files with a length limit and rechecks hashes. Browser resolvers permit only listed relative paths; `texture:<path>` carries bounded ArrayBuffers. The worker verifies original bytes and encoded dimensions before native decoding, then checks decoded dimensions again.
- Cache original bytes; transfer a separate copy to each worker. Decode only active images, evict inactive textures, and close bitmaps/renderers/fonts and paused timelines on disposal. A late asynchronous decode must close its result after cancellation.
- Runtime active decoded images are capped at 512 MiB. Keep at most three renderer sizes and nine compositor canvases. Export workers reserve 1280 MiB plus encoded image bytes per worker and 768 MiB shared headroom; unknown memory permits one raster worker. This accounts for simultaneous decoded bitmaps and GPU copies. Known software WebGL adapters select Canvas; hardware WebGL remains preferred. Reinitialize the text backing canvas whenever overlay contents change to prevent retained pixels on backward seeks.

## Production and verification

1. Use ImageGen with the accepted character/style reference; save the original and record its exact prompt and references.
2. Calibrate atlas cells, pivots, joint overlap and replacements in `lib/raster-catalog.mjs`. Packing uses deterministic PNG crop/encode operations; it never invents missing drawings.
3. Run `node scripts/build-storymaker-catalog.mjs`, then `node scripts/test-storymaker-raster-browser.mjs --previews --canvas`, then build again to publish the rendered thumbnails. Commit masters, production record, generator and outputs together.
4. Run the catalog `--check`, `npm run test:storymaker`, the Go catalog/render checks and native browser checks. `--matrix --canvas` renders every character/view/facing with all poses, gestures, expressions and mouth drawings (test-only viseme selection); `--environments --canvas` checks every day/night background in all three formats. `--software` checks automatic Canvas selection. `--webgl --loss` explicitly overrides that selection to verify the real WebGL path and context recovery through SwiftShader without a physical GPU. Reports go under ignored `ui/reports/storymaker-raster`.
   For visible joint review use `--matrix --canvas --character char.office-woman-1` (or another catalog identity): 512-pixel cells and separate pose/gesture frames expose seams that disappear in small thumbnails. `25-sprite-joints.mjs` checks all identities/views for actual alpha coverage around bent joints, matching widths, bone-aligned endpoints and bounded, unstretched hands.
5. `TestRasterReferenceExportBrowser` renders the 30-second two-character/ten-prop fixture, verifies MP4 frame count, subtitles and the library copy. Its seeded audio is a technical calibration tone, not synthesized speech. `AURAGO_STORYMAKER_REVIEW_DIR` retains review artifacts.

## Rights

Artwork was generated for this repository using ImageGen, without downloaded third-party artwork or branded characters. Retain the original production record. Shared fonts and existing sound effects keep their separate notices in `README.md` and `../../licences/THIRD_PARTY_LIBRARIES.md`. PixiJS is MIT; GSAP retains its Standard No Charge License and its product-use restrictions, reproduced beside the local vendor bundle.
