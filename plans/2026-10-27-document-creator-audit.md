# Audit: Document-Creator-Integration

**Datum:** 2026-10-27
**Scope:** `document_creator`-Tool (Agent-Dispatch), Gotenberg-Backend, Konfiguration/UI, Server-Endpunkte, Flow-Anbindung
**Methode:** Statische Analyse der Implementierung + gezielter Probe-Test (temporär, wieder entfernt) zur Verifikation des Panics

## Verified disposition

The original findings below describe the pre-fix source; their line numbers are
historical. Current-source verification refuted D-06 and D-14. The accepted
implementation is documented in `documentation/document-creator.md`.

| Findings | Selected action |
| --- | --- |
| D-01, D-07, D-08 | Shared section/table validation before either backend; matching title/header/body and sections/content semantics. |
| D-09, D-19 | Regular-file preflight and bounded reads (20 files / 32 MiB each / 64 MiB total), context-aware staging/assembly/publication. Maroto's internal generation remains synchronous; canceled results are discarded. |
| D-10, D-11 | Exclusive collision-suffixed output with checked write/sync/close; portable Unicode filenames and escaped multipart headers. |
| D-02, D-03, D-16 | Pinned hardened profile, uncached pre-upload `/debug` proof of version/network flags, conservative owned-container startup migration and native URL/port handling. |
| D-04, D-13 | Per-call current-timeout clients on the shared transport; same-origin redirects, no POST retries and explicit PDF/image 10 MiB errors. The response limit stays unchanged. |
| D-05 | Default-off admin `allow_remote_resources`; caller blocking can only tighten it. Public URL capture remains available; Detective exports remain offline. |
| D-12, D-20 | `title` documented only for `create_pdf`; Markdown example, conditional registry wording and nonexistent read-only option corrected. |
| D-06, D-14 | Refuted; existing Flow catalog and central administrator authorization retained and regression-tested. |
| D-15, D-17 | Outside scope: no automatic retention or new layout features. |
| D-18 | Only sanitized verification status/version/flag names are exposed from `/debug`; raw diagnostics are never returned or logged. Existing `/health` details retain their established shape. |

Regression suites cover renderer parity, file limits/collisions, cancellation,
security-proof failure before upload, lifecycle ownership/migration, saved
configuration, Flow/Detective compatibility and all 16 UI locales. Browser and
Linux race checks are separate from the opt-in real-image Docker acceptance;
an unavailable Docker daemon leaves that acceptance unverified. No running
installation is activated by these source changes.

### Verification outcome

- Focused Document Creator/Gotenberg/Detective-renderer checks and all 16 locale
  checks pass. The real browser checks cover the grant, boolean save/discard,
  saved-settings test and four diagnostic states at desktop/mobile widths.
- Full suites pass for `internal/server`, `internal/config`,
  `internal/detective` and `internal/flows`.
- The full tools/agent suites retain six failing test groups, independently
  reproduced in clean pre-change revision `4d6636a9f`: crawler Guardian isolation,
  local-LLM volume ownership, CYD volume ownership, the Dograh legacy fixture,
  execution-output isolation and tool search-term collisions. They are outside
  this implementation; no unrelated working changes were included.
- The opt-in real-image Docker acceptance compiles but is skipped because the
  daemon is unavailable. No daemon, running installation or existing container
  was started/reconfigured for validation.
- GitNexus was refreshed and complete staged/all change mappings reviewed.
  Structural flow extraction still has analyzer caps; FTS is disabled after its
  WAL failure, so current-source caller review and executable tests remain the
  evidence for behavior. Graft was refreshed; its unrelated generated CYD C++
  golden fixture exceeds the parser stack.
- DOX owners in tools, server, UI and Detective are updated. Root, agent and
  Docker-transport guides retain their existing ownership, wire-tool and
  transport contracts; those guides need no additional rule changes.

---

## 1. Architektur in Kürze

