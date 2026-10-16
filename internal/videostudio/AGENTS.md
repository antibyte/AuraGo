# Video Studio Core Contract

This package owns the Video Studio project model, validation, local media probing,
preview generation, runtime readiness checks, and bounded FFmpeg rendering.

- Inputs passed to FFmpeg are server-staged local regular files. Never accept URLs,
  shell fragments, arbitrary filter expressions, or demuxer names from project data.
- Keep all clip timing in integer frames of the project rate (`ProjectRate`: 24/25/30/50/60 fps, NTSC
  24/30/60 with a 1001 denominator). Convert with `Rate` methods only. Version 1 projects (fixed 30 fps) pass
  through `Upgrade`; `Validate` accepts version 2 only. Projects are limited to sixty minutes and the sizes in
  `CanvasSizes`.
- Markers (`markers`, at most 500) and the in/out points (`in_point`, `out_point`) are frames of the project rate.
  `validateMarkers` keeps a marker's frame and range inside `[0, MaxDurationFrames]`, its colour in the label palette
  (`LabelColors`), the in point before the timeline end and the out point at least frame 1, with in < out when both
  are set. Markers never render; the renderer ignores them and so does `Upgrade`.
- Bound projects to 8 video, 8 audio and 8 overlay tracks, 2000 clips and 1024 assets. One FFmpeg process uses
  at most `MaxSegmentInputs` (32) inputs. Asset durations are server-owned microseconds (`duration_us`);
  `duration_frames` is derived for the project rate (`AssetAtRate`). Source images/video frames are limited
  to 16,384 pixels per side and 64 megapixels.
- Render through `RenderWithOptions` (`Render` is the whole-timeline wrapper): `PlanVideoSegments` cuts ~10 s
  segments whose boundaries avoid transitions, `PlanAudioSegments` avoids fades, and a segment needing more than
  `MaxSegmentInputs` clips is split. Every clip gets its own `-ss`-seeked input; a clip past the decodable video
  seeks one second before its end and `tpad` holds the last frame. Ranges that start or end inside a transition
  or fade get pre-roll (`expandRange`) and are trimmed back.
