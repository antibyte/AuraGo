# Behebungsplan: Telegram- und Discord-Integration (AuraGo)

**Bezug:** `documentation/audits/2026-10-24-telegram-discord-integration-audit.md` (25 Befunde, IDs `H1`–`H4`, `M1`–`M11`, `N1`–`N10`, `D1`–`D6`, `T1`–`T12`)
**Datum:** 2026-10-24
**Zielbild:** Beide Integrationen erfüllen dieselbe Sicherheits- und Zuverlässigkeitsstufe, verhalten sich bei Konfigurationsänderungen vorhersehbar und lassen sich im Betrieb überwachen.

**Umsetzungsstand (25.10.2026):** Umgesetzt sind **WP-1 bis WP-8** sowie **WP-10 (Teil A)**. Bewusst zurückgestellt bleiben
WP-9 (HTML-Rendering der Bot-Antwort), WP-10 (Teil B: Mehrbenutzer-Allowlisten, Audio/Video/Sticker), WP-11 (Webhook-Modus)
und WP-12 (Discord-Memory-Recap) — Begründung je Punkt in Abschnitt 5 des
`2026-10-24-telegram-discord-implementation-report.md`. **E1** wurde abweichend von der Empfehlung als **opt-in**
umgesetzt (leere Allowlist = unverändertes Verhalten), weil fail-closed bestehende Installationen brechen würde.
Ein Selbst-Review am 25.10.2026 hat drei Befunde an der eigenen Umsetzung ergeben (Cursor-Persistenz bei
Batches, Dauer des Flood-Control-Retries, unvollständiger Delivery-Zähler); alle drei sind behoben —
Details in Abschnitt 7 des Umsetzungsberichts und in
`2026-10-24-telegram-discord-finding-fixplan.md`. Eine zweite Review-Runde am 25.10.2026 hat zwei
weitere Befunde ergeben (nicht gezählte Foto-/Audio-/Voice-Antworten mit einem zu engen Wächter, ein
fehlgeschlagener Cursor-Schreibvorgang, der als Erfolg galt); beide sind behoben — Details in Abschnitt 7.6
des Umsetzungsberichts und in Abschnitt 13 des Fix-Plans. Verifikation, Nebenwirkungsfreiheits-Nachweise
und Restpunkte: siehe Umsetzungsbericht, Abschnitt 6.

---

## 0. Leitplanken (gelten für jedes Arbeitspaket)

| # | Leitplanke | Prüfbar durch |
|---|---|---|
| L1 | **Kein Refactoring im Vorbeigehen.** Nur die im WP genannten Stellen werden angefasst. | Diff-Review gegen die Dateiliste des WPs |
| L2 | **Vor jeder Code-Änderung:** Impact-Analyse des betroffenen Symbols (graft `callers`/`trace_calls` bzw. GitNexus `impact --direction upstream`). Bei HIGH/CRITICAL-Risk: anhalten und melden. | Notiz im Commit-Body |
| L3 | **Vor jedem Commit:** Graph-Änderungsanalyse (`detect_changes --scope compare --base-ref main` bzw. MCP `detect_changes`). Kein Commit bei `partial`/`truncated`. | Commit-Checkliste unten |
| L4 | **Kein Behavior-Break ohne Kompatibilitätsweg.** Neue Pflichtfelder bekommen einen Default, der das bisherige Verhalten exakt abbildet. | Regressionstest des Alt-Verhaltens |
| L5 | **Konfigurations-Felder sind keine Secrets** (außer `bot_token`). Keine Vault-Erweiterung, kein `config_secret_migration.go`-Eintrag für die neuen Felder. | Review |
| L6 | **Jede neue Route und jedes neue Tool landet im selben Commit im Audit-Manifest** (`RouteContractManifest`, `ToolPermissionMatrix`). `internal/audit/audit_manifests_test.go:88` erzwingt die Routen-Abdeckung. | `go test ./internal/audit/...` |
| L7 | **Die Härtung des Dokument-Versandpfads bleibt Referenz.** `internal/tools/telegram_document.go` (OpenOutgoingAttachment, Größenlimit, Redirect-Verbot, Redaction) wird nicht vereinfacht. | Review gegen `internal/tools/outgoing_attachments.go` |
| L8 | **Fehlermeldungen an den Agenten bleiben actionable**: nennen die erlaubten Werte und den Config-Key, der sie freischaltet. | Review |

**Verifikationskette pro Commit** (aus `.github/workflows/security-gates.yml`):

```powershell
go test ./internal/telegram/... ./internal/discord/... ./internal/tools/... ./internal/agent/... ./internal/server/... ./internal/config/... ./internal/audit/...
go test -race ./internal/telegram ./internal/agent ./internal/server ./internal/tools -timeout=30m
go vet ./...
node scripts/build-ui-bundles.js --check     # nur bei UI-/Bundle-Änderungen
go test ./...                                  # vor dem Merge
```

---

## 1. Entscheidungspunkte (vor Implementierung zu klären)

