# Kapitel 5: Chat-Grundlagen

Effektive Kommunikation mit AuraGo – von einfachen Befehlen bis zu komplexen Workflows.

## Grundlagen der Kommunikation

AuraGo versteht natürliche Sprache. Du musst keine speziellen Befehle lernen, um Aufgaben zu erledigen. Schreib einfach, wie du es einem menschlichen Assistenten erklären würdest.

### Sprache und Formulierung

AuraGo erkennt automatisch die Sprache deiner Nachricht und antwortet entsprechend:

```
Du: Kannst du mir helfen, ein Python-Skript zu schreiben?
Du: Can you help me write a Python script?
Du: Peux-tu m'aider à écrire un script Python?
```

> 💡 **Tipp:** Die Sprache der Konfiguration (`system_language` in config.yaml) bestimmt die Standardsprache des Agents, aber du kannst jederzeit die Sprache wechseln.

### Klare vs. komplexe Anfragen

| Art | Beispiel | Ergebnis |
|-----|----------|----------|
| **Einfach** | "Was ist die Hauptstadt von Frankreich?" | Direkte Antwort |
| **Strukturiert** | "Liste alle Dateien im Ordner X, filtere nach .txt und zeige die neuesten 5 an" | Präzise Ausführung |
| **Kontextuell** | "Wie war nochmal der Name des Projekts, über das wir gestern gesprochen haben?" | Nutzt Memory-System |

## Nachrichtentypen und Formate

### Textnachrichten

Die gebräuchlichste Form der Kommunikation. Unterstützt:

- **Normale Texteingabe** – Standard-Konversationen
- **Markdown-Formatierung** – Wird von AuraGo in der Antwort verwendet
- **Code-Blöcke** – Werden mit Syntax-Highlighting dargestellt

```
Du: Erkläre mir den Unterschied zwischen REST und GraphQL
Agent: 🔍 **REST vs. GraphQL**

REST:
- Mehrere Endpoints (/users, /posts)
- Feste Datenstruktur
- HTTP-Methoden (GET, POST, PUT, DELETE)

GraphQL:
- Einzelner Endpoint
- Flexible Abfragen
- Präzise Datenabfrage
```

### Befehle mit `/`

Spezielle Steuerbefehle für den Chat:

| Befehl | Funktion | Beispiel |
|--------|----------|----------|
| `/help` | Alle verfügbaren Befehle anzeigen | `/help` |
| `/reset` | Chat-Verlauf löschen | `/reset` |
| `/stop` | Laufende Aktion abbrechen | `/stop` |
| `/debug on` | Detaillierte Tool-Ausgaben | `/debug on` |
| `/debug off` | Kompakte Ausgaben (Standard) | `/debug off` |
| `/budget` | Heutige API-Kosten anzeigen | `/budget` |
| `/personality <name>` | Persönlichkeit wechseln | `/personality pro` |
| `/sudo` | Sudo-Modus aktivieren (falls konfiguriert) | `/sudo` |
| `/credits` | OpenRouter Credits anzeigen | `/credits` |
| `/addssh` | SSH-Server zum Inventar hinzufügen | `/addssh server.com user key` |

### Mehrzeilige Nachrichten

Nutze `Shift + Enter` für Zeilenumbrüche:

```
Du: Schreibe eine Funktion, die:
- Eine Liste von Zahlen nimmt
- Die geraden Zahlen filtert
- Das Quadrat davon berechnet

[Shift + Enter]

Und bitte mit Type-Hinweisen.
```

## Datei-Uploads und Verarbeitung

### Unterstützte Dateiformate

| Kategorie | Formate | Verwendung |
|-----------|---------|------------|
| **Text** | .txt, .md, .csv, .json, .yaml | Lesen, Analysieren, Bearbeiten |
| **Dokumente** | .pdf | Extrahieren, Zusammenfassen |
| **Code** | .go, .py, .js, .html, etc. | Review, Debuggen, Erklären |
| **Bilder** | .jpg, .png, .gif, .webp | Analyse (siehe Bildanalyse) |
| **Archive** | .zip, .tar.gz | Extrahieren, Inhalte anzeigen |

### Datei hochladen

