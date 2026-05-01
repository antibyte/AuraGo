---
id: "tools_grafana"
conditions: ["grafana_enabled"]
---

### Grafana

Read-only Grafana observability access.

| Tool | Description |
| --- | --- |
| `grafana` (operation=`health`) | Check Grafana health and version |
| `grafana` (operation=`list_dashboards`) | List dashboards, optionally filtered by `query` |
| `grafana` (operation=`get_dashboard`) | Read a dashboard by `uid` |
| `grafana` (operation=`list_datasources`) | List configured data sources |
| `grafana` (operation=`query`) | Run a read query using `datasource_id` and `query` |
| `grafana` (operation=`list_alerts`) | List alert states |
| `grafana` (operation=`get_org`) | Read current organization information |

Notes:
- The integration is read-only. Do not attempt to create or modify dashboards, alerts, or data sources.
- `query` expects a Grafana `/api/ds/query` compatible expression for the selected data source.
