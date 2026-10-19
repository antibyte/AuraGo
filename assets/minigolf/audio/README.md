# AuraGolf audio sources

The recorded sound effects and the four music loops of AuraGolf come from
public-domain (CC0 1.0) works. This directory documents where they come from
and how the shipped files in `ui/3d/minigolf/audio/` are produced. The original
downloads are not committed; the shipped Ogg Vorbis files are reproducible from
them. Ambience, interface sounds and reverb impulse responses are synthesized
at runtime and need no source material.

- Recipe and provenance data: `assets/minigolf/audio/sources.json` (the only
  hand-edited input: sources, per-group members, trims, fades, loop regions).
- Build and verification script: `scripts/build-minigolf-audio.mjs`.
- Generated pack (committed): `ui/3d/minigolf/audio/sfx/*.ogg`,
  `ui/3d/minigolf/audio/music/*.ogg` and `ui/3d/minigolf/audio/manifest.json`.
  The manifest repeats the provenance of every shipped file (source,
  archive member, size, SHA-256, duration) and is what the game reads at runtime.
- License of all source works: [CC0 1.0 Universal](https://creativecommons.org/publicdomain/zero/1.0/)
  (SPDX `CC0-1.0`). Kenney's `License.txt` and the ezwa `info.txt` state
  CC0/public domain; every OpenGameArt page lists CC0.

## Provenance

| Used for | Work | Author | Page | Download | License | Source SHA-256 |
| --- | --- | --- | --- | --- | --- | --- |
| `strike`, `impact.wood.*`, `impact.stone`, `impact.metal.*`, `impact.soft.*`, `cup.rim`, `cup.rattle` | Impact Sounds | Kenney | https://kenney.nl/assets/impact-sounds | https://kenney.nl/media/pages/assets/impact-sounds/87b4ddecda-1677589768/kenney_impact-sounds.zip | CC0-1.0 | `029d734af1582474edf3a694d1b0cebc97c1c152f2f39fa34d4c2bafc5de77f8` |
| `cannon.boom`, `boost`, `teleport` | Sci-Fi Sounds | Kenney | https://kenney.nl/assets/sci-fi-sounds | https://kenney.nl/media/pages/assets/sci-fi-sounds/6b296f9ecf-1677589334/kenney_sci-fi-sounds.zip | CC0-1.0 | `119340f351a5098ad814f78719438c0da355a9ce8a4c8a3af6a8d48aa3d49e04` |
| `win.pirate`, `win.temple`, `win.moon` | Music Jingles | Kenney | https://kenney.nl/assets/music-jingles | https://kenney.nl/media/pages/assets/music-jingles/f37e530b9e-1677590399/kenney_music-jingles.zip | CC0-1.0 | `b729ba57959bd58793d2c5cafa348aaf2655d354f3da35ec4729e03ec77197b8` |
| `roll.hard` | Bowling Ball Rolling | qubodup | https://opengameart.org/content/bowling-ball-rolling | https://opengameart.org/sites/default/files/qubodup-bowling-roll.7z | CC0-1.0 | `3c99cfe42ecf8a87ab13f66f0f4c16d0a2bdbf75f825eb2e959baab581091af8` |
| `roll.soft` | 30 CC0 SFX Loops | rubberduck | https://opengameart.org/content/30-cc0-sfx-loops | https://opengameart.org/sites/default/files/sfx_loops.zip | CC0-1.0 | `9c013474c7e56192a0d1b2840535a1e1d8b93166948d4a9f7136bb6c3d5421cd` |
| `splash` | 6 Short Water Splashes | ezwa (cut by qubodup) | https://opengameart.org/content/6-short-water-splashes | https://opengameart.org/sites/default/files/ezwa-water_splash.7z | CC0-1.0 | `6187fa510cb7b5e8c3ac7c79f294ddc0e44d07472758bf6ecf2628a372ae9a26` |
| `cannon` | Cannon Fire | Thimras | https://opengameart.org/content/cannon-fire | https://opengameart.org/sites/default/files/cannon_fire_0.ogg | CC0-1.0 | `2b21ffa41e5c3be4eecdfbd3490544f5aa8b6d1581787265863e13de1a8e599d` |
| music `menu` | Happy Clappy Loop | OwlishMedia | https://opengameart.org/content/happy-clappy-loop | https://opengameart.org/sites/default/files/HappyClappyLoop.wav | CC0-1.0 | `1ca66b4e4d41e14a4ccfcc3b758e061cf283b9d5e332c4ea8e00f94f4deb6554` |
| music `pirate` | Pirate Adventure | Eldritch Grim | https://opengameart.org/content/pirate-adventure | https://opengameart.org/sites/default/files/pirate_adventure.wav | CC0-1.0 | `54c3bff9b139054685cd3ecb62c2da8815496df5cc8df7c8888d97891dcb5640` |
| music `temple` | Hidden Temple | Eldritch Grim | https://opengameart.org/content/hidden-temple | https://opengameart.org/sites/default/files/hidden_temple_0.wav | CC0-1.0 | `6feaea11112a772b1a4825033fd65f157309ec5216f9a2c074b1efb53954be12` |
| music `moon` | Galactic Temple | yd | https://opengameart.org/content/galactic-temple | https://opengameart.org/sites/default/files/GalacticTemple.zip | CC0-1.0 | `4713ef3fb215e51fb2bdc2a6ca8e1f399461108fc2efad63874c4df4dba91c8d` |

## Processing

Every output is decoded with ffmpeg, resampled to 44.1 kHz, trimmed, faded or
loop-baked as `sources.json` says, normalized and encoded with libvorbis. The
exact archive member of every file is listed in `sources.json` and in the
manifest.

- Sound effects: mono, leading silence removed (except the two rolling loops),
  peak-normalized to -1 dBFS before encoding at quality 4. Output names are
  `sfx/<group with dots replaced by dashes>-<n>.ogg`, `n` counting from 1.
- Music: stereo, normalized to -16 LUFS integrated (lowered if the peak would
  exceed -1 dBFS), quality 3 (`menu`), 2 (`pirate`, `temple`) or 1 (`moon`).
  Output names are `music/<id>.ogg`.
- Loops (`roll.hard` and every music track with a crossfade): for a loop region
  `[a, b)` and crossfade `C`, the file is the crossfade of `[a+C, b)` into
  `[a, a+C)` with square-root gains (constant power). The result is `b-a-C`
  seconds long and its end flows into its start, so a looping
  `AudioBufferSourceNode` is seamless. `menu` and `roll.soft` are already
  seamless and are only trimmed or re-encoded.
- Metadata is stripped and the encoder runs in bit-exact mode, so a rebuild with
  the same ffmpeg version reproduces identical files (verified with ffmpeg 8.1.1).

Budgets enforced by `--check`: all sound effects at most 2 MiB, all music at
most 6 MiB, manifest at most 128 KiB.

## Rebuild

Requirements: Node.js 22 or newer, ffmpeg 8 or newer built with libvorbis, and
bsdtar for `.zip`/`.7z` archives (on Windows the built-in
`%SystemRoot%\System32\tar.exe`; Git Bash's GNU tar cannot read 7z).

1. Download the files named in the Download column above into one folder.
   Keep their original file names (the `file` field of each source in
   `sources.json`): `kenney_impact-sounds.zip`, `kenney_sci-fi-sounds.zip`,
   `kenney_music-jingles.zip`, `qubodup-bowling-roll.7z`, `sfx_loops.zip`,
   `ezwa-water_splash.7z`, `cannon_fire_0.ogg`, `HappyClappyLoop.wav`,
   `pirate_adventure.wav`, `hidden_temple_0.wav` and `GalacticTemple.zip`.
2. Rebuild the pack from the repository root:

   ```bash
   node scripts/build-minigolf-audio.mjs --write --sources <folder>
   ```

   The script verifies the size and SHA-256 of every download first and stops
   with the expected download URL on a mismatch. Archives are listed before
   extraction (members with `..` or absolute paths are rejected) and are
   unpacked into a temporary directory outside the repository, which is removed
   afterwards. Stale `.ogg` files in `sfx/` and `music/` are replaced.
3. Verify the committed pack at any time (read-only, no ffmpeg needed):

   ```bash
   node scripts/build-minigolf-audio.mjs --check
   ```

   It checks the manifest, the size and SHA-256 of every file, the Ogg Vorbis
   headers, license and source fields, the recipe consistency, stale files and
   the size budgets. `npm run build:minigolf` runs it before bundling the game.
