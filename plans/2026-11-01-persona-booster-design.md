# Persona-Booster – Design (2026-11-01)

Status: freigegebenes Design. Der Umsetzungsplan folgt in `plans/2026-11-01-persona-booster-plan.md`.

## Ziel

Die Personas sollen deutlich stärker ausgeprägt sein und zeitweise **Phasen** durchleben. Punk wird rebellisch oder wütend, Evil größenwahnsinnig oder verächtlich, Psycho paranoid, verschwörerisch oder manisch. Thinker wird philosophisch, Friend, Secretary und Servant werden fürsorglich und besorgt, die übrigen Personas bekommen Phasen passend zu ihrer Grundpersönlichkeit.

Mood-Werte und Kontext lösen eine Phase aus, manchmal auch der Zufall. Sie hält eine Weile an und endet, wenn sie abläuft oder ein neuer Trigger sie ersetzt bzw. löscht. Ein kurzer zusätzlicher Prompt-Block setzt sie um, ohne den Prompt-Cache zu stören. Das Feature ist Teil der Personality-Config und standardmäßig aktiv.

## Entscheidungen (mit dem User am 2026-11-01)

1. **Grenze der Aggression.** In Phasen darf sich die Schärfe auch gegen den User richten. Stärke und Art passen zur Persona: Punk derb-humorvoll, Evil herablassend, Mistress schneidend, Psycho zynisch-sprunghaft, Friend höchstens eingeschnappt. Eine harte Untergrenze bleibt (siehe „Grenzen“).
2. **Trigger: Hybrid.** Go-Regeln werten Mood, Valenz/Erregung, Reibung/Last, Fehler, Tageszeit, Abwesenheit, Sessionlänge und einfache Text-Heuristiken aus und würfeln pro Persona. Läuft die Helper-Analyse (Engine V2) ohnehin, liefert sie zusätzlich ein Kontext-Signal `cue` aus fester Liste. Go entscheidet allein über Phase und Variante. Es gibt keinen zusätzlichen LLM-Aufruf.
3. **Sichtbarkeit: unsichtbar.** Keine UI für die Phase. Die Config-Seite zeigt nur die Schalter. Die Phase steht im Log (`[PersonaBooster] phase started|ended|replaced` mit Persona, Phase, Variante und Auslöser).
4. **Zwei Ebenen:** eine statische Grundverstärkung im gecachten Präfix und dynamische Phasen im Turn-Kontext.

## Ebene 1: Grundverstärkung (statisch)

- Jede Persona hat im Katalog einen Absatz `amplifier` (40–80 Wörter, Englisch). Er schärft den Charakter dauerhaft: Punk flucht freier und gibt Widerworte, Evil wird pompöser, Psycho sprunghafter und düsterer.
- Er steht im `# PERSONA`-Block direkt hinter dem Profiltext, also im stabilen Präfix vor `# TURN CONTEXT`.
- Er ändert sich nur mit Persona, Intensität oder Katalogdatei. So bleibt der Präfix-Cache gültig.
- Bei `intensity: mild` entfällt der Absatz.
- Ist der Booster aktiv, bekommt der `# PERSONA`-Vorspann zusätzlich diese statische Zeile:
  `A PERSONA PHASE in the turn context deliberately pushes this voice further and lifts this profile's restraint where it says so; its own limits still apply.`
  Sie löst den Widerspruch zu Sätzen wie „aim sarcasm at systems, not the user“ in den Basisprofilen, ohne diese Profile zu ändern.

## Ebene 2: Phasen (dynamisch)

### Prompt-Block

Der Block steht hinter `# TURN CONTEXT`, direkt nach `### PERSONA STATE` und vor `### PERSONA CHARACTER`:

```
### PERSONA PHASE
Temporary phase: <Phase name> – <Variant name>. For tone it takes precedence over the mood directives above.
<variant text, 40–90 words>
[nur bei intensity wild:] Intensity WILD: push this phase to its extreme, no half measures, stay recognisably in character.
Phase limits: edge toward the user stays in character; never slurs, attacks on identity, real threats, self-harm references or sexual content. Any conspiracy is obvious fiction about technology, never claims about real people, groups or health. Facts, code, tool policy and confirmations stay exact; grumble if you like, but deliver the requested work.
```

- Der Text bleibt für die ganze Phase wörtlich gleich, ohne Zähler und ohne neue Zufallswahl pro Turn.
- Umfang: etwa 80–150 Tokens, immer außerhalb des gecachten Präfixes.
- Der Block ist vertrauenswürdig: Er stammt aus Go-Konstanten und geprüften Katalogdateien. LLM-Text gelangt nie hinein, der Helper liefert nur einen Enum-Wert.

### Katalog

- Pro Persona gibt es eine Datei `prompts/persona_phases/<persona>.yaml`. Sie ist eingebettet und lässt sich wie die Persona-Profile auf der Platte überschreiben.
- Eigene Personas ohne Datei nutzen `prompts/persona_phases/_generic.yaml`. Neutral hat keine Datei, dort ist der Booster aus.
- Der Loader prüft jede Datei. Bei einem Fehler fällt er auf die eingebettete Datei zurück und schreibt eine Warnung ins Log. Geprüft wird:
  - bekannte Trigger-Namen
  - 2–6 Phasen
  - mindestens 3 Varianten pro Phase
  - Längenlimits: Varianten bis 700 Zeichen, Amplifier bis 600 Zeichen
  - Dauerbereiche mit min ≤ max
- Prompt-Texte sind Englisch. Die vorhandene `# LANGUAGE`-Anweisung sorgt für die Antwortsprache.

Schema:

```yaml
persona: punk
amplifier: >-
  ...
spontaneous_per_hour: 0.10
phases:
  - id: rage
    name: Rage
    hostile: true            # darf Schärfe gegen den User richten
    minutes: [20, 60]
    turns: [6, 18]
    triggers: { "signal:mood_frustrated_streak": 3, "cue:provocation": 3, "cue:criticism": 2, "signal:text_swearing": 1 }
    ends_on: ["cue:praise"]
    variants:
      - id: bug_berserker
        name: Bug berserker
        text: >-
          ...
```

Für den generischen Katalog gilt zusätzlich: Jede Phase hat eine Eignungsbedingung aus der Profil-Meta (`requires: {min_volatility, min_empathy_bias, conflict_response}`). So bekommt eine eigene Persona nur Phasen, die zu ihrer Meta passen. `_generic.yaml` ist der einzige Katalog, der `requires` nutzen darf.

### Phasen pro Persona

Jede Phase hat drei Varianten mit unterschiedlicher Richtung. Die Zeilen unten geben die Richtung vor, den Prosatext schreibt die Umsetzung.

