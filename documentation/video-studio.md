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

Newer effects and transitions need further FFmpeg filters. **Check Video Studio** and the editor report what the
server's FFmpeg lacks; the editor greys those choices out, and an export or range check of a project that uses one
stops at once with a message saying so.

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
Imports are copied; editing never modifies the original file. Videos wider or taller than 16:1 are refused
on import. Export, range render and frame export refuse a timeline that uses such a video, with a message.
Still images are not limited. A very wide or tall still on a cover (fill) clip is trimmed around its centre to at most
16 times the clip's shape before scaling; the clip shows the same picture and the export no longer runs out of memory.

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
L-cut), and imports a `(reversed)` media item in your language; reversing a reversed clip again gives back the plain
name instead of a second suffix. The reversed clip keeps its place and its linked parts of the same media
are mirrored around it, so picture and sound stay in sync (a J-cut becomes an L-cut); if a mirrored part would not fit
on its track, Reverse is refused until you make room or unlink it; freeze frame inserts a two-second still at the playhead; the PNG download
appears in the tasks list. Export frame as PNG saves the clip's original source frame at the playhead, without transform,
crop, flips, effects or LUT. Exporting the graded frame is not offered yet. If the clip changes while the job runs (for example it is slipped so another frame sits at the
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
reverse). In a narrow window the list fits the library without sideways scrolling: the name, and below it the duration
and the column you sorted by. On a phone, or in a window 650 px wide or less, the library is a strip below the preview:
grid and list scroll up and down, and the list shows the name, the duration and the column you sorted by (also resolution
and size when there is room); search and the type filters need a wider window. Search looks through every folder. The item menu opens a clip in the source monitor, renames it, moves it,
selects the clips that use it, or removes it. Removing media that the timeline uses first takes its clips off the timeline
(one undo step, gaps stay) and then deletes the file; **Remove unused media** deletes every item no clip uses. While the
files are deleted, editing and undo wait a moment. If a removal is interrupted (for example by a restart), the media is
already gone from the bin; removing it again finishes the job, and so do the next start and the next time the bin loads,
which also clears files that interrupted imports left behind. A window that has the same project open drops removed media it does not
use the next time it saves; if its timeline still uses that media, it asks which version to keep, and **Replace the latest
version** keeps its changes without the removed media and the clips that used it. The inspector shows the metadata of the
selected media. Visual media gets a poster when it is imported; older media gets one in the background (while no export or other task runs) and shows it the next time the bin loads.

Each project keeps automatic versions: when you save and the newest version is at least ten minutes old, the previous
saved state is kept (up to 20). **Save version now** keeps a copy with an optional label (up to 10). Versions count toward
the project's storage limit. Restoring a version first saves your changes and keeps the current state as a version, then
reloads the restored project; a version that uses media you have since removed cannot be restored.

**All projects…** in the project menu lists every project with its poster, format, length, storage and last change. Open,
rename, duplicate or delete from there. A duplicate copies the project file, its media and posters (not exports, previews
or versions) in the background and shows up in the list when it is ready; a copy that the server could not finish (it
stopped during the copy) is removed when it starts again. Deleting a project asks you first and cancels its running tasks.

Export can render only the in/out range when both points are set: the export dialog offers the range with its times, and
the download is named `<project> (in-out).mp4`.

## Keyframes and speed ramps

**Animate a clip.** The position, width, height, rotation and opacity of a picture, title or sticker can change over
time. In the inspector's Picture section each of them has a stopwatch: turn it on and the current value becomes the
first keyframe at the playhead. The diamond next to it adds or removes a keyframe at the playhead, and ◀ ▶ jump to the
previous or next keyframe. While a property is animated, every change at the playhead sets a keyframe there: a slider,
a field, a position button, Full frame or dragging the box in the preview. A new keyframe takes the easing of the
keyframe before it. Turning the stopwatch off removes the property's keyframes and keeps the value at the playhead. The
keyframe controls work only while the playhead is inside the selected clip.

