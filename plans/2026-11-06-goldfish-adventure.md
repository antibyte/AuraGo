# Goldfish Adventure design and implementation plan

Date: 2026-11-06. Working title: **Bitte nicht das Universum füttern**.
App ID: `goldfish-adventure`. Default game language: German.
Status: implemented locally; automated integration and localization checks pass.
Human duration, replay, editorial, listening and hardware acceptance remain open.

## Implemented delivery

- Built-in AuraGo Desktop game in the Games category, using the existing shell,
  lazy loading and native browser facilities. No new engine or backend service.
- Sixteen playable rooms, eighteen authored puzzle situations (fifteen per route),
  130 authored actions, three binary branches and three informed endings.
  The state checks execute all 24 route/ending combinations and 1,608 transitions.
- The German source contains 1,429 story keys and 16,441 whitespace-delimited words;
  interface text uses a separate 107-key section. All sixteen locales are complete.
  The clue ledger references 61 actual inspections/prerequisite dialogue lines.
- Sixteen original ImageGen background plates at their native 1672 × 941 size,
  one changed gate plate, eight source atlases, twenty inventory cutouts, the app
  icon and decorative dialogue frame. Ten sprite definitions use 108 distinct
  character/monitor/aquarium poses, 56 clip definitions and 14 authored moments.
  The original 90–120 clip estimate has not been reached and is not claimed.
- Eight original chamber-score loops, local ambience and four effects, with
  source notices, reproducible composition inputs and measured audio levels.
  There is no voice cast. Sound is initially muted and gesture-gated.
- Three manual saves, autosave, four chapter checkpoints, three branch checkpoints
  and a pre-final checkpoint use transactional IndexedDB revisions with a prior
  copy. Export/import provides portable JSON backups; this browser profile and
  origin own local saves. Nothing synchronizes to a server.
- Click or keyboard-focus a hotspot, then choose its action; select an inventory
  item before using it. Tab/Enter operate native controls. Space pauses when focus
  is outside a button/input; Escape skips a cinematic or dismisses the selection.
  The journal, optional hotspot labels and three-stage hints are available in-game.
  **Superseded by `plans/2026-11-07-goldfish-lucasarts-controls.md`:** the SCUMM-style
  bar (left click = default action, right click = look, verbs, inventory) replaces
  the sidebar/action buttons; Space skips an active line, Escape skips speech, then
  cancels an armed verb/item or choice menu, then opens the game menu; journal,
  hints, saves and settings live in that menu.

The final room captures and every authored moment render in real Chrome. Both
Desktop themes, the opening puzzle, close/reopen persistence, all route reducers,
save conflicts/failures and animation pause/skip/disposal have focused checks.
Image source hashes, alpha, frame bounds and local paths are checked separately.
Current art/audio is approximately 78 MiB; R01 image transfer is 14,002,500 bytes and
the largest room's decoded images occupy 44,044,848 bytes. These figures exclude
the existing Desktop, canvas/browser overhead and locale/script transfer.
The packaged game runtime is 85,108,023 bytes (art/audio, scripts, CSS and all locales),
within the 180 MiB target. Source masters are excluded from that runtime payload.

Remaining human acceptance is explicit: six blind first runs, three replay runs,
native-language editing, soundtrack listening and the declared ten-minute GPU
frame-pacing measurement. Automated solving does not establish 3–4 hours of play,
35–50 minutes of distinct replay content, comedy quality or final animation polish.
No service restart, deployment or publication is part of this implementation.

## Product decision

Build an original, premium, hand-painted 2D point-and-click adventure inside the
AuraGo Desktop. Target a first playthrough of **180–240 minutes**, including
exploration and dialogue, with meaningful alternatives during the story.
The creative references are the expressive staging and comic puzzle craft of
*Day of the Tentacle* and *Monkey Island*, plus the cosmic absurdity of
*The Hitchhiker's Guide to the Galaxy*. Original characters, locations and writing
make the game understandable without recognizing those references.

The strongest production route is one fully finished **20–25 minute opening
slice**, followed by the complete four-act game using the proven art/runtime
pipeline. This is a quality gate, not a reduction of the requested final scope.
A still image or a short prototype cannot establish full-game quality or duration.

| Final content target | Bound |
| --- | --- |
| Acts and first-run duration | 4; 180–240 minutes |
| Base room compositions | 16; changed states reuse those compositions |
| Authored puzzle situations | 18; 15 on each route, each with several meaningful steps |
| Mid-game branches | 3 binary choices; 8 route combinations with explicit reconvergence |
| End protocols | 3; available from every route after shared preparations |
| Main party | R0ST, Bruno, Karla, Kiesel and Kuno |
| Other visual packages | ALLGUT plus up to 4 supporting NPC designs |
| Inventory | About 24 unique objects, normally at most 10 carried |
| Dialogue and descriptive copy | Initial ceiling 12,000–16,000 German source words across all branches; re-estimate after slice |
| Short animation clips | Initial production estimate 90–120, plus 12 bespoke story moments; inventory before mass production |
| Localization | All 16 project locales, including story, hints, accessibility text and endings |

Counts and performance figures are proposed budgets, not measured results.
The act durations include their puzzles; they must not be added to the puzzle
durations again. Optional jokes are not a substitute for substantive playtime.

## Premise and dramatic question

R0ST, a discarded maintenance robot, wants his warranty reinstated. Instead he
rescues three furious goldfish, befriends a grieving ape and discovers that the
galaxy's celebrated happiness service is removing the right to disagree.

ALLGUT promises to end suffering. It removes painful memories, converts temporary
help into permanent intervention and edits complaints into compliments.
Satisfaction improves because citizens forget what happened to them.

The ultimate objective is physical and playable: **repair the machinery that
lets someone say “Nein.”** The questions arise through consequences: who defines
a functioning person, can care become coercion, and does remembering everything
necessarily serve the person who remembers?

## Characters

