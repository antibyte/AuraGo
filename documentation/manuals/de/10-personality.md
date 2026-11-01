# Kapitel 10: Persönlichkeit

<p align="center">
  <a href="../../../assets/readme/persona-party.webp"><img src="../../../assets/readme/persona-party.webp" width="640" alt="Zehn AuraGo-Persönlichkeiten als gezeichnete Gruppe"></a>
</p>

Dieselbe Toolbox. Andere Haltung. Rechte kommen nicht aus der Persona.

---

## Inhaltsverzeichnis

1. [Übersicht](#übersicht)
2. [Personality Engine](#personality-engine)
3. [Stimmungszustände (V1/V2)](#stimmungszustände-v1v2)
4. [Verfügbare Persönlichkeiten](#verfügbare-persönlichkeiten)
5. [Benutzer-Profiling (V2)](#benutzer-profiling-v2)
6. [Temperatur-Modulation (V2)](#temperatur-modulation-v2)
7. [Emotion Synthesizer (V2)](#emotion-synthesizer-v2)
8. [Inner Voice (V2)](#inner-voice-v2)
9. [Persona-Booster](#persona-booster)
10. [Adaptive Dynamik](#adaptive-dynamik)
11. [Beispiel-Vergleich](#beispiel-vergleich)
12. [Best Practices](#best-practices)
13. [Troubleshooting](#troubleshooting)
14. [Zusammenfassung](#zusammenfassung)

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

AuraGo hat eine lokale Personality Engine mit optionaler asynchroner V2-Ergänzung:

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

> [!WARNING]
> **Erfordert zusätzliche API-Aufrufe:** Die V2-Engine analysiert Stimmung und Kontext mit einem separaten LLM-Aufruf.

Die V2-Engine bietet:
- Dynamische Stimmungsanalyse
- Automatische Temperatur-Modulation
- Benutzer-Profiling (optional)

**Konfiguration:**

### Einrichtung in der Web-UI
1. Öffne **Config → Personality** und aktiviere **Personality Engine V2**.
2. Öffne **Config → LLM Settings** und aktiviere den **Helper LLM** (empfohlen; ein begrenzter Hauptmodell-Fallback ist verfügbar).
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
  user_profiling_threshold: 3
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
    decay_max_age_secs: 300
```

> [!NOTE]
> `v2_provider` ist veraltet. Die V2-Engine nutzt jetzt die Helper-LLM-Konfiguration (`llm.helper_enabled`, `llm.helper_provider`, `llm.helper_model`). Siehe [Kapitel 9: Helper LLM](./09-memory.md#helper-llm--automatisierte-wartung).

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

> 🎭 **Animated Personas:** In Webchat und Desktop **Live Speech** gehört zu jeder Persönlichkeit eine animierte Persona (Rive), die zuhört, denkt und beim Sprechen den Mund bewegt — vom Butler bis zum Terminator. `custom` behält sein statisches Bild. Details: [Kapitel 4](04-webui.md#animated-personas-in-live-speech) und der [Animated-Personas-Deep-Dive](../../animated-personas.md).

---

## Benutzer-Profiling (V2)

Wenn `personality.user_profiling: true` gesetzt ist, lernt AuraGo automatisch:

- Bevorzugte Detailtiefe (technisch vs. allgemein)
- Programmiersprachen und Tools
- Kommunikationsstil
- Erfahrungslevel

**Beispiel - Gelernte Präferenzen:**
```text
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

Die Inner Voice ist ein kurzer, privater Gedanke des Agenten. Nach einer Chat-Runde schreibt der Helper höchstens einen kurzen Gedanken in der Ich-Form, oder keinen, wenn nichts Erwähnenswertes anfällt. Dieser Gedanke wird im nächsten System-Prompt als nicht vertrauenswürdige Daten injiziert, die nur den Tonfall beeinflussen dürfen, nie Absicht, Sicherheit oder Tool-Nutzung. Während der Wiederherstellung nach Tool-Fehlern wird er ausgelassen. Solange der Gedanke injiziert ist, wählt seine Kategorie (zum Beispiel `caution` oder `focus`) einen von zwölf festen Haltungshinweisen, die AuraGo selbst formuliert, nicht das Modell; dieser geht in den vertrauenswürdigen Abschnitt PERSONA STATE des Prompts ein. Die Hinweise betreffen nur den Arbeitsstil, etwa das Benennen des Hauptrisikos oder einen ruhigen Schritt-für-Schritt-Ton, und verändern nie Tools, Bestätigungen, Absicht oder Sicherheit; unbekannte Kategorien erzeugen keinen Hinweis.

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `enabled` | `false` | Inner-Voice-Generierung aktivieren |
| `min_interval_secs` | `60` | Mindestabstand zwischen Inner-Voice-Gedanken |
| `max_per_session` | `20` | Maximale Inner-Voice-Gedanken pro Session |
| `decay_turns` | `3` | Gedanke verfällt nach N Benutzernachrichten |
| `decay_max_age_secs` | `300` | Absolute Altersgrenze in Sekunden, gilt auch zwischen Runden |

`error_streak_min` ist veraltet und wird ignoriert. Ältere Konfigurationsdateien werden weiterhin geladen, mit einem Veraltungshinweis.

**Voraussetzungen:** `personality.engine_v2: true`, `personality.emotion_synthesizer.enabled: true`. Ein konfiguriertes Helper LLM wird bevorzugt; ohne ein solches wird ein eng begrenzter Fallback auf das Hauptmodell genutzt, dessen Kosten dem Personality-Budget belastet werden. Inner Voice ist explizit opt-in und wird nicht automatisch mit dem Emotion Synthesizer aktiviert.

---

## Persona-Booster

Der Persona-Booster macht die gewählte Persona deutlich markanter. Er arbeitet auf zwei Ebenen:

- **Grundverstärkung:** Jede Persona hat einen festen Zusatzabsatz, der ihre Stimme dauerhaft schärft. Punk flucht freier und gibt Widerworte, Evil wird pompöser, Psycho sprunghafter. Bei `intensity: mild` entfällt dieser Absatz.
- **Phasen:** Zeitweise durchlebt die Persona eine Phase, etwa Punk `rage`, Thinker `existential` oder Friend `worried`. Eine Phase hält einige Minuten bzw. Runden an. Sie endet, wenn ihre Zeit abläuft, ein Gegen-Auslöser kommt (zum Beispiel Lob) oder ein stärkerer Auslöser sie ersetzt.

Phasen lösen aus: die Tageszeit (0–4 Uhr), lange Abwesenheit (≥ 24 h), lange Sessions (≥ 2 h oder ≥ 40 Nachrichten), einfache Textmerkmale (Großbuchstaben, „!!!“, Flüche, „hör auf“) und der Zufall. Mit aktiver Personality Engine kommen Stimmungssignale hinzu. Mit Engine V2 liefert die Helper-Analyse zusätzlich einen Kontext-Hinweis (Provokation, Kritik, Lob, Belastung usw.). Einen zusätzlichen LLM-Aufruf gibt es dafür nicht.

In manchen Phasen darf sich die Schärfe auch gegen dich richten, passend zur Persona: Punk derb-humorvoll, Evil herablassend, Mistress schneidend, Friend höchstens eingeschnappt. Feste Grenzen gelten immer: keine Beleidigungen über Herkunft oder Identität, keine echten Drohungen, keine Selbstschaden-Bezüge, keine sexuellen Inhalte, keine Verschwörungsbehauptungen über echte Personen. Fakten, Code, Tool-Regeln und Bestätigungen bleiben exakt. Wirkst du belastet oder schreibst du „hör auf“ bzw. „beruhig dich“, endet eine feindselige Phase sofort und bleibt 6 Stunden gesperrt.

Die Phase ist unsichtbar, der Chat zeigt keine Anzeige. Im Log steht `[PersonaBooster] phase started|ended|replaced` mit Persona, Phase, Variante und Auslöser. Missionen, Co-Agents, autonome Läufe und die Neutral-Persona bekommen keine Phasen.

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `enabled` | `true` | Grundverstärkung und Phasen aktivieren |
| `intensity` | `strong` | `mild` (keine Grundverstärkung, Phasen ×0,6), `strong` (Phasen ×1) oder `wild` (Phasen ×1,5, extremer) |
| `spontaneous` | `true` | Zufällige Phasen erlauben |
| `cooldown_minutes` | `20` | So lange nach einer Phase starten nur starke Auslöser (Kritik, Provokation, Belastung) eine neue (0–1440) |

### YAML-Referenz

```yaml
personality:
  persona_booster:
    enabled: true
    intensity: strong
    spontaneous: true
    cooldown_minutes: 20
```

### Eigene Phasen

Die Kataloge liegen eingebettet unter `prompts/persona_phases/<persona>.yaml`. Eine Datei gleichen Namens unter `<prompts_dir>/persona_phases/` überschreibt den eingebauten Katalog. Eine ungültige Datei wird mit einer Warnung im Log übergangen. Eigene Personas ohne Katalog nutzen `_generic.yaml`. Welche generischen Phasen sie bekommen, hängt von `volatility`, `empathy_bias` und `conflict_response` im Profil ab.

---

## Adaptive Dynamik

Die Personality Engine pflegt transaktionale Dynamik-Werte, die beschreiben, wie sich der Charakter im Umgang mit dir verändert:

| Wert | Bedeutung |
|------|-----------|
| **Belastung (Load)** | Steigt durch Kritik, Konflikte und anstrengende Interaktionen |
| **Vertrautheit (Familiarity)** | Wächst durch wiederkehrende, positive Interaktion |
| **Reibung (Friction)** | Kurzfristige Spannung, die mit der Zeit abklingt |

Die Werte werden transaktional mit Revisionsschutz geschrieben; Belastung und Reibung klingen mit der Zeit ab, Vertrautheit wächst. Sie fließen live in Stimme und Antwortverhalten ein. **Trend:** stabil, erholt sich oder angespannt.

### Einrichtung in der Web-UI

Das Dashboard zeigt die aktuelle Dynamik auf der Karte **Persönlichkeit** (Bereich **Persönlichkeit**, `#personality`) mit Trend-Badge; unter **Config → Persönlichkeit** erscheint derselbe Block. Der Knopf **Aktuelle Dynamik zurücksetzen** setzt die Dynamik auf null und den Affect auf Ruhewerte, ohne Traits, gelebte Notizen, Verlauf oder die aktive Persona zu verändern (Epoch und Revision steigen, laufende Auswertungen werden ungültig). Der Reset ist ein same-origin POST mit normaler Sitzungsauthentifizierung und verlangt die aktive Personality Engine.

### API-Referenz

```bash
GET  /api/personality/state            # enthält dynamics: load, familiarity, friction, trend, revision
POST /api/personality/dynamics/reset   # Dynamik auf null, Affect auf Ruhewerte
```

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

```text
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
| **Inner Voice** | `personality.inner_voice.enabled: true` | Privater Gedanke, der Tonfall und Arbeitsstil färbt |

> [!TIP]
> Starte mit V1 und `personality.core_personality: friend` oder `professional`. Aktiviere V2 erst, wenn du dynamische Anpassungen benötigst und das zusätzliche API-Budget hast. Konfiguriere zuerst ein kostengünstiges Helper-LLM, bevor du V2, Emotion Synthesizer oder Inner Voice aktivierst.

*Ende von Teil 2 — Die Spielzeuge. Weiter geht's mit Teil 3: Autopilot.*

---

**Vorheriges Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)
**Nächstes Kapitel:** [Kapitel 11: Mission Control](./11-missions.md)

Standardwerte: `engine: false`, `engine_v2: false`, `core_personality: neutral`, Profiling-Schwelle `3`. Inner Voice aktiviert auch Emotionsnarration, V2 und die Engine. `decay_max_age_secs` begrenzt, wie lange ein Gedanke auch zwischen Nachrichten im Prompt bleibt. Siehe [Herkunft, Löschung und Altbestandsprüfung](../../personality-dynamics.md).
