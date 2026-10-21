# Video Studio Core Contract

This package owns the Video Studio project model, validation, local media probing,
preview generation, runtime readiness checks, and bounded FFmpeg rendering.

- Inputs passed to FFmpeg are server-staged local regular files. Never accept URLs,
  shell fragments, arbitrary filter expressions, or demuxer names from project data.
  Keyframe and ramp expressions and `sendcmd` scripts are generated in Go from validated numbers only.
  FFmpeg reads LUTs only as Go-written `vs_lut_<n>.cube` copies of parsed tables (`lutFileName`: `^vs_lut_[0-9]+\.cube$`;
  one file per distinct table of a process, written once); uploaded `.cube` text never reaches a filter graph. Blend
  mode names reach `blend` only from the registry (`BlendModeByID`), the chroma key colour is written by Go as
  `0xRRGGBB` from the validated hex, and mask `geq` expressions hold only Go-formatted numbers and the fixed variables
  `X`, `Y` and `T`. xfade names reach the graph only from the registry (`TransitionByID`). The reveal `geq`, the `enable`
  expressions and the blur mask commands hold only Go numbers and the T-baked frame. The fadegrays self-test renders
  fixed Go-written lavfi patterns.
- Keep all clip timing in integer frames of the project rate (`ProjectRate`: 24/25/30/50/60 fps, NTSC
  24/30/60 with a 1001 denominator). Convert with `Rate` methods only. `Upgrade` steps version 1 (fixed 30 fps) → 2 → 3
  without modifying its input; `Validate` accepts version 3 only. Projects are limited to sixty minutes and the sizes in
  `CanvasSizes`.
- Markers (`markers`, at most 500) and the in/out points (`in_point`, `out_point`) are frames of the project rate.
  `validateMarkers` keeps a marker's frame and range inside `[0, MaxDurationFrames]`, its colour in the label palette
  (`LabelColors`), the in point before the timeline end and the out point at least frame 1, with in < out when both
  are set. Markers never render; the renderer ignores them and so does `Upgrade`.
- Bound projects to 8 video, 8 audio and 8 overlay tracks, 2000 clips and 1024 assets. One FFmpeg process uses
  at most `MaxSegmentInputs` (32) inputs and at most `MaxSegmentLUTInstances` (64) lut effects (on, with a non-empty
  ref and a table, sitting on enabled clips of visible visual tracks; `countLUTInstances`). Asset durations are server-owned microseconds (`duration_us`);
  `duration_frames` is derived for the project rate (`AssetAtRate`). Source images/video frames are limited
  to 16,384 pixels per side and 64 megapixels. Videos are also limited to a 16:1 aspect (`MaxVideoAspect`,
  `source_aspect.go`): `Probe` fails with `ErrSourceAspect`, and exports, range renders and frame exports refuse a
  video imported before the rule that an enabled visual clip in the range uses (`checkSourceAspects` in `renderOnce`
  and `RenderFrame`). The job failure code is `source_aspect_extreme` (UI key `videoStudio.sourceAspectExtreme`).
  Images are exempt (wide title PNGs); a static cover clip whose source is more than 16× off the box aspect is
  pre-cropped before scaling, so its intermediates stay bounded.
- Render through `RenderWithOptions` (`Render` is the whole-timeline wrapper): `PlanVideoSegments` cuts ~10 s
  segments whose boundaries avoid transitions, `PlanAudioSegments` avoids fades, and a segment needing more than
  `MaxSegmentInputs` clips or `MaxSegmentLUTInstances` lut effects is split; an instant that needs more lut effects than
  the cap fails with `ErrTooManyLUTs`, which wraps `ErrTooManyInputs` (server code `render_too_many_luts`; too many
  clips at one instant keep `ErrTooManyInputs` and `render_too_many_inputs`). Every clip gets its own `-ss`-seeked
  input; a clip past the decodable video seeks one second before its end and `tpad` holds the last frame. Ranges that
  start or end inside a transition
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
- Layer pieces (clips, gaps, transition heads, mixes and tails) join through `concatPieces`. FFmpeg's concat (5.1
  to 8.1) counts a one-frame video input as zero frames long and moves everything after it one frame early, so a
  concat with a one-frame input before its last renumbers its output (`settb`, then `setpts=N`); every other concat
  keeps its filter byte for byte. `TestSegmentedRenderOneFramePieces` checks the single pass against the source
  frames and cut windows and segmentations against it.
