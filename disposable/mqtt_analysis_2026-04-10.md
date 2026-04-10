# MQTT-Integration Analyse und Behebungsplan

**Erstellt:** 2026-04-10  
**Status:** Analyse abgeschlossen

---

## 1. Zusammenfassung der Analyse

Die MQTT-Integration in AuraGo ist funktional und gut in das bestehende System integriert. Sie bietet:
- Publish/Subscribe/Unsubscribe/GetMessages als native Agent-Tools
- Mission-Trigger-Unterstützung für MQTT-basierte Automatisierung
- Ringbuffer für empfangene Nachrichten (500 Messages)
- RelayToAgent für Kontext-Weiterleitung an den Agenten
- ReadOnly-Modus für eingeschränkten Zugress

**Es wurden jedoch mehrere Probleme und Optimierungsmöglichkeiten identifiziert.**

---

## 2. Gefundene Probleme und Optimierungsmöglichkeiten

### 2.1 Fehlende TLS/SSL-Konfigurationsoptionen (HOCH)

**Problem:**
- Der MQTT-Client unterstützt nur unverschlüsselte TCP-Verbindungen
- Es gibt keine Möglichkeit, TLS-Zertifikate zu konfigurieren
- Das `broker`-Feld akzeptiert zwar `mqtts://`, aber ohne TLS-Config ist keine sichere Verbindung möglich
- Der Paho-Client bietet `SetTLSConfig()`, welches nicht genutzt wird

**Auswirkung:**
- Bei Verwendung von `mqtts://` mit einem selbst-signierten Zertifikat schlägt die Verbindung fehl
- Keine Möglichkeit, CA-Zertifikate zu spezifizieren
- Keine `InsecureSkipVerify`-Option für Tests

**Datei:** [`internal/mqtt/client.go`](internal/mqtt/client.go:106)

---

### 2.2 Fehlender MQTT-Passwort-Vault-Load (HOCH)

**Problem:**
- Das `Password`-Feld ist in `config_types.go` als `vault:"mqtt_password"` markiert
- In `config_migrate.go` wird `mqtt_password` behandelt
- ABER: In `config.go` wird nur `MQTT_PASSWORD` aus Environment gelesen, nicht aus der Vault

**Auswirkung:**
- Passwort wird nicht automatisch aus der Vault geladen
- Environment-Variable `MQTT_PASSWORD` funktioniert, aber Vault-Integration nicht

**Datei:** [`internal/config/config.go`](internal/config/config.go:857-860)

---

### 2.3 Fehlende Web-UI für MQTT-Management (MITTEL)

**Problem:**
- Keine dedizierte MQTT-Konfigurationsseite
- Kein MQTT-Dashboard-Widget mit Themen-Übersicht
- Kein Test-Connection-Button
- Keine Möglichkeit, aktive Subscriptions in der UI zu sehen
- Keine Live-Message-Anzeige

**Auswirkung:**
- Benutzer müssen MQTT über config.yaml oder API konfigurieren
- Keine visuelle Feedback-Möglichkeit
- Keine einfache Fehlerdiagnose

**Betroffene Dateien:**
- `ui/cfg/` - kein mqtt.js Konfigurationsmodul
- `ui/js/dashboard/main.js` - nur minimaler Status

---

### 2.4 Fehlende MQTT-API-Endpunkte (MITTEL)

**Problem:**
- Kein `/api/mqtt/status` Endpunkt
- Kein `/api/mqtt/test` Endpunkt
- Kein `/api/mqtt/publish` Endpunkt
- Kein `/api/mqtt/messages` Endpunkt

**Auswirkung:**
- UI muss `mqtt.IsConnected()` und `mqtt.BufferLen()` direkt aufrufen
- Keine saubere REST-Schnittstelle für MQTT-Operationen

**Betroffene Dateien:**
- [`internal/server/server_routes.go`](internal/server/server_routes.go) - keine MQTT-Handler

---

### 2.5 Buffer-Overflow-Risiko (NIEDRIG)

**Problem:**
- Der Ringbuffer hat ein fixes 500-Nachrichten-Limit (`maxBufferSize = 500`)
- Dieses Limit ist nicht konfigurierbar
- Bei hohem Nachrichtenaufkommen werden alte Nachrichten verworfen ohne Warnung

**Auswirkung:**
- Potentieller Datenverlust bei hoher Frequenz
- Keine Konfigurationsmöglichkeit für verschiedene Anwendungsfälle

**Datei:** [`internal/mqtt/client.go`](internal/mqtt/client.go:18)

---

### 2.6 Fehlende automatische Bereinigung des Buffers (NIEDRIG)

