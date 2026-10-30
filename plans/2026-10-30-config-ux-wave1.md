# Config UX – Welle 1: gemeinsame Bausteine (2026-10-30)

Quelle: `documentation/audits/2026-10-30-config-ux-audit.md` §2, §3, §9 (Welle 1). Welle 0 ist auf main (`ef512e7ed`).
Worktree `.worktrees/cfg-wave1`, Branch `claude/cfg-wave1`.

## Ziele

1. **Ergebniszeile** `cfgActionResult(button, run, opts)`: Busy, Ergebnis direkt am Button mit Server-Grund, `role=status`.
2. **Statusbanner** `cfgStatusBanner(el, state, opts)` mit festen Zuständen
   `off · unconfigured · checking · connected · degraded · error(reason) · unknown(+retry)`; nie hängend (Timeout → unknown).
3. **Secrets ins Save-Dock**: Secret-Eingaben bekommen `data-path`, die eigenen „In Vault speichern“-Buttons entfallen;
   der Server schreibt sie über `extractSecretsToVault` in den Vault (gibt es schon).
4. **Test mit eingegebenen Werten**: Test-Buttons schicken den Entwurf, der Server testet ihn ohne zu speichern
   (maskierte/leere Secrets → gespeicherter/Vault-Wert). Tests, die echte Laufzeit brauchen, bleiben gesperrt mit Grund-Zeile.
5. **Rückfrage beim Einschalten** gefährlicher Schalter (Stufe A löschen/zerstören, Stufe B Agent-/Host-Rechte).

Entscheidungen: D1 Zustand `degraded` ja; D2 Vault-Zeilen werden im selben Modul-Paket auf das Dock umgestellt
(nicht erst auf cfgActionResult); D3 Bestätigung für Stufe A und B. Löschen eines Secrets über das Dock bleibt außen vor
(leer/maskiert = unverändert, wie bisher); Löschen weiter über den Vault-Manager.

## Verträge

### UI (`ui/js/config/feedback.js`, neu, zwischen `state.js` und `actions.js` geladen)

- `window.AuraConfigFeedback = { read, reason, busy, actionResult, showResult, resultHTML, statusBanner, bannerHTML, checkBanner, draftPatch, BANNER_STATES }`;
  Kurzformen `cfgActionResult` (+ `.show`, `.html`), `cfgStatusBanner` (+ `.html`, `.check`), `cfgResponseReason`, `cfgDraftPatch`.
- `read(value)` → `{ok, state: success|warning|error|cancelled, httpStatus, body, text, reason}`; Body wird einmal gelesen.
  Erfolg: `body.status ∈ ok success connected online ready running healthy`; Warnung: `partial warning degraded untested skipped`;
  ohne `status`: `response.ok && body.success!==false && body.ok!==false && !body.error`. Grund-Reihenfolge:
  `opts.reasonText` → `error(+details)` → `message` → `detail`/`reason` → `errors[0]` → Text (kein HTML) → `HTTP <code>`. Max. 300 Zeichen, nur `textContent`.
  Netzwerkfehler → `config.common.network_error`; `AbortError` → cancelled.
- Ergebniszeile: `<span id="…" class="cfg-action-result" role="status" aria-live="polite" aria-atomic="true" data-state="pending|success|warning|error">`;
  Button: `disabled`, `aria-busy`, `data-config-action-busy="true|false"` (nach Lauf `'false'`, nicht löschen), Label bleibt unverändert.
  Optionen: `target, pending, success, successDetail, failure, interpret, reasonText, confirm, lock, toast ('never'|'error'|'always'), clearAfter, onSuccess, onError`.
  Nie rejecten; busy/locked → `{skipped}`; Stale-Guard per Laufzähler.
- Banner: `<div class="cfg-state-banner" data-state="…" role="status" aria-live="polite">` mit `.cfg-state-dot`, `.cfg-state-label`, `.cfg-state-detail`, optional `.cfg-state-retry`, `links`.
  `cfgStatusBanner.check(el, fetcher, map, {timeoutMs:15000, retry})`.
