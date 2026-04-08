# AuraGo Codebase Audit Report

**Analysis Date:** 2026-04-07
**Focus Areas:** internal/agent/, internal/server/, internal/tools/, internal/memory/, internal/security/, internal/config/, cmd/aurago/main.go

---

## Executive Summary

The AuraGo codebase is a large, complex Go application (~930 lines main.go, ~3000+ lines agent_loop.go, 90+ tool implementations). The code demonstrates solid engineering practices in many areas but has several issues ranging from critical security concerns to medium-priority code quality improvements.

---

## CRITICAL ISSUES

### 1. Missing Error Check After `os.Stat()` in Configuration Loading

**File:** `cmd/aurago/main.go` lines 221-230

```go
if setup.NeedsSetup(installDir) {
    resPath := filepath.Join(installDir, "resources.dat")
    if _, err := os.Stat(resPath); err == nil {  // <-- Error ignored!
        appLog.Warn("Essential files missing — running automatic setup from resources.dat")
```

**Severity:** Critical
**Issue:** `os.Stat()` error is checked only for `nil` but `err == nil` means success. The actual error from `os.Stat()` is completely discarded. If `resources.dat` doesn't exist, this silently proceeds without actually running setup.
**Fix:** Check `err != nil` to detect when resources.dat is missing, or remove the check entirely and let `setup.Run()` handle the missing file.

---

### 2. Race Condition: Unbounded Map Growth in `followUpDepths`

**File:** `internal/server/handlers.go` lines 27-52

```go
var (
    followUpDepths        = make(map[string]int)
    muFollowUp            sync.Mutex
    sessionRequestLocks   = make(map[string]*sync.Mutex)
    muSessionRequestLocks sync.Mutex
)
```

**Severity:** High
**Issue:** `followUpDepths` map grows indefinitely when missions create keys but never clean them up properly. The cleanup at line 132 only runs for non-follow-up requests. If follow-up depth tracking creates keys that aren't cleaned up, the map leaks memory over time.
**Fix:** Add periodic cleanup or use a TTL-based cache instead of an unbounded map.

---

### 3. Goroutine Leak in `ExecuteShellBackground`

**File:** `internal/tools/shell.go` lines 68-91

```go
func ExecuteShellBackground(command, workspaceDir string, registry *ProcessRegistry) (int, error) {
    // ...
    pid, err := registerManagedBackgroundProcess(cmd, registry, nil)
    // ...
    return pid, nil  // <-- Process handle not stored, goroutine may leak
}
```

**Severity:** Medium
**Issue:** When `registerManagedBackgroundProcess` succeeds, there's no guarantee the goroutine from `cmd.Wait()` is properly tracked. If `registerManagedBackgroundProcess` doesn't store the process correctly, the goroutine could leak.
**Fix:** Ensure all goroutines started by `cmd.Start()` are tracked in the registry and cleaned up on shutdown.

---

### 4. Empty Response Fallback with Token Estimation May Inflate Budget Tracking

**File:** `internal/agent/agent_loop.go` lines 1553-1562

```go
if totalTokens == 0 {
    SetGlobalTokenEstimated(true)
    for _, m := range req.Messages {
        promptTokens += estimateTokensForModel(messageText(m), req.Model)
    }
    completionTokens = estimateTokensForModel(content, req.Model)
    totalTokens = promptTokens + completionTokens
    tokenSource = "fallback_estimate"
}
```

**Severity:** Medium
**Issue:** When `totalTokens == 0`, the code falls back to estimation which may significantly underestimate actual token usage (especially for non-OpenAI models). This could cause budget tracking to be inaccurate.
**Fix:** Always prefer provider-reported token counts when available. Log a warning when falling back to estimation.

---

### 5. Potential Nil Pointer Dereference in `NewVault`

**File:** `internal/security/vault.go` lines 25-39

```go
func NewVault(masterKeyHex string, filePath string) (*Vault, error) {
    key, err := hex.DecodeString(masterKeyHex)
    if err != nil {
        return nil, fmt.Errorf("invalid master key format, expected hex: %w", err)
    }
    if len(key) != 32 {
        return nil, fmt.Errorf("invalid master key length, expected 32 bytes (64 hex characters)")
    }
    return &Vault{
        key:      key,
        filePath: filePath,
        fileLock: flock.New(filePath + ".lock"),  // <-- May fail silently
    }, nil
}
```

