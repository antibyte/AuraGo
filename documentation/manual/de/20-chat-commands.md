# Chat-Commands

AuraGo unterstützt sogenannte **Slash-Commands** – Befehle, die direkt im Chat eingegeben werden können, um bestimmte Aktionen auszuführen. Alle Commands beginnen mit einem Schrägstrich `/`.

> 📅 **Stand:** März 2026

---

## Übersicht aller Commands

| Command | Beschreibung | Verfügbarkeit |
|---------|--------------|---------------|
| `/help` | Zeigt alle verfügbaren Befehle mit Kurzbeschreibung | Immer |
| `/reset` | Löscht den Chat-Verlauf und das Kurzzeitgedächtnis | Immer |
| `/stop` | Unterbricht die aktuelle Aktion des Agenten | Immer |
| `/restart` | Startet den AuraGo-Server neu | Immer |
| `/debug [on/off]` | Aktiviert/Deaktiviert den Debug-Modus | Immer |
| `/personality [name]` | Listet Persönlichkeiten auf oder wechselt sie | Immer |
| `/budget [en]` | Zeigt den aktuellen Budget-Status | Wenn Budget-Tracking aktiv |
| `/sudopwd <passwort>` | Speichert das sudo-Passwort im Vault | Immer |
| `/addssh` | Registriert einen neuen SSH-Server | Immer |
| `/credits` | Zeigt OpenRouter Credits und Verbrauch | Nur bei OpenRouter |

---

## Detaillierte Beschreibungen

### `/help`
Zeigt eine Liste aller registrierten Commands mit ihren Kurzbeschreibungen an.

**Beispiel:**
```
/help
```

**Ausgabe:**
```
📜 **Verfügbare Befehle:**

• /reset: Löscht den aktuellen Chat-Verlauf (Short-Term Memory).
• /stop: Unterbricht die aktuelle Aktion des Agenten.
• /help: Zeigt diese Hilfe an.
...
```

---

### `/reset`
Löscht den aktuellen Chat-Verlauf und das Kurzzeitgedächtnis (Short-Term Memory). Dies ist nützlich, um einen "frischen Start" zu machen, ohne die Langzeitspeicher oder andere Daten zu beeinflussen.

**Beispiel:**
```
/reset
```

**Ausgabe:**
```
🧹 Chat-Verlauf und Kurzzeitgedächtnis wurden gelöscht.
```

> ⚠️ **Hinweis:** Das Langzeitgedächtnis (LTM) und der Knowledge Graph bleiben erhalten.

---

### `/stop`
Unterbricht die aktuell laufende Aktion des Agenten. Dies ist hilfreich, wenn der Agent in eine Endlosschleife gerät oder eine unerwünschte Aktion ausführt.

**Beispiel:**
```
/stop
```

**Ausgabe:**
```
🛑 AuraGo wurde angewiesen, die aktuelle Aktion zu unterbrechen.
```

---

### `/restart`
Startet den AuraGo-Server neu. Dies kann nützlich sein, nachdem Änderungen an der Konfiguration vorgenommen wurden, die einen Neustart erfordern.

**Beispiel:**
```
/restart
```

**Ausgabe:**
```
🔄 AuraGo wird neu gestartet...
```

> ⚠️ **Hinweis:** Der Neustart erfolgt asynchron nach einer kurzen Verzögerung von 1 Sekunde.

---

### `/debug [on|off]`
Aktiviert oder deaktiviert den Debug-Modus des Agenten. Im Debug-Modus werden detailliertere Fehlermeldungen im System-Prompt aktiviert.

**Syntax:**
```
/debug [on|off|1|0|true|false]
```

**Beispiele:**
```
/debug on      # Debug-Modus aktivieren
/debug off     # Debug-Modus deaktivieren
/debug         # Toggle (umschalten)
```

**Ausgabe (aktiviert):**
```
🔍 **Agent Debug-Modus aktiviert.** Der Agent meldet Fehler jetzt mit detaillierten Informationen.
```

**Ausgabe (deaktiviert):**
```
🔇 **Agent Debug-Modus deaktiviert.** Der Agent verhält sich normal.
```

---

### `/personality [name]`
Listet alle verfügbaren Persönlichkeiten auf oder wechselt zu einer spezifischen Persönlichkeit.

**Syntax:**
```
/personality [name]
```

**Beispiele:**
```
/personality           # Listet alle Persönlichkeiten
/personality default   # Wechselt zur Standard-Persönlichkeit
/personality tech      # Wechselt zur Tech-Persönlichkeit
```

**Ausgabe (Listen):**
```
🎭 **Verfügbare Persönlichkeiten:**

• default ✅ (aktiv)
• tech
• creative
• professional

Nutze `/personality <name>` zum Umstellen.
```

