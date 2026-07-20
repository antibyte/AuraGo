---
id: "tool_network_shares"
tags: ["tool", "network", "smb", "nfs", "shares"]
priority: 50
---
# Local network shares

Inspect and manage SMB and NFS server shares on the local AuraGo host. The tool
is registered only when the saved configuration is enabled and at least one
protocol is readable. Its operation enum is reduced at runtime to the actions
permitted by both policy and host capabilities.

## Operations

- `status`: Return the passive SMB/NFS capability snapshot and allowed-root
  availability.
- `list`: List visible file shares inside allowed roots. Optional `protocol` and
  `managed` filters are supported.
- `get`: Return one share by stable `id`.
- `create`: Create an AuraGo-managed share for an existing directory.
- `update`: Change only `comment`, `read_only`, and access settings.
- `delete`: Remove only the share definition. It never removes the directory or
  any file.

Create, update, and delete appear only when their matching permission is
enabled, `network_shares.readonly` is false, and the selected host backend is
writable. AuraGo never installs packages, starts services, creates directories,
or manages operating-system accounts.

## SMB access

`acl` entries contain a configured, existing OS `principal` and one `level`:
`read`, `change`, `full`, or `deny`. `guest: true` is accepted only when the
administrator enabled guest access. Passwords are never accepted.

```json
{"action":"network_shares","operation":"create","protocol":"smb","name":"media","path":"/srv/shares/media","comment":"Media library","read_only":true,"guest":false,"acl":[{"principal":"media-readers","level":"read"}]}
```

## NFS access

`clients` contains only IP addresses or canonical CIDRs from the configured
allowlist. Free-form export options, wildcards, and `no_root_squash` are not
accepted. AuraGo always applies `sync,root_squash,no_subtree_check` plus `ro` or
`rw`. Linux accepts addresses and CIDRs. Windows NFS host permissions accept
individual IP addresses only; CIDRs are rejected because AuraGo does not create
or expand global Windows NFS client groups.

```json
{"action":"network_shares","operation":"create","protocol":"nfs","name":"backups","path":"/srv/shares/backups","read_only":false,"clients":["192.168.10.0/24"]}
```

Use an explicit remove-and-create sequence to change a name or path. Do not
attempt to update external, unmarked, or drifted shares.
