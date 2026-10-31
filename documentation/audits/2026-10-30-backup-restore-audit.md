# Audit: Backup- und Restore-Funktionen (2026-10-30)

Nur gelesen, nichts geändert. Zeilennummern beziehen sich auf `main` bei `e83e79752` mit Working Tree.

## Status (2026-10-31)

Welle 1 ist auf dem lokalen main committet, nicht gepusht und nicht deployed.

| Befund | Commit | Kern der Änderung |
| --- | --- | --- |
| B3, B4 | `e886e9817` | Body bis 64 KiB, 400 bei Übergröße, ungültigem JSON und Passwort > 1024 Byte; mit Passwort bricht ein Vault- oder Token-Exportfehler mit 500 ab |
| M1 | `97d27bd15` | Marker ohne Schreibzustand werden gelöscht; Chunk-Hash für Fakten > 4000 Zeichen; `NewSQLiteMemory` räumt beim Start Marker früherer Läufe ab (WARN mit Doc-IDs); Backup antwortet 503 über `ErrMemorySnapshotBlocked` |
| U1, U2 | `88b427132` | `abort_update` spielt Prompt-, Skill- und Tool-Kopien zurück; Recovery-Zustand für INT/TERM/Exit nach dem Stopp; Restore-Helfer ohne `errexit`-Abhängigkeit; `--no-restart` endet `rolled_back` |

Tests:

- Memory-Paket komplett grün.
- Backup-Tests im Server-Paket grün.
- Updater-Tests (`internal/audit`, `update`) unter Linux im Container `golang:1.27.3` grün, ohne Skips.

### Welle 2 (2026-10-31)

Welle 2 ist auf dem lokalen main (`7a2ecfab2..93ee8ddee`), nicht gepusht und nicht deployed.

| Befund | Commit | Kern der Änderung |
| --- | --- | --- |
| B1 | `7a2ecfab2` | 12 weitere Datenbanken, Laufzeitdateien, `data/documents/`, Agent-Skills (`agent_workspace/agent_skills/`); Ausnahmen dokumentiert (tsnet, Modelle/Caches, Suchindex); Guard-Test `TestSQLiteOpensAreBackedUpOrExcluded` |
| B2, B7 | `b0ae39698`, `82aead839`, `93ee8ddee` | Import validiert und stellt nur bereit (`data/.restore-pending`); Anwendung beim nächsten Start nach dem Instanz-Lock, vor Config und Datenbanken; Rollback-Journal mit `rollback-complete`-Marker (crash-sicher); Ziele werden aus relativen Pfaden neu berechnet und erneut geprüft; Sidecars werden entfernt; Ergebnis in `data/.restore-last.json`; Vault-Secrets werden ebenfalls bereitgestellt und nur nach erfolgreicher Anwendung übernommen; `GET/DELETE /api/backup/restore-status`; Manifest mit `app_version` und `user_version` je DB, neuere Backups → 400; SQLite-Snapshots bis zur Gesamtgrenze, übergroße Dokumente werden übersprungen |
| B8 | `85469dc88`, `ab5350f5c` | Bestätigung vor dem Import, Teilerfolg als Warnung, Neustart-Knopf nur bei `restart_required`, Status der bereitgestellten Wiederherstellung mit „Verwerfen“, robuste Fehleranzeige; 16 Sprachen |
| Doku | `0fcd2de5d` | Handbuch en/de: API-Referenz, Anhang, Schlüsselrotation, Tresor |

Tests:

- `restorepending`, `config`, `security`, `i18n` und `ui` grün.
- Backup/Restore/WebAuthn-Tests im Server-Paket grün, `internal/audit` komplett grün.
- Ein echter Neustart mit Wiederherstellung ist **nicht** geprüft. Das bleibt Abnahme auf aurago-test.

Neu offen aus dem Review:

- `snapshot.publication.correction.*`: Eine fehlgeschlagene Korrektur behält ihren Marker, bis dieselbe Korrektur erneut angewendet wird (`correction_apply.go:129-137`). Wird sie nie wiederholt, bleiben Snapshots gesperrt.
- `abort_before_file_changes` endet weiterhin `uncertain` und blockiert das nächste Update (Altlast, nicht durch Welle 1 entstanden).

