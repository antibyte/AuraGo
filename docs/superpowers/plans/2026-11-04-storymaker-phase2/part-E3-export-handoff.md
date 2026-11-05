# Part E3: Export dialog and Video Studio handoff

Batch 2D. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-E3-1 -->

### Task E3a: Export dialog module
**Batch:** 2D  **Depends on:** E2a
**Files:**
- Create: `ui/js/desktop/apps/storymaker-dialog.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/css/desktop-app-storymaker.css`
- Test: `ui/desktop_storymaker_registration_test.go`
- Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Write the failing test**

In `ui/desktop_storymaker_registration_test.go`, as E2b left it, replace this line.

Old:
```go
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export", "audio", "voice", "music"}
```

New:
```go
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export", "audio", "voice", "music", "dialog"}
```

Append this to the end of `ui/desktop_storymaker_model_test.go`. The file's imports already cover it.

```go
// runStorymakerExportScript runs a Node script on top of storymakerExportFakeDOM and returns its output.
func runStorymakerExportScript(t *testing.T, name, script string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, []byte(storymakerExportFakeDOM+script), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, file, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	return string(out)
}

// storymakerExportFakeDOM starts every Node script that drives the export modules. It holds just enough DOM for S.el, the modules
// and the checks. Timers are stubs, so a tracked job never polls. The modules come from the folder in argv[2].
const storymakerExportFakeDOM = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];
const walk = (node, test, out) => { if (test(node)) out.push(node); node.children.forEach(child => walk(child, test, out)); return out; };
class FakeNode {
  constructor(tag) {
    this.tagName = String(tag).toUpperCase();
    this.children = []; this.parentNode = null; this.attrs = {}; this.dataset = {}; this.listeners = {};
    this.className = ''; this.ownText = ''; this.hidden = false; this.disabled = false; this.checked = false; this.value = ''; this.open = false;
    this.style = { setProperty() {} };
  }
  setAttribute(name, value) {
    this.attrs[name] = String(value);
    if (name === 'hidden' || name === 'disabled' || name === 'checked') this[name] = true;
    if (name === 'value') this.value = String(value);
  }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter(other => other !== fn); }
  fire(type, event) { (this.listeners[type] || []).forEach(fn => fn(Object.assign({ target: this }, event || {}))); }
  click() { this.fire('click'); }
  appendChild(child) { child.parentNode = this; this.children.push(child); return child; }
  removeChild(child) { this.children.splice(this.children.indexOf(child), 1); child.parentNode = null; return child; }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
  get firstChild() { return this.children[0] || null; }
  contains(other) { return walk(this, node => node === other, []).length > 0; }
  get textContent() { return this.ownText + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this.children = []; this.ownText = String(text); }
  querySelector(selector) {
    const named = /data-stm="([^"]+)"/.exec(String(selector));
    return walk(this, node => node.tagName === 'BUTTON' && !node.disabled && (!named || node.dataset.stm === named[1]), [])[0] || null;
  }
  focus() { documentFake.activeElement = this; }
  showModal() { this.open = true; }
  close() { this.open = false; }
}
const documentFake = new FakeNode('#document');
documentFake.documentElement = { lang: 'en' };
documentFake.createElement = tag => new FakeNode(tag);
documentFake.createElementNS = (ns, tag) => new FakeNode(tag);
documentFake.createTextNode = text => { const node = new FakeNode('#text'); node.ownText = String(text); return node; };
const context = { console, document: documentFake, setTimeout: () => 0, clearTimeout: () => {} };
context.globalThis = context; context.window = context;
vm.createContext(context);
const load = names => names.forEach(name => vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name }));
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const byStm = (root, name) => walk(root, node => node.dataset.stm === name, []);
const one = (root, name) => { const list = byStm(root, name); if (list.length !== 1) throw new Error(name + ': found ' + list.length + ' nodes'); return list[0]; };
const settle = () => new Promise(resolve => setImmediate(resolve));
const fail = err => { console.error(err && err.stack || err); process.exit(1); };
`

