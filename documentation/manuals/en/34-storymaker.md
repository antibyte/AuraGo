# Chapter 34: Storymaker – explainer videos in the Virtual Desktop

Storymaker is the Virtual Desktop app for short explainer videos. You stage scenes with illustrated
people, props and environments from a built-in catalog, write the narration, pick a voice and music,
watch a live preview and export an MP4 with sound, captions, subtitle files and separate audio tracks.
A finished export can continue in Video Studio.

Storymaker is on by default. The AI author writes the whole film from a topic, in the app or from
chat. Editing and the preview need nothing but the browser; the export needs FFmpeg and a headless
browser. The full contract is in the [Storymaker guide](../../storymaker.md).

## Table of Contents
1. [Opening Storymaker](#opening-storymaker)
2. [Settings](#settings)
3. [The AI Author](#the-ai-author)
4. [Voice and Narration](#voice-and-narration)
5. [Exporting](#exporting)
6. [Continue in Video Studio](#continue-in-video-studio)
7. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening Storymaker

Open **Storymaker** from the start menu, in the category **Creative**. The window opens maximized.

The project list shows every story on the server with a **New project** form above it. A new project
starts with one empty scene. The form also lists the languages that have a voice; when the desktop
language has none, it offers **Create without a voice**.

Inside a project the editor has three columns:

- **Scenes (left).** One card per scene with a thumbnail the player draws, its first words and its
  duration. Add a scene or a code scene; move or delete a scene in the inspector.
- **Stage and timeline (centre).** The preview runs in a sandboxed frame. The timeline has one block
  per scene (code scenes are hatched); click or drag it, or use the arrow keys (Shift moves a second).
- **Inspector, Library, Music, Voice (right).** The inspector holds the environment, the time of day,
  the narration, the transition and the duration (automatic means as long as the narration takes to
  read). The library holds people, props and places from the catalog, with search. The music tab holds
  the background music, the voice tab the voice and the narration.

The toolbar carries the **Storyboard** and **Code** tabs and an **Issues** badge. The badge counts
errors and warnings; an error blocks the export and the list says where. The **Code** tab shows
`story.json` and every code scene in an editor. A code scene is a small program,
`export default function scene(api) { ... }`, that draws a scene itself; it cannot reach the network
or the page. With `virtual_desktop.readonly` on, you can watch but not edit, save or export.

## Settings

**Configuration → Virtual Desktop → Storymaker** holds the settings:

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

The **Check export setup** button asks the running server whether FFmpeg and a browser are ready. It
never starts a download on its own.

## The AI Author

The AI author writes a film from a topic: first the outline, then the cast, then one scene at a
time, then a check with fixes, and at the end the narration. It runs as an isolated job on one
project and can use only the Storymaker tools of its current step.

- **In the app.** The project list has a start form. Inside a project, open the **AI author** side tab
  of the editor. It holds the prompt, the length in seconds, the language, **Write the story** or
  **Revise the story**, a progress list, **Stop the author** (cancel), the outcome card,
  **Export without voice** and **Restore speech**.
- **From chat.** The `storymaker` tool creates a film, reports its status, revises it, exports it,
  opens it in the app and lists projects. "Mach mir ein Erklärvideo über Backups" is enough to start.
  The tool needs `storymaker.enabled`, `storymaker.agent.enabled` and the Virtual Desktop (see
  [Chapter 22](./22-internal-tools.md)).
- **Unattended runs.** With `agent.action_policy` switched on, missions, flows, scheduled runs and
  co-agents need an administrator rule to call `create`, `revise` or `render`. Chats you take part in
  need no rule. The policy is off by default.

**While a job runs.** The editor is locked and shows a banner. Saving, narration and export are
refused for that project. A job cannot start while a render, narration or music job of the project
runs. One job runs at a time, and eight can wait; a ninth is refused.

The request can name a language. Otherwise the job uses the language of the interface. Narration,
title and on-screen text are written in that language. Narration and on-screen text are checked for
dashes everywhere, and for stock phrases in English and German; a hit gets one optional fix round
and never blocks the outcome.

| Outcome | What it means |
| --- | --- |
| Your story is ready. Please watch it once. | Scenes, assets and timing were checked. Overlaps and text fit were not checked automatically. |
| Your story is partly done. | Some scenes still need work, and valid scenes are saved. The revise field is prefilled to finish them. A timeout or a model error ends here too. |
| The agent could not write this story. | No outline was saved. The card names the kind of failure in one line. |

Watch every film once: nothing checks yet whether a prop covers a face or a text overflows. A film may
run a little longer than asked, which is listed and is not a failure. Without a browser the compile
check is skipped and reported as `compile_unverified`; the job still finishes. The outcome card of a
revision lists the scenes that changed, that stayed and that were removed; the chat `status` answer
gives the same three lists.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the
story, and each scene gets a short on-screen text. To export, either install a voice and press
**Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives
each a reading time and turns speech bubbles into on-screen text. **Restore speech** undoes it while
its copy, `story.before-silent.json`, exists. From chat, `render` with `silent: true` does the same.

**Music.** The AI author does not do music. A request for music gets an answer that it cannot do
this, and the Music tab stays with you. The Music tab offers none, AI-generated music (only when
music generation is set up) or a WAV or MP3 file or a Noisemaker track. Generated music starts only
when you click the button, because a generation may cost money.

An agent job has no token limit of its own. Its cost is bounded by the job timeout (1800 seconds by
default), the fixed tool-call limits per step and `storymaker.agent.max_daily_jobs`. Job records are in
`data/storymaker/agent/`: `jobs.json`, `usage.json` and `events/<job id>.jsonl`, holding tool names,
scene ids, token counts and statuses, but no tool arguments and no model text.

## Voice and Narration

The voice panel picks a provider, a voice and a language. **Use this voice** writes them into the
story; **Generate narration** and **Play a sample** work with the saved voice, so a voice you picked
needs this click first. Phonon voices that are not installed are not listed: download them in the
Phonon settings under More voices. `not tested` means nobody has listened to that provider in that
language yet.

Audio is made one sentence at a time and only for sentences without audio, so an edit costs one
sentence. The panel shows the characters, what has no audio and the progress. **Play a sample** plays
a stored sentence and never calls a provider.

Cloud voices count characters per job and per local day (see the settings above). Local voices
(phonon, piper, sanotts, supertonic, speech_lab) are not counted. Cloud voices can cost money: a
finished job narrates by itself at its end, under the character limits above. The narration plays
with the preview, and the timeline shows its waveform. Speech bubbles, burned-in captions and lip-sync
are drawn in the preview and in the export; a switch in the voice panel turns the captions off. Greek,
Hindi, Arabic, Chinese and Japanese text uses bundled fallback fonts that load only when needed. A
character no font covers, such as an emoji or Thai and Hebrew text, is the error `glyph_missing`.

## Exporting

The **Export** button opens a panel that first says what is missing. Exporting needs:

1. FFmpeg (found on `PATH` or at `video_studio.ffmpeg_path`),
2. a browser that can run headless (Chrome or Chromium). Without one, the first export downloads a
   pinned Chromium (about 200 MB, checked against a SHA-256 before it is used) into the data folder;
   the export panel says so before you start. In the Docker image set `storymaker.render.browser_path`
   to a browser inside the container: the image has no Chromium libraries of its own,
3. a story without errors that is not longer than `max_duration_seconds`.

Editing and the preview work without 1 and 2. The export draws every frame in a headless browser, so a
15 minute video takes a while; the panel shows the phase and the frame count and can cancel. The
result is `Documents/Storymaker/<project id>/exports/<title>-<job>.mp4`; **Show in Files** opens that
folder. Videos carry the narration, the music and the sound effects.

Starting an export opens a dialog with these choices:

| Choice | Meaning |
| --- | --- |
| Quality | `standard`, or `high` for a better picture and a larger file. |
| Burned-in captions | Draws the captions into the picture. The preset follows the captions switch of the story. |
| SRT, VTT | Writes subtitle files next to the video. |
| Stems | Writes one WAV per audio layer: narration, music, sound effects. A silent layer is left out. |
| Open in Video Studio | Turns stems on, so the finished export can continue in Video Studio. |

The files of one export:

```
Documents/Storymaker/<project id>/exports/<slug>-<job8>.mp4
Documents/Storymaker/<project id>/exports/<slug>-<job8>/narration.wav  music.wav  sfx.wav  captions.srt  captions.vtt
Documents/Storymaker/<project id>/exports/<job id>.json
Videos/Storymaker/<slug>.mp4, <slug>-2.mp4, ...
```

The copy in `Videos/Storymaker/` makes the film easy to find. A failed extra leaves the MP4 in place
and shows a warning on the finished export. The stems add up to the mix exactly before encoding; the
AAC track in the MP4 differs slightly, about 23 dB below the mix.

## Continue in Video Studio

A finished export with stems offers **Open in Video Studio** when Video Studio is on, FFmpeg is
ready and the desktop is writable. The action creates a Video Studio project, copies the MP4 and each
stem into it, and opens it. The film sits on a video track with its own sound detached, and each
stem gets its own audio track (see [Chapter 26](./26-video-studio.md)).

This also works after you left the project or closed the Storymaker window: open the project again and
choose **Show the last export** in the export panel. If you ticked **Open in Video Studio** and only
left the project during the render, the handoff starts by itself when you open the project again in
the same window. A failed handoff fills the same Video Studio project; if it holds clips by then, a
new one is created, so your edits stay. Unsaved work is saved first.

## When Something Goes Wrong

| Problem | What helps |
|---------|------------|
| The app is not in the start menu | `storymaker.enabled` is off. An administrator can switch it on under **Config → Virtual Desktop → Storymaker**. |
| "Export is not ready yet (FFmpeg or browser missing)" | Install FFmpeg on the server, or set its path in the Video Studio settings (`video_studio.ffmpeg_path`). Without a browser the first export downloads a pinned Chromium; set `storymaker.render.browser_path` to an installed Chrome or Chromium if the download does not work, for example inside the Docker image. |
| The export is refused | The panel names what is missing: errors in the story, a video longer than `max_duration_seconds`, narration audio that is missing, or a missing music file. The panel opens the tab that fixes it. |
| The export is very slow or stopped by the timeout | The export draws every frame in a headless browser. On a CPU-only machine an installed Chrome is faster than the provisioned Chromium. Longer videos get more time automatically, so a higher `storymaker.render.timeout_seconds` is the fix. |
| The AI author does not start | `storymaker.agent.enabled` is off, no AI model is available, or the model is too small. The card names the reason. If the daily limit `storymaker.agent.max_daily_jobs` is used up, try again tomorrow or raise the limit. |
| "Eight agent jobs are already waiting" | The agent queue is full. Wait for one to finish. |
| The narration stops after a while or was never generated | The character limit for the day or the job is reached, or no voice is ready for that language, or the provider failed. Cloud voices are limited, local ones are not. Set up a voice and press **Generate narration**, or press **Export without voice**. |
| The narration is missing a character | The error `glyph_missing` names the text. That character is in no bundled font; rewrite the word or choose another language. |
| The music or the sound is missing in the export | The music file or track is gone, or nothing was prepared; choose a source again in the Music tab. A film without narration, music and sound effects exports with a silent track, and the export panel says so before you start. |
| **Open in Video Studio** is not offered | The export has no stems, Video Studio is off, FFmpeg is missing, or the desktop is read-only. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 22: Internal Tools](./22-internal-tools.md) · [Chapter 26: Video Studio](./26-video-studio.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 33: Aura Drift](./33-aura-drift.md)
**Next Chapter:** [Chapter 35: Please Do Not Feed the Universe](./35-goldfish-adventure.md)
