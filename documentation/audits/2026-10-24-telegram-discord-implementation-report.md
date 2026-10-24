# Umsetzungsbericht: Telegram- und Discord-Härtung (AuraGo)

**Datum:** 2026-10-24
**Bezug:** `documentation/audits/2026-10-24-telegram-discord-integration-audit.md` (Befunde H1–H4, M1–M11, N1–N10, D1–D6)
und `documentation/audits/2026-10-24-telegram-discord-remediation-plan.md` (WP-1 bis WP-13)
**Grundregel dieser Runde:** nebenwirkungsfrei — bestehendes Verhalten bleibt bei Standard-Konfiguration unverändert.

---

## 1. Was umgesetzt wurde

| Befund | Maßnahme | Dateien |
|---|---|---|
| **H1** Discord-Kanal-Escape | Kanal-Allowlist als **opt-in** Policy: leer ⇒ Legacy-Verhalten, gesetzt ⇒ nur Default-Kanal + gelistete Kanäle, Fehlermeldung nennt den Config-Key. Kanalliste wird gefiltert. | `internal/discord/policy.go` (neu), `internal/discord/bot.go`, `internal/agent/dispatch_messaging.go` |
| **H2** Telegram-Offset nur beim Stopp persistiert | Cursor-Datei `telegram_offsets.json` im DataDir (atomar, 0600), Persist alle 10 Updates / 30 s **nach** Übergabe an den Worker (at-least-once), Start mit 0 bei beschädigter Datei | `internal/telegram/offsets_store.go` (neu), `internal/telegram/runtime.go`, `internal/telegram/bot.go` |
| **H3** Polling ohne Timeout, Status immer 200 | 90-s-HTTP-Timeout, Staleness-Watchdog (3 min), 503 bei aktivem, aber nicht gesundem Poller | `internal/telegram/bot.go`, `internal/integrationstatus/telegram.go`, `internal/server/telegram_handlers.go` |
| **H4** Discord ohne Concurrency-Limit | Worker-Semaphore wie auf Telegram (Default 5), begrenztes Warten (60 s), danach Busy-Hinweis statt weiterem Lauf | `internal/discord/bot.go` |
| **M1** Kein Telegram-Read-only | `telegram.readonly` sperrt `send_telegram` (Text **und** Dokument) und den Notification-Kanal `telegram`; Parallelität zu `discord.readonly` | `internal/config/config_types.go`, `internal/agent/dispatch_messaging.go`, `internal/tools/notification.go`, `internal/server/flows_handlers_options.go` |
| **M2** Keine Längenprüfung im Tool-Pfad | Prüfung gegen 4096 Zeichen **nach Entity-Parsing** mit verständlicher Meldung | `internal/tools/notification.go` |
| **M5** `retry_after` ignoriert | 429 wird strukturiert erkannt (`*tgbotapi.Error`), Wartezeit = `RetryAfter` (Cap 60 s) | `internal/telegram/bot.go`, `internal/tools/notification.go` |
| **M6/N2** Stille Zustellverluste | Zustellfehler werden geloggt **und** gezählt (`delivery_failures`, `last_delivery_error_at`), Retry nur bei 429 (kein Doppelversand) | `internal/telegram/bot.go`, `internal/discord/bot.go`, `internal/integrationstatus/telegram.go` |
| **M7** Status-Semantik | `Polling` bedeutet jetzt „Poller läuft und ist nicht fehlgeschlagen“; `stale` als eigener Zustand | `internal/integrationstatus/telegram.go` |
| **M10** Discord-Discovery toter Pfad | Discovery-Modus loggt jetzt ID und Username (wie Telegram) | `internal/discord/bot.go` |
| **N1** Stille Trunkierung | Media-Download bricht hart ab statt still zu kürzen | `internal/telegram/bot.go` |
| **N4** Backoff ohne Jitter | ±20 % Jitter auf die Wartezeit | `internal/telegram/bot.go` |
| **N8** Globales Rate-Limit | bleibt (bewusst: nur ein Admin testet) | — |
| **N9/L6** Manifest-Lücken | Route-Verträge für `/api/telegram/test-connection`, `/api/discord/test`, `/api/health/discord`; Tool-Permission-Matrix um `send_telegram`, `send_discord`, `fetch_discord`, `list_discord_channels`; Regressionstest | `internal/audit/audit_manifests.go`, `internal/audit/audit_manifests_test.go` |
| **D1 (UI)** | Hilfetexte für `telegram.readonly`, `discord.allowed_channel_ids` und `discord.max_concurrent_workers` in **allen 16 Sprachdateien**. Der verwaiste Key `help.discord.read_only` wurde auf `help.discord.readonly` korrigiert — der YAML-Key heißt `readonly`, der alte Key wurde nie aufgelöst, der Tooltip des bestehenden Discord-Feldes fehlte dadurch | `ui/lang/help/*.json` (16 Dateien), `ui/config_help.json` |
| **D1** Falscher Config-Key `read_only` | Korrigiert auf `readonly` in Doku **und** Agenten-Fehlermeldung; das API-Feld `read_only` (anderer Namensraum) bleibt unverändert | `internal/agent/dispatch_messaging.go`, `documentation/configuration.md` |
| **D2** Cursor-Text | Neustart-Verhalten des Cursors beschrieben | `documentation/telegram_setup.md` |
| **D4** Kein Discord-Guide | `documentation/discord_setup.md` neu (Setup, Allowlist, Read-only, Concurrency, Health, Troubleshooting) | neu |
| **M4/M9/N5/WP-9/WP-11–13** | **bewusst zurückgestellt** (siehe Abschnitt 5) | — |