**Keyframes stay with the picture.** A keyframe belongs to a frame of the source, not to a place on the timeline.
Trimming, splitting, slipping, sliding and speed changes keep every keyframe on the same picture content. A split copies
the keyframes to both parts, so the motion continues across the cut. Keyframes outside the visible part of a clip are
kept. Reverse mirrors them, Freeze frame takes the values at the frozen frame, and a frame-rate change rescales them.
Paste attributes has a Keyframes group that copies them with the same timing relative to the clip start. Its Picture
group writes a key at the playhead for each animated value and changes the field only for static values; an animated
value is skipped while the playhead is outside the clip. With the Keyframes group in the same paste, the target's keys
are replaced instead.

**Easing.** Between two keyframes the easing of the first one applies:
- Linear.
- Ease in, Ease out, and Ease in and out (smooth cubic curves).
- Hold: the value jumps at the next keyframe.

Before the first and after the last keyframe the value stays constant. A property holds up to 64 keyframes and a clip
up to 512.

**Keyframe strip.** When the selected clip has keyframes or a speed ramp, a strip opens below its track. Its summary row
has a diamond for every frame that holds keyframes, and the chevron shows one row per property and the speed lane. A
diamond's shape shows its easing: a diamond for Linear, a circle for the smooth curves, a square for Hold.
- Click a diamond to select it; `Shift` adds to the selection.
- Drag a diamond along the timeline in whole frames; with snapping on, it snaps to the playhead.
- `←`/`→` move the selection by one frame (`Shift`: ten), and `Del` deletes it. `Del` never deletes the clip there.
- Double-click a diamond to put the playhead on it.
- Right-click a diamond, or press `Shift+F10`, for its easing, Copy keyframes, Paste keyframes at the playhead and Delete.

While keyframes are copied, the strip also opens under other picture clips, so the keyframes can be pasted there with
the same spacing.

**Speed ramps.** A video clip on a video track has an Animation section in the inspector with speed ramp presets:
- Ramp up: up to twice the speed, easing in and out.
- Ramp down: from twice the speed down.
- Slow middle: a quarter of the speed between 40 % and 60 % of the clip.
- Custom: a flat ramp that you shape in the speed lane.
- None: back to 1×.

Presets start from the clip's speed (1× when it already has a ramp). They keep the clip's source range and move the
later clips on the track, like a speed change. With **Keep duration** checked they keep the clip's length and use more
or less of the source, which needs enough media.

**Edit in the timeline** opens the speed lane. It draws the speed on a logarithmic scale, from 0.25× at the bottom to
16× at the top. In the lane:
- Drag a point sideways to move it, and up or down to change its speed.
- `←`/`→` move the point by a frame (`Shift`: ten), and `↑`/`↓` change its speed (`Shift`: by a factor of two).
- Double-click the lane to add a point, and press `Del` to remove one.
- Right-click a point for its easing.

A ramp holds up to 16 points. An edit the media cannot hold is not applied.

While a clip has a ramp, its Speed field is empty and shows **Ramp**. Entering a number replaces the ramp with that
constant speed; undo brings the ramp back. Slip and Reverse are not available on a ramped clip: reverse it first, then
add the ramp. Reverse is also unavailable when another clip of the same media that is linked to this one has a ramp;
its menu item and button are disabled. Freeze frame and Export frame as PNG take the frame the ramp shows at the playhead.

**Sound of ramped clips.** A ramped clip's own sound is muted in the preview and in the export. The inspector says so in
the Animation and Sound sections, and the clip shows a ramp badge with a crossed-out speaker. Use **Detach audio** to
keep the sound: the detached audio clip plays at normal speed and is not ramped.

**Shortcuts.**
- `Alt+K` adds or removes keyframes of every animated property of the selected clip at the playhead.
- `Alt+↑` and `Alt+↓` jump to the previous or next keyframe.
- `Del` in the keyframe strip deletes the selected keyframes.

