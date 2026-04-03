# Testing Patterns

**Analysis Date:** 2026-04-03

## Test Framework

**Framework:** Go testing (`testing` package)
**Assertion:** Standard Go error-based assertions
**Mocking:** Manual mocks (no mocking framework detected)
**Run Commands:**

```bash
# All tests
go test ./...

# Specific package
go test ./internal/config/...
go test ./internal/agent/...
go test ./internal/memory/...

# Verbose output
go test -v ./internal/tools/...

# Coverage
go test -cover ./internal/...

# Race detection
go test -race ./...

# Benchmarks
go test -bench=. ./internal/...
```

## Test File Organization

### Location
Test files are co-located with source files:

```
internal/
├── config/
│   ├── config.go          # Implementation
│   └── config_test.go     # Tests
├── memory/
│   ├── history.go         # Implementation
│   └── history_test.go    # Tests
├── security/
│   ├── vault.go           # Implementation
│   └── vault_test.go      # Tests
```

### Naming Convention
- **Pattern:** `*_test.go`
- **Source matching:** Tests for `config.go` are in `config_test.go`

### Test Function Naming
- **Pattern:** `TestFunctionName` for unit tests
- **Subtests:** Use `t.Run` for table-driven test cases:

```go
func TestGetSpecialist(t *testing.T) {
    tests := []struct {
        role    string
        wantNil bool
    }{
        {"researcher", false},
        {"coder", false},
        {"unknown", true},
    }
    for _, tt := range tests {
        t.Run(tt.role, func(t *testing.T) {
            got := cfg.GetSpecialist(tt.role)
            if (got == nil) != tt.wantNil {
                t.Errorf("GetSpecialist(%q) nil=%v, wantNil=%v", tt.role, got == nil, tt.wantNil)
            }
        })
    }
}
```

## Test Structure

### Basic Unit Test Pattern

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadAbsolutePaths(t *testing.T) {
    // Setup: create temp directory and config file
    tmpDir, err := os.MkdirTemp("", "config_test")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)

    configPath := filepath.Join(tmpDir, "config.yaml")
    configContent := `directories:`

    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        t.Fatalf("failed to write config file: %v", err)
    }

    // Execute
    cfg, err := Load(configPath)
    if err != nil {
        t.Fatalf("failed to load config: %v", err)
    }

    // Assert
    if cfg.Directories.DataDir != expectedDataDir {
        t.Errorf("expected DataDir %s, got %s", expectedDataDir, cfg.Directories.DataDir)
    }
}
```

### Table-Driven Tests

```go
func TestParseWorkflowPlan(t *testing.T) {
    tests := []struct {
        name         string
        content      string
        wantTools    []string
        wantStripped string
    }{
        {
            name:         "No tag",
            content:      "Hello, I will search for that.",
            wantTools:    nil,
            wantStripped: "Hello, I will search for that.",
        },
        {
            name:         "Single tool",
            content:      `<workflow_plan>["docker"]</workflow_plan>`,
            wantTools:    []string{"docker"},
            wantStripped: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            gotTools, gotStripped := parseWorkflowPlan(tt.content)
            // assertions...
        })
    }
}
```

### Test Fixtures with Helper Functions

```go
func testConfig(limit float64, enforcement string) *config.Config {
    cfg := &config.Config{}
    cfg.Budget.Enabled = true
    cfg.Budget.DailyLimitUSD = limit
    cfg.Budget.Enforcement = enforcement
    cfg.Budget.WarningThreshold = 0.8
    return cfg
}
```

### Temporary Directory Usage

```go
// Use t.TempDir() for automatic cleanup
func TestHistoryManager_PersistAndLoad(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "history.json")

    hm := NewHistoryManager(path)
    defer hm.Close()
    // test code...
}
```

## Mocking Patterns

### Interface-Based Mocking

```go
// Define an interface for the dependency
type SecretVault interface {
    ReadSecret(key string) (string, error)
    WriteSecret(key, value string) error
}

// Create a test implementation
type testSecretVault struct {
    data map[string]string
}

func (v *testSecretVault) ReadSecret(key string) (string, error) {
    return v.data[key], nil
}

func (v *testSecretVault) WriteSecret(key, value string) error {
    if v.data == nil {
        v.data = map[string]string{}
    }
    v.data[key] = value
    return nil
}

// Use in tests
func TestMigrateEggModeSharedKeyToVault(t *testing.T) {
    vault := &testSecretVault{data: map[string]string{}}
    MigrateEggModeSharedKeyToVault(configPath, vault, slog.Default())
}
```

### Global Variable Mocking (Used Sparingly)

```go
// Temporarily replace global state
func TestSandboxExecuteCode_ManagerNil(t *testing.T) {
    sandboxMgrMu.Lock()
    old := globalSandboxMgr
    globalSandboxMgr = nil
    sandboxMgrMu.Unlock()
    t.Cleanup(func() {
        sandboxMgrMu.Lock()
        globalSandboxMgr = old
        sandboxMgrMu.Unlock()
    })

    _, err := SandboxExecuteCode(`print("hi")`, "python", nil, 5, nil)
    if err == nil {
        t.Fatal("expected error when sandbox manager is nil")
    }
}
```

### Logger Mocking

```go
// Discard logger for silent tests
logger := slog.New(slog.NewTextHandler(io.Discard, nil))