---

## 2. Neue Konfiguration

```yaml
telegram:
  readonly: false              # true = der Bot antwortet, der Agent darf aber nicht senden

discord:
  allowed_channel_ids: []      # leer = unverändertes Verhalten; gesetzt = nur diese + default_channel_id
  max_concurrent_workers: 5    # <= 0 fällt auf 5 zurück
```

Beide Felder sind nicht-sensibel, brauchen **keinen** Vault-Eintrag und erscheinen automatisch in der Web-UI
(Schema wird serverseitig per Reflection erzeugt, `internal/server/config_handlers_providers.go:240`).

**Wichtig für den Betrieb:** `discord.allowed_channel_ids` ist bewusst **opt-in**. Eine leere Liste lässt das
Agent-Verhalten unverändert, damit bestehende Installationen nicht brechen; der Bot loggt beim Start einmalig
einen Hinweis. Wer die Lücke schließen will, setzt die Liste — ab dann gilt fail-closed.

---

## 3. Verhaltensänderungen (bewusst und dokumentiert)

| Änderung | Wo | Warum |
|---|---|---|
| `/api/telegram/status` liefert 503, wenn Telegram **aktiv**, aber nicht `healthy` ist | `internal/server/telegram_handlers.go` | Ein ausgefallener Bot muss sichtbar sein. Deaktiviertes Telegram liefert weiter 200 (wie `/api/health/discord`) |
| `discord.max_concurrent_workers` = 5 | `internal/discord/bot.go` | Parität zu Telegram. Bei Erschöpfung wartet eine Nachricht bis zu 60 s und wird dann mit einem Busy-Hinweis abgelehnt — kein Agentenlauf |
| Zustellfehler werden gezählt | beide Status-Strukturen | Vorher gingen verlorene Antworten vollständig stumm |
| Cursor-Datei `telegram_offsets.json` im DataDir | `internal/telegram/` | Persistenz über Neustarts. Ein Neustart während einer Verarbeitung kann **eine** Nachricht erneut ausführen (at-least-once) |

**Unverändert:** Standard-Konfiguration ohne die neuen Felder verhält sich exakt wie vorher — kein Worker-Limit
auf Telegram, keine Allowlist-Wirkung auf Discord, kein Read-only, Cursor nur im Speicher, wenn kein DataDir gesetzt ist.

---

## 4. Verifikation

