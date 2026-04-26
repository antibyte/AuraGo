# Plan: Optionale Dokumentations-/Cheatsheet-Anhänge für Skills

**Datum:** 2026-04-26
**Status:** Entwurf
**Scope:** Skill-Manager, AI-Draft, Import/Export, UI, Agent-Integration

---

## 1. Ziel

Beim Anlegen eines neuen Skills soll **optional eine Bedienungsanleitung** als
Markdown beigelegt werden können. Diese Anleitung erklärt dem Agenten, **wie** er
den Skill korrekt aufruft (typische Parameter, Beispiel-Calls, Fallstricke,
Output-Schema). Die Anleitung kann

- direkt im Web-UI (Code-Editor) erstellt,
- als `.md`-Datei hochgeladen,
- oder per "AI-Draft" zusammen mit dem Skill generiert werden.

Der Agent kann die Anleitung über ein Tool abrufen, **wenn er den Skill
benutzen will** – nicht permanent im System-Prompt. Beim Export/Import des
Skill-Bundles wird die Doku mitgeführt.

---

## 2. Begriffsklärung

Auragos Skill-Welt enthält zwei verwandte Dinge:

| Begriff | Wozu? |
|---|---|
| **Skill-Doku (neu)** | Bedienungsanleitung **nur** für diesen einen Skill. Lebt im Skill-Verzeichnis und im Skill-Bundle. |
| **CheatSheet** (existiert bereits, `internal/tools/cheatsheets.go`) | Globales Wissensobjekt, kann thematisch viele Skills oder Workflows abdecken. |

**Designentscheidung:** Wir führen *Skill-Doku* als **eigenständigen, an den
Skill gekoppelten Anhang** ein (1:1-Beziehung). Zusätzlich erlauben wir
optional, eine **Referenz auf eine vorhandene CheatSheet-ID** zu setzen,
falls der Nutzer auf bestehendes Wissen verweisen will (1:N, lose Kopplung).

---

## 3. Datenmodell

### 3.1 Dateisystem (`agent_workspace/skills/`)

Pro Skill existieren bisher zwei Dateien:

```
my_skill.json   # Manifest
my_skill.py     # Code
```

Neu:

```
my_skill.md     # OPTIONAL: Bedienungsanleitung (UTF-8, max 64 KB)
```

Die Doku-Datei trägt **denselben Basisnamen** wie das Manifest. So bleibt das
Sync-Verhalten (`SyncFromDisk`) deterministisch und der Skill bleibt
selbst-portierbar.

### 3.2 Manifest-Erweiterung (`SkillManifest` in `internal/tools/skills.go`)

Neues Feld:

```go
type SkillManifest struct {
    // ...
    Documentation     string   `json:"documentation,omitempty"`      // optional: Pfad relativ zum Skills-Dir, default "<name>.md" wenn vorhanden
    CheatsheetIDs     []string `json:"cheatsheet_ids,omitempty"`     // optional: verlinkte CheatSheet-IDs
}
```

Konvention: Liegt `<name>.md` neben dem Manifest **und** das Feld ist leer,
wird der Pfad bei `SyncFromDisk` automatisch befüllt.

### 3.3 DB-Schema (`skills_registry`, neue Migration)

```sql
ALTER TABLE skills_registry ADD COLUMN documentation_path TEXT DEFAULT '';
ALTER TABLE skills_registry ADD COLUMN documentation_hash TEXT DEFAULT '';
ALTER TABLE skills_registry ADD COLUMN cheatsheet_ids TEXT DEFAULT '';   -- JSON array
```

Speicherung:
- **Inhalt** der Doku liegt **immer** auf der Platte (`<name>.md`), nicht in
  der DB → konsistent mit Skill-Code (auch nicht in DB).
- DB führt nur Pfad + Hash für Drift-Detection im Audit.

### 3.4 `SkillRegistryEntry`

```go
type SkillRegistryEntry struct {
    // ...
    DocumentationPath string   `json:"documentation_path,omitempty"`
    DocumentationHash string   `json:"documentation_hash,omitempty"`
    HasDocumentation  bool     `json:"has_documentation"`
    CheatsheetIDs     []string `json:"cheatsheet_ids,omitempty"`
}
```

`HasDocumentation` ist ein abgeleitetes Feld für die UI (nur Bool, ohne
Inhalt zu laden).

### 3.5 Export-Bundle (`SkillExportBundle`)

```go
type SkillExportBundle struct {
    // ...
    Documentation string `json:"documentation,omitempty"` // Markdown-Inhalt direkt eingebettet
    // CheatsheetIDs werden NICHT exportiert (würden ohne Empfänger-DB ins Leere zeigen)
}
```

