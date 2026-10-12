# Person detection

## Purpose

D-FINE person detector for the Network Cameras detection mode: pinned model manifest, frame preprocessing and
person decoding, detection-set comparison, the WebGPU model patcher, the `aurago --person-detect-worker
prepare|serve` worker, and `NewDetector` (backend selection, GPU model cache, self-test acceptance,
supervision and restarts). The package is camera-independent and holds no config, store, scheduler, server
or UI code. Design: spec `docs/superpowers/specs/2026-10-11-network-cameras-person-detection-design.md` and
plans `docs/superpowers/plans/2026-10-11-person-detection-*.md` (detector API C2 and worker protocol C3 in the
overview; local, git-ignored).

## Ownership

`internal/persondetect` owns everything above, including the JSON-lines worker protocol, which has two ends in
this package: the worker (`worker.go`, `serve.go`, `protocol.go`, `engine.go`) and the client
(`worker_process.go`, `supervisor.go`, `detector.go`, `gpumodel.go`). Change both together. ONNX Runtime, the
asset cache and worker process environment belong to `internal/onnxrt` (`internal/onnxrt/AGENTS.md`).
`cmd/aurago/main.go` dispatches the worker through `persondetect.WorkerFlag` before any other
initialization, so stdout carries only the JSON protocol and no config, log or database setup runs in the
worker. ONNX Runtime is loaded only in worker subprocesses; the one in-process load is the opt-in
`TestPrepareIntegration`.

## Local Contracts

### Models and frames

- Pins in `model.go`, attribution in `THIRD_PARTY_NOTICES.md`: D-FINE-N (`standard`) is the AnnotateIt ONNX
  export of `ustc-community/dfine-nano-coco` (the onnx-community D-FINE-N export finds no persons), D-FINE-S
  (`accurate`) the onnx-community conversion of `ustc-community/dfine-small-coco`. Each URL names a commit; size
  and SHA-256 are pinned. Change a pin only with new size + SHA-256 and a passing opt-in integration run of
  both tiers (the patch count included).
- Both tiers share one I/O contract: input `pixel_values` float32 `[1,3,640,640]` RGB; outputs `logits`
  `[1,300,80]` (sigmoid, class 0 = person) and `pred_boxes` `[1,300,4]` (cx, cy, w, h, normalized). There is
  no NMS. `outputLayout` rejects any other shape.
- `Preprocess` accepts only complete JPEGs (grayscale and CMYK become RGB), resizes the whole frame with
  `x/image/draw` BiLinear (antialiased, no letterbox) to 640x640 and writes CHW `/255` values without
  mean/std. `DecodePersons` keeps sigmoid score >= `minScore`, converts to x1,y1,x2,y2 clamped to 0..1, drops
  degenerate and non-finite boxes and sorts by score. Boxes are normalized to the frame.
- Frame limits are checked on the JPEG header before decoding: at most 8192 px per side and 16 Mi px
  (`1<<24`; a pixel cap alone does not bound the decoder and scaler allocations). A frame is at most
  `maxFrameBytes` (16 MiB, defined once in `protocol.go`; the client, the worker and the request line limit
  all use it). A hostile frame at the limits costs up to ~580 MB in `Preprocess` (progressive CMYK 2048x8192);
  a normal 1280x720 frame ~23 MB and ~30 ms. The worker handles one frame at a time.

### Comparing detections and self-tests

- Raw GPU and CPU tensors differ on low-confidence queries (near-tie ordering), detections do not. Self-tests
  and `prepare` compare detection sets on `ReferenceImage()` (embedded CC0 photo, 800x533), never tensors.
  Both sets are decoded at 0.28 and compared with `SameConfidentDetections(a, b, 0.3, 0.9, 0.02)`: every
  box with score >= 0.32 in either set needs its own partner in the other (IoU >= 0.9, score difference
  <= 0.02, exact bipartite matching, independent of box order); boxes in the 0.28..0.32 band may stay
  unmatched. Do not decode at exactly 0.3: D-FINE-N scores one reference person at 0.303.
- Two empty sets compare as the same, so the reference side must also find >= 1 person with score >= 0.5
  (the original model for `prepare`, the CPU session for a WebGPU self-test, the session itself for a CPU
  worker). Go's resize differs slightly from PIL: tests expect >= 3 persons >= 0.5 for D-FINE-N (PIL: 4).
- Self-test timing: one unmeasured warm-up (the first WebGPU run compiles shaders for 2-3 s), then 3 runs
  timed as one interval (Windows process CPU time ticks at ~15.6 ms). A WebGPU worker also loads a CPU
  session of the same model with `cpuThreads()` threads, measures it, then measures the GPU session again
  and reports the lower GPU CPU time (cold-start shader work inflated a first measurement to exactly 50 %);
  the comparison session is released before the hello line. Process CPU time comes from `Getrusage` (Unix) or
  `GetProcessTimes` (Windows); a time that cannot be read fails the self-test.

