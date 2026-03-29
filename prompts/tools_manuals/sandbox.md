---
id: sandbox
tags: [core]
priority: 100
conditions: ["sandbox_enabled"]
---

# Sandbox Code Execution — Tool Manual

## Overview
The `execute_sandbox` tool runs code in an isolated Docker container via the llm-sandbox MCP server.
This is the **preferred** method for all code execution — it provides full isolation from the host system.

## Tool: `execute_sandbox`

### Basic Python execution
```json
{"action": "execute_sandbox", "code": "print('Hello, World!')", "sandbox_lang": "python"}
```

### JavaScript execution
```json
{"action": "execute_sandbox", "code": "console.log('Hello from Node.js');", "sandbox_lang": "javascript"}
```

### Go execution
```json
{
  "action": "execute_sandbox",
  "code": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello from Go\")\n}",
  "sandbox_lang": "go"
}
```

### With library installation
```json
{
  "action": "execute_sandbox",
  "code": "import requests\nresp = requests.get('https://api.github.com')\nprint(resp.status_code)",
  "sandbox_lang": "python",
  "libraries": ["requests"]
}
```

### Data processing with pandas
```json
{
  "action": "execute_sandbox",
  "code": "import pandas as pd\ndf = pd.DataFrame({'a': [1,2,3], 'b': [4,5,6]})\nprint(df.describe().to_json())",
  "sandbox_lang": "python",
  "libraries": ["pandas"]
}
```

## Supported Languages
| Language | `sandbox_lang` value | Container |
|---|---|---|
| Python | `python` (default) | python:3.11-slim |
| JavaScript | `javascript` | node:18-slim |
| Go | `go` | golang:1.26-alpine |
| Java | `java` | openjdk:17-slim |
| C++ | `cpp` | gcc:13-bookworm |
| R | `r` | r-base:latest |

## Fallback Behavior
If the sandbox is unavailable (Docker not running, package not installed, etc.) **and** the language is Python **and** `execute_python` is enabled in the Danger Zone, the code will automatically fall back to local Python execution. This is logged in the output.

## When to use `execute_python` instead
- **Persistent tools**: When saving a tool with `save_tool` — the tool must live on the host filesystem
- **Skills**: Running pre-registered skills via `execute_skill`
- **Venv packages**: When you need packages that persist across executions

## Configuration
```yaml
sandbox:
  enabled: true
  backend: "docker"          # or "podman"
  docker_host: ""            # auto-detect or inherit from docker.host
  image: "python:3.11-slim"  # base container image
  auto_install: true         # auto-install llm-sandbox[mcp-docker] Python package
  pool_size: 0               # container pool size (0 = no pooling)
  timeout_seconds: 30        # execution timeout per run
  network_enabled: false     # container network access
  keep_alive: true           # keep MCP server running between calls
```

> **Note:** `auto_install` installs `llm-sandbox[mcp-docker]` (or `mcp-podman` for Podman).
> Manual install: `pip install 'llm-sandbox[mcp-docker]'`
> The plain `llm-sandbox[docker]` extras are **not** sufficient — the `mcp` dependency is required.

## Security
- Code runs in an **isolated Docker container** — no access to the host filesystem
- No Danger Zone gate required — sandbox is inherently sandboxed
- Network access is configurable (disabled by default)
- Each execution gets a fresh environment (no state leakage between runs)
