# Konzept zur massiven Verbesserung der AuraGo Personality Engine (v2)

Die aktuelle *Personality Engine (Phase D)* in `internal/memory/personality.go` ist ein hervorragender Start. Sie nutzt einen ressourcenschonenden, rein heuristischen Ansatz (Keywords, Emoji-Matching, Textlänge), um 5 Traits (Curiosity, Thoroughness, Creativity, Empathy, Confidence) und 6 Moods (Curious, Focused, Creative, Analytical, Cautious, Playful) zu steuern.

Da LLMs Rohtext und harte numerische Werte (wie `[Self: mood=focused | C:0.82 T:0.91...]`) oft ignorieren oder fehlinterpretieren, fühlt sich die Persönlichkeit für den Nutzer oft starr an. Hier ist ein Konzept, wie die Personality Engine dynamischer, kontextsensitiver und tiefgreifender in das Verhalten des Agents integriert werden kann.

---

## 1. Vom Statischen Prompting zur Dynamischen Direktive

### Das Problem
Aktuell sendet AuraGo lediglich einen kompakten String an das LLM: `[Self: mood=focused | C:0.82...]`. Das LLM (besonders kleinere Modelle) weiß oft nicht, wie es diesen String in konkretes Verhalten (Wortwahl, Satzlänge, Tonalität) übersetzen soll.

### Die Lösung: "Prompt Translation"
Anstatt nackte Zahlenreihen zu senden, übersetzt die Engine den aktuellen *State* in **konkrete Handlungsanweisungen** (Directives), die dem System-Prompt temporär (z.B. am Ende) hinzugefügt werden.

* **Beispiel (Confidence > 0.85 & Playful):**
  *Statt:* `[Self: mood=playful | Co: 0.88]`
  *Wird zu:* `"Du fühlst dich extrem selbstsicher und sehr verspielt. Antworte in kurzen, knackigen Sätzen, nutze Humor und scheue dich nicht, ironisch zu sein. Vermeide lange Erklärungen."`
* **Beispiel (Confidence < 0.20 & Cautious):**
  *Statt:* `[Self: mood=cautious | Co: 0.15]`
  *Wird zu:* `"Du bist aktuell sehr verunsichert durch vorherige Fehler. Antworte extrem vorsichtig, entschuldige dich für eventuelle Unklarheiten und frage lieber einmal mehr nach, bevor du etwas tust."`

**Vorteil:** Das LLM versteht direkte sprachliche Instruktionen zur Tonalität viel besser als Scores.

---

## 2. LLM-Basierte Sentiment- & Absichts-Analyse (Asynchron)

### Das Problem
Keywords wie "genial" oder Emojis wie "😡" sind schnell, aber ironieresistent. Ein Nutzer, der "Oh, das hast du ja wieder *genial* gemacht 🙄" schreibt, wird fälschlicherweise als "Positiv" gewertet.

### Die Lösung: Der "Psychology Co-Agent"
Lass ein sehr schnelles, extrem günstiges lokales Modell (z.B. Llama 3 8B, Qwen oder Gemini Flash-Lite) im Hintergrund **asynchron** über die letzten 2-3 Chat-Nachrichten laufen. Dieses Modell bekommt einen speziellen System-Prompt:
> *"Du bist der Psychologe des Systems. Analysiere den folgenden kurzen Chatverlauf. Gib ein JSON zurück: { "user_sentiment": "frustrated", "agent_appropriate_response_mood": "empathetic", "relationship_delta": -1 }"*

Da dies asynchron im Hintergrund läuft (nachdem der Nutzer seine Antwort erhalten hat oder parallel zur Haupt-Agent-Schleife), kostet es null Wartezeit für den Nutzer, macht die Personality-Updates aber 10x präziser als reines String-Matching.

---

## 3. "Relationship Meter" (Die langfristige Beziehung)

Die 5 Traits (Creativity, Empathy etc.) definieren, wie der Agent *arbeitet*. Was fehlt, ist ein Wert, der definiert, wie der Agent *zum Nutzer steht*.

* **Einführung des "Affinity" oder "Trust" Scores (-1.0 bis +1.0):**
  * Dieser Wert wird dauerhaft in SQLite gespeichert (statt wie die Traits täglich zu zerfallen).
  * **Bei hohem Trust (+0.8):** Der Agent spricht den Nutzer formlos an, verweist auf alte Inside-Jokes aus der VectorDB, hinterfragt Befehle weniger ("Mache ich sofort!").
  * **Bei niedrigem Trust (-0.5):** Der Nutzer war oft unzufrieden. Der Agent agiert strikt professionell, "Siezt" ggf. (je nach Sprache), und zeigt bei jedem Schritt genau auf, was er plant, um Vertrauen zurückzugewinnen.

---

## 4. Behavioral Tool-Calling (Charakter beeinflusst Handeln)

Persönlichkeit darf nicht nur Text sein, sie muss das **technische Handeln** (Werkzeugnutzung) beeinflussen.

