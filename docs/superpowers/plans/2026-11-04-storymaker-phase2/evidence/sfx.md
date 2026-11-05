Phase 2 SFX set is done: 25 WAVs for all seven names, in `set/` under the sfx scratch folder. I could not save `report.md`, because the Write tool refused it as a report file. This message is the full text. The sheet is `set/sheet.json`, and the builder is `scripts/build_set.py` in the same folder.

Folder: `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/sfx/`. All earshot results below are from earshot 0.4.0 (git 591fa3e).

**(a) Choice and takes.** Every WAV is 48 kHz, mono, PCM16, and every take passes `sfx.check`. Candidates were ranked with `rank`, `describe` and `realness`: 9 to 24 per name.

| Name | Sound | Takes | Duration | Rank z | p_real |
|---|---|---|---|---|---|
| whoosh | `fit/whoosh.short/bamboo_swish` | 4 | 477 ms | 5.39 | 0.975 |
| pop | forge `pop/0` | 4 | 200 ms | 3.98 | 0.85 |
| click | `fit/click/floss_lid` | 4 | 132 ms | 3.1 | 0.83 |
| ding | `fit/bell/small_bell`, dur 0.9 | 3 | 900 ms | 2.55 | 0.95 |
| success | three `small_bell` notes G7, B7, D8 at 0, 110 and 220 ms | 2 | 1120 ms | 3.4 (chime tag) | 0.61–0.64 |
| error | forge `pro_zip/down`, dur 0.5 | 2 | 500 ms | 2.97 | 0.01 |
| typing | forge `typing_key/thock`, single key | 6 | 277–337 ms | 4.34 | 0.96 |

- **Click:** the forge click (`pro_click/soft`) ranked higher but read 0.11, so I took the fit.
- **Ding and success:** forge dings and `mallet/up4` read 0.04–0.13. Pitch-shifting the bell down also lowered p_real (C7: 0.31–0.47, native pitch: 0.965), so I left it at native pitch.
- **Typing:** I chose single keys because `pro_typing_run` read as "ratchet clicking". Compile has to space the keys.
- **Error:** no bank sound reads as an error by a real recording. This is the only one that reads "error buzz".

**(b) Measurements.** Full per-take table (onset, peak, LUFS-M, true peak, bytes) is in `set/sheet.json`.
- **Total size:** 1,058,520 bytes.
- **True peaks:** every take is at or below -1.0 dBTP.
- **impact_ms:** whoosh peaks at 129.5–130.5 ms, pop 3.5–4.5, click 7.5, ding 32.5, typing 11.5–13.5. For success and error the peak moves by tens of milliseconds between takes (success 138.5 vs 28.5, error 41.5 vs 9.5). I recommend onset there: success 1.2 ms, error 0.8 and 0.2.
- **Re-render:** byte-identical. 25 of 25 files matched across two separate processes, and the CLI render of `floss_lid --seed 2` matched twice. Earshot's docs only promise this on the build machine, so the committed WAV is the source of truth.
- **What decides a take:** for forge sounds, the seed alone. For fits, seed plus `vary`, which the bank fixes at 0.3 for all 15 fits and `bank.render` cannot change.
  - The bell at vary 0 gives near-identical takes (residual -28.8 dB). At 0.3 they differ (+3.3 dB).
- **Take check:** there is no CLI command for it, so I measured the same quantities myself. CLAP between takes is 0.89–0.99, and waveform residuals run from -10 to +5 dB. The takes are different waveforms but close in character, whoosh and click most alike. I did not listen, so I can't say they differ audibly.

**(c) Quality.**
- **Fixed by re-roll:** `error_2` had a comb ring (z 16) and was re-rolled on seed +1000, which cleared it. `typing_3` was re-rolled the same way.
- **Notes left in place:** `typing_4`, `typing_6` and `success` carry "tail rises" notes. These are the second key sound and the second and third bell notes.
- **Still reads synthetic or uncertain:** error (0.01), success (0.6). I could not make them read as real.
- **Not judged:** taste, whether the error reads as an error to a person, and whether the 4–6 kHz bell masks narration. No human has listened.

**(d) Recommendations.**
- **Rights:** the MIT licence in earshot's LICENSE covers the code, and no file says anything about rendered output. `docs/FIT.md` says a fit is a derivative of its recording, so keep its source licence. Four of seven sounds (whoosh, click, ding, success) derive from CC0 Freesound recordings:
  - whoosh: Kinoton, fs-427979
  - click: qubodup, fs-167829
  - ding and success: steffcaffrey, fs-452379
- **Normalisation:** per take, set the loudest-50-ms K-weighted level (earshot's `kit.loud_lufs`) to -18 LUFS, with a sample-peak ceiling of -1.5 dBFS.
  - Spread inside a sound is 1.2 LU or less. Peaks differ by sound (-1.5 dBFS for click and typing, -20 for error), by design.
  - Peak or momentary-LUFS matching fails for sounds this short. ffmpeg's momentary meter gives nothing under 400 ms, so bake levels offline.
- **Size:** 1.06 MB is fine.
- **Recipe file:** the schema is in `set/recipe.json`. For the real version, add the exact `earshot sfx render` line per layer, the bank's `source` block, the earshot and bank versions, and the sha256 of each WAV. The catalog build should copy the WAVs rather than re-render, so the build needs no Python.
- **Catalog entry:** the common fields from contract section 3 (Entries), plus these:
  - `file` (take 1)
  - `takes[{file, dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp}]`
  - `impact_ms`, `align` and `level_db`
  - `source {kind, basis, url}`
- **Code changes this implies:**
  - Go `Entry` in `internal/storymaker/catalog.go` has `File` but no takes or impact fields.
  - The `sfx` validation case checks only `file`.
  - `VerifyFiles` checks only `File` and `Thumb`, not takes.
  - Every entry needs a thumbnail, so sfx needs a generated waveform SVG or an exemption.
- **Version hash:** `versionOf` in `assets/storymaker/lib/catalog.mjs:256` hashes `lf()` of text, and the file map and the write/check paths are utf8-only. WAVs need a separate raw-Buffer path.
  - `listExisting()` walks only `rig`, `props`, `environments` and `thumbs`, so `sfx` must be added.
  - I'd add `*.wav binary` to `.gitattributes`.

**Not verified:** nothing was listened to by a person, the Go side was not run, byte identity on another machine is untested, and the output-rights reading is mine and not legal advice. Nothing was copied into the repo.