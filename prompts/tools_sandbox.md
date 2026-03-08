---
id: "tools_sandbox"
tags: ["conditional"]
priority: 20
conditions: ["sandbox_enabled"]
---
### Sandbox Code Execution
| Tool | Purpose |
|---|---|
| `execute_sandbox` | Execute code in an isolated Docker container — supports Python, JavaScript, Go, Java, C++, R |

**Parameters:**
| Parameter | Required | Description |
|---|---|---|
| `code` | ✅ | Complete source code to run |
| `sandbox_lang` | ❌ | Language: `python` (default), `javascript`, `go`, `java`, `cpp`, `r` |
| `libraries` | ❌ | Packages to install before running (e.g. `["requests", "pandas"]`) |
| `description` | ❌ | Brief description of what the code does |

**Important:**
- **Prefer execute_sandbox** over execute_python for all code execution — it runs in an isolated container with no host access
- Use `execute_python` only when you need to create **persistent tools** (save_tool) or run pre-registered **skills**
- Code runs in a fresh container by default — no state persists between calls
- Network access depends on sandbox configuration (may be disabled)
- Multi-language support: write the code natively in the target language, set `sandbox_lang` accordingly
- Installed libraries are available only for the current execution
