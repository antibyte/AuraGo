# Python Execution Tool (`execute_python`)

Execute Python scripts in a sandboxed virtual environment.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `code` | Yes | Complete Python code to execute |
| `description` | No | Brief description of what the script does |

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
