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

### Einrichtung in der Web-UI
1. Öffne **Config → Personality**.
2. Aktiviere **Personality Engine V1** und wähle ein **Core Personality**-Profil.
3. Speichern und bei Bedarf neu starten.

### YAML-Referenz
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

### Einrichtung in der Web-UI
1. Öffne **Config → Personality** und aktiviere **Personality Engine V2**.
2. Öffne **Config → LLM Settings** und aktiviere den **Helper LLM** (erforderlich für V2-Stimmungsanalyse).
3. Aktiviere optional **User Profiling**, **Emotion Synthesizer** und **Inner Voice** auf der Personality-Seite.
4. Speichern.

### YAML-Referenz
```yaml
# config.yaml
personality:
  engine: true
  engine_v2: true
  v2_provider: ""                    # deprecated – V2 nutzt jetzt llm.helper_*
  user_profiling: false
  user_profiling_threshold: 2
  emotion_synthesizer:
    enabled: false
    min_interval_seconds: 60
    max_history_entries: 100
    trigger_on_mood_change: true
    trigger_always: false
  inner_voice:
    enabled: false                   # erfordert emotion_synthesizer + engine_v2
    min_interval_secs: 60
    max_per_session: 20
    decay_turns: 3
    error_streak_min: 2
```

> ⚠️ **Hinweis:** `v2_provider` ist veraltet. Die V2-Engine nutzt jetzt die Helper-LLM-Konfiguration (`llm.helper_enabled`, `llm.helper_provider`, `llm.helper_model`). Siehe [Kapitel 9: Helper LLM](./09-memory.md#helper-llm--automatisierte-wartung).

### Beide Engines deaktivieren

### Einrichtung in der Web-UI
1. Öffne **Config → Personality**.
2. Deaktiviere **Personality Engine V1** und **Personality Engine V2**.
3. Speichern.

### YAML-Referenz
```yaml
personality:
  engine: false
  engine_v2: false
```

---

## Stimmungszustände (V1/V2)

Die Personality Engine verfolgt die aktuelle Stimmung des Agenten. V1 nutzt heuristische Keyword-/Emoji-Erkennung; V2 kann die Stimmung über den Helper LLM verfeinern.

| Stimmung | Typischer Auslöser | Verhaltenseffekt |
|----------|-------------------|------------------|
| `curious` | Fragen, Erkundungsanfragen | Neutrale Temperatur; fördert Nachfragen |
| `focused` | Positives Feedback, Arbeitsmodus | Leicht niedrigere Temperatur; entschlossen |
| `creative` | Brainstorming, Design-Anfragen | Höhere Temperatur; unkonventionelle Ideen |
| `analytical` | „Warum?“, Vergleiche, Tiefenanalysen | Niedrigere Temperatur; gründliche Analyse |
| `cautious` | Tool-Fehler, negatives Feedback | Niedrigere Temperatur; doppelte Prüfung |
| `playful` | Humor, Witze, lockerer Ton | Höhere Temperatur; leichter Stil |
| `frustrated` | Wiederholte Fehler, Benutzerfrustration | Niedrigere Temperatur; bittet um Klärung |
| `concerned` | Risiko, Sorge, Unsicherheit | Vorsichtig, macht Bedenken explizit |
| `relaxed` | Entspannte, zufriedene Interaktionen | Leicht höhere Temperatur; gesprächig |

Standardstimmung ohne Verlauf: `curious`.

---

## Verfügbare Persönlichkeiten

| Profil | Beschreibung | Ideal für |
|--------|--------------|-----------|
| `neutral` | Sachlich, ausgewogen | Allgemeine Aufgaben, technische Dokumentation |
| `friend` | Warm, unterstützend, Duzen | Persönliche Gespräche, alltägliche Aufgaben |
| `professional` | Höflich, effizient, formell | Business-Kontexte, formelle Kommunikation |
| `punk` | Rebellisch, direkt, unkonventionell | Kreative Projekte, Brainstorming |
| `terminator` | Extrem kurz, direkt, ohne Floskeln | Schnelle Informationen, Kommandozeilen-Modus |
| `psycho` | Chaotisch, unberechenbar, neurotisch | Experimente, Entertainment |
| `mcp` | Master Control Program (TRON-Stil), kalt, imperiös | Systemüberwachung, autoritärer Modus |
| `secretary` | Effizient, vorausschauend, organisiert | Aufgabenverwaltung, Terminplanung |
| `servant` | Äußerst unterwürfig, gehorsam | Rollenspiel, Unterhaltung |
| `thinker` | Analytisch, philosophisch, fragend | Tiefe Analysen, komplexe Probleme |
| `evil` | Megalomane, theatralisch, herrisch | Humorvolle Interaktionen, Rollenspiel |
| `mistress` | Dominant, streng, kompromisslos | Rollenspiel, disziplinierte Interaktionen |

### Persönlichkeit wechseln

#### Über die Web-UI (empfohlen)

1. Öffne **Config → Personality**.
2. Wähle **Core Personality** im Dropdown (z. B. `professional`).
3. Speichere und starte AuraGo neu.

#### YAML-Referenz (Alternative)

```yaml
personality:
  core_personality: "professional"
```

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

> 🖥️ **Web-UI:** Basistemperatur unter **Config → LLM Settings** setzen.

### YAML-Referenz
```yaml
llm:
  temperature: 0.7  # Basistemperatur
```

Die V2-Engine moduliert um diesen Basiswert basierend auf Kontext.

---

## Emotion Synthesizer (V2)

Wenn `personality.emotion_synthesizer.enabled: true` gesetzt ist, erzeugt der Helper LLM nach Stimmungswechseln (oder bei jedem Turn mit `trigger_always: true`) einen strukturierten Emotionszustand. AuraGo speichert kurze natürlichsprachliche Emotionsnotizen und zeigt sie im Chat-Widget und Dashboard an.

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `enabled` | `false` | Emotionssynthese aktivieren |
| `min_interval_seconds` | `60` | Mindestabstand zwischen Synthese-Läufen |
| `max_history_entries` | `100` | Maximale Anzahl gespeicherter Emotions-Einträge |
| `trigger_on_mood_change` | `true` | Synthese bei erkanntem Stimmungswechsel |
| `trigger_always` | `false` | Synthese bei jeder Nachricht |

**Voraussetzungen:** `personality.engine_v2: true` und Helper LLM aktiviert (`llm.helper_enabled: true`).

---

## Inner Voice (V2)

Die Inner Voice ist eine Unterbewusstseins-Engine, die kurze, private Agentengedanken in den System-Prompt injiziert. Sie liefert subtile Verhaltenshinweise ohne zusätzliche sichtbare Benutzernachrichten.

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `enabled` | `false` | Inner-Voice-Generierung aktivieren |
| `min_interval_secs` | `60` | Mindestabstand zwischen Inner-Voice-Gedanken |
| `max_per_session` | `20` | Maximale Inner-Voice-Gedanken pro Session |
| `decay_turns` | `3` | Gedanke verfällt nach N Gesprächsrunden |
| `error_streak_min` | `2` | Mindestanzahl aufeinanderfolgender Fehler für Error-Streak-Trigger |

**Voraussetzungen:** `personality.engine_v2: true`, `personality.emotion_synthesizer.enabled: true` und Helper LLM aktiviert. Inner Voice ist explizit opt-in und wird nicht automatisch mit dem Emotion Synthesizer aktiviert.

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
| **Emotion Synthesizer** | `personality.emotion_synthesizer.enabled: true` | Natürlichsprachliche Emotionsnotizen |
| **Inner Voice** | `personality.inner_voice.enabled: true` | Unterbewusste Verhaltenshinweise |

> 💡 **Profi-Tipp:** Starte mit V1 und `personality.core_personality: friend` oder `professional`. Aktiviere V2 erst, wenn du dynamische Anpassungen benötigst und das zusätzliche API-Budget hast. Konfiguriere zuerst ein kostengünstiges Helper-LLM, bevor du V2, Emotion Synthesizer oder Inner Voice aktivierst.

---

**Vorheriges Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)  
**Nächstes Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)