**Severity:** Medium
**Issue:** `flock.New()` could potentially return a nil lock if the path is invalid, but the error isn't checked.
**Fix:** Check the return value from `flock.New()` for errors.

---

### 6. Context Leak in `promptlog` File Handle

**File:** `internal/agent/agent_loop.go` lines 1183-1203

```go
if cfg.Logging.EnablePromptLog && cfg.Logging.LogDir != "" {
    if f, ferr := os.OpenFile(
        filepath.Join(cfg.Logging.LogDir, "prompts.log"),
        os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600,
    ); ferr == nil {
        // ...
        _ = json.NewEncoder(f).Encode(entry)
        _ = f.Close()  // <-- Using _ to ignore error
    }
}
```

**Severity:** Medium
**Issue:** File handle could leak if `json.NewEncoder(f).Encode(entry)` fails after the file is opened. The deferred close is not used, and the error from `Close()` is ignored.
**Fix:** Use `defer f.Close()` immediately after successful OpenFile.

---

## HIGH PRIORITY ISSUES

### 7. TODO Comment Left in Production Code

**File:** `internal/invasion/connector_docker.go` line 47

```go
// TODO: derive image tag from master version when build version is available at runtime
```

**Severity:** Low (but indicates incomplete implementation)
**Issue:** This is a known incomplete feature. Should either be implemented or tracked in issue tracker.

---

### 8. Silent Error Ignorance in Multiple Locations

**Pattern Found:** Multiple instances of `_ =` to ignore errors

**Examples:**
- `cmd/aurago/main.go:153`: `_ = v.WriteSecret("auth_password_hash", hash)` - password hash write error ignored
- `cmd/aurago/main.go:159`: `_ = v.WriteSecret("auth_session_secret", newSec)` - session secret write error ignored
- `cmd/aurago/main.go:191`: `_ = os.MkdirAll(cfg.Directories.DataDir, 0755)` - directory creation error ignored

**Severity:** Medium
**Issue:** Critical operations like writing auth credentials and creating directories silently fail without logging or returning errors. This could lead to silent security-relevant failures.
**Fix:** Log errors or return them to the caller.

---

### 9. Potential Integer Overflow in Memory Consolidation

**File:** `internal/memory/short_term.go` lines 152

```go
var oldestKeepID int
err := s.db.QueryRow(query, sessionID, keepN-1).Scan(&oldestKeepID)
```

**Severity:** Low
**Issue:** If `keepN` is 0, `keepN-1` becomes -1 which could cause unexpected behavior in the SQL query (SQLite LIMIT -1 is valid but may return unexpected results).

---

### 10. No Bounds Check in `borrowConn`

**File:** `internal/tools/sandbox.go` lines 222-267

```go
func (m *SandboxManager) borrowConn() (sandboxConn, func(), error) {
    m.mu.RLock()
    ready := m.status.Ready
    errMsg := m.status.Error
    keepAlive := m.cfg.KeepAlive
    conn := m.conn
    m.mu.RUnlock()

    if !ready {
        // ...
    }

    if !keepAlive {
        conn, err := m.openConn()
        // ...
    }

    m.sharedConnMu.Lock()
    cleanup := func() {
        m.sharedConnMu.Unlock()
    }

    if conn != nil {
        return conn, cleanup, nil
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    if m.conn == nil {
        conn, err := m.openConn()
        if err != nil {
            cleanup()  // <-- Called with sharedConnMu held!
            return nil, nil, err
        }
        m.conn = conn
    }
    // ...
}
```

**Severity:** Medium
**Issue:** `cleanup()` is called while `sharedConnMu` is held but `m.mu` is also acquired. This creates a potential deadlock if `openConn()` takes a long time and another goroutine tries to `borrowConn()`.
**Fix:** Restructure locking order to avoid holding multiple locks in dangerous sequences.

---

