# System World beauty pass — Phase 6 backlog (finish)

Collected while phases 1–3 landed (2026-11-10). Each item names the evidence. Phase 6 gets its own spec
and plan; this file only keeps the findings so they are not lost.

## Look

1. **White glow on sunlit or lit ceramic.** The mission terminal's control tower still glows white at
   night (`reports/system-world-survey/phase-3/facade-missions-night.png`); it did so before the beauty
   pass. Source not isolated (a light near (56, 47)? the tower's own panes?). The memory archive's upper
   facade band shows a similar glow at night (`phase-3-first-look/p3-memory-night.png`). (done in phase 6:
   the environment light at night, `NIGHT_ENV` .2; a sun highlight on the archive's cornice by day remains.)
2. **Light bars and lintels that bloom.** Archive hall ceiling light bars, the mission terminal's central
   lintel, the jamb strips beside the agent door: brighter than the surrounding lobby at night. (partly done
   in phase 6: at night the same change closes it, lobbies at or below .002 clipped; by day the archive hall's
   light bars still bloom, so the day case stays open.)
3. **Window rooms by day** read light and empty in the middle of large facades
   (`window-polish/facade-agent-day.png`); more wall colour variety and furniture contrast by day. (done in
   phase 6: the glass's own environment intensity, wall tones, darker furniture and depth falloff.)
4. **Low tier windows at night** look matt and unlit (`phase-2-low/facade-agent-night.png`). (done in phase 6:
   measured, the lit share of the agent tower's panes at night is .63 on low against .67 on high, .78 against
   .775 before the window changes; no change needed.)
5. **A soft white blob** in one pane of `phase-2-medium/proof-window-close.png` was never identified. (done in
   phase 6: not reproduced; no clipped pixel in `proof-window-close` on medium.)
6. **Zebra stripes and kerbs** are bright by day; marking was toned down once (0.62 → 0.5), re-check. (done in
   phase 6: `crossing` and `kerb-close` show no clipped pixel; palette unchanged.)
7. **Plots** are plain dark slabs between pavement and buildings; phase 4 furnishes them, the leftover
   surface still needs a paving pattern. (done in phase 4: lawn and paving slabs over the purpose plots,
   with leaf and setts surface maps; `sysworld-scene.js`.)
8. **Aviation lights**: two or three seem to float beside skyline towers at night
   (`phase-2-swap/skyline-night.png`). (done in phase 6: the aviation lights stand on the roof-top centre.)
23. **Interior pot plants** (`interior_assets3.py` `pot_plant`) still use the lumpy `blob` and have no soft
   part. Give them a soft `foliage` part like the planter and the flower bed (`furniture_assets3.py`,
   `life_assets3.py`). (done in phase 6: broad-leaved pot plants with a soft `foliage` part.)
24. **Night glare.** The agent tower's lower window rows are close to white at night
   (`phase-4-look1/facade-agent-night.png`), and a lamp cone close to the camera veils the picture edge
   (`phase-4-look1/life-street-trees-night.png`). (done in phase 6, as item 1: lamp cones fade near the camera.)

## Behaviour

9. **Entering a building with the camera pitched up.** Focusing a district in street mode aims the camera
   at 40 % of the building's height, so a visitor who then walks inside looks at the ceiling
   (`reports/aurora/world2-lobby-agent.png`). Level the pitch gradually when the visitor walks, or when a
   lobby is entered; make the browser test's lobby screenshot look into the hall. (done in phase 6: the view
   levels while walking and on lobby entry; the expansion browser test checks that the lobby view looks into
   the hall.)
10. **Feet on stairs.** On the agent steps the trailing sole hangs about 10 cm above the tread in two
    stills and overhangs the top edge on the landing (`phase-3-polish/lobby-agent-step-*.png`). Snap the
    foot target to tread centres on stair flights. (done in phase 6: feet snap to the tread centres.)
11. **Doors and the stand-in swap** have only been seen in stills. Record a short sequence of a visitor
    and a resident passing a door, and of the swap at 45 m. (done in phase 6: sequences `seq-door-visitor`,
    `seq-door-resident` and `seq-swap-45`.)
12. **Residents' workload** is chaotic: small changes move completed jobs between 22 and 86 in the living
    test. The machine hall got no visit in the final 122 s run. Consider seeding or a calmer scheduler.
13. **Parcel sorter** stands 0.7 m beside the mission stairs (wanted 1.5 m); needs a smaller footprint or
    another spot.
14. **Patrol robots** yield up to 22 s behind trams in the living test (not asserted). (phase 6 found the
    hard case behind it: from about 115 s a patrol boxed in between the rails and street furniture and a tram
    waited for each other for good, and both trams stood still. A patrol that cannot step aside for 3 s now
    moves to the nearest free spot outside the rail corridor; `scripts/test-system-world-tram-pocket.mjs`.
    In 200 s the rule fires five times; watch whether that looks restless.)
26. **Patrol robots: stale `yield` label.** The `yield` label stays set while a patrol keeps moving (33–59 s
   of label time in the living test, no blockage). `sysworld-society.js` walks a patrol in the `yield`
   state too (the walking branch accepts it). Related to item 14. (done in phase 6: the label clears; in the
   living run all label time was genuine, so item 14 stays open.)
27. **Visitor dropped onto a resident.** Two overlapping bodies cannot step free of each other. The
   lobby-walk test found it and now waits for a free flight foot (`scripts/test-system-world-lobby-walk.mjs`).
   Check whether any runtime path can place the visitor on an occupied spot. (done in phase 6 for the ride
   exits (tram, drone, lift, scope and hover); not covered: the sky cab exit and `destination(id)`, which rely
   on their callers.)

## Performance

22. **Triangles at street level.** Furniture is instanced per design city-wide at the tier's level of detail
   and is never culled (the only hide is `NEAR` above 40 m). About 640,000 triangles are drawn even when the
   camera looks out to sea; street scenes went from about 455,000 to about 900,000 (`reports/system-world-survey/phase-4-t4`,
   `phase-4-look2`). Candidate: per-design near and far instance sets (LOD 0 near the camera, LOD 2 far),
   measured against a frame-time budget (item 19). (done in phase 6: near and far sets for the heavy designs;
   the street scene on high draws 627,936 view triangles, 882,938 before.)

## Tests and tooling

15. **Low-tier survey scenes for lobbies** fail by design: `lobby-agent-day` and `lobby-agent-night` wait
    for an interior that the low tier never shows. Make lobby scenes skip that wait on low. (done before
    phase 6: the lobby scenes skip the interior wait on low.)
16. **Kit test pin**: `scripts/test-system-world-v3.mjs` accepts "none or six" interiors; pin six and 31
    designs. (done in phase 6: the kit test pins six interiors and 46 designs.)
17. **Survey harness**: jumping the camera across the city needs the visitor body moved with it (done per
    scene in the lobby scenes); move it into the harness for every scene. Whole-run timeout is 300 s by
    default; the full set needs `--timeout 2400`. (done in phase 6: the visitor body follows the survey camera
    in every scene.)
18. **Browser report** has only the final `loadedBytes`; log the figure at first display as well. (done in
    phase 6: the expansion browser test logs the bytes at first display.)
19. **Frame time** has never been measured for the new windows, interiors and lamp pools; add a timing
    scene per tier and a budget. (done in phase 6: frame-time budget per tier and scene in
    `scripts/system-world-budget.json`. The first budget was wrong: the 2026-11-10 harness timed the overview
    for every scene, so it read a flat 10 ms; see the execution notes.)
20. **`build_textures.py --check`** needs Python 3.13 on this machine (PIL missing in 3.14).
21. **GitNexus** has no index for the worktree; run change detection against `main` before merging. (open until
    the merge preparation: change detection against `main`.)
25. **Survey harness: camera placement.** Cameras placed inside a tree canopy or beside the visitor body
   (`life-neon-night`, `life-plaza-day`) give pictures that do not show the place. Move the visitor body
   with the camera for every scene (item 17). (partly done in phase 6: the survey reports a camera inside
   geometry; the `blocked` flag is unreliable, see item 36.)

34. **Main-path browser run and the sound start.** In the phase 5 regression the run with
   `AURAGO_SYSTEM_WORLD_MATRIX=1` alone failed twice in the test agent's shell at the 10 s wait for
   `sound.state==='running'` with `rms>.0001` (`ui/desktop_sysworld_city_browser_test.go`, after the click on
   the sound button) and passed on the same commit (`cc9dad876`) in the coordinator's shell (106 s; six
   recordings loaded, foley peak .063). Cause not established: no audio device in that shell, or the wait is
   too short while the larger sound graph is built. Find out which; if it is time, wait longer for the state.
   (phase 6: not reproduced; the run passed five times in two other shells. The wait is unchanged.)

35. **Night glare steps down within one survey page.** After about the 11th picture in one page the facade
   glare drops by itself (missions .0245 → .0157, memory .0400 → .0337), with lights, emissive values, environment
   intensity and triangle counts unchanged (phase 6 Task 4, `reports/system-world-survey/phase-6-find-drift`).
   Something time-driven runs on between scenes (`freeze` only holds inside one). Find what; if it is a slow
   easing of the night state, the first minute after a mood change is brighter than the settled picture.
36. **Survey instruments.** The `window-env-off` toggle did nothing when it was used (the window material had
   no environment map of its own); since `windows.setEnvironment` it has one, so re-check the toggle.
   `camera.blocked` reports a moving body near the camera, not a bad camera position
   (`life-street-trees-night`, `crossing`): check against solids only, or freeze before the check.
37. **A visitor standing on the rails stops a tram for good.** The tram's look-ahead waits as long as the
   visitor's body is in its way (`sysworld-experience.js`); the tram behind queues. Meant as courtesy, but it
   never gives up: ring the bell and creep, or ask the visitor to step aside.
38. **Browser tests and a slow GPU.** Chrome may pick the integrated GPU; the System World then runs at a third
   of the frame rate. The expansion test's walking helper now holds keys over frames, but other fixed-time
   waits in the desktop browser tests have not been looked at.
39. **The tram model** is still the plain v2 one and looks out of place beside the v3 kit.

## Sound

28. **Tram and footsteps do not read as such.** Coordinator's classifier finding on the rendered mix: the tram
   pass sits 6.7 LU over its background, with mid-band rolling noise, and a footfall lasts about 140 ms.
   Candidates: CC0 recordings of a tram pass and of footsteps (needs the user's approval again), or richer
   synthesis. (done in phase 6: two CC0 recordings under the synthesis, `tram` and `steps`; the classifier
   hears a tram passing at z 3.3 and footsteps at z 3.4.)
29. **The `leaves` loop** reads as rain-like hiss to the classifier (coordinator's finding). Try a darker cut or
   another source (`assets/system-world/audio-sources.json`, `leaves`, high-pass 110 Hz). (done in phase 6: the
   leaves loop is cut from the calm stretch; a low-pass was tried and dropped.)
30. **Browsers that cannot decode Ogg Vorbis** get the synthesis only: the recording layer stays silent
   (`sysworld-sound-recordings.js`). A second format would cover them, if that matters.
31. **The street** reads as a quiet hum with steps (coordinator's finding). Its vehicle sounds are the traffic wash
   (`BEDS.traffic`) and the swoosh event; more tram and robot presence would add to it. (improved in phase 6, not
   re-judged: trams are heard to 120 m, hover robots to 45 m, and the recorded tram pass (it starts within 70 m)
   and the recorded footfalls are in the mix.)
32. **The gulls loop** is 21 s. Its cut starts at 72.6 s, and the source note says the stretches before 72 s and
   after 95 s may carry voices. A longer clean source would repeat less.
33. **Height is not localised** (no HRTF): overhead gulls only pan, because the mixer uses stereo panners and
   no `PannerNode` (plan risk 11).
