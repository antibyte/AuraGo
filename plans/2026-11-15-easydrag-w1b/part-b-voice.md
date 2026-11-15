## Part B: Voice (rules, "du", ledger, error texts, own-action notifications, limits)

**Goal.** EasyDrag speaks with one voice. The rules and the word ledger are written down (`documentation/easydrag/voice.md`) and two lints keep them (`TestEasyDragGermanUsesLowercaseDu`, `TestEasyDragLocalesUseLedgerWords`). The capital "Du" forms become "du", the palette is "Schritte" and "Baustein" means only the reusable block. About 140 error texts are rewritten (B.3 to B.5, other files). The nine own-action notifications of spec §4.5 go or move to the control, and every capped text input gets `ED.kit.limit`. Spec: `plans/2026-11-15-easydrag-w1b-spec.md` §4 (decisions 1, 2, 5 and 6 of §1 apply).

**Shell and worktree.** Git Bash. Every command starts with `cd <worktree> && ...`. `W` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice`.

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1b-voice -b feat/easydrag-w1b-voice main
mkdir -p C:/disposable/easydrag-w1b D:/tmp/go-build
cd W && ls plans/2026-11-15-easydrag-w1b/keys/
```

Expected: the last command lists `b-du.json  b-errors-1.json  b-errors-2.json  b-errors-3.json  b-limit.json  b-notify.json  b-rename.json` (the coordinator commits the key files to main before this part starts). If one is missing, stop and ask the coordinator; do not write it by hand.

**Baseline (before any change).**

```
cd W && npm run test:easydrag:t0
```

Expected: the run ends without a FAIL line. Note the time (about 35-55 s). Then record the line counts the ratchet rule of B.6 and B.7 measures against:

```
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-dialogs.js ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-detail.js ui/js/desktop/apps/easydrag-kit.js
```

Expected on main 065b6fca8: `896`, `753`, `889`, `309`, `928` for the first five (`-editor`, `-interact`, `-home`, `-dialogs`, `-canvas`), `-detail` 878 and `-kit` 173 (these two only for the record). If main has moved, take the printed numbers as the ceilings: **editor, interact, home and dialogs must end at or below their baseline, canvas at or below 928, detail at or below its baseline.**

**The edit script.** The JS, CSS and test files of this part are CRLF in a Windows checkout, and most edits are multi-line or repeat a long snippet, so the edits of B.2, B.6 and B.7 are not made by hand. They sit in this file as *edit groups*: the blocks between `<!-- edits:NAME -->` and `<!-- /edits -->`, each with the exact old text and the exact new text (`**Edit**`) or a text to append (`**Append**`). The script below applies one group, keeps every file's line endings, requires each old text to occur exactly once, and writes nothing unless every edit of the group matches. Write it once with the Write tool to `C:/disposable/easydrag-w1b/apply-plan-edits.mjs`:

```js
#!/usr/bin/env node
// Applies the edit group NAME of a plan file to the files under the current directory (or --root DIR).
//   node apply-plan-edits.mjs <plan.md> <NAME> [--root DIR] [--check]
// The plan holds the group between the lines "<!-- edits:NAME -->" and "<!-- /edits -->":
//   **Edit** `path`          followed by the fences "Old:" and "New:" (language tag text)
//   **Append** `path`        followed by one fence: text added at the end of the file
// Every Old text must occur exactly once in the file. The file keeps its line endings (LF or CRLF), the
// plan texts are LF. Nothing is written unless every edit of the group matches. --check only verifies.
import fs from 'node:fs';
import path from 'node:path';

const args = process.argv.slice(2);
const flag = name => { const i = args.indexOf(name); if (i < 0) return null; args.splice(i, 1); return true; };
const check = !!flag('--check');
let root = process.cwd();
const rootAt = args.indexOf('--root');
if (rootAt >= 0) { root = path.resolve(args[rootAt + 1]); args.splice(rootAt, 2); }
const [planFile, name] = args;
if (!planFile || !name) { console.error('usage: apply-plan-edits.mjs <plan.md> <NAME> [--root DIR] [--check]'); process.exit(2); }

const lines = fs.readFileSync(planFile, 'utf8').replace(/\r\n/g, '\n').split('\n');
const begin = lines.indexOf('<!-- edits:' + name + ' -->');
if (begin < 0) { console.error('no edit group ' + name + ' in ' + planFile); process.exit(2); }
let end = begin + 1;
while (end < lines.length && lines[end] !== '<!-- /edits -->') end++;
if (end >= lines.length) { console.error('edit group ' + name + ' has no closing marker'); process.exit(2); }

// fence reads the next fenced block at or after index i: returns {text, next}.
function fence(i) {
    while (i < end && !lines[i].startsWith('```')) i++;
    if (i >= end) throw new Error('a fence is missing near line ' + (i + 1));
    const body = [];
    for (i++; i < end && !lines[i].startsWith('```'); i++) body.push(lines[i]);
    return { text: body.join('\n'), next: i + 1 };
}

const jobs = [];
for (let i = begin + 1; i < end; i++) {
    const head = /^\*\*(Edit|Append)\*\* `([^`]+)`/.exec(lines[i]);
    if (!head) continue;
    if (head[1] === 'Edit') {
        const a = fence(i + 1), b = fence(a.next);
        jobs.push({ kind: 'edit', file: head[2], old: a.text, text: b.text });
        i = b.next - 1;
    } else {
        const a = fence(i + 1);
        jobs.push({ kind: 'append', file: head[2], text: a.text });
        i = a.next - 1;
    }
}
if (!jobs.length) { console.error('edit group ' + name + ' is empty'); process.exit(2); }

const files = new Map();
const load = file => {
    if (!files.has(file)) {
        const full = path.join(root, file);
        const raw = fs.readFileSync(full, 'utf8');
        files.set(file, { full, raw, eol: raw.includes('\r\n') ? '\r\n' : '\n' });
    }
    return files.get(file);
};
let failed = 0;
for (const job of jobs) {
    const f = load(job.file);
    const eol = s => s.replace(/\n/g, f.eol);
    if (job.kind === 'append') {
        f.raw = (f.raw.endsWith('\n') ? f.raw : f.raw + f.eol) + eol(job.text) + f.eol;
        console.log('ok   append ' + job.file);
        continue;
    }
    const old = eol(job.old);
    const first = f.raw.indexOf(old);
    const count = first < 0 ? 0 : (f.raw.indexOf(old, first + old.length) < 0 ? 1 : 2);
    if (count !== 1) { failed++; console.log('FAIL ' + job.file + ': ' + (count ? 'more than one' : 'no') + ' match for: ' + job.old.split('\n')[0].slice(0, 90)); continue; }
    f.raw = f.raw.slice(0, first) + eol(job.text) + f.raw.slice(first + old.length);
    console.log('ok   edit   ' + job.file + ' (' + job.old.split('\n')[0].trim().slice(0, 60) + ')');
}
if (failed) { console.log(failed + ' edit(s) did not match; nothing was written.'); process.exit(1); }
if (!check) for (const f of files.values()) fs.writeFileSync(f.full, f.raw);
console.log((check ? 'checked ' : 'applied ') + jobs.length + ' edit(s) of group ' + name + ' in ' + files.size + ' file(s).');
```

`PLAN` below is `plans/2026-11-15-easydrag-w1b/part-b-voice.md` (the path of this file inside the worktree). A group is applied with `cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs PLAN <name>`; add `--check` to verify without writing.

**Go commands** in this part use `GOTMPDIR='D:\tmp\go-build'` for `./ui` and the prefix `GOTOOLCHAIN=go1.27.4`.

**Depends on.** Nothing. Parts A and C run in parallel. Part A edits `easydrag-editor.js`, `-home.js`, `-dialogs.js`, `-interact.js` and `-canvas.js` (menus, keyboard, shortcut table) in other functions than this part, and writes locale keys of its own. Rebase rule: on a conflict in `ui/lang/easydrag/*.json` take main's files and re-run this part's key files in task order (`b-du`, `b-rename`, `b-errors-1`, `b-errors-2`, `b-errors-3`, `b-notify`, `b-limit`); on a conflict in a JS file keep both edits (they sit in different functions). The key files are idempotent.

**Facts the plan rests on (checked in the code, 2026-11-15, main 065b6fca8).**
- The 29 German keys with a capital "Du" form are listed in B.1. The scan rule is the lint's rule: placeholders removed, capital allowed at the start of the text and after `. `, `! `, `? `, `: ` and a line break.
- Banned words today: de 6 keys (3 whole words: `canvas_role` "Node", `pack_blocks` and `pack_success_detail` "Blöcke"; 3 compounds: `pack_intro`, `pack_content`, `error_flow_pack_unsupported` with "Blockversionen"/"Blockinhalte"), en 13 keys (11 "building block(s)" and `canvas_role`, `authoring_notice` "node"). No text carries "Workflow", "Revision" or "Paket" outside placeholders: the `{revision}` and `{nodes}` placeholders are removed first.
- Announcers: `core.say(ed, text)` (`easydrag-core.js:141`) speaks through the editor's polite live region `[data-ed-live]` (`ed.announce`, set by the canvas, `easydrag-canvas.js:852`); the home has its own `voice = core.announcer(el.querySelector('[data-ed-home-live]'))` (`easydrag-home.js:136`). Both exist, so this part adds no announcer.
- The desktop notification is `ed.ctx.notify` / `ctx.notify`; it shows `.vd-toast` elements.
- Capped text inputs (grep `maxlength` in `easydrag-*.js`): 8 inputs plus the goal textarea of the authoring dialog (no `maxlength`, limit 4,000 code points). See B.7.
- `ui/css/desktop-app-easydrag.css` has CRLF line endings; the edit script keeps them.

---

### Task B.1: The voice guide, the "du" lint and the 29 converted keys

**Files:**
- Create: `documentation/easydrag/voice.md`
- Create: `ui/desktop_easydrag_voice_test.go` (the "du" lint; B.2 appends the ledger lint)
- Modify: `ui/lang/easydrag/de.json` (through `plans/2026-11-15-easydrag-w1b/keys/b-du.json`; the 15 other locale files are rewritten with the same texts, so git shows only `de.json`)
- Test: `ui/desktop_easydrag_voice_test.go`

Design: the lint reads `de.json` through the existing `readEasyDragLang` (`ui/desktop_easydrag_test.go:180`). It removes `{name}` and `{{name}}`, finds `Du`, `Dich`, `Dir`, `Dein`, `Deine`, `Deinem`, `Deinen`, `Deiner`, `Deines` as whole words and accepts them only at the start of the text or after a sentence end. Whole words, not the prefix "Dein": "Deinstallieren" must not match. The conversion key file (`b-du.json`) carries all 16 locales for each of the 29 keys (the helper needs that), with today's texts for the 15 locales that do not change and the new text for `de`.

- [ ] **Step 1: Write the failing lint** `W/ui/desktop_easydrag_voice_test.go`

```go
package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The voice lints of EasyDrag (W1b part B, documentation/easydrag/voice.md). They read the shipped
// locale files; a text that breaks a rule fails the test with the key and the offending word.

// easyDragVoicePlaceholder matches {name} and {{name}}. The lints remove placeholders before they read a text.
var easyDragVoicePlaceholder = regexp.MustCompile(`\{+[^{}]*\}+`)

// easyDragCapitalDu matches the capital forms of German "du" as whole words: Du, Dich, Dir and Dein
// with an ending. Whole words, so "Deinstallieren" does not match.
var easyDragCapitalDu = regexp.MustCompile(`\b(?:Du|Dich|Dir|Dein(?:e|em|en|er|es)?)\b`)

// easyDragAtSentenceStart reports whether before ends where a sentence begins: nothing before, or a
// sentence end, a colon or a line break.
func easyDragAtSentenceStart(before string) bool {
	if before == "" {
		return true
	}
	for _, mark := range []string{". ", "! ", "? ", ": ", "\n"} {
		if strings.HasSuffix(before, mark) {
			return true
		}
	}
	return false
}

// easyDragCapitalDuForms returns the capital "du" forms inside a German text (voice guide rule 3).
func easyDragCapitalDuForms(text string) []string {
	text = easyDragVoicePlaceholder.ReplaceAllString(text, "")
	var found []string
	for _, loc := range easyDragCapitalDu.FindAllStringIndex(text, -1) {
		if !easyDragAtSentenceStart(text[:loc[0]]) {
			found = append(found, text[loc[0]:loc[1]])
		}
	}
	return found
}

func TestEasyDragGermanUsesLowercaseDu(t *testing.T) {
	// The lint itself: a capital form inside a sentence is found, one at a sentence start is not.
	for text, want := range map[string]int{
		"Prüfe das, bevor Du startest.":     1,
		"Du startest.":                      0,
		"Fertig. Du kannst weiter.":         0,
		"Wähle: Dein Entwurf bleibt.":       0,
		"Zeile eins\nDir gehört das.":       0,
		"Deinstallieren ist nicht möglich.": 0,
		"{name}: bei Dir und Deinen Daten":  2,
		"bevor du deine Daten prüfst":       0,
		"Es hängt von Dir ab":               1,
	} {
		if got := len(easyDragCapitalDuForms(text)); got != want {
			t.Errorf("lint self-check %q: %d capital forms, want %d", text, got, want)
		}
	}
	var bad []string
	for key, text := range readEasyDragLang(t, "de") {
		if forms := easyDragCapitalDuForms(text); len(forms) > 0 {
			bad = append(bad, key+": "+strings.Join(forms, ", "))
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("%d German text(s) write du with a capital inside a sentence (voice guide, rule 3):\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
```

