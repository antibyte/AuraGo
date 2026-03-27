# Chapter 19: Skills

Create reusable Python skills to extend AuraGo's capabilities.

---

## What are Skills?

Skills are custom Python scripts that are made available as extended tools in the agent. They complement the built-in tools and are ideal for:

- **Recurring tasks** with complex logic
- **API integrations** with authentication
- **Data processing** and transformation
- **Web scraping** with specific requirements
- **Connections to external services**

---

## Skill Architecture

A skill consists of two files:

```
agent_workspace/skills/
├── my_skill.json       # Manifest (description, parameters)
└── my_skill.py         # Python code (implementation)
```

### The Manifest (JSON)

The manifest describes the skill for the agent:

```json
{
  "name": "weather_query",
  "description": "Fetches current weather data for a city",
  "parameters": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "Name of the city"
      },
      "unit": {
        "type": "string",
        "enum": ["celsius", "fahrenheit"],
        "default": "celsius"
      }
    },
    "required": ["city"]
  },
  "returns": {
    "type": "object",
    "description": "Temperature, humidity and weather description"
  },
  "entry_point": "weather_query.py",
  "function": "main",
  "dependencies": ["requests"],
  "vault_keys": ["openweather_api_key"]
}
```

**Fields explained:**

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Unique name of the skill | Yes |
| `description` | What the skill does (visible to the agent) | Yes |
| `parameters` | JSON schema of input parameters | Yes |
| `returns` | Description of return values | No |
| `entry_point` | Python file to execute | Yes |
| `function` | Function name in Python script (usually `main`) | Yes |
| `dependencies` | List of pip packages | No |
| `vault_keys` | List of vault secret names | No |
| `credential_ids` | List of credential IDs | No |

### The Python Code

The Python code contains the actual logic:

```python
#!/usr/bin/env python3
"""
Skill: Weather Query
Fetches current weather data from OpenWeatherMap.
"""

import os
import sys
import json
import requests


def main(city: str, unit: str = "celsius") -> dict:
    """
    Main function of the skill.
    
    Args:
        city: Name of the city
        unit: celsius or fahrenheit
    
    Returns:
        Dictionary with weather data
    """
    # API key from vault
    api_key = os.environ.get('AURAGO_SECRET_OPENWEATHER_API_KEY')
    
    if not api_key:
        return {
            "status": "error",
            "message": "API key not configured. Please store it in the vault under 'openweather_api_key'."
        }
    
    # Unit for API
    units = "metric" if unit == "celsius" else "imperial"
    
    # API request
    url = "https://api.openweathermap.org/data/2.5/weather"
    params = {
        "q": city,
        "appid": api_key,
        "units": units
    }
    
    try:
        response = requests.get(url, params=params, timeout=30)
        response.raise_for_status()
        data = response.json()
        
        return {
            "status": "success",
            "data": {
                "city": data["name"],
                "country": data["sys"]["country"],
                "temperature": data["main"]["temp"],
                "unit": unit,
                "humidity": data["main"]["humidity"],
                "description": data["weather"][0]["description"],
                "wind_speed": data["wind"]["speed"]
            }
        }
        
    except requests.exceptions.RequestException as e:
        return {
            "status": "error",
            "message": f"API error: {str(e)}"
        }


if __name__ == "__main__":
    # Parameters are passed as JSON via stdin
    try:
        params = json.load(sys.stdin)
        result = main(**params)
        print(json.dumps(result, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({
            "status": "error",
            "message": str(e)
        }), file=sys.stderr)
        sys.exit(1)
```

---

## Using Vault Secrets in Skills

Skills can access secrets from the vault to use API keys, tokens, or passwords.

### Step 1: Store secret in vault

1. **Open Web-UI** → "Secrets" → "New Secret"
2. **Enter name**: e.g., `openweather_api_key`
3. **Enter value**: Your API key
4. **Save**

> ⚠️ **Important:** Only secrets you created yourself are available in skills. System secrets (like LLM API keys) are blocked.

### Step 2: Declare in manifest

```json
{
  "vault_keys": ["openweather_api_key", "other_keys"]
}
```

### Step 3: Use in Python code

```python
import os

# Secrets are available as: AURAGO_SECRET_<KEY_NAME>
# The name is automatically: uppercase, special chars become _
api_key = os.environ.get('AURAGO_SECRET_OPENWEATHER_API_KEY')
```

### Security notes

> ⚠️ **Important:** Skills are a special case for vault access!
> 
> Normally, the agent has **no access** to vault secrets. For skills, however, the secrets must be transferred to the Python process in order to be used by the skill.
> 
> This means:
> - Secrets **leave** the protected vault environment of AuraGo
> - They are passed as environment variables to the skill process
> - During skill execution, they are only protected by **operating system user isolation**
> - The skill process runs in a sandbox (venv), but with access to the passed secrets
> 
> **Recommendation:**
> - Use dedicated, restricted API keys for skills (not your main keys)
> - Only enable `tools.python_secret_injection.enabled` when necessary
> - Review code from skills from unknown sources before execution

