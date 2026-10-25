// b5 checks (EasyDrag fix batch 5, 2026-10-25): the editor UX, accessibility and i18n follow-ups of the
// audits of 2026-10-23 and 2026-10-25 (reports/easydrag-audit-2026-10-25.md sections 4 to 6.3) and the
// run-event rendering cost (F6). Each block names its finding. They run on the c1d07 sandbox
// (test-easydrag-extra3.mjs returns sandbox and openEditor, whose EventSource stub lists its streams in
// h.sources); test-easydrag.mjs calls run(env) with its helpers and saverHarness and counts the failures.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

// flowDoc is a flow start -> alpha -> beta, as in test-easydrag-extra3.mjs.
function flowDoc(name) {
    return {
        schema: 1, name: name || 'Flow',
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
}

// chainDoc is a manual trigger followed by count-1 web searches in one line.
function chainDoc(count) {
    const nodes = [{ id: 'n_t0000000', key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} }];
    const edges = [];
    for (let i = 1; i < count; i++) {
        const id = 'n_s' + String(i).padStart(7, '0');
        nodes.push({ id, key: 'step_' + i, type: 'web.search', label: 'Step ' + i, position: { x: i * 300, y: 0 }, params: { query: 'q' + i }, settings: {} });
        edges.push({ id: 'e_' + i, source: { node: nodes[i - 1].id, port: 'out' }, target: { node: id, port: 'in' } });
    }
    return { schema: 1, name: 'Chain', nodes, edges };
}

// flowCard is a start-page list entry.
function flowCard(id, name, extra) {
    return Object.assign({ id, name, description: '', triggers: ['trigger.manual'], preview: [], published: true, enabled: true, last_run: null }, extra);
}

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });
const deferred = () => { let resolve, reject; const p = new Promise((a, b) => { resolve = a; reject = b; }); return { p, resolve, reject }; };

// missingKeys lists locale:key for each key that a locale lacks or leaves empty.
function missingKeys(apps, keys) {
    return LOCALES.flatMap(locale => {
        const words = JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));
        return keys.filter(key => typeof words[key] !== 'string' || !words[key].trim()).map(key => locale + ':' + key);
    });
}

export async function run(env) {
    const { apps, types, eq, guardAsync, settle, sandbox, openEditor, saverHarness } = env;
    const read = file => fs.readFileSync(path.join(apps, file), 'utf8');
    const css = () => fs.readFileSync(path.join(apps, '..', '..', '..', 'css', 'desktop-app-easydrag.css'), 'utf8');
    const lang = locale => JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));

    // session opens f1 and returns helpers: pick (select steps), key (a key on the canvas), card (a
    // card element), heard (what the live region says after an action, once its announcement is due).
    async function session(answer, opts) {
        const h = sandbox(answer || (() => undefined));
        const editor = openEditor(h, opts);
        await settle();
        const ed = editor.ed;
        const live = editor.el.querySelector('[data-ed-live]');
        const canvasEl = editor.el.querySelector('.ed-canvas');
        return {
            h, editor, ed, canvasEl,
            pick(...ids) { ed.selectedEdge = null; ed.selection.clear(); ids.forEach(id => ed.selection.add(id)); ed.bus.emit('selection', ed.selection); },
            key(k, extra) { return h.fireDoc('keydown', h.key(k, canvasEl, extra)); },
            card: id => editor.el.querySelector('[data-node-id="' + id + '"]'),
            async heard(act) { live.textContent = ''; await act(); await settle(); h.runTimers(30); return live.textContent; }
        };
    }

    // homePage opens the start page on a sandbox; openMenu opens a card's ⋯ menu, item finds an
    // entry of the last context menu by label, ids lists the flow cards shown.
    async function homePage(answer) {
        const h = sandbox(answer);
        let items = [];
        h.ctx.showContextMenu = (x, y, list) => { items = list; };
        const home = h.ED.home.create({ ctx: h.ctx, t: env.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: id => h.opened.push(id) });
        h.body.appendChild(home.el);
        await settle();
        return {
            h, home,
            ids: () => home.el.querySelectorAll('[data-ed-flow]').map(c => c.dataset.edFlow),
            item: label => items.find(i => i.label === label),
            openMenu: id => home.el.querySelector('[data-ed-card-menu="' + id + '"]').fire('click')
        };
    }

    await guardAsync('b5 harness', async () => {
        eq('b5 the batch-5 checks run on the c1d07 sandbox', [typeof sandbox, typeof openEditor, typeof saverHarness], ['function', 'function', 'function']);
    });

    // ── next checks go above this line ──
    void [T1, A, B, chainDoc, flowCard, apiError, deferred, missingKeys, read, css, lang, session, homePage, types, flowDoc];
}
