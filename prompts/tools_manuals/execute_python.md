# Python Execution Tool (`execute_python`)

Execute Python scripts in AuraGo's isolated Python virtual environment. This is not a security sandbox: it runs on the host with the permissions granted by `agent.allow_python`. Prefer `execute_sandbox` for untrusted code when the sandbox is available.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `code` | Yes | Complete Python code to execute |
| `description` | No | Brief description of what the script does |
| `background` | No | Run as background process (default: false) |
| `vault_keys` | No | List of vault secret key names to inject as environment variables (requires `tools.python_secret_injection.enabled: true`) |
| `credential_ids` | No | List of credential UUIDs to inject as environment variables (requires `tools.python_secret_injection.enabled: true` and per-credential `allow_python` flag) |
| `enable_tool_bridge` | No | Allow this foreground Python run to call allowlisted AuraGo tools through `import aurago` |
| `tool_bridge_call_limit` | No | Per-run limit for `aurago.call_tool` calls. Default: 10. Maximum: 50 |

## Vault Secret Injection

When `tools.python_secret_injection.enabled` is set to `true` in the config, you can request vault secrets to be injected as environment variables into the Python process.

- Pass `vault_keys` as an array of secret key names from the vault
- Each secret is available as `AURAGO_SECRET_<KEY>` (key name uppercased, special chars replaced with `_`)
- **Only user/agent-created secrets are accessible** — system and integration secrets (API keys, bot tokens, etc.) are blocked
- Secret values are automatically scrubbed from all output to prevent leaks
- After the process exits, secrets are removed (child process env is isolated)

## Credential Injection

When `tools.python_secret_injection.enabled` is set to `true`, you can also inject credentials from the Knowledge Center into the Python process.

- Pass `credential_ids` as an array of credential UUIDs
- Each credential must have the `allow_python` flag enabled — credentials without this flag are silently rejected
- Credentials are available as environment variables:
  - `AURAGO_CRED_<NAME>_USERNAME` — the username
  - `AURAGO_CRED_<NAME>_PASSWORD` — the password (for SSH and Login types)
  - `AURAGO_CRED_<NAME>_TOKEN` — the token (for Token/Key type)
- `<NAME>` is the credential name uppercased with special characters replaced by `_`
- Secret values (password, token) are automatically scrubbed from output

### Example with credential_ids

```json
{"action": "execute_python", "code": "import os\nuser = os.environ['AURAGO_CRED_MY_DATABASE_USERNAME']\npwd = os.environ['AURAGO_CRED_MY_DATABASE_PASSWORD']\nprint(f'Connected as {user}')", "credential_ids": ["a1b2c3d4-..."]}
```

### Example with vault_keys

```json
{"action": "execute_python", "code": "import os\napi_key = os.environ['AURAGO_SECRET_MY_API_KEY']\nprint('Key loaded successfully')", "vault_keys": ["my_api_key"]}
```

This also works with `execute_sandbox`, `execute_skill`, and `run_tool` (or skill manifests with `vault_keys` in the JSON).

## Tool Bridge Reentry

Use `enable_tool_bridge: true` only when a foreground Python script needs to call AuraGo tools as part of a data-processing workflow. It is not available for `background: true`.

Prerequisites in config:

```yaml
tools:
  python_tool_bridge:
    enabled: true
    allowed_tools:
      - filesystem
      - web_scraper
```

Only tools in `allowed_tools` can be called. The bridge uses the internal loopback API and the same tool dispatch rules as normal AuraGo tool calls.

Inside Python:

```python
import aurago

resp = aurago.call_tool("filesystem", {
    "operation": "read_file",
    "path": "README.md"
})
print(resp["status"])
print(resp["result"])
```

Example tool call:

```json
{"action": "execute_python", "enable_tool_bridge": true, "tool_bridge_call_limit": 5, "code": "import aurago\nresp = aurago.call_tool('filesystem', {'operation': 'read_file', 'path': 'README.md'})\nprint(resp['result'][:500])"}
```

`aurago.call_tool(tool_name, parameters=None, timeout=60)` returns a dictionary with `status` and `result`. It raises an exception when the bridge is disabled, the tool is not allowlisted, the per-run call limit is exceeded, or the tool returns an error.

## Environment
- Python 3.10+ with isolated venv in `agent_workspace/workdir/venv`
- Pre-installed: `requests`, `beautifulsoup4`, `pyyaml`, `pandas` (auto-installed on first use)
- Working directory: `agent_workspace/workdir/`
- Use `pip install` within the script if additional packages are needed

## Examples

```json
{"action": "execute_python", "code": "import json\ndata = {'key': 'value'}\nprint(json.dumps(data, indent=2))", "description": "Print formatted JSON"}
```

```json
{"action": "execute_python", "code": "import requests\nr = requests.get('https://api.github.com')\nprint(r.status_code, r.json())", "description": "Test GitHub API"}
```

```json
{"action": "execute_python", "code": "import os\nfor f in os.listdir('.'):\n    print(f)", "description": "List working directory"}
```

## Notes
- Scripts run with a timeout (configurable in config.yaml)
- stdout and stderr are captured and returned
- For persistent tools, use `save_tool` instead
- For reusable capabilities, prefer `create_skill_from_template` plus `execute_skill` verification instead of saving ad-hoc Python manually.
