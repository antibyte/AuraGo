# Kapitel 14: Sicherheit

Dieses Kapitel behandelt alle Sicherheitsaspekte von AuraGo – vom verschlüsselten Vault bis zur Zwei-Faktor-Authentifizierung. Für Produktivumgebungen sind diese Einstellungen essenziell.

> ⚠️ **Kritisch:** AuraGo führt Code auf deinem System aus. Eine korrekte Sicherheitskonfiguration ist nicht optional, sondern zwingend erforderlich.

---

## Der AES-256 Vault

Der Vault ist das Herzstück der AuraGo-Sicherheitsarchitektur. Hier werden alle sensiblen Daten – API-Keys, Passwörter, Tokens – verschlüsselt gespeichert.

### Technische Details

```
┌─────────────────────────────────────────────────────────┐
│                    AES-256-GCM Vault                    │
├─────────────────────────────────────────────────────────┤
│  • Verschlüsselung: AES-256 im GCM-Modus               │
│  • Schlüssellänge: 256 Bit (64 Hex-Zeichen)            │
│  • Nonce: 96 Bit, pro Operation zufällig generiert     │
│  • Authentifizierung: Galois/Counter Mode (AEAD)       │
│  • Datei-Locking: Verhindert parallele Zugriffe         │
└─────────────────────────────────────────────────────────┘
```

### Was wird im Vault gespeichert?

| Kategorie | Beispiele |
|-----------|-----------|
| **API-Keys** | OpenRouter, OpenAI, Google Workspace |
| **Zugangsdaten** | SMTP/IMAP-Passwörter, Datenbank-Passwörter |
| **Token** | OAuth-Refresh-Tokens, Bot-Tokens |
| **Intern** | Master-Key-Derivate, Session-Secrets |

> 💡 **Tipp:** Der Vault wird in `data/vault.bin` gespeichert. Das Master-Passwort (64 Hex-Zeichen) wird aus der Umgebungsvariable `AURAGO_MASTER_KEY` gelesen.

### Agent-Zugriff auf den Vault

> 🔒 **Wichtiger Sicherheitshinweis:**
> 
> Der Agent hat **niemals** direkten Zugriff auf die im Vault gespeicherten Geheimnisse bei internen Integrationen und Tools. Er kann diese weder lesen noch abrufen.
> 
> **Ausnahme:** Geheimnisse, die der Agent **selbst angelegt** hat (z.B. über den Chat oder die API), kann er jederzeit abrufen und verwalten.
> 
> Die Anwendung (nicht der Agent) lädt Vault-Secrets zur Laufzeit und injiziert sie sicher in die entsprechenden Tools, ohne dass der Agent jemals die Klartext-Werte sieht.

### Vault initialisieren

```bash
# Master-Key generieren (nur einmal)
openssl rand -hex 32
# Ausgabe: a1b2c3d4e5f6... (64 Zeichen)

# In Umgebungsvariable setzen
export AURAGO_MASTER_KEY="dein-generierter-key"

# AuraGo starten – Vault wird automatisch erstellt
./aurago
```

---

## LLM Guardian – KI-gestützte Sicherheitsüberwachung

Der LLM Guardian überwacht alle Aktionen des Agenten und schützt vor potenziell gefährlichen oder unerwünschten Aktionen. Er fungiert als unabhängige Sicherheitsebene zwischen dem Agenten und der Tool-Ausführung.

### Funktionsweise

```
┌─────────────────────────────────────────────────────────────┐
│                     LLM Guardian Flow                       │
├─────────────────────────────────────────────────────────────┤
│  1. Agent generiert Tool-Call                               │
│  2. Guardian analysiert den Call mit dediziertem LLM        │
│  3. Risiko-Score wird berechnet (0-100)                     │
│  4. Bei hohem Risiko: Blockierung oder Warnung              │
│  5. Nur bei niedrigem Risiko: Tool wird ausgeführt          │
└─────────────────────────────────────────────────────────────┘
```

