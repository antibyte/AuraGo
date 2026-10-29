# Game Maker Studio: Plan für mehr erfolgreiche Spiele, auch mit schwachen Modellen

**Stand:** 2026-10-26 · **Typ:** Analyse + Umsetzungsplan, keine Codeänderung
**Evidenz:** `reports/game-maker-forensics-2026-10-26/evidence.md` (40 echte Jobs von aurago-test,
Code-Ursachen mit Datei:Zeile) plus Rohdaten und Skripte im selben Ordner.
**Vorgänger:** `reports/game-maker-llm-audit-2026-10-22/llm-robustness-audit.md`,
`plans/2026-10-22-game-maker-llm-robustness.md` und `-handoff.md` (die Wellen 1–2 von dort sind gelandet).

## Umsetzungsstand

**Welle 1 (M1, P1–P8) gelandet auf `main` 476bad439 (2026-10-27), nach origin gepusht, auf aurago-test per update.sh
deployt (Build `ea5d6a138`, 2026-10-27 16:24).**

Erste Messung (2026-10-27, `reports/game-maker-forensics-2026-10-27/auswertung.md`):
- Nur 1 Job: agnes-3.0-flash, nativ.
- Planung im ersten Versuch angenommen.
- Bau an zwei 30-s-Stream-Stalls gescheitert, vermutlich weil das Gateway große Tool-Argumente nicht fortlaufend
  liefert.
- P7 greift auf aurago-test nie, weil Studio-Läufe dort `code_graph` im Scope haben. Das ist zu reparieren.
- Welle 1 ist noch ungemessen, GLM/MiniMax liefen nicht.

Fixes aus dieser Messung auf `main` `a2f1d5981`, nach origin gepusht mit `8c95a6556` (2026-10-27), noch nicht deployt:
- `a64f57029`: P7 erkennt Studio-Läufe auch mit `code_graph` im Scope.
- `7354a0cc8`: Ein Studio-Stream darf nach dem ersten Fortschritt 120 s still sein statt 30 s; die Job-Kopie der
  Konfiguration hebt `circuit_breaker.llm_stream_chunk_timeout_seconds` an, das Warten auf das erste Token bleibt
  unverändert.
- `a2f1d5981`: Die Doku sagt, welche Streams einen Stall einmal wiederholen.
- Offen: Der Bau des Starter-Quelltexts wiederholt einen Idle-Stall nicht
  (`game_maker_implementation_stream.go:94-121`).
Commits: P4 `10cd98793` + Review-Fix `3f46d68f8` · P1 `b264538f0` + Härtung `874538acc` · P2 `cd728a65d` ·
M1 `12a195d03` · P3 `4e1076a8b` + Review-Fix `67a5ddd38` · P5 `392090513` + Review-Fix `476bad439` ·
P6 `0d99c83f8` · P7 `11835b8f2` · P8 `9fe86e03b`.
- Replay: das echte GLM-Design wird jetzt angenommen; das MiniMax-Design erreicht die Validierung und
  scheitert dort korrigierbar (Pack-Namen statt Katalog-Asset-IDs) statt an `tool_scope_denied`.
- P7 erkennt Studio-Läufe am Scope (nur `game_maker_*`-Tools), damit `agent_loop.go` unberührt bleibt.
- Tests: build, llm, gamemaker, server- und ui-Teilmengen grün; in `internal/agent` schlagen drei Tests
  fehl, die auf `main` identisch fehlschlagen (ExecManage-Pfade, ExecutionOutputIsolation,
  ToolSearchTerms/`qr_code`).
- Bekannte Restpunkte:
  - Im Textmodus wird nur der erste Aufruf eines JSON-Arrays ausgeführt (MiniMax-Batch `[list_files, get_plan]`).
  - `tool_start`-Events zeigen den Rohnamen vor der Normalisierung.
  - Der Duplikat-Breaker sieht den Rohnamen.
- Nächster Schritt laut §8: P9 (Textmodus-Profil) und die beaufsichtigte Baseline M3.

## 1. Kurzfazit

- **Erfolgsquote heute:** Nur 1 von 11 Projekten wurde im ersten Job fertig. 8 Projekte kamen erst
  nach „Erneut versuchen“-Ketten ans Ziel (im Schnitt ~3,6 Jobs pro Projekt), 3 nie. Mindestens 2
  „ready“-Spiele hatten danach offensichtliche Fehler: Das Schiff schoss immer nach oben, links und
  rechts waren vertauscht.