## Umfang

| Bereich | Dateien |
| --- | --- |
| System-Backup (`.ago`) | `internal/server/backup_handlers.go`, `backup_validation.go`, `backup_crypto.go`, `backup_sqlite.go`, `backup_token_store.go`, `internal/config/sqlite_paths.go`, `internal/security/vault.go` (`BackupSnapshot`/`RestoreBackupSnapshot`), `ui/cfg/backup.js` |
| Memory-Snapshots | `internal/memory/snapshot_coordinator.go`, `snapshot_publication.go`, `automatic_snapshot.go`, `sqlite_backup.go`, `vector_ownership.go`, `automatic_write.go`, `internal/agent/maintenance.go` |
| Update-Rollback | `update.sh`, `repair-update.sh`, `internal/upkeep/*.go`, `internal/tsnetnode/tsnetnode.go` |

Nicht im Umfang: Revisions-Restores einzelner Apps (Game Maker, Flows, Homepage, Video Studio, CYD Creator) sowie Infra-Snapshots (TrueNAS, Proxmox).

Status je Befund:

- **bestätigt**: am Code nachgeprüft.
- **plausibel**: Der Codepfad ist belegt, die Laufzeitwirkung ist nicht reproduziert.

## Kurzfazit

Gut gelöst ist die Kryptografie: AES-256-GCM, Argon2id mit 64 MiB, frischer Salt und Nonce. Ebenfalls gut sind die Pfadprüfung beim Import (Zip-Slip, Symlinks, Zip-Bomb-Grenzen, Staging vor dem Schreiben) und der koordinierte Lese-Snapshot der drei Memory-Datenbanken.

Die Schwächen liegen fast alle in der **Betriebssicherheit**:

- Ein Backup kann stillschweigend unvollständig sein.
- Ein Restore ist nicht transaktional und gleicht den laufenden Prozess nicht ab.
- Ein hängengebliebener Memory-Marker kann jedes weitere Backup dauerhaft blockieren.
- Ein abgebrochenes Update verliert lokale Prompt- und Skill-Änderungen.

## Prioritäten

| # | Befund | Schwere |
| --- | --- | --- |
| 1 | M1 Hängender Publikationsmarker blockiert alle Memory-Snapshots **und** das manuelle Backup | High |
| 2 | U1 Abgebrochenes Git-Update verwirft lokale Änderungen an Prompts/Skills/Tools endgültig | High |
| 3 | B1 Backup enthält mehrere Datenbanken und Verzeichnisse nicht | High |
| 4 | B2 Restore nicht transaktional, laufender Prozess nicht abgeglichen | High |
| 5 | B3 Vault-/Token-Export scheitert still, Backup gilt trotzdem als vollständig | High |
| 6 | B4 Passwort über ~1 KB → stillschweigend **unverschlüsseltes** Backup | Medium |
| 7 | U2 Strg-C/SIGTERM während des Updates lässt AuraGo gestoppt zurück | High |

---

## A. System-Backup und -Restore (`.ago`)

### B1 — High — Backup ist unvollständig (bestätigt)
`internal/config/sqlite_paths.go:38-88` listet die gesicherten SQLite-Datenbanken. `backup_handlers.go:635-641` listet die gesicherten Laufzeitdateien. Mehrere Daten, die die App schreibt, fehlen in beiden Listen.

Fehlende Datenbanken:
- `detective.db` (`sqlite.detective_path`)
- `data/mailer/mailer.db` (`internal/mailer/service.go:77`)
- `data/huggingface.db` (`internal/huggingface/store.go:42`)
- `data/manus.db` (`internal/manus/runtime.go:491`)
- `data/sip_calls.db` (`internal/sipphone/manager.go:176`)

Fehlende Dateien:
- `data/core_memory.md`
- `data/github_trust.json`
- `data/speech_lab/deployment.json`
- tsnet-Zustand

Fehlende Verzeichnisse:
- `directories.agent_skills_dir` wird beim Erstellen nie durchlaufen
- `data/documents`

**Szenario:** Ein Umzug auf neue Hardware mit „vollständigem“ Backup. Danach fehlen Mails, Detective-Fälle, Anrufliste und Agent-Skills, ohne Hinweis.

