# Storyboard animation reference

Use storyboard scenes for staged people, dialogue and narrated explanations. Search/describe the current catalog for exact IDs and usage examples: twelve fixed characters, twelve day/night environments and eighty props. Never invent assets or change a character's clothing with unsupported parameters.

## Stage and composition

- Scene fields: environment, variant day/night, speaker, actors, props, beats, camera, transition and narration. A camera starts with framing wide/medium/close and an optional actor target.
- Set `speaker` to the on-stage actor's cast ID (for example `"speaker":"anna"`) whenever that character speaks the scene narration so their mouth lip-syncs the speech automatically; leave `speaker` as `"narrator"` only for off-screen voiceover.
- An actor uses its cast member ID; a prop uses its prop.* catalog ID plus a short instance id. Beat who/target/hide/highlight refer to these stage instance IDs, not character asset IDs.
- Positions are environment spot names. Never place two actors or an actor and a shown prop/text on the exact same spot (`spot_overlap`): put the actor at `"left"` or `"desk"` and the revealed prop or label at `"right"`, `"center"` or `"board"`. For a custom safe-area position in a tool call, use a string such as "60,40" (x/y percentages 0..100; actor y is the feet). The server converts it into a point. Use scale conservatively and props layer back/front deliberately.
- Use `previous_scene`, `next_scene`, `suggested_props` and `used_entries` from the fill context packet to keep actor positions consistent across scenes in the same environment and to vary `show` entry styles.
- In a film the agent makes (`style.compose` is `v2`), the player keeps actors and shown props above the caption band, enlarges the first cue-bound prop of a scene to about a quarter of the frame height, and moves the camera in slowly onto the speaker. A scene with its own camera (a framing other than wide, a camera target or a camera beat) is left alone. Give a show beat `scale` (0.5 to 2) when a prop should have another size: an explicit scale always wins.
- Check 16:9, 9:16 and 1:1 composition: keep faces and important props within the safe area, text short and labels apart from faces. The check measures where actors, props, texts and the caption stand in every storyboard scene and reports what covers what, what is too small and what lies outside the frame; fix what it reports. Code scenes are not measured.

## Actions

| Beat do | Fields and purpose |
| --- | --- |
| enter / exit | enter needs who, from and to; exit needs who and to |
| walk | who, to; use planted walking for people |
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

Never substitute move for a person's walk. Combine a meaningful gesture with a matching expression; do not repeat the same wave or entry in every scene. Breathing, blinking, rig articulation, planted feet and storyboard lip-sync are automatic. Perspective follows supported poses/look controls; do not invent a view field. Separate environment layers provide parallax during camera movement automatically.

## Timing

- Scene narration normally determines duration. Mark a spoken word with {1}, then use cue:1 on its beat. A cue must exist in that scene's narration. Keep narration when putting a scene, or resend it with the markers inserted.
- Without cue, actions follow the previous beat. with:"prev" starts together with the previous beat. delay offsets that start; duration controls the motion. Use count for repeated gestures.
- Eases: linear, ease-in, ease-out, ease-in-out, back-out, bounce-out. Use linear for constant travel, ease-in-out for camera/repositioning, back-out or bounce-out sparingly for emphasis. Text stops moving when it lands.
- In cast/revise, set_meta accepts style.rhythm calm/standard/lively and style.auto_sfx true/false. Rhythm changes default timings; explicit beat durations remain deliberate. Automatic sound effects require auto_sfx; music stays the user's choice in the app.
- Use fade/cut inside the same environment; wipe/cut when changing environment. Available transitions are cut, fade, slide-left and wipe.

Example scene payload (cast member anna must exist):

```json
{"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present","expression":"neutral"}],"narration":"Eine {1}Verschlüsselung schützt Deine Daten.","beats":[{"do":"show","id":"lock","item":"prop.lock","at":"right","entry":"pop","cue":1},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"emote","who":"anna","expression":"happy","with":"prev"},{"do":"highlight","target":"lock","style":"pulse","duration":0.8},{"do":"camera","op":"focus","target":"anna","duration":0.8,"ease":"ease-in-out"}]}
```

Use custom code only when storyboard controls cannot express the explanation, for example repeated data-flow particles or a bespoke diagram. Read topic code before writing it. Charts/icons/diagrams that report not_supported_yet are not rendered: use actual props/text/shapes or a code scene instead. Validate with storymaker_check and fix errors; do not claim unsupported effects or verified layout.
