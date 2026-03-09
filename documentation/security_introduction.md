# Sicherheit und Risikobewusstsein bei AuraGo

## Die Doppelschneidige Natur Agentic AI

Künstliche Intelligenz, die handelt – nicht nur antwortet – eröffnet transformative Möglichkeiten. Agentic AI Frameworks wie AuraGo können Code ausführen, Systeme verwalten, Daten verarbeiten und komplexe Workflows eigenständig durchführen. Doch genau diese **Autonomie birgt inhärente Risiken**.

> ⚠️ **Ein grundlegendes Verständnis:** Je mächtiger ein Werkzeug ist, desto größer ist sein Potenzial – sowohl für konstruktive als auch für destruktive Wirkung.

### Die Gefahren umfassender Systemzugriffe

Wenn ein AI-Agent uneingeschränkten Zugriff auf Ihr System hat, können Fehler des Language Models, Missverständnisse in der Anweisung oder schlichtweg unvorhergesehene Interaktionsketten zu:

- **Unbeabsichtigten Datenverlusten** (falsche Datei gelöscht/überschrieben)
- **Systeminstabilität** (falsche Konfiguration, abgestürzte Dienste)
- **Sicherheitslücken** (falsch gesetzte Berechtigungen, exponierte Ports)
- **Ressourcenverbrauch** (Endlosschleifen, Speicherüberlastung)

führen.

Die Herausforderung: Ein LLM ist deterministisch nur bedingt vorhersagbar. Selbst mit den besten Prompts kann ein Agent in bestimmten Kontexten unerwartete Entscheidungen treffen.

---

## AuraGos Ansatz: Defense in Depth

AuraGo wurde von Grund auf mit dem Bewusstsein dieser Risiken entwickelt. Das Framework implementiert ein **mehrschichtiges Sicherheitsmodell**, das dem Nutzer maximale Kontrolle gibt:

### 1. Feingranulare Capability Controls (Danger Zone)

AuraGo erlaubt es, jede einzelne Fähigkeit des Agents separat zu aktivieren oder zu deaktivieren:

```yaml
agent:
  # Kerneinschränkungen
  allow_shell: false              # Shell-Befehle verbieten
  allow_python: false             # Python-Ausführung verbieten
  allow_filesystem_write: false   # Schreibzugriff auf Dateien verbieten
  allow_network_requests: false   # HTTP/Netzwerkzugriff verbieten
  allow_remote_shell: false       # SSH-Fernzugriff verbieten
  allow_self_update: false        # Selbstmodifikation verbieten
  allow_mcp: false                # externe MCP-Server verbieten
  allow_web_scraper: false        # Web-Scraping verbieten
  sudo_enabled: false             # Sudo-Rechte verbieten
```

**Das Prinzip:** Standardmäßig ist nichts erlaubt. Sie entscheiden explizit, welche Fähigkeiten der Agent benötigt.

### 2. Tool-Level Read-Only Modus

Selbst aktivierte Tools können in den Nur-Lesen-Modus versetzt werden:

```yaml
tools:
  filesystem:
    enabled: true
    readonly: true                # Nur lesen, nicht schreiben
  docker:
    enabled: true
    readonly: true                # Nur anzeigen, nicht steuern
  missions:
    enabled: true
    readonly: true                # Nur ansehen, nicht ausführen
```

### 3. Vault-basierte Secret-Verwaltung

API-Keys, Passwörter und Tokens werden niemals im Klartext in der Konfiguration gespeichert, sondern in einem **AES-256-GCM verschlüsselten Vault**.

### 4. Circuit Breaker & Limits

```yaml
circuit_breaker:
  max_tool_calls: 20              # Hard-Limit für Tool-Aufrufe
  llm_timeout_seconds: 180        # Timeout für LLM-Antworten
  retry_intervals: ["10s", "2m"]  # Kontrollierte Wiederholungen
```

---

## Die Verantwortung des Nutzers

Trotz aller technischen Schutzmaßnahmen liegt die **ultimative Risikobewertung beim Nutzer**. AuraGo ist ein Werkzeug – Sie entscheiden, wie Sie es einsetzen.

### Szenario-Analyse: Wie viel Risiko ist akzeptabel?

| Einsatzszenario | Empfohlene Konfiguration | Risikostufe |
|-----------------|-------------------------|-------------|
| **Experimentieren, Lernen** | `allow_shell: false`, `allow_python: true` (Sandbox), `allow_filesystem_write: false` | 🟢 Niedrig |
| **Persönlicher Assistent** | `allow_shell: true`, `allow_python: true`, `allow_filesystem_write: true` (User-Directory) | 🟡 Mittel |
| **Systemadministration** | Alle Features aktiviert, aber `sudo_enabled: false` | 🟠 Hoch |
| **Full-Autonomy Mode** | Alle Features inkl. `sudo_enabled: true` | 🔴 Sehr hoch |

> 💡 **Faustregel:** Aktivieren Sie nur Features, die Sie für Ihren konkreten Anwendungsfall benötigen. Starten Sie restriktiv und öffnen Sie nach Bedarf.

---

## Empfohlene Deployment-Strategien

Die sicherste Konfiguration nützt wenig, wenn das zugrunde liegende System kritisch ist. Wir empfehlen dringend eine **isolierte Umgebung** für AuraGo.

### Option 1: Docker-Container (Empfohlen für Entwickler)

