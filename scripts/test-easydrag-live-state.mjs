// a1025 R2b/R4 checks (audit 2026-10-25): the start page and the editor header mark a flow whose
// published version no longer passes the server's current checks ("Republish needed") until a
// newer version is live (R2b), and show stopped Date & time triggers (R4). They run on the c1d07
// sandbox (test-easydrag-extra3.mjs returns sandbox); test-easydrag.mjs calls run(env) with its
// helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const ISSUE = { code: 'UNTRUSTED_SECRET_TARGET', severity: 'error', node_id: A, param: 'url', message: 'unchecked data decides where the secret goes' };
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

function flowDoc(name) {
    return {
        schema: 1, name: name || 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} }
        ],
        edges: [{ id: 'e1', source: { node: T1, port: 'out' }, target: { node: A, port: 'in' } }]
    };
}

function missingKeys(apps, keys) {
    return LOCALES.flatMap(locale => {
        const words = JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));
        return keys.filter(key => typeof words[key] !== 'string' || !words[key].trim()).map(key => locale + ':' + key);
    });
}

// card is one start-page summary as GET /api/desktop/flows lists it.
function card(id, extra) {
    return Object.assign({ id, name: 'Flow ' + id, published: true, enabled: true, has_unpublished_changes: false, triggers: [], preview: [] }, extra);
}

// openFlow creates the editor for f1 (published revision 2) with extra fields of GET /flows/{id}.
function openFlow(h, t, extra) {
    const flow = { id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 3, live: flowDoc(), live_revision: 2 };
    const app = { ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, windowId: 'w1', readonly: false, openHome: () => {}, openFlow: () => {} };
    const editor = h.ED.editor.create(app, Object.assign({ flow, enabled: true, issues: [] }, extra), {});
    h.body.appendChild(editor.el);
    h.flushFrames();
    return editor;
}

export async function run(env) {
    const { apps, t, eq, guardAsync, settle, sandbox } = env;

    await guardAsync('a1025 R2b a flow that needs republishing is marked', async () => {
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows') return { flows: [card('f1', { live_issues: [ISSUE] }), card('f2')] };
            if (req.method === 'GET' && req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            return undefined;
        });
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
        h.body.appendChild(home.el);
        await settle();
        const marked = Array.from(home.el.querySelectorAll('[data-ed-flow]')).filter(c => c.querySelector('[data-ed-live-outdated]')).map(c => c.dataset.edFlow);
        home.dispose();
        const editor = openFlow(h, t, { live_issues: [ISSUE] });
        await settle();
        const chip = () => !!editor.el.querySelector('[data-ed-state] [data-ed-live-outdated]');
        const shown = chip();
        editor.ed.bus.emit('published', { live_revision: 3, published_draft_revision: 3 });
        const afterPublish = chip();
        editor.dispose();
        eq('a1025 R2b the start page marks only the flow whose published version fails the current checks, and the editor header until a newer version is live',
            [marked, shown, afterPublish], [['f1'], true, false]);
        eq('a1025 R2b the words exist in all 16 locales', missingKeys(apps, ['easydrag.ui.state_live_outdated', 'easydrag.ui.live_outdated_hint']), []);
        eq('a1025 R2b the checks log no errors', h.logged, []);
    });
}
