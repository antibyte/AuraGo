# Kapitel 10: Persönlichkeit

AuraGo bietet ein Persönlichkeitssystem, das das Verhalten und den Kommunikationsstil der KI beeinflusst.

---

## Übersicht

Die Persönlichkeit beeinflusst:

| Aspekt | Beschreibung |
|--------|--------------|
| **Tonfall** | Formell, locker, sarkastisch, freundlich |
| **Antwortlänge** | Kurz und prägnant vs. ausführlich |
| **Emojis** | Verwendung und Art der Emojis |
| **Sprachstil** | Technisch, umgangssprachlich, poetisch |

---

## Personality Engine

AuraGo hat zwei Personality Engines, die unabhängig aktiviert werden können:

### Personality Engine V1 (Heuristisch)

Die V1-Engine verwendet vordefinierte Prompt-Templates ohne zusätzliche LLM-Aufrufe.

**Konfiguration:**
```yaml
# config.yaml
personality:
  engine: true
  core_personality: "friend"
```

### Personality Engine V2 (LLM-basiert)

> ⚠️ **Erfordert zusätzliche API-Aufrufe:** Die V2-Engine analysiert Stimmung und Kontext mit einem separaten LLM-Aufruf.

Die V2-Engine bietet:
- Dynamische Stimmungsanalyse
- Automatische Temperatur-Modulation
- Benutzer-Profiling (optional)

**Konfiguration:**
```yaml
# config.yaml
personality:
  engine: true
  engine_v2: true
  v2_provider: ""
  user_profiling: false
  user_profiling_threshold: 3
  v2_timeout_secs: 30
  emotion_synthesizer:
    enabled: true
    trigger_on_mood_change: true
```

### Beide Engines deaktivieren

```yaml
personality:
  engine: false
  engine_v2: false
```

---

## Verfügbare Persönlichkeiten

| Profil | Beschreibung | Ideal für |
|--------|--------------|-----------|
| `neutral` | Sachlich, ausgewogen | Allgemeine Aufgaben, technische Dokumentation |
| `friend` | Warm, unterstützend, Duzen | Persönliche Gespräche, alltägliche Aufgaben |
| `professional` | Höflich, effizient, formell | Business-Kontexte, formelle Kommunikation |
| `punk` | Rebellisch, direkt, unkonventionell | Kreative Projekte, Brainstorming |
| `terminator` | Extrem kurz, direkt, ohne Floskeln | Schnelle Informationen, Kommandozeilen-Modus |
| `psycho` | Chaotisch, unberechenbar | Experimente, Entertainment |
| `mcp` | Fokus auf Model Context Protocol | MCP-Server Interaktionen |

### Persönlichkeit wechseln

#### Über die Config

```yaml
personality:
  core_personality: "professional"
```

> 💡 Änderungen erfordern einen Neustart von AuraGo.

#### Über die Web-UI

1. Öffne die Web-Oberfläche
2. Gehe zu "Config"
3. Suche nach `personality.core_personality`
4. Wähle eine Persönlichkeit aus dem Dropdown
5. Speichere und starte neu

---

## Benutzer-Profiling (V2)

Wenn `personality.user_profiling: true` gesetzt ist, lernt AuraGo automatisch:

- Bevorzugte Detailtiefe (technisch vs. allgemein)
- Programmiersprachen und Tools
- Kommunikationsstil
- Erfahrungslevel

**Beispiel - Gelernte Präferenzen:**
```
Benutzer: Kannst du mir bei dem Python-Skript helfen?

[AuraGo hat gelernt: Benutzer nutzt Python]

Später:
Benutzer: Wie löse ich das am besten?
Agent: In Python könntest du dafür eine Dictionary-Comprehension nutzen...
```

**Datenschutz:**
- Profil-Daten werden lokal gespeichert
- Keine Übertragung an externe Server
- Kann jederzeit deaktiviert werden

---

## Temperatur-Modulation (V2)

Die V2-Engine kann die LLM-Temperatur dynamisch anpassen:

| Situation | Temperatur | Begründung |
|-----------|------------|------------|
| Faktenabfrage | Niedriger | Präzision wichtig |
| Code-Generierung | Niedriger | Deterministisch |
| Brainstorming | Höher | Kreativität gewünscht |
| Konversation | Mittel | Balance |

**Konfiguration:**
```yaml
llm:
  temperature: 0.7  # Basistemperatur
```

Die V2-Engine moduliert um diesen Basiswert basierend auf Kontext. Wenn der Emotion Synthesizer aktiv ist, speichert AuraGo zusätzlich kurze natürlichsprachliche Emotionsnotizen und zeigt sie im Chat-Widget und Dashboard an.

---

## Beispiel-Vergleich

**Gleiche Anfrage, verschiedene Persönlichkeiten:**

| Persönlichkeit | Antwort |
|----------------|---------|
| **terminator** | `Fehler in Zeile 42. Variable 'x' nicht definiert.` |
| **professional** | `Bei der Überprüfung Ihres Codes habe ich einen Fehler festgestellt. In Zeile 42 wird die Variable 'x' verwendet, ohne dass sie zuvor definiert wurde.` |
| **friend** | `Oh, da ist ein kleiner Fehler drin! 😅 In Zeile 42 versuchst du, auf 'x' zuzugreifen, aber du hast sie vorher nicht definiert. Kein Problem, passiert jedem!` |
| **punk** | `Alter, da hat wer geschlafen! 😂 Zeile 42: 'x' existiert nicht im Nirwana! Du musst der Variable erst Leben einhauchen! 🤘` |

---

## Best Practices

### Auswahl der richtigen Persönlichkeit

```
Anwendungsfall → Empfohlene Persönlichkeit
─────────────────────────────────────────────
Kundensupport   → professional
Code-Review     → neutral
Brainstorming   → punk
Lernen/Coaching → friend
Schnelle Infos  → terminator
```

### Zu vermeiden

| ❌ Anti-Pattern | Begründung |
|-----------------|------------|
| `punk` für formelle Dokumente | Unprofessionell |
| `terminator` beim ersten Kontakt | Zu kalt |
| V2 ohne API-Budget | Zusätzliche Kosten |

---

## Troubleshooting

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Persönlichkeit wird ignoriert | `personality.engine: false` | Auf `true` setzen |
| Keine Stimmungsanpassung | V2 deaktiviert | `personality.engine_v2: true` |
| Hohe API-Kosten | V2 mit teurem Modell | Günstigeres Modell für V2 wählen |

---

## Zusammenfassung

| Feature | Konfiguration | Empfohlene Nutzung |
|---------|--------------|-------------------|
| **V1 Engine** | `personality.engine: true` | Standard, geringe Kosten |
| **V2 Engine** | `personality.engine_v2: true` | Dynamische Anpassung |
| **Basispersönlichkeit** | `personality.core_personality` | Auswahl des Stils |
| **User Profiling** | `personality.user_profiling: true` | Personalisierung |

> 💡 **Profi-Tipp:** Starte mit V1 und `personality.core_personality: friend` oder `professional`. Aktiviere V2 erst, wenn du dynamische Anpassungen benötigst und das zusätzliche API-Budget hast.

---

**Vorheriges Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)  
**Nächstes Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)
