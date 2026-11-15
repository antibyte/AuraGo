#!/usr/bin/env node
// W1b Part A: the action registry (easydrag-actions.js). Every row has a label key in all 16 locales, an icon,
// a valid risk and scope and a shortcut no other row of its scope uses; every cause of a shared check has a
// reason in all 16 locales; state() refuses and run() does nothing when a check fails; items() marks risky
// rows danger, leaves out what nobody bound and drops stray separators; key() fires an action once.
// Legacy assert: the arrays and objects come out of the vm context, so a strict deepEqual would compare prototypes.
import assert from 'node:assert';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const lang = new URL('../ui/lang/easydrag/', import.meta.url);
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = Object.fromEntries(LOCALES.map(l => [l, JSON.parse(fs.readFileSync(new URL(l + '.json', lang), 'utf8'))]));

const box = vm.createContext({ window: {}, navigator: { platform: 'Linux' }, document: {}, crypto: webcrypto, console, Intl, Date, Set, Map, WeakMap });
for (const file of ['easydrag-core.js', 'easydrag-actions.js']) vm.runInContext(fs.readFileSync(new URL(file, apps), 'utf8'), box, { filename: file });
const ED = box.window.EasyDrag;
const A = ED.actions;

// 1. The table.
const seen = new Set();
const keysByScope = { editor: new Map(), canvas: new Map() };
for (const row of A.ROWS) {
    assert.match(row.id, /^(flow|edit|step|view)\.[a-z_]+$/, row.id + ' is not group.name');
    assert.ok(!seen.has(row.id), row.id + ' twice');
    seen.add(row.id);
    for (const l of LOCALES) assert.ok(words[l][row.label], row.id + ': ' + row.label + ' is missing in ' + l);
    assert.ok(row.icon, row.id + ' has no icon');
    assert.ok(!row.glyph || Object.prototype.hasOwnProperty.call(ED.core.ICONS, row.glyph), row.id + ': glyph ' + row.glyph + ' is not a core icon');
    assert.ok(['safe', 'undoable', 'confirm'].includes(row.risk), row.id + ' risk');
    assert.ok(['editor', 'canvas'].includes(row.scope), row.id + ' scope');
    row.needs.forEach(need => assert.equal(typeof A.CHECKS[need], 'function', row.id + ': unknown check ' + need));
    if (row.shortcut) {
        const taken = keysByScope[row.scope].get(row.shortcut);
        assert.equal(taken, undefined, row.id + ' and ' + taken + ' share ' + row.shortcut + ' in the ' + row.scope + ' scope');
        keysByScope[row.scope].set(row.shortcut, row.id);
    }
}
assert.deepEqual(A.ROWS.filter(r => r.risk !== 'safe').map(r => r.id + ':' + r.risk), ['flow.delete:confirm', 'edit.cut:undoable', 'edit.delete:undoable']);
assert.ok(Object.isFrozen(A.ROWS) && Object.isFrozen(A.ROWS[0]), 'the table is frozen');
assert.deepEqual([...A.TOOLS.map(([tool, id]) => tool + ':' + id + ':' + A.row(id).glyph)], ['add:step.add_next:plus', 'connect:step.connect:link', 'test:step.test:flask', 'disable:step.toggle:eye-off', 'duplicate:edit.duplicate:copy', 'delete:edit.delete:trash'], 'the card tools and their glyphs');
// the shortcuts the editor shows today, spelled canonically
assert.deepEqual(['flow.save', 'flow.test', 'view.search', 'view.find_step', 'edit.undo', 'edit.redo', 'edit.delete', 'view.zoom_fit', 'view.zoom_reset', 'view.shortcuts', 'edit.auto_layout']
    .map(id => A.row(id).shortcut), ['Ctrl+S', 'Ctrl+Enter', 'Ctrl+K', 'Ctrl+F', 'Ctrl+Z', 'Ctrl+Shift+Z', 'Delete', 'Shift+1', 'Ctrl+0', '?', 'Alt+Shift+F']);

