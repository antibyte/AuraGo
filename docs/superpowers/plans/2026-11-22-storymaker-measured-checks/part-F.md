# Part F: Documentation

> Generated from the drafted and tested commits 962b42edc..34e590195. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task F1: document the measured checks, cue phrases and the audio checks

**Files:**
- Modify: `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
- Modify: `documentation/manuals/de/34-storymaker.md`
- Modify: `documentation/manuals/en/34-storymaker.md`
- Modify: `documentation/storymaker.md`
- Modify: `internal/storymaker/AGENTS.md`
- Modify: `internal/storymaker/render/AGENTS.md`
- Modify: `ui/js/storymaker/AGENTS.md`

- [ ] **Step 1: Make the change**

In `docs/superpowers/specs/2026-11-01-storymaker-contract.md`, replace:

```markdown
| `layout_unverified` | minor | Player compile report: one per code scene (layout inside code is not checked) |
| `glyph_missing` | error | Go Validate: a character no face covers, at the text path |
| `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `cue_late` | warning | Player compile report, storyboard scenes |
| `entry_monotony`, `transition_variety` | minor | Player compile report |

Go constants are `Code<CamelCase>` (for example `CodeInvalidValue`). `DecodeStory` failures always return at least one issue, so a 422 response carries `issues`.
```

with:

```markdown
| `layout_unverified` | minor | Player compile report: one per code scene (layout inside code is not checked) |
| `glyph_missing` | error | Go Validate: a character no face covers, at the text path |
| `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `cue_late` | warning | Player compile report, storyboard scenes |
| `dead_stretch`, `first_frame_empty` | warning | Player compile report `checks` (frame probe), code scenes. Model-fixable advice; never blocks. |
| `text_offframe` | warning | Go `LayoutIssues`, layout text boxes: more than 1 % of a text's box lies outside the frame. Hard: the agent must fix it, and a left one makes the job `incomplete`. A person's export is never refused. |
| `text_unsafe` | warning | Go `LayoutIssues`, texts and the caption box: more than 2 % of the box lies in the aspect's safe zone. Advice only. |
| `text_contrast` | warning | Go, the stills of the compile check: contrast below 3:1 inside a text or caption box. Ratios of 1.15:1 and below are not reported. Advice only. |
| `flash` | warning | Player compile report `checks` (frame probe): more than 3 flashes in any one second. Hard, like `text_offframe`. |
| `flash_unverified` | warning | Player compile report `checks` (frame probe): the probe did not reach every frame within its 20 s deadline. Open check (`ready_unverified`); never blocks. |
| `slow_hook` | warning | Player compile report (film lint of scene 1): scene 1 has no narration, or its first show beat lands after 0.5 s. Advice only. |
| `entry_monotony`, `transition_variety` | minor | Player compile report |

Go constants are `Code<CamelCase>` (for example `CodeInvalidValue`). `DecodeStory` failures always return at least one issue, so a 422 response carries `issues`.
```

In `docs/superpowers/specs/2026-11-01-storymaker-contract.md`, replace:

```markdown
- Agent: `StorymakerPhaseToolSchemas`, `canonicalizeStorymakerOperationCall` and `RunConfig.RecoveryNudge`.
- Files under `<data>/storymaker/agent/`: `jobs.json`, `usage.json` (fails closed) and `events/<job id>.jsonl` (at most 500 lines per job, newest 200 files kept, no arguments and no model text).
- Issue codes for the job only: `scene_unfilled`, `over_target`, `over_budget`, `compile_failed`, `over_export_limit`, `words_tell`. Tool refusals: `operation_not_in_phase`, `outline_locked`, `corrections_exhausted`, `dry_runs_exhausted`, `check_rounds_exhausted`, `rev_mismatch`, `scene_not_found`, `invalid_data`.
- Open checks: `layout_unverified` (always), `compile_unverified`, `narration_unavailable`, `narration_failed`, `narration_budget`, `narration_pending`, `corrections_exhausted`, `model_error`.

---
```

with:

```markdown
- Agent: `StorymakerPhaseToolSchemas`, `canonicalizeStorymakerOperationCall` and `RunConfig.RecoveryNudge`.
- Files under `<data>/storymaker/agent/`: `jobs.json`, `usage.json` (fails closed) and `events/<job id>.jsonl` (at most 500 lines per job, newest 200 files kept, no arguments and no model text).
- Issue codes for the job only: `scene_unfilled`, `over_target`, `over_budget`, `compile_failed`, `over_export_limit`, `words_tell`. Tool refusals: `operation_not_in_phase`, `outline_locked`, `corrections_exhausted`, `dry_runs_exhausted`, `check_rounds_exhausted`, `rev_mismatch`, `scene_not_found`, `invalid_data`.
- Open checks: `layout_unverified` (always), `compile_unverified`, `flash_unverified`, `narration_unavailable`, `narration_failed`, `narration_budget`, `narration_pending`, `corrections_exhausted`, `model_error`.

---
```

In `docs/superpowers/specs/2026-11-01-storymaker-contract.md`, replace:

```markdown

- `end_frame` is exclusive. `frame_count` is the largest `end_frame`. `duration_ms = round(frame_count × 1000 / fps)`.
- Transitions overlap: `start_frame[i] < end_frame[i-1]` is allowed.
- `checks` is empty until phase 3. `unverified` holds one minor `layout_unverified` per code scene. `captions` is always filled, measured or estimated. Compile drops audio events at or past `duration_ms` and cuts captions at it.
- `issues` holds the player's issues (`narration_missing`, `not_supported_yet`, `unknown_cue`, `unknown_spot`, `unknown_cast`, `unknown_asset`, `unknown_name`, `missing_field`, `duplicate_id`, `invalid_value` (warnings, tolerant fallbacks), and `code_error` or `deadline` (errors)). An error issue blocks the render job.
- `audio_events` has one entry per piece and one per sound; Go owns the gain, so there is no `gain` field. For `sfx`, `ref` is the catalog id and `take` is a 0-based index into its `takes`. `start_ms` is always film-global: for `sfx` it is `Math.round((sceneBase + eventTime)*1000)`, where `eventTime` is scene-local, the `start` of scheduling step 7 for a plain sfx and the key's event time for a typing key. `cue_precision` is the share of cues placed by a span, or null without cues. `scenes[].cues[].at_ms` is scene-local.

### Timing rules (summary)
```

with:

```markdown

