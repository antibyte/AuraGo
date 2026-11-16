# Ansible sidecar

## Purpose

Authenticated Ansible execution for the managed and explicitly configured sidecar.

## Ownership

Own Python admission, child environment, input staging and response projection. Go callers and managed container reconciliation belong to `internal/tools`; config reload belongs to `internal/server`.

## Local Contracts

- Inventories are non-executable, regular static INI/YAML files, at most 1 MiB, rooted under the playbook directory. Only the exact configured default inventory may be outside it. Reject links, path escapes and dynamic plugins before spawning.
- An empty default uses a private localhost inventory, never an implicit file outside the playbook root.
- Run private inventory snapshots with only INI/YAML inventory parsers and an allowlisted environment; never pass the API token to children. Inventory results contain group/host structure and approved connection fields only, never arbitrary variables or raw errors.
- `ANSIBLE_READONLY=true` denies all playbook runs, including check/dry-run, and arbitrary ad-hoc modules. Ping/setup remain read operations. `/status` advertises `security_policy: 2`; Go callers reject older sidecars for inventory/execution.

## Work Guidance

- Keep Go local execution, sidecar policy and managed container environment aligned. Updating an existing sidecar requires a rebuilt image; do not silently adopt an incompatible running image.

## Verification

- `python -m unittest discover -s ansible_api`; Go Ansible tool, dispatch and config tests. POSIX no-follow/FIFO behavior needs Linux acceptance.

## Child DOX Index

None.