- [ ] **Step 2: Run it and see it fail for the stated reason**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragGermanUsesLowercaseDu' -count=1
```

Expected: `--- FAIL: TestEasyDragGermanUsesLowercaseDu` with `29 German text(s) write du with a capital inside a sentence` and 29 key lines such as `easydrag.ui.authoring_stale: Deine`. The self-check table passes (no `lint self-check` line).

- [ ] **Step 3: Convert the 29 keys** (key file `plans/2026-11-15-easydrag-w1b/keys/b-du.json`)

The 29 keys and their old and new German texts (only the capital letters change):

| Key (`easydrag.`) | de old | de new |
|---|---|---|
| `ui.error_flow_mission_missing` | Der Mission-Control-Eintrag des Flows fehlt. Repariere die Mission-Control-Verknüpfung, bevor Du erneut veröffentlichst. | Der Mission-Control-Eintrag des Flows fehlt. Repariere die Mission-Control-Verknüpfung, bevor du erneut veröffentlichst. |
| `ui.error_flow_test_data_redacted` | Ersetze ausgeblendete oder ausgelassene Beispielwerte, bevor Du einen echten Test startest. | Ersetze ausgeblendete oder ausgelassene Beispielwerte, bevor du einen echten Test startest. |
| `ui.recipe_setup_review` | Prüfe die benötigten Integrationen, bevor Du dieses Rezept verwendest. | Prüfe die benötigten Integrationen, bevor du dieses Rezept verwendest. |
| `ui.repair_mission_done` | Die Mission-Control-Verknüpfung ist repariert. Aktiviere den Flow, wenn Du bereit bist. | Die Mission-Control-Verknüpfung ist repariert. Aktiviere den Flow, wenn du bereit bist. |
| `ui.repair_mission_hint` | Stelle nur den fehlenden Mission-Control-Eintrag wieder her. Der veröffentlichte Flow bleibt pausiert, bis Du ihn aktivierst. | Stelle nur den fehlenden Mission-Control-Eintrag wieder her. Der veröffentlichte Flow bleibt pausiert, bis du ihn aktivierst. |
| `ui.run_next_action` | Öffne den fehlgeschlagenen Schritt und prüfe seine Einstellungen und Eingaben, bevor Du erneut testest. | Öffne den fehlgeschlagenen Schritt und prüfe seine Einstellungen und Eingaben, bevor du erneut testest. |
| `ui.simulation_hint` | Reine Verarbeitungsschritte laufen lokal. Externe Aktionen und Modellaufrufe verwenden nur die von Dir angegebenen Beispielausgaben. Fehlende Beispiele stoppen die Simulation. | Reine Verarbeitungsschritte laufen lokal. Externe Aktionen und Modellaufrufe verwenden nur die von dir angegebenen Beispielausgaben. Fehlende Beispiele stoppen die Simulation. |
| `ui.simulation_invalid` | Gib gültige JSON-Beispielausgaben ein, bevor Du simulierst. | Gib gültige JSON-Beispielausgaben ein, bevor du simulierst. |
| `ui.test_data_incomplete` | Ersetze ausgeblendete oder ausgelassene Beispielwerte, bevor Du einen echten Test startest. | Ersetze ausgeblendete oder ausgelassene Beispielwerte, bevor du einen echten Test startest. |
| `ui.authoring_stale` | Der Entwurf oder die Beschreibung wurde geändert, während der Vorschlag erstellt wurde. Lass einen neuen Vorschlag erstellen, damit Deine Änderungen erhalten bleiben. | Der Entwurf oder die Beschreibung wurde geändert, während der Vorschlag erstellt wurde. Lass einen neuen Vorschlag erstellen, damit deine Änderungen erhalten bleiben. |
| `ui.authoring_title` | Beschreibe Deine Automatisierung | Beschreibe deine Automatisierung |
| `ui.sample_fields_hint` | Einfache Felder kannst Du unten bearbeiten. Verwende JSON für verschachtelte Werte oder zusätzliche Felder. | Einfache Felder kannst du unten bearbeiten. Verwende JSON für verschachtelte Werte oder zusätzliche Felder. |
| `ui.library_preferences_limit` | Dieses Gerät kann bis zu 1.000 Favoriten und 20 Filter speichern. Entferne einen ungenutzten Eintrag, bevor Du einen weiteren hinzufügst. | Dieses Gerät kann bis zu 1.000 Favoriten und 20 Filter speichern. Entferne einen ungenutzten Eintrag, bevor du einen weiteren hinzufügst. |
| `template.manual_pdf.text_1` | Schreibe hier Deine Notiz. | Schreibe hier deine Notiz. |
| `ui.transform_preview_hint` | Die Vorschau läuft in Deinem Browser. Datum, HTML, Unicode-Groß-/Kleinschreibung und Rundung können auf dem Server anders ausfallen. Prüfe Deinen Entwurf mit einem Test. | Die Vorschau läuft in deinem Browser. Datum, HTML, Unicode-Groß-/Kleinschreibung und Rundung können auf dem Server anders ausfallen. Prüfe deinen Entwurf mit einem Test. |
| `ui.transform_stale` | Der Entwurf hat sich geändert oder Deine Texteingabe ist noch nicht abgeschlossen. Beende die Eingabe; öffne die Hilfe nach einer Entwurfsänderung erneut. | Der Entwurf hat sich geändert oder deine Texteingabe ist noch nicht abgeschlossen. Beende die Eingabe; öffne die Hilfe nach einer Entwurfsänderung erneut. |
| `ui.error_flow_version_note_conflict` | Die Versionsnotiz wurde zwischenzeitlich geändert. Lade sie neu, bevor Du Deine Änderung speicherst. | Die Versionsnotiz wurde zwischenzeitlich geändert. Lade sie neu, bevor du deine Änderung speicherst. |
| `ui.metadata_reload_confirm` | Beim Neuladen werden Deine ungespeicherten Metadatenänderungen verworfen und durch die zuletzt gespeicherten Werte ersetzt. Fortfahren? | Beim Neuladen werden deine ungespeicherten Metadatenänderungen verworfen und durch die zuletzt gespeicherten Werte ersetzt. Fortfahren? |
| `ui.group_invalid` | Die gespeicherten visuellen Gruppen lassen sich nicht sicher anzeigen. Prüfe ihre Daten, bevor Du sie entfernst. | Die gespeicherten visuellen Gruppen lassen sich nicht sicher anzeigen. Prüfe ihre Daten, bevor du sie entfernst. |
| `ui.group_readonly` | Diese Version ist schreibgeschützt. Ausklappen ändert nur Deine Ansicht. | Diese Version ist schreibgeschützt. Ausklappen ändert nur deine Ansicht. |
| `ui.extraction_conflict` | Die Auslagerung wurde auf dem Server gespeichert, aber Dein lokaler Entwurf hat sich geändert. Wähle, ob Du Deine lokalen Änderungen behältst oder den gespeicherten Entwurf lädst. | Die Auslagerung wurde auf dem Server gespeichert, aber dein lokaler Entwurf hat sich geändert. Wähle, ob du deine lokalen Änderungen behältst oder den gespeicherten Entwurf lädst. |
| `ui.extraction_recover_hint` | Wähle, ob Du Deine lokalen Änderungen behältst oder den aktuellen Server-Entwurf lädst. Ein bereits erstellter Baustein bleibt in der Bibliothek. Fehlende Daten allein beweisen nicht, dass die Auslagerung fehlgeschlagen ist. | Wähle, ob du deine lokalen Änderungen behältst oder den aktuellen Server-Entwurf lädst. Ein bereits erstellter Baustein bleibt in der Bibliothek. Fehlende Daten allein beweisen nicht, dass die Auslagerung fehlgeschlagen ist. |
| `ui.extraction_stale` | Entwurf, Auswahl oder Name haben sich geändert. Behalte Deine Eingaben und prüfe die Auswahl vor dem Anwenden erneut. | Entwurf, Auswahl oder Name haben sich geändert. Behalte deine Eingaben und prüfe die Auswahl vor dem Anwenden erneut. |
| `ui.error_flow_durable_capacity` | Die Grenze für gespeicherte Läufe ist erreicht. Prüfe bestehende wartende oder blockierte Läufe, bevor Du einen weiteren startest. | Die Grenze für gespeicherte Läufe ist erreicht. Prüfe bestehende wartende oder blockierte Läufe, bevor du einen weiteren startest. |
| `ui.error_flow_durable_generation_conflict` | Der gespeicherte Laufzustand hat sich geändert. Lade seinen aktuellen Status neu und prüfe ihn, bevor Du erneut stoppst. | Der gespeicherte Laufzustand hat sich geändert. Lade seinen aktuellen Status neu und prüfe ihn, bevor du erneut stoppst. |
| `ui.error_flow_mission_mismatch` | Die Flow-Missions-Zuordnung wurde geändert. Prüfe die Verknüpfung, bevor Du den Flow änderst. | Die Flow-Missions-Zuordnung wurde geändert. Prüfe die Verknüpfung, bevor du den Flow änderst. |
| `ui.error_flow_pack_conflict` | Der vorgeschlagene Import steht im Konflikt mit vorhandenen Daten. Prüfe den Importstatus, bevor Du eine neue Prüfung beginnst. | Der vorgeschlagene Import steht im Konflikt mit vorhandenen Daten. Prüfe den Importstatus, bevor du eine neue Prüfung beginnst. |
| `ui.pack_finish_status_text` | Der Import ist weiterhin unbestätigt. Prüfe die Flow-Bibliothek und Mission Control, bevor Du ein weiteres Pack importierst. Dadurch enden nur die lokalen Statusprüfungen; serverseitige Daten werden nicht geändert. | Der Import ist weiterhin unbestätigt. Prüfe die Flow-Bibliothek und Mission Control, bevor du ein weiteres Pack importierst. Dadurch enden nur die lokalen Statusprüfungen; serverseitige Daten werden nicht geändert. |
| `ui.error_flow_durable_approval_unavailable` | Der Freigabestatus konnte nicht bestätigt werden. Prüfe den Status erneut, bevor Du entscheidest. | Der Freigabestatus konnte nicht bestätigt werden. Prüfe den Status erneut, bevor du entscheidest. |

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-du.json
cd W && git diff --stat -- ui/lang/easydrag
```

Expected: the helper prints `b-du.json: 29 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`. `git diff --stat` lists exactly one file: `ui/lang/easydrag/de.json | 58 ++++++++++++++++++-----------------` with `29 insertions(+), 29 deletions(-)`. If another locale file shows up, a key file text differs from today's; stop and report.

Overlap with B.3 to B.5: eight of these keys are error texts (`error_flow_mission_missing`, `error_flow_test_data_redacted`, `error_flow_version_note_conflict`, `error_flow_durable_capacity`, `error_flow_durable_generation_conflict`, `error_flow_mission_mismatch`, `error_flow_pack_conflict`, `error_flow_durable_approval_unavailable`). The error key files rewrite them later, in lowercase; converting them here keeps this commit green on its own.

- [ ] **Step 4: Run the lint and the locale tests and see them pass**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragGermanUsesLowercaseDu|TestEasyDragUIKeysExistInAllLocales)' -count=1
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/flows -run 'TestCatalogTranslationsAreComplete' -count=1
```

Expected: `ok  aurago/ui` and `ok  aurago/internal/flows`.

- [ ] **Step 5: Write the guide** `W/documentation/easydrag/voice.md` (the folder does not exist yet; the Write tool creates it)

````markdown
# EasyDrag voice guide

How EasyDrag talks to its user: the rules, the word ledger and the lints that keep them. It applies to
every text under `ui/lang/easydrag/` (16 locales) and to the texts the code builds. English and German
are written first; the other 14 locales are translated from the English text through key files and
`locale-apply.mjs`, and a native speaker should review them.

Decided on 2026-11-14 and 2026-11-15 (EasyDrag W1b, spec `plans/2026-11-15-easydrag-w1b-spec.md`).

## Rules

### 1. Outcome first

A text says first what it means for the user, then what to do. The mechanism and log hints come last.

- en: "The run was stopped. Start it again; details are in the AuraGo log."
- de: "Der Lauf ist abgebrochen. Starte ihn noch einmal; Details stehen im AuraGo-Log."
- not: "The flow execution failed unexpectedly. Details are in the AuraGo log."

### 2. A next step

Every error ends with what to do, or says that nothing needs doing.

- en: "The draft changed on the server. Reload it, then apply your edit again."
- de: "Der Entwurf hat sich auf dem Server geändert. Lade ihn neu und wende deine Änderung dann noch einmal an."
- en, nothing to do: "The flow was already paused. Nothing changed."
- de, nothing to do: "Der Flow war schon pausiert. Es hat sich nichts geändert."

### 3. One form of address

German uses the informal "du": lowercase "du", "dich", "dir", "dein", "deine", "deinem", "deinen",
"deiner", "deines". A capital only at the start of a sentence (after a full stop, "!", "?", ":" or a
line break) or at the start of the text. English addresses the user as "you".

- de: "Prüfe die Auswahl, bevor du sie anwendest."
- de, sentence start: "Du kannst den Schritt jetzt testen."
- not: "Prüfe die Auswahl, bevor Du sie anwendest."

Lint: `TestEasyDragGermanUsesLowercaseDu` (`ui/desktop_easydrag_voice_test.go`).

### 4. One word per concept

The ledger below fixes the word for each concept, in every locale. A second word for the same concept is
a bug, and so is one word for two concepts. The lint `TestEasyDragLocalesUseLedgerWords` bans the
words that the ledger replaced.

### 5. Limits

Always-visible text names no limit. A counter ("1,812 / 2,000") appears from 90 % of the limit. A rule
hint ("from 1 second to 24 hours") is not shown in advance; it becomes the error message when an input
breaks the rule. An input that is capped without a rule says "At most 120 characters." (de: "Höchstens
120 Zeichen.") when the cap is reached. Help that the user opens may name limits. The helper is
`ED.kit.limit` (`ui/js/desktop/apps/easydrag-kit.js`).

- en, always visible: "Name"; shown at the cap: "At most 120 characters."
- de, always visible: "Name"; shown at the cap: "Höchstens 120 Zeichen."
- not, always visible: "Names have up to 120 characters."

A count that describes a value ("1,204 characters" of a preview) is not a limit and stays.

### 6. The user's own actions

No visual notification when the screen already shows the result: publishing (the header shows the new
version), switching a flow on or off (the switch shows it), restoring a version (the draft shows it).
Where the result is invisible (the clipboard), a short confirmation appears at the control itself
("Copied" on the button for two seconds, without motion). Screen-reader announcements stay and replace
every removed notification. Failures, partial results and skips keep their notification.

- en: publishing sends no toast; a screen reader hears "“Daily digest” is published."
- de: "„Tagesrückblick“ ist veröffentlicht."

### 7. Only what the code guarantees

A text says "nothing has changed" or "nothing was sent" only when the error code means exactly that.
Read the code behind an error before you write its text. When the outcome is not known, say that it
is not known and what to check.

- en: "The result is not confirmed. Check the flow list before you try again."
- de: "Das Ergebnis ist nicht bestätigt. Prüfe die Flow-Liste, bevor du es noch einmal versuchst."

## Concept ledger

| Concept | de | en | Meaning | Where it appears |
|---|---|---|---|---|
| Flow | Flow | flow | one automation | home cards, header, dialogs |
| Step | Schritt | step | one card on the canvas, and the palette of step types | canvas, palette (`palette_title`), step dialog, search |
| Building block | Baustein | block | a reusable published sub-flow | blocks list (`blocks_title`), block editor, "Call block" step |
| Trigger | Auslöser | trigger | the step a flow starts from | cards, palette, issue texts |
| Draft | Entwurf | draft | the editable state | header, save state, versions |
| Published state | vN | vN | one published state (v1, v2, ...) | header, versions dialog |
| Run | Lauf | run | one execution | run drawer, footer, home cards |
| Simulation | Simulation | simulation | a test run with sample outputs and no effects | test dialog |
| Template | Vorlage | template | a flow to start from | home template cards |
| Pack | Pack | pack | a local export or import bundle | pack import and export |

Words that the ledger replaced and that the lint bans (whole words, placeholders removed first):

- de: Workflow, Knoten, Node, Block, Blöcke, Blocks, Revision, Paket (and their plurals), and compounds
  such as "Blockversion" (write "Bausteinversion").
- en: workflow, node, building block, revision, package.

"Baustein" and "block" are not banned: they are the ledger words for the reusable sub-flow. They must not
mean a step, so the lint also checks a list of step texts (the palette, the search, the unknown-step
messages) for them.

### Open in the ledger

- "Vorlage" (template) next to "Rezept" (recipe): the Wave 3 recipe cards decide which word stays.
- "Simulation" next to a real test run: W1d settles the two words when it rewrites the test loop.
- "Testbank" (test bench) is not yet a word of the user interface.

## Keeping the rules

- Locale files change only through key files for `locale-apply.mjs`; all 16 locales for every key.
- Placeholders (`{name}`) are the same in every locale.
- New texts are written against these rules. Run `go test ./ui -run '(?i)easydrag(GermanUsesLowercaseDu|LocalesUseLedgerWords)'`
  after a change of `de.json` or `en.json`.
````

- [ ] **Step 6: Gates**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && gofmt -l ui/desktop_easydrag_voice_test.go
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(easydrag)' -count=1 -short
```