// The export dialog: burn-in starts from style.captions, and Open in Video Studio forces the stems and remembers the new job.
func TestStorymakerExportDialogInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerExportScript(t, "storymaker-export-dialog.js", storymakerExportDialogScript)
	if !strings.Contains(out, "storymaker export dialog ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportDialogScript = `
load(['storymaker-core.js', 'storymaker-dialog.js']);
const S = context.StorymakerParts;
function makeInst(captions, job) {
  const inst = {
    t: key => key, domId: name => 'test-' + name, handoffWish: new Set(), sent: [],
    story: { style: { palette: 'p', font: 'f', captions } },
    startExport(options) { inst.sent.push(options); return Promise.resolve(job); }
  };
  return inst;
}
(async () => {
  // Burn-in starts from style.captions, and a start sends what the form shows.
  for (const captions of [true, false]) {
    const inst = makeInst(captions, { id: 'job-1' });
    const dialog = S.createExportDialog(inst);
    dialog.open();
    eq(dialog.el.open, true, 'open uses showModal');
    eq(one(dialog.el, 'export-opt-burn').checked, captions, 'burn-in preset');
    one(dialog.el, 'export-confirm').click();
    await settle();
    eq(dialog.el.open, false, 'a start closes the dialog');
    eq(inst.sent, [{ burn_in_captions: captions, srt: false, vtt: false, stems: false, quality: 'standard' }], 'options');
    eq(inst.handoffWish.size, 0, 'no wish without Open in Video Studio');
  }

  // Open in Video Studio forces the stems while it is on, and the wish holds the new job.
  const inst = makeInst(true, { id: 'job-7' });
  const dialog = S.createExportDialog(inst);
  dialog.open();
  const handoff = one(dialog.el, 'export-opt-handoff');
  const stems = one(dialog.el, 'export-opt-stems');
  const note = one(dialog.el, 'export-stems-forced');
  handoff.checked = true; handoff.fire('change');
  eq([stems.checked, stems.disabled, note.hidden], [true, true, false], 'stems forced on');
  handoff.checked = false; handoff.fire('change');
  eq([stems.checked, stems.disabled, note.hidden], [false, false, true], 'stems released');
  handoff.checked = true; handoff.fire('change');
  one(dialog.el, 'export-confirm').click();
  await settle();
  eq(inst.sent[0].stems, true, 'the body asks for stems');
  eq(Array.from(inst.handoffWish), ['job-7'], 'the wish holds the job id');

  // A start that did not happen leaves no wish.
  const refused = makeInst(true, undefined);
  const refusedDialog = S.createExportDialog(refused);
  refusedDialog.open();
  const refusedHandoff = one(refusedDialog.el, 'export-opt-handoff');
  refusedHandoff.checked = true; refusedHandoff.fire('change');
  one(refusedDialog.el, 'export-confirm').click();
  await settle();
  eq(refused.handoffWish.size, 0, 'no job, no wish');

  console.log('storymaker export dialog ok');
})().catch(fail);
`
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(StorymakerExportDialogInNode|DesktopStorymakerRegistration)' -count=1
```

Expected: FAIL, for two reasons.
- `TestStorymakerExportDialogInNode` stops on `ENOENT` for `storymaker-dialog.js`, because the module does not exist.
- `TestDesktopStorymakerRegistration` says the loader lists `/js/desktop/apps/storymaker-dialog.js'` "out of order or not at all".

- [ ] **Step 3: Implement**

Create `ui/js/desktop/apps/storymaker-dialog.js`:

```js
// Storymaker app export dialog: a native <dialog> with the options of one export: quality, captions burned into the picture, SRT,
// VTT, stems and Open in Video Studio. Burned-in captions start from style.captions. Open in Video Studio forces the stems on and
// puts the new job's id into inst.handoffWish, a session-only Set. The dialog starts nothing itself: it hands the options to
// inst.startExport, which S.createExport sets. Classic script, publishes S.createExportDialog.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};

    // The form a story starts from. A story without the field counts as captions on, the server's default.
    function presetValues(story) {
        const style = story && story.style;
        return { quality: 'standard', burnIn: !(style && style.captions === false), srt: false, vtt: false, stems: false, handoff: false };
    }

    // The POST /render body for the form values.
    function renderOptions(values) {
        return {
            burn_in_captions: values.burnIn,
            srt: values.srt,
            vtt: values.vtt,
            stems: values.stems || values.handoff,
            quality: values.quality === 'high' ? 'high' : 'standard'
        };
    }

    function createExportDialog(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const node = el('dialog', { class: 'stm-export-dialog', ariaLabel: t('storymaker.export.dialog_title'), dataset: { stm: 'export-dialog' } });
        let disposed = false;

        function checkbox(name, labelKey, checked, onChange) {
            const input = el('input', { type: 'checkbox', dataset: { stm: 'export-opt-' + name }, checked: checked || null, onChange: onChange || null });
            return { input, row: el('label', { class: 'stm-export-check' }, input, el('span', { text: t(labelKey) })) };
        }

        function close() {
            if (node.open) node.close();
        }

        async function submit(values) {
            close();
            const job = await inst.startExport(renderOptions(values));
            if (values.handoff && job && job.id) inst.handoffWish.add(job.id);
        }

        function build(values) {
            const quality = el('select', { class: 'stm-select', id: inst.domId('export-quality'), dataset: { stm: 'export-opt-quality' } },
                el('option', { value: 'standard', selected: values.quality === 'standard' || null, text: t('storymaker.export.quality_standard') }),
                el('option', { value: 'high', selected: values.quality === 'high' || null, text: t('storymaker.export.quality_high') }));
            const burnIn = checkbox('burn', 'storymaker.export.burn_in', values.burnIn);
            const srt = checkbox('srt', 'storymaker.export.label_srt', values.srt);
            const vtt = checkbox('vtt', 'storymaker.export.label_vtt', values.vtt);
            const stems = checkbox('stems', 'storymaker.export.stems', values.stems);
            const note = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'export-stems-forced' }, text: t('storymaker.export.stems_forced') });
            let stemsWanted = values.stems;
            const handoff = checkbox('handoff', 'storymaker.export.open_in_studio', values.handoff, () => {
                const forced = handoff.input.checked;
                if (forced && !stems.input.disabled) stemsWanted = stems.input.checked;
                stems.input.checked = forced || stemsWanted;
                stems.input.disabled = forced;
                note.hidden = !forced;
            });
            const read = () => ({ quality: quality.value, burnIn: burnIn.input.checked, srt: srt.input.checked, vtt: vtt.input.checked, stems: stems.input.checked, handoff: handoff.input.checked });
            S.clear(node);
            node.appendChild(el('h3', { text: t('storymaker.export.dialog_title') }));
            node.appendChild(el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: inst.domId('export-quality'), text: t('storymaker.export.quality') }), quality));
            node.appendChild(burnIn.row);
            node.appendChild(el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.export.subtitles') }), srt.row, vtt.row));
            node.appendChild(stems.row);
            node.appendChild(handoff.row);
            node.appendChild(note);
            node.appendChild(el('div', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-dialog-cancel' }, onClick: () => close() }, el('span', { text: t('storymaker.export.dialog_cancel') })),
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-confirm' }, onClick: () => submit(read()) }, icon('film'), el('span', { text: t('storymaker.export.start') }))));
        }

        return {
            el: node,
            open() {
                if (disposed || node.open) return;
                build(presetValues(inst.story));
                node.showModal();
            },
            close,
            dispose() {
                disposed = true;
                close();
                node.remove();
            }
        };
    }

    S.createExportDialog = createExportDialog;
})();
```

In `ui/js/desktop/core/module-loader.js`, as E2b left it, replace this text inside the `storymaker` entry.

Old:
```js
'/js/desktop/apps/storymaker-music.js', '/js/desktop/apps/storymaker.js'
```

New:
```js
'/js/desktop/apps/storymaker-music.js', '/js/desktop/apps/storymaker-dialog.js', '/js/desktop/apps/storymaker.js'
```

In `ui/css/desktop-app-storymaker.css`, replace this comment line.

Old:
```css
/* The code tab has the whole body: the strip and the inspector belong to the storyboard */
```

New:
```css
/* Export dialog */
.stm-export-dialog { width: min(440px, calc(100vw - 32px)); max-height: min(80vh, 640px); padding: var(--ds-space-4); overflow: auto; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, 0 12px 32px color-mix(in srgb, var(--vd-text) 22%, transparent)); color: var(--vd-text); }
.stm-export-dialog::backdrop { background: color-mix(in srgb, var(--vd-text) 28%, transparent); }
.stm-export-dialog h3 { margin: 0 0 var(--ds-space-3); font-size: 15px; }
.stm-export-dialog .stm-actions { justify-content: flex-end; margin-top: var(--ds-space-3); }
.stm-export-check { display: flex; align-items: center; gap: var(--ds-space-2); margin: 0 0 var(--ds-space-2); cursor: pointer; }
.stm-export-check input { flex: 0 0 auto; margin: 0; }

/* The code tab has the whole body: the strip and the inspector belong to the storyboard */
```

- [ ] **Step 4: Run**

```powershell
go test ./ui -run '(?i)(StorymakerExportDialogInNode|DesktopStorymakerRegistration|StorymakerLoaderListsEveryScript|StorymakerSourcesAreSafeAndSmall|StorymakerStylesheetUsesOnlyTokens|StorymakerAppLoadsNothingExternal)' -count=1
```

Expected: `ok aurago/ui`. The module-shape guards accept the new file. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the new `storymaker.export.*` keys until E3g, which outline section 3 allows.

- [ ] **Step 5: Commit**

Run detect_changes per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

A PARTIAL answer on the new file is accepted with a grep confirmation in the commit body, as an extra `-m` before the trailer.

```powershell
git add ui/js/desktop/apps/storymaker-dialog.js ui/js/desktop/core/module-loader.js ui/css/desktop-app-storymaker.css ui/desktop_storymaker_registration_test.go ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): export options dialog module" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3b: Export start through the dialog, new blockers
**Batch:** 2D  **Depends on:** E3a
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-export.js`
- Modify: `ui/js/desktop/apps/storymaker.js`
- Test: `ui/desktop_storymaker_fixture_test.go`
- Test: `ui/desktop_storymaker_model_test.go`
- Test: `ui/desktop_storymaker_browser_test.go` (one added click, see the issues block)

- [ ] **Step 1: Impact**

Run this for each of `createExport`, `createInstance`, `storymakerFixture`, `TestStorymakerAppBrowser` and `storymakerExportRefusalsScript`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExport" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_model_test.go`, replace the import line.

Old:
```go
	"testing"
```

New:
```go
	"testing"

	"aurago/internal/storymaker/render"
```

Append this to the end of the same file. `runStorymakerExportScript` and `storymakerExportFakeDOM` are already in this file, from the dialog task.

```go
// Missing narration or music blocks the start with its reason and a way to the panel. The start button opens the dialog, and
// startRender receives the chosen options, in a shape the server's strict parser accepts. A 422 shows its reason in the panel, and
// the focus goes on to the next button after a start.
func TestStorymakerExportStartInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerExportScript(t, "storymaker-export-start.js", storymakerExportStartScript)
	if !strings.Contains(out, "storymaker export start ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	_, rest, found := strings.Cut(out, "options: ")
	if !found {
		t.Fatalf("the script printed no options:\n%s", out)
	}
	line, _, _ := strings.Cut(rest, "\n")
	options, err := render.ParseRenderOptions([]byte(strings.TrimSpace(line)))
	if err != nil {
		t.Fatalf("the server refuses the dialog's options %s: %v", line, err)
	}
	if options.BurnInCaptions == nil || *options.BurnInCaptions || !options.SRT || options.VTT || options.Stems || options.Quality != "high" {
		t.Fatalf("parsed options = %+v", options)
	}
}

const storymakerExportStartScript = `
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
const renders = [];
const job = { id: 'job-1', project_id: 'p1', kind: 'render', status: 'queued', progress: 0 };
let refusal = null;
const inst = {
  t: key => key, domId: name => 'test-' + name, bus: S.createBus(), parts: {}, ctx: {}, readonly: false, projectId: 'p1', etag: '"e1"', issues: [], handoffWish: new Set(),
  status: { ffmpeg: { ready: true, version: '8.1' }, browser: { ready: true, source: 'system' }, limits: { max_duration_seconds: 600 } },
  story: { format: { aspect: '16:9' }, style: { captions: false }, music: null },
  report: { frame_count: 90, fps: 30, duration_ms: 3000, issues: [], audio_events: [] },
  audioInfo: { narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null },
  flushEdits: () => Promise.resolve(true), showIssues() {}, notify() {},
  shown: [], showPanel(name) { inst.shown.push(name); },
  api: { startRender(id, etag, key, options) { if (refusal) return Promise.reject(refusal); renders.push({ id, etag, key, options }); return Promise.resolve(job); } }
};
const exporter = S.createExport(inst);
const root = exporter.el;
const reasons = () => byStm(root, 'export-blockers').flatMap(list => list.children.map(item => item.dataset.reason));
const says = (key, what) => { if (!one(root, 'export-blockers').textContent.includes(key)) throw new Error(what + ': no ' + key); };
(async () => {
  exporter.open();
  eq(reasons(), [], 'nothing blocks');

  // Missing narration blocks with its reason.
  inst.audioInfo.missing = [{ scene_id: 's1', block_id: 's1:n', chars: 12 }];
  inst.bus.emit('audio');
  eq(reasons(), ['narration'], 'missing narration');
  says('storymaker.export.blocked_narration', 'narration');
  eq(one(root, 'export-start').disabled, true, 'no start while narration is missing');
  one(root, 'export-go-voice').click();
  eq(inst.shown, ['voice'], 'the blocker leads to the voice panel');
  exporter.open();
  inst.audioInfo.missing = [];

  // Music that is on and has no audio blocks with its reason. The rule is the one of the server's music_incomplete.
  inst.story.music = { mode: 'file', file: 'Music/a.mp3' };
  inst.bus.emit('audio');
  eq(reasons(), ['music'], 'music on without audio');
  says('storymaker.export.blocked_music', 'music');
  one(root, 'export-go-music').click();
  eq(inst.shown, ['voice', 'music'], 'the blocker leads to the music panel');
  exporter.open();
  inst.audioInfo.music_issue = 'source_missing';
  inst.bus.emit('audio');
  says('storymaker.export.blocked_music_source', 'moved source');
  delete inst.audioInfo.music_issue;
  inst.story.music = { mode: 'file', file: 'Music/a.mp3', volume: 0 };
  inst.bus.emit('audio');
  eq(reasons(), [], 'volume 0 plays nothing');
  inst.story.music = { mode: 'none' };
  inst.bus.emit('audio');
  eq(reasons(), [], 'mode none');
  inst.story.music = { mode: 'generated', mood: 'playful' };
  inst.audioInfo.music = { key: 'a'.repeat(64), ext: 'mp3', duration_ms: 61000 };
  inst.bus.emit('audio');
  eq(reasons(), [], 'the music exists');

  // A 422 for the chosen burn-in says why in the panel. It stays out of the issues, so another choice can still start, and the focus
  // goes back to the start button. An audio answer leaves it, a changed story ends it.
  refusal = S.errors.fromResponse(422, { error: 'invalid_story', code: 'invalid_story', message: 'No font.', issues: [{ severity: 'warning', code: 'cue_late', scene_id: 's1', path: '', message: 'Late.' }, { severity: 'error', code: 'glyph_missing', scene_id: 's2', path: '/scenes/1/narration', message: 'No font.', fix: 'Replace them.' }] }, null);
  one(root, 'export-start').click();
  one(root, 'export-confirm').click();
  await settle();
  eq(one(root, 'export-refused').dataset.code, 'invalid_story', 'the refusal is shown');
  eq(one(root, 'export-missing').textContent, 's2: No font. Replace them.', 'its reason');
  eq(inst.issues, [], 'the refusal stays out of the issues');
  eq(one(root, 'export-start').disabled, false, 'another choice can still start');
  eq(documentFake.activeElement === one(root, 'export-start'), true, 'the focus is on the start button after a refusal');
  inst.bus.emit('audio');
  one(root, 'export-refused');
  inst.bus.emit('story');
  eq(byStm(root, 'export-refused').length, 0, 'a changed story ends the refusal');
  refusal = null;

  // The start button opens the dialog and starts nothing.
  one(root, 'export-start').click();
  const dialog = one(root, 'export-dialog');
  eq(dialog.open, true, 'the dialog is open');
  eq(renders.length, 0, 'nothing starts before the dialog says so');

  // Escape belongs to the open dialog: the popover stays open underneath.
  documentFake.fire('keydown', { key: 'Escape', stopPropagation() {} });
  eq(one(root, 'export-panel').hidden, false, 'Escape leaves the popover alone');

  // startRender receives the chosen options.
  one(dialog, 'export-opt-srt').checked = true;
  one(dialog, 'export-opt-quality').value = 'high';
  one(dialog, 'export-confirm').click();
  await settle();
  eq(renders.length, 1, 'one start');
  eq([renders[0].id, renders[0].etag], ['p1', '"e1"'], 'project and ETag');
  if (!renders[0].key) throw new Error('no idempotency key');
  eq(renders[0].options, { burn_in_captions: false, srt: true, vtt: false, stems: false, quality: 'high' }, 'options');
  eq(exporter.job().id, 'job-1', 'the job is tracked');
  one(root, 'export-progress');

  // The focus moves on from the closed dialog to Cancel, and the repaint of a job event keeps it there.
  eq(documentFake.activeElement === one(root, 'export-cancel'), true, 'the focus is on Cancel after a start');
  inst.bus.emit('job', Object.assign({}, job, { status: 'running', progress: 0.5 }));
  eq(documentFake.activeElement === one(root, 'export-cancel'), true, 'a repaint keeps the focus on Cancel');
  exporter.dispose();
  console.log('options: ' + JSON.stringify(renders[0].options));
  console.log('storymaker export start ok');
})().catch(fail);
`
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(StorymakerExportStartInNode)' -count=1
```

Expected: FAIL with `missing narration: got [], want ["narration"]`, because `blockers()` has no audio reasons yet.

- [ ] **Step 4: Implement**

E1f and E2d changed `ui/js/desktop/apps/storymaker-export.js` earlier. The old texts below are single lines outside the changes their rows name, so they read as in phase 1. Working-tree files may be CRLF, so apply each with the Edit tool.

Edit 1, the music rule. Old:
```js
    function dirOf(path) {
```
New:
```js
    // The rule of the server's music_incomplete: music is on when its mode is not none and its volume is above 0 (absent means 0.25).
    function musicOn(story) {
        const music = story && story.music;
        if (!music || !music.mode || music.mode === 'none') return false;
        return music.volume === undefined || music.volume === null || Number(music.volume) > 0;
    }

    function dirOf(path) {
```

Edit 2, the dialog lives inside the export root. Old:
```js
        const root = el('div', { class: 'stm-export' }, button, panel);
```
New:
```js
        const root = el('div', { class: 'stm-export' }, button, panel);

        // Created on first use. It lives inside root, so the popover's outside-click rule leaves it alone.
        let dialog = null;
        function optionsDialog() {
            if (!dialog) {
                dialog = S.createExportDialog(inst);
                root.appendChild(dialog.el);
            }
            return dialog;
        }
```

Edit 3, the blockers. Old:
```js
            if (!inst.report || !(inst.report.frame_count > 0)) list.push('not_ready');
```
New:
```js
            const audio = inst.audioInfo;
            if (audio && Array.isArray(audio.missing) && audio.missing.length > 0) list.push('narration');
            if (audio && !audio.music && musicOn(inst.story)) list.push('music');
            if (!inst.report || !(inst.report.frame_count > 0)) list.push('not_ready');
```

Edit 4, the reasons. Old:
```js
                case 'too_long': return t('storymaker.export.blocked_too_long', { max: S.format.seconds(max) });
```
New:
```js
                case 'too_long': return t('storymaker.export.blocked_too_long', { max: S.format.seconds(max) });
                case 'narration': return t('storymaker.export.blocked_narration');
                case 'music': return inst.audioInfo && inst.audioInfo.music_issue === 'source_missing' ? t('storymaker.export.blocked_music_source') : t('storymaker.export.blocked_music');
```

Edit 5, the start button opens the dialog. Old:
```js
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-start' }, disabled: blocked || starting || null, onClick: () => start() }, icon('film'), el('span', { text: starting ? t('storymaker.export.starting') : t('storymaker.export.start') }))));
```
New:
```js
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-start' }, ariaHaspopup: 'dialog', disabled: blocked || starting || null, onClick: () => optionsDialog().open() }, icon('film'), el('span', { text: starting ? t('storymaker.export.starting') : t('storymaker.export.open_options') }))));
```

Edit 6, `start` takes the options. Old:
```js
        async function start() {
```
New:
```js
        async function start(options) {
```

Edit 7. Old:
```js
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid());
```
New:
```js
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid(), options);
```

Edit 8. Old:
```js
                apply(created, true);
```
New:
```js
                apply(created, true);
                return created;
```

Edit 9, Escape belongs to an open dialog. Old:
```js
            if (open && event.key === 'Escape') { event.stopPropagation(); close(true); }
```
New:
```js
            if (open && event.key === 'Escape' && !(dialog && dialog.el.open)) { event.stopPropagation(); close(true); }
```

Edit 10, the way in for the dialog. Old:
```js
        const off = [
```
New:
```js
        // The dialog starts through this. It answers the new job, or nothing when the start did not happen. The repaints of the start
        // removed the button that the closed dialog gave the focus back to, so the focus moves on to what comes next.
        inst.startExport = async options => {
            const created = await start(options);
            if (open) focusAction('');
            return created;
        };

        const off = [
```

E2d's `audio` listener in the same list already repaints on a fresh audio answer, so the blockers need no listener of their own. Edit 19 narrows what that listener clears and adds a `story` listener for the story refusal.

Edit 11. Old:
```js
                off.forEach(fn => fn());
```
New:
```js
                off.forEach(fn => fn());
                if (dialog) dialog.dispose();
```

Edit 12, the narration and music blockers lead to their panels, beside the way from the errors blocker to the issues. Old:
```js
                if (reasons.indexOf('errors') >= 0) view.appendChild(el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-issues' }, onClick: () => { close(); inst.showIssues(); } }, icon('alert'), el('span', { text: t('storymaker.issues.show') })));
```
New:
```js
                if (reasons.indexOf('errors') >= 0) view.appendChild(el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-issues' }, onClick: () => { close(); inst.showIssues(); } }, icon('alert'), el('span', { text: t('storymaker.issues.show') })));
                if (reasons.indexOf('narration') >= 0) view.appendChild(goVoiceButton());
                if (reasons.indexOf('music') >= 0) view.appendChild(goMusicButton());
```

Edits 13 to 19 quote lines that E2d wrote, so they read as E2d left them.

Edit 13, one builder for each way to a panel, shared by the blockers and the refusals. Old:
```js
        function goPanel(name) {
```
New:
```js
        function goVoiceButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-voice' }, onClick: () => goPanel('voice') }, icon('user'), el('span', { text: t('storymaker.export.go_voice') }));
        }

        function goMusicButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-music' }, onClick: () => goPanel('music') }, icon('sparkle'), el('span', { text: t('storymaker.export.go_music') }));
        }

        function goPanel(name) {
```

Edit 14. Old:
```js
                action = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-voice' }, onClick: () => goPanel('voice') }, icon('user'), el('span', { text: t('storymaker.export.go_voice') }));
```
New:
```js
                action = goVoiceButton();
```

Edit 15. Old:
```js
                action = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-music' }, onClick: () => goPanel('music') }, icon('sparkle'), el('span', { text: t('storymaker.export.go_music') }));
```
New:
```js
                action = goMusicButton();
```

Edit 16, the reason of a 422. Old:
```js
        // A refused start says what is missing and offers the panel that fixes it.
```
New:
```js
        // A 422 names a problem of the story as this start would draw it, which the issue list need not hold: burned-in captions chosen
        // in the dialog also check narration that the saved story does not draw. It stays out of inst.issues, so another choice can still
        // start, and the panel shows the first error. Server texts are shown as text only.
        function storyRefusedView(data) {
            const issues = Array.isArray(data.issues) ? data.issues : [];
            const first = issues.find(item => item && item.severity === 'error') || {};
            const reason = [first.scene_id ? first.scene_id + ':' : '', first.message || data.message, first.fix].filter(Boolean).join(' ');
            return el('div', { class: 'stm-job', dataset: { stm: 'export-refused', code: refused.code } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: S.errors.describeJob(t, { code: refused.code }) })),
                el('p', { class: 'stm-hint', dataset: { stm: 'export-missing' }, text: reason.slice(0, 600) }));
        }

        // A refused start says what is missing and offers the panel that fixes it.
```

Edit 17. Old:
```js
            const data = refused.data && typeof refused.data === 'object' ? refused.data : {};
```
New:
```js
            const data = refused.data && typeof refused.data === 'object' ? refused.data : {};
            if (refused.story) return storyRefusedView(data);
```

Edit 18. Old:
```js
                if (err && (err.code === 'narration_incomplete' || err.code === 'music_incomplete')) refused = { code: err.code, data: err.data || {} };
```
New:
```js
                if (err && (err.code === 'narration_incomplete' || err.code === 'music_incomplete')) refused = { code: err.code, data: err.data || {} };
                if (err && err.status === 422) refused = { code: err.code, data: err.data || {}, story: true };
```

Edit 19. Old:
```js
            inst.bus.on('audio', () => { refused = null; if (open) paint(); }),
```
New:
```js
            // An audio answer ends an audio refusal and a changed story ends a story refusal. The audio answer of a save can land after
            // the 422 of the same start, and that must not wipe the reason.
            inst.bus.on('audio', () => { if (refused && !refused.story) refused = null; if (open) paint(); }),
            inst.bus.on('story', () => { if (refused && refused.story) { refused = null; if (open) paint(); } }),
```

Edits 20 to 22 keep the focus in the panel. `paint` replaces every button of the panel and a job event repaints, so one focus call after a start would be lost with the next job event.

Edit 20. Old:
```js
            S.clear(panel);
```
New:
```js
            // Every repaint replaces the panel's buttons, so the focus moves on to the new button of the same name.
            const active = document.activeElement;
            const focused = active && panel.contains(active) ? active.dataset.stm : '';
            S.clear(panel);
```

Edit 21. Old:
```js
            else panel.appendChild(startView());
```
New:
```js
            else panel.appendChild(startView());
            // While a start runs its button is off. The end of the start moves the focus, see inst.startExport.
            if (focused && !starting) focusAction(focused);
```

Edit 22. Old:
```js
        function toggle() { if (open) close(true); else openPanel(); }
```
New:
```js
        // Focuses the named button of the panel, else Cancel, Start or the first button that can take the focus.
        function focusAction(name) {
            const pick = stm => panel.querySelector('[data-stm="' + stm + '"]:not([disabled])');
            const next = (name && pick(name)) || pick('export-cancel') || pick('export-start') || panel.querySelector('button:not([disabled])');
            if (next) next.focus();
        }

        function toggle() { if (open) close(true); else openPanel(); }
```

In `ui/js/desktop/apps/storymaker.js`, inside `createInstance`, replace this line. Old:
```js
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0
```
New:
```js
            handoffWish: new Set(),
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0
```

In `ui/desktop_storymaker_fixture_test.go`, send the render body through the real parser, so the stand-in tour proves the wire. E1d and E2b added routes to this fixture, and these lines stay as quoted.

Old:
```go
	"aurago/internal/storymaker"
```
New:
```go
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
```

Old:
```go
		f.render(w, r, id)
```
New:
```go
		f.render(w, r, id, raw)
```

Old:
```go
func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string) {
```
New:
```go
func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string, raw []byte) {
	if _, err := render.ParseRenderOptions(raw); err != nil {
		f.fail(w, http.StatusBadRequest, "invalid_request", err.Error(), nil)
		return
	}
```

The fixture's `narration()` answers through the real `AudioAnswer`, and the stand-in server has no narrate route, so narration text there can never get audio. Edit 3 above turns that into the `narration` blocker, and the stand-in tour would wait forever for an enabled `export-start`. So the fixture reports nothing as missing, right after the `AudioAnswer` error check that E1d wrote. `TestStorymakerExportStartInNode` covers the blocker, and I2 walks it against the real server.

Old:
```go
	w.Header().Set("Cache-Control", "no-store")
	f.writeJSON(w, http.StatusOK, answer)
```
New:
```go
	// This stand-in has no narrate route, so its narration can never get audio. Without this, the export's narration blocker
	// would keep export-start disabled and stop the tour.
	answer.Missing = []storymaker.MissingBlock{}
	answer.Chars.Missing = 0
	w.Header().Set("Cache-Control", "no-store")
	f.writeJSON(w, http.StatusOK, answer)
```

In `ui/desktop_storymaker_browser_test.go`, the stand-in tour takes the extra step. Old:
```go
	storymakerClick(t, page, `[data-stm="export-start"]`)
```
New:
```go
	storymakerClick(t, page, `[data-stm="export-start"]`)
	storymakerClick(t, page, `[data-stm="export-confirm"]`)
```

In `ui/desktop_storymaker_model_test.go`, E2d's refusal test clicks the start button and expects `startRender`, so it now confirms in the dialog. Its script loads the dialog module, its fake DOM gets the dialog methods and its `makeInst` gets `domId`. E1f's narration status test never clicks the start button, so it stays as it is.

Old:
```js
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-export.js']) {
```
New:
```js
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-export.js']) {
```

Old:
```js
  querySelector() { return null; }
```
New:
```js
  querySelector() { return null; }
  showModal() { this.open = true; }
  close() { this.open = false; }
  remove() { if (this.parent) this.parent.removeChild(this); }
```

Old:
```js
    shown: [],
```
New:
```js
    shown: [],
    domId: name => 'test-' + name,
```

Old:
```js
  eq(find(exporter.el, 'export-refused').dataset.code, 'narration_incomplete', 'the refusal is shown');
```
New:
```js
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-refused').dataset.code, 'narration_incomplete', 'the refusal is shown');
```

Old:
```js
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music', 'a music file that is not ready');
```
New:
```js
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music', 'a music file that is not ready');
```

Old:
```js
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music_source', 'a moved source');
```
New:
```js
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music_source', 'a moved source');
```

- [ ] **Step 5: Run**

```powershell
go test ./ui -run '(?i)(StorymakerExportStartInNode|StorymakerExportDialogInNode|StorymakerExportRefusalsInNode|StorymakerSourcesAreSafeAndSmall)' -count=1
go test ./ui -run Storymaker -count=1 -timeout 20m
```

Expected:
- The first line prints `ok aurago/ui`.
- In the second line only `TestStorymakerAppUsesOnlyKeysItHas` fails, on the new `storymaker.export.*` keys, which outline section 3 allows until E3g. The browser tours skip without `AURAGO_RUN_BROWSER_SMOKE`.
- `TestStorymakerExportRefusalsInNode` passes through the dialog. `TestStorymakerNarrationStatusInNode` never clicks `export-start`, so this task does not change it.

- [ ] **Step 6: Commit**

Run detect_changes per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

```powershell
git add ui/js/desktop/apps/storymaker-export.js ui/js/desktop/apps/storymaker.js ui/desktop_storymaker_fixture_test.go ui/desktop_storymaker_model_test.go ui/desktop_storymaker_browser_test.go
git commit -m "feat(storymaker): start exports through the dialog, block on missing audio" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3c: Finished-job actions
**Batch:** 2D  **Depends on:** E3b
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-export.js`
- Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Impact**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExport" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append this to the end of `ui/desktop_storymaker_model_test.go`. `runStorymakerExportScript` and `storymakerExportFakeDOM` are already in this file.

```go
// A finished job lists what it wrote: stems, subtitles and the library copy show when present, each warning shows as text, and a
// job without extras shows only the MP4.
func TestStorymakerExportFinishedJobInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerExportScript(t, "storymaker-export-finished.js", storymakerExportFinishedScript)
	if !strings.Contains(out, "storymaker export finished ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportFinishedScript = `
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
// The handoff module joins the app after this task. Here no job offers it.
S.handoff = { canOffer: () => false, run: () => Promise.resolve({ projectId: '' }) };
function finished(output, warnings) {
  const opened = [];
  const inst = {
    t: key => key, domId: name => 'test-' + name, bus: S.createBus(), parts: {}, readonly: false, projectId: 'p1', etag: '"e1"', issues: [], handoffWish: new Set(),
    ctx: { openApp(app, target) { opened.push([app, target.path]); } },
    status: { ffmpeg: { ready: true }, browser: { ready: true, source: 'system' }, limits: {} },
    story: { format: { aspect: '16:9' }, style: { captions: true }, music: null },
    report: { frame_count: 90, fps: 30, duration_ms: 3000, issues: [], audio_events: [] },
    audioInfo: { narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null },
    flushEdits: () => Promise.resolve(true), showIssues() {}, notify() {}, api: {}
  };
  const exporter = S.createExport(inst);
  const job = { id: 'job-1', project_id: 'p1', kind: 'render', status: 'succeeded', progress: 1, output };
  if (warnings) job.warnings = warnings;
  exporter.apply(job, true);
  exporter.open();
  return { root: exporter.el, opened };
}
const mp4 = 'Documents/Storymaker/p1/exports/film-12345678.mp4';
const folder = 'Documents/Storymaker/p1/exports/film-12345678';

// Stems, SRT, VTT and library show when present. The silent music stem is absent, so it has no row.
const full = finished({
  path: mp4, size: 10,
  stems: { narration: folder + '/narration.wav', sfx: folder + '/sfx.wav' },
  srt: folder + '/captions.srt', vtt: folder + '/captions.vtt', library: 'Videos/Storymaker/film.mp4'
}, [{ code: 'stem_failed', message: 'music.wav could not be written' }, { code: 'library_failed', message: 'Videos is read-only' }]);
const rows = one(full.root, 'export-extras').children;
eq(rows.map(row => row.dataset.extra), ['narration', 'sfx', 'srt', 'vtt', 'library'], 'each extra that exists');
eq(rows.map(row => one(row, 'export-extra-path').textContent), [folder + '/narration.wav', folder + '/sfx.wav', folder + '/captions.srt', folder + '/captions.vtt', 'Videos/Storymaker/film.mp4'], 'the paths');
eq(one(full.root, 'export-path').textContent, mp4, 'the MP4 stays');

// Each row opens the folder of its own file.
one(rows[4], 'export-extra-show').click();
one(rows[0], 'export-extra-show').click();
eq(full.opened, [['files', 'Videos/Storymaker'], ['files', folder]], 'Files opens the folder of the file');

// Each warning shows as text.
eq(one(full.root, 'export-warnings').children.map(item => [item.dataset.code, item.textContent]), [['stem_failed', 'music.wav could not be written'], ['library_failed', 'Videos is read-only']], 'warnings');

// A job without extras shows only the MP4. Empty stems mean every stem was silent.
for (const plain of [{ path: mp4, size: 10 }, { path: mp4, size: 10, stems: {} }]) {
  const bare = finished(plain);
  eq(byStm(bare.root, 'export-extras').length, 0, 'no extras list');
  eq(byStm(bare.root, 'export-warnings').length, 0, 'no warnings list');
  eq(one(bare.root, 'export-path').textContent, mp4, 'only the MP4');
}
console.log('storymaker export finished ok');
`
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(StorymakerExportFinishedJobInNode)' -count=1
```

Expected: FAIL with `export-extras: found 0 nodes`, because `resultView` shows only the MP4 so far.

- [ ] **Step 4: Implement**

In `ui/js/desktop/apps/storymaker-export.js`, apply these two single-line replacements. They reuse classes that exist, so the stylesheet does not change.

Edit 1. Old:
```js
        function resultView() {
```
New:
```js
        // The extra files of a finished job, each with its own way to Files: stems and subtitles sit in the folder next to the
        // MP4, the library copy in Videos. A job without extras has no list.
        function extrasView(output) {
            const stems = output.stems || {};
            const rows = [
                ['narration', t('storymaker.export.label_narration'), stems.narration],
                ['music', t('storymaker.export.label_music'), stems.music],
                ['sfx', t('storymaker.export.label_sfx'), stems.sfx],
                ['srt', t('storymaker.export.label_srt'), output.srt],
                ['vtt', t('storymaker.export.label_vtt'), output.vtt],
                ['library', t('storymaker.export.label_library'), output.library]
            ].filter(row => row[2]);
            if (!rows.length) return null;
            const list = el('ul', { class: 'stm-caps', dataset: { stm: 'export-extras' } });
            rows.forEach(([name, label, path]) => {
                const folder = dirOf(path);
                list.appendChild(el('li', { class: 'stm-cap', dataset: { extra: name } },
                    el('div', { class: 'stm-cap-text' },
                        el('strong', { text: label }),
                        el('code', { class: 'stm-path', dataset: { stm: 'export-extra-path' }, text: String(path) })),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.export.show_in_files'), title: t('storymaker.export.show_in_files'), dataset: { stm: 'export-extra-show' }, disabled: !folder || typeof inst.ctx.openApp !== 'function' || null, onClick: () => { if (folder) inst.ctx.openApp('files', { path: folder }); } }, icon('folder'))));
            });
            return list;
        }

        // Server texts are shown as text only.
        function warningsView(finishedJob) {
            const warnings = Array.isArray(finishedJob.warnings) ? finishedJob.warnings : [];
            if (!warnings.length) return null;
            const list = el('ul', { class: 'stm-blockers', dataset: { stm: 'export-warnings' } });
            warnings.forEach(warning => {
                const item = warning || {};
                list.appendChild(el('li', { dataset: { code: String(item.code || '') }, text: String(item.message || item.code || '').slice(0, 600) }));
            });
            return [el('p', { class: 'stm-hint', text: t('storymaker.export.warnings') }), list];
        }

        function resultView() {
```

Edit 2. Old:
```js
                output.size ? el('p', { class: 'stm-hint', text: S.format.bytes(output.size) }) : null,
```
New:
```js
                output.size ? el('p', { class: 'stm-hint', text: S.format.bytes(output.size) }) : null,
                extrasView(output),
                warningsView(job),
```

- [ ] **Step 5: Run**

```powershell
go test ./ui -run '(?i)(StorymakerExportFinishedJobInNode|StorymakerExportStartInNode|StorymakerSourcesAreSafeAndSmall|StorymakerAppLoadsNothingExternal)' -count=1
```

Expected: `ok aurago/ui`. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the new keys until E3g, which outline section 3 allows.

- [ ] **Step 6: Commit**

Run detect_changes per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
```

```powershell
git add ui/js/desktop/apps/storymaker-export.js ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): show stems, subtitles, library copy and warnings of a finished export" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-E3-2 -->