The shortcuts also work on keyboards where Option or AltGr changes the typed character, and while a button has focus.
In a text or number field they keep their usual meaning there. The keyboard overview lists them.

**Preview and export.** The preview shows keyframes and ramps with the same formulas as the export, in the WebGL2 and in
the 2D drawing; golden tests compare both with FFmpeg's frames. The export moves and turns animated pictures exactly on
every frame, whatever segments it renders in. Exports of keyframed clips need FFmpeg's `sendcmd` filter, which the
FFmpeg 5.1 builds of supported systems include. When it is missing, **Check Video Studio** reports it and such an export
stops with a message.

## Colour and picture

**Crop and flip.** The Picture section of a video, image or sticker crops it by percent of its source on each side
(left, top, right and bottom; at most 95 % per side and per pair of opposite sides) and flips it horizontally or
vertically. Crop values always refer to the unflipped source, so a flip keeps the crop on the same content. Titles
take no crop or flip.

**Crop in the preview** shows the crop on the picture. A dashed outline marks the whole source, and the crop has
handles on its corners and edges. Use the **Crop in the preview** button in the inspector while the playhead is inside
the clip. Cropping ends when you select another clip, move the playhead out of the clip, or press `Enter` or `Esc`.
- Drag a handle to crop, or drag inside the crop to move it.
- With the crop focused, the arrow keys move it by 1 % (`Shift`: 10 %); on a focused handle they move that edge.
- Each drag and each key press is one undo step.

Crop values have keyframes like the position: with a crop stopwatch on, a drag or key at the playhead sets a keyframe
there.

**Effects.** The Effects section below Picture holds up to 8 effects per visual clip, titles and stickers included,
applied from top to bottom. **Add effect** lists the effects by category. Each effect can be collapsed, switched off,
reset and removed. Drag its grip, or press `Alt+↑` or `Alt+↓` on its header, to change the order. Every number except
the blur radius can have keyframes, with the same stopwatch and diamonds as the position.
- **Color:** exposure (in stops), contrast, saturation, temperature (in kelvin; higher values warm the picture), tint
  (positive values add magenta), and three colour wheels for lift (shadows), gamma (midtones) and gain (highlights).
  Drag on a wheel to set its point: the angle is the hue and the distance from the centre is the strength. The arrow
  keys move the point (`Shift`: larger steps), and a double-click or `Del` resets it. Each wheel's level has its own
  slider.
- **LUT:** a look table with an intensity. Its **Look** list offers none (no change), the eight built-in looks (Warm,
  Cool, Teal and orange, Black and white, Faded film, High contrast, Vintage, Bleach bypass) and the project's LUTs.
  **Upload .cube** imports a file and selects it for that effect.
- **Blur:** a soft blur. Its radius is a share of the picture height (up to 5 %), so it looks the same at every size.
- **Sharpen:** contrast-adaptive sharpening with a strength.
- **Vignette:** darkens towards the corners, with an amount and a size (a share of the picture height).

**Crop before or after the effects.** With a fixed crop, the effects see only the cropped picture. When the crop has
keyframes, the effects run on the whole picture first and the crop follows. The Effects section says so: a vignette
then centres on the uncropped picture, and a blur uses the full picture height.

**LUT files.** Import `.cube` files like other media: upload them, drop them into the window, pick them from the
Desktop files, or use **Upload .cube** in a LUT effect. Video Studio accepts 3D LUTs:
- 2 to 65 points per axis (`LUT_3D_SIZE`), optionally with `DOMAIN_MIN`/`DOMAIN_MAX` or `LUT_3D_INPUT_RANGE`;
- at most 16 MB per file and 32 LUTs per project.

1D LUTs and unknown keywords are refused, and a file that cannot be parsed names the bad line. A render refuses more than
64 LUT effects playing at the same time (only enabled clips on visible tracks count). In the library a LUT
shows as a **LUT** card, with its own **LUTs** filter while the project has LUTs. Drag it onto a clip to add a LUT
effect with it, or press its **+** to add it to the selected clip. A LUT that an effect uses counts as used. Removing
it asks first, removes those effects in the same undo step, and then deletes the file.

