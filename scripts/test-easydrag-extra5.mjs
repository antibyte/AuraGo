// FF2 checks: the editor follows deletes and switches made elsewhere, Run now waits while a flow
// is paused, EasyDrag uses the state words of Mission Control. They run on the c1d07 sandbox
// (test-easydrag-extra3.mjs returns sandbox and openEditor); test-easydrag.mjs calls run(env)
// with its helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const DRAFT_KEY = 'aurago.easydrag.draft.f1';
const VIEW_KEY = 'aurago.easydrag.view.f1';
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

const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });

// chips lists the texts of the editor's state chips.
function chips(editor) {
    return Array.from(editor.el.querySelector('[data-ed-state]').html.matchAll(/<span class="ed-chip ed-chip--\w+"[^>]*>([^<]*)<\/span>/g)).map(m => m[1]);
}

function readLang(apps, area, locale) {
    return JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', area, locale + '.json'), 'utf8'));
}

export async function run(env) {
    const { apps, types, t, eq, guardAsync, settle, sandbox, openEditor } = env;

    // published opens f1 as a published flow (live revision 1, draft revision 3 = published).
    // world.exists and world.enabled are what GET /flows/f1 answers; Run now answers FLOW_DISABLED
    // while the flow is paused. The more menu's items land in h.more.
    function publishedEditor(world) {
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') {
                if (!world.exists) throw apiError('FLOW_NOT_FOUND');
                return { flow: { id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 3, live: flowDoc(), live_revision: 1 }, enabled: world.enabled, issues: [] };
            }
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run') {
                if (!world.enabled) throw apiError('FLOW_DISABLED');
                return { run_id: 'r9' };
            }
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/enabled') {
                world.enabled = !!req.body.enabled;
                return { enabled: world.enabled };
            }
            if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
            return undefined;
        });
        h.ctx.showContextMenu = (x, y, items) => { h.more = items; };
        const editor = openEditor(h, { flow: { published_draft_revision: 3, live: flowDoc(), live_revision: 1 }, enabled: world.enabled });
        return { h, editor };
    }
    const gets = h => h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/f1').length;
    const changed = (h, flowId, reason) => h.fireDoc('aurago:flows-changed', { detail: { flow_id: flowId, reason } });
    // runNow reads Run now in the Flow menu and in the ⋯ menu: [disabled, hint] each.
    const runNow = (h, editor) => {
        const item = h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'run');
        editor.el.querySelector('[data-ed-cmd="more"]').fire('click');
        const more = h.more.find(i => i.label === 'run_now');
        return [[!!item.disabled, item.disabledHint || ''], [!!more.disabled, more.disabledHint || '']];
    };

    await guardAsync('ff2 I1 switches made elsewhere reach the open editor', async () => {
        const world = { exists: true, enabled: true };
        const { h, editor } = publishedEditor(world);
        await settle();
        const rows = [['open', chips(editor), runNow(h, editor), gets(h)]];
        // Mission Control pauses the flow: the broadcast names it (FF1).
        world.enabled = false;
        changed(h, 'f1', 'enabled');
        await settle();
        rows.push(['paused here', chips(editor), runNow(h, editor), gets(h), editor.el.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')]);
        // Another flow's switch, and a save without an id, are not this flow's news.
        changed(h, 'f9', 'enabled');
        changed(h, '', 'saved');
        await settle();
        rows.push(['other', gets(h)]);
        // A switch without an id (no single flow held the mission) may be this flow: read again.
        world.enabled = true;
        changed(h, '', 'enabled');
        await settle();
        rows.push(['maybe mine', chips(editor), runNow(h, editor), gets(h), editor.el.querySelector('[data-ed-cmd="active"]').getAttribute('aria-checked')]);
        eq('ff2 I1 a pause made elsewhere shows Paused and turns Run now off with its hint; a switch without an id is read again', rows, [
            ['open', ['state_published'], [[false, ''], [false, '']], 0],
            ['paused here', ['state_published', 'state_inactive'], [[true, 'error_flow_disabled'], [true, 'error_flow_disabled']], 1, 'false'],
            ['other', 1],
            ['maybe mine', ['state_published'], [[false, ''], [false, '']], 2, 'true']
        ]);
        // A Run now that raced the pause (a stale menu) shows the server's FLOW_DISABLED in words.
        world.enabled = false;
        await h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'run').action();
        await settle();
        eq('ff2 a refused Run now of a paused flow names the reason', [h.notes.map(n => n.message), h.logged], [['error_flow_disabled'], []]);
        editor.dispose();
        // The Active switch turned off here does the same.
        const s = publishedEditor({ exists: true, enabled: true });
        await settle();
        s.editor.el.querySelector('[data-ed-cmd="active"]').fire('click');
        await settle();
        const posted = s.h.requests.find(r => r.url === '/api/desktop/flows/f1/enabled');
        eq('ff2 switching the flow off here turns Run now off too', [posted && posted.body.enabled, chips(s.editor), runNow(s.h, s.editor), s.h.logged],
            [false, ['state_published', 'state_inactive'], [[true, 'error_flow_disabled'], [true, 'error_flow_disabled']], []]);
        s.editor.dispose();
    });

    await guardAsync('ff2 I1 a delete made elsewhere sends the editor home and drops its local copies', async () => {
        const outcomes = [];
        for (const how of ['named', 'without an id', 'without an id, still there']) {
            const world = { exists: true, enabled: true };
            const { h, editor } = publishedEditor(world);
            await settle();
            h.runTimers(400);
            editor.ed.model.setFlow({ description: 'unsaved' });
            h.store.set(VIEW_KEY, JSON.stringify({ cx: 1, cy: 2, zoom: 1 }));
            const had = [h.store.has(DRAFT_KEY), h.store.has(VIEW_KEY)];
            if (how !== 'without an id, still there') world.exists = false;
            changed(h, how === 'named' ? 'f1' : '', 'deleted');
            await settle();
            // Later edits and timers save nothing and write no copy under the dead id.
            editor.ed.model.setFlow({ description: 'after' });
            h.runTimers();
            await settle();
            outcomes.push([how, had, h.store.has(DRAFT_KEY), h.store.has(VIEW_KEY), h.homes.length, h.notes.map(n => n.message), h.puts().length, gets(h), h.logged]);
        }
        eq('ff2 I1 a flow deleted elsewhere (named, or found gone after a broadcast without an id) goes home once, keeps no copy and saves nothing', outcomes, [
            ['named', [true, true], false, false, 1, ['flow_deleted_elsewhere'], 0, 0, []],
            ['without an id', [true, true], false, false, 1, ['flow_deleted_elsewhere'], 0, 1, []],
            ['without an id, still there', [true, true], false, true, 0, [], 1, 1, []]
        ]);
    });

    await guardAsync('ff2 the start page: Paused lists switched-off published flows, Run now waits while paused', async () => {
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') {
                return { flows: [
                    { id: 'f1', name: 'On', published: true, enabled: true, triggers: [], preview: [] },
                    { id: 'f2', name: 'Paused', published: true, enabled: false, triggers: [], preview: [] },
                    { id: 'f3', name: 'Never', published: false, enabled: false, triggers: [], preview: [] }
                ] };
            }
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            return undefined;
        });
        let menu = null;
        h.ctx.showContextMenu = (x, y, items) => { menu = items; };
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
        h.body.appendChild(home.el);
        await settle();
        const shown = () => home.el.querySelectorAll('[data-ed-flow]').map(c => c.getAttribute('data-ed-flow'));
        const filters = {};
        for (const f of ['all', 'active', 'inactive']) {
            home.el.querySelector('[data-ed-filter="' + f + '"]').fire('click');
            filters[f] = shown();
        }
        home.el.querySelector('[data-ed-filter="all"]').fire('click');
        const runItems = ['f1', 'f2', 'f3'].map(id => {
            home.el.querySelector('[data-ed-card-menu="' + id + '"]').fire('click');
            const item = menu.find(i => i.label === 'run_now');
            return [id, !!item.disabled, item.disabledHint || ''];
        });
        const badges = home.el.querySelector('.ed-flow-grid').html.match(/ed-chip--muted">[^<]*/g);
        eq('ff2 the Paused filter lists published flows that are off; Run now is off with its hint for a paused flow; a never published flow reads "Not published yet"',
            [filters, runItems, badges],
            [{ all: ['f1', 'f2', 'f3'], active: ['f1'], inactive: ['f2'] }, [['f1', false, ''], ['f2', true, 'error_flow_disabled'], ['f3', true, '']], ['ed-chip--muted">state_draft']]);
        home.dispose();
    });

    await guardAsync('ff2 one vocabulary: EasyDrag says Paused and Not published yet like Mission Control, and Stop', async () => {
        const rows = LOCALES.map(locale => {
            const ed = readLang(apps, 'easydrag', locale);
            const desk = readLang(apps, 'desktop', locale);
            return [locale, ed['easydrag.ui.state_draft'] === ed['easydrag.ui.active_needs_publish_title'], !!ed['easydrag.ui.state_inactive'] && ed['easydrag.ui.state_inactive'] !== ed['easydrag.ui.home_filter_active'],
                !!desk['desktop.mc_flow_cancel_in_easydrag']];
        });
        const en = readLang(apps, 'easydrag', 'en');
        const enDesk = readLang(apps, 'desktop', 'en');
        eq('ff2 every locale: the never-published chip is EasyDrag\'s "Not published yet", the paused chip is its own word',
            rows, LOCALES.map(locale => [locale, true, true, true]));
        eq('ff2 en: Paused and Not published yet as in Mission Control; the MC hint names Stop',
            [en['easydrag.ui.state_inactive'], en['easydrag.ui.home_filter_inactive'], en['easydrag.ui.state_draft'], enDesk['desktop.mc_state_paused'], enDesk['desktop.mc_flow_unpublished'], enDesk['desktop.mc_flow_cancel_in_easydrag']],
            ['Paused', 'Paused', 'Not published yet', 'Paused', 'Not published yet', "No running flow run was found. Stop waiting runs in EasyDrag's run list."]);
    });

    await guardAsync('ff2 the desktop draws a disabled item\'s disabledHint as its tooltip', async () => {
        const routing = fs.readFileSync(path.join(apps, '..', 'core', 'menus-and-routing.js'), 'utf8').replace(/\r\n?/g, '\n');
        const between = (a, b) => routing.slice(routing.indexOf(a), routing.indexOf(b, routing.indexOf(a)));
        const box = vm.createContext({ esc: s => String(s).replace(/"/g, '&quot;'), menuLabel: item => item.label || item.labelKey, iconMarkup: () => '' });
        vm.runInContext(between('function normalizeWindowMenuItems(', 'function normalizeWindowMenus(') + between('function renderWindowMenuItems(', 'function setWindowMenus(') +
            '\nglobalThis.menu = { normalizeWindowMenuItems, renderWindowMenuItems };', box);
        const items = box.menu.normalizeWindowMenuItems([
            { id: 'run', label: 'Run now', disabled: true, disabledHint: 'Switch it "on" first', action() {} },
            { id: 'test', label: 'Test', disabledHint: 'unused while enabled', action() {} }
        ], 'flow', new Map(), ['w1', 'flow']);
        const html = box.menu.renderWindowMenuItems(items);
        eq('ff2 a disabled window-menu item carries its hint as title (escaped); an enabled one none; context menus do the same',
            [(html.match(/title="[^"]*"/g) || []), /item\.disabledHint \? ` title="\$\{esc\(item\.disabledHint\)\}"`/.test(between('function showContextMenu(', 'function showDesktopContextMenu('))],
            [['title="Switch it &quot;on&quot; first"'], true]);
    });

    // ── commit 2: state and performance ──

    // bigDoc is a trigger and n-1 search steps in rows of 20, each wired to the one before.
    function bigDoc(n) {
        const nodes = [{ id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} }];
        const edges = [];
        for (let i = 1; i < n; i++) {
            const id = 'n_' + String(i).padStart(8, 'a');
            nodes.push({ id, key: 'step_' + i, type: 'web.search', label: 'Step ' + i, position: { x: (i % 20) * 300, y: Math.floor(i / 20) * 160 }, params: { query: 'q' + i }, settings: {} });
            edges.push({ id: 'e' + i, source: { node: nodes[i - 1].id, port: 'out' }, target: { node: id, port: 'in' } });
        }
        return { schema: 1, name: 'Big', nodes, edges };
    }

    await guardAsync('ff2 M2 dragging a 200-step selection stays fast and writes the emergency copy rarely', async () => {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
        const editor = openEditor(h, { flow: { draft: bigDoc(200) } });
        await settle();
        h.menus.find(m => m.id === 'edit').items.find(i => i.id === 'select-all').action();
        let copies = 0;
        const set = h.store.set.bind(h.store);
        h.store.set = (k, v) => { if (k === DRAFT_KEY) copies++; return set(k, v); };
        const canvasEl = editor.el.querySelector('.ed-canvas');
        const card = editor.el.querySelector('[data-node-id="' + T1 + '"]');
        // drag moves the selection over 60 frames and returns [ms, copy writes].
        const drag = () => {
            copies = 0;
            const started = process.hrtime.bigint();
            card.fire('pointerdown', h.pe(1, 10, 10));
            for (let i = 1; i <= 60; i++) canvasEl.fire('pointermove', h.pe(1, 10 + i * 3, 10 + i));
            canvasEl.fire('pointerup', h.pe(1, 190, 70));
            return [Number(process.hrtime.bigint() - started) / 1e6, copies];
        };
        // A busy machine rarely slows three drags in a row: the fastest of up to three counts.
        let best = drag();
        for (let i = 0; i < 2 && best[0] >= 3000; i++) { const next = drag(); if (next[0] < best[0]) best = next; }
        const [ms, writes] = best;
        console.log('info ff2 M2 drag of 200 selected steps: ' + (ms / 60).toFixed(2) + ' ms per frame, ' + writes + ' copy writes');
        const moved = editor.ed.model.node(T1).position.x > 0;
        // Generous: a frame of a 200-step drag took 8.7 ms in a browser before FF2 (about 12 ms in
        // this sandbox, 9 after); 60 frames under 3 s leave room for a slow, busy machine and still
        // catch a quadratic regression.
        eq('ff2 M2 60 frames of a 200-step drag take under 3 s; the copy is written at most once per 500 ms and at the end',
            [moved, ms < 3000, writes >= 1 && writes <= 2 + Math.ceil(ms / 500), JSON.parse(h.store.get(DRAFT_KEY)).doc.nodes.find(n => n.id === T1).position, h.logged],
            [true, true, true, editor.ed.model.node(T1).position, []]);
        editor.dispose();
    });

    await guardAsync('ff2 review: a drag costs about linearly more with more steps', async () => {
        // bestDrag opens an n-step flow, selects everything and returns the fastest of three
        // 30-frame drags in ms (a busy machine rarely slows all three).
        const bestDrag = n => {
            const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [] } : undefined));
            const editor = openEditor(h, { flow: { draft: bigDoc(n) } });
            h.menus.find(m => m.id === 'edit').items.find(i => i.id === 'select-all').action();
            const canvasEl = editor.el.querySelector('.ed-canvas');
            const card = editor.el.querySelector('[data-node-id="' + T1 + '"]');
            let best = Infinity;
            for (let run = 0; run < 3; run++) {
                const started = process.hrtime.bigint();
                card.fire('pointerdown', h.pe(1, 10, 10));
                for (let i = 1; i <= 30; i++) canvasEl.fire('pointermove', h.pe(1, 10 + i * 3, 10 + i));
                canvasEl.fire('pointerup', h.pe(1, 100, 40));
                best = Math.min(best, Number(process.hrtime.bigint() - started) / 1e6);
            }
            const logged = h.logged.length;
            editor.dispose();
            return [best, logged];
        };
        bestDrag(50); // warm-up: the first editor pays for the JIT
        let [small, smallErrors] = bestDrag(100);
        let [large, largeErrors] = bestDrag(400);
        let ratio = large / Math.max(small, 0.01);
        // A pause during the large drags can still tip one measurement: one more try decides.
        if (ratio >= 8) {
            [small, smallErrors] = bestDrag(100);
            [large, largeErrors] = bestDrag(400);
            ratio = Math.min(ratio, large / Math.max(small, 0.01));
        }
        console.log('info ff2 drag of 100 vs 400 selected steps: ' + small.toFixed(1) + ' ms vs ' + large.toFixed(1) + ' ms (x' + ratio.toFixed(1) + ')');
        // Linear work grows 4x from 100 to 400 steps, quadratic 16x: under 8x leaves room for noise.
        eq('ff2 review: 4x the steps cost under 8x the time per drag (no quadratic lookups); model.node is a Map lookup that follows the document',
            [ratio < 8, smallErrors + largeErrors], [true, 0]);
        // model.node follows every write of the node list: commands, undo, redo and replaceDoc.
        const model = sandbox(() => undefined).ED.model.create(flowDoc(), { types });
        const seen = [];
        const added = model.addNode('web.search', { x: 0, y: 300 });
        seen.push(!!model.node(added));
        model.undo();
        seen.push(!!model.node(added));
        model.redo();
        seen.push(model.node(added) === model.doc.nodes.find(n => n.id === added));
        model.moveNodes([A], 10, 0);
        seen.push(model.node(A) === model.doc.nodes.find(n => n.id === A) && model.node(A).position.x === 310);
        model.replaceDoc(flowDoc('Other'));
        seen.push(!!model.node(added), model.node(A) === model.doc.nodes.find(n => n.id === A));
        eq('ff2 review: model.node stays in step with add, undo, redo, move and replaceDoc', seen, [true, false, true, true, false, true]);
    });

    // runAnswer answers the stored run r1 (with its own document) and the test endpoints.
    const runAnswer = extra => req => {
        if (req.url === '/api/desktop/flows/runs/r1?include=doc') return { run: { id: 'r1', status: 'success', mode: 'test', started_at: '2026-10-06T10:00:00Z', revision: 2 }, steps: [], doc: flowDoc('Run') };
        if (req.url === '/api/desktop/flows/f1/publish-preview') return { issues: [], effects: [] };
        if (req.url.startsWith('/api/desktop/flows/f1/test-data/')) return { data: {} };
        if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
        return extra ? extra(req) : undefined;
    };
    const count = (h, method, url) => h.requests.filter(r => r.method === method && r.url === url).length;

    await guardAsync('ff2 M1 pans and zooms are no changes: no save, no unpublished changes, no new test check', async () => {
        const h = sandbox(runAnswer(req => (req.url === '/api/desktop/flows/f1/test' ? { run_id: 'r9' } : undefined)));
        const editor = openEditor(h, { flow: { published_draft_revision: 3, live: flowDoc(), live_revision: 1 }, enabled: true });
        await settle();
        h.runTimers();
        await settle();
        const version = editor.ed.model.version;
        const canvasEl = editor.el.querySelector('.ed-canvas');
        const wheel = zoom => canvasEl.fire('wheel', { deltaX: 0, deltaY: 4, deltaMode: 0, ctrlKey: zoom, metaKey: false, shiftKey: false, clientX: 500, clientY: 300, preventDefault() {} });
        for (let i = 0; i < 20; i++) { wheel(false); wheel(true); }
        // A middle-button pan.
        canvasEl.fire('pointerdown', Object.assign(h.pe(2, 100, 100), { button: 1 }));
        canvasEl.fire('pointermove', h.pe(2, 160, 130));
        canvasEl.fire('pointerup', h.pe(2, 160, 130));
        for (let i = 0; i < 3; i++) { h.runTimers(); await settle(); }
        const panned = [h.puts().length, editor.ed.model.version === version, chips(editor), editor.ed.saver.state, h.store.has(DRAFT_KEY), h.store.has(VIEW_KEY), 'viewport' in editor.ed.model.doc];
        // The test dialog's check after the flush: a zoom before Run is no edit, so Run starts at once.
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        wheel(true);
        h.runTimers(600);
        dialog.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        eq('ff2 M1 a pan or zoom saves nothing, keeps Published and is stored per device; the test dialog runs at once after a zoom',
            [panned, count(h, 'GET', '/api/desktop/flows/f1/publish-preview'), count(h, 'POST', '/api/desktop/flows/f1/test'), h.logged],
            [[0, true, ['state_published'], 'saved', false, true, false], 1, 1, []]);
        editor.dispose();
    });

    await guardAsync('ff2 M3 a live run that starts while a run view shows waits behind it', async () => {
        let answerTest = null;
        const h = sandbox(runAnswer(req => {
            if (req.url === '/api/desktop/flows/f1/test') return new Promise(resolve => { answerTest = resolve; });
            return undefined;
        }));
        const editor = openEditor(h);
        await settle();
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        // A notification opens a stored run before the test's answer arrives.
        editor.showRun('r1');
        await settle();
        answerTest({ run_id: 'r9' });
        await settle();
        const during = [!!editor.ed.runView, editor.ed.run && editor.ed.run.id, !!(editor.ed.run && editor.ed.run.view), editor.ed.model.doc.name];
        editor.ed.exitRunView();
        await settle();
        eq('ff2 M3 the run view keeps its run; leaving it follows the parked live run',
            [during, !!editor.ed.runView, editor.ed.run && editor.ed.run.id, editor.ed.run && editor.ed.run.mode, !!(editor.ed.run && editor.ed.run.view), h.logged],
            [[true, 'r1', true, 'Run'], false, 'r9', 'test', false, []]);
        editor.dispose();
    });

    await guardAsync('ff2 M4 the hints check the draft, never a stored run\'s document', async () => {
        const h = sandbox(runAnswer());
        const editor = openEditor(h, { flow: { draft: flowDoc('Draft doc') } });
        await settle();
        const validations = () => h.requests.filter(r => r.url === '/api/desktop/flows/validate').map(r => r.body.doc.name);
        // The editor asks for hints when it opens (debounced); a run view opens before they are asked.
        editor.showRun('r1');
        await settle();
        h.runTimers(700);
        await settle();
        const inRunView = validations();
        editor.ed.draftModel.setFlow({ description: 'edited while the run shows' });
        h.runTimers(700);
        await settle();
        const afterEdit = validations();
        editor.ed.exitRunView();
        h.runTimers(700);
        await settle();
        eq('ff2 M4 no hints are asked while the run view shows; leaving it checks the draft',
            [inRunView, afterEdit, validations(), editor.ed.model === editor.ed.draftModel, h.logged], [[], [], ['Draft doc'], true, []]);
        editor.dispose();
    });

    // ── commit 3: accessibility ──

    // shellWith renders the EasyDrag window w1 for route on the sandbox: the catalog, the flow f1
    // and the list (listed: its flows) answer; extra answers first.
    async function shellWith(route, listed, extra) {
        const h = sandbox(req => {
            const own = extra ? extra(req) : undefined;
            if (own !== undefined) return own;
            if (req.url.startsWith('/api/desktop/flows/node-types')) return { node_types: Array.from(types.values()), categories: [] };
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: listed() };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return { flow: { id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: 3, published_draft_revision: 0, live_revision: 0 }, enabled: false, issues: [] };
            return undefined;
        });
        const container = h.body.appendChild(new h.dom.El('div', {}));
        h.win.EasyDragApp.render(container, 'w1', Object.assign({}, h.ctx, { api: h.transport }, route));
        await settle();
        h.flushFrames();
        return { h, container, App: h.win.EasyDragApp };
    }
    const card = id => ({ id, name: 'Flow ' + id, published: false, enabled: false, triggers: [], preview: [] });

    await guardAsync('ff2 M5 focus after screen changes: run view exit, start page, error card', async () => {
        // Leaving the run view puts the focus on the canvas.
        const r = sandbox(runAnswer());
        const editor = openEditor(r);
        await settle();
        editor.showRun('r1');
        await settle();
        const back = editor.el.querySelector('[data-ed-cmd="exit-run-view"]');
        back.focus();
        back.fire('click');
        const runView = [!!editor.ed.runView, r.dom.document.activeElement === editor.el.querySelector('.ed-canvas')];
        editor.dispose();
        // Back from the editor: New flow at once, the flow just left once the list is there.
        const outcomes = [];
        for (const [name, listed] of [['listed', () => [card('f2'), card('f1')]], ['gone', () => [card('f2')]]]) {
            const w = await shellWith({ flowId: 'f1' }, listed);
            const add = () => w.container.querySelector('.ed-home-hero [data-ed-new]');
            w.container.querySelector('.ed-editor [data-ed-cmd="home"]').fire('click');
            await settle();
            const active = w.h.dom.document.activeElement;
            outcomes.push([name, active && (active.getAttribute('data-ed-flow') || (active === add() ? 'new' : active.localName)), w.h.logged]);
            w.App.dispose('w1');
        }
        // Mission Control's New flow keeps the focus on New flow, the list's answer does not take it.
        const mc = await shellWith({ flowId: 'f1' }, () => [card('f1')]);
        mc.App.open('w1', { section: 'home' });
        await settle();
        const mcFocus = mc.h.dom.document.activeElement === mc.container.querySelector('.ed-home-hero [data-ed-new]');
        mc.App.dispose('w1');
        // A load that fails shows the error card with the focus on Retry.
        const failed = await shellWith({}, () => [], req => (req.url.startsWith('/api/desktop/flows/node-types') ? Promise.reject(apiError('FLOW_INTERNAL')) : undefined));
        const retry = failed.container.querySelector('[data-ed-shell="retry"]');
        eq('ff2 M5 the run view exit focuses the canvas; the start page focuses New flow, then the card just left; MC\'s New flow keeps New flow; an error focuses Retry',
            [runView, outcomes, mcFocus, !!retry && failed.h.dom.document.activeElement === retry],
            [[false, true], [['listed', 'f1', []], ['gone', 'new', []]], true, true]);
        failed.App.dispose('w1');
    });

    await guardAsync('ff2 M6 the start page announces a delete, not every refresh, and debounces run ends', async () => {
        let listed = [card('f1'), card('f2')];
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: listed };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.url === '/api/desktop/flows/f1' && req.method === 'DELETE') { listed = [card('f2')]; return { status: 'deleted', used_by: [] }; }
            return undefined;
        });
        let menu = null;
        h.ctx.showContextMenu = (x, y, items) => { menu = items; };
        h.confirmAnswer = true;
        const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
        h.body.appendChild(home.el);
        await settle();
        const lists = () => h.requests.filter(r => r.url === '/api/desktop/flows' && r.method === 'GET').length;
        const live = home.el.querySelector('[data-ed-home-live]');
        const politeGrid = home.el.querySelector('.ed-flow-grid').getAttribute('aria-live');
        home.el.querySelector('[data-ed-card-menu="f1"]').fire('click');
        await menu.find(i => i.label === 'home_delete').action();
        await settle();
        h.runTimers(30);
        const said = live.textContent;
        const before = lists();
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f2', reason: 'run_finished' } });
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f2', reason: 'run_finished' } });
        await settle();
        const waiting = [lists() - before, h.delays()];
        h.runTimers(1500);
        await settle();
        eq('ff2 M6 the grid is no live region; a delete is announced by name; run ends refresh once after a pause',
            [politeGrid, live.getAttribute('aria-live'), said, waiting, lists() - before, h.logged], [null, 'polite', 'home_flow_deleted:Flow f1', [0, [1500]], 1, []]);
        home.dispose();
    });

    await guardAsync('ff2 M7 only the selected step\'s tools and no screen-reader list button are tab stops', async () => {
        const h = sandbox(() => undefined);
        const doc = flowDoc();
        doc.nodes.push({ id: 'n_cccccccc', key: 'gamma', type: 'web.search', label: 'Gamma', position: { x: 900, y: 0 }, params: { query: 'z' }, settings: {} });
        doc.nodes.push({ id: 'n_dddddddd', key: 'delta', type: 'web.search', label: 'Delta', position: { x: 1200, y: 0 }, params: { query: 'w' }, settings: {} });
        const editor = openEditor(h, { flow: { draft: doc } });
        await settle();
        h.runTimers(400);
        const canvasEl = editor.el.querySelector('.ed-canvas');
        // stops lists the canvas's tab stops: the canvas, and every button or [tabindex] not set
        // to -1 outside a hidden part.
        const stops = () => [canvasEl].concat(canvasEl.querySelectorAll('button, [tabindex]')).filter(n => n.getAttribute('tabindex') !== '-1' && !n.closest('[hidden]')).length;
        const tools = id => editor.el.querySelector('[data-node-id="' + id + '"]').querySelectorAll('.ed-node-tool').map(b => b.getAttribute('tabindex'));
        const none = [stops(), tools(A), canvasEl.querySelectorAll('[data-ed-focus-node]').map(b => b.getAttribute('tabindex'))];
        h.fireDoc('keydown', h.key('ArrowRight', canvasEl));
        const one = [stops(), tools(T1), tools(A)];
        // A redrawn card (a new label) keeps its tools' tab stops.
        editor.ed.model.setLabel(T1, 'Begin');
        const redrawn = tools(T1);
        h.fireDoc('keydown', h.key('Escape', canvasEl));
        eq('ff2 M7 with 5 steps the canvas has 5 tab stops (itself and the zoom bar), 9 with one step selected; the list buttons are not tab stops',
            [none, one, redrawn, stops()],
            [[5, ['-1', '-1', '-1', '-1'], ['-1', '-1', '-1', '-1', '-1']], [9, ['0', '0', '0', '0'], ['-1', '-1', '-1', '-1']], ['0', '0', '0', '0'], 5]);
        const css = fs.readFileSync(path.join(apps, '..', '..', '..', 'css', 'desktop-app-easydrag.css'), 'utf8');
        eq('ff2 M7 a focused tool button shows its toolbar', /\.ed-node:focus-within \.ed-node-tools[^{]*\{ opacity: 1;/.test(css), true);
        editor.dispose();
    });

    // ── commit 4: leftovers ──

    await guardAsync('ff2 M9 a held dialog button gives its hour-long timer back when the dialog closes', async () => {
        const h = sandbox(() => undefined);
        const host = h.body.appendChild(new h.dom.El('div', {}));
        const limited = () => Promise.reject(Object.assign(new Error('slow'), { status: 429, retryAfter: 3600, body: { error: 'slow', code: 'FLOW_RATE_LIMITED' } }));
        const env = { t, esc: h.ED.core.esc, readonly: false, root: host, api: { secrets: () => Promise.resolve({ secrets: ['other'] }), deleteSecret: limited }, secretCache: {}, notify() {} };
        const picker = host.appendChild(h.ED.fields.secretRef(env, 'other', () => {}, 'sec-x'));
        await settle();
        picker.querySelector('[data-ed-secret-delete]').fire('click');
        const dialog = h.dialogs[h.dialogs.length - 1];
        const hours = () => h.delays().filter(ms => ms === 3600000).length;
        dialog.el.querySelector('[data-ed-action="delete"]').fire('click');
        await settle();
        const held = hours();
        // A second 429 replaces the hold instead of adding a second release.
        dialog.el.querySelector('[data-ed-action="delete"]').fire('click');
        await settle();
        const replaced = hours();
        dialog.close(null);
        await settle();
        eq('ff2 M9 one hold per dialog, ended when the dialog closes', [held, replaced, hours(), h.logged], [1, 1, 0, []]);
    });

    await guardAsync('ff2 M9 a tree drag whose row was redrawn leaves no document listener behind', async () => {
        const h = sandbox(() => undefined);
        const root = h.ED.core.el('<div class="ed-editor"></div>');
        const tree = h.ED.mapping.create({
            ed: { t, esc: h.ED.core.esc, readonly: false, root }, sourceId: 'run', onInsert: () => {},
            sources: [{ id: 'run', label: 'Run', roots: { alpha: { obj: { a: 1 } } } }],
            upstream: [{ key: 'alpha', label: 'Alpha', icon: 'search', cat: 'web', fields: [] }]
        });
        const drag = () => tree.el.querySelector('.ed-tree-row').fire('dragstart', { dataTransfer: { setData() {}, effectAllowed: '' } });
        const listening = () => [(h.docListeners.dragend || []).length, (h.docListeners.drop || []).length, root.classList.contains('is-mapping')];
        // The first drag's row is redrawn: its dragend never reaches the document. The next drag
        // ends it first; a drop anywhere ends the drag (a redrawn row's drop still arrives).
        drag();
        drag();
        const second = listening();
        h.fireDoc('drop', {});
        const dropped = listening();
        drag();
        h.fireDoc('dragend', {});
        eq('ff2 M9 one pair of document listeners per drag, removed by a drop or a dragend', [second, dropped, listening()], [[1, 1, true], [0, 0, false], [0, 0, false]]);
    });

    await guardAsync('ff2 M9 a deleted flow leaves no copy, view, confirmed effects or test trigger in this browser', async () => {
        const seed = store => ['draft', 'view', 'effects-ok', 'test-trigger'].forEach(k => store.set('aurago.easydrag.' + k + '.f1', JSON.stringify(k === 'effects-ok' ? ['sends_message'] : 'x')));
        const left = store => Array.from(store.keys()).filter(k => k.endsWith('.f1')).sort();
        const outcomes = [];
        // Deleted in the editor, and deleted elsewhere while it is open.
        for (const how of ['editor', 'elsewhere']) {
            const h = sandbox(req => (req.method === 'DELETE' ? { status: 'deleted', used_by: [] } : undefined));
            h.confirmAnswer = true;
            const editor = openEditor(h);
            await settle();
            seed(h.store);
            if (how === 'editor') await h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'delete').action();
            else h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'deleted' } });
            await settle();
            outcomes.push([how, left(h.store), h.homes.length]);
            editor.dispose();
        }
        // Deleted from a card of the start page, and broadcast as deleted while the start page shows.
        for (const how of ['card', 'broadcast']) {
            const h = sandbox(req => {
                if (req.url === '/api/desktop/flows' && req.method === 'GET') return { flows: [card('f1')] };
                if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
                if (req.method === 'DELETE') return { status: 'deleted', used_by: [] };
                return undefined;
            });
            let menu = null;
            h.ctx.showContextMenu = (x, y, items) => { menu = items; };
            h.confirmAnswer = true;
            const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
            h.body.appendChild(home.el);
            await settle();
            seed(h.store);
            h.store.set('aurago.easydrag.draft.f2', '"keep"');
            if (how === 'card') {
                home.el.querySelector('[data-ed-card-menu="f1"]').fire('click');
                await menu.find(i => i.label === 'home_delete').action();
            } else {
                h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'deleted' } });
            }
            await settle();
            outcomes.push([how, left(h.store), h.store.has('aurago.easydrag.draft.f2')]);
            home.dispose();
        }
        eq('ff2 M9 every way a flow is deleted drops its four local keys and nothing of another flow',
            outcomes, [['editor', [], 1], ['elsewhere', [], 1], ['card', [], true], ['broadcast', [], true]]);
        // core.forgetFlow's prefixes are the ones the modules write.
        const src = name => fs.readFileSync(path.join(apps, name), 'utf8');
        eq('ff2 M9 forgetFlow knows every per-flow key prefix',
            [src('easydrag-saver.js').includes("'aurago.easydrag.draft.'"), src('easydrag-editor.js').includes("'aurago.easydrag.view.'"),
                src('easydrag-runs.js').includes("'aurago.easydrag.effects-ok.'"), src('easydrag-runs.js').includes("'aurago.easydrag.test-trigger.'"),
                (src('easydrag-core.js').match(/const FLOW_KEYS = \[([^\]]*)\]/) || [])[1]],
            [true, true, true, true, "'aurago.easydrag.draft.', 'aurago.easydrag.view.', 'aurago.easydrag.effects-ok.', 'aurago.easydrag.test-trigger.'"]);
    });

    // ── FF2 review follow-up ──

    await guardAsync('ff2 review: a held-back emergency copy is written on pagehide, on hidden and 500 ms later at the latest', async () => {
        const outcomes = [];
        for (const how of ['pagehide', 'hidden', 'trailing', 'visible only']) {
            const h = sandbox(() => undefined);
            const editor = openEditor(h);
            await settle();
            const copied = () => JSON.parse(h.store.get(DRAFT_KEY)).doc.description;
            // The first change is copied at once; the next one, within 500 ms, is held back.
            editor.ed.model.setFlow({ description: 'first' });
            editor.ed.model.setFlow({ description: 'held' });
            const before = [copied(), h.delays().includes(500)];
            if (how === 'pagehide') (h.winListeners.pagehide || []).forEach(fn => fn({ type: 'pagehide' }));
            else if (how === 'trailing') h.runTimers(500);
            else {
                h.dom.document.visibilityState = how === 'hidden' ? 'hidden' : 'visible';
                h.fireDoc('visibilitychange', {});
            }
            outcomes.push([how, before, copied(), h.delays().includes(500)]);
            editor.dispose();
            outcomes.push([how + ' disposed', (h.winListeners.pagehide || []).length, (h.docListeners.visibilitychange || []).length]);
        }
        eq('ff2 review: pagehide, a hidden page and the trailing 500 ms timer write the held change; a visible page waits', outcomes, [
            ['pagehide', ['first', true], 'held', false], ['pagehide disposed', 0, 0],
            ['hidden', ['first', true], 'held', false], ['hidden disposed', 0, 0],
            ['trailing', ['first', true], 'held', false], ['trailing disposed', 0, 0],
            ['visible only', ['first', true], 'first', true], ['visible only disposed', 0, 0]
        ]);
    });

    await guardAsync('ff2 review: a delete that loses the race to a delete elsewhere still goes home; names come from the draft', async () => {
        const h = sandbox(runAnswer(req => {
            if (req.method === 'DELETE') throw apiError('FLOW_NOT_FOUND');
            return undefined;
        }));
        h.confirmAnswer = true;
        const anchors = [];
        const create = h.dom.document.createElement;
        h.dom.document.createElement = tag => { const node = create(tag); if (tag === 'a') anchors.push(node); return node; };
        const editor = openEditor(h);
        await settle();
        // The run view shows the stored run's document ("Run"); the draft is "Flow".
        editor.showRun('r1');
        await settle();
        h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'export').action();
        const exported = anchors.map(a => a.download);
        editor.ed.draftModel.setFlow({ description: 'unsaved' });
        h.store.set('aurago.easydrag.test-trigger.f1', '"n_tttttttt"');
        await h.menus.find(m => m.id === 'flow').items.find(i => i.id === 'delete').action();
        await settle();
        h.runTimers();
        await settle();
        eq('ff2 review: FLOW_NOT_FOUND on the own delete counts as deleted (home, no error, no copy, no save); export and confirm name the draft',
            [exported, h.confirms, h.homes.length, h.notes, h.store.has(DRAFT_KEY), h.store.has('aurago.easydrag.test-trigger.f1'), h.puts().length, h.logged],
            [['Flow.easydrag.json'], ['delete_text:Flow'], 1, [], false, false, 0, []]);
        // A delete elsewhere while the run view shows names the draft as well.
        const g = sandbox(runAnswer());
        const viewer = openEditor(g);
        await settle();
        viewer.showRun('r1');
        await settle();
        g.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'deleted' } });
        eq('ff2 review: the deleted-elsewhere notice names the draft, not the viewed run', g.notes, [{ title: 'Flow', message: 'flow_deleted_elsewhere' }]);
        editor.dispose();
        viewer.dispose();
    });

    // ── merge with main: a read-only desktop still stops runs (a stop is no write), nothing else ──
    await guardAsync('merge read-only desktop: the run view offers Stop; Test, Publish, the switch and Delete stay off', async () => {
        const at = '2026-10-06T10:00:00Z';
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows/runs/r2?include=doc') return { run: { id: 'r2', status: 'running', mode: 'test', started_at: at, revision: 1 }, steps: [], doc: flowDoc('Run') };
            if (req.method === 'POST' && req.url === '/api/desktop/flows/runs/r2/cancel') return { cancelled: true };
            if (req.url === '/api/desktop/flows/runs/r2') return { run: { id: 'r2', status: 'cancelled', mode: 'test', started_at: at, revision: 1 }, steps: [] };
            return undefined;
        });
        const editor = openEditor(h, { readonly: true, flow: { published_draft_revision: 3, live: flowDoc(), live_revision: 1 }, enabled: true });
        await settle();
        editor.showRun('r2');
        await settle();
        const btn = cmd => editor.el.querySelector('[data-ed-cmd="' + cmd + '"]');
        const flowMenu = h.menus.find(m => m.id === 'flow');
        const del = flowMenu && flowMenu.items.find(i => i.id === 'delete');
        const before = [!btn('stop-viewed').hidden, btn('test').disabled, btn('publish').disabled, btn('active').disabled, !!(del && del.disabled)];
        btn('stop-viewed').fire('click');
        await settle();
        const cancels = h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows/runs/r2/cancel').length;
        eq('merge read-only desktop: Stop shows in the run view and posts the cancel; Test, Publish, the switch and Delete stay off',
            [before, cancels, h.puts().length, h.notes, h.logged], [[true, true, true, true, true], 1, 0, [], []]);
        editor.dispose();
    });

    // ── audit 2026-10-08, finding 1.2: only FLOWS_DISABLED means "switched off" ──
    const A1008_CODES = ['FLOW_MISSION_CONTROL_UNAVAILABLE', 'FLOW_RUNNER_STOPPED', 'FLOW_VAULT_UNAVAILABLE', 'FLOW_REQUEST_CANCELLED'];
    await guardAsync('a1008 1.2 the other 503 causes show their error with Try again, not the switched-off card', async () => {
        const rows = [];
        for (const code of [...A1008_CODES, 'FLOWS_DISABLED']) {
            // The window while it loads (easydrag.js showError). The mini DOM keeps no text, so the
            // card's words are told by the keys it asked for after the failed load.
            const d = sandbox(() => { throw apiError(code); });
            const host = d.body.appendChild(new d.dom.El('div', {}));
            let asked = null;
            const spyT = (key, params) => { if (asked) asked.push(key.replace('easydrag.ui.', '')); return t(key, params); };
            d.win.EasyDragApp.render(host, 'a1008', Object.assign({}, d.ctx, { api: d.transport, t: spyT }));
            asked = [];
            await settle();
            const box = host.querySelector('.ed-shell-error');
            const shell = box && [asked.includes('disabled_title'), !!box.querySelector('[data-ed-shell="config"]'),
                !!box.querySelector('[data-ed-shell="retry"]'), asked.includes('error_' + code.toLowerCase())];
            d.win.EasyDragApp.dispose('a1008');
            // The start page's flow list (easydrag-home.js errorCard); New and Import stay usable.
            const h = sandbox(req => {
                if (req.url === '/api/desktop/flows') throw apiError(code);
                if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
                return undefined;
            });
            const home = h.ED.home.create({ ctx: h.ctx, t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow: () => {} });
            h.body.appendChild(home.el);
            await settle();
            const grid = home.el.querySelector('.ed-flow-grid').html;
            const card = [grid.includes('disabled_title'), grid.includes('data-ed-home-settings'), grid.includes('data-ed-home-retry'),
                grid.includes('error_' + code.toLowerCase()), home.el.querySelector('[data-ed-import]').disabled];
            home.dispose();
            rows.push([code, shell, card, d.logged.concat(h.logged)]);
        }
        eq('a1008 1.2 Mission Control, runner, vault and cancel errors are errors with Try again; FLOWS_DISABLED keeps the lock card',
            rows, [...A1008_CODES.map(code => [code, [false, false, true, true], [false, false, true, true, false], []]),
                ['FLOWS_DISABLED', [true, true, true, false], [true, true, true, false, true], []]]);
    });

    await guardAsync('a1008 1.2 the new codes read as their own sentence in every locale', async () => {
        const en = readLang(apps, 'easydrag', 'en');
        const realT = key => en[key] || key;
        const core = sandbox(() => undefined).ED.core;
        eq('a1008 1.2 en: errorText names the cause',
            A1008_CODES.map(code => core.errorText(realT, apiError(code))), [
                'Mission Control is not available right now.',
                'EasyDrag is shutting down or restarting. Try again in a moment.',
                'The Vault is not available, so flow secrets cannot be used.',
                'The request was cancelled.'
            ]);
        const missing = [];
        for (const locale of LOCALES) {
            const words = readLang(apps, 'easydrag', locale);
            for (const code of A1008_CODES) {
                const text = words['easydrag.ui.error_' + code.toLowerCase()];
                if (!text || !text.trim() || text === words['easydrag.ui.error_flows_disabled']) missing.push(locale + ':' + code);
            }
        }
        eq('a1008 1.2 every locale has the four sentences, none of them the switched-off one', missing, []);
    });
}
