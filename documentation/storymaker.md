# Storymaker

The built-in illustration library contains twelve fixed characters, twelve day/night environments and eighty props. PixiJS renders textured bitmap artwork with separate background depth layers; GSAP evaluates animation at the requested video frame. Existing projects automatically use the new artwork with their existing asset IDs, timing and movement names. Images and both runtime libraries ship locally; preview and export need no image-generation provider or CDN.

The renderer prefers WebGL and falls back to Pixi's Canvas renderer. Characters combine a thirteen-bone rig with facial, hand and seated replacement drawings. Source images, prompts, production steps and validation limits are documented in `assets/storymaker/ASSET-CONTRACT.md`.

Storymaker is the Virtual Desktop app for short explainer videos. You stage scenes with illustrated people, props and
environments from a built-in catalog, write the narration, pick a voice and music, watch a live preview and export an MP4 with
sound, captions, subtitle files and separate audio tracks. A finished export can continue in Video Studio.

For CPU-only export, prefer a current installed Chrome. The provisioned older Chromium also works, but renders these textured images more slowly; allow a larger export timeout for long films. Physical GPU speed depends on the machine. The `raw` transport is an existing profiling alternative when PNG encoding limits throughput; compare it on the target machine before changing the default.

## Switch it on

Storymaker is on by default. **Configuration → Virtual Desktop → Storymaker** holds its settings:

| Setting | Default | Meaning |
| --- | --- | --- |
| `storymaker.enabled` | `true` | Shows the app and answers its API. Off: the app is hidden and the API answers `storymaker_disabled`. |
| `storymaker.render.timeout_seconds` | `3600` | An export that runs longer is stopped (60 to 14400). Longer videos get more time automatically. |
| `storymaker.render.max_duration_seconds` | `900` | The longest video that can be exported (10 to 3600). |
| `storymaker.render.browser_path` | empty | Absolute path of Chrome or Chromium. Empty: the browser installed on the system, else a pinned download. |
| `storymaker.render.transport` | empty | How frames reach FFmpeg (`png`, `webp`, `raw`, `screenshot`). Empty is the measured default; change it only on advice. |
| `storymaker.narration.max_daily_chars` | `100000` | Characters that cloud voices may speak per local day (1000 to 100000000). |
| `storymaker.narration.max_job_chars` | `20000` | Characters that cloud voices may speak in one narration job (100 to 10000000). |
| `storymaker.agent.enabled` | `true` | Allows agent jobs from the app and from chat. |
| `storymaker.agent.provider` | empty | LLM provider for agent jobs. Empty: the main route with its configured failover. A chosen provider is pinned to the job without failover. |
| `storymaker.agent.model` | empty | Model for agent jobs. Empty: the model of the provider. |
| `storymaker.agent.job_timeout_seconds` | `1800` | A job that runs longer stops, and its valid scenes stay (120 to 7200). |
| `storymaker.agent.max_length_seconds` | `180` | A longer request is shortened to this length (15 to 3600). `max_duration_seconds` also applies. |
| `storymaker.agent.max_daily_jobs` | `10` | New films and revisions from the app and chat, plus exports from chat, per day (1 to 1000). |

The **Check export setup** button asks the running server whether FFmpeg and a browser are ready. The app is read-only while
`virtual_desktop.readonly` is on: you can watch the preview, but nothing is edited, saved or exported.

## The agent

The agent writes a film from a topic: first the outline, then the cast, then one scene at a time, then a check with fixes, and at the end the narration. It runs as an isolated job on one project. It can use only the Storymaker tools of its current step.

The author receives animation instructions in both normal and compact modes and can read the bundled animation/code references through its catalog tool. These cover staged movement, expressions, cues, overlapping actions, duration/easing, camera, highlights, sound effects and rhythm. Rig motion, blinking, breathing, storyboard lip-sync and background parallax are automatic.

For custom diagrams and effects it can read and write the existing restricted code-scene API, with an executable particle example. Code writes require a current revision, stay within 8 KiB and preserve saved source/bundles on syntax errors; larger user code stays for manual editing. Code is bundled before a scene is activated. This grants no direct PixiJS/GSAP, DOM or network access. Speaking actors use storyboard scenes; code scenes have no dialogue/lip-sync binding. Pixel layout still needs review, and real model output may need correction.

