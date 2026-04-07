# Ollama Tool (`ollama`)

Manage local Ollama LLM instance: models, GPU memory, and inference.

## Operations

### Model Management

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list`, `list_models` | List all installed models | — |
| `running`, `ps` | List models currently loaded in GPU | — |
| `show`, `info` | Show detailed model info (size, capabilities) | `model` |
| `pull`, `download` | Download/pull a model from Ollama library | `model` |
| `delete`, `remove` | Delete a locally stored model | `model` |
| `copy` | Copy/rename a model | `source`, `destination` |
| `load` | Load model into GPU memory for fast inference | `model` |
| `unload` | Unload model from GPU memory | `model` |

### Container Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `container_status` | Get Ollama container status | — |
| `container_start` | Start the Ollama container | — |
| `container_stop` | Stop the Ollama container | — |
| `container_restart` | Restart the Ollama container | — |
| `container_logs` | Get container logs | `tail` |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | for show/pull/delete/load/unload | Model name (e.g. `llama3:latest`, `codellama:13b`, `mistral:7b`) |
| `source` | string | for copy | Source model name |
| `destination` | string | for copy | Destination model name |
| `tail` | integer | for container_logs | Number of log lines to retrieve (default: 100) |

## Examples

```json
{"action": "ollama", "operation": "list"}
```

```json
{"action": "ollama", "operation": "running"}
```

```json
{"action": "ollama", "operation": "pull", "model": "llama3:latest"}
```

```json
{"action": "ollama", "operation": "show", "model": "codellama:13b"}
```

```json
{"action": "ollama", "operation": "load", "model": "llama3:latest"}
```

```json
{"action": "ollama", "operation": "unload", "model": "llama3:latest"}
```

```json
{"action": "ollama", "operation": "container_logs", "tail": 50}
```

## Configuration

```yaml
ollama:
  enabled: true
  url: "http://localhost:11434"  # Default local Ollama
  read_only: false  # Set true to block model changes
```

## Notes

- **Model naming**: Format is `modelname:tag` (e.g. `llama3:latest`, `mistral:7b`). Without tag, `:latest` is assumed.
- **GPU memory**: `load` keeps model in GPU for fast responses; `unload` frees GPU memory when not needed.
- **Container mode**: If Ollama runs in Docker, use container operations for lifecycle management.
- **Read-only mode**: When `ollama.read_only: true`, pull/delete/copy/load/unload operations are blocked.
- **Common models**: llama3, mistral, codellama, phi3, Gemma, neural-chat
