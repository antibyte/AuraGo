# Kapitel 16: Troubleshooting

Dieses Kapitel hilft dir bei häufigen Problemen und deren Lösungen.

## Systematische Fehlersuche

Bevor du einzelne Lösungen ausprobierst, gehe systematisch vor:

1. **Logs prüfen** – In `log/supervisor.log` stehen meist die Antworten
2. **Kürzliche Änderungen** – Was wurde zuletzt geändert?
3. **Isolation testen** – Tritt das Problem nur bei bestimmten Aktionen auf?
4. **Schrittweise einschränken** – Ein Feature nach dem anderen deaktivieren

## Installationsprobleme

### Binary startet nicht

| Symptom | Ursache | Lösung |
|---------|---------|--------|
| `permission denied` | Keine Ausführungsrechte | `chmod +x aurago` |
| `cannot execute binary` | Falsche Architektur | Richtige Binary für dein System downloaden |
| `no such file or directory` | Fehlende Bibliotheken | Statisch gelinkte Binary verwenden oder Abhängigkeiten installieren |

### Resources.dat nicht gefunden

```
Fehler: resources.dat not found in working directory
```

**Lösung:**
```bash
# Stelle sicher, dass resources.dat im gleichen Verzeichnis liegt
ls -la aurago resources.dat

# Falls nicht, kopiere es
cp /pfad/zu/resources.dat ./
```

### Python venv Fehler

```
Fehler: failed to create virtual environment
```

**Lösung:**
```bash
# Python 3.9+ installieren
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

> ⚠️ **Warnung:** `0.0.0.0` exponiert die UI im LAN. Nie ohne Auth im Internet nutzen!

### Telegram Bot reagiert nicht

```
Symptom: Keine Antwort auf Nachrichten
```

**Checkliste:**
1. Bot-Token korrekt in `config.yaml`?
2. Telegram-User-ID korrekt? (Mit @userinfobot prüfen)
3. AuraGo neu gestartet nach Config-Änderung?
4. Bot bei @BotFather gestartet? (/start)

**Logs prüfen:**
```bash
tail -f log/supervisor.log | grep -i telegram
```

## LLM/API-Fehler

### Authentifizierung fehlgeschlagen

```
Fehler: 401 Unauthorized - Invalid API key
```

**Lösung:**
```bash
# API-Key direkt testen
curl -H "Authorization: Bearer DEIN-KEY" \
  https://openrouter.ai/api/v1/models

# In config.yaml prüfen
nano config.yaml
# llm.api_key muss korrekt sein
```

### Rate Limiting (429)

```
Fehler: 429 Too Many Requests
```

**Lösungen:**
- `agent.step_delay_seconds` in config.yaml erhöhen
- Anderes/fasteres Modell verwenden
- Budget-Tracking aktivieren für Überblick

### Modell nicht gefunden (404)

```
Fehler: 404 Model not found
```

**Lösung:** Modell-ID in `llm.model` prüfen. Bei OpenRouter: Im Dashboard verifizieren.

## Gedächtnis- und Performance-Probleme

### Hoher RAM-Verbrauch

| Ursache | Lösung |
|---------|--------|
| Große Vektordatenbank | `embeddings.provider: disabled` testweise |
| Langer Chat-Verlauf | `/reset` oder `memory_compression_char_limit` senken |
| Memory Leak | AuraGo neustarten, Bug reporten |

### Datenbank-Locks

```
Fehler: database is locked
```

**Lösung:**
```bash
# Prozess beenden und Lock entfernen
pkill aurago
rm data/*.lock 2>/dev/null
./aurago
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

### Permission denied auf Docker-Socket

```
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

## Logs effektiv lesen

### Log-Dateien

| Datei | Inhalt |
|-------|--------|
| `log/supervisor.log` | Haupt-Anwendungslog |
| `log/agent.log` | Agent-spezifische Aktionen |
| `log/http.log` | Web-UI Zugriffe |

### Log-Level filtern

```bash
# Nur Fehler anzeigen
grep -i "error\|fatal" log/supervisor.log

# Letzte 100 Zeilen mit Kontext
tail -100 log/supervisor.log

# Echtzeit-Monitoring
tail -f log/supervisor.log | grep -i "tool\|error"
```

## Debug-Modus

### Debug aktivieren

```
Du: /debug on
Agent: ✅ Debug-Modus aktiviert
```

**Was zeigt Debug:**
- Vollständige Tool-Ausgaben
- Interne Fehlermeldungen
- API-Request/Response Details
- Timing-Informationen

### Debug wieder ausschalten

```
Du: /debug off
```

## Diagnose-Befehle

### System-Info

```
Du: Zeige System-Informationen
```

### Config-Validierung

```bash
./aurago --validate-config
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

### Vault-Wiederherstellung

Falls `AURAGO_MASTER_KEY` verloren:

> ⚠️ **Ohne Master-Key ist der Vault unwiderruflich verloren!**

**Neuer Vault:**
```bash
# Alten Vault löschen (Daten sind verloren!)
mv data/secrets.vault data/secrets.vault.backup.$(date +%s)

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
tail -50 log/supervisor.log > problem-report.log
```

### Community-Ressourcen

| Ressource | URL/Zugriff |
|-----------|-------------|
| GitHub Issues | github.com/antibyte/AuraGo/issues |
| Dokumentation | Ordner `documentation/` |
| Beispiel-Configs | `agent_workspace/prompts/` |

---

> 💡 **Tipp:** Die meisten Probleme lassen sich durch sorgfältiges Lesen der Logs lösen. Nimm dir Zeit, die Fehlermeldungen zu verstehen.
