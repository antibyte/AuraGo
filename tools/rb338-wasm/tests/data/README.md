# Test songs

Copied unchanged from OpenRB-338 (commit 0bebfcdfd544b3cdea14536f19fb28a8de314df5),
`tests/songs/diag/songs/`. Licence: GPL-3.0-only, like the rest of this crate.

Their reference WAVs are those of the OpenRB-338 corpus manifest (`tests/refs/manifest.json`,
captured with rbhost from the original ReBirth DLL). `wav_sha256` is the SHA-256 of the whole WAV
file (44-byte header plus 176400 stereo frames) after 9088 idle blocks.

| song | wav_sha256 | frames | idle blocks |
|---|---|---|---|
| `909_bass.rbs` | `62b37761b09a6cc439a2dd8432a78a0c3444dc33e196138c0b8792086878c80c` | 176400 | 9088 |
| `303_slide.rbs` | `558cc7b9d788e07e2784ade269401051271ff8026a966fc89c9bc071a203d59a` | 176400 | 9088 |
