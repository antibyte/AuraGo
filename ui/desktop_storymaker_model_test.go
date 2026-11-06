package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/storymaker/render"
)

// storymakerCode removes the comments of a script, so that a guard sees only what runs.
func storymakerCode(source string) string {
	blocks := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	return regexp.MustCompile(`(?m)(^|\s)//[^\n]*$`).ReplaceAllString(blocks, "")
}

// The iframe of the player: sandbox allow-scripts only, no permission delegation, messages only from its own window, a fixed
// schema with caps, posts to '*' because the origin is opaque, and no requests of its own.
func TestStorymakerPlayerBridgeIsSandboxed(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-player.js")
	for _, marker := range []string{
		"sandbox: 'allow-scripts'",
		"iframe.setAttribute('sandbox', 'allow-scripts');",
		"referrerpolicy: 'no-referrer'",
		"event.source !== iframe.contentWindow",
		"target.postMessage(Object.assign({ sm: 1 }, message), '*', transfer || []);",
		"const msg = checkIncoming(event.data);",
		"msg.sm !== 1",
		"LIMITS.keys",
		"LIMITS.scenes",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("storymaker-player.js lacks %q", marker)
		}
	}
	code := storymakerCode(source)
	for _, forbidden := range []string{"allow-same-origin", "allow-forms", "allow-modals", "allow-popups", "allow-top-navigation", "'allow',", "allow:"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("storymaker-player.js must not contain %q", forbidden)
		}
	}
	if strings.Contains(code, "fetch(") || strings.Contains(code, "XMLHttpRequest") {
		t.Error("the bridge must not make requests itself: the parent serves the assets through the resolver")
	}
}

