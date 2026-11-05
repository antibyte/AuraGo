# Storymaker starter catalog

Code-authored generators for the Storymaker starter catalog: one parametric human rig with four starter
characters, 23 props, four environments, poses, gestures, expressions, mouth shapes, walk and idle cycles, text
styles, transitions and two palettes. `scripts/build-storymaker-catalog.mjs` turns them into the committed files
below `ui/img/storymaker/` (format `storymaker-catalog/1`, spec section Catalog and rig).

The catalog is original work by the AuraGo project and ships under the repository licence. It contains no traced,
scanned or downloaded artwork. The default font is Geist (SIL OFL 1.1, already shipped in `ui/fonts`,
see `THIRD_PARTY_LIBRARIES.md`); the catalog only references it by path. The sound effects have their own
provenance, see Sound effects.

## Run

```bash
node scripts/build-storymaker-catalog.mjs                 # write ui/img/storymaker/** and ui/lang/storymaker_catalog/*.json
node scripts/build-storymaker-catalog.mjs --check         # exit 1 when a committed output differs from the generators
node scripts/build-storymaker-catalog.mjs --review <dir>  # review sheets as SVG and PNG (headless Chrome; --chrome <path>)
node --test assets/storymaker/test/geometry.test.mjs assets/storymaker/test/parts.test.mjs \
  assets/storymaker/test/rig.test.mjs assets/storymaker/test/motion.test.mjs assets/storymaker/test/props.test.mjs \
  assets/storymaker/test/environments.test.mjs assets/storymaker/test/catalog.test.mjs
go test ./ui -run TestStorymakerCatalog -count=1       # schema checks on the committed files
```

The build is deterministic: no clocks, no randomness outside seeded generators (`rng`), LF line endings, numbers
rounded to one decimal. `catalog.json` carries `version`, the first 12 hex digits of the SHA-256 over the
bytes of every file below `ui/img/storymaker/` except `catalog.json`, taken in sorted path order: text files
LF-normalised, WAV files raw.
Run the build after every generator change and commit the outputs together with it.
`--review` writes `cast-sheet`, `variety-sheet` and `env-prop-sheet` (keep them outside the repository, for
example in the git-ignored `disposable/`).

## Layout

| Path | Role |
| --- | --- |
| `lib/util.mjs` | path builders, shape helpers (`fill`, `shade`, `light`, `ribbon`, `limb`), seeded `rng` |
| `lib/pathdata.mjs` | strict SVG path reader used by the validator and the tests |
| `lib/svg.mjs` | the section 3a renderer (bones, slots, variants, tokens) that draws thumbnails and review sheets |
| `lib/dims.mjs` | rig proportions and the five body types |
| `lib/headgeo.mjs`, `face.mjs`, `hair.mjs`, `extras.mjs` | head surface model, skull, eyes, brows, mouths, hair, glasses, facial hair, accessories |
| `lib/body.mjs`, `clothes.mjs`, `rig.mjs` | torso, limbs, hands, shoes, garments, the assembled `rig/human.json` |
| `lib/motion.mjs` | poses, gestures, walk and idle cycles |
| `lib/props.mjs`, `environments.mjs` | the 23 props and the four environments with spots and aspect overrides |
| `lib/names.mjs` | display names in the 16 locales |
| `lib/catalog.mjs`, `validate.mjs`, `review.mjs` | manifest, thumbnails and hash; build-time validation; review sheets |
| `test/*.test.mjs` | generator tests (`node --test`) |

Outputs: `catalog.json`, `rig/human.json`, `props/<name>.json`, `environments/<name>.json`,
`sfx/<name>_<k>.wav` (byte copies of `assets/storymaker/sfx`),
`thumbs/<id>.svg` (one per entry except sfx; characters are the rig posed in `stand`, three-quarter view, resolved colours),
and `ui/lang/storymaker_catalog/<lang>.json` (keys `storymaker.catalog.<id with "." replaced by "_">`).
All outputs are `.json`, `.svg` or `.wav` below `ui/img` and `ui/lang`, which the asset-set allowlist ships
(`assets/web-assets.json`).

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

## Style rules

- Flat vector explainer look: clean rounded shapes, no outlines on silhouettes, a limited palette. Depth comes from
  one second tone (`shade`: the outline colour at low opacity) and an occasional light tone (`light`: white at low
  opacity), never from gradients on characters. Facial line work uses `$outline`.
- Characters use the tokens `$skin $hair $top $bottom $shoes $outline $eye_white $iris $lip`; props and
  environments use palette tokens (`$bg $bg2 $fg $accent $accent2 $text $outline`) plus a few fixed neutrals so they
  read on both palettes. Environments add local tokens defined for both `day` and `night`; the night variant may
  override `bg` and `bg2`.
- No external assets, fonts or images; no `<image>`, `<script>` or `href` in thumbnails.
- Strokes are avoided for anything that needs round caps: line work is drawn as filled ribbons (`ribbon`). The
  remaining strokes assume round caps and joins.

## Budgets

The catalog stays within 30 MiB and 2000 files (spec); it has 134 files, 25 of them WAVs (1,058,520 bytes).
`TestStorymakerCatalogFilesVersionAndBudget` enforces both limits and that every file below `ui/img/storymaker`
is referenced by `catalog.json`.

## Rig conventions (section 3a additions)

- Views face right. `_r` is the character's right limb: in the front view it is on the screen right (x > 0); in
  `three_quarter` and `profile` it is the near limb, drawn in front of the torso. Negative angles swing a limb
  towards +x (forward when facing right). Bone local frames are parallel to rig space at angle 0; limbs extend along
  local +y, torso, neck and head along local -y.
- Hands hang at local `y = 135` of the lower arms and feet at `y = 205` of the lower legs, for every body type, so
  body variants never move the end parts. Body types differ through rest offsets, shapes keyed by `body`, and `scale`.
- The slot `param` for back hair is `hair` (the same key as the front hair); no slot uses `hair_back` or `skin`.
- `pose.offset_y` (rig units, positive = down) shifts the root like a cycle's `offset_y`; seated poses use
  `rig.seat_drop` (215) so that the feet stay on the floor line.
- A shape draws its `fill` (or gradient) if present and its `stroke` if present; a shape with none of them is invalid.
  Gradient stop colours may be `$tokens`.
- Environment layers are listed in draw order (back to front); `parallax` is per layer and not monotonic.
- Aspect overrides: a 9:16 or 1:1 output shows a slice of the 16:9 design frame as wide as the output at the design
  height (31.6 % and 56.3 % of the width). `aspects[...].focus_x` is the slice centre and `spots` replaces the spots
  that would fall outside it.
- The `child` body is a small adult-proportioned figure; child-specific heads and garments are catalog expansion work.

## Adding to the catalog

1. Add the generator code (a prop in `lib/props.mjs`, a character in `CHARACTERS` of `lib/catalog.mjs`, ...).
2. Add its names to `lib/names.mjs` (all 16 languages, real translations) and the required id to `lib/validate.mjs`
   and `ui/storymaker_catalog_test.go` when it is part of a contract list.
3. Run the build, look at the review sheets, run the tests, commit generator and outputs together.
