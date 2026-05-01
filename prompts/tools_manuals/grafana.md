# Grafana (`grafana`)

Read observability information from a configured Grafana instance using a vault-stored API key. The integration is read-only in v1.

## Operations

| Operation | Purpose |
| --- | --- |
| `health` | Check Grafana health, database status, and version |
| `list_dashboards` | List dashboards, optionally filtered by `query` |
| `get_dashboard` | Fetch a dashboard by `uid` |
| `list_datasources` | List configured data sources |
| `query` | Run a read query against a data source by `datasource_id` and `query` |
| `list_alerts` | List legacy alert states exposed by Grafana |
| `get_org` | Read current organization metadata |

## Parameters

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| `operation` | string | yes | One of the operations above |
| `query` | string | for `query`, optional for `list_dashboards` | Dashboard search text or data source expression |
| `uid` | string | for `get_dashboard` | Grafana dashboard UID |
| `datasource_id` | integer | for `query` | Numeric Grafana data source ID |

## Examples

```json
{"action":"grafana","operation":"health"}
```

```json
{"action":"grafana","operation":"list_dashboards","query":"system"}
```

```json
{"action":"grafana","operation":"get_dashboard","uid":"system-overview"}
```

```json
{"action":"grafana","operation":"query","datasource_id":1,"query":"up"}
```

## Configuration

```yaml
grafana:
  enabled: true
  base_url: "http://grafana.local:3000"
  readonly: true
  insecure_ssl: false
  request_timeout: 15
```

The API key is stored in the encrypted vault under `grafana_api_key` and is sent as `Authorization: Bearer <key>`.
