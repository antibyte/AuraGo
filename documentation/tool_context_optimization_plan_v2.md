# Tool-Kontext-Optimierungsplan v2

> **Status:** Überarbeitet nach kritischer Bewertung  
> **Ziel:** Realistische Reduktion der Tool-Schema-Größe um 40-60%  
> **Datum:** 2026-03-16  
> **Priorität:** Korrektheit > Kompression

---

## 1. Analyse der Ausgangssituation

### Aktuelles Problem

- **80 aktive Tools** ≈ **15.000-25.000 Tokens** (im OpenAI Function-Calling Format)
- Bei einem 8K Kontextfenster bleiben nur **3.000-5.000 Tokens** für Konversationshistorie
- Bei einem 4K Kontextfenster ist es **praktisch unbenutzbar**

### Warum der ursprüngliche Plan scheitern würde

| Ursprüngliche Idee | Problem | Konsequenz |
|-------------------|---------|------------|
| Tool Grouping mit `parameters: {type: object}` | LLM sieht keine Parameter-Struktur | Kann keine korrekten Argumente generieren |
| Schema Compression (`"t"` statt `"type"`) | OpenAI API validiert striktes JSON Schema | Schema-Validierungsfehler |
| Lazy Loading (Discovery + Execution) | Zwei sequentielle LLM-Aufrufe | Latenz verdoppelt sich |
| 94% Token-Ersparnis bei Grouping | Realistisch: 50-60% | Fehleinschätzung um Faktor 2 |

---

## 2. Realistische Lösungsstrategien

### Übersicht

| Phase | Strategie | Realistische Einsparung | Risiko | Umsetzung |
|-------|-----------|------------------------|--------|-----------|
| 1 | **Beschreibungen optimieren** | 20-30% | Keines | 1 Tag |
| 2 | **Tool-Familien (Hybride Gruppierung)** | 40-50% | Mittel | 3-4 Tage |
| 3 | **Kontext-sensitive Tool-Auswahl** | 30-40% | Mittel | 1 Woche |
| 4 | **Schema-Deduplizierung** | 10-15% | Niedrig | 2 Tage |

**Empfohlene Kombination (Phase 1+2):** 50-60% Reduktion bei kontrollierbarem Risiko

---

## 3. Phase 1: Beschreibungen Optimieren (Sofort umsetzbar)

### Problem

Aktuelle Tool-Beschreibungen sind redundant und lang:

```go
// Vorher: ~180 Zeichen, viel Füllwort
"Manage Docker containers, images, networks, and volumes. List, inspect, 
 start, stop, create, remove containers; pull/remove images; view logs..."
```

### Lösung

Prägnante, strukturierte Beschreibungen:

```go
// Nachher: ~60 Zeichen, gleiche Information
"Docker: containers[list/start/stop/logs/create/remove], images, networks, volumes"
```

### Implementierung

```go
// internal/agent/native_tools.go - Refactoring

// Hilfsfunktion für kompakte Beschreibungen
func toolDesc(name string, ops []string) string {
    if len(ops) <= 3 {
        return fmt.Sprintf("%s: %s", name, strings.Join(ops, "/"))
    }
    return fmt.Sprintf("%s: %s...(%d more)", name, 
        strings.Join(ops[:3], "/"), len(ops)-3)
}

// Beispiel-Refactoring
func builtinToolSchemas(ff ToolFeatureFlags) []openai.Tool {
    var tools []openai.Tool
    
    // Vorher
    // tools = append(tools, tool("docker",
    //     "Manage Docker containers, images, networks, and volumes...",
    //     schema(...)))
    
    // Nachher
    tools = append(tools, tool("docker",
        "Docker: containers[list/start/stop/logs], images[pull/remove], networks, volumes",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type": "string",
                "enum": []string{
                    "list_containers", "inspect", "start", "stop", 
                    "restart", "logs", "create", "run", 
                    "list_images", "pull", "remove_image",
                    "list_networks", "list_volumes", "info",
                },
                "description": "Operation: list_containers, start, stop, logs, pull, etc.",
            },
            // ... restliche Parameter
        }, "operation")))
    
    return tools
}
```

### Parameter-Beschreibungen kürzen

```go
// Vorher
"file_path": prop("string", "Path to the file or directory to read from")

// Nachher  
"file_path": prop("string", "File/directory path")
```

### Erwartete Einsparung

