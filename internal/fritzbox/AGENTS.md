# Fritz!Box

## Purpose

TR-064 integration and Desktop widget behavior.

## Ownership

`internal/fritzbox` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Fritz!Box Desktop Widget Contract
- The opt-in `builtin-fritzbox` Desktop widget (hidden by default, added via the widget drawer) is read-only and uses only `GET /api/desktop/fritzbox/overview?sections=system,connection,devices,telephony` (`internal/server/desktop_fritzbox_widget.go`). The route requires the admin desktop scope, Virtual Desktop enabled (503 `desktop_disabled`) and `fritzbox.enabled` with a host (403 `fritzbox_disabled`); it never invokes an LLM and performs no switching actions.
- Section capabilities follow the Fritz!Box feature groups: system = `System.Enabled`, connection = `Network.Enabled`, devices = `Network.Enabled` plus `Hosts` or `WLAN`, telephony = `Telephony.Enabled` plus `CallLists` or `TAM`. Disabled sections are omitted; per-section failures return only the codes `auth_failed`, `timeout`, `unreachable` or `fetch_failed`, never raw errors.
- The payload is sanitized: no MAC addresses, serial numbers, answering-machine file paths/URLs, port forwardings, credentials or raw TR-064 text. Host and caller names are truncated to 48 runes, at most 40 hosts and 8 calls are returned, and bandwidth values are bit/s. The router password stays under Vault key `fritzbox_password`, read only when the widget backend client is rebuilt.
- One shared per-section TTL cache with single-flight serves all widget clients: system 5 min, connection 4 s (waits for a fresh value), devices/telephony 45 s (stale-while-revalidate), 10 s retry window after errors, backend client rebuilt after 10 min or on error, request wait capped at 15 s. Traffic history is a client-only ring buffer seeded from `X_AVM-DE_GetOnlineMonitor`; there is no server-side sampler or persistence.
- WAN reads live in `internal/fritzbox/service_wan.go` (`GetWANStatus`, `GetWANLinkInfo`, `GetOnlineMonitor`; PPP first, then IP connection) and require the Network feature group. Verify with `go test ./internal/fritzbox ./internal/desktop`, `go test ./internal/server -run 'FritzBox'` and `go test ./ui -run TestDesktopFritzBoxWidget`.

## Verification

- Run `go test ./internal/fritzbox` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