```
LLM-Tool-Call  →  agent_dispatch_exec.go:1780 "document_creator"
                   └─ decodeDocumentCreatorArgs (tool_args_execution.go:754)
                   └─ tools.ExecuteDocumentCreatorInWorkspace (document_creator.go:174)
                        ├─ Path-Jail + Staging der Quelldateien
                        └─ tools.ExecuteDocumentCreator (document_creator.go:75)
                             ├─ Backend "maroto"   → create_pdf only (Maroto, lokal)
                             └─ Backend "gotenberg" → alle 9 Operationen (Docker-Sidecar)
                   └─ Auto-Registrierung in der Media-Registry (agent_dispatch_exec.go:1788)

Konfiguration:  config.yaml → tools.document_creator{backend,output_dir,gotenberg{url,timeout}}
UI:             ui/cfg/document_creator.js  →  POST /api/document-creator/test
Server:         server_routes_config.go:276 → handleGotenbergTest (document_creator_handlers.go:13)
Sidecar:        docker-compose.yml:161 (compose) ODER EnsureGotenbergRunning (gotenberg.go:126, Auto-Start)
```

**Bewertung der Grundarchitektur:** Die Trennung Backend-Auswahl / Operation-Dispatch / Sicherheits-Policy (`block_remote_content`) ist sauber, die Fail-Closed-Haltung bei `block_remote_content` vorbildlich (Policy-Tests vorhanden), und die Flow-Anbindung über die Document-Bridge in `flows_tool_invoker_documents.go` ist durchdacht. Die nachfolgenden Befunde betreffen Randpfade, Robustheit und Konsistenz zwischen den beiden Backends.

---

## 2. Befundübersicht

| ID | Schwere | Bereich | Kurzfassung |
|----|---------|---------|-------------|
| D-01 | **Hoch** | Maroto-Renderer | Divide-by-zero-Panic bei leerer Tabellenzeile (`rows: [[]]`) |
| D-02 | **Hoch** | SSRF/Netz | SSRF-Prüfung läuft im Host-Netz, der Fetch passiert im Sidecar-Netz |
| D-03 | **Hoch** | Sidecar-Härtung | Auto-gestarteter Gotenberg-Container ist schwächer als der Compose-Container |
| D-04 | Mittel | HTTP-Client | Timeout wird global gecacht; Config-Änderung ohne Wirkung; Data Race |
| D-05 | Mittel | Security-Policy | `block_remote_content` ist per Default **aus** |
| D-06 | Widerlegt | Flows | Nicht angebotene URL-/Office-Operationen sind keine Regression |
| D-07 | Mittel | Backend-Parität | Maroto vs. Gotenberg: unterschiedliche Ausgabe für dieselbe Section |
| D-08 | Mittel | Backend-Parität | Ungültiges `sections`-JSON: Gotenberg ignoriert es still |
| D-09 | Mittel | Robustheit | Keine Größen-/Anzahl-Begrenzung für gestagte Quelldateien |
| D-10 | Mittel | Datenintegrität | Ausgabedateien werden still überschrieben |
| D-11 | Mittel | HTTP-Header | Un escapeter Dateiname im Multipart-`Content-Disposition` |
| D-12 | Niedrig | API/UX | `title` wird von 7 von 9 Operationen ignoriert |
| D-13 | Niedrig | Limits | 10-MB-Antwortlimit bricht große PDFs mit technischem Fehler |
| D-14 | Widerlegt | Autorisierung | Zentrale Admin-Bearer-Prüfung schützt die Route bereits |
| D-15 | Niedrig | Betrieb | Keine Retention für `data/documents` |
| D-16 | Niedrig | Konfiguration | UI-Default-URL passt nicht zu Native-Install; Port hart verdrahtet |
| D-17 | Niedrig | PDF-Qualität | Maroto: eine Seite pro Section, keine Kopf-/Fußzeilen, keine PDF-Metadaten |
| D-18 | Niedrig | Informationsleck | `health`-Operation gibt Sidecar-Interna an das LLM |
| D-19 | Niedrig | Abbruch | Maroto-Pfad ignoriert `ctx` (keine Cancellation) |
| D-20 | Niedrig | Doku/Impl-Drift | Doku nennt Read-Only-Modus, Config kennt kein Feld dafür |

---

## 3. Details

### D-01 — Divide-by-zero-Panic im Maroto-Renderer (Hoch)

**Ort:** `internal/tools/document_creator.go:410` und `:427`

```go
colSize := 12 / len(data[0])   // data[0] == []string{}  →  panic: integer divide by zero
```

`buildTableRows` prüft nur `len(data) == 0`, nicht `len(data[0]) == 0`. Das `sections`-JSON kommt ungefiltert vom LLM.

**Verifikation (Probe-Test, temporär):**

