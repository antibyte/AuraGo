# Goldfish Adventure Production Assets

## Purpose

Original artwork and sound for **Bitte nicht das Universum füttern**. The approved
scope lives in `../../plans/2026-11-06-goldfish-adventure.md`; runtime ownership is
`../../ui/js/desktop/apps/goldfish-adventure/AGENTS.md`.

## Ownership

- `concepts/` retains the approved scene and character identity references.
- `masters/`, the prompt records and source manifests own ImageGen outputs.
- `art-layout.json` owns grid layouts, frame clips, actor placement, conditions,
  foreground/state layers, room audio and authored cinematic beats.
- `build-assets.ps1` exports unscaled PNG copies/cells and frame pivots into
  `ui/img/goldfish-adventure/`, then writes its runtime manifest.
- `build-audio.py`, `audio-takes.json` and committed instrument takes own the
  original score. Audio notices retain Earshot MIT and reused recording terms.
- `music-templates/` owns the YuE2 Studio prompts (title, style, ABC score header,
  lyrics) for one original song per room. They are source material: generated takes are
  local masters, recorded in `audio-notices.txt` with their licence terms before any use.

## Local Contracts

- Generate original scenery, characters, props, inventory art, icons and
  decorative interface imagery with the built-in ImageGen tool. Preserve exact
  prompts and reference relationships. Localized text stays outside bitmaps.
- Reference the approved R0ST/Bruno identity for poses. R0ST's right clamp and
  left hand remain anatomical; do not mirror his walking directions.
- Preserve alpha and native dimensions. Atlas slicing/copying does not upscale
  or repaint. The NPC atlas uses authored `rowEdges` because generated rows are
  not evenly spaced; inspect the cropped cells for neighboring fragments.
  Some previews display hidden RGB; inspect decoded alpha as well
  as the composited scene before accepting a cutout.
- Scene plates contain no baked actors. Runtime actors, condition-controlled
  state plates and props must align with the authored hotspot coordinates.
- Cinematic beats are visual consequences of already committed state. They
  cannot award/consume items. Every sequence pauses, skips and disposes safely;
  reduced motion retains key poses without movement.
- Art and sound are local runtime resources. Masters and source recordings are
  excluded from the UI package. No franchise artwork or music is reused.

## Work Guidance

- Generate and inspect references before deriving poses. Review output at
  gameplay size, with its real alpha, baseline and scene lighting.
- Run `./assets/goldfish-adventure/build-assets.ps1` on Windows with PowerShell 7
  and System.Drawing. Metadata records measured dimensions and source hashes.
- Audio rebuilding uses Python with numpy/soundfile and FFmpeg, reading committed
  Earshot 0.4 takes. Sound generation/selection is checked with the local Earshot
  tools; generated score quality still needs human listening.
- The production estimate of 90–120 clips is not an achieved metric. Report
  actual frame/clip/moment counts from the shipped manifest.

## Verification

- `node scripts/verify-goldfish-assets.mjs` checks PNG decoding/alpha, source
  hashes, paths, atlas bounds, staging/clip references, sixteen locale keysets,
  placeholders and the runtime payload limit.
- `node scripts/test-goldfish-animation.mjs` checks pause, skip, replacement,
  disposal, reduced motion, per-frame sources and immutable gameplay rewards.
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestGoldfish` checks the real
  Desktop opening, first puzzle, themes and close/reopen behavior.
- Inspect final room captures and full-speed motion. Automated checks do not
  establish the 3–4 hour duration, humor, replay appeal or native-language quality.

## Child DOX Index

None.