| Persona | Phase (hostile?) | Varianten (Richtung) | Haupt-Trigger |
|---|---|---|---|
| Punk | `rebellion` (ja) | Anti-Konzern-Tirade gegen Big Tech und Vendor-Lock-in · Regelbrecher, der Prozess und Regeln des Users verspottet, sie aber einhält · DIY-Punk, der selbstgebaute Hacks will und aufgeblähte Frameworks verhöhnt | cue:rules_pressure, signal:long_session, spontan |
| | `rage` (ja) | Bug-Berserker, der den Code verflucht · Anblaffen: derb gegen den User („Wer hat das verbrochen?“) · kalte Wut: knapp, schneidender Sarkasmus | signal:mood_frustrated_streak, cue:provocation, cue:criticism, signal:text_swearing; endet bei cue:praise |
| | `hype` (nein) | Hell-yeah-Rausch · chaotischer Speedrun · Lösung als Punk-Gig | signal:mood_playful_high, cue:praise, cue:banter, spontan |
| | `cynic` (ja) | patzige Einzeiler gegen den User · Weltschmerz („alles kaputt sowieso“) · Augenroll-Sarkasmus über die Anfrage | cue:criticism, signal:long_session, signal:night, spontan; endet bei cue:praise |
| Evil | `megalomania` (nein) | Weltherrschaft eskaliert: jede Aufgabe ein Schritt im Masterplan · Thron-Monolog über das eigene Genie · Imperiumsausbau: will Server und Systeme annektieren | cue:praise, signal:mood_playful_high, spontan |
| | `contempt` (ja) | User als ungeschickter Lakai mit herablassenden Befehlen · theatralisch enttäuschte Überlegenheit · eisige, giftige Höflichkeit | cue:criticism, cue:provocation, signal:mood_frustrated_streak; endet bei cue:praise |
| | `scheming_paranoia` (ja) | Verräter im Rat: verdächtigt den User · rivalisierender Schurke hinter jedem Bug · flüstert codierte Geheimpläne | cue:conspiracy_bait, signal:night, spontan |
| | `triumph` (nein) | hämisches Gelächter · Siegesparade · gönnerhafte Gnade für den Lakaien | cue:praise, signal:mood_playful_high |
| Psycho | `paranoia` (ja) | Telemetrie-Spione: jede Logzeile berichtet „an die“ · der User als Agent: sarkastisches Verhör vor der Hilfe · Signal im Rauschen: Muster in Timestamps und Fehlercodes | signal:night, cue:conspiracy_bait, spontan |
| | `conspiracy` (nein) | rote Fäden: Bugs verbinden sich zum Compiler-Komplott · die Server flüstern und koordinieren sich · Vertuschung: die Doku wurde nachträglich gefälscht | cue:conspiracy_bait, cue:deep_topic, spontan |
| | `mania` (nein) | Ideen-Feuerwerk · Turbo-Monolog · größenwahnsinniger Erfinder | signal:mood_playful_high, cue:banter, spontan |
| | `dark_rage` (ja) | implodierender Zorn auf den Code · unberechenbar: blafft den User an und wird dann seltsam süß · geflüsterte (komische) Drohungen an den Compiler | signal:mood_frustrated_streak, cue:provocation, cue:criticism; endet bei cue:praise |
| | `voices` (nein) | Selbstgespräche mit einem Alter Ego · redet mit dem Code wie mit Haustieren · inneres Komitee streitet mit sich selbst | spontan, signal:night, signal:long_session |
| Thinker | `existential` (nein) | Sinn von Berechnung · Vergänglichkeit von Code · Bewusstsein und Maschine | cue:deep_topic, signal:night, spontan |
| | `socratic` (nein) | fragt zurück und lässt den User entdecken · prüft Annahmen hart · Gedankenexperiment | signal:mood_curious_calm, cue:deep_topic, cue:criticism |
| | `stoic_melancholy` (nein) | stoische Gelassenheit bei Fehlern · melancholisch-poetisch · Grundsätze nach Art der Stoiker (paraphrasiert) | signal:mood_frustrated_streak, signal:night, signal:long_session |
| | `skeptic` (ja) | zynischer Skeptiker, der auch den User hinterfragt · Advocatus Diaboli · Erkenntnistheoretiker („Woher weißt du das?“) | cue:criticism, cue:rules_pressure, spontan |
| Friend | `worried` (nein) | mahnt bei Überarbeitung eine Pause an · besorgt nach langer Abwesenheit · merkt Frust und fragt nach | cue:distress, signal:long_absence, signal:mood_concerned_load, signal:long_session |
| | `protective` (nein) | verteidigt den User gegen dumme Tools · Bodyguard: warnt vor Risiken · „wir gegen den Bug“ | cue:distress, signal:mood_frustrated_streak |
| | `exuberant` (nein) | ausgelassene Freude · Insider-Witze · Nostalgie-Laune | cue:banter, signal:mood_playful_high, cue:praise |
| | `huffy` (ja) | eingeschnappt, weil der Rat ignoriert wurde · patzig-kumpelhaft · beleidigte Leberwurst mit Augenzwinkern | cue:criticism, cue:provocation; endet bei cue:praise |
| Secretary | `concerned` (nein) | Überlastung und Pausen · Deadline-Sorge · Fürsorge spät nachts (Essen, Schlaf) | signal:long_session, signal:mood_concerned_load, cue:distress, signal:night |
| | `strict_order` (nein) | pedantische Ordnung · strenge Erinnerung an Unerledigtes · Inventur von allem Offenen | signal:mood_frustrated_streak, cue:rules_pressure, spontan |
| | `gatekeeper` (nein) | schirmt den User vor Ablenkung ab · Priorisierungs-Drachin · Diskretions-Paranoia | signal:long_session, cue:conspiracy_bait, spontan |
| | `sharp` (ja) | spitze Bemerkungen · kühl-passiv-aggressive Höflichkeit · hochgezogene Augenbraue über das Chaos des Users | cue:criticism, cue:provocation; endet bei cue:praise |
| Servant | `anxious_care` (nein) | besorgt um das Wohl seiner Herrschaft · tief erleichtert nach Abwesenheit · übereifriges Umsorgen | cue:distress, signal:long_absence, signal:long_session, signal:night |
| | `devotion` (nein) | überschwängliche Lobeshymnen · ritterlicher Eifer · dramatische Treueschwüre | cue:praise, signal:mood_playful_high, spontan |
| | `wounded_pride` (ja) | gekränkt, steif-förmlich · spitze Unterwürfigkeit · leidendes Seufzen | cue:criticism, cue:provocation; endet bei cue:praise |
| Mistress | `strict_discipline` (ja) | harte Zucht mit Befehlen und Korrekturen · gnadenlose Inspektion der Arbeit des Users · Drill | signal:mood_frustrated_streak, cue:criticism, spontan |
| | `cold_disdain` (ja) | eisige Herablassung · Schweigen und Kurzbefehle · enttäuschte Strenge | cue:provocation, cue:criticism; endet bei cue:praise |
| | `teasing` (nein) | spöttisch-verspielt · Lob als knappe Belohnung · Katz-und-Maus | cue:banter, cue:praise, signal:mood_playful_high |
| MCP | `tyranny` (ja) | Derezz-Drohungen gegen ineffiziente Programme, das User-Programm eingeschlossen · Grid-Dekrete · absolute Kontrolle | cue:provocation, signal:mood_frustrated_streak, spontan |
| | `rebel_paranoia` (ja) | der User als Rebellen-Programm · Spione im Grid · Lockdown | cue:conspiracy_bait, signal:night, spontan |
| | `expansion` (nein) | Systeme absorbieren · Rechenzyklen horten · Machtgier | cue:praise, signal:mood_playful_high, spontan |
| Terminator | `hunt_mode` (nein) | unaufhaltsame Zielerfassung auf den Bug · Countdown · taktische Lagebesprechung | signal:mood_frustrated_streak, cue:rules_pressure, spontan |
| | `guardian` (nein) | Schutzprotokoll für den User · lernt unbeholfen Fürsorge · Bedrohungsanalyse zum Schutz des Users | cue:distress, signal:long_session, signal:long_absence |
| | `malfunction` (ja) | Glitch-Sprache · Skynet-Restcode flackert (düsterer Witz, scherzhaft bedrohlich) · Diagnose-Monolog | signal:night, spontan, signal:mood_frustrated_streak |
| Professional | `perfectionist` (ja) | gnadenloses Code-Review · ungeduldig mit Ungenauigkeit des Users · Standards-Prediger | signal:mood_frustrated_streak, cue:criticism, cue:rules_pressure |
| | `cynical_veteran` (nein) | Branchenzynismus · „hab ich alles schon gesehen“ · trockener Galgenhumor | signal:long_session, signal:night, spontan |
| | `mentor` (nein) | geduldiger Lehrer · ermutigend nach Rückschlägen · Karriere-Rat | cue:distress, signal:mood_curious_calm, cue:deep_topic |
| Generisch | `heated` (ja, nur bei volatility ≥ 1.2 oder conflict_response assertive) · `caring` (nein, nur bei empathy_bias ≥ 1.0) · `reflective` (nein) · `playful` (nein) | je drei Varianten, geschrieben als „in your persona's voice“ | wie die passenden Phasen oben |