- `end_frame` is exclusive. `frame_count` is the largest `end_frame`. `duration_ms = round(frame_count × 1000 / fps)`.
- Transitions overlap: `start_frame[i] < end_frame[i-1]` is allowed.
- `checks` lists the frame probe's findings from `checks.js`: `flash`, `flash_unverified`, and the code-scene `dead_stretch` and `first_frame_empty`. The probe runs only when the compile run sets `boot.layout`; otherwise `checks` is empty. It has a 20 s deadline (`PROBE_DEADLINE_MS`) inside the compile deadline. Go keeps only those four codes from `checks`, forces them to severity `warning`, caps the list and ignores any `probe_error` field. `unverified` holds one minor `layout_unverified` per code scene. `captions` is always filled, measured or estimated. Compile drops audio events at or past `duration_ms` and cuts captions at it.
- `issues` holds the player's issues (`narration_missing`, `not_supported_yet`, `unknown_cue`, `unknown_spot`, `unknown_cast`, `unknown_asset`, `unknown_name`, `missing_field`, `duplicate_id`, `invalid_value` (warnings, tolerant fallbacks), `slow_hook` (warning), and `code_error` or `deadline` (errors)). An error issue blocks the render job.
- `audio_events` has one entry per piece and one per sound; Go owns the gain, so there is no `gain` field. For `sfx`, `ref` is the catalog id and `take` is a 0-based index into its `takes`. `start_ms` is always film-global: for `sfx` it is `Math.round((sceneBase + eventTime)*1000)`, where `eventTime` is scene-local, the `start` of scheduling step 7 for a plain sfx and the key's event time for a typing key. `cue_precision` is the share of cues placed by a span, or null without cues. `scenes[].cues[].at_ms` is scene-local.

### Timing rules (summary)
```

In `documentation/manuals/de/34-storymaker.md`, replace:

```markdown
Ergebniskarte einer Überarbeitung listet die Szenen, die sich geändert haben, die geblieben sind und die entfernt
wurden; die Chat-Antwort auf `status` nennt dieselben drei Listen.

**Keine Stimme.** Ein Auftrag auf einer Maschine ohne fertige Stimme läuft trotzdem fertig. Der Sprechtext bleibt in
der Geschichte, und jede Szene bekommt einen kurzen Text im Bild. Um zu exportieren, richte eine Stimme ein und drücke
**Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Diese Schaltfläche markiert die Szenen als stumm,
```

with:

```markdown
Ergebniskarte einer Überarbeitung listet die Szenen, die sich geändert haben, die geblieben sind und die entfernt
wurden; die Chat-Antwort auf `status` nennt dieselben drei Listen.

**Zeitpunkte.** Der Autor kann ein Bild auf eine Wortgruppe des Sprechtexts setzen statt auf eine Markierungsnummer. Die App findet die Wortgruppe und setzt die Markierung selbst. Steht die Wortgruppe nicht im Sprechtext, bekommt der Autor die nächstliegende Formulierung aus diesem Sprechtext und versucht es erneut.

**Bildprüfung.** Die Kompilierprüfung sieht sich auch das Bild an. Sie meldet diese Befunde:

- Text, der über den Bildrand ragt (`text_offframe`). Der Agent muss ihn beheben. Ein Film, der das beim Ende des Auftrags noch hat, ist `teilweise fertig`.
- Flackern: mehr als drei Blitze in einer Sekunde (`flash`). Auch das muss der Agent beheben.
- Text in den Bereichen, die Handy-Apps mit ihren Schaltflächen und Untertiteln überdecken (`text_unsafe`). Hinweis.
- Text, der sich schlecht vom Hintergrund abhebt, mit einem Kontrast unter 3 zu 1 (`text_contrast`). Hinweis.
- Ein langsamer Start: Szene 1 hat keinen Sprechtext, oder die erste Veränderung im Bild kommt erst nach einer halben Sekunde (`slow_hook`). Hinweis.
- In Code-Szenen: mehr als 3 Sekunden ohne sichtbare Veränderung (`dead_stretch`) oder ein leeres erstes Bild (`first_frame_empty`). Hinweis.

Hinweise blockieren das Ergebnis nie. Die Flackerprüfung hat eine Zeitgrenze. Kann sie nicht jedes Bild rechtzeitig prüfen, bleibt die Prüfung offen (`flash_unverified`), und der Film gilt nicht als `ready`. Exportierst du selbst, ist jeder Bildbefund eine Warnung; keiner davon stoppt den Export.

**Keine Stimme.** Ein Auftrag auf einer Maschine ohne fertige Stimme läuft trotzdem fertig. Der Sprechtext bleibt in
der Geschichte, und jede Szene bekommt einen kurzen Text im Bild. Um zu exportieren, richte eine Stimme ein und drücke
**Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Diese Schaltfläche markiert die Szenen als stumm,
```

In `documentation/manuals/de/34-storymaker.md`, replace:

````markdown
| Getrennte Tonspuren | Schreibt eine WAV je Audioebene: Sprechtext, Musik, Soundeffekte. Eine stumme Ebene bleibt weg. |
| In Video Studio öffnen | Schaltet die getrennten Tonspuren an, damit der fertige Export in Video Studio weitergehen kann. |

Die Dateien eines Exports:

```
````

with:

````markdown
| Getrennte Tonspuren | Schreibt eine WAV je Audioebene: Sprechtext, Musik, Soundeffekte. Eine stumme Ebene bleibt weg. |
| In Video Studio öffnen | Schaltet die getrennten Tonspuren an, damit der fertige Export in Video Studio weitergehen kann. |

Die eingebrannten Untertitel sitzen in einem 9:16-Film höher als in einem Querformat-Film, damit sie vom unteren Bildrand fernbleiben, wo Handy-Apps ihre Schaltflächen zeichnen. Bestehende 9:16-Filme zeigen die Untertitel beim nächsten Export etwas höher.

Die Dateien eines Exports:

```
````

In `documentation/manuals/de/34-storymaker.md`, replace:

```markdown
stehen und zeigt eine Warnung beim fertigen Export. Die Tonspuren ergeben zusammen genau den Mix, der vor dem Kodieren
daraus gemacht wird; die AAC-Spur im MP4 unterscheidet sich leicht, etwa 23 dB unter dem Mix.

## In Video Studio weiterschneiden

Ein fertiger Export mit Tonspuren bietet **In Video Studio öffnen** an, wenn Video Studio an ist, FFmpeg bereit ist
```

with:

```markdown
stehen und zeigt eine Warnung beim fertigen Export. Die Tonspuren ergeben zusammen genau den Mix, der vor dem Kodieren
daraus gemacht wird; die AAC-Spur im MP4 unterscheidet sich leicht, etwa 23 dB unter dem Mix.

**Tonwarnungen.** Der Export hört auch auf den Ton und listet, was er findet, als Warnungen am fertigen Export. Eine Warnung stoppt nie einen Export.

- `narration_dropout` oder `narration_clipped`: Ein Satz des Sprechtexts fiel mitten im Satz aus oder ist verzerrt (übersteuert). Die App erzeugt diesen Satz von selbst noch einmal. Hat die neue Aufnahme denselben Fehler, bleibt die Warnung. Formuliere den Satz um, oder wähle eine andere Stimme und drücke **Sprachausgabe erzeugen**.
- `bed_too_loud`: Die Musik ist beim Sprechen fast so laut wie die Stimme. Wähle im Musik-Reiter eine leisere Musikdatei.
- `bed_gap`: Die Musik fällt plötzlich um 12 dB oder mehr ab, sodass der Wechsel wie ein Schnitt klingt. Wähle eine Musikdatei ohne abrupte Enden.
- `mix_clipped`: Der gemischte Ton war mindestens einmal zu laut und wurde abgeschnitten. Meist hilft eine leisere Musikdatei.

