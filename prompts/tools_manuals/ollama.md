# Ollama Tool (`ollama`)

Manage local Ollama LLM instance: models, GPU memory, and inference.

## Operations

### Model Management

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List all installed models | — |
| `running` | List loaded models and background operation receipts, including pull failures | — |
| `show` | Show detailed model info (size, capabilities) | `model` |
| `pull` | Start a background model download and return its operation receipt | `model` |
| `delete` | Delete a locally stored model | `model` |
| `copy` | Copy/rename a model | `source`, `destination` |
| `load` | Load model into GPU memory for fast inference | `model` |
| `unload` | Unload model from GPU memory | `model` |

## Parameters

Use the canonical operation names above for native tool calls. Older text-call
aliases remain compatible but are not part of the native schema. A successful
`pull` response means accepted, not installed: check `running.operations` for the
returned ID. Only `state=ready` confirms completion. `failed`, `cancelled`, and
`unknown` must not be reported as success. An absent receipt after restart is
unknown. A refused connection or failed DNS lookup is `failed` and can be retried
after connectivity is restored. For `unknown`, inspect `list` and ask the
administrator to resolve the uncertain outcome; repeating `pull` alone does not
replay that write. Downloads stop on server shutdown, target changes, or revoked permissions. Container
lifecycle belongs to administrator settings, not the generic Docker tool.

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