**Empfehlung:** Die Liste aus einer zentralen Registry ableiten statt sie von Hand zu pflegen. Einen Test ergänzen, der jede `dbutil.Open`-/`OpenStore`-Stelle unter `data/` gegen die Backup-Liste prüft. Bewusste Auslassungen dokumentieren.

### B2 — High — Restore ist nicht transaktional und gleicht den laufenden Prozess nicht ab (bestätigt; WAL-Folge plausibel)
`backup_handlers.go:1177-1199` veröffentlicht Datei für Datei mit `agoAtomicCopyFile`. Scheitert Datei N, sind `config.yaml` und ein Teil der Datenbanken bereits ersetzt. Es gibt weder Rollback noch Journal.

Weitere Lücken im selben Pfad:
- Die In-Memory-Config wird nur im Vault-Zweig neu geladen (`:1216-1223`). Speichert die UI danach die Config, schreibt sie den alten Stand über die wiederhergestellte Datei.
- Alte `-wal`/`-shm`-Dateien neben ersetzten Datenbanken werden nicht entfernt. Nach einem Absturz vor dem Neustart würde SQLite eine fremde WAL auf die wiederhergestellte DB anwenden (plausibel).
- Unter Windows kann das Umbenennen über eine offene DB mit einer Sharing-Violation scheitern. Das führt direkt in den halben Restore.
- `memory_consistency` aus dem Manifest wird beim Import nicht geprüft. Der Memory-Koordinator wird für den Austausch nicht eingefroren.

**Empfehlung:**
- Vor dem Veröffentlichen eine Sicherung der überschriebenen Dateien anlegen und bei Fehlern zurückspielen. Besser noch: den Restore nur „vormerken“ und beim nächsten Start vor dem Öffnen der Datenbanken ausführen.
- Sidecars löschen.
- Bei jedem Restore `restart_required` setzen.

### B3 — High — Vault-/Token-Export scheitert still (bestätigt)
Fehler aus `exportVaultSecrets`/`exportTokenStore` werden nur geloggt (`backup_handlers.go:793-795`, `:810-812`). Die Antwort ist trotzdem HTTP 200 mit Download.

Die UI zeigt „Vault-Secrets werden eingeschlossen“, sobald ein Passwort eingegeben ist (`backup.js:10-17`). Antwort-Header oder Manifest melden nicht, was tatsächlich enthalten ist.

**Szenario:** Der Vault ist gesperrt. Das Backup sieht vollständig aus, enthält aber keinen einzigen API-Schlüssel.

**Empfehlung:** Mit Passwort hart abbrechen oder `X-AuraGo-Backup-Contents` setzen und den Inhalt in der UI anzeigen.

### B4 — Medium — Langes Passwort ergibt ein unverschlüsseltes Backup (bestätigt)
`backup_handlers.go:449-450` liest den Body mit `io.ReadAll(io.LimitReader(r.Body, 1024))` und verwirft den Fehler von `json.Unmarshal`. Ein Body über 1 KB wird abgeschnitten, das JSON ist ungültig und `Password` bleibt leer. Heraus kommt ein Klartext-Backup samt `data/` und Datenbanken.

Die einzige Bremse ist die Klartext-Secret-Prüfung der Config (`:564`).

**Empfehlung:** Ein größeres Limit setzen, den Decode-Fehler mit 400 beantworten und eine Mindestlänge für das Passwort festlegen.

### B5 — Medium — Ausschlussfilter vergleicht den absoluten Pfad (bestätigt)
`backup_handlers.go:536-541` prüft `strings.Contains(filepath.ToSlash(path), ex)` auf den vollen Pfad. Liegt die Installation z. B. unter `.../images/AuraGo/`, trifft `/images/` jede Datei. Das Workdir wird dann leer gesichert, und 0 Dateien gelten nicht als Fehler.

**Empfehlung:** Auf dem relativen Pfad (`rel`) filtern.

### B6 — Medium — Eigene Verzeichnisse landen beim Restore am falschen Ort (bestätigt)
Beim Erstellen werden feste Präfixe geschrieben (`prompts/`, `agent_workspace/skills|tools|workdir/`, `:579/635/756/767/778`). Beim Restore wird immer unter `configRoot` geschrieben (`:1069`).

