# Codebase Concerns

**Analysis Date:** 2026-04-03

## Security Considerations

### Vault and Master Key

**Risk:** Master key compromise leads to total vault compromise.
- Location: `internal/security/vault.go`
- The vault uses AES-256-GCM encryption with a 64-character hex master key
- Master key loaded from environment variables, Docker secrets (`/run/secrets/aurago_master_key`), or `.env` files
- If the master key is exposed, all encrypted credentials (database passwords, API keys, tokens) are compromised
- **No key rotation mechanism** - if compromised, vault data must be re-encrypted manually

**Risk:** Master key used directly without stretching.
- The master key is used directly as encryption key (not derived via KDF like Argon2 or scrypt)
- This means weak master keys are vulnerable to brute-force attacks

### Prompt Injection Defense (Guardian)

**Risk:** Regex-based injection detection can be bypassed.
- Location: `internal/security/guardian.go`
- Guardian uses ~30+ regex patterns for injection detection (English, German, CJK)
- Regex patterns may miss novel obfuscation techniques (zero-width spaces, nested encoding, etc.)
- The `max_scan_bytes` is 16KB with edge truncation - long payloads could hide malicious content

**Risk:** User input scanning is logging-only.
- `ScanUserInput()` in `internal/security/guardian.go:354` logs warnings but does NOT block
- Only LLM Guardian can block, and it is disabled by default (`llm_guardian.enabled: false`)

**Risk:** External data isolation via tags can be escaped.
- `IsolateExternalData()` wraps content in `<external_data>` tags
- Existing tags are escaped via string replacement, not proper HTML entity encoding
- Could potentially be bypassed with careful payload construction

### LLM Guardian

**Risk:** LLM-based security evaluation adds latency and cost.
- Location: `internal/security/llm_guardian.go`
- When enabled, every tool call may trigger an additional LLM call for security evaluation
- Rate limiting via semaphore (default 60 checks/minute) could cause delays
- Fail-safe defaults to "quarantine" if LLM fails - could block legitimate operations

**Risk:** Guardian prompt injection.
- The guardian prompt itself (`guardianSystemPrompt`) could potentially be manipulated
- Agent's justification is passed to `EvaluateClarification()` - if the agent is compromised, it could craft convincing justifications

### SSRF Protection

**Risk:** Tailscale bypasses SSRF protection.
- Location: `internal/security/ssrf.go:25` comment
- Tailscale VPN IPs (100.64.0.0/10) are blocked for `api_request`
- BUT: "the Tailscale tool uses its own dedicated HTTP client" - not subject to SSRF validation
- This is intentional but creates an attack vector if Tailscale credentials are compromised

**Risk:** DNS rebinding attacks.
- SSRF validation happens at request time but DNS can change between validation and connection
- `NewSSRFProtectedHTTPClientForURL()` pins the resolved IP, but redirect following re-validates

### Danger Zone Capabilities

**Risk:** Dangerous capabilities are toggled by config, not per-request.
- Location: `config_template.yaml` lines 27-36
- `allow_shell`, `allow_python`, `allow_filesystem_write`, `allow_network_requests`, `allow_remote_shell`, `allow_self_update`
- When enabled, the agent can execute arbitrary shell commands, write files, make network requests
- No per-request authorization - once enabled, any agent action can use these capabilities

**Risk:** `python_secret_injection` gives Python direct vault access.
- Location: `internal/config/config_types.go:1071`
- When enabled (`tools.python_secret_injection.enabled: true`), Python skills can request vault secrets via `vault_keys` parameter
- Default is disabled, but if enabled, Python code has direct access to all vault secrets
- Python code uploaded by users (if `allow_uploads: true`) could exfiltrate secrets

### Skill Manager Upload

**Risk:** User-uploaded Python skills execute with agent permissions.
- Location: `internal/tools/skill_security.go`
- `allow_uploads: true` allows users to upload Python skills to `agent_workspace/skills/`
- `require_scan: true` performs static analysis, but `scan_with_guardian: false` by default
- Static patterns detect `eval()`, `exec()`, `subprocess.shell=True`, `os.system()`, etc.
- Regex-based detection can be bypassed with obfuscation (e.g., `getattr(__builtins__,'eval')`)

**Risk:** VirusTotal integration is optional.
- `scan_with_guardian: false` means LLM-based code review is not used by default
- VirusTotal requires API key and is not free for heavy use

### Form Automation (Headless Browser)

**Risk:** Headless browser runs without sandbox.
- Location: `internal/tools/form_automation.go:73`
- `NoSandbox(true)` is hardcoded - browser runs with full privileges
- When enabled, the agent can interact with any web form (login forms, etc.)
- Could be used for credential harvesting or CSRF attacks

**Risk:** No URL validation against SSRF.
- `form_automation` validates scheme (http/https) but does not check for private IP ranges
- Could access internal web interfaces (router admin panels, etc.)

### Credential Scrubber

**Risk:** Scrubber uses global mutable state.
- Location: `internal/security/scrubber.go:24-26`
- `sensitiveValues` is a slice protected by `sync.RWMutex`
- `RegisterSensitive()` appends without bounds checking
- Pattern matching could miss edge cases in encoding (double encoding, etc.)

