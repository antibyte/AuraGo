# Please Do Not Feed the Universe

**Please Do Not Feed the Universe** (`goldfish-adventure`, category `games`) is a first-party, offline
point-and-click adventure for the Virtual Desktop. The German working title is
**Bitte nicht das Universum füttern**. It runs in the browser with no provider, no account, no backend
service and no external request at runtime: the artwork, the story and the room music ship with AuraGo.

The design references are *Day of the Tentacle* and *Monkey Island* for expressive staging and comic
puzzle craft, and *The Hitchhiker's Guide to the Galaxy* for cosmic absurdity. Original characters,
locations and writing make the game readable without recognising them.

Status: implemented locally and verified with automated integration, localization and asset checks.
Human acceptance is still open: six blind first runs, three replay runs, native-language editing,
soundtrack listening and a frame-pacing measurement on target hardware are not done. The 3–4 hour
playtime target is a design goal, not a measured result.

## What the game is

R0ST, a discarded maintenance robot, wants his warranty reinstated. Instead he rescues three furious
goldfish, befriends a grieving ape and discovers that the galaxy's celebrated happiness service is
removing the right to disagree.

ALLGUT promises to end suffering. It removes painful memories, converts temporary help into permanent
intervention and edits complaints into compliments. Satisfaction improves because citizens forget what
happened to them. The playable objective is physical: **repair the machinery that lets someone say "No."**
The questions arrive through consequences: who defines a functioning person, can care become coercion, and
does remembering everything necessarily serve the person who remembers?

| Character | Role |
| --- | --- |
| R0ST | Literal-minded, observant repair robot with mismatched arms; the player-controlled walker. From seeking a certificate of usefulness to defending people's right to refuse help. |
| Bruno | Orangutan, horticultural engineer, dry humor, excellent practical judgment. Wants a temporary relief contract cancelled after his sister Alma's death. He is allowed joy and sadness together; the game does not cure him. |
| Karla | Precise, severe goldfish who remembers the wording everyone else would prefer to forget. Witness to the original authorization records. |
| Kiesel | Tiny revolutionary goldfish furious about injustice, food and aquarium gravel. Learns that witnesses can also choose privacy. |
| Kuno | Round, curious goldfish with enormous confidence and some concealed uncertainty. Wants control over which memories to revisit. |
| ALLGUT | Pleasant geometric screen-face with relentlessly helpful announcements. Its coercion is an accountable policy choice; no secret tragedy excuses it. |
| Frau Fracht | Ship autopilot, represented by a bridge display and sound rather than another walking rig. |

## Starting the game

Open **Please Do Not Feed the Universe** from the Desktop start menu (Games). The start screen offers
**New journey** and **Continue journey** when a saved journey exists. The game language follows the
desktop language and falls back to German; the story is translated into all sixteen project locales.

Every project is a folder in the desktop workspace. Saves live in the browser (IndexedDB), not on the
server, so nothing is synchronised and clearing browser data removes the local saves. Export and import
give a portable JSON backup.

## Controls

The runtime follows `plans/2026-11-07-goldfish-lucasarts-controls.md`, which superseded the original
sidebar interaction of `plans/2026-11-06-goldfish-adventure.md`. The bar under the scene holds the
sentence line, the four verbs and the inventory.