### Task E3d: `buildTracks`. Wire: JS output against the Go validator
**Batch:** 2D  **Depends on:** none
**Files:**
- Create: `ui/js/desktop/apps/storymaker-handoff.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/desktop_storymaker_registration_test.go`
- Test: `ui/desktop_storymaker_handoff_test.go`

Tasks run in table order, so E3a has already added `dialog` to the loader and to `storymakerModules`.

- [ ] **Step 1: Write the failing test**

Create `ui/desktop_storymaker_handoff_test.go`:

```go
package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// storymakerHandoffPrelude loads the app core, the model and the handoff module into one context. The core gives S.uuid and
// S.errors, the model gives S.model.exportSize, and no module touches the DOM while it loads.
const storymakerHandoffPrelude = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];
const context = { console };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-handoff.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
`

// runStorymakerHandoffNode runs the prelude plus body in Node and returns what the script printed.
func runStorymakerHandoffNode(t *testing.T, body string) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-handoff-check.js")
	if err := os.WriteFile(script, []byte(storymakerHandoffPrelude+body), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(node, script, dir)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node check failed: %v\n%s", err, stderr.String())
	}
	return stdout.Bytes()
}

// decodeStorymakerHandoffProject reads a project the way the PUT handler does (unknown fields refused) and runs the Go validator.
func decodeStorymakerHandoffProject(t *testing.T, raw json.RawMessage) videostudio.Project {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var project videostudio.Project
	if err := decoder.Decode(&project); err != nil {
		t.Fatalf("the project does not decode strictly: %v\n%s", err, raw)
	}
	if err := videostudio.Validate(project); err != nil {
		t.Fatalf("videostudio.Validate refuses the project: %v\n%s", err, raw)
	}
	return project
}

// The tracks from Node pass the strict decoder and the Go validator; durations never pass the asset or the film; a silent stem has
// no track.
func TestStorymakerHandoffTracksInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerHandoffNode(t, storymakerHandoffTracksScript)
	var printed struct {
		Full   json.RawMessage `json:"full"`
		Silent json.RawMessage `json:"silent"`
	}
	if err := json.Unmarshal(out, &printed); err != nil {
		t.Fatalf("node output is not JSON: %v\n%s", err, out)
	}
	full := decodeStorymakerHandoffProject(t, printed.Full)
	silent := decodeStorymakerHandoffProject(t, printed.Silent)

	// The film has 250 frames, so each duration is min(asset frames, 250): video 300 -> 250, narration 180 -> 180,
	// music 400 -> 250, sfx 250 -> 250.
	want := []struct {
		kind     videostudio.TrackKind
		name     string
		asset    string
		duration int
	}{
		{videostudio.TrackVideo, "Video track", "a-video", 250},
		{videostudio.TrackAudio, "Narration track", "a-narration", 180},
		{videostudio.TrackAudio, "Music track", "a-music", 250},
		{videostudio.TrackAudio, "Effects track", "a-sfx", 250},
	}
	if len(full.Tracks) != len(want) {
		t.Fatalf("got %d tracks, want %d", len(full.Tracks), len(want))
	}
	for i, w := range want {
		track := full.Tracks[i]
		if track.Kind != w.kind || track.Name != w.name || len(track.Clips) != 1 || !strings.HasPrefix(track.ID, "track-") {
			t.Fatalf("track %d = %+v", i, track)
		}
		clip := track.Clips[0]
		if clip.AssetID != w.asset || clip.Duration != w.duration || clip.Start != 0 || clip.Offset != 0 || !strings.HasPrefix(clip.ID, "clip-") {
			t.Errorf("track %d clip = %+v", i, clip)
		}
		// Only the picture detaches its audio: the stems carry the sound.
		if clip.AudioDetached != (w.kind == videostudio.TrackVideo) {
			t.Errorf("track %d audio_detached = %v", i, clip.AudioDetached)
		}
	}

	// Music and sfx were silent, so their stems do not exist and they get no track.
	if len(silent.Tracks) != 2 || silent.Tracks[0].Kind != videostudio.TrackVideo || silent.Tracks[1].Clips[0].AssetID != "a-narration" {
		t.Fatalf("tracks without silent stems = %+v", silent.Tracks)
	}
}

const storymakerHandoffTracksScript = `
const audio = (id, frames) => ({ id, name: id + '.wav', path: 'media/' + id + '.wav', kind: 'audio', duration_frames: frames, duration_us: Math.round(frames / 30 * 1e6), has_audio: true, codec: 'pcm_s16le', size_bytes: 1000 });
const project = {
  version: 3, name: 'Firewall', width: 1920, height: 1080, fps: 30,
  assets: [
    { id: 'a-video', name: 'film.mp4', path: 'media/a-video.mp4', kind: 'video', duration_frames: 300, duration_us: 10000000, video_duration_us: 10000000, width: 1920, height: 1080, has_audio: true, source_fps: '30/1', codec: 'h264', size_bytes: 1000 },
    audio('a-narration', 180), audio('a-music', 400), audio('a-sfx', 250)
  ],
  tracks: []
};
const names = { video: 'Video track', narration: 'Narration track', music: 'Music track', sfx: 'Effects track' };
const full = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration', music: 'a-music', sfx: 'a-sfx' }, 250, names);
const silent = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration' }, 250, names);
console.log(JSON.stringify({ full: Object.assign({}, project, { tracks: full }), silent: Object.assign({}, project, { tracks: silent }) }));
`
```

In `ui/desktop_storymaker_registration_test.go`, replace this text, quoted as E3a leaves it:

```go
"music", "dialog"}
```

with:

```go
"music", "dialog", "handoff"}
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerHandoffTracksInNode|TestDesktopStorymakerRegistration)' -count=1
```

Expected: FAIL.
- `TestStorymakerHandoffTracksInNode`: node stops with `ENOENT` on `storymaker-handoff.js`, because the module does not exist.
- `TestDesktopStorymakerRegistration`: `the storymaker loader entry lists /js/desktop/apps/storymaker-handoff.js' out of order or not at all`.