**In der Web-UI:**
1. Klicke auf das 📎-Symbol unter dem Chat-Eingabefeld
2. Wähle eine Datei aus
3. Schreibe eine Nachricht mit Kontext

**Via Telegram:**
- Sende die Datei direkt an den Bot
- Oder nutze die "Datei senden"-Funktion

### Beispiele für Datei-Interaktionen

```
Du: [Datei: server.log]
Du: Analysiere diesen Log und finde alle ERROR-Einträge

Agent: 🔍 Log-Analyse:
   Gefunden: 3 ERROR-Einträge
   
   [14:23:05] ERROR: Connection timeout to database
   [14:45:12] ERROR: Failed to authenticate user 'admin'
   [15:01:33] ERROR: Disk space critical (< 5%)
```

```
Du: [Datei: config.yaml]
Du: Erkläre mir die wichtigsten Einstellungen in dieser Konfiguration

Agent: 📄 Konfigurationsübersicht:
   - Server läuft auf Port 8088
   - LLM: OpenRouter mit Modell trinity-large
   - Memory-System aktiviert
   - Budget-Tracking deaktiviert
```

> 💡 **Tipp:** Bei großen Dateien (> 1MB) wird AuraGo automatisch die relevanten Abschnitte extrahieren oder dich fragen, wonach suchen soll.

## Bildanalyse

AuraGo kann Bilder analysieren und beschreiben – sowohl in der Web-UI als auch über Telegram.

### Funktionsweise

Die Bildanalyse nutzt ein Vision-LLM (Standard: Google Gemini Flash), um:
- Objekte zu erkennen
- Text zu extrahieren (OCR)
- Szenen zu beschreiben
- Code-Screenshots zu analysieren

### Beispiele

```
Du: [Bild: screenshot.png]
Du: Was zeigt dieser Screenshot?

Agent: 📸 Bildanalyse:
   Das Bild zeigt einen VS Code Editor mit geöffneter 
   main.go Datei. Der Code enthält eine HTTP-Handler-
   Funktion. Unten rechts ist ein Fehlerindikator zu 
   sehen (rotes X), was auf einen Syntax-Fehler hindeutet.
```

```
Du: [Bild: diagramm.jpg]
Du: Erstelle eine Textbeschreibung dieses Architektur-Diagramms

Agent: 📊 Architekturbeschreibung:
   Das Diagramm zeigt eine 3-Schicht-Architektur:
   
   1. Präsentationsschicht (Web-UI, Telegram)
   2. Anwendungsschicht (Agent Core, Tools)
   3. Datenschicht (SQLite, VectorDB)
   
   Pfeile zeigen bidirektionale Kommunikation zwischen 
   allen Schichten.
```

> 🔍 **Deep Dive: Vision-LLM Konfiguration**
> Die Bildanalyse kann in der `config.yaml` unter dem `vision:`-Block konfiguriert werden:
> - `provider`: openrouter, ollama, openai
> - `model`: z.B. google/gemini-2.5-flash-lite-preview-09-2025
> - `api_key` und `base_url`: Für den jeweiligen Provider

## Sprachnachrichten (Telegram)

Über Telegram kannst du auch Sprachnachrichten senden, die automatisch transkribiert werden.

### Aktivierung

In `config.yaml`:
```yaml
telegram:
  bot_token: "dein-token"
  telegram_user_id: 12345678
```

### Nutzung

1. Halte das Mikrofon-Symbol in Telegram gedrückt
2. Spreche deine Nachricht
3. Lasse los – AuraGo transkribiert und antwortet

### Beispiel

```
[🎙️ Sprachnachricht empfangen]
Transkription: "Schicke eine E-Mail an Max mit dem Betreff 
Besprechung morgen und dem Text Wir treffen uns um 10 Uhr"

Agent: ✅ E-Mail wird vorbereitet...
   An: max@example.com
   Betreff: Besprechung morgen
   
   Möchtest du die E-Mail jetzt senden?
```

> 💡 **Tipp:** Sprachnachrichten sind besonders nützlich unterwegs oder für längere Anfragen.

## Chat-Verlauf und Kontext

### Wie der Kontext funktioniert

AuraGo behält den Konversationskontext bei, um kohärente Gespräche zu führen:

```
Du: Erstelle eine Todo-Liste für mein Projekt
Agent: ✅ Liste erstellt mit 5 Standard-Aufgaben

Du: Füge noch "Dokumentation schreiben" hinzu
Agent: ✅ "Dokumentation schreiben" zur Liste hinzugefügt
       
       Aktuelle Aufgaben:
       1. Projektstruktur erstellen
       2. README schreiben
       3. Dokumentation schreiben
```

> 💡 **Der Agent erinnert sich an den vorherigen Kontext**, auch wenn du nicht explizit wiederholst, worum es geht.

### Chat zurücksetzen

Nutze `/reset` für einen frischen Start:
- Löscht den aktuellen Chat-Verlauf
- Behält gespeicherte Notizen und Long-Term-Memory
- Nützlich bei Kontext-Verwirrung oder neuem Thema

```
Du: /reset
Agent: 🔄 Chat-Verlauf wurde zurückgesetzt.
       Wie kann ich dir helfen?
```

### Kontext-Fenster

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `context_window` | 131000 Tokens | Maximale Kontextgröße |
| `memory_compression_char_limit` | 50000 | Ab hier wird komprimiert |
| `system_prompt_token_budget` | 8192 | Reserviert für System-Instruktionen |

> 🔍 **Deep Dive: Kontext-Kompression**
> Wenn der Kontext zu groß wird, nutzt AuraGo ein intelligentes Komprimierungssystem:
> 1. Ältere Nachrichten werden zusammengefasst
> 2. Wichtige Fakten werden in das Long-Term-Memory übertragen
> 3. Der Agent behält den "Sinn" des Gesprächs bei

## Memory-Systeme im Überblick

AuraGo verfügt über mehrere Speicher-Ebenen:

```
┌─────────────────────────────────────────────────────┐
│  Short-Term Memory                                  │
│  Aktueller Chat-Verlauf (letzte ~20 Nachrichten)   │
├─────────────────────────────────────────────────────┤
│  Core Memory                                        │
│  Wichtige Fakten, die immer verfügbar sind         │
├─────────────────────────────────────────────────────┤
│  Long-Term Memory (RAG)                             │
│  Vektorbasierte semantische Suche                  │
├─────────────────────────────────────────────────────┤
│  Knowledge Graph                                    │
│  Strukturierte Entitäten und Beziehungen           │
└─────────────────────────────────────────────────────┘
```

### Short-Term Memory

Der aktuelle Gesprächskontext. Wird automatisch verwaltet.

### Core Memory

Permanente Fakten, die AuraGo immer "im Kopf" behält:

```
Du: Merke dir: Mein Lieblings-Editor ist VS Code
Agent: ✅ In Core Memory gespeichert

[Später, neuer Chat]
Du: Welchen Editor sollte ich nutzen?
Agent: Du hast erwähnt, dass VS Code dein 
       Lieblings-Editor ist. Ich empfehle ihn dir!
```

### Long-Term Memory (RAG)

Semantische Suche über alle vergangenen Gespräche:

```
Du: Erinnerst du dich an das Python-Skript, das wir 
    letzte Woche für die Datenverarbeitung geschrieben haben?

Agent: 🔍 Long-Term Memory durchsucht...
       Ja! Du meinst wahrscheinlich das Skript 
       'data_processor.py' vom 15.03., das CSV-Dateien 
       parst und in JSON umwandelt.
```

### Knowledge Graph

Strukturierte Informationen über Entitäten (Personen, Projekte, Konzepte):

```
Du: Erstelle einen Knowledge Graph für mein Projekt "Aura"
Du: Das Projekt hat 3 Module: Core, UI und API
Du: Max arbeitet am Core, Lisa an der UI

[Knowledge Graph wird automatisch aktualisiert]
```

## Best Practices für Prompts

### 1. Sei spezifisch

| ❌ Zu allgemein | ✅ Spezifisch |
|-----------------|---------------|
| "Schreibe Code" | "Schreibe eine Go-Funktion, die eine E-Mail validiert" |
| "Suche etwas" | "Suche nach Go 1.22 Release Notes auf GitHub" |
| "Zeige Dateien" | "Liste alle .go Dateien im aktuellen Verzeichnis" |

### 2. Kontext geben

