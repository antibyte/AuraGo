# Audit: Telegram- und Discord-Integration (AuraGo)

**Datum:** 2026-10-24
**Umfang:** `internal/telegram/`, `internal/discord/`, `internal/tools/` (Telegram-/Discord-Bridge, Notification-Pfad), `internal/agent/dispatch_messaging.go`, `internal/server/` (Status-/Test-Endpunkte), `internal/config/`, `internal/audit/`, `internal/integrationstatus/`, UI-Config-Sektion, `documentation/telegram_setup.md`
**Methode:** Statische Code-Analyse (vollständige Lektüre der genannten Dateien), Abgleich gegen das Repo-Audit-Manifest (`internal/audit/audit_manifests.go`), Gegenprüfung von Drittanbieter-Verhalten im Modul-Cache (`tgbotapi v5.5.1`, `discordgo v0.29.0`), Verifikation jeder Aussage an der Quellstelle.
**Nicht geprüft:** Laufzeitverhalten (kein Live-Telegram/Discord), Frontend-Rendering im Browser, Deployment-Varianten (Docker/Tailscale) jenseits der Konfiguration.

> **Status:** Die Befunde H1–H4, M1–M11, N1–N10 und D1–D6 wurden am selben Tag umgesetzt — siehe
> [`2026-10-24-telegram-discord-implementation-report.md`](2026-10-24-telegram-discord-implementation-report.md).
> Bewusst zurückgestellt: WP-9 (HTML-Rendering der Antworten), Mehrbenutzer-Allowlisten, Webhook-Modus.
> H1 ist **opt-in** umgesetzt (leere Allowlist = unverändertes Verhalten), damit bestehende Installationen nicht brechen.

---

## 1. Executive Summary

Beide Integrationen sind **funktional solide und sicherheitsbewusst gebaut** — das ist keine Spielzeug-Implementierung. Auffällig sind vor allem **Lücken zwischen den beiden Kanälen**: Discord hat einen `readonly`-Schalter, Telegram nicht; Telegram hat ein Worker-Limit, Discord nicht; Discord hat einen Health-Endpunkt mit 503, Telegram nicht. Und es gibt **eine echte Lücke im Tool-Layer**: `send_discord`/`fetch_discord` akzeptieren eine beliebige `channel_id` ohne Allowlist, während derselbe Kanal über `send_notification` strikt auf den konfigurierten Kanal begrenzt ist.

| Bereich | Telegram | Discord | Gesamt |
|---|---|---|---|
| Eingangs-Authentifizierung | ✅ User-ID exakt, privat-Chat-Operator-Schutz | ✅ User-ID exakt, Discovery-Modus fehlerhaft | 🟢 |
| Injection-Abwehr | ✅ Guardian HIGH-Block + `IsolateExternalData` | ✅ Guardian HIGH-Block | 🟢 |
| Ausgangs-Tool-Autorisierung | ⚠️ kein Read-only-Modus | 🔴 **keine Kanal-Allowlist** | 🔴 |
| Zustellungszuverlässigkeit | 🔴 Offset nur beim Stopp persistiert | 🟢 Gateway (serverseitig) | 🔴 |
| Ausfallsichtbarkeit | 🔴 Status immer 200, kein Watchdog | 🟢 503 bei getrenntem Bot | 🔴 |
| Ressourcen-Begrenzung | ✅ `max_concurrent_workers` | 🔴 **unbegrenzt** (discordgo-Goroutinen) | 🔴 |
| Konfigurations-Parität | 🔴 3 Felder | 🟡 6 Felder | 🔴 |
| Dokumentation | 🟡 Setup-Guide, teils ungenau | 🔴 kein Setup-Guide | 🔴 |

**Die fünf wirksamsten Maßnahmen (Aufwand → Nutzen):**

1. **Discord-Kanal-Allowlist** erzwingen (S/H) — schließt die einzige echte Berechtigungslücke.
2. **Telegram-Offset persistent machen** (M) — verhindert doppelte Antworten und doppelte Kosten nach jedem Neustart.
3. **Discord-Worker-Semaphore** nachziehen (S) — deckelt Kosten bei Nachrichtenschwall.
4. **Telegram-Status auf echten Zustand + 503** umstellen (S) — Monitoring kann Ausfälle überhaupt erst sehen.
5. **HTTP-Timeout im Telegram-Poll-Client + Staleness-Watchdog** (S) — verhindert einen stillen Totalausfall des Bots.

---

## 2. Architektur in Kürze

### 2.1 Telegram

```
Telegram App ──GetUpdates(Langpolling, 60s)──► internal/telegram/runtime.go:94 runPollingLoop
                                                   │
                                                   ├─ UpdateOffsets (prozesslokal, Map je Bot-ID)
                                                   ├─ Worker-Semaphore (max_concurrent_workers, Default 5)
                                                   └─ processUpdate (internal/telegram/bot.go:193)
                                                        ├─ Session: ResolveUserSession("default")  ← global, nicht pro Chat
                                                        ├─ Voice → ffmpeg OGG→MP3 → TranscribeAudio
                                                        ├─ Photo → AnalyzeImageContext (Vision)
                                                        ├─ Document → media.SaveAttachmentContext → attachments/
                                                        ├─ Guardian-High-Block → security.IsolateExternalData
                                                        ├─ Slash-Commands (nur privat-Chat)
                                                        ├─ ExecuteAgentLoop (HistoryManager-Recap + STM 60)
                                                        └─ TelegramBroker → Antworten/Audio/YouTube/Fotos
```

Zusätzlicher, **völlig separater** Transportweg für Tools: `internal/tools/telegram_document.go` + `internal/tools/notification.go` sprechen die Bot-API direkt über HTTP (mit eigenem Timeout, Redirect-Verbot, Token-Redaktion). Die Bot-Runtime nutzt dagegen das SDK `tgbotapi`.

### 2.2 Discord

```
Discord Gateway ──discordgo (je Event eigener Goroutine)──► internal/discord/bot.go:196 AddHandler
                                                              │
                                                              ├─ shouldHandleDiscordMessage (bot.go:447)
                                                              │    allowed_user_id / DM / default_channel / Guild / Mention
                                                              ├─ handleMessage (bot.go:489) — attachments, Commands, Guardian
                                                              └─ processDiscordMessage (bot.go:729)
                                                                   ├─ Session: discordConversationID(m) ← pro Guild/Channel/User
                                                                   ├─ buildDiscordAgentMessages(nil)  ← KEIN Memory-Recap
                                                                   └─ ExecuteAgentLoop (nur STM 60)
```