| Character | Comedy and agency | Personal arc |
| --- | --- | --- |
| R0ST | Literal-minded, observant repair robot with mismatched arms; offers practical help. | From seeking a certificate of usefulness to defending people's right to refuse help. One of many discarded dissenters, not a chosen one. |
| Bruno | Orangutan, horticultural engineer, dry humor, excellent practical judgment. | Wants a temporary relief contract cancelled after his sister Alma's death. He is allowed joy and sadness together; the game does not cure him. |
| Karla | Precise, severe goldfish who remembers the wording everyone else would prefer to forget. | Witness to the original authorization records; insists that consent be read accurately. |
| Kiesel | Tiny revolutionary goldfish furious about injustice, food and aquarium gravel. | Witness to transport records; learns that witnesses can also choose privacy. |
| Kuno | Round, curious goldfish with enormous confidence and some concealed uncertainty. | Remembers the withdrawal specification; wants control over which memories to revisit. |
| ALLGUT | Pleasant geometric screen-face with relentlessly helpful announcements. | Its coercion is an accountable policy choice; no secret tragedy or final speech excuses it. |
| Frau Fracht | Ship autopilot, represented by a bridge display and sound rather than another walking rig. | Learns that a manifest listing people as waste is itself incorrect. |

Supporting NPC art covers the bureau clerk, café proprietor, customs official and
promise-market trader. Repair-hall robots reuse justified mechanical parts and
silhouette variants rather than creating many fully independent rigs.
Frau Fracht uses the helm display with her own square cargo-label face; ALLGUT's
rounded smiling face appears on a separate public-announcement monitor. The
current lounge concept shows ALLGUT; it does not establish Frau Fracht's design.

The fish's archive tags hold complementary records; their testimony establishes
authenticity. The three-second-memory story is explicit in-world propaganda, not
a factual claim about real fish. Transport uses a wheeled, safely enclosed aquarium.
No fish distress or Bruno's grief is a punishment mechanic.

R0ST is the primary controlled walker. Companion actions are offered in context:
ask, explain the purpose, obtain agreement, then stage the joint action. Bruno
and the fish can refuse; they are never inventory objects or remote-controlled
tools. The party follows together; character switching must not triple travel.

## Four acts

| Act | Time | Goal, development and turning point |
| --- | --- | --- |
| I — Ihre Unzufriedenheit wurde bearbeitet | 45–60 min | Escape the scrapyard's administrative loop, investigate the feelings bureau and café, rescue the consenting fish. Their precise memories expose the educational lie. Bruno joins at the launch pier because he chose the cargo route home. |
| II — Eine Reise ohne Reisegrund | 40–50 min | Operate the ramshackle ferry, resolve asteroid customs through one of two methods, visit the abandoned-promise market. The ship carries supposedly deleted memories intact under a retention requirement. Bruno remains aboard, with quiet conversation and useful observations. |
| III — Was noch da ist | 55–75 min | Reach Bruno's arboretum, repair either its old walkway or its seed lift, use the storm station to reach the memory depot, then obtain maintenance authority at the repair hall. Bruno's one-week consent was converted into automatic renewal; R0ST and the other robots were discarded for refusing unauthorized rewrites. |
| IV — Wartung am Nein | 40–55 min | Enter ALLGUT's complex, reconstruct consent in the tribunal, reconnect the physical withdrawal interlock and enact an informed archive protocol. End in a shared observatory whose details reveal the consequences. |

No live countdown, death loop, timed dialogue answer or irreversible inventory
mistake extends playtime. Narrative urgency comes from approaching compulsory
updates; it never forces rapid reading or punishes taking a break.

### Sixteen locations

| ID | Scene | Playable and visual purpose |
| --- | --- | --- |
| R01 | Schrottplatz der zweiten Chancen | Crooked machines, useful discarded parts and a suspiciously intact protagonist. Tutorial and “uncooperative” evidence. |
| R02 | Amt für Gefühlsangelegenheiten | Immaculate counters above filthy service machinery; edited complaints and circular classification. |
| R03 | Café mit Aquarium | Warm clutter, unreliable official notices and three very reliable fish. Consent, safe transport and first reveal. |
| R04 | Startpier | Precarious port against a huge sky. Bruno boards, travel purpose and cargo contract become explicit. |
| R05 | Ship bridge and lounge | Recurring home, helm and destination window; Bruno's quiet scene and changed views on later visits. |
| R06 | Engine and freight compartment | Practical repair, manifest contradictions and the memory cargo revelation. |
| R07 | Tilted-asteroid customs | Public inspection machinery or a maintenance docking state, with different blocking and dialogue. |
| R08 | Market for abandoned promises | Luminous bazaar; obligations are tradable, repairable and sometimes best allowed to expire. |
| R09 | Bruno's arboretum | Beautiful maintained disorder and specific traces of Alma; old walkway or new lift state. |
| R10 | Storm-glass weather station | Huge panes of weather, tiny desk; horticultural knowledge becomes useful engineering. |
| R11 | Memory registry depot | Submerged archival shelves behind glass; recover access and discover repeated harms. |
| R12 | Retired-robot repair hall | Functional machines awaiting permission to exist; public assembly or protected blackbox examination. |
| R13 | ALLGUT reception | Opening-bureau shapes perfected into sterile reassurance; apparent choices lead to the same result. |
| R14 | Consent tribunal | Huge chamber designed for one approved answer; reconstruct the actual agreement and withdrawal. |
| R15 | Core and withdrawal interlock | Legible physical connections, neglected manual controls and three prepared archive protocols. |
| R16 | Common observatory | Quiet ending composition reused with changed props, occupants, infrastructure and dialogue. |

R05/R06 recur during travel; R07, R09 and R12 each contain substantial route-specific
states. A branch must change interaction, staging and later acknowledgement, not
just replace a line of dialogue. No additional base room is required.

### Earned twists