| Prüfung | Ergebnis |
|---|---|
| `go build ./...` | grün (Exit 0) |
| `go vet` über alle berührten Pakete (`telegram`, `discord`, `tools`, `config`, `audit`, `integrationstatus`, `agent`, `server`, `ui`) | grün (Exit 0) |
| `gofmt -l` über alle geänderten Go-Dateien | leer (alle Dateien formatiert) |
| `go test ./internal/telegram/...` | grün (41 Tests, 9 neu: Timeout, Flood-Control, `retry_after`-Cap, Jitter, Offset-Persistenz, beschädigte Cursor-Datei, leerer DataDir, End-to-End-Offset, Persistieren zur Laufzeit) |
| `go test ./internal/discord/...` | grün (24 Tests; neu: Allowlist-Policy, Worker-Semaphore, Busy-Pfad) |
| `go test ./internal/integrationstatus/...` | grün (3 Tests, vorher: „no test files“) |
| `go test ./internal/config/...` | grün |
| `go test ./internal/agent/ -run 'Discord\|Telegram\|Notification'` | grün |
| `go test ./internal/tools/ -run 'Telegram\|Notification\|Markdown'` | grün |
| `go test ./internal/audit/ -run 'ToolPermission\|RouteContract'` | grün |
| `go test ./internal/server/` (**vollständiges Paket**) | grün (387 s) — siehe Erläuterung zu Punkt 3 unten |
| `go test ./ui/` | grün (88 s) — deckt die 16 geänderten Sprachdateien ab |
| `node scripts/build-ui-bundles.js --check` | grün (kein Bundle-Drift) |
| `-race` | **nicht ausführbar** (CGO_ENABLED=0 in dieser Umgebung) |

### Vorbestehende Fehler, die nicht von dieser Änderung stammen

1. `internal/audit` → `TestNetworkClientInventoryCoversProductionHTTPClients` und
   `TestCredentialedHTTPClientsBindRedirectsToOrigin`: schlagen fehl, weil die **veraltete Repo-Kopie**
   `.tmp_aurago/baseline-check/` (git-ignoriert) vom Audit-Test mitgewalkt wird. **Jeder** beanstandete Pfad
   liegt unter `.tmp_aurago/` — kein echter Quellcode. Der Ordner stammt vom 23.10.2026, also aus einer früheren
   Session, und ist nicht Teil dieser Änderung. Entfernen mit `rm -rf .tmp_aurago/baseline-check`.
2. `internal/tools` → `TestPythonQualityValidationIgnoresAgentVenvCWDAndPythonPath`: verlangt ein
   vertrauenswürdiges System-Python, das in dieser Umgebung fehlt. Themenfremd.
3. `internal/server` → `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`: **umgebungsbedingt.**
   Der Test ruft `filepath.Rel` zwischen `t.TempDir()` und dem Paketverzeichnis auf. Auf dieser Maschine liegt
   das Go-Temp-Verzeichnis auf `D:` (`GOTMPDIR=D:\tmp\go-build`), das Repository auf `C:` — `filepath.Rel`
   scheitert auf Windows grenzüberschreitend ("can't make … relative to …"). Der Test ist unverändert
   (`internal/server/video_studio_handlers_test.go` ist nicht im Diff) und betrifft Video Studio, nicht
   Telegram/Discord. Mit `GOTMPDIR` auf demselben Volume wie das Repository läuft das **vollständige**
   `internal/server`-Paket grün (387 s, Exit 0).

---

## 5. Bewusst zurückgestellt

| Punkt | Grund |
|---|---|
| **E1 fail-closed als Default** | Widerspricht „nebenwirkungsfrei“. Umgesetzt als opt-in mit Startwarnung. |
| **WP-9 HTML-Rendering** im Antwortweg | Ändert sichtbares Verhalten der Bot-Antwort (Markdown wird heute literal angezeigt). Eigene Runde mit Fallback. |
| **WP-10 Mehrbenutzer-Allowlisten** (`[]int64`/`[]string`) | Slices in den Config-Structs brechen die `==`-Vergleiche in vier Dateien. Bewusst nicht im Sammelcommit. |
| **WP-10 Audio/Video/Sticker** auf Telegram | Eigene Format- und Vision-Anbindung, ohne Sicherheitsgewinn. |
| **WP-11 Webhook-Modus** | Ersetzt Polling; der Cursor-Fix löst den Alltagsfall. |
| **WP-12 Discord-Memory-Recap** | Unabhängig von der Sicherheitsarbeit, siehe Plan WP-12. |

---

## 6. Abschlussprüfung

**Lauf:** 24.10.2026 · Arbeitsbaum `C:\Users\Andi\Documents\repo\AuraGo` · committet in fünf Commits (siehe Abschnitt 8).

### 6.1 Nebenwirkungsfreiheit — geprüfte Zusicherungen

Jede neue Konfigurationsoption hat einen Default, der das Verhalten vor dieser Änderung exakt abbildet. Geprüft wurde
das jeweils an der Stelle, an der der Default angewendet wird:

| Zusicherung | Umsetzung | Beleg |
|---|---|---|
| Ohne `discord.allowed_channel_ids` bleibt **jeder** Kanal nutzbar | `ChannelAllowed` gibt `nil` zurück, sobald die Liste leer ist; der Filter greift erst danach | `internal/discord/policy.go:29-32` |
| Leere Liste = nicht gesetzte Liste | `normalizeChannelIDs` normalisiert auf `nil` (trimmen, leere Einträge und Duplikate entfernen) | `internal/config/config.go:257` |
| Ohne `discord.max_concurrent_workers` gilt das Telegram-Limit | Wert ≤ 0 ⇒ 5 | `internal/config/config.go:2352-2354` |
| Ohne `telegram.readonly` darf der Agent senden | Default `false`; die Sperre greift ausschließlich bei `true` | `internal/agent/dispatch_messaging.go:30`, `internal/tools/notification.go:104` |
| Ohne DataDir landet **kein** Cursor auf der Platte | `NewFileOffsetStore("")` liefert keinen Store — der Offset bleibt im Speicher | `internal/telegram/offsets_store.go:41-45` |
| Ein beschädigter Cursor blockiert den Start nicht | Fehlende/unlesbare Datei ⇒ „kein Cursor“, Start bei Offset 0 | `internal/telegram/offsets_store.go:70-72` |
| Ein Neustart wiederholt keine Nachricht | Der Cursor wird nach **jedem** an den Worker übergebenen Update geschrieben; ein Batch kann deshalb nicht ungespeichert bleiben | `internal/telegram/bot.go:106`, `internal/telegram/bot.go:187` |
| Kein Doppelversand bei Timeouts | Nur HTTP 429 (Flood-Control) wird wiederholt — diese Antwort sagt ausdrücklich, dass die Nachricht **nicht** angenommen wurde | `internal/tools/notification.go:342-377` |
| Ein gedrosseltes Telegram blockiert den Aufrufer nicht minutenlang | Die gesamte Sendung inklusive Flood-Control-Wartezeit bleibt in `telegramSendTotalBudget` (30 s); bei Ablauf kommt der Telegram-Fehler zurück | `internal/tools/notification.go:304`, `internal/tools/notification.go:338-339` |
| Die Längengrenze ist Telegrams reale Grenze | Gemessen wird **nach** dem Entity-Parsing (Escapes und Fettmarker zählen nicht mit): 4096 UTF-16-Einheiten | `internal/tools/notification.go:318-320` |
| Deaktiviert ist kein Ausfall | `/api/telegram/status` liefert 200 bei deaktivierter Integration; 503 nur bei aktivem, aber ungesundem Poller (Parität zu `/api/health/discord`) | `internal/server/telegram_handlers.go:38-44` |
| Die Allowlist wirkt nur auf Agent-Pfade | Sie greift in den Agent-Tool-Bridges und im Kanallisting; Person-Detect-Alerts bleiben auf dem Standardkanal | `internal/discord/policy.go`, `internal/agent/dispatch_messaging.go` |
| Konfigurations-Vergleiche bleiben korrekt | `oldCfg.Discord != newCfg.Discord` → `!reflect.DeepEqual(...)`, weil die Slices den `==`-Vergleich unzulässig machen | `internal/server/config_handlers_main.go:964` |
| Bestehende Discord-Felder behalten ihren Tooltip | `help.discord.read_only` → `help.discord.readonly`: der alte Key wurde nie aufgelöst, der Tooltip fehlte also schon vorher | `ui/lang/help/*.json` (16 Dateien) |

### 6.2 Testlauf

Vollständiger Lauf am 24.10.2026 (Details in Abschnitt 4):

- `go build ./...` → Exit 0
- `go vet` über `telegram`, `discord`, `tools`, `config`, `audit`, `integrationstatus`, `agent`, `server`, `ui` → Exit 0, keine Meldung
- `gofmt -l` über alle geänderten Go-Dateien → leer
- `go test ./internal/server/` (vollständiges Paket, 387 s) → **ok**, sobald `GOTMPDIR` auf demselben Volume wie das Repository liegt; der rote Erstlauf war der Windows-Umgebungseffekt aus Abschnitt 4, Punkt 3
- `go test ./internal/telegram/... ./internal/discord/... ./internal/integrationstatus/... ./internal/config/...` → ok
- `go test ./internal/agent/ -run 'Discord|Telegram|Notification'` → ok
- `go test ./internal/tools/ -run 'Telegram|Notification|Markdown'` → ok
- `go test ./internal/audit/ -run 'ToolPermission|RouteContract'` → ok
- `go test ./ui/` → ok (88 s; deckt die 16 geänderten Sprachdateien ab)
- `node scripts/build-ui-bundles.js --check` → Exit 0

