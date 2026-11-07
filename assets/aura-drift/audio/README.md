# Aura Drift audio

Every sound of Aura Drift 3D is shipped as Ogg Vorbis under `ui/3d/aura-drift/audio/` and described by
`ui/3d/aura-drift/audio/manifest.json` (the file the game reads). **The eight race songs are the exception:
they are generated with YuE2 Studio, not synthesised here (see "Race songs").** Engines, the menu and results music
and most sound groups are **own synthesis** (seeded JavaScript DSP, no random numbers, no clock), so a rebuild reproduces the same
bytes. Seventeen sfx groups are approved CC0 recordings instead (hash-pinned in `sources.json`, credited in
`ui/3d/aura-drift/audio/LICENSE.txt`): `animal.cow`, `animal.sheep`, `crowd.cheer`, `tyre.squeal`, `scrape.metal`,
`explosion.small`, `impact.light`, `impact.heavy`, `impact.metal`, `crate.break`, `prop.hay`, `prop.cone`,
`prop.barrel`, `amb.sea`, `amb.forest`, `amb.machinery` and `amb.desert`. Every other group stays synthesis
(`crowd.aww`, `animal.penguin`, `animal.llama`, `explosion.big`, `prop.tyres`, `amb.wind`, `amb.city`, `amb.snow`,
weapons, stingers, horns, UI). The originals are not committed; a group is replaced with one small data change (below).

- Build and verification: `scripts/build-aura-drift-audio.mjs`
- Synthesis code: `scripts/aura-drift-synth/*.mjs` (`dsp.mjs` toolkit, `kit.mjs` shared sound parts,
  `engines.mjs`, `sfx-world.mjs`, `sfx-weapons.mjs`, `sfx-comic.mjs`, `sfx-polish.mjs` (five
  feedback stinger groups, 12 variants), `sfx-weather.mjs` (four weather groups, 11 variants),
  `music*.mjs`, `registry.mjs`)
- Song encoder: `scripts/aura-drift-song-encode.mjs`; song record: `assets/aura-drift/audio/yue2-songs.json`
- Data input: `assets/aura-drift/audio/sources.json` (`sources`: 13 hash-pinned CC0 downloads, `overrides`: 17 sfx groups)
- Contract test: `scripts/test-aura-drift-audio.mjs`

## Commands

| Command | Purpose |
| --- | --- |
| `node scripts/build-aura-drift-audio.mjs [--check]` | Read-only verification of the committed pack (no ffmpeg). Fails when a file, hash, budget or the synth scripts changed without a rebuild. |
| `node scripts/build-aura-drift-audio.mjs --write [--sources DIR]` | Renders everything with worker threads, encodes with ffmpeg 8 or newer (libvorbis, bit-exact) and replaces the pack. `--sources` is needed because `overrides` is not empty; each download sits in `DIR/<source id>/<file>` (or directly in `DIR/<file>`). Archives need bsdtar or the Windows `tar.exe`. |
| `node scripts/build-aura-drift-audio.mjs --songs DIR` | Re-encodes the eight race songs from `DIR/<biome>.flac` (the YuE2 Studio masters; size and SHA-256 must match `yue2-songs.json`), updates their manifest records and leaves every other file untouched, then verifies the pack. Needs ffmpeg 8 or newer with libvorbis. `--write` keeps the committed songs as they are (it does not need the FLACs). |
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
  music:  { "<biome>": "music/<biome>-song.ogg"  (a song)  |  { bed, drive, peak }  (three aligned stems),
            menu: "music/menu.ogg", results: "music/results.ogg" },
  files:  [ { path, bytes, sha256, seconds, channels, loop, origin, license, member? } ],
  sources: [...], overrides: [...] }