### Was wird überwacht?

| Kategorie | Beispiele |
|-----------|-----------|
| **Tool-Calls** | Gefährliche Shell-Befehle, Datenlöschung, Systemänderungen |
| **Externe Daten** | Prompt Injection Versuche, bösartige Inhalte |
| **Dateien** | Malware, verdächtige Anhänge |
| **Emails** | Phishing-Versuche, Suspicious Content |

### Konfiguration

```yaml
llm_guardian:
  enabled: false                     # Guardian aktivieren
  provider: ""                       # Provider-ID (Referenz auf providers-Eintrag)
  model: ""                          # Modell-Override (leer = Provider-Default)
  default_level: "medium"            # Standard-Prüfstufe
  fail_safe: "quarantine"            # Verhalten bei Guardian-Fehler: "quarantine" | "allow"
  cache_ttl: 300                     # Cache-Gültigkeit in Sekunden
  max_checks_per_min: 60             # Max Prüfungen pro Minute
  tool_overrides: {}                 # Tool-spezifische Level-Overrides
  allow_clarification: false         # Rückfragen beim User erlauben
  scan_documents: false              # Hochgeladene Dokumente scannen
  scan_emails: false                 # Eingehende E-Mails scannen
```

> 💡 **Hinweis:** Der LLM Guardian nutzt das Provider-System. Erstelle einen eigenen Provider-Eintrag für den Guardian und referenziere ihn über die `provider`-ID.

### Prüfstufen

| Stufe | Beschreibung | Anwendungsfall |
|-------|--------------|----------------|
| `low` | Grundlegende Prüfung | Entwicklung, Testing |
| `medium` | Standard-Prüfung (Default) | Normale Nutzung |
| `high` | Verstärkte Prüfung | Produktivumgebungen |
| `strict` | Maximale Prüfung | Hochsichere Umgebungen |

### Fail-Safe-Verhalten

| Wert | Beschreibung |
|------|-------------|
| `quarantine` | Bei Guardian-Fehler wird der Tool-Call blockiert (Default) |
| `allow` | Bei Guardian-Fehler wird der Tool-Call erlaubt |

### Best Practices

- Verwende einen **dedizierten Provider** für den Guardian (nicht das Haupt-LLM)
- Wähle ein **schnelles, kostengünstiges Modell** für Echtzeit-Scans
- Konfiguriere **Ausnahmen** für häufige, unbedenkliche Operationen
- Überwache die **Guardian-Logs** auf verdächtige Muster

---

## Sudo Execution – Privilegierte Befehle

AuraGo unterstützt die Ausführung von Befehlen mit erhöhten Rechten (sudo). Das Sudo-Passwort wird über den `/sudopwd` Befehl sicher im Vault gespeichert.

### Sicherheitskonzept

```
┌─────────────────────────────────────────────────────────────┐
│                   Sudo Execution Flow                       │
├─────────────────────────────────────────────────────────────┤
│  1. Benutzer setzt Sudo-Passwort mit /sudopwd               │
│  2. Passwort wird sicher im Vault gespeichert               │
│  3. Passwort ist nie im Chat oder Logs sichtbar             │
│  4. Befehle werden mit sudo ausgeführt                      │
│  5. Automatischer Timeout nach konfigurierbarer Zeit        │
└─────────────────────────────────────────────────────────────┘
```

### Einrichtung

#### Schritt 1: Sudo-Passwort im Vault speichern

```bash
# Im Chat eingeben
/sudopwd

# Oder direkt im Vault unter dem Schlüssel "sudo_password"
```

#### Schritt 2: Konfiguration in config.yaml

```yaml
agent:
  sudo_enabled: true           # Sudo-Execution erlauben
  sudo_timeout_minutes: 10     # Timeout nach 10 Minuten
```

### Verwendung

