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
project limit must be at least the asset limit and at most 65536 MiB. The export
limit is at least 512 MiB (a smaller value from an older configuration is raised to
512 MiB) and never exceeds the project limit. The render timeout is 30–14400 seconds. These are admission and job limits, not operating
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

The timeline has three tools: Selection (`V`), Trim (`T`) and Blade (`B`). Dropping or dragging a clip
overwrites what lies underneath; hold `Ctrl` while dropping to insert instead, which moves later clips on every
unlocked track. Locked tracks never change. With the Trim tool, dragging the cut between two clips rolls it
(hold `Shift` to ripple that edge instead), a free edge ripples the rest of the track, and dragging the middle
of a clip slips its content (`Alt`: slides the clip between its neighbors); the preview shows the edited frame
while you drag. The Blade cuts the clicked clip (`Shift`: every unlocked track). Select several clips with
`Shift`/`Ctrl`+click, a rubber band on empty track space or `Ctrl+A`. `Del` deletes and `Shift+Del` deletes
and closes the gap; the Magnetic toggle closes gaps whenever clips are moved or deleted. `Ctrl+C`, `Ctrl+X`
and `Ctrl+V` copy, cut and paste at the playhead (`Ctrl+Shift+V` inserts), `Ctrl+Alt+V` pastes picture or
sound attributes onto the selected clips, `Ctrl+D` duplicates, `Alt+←`/`Alt+→` nudge by one frame (`Shift`:
ten), `Q`/`W` ripple trim the start or end of the clip under the playhead to it, `N` toggles snapping and
`Shift+Z` fits the timeline. Right-click a clip or empty track space (or press `Shift+F10` on a focused clip)
for a menu with these actions.