```

- `sfx`: mono one-shots and loops (`loop: true`: `scrape.metal`, `tyre.squeal`, `weapon.flame_loop`,
  `weapon.hole_loop`, all `amb.*`). Stingers and fanfares (`stinger.*`, `fanfare.*`) are stereo and live in the
  `sfx` map too, so the runtime plays them like any other group. Groups with several members are variants: pick one at random.
- `engine`: 8 cars x 4 layers, 2.000 s mono, seamless. Layer rpm: idle 1000, low 2600, mid 4600, high 6600
  (rendered at the nearest rpm that fits a whole number of engine cycles, within 30 rpm).
- `music`: a biome listed in `yue2-songs.json` is ONE stereo looping song file (`music/<biome>-song.ogg`, string entry,
  not adaptive). A biome without a song would have three stereo stems of 16 bars at the tempo of `aura-drift-music.js`,
  exactly the same length, seamless loops (object entry, adaptive drive / peak layering); today all eight biomes have
  songs, so no stems are shipped. `menu` and `results` are stereo 8-bar loops.

## Loudness

- sfx: peak -1 dBFS, engine: peak -3 dBFS. Dense, clipped sounds overshoot after lossy coding, so the
  build decodes every sfx and engine file and lowers the gain until the decoded peak honours the target.
- recordings: peak -1 dBFS like the synthesis, so a replaced group needs a new `AD.GROUP_GAIN_DB` trim in
  `ui/js/desktop/apps/aura-drift-audio-bank.js`: old trim plus the active-RMS difference between the old synthesis and the recording
  (frames within 30 dB of the loudest 20 ms frame), capped at +4 dB. `impact.heavy` hits that cap and sits about 0.3 dB under its target.
- music: **one gain per biome**. Songs: chosen so that the looped song measures -16 LUFS integrated, limited so the decoded
  peak stays at or under -1 dBFS (`glacier` and `industry` are peak-limited and land at -16.6 LUFS); the gain is recorded as
  `gainDb` in the file's manifest entry. Stems (not used at the moment): the three stems playing together measure -16 LUFS,
  relative balance kept.

## Race songs (YuE2 Studio)

- Origin: `origin: "YuE2 Studio (local generation)"`, instrumental (empty lyrics, chain of thought "full", 120 s requested).
  Title, style prompt and BPM of every song are in `yue2-songs.json` and copied into the manifest entry together with the
  SHA-256 of the FLAC master. The 48 kHz stereo FLAC masters (15-24 MB each) are **not committed**; keep them outside the repo.
- **OPEN ITEM, licence not verified:** the model and licence terms of YuE2 Studio for commercial or game use are not known. The
  manifest marks the files `license: "unverified"`. Do not release the pack publicly or commercially until the terms are checked
  or the songs are replaced. `ui/3d/aura-drift/audio/LICENSE.txt` says so as well.
- Encoding (`aura-drift-song-encode.mjs`): decode to float stereo 48 kHz, trim digital silence at both ends (10 ms windows under
  -50 dBFS, 20 ms margin), then bake the loop: the last `loopFade` = 3.5 s of the song is blended (equal power, cos / sin) with
  its first 3.5 s and the file starts after those 3.5 s, so the wrap point runs from the blended end straight into the
  continuing song (sample-continuous, no click; the intro is heard only inside the blend). One gain per song (see Loudness),
  `libvorbis -b:a 96k`, bit-exact, no metadata. 96 kbps is what fits the 10 MiB music budget with all eight songs (about 9.7 MB
  with menu and results); 112-128 kbps would need about 11-12 MB.
- The song scene replaces the stems in `aura-drift-music.js`: `pack.scene(biome)` returns `{ bed, adaptive: false }`; the
  adaptive drive / peak layering and the disco low-pass do not apply; stingers, ambience beds, ducking, crossfades, view changes
  and the live-synth fallback (when the file fails to load) work as before.

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
source in `sources` must list `license: "CC0-1.0"`. Overrides apply to `sfx` groups only; engines and the menu / results music
stay own work (dynamic stems need exact alignment). The race songs are the one non-CC0 input and are recorded in `yue2-songs.json`.

## Budgets (enforced by `--check`)

sfx at most 4 MiB, engine at most 1.5 MiB, music at most 10 MiB, manifest at most 128 KiB.
