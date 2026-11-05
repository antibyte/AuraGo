// Persistence regressions use the shared editor/saver harness and an omission-preserving CAS fixture.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const A = 'n_aaaaaaaa', B = 'n_bbbbbbbb', KEY = 'aurago.easydrag.draft.f1';
    const copy = value => JSON.parse(JSON.stringify(value));
    const group = () => ({ id: 'g_abcdefgh', title: 'Remote group', color: 'blue', members: [A, B] });
    const flow = () => ({ schema: 1, name: 'Flow', nodes: [
        { id: 'n_tttttttt', key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
        { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} },
        { id: B, key: 'beta', type: 'web.search', label: 'Beta', position: { x: 600, y: 0 }, params: { query: '' }, settings: {} }
    ], edges: [] });
    function backend(initial, version = 3) {
        let stored = copy(initial), revision = version;
        const h = sandbox(req => {
            if (req.url !== '/api/desktop/flows/f1') return undefined;
            if (req.method === 'GET') return { flow: { id: 'f1', draft: copy(stored), draft_revision: revision }, issues: [] };
            if (req.method !== 'PUT') return undefined;
            if (req.body.base_revision !== revision) throw Object.assign(new Error('conflict'), { body: { code: 'FLOW_REVISION_CONFLICT' } });
            const next = copy(req.body.doc);
            if (next.groups == null && Object.hasOwn(stored, 'groups')) next.groups = copy(stored.groups);
            stored = next;
            return { draft_revision: ++revision, issues: [] };
        });
        return { h, stored: () => copy(stored), revision: () => revision };
    }
    const puts = h => h.requests.filter(req => req.method === 'PUT' && req.url === '/api/desktop/flows/f1');

    await guardAsync('groups-save acknowledged group undo remains cleared after server reload', async () => {
        const api = backend(flow()), editor = openEditor(api.h), model = editor.ed.model;
        const id = model.createGroup([A, B], 'Created');
        await editor.ed.saver.flush();
        eq('groups-save creation reached the server before undo', [api.revision(), api.stored().groups[0].id], [4, id]);
        model.undo();
        await editor.ed.saver.flush();
        const reloaded = api.h.ED.model.create(api.stored(), api.h.catalog);
        eq('groups-save undo sends an explicit clear at the acknowledged revision', [puts(api.h).at(-1).body.base_revision, puts(api.h).at(-1).body.doc.groups, api.stored().groups, reloaded.doc.groups], [4, [], [], []]);
        editor.dispose();
    });

    await guardAsync('groups-save an older emergency copy explicitly replaces remote groups', async () => {
        const remote = flow(); remote.groups = [group()];
        const legacy = flow(); legacy.name = 'Recovered legacy';
        const api = backend(remote);
        api.h.store.set(KEY, JSON.stringify({ revision: 2, at: Date.now(), doc: legacy }));
        const editor = openEditor(api.h, { flow: { draft: copy(remote), draft_revision: 3 } });
        await settle();
        api.h.dialogs[0].el.querySelector('[data-ed-action="restore"]').fire('click');
        await settle();
        await editor.ed.saver.flush();
        eq('groups-save explicit restore clears remote groups and survives reload', [puts(api.h).at(-1).body.base_revision, puts(api.h).at(-1).body.doc.groups, api.stored().name, api.stored().groups, api.h.ED.model.create(api.stored(), api.h.catalog).doc.groups], [3, [], 'Recovered legacy', [], []]);
        editor.dispose();
    });

    await guardAsync('groups-save conflict keep explicitly replaces newer remote groups', async () => {
        const remote = flow(); remote.groups = [group()];
        const api = backend(remote, 4), editor = openEditor(api.h, { flow: { draft: flow(), draft_revision: 3 } });
        await settle();
        editor.ed.model.setFlow({ description: 'Local replacement' });
        const pending = editor.ed.saver.save();
        await settle();
        api.h.dialogs[0].el.querySelector('[data-ed-action="keep"]').fire('click');
        await pending;
        await settle();
        await editor.ed.saver.flush();
        const saved = puts(api.h).at(-1).body;
        eq('groups-save keep uses the fresh CAS and clears omitted local metadata', [saved.base_revision, saved.doc.groups, api.stored().groups, api.stored().description, editor.ed.model.doc.groups], [4, [], [], 'Local replacement', []]);
        editor.dispose();
    });

    await guardAsync('groups-save ordinary legacy edits retain omission without inventing metadata', async () => {
        const api = backend(flow()), editor = openEditor(api.h);
        editor.ed.model.setFlow({ description: 'Ordinary edit' });
        await editor.ed.saver.flush();
        eq('groups-save unchanged legacy metadata remains absent end to end', [Object.hasOwn(puts(api.h).at(-1).body.doc, 'groups'), Object.hasOwn(api.stored(), 'groups'), Object.hasOwn(editor.ed.model.doc, 'groups')], [false, false, false]);
        editor.dispose();
    });

    await guardAsync('groups-save readonly controller never edits or saves group data', async () => {
        const remote = flow(); remote.groups = [group()];
        const api = backend(remote), editor = openEditor(api.h, { readonly: true, flow: { draft: copy(remote) } });
        await settle();
        const before = editor.ed.model.toJSON();
        const dialogs = api.h.dialogs.length;
        editor.ed.selection.add(A);
        const pending = Promise.resolve(editor.ed.groups.createFromSelection());
        await settle();
        // Close unexpected prompts so an asynchronous regression cannot hang the suite.
        api.h.dialogs.slice(dialogs).forEach(dialog => dialog.close(null));
        let timer;
        try {
            await Promise.race([pending, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('readonly group command timed out')), 1000); })]);
        } finally { clearTimeout(timer); }
        await settle();
        eq('groups-save readonly group command leaves model and server untouched without a dialog', [editor.ed.model.toJSON(), puts(api.h).length, api.stored(), api.h.dialogs.length], [before, 0, remote, dialogs]);
        editor.dispose();
    });
}