- [ ] **Step 3: Implement**

Create `ui/js/desktop/apps/storymaker-handoff.js`:

```js
// Storymaker app handoff to Video Studio. buildTracks turns the imported film and its stems into the tracks of a Video Studio
// project: the picture on one video track with its audio detached, then one audio track for each stem that exists. It is pure, and
// a Node test checks its output against the Go validator. Classic script, publishes S.handoff.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const STEMS = ['narration', 'music', 'sfx'];

    // The fields follow Clip in internal/videostudio/model.go and makeClip in video-studio-timeline.js. Validate refuses one frame
    // past the asset, so a duration never exceeds the asset or the film.
    function clipOf(asset, frameCount) {
        return {
            id: 'clip-' + S.uuid(), asset_id: asset.id, start: 0, offset: 0,
            duration: Math.min(asset.duration_frames, frameCount),
            x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1,
            fade_in: 0, fade_out: 0, fit: 'contain'
        };
    }

    function trackOf(kind, name, clip) {
        return { id: 'track-' + S.uuid(), kind, name, muted: false, hidden: false, locked: false, clips: [clip] };
    }

    // buildTracks(project, assets, frameCount, names): assets maps video, narration, music and sfx to asset ids of project.assets;
    // names holds the track names under the same keys. A stem without an asset is skipped, because a silent stem has no file.
    function buildTracks(project, assets, frameCount, names) {
        const byId = new Map((project.assets || []).map(asset => [asset.id, asset]));
        const video = byId.get(assets.video);
        if (!video) throw new Error('The imported video is missing from the project.');
        const tracks = [trackOf('video', names.video, Object.assign(clipOf(video, frameCount), { audio_detached: true }))];
        STEMS.forEach(name => {
            const asset = byId.get(assets[name]);
            if (asset) tracks.push(trackOf('audio', names[name], clipOf(asset, frameCount)));
        });
        return tracks;
    }

    S.handoff = { buildTracks };
})();
```

In `ui/js/desktop/core/module-loader.js`, replace this text, which occurs once:

```js
'/js/desktop/apps/storymaker.js']
```

with:

```js
'/js/desktop/apps/storymaker-handoff.js', '/js/desktop/apps/storymaker.js']
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerHandoffTracksInNode|TestDesktopStorymakerRegistration|TestStorymakerLoaderListsEveryScript|TestStorymakerSourcesAreSafeAndSmall)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run the graph check. For a PARTIAL answer on the new file, a crash, a closed connection, a `not found` answer or a banner-only answer, follow outline section 5 and state the grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-handoff.js ui/js/desktop/core/module-loader.js ui/desktop_storymaker_registration_test.go ui/desktop_storymaker_handoff_test.go
git commit -m "feat(storymaker): build Video Studio tracks for the handoff" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3e: `handoff.run` and `canOffer`
**Batch:** 2D  **Depends on:** E3c, E3d
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-handoff.js`
- Modify: `ui/js/desktop/apps/storymaker-export.js`
- Modify: `ui/js/desktop/apps/storymaker-dialog.js`
- Test: `ui/desktop_storymaker_handoff_test.go`
- Test: `ui/desktop_storymaker_audio_test.go`
- Test: `ui/desktop_storymaker_model_test.go`

From this commit until E3g, `TestStorymakerAppUsesOnlyKeysItHas` is red on the `storymaker.handoff.*` keys and `storymaker.export.last_export`. Outline section 3 allows that.

- [ ] **Step 1: Impact**

Run this from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExport" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExportDialog" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_handoff_test.go`, replace this line, which E3d leaves in the file:

```go
const storymakerHandoffTracksScript = `
```

with:

```go
// The whole handoff against a fake fetch and a fake timer: the order of the calls, the import keys, the poll, the PUT with the fresh
// ETag and the launch. canOffer decides on the job and the Video Studio status.
func TestStorymakerHandoffRunInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerHandoffNode(t, storymakerHandoffRunScript)
	if !strings.Contains(string(out), "storymaker handoff ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// The export panel offers the handoff through the module and honours the wish from the dialog. The app tour runs it for real.
func TestStorymakerExportOffersHandoff(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-export.js")
	for _, marker := range []string{
		"S.handoff.canOffer(job, vsStatus)",
		"S.handoff.status()",
		"S.handoff.run(inst, job, handoffProjects)",
		"inst.handoffWish.delete(id)",
		"dataset: { stm: 'export-handoff' }",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("storymaker-export.js lacks %q", marker)
		}
	}
}

// An export that ended while no panel watched it: a replay alone changes nothing, a wish from this session runs the handoff, and the
// job list gives the way back to the result with its paths, warnings and handoff.
func TestStorymakerExportKeepsLastJobInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerExportScript(t, "storymaker-export-last.js", storymakerExportLastJobScript)
	if !strings.Contains(out, "storymaker export last job ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportLastJobScript = `
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-handoff.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
// The real canOffer decides. Only the two calls that leave the page are stand-ins.
const runs = [];
S.handoff.status = () => Promise.resolve({ enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false });
S.handoff.run = (inst, job) => { runs.push(job.id); return Promise.resolve({ projectId: 'vs1' }); };
const mp4 = 'Documents/Storymaker/p1/exports/film-12345678.mp4';
const finished = id => ({ id, project_id: 'p1', kind: 'render', status: 'succeeded', progress: 1, frames_total: 90, output: { path: mp4, size: 10, stems: { narration: 'Documents/Storymaker/p1/exports/film-12345678/narration.wav' } }, warnings: [{ code: 'library_failed', message: 'Videos is read-only' }] });
const ended = (id, status) => ({ id, project_id: 'p1', kind: 'render', status });
const answering = jobs => () => Promise.resolve(jobs);
function setup(listJobs) {
  return {
    t: key => key, domId: name => 'test-' + name, bus: S.createBus(), parts: {}, readonly: false, projectId: 'p1', etag: '"e1"', issues: [], handoffWish: new Set(),
    ctx: { openApp() {} },
    status: { ffmpeg: { ready: true }, browser: { ready: true, source: 'system' }, limits: {} },
    story: { format: { aspect: '16:9' }, style: { captions: true }, music: null },
    report: { frame_count: 90, fps: 30, duration_ms: 3000, issues: [], audio_events: [] },
    audioInfo: { narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null },
    flushEdits: () => Promise.resolve(true), showIssues() {}, notify() {},
    api: { listJobs }
  };
}
(async () => {
  // A finished job that the stream replays without a wish leaves the panel idle, and an empty list offers nothing.
  let inst = setup(answering([]));
  let exporter = S.createExport(inst);
  let root = exporter.el;
  await settle();
  inst.bus.emit('job', finished('old'));
  await settle();
  eq([exporter.job(), one(root, 'export').dataset.state], [null, 'idle'], 'a replayed job without a wish stays out');
  exporter.open();
  one(root, 'export-ready');
  eq(byStm(root, 'export-last').length, 0, 'an empty list offers nothing');
  eq(runs, [], 'nothing runs');
  exporter.dispose();

  // A replayed finished job with a wish runs the handoff once, and the wish is used up.
  inst = setup(answering([]));
  inst.handoffWish.add('wished');
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', finished('wished'));
  await settle();
  eq(runs, ['wished'], 'a wish from this session runs on the way back');
  eq(inst.handoffWish.size, 0, 'the wish is used up');
  inst.bus.emit('job', finished('wished'));
  await settle();
  eq(runs, ['wished'], 'a second replay runs nothing');
  exporter.dispose();

  // The replay can miss a wished job, the list cannot.
  runs.length = 0;
  inst = setup(answering([finished('listed')]));
  inst.handoffWish.add('listed');
  exporter = S.createExport(inst);
  await settle();
  eq(runs, ['listed'], 'the list also runs a wish');
  exporter.dispose();

  // The way back: the newest render that left a result. Narrate jobs and renders that ended without one do not count.
  runs.length = 0;
  inst = setup(answering([{ id: 'n1', project_id: 'p1', kind: 'narrate', status: 'succeeded' }, ended('r4', 'failed'), ended('r3', 'cancelled'), ended('r2', 'running'), finished('job-1'), finished('job-0')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  exporter.open();
  eq(byStm(root, 'export-last').length, 0, 'the list has not answered yet');
  await settle();
  one(root, 'export-ready');
  one(root, 'export-last').click();
  eq(exporter.job().id, 'job-1', 'the newest one');
  one(root, 'export-done');
  eq(one(root, 'export-path').textContent, mp4, 'its path');
  one(root, 'export-extras');
  one(root, 'export-warnings');
  eq(runs, [], 'the way back starts nothing by itself');
  await settle();
  one(root, 'export-handoff').click();
  await settle();
  eq(runs, ['job-1'], 'the handoff is one click away');
  one(root, 'export-again').click();
  eq(exporter.job(), null, 'back at the start view');
  one(root, 'export-last');
  exporter.dispose();

  // An interrupted export says so on the way back.
  inst = setup(answering([ended('cut', 'interrupted'), finished('older')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-last').click();
  eq(one(root, 'export-failed').dataset.status, 'interrupted', 'the interrupted text shows');
  exporter.dispose();

  // A list that fails offers nothing and breaks nothing.
  inst = setup(() => Promise.reject(new Error('offline')));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-ready');
  eq(byStm(root, 'export-last').length, 0, 'a failed list offers nothing');
  exporter.dispose();

  // A late answer does nothing for a panel that is gone or a project that was left.
  runs.length = 0;
  let release = null;
  inst = setup(() => new Promise(resolve => { release = resolve; }));
  inst.handoffWish.add('late');
  exporter = S.createExport(inst);
  exporter.dispose();
  release([finished('late')]);
  await settle();
  eq(runs, [], 'a disposed panel starts nothing');
  eq(inst.handoffWish.size, 1, 'the wish stays for the next panel');
  inst = setup(() => new Promise(resolve => { release = resolve; }));
  exporter = S.createExport(inst);
  root = exporter.el;
  exporter.open();
  inst.projectId = 'p2';
  release([finished('late')]);
  await settle();
  eq(byStm(root, 'export-last').length, 0, 'an answer for the project before stays out');
  exporter.dispose();

  // A wished job that ended without a film shows once and gives up the wish, so the next open of the project is clean.
  inst = setup(answering([]));
  inst.handoffWish.add('stopped');
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', ended('stopped', 'cancelled'));
  eq(exporter.job().id, 'stopped', 'the ending of a wished job shows');
  eq(inst.handoffWish.size, 0, 'the wish ends with the job');
  exporter.dispose();
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', ended('stopped', 'cancelled'));
  eq(exporter.job(), null, 'a later replay stays out');
  exporter.dispose();

  // A failed stem: the wish runs nothing, no action is offered, and the panel says why.
  runs.length = 0;
  inst = setup(answering([]));
  const notices = [];
  inst.notify = (text, kind) => notices.push([text, kind]);
  inst.handoffWish.add('holed');
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  inst.bus.emit('job', Object.assign(finished('holed'), { warnings: [{ code: 'stem_failed', message: 'music.wav could not be written' }] }));
  await settle();
  exporter.open();
  eq(runs, [], 'a failed stem starts no handoff');
  eq(notices, [['storymaker.handoff.unavailable', 'error']], 'the wish learns why');
  eq(byStm(root, 'export-handoff').length, 0, 'no action');
  eq(one(root, 'export-handoff-unavailable').textContent, 'storymaker.handoff.unavailable', 'the panel says why');
  exporter.dispose();

  // A failed run keeps its project for the next click.
  const kept = [];
  S.handoff.run = (_, job, projects) => { kept.push(projects.get(job.id) || ''); projects.set(job.id, 'vs1'); return Promise.reject(new Error('timed out')); };
  inst = setup(answering([finished('retry')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-last').click();
  await settle();
  one(root, 'export-handoff').click();
  await settle();
  eq(one(root, 'export-handoff-error').textContent, 'timed out', 'the failure shows');
  one(root, 'export-handoff').click();
  await settle();
  eq(kept, ['', 'vs1'], 'the retry finds the project of the first try');
  exporter.dispose();

  console.log('storymaker export last job ok');
})().catch(fail);
`

const storymakerHandoffRunScript = `
const API = '/api/desktop/video-studio';
const calls = [];
const delays = [];
const opened = [];
const polled = {};
let stuck = false;
let broken = false;
// The timer fires at once and records its delay, so 120 s of polling takes no real time.
context.setTimeout = (fn, ms) => { delays.push(ms); fn(); return 0; };
const answer = (value, status, headers) => new Response(JSON.stringify(value), { status: status || 200, headers: Object.assign({ 'Content-Type': 'application/json' }, headers || {}) });
const stored = {
  version: 3, name: 'Firewall', width: 1080, height: 1920, fps: 25,
  assets: [
    { id: 'a-video', name: 'film-job1.mp4', path: 'media/a-video_film-job1.mp4', kind: 'video', duration_frames: 300, width: 1080, height: 1920, has_audio: true },
    { id: 'a-narration', name: 'narration.wav', folder: 'Sound', path: 'media/a-narration_narration.wav', kind: 'audio', duration_frames: 180, has_audio: true },
    { id: 'a-sfx', name: 'sfx.wav', path: 'media/a-sfx_sfx.wav', kind: 'audio', duration_frames: 400, has_audio: true }
  ],
  tracks: []
};
context.fetch = (url, init) => {
  calls.push({ url, method: init.method, headers: init.headers, body: init.body, credentials: init.credentials });
  const route = init.method + ' ' + url;
  if (route === 'POST ' + API + '/projects') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"created"' }, 201, { ETag: '"created"' }));
  if (route === 'POST ' + API + '/projects/vs1/media') {
    const name = path.basename(JSON.parse(init.body).source_path).replace(/\.wav$/, '').replace(/^film.*$/, 'video');
    return Promise.resolve(answer({ asset: { id: 'a-' + name }, media_url: 'x', job: { id: 'probe-' + name, status: 'queued' } }, 202));
  }
  if (init.method === 'GET' && url.startsWith(API + '/jobs/')) {
    const id = url.slice((API + '/jobs/').length);
    polled[id] = (polled[id] || 0) + 1;
    // The first job answers running once, so the poll has to repeat.
    const running = stuck || (id === 'probe-video' && polled[id] === 1);
    return Promise.resolve(answer({ id, status: broken ? 'failed' : running ? 'running' : 'succeeded' }));
  }
  if (route === 'GET ' + API + '/projects/vs1') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"fresh"' }, 200, { ETag: '"fresh"' }));
  if (route === 'PUT ' + API + '/projects/vs1') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"saved"' }, 200, { ETag: '"saved"' }));
  return Promise.resolve(answer({ code: 'not_found', message: 'no route ' + route }, 404));
};
const inst = { t: key => key, disposed: false, story: { title: 'Firewall', format: { aspect: '9:16', fps: 25 } }, project: { title: 'Firewall' }, ctx: { openApp: (app, launch) => opened.push([app, launch]) } };
const exportsDir = 'Documents/Storymaker/p1/exports/';
// The music stem was silent, so job.output.stems has no music.
const job = { id: 'job1', status: 'succeeded', frames_total: 250, output: { path: exportsDir + 'film-job1.mp4', size: 9, stems: { narration: exportsDir + 'film-job1/narration.wav', sfx: exportsDir + 'film-job1/sfx.wav' } } };

(async () => {
  const result = await S.handoff.run(inst, job);
  eq(result, { projectId: 'vs1' }, 'the answer');
  eq(calls.map(call => call.method + ' ' + call.url.slice(API.length)), [
    'POST /projects',
    'POST /projects/vs1/media', 'POST /projects/vs1/media', 'POST /projects/vs1/media',
    'GET /jobs/probe-video', 'GET /jobs/probe-video', 'GET /jobs/probe-narration', 'GET /jobs/probe-sfx',
    'GET /projects/vs1',
    'PUT /projects/vs1'
  ], 'the order of the calls');
  eq(calls[0].credentials, 'same-origin', 'the session goes along');
  eq(JSON.parse(calls[0].body), { name: 'Firewall', width: 1080, height: 1920, fps: 25, ntsc: false }, 'canvas and rate follow the story');

  const imports = calls.slice(1, 4);
  eq(imports.map(call => call.headers['Idempotency-Key']), ['storymaker-job1-video', 'storymaker-job1-narration', 'storymaker-job1-sfx'], 'the import keys');
  eq(imports.map(call => JSON.parse(call.body)), [
    { source_path: exportsDir + 'film-job1.mp4', folder: '' },
    { source_path: exportsDir + 'film-job1/narration.wav', folder: '' },
    { source_path: exportsDir + 'film-job1/sfx.wav', folder: '' }
  ], 'the import bodies');
  eq(imports[0].headers['Content-Type'], 'application/json', 'without JSON the server answers 415');

  // Four polls: video twice (running, then succeeded), narration once, sfx once.
  eq(delays, [500, 500, 500, 500], 'one 500 ms wait before each poll');

  const put = calls[calls.length - 1];
  eq(put.headers['If-Match'], '"fresh"', 'If-Match carries the ETag of the last GET');
  const sent = JSON.parse(put.body);
  eq([sent.version, sent.name, sent.width, sent.height, sent.fps], [3, 'Firewall', 1080, 1920, 25], 'the whole project');
  eq(sent.assets, [{ id: 'a-video', name: 'film-job1.mp4' }, { id: 'a-narration', name: 'narration.wav', folder: 'Sound' }, { id: 'a-sfx', name: 'sfx.wav' }], 'only the asset fields the client owns');
  // frames_total is 250: video min(300, 250) = 250, narration min(180, 250) = 180, sfx min(400, 250) = 250.
  eq(sent.tracks.map(track => [track.kind, track.name, track.clips[0].asset_id, track.clips[0].duration]), [
    ['video', 'storymaker.handoff.track_video', 'a-video', 250],
    ['audio', 'storymaker.handoff.track_narration', 'a-narration', 180],
    ['audio', 'storymaker.handoff.track_sfx', 'a-sfx', 250]
  ], 'the tracks');
  eq(opened, [['video-studio', { projectId: 'vs1' }]], 'the launch');

  // A probe that never ends: 120 s at one poll per 500 ms is 120000 / 500 = 240 polls, all on the first job.
  calls.length = 0; delays.length = 0; opened.length = 0;
  Object.keys(polled).forEach(key => delete polled[key]);
  stuck = true;
  let refused = null;
  try { await S.handoff.run(inst, job); } catch (err) { refused = err; }
  if (!refused) throw new Error('a probe that never ends must reject');
  eq(polled, { 'probe-video': 240 }, 'the poll stops at 120 s');
  eq(delays.length, 240, 'one wait per poll');
  eq(delays.every(ms => ms === 500), true, 'every wait is 500 ms');
  eq(calls.some(call => call.method === 'PUT'), false, 'no PUT after a timeout');
  eq(opened, [], 'no launch after a timeout');

  // The panel's Map keeps the project of a failed run. The retry creates none, repeats the imports with the same keys and fills it.
  const projects = new Map();
  refused = null;
  try { await S.handoff.run(inst, job, projects); } catch (err) { refused = err; }
  if (!refused) throw new Error('the failed run must reject');
  eq(Array.from(projects), [['job1', 'vs1']], 'the failed run keeps its project');
  calls.length = 0; opened.length = 0;
  stuck = false;
  eq(await S.handoff.run(inst, job, projects), { projectId: 'vs1' }, 'the retry fills that project');
  eq(calls.map(call => call.method + ' ' + call.url.slice(API.length)).filter(route => !route.startsWith('GET /jobs/')), [
    'POST /projects/vs1/media', 'POST /projects/vs1/media', 'POST /projects/vs1/media',
    'GET /projects/vs1',
    'PUT /projects/vs1'
  ], 'the retry creates no project');
  eq(calls.slice(0, 3).map(call => call.headers['Idempotency-Key']), ['storymaker-job1-video', 'storymaker-job1-narration', 'storymaker-job1-sfx'], 'the same import keys');
  eq(opened, [['video-studio', { projectId: 'vs1' }]], 'the retry opens it');
  eq(projects.size, 0, 'a finished handoff forgets the project, so a later click cannot overwrite edits made in Video Studio');

  // A kept project that was deleted in Video Studio is given up once.
  calls.length = 0;
  projects.set('job1', 'gone');
  eq(await S.handoff.run(inst, job, projects), { projectId: 'vs1' }, 'a deleted project gets a new one');
  eq(calls.slice(0, 2).map(call => call.method + ' ' + call.url.slice(API.length)), ['POST /projects/gone/media', 'POST /projects'], 'the order after the 404');

  // A failed probe replays as failed for the same key, so only a new project can try the file again.
  broken = true;
  refused = null;
  try { await S.handoff.run(inst, job, projects); } catch (err) { refused = err; }
  eq(refused && refused.message, 'storymaker.handoff.import_failed', 'a failed probe says so');
  eq(projects.size, 0, 'a failed probe gives its project up');
  broken = false;

  const ready = { enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false };
  eq(S.handoff.canOffer(job, ready), true, 'a finished job with stems');
  eq(S.handoff.canOffer({ id: 'x', status: 'succeeded', output: { path: 'a', stems: {} } }, ready), true, 'stems asked for, all silent: the film still goes over');
  eq(S.handoff.canOffer(Object.assign({}, job, { status: 'failed' }), ready), false, 'a failed job');
  eq(S.handoff.canOffer({ id: 'x', status: 'succeeded', output: { path: 'a' } }, ready), false, 'no stems asked for');
  for (const field of ['enabled', 'desktop_enabled', 'ffmpeg_ready']) eq(S.handoff.canOffer(job, Object.assign({}, ready, { [field]: false })), false, field);
  eq(S.handoff.canOffer(job, Object.assign({}, ready, { read_only: true })), false, 'read-only');
  eq(S.handoff.canOffer(job, null), false, 'no status yet');
  eq(S.handoff.canOffer(Object.assign({}, job, { warnings: [{ code: 'stem_failed', message: 'narration.wav could not be written' }] }), ready), false, 'a failed stem would leave that sound out of the project');
  eq(S.handoff.canOffer(Object.assign({}, job, { warnings: [{ code: 'library_failed', message: 'Videos is read-only' }] }), ready), true, 'another warning does not stop the offer');
  eq([S.handoff.studioReady(ready), S.handoff.studioReady(Object.assign({}, ready, { read_only: true })), S.handoff.studioReady(null)], [true, false, false], 'the status half of canOffer');
  console.log('storymaker handoff ok');
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`

const storymakerHandoffTracksScript = `
```

The stream handler now reads `inst.handoffWish`, and the bus swallows what a handler throws, so the stand-in of `TestStorymakerNarrationStatusInNode` needs the `Set` that the app has. Without it the render event at `a render job reaches the popover` is lost. In `ui/desktop_storymaker_audio_test.go`, replace this line, as E1f wrote it:

```js
    t, bus: S.createBus(), selected: 0, projectId: 'p1', readonly: false, status: {}, issues: [], report: null, etag: '', ctx: {},
```

with:

```js
    t, bus: S.createBus(), selected: 0, projectId: 'p1', readonly: false, status: {}, issues: [], report: null, etag: '', ctx: {}, handoffWish: new Set(),
```

Opening the dialog now asks Video Studio for its status, so every script that opens the dialog loads the handoff module. The start and refusal scripts have no `fetch` in their context, so their status read fails. The dialog treats that as unavailable, and these scripts do not use the option. In `ui/desktop_storymaker_model_test.go`, as E3a to E3c leave it, apply these five replacements.

(a) The dialog test gets the real `studioReady` and a status answer it controls.

Old:

```js
load(['storymaker-core.js', 'storymaker-dialog.js']);
const S = context.StorymakerParts;
```

New:

```js
load(['storymaker-core.js', 'storymaker-handoff.js', 'storymaker-dialog.js']);
const S = context.StorymakerParts;
// What Video Studio answers when the dialog opens. The real studioReady decides.
const vsReady = { enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false };
let vsAnswer = () => Promise.resolve(vsReady);
S.handoff.status = () => vsAnswer();
```

(b) Old:

```js
  console.log('storymaker export dialog ok');
```

New:

```js
  // A studio that can take the film keeps the option and shows no reason.
  eq([handoff.disabled, one(dialog.el, 'export-opt-handoff-reason').hidden], [false, true], 'the option stays');

  // A studio that cannot, or a status read that fails: the option is off with its reason, and a tick made before the answer goes back
  // with its forced stems.
  const unavailable = [() => Promise.resolve(Object.assign({}, vsReady, { read_only: true })), () => Promise.reject(new Error('offline'))];
  for (const answer of unavailable) {
    vsAnswer = answer;
    const off = makeInst(true, { id: 'job-8' });
    const offDialog = S.createExportDialog(off);
    offDialog.open();
    const offHandoff = one(offDialog.el, 'export-opt-handoff');
    const offStems = one(offDialog.el, 'export-opt-stems');
    const reason = one(offDialog.el, 'export-opt-handoff-reason');
    offHandoff.checked = true; offHandoff.fire('change');
    await settle();
    eq([offHandoff.checked, offHandoff.disabled, offStems.checked, offStems.disabled, reason.hidden, reason.textContent], [false, true, false, false, false, 'storymaker.handoff.unavailable'], 'the option is off with its reason');
    one(offDialog.el, 'export-confirm').click();
    await settle();
    eq([off.sent[0].stems, off.handoffWish.size], [false, 0], 'no stems and no wish');
  }

  console.log('storymaker export dialog ok');
```

(c) The start test opens the dialog.

Old:

```js
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
const renders = [];
```

New:

```js
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-handoff.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
const renders = [];
```

(d) So does the refusal test.

Old:

```js
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-export.js']) {
```

New:

```js
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-handoff.js', 'storymaker-export.js']) {
```

(e) The result view now also asks the module about a failed stem, so the stand-in of the finished-job test answers that too.

Old:

```js
S.handoff = { canOffer: () => false, run: () => Promise.resolve({ projectId: '' }) };
```

New:

```js
S.handoff = { canOffer: () => false, stemFailed: () => false, run: () => Promise.resolve({ projectId: '' }) };
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerHandoffRunInNode|TestStorymakerExportOffersHandoff|TestStorymakerExportKeepsLastJobInNode|TestStorymakerExportDialogInNode)' -count=1
```

Expected: FAIL.
- `TestStorymakerHandoffRunInNode`: `S.handoff.run is not a function`.
- `TestStorymakerExportOffersHandoff`: `storymaker-export.js lacks "S.handoff.canOffer(job, vsStatus)"`, and the same for the other markers.
- `TestStorymakerExportKeepsLastJobInNode`: `a wish from this session runs on the way back: got [], want ["wished"]`.
- `TestStorymakerExportDialogInNode`: `export-opt-handoff-reason: found 0 nodes`.

- [ ] **Step 4: Implement `run`, `canOffer` and `status`**

In `ui/js/desktop/apps/storymaker-handoff.js`, replace this line, as E3d leaves it:

```js
    S.handoff = { buildTracks };
```

with:

```js
    // ---- the run --------------------------------------------------------------------------------------------------------------

    const API = '/api/desktop/video-studio';
    const POLL_MS = 500;
    const POLL_LIMIT_MS = 120000;
    const MAX_NAME_CHARS = 120;
    const PROBE_FAILED = 'handoff_probe_failed';

    // One Video Studio request: {body, etag}. A failure is thrown in the app's error shape (S.errors).
    async function call(method, path, headers, body) {
        const init = { method, headers: Object.assign({}, headers || {}), credentials: 'same-origin', cache: 'no-store' };
        if (body !== undefined) {
            init.headers['Content-Type'] = 'application/json';
            init.body = JSON.stringify(body);
        }
        let response;
        try {
            response = await window.fetch(API + path, init);
        } catch (err) {
            throw S.errors.normalize({ message: String(err && err.message || err) });
        }
        let answer = null;
        try { answer = await response.json(); } catch (_) { answer = null; }
        if (!response.ok) throw S.errors.fromResponse(response.status, answer, response.headers.get('retry-after'));
        const data = answer && typeof answer === 'object' ? answer : {};
        return { body: data, etag: response.headers.get('ETag') || String(data.etag || '') };
    }

    function refused(message, code) {
        return { stm: true, status: 0, code: code || 'handoff_failed', message, issues: [], network: false };
    }

    function wait(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    // Waits for the probe jobs in turn. They share one limit, so the whole wait stays within 120 s.
    async function waitForProbes(inst, jobIds) {
        let waited = 0;
        for (const id of jobIds) {
            for (;;) {
                if (inst.disposed) throw refused('');
                if (waited >= POLL_LIMIT_MS) throw refused(inst.t('storymaker.handoff.timeout'));
                await wait(POLL_MS);
                waited += POLL_MS;
                const { body } = await call('GET', '/jobs/' + encodeURIComponent(id));
                if (body.status === 'succeeded') break;
                if (body.status !== 'queued' && body.status !== 'running') throw refused(inst.t('storymaker.handoff.import_failed'), PROBE_FAILED);
            }
        }
    }

    // Canvas and rate follow the story. Video Studio accepts every Storymaker format, so nothing is rescaled.
    function projectSpec(inst) {
        const story = inst.story || {};
        const format = story.format || {};
        const size = S.model.exportSize(format.aspect);
        const title = String(story.title || (inst.project && inst.project.title) || '').trim() || 'Storymaker';
        return { name: Array.from(title).slice(0, MAX_NAME_CHARS).join('').trim(), width: size[0], height: size[1], fps: Number(format.fps) || 30, ntsc: false };
    }

    // The client owns only these asset fields. The server fills in everything else from its media records.
    function keptAsset(asset) {
        const kept = { id: asset.id, name: asset.name };
        if (asset.folder) kept.folder = asset.folder;
        return kept;
    }

    // Imports the MP4 and each stem into the project, waits for the probes, reads the project back for its ETag and assets and writes
    // the tracks with If-Match.
    async function fillProject(inst, job, projectId) {
        const output = job.output || {};
        const stems = output.stems || {};
        const sources = [['video', output.path]].concat(STEMS.filter(name => stems[name]).map(name => [name, stems[name]]));
        const base = '/projects/' + encodeURIComponent(projectId);
        const assets = {};
        const probes = [];
        for (const [name, source] of sources) {
            const imported = await call('POST', base + '/media', { 'Idempotency-Key': 'storymaker-' + job.id + '-' + name }, { source_path: source, folder: '' });
            assets[name] = imported.body.asset.id;
            probes.push(imported.body.job.id);
        }
        await waitForProbes(inst, probes);
        const fresh = await call('GET', base);
        const project = fresh.body.project;
        const tracks = buildTracks(project, assets, Number(job.frames_total) || 0, {
            video: inst.t('storymaker.handoff.track_video'),
            narration: inst.t('storymaker.handoff.track_narration'),
            music: inst.t('storymaker.handoff.track_music'),
            sfx: inst.t('storymaker.handoff.track_sfx')
        });
        await call('PUT', base, { 'If-Match': fresh.etag }, Object.assign({}, project, { version: 3, assets: project.assets.map(keptAsset), tracks }));
    }

    // run(inst, job, projects) -> {projectId}: creates the project, fills it and opens Video Studio on it. projects is the panel's Map from
    // job id to Video Studio project, and it is optional. It keeps the project of a failed try, so a retry fills that one and adds no
    // other: the import keys are per project, so the repeated imports replay the first ones and double no asset. A finished handoff
    // drops the entry, so a later click starts a new project and cannot overwrite edits made in Video Studio.
    async function run(inst, job, projects) {
        const kept = projects || new Map();
        const reused = kept.get(job.id);
        const projectId = reused || (await call('POST', '/projects', null, projectSpec(inst))).body.id;
        kept.set(job.id, projectId);
        try {
            await fillProject(inst, job, projectId);
        } catch (err) {
            // A 404 on a kept project means it was deleted in Video Studio. A failed probe replays as failed for the same key, so only a
            // new project can try the file again.
            const gone = reused && err && err.status === 404;
            if (gone || (err && err.code === PROBE_FAILED)) kept.delete(job.id);
            if (gone) return run(inst, job, kept);
            throw err;
        }
        inst.ctx.openApp('video-studio', { projectId });
        kept.delete(job.id);
        return { projectId };
    }

    // The status half of canOffer: Video Studio can take a film now. The export dialog asks this before a render starts.
    function studioReady(vsStatus) {
        const status = vsStatus || {};
        return !!(status.enabled && status.desktop_enabled && status.ffmpeg_ready && !status.read_only);
    }

    // A stem that failed to save has no track, and the film's own mix is detached from the picture, so that sound would be missing.
    function stemFailed(job) {
        return !!job && Array.isArray(job.warnings) && job.warnings.some(warning => warning && warning.code === 'stem_failed');
    }

    function canOffer(job, vsStatus) {
        return !!(job && job.status === 'succeeded' && job.output && job.output.stems && !stemFailed(job) && studioReady(vsStatus));
    }

    // The Video Studio status object that canOffer reads.
    async function status() {
        return (await call('GET', '/status')).body;
    }

    S.handoff = { canOffer, studioReady, stemFailed, run, status, buildTracks };
```

- [ ] **Step 5: Implement the export panel wiring and the dialog check**

Apply these eleven single-line replacements in `ui/js/desktop/apps/storymaker-export.js`. E3a to E3c leave each old line unchanged, and each occurs once.

(a) Old:

```js
        let copied = 0;
```

New:

```js
        let copied = 0;
        let vsStatus = null;
        let vsAskedFor = '';
        let handingOff = false;
        let handoffError = '';
        const handoffProjects = new Map();   // job id -> the Video Studio project of a failed try, so a retry fills it and adds no other
        let lastJob = null;   // the newest render that left a result, also one from before this panel
```

(b) The "again" button of resultView, the one without an icon, gets the handoff action in front of it.

Old:

```js
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, el('span', { text: t('storymaker.export.again') }))));
```

New:

```js
                    handoffAction(),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, el('span', { text: t('storymaker.export.again') }))));
```

(c) Old:

```js
        function failedView() {
```

New:

```js
        // A stem that failed would leave that sound out of the Video Studio project, so canOffer refuses and the panel says why.
        function handoffUnavailable() {
            return S.handoff.stemFailed(job) ? el('p', { class: 'stm-hint', dataset: { stm: 'export-handoff-unavailable' }, text: t('storymaker.handoff.unavailable') }) : null;
        }

        // The Video Studio action of a finished job, shown only when S.handoff.canOffer agrees. el() takes the array and skips the null.
        function handoffAction() {
            if (!S.handoff.canOffer(job, vsStatus)) return null;
            return [
                el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-handoff' }, disabled: handingOff || typeof inst.ctx.openApp !== 'function' || null, onClick: () => openInStudio() }, icon('film'), el('span', { text: handingOff ? t('storymaker.handoff.opening') : t('storymaker.handoff.open') })),
                handoffError ? el('code', { class: 'stm-cap-issue', dataset: { stm: 'export-handoff-error' }, text: handoffError }) : null
            ];
        }

        function failedView() {
```

(d) Old:

```js
            if (!isActive()) cancelling = false;
```

New:

```js
            if (!isActive()) cancelling = false;
            if (job.status === 'succeeded' || job.status === 'interrupted') lastJob = job;
            // A wish ends with its job: a job without a film has nothing to hand over, and its ending shows once.
            if (!isActive() && job.status !== 'succeeded') inst.handoffWish.delete(job.id);
            checkHandoff();
```

(e) Old:

```js
        async function cancel() {
```

New:

```js
        // Reads the Video Studio status once per finished job with stems. A wish from the export dialog starts the handoff here, so it
        // needs no further click.
        async function checkHandoff() {
            if (!job || job.status !== 'succeeded' || !job.output || !job.output.stems || vsAskedFor === job.id) return;
            const id = job.id;
            vsAskedFor = id;
            handoffError = '';
            try { vsStatus = await S.handoff.status(); } catch (_) { vsStatus = null; }
            if (!job || job.id !== id) return;
            const wished = inst.handoffWish.delete(id);
            if (wished && S.handoff.canOffer(job, vsStatus)) { openInStudio(); return; }
            if (wished) inst.notify(t('storymaker.handoff.unavailable'), 'error');
            paint();
        }

        async function openInStudio() {
            if (handingOff || !job) return;
            handingOff = true;
            handoffError = '';
            paint();
            try {
                await S.handoff.run(inst, job, handoffProjects);
            } catch (err) {
                handoffError = String(err && err.message || '').slice(0, 400);
                inst.notify(t('storymaker.handoff.failed'), 'error');
            } finally {
                handingOff = false;
                paint();
            }
        }

        // Reads the newest render that left a result, so one that ended while no panel watched it stays reachable. The job list has no
        // cut; the replay of the event stream holds only the latest five jobs of both queues. A wish from this session starts its
        // handoff here, and any other job waits behind the button of the start view.
        async function loadLastJob() {
            const id = inst.projectId;
            let jobs;
            try { jobs = await inst.api.listJobs(id); } catch (_) { return; }
            if (disposed || inst.projectId !== id) return;
            const found = jobs.find(item => item.kind === 'render' && (item.status === 'succeeded' || item.status === 'interrupted'));
            if (!found) return;
            if (!lastJob) lastJob = found;
            if (job) return;
            if (inst.handoffWish.has(found.id)) apply(found, true);
            else if (open) paint();
        }

        async function cancel() {
```

(f) The stream line, as E1f wrote it. A replayed job with a wish from this session is taken up, so the handoff runs when the user comes back.

Old:

```js
            inst.bus.on('job', next => { if (next && next.kind === 'render') apply(next, false); }),
```

New:

```js
            inst.bus.on('job', next => { if (next && next.kind === 'render') apply(next, inst.handoffWish.has(next.id)); }),
```

(g) The start view offers the way back, also when something blocks a new export. `apply` reuses the result and failure views, so paths, extras, warnings, the handoff and the interrupted text all appear.

Old:

```js
            return view;
```

New:

```js
            if (lastJob) view.appendChild(el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-last' }, onClick: () => apply(lastJob, true) }, icon('film'), el('span', { text: t('storymaker.export.last_export') })));
            return view;
```

(h) Read the job list once, after the panel's first paint. This line is the only `return {` of the file.

Old:

```js
        return {
```

New:

```js
        loadLastJob();
        return {
```

(i) E2d's `disposed` guards `apply`. `checkHandoff` waits for the Video Studio status, so a panel that is gone must find no job afterwards, or it would start a handoff.

Old:

```js
                document.removeEventListener('pointerdown', onDocumentPointer, true);
```

New:

```js
                job = null;
                document.removeEventListener('pointerdown', onDocumentPointer, true);
```

(j) The comment on `apply` would be false otherwise.

Old:

```js
        // another window); a finished one is shown when this panel started it (adopt).
```

New:

```js
        // another window); a finished one is shown only on request (adopt): this panel started it, its handoff was wished, or the user chose it.
```

(k) A failed stem says why the action is missing, right under the warning that names the file. E3c wrote this line.

Old:

```js
                warningsView(job),
```

New:

```js
                warningsView(job),
                handoffUnavailable(),
```

Then apply these two replacements in `ui/js/desktop/apps/storymaker-dialog.js`, as E3a leaves it.

(l) The Open in Video Studio option asks for the status when the form opens. The change handler gets a name, so the answer can take back a tick.

Old:

```js
            const handoff = checkbox('handoff', 'storymaker.export.open_in_studio', values.handoff, () => {
                const forced = handoff.input.checked;
                if (forced && !stems.input.disabled) stemsWanted = stems.input.checked;
                stems.input.checked = forced || stemsWanted;
                stems.input.disabled = forced;
                note.hidden = !forced;
            });
```

New:

```js
            const unavailable = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'export-opt-handoff-reason' }, text: t('storymaker.handoff.unavailable') });
            const syncStems = () => {
                const forced = handoff.input.checked;
                if (forced && !stems.input.disabled) stemsWanted = stems.input.checked;
                stems.input.checked = forced || stemsWanted;
                stems.input.disabled = forced;
                note.hidden = !forced;
            };
            const handoff = checkbox('handoff', 'storymaker.export.open_in_studio', values.handoff, syncStems);
            // Asked each time the form opens, so nobody waits for a whole render to learn that Video Studio cannot take the film. A failed
            // read counts as unavailable. The option stays usable until the answer, because the first status read tests the server's
            // encoders and can take seconds; a tick made meanwhile goes back with its forced stems. checkHandoff in storymaker-export.js
            // checks again when the render ends, for a status that changed.
            S.handoff.status().then(S.handoff.studioReady, () => false).then(ready => {
                if (ready) return;
                handoff.input.disabled = true;
                unavailable.hidden = false;
                if (handoff.input.checked) { handoff.input.checked = false; syncStems(); }
            });
```

(m) The reason sits under the option.

Old:

```js
            node.appendChild(note);
```

New:

```js
            node.appendChild(note);
            node.appendChild(unavailable);
```

- [ ] **Step 6: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerHandoffRunInNode|TestStorymakerExportOffersHandoff|TestStorymakerExportKeepsLastJobInNode|TestStorymakerNarrationStatusInNode|TestStorymakerExportStartInNode|TestStorymakerExportFinishedJobInNode|TestStorymakerExportRefusalsInNode|TestStorymakerExportDialogInNode|TestStorymakerHandoffTracksInNode|TestStorymakerSourcesAreSafeAndSmall)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run the graph check. For a PARTIAL answer on the new symbols, a crash, a closed connection, a `not found` answer or a banner-only answer, follow outline section 5 and state the grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-handoff.js ui/js/desktop/apps/storymaker-export.js ui/js/desktop/apps/storymaker-dialog.js ui/desktop_storymaker_handoff_test.go ui/desktop_storymaker_audio_test.go ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): hand a finished job with stems to Video Studio" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3f: `projectId` launch (dirty shell files)
**Batch:** 2D  **Depends on:** none
**Files:**
- Modify: `ui/js/desktop/apps/video-studio.js`
- Modify: `ui/js/desktop/core/window-shell-runtime.js`
- Modify: `ui/js/desktop/core/menus-and-routing.js`
- Test: `ui/desktop_video_studio_open_project_test.go`

- [ ] **Step 1: Impact**

These three app and shell files may hold another session's edits in the main checkout. Edit only the worktree copies, and never commit, stash or restore the other session's edits. Run this from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "initialize" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "markStudioReady" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "openApp" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "renderAppContent" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5. `openApp` and `renderAppContent` are shell-wide, so expect HIGH or above and say so. The edits add one branch that runs only for `video-studio` with a `projectId`.

- [ ] **Step 2: Write the failing test**

Create `ui/desktop_video_studio_open_project_test.go`:

```go
package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The projectId launch of Video Studio in Node. video-studio.js runs against stubbed parts and fake instances; the reuse branch of the
// window shell and the launch merge of the routing are cut from their files and run on their own.
func TestVideoStudioOpenProjectInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "video-studio-open-project.js")
	if err := os.WriteFile(script, []byte(videoStudioOpenProjectScript), 0o600); err != nil {
		t.Fatal(err)
	}
	apps, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	core, _ := filepath.Abs(filepath.Join("js", "desktop", "core"))
	out, err := exec.Command(node, script, apps, core).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "video studio open project ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const videoStudioOpenProjectScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [appsDir, coreDir] = process.argv.slice(2);
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const read = (dir, name) => fs.readFileSync(path.join(dir, name), 'utf8').replace(/\r\n/g, '\n');
const settle = async () => { for (let i = 0; i < 50; i++) await new Promise(resolve => setImmediate(resolve)); };

const API = '/api/desktop/video-studio';
const log = [];
let saveWorks = true;
let holdLoad = null;
const P = {
  API,
  request: async url => {
    if (url === API + '/status') return { body: { enabled: true, desktop_enabled: true, ffmpeg_ready: true }, etag: '' };
    if (url === API + '/projects') return { body: { projects: [{ id: 'first' }, { id: 'wanted' }] }, etag: '' };
    const id = decodeURIComponent(url.slice((API + '/projects/').length));
    log.push('load ' + id);
    if (holdLoad) await holdLoad;
    return { body: { project: { name: id, tracks: [{ id: 'track-1', clips: [] }] } }, etag: 'etag-' + id };
  },
  hydrateProject: (s, project) => project,
  canonicalProject: project => project,
  cancelTextApplies() {},
  recoverDraft: async () => false,
  loadJobs: async () => {},
  closePopovers() {},
  flushArtwork: async () => {},
  saveUntilClean: async s => { log.push('save'); if (saveWorks) s.dirty = false; return saveWorks; },
  clearNotice() {},
  showNotice() {},
  renderAIPanel() {}
};
const context = { console, VideoStudioParts: P, VideoStudioEffects: { adopt() {} }, setInterval: () => 0 };
context.window = context; context.globalThis = context;
vm.createContext(context);
vm.runInContext(read(appsDir, 'video-studio.js'), context, { filename: 'video-studio.js' });
const App = context.VideoStudioApp;

// renderUI and populateCanvas return early without their nodes, so only the popover lookup of loadProjects needs an answer.
function instance(id, fields) {
  const s = Object.assign({
    id, ctx: {}, abort: { signal: null }, disposed: false, project: null, projectId: '', projects: [], dirty: false, readonly: false, studioReady: false,
    app: { classList: { toggle() {} } }, stickerPicker: { setDisabled() {} },
    q: selector => (selector === '[data-project-popover]' ? { hidden: true } : null)
  }, fields || {});
  App.instances.set(id, s);
  return s;
}
const open = { project: { name: 'first', tracks: [{ id: 'track-1', clips: [] }] } };

(async () => {
  const preferred = instance('w1', { ctx: { projectId: 'wanted' } });
  await P.initialize(preferred);
  await settle();
  eq([preferred.projectId, preferred.studioReady], ['wanted', true], 'initialize opens a listed id');

  const gone = instance('w2', { ctx: { projectId: 'gone' } });
  await P.initialize(gone);
  await settle();
  eq(gone.projectId, 'first', 'an id that is not listed falls back to the first project');

  log.length = 0;
  const early = instance('w3');
  await App.openProject('w3', 'wanted');
  eq([early.ctx.projectId, early.projectId, log], ['wanted', '', []], 'before ready the id only waits in the context');

  // s.projects starts empty here, so a switch proves that openProject read the list again.
  const dirty = instance('w4', Object.assign({ studioReady: true, dirty: true, projectId: 'first' }, open));
  await App.openProject('w4', 'wanted');
  eq([log, dirty.projectId], [['save', 'load wanted'], 'wanted'], 'dirty work is saved before the switch');

  log.length = 0;
  saveWorks = false;
  const stays = instance('w5', Object.assign({ studioReady: true, dirty: true, projectId: 'first' }, open));
  await App.openProject('w5', 'wanted');
  saveWorks = true;
  eq([log, stays.projectId], [['save'], 'first'], 'a failed save keeps the open project');

  log.length = 0;
  const unlisted = instance('w6', Object.assign({ studioReady: true, projectId: 'first' }, open));
  await App.openProject('w6', 'gone');
  eq([log, unlisted.projectId], [[], 'first'], 'an id that is not listed changes nothing');

  // An id that arrives while the first load runs opens when that load ends.
  let release = null;
  holdLoad = new Promise(resolve => { release = resolve; });
  const racing = instance('w7');
  const first = P.initialize(racing);
  await settle();
  await App.openProject('w7', 'wanted');
  holdLoad = null;
  release();
  await first;
  await settle();
  eq(racing.projectId, 'wanted', 'an id stored during the first load opens afterwards');

  // The reuse branch of the window shell.
  const shell = read(coreDir, 'window-shell-runtime.js');
  const mark = "if (appId === 'video-studio' && context && !context.sessionRestore && typeof context.projectId === 'string' && context.projectId) {";
  const start = shell.indexOf(mark);
  if (start < 0) throw new Error('window-shell-runtime.js has no projectId branch in the reuse path');
  const closing = '\n            }';
  const reuse = new Function('appId', 'context', 'existing', 'window', shell.slice(start, shell.indexOf(closing + '\n', start) + closing.length));
  const opened = [];
  const live = { VideoStudioApp: { instances: new Map([['win', {}]]), openProject: (id, projectId) => opened.push([id, projectId]) } };
  const win = { id: 'win' };
  reuse('video-studio', { projectId: 'p1' }, win, live);
  eq([opened, 'pendingProjectId' in win], [[['win', 'p1']], false], 'a live window opens the project');
  const lazy = { id: 'lazy' };
  reuse('video-studio', { projectId: 'p2' }, lazy, {});
  eq(lazy.pendingProjectId, 'p2', 'a window that still loads keeps the id');
  const restored = { id: 'win' };
  reuse('video-studio', { projectId: 'p3', sessionRestore: {} }, restored, live);
  reuse('video-studio', { projectId: '' }, restored, live);
  eq([opened.length, 'pendingProjectId' in restored], [1, false], 'a session restore and an empty id do nothing');

  // The launch merge of the routing.
  const routing = read(coreDir, 'menus-and-routing.js');
  const from = routing.indexOf('const launch = Object.assign({}, context || {}, win?.pendingCameraImport');
  const endMark = 'delete win.pendingProjectId; }';
  const to = routing.indexOf(endMark, from);
  if (from < 0 || to < 0) throw new Error('menus-and-routing.js does not merge pendingProjectId');
  const merge = new Function('context', 'win', routing.slice(from, to + endMark.length) + '\nreturn launch;');
  const pending = { pendingProjectId: 'p4', pendingCameraImport: { importPath: 'Videos/a.webm' } };
  eq(merge({ a: 1 }, pending), { a: 1, importPath: 'Videos/a.webm', projectId: 'p4' }, 'launch takes the pending id');
  eq(Object.keys(pending), [], 'the pending values are used once');
  eq(merge({ a: 1 }, {}), { a: 1 }, 'no pending id adds nothing');

  console.log('video studio open project ok');
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestVideoStudioOpenProjectInNode$' -count=1
```

Expected: FAIL with `initialize opens a listed id: got ["first",true], want ["wanted",true]`, because `initialize` always opens `projects[0]`.

- [ ] **Step 4: Implement**

Every old text below is one line and occurs once in its file. These files use CRLF in this checkout, so do not match across lines.

In `ui/js/desktop/apps/video-studio.js`:

(a) Old:

```js
        if (P.beginCameraImport) P.beginCameraImport(s);
```

New:

```js
        if (P.beginCameraImport) P.beginCameraImport(s);
        // An id stored while the first load ran (openProject) opens now.
        if (s.ctx.projectId && s.ctx.projectId !== s.projectId) openProject(s.id, s.ctx.projectId);
```

(b) Old:

```js
        if (!s.project && s.projects.length) await loadProject(s, s.projects[0].id);
```

New:

```js
        if (!s.project && s.projects.length) {
            // A launch may name the project. One that is gone falls back to the first.
            const named = s.ctx.projectId && s.projects.some(item => item.id === s.ctx.projectId);
            await loadProject(s, named ? s.ctx.projectId : s.projects[0].id);
        }
```

(c) Old:

```js
    function receiveImport(id, context) {
```

New:

```js
    // Opens a project in a live window (the Storymaker handoff). Before the first load ends, the id waits in the context. Afterwards the
    // list is read again, because the project may be new, and switchProject saves dirty work first.
    async function openProject(id, projectId) {
        const s = instances.get(id);
        if (!s || s.disposed || s.featureDisabled || !projectId) return;
        if (!s.studioReady) { s.ctx.projectId = projectId; return; }
        try {
            await loadProjects(s);
            if (!s.disposed && s.projects.some(item => item.id === projectId)) await switchProject(s, projectId);
        } catch (_) { showNotice(s, 'loadFailed', 'Could not open this project.', true); }
    }

    function receiveImport(id, context) {
```

(d) Old:

```js
    window.VideoStudioApp = { render: appRender, dispose, instances, capturePreview, receiveImport };
```

New:

```js
    window.VideoStudioApp = { render: appRender, dispose, instances, capturePreview, receiveImport, openProject };
```

In `ui/js/desktop/core/window-shell-runtime.js`, old (the line right after the camera-import branch of the reuse path):

```js
            if (context && context.path) recordRecentFile(context.path, appId, context.pathKind);
```

New:

```js
            if (appId === 'video-studio' && context && !context.sessionRestore && typeof context.projectId === 'string' && context.projectId) {
                if (window.VideoStudioApp?.instances?.has(existing.id)) window.VideoStudioApp.openProject(existing.id, context.projectId);
                else existing.pendingProjectId = context.projectId;
            }
            if (context && context.path) recordRecentFile(context.path, appId, context.pathKind);
```

In `ui/js/desktop/core/menus-and-routing.js`:

(a) Old:

```js
            const launch = Object.assign({}, context || {}, win?.pendingCameraImport || {});
```

New:

```js
            const launch = Object.assign({}, context || {}, win?.pendingCameraImport || {}, win?.pendingProjectId ? { projectId: win.pendingProjectId } : {});
```

(b) Old:

```js
            if (win) delete win.pendingCameraImport;
```

New:

```js
            if (win) { delete win.pendingCameraImport; delete win.pendingProjectId; }
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(videostudio|camera)' -count=1 -timeout 20m
npm run build:ui
npm run check:ui
git status --short -- ui/js/desktop/bundles/main.bundle.js
```

Expected:
- `go test` passes.
- `npm run check:ui` passes on the rebuilt copy.
- `git status` shows `main.bundle.js` as modified. Leave it rebuilt and uncommitted, because I2 needs it and the last commit of the batch holds it (outline section 2, merge handling).

- [ ] **Step 6: Commit**

Run the graph check. For a PARTIAL answer on the new symbols, a crash, a closed connection, a `not found` answer or a banner-only answer, follow outline section 5 and state the grep confirmation in the commit body. Do not stage `main.bundle.js`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/video-studio.js ui/js/desktop/core/window-shell-runtime.js ui/js/desktop/core/menus-and-routing.js ui/desktop_video_studio_open_project_test.go
git commit -m "feat(videostudio): open a named project through the projectId launch" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3g: 2D strings
**Batch:** 2D  **Depends on:** E3a, E3b, E3c, E3d, E3e, E3f
**Files:**
- Modify: `ui/desktop_storymaker_i18n_test.go`
- Modify: `ui/lang/storymaker/cs.json`
- Modify: `ui/lang/storymaker/da.json`
- Modify: `ui/lang/storymaker/de.json`
- Modify: `ui/lang/storymaker/el.json`
- Modify: `ui/lang/storymaker/en.json`
- Modify: `ui/lang/storymaker/es.json`
- Modify: `ui/lang/storymaker/fr.json`
- Modify: `ui/lang/storymaker/hi.json`
- Modify: `ui/lang/storymaker/it.json`
- Modify: `ui/lang/storymaker/ja.json`
- Modify: `ui/lang/storymaker/nl.json`
- Modify: `ui/lang/storymaker/no.json`
- Modify: `ui/lang/storymaker/pl.json`
- Modify: `ui/lang/storymaker/pt.json`
- Modify: `ui/lang/storymaker/sv.json`
- Modify: `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Impact**

Run this from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerAppTranslationsAreTranslated" --direction upstream --repo .
```

A test has no callers, so expect UNKNOWN or LOW. Confirm with the text search that outline section 5 asks for.

- [ ] **Step 2: Write the failing test**

SRT and VTT are the same word in every language, so their labels join the exact list. In `ui/desktop_storymaker_i18n_test.go`, replace this line:

```go
	same := map[string]bool{
```

with:

```go
	same := map[string]bool{
		"storymaker.export.label_srt": true, "storymaker.export.label_vtt": true,
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1
```

Expected: FAIL. `TestStorymakerAppUsesOnlyKeysItHas` reports `js/desktop/apps/storymaker-export.js uses storymaker.handoff.open, which lang/storymaker/en.json lacks`, and the same for the other new keys, because no locale has them yet.

- [ ] **Step 4: Write the merge script**

Write this with the Write tool to `e3g-strings.mjs` in your session scratchpad. It inserts text lines after the `storymaker.export.again` line, so the groups and blank lines of each file stay as they are. Each row has its 16 texts in the order of `LANGS`.

```js
// Adds the 2D app strings to ui/lang/storymaker/*.json. Run from the worktree root. --check only compares the table with the code.
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const LANGS = ['en', 'cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const same = text => LANGS.map(() => text);

const ROWS = {
  'storymaker.handoff.open': ["Open in Video Studio", "Otevřít ve Video Studiu", "Åbn i Video Studio", "In Video Studio öffnen", "Άνοιγμα στο Video Studio", "Abrir en Video Studio", "Ouvrir dans Video Studio", "Video Studio में खोलें", "Apri in Video Studio", "Video Studio で開く", "Openen in Video Studio", "Åpne i Video Studio", "Otwórz w Video Studio", "Abrir no Video Studio", "Öppna i Video Studio", "在 Video Studio 中打开"],
  'storymaker.handoff.opening': ["Opening in Video Studio…", "Otevírání ve Video Studiu…", "Åbner i Video Studio…", "Wird in Video Studio geöffnet…", "Γίνεται άνοιγμα στο Video Studio…", "Abriendo en Video Studio…", "Ouverture dans Video Studio…", "Video Studio में खुल रहा है…", "Apertura in Video Studio…", "Video Studio で開いています…", "Bezig met openen in Video Studio…", "Åpner i Video Studio…", "Otwieranie w Video Studio…", "A abrir no Video Studio…", "Öppnar i Video Studio…", "正在 Video Studio 中打开…"],
  'storymaker.handoff.failed': ["The video could not be opened in Video Studio.", "Video se nepodařilo otevřít ve Video Studiu.", "Videoen kunne ikke åbnes i Video Studio.", "Das Video konnte nicht in Video Studio geöffnet werden.", "Το βίντεο δεν ήταν δυνατό να ανοίξει στο Video Studio.", "El vídeo no se pudo abrir en Video Studio.", "La vidéo n'a pas pu être ouverte dans Video Studio.", "वीडियो Video Studio में नहीं खुल सका।", "Non è stato possibile aprire il video in Video Studio.", "動画を Video Studio で開けませんでした。", "De video kon niet worden geopend in Video Studio.", "Videoen kunne ikke åpnes i Video Studio.", "Nie udało się otworzyć filmu w Video Studio.", "Não foi possível abrir o vídeo no Video Studio.", "Filmen kunde inte öppnas i Video Studio.", "无法在 Video Studio 中打开该视频。"],
  'storymaker.handoff.unavailable': ["Video Studio is not available, so the video stays in your project folder.", "Video Studio není k dispozici, takže video zůstává ve složce tvého projektu.", "Video Studio er ikke tilgængelig, så videoen bliver i din projektmappe.", "Video Studio ist nicht verfügbar. Das Video bleibt in deinem Projektordner.", "Το Video Studio δεν είναι διαθέσιμο, οπότε το βίντεο παραμένει στον φάκελο του έργου σας.", "Video Studio no está disponible, así que el vídeo se queda en la carpeta de tu proyecto.", "Video Studio n'est pas disponible : la vidéo reste dans le dossier de ton projet.", "Video Studio उपलब्ध नहीं है, इसलिए वीडियो आपके प्रोजेक्ट फ़ोल्डर में ही रहेगा।", "Video Studio non è disponibile, quindi il video resta nella cartella del tuo progetto.", "Video Studio を利用できないため、動画はプロジェクトフォルダに残ります。", "Video Studio is niet beschikbaar, dus de video blijft in je projectmap.", "Video Studio er ikke tilgjengelig, så videoen blir liggende i prosjektmappen din.", "Video Studio jest niedostępne, więc film pozostaje w folderze Twojego projektu.", "O Video Studio não está disponível, por isso o vídeo fica na pasta do teu projeto.", "Video Studio är inte tillgängligt, så filmen ligger kvar i din projektmapp.", "Video Studio 不可用，因此视频保留在你的项目文件夹中。"],
  'storymaker.handoff.timeout': ["The media import did not finish in time.", "Import médií nebyl dokončen včas.", "Medieimporten blev ikke færdig i tide.", "Der Medienimport wurde nicht rechtzeitig fertig.", "Η εισαγωγή πολυμέσων δεν ολοκληρώθηκε εγκαίρως.", "La importación de medios no terminó a tiempo.", "L'import des médias ne s'est pas terminé à temps.", "मीडिया इंपोर्ट समय पर पूरा नहीं हुआ।", "L'importazione dei media non è terminata in tempo.", "メディアの取り込みが時間内に終わりませんでした。", "De media-import werd niet op tijd voltooid.", "Medieimporten ble ikke ferdig i tide.", "Import multimediów nie zakończył się na czas.", "A importação de multimédia não terminou a tempo.", "Medieimporten blev inte klar i tid.", "媒体导入未能及时完成。"],
  'storymaker.handoff.import_failed': ["Video Studio could not read one of the files.", "Video Studio nedokázalo přečíst jeden ze souborů.", "Video Studio kunne ikke læse en af filerne.", "Video Studio konnte eine der Dateien nicht lesen.", "Το Video Studio δεν μπόρεσε να διαβάσει ένα από τα αρχεία.", "Video Studio no pudo leer uno de los archivos.", "Video Studio n'a pas pu lire l'un des fichiers.", "Video Studio एक फ़ाइल नहीं पढ़ सका।", "Video Studio non è riuscito a leggere uno dei file.", "Video Studio がファイルの 1 つを読み取れませんでした。", "Video Studio kon een van de bestanden niet lezen.", "Video Studio kunne ikke lese en av filene.", "Video Studio nie mogło odczytać jednego z plików.", "O Video Studio não conseguiu ler um dos ficheiros.", "Video Studio kunde inte läsa en av filerna.", "Video Studio 无法读取其中一个文件。"],
  'storymaker.handoff.track_video': ["Video track", "Videostopa", "Videospor", "Videospur", "Κανάλι βίντεο", "Pista de vídeo", "Piste vidéo", "वीडियो ट्रैक", "Traccia video", "ビデオトラック", "Videospoor", "Videospor", "Ścieżka wideo", "Faixa de vídeo", "Videospår", "视频轨道"],
  'storymaker.handoff.track_narration': ["Narration track", "Stopa komentáře", "Fortællerspor", "Sprecherspur", "Κανάλι αφήγησης", "Pista de narración", "Piste de commentaire", "वर्णन ट्रैक", "Traccia della narrazione", "ナレーショントラック", "Voice-overspoor", "Fortellerspor", "Ścieżka lektora", "Faixa de narração", "Berättarspår", "旁白轨道"],
  'storymaker.handoff.track_music': ["Music track", "Hudební stopa", "Musikspor", "Musikspur", "Κανάλι μουσικής", "Pista de música", "Piste musicale", "संगीत ट्रैक", "Traccia musicale", "音楽トラック", "Muziekspoor", "Musikkspor", "Ścieżka muzyczna", "Faixa de música", "Musikspår", "音乐轨道"],
  'storymaker.handoff.track_sfx': ["Sound effects track", "Stopa zvukových efektů", "Lydeffektspor", "Spur für Soundeffekte", "Κανάλι ηχητικών εφέ", "Pista de efectos de sonido", "Piste d'effets sonores", "साउंड इफ़ेक्ट ट्रैक", "Traccia degli effetti sonori", "効果音トラック", "Spoor voor geluidseffecten", "Lydeffektspor", "Ścieżka efektów dźwiękowych", "Faixa de efeitos sonoros", "Ljudeffektspår", "音效轨道"],

  'storymaker.export.open_options': ["Export options…", "Možnosti exportu…", "Eksportindstillinger…", "Exportoptionen…", "Επιλογές εξαγωγής…", "Opciones de exportación…", "Options d'export…", "एक्सपोर्ट विकल्प…", "Opzioni di esportazione…", "書き出しのオプション…", "Exportopties…", "Eksportalternativer…", "Opcje eksportu…", "Opções de exportação…", "Exportalternativ…", "导出选项…"],
  'storymaker.export.dialog_title': ["Export options", "Možnosti exportu", "Eksportindstillinger", "Exportoptionen", "Επιλογές εξαγωγής", "Opciones de exportación", "Options d'export", "एक्सपोर्ट विकल्प", "Opzioni di esportazione", "書き出しのオプション", "Exportopties", "Eksportalternativer", "Opcje eksportu", "Opções de exportação", "Exportalternativ", "导出选项"],
  'storymaker.export.quality': ["Quality", "Kvalita", "Kvalitet", "Qualität", "Ποιότητα", "Calidad", "Qualité", "गुणवत्ता", "Qualità", "画質", "Kwaliteit", "Kvalitet", "Jakość", "Qualidade", "Kvalitet", "画质"],
  'storymaker.export.quality_standard': ["Standard", "Standardní kvalita", "Standardkvalitet", "Standardqualität", "Τυπική ποιότητα", "Calidad estándar", "Qualité standard", "मानक गुणवत्ता", "Qualità standard", "標準画質", "Standaardkwaliteit", "Standardkvalitet", "Jakość standardowa", "Qualidade padrão", "Standardkvalitet", "标准画质"],
  'storymaker.export.quality_high': ["High", "Vysoká kvalita", "Høj kvalitet", "Hohe Qualität", "Υψηλή ποιότητα", "Alta calidad", "Haute qualité", "उच्च गुणवत्ता", "Alta qualità", "高画質", "Hoge kwaliteit", "Høy kvalitet", "Wysoka jakość", "Alta qualidade", "Hög kvalitet", "高画质"],
  'storymaker.export.burn_in': ["Burn captions into the picture", "Vypálit titulky do obrazu", "Brænd undertekster ind i billedet", "Untertitel ins Bild einbrennen", "Ενσωμάτωση υποτίτλων στην εικόνα", "Incrustar subtítulos en la imagen", "Incruster les sous-titres dans l'image", "कैप्शन को चित्र में जोड़ें", "Incorpora i sottotitoli nell'immagine", "字幕を映像に焼き込む", "Ondertitels in het beeld branden", "Brenn undertekster inn i bildet", "Wtop napisy w obraz", "Incorporar legendas na imagem", "Bränn in undertexter i bilden", "将字幕烧录到画面中"],
  'storymaker.export.subtitles': ["Subtitle files", "Soubory titulků", "Undertekstfiler", "Untertiteldateien", "Αρχεία υποτίτλων", "Archivos de subtítulos", "Fichiers de sous-titres", "उपशीर्षक फ़ाइलें", "File di sottotitoli", "字幕ファイル", "Ondertitelbestanden", "Undertekstfiler", "Pliki napisów", "Ficheiros de legendas", "Undertextfiler", "字幕文件"],
  'storymaker.export.label_srt': same("SRT"),
  'storymaker.export.label_vtt': same("VTT"),
  'storymaker.export.stems': ["Separate audio tracks", "Samostatné zvukové stopy", "Separate lydspor", "Getrennte Tonspuren", "Ξεχωριστά κανάλια ήχου", "Pistas de audio separadas", "Pistes audio séparées", "अलग ऑडियो ट्रैक", "Tracce audio separate", "個別の音声トラック", "Aparte audiosporen", "Separate lydspor", "Oddzielne ścieżki dźwiękowe", "Faixas de áudio separadas", "Separata ljudspår", "独立的音轨"],
  'storymaker.export.stems_forced': ["Video Studio needs the separate audio tracks.", "Video Studio potřebuje samostatné zvukové stopy.", "Video Studio skal bruge de separate lydspor.", "Video Studio braucht die getrennten Tonspuren.", "Το Video Studio χρειάζεται τα ξεχωριστά κανάλια ήχου.", "Video Studio necesita las pistas de audio separadas.", "Video Studio a besoin des pistes audio séparées.", "Video Studio को अलग ऑडियो ट्रैक चाहिए।", "Video Studio ha bisogno delle tracce audio separate.", "Video Studio には個別の音声トラックが必要です。", "Video Studio heeft de aparte audiosporen nodig.", "Video Studio trenger de separate lydsporene.", "Video Studio potrzebuje oddzielnych ścieżek dźwiękowych.", "O Video Studio precisa das faixas de áudio separadas.", "Video Studio behöver de separata ljudspåren.", "Video Studio 需要独立的音轨。"],
  'storymaker.export.open_in_studio': ["Open in Video Studio", "Otevřít ve Video Studiu", "Åbn i Video Studio", "In Video Studio öffnen", "Άνοιγμα στο Video Studio", "Abrir en Video Studio", "Ouvrir dans Video Studio", "Video Studio में खोलें", "Apri in Video Studio", "Video Studio で開く", "Openen in Video Studio", "Åpne i Video Studio", "Otwórz w Video Studio", "Abrir no Video Studio", "Öppna i Video Studio", "在 Video Studio 中打开"],
  'storymaker.export.dialog_cancel': ["Cancel", "Zrušit", "Annuller", "Abbrechen", "Ακύρωση", "Cancelar", "Annuler", "रद्द करें", "Annulla", "キャンセル", "Annuleren", "Avbryt", "Anuluj", "Cancelar", "Avbryt", "取消"],

  'storymaker.export.blocked_narration': ["Some narration has no audio yet. Generate it in the voice panel, or mark the scene as silent on the Scene tab.", "Část komentáře ještě nemá zvuk. Vygeneruj ho v panelu hlasu, nebo scénu na kartě Scéna označ jako němou.", "Noget af fortællerteksten har endnu ingen lyd. Generér den i stemmepanelet, eller markér scenen som tavs på fanen Scene.", "Ein Teil des Sprechtexts hat noch keinen Ton. Erzeuge ihn im Stimmen-Bereich, oder markiere die Szene im Tab Szene als stumm.", "Μέρος της αφήγησης δεν έχει ακόμη ήχο. Δημιουργήστε τον στον πίνακα φωνής ή σημειώστε τη σκηνή ως βουβή στην καρτέλα Σκηνή.", "Parte de la narración aún no tiene audio. Genéralo en el panel de voz o marca la escena como muda en la pestaña Escena.", "Une partie du commentaire n'a pas encore d'audio. Génère-le dans le panneau des voix, ou marque la scène comme muette dans l'onglet Scène.", "कुछ वर्णन का ऑडियो अभी नहीं है। इसे आवाज़ पैनल में बनाएँ, या दृश्य टैब में दृश्य को मौन चिह्नित करें।", "Una parte della narrazione non ha ancora l'audio. Generalo nel pannello voce, oppure segna la scena come muta nella scheda Scena.", "音声がまだないナレーションがあります。音声パネルで生成するか、シーンタブでシーンを無音に設定してください。", "Een deel van de voice-over heeft nog geen audio. Genereer die in het stempaneel, of markeer de scène als stil op het tabblad Scène.", "Noe av fortellerteksten har ikke lyd ennå. Generer den i stemmepanelet, eller merk scenen som stille på fanen Scene.", "Część tekstu lektora nie ma jeszcze dźwięku. Wygeneruj go w panelu głosu albo oznacz scenę jako bez głosu na karcie Scena.", "Parte da narração ainda não tem áudio. Gera-o no painel de voz ou marca a cena como muda no separador Cena.", "En del av berättarrösten saknar ännu ljud. Skapa det i röstpanelen eller markera scenen som tyst på fliken Scen.", "部分旁白还没有音频。请在语音面板中生成，或在“场景”选项卡中将该场景设为静音。"],
  'storymaker.export.blocked_music': ["The music is not ready yet. Prepare it in the music panel.", "Hudba ještě není připravená. Připrav ji v panelu hudby.", "Musikken er ikke klar endnu. Forbered den i musikpanelet.", "Die Musik ist noch nicht bereit. Bereite sie im Musik-Bereich vor.", "Η μουσική δεν είναι ακόμη έτοιμη. Προετοιμάστε την στον πίνακα μουσικής.", "La música aún no está lista. Prepárala en el panel de música.", "La musique n'est pas encore prête. Prépare-la dans le panneau de la musique.", "संगीत अभी तैयार नहीं है। इसे संगीत पैनल में तैयार करें।", "La musica non è ancora pronta. Preparala nel pannello musica.", "音楽はまだ準備できていません。音楽パネルで準備してください。", "De muziek is nog niet klaar. Bereid die voor in het muziekpaneel.", "Musikken er ikke klar ennå. Forbered den i musikkpanelet.", "Muzyka nie jest jeszcze gotowa. Przygotuj ją w panelu muzyki.", "A música ainda não está pronta. Prepara-a no painel de música.", "Musiken är inte klar än. Förbered den i musikpanelen.", "音乐尚未准备好。请在音乐面板中准备。"],
  'storymaker.export.blocked_music_source': ["The music source is gone. Choose another one in the music panel.", "Zdroj hudby chybí. Vyber jiný v panelu hudby.", "Musikkilden mangler. Vælg en anden i musikpanelet.", "Die Musikquelle fehlt. Wähle im Musik-Bereich eine andere.", "Η πηγή της μουσικής λείπει. Επιλέξτε άλλη στον πίνακα μουσικής.", "Falta la fuente de la música. Elige otra en el panel de música.", "La source de la musique a disparu. Choisis-en une autre dans le panneau de la musique.", "संगीत का स्रोत नहीं मिला। संगीत पैनल में कोई दूसरा चुनें।", "La fonte della musica non c'è più. Scegline un'altra nel pannello musica.", "音楽のソースがありません。音楽パネルで別のソースを選んでください。", "De muziekbron ontbreekt. Kies een andere in het muziekpaneel.", "Musikkkilden mangler. Velg en annen i musikkpanelet.", "Brak źródła muzyki. Wybierz inne w panelu muzyki.", "A fonte da música desapareceu. Escolhe outra no painel de música.", "Musikkällan saknas. Välj en annan i musikpanelen.", "音乐来源已丢失。请在音乐面板中选择其他来源。"],
  'storymaker.export.label_narration': ["Narration", "Komentář", "Fortællerlyd", "Sprachausgabe", "Αφήγηση", "Narración", "Commentaire", "वर्णन", "Narrazione", "ナレーション", "Voice-over", "Fortellerlyd", "Lektor", "Narração", "Berättarröst", "旁白"],
  'storymaker.export.label_music': ["Music", "Hudba", "Musik", "Musik", "Μουσική", "Música", "Musique", "संगीत", "Musica", "音楽", "Muziek", "Musikk", "Muzyka", "Música", "Musik", "音乐"],
  'storymaker.export.label_sfx': ["Sound effects", "Zvukové efekty", "Lydeffekter", "Soundeffekte", "Ηχητικά εφέ", "Efectos de sonido", "Effets sonores", "साउंड इफ़ेक्ट", "Effetti sonori", "効果音", "Geluidseffecten", "Lydeffekter", "Efekty dźwiękowe", "Efeitos sonoros", "Ljudeffekter", "音效"],
  'storymaker.export.label_library': ["Copy in Videos", "Kopie ve Videích", "Kopi i Videoer", "Kopie in Videos", "Αντίγραφο στα Βίντεο", "Copia en Vídeos", "Copie dans Vidéos", "वीडियो में प्रति", "Copia in Video", "ビデオ内のコピー", "Kopie in Video's", "Kopi i Videoer", "Kopia w Wideo", "Cópia em Vídeos", "Kopia i Videor", "视频中的副本"],
  'storymaker.export.last_export': ["Show the last export", "Zobrazit poslední export", "Vis den seneste eksport", "Letzten Export anzeigen", "Εμφάνιση της τελευταίας εξαγωγής", "Mostrar la última exportación", "Afficher le dernier export", "पिछला एक्सपोर्ट दिखाएँ", "Mostra l'ultima esportazione", "前回の書き出しを表示", "Laatste export tonen", "Vis siste eksport", "Pokaż ostatni eksport", "Mostrar a última exportação", "Visa senaste exporten", "显示上次导出"],
  'storymaker.export.warnings': ["Finished with warnings:", "Dokončeno s upozorněními:", "Færdig med advarsler:", "Mit Warnungen abgeschlossen:", "Ολοκληρώθηκε με προειδοποιήσεις:", "Terminado con advertencias:", "Terminé avec des avertissements :", "चेतावनियों के साथ पूरा हुआ:", "Completato con avvisi:", "警告ありで完了しました：", "Voltooid met waarschuwingen:", "Fullført med advarsler:", "Ukończono z ostrzeżeniami:", "Concluído com avisos:", "Klar med varningar:", "已完成，但有警告："]
};

for (const [key, row] of Object.entries(ROWS)) {
  if (row.length !== LANGS.length) throw new Error(key + ' has ' + row.length + ' texts, want ' + LANGS.length);
}

const langDir = path.join('ui', 'lang', 'storymaker');
const appDir = path.join('ui', 'js', 'desktop', 'apps');
const english = JSON.parse(readFileSync(path.join(langDir, 'en.json'), 'utf8'));
const literals = new Set();
for (const name of readdirSync(appDir)) {
  if (!/^storymaker.*\.js$/.test(name)) continue;
  for (const match of readFileSync(path.join(appDir, name), 'utf8').matchAll(/'(storymaker\.[A-Za-z0-9_.]+)'/g)) literals.add(match[1]);
}
// A literal that ends in . or _ is the prefix of a family built from a list, as in TestStorymakerAppUsesOnlyKeysItHas.
const families = [...literals].filter(key => key.endsWith('.') || key.endsWith('_'));
const missing = [...literals].filter(key => !families.includes(key) && !(key in english) && !(key in ROWS));
const unused = Object.keys(ROWS).filter(key => !literals.has(key) && !families.some(prefix => key.startsWith(prefix)) && !(key in english));
if (missing.length || unused.length) {
  console.log('used by the code, missing from the table:', missing);
  console.log('in the table, not used by the code:', unused);
  process.exit(1);
}
if (process.argv.includes('--check')) {
  console.log('table and code agree');
  process.exit(0);
}

LANGS.forEach((lang, index) => {
  const file = path.join(langDir, lang + '.json');
  const text = readFileSync(file, 'utf8');
  const values = JSON.parse(text);
  const lines = text.split('\n');
  const at = lines.findIndex(line => line.startsWith('  "storymaker.export.again":'));
  if (at < 0) throw new Error(file + ' has no storymaker.export.again line');
  const added = Object.entries(ROWS).filter(([key]) => !(key in values)).map(([key, row]) => '  ' + JSON.stringify(key) + ': ' + JSON.stringify(row[index]) + ',');
  lines.splice(at + 1, 0, ...added);
  const result = lines.join('\n');
  JSON.parse(result);
  writeFileSync(file, result, 'utf8');
});
console.log('added', Object.keys(ROWS).length, 'keys to', LANGS.length, 'locales');
```

- [ ] **Step 5: Compare the table with the code**

Run this from the worktree root, with `<scratchpad>` as your session scratchpad:

```powershell
node <scratchpad>/e3g-strings.mjs --check
```

Expected: `table and code agree`. The table holds exactly the `storymaker.export.*` keys that E3a to E3c use, `storymaker.export.last_export` from E3e and the `storymaker.handoff.*` keys that E3e uses. The same check runs before any write in Step 6, so a key with no row or a row no code uses stops the script with exit code 1 and changes no file. Stop and report any names it prints.

- [ ] **Step 6: Apply and run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
node <scratchpad>/e3g-strings.mjs
go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1
go test ./ui -run Storymaker -count=1 -timeout 20m
```

Expected: both `go test` lines pass. A `keeps the English text` failure names one row and language: translate that text in the row and edit that line of the locale file.

- [ ] **Step 7: Commit**

Run the graph check. For a PARTIAL answer, a crash, a closed connection, a `not found` answer or a banner-only answer, follow outline section 5. Do not stage `main.bundle.js`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/desktop_storymaker_i18n_test.go ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "feat(storymaker): strings for the export dialog and the handoff in 16 languages" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

