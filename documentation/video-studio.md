# Video Studio

Video Studio is the optional multitrack video editor in the Virtual Desktop.
It edits copied project media, keeps the timeline as JSON, and exports an MP4
with server-side FFmpeg. AI clip generation is optional; editing and exporting
do not require an AI provider.

## Enable and check

Enable the Virtual Desktop, then enable **Video Studio** in its configuration
section. Save the settings and select **Check Video Studio**. The app also
explains unavailable capabilities when opened from the Desktop.

```yaml
video_studio:
  enabled: false
  readonly: false
  ffmpeg_path: ""
  max_asset_size_mb: 4096
  max_project_size_mb: 16384
  max_export_size_mb: 8192
  render_timeout_seconds: 7200
  hardware_encoding: auto
  hardware_device: ""
```

An empty executable path discovers `ffmpeg` and `ffprobe` on the AuraGo server's
PATH. A custom FFmpeg path uses FFprobe in the same directory. Both programs,
H.264 encoding (`libx264`), AAC encoding, and the editor's required filters must
be available. The regular AuraGo Docker image already includes FFmpeg; Video
Studio does not create another container. Native installations must provide a
compatible FFmpeg build separately.

Size settings use MiB (1,048,576 bytes). The asset limit is 1–32768 MiB; the
project limit must be at least the asset limit and at most 65536 MiB. The render
timeout is 30–14400 seconds. These are admission and job limits, not operating
system memory isolation. FFmpeg runs under the AuraGo server account; allow
enough free disk space for source copies, staging, previews, and exports.
The project quota includes its cached previews. Uploads have an overall read
window of at least fifteen minutes (longer for large asset limits, sized for
4 MiB/s) and a thirty-second idle deadline.

Updates raise the limits of installations that still use the earlier defaults (1024 MB / 4096 MB / 3600 s) to the new defaults once; customized values are kept.

## Editing

Create a project in landscape (720p, 1080p, 1440p, 4K), portrait (720p, 1080p, 4K),
square (720p, 1080p, 2160p) or 4:5 (1080 × 1350) format at 24, 25, 30, 50 or 60
fps, or at the NTSC rates 23.976, 29.97 and 59.94 fps. Project settings (project
menu) change the name, format and frame rate. A frame-rate change rounds every
clip time to the new rate in one undoable step. The preview quality next to the
full-screen button lowers the preview resolution for smooth playback of 4K
projects; it does not affect exports. The timeline supports up to sixty
minutes, eight video, eight audio and eight overlay tracks, and 2000 clips.
Projects saved by earlier versions open unchanged at 30 fps. Import
video, audio, and still images from the Desktop or upload them from the browser.
Imports are copied; editing never modifies the original file.

The window has a toolbar (project menu, save state, undo/redo, format, tasks,
keyboard shortcuts, export), a library on the left with tabs for media, text,
stickers and AI clips, a large preview, an inspector for the selected clip on
the right, and a timeline whose height can be dragged. Drop files anywhere into
the window to import them; the library shows upload progress and thumbnails.
Use **+** on a library item to place it at the playhead, or drag it onto a track.
Text presets (title, subtitle, lower third, credits) and stickers land at the
playhead on a free overlay track. A selected picture, title or sticker can be
moved and scaled directly in the preview. The inspector uses seconds
(`4,5` or `1:04.5`) and percent; the shortcut list opens with the keyboard
button or `?`.

The media library, preview, clip inspector, and timeline support trimming,
splitting, moving, duplicating, deleting, snapping, frame stepping, and undo /
redo. Video tracks can be layered for picture-in-picture, with position, size,
rotation, opacity, and contain / cover fit. Music, voice, and video sound have
independent clip volume, mute, and fades. Text and static stickers are image
overlays; editable title text and style remain in the project. Text is rasterized
by the browser before import so preview and export use the same pixels.
If a source's audio continues after its video ends, both preview and export hold
the final video frame for the remaining clip duration, including trimmed tails.

Transitions include dissolve, fade through black, and wipes in both directions.
A transition belongs to the outgoing clip. Adjacent clips overlap by exactly
the transition duration; arbitrary overlap on the same track is rejected.
Place simultaneous elements on separate tracks.

Project saves use strong ETags. A second window cannot silently overwrite a
newer save. The editor retains a small local timeline draft for recovery; copied
media stays on the server. Closing the editor waits for its save guard.

