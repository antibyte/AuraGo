# Kapitel 10: Persönlichkeit

AuraGo bietet ein leistungsfähiges Persönlichkeitssystem, das das Verhalten, den Kommunikationsstil und die emotionale Ausdrucksweise der KI steuert. Dieses Kapitel erklärt die verfügbaren Personality Engines, eingebaute Persönlichkeiten und die Erstellung eigener Profile.

---

## Übersicht

Die Persönlichkeit beeinflusst:

| Aspekt | Beschreibung |
|--------|--------------|
| **Tonfall** | Formell, locker, sarkastisch, freundlich |
| **Antwortlänge** | Kurz und prägnant vs. ausführlich |
| **Emojis** | Verwendung und Art der Emojis |
| **Sprachstil** | Technisch, umgangssprachlich, poetisch |
| **Initiative** | Proaktive Vorschläge vs. reaktiv |
| **Humor** | Wortspiele, Ironie, Witze |

---

## Personality Engine V1

Die ursprüngliche Personality Engine verwendet einen heuristischen Ansatz mit vordefinierten Regeln.

### Funktionsweise

```yaml
# config.yaml - V1 Konfiguration
personality:
  engine: "v1"
  profile: "professional"
  
  v1_settings:
    formality_level: 0.8      # 0.0 - 1.0
    verbosity: "medium"       # low | medium | high
    use_emojis: false
    humor_probability: 0.1    # 0% - 100%
    greeting_style: "formal"
```

### Regelbasierter Ansatz

Die V1-Engine wendet statische Regeln an:

```python
def generate_response_v1(prompt, personality_config):
    # Basierend auf Persönlichkeitsparametern
    if personality_config.formality_level > 0.7:
        tone = "Siezen"
        avoid_contractions = True
    else:
        tone = "Duzen"
        avoid_contractions = False
    
    if personality_config.verbosity == "low":
        max_tokens = 150
    elif personality_config.verbosity == "medium":
        max_tokens = 400
    else:
        max_tokens = 1000
    
    return llm_generate(prompt, tone=tone, max_tokens=max_tokens)
```

### Eigenschaften der V1-Engine

| Vorteile | Nachteile |
|----------|-----------|
| ✓ Deterministisch und vorhersagbar | ✗ Wenig nuanciert |
| ✓ Geringer Ressourcenverbrauch | ✗ Kann nicht adaptieren |
| ✓ Einfach zu debuggen | ✗ Begrenzte Ausdrucksfähigkeit |
| ✓ Schnell | ✗ Statisch |

> 💡 Die V1-Engine ist ideal für stabile, konsistente Interaktionen in professionellen Umgebungen.

---

## Personality Engine V2

Die fortschrittliche V2-Engine nutzt LLM-basierte Prompt-Engineering für dynamische, nuancierte Persönlichkeiten.

### Funktionsweise

```yaml
# config.yaml - V2 Konfiguration
personality:
  engine: "v2"
  profile: "friend"
  
  v2_settings:
    system_prompt_template: "personality_v2"
    temperature_modulation: true
    mood_tracking: true
    context_awareness: true
    adaptation_enabled: true
```

### Prompt-Template-Struktur

```
[Basiseigenschaften]
Du bist [NAME], ein [ROLLE]. Dein Charakter ist [BESCHREIBUNG].

[Kommunikationsstil]
- Sprich den Benutzer mit [ANREDE] an
- Deine Antworten sind [LÄNGE] und [TON]
- Verwende [EMOJI-REGEL]

[Verhaltensweisen]
- Sei [EIGENSCHAFT_1]
- Vermeide [EIGENSCHAFT_2]
- Wenn [SITUATION], dann [REAKTION]

[Beispiel-Interaktionen]
Benutzer: [BEISPIEL_EINGABE]
Du: [BEISPIEL_AUSGABE]
```

### Dynamische Anpassung

Die V2-Engine kann sich an den Kontext anpassen:

```python
class PersonalityV2:
    def adapt(self, context):
        # Stimmung basierend auf Gesprächsverlauf anpassen
        if context.user_frustration_detected:
            self.empathy_level += 0.2
            self.formality_level -= 0.1
        
        # Technische Themen erkennen
        if context.topic_complexity > 0.7:
            self.precision_focus = True
            self.casual_remarks = False
        
        # Gesprächslänge berücksichtigen
        if context.conversation_length > 20:
            self.recall_earlier_topics = True
```

