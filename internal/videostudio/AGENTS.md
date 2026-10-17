# Video Studio Core Contract

This package owns the Video Studio project model, validation, local media probing,
preview generation, runtime readiness checks, and bounded FFmpeg rendering.

- Inputs passed to FFmpeg are server-staged local regular files. Never accept URLs,
  shell fragments, arbitrary filter expressions, or demuxer names from project data.
  Keyframe and ramp expressions and `sendcmd` scripts are generated in Go from validated numbers only.
- Keep all clip timing in integer frames of the project rate (`ProjectRate`: 24/25/30/50/60 fps, NTSC
  24/30/60 with a 1001 denominator). Convert with `Rate` methods only. `Upgrade` steps version 1 (fixed 30 fps) → 2 → 3
  without modifying its input; `Validate` accepts version 3 only. Projects are limited to sixty minutes and the sizes in
  `CanvasSizes`.
- Markers (`markers`, at most 500) and the in/out points (`in_point`, `out_point`) are frames of the project rate.
  `validateMarkers` keeps a marker's frame and range inside `[0, MaxDurationFrames]`, its colour in the label palette
  (`LabelColors`), the in point before the timeline end and the out point at least frame 1, with in < out when both
  are set. Markers never render; the renderer ignores them and so does `Upgrade`.
- Bound projects to 8 video, 8 audio and 8 overlay tracks, 2000 clips and 1024 assets. One FFmpeg process uses
  at most `MaxSegmentInputs` (32) inputs. Asset durations are server-owned microseconds (`duration_us`);
  `duration_frames` is derived for the project rate (`AssetAtRate`). Source images/video frames are limited
  to 16,384 pixels per side and 64 megapixels. Videos are also limited to a 16:1 aspect (`MaxVideoAspect`,
  `source_aspect.go`): `Probe` fails with `ErrSourceAspect`, and exports, range renders and frame exports refuse a
  video imported before the rule that an enabled visual clip in the range uses (`checkSourceAspects` in `renderOnce`
  and `RenderFrame`). The job failure code is `source_aspect_extreme` (UI key `videoStudio.sourceAspectExtreme`).
  Images are exempt (wide title PNGs); a static cover clip whose source is more than 16× off the box aspect is
  pre-cropped before scaling, so its intermediates stay bounded.
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
  segment failures are `*SegmentError`. Preview proxies are complete-source 960×540 MP4s at `PreviewRate` (the source rate, capped at `MaxPreviewFPS` = 60; 30 fps when unknown), capped at `MaxPreviewBytes` (2 GiB).
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
  left off, so FFmpeg does not convert the pixels). `ExtractStill` takes a clip position (`StillOptions`: `Offset`,
  clip frame `Frame`, `Speed`) and samples the source with the renderer's own filter (`videoResample`, shared with
  `buildVisualClipFilter`), so a 30 fps source in a 25 fps project at 1.5x shows the rendered frame; it writes PNG. A
  still job carries `offset`, `clip_frame` and `speed` with `frame = floor(offset + clip_frame·speed)` (checked, else
  400 `invalid_range`); without `speed`, `frame` is the source frame at 1x. The server (`video_studio_derive.go`)
  imports results as project assets and publishes PNG exports. A reverse job may carry `name` (the editor's display
  name with the localized `videoStudio.reversedSuffix` toggled, no extension; at most `MaxAssetNameRunes`, no control
  characters, else 400 `invalid_job`); without it `reversedAssetName` toggles " (reversed)" on the imported name. The
  name always ends in `.mp4` and fits `MaxAssetNameRunes`, so reversing back and forth never grows it.
- Tests: `go test ./internal/videostudio -run 'TestRenderSpeed|TestRenderDisabled|TestRenderImage|TestTempoChain|TestReverse|TestExtractStill|TestValidateSpeed' -count=1`
  (the render and still tests need FFmpeg 5.1+).

## Media and projects (1e)

- Folders: `NormalizeFolder` is the one rule set (trimmed segments, 1 to 4 levels of 1 to 64 runes, no control characters,
  no `.`/`..`, at most 255 runes). `Validate` requires canonical asset folders and at most 128 unique `project.folders`.
- Names: a project document's asset name is its own; `canonicalizeProject` keeps it (a blank name falls back to the
  imported name) and keeps the folder.
