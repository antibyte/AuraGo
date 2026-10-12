# MeshCentral

## Purpose

Control-WebSocket client for MeshCentral device inventory, events, wake, power and MeshAgent commands. Desktop relay, interactive shell and `meshrelay.ashx` stay unimplemented.

## Ownership

`internal/meshcentral` owns the client, TLS trust and command limits. Dispatch lives in `internal/agent`. Config loading and the power-action grandfather live in `internal/config`. `POST /api/meshcentral/test` lives in `internal/server`. Help text lives in `ui/lang/help/` for all 16 locales. The contracts below also bind those paths through the root routing table.

## Local Contracts

- Reads are `server_info`, `list_groups`, `list_devices`, `device_info` and `list_events`. Mutations are `wake`, `power_action` and `run_command`. `shell` stays unsupported.
- `meshcentral.enabled` gates the integration. `meshcentral.readonly` blocks every mutation. Wake and power also require `meshcentral.allow_power_actions`. `run_command` also requires `agent.allow_remote_shell`. `blocked_operations` still applies. Check these before resolving credentials or dialing.
- New installs and `config_template.yaml` set `readonly: true` and `allow_power_actions: false`. If the key is absent on an already enabled, writable integration, load `AllowPowerActions` as true and warn once. A written value, including false, ends that grandfathering.
- Vault `meshcentral_password` holds the account password or the login-token secret. Vault `meshcentral_token` holds the token name, not the secret. A token name with an empty secret fails before any dial with `MeshCentral login token requires meshcentral_password to hold the token secret`, in `ConnectContext`, dispatch and the test handler.
- Wake and power use a correlated `responseid` and wait `meshConfirmTimeout` (5s). A `result` string, or `ok` when that field is absent, is success. Timeout is the client error `meshcentral: unconfirmed` and dispatch status `unconfirmed` with the message that the request may already have reached the device and must not be retried blindly. Connection errors stay errors and are not replayed. Context cancellation stays `ctx.Err()`.
- `run_command` trims the command, rejects empty text and text longer than `MaxCommandBytes` (4096) before send, and keeps `runAsUser` 0 (MeshAgent service account: SYSTEM or root). The WebSocket read limit is 1 MiB. Tool results larger than 64 KiB return `truncated: true` and a cut `data` string. A command timeout says the command may still be running and must not be retried blindly.
- TLS uses `InsecureSkipVerify` only when `insecure` is true. Otherwise `ca_file`, when set, must be a PEM file that yields at least one certificate; a missing or empty file is a connect error. `insecure: true` ignores `ca_file`. Plain `http://` stays allowed. Warn once per client for insecure mode, an ignored CA file, and plaintext HTTP.
- Thread the CA path with `SetCAFile` on `Client` and `CachedClient`. Do not change `NewClient` or `NewCachedClient` signatures. The process cache stores `caFile` on its config so a CA change rebuilds the client. `GetClientContext` calls the inner `Client.SetCAFile` while holding the cache mutex; do not call `CachedClient.SetCAFile` under that same lock.
- An action-only response (no `responseid`) is delivered only when exactly one waiter exists for that action. Several waiters log `Ambiguous action-only response` and stay unmatched.
- Connection retry matches `errNotConnected`, a `net.Error`, or the phrases `websocket disconnected`, `not connected`, `use of closed network connection`, `broken pipe` and `connection reset`. Do not match the bare substring `websocket`.
- `POST /api/meshcentral/test` uses `requireAdmin`. A URL override must match the configured origin. Vault fallback applies only for that same origin. An empty configured URL uses `security.ValidateSSRF`. The handler applies `SetCAFile` from the saved config and uses the same token-secret check before dialing.
- Tool output stays classified as external data and isolated. Mutations go through `executeMutation` and are not replayed. Reads use `executeWithRetry`.
- The trust wizard sets `readonly` for trust levels 2 and below and `allow_power_actions` for level 3 and above.

## Work Guidance

- Do not add a device allowlist, a `run_as_user` switch, interactive shell, desktop relay, file transfer or MeshCentral user administration without a new contract.
- Do not edit a user's local `config.yaml` to apply these defaults. The template and the grandfather own that migration.
- Help keys `help.meshcentral.readonly`, `allow_power_actions`, `ca_file` and `insecure` stay translated in all 16 `ui/lang/help/<locale>.json` files. Do not copy English into other locales.

## Verification

- `go test ./internal/meshcentral/ ./internal/config/ ./internal/agent/ ./internal/server/ -count=1 -run "MeshCentral|Grandfather|IntegrationTestRoutesRequireAdmin"`
- Full package: `go test ./internal/meshcentral/ -count=1`
- Dispatch file: `go test ./internal/agent/ -count=1 -run MeshCentral`
- No live MeshCentral server is required. Do not press the Config test button against a real server as part of this check.

## Child DOX Index

None.
