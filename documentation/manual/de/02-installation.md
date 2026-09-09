# Kapitel 2: Installation

<p align="center">
  <a href="../images/manual-install.webp"><img src="../images/manual-install.webp" width="560" alt="AuraGo-Gopher packt Binary und Ressourcenpaket an der Werkbank aus"></a>
</p>

Drei Wege, derselbe Gopher: Installer, Docker oder selbst bauen. Die volle UI kommt als **passendes Ressourcenpaket** mit — nicht als zweite Website und nicht vollständig im Binary.

## Systemanforderungen

### Minimal
- 64-bit Betriebssystem (Linux, macOS, Windows 10+)
- 512 MB RAM (ohne lokale Modelle)
- 500 MB freier Speicherplatz plus das UI-Paket
- Internetverbindung, sobald ein gehostetes Modell oder der erste Download nötig ist

### Empfohlen
- 2 GB RAM oder mehr (lokale Modelle und GPU-Runtimes brauchen deutlich mehr)
- Python 3.10+ (für Skill-Ausführung)
- SSD

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
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo
docker compose up -d
```

Öffne **http://localhost:8088**. Das Image bringt Volumes und das UI-Paket mit. Es erzeugt einen Vault-Key, wenn keiner gesetzt ist. Key mit den Daten sichern.

Lege **kein** `config.yaml` aus dem GitHub-Root daneben — die Datei existiert im Repo nicht. Vorlage ist `config_template.yaml`. Ein Host-Mount namens `./config.yaml` wird unter Docker leicht zum Verzeichnis. Optional: `config/config.yaml`. Details: [Docker-Guide](../../docker_installation.md).

> **Docker-Vorteil:** Isolation, einfacheres Backup, Python nicht auf dem Host.

> Für LAN-Zugriff Host unter **Config → Server** auf `0.0.0.0` setzen.

### Option C: Manuelle Installation

**Schritt 1: Download**

Lade von GitHub Releases:

| Datei | Beschreibung |
|-------|--------------|
| `aurago_<os>_<arch>` | Die AuraGo-Executable (muss auf dasselbe Ressourcen-Set gepinnt sein) |
| `aurago-web-assets-<id>.tar.gz` | Volle Web-UI, Desktop, Game-Maker-Runtimes |
| `resources.dat` | Prompts, Skills und andere Backend-Ressourcen — **ersetzt die UI nicht** |

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

Danach das UI-Paket einspielen und prüfen:

```bash
./aurago --install-assets aurago-web-assets-<id>.tar.gz
./aurago --check-assets
```

Ohne passendes Paket oder ungepinntes Binary siehst du nur die Reparaturseite.

### Option D: Aus dem Quellcode bauen

Für Entwickler oder wenn du den Code modifizieren willst:

**Voraussetzungen:**
- Go 1.26.6+
- Python 3.10+ (optional, für Python-Tools)

```bash
# Repository klonen
git clone https://github.com/antibyte/AuraGo.git
cd AuraGo

# Bauen
go run ./cmd/assetpack -out deploy -stage assets/web
go build -ldflags="-s -w $(cat deploy/web-assets.ldflags)" -o aurago ./cmd/aurago

# Oder Release-Artefakte bauen
./make_deploy.sh  # Linux/macOS
# oder
make_deploy.bat   # Windows
```

## Erstkonfiguration

> 💡 **Empfohlen:** Nach dem ersten Start die Web-UI öffnen (**Menü → Config → Provider**), LLM-Provider und API-Key eintragen. Zugangsdaten landen sicher im Vault. `config.yaml` direkt bearbeiten ist nur für Headless- oder Skript-Setups nötig.

### 1. API-Key konfigurieren (Web-UI — empfohlen)

1. Öffne `http://localhost:8088` (oder deine konfigurierte Adresse).
2. Gehe zu **Menü → Config → Provider**.
3. Lege einen Provider an (z. B. OpenRouter), trage den API-Key ein und wähle ein Modell.
4. Unter **Config → LLM Settings** den **Provider** auf die neue Provider-ID setzen.
5. Auf **Speichern** klicken.

### 1b. API-Key konfigurieren (YAML — Alternative)

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
# Manuell starten:
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
tail -f log/aurago.log     # Direkt
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
│   ├── skills/               # Python-Skills
│   ├── tools/                # Agent-erstellte Tools
│   └── workdir/              # Sandkasten (Jail-Wurzel für Datei-Tools)
├── prompts/                  # Identity, Regeln, Persönlichkeiten (nicht unter workdir)
├── assets/web/               # Installiertes UI-Ressourcenpaket
├── data/
│   ├── short_term.db         # Chat-Verlauf
│   ├── vault.bin             # Secrets (AES-256-GCM)
│   ├── vectordb/             # Semantisches Gedächtnis
│   └── *.db                  # Inventar, Invasion, Medien, Knowledge Graph, …
└── log/
    ├── aurago.log            # Anwendung
    └── web_access.log        # HTTP-Zugriff
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
| Nur Reparatur-/Anmeldeseite | Binary ist ungepinnt oder das UI-Paket fehlt. `./aurago --check-assets`, `--install-assets`, sonst mit Asset-Flags neu bauen. [Web-Assets](../../web-assets.md) |
| `resources.dat not found` | Datei neben die Binary legen (Prompts/Skills — nicht die UI) |
| `AURAGO_MASTER_KEY is missing` | `.env` laden: `export $(cat .env \| xargs)` |
| Port bereits belegt | Port unter **Config → Server** ändern (YAML: `server.port`) |
| Python venv Fehler | Python 3.10+ installieren: `sudo apt install python3 python3-venv` |
| Permission denied (Docker) | `sudo usermod -aG docker $USER` und neu einloggen |

## Nächste Schritte

- **[Schnellstart](03-schnellstart.md)** – Die ersten 5 Minuten mit AuraGo
- **[Web-Oberfläche](04-webui.md)** – Die UI kennenlernen
- **[Konfiguration](07-konfiguration.md)** – Feintuning

Die vollständige Weboberfläche wird als passendes lokales Ressourcenpaket installiert; im Binary bleibt eine kleine Reparatur-/Anmeldeseite. Details: [Ressourcen und Offline-Installation](../../web-assets.md).