Expected: vet silent, gofmt prints nothing, `ok  aurago/ui` (the browser tests skip without `AURAGO_RUN_BROWSER_SMOKE=1`).

- [ ] **Step 7: Commit**

Message file `C:/disposable/easydrag-w1b/b1-msg.txt` (Write tool):

```
feat(easydrag): voice guide and the lowercase "du" lint

New documentation/easydrag/voice.md: the seven rules of the EasyDrag voice
and the concept ledger (flow, step, building block, trigger, draft, vN, run,
simulation, template, pack) with the open items. New lint
TestEasyDragGermanUsesLowercaseDu fails on a capital Du, Dich, Dir or Dein...
inside a German sentence (placeholders removed first). The 29 German texts
that broke the rule are converted; the other 15 locales are unchanged.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add documentation/easydrag/voice.md ui/desktop_easydrag_voice_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1b/b1-msg.txt
```

Expected: one commit with 3 files (`voice.md`, the test and `de.json`), because the other 15 locale files were rewritten with identical content.

---

### Task B.2: "Baustein" means only the reusable block (decision 5) and the ledger lint

**Files:**
- Modify: `ui/desktop_easydrag_voice_test.go` (append the ledger lint)
- Modify: `ui/lang/easydrag/*.json` (through `plans/2026-11-15-easydrag-w1b/keys/b-rename.json`: 20 keys, all 16 locales)
- Test: `ui/desktop_easydrag_voice_test.go`

Design: three kinds of change, all in one key file.
1. **Step sense** (13 keys): "Baustein"/"building block" means a step or a step type. de becomes "Schritt", en becomes "step", and the 14 other locales are translated again from the new en, because they use their word for "block" in the palette and in the unknown-step texts too (for example cs "Stavební bloky", fr "Blocs", ja "ブロック"), which is the confusion of decision 5. Two of the 13 keys only need one language group: `panel_resize_palette` (only de was wrong) and `rail_palette` (the rail label of the same palette: it, pt and ja used another word than the new panel title, so they follow the word the rest of their locale uses for "step").
2. **The word "node"** (2 keys, en and the other locales): `canvas_role` (the screen-reader role of the canvas: "Node-Editor" / "node editor" becomes "Flow-Editor" / "flow editor") and `authoring_notice` (en "node catalog" becomes "step catalog"; the other locales already say "steps").
3. **Block sense, de only** (5 keys): `pack_blocks` "Lokale Blöcke" becomes "Lokale Bausteine", and the compounds "Blockversionen", "Blockkopien" and "Blockinhalte" become "Bausteinversionen", "Bausteinkopien" and "Bausteininhalte" (`pack_intro`, `pack_content`, `error_flow_pack_unsupported`, `pack_success_detail`). Here the sense is already the block; only the German word was wrong. en keeps "block".

The lint bans, on de and en only and after removing placeholders: de Workflow, Knoten, Node, Block, Blöcke, Blocks, Revision, Paket (also their plurals and the compounds Blockversion, Blockkopie, Blockinhalt, Blockeingabe, Blockausgabe); en workflow, node, building block, revision, package (also plurals). A second check keeps the 12 step keys of the list below free of "Baustein" and "block" in de and en, so a later key file (the error texts of B.3 to B.5 rewrite two of them) cannot bring the confusion back.

- [ ] **Step 1: Find every text that says "Baustein" or "building block" and decide its sense**

```
cd W && node -e "const fs=require('fs');const de=JSON.parse(fs.readFileSync('ui/lang/easydrag/de.json','utf8')),en=JSON.parse(fs.readFileSync('ui/lang/easydrag/en.json','utf8'));let n=0;for(const k of Object.keys(de)){if(/baustein/i.test(de[k])||/building.?block/i.test(en[k])){n++;console.log(k.replace('easydrag.',''))}}console.log(n+' keys')"
```

Expected: 61 keys (on main 065b6fca8; the last line prints the count). Sort them with this rule: **a text is the step sense when it talks about the palette, the search for steps, a step type that does not exist or is not available, or "click ... together into automations"; it is the block sense when it names a reusable sub-flow (its name, inputs, outputs, versions, call, extraction, dependencies, the blocks list, a pack's blocks).** The result for main 065b6fca8 (the five block-sense German rows are the last five):

| Key (`easydrag.` prefix left out) | de old | de new | en old | en new | Why it is the step sense |
|---|---|---|---|---|---|
| `ui.palette_title` | Bausteine | Schritte | Building blocks | Steps | title of the step palette (`easydrag-palette.js`), the rail already says "Schritte" (`rail_palette`) |
| `ui.palette_search` | Bausteine suchen | Schritte suchen | Search building blocks | Search steps | search field of the palette |
| `ui.keys_search` | Bausteine suchen | Schritte suchen | Search building blocks | Search steps | Ctrl+K opens `searchPalette`, which searches step types (`easydrag-editor.js:548`, shortcut table `easydrag-dialogs.js:34`) |
| `ui.palette_hide` | Bausteine ausblenden | Schritte ausblenden | Hide building blocks | Hide steps | closes the step palette |
| `ui.palette_no_results` | Kein Baustein gefunden. | Kein Schritt gefunden. | No building block found. | No step found. | empty result of the palette search |
| `ui.badge_unknown` | Dieser Baustein ist nicht verfügbar. | Dieser Schritt ist nicht verfügbar. | This building block is not available. | This step is not available. | badge on a canvas card whose step type is missing from the catalog (`easydrag-canvas.js:417`, `-detail.js:353`) |
| `ui.error_node_type_unknown` | Dieser Baustein ist nicht verfügbar. | Dieser Schritt ist nicht verfügbar. | This building block is not available. | This step is not available. | error for an unknown step type |
| `ui.error_flow_node_unavailable` | Der Baustein ist nicht verfügbar. | Der Schritt ist nicht verfügbar. | The building block is not available. | The step is not available. | code `FLOW_NODE_UNAVAILABLE`: the step cannot run (its tool needs setup or is denied) |
| `ui.issue_node_type_unknown` | Diesen Baustein gibt es nicht (mehr). | Diesen Schritt gibt es nicht (mehr). | This building block does not exist (anymore). | This step does not exist (anymore). | publish issue about a step type |
| `ui.issue_node_unavailable` | Dieser Baustein ist gerade nicht verfügbar. | Dieser Schritt ist gerade nicht verfügbar. | This building block is not available right now. | This step is not available right now. | publish issue about a step |
| `ui.home_empty_text` | Klicke Bausteine zu Automationen zusammen: ein Auslöser, ein paar Schritte, fertig. ... | Klicke Schritte zu Automationen zusammen: ein Auslöser, ein paar weitere Schritte, fertig. ... | Click building blocks together into automations: a trigger, a few steps, done. ... | Click steps together into automations: a trigger, a few more steps, done. ... | the same sentence already calls its parts "steps"; the trigger is a step too |
| `ui.panel_resize_palette` | Breite der Baustein-Bibliothek ändern | Breite der Schrittbibliothek ändern | Resize step library (unchanged) | (unchanged) | width of the step palette; only de was wrong, the 14 other locales already say "step" |
| `ui.rail_palette` | Schritte | (unchanged) | Steps | (unchanged) | the rail button of the palette; only it "Passi" → "Passaggi", pt "Passos" → "Etapas", ja "手順" → "ステップ" change, so the rail and the panel title say the same word |
| `ui.canvas_role` | Node-Editor | Flow-Editor | node editor | flow editor | role description of the canvas ("node" is not a ledger word) |
| `ui.authoring_notice` | (unchanged: Schrittkatalog) | (unchanged) | ... the node catalog ... | ... the step catalog ... | the catalog lists step types |
| `ui.pack_blocks` | Lokale Blöcke ({count}) | Lokale Bausteine ({count}) | Local blocks ({count}) | (unchanged) | block sense, only the German word changes |
| `ui.pack_success_detail` | ... seine lokalen Blöcke ... | ... seine lokalen Bausteine ... | (unchanged) | (unchanged) | block sense |
| `ui.pack_intro` | ... die genauen Blockversionen ... lokale Blockkopien ... | ... die genauen Bausteinversionen ... lokale Bausteinkopien ... | (unchanged) | (unchanged) | block sense |
| `ui.pack_content` | Enthaltene Flow- und Blockinhalte | Enthaltene Flow- und Bausteininhalte | (unchanged) | (unchanged) | block sense |
| `ui.error_flow_pack_unsupported` | ... festgelegten Blockversionen. | ... festgelegten Bausteinversionen. | (unchanged) | (unchanged) | block sense (pinned block versions) |

Rows 1 to 13 are the step sense (13 keys: the 11 with "building block" in en, `panel_resize_palette` and `rail_palette`). Rows 14 and 15 are the word "node" (2 keys). Rows 16 to 20 are the block sense, German word only (5 keys). 20 keys in all, all in `keys/b-rename.json`; the 14 other locales change only in rows 1 to 11 (the step sense), in `canvas_role` ("flow editor" in each language) and, for it, pt and ja, in `rail_palette`.

**Kept as the block sense (about 45 keys, not touched):** `ui.blocks_title`, `ui.blocks_empty`, `ui.block_*` (new, name, delete, simulate, publish hint, pins hint, contract unavailable, dependencies hint, nested title and hint, self reference), `node.block_input.*`, `node.block_output.*`, `node.component_call.*`, `node.component_each.*`, `ui.batch_item_field`, `ui.extraction_*` (title, intro, apply, budget, groups, name invalid, reason texts, recover hint, success, undo hint, unknown), `ui.error_flow_extraction_*`, `ui.error_flow_block_in_use`, `ui.error_flow_block_unavailable`, `ui.error_flow_child_history_unavailable`, `ui.child_history_*`, `ui.child_document_*`, `ui.cred_purge_live_block_*`. Each of them names the reusable sub-flow, its versions, its call or its history.

- [ ] **Step 2: Append the ledger lint to the failing test** `W/ui/desktop_easydrag_voice_test.go`

Group `b2-lint` appends it to the file (the imports `regexp`, `sort`, `strings` and `testing` are already there):

<!-- edits:b2-lint -->

**Append** `ui/desktop_easydrag_voice_test.go`

```text

// easyDragBannedDe and easyDragBannedEn are the words the concept ledger replaced (voice guide rule 4).
// Whole words (a letter, digit or underscore next to the word means it is part of a longer word),
// placeholders removed first. "Baustein" and "block" are ledger words and not banned.
var (
	easyDragBannedDe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])(Workflows?|Knoten|Nodes?|Blocks?|Blöcke[n]?|Revisionen?|Pakete?n?)(?:[^\p{L}\p{N}_]|$)`)
	easyDragBannedEn = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(workflows?|nodes?|building blocks?|revisions?|packages?)(?:[^\p{L}\p{N}_]|$)`)
	// easyDragBannedDeCompound catches the German compounds of "Block" that mean a building block.
	easyDragBannedDeCompound = regexp.MustCompile(`\bBlock(?:version|kopie|inhalt|eingabe|ausgabe)\w*`)
)

// easyDragStepKeys are the texts that talk about a step or a step type. They must not call it a
// building block: "Baustein" and "block" belong to the reusable sub-flow (decision 5 of W1b).
var easyDragStepKeys = []string{
	"easydrag.ui.palette_title", "easydrag.ui.palette_search", "easydrag.ui.palette_hide", "easydrag.ui.palette_no_results",
	"easydrag.ui.keys_search", "easydrag.ui.badge_unknown", "easydrag.ui.error_node_type_unknown", "easydrag.ui.error_flow_node_unavailable",
	"easydrag.ui.issue_node_type_unknown", "easydrag.ui.issue_node_unavailable", "easydrag.ui.home_empty_text", "easydrag.ui.panel_resize_palette",
}

func TestEasyDragLocalesUseLedgerWords(t *testing.T) {
	// The lint itself.
	for _, c := range []struct {
		de   bool
		text string
		want bool
	}{
		{true, "Lokale Blöcke ({count})", true},
		{true, "Node-Editor", true},
		{true, "Die Blockversionen", true},
		{true, "Lokale Bausteine ({count})", false},
		{true, "Blockiert", false},
		{true, "Version v{revision} ({nodes} Knoten)", true},
		{true, "Version v{revision} mit {nodes} Schritten", false},
		{false, "This building block is not available.", true},
		{false, "Search building blocks", true},
		{false, "The node catalog", true},
		{false, "Local blocks ({count})", false},
		{false, "Version v{revision} and {nodes} steps", false},
		{false, "The agent permissions block this action.", false},
	} {
		text := easyDragVoicePlaceholder.ReplaceAllString(c.text, "")
		hit := easyDragBannedEn.MatchString(text)
		if c.de {
			hit = easyDragBannedDe.MatchString(text) || easyDragBannedDeCompound.MatchString(text)
		}
		if hit != c.want {
			t.Errorf("lint self-check %q (de %v): banned %v, want %v", c.text, c.de, hit, c.want)
		}
	}
	var bad []string
	for lang, banned := range map[string]*regexp.Regexp{"de": easyDragBannedDe, "en": easyDragBannedEn} {
		for key, text := range readEasyDragLang(t, lang) {
			text = easyDragVoicePlaceholder.ReplaceAllString(text, "")
			if m := banned.FindStringSubmatch(text); m != nil {
				bad = append(bad, lang+" "+key+": "+m[1])
			} else if m := easyDragBannedDeCompound.FindString(text); lang == "de" && m != "" {
				bad = append(bad, lang+" "+key+": "+m)
			}
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d text(s) use a word the concept ledger replaced (voice guide, rule 4):\n%s", len(bad), strings.Join(bad, "\n"))
	}
	// A step text must not call the step a building block.
	de, en := readEasyDragLang(t, "de"), readEasyDragLang(t, "en")
	for _, key := range easyDragStepKeys {
		if strings.Contains(strings.ToLower(de[key]), "baustein") {
			t.Errorf("de %s calls a step a building block: %q", key, de[key])
		}
		if strings.Contains(strings.ToLower(en[key]), "block") {
			t.Errorf("en %s calls a step a block: %q", key, en[key])
		}
	}
}
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b2-lint
cd W && gofmt -l ui/desktop_easydrag_voice_test.go
```