## In Video Studio weiterschneiden

Ein fertiger Export mit Tonspuren bietet **In Video Studio öffnen** an, wenn Video Studio an ist, FFmpeg bereit ist
```

In `documentation/manuals/de/34-storymaker.md`, replace:

```markdown
| Der Export ist sehr langsam oder wurde vom Timeout gestoppt | Der Export zeichnet jedes Bild in einem Headless-Browser. Auf einer reinen CPU-Maschine ist ein aktuell installiertes Chrome schneller als das mitgelieferte Chromium. Längere Videos bekommen automatisch mehr Zeit, ein höheres `storymaker.render.timeout_seconds` ist also die Lösung. |
| Der KI-Autor startet nicht | `storymaker.agent.enabled` ist aus, es ist kein KI-Modell verfügbar, oder das Modell ist zu klein. Die Karte nennt den Grund. Ist das Tageslimit `storymaker.agent.max_daily_jobs` aufgebraucht, versuche es morgen noch einmal oder erhöhe das Limit. |
| „Es warten schon acht Agenten-Aufträge“ | Die Auftragswarteschlange ist voll. Warte, bis einer fertig ist. |
| Der Sprechtext hört nach einer Weile auf | Das Zeichenlimit für den Tag oder den Auftrag ist erreicht. Cloud-Stimmen sind begrenzt, lokale nicht. |
| Der Sprechtext wurde nicht erzeugt oder ein Zeichen fehlt | Für diese Sprache ist keine Stimme bereit, oder der Anbieter hat versagt: richte eine Stimme ein und drücke **Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Fehlt ein Zeichen, nennt der Fehler `glyph_missing` die Stelle; es steckt in keiner mitgelieferten Schrift, also schreibe das Wort um oder wähle eine andere Sprache. |
| Die Musik oder der Ton fehlt im Export | Die Musikdatei oder der Titel ist weg, oder es wurde nichts vorbereitet; wähle im Musik-Reiter wieder eine Quelle. Ein Film ohne Sprechtext, Musik und Soundeffekte wird mit einer stummen Spur exportiert, und das Export-Panel sagt das vorher. |
```

with:

```markdown
| Der Export ist sehr langsam oder wurde vom Timeout gestoppt | Der Export zeichnet jedes Bild in einem Headless-Browser. Auf einer reinen CPU-Maschine ist ein aktuell installiertes Chrome schneller als das mitgelieferte Chromium. Längere Videos bekommen automatisch mehr Zeit, ein höheres `storymaker.render.timeout_seconds` ist also die Lösung. |
| Der KI-Autor startet nicht | `storymaker.agent.enabled` ist aus, es ist kein KI-Modell verfügbar, oder das Modell ist zu klein. Die Karte nennt den Grund. Ist das Tageslimit `storymaker.agent.max_daily_jobs` aufgebraucht, versuche es morgen noch einmal oder erhöhe das Limit. |
| „Es warten schon acht Agenten-Aufträge“ | Die Auftragswarteschlange ist voll. Warte, bis einer fertig ist. |
| Der fertige Export zeigt eine Tonwarnung | Warnungen stoppen nie einen Export. Die Liste am fertigen Export sagt, was zu ändern ist; die Tonwarnungen oben erklären jede einzelne. |
| Der Sprechtext hört nach einer Weile auf | Das Zeichenlimit für den Tag oder den Auftrag ist erreicht. Cloud-Stimmen sind begrenzt, lokale nicht. |
| Der Sprechtext wurde nicht erzeugt oder ein Zeichen fehlt | Für diese Sprache ist keine Stimme bereit, oder der Anbieter hat versagt: richte eine Stimme ein und drücke **Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Fehlt ein Zeichen, nennt der Fehler `glyph_missing` die Stelle; es steckt in keiner mitgelieferten Schrift, also schreibe das Wort um oder wähle eine andere Sprache. |
| Die Musik oder der Ton fehlt im Export | Die Musikdatei oder der Titel ist weg, oder es wurde nichts vorbereitet; wähle im Musik-Reiter wieder eine Quelle. Ein Film ohne Sprechtext, Musik und Soundeffekte wird mit einer stummen Spur exportiert, und das Export-Panel sagt das vorher. |
```

In `documentation/manuals/en/34-storymaker.md`, replace:

```markdown
revision lists the scenes that changed, that stayed and that were removed; the chat `status` answer
gives the same three lists.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the
story, and each scene gets a short on-screen text. To export, either install a voice and press
**Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives
```

with:

```markdown
revision lists the scenes that changed, that stayed and that were removed; the chat `status` answer
gives the same three lists.

**Timing.** The author can time a picture to a phrase of the narration instead of a marker number. The app finds the phrase and places the marker itself. If the phrase is not in the narration, the author gets the closest wording of that narration and tries again.

**Picture checks.** The compile check also looks at the picture. It reports these findings:

- Text that sticks out of the frame (`text_offframe`). The agent must fix it. A film that still has it when the job ends is `partly done`.
- Flashing: more than three flashes in one second (`flash`). The agent must fix this too, for the same reason.
- Text in the areas that phone apps cover with their buttons and captions (`text_unsafe`). Advice.
- Text that is hard to read against its background, with a contrast below 3 to 1 (`text_contrast`). Advice.
- A slow start: scene 1 has no narration, or its first change on screen comes after half a second (`slow_hook`). Advice.
- In code scenes: more than 3 seconds without any visible change (`dead_stretch`), or an empty first frame (`first_frame_empty`). Advice.

Advice never blocks the outcome. The flash check has a time limit. If it cannot test every frame in time, the check stays open (`flash_unverified`) and the film does not read `ready`. When you export yourself, every picture finding is a warning, so none of them stops the export.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the
story, and each scene gets a short on-screen text. To export, either install a voice and press
**Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives
```

In `documentation/manuals/en/34-storymaker.md`, replace:

````markdown
| Stems | Writes one WAV per audio layer: narration, music, sound effects. A silent layer is left out. |
| Open in Video Studio | Turns stems on, so the finished export can continue in Video Studio. |

The files of one export:

```
````

with:

````markdown
| Stems | Writes one WAV per audio layer: narration, music, sound effects. A silent layer is left out. |
| Open in Video Studio | Turns stems on, so the finished export can continue in Video Studio. |

In a 9:16 film the burned-in caption sits higher than in a wide film, so it stays clear of the bottom of the screen, where phone apps draw their buttons. Existing 9:16 films show the caption a little higher the next time they are exported.