- **Größter Einzelbefund:**
  - MiniMax M3 und GLM-5.3-Flash scheitern schon beim Planen, obwohl sie **vollständige, gültige
    Designs** liefern.
  - Ursache 1: Eine Namensregel zwingt beide in den Text-Tool-Modus (`tooling_policy.go:128-135`).
  - Ursache 2: Im Textmodus werden **keine Tool-Schemas** mitgeschickt (`agent_loop_init.go:513-547`),
    und das Game-Maker-Profil nennt die Toolnamen nicht.
  - Folge: Die Modelle rufen `{"tool":"set_design",…}` auf und bekommen nur „tool outside this run's
    scope“, ohne Hinweis auf den richtigen Aufruf. Danach geben sie auf.
- **Zweitgrößter Befund:** Die Validierung braucht ein **sichtbares Studio-Fenster beim Nutzer**
  (etwa 9 s nach dem Build). Ist keins offen, schlägt der Job fehl – ohne Wartezustand. Deshalb lässt
  sich auch der Benchmark nicht unbeaufsichtigt fahren.
- **Was gemessen werden muss:** Die Härtung vom 22.–24.10. (Dry-Run, Sampling, adapt-Modus,
  Kompaktprofil, Lint, Modell-Gate) ist **nie gemessen** worden. Seitdem liefen nur 4 Jobs: drei
  scheiterten an 402 (Zahlung), einer am Textmodus. Ein Teil der September-Fehler ist im aktuellen
  Code schon behoben: Mehrfach-Edits in `replace_many` seit 7c595d3a6, `read` ohne `operation` bei
  Zeilenbereich (`agent_dispatch_game_maker.go:545-554`).
- **Wo der Hebel liegt:** nicht in der Validierung. Er liegt bei Protokoll, Infrastruktur und Rückmeldung.

## 2. Ursachen nach Wirkung × Kosten

| # | Ursache | Fehljobs | Wirkung | Kosten | Schon behoben? |
|---|---|---|---|---|---|
| 1 | Textmodus ohne Tool-Schemas, keine Alias-Auflösung, Ablehnung ohne Ausweg | 3 | Zwei Modellfamilien: 0 % Planung | S + M | Nein |
| 2 | Validierung braucht sichtbares Studio | 4 (+1) | Alle Modelle; kein unbeaufsichtigter Benchmark | M / L | Nein |
| 3 | Gameplay-Check ohne Hinweis zur Erreichbarkeit repariert | 5 | Reparatur rät, Budget verbrannt | S–M | Teilweise (Budget, „+3“, `open_checks`) → messen |
| 4 | 402 als wiederholbar eingestuft (`*openai.RequestError` wird nicht klassifiziert) | 3 | 10 Retries pro Aufruf, unklare Meldung | S | Nein |
| 5 | Tool-freie Neuschrift von `src/main.ts`, kein Fallback | 4 | Schwerste Einzelaufgabe; großer Kontext ≠ fähiges Modell | M | 2 von 4 aus altem Build → messen |
| 6 | 3D-TypeError `reading 'released'` (Asset-Handle undefiniert) | 2 (16 Validate-Fehler) | Unverständliche Meldung aus minifiziertem Code | S | Nein |
| 7 | Voxel-Palettenpflicht auch für kreative Bauprojekte | 1 | „Burg bauen“ dreimal abgelehnt | S–M | Nein |
| 8 | Ready-Checks prüfen nur, ob Zähler steigen | ≥ 2 falsche Erfolge | Nutzer findet Bugs erst nach „ready“ | M | Nein |

Erst messen statt neu bauen: Dry-Run und Planbudget (4d8b502f7), Phasentemperatur, Repair-Budget
(1e0e858f0), StepFun-400, Stream-Abbrüche und leere Antworten.

## 3. Leitprinzip: großzügig annehmen, streng veröffentlichen

- **R1 Eindeutiges annehmen.** Gibt es genau eine sinnvolle Deutung, normalisiert der Server den
  Aufruf. Beispiele: Operation als Toolname, fehlende Operation, Operation am falschen Tool, genau ein
  Codeblock in Prosa. Das Ergebnis meldet die kanonische Form in `normalized_call`, damit das Modell
  dazulernt.
