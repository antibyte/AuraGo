# AuraGo — Installation Guide

## Quick Install (Linux / macOS)

Paste this into your terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/YOUR_USER/AuraGo/main/install.sh | bash
```

This will:
1. Detect your OS and CPU architecture
2. Download the correct binary + resource pack from GitHub Releases
3. Extract everything into `~/aurago/`
4. Install a system service for auto-start on boot

### Custom install directory

```bash
curl -fsSL https://raw.githubusercontent.com/YOUR_USER/AuraGo/main/install.sh | AURAGO_INSTALL_DIR=/opt/aurago bash
```

### Specific version

```bash
curl -fsSL https://raw.githubusercontent.com/YOUR_USER/AuraGo/main/install.sh | AURAGO_VERSION=v1.0.0 bash
```

---

## Manual Install (all platforms)

### Prerequisites

- A modern 64-bit OS: Linux, macOS, or Windows 10+
- Internet connection (for the LLM API)
- Python 3.9+ (for tool execution — optional but recommended)

### Step 1: Download

Download two files from the [GitHub Releases](https://github.com/YOUR_USER/AuraGo/releases) page:

| File | Description |
|---|---|
| `aurago_<os>_<arch>` | The AuraGo executable for your platform |
| `resources.dat` | Packed resource archive (prompts, skills, tools, config template) |

**Available binaries:**

| OS | amd64 (Intel/AMD) | arm64 (Apple M/ARM) |
|---|---|---|
| Linux | `aurago_linux_amd64` | `aurago_linux_arm64` |
| macOS | `aurago_darwin_amd64` | `aurago_darwin_arm64` |
| Windows | `aurago_windows_amd64.exe` | `aurago_windows_arm64.exe` |

### Step 2: Place files

Create a directory and put both files there:

```bash
mkdir ~/aurago && cd ~/aurago
# Move/copy the downloaded files here:
#   aurago (or aurago.exe)
#   resources.dat
chmod +x aurago   # Linux/macOS only
```

### Step 3: Run setup

```bash
./aurago --setup
```

**On Windows:**
```powershell
.\aurago.exe --setup
```

This command will:

1. **Extract `resources.dat`** → creates `agent_workspace/`, `config.yaml`, `data/`, `log/`
2. **Generate a master key** → saved to `.env` (used for vault encryption)
3. **Install a system service** for automatic boot start:
   - **Linux**: systemd unit (`aurago.service`) — moves the master key to `/etc/aurago/master.key` (root-only, mode 0600)
   - **macOS**: launchd agent (`com.aurago.agent`)
   - **Windows**: Scheduled Task (`AuraGo`, runs at logon)

### Step 4: Configure

Edit `config.yaml` with your favorite editor:

```bash
nano config.yaml   # or vim, code, notepad, etc.
```

**Minimum required settings:**

```yaml
llm:
  provider: openrouter          # or "openai", "local", etc.
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-YOUR_KEY_HERE"
  model: "arcee-ai/trinity-large-preview:free"
```

All other settings have sensible defaults and are optional.

### Step 5: Set the master key

The setup generated a `.env` file with your encryption key.

**If you installed as a systemd service (Linux):**
The install script already moved the key to `/etc/aurago/master.key` (root-only). It is injected automatically via `EnvironmentFile=` in the systemd unit — no manual action needed.

To verify: `sudo cat /etc/aurago/master.key`

**Manual start (no systemd) — Linux/macOS:**
```bash
export $(cat .env | xargs)
```

**Windows (PowerShell):**
```powershell
Get-Content .env | ForEach-Object {
  if ($_ -match '^(.+?)=(.+)$') { [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'User') }
}
```

> **Important:** Back up your master key. If you lose it, the encrypted secrets vault cannot be decrypted.
> - **systemd installs**: `/etc/aurago/master.key`
> - **Manual installs**: `.env` in the AuraGo directory
> - **Docker**: `aurago_master.key` on the Docker host (or `data/.env` inside the volume)

### Step 6: Start AuraGo

**Manual start:**
```bash
./aurago
```

**Via system service:**
```bash
# Linux
sudo systemctl start aurago
sudo systemctl status aurago

# macOS
launchctl start com.aurago.agent

# Windows (runs automatically at logon, or start manually)
schtasks /Run /TN AuraGo
```

### Step 7: Open the Web UI

Navigate to: **http://localhost:8088**

(Port is configurable in `config.yaml` under `server.port`)

---

## File Structure After Setup

```
~/aurago/
├── aurago                          # Executable
├── resources.dat                   # Resource archive (can be deleted after setup)
├── .env                            # Master key (only if NOT using systemd)
├── config.yaml                     # Your configuration
├── agent_workspace/
│   ├── prompts/                    # System prompts & personalities
│   ├── skills/                     # Pre-built skills (web search, etc.)
│   ├── tools/                      # Agent-created reusable tools
│   └── workdir/                    # Agent's working directory
│       └── attachments/            # Uploaded files
├── data/
│   ├── core_memory.md              # Agent's persistent memory
│   ├── chat_history.json           # Conversation history
│   ├── secrets.vault               # Encrypted secrets
│   └── vectordb/                   # Embedding storage
└── log/
    └── supervisor.log              # Application log
```

---

## Updating

To update AuraGo, simply replace the executable:

```bash
cd ~/aurago
curl -fSL -o aurago https://github.com/YOUR_USER/AuraGo/releases/latest/download/aurago_linux_amd64
chmod +x aurago
# Restart
sudo systemctl restart aurago   # or just ./aurago
```

`resources.dat` does **not** need to be re-extracted — the setup skips files that already exist (especially `config.yaml` is never overwritten).

---

## Uninstall

```bash
# Linux
sudo systemctl stop aurago
sudo systemctl disable aurago
sudo rm /etc/systemd/system/aurago.service
sudo rm -rf /etc/aurago
sudo systemctl daemon-reload

# macOS
launchctl unload ~/Library/LaunchAgents/com.aurago.agent.plist
rm ~/Library/LaunchAgents/com.aurago.agent.plist

# Windows
schtasks /Delete /TN AuraGo /F

# Then remove the install directory
rm -rf ~/aurago
```

---

## Troubleshooting

| Problem | Solution |
|---|---|
| `resources.dat not found` | Place `resources.dat` next to the `aurago` binary before running `--setup` |
| `AURAGO_MASTER_KEY is missing` | systemd: check `sudo cat /etc/aurago/master.key`. Manual: run `export $(cat .env \| xargs)` |
| Service install fails on Linux | Run `sudo ./aurago --setup` (systemd requires root for `/etc/systemd/system/`) |
| Port already in use | Change `server.port` in `config.yaml` |
| Python venv creation fails | Install Python 3.9+: `sudo apt install python3 python3-venv` |

---

## Building from Source

If you want to build deployment artifacts yourself:

```bash
git clone https://github.com/YOUR_USER/AuraGo.git
cd AuraGo

# Build for all platforms
./make_deploy.sh        # Linux/macOS
# or
make_deploy.bat         # Windows

# Output in deploy/
ls deploy/
#   aurago_linux_amd64   aurago_darwin_arm64   resources.dat
#   aurago_linux_arm64   aurago_windows_amd64.exe  ...
```

Upload the contents of `deploy/` to a GitHub Release.
