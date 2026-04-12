# Internal Tools

This documentation lists all internal tools that AuraGo makes available to the agent. These tools are invoked by the LLM via native function calling.

> 📅 **Updated:** March 2026  
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
| `target` | enum | all, cpu, memory, disk, processes, host, sensors, network_detail |

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

### `manage_daemon`
Manage daemon skills (long-running background processes).

### `context_manager`
Manage session contexts and switch between them.

---

## Communication & Media

### `send_image`
Send image to user (local path or URL).

### `send_audio`
Send audio file (inline player).

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
| `ssh_user` | string | SSH username |
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
| `direction` | enum | upload, download (for file transfer) |

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

### `mqtt_publish`
Publish MQTT message.

### `mqtt_subscribe`
Subscribe to MQTT topic.

### `mqtt_unsubscribe`
Unsubscribe from MQTT topic.

### `mqtt_get_messages`
Retrieve received MQTT messages.

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
Manage GitHub repositories, issues, PRs, commits.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | list_repos, create_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits |
| `name` | string | Repository name |
| `owner` | string | GitHub owner/org |
| `title` | string | Issue title |

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

### `site_monitor`
Monitor websites for changes.

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

### `homepage`
Develop, build, deploy homepage/web projects.

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | enum | init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, deploy_netlify, dev, deploy, git_init, git_commit, git_status, git_diff, git_log |
| `framework` | enum | next, vite, astro, svelte, vue, html |
| `command` | string | Shell command |
| `project_dir` | string | Project subdirectory |

### `homepage_registry`
Track homepage projects (URL, framework, deploy history).

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

---

## Permission System

Not all tools are available by default. Availability depends on **Tool Feature Flags**:

### Always Available
- `filesystem` (read-only if `allow_filesystem_write=false`)
- `file_search`, `file_reader_advanced`, `smart_file_read`
- `system_metrics`, `process_analyzer`
- `analyze_image`, `transcribe_audio`
- `send_image`, `send_audio`, `send_document`
- `list_skills`, `execute_skill`
- `dns_lookup`, `whois_lookup`
- `detect_file_type`, `archive`
- `pdf_operations`, `image_processing`
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
| `planner_enabled` | `manage_appointments`, `manage_todos` |
| `media_registry_enabled` | `media_registry` |
| `homepage_registry_enabled` | `homepage_registry` |

---

## Related Links

- [Tools Chapter](06-tools.md) – Using tools in chat
- [Chat Commands](20-chat-commands.md) – Manual commands
- [API Reference](21-api-reference.md) – REST API
- [Security](14-security.md) – Permissions & Danger Zone


---

## Additional Tools & Integrations

### `tts`
Text-to-Speech synthesis via configurable providers (ElevenLabs, Piper, MiniMax, Wyoming). Always available.

### `chromecast`
Discover and control Chromecast devices on the network (Play, TTS, Discover).

### `brave_search`
Web search via Brave Search API.

### `ddg_search`
Web search via DuckDuckGo (Instant Answers and HTML results).

### `wikipedia_search`
Search and read Wikipedia articles.

### `scraper_summary`
Automatically summarize and structure web scraping results.

### `truenas`
Manage TrueNAS SCALE storage (pools, datasets, shares, snapshots).

### `jellyfin`
Control Jellyfin Media Server (libraries, sessions, playback).

### `koofr`
Koofr cloud storage file operations (list, upload, download, links).

### `pdf_extractor`
Extract text and metadata from PDF files (including OCR support). Executed as a skill.
