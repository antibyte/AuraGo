# Tool-Kontext-Optimierungsplan

> **Status:** Entwurf  
> **Ziel:** Reduktion der Tool-Schema-Größe im LLM-Kontext um 60-80% ohne signifikante Beeinträchtigung der Reaktionszeit  
> **Datum:** 2026-03-16

---

## 1. Ausgangssituation

### Problemstellung

Bei aktiver Nutzung vieler Tools (70-80 Stück) nimmt die Tool-Übersicht 70-80% der verfügbaren Kontextzeichen ein:

| Metrik | Aktueller Wert |
|--------|----------------|
| Durchschnittliche Tokenanzahl pro Tool | ~250-400 Tokens |
| Bei 80 aktiven Tools | ~20.000-32.000 Tokens |
| Anteil an typischem 4K-Prompt | ~500-800% |
| Anteil an 8K-Prompt | ~250-400% |

### Analyse der Tool-Implementierung

Die aktuelle Implementierung in `internal/agent/native_tools.go` erzeugt für jedes aktivierte Tool ein vollständiges OpenAI-Function-Schema:

```go
// Beispiel: Einzelnes Tool-Schema (ca. 400 Tokens)
{
  "type": "function",
  "function": {
    "name": "docker",
    "description": "Manage Docker containers, images, networks, and volumes...",
    "parameters": {
      "type": "object",
      "properties": {
        "operation": {"type": "string", "enum": [...]},
        "container_id": {"type": "string"},
        // ... weitere Properties
      }
    }
  }
}
```

---

## 2. Lösungsstrategien

### Übersicht der Ansätze

| Phase | Strategie | Token-Ersparnis | Komplexität | Risiko |
|-------|-----------|-----------------|-------------|--------|
| 1 | **Tool Grouping** | ~60% | Niedrig | Gering |
| 2 | **Schema Compression** | ~30% | Minimal | Keines |
| 3 | **Tier-basierte Tool-Verfügbarkeit** | ~40% | Mittel | Gering |
| 4 | **Lazy Tool Loading** | ~40% | Hoch | Mittel |
| 5 | **Hierarchical Dispatch** | ~50% | Hoch | Mittel |

**Empfohlene Kombination (Phase 1+2+3):** ~70-80% Reduktion bei minimalem Risiko

---

## 3. Phase 1: Tool Grouping (Priorität: Hoch)

### Konzept

Statt einzelner Tool-Definitionen werden thematisch verwandte Tools zu Gruppen zusammengefasst. Die Tool-Auswahl erfolgt über einen `group`-Parameter.

### Architektur