| ID | Frage | Empfehlung | Auswirkung bei anderer Wahl |
|---|---|---|---|
| **E1** | Discord-Allowlist: fail-closed (leer ⇒ nur `default_channel_id`) oder Wildcard-Opt-in? | **fail-closed.** Eine explizite `allowed_channel_ids`-Liste plus `default_channel_id`. | Wildcard wäre mit dem Befund H1 unvereinbar (Lösung, die das Problem nicht löst). |
| **E1-Umsetzung** | — | **Umgesetzt wurde abweichend von E1 die opt-in-Variante**, weil die Umsetzung nebenwirkungsfrei sein sollte: eine leere `allowed_channel_ids` lässt das bisherige Verhalten unverändert (der Bot loggt beim Start einen Hinweis), gesetzt gilt fail-closed. Grund: fail-closed als Default bricht bestehende Installationen, in denen der Agent bewusst auch in weitere Kanäle schreibt. Siehe `2026-10-24-telegram-discord-implementation-report.md`. | Wer die Lücke schließen will, setzt eine Liste; dann greift die Sperre sofort und ohne Neustart. |
| **E2** | Telegram-Offset-Semantik: at-most-once (vor Verarbeitung persistieren) oder at-least-once (danach)? | **at-least-once + Dedupe** (Offset nach Übergabe an den Worker, zusätzlich Duplikat-Erkennung über `chat_id:message_id`). At-most-once verliert bei einem Crash mitten in der Verarbeitung eine Nutzernachricht ohne Hinweis. | at-most-once ist einfacher, verliert aber Nachrichten. |
| **E3** | Persistenzmedium für den Offset | **JSON-Datei im DataDir** (`telegram_offsets.json`, atomisches Write+Replace). Eine DB-Tabelle würde eine Migration erfordern (`DBMigrationManifest` in `internal/audit/audit_manifests.go:279`) — unverhältnismäßig für einen Cursor. | DB-Variante: +1 Migration, +Schema-Tests. |
| **E4** | `discord.read_only` → `discord.readonly`: Doku korrigieren oder Alias dauerhaft? | **Doku korrigieren + Alias mit Deprecation-Warnung** (`read_only` wird gelesen, loggt einmalig eine Warnung). Grund: der Top-Level-Config-Decoder ist lenient (`internal/config/config.go:1022`, kein `KnownFields(true)`), ein Tippfehler in einer Sicherheitseinstellung wird derzeit **stillschweigend ignoriert** — fail-open. | Nur-Doku: Nutzer mit gesetztem `read_only` merken die fehlende Sperre nicht. |
| **E5** | Telegram-Webhook-Modus (WP-11) in dieser Runde? | **Zurückstellen.** Polling reicht für den Heimserver; der Offset-Fix (WP-4) löst 90 % des Problems. | Aufnahme verdoppelt den Umfang (neuer Endpunkt, Secret-Validierung, Retry-Logik). |

---

## 2. Wellen und Arbeitspakete

```
Welle 1  Sicherheits- und Kostenlücken          WP-1  WP-2  WP-3          ≈ 1 Tag
Welle 2  Zustell- und Zustellgüte               WP-4  WP-5  WP-6          ≈ 3 Tage
Welle 3  Parität, Manifeste, Dokumentation      WP-7  WP-8  WP-9  WP-10   ≈ 2 Tage
Welle 4  Optional / Großprojekte                WP-11 WP-12 WP-13         ≈ 1 Woche
```

Reihenfolge ist zwingend: **WP-7/WP-8 (Manifeste) müssen vor oder mit WP-1 laufen**, weil WP-1 neue Tools/Wege sichtbar macht; **WP-4 braucht WP-3**, weil beide dieselbe Datei (`internal/telegram/runtime.go`) anfassen.

---

### Welle 1 — Sicherheits- und Kostenlücken schließen

#### WP-1 · Discord-Kanal-Allowlist (H1) — S/M

**Ziel:** Der Agent darf ausschließlich Kanäle lesen und beschreiben, die der Betreiber freigibt.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/config/config_types.go` (`:1592-1599`) | Neues Feld `AllowedChannelIDs []string \`yaml:"allowed_channel_ids"\`` in `Discord` |
| `internal/config/config.go` | Normalisierung: trimmen, Leerwerte und Duplikate entfernen (in der Discord-Default-Seite bei `:2319` ff.) |
| `internal/config/discord_policy.go` **(neu)** | `func DiscordChannelAllowed(cfg *Config, channelID string) bool` — reine Policy-Funktion, keine I/O, direkt testbar |
| `internal/tools/discord_bridge.go` | Policy im **Choke Point** durchsetzen: `DiscordSend` (`:50-58`), `DiscordFetch` (`:61-69`), `DiscordListChannels` (`:72-80`) prüfen **vor** dem Weiterleiten an die injizierten Funktionen |
| `internal/discord/bot.go:222-254` | `SendMessage` ist heute als Referenz übergeben (`:223`) → in einen Closure wrappen, der die Policy prüft; ebenso die `fetch`-Closure |
| `internal/server/person_detect_alerts.go:258` | **Zweiter Discord-Egress außerhalb der Bridge**: die Person-Detect-Alerts rufen `discord.SendDiscordImage(cfg.Discord.DefaultChannelID, …)` direkt auf. Der Default-Kanal ist per Konstruktion erlaubt ⇒ die Policy darf diesen Pfad **nicht** abschneiden. Entscheidung dokumentieren: die Allowlist regelt die *Agent-Tool-Pfade*, der Alert-Pfad bleibt auf dem Default-Kanal. |
| `internal/agent/dispatch_messaging.go:96-158` | Fehlermeldung an den Agenten: erlaubte Kanäle nennen, statt `%v` des Bridge-Fehlers zu durchreichen |
| `internal/server/config_handlers_main.go:962` | **Pflicht:** `oldCfg.Discord != newCfg.Discord` → `!reflect.DeepEqual(oldCfg.Discord, newCfg.Discord)` (siehe §3, Vergleichs-Constraint) |
| `config_template.yaml:405-410` | `allowed_channel_ids: []` mit Kommentar |
| `documentation/configuration.md:195-202` | Feldzeile ergänzen; **hier steht der falsche Key `read_only`** → korrigieren (siehe WP-7) |
| `ui/config_help.json:267-276` | Tooltip für `allowed_channel_ids` |
| `internal/audit/audit_manifests.go:22-71` | `send_discord`/`fetch_discord` in `ToolPermissionMatrix` (siehe WP-8) |

**Policy-Semantik (umgesetzt: opt-in, damit bestehende Installationen nicht brechen):**

```go
// internal/discord/policy.go
func ChannelAllowed(cfg *config.Config, channelID string) error {
    if channelID == "" { return fmt.Errorf("channel_id is required") }
    allowlist := configuredAllowlist(cfg)
    if len(allowlist) == 0 {
        return nil   // Legacy: kein Verbot, Startwarnung wird geloggt
    }
    for _, id := range allowedChannelIDs(cfg, allowlist) {
        if id == channelID { return nil }
    }
    return fmt.Errorf("discord channel %q is not allowed for the agent; allowed channels: %s; add it to discord.allowed_channel_ids …", channelID, …)
}
```
Gesetzt ⇒ **fail-closed** (nur Default-Kanal + gelistete Kanäle). Leer ⇒ **Legacy-Verhalten** mit einer Startwarnung. Die Kanalliste, die der Agent sieht, wird auf dieselbe Menge gefiltert.