- The server owns persistence, upload staging, job scheduling, and cancellation
  lifecycle. This package stays independent of server state.

Run `go test ./internal/videostudio` for validation, real-frame/audio FFmpeg
integration, and output metadata checks. The opt-in ten-minute local performance
check runs with `AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio
-run '^TestTenMinuteRenderPerformanceOptIn$' -count=1`.
The 2f export benchmark `TestEffectsExportBenchmarkOptIn` is described in the 2f section.

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
  `EffectByID`, `BaseParam`, `Registry.Unavailable`, `RegistryVersion = 1`. The 2a transitions map to xfade `fade`,
  `fadeblack`, `wiperight` (for `wipeleft`) and `wipeleft` (for `wiperight`); the 20 transitions of 2e use their xfade
  names as ids; `validateClip` and `appendTransition`
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
- Per-clip chain (`clip_chain.go`): `clipChain` stages `retime`, `window`, `picture`, `effects`, `animBlur` (2e), the animated crop, `fit`,
  `mask`, `animReveal` (2e), `orient` (`transform` is `fit` + `orient`), `opacity` and `place` own one clip's filters and its geometry (`clipBox`, `evenPixels`, `evenPosition`);
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
- Shared math fixtures: `testdata/parity/<topic>.json` (2a: `geometry`, `transitions`; 2e: `anim`, `xfade`). Go reads them with
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
- Export of keyframed geometry: when x, y, width, height or rotation has keyframes, `clipChain.fit` and
  `clipChain.orient` write per-frame expressions in `t`, never `n` (`clipChain.animatedFit`, which ends at
  `pad@vs_pad_L_C`, then the rotation in `orient`):
  - the `scale … :eval=frame` w/h (`fit`), the `rotate` angle (`orient`), and the `overlay … :eval=frame` x/y (`place`);
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

## Colour and picture (2c)

- Model (`model.go`):
  - Clip fields `crop` (`*Crop{L, T, R, B}`, fractions of the unflipped source; nil when there is no crop), `flip_h`,
    `flip_v` and `effects` (`[]EffectInstance{ID, On, Params, Keyframes}`, at most `MaxEffectsPerClip` = 8) follow
    `ramp`. 2d inserts `blend` and `mask` between `flip_v` and `effects`.
  - Asset kind `lut` with `lut_size` (2..65) before `size_bytes`; at most `MaxLUTAssetsPerProject` (32) per project.
- Validation (`validate_picture.go`): `validateCrop` and `validateEffects` return an error that is a
  `*FieldError{Path, Err}`, and the project PUT's 400 `invalid_project` carries `field`.
  - Crop: each side 0..0.95, l+r and t+b at most 0.95, never all zero (no crop is nil, so the canonical form stays
    unique). Crop, flips and effects only on visual tracks.
  - Effects: known ids and params with registry types and ranges; `params` is required (`{}` at least); a LUT ref is
    `""`, `builtin:<look>` (a registry look) or a project asset of kind `lut`; entry keyframes only on keyframable number
    params (the blur radius is not keyframable).
  - `countKeyframes` counts entry keys toward the 512 per clip; `ValueAt` resolves `crop.*` and `effects.<i>.<param>`.
- Registry (`effects.go`): base rows `crop.l|t|r|b`, `flip_h`, `flip_v`; effects `color`, `lut`, `blur`, `sharpen` and
  `vignette` with the rows of plan index §4.2; looks `warm` … `bleach_bypass`; `ColorWheelK` 0.2, `ColorNeutralKelvin`
  6500. Each `EffectSpec` carries the unexported `build` (`effects_build.go`).
- Shared math (`picture_math.go`, mirrored by `video-studio-picture.js`; parity topics `color`, `wheels`, `picture`,
  `lut`): the colour formula of index §4.20 (white balance, wheels, `ChannelCurve`, saturation matrix), `cropRect`
  (offsets and sizes rounded to even pixels), `mirroredCrop`, `vignetteAngle`, `blurSigma` and the CAS peak constants
  `casPeakP0` (15.104) and `casPeakP1` (4.009).
