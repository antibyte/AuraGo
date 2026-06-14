# Internal Tools

This documentation lists all internal tools that AuraGo makes available to the agent. These tools are invoked by the LLM via native function calling.

> 📅 **Updated:** June 2026
> 🔢 **Count:** 100+ Tools

---

## Table of Contents

1. [Skills & Code Execution](#skills--code-execution)
2. [Filesystem](#filesystem)
3. [System & Processes](#system--processes)
4. [Memory](#memory)
5. [Communication & Media](#communication--media)
6. [Device Management](#device-management)
7. [Integrations (Smart Home)](#integrations-smart-home)
8. [Integrations (Cloud & APIs)](#integrations-cloud--apis)
9. [Network & Security](#network--security)
10. [Web & Scraping](#web--scraping)
11. [Documents & Media Processing](#documents--media-processing)
12. [Databases](#databases)
13. [Infrastructure](#infrastructure)

---

## Skills & Code Execution

### `list_skills`
List available pre-built skills and integrations.

### `execute_skill`
Execute a registered skill (e.g., web search, PDF extraction, VirusTotal scan).

| Parameter | Type | Description |
|-----------|------|-------------|
| `skill` | string | Name of the skill |
| `skill_args` | object | Arguments for the skill |
| `vault_keys` | array | Vault secret keys for Python |

### `execute_python`
Execute Python code on the host system.

| Parameter | Type | Description |
|-----------|------|-------------|
| `code` | string | Python code |
| `description` | string | Brief description |
| `background` | boolean | Run as background process |

### `execute_sandbox` ⭐
Execute code in an isolated Docker sandbox (safer than `execute_python`).

| Parameter | Type | Description |
|-----------|------|-------------|
| `code` | string | Source code |
| `sandbox_lang` | string | Language: python, javascript, go, java, cpp, r |
| `libraries` | array | Additional packages |

### `save_tool`
Save a new Python tool to the tools directory.

### `list_skill_templates`
List available skill templates.

### `create_skill_from_template`
Create a skill from a template (api_client, file_processor, data_transformer, scraper).

### `golangci_lint`
Run golangci-lint on Go code (code quality checks).

### `get_skill_documentation`
Retrieve documentation for a skill (description, parameters, examples).

### `set_skill_documentation`
Set or update documentation for a skill.

### `invoke_tool`
Invoke a dynamic tool or skill directly by name.

| Parameter | Type | Description |
|-----------|------|-------------|
| `tool_name` | string | Name of the tool/skill |
| `arguments` | object | Arguments as JSON object |

### `list_agent_skills`
List enabled Agent Skills packages.

| Parameter | Type | Description |
|-----------|------|-------------|
| `search` | string | Optional search term for Agent Skill name or description |

### `activate_agent_skill`
Load full SKILL.md instructions for an enabled Agent Skill package.

| Parameter | Type | Description |
|-----------|------|-------------|
| `skill` | string | Agent Skill name to activate |
| `name` | string | Alias for skill |

### `run_agent_skill_script`
Run an approved Python script from an enabled Agent Skill package.

| Parameter | Type | Description |
|-----------|------|-------------|
| `skill` | string | Agent Skill name |
| `name` | string | Alias for skill |
| `script` | string | Script path under scripts/, e.g. scripts/analyze.py |
| `args` | object | JSON arguments sent to the script on stdin |

### `run_tool`
Run a saved custom Python tool from the agent tools directory.

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Custom tool filename or registered manifest name to run |
| `args` | array | Optional positional command-line arguments for the tool |
| `params` | object | Optional structured parameters; forwarded to the tool as one JSON argument |
| `background` | boolean | Run as background process (default false) |
| `vault_keys` | array | List of vault secret key names to inject as environment variables |
| `credential_ids` | array | List of credential UUIDs to inject as environment variables |

---

## Filesystem

### `filesystem`
Filesystem operations (read/write/move/copy/delete/list).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | read_file, write_file, delete, move, list_dir, create_dir, stat |
| `file_path` | string | Path to file |
| `content` | string | Content (for write_file) |

### `file_search`
Search text in files (grep, find).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | grep, grep_recursive, find |
| `pattern` | string | Regex search pattern |
| `glob` | string | File pattern (*.go) |

### `file_reader_advanced`
Advanced file reading (head, tail, line ranges).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | read_lines, head, tail, count_lines, search_context |
| `file_path` | string | File path |
| `start_line` | integer | Start line |
| `end_line` | integer | End line |

### `smart_file_read`
Intelligent reading of large files (analyze, sample, summarize).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | analyze, sample, structure, summarize |
| `file_path` | string | File path |
| `sampling_strategy` | enum | head, tail, distributed, semantic |

### `text_diff`
Compare two text strings and show differences.

### `discover_tools`
List all currently available tools for the agent (context-aware).

### `file_editor`
Precise text file editing (str_replace, insert, delete).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | str_replace, str_replace_all, insert_after, insert_before, append, prepend, delete_lines |
| `file_path` | string | File path |
| `old` | string | Text to find |
| `new` | string | Replacement text |

### `json_editor`
Read/modify JSON files (get, set, delete, validate).

### `yaml_editor`
Read/modify YAML files.

### `toml_editor`
Read and modify TOML files (get, set, delete, validate).

### `xml_editor`
Edit XML files with XPath.

### `archive`
Create/extract/list ZIP/TAR.GZ archives.

### `detect_file_type`
Detect file type via magic bytes (ignores extension).

---

## System & Processes

### `system_metrics`
Retrieve system resources (CPU, RAM, Disk, Network, Temperatures).

| Parameter | Type | Description |
|-----------|------|-------------|
| `target` | enum | all, cpu, memory, disk, disk_io, processes, connections, host, sensors, network_detail |

### `process_analyzer`
Analyze running processes (Top CPU/Memory, trees, details).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | top_cpu, top_memory, find, tree, info |
| `name` | string | Process name (for find) |
| `pid` | integer | Process ID |

### `process_management`
Manage background processes (list, kill, status).

### `execute_shell`
Execute shell commands (only if `allow_shell` enabled).

### `execute_sudo`
Execute commands with sudo privileges (only if `sudo_enabled`).

### `follow_up`
Schedule an autonomous background task.

### `wait_for_event`
Wait asynchronously for events (process exit, HTTP available, file change).

### `package_manager`
Detect, search, install, remove, update and inspect OS packages.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | detect, install, remove, update, upgrade, search, list_installed, info |
| `package` | string | Package name. Required for install, remove, search, and info |
| `manager` | string | Optional package manager override: apt, dnf, yum, pacman, zypper, apk, brew, winget, choco, or scoop |

---

## Memory

### `manage_memory`
Manage core memory (permanent facts).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | add, update, delete, remove, list |
| `fact` | string | Factual content |
| `id` | string | Fact ID |

### `query_memory`
Search all memory sources (Vector DB, Knowledge Graph, Journal, Notes).

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query |
| `sources` | array | activity, vector_db, knowledge_graph, journal, notes, core_memory |
| `limit` | integer | Max results |

### `retrieve_original_output`
Retrieve the **original uncompressed output** of a prior tool call when output compression truncated important details.

| Parameter | Type | Description |
|-----------|------|-------------|
| `tool_call_id` | string | ID of the compressed tool result to expand |
| `reason` | string | Why the original is needed (improves compression filters) |

**Config:** Works when `agent.output_compression` is enabled (default: true). See [Output Compression](../../output_compression.md) and ch. 08 Integrations.

### `context_memory`
Context-aware memory query with time window.

### `remember`
Automatically save to the right memory source.

| Parameter | Type | Description |
|-----------|------|-------------|
| `content` | string | Information to store |
| `category` | enum | fact, event, task, relationship |
| `title` | string | Title |

### `memory_reflect`
Reflect on memory activity (patterns, contradictions).

### `knowledge_graph`
Structured graph of entities and relationships.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | add_node, add_edge, delete_node, delete_edge, search |
| `id` | string | Node ID |
| `source` | string | Source node (for edges) |
| `target` | string | Target node |
| `relation` | string | Relationship type |

### `cheatsheet`
Manage cheat sheets (step-by-step instructions).

### `manage_notes`
Manage notes and to-dos.

### `manage_journal`
Manage journal entries (events, milestones).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | add, list, search, delete, get_summary |
| `title` | string | Title |
| `entry_type` | enum | activity, reflection, milestone, preference, task_completed |
| `tags` | string | Comma-separated tags |
| `importance` | integer | 1-5 |

### `secrets_vault`
Store/retrieve encrypted secrets in the vault.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | get, set, delete, list |
| `key` | string | Secret name |
| `value` | string | Secret value |

### `cron_scheduler`
Schedule recurring cron jobs.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | add, list, remove, enable, disable |
| `cron_expr` | string | Cron expression |
| `task_prompt` | string | Task |
| `label` | string | Label |

### `manage_missions`
Manage Mission Control background tasks.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | add, list, update, delete, run |
| `title` | string | Mission name |
| `command` | string | Task prompt |
| `cron_expr` | string | Cron expression |

### `manage_plan`
Create, inspect, and update structured work plans for complex multi-step tasks.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | create, list, get, update_task, advance, set_status, set_blocker, clear_blocker, append_note, attach_artifact, split_task, reorder_tasks, archive_completed, delete |
| `id` | string | Plan ID |
| `title` | string | Plan title (required for create) |
| `task_id` | string | Task ID for task-level operations |
| `items` | array | Tasks for create, split_task, or reorder_tasks |
| `status` | enum | draft, active, paused, blocked, completed, cancelled, pending, in_progress, failed, skipped |

### `manage_appointments`
Manage appointments (create, update, delete, list).

### `manage_todos`
Manage to-dos (create, update, delete, list).

### `manage_daemon`
Manage long-running daemon skills (`tools.daemon_skills.enabled`).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, status, start, stop, reenable, refresh |
| `skill_id` | string | Skill ID (required for status, start, stop, reenable) |

### `context_manager`
Manage session contexts and switch between them.

---

## Communication & Media

### `send_image`
Send image to user (local path or URL).

### `send_audio`
Send audio file (inline player).

### `send_video`
Send a local or remote video file to the user. Supported browser-playable formats are copied or downloaded to the generated video area, registered in the media registry, and rendered as an inline video player in the Web UI.

### `send_document`
Send document (with Open/Download buttons).

### `analyze_image`
Analyze image (Vision LLM for OCR, object detection).

### `transcribe_audio`
Transcribe audio to text.

### `generate_image`
Generate AI images (text-to-image, image-to-image).

| Parameter | Type | Description |
|-----------|------|-------------|
| `prompt` | string | Image description |
| `size` | string | 1024x1024, 1344x768, 768x1344 |
| `quality` | enum | standard, hd |
| `style` | enum | natural, vivid |

### `media_registry`
Browse/manage media registry (images, TTS, audio).

### `generate_music`
Generate AI music (text-to-music).

### `generate_video`
Generate short AI videos from text prompts or provider-supported image guidance. Supports configured MiniMax Hailuo and Google Veo providers, async polling, provider-specific model options, daily limits, and automatic Media Registry registration.

### `question_user`
Ask the user a targeted question and wait for a response. Useful for unclear requirements or when a decision is needed.

| Parameter | Type | Description |
|-----------|------|-------------|
| `question` | string | The question to ask |
| `options` | array | Optional choice options |

### `send_telegram`
Send a message or media via the configured Telegram bot.

| Parameter | Type | Description |
|-----------|------|-------------|
| `message` | string | Message text |
| `title` | string | Optional: title for the message |
| `priority` | string | Optional: priority (normal, high, low) |

### `send_agodesk_chat`
Send proactive text to a connected **AgoDesk/AgoChat** desktop client.

| Parameter | Type | Description |
|-----------|------|-------------|
| `device_id` | string | Connected RemoteHub device ID (from REACHABLE CHAT CHANNELS or `remote_control list_devices`) |
| `device_name` | string | Optional device name if `device_id` is omitted |
| `conversation_id` | string | Optional shared AuraGo chat conversation ID (`sess-...`) for targeted proactive responses |
| `message` | string | Message body shown in AgoChat |

**Prerequisites:** AgoDesk client paired via `/api/agodesk/ws`; `remote_control.enabled: true`. See ch. 08 **AgoDesk / AgoChat** and [`documentation/agodesk_backend_protocol.md`](../../agodesk_backend_protocol.md).

### `send_notification` / `notification_center` / `send_push_notification` / `web_push`
Deliver push notifications through configured providers (ntfy, Pushover) or browser Web Push (PWA). `notification_center` lists recent notifications; `web_push` targets subscribed PWA clients via VAPID. See ch. 08 **Notifications** and **Web Push / PWA Notifications**.

| Parameter | Type | Description |
|-----------|------|-------------|
| `title` | string | Notification title |
| `message` | string | Notification body |
| `priority` | string | normal, high, low (provider-dependent) |
| `url` | string | Optional click-through URL (Web Push) |

### `pin_message`
Pin or unpin a chat message in the Web UI history.

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Message ID to pin or unpin |
| `pinned` | boolean | true to pin, false to unpin |

### `ask_aurago`
Bridge tool for external editors (e.g. VS Code) to ask the agent a question and receive a structured response. Not intended for normal chat use.

### `send_youtube_video`
Send a YouTube video as an embedded player or link to the user.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | YouTube URL |
| `title` | string | Optional: display title |
| `start_seconds` | integer | Optional: start time in seconds |

### `tts`
Text-to-Speech: convert text to audio.

| Parameter | Type | Description |
|-----------|------|-------------|
| `text` | string | Text to synthesize into speech |
| `language` | string | Language code for the speech (e.g. 'en', 'de', 'es', 'fr') |

### `chromecast`
Cast media to Chromecast devices.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | discover, play, speak, stop, volume, status |
| `device_name` | string | Friendly device name (resolved via device registry) |
| `device_addr` | string | IP address of the Chromecast device |
| `device_port` | integer | Port of the Chromecast device (default: 8009) |
| `url` | string | Media URL to cast (for 'play' operation) |
| `local_path` | string | Local workspace file to cast (for 'play') |
| `content_type` | string | MIME type of the media (for 'play', e.g. 'video/mp4') |
| `text` | string | Text to speak aloud via TTS (for 'speak' operation) |
| `language` | string | Language code for TTS speech (for 'speak') |
| `volume` | number | Volume level 0.0–1.0 (for 'volume' operation) |

### `jellyfin`
Control Jellyfin media server.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | health, library_list, search, item_details, recent_items, sessions, playback_control, library_refresh, delete_item, activity_log |
| `query` | string | Search query (for search) |
| `media_type` | string | Filter by media type: movie, series, episode, music, album, artist |
| `item_id` | string | Media item ID (for item_details, delete_item) |
| `library_id` | string | Library ID (for library_refresh) |
| `session_id` | string | Session ID (for playback_control) |
| `command` | string | Playback command: play, pause, stop, next, previous |
| `limit` | integer | Max results to return (default: 20) |

### `agentmail`
Manage AgentMail inboxes, messages, threads, drafts, labels, and replies.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | test_connection, list_inboxes, get_inbox, create_inbox, update_inbox, delete_inbox, list_messages, get_message, update_message_labels, delete_message, send_message, reply_message, reply_all_message, forward_message, get_raw_message, get_attachment, list_threads, get_thread, list_drafts, get_draft, create_draft, update_draft, delete_draft, send_draft |
| `inbox_id` | string | AgentMail inbox ID |
| `message_id` | string | AgentMail message ID |
| `thread_id` | string | AgentMail thread ID |
| `draft_id` | string | AgentMail draft ID |
| `attachment_id` | string | AgentMail attachment ID |
| `limit` | integer | Maximum number of records to return |
| `cursor` | string | Pagination cursor |
| `after` | string | Optional ISO timestamp filter for list_messages |
| `labels` | array | Labels for list_messages filtering |
| `add_labels` | array | Labels to add for update_message_labels |
| `remove_labels` | array | Labels to remove for update_message_labels |
| `to` | array | Recipient email addresses for send/forward |
| `cc` | array | CC recipient addresses |
| `bcc` | array | BCC recipient addresses |
| `subject` | string | Email subject for send/draft operations |
| `text` | string | Plain text body for send/reply/forward/draft operations |
| `html` | string | HTML body for send/reply/forward/draft operations |
| `attachments` | array | Attachments as workspace paths or base64 objects |
| `username` | string | Inbox username for create_inbox |
| `domain` | string | Inbox domain for create_inbox |
| `display_name` | string | Inbox display name for create/update inbox |

### `send_discord`
Send messages to Discord channels.

| Parameter | Type | Description |
|-----------|------|-------------|
| `message` | string | Message text to send |
| `channel_id` | string | Discord channel ID (uses default_channel_id from config if omitted) |

### `fetch_discord`
Read messages from Discord channels.

| Parameter | Type | Description |
|-----------|------|-------------|
| `channel_id` | string | Discord channel ID (uses default from config if omitted) |
| `limit` | integer | Number of messages to fetch (default: 10) |

### `list_discord_channels`
List all text channels in the configured Discord server (guild).

---

## Device Management

### `query_inventory`
Search device inventory (by tag, type, hostname).

### `register_device`
Add new device to inventory.

| Parameter | Type | Description |
|-----------|------|-------------|
| `hostname` | string | Device name |
| `device_type` | string | server, docker, vm, network_device |
| `ip_address` | string | IP address |
| `username` | string | SSH username |
| `password` | string | Optional: SSH password |
| `private_key_path` | string | Optional: path to SSH private key |
| `port` | integer | Optional: SSH port (default: 22) |
| `description` | string | Optional: description |
| `tags` | string | Comma-separated tags |
| `mac_address` | string | MAC address for WOL |

### `wake_on_lan`
Wake device via Wake-on-LAN.

### `address_book`
Manage address book/contacts.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, search, add, update, delete |
| `name` | string | Name |
| `email` | string | Email |
| `phone` | string | Phone |

### `remote_execution`
Execute command on remote SSH server.

| Parameter | Type | Description |
|-----------|------|-------------|
| `server_id` | string | Server ID or hostname |
| `command` | string | Command to execute |

### `transfer_remote_file`
Transfer files to/from a remote SSH server.

| Parameter | Type | Description |
|-----------|------|-------------|
| `server_id` | string | Server ID or hostname |
| `remote_path` | string | Path on remote server |
| `local_path` | string | Local path |
| `direction` | enum | upload, download |

### `remote_control`
Control connected remote devices.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_devices, device_status, execute_command, read_file, write_file, edit_file |
| `device_id` | string | Device ID |
| `command` | string | Shell command |

---

## Integrations (Smart Home)

### `home_assistant`
Control Home Assistant smart home devices.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | get_states, get_state, call_service, list_services |
| `entity_id` | string | e.g., light.living_room |
| `domain` | string | light, switch, climate, scene |
| `service` | string | turn_on, turn_off, toggle |

### MQTT Tool Family (`mqtt.enabled`)

Four native tools share one broker connection and message buffer. Full examples live in `prompts/tools_manuals/mqtt.md`. Integration setup: ch. 08 **MQTT Integration**.

| Tool | Description | Key parameters |
|------|-------------|----------------|
| `mqtt_publish` | Publish a message to a topic | `topic`, `payload`, `qos` (0–2), `retain` |
| `mqtt_subscribe` | Subscribe to a topic or wildcard (`home/#`) | `topic`, `qos` |
| `mqtt_unsubscribe` | Stop receiving messages for a topic | `topic` |
| `mqtt_get_messages` | Read buffered inbound messages | `topic` (empty or `#` for all), `limit` (default 20) |

> 💡 **Workflow:** Subscribe or rely on configured `mqtt.topics`, wait for messages (or use `relay_to_agent`), then call `mqtt_get_messages`. Publish with `mqtt_publish` for outbound control.

### `fritzbox_system`
Fritz!Box system info, logs, reboot.

### `fritzbox_network`
WLAN, port forwarding, Wake-on-LAN.

### `fritzbox_telephony`
Call list, phone books, answering machine messages.

### `fritzbox_smarthome`
Control smart home devices (switches, thermostats, lamps).

### `fritzbox_storage`
NAS/Storage info, FTP server.

### `fritzbox_tv`
Fritz!Box TV stations and streaming info.

### `frigate`
Frigate NVR (Network Video Recorder) integration. Query camera status, object detection events, review summaries, snapshots, clips, recordings, and config.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | status, health, cameras, events, event, event_snapshot, event_clip, reviews, review_summary, review_activity, latest_frame, recordings_summary, export_recording, config, config_raw |
| `camera` | string | Camera name |
| `event_id` | string | Event ID |
| `label` | string | Object label filter (person, car, dog, etc.) |
| `zone` | string | Zone name filter |
| `after` / `before` | integer | Unix timestamp range |
| `limit` / `offset` | integer | Pagination for events and reviews |

---

## Integrations (Cloud & APIs)

### `api_request`
HTTP request to external APIs.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Target URL |
| `method` | enum | GET, POST, PUT, PATCH, DELETE |
| `headers` | object | HTTP headers |
| `body` | string | Request body |

### `github`
Manage GitHub repositories, issues, PRs, branches, files, commits, workflow runs, and local project tracking (`github.enabled`).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_repos, create_repo, delete_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits, list_workflow_runs, search_repos, list_projects, track_project, untrack_project |
| `name` | string | Repository or project name |
| `owner` | string | GitHub owner/org (defaults to configured owner) |
| `title` | string | Issue title |
| `path` | string | File path within the repository |
| `query` | string | Search query or branch name |

### `google_workspace`
Gmail, Calendar, Drive, Docs, Sheets.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | gmail_list, gmail_read, gmail_send, gmail_modify_labels, calendar_list, calendar_create, drive_search, docs_get, sheets_get, sheets_update |

### `onedrive`
Microsoft OneDrive file management.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, info, read, download, search, upload, delete, move, copy, share |
| `path` | string | Path in OneDrive |

### `s3_storage`
S3-compatible storage (AWS, MinIO, Wasabi).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_buckets, list_objects, upload, download, delete, copy, move |
| `bucket` | string | Bucket name |
| `key` | string | Object key |

### `netlify`
Manage Netlify sites, deploys, environment variables.

### `tailscale`
Manage Tailscale VPN network.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | devices, device, routes, enable_routes, disable_routes, dns, acl, local_status |

### `cloudflare_tunnel`
Manage Cloudflare Tunnel (cloudflared).

### `proxmox`
Manage Proxmox VE VMs and containers.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | overview, list_nodes, list_vms, list_containers, status, start, stop, shutdown, reboot, create_snapshot, list_snapshots |
| `node` | string | Node name |
| `vmid` | string | VM/Container ID |

### `docker`
Manage Docker containers, images, networks, volumes.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull |
| `container_id` | string | Container ID/Name |
| `image` | string | Docker image |

### `ollama`
Manage local Ollama LLM instance.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, running, show, pull, delete, copy, load, unload |
| `model` | string | Model name |

### `vercel`
Manage Vercel projects, deployments, and environment variables.

### `obsidian`
Read and search Obsidian vault notes.

### `uptime_kuma`
Monitor uptime via Uptime Kuma (status pages, heartbeats).

### `ldap`
Query LDAP directory (users, groups, search).

### `paperless_ngx`
Manage Paperless-ngx documents (search, upload, tags, correspondents).

### `adguard`
Manage AdGuard Home DNS server.

### `ansible`
Run Ansible playbooks and ad-hoc commands.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | status, list_playbooks, inventory, ping, adhoc, playbook, check, facts |
| `name` | string | Playbook file |
| `hostname` | string | Target host pattern |
| `module` | string | Ansible module |

### `meshcentral`
Manage MeshCentral devices.

### `koofr`
Access Koofr cloud storage: list, read, download, upload, move, and copy files.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, read, download, write, upload, mkdir, delete, rename, move, copy |
| `path` | string | File or directory path in Koofr |
| `destination` | string | Destination path for rename/move/copy operations |
| `content` | string | Non-empty text content to write (for 'write' operation only) |
| `local_path` | string | Existing local file path to upload (for 'upload' operation) |

### `yepapi_seo`
SEO data via YepAPI: domain overviews, keywords, competitors, and backlinks.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | keywords, keyword_ideas, domain_overview, domain_keywords, competitors, backlinks, onpage, trends |
| `keywords` | array | Array of keywords (for 'keywords' operation) |
| `seed` | string | Seed keyword for suggestions (for 'keyword_ideas') |
| `domain` | string | Domain name (for domain_* operations) |
| `target` | string | Target domain or URL (for 'backlinks') |
| `url` | string | Page URL to audit (for 'onpage') |

### `yepapi_serp`
Search engine results via YepAPI: Google, Google Maps, News, Images, and autocomplete.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | google, google_images, google_news, google_maps, google_datasets, google_autocomplete, google_ads, google_ai_mode, google_finance, yahoo, bing, baidu, youtube |
| `query` | string | Search query (required) |
| `depth` | integer | Number of results to return (default: 10) |
| `location` | string | Country code for localised results |
| `language` | string | Language code |
| `limit` | integer | Max results for Google Maps (default: 10) |
| `open_now` | boolean | Filter Google Maps for currently open places |

### `yepapi_scrape`
Scrape and extract web page content through YepAPI.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | scrape, js, stealth, screenshot, extract, ai_extract, search_google |
| `url` | string | URL to scrape |
| `query` | string | Google search query (for search_google operation) |
| `selector` | string | CSS selector for extract operation |
| `xpath` | string | XPath selector for extract operation |
| `format` | string | Output format: 'markdown' or 'html' (default: markdown) |
| `prompt` | string | Natural language extraction prompt (for ai_extract) |
| `limit` | integer | Max results for search_google |

### `yepapi_youtube`
YouTube data via YepAPI: search, videos, transcripts, comments, channels, and playlists.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | search, video, video_info, metadata, transcript, subtitles, comments, channel, channel_videos, channel_shorts, channel_livestreams, channel_live, channel_playlists, channel_community, channel_about, channel_search, channel_channels, channel_store, playlist, playlist_info, trending, related, screenshot, shorts, shorts_info, suggest, hashtag, post, post_comments, home, hype, resolve |
| `query` | string | Search query |
| `video_id` | string | YouTube video ID |
| `channel_id` | string | YouTube channel ID |
| `playlist_id` | string | YouTube playlist ID |
| `url` | string | YouTube URL (for resolve operation) |
| `tag` | string | Hashtag/tag without # |
| `post_id` | string | YouTube community post ID |
| `country` | string | Optional country code for feed-style operations |
| `language` | string | Optional language code for feed-style operations |
| `limit` | integer | Max results to return (default: 10) |

### `yepapi_tiktok`
TikTok data via YepAPI: videos, users, posts, comments, music, and challenges.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | search, search_user, search_challenge, search_photo, video, user, user_posts, user_followers, user_following, user_favorites, user_reposts, user_story, comments, comment_replies, music, music_videos, challenge, challenge_videos |
| `query` | string | Search query |
| `url` | string | TikTok video or music URL |
| `username` | string | TikTok username/unique_id |
| `name` | string | Challenge name |
| `comment_id` | string | TikTok comment ID |
| `limit` | integer | Max results to return (default: 10) |

### `yepapi_instagram`
Instagram data via YepAPI: users, posts, reels, comments, hashtags, and places.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | search, user, userinfo, user_info, profile, user_profile, user_about, user_posts, user_reels, user_stories, user_highlights, user_tagged, user_followers, user_similar, post, post_comments, post_likers, hashtag, media_id |
| `query` | string | Search query for search operation |
| `search_query` | string | Search query for search operation |
| `username` | string | Alias for username_or_url |
| `username_or_url` | string | Instagram username or profile URL |
| `shortcode` | string | Instagram post shortcode |
| `tag` | string | Hashtag without # |
| `limit` | integer | Max results to return (default: 10) |

### `yepapi_amazon`
Amazon data via YepAPI: products, reviews, offers, categories, and search.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | search, product, reviews, product_offers, products_by_category, categories, deals, best_sellers, influencer, seller, seller_reviews |
| `query` | string | Search query (for search operation) |
| `asin` | string | Amazon ASIN product ID |
| `country` | string | Amazon marketplace country code (default: 'US') |
| `category` | string | Category slug or browse node ID |
| `handle` | string | Amazon influencer handle |
| `seller_id` | string | Amazon seller ID |
| `limit` | integer | Max results to return where supported |
| `page` | integer | Page number for paginated operations |
| `sort_by` | string | Review sort order: 'TOP_REVIEWS' or 'MOST_RECENT' |

---

## Network & Security

### `dns_lookup`
Query DNS records (A, AAAA, MX, NS, TXT, CNAME, PTR).

| Parameter | Type | Description |
|-----------|------|-------------|
| `host` | string | Hostname |
| `record_type` | enum | all, A, AAAA, MX, NS, TXT, CNAME, PTR |

### `whois_lookup`
Retrieve WHOIS information for domains.

### `network_ping`
Send ping to host (ICMP).

### `port_scanner`
TCP port scan on target host.

| Parameter | Type | Description |
|-----------|------|-------------|
| `host` | string | Target host |
| `port_range` | string | 80, 443, 1-1024, common |

### `mdns_scan`
Search for mDNS/Bonjour devices on local network.

### `mac_lookup`
Look up MAC address via ARP table.

### `upnp_scan`
Discover UPnP/SSDP devices (routers, TVs, NAS).

### `virustotal_scan`
VirusTotal threat intelligence scan.

| Parameter | Type | Description |
|-----------|------|-------------|
| `resource` | string | URL, Domain, IP, File Hash |
| `file_path` | string | Local file |
| `mode` | enum | auto, hash, upload |

### `firewall`
Manage Linux firewall (iptables/ufw).

---

## Web & Scraping

### `web_scraper`
Extract web page as plain text (HTML → Text).

### `site_crawler`
Crawl website (follow links, multiple pages).

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Start URL |
| `max_depth` | integer | 1-5 |
| `max_pages` | integer | 1-100 |
| `allowed_domains` | string | Comma-separated domains |

### `web_capture`
Create screenshot or PDF from URL (Chromium).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | screenshot, pdf |
| `url` | string | Target URL |
| `selector` | string | CSS selector (wait) |
| `full_page` | boolean | Scroll full page |

### `web_performance_audit`
Measure Core Web Vitals (TTFB, FCP, LCP, etc.).

### `form_automation`
Automatically fill/submit web forms.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | get_fields, fill_submit, click |
| `url` | string | Page URL |
| `fields` | string | JSON {selector: value} |
| `selector` | string | CSS selector for click |

### `browser_automation`
Full browser session automation via the optional browser automation sidecar (CloakBrowser stealth Chromium). Supports multi-step navigation, UI inspection, clicks, typing, file uploads, screenshots, and downloads.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | create_session, close_session, navigate, click, type, select, press, wait_for, extract, screenshot, upload_file, list_downloads, get_download, current_state |
| `session_id` | string | Browser session ID (required except for create_session) |
| `url` | string | Target URL for create_session or navigate |
| `selector` | string | CSS selector for click, type, select, upload_file, wait_for |
| `text` | string | Text for the type operation |
| `file_path` | string | Workspace-relative path for upload_file |
| `wait_for` | enum | visible, hidden, attached, detached, load, networkidle |

### `site_monitor`
Monitor websites for changes.

### `ddg_search`
Search the web with DuckDuckGo and return the top results.

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | Search query to submit to DuckDuckGo |
| `max_results` | integer | Maximum number of results to return (default: 5) |
| `search_query` | string | Optional focused question for summary mode |

### `wikipedia_search`
Look up encyclopedic topics on Wikipedia.

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | The Wikipedia search term or page title to look up |
| `language` | string | Optional Wikipedia language code such as de, en, fr, or ja |
| `search_query` | string | Optional focused question for summary mode |

### `brave_search`
Search the web with Brave Search.

| Parameter | Type | Description |
|-----------|------|-------------|
| `query` | string | The search query string (required) |
| `count` | integer | Number of results to return (1-20, default: 10) |
| `country` | string | Two-letter country code for localised results |
| `lang` | string | Search language code |

---

## Documents & Media Processing

### `document_creator`
Create PDFs, convert, merge.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | create_pdf, url_to_pdf, html_to_pdf, markdown_to_pdf, convert_document, merge_pdfs, screenshot_url, screenshot_html |
| `title` | string | Document title |
| `content` | string | HTML/Markdown content |
| `url` | string | URL for screenshot/PDF |
| `paper_size` | enum | A4, A3, A5, Letter, Legal |
| `landscape` | boolean | Landscape orientation |

### `pdf_operations`
Manipulate PDFs (merge, split, watermark, encrypt, fill forms).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | merge, split, watermark, compress, encrypt, decrypt, metadata, page_count, form_fields, fill_form |
| `file_path` | string | Input PDF |
| `output_file` | string | Output PDF |
| `password` | string | Password |

### `image_processing`
Process images (resize, convert, crop, rotate).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | resize, convert, compress, crop, rotate, info |
| `file_path` | string | Image path |
| `width` | integer | Target width |
| `height` | integer | Target height |
| `quality_pct` | integer | 1-100 |
| `angle` | integer | 90, 180, 270 |

### `video_download`
Download videos from supported platforms (YouTube, Vimeo, etc.). Optionally converts to different formats and saves to the Media Registry.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Video URL |
| `format` | string | Desired format (mp4, webm, audio) |
| `quality` | string | Quality level |

### `media_conversion`
Convert media files between different formats (video, audio, images). Uses FFmpeg for transcoding.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | convert, info, extract_audio, extract_video, thumbnail |
| `file_path` | string | Input file |
| `output_file` | string | Output file |
| `output_format` | string | Target format (mp4, webm, mp3, wav, jpg, png) |
| `video_codec` | string | Optional: video codec (h264, hevc, vp9) |
| `audio_codec` | string | Optional: audio codec (aac, mp3, opus) |
| `video_bitrate` | string | Optional: video bitrate (e.g. 2M) |
| `audio_bitrate` | string | Optional: audio bitrate (e.g. 128k) |
| `width` | integer | Optional: target width in pixels |
| `height` | integer | Optional: target height in pixels |
| `fps` | integer | Optional: frames per second |
| `sample_rate` | integer | Optional: audio sample rate |
| `quality_pct` | integer | Optional: quality (1-100) |

### `pdf_extractor`
Extract and optionally summarise text from PDF documents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `filepath` | string | The path to the PDF file |
| `search_query` | string | When summary mode is active, tell the summariser what information to extract |

---

## Databases

### `sql_query`
Execute SQL query against registered database.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | query, describe, list_tables |
| `connection_name` | string | Connection name |
| `sql_query` | string | SQL statement |
| `table_name` | string | Table name |

### `manage_sql_connections`
Manage external database connections.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, get, create, update, delete, test, docker_create |
| `connection_name` | string | Name |
| `driver` | enum | postgres, mysql, sqlite |
| `host` | string | Database host |
| `port` | integer | Port |
| `database_name` | string | Database name |
| `allow_read/write/change/delete` | boolean | Permissions |

---

## Infrastructure

### `three_d_printer`
Inspect and control configured Elegoo Centauri Carbon and Klipper/Moonraker printers.

**Config:** `three_d_printers.enabled`; `three_d_printers.readonly: true` blocks write operations.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | `list_printers`, `status`, `attributes`, `files`, `history`, `camera_url`, `camera_snapshot`, `analyze_camera`, `show_live_stream`, `start_print`, `pause_print`, `resume_print`, `cancel_print`, `set_camera_light` |
| `printer_id` | string | Printer ID from config (uses `default_printer` if omitted) |
| `filename` | string | G-code path for `start_print` |
| `prompt` | string | Vision prompt for `analyze_camera` |
| `light_on` | boolean | Elegoo camera light (`set_camera_light`) |

**API:** `GET /api/3d-printers/test`, camera snapshot/stream endpoints per printer ID.

### `composio_call`
Search Composio toolkits/tools and execute user-approved Composio actions (`composio.enabled` + vault `composio_api_key`).

### `webdav`
List, read, write, move, and delete files on a configured WebDAV endpoint (`webdav.enabled`). Respects `webdav.readonly`.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list, read, write, mkdir, delete, move, info |
| `path` | string | Remote path relative to the configured base URL |
| `destination` | string | Target path for move |
| `content` | string | File content for write |

### `certificate_manager`
Inspect PEM certificates, check remote HTTPS endpoints, or generate local self-signed test certificates.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | info, check_remote, generate_self_signed |
| `file_path` | string | Workspace-resolved PEM path for info |
| `hostname` | string | Remote HTTPS hostname or IP for check_remote |
| `domain` | string | DNS name for generate_self_signed |
| `output_dir` | string | Output directory for cert.pem and key.pem |

### `invasion_control`
Manage Invasion Control (remote deployment).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_nests, list_eggs, nest_status, assign_egg, hatch_egg, stop_egg, egg_status, send_task, send_secret |
| `nest_id` | string | Nest ID |
| `egg_id` | string | Egg ID |
| `task` | string | Task description |

### `co_agent`
Spawn and manage parallel co-agents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | spawn, spawn_specialist, list, get_result, stop, stop_all |
| `task` | string | Task |
| `specialist` | enum | researcher, coder, designer, security, writer |
| `priority` | integer | 1=low, 2=normal, 3=high |

## Homepage & static site tool family

Requires `homepage.enabled: true`. Workspace paths in tool arguments are **relative** to the homepage root (e.g. `project_dir: "my-site"`), not absolute `/workspace/...` paths.

| Native tool | Purpose | Manual |
|-------------|---------|--------|
| `homepage_project` | Container/workspace lifecycle, `init_project`, `exec`, `install_deps` | `homepage_project.md` |
| `homepage_file` | `list_files`, `read_file`, `write_file`, `edit_file` | `homepage_file.md` |
| `homepage_deploy` | `build`, `dev`, local webserver, `deploy_netlify`, `deploy_vercel`, `tunnel` | `homepage_deploy.md` |
| `homepage_quality` | `lint`, `check_js`, `lighthouse`, `screenshot`, `optimize_images` | `homepage_quality.md` |
| `homepage_git` | `git_init`, `git_commit`, `git_status`, `git_diff`, `git_log`, `git_rollback` | `homepage_git.md` |
| `homepage_registry` | `register`, `search`, `list`, `log_edit`, `log_deploy`, `log_problem`, `list_history`, `add_history`, … | `homepage_registry.md` |
| `homepage` | **Legacy** combined `operation` surface (same capabilities, older prompts) | `homepage.md` |

**History workflow:** Call `homepage_registry` → `list_history` before major edits; `add_history` after meaningful changes (types: `decision`, `note`, `feedback`, `milestone`, …).

**UI:** Virtual Desktop app **Homepage Studio** uses the same workspace. **Config → Integrations → Homepage**.

### `homepage_registry` (detail)

SQLite-backed project catalog (`sqlite.homepage_registry_path`). Auto-registration on `init_project`. Operations include `register`, `search`, `get`, `list`, `update`, `delete`, `log_edit`, `log_deploy`, `log_problem`, `resolve_problem`, and history: `add_history`, `list_history`, `get_history`, `search_history`, `update_history`, `delete_history`.

### `manage_updates`
Check/install AuraGo updates.

### `manage_webhooks`
Manage incoming webhooks.

### `call_webhook` / `manage_outgoing_webhooks`
Trigger/manage outgoing webhooks.

### `telnyx_sms`
Send SMS/MMS via Telnyx.

### `telnyx_call`
Initiate/control voice calls via Telnyx.

### `telnyx_manage`
Manage Telnyx resources (numbers, balance, history).

### `fetch_email`
Fetch emails from registered accounts.

### `send_email`
Send emails via registered accounts.

### `list_email_accounts`
List registered email accounts.

### `mcp_call`
Call external MCP (Model Context Protocol) servers.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | list_servers, list_tools, call_tool |
| `server` | string | MCP server name |
| `tool_name` | string | Tool name |
| `mcp_args` | object | Arguments |

### `grafana`
Read Grafana observability data: health, dashboards, datasources, queries, alerts, and org info (`grafana.enabled`).

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | health, list_dashboards, get_dashboard, list_datasources, query, list_alerts, get_org |
| `uid` | string | Dashboard UID for get_dashboard |
| `query` | string | Search query for list_dashboards or read expression for query |
| `datasource_uid` | string | Datasource UID for query |
| `datasource_type` | string | prometheus, mimir, cortex, loki, or elasticsearch |

### `space_agent`
Send instructions to the configured Space Agent sidecar.

| Parameter | Type | Description |
|-----------|------|-------------|
| `instruction` | string | Clear instruction for the Space Agent instance |
| `information` | string | Optional: supporting context (no secrets) |
| `session_id` | string | Optional: correlation/session identifier |

### `virtual_desktop`
Control AuraGo's browser virtual desktop. Enables file operations, app installation, widgets, Office documents, and notifications.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | status, bootstrap, list_files, read_file, search_file, read_file_excerpt, write_file, patch_file, delete, delete_file, delete_path, delete_app, read_document, write_document, patch_document, read_workbook, write_workbook, set_cell, set_range, evaluate_formula, export_file, install_app, upsert_widget, open_app, open_in_app, show_notification, list_apps, get_app, list_widgets, get_widget, diagnose_app, diagnose_widget |
| `path` | string | Workspace-relative file or directory path |
| `file_path` | string | Alias for path |
| `content` | string | File content, document text, cell value, or notification text |
| `allow_empty` | boolean | Only for intentionally empty non-app/non-widget files |
| `query` | string | Search text for search_file |
| `line_start` | integer | Line number for read_file_excerpt |
| `line_count` | integer | Number of lines for read_file_excerpt (default: 80) |
| `max_matches` | integer | Maximum matches for search_file (default: 8) |
| `context_lines` | integer | Context lines around search_file matches (default: 2) |
| `case_sensitive` | boolean | Case-sensitive matching for search_file |
| `title` | string | Notification or widget title |
| `html` | string | Optional: HTML representation for documents |
| `document` | object | Document payload for write_document |
| `workbook` | object | Workbook payload for write_workbook |

### `truenas`
Manage TrueNAS storage (pools, datasets, snapshots, shares).

| Parameter | Type | Description |
|-----------|------|-------------|
| `action` | enum | truenas_health, truenas_pool_list, truenas_pool_scrub, truenas_dataset_list, truenas_dataset_create, truenas_dataset_delete, truenas_snapshot_list, truenas_snapshot_create, truenas_snapshot_delete, truenas_snapshot_rollback, truenas_smb_list, truenas_smb_create, truenas_smb_delete, truenas_fs_space |
| `name` | string | Dataset, snapshot, or SMB share name |
| `path` | string | SMB share local filesystem path (for truenas_smb_create) |
| `query` | string | Pool name or dataset path for filtering |
| `port` | integer | Numeric pool ID for truenas_pool_scrub, or SMB share ID for truenas_smb_delete |
| `limit` | integer | Quota in GB for truenas_dataset_create, or snapshot retention days |
| `content` | string | Compression type for truenas_dataset_create: lz4, zstd, gzip, off |
| `recursive` | boolean | Enable recursive operation |
| `force` | boolean | Force rollback |

### `office_document`
Create, read, patch, and export virtual desktop Writer documents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | read, write, patch, export |
| `path` | string | Workspace-relative document path |
| `file_path` | string | Alias for path |
| `output_path` | string | Workspace-relative target path for export |
| `format` | string | Export format: docx, html, md, or txt |
| `title` | string | Document title for write or patch |
| `content` | string | Plain document text for write, or seed text for patch |
| `text` | string | Alias for content |
| `html` | string | Optional HTML representation for write |
| `prepend_text` | string | Text to prepend during patch |
| `append_text` | string | Text to append during patch |
| `replacements` | array | Patch replacements, each item {find, replace} |
| `document` | object | Complete document payload for write |

### `office_workbook`
Create, read, edit, evaluate, and export virtual desktop spreadsheets.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | read, write, set_cell, set_range, evaluate_formula, export |
| `path` | string | Workspace-relative workbook path |
| `file_path` | string | Alias for path |
| `output_path` | string | Workspace-relative target path for export |
| `format` | string | Export format: xlsx or csv |
| `sheet` | string | Sheet name for workbook operations |
| `cell` | string | A1-style cell reference for set_cell |
| `start_cell` | string | A1-style top-left cell reference for set_range |
| `value` | string | Cell value for set_cell |
| `formula` | string | Cell formula for set_cell/evaluate_formula |
| `values` | array | 2D array for set_range |
| `workbook` | object | Workbook payload for write |

---

## Permission System

Not all tools are available by default. Availability depends on **Tool Feature Flags**:

### Always Available
- `filesystem` (read-only if `allow_filesystem_write=false`)
- `file_search`, `file_reader_advanced`, `smart_file_read`
- `text_diff`
- `system_metrics`, `process_analyzer`, `process_management`
- `discover_tools`
- `ddg_search`, `wikipedia_search`
- `analyze_image`, `transcribe_audio`
- `send_image`, `send_audio`, `send_video`, `send_document`
- `list_skills`, `execute_skill`
- `dns_lookup`, `whois_lookup`
- `detect_file_type`, `archive`
- `pdf_operations`, `image_processing`
- `follow_up`, `wait_for_event`
- `tts`

### Danger Zone (Configurable Permissions)
| Flag | Tools |
|------|-------|
| `allow_shell` | `execute_shell` |
| `allow_python` | `execute_python`, `save_tool`, `create_skill_from_template` |
| `allow_filesystem_write` | `filesystem` (write), `file_editor`, `json_editor`, `yaml_editor`, `xml_editor` |
| `allow_network_requests` | `api_request` |
| `allow_remote_shell` | `remote_execution` |
| `allow_self_update` | `manage_updates` |
| `sudo_enabled` | `execute_sudo` |

### Integration Flags
| Flag | Tools |
|------|-------|
| `home_assistant_enabled` | `home_assistant` |
| `docker_enabled` | `docker` |
| `proxmox_enabled` | `proxmox` |
| `github_enabled` | `github` |
| `mqtt_enabled` | `mqtt_publish`, `mqtt_subscribe`, `mqtt_unsubscribe`, `mqtt_get_messages` |
| `tailscale_enabled` | `tailscale` |
| `adguard_enabled` | `adguard` |
| `ansible_enabled` | `ansible` |
| `invasion_control_enabled` | `invasion_control` |
| `netlify_enabled` | `netlify` |
| `cloudflare_tunnel_enabled` | `cloudflare_tunnel` |
| `ollama_enabled` | `ollama` |
| `google_workspace_enabled` | `google_workspace` |
| `onedrive_enabled` | `onedrive` |
| `image_generation_enabled` | `generate_image` |
| `video_generation_enabled` | `generate_video` |
| `fritzbox_*_enabled` | `fritzbox_system`, `fritzbox_network`, `fritzbox_telephony`, `fritzbox_smarthome`, `fritzbox_storage`, `fritzbox_tv` |
| `telnyx_*_enabled` | `telnyx_sms`, `telnyx_call`, `telnyx_manage` |
| `webhooks_enabled` | `call_webhook`, `manage_outgoing_webhooks`, `manage_webhooks` |
| `mcp_enabled` | `mcp_call` |
| `sandbox_enabled` | `execute_sandbox` |
| `meshcentral_enabled` | `meshcentral` |
| `chromecast_enabled` | `chromecast` |
| `discord_enabled` | `send_discord`, `fetch_discord`, `list_discord_channels` |
| `jellyfin_enabled` | `jellyfin` |
| `truenas_enabled` | `truenas` |
| `koofr_enabled` | `koofr` |
| `homepage_enabled` | `homepage`, `homepage_registry` |
| `firewall_enabled` | `firewall` |
| `email_enabled` | `fetch_email`, `send_email`, `list_email_accounts` |
| `virustotal_enabled` | `virustotal_scan` |
| `s3_enabled` | `s3_storage` |
| `web_scraper_enabled` | `web_scraper`, `site_crawler`, `site_monitor` |
| `network_scan_enabled` | `mdns_scan`, `mac_lookup` |
| `network_ping_enabled` | `network_ping`, `port_scanner` |
| `upnp_scan_enabled` | `upnp_scan` |
| `form_automation_enabled` | `form_automation` (requires `web_capture_enabled` as well) |
| `sql_connections_enabled` | `sql_query`, `manage_sql_connections` |
| `music_generation_enabled` | `generate_music` |
| `golangci_lint_enabled` | `golangci_lint` |
| `daemon_skills_enabled` | `manage_daemon` |
| `remote_control_enabled` | `remote_control` |
| `document_creator_enabled` | `document_creator` |
| `web_capture_enabled` | `web_capture`, `web_performance_audit` |
| `planner_enabled` | `manage_plan`, `manage_appointments`, `manage_todos` |
| `media_registry_enabled` | `media_registry` |
| `homepage_registry_enabled` | `homepage_registry` |
| `vercel_enabled` | `vercel` |
| `obsidian_enabled` | `obsidian` |
| `uptime_kuma_enabled` | `uptime_kuma` |
| `ldap_enabled` | `ldap` |
| `paperless_ngx_enabled` | `paperless_ngx` |
| `python_secret_injection_enabled` | Vault secret injection into Python skills |

---

## Tool Manual Index (RAG)

AuraGo indexes `prompts/tools_manuals/*.md` (~165 files) for adaptive tool retrieval. At runtime the agent can call `discover_tools` with `get_tool_info` to load the matching manual.

### Native tool → manual file

Most tools use `{tool_name}.md`. Exceptions and grouped manuals:

| Native tool(s) | Manual file | Notes |
|----------------|-------------|-------|
| `mqtt_publish`, `mqtt_subscribe`, `mqtt_unsubscribe`, `mqtt_get_messages` | `mqtt.md` | MQTT tool family |
| `mcp_call` | `mcp.md` | MCP client operations |
| `execute_sandbox` | `sandbox.md` | Sandboxed execution |
| `fetch_email`, `send_email`, `list_email_accounts` | `email.md` | Email account tools |
| `send_discord`, `fetch_discord`, `list_discord_channels` | `discord.md` | Discord messaging |
| `call_webhook`, `manage_webhooks`, `manage_outgoing_webhooks` | `manage_webhooks.md` | Webhook management |
| `list_skills`, `get_skill_documentation`, `set_skill_documentation` | `skills_engine.md` | Skill runtime |
| `list_skill_templates`, `create_skill_from_template` | `skill_templates.md` | Skill authoring |
| `transfer_remote_file` | `remote_execution.md` | SFTP transfers |
| `manage_memory` | `context_memory.md`, `core_memory.md` | Memory layers |
| `yepapi_*` (7 tools) | `yepapi.md` | Shared YepAPI reference |

### Legacy / meta manuals (no 1:1 native tool name)

| Manual file | Purpose |
|-------------|---------|
| `homeassistant.md` | Legacy alias; native tool is `home_assistant` |
| `paperless.md` | Legacy alias; native tool is `paperless_ngx` |
| `invoke_tool.md` | Invoke hidden tools filtered by adaptive tooling |
| `run_tool.md` | Execute saved custom Python tools |
| `optimize_memory.md` | Nightly memory maintenance workflow |
| `budget_status.md` | Budget and cost status display |
| `ssh_key_manager.md` | SSH key helper (inventory workflows) |
| `skill_manifest_spec.md` | Skill manifest authoring reference |

> 📖 Source of truth for parameters and examples: `prompts/tools_manuals/` and `config_template.yaml`. This handbook chapter summarizes gates and operations; manuals contain call patterns.

---

## Related Links

- [Tools Chapter](06-tools.md) – Using tools in chat
- [Chat Commands](20-chat-commands.md) – Manual commands
- [API Reference](21-api-reference.md) – REST API
- [Security](14-security.md) – Permissions & Danger Zone


---