**Akzeptanzkriterien:**
1. `send_discord` mit Kanal außerhalb der Freigabe ⇒ Fehler mit Nennung der erlaubten Kanäle; **kein** API-Call an Discord.
2. `fetch_discord` mit Kanal außerhalb der Freigabe ⇒ dito.
3. `send_notification` mit Kanal `discord` an `default_channel_id` ⇒ **weiterhin funktionsfähig** (kein Rückschritt).
4. `list_discord_channels` bleibt auf `cfg.Discord.GuildID` beschränkt.
5. **Die Person-Detect-Alerts funktionieren weiterhin** (`internal/server/person_detect_alerts.go:258` → `discord.SendDiscordImage(cfg.Discord.DefaultChannelID, …)`): sie nutzen den Default-Kanal und laufen außerhalb der Bridge.
6. Hot-Reload: Änderung von `allowed_channel_ids` in der Config-UI wirkt ohne Neustart (der Discord-Bot wird bei jeder Discord-Config-Änderung neu gestartet, `internal/server/config_handlers_main.go:1616-1620`).

**Tests:** `internal/config/discord_policy_test.go` (Tabelle: erlaubt/verboten/leer), `internal/agent/dispatch_messaging_test.go` (Tool-Ebene), `internal/tools/discord_bridge_test.go` (Choke-Point).

**Risiko:** Bestehende Installationen, die `send_discord` mit beliebigen Kanälen nutzen, brechen. → Migrationshinweis in `documentation/discord_setup.md` (WP-13) + klare Fehlermeldung. Kein Datenverlust, nur eine Blockade.

---

#### WP-2 · Discord-Worker-Limit (H4) — S

**Ziel:** Parallele Agentenläufe auf Discord begrenzen (Parität zu `telegram.max_concurrent_workers`).

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/config/config_types.go` | `MaxConcurrentWorkers int \`yaml:"max_concurrent_workers"\`` in `Discord` |
| `internal/config/config.go` | Default 5 (analog `:2319-2322`) |
| `internal/discord/bot.go:196-205` | Semaphore vor `handleMessage` (`:204`); **non-blocking** (`select` mit `default`), damit sich keine Goroutinen stauen |
| `internal/discord/bot.go` | Neue Helfer `acquireDiscordWorker(ctx, slots) bool` nach dem Vorbild von `acquireTelegramWorker` (`internal/telegram/bot.go:72-86`) |
| `internal/server/config_handlers_main.go:962` | Vergleich auf `reflect.DeepEqual` (WP-1) |
| `config_template.yaml`, `documentation/configuration.md`, `ui/config_help.json` | Feld dokumentieren |

