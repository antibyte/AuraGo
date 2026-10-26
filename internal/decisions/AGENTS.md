# Decisions Service

## Purpose and ownership

`internal/decisions` owns the OpenRouter Decisions adapter and managed local multimodal decision model: pinned
model and runtime artifacts, native and Docker lifecycle, device/backend
attestation, request validation, candidate-logit normalization, and status.

## Contracts

- External requests use only the saved provider and canonical OpenRouter HTTPS
  Decisions API, never chat completions or caller-supplied endpoints. Refuse
  redirects and uncertain POST retries; bound time, body sizes and capabilities.
  Catalog discovery explicitly filters decision output and never runs inference.
  Preserve option IDs, labels, custom score values and selected-option probability
  semantics. Provider confidence is separate; external calibration is unverified.
  Missing cost stays unknown. Never claim local GPU/device attestation remotely.

- Model eligibility is shared and passive in status: a fresh catalog requires a
  positive context limit, expired metadata is unknown, and missing models are
  blocked. Explicit evaluations may refresh within their own budget. Preserve
  aliases and pin each call to the observed catalog version; alias changes revoke
  routing caches without rejecting the initial resolution's own response.
- Hundredth-grid provider distributions have a bounded rounding compatibility
  rule, not an upstream precision guarantee. Validate all keys/values/mass before
  normalization, and derive score intervals from the same rounding bounds.
- External image preparation preserves originals, alpha and aspect ratio. Apply
  JPEG orientation, enforce profile/context/base64 budgets and bounded sequential
  encoding, never enlarge/drop/crop. Return metadata-only image adjustments.
- Discovery has a separate 60-second pass budget, 1 MiB stdout and 64 KiB stderr
  caps while reading. Terminate descendants and drain readers. Discovery timeout
  is `runtime_discovery_timeout` (HTTP 504), never a reason for CPU fallback.

- The service is opt-in. Constructing or reconfiguring its manager is passive;
  only explicit install/start or an authorized evaluation may acquire runtime
  resources. Never claim ready until model, projector, runtime build, and
  selected device are verified against the configured pins and live attestation.
- Native process and Docker deployment must use private loopback/service
  transports with generated credentials. Model/projector files are read-only
  in the runtime. Never accept arbitrary runtime URLs or model paths from the
  decision request. Docker host config caps every Decisions container at 8 GiB
  of host RAM with MemorySwap equal to Memory; CUDA still requests every
  visible GPU and Vulkan still receives /dev/dri because the discover JSON has
  no stable Docker device id.
- GPU/iGPU backends are preferred in the fixed order for the platform, with
  discrete devices before integrated devices. Try full accelerator placement
  across all discovered devices first. Only persistent explicit
  `CPUFallbackAllowed` consent permits bounded partial-layer/CPU-projector
  attempts and then pure CPU as the last option. Never switch an active
  generation in response to an evaluation failure. Report the accelerator
  failure that caused a consented fallback; revoking consent stops any mixed
  or CPU generation before it can score another request.
- Resolve no media references in this package. The server supplies bounded,
  already-authorized image bytes and MIME types. Never log request state, image
  contents, prompts, or model output.
- Public responses contain normalized candidate probabilities, never raw
  logits. Candidate choices must map one-to-one to the runtime's selected
  token scores. Keep the response explicitly uncalibrated unless a verified
  calibration artifact is added.