The files of one export:

```
````

In `documentation/manuals/en/34-storymaker.md`, replace:

```markdown
and shows a warning on the finished export. The stems add up to the mix exactly before encoding; the
AAC track in the MP4 differs slightly, about 23 dB below the mix.

## Continue in Video Studio

A finished export with stems offers **Open in Video Studio** when Video Studio is on, FFmpeg is
```

with:

```markdown
and shows a warning on the finished export. The stems add up to the mix exactly before encoding; the
AAC track in the MP4 differs slightly, about 23 dB below the mix.

**Sound warnings.** The export also listens to the sound and lists what it finds as warnings on the finished export. A warning never stops an export.

- `narration_dropout` or `narration_clipped`: a sentence of the narration went silent in the middle or is distorted. The app makes that sentence once more by itself. If the new take has the same fault, the warning stays. Rewrite the sentence, or choose another voice and press **Generate narration**.
- `bed_too_loud`: the music is nearly as loud as the voice while someone speaks. Choose a quieter music file in the Music tab.
- `bed_gap`: the music drops suddenly by 12 dB or more, so the change is heard as a cut. Choose a music file without abrupt endings.
- `mix_clipped`: the mixed sound was too loud at least once and was cut. A quieter music file usually helps.

## Continue in Video Studio

A finished export with stems offers **Open in Video Studio** when Video Studio is on, FFmpeg is
```

In `documentation/manuals/en/34-storymaker.md`, replace:

```markdown
| The export is very slow or stopped by the timeout | The export draws every frame in a headless browser. On a CPU-only machine an installed Chrome is faster than the provisioned Chromium. Longer videos get more time automatically, so a higher `storymaker.render.timeout_seconds` is the fix. |
| The AI author does not start | `storymaker.agent.enabled` is off, no AI model is available, or the model is too small. The card names the reason. If the daily limit `storymaker.agent.max_daily_jobs` is used up, try again tomorrow or raise the limit. |
| "Eight agent jobs are already waiting" | The agent queue is full. Wait for one to finish. |
| The narration stops after a while or was never generated | The character limit for the day or the job is reached, or no voice is ready for that language, or the provider failed. Cloud voices are limited, local ones are not. Set up a voice and press **Generate narration**, or press **Export without voice**. |
| The narration is missing a character | The error `glyph_missing` names the text. That character is in no bundled font; rewrite the word or choose another language. |
| The music or the sound is missing in the export | The music file or track is gone, or nothing was prepared; choose a source again in the Music tab. A film without narration, music and sound effects exports with a silent track, and the export panel says so before you start. |
```

with:

```markdown
| The export is very slow or stopped by the timeout | The export draws every frame in a headless browser. On a CPU-only machine an installed Chrome is faster than the provisioned Chromium. Longer videos get more time automatically, so a higher `storymaker.render.timeout_seconds` is the fix. |
| The AI author does not start | `storymaker.agent.enabled` is off, no AI model is available, or the model is too small. The card names the reason. If the daily limit `storymaker.agent.max_daily_jobs` is used up, try again tomorrow or raise the limit. |
| "Eight agent jobs are already waiting" | The agent queue is full. Wait for one to finish. |
| The finished export shows a sound warning | Warnings never stop an export. The list on the finished export says what to change, and the sound warnings above explain each one. |
| The narration stops after a while or was never generated | The character limit for the day or the job is reached, or no voice is ready for that language, or the provider failed. Cloud voices are limited, local ones are not. Set up a voice and press **Generate narration**, or press **Export without voice**. |
| The narration is missing a character | The error `glyph_missing` names the text. That character is in no bundled font; rewrite the word or choose another language. |
| The music or the sound is missing in the export | The music file or track is gone, or nothing was prepared; choose a source again in the Music tab. A film without narration, music and sound effects exports with a silent track, and the export panel says so before you start. |
```

In `documentation/storymaker.md`, replace:

```markdown

Watch every film once: when the compile check runs, the layout run flags captions over an actor's feet, props that are too small, boxes outside the frame and an actor that hides a prop or a text. A film reads `ready` only when no check is left open. Scenes that look alike (three or more in a row) are flagged as `scenes_look_alike`. The job card links the contact sheet of the last check. A film may run a little longer than asked, which is listed and is not a failure. Without a browser the compile check is skipped and reported as `compile_unverified`. The job still finishes.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the story, and each scene gets a short on-screen text. To export, either repair the Phonon resources/select a supported speech language and press **Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives each a reading time and turns speech bubbles into on-screen text. **Restore speech** undoes it while its copy, `story.before-silent.json`, exists. From chat, `render` with `silent: true` does the same and says that speech was removed.

**Narration and cost.** A job narrates by itself at its end, under the character limits above, so a finished job can export. Cloud voices can cost money. **Music** is not part of the agent: a request for music gets an answer that it cannot do this, and the Music tab stays with you.
```

with:

```markdown

Watch every film once: when the compile check runs, the layout run flags captions over an actor's feet, props that are too small, boxes outside the frame and an actor that hides a prop or a text. A film reads `ready` only when no check is left open. Scenes that look alike (three or more in a row) are flagged as `scenes_look_alike`. The job card links the contact sheet of the last check. A film may run a little longer than asked, which is listed and is not a failure. Without a browser the compile check is skipped and reported as `compile_unverified`. The job still finishes.

**Measured checks.** The compile check measures picture findings without a vision model. Each finding names a scene and a path, and its class decides what happens to it:

| Code | Measured in | Rule | Class |
| --- | --- | --- | --- |
| `text_offframe` | Go, layout text boxes | More than 1 % of a text's box lies outside the frame. Texts get this code instead of `offscreen`; actors and props keep `offscreen`. | hard |
| `text_unsafe` | Go, layout text boxes | More than 2 % of a text or the caption box lies in the aspect's safe zone. A text that has `text_offframe` is not reported here too. | advice |
| `text_contrast` | Go, the still of each storyboard scene | Contrast below 3:1 inside a text or caption box, from the 3rd and 97th percentile of relative luminance over a grid of at most about 20 000 samples. A ratio of 1.15:1 or less means the text is masked or not drawn, and is not reported. | advice |
| `flash` | Page frame probe | More than 3 flashes in any 1 s window. | hard |
| `flash_unverified` | Page frame probe | The probe did not reach every frame within its deadline. | open check |
| `slow_hook` | Player lint | Scene 1 has no narration, or its first show beat lands after 0.5 s. | advice |
| `dead_stretch` (code scene) | Page frame probe | More than 3 s of frames whose mean difference per colour channel to the previous frame stays below 1 % of full scale. | advice |
| `first_frame_empty` (code scene) | Page frame probe | The first frame after the entry transition is one colour. | advice |

Hard codes are model-fixable. The agent gets them as issues and spends check rounds on them, and a hard code left at the end makes the job `incomplete`. Advice gets one optional fix loop and never blocks. `flash_unverified` is listed as an open check, so a film reads `ready_unverified`, and it never blocks. In a person's own export every new code is a warning, so no export is refused for a picture finding.