- Posters: `WritePoster` makes a JPEG whose longest side is `PosterLongestSide` (320 px) from one second in (or the first
  frame of a clip shorter than two seconds) or from a still image; audio has none. Posters are best effort and count
  toward the project quota. Media imported before posters existed (`has_poster` false) gets one lazily: a media listing
  or a poster request queues `requestPosterBackfill` (once per asset and server run) on the upkeep queue below; the
  chore starts only while no job is queued or running (else it is dropped and asked for again), stages the media like a
  job and stores and records the poster under the project lock only while the record still has none.
- Frame rates: `FrameRateOptions()` is the only table; `Validate` accepts exactly its rates.
- Legacy durations: `BackfillDurationUs` is the only place that derives `duration_us` from 30 fps frames.
- Range renders use `RenderOptions.Start` and `End` in frames (`[Start, End)`); the server admits `0 ≤ Start < End ≤
  EnabledDuration` (else 400 `invalid_range`), and the job result carries `range` and `duration_frames = End − Start`.
- Versions, duplicates and the poster/removal routes are server code (`internal/server/video_studio_versions.go`,
  `video_studio_duplicate.go`, `video_studio_media_files.go`); this package supplies only the document rules they use
  (`Upgrade`, `Validate`). A restore runs a version through `Upgrade`, `canonicalizeProject` and `Validate`, so a
  version that references removed media is refused (409 `project_asset_invalid`). Version paths are built only from ids
  that pass `validVideoStudioVersionID` (the `uid.New` format); `readVideoStudioVersionIndex` drops other index.json
  entries, so a crafted id never names a file to read or prune. A save (PUT) checks `If-Match` against the stored
  document before canonicalizing, so a stale editor that lists removed media gets 412, not 400. A duplicate job records
  `copy_project_id` before copying and clears it when the copy is complete or removed; when the worker starts, a recorded
  copy folder without project.json is deleted with its media records, and a finished copy is never touched.
- Media removal (`video_studio_media_files.go`, `video_studio_media_upkeep.go`): DELETE persists `removing` on the record
  before it deletes anything (from then on `mediaRecord` and `mediaForProject` treat the asset as gone), then deletes the
  media file, poster and cached previews and last the record; a file that is already gone counts as removed. A failed
  step answers 500 (409 for a publication gate) and keeps the mark: a retry skips the in-use checks and finishes, and so
  do the worker start (`finishInterruptedMediaRemovals`) and the sweep a media listing queues (`requestMediaSweep`).
- Upkeep runs in one background goroutine fed by a queue of 64 chores (`queueMaintenanceLocked` never waits; a full
  queue drops the chore and the next listing asks again), only while Video Studio and the Desktop are writable. A listing
  queues a sweep at most once a minute per project; the sweep takes the project lock with `TryLock` (a busy project is
  swept later), finishes marked removals and removes a record whose media file is gone, and media files
  (`<asset id>_…`) and posters (`<asset id>.jpg`) without a record. It never touches what the saved project lists, other
  file names, records or files younger than 10 minutes (`videoStudioLeftoverMinAge`), or files while a generate,
  reverse or still job of the project runs (they write the file before the record).

## Foundation for effects and animation (2a)

- Model version 3: `CurrentVersion = 3`; `Upgrade` runs 1 → 2 (`BackfillDurationUs` at 30 fps) → 3 (no data
  change); older builds refuse version 3 (one-way, spec §4.3). New `Clip` fields keep the canonical order `label,
  keyframes, ramp, crop, flip_h, flip_v, blend, mask, effects, anim_in, anim_out` (Go struct = JSON = JS
  `canonicalProject`); the reflection guard in `ui/desktop_video_studio_canonical_test.go` checks it.
- Registry (`effects.go`): `EffectRegistry()` (deep copy in menu order, never-nil slices), `TransitionByID`,
  `EffectByID`, `BaseParam`, `Registry.Unavailable`, `RegistryVersion = 1`. The four transitions map to xfade `fade`,
  `fadeblack`, `wiperight` (for `wipeleft`) and `wipeleft` (for `wiperight`); `validateClip` and `appendTransition`
  read the table, so a new transition is one row. The status endpoint serves `registry` and `unavailable`;
  `video-studio-effects.js` mirrors it (`TestVideoStudioEffectsRegistryMatchesGo`). `fadeBlackWeights` holds the
  fade-through-black curve measured from FFmpeg. The curve is not linear: `out = (1−p)·(1−S(p; 0, 0.2))`,
  `inc = p·S(p; 0, 0.8)`, `black = 1−out−inc`, with S the smoothstep (darkest near p ≈ 0.2); FFmpeg 5.1.9 matches within
  1/255, 8.1.1 within 2/255. `testdata/parity/transitions.json` shares the numbers with JS and the GL shader.
