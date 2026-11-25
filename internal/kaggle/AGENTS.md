# Kaggle platform integration

## Purpose

Native Kaggle resource access, permissions, bounded transfers and durable operation receipts shared by agent and Desktop.

## Ownership

This package owns the Kaggle API contract. `internal/notebooks` owns local notebooks and kernels; server owns HTTP authentication and lifetime. This contract also binds config, agent, UI, release and training integration.

In-process caller: Chibi Sensei drives Kaggle only through `internal/server/chibi_sensei_kaggle.go` (its `KaggleDriver`), with the server's snapshot access. It gates mutations before `Execute`, uses `RemoveFile` only for files it staged and `ReadKernelLog` only for its own run notebooks; see `../chibisensei/AGENTS.md`.

## Local Contracts

- Fixed API operations come from the pinned official SDK. HTTP POST is not a mutation classification. Never expose an arbitrary service, URL or raw authenticated request.
- Secrets come only from the Vault. Register before network access; never export tokens, signed storage URLs or local paths to the model or browser.
- Upload receipts store `kaggle_upload_<job-id>` Vault references in SQLite, never bearer material. The `kaggle_` Python-export denylist covers these slots too.
- Enabled/read-only and operation grants are checked against current configuration. Public mutations additionally require publish; writes default to the authenticated owner. Downloads require local write permission separately.
- Persist admission before mutations. Unknown remote outcomes are never replayed. Repeated request IDs return their original receipt only when the account and canonical request match.
- An explicit administrator confirmation that no remote execution remains may atomically change only `unknown` to `resolved_unknown`. This releases its compute slot without claiming success or cancellation; preserve the account, request, idempotency binding, result and error. Update the timestamp and emit one job event; repeats return the existing receipt. This local cleanup needs no Kaggle credentials/network, survives disabled/read-only policy, and is never an agent tool operation.
- Persist cancellation before sending it; uncertain cancellation stays requested until a remote terminal state. Benchmark cancellation accepts only recorded run IDs. SaveKernel does not promise a session ID: offer the Kaggle website when cancellation cannot be addressed safely, never infer a session from a kernel/version ID.
- Wire enums use SDK names (`QUICK_SAVE`, `SAVE_AND_RUN_ALL`). Structured benchmark slugs and run-ID wrappers are validated against the pinned SDK; heterogeneous agent fields use JSON strings decoded before validation.
- Downloads stage bounded regular files and publish atomically. Storage redirects are uncredentialed and SSRF-checked. Never extract an archive implicitly.
- Scope is datasets, models/variants/versions, notebooks, competition participation, benchmark tasks and read-only forums. Unsupported benchmark deletion is absent.
- `kaggle.allowed_machine_shapes` is the only gate for `save_kernel.machineShape` (the SDK has no enum). `NormalizeKaggleConfig` keeps the names of `config.KaggleMachineShapes` in canonical spelling and order; `CheckGrants` requires a listed name, `enableGpu: true` and `allow_gpu`. `dockerImage` stays refused. `WithConfigSnapshot` copies the list and `intersectConfig` intersects it, like `allowed_owners`.
- Notebook sessions are addressed by `userName` and `kernelSlug`. A version number is not a `versionLabel` (Kaggle answers 404 "Kernel version with label ... not found"), so AuraGo's own status and log reads send none and read the notebook's latest version and session.
- `save_kernel` answers `ref` as `/code/<owner>/<slug>` (`create_dataset`: `/datasets/<owner>/<slug>`), `get_kernel` and the request slug as `<owner>/<slug>`. `notebookRef` accepts both notebook forms and nothing else; `pollRemote` tries the result's `ref`, then the request slug, and skips a job without a parsable ref or `versionNumber`.
- A `remote_running` or `cancel_requested` notebook job ends when its status read is a `NotFound` (`endMissingNotebook`): 403 for the own account's notebook once the job is older than `notebookGoneGrace` (2 min) means it no longer exists (`failed`, or `cancelled` after `cancel_requested`; "The Kaggle notebook no longer exists"); any 403 or 404 ("No runs found") once the job is older than `remote_timeout_minutes` ends it the same way ("Kaggle reports no run for this notebook"). Both store `remote_status` `NOT_FOUND` and keep the rest of the result. Other errors and younger jobs keep waiting.
- `ReadKernelLog` is the only non-durable download-grant path: download grant, filesystem write and network; owner equal to the authenticated account; `get_kernel_session_logs_stream` (`userName`, `kernelSlug`, `waitForLogsUrlSeconds`, no version) through the raw client request; a storage redirect through `storageFactory`; a 16 MiB read cap; scrubbed text and the last `maxBytes`. A running session answers a `text/event-stream` (`data: {stream entry}` events) that replays the log and stays open until the session ends, a finished one a JSON array. The read is bounded by `kernelLogStreamMax` (45 s for request and read) and `kernelLogIdle` (3 s without a byte after the headers); for an event stream the idle end, the overall bound and a transport error return what arrived, any other content type returns the read error, and the caller's own context end is always an error. The body is returned as it arrived. It writes no job row and no project file. 404, 204 or an empty body mean no log yet (empty, nil); a JSON object body is refused. Do not expose it as an agent operation or HTTP route without a receipt decision.
- Kaggle answers a read of a missing resource with 403 `PERMISSION_DENIED`, not 404, and the same 403 for a foreign private resource. `NotFound` covers both statuses (not an uncertain error). `checkResource` lets `save_kernel` create a notebook on 404, or on 403 only when the slug's owner is the authenticated account; a 403 for another allowed owner stays an error. Other operations keep a failed pre-read as an error.
- `RemoveFile` deletes one regular project file under `filesMu`; a missing file is success; invalid names, symlinks and directories are refused. It applies no policy gate: in-process callers apply their own, and there is no HTTP or agent route.

## Work Guidance

- Keep the curated operation catalog, agent schemas, UI capabilities and training contracts aligned. Upstream SDK snapshots are protocol evidence, not executable runtime dependencies.
- Every background job belongs to the server lifetime; UI disconnection does not cancel it. Revocation prevents publication and new remote side effects.
- Attach `BeginJob` before exposing a service to preserve the host's revocation/publication gate. Keep active receipts beyond the completed-history window; SSE replay is bounded to 10,000 events and 64 MiB.

## Verification

Run the package tests and focused config, server, tools, agent and UI suites. Use local HTTP fixtures for writes; live tests require dedicated test resources.

Machine shapes, log reads and file removal: `TestKaggleMachineShape*`, `TestKaggleReadKernelLog*` (event stream, JSON stall, caller context), notebook polling `TestKaggleNotebookRef`, `TestKagglePollRemoteNotebookReachesTerminal`, `TestKagglePollRemoteNotebookGone`, `TestKaggleRemoveFilePathSafety`, missing resources `TestKaggleNotFound` and `TestKaggleSaveKernelCreatesWhenKaggleDeniesTheMissingNotebook`, and in `internal/config` `TestKaggleMachineShapes*` and `TestChibiSenseiTrainingConfig`.

## Child DOX Index

None.
