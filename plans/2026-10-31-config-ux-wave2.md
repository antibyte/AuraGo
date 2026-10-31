# Config UX – Welle 2: Texte, Beschriftungen, Hilfen (2026-10-31)

Quelle: `documentation/audits/2026-10-30-config-ux-audit.md` §4 (Tabelle, U10–U15), §8 (Sidebar-Kurzbeschreibungen), §9 „Welle 2“.
Basis: lokaler `main` 4aa0af74d (Welle 0 + 1). Branch `claude/cfg-wave2`, Worktree `.worktrees/cfg-wave2`.

## Ziel

Jedes sichtbare Steuerelement der Config-Seite hat ein echtes, übersetztes Label und eine kurze Beschreibung.
Keine englischen `formatKey`-Labels mehr, keine Hilfe nur im Platzhalter, keine hartkodierten Texte, keine Rohschlüssel.
Ein Browser-Guard über alle Sektionen verhindert Rückfälle.

## Verträge

1. **Generische Labels:** `config.<pfad>_label` (wie bisher von `fieldLabelText` gelesen). Neue generische Labels liegen in
   `ui/lang/config/generic/<lang>.json` (neuer Ordner, gehört Paket 1). `fieldLabelText` setzt beim Rückfall auf
   `formatKey` ein Marker-Attribut `data-label-fallback="true"` am `.field-label` (für den Guard).
2. **Hilfetexte generischer Felder:** `help.<pfad>` in `ui/lang/help/<lang>.json` (gehört Paket 1).
3. **Hilfetexte eigener Renderer:** Modul-Keys in `ui/lang/config/<modul>/<lang>.json` (wie Welle 1), gerendert als
   `.field-help` bzw. Form-Builder-`help`. Vorhandene, nie gerenderte Hilfen (homepage, sandbox, web_scraper, tailscale, tts)
   werden angeschlossen, nicht dupliziert.
4. **Kein Hilfetext nur im Platzhalter (U12):** Platzhalter bleiben Beispiele; die Erklärung steht als Hilfe darunter.
5. **Keine hartkodierten Texte (U14)** in `ui/cfg/*.js` und `ui/js/config/*.js`: sichtbarer Text geht über `t()`.
   Einheiten (GB/MB/s/USD) ebenfalls über Keys.
6. **`t()`-Rückfall (U15):** gemeinsamer Helfer `tOr(key, fallback)` in `ui/js/config/utils.js` (gibt `fallback` zurück, wenn
   `t()` den Key selbst liefert oder leer ist). `t('a') || t('b')` wird überall ersetzt; ein statischer Test verbietet das Muster.
7. **Sidebar-Kurzbeschreibungen** (`config.section.*.desc`, alle Sektionen): genau ein Satz, was der Bereich tut, mit Punkt am Ende,
   20–110 Zeichen in de/en; keine Stichwortlisten, kein Disclaimer. Begriffe wie gog, Decisions, Manifest, OmniRoute, Dograh, treg,
   EvoMap, RuView, zweimal „MCP-Server“ werden unterscheidbar erklärt (U13).
8. **Sprachen:** Die Pakete 1–4 schreiben **de und en** (beide echt, natürliches Deutsch mit Umlauten). Paket T übersetzt alle neuen
   und geänderten Keys in die übrigen 14 Sprachen. Danach erzwingt ein globaler Paritätstest: jeder Key aus `ui/lang/config/**/en.json`
   und `ui/lang/help/en.json` existiert in allen 16 Sprachen (bestehende Lücken: Baseline, die T leert).
