# Config UX – Welle 4: Design-Angleichung (2026-10-31)

Quelle: `documentation/audits/2026-10-30-config-ux-audit.md` §5 (U16–U21), §6 (U22, U23), §9 „Welle 4“ und
`reports/ui-consistency-audit-2026-10-31.md` §2.5 (M1/M2, M18–M22, M24).
Basis: lokaler `main` mit Welle 0–3 und den geteilten Bausteinen der UI-Konsistenz-Session (`showConfirmDialog`,
`openModal`/`closeModal` mit `onClose`, `.pw-empty-state`, `.pw-loading-state`, `.pw-badge[data-status]`).
Branch `claude/cfg-wave4`, Worktree `.worktrees/cfg-wave4`; Modulpakete in `.worktrees/cfg-w4-m<n>`.

## Ziel

Alle Config-Module sehen gleich aus und verhalten sich gleich: Enable-Schalter oben, ein Schaltertyp, SVG statt Emoji,
ein Lösch-Stil, ein Dialog-System, eine Status-Pille, einheitliche Lade-/Leer-/Fehlerzustände, gemeinsame `cfg-*`-Klassen,
Expertenfelder eingeklappt, Eingaben werden sofort geprüft statt still verworfen. Einfache Module nutzen den Form-Builder.

## Entscheidungen

- **E1 Drawer-Breakpoint 900 px** wie im Dashboard (statt 1099 px). Zwischen 900 und 1099 px muss das Layout tragen
  (Browser-Test bei 1000 px: kein horizontales Scrollen, Sidebar sichtbar).
- **E2 Verlauf:** Sektionswechsel per Klick/Suche/Übersicht = `pushState`; Zurück/Vor (`popstate`) wechselt die Sektion über
  denselben Unsaved-Changes-Weg; bleibt der Nutzer, wird der alte Hash wiederhergestellt. Erstes Laden und Normalisieren
  (`#unbekannt` → Fallback) bleiben `replaceState`. Tab-Titel = „<Sektion> · <bisheriger Titel>“.
- **E3 Klassen:** `adg-*` → `cfg-*` überall (JS, CSS, Tests): `adg-test-btn`→`cfg-test-btn`, `adg-password-row`→`cfg-secret-row`,
  `adg-password-input`→`cfg-secret-input`, `adg-password`→`cfg-secret`, `adg-save-btn`→`cfg-save-btn`,
  `adg-quick-stats`/`adg-stats-*`→`cfg-stats`/`cfg-stats-*`, `adg-status-banner`/`adg-test-result` → entfernen, falls tot,
  sonst `cfg-`. Statischer Test verbietet `adg-`.
- **E4 Schalter:** Ein/Aus-Einstellungen sind immer `.toggle` (wie der Form-Builder sie rendert). Native Checkboxen nur noch
  für „Einträge aus einer Liste auswählen“ und dann mit `data-cfg-choice`. Jede `div.toggle[role=switch]` ist per Tastatur
  bedienbar (`tabindex=0`, Leertaste/Enter) – zentral in `presentation.js`, nicht pro Modul.
- **E5 Enable oben:** Gibt es in einer Sektion ein Steuerelement für ihren Enable-Pfad (`sectionStatePaths`, Standard
  `<key>.enabled`), ist es das erste sichtbare Steuerelement der Sektion (nach Banner/Intro). Gilt auch für den generischen
  Renderer. Ausnahmen nur über eine kommentierte Liste im Test.
- **E6 Icons:** `ui/js/config/icons.js` mit `cfgIcon(name, {label})` – ein kleiner Satz Inline-SVGs (stroke `currentColor`,
  `aria-hidden` ohne Label), keine externen Assets. Abschnittstitel, Buttons und Leerzustände in `ui/cfg` nutzen ihn; kein
  Emoji mehr in `ui/cfg/*.js` (Ratchet `ui/testdata/config_emoji_modules.txt`, endet leer; Ausnahme nur Inhalte wie
  Flaggen in Sprachlisten über eine kommentierte Liste).
