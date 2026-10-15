# Kapitel 16: Troubleshooting

<p align="center">
  <a href="../images/manual-install.webp"><img src="../images/manual-install.webp" width="480" alt="AuraGo-Gopher beim Auspacken der Installation"></a>
</p>

Erst die Logs, dann die Schalter, dann das Ressourcenpaket. `log/aurago.log` ist die Spur — nicht `supervisor.log`.

## Inhaltsverzeichnis

1. [Systematische Fehlersuche](#systematische-fehlersuche)
2. [Installationsprobleme](#installationsprobleme)
3. [Verbindungsprobleme](#verbindungsprobleme)
4. [LLM/API-Fehler](#llmapi-fehler)
5. [Gedächtnis- und Performance-Probleme](#gedächtnis--und-performance-probleme)
6. [Docker-spezifische Probleme](#docker-spezifische-probleme)
7. [Logs effektiv lesen](#logs-effektiv-lesen)
8. [Debug-Modus](#debug-modus)
9. [Diagnose-Befehle](#diagnose-befehle)
10. [Wiederherstellungsverfahren](#wiederherstellungsverfahren)
11. [Hilfe erhalten](#hilfe-erhalten)
12. [Zusätzliche Diagnosepunkte für aktuelle Features](#zusätzliche-diagnosepunkte-für-aktuelle-features)
13. [Kurzreferenz](#kurzreferenz)

---

## Systematische Fehlersuche

Bevor du einzelne Lösungen ausprobierst, gehe systematisch vor:

1. **Logs prüfen** – In `log/aurago.log` stehen meist die Antworten
2. **Kürzliche Änderungen** – Was wurde zuletzt geändert?
3. **Isolation testen** – Tritt das Problem nur bei bestimmten Aktionen auf?
4. **Schrittweise einschränken** – Ein Feature nach dem anderen deaktivieren

### Schnell-Diagnose-Checkliste

```text
□ Läuft AuraGo? (Prozess/Systemctl-Status prüfen)
□ Ist die Web-UI erreichbar? (http://localhost:8088)
□ Werden Logs geschrieben? (log/aurago.log prüfen)
□ Ist der LLM-API-Key gültig? (mit curl testen)
□ Ist der Master-Key gesetzt? (echo $AURAGO_MASTER_KEY)
□ Ist die config.yaml gültig? (mit YAML-Linter prüfen)
```

## Installationsprobleme

### Binary startet nicht

| Symptom | Ursache | Lösung |
|---------|---------|--------|
| `permission denied` | Keine Ausführungsrechte | `chmod +x aurago` |
| `cannot execute binary` | Falsche Architektur | Richtige Binary für dein System downloaden |
| `no such file or directory` | Fehlende Bibliotheken | Statisch gelinkte Binary verwenden oder Abhängigkeiten installieren |

### Nur die Reparatur-/Anmeldeseite

Die volle UI fehlt oder das Binary ist nicht auf das Ressourcen-Set gepinnt.

```bash
./aurago --assets-info
./aurago --check-assets
./aurago --install-assets aurago-web-assets-<id>.tar.gz
```

`assetpack` allein repariert ein ungepinntes Binary nicht. Mit den generierten ldflags neu bauen, prüfen, neu starten. [Web-Assets](../../web-assets.md).

### Resources.dat nicht gefunden

`resources.dat` ist das **Backend-Archiv** (Prompts, Skills, Vorlage). Es ersetzt die Web-UI nicht. Fehlt nur die Oberfläche, siehe oben `--check-assets`.

```text
Fehler: resources.dat not found in working directory
```

**Lösung:** Datei neben die Binary legen (Release-Asset `resources.dat`), nicht eine UI-`tar.gz` umbenennen.

### Python venv Fehler

```text
Fehler: failed to create virtual environment
```

**Lösung:**
```bash
# Python 3.10+ installieren
sudo apt install python3 python3-venv python3-pip  # Debian/Ubuntu
brew install python@3.10                           # macOS

# Oder pfad explizit setzen
export PYTHON_PATH=/usr/bin/python3.10
```

## Verbindungsprobleme

### Web UI nicht erreichbar

| Prüfung | Befehl/Lösung |
|---------|---------------|
| Läuft AuraGo? | `ps aux \| grep aurago` |
| Richtiger Port? | In `config.yaml` prüfen: `server.port` |
| Firewall? | `sudo ufw allow 8088` oder Port ändern |
| Falsche IP? | `server.host` auf `0.0.0.0` setzen für LAN-Zugriff |

> [!WARNING]
> `0.0.0.0` exponiert die UI im LAN. Nie ohne Auth im Internet nutzen!

### Telegram Bot reagiert nicht

```text
Symptom: Keine Antwort auf Nachrichten
```

**Checkliste:**
1. Bot-Token korrekt in `config.yaml`?
2. Telegram-User-ID korrekt? (Mit @userinfobot prüfen)
3. AuraGo neu gestartet nach Config-Änderung?
4. Bot bei @BotFather gestartet? (/start)

**Logs prüfen:**
```bash
tail -f log/aurago.log | grep -i telegram
```

### Port bereits belegt

> [!WARNING]
> **Fehler:** `bind: address already in use`

**Lösung:**
```bash
# Prozess finden, der Port 8088 belegt
# Linux/macOS
lsof -i :8088
sudo kill -9 <PID>

# Windows
netstat -ano | findstr :8088
taskkill /PID <PID> /F

# Oder Port in der Web-UI ändern: Config → Server → Port
# YAML-Alternative:
server:
  port: 8089  # Anderen Port verwenden
```

### Discord-Bot-Probleme

> [!WARNING]
> **Bot erscheint offline:**

1. Bot-Token im Discord Developer Portal prüfen
2. `Gateway Intents` aktiviert? (Server Members, Message Content)
3. Bot braucht die Berechtigung `Send Messages` im Kanal
4. **Config → Messenger → Discord** → **Aktiviert** prüfen (YAML: `discord.enabled: true`)

## LLM/API-Fehler

### Authentifizierung fehlgeschlagen

```text
Fehler: 401 Unauthorized - Invalid API key
```

**Lösung:**
```bash
# API-Key direkt testen
curl -H "Authorization: Bearer DEIN-KEY" \
  https://openrouter.ai/api/v1/models

# Web-UI: Config → Provider → API-Key prüfen
# YAML-Alternative:
nano config.yaml
# Der API-Key wird im providers-Block konfiguriert, nicht unter llm.api_key
```

### Rate Limiting (429)

```text
Fehler: 429 Too Many Requests
```

**Lösungen:**
- **Config → Agent** → `step_delay_seconds` erhöhen (YAML: `agent.step_delay_seconds`)
- Anderes/schnelleres Modell unter **Config → LLM Settings** wählen
- **Config → Budget** aktivieren für Kostenüberblick

### YAML-Referenz
```yaml
# Verzögerung zwischen Aufrufen einbauen
agent:
  step_delay_seconds: 2  # 2 Sekunden zwischen Tool-Aufrufen warten
```

Oder Circuit-Breaker-Backoff implementieren:
```yaml
circuit_breaker:
  retry_intervals:
    - 10s   # Erster Retry nach 10 Sekunden
    - 2m    # Zweiter Retry nach 2 Minuten
    - 10m   # Dritter Retry nach 10 Minuten
```

### Modell nicht gefunden (404)

```text
Fehler: 404 Model not found
```

**Lösung:** Modell-ID unter **Config → LLM Settings** prüfen (YAML: `llm.model`). Bei OpenRouter: Im Dashboard verifizieren.

### Fallback-LLM-Aktivierung

Schlägt dein Haupt-LLM wiederholt fehl, aktiviere den Fallback.

> 🖥️ **Web-UI:** **Config → Fallback LLM**.

### YAML-Referenz
```yaml
fallback_llm:
  enabled: true
  base_url: "https://openrouter.ai/api/v1"
  api_key: "sk-or-v1-BACKUP-KEY"
  model: "meta-llama/llama-3.1-8b-instruct:free"
  error_threshold: 2  # Nach 2 aufeinanderfolgenden Fehlern wechseln
```

## Gedächtnis- und Performance-Probleme

### Hoher RAM-Verbrauch

| Ursache | Lösung |
|---------|--------|
| Große Vektordatenbank | **Config → Embeddings** → Provider deaktivieren |
| Langer Chat-Verlauf | `/reset` oder **Config → Agent** → Memory Compression Limit senken |
| Memory Leak | AuraGo neustarten, Bug reporten |

### YAML-Referenz
```yaml
agent:
  context_window: 8192        # Kontextfenster verkleinern
  memory_compression_char_limit: 25000  # Früher komprimieren
  max_tool_calls: 8           # Tool-Aufrufketten begrenzen

circuit_breaker:
  max_tool_calls: 15          # Hartes Limit
  llm_timeout_seconds: 120    # Kürzere Timeouts
```

### Datenbank-Locks

```text
Fehler: database is locked
```

**Lösung:**
```bash
# Prozess beenden und Lock entfernen
pkill aurago
rm data/*.lock 2>/dev/null
./aurago
```

### VectorDB-Probleme

> [!WARNING]
> **Fehler:** `chromem: index corrupted` oder die Suche liefert keine Ergebnisse

**Wiederherstellung:**
```bash
# Vektordatenbank neu aufbauen
rm -rf data/vectordb/*

# Beim nächsten Start baut AuraGo sie neu auf
# Wissensdateien gegebenenfalls neu indizieren
```

### Langsame Antworten

**Optimierungs-Tipps:**
1. Schnelleres LLM-Modell verwenden
2. `agent.memory_compression_char_limit` reduzieren
3. Nicht benötigte Tools deaktivieren
4. Embeddings auf `disabled` setzen (wenn nicht benötigt)

## Docker-spezifische Probleme

### Container startet nicht

```bash
# Logs prüfen
docker compose logs -f

# Häufige Ursachen:
# 1. Config-Datei fehlt
ls -la config.yaml

# 2. Falsche Berechtigungen
sudo chown -R $USER:$USER ./data

# 3. Port belegt
docker compose down
sudo lsof -i :8088  # Prüfen, was den Port blockiert
```

### Web UI in Docker nicht erreichbar

> [!WARNING]
> **Kritisch:** In Docker zwingend `host: "0.0.0.0"` verwenden!

```yaml
# config.yaml - KORREKT für Docker
server:
  host: "0.0.0.0"  # Auf allen Interfaces lauschen
  port: 8088
```

### Permission denied auf Docker-Socket

```text
Fehler: Cannot connect to Docker daemon
```

**Lösung:**
```bash
# Docker-Gruppe
sudo usermod -aG docker $USER
# Neu einloggen oder:
newgrp docker

# Oder Socket-Berechtigungen (unsicherer)
sudo chmod 666 /var/run/docker.sock
```

### Volume-Persistenz-Probleme

> [!WARNING]
> **Daten nach Container-Neustart verloren:**

**docker-compose.yml prüfen:**
```yaml
volumes:
  aurago_data:/app/data        # Muss ein benanntes Volume sein
  aurago_workdir:/app/agent_workspace/workdir
```

**Volumes prüfen:**
```bash
docker volume ls
docker volume inspect aurago_aurago_data
```

### Python-Skills schlagen in Docker fehl

> [!WARNING]
> **Fehler:** `pip install failed` oder `module not found`

**Lösung:**
```bash
# Workdir-Volume zurücksetzen
docker compose down
docker volume rm aurago_aurago_workdir
docker compose up -d
```

### Zugriff auf Host-Dienste aus Docker

| Dienst | URL in Docker |
|---------|---------------|
| Home Assistant | `http://host.docker.internal:8123` |
| Ollama | `http://host.docker.internal:11434/v1` |
| Lokale API | `http://host.docker.internal:PORT` |

> [!TIP]
> **Linux-Nutzer:** In `docker-compose.yml` ergänzen:
> ```yaml
> extra_hosts:
> - "host.docker.internal:host-gateway"
> ```

## Logs effektiv lesen

### Log-Dateien

| Plattform | Log-Ort |
|-----------|---------|
| Nativ | `log/aurago.log` (dazu `log/web_access.log`) |
| Docker | `docker compose logs -f` |
| Systemd | `journalctl -u aurago -f` |

`log/aurago.log`, `log/web_access.log` und `log/maintenance.log` sind nur für den AuraGo-Benutzer lesbar (`0600`); ein Log-Shipper unter einem anderen Benutzer braucht Gruppenzugriff, z. B. `chmod 640` auf diese Dateien plus eine mit dem Shipper geteilte Gruppe. Einen Modus ohne Zugriff für alle lässt AuraGo beim Neustart unverändert.

### Log-Level

AuraGo nutzt strukturiertes Logging mit diesen Leveln:

- `DEBUG` – Detaillierte Informationen für die Fehlersuche
- `INFO` – Allgemeine Betriebsinformationen
- `WARN` – Warnzustände
- `ERROR` – Fehlerzustände

### Häufige Log-Muster

```text
# Normaler Start
[INFO] AuraGo starting...
[INFO] Web UI available at http://localhost:8088
[INFO] Agent loop initialized

# LLM-API-Aufruf
[DEBUG] Sending request to LLM
[INFO] LLM response received

# Tool-Ausführung
[DEBUG] Dispatching tool: execute_shell
[INFO] Tool execute_shell completed

# Fehlermuster
[ERROR] Failed to connect to LLM API
[WARN] Retrying in 10s...
```

### Log-Level filtern

```bash
# Nur Fehler anzeigen
grep -i "error\|fatal" log/aurago.log

# Letzte 100 Zeilen mit Kontext
tail -100 log/aurago.log

# Echtzeit-Monitoring
tail -f log/aurago.log | grep -i "tool\|error"
```

## Debug-Modus

### Debug aktivieren

```text
Du: /debug on
Agent: ✅ Debug-Modus aktiviert
```

**Was zeigt Debug:**
- Vollständige Tool-Ausgaben
- Interne Fehlermeldungen
- API-Request/Response Details
- Timing-Informationen

### Debug-Anzeigen

| Anzeige | Bedeutung |
|---------|-----------|
| 🔍 Debug-Badge in der UI | Debug-Modus ist aktiv |
| Detaillierte Fehler-Traces | Vollständige Stack-Traces in Fehlern |
| Tool-Ausgabe sichtbar | Roh-Antworten der Tools sehen |
| Ausführliches Logging | Detailliertere Log-Einträge |

### Wann der Debug-Modus sinnvoll ist

**Debug-Modus verwenden bei:**
- Fehlersuche bei Tool-Fehlern
- Untersuchung unerwarteten Verhaltens
- Bug-Reports
- Entwicklung neuer Tools

**Debug-Modus nicht verwenden bei:**
- Produktivbetrieb (ausführliche Ausgaben)
- Geteilten Screenshots (können sensible Daten zeigen)
- Performance-kritischen Läufen (Mehraufwand)

### Debug wieder ausschalten

```text
Du: /debug off
```

## Diagnose-Befehle

### System-Info

```text
Du: Zeige System-Informationen
```

### Config-Validierung

```bash
./aurago -check-config
```

### Chat-basierte Diagnose

| Befehl | Zweck |
|--------|-------|
| `/help` | Alle verfügbaren Befehle auflisten |
| `/budget` | API-Nutzung und Kosten prüfen |
| `/debug on` | Detaillierte Ausgaben aktivieren |
| `/reset` | Möglicherweise korrumpierten Chat-Status leeren |
| `/stop` | Hängende Operationen abbrechen |

### Dateisystem-Diagnose

```bash
# Speicherplatz prüfen
df -h

# Größe des AuraGo-Verzeichnisses
du -sh ~/aurago

# Dateirechte prüfen
ls -la ~/aurago/data/

# Gesperrte Dateien finden
lsof +D ~/aurago/data/
```

### Netzwerk-Test

```bash
# LLM-Endpunkt erreichbar?
curl -I https://openrouter.ai/api/v1/models

# Lokaler Port offen?
netstat -tlnp | grep 8088
```

## Wiederherstellungsverfahren

### Notfall-Neustart

```bash
# 1. Graceful shutdown
pkill -TERM aurago
sleep 5

# 2. Falls hängt: Force kill
pkill -9 aurago

# 3. Locks entfernen
rm data/*.lock 2>/dev/null

# 4. Neustart
./aurago
```

### Chat-Verlauf zurücksetzen

> [!TIP]
> **Zerstörungsfrei:** Löscht nur die Konversation, das Gedächtnis bleibt

```text
/reset
```

Oder manuell löschen:
```bash
rm data/chat_history.json
rm data/short_term.db
```

### Chat-Verlauf konnte nicht geladen werden

Beim Start prüft AuraGo `data/chat_history.json`. Ist die Datei beschädigt (ungültiges JSON oder leer), wird sie nach `data/chat_history.json.corrupt-<Zeit>-<ID>` verschoben und ein neuer Verlauf beginnt; das Dashboard zeigt die Warnung „Chat history was unreadable and has been moved aside“. Die verschobene Datei bleibt für eine manuelle Wiederherstellung unverändert erhalten.

Kann die Datei gar nicht gelesen werden (zum Beispiel wegen fehlender Rechte oder einer Sperre durch ein anderes Programm), lässt AuraGo sie unverändert und speichert bis zum nächsten Start keine neuen Nachrichten; das Dashboard zeigt eine kritische Warnung. Rechte korrigieren oder das sperrende Programm schließen und AuraGo neu starten.

### Vault-Wiederherstellung

Falls `AURAGO_MASTER_KEY` verloren:

> [!WARNING]
> **Ohne Master-Key ist der Vault unwiderruflich verloren!:**

**Neuer Vault:**
```bash
# Alten Vault löschen (Daten sind verloren!)
mv data/vault.bin data/vault.bin.backup.$(date +%s)

# Neuen Key generieren
export AURAGO_MASTER_KEY=$(openssl rand -hex 32)
echo $AURAGO_MASTER_KEY > .env

# AuraGo startet mit neuem, leerem Vault
./aurago
```

### Kompletter Reset

```bash
# Alle Daten löschen (VORSICHT!)
rm -rf data/*
rm -rf agent_workspace/workdir/*
rm log/*.log

# Config behalten oder zurücksetzen:
# mv config.yaml config.yaml.backup
# cp agent_workspace/prompts/config.template.yaml config.yaml

# Neu starten
./aurago --setup
```

### Datenbank-Wiederherstellung

**SQLite-Korruption:**
```bash
# Datenbank-Integrität prüfen
sqlite3 data/short_term.db "PRAGMA integrity_check;"

# Bei Korruption aus Backup wiederherstellen
cp data/short_term.db.backup data/short_term.db

# Oder neu anlegen (Datenverlust)
rm data/short_term.db
# AuraGo neu starten — erstellt eine leere Datei
```

## Hilfe erhalten

### Informationen sammeln

Vor dem Melden eines Bugs:

```bash
# System-Info
echo "OS: $(uname -a)"
echo "AuraGo Version: $(./aurago --version 2>/dev/null || echo 'unknown')"
echo "Go Version: $(go version 2>/dev/null || echo 'not installed')"
echo "Python Version: $(python3 --version 2>/dev/null || echo 'not installed')"

# Letzte 50 Log-Zeilen
tail -50 log/aurago.log > problem-report.log
```

### Informationen beilegen

Beim Melden von Problemen bereitstellen:

```text
- AuraGo-Version: (./aurago --version)
- Betriebssystem: (uname -a)
- Installationsmethode: (Binary/Docker/Quelle)
- LLM-Provider: (OpenRouter/OpenAI/Ollama)
- Relevante config.yaml-Abschnitte (bereinigt)
- Fehlermeldungen aus den Logs
- Schritte zum Reproduzieren
```

### Community-Ressourcen

| Ressource | URL/Zugriff |
|-----------|-------------|
| GitHub Issues | github.com/antibyte/AuraGo/issues |
| Dokumentation | Ordner `documentation/` |
| Vorlage | `config_template.yaml` |

### GitHub-Issue-Vorlage

```markdown
**Beschreibung:**
Kurze Beschreibung des Problems

**Schritte zum Reproduzieren:**
1. Schritt eins
2. Schritt zwei
3. …

**Erwartetes Verhalten:**
Was passieren sollte

**Tatsächliches Verhalten:**
Was tatsächlich passiert

**Logs:**
```
Relevante Log-Zeilen hier einfügen
```text

**Umgebung:**
- OS: Ubuntu 22.04
- Version: v1.2.3
- Installation: Docker
- LLM: OpenRouter mit gpt-4
```

### Sicherheitsprobleme

> [!WARNING]
> **Für Sicherheitslücken KEINE öffentlichen Issues öffnen.:**

Kontakt: [Sicherheitskontakt-Informationen des Projekts]

---

## Zusätzliche Diagnosepunkte für aktuelle Features

| Bereich | Prüfen |
|---------|--------|
| Managed Ollama | `GET /api/ollama/managed/status`, Docker-Logs, GPU-Runtime, Modell-Volume |
| Security Proxy | `GET /api/proxy/status`, `GET /api/proxy/logs`, Caddyfile neu laden mit `POST /api/proxy/reload` |
| Web Push / PWA | HTTPS mit akzeptiertem Zertifikat oder localhost (self-signed LAN blockiert den Service Worker). Installierte App startet unter `/`. Nach Hintergrund/Freeze verbindet `AuraSSE` neu; nach Asset-Update einmal neu laden. `GET /api/push/status` |
| File KG Sync | `GET /api/debug/file-sync-status`, `GET /api/debug/kg-file-sync-stats`, verwaiste Nodes prüfen |
| Video Generation | Provider-Key im Vault, Tageslimit, Modellname, `POST /api/video-generation/test` |
| A2A | Agent Card, Auth-Modus, Remote-Agent-URL, aktivierte Bindings |
| Speech Lab / SIP | `/ready`-Snapshot, Ports 8765/8766, kein Cloud-Fallback |
| MeshCore | Companion verbunden? Operational Issues, nicht der normale Chat |
| go2rtc | Vault-Quellen, Container-Status, Viewer nur same-origin |
| Web-Assets | `./aurago --check-assets` — ungepinntes Binary bleibt Recovery |

Es gibt **keine** Fehlercodes `E001`–`E008`. Maßgeblich sind die Logzeile und das JSON-`error`-Feld. Vault-Fehler heißen fast immer falscher `AURAGO_MASTER_KEY`. `config.yaml` im Chat ist die Jail, kein Bug.

Bei schwer reproduzierbaren Fehlern zuerst Debug Mode, dann die Status-Endpunkte. Sidecars erklären mehr als der Chat allein.

---

> [!TIP]
> Die meisten Probleme lassen sich durch sorgfältiges Lesen der Logs lösen. Nimm dir Zeit, die Fehlermeldungen zu verstehen.

## Kurzreferenz

Es gibt keinen `E001`…`E008`-Katalog. Lies die Log-Zeile und das HTTP/JSON-`error`-Feld. Vault-Probleme bedeuten fast immer `AURAGO_MASTER_KEY`. Eine Recovery-Seite ohne Resource-ID deutet auf eine ungepinnt Binary hin. Chat kann `config.yaml` nicht lesen — das ist der Workspace-Jail.

> Logs, Toggles, dann der Resource-Satz. Neustart ist günstig; ein fehlender Master-Key ist es nicht.

---

**Vorheriges Kapitel:** [Kapitel 15: Co-Agenten](./15-coagents.md)
**Nächstes Kapitel:** [Kapitel 17: Glossar](./17-glossary.md)