```go
// internal/agent/tool_groups.go
package agent

type ToolGroup struct {
    Name        string
    Description string
    Operations  map[string]ToolOperation
}

type ToolOperation struct {
    Description string
    Parameters  map[string]interface{}
}

var ToolGroups = map[string]ToolGroup{
    "system": {
        Name:        "system",
        Description: "File system, shell commands, Python execution, system monitoring",
        Operations: map[string]ToolOperation{
            "filesystem_read": {
                Description: "Read file contents",
                Parameters: map[string]interface{}{
                    "file_path": map[string]interface{}{"type": "string"},
                },
            },
            "filesystem_write": {
                Description: "Write content to file",
                Parameters: map[string]interface{}{
                    "file_path": map[string]interface{}{"type": "string"},
                    "content":   map[string]interface{}{"type": "string"},
                },
            },
            "shell_execute": {
                Description: "Execute shell command",
                Parameters: map[string]interface{}{
                    "command": map[string]interface{}{"type": "string"},
                },
            },
            // ... weitere Operationen
        },
    },
    "container": {
        Name:        "container",
        Description: "Docker container and image management",
        Operations: map[string]ToolOperation{
            "docker_list":     {Description: "List containers", Parameters: ...},
            "docker_start":    {Description: "Start container", Parameters: ...},
            "docker_stop":     {Description: "Stop container", Parameters: ...},
            "docker_inspect":  {Description: "Inspect container", Parameters: ...},
        },
    },
    "media": {
        Name:        "media",
        Description: "Image generation, audio processing, media management",
        Operations: map[string]ToolOperation{
            "generate_image":      {Description: "Generate image from prompt", Parameters: ...},
            "transcribe_audio":    {Description: "Transcribe audio to text", Parameters: ...},
            "tts":                 {Description: "Text-to-speech conversion", Parameters: ...},
            "media_registry_add":  {Description: "Add entry to media registry", Parameters: ...},
        },
    },
    "infrastructure": {
        Name:        "infrastructure",
        Description: "VMs, networking, VPN, cloud infrastructure",
        Operations: map[string]ToolOperation{
            "proxmox_list_vms":        {Description: "List Proxmox VMs", Parameters: ...},
            "proxmox_start_vm":        {Description: "Start Proxmox VM", Parameters: ...},
            "tailscale_devices":       {Description: "List Tailscale devices", Parameters: ...},
            "cloudflare_tunnel_start": {Description: "Start Cloudflare tunnel", Parameters: ...},
        },
    },
    "home_automation": {
        Name:        "home_automation",
        Description: "Smart home device control and automation",
        Operations: map[string]ToolOperation{
            "ha_get_states":   {Description: "Get Home Assistant entity states", Parameters: ...},
            "ha_call_service": {Description: "Call Home Assistant service", Parameters: ...},
            "mqtt_publish":    {Description: "Publish MQTT message", Parameters: ...},
            "wake_on_lan":     {Description: "Send Wake-on-LAN packet", Parameters: ...},
        },
    },
    "development": {
        Name:        "development",
        Description: "Web development, deployment, version control",
        Operations: map[string]ToolOperation{
            "homepage_init":    {Description: "Initialize web project", Parameters: ...},
            "homepage_deploy":  {Description: "Deploy website", Parameters: ...},
            "github_repo_list": {Description: "List GitHub repositories", Parameters: ...},
            "netlify_deploy":   {Description: "Deploy to Netlify", Parameters: ...},
        },
    },
    "memory_knowledge": {
        Name:        "memory_knowledge",
        Description: "Memory storage, knowledge management, notes",
        Operations: map[string]ToolOperation{
            "memory_add":         {Description: "Add memory fact", Parameters: ...},
            "memory_query":       {Description: "Query memories", Parameters: ...},
            "knowledge_add_relation": {Description: "Add knowledge graph relation", Parameters: ...},
            "notes_create":       {Description: "Create note", Parameters: ...},
            "journal_add":        {Description: "Add journal entry", Parameters: ...},
        },
    },
    "integrations": {
        Name:        "integrations",
        Description: "External service integrations (AdGuard, Ollama, etc.)",
        Operations: map[string]ToolOperation{
            "adguard_status":  {Description: "Get AdGuard status", Parameters: ...},
            "ollama_list":     {Description: "List Ollama models", Parameters: ...},
            "ansible_run":     {Description: "Run Ansible playbook", Parameters: ...},
        },
    },
}
```

### Schema-Definition

```go
// Ersetzt alle Einzel-Tools durch ein gruppiertes Schema
func buildGroupedToolSchema(groups map[string]ToolGroup) []openai.Tool {
    // Erstelle Enum der verfügbaren Gruppen
    groupNames := make([]string, 0, len(groups))
    groupDescriptions := make([]string, 0, len(groups))
    
    for name, group := range groups {
        groupNames = append(groupNames, name)
        groupDescriptions = append(groupDescriptions, 
            fmt.Sprintf("%s: %s", name, group.Description))
    }
    
    return []openai.Tool{
        {
            Type: "function",
            Function: &openai.FunctionDefinition{
                Name:        "execute_operation",
                Description: "Execute system operations. Available groups:\n" + 
                            strings.Join(groupDescriptions, "\n"),
                Parameters: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "group": map[string]interface{}{
                            "type":        "string",
                            "enum":        groupNames,
                            "description": "Tool group/category",
                        },
                        "operation": map[string]interface{}{
                            "type":        "string",
                            "description": "Operation to execute within the group",
                        },
                        "parameters": map[string]interface{}{
                            "type":        "object",
                            "description": "Operation-specific parameters",
                        },
                    },
                    "required": []string{"group", "operation"},
                },
            },
        },
    }
}
```

