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

api.chart and api.diagram are placeholders and do not render. Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text and the drawing facade for every effect.

## Draw every motif yourself with g.path and shapes

1. Pick the one concrete motif of the scene from its narration and design that specific object, organism or process element, not a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette that the whole film shares. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Use api.shape for backdrops, bars and rules, and g.path for the main motif. onFrame draws above retained shapes and text, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key drawing roughly inside x=710..1210, y=110..800 in design coordinates; leave the lower region for subtitles. api.shape and api.text use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named: use `api.cue(n)` for existing markers and derive alpha, position or flow directly from t; use fixed times when the narration has no marker. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with patch_scene and {"kind":"code"}, then check.

The executable example below is one complete free scene; change its labels to the job language. Layout still needs visual review in the intended format; a successful check is not pixel-level approval.
