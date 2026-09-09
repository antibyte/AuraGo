# Chapter 2: Installation

<p align="center">
  <a href="../images/manual-install.webp"><img src="../images/manual-install.webp" width="560" alt="AuraGo gopher unpacking the binary and resource bundle at a workbench"></a>
</p>

Three roads, same gopher: installer, Docker, or build it yourself. The full UI arrives as a **matching resource set** — not a second website, and not entirely inside the binary.

## System Requirements

### Minimum
- 64-bit operating system (Linux, macOS, Windows 10+)
- 512 MB RAM (without local models)
- 500 MB free disk plus the UI set
- Internet, once a hosted model or the first download is needed

### Recommended
- 2 GB RAM or more (local models and GPU runtimes want much more)
- Python 3.10+ (for skill execution)
- SSD

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
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo
docker compose up -d
```

Open **http://localhost:8088**. The image ships volumes and the UI set. It creates a vault key if none is supplied. Keep that key with the backups.

Do **not** curl `config.yaml` from the GitHub root — that file is not in the repo. The template is `config_template.yaml`. A host bind named `./config.yaml` easily becomes a directory under Docker. Optional: `config/config.yaml`. Details: [Docker guide](../../docker_installation.md).

> **Docker advantage:** isolation, easier backups, no Python on the host.

> For LAN access set the host to `0.0.0.0` under **Config → Server**.

### Option C: Manual Installation

**Step 1: Download**

Download from GitHub Releases:

| File | Description |
|------|-------------|
| `aurago_<os>_<arch>` | The AuraGo executable (must be pinned to the same resource set) |
| `aurago-web-assets-<id>.tar.gz` | Full web UI, desktop, Game Maker runtimes |
| `resources.dat` | Prompts, skills and other backend resources — **not a UI replacement** |

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

Then install and verify the UI set:

```bash
./aurago --install-assets aurago-web-assets-<id>.tar.gz
./aurago --check-assets
```

Without a matching set — or with an unpinned binary — you only get the recovery page.

### Option D: Build from source

For developers or if you want to modify the code:

**Prerequisites:**
- Go 1.26.6+
- Python 3.10+ (optional, for tools)

```bash
# Clone repository
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo

# Build
go run ./cmd/assetpack -out deploy -stage assets/web
go build -ldflags="-s -w $(cat deploy/web-assets.ldflags)" -o aurago ./cmd/aurago

# Or build release artifacts
./make_deploy.sh  # Linux/macOS
# or
make_deploy.bat   # Windows
```

## Initial Configuration

> 💡 **Recommended:** After the first start, open the Web UI (**Menu → Config → Providers**) to add your LLM provider and API key. Credentials are stored securely in the vault. Editing `config.yaml` directly is only needed for headless or scripted setups.

### 1. Configure API Key (Web UI — recommended)

1. Open `http://localhost:8088` (or your configured host/port).
2. Go to **Menu → Config → Providers**.
3. Add a provider (e.g. OpenRouter), paste your API key, and select a model.
4. Under **Config → LLM Settings**, set **Provider** to your new provider ID.
5. Click **Save**.

### 1b. Configure API Key (YAML — alternative)

Edit `config.yaml`:

```bash
nano config.yaml   # or vim, code, notepad
```

Minimal configuration:

```yaml
providers:
  - id: main
    type: openrouter
    name: "Main LLM"
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-YOUR-API-KEY"
    model: "google/gemini-2.0-flash-001"

llm:
  provider: "main"

server:
  host: "127.0.0.1"
  port: 8088
```

> 💡 **No API key?** Visit [openrouter.ai](https://openrouter.ai) – there are free models available.

> ℹ️ **Note:** The provider system (using `providers:` list) is the recommended configuration method for AuraGo 2.x. The provider ID is then referenced in the `llm.provider` field.

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
tail -f log/aurago.log     # Direct
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
│   ├── skills/               # Python skills
│   ├── tools/                # Agent-created tools
│   └── workdir/              # Sandbox (jail root for file tools)
├── prompts/                  # Identity, rules, personalities (not under workdir)
├── assets/web/               # Installed UI resource set
├── data/
│   ├── short_term.db         # Chat history
│   ├── vault.bin             # Secrets (AES-256-GCM)
│   ├── vectordb/             # Semantic memory
│   └── *.db                  # Inventory, invasion, media, knowledge graph, …
└── log/
    ├── aurago.log            # Application
    └── web_access.log        # HTTP access
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
| Recovery/login page only | Unpinned binary or missing UI set. `./aurago --check-assets`, `--install-assets`, or rebuild with asset flags. [Web assets](../../web-assets.md) |
| `resources.dat not found` | Put the file next to the binary (prompts/skills — not the UI) |
| `AURAGO_MASTER_KEY is missing` | Load `.env`: `export $(cat .env \| xargs)` |
| Port already in use | Change port in **Config → Server** (YAML: `server.port`) |
| Python venv error | Install Python 3.10+: `sudo apt install python3 python3-venv` |
| Permission denied (Docker) | `sudo usermod -aG docker $USER` and re-login |

## Next Steps

- **[Quick Start](03-quickstart.md)** – First 5 minutes with AuraGo
- **[Web Interface](04-webui.md)** – Learn the UI
- **[Configuration](07-configuration.md)** – Fine-tuning

The full web interface is installed as a matching local resource set; the binary retains a small recovery/login page. See [resources and offline installation](../../web-assets.md).