// The pure part of the app in Node: pointer walking in JSON text, issue lists, the catalog index, story edits and the message schema.
func TestStorymakerAppModelInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-app-model.js")
	if err := os.WriteFile(script, []byte(storymakerModelScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	stories, _ := filepath.Abs(filepath.Join("..", "internal", "storymaker", "testdata", "stories", "firewall.json"))
	catalog, _ := filepath.Abs(filepath.Join("img", "storymaker", "catalog.json"))
	out, err := exec.Command(node, script, dir, stories, catalog).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker app model ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// The export panel in Node, on a small fake DOM: a refused start shows what is missing and leads to the panel that fixes it, and the
// silent note appears only for a film that really has no sound.
func TestStorymakerExportRefusalsInNode(t *testing.T) {
	t.Parallel()
	if strings.Contains(readDesktopAssetText(t, "js/desktop/apps/storymaker-export.js"), "silent_note") {
		t.Error("storymaker-export.js still names storymaker.export.silent_note")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-export-refusals.js")
	if err := os.WriteFile(script, []byte(storymakerExportRefusalsScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker export refusals ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportRefusalsScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = String(tag); this.children = []; this.parent = null; this.dataset = {}; this.attrs = {}; this.listeners = {}; this.own = ''; this.value = ''; this.hidden = false; this.disabled = false; this.className = ''; this.style = { setProperty() {} }; }
  appendChild(child) { child.parent = this; this.children.push(child); return child; }
  removeChild(child) { this.children = this.children.filter(item => item !== child); child.parent = null; return child; }
  get firstChild() { return this.children[0] || null; }
  get textContent() { return this.own + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this.children = []; this.own = String(text); }
  setAttribute(name, value) { this.attrs[name] = String(value); if (name === 'value') this.value = String(value); if (name === 'hidden') this.hidden = true; if (name === 'disabled') this.disabled = true; }
  removeAttribute(name) { delete this.attrs[name]; }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener() {}
  fire(type) { (this.listeners[type] || []).forEach(fn => fn({ target: this })); }
  click() { if (!this.disabled) this.fire('click'); }
  focus() {}
  querySelector() { return null; }
  showModal() { this.open = true; }
  close() { this.open = false; }
  remove() { if (this.parent) this.parent.removeChild(this); }
  contains() { return true; }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.own = String(text); return node; },
  addEventListener() {},
  removeEventListener() {},
  documentElement: { lang: 'en' },
  activeElement: null
};
const context = { console, document, setTimeout, clearTimeout };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-handoff.js', 'storymaker-export.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const find = (node, stm) => { if (node.dataset.stm === stm) return node; for (const child of node.children) { const hit = find(child, stm); if (hit) return hit; } return null; };
const flush = () => new Promise(resolve => setImmediate(resolve));
const t = (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key);
const quietAnswer = () => ({ narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null });

function makeInst(refusal, overrides) {
  const inst = Object.assign({
    t,
    bus: S.createBus(),
    status: { ffmpeg: { ready: true }, browser: { ready: true, source: 'system' } },
    readonly: false,
    projectId: 'p1',
    etag: '"e"',
    story: { format: { aspect: '16:9' }, scenes: [{ id: 's1' }] },
    report: { frame_count: 300, fps: 30, duration_ms: 10000, scenes: [{ id: 's1', start_frame: 0, end_frame: 300 }], audio_events: [], issues: [] },
    audioInfo: quietAnswer(),
    issues: [],
    ctx: {},
    notes: [],
    shown: [],
    domId: name => 'test-' + name,
    flushEdits: () => Promise.resolve(true),
    showIssues() {},
    notify(text) { inst.notes.push(text); },
    showPanel(name) { inst.shown.push(name); },
    api: { startRender() { return Promise.reject(refusal); } }
  }, overrides);
  return inst;
}

(async () => {
  // narration_incomplete names the missing scenes and characters and leads to the voice panel
  let refusal = S.errors.fromResponse(409, { error: 'narration_incomplete', code: 'narration_incomplete', message: 'm', missing: [{ scene_id: 's2', block_id: 's2:n', chars: 120 }, { scene_id: 's2', block_id: 's2:b1', chars: 30 }, { scene_id: 's4', block_id: 's4:n', chars: 10 }], chars: { total: 640, missing: 160 } }, null);
  let inst = makeInst(refusal);
  let exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-refused').dataset.code, 'narration_incomplete', 'the refusal is shown');
  const missing = find(exporter.el, 'export-missing').textContent;
  eq([missing.includes('s2, s4'), missing.includes('160')], [true, true], 'the missing scenes and characters are named');
  eq(inst.notes, ['storymaker.job_error.narration_incomplete'], 'the sentence of the refusal');
  find(exporter.el, 'export-go-voice').click();
  eq(inst.shown, ['voice'], 'the action leads to the voice panel');
  exporter.dispose();

  // music_incomplete leads to the music panel
  refusal = S.errors.fromResponse(409, { error: 'music_incomplete', code: 'music_incomplete', message: 'm', key: 'a'.repeat(64) }, null);
  inst = makeInst(refusal);
  exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music', 'a music file that is not ready');
  find(exporter.el, 'export-go-music').click();
  eq(inst.shown, ['music'], 'the action leads to the music panel');
  exporter.dispose();

  // a moved source says so
  refusal = S.errors.fromResponse(409, { error: 'music_incomplete', code: 'music_incomplete', message: 'm', key: '', music_issue: 'source_missing' }, null);
  inst = makeInst(refusal);
  exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  find(exporter.el, 'export-confirm').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music_source', 'a moved source');
  exporter.dispose();

  // silent_film only with no narration, no music and empty audio_events
  const report = { audio_events: [] };
  eq(S.isSilentFilm(quietAnswer(), report), true, 'a film without any sound');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { narration: { s1: { blocks: [{ id: 's1:n' }] } } }), report), false, 'narration');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { music: { key: 'k', ext: 'mp3', duration_ms: 1000 } }), report), false, 'music');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { missing: [{ scene_id: 's1', block_id: 's1:n', chars: 5 }] }), report), false, 'narration text without audio');
  eq(S.isSilentFilm(quietAnswer(), { audio_events: [{ kind: 'sfx', ref: 'sfx.pop', start_ms: 1, scene_id: 's1', take: 0 }] }), false, 'an sfx event');
  eq(S.isSilentFilm(null, report), false, 'an answer that has not arrived');
  inst = makeInst(null);
  exporter = S.createExport(inst);
  exporter.open();
  eq(find(exporter.el, 'export-silent').textContent, 'storymaker.export.silent_film', 'the note shows for a silent film');
  exporter.dispose();
  inst = makeInst(null, { audioInfo: Object.assign(quietAnswer(), { music: { key: 'k', ext: 'mp3', duration_ms: 1000 } }) });
  exporter = S.createExport(inst);
  exporter.open();
  eq(find(exporter.el, 'export-silent'), null, 'no note with music');
  exporter.dispose();
  // A panel that is gone is done with its job: a poll that was due asks for nothing, and an answer that arrives later starts no poll.
  const running = { id: 'r1', project_id: 'p1', kind: 'render', status: 'running', progress: 0 };
  const timers = [];
  const realSetTimeout = context.setTimeout;
  context.setTimeout = fn => { timers.push(fn); return timers.length; };
  let polled = 0;
  inst = makeInst(null, { api: { getJob() { polled++; return Promise.resolve(running); } } });
  exporter = S.createExport(inst);
  exporter.apply(running, true);
  eq(timers.length, 1, 'a running job polls');
  exporter.dispose();
  await timers[0]();
  eq(polled, 0, 'a poll that was due asks for nothing');
  exporter.apply(running, true);
  eq(timers.length, 1, 'an answer after dispose starts no poll');
  context.setTimeout = realSetTimeout;
  console.log('storymaker export refusals ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

// The Silent switch of the inspector in Node: it writes and clears scene.silent with an immediate save, for storyboard and code
// scenes alike, follows the story and locks when read-only. The narration text stays, because silent only keeps it out of the film.
func TestStorymakerInspectorSilentInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-inspector-silent.js")
	if err := os.WriteFile(script, []byte(storymakerInspectorSilentScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker inspector silent ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerInspectorSilentScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = String(tag); this.children = []; this.parent = null; this.dataset = {}; this.attrs = {}; this.listeners = {}; this.own = ''; this.value = ''; this.hidden = false; this.disabled = false; this.readOnly = false; this.checked = false; this.className = ''; this.style = { setProperty() {} }; }
  appendChild(child) { child.parent = this; this.children.push(child); return child; }
  removeChild(child) { this.children = this.children.filter(item => item !== child); child.parent = null; return child; }
  replaceWith(next) { const list = this.parent.children; list[list.indexOf(this)] = next; next.parent = this.parent; }
  get firstChild() { return this.children[0] || null; }
  get textContent() { return this.own + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this.children = []; this.own = String(text); }
  setAttribute(name, value) { this.attrs[name] = String(value); if (name === 'value') this.value = String(value); if (name === 'disabled') this.disabled = true; }
  removeAttribute(name) { delete this.attrs[name]; }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener() {}
  fire(type) { (this.listeners[type] || []).forEach(fn => fn({ target: this })); }
  focus() {}
  contains() { return false; }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.own = String(text); return node; },
  documentElement: { lang: 'en' },
  activeElement: null
};
const context = { console, document, setTimeout, clearTimeout };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-inspector.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const find = (node, stm) => { if (node.dataset.stm === stm) return node; for (const child of node.children) { const hit = find(child, stm); if (hit) return hit; } return null; };

