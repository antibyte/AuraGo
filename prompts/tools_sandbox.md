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
| `libraries` | ❌ | Must be empty; dependencies belong in reviewed pinned images |
| `description` | ❌ | Brief description of what the code does |

**Important:**
- `execute_sandbox` is the default isolated executor for supported languages; see `ctx_coding_guidelines.md` for the full execution priority rules
- Code runs in a fresh container by default — no state persists between calls
- Workers have no network; authorized native calls use the scoped supervisor broker
- Multi-language support: write the code natively in the target language, set `sandbox_lang` accordingly
- Dependencies must be present in the reviewed pinned image