- LUTs (`lut.go`):
  - `ParseCube` enforces every rule while it reads: lines of at most 1024 bytes, `MaxLUTBytes` (16 MiB) through a limit
    reader, exactly N³ data lines, finite values with |v| ≤ 16, DOMAIN lines before the data, 1D tables refused. Errors
    are `LUTError{Line, Reason}`; `FuzzParseCube` seeds run in the normal suite.
  - `LoadLUTFile` checks for a regular file (no symlinks) and its size before reading; `InspectLUTFile` records the asset.
  - `WriteCube` writes `%.6f` data without the title. `BuiltinLook`/`BuiltinLookCube` build the eight looks on a 33³
    grid from the colour formula (memoized; sha256 pinned by `TestBuiltinLooksDeterministic`).
- Export (`clip_chain.go`, `effects_build.go`):
  - Static crop: `crop` with `iw`/`ih` expressions of `cropRect` (correct under autorotation), then `hflip`/`vflip`, then
    the effects on the cropped picture.
  - Crop keyframes: `picture()` keeps only the flips; the effects run on the whole picture, and
    `crop@vs_picture_L_C` (w/h/x/y per window frame in decoded pixels, mirrored for flips, through `sendcmd`) opens the
    animated-crop stage before `transform()`. `TestRenderAnimatedCropMatchesStatic` checks it against the static crop on
    FFmpeg 5.1.9 and 8.1.1.
  - Every `scale` after `crop@vs_picture` has `eval=frame` and reads `t` in both w and h, with no other size-stateful
    filter between them. On FFmpeg 5.1 a constant-size scale after a commanded crop hangs. `TestClipChainPictureSnapshots`
    checks every animated-crop chain. The t-term costs about 2.3× on 5.1 and is emitted only with crop keyframes.
  - `effects()` builds the `on` entries in stack order. Each builder declares its input pixel format (`format=` only on
    a change) and skips identity entries; sharpen is never skipped.
  - Colour: `lutrgb` curves from `channelCurveExpr` plus a `colorchannelmixer` saturation matrix. LUT: `lut3d=interp=trilinear`
    in four forms (no table or 0: nothing; 1: `lut3d` alone; a static k in between: `blend=all_opacity` ≤ 0.999 with the
    LUT output first; keyframed: `blend=all_expr` with T), index §4.11. Blur: static `gblur` with `sigma = radius·H`
    (image clips premultiplied). Sharpen: `cas`. Vignette: `dither=0`, `eval=frame` only when keyframed, alpha wrapper on
    image clips.
- Commands (`keyframes.go`): targets match `^[a-z][a-z0-9]*@vs_[a-z]+_[0-9]+_[0-9]+(_[0-9]+)?$`, options come from
  `sendCmdOptions`, and curve tracks exist only for `lutrgb@vs_color` (`sendCmdExprOptions`), written through the fixed
  template and checked against `^[0-9a-z.()*/+,-]+$`. Group 0 follows `window()`; group g follows the g-th two-input
  stage (`vs_cmd_<L>_<C>_<g>.txt`), so no command crosses a two-input filter.
- Staging (`render.go`): `stageVideoGraph(dir, graph, rate, files)` writes `graph.txt`, the command files and
  `vs_lut_<n>.cube` (0o600, `WriteCube` of the table resampled onto the unit domain, `writeStagedLUT`). Each distinct
  table of a process is written once: `lutNames` numbers the files from 0 in order of first use (keyed by the `*LUT3D`
  identity, so one look or asset used by several effects is one file), `graphFiles.addLUTs` keeps each name once, and
  `stageVideoGraph` refuses more than `MaxSegmentLUTInstances` (64) LUT files.
  `collectRenderSources` parses the referenced LUT assets with `LoadLUTFile` and takes the decoded (autorotated) size
  from `probeVideo`. The model's `Asset` gains only `LUTSize`; the render side wraps each asset in `renderSource`.
- Capabilities: an `on` effect of an enabled visual clip needs its `Filters` (`effect:<id>`, job error
  `feature_unavailable`); crop and effect keyframes need `sendcmd` (`clipHasKeyframes`).
- Goldens (`goldenframe/cases_2c.go`): media `MediaChart` and `MediaDetail` (PNG) and `MediaCube`; colour, LUT, crop,
  flip, blur, sharpen and vignette cases per plan index §4.12 (effects on `gl`, crop and flip on `gl` and `2d`).
  `color_stack_order` uses `ThresholdDetail` because its stack ends in a vignette. Lowest SSIM per group: plan index §4.21.