Safe zones are fractions of the frame, measured from each edge:

| Aspect | Top | Right | Bottom | Left |
| --- | --- | --- | --- | --- |
| 9:16 | 11.5 % | 13 % | 21.9 % | 5.6 % |
| 16:9 | 5 % | 5 % | 5 % | 5 % |
| 1:1 | 5 % | 5 % | 5 % | 5 % |

The 9:16 zone is the union of the TikTok, Reels and Shorts overlays. The 16:9 and 1:1 zones equal the stage's safe area.

**Frame probe.** In the agent's check run only, never in an export, the check's browser page renders every frame of the film at 64 px width, in film order, within 20 s (`PROBE_DEADLINE_MS`). The probe runs only when the compile run sets `boot.layout`, and inside the compile deadline. Per frame it keeps the mean relative luminance and the mean difference per colour channel to the previous frame. A transition is a monotone run of luminance with a total change of at least 0.10 whose darker end is below 0.80. A flash is two transitions in opposite directions in a row. More than 3 flashes that start in any one second give one `flash` per scene, at the start of that second. On a fast PC a storyboard frame takes about 3 to 5 ms and a code-scene frame about 30 to 40 ms, so a long film with many code scenes can run out of the 20 s on a slower machine. Frames it does not reach give `flash_unverified`. The page may add a `probe_error` field to its report; Go ignores it. Go keeps only the four probe codes from the page's `checks` list, forces them to warnings and caps the list.

**Caption in 9:16.** The burned caption ends at 78 % of the frame height (it was 82 %) and is centred in the band from 5.6 % to 87 % of the width. Existing 9:16 films show the caption a little higher on their next export. 16:9 and 1:1 keep 92 %.

**Cue phrases.** A beat in `put_scene`, `patch_scene` or `dry_run` may carry `cue_phrase` (1 to 60 characters) instead of `cue`. Both on one beat are refused (`cue_phrase_conflict`). The Studio finds the phrase in the scene's narration. Case, punctuation and repeated spaces are ignored, and the phrase must start and end at word boundaries, except in CJK text. The first match wins, and the answer says so when the phrase occurs more than once. The Studio inserts a `{n}` marker before the match, reusing a marker that already stands there, and sets `cue: n`. A phrase that is not found is refused with `cue_phrase_not_found`; the answer names the closest word sequence of the same word count in that narration. A refusal counts as a write. The saved story stays plain `storymaker/1`, and code scenes keep `api.cue(n)`.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the story, and each scene gets a short on-screen text. To export, either repair the Phonon resources/select a supported speech language and press **Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives each a reading time and turns speech bubbles into on-screen text. **Restore speech** undoes it while its copy, `story.before-silent.json`, exists. From chat, `render` with `silent: true` does the same and says that speech was removed.

**Narration and cost.** A job narrates by itself at its end, under the character limits above, so a finished job can export. Cloud voices can cost money. **Music** is not part of the agent: a request for music gets an answer that it cannot do this, and the Music tab stays with you.
```

In `documentation/storymaker.md`, replace:

````markdown
story.json        the story document (the editor saves it with If-Match, so two windows cannot overwrite each other)
project.json      title, format and timestamps
scenes/<id>.js    code scenes (and <id>.bundle.js, the bundled copy the player runs)
audio/narration/  one audio file per sentence (<hash>.wav or .mp3) with its level curve (<hash>.env.json)
exports/          the exported videos and their records
```
````

with:

````markdown
story.json        the story document (the editor saves it with If-Match, so two windows cannot overwrite each other)
project.json      title, format and timestamps
scenes/<id>.js    code scenes (and <id>.bundle.js, the bundled copy the player runs)
audio/narration/  one audio file per sentence (<hash>.wav or .mp3) with its level curve and any audio faults (<hash>.env.json)
exports/          the exported videos and their records
```
````

In `documentation/storymaker.md`, replace:

```markdown

`POST /api/desktop/storymaker/projects/{id}/render` takes an optional body `{burn_in_captions, srt, vtt, stems, quality}`. Stems and subtitles land in `exports/<title>-<job>/`, and a copy of the MP4 lands in `Videos/Storymaker/`. A failed extra leaves the MP4 and adds a warning to the job. The export dialog below sets these options.

## Export options and extras

Starting an export opens a dialog with these choices:
```

with:

```markdown

`POST /api/desktop/storymaker/projects/{id}/render` takes an optional body `{burn_in_captions, srt, vtt, stems, quality}`. Stems and subtitles land in `exports/<title>-<job>/`, and a copy of the MP4 lands in `Videos/Storymaker/`. A failed extra leaves the MP4 and adds a warning to the job. The export dialog below sets these options.

**Audio checks.** Each freshly synthesized narration piece is checked for dropouts and clipping. A dropout is a speech run whose level falls by 35 dB or more in one 10 ms step, from at least -45 dBFS to below -80 dBFS, and stays below -80 dBFS for 50 ms to 3 s. Clipping is a run of at least 3 samples at or above 0.999 in magnitude, or of at least 4 equal samples at or above 0.05; a piece with 3 or more such runs in one second counts as clipped. A faulty piece is synthesized once more, and the retry is charged to the daily character budget. A fault that stays is kept in the piece's envelope (`faults`) and shown as `narration_dropout` or `narration_clipped` in the export and as `narration_faults` on the agent job card.

The export mix adds three detect-only warnings. None of them changes the mix:

- `bed_too_loud`: in a 6 s window in which the narration is active for at least half the time, the music is less than 12 dB below the voice, measured over the speaking moments only. One warning, with the time of the first such window.
- `bed_gap`: a steady bed (its level stays within 3 dB for at least 1 s) falls by 12 dB or more within 10 ms, outside the last 1.5 s.
- `mix_clipped`: the mix clamped at least one sample.

These are export job warnings, like `music_silent`. Audio findings never change the agent outcome and never refuse an export.

## Export options and extras