Beim **Export** wird der Inhalt von `<name>.md` in das Bundle eingebettet.
Beim **Import** wird der Inhalt nach `<skillsDir>/<imported_name>.md`
geschrieben, sofern das Bundle ein `documentation`-Feld enthält.

---

## 4. Backend-Änderungen

### 4.1 `internal/tools/skill_manager.go`

| Funktion | Änderung |
|---|---|
| `InitSkillsDB` | Migrationen für neue Spalten ergänzen (idempotent). |
| `SyncFromDisk` | Wenn `<name>.md` existiert, Pfad + Hash erfassen. Default-Pfad-Auto-Detection. |
| `CreateSkillEntry` | Neuer Parameter `documentation string` (oder neuer Setter). Schreibt `.md`-Datei atomar via `os.WriteFile` + `chmod 0644`. Größenlimit 64 KB. |
| `UpdateSkillMetadata` | Erweitern um `documentation *string` und `cheatsheetIDs []string`. Wird `nil` übergeben, bleibt der Wert unverändert. |
| `GetSkill` / `ListSkillsFiltered` | Neue Felder mit lesen, `HasDocumentation = (documentation_path != "")`. |
| `DeleteSkill` | `<name>.md` mitlöschen (best-effort, Fehler nur loggen). |

### 4.2 Neue Funktionen

```go
// In skill_manager.go (oder neuer Datei skill_documentation.go)
func (m *SkillManager) GetSkillDocumentation(id string) (string, error)
func (m *SkillManager) SetSkillDocumentation(id, content, updatedBy string) error
func (m *SkillManager) DeleteSkillDocumentation(id, updatedBy string) error
```

Audit-Einträge: `documentation_added`, `documentation_updated`,
`documentation_removed`.

### 4.3 `internal/tools/skill_history.go`

- `ExportSkillBundle`: lädt `<name>.md` und befüllt `bundle.Documentation`.
- `ImportSkillBundle`: schreibt `bundle.Documentation` nach
  `<skillsDir>/<entry.Name>.md`, falls non-empty, dann
  `m.SyncFromDisk()` für diesen einen Eintrag (oder direkter Update der
  DB-Felder).

### 4.4 REST-Endpunkte (`internal/server/skills_handlers_*.go`)

Neue Routen in `server_routes_tools.go`:

| Methode | Pfad | Zweck |
|---|---|---|
| `GET` | `/api/skills/{id}/documentation` | Markdown-Inhalt + Hash zurückliefern |
| `PUT` | `/api/skills/{id}/documentation` | JSON-Body `{ "content": "..." }` – speichert/überschreibt |
| `POST` | `/api/skills/{id}/documentation/upload` | Multipart `file` – akzeptiert `.md`/`.txt`, max 64 KB |
| `DELETE` | `/api/skills/{id}/documentation` | Doku entfernen |

Schutz:
- Respektieren von `tools.skill_manager.read_only` und `allow_uploads`.
- Content-Type-Whitelist: `text/markdown`, `text/plain`.
- Markdown-Sanitizing nicht nötig (wird nicht gerendert mit JS-Eval), aber
  HTML-Render im UI muss `marked.js` mit Sanitizer benutzen oder
  `pre`-Block-Fallback.
- Größenlimit `64 * 1024` Byte – per Konstante.

Anpassungen bestehender Endpunkte:
- `POST /api/skills/upload` (Code-Upload): zusätzliches Multipart-Feld
  `documentation` (Text) optional.
- `POST /api/skills/templates`: Body um `documentation` (Text) erweitern.
- `POST /api/skills/import`: bereits abgedeckt durch Bundle-Field.
- `POST /api/skills/generate`: Antwort-Schema um `documentation` erweitern (s. § 5).
- `GET /api/skills/{id}?code=true`: `documentation=true`-Flag, um Markdown
  inline zu liefern.
- `GET /api/skills/{id}/export`: Bundle enthält Doku.

### 4.5 Agent-Tool für Doku-Abruf

Ohne neues Tool wäre die Doku nur über Datei-Reader erreichbar. Cleaner ist
ein dedizierter Aufruf:

**Option A (bevorzugt):** Bestehendes `list_skills`/`execute_skill`-Tool
hat im Manifest-Snippet einen `documentation_excerpt` (erste 400 Zeichen),
sodass der Agent sieht, *dass* eine Doku existiert.

**Plus** neues Tool im Skills-Engine-Bundle (siehe
`prompts/tools_manuals/skills_engine.md`):

