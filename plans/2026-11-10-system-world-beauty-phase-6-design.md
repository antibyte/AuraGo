# System World beauty pass — Phase 6 design (finish)

Phases 0–5 are implemented on `feat/sysworld-beauty`. This phase removes the defects that are still visible
or audible, makes the furnished city cheap to draw on weak hardware, proves in motion what was only seen in
stills, and ends with the acceptance record for the whole pass. Findings and evidence are in
`plans/2026-11-10-system-world-beauty-phase-6-backlog.md` (item numbers below refer to it).

Nothing is pushed or deployed in this phase. Merging into `main` is the user's decision at the end.

## Goals

1. Night pictures without white glare: light sources read as lamps and lit rooms, not as blown-out patches.
2. Windows keep their life by day and on the low tier.
3. Entering a building feels right: the camera looks into the hall, feet stand on the treads.
4. The furniture costs little when it is far away, and frame time has a measured budget per tier.
5. Motion is verified, not assumed: doors, the interior swap, leaf sway, banners, boats and gulls are captured
   as short sequences.
6. One acceptance record: before and after pictures, sound survey, budgets, test results.

## Work packages

### W1 — Night glare (items 1, 2, 24, 8)
- **Find first.** For each patch (mission control tower, memory archive upper band, agent tower lower window
  rows, archive light bars, mission lintel, agent door jambs) isolate the source by switching things off one
  at a time in a scratch harness: the material's emission, the window shader's light budget, a nearby light,
  bloom. Record the cause per patch before any fix.