### WebGPU model preparation (`prepare`)

- The WebGPU EP rejects `MatMul` with a 1-D second operand. `prepare --runtime <dir> --in <model> --out
  <patched>` folds constants with ORT `OptimizationBasic` into `<out>.folded-<hex>.onnx` (never a higher
  level: it bakes in hardware-specific kernels), runs `PatchVectorMatMuls`, verifies original against patched
  on two CPU sessions with the comparison above, and publishes with a temp file `<out>.tmp-<hex>` plus the
  retrying `fileutil.Rename`. Stale `<out>.folded-*`/`<out>.tmp-*` files of killed runs are swept first (never
  the input, nothing when `--out` names no file); overlapping prepares of one output can make each other
  fail, so callers serialize them. `--in` and `--out` must be different files.
- `PatchVectorMatMuls` works on the protobuf wire format and copies everything else byte for byte. It
  rewrites a 1-D float initializer to `[K,1]` only when every consumer is a default-domain `MatMul` with two
  inputs and one output that uses it as input 1, the initializer name is unique and not a graph input,
  output or value_info, and each such `MatMul` output is no graph output and feeds only `Reshape` input 0.
  It then drops the stale value_info of those outputs. A model with subgraph attributes or training_info is
  returned unchanged with count 0; `prepare` fails on count 0. Both folded tiers patch 3 nodes (three
  integral `MatMul`s share one `[33]` initializer), which the integration test pins.
- Output: one JSON line `{"type":"prepared","patched_nodes":N}` or `{"type":"error","error":...}` on stdout, the
  verification summary on stderr. Exit codes: 0 success, 1 failure (including a recovered panic), 2 usage.

### Worker (`serve`)

- `serve --runtime <dir> --model <file> --provider cpu|webgpu --threads 1..64` loads the session (spinning
  disabled), runs the self-test, prints one hello line (`version`, `providers`, `provider`, `self_test`, or `error`
  when loading failed) and then answers JSON-line requests. Every stdout line is JSON; stderr is log only (ORT
  warnings such as "Some nodes were not assigned to the preferred execution providers" are not errors).
- Requests: `{"id","command":"detect","jpeg":<base64>,"min_score":(0,1]}` answers with
  `{"id","type":"result","boxes","width","height","wall_ms","cpu_ms"}` or `{"id","type":"result","error"}`;
  `{"id","command":"close"}` answers and exits 0. A line is at most base64(`maxFrameBytes`)+1 KiB; a bad field
  in a line with a readable id is answered for that id (error text cut to 256 bytes) and serving continues. An
  unreadable stream, an over-long line, invalid JSON or a field error without an id end the worker with exit
  1. A panic while one frame is handled is answered as that frame's `internal error`; any other panic is
  recovered by `RunWorker` as an id-less `{"type":"error","error":...}` line (exit 1), which clients treat as a
  fatal worker error. A native ORT crash ends the process without a line.
- `self_test.gpu_cpu_ms` may be 0 and is then omitted; read a missing value as 0. `cpu_ms` of a result has ~15.6 ms
  resolution on Windows and is 0 when unreadable.

### Detector (`NewDetector`)

- Models install to `<ModelDir>/<tier>/model.onnx`, runtimes to the shared `onnxrt` cache root
  `RuntimeCacheDir` (`<data>/embeddings`). `auto` and `gpu` try WebGPU first when
  `onnxrt.WebGPURuntimeAsset(GOOS, GOARCH)` exists and `GPUProbe` passes (default: Linux
  `onnxrt.AccessibleRenderNode("/dev/dri")`, elsewhere true); then CPU. `cpu` uses only the CPU bundle. A
  cancelled context returns its error instead of falling back.
- Acceptance at setup: a GPU worker needs a passed self-test, a CPU reference time and `GPUCPUMS` <= 50 % of
  `CPUCPUMS` (software Vulkan such as llvmpipe measured 3538 %). Otherwise the worker is closed and
  `FallbackReason` carries the cause: `no WebGPU runtime for <os>/<arch>`, `GPU not accessible: ...`,
  `WebGPU runtime: ...`, `GPU model preparation: ...`, `GPU worker: ...`, `self-test: ...`, `self-test: CPU
  reference time is missing` or `GPU path used N% of the CPU path's CPU time`. A CPU worker needs only a
  passed self-test. Restarted workers (both backends) are accepted on a passed self-test alone: the ratio is
  checked once, because re-applying it under CPU contention would turn restarts into failures.
