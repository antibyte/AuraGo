# Homepage Tool - Analyse & Reparaturplan

## 1. PROBLEMBESCHREIBUNG

### 1.1 Docker Connection Problem
- **Fehler**: `dial unix /var/run/docker.sock: connect: connection refused`
- **Ursache**: Docker Daemon läuft nicht oder ist nicht erreichbar
- **Auswirkung**: Alle Container-Operationen (init, start, rebuild, webserver_start) schlagen fehl

### 1.2 Fehlende Fallback-Mechanismen
- Keine Alternative wenn Docker nicht verfügbar ist
- Keine automatische Prüfung der Docker-Verfügbarkeit
- Keine Nutzung von Python/lighttpd/Node.js als Ersatz

### 1.3 Unklare Fehlermeldungen
- Status-Meldungen sind nicht aussagekräftig
- Keine Diagnose, warum Docker nicht funktioniert
- Keine Hinweise auf Alternativen

### 1.4 Caddy-Image Abhängigkeit
- `publish_local` benötigt `caddy:alpine` Image
- Wenn Docker nicht verfügbar, kann nicht deployt werden

## 2. AKTUELLER STATUS

### Funktionierende Teile:
- ✅ `status` - Gibt Container-Status zurück
- ✅ Datei-Operationen (read_file, write_file, list_files)
- ✅ Einfache HTML-Dateien existieren und sind lesbar

### Nicht funktionierende Teile:
- ❌ Docker-basierte Container (init, start, rebuild)
- ❌ Caddy Web-Server (fehlendes Image)
- ❌ Framework-Entwicklung (Next.js, Vite, etc.)

### Workaround (manuell):
```bash
python3 -m http.server 8080 --directory /workspace/
```

## 3. REPRODUKTIONSSCHRITTE

```bash
# Test 1: Docker Status
docker ps
# → "Cannot connect to the Docker daemon"

# Test 2: Homepage init
{"operation": "init"}
# → FEHLER: Docker connection refused

# Test 3: Python Fallback (funktioniert)
python3 -m http.server 8080
# → Server startet erfolgreich
```

## 4. LÖSUNGSPLAN

### Priorität 1: Docker-Verfügbarkeitsprüfung
```go
func checkDockerAvailable(dockerCfg DockerConfig) bool {
    _, _, err := dockerRequest(dockerCfg, "GET", "/version", "")
    return err == nil
}
```

### Priorität 2: Fallback-Mechanismus
Wenn Docker nicht verfügbar:
1. Python HTTP Server nutzen (falls Python verfügbar)
2. Node.js http-server nutzen (falls Node verfügbar)
3. Fehlermeldung mit Diagnose

### Priorität 3: Verbesserte Fehlermeldungen
- Diagnose: Ist Docker installiert? Läuft der Service?
- Klare Anweisungen: `systemctl start docker`
- Alternative Vorschläge

### Priorität 4: publish_local ohne Caddy
- Nutze Python/Node.js Server statt Caddy
- Automatische Port-Verfügbarkeitsprüfung
- Direkte URL-Ausgabe

## 5. IMPLEMENTIERUNG

### Phase 1: Docker Check
- `HomepageInit` prüft zuerst Docker-Verfügbarkeit
- Wenn nicht verfügbar: Python-Server starten

### Phase 2: Python Server Integration
```go
func startPythonServer(port int, directory string) (string, error) {
    cmd := exec.Command("python3", "-m", "http.server", 
        strconv.Itoa(port), "--directory", directory)
    err := cmd.Start()
    return fmt.Sprintf("http://localhost:%d", port), err
}
```

### Phase 3: Status-Verbesserung
- Zeigt Docker-Status an
- Zeigt Web-Server-Status an (egal ob Docker oder Python)
- Zeigt verfügbare Frameworks an

## 6. AKZEPTANZKRITERIEN

- [ ] `homepage init` funktioniert ohne Docker (mit Python)
- [ ] `homepage status` zeigt klar an welcher Server läuft
- [ ] `publish_local` gibt erreichbare URL zurück
- [ ] Fehlermeldungen enthalten Diagnose und Lösungsvorschläge
- [ ] Automatische Fallback-Wahl (Docker → Python → Node.js)
