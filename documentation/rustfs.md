# Managed RustFS S3 storage

AuraGo can optionally manage a private RustFS server in Docker and use it through the existing `s3` tool. It is disabled by default. External S3 configurations continue to work with the `external` provider; selecting `rustfs` retains those external settings for a later switch.

## Setup

1. Enable Docker management and allow Docker mutations. Open **Configuration → S3**.
2. Enable managed RustFS, choose its initial bucket, save, and run the prerequisite check.
3. Install RustFS. Installation pulls the fixed image, creates private persistent volumes, starts the server and provisions its bucket and restricted machine identity. The operation receipt distinguishes accepted, running, completed, failed and uncertain outcomes.
4. After provisioning, select **RustFS** as the S3 provider. Enable S3 and agent access, configure bucket/prefix scopes and the required permissions, then save.
5. Test the saved connection. A data test needs a read grant; without one, the result is unavailable, not a successful write-only connection test. Runtime health remains visible separately.

Managed placement requires a local Linux Docker Engine. Native AuraGo publishes only to loopback; a containerized AuraGo uses a proven shared application bridge without publishing the S3 port. The Docker control network is not a data network. An unproven container placement or remote Engine is refused; use the external S3 provider for an independently administered server.

Native Engine connections must use a local Unix socket or Windows named pipe. A loopback TCP address alone cannot distinguish a local daemon from a tunnel, so native TCP placement is refused. Container placement proves its own Engine identity and supports the existing private socket proxy.

The existing socket proxy needs container, image, network, volume, information/version and mutation access. RustFS does not require enabling Docker builds or exec. `docker.read_only` permits observations but refuses installation and lifecycle mutations.

## Permissions

```yaml
s3:
  enabled: true
  provider: rustfs
  readonly: false
rustfs:
  enabled: true
  bucket: aurago
  agent_access: true
  permissions:
    allow_read: true
    allow_write: true
    allow_delete: false
    allowed_locations:
      - bucket: aurago
        prefixes: ["agent/"]
```

The example grants reading and writing under the literal `agent/` prefix in `aurago`, with deletion disabled. Prefixes are case-sensitive object-key strings, not filesystem paths or wildcard patterns. An empty location list or prefix list grants no access; an explicit empty string prefix grants that entire bucket. Wildcards and policy-variable syntax are rejected.

- **Read** permits scoped listings and downloads. Bucket listing returns authorized locations rather than an unrestricted server bucket inventory.
- **Write** permits uploads and copy destinations. Copy also needs read access to the source.
- **Delete** permits object deletion. Move needs source read/delete and destination write; source deletion occurs only after the copy completes and authorization is rechecked.
- S3 read-only overrides write and delete. Bucket/IAM administration, lifecycle management and presigned sharing are outside the agent's S3 capabilities.

The server enforces a matching IAM policy on a dedicated child service account. Root and parent IAM credentials stay in private Vault state; only the backend receives the child credential for a scoped call. Credentials never enter YAML, browser responses, agent prompts or Python secret exports.

Managed uploads use AuraGo's authorized outgoing-file roots. Managed downloads use rooted temporary files, the existing Desktop file-size limit, and an atomic final publication check. Listings are bounded by item and tool-output budgets. Revoking permissions, disabling agent access or switching providers cancels admitted requests; an old request cannot borrow a newly selected provider's credentials. A remote mutation already accepted by S3 may have completed when cancellation arrives; uncertain results are not automatically replayed.

## Operations and recovery

Start, stop, recreate and remove are administrator actions on saved settings. Removal retains both data and credential volumes. Recreate replaces the owned container with the same qualified image and retained volumes; it does not erase or migrate storage. Normal AuraGo shutdown drains its own calls and leaves an established storage server running. Explicitly stopped or removed services are not automatically revived.

Before saving a change that removes Docker authority, AuraGo must stop the existing runtime using the old authority. An unsuccessful stop rejects the save. If configuration was changed outside AuraGo, the UI reports blocked or unknown state rather than claiming a stop succeeded.

The manager records the instance, Engine and volume identities. A missing or replaced volume, incompatible Engine, missing initialized bucket/IAM state or foreign container requires recovery. It must not silently create an empty replacement. Preserve the encrypted Vault, master key and both Docker volumes together for recovery. Take a consistent backup while the runtime is stopped before changing its image or storage layout. This integration does not provide an automatic downgrade or a conversion from single-node storage to a distributed deployment.

Generic agent Docker/Compose access to managed RustFS containers, ownership labels and protected volumes is denied independently of the Docker host-access switch. This boundary does not make unrestricted host shell or a separately exposed Docker socket safe; those grants remain independent operator authority.

## Pinned dependency and verification

The runtime is `docker.io/rustfs/rustfs:1.0.4@sha256:ed9210d6411d77ffceb70d94b964a3cb7d225983368bc25bebe01e995293dcc2`. Its console is disabled. Native IAM requests use signed JSON on `/rustfs/admin/v3`; the MinIO encrypted admin protocol is not used. Data calls reuse the existing MinIO Go S3 client.

The image supports Linux amd64 and arm64. Live amd64 testing does not qualify real arm64 hardware. A new image pin needs the native-IAM denial and S3 lifecycle acceptance suite before release; there is no floating-tag update button.

Focused checks: `go test ./internal/config ./internal/rustfs ./internal/tools ./internal/agent ./internal/server -run 'RustFS|S3'` and `go test ./ui -run RustFS`. The opt-in Docker fixture in `internal/rustfs/live_test.go` uses isolated temporary resource names and tests the actual pinned server. Enable it with `AURAGO_RUSTFS_LIVE=1`; inside a Linux container, also set `AURAGO_RUSTFS_CONTAINER=1` and the Docker proxy endpoint. Ordinary unit tests do not install or start RustFS.

RustFS is [Apache-2.0 licensed](https://github.com/rustfs/rustfs/blob/1.0.4/LICENSE), Copyright 2024–2026 RustFS, Inc. Its unmodified image remains a separate optional dependency. See `licences/UEBERSICHT.md` and `licences/texts/Apache-2.0.txt`.
