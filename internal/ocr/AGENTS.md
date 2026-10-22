# Optional OCR service

## Purpose and ownership

This package owns the Docker-only Indra runtime, private transport, transient
owner-bound queue, inputs and results. The server owns publication and lifecycle;
tools, chat and indexer share this manager. Python runs only inside the image.

## Local contracts

- Default off. Status is passive. Setup pulls the reviewed digest; no client
  image builds or arbitrary image/command URLs. An imported exact platform config
  ID is accepted offline. Start is explicit or once at enabled server startup.
  Manual stop survives configuration reloads until explicit start/server restart.
- Read authorization belongs to each ingress before Stage. Stage checks magic
  against extension, snapshots privately and bounds bytes. Owners are
  authenticated Desktop sessions, agent sessions, or trusted internal indexing.
  Neither caller-supplied paths nor job IDs grant authority.
- One file worker, 300 seconds, 100 pages, 32 MiB, four CPU threads, 2 GiB RAM.
  The 1 GiB temporary budget reserves 64 MiB for the parser tmpfs and the
  remainder for host inputs/results. Result output is at most 16 MiB per file.
- No root, writable rootfs, host workspace mounts, privileges or parser access to
  Docker. Native transport uses loopback on a dedicated bridge without
  masquerading; Docker mode uses a dedicated internal network and proven
  originating-container identity. A per-start token is never forwarded in the
  worker environment or logs.
- Reap orphan resources only by owner/component and installation scope; Docker
  scope also binds the proven controller name. Never delete another instance.
- Kill a process group on cancellation/timeout; disconnect alone cannot cancel
  a worker waiting for its first page. TIFF count/dimensions are checked before
  decoding and frames decoded individually. PDF page/render bounds precede Indra.
- Closing an app does not cancel accepted jobs. Terminal jobs expire after 24h,
  orphan uploads after 1h. Restart deletes only owned flat UUID files, no durable
  history. Partial output remains explicitly incomplete and never replaces a
  valid index generation.
- Queue capacity excludes retained terminal jobs. Automatic reuse accepts only
  queued, running or completed jobs; incomplete jobs permit a fresh attempt.
  Retained results still obey the shared storage budget and expiry.
- Model license copies stay byte-identical. Model/Python artifacts are locked by
  SHA-256. Rebuild the OCI export and run scripts/ocr/pin_image.py to change image
  pins; publication and server deployment are separate authorized release steps.

## Verification

Run focused OCR tests in this package, tools, agent, server, config and services.
Keep `TestOCRRetainedResultsDoNotConsumeQueueCapacity`,
`TestOCRAutomaticRetriesPartialAfterServiceRestart` and the full-queue reuse check.
Use TestOCRDockerSmoke with an immutable local image and generated synthetic
fixtures for real recognition, cancellation and both network variants. Native
arm64 runner smoke is required before publishing an arm64 release.

## Child DOX Index

None.
