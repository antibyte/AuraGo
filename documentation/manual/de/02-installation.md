# Kapitel 2: Installation

Dieses Kapitel führt dich Schritt für Schritt durch die Installation von AuraGo.

## Systemanforderungen

### Minimal
- 64-bit Betriebssystem (Linux, macOS, Windows 10+)
- 2 GB RAM
- 500 MB freier Speicherplatz
- Internetverbindung (für LLM-API)

### Empfohlen
- 4 GB RAM oder mehr
- Python 3.10+ (für Tool-Ausführung)
- SSD für bessere Performance

### Unterstützte Plattformen

| Betriebssystem | amd64 (Intel/AMD) | arm64 (Apple M/ARM) |
|----------------|-------------------|---------------------|
| Linux          | ✅                | ✅                  |
| macOS          | ✅                | ✅                  |
| Windows        | ✅                | ✅                  |

## Installationsmethoden

### Option A: One-Liner (empfohlen für Linux/macOS)

Die schnellste Methode – ein einziger Befehl:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

Das Script:
1. Erkennt dein Betriebssystem und Architektur
2. Lädt die passende Binary + Ressourcen herunter
3. Extrahiert alles nach `~/aurago/`
4. Erstellt einen systemd-Service für Autostart

**Mit benutzerdefiniertem Verzeichnis:**
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_INSTALL_DIR=/opt/aurago bash
```

**Bestimmte Version installieren:**
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_VERSION=v1.0.0 bash
```

### Option B: Docker (empfohlen für isolierte Umgebung)

Die sicherste Methode – AuraGo läuft in einem Container:

```bash
# Verzeichnis erstellen
mkdir aurago && cd aurago

# Compose-File und Config herunterladen
curl -O https://raw.githubusercontent.com/antibyte/AuraGo/main/docker-compose.yml
curl -O https://raw.githubusercontent.com/antibyte/AuraGo/main/config.yaml

# Konfigurieren (API-Key eintragen)
nano config.yaml

# Starten
docker compose up -d
```

> 💡 **Docker-Vorteil:** Vollständige Isolation, einfaches Backup, keine Python-Abhängigkeiten auf dem Host.

> ⚠️ **Wichtig für Docker:** Setze in `config.yaml` den Host auf `0.0.0.0`:
> ```yaml
> server:
>   host: "0.0.0.0"
> ```

### Option C: Manuelle Installation

**Schritt 1: Download**

Lade zwei Dateien von GitHub Releases herunter:

| Datei | Beschreibung |
|-------|--------------|
| `aurago_<os>_<arch>` | Die AuraGo-Executable |
| `resources.dat` | Ressourcen-Archiv (Prompts, Skills, Tools) |

**Schritt 2: Verzeichnis erstellen**

```bash
mkdir ~/aurago && cd ~/aurago
# Bewege die heruntergeladenen Dateien hierhin
chmod +x aurago   # Nur Linux/macOS
```

**Schritt 3: Setup ausführen**

```bash
./aurago --setup
```

Das Setup:
- Extrahiert `resources.dat`
- Generiert einen Master-Key (gespeichert in `.env`)
- Installiert einen System-Service (optional)

### Option D: Build from Source

Für Entwickler oder wenn du den Code modifizieren willst:

**Voraussetzungen:**
- Go 1.26.2+
- Python 3.10+ (optional, für Python-Tools)

```bash
# Repository klonen
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo

# Bauen
go build -o aurago ./cmd/aurago

# Oder mit Lifeboat (für Self-Updates)
./make_deploy.sh  # Linux/macOS
# oder
make_deploy.bat   # Windows
```

## Erstkonfiguration

### 1. API-Key konfigurieren

Bearbeite `config.yaml`:

```bash
nano config.yaml   # oder vim, code, notepad
```

Minimale Konfiguration:

```yaml
providers:
  - id: main
    type: openrouter
    name: "Haupt-LLM"
    base_url: https://openrouter.ai/api/v1
    api_key: "sk-or-v1-DEIN-API-KEY"
    model: "google/gemini-2.0-flash-001"

llm:
  provider: main
```

> 💡 **Keinen API-Key?** Besuche [openrouter.ai](https://openrouter.ai) – es gibt auch kostenlose Modelle. Das Provider-System erlaubt mehrere LLM-Provider mit Failover; Details im [Kapitel 7: Konfiguration](07-konfiguration.md).

### 2. Master Key setzen

Der Master Key verschlüsselt den Secrets-Vault. Er wurde beim Setup in `.env` gespeichert:

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

> ⚠️ **Wichtig:** Bewahre `.env` sicher auf! Ohne diesen Schlüssel kann der Vault nicht entschlüsselt werden.

### 3. System-Service einrichten (optional)

**Linux (systemd):**
```bash
sudo ./install_service_linux.sh
# oder manuell:
sudo systemctl enable --now aurago
```

**macOS (launchd):**
```bash
launchctl load ~/Library/LaunchAgents/com.aurago.agent.plist
```

**Windows:**
```powershell
# Wird automatisch beim Setup erstellt
# Manuel starten:
schtasks /Run /TN AuraGo
```

## Installation verifizieren

### 1. AuraGo starten

```bash
# Manuell
./aurago

# Oder via Service
sudo systemctl start aurago
```

### 2. Logs prüfen

```bash
# Direkt in der Konsole (beim manuellen Start)

# Oder via Service
sudo journalctl -u aurago -f   # Linux
tail -f log/supervisor.log     # Direkt
```

Du solltest sehen:
```
[INFO] AuraGo starting...
[INFO] Web UI available at http://localhost:8088
[INFO] Agent loop initialized
```

### 3. Web-UI öffnen

Navigiere zu: **http://localhost:8088**

Du solltest den Login-Screen oder den Chat sehen (je nach Auth-Konfiguration).

## Dateistruktur nach Installation

```
~/aurago/
├── aurago                    # Executable
├── resources.dat             # Kann nach Setup gelöscht werden
├── .env                      # Master Key (GEHEIM HALTEN!)
├── config.yaml               # Deine Konfiguration
├── agent_workspace/
│   ├── prompts/              # System-Prompts & Persönlichkeiten
│   ├── skills/               # Vorgefertigte Python-Skills
│   ├── tools/                # Agent-erstellte Tools
│   └── workdir/              # Arbeitsverzeichnis
│       └── attachments/      # Hochgeladene Dateien
├── data/
│   ├── core_memory.md        # Persistentes Gedächtnis
│   ├── chat_history.json     # Chat-Verlauf
│   ├── vault.bin             # Verschlüsselte Secrets (AES-256-GCM)
│   └── vectordb/             # Vektor-Datenbank
└── log/
    └── supervisor.log        # Anwendungs-Logs
```

## Update durchführen

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

> 💡 `resources.dat` muss NICHT neu extrahiert werden – deine Config bleibt erhalten.

## Deinstallation

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

| Problem | Lösung |
|---------|--------|
| `resources.dat not found` | Datei muss im gleichen Verzeichnis wie `aurago` liegen |
| `AURAGO_MASTER_KEY is missing` | `.env` laden: `export $(cat .env \| xargs)` |
| Port bereits belegt | `server.port` in `config.yaml` ändern |
| Python venv Fehler | Python 3.10+ installieren: `sudo apt install python3 python3-venv` |
| Permission denied (Docker) | `sudo usermod -aG docker $USER` und neu einloggen |

## Nächste Schritte

- **[Schnellstart](03-schnellstart.md)** – Die ersten 5 Minuten mit AuraGo
- **[Web-Oberfläche](04-webui.md)** – Die UI kennenlernen
- **[Konfiguration](07-konfiguration.md)** – Feintuning
