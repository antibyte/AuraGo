# Storyboard animation reference

Use storyboard scenes for staged characters, dialogue and narrated explanations. Search/describe the current catalog for exact IDs and usage examples: three 3D characters (char.hoodie, char.professor, char.punk), twelve day/night environments and eighty props. Never invent assets or change a character's clothing with unsupported parameters. Old human IDs resolve saved stories to these pets; choose the canonical IDs for new casts. Models have continuous joints and localized mouth/expression morphs. The original Desktop pets stay unchanged.

## Stage and composition

- Plan one visual explanation per scene: a recognisable main object, then only the supporting objects needed to explain a process, relationship or comparison. Reuse matching catalog props before drawing new ones. Usually a focal asset and one or two supporting objects are enough; do not add unrelated decoration to reach a quota.
- Search concrete concepts and synonyms in English, one focused query at a time. Check the meaning of results, not just spelling. `suggested_props`, `direction` and the fill example are starting points; use the actual subject of this scene. Avoid repeating the same generic lock/cloud/gear or replacing every concept with a label.
- When a focused search and a synonym search find no suitable motif, read topic code and create it as SVG path data with `g.path`. The embedded SVG asset example combines catalog props with a custom illustration and time-based motion. Save and activate it through the documented code workflow. Keep speaking characters in storyboard scenes; use off-screen narration for a custom illustration scene. Do not sacrifice dialogue/lip-sync to add decoration, invent a catalog ID or claim a separate SVG file was imported.
- Scene fields: environment, variant day/night, speaker, actors, props, beats, camera, transition and narration. A camera starts with framing wide/medium/close and an optional actor target.
- Speech always uses local Phonon in de/en/es/fr/pt. Cast voices follow language and catalog gender (Hoodie/Professor male, Punk female), independently of chat TTS. Do not invent voice IDs or choose another provider.
- Set `speaker` to the on-stage actor's cast ID (for example `"speaker":"anna"`) whenever that character speaks the scene narration so their mouth lip-syncs the speech automatically; leave `speaker` as `"narrator"` only for off-screen voiceover.
- An actor uses its cast member ID; a prop uses its prop.* catalog ID plus a short instance id. Beat who/target/hide/highlight refer to these stage instance IDs, not character asset IDs.
- Positions are environment spot names. Never place two actors or an actor and a shown prop/text on the exact same spot (`spot_overlap`): put the actor at `"left"` or `"desk"` and the revealed prop or label at `"right"`, `"center"` or `"board"`. For a custom safe-area position in a tool call, use a string such as "60,40" (x/y percentages 0..100; actor y is the feet). The server converts it into a point. Use scale conservatively and props layer back/front deliberately.
- Use `previous_scene`, `next_scene`, `suggested_props` and `used_entries` from the fill context packet to keep actor positions consistent across scenes in the same environment and to vary `show` entry styles.
- In a film the agent makes (`style.compose` is `v2`), the player keeps actors and shown props above the caption band, enlarges the first cue-bound prop of a scene to about a quarter of the frame height, and moves the camera in slowly onto the speaker. A scene with its own camera (a framing other than wide, a camera target or a camera beat) gets no push-in. Medium framing keeps the feet above the caption band; close framing, camera targets and camera moves are not adjusted, so the caption may cover the feet there. Give a show beat `scale` (0.5 to 2) when a prop should have another size: an explicit scale always wins.
- Check 16:9, 9:16 and 1:1 composition: keep faces and important props within the safe area, text short and labels apart from faces. The check measures where actors, props, texts and the caption stand in every storyboard scene and reports what covers what, what is too small and what lies outside the frame; fix what it reports. Code scenes are not measured.

## Actions

| Beat do | Fields and purpose |
| --- | --- |
| enter / exit | enter needs who, from and to; exit needs who and to |
| walk | who, to; use the character walking cycle |
| move | who, to; move props or slide an element without walking |
| pose | who, pose; stand, sit, typing, pointing and other catalog poses |
| gesture | who, gesture, optional count; a short action that returns to the base pose |
| emote | who, expression; change facial expression |
| look | who, target (actor or camera); redirect attention and perspective |
| say | who, style speech/thought, text; dialogue with speech/bubble/lip-sync. Without text, use the current narration |
| show | item or text, at, style, entry pop/fade/slide/none, optional id, optional scale 0.5 to 2 for an item |
| hide | target; remove an existing stage instance |
| highlight | target, style pulse/outline/arrow; draw attention briefly |
| camera | op zoom/pan/focus/shake; target for focus/pan, amount for zoom/shake or horizontal pan offset in percent |
| sfx | sound (sfx.* catalog ID); align a catalog sound with the visual event |
| wait | seconds; leave room to understand what changed |