**Ausgabe (Wechseln):**
```
🎭 Persönlichkeit auf **tech** umgestellt. Die Änderung ist permanent.
```

> ℹ️ Persönlichkeiten werden als Markdown-Dateien im `prompts/personalities/` Verzeichnis gespeichert.

---

### `/budget [en]`
Zeigt den aktuellen Budget-Status an, einschließlich der Tageskosten, des Limits und der verwendeten Modelle.

**Syntax:**
```
/budget [en]
```

**Parameter:**
- `en` (optional) – Zeigt die Ausgabe auf Englisch an

**Beispiele:**
```
/budget        # Deutsch
/budget en     # Englisch
```

**Ausgabe:**
```
💰 **Budget Status (2026-03-28)**

Heutige Kosten: $0.42 / $5.00 (8.4%)
Verwendete Modelle:
  • google/gemini-2.0-flash-001: $0.32
  • anthropic/claude-3.5-sonnet: $0.10
```

> ℹ️ Budget-Tracking muss in der Konfiguration aktiviert sein (`budget.enabled: true`).

---

### `/sudopwd <passwort>`
Speichert das sudo-Passwort sicher im Vault für das `execute_sudo` Tool. Das Passwort wird AES-256-GCM verschlüsselt gespeichert.

**Syntax:**
```
/sudopwd <passwort>
/sudopwd --clear
```

**Parameter:**
- `passwort` – Das sudo-Passwort
- `--clear` – Löscht das gespeicherte Passwort

**Beispiele:**
```
/sudopwd meinSicheresPasswort123
/sudopwd --clear
```

**Ausgabe:**
```
✅ Sudo-Passwort erfolgreich im Vault gespeichert.
⚠️ Hinweis: `agent.sudo_enabled` ist noch deaktiviert. Aktiviere es in der Config.
```

> 🔒 **Sicherheit:** Das Passwort wird niemals im Klartext gespeichert oder protokolliert.

---

### `/addssh`
Registriert einen neuen SSH-Server im Inventar und speichert die Zugangsdaten sicher im Vault.

**Syntax:**
```
/addssh host=NAME user=USER [ip=IP] [pass=PASS|keypath=PATH] [port=22] [tags=tag1,tag2]
```

**Parameter:**
| Parameter | Erforderlich | Beschreibung |
|-----------|--------------|--------------|
| `host` | Ja | Hostname des Servers |
| `user` | Ja | SSH-Benutzername |
| `ip` | Nein | IP-Adresse (falls von hostname abweichend) |
| `pass` | Bedingt | Passwort (entweder pass oder keypath) |
| `keypath` | Bedingt | Pfad zum SSH-Schlüssel |
| `port` | Nein | SSH-Port (Standard: 22) |
| `tags` | Nein | Komma-getrennte Tags |

**Beispiele:**
```
/addssh host=server1 user=root pass=geheim
/addssh host=server2 user=admin keypath=/home/user/.ssh/id_rsa port=2222 tags=produktion,web
```

**Ausgabe:**
```
✅ Server server1 erfolgreich registriert mit ID: abc-123-def
```

---

### `/credits`
Zeigt den aktuellen OpenRouter-Kontostand und den Verbrauch an. Nur verfügbar, wenn OpenRouter als LLM-Provider konfiguriert ist.

**Beispiel:**
```
/credits
```

**Ausgabe:**
```
💳 **OpenRouter Credits**

Kontostand: $25.43
Genutzter Betrag heute: $0.42
Verbleibend: $25.01
```

> ℹ️ Dieser Command ist nur verfügbar, wenn `llm.provider_type` auf `openrouter` gesetzt ist.

---

## Programmatische Verwendung

Commands können auch programmatisch über die API ausgelöst werden:

```bash
# Via Chat-Completion API mit speziellem Prefix
 curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "/reset"}]
  }'
```

---

## Troubleshooting

| Problem | Lösung |
|---------|--------|
| Command wird nicht erkannt | Stelle sicher, dass du mit `/` beginnst und keine Leerzeichen davor hast |
| `/budget` zeigt keine Daten | Budget-Tracking muss in `config.yaml` aktiviert sein |
| `/credits` funktioniert nicht | Nur bei OpenRouter als Provider verfügbar |
| `/addssh` meldet Fehler | Prüfe, ob `host` und `user` angegeben sind, sowie `pass` oder `keypath` |

---

## Weiterführende Links

- [Web-Oberfläche](04-webui.md) – Alternative zur Command-Zeile
- [REST API Referenz](21-api-reference.md) – Programmatischer Zugriff
- [Sicherheit](14-sicherheit.md) – Vault und Verschlüsselung
