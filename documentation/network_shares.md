# Local SMB and NFS share management

AuraGo can inspect and manage file-server shares on its own Linux or Windows
host. It performs a passive capability probe at startup and whenever an
administrator selects **Detect again**. Probing does not install packages,
start services, or modify server configuration.

This integration is separate from the TrueNAS integration. It does not mount
remote shares and does not manage remote hosts, users, groups, or passwords.

## Security model

The integration is disabled and read-only by default. Each mutation needs its
own permission in addition to `readonly: false`. Only shares created by AuraGo,
recorded in `data/network_shares.db`, and reconciled with a native marker can be
changed or removed. External shares inside allowed roots are visible as
read-only; shares outside allowed roots are not returned to the agent.

Share directories must already exist. Every path is resolved to an absolute,
canonical directory beneath an allowed root, including symbolic-link checks.
AuraGo never creates or deletes the directory or any file in it.

```yaml
network_shares:
  enabled: false
  readonly: true
  allow_create: false
  allow_update: false
  allow_delete: false
  allowed_roots: []
  smb:
    enabled: true
    allow_guest: false
    allowed_principals: []
  nfs:
    enabled: true
    allowed_clients: []

sqlite:
  network_shares_path: data/network_shares.db
```

Use absolute, distinct `allowed_roots`. Missing or inaccessible roots may be
saved but are reported unavailable at runtime. SMB principals must be existing
OS users or groups. NFS clients must be explicit IP addresses or CIDRs; AuraGo
does not accept host wildcards.

## Linux requirements

### Samba

Install Samba's `net` and `smbcontrol` commands and run an active `smbd`
service. AuraGo uses only Samba registry shares through `net conf`; it does not
edit ordinary share blocks in `smb.conf`. Enable this prerequisite yourself:

```ini
[global]
registry shares = yes
```

Confirm the existing setup before enabling writes:

```bash
net conf listshares
testparm -s --parameter-name="registry shares"
smbcontrol all ping
```

### NFS

Install the distribution's NFS server package, including `exportfs`, and run
the NFS server service. AuraGo owns only files named
`/etc/exports.d/aurago-<id>.exports` and reloads exports with `exportfs -ra`.
Each client receives `sync,root_squash,no_subtree_check` plus `ro` or `rw`.

Common package names are `samba`, `nfs-kernel-server` on Debian/Ubuntu, and
`nfs-utils` on several other distributions. AuraGo deliberately does not
install them.

Linux writes require a root process or AuraGo's existing unrestricted sudo
path. `agent.sudo_enabled` and `agent.sudo_unrestricted` must be enabled and the
runtime may not be constrained by `NoNewPrivileges` or `ProtectSystem=strict`.
The sudo password remains Vault-only.

## Windows requirements

SMB uses the installed `SmbShare` PowerShell module and its
`Get/New/Set/Remove-SmbShare` cmdlets. NFS requires the Windows Server for NFS
feature and its `NFS` module. AuraGo invokes fixed PowerShell scripts with JSON
input rather than constructing PowerShell commands from share values.

Read access depends on the installed feature and module. Create, update, and
delete require AuraGo itself to run in an elevated process. Windows SMB guest
access is intentionally unavailable in this integration. Microsoft NFS host
permissions accept individual host names or IP addresses, not CIDR networks;
because AuraGo intentionally does not create or expand global NFS client
groups, Windows NFS shares must select individual IP entries from
`allowed_clients`. Linux NFS continues to support both individual addresses and
CIDRs.

## Docker and hardened services

The standard AuraGo container reports network-share writes as unavailable.
Host share management crosses the container boundary and is not enabled by the
default Docker deployment. Native services with `NoNewPrivileges` or
`ProtectSystem=strict` can still inspect a readable backend, but cannot mutate
host share configuration.

## Failure handling and drift

A mutation validates the request, saves the observed state, applies and reloads
the native configuration, reads it again, and only then commits the ledger. If
verification fails, AuraGo attempts rollback. A failed rollback marks the share
as drifted and locks further mutations until an administrator repairs the
native state and re-runs detection.

Stable errors include:

- `NETWORK_SHARES_DISABLED`
- `SHARE_PROTOCOL_UNAVAILABLE`
- `SHARE_OUTSIDE_ALLOWED_ROOT`
- `SHARE_READ_ONLY`
- `SHARE_NOT_MANAGED`
- `SHARE_CONFLICT`
- `SHARE_DRIFT`
- `SHARE_APPLY_FAILED`

The Admin UI exposes status, validation, create, edit, and remove actions after
the configuration is saved. Removing a share always leaves its directory and
contents untouched.

## Platform references

- [Samba `net conf`](https://www.samba.org/samba/docs/current/man-html/net.8.html)
- [Samba registry shares](https://www.samba.org/samba/docs/current/man-html/smb.conf.5)
- [Linux `exports(5)`](https://man7.org/linux/man-pages/man5/exports.5.html)
- [Linux `exportfs(8)`](https://man7.org/linux/man-pages/man8/exportfs.8.html)
- [Windows `New-SmbShare`](https://learn.microsoft.com/en-us/powershell/module/smbshare/new-smbshare?view=windowsserver2025-ps)
- [Windows `New-NfsShare`](https://learn.microsoft.com/en-us/powershell/module/nfs/new-nfsshare?view=windowsserver2025-ps)
- [Windows `Grant-NfsSharePermission`](https://learn.microsoft.com/en-us/powershell/module/nfs/grant-nfssharepermission?view=windowsserver2025-ps)