---

## Scalability Limitations

### Single-Writer Database Architecture

**Risk:** SQLite write concurrency is limited.
- 9+ separate SQLite databases: `short_term.db`, `long_term.db`, `inventory.db`, `invasion.db`, `cheatsheets.db`, `image_gallery.db`, `media_registry.db`, `homepage_registry.db`, `contacts.db`, `site_monitor.db`, `sql_connections.db`
- SQLite allows only one writer at a time per database
- High write throughput (e.g., from multiple concurrent agents) will bottleneck

**Risk:** Vector DB (chromem-go) is embedded and single-instance.
- Location: `internal/memory/` - embedded vector database
- No horizontal scaling for memory retrieval
- `memory_compression_char_limit: 60000` limits context but memory growth is unbounded

### Concurrency Limits

**Risk:** Agent loop concurrency limiter is bounded.
- Location: `internal/agent/agent_loop.go:86`
- `maxConcurrentAgentLoops = 8` - only 8 concurrent agent loops
- Background tasks queue when limit reached (`queue_when_busy: true`)

**Risk:** Global token counting uses simple mutex.
- Location: `internal/agent/agent.go:92`
- `muTokens sync.Mutex` protects `GlobalTokenCount`
- This is a global lock that could become a contention point

### Large File Processing

**Risk:** Large tool output truncation.
- Location: `config_template.yaml:19`
- `tool_output_limit: 50000` characters per tool result
- Very large command outputs (e.g., `ls -R`, `find`, large log files) will be truncated
- Truncation could hide important information or error messages

---

## Maintenance Concerns

### Dependency Version Risks

**Risk:** Several dependencies are significantly outdated.

| Package | Version | Risk |
|---------|---------|------|
| `discordgo` | v0.29.0 | Discord API changes may break integration; security patches may be missing |
| `go-openai` | v1.41.2 | Older version; newer API features and bug fixes missed |
| `github.com/gofrs/flock` | v0.13.0 | File locking may have platform-specific issues |
| `tailscale.com` | v1.96.1 | VPN integration; security patches critical |

**Action:** Run `go get -u` periodically to check for updates, but test thoroughly before upgrading major versions.

### Go Version Requirement

**Risk:** Go 1.26.1+ requirement excludes older systems.
- Location: `go.mod:3`
- Go 1.26.1 released January 2025 - relatively recent
- Some LTS Linux distributions may ship older Go versions
- Binary distribution mitigates this for end users

### Sandboxing Platform Limitations

**Risk:** Landlock sandbox only works on Linux.
- Location: `internal/sandbox/sandbox.go`
- Landlock LSM requires Linux 5.13+ (released 2021)
- Windows and macOS fall back to unsandboxed execution with warning
- `in_docker: true` also disables sandboxing
- This means majority of non-Linux deployments have no shell command sandboxing

### File Size Concerns

**Risk:** Several files are extremely large and hard to maintain.

| File | Lines | Concern |
|------|-------|---------|
| `internal/agent/agent_loop.go` | ~2397 | Core agent loop - extremely large for single file |
| `internal/agent/agent_dispatch_comm.go` | ~1962 | Communication dispatching |
| `internal/agent/agent_dispatch_exec.go` | ~1743 | Execution dispatching |
| `internal/memory/graph_sqlite.go` | ~1872 | Knowledge graph implementation |
| `internal/tools/homepage.go` | ~1495 | Homepage tool implementation |

Large files are harder to:
- Review for security issues
- Refactor without breaking functionality
- Test comprehensively
- Onboard new developers

### Build Complexity

**Risk:** Cross-platform builds require multiple scripts.
- Linux/macOS: `make_deploy.sh`
- Windows: `make_release.bat`
- Docker: separate `Dockerfile` and `docker-compose.yml`
- Inconsistent build tooling across platforms could cause release issues

---

## Operational Risks

### File Locking on Windows

**Risk:** `gofrs/flock` may have issues on Windows.
- Location: `cmd/aurago/main.go` and various lock usages
- File-based locking behavior differs between platforms
- NFS shares or certain filesystem types may not support proper locking
- Lock file cleanup on crash is not guaranteed

### Background Task Queue

**Risk:** Unbounded task queue growth.
- Location: `config_template.yaml:49-56`
- `background_tasks.enabled: true` queues follow_up, cron, and wait events
- If LLM is slow or tasks fail repeatedly, queue could grow unbounded
- `max_retries: 2` limits retries but not queue size

**Risk:** Cron tasks run on single instance.
- No distributed cron - if running multiple instances, cron tasks run multiple times
- `invasion_control` is designed for distributed execution but other cron tasks are not

### Global Mutable State

**Risk:** Multiple global variables in agent package.
- Location: `internal/agent/agent.go:89-102`
- `GlobalTokenCount`, `sessionInterrupts`, `debugModeEnabled`, `voiceModeEnabled`
- All protected by individual mutexes, but state is inherently global
- Makes testing harder and state management complex