```
Du: /sudopwd
Agent: 🔐 Sudo-Passwort wurde sicher im Vault gespeichert.

Du: Installiere das Paket nginx
Agent: 🛠️ Shell: sudo apt install nginx -y
       ✅ Paket erfolgreich installiert
```

### Sicherheitshinweise

> ⚠️ **Wichtig:**
> - Das Sudo-Passwort wird **niemals** im Chat angezeigt oder geloggt
> - Es ist nur im verschlüsselten Vault gespeichert
> - Der Sudo-Modus hat einen **automatischen Timeout**
> - Jede Sudo-Ausführung wird im **Journal protokolliert**
> - Kombiniere Sudo mit dem **LLM Guardian** für zusätzliche Sicherheit

### Read-Only Modus

Für besonders sensible Systeme kann Sudo komplett deaktiviert werden:

```yaml
agent:
  sudo_enabled: false          # Keine Sudo-Ausführung erlaubt
```

In diesem Fall werden sudo-Befehle mit einer Fehlermeldung abgelehnt.

---

## Web UI Authentication

AuraGo bietet ein mehrschichtiges Authentifizierungssystem für die Web-Oberfläche.

### Übersicht der Auth-Methoden

| Methode | Sicherheit | Anwendungsfall |
|---------|------------|----------------|
| **Passwort (bcrypt)** | Hoch | Basisschutz der Web-UI |
| **TOTP 2FA** | Sehr hoch | Zusätzliche Sicherheit für externen Zugriff |
| **Session-Cookies** | Hoch | Stateless-Authentifizierung |
| **IP-Rate-Limiting** | Mittel | Schutz gegen Brute-Force |

### bcrypt-Passwort-Hashing

- **Algorithmus:** bcrypt mit Cost-Faktor 12
- **Salt:** Automatisch, pro Passwort eindeutig
- **Zeitaufwand:** ~250ms pro Hashing-Operation (Schutz gegen Rainbow Tables)

### TOTP (Time-based One-Time Password)

- **Standard:** RFC 6238 / RFC 4226
- **Zeitfenster:** 30 Sekunden
- **Toleranz:** ±1 Fenster (90 Sekunden Grace Period)
- **QR-Code:** Kompatibel mit Google Authenticator, Authy, Bitwarden

---

## Passwort-Schutz einrichten

### Schritt 1: Erstkonfiguration

```yaml
# config.yaml
auth:
    enabled: true
    password_hash: ""              # Wird beim ersten Login gesetzt
    session_secret: ""             # Wird automatisch generiert
    session_timeout_hours: 24
    totp_secret: ""
    totp_enabled: false
    max_login_attempts: 5
    lockout_minutes: 15
```

### Schritt 2: Passwort über die Web-UI setzen

1. Starte AuraGo mit `auth.enabled: true`
2. Öffne die Web-UI (`http://localhost:8088`)
3. Du wirst automatisch zur Einrichtungsseite weitergeleitet
4. Wähle ein starkes Passwort (min. 12 Zeichen, gemischte Zeichensätze)

> ⚠️ **Warnung:** Ohne gesetztes Passwort wird die Authentifizierung automatisch deaktiviert, um einen Lockout zu verhindern!

### Passwort manuell hashen (CLI)

Falls du das Passwort direkt in der `config.yaml` setzen möchtest:

```bash
# Mit Python
python3 -c "import bcrypt; print(bcrypt.hashpw(b'dein-passwort', bcrypt.gensalt(12)).decode())"

# Mit Node.js
node -e "const bcrypt = require('bcrypt'); console.log(bcrypt.hashSync('dein-passwort', 12))"
```

---

## Zwei-Faktor-Authentifizierung (2FA) aktivieren

### Voraussetzungen

- Passwort-Authentifizierung ist eingerichtet
- TOTP-App installiert (Google Authenticator, Authy, etc.)

### Einrichtung

1. **In der Config aktivieren:**
   ```yaml
   auth:
       totp_enabled: true
       totp_secret: ""              # Wird automatisch generiert
   ```

2. **AuraGo neu starten**

