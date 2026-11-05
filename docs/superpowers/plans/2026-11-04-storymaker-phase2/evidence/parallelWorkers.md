**Spike result: parallel player workers reach 24 fps at N=4 (35 fps with worker-side PNG encoding), up from 12-15 fps serial, and no frame changes. The restart and crash paths were not run.**

I could not save `report.md`: the Write tool refused it as a report file. This message is the report. `prototype.diff` is saved. The worktree is deregistered and has no node_modules junction. An empty `wt/` directory is left because it was "busy" and I could not delete it. Nothing in the main checkout was touched.

**Box and method.** i7-11700B, 8 cores / 16 logical CPUs, 64 GB, Chrome 156.0.8078.80, FFmpeg 8.1.1 (libx264 veryfast crf23). Story: firewall.json at 1080p30 (342 frames) through the real path. Each figure was measured at least twice. The serial loop alone varied 12-15 fps between runs, so differences under 10% are noise.

**Prototype.**
- The host runs N player workers, each with its own compile and assets. Chunks (C=8) go to worker `j mod N`, and a reorder buffer capped at N·C·2 feeds one in-order poster, so Go is unchanged.
- Pre-roll of 2 frames is switchable.
- 19 of 43 chunks contain code-scene frames (44%). They stay on worker 0, which is the serial part.
- Worker-side PNG encoding is a switch.

**(a) fps and memory** (host-side PNG, pre-roll on, C=8; fps from two runs a/b):

| N | fps a / b | Chrome tree RSS MB |
|---|---|---|
| 0 (today's serial loop) | 12.2 / 14.9 | 685 |
| 1 | 15.6 / 19.4 | 683 |
| 2 | 22.1 / 21.5 | 732 |
| 3 | 22.4 / 23.9 | 756 |
| 4 | 24.4 / 24.6 | 773 (renderer 333, browser 314) |
| 6 | 22.4 / 23.7 | 807 |
| 8 | 24.2 / 23.5 | 819 |

- Each added worker costs about 18 MB in the renderer plus about 9 MB of reorder buffer.
- At N=8 the buffer peaked at about 70 frames and 67-80 MB. Memory is not the limit at 1080p.
- With the PNG encoded in the worker: N=1 gives 19.7 fps, N=4 gives 30-36, N=6 gives 36-41 (876 MB).

**(b) Parity.** I compared the SHA-256 of the PNG bytes and of the decoded RGBA for all 342 frames, with two runs per configuration.
- Every pair is identical in both hashes: N=0 vs 1, 1 vs 4, pre-roll on vs off, N=4 with C=3, and worker-encoded vs host-encoded PNG. Repeat runs of one configuration are also identical.
- Pre-roll changes no frame, and chunk borders show no difference. The frames are not blank: 320 of the 342 are distinct.
- Likely reason: `transferToImageBitmap` empties the worker canvas after every frame, and `renderFrame` repaints it fully.
- Not proven: I did not mutate the code to confirm the comparison would catch a difference. Only one story and one Chrome build were tested.

**(c) Bottleneck** (ms per frame, serial, about 75 in total):
- **Draw:** 17-23 as the host sees it, of which 4.5-6.7 is the actual player draw.
- **PNG encode:** 28-30 on the host (`drawImage` + `toBlob`), 26-46 in the worker under load.
- **POST:** 20-29. Go's `Ingest.Accept`, including the write into FFmpeg stdin, takes 1.1-1.5.
- **FFmpeg:** offline on the dumped PNGs with the exact production arguments, 170-179 fps (three runs). It is not the limit; hardware encoders were not tried.
- **Host main thread:** it is the first ceiling. With host encoding at N≥4, per-frame encode time rises to 63-149 ms and fps stalls near 25.
- **In-order POST:** it is the next ceiling, at roughly 45 fps.

Encoding in the worker changes the picture: it is worth about +40-50% beyond N=4, with identical bytes.

**(d) Recommendation.**
- **Encoding:** encode the PNG in the worker.
- **N:** `N = clamp(floor((logicalCPUs − 2) / 3), 1, 6)`. That gives 4 on this box, 2 on 8 CPUs and 1 on 4 or fewer. N=6 adds about 10% for about 60 MB. Pass the CPU count from Go in `boot.json` so tests can pin it.
- **Memory bound:** `N ≤ freeMB / (60 + 2·C·frameMB)`, with frame size about 0.6 MB at 1080p (times 4 at 4K).
- **C:** 8.
- **Pre-roll:** off by default, but keep the switch and run the parity test with it on and off.
- **Render timeout:** keep 6 fps for PNG. One CPU with N=1 is still faster than today, and aurago-test is not measured.
- **Functions that change:**
  - `host-render.js`: the serial loop becomes a scheduler (chunk plan, per-worker loop, poster, generation loop).
  - `host-core.js`: helper engines that build no scene workers, and `render` can return a worker-encoded blob.
  - `worker-player.js`: `render` accepts a `png` option.
  - `listener.go`: `PageParams` gets worker-count and chunk fields.
- **Worker crash or deadline:** `Engine.renderNow` already kills the engine and rejects. That aborts the generation, the page posts `/error`, and the job fails as today.
- **Restart (hardware-encoder fallback to libx264 from frame 0):** the poster stops and the generation is abandoned. The buffer is cleared and the same engines re-run from frame 0. Only one POST is ever in flight, so no stale frame can reach the ingest.

**Not verified.**
- The restart path and a worker crash or deadline were written but never run.
- No hardware encoder, no aurago-test or low-CPU box, no 4K, and no story without code scenes (the speed-up should be larger there).
- The strict-proxy cost of the POST was not isolated.
- The player static guard bans `performance.now`. My spike code captures it before the determinism shadow, so the guard would flag it.

**Repeat.** In a scratch worktree, run `bash …/spikes/parallel/run.sh <name>` with `AURAGO_PSPIKE_NS=0,1,2,4 AURAGO_PSPIKE_ENC=host|worker AURAGO_PSPIKE_HASH=1`. It needs `AURAGO_RUN_BROWSER_SMOKE=1`, `AURAGO_PSPIKE=1` and the sandbox off. `parity.sh`, `split.sh`, `cmpall.sh`, `analyze.mjs` and `ffbench.mjs` sit beside it, and the raw results are in `out/`.

Files are in `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/parallel/`:
- `prototype.diff`
- `run.sh`
- `parity.sh`
- `split.sh`
- `cmpall.sh`
- `analyze.mjs`
- `ffbench.mjs`
- `out/`