- Tests:
  - `go test ./internal/videostudio -run 'Effect|Colou?r|LUT|Cube|Look|Crop|Flip|Blur|Sharpen|Vignette|Picture|Parity|SendCmd|StageVideoGraph|ClipChain|Validate|Capabilit|Segmented' -count=1`
  - `go test ./internal/server -run 'VideoStudio(LUT|BuiltinLUT|Media|Status|ProjectEffects|RenderAssetIDs)' -count=1`
  - The FFmpeg 5.1 container check (plan index §6) runs the gated 2c tests, including
    `TestRenderAnimatedCropMatchesStatic` and `TestRenderLUTMaxGrid`.

## Compositing (2d)

- Model (`model.go`): `Clip.Blend` and `Clip.Mask` sit between `flip_v` and `effects`.
  - `blend` is a registry mode other than `normal`; absent means normal, and Validate refuses `"normal"`, so the
    canonical form stays unique.
  - `mask` is `*Mask{shape, x, y, w, h, rotation, feather, invert}`, always with all eight keys. x and y are the top
    left of the unrotated mask in box fractions (−1..1), w and h are 0.01..2, rotation (−360..360) turns it about its
    centre, and feather (0..0.5) is the full ramp width F as a fraction of the box height.
  - Mask keys are the base paths `mask.x|y|w|h|rotation|feather` in `clip.keyframes`. `shape` and `invert` are never
    keyed.
  - Both fields apply to visual clips only; `validateBlend` and `validateMask` refuse them on an audio track.
- Validation (`validate_picture.go`): `validateBlend` and `validateMask` return `*FieldError` with the paths
  `clips/<id>/blend`, `clips/<id>/mask` and `clips/<id>/mask/<field>`. Mask keys without a mask are refused at
  `clips/<id>/mask`, and out-of-range mask key values are refused by `validateKeyframes` with plain errors.