- Secrets are automatically removed from all outputs (scrubbing)
- Secrets are only available during execution
- The process runs in an isolated environment (venv)
- Requires `tools.python_secret_injection.enabled: true` in `config.yaml`

---

## Examples

### Example 1: Simple File Analyzer

**Manifest** (`file_analyzer.json`):
```json
{
  "name": "file_analyzer",
  "description": "Analyzes a text file and returns statistics",
  "parameters": {
    "type": "object",
    "properties": {
      "filepath": {
        "type": "string",
        "description": "Path to the file to analyze"
      }
    },
    "required": ["filepath"]
  },
  "entry_point": "file_analyzer.py",
  "function": "main"
}
```

**Python** (`file_analyzer.py`):
```python
#!/usr/bin/env python3
import os
import json
import sys


def main(filepath: str) -> dict:
    """Analyze a file and return statistics."""
    
    if not os.path.exists(filepath):
        return {
            "status": "error", 
            "message": f"File not found: {filepath}"
        }
    
    with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
        content = f.read()
    
    lines = content.split('\n')
    words = content.split()
    
    return {
        "status": "success",
        "data": {
            "filepath": filepath,
            "size_bytes": os.path.getsize(filepath),
            "line_count": len(lines),
            "word_count": len(words),
            "char_count": len(content)
        }
    }


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

**Usage in chat:**
```
You: Use the file_analyzer skill for the file document.txt
Agent: 🛠️ Skill: file_analyzer
       
       ✅ Analysis complete:
       - Size: 12,450 bytes
       - Lines: 234
       - Words: 1,892
       - Characters: 12,448
```

### Example 2: GitHub Repository Info

**Manifest** (`github_repo.json`):
```json
{
  "name": "github_repo",
  "description": "Fetches information about a GitHub repository",
  "parameters": {
    "type": "object",
    "properties": {
      "owner": {
        "type": "string",
        "description": "Repository owner"
      },
      "repo": {
        "type": "string",
        "description": "Repository name"
      }
    },
    "required": ["owner", "repo"]
  },
  "entry_point": "github_repo.py",
  "function": "main",
  "dependencies": ["requests"],
  "vault_keys": ["gh_access_token"]
}
```

> **Note:** Use a custom name like `gh_access_token` for your GitHub Personal Access Token. Store it in the vault under that name. `github_token` is reserved for system use.

**Python** (`github_repo.py`):
```python
#!/usr/bin/env python3
import os
import json
import sys
import requests


def main(owner: str, repo: str) -> dict:
    """Fetch GitHub repository information."""
    
    token = os.environ.get('AURAGO_SECRET_GH_ACCESS_TOKEN')
    
    url = f"https://api.github.com/repos/{owner}/{repo}"
    headers = {}
    
    if token:
        headers["Authorization"] = f"token {token}"
    
    try:
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()
        data = response.json()
        
        return {
            "status": "success",
            "data": {
                "name": data["name"],
                "description": data["description"],
                "stars": data["stargazers_count"],
                "forks": data["forks_count"],
                "language": data["language"],
                "created_at": data["created_at"][:10],
                "last_update": data["updated_at"][:10]
            }
        }
    except requests.exceptions.RequestException as e:
        return {"status": "error", "message": f"API error: {str(e)}"}


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

### Example 3: Data Converter

**Manifest** (`data_converter.json`):
```json
{
  "name": "data_converter",
  "description": "Converts data between JSON, CSV and YAML formats",
  "parameters": {
    "type": "object",
    "properties": {
      "data": {
        "type": "string",
        "description": "The data to convert as a string"
      },
      "from_format": {
        "type": "string",
        "enum": ["json", "csv", "yaml"],
        "description": "Source format"
      },
      "to_format": {
        "type": "string",
        "enum": ["json", "csv", "yaml"],
        "description": "Target format"
      }
    },
    "required": ["data", "from_format", "to_format"]
  },
  "entry_point": "data_converter.py",
  "function": "main",
  "dependencies": ["pyyaml", "pandas"]
}
```