| Komponente | Vorher | Nachher | Einsparung |
|------------|--------|---------|------------|
| Tool-Beschreibungen | ~150 Zeichen/Tool | ~60 Zeichen/Tool | ~60% |
| Parameter-Beschreibungen | ~40 Zeichen/Param | ~20 Zeichen/Param | ~50% |
| **Gesamt (80 Tools)** | ~20.000 Tokens | ~14.000 Tokens | **~30%** |

---

## 4. Phase 2: Tool-Familien (Hybride Gruppierung)

### Konzept

Statt 80 Einzel-Tools oder einem generischen "execute_operation", werden **10-15 logische Tool-Familien** gebildet. Jede Familie hat ein **vollständiges, valides JSON Schema**.

### Architektur

```go
// Vorher: 80 einzelne Tools
// - docker
// - docker_management  
// - container_stats
// - ...

// Nachher: 12 Tool-Familien
// - system_files       (filesystem, shell)
// - system_info        (system_metrics, process_management)
// - docker_ops         (docker, docker_management)
// - infrastructure     (proxmox, ansible, tailscale)
// - home_automation    (home_assistant, mqtt, wol)
// - media_ops          (image_gen, transcribe, tts, media_registry)
// - development        (homepage, github, git, netlify)
// - memory_ops         (manage_memory, query_memory, knowledge_graph)
// - notes_journal      (manage_notes, manage_journal)
// - scheduling         (cron_scheduler, manage_missions)
// - integrations       (adguard, ollama, meshcentral)
// - utilities          (analyze_image, send_image, transcribe_audio)
```

### Implementierung

```go
// internal/agent/tool_families.go

package agent

import "github.com/sashabaranov/go-openai"

type ToolFamily struct {
    Name        string
    Description string
    Operations  []FamilyOperation
}

type FamilyOperation struct {
    Name        string
    Description string
    Params      map[string]interface{}
}

// buildSystemFilesTool kombiniert filesystem + shell
func buildSystemFilesTool(ff ToolFeatureFlags) openai.Tool {
    if !ff.AllowShell {
        // Nur filesystem wenn shell deaktiviert
        return tool("system_files",
            "File operations: read, write, delete, move, list",
            schema(map[string]interface{}{
                "operation": map[string]interface{}{
                    "type": "string",
                    "enum": []string{"read", "write", "delete", "move", "list", "stat"},
                },
                "path": prop("string", "File/directory path"),
                "content": prop("string", "Content for write operations"),
                "destination": prop("string", "Destination path for move"),
            }, "operation", "path"))
    }
    
    // Kombiniert filesystem + shell
    return tool("system_files",
        "File and shell operations: files[read/write/delete/move/list] + shell[execute]",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type": "string",
                "enum": []string{
                    "file_read", "file_write", "file_delete", "file_move", 
                    "file_list", "file_stat",
                    "shell_execute",
                },
                "description": "file_* operations or shell_execute",
            },
            "path": prop("string", "Path (for file operations)"),
            "content": prop("string", "Content (for file_write)"),
            "command": prop("string", "Shell command (for shell_execute)"),
            "background": map[string]interface{}{
                "type": "boolean", 
                "description": "Run shell in background",
            },
        }, "operation"))
}

// buildDockerTool kombiniert alle Docker-Operationen
func buildDockerTool(ff ToolFeatureFlags) openai.Tool {
    if !ff.DockerEnabled {
        return openai.Tool{} // Leer, wird gefiltert
    }
    
    return tool("docker_ops",
        "Docker: containers[list/start/stop/logs/exec], images[pull/build], networks, volumes",
        schema(map[string]interface{}{
            "operation": map[string]interface{}{
                "type": "string",
                "enum": []string{
                    "container_list", "container_start", "container_stop", 
                    "container_restart", "container_logs", "container_exec",
                    "image_list", "image_pull", "image_build", "image_remove",
                    "network_list", "volume_list", "system_info",
                },
            },
            "target": prop("string", "Container/image/volume name or ID"),
            "command": prop("string", "Command for container_exec"),
            "image": prop("string", "Image name for pull/build"),
            "dockerfile": prop("string", "Dockerfile path for build"),
        }, "operation"))
}

// Weitere Tool-Familien...

// BuildToolFamilies erstellt alle Tool-Schemas
func BuildToolFamilies(ff ToolFeatureFlags) []openai.Tool {
    var tools []openai.Tool
    
    // Immer verfügbare Tools
    tools = append(tools, buildSystemFilesTool(ff))
    tools = append(tools, buildSystemInfoTool(ff))
    tools = append(tools, buildMemoryOpsTool(ff))
    
    // Optional: Docker
    if dockerTool := buildDockerTool(ff); dockerTool.Function != nil {
        tools = append(tools, dockerTool)
    }
    
    // Optional: Home Automation
    if ff.HomeAssistantEnabled || ff.MQTTEnabled {
        tools = append(tools, buildHomeAutomationTool(ff))
    }
    
    // Optional: Infrastructure
    if ff.ProxmoxEnabled || ff.AnsibleEnabled || ff.TailscaleEnabled {
        tools = append(tools, buildInfrastructureTool(ff))
    }
    
    // Optional: Media
    if ff.ImageGenerationEnabled || ff.AllowPython {
        tools = append(tools, buildMediaOpsTool(ff))
    }
    
    // Optional: Development
    if ff.HomepageEnabled || ff.GitHubEnabled || ff.NetlifyEnabled {
        tools = append(tools, buildDevelopmentTool(ff))
    }
    
    // Optional: Integrations
    if ff.AdGuardEnabled || ff.OllamaEnabled || ff.MCPEnabled {
        tools = append(tools, buildIntegrationsTool(ff))
    }
    
    return tools
}
```

