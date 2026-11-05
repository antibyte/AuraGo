**Report: does the planned mix land near -16 LUFS on real material?**

Normal shapes A, B, D, E and F land within 1.1 LU of -16. Two cases miss by more than 3 LU: C with two speakers at different levels, and the short film I. The earshot `mcp__earshot__measure` tool was unavailable to me (no ToolSearch in this session), so I did not run it. As the second meter I used FFmpeg's `ebur128` filter (it reports sample peak, not true peak).

**Setup.** I copied the mixer to `.../p2/spikes/mixreal/mixer/` (the original folder is untouched). New inputs are real TTS sentences, the 25 real sfx and the sheet's `l50_lufs`, with the level rule and dips as specified.
- Music: `brain_in_the _machine.mp3` is a Suno song with vocals, 218 s, not instrumental. I also ran the CC0 loops pirate, temple and moon from `ui/3d/minigolf/audio/music`.
- Narration is unnormalised: phonon -21.9 LUFS with peak -6.1 dBFS, sanotts -26.4 with peak -12.1, anton -21.8.
- Each shape was 150 s, except I at 8 s.

**Results, 150 s per shape.** "Mix" is the pre-gain mix. "Loud/TP/stem" are the three gains in dB (loudness, true peak, stem guard); the bound gain is in brackets.

| Shape | Narr I | Bed gain | Mix I / TP | Loud / TP / stem | Final I (loudnorm) | Final TP |
|---|---|---|---|---|---|---|
| A | -21.90 | -22.29 | -22.06 / -5.98 | +6.06 / **+4.98** / +6.05 | -17.08 | -1.00 |
| B | -26.46 | -26.85 | -26.81 / -11.92 | **+10.81** / +10.92 / +11.97 | -16.00 | -1.11 |
| C | -24.38 | -24.77 | -24.79 / -6.13 | +8.79 / **+5.13** / +6.05 | **-19.66** | -1.00 |
| D | -21.90 | no bed | -21.90 / -6.13 | +5.90 / **+5.13** / +6.05 | -16.77 | -1.00 |
| E | -21.90 | no bed | -21.90 / -6.13 | +5.90 / **+5.13** / +6.05 | -16.78 | -1.00 |
| F | no narration | -6.39 | -18.16 / -3.46 | **+2.16** / +2.46 / +3.79 | -16.01 | -1.30 |
| G | no narration | no bed | -24.83 / -3.77 | +8.83 / **+2.77** / +3.79 | **-22.06** | -1.00 |
| H, vol 0.5 | -21.90 | -16.27 | -22.32 / -5.74 | +6.32 / **+4.74** / +6.05 | -17.58 | -1.00 |
| H, vol 0.1 | -21.90 | -30.25 | -21.93 / -6.13 | +5.93 / **+5.13** / +6.05 | -16.80 | -0.99 |
| I (8 s) | -21.92 | -10.27 | -24.65 / -6.56 | +8.65 / **+5.56** / +6.59 | **-19.09** | -1.00 |

- Variants of A with the CC0 loops (pirate/temple/moon) landed at -17.11, -16.98 and -16.93 final. B with moon landed at -16.00.
- Narr-minus-bed under speech (rms): 20.7 dB for A, 21.7 for B, 21.3 for C, 14.7 at vol 0.5 and 28.6 at vol 0.1.
- Clamps were 0 in every run. The null test (mix minus the three stems) was 0 nonzero samples in every run.
- ebur128 agrees with loudnorm within 0.1 LU on every long film. The exception is I, where ebur128 reads -17.5 against loudnorm's -19.09. loudnorm under-reads an 8 s film by about 1.6 LU.
- AAC 192k on A: decoded loudness -17.10 and true peak -1.07 (source -1.00), so no overshoot. B gave -1.10.

**Cause of the peak-bound results.** Phonon speech has a true peak 15.8 dB above its loudness. Narration alone therefore reaches only -16.8 at -1 dBTP (E). The rule's linear gain is bound by true peak in nearly every case, not by loudness.

**Case C, two speakers.** The quiet speaker pulls loudness down while the loud speaker sets the peak. I re-ran C with sanotts clips raised by +4.56 dB (`-gainB 4.56`, output in `c_fix.txt`). That gave -17.09 instead of -19.66, and the null test still held.

**Pipe metering at 900 s**, shape A, using the exact `-f f32le ... pipe:0` command:

| Run | Pipe I / TP | Pipe wall | Peak RSS |
|---|---|---|---|
| 8.1.1 f32le | -22.02 / -5.84 | 28.8 s (the pass-2 mix takes 28.7 s) | 67 MiB |
| 8.1.1 s16 at -12 dB | -34.05 / -17.89 (+12.04 gives -22.01 / -5.85) | 31.1 s | 69 MiB |
| 5.1.9 in debian:12, f32le | -22.02 / -5.84 | 39 s (mix 38.4 s) | 109 MiB |

- The file-based figure on the same data is -22.02 / -5.84 on both versions, so the pipe is byte-for-byte equivalent.
- A file-based 8.1.1 measure of the 345.6 MB temp file takes 27.5 s on top of the mix. On 5.1.9 it takes 43 s and 110 MiB.
- My container `mixreal-ff51` was started with `--rm` and stopped afterwards (verified removed). I made no worktree.

**Recommendations**
1. **Linear gain without a limiter is good enough.** A, B, D, E and F land at -16.0 to -17.1, and H at -16.8 to -17.6. Two things need action:
   - **Two-speaker film (C, -19.66):** add a per-clip or per-speaker narration gain match to the loudest speaker, as in the experiment above (the clip gain scales the narration stem, so stems still sum to the mix). This brings C to -17.1.
   - **Short films (I, about 8 s):** the film is peak-bound at -17.5. It also needs a meter that is correct for short films, such as `ebur128` or an own BS.1770 implementation, not loudnorm.
2. **No upward gain for G.** The formula already applies +2.77 dB, and the film ends at -22.06 because the sfx peaks bind it. Anything more needs a limiter. Accept it.
3. **Drop the temp file.** Pipe metering gives identical numbers on 8.1.1 and 5.1.9 and adds no wall time (the mix and meter overlap). Use f32le: the s16 at -12 dB variant works but offers no benefit. Add `ebur128` to the same pipe for correct short-film I.

**Not verified:** the earshot measure; true peak from `ebur128` (only sample peak); and listening quality. The sentence segmentation is mine, so B has only 8 clips. The narration clips were not normalised before mixing.

Repeat with `.../p2/spikes/mixreal/prep.sh`, `runall.sh`, `ebur.sh`, `aac.sh` and `pipe900.sh`. The pipe run needs the `mixreal-ff51` container started first. Results are in `runall.txt`, `ebur.txt`, `aac.txt`, `c_fix.txt` and `pipe900.txt` in the same folder.