3. **QR-Code scannen:**
   - Öffne `/auth/setup` in der Web-UI
   - Scan den angezeigten QR-Code mit deiner Authenticator-App
   - Gib den 6-stelligen Code zur Bestätigung ein

### Wichtige Hinweise zur 2FA

> ⚠️ **Backup-Code speichern!** Wenn du den Zugang zu deinem Authenticator verlierst, kannst du dich ohne das TOTP-Secret nicht mehr anmelden. Exportiere das Secret aus der Config oder sichere es separat.

```yaml
# Backup: Diesen Wert sicher aufbewahren!
auth:
    totp_secret: "JBSWY3DPEHPK3PXP"    # Base32-kodiertes Secret
```

---

## Danger Zone – Capability Gates

Die Danger Zone kontrolliert, welche potenziell gefährlichen Fähigkeiten der Agent besitzen darf. Jede Funktion kann einzeln auf **Nur-Lesen** oder **Deaktiviert** gesetzt werden.

### Tool-spezifische Gates

```yaml
# tools: Sektion in config.yaml
tools:
    memory:
        enabled: true
        read_only: false           # Agent darf Memories nicht ändern
    
    knowledge_graph:
        enabled: true
        read_only: false
    
    secrets_vault:
        enabled: true
        read_only: false           # ❌ Empfohlen: true
    
    scheduler:
        enabled: true
        read_only: false
    
    notes:
        enabled: true
        read_only: false
    
    missions:
        enabled: true
        read_only: false
    
    stop_process:
        enabled: true              # Prozesse beenden erlaubt?
    
    inventory:
        enabled: true
    
    memory_maintenance:
        enabled: true              # Archivierung, Optimierung
```

### Integration-spezifische Gates

| Integration | Read-Only Option | Beschreibung |
|-------------|------------------|--------------|
| **Docker** | `docker.read_only` | Keine Container-Start/Stop-Operationen |
| **Home Assistant** | `home_assistant.read_only` | Nur Gerätestatus lesen, nicht steuern |
| **Email** | `email.read_only` | Kein Versand von E-Mails |
| **Discord** | `discord.read_only` | Nur lesen, keine Nachrichten senden |

> 💡 **Best Practice:** Starte mit `read_only: true` für alle kritischen Integrationen und aktiviere Schreibzugriff nur bei Bedarf.

---

## Read-only vs Read-write Modus

### Vergleich

| Aspekt | Read-only | Read-write |
|--------|-----------|------------|
| **Sicherheit** | 🔒 Sehr hoch | 🔓 Erhöhtes Risiko |
| **Nutzen** | Begrenzt | Vollständig |
| **Einsatz** | Monitoring, Recherche | Automatisierung, Steuerung |
| **Fehlermeldung** | "Tool is in read-only mode" | – |

### Granulare Kontrolle

```yaml
# Beispiel: Sichere Konfiguration für Monitoring
docker:
    enabled: true
    read_only: true              # Container anzeigen, aber nicht ändern

home_assistant:
    enabled: true
    read_only: true              # Temperatur lesen, Heizung nicht steuern

email:
    enabled: true
    read_only: true              # E-Mails lesen, nicht senden

# Der Agent kann trotzdem:
# - Dateien im Workspace lesen/schreiben
# - Web-Searches durchführen
# - Python-Code ausführen
```

---

## File Locks und Instance Prevention

AuraGo verwendet mehrere Locking-Mechanismen, um Datenkorruption und Race Conditions zu verhindern.

### Vault File Lock

```go
// Interner Mechanismus
fileLock := flock.New(vaultFile + ".lock")
// Sperrt während Lese-/Schreiboperationen
```

- **Datei:** `data/vault.bin.lock`
- **Funktion:** Verhindert gleichzeitige Vault-Zugriffe
- **Timeout:** Kein Timeout – blockiert bis zur Freigabe

### Instance Lock (Lifeboat)