Navigation: `J`, `K` and `L` shuttle back, stop and forward; repeated presses double the speed up to eight times, and
`K` with `J` or `L` steps one frame. `I` and `O` set the in and out points at the playhead, `Shift+I` and `Shift+O` go to
them and `Alt+X` clears both; the loop button in the transport plays from in to out (or the whole timeline when they are
not set). `↑` and `↓` jump to the previous or next edit point on the unlocked, visible tracks. `M` adds a marker at the
playhead (a second `M` on a marker opens it), `Shift+M` and `Ctrl+Shift+M` step to the next and previous marker, and
`Ctrl+G` (or a click on the time) goes to a timecode. Markers appear as flags in the ruler: click to go there, drag to
move (snapping to clips, the playhead and the in/out points), double-click to edit the name, colour, note and duration.
The overview strip above the ruler shows the whole project; drag the window to scroll, its edges to zoom. The
Properties panel lists the markers and the in and out points when no clip is selected; its Set in and Set out buttons
set a point at the playhead (the transport's In and Out buttons make room for other controls in a narrow window).
`Shift`+wheel scrolls the timeline sideways.

Tracks: each track header has a kind badge that makes it the paste target (one per kind), a solo button on audio tracks
(it only changes what the preview plays, and is not saved), mute, hide and lock buttons, and a `…` menu. In narrow
windows the mute, hide and lock buttons leave the header and are offered in the `…` menu instead. Double-click a name to
rename it. The track menu (also on right-click) adds a track above or below, sets the height (small, medium or large),
toggles the paste target, solo, mute, hide and lock, and deletes the track: an empty track goes at once, a track with clips
asks first (and a locked track must be unlocked first); the deletion can be undone. Drag a header onto another header of
the same kind to move it. Heights and targets are remembered on this computer for each project.

Clips can be linked: `Ctrl+L` links the selected clips, or unlinks them when they already share a link. Clicking or
selecting a linked clip selects its whole link group; hold `Alt` while clicking to select the clip alone. Delete, copy,
duplicate, nudge and the other commands act on the selection as it is. Dragging a selected clip moves the selection
(hold `Alt` when you start the drag to move only that clip); trims and cuts follow the link group (hold `Alt` to act on
one clip alone, for example for a J or L cut). Detach audio (inspector or clip menu) moves a video clip's sound to an audio clip
on a free audio track, and Reattach audio removes that audio clip again (if the audio clip was deleted on its own,
Reattach audio gives the video clip its sound back; a copy pasted without its audio clip keeps its sound). A link group
holds up to 16 clips; cuts, overwrites and inserts inside linked clips put the parts after the cut into a new group, and
a cut inside a transition is refused for the whole group. Speed (inspector, paste attributes) plays a
clip faster or slower, from 0.25 to 16×: by default the clip keeps its source range and changes length, and Keep
duration changes the source range instead. Audio keeps its pitch up to 4× and is silent above that. `Shift+E` disables
or enables the selected clips: a disabled clip stays on the timeline but is neither shown nor exported. Colour labels
mark a clip's body. Reverse, Freeze frame and Export frame as PNG are in the clip menu and the inspector. Reverse works
on up to five minutes of a clip, counted together with its linked parts of the same media (such as the audio of a J- or
L-cut), and imports a `(reversed)` media item. The reversed clip keeps its place and its linked parts of the same media
are mirrored around it, so picture and sound stay in sync (a J-cut becomes an L-cut); if a mirrored part would not fit
on its track, Reverse is refused until you make room or unlink it; freeze frame inserts a two-second still at the playhead; the PNG download
appears in the tasks list. If the clip changes while the job runs (for example it is slipped so another frame sits at the
playhead), the edit is not applied and the new media stays in the library. Video tracks take still images as well as videos.

Transitions include dissolve, fade through black, and wipes in both directions.
A transition belongs to the outgoing clip. Adjacent clips overlap by exactly
the transition duration; arbitrary overlap on the same track is rejected.
Place simultaneous elements on separate tracks.

Project saves use strong ETags. A second window cannot silently overwrite a
newer save. The editor retains a small local timeline draft for recovery; copied
media stays on the server. Closing the editor waits for its save guard.

### Source monitor

The **Source | Timeline** switch above the preview shows one library item in the source monitor, with its own
transport, `J`/`K`/`L` shuttle, `I`/`O` marks (`Shift+I`/`Shift+O` go to them, `Alt+X` clears them) and a waveform for
audio. Double-click a library item to open it; `Escape` returns to the timeline. Playback stops at the end of the item
(it does not wrap). Marks are remembered per item in this browser.

`,` inserts and `.` overwrites the item on the target tracks (the Video, Audio and Overlay targets); the item stays
loaded when you return to the timeline, so both keys work there too. With timeline in and out set, the item lands at in
with out − in frames, and its in mark decides where the source starts; nothing is stretched, so a source too short for
that length is refused. With only in set, it lands at in with its full length; with only out set, it ends at out; with
neither, it lands at the playhead. Afterwards the playhead moves to the end of the edit. Images last 5 s unless the in
and out points define the length. A video with sound lands as a linked pair (picture detached from its sound, sound on
the audio target) when an audio target is active, otherwise as one clip with its sound.

Drag the **Picture only** or **Sound only** handle onto a track to place only that part at the drop point (`Ctrl`
inserts). A drop places the marked range, or the whole item, and ignores the timeline in and out points; the picture
needs a track that accepts it and the sound needs an audio track, otherwise the drop is refused and nothing changes. A
picture-only video clip keeps its embedded sound silent.

The handles also work from the keyboard and with a plain click: `Enter`, `Space` or a click on **Picture only** or
**Sound only** places that part at the edit position on the matching target track (picture on the Video target, or on
the Overlay target when that one takes the media; sound on the Audio target), overwriting what is there; `Ctrl+Enter`
(or a `Ctrl`-click) inserts instead. The same rules as for `,` and `.` apply (timeline in and out, the playhead, the
marks), the same refusals appear, and the playhead moves to the end of the edit. Opening a library item is by
double-click for now; opening one from the keyboard comes with the library's item menu.

## Media and projects

The media bin shows the open folder as a breadcrumb. Folders are created with **New folder** (use `/` for subfolders, up
to four levels), renamed and deleted from their menus; deleting a folder moves its media and subfolders to the folder
above after a confirm. Drag media onto a folder or a breadcrumb crumb to move it, or use the item menu's **Move to
folder**. Switch between grid and list with the view buttons; in the list, click a column header to sort (click again to
reverse). Search looks through every folder. The item menu opens a clip in the source monitor, renames it, moves it,
selects the clips that use it, or removes it. Removing media that the timeline uses first takes its clips off the timeline
(one undo step, gaps stay) and then deletes the file; **Remove unused media** deletes every item no clip uses. The inspector
shows the metadata of the selected media. Visual media gets a poster when it is imported; older media shows its thumbnails.

Each project keeps automatic versions: when you save and the newest version is at least ten minutes old, the previous
saved state is kept (up to 20). **Save version now** keeps a copy with an optional label (up to 10). Versions count toward
the project's storage limit. Restoring a version first saves your changes and keeps the current state as a version, then
reloads the restored project; a version that uses media you have since removed cannot be restored.

**All projects…** in the project menu lists every project with its poster, format, length, storage and last change. Open,
rename, duplicate or delete from there. A duplicate copies the project file, its media and posters (not exports, previews
or versions) in the background and shows up in the list when it is ready. Deleting a project asks you first and cancels
its running tasks.

Export can render only the in/out range when both points are set: the export dialog offers the range with its times, and
the download is named `<project> (in-out).mp4`.

## Exports and background jobs

Exports are rendered in segments of about ten seconds. Every clip is read from its own position in the
source, so late clips in long recordings start quickly. Audio is mixed for the whole export and encoded once.
The server needs free space for the segments, the mixed audio (about 1.4 GB per hour) and the final file; an
export stops early with a clear message when the space is missing. A failed export names the part of the
timeline where it failed. `max_export_size_mb` limits one export file (at most the project limit); a bigger
export stops with a message that names the export size limit or the project storage limit.

Export produces H.264 / AAC MP4 in the Desktop project files, with a download
link. Preview preparation, import probing, rendering, project duplication, and
AI generation are background jobs. Closing an app window does not cancel a job. Reopening the
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