const saves = [];
const inst = {
  t: key => key,
  bus: S.createBus(),
  domId: name => 'x-' + name,
  story: { scenes: [{ id: 's1', narration: 'Hello there.' }, { id: 'c1', kind: 'code', narration: 'A code scene.' }] },
  selected: 0,
  catalog: null,
  status: null,
  readonly: false,
  moveScene() {},
  deleteScene() {},
  openCode() {},
  editScene(id, fn, options) {
    const draft = JSON.parse(JSON.stringify(inst.story));
    fn(draft.scenes.find(scene => scene.id === id), draft);
    inst.story = draft;
    saves.push(!!(options && options.immediate));
    inst.bus.emit('story');
    return true;
  }
};
const panel = S.createInspector(inst);
const box = () => find(panel.el, 'scene-silent');

eq(box().checked, false, 'a normal scene is not silent');
box().checked = true;
box().fire('change');
eq(inst.story.scenes[0].silent, true, 'checking writes silent');
eq(inst.story.scenes[0].narration, 'Hello there.', 'the text stays');
eq(saves, [true], 'the save is immediate, so the audio answer and the blocker follow');
eq(box().checked, true, 'the switch stays on');
box().checked = false;
box().fire('change');
eq('silent' in inst.story.scenes[0], false, 'unchecking deletes the field');

// the story moves the switch
inst.story = JSON.parse(JSON.stringify(inst.story));
inst.story.scenes[0].silent = true;
inst.bus.emit('story');
eq(box().checked, true, 'the story moves the switch');

// a code scene has it too
inst.selected = 1;
inst.bus.emit('selection');
eq(box().checked, false, 'the code scene is not silent');
box().checked = true;
box().fire('change');
eq(inst.story.scenes[1].silent, true, 'a code scene can be silent');