- **Klassennamen tabu**: `cfg-status-banner` (Legacy-Audit-Test), `.cfg-status-*` (tote CSS).
- `cfgDraftPatch(prefixes)` → Patch-Objekt (gleiche Form wie der PUT /api/config) mit den **aktuellen Entwurfswerten** aller
  `data-path`-Felder unter den Präfixen; maskierte (`••••••••`) und leere Secret-Werte werden weggelassen.
- Test-Buttons, die den Entwurf schicken, tragen `data-config-action-draft="true"`. `actions.js` sperrt sie **nicht** bei
  ungespeicherten Änderungen; `requiredPaths`/`credentialPaths` werden dann gegen den **Entwurf** geprüft (Maske zählt als vorhanden).
  Buttons ohne das Attribut verhalten sich wie bisher (requiresSaved). Tests, die nicht mit dem Entwurf gehen (Klasse C),
  bekommen `data-config-action-draft="false"` und eine Grund-Zeile `config.feedback.runtime_only` („Prüft den laufenden Dienst – erst speichern“), solange etwas ungespeichert ist.
- Secret-Felder mit `data-path`: Anzeige leer + Platzhalter „gespeichert“, wenn der Server `••••••••` liefert.
  `state.js` darf leeres Feld vs. gespeicherte Maske **nicht** als ungespeicherte Änderung zählen.
- Gefährliche Schalter: Registry `confirmOnEnable` in `catalog.js` (Pfad → Kategorie `delete|destroy|host|write`),
  zentraler Capture-Guard in `actions.js` für `.toggle[data-path]` und `[data-cfg-confirm-enable]`; nur beim Einschalten.

### Server (`internal/server`)

- `func (s *Server) draftTestConfig(r *http.Request) (*config.Config, error)` (neue Datei `config_draft_test_config.go`):
  Body optional `{"draft": <Patch wie PUT /api/config>}`. Ergebnis = Klon des gespeicherten Snapshots, darüber der Entwurf;
  Secret-Blätter (sensitiveKeys / vaultKeyMap-Pfade, auch `yaml:"-"`-Felder) werden mit dem Entwurfswert gesetzt, wenn er nicht leer und nicht `••••••••` ist,
  sonst gilt der gespeicherte/Vault-Wert. **Schreibt nichts** (keine Datei, kein Vault, kein Snapshot-Mutieren). Ungültiger Body → 400 mit Grund.
- `handleConfiguredIntegrationTest` nutzt `draftTestConfig`.
- `vaultKeyMap` + `config_secret_migration.go` angleichen: `fritzbox.password`, `homepage.deploy_password`, `homepage.deploy_key` (+ GET-Indikator).
- PUT-Nebenwirkungen wie beim Vault-POST: MQTT-Passwort → `RegisterSensitive` + `refreshMQTTPassword`; tsnet-Auth-Key →
  `validateTsNetAuthKey` vor dem Schreiben (ungültig → 400, nichts geschrieben) + `applyTsNetCredentialMutation`; Secret-Werte trimmen.
- Neue Test-Routen (heute nur Status): `POST /api/jellyfin/test`, `POST /api/truenas/test`, `POST /api/obsidian/test` – mit `draftTestConfig`.
- GET-Test-Endpunkte (Bild/Video/Musik/Google Workspace/OneDrive) nehmen zusätzlich POST mit Body.

## Pakete

Jedes Modul-Paket ist ein **vertikaler Schnitt**: Modul-JS + zugehöriger Handler + Tests des Moduls. Keine geteilten Dateien
(nicht anfassen: `ui/js/config/*`, `ui/cfg/form-builder.js`, `ui/lang/config/common/*`, `ui/lang/config/<lang>.json` (flach),
`ui/lang/config/sections/*`, `internal/server/server_routes*.go`, `internal/server/config_handlers_main.go`,
`ui/config_layout_regression_test.go`, `ui/config_wave0_*_test.go`). Neue Modul-Texte in die Modul-Sprachdateien
(`ui/lang/config/<modul>/<lang>.json`, alle 16 Sprachen). Funktionsnamen, Endpunkt-Strings und von Tests gelesene IDs bleiben.

