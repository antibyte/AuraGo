# LDAP Integration Plan für AuraGo

> **Status:** Planung  
> **Ziel:** Vollständige LDAP/Active-Directory-Anbindung für den Agenten zur Verwaltung und Abfrage von Benutzern, Gruppen und Organisationseinheiten.

---

## 1. Ziel & Scope

Der Agent soll über ein natives Tool mit LDAP-Servern (OpenLDAP, Active Directory, etc.) interagieren können:

- **Lesende Operationen:** Suchen, Benutzer/Gruppen auflisten, einzelne Einträge abfragen, Authentifizierung testen
- **Schreibende Operationen:** Benutzer/Gruppen anlegen, ändern, löschen (durch `readonly`-Flag absicherbar)
- **UI-Integration:** Konfigurationsseite erweitern mit "Test Connection"-Button
- **Security:** Vault-geschützte Credentials, ReadOnly-Modus, Sensitive-Data-Scrubbing

---

## 2. Technologie

- **Go-Library:** `github.com/go-ldap/ldap/v3` (De-facto-Standard, aktiv maintained)
- **Pattern:** Anlehnung an bestehende Integrationen (FritzBox, MeshCentral, Docker)

---

## 3. Konfiguration (`config.yaml`)

```yaml
ldap:
  enabled: false
  readonly: true
  host: "ldap.example.com"
  port: 636
  use_tls: true
  insecure_skip_verify: false
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=service,dc=example,dc=com"
  bind_password: ""          # Wird aus Vault geladen (Key: ldap_bind_password)
  user_search_base: ""       # Optional, default: base_dn
  group_search_base: ""      # Optional, default: base_dn
```

### 3.1 Config-Typen (`internal/config/config_types.go`)

```go
LDAP struct {
    Enabled            bool   `yaml:"enabled"`
    ReadOnly           bool   `yaml:"readonly"`
    Host               string `yaml:"host"`
    Port               int    `yaml:"port"`
    UseTLS             bool   `yaml:"use_tls"`
    InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
    BaseDN             string `yaml:"base_dn"`
    BindDN             string `yaml:"bind_dn"`
    BindPassword       string `yaml:"-" vault:"ldap_bind_password"`
    UserSearchBase     string `yaml:"user_search_base"`
    GroupSearchBase    string `yaml:"group_search_base"`
} `yaml:"ldap"`
```

### 3.2 Config-Defaults (`internal/config/config.go`)

```go
cfg.LDAP.Port = 389
cfg.LDAP.UseTLS = true
```

---

## 4. Architektur & Dateien

### 4.1 LDAP Client Package — `internal/ldap/client.go`

Reines Go-Package für die LDAP-Kommunikation (analog `internal/fritzbox/`, `internal/meshcentral/`):

```go
package ldap

import "github.com/go-ldap/ldap/v3"

type Config struct {
    Host               string
    Port               int
    UseTLS             bool
    InsecureSkipVerify bool
    BaseDN             string
    BindDN             string
    BindPassword       string
}

type Client struct {
    cfg  Config
    conn *ldap.Conn
}

func NewClient(cfg Config) *Client
func (c *Client) Connect() error
func (c *Client) Close() error
func (c *Client) Search(filter, baseDN string, attrs []string) ([]*ldap.Entry, error)
func (c *Client) Authenticate(userDN, password string) error
func (c *Client) Add(req *ldap.AddRequest) error
func (c *Client) Modify(req *ldap.ModifyRequest) error
func (c *Client) Delete(dn string) error
```

**Rationale:** Trennung von Geschäftslogik (Client) und Agent-Tool-Layer (JSON-Wrapper).

---

### 4.2 Tool-Implementierung — `internal/tools/ldap.go`

Exportierte Funktionen, die den Client nutzen und **JSON-Strings** zurückgeben:

```go
package tools

type LDAPConfig struct {
    Host, BindDN, BindPassword, BaseDN string
    Port                               int
    UseTLS, InsecureSkipVerify         bool
}

func LDAPSearch(cfg LDAPConfig, filter, baseDN string, attrs []string) string
func LDAPGetUser(cfg LDAPConfig, username, userSearchBase string) string
func LDAPGetGroup(cfg LDAPConfig, groupname, groupSearchBase string) string
func LDAPListUsers(cfg LDAPConfig, userSearchBase string) string
func LDAPListGroups(cfg LDAPConfig, groupSearchBase string) string
func LDAPAuthenticate(cfg LDAPConfig, userDN, password string) string
func LDAPAddUser(cfg LDAPConfig, username, cn, ou, password string, attrs map[string][]string) string
func LDAPUpdateUser(cfg LDAPConfig, dn string, changes map[string][]string) string
func LDAPDeleteUser(cfg LDAPConfig, dn string) string
func LDAPAddGroup(cfg LDAPConfig, groupname, ou string, members []string, attrs map[string][]string) string
func LDAPUpdateGroup(cfg LDAPConfig, dn string, changes map[string][]string) string
func LDAPDeleteGroup(cfg LDAPConfig, dn string) string
```

**Security:**
- `security.RegisterSensitive(cfg.BindPassword)` vor jeder Operation
- `security.Scrub()` auf Fehlermeldungen, die Credentials enthalten könnten

---

### 4.3 Feature Flags — `internal/agent/native_tools.go`

```go
type ToolFeatureFlags struct {
    // ... existing flags ...
    LDAPEnabled bool
}
```

### 4.4 Registry-Mapping — `internal/agent/native_tools_registry.go`

In `allBuiltinToolFeatureFlags()`:
```go
LDAPEnabled: cfg.LDAP.Enabled,
```

In `ToolNamesFromConfig()` / `ToolSummariesFromConfig()`:
```go
if cfg.LDAP.Enabled {
    names = append(names, "ldap")
    summaries = append(summaries, "LDAP directory operations (search users/groups, authenticate, manage entries)")
}
```

---

### 4.5 Tool Schema — `internal/agent/native_tools_integrations.go`

```go
if ff.LDAPEnabled {
    tools = append(tools, tool("ldap",
        "Perform LDAP directory operations such as searching users/groups, authenticating accounts, and managing directory entries.",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type":        "string",
                "description": "The LDAP operation to perform",
                "enum": []string{
                    "search", "get_user", "get_group", "list_users", "list_groups",
                    "authenticate", "add_user", "update_user", "delete_user",
                    "add_group", "update_group", "delete_group",
                },
            },
            "filter":     prop("string", "LDAP search filter (e.g., (objectClass=person)). Used for search operation."),
            "dn":         prop("string", "Distinguished Name of the entry. Used for get/update/delete operations."),
            "username":   prop("string", "Username or sAMAccountName. Used for user operations."),
            "groupname":  prop("string", "Group name or cn. Used for group operations."),
            "ou":         prop("string", "Organizational unit / container to place the entry under."),
            "attributes": prop("array",  "List of LDAP attributes to return or set."),
            "password":   prop("string", "Password for authentication or new user creation."),
            "changes":    prop("object", "Map of attribute changes for update operations."),
        }, "operation"),
    ))
}
```

---

### 4.6 Argument Decoder — `internal/agent/tool_args_ldap.go`

Neue Datei (analog `tool_args_proxmox.go` o.ä.):

```go
package agent

type ldapArgs struct {
    Operation  string
    Filter     string
    DN         string
    Username   string
    Groupname  string
    OU         string
    Attributes []string
    Password   string
    Changes    map[string][]string
}

func decodeLDAPArgs(tc ToolCall) ldapArgs {
    return ldapArgs{
        Operation:  firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
        Filter:     toolArgString(tc.Params, "filter"),
        DN:         toolArgString(tc.Params, "dn"),
        Username:   toolArgString(tc.Params, "username"),
        Groupname:  toolArgString(tc.Params, "groupname"),
        OU:         toolArgString(tc.Params, "ou"),
        Attributes: toolArgStringSlice(tc.Params, "attributes"),
        Password:   toolArgString(tc.Params, "password"),
        Changes:    toolArgMapStringSlice(tc.Params, "changes"),
    }
}
```

