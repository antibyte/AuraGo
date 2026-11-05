Static reading of the code plus your spike results; I ran nothing. Spec lines refer to C:/Users/Andi/Documents/repo/AuraGo/docs/superpowers/specs/2026-11-01-storymaker-design.md, contract lines to C:/Users/Andi/Documents/repo/AuraGo/docs/superpowers/specs/2026-11-01-storymaker-contract.md.

**1. Refuted or unsafe, by severity**

1. **CRITICAL, D6 "mixing during onReport".** The handler is synchronous (C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/render/listener.go:390). Three timers cut it:
   - WriteTimeout, 2 min (:146)
   - report timer, 3 min until `close(reportCh)` (C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/render/render.go:427-443, :626)
   - proxy header timeout, 5 min (:52)

   The spike's 57 s at 900 s excludes meters and clip decode, on 16 CPUs. Instead: answer `{pending:true}` at once, close `reportCh`, mix in a goroutine without `run.mu`. The page polls `GET <base>/start` for `{start:true}` or `{error,message}`. FFmpeg, the watchdog and `driveScreenshots` (:452) wait on a new `startCh`.
2. **HIGH, report trust behind D6/D9.** The first report wins (render.go:594-598), and scene workers run before the host posts (C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/host-core.js:222, :279). Contract line 800 accepts this, which is unsafe once the report drives the mix and SRT. Guard in answer 4.
3. **HIGH, spec cue rule, no decision.** TTS spike: vowel weights over spans give 581-730 ms mean error, proportional 341 ms. Add D22: `placed:span` means speech onset plus sentence starts anchored to the k longest gaps when counts match, proportional inside a sentence; otherwise `proportion`. This changes the spec, so it needs user sign-off.
4. **HIGH, D7 mode `track`.** C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/validate.go:209-217 refuses it, and spec lines 171 and 511 put track under mode `file`. `Music.Track` already exists (C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/model.go:142). Keep mode `file`, file XOR track, track numeric.
5. **HIGH, D5/D7 large files.** `Store.ReadFile` stops at 50 MB (C:/Users/Andi/Documents/repo/AuraGo/internal/desktop/service_files.go:347-353), and a 600 s WAV is 115 MB. Add `Store.Open` over `OpenPreviewFile`; write with `WriteStream`.
6. **MEDIUM, D3.**
   - `Enqueue` hard-codes render (C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/render/jobs.go:384) and `load` drops other kinds (:266).
   - A shared Dir would clobber jobs.json and wipe render staging (:226, :233).
   - Handlers and SSE use one `rt.jobs` and one revision (C:/Users/Andi/Documents/repo/AuraGo/internal/server/storymaker_handlers.go:595, :632, :666-701).
   - C:/Users/Andi/Documents/repo/AuraGo/ui/js/desktop/apps/storymaker-export.js:267 treats every job as the render.
   - Needs `ManagerOptions{Kind}`, Dir `storymaker/narrate`, a router, two SSE revisions, a kind filter, and coalescing per project.
7. **MEDIUM, D13.** Decide on compile: `needs` exists (C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/compile.js:106-113) and `ensureAssets` filters to it (host-core.js:320).
   - Make C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/host-render.js:72-76 lazy.
   - Keep faces out of `catalog.fonts`, or `style.font` accepts them (validate.go:166).
   - The CJK exception must cover both font paths; C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/host-app.js:42 checks `load` only.
   - D19's N bound must add 80-120 MiB per worker with CJK.
