# Notebook documents and local kernels

## Purpose

Preserve nbformat 4 documents and provide explicitly local, stateful Python execution.

## Ownership

This package owns notebook revisions, kernel sessions and the fixed Jupyter bridge protocol. `internal/kaggle` owns rooted project storage, remote jobs and Kaggle credentials. The server owns authentication and lifecycle; Desktop owns editing and safe rendering.

## Local Contracts

- Preserve unknown notebook metadata, attachments and original output MIME bundles. Conditional writes require the current content ETag; never overwrite a conflicting revision.
- Every execution identifies the notebook revision, cell and execution. Late output does not replace edited content. Never auto-run code after reconnect or restart.
- Local execution requires the integration, local runtime, execution, Python, filesystem and Docker grants. Read-only blocks execution. Internet is independently disabled by default.
- One owned, non-root, resource-limited container per session. No host secrets, Docker socket or published port. Only explicitly selected project files cross the boundary.
- The bridge uses `jupyter_client` and bounded JSON frames over Docker exec attach. Original notebook JavaScript/widgets remain inert; HTML display is sanitized.
- Configuration revocation stops affected sessions. Server shutdown removes owned containers; variable state is lost and must be reported.
- Failed container removal remains `stopping`, retains its container identity and session slot, and is retried by maintenance or an explicit stop. Only confirmed removal reports `stopped`; concurrent cleanup requests serialize per session.
- Protect the canonical managed-container label and name prefix from generic Docker tools. Readiness and output events can precede start/restart/execute HTTP responses; retain correlation through those races.

## Work Guidance

Keep the image version, pinned multiarch base, bridge protocol, release workflow and operator guide aligned. Pin the published, signature-verified runtime OCI index digest in `RuntimeImage`; inspect, pull and create use that same digest with no tag fallback. Re-inspect after pulling before admitting a session. No Kaggle credential may enter the kernel environment.

Return `SessionSnapshot` through HTTP, tools and events; runtime sessions retain their locks and private handles internally. Preserve the snapshot JSON fields.

## Verification

Roundtrip/conflict tests, protocol size and correlation tests, two dependent Python cells, interrupt/restart, revocation and Docker smoke tests.

## Child DOX Index

None.