```
Du: Ich arbeite an einem REST-API-Projekt in Go.
    Die Struktur folgt dem Clean Architecture Pattern.
    
    Erstelle einen neuen Endpoint für Benutzer-Registrierung.
```

### 3. Schrittweise arbeiten

```
Du: Schritt 1: Erstelle die Datenbank-Verbindung
Du: Schritt 2: Schreibe die Migration für die users-Tabelle
Du: Schritt 3: Implementiere den Registrierungs-Handler
```

### 4. Beispiele nennen

```
Du: Erstelle eine Regex, die E-Mail-Adressen validiert.
    Beispiele für gültige Adressen:
    - user@example.com
    - name.surname@company.co.uk
    
    Beispiele für ungültige Adressen:
    - @example.com
    - user@.com
```

### 5. Format wünschen

```
Du: Zeige mir die Systeminformationen als Tabelle
Du: Fasse das Dokument in 3 Stichpunkten zusammen
Du: Erkläre es mir wie einem Anfänger
```

### Die 5-W-Fragen

Nutze für komplexe Aufgaben die 5-W-Methode:

| Frage | Beispiel |
|-------|----------|
| **Was** | "Erstelle ein Python-Skript" |
| **Wer** | "Für mein Entwicklungsteam" |
| **Wann** | "Soll sofort funktionieren" |
| **Wo** | "Im Ordner /projects/utils" |
| **Warum** | "Zur Automatisierung von Backups" |

## Beispiel-Konversationen

### Level: Anfänger

**Einfache Begrüßung und Frage:**
```
Du: Hallo! Was kannst du alles machen?

Agent: Hallo! 👋 Ich bin AuraGo, dein persönlicher 
       KI-Assistent. Ich kann:
       
       📁 Dateien verwalten (erstellen, lesen, bearbeiten)
       🔍 Im Web suchen und Informationen finden
       💻 Code schreiben und erklären
       📧 E-Mails senden (wenn konfiguriert)
       🏠 Smart-Home-Geräte steuern
       
       Und vieles mehr! Was interessiert dich?
```

**Eine einfache Datei erstellen:**
```
Du: Erstelle eine Datei mit meinen Einkäufen

Agent: Gerne! Was genau soll in die Datei? 
       Zum Beispiel:
       
       einkaufsliste.txt:
       - Milch
       - Brot
       - Äpfel
       
       Soll ich diese Liste erstellen oder hast du 
       andere Artikel?

Du: Ja, genau die Liste

Agent: ✅ Datei erstellt: agent_workspace/workdir/einkaufsliste.txt
       Inhalt: Milch, Brot, Äpfel
```

### Level: Fortgeschritten

**Code-Review mit Datei:**
```
Du: [Datei: main.go]
Du: Bitte reviewe diesen Code und gib Verbesserungsvorschläge

Agent: 🔍 Code-Review für main.go:

       ✅ Gut:
       - Klare Struktur mit main() und init()
       - Nutzung von fmt für Logging
       
       ⚠️ Verbesserungsvorschläge:
       1. Zeile 23: Error-Handling fehlt bei file.Open()
       2. Zeile 45: Hardcoded Port – besser über 
          Umgebungsvariable
       3. Zeile 67: Goroutine ohne WaitGroup könnte zu 
          Race Conditions führen
       
       🔧 Konkrete Änderungen:
       [Zeigt verbesserte Code-Version]
```

**Komplexe Suche:**
```
Du: Suche im Web nach aktuellen Best Practices für 
    Go Error Handling 2024
    
    Konzentriere dich auf:
    - Error Wrapping
    - Custom Error Types
    - Error Handling in Goroutines

Agent: 🔍 Suche läuft...
       📄 3 relevante Artikel gefunden
       
       Zusammenfassung der Best Practices 2024:
       
       1. **Error Wrapping**
          Nutze fmt.Errorf mit %w:
          return fmt.Errorf("failed to connect: %w", err)
       
       2. **Custom Error Types**
          Definiere spezifische Fehler für dein Domain:
          type ValidationError struct { Field string }
       
       3. **Goroutine Errors**
          Nutze errgroup oder Kanäle für Fehler-Propagation
```

### Level: Experte

