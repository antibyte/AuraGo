# Plan: Manual-Audit gegen die Codebasis

## Ziel

Alle Inhalte aus [`documentation/manuals/README.md`](../documentation/manuals/README.md) sowie sämtlichen Kapiteln unter [`documentation/manual/`](../documentation/manual/) systematisch gegen die aktuelle Codebasis prüfen, falsche oder veraltete Aussagen korrigieren und fehlende Inhalte ergänzen.

## Scope

- Einstiegspfad: [`documentation/manuals/README.md`](../documentation/manuals/README.md)
- Sammelübersicht: [`documentation/manual/README.md`](../documentation/manual/README.md)
- Deutsche Manuals: [`documentation/manual/de/`](../documentation/manual/de/)
- Englische Manuals: [`documentation/manual/en/`](../documentation/manual/en/)

Nicht im Scope sind andere Doku-Dateien außerhalb dieses Pfads, außer sie müssen als Linkziel angepasst werden, damit die Manuals keine falschen Verweise enthalten.

## Wahrheitsquellen

- Einstieg und Laufzeitverhalten: [`cmd/aurago/main.go`](../cmd/aurago/main.go)
- Plattformverhalten: [`cmd/aurago/platform_windows.go`](../cmd/aurago/platform_windows.go), [`cmd/aurago/platform_unix.go`](../cmd/aurago/platform_unix.go)
- Konfiguration und Defaults: [`internal/config/`](../internal/config/), [`config_template.yaml`](../config_template.yaml)
- Chat-Commands: [`internal/commands/`](../internal/commands/)
- Tools: [`internal/tools/`](../internal/tools/)
- Memory, Personality, Agent-Logik: [`internal/memory/`](../internal/memory/), [`internal/agent/`](../internal/agent/), [`internal/prompts/`](../internal/prompts/)
- Web-UI und UX-Flows: [`ui/`](../ui/), [`internal/server/`](../internal/server/)
- Mission, Planner, Skills, Invasion: [`internal/planner/`](../internal/planner/), [`internal/invasion/`](../internal/invasion/), [`agent_workspace/`](../agent_workspace/)
- Integrationen: passende Pakete unter [`internal/`](../internal/)

## Arbeitsprinzipien

1. Aussagen nur dann im Manual belassen, wenn sie durch Code, Konfiguration oder tatsächlich vorhandene UI/API-Flows belegbar sind.
2. Marketing- oder Versionsbehauptungen entfernen oder abschwächen, wenn sie nicht klar verifizierbar sind.
3. Deutsche Inhalte zuerst korrigieren, danach Englisch inhaltlich synchronisieren.
4. Kapitelgrenzen sauber halten: keine widersprüchlichen Dopplungen zwischen Kapiteln.
5. Links, Dateinamen und Referenzen immer auf tatsächlich vorhandene Dateien oder Endpunkte abgleichen.

## Audit-Matrix nach Kapitelgruppen

### 1. Einstieg und Navigation

- Prüfen: [`documentation/manuals/README.md`](../documentation/manuals/README.md), [`documentation/manual/README.md`](../documentation/manual/README.md), [`documentation/manual/de/README.md`](../documentation/manual/de/README.md), [`documentation/manual/en/README.md`](../documentation/manual/en/README.md)
- Gegenprüfen mit: Kapitelstruktur unter [`documentation/manual/de/`](../documentation/manual/de/) und [`documentation/manual/en/`](../documentation/manual/en/), zentrale Produktbeschreibung in [`cmd/aurago/main.go`](../cmd/aurago/main.go) und UI-Einstiegen unter [`ui/`](../ui/)
- Ziel: korrekte Navigation, keine toten Links, keine unbelegten Feature-Highlights

### 2. Grundlagenkapitel

- Dateien: Kapitel 01 bis 10 in [`documentation/manual/de/`](../documentation/manual/de/) und später Spiegelung nach [`documentation/manual/en/`](../documentation/manual/en/)
- Gegenprüfen mit:
  - Einstieg und Setup: [`cmd/aurago/`](../cmd/aurago/), [`internal/setup/`](../internal/setup/)
  - UI: [`ui/index.html`](../ui/index.html), [`ui/config.html`](../ui/config.html), [`ui/dashboard.html`](../ui/dashboard.html), weitere UI-Seiten
  - Konfiguration: [`internal/config/`](../internal/config/), [`config_template.yaml`](../config_template.yaml)
  - Tools und Memory: [`internal/tools/`](../internal/tools/), [`internal/memory/`](../internal/memory/)
