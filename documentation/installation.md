# AuraGo — Installation Guide

## Installer script (Linux)

The installer is a Linux script (x86_64 and arm64). macOS and Windows use the [manual install](#manual-install-all-platforms) below.

### Verified install (recommended)

Download the installer and its checksum list from the latest release, and run the script only if the checksum matches:

```bash
curl -fsSLO https://github.com/antibyte/AuraGo/releases/latest/download/install.sh
curl -fsSLO https://github.com/antibyte/AuraGo/releases/latest/download/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS && bash install.sh
```

### Quick install (runs the script from the main branch)

Paste this into your terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

This will:
1. Detect your OS and CPU architecture
2. Download the correct binary + resource pack from GitHub Releases
3. Extract everything into `~/aurago/`
4. Install a system service for auto-start on boot

### HTTPS and network access

New installations use HTTPS with an automatically generated local certificate.
Native setup probes `server.host` on port **443**, then **8443** if 443 is busy or
requires privileges the process does not have. If neither works, setup fails
with an actionable error. The selected port is saved; later starts never move
an explicitly configured port. The installer prepares Linux bind capabilities
before probing, and its systemd service retains `CAP_NET_BIND_SERVICE`.

Network access is a separate installer choice. Without interactive input,
HTTPS is enabled and `server.host` remains `127.0.0.1`. Existing installations
keep their transport, ports and certificate mode, even when the old file has
no HTTPS fields. See [HTTPS configuration](https.md) for browser trust, proxies,
Let’s Encrypt, and switching back to HTTP.

### Custom install directory

The installer reads the target directory from `AURAGO_DIR` (default `~/aurago`):

```bash
AURAGO_DIR=/opt/aurago bash install.sh
```

With the quick install, set the variable on the `bash` side of the pipe:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_DIR=/opt/aurago bash
```

### Older versions

The installer always installs the latest release. For an older version, use the [manual install](#manual-install-all-platforms) with the assets of `https://github.com/antibyte/AuraGo/releases/tag/<tag>`.

---

## Manual Install (all platforms)

### Prerequisites

- A modern 64-bit OS: Linux, macOS, or Windows 10+
- Internet connection (for the LLM API)
- Python 3.9+ (for tool execution — optional but recommended)

### Step 1: Download

Download two files from the [GitHub Releases](https://github.com/antibyte/AuraGo/releases) page:

| File | Description |
|---|---|
| `aurago_<os>_<arch>` | The AuraGo executable for your platform |
| `resources.dat` | Packed resource archive (prompts, skills, tools, config template) |

**Available binaries:**

| OS | amd64 (Intel/AMD) | arm64 (Apple M/ARM) |
|---|---|---|
| Linux | `aurago_linux` | `aurago_linux_arm64` |
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

After starting AuraGo, open its HTTPS address and complete the Setup Wizard.
Add the provider and API key through the wizard or **Config → Providers**;
credentials belong in the encrypted Vault. Server settings can be changed in
**Config → Server** after setup. The wizard displays connection status and
browser feature requirements without changing the transport mid-session.

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

Open **https://localhost** or **https://localhost:8443**, as reported by the installer. The saved port is `server.https.https_port`. Verify the local certificate and configure browser/device trust if needed. Existing HTTP installations retain their original address.

On the same machine the setup wizard opens directly. If AuraGo is reachable from other devices (a `server.host` other than loopback, HTTPS, a reverse proxy, a Cloudflare tunnel or Tailscale), the wizard first asks for a one-time setup token. AuraGo prints it to the log on startup as `bootstrap_token`, together with a `/setup#bootstrap=<token>` path that fills it in automatically. The token changes on every restart and stops working once setup is finished.

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

**Installed with the installer script:** run the updater from the install directory (the installer put `update.sh` there; the binary is `bin/aurago_linux`):

```bash
cd ~/aurago
./update.sh
```

**Manual install:** replace the executable (on arm64 download `aurago_linux_arm64`):

```bash
cd ~/aurago
curl -fSL -o aurago https://github.com/antibyte/AuraGo/releases/latest/download/aurago_linux
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
| Page capture or the browser tools report `No usable sandbox` | See [Headless browser sandbox](#headless-browser-sandbox) |

---

## Headless browser sandbox

**Symptom:** Homepage Studio says page capture can't start Chromium because the system blocks its sandbox, and the AuraGo log contains `No usable sandbox`. The scraper, web capture, form and performance tools fail the same way.

**Cause:** Chromium builds its sandbox from unprivileged user namespaces. AuraGo keeps that sandbox on instead of starting Chromium with `--no-sandbox`, so Chromium refuses to start where the host forbids those namespaces.

### Ubuntu 23.10+ (AppArmor)

Ubuntu 23.10+ sets `kernel.apparmor_restrict_unprivileged_userns = 1`. Only programs with an AppArmor profile that grants `userns` may create user namespaces, and the Chromium that AuraGo downloads into `~/.cache/rod/browser/` has no such profile.

**Fix:** `install.sh` and `update.sh` write `/etc/apparmor.d/aurago-chromium-<user>` for the AuraGo service user when the setting above is `1` (they ask for your sudo password). There is one profile per user, so several AuraGo instances on one host do not overwrite each other. Run `./update.sh` once, even if AuraGo is already up to date. An already-current update repairs a missing or stale profile without rebuilding or restarting AuraGo; an unchanged profile needs no privileged command.

**Other setups** (manual installs, a different user or home directory): replace `aurago` in the file name, the profile name and the `local/` include with the user that runs AuraGo, and `/home/aurago` with that user's home directory, then load the profile.

```bash
sudo tee /etc/apparmor.d/aurago-chromium-aurago >/dev/null <<'EOF'
abi <abi/4.0>,
include <tunables/global>

profile aurago-chromium-aurago /home/aurago/.cache/rod/browser/chromium-*/chrome flags=(unconfined) {
  userns,

  include if exists <local/aurago-chromium-aurago>
}
EOF
sudo apparmor_parser -r /etc/apparmor.d/aurago-chromium-aurago
sysctl kernel.apparmor_restrict_unprivileged_userns   # 1 = restricted, the profile is needed
```

**Trade-off:** `flags=(unconfined)` means the Chromium at that path runs without AppArmor confinement, like every program without a profile on a stock Ubuntu host, and may create user namespaces. The AuraGo user can write to that path, so any program it places there is treated the same way. This is the same trade-off as Puppeteer's documented profile for its cached Chrome, and it keeps Chromium's own sandbox active instead of switching it off.

### Other systems

The same `No usable sandbox` error appears wherever unprivileged user namespaces are disabled: Debian-style kernels with `kernel.unprivileged_userns_clone = 0`, or containers whose seccomp profile blocks `clone` with `CLONE_NEWUSER`. Allow them for AuraGo's Chromium (for example `sudo sysctl -w kernel.unprivileged_userns_clone=1`, made permanent in `/etc/sysctl.d/`, or a seccomp profile that permits user namespaces), or run AuraGo in its Docker image: inside a container the browser launcher switches Chromium's sandbox off by itself.

---

## Building from Source

If you want to build deployment artifacts yourself:

```bash
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo

# Build for all platforms
./make_deploy.sh        # Linux/macOS
# or
make_deploy.bat         # Windows

# Output: Linux binaries in bin/, everything else in deploy/
ls bin/ deploy/
#   bin/aurago_linux          deploy/aurago_darwin_arm64   deploy/resources.dat
#   bin/aurago_linux_arm64    deploy/aurago_windows_amd64.exe  ...
```

Upload the contents of `deploy/` and the Linux binaries from `bin/` to a GitHub Release.

Full browser resources are supplied by the version-bound local asset set. The installer and Docker image include it; bare executables retain a small recovery/login page. See [resource packaging and offline installation](web-assets.md).