- Video segments run with `-copyts -start_at_zero`: video inputs seek about one second early and `fps` samples
  absolute source timestamps, so sources at another rate than the project keep the single pass's cadence in every
  segmentation. A first decoded frame later than the wanted position (video starting after its container) is
  moved back to it. The graph ends in an explicit `scale=...:out_color_matrix=bt709:out_range=tv,format=yuv420p`
  (FFmpeg before 7.1 converts RGBA with BT.601 in the encoder's scaler); hardware uploads append after it.
- Video segments use identical encoder settings (closed GOP, keyframe at frame 0, BT.709 tags) in MPEG-TS; audio
  segments are float PCM WAV with exactly `SampleAt(end)-SampleAt(start)` samples. The final pass uses the
  concat demuxer with explicit durations, `-c:v copy`, one `alimiter` and one AAC encode. Intermediates only
  pass the `concat,mpegts,wav` demuxer allowlist.
- Commands run through `exec.CommandContext` without a shell, one decoder thread per input and four filter
  threads, a four-hour ceiling, bounded diagnostics with scrubbed paths, and `MaxOutputBytes` (default 2 GiB;
  the server passes `max_export_size_mb`); an output that reaches it fails with `ErrOutputLimit` (FFmpeg 8 may
  write past `-fs` on stream copy, so the size check decides). `checkRenderSpace` fails early with `ErrInsufficientDiskSpace`;
  segment failures are `*SegmentError`. Preview proxies are complete-source 540p/30 fps MP4s capped at
  `MaxPreviewBytes` (2 GiB).
- `Encoder` (`encode.go`) holds codec, render node and CRF-equivalent quality. `SelectEncoder` tests candidates
  with a one-second encode (`testEncode` is replaceable in tests). Hardware encoders only encode: segment graphs
  end in `format=nv12,hwupload` for VAAPI/QSV, device setup comes from `globalArgs`. `RenderWithOptions`
  restarts once with libx264 when a hardware video segment fails (`OnEncoderFallback`); previews retry in
  software. Run `TestVAAPIRenderOptIn` on a host with a render node.
- Preview and export hold the last video frame through longer source audio,
  including clip offsets beyond video EOF; FFmpeg readiness requires `tpad` for
  bounded render padding.
- A transition belongs to its outgoing clip and must equal the explicit overlap
  with the next clip on that same visual track. Other same-track overlaps fail.
  `settb` to the project frame timebase follows every concat and feeds both xfade
  inputs. Only the overlapping frames are premultiplied, cross-faded and
  unpremultiplied (`premultiply`/`unpremultiply`, both required for readiness) so
  a dissolve does not darken picture-in-picture clips; that round trip changes
  pixels even at full opacity, so frames outside a transition never pass through
  it and render identically in every segmentation.
- The server owns persistence, upload staging, job scheduling, and cancellation
  lifecycle. This package stays independent of server state.

Run `go test ./internal/videostudio` for validation, real-frame/audio FFmpeg
integration, and output metadata checks. The opt-in ten-minute local performance
check runs with `AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio
-run '^TestTenMinuteRenderPerformanceOptIn$' -count=1`.

## Speed, links, labels and derived media (phase 1b)

- Clip fields: `speed` (absent = 1; 0.25–16; stills stay at 1), `link` (≤ 16 clips), `audio_detached` (video
  clips with sound only), `disabled`, `label` (`LabelColors`). The source extent rule is
  `offset + (duration − 1) × speed ≤ duration_frames − 1`; `ClipSpeed` and `EnabledDuration` are the helpers.
- Rendering: a clip with a speed is retimed around its first source frame (`clip.Offset`) before `fps`. Each source
  frame is snapped to the whole output frame where it first shows, so output frame n shows source frame
  `floor(offset + n·speed)`; every segment anchors on `Offset`, so every segmentation samples the same source frames.
  Audio uses `tempoChain` (atempo factors within 0.5–2) and leaves clips above 4× out of the mix. Disabled clips
  are left out of layers, mixes and staging; export length ends at the last enabled clip. A transition applies only
  when the clip after it (in the unfiltered order) is enabled.
- Derived media (`derive.go`): `ReverseClip` renders the source range backwards in chunks (2 s, or 1 s at ≥ 2560×1440
  pixels, shortened so one chunk's decoded frames fit in 256 MiB: frames are converted to yuv420p, 1.5 bytes per pixel,
  before the `reverse` filter buffers the chunk; `ReverseChunkFrames` uses the probed size) and joins them with the
  render mux; the chunks keep the source's colour tags (the renderer's BT.709 tags are
  left off, so FFmpeg does not convert the pixels). `ExtractStill` picks one source frame with the renderer's
  sampling and writes PNG. The server (`video_studio_derive.go`) imports results as project assets and publishes
  PNG exports.
- Tests: `go test ./internal/videostudio -run 'TestRenderSpeed|TestRenderDisabled|TestRenderImage|TestTempoChain|TestReverse|TestExtractStill|TestValidateSpeed' -count=1`
  (the render and still tests need FFmpeg 5.1+).

## Media and projects (1e)

- Folders: `NormalizeFolder` is the one rule set (trimmed segments, 1 to 4 levels of 1 to 64 runes, no control characters,
  no `.`/`..`, at most 255 runes). `Validate` requires canonical asset folders and at most 128 unique `project.folders`.
- Names: a project document's asset name is its own; `canonicalizeProject` keeps it (a blank name falls back to the
  imported name) and keeps the folder.
- Posters: `WritePoster` makes a JPEG whose longest side is `PosterLongestSide` (320 px) from one second in (or the first
  frame of a clip shorter than two seconds) or from a still image; audio has none. Posters are best effort and count
  toward the project quota.
- Frame rates: `FrameRateOptions()` is the only table; `Validate` accepts exactly its rates.
- Legacy durations: `BackfillDurationUs` is the only place that derives `duration_us` from 30 fps frames.
- Range renders use `RenderOptions.Start` and `End` in frames (`[Start, End)`); the server admits `0 ≤ Start < End ≤
  EnabledDuration` (else 400 `invalid_range`), and the job result carries `range` and `duration_frames = End − Start`.
- Versions, duplicates and the poster/removal routes are server code (`internal/server/video_studio_versions.go`,
  `video_studio_duplicate.go`, `video_studio_media_files.go`); this package supplies only the document rules they use
  (`Upgrade`, `Validate`). A restore runs a version through `Upgrade`, `canonicalizeProject` and `Validate`, so a
  version that references removed media is refused (409 `project_asset_invalid`).
