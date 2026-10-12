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

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | for show/pull/delete/load/unload | Model name (e.g. `llama3:latest`, `codellama:13b`, `mistral:7b`) |
| `source` | string | for copy | Source model name |
| `destination` | string | for copy | Destination model name |

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

## Configuration

```yaml
ollama:
  enabled: true
  url: "http://localhost:11434"  # Default local Ollama
  readonly: false  # Set true to block model changes
  managed_instance:
    enabled: false
    container_port: 11434
    use_host_gpu: false
    gpu_backend: "auto"
    default_models: []
    memory_limit: ""
    volume_path: ""
```

## Notes

- **Model naming**: Format is `modelname:tag` (e.g. `llama3:latest`, `mistral:7b`). Without tag, `:latest` is assumed.
- **GPU memory**: `load` keeps model in GPU for fast responses; `unload` frees GPU memory when not needed.
- **Read-only mode**: When `ollama.readonly: true`, pull/delete/copy/load/unload operations are blocked.
- **Common models**: llama3, mistral, codellama, phi3, Gemma, neural-chat