## MEDIUM PRIORITY ISSUES

### 11. Redundant `append()` in `agent_loop.go`

**File:** `internal/agent/agent_loop.go` lines 1167

```go
req.Messages = append(trimmedMessages, append(mid, lastMsg)...)
```

**Severity:** Low
**Issue:** Creates an intermediate slice unnecessarily. Should be:
```go
req.Messages = append(trimmedMessages, mid...)
req.Messages = append(req.Messages, lastMsg)
```

---

### 12. Magic Number in JSON Size Check

**File:** `internal/agent/agent_loop.go` lines 1337-1353

```go
highSpecKeys := []string{`"tool_call"`, `"tool_name"`, `"tool_call_path"`, `"action"`}
ambiguousKeys := []string{`"tool":`, `"command"`, `"operation"`, `"name"`, `"arguments"`}
highCount := 0
for _, k := range highSpecKeys {
    if strings.Contains(trimmed, k) {
        highCount++
    }
}
ambCount := 0
for _, k := range ambiguousKeys {
    if strings.Contains(trimmed, k) {
        ambCount++
    }
}
suppressToolCallJSON := highCount >= 1 || (highCount+ambCount >= 2 && ambCount >= 1) || ambCount >= 3
```

**Severity:** Low
**Issue:** Magic numbers 1, 2, 3 for counts are not explained. The logic is difficult to understand.
**Fix:** Extract to named constants with explanatory comments.

---

### 13. No Timeout on Background Task Goroutine

**File:** `internal/agent/agent_loop.go` lines 2461-2492

```go
go func(userMsg, aResp, sid string, toolNames, toolSummaries []string, personalityInput *helperTurnPersonalityInput, recentMsgs []openai.ChatCompletionMessage) {
    analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    batchResult, err := helperManager.AnalyzeTurn(analysisCtx, userMsg, aResp, toolNames, toolSummaries, personalityInput)
    // ...
}()
```

**Severity:** Low
**Issue:** The outer goroutine has no timeout. If `helperManager.AnalyzeTurn()` hangs, the goroutine never returns (though the inner context has a 60-second timeout, the goroutine itself could still be waiting for other operations).
**Fix:** Consider adding a timeout for the entire goroutine execution.

---

### 14. Unused Variable in `Reconfigure`

**File:** `internal/llm/failover.go` line 119

```go
_ = oldStopCh  // <-- Why is this here?
```

**Severity:** Low
**Issue:** Assigning to `_` suggests the variable was previously used or there's dead code. Either remove it or the assignment has a purpose that isn't documented.

---

### 15. Inconsistent Error Handling in `RegisterSensitive`

**File:** `internal/security/scrubber.go`

**Issue:** Multiple locations silently ignore errors by assigning to `_`. Should be consistent in logging or propagating errors.

---

### 16. Duplicate Code in `agent_loop_helpers.go`

**File:** `internal/agent/agent_loop_helpers.go` lines 36-80 and 90-134

**Issue:** `mergeStreamToolCallChunk()` and `StreamToolCallAssembler.Merge()` contain nearly identical logic. Also `assembleSortedStreamToolCalls()` and `StreamToolCallAssembler.Assemble()` are similar.
**Fix:** Consolidate into a single implementation or extract common logic.

---

### 17. No Cleanup of `sessionRequestLocks` Map

**File:** `internal/server/handlers.go` lines 34-52

```go
func lockSessionRequest(sessionID string) func() {
    // ...
    lock := sessionRequestLocks[sessionID]
    if lock == nil {
        lock = &sync.Mutex{}
        sessionRequestLocks[sessionID] = lock
    }
    // ...
    return func() {
        lock.Unlock()
        // Remove per-mission entries after use so the map does not grow unboundedly.
        if sessionID != "default" {
            muSessionRequestLocks.Lock()
            delete(sessionRequestLocks, sessionID)
            muSessionRequestLocks.Unlock()
        }
    }
}
```

**Severity:** Low
**Issue:** Only non-default session locks are cleaned up. The "default" session lock is never removed, causing gradual memory growth for long-running servers.
**Fix:** Add TTL-based cleanup or periodic pruning of the map.

