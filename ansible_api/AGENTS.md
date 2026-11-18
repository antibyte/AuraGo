# Ansible sidecar

## Purpose

Authenticated Ansible execution for the managed and explicitly configured sidecar.

## Ownership

Own Python admission, child environment, input staging and response projection. Go callers and managed container reconciliation belong to `internal/tools`; config reload belongs to `internal/server`.

## Local Contracts

- Inventories are non-executable, regular UTF-8 static INI/YAML files, at most 1 MiB, rooted under the playbook directory. Only the exact configured default inventory may be outside it. Reject links, path escapes, dynamic plugins and templates before execution. Validate decoded inventory exports too, including escaped template syntax.
- An empty default uses a private localhost inventory, never an implicit file outside the playbook root.
- Run private inventory snapshots with only INI/YAML inventory parsers and an allowlisted environment; never pass the API token to children. Inventory results contain group/host structure and approved connection fields only, never arbitrary variables or raw errors.
- In both modes and with or without read-only, extra variables are literal JSON objects; reject templates, reserved Ansible engine/connection keys and file references. Resolve host patterns using the private inventory snapshot and require every selected host to be explicitly declared. Empty matches fail; implicit localhost is unavailable unless explicitly inventoried, including the generated empty-default inventory.
- `ANSIBLE_READONLY=true` denies all playbook runs, including check/dry-run, and arbitrary ad-hoc modules. Ping/setup remain read operations. Both local and sidecar execution validate read-only module arguments before starting a child. Reject templates in raw arguments and decoded JSON. Use JSON for values that need backslash escapes.
- `/status` advertises `security_policy: 4`. Go callers reject older sidecars for inventory/execution. Managed images carry `io.aurago.ansible.security-policy=4`. Verify or rebuild the image before replacing a container, including token rotation. Build failure retains the existing container. External images need the label and the policy implementation. Recreated containers retain configured mounts and the current Vault token.
- Inventory export, host-list preflight and execution share one timeout and sanitized environment. Playbooks remain trusted operator code and require write permission; the input checks do not sandbox arbitrary playbook tasks.

## Work Guidance

- Keep Go local execution, sidecar policy and managed container environment aligned. Updating an existing sidecar requires a rebuilt image; do not silently adopt an incompatible running image.

## Verification

- `python -B -m unittest discover -s ansible_api`; Go `TestAnsible*` and A2A tests. Both languages consume `policy_cases.json`.
- `AURAGO_RUN_ANSIBLE_SMOKE=1` requires real Ansible executables and runs Python and Go local acceptance; missing binaries fail. Security Gates builds the Ansible image and runs the real Python suite without network access. POSIX no-follow/FIFO behavior needs Linux acceptance.

## Child DOX Index

None.
