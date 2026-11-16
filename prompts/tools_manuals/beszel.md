# Beszel (`beszel`)

Read the connected Beszel hub using its normal user account. No administration,
alert creation, container commands or additional monitoring loop is available.

| Operation | Arguments | Result |
|---|---|---|
| `list_systems` | optional `limit` (1–50), `offset` | System IDs, names and status |
| `host_metrics` | `system_id` | Latest one-minute host sample |
| `container_metrics` | `system_id` | Latest one-minute container samples |

Every sample includes a timestamp and `stale` indicator (older than five minutes,
missing or invalid time). Never describe stale samples as live measurements.
Host legacy memory/disk values use GiB, container memory MiB; byte fields and
percentage fields have explicit units in the result.

```json
{"action":"beszel","operation":"list_systems"}
```

```json
{"action":"beszel","operation":"host_metrics","system_id":"examplehost001"}
```

Setup: connect a normal Beszel user limited to the intended systems; superuser
authentication is refused. Reuse the Store's optional Beszel agent setup.
