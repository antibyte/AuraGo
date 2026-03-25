# Python Execution Tool (`execute_python`)

Execute Python scripts in a sandboxed virtual environment.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `code` | Yes | Complete Python code to execute |
| `description` | No | Brief description of what the script does |
| `background` | No | Run as background process (default: false) |
| `vault_keys` | No | List of vault secret key names to inject as environment variables (requires `tools.python_secret_injection.enabled: true`) |
| `credential_ids` | No | List of credential UUIDs to inject as environment variables (requires `tools.python_secret_injection.enabled: true` and per-credential `allow_python` flag) |

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
