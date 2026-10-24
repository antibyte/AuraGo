# Fix-Plan: Review-Befunde der Telegram-/Discord-Umsetzung

**Bezug:** `documentation/audits/2026-10-24-telegram-discord-implementation-report.md` (Umsetzung), `documentation/audits/2026-10-24-telegram-discord-remediation-plan.md` (Ursprungsplan)
**Anlass:** Selbst-Review der eigenen, noch nicht committeten Änderung (3 Befunde)
**Datum:** 2026-10-24
**Status:** **umgesetzt am 25.10.2026** — alle drei Befunde behoben, Tests ergänzt, siehe Abschnitt 12

---

## 1. Befundlage

| # | Befund | Datei | Schwere |
|---|---|---|---|
| 1 | Cursor wird bei einem Burst von 2–9 Updates nicht persistiert → Replay nach Neustart | `internal/telegram/bot.go:187` | mittel |
| 2 | 429-Retry blockiert den Notification-Pfad bis ~60 s | `internal/tools/notification.go:332-357` | niedrig |
| 3 | Delivery-Zähler erfasst zwei Telegram-Sendepfade nicht | `internal/telegram/bot.go:512, 520` | niedrig |

### 1.1 Korrektur an der eigenen Befundbeschreibung

Die erste Fassung von Befund 1 war **falsch formuliert**. Sie behauptete, der 30-S-Zweig würde im Leerlauf
nie ausgewertet, weil die Update-Schleife nicht betreten wird. Das trifft nicht zu: der Long Poll läuft
bis zu 60 s (`u.Timeout = 60`), deshalb ist `time.Since(lastPersist)` beim **ersten** Update nach einer
Pause immer ≥ 30 s — der Zeit-Zweig feuert dort zuverlässig.

Die Timeline des Pollers (`internal/telegram/bot.go:117-194`):

```
t=0     lastPersist = now;  GetUpdates blockiert bis zu 60 s
t=60    Update #1 → pending=1 → time.Since(lastPersist)=60 s ≥ 30 s → PERSIST ✅
        lastPersist = t=60, pending = 0
        Update #2..#9 (gleicher Batch, Millisekunden später)
        → pending<10 UND time.Since(lastPersist)<30 s → KEIN PERSIST ❌
t=120   GetUpdates liefert leer → Update-Schleife wird nicht betreten
        → der ausstehende Cursor bleibt nur im Speicher
```

**Korrekter Auslöser:** ein **Batch von 2–9 Updates, dem keine weitere Nachricht folgt**. Der Cursor
steht dann auf dem Wert des ersten Updates; die Updates 2–9 werden nach einem Neustart erneut
zugestellt und vom Agenten erneut verarbeitet.