### Dispatch-Implementierung

```go
// internal/agent/agent_dispatch.go

func DispatchGroupedTool(ctx context.Context, call ToolCall, cfg *config.Config, 
    logger *slog.Logger, /* ... */) string {
    
    groupName := call.Group
    operation := call.Operation
    params := call.Parameters
    
    // Route zur entsprechenden Handler-Funktion
    switch groupName {
    case "system":
        return dispatchSystemOperation(operation, params, cfg, logger)
    case "container":
        return dispatchContainerOperation(operation, params, cfg, logger)
    case "media":
        return dispatchMediaOperation(operation, params, cfg, logger)
    case "infrastructure":
        return dispatchInfrastructureOperation(operation, params, cfg, logger)
    case "home_automation":
        return dispatchHomeAutomationOperation(operation, params, cfg, logger)
    case "development":
        return dispatchDevelopmentOperation(operation, params, cfg, logger)
    case "memory_knowledge":
        return dispatchMemoryOperation(operation, params, cfg, logger)
    case "integrations":
        return dispatchIntegrationOperation(operation, params, cfg, logger)
    default:
        return fmt.Sprintf("[Error] Unknown tool group: %s", groupName)
    }
}

// Beispiel-Handler für System-Operationen
func dispatchSystemOperation(operation string, params map[string]interface{}, 
    cfg *config.Config, logger *slog.Logger) string {
    
    switch operation {
    case "filesystem_read":
        filePath := getStringParam(params, "file_path")
        return tools.ReadFile(filePath)
        
    case "filesystem_write":
        filePath := getStringParam(params, "file_path")
        content := getStringParam(params, "content")
        return tools.WriteFile(filePath, content)
        
    case "shell_execute":
        if !cfg.Agent.AllowShell {
            return "[Error] Shell execution not allowed"
        }
        command := getStringParam(params, "command")
        return tools.ExecuteShell(command, cfg, logger)
        
    // ... weitere Operationen
    
    default:
        return fmt.Sprintf("[Error] Unknown system operation: %s", operation)
    }
}
```

### Konfiguration

```yaml
# config.yaml
agent:
  tool_grouping:
    enabled: true
    
    # Optional: Gruppen explizit deaktivieren
    disabled_groups:
      # - integrations
      # - development
    
    # Mapping: Welche Tools gehören zu welcher Gruppe
    # (Wenn nicht angegeben, wird Standard-Mapping verwendet)
    custom_mappings:
      custom_tool_name: "system"
```

### Erwartete Einsparung

| Szenario | Vorher | Nachher | Einsparung |
|----------|--------|---------|------------|
| 80 Tools (alle aktiv) | ~28.000 Tokens | ~1.500 Tokens | ~95% |
| 40 Tools (typisch) | ~14.000 Tokens | ~1.500 Tokens | ~90% |
| Minimal (10 Tools) | ~3.500 Tokens | ~1.200 Tokens | ~65% |

---

## 4. Phase 2: Schema Compression (Priorität: Mittel)

### Konzept

JSON Schemas sind sehr verbose. Durch kompakte Notation und Deduplizierung können weitere 30% eingespart werden.

### Implementierung

```go
// internal/agent/schema_compressor.go

// CompactType repräsentiert einen kompakten Typ
const (
    TypeStr     = "s"  // string
    TypeInt     = "i"  // integer
    TypeBool    = "b"  // boolean
    TypeObj     = "o"  // object
    TypeArr     = "a"  // array
    TypeEnum    = "e"  // enum
)

// CompressSchema wandelt ein JSON Schema in kompakte Notation um
func CompressSchema(name, description string, params map[string]interface{}) map[string]interface{} {
    compressed := map[string]interface{}{
        "n": name,                    // name
        "d": truncateDesc(description, 100), // description (gekürzt)
        "p": compressParameters(params), // parameters
    }
    return compressed
}

func compressParameters(params map[string]interface{}) map[string]string {
    result := make(map[string]string)
    
    for key, val := range params {
        switch v := val.(type) {
        case map[string]interface{}:
            paramType := getString(v, "type")
            desc := truncateDesc(getString(v, "description"), 50)
            
            // Kompakte Typ-Notation
            typeCode := TypeStr
            switch paramType {
            case "string":
                if enum, ok := v["enum"].([]string); ok {
                    typeCode = TypeEnum + "[" + strings.Join(enum, ",") + "]"
                }
            case "integer", "number":
                typeCode = TypeInt
            case "boolean":
                typeCode = TypeBool
            case "object":
                typeCode = TypeObj
            case "array":
                typeCode = TypeArr
            }
            
            result[key] = fmt.Sprintf("%s:%s", typeCode, desc)
        }
    }
    
    return result
}
```