- **R2 Scope nie erweitern.** Aliase zielen nur auf Tools innerhalb der `AllowedTools` der Phase;
  sonst bleibt die heutige Ablehnung. Für den Hauptagenten gibt es keine Aliase.
- **R3 Nichts erfinden, was Spielverhalten vortäuscht.** Automatisch ergänzt wird nur, was die Engine
  braucht, immer mit Warnung. Nie Zähler, Checks oder Evidenz.
- **R4 Hinweise nur in Tool-Ergebnissen oder User-Nachrichten.** Nie dynamisch im System-Prompt, sonst
  bricht das Prompt-Caching.
- **R5 Veröffentlichung bleibt.** `validateForPublication` und Fingerprint bleiben unverändert;
  `passed` entsteht nur aus beobachteter Browser-Evidenz. Neue Proben sind zunächst nur Hinweise.
  Ungeprüftes wird angezeigt statt verschwiegen.

**Kreativität:** Der Agent soll nicht enger geführt, sondern besser informiert werden. Regeln, die
legitime Spielideen ablehnen, werden zu Ergänzungen mit Warnung (Welle 3). Freier Code, freie Bases
und eigene Szenarien bleiben erhalten.

**Pflicht pro Paket:**
- GitNexus-Impact vor jeder Änderung an zentralen Symbolen. Bei `canonicalizeLeafCall`,
  `resolveModelCapabilities`, `ClassifyError` und `checkDispatchActionPolicy` ist HIGH/CRITICAL zu
  erwarten; das muss benannt werden.
- `detect_changes` vor jedem Commit.
- DOX-Pass: `internal/gamemaker/AGENTS.md`, `internal/agent/AGENTS.md`,
  `documentation/game-maker-studio.md`, bei UI auch `ui/AGENTS.md`.
- Neue UI-Texte in allen 16 Sprachen.

## 4. Arbeitspakete

### Welle 0 – Messgrundlage (parallel zu Welle 1)

**M1 Replay-Suite aus echten Verläufen (S–M, ohne LLM, im CI)**
- Die Assistenten-Antworten der gespeicherten Kontexte (`gm_context.txt`: MiniMax, GLM,
  DeepSeek-Voxel, step-5-preview) werden als vorgegebene Antworten durch `ExecuteAgentLoop`
  geschickt. Muster: `internal/gamemaker/game_maker_planning_loop_test.go`.
- Pro Verlauf ist die erwartete Reaktion festgelegt (angenommen, normalisiert oder abgelehnt mit Code).
- Vorher bereinigen: nur Designs und Code übernehmen.
- Abnahme: Der Test ist heute rot für die Textmodus-Verläufe und wird nach Welle 1 grün.

**M2 Telemetrie pro Job (S)**
- Additiv und begrenzt: `text_mode`, Zähler `protocol_normalizations`, Tool-Fehlercodes,
  Provider-Retries, Position des ersten Schreibaufrufs, Wechsel des `building_mode`.
- Sichtbar in `job_status`.

**M3 Beaufsichtigte Baseline (kein Code)**
- `scripts/game-maker-benchmark.mjs` mit offenem Studio auf einer lokalen Scratch-Instanz.
- Modellsatz siehe §5.
- Prüft vor allem die Klassen 3 und 5 und die vermutlich behobenen Punkte.

### Welle 1 – größter Hebel, geringstes Risiko

**P1 Operationsnamen als Toolnamen annehmen (S)**
- Neue Funktion `canonicalizeGameMakerOperationCall` in `internal/agent/canonical_action.go`,
  aufgerufen in `canonicalizeLeafCall` *vor* dem discover-Alias (sonst kollidiert `search`).
- Die Operationsmengen der vier Tools sind disjunkt (`native_tools_game_maker.go`), die Zuordnung ist
  also eindeutig:
  - `set_design`, `get_plan`, `inspect`, `list_files`, … → `game_maker_project`
  - `read`, `search`, `write`, `replace`, `replace_many` → `game_maker_file`
  - `search_assets`, `describe_asset` → `game_maker_asset`
  - `validate` → `game_maker_validate`