### 6.3 Nicht ausgeführt / offen

| Punkt | Grund |
|---|---|
| `-race` | In dieser Umgebung ist `CGO_ENABLED=0` gesetzt; der Race-Detector braucht cgo. |
| `go test ./internal/audit/` (vollständig) | Fehlschläge stammen ausschließlich aus der veralteten Kopie `.tmp_aurago/baseline-check/` (23.10.2026, frühere Session). Kein Pfad dieser Kopie ist von der Änderung betroffen. |
| `go test ./internal/tools/` (vollständig) | `TestPythonQualityValidationIgnoresAgentVenvCWDAndPythonPath` verlangt ein vertrauenswürdiges System-Python, das hier fehlt. |

### 6.4 Umfang der Änderung

**Neu:** `internal/discord/policy.go`, `internal/telegram/offsets_store.go`, `internal/integrationstatus/telegram_test.go`,
`documentation/discord_setup.md`, `documentation/audits/` (Audit, Plan, dieser Bericht).

**Geändert (Go):** `internal/config/config.go`, `internal/config/config_types.go`,
`internal/server/config_handlers_main.go`, `internal/server/flows_handlers_options.go`,
`internal/server/telegram_handlers.go`, `internal/server/telegram_handlers_test.go`,
`internal/discord/bot.go`, `internal/discord/bot_test.go`, `internal/telegram/bot.go`,
`internal/telegram/runtime.go`, `internal/telegram/polling_test.go`, `internal/telegram/runtime_test.go`,
`internal/integrationstatus/telegram.go`, `internal/tools/notification.go`, `internal/tools/telegram_document.go`,
`internal/agent/dispatch_messaging.go`, `internal/agent/native_tools_integrations.go` (nur die Discord-Toolbeschreibungen),
`internal/audit/audit_manifests.go`, `internal/audit/audit_manifests_test.go`.

**Geändert (Konfiguration/Doku/UI):** `config_template.yaml`, `ui/config_help.json`, `ui/lang/help/*.json` (16 Dateien),
`documentation/configuration.md`, `documentation/telegram_setup.md`, `documentation/manuals/de/*.md`,
`documentation/manuals/en/*.md`.

**Hinweis zur Abgrenzung:** Im gemeinsamen Arbeitsbaum arbeitet parallel eine zweite Session an Paperless-, CYD- und
Release-Signing-Themen (`internal/cyd/hub.go`, `internal/server/cyd_handlers.go`, `internal/audit/release_signature_policy_test.go`,
`make_release.*`, `scripts/sign-release-checksums.*`, `ui/js/desktop/*`). Diese Dateien sind **nicht** Teil dieser
Umsetzung und wurden nicht gestaged. Einzige Überschneidung war `internal/agent/native_tools_integrations.go`: die
Paperless-Beschreibungen wurden von der Parallel-Session zuerst committet, der Commit dieser Umsetzung enthält dort
nur die beiden Discord-Toolbeschreibungen.

---

## 7. Selbst-Review und Nachbesserung

**Lauf:** 25.10.2026 · die eigene, noch nicht committete Änderung wurde gegen `HEAD` geprüft.
**Plan:** `documentation/audits/2026-10-24-telegram-discord-finding-fixplan.md`.

### 7.1 Befunde

| # | Befund | Datei | Status |
|---|---|---|---|
| 1 | Cursor wird bei einem Batch von 2–9 Updates nicht persistiert → Replay nach Neustart | `internal/telegram/bot.go:187` | behoben |
| 2 | 429-Retry blockiert den Notification-Pfad bis ~60 s | `internal/tools/notification.go:332-357` | behoben |
| 3 | Delivery-Zähler erfasst zwei Telegram-Sendepfade nicht | `internal/telegram/bot.go:512, 520` | behoben |

### 7.2 Korrektur an der ersten Befundbeschreibung

Die Erstfassung von Befund 1 war zu weit formuliert: Sie behauptete, der 30-S-Zweig würde im Leerlauf nie
ausgewertet. Das trifft nicht zu — der Long Poll läuft bis zu 60 s (`u.Timeout = 60`), deshalb ist
`time.Since(lastPersist)` beim **ersten** Update nach einer Pause immer ≥ 30 s und der Zeit-Zweig feuert
zuverlässig. Der Einzelnachricht-Fall war also nie betroffen.