- **In the app.** The project list has a start form. Inside a project, open the **AI author** side tab of the editor. It holds the prompt, length, language, **Write the story** or **Revise the story**, a progress list, **Stop the author** (cancel), the outcome card, **Export without voice** and **Restore speech**.
- **From chat.** The `storymaker` tool can create a film, report its status, revise it, export it, open it in the app and list projects. "Mach mir ein Erklärvideo über Backups" is enough to start.
- **Unattended runs.** With `agent.action_policy` switched on, missions, flows, scheduled runs and co-agents need an administrator rule to call `create`, `revise` or `render`. Chats you take part in need no rule. The policy is off by default. A run that is not watched is then limited only by `storymaker.agent.max_daily_jobs` and the character limits of the voices.
- **While a job runs.** The editor is locked and shows a banner. Saving, narration and export are refused for that project. A job cannot start while a render, narration or music job of the project runs. One job runs at a time, and eight can wait.
- **Language.** The request can name a language. Otherwise the job uses the language of the interface. Narration, title and on-screen text are written in that language.
- **Writing check.** Narration and on-screen text are checked for dashes everywhere, and for stock phrases in English and German. A hit gets one optional fix round and never blocks the outcome or appears among the open checks.
- **Revise.** The outcome card lists the scenes that changed, the scenes that stayed as they were and the scenes that were removed. The chat `status` answer gives the same three lists.

| Outcome | What it means |
| --- | --- |
| Your story is ready. Please watch it once. | Scenes, assets and timing were checked, and the layout too when the browser ran the compile check. |
| Your story is partly done. | Some scenes still need work, and valid scenes are saved. The revise field is prefilled to finish them. A timeout or a model error ends here too. |
| The agent could not write this story. | No outline was saved. The card names the kind of failure in one line. |

Watch every film once: when the compile check runs, the layout run flags captions over an actor's feet, props that are too small, boxes outside the frame and an actor that hides a prop or a text. A film reads `ready` only when no check is left open. Scenes that look alike (three or more in a row) are flagged as `scenes_look_alike`. The job card links the contact sheet of the last check. A film may run a little longer than asked, which is listed and is not a failure. Without a browser the compile check is skipped and reported as `compile_unverified`. The job still finishes.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the story, and each scene gets a short on-screen text. To export, either install a voice and press **Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives each a reading time and turns speech bubbles into on-screen text. **Restore speech** undoes it while its copy, `story.before-silent.json`, exists. From chat, `render` with `silent: true` does the same and says that speech was removed.

**Narration and cost.** A job narrates by itself at its end, under the character limits above, so a finished job can export. Cloud voices can cost money. **Music** is not part of the agent: a request for music gets an answer that it cannot do this, and the Music tab stays with you.

An agent job has no token limit of its own. Its cost is bounded by the job timeout (1800 seconds by default), the fixed tool-call limits per step and `storymaker.agent.max_daily_jobs`.

Job records are in `data/storymaker/agent/`: `jobs.json`, `usage.json` and `events/<job id>.jsonl`. The event files hold tool names, scene ids, token counts and statuses, but no tool arguments and no model text. `jobs.json` also keeps the request, its first 200 characters, and for a ready film the summary.

## Projects

Every project is a folder in the desktop workspace, `Documents/Storymaker/<project id>/`:

```
story.json        the story document (the editor saves it with If-Match, so two windows cannot overwrite each other)
project.json      title, format and timestamps
scenes/<id>.js    code scenes (and <id>.bundle.js, the bundled copy the player runs)
audio/narration/  one audio file per sentence (<hash>.wav or .mp3) with its level curve (<hash>.env.json)
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
- **Music (right).** The source of the background music: none, generated by AI (only when music generation is set up), or a WAV or MP3 file or a Noisemaker track. Generated music starts only when you click the button, because a generation may cost money. A file or a track is copied into the project by itself. A moved file is named here, with an offer to choose another one.
- **Code tab.** `story.json` and every code scene in an editor. Problems are marked at their line and listed below; saving checks
  the story again. A code scene is a small program, `export default function scene(api) { ... }`, that draws a scene itself; it
  cannot reach the network or the page.
- **Issues.** The badge in the toolbar counts errors and warnings. An error blocks the export; the list says where.

## Voice and narration

- **Voice panel.** Pick a provider, a voice and a language, then **Use this voice** writes them into the story. **Generate narration** and **Play a sample** work with the saved voice, so a voice you picked needs this click first. Phonon voices that are not installed are not listed: download them in the Phonon settings under More voices. `not tested` means nobody has listened to that provider in that language yet. A new project asks for a voice in the desktop language; when no voice speaks it, the app lists the languages that have one and offers to create the project without a voice.
- **Generate narration.** Audio is made one sentence at a time and only for sentences without audio, so an edit costs one sentence. The panel shows the characters, what has no audio and the progress. **Play a sample** plays a stored sentence and never calls a provider.
- **Limits.** Cloud voices count characters per job and per local day (settings above). Local voices (phonon, piper, sanotts, supertonic, speech_lab) are not counted.
- **Preview.** The narration plays with the preview, and the timeline shows its waveform. Speech bubbles, burned-in captions and lip-sync are drawn in the preview and in the export. A switch in the voice panel turns the captions off. Music and sound effects play along at about their export level. The preview has no presence carve, voice match or impact dips, so its balance is close, but the export is final.
- **Languages.** Greek, Hindi, Arabic, Chinese and Japanese text uses bundled fallback fonts that load only when needed. A character no font covers, such as an emoji or Thai and Hebrew text, is the error `glyph_missing` at that text.

Projects from the first version change in two ways. Captions default to on, so every narrated older story starts drawing them. A `say` without text whose `who` is not the scene's speaker is now an error that blocks the export.

## Export

The **Export** button opens a panel that first says what is missing. Exporting needs:

1. FFmpeg (found on `PATH` or at `video_studio.ffmpeg_path`),
2. a browser that can run headless (Chrome or Chromium). Without one, the first export downloads a pinned Chromium (about 200 MB,
   checked against a SHA-256 before it is used) into the data folder; the export panel says so before you start. In the Docker
   image set `storymaker.render.browser_path` to a browser inside the container: the image has no Chromium libraries of its own,
3. a story without errors that is not longer than `max_duration_seconds`.

Editing and the preview work without 1 and 2. The export draws every frame in a headless browser, so a 15 minute video takes a
while; the panel shows the phase and the frame count and can cancel. The result is
`Documents/Storymaker/<project id>/exports/<title>-<job>.mp4`; **Show in Files** opens that folder. Videos carry the narration, the music and the sound effects.

## Motion

The player times a story by motion rules. They also change how existing projects play: a text stays on screen long enough to be
read, a hide takes 0.35 s, and a beat on another actor starts while the previous beat is still running.

- **Rhythm.** `style.rhythm` is `calm`, `standard` or `lively`. Edit it in the Code tab. It scales default beat durations and
  lengthens text holds. It never changes an explicit `duration`, `delay` or `wait`.
- **Cues.** An arrival with a cue lands on its narration marker, so it starts a little earlier. If its actor is still busy, it lands
  late and the issues list says so.
- **Text.** A text that does not fit shrinks in steps, at most to 45 % of its size and never below 2.6 % of the short side of the
  frame. Its lines are then balanced.
- **Highlight.** The `highlight` beat draws attention to an actor, a prop or a text. Its style is `pulse`, `outline` or `arrow`.
- **Default transition.** A new scene gets a wipe when its environment differs from the previous scene's, and a fade otherwise.
  Scenes you already saved keep their transition, with one exception: picking another environment in the editor turns a fade into
  a wipe, on that scene and on the next. Any other transition stays, and you can pick a fade again.

The issues list also shows motion hints. A hint never blocks an export.

| Code | Meaning |
| --- | --- |
| `line_unread` | A text leaves before it can be read. |
| `dead_stretch` | Nothing moves and nothing is spoken for more than 3 s. |
| `first_frame_empty` | The first frame after the transition shows no actor, prop or text. |
| `text_too_small` | A text style is below 2.6 % of the short side of the frame. |
| `fade_between_worlds` | A scene fades into another environment. Use a cut or a wipe. |
| `cue_late` | An arrival lands after its marker, because its actor is still busy. |
| `entry_monotony` | Three or more show beats, all with the same entry. |
| `transition_variety` | More than five transition kinds in one film. |

## Sound

The export mixes narration, music and sound effects in AuraGo itself and writes AAC 192k, about -16 LUFS at -1 dBTP at most. A film with only sound effects ends quieter, near -22 LUFS, because its peaks limit the gain. The music ducks under speech. An `sfx` beat plays a sound from the catalog (`sfx.whoosh`, `sfx.pop`, `sfx.click`, `sfx.ding`, `sfx.success`, `sfx.error`, `sfx.typing`) on its landing, and `style.auto_sfx` adds a pop to each show with entry `pop`.

An export is refused while narration audio or the music file is missing. The export panel says what is missing and opens the panel that fixes it. A scene marked as silent on the Scene tab exports without its narration and keeps the text, while a `say` with its own text still needs audio. A film without narration, music and sound effects exports with a silent track, and the panel says so before you start.

`POST /api/desktop/storymaker/projects/{id}/render` takes an optional body `{burn_in_captions, srt, vtt, stems, quality}`. Stems and subtitles land in `exports/<title>-<job>/`, and a copy of the MP4 lands in `Videos/Storymaker/`. A failed extra leaves the MP4 and adds a warning to the job. The export dialog below sets these options.

## Export options and extras

Starting an export opens a dialog with these choices:

| Choice | Meaning |
| --- | --- |
| Quality | `standard`, or `high` for a better picture and a larger file. |
| Burned-in captions | Draws the captions into the picture. The preset follows the captions switch of the story. |
| SRT, VTT | Writes subtitle files next to the video. |
| Stems | Writes one WAV per audio layer: narration, music, sound effects. A silent layer is left out. |
| Open in Video Studio | Turns stems on, so the finished export can continue in Video Studio. |

An export does not start while narration or music is missing. The panel says what is missing and leads to the voice or the
music panel.

The files of one export:

    Documents/Storymaker/<project id>/exports/
      <slug>-<job8>.mp4
      <slug>-<job8>/narration.wav  music.wav  sfx.wav  captions.srt  captions.vtt
      <job id>.json
    Videos/Storymaker/<slug>.mp4, <slug>-2.mp4, ...

The copy in `Videos/Storymaker/` makes the film easy to find. A failed extra leaves the MP4 in place and shows a warning on the
finished export. The stems add up to the mix exactly before encoding. The AAC track in the MP4 differs slightly, about 23 dB below
the mix.

## Continue in Video Studio

A finished export with stems offers **Open in Video Studio** when Video Studio is on, FFmpeg is ready and the desktop is
writable. The action creates a Video Studio project, copies the MP4 and each stem into it, and opens it. The film sits on a video
track with its own sound detached, and each stem gets its own audio track. This also works after you left the project or closed the
Storymaker window: open the project again and choose **Show the last export** in the export panel, which offers the newest finished
export. If you ticked **Open in Video Studio** in the dialog and only left the project during the render, the handoff starts by itself when you open the project again in the same window. When a handoff fails, trying again fills the same Video Studio project. If that project holds clips by then, a new project is created, so your edits stay. Unsaved work in an open Video Studio project is saved first.

## Export speed

With the `png` transport the export draws frames in several workers: `(CPUs - 2) / 3`, rounded down, between 1 and 6, and fewer
when free memory is short. A machine with 8 logical CPUs uses 2 workers. The frames are identical to a serial export.

## Not yet in Storymaker

The asset generator, the agent's layout checks and still review, charts and diagrams, most other transitions (cut, fade, slide-left and wipe
exist) and drag and drop staging. The story format already accepts the fields of later phases; the editor shows them as "not
supported yet" and the player ignores them. Nobody has listened to the voices yet, so every provider and language shows a
`not tested` mark.

## For developers

`ui/js/desktop/apps/AGENTS.md` (Storymaker), `internal/storymaker/AGENTS.md`, `internal/storymaker/render/AGENTS.md` and
`ui/js/storymaker/AGENTS.md` hold the contracts. `docs/superpowers/specs/2026-11-01-storymaker-contract.md` is the reference.

- Player, in Node: `npm run test:storymaker`.
- The app tour against a stand-in server: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`.
- The same app against the real handlers, ending in a rendered MP4 with sound and the Video Studio handoff:
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`.
- App player against render page: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerParityAppPlayerAndRenderPage$' -count=1 -timeout 15m`.
- Parallel workers: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m`.
- The agent with a scripted model, in native and text mode: `go test ./internal/server -run '^TestStorymakerAgentScripted' -count=1`.

FFmpeg and Chrome must be installed for the browser lines.
