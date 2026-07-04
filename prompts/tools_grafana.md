---
id: "tools_grafana"
conditions: ["grafana_enabled"]
---

### Grafana

Read-only Grafana observability access.

| Tool | Description |
| --- | --- |
| `grafana` (operation=`health`) | Check Grafana health and version |
| `grafana` (operation=`list_dashboards`) | List dashboards, optionally filtered by `query`, `limit`, and `page` |
| `grafana` (operation=`get_dashboard`) | Read a dashboard by `uid` |
| `grafana` (operation=`list_datasources`) | List configured data sources |
| `grafana` (operation=`query`) | Run a read query using `datasource_uid` or `datasource_id`, plus `query` and optional time/range controls |
| `grafana` (operation=`list_alerts`) | List active alert instances and configured alert rules with legacy fallback |
| `grafana` (operation=`get_org`) | Read current organization information |

Notes:
- The integration is read-only. Do not attempt to create or modify dashboards, alerts, or data sources.
- Prefer `datasource_uid` over numeric `datasource_id` when querying a data source.
- `query` maps simple text expressions for `prometheus`, `mimir`, `cortex`, `loki`, and `elasticsearch`. Use `datasource_type` when the source is not Prometheus-compatible.
- `query` defaults to `from:"now-1h"` and `to:"now"`; set `format`, `max_data_points`, and `interval_ms` when the data source needs a specific response shape.
- `list_dashboards` defaults to `limit: 50` and caps at `limit: 200`.
