# Aura Drift audio

Every sound of Aura Drift 3D is shipped as Ogg Vorbis under `ui/3d/aura-drift/audio/` and described by
`ui/3d/aura-drift/audio/manifest.json` (the file the game reads). Engines, music and most sound groups are
**own synthesis** (seeded JavaScript DSP, no random numbers, no clock), so a rebuild reproduces the same
bytes. Seventeen sfx groups are approved CC0 recordings instead (hash-pinned in `sources.json`, credited in
`ui/3d/aura-drift/audio/LICENSE.txt`): `animal.cow`, `animal.sheep`, `crowd.cheer`, `tyre.squeal`, `scrape.metal`,
`explosion.small`, `impact.light`, `impact.heavy`, `impact.metal`, `crate.break`, `prop.hay`, `prop.cone`,
`prop.barrel`, `amb.sea`, `amb.forest`, `amb.machinery` and `amb.desert`. Every other group stays synthesis
(`crowd.aww`, `animal.penguin`, `animal.llama`, `explosion.big`, `prop.tyres`, `amb.wind`, `amb.city`, `amb.snow`,
weapons, stingers, horns, UI). The originals are not committed; a group is replaced with one small data change (below).

- Build and verification: `scripts/build-aura-drift-audio.mjs`
- Synthesis code: `scripts/aura-drift-synth/*.mjs` (`dsp.mjs` toolkit, `kit.mjs` shared sound parts,
  `engines.mjs`, `sfx-world.mjs`, `sfx-weapons.mjs`, `sfx-comic.mjs`, `sfx-polish.mjs` (five
  feedback stinger groups, 12 variants), `music*.mjs`, `registry.mjs`)
- Data input: `assets/aura-drift/audio/sources.json` (`sources`: 13 hash-pinned CC0 downloads, `overrides`: 17 sfx groups)
- Contract test: `scripts/test-aura-drift-audio.mjs`

## Commands

| Command | Purpose |
| --- | --- |
| `node scripts/build-aura-drift-audio.mjs [--check]` | Read-only verification of the committed pack (no ffmpeg). Fails when a file, hash, budget or the synth scripts changed without a rebuild. |
| `node scripts/build-aura-drift-audio.mjs --write [--sources DIR]` | Renders everything with worker threads, encodes with ffmpeg 8 or newer (libvorbis, bit-exact) and replaces the pack. `--sources` is needed because `overrides` is not empty; each download sits in `DIR/<source id>/<file>` (or directly in `DIR/<file>`). Archives need bsdtar or the Windows `tar.exe`. |
| `node scripts/build-aura-drift-audio.mjs --preview DIR [--only REGEX]` | Writes normalised float WAVs for listening or analysis, nothing in the repo changes. Job ids look like `sfx:weapon.horn_blast:0`, `engine:vektor:mid`, `music:alpine`. |
| `node scripts/build-aura-drift-audio.mjs --list` | Prints every job id. |
| `node scripts/test-aura-drift-audio.mjs [--fast]` | Manifest groups against the runtime contract, engine set, aligned music stems, budgets, decoded peak and loudness checks. |

`--write` overwrites the committed `.ogg` files in place and removes only files that no recipe
produces any more. A rebuild therefore never mass-deletes the shipped pack, and a failed run
cannot leave it half empty.

## Pack layout

Output names match `^(sfx|engine|music)/[a-z0-9-]+\.ogg$`.

```
{ version: 1, synth: <sha256 of the synth scripts>,
  sfx:    { "<group>": ["sfx/<group with . and _ as ->-<n>.ogg", ...] },
  engine: { "<carId>": { idle, low, mid, high } },
  music:  { "<biome>": { bed, drive, peak }, menu: "music/menu.ogg", results: "music/results.ogg" },
  files:  [ { path, bytes, sha256, seconds, channels, loop, origin, license, member? } ],
  sources: [...], overrides: [...] }
```

- `sfx`: mono one-shots and loops (`loop: true`: `scrape.metal`, `tyre.squeal`, `weapon.flame_loop`,
  `weapon.hole_loop`, all `amb.*`). Stingers and fanfares (`stinger.*`, `fanfare.*`) are stereo and live in the
  `sfx` map too, so the runtime plays them like any other group. Groups with several members are variants: pick one at random.
