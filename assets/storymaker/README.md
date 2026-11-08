# Storymaker illustration library

Twelve fixed characters, twelve day/night environments and eighty props, illustrated with ImageGen during development. Three-view bitmap attachments, facial/hand swaps and seated replacements share the existing thirteen-bone skeleton and animation names. The delivered catalog is `storymaker-catalog/2`; projects remain `storymaker/1`.

`AGENTS.md` owns production; [ASSET-CONTRACT.md](ASSET-CONTRACT.md) defines the exact schema, atlas conventions, budgets, rights and review workflow. `production.json` records all 123 generated originals with prompts, references, dimensions, byte sizes and hashes. Ordinary builds use retained sources offline.

## Build and review

`node scripts/build-storymaker-catalog.mjs` writes the catalog and sixteen locale tables; `--check` verifies deterministic output without writing. `--review <dir>` renders real PNG previews with local Chrome. After changing rig artwork, run `node scripts/test-storymaker-raster-browser.mjs --previews --canvas` and rebuild the catalog. Commit masters, metadata and generated output together.

`node scripts/build-storymaker-vendor.mjs [--check]` bundles the exact PixiJS 8.22.0 and GSAP 3.15.1 lock versions with all included notices. Preview, code scenes and export share the renderer. The renderer prefers WebGL and falls back to Pixi Canvas; GSAP is advanced only by explicit frame-time sampling.

## Source layout

| Path | Responsibility |
| --- | --- |
| `masters/`, `production.json` | Unmodified ImageGen originals, references, prompts and rendered previews |
| `lib/raster-catalog.mjs`, `raster-validate.mjs` | Catalog v2 assembly, calibrated regions, bounds and integrity |
| `lib/png.mjs` | Deterministic CRC-checked PNG decoding, slicing, packing and encoding |
| `lib/raster-names.mjs`, `names.mjs` | All sixteen translated catalog tables |
| `lib/dims.mjs`, `rig.mjs`, `motion.mjs`, `stage.mjs` | Compatible skeleton, five body types, motion and stage definitions |
| `lib/glyphs.mjs`, `sfx.mjs`, `sfx/` | Shared font coverage and retained audio takes |
| Remaining vector generators and `test/` | Legacy v1 fixtures and geometry/motion compatibility tests |

Delivery below `ui/img/storymaker` contains JSON descriptors, transparent PNG atlases, day/night backgrounds, actual rendered PNG thumbnails and the unchanged WAV takes. Every file except `catalog.json` is listed with exact size, SHA-256 and image dimensions. Budget: 256 MiB, 2000 files, 8 MiB per file; atlases at most 2048², backgrounds at most 3840 × 2160, thumbnails at most 256². Fonts remain in the shared UI font inventory with their existing OFL notices.

The original illustration masters were generated for AuraGo, without downloaded/traced third-party artwork or branded characters. Their generated status and source record must remain available. Shared font and sound rights remain separate; see below and `licences/THIRD_PARTY_LIBRARIES.md`.

## Sound effects

`sfx/*.wav` holds 25 takes of seven sounds. They are not downloads. Each take was rendered once at development time with
earshot 0.4.0 (git 591fa3e, bank forge_v2r6+forge04r1+fit1+pluck1). `scripts/build-storymaker-catalog.mjs` copies them
to `ui/img/storymaker/sfx/` and never renders, so the build needs no Python. The committed WAV is the source of truth,
because earshot promises byte-identical renders only on the machine that made them.

All takes are 48 kHz, mono, PCM16. Each take is normalised on its own: the loudest 50 ms (K-weighted, earshot's
`kit.loud_lufs`) go to -18 LUFS, with a sample-peak ceiling of -1.5 dBFS. Takes that hit the ceiling stay below -18 LUFS
(`click_2`, `click_4`, `typing_2`, `typing_5`, `typing_6`). The measured values of every take are in `SFX` in
`lib/catalog.mjs`.

