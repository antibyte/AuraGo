# Chapter 2: Installation

This chapter guides you step by step through installing AuraGo.

## System Requirements

### Minimum
- 64-bit operating system (Linux, macOS, Windows 10+)
- 2 GB RAM
- 500 MB free disk space
- Internet connection (for LLM API)

### Recommended
- 4 GB RAM or more
- Python 3.10+ (for tool execution)
- SSD for better performance

### Supported Platforms

| Operating System | amd64 (Intel/AMD) | arm64 (Apple M/ARM) |
|------------------|-------------------|---------------------|
| Linux            | ✅                | ✅                  |
| macOS            | ✅                | ✅                  |
| Windows          | ✅                | ✅                  |

## Installation Methods

### Option A: One-Liner (recommended for Linux/macOS)

The fastest method – a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

The script will:
1. Detect your OS and architecture
2. Download the correct binary + resources
3. Extract everything to `~/aurago/`
4. Create a systemd service for auto-start

**With custom directory:**
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_INSTALL_DIR=/opt/aurago bash
```

**Install specific version:**
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_VERSION=v1.0.0 bash
```

### Option B: Docker (recommended for isolated environments)

The safest method – AuraGo runs in a container:

```bash
# Create directory
mkdir aurago && cd aurago

# Download compose file and config
curl -O https://raw.githubusercontent.com/antibyte/AuraGo/main/docker-compose.yml
curl -O https://raw.githubusercontent.com/antibyte/AuraGo/main/config.yaml

# Configure (add API key)
nano config.yaml

# Start
docker compose up -d
```

> 💡 **Docker advantage:** Complete isolation, easy backup, no Python dependencies on host.

> ⚠️ **Important for Docker:** Set host to `0.0.0.0` in `config.yaml`:
> ```yaml
> server:
>   host: "0.0.0.0"
> ```

### Option C: Manual Installation

**Step 1: Download**

Download two files from GitHub Releases:

| File | Description |
|------|-------------|
| `aurago_<os>_<arch>` | The AuraGo executable |
| `resources.dat` | Resource archive (prompts, skills, tools) |

**Step 2: Create directory**

```bash
mkdir ~/aurago && cd ~/aurago
# Move downloaded files here
chmod +x aurago   # Linux/macOS only
```

**Step 3: Run setup**

```bash
./aurago --setup
```

The setup will:
- Extract `resources.dat`
- Generate a master key (saved in `.env`)
- Install a system service (optional)

### Option D: Build from Source

For developers or if you want to modify the code:

**Prerequisites:**
- Go 1.26.1+
- Python 3.10+ (optional, for tools)

```bash
# Clone repository
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo

# Build
go build -o aurago cmd/aurago/main.go

# Or with lifeboat (for self-updates)
./make_deploy.sh  # Linux/macOS
# or
make_deploy.bat   # Windows
```

## Initial Configuration

### 1. Configure API Key

Edit `config.yaml`:

```bash
nano config.yaml   # or vim, code, notepad
```

Minimal configuration:

```yaml
server:
  host: "127.0.0.1"
  port: 8088

llm:
  provider: openrouter
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-YOUR-API-KEY"
  model: "arcee-ai/trinity-large-preview:free"
```

> 💡 **No API key?** Visit [openrouter.ai](https://openrouter.ai) – there are free models available.

### 2. Set Master Key

The master key encrypts the secrets vault. It was saved to `.env` during setup:

**Linux/macOS:**
```bash
export $(cat .env | xargs)
```

**Windows (PowerShell):**
```powershell
Get-Content .env | ForEach-Object {
  if ($_ -match '^(.+?)=(.+)$') { 
    [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'User') 
  }
}
```

> ⚠️ **Important:** Keep `.env` safe! Without this key, the vault cannot be decrypted.

### 3. Set up system service (optional)

**Linux (systemd):**
```bash
sudo ./install_service_linux.sh
# or manually:
sudo systemctl enable --now aurago
```

**macOS (launchd):**
```bash
launchctl load ~/Library/LaunchAgents/com.aurago.agent.plist
```

**Windows:**
```powershell
# Created automatically during setup
# Manual start:
schtasks /Run /TN AuraGo
```

## Verify Installation

### 1. Start AuraGo

```bash
# Manual
./aurago

# Or via service
sudo systemctl start aurago
```

### 2. Check logs

```bash
# Direct in console (when starting manually)

# Or via service
sudo journalctl -u aurago -f   # Linux
tail -f log/supervisor.log     # Direct
```

You should see:
```
[INFO] AuraGo starting...
[INFO] Web UI available at http://localhost:8088
[INFO] Agent loop initialized
```

### 3. Open Web UI

Navigate to: **http://localhost:8088**

You should see the login screen or chat (depending on auth configuration).

## Directory Structure After Installation

```
~/aurago/
├── aurago                    # Executable
├── resources.dat             # Can be deleted after setup
├── .env                      # Master key (KEEP SECRET!)
├── config.yaml               # Your configuration
├── agent_workspace/
│   ├── prompts/              # System prompts & personalities
│   ├── skills/               # Pre-built Python skills
│   ├── tools/                # Agent-created tools
│   └── workdir/              # Working directory
│       └── attachments/      # Uploaded files
├── data/
│   ├── core_memory.md        # Persistent memory
│   ├── chat_history.json     # Chat history
│   ├── secrets.vault         # Encrypted secrets
│   └── vectordb/             # Vector database
└── log/
    └── supervisor.log        # Application logs
```

## Updating

### One-Liner Installation:
```bash
cd ~/aurago
curl -fSL -o aurago https://github.com/antibyte/AuraGo/releases/latest/download/aurago_linux_amd64
chmod +x aurago
sudo systemctl restart aurago
```

### Docker:
```bash
docker compose pull
docker compose up -d
```

> 💡 `resources.dat` does NOT need to be re-extracted – your config is preserved.

## Uninstallation

**Linux:**
```bash
sudo systemctl stop aurago
sudo systemctl disable aurago
sudo rm /etc/systemd/system/aurago.service
rm -rf ~/aurago
```

**macOS:**
```bash
launchctl unload ~/Library/LaunchAgents/com.aurago.agent.plist
rm ~/Library/LaunchAgents/com.aurago.agent.plist
rm -rf ~/aurago
```

**Windows:**
```powershell
schtasks /Delete /TN AuraGo /F
Remove-Item -Recurse -Force C:\Users\$env:USERNAME\aurago
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `resources.dat not found` | File must be in same directory as `aurago` |
| `AURAGO_MASTER_KEY is missing` | Load `.env`: `export $(cat .env \| xargs)` |
| Port already in use | Change `server.port` in `config.yaml` |
| Python venv error | Install Python 3.9+: `sudo apt install python3 python3-venv` |
| Permission denied (Docker) | `sudo usermod -aG docker $USER` and re-login |

## Next Steps

- **[Quick Start](03-quickstart.md)** – First 5 minutes with AuraGo
- **[Web Interface](04-webui.md)** – Learn the UI
- **[Configuration](07-configuration.md)** – Fine-tuning