- **E7 Löschen:** ein Stil – `btn-danger` (geteilt) über `cfgDeleteButton({label, iconOnly, attrs})`: Papierkorb-SVG, sichtbarer
  Text oder bei `iconOnly` `aria-label` + `title`. Nie `btn-save`/`btn-secondary` für Löschen. Eigene Klassen
  (`wh-btn-danger`, `cfgdev-icon-btn-danger`, `em-card-btn-delete`, `mcp-card-btn-delete`, `devices-action-btn-delete`,
  `prov-icon-btn is-delete`, `sip-danger-button`) verschwinden. Rückfrage über `showConfirmDialog({danger: true, …})`.
- **E8 Dialoge:** Config-Dialoge nutzen das geteilte Overlay (`.modal-overlay` + `openModal`/`closeModal`, `role=dialog`,
  `aria-modal`, `h2`-Titel, Schließen-Button mit SVG-X). Helfer `cfgModal({id, title, body, actions, size})` baut das Markup
  einmal. Eigene Familien (`cfgdev-modal-*`, `devices-modal-*`, `cmp-`, `mcp-`, `prov-`, `sec-`, `sqlconn-`, `og-modal-*`,
  `email-modal-card`, `tts-voice-modal`, `secrets-modal-card`) verschwinden (Test mit Präfixliste).
- **E9 Status-Pillen:** `cfgBadge(status, text)` → `<span class="pw-badge" data-status="ok|warn|error|idle|running">`.
  Eigene `*-badge`/`*-pill`-Klassen für Status in `ui/cfg` verschwinden (Ratchet).
- **E10 Listen-Zustände:** `cfgListState(el, {state: 'loading'|'empty'|'error', title, text, retry})` auf Basis von
  `.pw-loading-state`/`.pw-empty-state`; der Fehlerzustand hat „Erneut versuchen“, wenn `retry` gesetzt ist.
- **E11 Feedback-Regel:** Ergebnis eines Klicks → `cfgActionResult` am Button (Toast höchstens zusätzlich). `showToast`
  bleibt für Hintergrund-Ereignisse und „Kopiert“. Keine Emoji-Präfixe (❌ ✅) und keine rohen URLs in Toasts, kein
  hartkodiertes Englisch. `showAlert` für blockierende Speicher-/Neustartfehler in `main.js` bleibt (bewusst).
- **E12 Form-Builder:** Module, die nur Felder + Test-Button + Banner rendern, werden auf `AuraConfigForm.renderSpec`
  umgestellt. Ratchet `ui/testdata/config_handbuilt_modules.txt` (Module ohne `renderSpec`) darf nur schrumpfen. Große Module
  mit eigenen Widgets (providers, sip, webhooks, mcp, decisions …) bleiben handgebaut, nutzen aber die Helfer aus E6–E10.
- **E13 Expertenfelder:** `sectionTiers` in `catalog.js` markiert Felder, die ein typischer Nutzer nie ändert (Timeouts,
  Retries, Puffer, interne Pfade, Debug-Schalter), als `advanced` – Schwerpunkt Tool-Berechtigungen, Optimierungen,
  Datenbanken, Agent, LLM, Server, Speicher-Pfade. Pflichtfelder und Enable-Schalter nie.
- **E14 Validierung:** `validationRules` kennt `url`, `range {min,max}`, `integer`, `unit`. Abgeleitete Standardregeln für
  generische Felder: `port` → 1–65535, `*url`/`base_url`/`endpoint` → URL, `*_seconds`/`*_ms`/`*_minutes`/`timeout*`/
  `interval*` → ≥ 0 mit Einheit. Prüfung beim Tippen/Verlassen (`aria-invalid`, Meldung mit `role=alert` unter dem Feld),
  Speichern bleibt gesperrt. Explizite Tabelleneinträge schlagen die Ableitung.
- **E15 Still verworfene Eingaben (U23)** melden sich inline statt zu verschwinden: Detective-Allowlist, MCP-Env
  `KEY=VALUE`, Webhook-Header, leerer Indexierungspfad, gog-Kontolimit, E-Mail-ID-Bereinigung.

## Pakete

**F – Fundament (zuerst, allein, Worktree `cfg-wave4`):**
1. `ui/js/config/icons.js` (E6), Helfer `cfgDeleteButton`, `cfgModal`, `cfgBadge`, `cfgListState` (neu
   `ui/js/config/components.js` oder in `feedback.js`), Script-Tags in `config.html`, Fixture-Regel wie bei `feedback.js`.
