## Tool: golangci-lint (`golangci_lint`)

Run static analysis on Go source code using [golangci-lint](https://github.com/golangci/golangci-lint), the industry-standard meta-linter. golangci-lint is automatically installed via `go install` if not already present on the system.

### Parameters

| Parameter | Required | Description |
|---|---|---|
| `path` | No | Package path or directory to lint. Defaults to `./...` (entire module). Examples: `./internal/agent`, `./cmd/aurago`, `./...` |
| `config` | No | Path to a `.golangci.yml` config file, relative to workspace root. Uses golangci-lint auto-detection if omitted. |

### Return Value

JSON object with the following fields:

| Field | Type | Description |
|---|---|---|
| `status` | string | `"ok"` (no issues), `"issues"` (lint findings), `"error"` (tool failure) |
| `message` | string | Human-readable summary |
| `issues` | array of strings | Individual lint findings, one per line (only present when `status="issues"`) |
| `issue_count` | integer | Number of findings |

### Examples

```json
{"action": "golangci_lint", "path": "./..."}
```

```json
{"action": "golangci_lint", "path": "./internal/tools", "config": ".golangci.yml"}
```

```json
{"action": "golangci_lint", "path": "./cmd/aurago"}
```

### Notes

- **Timeout**: 5 minutes maximum. For very large codebases, narrow the `path` to avoid timeouts.
- **Auto-install**: If `golangci-lint` is not in PATH or `~/go/bin`, it is installed via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`. This requires network access and may take ~30 seconds on first use.
- **Exit codes**: golangci-lint exits with code 1 when issues are found (not a tool error) and 2+ for configuration/runtime errors.
- **Config**: Without a config file, golangci-lint uses its built-in defaults which include around 10 popular linters. A `.golangci.yml` in the project root is auto-detected.
- Enable this tool via `golangci_lint.enabled: true` in `config.yaml`.