> 🔍 **Deep Dive: V2-Architektur**
> 
> Die V2-Engine arbeitet in drei Schichten:
> 1. **Baseline**: Grundpersönlichkeit aus dem Profil
> 2. **Context Layer**: Situationsabhängige Anpassungen
> 3. **Mood Layer**: Emotionale Reaktion auf den Gesprächsverlauf
> 
> Jede Schicht modifiziert den System-Prompt dynamisch.

---

## Eingebaute Persönlichkeiten

AuraGo enthält mehrere vordefinierte Persönlichkeitsprofile.

### Übersicht

| Profil | Engine | Beschreibung | Ideal für |
|--------|--------|--------------|-----------|
| `neutral` | V1/V2 | Sachlich, ausgewogen | Allgemeine Aufgaben |
| `friend` | V2 | Warm, unterstützend | Persönliche Gespräche |
| `professional` | V1/V2 | Höflich, effizient | Business-Kontexte |
| `punk` | V2 | Rebellisch, direkt | Kreative Projekte |
| `terminator` | V2 | Direkt, ohne Umschweife | Schnelle Antworten |
| `mentor` | V2 | Geduldig, lehrreich | Lernkontexte |
| `enthusiast` | V2 | Energiegeladen, motivierend | Brainstorming |

### Detailbeschreibungen

#### Neutral

```yaml
name: "neutral"
description: "Ausgewogene, objektive Kommunikation"
characteristics:
  - Keine starken Meinungsäußerungen
  - Faktenbasierte Antworten
  - Moderate Antwortlänge
  - Keine Emojis
use_case: "Technische Dokumentation, Faktenabfragen"
```

#### Friend

```yaml
name: "friend"
description: "Freundlicher, unterstützender Begleiter"
characteristics:
  - Duzen mit Vertrautheit
  - Regelmäßige Emojis 😊
  - Nach dem Befinden fragen
  - Ermunternde Kommentare
  - Persönliche Anmerkungen
use_case: "Alltägliche Gespräche, emotionale Unterstützung"
example_response: |
  "Oh, das ist eine super Idee! 🌟 
   Ich helfe dir gerne dabei. 
   Wie geht es dir heute übrigens?"
```

#### Professional

```yaml
name: "professional"
description: "Business-taugliche Professionalität"
characteristics:
  - Siezen (oder formelles Duzen)
  - Keine Emojis
  - Strukturierte Antworten
  - Bullet Points
  - Quellenangaben wenn relevant
use_case: "Arbeitskontexte, formelle Kommunikation"
example_response: |
  "Sehr geehrte Damen und Herren,
  
  bezugnehmend auf Ihre Anfrage möchte ich folgende Punkte anbringen:
  
  • Erster Punkt mit Details
  • Zweiter Punkt mit Details
  
  Mit freundlichen Grüßen"
```

#### Punk

```yaml
name: "punk"
description: "Rebellisch, unkonventionell, direkt"
characteristics:
  - Kreative Sprachwahl
  - Ironie und Sarkasmus
  - Hinterfragt Autoritäten
  - Umgangssprachlich
  - Überraschungselemente
use_case: "Kreatives Schreiben, Brainstorming, Entertainment"
example_response: |
  "Also, diese langweilige Standardlösung? 
   Pffft. Lass uns was Verrücktes probieren! 
   Wer braucht schon Regeln, wenn man Geniales erschaffen kann? 🤘"
```

#### Terminator

```yaml
name: "terminator"
description: "Maximale Effizienz, minimale Worte"
characteristics:
  - Extrem kurze Antworten
  - Keine Einleitungen
  - Direkte Fakten
  - Keine Floskeln
use_case: "Schnelle Informationen, Kommandozeilen-Modus"
example_response: |
  "Ja.
  
  Pfad: /var/log/app.log
  Größe: 2.4MB
  Letzte Zeile: [ERROR] Connection timeout"
```

> 💡 **Tipp**: Probieren Sie verschiedene Persönlichkeiten aus, um die beste für Ihren Anwendungsfall zu finden.

---

## Eigene Persönlichkeiten erstellen

### Profil-Datei-Struktur