> **Hinweis:** Falls `toolArgMapStringSlice` nicht existiert, als Helper hinzufügen.

---

### 4.7 Dispatch-Router — `internal/agent/dispatch_ldap.go`

Neue Datei, analog `agent_dispatch_fritzbox.go`:

```go
package agent

func handleLDAPToolCall(tc ToolCall, cfg *config.Config) string {
    if !cfg.LDAP.Enabled {
        return `{"status":"error","message":"LDAP integration is not enabled."}`
    }

    req := decodeLDAPArgs(tc)
    ldapCfg := tools.LDAPConfig{
        Host:               cfg.LDAP.Host,
        Port:               cfg.LDAP.Port,
        UseTLS:             cfg.LDAP.UseTLS,
        InsecureSkipVerify: cfg.LDAP.InsecureSkipVerify,
        BaseDN:             cfg.LDAP.BaseDN,
        BindDN:             cfg.LDAP.BindDN,
        BindPassword:       cfg.LDAP.BindPassword,
    }

    // ReadOnly-Gating
    if cfg.LDAP.ReadOnly {
        switch req.Operation {
        case "add_user", "update_user", "delete_user",
             "add_group", "update_group", "delete_group":
            return `{"status":"error","message":"LDAP is in read-only mode."}`
        }
    }

    switch req.Operation {
    case "search":
        return tools.LDAPSearch(ldapCfg, req.Filter, cfg.LDAP.BaseDN, req.Attributes)
    case "get_user":
        return tools.LDAPGetUser(ldapCfg, req.Username, cfg.LDAP.UserSearchBase)
    case "get_group":
        return tools.LDAPGetGroup(ldapCfg, req.Groupname, cfg.LDAP.GroupSearchBase)
    case "list_users":
        return tools.LDAPListUsers(ldapCfg, cfg.LDAP.UserSearchBase)
    case "list_groups":
        return tools.LDAPListGroups(ldapCfg, cfg.LDAP.GroupSearchBase)
    case "authenticate":
        return tools.LDAPAuthenticate(ldapCfg, req.DN, req.Password)
    case "add_user":
        return tools.LDAPAddUser(ldapCfg, req.Username, req.Username, req.OU, req.Password, req.Changes)
    case "update_user":
        return tools.LDAPUpdateUser(ldapCfg, req.DN, req.Changes)
    case "delete_user":
        return tools.LDAPDeleteUser(ldapCfg, req.DN)
    case "add_group":
        return tools.LDAPAddGroup(ldapCfg, req.Groupname, req.OU, req.Attributes, req.Changes)
    case "update_group":
        return tools.LDAPUpdateGroup(ldapCfg, req.DN, req.Changes)
    case "delete_group":
        return tools.LDAPDeleteGroup(ldapCfg, req.DN)
    default:
        return `{"status":"error","message":"Unknown LDAP operation: ` + req.Operation + `"}`
    }
}
```

**Integration in `DispatchToolCall` (`agent_parse.go` oder `agent_loop_tools.go`):**
```go
case "ldap":
    return handleLDAPToolCall(tc, cfg)
```

---

### 4.8 Tool Manual — `prompts/tools_manuals/ldap.md`

RAG-indexierte Dokumentation für den Agenten:

```markdown
# LDAP Tool

Allows the agent to query and manage LDAP directories (OpenLDAP, Active Directory).

## Prerequisites
- `ldap.enabled: true` in `config.yaml`
- Valid `bind_dn` and `bind_password` (stored in vault)

## Operations

### search — Search directory entries
```json
{"action": "ldap", "operation": "search", "filter": "(objectClass=person)", "attributes": ["cn","mail"]}
```

### get_user — Get a specific user
```json
{"action": "ldap", "operation": "get_user", "username": "john.doe"}
```

### authenticate — Verify user credentials
```json
{"action": "ldap", "operation": "authenticate", "dn": "cn=john.doe,ou=users,dc=example,dc=com", "password": "secret"}
```

### add_user — Create a new user (requires read-only = false)
```json
{"action": "ldap", "operation": "add_user", "username": "jane.doe", "ou": "ou=users,dc=example,dc=com", "password": "changeme"}
```
```

