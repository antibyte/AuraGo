# Interne Tools

Diese Dokumentation listet alle internen Tools, die AuraGo dem Agenten zur Verfügung stellt. Diese Tools werden vom LLM über native Function Calling aufgerufen.

> 📅 **Stand:** März 2026  
> 🔢 **Anzahl:** 100+ Tools

---

## Inhaltsverzeichnis

1. [Skills & Code-Ausführung](#skills--code-ausführung)
2. [Dateisystem](#dateisystem)
3. [System & Prozesse](#system--prozesse)
4. [Speicher & Gedächtnis](#speicher--gedächtnis)
5. [Kommunikation & Medien](#kommunikation--medien)
6. [Geräteverwaltung](#geräteverwaltung)
7. [Integrationen (Smart Home)](#integrationen-smart-home)
8. [Integrationen (Cloud & APIs)](#integrationen-cloud--apis)
9. [Netzwerk & Sicherheit](#netzwerk--sicherheit)
10. [Web & Scraping](#web--scraping)
11. [Dokumente & Medien](#dokumente--medien)
12. [Datenbanken](#datenbanken)
13. [Infrastruktur](#infrastruktur)

---

## Skills & Code-Ausführung

### `list_skills`
Listet verfügbare vordefinierte Skills und Integrationen auf.

### `execute_skill`
Führt einen registrierten Skill aus (z.B. Web-Suche, PDF-Extraktion, VirusTotal-Scan).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `skill` | string | Name des Skills |
| `skill_args` | object | Argumente für den Skill |
| `vault_keys` | array | Vault-Secret-Keys für Python |

### `execute_python`
Führt Python-Code aus (Host-System).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `code` | string | Python-Code |
| `description` | string | Kurzbeschreibung |
| `background` | boolean | Als Hintergrundprozess |

### `execute_sandbox` ⭐
Führt Code in isoliertem Docker-Sandbox aus (sicherer als `execute_python`).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `code` | string | Quellcode |
| `sandbox_lang` | string | Sprache: python, javascript, go, java, cpp, r |
| `libraries` | array | Zusätzliche Pakete |

### `save_tool`
Speichert ein neues Python-Tool im Tools-Verzeichnis.

### `list_skill_templates`
Listet verfügbare Skill-Templates auf.

### `create_skill_from_template`
Erstellt einen Skill aus einem Template (api_client, file_processor, data_transformer, scraper).

### `golangci_lint`
Führt golangci-lint auf Go-Code aus (Code-Qualitäts-Checks).

---

## Dateisystem

### `filesystem`
Dateisystem-Operationen (read/write/move/copy/delete/list).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | read_file, write_file, delete, move, list_dir, create_dir, stat |
| `file_path` | string | Pfad zur Datei |
| `content` | string | Inhalt (für write_file) |

### `file_search`
Text-Suche in Dateien (grep, find).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | grep, grep_recursive, find |
| `pattern` | string | Regex-Suchmuster |
| `glob` | string | Datei-Muster (*.go) |

### `file_reader_advanced`
Fortgeschrittenes Datei-Lesen (head, tail, Zeilenbereiche).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | read_lines, head, tail, count_lines, search_context |
| `file_path` | string | Dateipfad |
| `start_line` | integer | Startzeile |
| `end_line` | integer | Endzeile |

### `smart_file_read`
Intelligentes Lesen großer Dateien (analysieren, samplen, zusammenfassen).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | analyze, sample, structure, summarize |
| `file_path` | string | Dateipfad |
| `sampling_strategy` | enum | head, tail, distributed, semantic |

### `file_editor`
Präzise Text-Datei-Edits (str_replace, insert, delete).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | str_replace, str_replace_all, insert_after, insert_before, append, prepend, delete_lines |
| `file_path` | string | Dateipfad |
| `old` | string | Zu findender Text |
| `new` | string | Ersatztext |

### `json_editor`
JSON-Dateien lesen/modifizieren (get, set, delete, validate).

### `yaml_editor`
YAML-Dateien lesen/modifizieren.

### `xml_editor`
XML-Dateien mit XPath bearbeiten.

### `archive`
ZIP/TAR.GZ Archive erstellen/extrahieren/listen.

### `detect_file_type`
Dateityp per Magic-Bytes erkennen (ignoriert Extension).

---

## System & Prozesse

### `system_metrics`
System-Ressourcen abrufen (CPU, RAM, Disk, Netzwerk, Temperaturen).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `target` | enum | all, cpu, memory, disk, processes, host, sensors, network_detail |

### `process_analyzer`
Laufende Prozesse analysieren (Top CPU/Memory, Bäume, Details).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | top_cpu, top_memory, find, tree, info |
| `name` | string | Prozessname (für find) |
| `pid` | integer | Prozess-ID |

### `process_management`
Hintergrundprozesse verwalten (list, kill, status).

### `execute_shell`
Shell-Befehle ausführen (nur wenn `allow_shell` aktiviert).

### `execute_sudo`
Befehle mit sudo-Rechten ausführen (nur wenn `sudo_enabled`).

### `follow_up`
Autonome Hintergrundaufgabe planen.

### `wait_for_event`
Asynchron auf Ereignisse warten (Prozess-Ende, HTTP verfügbar, Datei-Änderung).

---

## Speicher & Gedächtnis

### `manage_memory`
Core Memory verwalten (permanente Fakten).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | add, update, delete, remove, list |
| `fact` | string | Faktischer Inhalt |
| `id` | string | Fakt-ID |

### `query_memory`
Alle Memory-Quellen durchsuchen (Vektor-DB, Knowledge Graph, Journal, Notizen).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `query` | string | Suchanfrage |
| `sources` | array | activity, vector_db, knowledge_graph, journal, notes, core_memory |
| `limit` | integer | Max. Ergebnisse |

### `context_memory`
Kontext-bewusste Memory-Abfrage mit Zeitfenster.

### `remember`
Automatisch in die richtige Memory-Quelle speichern.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `content` | string | Zu speichernde Information |
| `category` | enum | fact, event, task, relationship |
| `title` | string | Titel |

### `memory_reflect`
Reflexion über Memory-Aktivität (Muster, Widersprüche).

### `knowledge_graph`
Strukturierter Graph von Entitäten und Beziehungen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | add_node, add_edge, delete_node, delete_edge, search |
| `id` | string | Node-ID |
| `source` | string | Quell-Node (für Edges) |
| `target` | string | Ziel-Node |
| `relation` | string | Beziehungstyp |

### `cheatsheet`
Cheat Sheets verwalten (Schritt-für-Schritt Anleitungen).

### `manage_notes`
Notizen und To-Dos verwalten.

### `manage_journal`
Journal-Einträge verwalten (Ereignisse, Meilensteine).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | add, list, search, delete, get_summary |
| `title` | string | Titel |
| `entry_type` | enum | activity, reflection, milestone, preference, task_completed |
| `tags` | string | Komma-getrennte Tags |
| `importance` | integer | 1-5 |

### `secrets_vault`
Verschlüsselte Secrets im Vault speichern/abrufen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | get, set, delete, list |
| `key` | string | Secret-Name |
| `value` | string | Secret-Wert |

### `cron_scheduler`
Wiederkehrende Cron-Jobs planen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | add, list, remove, enable, disable |
| `cron_expr` | string | Cron-Ausdruck |
| `task_prompt` | string | Aufgabe |
| `label` | string | Beschriftung |

### `manage_missions`
Mission Control Hintergrund-Aufgaben verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | add, list, update, delete, run |
| `title` | string | Missions-Name |
| `command` | string | Aufgaben-Prompt |
| `cron_expr` | string | Cron-Ausdruck |

### `manage_daemon`
Daemon-Skills (langlaufende Hintergrundprozesse) verwalten.

### `context_manager`
Sitzungs-Kontexte verwalten und zwischen ihnen wechseln.

---

## Kommunikation & Medien

### `send_image`
Bild an Benutzer senden (lokaler Pfad oder URL).

### `send_audio`
Audio-Datei senden (inline-Player).

### `send_document`
Dokument senden (mit Open/Download Buttons).

### `analyze_image`
Bild analysieren (Vision LLM für OCR, Objekterkennung).

### `transcribe_audio`
Audio zu Text transkribieren.

### `generate_image`
KI-Bilder generieren (Text-zu-Bild, Bild-zu-Bild).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `prompt` | string | Bildbeschreibung |
| `size` | string | 1024x1024, 1344x768, 768x1344 |
| `quality` | enum | standard, hd |
| `style` | enum | natural, vivid |

### `media_registry`
Medien-Registry durchsuchen/verwalten (Bilder, TTS, Audio).

### `generate_music`
KI-Musik generieren (Text-zu-Musik).

---

## Geräteverwaltung

### `query_inventory`
Geräte-Inventar durchsuchen (nach Tag, Typ, Hostname).

### `register_device`
Neues Gerät zum Inventar hinzufügen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `hostname` | string | Gerätename |
| `device_type` | string | server, docker, vm, network_device |
| `ip_address` | string | IP-Adresse |
| `ssh_user` | string | SSH-Benutzername |
| `tags` | string | Komma-getrennte Tags |
| `mac_address` | string | MAC-Adresse für WOL |

### `wake_on_lan`
Gerät per Wake-on-LAN aufwecken.

### `address_book`
Adressbuch/Kontakte verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list, search, add, update, delete |
| `name` | string | Name |
| `email` | string | E-Mail |
| `phone` | string | Telefon |

### `remote_execution`
Befehl auf entferntem SSH-Server ausführen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `server_id` | string | Server-ID oder Hostname |
| `command` | string | Auszuführender Befehl |
| `direction` | enum | upload, download (für Datei-Transfer) |

### `transfer_remote_file`
Datei zu/von einem entfernten SSH-Server übertragen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `server_id` | string | Server-ID oder Hostname |
| `remote_path` | string | Pfad auf dem Remote-Server |
| `local_path` | string | Lokaler Pfad |
| `direction` | enum | upload, download |

### `remote_control`
Fernsteuerung verbundener Remote-Geräte.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list_devices, device_status, execute_command, read_file, write_file, edit_file |
| `device_id` | string | Geräte-ID |
| `command` | string | Shell-Befehl |

---

## Integrationen (Smart Home)

### `home_assistant`
Home Assistant Smart Home Geräte steuern.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | get_states, get_state, call_service, list_services |
| `entity_id` | string | z.B. light.living_room |
| `domain` | string | light, switch, climate, scene |
| `service` | string | turn_on, turn_off, toggle |

### `mqtt_publish`
MQTT-Nachricht publizieren.

### `mqtt_subscribe`
MQTT-Topic abonnieren.

### `mqtt_unsubscribe`
Von MQTT-Topic abmelden.

### `mqtt_get_messages`
Empfangene MQTT-Nachrichten abrufen.

### `fritzbox_system`
Fritz!Box System-Infos, Logs, Reboot.

### `fritzbox_network`
WLAN, Port-Forwarding, Wake-on-LAN.

### `fritzbox_telephony`
Anrufliste, Telefonbücher, AB-Nachrichten.

### `fritzbox_smarthome`
Smart Home Geräte steuern (Steckdosen, Thermostate, Lampen).

### `fritzbox_storage`
NAS/Storage Info, FTP-Server.

### `fritzbox_tv`
Fritz!Box TV-Stationen und Streaming-Info.

---

## Integrationen (Cloud & APIs)

### `api_request`
HTTP-Request an externe APIs.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `url` | string | Ziel-URL |
| `method` | enum | GET, POST, PUT, PATCH, DELETE |
| `headers` | object | HTTP-Header |
| `body` | string | Request-Body |

### `github`
GitHub Repositories, Issues, PRs, Commits verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list_repos, create_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits |
| `name` | string | Repository-Name |
| `owner` | string | GitHub Owner/Org |
| `title` | string | Issue-Titel |

### `google_workspace`
Gmail, Calendar, Drive, Docs, Sheets.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | gmail_list, gmail_read, gmail_send, gmail_modify_labels, calendar_list, calendar_create, drive_search, docs_get, sheets_get, sheets_update |

### `onedrive`
Microsoft OneDrive Dateien verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list, info, read, download, search, upload, delete, move, copy, share |
| `path` | string | Pfad in OneDrive |

### `s3_storage`
S3-kompatibler Storage (AWS, MinIO, Wasabi).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list_buckets, list_objects, upload, download, delete, copy, move |
| `bucket` | string | Bucket-Name |
| `key` | string | Objekt-Key |

### `netlify`
Netlify Sites, Deploys, Umgebungsvariablen verwalten.

### `tailscale`
Tailscale VPN-Netzwerk verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | devices, device, routes, enable_routes, disable_routes, dns, acl, local_status |

### `cloudflare_tunnel`
Cloudflare Tunnel (cloudflared) verwalten.

### `proxmox`
Proxmox VE VMs und Container verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | overview, list_nodes, list_vms, list_containers, status, start, stop, shutdown, reboot, create_snapshot, list_snapshots |
| `node` | string | Node-Name |
| `vmid` | string | VM/Container ID |

### `docker`
Docker Container, Images, Netzwerke, Volumes verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull |
| `container_id` | string | Container ID/Name |
| `image` | string | Docker Image |

### `ollama`
Lokale Ollama LLM-Instanz verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list, running, show, pull, delete, copy, load, unload |
| `model` | string | Modell-Name |

### `adguard`
AdGuard Home DNS-Server verwalten.

### `ansible`
Ansible Playbooks und Ad-hoc-Befehle ausführen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | status, list_playbooks, inventory, ping, adhoc, playbook, check, facts |
| `name` | string | Playbook-Datei |
| `hostname` | string | Ziel-Host-Pattern |
| `module` | string | Ansible-Modul |

### `meshcentral`
MeshCentral Geräte verwalten.

---

## Netzwerk & Sicherheit

### `dns_lookup`
DNS-Records abfragen (A, AAAA, MX, NS, TXT, CNAME, PTR).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `host` | string | Hostname |
| `record_type` | enum | all, A, AAAA, MX, NS, TXT, CNAME, PTR |

### `whois_lookup`
WHOIS-Informationen für Domains abrufen.

### `network_ping`
Ping zu Host senden (ICMP).

### `port_scanner`
TCP-Port-Scan auf Ziel-Host.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `host` | string | Ziel-Host |
| `port_range` | string | 80, 443, 1-1024, common |

### `mdns_scan`
mDNS/Bonjour-Geräte im lokalen Netzwerk suchen.

### `mac_lookup`
MAC-Adresse über ARP-Tabelle nachschlagen.

### `upnp_scan`
UPnP/SSDP-Geräte entdecken (Router, TVs, NAS).

### `virustotal_scan`
VirusTotal Threat Intelligence Scan.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `resource` | string | URL, Domain, IP, File Hash |
| `file_path` | string | Lokale Datei |
| `mode` | enum | auto, hash, upload |

### `firewall`
Linux Firewall (iptables/ufw) verwalten.

---

## Web & Scraping

### `web_scraper`
Webseite als reinen Text extrahieren (HTML → Text).

### `site_crawler`
Website crawlen (Links folgen, mehrere Seiten).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `url` | string | Start-URL |
| `max_depth` | integer | 1-5 |
| `max_pages` | integer | 1-100 |
| `allowed_domains` | string | Komma-getrennte Domains |

### `web_capture`
Screenshot oder PDF von URL erstellen (Chromium).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | screenshot, pdf |
| `url` | string | Ziel-URL |
| `selector` | string | CSS-Selektor (warten) |
| `full_page` | boolean | Ganze Seite scrollen |

### `web_performance_audit`
Core Web Vitals messen (TTFB, FCP, LCP, etc.).

### `form_automation`
Web-Formulare automatisch ausfüllen/absenden.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | get_fields, fill_submit, click |
| `url` | string | Seiten-URL |
| `fields` | string | JSON {selector: value} |
| `selector` | string | CSS-Selektor für Click |

### `site_monitor`
Webseiten auf Änderungen überwachen.

---

## Dokumente & Medien

### `document_creator`
PDFs erstellen, konvertieren, zusammenführen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | create_pdf, url_to_pdf, html_to_pdf, markdown_to_pdf, convert_document, merge_pdfs, screenshot_url, screenshot_html |
| `title` | string | Dokument-Titel |
| `content` | string | HTML/Markdown-Inhalt |
| `url` | string | URL für Screenshot/PDF |
| `paper_size` | enum | A4, A3, A5, Letter, Legal |
| `landscape` | boolean | Querformat |

### `pdf_operations`
PDFs manipulieren (merge, split, watermark, encrypt, fill forms).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | merge, split, watermark, compress, encrypt, decrypt, metadata, page_count, form_fields, fill_form |
| `file_path` | string | Eingabe-PDF |
| `output_file` | string | Ausgabe-PDF |
| `password` | string | Passwort |

### `image_processing`
Bilder verarbeiten (resize, convert, crop, rotate).

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | resize, convert, compress, crop, rotate, info |
| `file_path` | string | Bildpfad |
| `width` | integer | Ziel-Breite |
| `height` | integer | Ziel-Höhe |
| `quality_pct` | integer | 1-100 |
| `angle` | integer | 90, 180, 270 |

---

## Datenbanken

### `sql_query`
SQL-Query gegen registrierte Datenbank ausführen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | query, describe, list_tables |
| `connection_name` | string | Verbindungsname |
| `sql_query` | string | SQL-Statement |
| `table_name` | string | Tabellenname |

### `manage_sql_connections`
Externe Datenbank-Verbindungen verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list, get, create, update, delete, test, docker_create |
| `connection_name` | string | Name |
| `driver` | enum | postgres, mysql, sqlite |
| `host` | string | Datenbank-Host |
| `port` | integer | Port |
| `database_name` | string | Datenbank-Name |
| `allow_read/write/change/delete` | boolean | Berechtigungen |

---

## Infrastruktur

### `invasion_control'
Invasion Control (Remote Deployment) verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | list_nests, list_eggs, nest_status, assign_egg, hatch_egg, stop_egg, egg_status, send_task, send_secret |
| `nest_id` | string | Nest-ID |
| `egg_id` | string | Egg-ID |
| `task` | string | Aufgaben-Beschreibung |

### `co_agent`
Parallele Co-Agents spawnen und verwalten.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | spawn, spawn_specialist, list, get_result, stop, stop_all |
| `task` | string | Aufgabe |
| `specialist` | enum | researcher, coder, designer, security, writer |
| `priority` | integer | 1=low, 2=normal, 3=high |

### `homepage`
Homepage/Web-Projekte entwickeln, bauen, deployen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | enum | init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, deploy_netlify, dev, deploy, git_init, git_commit, git_status, git_diff, git_log |
| `framework` | enum | next, vite, astro, svelte, vue, html |
| `command` | string | Shell-Befehl |
| `project_dir` | string | Projekt-Unterverzeichnis |

### `homepage_registry`
Homepage-Projekte tracken (URL, Framework, Deploy-History).

### `manage_updates`
AuraGo Updates prüfen/installieren.

### `manage_webhooks`
Eingehende Webhooks verwalten.

### `call_webhook` / `manage_outgoing_webhooks`
Ausgehende Webhooks auslösen/verwalten.

### `telnyx_sms`
SMS/MMS via Telnyx senden.

### `telnyx_call`
Sprachanrufe via Telnyx initiieren/steuern.

### `telnyx_manage`
Telnyx Ressourcen verwalten (Nummern, Balance, History).

### `fetch_email`
E-Mails von registrierten Konten abrufen.

### `send_email`
E-Mails über registrierte Konten versenden.

### `list_email_accounts`
Registrierte E-Mail-Konten auflisten.

### `mcp_call`
Externe MCP (Model Context Protocol) Server aufrufen.

| Parameter | Typ | Beschreibung |
|-----------|-----|--------------|
| `operation` | string | list_servers, list_tools, call_tool |
| `server` | string | MCP-Server-Name |
| `tool_name` | string | Tool-Name |
| `mcp_args` | object | Argumente |

---

## Berechtigungs-System

Nicht alle Tools sind standardmäßig verfügbar. Die Verfügbarkeit hängt von den **Tool Feature Flags** ab:

### Always Available (Immer verfügbar)
- `filesystem` (read-only wenn `allow_filesystem_write=false`)
- `file_search`, `file_reader_advanced`, `smart_file_read`
- `system_metrics`, `process_analyzer`
- `analyze_image`, `transcribe_audio`
- `send_image`, `send_audio`, `send_document`
- `list_skills`, `execute_skill`
- `dns_lookup`, `whois_lookup`
- `detect_file_type`, `archive`
- `pdf_operations`, `image_processing`
- `tts`

### Danger Zone (Konfigurierbare Berechtigungen)
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
| `form_automation_enabled` | `form_automation` (benötigt zusätzlich `web_capture_enabled`) |
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

## Weiterführende Links

- [Tools Kapitel](06-tools.md) – Nutzung der Tools im Chat
- [Chat-Commands](20-chat-commands.md) – Manuelle Befehle
- [API Referenz](21-api-reference.md) – REST API
- [Sicherheit](14-sicherheit.md) – Berechtigungen & Danger Zone


---

## Zusätzliche Tools & Integrationen

### `tts`
Text-to-Speech Synthese über konfigurierbare Provider (ElevenLabs, Piper, MiniMax, Wyoming). Immer verfügbar.

### `chromecast`
Chromecast-Geräte im Netzwerk entdecken und steuern (Play, TTS, Discover).

### `brave_search`
Websuche über die Brave Search API.

### `ddg_search`
Websuche über DuckDuckGo (Instant Answers und HTML-Ergebnisse).

### `wikipedia_search`
Wikipedia-Artikel durchsuchen und auslesen.

### `scraper_summary`
Web-Scraping-Ergebnisse automatisch zusammenfassen und strukturieren.

### `truenas`
TrueNAS SCALE Storage verwalten (Pools, Datasets, Shares, Snapshots).

### `jellyfin`
Jellyfin Media Server steuern (Bibliotheken, Sessions, Wiedergabe).

### `koofr`
Koofr Cloud-Storage Dateioperationen (Listen, Upload, Download, Links).

### `pdf_extractor`
Text und Metadaten aus PDF-Dateien extrahieren (inkl. OCR-Unterstützung). Wird als Skill ausgeführt.
