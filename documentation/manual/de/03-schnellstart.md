# Kapitel 3: Schnellstart

Deine ersten 5 Minuten mit AuraGo – von der Installation zum produktiven Chat.

## Voraussetzungen

- AuraGo ist [installiert](02-installation.md) und läuft
- Du hast einen [API-Key](02-installation.md#1-api-key-konfigurieren) konfiguriert
- Die Web-UI ist erreichbar unter http://localhost:8088

## Der Quick Setup Wizard

Beim ersten Start führt dich der **Quick Setup Wizard** durch die wichtigsten Einstellungen:

### Schritt 1: Sprache wählen
```
🌍 Willkommen bei AuraGo!
Wähle deine bevorzugte Sprache / Choose your preferred language:
- Deutsch
- English
- ...
```

> 💡 Diese Einstellung beeinflusst die Sprache des Agents und der Web-UI.

### Schritt 2: LLM Provider

Falls noch nicht in `config.yaml` gesetzt:

```
🔌 LLM Provider
Wähle deinen KI-Provider:
- OpenRouter (empfohlen) – Zugriff auf viele Modelle
- Ollama – Lokale Modelle
- OpenAI – GPT-4, GPT-3.5
- Anderer (manuelle Konfiguration)
```

**Für Anfänger empfohlen:** OpenRouter mit einem kostenlosen Modell wie `arcee-ai/trinity-large-preview:free`.

### Schritt 3: Persönlichkeit wählen

```
🎭 Wähle eine Persönlichkeit:
- Freund – Locker, humorvoll, gesprächig
- Professional – Sachlich, effizient, direkt
- Neutral – Ausgewogen, neutral
- Punk – Rebellisch, unkonventionell
```

> 💡 Du kannst die Persönlichkeit später jederzeit ändern.

### Schritt 4: Erste Integrationen (optional)

```
📱 Integrationen einrichten (optional):
- Telegram Bot
- Discord
- Home Assistant
- Überspringen
```

Diese Schritte kannst du auch später nachholen.

## Der erste Chat

Nach dem Setup landest du im **Chat** – das Herzstück von AuraGo.

### Die Chat-Oberfläche

```
┌─────────────────────────────────────────────┐
│ ⚡ AURAGO              🌙         ≡         │  ← Header
├─────────────────────────────────────────────┤
│                                             │
│  👤 Hallo! Wie kann ich dir helfen?        │  ← Agent
│                                             │
│  🤖 Hi! Ich bin dein neuer Assistent.      │  ← Du
│                                             │
├─────────────────────────────────────────────┤
│  📎  [Nachricht eingeben...]  [➤]          │  ← Eingabe
└─────────────────────────────────────────────┘
```

### Deine ersten Nachrichten

**Test 1: Einfache Begrüßung**
```
Du: Hallo!
Agent: Hallo! Schön, dich kennenzulernen. Wie kann ich dir heute helfen?
```

**Test 2: Eine Datei erstellen**
```
Du: Erstelle eine Datei test.txt mit dem Inhalt "Hallo Welt"
Agent: ✅ Ich habe die Datei test.txt erstellt.
   📄 Inhalt: "Hallo Welt"
   📁 Pfad: agent_workspace/workdir/test.txt
```

**Test 3: Systeminformationen**
```
Du: Zeige mir Systeminformationen
Agent: 🔍 Systeminformationen:
   💻 CPU: 4 Kerne, 15% Auslastung
   🧠 RAM: 8 GB gesamt, 3.2 GB frei
   💾 Disk: 100 GB gesamt, 45 GB frei
   🖥️  OS: Linux x86_64
```

## Wichtige Chat-Befehle

AuraGo versteht spezielle Befehle, die mit `/` beginnen:

| Befehl | Funktion |
|--------|----------|
| `/help` | Alle verfügbaren Befehle anzeigen |
| `/reset` | Chat-Verlauf löschen, frischer Start |
| `/stop` | Laufende Agent-Aktion abbrechen |
| `/debug on` | Detaillierte Tool-Ausgaben anzeigen |
| `/debug off` | Kompakte Ausgaben (Standard) |
| `/budget` | Heutige API-Kosten anzeigen |
| `/personality friend` | Persönlichkeit wechseln |

### Beispiele

```
Du: /help
Agent: 📋 Verfügbare Befehle:
   /help, /reset, /stop, /debug on/off, 
   /budget, /personality <name>

Du: /budget
Agent: 💰 Budget-Übersicht (heute):
   Eingegeben: 1,245 Tokens
   Ausgegeben: 3,892 Tokens
   Geschätzte Kosten: $0.0023
```

## Dateien hochladen

1. **Klick** auf das Paperclip-Symbol 📎 unter dem Chat
2. **Wähle** eine Datei aus
3. **Sende** eine Nachricht mit Kontext

```
Du: [Datei: dokument.pdf]
Du: Fasse dieses Dokument zusammen
Agent: 📄 Zusammenfassung von dokument.pdf:
   Das Dokument behandelt...
```

> 💡 Unterstützte Formate: TXT, PDF, Bilder (JPG, PNG), Code-Dateien, und mehr.

## Die ersten Tools kennenlernen

AuraGo hat über 90 eingebaute Tools. Hier sind einige zum Ausprobieren:

### Dateisystem
```
Du: Liste alle Dateien im aktuellen Verzeichnis
Du: Erstelle einen Ordner "projekte"
Du: Lies die Datei config.yaml
```

### Web & Suche
```
Du: Suche im Web nach "Go 1.21 Release Notes"
Du: Rufe die Seite example.com ab und zeige den Titel
```

### System
```
Du: Wie spät ist es?
Du: Zeige den aktuellen Pfad
Du: Welches Betriebssystem läuft hier?
```

### Notizen
```
Du: Speichere als Notiz: "Morgen Umzug nicht vergessen"
Du: Zeige alle meine Notizen
```

## Navigation in der Web-UI

Klicke auf das **Radial-Menü** (☰ oben rechts) für den Hauptmenü:

| Bereich | Funktion |
|---------|----------|
| 💬 Chat | Haupt-Chat-Oberfläche |
| 📊 Dashboard | System-Metriken, Mood-Verlauf |
| 🚀 Missions | Automatisierte Aufgaben |
| 🥚 Invasion | Remote-Deployment |
| ⚙️ Config | Einstellungen bearbeiten |

## Dark/Light Theme

Klicke auf das **Mond/Sonne-Symbol** (🌙/☀️) im Header, um zwischen Dark und Light Mode zu wechseln.

## Nächste Schritte

| Wenn du... | Dann... |
|------------|---------|
| Mehr über die UI erfahren willst | → [Kapitel 4: Web-Oberfläche](04-webui.md) |
| Besser chatten lernen willst | → [Kapitel 5: Chat-Grundlagen](05-chatgrundlagen.md) |
| Alle Tools erkunden willst | → [Kapitel 6: Werkzeuge](06-tools.md) |
| Telegram einrichten willst | → [Kapitel 8: Integrationen](08-integrations.md) |
| Die Konfiguration verstehen willst | → [Kapitel 7: Konfiguration](07-konfiguration.md) |

## Schnell-Checkliste

- [ ] AuraGo installiert und gestartet
- [ ] Quick Setup Wizard durchlaufen
- [ ] Erste Chat-Nachricht gesendet
- [ ] Ein Tool ausprobiert (/help, Datei erstellen, etc.)
- [ ] Eine Datei hochgeladen (optional)
- [ ] Theme gewechselt (optional)
- [ ] Alle UI-Bereiche angeklickt

> 🎉 **Herzlichen Glückwunsch!** Du hast AuraGo erfolgreich eingerichtet und die ersten Schritte gemacht.
