# Config UX – Welle 3: Sidebar-Struktur und Zustand (2026-10-31)

Quelle: `documentation/audits/2026-10-30-config-ux-audit.md` §8 (IA1–IA13, Zielstruktur) und §9 „Welle 3“.
Basis: lokaler `main` mit Welle 0–2. Branch `claude/cfg-wave3`, Worktree `.worktrees/cfg-wave3`.

## Ziel

Die Sidebar folgt dem Einrichtungsweg (14 Gruppen, innerhalb nach Wichtigkeit), zeigt pro Eintrag den Zustand
(aktiv / eingerichtet, aber aus / Fehler bei der letzten Prüfung / Neustart nötig), lässt sich auf „Nur aktive“ filtern,
und die Übersicht zeigt aktive Integrationen und Probleme. Sektions-Keys und URLs (`#providers` …) bleiben unverändert.

## Verträge

1. **Gruppen:** Jede Gruppe in `SECTIONS` bekommt eine stabile `id`; das Label kommt aus `config.group.<id>`
   (`ui/lang/config/sections/<lang>.json`, 16 Sprachen). Reihenfolge und Inhalt wie §8:
   `models` · `behavior` · `speech` · `tools` · `tool_sources` · `apps` · `media` · `communication` · `productivity` ·
   `storage` · `smart_home` · `infrastructure` · `security` · `system` · `danger` (abgesetzt, `dangerGroup: true`).
   Dograh (in §8 vergessen) kommt zu `speech`. `onedrive`/`devices` werden nur verlinkt, wenn Modul und Backend-Config
   vorhanden sind und die Sektion rendert; sonst bleibt es dokumentiert offen. Alte `config.group.*`-Keys, die niemand mehr
   nutzt, werden entfernt.
2. **Labels (IA11):** deutsche Labels ohne Sprachmix, wo es ein übliches deutsches Wort gibt (Decisions → Entscheidungen,
   Detective → Detektiv, Newspaper → Zeitung, Logging → Protokolle, Vision → Bildverständnis); Produktnamen bleiben
   (Space Agent, EasyDrag, RuView). `ruview` bekommt einen echten Label-Key. Doppelt definierte Keys an beiden Stellen
   gleich ändern (Duplikat-Test).
3. **Icons (IA13):** Das ungenutzte Emoji-Feld `icon` verschwindet aus `SECTIONS` und aus den sechs Form-Builder-Modulen
   (webdav, koofr, grafana, evomap, adguard, agentmail); die Sidebar nutzt weiter das SVG-Sprite je Key.
4. **Zustand je Sektion** – eine Funktion `cfgSectionState(key)` (neu, `ui/js/config/section_state.js`) liefert
   `{ state, reason }` mit `state` ∈ `active` | `configured` | `error` | `restart` | `off` | `none`:
   - `active`: Enable-Flag an. Die Pfade stehen in einer Tabelle `sectionStatePaths` in `catalog.js` (Standard
     `<key>.enabled`; Ausnahmen wie `tools.web_scraper.enabled`, `a2a.server.enabled`, `auth.enabled`, telegram = Token
     vorhanden, tts = Anbieter gesetzt; angelehnt an `internal/server/dashboard_integrations.go`).
   - `configured`: Flag aus, aber Pflichtfelder/Zugangsdaten gesetzt (aus `actionRules` `requiredPaths`/`credentialPaths`,
     gespeicherte Secrets zählen).
   - `error`: die letzte Statusprüfung dieser Sektion (cfgStatusBanner.check) meldete `error`/`degraded` in dieser Sitzung.
     `feedback.js` meldet Ergebnisse als Event `aura-config-section-status` `{section, state, reason}`; gespeichert in
     `sessionStorage` (`aurago.config.sectionStatus.v1`), nicht im Draft.
   - `restart`: nach einem Speichern mit `needs_restart` die Sektionen, deren Pfade geändert wurden, bis der Server neu
     gestartet ist (Merkmal: `restart_from`/Startzeit aus der Save-Antwort bzw. vorhandenem Status; sonst bis Sitzungsende).
   - `none`: Kernbereiche ohne Ein/Aus (server, llm, agent, providers, directories …) – kein Punkt.
   Vorrang: error > restart > active > configured > off.
5. **Darstellung:** kleiner Punkt am Sidebar-Eintrag (Klassen `cfg-state-dot cfg-state-<state>`, Farben aus Theme-Tokens,
   `configured` als Ring), Zustand zusätzlich als Text für Screenreader (`aria-describedby`/visually-hidden) und im `title`.
   Kein Punkt für `off`/`none`. Aktualisiert sich nach Laden, Speichern, Toggle-Änderung (Draft) und Status-Events.
6. **Filter „Nur aktive“:** Schalter oben in der Sidebar (neben/unter der Suche); zeigt nur Einträge mit `active`, `error`,
   `restart` (und die aktuelle Sektion), blendet leere Gruppen aus, kombiniert sich mit der Suche, Zustand in
   `localStorage` (`aurago-cfg-only-active`). Leerer Zustand mit Hinweistext.
7. **Übersicht:** neuer Block zwischen Hero und „Zuletzt geöffnet“: „Aktiv“ (Liste der aktiven Integrationen mit Punkt,
   klickbar) und „Braucht Aufmerksamkeit“ (Fehler, Neustart nötig, gesperrte Sektionen mit Grund). Gruppenkarten zeigen
   „n aktiv“. Klasse `pw-overview-stat` bleibt verboten (bestehender Test).
8. **Einklapp-Zustand:** `aurago-cfg-groups-collapsed` speichert Gruppen-`id`s statt übersetzter Namen (alte Werte werden
   ignoriert); DOM-IDs der Gruppen über `id` statt Index; Auto-Aufklappen der aktiven Sektion über `id`.
9. **Texte:** alle neuen Texte in 16 Sprachen (Welle-2-Paritätstests gelten), Guard `TestConfigEveryControlLabeledBrowser`
   bleibt grün (der neue Schalter braucht Label + Beschreibung).

## Pakete

- **A – Struktur (Verträge 1–3):** `SECTIONS` neu ordnen + Gruppen-`id`s, Gruppen-/Sektionslabels in 16 Sprachen, Icon-Feld
  entfernen, Tests anpassen, die Zeilen/Icons/Reihenfolge festschreiben (cyd_creator, local_wikipedia, precision workspace).
  Besitzt den `SECTIONS`-Block in `main.js`, `ui/lang/config/sections/*`, Label-Keys, die sechs Form-Builder-Module.
- **B – Zustand (Verträge 4–8):** `section_state.js`, `sectionStatePaths` in `catalog.js`, Status-Event in `feedback.js`,
  Restart-Merker, Sidebar-Punkte, Filter, Übersicht, Einklapp-Zustand per `id` (nutzt `group.id`, Fallback Index), CSS,
  Texte in 16 Sprachen (`ui/lang/config/common/*`), Tests (statisch + Browser). Besitzt `buildSidebar`/Overview/Search-Teile
  von `main.js`, `config.html` (Script-Tag), CSS.
- **Z – Abschluss:** Merge, Review, Report §9 „Stand Welle 3“, Tests.

## Tests

- `go test ./ui -skip Browser -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestConfig(Precision|UX|Sidebar|EveryControl|Refresh|Wave3)|Decisions' -count=1 -timeout 30m`
- Bekannt rot: siehe Report §9 „Stand Welle 1/2“.