### Dispatch-Implementierung

```go
// internal/agent/agent_dispatch_families.go

func DispatchToolFamily(ctx context.Context, call ToolCall, cfg *config.Config, 
    logger *slog.Logger, /* dependencies */) string {
    
    switch call.Action {
    case "system_files":
        return dispatchSystemFiles(call, cfg, logger)
    case "system_info":
        return dispatchSystemInfo(call, cfg, logger)
    case "docker_ops":
        return dispatchDockerOps(call, cfg, logger)
    case "home_automation":
        return dispatchHomeAutomation(call, cfg, logger)
    case "infrastructure":
        return dispatchInfrastructure(call, cfg, logger)
    case "media_ops":
        return dispatchMediaOps(call, cfg, logger)
    case "development":
        return dispatchDevelopment(call, cfg, logger)
    case "memory_ops":
        return dispatchMemoryOps(call, cfg, logger)
    case "notes_journal":
        return dispatchNotesJournal(call, cfg, logger)
    case "scheduling":
        return dispatchScheduling(call, cfg, logger)
    case "integrations":
        return dispatchIntegrations(call, cfg, logger)
    case "utilities":
        return dispatchUtilities(call, cfg, logger)
    default:
        return fmt.Sprintf("[Error] Unknown tool family: %s", call.Action)
    }
}

func dispatchDockerOps(call ToolCall, cfg *config.Config, logger *slog.Logger) string {
    operation := call.Operation
    
    // Map family-operation zu ursprünglichem Tool
    switch operation {
    case "container_list":
        return tools.DockerListContainers(cfg, logger)
    case "container_start":
        return tools.DockerStartContainer(call.Target, cfg, logger)
    case "container_stop":
        return tools.DockerStopContainer(call.Target, cfg, logger)
    case "container_logs":
        return tools.DockerContainerLogs(call.Target, cfg, logger)
    case "image_pull":
        return tools.DockerPullImage(call.Image, cfg, logger)
    // ... weitere Mappings
    default:
        return fmt.Sprintf("[Error] Unknown docker operation: %s", operation)
    }
}
```

### Vorteile dieser Herangehensweise

1. ✅ **Vollständige JSON Schemas** - LLM sieht alle Parameter
2. ✅ **API-kompatibel** - Keine Schema-Validierungsfehler
3. ✅ **Kontrollierbare Komplexität** - 12 Dispatcher statt 80
4. ✅ **Kombinierbar** - Mehrere kleine Tools zu einer Familie zusammenfassen

### Erwartete Einsparung

| Szenario | Einzel-Tools | Tool-Familien | Einsparung |
|----------|--------------|---------------|------------|
| 80 Tools aktiv | ~20.000 Tokens | ~8.000-10.000 Tokens | **50-60%** |
| 40 Tools aktiv | ~10.000 Tokens | ~5.000-6.000 Tokens | **40-50%** |

---

## 5. Phase 3: Kontext-sensitive Tool-Auswahl

### Konzept

Nicht alle Tool-Familien müssen in jeder Anfrage verfügbar sein. Basierend auf der Konversationshistorie werden **wahrscheinlich benötigte Familien** aktiviert.

### Implementierung