Expected: `applied 1 edit(s) of group b2-lint in 1 file(s).` and gofmt prints nothing.

- [ ] **Step 3: Run it and see it fail for the stated reason**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragLocalesUseLedgerWords' -count=1
```

Expected: `--- FAIL`. The message `19 text(s) use a word the concept ledger replaced` lists 6 de lines (`canvas_role: Node`, `pack_blocks: Blöcke`, `pack_success_detail: Blöcke`, `pack_intro`, `pack_content`, `error_flow_pack_unsupported` with the compound) and 13 en lines (11 `building block(s)`, `canvas_role: node`, `authoring_notice: node`). Below it come 12 lines `de easydrag.ui... calls a step a building block` (all 12 step keys) and 11 lines `en ... calls a step a block` (all but `panel_resize_palette`, which already says "step" in en). The self-check table passes.

- [ ] **Step 4: Run the key file and see the lint pass**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-rename.json
cd W && git diff --stat -- ui/lang/easydrag
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragLocalesUseLedgerWords|TestEasyDragGermanUsesLowercaseDu|TestEasyDragUIKeysExistInAllLocales)' -count=1
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/flows -run 'TestCatalogTranslationsAreComplete' -count=1
```

Expected: the helper prints `b-rename.json: 20 keys set, 0 deleted, in 16 files of ui/lang/easydrag/`; `git diff --stat` lists all 16 locale files: `de.json` 18 changed lines, `en.json` 13, and each of the 14 others 12 (the 11 step keys of rows 1 to 11 and `canvas_role`), 13 for it, pt and ja (plus `rail_palette`); both Go commands print `ok`.

- [ ] **Step 5: Check that no test or code expects the old words**

```
cd W && git grep -n "Bausteine suchen\|Kein Baustein\|Search building blocks\|Building blocks\|Node-Editor" -- scripts ui/*.go ui/js internal
```

Expected: no output. (The manual `documentation/manuals/de/24-easydrag.md` and the English manual still call steps "Bausteine"/"building blocks" and use a capital "Deine"; they are outside the lints. See "Decisions needed".)

- [ ] **Step 6: Gates**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && gofmt -l ui/desktop_easydrag_voice_test.go
cd W && node scripts/test-easydrag.mjs
```

Expected: vet silent, gofmt prints nothing, the Node run ends with `all EasyDrag checks passed`.

- [ ] **Step 7: Commit**

Message file `C:/disposable/easydrag-w1b/b2-msg.txt`:

```
feat(easydrag): "Baustein" means only the reusable block, ledger lint

The step palette and the unknown-step texts said "Baustein" / "building
block" for a step; the reusable sub-flow said the same. Step texts now say
"Schritt" / "step" in de and en (13 keys), the 14 other locales are
translated again from the new en. canvas_role and authoring_notice lose the
word "node". The German pack texts say "Bausteine" (pack_blocks, pack_intro,
pack_content, pack_success_detail, error_flow_pack_unsupported). New lint
TestEasyDragLocalesUseLedgerWords bans the replaced words in de and en
(placeholders removed first) and keeps the step texts free of "block".

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/desktop_easydrag_voice_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1b/b2-msg.txt
```

---

### Tasks B.3, B.4 and B.5: the error texts (other files)

The 140 error and recovery texts of spec §4.3 (the `easydrag.ui.error_*` keys, `durable_reason_*`, `approval_blocked_*` and `load_failed`) are planned in three files, one per writer. Run them in this order, after B.2 and before B.6. Each task there ends with its own commit and its own key file; the lints of B.1 and B.2 run in every one of them.

| Task | File | Key file |
|---|---|---|
| B.3 | `part-b-errors-1.md` | `plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json` |
| B.4 | `part-b-errors-2.md` | `plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json` |
| B.5 | `part-b-errors-3.md` | `plans/2026-11-15-easydrag-w1b/keys/b-errors-3.json` |

Two keys of B.2 are error texts again: `error_node_type_unknown` and `error_flow_node_unavailable`. The error writers rewrite them with "Schritt"/"step"; the step-key check of `TestEasyDragLocalesUseLedgerWords` fails if one of them brings "Baustein" or "block" back.

---

### Task B.6: Own-action notifications (spec §4.5, decision 1)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-publish.js` (236, 254, 320), `-blocks.js` (183), `-home.js` (421, 424, 477, 659), `-home-grid.js` (the `lastRun` line and the state pull), `-editor.js` (477), `-extraction.js` (81), `-authoring.js` (93), `-detail.js` (779-781)
- Modify: `ui/js/desktop/apps/easydrag-kit.js` (`ED.kit.flash`)
- Modify: `ui/css/desktop-app-easydrag.css` (the flash rule; CRLF)
- Modify: `ui/lang/easydrag/*.json` (through `plans/2026-11-15-easydrag-w1b/keys/b-notify.json`: new `copy_json_done`, deletes `copied_json`)
- Modify (tests): `scripts/test-easydrag-extra2.mjs` (188 and the harness), `scripts/test-easydrag-extra7.mjs` (117), `scripts/test-easydrag-kit.mjs` (flash test), `ui/desktop_easydrag_browser_test.go` (3 call lines in `TestDesktopEasyDragBrowser`)
- Create: `ui/desktop_easydrag_voice_browser_test.go` (helpers for the journey, and the home Run now check)
- Test: `scripts/test-easydrag.mjs` (runs the three Node files), `TestDesktopEasyDragBrowser`, `TestDesktopEasyDragHomeRunNowBrowser`

**What changes at the nine call sites** (voice guide rule 6). A visual notification goes where the screen already shows the result; the screen-reader announcement replaces it, through the announcer that exists at that place.

| Key | Call site (main 065b6fca8) | Result on screen | Now |
|---|---|---|---|
| `publish_done` | `publish.js:236` | the header shows the new vN | `core.say(ed, ...)` (editor live region) |
| `active_on`, `active_off` | `publish.js:254` | the switch | `core.say(ed, ...)` |
| `versions_restored` | `publish.js:320` and `blocks.js:183` | the draft and the header | `core.say(ed, ...)` |
| `run_started` | `home.js:659` | none yet: the card's last-run line | the card shows "Läuft" with the `running` disc until the next list load, plus `voice.announce(...)` (home live region); a skipped run keeps its notification |
| `repair_mission_done` | `editor.js:477` | the issue disappears | `canvas.announce(...)` |
| `extraction_success` | `extraction.js:81` | the canvas shows the new block call | `core.say(ed, ...)` |
| `authoring_applied` | `authoring.js:93` | the draft shows the change | `core.say(ed, ...)` |
| `secret_deleted` | `fields.js:578` | **kept**: it is a warning ("Still used by ...: flows that use it fail"), shown only when flows still use the secret, so it is a partial result and the list does not show it. See "Deviations" | unchanged |
| `copied_json` | `detail.js:780` | invisible (the clipboard) | "Kopiert" on the button for 2 s, no motion (`ED.kit.flash`), plus `core.say` |

Failure, partial and skip notifications stay: `publish_partial`, `run_now_skipped`, every `type: 'error'`.

Design: `ED.kit.flash(button, text, ms)` adds one `<span class="ed-flash-text">` to the button, sets `data-ed-flash`, the label and the title to the text, and after 2 s (`FLASH_MS`) removes the span and restores label and title. The stylesheet hides the icon while `data-ed-flash` is set. Nothing moves or fades, so it needs no reduced-motion rule. The home gets one new state: `runningNow`, a `Set` of flow ids that were started with Run now; the grid draws "Läuft" with the status disc for them; the next list load clears it. Line rule: `easydrag-home.js` ends at or below its baseline (every edit below replaces a line by a line), `easydrag-editor.js` stays equal, `easydrag-detail.js` loses a line.

- [ ] **Step 1: IMPACT and the facts**

Before changing these functions run (the symbols: `setActive`, `publish` and `restoreVersion` handler live in `easydrag-publish.js`; `runNow` in `easydrag-home.js`; `adopt` in `easydrag-extraction.js`):

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "setActive" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "runNow" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "adopt" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "lastRun" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Warn on HIGH or CRITICAL. UNKNOWN is unresolved: confirm with `cd W && git grep -n "setActive(\|runNow(\|lastRun(" -- ui/js/desktop/apps`. Expected callers: `setActive` from the switch handler, the publish dialog and the menus of `easydrag-editor.js`; `runNow` from `cardMenu` and the card in `easydrag-home.js`; `lastRun` from the card template in `easydrag-home-grid.js` only. None of the edits changes a signature or a return value.

Check the line endings (most JS files are CRLF in a Windows checkout) and that the announcers exist where the edits use them:

```
cd W && node -e "const fs=require('fs');for(const f of ['easydrag-publish','easydrag-home','easydrag-home-grid','easydrag-editor','easydrag-detail','easydrag-kit']){const s=fs.readFileSync('ui/js/desktop/apps/'+f+'.js','utf8');console.log(f,/\r\n/.test(s)?'CRLF':'LF')}"
cd W && git grep -n "function say\|ed.announce = announce\|const voice = core.announcer" -- ui/js/desktop/apps
```

Expected: the `say` definition in `easydrag-core.js`, `ed.announce = announce` in `easydrag-canvas.js`, `voice = core.announcer` in `easydrag-home.js`. The edit script of the next step keeps each file's line endings.

- [ ] **Step 2: Check that every edit group of this task matches the worktree**

```
cd W && for g in b6-tests b6-kit b6-calls b6-browser; do node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md $g --check | tail -1; done
```

Expected: four lines `checked N edit(s) of group ...` (4, 3, 15 and 3 edits). A `FAIL` line names the file and the first line of the old text that no longer matches: main has moved. Read the current line, correct the old text of that edit in the plan, and check again.

- [ ] **Step 3: Write the failing tests first**

The three Node checks change before the code does. Group `b6-tests`:

<!-- edits:b6-tests -->

**Edit** `scripts/test-easydrag-extra2.mjs`

Old:

```text
[h.ed.publishIncomplete, enables(), h.ed.flowEnabled, h.notes.slice(1).map(n => n.message)], ['', 1, true, ['publish_done:Flow']]);
```

New:

```text
[h.ed.publishIncomplete, enables(), h.ed.flowEnabled, h.notes.slice(1).map(n => n.message), h.announced], ['', 1, true, [], ['publish_done:Flow']]);
```

**Edit** `scripts/test-easydrag-extra2.mjs`

Old:

```text
ctx: { notify: n => notes.push(n), confirmDialog: async () => false }, t, esc: ED.core.esc,
```

New:

```text
ctx: { notify: n => notes.push(n), confirmDialog: async () => false }, announce: text => announced.push(text), t, esc: ED.core.esc,
```

**Edit** `scripts/test-easydrag-extra7.mjs`

Old:

```text
[1, true, [['versions_title', 'versions_restored:2', undefined]], false, undefined, []]
```

New:

```text
[1, true, [], false, undefined, []]
```

**Edit** `scripts/test-easydrag-kit.mjs`

Old:

```text
ED.core.modal(noWidth, { title: 'T', body: '' }).el.classList.contains('ed-sheet'), false);
```

New:

```text
ED.core.modal(noWidth, { title: 'T', body: '' }).el.classList.contains('ed-sheet'), false);

    // flash(): the text sits on the button for 2 s, then label, title and look come back
    {
        const button = new dom.El('button', { 'aria-label': 'Copy JSON', title: 'Copy JSON' });
        const icon = button.appendChild(new dom.El('svg', {}));
        kit.flash(button, 'Copied');
        const timer = timers[timers.length - 1];
        eq('kit flash shows the text, names the button by it and arms 2 seconds',
            [button.querySelector('.ed-flash-text').textContent, button.getAttribute('aria-label'), button.getAttribute('title'), button.getAttribute('data-ed-flash'), timer.ms, button.children.includes(icon)],
            ['Copied', 'Copied', 'Copied', '', 2000, true]);
        kit.flash(button, 'Copied');
        eq('kit flash twice restarts the time and keeps one text', [button.querySelectorAll('.ed-flash-text').length, timer.fn], [1, null]);
        timers[timers.length - 1].fn();
        eq('kit flash gives the button back its label, title and look',
            [button.querySelector('.ed-flash-text'), button.getAttribute('aria-label'), button.getAttribute('title'), button.getAttribute('data-ed-flash')], [null, 'Copy JSON', 'Copy JSON', null]);
    }
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b6-tests
cd W && node scripts/test-easydrag.mjs
```

Expected: the script prints `ok   edit ...` four times and `applied 4 edit(s) of group b6-tests in 3 file(s).` The Node run prints `FAIL c1d06 a full publish clears the flag and activates ...` (the old code still notifies: `h.notes` has the toast and `h.announced` is empty) and `FAIL a1023 F1 an unsaved draft ...` (the restore still notifies), and then stops with `TypeError: kit.flash is not a function` in `test-easydrag-kit.mjs`. These are the failures the next steps remove.