// read-only locks it
inst.readonly = true;
inst.bus.emit('readonly');
eq(box().disabled, true, 'read-only disables the switch');
console.log('storymaker inspector silent ok');
`

const storymakerModelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir, storyFile, catalogFile] = process.argv.slice(2);
class FakeBitmap { constructor() { this.width = 1; this.height = 1; } }
const context = { console, ImageBitmap: FakeBitmap };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-model.js', 'storymaker-player.js']) {
  // the player module reads S.el only inside createPlayer, so the pure parts load without a DOM
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts, M = S.model;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const story = JSON.parse(fs.readFileSync(storyFile, 'utf8'));
const text = M.canonicalText(story);

// locate: a pointer finds its value, a missing field falls back to the deepest value that exists
const hit = M.locate(text, '/scenes/0/actors/0/at');
eq([hit.found, text.slice(hit.from, hit.to)], [true, '"desk"'], 'pointer to a string');
eq(M.locate(text, '/title').found, true, 'title');
const missing = M.locate(text, '/scenes/1/beats/0/expression');
eq([missing.found, missing.depth], [false, 4], 'a missing field gives the beat');
eq(M.locate(text, '/scenes/9/id').found, false, 'index out of range');
eq(M.parsePointer('/a~1b/c~0d'), ['a/b', 'c~d'], 'pointer escapes');
eq(M.lineColumn(text, M.locate(text, '/title').from), { line: 3, column: 12 }, 'line and column');
eq(M.positionFromMessage('invalid character at line 3, column 14'), { line: 3, column: 14 }, 'position from a message');
eq(M.offsetOf('ab\ncd\nef', 2, 2), 4, 'offset');
// a quote, a brace and an escape inside strings must not confuse the walker
const tricky = '{"a": "x\\"}{", "b": [1, {"c": "]"}], "d": 5}';
eq(tricky.slice(M.locate(tricky, '/d').from, M.locate(tricky, '/d').to), '5', 'string with quotes and braces');
eq(tricky.slice(M.locate(tricky, '/b/1/c').from, M.locate(tricky, '/b/1/c').to), '"]"', 'nested pointer');

// issues
const merged = M.mergeIssues([{ severity: 'minor', code: 'a', path: '/x', message: 'm' }, { severity: 'error', code: 'b', path: '', message: 'e' }], [{ severity: 'minor', code: 'a', path: '/x', message: 'm' }]);
eq(merged.map(i => i.severity), ['error', 'minor'], 'errors first, duplicates gone');
eq(M.visibleIssues([{ code: 'narration_missing' }, { code: 'x' }]).length, 2, 'narration_missing is shown');
eq(M.sceneIndexOfIssue({ path: '/scenes/2/beats/0' }, story), 2, 'scene of an issue by path');
eq(M.sceneIndexOfIssue({ scene_id: 's2', path: '' }, story), 1, 'scene of an issue by id');

// catalog and edits
const catalog = M.indexCatalog(JSON.parse(fs.readFileSync(catalogFile, 'utf8')));
eq(M.entriesOf(catalog, 'character').length, 4, 'four characters');
const draft = M.cloneStory(story);
eq(M.addScene(draft, 0, {}), 1, 'a scene after the first');
eq(draft.scenes.map(s => s.id), ['s1', 's4', 's2', 's3'], 'scene ids');
eq(draft.scenes[1].environment, 'env.office', 'a new scene keeps the environment');
const code = M.addScene(draft, 3, { kind: 'code' });
eq([draft.scenes[code].kind, draft.scenes[code].id], ['code', 'code5'], 'a code scene');
eq(M.removeScene(draft, 0), 0, 'remove');
eq(M.moveScene(draft, 0, 2), 2, 'move');
const man = catalog.byId['char.office-man-1'];
eq(M.insertActor(draft, 0, man, 'Office man').ok, true, 'first insert');
eq(M.insertActor(draft, 0, man, 'Office man').reason, 'already_in_scene', 'the same person twice');
eq(draft.cast.filter(c => c.character === 'char.office-man-1').length, 1, 'one cast member');
const props = M.insertProp(draft, 0, catalog.byId['prop.laptop'], catalog.byId['env.office']);
eq([props.ok, draft.scenes[0].props[0].at], [true, 'right'], 'a prop takes a free spot');
eq(M.insertActor(draft, M.scenesOf(draft).length - 1, man, 'x').ok, false, 'no actors in a code scene');
const one = { schema: 'storymaker/1', scenes: [{ id: 's1' }] };
eq(M.removeScene(one, 0), -1, 'the last scene stays');
eq(one.scenes[M.addScene(one, 0, { kind: 'code', taken: ['code2'] })].id, 'code3', 'a code file that outlived its scene keeps its id');
// the editor's environment edit: the server's default only reaches a scene without a transition
const worlds = { scenes: [{ id: 'a', environment: 'env.office', transition: 'cut' }, { id: 'b', environment: 'env.office', transition: 'fade' }, { id: 'c', environment: 'env.office', transition: 'fade' }, { id: 'd', environment: 'env.office', transition: 'fade' }] };
M.setEnvironment(worlds, worlds.scenes[1], 'env.home-office');
eq(worlds.scenes[1].environment, 'env.home-office', 'the environment is set');
eq(worlds.scenes.map(s => s.transition), ['cut', 'wipe', 'wipe', 'fade'], 'a fade between worlds becomes a wipe, on this scene and the next');
M.setEnvironment(worlds, worlds.scenes[1], 'env.office');
eq(worlds.scenes.map(s => s.transition), ['cut', 'wipe', 'wipe', 'fade'], 'a wipe stays when the worlds match again');
const chosen = { scenes: [{ id: 'a', environment: 'env.office', transition: 'cut' }, { id: 'b', environment: 'env.office', transition: 'slide-left' }] };
M.setEnvironment(chosen, chosen.scenes[1], 'env.home-office');
eq(chosen.scenes[1].transition, 'slide-left', 'another transition stays');
const opening = { scenes: [{ id: 'a', environment: 'env.office', transition: 'fade' }, { id: 'b', environment: 'env.office', transition: 'fade' }] };
M.setEnvironment(opening, opening.scenes[0], 'env.home-office');
eq(opening.scenes.map(s => s.transition), ['fade', 'wipe'], 'the first scene has no transition to change, the next one has');
const unsaved = { scenes: [{ id: 'a', environment: 'env.office' }, { id: 'b', environment: 'env.office' }] };
M.setEnvironment(unsaved, unsaved.scenes[1], 'env.home-office');
eq('transition' in unsaved.scenes[1], false, 'an unset transition is left to the server default');
const scene = { id: 'x', narration: 'hi' };
M.setSceneField(scene, 'narration', ''); eq('narration' in scene, false, 'an empty field leaves the document');
M.setDuration(scene, false, '7.26'); eq(scene.duration, 7.3, 'fixed duration');
M.setDuration(scene, true, 0); eq(scene.duration, 'auto', 'auto duration');
const report = { frame_count: 300, fps: 30, scenes: [{ id: 'a', start_frame: 0, end_frame: 150 }, { id: 'b', start_frame: 140, end_frame: 300 }] };
eq(M.sceneAtFrame(report, 139), 0, 'frame in the first scene'); eq(M.sceneAtFrame(report, 145), 1, 'transitions overlap, the later scene wins');
eq(M.frameForScene(report, 1), 140 + Math.floor(160 * 0.4), 'frame for a scene');
eq([{ browser: { ready: true } }, { browser: { ready: false, can_download: true } }, { browser: { ready: false } }, {}, null].map(M.browserState), ['ready', 'download', 'missing', 'missing', 'missing'], 'the render browser is ready, fetched by the first export, or missing');

// the message schema: only fixed shapes, with caps
const P = S.player.checkIncoming;
const valid = { sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard' }], issues: [{ severity: 'warning', code: 'x', path: '/a', message: 'm' }] } };
eq(P(valid).report.scenes.length, 1, 'a valid report');
eq(P(Object.assign({}, valid, { sm: 2 })), null, 'wrong version');
eq(P({ sm: 1, type: 'load' }), null, 'a parent-to-host type is refused');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: new Array(401).fill({ id: 's', start_frame: 0, end_frame: 1 }) } }), null, 'too many scenes');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: -1, fps: 30, scenes: [] } }), null, 'negative frame count');
eq(P({ sm: 1, type: 'time', frame: 3, playing: 'yes' }), null, 'time needs a boolean');
eq(P({ sm: 1, type: 'time', frame: 3, playing: true }), { type: 'time', frame: 3, playing: true }, 'time');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: new Array(2001).fill('catalog:x') }), null, 'too many keys');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: ['catalog:rig/human.json', 7] }), null, 'a key that is not a string');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: {} }), null, 'a thumb needs an ImageBitmap');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: new FakeBitmap() }).scene_id, 's1', 'a thumb');
eq(P({ sm: 1, type: 'error', code: 'code_error', message: 'x'.repeat(5000), scene_id: 's3' }).message.length, 2000, 'long messages are cut');
eq(P({ sm: 1, type: 'ready', caps: { workers: true, offscreen: false }, extra: 'x' }), { type: 'ready', caps: { workers: true, offscreen: false } }, 'unknown fields are dropped');
// the compiled whitelist of phase 2: a malformed entry is dropped, the message stays
{
  const evHash = 'a'.repeat(64);
  const rich = P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, cue_precision: 0.5,
    scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard', cues: [{ marker: 1, at_ms: 1840, placed: 'span' }, { marker: 'x', at_ms: 1, placed: 'span' }, { marker: 2, at_ms: 5, placed: 'guess' }] }],
    audio_events: [
      { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0, extra: 'x' },
      { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 },
      { kind: 'music', ref: evHash, start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: 'r'.repeat(81), start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: -1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: 1, scene_id: 's1', piece: 'x' },
      'nonsense'
    ],
    captions: [
      { start_ms: 500, end_ms: 2600, lines: ['one', 'two', 'three'], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 2600, end_ms: 3600, lines: ['x'.repeat(500)], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 3600, end_ms: 4600, lines: [7], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 4600, end_ms: 5600, lines: [], scene_id: 's1', block_id: 's1:n' }
    ]
  } });
  eq(rich.report.audio_events, [
    { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 }
  ], 'malformed events are dropped, unknown fields too, and the message stays');
  eq(rich.report.captions.map(c => c.lines.map(line => line.length)), [[3, 3], [400]], 'at most 2 lines, each clipped at 400 characters');
  eq(rich.report.scenes[0].cues, [{ marker: 1, at_ms: 1840, placed: 'span' }], 'only well-formed cues stay');
  eq(rich.report.cue_precision, 0.5, 'cue_precision');
  const bare = P(valid).report;
  eq([bare.audio_events, bare.captions, bare.cue_precision, bare.scenes[0].cues], [[], [], null, []], 'a report without the new fields');
  // the other side of the wire: a report from a real compile loses nothing (dir is ui/js/desktop/apps, four levels below the repo root)
  const measured = JSON.parse(fs.readFileSync(path.join(dir, '..', '..', '..', '..', 'internal', 'storymaker', 'render', 'testdata', 'report_measured.json'), 'utf8'));
  const wire = P({ sm: 1, type: 'compiled', report: measured }).report;
  eq(wire.audio_events.length, measured.audio_events.length, 'every event of a real compile passes');
  eq(wire.captions.length, (measured.captions || []).length, 'every caption of a real compile passes');
}

// the export size follows the aspect, as render.CanvasSize does (the preview compiles smaller)
eq([M.exportSize('16:9'), M.exportSize('9:16'), M.exportSize('1:1'), M.exportSize(''), M.exportSize('4:3')], [[1920, 1080], [1080, 1920], [1080, 1080], [1920, 1080], [1920, 1080]], 'export size by aspect');

console.log('storymaker app model ok');
`