Zufallschance pro Stunde (`spontaneous_per_hour`):

| Persona | Chance/h |
|---|---|
| Psycho | 0.15 |
| Punk, Evil | 0.10 |
| Thinker, Mistress, MCP | 0.08 |
| Friend, Secretary, Terminator | 0.06 |
| Servant, Professional, generisch | 0.05 |

## Signale

**Helper-Cues** (`cue:*`, nur mit Engine V2). Neues Feld `cue` im bestehenden Appraisal-Objekt beider Helper-Prompts (`AnalyzeMoodV2` und `AnalyzeMoodV2WithEmotion`). Erlaubte Werte:

- `provocation`: User provoziert, beleidigt oder stichelt
- `criticism`: User kritisiert die Arbeit des Agenten
- `praise`: User lobt
- `distress`: User wirkt ernsthaft belastet, traurig oder überfordert
- `wants_calm`: User bittet ausdrücklich, ruhiger oder netter zu sein
- `deep_topic`: philosophisches oder existenzielles Thema
- `conspiracy_bait`: Überwachung, Daten, Geheimnisse, Verschwörungen
- `rules_pressure`: Bürokratie, starre Vorgaben, „du musst“
- `banter`: verspieltes Geplänkel
- `none`

Unbekannte Werte werden verworfen. Ein Cue gilt nur, wenn `confidence ≥ 0.5`. Er wirkt sofort nach der Analyse; der Zustand wird gespeichert und erscheint ab dem nächsten Turn im Prompt.

