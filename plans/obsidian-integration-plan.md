# Plan: Obsidian Integration via Local REST API

## Übersicht

Integration von [Obsidian](https://obsidian.md/) als Wissensmanagement-Tool über das Community-Plugin **[Local REST API](https://github.com/coddingtonbear/obsidian-local-rest-api)** (v3.6.1+). AuraGo erhält damit Lese-/Schreibzugriff auf den Obsidian-Vault und kann Notizen durchsuchen, erstellen, bearbeiten und die Vault-Struktur navigieren.

## Ziel

Der AuraGo-Agent kann:
- Notizen lesen, erstellen, aktualisieren, löschen (Full CRUD)
- Gezielt Abschnitte (Headings, Block References, Frontmatter) lesen und bearbeiten
- Im Vault suchen (Volltext + Dataview DQL)
- Vault-Struktur (Ordner/Dateien) auflisten
- Periodische Notizen (Daily, Weekly, Monthly, etc.) verwalten
- Tags abrufen
- Obsidian-Befehle ausführen
- Dateien in Obsidian öffnen lassen

## Nicht-Ziel

- Kein Echtzeit-Sync oder bidirektionale Synchronisation
- Keine eigene Obsidian-Plugin-Entwicklung
- Kein Ersetzen des bestehenden Dateisystem-Tools — Obsidian-Tool ist vault-spezifisch
- Keine automatische Vault-Indizierung in den AuraGo-Vektorspeicher (kann als späteres Feature folgen)

---

## Obsidian Local REST API — Referenz

### Verbindungsdaten

| Parameter | Default |
|-----------|---------|
| Host | `127.0.0.1` |
| HTTPS Port | `27124` |
| HTTP Port | `27123` |
| Auth | Bearer Token (API Key aus Plugin-Einstellungen) |
| TLS | Selbstsigniertes Zertifikat (InsecureSSL-Option nötig) |

### API Endpoints

| Endpoint | Methoden | Beschreibung |
|----------|----------|--------------|
| `/` | GET | Serverstatus, Auth-Check (kein Auth nötig) |
| `/vault/` | GET | Dateien im Vault-Root auflisten |
| `/vault/{path}/` | GET | Dateien in Unterverzeichnis auflisten |
| `/vault/{filename}` | GET, POST, PUT, PATCH, DELETE | CRUD auf Dateien im Vault |
| `/active/` | GET, POST, PUT, PATCH, DELETE | Aktive Datei in Obsidian |
| `/periodic/{period}/` | GET, POST, PUT, PATCH, DELETE | Aktuelle periodische Notiz (daily, weekly, monthly, quarterly, yearly) |
| `/periodic/{period}/{year}/{month}/{day}/` | GET, POST, PUT, PATCH, DELETE | Periodische Notiz für bestimmtes Datum |
| `/search/simple/` | POST | Volltext-Suche (Fuzzy, mit Context-Snippets) |
| `/search/` | POST | Strukturierte Suche (Dataview DQL oder JsonLogic) |
| `/commands/` | GET | Verfügbare Obsidian-Commands auflisten |
| `/commands/{commandId}/` | POST | Command ausführen |
| `/tags/` | GET | Alle Tags mit Verwendungszählung |
| `/open/{filename}` | POST | Datei in Obsidian-UI öffnen |

### Sub-Document Targeting

Gezielte Operationen auf Teile einer Notiz via Headers oder URL-Pfad:

- **Target-Types**: `heading`, `block`, `frontmatter`
- **PATCH Operations**: `append`, `prepend`, `replace`
- **Headers**: `Target-Type`, `Target`, `Operation`, `Create-Target-If-Missing`, `Target-Delimiter` (default `::`)
- **URL-Variante**: `/vault/note.md/heading/My%20Section`, `/vault/note.md/frontmatter/status`
- **Document Map**: `Accept: application/vnd.olrapi.document-map+json` → zeigt verfügbare Headings, Blocks, Frontmatter-Fields
- **Metadata**: `Accept: application/vnd.olrapi.note+json` → JSON mit Tags, Frontmatter, Stat

### NoteJson Schema

```json
{
  "content": "string",
  "frontmatter": {},
  "path": "string",
  "tags": ["string"],
  "stat": {
    "ctime": 0,
    "mtime": 0,
    "size": 0
  }
}
```

---

## Implementierungsplan

### Phase 1: Config & Client Package

#### 1.1 Config-Struct

**Datei**: `internal/config/config_types.go`

```go
type ObsidianConfig struct {
    Enabled          bool   `yaml:"enabled"`
    ReadOnly         bool   `yaml:"readonly"`
    AllowDestructive bool   `yaml:"allow_destructive"`
    Host             string `yaml:"host"`              // Default: 127.0.0.1
    Port             int    `yaml:"port"`              // Default: 27124
    UseHTTPS         bool   `yaml:"use_https"`         // Default: true
    InsecureSSL      bool   `yaml:"insecure_ssl"`      // Default: true (selbstsigniertes Zertifikat)
    APIKey           string `yaml:"-" json:"-"`        // Vault-Only: obsidian_api_key
    ConnectTimeout   int    `yaml:"connect_timeout"`   // Default: 10 (Sekunden)
    RequestTimeout   int    `yaml:"request_timeout"`   // Default: 30 (Sekunden)
}
```

**Datei**: `internal/config/config.go` — Defaults setzen:
- `Port: 27124`
- `UseHTTPS: true`
- `InsecureSSL: true`
- `ConnectTimeout: 10`
- `RequestTimeout: 30`

**Datei**: `config_template.yaml` — Template-Abschnitt ergänzen.

**Vault Key**: `obsidian_api_key` — zur Forbidden-Export-Liste hinzufügen.

#### 1.2 Client Package

**Package**: `internal/obsidian/`

| Datei | Inhalt |
|-------|--------|
| `client.go` | HTTP-Client-Init, Base-URL-Builder, Auth-Header, TLS-Config, Ping/Health |
| `types.go` | Go-Structs für API-Responses (NoteJson, SearchResult, Command, Tag, Error, FileList, ServerStatus) |
| `vault.go` | Vault-File-Operationen: Read, Create, Update, Patch, Delete, List |
| `search.go` | Simple-Search, Dataview-DQL-Search |
| `periodic.go` | Periodic-Note-Operationen (Daily, Weekly, etc.) |
| `commands.go` | List-Commands, Execute-Command |
| `tags.go` | List-Tags |

**Client-Design**:
```go
type Client struct {
    httpClient *http.Client
    baseURL    string
    apiKey     string
    logger     *slog.Logger
}

func NewClient(cfg config.ObsidianConfig, vault security.VaultReader, logger *slog.Logger) (*Client, error)
func (c *Client) Ping(ctx context.Context) (*ServerStatus, error)
```

**Sicherheitsaspekte**:
- API Key wird aus Vault gelesen, nie aus Config
- Alle externen Inhalte (Notiz-Inhalte) mit `<external_data>` wrappen wenn sie an den Agent gehen
- InsecureSSL nur für selbstsignierte Zertifikate (Default-Use-Case)
- Timeout-Limits konfigurierbar
- Content von Obsidian ist untrusted — Prompt-Injection-Schutz beim Zurückgeben an den Agent

#### 1.3 Tests

**Datei**: `internal/obsidian/client_test.go`

- Unit-Tests mit httptest.Server für alle Endpoint-Kategorien
- Test für Auth-Header-Setzung
- Test für TLS-InsecureSkipVerify
- Test für Fehlerbehandlung (404, 400, 405, Auth-Fehler)
- Test für Timeout-Verhalten

---

### Phase 2: Tool Definition & Dispatch

#### 2.1 Tool-Schema

**Datei**: `internal/agent/native_tools_integrations.go`

Feature-Flag: `ff.ObsidianEnabled`

```go
tool("obsidian",
    "Interact with an Obsidian vault via the Local REST API plugin. "+
        "Read, create, search, and manage notes in Obsidian.",
    schema(map[string]interface{}{
        "operation": map[string]interface{}{
            "type":        "string",
            "description": "Operation to perform",
            "enum": []string{
                "health",
                "list_files",
                "read_note",
                "create_note",
                "update_note",
                "patch_note",
                "delete_note",
                "search",
                "search_dataview",
                "list_tags",
                "daily_note",
                "periodic_note",
                "list_commands",
                "execute_command",
                "open_in_obsidian",
                "document_map",
            },
        },
        "path":        prop("string", "File path relative to vault root (e.g. 'Notes/myfile.md')"),
        "content":     prop("string", "Content for create/update/patch operations"),
        "query":       prop("string", "Search query (for search/search_dataview)"),
        "target_type": map[string]interface{}{
            "type":        "string",
            "description": "Sub-document target type for read/patch",
            "enum":        []string{"heading", "block", "frontmatter"},
        },
        "target":      prop("string", "Target name (heading name, block ID, frontmatter field)"),
        "patch_op":    map[string]interface{}{
            "type":        "string",
            "description": "Patch operation type",
            "enum":        []string{"append", "prepend", "replace"},
        },
        "period":      map[string]interface{}{
            "type":        "string",
            "description": "Period for periodic notes",
            "enum":        []string{"daily", "weekly", "monthly", "quarterly", "yearly"},
        },
        "command_id":  prop("string", "Command ID to execute (from list_commands)"),
        "directory":   prop("string", "Directory path for list_files (empty = vault root)"),
        "context_length": map[string]interface{}{
            "type":        "integer",
            "description": "Context length for search results (default: 100)",
        },
    }, "operation"),
)
```

#### 2.2 Tool-Dispatch

**Datei**: `internal/tools/obsidian.go`

```go
func DispatchObsidianTool(operation string, params map[string]string,
    cfg *config.Config, vault *security.Vault, logger *slog.Logger) string
```

**Operationen und Permission-Gates:**

| Operation | Mindest-Berechtigung |
|-----------|---------------------|
| `health`, `list_files`, `read_note`, `search`, `search_dataview`, `list_tags`, `list_commands`, `document_map` | Enabled |
| `daily_note` (read), `periodic_note` (read) | Enabled |
| `create_note`, `update_note`, `patch_note`, `open_in_obsidian`, `execute_command` | Enabled + ReadOnly off |
| `daily_note` (write), `periodic_note` (write) | Enabled + ReadOnly off |
| `delete_note` | Enabled + ReadOnly off + AllowDestructive |

**Prompt-Injection-Schutz**: Alle Notiz-Inhalte die an den Agent zurückgegeben werden, müssen in `<external_data>` gewrappt werden.

**Content-Größenlimit**: Notizen > 50 KB werden gekürzt mit Hinweis auf Größe.

#### 2.3 Feature-Flag

**Datei**: `internal/agent/native_tools.go` (oder wo FeatureFlags definiert sind)

Feld `ObsidianEnabled bool` hinzufügen, gespeist aus `cfg.Obsidian.Enabled`.

---

### Phase 3: Server Handler & UI

#### 3.1 API Handlers

**Datei**: `internal/server/obsidian_handlers.go`

| Route | Methode | Beschreibung |
|-------|---------|--------------|
| `/api/obsidian/status` | GET | Verbindungsstatus + Server-Info |
| `/api/obsidian/test` | POST | Test-Verbindung (für Config-UI) |

`registerObsidianHandlers(mux *http.ServeMux, s *Server)` — Registrierung in Server-Init.

#### 3.2 Config-UI

Obsidian-Sektion in der Config-Seite (`ui/config.html` oder zugehöriges JS-Modul):

| Feld | Typ | Default | Beschreibung |
|------|-----|---------|--------------|
| Enabled | Toggle | off | Integration aktivieren |
| Read-Only | Toggle | off | Nur Lesezugriff |
| Allow Destructive | Toggle | off | Löschen erlauben |
| Host | Text | 127.0.0.1 | Obsidian-Host |
| Port | Number | 27124 | API-Port |
| Use HTTPS | Toggle | on | HTTPS verwenden |
| Insecure SSL | Toggle | on | Selbstsigniertes Zertifikat akzeptieren |
| API Key | Password | — | Aus Vault (obsidian_api_key) |
| Connect Timeout | Number | 10 | Verbindungs-Timeout (Sek.) |
| Request Timeout | Number | 30 | Request-Timeout (Sek.) |
| **Test Connection** | Button | — | Verbindung testen |

#### 3.3 Dashboard

Falls Obsidian aktiviert ist, ein kleines Status-Widget auf dem Dashboard:
- Online/Offline-Indikator
- Vault-Name (aus Server-Info)
- Obsidian-Version

---

### Phase 4: Dokumentation & Übersetzungen

#### 4.1 Tool-Manual

**Datei**: `prompts/tools_manuals/obsidian.md`

Vollständige Dokumentation aller Operationen mit Beispielen, Parametern und Permission-Matrix.

#### 4.2 Übersetzungen

**Alle 15 Sprachen**: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh

Translation-Keys (Beispiele):
```
config.obsidian.section_title
config.obsidian.enabled_label
config.obsidian.readonly_label
config.obsidian.readonly_hint
config.obsidian.allow_destructive_label
config.obsidian.allow_destructive_hint
config.obsidian.host_label
config.obsidian.port_label
config.obsidian.use_https_label
config.obsidian.insecure_ssl_label
config.obsidian.insecure_ssl_hint
config.obsidian.api_key_label
config.obsidian.api_key_hint
config.obsidian.connect_timeout_label
config.obsidian.request_timeout_label
config.obsidian.test_connection_label
config.obsidian.test_success
config.obsidian.test_failed
help.obsidian.setup_instructions
dashboard.obsidian.title
dashboard.obsidian.status_online
dashboard.obsidian.status_offline
```

#### 4.3 Config-Template

`config_template.yaml` — Obsidian-Abschnitt mit kommentierten Defaults.

#### 4.4 Vault-Forbidden-Export

Obsidian-API-Key (`obsidian_api_key`) zur Liste der verbotenen Python-Tool-Exporte hinzufügen.

#### 4.5 Hilfetext

Web-UI Help-Text für Obsidian-Integration aktualisieren.

---

## Dateien-Übersicht (Neu/Geändert)

### Neue Dateien

| Datei | Beschreibung |
|-------|--------------|
| `internal/obsidian/client.go` | HTTP-Client, Auth, TLS, Ping |
| `internal/obsidian/types.go` | Go-Structs für API-Responses |
| `internal/obsidian/vault.go` | Vault-File-CRUD |
| `internal/obsidian/search.go` | Such-Operationen |
| `internal/obsidian/periodic.go` | Periodische Notizen |
| `internal/obsidian/commands.go` | Command-Listing & Execution |
| `internal/obsidian/tags.go` | Tag-Listing |
| `internal/obsidian/client_test.go` | Unit-Tests |
| `internal/tools/obsidian.go` | Tool-Dispatch |
| `internal/tools/obsidian_test.go` | Dispatch-Tests |
| `internal/server/obsidian_handlers.go` | API-Handler für Status/Test |
| `prompts/tools_manuals/obsidian.md` | Agent-Tool-Manual |

### Geänderte Dateien

| Datei | Änderung |
|-------|----------|
| `internal/config/config_types.go` | `ObsidianConfig` struct hinzufügen |
| `internal/config/config.go` | Defaults, Config-Feld in Haupt-Config |
| `internal/agent/native_tools_integrations.go` | Tool-Schema registrieren |
| `internal/agent/native_tools.go` | Feature-Flag `ObsidianEnabled` |
| `internal/server/server.go` | Handler-Registrierung |
| `config_template.yaml` | Obsidian-Config-Template |
| `ui/lang/config/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | Übersetzungen |
| UI-Config-Dateien | Config-Sektion für Obsidian |
| Security/Vault-Forbidden-Liste | `obsidian_api_key` hinzufügen |

---

## Sicherheitsaspekte

1. **API Key im Vault** — Nie in config.yaml, nie im Code
2. **Prompt-Injection**: Alle Vault-Inhalte sind untrusted → `<external_data>` Wrapper
3. **Content-Size-Limit**: Große Notizen kürzen (>50 KB) um Context-Überflutung zu verhindern
4. **ReadOnly / AllowDestructive**: Granulare Permission-Gates
5. **TLS**: InsecureSSL nur für selbstsignierte Zertifikate, Default-Use-Case
6. **Timeout**: Konfigurierbare Timeouts gegen Hänger
7. **Vault-Export-Verbot**: API Key darf nicht an Python-Tools exportiert werden
8. **SSRF-Schutz**: Host/Port sind konfiguriert und gehen nicht an den Agent — keine dynamischen URLs

## Risiken & Mitigationen

| Risiko | Mitigation |
|--------|-----------|
| Obsidian nicht erreichbar (Desktop-App) | Health-Check + klare Fehlermeldung, Test-Connection-Button |
| Selbstsigniertes Zertifikat | InsecureSSL Default true, Doku im Setup |
| Große Vaults → große Responses | Content-Limit, Pagination bei Directory-Listing |
| Plugin nicht installiert | Health-Endpoint gibt klare Fehlermeldung |
| Prompt Injection über Notiz-Inhalte | `<external_data>` Wrapper für alle Inhalte |
| Netzwerklatenz (Remote-Obsidian) | Konfigurierbare Timeouts |

## Reihenfolge

1. **Phase 1**: Config + Client Package (Foundation) — testbar isoliert
2. **Phase 2**: Tool Definition + Dispatch — Agent kann Obsidian nutzen
3. **Phase 3**: Server Handler + Config UI — User kann konfigurieren
4. **Phase 4**: Doku, Übersetzungen, Hilfe — Polishing & Vollständigkeit

Jede Phase ist einzeln committbar und testbar.
