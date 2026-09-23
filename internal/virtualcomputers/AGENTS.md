# Virtual Computers

## Purpose

Workspace lease and managed Garage storage lifecycle.

## Ownership

`internal/virtualcomputers` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Virtual Computers Storage / Managed Garage Contract
- Workspace close treats boringd's JSON `404 {"error":"not found"}` as completed deletion, clears stale errors, closes jobs/browser sessions/grants and resolves only that workspace's `lease_close_failed` issue. Router/proxy 404s, authentication failures and server errors remain failures; closed workspaces must leave lease reconciliation.
- Default `virtual_computers.storage.mode` is `managed_garage`; `external_s3` remains supported. Legacy configs without `mode` normalize to `external_s3` when an endpoint is set, otherwise `managed_garage`.
- Managed Garage runs on the boringd control-plane host (`local_host` or `ssh_host`), never as a general AuraGo Compose service. Image is pinned `dxflrs/garage:v2.3.0@sha256:866bd13ed2038ba7e7190e840482bc27234c4afaf77be8cfa439ae088c1e4690`. Only S3 binds `127.0.0.1:3900`. Data lives under `${install_dir}/data/sidecars/garage`.
- Docker is required for managed volumes but is not installed by AuraGo. Missing Docker is a storage warning and must not set core preflight `Supported=false`.
- Managed Garage Vault keys (`virtual_computers_garage_*`) are separate from external S3 keys. Never export them to Python/skills; never show them in the UI. Stop without delete retains container data and source objects.
- Storage identity includes mode, endpoint/bucket/region/SSL and, for managed Garage, control-plane mode/host/install_dir. Source objects are never auto-deleted on switch. `previous_store` ledger volumes must not be removed by normal JSON-404 cleanup.
- Config save must return HTTP 409 (`storage_switch_required`) when identity changes while available volumes exist, unless a single-use `X-AuraGo-Storage-Switch-Token` from `/api/virtual-computers/storage/switch/authorize` matches the target identity hash. Switch-without-migration marks volumes `previous_store` and may stop managed Garage; automated object-copy migration is optional/not required for the gate.
- Agent Docker tools must hide and block lifecycle/inspect/exec/mount access to `aurago-boring-garage` and Garage data paths, same fail-closed pattern as Local LLM.

## Verification

- Run `go test ./internal/virtualcomputers` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
