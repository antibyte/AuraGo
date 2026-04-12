## Tool: Skill Templates

Create new skills from built-in templates instead of writing Python code from scratch.

### List Templates (`list_skill_templates`)

Returns all available templates with their names, descriptions, and expected parameters.

```json
{"action": "list_skill_templates"}
```

### Create Skill from Template (`create_skill_from_template`)

Generate a complete skill (manifest + Python code) from a template. The skill is immediately usable via `execute_skill`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `template` | string | yes | Template name (see below) |
| `name` | string | yes | Unique name for the new skill (e.g. `weather_api`) |
| `description` | string | no | What this skill does |
| `url` | string | no | Base URL (used by api_client, notification_sender) |
| `dependencies` | array | no | Additional pip packages beyond template defaults |
| `vault_keys` | array | no | Vault secret keys the skill needs at runtime |

#### Templates

**api_client** — REST API client with Bearer/Basic/API-Key auth, retry logic, and pagination support.
- Default deps: `requests`
- Vault: `API_KEY` or `USERNAME`+`PASSWORD`, `BASE_URL` (optional)
- Params: `endpoint`, `method`, `body`, `headers`, `auth_type` (bearer/basic/api_key/none), `max_pages`

**data_transformer** — Convert between JSON, CSV, YAML, and XML formats with field filtering, sorting, and limit.
- Default deps: `pyyaml`
- Params: `input_path`, `output_path`, `input_format`, `output_format`, `fields`, `sort_by`, `limit`

**notification_sender** — Send notifications via Telegram, Discord, email (SMTP), or generic webhook.
- Default deps: `requests`
- Vault: channel-specific keys (see below)
- Params: `channel` (telegram/discord/email/webhook), `message`, `title`, `attach`, `priority`
- Required vault keys per channel:
  - Telegram: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`
  - Discord: `DISCORD_WEBHOOK_URL`
  - Email: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `EMAIL_FROM`, `EMAIL_TO`
  - Webhook: `WEBHOOK_URL`, optional `WEBHOOK_KEY`

**monitor_check** — Health check for HTTP endpoints, TCP ports, and DNS resolution.
- Default deps: `requests`
- Params: `target`, `check_type` (http/tcp/dns), `timeout`, `expected`, `keyword`

**log_analyzer** — Parse and analyze log files: filter by time range, severity, pattern; extract errors and summarize.
- No default deps (stdlib only)
- Params: `log_path`, `operation` (summary/errors/search/tail/count_by_level), `pattern`, `since` (e.g. "1h", "24h"), `max_results`

**docker_manager** — Manage Docker containers: list, inspect, start, stop, restart, logs, stats.
- Default deps: `requests`
- Params: `action` (list/inspect/start/stop/restart/logs/stats), `container`, `tail`, `all`

**backup_runner** — Backup files/directories as compressed archives with rotation and integrity check.
- No default deps (stdlib only)
- Params: `action` (create/list/restore/cleanup), `source`, `output`, `keep`, `exclude`

**database_query** — Execute SQL queries against SQLite, PostgreSQL, or MySQL databases.
- No default deps (add `psycopg2-binary` for PostgreSQL, `pymysql` for MySQL)
- Params: `query`, `db_type` (sqlite/postgresql/mysql), `connection`, `params`, `limit`

**ssh_executor** — Execute commands on remote hosts via SSH with key or password auth.
- Default deps: `paramiko`
- Vault: `SSH_KEY` or `SSH_PASSWORD`, optional `SSH_USER`
- Params: `host`, `command`, `user`, `port`, `timeout`

**mqtt_publisher** — Publish and subscribe to MQTT topics for IoT device control and sensor data.
- Default deps: `paho-mqtt`
- Vault: `MQTT_HOST`, optional `MQTT_PORT`, `MQTT_USER`, `MQTT_PASSWORD`
- Params: `action` (publish/subscribe), `topic`, `payload`, `qos`, `retain`, `timeout`

#### Example

```json
{
  "action": "create_skill_from_template",
  "template": "api_client",
  "name": "weather_api",
  "description": "Fetch weather data from OpenWeatherMap",
  "url": "https://api.openweathermap.org/data/2.5",
  "vault_keys": ["API_KEY"]
}
```

After creation, use:
```json
{"action": "execute_skill", "skill": "weather_api", "skill_args": {"endpoint": "weather?q=Berlin", "method": "GET"}}
```

### Vault Secrets — User Action Required

When a skill uses `vault_keys`, the user must manually configure the secrets before the skill can work:

1. **Store secret in vault**: Web UI → Settings → Secrets → New Secret (e.g. name: `API_KEY`, value: the actual key)
2. **Assign secret to skill**: Web UI → Skills → select the skill → Assign Secrets → check the matching vault entries → Save

**Always inform the user** which secrets they need to store and assign. Without this step, the skill will receive empty values and fail.

Example message to the user:
> I created the skill `weather_api`. It requires the vault secret `API_KEY`. Please store your OpenWeatherMap API key in the vault (Settings → Secrets) and then assign it to the skill (Skills → weather_api → Assign Secrets).

### Daemon Skill Templates

Daemon skills are long-running background processes managed by the Daemon Supervisor. They run continuously and wake the agent when events occur.

**daemon_monitor** — Periodically checks a resource (disk, CPU, service, URL) and wakes the agent on threshold violations.
- Params: `target`, `threshold`, `interval`, `alert_severity`

**daemon_watcher** — Watches a directory for file changes and wakes the agent on create/modify/delete events.
- Params: `watch_path`, `patterns`, `events`, `cooldown`, `recursive`

**daemon_listener** — Listens on a Unix domain socket or named pipe for external events and forwards them to the agent.
- Params: `socket_path`, `protocol`, `max_clients`

**daemon_mission** — Monitors a backup directory or status file and emits events that can trigger a follow-up mission.
- Params: `watch_dir`, `status_file`, `backup_pattern`, `check_interval`, `cooldown`

#### Daemon Example

```json
{
  "action": "create_skill_from_template",
  "template": "daemon_monitor",
  "name": "disk_monitor",
  "description": "Alert when disk usage exceeds 90%"
}
```

After creation, configure daemon settings (wake_agent, trigger_mission_id, etc.) via the Web UI or daemon supervisor.
