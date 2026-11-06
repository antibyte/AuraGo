# Asset-pack acceptance, phase 2 (task I6)

- Date and commit: 2026-11-06, storymaker-p2d at 1bc4ac6ff.
- Chrome 156.0.8078.80 (installed), FFmpeg 8.1.1 (8.1.1-full_build-www.gyan.dev), local sanoTTS voice, scratch instance on port 18733.
- OK lines, verbatim:
  - `OK login      1s`
  - `OK app        16s`
  - `OK voices     16s`
  - `OK project    17s`
  - `OK narration  18s`
  - `OK narrate    1m14s`
  - `OK export     2m3s`
  - `OK handoff    2m10s`
- RESULT line, verbatim: `RESULT project=8639c0c5-5568-4973-ba43-16ddee976aba job=dc210e52-9e09-4ed0-a8a8-f1954c650d20 frames=215 mp4=Documents/Storymaker/8639c0c5-5568-4973-ba43-16ddee976aba/exports/I6-acceptance-dc210e52.mp4 size=265243 stems=1 srt=Documents/Storymaker/8639c0c5-5568-4973-ba43-16ddee976aba/exports/I6-acceptance-dc210e52/captions.srt vtt=Documents/Storymaker/8639c0c5-5568-4973-ba43-16ddee976aba/exports/I6-acceptance-dc210e52/captions.vtt library=Videos/Storymaker/I6-acceptance.mp4 videostudio=9ec12cea-af7f-4269-a4be-adf2182e8b64`, then `PASS`. The narrate step reported `voice chars=128 pieces=3`.
- Step 7: 215 video packets (equals frames=215); audio stream `aac,48000,167367`; integrated loudness `I: -16.0 LUFS` (finite, not silent); narration.wav 1376044 bytes, captions.srt 197 bytes, captions.vtt 201 bytes; the SRT shows the three narration sentences in two cues.
- Screenshots: the editor shows the scene with the subtitle and the scrubber waveform; Video Studio shows the handed-off project with one video track (the MP4) and one audio track (narration.wav).
- Page errors: no page errors.
- Not covered: the export dialog and the panels (the I2 tour covers them), music, phonon, restart, cancel.
