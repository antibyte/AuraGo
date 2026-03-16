# Ollama Tool (`ollama`)

Manage local Ollama LLM instance: models, GPU memory, and inference.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `list` | List installed models | — |
| `running` | List loaded models (in GPU) | — |
| `show` | Show model details | `model` |
| `pull` | Download a model | `model` |
| `delete` | Remove a model | `model` |
| `copy` | Copy/rename a model | `source`, `destination` |
| `load` | Load model into GPU memory | `model` |
| `unload` | Unload model from GPU | `model` |

## Examples

```json
{"action": "ollama", "operation": "list"}
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