Starting an export opens a dialog with these choices:
```

In `internal/storymaker/AGENTS.md`, replace:

```markdown
- `get_code`/`put_code` operate only on existing scenes in fill/check/revise; fill is restricted to its scene. Source exchanges are capped at `StudioCodeBytes` (8 KiB). Writes require the code revision from `get_code`, pre-bundle before saving and pass the observed full etag to `SaveCodeScene`. Invalid code preserves saved files. Larger user sources stay for manual editing. Code writes share scene write/correction budgets and the live read-only gate.
- Bundle code before activating `kind: code` with `put_scene`; source writes alone do not complete fill. Code changes invalidate compile/check caches and participate in revision change lists even when story JSON is unchanged. Reverting source does not count as changed. Story/project format and renderer isolation remain unchanged.
- A loop per phase, and a loop per scene in fill. The story is the checkpoint, so a job is never resumed: a later revise finishes it. An invalid write stays a per-scene draft and is dropped at the end. A failed or exhausted scene keeps its outline text and is recorded as `scene_unfilled`.
- Charging: a drop in the issue count, a first write and a new scene are free. Limits are `StudioSceneChargedLimit`, `StudioSceneWriteLimit`, `StudioDryRunsPerScene`, `StudioDryRunsPerJob` and `StudioCheckRounds`. `IssueAdvisory` lists the non-blocking quality codes (`first_frame_empty`, `dead_stretch`, `fade_between_worlds`, `cue_late`, `words_tell`, `line_unread`, `text_too_small`, `entry_monotony`, `over_budget`, `spot_overlap`) that earn one optional fix loop and never block. `LintForJob` checks `scene_unfilled`, `over_budget`, `fade_between_worlds`, `entry_monotony` and `spot_overlap` statically, and `evaluateSceneWrite` includes `spot_overlap` warnings on saved/dry-run scene writes.
- Phase packets (`studio_packet.go`) ground outline in catalog `environments`, cast in `character_looks` (normal mode) and `style_options`, and fill in adjacent scene continuity (`previous_scene`, `next_scene`, `used_entries`, `suggested_props`) plus a cued, lip-synced `example`.
- `DecideOutcome` is the one rule for outcomes. `ready` means nothing is left open: the compile ran, the player measured the layout of every storyboard scene, no code scene exists, the narration is done and the open-check list is empty. `layout_unverified` is listed only when the layout was not verified (a code scene, no compile, no measurement). Craft and layout findings are model-fixable while rounds remain and never block (`issueNeverBlocks`). `over_target` and environment items never downgrade a film but keep it from `ready`.
- With `Settings.Stills` the compile run keeps its page open (transport `screenshot`, report answered with `start`), captures the hold frame of every measured scene (`render/compilestills.go`), and returns `Stills` (difference hashes) and `Sheet` (a 4 column contact sheet of 480 px thumbnails). `LookAlikeIssues` reports runs of three or more consecutive scenes within `lookAlikeMaxDistance` (16) bits as `scenes_look_alike` (layout class, never blocks). The agent job saves the sheet to `review/contact-sheet.png` (`ProjectStore.SaveReviewSheet`), records `agent.review` in the job document and serves it at `GET /api/desktop/storymaker/projects/{id}/review/contact-sheet.png`. The caption-overlap rule skips scenes with a close framing, a camera target or a camera beat (`LayoutOptions.CaptionExempt`).
- `LintFilmCraft` (`studio_craft.go`) is the create job's film-level lint: `environment_monotony`, `framing_monotony`, `gesture_monotony`, `cast_unused`, for films of at least 4 scenes. `IssueCraft` codes are model-fixable while check rounds remain and never block: `issueNeverBlocks` keeps them out of `incomplete`, and what is left is an open check. `DirectionPlan` (`studio_direction.go`, curated guide in `data/env_direction.json`) gives each fill packet a `direction` and builds the worked example from it. A new environment in the catalog needs a line in the guide: `TestEnvironmentGuideCoversTheCatalog` fails without it. Show entries are `pop`, `fade`, `slide` and `none`; never name another in a prompt or a fix text.
- The backup methods of `ProjectStore` (`SaveSilentBackup`, `LoadSilentBackup`, `HasSilentBackup`, `RemoveSilentBackup`) keep `story.before-silent.json`. `MakeSilent` is pure: it sets `silent: true` and `duration = max(1, WordCount / 2.3 + 0.4)`, removes `cue`, and turns each `say` with text into a `show`. Without the duration the film collapses to about 1 s per scene. Ticking `silent` by hand has no such repair.
```

with:

```markdown
- `get_code`/`put_code` operate only on existing scenes in fill/check/revise; fill is restricted to its scene. Source exchanges are capped at `StudioCodeBytes` (8 KiB). Writes require the code revision from `get_code`, pre-bundle before saving and pass the observed full etag to `SaveCodeScene`. Invalid code preserves saved files. Larger user sources stay for manual editing. Code writes share scene write/correction budgets and the live read-only gate.
- Bundle code before activating `kind: code` with `put_scene`; source writes alone do not complete fill. Code changes invalidate compile/check caches and participate in revision change lists even when story JSON is unchanged. Reverting source does not count as changed. Story/project format and renderer isolation remain unchanged.
- A loop per phase, and a loop per scene in fill. The story is the checkpoint, so a job is never resumed: a later revise finishes it. An invalid write stays a per-scene draft and is dropped at the end. A failed or exhausted scene keeps its outline text and is recorded as `scene_unfilled`.
- Charging: a drop in the issue count, a first write and a new scene are free. Limits are `StudioSceneChargedLimit`, `StudioSceneWriteLimit`, `StudioDryRunsPerScene`, `StudioDryRunsPerJob` and `StudioCheckRounds`. `IssueAdvisory` lists the non-blocking quality codes (`first_frame_empty`, `dead_stretch`, `fade_between_worlds`, `cue_late`, `words_tell`, `line_unread`, `text_too_small`, `text_unsafe`, `text_contrast`, `slow_hook`, `entry_monotony`, `over_budget`, `spot_overlap`) that earn one optional fix loop and never block. `cuephrase.go` turns a beat's `cue_phrase` into a `{n}` marker and `cue: n` before save; the story format and the player do not change. `LintForJob` checks `scene_unfilled`, `over_budget`, `fade_between_worlds`, `entry_monotony` and `spot_overlap` statically, and `evaluateSceneWrite` includes `spot_overlap` warnings on saved/dry-run scene writes.
- Phase packets (`studio_packet.go`) ground outline in catalog `environments`, cast in `character_looks` (normal mode) and `style_options`, and fill in adjacent scene continuity (`previous_scene`, `next_scene`, `used_entries`, `suggested_props`) plus a cued, lip-synced `example`.
- `DecideOutcome` is the one rule for outcomes. `ready` means nothing is left open: the compile ran, the player measured the layout of every storyboard scene, no code scene exists, the narration is done and the open-check list is empty. `layout_unverified` is listed only when the layout was not verified (a code scene, no compile, no measurement). Craft and layout findings are model-fixable while rounds remain and never block (`issueNeverBlocks`). `text_offframe` and `flash` are the hard exception: model-fixable, warnings for exports, and a left one makes the job `incomplete`. `over_target` and environment items never downgrade a film but keep it from `ready`.
- With `Settings.Stills` the compile run keeps its page open (transport `screenshot`, report answered with `start`), captures the hold frame of every measured scene (`render/compilestills.go`), and returns `Stills` (difference hashes) and `Sheet` (a 4 column contact sheet of 480 px thumbnails). `LookAlikeIssues` reports runs of three or more consecutive scenes within `lookAlikeMaxDistance` (16) bits as `scenes_look_alike` (layout class, never blocks). The agent job saves the sheet to `review/contact-sheet.png` (`ProjectStore.SaveReviewSheet`), records `agent.review` in the job document and serves it at `GET /api/desktop/storymaker/projects/{id}/review/contact-sheet.png`. The caption-overlap rule skips scenes with a close framing, a camera target or a camera beat (`LayoutOptions.CaptionExempt`).
- `LintFilmCraft` (`studio_craft.go`) is the create job's film-level lint: `environment_monotony`, `framing_monotony`, `gesture_monotony`, `cast_unused`, for films of at least 4 scenes. `IssueCraft` codes are model-fixable while check rounds remain and never block: `issueNeverBlocks` keeps them out of `incomplete`, and what is left is an open check. `DirectionPlan` (`studio_direction.go`, curated guide in `data/env_direction.json`) gives each fill packet a `direction` and builds the worked example from it. A new environment in the catalog needs a line in the guide: `TestEnvironmentGuideCoversTheCatalog` fails without it. Show entries are `pop`, `fade`, `slide` and `none`; never name another in a prompt or a fix text.
- The backup methods of `ProjectStore` (`SaveSilentBackup`, `LoadSilentBackup`, `HasSilentBackup`, `RemoveSilentBackup`) keep `story.before-silent.json`. `MakeSilent` is pure: it sets `silent: true` and `duration = max(1, WordCount / 2.3 + 0.4)`, removes `cue`, and turns each `say` with text into a `show`. Without the duration the film collapses to about 1 s per scene. Ticking `silent` by hand has no such repair.
```

In `internal/storymaker/render/AGENTS.md`, replace:

```markdown
  `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p` plus the encoder's filter tail, `+faststart`, and AAC 192k from the mix WAV (input 1, `-format_whitelist wav`), or from `anullsrc` with `-t` for a film without sound
  (`-shortest` is gone; the frame gate checks the audio length). A hardware encoder gets 1.5 s after the first frame to fail; a failure in the
  stream restarts once with libx264 (`{restart:true}` to the page).