Never substitute move for a person's walk. Combine a meaningful gesture with a matching expression; do not repeat the same wave or entry in every scene. Breathing, blinking, rig articulation, ground contact and storyboard lip-sync are automatic. Perspective follows supported poses/look controls; do not invent a view field. Separate environment layers provide parallax during camera movement automatically.

## Timing

- Scene narration normally determines duration. To start a beat on a spoken word, give the beat cue_phrase: 1 to 60 characters copied from that scene's narration (case and punctuation do not matter, umlauts do). The Studio finds the first match, sets the marker and cue itself, and refuses a phrase it cannot find with the closest wording. Write narration as plain text. Markers like {1} that you see in a saved narration are the Studio's: keep them when you resend the narration.
- Without cue, actions follow the previous beat. with:"prev" starts together with the previous beat. delay offsets that start; duration controls the motion. Use count for repeated gestures.
- Eases: linear, ease-in, ease-out, ease-in-out, back-out, bounce-out. Use linear for constant travel, ease-in-out for camera/repositioning, back-out or bounce-out sparingly for emphasis. Text stops moving when it lands.
- In cast/revise, set_meta accepts style.rhythm calm/standard/lively and style.auto_sfx true/false. Rhythm changes default timings; explicit beat durations remain deliberate. Automatic sound effects require auto_sfx; music stays the user's choice in the app.
- Use fade/cut inside the same environment; wipe/cut when changing environment. Available transitions are cut, fade, slide-left and wipe.

Example scene payload (cast member anna must exist):

```json
{"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present","expression":"neutral"}],"narration":"Eine Verschlüsselung schützt Deine Daten.","beats":[{"do":"show","id":"lock","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Verschlüsselung"},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"emote","who":"anna","expression":"happy","with":"prev"},{"do":"highlight","target":"lock","style":"pulse","duration":0.8},{"do":"camera","op":"focus","target":"anna","duration":0.8,"ease":"ease-in-out"}]}
```

Use custom code when storyboard controls or catalog assets cannot express the explanation, for example a missing visual motif, repeated data-flow particles or a bespoke diagram. Read topic code before writing it. Diagrams, charts, icon cards, headlines and lists are show beats, not custom code: see Choosing the picture. Validate with storymaker_check and fix errors; do not claim unsupported effects or verified layout.

## Choosing the picture

One visual idea per scene. Pick it from what the narration says; the fill context may name one in direction.suggested_block.

- steps or a process: diagram flow. A loop: cycle. Parts of a system: hub (the first node is the centre) or stack (the first node is the bottom layer). A versus B, or before and after: compare (two nodes).
- numbers: chart bar (1-8 values), line (2-24), pie (2-6) or counter (one number).
- a thing or a concept: icon card, or a prop. The key sentence: headline. Up to 4 short lines: list.

Variety: never the same block type in three scenes in a row. In a film of 7 scenes make 1 to 3 scenes a full board (environment env.board, at full, no actor); the other scenes pair the actor with a prop or an icon card. When a block sits at board, the actor points at it: gesture point, target board, with prev. In 9:16 two blocks side by side are too narrow: show one at full or top.

Block beats (find an icon id with storymaker_catalog kind icon and one English word; node labels 1-3 words, headline at most 8 words):

```json
{"do":"show","diagram":{"layout":"flow","nodes":[{"id":"in","label":"Paket","icon":"icon.send"},{"id":"fw","label":"Firewall","icon":"icon.shield"},{"id":"net","label":"Netz"}]},"at":"board","entry":"draw","cue_phrase":"Firewall"}
{"do":"highlight","target":"diagram-1.fw","style":"pulse","cue_phrase":"Firewall"}
{"do":"show","chart":"bar","data":{"labels":["Alt","Neu"],"values":[40,90],"unit":"%"},"at":"full","entry":"fade"}
{"do":"show","icon":"icon.cpu","style":"card","text":"Prozessor","at":"board","entry":"draw"}
{"do":"show","text":"Regeln statt *Zufall*","style":"headline","at":"top","entry":"fade"}
{"do":"show","text":"Paket lesen\nRegel prüfen\nWeiterleiten","style":"list","at":"right","entry":"fade"}
```

Areas for at: full, left, right, top, center, board. Block ids are icon-<name>-1 (icon.cpu is icon-cpu-1), diagram-1, chart-1 and text-1; a node is highlighted as diagram-1.<node id>. Two worked 7-scene plans: actor and prop, flow at the board, actor and icon card, bar chart at full, actor and prop, compare at full, headline.