```
get_skill_documentation(name: string) -> string  // gibt vollständiges Markdown zurück
```

Implementierung: thin wrapper auf `GetSkillDocumentation`. Output wird
gescrubbt durch bestehenden Sensitive-Scrubber.

System-Prompt-Hinweis (in `prompts/tools_manuals/skills_engine.md`):
> Hat ein Skill `has_documentation: true`, rufe vor dem ersten Aufruf
> `get_skill_documentation(<name>)` auf, um Parameter und Beispielcalls zu
> kennen.

---

## 5. AI-Draft-Generierung (`handleGenerateSkillDraft`)

### 5.1 Schema-Erweiterung

`generatedSkillDraft` bekommt ein neues Feld:

```go
type generatedSkillDraft struct {
    // ...
    Documentation string `json:"documentation"` // Markdown
}
```

### 5.2 System-Prompt-Update

Aktuelles Schema im System-Prompt erweitern:

```
{
  "name":"...",
  "description":"...",
  "category":"...",
  "tags":[...],
  "dependencies":[...],
  "code":"...",
  "documentation":"# <skill_name>\n\n## Was tut der Skill\n...\n## Eingabe\n...\n## Ausgabe\n...\n## Beispiel\n```json\n{...}\n```\n## Fehlerfälle\n..."
}
```

Anweisung:
- Pflicht-Sektionen: **Was tut der Skill**, **Eingabe-Parameter**,
  **Ausgabe-Schema**, **Beispielaufruf**, **Fehlerfälle**.
- Markdown nur GFM, kein HTML.
- Max ~2 KB.

### 5.3 Repair-Path

`maybeRepairGeneratedSkillDraft`: wenn `documentation` fehlt, Repair-Call
nachfragen oder leer lassen (kein BadGateway, da optional).

### 5.4 Frontend-Flow

Im "AI Draft"-Modal: Nach Generierung erscheint zusätzlich ein
Markdown-Editor-Tab mit dem generierten Doku-Vorschlag (Edit-fähig).
"Save"-Button speichert beim finalen Skill-Create gleich die `.md`.

### 5.5 Placeholder-Validation

`generatedSkillPlaceholderIssues` prüft auch `documentation` auf typische
Halluzinations-Patterns (`...`, `[ ... ]`, `lorem ipsum`).

---

## 6. UI-Änderungen

### 6.1 `ui/skills.html`

**Detail-Modal:** Neuer Tab/Abschnitt "Documentation" mit:
- Render-Ansicht (über `marked.js`, sanitisiert).
- "Edit"-Button → öffnet Markdown-Editor (CodeMirror, Mode `gfm`).
- "Upload .md"-Button → File-Input.
- "Delete"-Button (wenn vorhanden).
- Anzeige der verknüpften CheatSheet-IDs als Chips (klickbar zum
  CheatSheet-Manager).

**Upload-Modal (Skill-Upload):** Zweiter optionaler File-Input
"Documentation (.md)" + Markdown-Textarea als Fallback.

**Code-Draft-Modal (manueller Skill-Entwurf):** Neuer Tab "Documentation"
neben "Manifest"/"Code".

**From-Template-Modal:** Optionales Feld "Documentation" (Markdown-Textarea)
mit Default-Vorlage:

```md
# {{ skill_name }}

## Beschreibung
{{ description }}

## Parameter
- (auto-fill aus Template)

## Beispielaufruf
```

**AI-Draft-Modal:** Nach Generierung dritter Tab "Documentation" mit dem
generierten Markdown.

**Import-Modal:** Bei Erfolg Toast erweitern → "Skill imported (with
documentation)" wenn Bundle Doku enthielt.

**Skill-Karte (List-View):** Kleines Buch-Icon, wenn `has_documentation:
true`. Tooltip: "Has agent manual".

### 6.2 `ui/js/skills/main.js`

Neue Funktionen:

```js
async function loadSkillDocumentation(id)            // GET .../documentation
async function saveSkillDocumentation(id, markdown)  // PUT
async function uploadSkillDocumentation(id, file)    // multipart
async function deleteSkillDocumentation(id)          // DELETE
function renderMarkdown(md)                          // marked + sanitize
```

Anpassen:
- `submitImportSkill`: kein JS-seitiger Patch nötig, Bundle wird
  passthrough geschickt.
- `submitGenerateSkill`: nach Erfolg `draft.documentation` in dritten
  Editor-Tab füllen; beim finalen Save als Body-Field mitsenden.
