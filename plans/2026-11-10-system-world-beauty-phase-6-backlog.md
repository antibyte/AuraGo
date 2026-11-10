# System World beauty pass — Phase 6 backlog (finish)

Collected while phases 1–3 landed (2026-11-10). Each item names the evidence. Phase 6 gets its own spec
and plan; this file only keeps the findings so they are not lost.

## Look

1. **White glow on sunlit or lit ceramic.** The mission terminal's control tower still glows white at
   night (`reports/system-world-survey/phase-3/facade-missions-night.png`); it did so before the beauty
   pass. Source not isolated (a light near (56, 47)? the tower's own panes?). The memory archive's upper
   facade band shows a similar glow at night (`phase-3-first-look/p3-memory-night.png`).
2. **Light bars and lintels that bloom.** Archive hall ceiling light bars, the mission terminal's central
   lintel, the jamb strips beside the agent door: brighter than the surrounding lobby at night.
3. **Window rooms by day** read light and empty in the middle of large facades
   (`window-polish/facade-agent-day.png`); more wall colour variety and furniture contrast by day.
4. **Low tier windows at night** look matt and unlit (`phase-2-low/facade-agent-night.png`).
5. **A soft white blob** in one pane of `phase-2-medium/proof-window-close.png` was never identified.
6. **Zebra stripes and kerbs** are bright by day; marking was toned down once (0.62 → 0.5), re-check.
7. **Plots** are plain dark slabs between pavement and buildings; phase 4 furnishes them, the leftover
   surface still needs a paving pattern.
8. **Aviation lights**: two or three seem to float beside skyline towers at night
   (`phase-2-swap/skyline-night.png`).

## Behaviour

9. **Entering a building with the camera pitched up.** Focusing a district in street mode aims the camera
   at 40 % of the building's height, so a visitor who then walks inside looks at the ceiling
   (`reports/aurora/world2-lobby-agent.png`). Level the pitch gradually when the visitor walks, or when a
   lobby is entered; make the browser test's lobby screenshot look into the hall.
10. **Feet on stairs.** On the agent steps the trailing sole hangs about 10 cm above the tread in two
    stills and overhangs the top edge on the landing (`phase-3-polish/lobby-agent-step-*.png`). Snap the
    foot target to tread centres on stair flights.
11. **Doors and the stand-in swap** have only been seen in stills. Record a short sequence of a visitor
    and a resident passing a door, and of the swap at 45 m.
12. **Residents' workload** is chaotic: small changes move completed jobs between 22 and 86 in the living
    test. The machine hall got no visit in the final 122 s run. Consider seeding or a calmer scheduler.
13. **Parcel sorter** stands 0.7 m beside the mission stairs (wanted 1.5 m); needs a smaller footprint or
    another spot.
14. **Patrol robots** yield up to 22 s behind trams in the living test (not asserted).

## Tests and tooling

15. **Low-tier survey scenes for lobbies** fail by design: `lobby-agent-day` and `lobby-agent-night` wait
    for an interior that the low tier never shows. Make lobby scenes skip that wait on low.
16. **Kit test pin**: `scripts/test-system-world-v3.mjs` accepts "none or six" interiors; pin six and 31
    designs.
17. **Survey harness**: jumping the camera across the city needs the visitor body moved with it (done per
    scene in the lobby scenes); move it into the harness for every scene. Whole-run timeout is 300 s by
    default; the full set needs `--timeout 2400`.
18. **Browser report** has only the final `loadedBytes`; log the figure at first display as well.
19. **Frame time** has never been measured for the new windows, interiors and lamp pools; add a timing
    scene per tier and a budget.
20. **`build_textures.py --check`** needs Python 3.13 on this machine (PIL missing in 3.14).
21. **GitNexus** has no index for the worktree; run change detection against `main` before merging.