- **Mix** (`mix.go`, `mixinput.go`, `meter.go`): a pure Go stream mixer at 48 kHz in blocks of 4800 frames. FFmpeg decodes pieces and music to s16 in job staging (`-protocol_whitelist file -format_whitelist wav,mp3 -map 0:a:0`, `-t` and `-fs` against decode bombs), each hash once. Order: voice match (cap +6 dB), bed gain to the reference (narration minus 12 LU, -18 LUFS without narration, relative to volume 0.25, cap +24 dB), ducking (6 dB, attack 50 ms, release 400 ms, look-ahead 50 ms, threshold -45 dBFS), carve (2200 Hz, Q 1.0, -5 dB, 20 ms ramps, only inside spans), impact dips, master gain `min(-16-I, -1-TP, -0.1-max stem peak)`. No limiter. Mix minus the three stems is 0 at every sample before encoding, and only there. The meter is one FFmpeg process on a float pipe: I from ebur128, TP from loudnorm. `MeterStored` runs the same graph over a stored file on stdin, for the music job. A silent bed is dropped with the warning `music_silent`.
- **Deferred start** (`render.go`, `listener.go`): `POST /report` validates and answers `pending` at once, a goroutine mixes on `mixCtx`, starts FFmpeg and the watchdog, and closes `startCh`. The page long-polls `GET /start` (held 20 s). `run.audioPath` survives `recoverEncoder`, so a hardware-encoder fallback keeps the mix. `execute` cancels the mix and waits for it before staging is removed.
- **Frame gate** (`gate.go`): packet count equal to the frame count and audio against video under 25 ms, after FFmpeg and before anything is published. See contract section 7.
- **Options and extras** (`options.go`, `subtitles.go`, `publishExtras`): `RenderOptions` joins the idempotency fingerprint, `high` sets CRF 18, `burn_in_captions` overrides `style.captions` in the snapshot before `Validate`. Stems and subtitles go to `exports/<slug>-<job8>/`, the library copy to `Videos/Storymaker/`. A failed extra leaves the MP4 and adds `stem_failed`, `subtitle_failed` or `library_failed`. A silent stem is omitted. The snapshot holds no sfx WAV.
```

with:

```markdown
  `scale=out_color_matrix=bt709:out_range=tv,format=yuv420p` plus the encoder's filter tail, `+faststart`, and AAC 192k from the mix WAV (input 1, `-format_whitelist wav`), or from `anullsrc` with `-t` for a film without sound
  (`-shortest` is gone; the frame gate checks the audio length). A hardware encoder gets 1.5 s after the first frame to fail; a failure in the
  stream restarts once with libx264 (`{restart:true}` to the page).
- **Mix** (`mix.go`, `mixinput.go`, `meter.go`): a pure Go stream mixer at 48 kHz in blocks of 4800 frames. FFmpeg decodes pieces and music to s16 in job staging (`-protocol_whitelist file -format_whitelist wav,mp3 -map 0:a:0`, `-t` and `-fs` against decode bombs), each hash once. Order: voice match (cap +6 dB), bed gain to the reference (narration minus 12 LU, -18 LUFS without narration, relative to volume 0.25, cap +24 dB), ducking (6 dB, attack 50 ms, release 400 ms, look-ahead 50 ms, threshold -45 dBFS), carve (2200 Hz, Q 1.0, -5 dB, 20 ms ramps, only inside spans), impact dips, master gain `min(-16-I, -1-TP, -0.1-max stem peak)`. No limiter. Mix minus the three stems is 0 at every sample before encoding, and only there. The meter is one FFmpeg process on a float pipe: I from ebur128, TP from loudnorm. `MeterStored` runs the same graph over a stored file on stdin, for the music job. A silent bed is dropped with the warning `music_silent`. `mixcheck.go` adds the detect-only warnings `bed_too_loud`, `bed_gap` and `mix_clipped` on the pass-3 stem blocks; they never change the mix or refuse an export. Narration faults (`narration_dropout`, `narration_clipped`) come from the piece envelopes and are warnings too.
- **Deferred start** (`render.go`, `listener.go`): `POST /report` validates and answers `pending` at once, a goroutine mixes on `mixCtx`, starts FFmpeg and the watchdog, and closes `startCh`. The page long-polls `GET /start` (held 20 s). `run.audioPath` survives `recoverEncoder`, so a hardware-encoder fallback keeps the mix. `execute` cancels the mix and waits for it before staging is removed.
- **Frame gate** (`gate.go`): packet count equal to the frame count and audio against video under 25 ms, after FFmpeg and before anything is published. See contract section 7.
- **Options and extras** (`options.go`, `subtitles.go`, `publishExtras`): `RenderOptions` joins the idempotency fingerprint, `high` sets CRF 18, `burn_in_captions` overrides `style.captions` in the snapshot before `Validate`. Stems and subtitles go to `exports/<slug>-<job8>/`, the library copy to `Videos/Storymaker/`. A failed extra leaves the MP4 and adds `stem_failed`, `subtitle_failed` or `library_failed`. A silent stem is omitted. The snapshot holds no sfx WAV.
```

In `internal/storymaker/render/AGENTS.md`, replace:

```markdown
- **Timeouts**: the browser setup of a first export (the Chromium download) runs under `render.timeout_seconds` before the render clock starts
  (`OpenPage` keeps the job context: the browser lives as long as it). Report within 3 minutes of the navigation, no frame within 10 s after FFmpeg started (after the mix, not when the report is accepted) kills the job as `code_error` (the timer is held while Go itself works on a delivered frame: FFmpeg write, hardware settle, software restart), overall
  `max(render.timeout_seconds, 3 x frames / measuredFPS[transport])`. Any `POST /error` from the page is fatal.