```
=== RUN   TestAuditProbeEmptyTableRow
    PANIC from buildTableRows: runtime error: integer divide by zero
--- FAIL: TestAuditProbeEmptyTableRow (0.00s)
```

Eingabe: `{"operation":"create_pdf","sections":"[{\"type\":\"table\",\"rows\":[[]]}]"}`

**Wirkung:** Der Panic wird von `recoverToolDispatch` (`internal/agent/agent_parse.go:97`) abgefangen — kein Prozessabsturz. Aber das Tool liefert `{"status":"unknown","code":"tool_panic",...}` statt eines verwertbaren Fehlers; das LLM versteht nicht, dass die Section-Form falsch war, und der Aufruf ist beendet. Ein naheliegendes LLM-Schema (`rows: [[]]` für eine leere Tabelle) triggert den Fehler deterministisch.

**Fix:**
```go
if len(data) == 0 || len(data[0]) == 0 {
    return nil
}
colSize := max(1, 12/len(data[0]))
```
Zusätzlich: Spaltenzahl auf 12 begrenzen und alle Zeilen auf die Breite der Kopfzeile normalisieren, damit `colSize` je Zeile nicht springt.

**Testlücke:** `internal/tools/document_creator_test.go` deckt `create_pdf` mit Sections ab, aber keinen Tabellen-Edge-Case. Regressionstest ergänzen.

---

### D-02 — SSRF-Prüfung im falschen Netzkontext (Hoch)

**Ort:** `internal/tools/gotenberg.go:295` (url_to_pdf), `:468` (screenshot_url) — Prüfung in `internal/security/ssrf.go:109-145`

AuraGo validiert die URL **auf dem Host**: `ValidateSSRF` löst den Hostnamen auf dem Host-System auf und lehnt private IPs ab. Der eigentliche Abruf passiert aber **im Gotenberg-Container**: Die URL geht als Formularfeld an Gotenberg (`fields["url"] = url`), und Chromium im Container lädt sie.

Damit existieren zwei Lücken:

1. **Resolver-Split:** Hostname löst sich vom Host aus öffentlich auf, im Container dagegen auf 127.0.0.1, einen Docker-Service (`redis`, `postgres`) oder eine interne IP. Der Check sieht eine öffentliche IP, der Container trifft das interne Ziel. Klassisches DNS-Rebinding/TOCTOU.
2. **Container-Netzwerk:** Gotenberg läuft mit unbeschränktem Egress und vollem Docker-Netz-Zugriff. `169.254.169.254` (Cloud-Metadata) und andere Sidecars sind von dort erreichbar.

`block_remote_content` deckt genau diese beiden Operationen **nicht** ab (sie sind laut `document_creator.go:67` per Design nicht schützbar) — der SSRF-Pfad bleibt also offen.

**Fix (priorisiert):**
1. Gotenberg-seitige Kontrolle nutzen: die Chromium-Routen akzeptieren `denyList` und `allowedHosts` als Formularfelder. `allowedHosts` auf den validierten Host setzen bzw. `denyList` auf private/reserved Bereiche + Metadata-IP. Damit greift die Policy dort, wo der Abruf tatsächlich stattfindet.
2. Container-Netzwerk härten: eigenes Netz ohne Default-Gateway-Egress bzw. `internal: true` und nur die nötigen Peers.
3. Optional: URL-Auflösung einmalig in AuraGo durchführen und die IP (nicht den Hostnamen) an Gotenberg übergeben, um Split-Brain auszuschließen.

---

### D-03 — Auto-gestarteter Sidecar ist schwächer als der Compose-Sidecar (Hoch)

**Ort:** `internal/tools/gotenberg.go:180-193` vs. `docker-compose.yml:161-179`

| | Compose-Service | `EnsureGotenbergRunning` |
|---|---|---|
| Capabilities | `cap_drop: ALL` + nur `CAP_CHOWN`/`CAP_FOWNER` | `CapAdd: ["SYS_ADMIN"]` |
| no-new-privileges | ja | nein |
| Ressourcenlimits | `cpus: 1.0`, Memory | keine |
| Netzwerk | internes Compose-Netz | Default-Bridge |

Der Auto-Start-Pfad startet also eine **privilegiertere** Chromium-Instanz (SYS_ADMIN) ohne Ressourcenlimits. Ein entgleister Render kann den Host lahmlegen (OOM), und die Sidecar-Härtung aus dem Compose-File gilt für native Installationen nicht.