**Konkret:** der Nutzer schickt drei Sätze schnell hintereinander („Wie ist das Wetter?", „und morgen?",
„danke") → alle drei in einem Long-Poll-Batch → nur Update #1 wird geschrieben → AuraGo startet neu →
alle drei Nachrichten werden ein zweites Mal beantwortet.

Der Einzelnachricht-Fall (der häufigste) ist **nicht** betroffen — er wird bereits persistiert. Damit
bleibt der Befund ein realer, aber engerer Defekt als zunächst beschrieben.

---

## 2. Fix 1 — Cursor nach jedem Update persistieren

### 2.1 Ziel

Der Cursor wird nach **jedem** an den Worker übergebenen Update geschrieben. Das Replay-Fenster
beträgt damit 0 Updates; die Übergabe-Semantik („Offset rückt nach der Übergabe vor") bleibt unverändert.

### 2.2 Änderung

`internal/telegram/bot.go:101-105` — Schwelle von 10 auf 1:

```go
const (
	// telegramOffsetPersistEveryUpdates and telegramOffsetPersistInterval
	// bound how long a handed-out cursor may stay unpersisted.
	telegramOffsetPersistEveryUpdates = 1
	telegramOffsetPersistInterval     = 30 * time.Second
```

`internal/telegram/bot.go:183-190` — Kommentar korrigieren und Bedingung vereinfachen:

```go
			// The cursor advances only after the update was handed to the
			// worker, so a crash may lose at most this one message.
			// Persisting after every update keeps a restart from replaying
			// the rest of the batch.
			updateCfg.Offset = update.UpdateID + 1
			pending++
			if persist != nil && (pending >= telegramOffsetPersistEveryUpdates || time.Since(lastPersist) >= telegramOffsetPersistInterval) {
```

Der `pending`-Zähler und der 30-S-Zweig bleiben als Sicherheitsnetz bestehen (sie greifen, falls ein
Persist fehlschlägt).

### 2.3 Nebenwirkungsanalyse

| Aspekt | Bewertung |
|---|---|
| Erfolgsfall | unverändert — der Cursor lag im Einzelnachricht-Fall schon auf der Platte |
| Dateischreibvorgänge | 1 zusätzlicher JSON-Roundtrip (Read+Write, ~100 B) **pro Update** statt alle 10 Updates. Die Telegram-Roundtrip-Zeit (≥ 100 ms) dominiert; der Schreibvorgang liegt bei ~0,1 ms. Kein messbarer Durchsatzverlust. |
| Zustellgarantie | Wechsel von „at-least-once" zu „at-most-once" **für die Updates 2–9 eines Batches**. Das Update #1 verhält sich heute bereits so (Offset rückt bei Übergabe vor). Die Änderung macht das Verhalten **einheitlich** und deckt sich mit dem bestehenden Codekommentar sowie dem Plan-Kriterium T-1 („Update wird nicht doppelt verarbeitet"). |
| Nebenläufigkeit | unverändert — der Persist läuft im Poller-Goroutine, `FileOffsetStore.Save` ist mutex-geschützt (`internal/telegram/offsets_store.go:88`) |
| Cursor-Datei | gleiches Format, gleiche atomare Schreibweise, gleiche Rechte (0600) |

### 2.4 Tests

`internal/telegram/runtime_test.go` (neu):

1. `TestTelegramCursorIsPersistedForEveryUpdateOfABurst` — Batch mit 3 Updates; der Persist-Callback
   muss die Offsets 2, 3, 4 **jeweils einzeln** melden (heute nur 2).
2. `TestTelegramCursorSurvivesRestartWithoutReplay` — Batch mit 3 Updates → letzter persistierter Offset
   4; ein neuer Poller mit diesem Offset liefert keine Updates mehr.

Bestehender Test `TestTelegramCursorIsPersistedWhilePolling` (Batch von 12) bleibt gültig und deckt
zusätzlich die 10er-Schwelle ab, falls die Schwelle später wieder angehoben wird.

---

## 3. Fix 2 — Gesamtbudget für den Flood-Control-Retry

### 3.1 Ziel

Ein 429 darf den aufrufenden Pfad nicht länger unbeschränkt blockieren. Obergrenze: **30 s** für die
gesamte Sendung inklusive Wartezeiten.

### 3.2 Änderung

`internal/tools/notification.go` — neues Budget als Variable (testbar, Muster wie
`discordWorkerWaitTimeout`):

```go
// telegramSendTotalBudget bounds a whole Telegram send including the
// flood-control waits. A throttled API must not block the caller for
// minutes.
var telegramSendTotalBudget = 30 * time.Second
```

In `sendTelegramNotification` (aktuell Zeile 308-359) den Kontext mit dem Budget begrenzen und im
Abbruchfall den **letzten Telegram-Fehler** zurückgeben, nicht den Kontextfehler:

```go
	ctx, cancel := context.WithTimeout(nativeHTTPContext(contexts...), telegramSendTotalBudget)
	defer cancel()
	...
	for attempt := 1; attempt <= telegramSendMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}
		...
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-time.After(telegramRetryWait(statusErr.retryAfter, attempt)):
		}
	}
```

### 3.3 Nebenwirkungsanalyse

| Aspekt | Bewertung |
|---|---|
| Erfolgsfall | unverändert (Sendeantwort in < 1 s, kein Timeout) |
| Aufrufer mit kürzerer Deadline | gewinnt weiterhin — `WithTimeout` nimmt das Minimum |
| Worst Case | vorher ~60 s (2 Wartezeiten à 30 s), jetzt **30 s** |
| Betroffene Aufrufer | `tools.SendNotification` läuft mit `context.Background()` (`internal/tools/notification.go:62`), also ohne Deadline: der Agent-Dokumentpfad (`internal/tools/telegram_document.go:362` → `SendTelegramFile`) blockiert bis zu 30 s statt 60 s; der Flow-Pfad (`internal/server/flows_bridge_notify.go:150`) läuft auf eigener Goroutine mit Slot-Limit |
| Doppelversand | unverändert nicht möglich — es wird nur bei 429 wiederholt, und diese Antwort sagt ausdrücklich, dass die Nachricht **nicht** angenommen wurde |
| Fehlermeldung | der Agent sieht weiterhin den Telegram-Fehler („telegram returned HTTP 429 …"), nicht „context deadline exceeded" |

### 3.4 Tests

`internal/tools/notification_test.go` (neu):

`TestSendTelegramNotificationStopsAtTheRetryBudget` — Testserver antwortet immer mit 429 und
`retry_after: 60`; `telegramSendTotalBudget` wird im Test auf 50 ms gesetzt. Erwartung: die Funktion
kehrt binnen ~1 s mit einem Fehler zurück und hat **höchstens 2** Anfragen gestellt.

---

## 4. Fix 3 — Delivery-Zähler für alle Telegram-Sendepfade

### 4.1 Ziel

Jeder verlorene Telegram-Versand wird in `delivery_failures` sichtbar — auch die Kommando-Antwort und
die Guardian-Sperrnotiz.

### 4.2 Änderung

`internal/telegram/bot.go:700-712` — `deliverMessage` um einen Kontext-Parameter erweitern (Muster wie
`DiscordBroker.deliver(text, what)` in `internal/discord/bot.go:1114`):

```go
func deliverMessage(bot *tgbotapi.BotAPI, chatID int64, text string, what string, logger *slog.Logger) error {
	err := sendTelegramMessage(bot, chatID, text)
	if err != nil {
		integrationstatus.MarkTelegramDeliveryFailure(time.Now())
		if logger != nil {
			logger.Error("Failed to send Telegram "+what, "error", security.Scrub(err.Error()))
		}
	}
	return err
}
```

Die drei Aufrufer anpassen:

| Zeile | vorher | nachher |
|---|---|---|
| `bot.go:512` | `sendTelegramMessage(bot, msg.From.ID, cmdResult)` + eigenes `logger.Error` | `deliverMessage(bot, msg.From.ID, cmdResult, "command result", logger)` |
| `bot.go:520` | `sendTelegramMessage(bot, msg.From.ID, "Your message was blocked by the security guardian.")` + eigenes `logger.Error` | `deliverMessage(bot, msg.From.ID, "…", "security block notice", logger)` |
| `bot.go:695` | `deliverMessage(bot, msg.From.ID, cleanText, logger)` | `deliverMessage(bot, msg.From.ID, cleanText, "answer", logger)` |

`TelegramBroker.deliver` (Zeile 914) zählt bereits — bleibt unverändert.

### 4.3 Nebenwirkungsanalyse

| Aspekt | Bewertung |
|---|---|
| Zählung | wird von 1 auf 3 Sendepfade erweitert; `delivery_failures` steigt in Szenarien, die vorher unbgezählt blieben — **das ist der Zweck** |
| Logtext | die Meldung „Failed to send Telegram message" wird zu „Failed to send Telegram command result" / „… security block notice" / „… answer". Präziser, aber ein Textwechsel in Logs (kein Parser hängt daran) |
| API | `deliverMessage` ist package-privat; alle drei Aufrufer liegen in derselben Datei → keine externe Breite |

### 4.4 Tests

`internal/telegram/polling_test.go` oder neue Datei (neu):

`TestTelegramDeliveryFailureCounterCountsEverySendPath` — erzwungener Sendefehler in jedem der drei
Pfade → `integrationstatus.TelegramStatus().DeliveryFailures` steigt um 1 pro Fehler,
`LastDeliveryErrorAt` ist gesetzt.

---

## 5. Doku-Nachträge

| Datei | Änderung |
|---|---|
| `documentation/telegram_setup.md` | Absatz „Message length" / Cursor-Satz: die Persistenz-Aussage präzisieren — der Cursor wird nach **jedem** Update geschrieben, nicht „alle 10 Updates / 30 s". |
| `documentation/audits/2026-10-24-telegram-discord-implementation-report.md` | neuer Abschnitt 7: Selbst-Review, die drei Befunde, die Korrektur an Befund 1, der Fix-Stand und die neue Testlage. Abschnitt 6.1 wird um die korrigierte Cursor-Zusage ergänzt. |
| `documentation/audits/2026-10-24-telegram-discord-remediation-plan.md` | Umsetzungsstand um die Nachbesserung ergänzen (Plan-Kriterium T-1 wird mit Fix 1 nachträglich erfüllt). |
| `documentation/audits/2026-10-24-telegram-discord-finding-fixplan.md` | dieses Dokument — nach Umsetzung auf „umgesetzt" setzen und mit dem Commit-Hash versehen. |

---

## 6. Reihenfolge und Commit-Struktur

| # | Commit | Inhalt | Prüfung vor dem Commit |
|---|---|---|---|
| A | `fix(telegram): persist the update cursor after every update` | Fix 1 + Tests + Kommentar | `go test ./internal/telegram/... -count=1` |
| B | `fix(tools): bound the telegram flood-control retry` | Fix 2 + Test | `go test ./internal/tools/ -run 'Telegram|Notification' -count=1` |
| C | `fix(telegram): count every failed telegram delivery` | Fix 3 + Test | `go test ./internal/telegram/... -count=1` |
| D | `docs(telegram): correct the cursor persistence guarantee` | Doku-Nachträge + Bericht Abschnitt 7 | — |

Nach Commit D: Gesamtverifikation (siehe Abschnitt 8).

**Reihenfolge-Begründung:** A zuerst, weil er das dokumentierte Versprechen wieder einlöst und die
übrigen Fixes unabhängig davon sind; B und C sind unabhängig voneinander und beide klein.

---

## 7. Risikoübersicht

| Fix | Risiko | Gegenmaßnahme |
|---|---|---|
| 1 | Wechsel der Zustellgarantie für Batch-Updates 2–9 (von at-least-once zu at-most-once) | einheitlich mit dem bereits akzeptierten Verhalten für Update #1; deckt sich mit Codekommentar und Plan-Kriterium T-1; Test deckt den Burst explizit ab |
| 1 | Mehr Dateischreibvorgänge | Größenordnung unkritisch (JSON ~100 B gegen Telegram-Roundtrip ≥ 100 ms) |
| 2 | Abbruch eines legitimen Flood-Control-Retrys, wenn Telegram länger als 30 s braucht | Der Aufrufer bekommt den Telegram-Fehler und kann es später erneut versuchen; keine Nachricht geht verloren (Telegram hat sie nicht angenommen) |
| 3 | Geänderte Logtexte | kein Parser hängt an den Meldungen |

---

## 8. Verifikation nach der Umsetzung

```powershell
go build ./...                                                        # Exit 0
gofmt -l <alle geänderten Go-Dateien>                                 # leer
go vet ./internal/telegram/... ./internal/tools/...                   # Exit 0
go test ./internal/telegram/... -count=1                              # grün
go test ./internal/tools/ -run 'Telegram|Notification|Markdown' -count=1   # grün
go test ./internal/server/ -run 'Telegram' -count=1                   # grün
go test ./internal/agent/ -run 'Discord|Telegram|Notification' -count=1    # grün
go test ./internal/ui/ -count=1                                       # grün (falls UI betroffen)
node scripts/build-ui-bundles.js --check                              # Exit 0
```

Voller `internal/server`-Lauf wie bisher nur mit `GOTMPDIR` auf demselben Volume wie das Repository
(Windows-Umgebungseffekt, siehe Implementierungsbericht Abschnitt 4).

---

## 9. Nicht in diesem Fix-Umfang

| Punkt | Begründung |
|---|---|
| Allowlist für den Inbound-Pfad und den Bot-eigenen Antwortweg | bewusst so entschieden und in `documentation/discord_setup.md` Abschnitt 3 korrekt beschrieben |
| Discord-Bot-Neustart bei jeder Config-Speicherung | vorbestehend (der `==`-Vergleich enthielt das Vault-Token ebenfalls), durch `reflect.DeepEqual` nicht verursacht und nicht verschärft |
| „echtes" at-least-once: Cursor erst nach Worker-Abschluss schreiben | sauberere Langzeitlösung, erfordert aber einen neuen Abschluss-Rückkanal aus dem Worker (`internal/telegram/runtime.go:186`) und eine Offset-Berechnung über Out-of-order-Abschlüsse → als **Folge-Option** festgehalten, nicht jetzt umgesetzt |
| `ReadOnlyGate`-Text im Audit-Manifest vereinheitlichen | kosmetisch; kein Parser wertet das Feld aus |

---

## 10. Impact-Analyse (graft)

| Symbol | Aufrufer | Bewertung |
|---|---|---|
| `runPollingLoop` (`internal/telegram/bot.go:117`) | `startLongPolling` (`runtime.go:120`) → `StartLongPolling` (`bot.go:65`); Tests: `polling_test.go` (4), `runtime_test.go` (4) | geschlossen, nur die Persist-Schwelle ändert sich; keine Signaturänderung |
| `sendTelegramNotification` (`internal/tools/notification.go:308`) | `SendNotificationContext` → `SendNotification` → `sendTelegramFileText` (`telegram_document.go:362`) → `SendTelegramFile`; außerdem `flowSendNotification` (`internal/server/flows_bridge_notify.go:136`) und die Person-Detect-Alerts | keine Signaturänderung; die neue Budget-Timeout wirkt nur im Fehlerfall |
| `persistTelegramOffset` (`internal/telegram/runtime.go:202`) | `runtime.go:193` (Persist-Callback), `runtime.go:195` (nach Polling-Ende) | unverändert |
| `deliverMessage` (`internal/telegram/bot.go:703`) | `bot.go:484`, `:512`, `:516`, `:522`, `:622`, `:695` (sechs Aufrufer nach der Umsetzung) | package-privat, alle Aufrufer in derselben Datei; Signatur wurde um `what string` erweitert |
| `FileOffsetStore.Save` (`internal/telegram/offsets_store.go:88`) | `UpdateOffsets.Save` (`runtime.go:86`) | unverändert, mutex-geschützt |

*Hinweis:* Der Graft-Graph löst `deliverMessage` auf `internal/webhooks/handler.go:502` auf
(Namenskollision). Die Telegram-Aufrufer wurden deshalb per Textsuche in `internal/telegram/bot.go`
verifiziert.

---

## 11. Abnahmekriterien

1. Ein Batch von 3 Telegram-Updates löst **drei** Persist-Aufrufe mit den Offsets 2, 3, 4 aus.
2. Ein Neustart nach diesem Batch liefert **keine** der drei Nachrichten erneut.
3. `sendTelegramNotification` kehrt bei dauerhaftem 429 binnen `telegramSendTotalBudget` (Default 30 s)
   mit dem Telegram-Fehler zurück.
4. Ein fehlgeschlagener Versand über einen der drei Telegram-Pfade erhöht `delivery_failures` um 1.
5. `go build ./...`, `gofmt`, `go vet` und alle betroffenen Testpakete sind grün.
6. Die Cursor-Zusage in `documentation/telegram_setup.md` entspricht exakt dem implementierten Verhalten.

---

## 12. Umsetzungsstand (25.10.2026)

Alle drei Befunde sind behoben und mit Tests abgesichert.

| Fix | Umsetzung | Test |
|---|---|---|
| 1 | `internal/telegram/bot.go:101-107` — Schwelle 10 → 1; Kommentar `:185-193` auf die tatsächliche Semantik korrigiert | `TestTelegramCursorIsPersistedWhilePolling` (angepasst: erwartet 2, 3, 4), `TestTelegramCursorSurvivesRestartWithoutReplay` (neu) |
| 2 | `internal/tools/notification.go:299-304` (Budget-Variable), `:335-339` (Timeout), `:342-377` (Rückgabe des Telegram-Fehlers statt des Deadline-Fehlers) | `TestSendTelegramNotificationStopsAtTheRetryBudget` (neu) |
| 3 | `internal/telegram/bot.go:700-716` — `deliverMessage` mit `what`; sechs Aufrufer umgestellt (Reply-Aufforderung, Kommando-Fehler, Kommando-Antwort, Guardian-Sperrnotiz, Fehlernotiz, Antwort) | `TestTelegramDeliveryFailureCounterCountsEverySendPath` (neu), `TestTelegramSendsAllPassThroughTheDeliveryCounter` (neu, Wächter) |

**Zusätzlicher Wächter:** `TestTelegramSendsAllPassThroughTheDeliveryCounter` liest `bot.go` und stellt sicher,
dass `sendTelegramMessage` nur in `deliverMessage` und im Broker aufgerufen wird. Ein künftiger direkter
Versand würde damit sofort auffallen — genau der Defekt, den Befund 3 beschrieben hat.

### Abnahmekriterien

| # | Kriterium | Ergebnis |
|---|---|---|
| 1 | Ein Batch von 3 Updates löst drei Persist-Aufrufe mit 2, 3, 4 aus | ✅ erfüllt |
| 2 | Ein Neustart nach diesem Batch liefert keine Nachricht erneut | ✅ erfüllt |
| 3 | `sendTelegramNotification` kehrt bei dauerhaftem 429 im Budget mit dem Telegram-Fehler zurück | ✅ erfüllt |
| 4 | Ein fehlgeschlagener Versand erhöht `delivery_failures` um 1 | ✅ erfüllt |
| 5 | `go build`, `gofmt`, `go vet` und alle betroffenen Testpakete grün | ✅ erfüllt |
| 6 | Cursor-Zusage in `documentation/telegram_setup.md` entspricht dem Verhalten | ✅ erfüllt |

### Doku-Nachträge

- `documentation/telegram_setup.md` — Cursor-Satz und Flood-Control-Absatz an das neue Verhalten angepasst.
- `documentation/audits/2026-10-24-telegram-discord-implementation-report.md` — Abschnitt 6.1 um die
  Cursor- und Budget-Zusage ergänzt, neuer Abschnitt 7 „Selbst-Review und Nachbesserung" mit der
  Korrektur der Befundbeschreibung.
- `documentation/audits/2026-10-24-telegram-discord-remediation-plan.md` — Umsetzungsstand um die
  Nachbesserung ergänzt.