- Cancellation, stop, reconfigure, and shutdown must drain or terminate the
  process/container generation; stale generations may not publish status or
  results. The serialized active evaluation has a manager-owned cancel context;
  policy publication cancels it synchronously so even Docker-backed scoring
  stops accepting work before config save returns. Status snapshots must be
  detached copies. `PrepareReconfigure` is the fast policy-publication half:
  it cancels active work, revokes the old session, and returns cleanup for the
  caller to run after releasing broad config locks. Pending reconfigure
  cleanup gates new Start/Install operations. Stop also publishes a lifecycle
  barrier before releasing the manager lock and holds it through teardown.
  After authorization, Start publishes `starting` and Install publishes
  `installing` before `stopLocked`, with an empty code and Ready false, even
  while `operationCancel` is still nil. A busy lock or an already-pending
  cleanup does not publish. If `stopLocked` or operation registration then
  fails, status becomes `error` only for the current policy and absent cleanup;
  `cancelled` and `lifecycle_busy` never replace lifecycle status.
  Cleanup has a manager-owned ten-second budget, including context-aware lock
  acquisition. Caller cancellation may return earlier. Retain failed sessions
  and the admission barrier with `stopped_with_error`; Stop retries teardown.
  Revoke active device/offload attestation synchronously, retaining package pins.
  Native discovery and resident processes share descendant supervision and a
  two-second pipe drain limit. Docker teardown confirms container removal.
  Keep private-network cleanup and its token until network removal succeeds,
  independently of whether a container was created or is already gone. Failed
  network cleanup retains the session and admission barrier for another Stop.
  AutoStart waits/retries bounded
  lifecycle-busy windows so a replacement cannot race old runtime teardown. Evaluation and
  Start preserve caller cancellation causes, including deadlines, while a
  successful Start detaches the resident process lifetime from the caller.
  The synchronous `Reconfigure` wrapper is only for callers that do not hold
  broad locks.
  Explicit Install also stops an active generation before runtime files can
  be provisioned or replaced; user-facing install controls should make that
  effect clear.
- Setup is a single server-owned operation: save only Decisions settings, acquire
  the pinned runtime/model, start, and validate a text score before publishing
  ready. A low score is valid. HTTP disconnects do not cancel accepted setup;
  stop, saved policy changes and shutdown do. Setup suppresses duplicate autostart.
- Downloads are resumable but are published only after exact size and SHA-256
  verification. Native runtime bundles use a compiled release catalog with fixed
  archive and manifest hashes; v2 inventories include every library and license.
  Published Docker images use compiled immutable digests and need no BUILD grant.
  Developer bundles remain supported. Never invent missing release pins. See
  `documentation/decisions-runtime-release.md` for the publication gate.
  Reuse a verified installed accelerator package without requiring downloads for
  additional backends. A CPU package alone still preserves accelerator acquisition;
  availability never grants readiness or CPU permission.
  Verify the pinned build identity and binary digest before provisioning.
  Publish with `fileutil.RenameContext` without deleting the existing target.
  Verification and publication must observe cancellation; failed replacement
  preserves the previous file.
- Explicit installation checks reuse the install package verification without
  provisioning, downloads, process/container starts, or device inference.
  Native checks hash the bundle with request cancellation; package availability
  must never imply hardware readiness or CPU permission.
- Native and Docker readiness share a five-minute startup budget and bounded
  probes. A timeout is not evidence of device failure: timeout, cancellation,
  pin mismatch and invalid startup configuration end the attempt without
  another device or CPU fallback. Only allowlisted, bounded startup diagnostics
  may reach status. Progress updates are fenced by the active operation ID.
  Installation success means installed, not ready; only attested Start grants
  readiness. The UI keeps accepted operations pending until status resolves.
- Each native/Docker score has a two-minute context through the full response
  read. Its own expiry is `runtime_score_timeout` (HTTP 504), terminal for that
  evaluation only: no retry, device switch or generation retirement. Preserve
  caller cancellation causes and keep the cancel endpoint active while reading.

## Verification

The task router consumes the existing service through an injected scoring
capability. Passive readiness and `CacheGeneration` must not start/install a
runtime. Reconfigure, stop and credential revocation invalidate classifications;
caller deadlines include queue/metadata work. The server alone attributes
router calls to `routing`; ordinary Decisions consumers retain `decisions`.

Test consent/defaults, backend ordering, full-GPU-before-hybrid-before-CPU
retry order, attestation failures, lifecycle revocation, bounded queue and
generation cancellation, download resume/hash failures, native/Docker
cancellation and container payloads, image/request limits, candidate
normalization, cumulative token limits, and malformed runtime replies.