- `game_maker_file` mit `list_files` wird zu `game_maker_project.list_files`.
- Greift nur bei gebundenem Job (`gamemaker.JobIDFromContext`) und nur innerhalb der `AllowedTools`.
- Abnahme:
  - Die wörtlichen Formen `<tool_call>[{"tool":"set_design","args":{"design":…}}]` und
    `{"tool":"set_design","arguments":{…}}` führen `SetDesignJSON` aus, im Text- und im nativen Pfad.
  - Ein Alias auf ein nicht erlaubtes Tool bleibt `tool_scope_denied`.
  - Ohne Job wird nichts umgeschrieben.

**P2 Ablehnung mit Ausweg (S)**
- `action_policy.go:267-268` und `tool_scope.go:68-71` geben bei eingeschränktem Scope zusätzlich
  `allowed_tools` (pro Phase stabil) und ein einzeiliges Aufrufbeispiel zurück.
- Abnahme: Das Ablehnungs-JSON enthält beides; Läufe ohne Scope bleiben byte-gleich.

**P3 Namensregel normalisieren (S)**
- In `tooling_policy.go:128-135` Leerzeichen und Unterstriche vor der Prüfung in `-` wandeln.
- Hintergrund: „MiniMax M3“ verfehlt heute die M3-Ausnahme.
- Abnahme: Tabellentest – „MiniMax M3“, „minimax_m3“ → nativ; „abab6.5“, „glm-4“ → Text.
- Wie mit GLM-5.x umzugehen ist, entscheidet E2.

**P4 402/401/403 aus `RequestError` nicht wiederholen (S)**
- `internal/llm/errors.go:79-82` wertet auch `*openai.RequestError` (HTTP-Status) über
  `classifyHTTPError` aus. Muster: `task_route_client.go:173-175`. 402 → `QuotaExceeded` mit klarem
  Text.
- In der Studio-Fehlerkarte erscheint dazu ein Hinweis auf „Mit anderem Modell“ (der Button existiert
  seit 3526e3a44).
- Abnahme: Ein `RequestError` mit 402 und dem Body `{"error":"No active plan…"}` führt zu genau
  1 Aufruf; 5xx bleibt wiederholbar.

**P5 Ehrliche Planungsfehler (S)**
- Lief nie ein `set_design`, setzt `internal/gamemaker/service.go:716-717` den Code
  `planning_protocol` („Modell hat keinen gültigen Tool-Aufruf abgegeben“) statt „charged correction
  budget was spent“. Die UI-Karte erklärt das.
- Abnahme: Lauf ohne Toolaufruf → `planning_protocol`; drei abgelehnte Designs → bisheriger Text.

**P6 Verständliche Diagnosen (S)**
- `reading 'released'` wird im Muster `boundedPreviewDiagnostics` (`validation.go:38-70`) übersetzt:
  „Asset-Handle ist undefiniert – `createInstance` nur mit dem Ergebnis von `await loadAsset` und
  einer geplanten Rolle aufrufen“.
- Ein Read auf eine fehlende Datei listet die vorhandenen `src/`-Dateien. Hintergrund: 3D-Läufe
  suchten `src/common.ts`.
- Bei increased/decreased/changed druckt `gameplay.go:328` keinen Erwartungswert mehr; die „0“ wirkt
  heute wie eine Schwelle.
- Reason-Codes in `gameplay.go:460` bekommen einen Satz Klartext.
- Abnahme: Unit-Tests auf die Texte; die Check-Status bleiben unverändert.

**P7 Passende Korrektur-Hinweise für Studio-Läufe (S–M)**
- `agent_loop_retry.go:94-107, :253-328` und `recovery_session.go:227` nennen `<done/>`,
  `discover_tools` und `file_editor`. Das gibt es in Studio-Läufen nicht.
- Bei `PreparedPrompt != nil` kommen eigene, phasenbezogene Texte (welche Tools, welcher nächste
  Schritt).
- Abnahme: Ein Textlauf mit Ankündigung ohne Tool erhält einen Hinweis ohne diese Begriffe.

**P8 Versteckter Tab ist kein Spielfehler (S)**
- `preview-tests.js:196` meldet `host_hidden`; `gameplay.go:446` behandelt das als nicht
  reparierbar. So verbraucht ein versteckter Tab kein Reparaturbudget mehr.