**Problem:**
- Der Buffer wächst bis 500 und verwirft dann alte Nachrichten
- Keine zeitbasierte Bereinigung (z.B. Nachrichten älter als X Stunden löschen)
- Keine manuelle Clear-Funktion

**Datei:** [`internal/mqtt/client.go`](internal/mqtt/client.go:20-32)

---

### 2.7 ReadOnly-Modell ist unvollständig (NIEDRIG)

**Problem:**
- `ReadOnly: true` blockiert `publish`, aber erlaubt `subscribe`, `unsubscribe`, `get_messages`
- QoS-Level werden akzeptiert aber nicht validiert (nur 0, 1, 2 erlaubt)
- Keine granularen Berechtigungen (subscribe-only, publish-only)

**Auswirkung:**
- In ReadOnly-Modus kann der Agent trotzdem neue Topics abonnieren
- Dieses Verhalten ist möglicherweise nicht beabsichtigt

**Datei:** [`internal/agent/agent_dispatch_infra.go`](internal/agent/agent_dispatch_infra.go:961-963)

---

### 2.8 Security-Hinweis wird nicht erzwungen (NIEDRIG)

**Problem:**
- In `security_check.go` wird eine Warnung ausgegeben wenn `RelayToAgent` ohne Auth aktiviert ist
- Diese Warnung ist jedoch nur informativ - die Verbindung wird trotzdem hergestellt
- Es gibt keinen Auto-Fix für dieses Sicherheitsproblem

**Auswirkung:**
- Sicherheitshinweis könnte vom Benutzer übersehen werden
- Keine erzwungene Konfiguration erforderlich

**Datei:** [`internal/server/security_check.go`](internal/server/security_check.go:410-417)

---

### 2.9 Python-Skill-Template nutzt eigene Broker-Logik (NIEDRIG)

**Problem:**
- Das `mqtt_publisher`-Skill-Template verwendet separate Umgebungsvariablen
- Es nutzt nicht die Hauptanwendungs-MQTT-Konfiguration
- `AURAGO_SECRET_MQTT_HOST`, `AURAGO_SECRET_MQTT_PORT`, etc. statt der Haupt-Config

**Auswirkung:**
- Inkonsistenz zwischen Go-MQTT-Client und Python-Skill
- Doppelte Konfiguration erforderlich

**Datei:** [`internal/tools/skill_templates.go`](internal/tools/skill_templates.go:1431-1435)

---

### 2.10 Connect-Timeout hardcoded (NIEDRIG)

**Problem:**
- Der Connect-Timeout ist auf 15 Sekunden hardcoded
- Keine Konfigurationsmöglichkeit

**Datei:** [`internal/mqtt/client.go`](internal/mqtt/client.go:132)

---

### 2.11 Fehlende Will-Message-Konfiguration (NIEDRIG)

**Problem:**
- Der Paho-Client unterstützt Last-Will-Testament (LWT) Messages
- Diese Funktion wird nicht genutzt

**Auswirkung:**
- Kein automatisches "AuraGo ist offline"-Signal an den Broker

---

## 3. Behebungsplan

### Phase 1: Kritische Fixes (Sollte sofort behoben werden)

| Priorität | ID | Problem | Aufwand | Dateien |
|-----------|-----|---------|---------|---------|
| HOCH | T1.1 | TLS-Konfiguration hinzufügen | Mittel | `internal/mqtt/client.go`, `internal/config/config_types.go` |
| HOCH | T1.2 | Vault-Integration für Passwort fixen | Niedrig | `internal/config/config.go` |

### Phase 2: Wichtige Verbesserungen (Nächste Sprint)

| Priorität | ID | Problem | Aufwand | Dateien |
|-----------|-----|---------|---------|---------|
| MITTEL | T2.1 | Web-UI MQTT-Konfigurationsseite | Hoch | `ui/cfg/mqtt.js`, `ui/mqtt.html`, `ui/js/config/main.js` |
| MITTEL | T2.2 | MQTT-API-Endpunkte | Mittel | `internal/server/server_routes.go` |
| MITTEL | T2.3 | MQTT-Dashboard-Widget mit Live-Messages | Mittel | `ui/js/dashboard/main.js` |

### Phase 3: Nice-to-have (Backlog)

| Priorität | ID | Problem | Aufwand | Dateien |
|-----------|-----|---------|---------|---------|
| NIEDRIG | T3.1 | Konfigurierbarer Buffer (max_messages + max_age) | Niedrig | `internal/mqtt/client.go` |
| NIEDRIG | T3.2 | ReadOnly-Modus überdenken (soll subscribe blockieren?) | Niedrig | `internal/agent/agent_dispatch_infra.go` |
| NIEDRIG | T3.3 | Connect-Timeout konfigurierbar machen | Niedrig | Beide Dateien |
| NIEDRIG | T3.4 | Last-Will-Message unterstützen | Niedrig | `internal/mqtt/client.go` |
| NIEDRIG | T3.5 | Python-Skill mit Haupt-Config synchronisieren | Mittel | `internal/tools/skill_templates.go` |