---

### 4.9 Web UI & Translations

**Config-Seite (`ui/config.html` oder zugehöriges JS):**
- Neuer Abschnitt "LDAP Directory"
- Felder: Enabled (Toggle), ReadOnly (Toggle), Host, Port (Dropdown 389/636/Custom), UseTLS (Toggle), InsecureSkipVerify (Toggle), BaseDN, BindDN, BindPassword, UserSearchBase, GroupSearchBase
- **"Test Connection"-Button** (ruft intern `ldap/authenticate` mit BindDN auf)

**Translations (`ui/lang/*.json`):**
Neue Keys für alle 15 Sprachen (cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh):
- `ldap.title`, `ldap.enabled`, `ldap.readonly`, `ldap.host`, `ldap.port`, `ldap.use_tls`, `ldap.insecure`, `ldap.base_dn`, `ldap.bind_dn`, `ldap.bind_password`, `ldap.user_search_base`, `ldap.group_search_base`, `ldap.test_connection`, `ldap.test_success`, `ldap.test_failed`

---

### 4.10 Security Checklist

| Maßnahme | Umsetzung |
|----------|-----------|
| Vault-Speicherung | `bind_password` über `yaml:"-" vault:"ldap_bind_password"` |
| Sensitive Scrubbing | `security.RegisterSensitive(cfg.BindPassword)` in `internal/tools/ldap.go` |
| ReadOnly-Modus | Alle schreibenden Operationen (`add_*`, `update_*`, `delete_*`) blockieren wenn `cfg.LDAP.ReadOnly == true` |
| TLS-Verschlüsselung | `UseTLS` standardmäßig `true`, Port-Default 636 |
| Insecure-Flag | Explizit `insecure_skip_verify` nötig, um Zertifikatsfehler zu ignorieren |

---

## 5. Implementierungs-Reihenfolge

1. **Config** — `config_types.go` + `config.go` (Defaults)
2. **Go Dependency** — `go get github.com/go-ldap/ldap/v3`
3. **Client Package** — `internal/ldap/client.go` (+ Verbindung, Search, Auth)
4. **Tool Layer** — `internal/tools/ldap.go` (JSON-Wrapper)
5. **Agent Integration** — Feature Flags, Schema, Args-Decoder, Dispatch
6. **Dokumentation** — `prompts/tools_manuals/ldap.md`
7. **Web UI** — Formularfelder + Test-Connection-Button
8. **Translations** — Alle 15 Sprachen
9. **Tests** — Client-Unit-Tests, Dispatch-Tests
10. **E2E-Validierung** — Mit Test-LDAP (z.B. `osixia/openldap` Docker-Image)

---

## 6. Test-Strategie

| Test | Datei | Beschreibung |
|------|-------|--------------|
| Unit: Client | `internal/ldap/client_test.go` | Mocked LDAP-Verbindung (via `ldap.NewConn` mit `net.Pipe`) |
| Unit: Tools | `internal/tools/ldap_test.go` | JSON-Output-Validierung mit gemocktem Client |
| Unit: Dispatch | `internal/agent/dispatch_ldap_test.go` | ReadOnly-Gating, unbekannte Operationen, disabled-Check |
| Integration | Docker-Test | `osixia/openldap` Container hochfahren, echte Suche/Auth testen |

---

## 7. Offene Punkte / Entscheidungen

| Punkt | Empfehlung |
|-------|------------|
| Connection Pooling? | Erste Version: Connect-on-Dispatch (einfach, stateless). Bei Performance-Problemen später Pooling in `internal/ldap/` ergänzen. |
| Active Directory-spezifische Attribute? | Generisches Schema (`username` → `(sAMAccountName=` oder `(uid=`). AD-Erweiterungen (z.B. `userAccountControl`) später via `attributes`/`changes` frei definierbar. |
| Gruppen-Mitgliederverwaltung? | In `LDAPUpdateGroup` über `changes` (z.B. `member: [add/del]`) abbildbar. Erste Version: Einzelne `member` Modifikationen. |