### Alternative: Minified JSON

```go
// Remove unnötige Whitespace und lange Schlüssel
func MinifySchema(schema map[string]interface{}) map[string]interface{} {
    minified := map[string]interface{}{
        "type": "function",
        "function": map[string]interface{}{
            "name": schema["name"],
            "desc": truncateString(schema["description"].(string), 120),
            "params": minifyParams(schema["parameters"]),
        },
    }
    return minified
}

func minifyParams(params map[string]interface{}) map[string]interface{} {
    props, _ := params["properties"].(map[string]interface{})
    minProps := make(map[string]interface{})
    
    for key, val := range props {
        if prop, ok := val.(map[string]interface{}); ok {
            // Nur essentielle Felder behalten
            minProp := map[string]interface{}{
                "t": prop["type"],  // type -> t
            }
            
            if desc, ok := prop["description"]; ok {
                minProp["d"] = truncateString(desc.(string), 60)  // description -> d
            }
            
            if enum, ok := prop["enum"]; ok {
                // Bei langen Enums: nur erste 3 + Count
                if arr, ok := enum.([]interface{}); ok && len(arr) > 5 {
                    minProp["e"] = append(arr[:3], fmt.Sprintf("...+%d", len(arr)-3))
                } else {
                    minProp["e"] = enum  // enum -> e
                }
            }
            
            minProps[key] = minProp
        }
    }
    
    return map[string]interface{}{
        "type":       "object",
        "properties": minProps,
    }
}
```

---

## 5. Phase 3: Tier-basierte Tool-Verfügbarkeit (Priorität: Mittel)

### Konzept

Erweiterung der bestehenden Tier-Logik (`DetermineTierAdaptive`) auf Tool-Ebene. Je nach Gesprächskontext werden weniger oder mehr Tool-Gruppen bereitgestellt.

### Implementierung