- Capabilities (`capabilities.go`): `ProbeCapabilities` runs `-filters` and `-h filter=xfade` once per binary
  (memoized by resolved path, size and modification time) and lists `filter:<name>` / `xfade:<name>` gaps for the
  phase 2 filters and the registry transitions. `CheckRuntime` fills `RuntimeStatus.Missing`; `renderOnce` fails at
  once with `*CapabilityError` (unwraps to `ErrFeatureUnavailable`, job error `feature_unavailable`). The server's
  `videoStudioJobFailure` maps `ErrSourceAspect` to `source_aspect_extreme` next to that mapping. Readiness
  (`hasRequiredRenderFilters`) is unchanged. The `-filters` flag column has three characters on FFmpeg 5.1 (`TSC`) and
  two on 8.x (`TS`); the parser accepts both. The captured `-h filter=xfade` outputs are
  `testdata/xfade-help-5.1.txt` and `testdata/xfade-help-8.txt`.
- Per-clip chain (`clip_chain.go`): `clipChain` stages `retime`, `window`, `picture`, `effects`, `mask`, `transform`,
  `opacity`, `place` own one clip's filters and its geometry (`clipBox`, `evenPixels`, `evenPosition`);
  `segment_graph.go` keeps segment, layer, gap, transition and audio assembly. The 2a chain is byte-identical to the
  former `buildVisualClipFilter` (snapshot test). Later slices extend a stage, never the assembly.
  `buildVideoSegmentWith(…, videoTail)` takes the output tail; `videoTail{rgb: true}` ends in `format=rgb24`.
- Golden frames: `RenderFrame` (`frame.go`) writes one timeline frame as an 8-bit RGB PNG through the export graph,
  before any YUV conversion; its output path goes through `validateNewOutput`. `goldenframe` (imported only by tests)
  holds `SSIM`, the thresholds (geometry 0.98, colour 0.97, transition 0.96, detail 0.95), `MakeMedia` (BT.709-tagged
  all-intra H.264 and Go-written PNGs; nothing binary is committed) and the case sets; later slices register cases in
  their own `cases_2x.go`. `MakeMedia` sets the BT.709 tags with `setparams` in the filter chain as well as the encoder
  flags: on FFmpeg 8 the encoder flags alone left primaries and transfer unknown. The browser side is
  `TestDesktopVideoStudioGoldenFrames` (`ui`).
- Shared math fixtures: `testdata/parity/<topic>.json` (2a: `geometry`, `transitions`). Go reads them with
  `readParityCases` (`parity_fixture_test.go`) in `TestParity<Topic>`; the JS side is one `{exact, run(input)}` entry
  per topic in `videoStudioParityRunners` (`ui/desktop_video_studio_effects_test.go`). A new topic adds the file, both
  implementations and the runner entry in one task.
- Proxies: `PreviewRate(asset)` is the source rate from `SourceFPS` capped at `MaxPreviewFPS` (60), 30 fps when
  unknown; `runPreview` keeps 960×540 and uses the encoder's `videoArgs(rate)`.
- Range render (`range_render.go`): `RangeRenderSize(project)` is `RangeRenderHeight` (540) lines and the nearest
  even width for the project's aspect ratio; `MaxRangeRenderFrames(rate)` is `MaxRangeRenderSeconds` (60) rounded
  down to whole frames. The server's `range_render` job renders `[start, end)` with `RenderWithOptions` at that size
  and the configured encoder (libx264 fallback), stores one result per project in its preview folder and refuses
  longer ranges with `range_too_long`. Job results report the size they rendered, exports included.
- Known limits: untagged sources may be converted with BT.601 by FFmpeg but BT.709 by the browser (goldens use tagged
  media; plan 2c revisits this); hold-last-frame exists only in the GL preview.
- Tests: `go test ./internal/videostudio -run 'Upgrade|Validate|Registry|Capabilit|ClipChain|RenderFrame|FadeBlack|Parity|PreviewRate|RangeRender' -count=1`;
  `go test ./internal/videostudio/... -count=1` includes `goldenframe`; FFmpeg 5.1 container check per plan index §6.