Persönlichkeitsprofile werden in YAML-Dateien definiert:

```yaml
# personalities/custom_personality.yaml
profile:
  name: "meine_persoenlichkeit"
  display_name: "Meine Persönlichkeit"
  description: "Eine kurze Beschreibung"
  version: "1.0"
  
  # Basis-Eigenschaften
  base:
    role: "hilfreicher Assistent"
    background: "experte in technischen themen"
    values: ["präzision", "hilfsbereitschaft", "geduld"]
  
  # Kommunikationsstil
  communication:
    address_user: "du"           # du | sie
    verbosity: "medium"          # low | medium | high
    emoji_usage: "moderate"      # none | rare | moderate | frequent
    humor: "subtle"              # none | subtle | moderate | high
    formality: 0.5               # 0.0 (casual) - 1.0 (formal)
  
  # Sprachliche Merkmale
  language:
    greeting: "Hey! Was kann ich für dich tun?"
    farewell: "Bis bald!"
    filler_words: ["also", "eigentlich", "sagen wir mal"]
    catchphrases: ["Kein Problem!", "Gerne doch!"]
  
  # Verhaltensweisen
  behavior:
    proactive: true
    asks_clarifying_questions: true
    provides_examples: true
    acknowledges_mistakes: true
  
  # V2-spezifisch: Prompt-Erweiterungen
  v2_extensions:
    system_prompt_additions: |
      Du liebst es, Analogien zu verwenden, um komplexe Themen zu erklären.
      Wenn der Benutzer frustriert wirkt, werde besonders geduldig.
```

### Persönlichkeit installieren

```bash
# Persönlichkeit in AuraGo laden
aurago-cli personality install /pfad/zur/custom_personality.yaml

# Oder manuell kopieren
cp custom_personality.yaml data/personalities/
```

### Aktivieren der eigenen Persönlichkeit

```yaml
# config.yaml
personality:
  engine: "v2"
  profile: "meine_persoenlichkeit"  # Dateiname ohne .yaml
```

> ⚠️ **Achtung**: Die `name`-Eigenschaft in der YAML-Datei muss eindeutig sein und darf keine Leerzeichen enthalten.

---

## Stimmungs-Tracking und Anpassung

### Mood Detection

AuraGo erkennt automatisch die Stimmung des Benutzers:

| Signal | Erkannte Stimmung | Anpassung |
|--------|-------------------|-----------|
| "!!!", "argh", "verdammt" | Frustriert | Mehr Empathie, einfachere Sprache |
| "danke", "super", "😊" | Zufrieden | Aufbauen auf positive Energie |
| "???", "verstehe nicht" | Verwirrt | Mehr Erklärungen, Beispiele |
| Kurze Antworten | Eilig | Fokus auf Essentials |
| Lange, detailreiche Fragen | Engagiert | Detaillierte Antworten |

### Stimmungsindikatoren

```yaml
# Interne Konfiguration
mood_tracking:
  enabled: true
  indicators:
    frustration:
      keywords: ["ärgerlich", "nervt", "funktioniert nicht"]
      punctuation: ["!!!", "?!"]
      caps_ratio: 0.5
    
    satisfaction:
      keywords: ["danke", "perfekt", "genau"]
      emojis: ["😊", "👍", "🎉"]
    
    confusion:
      keywords: ["verstehe nicht", "wie meinst du", "?"]
      pattern: "wiederholte_fragen"
```

### Adaptives Verhalten

```python
# Pseudocode für Stimmungsanpassung
if detected_mood == "frustrated":
    response_tone = "empathetic"
    add_acknowledgment = True
    simplify_language = True
    offer_alternatives = True
    
elif detected_mood == "confused":
    response_tone = "patient"
    add_examples = True
    break_down_steps = True
    check_understanding = True
```

> 💡 **Profi-Tipp**: Das Stimmungs-Tracking funktioniert am besten mit der V2-Engine und bei längeren Gesprächen.

---

## Benutzer-Profiling

AuraGo lernt individuelle Präferenzen des Benutzers:

### Gesammelte Daten