- Abnahme: Test von `compareGameObservations` → `Repairable=false`.

**P9 Eigenes Profil für den Textmodus (M)**
- Neue Variante `game-maker/v4/<phase>/<dim>/text`; v2 und v3 bleiben unverändert.
- Inhalt: statisch aus den Phasen-Schemas gerenderte Tool- und Operations-Signaturen plus ein
  Aufrufbeispiel im Format, das der Parser akzeptiert.
- Auswahl über `agent.BuildToolingPolicy(&cfg,"")` in `game_maker_service.go:640-651`.
- Abnahme:
  - Jeder Tool- und Operationsname der Phase steht im Profil.
  - Fingerprints sind gepinnt; die nativen Profile bleiben byte-identisch.
  - Der M1-Replay der GLM/MiniMax-Verläufe endet mit angenommenem Plan.

### Welle 2 – Infrastruktur und Bauphase

**P11 Warten statt Fehlschlag ohne Vorschau (M)**
- Bei `unavailable` ohne Host (`service.go:801-831`, `validation.go:148-185`) geht der Job in den
  Zustand `waiting_preview` (begrenzt, etwa 15 Minuten).
- Der Nutzer wird benachrichtigt (Studio-Toast plus vorhandene Notification).
- Meldet ein Host „ready“, läuft der Job automatisch weiter; erst nach Ablauf kommt der heutige
  Fehler `preview`.
- Ein offenes `preview_reload` wird beim SSE-Nachladen nachgeholt (`game-maker-studio.js:587-601`).
- Abnahme: Go-Test für Warten → Fortsetzen → Timeout; UI-Test für das nachgeholte Reload.

**H1 Headless-Validierung auf dem Server (L, hängt an E1)**
- Ist kein Studio offen, startet der Server ein Chrome über rod:
  - ein Slot, Timeout, Egress nur zur eigenen Origin über `internal/security/capture_proxy.go`;
  - Muster: `internal/tools/homepage_annotation_capture.go:92-200`.
- Es lädt eine Host-Seite auf derselben Origin (CSP `frame-ancestors 'self'`,
  `game_maker_handlers.go:527`), mit einem einmaligen Token, das an die `validation_id` gebunden ist.
- Die Host-Seite bindet das Sandbox-iframe per `CreatePreviewGrant` ein und meldet über
  `ReportPreview`. Vorbild ist der Test-Harness `guided_browser_test.go:113-153`.
- Ein offenes Studio hat Vorrang. Config-Schalter `game_maker.headless_validation: auto|off`.
- Vertrag: Es bleibt echte Browser-Beobachtung, `passed` bleibt also gültig. Security-Review ist Pflicht.
- Risiko: WebGL ohne GPU (SwiftShader) kann das 55-s-Budget der 3D-Checks sprengen. Deshalb vorher
  einen Messlauf.
- Effekt: Damit läuft der Benchmark unbeaufsichtigt.

**P12 Fallback statt Abbruch bei der tool-freien Generierung (M)**
- Scheitert auch der eine Retry (`implementation_stream.go:119-148`), wechselt der Job:
  - auf compact + adapt bei Templates mit Ankern,
  - sonst auf den normalen Tool-Loop (minimal, three, voxel, guided 3D).
- Heute endet der Job dort mit `model` (`service.go:788-790`). Gleiches gilt für den Wiedereinstieg
  in die Reparatur (`game_maker_service.go:637`).
