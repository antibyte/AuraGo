# Video Studio Core Contract

This package owns the Video Studio project model, validation, local media probing,
preview generation, runtime readiness checks, and bounded FFmpeg rendering.

- Inputs passed to FFmpeg are server-staged local regular files. Never accept URLs,
  shell fragments, arbitrary filter expressions, or demuxer names from project data.
- Keep all clip timing in integer frames of the project rate (`ProjectRate`: 24/25/30/50/60 fps, NTSC
  24/30/60 with a 1001 denominator). Convert with `Rate` methods only. Version 1 projects (fixed 30 fps) pass
  through `Upgrade`; `Validate` accepts version 2 only. Projects are limited to sixty minutes and the sizes in
  `CanvasSizes`.
- Bound projects to 8 video, 8 audio and 8 overlay tracks, 2000 clips and 1024 assets. One FFmpeg process uses
  at most `MaxSegmentInputs` (32) inputs. Asset durations are server-owned microseconds (`duration_us`);
  `duration_frames` is derived for the project rate (`AssetAtRate`). Source images/video frames are limited
  to 16,384 pixels per side and 64 megapixels.
- Render through `exec.CommandContext` without a shell, with protocol and demuxer
  allowlists, one decoder thread per input and four output/filter threads, a
  four-hour hard wall-time ceiling, bounded diagnostics, and a 2 GiB output
  ceiling. Preview proxies are complete-source 540p/30 fps MP4s capped at
  512 MiB.
- Preview and export hold the last video frame through longer source audio,
  including clip offsets beyond video EOF; FFmpeg readiness requires `tpad` for
  bounded render padding.
- A transition belongs to its outgoing clip and must equal the explicit overlap
  with the next clip on that same visual track. Other same-track overlaps fail.
  `settb=1/30` follows every concat so later xfades accept their inputs, and
  xfade runs on premultiplied track layers (`premultiply`/`unpremultiply`, both
  required for readiness) so a dissolve does not darken picture-in-picture clips.
- The server owns persistence, upload staging, job scheduling, and cancellation
  lifecycle. This package stays independent of server state.

Run `go test ./internal/videostudio` for validation, real-frame/audio FFmpeg
integration, and output metadata checks. The opt-in ten-minute local performance
check runs with `AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio
-run '^TestTenMinuteRenderPerformanceOptIn$' -count=1`.