**Paket 0a – UI-Fundament** (seriell zuerst, parallel zu 0b): `feedback.js`, `actions.js` (ein Busy-System, Draft-Regel, Toggle-Guard, `aria-describedby` zusammenführen),
`catalog.js` (`confirmOnEnable`), `state.js` (Secret-Dirty), `integration_actions.js` (auf `cfgActionResult` + `cfgDraftPatch` + Draft-Attribut),
`presentation.js` (neue Klassen; Vault-Hinweis nur noch, wo ein Button bleibt), `form-builder.js` (`form.password` nimmt `path`; `form.actionResult`, `form.statusBanner`; Layout kennt `.cfg-state-banner`),
`config.html`, `css/config-workspace.css`, 16× `lang/config/common`, `data-cfg-confirm-enable` in `mcp.js`/`sql_connections.js`-Modalen.
Tests: `ui/config_feedback_test.go` (statisch + Browser-Unit für read/actionResult/statusBanner/draftPatch/Toggle-Guard),
Marker in `config_layout_regression_test.go` und `config_wave0_states_test.go` so lockern, dass alte Marker **oder** `cfgActionResult(`/`cfgStatusBanner(` akzeptiert werden;
`config_precision_workspace_test.go` (Skriptliste, Schlüssel).

**Paket 0b – Server-Fundament** (parallel zu 0a): `draftTestConfig` + Tests, Integrationstest-Handler, vaultKeyMap/Migration-Map,
PUT-Nebenwirkungen + Trim, drei neue Test-Routen, POST für GET-Tests. Tests in `internal/server/*_test.go`.

**Paket 1** (T): adguard, agentmail, evomap, grafana, jellyfin, koofr, mqtt, obsidian, telnyx, truenas, uptime_kuma, webdav.
Server-Draft: adguard_handlers.go:48, agentmail_handlers.go:63, evomap_handlers.go:101, grafana_handlers.go:90, koofr_handlers.go:27,
mqtt_handlers.go:80, telnyx_handlers.go:28, uptime_kuma_handlers.go:74, webdav_handlers.go:27 (+ neue jellyfin/truenas/obsidian-Tests aus 0b nutzen).
Secrets → Dock: adguard.password, agentmail.api_key, evomap.api_key/node_secret, grafana.api_key, jellyfin.api_key, koofr.app_password,
mqtt.password, obsidian.api_key, telnyx.api_key, truenas.api_key, uptime_kuma.api_key, webdav.token|password (Pfad je auth_type).
Besitzt `ui/config_refresh_interactions_test.go`.

**Paket 2** (J): ai_gateway, bluetooth, cloudflare_tunnel, composio, dograh, fritzbox, huggingface, manifest, manus, network_shares, omniroute.
Server-Draft: ai_gateway (ProviderEntry aus Body, Maske → gespeichert), composio_handlers.go:52, huggingface_handlers.go:49,
manifest_handlers.go:27, manus_handlers.go:42, omniroute_handlers.go:27; fritzbox ist schon Draft (nur UI-Sperre lösen).
Secrets → Dock: cloudflare_tunnel.token, composio.api_key, fritzbox.password, huggingface.token, manus.api_key.
Dograh: Webhook-Token-Anzeige in eigenes Element, Banner mit `links`; network_shares: `reasonText` mit lokalisierten Codes.

**Paket 3** (T): browser_automation, document_creator, email, image_generation, ldap, media_conversion, music_generation, paperless,
space_agent, three_d_printers, video_generation, yepapi.
Server-Draft: image_generation_handlers.go:31, music_handlers.go:13, video_generation_handlers.go:14, ldap_handlers.go:18,
paperless_handlers.go:12, yepapi_handlers.go:14, document_creator (URL-Health; Managed-Restart bleibt gespeichert);
schon Draft: browser_automation, email, media_conversion, three_d_printers. space_agent = C.
Secrets → Dock: ldap.bind_password, paperless_ngx.api_token.

