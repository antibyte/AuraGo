# Local Granite embeddings

## Purpose

Own managed local Granite transports and backend health/fallback decisions.

## Ownership

Native llama.cpp, managed Docker llama.cpp and ONNX workers belong here.
`../onnxrt/AGENTS.md` owns shared ONNX runtime provisioning; memory owns indexes.

## Local Contracts

- Native and Docker HTTP 400/413/422 responses wrap `ErrInvalidInput`. They do
  not count as backend failures, restart servers, activate fallback, schedule
  reindex or require restart. Authentication, transport and server failures
  retain backend handling. Do not silently truncate input or mix fingerprints.
- Cross-format fallback still requires the existing controlled reindex/restart.
  Preserve caller-cancellation and unresponsive-backend distinctions.

## Work Guidance

- Share HTTP classification in `errors.go`; keep error bodies bounded and
  registered credentials out of logs.

## Verification

- `go test ./internal/embeddings`; `TestLlamaHTTPInputErrorsDoNotPoisonManager`
  exercises both transports and a subsequent valid request.
- Real model/hardware acceptance remains separate from local HTTP fixtures.

## Child DOX Index

None.
