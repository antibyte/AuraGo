# Goldfish Adventure: LucasArts controls

Date: 2026-11-07. App `goldfish-adventure` (**Bitte nicht das Universum füttern**).
Status: approved design, implementation in progress. Supersedes the interaction
bullets of `plans/2026-11-06-goldfish-adventure.md` ("Click or keyboard-focus a
hotspot, then choose its action"; "Only explicit Next/Close acknowledges a line").

## Problem

The shipped game feels like a web app, not a LucasArts adventure:

- every click on an object opens a text card that must be confirmed with "Weiter";
- actions are buttons in a right-hand sidebar, with a toolbar on top and a status footer;
- the 29 puzzle prompts (12 choice, 10 sequence, 7 match) open form dialogs with radio
  buttons, selects and a "Prüfen" button.

## Decisions (user-approved)

SCUMM-style bar with quick click, dialogue that runs by itself, puzzle prompts as
conversation menus. Content, `GoldfishState`, save format, art, animation, audio and
story translations do not change.

### Layout

- Toolbar, sidebar and footer are removed. The app is the scene (16:9, as wide as the
  window allows) with the **bar** directly below it.
- Bar: sentence line on top, left the four verbs (Ansehen, Nehmen, Benutzen, Sprechen),
  right the inventory grid (about 6 × 2 slots, up/down scroll arrows), a small ☰ menu
  button at the right end of the sentence row. Idle sentence line shows the room title
  (muted). The chapter ("Akt N") appears in the menu dialog.
- Menu dialog (button and Esc when nothing else is active): Reisejournal, Ein kleiner
  Hinweis, Spielstände, Gesprochenes (speech log), Einstellungen, Pause/Weiterspielen,
  chapter + room + playtime, save status and the export button when a save failed.
  The ☰ button carries the `agfish-warning` state when the save status is a warning.
  A visually hidden `role="status"` `.agfish-save-status` stays in the root so the
  status is always readable by tests and screen readers.
- Pixel look: Px437 face (already shipped), square corners, 1 px pixel borders, a pixel
  cursor over the scene. Scripts the bitmap face cannot draw (cs, el, hi, ja, pl, zh)
  keep the existing serif fallback. Both Desktop themes keep readable bar contrast.

### Interaction

| Input | Result |
| --- | --- |
| Hover hotspot / exit / item | Sentence line shows the would-be command, e.g. "Benutze Förderband und Not-Aus". Exits show their own label. Idle: room title. |
| Left click, no verb armed | R0ST walks to the hotspot's `walk` point, then runs the default action. Default = the available actions without `item`; exactly one → perform it; two or more → choice list in the bar; none → look. If only item actions exist, R0ST says `goldfish.ui.item_needed`. |
| Right click / long press / Shift+Enter | Ansehen (the hotspot's `look` lines). |
| Click a verb, then an object | Applies that verb to the next click, then the verb resets. Matching actions are filtered by verb; none → an R0ST refusal line (`goldfish.ui.refuse_<verb>_1/2`, alternating). |
| Verb **Nehmen** | Also matches actions whose effects add items (otherwise it has 2 hits in 130 actions). |
| Click an inventory item | Arms "Benutze {item} mit …". Next click on a hotspot performs the action whose `item` is that item, else `refuse_item_1/2`. Click the item again, Esc, or right click on the scene disarms. Right click on an item = Ansehen (its description). |
| Click on the floor | R0ST walks there (as today). A click while R0ST is walking retargets; a second click on the same target finishes the walk immediately. |
| Enter on a focused hotspot | Same as left click (the semantic hotspot buttons stay focusable). Tab order: hotspots, exits, verbs, items, menu. |
| Touch | Tap = left click without hover; long press (500 ms) = Ansehen. |
| Esc | Active speech: skip the queue (see below). Else armed verb/item or open choice: cancel. Else open the menu. |
| Space | Active speech: skip the line. Otherwise pause as today. |

Exits are clickable zones (the existing rects) without a text button look; locked exits
keep their `blocked` lines through R0ST speech.

### Dialogue

- No "Weiter" or "Überspringen" buttons and no text band. A line appears as outlined
  pixel text in the speaker's colour above the speaker's head (renderer exposes
  `anchor(id)`; `speakers` of scene actors map to actors, R0ST uses the player
  position, narrator and speakers without an actor appear top centre with the speaker
  name). Text is clamped inside the scene.
- Duration per line: `clamp(900 + chars × rate, 1600, 9000)` ms with rate 70 / 48 / 32
  for slow / normal / fast. The timer pauses while the game is paused/hidden.
- Lines advance by themselves. Click on the scene or Space skips the current line.
- Two kinds of queues:
  - *Story queues* (action results, room entry, resume): a click only skips the current
    line, hotspots and verbs wait until the queue ends (bar dimmed).
  - *Ambient queues* (look lines, item descriptions, refusals, blocked exits): any click
    on a hotspot, verb, item or the floor cancels the speech and acts immediately.
- A line counts as **read** when its timer has elapsed while visible, or the player
  skipped it. Read lines are added to `state.seenLines` (story keys only; `goldfish.ui.*`
  lines are never recorded because `GoldfishState.validate` accepts story keys only).
  Result queues persist each newly read line as today. Esc cancels the rest of a queue
  **without** marking unread lines (they return after continue/load/import, the existing
  recovery contract). Esc over an already read queue just skips it.
- Preferences live in `localStorage` (try/catch, never in the save): `autoAdvance`
  (default on) and `readSpeed` (`slow|normal|fast`, default normal). With auto-advance
  off a line waits for a click/Space/Enter and shows a small ▶ marker.
- Speech log: the last 40 lines (speaker + text) in the menu ("Gesprochenes"). Every
  line is also written to a visually hidden `aria-live="polite"` region.
- No timed answers: choices never expire.

### Puzzle prompts as conversation menus

The challenge forms become menus inside the bar (the verb/inventory area is replaced):

- *choice*: one line per option, click answers immediately.
- *sequence*: options are clicked in order, picked steps are shown in the sentence line
  ("Anhalten → Halten → …"); the last step checks automatically; a "↶ Einen Schritt
  zurück" line undoes one step.
- *match*: pairs are asked one after another ("Was gehört zu „{left}“?"), checked after
  the last pair; the same undo line applies.
- Every menu has the prompt text as heading and a cancel line (Esc). Wrong answers play
  the record's `wrongLines` and reset the menu as before; correct ones call `perform`.
  Answer correctness stays in `GoldfishState.apply`; the menu only collects answers.

### Not changing

`content*.js`, `state.js`, `save.js`, save schema/limits, the renderer's animation and
moments, art, audio, `ui/lang/goldfish/*`, and the 16 puzzle/route/ending reducers.
Failed-save rollback, refreshed checkpoints, close guard, new-game rollback and
unread-result recovery keep their exact contracts (only the acknowledgement trigger
changes as described above).

## Modules and interfaces

Load order (module-loader.js, Go registration test, bundle): `content`, `content-act1..4`,
`state`, `save`, `renderer`, **`commands`**, `interactions`, **`speech`**, **`panel`**, `app`.
Each file stays below 1100 lines; globals are frozen `window.Goldfish*` objects.

- `commands.js` → `GoldfishCommands` (pure, no DOM): `verbOf(record)`
  (`use|talk|take`; `take` also for item-granting actions), `actionsFor(state, hotspot,
  {verb, item})`, `defaultAction(state, hotspot)` → `{kind: 'one'|'many'|'item'|'look', ...}`,
  `sentence(t, {verb, hotspot, exit, item})`. Unit-testable in Node.
- `speech.js` → `GoldfishSpeech.create({scene, t, anchor, live, onRead, onEnd})`:
  `play(lines, {story, allSeen, seen})`, `skipLine()`, `skipAll()` (Esc), `cancel()`,
  `setPaused(bool)`, `active()`, `story()`, `log()`, `prefs()/setPrefs()`, `dispose()`.
- `panel.js` → `GoldfishPanel.create({root, t, button, element})`: sentence line, verb
  buttons, inventory grid with scrolling, choice-menu mode (`menu({title, lines, onPick,
  onBack, onCancel})`, `closeMenu()`), `setDim(bool)`, `setSentence(text)`,
  `armedVerb()`, `armedItem()`, `render(state)`.
- `interactions.js`: `challenge(record, complete)` drives `panel.menu`; saves/import/
  export unchanged.
- `renderer.js`: add `anchor(id)` returning the normalised head position `{x, y}` or
  `null`; nothing else changes.
- `app.js`: wiring only (state, persistence, travel, perform, hint/journal/settings
  dialogs, menu dialog, start screen, lifecycle, pause).

### DOM contract kept for tests

`.agfish-app[data-room]`, `.agfish-busy`, `.agfish-art-loading`, `.agfish-start` (+
`.agfish-primary`), `.agfish-art-status`, `.agfish-canvas`, `.agfish-hotspot[data-agfish-hotspot]`,
`.agfish-exit[data-agfish-exit]`, `.agfish-item[data-agfish-item]`, `.agfish-save-status`.
New: `.agfish-panel`, `.agfish-sentence`, `.agfish-verb[data-agfish-verb]`,
`.agfish-menu-button`, `.agfish-choice-line` (`data-agfish-action` for action choices,
`data-agfish-option` for puzzle options), `.agfish-speech[data-speaker]` with
`.agfish-line`, `.agfish-speech-live`. Removed: `.agfish-toolbar`, `.agfish-sidebar`,
`.agfish-context`, `.agfish-dialogue*`, `.agfish-footer`, the challenge dialog.

## New interface strings (`ui/lang/goldfish-interface/<locale>.json`, all 16 locales)

German source text; genuine translations, written in each locale's own register (R0ST is
dry and literal). `{…}` placeholders stay verbatim; word order is the translator's choice.

```
goldfish.ui.verb_look        Ansehen
goldfish.ui.verb_take        Nehmen
goldfish.ui.verb_use         Benutzen
goldfish.ui.verb_talk        Sprechen
goldfish.ui.sentence_look    Sieh dir {object} an
goldfish.ui.sentence_take    Nimm {object}
goldfish.ui.sentence_use     Benutze {object}
goldfish.ui.sentence_talk    Sprich mit {object}
goldfish.ui.sentence_walk    Gehe zu {object}
goldfish.ui.sentence_use_item       Benutze {item} mit {object}
goldfish.ui.sentence_use_item_open  Benutze {item} mit …
goldfish.ui.panel_label      Aktionsleiste
goldfish.ui.verbs_label      Verben
goldfish.ui.inventory_up     Inventar nach oben
goldfish.ui.inventory_down   Inventar nach unten
goldfish.ui.menu             Menü
goldfish.ui.speech_log       Gesprochenes
goldfish.ui.speech_log_empty Noch nichts gesagt.
goldfish.ui.auto_advance     Dialoge automatisch weiterschalten
goldfish.ui.read_speed       Lesetempo
goldfish.ui.read_speed_slow  Gemächlich
goldfish.ui.read_speed_normal Normal
goldfish.ui.read_speed_fast  Zügig
goldfish.ui.choice_back      Einen Schritt zurück
goldfish.ui.match_pick       Was gehört zu „{left}“?
goldfish.ui.refuse_take_1 / _2   R0ST: das lässt sich nicht mitnehmen (zwei Varianten)
goldfish.ui.refuse_use_1 / _2    R0ST: das lässt sich so nicht benutzen
goldfish.ui.refuse_talk_1 / _2   R0ST: das hört nicht zu / antwortet nicht
goldfish.ui.refuse_item_1 / _2   R0ST: der Gegenstand passt hier nicht
```

Changed text: `goldfish.ui.pause_hint` → "Linksklick: handeln · Rechtsklick: ansehen · Esc: Menü ·
Leertaste: Pause" (translated). Old keys that become unused stay (no churn).
Refusal lines are spoken by `r0st` (`goldfish.ui.speaker_r0st` exists).

## Tasks

1. **Core** (one maker): `commands.js`, `speech.js`, `panel.js`, `interactions.challenge`,
   `renderer.anchor`, `app.js` rewiring, `ui/css/desktop-app-goldfish-adventure.css`,
   `module-loader.js` + Desktop bundle rebuild, Go registration list, both `AGENTS.md`
   contracts and the superseded notes in the 2026-11-06 plan.
2. **Strings** (parallel maker): the 33 new keys + changed `pause_hint` in all 16
   `ui/lang/goldfish-interface/*.json`, JSON-valid, UTF-8 without BOM, same key set in
   every locale.
3. **Tests**: `scripts/test-goldfish-commands.mjs` (pure logic: default action, verb
   filter, Nehmen rule, sentence), adapt `scripts/test-goldfish-app.mjs` to the new DOM
   (every existing regression must keep its meaning), adapt
   `ui/desktop_goldfish_adventure_test.go` (registration order, opening browser flow).
4. **Acceptance**: real Chrome through the Desktop harness: opening puzzle with
   quick clicks, no confirmation needed, speech placement, choice menus, both themes,
   narrow window, keyboard-only path, pause/close/reopen.

## Verification

`node scripts/test-goldfish-commands.mjs`, `node scripts/test-goldfish-state.mjs`,
`node scripts/test-goldfish-save.mjs`, `node scripts/test-goldfish-app.mjs`,
`node scripts/test-goldfish-animation.mjs`, `node scripts/verify-goldfish-assets.mjs`,
`go test ./ui -run 'Goldfish' -count=1`, plus the screenshots of task 4.
