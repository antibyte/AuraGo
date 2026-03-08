---
id: "tools_ollama"
tags: ["conditional"]
priority: 31
conditions: ["ollama_enabled"]
---
### Ollama Model Management
| Tool | Purpose |
|---|---|
| `ollama` | Manage local Ollama LLM instance: list models, pull/delete models, show details, load/unload from memory |

**Operations:**
- `list` — List all locally available models (name, size, date)
- `running` — List currently loaded/running models
- `show` — Show model details/metadata (requires model name)
- `pull` — Download a model (requires model name; may take a long time)
- `delete` — Remove a model from local storage (requires model name)
- `copy` — Copy/alias a model (requires source and destination)
- `load` — Preload a model into GPU/memory
- `unload` — Unload a model from GPU/memory

**Parameters:** `operation`, `model` (model name), `source`, `destination`
