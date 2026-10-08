// a1008 checks (audit 2026-10-08, the editor findings): the test dialog confirms effects only for
// a test that started. They run on the c1d07 sandbox (test-easydrag-extra3.mjs returns sandbox and
// openEditor); test-easydrag.mjs calls run(env) with its helpers and counts the failures.

const A = 'n_aaaaaaaa';
const EFFECTS_KEY = 'aurago.easydrag.effects-ok.f1';

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });

export async function run(env) {
    const { types, eq, guardAsync, settle, sandbox, openEditor } = env;

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
}