Der Container bietet Prozess- und Dateisystem-Isolation:

```yaml
# docker-compose.yml
services:
  aurago:
    image: aurago:latest
    volumes:
      # Nur explizit gemountete Verzeichnisse sind zugänglich
      - ./data:/app/data
      - ./workspace:/app/workspace
    # Kein Zugriff auf Host-Docker (optional)
    # - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - AURAGO_MASTER_KEY=${MASTER_KEY}
    # Ressourcenlimits
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
```

**Vorteile:**
- Einfaches Backup (nur Volumes sichern)
- Schnelle Wiederherstellung bei Problemen
- Keine Abhängigkeiten vom Host-System
- Portabel zwischen Systemen

### Option 2: Dedizierter Mini-PC (Empfohlen für 24/7-Betrieb)

Für produktive, dauerhafte Installationen bietet sich ein **separater Mini-PC** an. Moderne N100/N150-basierte Systeme sind hier ideal:

| Hardware | Preis | Stromverbrauch | Leistung |
|----------|-------|----------------|----------|
| Intel N100/N150 Mini-PC | 80-150€ | 6-15W TDP | Ausreichend für AuraGo + SQLite + kleine Modelle |
| Refurbished Thin Client | 50-100€ | 10-25W | Gut für reine Agent-Aufgaben |
| Raspberry Pi 5 | 80-120€ | 5-8W | Möglich, aber begrenzt bei Embeddings |

**Warum ein separater Rechner?**

1. **Netzwerk-Isolation:** Der Agent läuft in einem separaten Subnetz, Zugriff nur über explizit freigegebene Ports
2. **Hardware-Isolation:** Ein Fehler kann Ihren Hauptrechner nicht beeinträchtigen
3. **Stromsparend:** N100/N150-Systeme verbrauchen weniger als eine Glühbirne
4. **Kostengünstig:** Für unter 150€ voll funktionsfähig
5. **Leise:** Lüfterlos oder nahezu lautlos betreibbar

### Beispiel-Setup: N100 Mini-PC

```
┌─────────────────────────────────────────┐
│         Ihr Hauptnetzwerk              │
│  (Laptop, PC, Smartphones...)          │
└────────────┬────────────────────────────┘
             │
             │ VPN/Tailscale (optional)
             ▼
┌─────────────────────────────────────────┐
│   Dedizierter N100 Mini-PC             │
│   ┌─────────────────────────────────┐  │
│   │  AuraGo Container               │  │
│   │  - Limitierte CPU/Memory        │  │
│   │  - Bind Mounts nur für /data    │  │
│   │  - Kein Host-Netzwerk-Modus     │  │
│   └─────────────────────────────────┘  │
│   OS: Ubuntu Server / Debian / Proxmox │
│   Firewall: UFW aktiviert              │
│   Automatische Updates: aktiviert      │
└─────────────────────────────────────────┘
```

### Option 3: Virtuelle Maschine (Enterprise)

Für Unternehmensumgebungen oder wenn Sie bereits virtualisieren:

- **Proxmox VE:** Kostenlos, Web-Interface, Backup/Restore
- **VMware ESXi:** Enterprise-Standard
- **Hyper-V:** Windows-Integration

**Vorteile:** Snapshots vor kritischen Änderungen, Live-Migration, Ressourcen-Quota.

---

## Sicherheits-Checkliste

Vor dem ersten Produktiveinsatz:

- [ ] **Umgebung:** Container oder dedizierter Mini-PC eingerichtet
- [ ] **Netzwerk:** Web-UI nicht direkt aus dem Internet erreichbar (VPN/Reverse Proxy mit Auth)
- [ ] **Config:** Nur benötigte `allow_*` Parameter auf `true` gesetzt
- [ ] **Tools:** Kritische Tools auf `readonly: true` gesetzt
- [ ] **Auth:** Web-UI Authentifizierung aktiviert (`auth.enabled: true`)
- [ ] **Secrets:** Alle API-Keys im Vault gespeichert, nicht in Config
- [ ] **Limits:** `max_tool_calls` und Timeouts gesetzt
- [ ] **Backup:** Regelmäßiges Backup der `data/`-Verzeichnis eingerichtet
- [ ] **Monitoring:** Logs auf ungewöhnliche Aktivitäten überwachen

---

## Fazit

AuraGo bietet mit seinem mehrschichtigen Sicherheitskonzept ein hohes Maß an Kontrolle – aber **keine Technologie kann vollständige Sicherheit garantieren**. Das Verhältnis von Nutzen zu Risiko liegt in Ihrer Hand:

- Nutzen Sie die **feingranularen Steuerungsmöglichkeiten**
- Betreiben Sie den Agent in einer **isolierten Umgebung**
- Bewerten Sie für jeden Einsatzfall das **akzeptable Risiko**
- Beginnen Sie **konservativ** und erweitern Sie nach Bedarf

> 🛡️ **Das oberste Gebot:** Traue dem Agent – aber verifiziere. Starte mit minimalen Rechten, teste in sicherer Umgebung, und öffne die Fähigkeiten schrittweise.

---

**Weiterführende Dokumentation:**
- [Sicherheitskonfiguration](./manual/de/14-sicherheit.md)
- [Docker-Installation](./docker_installation.md)
- [Best Practices für Production](./manual/de/18-anhang.md)
