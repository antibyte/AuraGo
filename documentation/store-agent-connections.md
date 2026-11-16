# Store agent connections

Use **Connect to agent** on an installed Software Store app. Finish the app's
own setup first, save a dedicated account or access token, select permissions,
then test the saved connection. The same dialog supports existing volumes.
Installation alone does not enable agent access. Connection tests only read
capabilities and never execute an app action.

Credentials are encrypted in the Vault and responses expose only whether a
credential is stored. Empty credential input preserves it; removal is explicit.
AdGuard and Uptime Kuma retain their existing configuration sections and Vault
keys. Replacing a manual integration requires selecting the Store instance.
Settings use revisions: a stale dialog cannot overwrite a more recent save.

Native AuraGo uses the actual published host port. Containerized AuraGo needs a
selected Docker network shared with the app; an explicit Store update applies
network changes. A remote Docker host requires an address reachable from AuraGo.

Store operations exclude connection changes and tests. Stopping, updating,
removing, changing configuration or revoking permissions cancels active access.
Restart and successful updates recheck the saved connection. Normal uninstall
keeps settings; `delete_data` removes this binding and its owned credentials,
without deleting a different manual integration.

## AdGuard Home

Complete AdGuard's initial setup with permanent admin port **3000**. Supply its
username and password, not a Kuma-style API key. Start with reading enabled;
execution is separate and the existing AdGuard read-only gate still applies.

An existing configuration using admin port **80** requires a Store update to
add its port mapping. Select that admin port and test it. The verified endpoint
is then used when opening the app. No existing AdGuard configuration is reset.

Optional DNS publishing takes a specific LAN address and an explicit Store
update. Both **53/TCP and 53/UDP** use that exact address and port. A Docker bind
conflict fails the update and retains rollback; no random DNS port is chosen.
Router, DHCP and operating-system DNS settings are never changed automatically.

## Uptime Kuma

Create an API key in Kuma and enter it once. AuraGo verifies the existing
authenticated `/metrics` interface. **Report status changes to the agent** is a
separate option, initially off unless an existing choice is preserved. It uses
the existing deduplicated poller and isolated agent session. No Socket.IO account
or key provisioning is performed.

## Administrative API

`GET`, `PUT`, `DELETE /api/desktop/store/apps/{id}/integration` and
`POST /api/desktop/store/apps/{id}/integration/test` retain Desktop admin,
authentication, CSRF and Store operation permissions. `DELETE` disables the
binding; app uninstall with `delete_data` owns permanent cleanup. Agent requests
also obey current Desktop agent-control, network and app-specific grants, even
when the caller captured an older configuration snapshot.
