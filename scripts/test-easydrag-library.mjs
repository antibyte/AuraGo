import { webcrypto } from 'node:crypto';

export async function run({ sandbox, eq, guardAsync, settle }) {
    const flows = [
        { id: 'f1', name: 'Alpha', published: true, enabled: true, triggers: [] },
        { id: 'f2', name: 'Beta', published: true, enabled: false, triggers: [], last_run: { status: 'error' } },
        { id: 'f3', name: 'Gamma', published: false, enabled: false, triggers: [] }
    ];
    const KEY = 'aurago.easydrag.library';
    function harness() {
        const h = sandbox(req => req.url === '/api/desktop/flows' ? { flows } : req.url.startsWith('/api/desktop/flows/templates') ? { templates: [] } : undefined);
        h.win.crypto = webcrypto;
        h.home = () => {
            const home = h.ED.home.create({ ctx: h.ctx, t: h.ctx.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: true, openFlow: id => h.opened.push(id) });
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
}