- `submitCodeDraft` / `submitTemplateSkill`: Documentation-Field mitsenden.

### 6.3 Translations (15 Sprachen, `ui/lang/`)

Neue i18n-Keys (Beispiele):

```
skills.documentation_title              "Agent Manual"
skills.documentation_empty              "No manual yet. Add a Markdown guide so the agent knows how to use this skill."
skills.documentation_upload             "Upload .md"
skills.documentation_edit               "Edit"
skills.documentation_save               "Save Manual"
skills.documentation_delete             "Delete Manual"
skills.documentation_uploaded           "Manual uploaded"
skills.documentation_saved              "Manual saved"
skills.documentation_deleted            "Manual deleted"
skills.documentation_too_large          "Manual exceeds 64 KB limit"
skills.documentation_invalid_type       "Only .md or .txt files are allowed"
skills.has_documentation_badge          "Has agent manual"
skills.cheatsheet_links_label           "Linked Cheatsheets"
skills.generate_includes_manual         "AI will also draft an agent manual"
```

Sprachen, die zwingend übersetzt werden: cs, da, de, el, en, es, fr, hi,
it, ja, nl, no, pl, pt, sv, zh (15 — siehe AGENTS.md Regel).

---

## 7. Tool-Manuals & Prompts

### 7.1 `prompts/tools_manuals/skill_manager.md`

- Endpoint-Tabelle erweitern um die 4 neuen `/documentation`-Routen.
- Hinweis zu `documentation` im Upload/Template/Generate-Body.
- Bundle-Format-Notiz: Bundle führt `documentation`-Feld.

### 7.2 `prompts/tools_manuals/skills_engine.md`

- Neuer Abschnitt **"Skill manuals"**:
  > Skills können eine optionale Bedienungsanleitung haben. Wenn
  > `list_skills` für einen Skill `has_documentation: true` liefert, rufe
  > **vor** der ersten Verwendung `get_skill_documentation(name)` auf.
  > Die Anleitung beschreibt Parameter, Output-Schema, typische
  > Fehlerfälle und Beispielcalls.

### 7.3 Native-Tool-Definition

Falls `get_skill_documentation` als eigenes Tool registriert wird:
`internal/agent/native_tools_*.go` (oder vorhandene Skill-Engine-Datei)
um die Definition + Dispatcher-Branch erweitern.

---

## 8. Manual-Doku (`documentation/manual/{de,en}/19-skills.md`)

Neue Sektion:

- **Skill manuals (DE: Skill-Anleitungen)**
  - Wofür?
  - Wie anlegen (UI / Upload / AI-Draft)
  - Wie der Agent sie nutzt (`get_skill_documentation`)
  - Größenlimit, Markdown-Konventionen, empfohlene Struktur
  - Verhalten bei Export/Import

API-Reference (`21-api-reference.md`) um die 4 neuen Endpunkte ergänzen
(DE + EN).

---

## 9. Tests

### 9.1 Unit (`internal/tools/`)

- `skill_manager_test.go`:
  - `SetSkillDocumentation`/`GetSkillDocumentation`/`DeleteSkillDocumentation` Roundtrip.
  - `SyncFromDisk` erkennt `.md` automatisch.
  - Größenlimit (64 KB +1 → Error).
  - Hash-Drift-Detection.
- `skill_history_test.go`:
  - Export → Import-Roundtrip mit Doku.
  - Import ohne Doku-Feld bricht nicht.
  - Import mit Doku schreibt `.md`-Datei.

### 9.2 Server (`internal/server/skills_handlers_test.go`)

- 4 neue Endpunkte: 200/400/403/413/404-Pfade.
- Read-only-Mode blockiert PUT/DELETE/Upload.
- AI-Draft: Antwort enthält `documentation`-Feld; Repair-Path; Placeholder-Reject.

### 9.3 Frontend

Manueller Smoke-Test reicht (kein E2E-Framework eingebunden):
- Upload, Edit, Delete, AI-Draft inkl. Doku, Import-Bundle mit Doku.

---

## 10. Sicherheits-/Permission-Aspekte