// 2. Every cause has a reason in all 16 locales; so do the reasons that bound actions return and the key name of Delete.
const reasonKeys = Object.values(A.REASONS).concat(['nothing_to_paste', 'nothing_to_undo', 'nothing_to_redo', 'palette_hidden', 'no_steps', 'no_output', 'export_unsaved', 'stop_unavailable'].map(k => 'easydrag.ui.reason_' + k), ['easydrag.ui.key_delete', 'easydrag.ui.export_email']);
for (const key of reasonKeys) for (const l of LOCALES) assert.ok(words[l][key] && words[l][key].trim(), key + ' is missing in ' + l);
assert.equal(new Set(Object.values(A.REASONS)).size, Object.keys(A.REASONS).length, 'one reason per cause');

// 3. state, run, items, button and key on a stub host.
const t = key => words.en[key] || key;
function makeHost(extra) {
    return Object.assign({ t, readonly: false, runView: null, selection: new Set(['a']), quickAdd: null, flow: { live: {} }, flowEnabled: true, model: { doc: { nodes: [{}, {}] } } }, extra || {});
}
function bindAll(host, calls) {
    const note = id => () => { calls.push(id); };
    ['flow.save', 'flow.test', 'flow.run_now', 'flow.delete', 'edit.undo', 'edit.cut', 'edit.copy', 'edit.delete', 'edit.paste', 'view.zoom_fit', 'view.zoom_in', 'view.zoom_out', 'step.open', 'step.toggle', 'edit.auto_layout']
        .forEach(id => A.bind(id, { run: note(id) }, host));
}
{
    const calls = [];
    const host = makeHost();
    bindAll(host, calls);
    const c = { host };
    assert.deepEqual(A.state('flow.save', c), { enabled: true, reason: '' });
    assert.equal(A.state('flow.stop', c).enabled, false, 'an action nobody bound is unavailable');
    A.run('flow.save', c);
    assert.deepEqual(calls, ['flow.save']);

    // writable: a past run, read-only mode, a locked list; the reasons are the one text per cause
    host.runView = {};
    assert.deepEqual(A.state('flow.save', c), { enabled: false, reason: words.en['easydrag.ui.reason_run_view'] });
    assert.equal(A.run('flow.save', c), false);
    assert.deepEqual(calls, ['flow.save'], 'run does nothing when a check fails');
    host.runView = null; host.readonly = true;
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_readonly']);
    assert.equal(A.state('edit.copy', c).enabled, true, 'Copy has no writable check: it works in read-only mode');
    assert.equal(A.state('edit.cut', c).reason, words.en['easydrag.ui.reason_readonly']);
    host.readonly = false; host.actionFacts = () => ({ locked: true });
    assert.equal(A.state('flow.delete', c).reason, words.en['easydrag.ui.reason_locked']);
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_locked']);
    // free: a dialog or the restore offer
    host.actionFacts = () => ({ dialog: true });
    assert.equal(A.state('edit.delete', c).reason, words.en['easydrag.ui.reason_dialog_open']);
    assert.equal(A.state('flow.save', c).reason, words.en['easydrag.ui.reason_dialog_open']);
    host.actionFacts = null;
    host.quickAdd = {};
    assert.equal(A.state('edit.delete', c).reason, words.en['easydrag.ui.reason_dialog_open'], 'edits wait for quick-add');
    assert.equal(A.state('flow.save', c).enabled, true, 'a save does not');
    host.quickAdd = null;
    host.actionFacts = () => ({ restoring: true });
    assert.equal(A.state('flow.save', c).reason, words.en['easydrag.ui.reason_restoring']);
    // idle, steps2, selection, published, active
    host.actionFacts = () => ({ running: true });
    assert.equal(A.state('flow.test', c).reason, words.en['easydrag.ui.reason_running']);
    host.actionFacts = null;
    host.model.doc.nodes.length = 1;
    assert.equal(A.state('edit.auto_layout', c).reason, words.en['easydrag.ui.reason_two_steps']);
    host.selection.clear();
    assert.equal(A.state('edit.cut', c).reason, words.en['easydrag.ui.reason_no_selection']);
    assert.equal(A.state('edit.cut', { host, node: 'n1' }).enabled, true, 'a card names its step');
    host.flow.live = null;
    assert.equal(A.state('flow.run_now', c).reason, words.en['easydrag.ui.reason_not_published']);
    host.flow.live = {}; host.flowEnabled = false;
    assert.equal(A.state('flow.run_now', c).reason, words.en['easydrag.ui.reason_paused']);
    assert.equal(A.state('flow.run_now', { host, flow: { published: true, enabled: true } }).enabled, true, 'a start page card brings its own flow');
    host.flowEnabled = true;

    // a bound check adds its own reason, label and show
    A.bind('step.test', { run() {}, check: () => 'easydrag.ui.reason_no_output', label: () => 'easydrag.ui.block_simulate', show: () => true }, host);
    assert.equal(A.state('step.test', { host, node: 'n1' }).reason, words.en['easydrag.ui.reason_no_output']);
    assert.equal(A.label('step.test', c), words.en['easydrag.ui.block_simulate']);

    // items: ids to menu entries
    host.selection.add('a');
    const out = A.items(['-', 'flow.save', '-', '-', 'flow.stop', '-', 'edit.cut', 'edit.copy', 'view.zoom_fit', 'flow.delete', '-'], c);
    assert.deepEqual(out.map(e => (e.separator ? '|' : e.id)), ['save', '|', 'cut', 'copy', 'zoom-fit', 'delete'], 'unbound rows, doubled, leading and trailing separators are dropped');
    const byId = Object.fromEntries(out.filter(e => !e.separator).map(e => [e.id, e]));
    assert.equal(byId.cut.danger, true, 'undoable is danger');
    assert.equal(byId.delete.danger, true, 'confirm is danger');
    assert.equal(byId.copy.danger, undefined);
    assert.equal(byId.save.shortcut, 'Ctrl+S', 'a shell shortcut is a real shortcut');
    assert.equal(byId.save.shortcutHint, undefined);
    assert.equal(byId.cut.shortcutHint, 'Ctrl+X', 'other shortcuts are only drawn');
    assert.equal(byId.cut.shortcut, undefined);
    assert.equal(byId.save.label, 'Save now');
    const keyed = A.items(['flow.save'], { host, keyed: true })[0];
    assert.deepEqual([keyed.labelKey, keyed.label], ['easydrag.ui.save_now', undefined], 'a window menu entry carries the label key');
    host.readonly = true;
    const ro = A.items(['edit.cut', 'edit.copy', 'edit.delete'], c);
    assert.deepEqual(ro.map(e => [e.disabled, e.disabledHint]), [[true, words.en['easydrag.ui.reason_readonly']], [false, undefined], [true, words.en['easydrag.ui.reason_readonly']]]);
    assert.equal(ro[2].shortcutHint, 'Del');
    ro[0].action();
    assert.equal(calls.includes('edit.cut'), false, 'a disabled entry does nothing when it is clicked');
    host.readonly = false;

    // button: aria-disabled with the reason, never disabled
    const on = A.button('edit.delete', c);
    assert.deepEqual(on, { 'aria-label': 'Delete', title: 'Delete (Del)' });
    host.readonly = true;
    const off = A.button('edit.delete', c);
    assert.equal(off['aria-disabled'], 'true');
    assert.equal(off['aria-description'], words.en['easydrag.ui.reason_readonly']);
    assert.ok(off.title.startsWith('Delete (Del)') && off.title.endsWith(words.en['easydrag.ui.reason_readonly']));
    assert.equal(A.button('flow.stop', c), null, 'an unbound action has no button');
    host.readonly = false;
    assert.equal(A.attrString({ title: 'a "b"' }), ' title="a &quot;b&quot;"');

    // display: the named Delete, the real minus
    assert.equal(A.display('Delete', t), 'Del');
    assert.equal(A.display('-', t), String.fromCodePoint(0x2212));
    assert.equal(A.display('Ctrl+Shift+Z', t), 'Ctrl+Shift+Z');
}