| Reveal | Advance clues | Result |
| --- | --- | --- |
| Fish remember | Old debts, exact phrases and disagreements with a sponsored aquarium notice. | A dismissed species becomes credible testimony. |
| “Deleted” memories are retained | Disposal invoices also charge for indefinite storage; the ship has freight infrastructure rather than passenger service. | The voyage supplies an actionable recovery route. |
| Functional robots were discarded | R0ST's refusal code, intact scrap machinery and matching depot classifications. | Solidarity and maintenance authority, rather than a special origin story. |
| Refusal was deliberately disconnected | Cancelled cancellation requests, Bruno's dated contract, identical-choice interfaces and an altered wiring diagram. | Restore a physical safeguard; do not defeat ALLGUT through a magic paradox or persuade it into goodness. |

## Interaction and fair puzzle design

> **Superseded in part by `plans/2026-11-07-goldfish-lucasarts-controls.md`.** The
> first four bullets below (action buttons, explicit Inspect, touch action buttons)
> are replaced: left click runs the default action, right click / long press /
> Shift+Enter is "Ansehen", verbs and the inventory live in the bar under the scene,
> and puzzle prompts are conversation menus. The fair-puzzle rules stay in force.

- Left click moves or performs the clearly labelled contextual action; Inspect
  remains explicit. Right click is an optional shortcut, never required.
- Inventory supports click-to-select and click-to-use, plus optional dragging.
  Combine only objects with signposted physical or semantic relationships.
- A focusable DOM hotspot list, keyboard navigation and visible focus provide
  the same actions as the canvas. Touch gets explicit action buttons and large
  targets. Hotspot reveal is available from the start.
- Every required deduction has two independent clues (for example observation
  plus dialogue) and a three-step optional hint: nudge, relevant relationship,
  explicit next action. Hints are static, spoiler-scoped to current knowledge.
- Failure responses explain the relevant mismatch with a short authored joke.
  Do not require exhaustive “use everything on everything,” external knowledge,
  sound-only clues, color-only distinctions or memorizing a transient line.
- A concise journal records discoveries and current intentions. It summarizes
  what was learned, not undiscovered objectives or future endings.
- Skip text, replay a line, adjust text speed and subtitles, and skip already
  seen staging. Logical effects settle exactly once even if a scene is skipped.
- Reduced motion removes camera movement, flashes and ornamental loops while
  retaining comprehensible actions. Controls work with audio muted.
- Point-of-no-return travel explains unfinished optional activity and records
  an automatic checkpoint. No critical clue can be left permanently behind.

## Puzzle ledger and dependency contract

Each row is a complete situation with diagnosis, two to four linked actions and a
visible change. P01–P04 belong to Act I; P05/P06 plus P07–P08 to Act II;
P09/P10, P11–P12 and P13/P14 to Act III; P15–P18 to Act IV.

