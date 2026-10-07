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
        const started = process.hrtime.bigint();
        card.fire('pointerdown', h.pe(1, 10, 10));
        for (let i = 1; i <= 60; i++) canvasEl.fire('pointermove', h.pe(1, 10 + i * 3, 10 + i));
        canvasEl.fire('pointerup', h.pe(1, 190, 70));
        const ms = Number(process.hrtime.bigint() - started) / 1e6;
        console.log('info ff2 M2 drag of 200 selected steps: ' + (ms / 60).toFixed(2) + ' ms per frame, ' + copies + ' copy writes');
        const moved = editor.ed.model.node(T1).position.x > 0;
        // Generous: a frame of a 200-step drag took 8.7 ms in a browser before FF2; 60 frames
        // under 3 s leave room for a slow, busy machine and still catch an O(n²) regression.
        eq('ff2 M2 60 frames of a 200-step drag take under 3 s; the copy is written at most once per 500 ms and at the end',
            [moved, ms < 3000, copies >= 1 && copies <= 2 + Math.ceil(ms / 500), JSON.parse(h.store.get(DRAFT_KEY)).doc.nodes.find(n => n.id === T1).position, h.logged],
            [true, true, true, editor.ed.model.node(T1).position, []]);
        editor.dispose();
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
}