2. Tastatur für `div.toggle[role=switch]` zentral (E4); `.section-header`-Ebenen korrigieren (Seitentitel h1, Sektionen
   `aria-level=2` statt überall 1).
3. E1 + E2 in `main.js`/`config-workspace.css`.
4. E3 komplett (alle 37 Module, CSS, Tests) als mechanischer Schritt – danach berühren Modulpakete `adg-` nicht mehr.
5. Form-Builder erweitern, was Modulpakete brauchen: `number` mit `min`/`max`/`step`/`unit`, `url`-Typ, `advanced`,
   Auswahlliste (`data-cfg-choice`), Aktionszeile mit `data-config-action`, Slot für `cfgStatusBanner`.
6. Generischer Renderer: Enable-Pfad zuerst (E5).
7. Geteilte Texte (Löschen, Schließen, Erneut versuchen, Lädt, Leer, Validierungsmeldungen) in `ui/lang/config/common/`
   bzw. `generic/`, 16 Sprachen.
8. Guards (statisch, `ui/config_wave4_*_test.go`): kein `adg-`; Emoji-Ratchet; native Checkbox nur mit `data-cfg-choice`
   (Ratchet); verbotene Lösch-/Modal-/Badge-Präfixe (Ratchet-Dateien je Muster in `ui/testdata/`); Form-Builder-Ratchet;
   Browser-Guard „Enable zuerst“ über alle Sektionen (mit Ratchet `config_enable_not_first.txt`); Browser-Test
   Drawer/Back bei 1000 px und 800 px.

**M1–M5 – Module (parallel nach F, je ein Worktree, disjunkte Modullisten):**
- M1: providers llm_router local_llm ollama co_agents prompts rules decisions guardian llm_guardian
- M2: mcp mcp_server a2a composio manus huggingface treg evomap ai_gateway omniroute manifest dograh security_proxy
  output_compression memory_analysis heartbeat skill_manager daemon_skills mission_preparation webhooks
- M3: sip telephone_agent realtime_speech speech_lab tts music_generation image_generation video_generation
  media_conversion video_download document_creator scraper browser_automation virtual_desktop virtual_computers space_agent
  game_maker cyd_creator detective newspaper flows local_wikipedia
- M4: email agentmail github google_workspace gog paperless obsidian yepapi s3 webdav koofr onedrive netlify vercel here_now
  cloudflare_tunnel homepage server auth auth_webauthn backup updates indexing firewall secrets danger sandbox
  sql_connections
- M5: mqtt cyd chromecast bluetooth meshcore adguard fritzbox ldap network_shares truenas uptime_kuma jellyfin tailscale
  go2rtc ruview three_d_printers remote_control grafana devices telnyx

Je Modul: E4 (Schalter), E5 (Enable oben), E6 (Icons), E7 (Löschen), E8 (Dialoge), E9 (Pillen), E10 (Listen), E11 (Feedback,
inkl. M24), E12 (Form-Builder, wo einfach), E15 (eigene Fälle). Ratchet-Zeilen der eigenen Module entfernen.
Modul-CSS: nur Regeln der eigenen, abgelösten Klassen in `config.css` löschen; neue geteilte Regeln nur über F.
Texte: nur in den Sprachdateien der eigenen Module (`ui/lang/config/<modul>/` bzw. wo die Keys schon liegen).

**V – Validierung + Expertenfelder (parallel zu M, berührt nur `catalog.js`, Validierungscode in `main.js`/`state.js`,
Validierungstexte):** E13 + E14, Browser-Test für Inline-Prüfung (Port 70000, URL ohne Schema, negativer Timeout) und für das
Einklappen der Expertenfelder in Tool-Berechtigungen.

Reihenfolge: F → (M1, M2, V) → (M3, M4, M5) → gemeinsame Review → Fixes → Landen (ff, keep-foreign).

## Prüfung

- `node --check` auf geänderte JS-Dateien; `go test ./ui -skip Browser -count=1`.
- Browser je Paket: Guards aus F, Welle-2-Guard `TestConfigEveryControlLabeledBrowser`, Welle-3-Sidebar-Tests, Modul-Tests der
  eigenen Module (`-run` mit `(?i)(…)`-Gruppe), Fixture-Tests, die `form-builder.js` laden.
- Bekannt rot (ignorieren): siehe Memory `project_config_ux_audit`; `TestConfigSpacingBrowser` nie starten.