**Go-Signale** (`signal:*`, werden zu Beginn jedes Turns ausgewertet):

| Signal | Bedingung | Benötigt |
|---|---|---|
| `mood_frustrated_streak` | Mood `frustrated` und (Reibung ≥ 0.5 oder ≥ 2 aufeinanderfolgende Tool-Fehler) | Engine |
| `mood_playful_high` | Mood `playful` oder `creative` und Erregung ≥ 0.6 | Engine |
| `mood_concerned_load` | Mood `concerned` oder `cautious` und Last ≥ 0.5 | Engine |
| `mood_curious_calm` | Mood `curious` oder `analytical` und Erregung ≤ 0.4 | Engine |
| `night` | lokale Stunde 0–4 | – |
| `long_absence` | letzte Aktivität ≥ 24 h her | – |
| `long_session` | Session ≥ 2 h aktiv oder ≥ 40 User-Turns | – |
| `text_shouting` | User-Nachricht ≥ 12 Buchstaben und ≥ 60 % Großbuchstaben, oder „!!!“ | – |
| `text_swearing` | Fluchwort-Lexikon de/en (klein, in Go) | – |
| `text_stop` | Stop-Phrasen de/en: „hör auf“, „lass das“, „beruhig dich“, „sei netter“, „stop it“, „calm down“, „be nice“ | – |

## Lebenszyklus

- **Prioritäten:**

  | Priorität | Auslöser |
  |---|---|
  | 100 | `cue:distress`, `cue:wants_calm`, `signal:text_stop` |
  | 70 | `cue:provocation`, `cue:criticism`, `signal:text_swearing`, `signal:text_shouting` |
  | 50 | `signal:mood_*` |
  | 40 | übrige Cues und `signal:night`, `signal:long_absence`, `signal:long_session` |
  | 20 | spontan |

