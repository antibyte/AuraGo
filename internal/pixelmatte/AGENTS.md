# Pixel AI cutout (background matting)

## Purpose

On-demand AI background removal for the Pixel image editor: pinned ONNX matting models, the image pre- and
post-processing around them, an isolated one-shot worker process (`aurago --pixel-matte-worker run`) and the
`Manager` that `internal/server/pixel_cutout.go` serves as `/api/pixel/cutout*`. Plan:
`docs/superpowers/plans/2026-10-21-pixel-pro-wp8b.md` (local, git-ignored).

## Ownership

`internal/pixelmatte` owns the model pins (`model.go`), tensor name discovery (`onnxio.go`), pre/post-processing
(`image.go`), the worker (`worker.go`), the worker launcher (`runner.go`) and the download/status/run state
(`manager.go`). ONNX Runtime, the asset cache and the worker environment belong to `internal/onnxrt`
(`internal/onnxrt/AGENTS.md`); this package adds no C API index there. `cmd/aurago/main.go` dispatches
`WorkerFlag` in `runIsolatedWorker` before any initialization. The package holds no config, HTTP or UI code and
does not import `internal/config` or `internal/server`.

## Local Contracts

### Models and processing

- Pins in `model.go`: `standard` = ISNet general-use (Apache-2.0), `precise` = BiRefNet general lite
  (swin_v1_tiny, MIT), both rembg release v0.0.0 downloads with pinned size and SHA-256. Change a pin only with
  new size + SHA-256 and a passing run of `TestRealModelsIntegration`.
- Both models read `[1,3,1024,1024]` float32 and the first graph output carries the matte in channel 0 (ISNet has
  eleven outputs, only the first is requested). Tensor names come from `ParseIONames`, which reads the
  protobuf wire format of `ModelProto.graph` (first graph input that is not an initializer, first graph output);
  both pinned models use `input_image` / `output_image`.
- `Preprocess` (rembg): decode PNG/JPEG (at most 32 Mi pixels, `MaxPixels`), drop alpha (straight colour, like PIL
  `convert("RGB")`), resize to 1024x1024 ignoring the aspect ratio (bilinear), divide by the largest 8-bit channel
  value of the resized image (at least 1e-6), then `(x - mean) / std` (ISNet mean 0.485/0.456/0.406 std 1;
  BiRefNet the same mean, std 0.229/0.224/0.225). `Postprocess`: sigmoid for BiRefNet only, min-max normalise (a
  constant plane becomes 0), quantise to 8 bit, resize back bilinearly, PNG `image.Gray`.

### Worker

- `RunWorker`: usage errors exit 2, failures exit 1 with exactly one stderr line (a recovered panic prints the
  stack first and the one-line `pixel matte worker panic: ...` last), success writes the matte PNG to stdout and
  exits 0. Input is one image on stdin, at most 64 MiB. One run per process; the session uses the CPU provider
  with `DisableSpinning`. `DefaultThreads` is `max(1, NumCPU/2)`, at most 8.
- Native code runs only here. The server starts the worker with `onnxrt.ConfigureWorkerCommand`, kills it when the
  request context ends or after the run timeout (`DefaultRunTimeout`, 180 s) and caps its stdout at 48 MiB.

### Manager

- Layout: models in `<data>/pixelmatte/models/<asset id>.onnx` (own `AssetCache` root), the runtime in the shared
  `<data>/embeddings` cache (`onnxrt.CPURuntimeAsset`). Nothing is downloaded unless `Prepare` is called.
- `Status` never hashes: a model is ready when the file has the pinned size (downloads land by rename after the
  SHA-256 check), the runtime when `.complete.json` and the library exist. `Prepare` is single-flight per model,
  runs detached from the request and ends with the context passed in (the server lifetime). One run at a time
  (`ErrBusy`); `ErrNotReady` when the model or runtime is missing.
- `Available()` = a CPU runtime asset exists for GOOS/GOARCH. Tests use the `Installer` and `Runner` seams and
  the `openEngine` variable; no native code runs in unit tests.

## Verification

- `go build ./cmd/aurago`, `go vet ./internal/pixelmatte`, `go test ./internal/pixelmatte -count=1`,
  `go test ./internal/server -run 'TestPixel' -count=1`, `go test ./cmd/aurago -run 'Worker' -count=1`.
- Opt-in real run, no downloads: `AURAGO_PIXELMATTE_REAL=1 AURAGO_PIXELMATTE_RUNTIME=<onnxruntime dir>
  AURAGO_PIXELMATTE_MODELS=<dir with the .onnx files under their download names> go test ./internal/pixelmatte
  -run TestRealModelsIntegration -v -timeout 30m`; `AURAGO_PIXELMATTE_MODELS` alone also enables
  `TestPinnedModelTensorNames`.

## Child DOX Index

None.