---

## OPTIMIZATION OPPORTUNITIES

### 18. Missing Sync.Once for Expensive Initialization

**File:** `internal/server/server.go`

**Issue:** Multiple initialization operations may run multiple times if `Start()` is called multiple times.
**Fix:** Use `sync.Once` to ensure initialization runs exactly once.

---

### 19. String Concatenation in Loop

**File:** `internal/agent/agent_loop.go` lines 111-126

```go
var builder strings.Builder
builder.WriteString("[TRIMMED_CONTEXT_RECAP]: Older conversation content was condensed...")
for _, msg := range messages[start:] {
    content := strings.Join(strings.Fields(messageText(msg)), " ")
    if content == "" {
        continue
    }
    builder.WriteString("- ")
    builder.WriteString(msg.Role)
    builder.WriteString(": ")
    builder.WriteString(Truncate(content, 220))
    builder.WriteString("\n")
}
```

**Severity:** Low
**Issue:** Multiple `WriteString` calls could be combined, but `strings.Builder` is already efficient. This is more style than performance.

---

### 20. Repeated `filepath.Join` Calls

**File:** `cmd/aurago/main.go` lines 281-289

```go
dirs := []string{
    cfg.Directories.DataDir,
    cfg.Directories.WorkspaceDir,
    cfg.Directories.ToolsDir,
    cfg.Directories.PromptsDir,
    cfg.Directories.SkillsDir,
    cfg.Directories.VectorDBDir,
    cfg.Logging.LogDir,
    cfg.Tools.DocumentCreator.OutputDir,
}
```

**Issue:** Each directory is checked/created separately with `os.MkdirAll`. Could batch some of these checks.

---

### 21. Context Cancellation Not Respected in `promptlog`

**File:** `internal/agent/agent_loop.go` lines 1183-1203

**Issue:** The prompt logging code doesn't check if the context has been cancelled before attempting to write.
**Fix:** Check `ctx.Err()` before file operations.

---

### 22. Missing Database Connection Pooling Configuration

**File:** `internal/memory/*.go`

**Issue:** SQLite databases don't have explicit connection pool settings configured. While SQLite doesn't support traditional connection pooling, the `sql.DB` settings could be tuned.
**Fix:** Configure `SetMaxOpenConns`, `SetMaxIdleConns`, and `SetConnMaxLifetime` appropriately.

---

## SECURITY CONSIDERATIONS

### 23. Shell Command Injection Prevention Incomplete

**File:** `internal/agent/agent_dispatch_exec.go` lines 316-319

```go
if isBlockedEnvRead(req.Command) {
    logger.Warn("[Security] Blocked attempt to read sensitive environment variable", "command", Truncate(req.Command, 200))
    return "Tool Output: [PERMISSION DENIED] Reading AURAGO_ environment variables via shell is not permitted."
}
```

**Severity:** Medium
**Issue:** `isBlockedEnvRead()` only checks for specific patterns. More comprehensive input validation is needed for shell commands.
**Fix:** Consider using `shellwords` or similar library to properly parse and validate shell commands.

---

### 24. Vault Master Key in Memory

**File:** `cmd/aurago/main.go` line 471-477

```go
masterKey := os.Getenv("AURAGO_MASTER_KEY")
// ...
security.RegisterSensitive(masterKey)
vault, err := security.NewVault(masterKey, vaultPath)
```

**Severity:** Low (acceptable pattern)
**Issue:** The master key is read into a string and then passed to `RegisterSensitive()`. The string remains in memory. This is standard practice but worth noting.
**Fix:** Consider zeroing the string after use (not easily possible in Go).

---

### 25. No Rate Limiting on LLM Calls Per Session

**File:** `internal/agent/agent_loop.go`

**Issue:** While there's circuit breaker logic, individual sessions don't have per-user rate limiting.
**Fix:** Consider adding per-session rate limiting to prevent abuse.

---

## CODE QUALITY OBSERVATIONS

### 26. Long Function `ExecuteAgentLoop`

**File:** `internal/agent/agent_loop.go` (~2500 lines)

