// a1008 checks (audit 2026-10-08, the editor findings): the test dialog confirms effects only for
// a test that started; the step dialog blocks the editor's shortcuts and window menus like any
// other dialog, and its pending note is saved before the editor leaves; Apply in the flow
// settings keeps the maximum run time to the second; run updates leave a focused card tool
// where it is; the template preview names errors in the user's language. They run on the c1d07
// sandbox (test-easydrag-extra3.mjs returns sandbox, openEditor and desktopMenus);
// test-easydrag.mjs calls run(env) with its helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const EFFECTS_KEY = 'aurago.easydrag.effects-ok.f1';
const DRAFT_KEY = 'aurago.easydrag.draft.f1';

// flowDoc is a flow start -> alpha -> beta, as in test-easydrag-extra3.mjs.
function flowDoc(settings) {
    const doc = {
        schema: 1, name: 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} },
            { id: B, key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: '' }, settings: {} }
        ],
        edges: [
            { id: 'e1', source: { node: T1, port: 'out' }, target: { node: A, port: 'in' } },
            { id: 'e2', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }
        ]
    };
    if (settings) doc.settings = settings;
    return doc;
}

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

function readLang(apps, locale) {
    return JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));
}

// missingKeys lists locale:key for each key that a locale lacks or leaves empty.
function missingKeys(apps, keys) {
    return LOCALES.flatMap(locale => {
        const words = readLang(apps, locale);
        return keys.filter(key => typeof words[key] !== 'string' || !words[key].trim()).map(key => locale + ':' + key);
    });
}

