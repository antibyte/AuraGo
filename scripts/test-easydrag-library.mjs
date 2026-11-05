import { webcrypto } from 'node:crypto';

export async function run({ sandbox, eq, guardAsync, settle }) {
    const flows = [
        { id: 'f1', name: 'Alpha', published: true, enabled: true, triggers: [] },
        { id: 'f2', name: 'Beta', published: true, enabled: false, triggers: [], last_run: { status: 'error' } },
        { id: 'f3', name: 'Gamma', published: false, enabled: false, triggers: [] }
    ];
    const KEY = 'aurago.easydrag.library';
    function harness(input = flows) {
        const rows = input.map(flow => Object.assign({}, flow));
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows') return { flows: rows };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            const match = /^\/api\/desktop\/flows\/([^/]+)\/enabled$/.exec(req.url);
            if (!match) return undefined;
            const flow = rows.find(row => row.id === match[1]);
            if (!flow) throw Object.assign(new Error('missing'), { status: 404, body: { code: 'FLOW_NOT_FOUND' } });
            if (req.method === 'GET') return { mission_state: flow.enabled ? 'enabled' : 'disabled' };
            if (req.method === 'POST') { flow.enabled = !!req.body.enabled; return { enabled: flow.enabled }; }
            return undefined;
        });
        h.rows = rows;
        h.win.crypto = webcrypto;
        h.home = (readonly = true) => {
            const home = h.ED.home.create({ ctx: h.ctx, t: h.ctx.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly, openFlow: id => h.opened.push(id) });
            h.body.appendChild(home.el); return home;
        };
        return h;
    }
    const ids = home => home.el.querySelectorAll('[data-ed-flow]').map(card => card.dataset.edFlow);

    await guardAsync('library preferences preserve flow identity and readonly guards', async () => {
        const h = harness(); let home = h.home(); await settle();
        home.el.querySelector('[data-ed-library-view="list"]').fire('click');
        home.el.querySelector('[data-ed-favorite="f2"]').fire('click');
        home.el.querySelector('[data-ed-library-favorites]').fire('click');
        home.el.querySelector('[data-ed-filter="errors"]').fire('click');
        const sort = home.el.querySelector('[data-ed-home-sort]'); sort.value = 'name'; sort.fire('change');
        const search = home.el.querySelector('[data-ed-home-search]'); search.value = 'Beta'; search.fire('input');
        h.ctx.promptDialog = async () => 'Needs attention';
        home.el.querySelector('[data-ed-library-save]').fire('click'); await settle();
        const prefs = JSON.parse(h.store.get(KEY)), preset = prefs.filters[0];
        eq('library compact list, favorites and named filters save only explicit view choices', [home.el.querySelector('.ed-flow-grid').classList.contains('is-list'), ids(home), prefs, home.createOff(), h.requests.every(req => req.method === 'GET')], [true, ['f2'], { view: 'list', favorites: ['f2'], filters: [{ id: preset.id, name: 'Needs attention', query: 'Beta', filter: 'errors', sort: 'name', favorites: true }] }, true, true]);
        home.dispose(); home = h.home(); await settle();
        const picker = home.el.querySelector('[data-ed-library-filter]');
        const input = home.el.querySelector('[data-ed-home-search]'); input.value = 'unfinished'; input.fire('input');
        picker.value = preset.id; picker.fire('change'); h.runTimers(120);
        eq('library saved filters reapply exact local choices and cancel pending searches', [ids(home), picker.value, input.value, home.el.querySelector('[data-ed-home-sort]').value, home.el.querySelector('[data-ed-library-favorites]').getAttribute('aria-pressed')], [['f2'], preset.id, 'Beta', 'name', 'true']);
        home.el.querySelector('[data-ed-favorite="f2"]').fire('click');
        eq('library removing the last filtered favorite keeps an explicit recovery control', [ids(home), !!home.el.querySelector('[data-ed-home-clear-filter]')], [[], true]);
        home.el.querySelector('[data-ed-home-clear-filter]').fire('click');
        const clear = home.el.querySelector('[data-ed-home-clear-search]');
        if (clear) clear.fire('click');
        input.value = ''; input.fire('input'); h.runTimers(120);
        picker.value = preset.id; picker.fire('change');
        home.el.querySelector('[data-ed-library-delete]').fire('click');
        eq('library deleting a preset never deletes or changes a flow', [JSON.parse(h.store.get(KEY)).filters, h.requests.every(req => req.method === 'GET'), h.opened, h.logged], [[], true, [], []]);
        home.dispose();
    });

    await guardAsync('library local preferences merge current storage and exclude payloads', async () => {
        const h = harness(), home = h.home(); await settle();
        h.store.set(KEY, JSON.stringify({ view: 'cards', favorites: ['f3'], payload: { secret: 'never copy this' }, filters: [{ id: 'filter_other', name: 'From another window', filter: 'drafts', sort: 'updated', query: '', favorites: false, samples: { name: 'private' } }] }));
        home.el.querySelector('[data-ed-favorite="f1"]').fire('click');
        const prefs = JSON.parse(h.store.get(KEY));
        eq('library favorites preserve another windows saved filters while discarding unknown payload fields', [prefs.favorites, prefs.filters[0].name, Object.keys(prefs).sort(), Object.keys(prefs.filters[0]).sort(), h.requests.every(req => req.method === 'GET')], [['f3', 'f1'], 'From another window', ['favorites', 'filters', 'view'], ['favorites', 'filter', 'id', 'name', 'query', 'sort'], true]);
        home.dispose();
    });

    await guardAsync('library blocked preference storage remains usable and reports the limitation once', async () => {
        const h = harness(), home = h.home(); await settle();
        h.ED.core.storage.set = () => false;
        home.el.querySelector('[data-ed-library-view="list"]').fire('click');
        home.el.querySelector('[data-ed-favorite="f1"]').fire('click');
        eq('library unavailable storage keeps transient choices without pretending they persisted', [home.el.querySelector('.ed-flow-grid').classList.contains('is-list'), home.el.querySelector('[data-ed-favorite="f1"]').getAttribute('aria-pressed'), h.notes.map(note => note.message), h.store.has(KEY)], [true, 'true', ['library_preferences_unavailable'], false]);
        home.dispose();
    });

    await guardAsync('library list failures never write preferences or expose local controls', async () => {
        const h = sandbox(req => { if (req.url === '/api/desktop/flows') throw { status: 401, body: { code: 'UNAUTHORIZED' } }; if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] }; });
        h.store.set(KEY, JSON.stringify({ view: 'list', favorites: ['f1'], filters: [] }));
        const before = h.store.get(KEY);
        const home = h.ED.home.create({ ctx: h.ctx, t: h.ctx.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow() {} });
        h.body.appendChild(home.el); await settle();
        home.el.querySelector('[data-ed-library-view="cards"]').fire('click');
        eq('library access loss hides controls, preserves stored preferences and keeps mutations blocked', [home.el.querySelector('[data-ed-library-tools]').hidden, h.store.get(KEY), home.createOff(), ids(home)], [true, before, true, []]);
        home.dispose();
    });

    await guardAsync('library preferences refresh sibling desktop windows in the same document', async () => {
        const h = harness(), first = h.home(), second = h.home(); await settle();
        first.el.querySelector('[data-ed-favorite="f2"]').fire('click');
        second.el.querySelector('[data-ed-library-favorites]').fire('click');
        const favorite = second.el.querySelector('[data-ed-favorite="f2"]').getAttribute('aria-pressed');
        first.el.querySelector('[data-ed-library-view="list"]').fire('click');
        const view = second.el.querySelector('.ed-flow-grid').classList.contains('is-list');
        h.ctx.promptDialog = async () => 'Shared view';
        first.el.querySelector('[data-ed-library-save]').fire('click'); await settle();
        const presetCount = second.el.querySelector('[data-ed-library-filter]').querySelectorAll('option').length;
        first.el.querySelector('[data-ed-favorite="f2"]').fire('click');
        eq('library same-document favorite, view and preset changes update every live home', [favorite, view, presetCount, ids(second), h.requests.every(req => req.method === 'GET')], ['true', true, 2, [], true]);
        first.dispose();
        second.el.querySelector('[data-ed-library-view="cards"]').fire('click');
        eq('library disposed homes release local subscriptions', [first.el.parentNode, second.el.querySelector('.ed-flow-grid').classList.contains('is-list'), h.logged], [null, false, []]);
        second.dispose();
    });

    await guardAsync('library pause selection stays explicit, published-only, capped and clears on query changes', async () => {
        const many = Array.from({ length: 51 }, (_, index) => ({ id: 'f' + String(index + 1), name: 'Flow ' + (index + 1), published: true, enabled: true, triggers: [] }));
        const h = harness(many), home = h.home(false); await settle();
        const boxes = home.el.querySelectorAll('[data-ed-select-flow]');
        boxes.slice(0, 50).forEach(box => { box.checked = true; box.fire('change', { target: box }); });
        boxes[50].checked = true; boxes[50].fire('change', { target: boxes[50] });
        const selectedAtLimit = [home.el.querySelector('[data-ed-bulk-count]').textContent, boxes.filter(box => box.checked).length, boxes[50].checked, h.requests.filter(req => req.method === 'POST').length];
        const search = home.el.querySelector('[data-ed-home-search]'); search.value = 'Flow 1'; search.fire('input', { target: search });
        eq('library pause selects at most 50 published flows and query input clears the temporary selection without writes', [selectedAtLimit, home.el.querySelector('[data-ed-bulk-selection]').hidden, h.requests.filter(req => req.method === 'POST').length], [['home_bulk_selected:50', 50, false, 0], true, 0]);
        home.dispose();
    });

    await guardAsync('library pause drops a selected flow when it disappears from a live favorites filter', async () => {
        const h = harness(), home = h.home(false); await settle();
        home.el.querySelector('[data-ed-favorite="f1"]').fire('click');
        home.el.querySelector('[data-ed-library-favorites]').fire('click');
        const checkbox = home.el.querySelector('[data-ed-select-flow="f1"]');
        checkbox.checked = true; checkbox.fire('change', { target: checkbox });
        const before = [home.el.querySelector('[data-ed-bulk-count]').textContent, ids(home)];
        home.el.querySelector('[data-ed-favorite="f1"]').fire('click');
        eq('library pause never keeps a hidden flow selected after a favorite changes in the current filter', [before, home.el.querySelector('[data-ed-bulk-selection]').hidden, home.el.querySelector('[data-ed-bulk-count]').textContent, ids(home)], [['home_bulk_selected:1', ['f1']], true, 'home_bulk_selected:0', []]);
        home.dispose();
    });

    await guardAsync('library pause prunes a selected flow after an external reload changes filter membership', async () => {
        const h = harness(), home = h.home(false); await settle();
        home.el.querySelector('[data-ed-filter="active"]').fire('click');
        const checkbox = home.el.querySelector('[data-ed-select-flow="f1"]');
        checkbox.checked = true; checkbox.fire('change', { target: checkbox });
        h.rows.find(row => row.id === 'f1').enabled = false;
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'enabled' } });
        await settle();
        eq('library pause never reviews a flow hidden by a newer list response', [home.el.querySelector('[data-ed-bulk-selection]').hidden, home.el.querySelector('[data-ed-bulk-count]').textContent, ids(home), h.requests.filter(req => req.method === 'POST').length], [true, 'home_bulk_selected:0', [], 0]);
        home.dispose();
    });

    await guardAsync('library deletion clears per-flow browser data while a pause write is in flight', async () => {
        const h = harness(), home = h.home(false);
        let releasePost, postStarted = false, reads = 0;
        const heldPost = new Promise(resolve => { releasePost = resolve; });
        h.ED.core.FLOW_KEYS.forEach(prefix => h.store.set(prefix + 'f1', 'sensitive fixture data'));
        h.api.enabledStatus = async () => ({ mission_state: ++reads >= 3 ? 'disabled' : 'enabled' });
        h.api.setEnabled = async () => { postStarted = true; await heldPost; return { enabled: false }; };
        try {
            await settle();
            const checkbox = home.el.querySelector('[data-ed-select-flow="f1"]');
            checkbox.checked = true; checkbox.fire('change', { target: checkbox });
            home.el.querySelector('[data-ed-bulk-review]').fire('click');
            await settle();
            home.el.querySelector('.ed-modal--bulk-pause [data-ed-action="confirm"]').fire('click');
            await settle();
            if (!postStarted) throw new Error('pause POST did not enter its held in-flight state');
            h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'deleted' } });
            const retained = h.ED.core.FLOW_KEYS.filter(prefix => h.store.has(prefix + 'f1'));
            eq('library deletion forgets private per-flow browser data without starting a concurrent list reload', [retained, h.requests.filter(req => req.url === '/api/desktop/flows').length], [[], 1]);
        } finally {
            releasePost();
            await settle();
            home.dispose();
        }
    });

    await guardAsync('library pause review and execution use exact statuses and one serial pause with readback', async () => {
        const states = new Map([['a', 'enabled'], ['b', 'disabled'], ['c', 'missing'], ['d', 'mismatch'], ['e', 'unavailable']]);
        const calls = [];
        const api = {
            enabledStatus: async id => { calls.push('GET ' + id); return { mission_state: states.get(id) || 'unavailable' }; },
            setEnabled: async (id, enabled) => { calls.push('POST ' + id + ' ' + enabled); states.set(id, 'disabled'); return { enabled }; }
        };
        const targets = Array.from(states.keys(), id => ({ id, name: 'Flow ' + id }));
        const h = harness(), review = await h.ED.home.pauseOps.review(api, targets);
        const previewCalls = calls.slice();
        const result = await h.ED.home.pauseOps.execute(api, review.rows);
        eq('library pause review is read-only and execution rechecks, writes once, then reads back', [review.rows.map(row => row.state), previewCalls.every(call => call.startsWith('GET ')), calls.slice(5), result.rows.map(row => row.outcome)], [['enabled', 'disabled', 'missing', 'mismatch', 'unavailable'], true, ['GET a', 'POST a false', 'GET a'], ['confirmed', 'disabled', 'missing', 'mismatch', 'unavailable']]);
    });

    await guardAsync('library pause lost POST reply reconciles once and stops unsent work', async () => {
        const calls = [], count = new Map();
        const api = {
            enabledStatus: async id => { calls.push('GET ' + id); const n = (count.get(id) || 0) + 1; count.set(id, n); return { mission_state: id === 'a' && n > 1 ? 'disabled' : 'enabled' }; },
            setEnabled: async id => { calls.push('POST ' + id); throw Object.assign(new Error('lost'), { status: 500, body: { code: 'FLOW_INTERNAL' } }); }
        };
        const reviewed = [{ id: 'a', name: 'Alpha', state: 'enabled' }, { id: 'b', name: 'Beta', state: 'enabled' }];
        const result = await harness().ED.home.pauseOps.execute(api, reviewed);
        eq('library pause never retries an uncertain POST and marks later targets not attempted', [calls, result.rows.map(row => row.outcome), result.stopped], [['GET a', 'POST a', 'GET a'], ['save_uncertain', 'not_attempted'], true]);
    });

    await guardAsync('library pause stops later writes when readback no longer proves the target identity', async () => {
        const results = [];
        for (const state of ['missing', 'mismatch']) {
            const calls = [], api = {
                enabledStatus: async id => { calls.push('GET ' + id); return { mission_state: calls.filter(call => call === 'GET a').length === 1 ? 'enabled' : state }; },
                setEnabled: async (id, enabled) => { calls.push('POST ' + id + ' ' + enabled); return { enabled }; }
            };
            const result = await harness().ED.home.pauseOps.execute(api, [{ id: 'a', name: 'Alpha', state: 'enabled' }, { id: 'b', name: 'Beta', state: 'enabled' }]);
            results.push([calls, result.rows.map(row => row.outcome), result.stopped]);
        }
        eq('library pause keeps missing or mismatched post-write identity unresolved and sends no later POST', results, [
            [['GET a', 'POST a false', 'GET a'], ['unresolved', 'not_attempted'], true],
            [['GET a', 'POST a false', 'GET a'], ['unresolved', 'not_attempted'], true]
        ]);
    });

    await guardAsync('library pause read-only loss stops the queue and keeps every selected result', async () => {
        const calls = [], counts = new Map();
        const api = {
            enabledStatus: async id => { calls.push('GET ' + id); const n = (counts.get(id) || 0) + 1; counts.set(id, n); return { mission_state: n > 1 ? 'disabled' : 'enabled' }; },
            setEnabled: async id => { calls.push('POST ' + id); throw Object.assign(new Error('readonly'), { status: 403, body: { code: 'desktop_readonly' } }); }
        };
        const reviewed = [{ id: 'a', name: 'Alpha', state: 'enabled' }, { id: 'b', name: 'Beta', state: 'enabled' }];
        const result = await harness().ED.home.pauseOps.execute(api, reviewed);
        eq('library pause stops unsent requests after a read-only denial while preserving uncertain readback', [calls, result.permissionLost, result.rows.map(row => row.outcome)], [['GET a', 'POST a', 'GET a'], true, ['save_uncertain', 'not_attempted']]);
    });

    await guardAsync('library pause close cancels an unsent candidate after its precheck', async () => {
        const calls = []; let cancelled = false;
        const api = {
            enabledStatus: async id => { calls.push('GET ' + id); return { mission_state: 'enabled' }; },
            setEnabled: async id => { calls.push('POST ' + id); return {}; }
        };
        const reviewed = [{ id: 'a', name: 'Alpha', state: 'enabled' }, { id: 'b', name: 'Beta', state: 'enabled' }];
        const result = await harness().ED.home.pauseOps.execute(api, reviewed, { cancelled: () => cancelled, onProgress() { cancelled = true; } });
        eq('library pause does not post after cancellation while candidates are being checked', [calls, result.rows.map(row => row.outcome), result.stopped], [['GET a'], ['not_attempted', 'not_attempted'], true]);
    });

    await guardAsync('library pause refuses a new POST when the operation deadline expires after precheck', async () => {
        const started = Date.now();
        const calls = [], api = {
            enabledStatus: async id => { calls.push('GET ' + id); return { mission_state: 'enabled' }; },
            setEnabled: async id => { calls.push('POST ' + id); return {}; }
        };
        const result = await harness().ED.home.pauseOps.execute(api, [{ id: 'a', name: 'Alpha', state: 'enabled' }], {
            totalMs: 50, requestMs: 5000,
            onProgress(_current, _total, _target, stage) { if (stage === 'pause') while (Date.now() < started + 100) {} }
        });
        eq('library pause deadline check immediately before the mutation leaves it unsent', [calls, result.rows[0].outcome, result.stopped], [['GET a'], 'not_attempted', true]);
    });

    await guardAsync('library pause preserves permission loss when its one mutation readback also fails', async () => {
        const calls = [], api = {
            enabledStatus: async id => { calls.push('GET ' + id); if (calls.length === 1) return { mission_state: 'enabled' }; throw Object.assign(new Error('readback failed'), { status: 500, body: { code: 'FLOW_INTERNAL' } }); },
            setEnabled: async id => { calls.push('POST ' + id); throw Object.assign(new Error('read only'), { status: 403, body: { code: 'desktop_readonly' } }); }
        };
        const reviewed = [{ id: 'a', name: 'Alpha', state: 'enabled' }, { id: 'b', name: 'Beta', state: 'enabled' }];
        const result = await harness().ED.home.pauseOps.execute(api, reviewed);
        eq('library pause keeps the write-side permission loss across a failed readback and sends no later mutation', [calls, result.permissionLost, result.rows.map(row => row.outcome)], [['GET a', 'POST a', 'GET a'], true, ['unresolved', 'not_attempted']]);
    });
}