### Database Migrations

**Risk:** No formal migration system.
- SQLite migrations are "handled automatically on startup" (per docs)
- No version tracking for schema changes
- Backward compatibility is assumed but not enforced
- Could lose data if migration logic has bugs

### Circuit Breaker Complexity

**Risk:** Multiple circuit breaker implementations.
- LLM calls: `circuit_breaker.llm_timeout_seconds: 600`
- Tool calls: `circuit_breaker.max_tool_calls: 20`
- Co-agents: separate circuit breaker config
- Recovery policies: `retry_intervals: [10s, 2m, 10m]`
- Hard to debug when circuits open unexpectedly

---

## Technical Debt Areas

### Unprotected External Network Access

**Risk:** Tailscale HTTP client bypasses all security controls.
- Location: `internal/security/ssrf.go:25`
- Tailscale tool has its own HTTP client that doesn't go through SSRF validation
- This is documented but creates asymmetric security posture

**Risk:** Cloudflare Tunnel has its own HTTP handling.
- Location: `internal/tools/cloudflare_tunnel.go`
- tunnel traffic flows through cloudflared, not through SSRF-protected client

### Incomplete Error Handling

**Risk:** Some functions return nil without error on failure paths.
- Found via grep: multiple `return nil` statements in error conditions
- Silent failures could mask underlying problems
- Examples in `cmd/lifeboat/main.go` at lines 285, 352, 366, 383, 410

### Skill Security Static Analysis Limitations

**Risk:** Regex-based Python analysis is easily bypassed.
- Location: `internal/tools/skill_security.go:54-100`
- Patterns like `\beval\s*\(` miss obfuscated calls like:
  - `getattr(__builtins__,'eval')`
  - `exec("".join(['e','val']))`
  - Base64-encoded payloads

### Coherent Memory System Complexity

**Risk:** Memory system has many interacting components.
- Short-term memory (SQLite sliding window)
- Long-term memory (Vector DB with semantic search)
- Knowledge graph (entity-relationship store)
- Core memory (permanent facts)
- Consolidation/archival system
- `memory_analysis_rollout.go`, `memory_automation.go`, `memory_conflicts.go`, `memory_effectiveness.go`, `memory_priority.go`, `memory_ranking.go`, `memory_retrieval_policy.go`
- Complex interactions between these systems are hard to debug

### Missing Telemetry in Production

**Risk:** Telemetry collection may expose sensitive data.
- Location: `internal/agent/telemetry.go`, `internal/agent/telemetry_scope.go`
- `telemetry_capture: true` in config
- If enabled without proper filtering, could capture sensitive tool parameters or outputs

---

## Fragile Code Areas

### Agent Loop Parsing

**Area:** `internal/agent/tool_call_pipeline.go`
- Handles multiple JSON parsing strategies (native, reasoning_clean_json, content_json)
- Complex fallback logic for handling LLM responses
- `ParseToolCall()` and `extractExtraToolCalls()` are critical path

### Tool Argument Decoding

**Area:** `internal/agent/tool_args_*.go` (8 files)
- Multiple files handle different tool argument types
- `tool_args_comm.go`, `tool_args_extended.go`, `tool_args_infra.go`, `tool_args_services.go`, etc.
- Changes to tool schemas require updates in multiple files

### Dynamic Prompt Building

**Area:** `internal/prompts/builder_modules.go`
- Dynamic system prompt construction with many conditional flags
- Tool availability determined at runtime based on config
- Hard to test all combinations

---

## Test Coverage Gaps

**Noted Gaps:**

1. **No E2E tests** - Only unit tests and integration tests found
2. **Security tests are minimal** - `guardian_test.go`, `llm_guardian_test.go`, `ssrf_test.go` exist but coverage unclear
3. **Concurrent access testing** - No stress tests for multi-user scenarios
4. **Migration testing** - No tests for SQLite schema migration edge cases
5. **Skill security testing** - `skill_validation_test.go` exists but may not cover obfuscation bypasses

---

## Recommendations Summary

### High Priority

1. **Key Rotation Mechanism** - Add vault key rotation to mitigate master key compromise risk
2. **Skill Upload Sandboxing** - Run uploaded Python skills in strict sandbox environment
3. **Form Automation Sandbox** - Remove `NoSandbox(true)` or add proper Chromium sandbox flags
4. **LLM Guardian by Default** - Enable LLM Guardian at medium level by default for better protection

### Medium Priority

1. **Large File Refactoring** - Break `agent_loop.go` into smaller, focused modules
2. **Dependency Updates** - Schedule regular dependency update reviews
3. **Database Write Concurrency** - Consider if SQLite is sufficient or if PostgreSQL needed
4. **Windows Sandboxing** - Investigate alternative sandboxing for Windows (AppContainer, etc.)

### Low Priority

1. **Test Coverage Expansion** - Add E2E tests for critical user flows
2. **Telemetry Audit** - Review what data is captured in telemetry
3. **Circuit Breaker Unification** - Single circuit breaker framework across all components

---

*Concerns audit: 2026-04-03*