Der tatsächliche Auslöser ist ein **Batch**: Updates 2–9 einer Folge werden innerhalb von Millisekunden
verarbeitet, deshalb greift weder die 10er-Schwelle noch der 30-S-Zweig. Folgt keine weitere Nachricht,
bleiben diese Updates nur im Speicher und werden nach einem Neustart erneut zugestellt.

### 7.3 Behebung

| Fix | Umsetzung | Datei |
|---|---|---|
| 1 | `telegramOffsetPersistEveryUpdates` von 10 auf 1 — der Cursor wird nach jedem an den Worker übergebenen Update geschrieben; der Codekommentar wurde auf die tatsächliche Semantik korrigiert (Hand-off, nicht Abschluss) | `internal/telegram/bot.go:101-107`, `:185-193` |
| 2 | `telegramSendTotalBudget` (30 s) begrenzt die gesamte Sendung inklusive Wartezeit; bei Ablauf kommt der Telegram-Fehler zurück, nicht der Deadline-Fehler | `internal/tools/notification.go:299-304`, `:335-339`, `:342-377` |
| 3 | `deliverMessage` nimmt einen Pfad-Bezeichner (`what`) entgegen; Kommando-Antwort, Kommando-Fehler, Antwort, Fehlernotiz, Reply-Aufforderung und Guardian-Sperrnotiz laufen alle darüber | `internal/telegram/bot.go:700-716` und 6 Aufrufer |

**Zustellgarantie nach Fix 1:** Für die Updates 2–9 eines Batches wechselt die Garantie von at-least-once zu
at-most-once. Das Update #1 verhält sich seit der ursprünglichen Umsetzung bereits so (der Cursor rückt bei
der Übergabe an den Worker vor), die Änderung macht das Verhalten einheitlich und deckt sich mit dem
Codekommentar sowie dem Plan-Kriterium T-1 („Update wird nicht doppelt verarbeitet").

### 7.4 Neue Tests

| Test | Schließt |
|---|---|
| `TestTelegramCursorIsPersistedWhilePolling` (angepasst) | Cursor wird nach jedem Update geschrieben (erwartet 2, 3, 4) |
| `TestTelegramCursorSurvivesRestartWithoutReplay` | Ein Neustart spielt ein Batch nicht erneut ab |
| `TestTelegramDeliveryFailureCounterCountsEverySendPath` | verlorene Sendungen zählen und werden zeitgestempelt |
| `TestTelegramSendsAllPassThroughTheDeliveryCounter` | Wächter: kein Sendepfad darf den Zähler umgehen |
| `TestSendTelegramNotificationStopsAtTheRetryBudget` | ein gedrosseltes Telegram blockiert den Aufrufer nicht |

### 7.5 Bewusst nicht geändert

- **Allowlist für den Inbound-Pfad und den Bot-eigenen Antwortweg** — so entschieden und in `documentation/discord_setup.md` Abschnitt 3 korrekt beschrieben.
- **Discord-Bot-Neustart bei jeder Config-Speicherung** — vorbestehend: der frühere `==`-Vergleich enthielt das Vault-Token ebenfalls, `reflect.DeepEqual` hat das weder verursacht noch verschärft.
- **„Echtes" at-least-once** (Cursor erst nach Worker-Abschluss) — die sauberere Langzeitlösung; sie braucht einen Abschluss-Rückkanal aus dem Worker (`internal/telegram/runtime.go:186`) und ist als Folge-Option im Fix-Plan festgehalten.

---

## 8. Commits

| Commit | Inhalt |
|---|---|
| `da449bb01` | `feat(telegram,discord): harden both messaging integrations` — die Umsetzung der Arbeitspakete WP-1 bis WP-8 und WP-10 (Teil A) |
| `525fcdb17` | `fix(telegram): persist the update cursor after every update` — Befund 1 |
| `9034a35b9` | `fix(tools): bound the telegram flood-control retry` — Befund 2 |
| `a8d432391` | `fix(telegram): count every failed telegram delivery` — Befund 3 |
| `docs(telegram): …` | `docs(telegram): correct the cursor persistence guarantee` — Audit, Plan, Fix-Plan, Bericht und die korrigierte Setup-Doku (dieser Commit; der Hash ändert sich beim Amend) |

---

*Umsetzung am 2026-10-24/25.10.2026 im Arbeitsbaum von `C:\Users\Andi\Documents\repo\AuraGo`.*
