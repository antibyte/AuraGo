# Network shares

## Purpose

SMB/NFS capability, ownership, and mutation policy.

## Ownership

`internal/networkshares` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Local Network Share Integration Contract
- Local SMB/NFS capability probing must remain passive: never install packages, start services, enable Samba registry shares, or change global server configuration.
- The native `network_shares` tool and Admin UI expose reads only when a configured protocol is actually readable. Mutations require the matching granular permission, `readonly: false`, a writable runtime backend, and an existing canonical directory inside an allowed root.
- Only AuraGo-created shares reconciled through the ledger and a native marker where the backend supports markers may be updated or removed. Marker-less Windows NFS requires an exact ledger match on protocol, name, and canonical path. External, orphaned, unsafe, and drifted shares are read-only, out-of-root shares stay hidden, and removing a share must never remove its directory or files.
- SMB access is limited to configured existing OS principals; no account or password management. NFS accepts only configured IP addresses/CIDRs and must use `sync,root_squash,no_subtree_check` plus `ro` or `rw`; Windows NFS host permissions are limited to individual IP addresses because AuraGo does not manage global client groups.
- Linux Samba uses only `net conf` registry shares when `registry shares = yes` already exists. Linux NFS owns only `/etc/exports.d/aurago-<id>.exports`. Windows uses fixed JSON-driven PowerShell scripts and installed SMBShare/NFS cmdlets.
- Standard Docker, `NoNewPrivileges`, `ProtectSystem=strict`, and insufficient elevation must disable host mutations without hiding otherwise readable status.

## Verification

- Run `go test ./internal/networkshares` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
