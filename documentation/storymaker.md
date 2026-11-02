# Storymaker

Storymaker is the Virtual Desktop app for short explainer videos. You stage scenes with cartoon people, props and
environments from a built-in catalog, write the narration text, watch a live preview and export an MP4. This page describes
the first version (phase 1): the editor, the preview and a silent export. Voices, music, captions and the agent come in later
phases.

## Switch it on

Storymaker is on by default. **Configuration → Virtual Desktop → Storymaker** holds its settings:

| Setting | Default | Meaning |
| --- | --- | --- |
| `storymaker.enabled` | `true` | Shows the app and answers its API. Off: the app is hidden and the API answers `storymaker_disabled`. |
| `storymaker.render.timeout_seconds` | `3600` | An export that runs longer is stopped (60 to 14400). Longer videos get more time automatically. |
| `storymaker.render.max_duration_seconds` | `900` | The longest video that can be exported (10 to 3600). |
| `storymaker.render.browser_path` | empty | Absolute path of Chrome or Chromium. Empty: the browser installed on the system, else a pinned download. |
| `storymaker.render.transport` | empty | How frames reach FFmpeg (`png`, `webp`, `raw`, `screenshot`). Empty is the measured default; change it only on advice. |

The **Check export setup** button asks the running server whether FFmpeg and a browser are ready. The app is read-only while
`virtual_desktop.readonly` is on: you can watch the preview, but nothing is edited, saved or exported.

## Projects

Every project is a folder in the desktop workspace, `Documents/Storymaker/<project id>/`:

```
story.json        the story document (the editor saves it with If-Match, so two windows cannot overwrite each other)
project.json      title, format and timestamps
scenes/<id>.js    code scenes (and <id>.bundle.js, the bundled copy the player runs)
exports/          the exported videos and their records
```

Delete a project in the list: the whole folder, exports included, goes away.

## The editor

- **Scenes (left).** One card per scene with a thumbnail the player draws, its first words and its duration. Add a scene or a
  code scene; move or delete a scene in the inspector.
- **Stage and timeline (centre).** The preview runs in a sandboxed frame. The timeline has one block per scene (code scenes are
  hatched); click or drag it, or use the arrow keys (Shift moves a second). Play and pause start the real-time preview.
- **Inspector (right).** Environment, time of day, narration, the transition into the scene and its duration (automatic is as
  long as the narration takes to read). The people and props in the scene can be removed there.
- **Library (right).** People, props and places from the catalog, with search. A click adds a person or a prop to the selected
  scene, or sets its environment.
- **Code tab.** `story.json` and every code scene in an editor. Problems are marked at their line and listed below; saving checks
  the story again. A code scene is a small program, `export default function scene(api) { ... }`, that draws a scene itself; it
  cannot reach the network or the page.
- **Issues.** The badge in the toolbar counts errors and warnings. An error blocks the export; the list says where.

## Export

The **Export** button opens a panel that first says what is missing. Exporting needs:

1. FFmpeg (found on `PATH` or at `video_studio.ffmpeg_path`),
2. a browser that can run headless (Chrome or Chromium). In the Docker image set `storymaker.render.browser_path` to a
   browser inside the container: the image has no Chromium libraries of its own,
3. a story without errors that is not longer than `max_duration_seconds`.

Editing and the preview work without 1 and 2. The export draws every frame in a headless browser, so a 15 minute video takes a
while; the panel shows the phase and the frame count and can cancel. The result is
`Documents/Storymaker/<project id>/exports/<title>-<job>.mp4`; **Show in Files** opens that folder. Phase 1 videos have no sound.

## Not in phase 1

Voices and narration audio, music and sound effects, captions, the agent panel and chat tool, the asset generator, charts and
diagrams, most transitions (cut, fade, slide-left and wipe exist) and drag and drop staging. The story format already accepts
the fields of later phases; the editor shows them as "not supported yet" and the player ignores them.

## For developers

`ui/js/desktop/apps/AGENTS.md` (Storymaker), `internal/storymaker/AGENTS.md` and `ui/js/storymaker/AGENTS.md` hold the
contracts. The app tour test is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`.