// A new project sends the desktop language. When no voice speaks it, the form shows the languages from the error and offers a
// create without a voice, and that second call sends no language.
func TestStorymakerNewProjectLanguageInNode(t *testing.T) {
	t.Parallel()
	if !strings.Contains(readDesktopAssetText(t, "js/desktop/apps/storymaker.js"), "get language() { return String(window.SYSTEM_LANG || document.documentElement.lang || 'en'); }") {
		t.Error("storymaker.js does not hand the desktop language to the project list")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-new-project.js")
	if err := os.WriteFile(script, []byte(storymakerNewProjectScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker new project ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerNewProjectScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = tag; this.kids = []; this.parent = null; this.attrs = {}; this.dataset = {}; this.handlers = {}; this.style = { setProperty() {} }; this.ownText = ''; this.className = ''; }
  get firstChild() { return this.kids[0] || null; }
  appendChild(kid) { if (kid.parent) kid.parent.removeChild(kid); kid.parent = this; this.kids.push(kid); return kid; }
  removeChild(kid) { this.kids.splice(this.kids.indexOf(kid), 1); kid.parent = null; return kid; }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  getAttribute(name) { return name in this.attrs ? this.attrs[name] : null; }
  get hidden() { return 'hidden' in this.attrs; }
  set hidden(on) { if (on) this.attrs.hidden = ''; else delete this.attrs.hidden; }
  get disabled() { return 'disabled' in this.attrs; }
  set disabled(on) { if (on) this.attrs.disabled = ''; else delete this.attrs.disabled; }
  get value() { return this.val !== undefined ? this.val : (this.attrs.value || ''); }
  set value(next) { this.val = String(next); }
  get textContent() { return this.kids.length ? this.kids.map(kid => kid.textContent).join('') : this.ownText; }
  set textContent(next) { this.kids = []; this.ownText = String(next); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  fire(type) { (this.handlers[type] || []).slice().forEach(fn => fn({ type, target: this, preventDefault() {}, stopPropagation() {} })); }
  click() { if (!this.disabled) this.fire('click'); }
  focus() {}
  querySelectorAll() { return []; }
  find(name) { if (this.dataset.stm === name) return this; for (const kid of this.kids) { const hit = kid.find(name); if (hit) return hit; } return null; }
  findTag(tag) { if (this.tag === tag) return this; for (const kid of this.kids) { const hit = kid.findTag(tag); if (hit) return hit; } return null; }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.ownText = String(text); return node; },
  documentElement: { lang: 'en' }
};
const context = { console, setTimeout, clearTimeout, document };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-projects.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const flush = () => new Promise(resolve => setImmediate(resolve));
const t = (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key);

function open(results) {
  const calls = [];
  const inst = {
    t, readonly: false, language: 'de', domId: suffix => 'stm-test-' + suffix,
    createProject(spec) { calls.push(spec); const next = results.shift(); return next ? Promise.reject(next) : Promise.resolve(); },
    openProject() {}, deleteProject() {}, confirm: async () => false
  };
  S.createAgentStart = () => ({ el: document.createElement('div'), refresh() {} });
  const view = S.createProjects(inst);
  view.setProjects([]);
  view.el.find('new-title').value = 'Film';
  return { calls, view, f: name => view.el.find(name) };
}

(async () => {
  const refused = S.errors.fromResponse(422, { error: 'narration_unavailable', code: 'narration_unavailable', message: 'No voice speaks ja.', languages: ['de', 'en'] }, 0);
  const first = open([refused, null]);
  eq(first.f('projects-offer').hidden, true, 'no offer before a refusal');
  first.view.el.findTag('form').fire('submit');
  await flush();
  eq(first.calls, [{ title: 'Film', aspect: '16:9', language: 'de' }], 'the first call sends the desktop language');
  const text = first.f('projects-error').textContent;
  eq(text.includes('storymaker.error.narration_unavailable'), true, 'the refusal has its own sentence');
  eq(text.includes('storymaker.projects.languages {"languages":"de, en"}'), true, 'err.data.languages shows');
  eq(first.f('projects-offer').hidden, false, 'the offer shows');

  first.f('create-without-voice').click();
  await flush();
  eq(first.calls[1], { title: 'Film', aspect: '16:9' }, 'the second call sends no language');
  eq([first.f('projects-offer').hidden, first.f('projects-error').hidden], [true, true], 'a success clears both');

  // Any other failure keeps its plain sentence and offers nothing.
  const plain = open([S.errors.fromResponse(500, { error: 'internal', code: 'internal', message: 'boom' }, 0)]);
  plain.view.el.findTag('form').fire('submit');
  await flush();
  eq(plain.f('projects-error').textContent, 'storymaker.error.generic boom', 'a plain failure');
  eq(plain.f('projects-offer').hidden, true, 'no offer for a plain failure');

  console.log('storymaker new project ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

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
load(['storymaker-core.js', 'storymaker-handoff.js', 'storymaker-dialog.js']);
const S = context.StorymakerParts;
// What Video Studio answers when the dialog opens. The real studioReady decides.
const vsReady = { enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false };
let vsAnswer = () => Promise.resolve(vsReady);
S.handoff.status = () => vsAnswer();
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
})().catch(fail);
`

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
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-dialog.js', 'storymaker-handoff.js', 'storymaker-export.js']);
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
S.handoff = { canOffer: () => false, stemFailed: () => false, run: () => Promise.resolve({ projectId: '' }) };
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