- **Fix by cause.** Expected classes: an emissive kit material above the bloom threshold (lower the emission
  in the Blender palette or per part; coordinator's work), a window room whose night light exceeds the
  budget on large panes (cap in `sysworld-windows.js`), a real light too close to a pale wall (range or
  intensity), aviation lights whose position is not on a roof (place them from the tower's top bounds).
- **Lamp cone near the camera:** fade a lamp's cone and haze out when the camera is inside or within 3 m of
  the cone volume.
- **Acceptance:** in the night scenes `facade-missions-night`, `facade-agent-night`, `p3-memory-night`
  (add a survey scene for the archive front if none exists), `life-street-trees-night` and `skyline-night`
  no pane, wall or bar is clipped white over an area larger than a lamp head; measured as the share of
  pixels above 250/255 in all three channels inside the building's screen rectangle: at most 0.5 %.

### W2 — Windows by day and on low (items 3, 4, 5)
- **By day:** rooms get more wall colour variety and darker furniture, and the daylight fill falls off with
  room depth, so that large facades do not read as rows of pale boxes.
- **Low tier at night:** panes show a flat lit state (a warm or cool tint per room seed, no interior
  mapping), so the low tier's city is not dark at night.
- The unexplained soft white blob in one pane (item 5) is looked for with the W1 method; if it does not
  reproduce, the item is closed with that note.
- **Acceptance:** `facade-agent-day` and `window-polish` scenes show at least three distinguishable wall
  tones per facade and no pane brighter than the facade's sunlit ceramic; the low-tier night picture shows
  lit panes in the same share as the high tier (within 10 points).

### W3 — Entering a building (items 9, 10)
- **Camera:** when the visitor walks in street mode, the pitch eases towards level (time constant about
  1.5 s) unless the user is steering the pitch; entering a lobby does the same at once. The browser test's
  lobby screenshot must look into the hall.
- **Feet on stairs:** on a stair flight the foot target snaps to tread centres (the registry knows the
  treads), for residents and the visitor robot.
- **Acceptance:** the stair-step survey stills show both soles on treads within 2 cm; the lobby screenshot's
  horizon lies within the middle third of the picture.

### W4 — Furniture cost (items 22, 19)
- **Near and far sets.** Each furniture design keeps two instanced sets: pieces within 45 m of the camera
  (60 m to switch back) at the tier's level of detail, the rest at LOD 2. Sets are re-sorted at most twice a
  second and only when the camera moved more than 4 m. Trees keep their sway in both sets. The existing
  hide above 40 m camera height stays.
- **Budget.** `--timing` runs for `overview-day`, `street-agent`, `life-park-day` and `facade-agent-night` on
  every tier become part of the regression; the budget is the phase 5 median plus 10 % per scene and tier on
  the reference machine, and the triangle count at street level drops below 650,000 on the high tier.
- **Acceptance:** triangles and frame times reported before and after; no visible popping in a sequence that
  walks 80 m along a street (W5 captures it).

### W5 — Motion proof and survey harness (items 11, 17, 25, 18)
- **Sequences.** The survey gets a sequence mode: N frames at a fixed simulated interval from a fixed or
  moving camera, written as a contact sheet. Sequences: a visitor and a resident through a lobby door; the
  interior swap at 45 m; leaf sway and banners in a gust; the launch along the quay; gulls; a tram pass; a
  walk of 80 m along a street (for W4).
- **Harness:** the visitor body moves with the survey camera for every scene; cameras must not start inside
  a solid or a canopy (the harness reports it); the browser report logs the loaded bytes at first display.
- **Acceptance:** every sequence shows the motion it is for (the coordinator judges the sheets); no scene
  starts inside geometry.

### W6 — Small model and behaviour fixes (items 23, 26, 27, 6)
- Interior pot plants get a soft `foliage` part (Blender; coordinator).
- The patrol's `yield` label clears when it moves again.
- Check every runtime path that places the visitor (focus, fly-to, lobby entry, respawn) for an occupied
  target; use the traffic's free-spot search where one is missing.
- Zebra stripes and kerbs by day: re-check in the final pictures; tone the palette only if they clip.

### W7 — Sound finish (items 28, 29, 31, 32)
- **Tram and footsteps** do not read as such to the classifier. Two routes: (a) two more CC0 recordings
  (a tram pass-by, footsteps on a hard surface), which needs the user's approval for the concrete files as in
  phase 5; (b) without approval, richer synthesis (the tram's wheel-rail roar from several detuned bands and
  flange ticks, the footfall with a short body resonance per surface). The plan covers (b) and treats (a) as a
  separately approved task.
- **Leaves loop:** a darker cut of the same source (low-pass near 7 kHz, more of the gusty stretch) so that
  it reads less like rain; judged with the classifier.
- **Street presence:** hover robots and trams audible from further away on streets (reach and level), within
  the survey's checks.
- **Acceptance:** the four strict survey checks still pass; `find` on the rendered `tram-corner` and
  `walk-surfaces` reaches z ≥ 2.5 for "a tram passing" and "footsteps", or the route taken is recorded as
  not reaching it.

### W8 — Acceptance record
- Full picture survey on high, medium and low; a before/after sheet of the same scenes from the baseline of
  2026-11-09 and now; the sound survey; all test scripts; payload, bundle and audio checks; both browser
  runs; budgets (model directory, first display, whole app, audio); frame times.
- README and manual pictures of the System World are refreshed from the final survey where they show it.
- GitNexus change detection against `main`, a rebase onto the current `main`, and a short merge note for the
  user. No push, no deploy.

## Out of scope (stays in the backlog with a note)
- The parcel sorter beside the mission stairs (item 13): a smaller model or a rotation needs changes to the
  v2 living places; cosmetic.
- A calmer job scheduler for residents (item 12): the simulation passes its limits with margin since phase 4.
- A second audio format for browsers without Ogg Vorbis (item 30), a longer gull source (item 32) and height
  localisation (item 33).
- `build_textures.py --check` on Python 3.14 (item 20).

## Order
W1 (find) → W1 (fix) ∥ W2 ∥ W3 → W4 → W5 → W6 ∥ W7 → W8. W5's harness part can start with W1, because the
find step uses it.

## Rules carried over
- Blender authoring, pictures and sound are judged by the coordinator; code, tests and docs go through the
  agent roster; agents do not change git state.
- The resident simulation's limits and every existing test limit stay; a limit is never lowered to pass.
- New downloads only with the user's approval of the concrete files.
