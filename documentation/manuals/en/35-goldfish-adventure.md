# Chapter 35: Please Do Not Feed the Universe – a painted point-and-click adventure

Please Do Not Feed the Universe is a hand-painted point-and-click adventure in the Virtual Desktop. It
runs entirely in your browser and needs no provider, no account and no external service: the art, the
story and the music ship with AuraGo, and the saves never leave your browser. A discarded maintenance
robot wants his warranty reinstated, and along the way he rescues three furious goldfish, befriends a
grieving ape and finds out that the galaxy's celebrated happiness service is removing the right to
disagree.

The game has sixteen rooms in four acts, original artwork, and one generated instrumental song per
room. It is a complete game with its own saves, its own journal and its own hints. The German title is
**Bitte nicht das Universum füttern**.

## Table of Contents
1. [Opening the Game](#opening-the-game)
2. [Controls](#controls)
3. [The Action Bar](#the-action-bar)
4. [Dialogue](#dialogue)
5. [The Sixteen Rooms](#the-sixteen-rooms)
6. [Music and Sound](#music-and-sound)
7. [Saves and Settings](#saves-and-settings)
8. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening the Game

Open **Please Do Not Feed the Universe** from the start menu, in the category **Games**. The start
screen offers **New journey** and, when a saved journey exists, **Continue journey**.

The game runs in sixteen rooms over four acts. Every required clue has two independent sources (an
observation plus a line of dialogue, for example), so nothing has to be guessed from a pixel hunt.
There are 18 authored puzzle situations, 130 authored actions, three binary branch points and three
endings; all 24 route-and-ending combinations work.

You walk R0ST, the robot. Companions act with you in context: you ask, you explain the purpose, you get
the agreement, and then the joint action happens. Bruno and the fish can refuse. They are never
inventory objects and never remote-controlled tools.

## Controls

The game uses a LucasArts-style scheme. A left click is the main action, a right click looks at
something.

| Input | Result |
|-------|--------|
| Hover a hotspot, exit or item | The sentence line shows the command that would run, for example "Use conveyor and emergency stop". Exits show their own label. Idle: the room title. |
| Left click | R0ST walks to the hotspot and runs its default action. Exactly one available action runs it; two or more open a choice list in the bar; none means look. |
| Right click, long press or Shift+Enter | **Look** at the hotspot. |
| Click a verb, then an object | Applies that verb to the next click; the verb then resets. |
| Click an inventory item | Arms "Use {item} with …". The next click on a hotspot performs the matching action. Click the item again, press Escape or right-click the scene to disarm. |
| Right click on an inventory item | **Look** at its description. |
| Click on the floor | R0ST walks there. A click while he is walking retargets; a second click on the same spot finishes the walk immediately. |
| Enter on a focused hotspot | The same as a left click. The tab order is hotspots, exits, verbs, items, menu. |
| Escape | Active speech: skip the queue. Otherwise: cancel an armed verb, item or open choice. Otherwise: open the menu. |
| Space | Active speech: skip the line. Otherwise: pause. |
| Touch | Tap is a left click without hover; a long press of 500 ms is **Look**. |

Every exit is a clickable zone with a permanent direction sign on the frame edge: gold while open, grey
and dashed while locked. Its label stays in the sentence line on hover.

## The Action Bar

Under the scene sits a single bar, in the style of the classic point-and-click games. The sentence
line is on top; below it, on the left, are the four verbs, and on the right the inventory grid (about
6 × 2 slots, with scroll arrows).

| Verb | What it does |
|------|--------------|
| **Look** | Reads the hotspot's own look lines. |
| **Take** | Picks up what can be picked up. It also matches actions that hand you an item. |
| **Use** | The default for anything else. |
| **Talk** | Starts a conversation. |

A small ☰ button at the right end of the sentence row opens the menu: travel journal, a small hint,
saved journeys, what was said (the speech log), settings, pause and resume, plus the current act, the
room and the playtime. The save status is shown there, and the export button appears when a save
failed. The ☰ button carries a warning mark when the save status is a warning.

Puzzle prompts are conversation menus in the same bar, not separate dialogs:

- A **choice** shows one line per option; a click answers immediately.
- A **sequence** asks for the steps in order; the picked steps show in the sentence line, the last step
  checks on its own, and a "back one step" line undoes one step.
- A **match** asks one pair after another, for example "What goes with ‘{left}’?", and checks after the
  last pair.

Every menu has the prompt text as its heading and a cancel line (Escape). A wrong answer plays the
authored refusal and resets the menu.

## Dialogue

There is no **Continue** button. A line appears as outlined pixel text in the speaker's colour above
the speaker's head, and it advances by itself. Click on the scene or press Space to skip the current
line.

Lines last `clamp(900 + characters × rate, 1600, 9000)` milliseconds, with a rate of 70 for slow, 48
for normal and 32 for fast reading. The timer pauses while the game is paused or the window is hidden.

Two kinds of queues exist. **Story queues** (an action result, entering a room, resuming) keep the
scene blocked until they end; the bar is dimmed and hotspots and verbs wait. **Ambient queues** (look
lines, item descriptions, refusals, blocked exits) give way to any click on a hotspot, verb, item or
the floor.

A line counts as read when its timer has run out while it was visible, or when you skipped it. Read
lines are recorded, so you can skip staging you have already seen. Pressing Escape over a queue cancels
the rest of it without marking the unread lines as read; they come back after continue, load or
import.

Two preferences live in the browser, never in the save: **Advance dialogue automatically** (on by
default) and **Reading speed** (leisurely, normal or brisk; normal by default). With auto-advance off, a
line waits for a click, Space or Enter and shows a small ▶ marker.

The speech log keeps the last 40 lines with their speaker. It is in the menu under **What was said**.

## The Sixteen Rooms

| ID | Room | Act | What happens there |
|----|------|-----|---------------------|
| R01 | Junkyard of Second Chances | I | The tutorial and the first evidence. R0ST stops the conveyor, repairs the grabber and opens the exit. |
| R02 | Office of Emotional Affairs | I | R0ST's own dated maintenance record and the public procedure for correcting facts. |
| R03 | Café with Aquarium | I | The fish remember a test differently; the café recording was cut. The consent and the safe transport happen here. |
| R04 | Launch Pier under a Wide Sky | I | A fuel gauge contradicts the visible flow. Bruno boards and shares his cancellation receipt. |
| R05 | The Maybe Thursday – Bridge and Lounge | II | The recurring home. Bruno's quiet scene and changed views on later visits. |
| R06 | Engine Room and Cargo Belly | II | The manifest contradictions and the memory cargo revelation. |
| R07 | Crooked Customs on the Tilted Asteroid | II | The public route or the maintenance route; both are real, and they differ in dialogue and blocking. |
| R08 | Market of Abandoned Promises | II | Obligations are tradable, repairable and sometimes best left to expire. |
| R09 | Bruno's Arboretum | III | The old walkway or the new seed lift; two ways, one destination. |
| R10 | Weather Station under Storm Glass | III | Horticultural knowledge becomes engineering. A storm is trapped behind a fake heater. |
| R11 | Register Depot behind Deep Glass | III | The memory registry. Bruno's one-week consent was converted into automatic renewal. |
| R12 | Repair Hall of the Retired | III | Robots waiting for permission to exist. The public assembly or the protected examination. |
| R13 | ALLGUT – Reception of Pleasantness | IV | Apparent choices lead to the same result. |
| R14 | Tribunal of the Actual Agreement | IV | Reconstruct the agreement that actually existed, not the approved one. |
| R15 | The Core – A No with a Line | IV | The physical withdrawal interlock and three prepared archive protocols. |
| R16 | Shared Observatory | IV | The ending. The same room composition is reused with changed props, occupants, infrastructure and dialogue. |

R05 and R06 come back during the travel, and R07, R09 and R12 each hold substantial route-specific
states. A branch changes the interaction and the staging, not only a line of dialogue.

There is no countdown, no death loop, no timed dialogue answer and no irreversible inventory mistake.
Nothing punishes taking a break.

## Music and Sound

Every room has its own generated instrumental song, sixteen in total, composed from a style prompt per
room and looped quietly under the dialogue (the runtime level is 0.15).

Sound starts muted and only after your first gesture. Every conversation appears as subtitles, so the
game is fully playable with the sound off. The settings panel in the menu holds **Turn sound on**,
**Reduced motion**, **Highlight interactive objects**, **Advance dialogue automatically** and the
**Reading speed**.

## Saves and Settings

Saves live in the browser, in IndexedDB, on this profile and this address. Nothing is written to the
AuraGo server and nothing is synchronised.

There are three manual slots, one automatic slot, four chapter slots (the beginning of each act), three
branch slots (before the customs decision, before Bruno's choice of path, before the decision in the
robot hall) and one slot before the final protocol. Each slot keeps its last valid version as a
backup, so **Load previous version** brings the earlier state back.

Export and import give you a portable JSON backup. Clearing the browser data removes the local saves;
the export is your way out of that.

All of them together make twelve slots. The total payload is capped at 4 MiB; when the storage is full,
the game says so and keeps your journey in memory so you can export it.

## When Something Goes Wrong

| Problem | What helps |
|---------|------------|
| The game window stays empty | The images for a room could not be loaded. The game says "The images for this room could not be loaded." and offers **Try again**. |
| "The journey could not be loaded. Your saves are safe." | The server did not answer as expected. Reload the Virtual Desktop and check that AuraGo is running. |
| "The selected game language is not yet available." | The game language follows the desktop language. The story is translated into all sixteen project locales; if the chosen one is missing, the game falls back to German. |
| A click does nothing | Click the hotspot once and wait: R0ST walks to it first, then acts. |
| R0ST refuses an action | He says the object does not fit there. Take the item, or use a different verb. |
| The scene does not change | A story queue is still running. Click or press Space to skip the current line, or Escape to cancel the rest of the queue. |
| The puzzle menu does not open | A puzzle prompt belongs to a specific action in a room. Look for the object in the room and use the right verb. |
| Nothing is saved | The storage may be full or blocked. The game keeps your journey in memory and offers the export. |
| "Another journey has changed this slot in the meantime." | Load the saved journey, or deliberately save yours in another slot. |
| The music does not play | The sound is off by default and needs a gesture. Turn it on in the settings. The songs are quiet by design. |
| A room has no music | Every room names its own song in the manifest. If that file is missing, the room is silent; the other rooms still play. |
| A character does not appear | Some characters are represented by a display or a sound rather than a walking figure: Frau Fracht is the ship's bridge. |
| The game is too slow on a big screen | The scene is drawn in the browser. Reduced motion removes camera movement, flashes and ornamental loops and keeps the key poses. |
| The right click opens a context menu | The browser context menu takes the right click on some pages. Use Shift+Enter instead, or hover and press the verb **Look**. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 33: Aura Drift](./33-aura-drift.md) · [Chapter 36: Synth Studio](./36-synth-studio.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 34: Storymaker](./34-storymaker.md)
**Next Chapter:** [Chapter 36: Synth Studio](./36-synth-studio.md)
