I could not save the report: the Write tool rejected `report.md` as a report file. This message is the report, and the scripts and logs are in the folder named under "Repeat".

**Setup.** FFmpeg 8.1.1 ran on the host and 5.1.9-0+deb12u1 in a `debian:12` container. Docker Desktop was off, so I started it, pulled `debian:12` and ran one `--rm` container. The inputs are synthetic: Windows SAPI speech, chord music, generated sound effects.

**(a) loudnorm as a meter.**
- The JSON is identical on both versions. I diffed all 5 files. It is printed on stderr, and every value is a string. Parse from the last `{` to the last `}`.
- The keys are `input_i`, `input_tp`, `input_lra`, `input_thresh`, the four `output_*` keys, `normalization_type` and `target_offset`. Use `input_i` and `input_tp`.
- A silent file gives `"-inf"` for `input_i`, `input_tp` and the output values, and `target_offset` is `"inf"`. Test for infinity and fail with `narration_failed`.
- Clips shorter than 3 s work, down to 0.5 s, and report `"linear"`. I did not test clips under 400 ms.

| File | I / TP in | Gain, dB | Re-measured I / TP |
|---|---|---|---|
| speech 27.5 s | -21.47 / -5.77 | +4.77 (peak-bound) | -16.69 / -1.00 |
| speech 1.6 s | -23.27 / -10.62 | +7.27 | -15.97 / -3.35 |
| music 20 s | -17.90 / -8.31 | +1.90 | -16.00 / -6.41 |
| sine 0.5 s | -31.05 / -31.07 | +15.05 | -16.05 / -16.02 |

The re-measured values were the same on both versions. The gain `min(-16-I, -1-TP)` lands within 0.05 LU of -16 when loudness-bound. When peak-bound it ends below -16 and exactly at -1.00 dBTP.

**(b) Streaming mixer.** It is a separate Go module, `mixer/*.go`, built as `mixspike.exe`.
- **Design.**
  - 48 kHz stereo s16, blocks of 4800 frames.
  - Clips are loaded when they start and dropped when they end.
  - The bed is streamed through two read windows. A single window thrashed, and the first run took over 300 s.
  - Equal-power loop crossfade of 2 s.
  - Duck keyed by the narration stem with a 50 ms look-ahead, which needs one extra narration block.
  - The 2.2 kHz biquad carve keeps its state and is gated by the speech spans.
  - Impact dips.
  - Stems are quantised after the master gain, and the mix is the clamp of the three quantised stems.
- **Passes.** P1 measures narration and the raw bed. P2 writes a float32 mix and measures it. P3 writes the final stems and mix.

| | 180 s, 1000 clips | 900 s, 1000 clips |
|---|---|---|
| Passes 1 / 2 / 3 | 1.76 / 5.07 / 4.16 s | 11.95 / 22.78 / 22.66 s |
| Mixer total (excluding the ffmpeg meter runs) | 11.0 s | 57.4 s |
| Peak working set of the Go process | 30.4 MiB | 27.5 MiB |
| Final mix | I -17.89, TP -1.00 | I -21.54, TP -1.00 |
| Clamps | 0 | 0 |
| Null test, read back from files | 0 of 17.28M samples nonzero | 0 of 86.4M samples nonzero |

- **Gain choice.** Both runs ended below -16 LUFS because the gain was bound by true peak. My synthetic effects were hot, so that is expected.
- **Wall time.** It includes writing about 1.3 GB at 900 s.
- **Clipping demo.** I added 6 dB to the final gain on the 180 s run. That gave 5585 mix clamps, 5585 nonzero null samples and a maximum error of 14193 LSB. The -1 dBTP gain prevents this, because sample peak is at most true peak. A stem can peak above the mix, so I added a guard: `G <= -0.1 - max stem peak`. It was not the binding limit here (-3.83 dB against -5.20 dB).
- **Ducking depth.** With the bed at narration minus 12 LU, narration RMS minus bed RMS under speech was 16.5 dB at 4 dB depth, 18.3 at 6, 20.1 at 8 and 21.9 at 10.

**(c) AAC.** I used `-c:a aac -b:a 64k -ar 48000 -ac 2 -movflags +faststart -f mp4`, plus 192k for comparison.
- **Length.** The decoded files are 512 frames longer than the mix, so trim to N.
- **Residual of decoded mix minus the sum of decoded stems:**
  - 64k: rms -38.3 dBFS, which is 18.2 dB below the mix, with peak -11.6 dBFS.
  - 192k: rms -42.8 dBFS, which is 22.8 dB below the mix, with peak -12.4 dBFS.
- **Ceiling.** The decoded mix true peak was -1.78 dBTP at 64k and -1.48 at 192k, against -1.00 before encoding. There was no overshoot on this one signal.
- **Stems.** They sum exactly only before encoding. State the stems-sum guarantee as "before encoding".

**(d) Frame gate.**
- **Command.** `ffprobe -v error -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 FILE.mp4`
- **Cases.** I ran 91 frames at 30 fps, 83 at 25 and 100 at 35. The 100-frame case has 137,143 samples, which is round(137142.857). Each had exact, 1 s short and 1 s long audio, on both versions.
- **Packet count.** The output was 91, 83 and 100 in all 18 runs, so audio length never changed it.
- **Audio duration difference.** With exact audio the audio stream is within 0.34 ms of the video stream, below one AAC frame. A short stream leaves the container at video length. A long stream makes the file 1.0 s longer.
- **Consequence.** The packet gate checks frames only. Add an audio check, for example |audio duration - N/48000| < 25 ms.

**(e) sidechaincompress.** It exists on both versions with the same 12 options, and `diff` of the option lists was identical. A run with attack 50, release 400 and an `acrossfade` run both worked on both. D5 is therefore not about compatibility. Its reasons are exact stems, the 1000-clip graph, look-ahead and unit tests.

**Verdicts.**
- **D5: confirmed.** The memory use is flat and the null test is exact.
- **D6: confirmed.** Add the audio-duration check beside the packet count.
- **Ducking.** Use 6 dB, within a range of 5 to 8. Attack 50 ms with 50 ms look-ahead, release 400 ms, detector threshold -45 dBFS.
- **Block design changes.**
  - Feed the detector one block ahead.
  - Define the mix as the clamp of the quantised stems, so the integer null is exact.
  - Add the stem-peak guard to G.
  - Trim decoded AAC to N.
  - P2 needs a float32 file of 345 MB at 900 s. Writing s16 at -12 dB and adding 12 dB to G would avoid it. I did not test that.

**Not verified.**
- I did not listen to anything.
- The speech, music and effects are synthetic.
- The carve spans come from clip placement, not from an envelope.
- I tested FFmpeg 5.1.9 only.
- Peak RSS covers the Go process only, on Windows.
- The -12 LU reference and the carve Q are not tuned.
- A mistaken python heredoc may have left one idle python process, PID unknown.

**Repeat.** The folder is `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/mix/`.
- `a1.sh` is part (a), and `all.sh` runs the Docker side.
- `mixer/` holds the mixer, built as `mixspike.exe`, with its logs in `run180.txt` and `run900.txt`.
- `c_aac.sh` is part (c).
- `g.sh` is parts (d) and (e).
- Logs: `media/out_v8/`, `media/out_v51/`, `gate/out_v8/`, `gate/g_v51.log` and `c_aac.txt`.