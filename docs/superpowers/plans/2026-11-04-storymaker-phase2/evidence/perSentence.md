Recommendation: synthesise one sentence per piece for sanotts and phonon, but merge fragments under about 20 characters into a neighbour. I could not run the earshot check (item 6), and none of this is a prosody judgement.

**Setup.** Everything is under `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/persentence/` (call it `P`).
- Texts are the four 3-sentence texts from the first spike, plus `in/de12.txt` (12 sentences, including "Genau." and one 189-character sentence with 5 commas) and `in/frag.txt` ("Ja.", "Nein!", "Okay.").
- Sanotts: `src/psspike_main.go`, built with a Go overlay at `bin/psspike.exe` and run with `run-sano.sh`. Log: `sano-run.log`.
- Phonon: `phonon-ps.sh` inside my own `golang:1.27.3 --rm` container with deploy/phonon read-only. Log: `phonon-run.log`.
- Analysis: `src/psanalyze_main.go`, run as `bin/psanalyze.exe <P>`. Output: `analyze.log`. Joined WAVs are in `out/concat/`.
- I created no worktree and did not touch the main checkout.

**1. Wall time.** Both providers start a new process per call.
- **sanotts:** 12 calls took 21.5 s (1.79 s per call). One call with all 12 took 4.9 s, so 4.35 times slower. A 6-character clip took 1.8 s and a 189-character clip 2.0 s. The fit is wall time ≈ 1.5 s + 3 ms per character.
- **phonon:** 12 calls took 13.2 s (1.10 s per call). One call took 5.1 s, so 2.6 times slower. Wall time minus the reported generation time has a median of 0.54 s (mean 0.65 s, one 3.8 s outlier).
- For the 3-sentence texts, sanotts took 2.3 to 2.7 times as long as one call, and phonon 1.9 to 2.2 times (French 4.2, with a 4.4 s first call).

**2. Silence at the edges and at joins.**

| | Lead | Tail | Join pause (tail + lead) | Pause in one call | Concat vs one call |
|---|---|---|---|---|---|
| sanotts | de 92 ms, en 320, fr 100, es 23 | de 212, en 502, fr 162, es 199 | de12 mean 308 ms (max 440), en 816, fr 258, es 246 | about 110 to 350 ms (de12 longest 190 to 350) | +2.1 to +8.8 % |
| phonon | 120 to 210 ms | 560 to 850 ms | de12 mean 852 ms (max 1040) | 350 to 910 ms (de12 mean 572) | +9.2 to +16.5 % |

- **sanotts pauses:** in one call, a sentence gap is the same length as a comma gap, so I could not find sentence boundaries reliably. The values are from picking the gaps whose segment lengths best match the per-sentence clips. French and de12 still fit loosely (mean error 527 and 347 ms).
- **phonon pauses:** its tail silence is long, so raw joins are about 300 to 450 ms longer than the natural pause. Trim the tails.
- **Speed:** speech time only is 1 to 4 % longer per sentence, apart from English on sanotts, which is 4 % shorter. Speaking speed is effectively unchanged.

**3. Commas inside a sentence.** I used proportional placement over the clip's speech span, matching each comma to the nearest gap of 100 ms or more within 700 ms.
- **sanotts:** mean error 130 ms (n=10, p90 241). The same method on the one-call audio gives 189 ms.
- **phonon:** mean error 193 ms (n=6 of 10 commas; 4 had no gap). The one-call baseline is 231 ms (n=7).
- I could not reproduce the first spike's 397 ms, so I compared with my own baseline instead. Both numbers are biased low because I only used commas that have a gap, and the samples are small.

**4. Short fragments.** All three contain complete speech and none is cut (last 20 ms around -77 dB or quieter).

| | Clip duration | Speech span | RMS over speech | Other clips (de12) |
|---|---|---|---|---|
| sanotts | 499 to 638 ms | 220 to 290 ms | -25.0 to -27.2 dB | -23.9 to -27.6 dB |
| phonon | 1200 to 1280 ms | 260 to 400 ms | -22.8 to -26.2 dB | mean -21.4 dB ("Okay." is 4 to 5 dB below) |

**5. Level spread (RMS over speech spans, de12).**
- sanotts: 3.8 dB per clip (sd 0.94) against about 4.8 dB per sentence in one call. The one-call figure depends on my approximate sentence boundaries.
- phonon: 4.8 dB per clip (sd 1.11) against 3.5 dB in one call.
- The spread comes from which sentence it is, not from splitting. A per-clip RMS match to about ±2 dB is cheap if wanted.

**6. Clicks.** The earshot tools are not available to me here: ToolSearch does not exist in this session. I checked programmatically instead and listened to nothing.
- A raw hard join, with silence kept at the clip edges, has a jump of 0.0009 or less (-60 dBFS). That is not a click.
- If you trim at the -40 dB speech edge, the hard join jumps by up to 0.036 (-29 dBFS). That is up to half the largest normal sample-to-sample step in the speech, so a faint tick is possible.
- Use a 3 to 5 ms fade per clip if you trim. It is not needed on untrimmed clips.

**Recommendation.**
- **One sentence per piece:** yes, for the local providers. Sentence starts become exact (the old error was 341 ms mean, 713 ms p90), and an edit re-synthesises only its own sentence. The cost is the extra time below.
- **Merging fragments:** merge any sentence under 20 characters, or 3 words or fewer, into the next sentence (into the previous one if it is last). "Genau.", "Das ist wichtig." and "Ist sie neu?" all qualify. Each tiny call costs a full 1.5 s on sanotts. The threshold is a cost judgement, not a measured quality limit.
- **Sentence over the provider limit:** split at the comma nearest the middle, keeping each part at least 40 characters, and use a join pause of about 150 ms. The 189-character sentence synthesised fine as one call, but I did not test splitting and I did not look up the provider limit.
- **Joins:** trim each clip to its speech span, then insert an explicit pause instead of the raw tails. Start from 250 to 350 ms for sanotts and 400 to 500 ms for phonon, with a 5 ms fade.

**Extra time for a typical film (20 scenes, about 2 sentences each, so 40 calls against 20).** I extrapolated from the measured per-call times.
- **sanotts:** a 2-sentence scene takes about 3.5 s against about 1.9 s as one call, so +1.5 s per scene and about +30 s for the film (roughly 70 s against 38 s). The range is +0 s with 1 sentence per scene to +60 s with 3.
- **phonon:** about +0.65 s per scene, so about +13 s (range +0 to +34 s).
- I did not test parallel calls, which could reduce this.

**Cloud providers.** The same characters are billed, with about twice as many calls. I measured nothing for them.

**Not verified.** Prosody, audible clicks, English and Spanish comma behaviour (neither text has commas), other voices, and parallel synthesis.