```go
// internal/agent/tool_tiers.go

type ToolTierConfig struct {
    AlwaysAvailable []string          // Immer verfügbar
    TierGroups      map[string][]string // Gruppen pro Tier
    ContextGroups   map[string][]string // Kontext-spezifische Gruppen
}

var DefaultToolTierConfig = ToolTierConfig{
    // Diese Gruppen sind IMMER verfügbar
    AlwaysAvailable: []string{"system", "memory_knowledge"},
    
    // Tier-basierte Gruppen
    TierGroups: map[string][]string{
        "minimal":  {"system", "memory_knowledge"},
        "compact":  {"system", "memory_knowledge", "media", "home_automation"},
        "full":     {"system", "memory_knowledge", "media", "home_automation", 
                     "container", "infrastructure", "development", "integrations"},
    },
    
    // Kontext-spezifische Aktivierungen
    ContextGroups: map[string][]string{
        "docker_mentioned":     {"container"},
        "github_mentioned":     {"development"},
        "proxmox_mentioned":    {"infrastructure"},
        "ha_mentioned":         {"home_automation"},
        "coding_task":          {"development", "system"},
        "deployment_task":      {"development", "container", "infrastructure"},
    },
}

// GetToolsForContext bestimmt welche Tool-Gruppen aktiv sein sollten
func GetToolsForContext(flags prompts.ContextFlags, userMessage string, 
    recentTools []string) []string {
    
    config := DefaultToolTierConfig
    selected := make(map[string]bool)
    
    // 1. Immer verfügbare Gruppen
    for _, group := range config.AlwaysAvailable {
        selected[group] = true
    }
    
    // 2. Tier-basierte Gruppen
    tier := flags.Tier
    if tier == "" {
        tier = prompts.DetermineTierAdaptive(flags)
    }
    
    for _, group := range config.TierGroups[tier] {
        selected[group] = true
    }
    
    // 3. Kontext-basierte Aktivierung (Keyword-Matching)
    messageLower := strings.ToLower(userMessage)
    for trigger, groups := range config.ContextGroups {
        if keywordMatches(messageLower, trigger) {
            for _, group := range groups {
                selected[group] = true
            }
        }
    }
    
    // 4. Recently used Tools -> deren Gruppen beibehalten
    for _, tool := range recentTools {
        if group := findGroupForTool(tool); group != "" {
            selected[group] = true
        }
    }
    
    // 5. Error State -> alle Gruppen aktivieren
    if flags.IsErrorState {
        for _, groups := range config.TierGroups {
            for _, group := range groups {
                selected[group] = true
            }
        }
    }
    
    return mapToSlice(selected)
}

func keywordMatches(message, trigger string) bool {
    // Einfaches Keyword-Matching (kann durch Embedding-basiertes Matching ersetzt werden)
    keywords := map[string][]string{
        "docker_mentioned":     {"docker", "container", "image", "compose"},
        "github_mentioned":     {"github", "git", "repository", "commit", "pull request"},
        "proxmox_mentioned":    {"proxmox", "vm", "virtual machine", "lxc"},
        "ha_mentioned":         {"home assistant", "smart home", "mqtt", "zigbee"},
        "coding_task":          {"code", "program", "script", "function", "bug"},
        "deployment_task":      {"deploy", "publish", "host", "server"},
    }
    
    if words, ok := keywords[trigger]; ok {
        for _, word := range words {
            if strings.Contains(message, word) {
                return true
            }
        }
    }
    return false
}
```

### Integration in Agent Loop

```go
// In internal/agent/agent_loop.go

// Vor BuildNativeToolSchemas:
activeGroups := GetToolsForContext(flags, lastUserMsg, recentTools)
filteredGroups := filterGroupsByAvailability(ToolGroups, activeGroups, cfg)

// Statt alle Tools:
toolSchemas := buildGroupedToolSchema(filteredGroups)
```

---

## 6. Phase 4 & 5: Lazy Loading & Hierarchical Dispatch

> **Hinweis:** Diese Phasen sind komplexer und sollten erst nach erfolgreicher Implementierung der Phasen 1-3 in Betracht gezogen werden.

### Phase 4: Lazy Tool Loading

Tools werden erst bei Bedarf (basierend auf User-Intent) geladen.

```go
// Konzept: Discovery-Tool
type DiscoveryResponse struct {
    AvailableGroups []GroupInfo `json:"groups"`
}

type GroupInfo struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Operations  []string `json:"operations"`
}

// LLM kann "discover_tools" aufrufen um verfügbare Tools zu sehen
// Danach werden die spezifischen Gruppen aktiviert
```

### Phase 5: Hierarchical Dispatch

Zwei-Ebenen-System: Discovery + Execution.

```go
// Ebene 1: Kompakte Meta-Tools
tools := []openai.Tool{
    {
        Name: "discover_capabilities",
        Description: "List available system capabilities and tools",
    },
    {
        Name: "execute_capability", 
        Description: "Execute a specific capability with given parameters",
    },
}

// Ebene 2: Vollständige Tool-Details werden per RAG geladen
```

---

## 7. Implementierungsplan

### Schritt-für-Schritt Umsetzung

#### Schritt 1: Vorbereitung (1 Tag)

- [ ] `internal/agent/tool_groups.go` erstellen
- [ ] Tool-zu-Gruppe Mapping definieren
- [ ] Konfigurations-Optionen hinzufügen

#### Schritt 2: Core-Implementierung (2-3 Tage)