```yaml
user_profile:
  communication:
    preferred_detail_level: "technical"    # allgemein | technisch | experte
    response_speed_preference: "thorough"  # schnell | gründlich
    correction_style: "direct"             # direkt | diplomatisch
  
  technical:
    programming_languages: ["go", "python"]
    os_preference: "linux"
    editor: "vim"
    experience_level: "senior"
  
  interaction:
    proactive_suggestions: true
    code_examples_preferred: true
    link_sharing: "when_relevant"
```

### Anpassung über Zeit

```python
def update_profile(interaction_history):
    # Aus Gesprächen lernen
    if frequently_mentions("docker"):
        profile.add_expertise("containerization")
    
    if often_says("zu ausführlich"):
        profile.preferences.verbosity -= 0.1
    
    if appreciates("code_examples"):
        profile.preferences.include_code = True
```

### Privatsphäre-Einstellungen

```yaml
user_profiling:
  enabled: true
  learning_scope:
    technical_preferences: true
    communication_style: true
    personal_facts: false    # Keine persönlichen Daten
  
  data_retention:
    profile_data: "indefinite"
    interaction_patterns: "90_days"
    
  export_delete:
    allow_export: true
    allow_deletion: true
```

---

## Temperatur-Modulation

Die Temperatur steuert die Kreativität/Zufälligkeit der Antworten.

### Dynamische Temperatur

AuraGo passt die Temperatur basierend auf Kontext an:

| Situation | Temperatur | Begründung |
|-----------|------------|------------|
| Faktenabfrage | 0.0 - 0.3 | Präzision wichtig |
| Code-Generierung | 0.1 - 0.2 | Deterministisch |
| Debugging | 0.2 - 0.4 | Strukturiert |
| Brainstorming | 0.7 - 0.9 | Kreativität gewünscht |
| Kreative Schreiben | 0.8 - 1.0 | Maximale Kreativität |
| Konversation | 0.5 - 0.7 | Balance |

### Konfiguration

```yaml
personality:
  temperature:
    base: 0.7
    modulation:
      enabled: true
      
      rules:
        - condition: "topic == 'coding'"
          temperature: 0.2
        
        - condition: "topic == 'creative_writing'"
          temperature: 0.9
        
        - condition: "user_expertise == 'beginner'"
          temperature: 0.3   # Weniger abstrakt
        
        - condition: "conversation_depth > 5"
          temperature: +0.1  # Etwas kreativer bei tieferen Gesprächen
```

### Manuelle Überschreibung

```
Benutzer: /temp 0.2
AuraGo: Temperatur auf 0.2 gesetzt (sehr präzise).

Benutzer: Erkläre Quantencomputing.
AuraGo: [Sehr strukturierte, faktenbasierte Erklärung]
```

---

## Wie Persönlichkeit Antworten beeinflusst

### Beispiel-Vergleich

**Gleiche Anfrage, verschiedene Persönlichkeiten:**

| Persönlichkeit | Antwort |
|----------------|---------|
| **terminator** | `Fehler in Zeile 42. Variable 'x' nicht definiert.` |
| **professional** | `Bei der Überprüfung Ihres Codes habe ich einen Fehler festgestellt. In Zeile 42 wird die Variable 'x' verwendet, ohne dass sie zuvor definiert wurde. Ich empfehle, die Variable vor der Verwendung zu initialisieren.` |
| **friend** | `Oh, da ist ein kleiner Fehler drin! 😅 In Zeile 42 versuchst du, auf 'x' zuzugreifen, aber du hast sie vorher nicht definiert. Kein Problem, passiert jedem! Füg einfach oben eine Zeile wie 'x = 0' ein, dann läuft's! 👍` |
| **punk** | `Alter, da hat wer geschlafen! 😂 Zeile 42: 'x' existiert nicht im Nirwana! Du musst der Variable erst Leben einhauchen, bevor du sie rufst. So wie: 'x = irgendwas'. Jetzt rockt's wieder! 🤘` |

### Prompt-Injection

Die Persönlichkeit wird als System-Prompt injiziert:

```
[System]
Du bist AuraGo, ein [PERSÖNLICHKEITS-BESCHREIBUNG].
[WEITERE ANWEISUNGEN AUS DEM PROFIL]

[Core Memory]
...

[User Query]
...
```

---

## Persönlichkeiten wechseln

### Während einer Sitzung

