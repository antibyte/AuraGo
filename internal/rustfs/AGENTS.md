# Managed RustFS

## Purpose

Operate one optional single-node RustFS S3 server with retained Docker storage and a separate scoped machine credential.

## Ownership

`internal/rustfs` owns the digest pin, native SigV4 admin client, IAM policy generation, Docker placement/ownership, Vault identity, lifecycle receipts and S3 access leases. Config/server/tools own their publication, administrative routing and data-operation boundaries.

## Local Contracts

- Default construction is passive for uninitialized stores. Only explicit Install creates the identity, image, volumes, bucket and IAM. Start/Recreate require provisioned identity. Stopped/removed intent survives process restart.
- Pin `Image`; runtime UID/GID is `10001:10001`. Root key files use a private named volume seeded by an unstarted, network-none helper. Neither helper nor runtime exposes root values in environment/arguments, host bind paths or logs.
- `rustfs_state` is reserved encrypted Vault state. Preserve root/parent/child keys, instance and Engine identity, original bucket, volume creation identity and running intent. Persist `IAMPending` and the pending policy hash before IAM mutations, retaining the established policy hash until verified completion. Missing initialized storage/IAM requires recovery; never generate replacement credentials for populated storage.
- Manager root, dedicated IAM parent and child service account are separate. Data access uses the child only. Explicit admin/STS denies occupy separate policy statements. IAM and local literal bucket/prefix boundaries must agree; Readonly overrides mutations.
- Native admin requests use JSON and AWS SigV4 service `s3`; reject all redirects and never automatically replay mutations. Unknown outcomes require inspection before explicit Start recovery; a successful permission-save retry never clears the global uncertainty latch.
- Stop/delete prove ownership independently of the current image pin. Start/data readiness additionally validate the desired runtime spec and observed private endpoint. Remove runtime retains both named volumes; normal operations never delete volumes.
- Native placement uses loopback publication through a local Unix socket or Windows named pipe. Native TCP (including loopback tunnels) is unproven and rejected. Container placement proves the current container and selects a shared bridge excluding Docker-control/host/shared namespaces; no host publication or Console. Use the owned runtime's literal unicast address on the verified network ID, never its shared DNS alias. Revalidate before managed lease admission and administrative revocation; address replacement cancels managed leases only.
- Config publication performs no I/O: cancel changed authority and reconcile asynchronously. Pre-persistence preparation revokes old machine access and confirms required stops using existing authority. While preparation is unpublished or failed, background observation cannot restore authority. Pending IAM recovery requires read-verified explicit Start or retry of the same permission save; an unrelated save or process restart cannot replay an uncertain mutation. Only the exact successful preparation permits pending asynchronous reconciliation during publication. Startup reconciles a known running store to saved tightened rights without restarting a stopped store.
- External S3 leases have a separate epoch from managed lifecycle. Provider/credential/readonly/filesystem authority changes cancel applicable leases; inactive RustFS lifecycle must not disrupt external transfers. `Publish` admits only the short final local rename under the generation lock.
- Receipts are process-local and bounded. Late/cancelled jobs cannot publish readiness across job/generation changes. Server shutdown cancels/drains work and leaves an established runtime running.
- Health probes use `/health/ready`; data readiness requires storage, actual owned IAM and the current permission policy, not merely Docker running or a persisted hash. Connection probes remain read-only and may report unavailable read verification for zero-read scopes.

## Work Guidance

Keep secrets out of diagnostics; register each Vault-read credential before network activity. Treat Engine/provider responses as untrusted data. Preserve immutable ownership and retained storage before mutating.

## Verification

- `go test ./internal/rustfs` covers policy intersections, external authority, publication fencing, owned endpoint refresh, interrupted IAM recovery, Vault failure before mutation, redirected mutations, nil Vault and optional defaults.
- `AURAGO_RUSTFS_LIVE=1 go test ./internal/rustfs -run '^TestLiveRustFS$' -count=1 -v` uses uniquely labeled disposable containers/volumes and cleans only those resources. It validates native IAM, actual MinIO S3/multipart behavior, privilege denials, live/startup revocation, stopped intent, retention and missing-IAM recovery.
- `AURAGO_RUSTFS_CONTAINER=1` enables the same opt-in fixture inside a Linux test container with an explicitly configured Docker proxy and a disposable duplicate DNS alias. Run physical amd64/arm64 and Windows Docker Desktop acceptance separately; compilation alone never qualifies a daemon or CPU architecture.

## Child DOX Index

None.