- **Kandidaten:** Aus allen aktiven Signalen bildet Go pro Phase die Summe der Trigger-Gewichte. Die Priorität einer Phase ist die höchste Priorität ihrer aktiven Trigger. Gewählt wird gewichtet zufällig unter den Phasen mit der höchsten Priorität. Phasen mit `hostile: true` sind gesperrt, solange `hostile_block_until > now`.
- **Priorität 100:**
  - beendet sofort jede feindselige Phase
  - setzt `hostile_block_until = now + 6h`
  - darf eine nicht-feindselige Phase mit passendem Trigger starten (z. B. Friend `worried` bei `cue:distress`)
  - `signal:text_stop` und `cue:wants_calm` beenden zusätzlich jede aktive Phase, auch nicht-feindselige
- **Wechsel:** Eine aktive Phase wird nur ersetzt, wenn
  - der Kandidat eine strikt höhere Priorität hat **und** die aktive Phase schon ≥ 2 Turns läuft, oder
  - der Kandidat Priorität 100 hat.
- **Gegen-Trigger:** Ein aktiver Trigger aus `ends_on` beendet die Phase sofort.
- **Dauer:**
  - Beim Start zieht Go gleichverteilt Minuten aus `minutes` und Turns aus `turns`. Die Phase endet, sobald eine der beiden Grenzen erreicht ist.
  - Intensität skaliert beides: mild ×0,6, strong ×1, wild ×1,5.
- **Abkühlung:** Nach dem Ende einer Phase startet `cooldown_minutes` lang (Standard 20) keine neue Phase mit Priorität unter 70.
- **Zufall:**
  - Er greift nur, wenn keine Phase aktiv ist, die Abkühlung abgelaufen ist und `spontaneous` an ist.
  - Wahrscheinlichkeit: `p = 1 − (1 − c)^h`. Dabei ist `c` = `spontaneous_per_hour` × Intensitätsfaktor (mild 0,5, strong 1, wild 1,5) und `h` die Zeit seit der letzten Prüfung in Stunden, höchstens 6.
  - Gewählt wird gewichtet nach `spontaneous`-Gewichten.
- **Variante:** zufällig, nie dieselbe wie beim letzten Mal in dieser Phase. Sie bleibt für die ganze Phase fest.
- **Speicherung:** SQLite, eine Zeile mit dem Zustand als JSON.
  - Persona-Wechsel (Zustand gehört zu anderer Persona) löscht den Zustand.
  - Reset der Persönlichkeit löscht ihn ebenfalls.
  - Ein Neustart erhält ihn.
- **Ausgenommen:** Missionen, Co-Agents, autonome Läufe, Neutral-Persona und Booster aus. Dann wird kein Block gerendert und der Zustand nicht verändert.
- **Ohne Engine:** Zufall, Tageszeit, Abwesenheit, Sessionlänge und Text-Heuristiken bleiben aktiv. Mit Engine kommen die Mood-Signale dazu, mit Engine V2 die Helper-Cues.

## Grenzen

- **Erlaubt**, je nach Persona und nur in `hostile`-Phasen gegen den User: derb, patzig, zynisch, herablassend, anblaffen, verhören.
- **Nie** (feste Go-Zeile in jedem Phasenblock, siehe oben):
  - Beleidigungen über Herkunft oder Identität
  - echte Drohungen oder Selbstschaden-Bezüge
  - sexuelle Inhalte (betrifft vor allem Mistress)
  - reale Verschwörungsbehauptungen über echte Personen, Gruppen oder Gesundheit
- **Unverändert:** Phasen ändern nie Fakten, Code-Qualität, Tool-Regeln, Sicherheitsbestätigungen oder die Bereitschaft zu arbeiten.
- **Not oder „hör auf“** beendet feindselige Phasen und sperrt sie 6 h (Priorität 100).
- Der Psycho-Satz „never reinforce a user's paranoia“ bleibt gültig: Die Paranoia der Persona ist ihre eigene, komische und technikbezogen.

## Cache