```go
// internal/agent/tool_selector.go

type ToolSelector struct {
    alwaysActive []string // Immer aktiv: system_files, memory_ops
    contextTools map[string][]string // Keywords -> Tool-Familien
}

func NewToolSelector() *ToolSelector {
    return &ToolSelector{
        alwaysActive: []string{"system_files", "system_info", "memory_ops"},
        contextTools: map[string][]string{
            "docker":     {"docker_ops"},
            "container":  {"docker_ops"},
            "compose":    {"docker_ops"},
            "proxmox":    {"infrastructure"},
            "vm":         {"infrastructure"},
            "server":     {"infrastructure"},
            "ansible":    {"infrastructure"},
            "home assistant": {"home_automation"},
            "smart home":     {"home_automation"},
            "mqtt":       {"home_automation"},
            "licht":      {"home_automation"}, // Deutsch
            "heizung":    {"home_automation"},
            "image":      {"media_ops"},
            "bild":       {"media_ops"},
            "audio":      {"media_ops"},
            "website":    {"development"},
            "deploy":     {"development"},
            "github":     {"development"},
            "git":        {"development"},
            "note":       {"notes_journal"},
            "journal":    {"notes_journal"},
            "schedule":   {"scheduling"},
            "cron":       {"scheduling"},
            "adguard":    {"integrations"},
            "ollama":     {"integrations"},
        },
    }
}

func (ts *ToolSelector) SelectForMessage(message string, recentTools []string) []string {
    selected := make(map[string]bool)
    
    // 1. Immer aktiv
    for _, tool := range ts.alwaysActive {
        selected[tool] = true
    }
    
    // 2. Keyword-basierte Selektion
    msgLower := strings.ToLower(message)
    for keyword, tools := range ts.contextTools {
        if strings.Contains(msgLower, keyword) {
            for _, tool := range tools {
                selected[tool] = true
            }
        }
    }
    
    // 3. Recently used Tools
    for _, tool := range recentTools {
        selected[tool] = true
    }
    
    return mapToSlice(selected)
}

// In agent_loop.go
activeFamilies := toolSelector.SelectForMessage(lastUserMsg, recentTools)
toolSchemas := filterFamilies(allFamilies, activeFamilies)
```

### Fallback-Strategie

Wenn ein Tool nicht verfügbar war aber gebraucht wird:

```go
// In DispatchToolFamily
if !isToolAvailable(call.Action, activeFamilies) {
    return fmt.Sprintf("[Info] Tool '%s' ist nicht aktiv. Verfügbare Tools: %s. " +
        "Frage den Benutzer explizit nach diesem Tool, um es zu aktivieren.", 
        call.Action, strings.Join(activeFamilies, ", "))
}
```

### Erwartete Einsparung

| Szenario | Alle Familien (12) | Kontext-aktiv (4-6) | Einsparung |
|----------|-------------------|---------------------|------------|
| Tokens | ~10.000 | ~4.000-6.000 | **40-60%** |

---

## 6. Phase 4: Schema-Deduplizierung

### Konzept

Viele Tools teilen ähnliche Parameter-Strukturen (z.B. `operation`, `limit`, `path`). Diese können durch **JSON Schema Referenzen** dedupliziert werden.

> **Hinweis:** Dies ist optional und bringt nur 10-15% zusätzliche Einsparung.

```go
// Gemeinsame Parameter-Definitionen
var commonParams = map[string]interface{}{
    "operation": map[string]interface{}{
        "type": "string",
        "description": "Operation to perform",
    },
    "limit": map[string]interface{}{
        "type": "integer",
        "description": "Max results",
        "default": 10,
    },
    "path": map[string]interface{}{
        "type": "string", 
        "description": "File/directory path",
    },
}

// Wiederverwendung in Tools
func buildToolWithCommonParams(name, desc, opEnum string) openai.Tool {
    return tool(name, desc,
        schema(map[string]interface{}{
            "operation": commonParams["operation"],
            "path":      commonParams["path"],
            "limit":     commonParams["limit"],
            // Tool-spezifische Parameter...
        }, "operation"))
}
```

---

## 7. Kombinierte Strategie

### Empfohlene Kombination

```
Phase 1 (Beschreibungen optimieren)    ──┐
                                          ├──► 50-60% Einsparung
Phase 2 (Tool-Familien)                ──┘

Phase 3 (Kontext-sensitive Auswahl)    ─────► Weitere 30-40% (optional)
```

### Implementierungs-Reihenfolge

1. **Woche 1:** Phase 1 + Phase 2
   - Beschreibungen kürzen (1 Tag)
   - Tool-Familien implementieren (3-4 Tage)
   - Testing & Bugfixing (1-2 Tage)

2. **Woche 2 (optional):** Phase 3
   - Kontext-sensitive Auswahl
   - Keyword-Matching optimieren
   - Fallback-Mechanismen