Ist `tools_dir` oder `workspace_dir` umkonfiguriert, landen die Dateien dort, wo die laufende Config nicht liest.

Eine DB außerhalb von `configRoot` wird nur geloggt und übersprungen (`:696-703`). Eine DB innerhalb von `configRoot`, aber außerhalb der Allowlist, lässt den ganzen Import mit 400 scheitern (`:1063-1067`).

### B7 — Medium — Versionsprüfung ist wirkungslos (bestätigt)
`currentDBSchemaVersion = 1` (`:58`) ist eine Konstante ohne Bezug zu den echten Schemata (mailer 8, bookshelf 6, Memory v17). Die Warnung bei Abweichung feuert deshalb nie.

Ihr Text verspricht automatische Migrationen. Mailer verweigert aber eine neuere DB (`internal/mailer/store.go:51-52`).

Manifest-Decode-Fehler werden verschluckt (`:1056`). Das Manifest enthält keine App-Version.

**Empfehlung:** App-Version und `PRAGMA user_version` je DB ins Manifest schreiben. Beim Import warnen oder blockieren, wenn ein Backup neuer ist als der laufende Build.

### B8 — Medium — Restore-UI (bestätigt)
`ui/cfg/backup.js:187-230`:

- Vor dem Überschreiben wird nicht per `confirm()` nachgefragt.
- `status: "ok"` mit grüner Erfolgsbox erscheint auch, wenn der Vault- oder Token-Import scheiterte.
- „Jetzt neu starten“ wird immer angezeigt, `restart_required` wird ignoriert.
- `resp.json()` wirft bei der Nicht-JSON-Antwort des `MaxBytesReader` (`:901`). Der Nutzer sieht dann einen Parserfehler.

`skipped` wird nie hochgezählt (`:977`, `:1285`).

### B9 — Medium — Restore vertraut dem Archiv vollständig (bestätigt; Designfrage)
Ein unsigniertes Klartext-`.ago` darf `config.yaml`, `prompts/`, `agent_workspace/tools/` und `agent_workspace/skills/` ersetzen (`backup_validation.go:11-29`). Es gibt keine inhaltliche Prüfung: `auth.enabled=false` oder ausführbare Skills werden übernommen.

Ein verschlüsseltes Archiv bringt außerdem `password_hash` (nur `auth_session_secret` wird ausgelassen) und einen kompletten `tokens.json` mit. Der Autor des Archivs kann darin ein Admin-Token mit bekanntem Klartext hinterlegen.

Beim eigenen Backup ist genau das gewollt. Bei einem fremden Archiv („Vorlage“, „Migration“) bedeutet es Übernahme der Instanz.

**Empfehlung:** Vor dem Restore sicherheitsrelevante Änderungen anzeigen und bestätigen lassen (Auth an/aus, Bind-Adresse, neue Skills/Tools, Passwort-Hash, Token-Anzahl). Token-Scopes validieren.

### B10 — Medium — Ohne Login: Vault-Export als Entschlüsselungsorakel (bestätigt; nur `auth.enabled=false`)
`requireAdmin` lässt bei deaktiviertem Login alles durch (`auth.go:480`). Die CSRF-Prüfung vergleicht Origin nur mit Host (`trusted_proxy.go:104`). Per DNS-Rebinding kann eine fremde Webseite deshalb `/api/backup/create` mit selbst gewähltem Passwort aufrufen und alle Vault-Secrets entschlüsseln.

Das ist ein Querschnittsproblem für alle Endpunkte bei abgeschaltetem Login, beim Backup aber besonders folgenreich.

**Empfehlung:** Eine Host-Allowlist einführen, oder Backup-Endpunkte ohne aktiven Login verweigern.

### B11 — Low — Sonstiges

