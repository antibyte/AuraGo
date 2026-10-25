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
    await guardAsync('b5 F13 an editor that fails late in its build leaves nothing behind: no listeners, no close guard, no refresh', async () => {
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return { flow: { id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3 }, enabled: false, issues: [] };
            return undefined;
        });
        const guards = [];
        const ctx = Object.assign({}, h.ctx, { api: h.transport, flowId: 'f1', setWindowBeforeClose: (id, fn) => { guards.push(fn ? 'set' : 'clear'); } });
        const makeWires = h.ED.wires.create;
        h.ED.wires.create = (...args) => { const w = makeWires(...args); w.render = () => { throw new Error('late failure'); }; return w; };
        const container = h.body.appendChild(new h.dom.El('div', {}));
        h.win.EasyDragApp.render(container, 'w1', ctx);
        await settle();
        h.flushFrames();
        const failed = [!!container.querySelector('.ed-shell-error'), h.logged.length];
        const before = h.requests.length;
        const notes = h.notes.length;
        const homes = h.opened.length;
        ['deleted', 'published'].forEach(reason => h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason } }));
        h.fireDoc('keydown', h.key('a', container, {}));
        await settle();
        h.runTimers(2000);
        await settle();
        const listeners = ['aurago:flows-changed', 'keydown', 'keyup'].map(type => (h.docListeners[type] || []).length);
        eq('b5 F13 a late build failure releases listeners and the close guard; nothing is fetched, shown or opened afterwards',
            [failed, guards, h.requests.length - before, h.notes.length - notes, h.opened.length - homes, listeners.slice(0, 1)],
            [[true, 1], ['set', 'clear'], 0, 0, 0, [0]]);
        h.ED.wires.create = makeWires;
        container.querySelector('[data-ed-shell="retry"]').fire('click');
        await settle();
        h.flushFrames();
        const inst = h.win.EasyDragApp._instances.get('w1');
        eq('b5 F13 Try again after a late failure builds a working editor; only the expected error was logged',
            [!!(inst && inst.screen && inst.screen.ed), h.logged.length, guards], [true, 1, ['set', 'clear', 'set']]);
        h.win.EasyDragApp.dispose('w1');
    });
    // ── U5: required, invalid and JSON-error states ──

    await guardAsync('b5 U5 required fields, issues and JSON errors reach assistive technology', async () => {
        const reqTypes = new Map(types);
        reqTypes.set('web.search', Object.assign({}, types.get('web.search'), { params: [
            { name: 'query', kind: 'text', label: 'Query', required: true },
            { name: 'count', kind: 'number', label: 'Count', default: 5 },
            { name: 'payload', kind: 'json', label: 'Payload' }
        ] }));
        const h = sandbox(() => undefined);
        h.catalog = { types: reqTypes, list: Array.from(reqTypes.values()), categories: [] };
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        ed.issues = [{ node_id: B, param: 'query', code: 'PARAM_REQUIRED', severity: 'error', message: 'required' }];
        ed.bus.emit('open-detail', { nodeId: B });
        const id = name => 'ed-p-w1-' + B + '-' + name;
        const byId = x => editor.el.querySelector('[id="' + x + '"]');
        const attrs = n => ['aria-required', 'aria-invalid', 'aria-describedby'].map(a => n.getAttribute(a));
        const view = () => byId(id('query')).parentNode.querySelector('.ed-tpl-view');
        const query = [attrs(view()), attrs(byId(id('query'))), !!byId(id('query') + '-issue')];
        const count = attrs(byId(id('count')));
        ed.issues = [];
        ed.bus.emit('issues', ed.issues);
        const cleared = attrs(view());
        const area = byId(id('payload'));
        area.focus();
        area.value = '{bad';
        area.fire('input');
        const error = byId(id('payload') + '-error');
        const bad = [attrs(area).slice(1), error.getAttribute('role'), error.hidden];
        area.value = '{}';
        area.fire('input');
        const good = [attrs(area).slice(1), error.hidden];
        eq('b5 U5 aria-required on the required field, aria-invalid and aria-describedby to the issue while it shows, a JSON error as an alert linked to its field',
            [query, count, cleared, bad, good, h.logged], [
                [['true', 'true', id('query') + '-issue'], ['true', 'true', id('query') + '-issue'], true],
                [null, null, null],
                ['true', null, null],
                [['true', id('payload') + '-error'], 'alert', false],
                [[null, null], true],
                []
            ]);
        editor.dispose();
    });

    // ── U4, U6: step list region and roles ──

    await guardAsync('b5 U4 U6 the step list is a region beside the canvas; roles are whole and named', async () => {
        const setupTypes = new Map(types);
        setupTypes.set('notify.mail', { type: 'notify.mail', label: 'Mail', category: 'notify', inputs: ['in'], outputs: ['out'], params: [], availability: { state: 'needs_setup', config_section: 'email' } });
        const h = sandbox(() => undefined);
        h.catalog = { types: setupTypes, list: Array.from(setupTypes.values()), categories: [{ id: 'notify', label: 'Notify' }] };
        const editor = openEditor(h);
        await settle();
        h.runTimers(400);
        const ed = editor.ed;
        const region = editor.el.querySelector('.ed-node-region');
        const listButton = region.querySelector('[data-ed-focus-node]');
        listButton.fire('click');
        const u4 = [region.localName, region.getAttribute('aria-label'), region.parentNode === editor.el.querySelector('.ed-body'), !!editor.el.querySelector('.ed-canvas .ed-node-list'), Array.from(ed.selection)];
        const link = editor.el.querySelector('.ed-palette [data-ed-setup]');
        const palette = [link.localName, link.getAttribute('href'), link.getAttribute('role'), !!link.closest('[data-ed-type]')];
        ed.bus.emit('quick-add', { x: 0, y: 0 });
        const quick = editor.el.querySelector('.ed-quick ul').html;
        const quickRow = [quick.includes('data-ed-setup'), quick.includes('role="link"')];
        ed.quickAdd.close();
        ed.bus.emit('find-step');
        const find = h.dialogs[h.dialogs.length - 1];
        const findList = find.el.parentNode.html.includes('role="list"');
        find.close(null);
        ed.bus.emit('connect-picker', { nodeId: A });
        const pick = h.dialogs[h.dialogs.length - 1];
        const pickList = pick.el.parentNode.html.includes('role="list"');
        pick.close(null);
        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click');
        const drawerGroup = editor.el.querySelector('.ed-drawer [role="radiogroup"]').getAttribute('aria-label');
        ed.bus.emit('open-detail', { nodeId: A });
        const tabs = editor.el.querySelectorAll('.ed-detail [role="tab"]').map(b => {
            const p = editor.el.querySelector('[id="' + b.getAttribute('aria-controls') + '"]');
            return !!p && p.getAttribute('role') === 'tabpanel' && p.getAttribute('aria-labelledby') === b.id;
        });
        const groups = editor.el.querySelectorAll('.ed-detail [role="radiogroup"]').map(g => g.getAttribute('aria-label'));
        const tools = editor.el.querySelector('[data-node-id="' + A + '"] .ed-node-tools').getAttribute('aria-label');
        eq('b5 U4 U6 region beside the canvas; setup link of its own; no list roles without items; named groups; tabs with panels',
            [u4, palette, quickRow, findList, pickList, drawerGroup, tabs, groups, tools, h.logged], [
                ['section', 'node_list', true, false, [listButton.dataset.edFocusNode]],
                ['a', '/config#email', null, false], [false, false], false, false, 'runs_filter_label',
                [true, true, true, true, true, true], ['detail_output', 'output_view_label'], 'node_tools:Alpha', []
            ]);
        eq('b5 U6 the new names exist in all 16 locales', missingKeys(apps, ['easydrag.ui.runs_filter_label', 'easydrag.ui.output_view_label', 'easydrag.ui.node_tools']), []);
        editor.dispose();
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
    // ── G3: the version of a viewed run was cleaned up ──

    await guardAsync('b5 G3 a run whose version was cleaned up says so in the run view', async () => {
        const rows = [];
        for (const missing of ['version_pruned', 'not_stored']) {
            const s = await session(req => (req.url === '/api/desktop/flows/runs/r1?include=doc'
                ? { run: { id: 'r1', status: 'success', mode: missing === 'not_stored' ? 'test' : 'live', started_at: '2026-10-25T10:00:00Z', revision: 7 }, steps: [], doc_missing: missing } : undefined));
            s.editor.showRun('r1');
            await settle();
            rows.push([missing, s.h.notes.map(n => [n.title, n.message, n.type]), !!s.ed.runView, s.h.logged]);
            s.editor.dispose();
        }
        eq('b5 G3 a pruned version names the version and the 50-version rule; any other missing document keeps the general text', rows, [
            ['version_pruned', [['runs_title', 'run_view_pruned:7', 'error']], false, []],
            ['not_stored', [['runs_title', 'run_view_missing', 'error']], false, []]
        ]);
        eq('b5 G3 the words exist in all 16 locales', missingKeys(apps, ['easydrag.ui.run_view_pruned']), []);
    });
    // ── U12: header Undo/Redo and the deletion toast ──

    await guardAsync('b5 U12 header Undo and Redo, and a toast that takes a deletion back', async () => {
        const s = await session();
        const head = s.editor.el.querySelector('[data-ed-history-host]');
        const undo = head.querySelector('[data-ed-history="undo"]');
        const redo = head.querySelector('[data-ed-history="redo"]');
        const off = b => b.getAttribute('aria-disabled') === 'true';
        const names = [undo.getAttribute('aria-label'), redo.getAttribute('aria-label'), head.getAttribute('aria-label'), head.getAttribute('role')];
        const fresh = [off(undo), off(redo)];
        s.ed.model.setLabel(A, 'Renamed');
        const edited = [off(undo), off(redo)];
        const heardUndo = await s.heard(() => undo.fire('click'));
        const undone = [s.ed.model.node(A).label, off(undo), off(redo)];
        await s.heard(() => redo.fire('click'));
        const redone = s.ed.model.node(A).label;
        const toast = s.editor.el.querySelector('[data-ed-toast]');
        s.pick(B);
        s.key('Delete');
        const shown = [toast.hidden, toast.querySelector('[data-ed-toast-text]').textContent, !!s.ed.model.node(B)];
        toast.querySelector('[data-ed-toast-undo]').fire('click');
        const back = [toast.hidden, !!s.ed.model.node(B)];
        s.pick(B);
        s.key('Delete');
        s.ed.model.setLabel(A, 'Other');
        const closedByEdit = toast.hidden;
        s.ed.model.undo();
        s.ed.model.undo();
        s.pick(B);
        s.key('Delete');
        const beforeTimer = toast.hidden;
        s.h.runTimers(8000);
        eq('b5 U12 named buttons that follow the history, an Undo toast after a delete that any other change or 8 s close',
            [names, fresh, edited, heardUndo, undone, redone, shown, back, closedByEdit, beforeTimer, toast.hidden, s.h.logged],
            [['undo', 'redo', 'history_label', 'group'], [true, true], [false, true], 'announce_undo', ['Alpha', true, false], 'Renamed',
                [false, 'announce_deleted_one:Beta', false], [true, true], true, false, true, []]);
        s.editor.dispose();
        const view = await session(req => (req.url === '/api/desktop/flows/runs/r1?include=doc'
            ? { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-25T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') } : undefined));
        view.ed.model.setLabel(A, 'Edited');
        view.editor.showRun('r1');
        await settle();
        const host = view.editor.el.querySelector('[data-ed-history-host]');
        const inView = ['undo', 'redo'].map(k => host.querySelector('[data-ed-history="' + k + '"]').getAttribute('aria-disabled'));
        view.editor.dispose();
        const ro = await session(undefined, { readonly: true });
        eq('b5 U12 both buttons are off in the run view; a read-only desktop shows no history group',
            [inView, ro.editor.el.querySelector('[data-ed-history-host]').hidden], [['true', 'true'], true]);
        ro.editor.dispose();
        eq('b5 U12 the group name exists in all 16 locales', missingKeys(apps, ['easydrag.ui.history_label']), []);
    });
    // ── U15: test again with the data of a viewed run ──

    await guardAsync('b5 U15 Test again with this data opens the test dialog with the run\'s trigger data', async () => {
        const viewed = extra => ({ run: Object.assign({ id: 'r1', status: 'error', mode: 'live', started_at: '2026-10-25T10:00:00Z', revision: 2, trigger_node: T1, trigger_data: { name: 'Ada', token: '[redacted]' } }, extra), steps: [], doc: flowDoc('Run') });
        const s = await session(req => {
            if (req.url === '/api/desktop/flows/runs/r1?include=doc') return viewed();
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test') return { run_id: 'r9' };
            return undefined;
        });
        s.editor.showRun('r1');
        await settle();
        const button = s.editor.el.querySelector('[data-ed-cmd="rerun-viewed"]');
        const offered = button.hidden;
        const dialogs = s.h.dialogs.length;
        button.fire('click');
        await settle();
        const dialog = s.h.dialogs[s.h.dialogs.length - 1];
        const html = dialog.el.parentNode.html;
        const opened = [s.ed.runView, s.h.dialogs.length - dialogs, html.includes('Ada'), dialog.body.querySelector('[data-ed-test-remember]').checked,
            !!dialog.body.querySelector('[data-ed-rerun-redacted]'), s.h.requests.some(r => r.url.includes('test-data'))];
        // The browser's text area holds the shown JSON; the run sends it although it is unchanged.
        dialog.body.querySelector('[data-ed-test-data]').value = JSON.stringify({ name: 'Ada', token: '[redacted]' }, null, 2);
        dialog.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const post = s.h.requests.find(r => r.method === 'POST' && r.url === '/api/desktop/flows/f1/test');
        eq('b5 U15 the run view leaves, the dialog shows the data, says what is hidden, remembers nothing by default and sends the data',
            [offered, opened, post && [post.body.trigger_node, post.body.trigger_data, post.body.remember_data, post.body.base_revision], s.h.logged],
            [false, [null, 1, true, false, true, false], [T1, { name: 'Ada', token: '[redacted]' }, false, 3], []]);
        s.editor.dispose();
        const hiddenFor = async (opts, run) => {
            const v = await session(req => (req.url === '/api/desktop/flows/runs/r1?include=doc' ? viewed(run) : undefined), opts);
            v.editor.showRun('r1');
            await settle();
            const hidden = v.editor.el.querySelector('[data-ed-cmd="rerun-viewed"]').hidden;
            v.editor.dispose();
            return hidden;
        };
        eq('b5 U15 no button on a read-only desktop, without trigger data, or for a trigger the draft no longer has',
            [await hiddenFor({ readonly: true }, {}), await hiddenFor({}, { trigger_data: undefined }), await hiddenFor({}, { trigger_node: 'n_gonegone' })], [true, true, true]);
        eq('b5 U15 the words exist in all 16 locales', missingKeys(apps, ['easydrag.ui.run_rerun_test', 'easydrag.ui.rerun_redacted']), []);
    });
    // ── V1: run-view header ──

    await guardAsync('b5 V1 the run view header says Past run once and hides a Saved chip that means nothing there', async () => {
        const s = await session(req => (req.url === '/api/desktop/flows/runs/r1?include=doc'
            ? { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-25T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') } : undefined));
        const head = () => {
            const state = s.editor.el.querySelector('[data-ed-state]').html;
            const rev = s.editor.el.querySelector('[data-ed-revisions]');
            const save = s.editor.el.querySelector('[data-ed-save]');
            return [(state.match(/state_run_view/g) || []).length + (rev.textContent === 'state_run_view' ? 1 : 0), !!rev.hidden, !!save.hidden];
        };
        const before = head();
        s.editor.showRun('r1');
        await settle();
        const inView = head();
        s.editor.el.querySelector('[data-ed-cmd="exit-run-view"]').fire('click');
        eq('b5 V1 draft: no Past run; run view: once, revision line and Saved hidden; back: as before',
            [before, inView, head(), s.h.logged], [[0, false, false], [1, true, true], [0, false, false], []]);
        s.editor.dispose();
    });
    // ── V4, U13: phone header ──

    await guardAsync('b5 V4 U13 a phone header moves Runs and Test into the more menu and keeps Paused in view', async () => {
        const s = await session(undefined, { enabled: false, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });
        let items = [];
        s.h.ctx.showContextMenu = (x, y, list) => { items = list; };
        const labels = () => items.map(i => (i.separator ? '|' : i.label));
        const more = s.editor.el.querySelector('[data-ed-cmd="more"]');
        more.fire('click');
        const wide = labels();
        s.editor.el.getBoundingClientRect = () => ({ left: 0, top: 0, right: 390, bottom: 800, width: 390, height: 800 });
        more.fire('click');
        const phone = labels();
        items.find(i => i.label === 'runs_title').action();
        const drawer = !!s.editor.el.querySelector('.ed-drawer');
        const dialogs = s.h.dialogs.length;
        await items.find(i => i.label === 'test_draft').action();
        await settle();
        const testOpened = s.h.dialogs.length - dialogs;
        const paused = s.editor.el.querySelector('[data-ed-state]').html.includes('data-ed-paused');
        const sheet = css();
        const narrow = sheet.slice(sheet.indexOf('@container (max-width: 560px)')).split(/\r?\n\}\r?\n/)[0];
        eq('b5 V4 U13 wide: Run now first; phone: Runs and Test first and working; Paused chip marked; the 560 px rules hide the three buttons and only the non-paused chips',
            [wide.slice(0, 2), phone.slice(0, 4), drawer, testOpened, paused,
                narrow.includes('.ed-head-actions [data-ed-cmd="runs"], .ed-head-actions [data-ed-cmd="test"], .ed-head-actions [data-ed-cmd="run-now"] { display: none; }'),
                narrow.includes('.ed-state > :not([data-ed-paused]) { display: none; }'), narrow.includes('.ed-state { display: none; }'), s.h.logged],
            [['run_now', 'versions_title'], ['runs_title', 'test_draft', '|', 'run_now'], true, 1, true, true, true, false, []]);
        s.editor.dispose();
    });
    // ── next checks go above this line ──
    void [T1, A, B, chainDoc, flowCard, apiError, deferred, missingKeys, read, css, lang, session, homePage, types, flowDoc];
}