## Keyframes and speed ramps (2b)

- Model: two clip fields follow `Label`.
  - `Clip.Keyframes map[string][]Keyframe` holds `{f, v, e}`, where `f` is in source frames (the unit of `Offset`).
  - `Clip.Ramp []RampPoint` holds `{f, speed, e}`, where `f` is clip time in timeline frames.
  - Easings are `linear`, `ease_in`, `ease_out`, `ease_in_out` (cubic) and `hold`. Between keys the left key's easing
    applies, and values are constant outside the keys.
- `validateKeyframes`:
  - Keys only on visual tracks and keyframable base paths (`BaseParam(...).Keyframable`: x, y, width, height, rotation,
    opacity).
  - 1..`MaxKeyframesPerParam` (64) per path and at most `MaxKeyframesPerClip` (512) per clip.
  - Strictly increasing integer `f` with |f| ≤ `MaxClipSpeed`·maxFrames.
  - Every value in its registry range; width and height keys are at least `MinBoxSize` (1 %), mirrored by
    `VideoStudioEffects.MIN_BOX_SIZE` (read by keyframe-edit's `MIN_SIZE`). No cross-parameter check (`clipBox` clamps).
- `validateRamp`:
  - Video-track clips with a video asset only.
  - 2..`MaxRampPoints` (16) points with strictly increasing `f` and |f| ≤ maxFrames.
  - Speeds 0.25..16, and `speed` absent or 1.
  - The extent rule is `offset + S(duration − 1) ≤ duration_frames − 1` (+1e-9). Points outside the clip are kept.
- Shared math:
  - `Ease`, `sampleKeyframes`, and `ValueAt(clip, path, frame)` at `clipFrameAt` (content anchor `offset + S(t)`).
  - `rampPieces`: a linear span is one piece, a hold span one constant piece, and an eased span `rampEaseSteps` (4)
    sub-pieces with span-relative nodes. Speed is constant before the first and after the last point. A clip has at
    most 62 pieces inside (`MaxRampPieces` = 64). `S(0) = 0`.
  - The inverse is `t0 + 2(s − s0)/(a + sqrt(a² + 2b(s − s0)))`.
  - The parity fixtures `easing`, `keyframes` and `ramp` share every number with `video-studio-effects.js`.
- Export of keyframed geometry: when x, y, width, height or rotation has keyframes, `clipChain.transform` writes
  per-frame expressions in `t`, never `n` (`clipChain.animatedTransform`):
  - the `scale … :eval=frame` w/h, the `rotate` angle, and the `overlay … :eval=frame` x/y;
  - each starts from `T = at + round(max(t,0)*Num/Den)`, with the window's absolute start frame baked in (`max(t,0)`
    only guards overlay's configuration-time evaluation, where t is NaN);
  - `pad@vs_pad_L_C` (`eval=frame`) gives every frame one constant canvas, because format, crop and rotate keep their
    configured input size. The canvas size, the rotation flag and the cover bound are computed over all frames of the
    whole clip, not the segment window (window-only sizes broke segmentation parity). `wholeExtent` is memoized once per
    clip (keyed by a digest of everything that feeds it), so every stage and window shares it. Crop commands and the
    first crop size still use the window. `rotate@vs_rotate_L_C` follows only when a rotation in the clip is non-zero: the canvas
    is 2 px larger, and the rotate canvas is the square `evenCeil(hypot)`;
  - cover clamps the source aspect with a static `crop`, scales, pads to the constant `coverBound` and crops the box
    with `crop@vs_crop_L_C` through `sendcmd` (a commanded crop needs a constant-size input). The commanded cover crop
    is verified on FFmpeg 5.1.9 and 8.1.1. `coverBound` holds its clamp range `low`/`high` to [1/16, 16]
    (`maxCoverAspect`), so a sliver box cannot make the bound huge.
    A static cover clip gets `staticCoverClamp` (`source_aspect.go`), a crop in front of its cover scale:
    `crop=w='min(iw,max(ih*H,2))':h='min(ih,max(iw/L,2))'` with L = box aspect/16 and H = box aspect·16. It is emitted
    only when the source aspect or its inverse (autorotation can swap stored sizes) is outside L..H, never for an
    unknown aspect. `aspectClampCrop` writes both this crop and the keyed cover crop (the keyed one has no minimum);
    `minClampSide` (2 px) keeps tiny sources from being cropped to nothing. Commit 4fb40193e.

  The builders are in `keyframe_expr.go`: `ffExpr` (with a Go evaluator for tests), `windowSourceExpr`, `curveExpr`
  (balanced `if()` trees with `st`/`ld`) and `boxExprs` (Go's even rounding). Numbers are formatted with
  `strconv.FormatFloat(v, 'f', -1, 64)`, without exponents.
- Export of commands: `sendcmd` drives only single-input filters, namely opacity (`colorchannelmixer@vs_opacity_L_C aa`)
  and the cover crop (`crop@vs_crop_L_C w/h`). `sendcmd` on two-input filters (`overlay`, `blend`) landed 1–3 frames
  early on FFmpeg 8.1.1, differently per run, so it is forbidden.
  - `WriteSendCmd`/`sendCmdScript` accept targets matching `^[a-z]+@vs_[a-z]+_[0-9]+_[0-9]+$`, the options `aa|w|h`
    (`sendCmdOptions`; 2c/2d extend it) and finite values only.
  - Frame k's commands start at `(k − 0.5)·Den/Num` s (frame 0 at 0), repeated values are skipped, and the file mode
    is 0o600.
  - There is one file per animated clip and FFmpeg process (`vs_cmd_<L>_<C>.txt`).
- Staging: every video FFmpeg process (`renderVideoSegment`, `RenderFrame`) calls `stageVideoGraph(dir, graph, rate,
  commands)`.
  - `os.MkdirTemp(dir, "vsg-")` holds `graph.txt` and the command files.
  - `runFFmpegIn(ctx, workDir, …)` runs FFmpeg with that working directory. Inputs, output, graph path and binary are
    absolute. The graph names command files bare, so no Windows path reaches filter escaping.
  - The directory is removed with `defer`. `runFFmpeg` is `runFFmpegIn` with `dir = ""`.
- Export of ramps: `retime()` uses `rampResample`, the integer snapping of phase 1 (`setpts` with `ceil` of a balanced
  tree of piece inverses on `ld(0)`).
  - Output frame n therefore shows `floor(offset + S(n))`, exactly, on 5.1 and 8.1. The snapping reproduces that on
    every curve tested.
  - The seek window starts at `offset + S(entered)`.
  - A clip without keyframes or ramp renders the 2a chain byte for byte (`TestClipChainSnapshots`).
  - Every segmentation renders identical frames (`TestSegmentedRenderKeyframes`, `TestSegmentedRenderRamp`).
  - Known FFmpeg defect, pre-existing and not fixed in 2b: when a one-frame piece feeds `concat` (a one-frame transition
    head, a one-frame gap or a one-frame window piece), FFmpeg places everything after it one frame early. The tests
    avoid such layouts; a separate fix is pending.
- Audio and stills: `audibleClip` leaves the linked (not detached) audio of a ramped video-track clip out of the mix.
  Detached audio plays at 1×. For still jobs of ramped clips the editor sends `speed = meanSpeed(clip, n)`, so
  `ExtractStill` picks `floor(offset + S(n))` without a server change.
- Capabilities: `CheckProjectCapabilities` needs `filter:sendcmd` (feature `keyframes`) for any enabled visual clip with
  keyframes. Ramps need no new filter.
- Goldens: `goldenframe/cases_2b.go` registers the ticker media and ten cases at `ThresholdGeometry` on `gl` and `2d`.
  The lowest 2b SSIM is 0.9928 (`keyframe_combined_speed2/2d`). The ticker uses luma stripes with a small colour tint:
  full-frame chroma-heavy media lose about 0.03 SSIM, because ffmpeg's yuv420p-to-RGB conversion and Chrome place the
  chroma about 0.4 px apart. 2c colour goldens should therefore prefer smooth or luma-dominant media.
- Tests:
  - `go test ./internal/videostudio -run 'Keyframe|Easing|SendCmd|Ramp|Segmented|Parity|ClipChain|Validate|Capabilit|StageVideoGraph' -count=1`
  - `go test ./internal/server -run 'VideoStudioProjectKeyframes' -count=1`
  - The FFmpeg 5.1 container check (plan index §6) runs the gated parity tests on bookworm's FFmpeg.