**Effects library.** The library's **Effects** tab lists the effects and the built-in looks by category: **Color**,
**Detail** and **Stylize** (the looks are listed under Color). Drag one onto a clip in the timeline, or double-click it,
or press `Enter` or `Space`, to add it to the selected clip. Effects that the server's FFmpeg lacks are greyed out.

**Paste attributes** has two more groups: **Crop, flip, blend mode and mask** (the crop, the flips, the blend mode and
the mask; mask values that the target animates get a keyframe at the playhead, and a source without a mask removes the
target's mask) and **Effects** (the whole stack with its keyframes, timed relative to the clip start).

**Preview and export.** The WebGL2 preview draws every effect with the same formulas as the export, and golden tests
compare it with FFmpeg's frames. The 2D drawing shows crop and flip but no effects, and its notice says so. If the
graphics driver cannot draw an effect, the preview leaves that effect out and a notice says so; the export is not
affected. Effects the server's FFmpeg lacks are still drawn in the preview and marked **No export** in the inspector.
An export that needs them stops with a message, and **Check Video Studio** lists the missing filters.
- The export applies effects at the source's resolution. Sizes such as the blur radius and the vignette scale with
  the picture, so preview and export match.
- Sharpening has no radius and works on pixels, so at preview size a large source can look sharper in the preview
  than in the export.
- A keyframed LUT intensity and a keyframed vignette make that step of the export about 20 and 12 times slower; a
  fixed intensity below 100 % about 4 times. Use them where they matter.

## Compositing

**Blend modes.** The Picture section's **Blend mode** list decides how a visual clip mixes with the tracks below it.
Titles and stickers have the list too. The modes are **Normal** (the default), **Multiply**, **Screen**, **Overlay**,
**Soft light**, **Hard light**, **Darken**, **Lighten**, **Difference**, **Exclusion**, **Add**, **Subtract**,
**Color dodge** and **Color burn**.
- The clip's opacity, its mask and its transparent areas still count. Where the clip is half transparent, the result
  lies halfway between the picture below and the blended picture.
- Gaps and fully transparent parts change nothing in any mode.
- During a transition between two clips with the same mode, the overlap blends with that mode. When the modes differ,
  the overlap uses Normal, and the inspector says so.
- A mode that the server's FFmpeg cannot export is greyed out in the list. A clip that already uses one keeps it, and
  an export that needs it stops with a message; **Check Video Studio** lists the missing filters.

**Masks.** A mask shows one part of a visual clip and hides the rest. The **Mask** block in the Picture section offers
**None**, **Rectangle** and **Ellipse**. Choosing a shape adds a mask in the middle of the clip; **None** removes the
mask with its keyframes. The mask belongs to the clip's box: moving, resizing or rotating the clip carries the mask
along, while crop, flips and effects change only the picture under it.
- **Mask X** and **Mask Y** place the top left corner of the unrotated mask, as percentages of the clip box's width and
  height. **Mask width** and **Mask height** set its size in the same way. **Mask rotation** turns the mask about its
  centre.
- **Feather** softens the edge: it is the width of the soft band, centred on the outline, in percent of the clip box
  height (0 to 50 %).
- **Invert mask** shows the outside instead of the inside.
- Position, size, rotation and feather can have keyframes, with the same stopwatch and diamonds as the position.

**Edit mask in the preview** shows the mask on the picture while the playhead is inside the clip:
- Drag inside the outline to move the mask, or drag a corner or edge handle to resize it while the opposite side stays
  in place. The round handle above the mask rotates it, and the handle below it softens the edge.
- With the mask focused, the arrow keys move it by 1 % of the box (`Shift`: 10 %). On a focused size handle they move
  that edge; on the rotate handle they turn the mask by 1° (`Shift`: 15°); on the feather handle they change the
  feather by 0.1 % (`Shift`: 1 %).
- `Enter` or `Esc` ends mask editing; so do selecting another clip and moving the playhead out of the clip. Mask
  editing and **Crop in the preview** replace each other.
- Each drag and each key press is one undo step. With a stopwatch on, a drag or key at the playhead sets a keyframe
  there.

**Chroma key.** The **Chroma key** effect makes a coloured background transparent, for example a green screen. It is
in the **Key** category of **Add effect** and of the effects library. A new chroma key goes to the top of the effect
stack, so it sees the camera picture before any colour effect.
- **Key color** is the background colour. Pick it, or press **Green screen** or **Blue screen**.
- **Similarity** sets how close a colour must be to the key colour to disappear; raise it until the background is gone.
- **Smoothness** softens the step between kept and removed colours; 0 makes a hard edge.
- **Spill suppression** removes the green or blue cast that the background leaves on edges and hair. Other key colours
  use whichever of green and blue is stronger in them.
- Similarity and smoothness can have keyframes. The key colour cannot. Spill suppression can have keyframes too.

**Editing.** Splitting, duplicating, copying and pasting a clip keep its blend mode, mask and mask keyframes.
**Reverse** reverses mask keyframes like the other keyframes. **Freeze frame** keeps the blend mode and the mask as they
are at that frame, without keyframes. **Detach audio** gives the sound clip neither.

**Preview and export.** The WebGL2 preview draws blend modes, masks and chroma keys with the export's formulas. Golden
tests compare it with FFmpeg's frames, and each 2d golden case also checks that the change the preview makes follows the
change the export makes. The 2D drawing shows none of them, and its notice says so; mask editing still works there and
shows the outline. If the graphics driver cannot draw a blend mode or a mask, the preview shows the clip in Normal or
without its mask, and a notice says so; the export is not affected.
- A mask with keyframes is computed for every frame at a quarter of the clip box's resolution and then scaled up. A
  very hard edge (feather 0) can therefore sit up to about 1.3 pixels away from the preview's edge.
- Export cost at 1080p, measured with FFmpeg 8.1 and 5.1:
  - a mask with keyframes adds about 15 ms per frame on FFmpeg 8.1 and about 37 ms on 5.1;
  - a mask without keyframes on a clip that does not move is computed once per export segment and costs about 0.2 to
    0.4 seconds (measured: 3.2 ms per frame on FFmpeg 8.1 and 6.9 ms on 5.1 over a 50-frame clip);
  - each blend mode other than Normal adds about 3 to 8 ms per frame and track; a layer that mixes three modes with
    Normal adds about 16 to 31 ms per frame;
  - a chroma key adds about 4 to 6 ms per frame.

  Keyframed masks cost the most, so use them where they matter. Many keyframed masks playing at the same time make an
  export noticeably slower. If the server's FFmpeg lacks a filter that blend modes or masks need, the export stops with
  a message, and **Check Video Studio** lists the filter.

**Videos with their own alpha channel.** A video with an alpha channel, such as WebM VP9 with alpha, is treated as
opaque in the export, because FFmpeg's built-in VP9 decoder drops the alpha. The browser preview may still show the
transparency. This also applies when such a clip is keyed.

## Preview engine, proxies and range check

**Preview engine.** The timeline preview draws with WebGL2 when the browser offers it. It shows what the export
renders for position, size, rotation, opacity, contain / cover and the four transitions; fade through black darkens
the tracks below as well, like the export. When WebGL2 is missing, fails to start, or the browser loses the graphics
context three times within a minute, the preview switches to a simpler 2D drawing and shows a notice in the preview
area; editing and exporting are not affected. A single lost context is restored without a notice. The source monitor
always uses the 2D drawing because it shows the media unchanged.

Troubleshooting a graphics driver: the hidden browser setting `aurago.videoStudio.previewEngine` forces the 2D
drawing. In the browser's developer console run `localStorage.setItem('aurago.videoStudio.previewEngine', '2d')` and
reopen Video Studio; `localStorage.removeItem('aurago.videoStudio.previewEngine')` turns it off again. Only the value
`2d` has an effect. The setting applies to this browser only and has no control in the editor.

Known differences between preview and export:
- Hold last frame: while a video seeks or buffers, the WebGL2 preview keeps the clip's last frame. The 2D drawing skips
  a clip that has no decoded frame yet, as before.
- Sources without colour tags: browsers and FFmpeg may convert untagged video with different colour matrices (FFmpeg
  assumes BT.601 for untagged input). Such sources can look slightly different in the preview than in the export;
  BT.709-tagged sources, which cameras and phones write for HD video, match.
- Fade through black uses the curve measured from FFmpeg's `fadeblack` output. Go, the preview and the golden tests
  share the same numbers, so preview and export agree within two brightness steps of 255 on the FFmpeg versions tested
  (5.1 and 8.1).

**Proxies.** After a video is imported, the server makes a preview proxy: 960 × 540 at the source frame rate (at most
60 fps; an unknown rate uses 30 fps). Proxies are made one at a time per project so exports are not held up. The
library shows each video's proxy state. The preview quality decides what plays: **Full** plays the originals, and
**Auto**, **Half** and **Quarter** play the proxy once it is ready. If a proxy fails, the original plays. The source
monitor always plays originals. Media imported before this version gets a proxy from its item menu with **Create
preview proxy**. Proxies count toward the project's storage.

**Render range exactly.** The transport button next to full screen renders a short part of the timeline through the
export pipeline, so you can check what the export will show: the in/out range when both points are set, otherwise
5 seconds around the playhead. It renders at 540 lines (the width follows the project's aspect ratio) with the export
encoder, falling back to the CPU like an export. A range longer than 60 seconds is refused with a notice; nothing is
shortened. The project is saved first. While the range renders, the preview shows the progress; then the result plays
over the preview with its own controls. **Rendered** and **Live preview** switch between the rendered video and the
live preview at the same timeline frame. `Escape` or the close button ends the comparison and cancels a render that is
still running; closing the window lets it finish like other jobs. A failure shows in the comparison with **Render
again**. Each project keeps one temporary result in its preview folder: it counts toward the project's storage, is
replaced by the next range check, and is removed with the project or when old jobs are cleaned up. It is never listed
among exports.

**Project version 3.** Projects are now saved as version 3. Older projects and older saved versions are upgraded
step by step to version 3 when they are opened, restored or saved. The step from version 2 to 3 changes no content
(version 1 projects first get the version 2 duration backfill as before). The upgrade is one-way: older AuraGo builds
cannot open version 3 projects, so update every AuraGo installation that opens the same projects.

## Exports and background jobs

Exports are rendered in segments of about ten seconds. Every clip is read from its own position in the
source, so late clips in long recordings start quickly. Audio is mixed for the whole export and encoded once.
The server needs free space for the segments, the mixed audio (about 1.4 GB per hour) and the final file; an
export stops early with a clear message when the space is missing. A failed export names the part of the
timeline where it failed. `max_export_size_mb` limits one export file (at most the project limit); a bigger
export stops with a message that names the export size limit or the project storage limit.

Export produces H.264 / AAC MP4 in the Desktop project files, with a download
link. Preview proxies, import probing, rendering, range checks, project duplication, and AI generation are background
jobs; proxies and range checks show where they are used (the library and the preview), not in the job list. Closing an
app window does not cancel a job. Reopening the project reattaches to its status, and the job list offers explicit
cancellation.

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
NVENC, Quick Sync and AMD AMF on Windows) and uses the first encoder that works for exports, range checks and preview
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
automatic subtitles, or an AI editing assistant.
Provider credentials and runtime state are never stored in project JSON. No agent
tool is added by enabling the editor.

Local verification includes model validation, real FFmpeg video/audio fixtures,
file and job permission tests, conflict handling, and browser flows. A local
fixture pass does not establish rendering speed on a particular home-lab server
or acceptance of a paid generation by an external provider.