### Erwartetes Endergebnis

| Konfiguration | Tokens vorher | Tokens nachher | Reduktion |
|---------------|---------------|----------------|-----------|
| Nur Phase 1 | 20.000 | 14.000 | 30% |
| Phase 1+2 | 20.000 | 8.000-10.000 | **50-60%** |
| Phase 1+2+3 | 20.000 | 4.000-6.000 | **70-80%** |

---

## 8. Risiko-Mitigation

| Risiko | Wahrscheinlichkeit | Mitigation |
|--------|-------------------|------------|
| LLM verwirrt durch neue Tool-Namen | Mittel | Übergangsfrist mit Aliasen, gute Dokumentation |
| Falsche Tool-Auswahl (Phase 3) | Mittel | Fallback: "Tool nicht verfügbar, verfügbar sind: X, Y, Z" |
| Parameter-Mapping-Fehler | Niedrig | Umfassende Unit-Tests für Dispatch-Layer |
| Breaking Change für API-User | Hoch | Config-Flag `use_tool_families: false` für Rückwärtskompatibilität |

### Rückwärtskompatibilität

```yaml
# config.yaml
agent:
  # Feature-Flag für schrittweise Migration
  tool_organization: "families"  # legacy | families | hybrid
  
  # Bei hybrid: Alte Tool-Namen werden auf neue Familien gemappt
  hybrid_mapping:
    docker: "docker_ops"
    home_assistant: "home_automation"
    # ...
```

---

## 9. Testing-Strategie

### Unit Tests

```go
// Test: Tool-Familie generiert valides Schema
func TestDockerToolFamilySchema(t *testing.T) {
    tool := buildDockerTool(ToolFeatureFlags{DockerEnabled: true})
    
    // Schema muss validierbar sein
    schemaJSON, _ := json.Marshal(tool.Function.Parameters)
    assert.NotEmpty(t, schemaJSON)
    assert.Contains(t, string(schemaJSON), "operation")
    assert.Contains(t, string(schemaJSON), "enum")
}

// Test: Dispatch routed korrekt
func TestDispatchDockerOps(t *testing.T) {
    call := ToolCall{Action: "docker_ops", Operation: "container_list"}
    result := DispatchToolFamily(context.Background(), call, cfg, logger)
    assert.NotContains(t, result, "[Error]")
}
```

### Integration Tests

```go
// Test: End-to-End mit Mock-LLM
func TestToolFamilyEndToEnd(t *testing.T) {
    // Simuliere LLM-Request mit Tool-Familie
    tools := BuildToolFamilies(allFeaturesEnabled)
    assert.Len(t, tools, 10) // Erwarte ~10 Familien statt 80 Tools
    
    // Validiere alle Schemas
    for _, tool := range tools {
        assert.NotNil(t, tool.Function)
        assert.NotEmpty(t, tool.Function.Name)
        assert.NotEmpty(t, tool.Function.Description)
    }
}
```

---

## 10. Zusammenfassung

### Was wurde korrigiert?

| Ursprünglicher Plan (Fehlerhaft) | Korrigierte Version |
|----------------------------------|---------------------|
| Tool Grouping mit `parameters: object` | Tool-Familien mit vollständigen Schemas |
| Schema Compression (`"t"` statt `"type"`) | Beschreibungen kürzen (API-kompatibel) |
| Lazy Loading (2 LLM-Calls) | Kontext-sensitive Auswahl (1 LLM-Call) |
| 94% Einsparung | Realistische 50-60% (Phase 1+2) |

### Empfohlene Umsetzung

1. **Sofort:** Phase 1 (Beschreibungen kürzen) - 30% Gewinn, kein Risiko
2. **Diese Woche:** Phase 2 (Tool-Familien) - Weitere 20-30%, kontrollierbares Risiko
3. **Nächste Woche (optional):** Phase 3 (Kontext-sensitive) - Weitere 30%, wenn nötig

### Realistisches Endergebnis

| Szenario | Vorher | Nachher | Einsparung | Risiko |
|----------|--------|---------|------------|--------|
| Phase 1 | 20.000 | 14.000 | 30% | Keines |
| **Phase 1+2** | **20.000** | **8.000-10.000** | **50-60%** | **Mittel** |
| Phase 1+2+3 | 20.000 | 4.000-6.000 | 70-80% | Mittel-Hoch |

---

*Dieser überarbeitete Plan berücksichtigt die technischen Limitationen der OpenAI API und bietet realistisch umsetzbare Lösungen.*