- [ ] **Step 4: The key file, `ED.kit.flash` and its CSS**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-notify.json
```

Expected: `b-notify.json: 1 keys set, 1 deleted, in 16 files of ui/lang/easydrag/`. New key `easydrag.ui.copy_json_done`:

| Key | en | de |
|---|---|---|
| `easydrag.ui.copy_json_done` | Copied | Kopiert |

(Deleted: `easydrag.ui.copied_json`, "Copied JSON to clipboard." / "JSON in die Zwischenablage kopiert.", unused after this task.) The other 14 locales are in the key file.

Group `b6-kit`:

<!-- edits:b6-kit -->

**Edit** `ui/js/desktop/apps/easydrag-kit.js`

Old:

```text
    const TOAST_MS = 8000;
```

New:

```text
    const TOAST_MS = 8000;
    // FLASH_MS is how long a button shows its confirmation (flash).
    const FLASH_MS = 2000;
```

**Edit** `ui/js/desktop/apps/easydrag-kit.js`

Old:

```text
    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar };
```

New:

```text
    // flash shows text on a button for ms (default 2 s) and then gives the button back its own look: the
    // text is one added span (the stylesheet hides the icon while data-ed-flash is set), and the label and
    // title are restored. Nothing moves or fades. A second flash on the same button restarts the time.
    function flash(button, message, ms) {
        if (!button) return;
        if (button.edFlash) clearTimeout(button.edFlash.timer);
        else {
            const span = document.createElement('span');
            span.className = 'ed-flash-text';
            button.appendChild(span);
            button.edFlash = { span, label: button.getAttribute('aria-label'), title: button.getAttribute('title') };
        }
        const saved = button.edFlash;
        saved.span.textContent = text(message);
        button.setAttribute('data-ed-flash', '');
        button.setAttribute('aria-label', text(message));
        button.setAttribute('title', text(message));
        saved.timer = setTimeout(() => {
            saved.span.remove();
            if (saved.label == null) button.toggleAttribute('aria-label', false); else button.setAttribute('aria-label', saved.label);
            if (saved.title == null) button.toggleAttribute('title', false); else button.setAttribute('title', saved.title);
            button.toggleAttribute('data-ed-flash', false);
            button.edFlash = null;
        }, ms || FLASH_MS);
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash };
```

**Edit** `ui/css/desktop-app-easydrag.css`

Old:

```text
.ed-icon-btn--small { width: 24px; height: 24px; }
```

New:

```text
.ed-icon-btn--small { width: 24px; height: 24px; }
.ed-icon-btn[data-ed-flash] { width: auto; min-width: 30px; padding: 0 8px; color: var(--ed-ok-ink); font-size: var(--ed-font-meta); font-weight: 600; white-space: nowrap; }
.ed-icon-btn[data-ed-flash] > :not(.ed-flash-text) { display: none; }
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b6-kit
cd W && node -e "const s=require('fs').readFileSync('ui/css/desktop-app-easydrag.css','utf8');console.log((s.match(/\r\n/g)||[]).length,(s.match(/\n/g)||[]).length)"
```

Expected: `applied 3 edit(s) of group b6-kit in 2 file(s).` and the two CSS counts are equal (every line still ends in CRLF).

- [ ] **Step 5: Move or remove the call sites** (group `b6-calls`, 15 edits in 8 files)

<!-- edits:b6-calls -->

**Edit** `ui/js/desktop/apps/easydrag-publish.js` (line 236, publish_done)

Old:

```text
ed.ctx.notify({ title: t('easydrag.ui.publish'), message: t('easydrag.ui.publish_done', { name: ed.model.doc.name }) });
```

New:

```text
core.say(ed, t('easydrag.ui.publish_done', { name: ed.model.doc.name }));
```

**Edit** `ui/js/desktop/apps/easydrag-publish.js` (line 254, active_on and active_off)

Old:

```text
if (!quiet) ed.ctx.notify({ title: ed.model.doc.name, message: on ? t('easydrag.ui.active_on') : t('easydrag.ui.active_off') });
```

New:

```text
if (!quiet) core.say(ed, on ? t('easydrag.ui.active_on') : t('easydrag.ui.active_off'));
```

**Edit** `ui/js/desktop/apps/easydrag-publish.js` (line 320, versions_restored in the flow editor)

Old:

```text
ed.ctx.notify({ title: t('easydrag.ui.versions_title'), message: t('easydrag.ui.versions_restored', { revision: rev }) });
```

New:

```text
core.say(ed, t('easydrag.ui.versions_restored', { revision: rev }));
```

**Edit** `ui/js/desktop/apps/easydrag-blocks.js` (line 183, versions_restored in the block editor)

Old:

```text
ed.ctx.notify({ title: ed.t('easydrag.ui.versions_title'), message: ed.t('easydrag.ui.versions_restored', { revision: rev }) });
```

New:

```text
ED.core.say(ed, ed.t('easydrag.ui.versions_restored', { revision: rev }));
```

**Edit** `ui/js/desktop/apps/easydrag-home.js` (line 421: the set of flows started with Run now, declared before the grid reads the state)

Old:

```text
        const gridView = ED.homeGrid.create({
```

New:

```text
        const runningNow = new Set(); const gridView = ED.homeGrid.create({
```

**Edit** `ui/js/desktop/apps/easydrag-home.js` (line 424: the state the grid pulls)

Old:

```text
organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError })
```

New:

```text
organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError, runningNow })
```

**Edit** `ui/js/desktop/apps/easydrag-home.js` (line 477: the next list load clears the "running" state)

Old:

```text
            flows = list.flows || [];
```

New:

```text
            flows = list.flows || []; runningNow.clear();
```

**Edit** `ui/js/desktop/apps/easydrag-home.js` (line 659, run_started)

Old:

```text
if (!gone) ctx.notify({ title: f.name, message: t(skipped ? 'easydrag.ui.run_now_skipped' : 'easydrag.ui.run_started') });
```

New:

```text
if (!gone) { if (skipped) ctx.notify({ title: f.name, message: t('easydrag.ui.run_now_skipped') }); else { runningNow.add(f.id); render(); voice.announce(t('easydrag.ui.run_started')); } }
```

**Edit** `ui/js/desktop/apps/easydrag-home-grid.js` (the declaration of the pulled state)

Old:

```text
            organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError;
```

New:

```text
            organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError, runningNow;
```

**Edit** `ui/js/desktop/apps/easydrag-home-grid.js` (the pull itself)

Old:

```text
                organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError } = host.state());
```

New:

```text
                organizationFolder, organizationTag, selectedFlows, loadError, staleError, timersRunning, templateError, runningNow } = host.state());
```

**Edit** `ui/js/desktop/apps/easydrag-home-grid.js` (the last-run line of the card)

Old:

```text
            const r = f.last_run;
            if (!r) return '<span class="ed-muted">'
```

New:

```text
            if (runningNow.has(f.id)) return ED.status.label(t, 'running'); const r = f.last_run;
            if (!r) return '<span class="ed-muted">'
```

**Edit** `ui/js/desktop/apps/easydrag-editor.js` (line 477, repair_mission_done)

Old:

```text
ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.repair_mission_done') });
```

New:

```text
canvas.announce(t('easydrag.ui.repair_mission_done'));
```

**Edit** `ui/js/desktop/apps/easydrag-extraction.js` (line 81, extraction_success)

Old:

```text
ed.ctx.notify({ title: t('easydrag.ui.extraction_title'), message: t('easydrag.ui.extraction_success') });
```

New:

```text
core.say(ed, t('easydrag.ui.extraction_success'));
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js` (line 93, authoring_applied)

Old:

```text
ed.ctx.notify({ title: t('easydrag.ui.authoring_title'), message: t('easydrag.ui.authoring_applied') });
```

New:

```text
core.say(ed, t('easydrag.ui.authoring_applied'));
```

**Edit** `ui/js/desktop/apps/easydrag-detail.js` (lines 779-781, copied_json becomes "Kopiert" on the button)

Old:

```text
                if (ed.ctx && typeof ed.ctx.notify === 'function') {
                    ed.ctx.notify({ title: node.label || node.type, message: t('easydrag.ui.copied_json') });
                }
```

New:

```text
                ED.kit.flash(event.target.closest('[data-ed-copy-json]'), t('easydrag.ui.copy_json_done'));
                core.say(ed, t('easydrag.ui.copy_json_done'));
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b6-calls
```

Expected: 15 `ok   edit` lines and `applied 15 edit(s) of group b6-calls in 8 file(s).` (`-publish.js` 3 edits, `-blocks.js` 1, `-home.js` 4, `-home-grid.js` 3, `-editor.js` 1, `-extraction.js` 1, `-authoring.js` 1, `-detail.js` 1).

Proof that no old form remains:

```
cd W && git grep -nE "notify\(\{[^}]*(publish_done|active_on|active_off|versions_restored|run_started|repair_mission_done|extraction_success|authoring_applied|copied_json)" -- ui/js/desktop/apps
```

Expected: no output (no `notify({ ... })` call carries one of these keys any more; `run_now_skipped` and the failure and partial notifications are not in the pattern). The `secret_deleted` call in `easydrag-fields.js` stays on purpose.

Check the home line rule:

```
cd W && wc -l ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-detail.js
```

Expected: `-home.js` 889 (equal to the baseline: all four edits replace a line by a line), `-editor.js` 896, `-detail.js` one less than the baseline.

- [ ] **Step 6: Run the Node checks and see them pass**

```
cd W && node scripts/test-easydrag.mjs
cd W && node scripts/test-easydrag-render-baseline.mjs
```

Expected: the first run ends with `all EasyDrag checks passed` (the three failures of Step 3 are gone: `c1d06 a full publish clears the flag and activates`, `a1023 F1 an unsaved draft ...` and the three `kit flash` checks pass), the second prints `PASS render baseline (unchanged)`.

Two things can differ from the plan; the fix is stated:
- if `c1d06 a full publish ...` shows more entries in `h.announced` than `['publish_done:Flow']`, an earlier step of that test announced too: filter with `h.announced.filter(a => a.startsWith('publish_done'))` in the new line;
- if `TestEasyDragUIKeysExistInAllLocales` later fails on `copy_json_done`, the key file of Step 4 was not applied.

- [ ] **Step 7: The browser assertions** (spec §4.7: publishing and switching on send no notification and do announce; "JSON kopieren" shows "Kopiert")

Create `W/ui/desktop_easydrag_voice_browser_test.go` (tabs for indentation, like every Go file):

```go
package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// W1b B.6 (voice guide rule 6): the editor does not toast its own visible results. Publishing and the
// active switch speak through the editor's live region instead, and "JSON kopieren" confirms on its button.
// The helpers below run inside TestDesktopEasyDragBrowser; the home Run now check is its own small test.

const easyDragToastCount = `()=>document.querySelectorAll('.vd-toast').length`

// recordAnnouncements starts writing every text of the editor's live region to window.__edSpoken.
func (s *easyDragSmoke) recordAnnouncements() {
	s.t.Helper()
	s.page.MustEval(`()=>{window.__edSpoken=[];const live=document.querySelector('[data-ed-live]');new MutationObserver(()=>{if(live.textContent)window.__edSpoken.push(live.textContent)}).observe(live,{childList:true,characterData:true,subtree:true})}`)
}

// publishAndActiveAreQuiet checks, after the journey's complete publish, that it was announced and not
// toasted, and that switching the flow off and on again does the same.
func (s *easyDragSmoke) publishAndActiveAreQuiet() {
	s.t.Helper()
	page := s.page
	s.wait(`()=>{const ed=edFixture.editor();const text=ed.t('easydrag.ui.publish_done',{name:ed.model.doc.name});return window.__edSpoken.includes(text) && ![...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(text))}`)
	for i := 0; i < 2; i++ {
		page.MustEval(`()=>{window.__edSpoken.length=0}`)
		page.MustElement(`[data-ed-cmd="active"]`).MustClick()
		s.wait(`()=>{const ed=edFixture.editor();const text=ed.t(ed.flowEnabled?'easydrag.ui.active_on':'easydrag.ui.active_off');return window.__edSpoken.includes(text) && ![...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(text))}`)
	}
}

// copyJSONConfirms opens the Telegram step after the test run, copies its output and checks that the
// button says "Kopiert" for a moment, that no notification appears, and that the label comes back.
func (s *easyDragSmoke) copyJSONConfirms() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('open-detail',{nodeId:ed.model.doc.nodes.find(n=>n.type==='notify.telegram').id})}`)
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');return !!b && !b.hidden && b.getClientRects().length>0}`)
	toasts := page.MustEval(easyDragToastCount).Int()
	page.MustEval(`()=>document.querySelector('[data-ed-copy-json]').click()`)
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');const done=edFixture.editor().t('easydrag.ui.copy_json_done');return done!=='easydrag.ui.copy_json_done' && b.hasAttribute('data-ed-flash') && b.textContent.trim()===done && b.getAttribute('aria-label')===done}`)
	if got := page.MustEval(easyDragToastCount).Int(); got > toasts {
		s.t.Fatalf("copying the JSON opened a desktop notification (%d toasts before, %d after)", toasts, got)
	}
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');return !b.hasAttribute('data-ed-flash') && b.getAttribute('aria-label')===edFixture.editor().t('easydrag.ui.copy_json')}`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
}

// TestDesktopEasyDragHomeRunNowBrowser: Run now on a home card sends no notification; the card's last-run
// line says "Läuft" with the running disc, and the start is announced.
func TestDesktopEasyDragHomeRunNowBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustEval(`()=>{window.__edSpoken=[];const live=document.querySelector('[data-ed-home-live]');new MutationObserver(()=>{if(live.textContent)window.__edSpoken.push(live.textContent)}).observe(live,{childList:true,characterData:true,subtree:true})}`)
	p.MustElement(`.ed-flow-card [data-ed-card-menu]`).MustClick()
	s.wait(`()=>[...document.querySelectorAll('.vd-context-menu .vd-context-item')].some(b=>b.textContent.includes(t('easydrag.ui.run_now')) && !b.disabled)`)
	p.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(b=>b.textContent.includes(t('easydrag.ui.run_now'))).click()`)
	s.wait(`()=>{const run=document.querySelector('.ed-flow-card .ed-flow-run [data-status="running"]');return !!run && run.textContent.includes(t('easydrag.status.running')) && window.__edSpoken.includes(t('easydrag.ui.run_started')) && !document.querySelector('.vd-toast')}`)
	s.noRawKeys("home run now")
	s.failOnPageErrors("home run now")
}
```

Group `b6-browser` (three call lines in the journey):

<!-- edits:b6-browser -->

**Edit** `ui/desktop_easydrag_browser_test.go` (after the test run)

Old:

```text
	s.failOnPageErrors("test run")
```

New:

```text
	s.failOnPageErrors("test run")
	// Own actions stay quiet (W1b B.6): "JSON kopieren" confirms on its button, not in a notification.
	s.copyJSONConfirms()
```