- Threads: CPU path `min(4, max(1, NumCPU/2))` with spinning disabled, GPU path 1.
- GPU model cache: `<ModelDir>/<tier>/model.webgpu-1.27.0.onnx` plus sidecar `.json`
  (`source_sha256`, `ort_version`, `patched_nodes`); reused only when the source SHA-256 and ORT version match,
  `patched_nodes` >= 1 and the model file is non-empty. `prepare` runs in a subprocess under a per-model-file
  lock, returns only after that process exited (also when cancelled) and the sidecar is written last. A GPU
  worker that fails to start or whose self-test did not pass discards the cached model so the next setup
  prepares it again; a CPU-time ratio failure keeps it.
- Supervision (`supervisor.go`): frames are analyzed one at a time. Every unanswered request is tracked with
  its send time and the oldest one bounds the 10 s detect budget, also across callers that gave up, so a
  hung worker is killed once its oldest request is overdue. The hello and self-test phase uses the separate
  5 minute start-up timeout. After a failure the worker restarts with backoff 1, 2, 4 ... capped at 60 s; with
  the production limit of 5 consecutive failures the delays are 1, 2, 4, 8 s, and the fifth failure makes
  every `Detect` return an error wrapping `ErrDetectorFailed` until `Close`. A good answer resets the count.
- `Detect` errors: `ErrDetectorClosed` after `Close`; plain `ErrWorkerRestarting` while a restart is pending;
  the failure that triggers a restart wraps `ErrWorkerRestarting` and its cause; the caller's `ctx` error when it
  gives up (no failure counted, the late answer is skipped); frame-level worker errors are returned as
  `detect: ...` without counting a failure. Callers must treat the worker errors as service state, not as a
  fault of the camera that sent the frame.
- Worker logs: stderr keeps the first 1 KiB, the last 4 KiB and the last crash header line (`panic: `,
  `fatal error: `, `Exception 0x...`, `SIG...: `). Error texts carry one line (`crash report: <header>`, else
  `last log line: <line>`, cut to 400 bytes); the slog warnings carry the whole kept log as `worker_log`.
  Non-JSON stdout lines go to the log, stdout lines over 1 MiB are skipped.
- `Close` asks the worker to exit (2 s), then kills it; afterwards every `Detect` returns `ErrDetectorClosed`.

## Work Guidance

- Unit tests never download or load ONNX Runtime: the test binary acts as the worker through `TestMain` (the
  tokens of `AURAGO_PDFAKE_MODE` are listed in `main_test.go`). Real runtimes and models belong in opt-in tests.
- Keep the worker flag in `WorkerFlag` only; the dispatcher and the client must not spell it separately.

## Verification

- `go test ./internal/persondetect/ ./internal/onnxrt/ ./cmd/aurago/ -count=1`; `gofmt -l`; `go vet` for
  windows/amd64, linux/amd64, linux/arm64 and darwin/arm64; `go build ./...`.
- Opt-in real runs download the pinned model(s) and runtime and need `-timeout 30m`:
  `AURAGO_PERSONDETECT_INTEGRATION=1 go test ./internal/persondetect/ -run '^(TestPrepareIntegration|TestDetectorIntegrationCPU)$' -count=1 -timeout 30m -v`.
  Add `AURAGO_PERSONDETECT_GPU=1` and `TestDetectorIntegrationGPU` for the WebGPU path (needs a GPU; it fails
  rather than falls back when the GPU path is not active). The cache root defaults to
  `disposable/person-detection/cache` (`<cache>/embeddings` runtimes, `<cache>/persondetect/models`
  models); override it with `AURAGO_PERSONDETECT_CACHE`. The `onnxrt` integration tests are separate runs
  (`internal/onnxrt/AGENTS.md`).
- Linux GPU check on aurago-test (there the `aurago` user is not in `render`, so only a container sees the
  iGPU): build the test binaries with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o <dir>/persondetect.test
  ./internal/persondetect/` (and `onnxrt.test`), copy them to a scratch directory, and run them in a
  throw-away `docker run --rm --name <unique> --device /dev/dri --group-add 993 --group-add 44 -e
  AURAGO_PERSONDETECT_INTEGRATION=1 -e AURAGO_PERSONDETECT_GPU=1 -e AURAGO_PERSONDETECT_CACHE=/w/cache -v
  <dir>:/w ubuntu:24.04` after `apt-get install ca-certificates mesa-vulkan-drivers` (993 and 44 are the
  `render` and `video` group ids of `/dev/dri` on that host; check `ls -l /dev/dri`). The default Vulkan
  adapter is RADV on the Ryzen iGPU; Ubuntu 24.04 names the ICD files `radeon_icd.json` and `lvp_icd.json`
  (`VK_ICD_FILENAMES` selects one; llvmpipe must end in the CPU-time rejection). `chown` the work directory
  back on exit. Never touch existing containers, volumes or data there.

## Child DOX Index

None.