Der Tool-Layer ist austauschbar gekapselt: `internal/tools/discord_bridge.go:37 RegisterDiscordBridge` injiziert `send`/`fetch`/`listCh` als Funktionswerte — die konkrete Discordgo-Anbindung bleibt im Bot-Paket.

### 2.3 Landkarte der Zuständigkeiten

| Datei | Rolle |
|---|---|
| `internal/telegram/runtime.go` | Polling-Lifecycle, Offset-Haltung, Worker-Semaphore |
| `internal/telegram/bot.go` | Nachrichtenverarbeitung, Voice/Vision/Attachments, Broker, Operator-Commands |
| `internal/telegram/http_client.go` | Kontextbindung des SDK-HTTP-Clients an den Server-Kontext |
| `internal/telegram/connection.go` | Token-Prüfung (`getMe`) für den Connection-Test |
| `internal/tools/telegram_document.go` | gehärteter Direkt-HTTP-Pfad für Dokumente (Attachment-Freigabe, Größe, Caption, Redaction) |
| `internal/tools/notification.go` | Fan-out an Ntfy/Pushover/Telegram/Discord/Telnyx/CYD |
| `internal/tools/discord_bridge.go` | Tool-Fassade für Discord |
| `internal/agent/dispatch_messaging.go` | Tool-Dispatch: `send_telegram`, `send_discord`, `fetch_discord`, `list_discord_channels` |
| `internal/server/telegram_handlers.go` | `/api/telegram/status`, `/api/telegram/test` |
| `internal/server/discord_handlers.go` | `/api/health/discord` |
| `internal/integrationstatus/telegram.go` | sanitisierter Telegram-Runtime-Status |

---

## 3. Befunde

### 3.1 HOCH

#### H1 — Discord-Kanal-Escape: `send_discord`/`fetch_discord` ohne Allowlist

**Stelle:** `internal/agent/dispatch_messaging.go:96-119` (Senden), `:121-158` (Lesen)
**Beobachtung:** `req.ChannelID` stammt direkt aus dem LLM-Tool-Aufruf. Es gibt keine Prüfung gegen `cfg.Discord.GuildID` oder `cfg.Discord.DefaultChannelID`. Der Agent kann damit **jeden Kanal lesen und beschreiben**, in dem der Bot Mitglied ist — auch in fremden Guilds, auch in Kanälen, die für den Nutzer gedacht sind. Ein Prompt-Injection-Vorfall genügt.

Zum Vergleich im selben Dispatcher:
- `list_discord_channels` ist auf `cfg.Discord.GuildID` festgenagelt (`dispatch_messaging.go:164`).
- `send_notification` mit Kanal `discord` geht strikt an `cfg.Discord.DefaultChannelID` (`internal/tools/notification.go:326-328`).

Der Read-only-Schalter greift nur für `send_discord` (`dispatch_messaging.go:100`) und **nicht** für `fetch_discord` — das ist gewollt, aber es unterstreicht die Lücke: im Read-only-Modus darf der Agent weiterhin beliebig Discord-Inhalte lesen.

Zusätzlich fehlen die direkten Messaging-Tools im **Tool-Permission-Manifest**: `ToolPermissionMatrix` (`internal/audit/audit_manifests.go:22-71`) listet `send_notification` (`:40`) — das dieselben Discord-/Telegram-Kanäle erreicht — aber **nicht** `send_telegram`, `send_discord`, `fetch_discord` oder `list_discord_channels`. Die mächtigeren Wege sind damit gerade die undokumentierten.

**Zweiter Discord-Egress-Pfad außerhalb der Bridge:** `internal/server/person_detect_alerts.go:258` ruft `discord.SendDiscordImage(cfg.Discord.DefaultChannelID, …)` direkt auf — der Bot-Pfad umgeht `internal/tools/discord_bridge.go` komplett. Eine Allowlist muss deshalb **zwei** Dinge klären: (a) welche Kanäle der Agent per Tool lesen/darf, (b) dass der Alert-Pfad auf dem Default-Kanal bleibt (er ist per Konstruktion erlaubt) und nicht von der Tool-Allowlist abgeschnitten wird.

**Risiko:** Datenabfluss aus Kanälen außerhalb des konfigurierten Scopes; Post-Missbrauch (Spam, Datenmanipulation) in beliebigen Kanälen.

**Vorschlag:**
0. Die vier Tools in die `ToolPermissionMatrix` aufnehmen (ConfigGate + ReadOnlyGate + Capabilities) — die Matrix beschreibt sonst nicht den tatsächlichen Werkzeugbestand.1. Neue Config `discord.allowed_channel_ids []string` (leer = nur `default_channel_id`).
2. **Durchsetzung in `internal/tools/discord_bridge.go`**, nicht nur im Dispatcher — damit gelten die Regeln für jeden Aufrufer (Tool-Layer, Web-UI, Flows). Beispiel:

```go
// internal/tools/discord_bridge.go
func RegisterDiscordBridge(cfg *config.Config, send ..., fetch ..., listCh ...) {
    allowed := func(channelID string) bool {
        id := strings.TrimSpace(channelID)
        if id == "" { return false }
        if id == strings.TrimSpace(cfg.Discord.DefaultChannelID) { return true }
        for _, a := range cfg.Discord.AllowedChannelIDs {
            if strings.TrimSpace(a) == id { return true }
        }
        return false
    }
    // im injizierten fetch/send vor dem Weiterleiten prüfen
}
```
3. Fehlermeldung an den Agenten explizit: *„channel_id ist nicht in discord.allowed_channel_ids; erlaubt sind: …"*.
4. Ergänzend: `discord.allowed_guild_ids` prüfen, wenn der Bot mehreren Guilds beitreten soll.

**Aufwand:** S (≈ 60 Zeilen + Config + UI-Feld + Tests).

---

#### H2 — Telegram-Update-Offset wird nur beim Stopp persistiert → Doppelverarbeitung nach Neustart

