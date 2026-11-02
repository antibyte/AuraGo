# Storymaker starter catalog

Code-authored generators for the Storymaker starter catalog: one parametric human rig with four starter
characters, 23 props, four environments, poses, gestures, expressions, mouth shapes, walk and idle cycles, text
styles, transitions and two palettes. `scripts/build-storymaker-catalog.mjs` turns them into the committed files
below `ui/img/storymaker/` (format `storymaker-catalog/1`, spec section Catalog and rig).

The catalog is original work by the AuraGo project and ships under the repository licence. It contains no traced,
scanned or downloaded artwork. The default font is Geist (SIL OFL 1.1, already shipped in `ui/fonts`,
see `THIRD_PARTY_LIBRARIES.md`); the catalog only references it by path.

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
LF-normalised bytes of every file below `ui/img/storymaker/` except `catalog.json`, taken in sorted path order.
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
`thumbs/<id>.svg` (one per entry; characters are the rig posed in `stand`, three-quarter view, resolved colours),
and `ui/lang/storymaker_catalog/<lang>.json` (keys `storymaker.catalog.<id with "." replaced by "_">`).
All outputs are `.json` or `.svg` below `ui/img` and `ui/lang`, which the asset-set allowlist ships
(`assets/web-assets.json`).

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

The catalog stays within 30 MiB and 2000 files (spec); it currently is about 2.4 MiB in 105 files.
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
