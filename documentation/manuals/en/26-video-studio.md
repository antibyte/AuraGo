# Chapter 26: Video Studio – edit videos in the Virtual Desktop

Video Studio is the multitrack video editor in the Virtual Desktop. You arrange clips, music and pictures on a timeline,
add titles, effects and transitions, and export an MP4 file that AuraGo renders on the server with FFmpeg. It is meant
for projects of up to 60 minutes. It is not a camera app, a subtitle generator or an AI editing assistant, and it is off
by default.

Technical details for administrators are in the [operator reference](../../video-studio.md).

## Table of Contents
1. [Switching It On](#switching-it-on)
2. [First Steps](#first-steps)
3. [Media and the Timeline](#media-and-the-timeline)
4. [Keyframes and Speed Ramps](#keyframes-and-speed-ramps)
5. [Colour, Effects, Masks and Keying](#colour-effects-masks-and-keying)
6. [Transitions and Animations](#transitions-and-animations)
7. [Preview, Range Check and Export](#preview-range-check-and-export)
8. [Projects and Versions](#projects-and-versions)
9. [Optional AI Clips](#optional-ai-clips)
10. [Good to Know: Limits](#good-to-know-limits)
11. [Keyboard Shortcuts](#keyboard-shortcuts)
12. [Privacy and Safety](#privacy-and-safety)
13. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Switching It On

Video Studio is off by default. An administrator switches it on under **Config → Virtual Desktop**:

- **Enable Virtual Desktop** must be on first. The Video Studio settings appear only then.
- **Config → Virtual Desktop → Video Studio → "Enable Video Studio"** (`video_studio.enabled`, default `false`). The
  configuration page asks for no restart; the server reads the saved values on each request.
- **Read-only projects** (`video_studio.readonly`, default `false`) opens every project read-only, as does the Virtual
  Desktop's **Read-only mode**. You can still play and stop running tasks, but nothing changes.
- **Check Video Studio** reports whether the server is ready. Save first. A good result says "Video Studio is ready."
  and names the encoder.
- **FFmpeg:** the Docker image includes FFmpeg and FFprobe. Native installations need both on the `PATH`, or a path set
  under **FFmpeg executable (empty: automatic)** (`video_studio.ffmpeg_path`). Effects and transitions that the
  server's FFmpeg cannot render are greyed out in the editor.
- **Agent access:** enabling Video Studio adds no agent tool, so the agent cannot open or change your projects. AI
  clips use the separate Video Generation integration.

While it is off, the app still appears in the start menu (category **Creative**). It then shows "Video Studio is
disabled in Settings."

## First Steps

1. Open **Video Studio** from the start menu. It opens maximized.
2. Click **New project**, choose a name, format and frame rate, then click **Create project**. Formats are landscape
   720p, 1080p, 1440p or 4K; portrait 720p, 1080p or 4K; square 720p, 1080p or 2160p; and 4:5 at 1080 × 1350. Frame
   rates are 24, 25, 30, 50 and 60 fps, plus the NTSC rates 23.976, 29.97 and 59.94 fps.
3. Add media with **Upload**, with **From files** (Desktop files), or by dropping files into the window.
4. Press **+** on a library item to place it at the playhead, or drag it onto a track. Play with the transport buttons
   or with `J`, `K` and `L`.

## Media and the Timeline

Imports are copied into the project, so your original file never changes. The file picker offers video, audio, PNG and
WebP images, and `.cube` LUT files. Common formats work, for example MP4, MOV, MKV, WebM, AVI, MP3, WAV, FLAC and GIF.
A video whose longer side is more than 16 times its shorter side is refused.

The timeline has up to eight video, eight audio and eight overlay tracks, with up to 2000 clips. Dropping a clip
overwrites what lies under it. Hold `Ctrl` while dropping to insert it instead. Each track header has mute, hide and
lock buttons and a `…` menu for height, paste target and deletion.

- **Tools:** Selection tool (`V`), Trim tool (`T`) and Blade tool (`B`). With Trim, drag a cut to roll it, or drag a
  free edge to ripple the track. With Blade, click a clip to cut it.
- **Editing:** `Del` deletes, `Shift+Del` deletes and closes the gap. **Snap** (`N`) snaps edits to clips, the playhead
  and the in and out points. **Magnetic** closes gaps when you move or delete clips.
- **Sound:** `Ctrl+L` links the selected clips, so a video and its sound move together (again to unlink).
  **Detach audio** gives the sound its own clip, and **Reattach audio** brings it back. Speed runs from 0.25× to 16×.
  Audio keeps its pitch up to 4×.
- **Overlays:** stack video tracks for picture-in-picture. The **Title**, **Subtitle**, **Lower third** and **Credits**
  presets and stickers land at the playhead on an overlay track. Stickers are static.
- **Source monitor:** double-click a library item to open it. The **Source** and **Timeline** switch sits above the
  preview. `I` and `O` mark in and out points, `,` inserts and `.` overwrites on the target tracks. Drag **Picture
  only** or **Sound only** onto a track to place one part.

## Keyframes and Speed Ramps

Position, size, rotation, opacity, crop and effect values can change over time. Click the stopwatch next to a value in
the inspector's **Picture** section. The current value becomes the first keyframe, and later changes at the playhead add
keyframes. The diamond adds or removes one, and ◀ and ▶ jump between them. Keyframes stay with the picture, so trims and
splits keep them on the same frame. A property holds 64 keyframes, a clip 512.

Speed ramps are in the **Animation** section of a video clip: **Ramp up**, **Ramp down**, **Slow middle**, **Custom** and
**None**. **Keep duration** keeps the clip's length. For a custom curve, choose **Edit in the timeline** and drag points
in the speed lane. A ramp holds up to 16 points. A ramped clip's own sound is muted; **Detach audio** keeps it.

## Colour, Effects, Masks and Keying

**Crop and flip** are in the **Picture** section, and **Crop in the preview** lets you drag the crop on the picture. Each
visual clip holds up to eight effects. The library's **Effects** tab groups them under **Color**, **Detail**, **Stylize**
and **Key**.

- **Color:** exposure, contrast, saturation, temperature, tint and lift, gamma and gain colour wheels.
- **LUT:** one of eight built-in looks or a project LUT, or **Upload .cube**. Only 3D `.cube` files work: 2 to 65 points
  per axis, at most 16 MB each, 32 per project.
- **Blur**, **Sharpen** and **Vignette** are the other basic effects.
- **Chroma key:** set the **Key color** (or press **Green screen** or **Blue screen**), raise **Similarity**, then
  soften the edge with **Smoothness**. **Spill suppression** removes colour fringes.

The **Blend mode** list mixes a clip with the tracks below it: Normal, Multiply, Screen, Overlay, Soft light, Hard light,
Darken, Lighten, Difference, Exclusion, Add, Subtract, Color dodge and Color burn. **Mask shape** offers **None**,
**Rectangle** and **Ellipse**. **Feather** softens the edge and **Invert mask** shows the outside. The mask follows its
clip when you move, scale or rotate it.

The WebGL2 preview draws effects, blend modes, masks and keying with the export's formulas. The simple 2D preview shows
crop and flip, but no effects, blend modes or masks, and says so.

## Transitions and Animations

A transition joins a clip to the next clip on the same video or overlay track. The clips overlap by its length, and the
transition belongs to the outgoing clip. Open **Transition to next clip** in the inspector and pick one of 24 types in
four groups: **Basic transitions** (Dissolve, Through black, Through white, Through grayscale, Pixelate), **Wipes**,
**Push and zoom** and **Shapes**. A new transition is half a second long, up to three seconds, and shorter than both
clips. Types that the server's FFmpeg cannot render are greyed out.

Pictures, titles and stickers also have an **In animation** and an **Out animation**: **Fade**, **Slide up**, **Slide
down**, **Slide left**, **Slide right**, **Pop**, **Wipe** and **Blur**. Each lasts 0.2 to 3 seconds, half a second by
default.

## Preview, Range Check and Export

The timeline preview uses WebGL2 when the browser offers it. If it is missing or fails, the preview switches to a simpler
2D drawing and shows a notice. Editing and export are not affected. **Preview quality** (next to full screen) plays the
**Full** originals, or **Auto**, **Half** or **Quarter** previews. These are 960 × 540 proxies that are made in the
background after an import. If a proxy fails, the original plays.

**Render range exactly** renders the in and out range, or five seconds around the playhead, through the export pipeline.
Ranges over 60 seconds are refused. Switch between **Rendered** and **Live preview** to compare; `Escape` closes the
comparison.

**Export**, then **Export video**, offers **Whole project** or **Only the in/out range**. The result is an H.264 / AAC MP4
with a download link. Exports render in the background in segments of about ten seconds, so you can keep editing. Follow
them in **Tasks**; **Cancel job** stops one. Closing the window does not cancel a task. The server needs free disk space
for the segments, the mixed audio and the output file.

Admins set **Hardware encoding** (`video_studio.hardware_encoding`). **Automatic**, the default, tests the graphics
hardware and uses the first encoder that works; the result is kept for ten minutes. The GPU only encodes. If a GPU
export fails, it restarts once on the CPU. **Off (CPU only)** always uses the processor.

## Projects and Versions

**All projects…** in the **Project** menu lists your projects with their size and last change. You can open, rename,
duplicate or delete them there. A copy keeps the project file, its media and posters, but not exports or versions.
Deleting a project removes its media, exports and versions and cancels its tasks. This cannot be undone.

When you save and the newest version is at least ten minutes old, the previous state is kept as an automatic version (up
to 20). **Save version now** adds a version with an optional label. Up to 10 manual versions are kept. **Restore** saves
your changes first. A version that uses media you have since removed cannot be restored.

## Optional AI Clips

The **AI** tab in the library creates short clips from a text prompt through the provider set under
**Config → Video Generation** (`video_generation.enabled`, default `false`). Video Studio's own switch is not enough: the
integration must be on, with a provider and its API key. Otherwise the tab explains what is missing. The provider decides
which durations, resolutions and start images you can pick. Generation uses the existing `video_generation` budget and
stops when the budget is used up or the project is at its storage limit. Finished clips appear in the library.

## Good to Know: Limits

| Item | Limit |
|---|---|
| Project length | 60 minutes |
| Tracks | 8 video, 8 audio, 8 overlay |
| Clips and markers | 2000 clips, 500 markers per project |
| Media file | 4096 MB by default (`max_asset_size_mb`, 1–32768) |
| Project storage | 16384 MB by default (`max_project_size_mb`, 1–65536), including previews and versions |
| Export file | 8192 MB by default (`max_export_size_mb`, at least 512 MB) |
| Render timeout | 7200 seconds by default (`render_timeout_seconds`, 30–14400) |
| Source picture | 16384 px per side, 64 megapixels; longer side at most 16 times the shorter side |
| Frame rates | 24, 25, 30, 50, 60 fps; NTSC 23.976, 29.97, 59.94 fps |
| Speed | 0.25× to 16× |
| Effects, keyframes, ramp points | 8 effects per clip; 64 keyframes per property, 512 per clip; 16 ramp points |
| Reverse | up to 5 minutes of source |
| Range check | up to 60 seconds |
| LUT files | 3D `.cube`, 2–65 points per axis, 16 MB each, 32 per project |

- Projects are saved in format version 3. Older AuraGo builds cannot open them, so update every installation that uses
  the same projects.
- Not available: animated stickers, automatic subtitles and an AI editing assistant.

## Keyboard Shortcuts

| Key | Action |
|---|---|
| `V`, `T`, `B` | Selection, Trim and Blade tools |
| `J`, `K`, `L` | Shuttle back, stop, forward |
| `I`, `O` | Set the in or out point at the playhead |
| `Shift+I`, `Shift+O`, `Alt+X` | Go to the in or out point; clear both |
| `M`, `Shift+M`, `Ctrl+Shift+M` | Add a marker; next marker; previous marker |
| `Ctrl+G` | Go to a timecode |
| `N`, `S` | Toggle snapping; split the selected clips at the playhead |
| `Del`, `Shift+Del` | Delete; with Shift, close the gap too |
| `Ctrl+C`, `Ctrl+X`, `Ctrl+V` | Copy, cut, paste at the playhead (`Ctrl+Shift+V` inserts) |
| `Ctrl+Alt+V` | Paste picture or sound attributes onto the selected clips |
| `Ctrl+D`, `Ctrl+A`, `Ctrl+L` | Duplicate; select all; link or unlink clips |
| `Alt+←`, `Alt+→` | Move the selection one frame (with Shift: ten frames) |
| `Shift+E`, `Shift+Z` | Disable or enable the selected clips; fit the timeline |
| `↑`, `↓` | Previous or next edit point |
| `,`, `.` | Insert or overwrite the source monitor item |

## Privacy and Safety

- **Local processing.** Imports are copied. Editing, proxies, range checks and exports run on your AuraGo server with
  FFmpeg. The browser only draws the preview.
- **Storage.** Project media, posters, exports and versions live in the Desktop files under
  `Documents/Video Studio/<project-id>/`, by default inside `agent_workspace/virtual_desktop/`. Job state and preview
  files are kept under `data/video_studio/`.
- **Leaving the machine.** Only an AI clip request that you submit leaves the server: your prompt and the start image
  you chose. Provider keys are never written into project files.
- **Audit and agent.** Video Studio writes no audit entries of its own, and the agent has no access to it. The
  read-only switches block every change. Stopping a running task still works.

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Video Studio is disabled in Settings." | An administrator turns on **Enable Video Studio** and **Enable Virtual Desktop**. |
| "FFmpeg is not available on this server." | Install FFmpeg and FFprobe, or set the path under **FFmpeg executable**, then click **Check Video Studio**. |
| "Enable Video Studio and install FFmpeg/FFprobe on the server. Open the app for details." | The check failed. Fix the two points above, save, and check again. |
| "Read-only mode prevents editing." | Read-only projects or the desktop's **Read-only mode** is on. An administrator must turn it off. |
| "Save failed" | Check the connection, then make the change again. |
| "Not saved: changed elsewhere" | Another window changed the project and autosave is paused. Click the chip and choose the version to keep. |
| "The project changed again. Review the conflict and try again." | Choose the version to keep, then try again. |
| "Could not load the project. Check your connection and try again." | Check the connection and reload the window. |
| "Could not import this media." | Check that another player opens the file. Video Studio reads it with FFprobe. |
| "This file is larger than the server limit." | Use a smaller file, or ask an administrator to raise the media size limit. |
| "This project has reached its storage limit. Remove media or use another project." | Remove unused media (**Remove unused media**) or delete old versions. |
| "This video is too wide or too tall. Its longer side may be at most 16 times its shorter side." | Crop or convert the video outside the editor, then import it again. |
| "The project reached its 60-minute limit." / "This project reached its clip limit." | Shorten the timeline or remove unused clips. |
| "Render range exactly works on up to 60 seconds. Move the in and out points closer together." | Shorten the in and out range. |
| "Rendering failed between … and …. Check the clips there and try again." | Check the clips in that part, then export again. |
| "The video is larger than the export size limit set in the Video Studio settings." | Export only a range, or ask an administrator to raise the export size limit. |
| "The server does not have enough free disk space for this export." | Ask an administrator to free disk space on the server. |
| "This task could not be completed." | Open **Tasks**, check the job and try again. |
| "The server's FFmpeg lacks a filter this project needs. Update FFmpeg or remove that effect or transition." | Remove that effect or transition, or ask an administrator to update FFmpeg. |
| "AI video generation is not set up. An administrator can enable it under Configuration → Video generation." | An administrator configures **Video Generation**. |
| "The video generation budget is used up for now." | Wait, or ask an administrator to check the budget. |
| "The provider status is unclear; it may still be processing. Check its status before making another paid request." | Check the provider's status before you start another paid generation. |
| "This .cube file cannot be used: line … is not valid." | Use a 3D `.cube` file with 2 to 65 points per axis. |
| "Preview proxy failed; the original plays" | The original plays. Use **Create preview proxy** in the item menu to try again. |

If the preview is slow or shows graphics errors, you can force the simple 2D preview in this browser only. In the browser's
developer console, run `localStorage.setItem('aurago.videoStudio.previewEngine', '2d')` and reopen Video Studio.
`localStorage.removeItem('aurago.videoStudio.previewEngine')` turns it off again.

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 25: CYD Creator](./25-cyd-creator.md)
**Next Chapter:** [Chapter 27: Bookshelf](./27-bookshelf.md)
