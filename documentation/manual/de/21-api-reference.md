# REST API Referenz

AuraGo bietet eine umfassende REST API für den programmatischen Zugriff auf alle Funktionen. Die API folgt den REST-Prinzipien und verwendet JSON für die Datenübertragung.

> 📅 **Stand:** März 2026  
> 🔌 **Basis-URL:** `http://localhost:8080` (Standard)

---

## Inhaltsverzeichnis

1. [Authentifizierung](#authentifizierung)
2. [Chat API](#chat-api)
3. [Memory API](#memory-api)
4. [Dashboard API](#dashboard-api)
5. [Config API](#config-api)
6. [Vault API](#vault-api)
7. [Geräte API](#geräte-api)
8. [Mission Control API](#mission-control-api)
9. [Container API](#container-api)
10. [Skills API](#skills-api)
11. [Webhook API](#webhook-api)
12. [SSE Events](#sse-events)

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
  "configured": true,
  "totp_enabled": false
}
```

### Login
```http
POST /auth/login
Content-Type: application/x-www-form-urlencoded

password=geheim&totp_code=123456
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
    "port": 8080
  }
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

## Skills API

### Skills auflisten
```http
GET /api/skills
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
PUT /api/cron/{id}
DELETE /api/cron/{id}
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

### Backup erstellen
```http
POST /api/backup/create
```

### Backup importieren
```http
POST /api/backup/import
Content-Type: multipart/form-data
```

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

### Managed Ollama neu erstellen
```http
POST /api/ollama/managed/recreate
```

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

### Setup speichern
```http
POST /api/setup
Content-Type: application/json

{
  "llm_provider": "openrouter",
  "api_key": "sk-or-..."
}
```

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