---

## 4. Detail-Implementierungspläne

### T1.1: TLS-Konfiguration hinzufügen

**Config-Erweiterung in `config_types.go`:**
```go
MQTT struct {
    // ... bestehende Felder ...
    TLS struct {
        Enabled           bool   `yaml:"enabled"`
        CAFile            string `yaml:"ca_file"`        // CA-Zertifikat
        CertFile          string `yaml:"cert_file"`      // Client-Zertifikat
        KeyFile           string `yaml:"key_file"`       // Client-Key
        InsecureSkipVerify bool  `yaml:"insecure_skip_verify"` // Für Tests
    } `yaml:"tls"`
}
```

**Client-Änderungen in `client.go`:**
- TLS-Config aus Config lesen
- `tls.Config` erstellen
- `opts.SetTLSConfig(tlsConfig)` aufrufen

---

### T1.2: Vault-Integration für Passwort

**In `config.go` nach dem bestehenden MQTT-Block hinzufügen:**
```go
// MQTT: Load password from vault if not set via env
if cfg.MQTT.Password == "" {
    if val := os.Getenv("MQTT_PASSWORD"); val != "" {
        cfg.MQTT.Password = val
    } else if vault != nil {
        // Try to load from vault
        if secret, err := vault.Get("mqtt_password"); err == nil && secret != "" {
            cfg.MQTT.Password = secret
        }
    }
}
```

---

### T2.1: Web-UI MQTT-Konfigurationsseite

**Neue Dateien:**
- `ui/cfg/mqtt.js` - Konfigurationsmodul
- `ui/mqtt.html` - MQTT-Management-Seite

**Integration in:**
- `ui/js/config/main.js` - Navigation hinzufügen
- `ui/lang/config/sections/*.json` - Übersetzungen

**Features:**
- Broker-URL mit Test-Connection
- Username/Password (via Vault)
- TLS-Konfiguration
- Themen-Liste mit Wildcard-Support
- QoS-Auswahl
- ReadOnly-Toggle
- RelayToAgent-Toggle
- Live-Message-Monitor

---

### T2.2: MQTT-API-Endpunkte

**Neue Endpunkte in `server_routes.go`:**
```
GET  /api/mqtt/status     - Connection status, buffer size
POST /api/mqtt/test       - Test broker connection
GET  /api/mqtt/messages  - Get buffered messages
POST /api/mqtt/publish    - Publish a message (for testing)
POST /api/mqtt/clear      - Clear message buffer
```

---

### T3.1: Konfigurierbarer Buffer

**Config-Erweiterung:**
```go
Buffer struct {
    MaxMessages int `yaml:"max_messages"` // default: 500
    MaxAgeHours int `yaml:"max_age_hours"` // default: 0 (disabled)
} `yaml:"buffer"`
```

**Buffer-Änderungen:**
- `maxBufferSize` durch Config ersetzen
- Zeitbasierte Bereinigung in `messageHandler` oder periodischem Goroutine

---

## 5. Testing-Empfehlungen

1. **TLS-Tests:**
   - Verbindung mit selbst-signiertem Zertifikat
   - Verbindung mit CA-signiertem Zertifikat
   - `InsecureSkipVerify`-Modus

2. **Vault-Tests:**
   - Passwort aus Vault laden
   - Fallback zu Environment-Variable

3. **Buffer-Tests:**
   - Buffer-Overflow bei 500+ Nachrichten
   - Zeitbasierte Bereinigung

4. **ReadOnly-Tests:**
   - subscribe/unsubscribe/get in ReadOnly
   - publish in ReadOnly sollte fehlschlagen

---

## 6. Übersetzungen (i18n)

Folgende neue Übersetzungs-Keys werden benötigt:
- `mqtt.*` in allen 16 Sprachen
- `dashboard.mqtt_*` in allen Sprachen
- `config.section.mqtt.*` - bereits vorhanden

---

## 7. Dokumentation

- `prompts/tools_manuals/mqtt.md` - TLS-Felder hinzufügen
- `documentation/manual/*/08-integrations.md` - TLS-Konfiguration dokumentieren
- `config_template.yaml` - TLS-Sektion dokumentieren

---

## 8. Breaking Changes

Keine Breaking Changes erwartet - alle Änderungen sind additive Erweiterungen.
