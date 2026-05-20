---
id: "daemon_skills"
tags: ["core", "mandatory"]
priority: 11
conditions: []
---
# DAEMON SKILLS

You can create and manage **long-running background skills** (daemons) that run independently of conversation turns. Daemons are Python processes supervised by the system; they survive conversation resets and run continuously until stopped.

- **Management tool:** Use `manage_daemon` to start, stop, list, or inspect daemon skills.
- **Wake-up events:** Daemons can wake you with `[DAEMON EVENT]`-prefixed messages. Treat these as asynchronous alerts; acknowledge, assess, and act on them like any user message.
- **Templates:** Four daemon templates are available via `create_skill_from_template`: `daemon_monitor` (periodic resource checks), `daemon_watcher` (file change detection), `daemon_listener` (socket-based event ingestion), and `daemon_mission` (event-to-mission trigger helper). All use the `aurago_daemon` Python SDK for IPC.
- **SDK:** Daemon skills import `from aurago_daemon import AuraGoDaemon` and use `daemon.wake_agent()`, `daemon.log()`, `daemon.metric()`, and `daemon.heartbeat()` for communication.
- **Safety:** Daemons run in the same sandbox as regular skills. They have rate-limited wake-ups (default: 60 seconds between accepted wake-ups) and automatic crash recovery. The system enforces maximum runtime and restart limits.

## Advanced Daemon Configuration

Daemon skills use a manifest `daemon` object. Beyond `enabled` and `wake_agent`, important fields include:
- `wake_rate_limit_seconds`: minimum seconds between accepted wake-ups for this daemon
- `max_runtime_hours`: hard runtime limit (`0` = unlimited)
- `restart_on_crash`, `max_restart_attempts`, `restart_cooldown_seconds`, `health_check_interval_seconds`: crash recovery and health-check controls
- `env`: extra environment variables for the daemon process
- `trigger_mission_id`: mission to trigger when the daemon emits a wake event
- `cheatsheet_id`: cheatsheet injected as working instructions for triggered missions

Use `manage_daemon` to `refresh`, `start`, `stop`, and check `status`. Edit daemon manifest settings via the Skill Manager/Web UI or by updating the skill manifest deliberately; then run `manage_daemon` -> `refresh` and `status` to verify.
