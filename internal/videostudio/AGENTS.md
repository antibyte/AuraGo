# Video Studio Core Contract

This package owns the Video Studio project model, validation, local media probing,
preview generation, runtime readiness checks, and bounded FFmpeg rendering.

- Inputs passed to FFmpeg are server-staged local regular files. Never accept URLs,
  shell fragments, arbitrary filter expressions, or demuxer names from project data.
- Keep all clip timing in integer 30 fps frames. Project renders are limited to
  ten minutes and the supported 720p/1080p landscape, portrait, and square sizes.
- Bound projects to 4 video, 4 audio, and 4 overlay tracks, 256 clips, 512
  assets, and 32 distinct active render inputs. Source images/video frames are
  limited to 16,384 pixels per side and 64 megapixels.
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
- The server owns persistence, upload staging, job scheduling, and cancellation
  lifecycle. This package stays independent of server state.

Run `go test ./internal/videostudio` for validation, real-frame/audio FFmpeg
integration, and output metadata checks. The opt-in ten-minute local performance
check runs with `AURAGO_VIDEO_STUDIO_BENCHMARK=1 go test ./internal/videostudio
-run '^TestTenMinuteRenderPerformanceOptIn$' -count=1`.