| Sound | Takes | Length | Align | Recipe | Origin |
| --- | --- | --- | --- | --- | --- |
| whoosh | 4 | 477 ms | peak | `fit/whoosh.short/bamboo_swish` | fit of "Short Whoosh, 13x" by Kinoton, [fs-427979](https://freesound.org/people/Kinoton/sounds/427979/), CC0 1.0 |
| pop | 4 | 200 ms | peak | `pop/0` | forge, code-made, no recording |
| click | 4 | 132 ms | peak | `fit/click/floss_lid` | fit of "Plastic Clicks.flac" by qubodup, [fs-167829](https://freesound.org/people/qubodup/sounds/167829/), CC0 1.0 |
| ding | 3 | 900 ms | peak | `fit/bell/small_bell`, `dur` 0.9 | fit of "Small Bell #2" by steffcaffrey, [fs-452379](https://freesound.org/people/steffcaffrey/sounds/452379/), CC0 1.0 |
| success | 2 | 1120 ms | onset | three `fit/bell/small_bell` layers: G7 at 0 ms (`dur` 0.8), B7 at 110 ms (`dur` 0.8), D8 at 220 ms (`dur` 0.9) | same recording as ding |
| error | 2 | 500 ms | onset | `pro_zip/down`, `dur` 0.5 | forge, code-made, no recording |
| typing | 6 | 277 to 337 ms | peak | `typing_key/thock`, one key per take | forge, code-made, no recording |

Take k, counted from 0, renders every layer on the bank seed of its sound plus k. A take that fails `sfx.check` renders
again on seed + 1000; this happened once each for `error_2` and `typing_3`. The bank fixes `vary` at 0.3 for all fits.
The builder and its input are `build_set.py` and `set/recipe.json` in
`docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/sfx/`.

Rights: earshot's MIT licence covers its code and says nothing about rendered output, so the rights reading of the
renders stays open. A fit stores the structure of a recording, not its samples, and keeps the recording's licence.

Nobody has listened to this set. `error` reads synthetic (p_real 0.01) and `success` is uncertain (p_real 0.6).

`test/sfx.test.mjs` compares the files with this table.

| File | Bytes | SHA-256 |
| --- | --- | --- |
| `sfx/whoosh_1.wav` | 45838 | 27838ddd5ab530b5b9fa6302f54cd84e406cf47f637ccdaa7f6b006ae8855d9c |
| `sfx/whoosh_2.wav` | 45838 | c51fb7c3d426e83119c633690b8ea863950b213c9933ef70da0abd11d20b7da4 |
| `sfx/whoosh_3.wav` | 45838 | f986c2a7922977bb5eb967fcd3114d81dc537427eb26d802fc3c51b8fde166c2 |
| `sfx/whoosh_4.wav` | 45838 | bb807db30dcdf242dd2d4aab53fa06f170c4c7d7e916f5ae1f3b4e2c01c6f992 |
| `sfx/pop_1.wav` | 19244 | 8a2cfe4907ecfab19daaa322d690a714badb41baf5e1d94aa27e64bd63a7dc03 |
| `sfx/pop_2.wav` | 19244 | 8a076996ed39ccaf00313f42fa8073c70d08bb7a94201dedf15d8d876e68fa90 |
| `sfx/pop_3.wav` | 19244 | fba9b910c34ea1b9d7f545905d1c08ce1852d7ec0813eba1cc5d3d46782d1d25 |
| `sfx/pop_4.wav` | 19244 | 9e45e2014c9e886d8c83b32ae49ae7d3c0f0cd4ed28cd3a659cf8853b0c51e89 |
| `sfx/click_1.wav` | 12718 | cae0cc035fb73067600fb189a921e811a0d164de4ee242d863a0da0b5e4369e9 |
| `sfx/click_2.wav` | 12718 | 10f7ab91f3f30788903a389edff802dd4df5f1a40f7f0e117440f74d77cf08aa |
| `sfx/click_3.wav` | 12718 | 9d2fd15d88e96168529138d7607d49c7a37b52047725fe1653cbb03b6366f59b |
| `sfx/click_4.wav` | 12718 | 80b08cba555608cd01187f8832eeba3af5355a3fcf8e83d9e1338590d1e58e6f |
| `sfx/ding_1.wav` | 86444 | 5774818d2c128a5b70fb09af9c4ec13f03b128185d5d0be3d6adb3783f1ba614 |
| `sfx/ding_2.wav` | 86444 | caee5f9b0668f0d3ba66b828ce48e4e2f1d716149b6ccac0c0856f4d44ea8b0a |
| `sfx/ding_3.wav` | 86444 | 5ca184501d5ddf822ec0a02c8065bec4006b56277d1b80901e5ed281b9f5a43f |
| `sfx/success_1.wav` | 107564 | a74b35b96b1361fe27809508fc5df08d5269af73832fb5c48785ca03c19956fc |
| `sfx/success_2.wav` | 107564 | b619063209fc250dfd4f50f6894f921398a0b47e7bce2a5d21f702fb842b636c |
| `sfx/error_1.wav` | 48044 | b12574f568b88f54312ff86d490de82823b980d5c6c7ba732104721bbaeb3a45 |
| `sfx/error_2.wav` | 48044 | c07dcc6f77eba83c98e91f3038db00454e4a3eb554485e3e07626296f833c449 |
| `sfx/typing_1.wav` | 32350 | e5a4ed4a4eec9f6f547d57def6ec6714c7abb34817f37c57861c14bf3d0a26d9 |
| `sfx/typing_2.wav` | 29950 | ede7b0c80172fe7ee778982d890e5cc59527b3806d76f62f1482df375e37b87f |
| `sfx/typing_3.wav` | 29614 | 483a4ff4fc7176c381566473b770a2370f13179629171012d448f5eb0060cdff |
| `sfx/typing_4.wav` | 28696 | e54bcc3c5a8013f4a6381a37767e587afad0cf6ee98e97eb3d38e97af3be7ac3 |
| `sfx/typing_5.wav` | 26584 | e0a8bb52be6c30f4899241f615d35e1cb3dbd8eaecc581880d110097cb0f2280 |
| `sfx/typing_6.wav` | 29578 | 5545db8665c0e3d2328b0564ae40c0f11615292997ab3d2338d18eef66fb2ea5 |

## Verification

- `npm run test:storymaker`: compiler, timing, sandbox, text, speech and v1 compatibility; real GSAP seek/disposal, bitmap trust and planted gait checks.
- `node --test assets/storymaker/test/*.test.mjs`: retained skeleton/motion and v1-generator checks.
- `go test ./ui -run TestStorymakerCatalog -count=1`: delivered v2 inventory, regions, names, bounds and hashes.
- `node scripts/test-storymaker-raster-browser.mjs --canvas` and `--webgl --loss`: native three-format/code-scene, seek/reload and context-recovery acceptance.
- `--matrix --canvas`: every character, perspective, facing, pose, gesture and expression. `--environments --canvas`: all day/night backgrounds in all formats. Review output belongs under ignored `ui/reports`.
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run TestRasterReferenceExportBrowser`: real 30-second MP4, subtitles and library copy. Fixture audio is a calibration tone.