**Edit** `ui/desktop_easydrag_browser_test.go` (before the publishes)

Old:

```text
	s.shot("publish-dialog")
```

New:

```text
	s.shot("publish-dialog")
	s.recordAnnouncements()
```

**Edit** `ui/desktop_easydrag_browser_test.go` (after the complete publish)

Old:

```text
	s.clearToasts()

	// Run drawer and read-only run view; Esc returns to the draft.
```

New:

```text
	// Publishing and the active switch announce and do not notify (W1b B.6).
	s.publishAndActiveAreQuiet()
	s.clearToasts()

	// Run drawer and read-only run view; Esc returns to the draft.
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b6-browser
cd W && gofmt -l ui/desktop_easydrag_voice_browser_test.go ui/desktop_easydrag_browser_test.go
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestDesktopEasyDragBrowser|TestDesktopEasyDragHomeRunNowBrowser)$' -count=1 -timeout 15m -v
```

Run the last command with the Bash sandbox disabled. Expected: gofmt prints nothing; the test output has `--- PASS: TestDesktopEasyDragBrowser` and `--- PASS: TestDesktopEasyDragHomeRunNowBrowser` (the journey takes about 1 to 2 minutes). The rod page has a 150 s deadline: if one fails on a timeout, run that test alone again before counting a failure.

What to do when a check fails:
- `copyJSONConfirms` waits forever for `[data-ed-copy-json]`: the output pane of the step is not shown after the test run. Open it first with `page.MustElement(".ed-detail [data-ed-narrow=\"output\"]").MustClick()` before the wait (the narrow tabs exist in the detail view) and rerun.
- `publishAndActiveAreQuiet` fails on `__edSpoken`: another announcement (the save state) replaced the live text before the observer saw it. The observer records every text, so this means the announcement did not happen: check Step 5 (`core.say(ed, ...)` in `easydrag-publish.js`).
- the switch click opens a confirmation dialog: the journey's fixture asks before pausing; then click its primary action and keep the wait.
- the home test sees `__edSpoken` without `Lauf gestartet`: the page language is not German; the checks use `t(...)` of the page, so compare against `t('easydrag.ui.run_started')` only (already so) and look at the screenshot.

Then view the Kopiert state once with a screenshot (the journey writes `D:/tmp/easydrag-w1b-shots/`; add `s.shot("copy-json-done")` after the wait in `copyJSONConfirms` for this one run, then take it out again) and check that the button shows the word "Kopiert" in the status colour, and that the detail header does not jump.

- [ ] **Step 8: Gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && gofmt -l ui/desktop_easydrag_voice_browser_test.go ui/desktop_easydrag_browser_test.go
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
cd W && npm run test:easydrag:t0
```

Expected: vet silent, gofmt prints nothing, `ok  aurago/ui`, T0 without a FAIL line.

Message file `C:/disposable/easydrag-w1b/b6-msg.txt`:

```
feat(easydrag): no notification for the user's own visible actions

Publish, the active switch, restoring a version, the repaired Mission
Control link, the block extraction and an applied proposal no longer open a
desktop notification: the screen shows the result and a screen reader hears
the same text through the editor's live region. Run now on a home card shows
"Läuft" with the running disc on the card until the next list load and
announces the start. "JSON kopieren" says "Kopiert" on the button for two
seconds (ED.kit.flash, no motion). Failures, partial results and skips keep
their notification; the "secret still used" warning stays.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-publish.js ui/js/desktop/apps/easydrag-blocks.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-home-grid.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-extraction.js ui/js/desktop/apps/easydrag-authoring.js ui/js/desktop/apps/easydrag-detail.js ui/js/desktop/apps/easydrag-kit.js ui/css/desktop-app-easydrag.css ui/lang/easydrag scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra7.mjs scripts/test-easydrag-kit.mjs ui/desktop_easydrag_browser_test.go ui/desktop_easydrag_voice_browser_test.go && git commit -F C:/disposable/easydrag-w1b/b6-msg.txt
```

---

### Task B.7: `ED.kit.limit`, the capped inputs and the two rule hints (spec §4.6, decision 2)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-kit.js` (`ED.kit.limit`)
- Modify: `ui/js/desktop/apps/easydrag-editor.js` (84), `-home.js` (802), `-detail.js` (806), `-dialogs.js` (`flowSettings`, 190-290), `-blocks.js` (`create`, 99-108), `-credentials-move.js` (94-108), `-block-fields.js` (162, 188), `-authoring.js` (6-14, 48, 58, 100-109)
- Modify: `ui/css/desktop-app-easydrag.css` (CRLF)
- Modify: `ui/lang/easydrag/*.json` (through `plans/2026-11-15-easydrag-w1b/keys/b-limit.json`: new `limit_counter`, `limit_reached`; deletes `block_fields_hint`, `flow_max_run_hint`)
- Modify (tests): `scripts/test-easydrag-kit.mjs`, `scripts/test-easydrag-extra6.mjs`, `scripts/test-easydrag-authoring.mjs`, `ui/desktop_easydrag_test_mode_browser_test.go`
- Test: `scripts/test-easydrag.mjs`

**The helper** (contract of the brief, with two optional additions): `ED.kit.limit(input, { max, counter, count, t, after })` inserts one `<div class="ed-limit" aria-live="polite" hidden>` after the input and returns `{ update, detach }`.
- `counter: true` (long text): "1.812 / 2.000" from 90 % of `max` (`length * 10 >= max * 9`), numbers through `ED.format.number`; `counter: false` (one line): "Höchstens 120 Zeichen." once `length >= max`. Both hide again when the text is shorter. The note gets the class `is-over` above `max` (warn ink) and `data-ed-limit="counter"` or `"cap"`.
- `count(value)` measures the value (default `value.length`).
- `t` is the translator of the caller (the kit has none of its own); it is required for readable texts, because the helper builds `easydrag.ui.limit_counter` and `easydrag.ui.limit_reached` with it.
- `after` is the element the note follows. Default: the `label` around the input if there is one, else the input. A note inside a label would become part of the label's accessible name, so it goes after the label.
- The input gets `aria-describedby` for the note (appended to an existing one); `detach()` removes listeners, note and the description. It listens to `input`, `change` and `blur`, so a value that code sets back (the Escape key of the title) is noticed.

**Capped inputs found by `grep -n maxlength ui/js/desktop/apps/easydrag-*.js`** (8 inputs) **plus the goal textarea** (limit in code, no `maxlength`): 9 in all, all covered here.

| # | Input | File and line | Limit | Mode | Where the note goes |
|---|---|---|---|---|---|
| 1 | flow name in the header | `editor.js:54` / 84 | 120 | cap | under the title (CSS: `.ed-title` is positioned, the note floats below it) |
| 2 | flow name in the settings dialog | `dialogs.js:219` | 120 | cap | after its label |
| 3 | flow description in the settings dialog | `dialogs.js:220` | 2000 | counter | after its label |
| 4 | home search | `home.js:99` / 802 | 4000 | cap | under the search pill (CSS floats it) |
| 5 | block name (new block dialog) | `blocks.js:99` | 120 | cap | after its label |
| 6 | secret name (move to the vault) | `credentials-move.js:94` | 40 | cap | after its label |
| 7 | step label | `detail.js:208` / 806 | 80 **code points** (input `maxlength` 160 UTF-16 units, `ui/AGENTS.md:193`) | cap, `count: value => Array.from(value).length` | the grid under the title input |
| 8 | block field name | `block-fields.js:188` | 40 | cap | after its label, a full grid row |
| 9 | authoring goal | `authoring.js` (no `maxlength`) | 4000 code points, trimmed (the server counts so) | counter | after its label; keeps the warn colour and the disabled Generate above the limit |

The spec says "2000" for the goal; the code and the server limit are 4,000 (`GOAL_MAX`, `flows_ai_proposal.go`). See "Deviations".

**The two rule hints:**
- `block_fields_hint` (`block-fields.js:162`): the always-visible line goes. The rule is already in `block_fields_invalid`, which the form shows when a name breaks it (`error()` in `boundaryForm`); the name input also gets the cap note (row 8). The key is deleted.
- `flow_max_run_hint` (`dialogs.js:226`): the always-visible hint goes. The error `flow_max_run_invalid` ("Enter whole minutes (0 to 1440) and seconds (0 to 59), together from 1 second to 24 hours.") already carries the rule and is shown when Apply finds a run time that breaks it. The number fields drop their `aria-describedby` for the hint; while the error shows they point at it, as before. The sentence "Leave both fields empty for the default of 30 minutes" goes with the hint: the fields show the default (30 and 0) anyway. The key is deleted.
- Collapsed help that the user opens keeps its limits: `group_limit`, `transform_limit`, `block_sample_limit`, `batch_limit_hint`. The preview character count (`preview_text`) describes a value and stays.

Line rule: `dialogs.js` ends at its baseline (309): the removed `hintId` line pays for the added `ED.kit.limit` line; `home.js`, `editor.js` and `detail.js` get their calls appended to an existing line.

- [ ] **Step 1: IMPACT and a check of the capped inputs**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "flowSettings" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "boundaryForm" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "goalCounter" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
cd W && git grep -n "maxlength\|maxLength" -- ui/js/desktop/apps/easydrag-*.js | cut -c1-120
```

Warn on HIGH or CRITICAL; UNKNOWN is unresolved: confirm with `git grep -n "flowSettings(\|boundaryForm(\|goalCounter(" -- ui scripts`. Expected callers: `flowSettings` from the editor menus, `boundaryForm` from `easydrag-block-fields.js` only, `goalCounter` from the authoring dialog and `test-easydrag-authoring.mjs`. The grep lists exactly the 8 `maxlength` inputs of the table (rows 1 to 8). A ninth hit means a new capped input: give it a row of its own before you go on.

- [ ] **Step 2: Write the failing tests first** (group `b7-tests`)

The kit test for the helper (spec §4.7: hidden at 50 %, counter at 90 %, cap message at 100 %, hidden again below), the two changed settings-dialog checks, the authoring test stub and the goal-counter browser test. The test file for the kit loads `easydrag-format.js` too, because the helper formats numbers with it.

<!-- edits:b7-tests -->

**Edit** `scripts/test-easydrag-kit.mjs`

Old:

```text
for (const file of ['easydrag-core.js', 'easydrag-kit.js'])
```

New:

```text
for (const file of ['easydrag-core.js', 'easydrag-format.js', 'easydrag-kit.js'])
```

**Edit** `scripts/test-easydrag-kit.mjs`

Old:

```text
    // flash(): the text sits on the button for 2 s, then label, title and look come back
```

New:

```text
    // limit(): the length note of a capped input (W1b B.7). A counter input shows "count / max" from 90 % and
    // goes over at max + 1; a one-line input shows the cap message at max; both hide again when shorter.
    const limitT = (key, vars) => (key === 'easydrag.ui.limit_counter' ? vars.count + ' / ' + vars.max : 'At most ' + vars.max + ' characters.');
    const typeInto = (input, text) => { input.value = text; input.fire('input'); };
    const shown = note => note.getAttribute('hidden') === null;
    {
        const form = new dom.El('div', {});
        const area = form.appendChild(new dom.El('textarea', {}));
        const guard = kit.limit(area, { max: 2000, counter: true, t: limitT });
        const note = form.children[1];
        eq('kit limit adds one polite live region after the input, hidden, and describes the input by it',
            [form.children.length, note.className, note.getAttribute('aria-live'), shown(note), area.getAttribute('aria-describedby') === note.id && note.id !== ''],
            [2, 'ed-limit', 'polite', false, true]);
        const at = n => { typeInto(area, 'x'.repeat(n)); return [shown(note), note.textContent, note.classList.contains('is-over')]; };
        eq('kit limit counter: hidden at 50 % and just below 90 %', [at(1000), at(1799)], [[false, '', false], [false, '', false]]);
        eq('kit limit counter: shown from 90 % in the UI number format', at(1800), [true, '1,800 / 2,000', false]);
        eq('kit limit counter: still a counter at the limit, over it one character later', [at(2000), at(2001)], [[true, '2,000 / 2,000', false], [true, '2,001 / 2,000', true]]);
        eq('kit limit counter: hidden again when the text is short', at(1000), [false, '', false]);
        guard.detach();
        eq('kit limit detach removes the note and the description', [form.children.length, area.getAttribute('aria-describedby')], [1, null]);
    }
    {
        const form = new dom.El('div', {});
        const label = form.appendChild(new dom.El('label', {}));
        const input = label.appendChild(new dom.El('input', { 'aria-describedby': 'help' }));
        kit.limit(input, { max: 120, counter: false, t: limitT });
        const note = form.children[1];
        eq('kit limit puts the note after the label, not inside it, and keeps an existing description',
            [label.children.length, form.children.length, input.getAttribute('aria-describedby')], [1, 2, 'help ' + note.id]);
        const at = n => { typeInto(input, 'x'.repeat(n)); return [shown(note), note.textContent]; };
        eq('kit limit cap message: hidden below the limit, shown at it, hidden again below', [at(0), at(119), at(120), at(125), at(60)],
            [[false, ''], [false, ''], [true, 'At most 120 characters.'], [true, 'At most 120 characters.'], [false, '']]);
    }
    {
        const form = new dom.El('div', {});
        const input = form.appendChild(new dom.El('input', {}));
        kit.limit(input, { max: 80, counter: false, count: value => Array.from(value).length, t: limitT });
        const note = form.children[1];
        const smile = String.fromCodePoint(0x1F600);
        typeInto(input, smile.repeat(79));
        const below = shown(note);
        typeInto(input, smile.repeat(80));
        eq('kit limit counts with the given function: 80 code points are the limit, 79 are not', [below, shown(note), input.value.length], [false, true, 160]);
    }

    // flash(): the text sits on the button for 2 s, then label, title and look come back
```

**Edit** `scripts/test-easydrag-extra6.mjs`

Old:

```text
        const hint = 'ed-max-run-w1-hint';
        const both = hint + ' ed-max-run-w1-error';
        eq('a1008 1.4 review: aria-describedby names the error while it shows, and only the hint once typing clears it',
            [before, shown, cleared, d.h.logged], [[hint, hint], [[both, both], false, 'ed-max-run-w1-error'], [[hint, hint], true, [null, null]], []]);
```

New:

```text
        const error = 'ed-max-run-w1-error';
        eq('a1008 1.4 review: aria-describedby is empty, names the error while it shows, and is empty again once typing clears it',
            [before, shown, cleared, d.h.logged], [[null, null], [[error, error], false, error], [[null, null], true, [null, null]], []]);