- Genau ein ```ts-Block in Prosa wird akzeptiert (`game_maker_implementation.go:70-77`).
- Der Moduswechsel ist ein Event mit Badge, kein Modellwechsel.
- Abnahme: Ein Fake-Client liefert zweimal Tool-Markup → der Job läuft weiter; veröffentlicht wird
  nur nach Validierung.

**P13 Herumsuchen in der Bauphase bremsen (S–M, nur Hinweise)**
- Kein Suchtreffer bei Regex-Syntax → „Suche ist wörtlich“ (`source_search.go:32-103`).
- Datei liegt schon in `current_sources` → „bereits im Kontext“.
- Nach 8 Lese- oder Suchaufrufen ohne Schreiben schlägt das Tool-Ergebnis `replace` mit den passenden
  Ankern vor.
- `read`-Ergebnisse von `src/main.ts` enthalten eine kurze Gliederung (Funktionen/Klassen mit Zeilen),
  damit schwache Modelle gezielt springen statt zu suchen.
- `content:""` gilt nicht mehr als `write`; ein reiner Pfad wird zu `read`.
- Das Tool-Budget bleibt unverändert.
- Abnahme: Dispatch-Tests je Fall.

**P14 Kurzreferenz nach Wichtigkeit statt abschneiden (S)**
- `CompactRuntimeReference` (`runtime_reference.go:101-127`) schneidet heute hinten ab; dabei fallen
  gerade die Helper- und Checks-Zeilen weg. Stattdessen eine Prioritätsliste der Zeilen.
- `pickup_events` wird in die 2D-Zählerliste aufgenommen (`:61`).
- Den Widerspruch zu `internal/gamemaker/AGENTS.md:72-73` („feedback changes no counters“, obwohl
  `feedback('pickup')` zählt) auflösen.
- Abnahme: Die kompakte Phaser-Referenz bleibt ≤ 2000 B und enthält `feedback('pickup'` und `Checks:`.

**P15 Erreichbarkeits-Hinweise in der Reparatur (M)**
- Aus inputs/contacts/effects/reason entstehen in `gameplay.go:430-470` und `validation_plan.go:158`
  feste Ratschläge.
- Beispiel: inputs ≤ 1 und contacts = 0 → „Ziel ist für den Testtreiber unerreichbar“. Die Wegsuche
  sieht nur Hindernisse, die mit dem Spieler verbunden sind (`preview-tests.js:146-179`), und springt
  nur vom Boden aus (`:322`).
- Abnahme: Tabellentest; die Status bleiben unverändert.

### Welle 3 – Kreativität und ehrlicher Erfolg

**C1 Voxel-Palette ergänzen statt ablehnen (S–M)**
- Fehlende Terrain-Materialien (`voxel.go:168-176, :308`) ergänzt der Server mit freien IDs und
  Drop-Items aus `design_example`, mit Warnung „Name, Farbe und Drop sind frei“. Die Engine braucht
  die Materialien selbst (`runtime/voxel-world.js:44-59`).
- Null Rezepte im Kreativmodus nur, wenn die Runtime das verträgt (vorher prüfen).
- Abnahme: Ein Burg-Design ohne ore/sand wird angenommen; die Weltgenerierung läuft im Browser-Test.

**C2 Regel-Check aus der Spielidee ableiten (M)**
- platformer/topdown/minimal ohne Item-Ziel bleiben heute `unavailable` (`gameplay.go:234-239`).
- Stattdessen ordnet der Server aus vorhandenen Designdaten (`mechanics.outcomes`, reach/survive/collect)
  ein passendes `required_rules`-Szenario zu.
- Das Minimum wird ersetzt, nicht entfernt; dafür braucht es eine neue validation-plan-Version.
- Abnahme: Ein Platformer, in dem man nur das Ziel erreichen muss, wird veröffentlicht.

**S1 Ungeprüftes auch bei „ready“ anzeigen (S–M)**
- `unverified_checks` analog zu `open_checks` (c3615cb4a). Die Ready-Karte zeigt `natural_outcomes`
  und `role_*` als „nicht geprüft“.

**S2 Richtungsproben als Hinweis (M–L)**
- Zwei Proben: „RIGHT bewegt nach bildschirmrechts“ (die Metrik `player_right` existiert,
  `gameplay.go:314`) und „Projektil folgt der Blickrichtung“.
- Nur Hinweis, nie Gate; das Modell erhält sie als `advisories`.
- Beide beobachteten Falsch-Erfolge wären damit aufgefallen.

**S3 Schnelles Spieltest-Feedback nach „ready“ (S–M, UI)**
- Chips wie „Steuerung vertauscht“, „Springen geht nicht“, „Schuss in falsche Richtung“,
  „Ziel unerreichbar“.
- Ein Klick erzeugt einen Edit-Job mit strukturiertem Befund und dem passenden Check als Ziel.
- Damit wird der heutige Freitext-Retry zu einer gezielten Reparatur.

**P16 Shooter-Primärtaste (S)**
- `shooter_scenarios.go:19-21` passt nur SPACE an. Die im Plan deklarierte Primärtaste ebenso behandeln.
- Wo der Plan diese Taste führt, ist noch zu klären.

**P10 Hybrid-Parsing (M)** – nur nach Entscheidung E3.

## 5. Messplan

**Ausgangswerte** (aurago-test, 16.09.–23.10.):

| Metrik | Wert |
|---|---|
| Beim ersten Job fertig | 1/11 Projekte (9 %) |
| Fertige Jobs | 11/40 (27,5 %) |
| Jobs pro fertigem Projekt | ~3,6 |
| Falsche Erfolge | ≥ 2 von 11 |
| Planung im Textmodus | 0/3 |
| Retries bei 402 | 10 pro Aufruf |
| An der Vorschau gescheitert | 4 (+1) Jobs |
| Aufwand pro Job | 30–95 Aufrufe, 0,6–4,5 M Input-Token |

**Ziele** (8 Benchmark-Fälle × fester Modellsatz):

| Metrik | Ziel |
|---|---|
| Beim ersten Job fertig | ≥ 4 von 8 Fällen pro Modell |
| Jobs pro fertigem Projekt | ≤ 1,5 |
| Replay der aufgezeichneten Formen | 100 % angenommen (live ≥ 2 von 3) |
| Aufrufe bei 402 | genau 1 |
| Vorschau-Fehler, unbeaufsichtigt | 0 |
| Falsche Erfolge (manuell bewertet) | ≤ 1 von 8 |
| Aufrufe pro fertigem Job | Median ≤ 40 |

**Modellsatz:**
- ein Modell im Textmodus (GLM-5.3-Flash oder MiniMax M3),
- ein schwaches Modell mit großem Kontext (DeepSeek-V4-Flash),
- ein Modell unter 64k Kontext,
- ein starkes Referenzmodell als Obergrenze.

Im Report stehen jeweils Commit, Temperaturen und Profilnamen.

**Vorgehen:**
1. M1 im CI für die Protokoll-Klassen.
2. Benchmark zunächst beaufsichtigt (M3), ab H1 unbeaufsichtigt. Ein neues Flag `--expect-headless`
   bricht ab, wenn ein Job in `waiting_preview` hängt.
3. Die Skripte `gm_overview.py` und `gm_tool_errors.py` regelmäßig read-only gegen aurago-test laufen
   lassen, gruppiert nach deploytem Commit.
4. „Spielbar“ bewerten Menschen anhand von 3 Fragen: Stimmt die Steuerung? Ist das Ziel erreichbar?
   Gibt es einen offensichtlichen Bug?

Nach jeder Welle wird neu gemessen. Qualitätsaussagen gibt es nur mit diesen Zahlen.

## 6. Offene Entscheidungen (Empfehlung zuerst)

**E1 – Chrome/Chromium auf aurago-test für die Headless-Validierung (H1)?**
- *Empfehlung:* ja, Modus `auto`.
- *Abwägung:* unbeaufsichtigte Validierung und Benchmark gegen ~300–500 MB, einen weiteren Prozess
  als Angriffsfläche (Sandbox und Proxy) und mögliche 3D-Timeouts ohne GPU.
- *Ohne Chrome:* nur P11.

**E2 – GLM/MiniMax-Namensregel?**
- *Empfehlung:* P3 global umsetzen, dazu eine Provider-Einstellung `tool_call_mode: auto|native|text`.
  Standard `auto` (= heutiges Verhalten).
- Für synthetic und oference nach einem Probelauf bewusst auf `native` stellen.
- *Abwägung:* explizite Wahl statt Automatik. Meldet ein Provider fälschlich Tool-Calling, kommen
  keine Toolaufrufe an.

**E3 – Hybrid-Parsing (Text-Toolaufrufe auch im nativen Modus)?**
- *Empfehlung:* ja, aber nur für Game-Maker-Namen und -Aliase und nur, wenn die Antwort keine
  nativen `tool_calls` enthält.
- *Abwägung:* Risiko, dass zitierte Beispiele ausgeführt werden.
- Vorher prüfen, was der native Pfad schon parst (`agent_loop_retry.go:71-119`).

**E4 – Automatischer Fallback auf compact/adapt (P12)?**
- *Empfehlung:* ja, mit Event und Badge. Das ist ein Moduswechsel, kein Modellwechsel.
- *Abwägung:* kleineres Quellfenster (12 KB).

**E5 – Kompaktmodus als Schalter im Startdialog?**
- Die API nimmt `small_model` schon an, die UI sendet es nicht.
- *Empfehlung:* expliziter Schalter, Standard aus.
- *Abwägung:* Im kompakten Design wählt das Modell keine Assets und keine Präsentation. Das kostet
  Kreativität, deshalb nur auf Wunsch.

**E6 – Richtungsproben (S2) später als Gate?**
- *Empfehlung:* erst bei < 5 % Fehlalarmen über ≥ 20 fertige Jobs, dann pro Template und mit neuer
  Plan-Version.
- Panzer- oder Drehsteuerung ist legitim.

**E7 – Reines Design-JSON ohne Tool-Hülle in der Planung als `set_design` werten?**
- *Empfehlung:* zunächst nein. Erst wenn der Replay nach P1, P2 und P9 noch solche Antworten zeigt.
- Dann nur als JSON-Objekt mit `base` und `objective`, über dieselbe Validierung. Nie Markdown-Prosa.

## 7. Was ausdrücklich nicht getan wird

- Keine Gates aufweichen:
  - `unavailable` wird nie `passed`;
  - 2D wird nicht auf Startup-Checks reduziert;
  - Prosa wird nicht als Plan oder Erfolg übernommen.
- Profiltexte v2/v3 nicht direkt ändern. Keine dynamischen Tool-Listen, Budget- oder Suchhinweise im
  System-Prompt.
- Kein automatischer Modell- oder Providerwechsel und keine „Eskalation zum stärkeren Modell“.
- Keine Aliase für den Hauptagenten und keine außerhalb der `AllowedTools`.
- Vendor-Dateien nicht freigeben, `maxRetryAttempts` nicht erhöhen.
- Tool-Budget nicht kürzen und frühes Schreiben nicht erzwingen; das bremst fähige Modelle.
- Material-Identitäten im Voxel nicht abschaffen, sondern ergänzen.
- Den Tool-Split (WP-K aus dem Handoff) erst angehen, wenn die M2-Telemetrie in mindestens zwei Fällen
  Fehler bei der Operationswahl zeigt.

## 8. Reihenfolge und Aufwand

| Schritt | Pakete | Aufwand | Voraussetzung |
|---|---|---|---|
| 1 | M1, M2, P1–P8 | ~3–4 Tage | – |
| 2 | P9, M3 (beaufsichtigte Baseline) | ~2 Tage | Schritt 1; Scratch-Instanz mit offenem Studio |
| 3 | P11, P12–P15 | ~5–7 Tage | Baseline-Zahlen |
| 4 | H1 | ~4–6 Tage | E1 |
| 5 | C1, C2, S1–S3, P16 (P10 nach E3) | ~5–7 Tage | Messung nach Schritt 3/4 |

P1–P8 sind unabhängig voneinander und lassen sich parallel bauen. Beim gemeinsamen Datei-Paar
`canonical_action.go` / `action_policy.go` muss die Reihenfolge P1 vor P2 sein.

## 9. Unsicherheiten

- Ob GLM-5.3 bei synthetic im nativen Modus echte `tool_calls` liefert. Das klärt ein Probelauf vor E2.
- Ob der native Pfad Text-Toolaufrufe heute schon teilweise ausführt (E3).
- Ob die Voxel-Runtime ohne Rezepte läuft (C1).
- Ob `preview-tests.js:199` (`view.active`) den Zustand des Spiels oder des Hosts meint (P8).
- Wo der Plan die Shooter-Primärtaste führt (P16).
- Ob der Falsch-Erfolg beim 3D-Runner auf einer guided- oder einer free-three-Basis lief. Free three
  veröffentlicht nur mit Startup-Check (`service.go:797-800`).
- Ob der im Audit geplante Screenshot im Reparatur-Paket (F7, `advisory_screenshot`) gelandet ist.
- Das deployte Binary auf aurago-test (`vcs.revision=8aa5981b0`, 26.10., `modified=true`) existiert im
  lokalen Repo nicht. Vor der Messung klären, welcher Stand dort läuft.