| Input | Result |
| --- | --- |
| Hover a hotspot, exit or item | The sentence line shows the command that would run. Idle: the room title. |
| Left click | R0ST walks to the hotspot's `walk` point, then runs the default action. Exactly one available action runs it; two or more open a choice list in the bar; none means look. If only item actions exist, R0ST says the item is needed. |
| Right click / long press (500 ms) / Shift+Enter | Ansehen (the hotspot's `look` lines). |
| Click a verb, then an object | Applies that verb to the next click, then the verb resets. No matching action gives an R0ST refusal line. |
| Click an inventory item | Arms "Benutze {item} mit …". The next click on a hotspot performs the action whose `item` is that item. Click the item again, Escape, or right-click the scene to disarm. Right click on an item shows its description. |
| Click on the floor | R0ST walks there. A click while walking retargets; a second click on the same target finishes the walk immediately. |
| Enter on a focused hotspot | Same as a left click. Tab order: hotspots, exits, verbs, items, menu. |
| Escape | Active speech: skip the queue. Otherwise: cancel an armed verb, item or open choice. Otherwise: open the menu. |
| Space | Active speech: skip the line. Otherwise: pause. |

The four verbs are Ansehen (look), Nehmen (take), Benutzen (use) and Sprechen (talk). **Nehmen** also
matches actions whose effects add items, otherwise it would have two hits in the 130 authored actions.

Dialogue runs by itself: there is no "Weiter" or "Überspringen" button and no text band. A line appears
as outlined pixel text in the speaker's colour above the speaker's head. Its duration is
`clamp(900 + chars × rate, 1600, 9000)` ms, with a rate of 70 for slow, 48 for normal and 32 for fast
reading. Story queues (action results, room entry, resume) block the scene until they end; ambient
queues (look lines, item descriptions, refusals, blocked exits) yield to any click.

Puzzle prompts are conversation menus in the bar, in three kinds: **choice** (one line per option),
**sequence** (steps in order, checked automatically after the last one, with a "one step back" line)
and **match** (pairs asked one after another, checked after the last pair). The correctness decision
stays in `GoldfishState.apply`; the menu only collects answers.

The menu (the ☰ button, or Escape when nothing else is active) holds the travel journal, a small hint,
the saved journeys, the speech log (the last 40 lines), the settings, pause and resume, plus the
current act, room and playtime. The settings dialog holds the sound switch, reduced motion, the
hotspot highlight, auto-advance and the reading speed. Auto-advance and reading speed live in
`localStorage` only, never in the save.

## The sixteen rooms

Sixteen base room compositions, each with its own background plate, its own song and its own set of
hotspots. R05 and R06 recur during the travel; R07, R09 and R12 each contain substantial route-specific
states. A branch changes interaction, staging and later acknowledgement, not just a line of dialogue.

| ID | Room | Act | Purpose |
| --- | --- | --- | --- |
| R01 | Junkyard of Second Chances | I | Crooked machines, useful discarded parts and a suspiciously intact protagonist. Tutorial and "uncooperative" evidence. |
| R02 | Office of Emotional Affairs | I | Immaculate counters above filthy service machinery; edited complaints and circular classification. |
| R03 | Café with Aquarium | I | Warm clutter, unreliable official notices and three very reliable fish. Consent, safe transport and first reveal. |
| R04 | Launch Pier under a Wide Sky | I | Precarious port against a huge sky. Bruno boards, travel purpose and cargo contract become explicit. |
| R05 | The Maybe Thursday – Bridge and Lounge | II | Recurring home, helm and destination window; Bruno's quiet scene and changed views on later visits. |
| R06 | Engine Room and Cargo Belly | II | Practical repair, manifest contradictions and the memory cargo revelation. |
| R07 | Crooked Customs on the Tilted Asteroid | II | Public inspection machinery or a maintenance docking state, with different blocking and dialogue. |
| R08 | Market of Abandoned Promises | II | Luminous bazaar; obligations are tradable, repairable and sometimes best allowed to expire. |
| R09 | Bruno's Arboretum | III | Beautiful maintained disorder and specific traces of Alma; old walkway or new lift state. |
| R10 | Weather Station under Storm Glass | III | Huge panes of weather, tiny desk; horticultural knowledge becomes useful engineering. |
| R11 | Register Depot behind Deep Glass | III | Submerged archival shelves behind glass; recover access and discover repeated harms. |
| R12 | Repair Hall of the Retired | III | Functional machines awaiting permission to exist; public assembly or protected blackbox examination. |
| R13 | ALLGUT – Reception of Pleasantness | IV | Opening-bureau shapes perfected into sterile reassurance; apparent choices lead to the same result. |
| R14 | Tribunal of the Actual Agreement | IV | Huge chamber designed for one approved answer; reconstruct the actual agreement and withdrawal. |
| R15 | The Core – A No with a Line | IV | Legible physical connections, neglected manual controls and three prepared archive protocols. |
| R16 | Shared Observatory | IV | Quiet ending composition reused with changed props, occupants, infrastructure and dialogue. |

The room titles are the shipped ones in `ui/lang/goldfish/en.json`; the German file carries the German
room titles under the same keys.

The shipped story has 18 authored puzzle situations (P01–P18, fifteen on each route), 130 authored
actions, three binary branch points and three informed endings, so eight route histories and 24
route-and-ending combinations. The state checks execute all of them.

| Protocol | Physical action and outcome |
| --- | --- |
| `keys` | Validate the individual handover before removing the master key. |
| `custody` | Separate the write/delete bus, test revocation and seal the control port. |
| `purge` | Preserve nonpersonal service data, check the external-copy scope, open the guard and confirm the destruction lever. |

All three protocols can be prepared through the shared critical path, independent of the route
choices. There is no invisible kindness score and no secret correct sequence.

## The music

Every room has one generated instrumental song, sixteen in total, named `song-r01` to `song-r16`. They
were produced locally with YuE2 Studio from the "Style (instrumental)" line of each prompt template in
`assets/goldfish-adventure/music-templates/`, and prepared by `build-songs.mjs`: trim digital silence,
bake a 3 s equal-power loop crossfade, scale the decoded file to a −8 dBFS peak and encode it as Ogg
Vorbis at about 104 kbps. The runtime loops the song of the current room at the manifest's `audioVolume`
of 0.15.

The prompt text and the SHA-256 of every source FLAC and every shipped Ogg are recorded in
`assets/goldfish-adventure/audio-manifest.json`. The FLAC masters are not committed and not shipped.

**Licence note.** `assets/goldfish-adventure/audio-notices.txt` records an open item: the licence and
usage terms of the YuE2 model and Studio for use in a distributed game **have not been verified**, and
no licence is claimed for these songs. The terms (model licence, output restrictions, attribution
duties) must be confirmed before a release ships them, or the songs must be replaced. The same file
retains the Earshot MIT terms, the reused recording terms and the CC0 notices for the third-party
samples.

## Asset and licence sources

- **Artwork.** All original, generated for this repository with the built-in ImageGen tool. No
  franchise artwork, no downloaded third-party artwork and no branded characters. Source images, the
  exact prompts and the reference relationships stay in `assets/goldfish-adventure/` (the concept,
  prompt and master records), and the production record lives in `assets/goldfish-adventure/AGENTS.md`.
- **Runtime payloads.** The shipped assets live under `ui/img/goldfish-adventure/`: sixteen room plates
  plus the changed R01 gate plate, seven character, actor and monitor atlases, twenty inventory cutouts
  and the app icon. The runtime manifest is `ui/img/goldfish-adventure/manifest.json` (version 1), with
  16 rooms, 10 sprite definitions, 14 authored moments and 3 portrait entries.
- **Sound.** Beyond the sixteen generated songs, the runtime plays four effect files from
  `ui/img/goldfish-adventure/audio/`: door-metal, ui-click, water-splash and electric-spark. The
  shipped notices name their sources — door-metal is Kenney Impact Sounds (CC0-1.0), the other three
  are original AuraGo synthesis (MIT) — and also list forest-day (isaiah658, Ambient Bird Sounds,
  CC0-1.0) and rain-light (Kresiek The Furry, amb rain loop 2, CC0-1.0) as reused local AuraGo
  ambience records. The shipped copy of the notices is
  `ui/img/goldfish-adventure/audio/audio-notices.txt`.
- **Strings.** Story text uses flat `goldfish.*` keys in `ui/lang/goldfish/<locale>.json` (16 locales);
  interface text uses a separate section in `ui/lang/goldfish-interface/<locale>.json` (16 locales).
  Both sets carry the same keys in every locale.
- **Own code and synthesised sound:** MIT. Third-party samples: CC0-1.0.

## Where the pipeline lives

| Concern | Location |
| --- | --- |
| Approved scope and content | `plans/2026-11-06-goldfish-adventure.md` |
| Approved control scheme | `plans/2026-11-07-goldfish-lucasarts-controls.md` |
| Production assets, prompts, manifests | `assets/goldfish-adventure/` |
| Room music prompt templates | `assets/goldfish-adventure/music-templates/` |
| Runtime modules | `ui/js/desktop/apps/goldfish-adventure/` (`content*.js`, `state.js`, `save.js`, `renderer.js`, `commands.js`, `speech.js`, `panel.js`, `interactions.js`, `app.js`) |
| Runtime contract | `ui/js/desktop/apps/goldfish-adventure/AGENTS.md` |
| Grid layouts, clips, placements, room audio | `assets/goldfish-adventure/art-layout.json` |
| Runtime index and payload limits | `ui/img/goldfish-adventure/manifest.json` |

## For developers

- **Asset build.** `./assets/goldfish-adventure/build-assets.ps1` on Windows with PowerShell 7 and
  System.Drawing. It exports unscaled PNG copies and cells plus frame pivots into
  `ui/img/goldfish-adventure/` and then writes the runtime manifest. Metadata records measured
  dimensions and source hashes.
- **Song build.** `node assets/goldfish-adventure/build-songs.mjs --sources <folder>` expects the FLAC
  masters `R01.flac` to `R16.flac` in that folder, needs FFmpeg on `PATH`, and writes the Ogg files and
  `audio-manifest.json`. The masters are not committed.
- **Checks.** `node scripts/verify-goldfish-assets.mjs` (PNG decoding and alpha, source hashes, paths,
  atlas bounds, staging and clip references, sixteen locale keysets, placeholders and the runtime
  payload limit), `node scripts/test-goldfish-animation.mjs` (pause, skip, replacement, disposal,
  reduced motion, per-frame sources and immutable gameplay rewards), `node scripts/test-goldfish-commands.mjs`
  (default action, verb filter, the Nehmen rule, the sentence line) and
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestGoldfish` (the real Desktop opening, the first
  puzzle, the themes and close/reopen behaviour).
- **Contracts.** `GoldfishContent` is declarative data with stable IDs and localization keys; no eval,
  no dynamic scripts, no remote assets and no model-generated runtime story. `GoldfishState` is the only
  writer of puzzle state. Saves use one IndexedDB transaction for the revision comparison and the write;
  twelve named current slots plus prior revisions, 128 KiB per state and 4 MiB total payload.
  Cinematic beats are visual consequences of already committed state: they cannot award or consume
  items, and every sequence pauses, skips and disposes safely. Reduced motion retains the key poses
  without the movement.
- **Budget.** The plan records a packaged game runtime of 85,108,023 bytes (art, audio, scripts, CSS and
  all locales) within a 180 MiB target, with art and audio at about 78 MiB. The production estimate of
  90–120 clips was never reached and is not claimed; the source layout holds 10 sprite definitions with
  56 clip definitions, and the shipped manifest holds 14 authored moments and 3 portrait entries.
- **Not established by the automated checks:** the 3–4 hour duration, the humour, the replay appeal
  (35–50 minutes of distinct replay content is a target) and the native-language quality.
