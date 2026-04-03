# code_analysis — Code Analysis Tool

Analyzes code structure, finds functions/classes, measures complexity, and extracts dependencies. Supports multiple languages (Go, Python, JavaScript/TypeScript, etc.).

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `functions` | Extract all functions/methods from a file | `file_path` |
| `classes` | Extract all classes/types from a file | `file_path` |
| `imports` | Extract all imports/dependencies from a file | `file_path` |
| `complexity`| Calculate cyclomatic complexity for functions | `file_path` |
| `todo` | Find all TODO/FIXME comments in a file | `file_path` |

## Key Behaviors

- Analyzes the AST (Abstract Syntax Tree) where possible, or uses robust regex for unsupported languages.
- Returns structured JSON data containing line numbers, signatures, and context.
- Helps quickly understand large files without reading them entirely into context.

## Examples

```
# List all functions in a Go file
code_analysis(operation="functions", file_path="internal/server/server.go")

# Find classes in a Python script
code_analysis(operation="classes", file_path="script.py")

# Check imports to see dependencies
code_analysis(operation="imports", file_path="ui/js/app.js")

# Find complex areas of code
code_analysis(operation="complexity", file_path="cmd/main.go")
```

## Tips
- Use this BEFORE editing large files to locate the exact functions you need to change.
- Combining `functions` and `read_file` (with specific line ranges) saves context tokens.