- `engine`: 8 cars x 4 layers, 2.000 s mono, seamless. Layer rpm: idle 1000, low 2600, mid 4600, high 6600
  (rendered at the nearest rpm that fits a whole number of engine cycles, within 30 rpm).
- `music`: per biome three stereo stems of 16 bars at the tempo of `aura-drift-music.js`, exactly the same length
  (same sample count), seamless loops. `menu` and `results` are stereo 8-bar loops.

## Loudness

- sfx: peak -1 dBFS, engine: peak -3 dBFS. Dense, clipped sounds overshoot after lossy coding, so the
  build decodes every sfx and engine file and lowers the gain until the decoded peak honours the target.
- recordings: peak -1 dBFS like the synthesis, so a replaced group needs a new `AD.GROUP_GAIN_DB` trim in
  `ui/js/desktop/apps/aura-drift-audio-bank.js`: old trim plus the active-RMS difference between the old synthesis and the recording
  (frames within 30 dB of the loudest 20 ms frame), capped at +4 dB. `impact.heavy` hits that cap and sits about 0.3 dB under its target.
- music: **one gain per biome**, chosen so that the three stems playing together measure -16 LUFS (peak limited to -1 dBFS).
  Stems keep their relative balance; `bed` alone is therefore quieter than the full mix, as intended for dynamic layers.

## Synthesis notes

- Engines: a train of firing events (one pulse per cylinder with its own amplitude and timing error, crank-angle
  order, bank layout) driven through exhaust-pipe resonators, plus diesel knock, intake noise, valve ticks, turbo whistle
  and a mechanical wash. Per car class: wedge turbo (four-cylinder, whistle), GT six (smooth, intake rasp), muscle V8
  (cross-plane order over two banks), rally boxer (unequal-length headers), van diesel (knock), mower two-stroke (one firing
  per revolution, idle misfires, blade whirr), sofa electric whine with spring squeaks, truck big diesel.
  Everything is computed circularly, so the 2 s loop closes without a crossfade.
- Weapons and comic sounds are built from formant voices (duck, cow, sheep, crowd), modal bells, plucked strings and noise.
- Music: sixteenth-note arranger with swing, sidechain pumping derived from the kick pattern (applied to pads, bass, arps and
  stabs in every stem), drum synthesis (kick, snare, clap, hats, toms, darbuka, anvil), filtered supersaw pads, bass voices,
  reeds, plucks, bells and a lead hook per biome with eight two-bar slots over 16 bars. Reverb and delay tails wrap around the loop.

## Replacing a group with an approved CC0 recording

Add the download to `sources` (hash-pinned, same fields as `assets/minigolf/audio/sources.json`) and one entry to `overrides`:

```json
{ "sources": [ { "id": "kenney-impact", "title": "Impact Sounds", "author": "Kenney", "page": "https://kenney.nl/assets/impact-sounds",
                 "download": "https://...", "file": "kenney_impact-sounds.zip", "bytes": 800850, "sha256": "...", "license": "CC0-1.0",
                 "licenseUrl": "https://creativecommons.org/publicdomain/zero/1.0/" } ],
  "overrides": [ { "group": "impact.metal", "source": "kenney-impact", "members": ["Audio/impactMetal_light_000.ogg", "Audio/impactMetal_light_001.ogg"],
                  "start": 0, "end": 1.2, "fadeOut": { "start": 0.9, "duration": 0.3 }, "crossfade": 0, "trimSilence": true, "loop": false } ] }
```

Then run `node scripts/build-aura-drift-audio.mjs --write --sources <dir with the downloads>`. For that group the recording wins
over the synth recipe: members become `sfx/<group>-<n>.ogg` (mono, 44.1 kHz, peak -1 dBFS, optional trim, fades or a baked equal-power
loop crossfade exactly like AuraGolf) and the manifest records source, archive member and license. Archive members are checked for
absolute or `..` paths, and the download must match the pinned size and SHA-256. Only explicitly CC0 works are acceptable; every
source in `sources` must list `license: "CC0-1.0"`. Overrides apply to `sfx` groups only; engines and music stay own work
(dynamic stems need exact alignment).

## Budgets (enforced by `--check`)

sfx at most 4 MiB, engine at most 1.5 MiB, music at most 10 MiB, manifest at most 128 KiB.
