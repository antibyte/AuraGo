# Restricted code-scene reference (free project)

This project has no catalog assets. Never call api.add. Every scene is a code scene that draws its whole picture itself, with shapes, text and the g drawing facade. The API is the same isolated renderer used by the editor; it grants no filesystem, network, DOM, PixiJS, GSAP or Three.js access. A code scene has no speaker and no lip-sync: the narrator speaks off-screen and the code draws the picture.

## Save workflow

1. Use an existing scene_id from the outline/current project. Call storymaker_story get_code for that scene; it returns source and a code rev, including the create-only rev when no source exists.
2. Call put_code with scene_id, expected_rev copied from get_code, and data containing only source. Source is limited to 8 KiB. Syntax/forbidden-API errors are returned without replacing the saved source or bundle. A stale rev requires a fresh get_code, never an unconditional overwrite.
3. After the source bundles, activate the scene with patch_scene and data {"kind":"code"} (put_scene with the same data also works in the fill phase). The narration stays. A source write alone does not activate a scene. Use the story rev from get for this scene write; code rev and story rev are different.
4. Existing code scenes need only put_code for source edits. Run storymaker_check. Preserve unrelated scenes and user code; a source larger than the agent limit must stay for manual app editing.

## Module and timing

Export exactly `export default function scene(api) { ... }`. It runs once to construct the scene; draw callbacks receive explicit scene seconds. api.duration(seconds) sets a positive length up to 600 seconds, also bounded by the project's export limit. Allow enough time for narration and the entry transition; do not cut speech. Omitting duration uses motion end plus a margin, so explicit duration is preferable.

Timings are `{start: seconds, dur: seconds, ease: name}`. The field is dur, not duration. api.cue(n) gives the time of an existing {n} narration marker; it includes the entry-transition offset and throws when the narration has no such marker. Eases: linear, ease-in, ease-out, ease-in-out, back-out, bounce-out. Use linear for constant travel and back-out or bounce-out sparingly for emphasis.

## Scene API

- api.text(text, "title" or "label", {at}) returns a handle with setText(text), which is a build-time update. at is a spot or {x,y} safe-area percentages. For time-varying text, draw g.text directly from t. Never call setText or handle/tween methods inside onFrame: they mutate retained scene state and break reverse seeking.
- api.shape("rect"/"circle"/"line", {at,w,h,fill,stroke}) creates a shape; w/h are design pixels (a circle takes its radius as w).
- Handles of api.shape and api.text support moveTo(position, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale,timing) and rotateTo(degrees,timing).
- api.tween(handle, {opacity,x,y,scale,rotation,width,height}, timing) animates numeric properties. timing.from optionally supplies starting values. Rotation is degrees; width/height are design pixels. Tween x/y use percentages of the full 1920x1080 design, so prefer moveTo for aspect-safe positions.
- api.camera.zoom(factor,timing), pan(position,timing), shake(amount,timing) control the camera. Keep movement restrained and text readable.
- api.random("fixed-seed") returns a seeded random function. Compute random constants during build; do not advance a generator in onFrame.
- api.onFrame(function(t,g){...}) draws above the scene using a frozen 2D facade. g supports save/restore, translate(x,y), rotate(degrees), scale(x,y), alpha(0..1), fill(color), stroke(color,width), rect, strokeRect, circle, line, path(svgPath,mode), and text(text,x,y,{size,weight,align,color}). Coordinates here are 1920x1080 design pixels, then cropped by the aspect/camera; keep important drawing in the central safe region.

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text and the drawing facade for every effect.

## Explainer blocks

Blocks are drawn by the same renderer as the storyboard beats. Prefer them to hand-drawn diagrams, charts, lists and headlines. Every call returns a handle with the handle methods above plus highlight(nodeId, timing), and takes the options {at, entry, start, dur}.

- at is full (center for an icon), left, right, top, center, board or {x, y} in safe-area percent. An unknown area throws a RangeError that lists the valid ones. board is the board area of the environment; on env.board it is the full frame. Elsewhere it falls back to the right half of the safe area in 16:9 and to the top in 9:16 and 1:1. In code scenes only env.board draws a board.
- entry is draw (icons and diagrams), pop, fade or none. start is a second or api.cue(n), default 0. dur defaults to the natural length of the block.
- A block that fails its limits throws a RangeError whose message names the fix. A scene has at most 32 blocks.

Calls:

- api.icon(id, {at, style, label, entry, start, dur}): id is an exact icon id such as icon.cpu; find it with storymaker_catalog operation search, kind icon and one English word. style is line, badge or card; the label shows on the card style.
- api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): layout is flow (2-5 nodes), cycle (3-6), hub (the first node in the centre, 2-6 around it), stack (2-5, first node at the bottom) or compare (2 columns). A node is {id, label, icon, points}: id is unique and made of lowercase letters, digits and dashes; label has at most 40 characters, keep it to 3 words; icon is optional; points (compare only) has up to 3 short lines. flow: true moves dots along the arrows. reveals holds one start second per node, for example [api.cue(1), api.cue(2)]; without it the nodes appear one after another over dur. highlight(nodeId, {start, dur}) pulses one node for 1.2 s.
- api.chart({type, labels, values, unit, title}, {at, entry, start, dur}): type is bar (1-8 values), line (2-24), pie (2-6, no negative values) or counter (exactly 1 value). labels and values have the same length; a label has at most 24 characters, unit 8, title 40.
- api.headline('One *marked* word', {at, entry, start, dur}): at most 80 characters; the word between asterisks is accented.
- api.list(['line', ...], {at, entry, start, dur, reveals}): 1-4 lines of at most 40 characters; reveals holds one start second per line.

Put a headline in top and a diagram or chart below it instead of one crowded block, and keep onFrame drawing for small accents on top of the blocks.

## Choosing the picture

One visual idea per scene. Pick it from what the narration says; the fill context may name one in suggested_block.

- steps or a process: api.diagram layout flow. A loop: cycle. Parts of a system: hub or stack. A versus B, or before and after: compare.
- numbers: api.chart bar, line or pie; counter for one number.
- a thing or a concept: api.icon with style card, or shapes you draw. The key sentence: api.headline.

Variety: never the same block type in three scenes in a row. In a film of 7 scenes make 1 to 3 scenes a full board (a diagram or chart at full); draw the others yourself. Node labels have 1-3 words, a headline at most 8 words.

Example, flow with a highlight on the second cue:

```javascript
const d = api.diagram({ layout: 'flow', nodes: [{ id: 'in', label: 'Paket' }, { id: 'fw', label: 'Firewall' }, { id: 'net', label: 'Netz' }] }, { at: 'full', reveals: [api.cue(1), api.cue(2), 3] });
d.highlight('fw', { start: api.cue(2) });
```

## Draw every motif yourself with g.path and shapes

1. Pick the one concrete motif of the scene from its narration and design that specific object, organism or process element, not a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette that the whole film shares. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Use api.shape for backdrops, bars and rules, and g.path for the main motif. onFrame draws above retained shapes and text, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key drawing inside api.safe() (the safe_box of the context), which is the visible part of the format above the captions. api.shape and api.text use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named: use `api.cue(n)` for existing markers and derive alpha, position or flow directly from t; use fixed times when the narration has no marker. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with patch_scene and {"kind":"code"}, then check.

The executable example below is one complete free scene; change its labels to the job language. Layout still needs visual review in the intended format; a successful check is not pixel-level approval.
