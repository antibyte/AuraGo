# Grafana (`grafana`)

Read observability information from a configured Grafana instance using a vault-stored API key. The integration is read-only in v1.

## Operations

| Operation | Purpose |
| --- | --- |
| `health` | Check Grafana health, database status, and version |
| `list_dashboards` | List dashboards, optionally filtered by `query`; supports `limit` and `page` |
| `get_dashboard` | Fetch a dashboard by `uid` |
| `list_datasources` | List configured data sources |
| `query` | Run a read query against a data source by `datasource_uid` or `datasource_id` and `query`; supports time range and rendering controls |
| `list_alerts` | List active alert instances and configured alert rules, with legacy fallback |
| `get_org` | Read current organization metadata |

## Parameters

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| `operation` | string | yes | One of the operations above |
| `query` | string | for `query`, optional for `list_dashboards` | Dashboard search text or data source expression |
| `uid` | string | for `get_dashboard` | Grafana dashboard UID |
| `datasource_uid` | string | for `query` unless using `datasource_id` | Stable Grafana data source UID; prefer this over numeric IDs |
| `datasource_id` | integer | for `query` unless using `datasource_uid` | Numeric Grafana data source ID |
| `datasource_type` | string | optional for `query` | Payload mapping for `prometheus`, `mimir`, `cortex`, `loki`, or `elasticsearch`; defaults to Prometheus-style `expr` |
| `from` | string | optional for `query` | Query range start such as `now-1h` or an epoch millisecond timestamp; default `now-1h` |
| `to` | string | optional for `query` | Query range end such as `now` or an epoch millisecond timestamp; default `now` |
| `format` | string | optional for `query` | Result format such as `time_series` or `table` |
| `max_data_points` | integer | optional for `query` | Maximum rendered points for the query response |
| `interval_ms` | integer | optional for `query` | Query interval in milliseconds |
| `limit` | integer | optional for `list_dashboards` | Max dashboards to return; default 50, maximum 200 |
| `page` | integer | optional for `list_dashboards` | Dashboard search result page; default 1 |

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
{"action":"grafana","operation":"list_dashboards","query":"system","limit":25,"page":1}
```

```json
{"action":"grafana","operation":"query","datasource_uid":"prometheus-main","datasource_type":"prometheus","query":"up","from":"now-15m","to":"now","format":"time_series","max_data_points":400,"interval_ms":30000}
```

For Loki, use `datasource_type:"loki"` and pass the LogQL expression in `query`. For SQL-style data sources, first inspect `list_datasources`; raw datasource-specific payloads are not part of this v1 read interface.

`list_alerts` returns both active alert instances from Grafana's Prometheus-compatible alert endpoint and configured Grafana-managed alert rules from the provisioning API when available. Use the returned `source` field to distinguish `prometheus_alerts`, `alert_rules`, and `legacy_alerts`. If one modern alert endpoint fails while another alert source still returns data, the response includes `partial_errors` with endpoint diagnostics.

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