- **Compile check with stills** (`compilecheck.go`): with `Settings.Stills` the check bounds its time at 3 times `compileCheckLimit` (`stillsLimitFactor`), uses the screenshot transport and answers the report with `start`; without it nothing changes.
- **Workers** (`workers.go`): `ChooseWorkers(transport, logicalCPUs, freeMB, width, height, chunk, cjk, codeScenes)` sets `PageParams.Workers`. Chunk is 8, pre-roll 0 by default and 2 when on. All three go into `boot.json` with `omitempty`. `Settings.LogicalCPUs` and `Settings.FreeMemoryMB` come from the server, and `FreeMemoryMB` 0 means no legacy memory bound (catalog v2 additionally uses `BoundRasterWorkers` and selects one worker when unknown). Only `png` runs parallel, because worker encoding exists only for PNG. Ingest is unchanged: one in-order poster, one POST in flight, and the restart stays an answer to `/frame`. `TestParallelParityBrowser` pins identical RGBA for N workers against 1 on `firewall.json` and `speech.json`, pre-roll on and off. `TestParallelEncoderFailureBrowser` forces one encoder failure.
- **Visual acceptance**: `AURAGO_STORYMAKER_REVIEW_DIR` retains the reference MP4, subtitles and timing metrics. Set `AURAGO_STORYMAKER_BENCHMARK=1` separately for process-tree RSS sampling; OS enumeration must not slow ordinary export checks.
```

with:

```markdown
- **Timeouts**: the browser setup of a first export (the Chromium download) runs under `render.timeout_seconds` before the render clock starts
  (`OpenPage` keeps the job context: the browser lives as long as it). Report within 3 minutes of the navigation, no frame within 10 s after FFmpeg started (after the mix, not when the report is accepted) kills the job as `code_error` (the timer is held while Go itself works on a delivered frame: FFmpeg write, hardware settle, software restart), overall
  `max(render.timeout_seconds, 3 x frames / measuredFPS[transport])`. Any `POST /error` from the page is fatal.
- **Frame probe** (`compilecheck.go`): runs inside the compile check only when `boot.layout` is set, never in exports. The page's `checks` list is kept to four codes (`flash`, `flash_unverified`, `dead_stretch`, `first_frame_empty`), forced to warnings, and the page's `probe_error` is ignored. The probe deadline is 20 s (`PROBE_DEADLINE_MS`) inside the compile deadline.
- **Compile check with stills** (`compilecheck.go`): with `Settings.Stills` the check bounds its time at 3 times `compileCheckLimit` (`stillsLimitFactor`), uses the screenshot transport and answers the report with `start`; without it nothing changes.
- **Workers** (`workers.go`): `ChooseWorkers(transport, logicalCPUs, freeMB, width, height, chunk, cjk, codeScenes)` sets `PageParams.Workers`. Chunk is 8, pre-roll 0 by default and 2 when on. All three go into `boot.json` with `omitempty`. `Settings.LogicalCPUs` and `Settings.FreeMemoryMB` come from the server, and `FreeMemoryMB` 0 means no legacy memory bound (catalog v2 additionally uses `BoundRasterWorkers` and selects one worker when unknown). Only `png` runs parallel, because worker encoding exists only for PNG. Ingest is unchanged: one in-order poster, one POST in flight, and the restart stays an answer to `/frame`. `TestParallelParityBrowser` pins identical RGBA for N workers against 1 on `firewall.json` and `speech.json`, pre-roll on and off. `TestParallelEncoderFailureBrowser` forces one encoder failure.
- **Visual acceptance**: `AURAGO_STORYMAKER_REVIEW_DIR` retains the reference MP4, subtitles and timing metrics. Set `AURAGO_STORYMAKER_BENCHMARK=1` separately for process-tree RSS sampling; OS enumeration must not slow ordinary export checks.
```

In `ui/js/storymaker/AGENTS.md`, replace:

```markdown
  cut fade slide-left wipe (others fall back to fade with a `not_supported_yet` note); everything else the validator accepts is ignored.
- Motion: `motion.js` holds rhythm factors, beat defaults, lead fractions and holds; `beats.js` runs the scheduler; `lint.js` holds the
  static lint, which reads `built.log` and never blocks a render. Code-scene API defaults are not scaled.
- `compose.js` holds the composition v2 rules (hero scale, push-in, camera test); `stage.js` applies the caption-safe floor. Both are inert without `style.compose: "v2"`.
- Text: compile never measures. Shrink to fit and balance run in `text.layout`. The layout memo is keyed on text, style name, canvas
  size, family and `maxWidth`, because `maxWidth` follows the anchor, so frame n stays a pure function of n.
```

with:

```markdown
  cut fade slide-left wipe (others fall back to fade with a `not_supported_yet` note); everything else the validator accepts is ignored.
- Motion: `motion.js` holds rhythm factors, beat defaults, lead fractions and holds; `beats.js` runs the scheduler; `lint.js` holds the
  static lint, which reads `built.log` and never blocks a render. Code-scene API defaults are not scaled.
- `checks.js` (`SM.checks`) holds the frame probe rules (flash, code-scene `dead_stretch` and `first_frame_empty`). The probe runs in `host-render.js` only when `boot.layout` is set, under `PROBE_DEADLINE_MS` (20 s). The 9:16 caption bottom (0.78) and its band (0.056 to 0.870) copy `layout.go SafeZone` and must follow it.
- `compose.js` holds the composition v2 rules (hero scale, push-in, camera test); `stage.js` applies the caption-safe floor. Both are inert without `style.compose: "v2"`.
- Text: compile never measures. Shrink to fit and balance run in `text.layout`. The layout memo is keyed on text, style name, canvas
  size, family and `maxWidth`, because `maxWidth` follows the anchor, so frame n stays a pure function of n.
```

(No red state: documentation.)

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-11-01-storymaker-contract.md documentation/manuals/de/34-storymaker.md documentation/manuals/en/34-storymaker.md documentation/storymaker.md internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md ui/js/storymaker/AGENTS.md
git commit -m "storymaker: document the measured checks, cue phrases and the audio checks"
```

