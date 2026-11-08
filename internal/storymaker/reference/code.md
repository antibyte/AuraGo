# Restricted code-scene reference

Use this for custom diagrams and effects that need programmed drawing. Keep ordinary dialogue and speaking actors in storyboard scenes. The API is the same isolated renderer used by the editor; it grants no filesystem, network, DOM, PixiJS or GSAP access.

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

The following executable example uses four moving data packets, a catalog character/prop, numeric tweens, a gesture and a gentle zoom. Change its labels to the job language and replace IDs only with catalog IDs.