| Aspekt | Maßnahme |
|---|---|
| **XSS** | Markdown beim Render mit `DOMPurify` oder `marked` mit `sanitize: true`. Kein `innerHTML` ohne Sanitizing. |
| **Path-Traversal** | Doku-Pfad wird **immer** aus `<name>.md` derived, niemals aus User-Input. |
| **Secret-Leakage** | `Sensitive-Scrubber` läuft auch über Doku-Inhalt vor Auslieferung an Agent. |
| **DoS / Storage** | Hartes 64-KB-Limit pro Skill. |
| **Write-Permission** | An `tools.skill_manager.read_only` / `allow_uploads` gekoppelt. |
| **Audit** | Add/Update/Delete schreiben in `skill_audit_log`. |
| **Vault-Forbidden-List** | Doku-Inhalt landet **nicht** in der Python-Sandbox-Env (nur Code + Vault-Keys), daher unkritisch. Trotzdem AGENTS.md-Hinweis beachten: keine Geheimnisse in Doku schreiben → UX-Hinweis im Editor: "Don't put secrets here, the agent reads this verbatim." |

---

## 11. Migration / Backward Compatibility

- DB-Migrationen sind additiv (`ALTER TABLE ... DEFAULT ''`).
- Bestehende Skills funktionieren unverändert: `documentation_path = ''`.
- Bestehende Bundles ohne `documentation`-Feld importieren weiterhin
  problemlos (Feld ist `omitempty`).
- Kein Breaking-Change am Skill-Manifest – `documentation` ist
  `omitempty`.

---

## 12. Aufwandsschätzung (Phasen)

| Phase | Inhalt | Datei-Touchpoints |
|---|---|---|
| **P1 – Core (Backend)** | Manifest-Field, DB-Migration, `Get/Set/DeleteSkillDocumentation`, `SyncFromDisk`-Erweiterung, Audit. | `internal/tools/skills.go`, `skill_manager.go`, `skill_history.go`, neuer `skill_documentation.go` |
| **P2 – REST** | 4 neue Endpunkte + Erweiterung Upload/Template/Get/Export/Import. | `internal/server/skills_handlers_*.go`, `server_routes_tools.go` |
| **P3 – Agent-Tool** | `get_skill_documentation`-Tool + Manual-Update. | `internal/agent/native_tools_*.go`, `prompts/tools_manuals/*.md` |
| **P4 – AI-Draft** | System-Prompt erweitern, Schema, Repair, Placeholder-Check. | `internal/server/skills_handlers_templates.go` |
| **P5 – UI** | Detail-Tab, Editor, Upload, Modals, List-Badge. | `ui/skills.html`, `ui/js/skills/main.js`, `ui/css/*` |
| **P6 – i18n** | Neue Keys in 15 Sprachen. | `ui/lang/*.json` |
| **P7 – Tests** | Unit + Server-Handler-Tests. | `*_test.go` |
| **P8 – Docs** | Manual + API-Reference. | `documentation/manual/{de,en}/19-skills.md`, `21-api-reference.md` |

Phasen können sequenziell als GSD-Phasen (`/gsd:plan-phase` → `/gsd:execute-phase`) abgearbeitet werden. P1+P2 sind blocker-frei, P3-P8 hängen davon ab.

---

## 13. Offene Fragen

1. **CheatSheet-Verlinkung:** Im ersten Wurf nur Datenfeld ohne UI-Picker —
   reicht das, oder soll der CheatSheet-Picker direkt mit ausgeliefert
   werden? (Vorschlag: erst Phase 2 Feature.)
2. **Doku-Versionierung:** Soll jede Doku-Änderung eine Version in
   `skill_versions` triggern, oder eigene Tabelle `skill_documentation_versions`?
   (Vorschlag: kein eigenes Versioning v1; nur Audit-Log.)
3. **Auto-Generierung beim Code-Upload:** Wenn ein Nutzer einen Skill ohne
   Doku hochlädt – Button "Generate manual via AI" anbieten? (Bonus,
   nicht Teil des MVP.)
4. **Agent darf Doku schreiben?** Aktuell: nein – Doku ist Nutzer-kuratiert.
   Falls ja, müsste `set_skill_documentation` als Agent-Tool exponiert
   werden, was Prompt-Injection-Risiko erhöht. (Vorschlag: für v1 ausschließen.)

---

## 14. Done-Definition

- [ ] Skill mit Doku anlegen via UI funktioniert.
- [ ] AI-Draft erzeugt Code **und** Doku.
- [ ] `get_skill_documentation` ist als Agent-Tool aufrufbar und liefert das Markdown.
- [ ] Export-/Import-Bundle führt die Doku verlustfrei mit.
- [ ] 15 Sprachen vollständig übersetzt.
- [ ] Tool-Manuals + Manual-Doku aktualisiert.
- [ ] Unit-Tests grün (`go test ./internal/tools/... ./internal/server/...`).
- [ ] Read-only-Modus blockiert Schreib-Endpunkte.
- [ ] Markdown-Render im UI ist sanitisiert.