**Multi-Step Workflow:**
```
Du: Ich möchte ein automatisiertes Backup-Skript erstellen:

1. Es soll alle 6 Stunden laufen
2. Backup von ~/Documents und ~/Projects
3. Komprimierung als tar.gz mit Zeitstempel
4. Upload zu WebDAV (wenn konfiguriert)
5. Löschung von Backups älter als 30 Tage
6. Status-Email bei Erfolg/Fehler

Erstelle das komplette Setup.

Agent: 🚀 Erstelle automatisiertes Backup-System...

Schritt 1/6: Backup-Skript erstelle
✅ backup.sh erstellt mit:
   - Timestamp-Generierung
   - Incremental backup Option
   - Fehler-Handling

Schritt 2/6: Cron-Job konfigurieren
✅ Cron-Eintrag: 0 */6 * * * /path/to/backup.sh

Schritt 3/6: WebDAV-Upload
✅ Upload-Funktion integriert
   Prüft Verbindung vor Upload

Schritt 4/6: Cleanup-Logik
✅ Löscht Backups > 30 Tage
   Behält mindestens 5 Backups

Schritt 5/6: Email-Benachrichtigung
✅ Erfolgsmeldung mit Statistik
⚠️ Fehlermeldung mit Log-Auszug

Schritt 6/6: Test-Modus
🧪 Möchtest du einen Testlauf starten?
```

**Selbstmodifizierender Agent:**
```
Du: Ich möchte, dass du dir ein neues Tool erstellst.

Es soll "weather_check" heißen und:
- Die aktuelle Wetterdaten für einen Ort abfragen
- Eine einfache Zusammenfassung geben
- Open-Meteo API nutzen (kein API-Key nötig)

Erstelle das Tool und teste es mit "Berlin".

Agent: 🛠️ Erstelle neues Tool "weather_check"...
       
       ✅ Tool erstellt: agent_workspace/tools/weather_check.py
       ✅ In Manifest registriert
       ✅ Tool steht jetzt zur Verfügung
       
       🧪 Teste Tool mit Berlin...
       
       🌤️ Wetter in Berlin:
       Aktuell: 18°C, leicht bewölkt
       Gefühlt: 16°C
       Wind: 12 km/h aus SW
       Niederschlag: 0% Wahrscheinlichkeit
       
       Heute: Max 21°C, Min 12°C
       Morgen: Max 23°C, sonnig
```

## Fehlerbehebung

| Problem | Mögliche Ursache | Lösung |
|---------|------------------|--------|
| Agent versteht nicht, was ich will | Zu komplexe Anfrage | In kleinere Schritte aufteilen |
| Falsche Tool-Ausführung | Unklare Formulierung | Spezifischer werden, Beispiele nennen |
| Kein Kontext aus früheren Chats | `/reset` wurde genutzt | Wichtige Infos in Notizen speichern |
| Zu lange Antworten | Keine Format-Vorgabe | "Kurze Antwort" oder "In Stichpunkten" ergänzen |
| Agent "halluziniert" Fakten | Kein Web-Zugriff bei Fakten | "Suche im Web nach..." explizit sagen |

## Zusammenfassung: Dos and Don'ts

| ✅ Do | ❌ Don't |
|-------|----------|
| Sei spezifisch bei Anfragen | Sei zu vage ("Mach irgendwas mit...") |
| Gib Kontext bei komplexen Aufgaben | Erwarte, dass der Agent alles errät |
| Nutze `/reset` bei neuen Themen | Den alten Kontext endlos mitführen |
| Speichere wichtige Infos als Notizen | Alles nur im Chat erwähnen |
| Überprüfe Tool-Ausgaben | Blind vertrauen, besonders bei kritischen Operationen |
| Nutze Debug-Modus bei Problemen | Frustriert aufgeben |

---

> 💡 **Merke:** Je besser du kommunizierst, desto besser werden die Ergebnisse. AuraGo ist ein Werkzeug – du bist der Architekt!

## Nächste Schritte

- **[Werkzeuge](06-tools.md)** – Alle verfügbaren Tools im Detail
- **[Konfiguration](07-konfiguration.md)** – Feintuning deines Agents
- **[Integrationen](08-integrationen.md)** – Telegram, Discord, Email einrichten