**Fix:** Das Compose-Hardening-Profil in `EnsureGotenbergRunning` spiegeln: `CapDrop: ["ALL"]` + `CapAdd: ["CHOWN","FOWNER"]`, `SecurityOpt: ["no-new-privileges:true"]`, `Memory`/`NanoCpus` setzen, in ein dediziertes Netz mit `Internal: true` hängen und nur die nötigen Ports publizieren. Port-Binding an die konfigurierte URL koppeln (siehe D-16).

---

### D-04 — Global gecachter HTTP-Client (Mittel)

**Ort:** `internal/tools/gotenberg.go:23-36`

```go
var gotenbergClient *http.Client
func getGotenbergClient(timeoutSec int) *http.Client {
    if gotenbergClient != nil { return gotenbergClient }   // erster Aufruf gewinnt für immer
    ...
}
```

Zwei Folgen:

1. **Config-Änderung ohne Wirkung:** `tools.document_creator.gotenberg.timeout` wird im UI gespeichert, aber der Client existiert bereits mit dem alten Timeout. Der Wert greift erst nach einem Neustart. Der UI-Testknopf „Gotenberg testen" nutzt denselben gecachten Client und bestätigt damit einen Timeout, den der echte Betrieb nicht verwendet.
2. **Data Race:** `gotenbergClient` wird ohne Synchronisation geschrieben und gelesen. Der Agent dispatcht Tools asynchron (Async-Task-Group, Flows, parallele Sessions) — der erste konkurrierende Aufruf ist ein Datenrace auf Package-Ebene (`go test -race` würde es melden).

**Fix:** Client nicht global cachen, sondern pro Timeout-Wert bauen (z.B. `sync.Map` mit Key `timeoutSec`) oder einen `sync.Once`-basierten Client mit atomarem Timeout-Update verwenden. Beim Config-Reload den Cache leeren.

---

### D-05 — `block_remote_content` ist per Default aus (Mittel)

