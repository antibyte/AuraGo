# Kapitel 21: REST API Referenz

AuraGo bietet eine umfassende REST API für den programmatischen Zugriff auf alle Funktionen. Die API folgt den REST-Prinzipien und verwendet JSON für die Datenübertragung.

> 📅 **Stand:** 7. Juni 2026
> 🔌 **Basis-URL:** `http://localhost:8088` (Standard)

---

## Inhaltsverzeichnis

1. [Authentifizierung](#authentifizierung)
2. [Chat API](#chat-api)
3. [Memory API](#memory-api)
4. [Dashboard API](#dashboard-api)
5. [Config API](#config-api)
6. [Config Rules API](#config-rules-api)
7. [Vault API](#vault-api)
8. [Geräte API](#geräte-api)
9. [Credentials API](#credentials-api)
10. [Mission Control API](#mission-control-api)
11. [Container API](#container-api)
12. [Desktop API](#desktop-api)
13. [Pixel Bildeditor API](#pixel-bildeditor-api)
14. [Skills API](#skills-api)
15. [Knowledge API](#knowledge-api)
16. [Cheat Sheets API](#cheat-sheets-api)
17. [Contacts API](#contacts-api)
18. [SQL Connections API](#sql-connections-api)
19. [Cron API](#cron-api)
20. [Background Tasks API](#background-tasks-api)
21. [Indexing API](#indexing-api)
22. [Backup API](#backup-api)
23. [Update API](#update-api)
24. [Budget API](#budget-api)
25. [Upload API](#upload-api)
26. [Embeddings API](#embeddings-api)
27. [A2A Protocol API](#a2a-protocol-api)
28. [Invasion Control API](#invasion-control-api)
29. [Music Generation API](#music-generation-api)
30. [Video Generation API](#video-generation-api)
31. [Document Creator API](#document-creator-api)
32. [Knowledge Graph API](#knowledge-graph-api)
33. [Planner API](#planner-api)
34. [Helper LLM API](#helper-llm-api)
35. [Notification API](#notification-api)
36. [Push API](#push-api-pwa)
37. [Webhook API](#webhook-api)
38. [Token API](#token-api)
39. [Provider API](#provider-api)
40. [Ollama API](#ollama-api)
41. [Security Proxy API](#security-proxy-api)
42. [OpenRouter API](#openrouter-api)
43. [Personality API](#personality-api)
44. [Sandbox API](#sandbox-api)
45. [Security API](#security-api)
46. [System API](#system-api)
47. [i18n API](#i18n-api)
48. [Setup API](#setup-api)
49. [Health Check](#health-check)
50. [Agent Skills API](#agent-skills-api)
51. [Daemon Skills API](#daemon-skills-api)
52. [Agent Questions API](#agent-questions-api)
53. [People API](#people-api)
54. [Appointments API](#appointments-api)
55. [Todos API](#todos-api)
56. [Preferences API](#preferences-api)
57. [Space Agent API](#space-agent-api)
58. [Warnings API](#warnings-api)
59. [SSE Events](#sse-events)
60. [Fehlerbehandlung](#fehlerbehandlung)

---

## Authentifizierung

### Auth-Status prüfen
```http
GET /api/auth/status
```

**Antwort:**
```json
{
  "enabled": true,
  "password_set": true,
  "totp_enabled": false,
  "authenticated": true
}
```

### Passwort setzen
```http
POST /api/auth/password
Content-Type: application/json

{
  "password": "neues-geheim",
  "current_password": "altes-geheim"
}
```

Setzt oder ändert das Login-Passwort. Ohne Authentifizierung nur erlaubt, wenn noch kein Passwort gesetzt ist; sonst aktive Session erforderlich.

### TOTP einrichten
```http
GET /api/auth/totp/setup
```

Erzeugt ein neues TOTP-Secret und gibt die `otpauth`-URI zurück. Erfordert Authentifizierung. Aktiviert 2FA erst nach Bestätigung.

### TOTP bestätigen
```http
POST /api/auth/totp/confirm
Content-Type: application/json

{
  "code": "123456"
}
```

Verifiziert den ersten TOTP-Code und aktiviert Zwei-Faktor-Authentifizierung.

### TOTP deaktivieren
```http
DELETE /api/auth/totp
```

Deaktiviert TOTP. Erfordert Authentifizierung.

### Login
```http
POST /auth/login
Content-Type: application/x-www-form-urlencoded

password=geheim&totp_code=123456
```

**Antwort:**
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### Logout
```http
POST /api/auth/logout
```

---

## Chat API

### Chat Completion (OpenAI-kompatibel)
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "messages": [
    {"role": "user", "content": "Hallo AuraGo!"}
  ],
  "stream": true
}
```

**Antwort:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hallo! Wie kann ich dir helfen?"
      },
      "finish_reason": "stop"
    }
  ]
}
```

### Chat-Verlauf abrufen
```http
GET /history
```

**Antwort:**
```json
[
  {
    "id": "msg-123",
    "role": "user",
    "content": "Hallo!",
    "timestamp": "2026-03-28T10:00:00Z"
  }
]
```

### Chat-Verlauf löschen
```http
DELETE /clear
```

### Aktuelle Aktion unterbrechen
```http
POST /api/admin/stop
```

### Chat-Sessions auflisten
```http
GET /api/chat/sessions
```

Gibt aktuelle Chat-Sessions aus dem Kurzzeitgedächtnis zurück.

### Chat-Session erstellen
```http
POST /api/chat/sessions
```

Erstellt eine neue Chat-Session und gibt deren Metadaten zurück.

### Chat-Session abrufen
```http
GET /api/chat/sessions/{id}
```

Gibt Session-Metadaten und sichtbare Nachrichten zurück.

### Chat-Session löschen
```http
DELETE /api/chat/sessions/{id}
```

---

## Memory API

### Gedächtnis archivieren
```http
POST /api/memory/archive
Content-Type: application/json

{
  "session_id": "default",
  "before_message_id": 123
}
```

**Antwort:**
```json
{
  "success": true,
  "archived": 5
}
```

### Memory-Aktivitätsübersicht
```http
GET /api/memory/activity-overview
```

---

## Dashboard API

### System-Metriken
```http
GET /api/dashboard/system
```

**Antwort:**
```json
{
  "cpu": 15.2,
  "memory": 45.8,
  "disk": 72.1,
  "network": {
    "rx_mbps": 1.2,
    "tx_mbps": 0.8
  },
  "uptime_seconds": 86400
}
```

### Stimmungsverlauf
```http
GET /api/dashboard/mood-history
```

### Emotionsverlauf
```http
GET /api/dashboard/emotion-history
```

### Memory-Übersicht
```http
GET /api/dashboard/memory
```

### Core Memory
```http
GET /api/dashboard/core-memory
```

### Core Memory mutieren
```http
POST /api/dashboard/core-memory/mutate
Content-Type: application/json

{
  "action": "add",
  "content": "Wichtige Information"
}
```

### Profil-Daten
```http
GET /api/dashboard/profile
PUT /api/dashboard/profile/entry
```

### Aktivitäts-Statistiken
```http
GET /api/dashboard/activity
```

Gibt Laufzeit-Aktivität zurück, inkl. Cron-Jobs, Prozesse, Webhooks, Background-Tasks und aktiver **Co-Agenten** (Feld `coagents`).

### GitHub Repositories
```http
GET /api/dashboard/github-repos
GET /api/github/repos
```

### Logs
```http
GET /api/dashboard/logs
```

### Dashboard-Übersicht
```http
GET /api/dashboard/overview
```

### Notizen
```http
GET /api/dashboard/notes
```

### Journal
```http
GET /api/dashboard/journal
GET /api/dashboard/journal/summaries
GET /api/dashboard/journal/stats
```

### Guardian-Status
```http
GET /api/dashboard/guardian
```

### Fehler-Übersicht
```http
GET /api/dashboard/errors
```

### Prompt-Statistiken
```http
GET /api/dashboard/prompt-stats
```

### Tool-Statistiken
```http
GET /api/dashboard/tool-stats
```

---

## Config API

### Konfiguration abrufen
```http
GET /api/config
```

### Konfiguration aktualisieren
```http
PUT /api/config
Content-Type: application/json

{
  "server": {
    "host": "0.0.0.0",
    "port": 8088
  }
}
```

**Antwort:**
```json
{
  "success": true,
  "restart_required": false
}
```

### Konfigurations-Schema
```http
GET /api/config/schema
```

### UI-Sprache ändern
```http
POST /api/ui-language
Content-Type: application/json

{
  "language": "en"
}
```

---

## Config Rules API

Task Rules sind editierbare Markdown-Leitplanken unter `prompts/rules/<id>/rule.md`. Alle Endpunkte benötigen Admin-Zugriff.

### Regeln auflisten
```http
GET /api/config/rules
```

Gibt den Aktivstatus, Regel-Metadaten und Kandidaten für Tools/Workflows im Regeleditor zurück.

### Regel erstellen
```http
POST /api/config/rules
Content-Type: application/json

{
  "id": "homepage",
  "title": "Homepage-Regeln",
  "enabled": true,
  "priority": 50,
  "tools": ["homepage"],
  "workflows": ["website"],
  "keywords": ["landing page"],
  "body": "Markdown-Regeltext",
  "design": "Optionaler DESIGN.md-Inhalt"
}
```

### Regel abrufen / aktualisieren / löschen
```http
GET /api/config/rules/{id}
PUT /api/config/rules/{id}
DELETE /api/config/rules/{id}
```

### Eingebaute Regel wiederherstellen
```http
POST /api/config/rules/{id}/restore
```

Entfernt den Disk-Override, sodass wieder die eingebettete Regel verwendet wird.

---

## Vault API

### Vault-Status
```http
GET /api/vault/status
```

**Antwort:**
```json
{
  "initialized": true,
  "secret_count": 12
}
```

### Secrets auflisten
```http
GET /api/vault/secrets
```

### Secret löschen
```http
DELETE /api/vault?key=secret_name
```

---

## Geräte API (Device Registry)

### Geräte auflisten
```http
GET /api/devices
```

**Antwort:**
```json
{
  "devices": []
}
```

### Gerät erstellen
```http
POST /api/devices
Content-Type: application/json

{
  "hostname": "server1",
  "device_type": "server",
  "ip_address": "192.168.1.10",
  "ssh_port": 22,
  "ssh_user": "root",
  "tags": ["produktion", "web"]
}
```

**Antwort:**
```json
{
  "id": "dev-123",
  "success": true
}
```

### Gerät abrufen
```http
GET /api/devices/{id}
```

### Gerät aktualisieren
```http
PUT /api/devices/{id}
Content-Type: application/json

{
  "tags": ["neu", "aktualisiert"]
}
```

### Gerät löschen
```http
DELETE /api/devices/{id}
```

### MAC Lookup
```http
GET /api/tools/mac_lookup?mac=aa:bb:cc:dd:ee:ff
```

---

## Credentials API

### Credentials auflisten
```http
GET /api/credentials
```

### Python-zugängliche Credentials
```http
GET /api/credentials/python-accessible
```

### Credential erstellen
```http
POST /api/credentials
Content-Type: application/json

{
  "name": "api_key_production",
  "type": "token",
  "value": "sk-...",
  "python_accessible": false
}
```

### Credential aktualisieren/löschen
```http
PUT /api/credentials/{id}
DELETE /api/credentials/{id}
```

---

## Mission Control API

### Missionen auflisten
```http
GET /api/missions
GET /api/missions/v2
```

### Mission erstellen
```http
POST /api/missions/v2
Content-Type: application/json

{
  "name": "Backup erstellen",
  "description": "Tägliches Backup der Datenbank",
  "trigger": {
    "type": "cron",
    "schedule": "0 2 * * *"
  },
  "actions": [
    {
      "type": "shell",
      "command": "pg_dump mydb > backup.sql"
    }
  ]
}
```

**Antwort:**
```json
{
  "id": "mission-456",
  "status": "queued"
}
```

### Mission verwalten
```http
GET /api/missions/v2/{id}
PUT /api/missions/v2/{id}
DELETE /api/missions/v2/{id}
```

### Warteschlange
```http
GET /api/missions/v2/queue
```

### Ausführungen
```http
GET /api/missions/v2/execution
```

### Abhängigkeiten
```http
GET /api/missions/v2/dependencies
```

---

## Container API

### Container auflisten
```http
GET /api/containers
```

**Antwort:**
```json
{
  "containers": []
}
```

### Container-Aktionen
```http
POST /api/containers/{id}/start
POST /api/containers/{id}/stop
POST /api/containers/{id}/restart
POST /api/containers/{id}/pause
POST /api/containers/{id}/unpause
DELETE /api/containers/{id}
```

### Runtime-Informationen
```http
GET /api/runtime
```

---

## Desktop API

Desktop-Endpunkte verwenden dasselbe Session-/Auth-Modell wie die Web-UI. Schreibende Operationen benötigen Desktop-Admin-/Write-Rechte und beachten `virtual_desktop.readonly`.

### Desktop-Status
```http
GET /api/desktop/apps
GET /api/desktop/shortcuts
GET /api/desktop/widgets
GET /api/desktop/settings
POST /api/desktop/settings
GET /api/desktop/embed-token
```

### Desktop-Chat und Streams
```http
POST /api/desktop/chat
GET /api/desktop/chat/stream
GET /api/desktop/ws
GET /api/agodesk/ws
GET /api/agodesk/media/{bucket}/{path}
```

`/api/agodesk/ws` nutzt das AgoDesk-JSON-Envelope-Protokoll. Produktionsclients koppeln sich mit `session.start`; lokale Entwicklung kann `?insecure_loopback=1` verwenden. Nach `session.accepted` speichert AgoDesk `advertised_capabilities` und nutzt die akzeptierte `session_id` als Transport-Session.

AgoDesk-Chat kann dieselben AuraGo-Webchat-Sessions nutzen, wenn `chat.sessions` ausgehandelt ist:

- `chat.sessions.list` liefert die letzten `sess-*`-Konversationen.
- `chat.session.create` startet New Chat und liefert `chat.session`.
- `chat.session.load` lädt eine Konversation und sichtbare, nicht-interne Nachrichten.
- `chat.message` sollte die AgoDesk-`session_id` und die aktive `conversation_id` senden.
- `chat.cancel` stoppt den aktiven Request und liefert `chat.cancelled`.
- `chat.audio` wird nur gesendet, wenn `chat.audio_events` ausgehandelt ist. AuraGo-generierte TTS-Pfade nutzen `/api/agodesk/tts/<filename>`, damit AgoDesk Audio ohne Web-UI-Login-Cookie abrufen kann. `chat.voice_output` wird nur angeboten, wenn AuraGo-TTS konfiguriert ist.
- `chat.media` wird gesendet, wenn `chat.media_events` ausgehandelt ist, und transportiert Nicht-TTS-Bilder, Audio/Musik, Dokumente, Videos, STL, Links und YouTube-Embeds. Geschützte `/files/...` Assets werden für explizite Buckets wie `images`, `generated_images`, `generated_videos`, `audio`, `documents` und `downloads` auf kurzlebig signierte `/api/agodesk/media/...` URLs umgeschrieben; Clients müssen die gelieferten Query-Parameter beibehalten und Media-Metadaten nach `401` neu laden.
- `chat.voice_output.status` erlaubt AgoDesk, dieselbe `speaker_mode`-Präferenz wie der Webchat zu melden, wenn Sprachausgabe aktiviert oder deaktiviert wird.
- `integrations.webhosts.list` liefert bei ausgehandeltem `integrations.webhosts` dieselbe Integrations-Webhost-Liste wie der Webchat-Integrationen-Drawer.
- `system.warnings.list` und `system.warning.acknowledge` liefern bei ausgehandeltem `system.warnings` die Systemwarnungen des Webchats und den Acknowledge-Flow.

Details zu Payloads und Client-Checkliste stehen im [AgoDesk Backend Protocol](../../agodesk_backend_protocol.md).

### Remote-Desktop-Proxys
```http
GET /api/desktop/ssh
GET /api/desktop/vnc
```

### Desktop Software Store
```http
GET /api/desktop/store/catalog
GET /api/desktop/store/apps
POST /api/desktop/store/install
GET /api/desktop/store/operations/{operation_id}
POST /api/desktop/store/apps/{app_id}/{start|stop|restart|update}
DELETE /api/desktop/store/apps/{app_id}?delete_data=false
GET /api/desktop/store/apps/{app_id}/open-url?port_id=web
GET /api/desktop/store/apps/{app_id}/credentials
POST /api/desktop/store/apps/beszel/companions/agent/config
```

Install-Request:
```json
{
  "app_id": "termix",
  "bind_mode": "local"
}
```

Store-Mutationen geben `202 Accepted` mit einem Operation-Objekt zurück. Poll `/api/desktop/store/operations/{operation_id}`, bis die Operation einen finalen Status erreicht.

### Code Studio
```http
GET /api/code-studio/status
GET /api/code-studio/files?path=/workspace
GET /api/code-studio/file?path=/workspace/main.go
PUT /api/code-studio/file
PATCH /api/code-studio/file
DELETE /api/code-studio/file?path=/workspace/main.go
POST /api/code-studio/directory
POST /api/code-studio/upload
GET /api/code-studio/download?path=/workspace/main.go
POST /api/code-studio/exec
GET /api/code-studio/search?q=needle&path=/workspace
GET /api/code-studio/terminal
```

Code-Studio-Pfade werden im Container-Workspace sanitisiert und unter `/workspace` gemountet.

---

## Pixel Bildeditor API

Pixel ist der Desktop-Bildeditor. Er kann Canvas-Daten in den Virtual-Desktop-Workspace speichern und optional den konfigurierten Image-Generation-Provider nutzen.

### Fähigkeiten
```http
GET /api/pixel/config
```

Gibt zurück, ob Bildgenerierung aktiv ist, welchen Provider/welches Modell AuraGo nutzt, welche Standardgröße/-qualität/-style gesetzt sind und ob Image-to-Image unterstützt wird.

### Bild generieren
```http
POST /api/pixel/generate
Content-Type: application/json

{
  "prompt": "A clean product photo of a brass lamp",
  "size": "1024x1024",
  "quality": "standard",
  "style": "natural"
}
```

### Bild verbessern
```http
POST /api/pixel/enhance
Content-Type: application/json

{
  "source_path": "agent_workspace/virtual_desktop/image.png",
  "prompt": "Improve sharpness and lighting",
  "strength": 0.7
}
```

`source_data` kann statt `source_path` als Base64-Data-URL übergeben werden.

### Canvas speichern
```http
POST /api/pixel/save
Content-Type: application/json

{
  "path": "Images/edited.png",
  "data": "data:image/png;base64,...",
  "format": "png",
  "quality": 92
}
```

Relative Pfade werden unter dem konfigurierten Data-Workspace gespeichert.

---

## Skills API

### Skills auflisten
```http
GET /api/skills
```

**Antwort:**
```json
{
  "skills": []
}
```

### Skill erstellen
```http
POST /api/skills
Content-Type: application/json

{
  "name": "Mein Skill",
  "description": "Beschreibung",
  "code": "..."
}
```

**Antwort:**
```json
{
  "id": "skill-789",
  "success": true
}
```

### Skill verwalten
```http
GET /api/skills/{id}
PUT /api/skills/{id}
DELETE /api/skills/{id}
```

### Skill testen
```http
POST /api/skills/{id}/test
```

### Skill verifizieren
```http
POST /api/skills/{id}/verify
```

### Skill exportieren
```http
GET /api/skills/{id}/export
```

### Skill-Versionen
```http
GET /api/skills/{id}/versions
```

### Skill-Audit
```http
GET /api/skills/{id}/audit
```

### Templates
```http
GET /api/skills/templates
POST /api/skills/templates
```

### Skill importieren
```http
POST /api/skills/import
Content-Type: multipart/form-data
```

### Skill-Entwurf generieren
```http
POST /api/skills/generate
Content-Type: application/json

{
  "description": "Ein Skill der Dateien analysiert"
}
```

### Skill-Statistiken
```http
GET /api/skills/stats
```

### Skill-Handbuch abrufen
```http
GET /api/skills/{id}/documentation
```

**Antwort:**
```json
{
  "status": "ok",
  "skill_id": "user_mein_skill_1714123456789",
  "has_documentation": true,
  "content": "# mein_skill\n\nMacht nützliche Dinge.\n"
}
```

Gibt `has_documentation: false` und `content: ""` zurück, wenn kein Handbuch vorhanden ist.
Gibt `404` zurück, wenn die Skill-ID nicht gefunden wurde.

### Skill-Handbuch erstellen / ersetzen
```http
PUT /api/skills/{id}/documentation
Content-Type: application/json

{
  "content": "# mein_skill\n\nMarkdown-Inhalt hier.\n"
}
```

**Antwort:**
```json
{ "status": "saved" }
```

| Status | Bedeutung |
|--------|-----------|
| `200`  | Handbuch gespeichert |
| `400`  | Skill-ID fehlt oder Inhalt ungültig |
| `403`  | Skill Manager ist im Nur-Lesen-Modus |
| `413`  | Inhalt überschreitet 64-KB-Grenze |

Ein leerer `content`-Wert löscht das vorhandene Handbuch.

### Skill-Handbuch löschen
```http
DELETE /api/skills/{id}/documentation
```

**Antwort:**
```json
{ "status": "deleted" }
```

Gibt `403` zurück, wenn der Skill Manager im Nur-Lesen-Modus ist. Das Löschen eines nicht vorhandenen Handbuchs ist eine No-Op.

### Skill-Handbuch-Datei hochladen
```http
POST /api/skills/{id}/documentation/upload
Content-Type: multipart/form-data

file=<binary>  (Feldname: "file")
```

Akzeptiert `.md`-, `.markdown`- und `.txt`-Dateien. Maximale Größe: 64 KB.

**Antwort:**
```json
{ "status": "uploaded" }
```

| Status | Bedeutung |
|--------|-----------|
| `200`  | Datei gespeichert |
| `400`  | Falsche Dateiendung |
| `403`  | Nur-Lesen-Modus oder Uploads deaktiviert (`allow_uploads: false`) |
| `413`  | Datei überschreitet 64-KB-Grenze |

### Daemon-Skill-Einstellungen
```http
GET /api/skills/{id}/daemon
PUT /api/skills/{id}/daemon
```

Liest oder aktualisiert daemon-spezifische Manifest-Einstellungen (`wake_agent`, `trigger_mission_id`, `cheatsheet_id`) für Daemon-Skills.

---

## Knowledge API

### Knowledge-Dateien
```http
GET /api/knowledge
POST /api/knowledge/upload
Content-Type: multipart/form-data
```

### Knowledge-Datei verwalten
```http
GET /api/knowledge/{id}
DELETE /api/knowledge/{id}
```

---

## Cheat Sheets API

### Cheat Sheets
```http
GET /api/cheatsheets
POST /api/cheatsheets
```

**Antwort (GET):**
```json
{
  "cheatsheets": []
}
```

### Cheat Sheet verwalten
```http
GET /api/cheatsheets/{id}
PUT /api/cheatsheets/{id}
DELETE /api/cheatsheets/{id}
```

---

## Contacts API

### Kontakte
```http
GET /api/contacts
POST /api/contacts
```

**Antwort (GET):**
```json
{
  "contacts": []
}
```

### Kontakt verwalten
```http
GET /api/contacts/{id}
PUT /api/contacts/{id}
DELETE /api/contacts/{id}
```

---

## SQL Connections API

### Verbindungen auflisten
```http
GET /api/sql-connections
```

### Verbindung erstellen
```http
POST /api/sql-connections
Content-Type: application/json

{
  "name": "Produktions-DB",
  "type": "postgres",
  "host": "localhost",
  "port": 5432,
  "database": "mydb"
}
```

**Antwort:**
```json
{
  "id": "sql-001",
  "success": true
}
```

### Verbindung testen
```http
POST /api/sql-connections/{id}/test
```

### Verbindung verwalten
```http
GET /api/sql-connections/{id}
PUT /api/sql-connections/{id}
DELETE /api/sql-connections/{id}
```

---

## Cron API

### Cron-Jobs verwalten
```http
GET /api/cron
POST /api/cron
PUT /api/cron
DELETE /api/cron?id={id}
DELETE /api/cron/{id}
```

Benötigt eine Admin-Sitzung. Der Legacy-Handler ordnet `GET=list`, `POST=add`, `PUT=update` und `DELETE=remove` zu; Fehler nutzen HTTP 400/404/500 statt einer erfolgreichen Antwort mit Fehlertext.

**Antwort (GET):**
```json
{
  "jobs": []
}
```

---

## Background Tasks API

### Aufgaben auflisten
```http
GET /api/background-tasks
```

### Aufgabe abrufen
```http
GET /api/background-tasks/{id}
```

---

## Indexing API

### Indexing-Status
```http
GET /api/indexing/status
```

### Neu scannen
```http
POST /api/indexing/rescan
```

### Verzeichnisse verwalten
```http
GET /api/indexing/directories
POST /api/indexing/directories
```

---

## Backup API

AuraGo-Backups verwenden das `.ago`-Format. Archive sind ZIP-basiert und können mit AES-256-GCM und Argon2id-abgeleiteten Schlüsseln verschlüsselt werden. Sie können Konfiguration, konsistente SQLite-Datenbank-Snapshots, VectorDB-Daten, Skills, Tools, ausgewählte Workspace-Dateien und separat verschlüsselte Vault-Secrets für Instanzmigrationen enthalten.

### Backup erstellen
```http
POST /api/backup/create
Content-Type: application/json

{
  "include_vectordb": true,
  "include_workdir": false,
  "password": "starkes-passwort"
}
```

Gibt das `.ago`-Archiv als `application/octet-stream`-Download mit `Content-Disposition`-Dateiname zurück. Lass `password` leer für ein unverschlüsseltes, ZIP-kompatibles `.ago`; setze es, um das Archiv zu verschlüsseln und portable Vault-/Token-Exporte einzuschließen.

### Backup importieren
```http
POST /api/backup/import
Content-Type: multipart/form-data
```

Form-Felder:

| Feld | Pflicht | Beschreibung |
|------|---------|--------------|
| `file` | ja | `.ago`-Backup-Archiv |
| `password` | nein | Passwort für verschlüsselte Backups |

Importe werden zuerst gestaged, auf Path-Traversal, Schema-Warnungen und Archiv-Kompatibilität geprüft und danach so atomar wie möglich wiederhergestellt. Ein SQLite-Restore gibt `restart_required: true` zurück.

---

## Update API

### Update prüfen
```http
GET /api/updates/check
```

### Update installieren
```http
POST /api/updates/install
```

### Neustart
```http
POST /api/restart
```

---

## Budget API

### Budget-Status
```http
GET /api/budget
```

**Antwort:**
```json
{
  "today_usd": 0.5,
  "limit_usd": 5.0
}
```

### OpenRouter Credits
```http
GET /api/credits
```

---

## Upload API

### Datei hochladen
```http
POST /api/upload
Content-Type: multipart/form-data
```

### Sprachnachricht hochladen
```http
POST /api/upload-voice
Content-Type: multipart/form-data
```

---

## Embeddings API

### Embeddings zurücksetzen
```http
POST /api/embeddings/reset
```

---

## A2A Protocol API

### A2A Status
```http
GET /api/a2a/status
```

### Remote Agents
```http
GET /api/a2a/remote-agents
POST /api/a2a/remote-agents
PUT /api/a2a/remote-agents/{id}
DELETE /api/a2a/remote-agents/{id}
```

### Agent Card
```http
GET /api/a2a/card
```

### A2A Test
```http
POST /api/a2a/test
```

---

## Invasion Control API

### Nests
```http
GET /api/invasion/nests
```

### Eggs
```http
GET /api/invasion/eggs
POST /api/invasion/eggs
PUT /api/invasion/eggs/{id}
DELETE /api/invasion/eggs/{id}
```

### WebSocket
```http
WS /api/invasion/ws
```

---

## Music Generation API

### Test Connection
```http
POST /api/music-generation/test
```

---

## Video Generation API

### Verbindung testen
```http
POST /api/video-generation/test
```

Validiert Provider-Konfiguration und Credential-Verfügbarkeit für das konfigurierte Video-Generation-Backend.

---

## Document Creator API

### Test Gotenberg Connection
```http
POST /api/document-creator/test
```

---

## Knowledge Graph API

### Nodes
```http
GET /api/knowledge-graph/nodes
POST /api/knowledge-graph/nodes
```

### Edges
```http
GET /api/knowledge-graph/edges
POST /api/knowledge-graph/edges
```

### Node Detail
```http
GET /api/knowledge-graph/node/{id}
POST /api/knowledge-graph/node/protect
```

### Edge Mutate
```http
POST /api/knowledge-graph/edge
```

### Search / Stats / Quality
```http
GET /api/knowledge-graph/search
GET /api/knowledge-graph/stats
GET /api/knowledge-graph/quality
GET /api/knowledge-graph/important
```

`/api/knowledge-graph/quality` meldet isolierte Nodes, untypisierte Nodes und mögliche Duplikate. `POST /api/knowledge-graph/node/protect` markiert wichtige Nodes als geschützt, damit automatische Bereinigung sie nicht versehentlich entfernt.

### File-Sync-Debugging
```http
GET /api/debug/kg-file-sync-stats
GET /api/debug/kg-orphans
GET /api/debug/file-sync-status
GET /api/debug/file-sync-last-run
GET /api/debug/kg-file-entities
GET /api/debug/kg-node-sources
POST /api/debug/kg-file-sync-cleanup
```

Diese Endpunkte prüfen und warten den Hintergrunddienst File KG Sync, der Entitäten und Beziehungen aus indexierten Dateien in den Knowledge Graph extrahiert.

---

## Planner API

### Pläne auflisten
```http
GET /api/plans
```

**Antwort:**
```json
{
  "plans": []
}
```

### Aktiven Plan abrufen
```http
GET /api/plans/active
```

### Plan verwalten
```http
GET /api/plans/{id}
PUT /api/plans/{id}
DELETE /api/plans/{id}
```

---

## Helper LLM API

### Stats
```http
GET /api/debug/helper-llm/stats
GET /api/dashboard/helper-llm
```

---

## Notification API

### Benachrichtigungen
```http
GET /notifications
POST /notifications/read
```

---

## Push API (PWA)

### VAPID Public Key
```http
GET /api/push/vapid-pubkey
```

### Push abonnieren
```http
POST /api/push/subscribe
Content-Type: application/json

{
  "endpoint": "https://push.service/...",
  "keys": {
    "p256dh": "...",
    "auth": "..."
  }
}
```

### Push abbestellen
```http
POST /api/push/unsubscribe
```

### Push-Status
```http
GET /api/push/status
```

---

## Webhook API

### Webhooks auflisten
```http
GET /api/webhooks
```

**Antwort:**
```json
{
  "webhooks": []
}
```

### Webhook erstellen
```http
POST /api/webhooks
Content-Type: application/json

{
  "name": "GitHub Webhook",
  "url": "https://example.com/webhook",
  "events": ["push", "pull_request"]
}
```

**Antwort:**
```json
{
  "id": "wh-001",
  "success": true
}
```

### Webhook verwalten
```http
GET /api/webhooks/{id}
PUT /api/webhooks/{id}
DELETE /api/webhooks/{id}
```

### Webhook-Log
```http
GET /api/webhooks/{id}/log
GET /api/webhooks/log
```

### Webhook testen
```http
POST /api/webhooks/{id}/test
```

### Webhook-Presets
```http
GET /api/webhooks/presets
```

### Webhook-Receiver (öffentlich)
```http
POST /webhook/{slug}
```

---

## Token API

### Tokens auflisten
```http
GET /api/tokens
```

### Token erstellen
```http
POST /api/tokens
Content-Type: application/json

{
  "name": "API-Zugriff",
  "scopes": ["read", "write"]
}
```

**Antwort:**
```json
{
  "id": "token-001",
  "token": "agt_xxxxxxxxxxxxxxxx"
}
```

### Token verwalten
```http
PUT /api/tokens/{id}
DELETE /api/tokens/{id}
```

---

## Provider API

### Provider auflisten
```http
GET /api/providers
```

### Provider-Pricing
```http
GET /api/providers/pricing
```

---

## Ollama API

### Modelle auflisten
```http
GET /api/ollama/models
```

### Managed Ollama Status
```http
GET /api/ollama/managed/status
```

Gibt Status des verwalteten Docker-Containers, erkannte Runtime, Modell-Volume-Status, verfügbare GPU-Unterstützung soweit erkennbar und konfigurierte Standardmodelle zurück.

### Managed Ollama neu erstellen
```http
POST /api/ollama/managed/recreate
```

Erstellt den verwalteten Container `aurago_ollama_managed` nach Konfigurationsänderungen neu.

---

## Security Proxy API

### Proxy-Lifecycle
```http
GET /api/proxy/status
POST /api/proxy/start
POST /api/proxy/stop
POST /api/proxy/destroy
POST /api/proxy/reload
GET /api/proxy/logs
```

Die Security Proxy API steuert die verwaltete Caddy-Schutzschicht für Rate-Limiting, TLS-Terminierung, IP-Filter, Geo-Blocking und öffentliche Härtung.

---

## OpenRouter API

### OpenRouter Modelle
```http
GET /api/openrouter/models
```

---

## Personality API

### Persönlichkeiten auflisten
```http
GET /api/personalities
```

### Persönlichkeit aktualisieren
```http
POST /api/personality
Content-Type: application/json

{
  "core_personality": "tech"
}
```

**Antwort:**
```json
{
  "success": true
}
```

### Persönlichkeits-Status
```http
GET /api/personality/state
```

### Persönlichkeits-Feedback
```http
POST /api/personality/feedback
Content-Type: application/json

{
  "mood": "happy",
  "trigger": "positive_interaction"
}
```

### Persönlichkeits-Dateien
```http
GET /api/config/personality-files
POST /api/config/personality-files
DELETE /api/config/personality-files
```

---

## Sandbox API

### Sandbox-Status
```http
GET /api/sandbox/status
```

### Shell-Sandbox-Status
```http
GET /api/sandbox/shell-status
```

---

## Security API

### Sicherheits-Status
```http
GET /api/security/status
```

### Sicherheitshinweise
```http
GET /api/security/hints
```

### Auto-Hardening
```http
POST /api/security/harden
```

---

## System API

### Betriebssystem
```http
GET /api/system/os
```

### Runtime-Umgebung
```http
GET /api/runtime
```

---

## i18n API

### Übersetzungen
```http
GET /api/i18n?lang=de
```

---

## Setup API

### Setup-Status
```http
GET /api/setup/status
```

### Setup-Profile
```http
GET /api/setup/profiles
```

Gibt vorkonfigurierte Provider-Profile für die Plan-Auswahl im Setup-Wizard zurück.

### Provider-Verbindung testen
```http
POST /api/setup/test
Content-Type: application/json

{
  "provider_type": "openrouter",
  "base_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-...",
  "model": "openai/gpt-4o"
}
```

Führt einen leichtgewichtigen LLM-Verbindungstest vor dem Speichern durch. Nur verfügbar, solange Setup noch nicht abgeschlossen ist.

### Setup speichern
```http
POST /api/setup
Content-Type: application/json

{
  "llm_provider": "openrouter",
  "api_key": "sk-or-..."
}
```

**Antwort:**
```json
{
  "success": true,
  "restart_required": true
}
```

Erfordert den CSRF-Token von `GET /api/setup/status`.

---

## Health Check

### Health
```http
GET /api/health
```

**Antwort:**
```json
{
  "status": "ok"
}
```

### Readiness
```http
GET /api/ready
```

Gibt `200` mit `{"status":"ready"}` zurück, wenn die Initialisierung abgeschlossen ist, oder `503` mit `{"status":"initializing"}` während des Starts. Wird von Docker-Healthchecks und Load-Balancern genutzt.

---

## Agent Skills API

Agent Skills sind dateisystembasierte Skills, getrennt vom klassischen Skill Manager.

### Agent Skills auflisten
```http
GET /api/agent-skills
GET /api/agent-skills?enabled=true&search=keyword
```

### Agent Skill erstellen
```http
POST /api/agent-skills
Content-Type: application/json

{
  "name": "mein_skill",
  "description": "Macht nützliche Dinge",
  "body": "# SKILL.md Inhalt"
}
```

### Agent Skill importieren
```http
POST /api/agent-skills/import
Content-Type: multipart/form-data
```

### Agent Skill verwalten
```http
GET /api/agent-skills/{id}
GET /api/agent-skills/{id}?content=true
PUT /api/agent-skills/{id}
DELETE /api/agent-skills/{id}
```

### Verifizieren / Freigeben / Testen
```http
POST /api/agent-skills/{id}/verify
POST /api/agent-skills/{id}/approve-warning
POST /api/agent-skills/{id}/test
```

### Agent-Skill-Dateien
```http
GET /api/agent-skills/{id}/files?path=scripts/run.py
PUT /api/agent-skills/{id}/files
POST /api/agent-skills/{id}/files
```

---

## Daemon Skills API

Langlaufende Daemon-Skills, überwacht vom Daemon-Subsystem.

### Daemons auflisten
```http
GET /api/daemons
```

### Daemon-Liste aktualisieren
```http
POST /api/daemons/refresh
```

Scannt Skills von der Festplatte neu und gleicht laufende Daemons ab.

### Daemon-Status / Aktionen
```http
GET /api/daemons/{id}
POST /api/daemons/{id}/start
POST /api/daemons/{id}/stop
POST /api/daemons/{id}/reenable
```

---

## Agent Questions API

Interaktive Fragen in der Web-UI, während der Agent auf Benutzereingaben wartet.

### Frage-Status
```http
GET /api/agent/question-status?session=default
```

Gibt `{"status":"none"}` oder `{"status":"pending","question":{...}}` zurück.

### Antwort senden
```http
POST /api/agent/question-response
Content-Type: application/json

{
  "session_id": "default",
  "selected_value": "option_a",
  "free_text": ""
}
```

---

## People API

Knowledge-Graph- und Kontakt-Helfer für personenbezogene Ansichten.

### Personensuche
```http
GET /api/people/lookup?q=name&mode=fts
```

Durchsucht den Knowledge Graph nach Personen-Nodes und zugehörigen Kanten.

### KG-Personen
```http
GET /api/people/kg-persons?limit=100
```

Gibt Personen-Nodes aus dem Knowledge Graph zurück.

### Anstehende Geburtstage
```http
GET /api/people/upcoming?days=30
```

Gibt anstehende Geburtstage aus der Kontakt-Datenbank zurück.

---

## Appointments API

### Termine auflisten / erstellen
```http
GET /api/appointments?q=search&status=scheduled
POST /api/appointments
```

### Termin verwalten
```http
GET /api/appointments/{id}
PUT /api/appointments/{id}
DELETE /api/appointments/{id}
```

---

## Todos API

### Todos auflisten / erstellen
```http
GET /api/todos?q=search&status=open
POST /api/todos
```

### Todo verwalten
```http
GET /api/todos/{id}
PUT /api/todos/{id}
DELETE /api/todos/{id}
POST /api/todos/{id}/complete
```

### Todo-Items
```http
POST /api/todos/{id}/items
PUT /api/todos/{id}/items/{item_id}
DELETE /api/todos/{id}/items/{item_id}
POST /api/todos/{id}/items/reorder
```

---

## Preferences API

Session-UI-Einstellungen, die das Agent-Verhalten beeinflussen.

### Einstellungen abrufen
```http
GET /api/preferences
```

**Antwort:**
```json
{
  "speaker_mode": false
}
```

### Einstellungen aktualisieren
```http
POST /api/preferences
Content-Type: application/json

{
  "speaker_mode": true
}
```

---

## Space Agent API

Space-Agent-Sidecar-Integration für externe Messaging-Bridges.

### Status
```http
GET /api/space-agent/status
```

### Sidecar neu erstellen
```http
POST /api/space-agent/recreate
```

### Nachricht senden
```http
POST /api/space-agent/send
```

### Bridge-Nachrichten
```http
POST /api/space-agent/bridge/messages
```

Eingehender Bridge-Endpunkt mit eigener Bearer-Token-Authentifizierung.

---

## Warnings API

Laufzeit-Gesundheitswarnungen für die Web-UI.

### Warnungen auflisten
```http
GET /api/warnings
```

Gibt alle Warnungen sowie `total` und `unacknowledged` zurück.

### Warnungen bestätigen
```http
POST /api/warnings/acknowledge
Content-Type: application/json

{"id": "warning_id"}
```

Oder alle bestätigen:

```json
{"all": true}
```

---

## 3D Printer API

### Verbindung testen
```http
GET /api/3d-printers/test
```

### Kamera-Snapshot / Stream
```http
GET /api/3d-printers/{printer_id}/camera/snapshot
GET /api/3d-printers/{printer_id}/camera/stream
```

**Test-Antwort-Beispiel:**
```json
{
  "printers": [
    {
      "name": "elegoo",
      "type": "elegoo_centauri_carbon",
      "status": "printing",
      "progress": 42.5
    }
  ]
}
```

---

## AgentMail API

### Status abrufen
```http
GET /api/agentmail/status
```

**Antwort:**
```json
{
  "enabled": true,
  "inbox_id": "inbox_123",
  "address": "aurago@agentmail.io",
  "unread_count": 3
}
```

### Verbindung testen
```http
POST /api/agentmail/test
```

**Antwort:**
```json
{
  "success": true,
  "message": "Connection successful"
}
```

---

## SSE Events

AuraGo unterstützt Server-Sent Events (SSE) für Echtzeit-Updates.

### Event-Stream verbinden
```http
GET /events
Accept: text/event-stream
```

### Event-Typen

| Event | Beschreibung |
|-------|--------------|
| `system_metrics` | System-Metriken (CPU, RAM, etc.) |
| `container_update` | Container-Status-Änderungen |
| `personality_update` | Persönlichkeits-Status |
| `tsnet_status` | Tailscale tsnet Status |

**Beispiel-Stream:**
```
event: system_metrics
data: {"cpu": 15.2, "memory": 45.8, "disk": 72.1}

event: container_update
data: [{"id": "abc", "state": "running"}]
```

---

## Fehlerbehandlung

Die API verwendet standard HTTP-Statuscodes:

| Code | Bedeutung |
|------|-----------|
| 200 | OK |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

**Fehler-Antwort:**
```json
{
  "error": "Beschreibung des Fehlers"
}
```

---

## Weiterführende Links

- [Chat-Commands](20-chat-commands.md) – Alternative API via Chat
- [Mission Control](11-missions.md) – Automatisierung
- [Sicherheit](14-sicherheit.md) – Authentifizierung & Vault
