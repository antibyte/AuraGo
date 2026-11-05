Report not saved: the Write tool refused `report.md` ("subagents should return findings as text"), so I did not work around it. The full detail is below.

Base folder T = `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/tts/`

**What ran**
- **sanotts:** ran through `tools.TTSSynthesizeInMemoryContext` for de, en, fr, es. The first call installed a pip venv under `T/data/sanotts` (156 MB on disk) and needed the sandbox off.
- **phonon:** the Windows `ptts.exe` in `deploy/phonon` loads the model but fails on every output path with `cannot write …: Zugriff verweigert (os error 5)`. So `TTSSynthesizeInMemoryContext(phonon)` fails on this box; I did not find the cause.
  - I made the phonon samples with the same pinned linux `ptts` in my own `--rm` golang container, deploy/phonon mounted read-only (`T/phonon-all.sh`). That is the same CLI as `internal/phonon/synthesize.go`, but not the Go path.
- **google (keyless):** 3 short calls (de, en, fr), added only to get an MP3 sample.
- **Not run:** piper and supertonic, since both need a docker sidecar and no image is local.
- **(b) aurago-test:** `ssh` timed out on port 22 (4 tries, ping 100% loss). So the active provider and voice, installed voices and existing audio there are unknown.
- Existing local files: 3 google MP3s in `data/tts` plus one music MP3.

**(c) Formats** (samples in `T/samples/`)

| source | codec | rate | ch | format | bytes/s |
|---|---|---|---|---|---|
| phonon de/en/fr/es (8 voices) | WAV PCM | 24000 | 1 | s16 | 48000 |
| sanotts de/fr/es | WAV PCM | 22050 | 1 | s16 | 44100 |
| sanotts en | WAV PCM | 24000 | 1 | s16 | 48000 |
| google (and the 3 cached files) | MP3 | 24000 | 1 | fltp, 64 kbit/s | 8000 |

- `DecodeWAVPCM16Source` accepted every WAV sample and refuses every MP3 (`invalid WAV container`).
- On constructed headers (not provider output) it also refuses stereo, 24-bit, float32, extensible (tag 0xFFFE), streamed data size 0 or 0xFFFFFFFF, and rates under 4 kHz. Which of these cloud providers really emit is unknown.

**(e) go-mp3**
- `SampleRate()` is the source rate (24000 here, no resampling) and the output is always stereo s16le, even for mono input.
- Decoding 5–10 s of audio took 85–165 ms.
- 1500-character German piece (1492 runes, 14 sentences):
  - phonon: 18.3 s wall, 78.9 s of audio, 3.79 MB, RTF 0.23.
  - sanotts: 7.9 s wall (the first call took 46.7 s, including install), 64.0 s of audio, 2.82 MB.
  - google: limit is 200 runes, not tested at 1500.

**(d) Envelope and cue placement**
- **Levels size:** 3.3–3.8 B/frame as JSON ints 0–255. 60 s (1800 frames) is about 6.4 KB, and 27000 frames about 95 KB, which matches D2.
- **Ground truth:** phonon Nele has true gaps at 2580 ms and 4260 ms (400–430 ms long). I read the same positions by eye from `T/nele.png`.
- **Pauses:** phonon does not pause at commas in de/en; the fr voices do. Phonon's pause floor is about -55 dB and sanotts' about -50 dB.
- **Spans per 3-sentence text** (mean over 15 samples):

| threshold | raw runs | after bridging gaps under 100 ms |
|---|---|---|
| -40 dB | about 14 (4–5 per sentence) | about 4.7 (about 1.6 per sentence) |
| -30 dB | about 21 | — |
| -50 dB | about 9 | about 3.5 |

- **Sentence-start error** (n=30, truth = the k longest 10 ms gaps):
  - Proportional rule: mean 341 ms, median 313 ms, p90 713 ms.
  - Vowel-weight rule over spans: mean 730 ms at -30, 600 ms at -40, 581 ms at -50, with markers early by 0.5–1 s on sentence 3.
  - Snapping to a span start within 333 ms gives 543–565 ms.
  - So the weight rule was worse than proportional at every threshold here. The best single case was phonon Nele at -40: +250 and +26 ms.
- **Comma onsets** (n=17): proportional mean 397 ms (median 184), global weights 529 (424), sentence-anchored weights 582 (400).
- **First word:** speech starts 90–320 ms after 0 (mean 153 ms). A span rule fixes this exactly.
- The 14-syllable sentence 3 shows about 14 humps in the waveform, which supports vowel groups as a weight, but humps are not word boundaries.

**Verdicts**
- **D4:** the idea holds but needs a new decoder. `DecodeWAVPCM16Source` is enough for local WAV today. It refuses all MP3 and any non-mono or non-s16 layout. Use WAV (s16/s24/f32, mono/stereo, extensible, streamed size) plus go-mp3 (stereo, source rate). The rates seen (22050, 24000) need the planned 48 kHz resample.
- **-40 dB:** fine as a start value for gap detection, but spans fragment at 30 fps. Bridge gaps under 100 ms. -50 sits on the pause floor and -30 splits words.
- **Cue rule:** keep proportional as the default. Use the envelope for the speech onset (about 150 ms) and for anchoring sentence starts to detected long pauses, falling back when the counts differ. In-sentence vowel-weight placement is not shown to beat proportional here and needs ears or forced alignment.
- **Expected cue error:** about 0.3 s median at sentence starts, 0.7 s at p90, on these local voices.

**Unknown for cloud providers**
- Rate, channels and container (MP3, WAV, opus) for openai, elevenlabs, minimax, mistral, xai, stepfun, resemble and speech_lab.
- Whether any emits streamed WAV headers.
- Their pause and noise floor.
- Time and size per 400-character piece.
- Piper and supertonic output.
- The active provider and voice on aurago-test.
- Why the Windows `ptts.exe` cannot write files here.
- Languages other than de/en/fr/es (zh, ja, hi, el).

**Repeat it**
- Programs: `T/src/ttsspike_main.go` and `T/src/ttsanalyze_main.go`. They were built from a throwaway worktree (cmd/ttsspike, cmd/ttsanalyze), which is now removed.
- Analysis: `ttsanalyze.exe <T>` writes `T/analyze.log`. Other logs are `T/phonon-run.log`, `T/phonon-docker.log`, `T/sanotts-run.log`, `T/google-run.log`.