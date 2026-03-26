## Tool: Skill Manager

Web UI and API for managing, uploading, and monitoring Python skills. Provides a registry of all skills with security scanning, template-based creation, and fine-grained access controls.

### Overview

The Skill Manager extends the Skills Engine with:
- **Visual management** via Web UI at `/skills`
- **Upload support** for user-created Python skills
- **Security scanning** (static analysis, optional VirusTotal + LLM Guardian)
- **Template-based creation** for scaffolding new skills quickly
- **Read-only mode** and upload toggles for safe operation

### API Endpoints

#### List Skills
`GET /api/skills?type=agent&status=clean&enabled=true&search=foo`

Returns all skills with optional filters. Response includes `skills` array and `stats` object.

| Parameter | Type | Description |
|---|---|---|
| `type` | string | Filter by type: `agent`, `user`, `builtin` |
| `status` | string | Filter by security status: `clean`, `warning`, `dangerous`, `pending` |
| `enabled` | string | Filter by enabled state: `true` or `false` |
| `search` | string | Search in name and description |

#### Get Skill Detail
`GET /api/skills/{id}?code=true`

Returns a single skill entry. Add `?code=true` to include the full source code.

#### Upload Skill
`POST /api/skills/upload`

Multipart form upload. Field name: `file`. Accepts `.py` files up to the configured max size. Automatically runs security scanning.

#### Create from Template
`POST /api/skills/templates` with JSON body:
```json
{"template": "api_client", "name": "my_skill", "description": "Fetches data from API"}
```

#### Toggle Skill
`PUT /api/skills/{id}` with JSON body:
```json
{"enabled": true}
```

#### Verify/Re-scan Skill
`POST /api/skills/{id}/verify`

Re-runs security scanning. Supports optional query params `?vt=true&guardian=true` to override config-level VirusTotal/Guardian settings.

#### Delete Skill
`DELETE /api/skills/{id}?delete_files=true`

Removes skill from registry. Add `?delete_files=true` to also delete the `.py` and `.json` files from disk.

#### Get Stats
`GET /api/skills/stats`

Returns `{total, agent, user, pending}` counts.

### Security Scanning

Every uploaded skill goes through:
1. **Static analysis** — 15 regex patterns checking for dangerous code (eval, exec, subprocess, os.system, pickle, etc.)
2. **VirusTotal** (optional) — File hash lookup if API key is configured
3. **LLM Guardian** (optional, FC#1) — AI-powered code review when `scan_with_guardian` is enabled

Security statuses: `clean`, `warning`, `dangerous`, `pending`, `error`

### Configuration

```yaml
tools:
  skill_manager:
    enabled: true              # Enable the Skill Manager
    allow_uploads: true        # Allow user uploads
    read_only: false           # Block all write operations
    require_scan: true         # Mandatory security scan before enabling
    max_upload_size_mb: 1      # Max file size for uploads
    auto_enable_clean: false   # Auto-enable skills that pass scanning
    scan_with_guardian: false   # Use LLM Guardian for code review
```

### Access Controls

- **Read-only mode** (`read_only: true`): Disables upload, delete, toggle, and template creation
- **Upload toggle** (`allow_uploads: false`): Only disables file uploads while allowing other operations
- Both are enforced server-side in all write handlers