| ID | Befund | Stelle |
| --- | --- | --- |
| B11a | Eine Datei > 100 MiB oder eine Auswahl > 1 GiB lässt das ganze Backup mit generischem 500 scheitern | `:481-486`, `:747-786` |
| B11b | Das ganze Archiv liegt 2–3-fach im RAM; das Staging liegt im System-TEMP statt unter `configRoot` | `:456-461`, `:653`, `:922-958` |
| B11c | Nur das Memory-Trio wird gemeinsam gesichert; andere DB-Paare (Game Maker/Flows) können zeitlich auseinanderlaufen; JSON-Dateien werden live gelesen | `:688-735`, `:503-506` |
| B11d | Legacy-KDF (65.535 × SHA-256) bleibt Fallback beim Entschlüsseln; Brute-Force alter Archive ist billig | `:156-168`, `:201-240` |
| B11e | Datei-Modi aus dem Archiv werden übernommen (0777 möglich auf POSIX) | `:1095`, `:1148-1154` |
| B11f | Kein Throttling bei falschen Import-Passwörtern | `:920-952` |
| B11g | Nach einem Absturz bleibt das Verzeichnis `.ago-import-*` liegen | `:1103-1108` |
| B11h | Fehlt `s.Vault`, wird der Vault-Import kommentarlos übersprungen | `:1205` |

---

## B. Memory-Snapshots

### M1 — High — Hängender Publikationsmarker blockiert alle Snapshots dauerhaft (bestätigt)
`startMemoryPublication` schreibt `snapshot.publication.<rand>` (`snapshot_publication.go:53`). `finish(false)` lässt den Marker stehen, wenn `rollbackAllowed` falsch ist oder keine ID entstanden ist (`:68-71`). Kein Code räumt solche Marker beim Start oder später auf (Grep: einzige Leser sind der Zähler in `snapshot_coordinator.go:267` und der Rollback-Pfad).

`FreezeMemorySnapshots` verweigert danach jeden Snapshot. Damit scheitern:
- der tägliche automatische Snapshot
- das manuelle Backup („Failed to create a coordinated memory backup“, `backup_handlers.go:679-682`, HTTP 500)

**Auslöser:**
- Ein Lesefehler in `findIdenticalMemory` (`vector_ownership.go:116`).
- Ein Abbruch vor `storeDocumentLocked`.
- Wiederverwendete IDs (`automatic_write.go:156`).
- Fakten über 4000 Zeichen: Ohne `ContentHash` ist der Rollback unmöglich (`automatic_write.go:166`, `:254-258`).

**Empfehlung:**
- Marker ohne erzeugte IDs in `finish` immer löschen.
- Beim Start alte Marker mit `status: publishing` und ohne IDs aufräumen.
- Die Fehlermeldung des Backups nennt den Grund.
- Tests für 0 IDs, wiederverwendete IDs und den Fall über 4000 Zeichen.

### M2 — Medium — Wartung löscht weiter, auch wenn der Sicherheits-Snapshot scheitert (bestätigt)
`internal/agent/maintenance.go:128-141`: Ein Snapshot-Fehler landet nur im Ledger, danach laufen STM-Retention und Pattern-Cleanup. Zusammen mit M1 laufen die Löschungen täglich ungeschützt.

**Empfehlung:** Destruktive Schritte überspringen, wenn kein gültiger Snapshot von heute existiert.

### M3 — Medium — Fehlgeschlagene Snapshots hinterlassen volle Kopien (bestätigt)
`automatic_snapshot.go:70-98`: Das Verzeichnis `auto-*` bleibt bei jedem Fehler liegen. Die Retention entfernt nur verifizierte Snapshots, und der Test `automatic_snapshot_test.go:68-89` schreibt dieses Verhalten fest. Bei vollem Datenträger wächst der Verbrauch also täglich weiter.

### M4 — Low — Sonstiges

| ID | Befund | Stelle |
| --- | --- | --- |
| M4a | `VACUUM INTO`-Kopie bleibt liegen, wenn `RestrictToOwner` scheitert; Migrations-`.bak` werden nie gelöscht | `sqlite_backup.go:25-31` |
| M4b | Stilles No-op bei leerem DataDir oder weniger als 3 Quellen; Lesefehler bei der Prüfung werden nicht geloggt | `automatic_snapshot.go:33-44`, `:163-196` |
| M4c | Die Retention hasht für jeden abgelaufenen Kandidaten alle Snapshots neu (quadratisch) | `automatic_snapshot.go:228` |
| M4d | Verschluckte Fehler in KG-Helfern (`json.Marshal`, `rows.Err`, `QueryRow`) | `graph_backup.go:280`, `:317`, `:345-367`, `:528-544` |
| M4e | Koordinator-Schlüssel: Config-Pfad vs. PRAGMA-Pfad; unter Windows (Groß-/Kleinschreibung, Junction) möglicherweise zwei Koordinatoren (plausibel) | `snapshot_coordinator.go:77`, `:184` |