**Paket 4** (T/J): a2a, github, here_now, homepage, netlify, remote_control, tailscale, vercel, virtual_computers, virtual_desktop.
Server-Draft: here_now_handlers.go:69, netlify_handlers.go:83, vercel_handlers.go:82, tailscale_handlers.go:27,
virtual_computers storage test (virtual_computers_handlers.go:399); homepage schon Draft; a2a/virtual_desktop = C.
Secrets → Dock: a2a.auth.api_key/bearer_secret, a2a remote per Index, github.token, here_now.api_key, homepage.deploy_password/deploy_key,
netlify.token, tailscale.api_key, tailscale.tsnet.auth_key, vercel.token, virtual_computers.s3_access_key_id/s3_secret_key/boring_token.
`sudo_password` bleibt Button (reiner Vault-Schlüssel), bekommt aber `cfgActionResult`. tailscale: native `confirm`/`alert` → `showConfirm`/`showAlert`.

**Paket 5** (J): chromecast, cyd, go2rtc, local_llm, newspaper, ollama, providers, realtime_speech, sip, treg, tts.
Server-Draft: providers-Test (ProviderEntry aus Body, Maske → gespeichert), Ollama-Modelle (URL aus Entwurf), treg_handlers.go:19 (Token aus Body, Maske → Vault);
schon Draft: MeshCentral, realtime_speech. C: chromecast, cyd, go2rtc, sip, Ollama-Recreate.
Secrets → Dock: treg.token, tts.<provider>.api_key (xai, openai, stepfun, resemble, elevenlabs, minimax, mistral).
`#sip-action-status` bleibt reiner Text (main.js liest ihn).

**Paket 6** (J): google_workspace, onedrive, sql_connections, auth, auth_webauthn, backup, indexing, mcp_server, prompts, rules, devices.
Server-Draft: google_workspace_handlers.go:10, onedrive_handlers.go:302, handlers_sql_connections.go:253 (Datensatz-Kopie aus Body, Maske → gespeichert).
Secrets → Dock: google_workspace.client_secret. Übrige: Ergebniszeilen statt Toast (U1/U5), Busy (U3).

**Paket Z** (seriell, zuletzt): Marker wieder schärfen; `TestConfigLegacyPatternAudit` verbietet `adg-test-result`, `adg-status-banner`,
`dc-test-result`, `*-test-result`-Klassen in `ui/cfg` (Allowlist für meshcore, ruview, s3, speech_lab, telephone_agent, security_proxy, server …);
tote CSS entfernen; Report + Memory aktualisieren.

Nicht in Welle 1: meshcore, ruview, s3 (schon gut), speech_lab-Fortschritt, reiche Statuspanels (telephone_agent, security_proxy, server, sandbox,
daemon_skills, indexing-Karten, updates, secrets, skill_manager, local_wikipedia), statische Hinweis-Banner.

## Ablauf und Prüfung

- 0a ∥ 0b im Worktree `cfg-wave1` (getrennte Verzeichnisse), dann Commit.
- Pakete 1–3 parallel, je eigener Worktree ab Fundament-Commit; danach 4–6. Je Paket ein kombiniertes Review (nur Critical/Important), dann Merge in `claude/cfg-wave1`.
- Pro Paket: `node --check` geänderter JS; `go test ./ui -run '(?i)(config|lang|translation|i18n|wave0|frontend_|<modul>)' -skip Browser`;
  Browser-Tests der Module mit `AURAGO_RUN_BROWSER_SMOKE=1`; Server: `go test ./internal/server -run '<Handler>'`.
- Bekannt vorher rot: `TestConfigChoiceSelectsKeepSavedValueWhenListsFailBrowser`, `TestConfigRefreshPopulatedBrowser`, `TestConfigRefreshRealSectionsBrowser`; `TestConfigRefreshSIPStepsBrowser` flackert unter Last.