**Ort:** `internal/agent/native_tools_integrations.go:1993` (Schema-Beschreibung „default: false"), `prompts/tools_manuals/document_creator.md:32`

Der Parameter ist vom LLM frei setzbar und standardmäßig `false`. Inhalt, den das LLM typischerweise rendert, stammt aber aus gescrapten Webseiten — also genau dem Fall, für den die Policy existiert. Default-mäßig rendert Chromium beliebiges Fremd-HTML mit vollem Netzwerkzugriff im Container.

**Fix:** Default auf `true` umstellen (fail closed) und `false` als explizite Opt-in-Ausnahme für vertrauenswürdige Inhalte dokumentieren. Alternativ als Policy-Ebene im Config (`tools.document_creator.block_remote_content: true`) hinterlegt, die das LLM nicht überschreiben kann.

---

### D-06 — Refuted: Flow operation scope

This is not a security defect. The Flow document bridge exposes only text,
HTML and Markdown rendering. It does not promise URL capture or Office
conversion, so its mandatory `block_remote_content` does not disable advertised
operations. Preserve the existing Flow restriction and regression coverage.

---

### D-07 — Backend-Parität: Section-Ausgabe divergiert (Mittel)

**Ort:** Maroto `internal/tools/document_creator.go:348-395` vs. Gotenberg `:472-516`

Für `{"type":"text","header":"X","body":""}`:

- **Maroto:** rendert `header` als fette Überschrift (Größe 14) **und** erneut als Fließtext (weil `bodyText == ""` auf `sec.Header` zurückfällt) → Text doppelt im PDF.
- **Gotenberg:** rendert `<h2>X</h2>` plus ein leeres `<p></p>` → Überschrift einmal, leerer Absatz.

Dieselbe Eingabe erzeugt je nach Backend unterschiedliche Dokumente. Das ist für den LLM schwer erklärbar und in Tests nicht abgedeckt.

**Fix:** Gemeinsame Semantik definieren (z. B. „Header wird immer als Überschrift gerendert; Body-Fallback auf Header entfällt") und beide Renderer daran ausrichten. Idealerweise eine gemeinsame Section→Blocks-Aufbereitung extrahieren, die nur die Formatierung pro Backend unterscheidet.

---

### D-08 — Ungültiges `sections`-JSON wird still ignoriert (Mittel)

**Ort:** `internal/tools/document_creator.go:260` (Maroto: Fehler wird gemeldet) vs. `:474` (Gotenberg: `if err == nil` → Fehler wird verworfen)

Auf dem Gotenberg-Backend führt ein syntaktisch falsches `sections`-JSON zu einem PDF **ohne** die Sections — ohne jede Rückmeldung. Das LLM glaubt, ein strukturiertes Dokument erzeugt zu haben, und liefert ein leeres aus.

**Fix:** In `buildHTMLFromSections` denselben Fehlerpfad wie Maroto verwenden (Fehler zurückgeben statt still zu verwerfen).

---

### D-09 — Keine Begrenzung für gestagte Quelldateien (Mittel)

**Ort:** `internal/tools/document_creator.go:184-219`

Jede in `source_files` genannte Datei wird ungeprüft in ein Temp-Verzeichnis kopiert (`io.Copy` ohne Limit). Es gibt weder ein Größen- noch ein Mengenlimit; eine einzelne Datei beliebiger Größe oder beliebig viele Dateien werden kopiert. Der Flow-Pfad hat bereits eine passende Schranke (`flowDocumentCopyLimit = 32 MiB`, `flows_tool_invoker_documents.go:48`), der Agent-Pfad nicht.

**Fix:** Dieselben Schranken übernehmen: max. Dateigröße (32 MiB), max. Anzahl (z. B. 20), max. Gesamtvolumen. `io.Copy` durch `io.CopyN` mit `LimitReader` ersetzen und bei Überschreitung eine klare Fehlermeldung liefern.

---

### D-10 — Ausgabedateien werden still überschrieben (Mittel)

**Ort:** `internal/tools/gotenberg.go:110` (`os.WriteFile`) und `internal/tools/document_creator.go:274` (`doc.Save`)

Ein vom LLM vorgegebener `filename` überschreibt eine bestehende Datei im Output-Verzeichnis ohne Warnung. Da der Output-Ordner auch von der Media-Registry und `/files/documents/` ausgeliefert wird, kann ein späterer Aufruf ein earlier ausgeliefertes Dokument ersetzen (auch über `/files/documents/<name>`).

**Fix:** Mit `O_EXCL` schreiben; bei Kollision einen Suffix anhaengen (`name-1.pdf`) und den finalen Namen im Resultat zurückgeben.

---

### D-11 — Un escapeter Dateiname im Multipart-Header (Mittel)

**Ort:** `internal/tools/gotenberg.go:57`

```go
h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, name))
```

`name` stammt aus `filepath.Base(p)`, wobei die Endung über `filepath.Ext` vom LLM kontrolliert wird (`document_creator.go:203`: `source-%d` + `filepath.Ext(rp)`). Ein Endungswert mit `"` oder CRLF bricht den Header bzw. injiziert Zeilen in die Anfrage an den Sidecar.

**Fix:** Dateinamen vor dem Schreiben auf ein sicheres Zeichensatz setzen (alphanumerisch, `-`, `_`, `.`) oder `multipart.Writer.CreateFormFile` mit bereinigtem Namen verwenden.

---

### D-12 — `title` wird von 7 von 9 Operationen ignoriert (Niedrig)

**Ort:** `internal/tools/document_creator.go:94-161` — nur `create_pdf` nutzt `title`. `markdown_to_pdf`, `html_to_pdf`, `url_to_pdf`, `convert_document`, `merge_pdfs`, `screenshot_url`, `screenshot_html` verwerfen ihn.

Das Tool-Manual (`prompts/tools_manuals/document_creator.md:57`) zeigt aber genau das Beispiel `{"operation":"markdown_to_pdf","title":"My Document",...}` — der Titel landet nicht im PDF, nur in der Media-Registry (`Description: req.Title`).

**Fix:** Titel in die HTML-/Markdown-Erzeugung übernehmen (als `<h1>` bzw. Front-Matter) oder das Argument aus Schema und Manual entfernen.

---

### D-13 — 10-MB-Antwortlimit bricht große PDFs (Niedrig)

**Ort:** `internal/tools/http_limits.go:8`, verwendet in `internal/tools/gotenberg.go:84`

`maxHTTPResponseSize` (10 MiB) gilt auch für die PDF-Nutzlast. Größere Dokumente scheitern mit `response body exceeds limit of 10485760 bytes` — eine technische Meldung ohne Bezug zum eigentlichen Anlass.

**Fix:** Für Binärantworten ein eigenes, largeres Limit (z. B. 50 MiB) verwenden und die Meldung dokumentieren.

---

### D-14 — Refuted: existing administrator authorization

The central authentication middleware's `validRouteBearer` fallback already
requires administrator authority for this route when authentication is enabled.
A token with only `read` receives HTTP 403 before the handler. The endpoint
retains the existing auth-disabled behavior. An additional `requireAdmin`
wrapper is unnecessary; the regression test pins the actual boundary.

---

### D-15 — Keine Retention für das Output-Verzeichnis (Niedrig)

`data/documents` wächst unbegrenzt; es existiert keine Aufräum-Logik (nur `main.go:467` legt das Verzeichnis an). Anders als die Flow-Scratch-Folder (`.easydrag`, Sweep nach `flowDocumentScratchRetention`) fehlt hier eine Retention.

**Fix:** Altersbasierte Bereinigung analog zum Flow-Sweep, optional konfigurierbar (`tools.document_creator.retention_days`).

---

### D-16 — Port hart verdrahtet, UI-Default passt nicht zur Installation (Niedrig)

**Ort:** `internal/tools/gotenberg.go:187` (`"HostPort": "3000"`), `ui/cfg/document_creator.js:68` (Default `http://gotenberg:3000`)

Der Auto-Start bindet immer `127.0.0.1:3000`, unabhängig von der konfigurierten URL. Wer `tools.document_creator.gotenberg.url` auf einen anderen Port setzt, bekommt einen Sidecar, den AuraGo nicht erreicht. Der UI-Default `http://gotenberg:3000` ist der Compose-Service-Name und damit für Native-Installationen (Bare Metal) falsch — dort wäre `http://127.0.0.1:3000` korrekt.

**Fix:** Port aus der konfigurierten URL ableiten; UI-Default auf `http://127.0.0.1:3000` setzen und den Compose-Wert im Compose-Setup dokumentieren.

---

### D-17 — PDF-Qualität im Maroto-Backend (Niedrig)

**Ort:** `internal/tools/document_creator.go:320-398`

- Jede Section beginnt auf einer **neuen Seite** (`m.AddPages(...)` pro Section) — lange Texte fließen nicht, jedes Abschnittsfragment erzwingt einen Seitenumbruch.
- Keine Kopf-/Fußzeilen, keine Seitenzahlen, kein PDF-Metadatum (Titel/Autor fehlen in der Datei).
- Nur Vordergrund-Absätze; der Standardfont deckt keine CJK-Schriftzeichen ab.
- `buildMarotoConfig` setzt linken/rechten/oberen Rand, aber keinen unteren.

**Fix:** Fließtext über `Maroto`-Text-Listen mit automatischem Seitenumbruch rendern, Metadaten setzen (`WithMetadata`), Ränder vervollständigen.

---

### D-18 — `health`-Operation gibt Sidecar-Interna an das LLM (Niedrig)

**Ort:** `internal/tools/gotenberg.go:261-291`

Die Operation ist als reguläres Tool-Argument verfügbar und liefert den Roh-JSON des Sidecar-Health-Endpoints samt Versionsangaben. Für Diagnose sinnvoll, für ein Modell unnötig.

**Fix:** Antwort auf ein minimales Feld reduzieren (erreichbar/nicht erreichbar + Version) und die Rohdetails nur im Server-Log bzw. im UI-Testknopf ausgeben.

---

### D-19 — Maroto-Pfad ignoriert `ctx` (Niedrig)

`createPDFMaroto` (und `buildMarotoDocument`) erhalten keinen `context.Context`; eine lange PDF-Erzeugung lässt sich nicht abbrechen. Der Gotenberg-Pfad bricht dagegen sauber ab.

**Fix:** `ctx` durchreichen und zwischen den Sections prüfen.

---

### D-20 — Doku/Implementierungs-Drift (Niedrig)

- `documentation/manuals/de/07-configuration.md:320` nennt `document_creator` als Tool, das auf „Nur-Lesen" gesetzt werden kann. `config.DocumentCreatorConfig` (`internal/config/config_types.go:257-263`) besitzt **kein** solches Feld — die Option existiert nicht.
- Der `content`-Parameter wird im Schema als „HTML content (for html_to_pdf, screenshot_html), Markdown content …, or text content (for create_pdf without sections)" beschrieben; `create_pdf` nutzt `content` aber nur, wenn **keine** Sections übergeben wurden (`document_creator.go:336`) — Sections und Content gleichzeitig sind stillschweigend doppelt.
- Der Hinweis „Generated files are saved to the media registry" im Manual (`prompts/tools_manuals/document_creator.md:93`) gilt nur, wenn `mediaRegistryDB != nil` (`agent_dispatch_exec.go:1789`).

**Fix:** Doku an die Implementierung angleichen oder die fehlenden Flags (Read-Only, Retentionsdauer) implementieren.

---

## 4. Empfohlene Reihenfolge

### Sofort (Kleinsts fix, hoher Nutzen)
1. **D-01** Divide-by-zero beheben + Regressionstest. (2 Zeilen Code, verhindert Tool-Panic.)
2. **D-08** Ungültiges Sections-JSON auf dem Gotenberg-Backend melden.
3. **D-07** Section-Semantik angleichen (Header-Doppelung in Maroto entfernen).
4. **D-12** `title` entweder umsetzen oder aus Schema/Manual entfernen.
5. **D-14** `requireAdmin` für `/api/document-creator/test`.

### Nächste Iteration
6. **D-04** HTTP-Client pro Timeout bauen (behebt Race und „Config ohne Wirkung").
7. **D-09** Größen-/Mengenlimit für gestagte Quelldateien.
8. **D-10** Kollisionsfreies Schreiben der Ausgabedateien.
9. **D-11** Dateinamen im Multipart-Header escapen.
10. **D-05** Default für `block_remote_content` auf `true` (Policy-Fail-Closed).
11. **D-06** Flow-Policy auf die schützbaren Operationen begrenzen.

### Strukturell
12. **D-03** Sidecar-Hardening-Profil aus Compose in den Auto-Start spiegeln.
13. **D-02** Gotenberg-seitige `denyList`/`allowedHosts` nutzen + Container-Netzwerk härten.
14. **D-16/D-17/D-15** Port aus Config ableiten, Maroto-Paginierung/PDF-Metadaten, Retention.
15. **D-13/D-18/D-19/D-20** Limits dokumentieren, `health` minimieren, `ctx` durchreichen, Doku angleichen.

---

## 5. Testabdeckung — Lücken

Bestehende Tests decken gut ab: JSON-Validität der Ergebnisse, Uniqueness der Default-Namen, Path-Escape beim Staging, `block_remote_content` (Policy, Tag-Neutralisierung, Fail-Closed, Reichweite über alle Chromium-Routen) sowie die Backend-Verweigerung.

**Nicht abgedeckt:**

| Bereich | Fehlender Test |
|---|---|
| Tabellen-Rendering | Leere Kopfzeile (`rows: [[]]`), Spaltenzahl ≠ Kopfzeile, > 12 Spalten |
| Backend-Parität | Gleiche Sections → identische Semantik auf beiden Backends |
| Section-Parsing | Ungültiges JSON auf dem Gotenberg-Backend (muss Fehler liefern) |
| HTTP-Client | Timeout-Wechsel nach Config-Reload; Parallelaufrufe (`-race`) |
| Limits | Quelldatei > 32 MiB; Antwort > 10 MiB |
| Ausgabe-Kollision | Zweiter Aufruf mit gleichem `filename` überschreibt nicht |
| Multipart | Dateiname mit `"` und CRLF in der Endung |

Empfehlung: Die neuen Tests in `internal/tools/document_creator_hardening_test.go` bündeln, damit die Fail-Closed-Logik der Policy und die Robustheit des Renderers an einer Stelle liegen.

---

## 6. Verifikationsstand

- **Empirisch reproduziert:** D-01 (Probe-Test, Panic bestätigt, Testdatei danach entfernt; Arbeitsverzeichnis unverändert).
- **Per Quellcode belegt:** D-02 bis D-20 (Zeilenverweise siehe jeweiliger Abschnitt).
- **Nicht geprüft:** Laufzeitverhalten der Gotenberg-Version 8 (kein Sidecar im Testbetrieb), tatsächliches Verhalten der `denyList`-/`allowedHosts`-Felder in der installierten Gotenberg-Version, Rendering-Qualität der Maroto-Ausgabe in der Praxis.