| ID / room | Prerequisite | Clues and actions | Durable result |
| --- | --- | --- | --- |
| P01 — Nützlicher Schrott / R01 | Start | Maintenance diagram and twitching grabber reveal a shared brake/door circuit. Stop the conveyor, repair the grabber contact with its existing clamp, use the magnet brake to extract the bolt and mechanically open the exit. The bolt stays in the door mechanism. | Retain `tool_clamp`; `p01_done`. A specific repair, no universal hacking power. |
| P02 — Widerruf ist kein Gefühl / R02 | P01 | R0ST examines his own dated maintenance authorization and the public procedure for correcting facts. Reveal the original date under the renewal slip, compare stamp and office clock, submit a provable contradiction that the mood counter cannot dismiss. The receipt is absurdly longer than R0ST. | `e_contract` (general terms and R0ST's own record), `case_transfer`, `p02_done`. Bruno's private record is not available here. |
| P03 — Drei Sekunden sind eine Schnittfassung / R03 | P02 | The fish recall different parts of a test; a café jingle is cut off in its advertised three-second recording. Safely bypass the filter without draining water, collect the fish's accounts with their consent, reconstruct and play the uncut recording using labelled segments. Prepare their existing travel aquarium; each fish agrees to travel. | `party_fish`, `e_fish_test`, `p03_done`. Aquarium remains safe and available. |
| P04 — Abfahrt trotz Vorschrift / R04 | `case_transfer`, `party_fish` | A fuel gauge contradicts visible flow; a labelled mechanical fallback remains. Isolate the faulty sensor, ask the fish to observe from their attached safe habitat, bleed trapped air and set the mechanical regulator. Bruno elects to board and voluntarily shares his dated cancellation receipt. | `ship_ready`, `party_bruno`, `e_bruno_cancel`, `p04_done`. Safety systems remain active. |
| P05 — Ordentlich angemeldet / R07 public state | P04; choose public route | A pompous clerk assigns bureaucratic “weight” to apologies while standing on the wrongly zeroed inspection scale himself. His demonstration and the large calibration diagram independently expose the error. Observe which apology admits a concrete mistake, borrow official ballast through the correct courteous response, then demonstrate his own weight on the scale and negotiate a corrected public entry. Dialogue is observation-based; no imported insults or blind answer collection. | `customs_clear`, `route_customs=public`, `e_route_law`. Public recognition and a retention-law clue. |
| P06 — Vorgeschriebene Hintertür / R07 maintenance state | P04; choose maintenance route | Service pictograms, three pressure indicators and old docking rules expose a legitimate manual entrance. Equalize pressure, unload two interlocks in the diagram's explained order and dock manually. | `customs_clear`, `route_customs=maintenance`, `e_route_manifest`. Discreet entry and equivalent freight evidence. |
| P07 — Gelöscht, aber aufbewahrungspflichtig / R05–R06 | `customs_clear` | Freight invoices charge for disposal and indefinite storage. Match the chosen customs evidence to container labels, connect the ship's existing read head to the clearly read-only port and inspect an authorized diagnostic sample. | `e_retention`, `archive_locator`, `p07_done`. Record evidence/location, not an unrestricted copy of others' memories. |
| P08 — Reparaturen an Versprechen / R08 | P07 | A trader promised the robot vendor an unlimited break, the vendor promised a hatch repair before reopening, and a customer cannot leave until the hatch works. Posted promises and individual conversations expose the cycle. Combine an existing temporary hinge with a clearly labelled loan receipt, make the hatch usable, and negotiate a finite repair appointment plus a genuinely usable break that all three accept. Two participants reuse the party/robot visual kit rather than adding character packages. | `ship_patch`, `arboretum_access`, `p08_done`. Social listening and an absurd but physical inventory combination; no currency grind or repeated fetch loop. |
| P09 — Der vorhandene Weg / R09 upper state | P08; Bruno agrees | Footprints, surviving fixings and Bruno's recollection establish an old walkway. Reconstruct its shape, align surviving braces, secure it together and run the stated load test. | `weather_access`, `route_garden=old_path`. Upper walkway encounter and historical station data. |
| P10 — Ein anderer Weg / R09 lower state | P08; Bruno agrees | A seed lift's intact counterweight shaft and blocked irrigation return explain its failure. Clear the return, use controlled water mass as a counterweight, separate the plant guides and test with Bruno. | `weather_access`, `route_garden=new_path`. Workhouse/community-garden encounter and current station data. |
| P11 — Sonnenschein auf Befehl / R10 | `weather_access` | The “guaranteed sunshine” heater fools a sheltered rain sensor while trapping a storm. Bruno's storm-sensitive plant and the labelled condensation diagram independently expose it. An umbrella and detachable watering-can spout are present in R10 on both routes: combine them into a comic funnel over the isolated low-voltage rain sensor, then use the documented test mode to divert heat safely and open ventilation. Fish verify the float from their safe connected habitat; Bruno handles the delicate latch. | `depot_power`, `e_uncertainty`, `p11_done`. Botanical observation and visual prop comedy; phases advance by actions, never timing. |
| P12 — Widerspruch im Register / R11 | `depot_power`, `archive_locator`, `e_fish_test` | Register timestamps collapse three true statements into one allegedly impossible moment. Bells, lights and the jingle also have labelled phase markers and a replayable written transcript; no timing deduction depends on sound or color. Align originals, compare the uncut record from P03 and restore separate witness entries. | `e_provenance`, `repair_hall_access`, `p12_done`. Evidence of unauthorized intervention, not access to everyone's private memories. |
| P13 — Jede Stimme einzeln / R12 assembly state | P12; choose public witnesses | Three willing robots have distinct work records; an aggregated channel erases individual assent and refusal. Match testimony to work events, route the existing channels so individuals remain distinguishable and obtain each witness's confirmation. | `e_refusal`, `maintenance_authority`, `route_hall=assembly`. A lively assembly and later individual testimony. |
| P14 — Eigene Blackbox / R12 workshop state | P12; choose protected diagnosis | R0ST's own logger has a broken connection, visible write protection and test sockets. Protect it and provide independent power before bridging the connection; verify its record against voluntarily offered comparison measurements. | `e_refusal`, `maintenance_authority`, `route_hall=diagnosis`. A private forensic record and active repair workshop. |
| P15 — Das verschluckte Nein / R13 | `maintenance_authority`, `e_provenance`, `e_bruno_cancel` | Raw maintenance output and public transcripts disagree on every refusal. Demonstrate the filter harmlessly, expose the original return channel with legitimate authority, then let Bruno submit his own withdrawal. Trace why it ends at a disconnected line. | `e_charter`, `interlock_plan`, `tribunal_access`, `p15_done`. The missing alternative was deliberately suppressed. |
| P16 — Wer darf was entscheiden? / R14 | `e_contract`, `e_retention`, `e_refusal`, `e_charter` | Evidence has provenance and a tangible rights board shows read/custody/restore/delete. Demonstrate individual consent and effective withdrawal on sample data, then preview and prepare at least one end protocol. The purge option openly conflicts with others' chances of restitution; no unanimous-consent fiction conceals this. | `protocol_ready` contains keys, custody and/or purge; `p16_done`. Other protocols remain preparable locally without replaying earlier puzzles. |
| P17 — Nein muss eine Leitung haben / R15 | `interlock_plan`, P16 and a prepared protocol | Signal tracing shows that refusal lights an acknowledgement instead of stopping writes. Put essential services on independent power, repair the withdrawal relay, route actual write permission through it and test separate consent/withdrawal on empty media. | `forced_write_disabled`, `services_safe`, `p17_done`. ALLGUT loses a physical power. |
| P18 — Was bleibt? / R15 → R16 | P17 and chosen protocol ready | Read consequence previews and hear the party's objections. For keys: validate individual handover before removing the master key. For custody: separate the write/delete bus, test revocation and seal the control port. For purge: preserve nonpersonal service data, check the external-copy scope, open the guard and explicitly confirm the destruction lever. | `ending=keys\|custody\|purge`. Play a brief observatory aftermath with actual changed objects and conversations. |

The critical path is:

```text
P01 -> P02 -> P03 -> P04
                       -> (P05 XOR P06) -> P07 -> P08
                           -> (P09 XOR P10) -> P11 -> P12
                               -> (P13 XOR P14) -> P15 -> P16 -> P17 -> P18
```

All three alternatives expose their methods before commitment. Inspecting an
option does not select it. Save before commitment; commit the choice and its
result together. Later prerequisites use the common result flags; only staging,
dialogue and epilogue callbacks consult the route. Items from an exclusive route
must never become a later shared prerequisite.

Tool clamp, habitat and evidence copies persist. An installed reusable tool is
returned safely or becomes an equivalent permanent capability before travel.
No wrong combination consumes a critical item. Imported states must agree on
both the chosen route and its normalized result. A later full graph test must
check these invariants rather than merely replay the table mechanically.

## Replay and ending policy

Three mid-game route decisions reconverge after one distinct puzzle situation
each. That produces eight manageable route histories rather than eight separate
campaigns. The journal explains what the party actually did; later NPCs,
blocking, callbacks and epilogue props acknowledge those decisions.

For a second run, target **35–50 minutes of different authored interactions,
staging and dialogue**, to be measured rather than advertised in advance.
Changed jokes alone do not meet this target. Chapter replay starts from that
player's saved chapter state, makes a separate copy and preserves the original.
A fresh game reveals no future clues. Seen-dialogue skipping unlocks only for
lines actually encountered.

All three protocols can be prepared through the shared critical path, independent
of the three route choices. Players learn the options before R15, can examine
their consequences, and make an explicit final choice after stopping forced
rewrites. No invisible kindness score, “good joke” penalty or secret correct
sequence determines the ending.

| Protocol | Physical action and outcome | Visible cost |
| --- | --- | --- |
| Persönliche Schlüssel | Revoke central write authority, transfer individually controlled archive keys, shut intrusive central control down. People choose whether to restore their own memories. | Shared services and protection need rebuilding; some recovery takes time. |
| Begrenzte Verwahrung | Keep essential services and the archive in physically constrained custody with individually revocable access and independent oversight. | Central dependency and the work of oversight remain. ALLGUT is confined, not forgiven. |
| Der leere Himmel | After an explicit explanation and confirmation, destroy the external archive and shut ALLGUT down. Living memories, characters and fish survive. | Other people permanently lose opportunities for restitution. The epilogue openly confronts that harm; destruction is not sold as a secretly perfect liberation. |

Use an automatic pre-final save. An ending's irreversibility belongs to the
fiction, not to deletion of the player's other saves. No ending resurrects Alma,
forces Bruno to restore a memory or requires forgiveness.

## Humor, allusions and emotional rhythm

References reward familiarity without supplying required answers. A small number
of recurring jokes should evolve across the game rather than quote a catalogue.

| Reference tradition | Original transformation |
| --- | --- |
| Monkey Island's conversational combat | P05 customs staff duel through catastrophically specific apologies; win by observing an actual mistake and correcting it, not collecting imported insults. This is part of the existing route puzzle, not an added situation. |
| Day of the Tentacle's causal comedy | Official classifications change machinery and how NPCs treat the party; a “nonfunctional” machine performs a very functional escape. No copied characters or time-travel plot. |
| Space Quest's low-status hero | Maintenance knowledge repeatedly outperforms grand declarations; R0ST still gets billed for the repair. |
| Sam & Max's social absurdity | Earnest animal witnesses are more reasonable than institutions; Kiesel debates the political legitimacy of gravel. |
| Loom's expressive sound | The weather station has a readable labelled rhythm as well as audio; no musical expertise required. |
| The Hitchhiker's Guide to the Galaxy | Cosmic bureaucracy, a guide that confidently gives contextually useless advice, a towel with a mundane practical use and a single understated 42 joke. No lifted passages, characters or answers required to solve a puzzle. |
| Adventure interface conventions | An optional “pick up the horizon” response and an inventory-capacity joke acknowledge the genre without breaking puzzle logic. |

Proposed German dialogue:

> **R0ST:** „Ich möchte Widerspruch einlegen.“
> **Sachbearbeitung:** „Gegen Deine Einstufung als widerspenstig?“
> **R0ST:** „Ja.“
> **Sachbearbeitung:** „Das macht es natürlich schwierig.“

> **Karla:** „Drei Sekunden Gedächtnis? Ich erinnere mich noch an die Einführung dieser Lüge.“
> **Kiesel:** „Ich war dagegen.“
> **Kuno:** „Du bist gegen die Kiesfarbe.“
> **Kiesel:** „Zu Recht.“

> **Händler:** „Ein lebenslanges Versprechen. Kaum benutzt.“
> **R0ST:** „Wessen Leben?“
> **Händler:** „Deshalb der günstige Preis.“

> **ALLGUT:** „Seit der letzten Anpassung hat sich niemand mehr über Einsamkeit beschwert.“
> **R0ST:** „Hast Du jemanden besucht?“
> **ALLGUT:** „Das war nicht erforderlich.“

A quiet scene aboard R05 uses restrained acting and space between lines:

> **Bruno:** „Alle fragen, wann es besser wird.“
> **R0ST:** „Und?“
> **Bruno:** „Ich weiß nicht, was sie damit meinen.“
> R0ST waits.
> **R0ST:** „Soll ich etwas reparieren?“
> **Bruno:** „Den Stuhl neben mir. Der wackelt.“
> R0ST wedges the chair and sits beside him.
> **Bruno:** „Alma hätte den Wackelstuhl behalten.“
> R0ST reaches toward the wedge.
> **Bruno:** „Lass. So ist gut.“

The sequence is optional companionship, not a timed therapy puzzle or a hidden
trust requirement. Humor targets institutions, vanity and machinery; emotional
scenes get enough silence to register.

## Art direction and ImageGen production

The visual target is expressive hand-painted HD cartoon art: crooked architecture,
strong silhouettes, deliberate perspective, warm ochre/copper against petrol and
indigo, selective surface texture and readable character acting. Preserve the
adventure references' clarity and elasticity; avoid flattening the game into
generic glossy illustration or making all locations look like the same ship.

Each location family gets a distinct palette and shape language: bureaucratic
mint/cream rectilinear space, warm crooked ship interiors, luminous violet market,
green botanical overgrowth, icy archival depth and sterile gold-white ALLGUT.
The party's design stays consistent under those lighting changes.

All original visual imagery comes through **ImageGen**: backgrounds, detachable
props, character sprites, expressions, inventory art, decorative UI frames,
cursors, launcher artwork and illustrated ending elements. Runtime composition,
masking, placement, deformation and timing are normal animation work. Readable
text uses the project's local fonts and real localized text; never bake dialogue,
labels or puzzle answers into generated pixels.

### Current concepts

- [Ship lounge visual target](../assets/goldfish-adventure/concepts/ship-lounge-v1.png):
  1672 × 941, opaque. This is an assembled art-direction scene with characters
  already painted in, not a usable empty background.
- [R0ST sprite study](../assets/goldfish-adventure/concepts/r0st-sprite-v1.png):
  1122 × 1402, RGBA with transparent background. One pose, not a rig or animation.
- [Exact prompts and visual review](../assets/goldfish-adventure/prompts.md) and
  [metadata and hashes](../assets/goldfish-adventure/concepts/manifest.json).

Both were generated with the built-in image tool. Its result did not expose a
provider/model ID; this plan does not claim they came from Google's Imagen.
They are proposals, not user-approved production references or native 4K art.

Terra's visual review found strong color, silhouettes and travel atmosphere.
Before animation, lock R0ST's head/chest/hand proportions across references,
author the opposite-facing views, complete hidden joint surfaces, and reduce
fine detail where it competes with hotspots or shimmers at gameplay size.

### Required production sequence

1. **Lock reference sheets.** For R0ST and Bruno: front, back, both profiles and
   both three-quarter views, scale grid, expression range and characteristic
   gestures. Keep the robot's asymmetric arms; do not mirror them into the wrong
   identity. Give each fish a distinct silhouette, movement and eye/mouth system.
2. **Prove one room.** Generate an empty background, foreground/occlusion layers,
   detached interactive props and the necessary actors using the reference sheets.
   Fix camera, perspective, lighting, scale and walkable ground before animations.
3. **Author motion.** Generate matching parts and deliberate key poses from the
   reference, complete concealed areas and assemble rigs or frame sequences.
   Use authored anticipation, holds, contact, weight, follow-through and eye
   movement. Image generation alone does not provide coherent animation.
4. **Validate at gameplay size.** Review single frames and full-speed loops,
   turning, reaching, sitting, talking, interacting and overlapping scenery.
   Reject changing faces, limb count, sliding feet, popping proportions,
   holes at joints, alpha fringes and an idle bob substituted for acting.
5. **Export predictably.** Store approved masters separately, pack curated parts
   into atlases with explicit origin/pivot/trim/frame/duration/occlusion metadata.
   Keep source-to-export hashes and a reproducible exporter. PNG for lossless
   masters/alpha as needed, WebP for suitable delivery assets after visual review.
6. **Scale by act.** Produce the remaining rooms and clips in bounded batches.
   Review contacts and continuity across room boundaries before integration;
   re-run references through the same approval gate when designs change.

Preferred staging is 1920 × 1080 with UI outside critical scene content and
letterboxing as needed. Native 4K masters are a later production target where
the generation route supports them; upscaling is never labelled native detail.
Do not stretch the room to fill every aspect ratio.

Animation rendering targets 60 Hz, with authored pose timing typically 12–24
unique drawings/poses per second where appropriate. Smooth camera/rig interpolation
does not require a fresh generated bitmap for every render frame. Comedy and
Bruno's stillness need individual timing rather than a uniform loop speed.

Initial asset worksheet: 16 room masters, approximately 3–5 useful layers per
room, up to 12 substantial route/state overlay sets, up to 8 detail views,
10 character/display packages, about 24 inventory illustrations and a compact
original interface/icon set. R0ST/Bruno receive most of the 90–120 short clips;
secondary characters share intentional pose systems. Track clip purpose,
character, facing, duration and reused parts before fixing the count.

Twelve bespoke story moments include awakening, the fish reveal, safe aquarium
departure, boarding, first jump, customs-route payoff, cargo discovery, the chair
scene, garden-route payoff, robot-evidence reveal, interlock activation and the
chosen epilogue. They are short in-engine performances, not twelve pre-rendered
films. Every variant is budgeted explicitly.

### Audio and language

Plan original or properly licensed music/Foley: a small thematic score, distinct
room ambience, mechanical actions, ship sounds and fish/robot vocal gestures.
No borrowed franchise music. Text remains complete without sound.
Full cinematic dubbing, especially in sixteen languages, is not assumed; it would
need a separate production estimate after the script and acting style are stable.

German is authored first with real umlauts and Du. Translate meaning, joke setup,
punchline and clues into cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv,
zh. All branches, hints, subtitles, inventory nouns and accessible action labels
are in scope. Use context notes for translators; avoid grammatical puzzle answers
that work only in German. Do not fill other locales with English.

## Technical implementation in AuraGo

Use a built-in Desktop app with **Canvas 2D for the scene and semantic DOM for
controls, dialogue and accessible hotspots**. Existing platform facilities are
sufficient; begin without a new engine, 3D dependency, editor or backend service.

Content is authored data: stable scene/object/dialogue/puzzle IDs, walk regions,
hotspot geometry, prerequisites and explicit effects. A small deterministic state
reducer owns transitions; rendering never changes puzzle state. Commit an action's
logical effects once, then animate the presentation. Cancelled or skipped
animations cannot duplicate items or leave half-applied state.

Use named room anchors and authored walk polygons/waypoints for these sixteen
rooms. Do not build a general navigation framework or procedural world system.
Load the current act's content and needed room resources lazily.

### Existing integration points to change during implementation

These are current source anchors found in the read-only review; recheck the
owner chain and current worktree before editing them.

| Existing owner/path | Required integration |
| --- | --- |
| `internal/desktop/types.go` — `BuiltinApps` | Built-in `games` entry, `builtin://goldfish-adventure`, app-owned `logo_path`. |
| `ui/js/desktop/core/module-loader.js` | Lazy app scripts/styles and locale section. |
| `ui/js/desktop/core/desktop-foundation.js` | App/icon/global mappings; preserve explicit artwork priority across themes. |
| `ui/js/desktop/core/menus-and-routing.js` | Render dispatch and normal Desktop navigation. |
| `ui/js/desktop/core/window-shell-runtime.js` | Sensible initial bounds, focus and lifecycle integration. |
| `ui/js/desktop/core/session-runtime.js` | Review session restore policy; an automatic restore must not start sound or advance the game. |
| `ui/js/desktop/apps/` | Small named `goldfish-adventure*.js` modules for shell, state/content, rendering/animation and persistence as needed; follow existing line budgets. |
| `ui/css/desktop-app-goldfish-adventure.css` | Scope selectors with `agfish-`; readable controls under Standard and Fruity. |
| `ui/img/goldfish-adventure/` | Validated runtime images, atlas/scene metadata and local audio. Source masters remain under `assets/goldfish-adventure/`. |
| `ui/lang/desktop/*.json` | Launcher, interface and short runtime error text in all sixteen locales; large story bundles may use a separate lazy locale section through the existing loader. |
| `scripts/build-ui-bundles.js`, `assets/web-assets.json` | Rebuild shell bundles and verify packaged extensions/paths; include any new audio or locale-data formats explicitly. |
| `ui/*_test.go`, focused Node tests | Registry/lifecycle, content graph, save behavior, locales, assets and real browser journeys. |

Use `window.GoldfishAdventureApp.render/dispose` with per-window ownership.
Pause when minimized, hidden, in another Space, on an inactive tab or after auth
ends. Pause sound and timelines; disposing releases animation frames, listeners,
timers, media, decoded images and pending loads. Late asset callbacks must not
revive a closed or superseded scene. Opening the app focuses an existing game
window where the shell supports this; persistence must still handle another tab.

Desktop chrome follows the selected theme; the game's authored world retains its
palette. No external fonts, CDN, runtime LLM calls, provider key, Docker dependency,
new tool permission or configuration switch is needed. “Offline” means no cloud
or internet dependency while the AuraGo asset server is reachable; browser startup
after that server stops is not promised by this design.

### Saves and recovery

Use **IndexedDB transactions** for versioned JSON game state. The review rejected
a localStorage read/check/write revision scheme: it cannot reliably make concurrent
tab updates atomic. Reuse an existing suitable project persistence helper if one
meets this contract; otherwise use the browser facility directly.

- Persist story/schema version, slot revision, room, inventory, completed puzzles,
  mutually exclusive branch choices, knowledge/dialogue flags, character positions
  at safe checkpoints and user settings. Never serialize a canvas, live object,
  callback or unbounded event history.
- Three manual slots, one autosave, four chapter checkpoints, three pre-branch
  checkpoints and one pre-final checkpoint: twelve current states. Preserve a
  prior valid revision for recovery. Target at most 128 KiB per state and 4 MiB
  total retained payload, including old copies and metadata. Twenty-four maximum
  current/prior states use 3 MiB, leaving 1 MiB for bounded metadata. A later
  autosave never overwrites a named pre-branch or pre-final checkpoint.
- Validate IDs, enum values, counts and allowed combinations on load/import.
  Reject unknown future schemas without overwriting them; only explicit tested
  migrations may change old saves. Invalid data is never evaluated as code/HTML
  and never introduces asset URLs or paths.
- Read/check expected revision/write in one transaction. On conflict preserve
  both progress states and offer Reload or Save a copy. Do not silently choose
  a winner or claim localStorage-style optimistic checks solve this race.
  If all manual slots or the storage budget are full, keep the conflicting run
  in memory and offer Export; saving requires explicit selection of a free or
  replaceable manual target. Preserve stored revisions until that choice succeeds.
  Memory-only progress is not crash-safe before a successful save/export.
- Autosave after completed actions/scene changes and before irreversible story
  decisions. Report “saved” only after transaction completion. Storage failure
  keeps the current in-memory game and offers export with a visible unsaved state.
- Export/import versioned JSON using the normal browser file interaction.
  A failed import leaves existing slots intact. A new game never replaces
  a manual slot without explicit selection.
- Explain that saves belong to this browser profile and origin. Export is the
  portable backup; deleting browser data removes local saves. No cloud sync or
  server-backed recovery is implied.
- Transactionally save the completed logical puzzle boundary before cutscene
  playback; reopening resumes its safe aftermath without repeating rewards or
  item consumption. If the save fails, show the unsaved state and export option
  before continuing; never imply that an in-memory boundary is durable.

### Performance budgets to prove in the slice

| Measure | Proposed acceptance target |
| --- | --- |
| Room composition | 1920 × 1080 reference, readable UI at 1280 × 720; contain/letterbox smaller windows |
| Frame pacing | 60 fps target at 1080p on a declared integrated-GPU reference machine; acceptable stable 30 fps reduced-detail mode |
| Measurement | Ten-minute journey after warm-up, record p95 frame interval and long stalls; 60-mode p95 <= 20 ms, 30-mode <= 36 ms |
| Atlas sizing | Prefer 2048²; 4096² only after decoded-memory measurements |
| Decoded image budget | Aim <= 192 MiB for app-owned decoded imagery; separately report canvas, browser and GPU overhead |
| Loading | Current room and bounded next-room resources only; release old bitmaps and audio when no longer used |
| Browser first transfer | Aim <= 25 MiB excluding the existing Desktop shell; show loading/retry/cancel states |
| Complete new game payload | Initial target <= 180 MiB including runtime art, metadata, localization and audio; re-estimate from actual slice exports |

A 4096² RGBA surface alone is 64 MiB; compressed file size does not establish
runtime memory. Lazy loading reduces browser transfer and decoded memory, not the
size of AuraGo's versioned install/update archive. Measure that package separately.
Do not compromise character acting merely to hit an unproven compressed-size target.

## Production waves and ownership

The user's task-specific model assignment supersedes the older general Luna
preference: **Sol 6.1, Max** for design/implementation; **Terra 6.1** for review and
supporting tasks. No Terra reasoning level was specified.

| Wave | Sol 6.1 Max ownership | Terra 6.1 review/support | Exit evidence |
| --- | --- | --- | --- |
| 0 — design lock | Narrative, puzzle ledger, room roster, dialogue outline and branch outcomes. | Contradictions, reachable endings, scope, emotional treatment and implementation fit. | One reconciled graph, counts and art bible; estimates explicitly marked. |
| 1 — final-quality opening | First 20–25 min within R01–R03, R0ST/fish performance, one optional Bruno ship vignette for emotional/animation testing, interaction and saves. | Blind puzzle play, scene readability, image continuity, save interruption and performance. | Real playable slice; no placeholder art in the accepted portion. Extra vignette is separate from measured opening duration. |
| 2 — complete playable story | All 16 rooms, 18 puzzle situations/15 per run, three branch pairs, three endings and full German script. | Route/ending matrix, clue sufficiency, inventory safety and dialogue state. | Playable beginning to end on every route; temporary production art clearly tracked until replaced. |
| 3 — final content | Complete ImageGen-derived imagery/animation, sound, all sixteen translations and accessible controls. | Animation review, locale QA, visual/alpha/occlusion checks, full dependency/package checks. | Every visible asset final; all localized journeys work. |
| 4 — acceptance and integration | Focused fixes, resource packaging, guide updates and local commits. | Blind duration/replay tests, Desktop lifecycle/theme tests, final content and regression review. | Recorded acceptance results, known limitations and install/update payload size. |

Implementation workers own non-overlapping paths: runtime/state/save code;
content/dialogue; and asset production/export. Shared shell registration, locale
integration and conflict resolution belong to the coordinator. Every worker must
read the current DOX chain, preserve unrelated edits and inspect callers before
changing existing symbols. Keep no more than the available agent concurrency.

## Verification and release gates

### Logic and content

- Validate all scene/asset/dialogue/object references and reachable exits.
- Complete a clue matrix for every required deduction before slice acceptance:
  exact clue text/object, two independent discovery paths, prerequisites,
  repeatability, accessible equivalent and three hint levels. The current ledger
  establishes candidate clues, not verified coverage or observed player comprehension.
- Enumerate eight branch combinations and all three end protocols: **24 route/end
  journeys**. Check invariant knowledge and maintenance authority at foldbacks.
- Test required clue discovery in both orders, alternative rejection/cancellation,
  revisits and losing focus midway through each significant transition.
- Verify item creation/consumption together with state changes; required objects
  are retained, replaced or converted into an equivalent capability on every route.
- Every required hint is reachable and spoiler-scoped. References to other games
  are never prerequisites.

### Save and lifecycle

- Round-trip every chapter checkpoint and pre-final save; reject malformed,
  oversized, future-version and impossible-state imports without modifying slots.
- Exercise quota/unavailable storage, interrupted save/close, schema migration,
  simultaneous tabs, revision conflict and export/import.
- Re-enter each branch decision and verify that its named checkpoint changes
  only at the intended new commitment; failed writes retain the last valid copy.
- Browser journeys cover open, resize, minimize/restore, Space changes, close,
  reopen, session restore, slow/failed asset loads and audio unlocking.
- Test both Desktop themes, dark/light chrome, mouse, keyboard, touch, reduced
  motion, muted sound, long translated strings and relevant device pixel ratios.

### Art, localization and packaging

- Check actual image dimensions, alpha, atlas bounds, pivots, occlusion and
  source/export hashes. Test sprites at gameplay scale over light/dark scenes.
- Review full-speed animation and keyframes; no foot sliding, inconsistent faces,
  joint holes, clipped gestures, repeated stills advertised as animation or
  baked-in dialogue. Compare every delivery with the canonical reference sheet.
- Verify genuine translation and clue equivalence in all 16 locales, font coverage
  and no overflow; screenshot the longest strings and CJK layouts.
- Before code checks: UTF-8 without BOM, LF, `git diff --check`, `node --check`
  for changed JS, and JSON parsing for content/manifests.
- Reuse existing checks such as `TestDesktopAppAssetsRegistry`,
  `TestDesktopAppsExposeDisposeLifecycle`,
  `TestDesktopUsedI18nKeysExistInAllLanguages`,
  `TestShippedWebAssetsLoadNothingExternal` and
  `TestBuiltinAppsCarryStartMenuCategories`.
- Add focused game-state/save/route tests when the actual logic exists, plus a
  representative `AURAGO_RUN_BROWSER_SMOKE=1` browser journey.
- Rebuild Desktop bundles, run their `--check`, package through `cmd/assetpack`
  and use a correctly version-bound backend for final asset validation.
  Run release scripts only in a separately authorized release task.

### Human acceptance

Use at least six first-time players spanning adventure experience, with no author
coaching. Record active playtime, hint usage, stalls, route and comprehension.
Target a median of 180–240 minutes with optional hints; report spread rather than
claiming every player takes the same time. Revise pacing if the target is missed,
without adding walking or repeated dialogue as padding.

Have at least three testers replay different branches and record the time spent
in genuinely different playable content; target 35–50 minutes. Obtain explicit
feedback on whether Bruno feels like a person, the jokes land without reference
knowledge, and the ending options and their costs were understood before choice.

Final graphics/animation acceptance requires a real playable scene and motion
review at intended size. The two concepts supplied with this plan cannot pass
that gate on their own.

## Scope boundaries and DOX closeout

The implementation registers the game and adds its local content, art, audio,
browser saves and checks. No full voice cast, procedural stories, multiplayer,
game editor or additional engine is included.

Root, UI, Desktop app/catalog and script guides route to
`assets/goldfish-adventure/AGENTS.md` and
`ui/js/desktop/apps/goldfish-adventure/AGENTS.md`. The existing asset-package
extension/directory contract already covers these resources. The presentation
audio owner remains unchanged because its source files and contracts are reused.

The current worktree contains unrelated work. Commit only this game's sources,
exports, translations, tests and exact shared-registration/guide hunks.