**Python** (`data_converter.py`):
```python
#!/usr/bin/env python3
import json
import sys
from io import StringIO


def main(data: str, from_format: str, to_format: str) -> dict:
    """Convert data between different formats."""
    
    try:
        # Parse input
        if from_format == "json":
            parsed = json.loads(data)
        elif from_format == "yaml":
            import yaml
            parsed = yaml.safe_load(data)
        elif from_format == "csv":
            import pandas as pd
            df = pd.read_csv(StringIO(data))
            parsed = df.to_dict('records')
        
        # Convert output
        if to_format == "json":
            output = json.dumps(parsed, indent=2, ensure_ascii=False)
        elif to_format == "yaml":
            import yaml
            output = yaml.dump(parsed, allow_unicode=True, default_flow_style=False)
        elif to_format == "csv":
            import pandas as pd
            if isinstance(parsed, list) and len(parsed) > 0:
                df = pd.DataFrame(parsed)
                output = df.to_csv(index=False)
            else:
                return {"status": "error", "message": "CSV requires list of objects"}
        
        return {
            "status": "success",
            "data": {
                "output": output,
                "entries": len(parsed) if isinstance(parsed, list) else 1
            }
        }
        
    except Exception as e:
        return {"status": "error", "message": f"Conversion error: {str(e)}"}


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

---

## Creating and Managing Skills

### Create a skill

1. **Create files**: Create `.json` and `.py` files in `agent_workspace/skills/`
2. **Set permissions**: Ensure files are readable
3. **Inform agent**: The agent automatically detects new skills via `list_skills`

### View skills

```
You: What skills are available?
Agent: 🛠️ Available skills:
       - file_analyzer: Analyzes text files
       - github_repo: GitHub repository information
       - data_converter: Converts JSON/CSV/YAML
```

### Execute a skill

```
You: Run the file_analyzer skill with filepath="readme.txt"

Or simpler:
You: Analyze the file readme.txt with the file_analyzer
```

### Delete a skill

Simply delete both files from the `agent_workspace/skills/` directory.

---

## Best Practices

### 1. Parameter validation

Always validate inputs:

```python
def main(url: str, max_items: int = 10) -> dict:
    # Validate URL
    if not url.startswith(('http://', 'https://')):
        return {"status": "error", "message": "URL must start with http:// or https://"}
    
    # Validate range
    if not (1 <= max_items <= 100):
        return {"status": "error", "message": "max_items must be between 1 and 100"}
```

### 2. Set timeouts

Always use timeouts for HTTP requests:

```python
import requests

# Timeout in seconds
response = requests.get(url, timeout=30)
```

### 3. Error handling

Catch specific errors:

```python
try:
    # Your logic
    return {"status": "success", "data": result}
except FileNotFoundError:
    return {"status": "error", "message": "File not found"}
except requests.exceptions.RequestException as e:
    return {"status": "error", "message": f"Network error: {e}"}
except Exception as e:
    return {"status": "error", "message": f"Unexpected error: {e}"}
```

### 4. Close resources

Use context managers:

```python
# Good - file is automatically closed
with open(filepath, 'r') as f:
    content = f.read()

# Bad - file remains open
f = open(filepath, 'r')
content = f.read()
```

### 5. Declare dependencies

List all required pip packages in the manifest:

```json
{
  "dependencies": ["requests", "beautifulsoup4", "pandas"]
}
```

Packages are automatically installed on first call.

---

## Using Predefined Templates

AuraGo offers templates for common skill types:

### View templates

```
You: What skill templates are available?
Agent: 📋 Available templates:
       - api_client: REST API client with auth
       - file_processor: File read and write operations
       - data_transformer: Data format conversion
       - scraper: Web scraper with CSS selectors
```

### Create skill from template

```json
{
  "action": "create_skill_from_template",
  "template": "api_client",
  "name": "my_api",
  "description": "My API integration",
  "vault_keys": ["api_key"]
}
```

---

## Troubleshooting

### Skill not found

| Problem | Solution |
|---------|----------|
| Wrong filename | Check that `.json` and `.py` are named identically |
| Invalid JSON | Validate JSON with an online validator |
| Missing permissions | Ensure files are readable |
| Stale cache | Call `list_skills` to refresh the cache |

### Import error

```
ModuleNotFoundError: No module named 'requests'
```

**Solution:** Add `requests` to the `dependencies` in the manifest.

### Vault key not available

```
AURAGO_SECRET_MY_KEY is None
```

**Solution:**
1. Check that `tools.python_secret_injection.enabled: true` in `config.yaml`
2. Ensure the key exists in the vault
3. Verify it's a user-created secret (not a system secret)

### Timeout during execution

**Solution:**
- Increase timeout in configuration
- Run long operations in the background
- Optimize code (e.g., streaming for large data)

---

## Summary

| Aspect | Description |
|--------|-------------|
| **Structure** | 2 files: `.json` (manifest) + `.py` (code) |
| **Location** | `agent_workspace/skills/` |
| **Secrets** | Declare via `vault_keys` in manifest, use as `AURAGO_SECRET_*` |
| **Dependencies** | Specify in manifest, auto-installed |
| **Parameters** | Passed as JSON via stdin |
| **Return** | JSON string via stdout |

---

**Next Steps**

- **[Chapter 6: Tools](06-tools.md)** – Learn about built-in tools
- **[Chapter 14: Security](14-security.md)** – Security policies for skills
- **Web-UI** → Skills → Discover new skills