---

## C. Update-Rollback (`update.sh`, `upkeep`)

### U1 — High — Abgebrochenes Git-Update verwirft lokale Änderungen endgültig (bestätigt)
`clean_tracked_changes` (`update.sh:3148`) setzt geänderte, getrackte Dateien zurück. Die Kopien liegen in `$BACKUP_DIR/prompts__custom` und in den `PROTECTED_DIRS`-Kopien von `agent_workspace/tools|skills`.

`abort_update` (`:2963-2992`) stellt aber nur `PROTECTED_FILES` und `DATA_FILES` wieder her (`restore_critical_user_data_after_failure`, `:2867-2881`). Danach markiert ein erfolgreicher Rollback die Transaktion als `rolled_back`, und `update_retention_cleanup` löscht jede nicht bestätigte Transaktion (`internal/upkeep/cleanup.go:278-306`). Damit verschwindet auch die einzige Kopie.

**Szenario:** Eine lokal geänderte `prompts/*.md` trifft auf einen Merge-Konflikt. Die Meldung lautet „pre-update checkout was restored“, die Änderung ist aber weg.

**Empfehlung:** Die Kopien in `abort_update` zurückspielen, und vor dem Zurückspielen nichts löschen.

### U2 — High — Unterbrechung lässt AuraGo gestoppt zurück (bestätigt)
`trap 'exit 130' INT` / `'exit 143' TERM` (`:2245-2246`) führen nur `update_exit_cleanup` aus (`:1421-1443`). Diese Funktion setzt das Manifest auf `uncertain`, startet AuraGo aber nicht neu und rollt nicht zurück.

Dasselbe gilt für ungeschützte Befehle zwischen Stopp und Build (`:2994-3066`).

**Folge:** Der Dienst bleibt aus, und das nächste Update ist blockiert, bis `--update-maintenance --resolve` läuft.

**Empfehlung:** Nach dem Stopp in Trap und `ERR` `abort_update` aufrufen.

### U3 — Medium — SQLite-Sidecars beim Update (bestätigt; Folge plausibel)
`DATA_FILES` (`:2139-2159`) sichert nur die Haupt-`.db`. `-wal`/`-shm` werden weder gesichert noch beim Restore entfernt.

**Szenarien:**
- AuraGo wird beim Stopp per SIGKILL beendet. Committete Zeilen stehen dann nur in der WAL und fehlen im Backup.
- Nach dem Rollback liegt die WAL des neuen Builds neben der alten DB.

### U4 — Medium — `agent_workspace/prompts` wird nach einem Teilfehler gelöscht (bestätigt)
`:3193-3210`: Ein Fehler von `rsync --ignore-existing` wird zur Warnung herabgestuft. Danach läuft `rm -rf "$OLD_PROMPTS"` ohne Bedingung. Im Git-Modus liegt keine Kopie vor.

### U5 — Medium — Config aus `/tmp` wird übernommen (bestätigt)
`:3006-3014`: Fehlt `config.yaml`, nimmt das Skript die neueste `/tmp/aurago-backup-*/config.yaml` und installiert sie (`:3179-3181`). Besitzer und Inhalt werden nicht geprüft. Auf Mehrbenutzer-Hosts kann ein anderer Nutzer so eine Config unterschieben, sonst wird eine veraltete Config übernommen.

### U6 — Low/Medium — Sonstiges

| ID | Befund | Stelle |
| --- | --- | --- |
| U6a | `--no-restart` + erfolgreicher Rollback → Manifest bleibt `pending` → `uncertain` → nächstes Update blockiert | `:2987` |
| U6b | Legacy-Backups aus `/tmp` gelten pauschal als `confirmed` und vollständig | `internal/upkeep/legacy.go:86` |
| U6c | `safe_restore_file` löscht das Ziel vor dem Kopieren | `:1911` |
| U6d | PID-Datei-Prüfung vor `flock`; der Verlierer hinterlässt seine Skriptkopie | `:1585-1591` |
| U6e | `repair-update.sh` hält nur eine Generation und gibt den Lock vor `exec` frei | `:198-208` |
| U6f | Root-Update: Wiederhergestellte Prompts/Skills/Tools gehören root (plausibel) | `:1926-1944` |
| U6g | tsnet: Scheitern Start und Rollback, bleibt der Backup-Ordner mit Node-Schlüsseln ohne Marker liegen (plausibel) | `tsnetnode.go:633-649` |