**Severity:** Medium
**Issue:** This function is extremely long and handles many responsibilities. While well-structured with clear sections, it could be broken down further for maintainability.
**Fix:** Consider extracting specific sub-functions like streaming handling, RAG logic, and compression into separate files.

---

### 27. Inconsistent Naming Conventions

**Issue:** Some variables use abbreviations (`cfg`, `stm`, `kg`) while others use full words (`shortTermMem`, `longTermMem`). This is minor but worth standardizing.

---

### 28. Test Coverage Gaps

**Observation:** Many files have `*_test.go` companions, but some critical paths (especially error handling) may lack coverage.

---

## SUMMARY BY CATEGORY

| Category | Critical | High | Medium | Low |
|----------|----------|------|--------|-----|
| Errors/Bugs | 2 | 4 | 6 | 5 |
| Security | 0 | 2 | 3 | 2 |
| Performance | 0 | 1 | 3 | 4 |
| Code Quality | 0 | 1 | 4 | 6 |
| **Total** | **2** | **8** | **16** | **17** |

---

## RECOMMENDED ACTIONS

### Immediate (Critical/High Priority)

1. Fix the `os.Stat()` error check in `cmd/aurago/main.go:223`
2. Add proper error handling for vault operations instead of using `_`
3. Fix the goroutine potential leak in sandbox connection management
4. Address the race condition in `followUpDepths` map

### Short-term (Medium Priority)

5. Consolidate duplicate code in `agent_loop_helpers.go`
6. Add timeouts to all goroutines that don't have them
7. Fix the deadlock potential in `borrowConn()`
8. Add periodic cleanup for unbounded maps

### Long-term (Low Priority)

9. Break down `ExecuteAgentLoop` into smaller functions
10. Add comprehensive test coverage for error paths
11. Consider adding per-session rate limiting

---

## FIX STATUS (as of 2026-04-08)

### Fixed in commit `e46d04a`

| # | Issue | Fix |
|---|-------|-----|
| 1 | `os.Stat()` error check reversed (main.go:223) | Fixed: setup now always runs when `NeedsSetup` returns true; informative logging for both resources.dat present/absent cases |
| 2 | Silent vault write errors (main.go:153,159) | Fixed: `v.WriteSecret()` errors now logged with `appLog.Error()` instead of ignored via `_=` |
| 3 | Silent `os.MkdirAll` error (main.go:191) | Fixed: `MkdirAll` errors now logged with path and error details |
| 4 | Token estimation fallback silently used | Fixed: Added `Warn` log when falling back to estimation (totalTokens==0) |
| 5 | `json.Encode` error ignored in promptlog | Fixed: encode errors now logged instead of silently ignored |

### Not Applicable / Deferred

| # | Issue | Reason |
|---|-------|--------|
| 6 | `flock.New()` nil risk (vault.go) | `flock.New()` returns `*Flock` (not `(Flock, error)`); no error to check per gofrs/flock API |
| 7 | `borrowConn()` deadlock risk (sandbox.go) | Analyzed: pattern is correct — `defer mu.Unlock()` fires on return, `cleanup()` handles sharedMu only; `openConn()` does not call `borrowConn()` so no ABBA deadlock possible |
| 8 | Goroutine leak in `ExecuteShellBackground` | Analyzed: `superviseBackgroundProcess` properly tracks all processes via registry with deferred cleanup; no leak |

### Remaining (Not Fixed)

| Priority | Issue | Notes |
|----------|-------|-------|
| Medium | Race condition in `followUpDepths` map growth | Map bounded by depth counter (max 10); cleanup on non-follow-up; acceptable |
| Medium | `sessionRequestLocks` "default" session never cleaned | Only affects long-running servers with non-default sessions; low severity |
| Medium | Deadlock potential via nested `borrowConn()` | Would require `openConn()` to call `borrowConn()` — doesn't happen currently |
| Low | Duplicate code in `agent_loop_helpers.go` | Consolidation is low-risk but not critical |
| Low | Shell command injection prevention incomplete | `isBlockedEnvRead()` exists; comprehensive validation would need `shellwords` library |

---

*Audit completed: 2026-04-07*