8. **MEDIUM, D12/D15.** Exempt default-ignorables (U+200C and U+200D are normal in Hindi and Arabic), check the NFC form, and always include U+2026. D15's override must precede `Validate` in `Prepare` (render.go:157-158), or burned captions skip the glyph check.
9. **MEDIUM, shrink to fit.** Compile has no canvas and runs before fonts (host-core.js:279 vs :289). Shrink in `text.layout`; phase 2 `text_too_small` reads style size only.
10. **MEDIUM, D14.** `decodeAudioData` of a 15-minute bed is 345 MB. Play the bed through `<audio>` and MediaElementSource; decode pieces in a sliding window.
11. **LOW, D10.** `cue_late` on a clamp at 0 fires on most first-scene cues, but spec line 413 says target busy only. Compare resolved instances (C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/beats.js:287), not strings.
12. **LOW, D2/D4.** Per-piece frame envelopes drift when concatenated. Store a 10 ms envelope per piece and build block levels and spans in Go (this changes the spec's `<hash>.<fps>` file). Add s24, extensible and streamed-size WAV.
13. **LOW, D1.**
    - No blocks for say beats in code scenes (compile.js:61-63).
    - A cast speaker's narration uses the cast voice.
    - Go `\s` is ASCII, JS `\s` is not (C:/Users/Andi/Documents/repo/AuraGo/ui/js/storymaker/narration.js:19), so the fixture needs U+00A0 and U+3000.
14. **LOW, D6/D21.**
    - `-inf` fails only with narration events. A film without events keeps anullsrc; SFX-only `-inf` gives G=0.
    - `Settings` lacks FFprobePath (render.go:64-73), and no runtime check covers loudnorm.
    - AAC stays at 64k (C:/Users/Andi/Documents/repo/AuraGo/internal/storymaker/render/ffmpeg.go:25); pin 192k.
    - Add to D21: `gpt-4o-mini-tts` is hard-coded (C:/Users/Andi/Documents/repo/AuraGo/internal/tools/tts_cloud.go:21) and shuts down 2027-01-06.

**2. Uncovered**
- Highlight beat.
- Speech bubbles: catalog styles, say without text (beats.js:266 draws title).
- `narration_incomplete` gate (504, 531).
- Character caps and their config (507).
- Project voice snapshot, language resolution, `narration_unavailable` (170, 505). Phase 1 stories have `voice:null`, so `POST /narrate` needs 409 `voice_required` and `POST /projects` a `language`.
- Generated-track identity: `audio/music/<sha256(mood|prompt)>.<ext>`, length excluded, render 409 `music_incomplete`.
- `sfx.typing` expansion into spaced keys.
- Read-only recheck before paid calls (771).
- Holds for Han text, which counts as one word (narration.js:31).
- QR AGENTS note, Noto and Inter notices (328, 458).
- 5.1.9 swatch check (836).
- Locale entries for new issue codes.

**3. Interfaces**
- **Routes:** `GET /projects/{id}/audio/narration/{hash}.{ext}`, `GET /projects/{id}/audio/music/{hash}.{ext}`; SFX from `/img/storymaker/sfx/`.
- **Block:** `{id:"<scene>:n"|"<scene>:b<beat>", kind, beat?, speaker, duration_ms=Σpieces, spans, levels, pieces:[{hash, ext, duration_ms, chars}]}`. `chars` (UTF-16, with joining space) anchors cues at piece starts.
- **Event:** `{kind, ref, start_ms, gain:1, scene_id, block_id?, take?}`. Go owns gain.
- **Report cap:** `maxReportBytes` from 1 MiB (listener.go:24) to 8 MiB, landing in 2B. A maximal Hindi story's captions alone reach it.
- **Boot:** `audio_info` from `Plan.AudioInfo` at queue time; `fonts[]` gains `fallback`, `unicode_range`. `catalog.json` names `glyphs:{file}`, otherwise the key is unlisted.
- **Job:** `output:{path,size,stems?:{narration,music,sfx},srt?,vtt?,library?}`, `warnings[]`. A library-copy failure is a warning. Silent stems are omitted; the track builder skips them and clips at min(asset frames, frame_count).
- **Options:** `{burn_in_captions?, srt, vtt, stems, quality:"standard"|"high"}`; an empty body means defaults. Drop `handoff`: make it a client action on any finished job with stems, so a closed window cannot lose it.
- **Audio module:** `createAudio({api}) -> {load(info,events), play(frame), pause(), seek(frame), dispose()}`.
- **Takes:** the mix reads them from `deps.UI`; the snapshot copies only `File` (render.go:558-562).
- **Ownership:** `validate.go`, `plan.js` and `host-render.js` are edited by several parts, so name one owner per batch. Locale files go to the last part. `storymaker.js` has 162 lines left, so panels are modules.

**4. Security**
- **Report:** one-shot token. boot.json carries `token` only on its first GET, which the host makes before any worker exists (host-render.js:55); `/report` and `/frame` require it. Still validate:
  - refs in Go's own block map, SFX and take in the catalog
  - event count under a derived bound, times inside the film, JS gain ignored
  - caption text a normalised substring of a block, controls stripped, `<&>` and `-->` escaped
- **Audio routes:** `ValidProjectID`, hash regex, extension whitelist, fixed Content-Type, nosniff, sandbox CSP (`writeStorymakerScript`, storymaker_handlers.go:502), `ServeContent`.
- **Music:** file only via `OpenPreviewFile`. A `GetMedia` path must be audio, under DataDir/audio, not protected. Copy to a hash name; FFmpeg sees only that, with `-protocol_whitelist file`, `-map 0:a:0`, `-t film+2`, and `-fs` against decode bombs.
- **Disk:** estimate before mixing; 256 MiB (render.go:51) is below the spike's 1.3 GB.
- **Videos:** the method accepts a slug only and builds the path; the desktop read-only flag is honoured.
- **Parent:** builds URLs only from its own `/narration` answer and the catalog, never from message text.
- **Go decoders:** cap WAV length at file size; recover go-mp3 panics.

**5. Determinism**
- No project seed exists in compile inputs. Seed takes from scene id, beat index and sound id; lip-sync from block id.
- `Intl.Segmenter`, `\p{Script}` and `normalize` vary with ICU and V8 (Chrome 128 vs 156), breaking timeline identity. Use generated range tables in compile.
- Set `ctx.direction` per text, or canvas state leaks across frames and workers diverge.
- A need calculation that misses a face draws host fonts silently (fonts spike c).
- Worker PNG parity rests on one story. Extend it to CJK, RTL and bubbles.

**6. Order: sound, with moves**
- **2A:** green after regenerating timing tests, contract line 618 and D11 default goldens. Leave the sfx rule at impact 0; accept `auto_sfx` in 2C, when it works.
- **2B:** put `audio_info` into `Plan` and boot, or the parity test diverges once audio exists. Do not land the `narration_incomplete` gate here: it breaks tests using narrated fixtures (storymaker_e2e_test.go, storymaker_parity_browser_test.go, browser_smoke_test.go) and guards a silent export. Move the audio route, narration-only preview and report cap here, or the voice panel cannot audition.
- **2C:** the gate lands with a seeded-audio test helper. D15 body parsing lands here, tested by API, with the UI in 2D. Export is silent until here.
- **2D:** W last is right, and droppable. E3 touches files dirty in the main checkout (menus-and-routing.js, window-shell-runtime.js, main.bundle.js), so rebuild the bundle at merge.

**7. Riskiest assumption**
That a linear gain lands near -16 LUFS on real material. Both spike runs were peak-bound (-17.9, -21.5 LUFS) with synthetic effects, and nobody has listened. Cheapest check at the start of 2C: run the spike mixer on the 25 real WAVs and Phonon narration over firewall.json, and read I and TP with earshot `measure`. If it lands more than 3 LU low, put the limiter question to the user. Runner-up: the worker restart never ran, so force one encoder failure in the parallel smoke.