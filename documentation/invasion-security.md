# Egg security cutover

Master and Eggs must be upgraded together to Bridge protocol 3. Older peers are
disconnected with the existing upgrade notice. Generated configurations require
`egg_mode.security_policy_version: 1`; do not add it to a legacy file by hand.

Before hatching, review the Egg's tool list and all grants. Empty or `[]` permits
neither shell nor Python. File writes, network requests and shell without isolation
default off. Host shell access additionally requires a shell tool selection.
Python uses the configured isolation unless host access was explicitly granted.
Safe reconfiguration preserves applied permission snapshots; unchecked controls
and an empty tool list revoke rights. Legacy revision rollback is refused.

Use Docker over TLS/mTLS, Docker over SSH, a local Docker socket or SSH deployment.
Secret bootstrap over remote unencrypted Docker HTTP is refused. An unreadable
master certificate blocks hatching; repair the certificate before retrying.
Certificate pins and host-trusted certificate chains remain supported.

Generated YAML contains no master key, shared HMAC key or provider API key. The
provider binding uses the normal provider system. A temporary private bootstrap
imports selected Vault entries and provider/bridge secrets atomically before the
Egg starts working, then removes itself. Import or cleanup failure blocks startup.
The existing Egg Vault and its actual local master key are retained. Restore a
missing original key only through the authenticated management path. Never replace
it with a new key. `include_vault` still exports only the selected Egg/nest entries.

A saved Egg provider ID resolves its type, URL, model, capabilities and matching
Vault key. Explicit per-Egg key references take precedence. OAuth sessions and
the master's managed-local provider cannot be copied to another machine; select
a separately reachable static provider for that Egg.

Reconfiguration and rollback use the same provider binding and private key
selection. Later partial edits retain the chosen connection settings. Permission
snapshots belong to one nest/Egg pair; reassigning a nest requires a new explicit
permission review, and another Egg's revision cannot be rolled back onto it.

Back up the installation before the coordinated upgrade. New Invasion database
columns are migrated only after a consistent private backup succeeds. Binary
rollback cannot start unmarked legacy backups. Existing remote machines are not
automatically reconfigured; disconnected Eggs remain unavailable until upgraded
and safely re-hatched. This cutover requires acceptance on real Docker and SSH
Eggs; local protocol fixtures do not prove target-host operation.