- Ziel: Installation, Quickstart, Web-UI, Chat-Grundlagen, Tools, Config, Integrationen, Memory und Personality nur entlang real vorhandener Funktionen beschreiben

### 3. Fortgeschrittene Kapitel

- Dateien: Kapitel 11 bis 19 in [`documentation/manual/de/`](../documentation/manual/de/)
- Gegenprüfen mit:
  - Missions/Planner: [`internal/planner/`](../internal/planner/), relevante Server-Handler unter [`internal/server/`](../internal/server/), UI unter [`ui/missions_v2.html`](../ui/missions_v2.html), [`ui/js/`](../ui/js/)
  - Invasion: [`internal/invasion/`](../internal/invasion/), [`ui/invasion_control.html`](../ui/invasion_control.html)
  - Dashboard: [`ui/dashboard.html`](../ui/dashboard.html), zugehörige Handler unter [`internal/server/`](../internal/server/)
  - Skills: [`agent_workspace/skills/`](../agent_workspace/skills/), Skill-bezogene UI unter [`ui/js/skills/main.js`](../ui/js/skills/main.js)
- Ziel: nur reale Bedienpfade, echte Optionen, vorhandene Grenzen und Abhängigkeiten dokumentieren

### 4. Referenzkapitel

- Dateien: Kapitel 20 bis 23 plus [`documentation/manual/de/faq.md`](../documentation/manual/de/faq.md)
- Gegenprüfen mit:
  - Chat-Commands: [`internal/commands/commands.go`](../internal/commands/commands.go), weitere Dateien unter [`internal/commands/`](../internal/commands/)
  - API und Server: [`internal/server/`](../internal/server/)
  - Tools: [`internal/tools/`](../internal/tools/)
  - Architektur und Modulstruktur: [`cmd/`](../cmd/), [`internal/`](../internal/), [`ui/embed.go`](../ui/embed.go)
- Ziel: vollständige, faktisch belastbare Referenz ohne erfundene Endpunkte, Kommandos oder Modulbeschreibungen

### 5. Englisch synchronisieren

- Alle bestätigten Korrekturen aus [`documentation/manual/de/`](../documentation/manual/de/) strukturgleich nach [`documentation/manual/en/`](../documentation/manual/en/) übertragen
- Dabei englische Texte ebenfalls gegen denselben Code prüfen, nicht blind übersetzen

## Konkrete Ausführungsreihenfolge

1. Manual-Startseiten und Navigationspfade prüfen und bereinigen
2. Reale Command-, Tool-, API- und UI-Flächen aus Code sammeln
3. Deutsche Kapitel 01 bis 10 korrigieren
4. Deutsche Kapitel 11 bis 19 korrigieren
5. Deutsche Kapitel 20 bis 23 plus FAQ korrigieren
6. Englische Kapitel vollständig nachziehen
7. Abschlussrunde für Links, Konsistenz, Dubletten und fehlende Inhalte
8. Änderungen committen

## Korrekturregeln

- Veraltete Zahlen wie 50+, 90+ oder 100+ nur verwenden, wenn sie aktuell klar belegbar sind, sonst neutral formulieren
- Nicht vorhandene CLI-Features, UI-Elemente, Integrationen oder Workflows entfernen
- Fehlende tatsächlich vorhandene Funktionen ergänzen, wenn sie für das Kapitel relevant sind
- Relative Links und Kapitelverweise auf reale Dateien prüfen
- Screenshots oder Dateiverweise entfernen oder anpassen, wenn Ressourcen fehlen
- DE und EN dürfen inhaltlich nicht auseinanderlaufen

## Abnahmekriterien

- Jede Datei unter [`documentation/manual/de/`](../documentation/manual/de/) wurde gegen Code oder Konfiguration geprüft
- Jede Datei unter [`documentation/manual/en/`](../documentation/manual/en/) spiegelt den validierten Stand korrekt wider
- [`documentation/manuals/README.md`](../documentation/manuals/README.md) und [`documentation/manual/README.md`](../documentation/manual/README.md) zeigen nur gültige Einstiegspfade
- Keine offensichtlichen toten Links innerhalb des Manual-Baums
- Keine unbelegten oder falschen Produktbehauptungen mehr in den Manuals
- Ergebnis ist bereit für direkten Abschluss in [`💻 Code`](code)

## Übergabe in die Umsetzung

Nach Freigabe wird der Plan ohne weitere Zwischenfreigaben in [`💻 Code`](code) umgesetzt: Audit, Korrekturen, Ergänzungen, Konsistenzprüfung und Abschluss-Commit.