- Registry (`effects.go`):
  - base rows `blend` (choice, default `normal`), `mask.shape` (rect or ellipse), `mask.x|y|w|h|rotation|feather`
    (keyframable) and `mask.invert`;
  - the 14 `registryBlendModes` with `BlendModeByID` (normal has `Blend ""`; unexported `blendSwapped` = dodge, burn);
  - the effect `chroma_key` (category `key`) with `color` (#00b140, not keyframable), `similarity` (0.25),
    `smoothness` (0.1) and `spill` (0.5), the last three keyframable. Its `Filters` are `colorkey`, `despill`,
    `alphaextract`, `alphamerge` and `blend`. The defaults were kept after the greenscreen golden (index §4.23).

  `Registry.Unavailable` marks a blend mode unavailable when one of `blendModeFilters` (`blend`, `maskedmerge`,
  `alphaextract`) is missing.
- Shared math (`compositing_math.go`, mirrored by `video-studio-picture.js`; parity topics `blend`, `mask`, `key`):
  - the exact 8-bit forms `blend8`, `mix8`, `keyAlpha8` and `despill8`; the gated render tests compare exports with them
    at 0 levels;
  - the real forms `blendChannel`, `compositeChannel`, `maskFrameOf`/`maskFrameAt`/`maskAt` (law L), `keyAlpha` and
    `despillRGB`, which the GL preview evaluates. They differ from the 8-bit forms only by truncation: soft light up to
    4 levels, the other modes up to 2, the key up to 1.
- Export:
  - Chroma key (`buildChromaKeyEffect`, `effects_build.go`):
    - `colorkey=color=0xRRGGBB:similarity=…:blend=…` on rgba, with `despill` only for spill > 0 (type from the key
      colour, mix 0.5, expand 0, the screen channel's scale −spill).
    - Keyed similarity and smoothness go through `colorkey@vs_key_L_C_I` (`similarity`, `blend`) by sendcmd. Keyed spill
      goes through `despill@vs_spill_L_C_I` (`green` or `blue`) by sendcmd; `TestRenderDespillCommands` gates it on
      FFmpeg 5.1.9 and 8.1.1.
    - `colorkeyReplacesAlpha` is true (`TestRenderColorkeyAlphaSemantics`): colorkey replaces an input alpha. The key
      on stills, and a key after an earlier key, therefore runs in the alpha wrapper: `split`, `alphaextract` of the
      input alpha, the keyed picture split again, `alphaextract` of its key, `blend all_mode=multiply` on gray, and
      `alphamerge`. The wrapper is a two-input stage (labels `vs_fx_L_C_I_<a|keyed|k0|k1|ka|m>`); later commands go to
      the next command group, and its `colorkey@vs_key` sits in the split branch and reads the group in front.
  - Mask (`clip_mask.go`, `clipChain.mask`): a two-input stage in box space, after `fit` and before `orient`. The clip
    converts to gbrap there (the 2c-tolerated 5.1 alpha step for packed rgba input). The mask stream is gbrap with
    alpha 255·m, merged by
    `blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply`. The canvas is BW×BH for static geometry, or
    the pad canvas PW×PH with the box centred for keyframed geometry.
    - Static form (no `mask.*` key, static box geometry): one full-resolution `geq` frame of Go numbers, then `loop`,
      trimmed to the window. It costs 0.36 s (8.1.1) and 0.50 s (5.1.9) at 1080p, once per process.
    - Keyframed form (a `mask.*` key, or keyframed box geometry): `geq` per frame in `T` on a grid of ceil(canvas/4) per
      side, then `scale=…:flags=bilinear`. The window start is baked into `T` (`windowSourceExprVar`), so every
      segmentation evaluates the same absolute frames.
    - Row cache: the frame quantities depend on `T` alone. Stage A runs once per row, at the row's first pixel:
      `if(lt(X,0.5), stage A, 0)`. The registers kept for the row are 2 = cx, 5 = cy, 9 = rx, 0 = ry, 1 = cos θ, 7 = sin θ
      and 8 = F. In the keyed form, stage B uses only registers 3, 4 and 6. The static form has every register except
      0, 1, 7, 8 and 9 to itself, because its frame quantities are Go numbers. The window source uses registers 0 and 1
      (2 and 3 with a ramp) in stage A. `maskTerms.kept` selects the layout.
    - `maskExpr` stays within registers 0–9 (the allocation is documented at the top of `clip_mask.go`). The mask branch
      never uses sendcmd or gblur; the opacity commands go to the next command group.
  - Blend (`blend_layers.go`, `buildVideoSegmentWith`):
    - Each track layer splits into runs of one effective mode (`blendRuns`): a clip's frames take its mode, a transition
      takes the shared mode or normal (`overlapBlend`), and transparent frames join a neighbouring run.
    - An all-normal layer keeps the 2c overlay byte for byte (`normalComposite`).
    - Otherwise each sublayer (`splitBlendLayers`) is padded to the window, and the sublayers composite onto the base in
      turn: normal first, then the other modes in registry order (`blendComposite`). A mode composite is
      `blend c0..c2_mode=M` on gbrp (dodge and burn with the inputs swapped), then `maskedmerge=planes=7` with the
      layer's alpha from `alphaextract`. Labels are `vs_bl_<L>_<n>_<part>`.
    - No stage takes runtime commands: never sendcmd on `blend`, `overlay` or `maskedmerge`, and `blend`'s
      `all_opacity` is never used.
- Capabilities (`capabilities.go`): `blend:<mode>` needs `blend`, `maskedmerge` and `alphaextract`
  (`blendModeFilters`); `mask` needs `geq`, `blend` and `loop` (`maskFilters`); chroma_key goes through its `Filters`.
  All of them are in `phase2Filters` too. A missing filter is a `*CapabilityError`, and the job error is
  `feature_unavailable`.
- Costs: index §4.22 (spike) and §4.23 (measured in Task 8). A keyed mask is the expensive part, and it is accepted for
  export.
- Goldens:
  - `goldenframe/cases_2d.go`: media `MediaGreen` (`greenscreen`) and `MediaLayer` (`layer`), and the key, mask, blend,
    blend-transition and combined cases on `gl` (`compositingCases`).
  - `goldenframe/delta.go`: `Delta`, `DeltaFidelity`, `DeltaBlend`, `DeltaMask` and `DeltaKey`. The bounds are a changed
    share of at least 0.02, a slope of 0.9 to 1.1, and r of at least 0.98 (blend) or 0.95 (mask, key). Every 2d case
    carries a Delta. Of the four 2c opt-ins, `color_temperature_tint` and `lut_keyframed_intensity` carry a normal Delta
    since 2f, and `blur_logo_alpha` and `sharpen_static` stay `ReportOnly` (section 2f).
  - Minima and Delta triples: index §4.23.
- Tests:
  - `go test ./internal/videostudio -run 'Blend|Mask|ChromaKey|Colorkey|Despill|Key(8|Alpha|Params)|Composit|Parity|ClipChain|SegmentGraph|Segmented|Validate|Capabilit|Registry|SendCmd|Expr' -count=1`
  - `go test ./internal/videostudio/goldenframe -run 'Delta|CompositingCases' -count=1`
  - `go test ./internal/server -run 'VideoStudio(ProjectCompositing|ProjectEffects|Status)' -count=1`
  - The FFmpeg 5.1 container check (plan index §6) runs the gated 2d tests: `TestRenderChromaKeyMatchesFormula`,
    `TestRenderColorkeyAlphaSemantics`, `TestRenderDespillCommands`, `TestRenderChromaKeyKeyframes`,
    `TestRenderMaskMatchesFormula`, `TestRenderMaskFollowsKeyframedBox`, `TestRenderBlendModesMatchFormula`,
    `TestRenderBlendTransitionModes` and `TestSegmentedRenderCompositing`.
  - `TestCompositingCostRecordOptIn` records the 2d costs (`AURAGO_VIDEO_STUDIO_BENCHMARK=1`) and asserts nothing.

## Transitions and animation (2e)

- Model (`model.go`): `Clip.AnimIn` and `Clip.AnimOut` (`*ClipAnim{preset, duration}`) follow `effects`.
  - The duration is in timeline frames within `AnimFrameRange(rate)`: 0.2 to 3 s, default 0.5 s, rounded half away
    from zero.
  - It is never checked against the clip length. The render clamps each span to floor(duration/2) (`animSpans`).
- Validation (`validate_picture.go`): `validateAnim(track, clip, rate)` returns `*FieldError` with these paths:
  - `clips/<id>/anim_in` (the animation sits on an audio clip);
  - `…/anim_in/preset` (empty or unknown; an unavailable preset validates);
  - `…/anim_in/duration`.
- Registry (`effects.go`):
  - 24 transitions in the categories `basic`, `wipe`, `motion` and `shape`;
  - 8 `AnimPresets` with `Filters`, and `AnimPresetByID`;
  - `Registry.Unavailable` greys a preset when one of its `Filters` is missing. `RegistryVersion` stays 1.
- Shared transition parameters (D1, `transition_math.go`):
  - `transitionFrameAt(kind, i, D, width, height)` is the only source of per-frame transition numbers. They are
    float32, with every operation wrapped in `float32(...)` so that no fused multiply-add changes a value.
  - JS mirrors it in `VideoStudioPicture.transitionParams` (parity topic `xfade`, exact), and `wipeEdge` derives the 2a
    wipes from it.
  - The xfade probe fixture `testdata/xfade/probes.json` (`xfade_probe_test.go`) records xfade itself on 8.1.1.
- fadegrays gate (`capabilities.go`, `fadegraysSelfTest`): when `-h filter=xfade` lists fadegrays, the probe renders a
  fixed Go-written pattern pair twice with 4 filter threads and compares both runs with `fadegraysModel`. A mismatch,
  a difference between the runs or any other failed self-test render adds `xfade:fadegrays` to missing; the result is
  memoized with the probe. The gate passes on FFmpeg 8.1.1 and fails on 5.1.9. The patterns are 64×36 with D = 5 (0.2 s
  at 25 fps; output 34 560 bytes).
  - A probe context that ends (timeout, cancel) is a probe error and is not memoized. The probe then fails closed for
    fadegrays only: a render with an effective fadegrays transition is refused with a `CapabilityError` that wraps the
    probe error, and `CheckRuntime` reports `xfade:fadegrays` missing and `RuntimeStatus.ProbeError`
    (`probe_error`). Other capabilities keep leaving their check to FFmpeg when the probe fails.
  - Accepted caveat: `videoStudioJobFailure` checks context errors first, so a probe that hits its own 10 s timeout, or
    a cancel, reports a timeout or cancelled job, not `feature_unavailable`.
- Animation math (`anim.go`, mirrored by `video-studio-anim.js`; parity topic `anim`):
  - `animAt`, `animBox`, `animRevealPx`, `animBlurMasks`, `animKindsOf`, `animExprs` and `animSpanExpr`;
  - `clipOpacityAt` (`keyframes.go`) multiplies the keyed opacity by the animation's.
- Export:
  - Slides and pop: `clipAnimation` ORs `animKindsOf`, so the whole clip takes the 2b animated-geometry path. `boxExprs`
    applies the animation's scale and offset after the even clamp (registers 7 and 8; register 4 inside `easeExpr`). A
    clip without an effective animation gets the 2d chain byte for byte.
  - Opacity: the `colorchannelmixer@vs_opacity` commands carry keyed × animation opacity per frame.
  - Wipe: `animReveal()` (`clip_anim.go`) is a two-input stage after `mask`: a one-row `geq`, a neighbour upscale and
    `blend c3_mode=multiply`. The merge is gated by `enable` and runs only inside the wipe spans. The one-row `geq` and
    the neighbour scale run for the whole clip. Outside the spans the frames match the clip without a wipe
    (`TestRenderAnimWipeEdges`). On a refusal, `animRevealGated = false` keeps the pixels exact.
  - blur_in: `animBlur()` runs after the effects. It is the {S, S/4, sharp} `maskedmerge` cascade:
    - constant mask branches are built in Go, with `colorchannelmixer@vs_animk_L_C_<n>` driven by sendcmd from their
      own files `vs_cmd_<L>_<C>_m<n>.txt` (carried in `chainStage.files`);
    - the `gblur`s and an alpha layer's `premultiply`/`unpremultiply` carry `enable` for the blur_in spans; the
      `maskedmerge`s carry it only while `animMergeGated` is true (the fallback `false` gives the same pixels, with
      the merges running on every frame);
    - alpha layers use the premultiplied two-merge recipe. Its alpha mask is exact since 2f (see the 2f section).
  - Every value comes from absolute frames (T baked into the expressions and `enable`), so every segmentation renders
    the same frames (`TestSegmentedRenderAnim`).
- Capabilities: an enabled visual clip with an effective animation needs its preset's `Filters` (`anim:<preset>`, job
  error `feature_unavailable`). Transitions keep `xfade:<name>`.
- Costs: index §4.24 (spike) and §4.25 (measured in Task 9 and in the S4-basis run). Slides and pop put the whole clip
  on the animated-geometry path; blur_in and the reveal's merge cost only during their spans.
- Goldens (`goldenframe/cases_2e.go`, `transitionAnimCases`):
  - one case per new transition and per preset, plus combined cases; every case has a Delta;
  - the shared runner skips a case that the local FFmpeg cannot export (`CheckProjectCapabilities` against the probe;
    fadegrays on 5.1).
- Tests:
  - `go test ./internal/videostudio -run 'Anim|Transition|XFade|Xfade|Fadegrays|Parity|ClipChain|SegmentGraph|Segmented|Validate|Capabilit|Registry|SendCmd|Expr|KeyframeExpr|StageVideoGraph' -count=1`
  - `go test ./internal/videostudio/goldenframe -run 'TransitionAnimCases|Delta' -count=1`
  - `go test ./internal/server -run 'VideoStudio(ProjectAnim|Status)' -count=1`
  - The FFmpeg 5.1 container check (plan index §6) runs the gated 2e tests: `TestXfadeProbesMatchFixture`,
    `TestXfadeModelMatchesFixture`, `TestRenderRevealBlendEnableGate`, `TestFadegraysSelfTestOnInstalledFFmpeg`,
    `TestProbeCapabilitiesLocalFFmpegMissesNothing` (exactly `xfade:fadegrays` missing on 5.1.9),
    `TestRenderAnimBoxFollowsReference`, `TestRenderAnimOpacityFollowsReference`, `TestRenderAnimWipeEdges`,
    `TestRenderAnimBlurMatchesCascade` and `TestSegmentedRenderAnim`.
  - `TestAnimCostRecordOptIn` records the 2e costs (`AURAGO_VIDEO_STUDIO_BENCHMARK=1`) and asserts nothing.

## Scopes, performance and polish (2f)

- LUT cap error (`render.go`, `plan.go`):
  - `ErrTooManyLUTs` = `fmt.Errorf("%w: too many simultaneous LUT effects", ErrTooManyInputs)`, so `errors.Is` holds for
    both errors.
  - `tooManyLUTs` and `stageVideoGraph`'s refusal of more than `MaxSegmentLUTInstances` LUT files wrap it.
  - The server's `videoStudioJobFailure` checks it before `ErrTooManyInputs` and returns the code
    `render_too_many_luts` (UI key `videoStudio.tooManyLUTsAtOnce`).
- blur_in alpha masks (`animMaskBranch`, `clip_anim.go`): the alpha branch converts to gbrap before `alphamerge`
  (`[ka<n>]format=gbrap[kq<n>]`, labels `vs_an_<L>_<C>_kq<n>`), so every mask byte equals m on FFmpeg 8.1.1 and 5.1.9
  (`TestRenderAnimBlurMatchesCascade`). The cascade's alpha tolerance is 0 on every plane. The clip chain at rest keeps
  its tolerance 1 below FFmpeg 6 (the closing rgba step, §4.23).
- Goldens (`goldenframe/cases_2c.go`):
  - the native-size guards `blur_logo_alpha_native` (the logo drawn 1:1 in the four quadrants, four overlay tracks over
    the chart, radius 0.02) and `sharpen_static_native` (detail.png drawn 1:1 in the four quadrants, four video tracks,
    strength 0.8), built by `pcLogoTiles` and `pcDetailTiles`;
  - both use ThresholdDetail with DeltaMask's bounds, the same project without the effect as baseline, and are never
    report-only;
  - F2 outcome B: `color_temperature_tint` and `lut_keyframed_intensity` carry a normal Delta; `blur_logo_alpha` and
    `sharpen_static` stay `ReportOnly`, because their weaker preview comes from magnification (GL bilinear against
    swscale bicubic), not from the passes (native slopes 0.9314 and 0.9654; upscaled 0.8660 and 0.8815; index §4.27);
  - 116 registered cases, 74 with a delta check.
- Export benchmark `TestEffectsExportBenchmarkOptIn` (`render_effects_benchmark_test.go`; helpers `effectsBenchLayers`,
  `effectsBenchEntry`, `effectsBenchMedia`, `effectsBenchProject`). It asserts nothing.
  - Project: four full-frame 1080p30 video layers × two effects over 1800 frames, one layer per video track.
  - Variants: `fx`, `plain` (no effects, for the effect share) and `mixed` (blend modes on layers 2–4).
  - Measures: the export through `RenderWithOptions` with the default encoder (best of `AURAGO_VS_BENCH_EXPORT_RUNS`,
    default 3), and the graph's ms per frame (`segmentCost`, default threads and the export's thread limit).
  - Report: lines `vs-export-bench v2 …`.
  - Run: `AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio -run '^TestEffectsExportBenchmarkOptIn$' -count=1
    -v -timeout 150m`. The 5.1 container run passes `AURAGO_VS_BENCH_EXPORT_RUNS=1`. Figures: plan index §4.27.
- No export effect resolution cap (spike S4):
  - effects run on the source frame before scale and transform (spec §7.1);
  - a 1280 × 720 canvas renders as slowly as 1080p, and a cap would change output pixels;
  - the preview's effect cap (O2) lives in the UI only.
- Tests:
  - `go test ./internal/videostudio -run 'AnimBlur|ClipChain|LUT|TooMany|Plan|Segmented' -count=1`
  - `go test ./internal/videostudio/goldenframe -run 'PictureCases|CompositingCases|Delta' -count=1`
  - `go test ./internal/server -run 'VideoStudio(RenderFailureCodes|FeatureUnavailable)' -count=1`
  - The FFmpeg 5.1 container check (plan index §6) runs the gated 2f tests: `TestRenderAnimBlurMatchesCascade` (alpha
    masks), `TestRenderAnimBlurEnableGate`, `TestSegmentedRenderAnim`, `TestLUTTablesAreStagedOncePerProcess` and
    `TestLUTInstancesAreBoundedPerProcess`; `golden.test` runs `TestPictureCasesCoverGroups` and
    `TestCompositingCasesCoverGroups`.
