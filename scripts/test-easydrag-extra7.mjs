// a1023 checks (audit 2026-10-23, the editor findings): clearing a step setting takes it back to
// the default (F2); a version is not restored over a draft that cannot be saved (F1); a step
// without outputs does not cut a wire when it is dropped on it (F3); an emergency copy of an older
// server revision is offered, not lost (F4); the editor does not stay "running" when the stored
// result of a finished run cannot be fetched once (F5). F9 (the Uncertain port word) is checked in
// ui/desktop_easydrag_test.go. They run on the c1d07 sandbox (test-easydrag-extra3.mjs returns
// sandbox and openEditor, whose EventSource stub lists its streams in h.sources); test-easydrag.mjs
// calls run(env) with its helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';
const B = 'n_bbbbbbbb';
const STOP = 'n_ssssssss';
const DRAFT_KEY = 'aurago.easydrag.draft.f1';

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

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

// missingKeys lists locale:key for each key that a locale lacks or leaves empty.
function missingKeys(apps, keys) {
    return LOCALES.flatMap(locale => {
        const words = JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));
        return keys.filter(key => typeof words[key] !== 'string' || !words[key].trim()).map(key => locale + ':' + key);
    });
}

export async function run(env) {
    const { apps, types, eq, guardAsync, settle, sandbox, openEditor } = env;

    // ── F2: a cleared setting is removed, not kept ──

    await guardAsync('a1023 F2 clearing a step setting removes it, and one undo brings it back', async () => {
        const model = sandbox(() => undefined).ED.model.create(flowDoc(), { types });
        const stored = () => model.toJSON().nodes.find(n => n.id === A).settings;
        const settings = [['on_error', 'continue'], ['retry', { count: 2, delay_seconds: 10 }], ['timeout_seconds', 30], ['notes', 'Call Ada first']];
        const clears = [['undefined', undefined], ['null', null], ['empty text', '']];
        const rows = [];
        for (const [key, value] of settings) {
            for (const [name, cleared] of clears) {
                model.setSettings(A, { [key]: value });
                const set = model.node(A).settings[key];
                // A change in between keeps the clear from merging into the set (one coalesce key within 1.2 s).
                model.setLabel(A, 'Alpha ' + rows.length);
                model.setSettings(A, { [key]: cleared });
                const gone = [key in model.node(A).settings, key in stored()];
                model.undo();
                rows.push([key, name, set, gone, model.node(A).settings[key]]);
                model.undo();
                model.undo();
            }
        }
        eq('a1023 F2 on_error, retry, timeout and note: set, cleared with undefined, null or empty text, then one undo',
            rows, settings.flatMap(([key, value]) => clears.map(([name]) => [key, name, value, [false, false], value])));
        // Back to the default error handling drops the wire of the error port with it.
        model.setSettings(A, { on_error: 'error_port' });
        model.connect(A, 'error', B, 'in');
        const wired = model.outgoing(A).map(e => e.source.port);
        model.setSettings(A, { on_error: undefined });
        eq('a1023 F2 clearing the error handling drops the wire of the error port and leaves the other settings',
            [wired, model.outgoing(A).map(e => e.source.port), stored()], [['out', 'error'], ['out'], {}]);
    });

    // ── F1: no version is restored over a draft that cannot be saved ──

    await guardAsync('a1023 F1 a version is not restored over a draft that cannot be saved', async () => {
        // restoreOver opens f1 with an unsaved edit, whose save fails (putFails) or works, and
        // restores version 2 from the history dialog.
        async function restoreOver(putFails) {
            const h = sandbox(req => {
                if (req.method === 'GET' && req.url === '/api/desktop/flows/f1/versions') {
                    return { live_revision: 2, versions: [{ revision: 2, published_at: '2026-10-06T10:00:00Z', node_count: 3, edge_count: 2 }] };
                }
                if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/versions') return { flow: { draft: flowDoc('Restored'), draft_revision: 5 }, issues: [] };
                if (req.method === 'PUT') {
                    if (putFails) throw new TypeError('Failed to fetch');
                    return { draft_revision: 4, issues: [] };
                }
                return undefined;
            });
            h.confirmAnswer = true;
            const editor = openEditor(h);
            await settle();
            editor.ed.model.setFlow({ description: 'unsaved edit' });
            h.menus.flatMap(m => m.items).find(i => i.id === 'versions').action();
            await settle();
            const button = h.dialogs[h.dialogs.length - 1].el.querySelector('[data-ed-restore-rev="2"]');
            button.fire('click');
            await settle();
            const posts = h.requests.filter(r => r.method === 'POST' && r.url === '/api/desktop/flows/f1/versions').length;
            const row = [posts, button.disabled, h.notes.map(n => [n.title, n.message, n.type]), h.store.has(DRAFT_KEY), editor.ed.model.doc.description, h.logged];
            editor.dispose();
            return row;
        }
        eq('a1023 F1 an unsaved draft: when its save fails nothing is restored, the button works again, the notice says why and the edit and its copy stay; when it saves, the restore goes on',
            [await restoreOver(true), await restoreOver(false)], [
                [0, false, [['versions_title', 'versions_unsaved', 'error']], true, 'unsaved edit', []],
                [1, true, [['versions_title', 'versions_restored:2', undefined]], false, undefined, []]
            ]);
        eq('a1023 F1 the notice exists in all 16 locales', missingKeys(apps, ['easydrag.ui.versions_unsaved']), []);
    });

    // ── F3: a step without outputs does not cut a wire ──

    await guardAsync('a1023 F3 a step without outputs is not dropped onto a wire', async () => {
        // logic.stop has no output port (catalog outputs: []).
        const stopTypes = new Map(types);
        stopTypes.set('logic.stop', { type: 'logic.stop', label: 'Stop', inputs: ['in'], outputs: [], params: [] });
        const h = sandbox(() => undefined);
        h.catalog = { types: stopTypes, list: Array.from(stopTypes.values()), categories: [] };
        const doc = flowDoc();
        doc.nodes.push({ id: STOP, key: 'stop', type: 'logic.stop', label: 'Stop', position: { x: 450, y: 200 }, params: {}, settings: {} });
        const editor = openEditor(h, { flow: { draft: doc } });
        await settle();
        const ed = editor.ed;
        const wires = () => ed.model.doc.edges.map(e => e.id + ':' + e.source.node + '>' + e.target.node).join(' ');
        const before = wires();
        // Dragging the unconnected Stop card onto the wire from alpha to beta: it neither marks the
        // wire nor is inserted into it.
        const canvasEl = editor.el.querySelector('.ed-canvas');
        const zoom = ed.view.zoom;
        editor.el.querySelector('[data-node-id="' + STOP + '"]').fire('pointerdown', h.pe(1, 10, 300));
        canvasEl.fire('pointermove', h.pe(1, 10, 300 - 200 * zoom));
        const marked = editor.el.querySelectorAll('.ed-edge').filter(g => g.classList.contains('is-drop-target')).length;
        canvasEl.fire('pointerup', h.pe(1, 10, 300 - 200 * zoom));
        const dropped = [marked, wires() === before, ed.model.incoming(B).map(e => e.source.node)];
        // The model refuses it too, for a new type and for the card.
        const nodes = ed.model.doc.nodes.length;
        const refused = [ed.model.insertOnEdge('e2', 'logic.stop', { x: 450, y: 0 }), ed.model.insertOnEdge('e2', STOP, null), wires() === before, ed.model.doc.nodes.length === nodes];
        // A palette drop or quick-add on the wire adds the step beside it, unconnected.
        const id = h.ED.palette.place(ed, 'logic.stop', { x: 450, y: 0 }, { edge: 'e2' });
        const added = [typeof id, id ? ed.model.incoming(id).length + ed.model.outgoing(id).length : -1, wires() === before, ed.model.doc.nodes.length - nodes];
        eq('a1023 F3 the wire from alpha to beta stays whole: no drop mark, no insert by drag or model, the palette adds the step unconnected',
            [dropped, refused, added, h.logged],
            [[0, true, [A]], [null, null, true, true], ['string', 0, true, 1], []]);
        // A step with outputs still goes in (test-easydrag.mjs: 'insert on edge').
        const search = ed.model.insertOnEdge('e2', 'web.search', { x: 450, y: 0 });
        eq('a1023 F3 a step with an output still goes onto the wire',
            [ed.model.incoming(B).map(e => e.source.node)[0] === search, ed.model.incoming(search).map(e => e.source.node)], [true, [A]]);
        editor.dispose();
    });

    // ── F4: an emergency copy of an older revision is offered ──

    await guardAsync('a1023 F4 an emergency copy made on an older server revision is offered and restored', async () => {
        // offer opens f1 at serverRevision with the stored copy {revision, name}, answers the
        // restore dialog (if any) with action and lets the saver send what a restore changed.
        async function offer(revision, name, serverRevision, action) {
            const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: serverRevision + 1, issues: [] } : undefined));
            // The saver stores model.toJSON(), the document as the model keeps it.
            const doc = h.ED.model.create(flowDoc(name), { types }).toJSON();
            h.store.set(DRAFT_KEY, JSON.stringify({ revision, at: Date.now(), doc }));
            const editor = openEditor(h, { flow: { draft_revision: serverRevision } });
            await settle();
            const dialog = h.dialogs[0] || null;
            // shown: a dialog is offered, with the line that restoring replaces a newer draft.
            const shown = [!!dialog, !!(dialog && dialog.body.querySelector('[data-ed-restore-stale]'))];
            if (dialog && action) dialog.el.querySelector('[data-ed-action="' + action + '"]').fire('click');
            else if (dialog) dialog.close(null);
            await settle();
            h.runTimers(1000);
            await settle();
            const row = [action, shown, h.store.has(DRAFT_KEY), editor.ed.model.doc.name, h.puts().map(p => p.body.base_revision), h.dialogs.length, h.logged];
            editor.dispose();
            return row;
        }
        eq('a1023 F4 copy of revision 3, server draft at 4: restore, discard and no answer; a copy of the current revision says nothing about a newer draft; an unchanged one is dropped',
            [await offer(3, 'Local', 4, 'restore'), await offer(3, 'Local', 4, 'discard'), await offer(3, 'Local', 4, null),
                await offer(4, 'Local', 4, 'discard'), await offer(3, 'Flow', 4, null)], [
                ['restore', [true, true], false, 'Local', [4], 1, []],
                ['discard', [true, true], false, 'Flow', [], 1, []],
                [null, [true, true], true, 'Flow', [], 1, []],
                ['discard', [true, false], false, 'Flow', [], 1, []],
                [null, [false, false], false, 'Flow', [], 0, []]
            ]);
        eq('a1023 F4 the stale notice exists in all 16 locales', missingKeys(apps, ['easydrag.ui.restore_stale']), []);
    });

    // ── F5: a finished run whose result cannot be fetched once does not stay running ──

    await guardAsync('a1023 F5 the editor leaves "running" when the stored result is fetched on a retry', async () => {
        // follow starts a live run r1, ends its stream while the streamed state is still
        // "running", and plays the retries (timers, then a new stream's "end") for up to rounds
        // rounds. failures: how often the fetch of the stored run fails before it answers (Infinity: always).
        async function follow(failures, rounds) {
            let failed = 0;
            const h = sandbox(req => {
                if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run') return { run_id: 'r1' };
                if (req.method === 'GET' && req.url === '/api/desktop/flows/runs/r1') {
                    if (failed++ < failures) throw Object.assign(new Error('HTTP 500'), { status: 500, body: {} });
                    return { run: { id: 'r1', status: 'success', mode: 'live', started_at: '2026-10-23T10:00:00Z' }, steps: [] };
                }
                return undefined;
            });
            const editor = openEditor(h, { enabled: true, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });
            await settle();
            editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click');
            await settle();
            h.sources[0].emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] });
            const testBtn = editor.el.querySelector('[data-ed-cmd="test"]');
            const running = testBtn.disabled;
            h.sources[0].emit('end', {});
            await settle();
            let used = 0;
            for (; used < rounds && testBtn.disabled; used++) {
                h.runTimers();
                await settle();
                const last = h.sources[h.sources.length - 1];
                if (!last.closed) last.emit('end', {});
                await settle();
            }
            const fetches = h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/runs/r1').length;
            const row = [running, testBtn.disabled, editor.ed.run.status, !!editor.ed.run.stale, fetches, h.notes.map(n => n.title), h.logged];
            editor.dispose();
            return { row, used };
        }
        const once = await follow(1, 3);
        eq('a1023 F5 stream end, fetch fails once, then answers: the run ends as success and Test works again',
            once.row, [true, false, 'success', false, 2, [], []]);
        // A fetch that never works is given up on, as the stream probe does: not a loop without end.
        const never = await follow(Infinity, 12);
        eq('a1023 F5 a fetch that always fails gives the run up after a few tries: stale, one notice, Test works again',
            [never.row[1], never.row[3], never.row[5], never.row[4] <= 6, never.used <= 6, never.row[6]], [false, true, ['run_stream_lost'], true, true, []]);
    });
}