Ein Windows-Updater existiert nicht. Die `.ps1`-Dateien dienen Release und Signierung.

---

## Geprüft und in Ordnung

- **Krypto:** AES-256-GCM, 16 Byte Salt, 12 Byte Nonce je Verschlüsselung; Argon2id (3 Iterationen, 64 MiB, 4 Threads).
- **Passwort:** Übertragung nur im Body oder in Multipart-Daten, nie in der URL. Logs enthalten keine Secrets.
- **Import-Härtung:**
  - Zip-Slip, absolute Pfade, Laufwerks- und UNC-Pfade abgewiesen
  - Symlink-Einträge und Symlink-Vorfahren abgewiesen
  - Grenzen: 1 GiB gesamt, 100 MiB je Datei, Kompressionsrate 1000, 100.000 Einträge
  - `MaxBytesReader` begrenzt den Upload
  - Staging plus `integrity_check` vor dem Schreiben
- **Klartext-Backups:** enthalten keine Vault-Secrets. OAuth-Dateien in Skills werden ausgelassen.
- **Zugriff:** Endpunkte hinter `requireAdmin`; Bearer-Tokens brauchen den Admin-Scope; Origin/Referer-CSRF für Cookie-Sessions.
- **SQLite-Snapshot:** `VACUUM INTO` auf einer `mode=ro`-Verbindung; Sidecars werden nicht archiviert. Das Memory-Trio wird über gepinnte WAL-Lesetransaktionen gemeinsam gesichert.
- **Automatische Snapshots:**
  - O_EXCL mit 0600 bzw. Owner-DACL
  - `quick_check` und SHA-256 vor der Veröffentlichung
  - `complete.json` atomar geschrieben
  - Retention behält die 3 neuesten verifizierten Snapshots
- **Updater:**
  - `pending`/`uncertain` blockieren neue Updates (Bash und Go)
  - Zwei verifizierte Rollback-Generationen; Identitätsprüfung vor dem Löschen
  - Atomare Manifeste; gemeinsamer `flock`
  - `UPDATE_BACKUP_COMPLETE` erst nach allen Kopien gesetzt
  - Konsistente Speicherplatz-Rechnung

## Testlücken

- Restore: Fehler mitten im Veröffentlichen (B2), Sidecars nach dem Restore, Windows-Sharing-Violation.
- Rundlauf Export → Import für das koordinierte Memory-Trio.
- Fehlgeschlagener Vault-/Token-Export (B3), Body über 1 KB (B4), Ausschluss über absoluten Pfad (B5), eigene `tools_dir`/`workspace_dir` (B6), Schema-/Manifest-Abweichung (B7).
- Registry-Test: alle Laufzeit-DBs unter `data/` sind im Backup (B1).
- Memory: Publikationsfehler mit 0 IDs, wiederverwendeten IDs und über 4000 Zeichen (M1); Aufräumen nach fehlgeschlagenem Snapshot (M3); Wartung bei Snapshot-Fehler (M2).
- UI: Bestätigung vor dem Überschreiben, Darstellung von Teilerfolg, Nicht-JSON-Fehler (B8).
- Updater: Merge-Konflikt-Abbruch mit lokal geänderten Prompts (U1), SIGINT nach dem Stopp (U2).

## Vorschlag für Fix-Wellen

1. **Welle 1 (Datenverlust/Blockade):** M1, U1, B3, B4, U2.
2. **Welle 2 (Restore-Robustheit):** B2 (Restore beim Start vormerken + Sidecars), B1 (Registry + Test), B7, B8.
3. **Welle 3 (Härtung):** B9, B10, M2, M3, U3–U5, B5, B6.
4. **Rest:** Low-Punkte nach Gelegenheit.