```
┌─────────────────┐
│  lifeboat.lock  │  ← Verhindert parallele AuraGo-Instanzen
└─────────────────┘
```

- **Datei:** `lifeboat.lock` im Projektverzeichnis
- **Zweck:** Verhindert, dass mehrere AuraGo-Prozesse gleichzeitig laufen
- **Neustart:** Wird beim sauberen Beenden automatisch aufgeräumt

> ⚠️ **Achtung:** Falls AuraGo abstürzt, kann die Lock-Datei bestehen bleiben. Lösche sie manuell: `rm lifeboat.lock`

---

## Rate Limiting konfigurieren

Das integrierte Rate Limiting schützt vor Brute-Force-Angriffen auf den Login.

### Standard-Konfiguration

```yaml
auth:
    max_login_attempts: 5          # Fehlversuche vor Lockout
    lockout_minutes: 15            # Sperrdauer in Minuten
```

### Funktionsweise

| Versuch | Aktion |
|---------|--------|
| 1–4 | Fehler wird gezählt, Login weiter möglich |
| 5 | Account wird für 15 Minuten gesperrt |
| Nach Sperre | Zähler wird zurückgesetzt |

### Lockout-Status prüfen

```bash
# Im Log erkennbar:
# [Auth] IP 192.168.1.100 locked out until 2026-03-08T14:30:00
```

> 💡 **Tipp:** Bei Reverse-Proxy-Einsatz (nginx, Traefik) wird die `X-Forwarded-For`-Header-IP verwendet.

---

## Security Best Practices

### Checkliste für Produktivumgebungen

- [ ] **Master-Key** ist 64 zufällige Hex-Zeichen
- [ ] **Passwort** ist mindestens 12 Zeichen lang
- [ ] **2FA** ist aktiviert bei externem Zugriff
- [ ] **HTTPS** wird verwendet (nicht HTTP)
- [ ] **VPN** für externen Zugriff (WireGuard, Tailscale)
- [ ] **Firewall** blockiert Port 8088 von außen
- [ ] **Read-only Gates** für sensible Integrationen
- [ ] **Rate Limiting** ist aktiv
- [ ] **Logging** ist aktiviert und überwacht
- [ ] **Backups** des Vault-Keys existieren

### Netzwerksicherheit

```
┌─────────────────────────────────────────────────────────┐
│                     Internet                            │
└────────────────┬────────────────────────────────────────┘
                 │ VPN (WireGuard/Tailscale)
                 ▼
┌─────────────────────────────────────────────────────────┐
│  Reverse Proxy (nginx/Traefik)                          │
│  • TLS-Terminierung                                     │
│  • Zusätzliche Auth-Layer                               │
└────────────────┬────────────────────────────────────────┘
                 │ LAN
                 ▼
┌─────────────────────────────────────────────────────────┐
│  AuraGo (127.0.0.1:8088)                                │
│  • Interne Auth + 2FA                                   │
│  • Vault verschlüsselt                                  │
└─────────────────────────────────────────────────────────┘
```

---

## Vault Management

### Vault zurücksetzen

> ⚠️ **Warnung:** Dies löscht ALLE gespeicherten Secrets unwiderruflich!

```bash
# 1. AuraGo stoppen
pkill aurago

# 2. Vault-Datei löschen
rm data/vault.bin

# 3. Optional: Neuen Master-Key generieren
export AURAGO_MASTER_KEY="$(openssl rand -hex 32)"

# 4. AuraGo neu starten
./aurago
```

### Vault-Schlüssel rotieren

```bash
# 1. Aktuellen Vault entschlüsseln (mit altem Key)
export AURAGO_MASTER_KEY="alter-key"
./aurago --export-vault > vault_backup.json

# 2. Neuen Key setzen
export AURAGO_MASTER_KEY="neuer-key"

# 3. Vault neu importieren
./aurago --import-vault vault_backup.json
```

### Vault-Status prüfen

