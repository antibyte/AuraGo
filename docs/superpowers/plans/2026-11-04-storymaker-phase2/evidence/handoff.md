**No measurements were run.** This session has only Read, Grep and Glob. There is no Bash or Write tool, so I could not run ffprobe, Go, a worktree, or save `scratchpad/p2/spikes/handoff/report.md`. Everything below is static reading of the working tree in `C:/Users/Andi/Documents/repo/AuraGo/` on 2026-11-04. I did not compare it with HEAD 79d40e50b. The frame arithmetic is my hand calculation.

**1. JSON bodies and PUT rules**
- POST projects takes `{name, width, height, fps, ntsc}`, defaults 1920x1080 at 30 fps, and returns 201 with `{id, project, desktop_path, etag, updated_at}` (`internal/server/video_studio_handlers.go:367-421`).
- POST media takes JSON `{source_path, folder}` and requires `Idempotency-Key` (648-652, 666-678). It returns 202 with `{asset, media_url, job}` (825).
- GET job returns the job object (`video_studio_service.go:66-78`, handler 1169-1183). I did not confirm what a probe job puts in `result`.
- PUT needs `If-Match` (469-472). The body is the whole project, decoded strictly with an 8 MiB limit (1385-1397, 29). Send version 3, name, width, height, fps, ntsc, assets, tracks, and markers and folders if present (`internal/videostudio/model.go:37-50`).
- Server-owned asset fields are kind, path, duration_frames, duration_us, video_duration_us, width, height, has_audio, codec, source_fps and size_bytes. `canonicalizeProject` rewrites them from the media record (`internal/server/video_studio_worker.go:969-989`). The client keeps `id`, a non-blank `name` and `folder`. An unknown asset id returns 400 `project_asset_invalid` (502-506).
- The editor makes ids `track-<uuid>` and `clip-<uuid>` (`ui/js/desktop/apps/video-studio-timeline.js:26-28`). The server needs 1 to 128 characters, no outer whitespace, no NUL, and unique track and clip ids (`internal/videostudio/validate.go:89-95, 227-230`). Asset ids are server UUIDs (`internal/uid/uid.go:14-23`).
- Traps: a video clip needs width and height above 0 (280-282) and `fit` set to `contain` or `cover` (291-293). Audio-track clips skip those checks (279).
- Your examples: (a) a video track with one clip and `audio_detached: true` is accepted when the MP4 asset has `has_audio` (276-277). (b) Three audio tracks with one WAV each is accepted (202, 272; at most 8 audio tracks, `model.go:223`).
- Frames: start, offset and duration are integers at the project fps (validate.go:235). Asset frames are `ceil(duration_us × fps / 1e6 − 0.001)` (`internal/videostudio/time.go:55-67`). `duration_us` is `round(ffprobe duration × 1e6)` (`probe.go:151`). The probe first uses 30 fps (152), and canonicalisation re-expresses at the project rate (`model.go:284-289`).
- Refusal: at speed 1 the rule is `duration − 1 ≤ frames − 1 − offset` (validate.go:250, `ramp.go:233-235`). A duration one frame past the asset is refused. Example by hand: a 2.5 s WAV at 25 fps gives ceil(62.499) = 63 frames. 63 is accepted and 64 is refused.

**2. source_path import**
- `OpenPreviewFile` (`internal/desktop/service_files.go:373-418`) accepts workspace paths, so `Documents/Storymaker/<id>/exports/` works. The demuxer whitelist includes `wav` and `mov` (`internal/videostudio/probe.go:16-17`).
- The file is copied into `projects/<id>/media/` (`video_studio_handlers.go:781`), not referenced.
- The asset limit is `max_asset_size_mb`, default 4096 MB and capped at the project's 16384 MB (`internal/config/video_studio.go:24-28`, handler 653, 743-746). The probe has a 45 s timeout, an 8 MiB output cap and a 366-day duration cap (`probe.go:76, 84, 148`).

**3. Audio dependence**
Yes. Detaching audio needs `has_audio` (validate.go:276-277). An audio-track clip from a video needs an audio stream (272). `probe.go:113-115` sets `HasAudio` for any audio stream, so the silent AAC track counts. A detached clip is silent on its video track, and its audio clip plays instead (`internal/videostudio/plan.go:247-255`).

**4. Opening a project**
- Fresh window: `openApp` copies the context (`ui/js/desktop/core/window-shell-runtime.js:1713-1721`), and `menus-and-routing.js:1647-1654` passes it to `appRender`. `ctx.projectId` arrives unchanged.
- Existing window: `matchesExistingAppWindow` ignores context (1424-1432), so the window is always reused. The reuse branch (1615-1619) handles only `importPath`. It needs a `projectId` branch that calls a new `VideoStudioApp` method.
- `initialize` opens `projects[0]` (`ui/js/desktop/apps/video-studio.js:375`). It should prefer `ctx.projectId` when that id is in the list.
- The dirty guard exists. `switchProject` saves a dirty project first (`video-studio.js:456-463`). The reuse path must use it instead of `loadProject`. The close guard is at 304-312.

**5. Library copy**
- `ListFiles` shows subfolders as directories (`service_files.go:17-39`).
- `Videos` is `DataDir/generated_videos` on every OS, not the Windows Videos folder (`service.go:661-663`). It shows at root once the desktop is initialised (`service_files.go:71-86`, `service.go:227-231`). Visibility with desktop disabled is unverified.
- `video_generation.go:202-220` calls `RegisterMedia` (`internal/tools/media_registry.go:267`). The desktop layer updates the registry only on move and delete (`service_media.go:198-238`). Recommendation: skip registration in phase 2. It would only make the copy findable by agent media tools.
- Writes to mounts are refused by both `WriteFileStreamConditional` (`service_file_stream.go:29-33`) and `writeFileBytes` (`service_files.go:454-461`).
- A narrow method would follow the stream path (`service_file_stream.go:19-113`): `resolveMediaMount`, a ReadOnly check, `os.OpenRoot(mount.Dir)`, a staged temp file with a size budget, then a create-only check under `desktopMutationMu` and `RenameRootContext`. Add a free-space check before staging. The stream path has none.

**6. Export record and stems**
- `renderRun.finish` (`internal/storymaker/render/render.go:470-524`) publishes the MP4 at 499-506 and the record at 512-522. The record uses `WriteBytes`, which replaces rather than creating only.
- `Publisher` is at `render.go:56-61`, implemented by `storymaker_store.go:146-158`. `WriteStream` is create-only. `WriteBytes` is capped at 50 MB.
- The render cap is 2 GiB (`render.go:50`). The spec's 8 GiB is Video Studio's `max_export_size_mb` default (`config/video_studio.go:30-31`). Stems at 48 kHz stereo s16 are 192 000 B/s, so 15 minutes is 172.8 MB each, about 518 MB for three.
- Recommendation: after the MP4, write stems with `WriteStream` and SRT/VTT with `WriteBytes` in `finish()`. Decide whether a failed stem leaves or removes the MP4.

**Could not verify**
- Anything that needs running: ffprobe output, Go behaviour, Windows paths.
- The GET project body and ETag (handlers 442-463), and the probe job's `result`.
- Videos visibility with desktop disabled or on other OSes.
- Whether `minFreeStagingByte` is used at `render.go:350`. This is from the code map only.
- Working tree versus HEAD.