// Capture logs for assertion
logger := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{
    Level: slog.LevelError,
}))
```

## Test Patterns by Area

### Config Tests (`internal/config/config_test.go`)

- Create temp YAML files
- Load config and assert values
- Test migration logic
- Test Save/Load round-trip

```go
func TestConfigSaveWritesUpdatedField(t *testing.T) {
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "config.yaml")
    // write config...
    cfg, err := Load(configPath)
    cfg.Server.UILanguage = "de"
    if err := cfg.Save(configPath); err != nil {
        t.Fatalf("Save() error = %v", err)
    }
    // verify saved content
}
```

### Memory Tests (`internal/memory/history_test.go`)

- Test in-memory operations
- Test disk persistence
- Test concurrent access with goroutines
- Test edge cases (empty, pinned messages, trim limits)

```go
func TestHistoryManager_ConcurrentAdd(t *testing.T) {
    hm := NewEphemeralHistoryManager()
    defer hm.Close()

    const workers = 10
    const msgsPerWorker = 20
    var wg sync.WaitGroup
    wg.Add(workers)
    for w := 0; w < workers; w++ {
        go func(w int) {
            defer wg.Done()
            for i := 0; i < msgsPerWorker; i++ {
                _ = hm.Add("user", "concurrent", int64(w*1000+i), false, false)
            }
        }(w)
    }
    wg.Wait()
}
```

### Security Tests (`internal/security/vault_test.go`)

- Test encryption/decryption
- Test atomic write/read
- Test key validation

```go
func TestVaultWriteSecretPersistsAtomically(t *testing.T) {
    vaultPath := filepath.Join(t.TempDir(), "vault.bin")
    v, err := NewVault(strings.Repeat("a", 64), vaultPath)
    if err != nil {
        t.Fatalf("NewVault() error = %v", err)
    }

    if err := v.WriteSecret("demo", "value"); err != nil {
        t.Fatalf("WriteSecret() error = %v", err)
    }

    got, err := v.ReadSecret("demo")
    if err != nil {
        t.Fatalf("ReadSecret() error = %v", err)
    }
    if got != "value" {
        t.Fatalf("secret = %q, want value", got)
    }
}
```

### Budget Tests (`internal/budget/tracker_test.go`)

- Test limit enforcement
- Test persistence
- Test state transitions

### Tool Tests (`internal/tools/python_test.go`)

- Test size limits
- Test cleanup behavior
- Test error conditions

```go
func TestWriteScript_SizeLimitRejected(t *testing.T) {
    toolsDir := t.TempDir()
    oversized := strings.Repeat("x", maxScriptBytes+1)

    _, cleanup, err := writeScript(oversized, toolsDir)
    if err == nil {
        if cleanup != nil {
            cleanup()
        }
        t.Fatal("expected error for oversized script, got nil")
    }
}
```

## Coverage Approach

### Target
No explicit coverage target enforced, but the project aims for:

> "good test coverage, especially for complex logic and critical operations" - CLAUDE.md

### Viewing Coverage

```bash
# Per-package coverage
go test -cover ./internal/config/...

# Overall coverage
go test -cover ./internal/...

# Coverage with output
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Critical Areas with Tests
- `internal/config/` - Configuration parsing and migration
- `internal/memory/` - History, plans, journals
- `internal/security/` - Vault encryption
- `internal/budget/` - Cost tracking
- `internal/agent/` - Agent loop, dispatch logic
- `internal/tools/` - Tool execution

## Integration vs Unit Tests

### Unit Tests
- **Scope:** Individual functions, methods
- **Location:** `*_test.go` files co-located with source
- **Dependencies:** Mocked or minimal
- **Examples:**
  - `TestHistoryManager_TotalChars` - Testing single method
  - `TestVaultWriteSecretPersistsAtomically` - Single operation

### Integration Tests
- **Scope:** Multiple components working together
- **Location:** Same `*_test.go` files
- **Dependencies:** Real implementations (SQLite, files)
- **Examples:**
  - `TestHistoryManager_PersistAndLoad` - Tests disk persistence
  - `TestConfigSaveWritesUpdatedField` - Tests load/save cycle

### No E2E Tests Detected
The project does not appear to have formal end-to-end tests. Testing is primarily done via:
1. Manual testing through the Web UI
2. Docker-based testing
3. Integration tests with real dependencies

## CI/CD

### GitHub Actions Workflow (`.github/workflows/docker-publish.yml`)

**Trigger:**
- Push to `main` or `master` branch
- Version tags (`v*`)
- Manual workflow dispatch

**Steps:**
```yaml
- Checkout repository
- Lowercase image name (GitHub registry requires lowercase)
- Set up QEMU (for multi-platform emulation)
- Set up Docker Buildx
- Log in to GitHub Container Registry
- Extract metadata (tags, labels)
- Build and push (platforms: linux/amd64, linux/arm64)
```

**Caching:**
- Registry-based cache for Docker layers

**Note:** The CI workflow focuses on Docker image building. Go unit tests are not run in CI based on current configuration.

### Docker Build
```bash
docker build -t aurago:latest .
docker-compose up -d
```

## Test Examples Reference

### Example Files in Project

| File | Purpose |
|------|---------|
| `internal/config/config_test.go` | Configuration loading, migration, save |
| `internal/memory/history_test.go` | Memory manager, persistence, concurrency |
| `internal/security/vault_test.go` | Vault encryption |
| `internal/budget/tracker_test.go` | Budget tracking |
| `internal/tools/python_test.go` | Python execution limits |
| `internal/agent/agent_test.go` | Agent workflow parsing |
| `internal/llm/failover_test.go` | LLM failover logic |

## Test Commands Summary

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/config/...

# Run with verbose output
go test -v ./internal/tools/...

# Run with coverage
go test -cover ./internal/...

# Run race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./internal/...

# Run specific test
go test -v -run TestHistoryManager_PersistAndLoad ./internal/memory/...
```

---

*Testing analysis: 2026-04-03*
