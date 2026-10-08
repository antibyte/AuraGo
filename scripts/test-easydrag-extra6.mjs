// a1008 checks (audit 2026-10-08, the editor findings): the test dialog confirms effects only for
// a test that started; the step dialog blocks the editor's shortcuts and window menus like any
// other dialog, and its pending note is saved before the editor leaves. They run on the c1d07
// sandbox (test-easydrag-extra3.mjs returns sandbox, openEditor and desktopMenus);
// test-easydrag.mjs calls run(env) with its helpers and counts the failures.

const A = 'n_aaaaaaaa';
const EFFECTS_KEY = 'aurago.easydrag.effects-ok.f1';

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });

export async function run(env) {
    const { types, eq, guardAsync, settle, sandbox, openEditor, desktopMenus } = env;

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

    await guardAsync('a1008 1.3 under the step dialog Ctrl+S, Ctrl+Enter and Ctrl+K do nothing and Mod+S never reaches the browser', async () => {
        const outcomes = [];
        for (const [platform, modifier] of [['MacIntel', 'metaKey'], ['Win32', 'ctrlKey']]) {
            // Focus on a button of the dialog (the desktop dispatches all three menu keys) and in
            // the note (the desktop dispatches only Mod+S; the editor's handler sees Enter and K).
            for (const where of ['button', 'note']) {
                const { h, editor, detail } = await stepDialog(platform);
                const target = where === 'note' ? noteField(detail) : detail.querySelector('[data-ed-detail-close]');
                target.focus();
                const count = counters(editor);
                const dialogs = h.dialogs.length;
                const desktop = desktopMenus(h.menus, []);
                let saveKey = null;
                // Ctrl+K first: before the fix Ctrl+Enter opened the test dialog, under which K waits.
                for (const key of ['k', 's', 'Enter']) {
                    const event = {
                        type: 'keydown', key, code: key === 'Enter' ? 'Enter' : 'Key' + key.toUpperCase(), target, repeat: false,
                        ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, defaultPrevented: false,
                        preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}
                    };
                    event[modifier] = true;
                    desktop.handleWindowMenuShortcut(event); // the desktop's keydown listener runs first
                    h.fireDoc('keydown', event); // then the editor's
                    if (key === 's') saveKey = event.defaultPrevented;
                }
                await settle();
                outcomes.push([platform, where, count.saves, count.searches, h.dialogs.length - dialogs, testRequests(h), saveKey,
                    !!editor.ed.detail, h.dom.document.activeElement === target, h.logged]);
                editor.dispose();
            }
        }
        eq('a1008 1.3 with the step dialog open nothing saves, no test dialog opens, the palette search keeps no focus, Mod+S is prevented (macOS, Windows)',
            outcomes, ['MacIntel', 'Win32'].flatMap(p => ['button', 'note'].map(w => [p, w, 0, 0, 0, 0, true, true, true, []])));
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
}