- [ ] `ToolGroup` Strukturen implementieren
- [ ] `buildGroupedToolSchema()` Funktion erstellen
- [ ] Alle Tool-Operationen in Gruppen strukturieren

#### Schritt 3: Dispatch-Layer (2 Tage)

- [ ] Dispatch-Funktionen pro Gruppe implementieren
- [ ] Fehlerbehandlung und Logging
- [ ] Rückwärtskompatibilität sicherstellen

#### Schritt 4: Integration (1-2 Tage)

- [ ] Integration in `agent_loop.go`
- [ ] Konfiguration über `config.yaml`
- [ ] Feature-Flag für schrittweise Einführung

#### Schritt 5: Testing & Optimierung (2-3 Tage)

- [ ] Unit Tests für Dispatch-Logik
- [ ] Integrationstests mit realen Prompts
- [ ] Token-Zählung und Vergleich

### Konfigurations-Optionen

```yaml
# config.yaml
agent:
  # Feature-Flag für Tool Grouping
  use_tool_grouping: true
  
  tool_grouping:
    # Standard-Verhalten
    default_behavior: "grouped"  # grouped | individual | hybrid
    
    # Bei hybrid: Einzel-Tools für kleine Installationen
    individual_threshold: 10  # Weniger als 10 Tools -> einzeln
    
    # Aktivierte Gruppen (leer = alle)
    enabled_groups: []
    
    # Deaktivierte Gruppen
    disabled_groups: []
    
    # Benutzerdefinierte Gruppen
    custom_groups:
      my_custom_group:
        description: "Custom operations"
        tools:
          - tool1
          - tool2
```

---

## 8. Risiken & Mitigation

| Risiko | Wahrscheinlichkeit | Impact | Mitigation |
|--------|-------------------|--------|------------|
| LLM versteht gruppierte Tools nicht | Mittel | Hoch | Umfassende Tests, Fallback auf Einzel-Tools |
| Operation-Name-Kollisionen | Niedrig | Mittel | Namespace-Prefix: `docker.list`, `system.shell` |
| Verlust von Tool-Kontext | Mittel | Mittel | Recently-Used Tool-Tracking, Tool-Chains unterstützen |
| Breaking Change für bestehende Skills | Mittel | Hoch | Rückwärtskompatibilitätsschicht, Migration-Guide |
| Performance-Regression | Niedrig | Mittel | Benchmarking, Caching der Schemas |

---

## 9. Erfolgsmessung

### Metriken

| Metrik | Ziel | Messung |
|--------|------|---------|
| Token-Reduktion | >60% | Vorher/Nachher-Vergleich |
| Reaktionszeit | <+10% | Latenz-Messung |
| Tool-Nutzungsrate | Keine Änderung | Usage Analytics |
| Error-Rate | Keine Erhöhung | Error-Tracking |

### Test-Szenarien

1. **Einfache Anfrage:** "Wie spät ist es?" (wenige Tools nötig)
2. **Komplexe Anfrage:** "Starte meinen Docker-Container und aktualisiere Home Assistant" (mehrere Gruppen)
3. **Unbekannte Anfrage:** "Erledige X" (Discovery-Fähigkeit testen)

---

## 10. Zusammenfassung

### Empfohlene Vorgehensweise

1. **Phase 1 (Tool Grouping)** sofort umsetzen - hoher Impact, niedriges Risiko
2. **Phase 2 (Schema Compression)** parallel oder kurz danach
3. **Phase 3 (Tier-basiert)** nach erfolgreichem Rollout von Phase 1
4. **Phase 4 & 5** nur bei Bedarf (wenn Phasen 1-3 nicht ausreichen)

### Erwartetes Endergebnis

| Szenario | Tokens vorher | Tokens nachher | Einsparung |
|----------|---------------|----------------|------------|
| Minimal (5 Tools) | 1.500 | 600 | 60% |
| Standard (25 Tools) | 8.000 | 1.200 | 85% |
| Voll (80 Tools) | 28.000 | 1.800 | 94% |

---

*Dieser Plan wurde basierend auf der Analyse der aktuellen Implementierung in `internal/agent/native_tools.go` und `internal/prompts/builder.go` erstellt.*