9. **Guard (Browser):** `TestConfigEveryControlLabeledBrowser` rendert jede Sektion aus `SECTIONS` mit Fixture-Config und prüft jedes
   sichtbare Eingabe-Steuerelement (input ohne hidden, select, textarea, `.toggle`, `[role=switch]`): Label vorhanden (Text in
   `.field-label`/`label[for]`/`aria-label`/`aria-labelledby`, nicht `data-label-fallback`) und Beschreibung vorhanden
   (`.field-help` bzw. Hilfe-Element der Gruppe oder `aria-describedby` mit Text). Lücken werden mit Sektion + Pfad/ID als Zeile
   in `ui/testdata/config_text_gaps.txt` (sortiert) geführt. Der Test schlägt nur bei **neuen** Lücken fehl; erledigte Zeilen werden
   gemeldet. `AURAGO_UPDATE_TEXT_GAPS=1` schreibt die Datei neu. Paket Z verlangt eine leere oder begründete Restliste.

## Pakete

- **0 – Fundament (zuerst, ein Agent):** Guard + Baseline (Vertrag 9), Marker in `fieldLabelText` (1), `tOr` + Ersetzen aller
  `t(..) || t(..)` + statischer Verbotstest (6), globaler Paritätstest mit Baseline-Datei `ui/testdata/config_i18n_missing.txt` (8),
  statischer Test gegen doppelte Keys mit unterschiedlichem Wert über Dateien hinweg (Ladereihenfolge macht sonst still einen Text
  ungültig), Sidebar-Beschreibungstest (7) mit Baseline. Liefert die Lückenliste pro Sektion für die Paketaufteilung.
- **1 – Generische Sektionen:** alle Labels der generischen Felder (§4: 302) und fehlenden `help.*` (74), inkl. sqlite, frigate,
  directories, agent, optimizations, notifications, llm, proxmox, ansible, network_tools, rocketchat, docker, logging,
  personality, home_assistant. Besitzt `ui/lang/config/generic/` und `ui/lang/help/`.
- **2 – Eigene Renderer A:** server, sql_connections, security_proxy, guardian (inkl. `config.llm_guardian.tool_*`), realtime/speech,
  telephone_agent, local_llm/local models, co_agents, rules, game_maker, cyd_creator, detective, newspaper, flows, virtual desktop
  (video studio, retro), secrets.
- **3 – Eigene Renderer B:** tts (ElevenLabs, Supertonic, Piper; Abschnittstitel), image/video/music generation, homepage, sandbox,
  web_scraper, tailscale, meshcore, sip, go2rtc, ruview, remote_control (`ssh_insecure_host_key`), fritzbox, email (Konto-Modal),
  mcp (Modal inkl. `allow_private_network`, `allow_destructive`), webhooks (Editor, Feldmapping), a2a, ai_gateway, gog,
  google_workspace, github, yepapi, treg, totp/auth.
- **4 – Sidebar + Begriffe + Rest:** alle `config.section.*.desc` (Vertrag 7, U13), restliche Module mit Guard-Lücken, die nicht in
  1–3 stehen (laut Baseline aus Paket 0), hartkodierte Texte in `ui/js/config/*`.
- **T – Übersetzung (nach 1–4, zwei Agenten je 7 Sprachen):** alle Keys aus der Paritäts-Baseline und alle in 1–4 neu
  angelegten/geänderten Keys in cs, da, el, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh. Baseline danach leer.
- **Z – Abschluss:** Guard-Baseline neu schreiben und leeren (Rest begründet), Report §9 „Stand Welle 2“, Tests.

Jedes Paket: nur eigene Module/Dateien; Pakete 2–4 fassen weder `ui/lang/help/` noch `ui/lang/config/generic/` an.
Gemeinsame Dateien (`ui/js/config/*`, `ui/cfg/form-builder.js`, `ui/config.html`, `ui/lang/config/common/*`,
`ui/lang/config/sections/*`, Guard-/Paritätstests und Baselines) gehören Paket 0, danach Paket 4 (sections, `ui/js/config/*`-Texte)
bzw. Z (Baselines). Pakete melden nötige Änderungen an fremden Dateien im Bericht.

## Tests

- `go test ./ui -skip Browser -count=1`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestConfigEveryControlLabeledBrowser|<Modul-Tests>' -count=1 -timeout 20m`
- Bekannt rot/hängend (nicht durch Welle 2): siehe Report §9 „Stand Welle 1“.