```bash
# Über die Web-UI
GET /api/vault/status

# Response:
{
    "encrypted": true,
    "key_count": 12,
    "last_modified": "2026-03-08T10:30:00Z"
}
```

---

## Incident Response

### Szenario 1: Verdacht auf Kompromittierung

```bash
# Sofortmaßnahmen:

1. AuraGo stoppen
   pkill aurago

2. Netzwerkverbindung trennen (bei VM: Netzwerk-Adapter deaktivieren)

3. Logs sichern
   cp -r log/ incident_logs_$(date +%Y%m%d)/

4. Vault-Datei prüfen (Zeitstempel)
   ls -la data/vault.bin

5. Master-Key rotieren (siehe oben)
```

### Szenario 2: Lockout (Passwort vergessen)

```bash
# Wenn du das Passwort vergessen hast:

1. AuraGo stoppen
   pkill aurago

2. Auth in config.yaml deaktivieren
   sed -i 's/enabled: true/enabled: false/' config.yaml

3. AuraGo starten und neues Passwort setzen
   ./aurago
   # → Web-UI öffnen → /auth/setup

4. Auth wieder aktivieren
```

### Szenario 3: TOTP verloren

```bash
# Ohne Backup des TOTP-Secrets:

1. Config editieren
   # Setze: totp_enabled: false

2. AuraGo neu starten

3. 2FA neu einrichten (QR-Code wird neu generiert)
```

### Log-Analyse

```bash
# Auth-bezogene Ereignisse filtern
grep "\[Auth\]" log/aurago.log

# Muster für Angriffsversuche
grep "failed login\|locked out\|unauthorized" log/aurago.log

# Vault-Zugriffe
grep "vault\|secret" log/aurago.log | grep -v "secret_key"
```

---

## 🔍 Deep Dive: Kryptographische Implementierung

### AES-256-GCM Details

```go
// Vereinfachte Darstellung
block, _ := aes.NewCipher(key)      // 256-Bit Schlüssel
gcm, _ := cipher.NewGCM(block)      // GCM-Modus
nonce := make([]byte, gcm.NonceSize()) // 96-Bit Nonce
io.ReadFull(rand.Reader, nonce)     // Kryptographisch sicher

ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
```

**Sicherheitseigenschaften:**
- **Vertraulichkeit:** AES-256 Verschlüsselung
- **Integrität:** GCM-Authentifizierung (AEAD)
- **Non-Replay:** Eindeutiger Nonce pro Operation
- **Forward Secrecy:** Keine implizite Key-Derivation

### Session-Cookie Sicherheit

```
Cookie: aurago_session=<payload>.<hmac>

payload = base64("user|<expiry_unix>")
hmac = SHA256(secret + payload)
```

- **HttpOnly:** Verhindert XSS-Zugriff auf Cookie
- **SameSite=Strict:** CSRF-Schutz
- **HMAC-SHA256:** Integritätsschutz
- **Ablauf:** Konfigurierbar (Standard: 24 Stunden)

---

## Zusammenfassung

| Feature | Konfiguration | Empfohlene Einstellung |
|---------|---------------|------------------------|
| Vault-Verschlüsselung | `AURAGO_MASTER_KEY` env | 64 zufällige Hex-Zeichen |
| Passwort-Auth | `auth.enabled` | `true` (Produktion) |
| 2FA | `auth.totp_enabled` | `true` (externer Zugriff) |
| Session-Timeout | `auth.session_timeout_hours` | `24` |
| Rate Limiting | `auth.max_login_attempts` | `5` |
| Docker-Gate | `docker.read_only` | `true` (Monitoring) |
| Vault-Gate | `tools.secrets_vault.read_only` | `true` |

---

> 💡 **Nächste Schritte:**
> - **[Co-Agenten](15-coagents.md)** – Parallele Agenten sicher nutzen
> - **[Konfiguration](07-konfiguration.md)** – Feintuning aller Parameter
> - **[Docker-Deployment](../../docker_installation.md)** – Isolierte Produktivumgebung
