# Chapter 16: Troubleshooting

This chapter helps you diagnose and resolve common issues with AuraGo. From installation problems to runtime errors, you'll find solutions and guidance here.

---

## Table of Contents

1. [Diagnostic Approach](#diagnostic-approach)
2. [Installation Issues](#installation-issues)
3. [Connection Problems](#connection-problems)
4. [LLM/API Errors](#llmapi-errors)
5. [Memory and Performance Issues](#memory-and-performance-issues)
6. [Docker-Specific Problems](#docker-specific-problems)
7. [Reading Logs Effectively](#reading-logs-effectively)
8. [Debug Mode Usage](#debug-mode-usage)
9. [Diagnostic Commands](#diagnostic-commands)
10. [Recovery Procedures](#recovery-procedures)
11. [Getting Help](#getting-help)

---

## Diagnostic Approach

When encountering issues, follow this systematic approach:

1. **Check the logs** – Always start with `log/supervisor.log`
2. **Verify configuration** – Ensure `config.yaml` is valid
3. **Test connectivity** – Check network and API access
4. **Isolate the problem** – Disable integrations one by one
5. **Enable debug mode** – Get detailed error information

### Quick Diagnostic Checklist

```
□ Is AuraGo running? (check process/systemctl status)
□ Is the Web UI accessible? (http://localhost:8088)
□ Are logs being written? (check log/supervisor.log)
□ Is the LLM API key valid? (test with curl)
□ Is the master key set? (echo $AURAGO_MASTER_KEY)
□ Is the config.yaml valid? (check with YAML linter)
```

---

## Installation Issues

### Binary Won't Start

| Symptom | Cause | Solution |
|---------|-------|----------|
| `permission denied` | Missing execute permission | `chmod +x aurago` |
| `command not found` | Not in PATH | Use `./aurago` or add to PATH |
| `resources.dat not found` | Resource file missing | Ensure `resources.dat` is in the same directory |
| `no such file or directory` | Architecture mismatch | Download correct binary for your OS/arch |

### Missing Resources

> ⚠️ **Error:** `resources.dat not found or corrupted`

**Solution:**
```bash
# Re-download resources.dat
curl -O https://github.com/antibyte/AuraGo/releases/latest/download/resources.dat

# Or run setup again
./aurago --setup
```

### Build from Source Failures

| Error | Solution |
|-------|----------|
| `go: command not found` | Install Go 1.26.1+: [golang.org/dl](https://golang.org/dl) |
| `module not found` | Run `go mod download` |
| `CGO errors` | Ensure CGO is disabled or gcc is installed |
| `sqlite build errors` | Use `CGO_ENABLED=0` when building |

---

## Connection Problems

### Web UI Not Accessible

> ⚠️ **Symptom:** Browser shows "Connection refused" or "This site can't be reached"

**Checklist:**

1. **Verify AuraGo is running:**
   ```bash
   # Linux/macOS
   ps aux | grep aurago
   
   # Windows
   Get-Process | Where-Object { $_.Name -like "*aurago*" }
   ```

2. **Check host binding:**
   ```yaml
   # For local access only
   server:
     host: "127.0.0.1"
     port: 8088
   
   # For LAN access
   server:
     host: "0.0.0.0"
     port: 8088
   ```

3. **Check firewall:**
   ```bash
   # Linux (ufw)
   sudo ufw allow 8088/tcp
   
   # Linux (firewalld)
   sudo firewall-cmd --add-port=8088/tcp --permanent
   sudo firewall-cmd --reload
   ```

### Port Already in Use

> ⚠️ **Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using port 8088
# Linux/macOS
lsof -i :8088
sudo kill -9 <PID>

# Windows
netstat -ano | findstr :8088
taskkill /PID <PID> /F

# Or change port in config.yaml
server:
  port: 8089  # Use different port
```

### Telegram Bot Not Responding

| Check | Command/Action |
|-------|----------------|
| Token valid? | Test with: `curl https://api.telegram.org/bot<TOKEN>/getMe` |
| User ID correct? | Message @userinfobot to get your ID |
| Bot started? | Send `/start` to the bot |
| Webhook conflict? | Ensure no other bot uses same token |

### Discord Bot Issues

> ⚠️ **Bot appears offline**

1. Verify bot token in Discord Developer Portal
2. Check `gateway intents` are enabled (Server Members, Message Content)
3. Ensure bot has `Send Messages` permission in channel
4. Check `discord.enabled: true` in config.yaml

---

## LLM/API Errors

### API Key Issues

> ⚠️ **Error:** `401 Unauthorized` or `invalid_api_key`

**Solutions:**

1. **Verify API key is set:**
   ```bash
   # Check config.yaml
   cat config.yaml | grep api_key
   
   # Test API key directly
   curl -H "Authorization: Bearer YOUR_API_KEY" \
        https://openrouter.ai/api/v1/models
   ```

2. **Common provider issues:**

   | Provider | Common Issue | Solution |
   |----------|--------------|----------|
   | OpenRouter | Key not activated | Verify email, add credits |
   | OpenAI | Expired key | Generate new key in dashboard |
   | Ollama | Wrong base_url | Use `http://localhost:11434/v1` |
   | Local models | Model not loaded | `ollama pull <model>` first |

### Rate Limiting (429 Errors)

> ⚠️ **Error:** `429 Too Many Requests`

**Solutions:**

```yaml
# config.yaml - Add delay between calls
agent:
  step_delay_seconds: 2  # Wait 2 seconds between tool calls
```

Or implement circuit breaker backoff:
```yaml
circuit_breaker:
  retry_intervals:
    - 10s   # First retry after 10 seconds
    - 2m    # Second retry after 2 minutes
    - 10m   # Third retry after 10 minutes
```

### Model Not Found

> ⚠️ **Error:** `model not found` or `invalid model identifier`

**Solution:**
```bash
# List available models (OpenRouter example)
curl -H "Authorization: Bearer $API_KEY" \
     https://openrouter.ai/api/v1/models | jq '.[].id'

# Common free models to try:
# - arcee-ai/trinity-large-preview:free
# - meta-llama/llama-3.1-8b-instruct:free
# - google/gemini-2.5-flash-lite-preview-09-2025
```

### Fallback LLM Activation

If your primary LLM fails repeatedly, enable fallback:

```yaml
fallback_llm:
  enabled: true
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-BACKUP-KEY"
  model: "meta-llama/llama-3.1-8b-instruct:free"
  error_threshold: 2  # Switch after 2 consecutive errors
```

---

## Memory and Performance Issues

### High Memory Usage

> ⚠️ **Symptom:** AuraGo using excessive RAM (>2GB)

**Causes and Solutions:**

| Cause | Solution |
|-------|----------|
| Large VectorDB | Reduce `memory_compression_char_limit` |
| Too many background processes | Kill with `/stop` or restart |
| Memory leak in Python tools | Restart AuraGo, check tool code |
| Unbounded chat history | Use `/reset` to clear |

**Configuration adjustments:**
```yaml
agent:
  context_window: 8192        # Reduce context window
  memory_compression_char_limit: 25000  # Compress sooner
  max_tool_calls: 8           # Limit tool call chains

circuit_breaker:
  max_tool_calls: 15          # Hard limit
  llm_timeout_seconds: 120    # Shorter timeouts
```

### Database Locked Errors

> ⚠️ **Error:** `database is locked` (SQLite)

**Solutions:**
```bash
# 1. Check for multiple AuraGo instances
ps aux | grep aurago

# 2. Kill duplicate processes
kill -9 <PID>

# 3. If corruption suspected, restore from backup
cp data/short_term.db.backup data/short_term.db
```

### VectorDB Issues

> ⚠️ **Error:** `chromem: index corrupted` or search returns no results

**Recovery:**
```bash
# Rebuild vector database
rm -rf data/vectordb/*

# On next start, AuraGo will recreate it
# You may need to re-index knowledge files
```

### Slow Response Times

| Symptom | Cause | Solution |
|---------|-------|----------|
| Long delays before response | Slow LLM provider | Switch to faster model |
| Tool execution slow | Network requests | Enable `step_delay_seconds` |
| First message slow | Cold start / model loading | Normal for some providers |
| All operations slow | Resource exhaustion | Check CPU/memory usage |

---

## Docker-Specific Problems

### Container Won't Start

> ⚠️ **Error:** `Error response from daemon: driver failed programming external connectivity`

**Solution:**
```bash
# Port conflict - change in docker-compose.yml
ports:
  - "8089:8088"  # Map host 8089 to container 8088
```

### Web UI Not Accessible in Docker

> ⚠️ **Critical:** Must use `host: "0.0.0.0"` in Docker!

```yaml
# config.yaml - CORRECT for Docker
server:
  host: "0.0.0.0"  # Listen on all interfaces
  port: 8088
```

### Permission Denied on Docker Socket

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Or fix permissions temporarily
sudo chmod 666 /var/run/docker.sock
```

### Volume Persistence Issues

> ⚠️ **Data lost after container restart**

**Check docker-compose.yml:**
```yaml
volumes:
  aurago_data:/app/data        # Must be named volume
  aurago_workdir:/app/agent_workspace/workdir
```

**Verify volumes exist:**
```bash
docker volume ls
docker volume inspect aurago_aurago_data
```

### Python Skills Fail in Docker

> ⚠️ **Error:** `pip install failed` or `module not found`

**Solution:**
```bash
# Reset workdir volume
docker compose down
docker volume rm aurago_aurago_workdir
docker compose up -d
```

### Accessing Host Services from Docker

| Service | URL in Docker |
|---------|---------------|
| Home Assistant | `http://host.docker.internal:8123` |
| Ollama | `http://host.docker.internal:11434/v1` |
| Local API | `http://host.docker.internal:PORT` |

> 💡 **Linux users:** Add to `docker-compose.yml`:
> ```yaml
> extra_hosts:
>   - "host.docker.internal:host-gateway"
> ```

---

## Reading Logs Effectively

### Log Locations

| Platform | Log Location |
|----------|--------------|
| Native | `./log/supervisor.log` |
| Docker | `docker compose logs -f` |
| Systemd | `journalctl -u aurago -f` |

### Log Levels

AuraGo uses structured logging with these levels:

- `DEBUG` – Detailed information for debugging
- `INFO` – General operational information
- `WARN` – Warning conditions
- `ERROR` – Error conditions

### Common Log Patterns

```
# Normal startup
[INFO] AuraGo starting...
[INFO] Web UI available at http://localhost:8088
[INFO] Agent loop initialized

# LLM API call
[DEBUG] Sending request to LLM
[INFO] LLM response received

# Tool execution
[DEBUG] Dispatching tool: execute_shell
[INFO] Tool execute_shell completed

# Error pattern
[ERROR] Failed to connect to LLM API
[WARN] Retrying in 10s...
```

### Filtering Logs

```bash
# View only errors
grep ERROR log/supervisor.log

# Follow logs in real-time
tail -f log/supervisor.log

# Last 100 lines
tail -n 100 log/supervisor.log

# Search for specific tool
grep "execute_shell" log/supervisor.log
```

---

## Debug Mode Usage

### Enabling Debug Mode

**Via chat command:**
```
/debug on
```

**Effect:** Agent includes detailed error information in responses and shows tool results.

### Debug Indicators

| Indicator | Meaning |
|-----------|---------|
| 🔍 Debug badge in UI | Debug mode is active |
| Detailed error traces | Full stack traces in errors |
| Tool output visible | See raw tool responses |
| Verbose logging | More detailed log entries |

### When to Use Debug Mode

**Use debug mode when:**
- Troubleshooting tool failures
- Investigating unexpected behavior
- Reporting bugs
- Developing new tools

**Don't use debug mode when:**
- In production (verbose output)
- Sharing screenshots (may expose sensitive data)
- Performance is critical (adds overhead)

### Disabling Debug Mode

```
/debug off
```

Or toggle:
```
/debug  # Switches current state
```

---

## Diagnostic Commands

### System Information Commands

```bash
# Check if AuraGo is running
curl http://localhost:8088/api/health

# View current configuration (masked)
curl http://localhost:8088/api/config

# Check memory usage
curl http://localhost:8088/api/dashboard
```

### Chat-Based Diagnostics

| Command | Purpose |
|---------|---------|
| `/help` | List all available commands |
| `/budget` | Check API usage and costs |
| `/debug on` | Enable detailed output |
| `/reset` | Clear potentially corrupted chat state |
| `/stop` | Interrupt hanging operations |

### File System Diagnostics

```bash
# Check disk space
df -h

# Check AuraGo directory size
du -sh ~/aurago

# Verify file permissions
ls -la ~/aurago/data/

# Check for locked files
lsof +D ~/aurago/data/
```

### Network Diagnostics

```bash
# Test LLM connectivity
curl -I https://openrouter.ai/api/v1/models

# Check DNS resolution
nslookup openrouter.ai

# Test with actual API call
curl -H "Authorization: Bearer $API_KEY" \
     https://openrouter.ai/api/v1/chat/completions \
     -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hi"}]}'
```

---

## Recovery Procedures

### Emergency Restart

```bash
# 1. Stop AuraGo
# Native:
Ctrl+C
# Or: killall aurago

# Systemd:
sudo systemctl stop aurago

# Docker:
docker compose down

# 2. Start again
./aurago
# Or: sudo systemctl start aurago
# Or: docker compose up -d
```

### Reset Chat History

> 💡 **Non-destructive:** Only clears conversation, keeps memories

```
/reset
```

Or delete manually:
```bash
rm data/chat_history.json
rm data/short_term.db
```

### Vault Recovery

> ⚠️ **Critical:** Without master key, encrypted data is lost forever

**If master key is lost:**
1. Stop AuraGo
2. Delete vault: `rm data/secrets.vault`
3. Regenerate master key: `export AURAGO_MASTER_KEY=$(openssl rand -hex 32)`
4. Restart AuraGo and re-enter API keys

**Rotating master key (with old key):**
```bash
# 1. Export old key
export AURAGO_MASTER_KEY=<old_key>

# 2. Generate new key
export NEW_KEY=$(openssl rand -hex 32)

# 3. Use Web UI vault management to rotate
# Or manually migrate and re-encrypt
```

### Complete Reset (Fresh Start)

> ⚠️ **Destructive:** Deletes all data!

```bash
# Backup first
tar czf aurago_backup_$(date +%Y%m%d).tar.gz ~/aurago/data ~/aurago/config.yaml

# Reset everything
rm -rf ~/aurago/data/*
rm -rf ~/aurago/agent_workspace/workdir/*
rm -f ~/aurago/.env

# Re-run setup
./aurago --setup
```

### Database Recovery

**SQLite corruption:**
```bash
# Check database integrity
sqlite3 data/short_term.db "PRAGMA integrity_check;"

# If corrupted, restore from backup
cp data/short_term.db.backup data/short_term.db

# Or recreate (data loss)
rm data/short_term.db
# Restart AuraGo - will recreate empty
```

---

## Getting Help

### Before Asking for Help

1. ✅ Check this troubleshooting guide
2. ✅ Review logs for error messages
3. ✅ Try debug mode (`/debug on`)
4. ✅ Search existing issues
5. ✅ Test with minimal config (default settings)

### Information to Include

When reporting issues, provide:

```
- AuraGo version: (./aurago --version)
- Operating system: (uname -a)
- Installation method: (binary/Docker/source)
- LLM provider: (OpenRouter/OpenAI/Ollama)
- Relevant config.yaml sections (sanitized)
- Error messages from logs
- Steps to reproduce
```

### Community Resources

| Resource | Link | Purpose |
|----------|------|---------|
| GitHub Issues | github.com/antibyte/AuraGo/issues | Bug reports, feature requests |
| Discussions | github.com/antibyte/AuraGo/discussions | Q&A, ideas |
| Documentation | /documentation folder | Detailed guides |
| README | README.md | Quick reference |

### GitHub Issue Template

```markdown
**Description:**
Brief description of the issue

**Steps to Reproduce:**
1. Step one
2. Step two
3. ...

**Expected Behavior:**
What should happen

**Actual Behavior:**
What actually happens

**Logs:**
```
Paste relevant log entries here
```

**Environment:**
- OS: Ubuntu 22.04
- Version: v1.2.3
- Installation: Docker
- LLM: OpenRouter with gpt-4
```

### Security Issues

> ⚠️ **For security vulnerabilities, do NOT open public issues.**

Contact: [Security contact information from project]

---

## Quick Reference: Error Codes

| Error | Meaning | Action |
|-------|---------|--------|
| `E001` | Vault decryption failed | Check AURAGO_MASTER_KEY |
| `E002` | Config parse error | Validate YAML syntax |
| `E003` | Database locked | Kill duplicate processes |
| `E004` | LLM timeout | Increase timeout or check API |
| `E005` | Tool execution failed | Check tool parameters |
| `E006` | Rate limited | Wait or increase step_delay |
| `E007` | Circuit breaker open | Wait for retry interval |
| `E008` | Budget exceeded | Check /budget, reset at midnight |

---

> 💡 **Remember:** Most issues can be resolved by checking logs, verifying configuration, and ensuring network connectivity. When in doubt, restart AuraGo – it's designed to be stateless and recover gracefully.