// 4. key: one handler, once; scope and text-field rules.
{
    const calls = [];
    const host = makeHost();
    bindAll(host, calls);
    A.bind('edit.undo', { run: () => { calls.push('edit.undo'); }, check: () => 'easydrag.ui.reason_nothing_to_undo' }, host);
    A.bind('edit.redo', { run: () => { calls.push('edit.redo'); } }, host);
    A.bind('step.open', { run: () => { calls.push('step.open'); }, accepts: (event, where) => !where.control }, host);
    let observed = [];
    const stop = A.observe(id => observed.push(id));
    const press = (k, extra, where) => {
        const event = Object.assign({ key: k, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }, extra);
        const took = A.key(event, Object.assign({ host, onCanvas: true }, where));
        return { took, prevented: event.defaultPrevented };
    };
    assert.deepEqual(press('s', { ctrlKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('s', { ctrlKey: true }, { editable: true }), { took: true, prevented: true }, 'Ctrl+S works from a text field');
    assert.deepEqual(press('Enter', { ctrlKey: true }, { editable: true, quickAdd: true }), { took: true, prevented: true });
    assert.deepEqual(press('z', { ctrlKey: true }, { editable: true }), { took: false, prevented: false }, 'a text field keeps its own undo');
    assert.deepEqual(press('z', { ctrlKey: true }, { quickAdd: true }), { took: false, prevented: false }, 'edits wait for quick-add');
    assert.deepEqual(press('z', { ctrlKey: true }, { onCanvas: false }), { took: false, prevented: false }, 'canvas keys need the canvas');
    assert.deepEqual(press('Z', { ctrlKey: true, shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('y', { ctrlKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('z', { ctrlKey: true }), { took: true, prevented: true }, 'a refused action still takes the key');
    assert.deepEqual(press('Delete'), { took: true, prevented: true });
    assert.deepEqual(press('Backspace'), { took: true, prevented: true });
    assert.deepEqual(press('1', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('!', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('=', {}), { took: true, prevented: true });
    assert.deepEqual(press('+', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('-', {}), { took: true, prevented: true });
    assert.deepEqual(press('d', {}), { took: true, prevented: true });
    assert.deepEqual(press('D', { shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('Enter', {}, { control: true }), { took: false, prevented: false }, 'Enter on a button inside the canvas keeps its default');
    assert.deepEqual(press('Enter', {}), { took: true, prevented: true });
    assert.deepEqual(press('f', { altKey: true, shiftKey: true }), { took: true, prevented: true });
    assert.deepEqual(press('f', { altKey: true, shiftKey: true }, { editable: true }), { took: false, prevented: false }, 'Alt+Shift+F needs the focus outside a text field');
    assert.deepEqual(press('c', { ctrlKey: true }), { took: true, prevented: true });
    host.selection.clear();
    assert.deepEqual(press('c', { ctrlKey: true }), { took: false, prevented: false }, 'Copy with nothing selected leaves the browser its own copy');
    host.selection.add('a');
    assert.deepEqual(press('d', { ctrlKey: true }), { took: false, prevented: false }, 'an action nobody bound takes no key');
    assert.deepEqual(press('x', {}), { took: false, prevented: false });
    assert.deepEqual(press('s', { metaKey: true }), { took: false, prevented: false }, 'Cmd is no Ctrl here (isMod is ctrlKey off macOS)');
    assert.deepEqual(observed, ['flow.save', 'flow.save', 'flow.test', 'edit.redo', 'edit.redo', 'edit.delete', 'edit.delete', 'view.zoom_fit', 'view.zoom_fit', 'view.zoom_in', 'view.zoom_in', 'view.zoom_out', 'step.toggle', 'step.toggle', 'step.open', 'edit.auto_layout', 'edit.copy']);
    stop();
    assert.deepEqual(calls.filter(id => id === 'edit.undo'), [], 'a refused undo never ran');
    observed = [];
    press('s', { ctrlKey: true });
    assert.deepEqual(observed, [], 'observe stops');

    // the table: actions with their shortcut, gestures as written
    const layout = [['g', ['edit.undo', 'flow.home', ['Space', 'easydrag.ui.keys_pan'], 'view.search']]];
    assert.deepEqual(A.table(layout, { host }), [['g', [['Ctrl+Z', 'easydrag.ui.undo'], ['Space', 'easydrag.ui.keys_pan'], ['Ctrl+K', 'easydrag.ui.keys_search']]]]);
    A.bind('view.search', { run() {}, label: () => 'easydrag.ui.keys_add' }, host);
    assert.equal(A.table(layout, { host })[0][1][2][1], 'easydrag.ui.keys_add');
}

// 5. A global binding serves every host; a host binding wins.
{
    const host = makeHost();
    A.bind('view.liveliness_normal', { run() {}, checked: () => true });
    A.bind('view.liveliness_calm', { run() {}, checked: () => false });
    const [parent] = A.items(['view.liveliness'], { host });
    assert.equal(parent.label, 'Liveliness');
    assert.deepEqual(parent.items.map(e => [e.id, e.checked, e.shortcutHint]), [['liveliness-normal', true, String.fromCodePoint(0x2713)], ['liveliness-calm', false, '']]);
}
console.log('PASS action registry: ' + A.ROWS.length + ' rows');