**Stelle:** `internal/telegram/runtime.go:113-137`

```go
offset := runPollingLoop(ctx, bot, u, plannerDB, logger, func(update tgbotapi.Update) bool { ... })
offsets.set(bot.Self.ID, offset)   // ← erst NACH Ende der Polling-Schleife
```

Der Cursor wird **während** der Laufzeit nur in der lokalen Variable `updateCfg.Offset` geführt (`bot.go:152`). `offsets.set` (`runtime.go:30-37`) schreibt in eine **prozesslokale Map** (`UpdateOffsets`), die zudem beim Serverstart leer ist.

**Folge:** Nach einem Neustart, Crash oder `kill -9` liefert Telegram alle seit dem letzten graceful Stop nicht bestätigten Updates erneut. AuraGo verarbeitet sie erneut: voller Agentenlauf, erneute Tool-Aufrufe, erneute LLM-Kosten, erneute Antwort an den Nutzer. Bei 10 unbeantworteten Nachrichten sind das 10 doppelte Läufe.

Der Doku-Text in `documentation/telegram_setup.md:44-46` („The update cursor is preserved for the same verified bot during this server process") beschreibt diesen Zustand zwar korrekt für **Konfigurationswechsel**, erwähnt aber das Neustart-Verhalten nicht.

**Risiko:** Kostenexplosion, doppelte Antworten, unerwartete Tool-Ausführung (z. B. doppelte Cronjobs, doppelte Mails).

**Vorschlag:** Cursor periodisch persistieren, nicht erst beim Stopp.
- `UpdateOffsets` um eine Persistenz erweitern (SQLite-Tabelle `telegram_offsets(bot_id INTEGER PRIMARY KEY, offset INTEGER, updated_at TEXT)` — die DB (`plannerDB`/`inventoryDB`) ist ohnehin verfügbar).
- Alle N Updates **oder** alle X Sekunden schreiben (z. B. alle 5 Updates oder nach 3 s), und den Cursor **vor** der Verarbeitung der Nachricht bestätigen (At-most-once) bzw. danach (At-least-once) — bewusst entscheiden und dokumentieren.
- `offsets.get` beim Start aus der DB laden statt aus der Map.

**Aufwand:** M (≈ 100 Zeilen + Migration + Tests).

---

#### H3 — Telegram-Long-Polling ohne HTTP-Timeout und ohne Staleness-Erkennung

**Stelle:** `internal/telegram/bot.go:58`

```go
return startLongPolling(ctx, cfg, logger, plannerDB, offsets, tgbotapi.APIEndpoint, &http.Client{}, ...)
```

`&http.Client{}` hat **kein Timeout**. Kombiniert mit `u.Timeout = 60` (`runtime.go:111`) bedeutet das: Der TCP-Verbindungsaufbau zu `api.telegram.org` kann unbegrenzt hängen (Firewall-Drop, IPv6-Blackhole, Netzwerkwechsel im VPN). `GetUpdates` kehrt nie zurück.

Gleichzeitig gibt es **keinen Watchdog**: `integrationstatus.MarkTelegramPollSuccess` setzt `State = "healthy"` (`internal/integrationstatus/telegram.go:53-61`), aber niemand prüft, ob `LastSuccessfulPollAt` veraltet ist. Der Status meldet also weiter „healthy", während der Bot seit Stunden tot ist. `handleTelegramStatus` gibt zusätzlich **immer** HTTP 200 und `"status":"ok"` zurück (`internal/server/telegram_handlers.go:29-34`).

**Risiko:** Totalausfall des Telegram-Kanals wird weder in der UI noch im Monitoring sichtbar.

**Vorschlag:**
```go
// internal/telegram/bot.go:58
telegramHTTPClient := &http.Client{Timeout: 90 * time.Second} // > Long-Poll-Timeout (60s)
```
Zusätzlich im Status-Getter eine Staleness-Regel:
```go
// internal/integrationstatus/telegram.go
func TelegramStatus() TelegramRuntimeStatus {
    // ... bestehende Felder
    if st.State == "healthy" && st.LastSuccessfulPollAt != "" {
        if time.Since(parse(st.LastSuccessfulPollAt)) > 3*time.Minute {
            st.State = "stale"
        }
    }
    return st
}
```
und in `handleTelegramStatus` bei `state != "healthy"` **503** zurückgeben (analog zu `handleDiscordHealth`, `internal/server/discord_handlers.go:30-34`).

**Aufwand:** S (≈ 30 Zeilen + Test).

---

#### H4 — Discord ohne Concurrency-Limit (unbegrenzte Agentenläufe)

**Stelle:** `internal/discord/bot.go:196-205` (Handler), `:624` (Aufruf von `processDiscordMessage`)

discordgo startet **jeden** Event-Handler in einer eigenen Goroutine (`discordgo@v0.29.0/event.go:171`: `go eh.eventHandler.Handle(s, i)`), sofern `SyncEvents` gesetzt wäre — was AuraGo **nicht** tut (im ganzen Repo kommt `SyncEvents` nicht vor). Jede Discord-Nachricht startet damit einen ungebremsten, vollständigen Agentenlauf.

Telegram begrenzt das über `workerSem` (`internal/telegram/runtime.go:103-107`, `acquireTelegramWorker` in `bot.go:72-86`). Discord hat keine Entsprechung, und `cfg.Discord` besitzt kein `max_concurrent_workers`-Feld (`internal/config/config_types.go:1592-1599`).

**Risiko:** Ein Nachrichtenschwall (Versehen, Spam, ein launischer Nutzer) startet beliebig viele parallele LLM-Läufe, Tool-Aufrufe und Memory-Schreibvorgänge. Kosten- und Ressourcenexplosion ohne jede Bremse.

**Vorschlag:**
- `discord.max_concurrent_workers int` (Default 5) in `internal/config/config_types.go` + Default in `internal/config/config.go` (analog `config.go:2319-2322`).
- In `handleMessage` vor `processDiscordMessage` ein Semaphore anfordern (identisch zu `acquireTelegramWorker`), bei voller Belegung die Nachricht ablehnen statt zu blockieren.

**Aufwand:** S (≈ 40 Zeilen).

---

### 3.2 MITTEL

#### M1 — Fehlende Read-only-Sperre für Telegram

`discord.readonly` existiert (`internal/config/config_types.go:1594`), Telegram hat kein Gegenstück (`config_types.go:1442-1446` enthält nur `telegram_user_id`, `bot_token`, `max_concurrent_workers`). Über `send_telegram` (`internal/agent/dispatch_messaging.go:29-44`) und `send_notification` mit Kanal `telegram` kann der Agent aus Telegram heraus immer schreiben — auch aus einem Szenario, in dem der Nutzer den Kanal nur lesen lassen wollte.

**Vorschlag:** `telegram.readonly bool` ergänzen; im `send_telegram`-Dispatch und im Notification-Pfad (`internal/tools/notification.go`, analog zur Discord-Prüfung in `:106-109`) durchsetzen. Fehlermeldung an den Agenten in derselben Form wie Discord.

#### M2 — `send_telegram` ohne Längenprüfung gegen das 4096-Limit

`send_telegram` → `tools.SendNotificationContext` → `sendTelegramNotification` (`internal/tools/notification.go:294-316`) sendet Titel + Text als **eine** Nachricht, ohne die Grenze von 4096 UTF-16-Units zu prüfen. Eine lange Agenten-Antwort scheitert daher mit einem Telegram-API-Fehler, obwohl der Bot-Antwortweg (`internal/telegram/bot.go:656-664` + `internal/telegram/text_chunks.go`) sauber chunked. Der Flows-Pfad prüft die Grenze bereits (`internal/flows/catalog_actions_notify_hardening_test.go`), der Agent-Tool-Pfad nicht.

**Vorschlag:** Längenprüfung in `sendTelegramNotification` ergänzen (Fehlermeldung mit tatsächlicher/erlaubter Länge) oder dort chunken. `utf16Units` existiert bereits in `internal/tools/telegram_document.go`.

#### M3 — Zwei verschiedene Markdown-Semantiken auf demselben Kanal

- **Antwort an den Chat-Bot:** `sendTelegramMessage` (`internal/telegram/bot.go:656-664`) sendet **Plaintext ohne `parse_mode`** → LLM-Markdown (`**fett**`, Code-Fences, Listen) erscheint literal als Sternchen und Backticks.
- **Benachrichtigung:** `sendTelegramNotification` (`internal/tools/notification.go:301-307`) nutzt `MarkdownV2` mit korrektem Escaping → wird gerendert.

Der Nutzer sieht also je nach Weg eine unterschiedliche Darstellung derselben Inhalte.

**Vorschlag:** Den Antwortweg auf `parse_mode: HTML` (oder MarkdownV2 mit Escaping) umstellen und `escapeMarkdownV2`/`html.EscapeString` verwenden. Achtung: Der Inhalt ist LLM-Output und muss vor dem Escaping von Markdown in Telegram-Syntax übersetzt werden — besser ein kleiner, robuster Konverter (Bold/Italic/Code/Fences/Listen) statt reines Escaping.

#### M4 — Kein Telegram-`enabled`-Schalter

Discord hat `discord.enabled` (`config_types.go:1593`), Telegram nicht. Ein Telegram-Bot mit gesetztem Token startet immer (sofern `EggMode` aus ist, `runtime.go:66`). Abschalten geht nur über das Entfernen des Tokens aus dem Vault — ein粒ularerer Schalter fehlt.

**Vorschlag:** `telegram.enabled bool` ergänzen (Default `true`, wenn Token gesetzt), analog zu Discord.

#### M5 — Flood-Control: `retry_after` wird ignoriert

`telegramPollErrorCode` (`internal/telegram/bot.go:157-170`) erkennt „429" nur per String-Matching und leitet daraus einen eigenen Backoff ab (1 s → 30 s, `bot.go:129-134`). Die Bot-API liefert im Fehlerfall aber `parameters.retry_after`; dieses strukturierte Feld wird nicht ausgewertet.

**Korrekte Zugriffsform (verifiziert gegen `tgbotapi v5.5.1`):** Der Fehler ist ein **Zeiger** `*tgbotapi.Error`; `RetryAfter` ist ein Feld des eingebetteten `ResponseParameters`:

```go
var tgErr *tgbotapi.Error
if errors.As(err, &tgErr) && tgErr.Code == 429 {
    wait := tgErr.RetryAfter // Sekunden
}
```
`bot.Send` → `Request` → `MakeRequest` (`bot.go:95-141`) und `bot.GetUpdates` → `Request` → `MakeRequest` (`bot.go:404-414`) nutzen **beide** die Variante, die `Code` setzt (`bot.go:133-137`). Die `Code`-lose Variante (`bot.go:257-260`) wird von diesen Pfaden nicht verwendet.

**Vorschlag:** Bei `*tgbotapi.Error` den Wert `RetryAfter` lesen (siehe oben) und die Wartezeit darauf setzen; zusätzlich Jitter in den Backoff.

#### M6 — Stille Zustellverluste (uneinheitliches Fehler-Handling)

- **Telegram-Broker:** `sendTelegramMessage(...)` wird **dreimal ohne Fehlerprüfung** aufgerufen (`internal/telegram/bot.go:776`, `:823`, `:826`, `:829`). Eine fehlgeschlagene Zustellung ist damit unsichtbar — weder im Log noch im Status.
- **Discord:** Der Broker prüft Fehler und loggt sie (`internal/discord/bot.go:898-953`), aber `handleMessage` ignoriert **viermal** das Ergebnis von `s.ChannelMessageSend(...)`: `bot.go:558`, `:581`, `:595`, `:817` (u. a. die Fehlermeldung an den Nutzer selbst).

**Vorschlag:** Einen gemeinsamen Send-Wrapper mit Fehlerzählung einführen, `delivery_failures`/`last_delivery_error` in den Status aufnehmen und beide Broker darauf umstellen. Status-Endpunkt von M1/H3 profitiert direkt davon.

#### M7 — Status-Semantik: `Polling: true` trotz Fehler, kein `enabled`

`MarkTelegramPollFailure` setzt `Polling = true` (`internal/integrationstatus/telegram.go:63-72`). Ein dauerhaft fehlschlagendes Polling meldet damit „polling: true" und „state: degraded" gleichzeitig. `SetTelegramConfigured` setzt `Polling = false` (`:32`), `MarkTelegramUnavailable` ebenfalls (`:77`) — die drei Markierungs-Funktionen haben also **drei verschiedene Bedeutungen** für dasselbe Feld.

**Vorschlag:** `Polling` nur setzen, wenn tatsächlich gepollt wird; Fehlerfall → `Polling: false, State: "degraded"`. Felder umbenennen (`polling_active`) oder in ein Unterstruktur-Modell (`state` + `details`) überführen. Zusätzlich `Enabled` aus dem Config ableiten, nicht aus dem Prozesszustand.

#### M8 — `handleTelegramTest` kann unbegrenzt hängen

`handleTelegramTest` (`internal/server/telegram_handlers.go:37-67`) ruft `telegram.SendTestMessage` → `tgbotapi.NewBotAPI(...)` (`internal/telegram/bot.go:177`). Der SDK-Default-Client hat **keinen Timeout**; ist die Telegram-API nicht erreichbar, blockiert der Handler-Request unbegrenzt. Zusätzlich ist das Rate-Limit **prozessglobal** (Mutex + Zeitstempel, `:14-17`, `:44-51`) — bei mehreren Admins blockiert sich der nächste Test für eine Minute.

**Vorschlag:** `tgbotapi.NewBotAPIWithClient` mit `&http.Client{Timeout: 15 * time.Second}`; Rate-Limit pro Admin-Subjekt (oder pro Quell-IP) statt global.

#### M9 — Nur eine erlaubte Benutzer-ID pro Kanal

Telegram: genau eine `telegram_user_id` (int64). Discord: genau eine `allowed_user_id` (String). Für Teams, Rollen oder mehrere Geräte mit unterschiedlichen Accounts ist das zu eng; eine Liste existiert nicht.

**Vorschlag:** `telegram.allowed_user_ids []int64` bzw. `discord.allowed_user_ids []string` ergänzen, Singular-Feld als Rückwärtskompatibilität weiter auswerten.

#### M10 — Discord-„Discovery mode" ist ein toter Pfad mit irreführendem Verhalten

`allowedUserID == "0"` führt zu `messageDecision{Reason: "discovery_mode"}` → die Nachricht wird **blockiert** und geloggt (`internal/discord/bot.go:459-461`, `:499-500`). Der Telegram-Weg hat einen echten Discovery-Modus: `telegram_user_id: 0` → die Nutzer-ID wird ins Log geschrieben (`internal/telegram/runtime.go:118-121`, dokumentiert in `documentation/telegram_setup.md:25-29`).

**Vorschlag:** Discord-Discovery analog implementieren (User-ID im Log ausgeben, Nachricht blockieren) oder das Verhalten entfernen und das Feld aus der Doku streichen.

#### M11 — `send_telegram` und `send_notification` sind zwei Wege mit unterschiedlicher Semantik

Beide zielen auf denselben Chat, aber: `send_notification` kennt Priorität und Channel-Fan-out, `send_telegram` nicht; `send_telegram` kennt zusätzlich `file_path` (Dokumentversand), `send_notification` nicht; `send_notification` respektiert `discord.readonly`, `send_telegram` hat kein Äquivalent. Für den Agenten ist das schwer vorhersehbar.

**Vorschlag:** Entweder konsolidieren (ein Telegram-Tool mit optionalem `file_path`) oder die Semantik beider Wege explizit im Tool-Manual dokumentieren (`prompts/tools_discord.md`, `prompts/tools_manuals/discord.md`).

---

### 3.3 NIEDRIG / ROBUSTHEIT

#### N1 — Stille Trunkierung beim Telegram-Mediendownload

`downloadFile` (`internal/telegram/bot.go:848-890`) prüft die Größe nur über `resp.ContentLength` (`:867`) und liest dann mit `io.LimitReader(resp.Body, 50MB)` (`:884`). Fehlt der Content-Length (chunked), wird die Datei **stumm abgeschnitten** — das Bild ist dann kaputt, die Vision-Analyse scheitert mit einer irreführenden Meldung.

**Vorschlag:** `limit+1` Bytes lesen und bei Überschreitung hart fehlschlagen.

#### N2 — Kein Retry beim Senden

Weder `sendTelegramMessage` (`internal/telegram/bot.go:656-664`) noch `SendMessage` (`internal/discord/bot.go:281-325`) wiederholen einen fehlgeschlagenen Versand. Ein einzelner 429 oder ein TCP-Reset bedeutet: die Antwort ist verloren, der Nutzer sieht nichts und bekommt keine Fehlermeldung. discordgo hat einen internen Rate-Limiter mit Retry; der Telegram-Pfad hat nichts.

**Vorschlag:** Gemeinsamer Retry-Wrapper: bei 429 mit `retry_after`, bei 5xx/Timeout mit Backoff, max. 3 Versuche.

#### N3 — Polling ist nicht horizontal skalierbar

`startLongPolling` löscht beim Start aktiv jeden Webhook (`internal/telegram/runtime.go:95`) und pollt thereafter exklusiv. Zwei Instanzen mit demselben Token verarbeiten dieselben Updates **doppelt** (beide lesen denselben Offset-Stand). Eine Leader-Election gibt es nicht.

**Vorschlag:** Single-Instance-Annahme explizit dokumentieren (README/Setup-Guide) oder per Datei-Lock im DataDir absichern.

#### N4 — Backoff ohne Jitter

`internal/telegram/bot.go:129-134` verdoppelt den Backoff determinisch. Bei mehreren Instanzen oder mehreren Bots im selben Prozess synchronisieren sich die Wiederholungen.

**Vorschlag:** ±20 % Jitter einstreuen.

#### N5 — Nur `document`-Attachments werden verarbeitet

Telegram verarbeitet ausschließlich `msg.Document` (`internal/telegram/bot.go:358`). Audio-, Video- und Sticker-Nachrichten werden ignoriert (Feld `Sticker` wird nirgends gelesen). Discord verarbeitet Attachments generisch (`internal/discord/bot.go:629-728`).

**Vorschlag:** Audio-/Video-/Sticker-Dateien analog zum Dokument-Pfad aufnehmen (Sticker: als Bild an die Vision).

#### N6 — Zwei Telegram-Transportwege mit unterschiedlicher Härtung

| | Bot-Runtime (SDK) | Tool-/Notification-Pfad (HTTP) |
|---|---|---|
| Timeout | **keiner** (`bot.go:58`) | 15 s / Upload-Deadline (`telegram_document.go:39-43`) |
| Redirect-Verbot | **nein** | ja (`telegramNoRedirect`, `:33-35`) |
| Token-Redaktion im Fehler | nur indirekt über den Logger-Scrubber | explizit (`redactTelegramToken`, `telegramRequestError`) |

Hinweis zur Einordnung: Der Logger scrubbt zentral (`internal/logger/logger.go:104-110` ruft `security.Scrub` auf allen Attributen), und der Token wird beim Start des Pollers registriert (`internal/telegram/runtime.go:80`). Ein Token-Leak in die Logs ist daher **derzeit nicht zu befürchten**. Der Befund ist eine *Abweichung der Härtungsstrategie*: der SDK-Pfad verlässt sich auf eine globale Invariante, der HTTP-Pfad schützt sich selbst. Wird der Logger je ausgetauscht (z. B. `slog.Default()` in einem Tool-Kontext), ist der SDK-Pfad ungeschützt.

**Vorschlag:** Den SDK-Pfad auf denselben abgesicherten Client umstellen (`&http.Client{Timeout: 90*time.Second, CheckRedirect: telegramNoRedirect}`) und Fehler explizit redigieren — dann gilt die Härtung unabhängig vom Logger.

#### N7 — `RegisterSensitive` nicht im Test-Pfad

`security.RegisterSensitive(cfg.Telegram.BotToken)` wird nur beim Start des Pollers aufgerufen (`internal/telegram/runtime.go:80`), nicht in `SendTestMessage` (`internal/telegram/bot.go:173-187`). Unkritisch (Logger scrubbt), aber inkonsistent.

#### N8 — Rate-Limit des Test-Endpunkts ist prozessglobal

`internal/server/telegram_handlers.go:14-17` und `:44-51`: ein globaler Mutex + Zeitstempel für **alle** Aufrufer.

#### N9 — Audit-Manifest deklariert die Admin-Routen schwächer als implementiert

`/api/telegram/test-connection` und `/api/discord/test` sind im Code admin-geschützt (`internal/server/server_routes_config.go:123-124`: `requireAdmin(s, …)`), im Route-Contract-Manifest aber **nicht** explizit aufgeführt — sie fallen auf den Sammelvertrag `{Pattern: "/api/", Auth: "session", Category: "misc"}` zurück (`internal/audit/audit_manifests.go:177`). Für `/api/telegram/status` und `/api/telegram/test` existieren dagegen explizite Einträge (`:188-189`). Der Manifest-Test (`internal/audit/audit_manifests_test.go:88-119`) prüft nur die Abdeckung, nicht die Auth-Stufe.

**Vorschlag:** Explizite Manifest-Einträge für `/api/telegram/test-connection` (admin), `/api/discord/test` (admin), `/api/health/discord` (session) ergänzen — dann stimmt das Sicherheits-Contract-Manifest mit dem Code überein. Zusätzlich die vier Messaging-Tools (`send_telegram`, `send_discord`, `fetch_discord`, `list_discord_channels`) in die `ToolPermissionMatrix` (`internal/audit/audit_manifests.go:22-71`) aufnehmen; dort steht bisher nur `send_notification` (`:40`).

#### N10 — Status-Endpunkte sind bei deaktivierter Auth öffentlich

`/api/telegram/status` und `/api/health/discord` hängen am globalen `authMiddleware`. Ist `auth.enabled` aus, sind beide ohne Auth erreichbar und geben interne Zustände preis (Polling-Zustand, Fehlercodes, ob ein Bot-Token konfiguriert ist). Für einen selbst gehosteten Heimserver ist das ggf. gewollt — es sollte aber dokumentiert sein.

---

## 4. Paritäts-Matrix Telegram ↔ Discord

| Fähigkeit | Telegram | Discord | Lücke |
|---|---|---|---|
| Aktivierung | implizit (Token+UserID) | `discord.enabled` | **Telegram** (M4) |
| Read-only-Modus | ✗ | ✓ `discord.readonly` | **Telegram** (M1) |
| User-Allowlist | 1 ID (int64) | 1 ID (String) | beide (M9) |
| Discovery-Modus | ✓ (ID ins Log) | ✗ (toter Pfad) | **Discord** (M10) |
| Kanal-/Chat-Bindung | implizit (UserID) | `guild_id` nur bei `list_discord_channels` erzwungen | **Discord** (H1) |
| Allowlist beim Senden/Lesen | n/a | ✗ | **Discord** (H1) |
| Concurrency-Limit | ✓ `max_concurrent_workers` | ✗ | **Discord** (H4) |
| Offset-/Cursor-Haltung | nur prozesslokal | serverseitig (Gateway) | **Telegram** (H2) |
| Transport | Polling (SDK) | Gateway (discordgo) | ok |
| Health-Endpunkt | `/api/telegram/status` (immer 200) | `/api/health/discord` (503) | **Telegram** (H3/M1) |
| Status-Granularität | `state`, `polling`, `last_success`, Fehlercode | `status`, `connected`, `message` | **Telegram** feiner |
| Attachments | nur `document` | generisch | **Telegram** (N5) |
| Voice | ✓ (OGG→MP3→Transkription) | ✓ (gleiche Telegram-Basis) | ok |
| Bilder | ✓ Vision + Foto | ✓ Vision + Bild | ok |
| Markdown-Ausgabe | Antwort: plain / Notification: MarkdownV2 | Markdown | **Telegram** (M3) |
| Fehlerbehandlung beim Senden | ignoriert | gemischt | beide (M6) |
| Token-Redaktion | Tool-Pfad explizit, SDK-Pfad indirekt | `security.Scrub` | **Telegram** (N6) |
| Connection-Test | ✓ `/api/telegram/test` + `/api/telegram/test-connection` | ✓ `/api/discord/test` | ok |
| Setup-Dokumentation | ✓ `documentation/telegram_setup.md` | ✗ | **Discord** |

---

## 5. Dokumentations-Widersprüche

| # | Aussage | Realität | Stelle |
|---|---|---|---|
| D1 | (a) Fehlermeldung an den Agenten: *„Disable `discord.read_only` to allow changes"*; (b) **zentrale Config-Referenz** listet den Key `read_only` | Der YAML-Key heißt `discord.readonly` (kein Unterstrich). Der Top-Level-Decoder nutzt **kein** `KnownFields(true)` (`internal/config/config.go:1022`) ⇒ ein gesetztes `read_only: true` wird **stillschweigend ignoriert**, die Schreibsperre ist nicht aktiv (fail-open) | `internal/agent/dispatch_messaging.go:101`; `documentation/configuration.md:198` vs. `internal/config/config_types.go:1594` |
| D2 | *„Active messages are not automatically replayed. The update cursor is preserved … during this server process"* | gilt nur für Konfigurationswechsel; nach einem **Prozessneustart** werden offene Updates erneut zugestellt (kein persistenter Cursor) | `documentation/telegram_setup.md:44-46` vs. `internal/telegram/runtime.go:137` |
| D3 | YAML-Beispiel `telegram: telegram_user_id: 987654321` | redundant/irreführend; die UI-Felder heißen `telegram.telegram_user_id` und `telegram.bot_token` | `documentation/telegram_setup.md:37-41` vs. `ui/js/config/integration_actions.js:11` |
| D4 | (fehlend) | Kein Discord-Setup-Guide (`documentation/discord_setup.md` existiert nicht), obwohl Discord dieselbe Konfigurationstiefe hat | `documentation/` |
| D5 | (fehlend) | Keine Dokumentation der Grenzen von `send_discord`/`fetch_discord` (keine Kanal-Allowlist) — weder im Manual noch im Tool-Prompt | `prompts/tools_discord.md`, `prompts/tools_manuals/discord.md` |
| D6 | (fehlend) | `discord.readonly` verhindert Senden, aber **nicht** Lesen — im Prompt-Manual nicht ausgewiesen | `internal/agent/dispatch_messaging.go:100-102` vs. `:121-158` |

**Abgrenzung zu D1:** `discord.readonly` (Config-Key, `config_types.go:1594`) und `read_only` (JSON-Feld der Status-API, `internal/discord/bot.go:76`) sind **verschiedene Namensräume** — Config-Key einerseits, API-Antwort andererseits. Nur der Config-Key ist falsch dokumentiert; das API-Feld darf beim Fix nicht mit umbenannt werden. Umgekehrt ist `read_only` als API-Feld der einzige Ort, an dem die Schreibsperre für Clients sichtbar wird — dieser Name sollte bleiben.

---

## 6. Testlücken

Vorhandene Tests (Auswahl): `internal/telegram/{polling,runtime,connection,guardian,cancellation,turn_serialization,text_chunks,operator_commands,vision,voice_redirect,send_photo,context_audit}_test.go`, `internal/tools/telegram_document{,_hardening}_test.go`, `internal/server/{telegram_runtime,telegram_handlers,discord_handlers}_test.go`, `internal/agent/send_telegram_path_hardening_test.go`, `internal/discord/{bot,connection,operator_commands,ingress_cancellation}_test.go`.

**Nicht abgedeckt (Vorschlag für neue Tests):**

| # | Fehlender Test | Ziel-Datei | Schließt |
|---|---|---|---|
| T1 | Offset-Persistenz über Bot-Neustart (Update wird nicht doppelt verarbeitet) | `internal/telegram/runtime_test.go` | H2 |
| T2 | Polling-Client bricht bei hängender Verbindung ab (Timeout) | `internal/telegram/polling_test.go` | H3 |
| T3 | Status-Watchdog: veralteter `last_successful_poll_at` ⇒ `state != healthy` | `internal/integrationstatus/` | H3/M7 |
| T4 | `handleTelegramStatus` liefert 503 bei `degraded`/`stale` | `internal/server/telegram_handlers_test.go` | H3 |
| T5 | `send_telegram` mit >4096 Units ⇒ verständlicher Fehler statt API-Fehler | `internal/agent/send_telegram_path_hardening_test.go` | M2 |
| T6 | `send_discord`/`fetch_discord` mit Kanal außerhalb der Allowlist ⇒ abgelehnt | `internal/agent/` (neu) | H1 |
| T7 | Discord-Worker-Limit: N+1 parallele Nachrichten ⇒ N laufen, eine wird abgelehnt | `internal/discord/bot_test.go` | H4 |
| T8 | `send_telegram` in `telegram.readonly` ⇒ abgelehnt | `internal/agent/` | M1 |
| T9 | Bot-Antwortweg rendert Markdown (parse_mode gesetzt) | `internal/telegram/` | M3 |
| T10 | Delivery-Fehler werden gezählt (`delivery_failures` im Status) | beide Broker-Tests | M6 |
| T11 | 429-/`retry_after`-Handling im Polling-Backoff | `internal/telegram/polling_test.go` | M5 |
| T12 | Route-Contract-Manifest enthält explizite Einträge für die Discord-/Telegram-Testrouten | `internal/audit/audit_manifests_test.go` | N9 |

---

## 7. Was bereits gut ist

Der Audit soll nicht nur Mängel listen — diese Punkte sind ausdrücklich gelungen und sollten bei Refactorings erhalten bleiben:

- **Attachment-Freigabe ist vorbildlich.** `OpenOutgoingAttachment` (`internal/tools/outgoing_attachments.go:65-95`) öffnet Dateien über ein `os.Root` innerhalb von Workspace/Dokumentordner, prüft Symlink-Austausch, reguläre Dateien und schließt den DataDir/Config-Verzeichnisse aus. Ein Prompt-Injection-Angriff kann so keine `config.yaml` oder Vault-Datei exfiltrieren.
- **Der Dokument-Versandpfad ist vollständig gehärtet:** eigener HTTP-Client mit Timeout, `telegramNoRedirect`, Größenlimit 50 MB, Caption-Kürzung auf 1024 UTF-16-Units, explizite Token-Redaktion in Fehlern, Streaming-Upload per `io.Pipe` (kein Speicher-Overhead), sauberes Goroutine-Join.
- **Injection-Abwehr ist auf beiden Kanälen gleich gut:** Guardian-Block ab `ThreatHigh`, danach `security.IsolateExternalData` zur Abgrenzung externer Inhalte; zusätzlich Guardian-Scan auf `fetch_discord`-Ergebnissen (`internal/agent/dispatch_messaging.go:142-152`).
- **Die Status-Exposition ist sauber:** `TelegramRuntimeStatus` (`internal/integrationstatus/telegram.go:8-20`) enthält bewusst **keinen** Token, keine Chat-Inhalte, keine Usernamen und keine rohen Provider-Fehler — nur Codes.
- **Polling-Lifecycle ist korrekt abgesichert:** Kontextbindung des SDK-Clients an den Server-Kontext (`internal/telegram/http_client.go`), Abbruch bei Konfigurationswechsel, Backoff mit Cap, Aufzeichnung eines operationalen Issues nach 3 Fehlern mit Fingerprint und automatisches Auflösen nach Erfolg (`internal/telegram/bot.go:108-140`).
- **Die Geheimnisbehandlung ist vorbildlich und konsistent.** Beide Tokens sind Vault-only (`yaml:"-"` + `vault:"bot_token"`, `internal/config/config_types.go:1444`, `:1595`), werden per `config_secret_migration.go:18-19` aus Klartext migriert (`telegram.bot_token` → `telegram_bot_token`, `discord.bot_token` → `discord_bot_token`), beim Start aus dem Vault gelesen (`config_migrate.go:824-826`) und sind explizit **für Python-Tools gesperrt** (`internal/tools/python_secrets.go:86-87`). Es gibt also weder einen Klartext-Leak in `config.yaml` noch einen Weg, den Bot-Token aus einem Agenten-Skript zu ziehen.
- **Der `requireAdmin`-Schutz auf allen Integrationstest-Routen** (`internal/server/server_routes_config.go:123-124`) und die getrennte, lesende Connection-Probe versus die Nachrichtentest-Route sind sauber getrennt.
- **Der zentrale Logger scrubbt alle Attributwerte** (`internal/logger/logger.go:104-110`) — ein wirksames Sicherheitsnetz gegen Secret-Leaks in Logs.

---

## 8. Roadmap

### Stufe 1 — Quick Wins (je S, zusammen ≈ 1–2 Tage)

| Maßnahme | Befund | Datei |
|---|---|---|
| Discord-Worker-Semaphore + `max_concurrent_workers` | H4 | `internal/discord/bot.go`, `internal/config/config_types.go`, `internal/config/config.go` |
| Telegram-HTTP-Timeout (90 s) im Poll-Client | H3 | `internal/telegram/bot.go:58` |
| Staleness-Watchdog + 503 bei `state != healthy` | H3, M7 | `internal/integrationstatus/telegram.go`, `internal/server/telegram_handlers.go` |
| `telegram.readonly` (Config + Dispatch + Notification) | M1 | `internal/config/config_types.go`, `internal/agent/dispatch_messaging.go`, `internal/tools/notification.go` |
| Längenprüfung 4096 im `send_telegram`-Pfad | M2 | `internal/tools/notification.go` |
| `discord.read_only` → `discord.readonly` in der Agenten-Fehlermeldung | D1 | `internal/agent/dispatch_messaging.go:101` |
| Explizite Audit-Manifest-Einträge für die vier Routen | N9 | `internal/audit/audit_manifests.go` |
| `retry_after` auswerten + Jitter im Backoff | M5, N4 | `internal/telegram/bot.go` |
| Download-Trunkierung hart fehlschlagen lassen | N1 | `internal/telegram/bot.go:884` |

### Stufe 2 — Substanziell (je M, zusammen ≈ 3–5 Tage)

| Maßnahme | Befund |
|---|---|
| Discord-Kanal-Allowlist (Config + `tools/discord_bridge.go` + Tool-Fehlermeldung + Tests) | H1 |
| Telegram-Offset persistent machen (DB-Tabelle + periodisches Schreiben + At-most-once-Entscheidung) | H2 |
| Einheitlicher Sende-Wrapper mit Retry (429/`retry_after`, 5xx) + Delivery-Zähler im Status | N2, M6 |
| Markdown-Ausgabe im Bot-Antwortweg (Konverter + `parse_mode`) | M3 |
| Fehlerbehandlung in Discord-Broker/`handleMessage` vervollständigen | M6 |
| `telegram.enabled`, Discovery-Mode für Discord, Allowlist-Listen (Mehrbenutzer) | M4, M9, M10 |

### Stufe 3 — Ausbau (je L, zusammen ≈ 1–2 Wochen)

| Maßnahme | Befund |
|---|---|
| Telegram-Webhook-Modus als Alternative zum Polling (skalierbar, kein Offset-Problem) | H2, N3 |
| Audio-/Video-/Sticker-Unterstützung auf Telegram | N5 |
| Discord-History-Recap (Memory-Manager) wie auf Telegram | M-Abschnitt 2.2 (`buildDiscordAgentMessages(nil)`) |
| Einheitliche Status-/Health-Semantik + Dashboard-Widget in der UI | H3, M7 |
| `documentation/discord_setup.md` + Korrektur von `telegram_setup.md` | D2–D4 |

---

## 9. Empfohlene Reihenfolge

1. **Sofort:** H1 (Kanal-Allowlist) — die einzige Lücke, bei der ein Angreifer direkt fremde Daten lesen oder Nachrichten posten kann.
2. **Sofort:** H4 (Discord-Limit) + H3 (Timeout/Watchdog) — verhindern Kostenexplosion und stille Totalausfälle.
3. **Danach:** H2 (Offset-Persistenz) — größter Alltagsnutzen, verhindert doppelte Läufe nach jedem Neustart.
4. **Danach:** Stufe 1 gesammelt (M1–M7, N1–N9) — Parität herstellen und die Inkonsistenzen auflösen.
5. **Abschließend:** Stufe 3 und die Testlücken T1–T12.

---

*Audit erstellt am 2026-10-24 gegen den Arbeitsstand von `C:\Users\Andi\Documents\repo\AuraGo`. Jede Aussage ist gegen die genannte Quellstelle geprüft; Drittanbieter-Verhalten wurde gegen `tgbotapi v5.5.1` und `discordgo v0.29.0` im Modul-Cache verifiziert.*