```

**Edit** `scripts/test-easydrag-extra6.mjs`

Old:

```text
        eq('a1008 1.4 seconds unit, range hint and range error in all 16 locales',
            missingKeys(apps, ['easydrag.ui.unit_seconds', 'easydrag.ui.flow_max_run_hint', 'easydrag.ui.flow_max_run_invalid']), []);
```

New:

```text
        eq('a1008 1.4 seconds unit and range error in all 16 locales',
            missingKeys(apps, ['easydrag.ui.unit_seconds', 'easydrag.ui.flow_max_run_invalid']), []);
```

**Edit** `scripts/test-easydrag-authoring.mjs`

Old:

```text
    const goalEl = fakeEl(), counterEl = fakeEl(), errorEl = fakeEl(), previewEl = fakeEl();
```

New:

```text
    const goalEl = fakeEl(), errorEl = fakeEl(), previewEl = fakeEl();
```

**Edit** `scripts/test-easydrag-authoring.mjs`

Old:

```text
'[data-ed-authoring-goal]': goalEl, '[data-ed-goal-counter]': counterEl, '.ed-error': errorEl,
```

New:

```text
'[data-ed-authoring-goal]': goalEl, '.ed-error': errorEl,
```

**Edit** `scripts/test-easydrag-authoring.mjs`

Old:

```text
    let calls = 0, release = null;
```

New:

```text
    ED.kit = { limit: () => ({ update() {}, detach() {} }) };
    let calls = 0, release = null;
```

**Edit** `ui/desktop_easydrag_test_mode_browser_test.go`

Old:

```text
&& !!document.querySelector('[data-ed-goal-counter]')`)
```

New:

```text
&& !!document.querySelector('.ed-modal .ed-limit')`)
```

**Edit** `ui/desktop_easydrag_test_mode_browser_test.go`

Old:

```text
c=document.querySelector('[data-ed-goal-counter]');if(a.hasAttribute('maxlength'))throw Error('maxlength is still set');if(!c.id || a.getAttribute('aria-describedby')!==c.id)throw Error('the counter is not linked');
```

New:

```text
c=document.querySelector('.ed-modal .ed-limit');if(a.hasAttribute('maxlength'))throw Error('maxlength is still set');if(!c.id || !(a.getAttribute('aria-describedby')||'').split(' ').includes(c.id))throw Error('the limit note is not linked');
```

**Edit** `ui/desktop_easydrag_test_mode_browser_test.go`

Old:

```text
	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]'),nf=
