# Coding Conventions

**Analysis Date:** 2026-04-03

## Language

**Primary:** Go 1.26.1+
**Comments:** English for all code comments
**Documentation:** English

## Naming Conventions

### Files
- **Multi-word files:** `snake_case.go` (e.g., `config_test.go`, `agent_loop.go`)
- **Single-word files:** `word.go`
- **Test files:** `*_test.go` co-located with source

### Types
- **Exported types:** `PascalCase` (e.g., `Agent`, `Vault`, `HistoryManager`)
- **Unexported types:** `camelCase` (e.g., `dockerConfig`, `historyMessage`)

### Functions
- **Exported functions:** `PascalCase` (e.g., `NewVault`, `Load`, `ExecuteWithRetry`)
- **Unexported functions:** `camelCase` (e.g., `buildLogger`, `parseWorkflowPlan`, `writeVaultFileAtomic`)

### Variables
- **General:** `camelCase` (e.g., `logger`, `configPath`, `vaultPath`)
- **Constants:** `PascalCase` for exported (e.g., `MaxScriptBytes`), `camelCase` for unexported (e.g., `dockerAPIVersion`)

### Packages
- **Naming:** Short, lowercase, no underscores (e.g., `config`, `llm`, `tools`)
- **Avoid:** Generic names like `util` or `common`

## Code Style

### Formatting
- **Tool:** `gofmt` (standard Go formatter)
- **No golangci configuration detected** - project relies on `gofmt` defaults

### Line Length
- No strict line length limit enforced

### Imports
Standard library first, then third-party, then internal:

```go
import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "path/filepath"

    "github.com/sashabaranov/go-openai"
    "github.com/gofrs/flock"

    "aurago/internal/config"
    "aurago/internal/llm"
    loggerPkg "aurago/internal/logger"
)
```

### Indentation
- **Tabs:** Use tabs for indentation (Go standard)
- **Align:** Multiple imports on separate lines when needed

## Error Handling

### Pattern
Always wrap errors with context using `fmt.Errorf`:

```go
// Wrong
return nil, err

// Correct
return nil, fmt.Errorf("failed to read config file: %w", err)
```

### Error Context Rules
1. **Include the operation that failed:** "failed to read", "failed to create", "failed to write"
2. **Include relevant identifiers:** file path, key name, device name
3. **Wrap the original error:** Always use `%w` to preserve the error chain

### Examples from codebase

```go
// From internal/security/vault.go
return nil, fmt.Errorf("invalid master key format, expected hex: %w", err)
return nil, fmt.Errorf("failed to read vault file: %w", err)
return nil, fmt.Errorf("failed to create cipher: %w", err)

// From internal/tools/docker.go
return nil, fmt.Errorf("build ping request: %w", err)
return nil, fmt.Errorf("docker unreachable: %w", err)
return nil, fmt.Errorf("docker _ping returned status %d", resp.StatusCode)
```

### Nil Checks
Check for nil early and return descriptive errors:

```go
if client == nil {
    return nil, fmt.Errorf("docker client is required")
}
```

### Sentinel Errors
For cases where error type matters:

```go
// Not detected - project primarily uses wrapped errors
```

## Logging

### Framework
**`log/slog`** - Standard library structured logging

### Logger Creation
```go
// Basic logger
logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

// With custom level
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

// Discard handler (for tests)
logger := slog.New(slog.NewTextHandler(io.Discard, nil))
```

### Log Levels
- **`slog.LevelDebug`:** Detailed debugging information
- **`slog.LevelInfo`:** General operational information (default)
- **`slog.LevelWarn`:** Warning conditions
- **`slog.LevelError`:** Error conditions

### Key-Value Pairs
Always use structured logging with key-value pairs:

```go
// Wrong
log.Info("Processing took 150ms")

// Correct
log.Info("query completed", "duration_ms", 150, "tokens", 42)
```

### Logger Injection
Components receive loggers via dependency injection:

```go
type DockerManager struct {
    client DockerClient
    logger *slog.Logger
}

func NewDockerManager(client DockerClient, logger *slog.Logger) (*DockerManager, error)
```

### Silent Operations
For expected silent failures:

```go
// os.IsNotExist is often silently handled
if os.IsNotExist(err) {
    return secrets, nil // Return empty map if file doesn't exist
}
```

## Comments

### Style
Use complete sentences with proper capitalization:

```go
// DockerPing checks if the Docker Engine is reachable at the given host.
// Returns nil on success, or an error describing the failure.
func DockerPing(host string) error
```

### When to Comment
1. **Public API documentation:** Explain purpose, parameters, return values
2. **Non-obvious logic:** Why something is done, not what it does
3. **Complex algorithms:** Step-by-step explanation
4. **Platform-specific code:** Why platform-specific handling exists

