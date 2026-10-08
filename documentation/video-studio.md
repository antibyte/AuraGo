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
  max_asset_size_mb: 1024
  max_project_size_mb: 4096
  render_timeout_seconds: 3600
```

An empty executable path discovers `ffmpeg` and `ffprobe` on the AuraGo server's
PATH. A custom FFmpeg path uses FFprobe in the same directory. Both programs,
H.264 encoding (`libx264`), AAC encoding, and the editor's required filters must
be available. The regular AuraGo Docker image already includes FFmpeg; Video
Studio does not create another container. Native installations must provide a
compatible FFmpeg build separately.

Size settings use MiB (1,048,576 bytes). The asset limit is 1–8192 MiB; the
project limit must be at least the asset limit and at most 65536 MiB. The render
timeout is 30–14400 seconds. These are admission and job limits, not operating
system memory isolation. FFmpeg runs under the AuraGo server account; allow
enough free disk space for source copies, staging, previews, and exports.
The project quota includes its cached previews. Uploads have a fifteen-minute
overall read window and a thirty-second idle deadline.

## Editing

Create a project in landscape, portrait, or square format at 720p or 1080p.
The timeline uses 30 frames per second and supports up to ten minutes, four
video tracks, four audio tracks, and four overlay tracks. Projects support up to
256 clips and 32 distinct media inputs used by an export. Import
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