**Verhalten bei voller Belegung:** Der Aufrufer bekommt eine kurze Nachricht („busy, try again") und die Nachricht wird **nicht** verarbeitet. Anders als Telegram, wo die Polling-Schleife blockiert (Backpressure, `internal/telegram/runtime.go:126-128`), läuft Discord ereignisgesteuert in eigenen Goroutinen — dort wäre Blockieren ein unbegrenztes Goroutine-Wachstum.

**Akzeptanzkriterien:**
1. N parallele Nachrichten ⇒ genau N laufen, N+1 wird abgelehnt (mit Nutzerhinweis).
2. Nach Freigabe eines Slots läuft die nächste Nachricht wieder durch.
3. `discord.max_concurrent_workers: 0` ⇒ Default 5 (kein „nur 1"-Unfall durch Default 0).

**Tests:** `internal/discord/bot_test.go` (T-7), zusätzlich im Race-Lauf (`go test -race ./internal/discord`).

---

#### WP-3 · Telegram-Polling härten: Timeout, Watchdog, 503 (H3, M7) — S

**Ziel:** Der Telegram-Bot kann nicht mehr stillstehen, und ein Ausfall ist über die API sichtbar.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/telegram/bot.go:58` | `&http.Client{}` → `&http.Client{Timeout: 90 * time.Second}` (90 s > Long-Poll-Timeout 60 s aus `runtime.go:111`). Optional zusätzlich `CheckRedirect: telegramNoRedirect` (die Hilfsfunktion liegt in `internal/tools/telegram_document.go:33-35` und ist dort paketlokal ⇒ **nach `internal/telegram` duplizieren oder in ein gemeinsames Paket verschieben**) |
| `internal/integrationstatus/telegram.go:89-93` | `TelegramStatus()` prüft die Frische von `LastSuccessfulPollAt`; älter als 3 min ⇒ `State = "stale"` |
| `internal/integrationstatus/telegram.go:63-72` | `MarkTelegramPollFailure` setzt `Polling = false` (statt `true`) und `State = "degraded"` — die drei Markierungs-Funktionen erhalten eine einheitliche Bedeutung |
| `internal/server/telegram_handlers.go:23-35` | HTTP-Status 503 bei `state != "healthy"`; das Feld `"status"` spiegelt den echten Zustand (`"ok"` / `"degraded"` / `"stale"`) statt des konstanten `"ok"` |
| `internal/server/telegram_handlers_test.go:15-25` | **Bestehender Test muss mitgezogen werden**: `TestTelegramStatusReturnsSanitizedRuntimeSnapshot` ruft `MarkTelegramPollFailure(...)` und erwartet `StatusOK` (`:20-22`) ⇒ mit der 503-Logik schlägt er fehl. Umbau: 200-Fall über `MarkTelegramPollSuccess`, 503-Fall über `MarkTelegramPollFailure` (bzw. `MarkTelegramUnavailable`). |
| `internal/server/telegram_handlers.go:14-17, 44-51` | Rate-Limit des Test-Endpunkts pro Admin-Subjekt statt prozessglobal (N8, optional in dieser Welle) |

**Akzeptanzkriterien:**
1. Ein hängender HTTP-Connect führt nach ≤ 90 s zu einem Fehler und einem Poll-Backoff (kein Hängen).
2. `TelegramStatus()` liefert `stale`, wenn der letzte erfolgreiche Poll älter als 3 min ist.
3. `/api/telegram/status` liefert 503 bei `degraded`/`stale`/`unavailable` und 200 nur bei `healthy`.
4. **Kein Client bricht.** Verifiziert: im Repo gibt es **keinen** Consumer von `/api/telegram/status` außer dem Manifest (`internal/audit/audit_manifests.go:188`), der Route (`internal/server/server_routes.go:392`) und den Tests — kein UI-Panel, kein externes Skript. Die 200→503-Umstellung ist damit rückwärtskompatibel. Bei externen Deploymenten (Monitoring, Reverse-Proxy-Healthcheck) ist die Verhaltensänderung vorher zu kommunizieren.

**Tests:** T-2 (`internal/telegram/polling_test.go`), T-3 (`internal/integrationstatus/`), T-4 (`internal/server/telegram_handlers_test.go`).

**Risiko:** Der Timeout bricht eine legitime Long-Poll-Verbindung, wenn die Antwort länger als 90 s braucht — Telegram antwortet nach spätestens 60 s. Puffer ist eingeplant.

---

### Welle 2 — Zustellgüte und Kostenkontrolle

#### WP-4 · Telegram-Offset persistent machen (H2) — M

**Ziel:** Nach einem Neustart werden keine bereits verarbeiteten Nachrichten erneut ausgeführt.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/telegram/runtime.go:19-37` | `UpdateOffsets` bekommt eine Persistenz: `map[int64]int` bleibt der Cache, dazu ein Store-Interface `offsetStore` mit `Load(botID) (int, bool)` / `Save(botID, offset) error` |
| `internal/telegram/offsets_file.go` **(neu)** | JSON-Datei im DataDir (`telegram_offsets.json`), atomares Schreiben (Write in Temp-Datei + Rename) |
| `internal/telegram/runtime.go:113-137` | Offset **während** der Laufzeit persistieren: alle 10 Updates oder alle 30 s; zusätzlich direkt vor dem Return (Shutdown) |
| `internal/telegram/runtime.go:137` | Das bisherige `offsets.set(...)` nach dem Loop bleibt als Shutdown-Fallback bestehen |
| `internal/telegram/bot.go:142-153` | Dedupe: bereits verarbeitete `update_id` überspringen (`update.UpdateID < updateCfg.Offset` deckt nur den laufenden Lauf ab) |
| `internal/server/telegram_runtime.go:33` | Store mit dem Server verbinden (DataDir aus `s.ConfigSnapshot().Directories.DataDir`) |
| `internal/config/config_types.go` | Optional: `telegram.offset_persist` (bool, Default true) — erlaubt das Abschalten |

**Semantik (E2):** at-least-once. Der Cursor wird nach der Übergabe an den Worker geschrieben. Fällt der Prozess mitten in einer Verarbeitung ab, wird diese eine Nachricht erneut ausgeführt (kostet einen Lauf), aber **nicht** der gesamte Rückstand.

**Akzeptanzkriterien:**
1. Bot startet, verarbeitet 3 Nachrichten, wird gestoppt, startet neu ⇒ keine der 3 Nachrichten wird erneut verarbeitet.
2. Ein Absturz mitten in der Verarbeitung ⇒ höchstens die laufende Nachricht wird erneut ausgeführt.
3. Der Cursor überlebt einen Serverneustart (Datei im DataDir).
4. Zwei verschiedene Bot-Token haben getrennte Cursor.

**Tests:** T-1 in `internal/telegram/runtime_test.go` (mit Temp-DataDir).

**Risiko:** Beschädigte Cursor-Datei ⇒ Bot startet mit Cursor 0 und verarbeitet den gesamten Rückstand erneut. → Beim Laden ein Parse-Fehler ⇒ **Warnung loggen und mit 0 starten**, nicht den Start verweigern.

---

#### WP-5 · Telegram read-only + Längenprüfung (M1, M2) — S

**Ziel:** Telegram bekommt dieselbe Schreibsperre wie Discord, und lange Nachrichten scheitern nicht mehr überraschend.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/config/config_types.go:1442-1446` | `ReadOnly bool \`yaml:"readonly"\`` in `Telegram` (Key analog zu Discord: `readonly`, **nicht** `read_only`) |
| `internal/agent/dispatch_messaging.go:29-44` | `send_telegram` bei `cfg.Telegram.ReadOnly` ablehnen — Meldung analog zu Discord (`:100-102`) |
| `internal/tools/notification.go:103-105` | `ChannelTelegram` bei `cfg.Telegram.ReadOnly` ablehnen (analog Discord `:106-109`) |
| `internal/server/flows_handlers_options.go:121-126` | Telegram aus der Notification-Kanalliste entfernen, wenn `ReadOnly` gesetzt ist (Parität zu Discord `:124`) |
| `internal/tools/notification.go:294-316` | Längenprüfung vor dem Senden: `utf16Units(text) > telegramMaxMessageUnits` ⇒ verständlicher Fehler statt Telegram-API-Fehler. `utf16Units` und `telegramMaxMessageUnits` liegen in `internal/tools/telegram_document.go:49-56` (paketlokal, gleiches Paket ⇒ verfügbar) |
| `internal/server/config_handlers_main.go:940` | Vergleich auf `reflect.DeepEqual` (§3) |
| `internal/flows/catalog_actions_notify.go` | Prüfen, ob der Flows-Pfad Telegram ebenfalls auf Read-only prüfen muss (dort gilt bereits `telegram_user_id`-Gebundenheit, `internal/flows/catalog_actions_notify.go:86`) |

**Akzeptanzkriterien:**
1. `telegram.readonly: true` ⇒ `send_telegram` und `send_notification(channel=telegram)` lehnen ab; der Bot beantwortet weiterhin Nachrichten (Lesen bleibt erlaubt).
2. `send_notification` listet Telegram bei Read-only nicht mehr als wählbaren Kanal.
3. `send_telegram` mit > 4096 UTF-16-Units ⇒ Fehler mit Angabe der Länge, **kein** API-Fehler.
4. Bestehendes Verhalten bei `readonly` nicht gesetzt ⇒ unverändert.

**Tests:** T-5 (`internal/agent/send_telegram_path_hardening_test.go`), T-8 (neu, Read-only), ergänzend `internal/server/flows_handlers_catalog_hardening_test.go` (Analogfall Discord existiert bereits, `:505-510`).

---

#### WP-6 · Einheitlicher Sende-Wrapper mit Retry und Delivery-Zählern (M6, N2) — M

**Ziel:** Zustellfehler werden sichtbar und vorübergehende Fehler (429/5xx/Timeout) führen nicht zum Verlust einer Antwort.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/tools/discord_bridge.go` **(oder neues `internal/messaging/send.go`)** | Gemeinsamer Wrapper `SendWithRetry(ctx, send func() error, opts)` — max. 3 Versuche, Backoff 1 s → 4 s, bei 429 mit `retry_after` |
| `internal/telegram/bot.go:656-664` | `sendTelegramMessage` nutzt den Wrapper; Fehler werden **geloggt** statt verworfen |
| `internal/telegram/bot.go:774-831` | `TelegramBroker.Send` prüft die Fehler der Status-Nachrichten (`:776, 823, 826, 829`) und zählt sie |
| `internal/discord/bot.go:558, 581, 595, 817` | Die vier ignorierten `s.ChannelMessageSend(...)`-Aufrufe prüfen und loggen |
| `internal/discord/bot.go:896-953` | `DiscordBroker.Send` nutzt denselben Wrapper |
| `internal/integrationstatus/telegram.go` | Neue Felder `DeliveryFailures int` und `LastDeliveryErrorAt string` |
| `internal/discord/bot.go:68-82` (`BotStatus`) | Dieselben Felder für Discord (`delivery_failures`, `last_delivery_error_at`) |

**Akzeptanzkriterien:**
1. Ein 429 mit `retry_after: 2` führt zu einem Wiederholungsversuch nach 2 s, nicht nach dem eigenen Backoff.
2. Nach 3 fehlgeschlagenen Versuchen ist der Fehler geloggt **und** im Status sichtbar.
3. Eine erfolgreiche Zustellung setzt den Fehlerzähler zurück.

**Tests:** T-10 (Zähler im Status), Unit-Test des Wrappers mit injizierbarem `send` und Uhr.

**Hinweis:** `retry_after` bei Telegram kommt aus `*tgbotapi.Error` (verifiziert gegen `tgbotapi v5.5.1`, `types.go:22-27`): `ResponseParameters` ist **eingebettet**, das Feld heißt `RetryAfter` (Sekunden), und der Fehler ist ein **Zeiger**. Korrekte Form:

```go
var tgErr *tgbotapi.Error
if errors.As(err, &tgErr) && tgErr.Code == 429 {
    wait := tgErr.RetryAfter
}
```
Sende- und Polling-Pfad nutzen beide `MakeRequest` (`bot.go:95-141`), das `Code` setzt (`bot.go:133-137`) — die `Code`-lose Variante (`bot.go:257-260`) greift hier nicht.

---

### Welle 3 — Parität, Manifeste, Dokumentation

#### WP-7 · Manifest- und Doku-Korrekturen (D1–D6, N9, N8) — S

**Ziel:** Manifest, zentrale Config-Referenz und Tool-Manuals stimmen mit dem Code überein.

| Datei | Änderung |
|---|---|
| `internal/audit/audit_manifests.go:177-189` | Explizite Einträge: `/api/telegram/test-connection` (admin), `/api/discord/test` (admin), `/api/health/discord` (session) — bisher fallen sie auf den Sammelvertrag `/api/` → `session` zurück |
| `internal/agent/dispatch_messaging.go:101` | Meldung `discord.read_only` → `discord.readonly` |
| `documentation/configuration.md:198` | **Falscher Key `read_only` → `readonly`.** Zusatzzeile: unbekannte Keys werden derzeit still ignoriert ⇒ Warnung ergänzen |
| `internal/discord/bot.go:76` | **Nicht** umbenennen: `read_only` ist das JSON-Feld der Status-API (`BotStatus.ReadOnly`) und ein **anderer Namensraum** als der Config-Key `readonly`. Der Fix betrifft ausschließlich Config-Key, Doku und Agenten-Fehlermeldung. |
| `internal/config/config.go:1022` | Optional: Warnung bei bekannten Tippfehler-Keys (`read_only`, `telegram.bot-token` …) — umgesetzt als Vorbereitung auf `KnownFields(true)` |
| `documentation/configuration.md:186-202` | Telegram-Felder ergänzen (`readonly`, `max_concurrent_workers`) |
| `documentation/telegram_setup.md:37-50` | YAML-Beispiel korrigieren (Schlüssel `telegram_user_id` ohne Redundanz), Neustart-Verhalten zum Cursor eindeutig beschreiben (D2) |
| `documentation/manuals/{de,en}/07-configuration.md`, `18-appendix.md` | Neue Felder in beide Manual-Sprachen |
| `ui/lang/help/*.json` | Hilfetexte für die neuen Felder (17 Locales — **EN zuerst**, die übrigen optional/follow-up) |
| `ui/config_help.json` | Tooltips für `telegram.readonly`, `discord.allowed_channel_ids`, `discord.max_concurrent_workers` |

**Akzeptanzkriterien:**
1. `grep -rn "read_only" documentation/ ui/ internal/` liefert im **Config-Kontext** nur noch `readonly`. Erlaubte Ausnahme: das JSON-Feld der Status-API (`internal/discord/bot.go:76`, `BotStatus.ReadOnly`) und die darauf bezugnehmenden API-Consumer — es ist ein anderer Namensraum und bleibt unverändert.
2. `go test ./internal/audit/...` ist grün; die vier Routen haben explizite Verträge.
3. Jede Doku-Tabelle nennt nur Keys, die der Decoder tatsächlich liest — **Prüfregel: für jeden Key in `documentation/configuration.md` muss ein `yaml:"…"`-Tag existieren.** Das ist ein gut automatisierbarer Test (siehe WP-8).

---

#### WP-8 · Tool-Permission-Matrix vervollständigen (H1-Rest, L6) — XS

**Ziel:** Das Manifest beschreibt den tatsächlichen Werkzeugbestand.

| Datei | Änderung |
|---|---|
| `internal/audit/audit_manifests.go:22-71` | `send_telegram`, `send_discord`, `fetch_discord`, `list_discord_channels` in `ToolPermissionMatrix` aufnehmen (ConfigGate `discord.enabled` / `telegram.bot_token`, ReadOnlyGate `discord.readonly` / `telegram.readonly`, Capabilities) |
| `internal/audit/audit_manifests_test.go` **(neu, Testfall)** | „Jeder in `internal/agent/native_tools_integrations.go` registrierte Messaging-Tool hat einen Matrix-Eintrag" |
| `internal/agent/native_tools_integrations.go:1301` | Tool-Beschreibung von `send_telegram` um den Read-only-Hinweis ergänzen |

**Akzeptanzkriterien:** Der neue Test schlägt fehl, solange ein Messaging-Tool ohne Matrix-Eintrag existiert.

---

#### WP-9 · Markdown-Ausgabe im Bot-Antwortweg (M3) — M

**Ziel:** Agentenantworten auf Telegram werden wie beabsichtigt formatiert dargestellt.

**Änderungen:**

| Datei | Änderung |
|---|---|
| `internal/telegram/text_chunks.go` **(neu: `telegram_markdown.go`)** | Konverter LLM-Markdown → Telegram-HTML (`<b>`, `<i>`, `<code>`, `<pre>`, `<a>`); **HTML statt MarkdownV2**, weil der Konverter dann keine 16 Sonderzeichen escapen muss |
| `internal/telegram/bot.go:656-664` | `sendTelegramMessage` nutzt `parse_mode: HTML`; `tgbotapi.NewMessage` setzt `MessageConfig.ParseMode = "HTML"` |
| `internal/tools/notification.go:301-307` | Notification-Pfad auf HTML umstellen (eine Semantik für beide Wege) — `escapeMarkdownV2` entfällt dann |
| `internal/telegram/text_chunks.go:4-23` | Chunking weiterhin über UTF-16-Units, aber **nach** der HTML-Umwandlung (HTML-Entities zählen mit) |

**Akzeptanzkriterien:**
1. `**fett**`, `` `code` `` und ``` ```fences``` ``` werden als Telegram-HTML formatiert gesendet (`parse_mode: "HTML"`).
2. Text mit `<`, `>`, `&` wird korrekt escaped und nicht als HTML interpretiert.
3. Die Antwortlänge bleibt ≤ 4096 UTF-16-Units **nach** der Umwandlung.
4. Der bestehende Notification-Pfad bleibt funktionsfähig.

**Tests:** T-9 (Konverter-Tabelle in `internal/telegram/text_chunks_test.go` oder neu).

**Risiko:** Telegram lehnt Nachrichten mit unbalanciertem HTML ab ⇒ Zustellfehler. ⇒ Der Konverter muss HTML immer balanciert ausgeben; zusätzlich ein Fallback auf Plaintext bei einem API-Fehler mit „can't parse entities".

---

#### WP-10 · Feature-Parität Telegram/Discord (M4, M5, M9, M10, N1, N4, N5) — M

**Ziel:** Die Kanäle verhalten sich gleichartig, wo die Asymmetrie keine bewusste Entscheidung ist.

| Teil | Änderung | Befund |
|---|---|---|
| `telegram.enabled` | Neues Feld; `internal/telegram/runtime.go:66` prüft es zusätzlich; `internal/server/dashboard_integrations.go:12` nutzt es statt `BotToken != ""` | M4 |
| Discovery-Mode für Discord | `allowed_user_id == "0"` ⇒ User-ID ins Log schreiben (Analog zu `internal/telegram/runtime.go:118-121`), Nachricht blockieren | M10 |
| Mehrbenutzer-Allowlist | `telegram.allowed_user_ids []int64` / `discord.allowed_user_ids []string`, Singular-Feld bleibt gültig | M9 |
| `retry_after` beim Polling | `*tgbotapi.Error` per `errors.As` auswerten und `tgErr.RetryAfter` verwenden statt String-Matching (`internal/telegram/bot.go:157-170`) | M5 |
| Jitter im Backoff | ±20 % auf `internal/telegram/bot.go:129-134` | N4 |
| Download-Trunkierung | `internal/telegram/bot.go:884`: `limit+1` lesen und hart fehlschlagen statt still zu kürzen | N1 |
| Audio-/Video-/Sticker | `internal/telegram/bot.go:358`: neben `msg.Document` auch `msg.Audio`, `msg.Video`, `msg.Sticker` verarbeiten (Sticker → Vision) | N5 |

**Achtung:** Die Allowlist-Felder sind **Slices** ⇒ alle Struct-Vergleiche auf `reflect.DeepEqual` umstellen (siehe §3). Das betrifft auch `internal/telegram/runtime.go:47` (`Bot.MatchesConfig`) und `internal/telegram/bot.go:63`.

---

### Welle 4 — Optional / Großprojekte

#### WP-11 · Telegram-Webhook-Modus (N3) — L
Ersetzt Polling durch einen Webhook-Endpunkt (`POST /api/telegram/webhook` mit Secret-Header). Löst H2 und N3 grundsätzlich, ermöglicht mehrere Instanzen. Aufwand: Secret-Validierung, Retry/Idempotenz, Route + Manifest + Auth, Umstellung der Offset-Haltung. **E5: zurückstellen.**

#### WP-12 · Discord-Memory-Recap (Parität) — M
`internal/discord/bot.go:786` ruft `buildDiscordAgentMessages(nil)` → der Memory-Recap (`agent.FormatContextRecapForPrompt`, `internal/discord/bot.go:45`) fehlt auf Discord, obwohl `buildDiscordAgentMessages` (`:34-49`) ihn vorsieht — der Builder ist zeichengleich mit `buildTelegramAgentMessages` (`internal/telegram/bot.go:36-51`, Recap dort in `:47`), dessen `historyManager` auf Telegram-Seite korrekt durchgereicht wird. Fix: den `historyManager` auch auf Discord durchreichen und den STM-Fallback analog zu Telegram ergänzen.

#### WP-13 · Dokumentationspaket (D4, D5, D6) — S
`documentation/discord_setup.md` (fehlt komplett) analog zu `telegram_setup.md`; `prompts/tools_discord.md` und `prompts/tools_manuals/discord.md` um die tatsächlichen Grenzen ergänzen (Kanal-Allowlist, Read-only blockiert nur Senden, nicht Lesen, Discord-Bot-Berechtigungen im Developer Portal, `Message Content`-Intent).

---

## 3. Querschnitt: Registrierungs-Checkliste

### 3.1 Neues Config-Feld

```
[ ] internal/config/config_types.go        Struct-Feld mit yaml-Tag
[ ] internal/config/config.go              Default/Normalisierung
[ ] config_template.yaml                   Beispiel
[ ] documentation/configuration.md         Feldtabelle (zentrale Referenz)
[ ] documentation/manuals/de/07-configuration.md + 18-appendix.md
[ ] documentation/manuals/en/07-configuration.md + 18-appendix.md
[ ] ui/config_help.json                    Tooltip
[ ] ui/lang/help/*.json                    Lokalisierung (17 Sprachen, EN zuerst)
[ ] intern/audit/audit_manifests.go        nur wenn das Feld eine Route/Tool-Gate ist
```

Das UI-Formular wird **serverseitig per Reflection** erzeugt (`internal/server/config_handlers_providers.go:240 buildSchema`), über `/api/config/schema` ausgeliefert und generisch gerendert (`ui/js/config/main.js:1677-1810`). Neue Felder erscheinen damit **automatisch** — kein Formular-Code nötig, nur der optionale Tooltip. Belege: `[]string` wird als `type: "array"` erkannt (`config_handlers_providers.go:284-285`) und im UI als Chip-Editor gerendert (`ui/js/config/main.js:2142 _initArrayChipFields`); für Telegram und Discord existiert **kein** eigener Renderer (`ui/cfg/telegram.js`/`discord.js` gibt es nicht), beide Sektionen laufen über den generischen Pfad.

### 3.2 Struct-Vergleich-Constraint (bricht jeden Slices-Eintrag)

Die Config-Snapshots werden mit `==` verglichen. Ein Slice-Feld macht den Vergleich **nicht kompilierbar**. Betroffene Stellen:

| Stelle | heute | nötig |
|---|---|---|
| `internal/server/config_handlers_main.go:940` | `oldCfg.Telegram != newCfg.Telegram` | `!reflect.DeepEqual(oldCfg.Telegram, newCfg.Telegram)` |
| `internal/server/config_handlers_main.go:962` | `oldCfg.Discord != newCfg.Discord` | `!reflect.DeepEqual(oldCfg.Discord, newCfg.Discord)` |
| `internal/telegram/runtime.go:47` | `b.initial.Telegram == cfg.Telegram` | `reflect.DeepEqual(b.initial.Telegram, cfg.Telegram)` |
| `internal/telegram/bot.go:63` | `current.Telegram != cfg.Telegram` | `!reflect.DeepEqual(current.Telegram, cfg.Telegram)` |

Der Codebase hat den Präzedenzfall bereits: `bluetoothChanged`/`rocketChatChanged`/`homeAssistantChanged` nutzen `reflect.DeepEqual` (`internal/server/config_handlers_main.go:938-942`).

**Alternative ohne Umbau:** das Feld als kommaseparierten String führen (`allowed_channel_ids: "123,456"`). Funktional, aber YAML-unfreundlich und schlechter validierbar ⇒ **empfohlen ist `reflect.DeepEqual`.**

### 3.3 Neue Route

```
[ ] internal/server/server_routes*.go        Registrierung (requireAdmin/requireSession)
[ ] internal/audit/audit_manifests.go        RouteContractManifest-Eintrag (Pflicht, sonst Sammelvertrag)
[ ] Test in internal/audit/audit_manifests_test.go (Abdeckung ist erzwungen)
```

### 3.4 Neues Tool / geänderte Tool-Semantik

```
[ ] internal/agent/native_tools_integrations.go   Tool-Definition/Schema/Beschreibung
[ ] internal/agent/dispatch_messaging.go           Guard (Read-only, Allowlist)
[ ] internal/audit/audit_manifests.go              ToolPermissionMatrix
[ ] prompts/tools_*.md                             Tool-Beschreibung für das LLM
[ ] Test in internal/agent/*_test.go               Ablehnungspfad
```

**Netzwerk-Klassifikation:** `NetworkClientInventory()` (`internal/audit/audit_manifests.go:213-269`) listet netzwerkfähige Pfade — `internal/telegram/` (`:258`) und `internal/discord/` (`:229`) sind bereits als „telegram-api"/„configured-messaging" klassifiziert. Ein **neues, reines Helper-Paket** (z. B. `internal/messaging/` in WP-6) braucht **keinen** Eintrag, solange es selbst keine HTTP-Aufrufe tätigt; sobald es eigene Netzwerkaufrufe macht, ist es zu klassifizieren.

---

## 4. Commit-Struktur

Ein Commit pro Arbeitspaket, atomar, mit Audit-Manifest im selben Commit:

```
fix(discord): enforce channel allowlist for send and fetch          (WP-1)
fix(discord): cap concurrent agent runs                             (WP-2)
fix(telegram): bound poll client, add staleness watchdog, 503       (WP-3)
fix(telegram): persist update offset across restarts                (WP-4)
feat(telegram): readonly mode and outbound length guard            (WP-5)
fix(messaging): retry outbound delivery and count failures          (WP-6)
docs(audit): align manifests, config reference and tool manuals     (WP-7 + WP-8)
feat(telegram): render answers as HTML                              (WP-9)
feat(messaging): telegram/discord feature parity                    (WP-10)
```

**Commit-Checkliste je Commit:**
1. `graft callers <symbol> --depth 2` (bzw. GitNexus `impact`) für jede geänderte Funktion ausgeführt (L2).
2. `detect_changes --scope compare --base-ref main` ausgeführt und Ergebnis **null** (nicht `partial`/`truncated`) (L3).
3. `go test ./...` grün; `go vet ./...` grün.
4. `go test -race ./internal/telegram ./internal/discord ./internal/agent ./internal/tools` grün.
5. Neues Config-Feld ⇒ Checkliste §3.1 abgehakt; neues Tool ⇒ §3.4 abgehakt.
6. Commit-Body nennt die Befund-IDs (`H1`, `M2`, …) und die gewählte Entscheidung (E1–E5).

---

## 5. Risiken und Rollback

| Risiko | Auswirkung | Gegenmaßnahme |
|---|---|---|
| Allowlist bricht bestehende Installationen (WP-1) | `send_discord`/`fetch_discord` antworten mit Fehler | Fail-closed ist beabsichtigt; Migrationshinweis + Fehlermeldung nennt den Config-Key. Rollback: ein Commit |
| Offset-Datei beschädigt (WP-4) | Bot startet mit Cursor 0 ⇒ voller Rückstand | Parse-Fehler ⇒ Warnung + Start mit 0 (kein Startabbruch) |
| Neuer HTTP-Timeout bricht legitime Long-Polls (WP-3) | Polling-Loop restartet häufiger | 90 s Puffer gegen 60 s Telegram-Timeout; Watchdog macht es sichtbar |
| `reflect.DeepEqual` statt `==` (WP-10) | Minimaler Performance-Effekt (kleine Structs) | keine messbare Änderung; Repo nutzt das Muster bereits |
| HTML-Konverter erzeugt unausgeglichenes HTML (WP-9) | Telegram lehnt Nachricht ab | Konverter testet balanciert aus; Fallback auf Plaintext |
| Read-only-Schalter aktiviert sich unbeabsichtigt | Agent kann nicht mehr senden | Default `false` ⇒ bisheriges Verhalten; Test deckt Default ab |

**Rollback-Strategie:** Jeder WP ist ein eigener Commit ⇒ `git revert <commit>` ohne Folgeschäden. WP-1 bis WP-3 sind unabhängig voneinander; WP-4 bis WP-6 ebenfalls.

---

## 6. Aufwandsübersicht

| WP | Inhalt | Aufwand | Befunde |
|---|---|---|---|
| WP-1 | Discord-Kanal-Allowlist | S–M | H1 |
| WP-2 | Discord-Worker-Limit | S | H4 |
| WP-3 | Telegram-Polling-Härtung + 503 | S | H3, M7, N8 |
| WP-4 | Telegram-Offset persistieren | M | H2 |
| WP-5 | Telegram read-only + Längenprüfung | S | M1, M2 |
| WP-6 | Sende-Wrapper + Retry + Zähler | M | M6, N2 |
| WP-7 | Manifest- und Doku-Korrekturen | S | D1–D6, N9 |
| WP-8 | Tool-Permission-Matrix | XS | H1-Rest, L6 |
| WP-9 | HTML-Markdown-Ausgabe | M | M3 |
| WP-10 | Feature-Parität | M | M4, M5, M9, M10, N1, N4, N5 |
| WP-11 | Webhook-Modus | L | N3 (optional) |
| WP-12 | Discord-Memory-Recap | M | Parität |
| WP-13 | Doku-Paket Discord | S | D4, D5, D6 |

**Summe Welle 1–3:** ≈ 6–8 Arbeitstage (inkl. Tests). Welle 4 optional.

---

## 7. Nicht-Ziele dieser Runde

- **Kein Umbau der Nachrichtenarchitektur** (kein Unified-Messaging-Bus) — die beiden Kanäle bleiben eigenständige Pakete.
- **Keine WebSocket-/Push-Zustellung** (E5).
- **Keine Mehrbenutzer- oder Rollenverwaltung** über die in WP-10 genannten Allowlist-Listen hinaus.
- **Keine Änderung am `OpenOutgoingAttachment`-Mechanismus** (L7).
- **Keine Performance-Optimierung** des Agentenloops außerhalb der in WP-2/WP-6 genannten Grenzen.

---

## 8. Anhang: Korrekturen aus der Selbstprüfung (2026-10-24)

Der Plan wurde nach dem Schreiben gegen den Code geprüft. Acht Korrekturen wurden vorgenommen — **diese Liste ist der maßgebliche Stand**; wer eine frühere Fassung gelesen hat, arbeitet mit den korrigierten Stellen:

| # | Fehler in der Erstfassung | Korrektur | Beleg |
|---|---|---|---|
| K1 | `tgbotapi.Error.ResponseParams.RetryAfter` — **falscher Feldname und falscher Typ** | Richtig ist `*tgbotapi.Error` mit **eingebettetem** `ResponseParameters`; Feld heißt `RetryAfter`. Zugriff: `var tgErr *tgbotapi.Error; errors.As(err, &tgErr); tgErr.RetryAfter` | `tgbotapi v5.5.1` `types.go:22-27`, `types.go:1780-1789`, `bot.go:133-137` |
| K2 | WP-3 (503 bei `state != healthy`) bricht einen **bestehenden Test** | `TestTelegramStatusReturnsSanitizedRuntimeSnapshot` erwartet nach `MarkTelegramPollFailure` Status 200 ⇒ Testumbau ist Teil von WP-3 | `internal/server/telegram_handlers_test.go:15-25` |
| K3 | Akzeptanzkriterium „kein Consumer außer dem UI-Panel" — **unbelegt** | Verifiziert: es gibt **keinen** Consumer (nur Manifest, Route, Tests) | `internal/audit/audit_manifests.go:188`, `internal/server/server_routes.go:392` |
| K4 | Allowlist Enforcement nur an der Bridge — **ein zweiter Discord-Egress existiert** | Person-Detect-Alerts senden direkt via `discord.SendDiscordImage(cfg.Discord.DefaultChannelID, …)`; Default-Kanal muss erlaubt bleiben | `internal/server/person_detect_alerts.go:258` |
| K5 | „UI-Formular wird generisch gerendert, kein Formular-Code nötig" — **Annahme** | Verifiziert: Schema entsteht serverseitig per Reflection (`buildSchema`), `[]string` → `type: "array"` → Chip-Editor; für Telegram/Discord existiert kein eigener Renderer | `internal/server/config_handlers_providers.go:240, 284-285`; `ui/js/config/main.js:1677-1810, 2142` |
| K6 | `read_only` — **die Empfehlung hätte das API-Feld mit umbenannt** | Config-Key `readonly` und API-JSON-Feld `read_only` sind verschiedene Namensräume; nur der Config-Key wird korrigiert | `internal/config/config_types.go:1594` vs. `internal/discord/bot.go:76` |
| K7 | WP-12 verwies auf `internal/telegram/bot.go:505-517` für den Recap | Richtig ist `buildTelegramAgentMessages` (`:36-51`, Recap `:47`) | `internal/telegram/bot.go:36-51` |
| K8 | Neues Helper-Paket in WP-6 — Einordnung im Audit-Manifest unklar | `NetworkClientInventory` listet nur netzwerkfähige Pfade; `internal/telegram/` (`:258`) und `internal/discord/` (`:229`) sind bereits klassifiziert. Ein reines Helper-Paket braucht **keinen** Eintrag | `internal/audit/audit_manifests.go:213-269` |

Zusätzlich verifiziert und bestätigt (keine Änderung): `cfg.Directories.DataDir` existiert (`internal/config/config_types.go:1182`); `acquireTelegramWorker` blockiert (Backpressure, `internal/telegram/bot.go:72-86`), `runtime.go:126-128` gibt nur bei Kontextabbruch `false` zurück; `bot.Send` und `bot.GetUpdates` nutzen beide die `Code`-setzende `MakeRequest`-Variante (`tgbotapi bot.go:336`, `:404-414`).

---

*Plan erstellt am 2026-10-24 gegen den Arbeitsstand von `C:\Users\Andi\Documents\repo\AuraGo`. Alle Dateiverweise und Verhaltensannahmen wurden gegen die genannten Quellstellen geprüft; Drittanbieter-Verhalten gegen `tgbotapi v5.5.1` und `discordgo v0.29.0` im Modul-Cache verifiziert.*
