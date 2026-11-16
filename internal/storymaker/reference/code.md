# Restricted code-scene reference

Use this for custom diagrams and effects that need programmed drawing. Keep ordinary dialogue and speaking actors in storyboard scenes. The API is the same isolated renderer used by the editor; it grants no filesystem, network, DOM, PixiJS, GSAP or Three.js access. Character handles use the same 3D rigs as the storyboard; select char.hoodie, char.professor or char.punk from the catalog.

## Save workflow

1. Use an existing scene_id from the outline/current project. Call storymaker_story get_code for that scene; it returns source and a code rev, including the create-only rev when no source exists.
2. Call put_code with scene_id, expected_rev copied from get_code, and data containing only source. Source is limited to 8 KiB. Syntax/forbidden-API errors are returned without replacing the saved source or bundle. A stale rev requires a fresh get_code, never an unconditional overwrite.
3. To convert a storyboard scene during fill/revise, call put_scene with data.kind "code" after the source bundles. Leave narration out to preserve it; omit actors, props, beats and environment. A source write alone does not activate a scene. Use the story rev from get for this scene write; code rev and story rev are different.
4. Existing code scenes need only put_code for source edits. Run storymaker_check. Preserve unrelated scenes and user code; a source larger than the agent limit must stay for manual app editing.

## Module and timing

Export exactly `export default function scene(api) { ... }`. It runs once to construct the scene; draw callbacks receive explicit scene seconds. api.duration(seconds) sets a positive length up to 600 seconds, also bounded by the project's export limit. Allow enough time for narration and the entry transition; do not cut speech. Omitting duration uses motion end plus a margin, so explicit duration is preferable for custom effects.

Timings are `{start: seconds, dur: seconds, ease: name}`. The field is dur, not duration. api.cue(n) gives the time of an existing {n} narration marker; it includes the entry-transition offset. Available eases match the animation reference.

## Scene API

- api.add("char.*" or "prop.*", {at, scale, pose, facing, layer}) returns a handle. Use exact catalog asset IDs here, not cast member IDs. at is a spot or {x,y} safe-area percentages.
- Every handle supports moveTo(position, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale,timing), rotateTo(degrees,timing).
- A character also supports walkTo(position,timing), pose(name,timing), gesture(name,{start,dur,count,ease}), emote(name,timing) and look("camera" or position,timing). Use bare pose/gesture/expression names from usage examples, not pose.* IDs. Blink/idle/walk articulation is automatic. The code API has no say method or dialogue/lip-sync binding; use storyboard for speech by actors.
- api.text(text, "title"/"label"/other catalog text style, {at}) returns a handle with setText(text), which is a build-time update. For time-varying text, draw g.text directly from t. Never call setText or handle/tween methods inside onFrame: they mutate retained scene state and break reverse seeking.
- api.shape("rect"/"circle"/"line", {at,w,h,fill,stroke}) creates a shape; w/h are design pixels.
- api.tween(handle, {opacity,x,y,scale,rotation,width,height}, timing) animates numeric properties. timing.from optionally supplies starting values. Rotation is degrees; width/height are design pixels. Tween x/y use percentages of the full 1920x1080 design, so prefer moveTo for aspect-safe positions.
- api.camera.zoom(factor,timing), pan(position,timing), shake(amount,timing) control the camera. Keep movement restrained and text readable.
- api.random("fixed-seed") returns a seeded random function. Compute random constants during build; do not advance a generator in onFrame.
- api.onFrame(function(t,g){...}) draws above the scene using a frozen 2D facade. g supports save/restore, translate(x,y), rotate(degrees), scale(x,y), alpha(0..1), fill(color), stroke(color,width), rect, strokeRect, circle, line, path(svgPath,mode), and text(text,x,y,{size,weight,align,color}). Coordinates here are 1920x1080 design pixels, then cropped by the aspect/camera; keep important custom drawing in the central safe region.

Every frame must be a pure function of t and immutable constants. No counters incremented per draw, retained drawing state, Date/performance clocks, Math.random, timers, imports, global objects, document, fetch or raw canvas/context access. Balance save/restore and make drawing choices per call. Never bypass a sandbox or increase deadlines to fix slow code. Use bounded loops; reduce detail when a frame times out.

api.chart and api.diagram are placeholders and do not render. Advanced filters, shader/bloom/blur calls and raw sprite/rig attachments are not part of this API. Use shapes, text, catalog props and the drawing facade for custom effects.

## Create a missing SVG motif

1. Search the catalog for the concrete object and a synonym. Use an existing asset when it conveys the intended meaning. Spelling alternatives are not evidence that a motif is appropriate. For a missing device, organism or process element, design that specific motif rather than a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette suited to the scene. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size; avoid photorealistic claims for a small vector illustration. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Combine the new drawing with relevant `api.add` catalog props. `onFrame` draws above those props, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key custom drawing roughly inside x=710..1210, y=110..800 in design coordinates; leave the lower region for subtitles. Catalog handles use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named. In a narrated scene use `api.cue(n)` for existing markers, and derive alpha, position or flow directly from t; for silent examples use fixed times. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with put_scene kind code, then check. The paths remain editable in the saved scene source and use the same renderer for preview and export.

The following two executable examples show motion and SVG asset composition. The SVG example uses a custom charge controller with catalog solar-panel and battery props; it is a narrated illustration, not an on-screen speaking character. Change labels to the job language and IDs only to verified catalog IDs. Preserve existing dialogue scenes and user code during revisions. Code-scene layout still needs visual review in the intended format; a successful check is not pixel-level approval.
