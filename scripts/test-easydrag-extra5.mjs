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
}