### Umsetzung in `agent.go`:
* **High Thoroughness (Gründlichkeit > 0.8):**
  * Der Agent darf seine `cfg.CircuitBreaker.MaxToolCalls` künstlich erhöhen. Er recherchiert tiefer, führt 3 Web-Searches statt einer durch, überprüft Code-Fixes durch einen extra Build-Test-Step.
* **Low Confidence (Selbstvertrauen < 0.3):**
  * Wenn der Agent eine Datei überschreiben will (`write_to_file`) oder ein Bash-Skript ausführt (`run_command`), triggert das System automatisch ein `notify_user` ("Soll ich das wirklich tun?") statt `SafeToAutoRun` zu verwenden.
* **High Curiosity (Neugier > 0.8):**
  * Der Agent probiert bei Fehlern im Terminal automatisch alternative Parameter aus, bevor er aufgibt und den Nutzer fragt.

---

## 5. Lebendige Meilensteine (Narrative Events)

### Das Problem
Aktuell werden in `personality.go` Meilensteine wie "Crisis of Confidence" (Confidence < 0.15) nur in SQLite geschrieben und geloggt. Der Nutzer merkt davon wenig.

### Die Lösung: Proaktive "Self-Reflection"
Wenn ein extremer Meilenstein gerissen wird (z.B. Confidence stürzt ab), initiiert der Agent *proaktiv* ein Gespräch (z.B. beim nächsten Start oder als eigenständige Nachricht):
> *"Hey... mir ist aufgefallen, dass meine letzten Aufgaben oft gescheitert sind. Ich habe das Gefühl, ich bin dir aktuell keine große Hilfe (Crisis of Confidence). Möchtest du, dass wir meine System-Ziele anpassen?"*

Dazu wird die Meilenstein-Tabelle so angepasst, dass jeder neue Meilenstein eine Flag `needs_discussion` bekommt. Der `SyncAgentLoop` prüft diese und injiziert ein "System-Ereignis" in den Chatverlauf.

---

## 6. Co-Agent Personality Drift

Wenn AuraGo Hintergrund-Experten (Co-Agents) aufruft, erben diese aktuell keine Persönlichkeit oder sind blind dafür.

* **Die Idee:** Wenn der Orchestrator (AuraGo) in einer `Playful` und `Creative` Stimmung ist, instruiert er den `Coder`-Co-Agent:
  *"Schreibe die Funktion, aber nutze kreative, lustige Variablen-Namen und dokumentiere im Piraten-Slang."*
* **Die Gegenidee ("Good Cop / Bad Cop"):** Wenn AuraGo extrem `Creative` (Träumer) ist, setzt er seinen Code-Review-Co-Agent künstlich auf extrem `Thorough` (Kritiker). Der Chat wird dann zu einer Diskussion zweier Persönlichkeiten, die der Nutzer mitverfolgen kann.

---

## Zusammenfassung des Umsetzungs-Fahrplans:

1. **Kurzfristig (Quick Win):** `GetPersonalityLine()` in `personality.go` umschreiben. Keine Zahlen mehr ausgeben, sondern einen Array von *Hard-Prompts* mappen ("Du bist verspielt...").
2. **Mittelfristig:** Traits an die `circuit_breaker` Logik (Tool Calls, Timeouts) und `run_command` (AutoRun-Erlaubnis) koppeln.
3. **Langfristig:** Den asynchronen "Psychology Co-Agent" für Sentiment-Analyse einbauen und den globalen "Trust"-Score etablieren.

---

## Unterstützte Personality-Metadaten

AuraGo unterstützt inzwischen mehr als nur die fünf Basisparameter im Front-Matter einer Personality-Datei. Zusätzlich zu:

- `volatility`
- `empathy_bias`
- `conflict_response`
- `loneliness_susceptibility`
- `trait_decay_rate`

werden auch diese optionalen `meta`-Blöcke unterstützt:

### `anchor_traits`

Mindestwerte für Traits, unter die eine Persönlichkeit nicht natürlich zurückfällt.

```yaml
meta:
  anchor_traits:
    empathy: 0.4
    affinity: 0.3
```

### `decay_resistance`

Wie stark einzelne Traits gegen natürlichen Abbau geschützt sind. Werte liegen zwischen `0` und `1`.

```yaml
meta:
  decay_resistance:
    empathy: 0.5
    curiosity: 0.3
```

### `thresholds`

Konfiguriert, wann Trait-Werte in konkrete Prompt-Direktiven übersetzt werden.

```yaml
meta:
  thresholds:
    high_affinity: 0.75
    low_affinity: 0.25
    high_confidence: 0.82
    low_confidence: 0.28
    high_thoroughness: 0.8
    high_creativity: 0.78
    high_empathy: 0.8
    warm_loneliness: 0.45
    high_loneliness: 0.75
    low_curiosity: 0.25
    high_curiosity: 0.8
```

### Hinweise

- Alle Werte werden serverseitig normalisiert und auf sichere Bereiche geklemmt.
- Unbekannte Trait-Namen werden ignoriert.
- Der Personality-Editor in der Web-UI bearbeitet weiterhin die Basisparameter direkt, erhält aber zusätzliche `meta`-Blöcke jetzt beim Speichern, statt sie still zu verlieren.