## Exports and background jobs

Exports are rendered in segments of about ten seconds. Every clip is read from its own position in the
source, so late clips in long recordings start quickly. Audio is mixed for the whole export and encoded once.
The server needs free space for the segments, the mixed audio (about 1.4 GB per hour) and the final file; an
export stops early with a clear message when the space is missing. A failed export names the part of the
timeline where it failed. `max_export_size_mb` limits one export file (at most the project limit); a bigger
export stops with a message that names the export size limit or the project storage limit.

Export produces H.264 / AAC MP4 in the Desktop project files, with a download
link. Preview preparation, import probing, rendering, and AI generation are
background jobs. Closing an app window does not cancel a job. Reopening the
project reattaches to its status, and the job list offers explicit cancellation.

Shutdown interrupts unfinished work. Interrupted imports and renders are not
reported as successful. An interrupted AI request may already have incurred a
provider charge; its status is uncertain and AuraGo does not repeat it
automatically. Cancelling local work does not guarantee that a remote provider
cancels or refunds its generation.

The app's read-only switch and Desktop permissions prevent new mutations.
Stopping an existing job remains possible after write access is revoked.
Publication is checked again before a completed background job writes its result.

## Hardware encoding

`hardware_encoding: auto` tests the graphics hardware once (VAAPI, Intel Quick Sync and NVIDIA NVENC on Linux;
NVENC, Quick Sync and AMD AMF on Windows) and uses the first encoder that works for exports and preview
proxies. `off` always uses the CPU (libx264); `vaapi`, `qsv`, `nvenc` or `amf` test only that encoder.
`hardware_device` selects the render node for VAAPI and Quick Sync on Linux (default `/dev/dri/renderD128`).
Decoding and picture composition stay on the CPU; the GPU only encodes. When the GPU
fails during an export, the export restarts once on the CPU; the finished job's result records the encoder
(`encoder`) and the fallback (`encoder_fallback: true`).
"Check Video Studio" shows the chosen encoder. The test result is kept for ten minutes or until the
configuration changes; an export that had to fall back to the CPU tests again next time.
Preview proxies may grow to 2 GiB, enough for hour-long sources.

The Docker image includes VA drivers for AMD (Mesa) and, on amd64, for Intel (iHD for Broadwell and newer,
i965 for older GPUs). Intel Quick Sync (QSV) additionally needs the oneVPL/Media SDK runtime, which the image
does not include, so Intel GPUs encode through VAAPI in the container. Pass the render node into the
container and add the group that owns it:

```yaml
services:
  aurago:
    devices:
      - /dev/dri:/dev/dri
    group_add:
      - "110"   # numeric GID of /dev/dri/renderD128 on the host: stat -c %g /dev/dri/renderD128
```

Native installations: install the VA driver for your GPU (for example `mesa-va-drivers` for AMD,
`intel-media-va-driver` or `i965-va-driver` for Intel) and add the AuraGo service user to the `render` group.

## Optional AI clips

Configure the existing **Video Generation** integration and its provider first.
Video Studio uses the configured model, provider credentials in the Vault,
generation limits, and the existing video-generation budget category. Provider
availability, duration, resolution, and start-image support determine the choices
shown by the editor. Generated clips are imported into the project library and
can be cut like uploaded clips.

A project that has already reached its storage quota cannot start a paid
generation. The quota is checked again before importing the result because
other edits may have used space in the meantime. Local storage or import errors
are reported separately from provider failures; they do not imply a refund.

MiniMax and Veo can receive a supported local start image through their existing
provider-specific image payloads. Agnes supports text-to-video in this app;
local project files are not automatically published to obtain a public image
URL. Images leave the server only after the user submits a supported AI request.
MiniMax has no independent aspect-ratio control in the current adapter, so the
editor does not offer that selection for MiniMax generation. The project's
landscape, portrait, or square export format remains independently selectable.

## Scope and verification

Version 1 focuses on short videos. It does not include 4K, animated stickers,
keyframe animation, automatic subtitles, professional color grading, or an AI
editing assistant. Provider credentials and runtime state are never stored in
project JSON. No agent tool is added by enabling the editor.

Local verification includes model validation, real FFmpeg video/audio fixtures,
file and job permission tests, conflict handling, and browser flows. A local
fixture pass does not establish rendering speed on a particular home-lab server
or acceptance of a paid generation by an external provider.
