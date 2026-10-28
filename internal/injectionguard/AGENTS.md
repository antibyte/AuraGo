# Local prompt-injection classifier

## Purpose

Optional Horizon Labs CPU classifier for web and email content, controlled by administrators.

## Ownership

This package owns artifact pins, the model-specific tokenizer, scoring windows and
private worker lifecycle. Security owns content decisions; server owns Config publication
and administrative APIs; UI owns the Guardian card. Read `../onnxrt/AGENTS.md` for the
shared native runtime/cache contracts.

## Local Contracts

- Default off. Only `Prepare` downloads. Construction, `Status`, `Inspect`, evaluation
  and self-tests never download. `Inspect` verifies an existing cache asynchronously.
  Activation requires verified files and a successful worker self-test.
- Keep revision, sizes and SHA-256 pins together. Model/tokenizer are Apache-2.0;
  update all three third-party inventories when pins change. Never commit weights.
- Native code loads only in `--injection-guard-worker`, before application startup.
  Use the filtered environment/hidden-window helper and verified shared CPU runtime.
  Private text travels over pipes, never command arguments, logs or status responses.
- Tokenization follows the pinned BPE/Metaspace pipeline, including added tokens,
  normalizers and byte fallback. Ignore embedded 1024-token truncation. Score every
  2048-token window including BOS/EOS, with 512 content-token overlap. Maximum INJECTION
  probability wins at threshold 0.5; later windows must not be skipped.
- One inference, eight waiting evaluations, 20 seconds including queue/startup, or
  the earlier caller deadline. Reject excessive input, aborts, worker failures and
  stale generations. Configure closes the worker and revokes in-flight decisions.
  Disable retains files and leaves other ONNX users alone.
- Downloads belong to the server lifecycle, not the HTTP request. Cancellation leaves
  verified assets reusable. Self-tests use fixed local texts and no LLM provider.

## Work Guidance

Reuse `onnxrt.AssetCache`, `VerifyRuntime`, `ConfigureWorkerCommand` and the existing CPU
artifact matrix. No production Python, CGO or GPU dependency. Reference Python is
development-only and runs offline.

## Verification

- `go test ./internal/injectionguard ./internal/security ./internal/onnxrt`
- Real CPU acceptance: set `AURAGO_INJECTION_GUARD_CACHE` to the absolute data directory
  and `AURAGO_INJECTION_GUARD_INTEGRATION=1`; run this package with `-v -count=1 -timeout 10m`.
  Includes exact token IDs, reference scores, long windows and crash recovery.
  `TestModelIntegration` exercises preparation; `TestModelReferenceOffline` uses only
  verified local files. `testdata/reference_scores.py` regenerates score fixtures with
  developer-installed tokenizers/numpy/onnxruntime; record reference versions.
- Run affected agent/mail/summary/server suites and UI `TestInjectionGuard*` with
  `AURAGO_RUN_BROWSER_SMOKE=1`. Record sample accuracy, latency and worker RSS in ignored
  acceptance reports; do not claim a benchmark sample proves attack immunity.

## Child DOX Index

None.