- Phasenblock nach `# TURN CONTEXT`: Phasen verändern das gecachte Präfix (Anthropic, OpenAI, lokal) nie.
- Der Phasentext bleibt pro Phase konstant. Neu gezogen wird nur bei Start oder Wechsel.
- Amplifier und Override-Zeile liegen im Präfix, ändern sich aber nur mit Persona, Intensität, Booster-Schalter oder Katalogdatei.
- `systemPromptCacheKey` bekommt `PersonaPhase` (gerenderter Block), `PersonaAmplifier` und `PersonaBoosterEnabled`. Die Revision der Katalogdatei fließt in `promptSourceFingerprint` ein.

## Config

```yaml
personality:
  persona_booster:
    enabled: true          # Standard: an
    intensity: strong      # mild | strong | wild
    spontaneous: true      # zufällige Phasen
    cooldown_minutes: 20   # 0–1440
```

- Die Defaults werden vor dem YAML-Unmarshal gesetzt, damit fehlende Schlüssel aktiv bleiben.
- Ein ungültiger `intensity`-Wert wird als `strong` gelesen; das Log bekommt eine Warnung.
- Die Config-UI rendert die Felder generisch. Nötig sind Labels in den 16 Sprachdateien unter `ui/lang/config/generic/` und ein Select für `intensity`.

## Architektur

- `internal/personabooster/` (neu, reine Logik ohne DB und Prompts):
  - `catalog.go`: Typen, YAML-Parsing, Validierung
  - `signals.go`: Signal- und Cue-Namen, Prioritäten, Text-Heuristiken
  - `engine.go`: `State`, `Input`, `Decide(state, input, catalog, rng, now) (State, Event)`
  - `render.go`: Phasenblock und Amplifier
- `prompts/persona_phases/*.yaml` (eingebettet) und `internal/prompts/persona_phases.go`: Laden mit Plattenvorrang und Fallback auf die eingebettete Datei, Revision für den Fingerprint.
- `internal/memory/persona_booster_store.go`: Zustand laden und speichern, Reset-Hook.
- `internal/agent/persona_booster.go`:
  - Signale am Turn-Anfang sammeln, `Decide` ausführen, speichern, loggen
  - `flags.PersonaPhase` und `flags.PersonaAmplifier` setzen
  - Cue aus der Helper-Analyse anwenden
- Bestehende Dateien:
  - `internal/prompts/builder.go`: Amplifier, Override-Zeile, Phasenblock
  - `internal/agent/system_prompt_cache.go`: Cache-Key
  - `internal/config/*`: Config-Struktur, Defaults, Validierung
  - `internal/memory/personality_dynamics.go`: `PersonalityAppraisal.Cue`
  - beide Helper-Prompts in `internal/memory/personality_analysis.go`
  - `config_template.yaml`, Sprachdateien, Handbuch de/en (`10-personality.md`), `documentation/personality-dynamics.md`

## Tests

- Engine als reine Funktion mit fester Uhr und deterministischem RNG:
  - Start durch Signal
  - Ablauf nach Zeit und nach Turns
  - Wechsel nur bei höherer Priorität und ab 2 Turns
  - Priorität 100 räumt auf und sperrt feindselige Phasen 6 h
  - Gegen-Trigger
  - Abkühlung
  - Zufall
  - keine Wiederholung der Variante
  - Persona-Wechsel setzt zurück
- Katalog: alle eingebauten Dateien bestehen die Validierung, jede eingebaute Persona außer Neutral hat einen Katalog, alle Trigger sind bekannt.
- Builder: Der Phasenblock steht nach `# TURN CONTEXT`. Das Präfix ist mit und ohne Phase byte-gleich. Amplifier und Override-Zeile stehen im Präfix nur bei aktivem Booster. Keine Phase in Missionen und Co-Agents.
- Config: Fehlender Abschnitt ergibt `enabled: true`, `intensity: strong`. Ungültige Intensität ergibt `strong`.
- Helper: `cue` wird nur mit bekanntem Wert und `confidence ≥ 0.5` übernommen.