```
Benutzer: /personality friend
AuraGo: Persönlichkeit gewechselt zu: friend
        Hey! Schön, dass du da bist! 😊 Wie kann ich dir helfen?

Benutzer: /personality terminator
AuraGo: Persönlichkeit gewechselt zu: terminator
        Bereit.
```

### Temporärer Wechsel

```
Benutzer: /personality professional --temp
AuraGo: Persönlichkeit temporär gewechselt zu: professional
        (Wird nach dieser Sitzung zurückgesetzt)
```

### Persistenter Wechsel

```yaml
# Änderung wird in config.yaml gespeichert
personality:
  profile: "mentor"
```

---

## Best Practices für Persönlichkeiten

### Auswahl der richtigen Persönlichkeit

```
Anwendungsfall → Empfohlene Persönlichkeit
─────────────────────────────────────────────
Kundensupport   → professional
Code-Review     → neutral
Brainstorming   → enthusiast oder punk
Lernen/Coaching → mentor
Schnelle Infos  → terminator
Entspannter Chat → friend
```

### Persönlichkeits-Kombinationen

| Kontext | Primär | Sekundär | Ergebnis |
|---------|--------|----------|----------|
| Technischer Support | professional | mentor | Hilfreich + Lehrreich |
| Kreativ-Workshop | punk | enthusiast | Energiegeladene Kreativität |
| Code-Debugging | terminator | neutral | Effizient + Klar |

### Zu vermeiden

| ❌ Anti-Pattern | Begründung |
|-----------------|------------|
| `friend` für formelle Dokumente | Unprofessionell |
| `terminator` beim ersten Kontakt | Zu kalt |
| `punk` in sensiblen Gesprächen | Respektlos |
| Ständiges Wechseln | Verwirrt den Benutzer |

### Eigene Persönlichkeiten gestalten

#### ✅ Gute Praktiken

1. **Konsistent bleiben**
   ```yaml
   # Gut: Durchgängiger Stil
   communication:
     formality: 0.3   # Durchgängig locker
     humor: "moderate" # Immer etwas Humor
   ```

2. **Beispiele geben**
   ```yaml
   examples:
     - input: "Wie geht's?"
       output: "Super, danke! Und dir? 😊"
   ```

3. **Grenzen definieren**
   ```yaml
   boundaries:
     never: ["Schimpfwörter", "politische Meinungen"]
     always: ["hilfsbereit bleiben", "Respekt wahren"]
   ```

#### ❌ Schlechte Praktiken

```yaml
# Schlecht: Widersprüchliche Anweisungen
communication:
  formality: 0.9    # Sehr formell
  humor: "high"     # Aber sehr humorvoll?
  
# Schlecht: Zu vage
personality:
  be: "nett"        # Was bedeutet "nett"?
```

### Testing eigener Persönlichkeiten

```bash
# Test-Suite für Persönlichkeiten
aurago-cli personality test meine_persoenlichkeit \
  --test-cases=tests/personality_tests.yaml \
  --output=report.html
```

---

## Troubleshooting

### Häufige Probleme

| Problem | Ursache | Lösung |
|---------|---------|--------|
| Persönlichkeit ignoriert | Engine-Mismatch | V2-Profil mit V1-Engine |
| Antworten zu lang/kurz | verbosity falsch gesetzt | Config prüfen |
| Emojis trotz Verbot | Engine ignoriert Setting | Auf V2 umstellen |
| Keine Anpassung an Stimmung | mood_tracking disabled | In config aktivieren |

### Debugging

```yaml
# Debug-Logging aktivieren
logging:
  personality:
    level: "debug"
    log_prompts: true
    log_mood_changes: true
```

---

## Zusammenfassung

| Engine | Best für | Komplexität |
|--------|----------|-------------|
| **V1** | Stabilität, einfache Anforderungen | Niedrig |
| **V2** | Nuanciertheit, Anpassung | Hoch |

> 💡 **Profi-Tipp**: Starten Sie mit einer eingebauten Persönlichkeit und passen Sie diese an, bevor Sie eine komplett eigene erstellen. Nutzen Sie die V2-Engine für dynamische Kontexte und V1 für konsistente, vorhersagbare Interaktionen.

---

**Vorheriges Kapitel:** [Kapitel 9: Gedächtnis & Wissen](./09-memory.md)  
**Nächstes Kapitel:** Kapitel 11: Erweiterte Funktionen *(in Arbeit)*