### What NOT to Comment
1. Obvious code that speaks for itself
2. Commented-out code (delete it, use git history)
3. TODO comments left in code (create issues instead)

### TODO/FIXME Pattern
Not heavily used. If present:

```go
// TODO(developer): Consider adding caching here
// FIXME: This breaks when X is true
```

## File Organization

### One Responsibility Per File
Each file should have a single, clear purpose:

```
internal/
├── config/
│   ├── config.go        # Load, parse, save config
│   ├── config_types.go  # Config struct definitions
│   └── config_test.go   # Config tests
├── security/
│   ├── vault.go         # Vault encryption
│   ├── vault_test.go    # Vault tests
│   └── guardian.go      # LLM Guardian
```

### Large File Threshold
If a file exceeds ~500-800 lines, consider splitting:

```go
// From CLAUDE.md: "If files get too big and unwieldy, split them into smaller pieces"
```

### Test File Co-location
Tests live alongside the code they test:

```
internal/config/config.go      # Implementation
internal/config/config_test.go # Tests
```

### Package Structure
- **`internal/`:** Private application code (not importable by other projects)
- **`cmd/`:** Application entry points
- **`internal/tools/`:** 90+ tool implementations
- **`internal/server/`:** 60+ HTTP server handlers

## Security Patterns

### Vault System
AES-256-GCM encryption for all secrets:

```go
// From internal/security/vault.go
type Vault struct {
    mu       sync.Mutex
    key      []byte      // 32 bytes (64 hex characters)
    filePath string
    fileLock *flock.Flock
}
```

### Master Key
- **Environment variable:** `AURAGO_MASTER_KEY`
- **Format:** 64 hex characters (32 bytes)
- **Never commit:** `.env` files, vault files

### Sensitive Data Scrubbing
Register values that should never appear in logs or LLM outputs:

```go
// From CLAUDE.md: Use security.RegisterSensitive(value)
```

### Atomic File Writes
Prevent partial writes using temp file + rename:

```go
// From internal/config/config.go
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    // Write to temp, sync, close, rename
    // Cleanup on failure
}
```

### Permission Toggles
Dangerous operations require explicit enable flags:

```yaml
allow_shell: false
allow_python: false
allow_filesystem_write: false
allow_network_requests: false
allow_remote_shell: false
allow_self_update: false
allow_mcp: false
allow_web_scraper: false
```

### Forbidden Vault Exports
Tools using vault secrets must be added to the forbidden export list:

```go
// From CLAUDE.md: "If you add a new tool or integration that uses the vault,
// add it to the list of secrets that are forbidden to be exported to Python tools!"
```

## Import Organization

### Order
1. Standard library (alphabetical within group)
2. Third-party packages
3. Internal packages (with alias if needed)

```go
import (
    // Standard library
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "os"
    "path/filepath"

    // Third-party
    "github.com/sashabaranov/go-openai"
    "github.com/gofrs/flock"

    // Internal (with alias if name conflicts)
    "aurago/internal/config"
    loggerPkg "aurago/internal/logger"
)
```

### Path Aliases
Use when package name conflicts or for clarity:

```go
loggerPkg "aurago/internal/logger"  // Avoids conflict with log/slog
```

## Function Design

### Constructor Pattern
Use `New` prefix for constructors that can fail:

```go
func NewVault(masterKeyHex string, filePath string) (*Vault, error)
func NewAgent(cfg *config.Config, logger *slog.Logger, ...) *Agent
func NewTracker(cfg *config.Config, logger *slog.Logger, dataDir string) *Tracker
```

### Parameter Order
1. Context (`ctx context.Context`) - if async/cancellable
2. Configuration
3. Logger
4. Dependencies
5. Input values
6. Output/callback parameters

### Return Values
- **Errors:** Return `error` as last return value
- **Multiple values:** Group related returns (result + error)

## Concurrency

### Mutex Pattern
Protect shared state with mutexes:

```go
type Vault struct {
    mu       sync.Mutex
    key      []byte
    filePath string
}

func (v *Vault) ReadSecret(key string) (string, error) {
    v.mu.Lock()
    defer v.mu.Unlock()
    // ... protected code
}
```

### Channel-Based Communication
For background workers:

```go
type HistoryManager struct {
    saveChan chan struct{}
    doneChan chan struct{}
}
```

### WaitGroups
For parallel operations:

```go
var wg sync.WaitGroup
wg.Add(workers)
for w := 0; w < workers; w++ {
    go func(w int) {
        defer wg.Done()
        // ... work
    }(w)
}
wg.Wait()
```

---

*Convention analysis: 2026-04-03*