export async function run(env) {
    const { apps, types, eq, guardAsync, settle, sandbox, openEditor, desktopMenus } = env;

    // ── 1.1: the effects are confirmed by a test that started, not by a click on Run ──

    // fxEditor opens f1 with alpha and beta sending messages (catalog effects; the publish
    // preview adds none); onTest(req) answers POST /test.
    function fxEditor(onTest) {
        const fxTypes = new Map(types);
        fxTypes.set('web.search', Object.assign({}, types.get('web.search'), { effects: ['sends_message'] }));
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows/f1/publish-preview') return { issues: [], effects: [], diff: {} };
            if (/\/test-data\//.test(req.url)) return { data: {} };
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test') return onTest(req);
            return undefined;
        });
        h.catalog = { types: fxTypes, list: Array.from(fxTypes.values()), categories: [] };
        return { h, editor: openEditor(h) };
    }
    // openTest clicks Test and returns the test dialog with a live result ('open' until it closes).
    async function openTest(h, editor) {
        const before = h.dialogs.length;
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        const dialog = h.dialogs.length > before ? h.dialogs[h.dialogs.length - 1] : null;
        const state = { dialog, result: 'open' };
        if (dialog) dialog.done.then(value => { state.result = value; });
        return state;
    }
    const lists = dialog => !!(dialog && dialog.body.querySelector('[data-ed-effect="sends_message"]'));

    await guardAsync('a1008 1.1 a test whose POST fails confirms no effects; the next attempt asks again', async () => {
        const failures = [() => apiError('FLOW_INVALID'), () => Object.assign(new Error('HTTP 500'), { status: 500, body: {} })];
        let fail = true;
        const { h, editor } = fxEditor(() => {
            if (fail && failures.length) throw failures.shift()();
            return { run_id: 'r1' };
        });
        await settle();
        const confirmed = () => Array.from(editor.ed.effectsConfirmed || []);
        const stored = () => (h.store.has(EFFECTS_KEY) ? h.store.get(EFFECTS_KEY) : null);
        const first = await openTest(h, editor);
        // "Don't ask again" is ticked: a stored confirmation would outlive this window.
        first.dialog.body.querySelector('[data-ed-effects-skip]').checked = true;
        const runBtn = first.dialog.el.querySelector('[data-ed-action="run"]');
        const rows = [];
        for (const name of ['409 FLOW_INVALID', '500']) {
            runBtn.fire('click');
            await settle();
            rows.push([name, first.result, confirmed(), stored(), runBtn.disabled, lists(first.dialog)]);
        }
        // Closed without a run, the dialog of the next test lists the same effect again.
        first.dialog.el.querySelector('[data-ed-action="cancel"]').fire('click');
        await settle();
        const again = await openTest(h, editor);
        const asksAgain = lists(again.dialog);
        // A run that starts confirms them, and "Don't ask again" stores them.
        fail = false;
        const skip = again.dialog.body.querySelector('[data-ed-effects-skip]');
        if (skip) skip.checked = true;
        again.dialog.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const tests = h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows/f1/test').length;
        eq('a1008 1.1 a failed test POST keeps the dialog open and stores no confirmation; the next dialog asks again; a started test confirms',
            [rows, first.result, asksAgain, again.result, confirmed(), stored(), tests, h.notes.map(n => n.message), h.logged],
            [[['409 FLOW_INVALID', 'open', [], null, false, true], ['500', 'open', [], null, false, true]], 'cancel', true, 'run',
                ['sends_message'], '["sends_message"]', 3, ['error_flow_invalid', 'error_network'], []]);
        editor.dispose();
    });

    // ── 1.3: the step dialog is a dialog for the editor's shortcuts and window menus ──

    // stepDialog opens f1 (a save answers revision 4) with alpha's step dialog open.
    async function stepDialog(platform) {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined), platform);
        const editor = openEditor(h);
        await settle();
        editor.ed.bus.emit('open-detail', { nodeId: A });
        return { h, editor, detail: editor.el.querySelector('.ed-detail') };
    }
    // noteField shows the step dialog's note tab and returns its text area.
    function noteField(detail) {
        detail.querySelector('[data-ed-pane="note"]').fire('click');
        return detail.querySelector('.ed-note');
    }
    // counters wraps saveNow's saver.save and the palette search's focus and counts their calls.
    function counters(editor) {
        const count = { saves: 0, searches: 0 };
        const save = editor.ed.saver.save;
        editor.ed.saver.save = (...args) => { count.saves++; return save.apply(editor.ed.saver, args); };
        const search = editor.el.querySelector('.ed-palette-search');
        const focus = search.focus.bind(search);
        search.focus = () => { count.searches++; focus(); };
        return count;
    }
    // testRequests counts what a test or publish dialog asks for (publish preview, test data, test).
    const testRequests = h => h.requests.filter(r => /\/(publish-preview|test)$|\/test-data\//.test(r.url)).length;

    await guardAsync('a1008 1.3 under the step dialog and any other dialog Ctrl+S, Ctrl+Enter and Ctrl+K do nothing and never reach the browser', async () => {
        const outcomes = [];
        for (const [platform, modifier] of [['MacIntel', 'metaKey'], ['Win32', 'ctrlKey']]) {
            // Focus on a button of the step dialog (the desktop dispatches all three menu keys), in
            // its note and in a field of the flow settings (the desktop dispatches only Mod+S there;
            // the editor's handler sees Enter and K).
            for (const where of ['button', 'note', 'settings']) {
                let h, editor, target;
                if (where === 'settings') {
                    h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined), platform);
                    editor = openEditor(h);
                    await settle();
                    target = h.ED.dialogs.flowSettings(editor.ed).body.querySelector('[data-ed-set="description"]');
                } else {
                    const opened = await stepDialog(platform);
                    ({ h, editor } = opened);
                    target = where === 'note' ? noteField(opened.detail) : opened.detail.querySelector('[data-ed-detail-close]');
                }
                target.focus();
                const count = counters(editor);
                const dialogs = h.dialogs.length;
                const desktop = desktopMenus(h.menus, []);
                const prevented = [];
                // Ctrl+K first: before the fix Ctrl+Enter opened the test dialog, under which K waits.
                for (const key of ['k', 's', 'Enter']) {
                    const event = {
                        type: 'keydown', key, code: key === 'Enter' ? 'Enter' : 'Key' + key.toUpperCase(), target, repeat: false,
                        ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, defaultPrevented: false,
                        preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}
                    };
                    event[modifier] = true;
                    desktop.handleWindowMenuShortcut(event); // the desktop's keydown listener runs first
                    const seen = h.fireDoc('keydown', event); // then the editor's (on a copy of the event)
                    prevented.push(seen.defaultPrevented);
                }
                await settle();
                outcomes.push([platform, where, count.saves, count.searches, h.dialogs.length - dialogs, testRequests(h), prevented,
                    !!editor.ed.detail, h.dom.document.activeElement === target, h.logged]);
                editor.dispose();
            }
        }
        eq('a1008 1.3 with a dialog open nothing saves, no test dialog opens, the palette search keeps no focus; Mod+S, Mod+K and Mod+Enter are prevented (macOS, Windows)',
            outcomes, ['MacIntel', 'Win32'].flatMap(p => ['button', 'note', 'settings'].map(w => [p, w, 0, 0, 0, 0, [true, true, true], w !== 'settings', true, []])));
    });

    await guardAsync('a1008 1.3 the window menu items that wait for a dialog wait for the step dialog too', async () => {
        const { h, editor } = await stepDialog('Win32');
        const count = counters(editor);
        const dialogs = h.dialogs.length;
        const items = h.menus.flatMap(m => m.items);
        const ids = ['save', 'test', 'search', 'publish', 'settings', 'delete', 'keys'];
        ids.forEach(id => items.find(i => i.id === id).action());
        await settle();
        eq('a1008 1.3 Save, Test, Search, Publish, Flow settings, Delete and Shortcuts do nothing while the step dialog is open',
            [count.saves, count.searches, h.dialogs.length - dialogs, h.confirms.length, testRequests(h), !!editor.ed.detail, h.logged], [0, 0, 0, 0, 0, true, []]);
        // Closed, the step dialog no longer holds them back.
        editor.el.querySelector('[data-ed-detail-close]').fire('click');
        items.find(i => i.id === 'keys').action();
        eq('a1008 1.3 once the step dialog closed, the menu works again', [!!editor.ed.detail, h.dialogs.length - dialogs], [false, 1]);
        editor.dispose();
    });

    await guardAsync('a1008 1.3 a note typed into the step dialog is saved before the editor leaves', async () => {
        const { h, editor, detail } = await stepDialog('Win32');
        const note = noteField(detail);
        note.focus();
        note.value = 'Call Ada first';
        note.fire('input'); // saved 400 ms after the last key, unless something flushes it
        const left = await editor.leave();
        const put = h.puts().pop();
        const saved = put ? (put.body.doc.nodes.find(n => n.id === A).settings || {}).notes : null;
        eq('a1008 1.3 leave() saves the pending note (the step dialog stays open until the editor goes)',
            [left, saved, editor.ed.saver.state, !!editor.ed.detail, h.logged], [true, 'Call Ada first', 'saved', true, []]);
        editor.dispose();
    });

    await guardAsync('a1008 1.3 review: a note typed just before the page goes away reaches the emergency copy', async () => {
        const rows = [];
        for (const how of ['pagehide', 'hidden']) {
            const { h, editor, detail } = await stepDialog('Win32');
            const note = noteField(detail);
            note.focus();
            note.value = 'Call Ada first';
            note.fire('input'); // less than 400 ms before the page goes away
            if (how === 'pagehide') (h.winListeners.pagehide || []).slice().forEach(fn => fn({ type: 'pagehide' }));
            else {
                h.dom.document.visibilityState = 'hidden';
                h.fireDoc('visibilitychange', {});
            }
            const copy = h.store.has(DRAFT_KEY) ? JSON.parse(h.store.get(DRAFT_KEY)) : null;
            const node = copy && copy.doc && copy.doc.nodes.find(n => n.id === A);
            rows.push([how, node ? (node.settings || {}).notes || null : null, h.logged]);
            editor.dispose();
        }
        eq('a1008 1.3 review: pagehide and a hidden page write the pending note into the emergency copy',
            rows, [['pagehide', 'Call Ada first', []], ['hidden', 'Call Ada first', []]]);
    });

    // ── 1.4: Apply in the flow settings keeps the maximum run time ──

    // settingsDialog opens f1 with the draft's settings (max_run_seconds: seconds; none when
    // undefined) and its flow settings; fields reads [minutes, seconds] as shown.
    async function settingsDialog(seconds) {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
        const editor = openEditor(h, { flow: { draft: flowDoc(seconds === undefined ? null : { max_run_seconds: seconds }) } });
        await settle();
        const dialog = h.ED.dialogs.flowSettings(editor.ed);
        let result = 'open';
        dialog.done.then(value => { result = value; });
        const field = name => dialog.body.querySelector('[data-ed-set="' + name + '"]');
        // A field with validity.badInput reads as empty, as a browser's number field does for
        // text it cannot parse.
        ['max_run_minutes', 'max_run_seconds'].forEach(name => { if (field(name)) field(name).validity = { badInput: false }; });
        return {
            h, editor, dialog, field,
            fields: () => ['max_run_minutes', 'max_run_seconds'].map(name => (field(name) ? field(name).value : null)),
            result: () => result,
            async apply() { dialog.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle(); }
        };
    }
    const maxRun = editor => {
        const settings = editor.ed.model.doc.settings || {};
        return Object.prototype.hasOwnProperty.call(settings, 'max_run_seconds') ? settings.max_run_seconds : 'none';
    };
    // BAD is input a number field cannot parse: value '' and validity.badInput.
    const BAD = { bad: true };
    const typeInto = (field, value) => {
        if (value === BAD) { field.value = ''; field.validity = { badInput: true }; } else field.value = value;
    };

    await guardAsync('a1008 1.4 Apply without touching the run time keeps it to the second', async () => {
        const rows = [];
        for (const seconds of [10, 90, 1800, 86400, undefined]) {
            const d = await settingsDialog(seconds);
            const shown = d.fields();
            d.field('name').value = 'Renamed';
            await d.apply();
            rows.push([seconds === undefined ? 'none' : seconds, shown, d.result(), d.editor.ed.model.doc.name, maxRun(d.editor), d.h.logged]);
            d.editor.dispose();
        }
        eq('a1008 1.4 10 s, 90 s, 30 min, 24 h and no value show as minutes and seconds and stay as they were when only the name changes', rows, [
            [10, ['0', '10'], 'apply', 'Renamed', 10, []],
            [90, ['1', '30'], 'apply', 'Renamed', 90, []],
            [1800, ['30', '0'], 'apply', 'Renamed', 1800, []],
            [86400, ['1440', '0'], 'apply', 'Renamed', 86400, []],
            ['none', ['30', '0'], 'apply', 'Renamed', 'none', []]
        ]);
    });

    await guardAsync('a1008 1.4 the run time fields match the server range, read 0 as a value and refuse what is out of range visibly', async () => {
        const probe = await settingsDialog(1800);
        const attrs = ['max_run_minutes', 'max_run_seconds'].map(name => ['min', 'max', 'step'].map(a => probe.field(name).getAttribute(a)));
        probe.editor.dispose();
        const rows = [];
        const cases = [['2', '5'], ['0', '1'], ['1440', '0'], ['', '45'], ['3', ''], ['', ''], ['0', '0'], ['1441', '0'], ['1440', '1'], ['0', '60'], ['-1', '0'], ['1.5', '0'], ['0', '0.5'],
            [BAD, ''], [BAD, '30'], ['5', BAD]];
        const shownAs = v => (v === BAD ? 'bad' : v);
        for (const [minutes, seconds] of cases) {
            const d = await settingsDialog(600);
            typeInto(d.field('max_run_minutes'), minutes);
            typeInto(d.field('max_run_seconds'), seconds);
            await d.apply();
            const err = d.dialog.body.querySelector('[data-ed-max-run-error]');
            const shownError = !!err && err.hidden === false ? err.textContent : '';
            const invalid = ['max_run_minutes', 'max_run_seconds'].map(name => String(d.field(name).getAttribute('aria-invalid')));
            rows.push([shownAs(minutes) + ':' + shownAs(seconds), d.result(), maxRun(d.editor), shownError, invalid.join(',')]);
            d.editor.dispose();
        }
        const ok = (input, seconds) => [input, 'apply', seconds, '', 'null,null'];
        const refused = input => [input, 'open', 600, 'flow_max_run_invalid', 'true,true'];
        eq('a1008 1.4 minutes 0-1440 and seconds 0-59 in whole numbers', attrs, [['0', '1440', '1'], ['0', '59', '1']]);
        eq('a1008 1.4 typed run times: 1 s to 24 h is kept to the second, both fields empty drop the value (the server\'s 30-minute default), anything else, also input the browser cannot parse, is refused with a visible error', rows, [
            ok('2:5', 125), ok('0:1', 1), ok('1440:0', 86400), ok(':45', 45), ok('3:', 180), ok(':', 'none'),
            refused('0:0'), refused('1441:0'), refused('1440:1'), refused('0:60'), refused('-1:0'), refused('1.5:0'), refused('0:0.5'),
            refused('bad:'), refused('bad:30'), refused('5:bad')
        ]);
    });

    await guardAsync('a1008 1.4 review: the run time error describes both fields while it shows', async () => {
        const d = await settingsDialog(600);
        const described = () => ['max_run_minutes', 'max_run_seconds'].map(name => d.field(name).getAttribute('aria-describedby'));
        const err = d.dialog.body.querySelector('[data-ed-max-run-error]');
        const before = described();
        d.field('max_run_minutes').value = '0';
        d.field('max_run_seconds').value = '0';
        await d.apply();
        const shown = [described(), err.hidden, err.id];
        d.field('max_run_minutes').value = '1';
        d.field('max_run_minutes').fire('input');
        const cleared = [described(), err.hidden, ['max_run_minutes', 'max_run_seconds'].map(name => d.field(name).getAttribute('aria-invalid'))];
        const hint = 'ed-max-run-w1-hint';
        const both = hint + ' ed-max-run-w1-error';
        eq('a1008 1.4 review: aria-describedby names the error while it shows, and only the hint once typing clears it',
            [before, shown, cleared, d.h.logged], [[hint, hint], [[both, both], false, 'ed-max-run-w1-error'], [[hint, hint], true, [null, null]], []]);
        d.editor.dispose();
    });

    await guardAsync('a1008 1.4 the run time strings exist in every locale', async () => {
        eq('a1008 1.4 seconds unit, range hint and range error in all 16 locales',
            missingKeys(apps, ['easydrag.ui.unit_seconds', 'easydrag.ui.flow_max_run_hint', 'easydrag.ui.flow_max_run_invalid']), []);
    });

    // ── 1.5: run updates keep the focus on a card's tools ──

    await guardAsync('a1008 1.5 a run event that changes a card keeps the focus on its tool button and redraws only the status', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        ed.selection.add(A);
        ed.bus.emit('selection', ed.selection);
        const card = editor.el.querySelector('[data-node-id="' + A + '"]');
        // rebuilds counts the card's innerHTML writes.
        let rebuilds = 0;
        const setHTML = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(card), 'innerHTML').set;
        Object.defineProperty(card, 'innerHTML', { configurable: true, set(value) { rebuilds++; setHTML.call(this, value); } });
        const runWith = (status, step) => {
            ed.run = { id: 'r1', status, mode: 'test', steps: new Map(step ? [[A, Object.assign({ node_id: A }, step)]] : []), record: null, error: '' };
            ed.bus.emit('run', ed.run);
        };
        const states = [
            ['running', { status: 'running' }],
            ['success', { status: 'success', duration_ms: 1200 }],
            ['error', { status: 'error', duration_ms: 900, error_code: 'FLOW_TOOL_ERROR', error_message: 'boom' }],
            ['none', null]
        ];
        const rows = [];
        for (const tool of ['test', 'disable', 'duplicate', 'delete']) {
            const btn = card.querySelector('[data-ed-node-tool="' + tool + '"]');
            btn.focus();
            for (const [name, step] of states) {
                runWith(step ? 'running' : 'success', step);
                const status = card.querySelector('.ed-status');
                const error = card.querySelector('.ed-node-error');
                rows.push([tool, name, h.dom.document.activeElement === btn && card.contains(btn), btn.getAttribute('tabindex'),
                    card.classList.contains('status-' + name), status ? status.getAttribute('title') : null, error ? error.textContent : null]);
            }
        }
        const expected = tool => [
            [tool, 'running', true, '0', true, 'status_running', null],
            [tool, 'success', true, '0', true, 'status_success · 1.2 s', null],
            [tool, 'error', true, '0', true, 'status_error · 900 ms', 'error_flow_tool_error: boom'],
            [tool, 'none', true, '0', false, null, null]
        ];
        const statusRebuilds = rebuilds;
        // An edit of the step still redraws its card.
        ed.model.setParam(A, 'query', 'changed');
        eq('a1008 1.5 each tool button keeps the focus and its tab stop through running, success, error and a new run; status, duration and error show',
            [rows, statusRebuilds, rebuilds - statusRebuilds, h.logged], [['test', 'disable', 'duplicate', 'delete'].flatMap(expected), 0, 1, []]);
        editor.dispose();
    });

    await guardAsync('a1008 1.5 review: an open issue shows and goes in place, the tools keep the focus', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        ed.selection.add(A);
        ed.bus.emit('selection', ed.selection);
        const card = editor.el.querySelector('[data-node-id="' + A + '"]');
        let rebuilds = 0;
        const setHTML = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(card), 'innerHTML').set;
        Object.defineProperty(card, 'innerHTML', { configurable: true, set(value) { rebuilds++; setHTML.call(this, value); } });
        const btn = card.querySelector('[data-ed-node-tool="test"]');
        btn.focus();
        const badge = () => !!card.querySelector('.ed-badge--issue');
        const focused = () => h.dom.document.activeElement === btn && card.contains(btn);
        const rows = [[badge(), focused()]];
        ed.issues = [{ node_id: A, severity: 'error', code: 'PARAM_REQUIRED', message: 'query is required' }];
        ed.bus.emit('issues', ed.issues);
        rows.push([badge(), focused()]);
        ed.issues = [];
        ed.bus.emit('issues', ed.issues);
        rows.push([badge(), focused()]);
        eq('a1008 1.5 review: the issue badge comes and goes without a card rebuild, the focused tool stays',
            [rows, rebuilds, h.logged], [[[false, true], [true, true], [false, true]], 0, []]);
        editor.dispose();
    });

    await guardAsync('a1008 1.5 review: Disable from the keyboard keeps the focus on the tool of the redrawn card', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        ed.selection.add(A);
        ed.bus.emit('selection', ed.selection);
        const card = editor.el.querySelector('[data-node-id="' + A + '"]');
        const tool = () => card.querySelector('[data-ed-node-tool="disable"]');
        const rows = [];
        for (let i = 0; i < 2; i++) {
            const before = tool();
            before.focus();
            before.fire('click'); // Enter or Space on a focused button
            const now = h.dom.document.activeElement;
            const focusAfterDisable = now === tool() && card.contains(now);
            rows.push([!!ed.model.node(A).settings.disabled, focusAfterDisable, now !== before, now ? now.getAttribute('tabindex') : null]);
        }
        eq('a1008 1.5 review: off and on again, the new Disable button has the focus and stays a tab stop',
            [rows, h.logged], [[[true, true, true, '0'], [false, true, true, '0']], []]);
        editor.dispose();
    });

    await guardAsync('a1008 1.5 review: a rebuilt card that is no longer selected gives a tool\'s focus to the canvas', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        ed.model.setParam(A, 'query', 'changed'); // something Ctrl+Z takes back, which rebuilds the card
        ed.selection.add(A);
        ed.bus.emit('selection', ed.selection);
        const card = editor.el.querySelector('[data-node-id="' + A + '"]');
        const canvasEl = editor.el.querySelector('.ed-canvas');
        const tool = card.querySelector('[data-ed-node-tool="disable"]');
        tool.focus();
        // Escape deselects the card; the focus stays on its tool, which is no tab stop any more.
        h.fireDoc('keydown', h.key('Escape', tool));
        const afterEscape = [ed.selection.size, h.dom.document.activeElement === tool, tool.getAttribute('tabindex')];
        h.fireDoc('keydown', h.key('z', tool, { ctrlKey: true }));
        const now = h.dom.document.activeElement;
        eq('a1008 1.5 review: focus, Escape, Ctrl+Z: the undone card is redrawn and the focus goes to the canvas, not to a tool outside the tab order',
            [afterEscape, ed.model.node(A).params.query, card.contains(tool), now === canvasEl, now && now.dataset ? now.dataset.edNodeTool || null : null, h.logged],
            [[0, true, '-1'], 'x', false, true, null, []]);
        editor.dispose();
    });

    // ── 1.6: the template preview names template errors in the user's language ──

    // translator is t over one locale's words, with {name} placeholders filled in.
    const translator = words => (key, params) => {
        const text = Object.prototype.hasOwnProperty.call(words, key) ? words[key] : key;
        return params ? text.replace(/\{(\w+)\}/g, (m, name) => (params[name] !== undefined ? String(params[name]) : m)) : text;
    };
    // TPL_CASES: [query value, key of the reason, its params]; null: no code, the generic reason.
    const deep = (() => { let v = '{{alpha.text}}'; for (let i = 0; i < 32; i++) v = [v]; return v; })();
    const TPL_CASES = [
        ['{{alpha | nope}}', 'easydrag.ui.tpl_error_unknown_filter', { name: 'nope' }],
        ['{{alpha.text | default(foo)}}', 'easydrag.ui.tpl_error_expected_value', { name: 'default' }],
        ['{{1bad}}', 'easydrag.ui.tpl_error_expected_name', {}],
        ['{{alpha.}}', 'easydrag.ui.tpl_error_expected_name', {}],
        ['{{alpha |}}', 'easydrag.ui.tpl_error_expected_name', {}],
        ['Hallo {{alpha.text', 'easydrag.ui.tpl_error_unclosed', {}],
        ['{{alpha[1.5]}}', 'easydrag.ui.tpl_error_bad_index', {}],
        ['{{alpha ~}}', 'easydrag.ui.tpl_error_unexpected', { text: '~' }],
        ['{{alpha beta}}', 'easydrag.ui.tpl_error_unexpected', { text: 'beta' }],
        ['{{alpha | upper(1)}}', 'easydrag.ui.tpl_error_filter_args', { name: 'upper' }],
        ['{{alpha | truncate(0)}}', 'easydrag.ui.tpl_error_filter_args', { name: 'truncate' }],
        ['{{alpha | nope(1,)}}', 'easydrag.ui.tpl_error_filter_args', { name: 'nope' }],
        ['{{alpha.text | date("DD.MM.YYYY")}}', 'easydrag.ui.tpl_error_wrong_value', { name: 'date' }],
        ['{{alpha.text | round}}', 'easydrag.ui.tpl_error_wrong_value', { name: 'round' }],
        [deep, 'easydrag.ui.tpl_error_too_large', {}],
        ['{{alpha.when | date("DD", "Mars/Olympus")}}', null, null]
    ];

    // previews renders alpha's query with each case under the translator t and reads the
    // preview: [class, text, English engine message].
    async function previews(t) {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = Object.assign(Object.create(editor.ed), { t });
        const roots = { alpha: { text: 'kein Datum', when: '2026-10-04T07:30:00Z' } };
        const rows = TPL_CASES.map(([value]) => {
            let english = '';
            try { h.ED.template.evaluate(value, roots); } catch (err) { english = err.message; }
            const form = h.ED.forms.render({
                ed, node: { id: 'n_previewx', key: 'preview', type: 'web.search', label: 'Preview', position: { x: 0, y: 0 }, params: { query: value }, settings: {} },
                info: types.get('web.search'), upstream: [], roots, issues: [], onChange: () => {}
            });
            const box = form.el.querySelector('.ed-field[data-param="query"] .ed-field-preview');
            form.dispose();
            return [box.className, box.textContent, english];
        });
        editor.dispose();
        return { rows, logged: h.logged };
    }

    await guardAsync('a1008 1.6 the template preview shows the reason in German, never the engine\'s English text', async () => {
        const de = translator(readLang(apps, 'de'));
        const got = await previews(de);
        const want = TPL_CASES.map(([, key, params]) => ['ed-field-preview is-error', de('easydrag.ui.preview_failed', { reason: key ? de(key, params) : de('easydrag.ui.error_generic') })]);
        eq('a1008 1.6 de: each template error reads as its German sentence; an error without a code reads "Etwas ist schiefgelaufen."',
            [got.rows.map(r => r.slice(0, 2)), got.logged], [want, []]);
        eq('a1008 1.6 de: no preview contains the engine\'s English message', got.rows.filter(r => !r[2] || r[1].includes(r[2])).map(r => r[1]), []);
    });

    await guardAsync('a1008 1.6 the template error sentences exist in every locale and read well in English', async () => {
        const keys = Array.from(new Set(TPL_CASES.filter(c => c[1]).map(c => c[1])));
        eq('a1008 1.6 nine tpl_error sentences in all 16 locales', [keys.length, missingKeys(apps, keys)], [9, []]);
        const en = translator(readLang(apps, 'en'));
        const got = await previews(en);
        eq('a1008 1.6 en: the preview says what is wrong', got.rows.slice(0, 2).map(r => r[1]).concat(got.rows[got.rows.length - 1][1]), [
            'Not computable: There is no filter “nope”.',
            'Not computable: Filter “default”: each argument must be a text in quotes, a number, true, false or null.',
            'Not computable: Something went wrong.'
        ]);
    });
}
