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

    // ── G2: Run now under concurrency "skip" while a run goes on ──

    await guardAsync('b5 G2 a skipped Run now says so, in the editor and on the start page', async () => {
        const s = await session(req => (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run' ? { status: 'skipped' } : undefined),
            { enabled: true, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });
        const said = await s.heard(() => s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click'));
        const editorRow = [s.h.notes.map(n => [n.title, n.message, n.type]), said, s.h.sources.length, !!s.ed.run, s.h.logged];
        s.editor.dispose();
        const p = await homePage(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: [flowCard('f1', 'Alpha')] };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.url === '/api/desktop/flows/f1/run' && req.method === 'POST') return { status: 'skipped' };
            return undefined;
        });
        p.openMenu('f1');
        await p.item('run_now').action();
        await settle();
        eq('b5 G2 the editor and a card menu name the skip; no run is followed', [editorRow, p.h.notes.map(n => [n.title, n.message]), p.h.logged], [
            [[['run_now', 'run_now_skipped', undefined]], 'run_now_skipped', 0, false, []],
            [['Alpha', 'run_now_skipped']], []
        ]);
        eq('b5 G2 the notice exists in all 16 locales', missingKeys(apps, ['easydrag.ui.run_now_skipped']), []);
        p.home.dispose();
    });

    // ── F12: quick-add whose wire or source step is gone ──

    await guardAsync('b5 F12 quick-add on a wire or step that is gone adds nothing, closes and says why', async () => {
        const rows = [];
        for (const [name, req, remove] of [
            ['wire', { x: 0, y: 0, edge: 'e2' }, ed => ed.model.disconnect(['e2'])],
            ['step', { x: 0, y: 0, from: { node: B, port: 'out' } }, ed => ed.model.removeNodes([B])]
        ]) {
            const s = await session();
            s.ed.bus.emit('quick-add', req);
            const opened = !!s.ed.quickAdd;
            remove(s.ed);
            const nodes = s.ed.model.doc.nodes.length;
            const said = await s.heard(() => s.editor.el.querySelector('.ed-quick-search').fire('keydown', { key: 'Enter', keyCode: 13, isComposing: false }));
            rows.push([name, opened, s.ed.quickAdd, s.ed.model.doc.nodes.length - nodes, !!s.editor.el.querySelector('.ed-quick'), said, s.h.logged]);
            s.editor.dispose();
        }
        eq('b5 F12 the popover closes without a step and the live region says the target is gone', rows,
            [['wire', true, null, 0, false, 'quick_target_gone', []], ['step', true, null, 0, false, 'quick_target_gone', []]]);
        eq('b5 F12 the words exist in all 16 locales', missingKeys(apps, ['easydrag.ui.quick_target_gone']), []);
    });

    // ── F13: start page ──

    await guardAsync('b5 F13 templates that fail leave the flows listed and offer their own Try again', async () => {
        let templatesFail = true;
        const p = await homePage(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: [flowCard('f1', 'Alpha')] };
            if (req.url.startsWith('/api/desktop/flows/templates')) {
                if (templatesFail) throw apiError('FLOW_INTERNAL');
                return { templates: [{ id: 't1', name: 'Tpl', description: 'd', categories: [] }] };
            }
            return undefined;
        });
        const grid = p.home.el.querySelector('.ed-flow-grid');
        const tpl = p.home.el.querySelector('.ed-template-grid');
        const failed = [p.ids(), grid.html.includes('ed-error'), tpl.html.includes('home_templates_failed:error_flow_internal'), !!tpl.querySelector('[data-ed-templates-retry]'), p.home.createOff()];
        templatesFail = false;
        tpl.querySelector('[data-ed-templates-retry]').fire('click');
        await settle();
        const retried = [p.ids(), !!tpl.querySelector('[data-ed-template="t1"]'), (tpl.html || '').includes('home_templates_failed'), p.h.requests.filter(r => r.url.startsWith('/api/desktop/flows/templates')).length];
        eq('b5 F13 a template failure shows the flows, a template error and its Try again, which loads the templates',
            [failed, retried, p.h.logged], [[['f1'], false, true, true, false], [['f1'], true, false, 2], []]);
        eq('b5 F13 the template words exist in all 16 locales', missingKeys(apps, ['easydrag.ui.home_templates_failed']), []);
        p.home.dispose();
    });

    await guardAsync('b5 F13 a double Duplicate on a start-page card reads and copies the flow once', async () => {
        const hold = deferred();
        const p = await homePage(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: [flowCard('f1', 'Alpha')] };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.url === '/api/desktop/flows/f1' && req.method === 'GET') return hold.p;
            if (req.url === '/api/desktop/flows' && req.method === 'POST') return { flow: { id: 'f2' } };
            return undefined;
        });
        p.openMenu('f1');
        const first = p.item('home_duplicate').action();
        const second = p.item('home_duplicate').action();
        hold.resolve({ flow: { id: 'f1', draft: flowDoc('Alpha') } });
        await first;
        await second;
        await settle();
        const gets = p.h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/f1').length;
        const posts = p.h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows').length;
        eq('b5 F13 one read, one copy, one flow opened', [gets, posts, p.h.opened, p.h.logged], [1, 1, ['f2'], []]);
        p.home.dispose();
    });
    // ── F13: editor and window shell ──

    await guardAsync('b5 F13 a double Duplicate in the editor creates one copy', async () => {
        const s = await session(req => (req.method === 'POST' && req.url === '/api/desktop/flows' ? { flow: { id: 'f2' } } : undefined));
        let items = [];
        s.h.ctx.showContextMenu = (x, y, list) => { items = list; };
        s.editor.el.querySelector('[data-ed-cmd="more"]').fire('click');
        const dup = items.find(i => i.label === 'home_duplicate');
        const first = dup.action();
        const second = dup.action();
        await first;
        await second;
        await settle();
        const posts = s.h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows').length;
        eq('b5 F13 two Duplicate clicks post one copy and open it once', [posts, s.h.opened, s.h.logged], [1, ['f2'], []]);
        s.editor.dispose();
    });

    await guardAsync('b5 F13 an editor that cannot be built shows the error card with Try again, not an empty window', async () => {
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return { flow: { id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3 }, enabled: false, issues: [] };
            return undefined;
        });
        const create = h.ED.editor.create;
        h.ED.editor.create = () => { throw new Error('broken document'); };
        const container = h.body.appendChild(new h.dom.El('div', {}));
        h.win.EasyDragApp.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport, flowId: 'f1' }));
        await settle();
        h.flushFrames();
        const shown = [!!container.querySelector('.ed-shell-error'), !!container.querySelector('[data-ed-shell="retry"]'), h.logged.length, String(h.logged[0] || '').startsWith('EasyDrag editor failed')];
        h.ED.editor.create = create;
        container.querySelector('[data-ed-shell="retry"]').fire('click');
        await settle();
        h.flushFrames();
        const inst = h.win.EasyDragApp._instances.get('w1');
        eq('b5 F13 the error card offers Try again, and Try again opens the editor', [shown, !!(inst && inst.screen && inst.screen.ed)], [[true, true, 1, true], true]);
        h.win.EasyDragApp.dispose('w1');
    });
    // ── F13: storage ──

    await guardAsync('b5 F13 a refused emergency copy backs off and then stops; a write that lands resets it', async () => {
        const h = saverHarness(null);
        const storage = h.core.storage;
        const realSet = storage.set.bind(storage);
        let refuse = true;
        let sets = 0;
        storage.set = (key, value) => {
            if (String(key).startsWith('aurago.easydrag.draft.')) { sets++; if (refuse) return false; }
            return realSet(key, value);
        };
        // dropSave removes the 1 s save timer a change arms, so only the copy timers are left.
        const dropSave = () => { for (const [id, entry] of Array.from(h.timers)) if (entry.ms === 1000 && !h.copyFirst) { h.timers.delete(id); break; } };
        h.change('A');
        dropSave();
        const delays = [];
        for (let i = 0; i < 8 && h.timers.size; i++) {
            const ms = Array.from(h.timers.values())[0].ms;
            delays.push(ms);
            await h.fire(ms);
        }
        const stopped = [delays, sets, h.timers.size];
        h.change('B');
        dropSave();
        const meanwhile = [sets, h.timers.size];
        refuse = false;
        h.saver.flushCopy();
        const landed = [sets, h.copy() && h.copy().doc.name];
        h.change('C');
        eq('b5 F13 refused copies wait 0.5, 1, 2, 4 and 8 s, then no timer; a change meanwhile writes nothing; flushCopy writes; the next change holds back 500 ms again',
            [stopped, meanwhile, landed, h.delays().includes(500)], [[[500, 1000, 2000, 4000, 8000], 6, 0], [6, 0], [7, 'B'], true]);
    });

    await guardAsync('b5 F13 the clipboard fragment in localStorage carries its time and expires after 24 h', async () => {
        const KEY = 'aurago.easydrag.clipboard';
        const s = await session();
        s.pick(A);
        s.key('c', { ctrlKey: true });
        const kept = JSON.parse(s.h.store.get(KEY));
        const stamped = [typeof kept.at, kept.frag && kept.frag.easydrag, kept.frag && kept.frag.nodes.length];
        const nodes = () => s.ed.model.doc.nodes.length;
        const before = nodes();
        kept.at -= 25 * 60 * 60 * 1000;
        s.h.store.set(KEY, JSON.stringify(kept));
        s.key('v', { ctrlKey: true });
        await settle();
        const expired = [nodes() - before, s.h.store.has(KEY)];
        s.h.store.set(KEY, JSON.stringify(kept.frag));
        s.key('v', { ctrlKey: true });
        await settle();
        const legacy = [nodes() - before, s.h.store.has(KEY)];
        s.pick(A);
        s.key('c', { ctrlKey: true });
        s.key('v', { ctrlKey: true });
        await settle();
        eq('b5 F13 a stamped copy pastes; an expired or unstamped one is dropped unpasted', [stamped, expired, legacy, nodes() - before, s.h.logged],
            [['number', 1, 1], [0, false], [0, false], 1, []]);
        s.editor.dispose();
    });
    // ── exitRunView and the docked palette ──

    await guardAsync('b5 leaving the run view shows the docked palette by the stored choice, also after a narrow start', async () => {
        const h = sandbox(req => (req.url === '/api/desktop/flows/runs/r1?include=doc'
            ? { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-25T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') } : undefined));
        // getComputedStyle is a global of the sandbox's realm (reached through a sandbox function's
        // Function constructor); the palette floats (position absolute) while narrow is true.
        const realm = h.ED.core.el.constructor('return this')();
        let narrow = true;
        realm.getComputedStyle = node => ({ position: narrow && node.classList && node.classList.contains('ed-palette') ? 'absolute' : 'static', display: 'block', visibility: 'visible', transitionDuration: '0s' });
        const editor = openEditor(h);
        await settle();
        const palette = editor.el.querySelector('.ed-palette');
        const open = () => !palette.classList.contains('is-collapsed');
        const start = open();
        editor.showRun('r1');
        await settle();
        h.flushFrames();
        const inView = open();
        narrow = false;
        editor.el.querySelector('[data-ed-cmd="exit-run-view"]').fire('click');
        h.flushFrames();
        const after = open();
        narrow = true;
        editor.showRun('r1');
        await settle();
        h.flushFrames();
        editor.el.querySelector('[data-ed-cmd="exit-run-view"]').fire('click');
        eq('b5 narrow start: closed, hidden in the run view, back when docked on exit; a floating palette stays closed after the run view',
            [start, inView, after, open(), h.logged], [false, false, true, false, []]);
        editor.dispose();
    });
    // ── next checks go above this line ──
    void [T1, A, B, chainDoc, flowCard, apiError, deferred, missingKeys, read, css, lang, session, homePage, types, flowDoc];
}