```

New:

```text
	s.wait(`()=>{const c=document.querySelector('.ed-modal .ed-limit'),g=document.querySelector('.ed-modal [data-ed-action="generate"]'),nf=
```

**Edit** `ui/desktop_easydrag_test_mode_browser_test.go`

Old:

```text
	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]');return !c.classList
```

New:

```text
	s.wait(`()=>{const c=document.querySelector('.ed-modal .ed-limit'),g=document.querySelector('.ed-modal [data-ed-action="generate"]');return !c.classList
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b7-tests
cd W && node scripts/test-easydrag.mjs
```

Expected: the script prints `applied 11 edit(s) of group b7-tests in 4 file(s).` The Node run stops with `TypeError: kit.limit is not a function` in `test-easydrag-kit.mjs` (after the earlier files ran). `a1008 1.4 review: aria-describedby is empty ...` fails before that (the dialog still has the hint), and the authoring check still passes. These are the failures the next steps remove.

- [ ] **Step 3: The key file, `ED.kit.limit` and the stylesheet**

```
cd W && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-limit.json
```

Expected: `b-limit.json: 2 keys set, 2 deleted, in 16 files of ui/lang/easydrag/`.

| Key | en | de |
|---|---|---|
| `easydrag.ui.limit_counter` | {count} / {max} | {count} / {max} |
| `easydrag.ui.limit_reached` | At most {max} characters. | Höchstens {max} Zeichen. |

(Deleted: `easydrag.ui.block_fields_hint`, `easydrag.ui.flow_max_run_hint`.) Group `b7-kit`:

<!-- edits:b7-kit -->

**Edit** `ui/js/desktop/apps/easydrag-kit.js`

Old:

```text
    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash };
```

New:

```text
    // limit is the length note of a capped text input (voice guide rule 5): opts {max, counter, count, t, after}.
    // counter true (long text) shows "count / max" from 90 % of max; counter false (one line) shows
    // "At most {max} characters." once the input reaches max. count(value) measures the value (default:
    // value.length; a step label counts code points). t translates. after is the element the note follows
    // (default: the label around the input, else the input). The note is one polite live region and hides
    // again when the text is shorter. Returns {update, detach}.
    function limit(input, opts) {
        const o = opts || {};
        const max = Number(o.max);
        const measure = typeof o.count === 'function' ? o.count : value => value.length;
        const say = typeof o.t === 'function' ? o.t : key => key;
        const note = document.createElement('div');
        note.className = 'ed-limit';
        note.id = 'ed-limit-' + (++serial);
        note.setAttribute('aria-live', 'polite');
        note.setAttribute('aria-atomic', 'true');
        note.toggleAttribute('hidden', true);
        const anchor = o.after || (typeof input.closest === 'function' && input.closest('label')) || input;
        if (anchor.parentNode) anchor.parentNode.insertBefore(note, anchor.nextSibling || null);
        const described = input.getAttribute('aria-describedby');
        input.setAttribute('aria-describedby', described ? described + ' ' + note.id : note.id);
        function update() {
            const length = measure(String(input.value == null ? '' : input.value));
            let kind = '';
            let message = '';
            if (o.counter) {
                if (length * 10 >= max * 9) { kind = 'counter'; message = say('easydrag.ui.limit_counter', { count: ED.format.number(length), max: ED.format.number(max) }); }
            } else if (length >= max) { kind = 'cap'; message = say('easydrag.ui.limit_reached', { max: ED.format.number(max) }); }
            if (note.textContent !== message) note.textContent = message;
            note.toggleAttribute('hidden', !kind);
            note.classList.toggle('is-over', length > max);
            note.setAttribute('data-ed-limit', kind || 'none');
        }
        const events = ['input', 'change', 'blur'];
        events.forEach(name => input.addEventListener(name, update));
        update();
        return {
            update,
            detach() {
                events.forEach(name => input.removeEventListener(name, update));
                note.remove();
                if (described) input.setAttribute('aria-describedby', described); else input.toggleAttribute('aria-describedby', false);
            }
        };
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash, limit };
```

**Edit** `ui/css/desktop-app-easydrag.css` (the title is the anchor of its note)

Old:

```text
.ed-title { flex: 1 1 12rem; min-width: 0; display: flex;
```

New:

```text
.ed-title { position: relative; flex: 1 1 12rem; min-width: 0; display: flex;
```

**Edit** `ui/css/desktop-app-easydrag.css` (the old goal counter becomes the shared limit note, in tokens of the sheet)

Old:

```text
.ed-goal-counter { margin: 4px 0 0; color: var(--ed-muted); font-size: var(--ed-font-meta); font-variant-numeric: tabular-nums; text-align: end; }
.ed-goal-counter.is-over { color: var(--ed-warn-ink); font-weight: 600; }
```

New:

```text
.ed-limit { margin: 4px 0 0; color: var(--ed-muted); font-size: var(--ed-font-meta); font-variant-numeric: tabular-nums; text-align: end; }
.ed-limit[data-ed-limit="cap"] { color: var(--ed-warn-ink); text-align: start; }
.ed-limit.is-over { color: var(--ed-warn-ink); font-weight: 600; }
.ed-title > .ed-limit, .ed-search > .ed-limit { position: absolute; top: 100%; left: 0; z-index: 2; margin: 2px 0 0; white-space: nowrap; }
```

**Edit** `ui/css/desktop-app-easydrag.css` (a note in the block field grid takes a full row)

Old:

```text
.ed-app .ed-block-field > .ed-icon-btn { justify-self: end; }
```

New:

```text
.ed-app .ed-block-field > .ed-icon-btn { justify-self: end; }
.ed-app .ed-block-field > .ed-limit { grid-column: 1 / -1; }
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b7-kit
cd W && node scripts/test-easydrag.mjs
```

Expected: `applied 4 edit(s) of group b7-kit in 2 file(s).` and the Node run now passes the 12 `kit limit` and `kit flash` checks; the dialog check of `test-easydrag-extra6.mjs` (aria-describedby) is the one that stays red until Step 4.

- [ ] **Step 4: Apply the helper to every capped input and drop the two hints** (group `b7-calls`, 19 edits in 8 files)

<!-- edits:b7-calls -->

**Edit** `ui/js/desktop/apps/easydrag-editor.js` (row 1, line 84)

Old:

```text
        const nameInput = el.querySelector('[data-ed-name]');
```

New:

```text
        const nameInput = el.querySelector('[data-ed-name]'); bag.add(ED.kit.limit(nameInput, { max: 120, counter: false, t }).detach);
```

**Edit** `ui/js/desktop/apps/easydrag-home.js` (row 4, line 802)

Old:

```text
        bag.listen(el.querySelector('[data-ed-home-search]'), 'input', event => { clearSelection(); searchUpdate(event); });
```

New:

```text
        bag.listen(el.querySelector('[data-ed-home-search]'), 'input', event => { clearSelection(); searchUpdate(event); }); bag.add(ED.kit.limit(el.querySelector('[data-ed-home-search]'), { max: 4000, counter: false, t }).detach);
```

**Edit** `ui/js/desktop/apps/easydrag-detail.js` (row 7, line 806)

Old:

```text
        bag.listen(labelInput, 'input', () => ed.model.setLabel(node.id, labelInput.value));
```

New:

```text
        bag.listen(labelInput, 'input', () => ed.model.setLabel(node.id, labelInput.value)); bag.add(ED.kit.limit(labelInput, { max: 80, counter: false, count: value => Array.from(value).length, t }).detach);
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (the hint id goes)

Old:

```text
        const hintId = runId + '-hint';
        const errorId = runId + '-error';
```

New:

```text
        const errorId = runId + '-error';
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (the run time fields no longer point at a hint)

Old:

```text
value="' + esc(value) + '" aria-describedby="' + esc(hintId) + '"'
```

New:

```text
value="' + esc(value) + '"'
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (the always-visible hint goes; the error stays and carries the rule)

Old:

```text
'<p class="ed-hint" id="' + esc(hintId) + '">' + esc(t('easydrag.ui.flow_max_run_hint')) + '</p><p class="ed-error" id="' + esc(errorId)
```

New:

```text
'<p class="ed-error" id="' + esc(errorId)
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (while the error shows, the fields point at it)

Old:

```text
f.setAttribute('aria-describedby', hintId + ' ' + errorId);
```

New:

```text
f.setAttribute('aria-describedby', errorId);
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (typing takes the error and the description back)

Old:

```text
f.setAttribute('aria-describedby', hintId);
```

New:

```text
f.removeAttribute('aria-describedby');
```

**Edit** `ui/js/desktop/apps/easydrag-dialogs.js` (rows 2 and 3)

Old:

```text
        // Typing in the run time takes a shown error back until the next Apply.
```

New:

```text
        ED.kit.limit(modal.body.querySelector('[data-ed-set="name"]'), { max: 120, counter: false, t }); ED.kit.limit(modal.body.querySelector('[data-ed-set="description"]'), { max: 2000, counter: true, t });
        // Typing in the run time takes a shown error back until the next Apply.
```

**Edit** `ui/js/desktop/apps/easydrag-blocks.js` (row 5)

Old:

```text
            } });
            return dialog;
```

New:

```text
            } });
            ED.kit.limit(dialog.body.querySelector('[data-ed-block-name]'), { max: 120, counter: false, t });
            return dialog;
```

**Edit** `ui/js/desktop/apps/easydrag-credentials-move.js` (row 6)

Old:

```text
            dialog.body.querySelector('[data-ed-credential-name]').value = proposeName(node, finding);
```

New:

```text
            const nameField = dialog.body.querySelector('[data-ed-credential-name]');
            nameField.value = proposeName(node, finding);
            ED.kit.limit(nameField, { max: 40, counter: false, t });
```

**Edit** `ui/js/desktop/apps/easydrag-block-fields.js` (the always-visible rule hint goes)

Old:

```text
<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_fields_hint')) + '</p><div data-ed-block-fields>
```

New:

```text
<div data-ed-block-fields>
```

**Edit** `ui/js/desktop/apps/easydrag-block-fields.js` (row 8)

Old:

```text
            el.querySelectorAll('[data-ed-block-key]').forEach(input => { input.edCommit = commit; });
```

New:

```text
            el.querySelectorAll('[data-ed-block-key]').forEach(input => { input.edCommit = commit; });
            el.querySelectorAll('[data-ed-block-key="name"]').forEach(input => ED.kit.limit(input, { max: 40, counter: false, t: ed.t }));
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js` (the unused counter serial goes)

Old:

```text
    const GOAL_MAX = 4000;
    let goalCounters = 0;
```

New:

```text
    const GOAL_MAX = 4000;
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js` (the one way to measure a goal)

Old:

```text
    // goalCounter counts the goal the way the server does
```

New:

```text
    // goalLength is the length of a goal the way the server counts it: trimmed, in code points.
    function goalLength(value) { return Array.from(String(value || '').trim()).length; }

    // goalCounter counts the goal the way the server does
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js`

Old:

```text
        const count = Array.from(String(value || '').trim()).length;
```

New:

```text
        const count = goalLength(value);
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js` (row 9: the dialog markup has no counter paragraph any more)

Old:

```text
            const counterId = 'ed-goal-counter-' + (++goalCounters);
            // syncGoal is set once the dialog exists;
```

New:

```text
            // syncGoal is set once the dialog exists;
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js`

Old:

```text
<textarea class="ed-input" rows="5" aria-describedby="' + counterId + '" data-ed-authoring-goal></textarea></label><p class="ed-goal-counter" id="' + counterId + '" data-ed-goal-counter></p><p class="ed-error"
```

New:

```text
<textarea class="ed-input" rows="5" data-ed-authoring-goal></textarea></label><p class="ed-error"
```

**Edit** `ui/js/desktop/apps/easydrag-authoring.js` (the limit note replaces the counter; Generate stays off above the limit)

Old:

```text
            const counter = owned.body.querySelector('[data-ed-goal-counter]');
            const locale = (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || undefined;
            // Above the limit the counter marks a state (warn ink) and Generate is disabled; the
            // length check in onAction stays as a backstop.
            syncGoal = () => {
                const state = goalCounter(goalInput.value, t, locale);
                counter.textContent = state.text;
                if (state.over) counter.classList.add('is-over'); else counter.classList.remove('is-over');
                owned.el.querySelector('[data-ed-action="generate"]').disabled = state.over || busy;
            };
```

New:

```text
            // The limit note shows the count from 90 % of the limit and turns to warn ink above it; Generate
            // is disabled above it too, and the length check in onAction stays as a backstop.
            ED.kit.limit(goalInput, { max: GOAL_MAX, counter: true, count: goalLength, t });
            syncGoal = () => {
                owned.el.querySelector('[data-ed-action="generate"]').disabled = goalLength(goalInput.value) > GOAL_MAX || busy;
            };
```

<!-- /edits -->

```
cd W && node C:/disposable/easydrag-w1b/apply-plan-edits.mjs plans/2026-11-15-easydrag-w1b/part-b-voice.md b7-calls
```

Expected: 19 `ok   edit` lines and `applied 19 edit(s) of group b7-calls in 8 file(s).` (`-editor.js` 1 edit, `-home.js` 1, `-detail.js` 1, `-dialogs.js` 6, `-blocks.js` 1, `-credentials-move.js` 1, `-block-fields.js` 2, `-authoring.js` 6).

Proof that no old form remains:

```
cd W && git grep -n "block_fields_hint\|flow_max_run_hint\|data-ed-goal-counter\|ed-goal-counter\|hintId" -- ui/js scripts ui/*.go
```

Expected: no output.

Check the line rule and the line endings:

```
cd W && wc -l ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-dialogs.js ui/js/desktop/apps/easydrag-detail.js ui/js/desktop/apps/easydrag-canvas.js
cd W && node -e "const s=require('fs').readFileSync('ui/css/desktop-app-easydrag.css','utf8');console.log((s.match(/\r\n/g)||[]).length,(s.match(/\n/g)||[]).length)"
```

Expected: `-editor.js` 896, `-home.js` 889, `-dialogs.js` 309, `-detail.js` as after B.6, `-canvas.js` 928 (no edit); the two CSS counts are equal. If `-dialogs.js` is 310, the `hintId` removal did not apply: re-check the first dialog edit.

- [ ] **Step 5: Run the Node checks and see them pass**

```
cd W && node scripts/test-easydrag.mjs
cd W && node scripts/test-easydrag-render-baseline.mjs
```

Expected: `all EasyDrag checks passed` and `PASS render baseline (unchanged)`. If a fake-DOM script fails with `ED.kit is undefined` or `parentNode`/`insertBefore`, the harness of that script does not load `easydrag-kit.js` or its element lacks a parent: add `'easydrag-kit.js'` and `'easydrag-format.js'` to the module list of that script (the lists are at the top of `test-easydrag-extra.mjs`, `-extra3.mjs` and `test-easydrag.mjs`), or, when the element is a stand-in without a parent, give the test the same one-line stub as the authoring test: `ED.kit = { limit: () => ({ update() {}, detach() {} }) };`. Do not guard the helper in production code for a test double. (In the dry run of this plan on main 065b6fca8 every Node script passed without such a change.)

- [ ] **Step 6: Browser checks**

```
cd W && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1b-shots GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestDesktopEasyDragGoalCounterBrowser|TestDesktopEasyDragBrowser|TestEasyDragTargets.*)$' -count=1 -timeout 20m -v
```

Run with the Bash sandbox disabled. Expected: `--- PASS: TestDesktopEasyDragGoalCounterBrowser` (the changed selectors: the note is linked, turns `is-over` at 4,001 code points with Generate disabled and is plain at 4,000) and `--- PASS: TestDesktopEasyDragBrowser`; the targets test (if the pattern matches one) passes. Look at the screenshots in `D:/tmp/easydrag-w1b-shots/` for the settings dialog and the header: no limit note is visible in them (nothing is near its limit), and the block editor's fields show no hint line. One manual look at the cap note, with a throw-away line in a scratch test or the page console of a scratch instance:

```
(()=>{const i=document.querySelector('[data-ed-name]');i.value='x'.repeat(120);i.dispatchEvent(new Event('input',{bubbles:true}));return document.querySelector('.ed-title .ed-limit').textContent})()
```

Expected `Höchstens 120 Zeichen.` under the title, readable on the canvas colour (warn ink), without moving the header.

- [ ] **Step 7: Gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd W && gofmt -l ui/desktop_easydrag_test_mode_browser_test.go
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short
cd W && npm run test:easydrag:t0
```

Expected: vet silent, gofmt prints nothing, `ok  aurago/ui`, T0 without a FAIL line (this runs the two voice lints, the key test and the catalog test with the final locale files).

Message file `C:/disposable/easydrag-w1b/b7-msg.txt`:

```
feat(easydrag): length notes for capped inputs, rule hints become errors

New ED.kit.limit(input, {max, counter, count, t, after}): a polite live
region after the input shows "1.812 / 2.000" from 90 % of the limit
(multi-line text) or "Höchstens 120 Zeichen." at the cap (one line), and
hides again below. Applied to the flow name (header and settings), the flow
description, the home search, the block name, the secret name, the block
field name, the step label (80 code points) and the authoring goal (which
keeps its warn colour and the disabled Generate button above 4,000). The
always-visible hints block_fields_hint and flow_max_run_hint are gone: the
errors block_fields_invalid and flow_max_run_invalid carry the rules.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd W && git add ui/js/desktop/apps/easydrag-kit.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-home.js ui/js/desktop/apps/easydrag-detail.js ui/js/desktop/apps/easydrag-dialogs.js ui/js/desktop/apps/easydrag-blocks.js ui/js/desktop/apps/easydrag-credentials-move.js ui/js/desktop/apps/easydrag-block-fields.js ui/js/desktop/apps/easydrag-authoring.js ui/css/desktop-app-easydrag.css ui/lang/easydrag scripts/test-easydrag-kit.mjs scripts/test-easydrag-extra6.mjs scripts/test-easydrag-authoring.mjs ui/desktop_easydrag_test_mode_browser_test.go && git commit -F C:/disposable/easydrag-w1b/b7-msg.txt
```

---

## Coverage of spec §4 (Part B)

| Spec item | Task |
|---|---|
| §4.1 the guide `documentation/easydrag/voice.md` with the seven rules, en and de examples | B.1 (Step 5) |
| §4.1 the concept ledger table and the open items (§4.4) | B.1 (Step 5) |
| §4.2 the 29 German keys with a capital "Du/Dich/Dir/Dein..." converted | B.1 (Step 3, table of old and new texts) |
| §4.2 the lint `TestEasyDragGermanUsesLowercaseDu` | B.1 (Steps 1 to 4) |
| §4.3 the 121 `error_*` keys, 8 `durable_reason_*`, 10 `approval_blocked_*`, `load_failed` (about 140 texts, en and de first, 14 locales translated) | B.3: see `part-b-errors-1.md`; B.4: see `part-b-errors-2.md`; B.5: see `part-b-errors-3.md` |
| §4.4 the rename: palette and step-type texts "Baustein" to "Schritt" (de), "building block" to "step" (en), `pack_blocks` "Lokale Bausteine", the 14 other locales translated again, every renamed key listed | B.2 (Step 1 table, Step 4) |
| §4.4 the ledger lint `TestEasyDragLocalesUseLedgerWords` and the fix of the real hits (de 3 keys plus 3 compounds, en 13 keys) | B.2 (Steps 2 to 4) |
| §4.5 `publish_done`, `active_on`/`active_off`, `versions_restored` (2 sites), `repair_mission_done`, `extraction_success`, `authoring_applied` removed, each with an announcement | B.6 (Step 5) |
| §4.5 `run_started`: the card's last-run line shows "Läuft" with the `running` disc until the next list load | B.6 (Step 5, `runningNow`) |
| §4.5 `copied_json`: "Kopiert" on the button for 2 s, no motion | B.6 (Steps 4, 5; `ED.kit.flash`) |
| §4.5 `secret_deleted` | B.6: kept on purpose, see "Deviations" |
| §4.5 failures, partial results and skips keep their notification; the aria-live echoes stay | B.6 (nothing else is touched; `publish_partial`, `run_now_skipped` and the error toasts are named in the task) |
| §4.6 `ED.kit.limit` (counter from 90 %, cap message, polite live region, hides again) | B.7 (Steps 2, 3) |
| §4.6 every capped text input (8 `maxlength` inputs and the goal textarea; step label counts code points) | B.7 (table and Step 4) |
| §4.6 the authoring goal keeps its warning colour and the disabled Generate above the limit | B.7 (Step 4, `authoring.js`) |
| §4.6 `block_fields_hint` and `flow_max_run_hint` become the error text | B.7 (Step 4; the keys are deleted) |
| §4.6 collapsed help (`group_limit`, `transform_limit`, `block_sample_limit`, `batch_limit_hint`) and `preview_text` stay | B.7 (not touched) |
| §4.7 T0 Go lints | B.1, B.2 |
| §4.7 T0 node test of `kit.limit` (hidden at 50 %, counter at 90 %, cap at 100 %, hidden below) | B.7 (Step 2, `test-easydrag-kit.mjs`) |
| §4.7 browser assertion in an existing journey (publish and active send no notification and do announce; "JSON kopieren" shows "Kopiert") | B.6 (Step 7, `TestDesktopEasyDragBrowser`) |
| §4.7 the catalog tests keep the 16 locales complete | B.1, B.2, B.6, B.7 (each runs them) |
| §8 "No capital Du", "the palette says Schritte", "the nine notifications", "counters appear from 90 %" | B.1, B.2, B.6, B.7 |
| §8 "the about 140 error texts follow rules 1, 2 and 7" | B.3 to B.5 |

## Deviations from the spec

1. **`secret_deleted` stays (8 notifications change, not 9).** The call at `easydrag-fields.js:578` is shown only when flows still use the deleted secret, has `type: 'warning'` and its message is "Still used by {flows}. Flows that use it fail until the secret is set again." That is a warning about a consequence the list does not show, so it is a partial result in the sense of rule 6 ("failures, partial results and skips keep their notification"). The existing Node check (`test-easydrag-extra4.mjs:492`) keeps passing unchanged.
2. **The goal limit is 4,000, not 2,000.** The spec's list says "the description goal (authoring, 2000)". The code (`GOAL_MAX = 4000`, `easydrag-authoring.js`) and the server (`flows_ai_proposal.go`) limit the goal to 4,000 code points; 2,000 is the flow description (`dialogs.js:220`). B.7 uses 4,000 for the goal and 2,000 for the description.
3. **Two options on `ED.kit.limit` beyond the brief's contract:** `t` (the caller's translator; the kit has none) and `after` (the element the note follows). By default the note goes after the `label` around the input, not inside it, so it does not become part of the label's accessible name.
4. **One more kit helper, `ED.kit.flash(button, text, ms)`**, for the 2-second "Kopiert" on the copy button (no motion, nothing fades). The spec asks for the behaviour, not for a helper; a helper keeps `easydrag-detail.js` to existing lines and gets a Node test.
5. **The lints are a little stricter than the spec:** the de "Du" lint knows the endings (Deine, Deinem, Deinen, Deiner, Deines) and matches whole words; the ledger lint also bans plurals (Workflows, Nodes, Blocks, Blöcken, Pakete, Revisionen, workflows, nodes, building blocks, ...), the German compounds Blockversion, Blockkopie, Blockinhalt, Blockeingabe, Blockausgabe, and keeps the 12 step texts free of "Baustein" and "block" so that B.3 to B.5 cannot bring the confusion back.
6. **The rename touches 20 keys, more than the 11 "building block" texts of the spec.** `canvas_role` ("node editor") and `authoring_notice` ("node catalog") carry the banned word "node"; the German pack texts say "Blöcke", "Blockversionen", "Blockkopien" and "Blockinhalte" (block sense, only the German word changes); `panel_resize_palette` (de only) and `rail_palette` (it, pt, ja) are the same palette under the old word. Without them the ledger lint cannot pass.
7. **`flow_max_run_hint` loses its second sentence** ("Leave both fields empty for the default of 30 minutes."): the fields show the default value anyway. The rule itself moves into the existing error `flow_max_run_invalid`, shown when Apply finds a run time that breaks it (the same moment as today).
8. **`goalCounter().text` and the key `goal_counter` stay**, although the dialog no longer shows that text (the limit note shows `limit_counter`). `goalCounter()` and its Node test stay because they are the one place that tests the code-point count. A later clean-up can remove them. The now unused CSS rules `.ed-goal-counter` are replaced by `.ed-limit`.
9. **One browser test beyond the spec's one assertion:** `TestDesktopEasyDragHomeRunNowBrowser` checks the new "Läuft" line and the announcement on the home card, because that behaviour is new and no existing test reaches it. It is a separate test because a started run would break the run-row count of the journey.
10. **Plan mechanics, not a spec change:** the JS and CSS files are CRLF in a Windows checkout and most edits are multi-line, so the edits of B.6 and B.7 are applied by `apply-plan-edits.mjs` from the edit groups of this file (exact old text, exact new text, one match each) instead of by hand.

## Decisions needed

1. **The manuals still use the old words.** `documentation/manuals/de/24-easydrag.md` calls steps "Bausteine" (line 3, "ein Flow ... besteht aus Bausteinen") and uses a capital "Deine/Deinem" (for example line 82), and the English manual says "building blocks" for steps. The lints do not read the manuals and the spec does not list them. Recommendation: one small docs task after Part B (docs-agent) that applies the same ledger and "du" to both manuals; the test `b5 U18 manual chapter 24 ...` in `scripts/test-easydrag.mjs` must keep passing.
2. **`secret_deleted` (Deviation 1).** Recommendation: keep the warning. If the owner wants the spec's literal reading, the call goes and the warning text moves into the secret list as a callout; that is a separate design.
3. **A live region that speaks on every keystroke from 90 %.** `ED.kit.limit` updates its text on every `input` event, so a screen reader reads "1.801 / 2.000", "1.802 / 2.000", ... while the user types in the last 10 %. Recommendation: ship as specified; if testers find it noisy, debounce the note by 400 ms in `update` (one line in the kit).
4. **Native-speaker review** of the translations in the 14 other locales (`b-rename.json`, `b-notify.json`, `b-limit.json`; the error texts in B.3 to B.5), as for W1a. The words for "step" follow the dominant word of each locale file (pt "etapas", it "passaggi", ja "ステップ").
5. **`rail_palette` in it, pt and ja changes with this part** (Deviation 6). Recommendation: keep; the alternative is to leave the rail label with the old word and let the palette title differ from it.
