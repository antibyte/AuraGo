// a1023 checks (audit 2026-10-23, the editor findings): clearing a step setting takes it back to
// the default (F2); a version is not restored over a draft that cannot be saved (F1); a step
// without outputs does not cut a wire when it is dropped on it (F3); an emergency copy of an older
// server revision is offered, not lost (F4); the editor does not stay "running" when the stored
// result of a finished run cannot be fetched once (F5). F9 (the Uncertain port word) is checked in
// ui/desktop_easydrag_test.go. The a1023b checks are the audit's accessibility findings: Stop and the
// Active switch keep their names when the header hides their text (U1), the inspector lists, removes
// and inserts into a step's connections (U2), and user actions reach the live region (U3). They run
// on the c1d07 sandbox (test-easydrag-extra3.mjs returns
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

    // ── a1023b: audit 2026-10-23, accessibility ──

    const NEW_KEYS = {
        u2: ['connections_title', 'connections_from', 'connections_to', 'connections_none', 'connections_remove', 'connections_insert'].map(k => 'easydrag.ui.' + k),
        u3: ['announce_added_one', 'announce_added_many', 'announce_deleted_one', 'announce_deleted_many', 'announce_disabled_one', 'announce_disabled_many',
            'announce_enabled_one', 'announce_enabled_many', 'announce_moved_one', 'announce_moved_many', 'announce_undo', 'announce_redo',
            'announce_disconnected', 'announce_test_started', 'announce_run_started', 'announce_step_failed'].map(k => 'easydrag.ui.' + k)
    };

    // ── U1: Stop and the Active switch lose their text below 900 px ──

    await guardAsync('a1023b U1 Stop and the Active switch keep an accessible name when the header hides their text', async () => {
        const css = fs.readFileSync(path.join(apps, '..', '..', '..', 'css', 'desktop-app-easydrag.css'), 'utf8');
        // The first rule under max-width 900 px hides the text of every header button except the ones it spares.
        const spared = Array.from(css.slice(css.indexOf('@container (max-width: 900px)')).split('}')[0].matchAll(/:not\(\[data-ed-cmd="([\w-]+)"\]\)/g), m => m[1]).sort();
        const named = b => b.getAttribute('aria-label') || b.getAttribute('title') || '';
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const buttons = editor.el.querySelectorAll('.ed-head-actions button');
        const cmd = b => b.dataset.edCmd;
        eq('a1023b U1 the header hides the text of Stop, the Active switch and More, and spares four buttons',
            [spared, buttons.map(cmd)], [['publish', 'run-now', 'runs', 'test'], ['runs', 'test', 'cancel', 'publish', 'run-now', 'active', 'more']]);
        eq('a1023b U1 every header button that loses its text has a name of its own',
            buttons.filter(b => !spared.includes(cmd(b)) && !named(b)).map(cmd), []);
        eq('a1023b U1 the names say what the text says, so the visible label is part of the name',
            buttons.filter(b => ['cancel', 'active'].includes(cmd(b))).map(b => [cmd(b), b.getAttribute('aria-label'), b.getAttribute('title')]),
            [['cancel', 'run_cancel', 'run_cancel'], ['active', 'active', 'active']]);
        // The header's Stop says "Stopping…" while the request is out; its name follows the text.
        let release;
        const hold = new Promise(resolve => { release = resolve; });
        const running = sandbox(req => {
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test') return { run_id: 'r1' };
            if (req.method === 'POST' && req.url === '/api/desktop/flows/runs/r1/cancel') return hold;
            return undefined;
        });
        const live = openEditor(running);
        await settle();
        live.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        running.dialogs[running.dialogs.length - 1].el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        const stop = live.el.querySelector('[data-ed-cmd="cancel"]');
        const rows = [[stop.hidden, stop.disabled, named(stop)]];
        stop.fire('click');
        await settle();
        rows.push([stop.hidden, stop.disabled, named(stop)]);
        release({});
        await settle();
        rows.push([stop.hidden, stop.disabled, named(stop)]);
        eq('a1023b U1 Stop keeps a name that matches its text while a stop is out, and after it',
            [rows, running.logged], [[[false, false, 'run_cancel'], [false, true, 'run_stopping'], [false, false, 'run_cancel']], []]);
        eq('a1023b U1 the names need no new translation', missingKeys(apps, ['easydrag.ui.run_cancel', 'easydrag.ui.active', 'easydrag.ui.run_stopping']), []);
        live.dispose();
        editor.dispose();
    });

    // ── U2: connections can be listed, removed and inserted into from the inspector ──

    await guardAsync('a1023b U2 the inspector lists a step\'s connections; Remove and Insert work by keyboard', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h);
        await settle();
        const ed = editor.ed;
        const live = editor.el.querySelector('[data-ed-live]');
        const active = () => h.dom.document.activeElement;
        ed.bus.emit('open-detail', { nodeId: A });
        editor.el.querySelector('[data-ed-pane="settings"]').fire('click');
        const section = () => editor.el.querySelector('.ed-detail [data-ed-connections]');
        const buttons = kind => section().querySelectorAll('[data-ed-conn-' + kind + ']');
        const labelOf = (kind, id) => {
            const b = buttons(kind).find(x => x.dataset['edConn' + kind[0].toUpperCase() + kind.slice(1)] === id);
            return b ? b.getAttribute('aria-label') : null;
        };
        // rows: [wire, text, port, the names of its Remove and Insert buttons]
        const rows = () => Array.from((section().querySelector('ul').html || '').matchAll(/data-ed-conn="([^"]*)"[^>]*><span class="ed-conn-text">([^<]*)<\/span><span class="[^"]*">([^<]*)<\/span>/g),
            m => [m[1], m[2], m[3], labelOf('remove', m[1]), labelOf('insert', m[1])]);
        eq('a1023b U2 step Alpha lists its incoming and its outgoing wire with the other step, the port and a name that holds both ends',
            rows(), [
                ['e1', 'connections_from:Start', 'port_out', 'connections_remove:Start,Alpha', 'connections_insert:Start,Alpha'],
                ['e2', 'connections_to:Beta', 'port_out', 'connections_remove:Alpha,Beta', 'connections_insert:Alpha,Beta']
            ]);
        // Remove on the first row: the wire goes, the announcement says which, and the focus moves to the next row's button.
        const first = buttons('remove')[0];
        first.focus();
        live.textContent = '';
        first.fire('click');
        h.runTimers(30);
        eq('a1023b U2 removing a wire deletes it from the model, announces it and keeps the focus on the next row in the inspector',
            [ed.model.doc.edges.map(e => e.id), rows().map(r => r[0]), section().contains(active()), active().dataset.edConnRemove, live.textContent],
            [['e2'], ['e2'], true, 'e2', 'announce_disconnected:Start,Alpha']);
        // An undo (a change that touches no step) brings the row back: edge changes redraw the list.
        ed.model.undo();
        eq('a1023b U2 an edge change that touches no step still redraws the list', rows().map(r => r[0]), ['e1', 'e2']);
        // The last row's removal moves the focus up; the final one leaves the heading.
        buttons('remove')[1].focus();
        buttons('remove')[1].fire('click');
        const oneLeft = [rows().map(r => r[0]), active().dataset.edConnRemove];
        buttons('remove')[0].focus();
        buttons('remove')[0].fire('click');
        const empty = section().querySelector('[data-ed-conns-empty]');
        eq('a1023b U2 focus stays in the section down to the last row and then rests on the heading, with a hint for the empty list',
            [oneLeft, rows().length, active() === section().querySelector('[data-ed-conns-title]'), empty.hidden], [[['e1'], 'e1'], 0, true, false]);
        // Insert opens quick-add bound to the wire; the step chosen there ends up between the two ends.
        ed.model.undo();
        ed.model.undo();
        const insert = buttons('insert')[1];
        insert.focus();
        insert.fire('click');
        const quick = editor.el.querySelector('.ed-quick');
        const opened = [!!quick, !!ed.quickAdd];
        quick.querySelector('[data-ed-index="0"]').fire('pointerdown');
        const added = ed.model.doc.nodes.find(n => ![T1, A, B].includes(n.id));
        const back = active();
        eq('a1023b U2 Insert on the wire to Beta opens quick-add, the chosen step lands between Alpha and Beta and the focus returns to the section',
            [opened, added && added.label, ed.model.incoming(B).map(e => e.source.node === added.id), ed.model.incoming(added.id).map(e => e.source.node === A),
                ed.model.doc.edges.some(e => e.id === 'e2'), section().contains(back) && back.dataset.edConnInsert !== undefined, !!ed.quickAdd],
            [[true, true], 'Websuche', [true], [true], false, true, false]);
        eq('a1023b U2 the section follows the new step too', rows().map(r => r[1]), ['connections_from:Start', 'connections_to:Websuche']);
        // A read-only desktop lists the wires and offers no buttons.
        const ro = sandbox(() => undefined);
        const roEditor = openEditor(ro, { readonly: true });
        await settle();
        roEditor.ed.bus.emit('open-detail', { nodeId: A });
        roEditor.el.querySelector('[data-ed-pane="settings"]').fire('click');
        const roSection = roEditor.el.querySelector('.ed-detail [data-ed-connections]');
        eq('a1023b U2 a read-only desktop lists both wires without Remove or Insert',
            [(roSection.querySelector('ul').html.match(/data-ed-conn="/g) || []).length, roSection.querySelectorAll('[data-ed-conn-remove]').length + roSection.querySelectorAll('[data-ed-conn-insert]').length],
            [2, 0]);
        eq('a1023b U2 the section checks log no errors', [h.logged, ro.logged], [[], []]);
        eq('a1023b U2 the section words exist in all 16 locales', missingKeys(apps, NEW_KEYS.u2.concat('easydrag.ui.announce_disconnected')), []);
        editor.dispose();
        roEditor.dispose();
    });

    // ── U3: user actions reach the live region ──

    // session opens f1 and returns helpers: pick (select steps), key (a key on the canvas), heard (what the
    // live region says after an action, once its announcement is due).
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
            menu: id => h.menus.flatMap(m => m.items).find(i => i.id === id),
            async heard(act) { live.textContent = ''; await act(); await settle(); h.runTimers(30); return live.textContent; }
        };
    }

    await guardAsync('a1023b U3 adding, deleting and turning steps off or on are announced', async () => {
        const s = await session();
        const rows = [];
        const say = async (name, act) => rows.push([name, await s.heard(act)]);
        let made = null;
        await say('place', () => { made = s.h.ED.palette.place(s.ed, 'web.search', { x: 0, y: 320 }); });
        await say('duplicate one', () => { s.pick(A); s.key('d', { ctrlKey: true }); });
        await say('duplicate several', () => { s.pick(A, B); s.key('d', { ctrlKey: true }); });
        const copies = s.ed.model.doc.nodes.filter(n => ![T1, A, B, made].includes(n.id)).map(n => n.id);
        await say('duplicate from the menu', () => { s.pick(B); s.menu('duplicate').action(); });
        await say('copy', () => { s.pick(B); s.key('c', { ctrlKey: true }); });
        await say('paste', () => s.key('v', { ctrlKey: true }));
        await say('delete with the key', () => { s.pick(made); s.key('Delete'); });
        await say('delete several', () => { s.pick(...copies); s.key('Delete'); });
        await say('delete from the menu', () => { s.pick(B); s.menu('delete-sel').action(); });
        await say('delete with the node tool', () => { s.pick(A); s.card(A).querySelector('[data-ed-node-tool="delete"]').fire('click'); });
        const left = s.ed.model.doc.nodes.length;
        await say('cut says what it did to the clipboard, nothing more', () => { s.pick(T1); s.menu('cut').action(); });
        s.ed.model.undo();
        s.ed.model.undo();
        await say('turn off', () => { s.pick(A); s.key('d'); });
        await say('turn on', () => s.key('d'));
        await say('turn several off', () => { s.pick(A, T1); s.key('d'); });
        await say('turn several on', () => s.key('d'));
        await say('turn off with the node tool', () => { s.pick(A); s.card(A).querySelector('[data-ed-node-tool="disable"]').fire('click'); });
        eq('a1023b U3 the live region names added, deleted and turned-off steps (one: the name, several: the count)', [rows, left, s.h.logged], [[
            ['place', 'announce_added_one:Websuche'],
            ['duplicate one', 'announce_added_one:Alpha'],
            ['duplicate several', 'announce_added_many:2'],
            ['duplicate from the menu', 'announce_added_one:Beta'],
            ['copy', 'copied:1'],
            ['paste', 'announce_added_one:Beta'],
            ['delete with the key', 'announce_deleted_one:Websuche'],
            ['delete several', 'announce_deleted_many:3'],
            ['delete from the menu', 'announce_deleted_one:Beta'],
            ['delete with the node tool', 'announce_deleted_one:Alpha'],
            ['cut says what it did to the clipboard, nothing more', 'copied:1'],
            ['turn off', 'announce_disabled_one:Alpha'],
            ['turn on', 'announce_enabled_one:Alpha'],
            ['turn several off', 'announce_disabled_many:2'],
            ['turn several on', 'announce_enabled_many:2'],
            ['turn off with the node tool', 'announce_disabled_one:Alpha']
        ], 3, []]);
        s.editor.dispose();
    });

    await guardAsync('a1023b U3 keyboard moves are announced once they settle, undo and redo at once, pointer drags never', async () => {
        const s = await session();
        const rows = [];
        const say = async (name, act) => rows.push([name, await s.heard(act)]);
        await say('undo with nothing to undo', () => s.key('z', { ctrlKey: true }));
        s.pick(A);
        const x0 = s.ed.model.node(A).position.x;
        await say('two moves, still moving', () => { s.key('ArrowRight', { shiftKey: true }); s.key('ArrowRight', { shiftKey: true }); });
        await say('the moves settle', () => s.h.runTimers(500));
        const x1 = s.ed.model.node(A).position.x;
        await say('another move settles on its own', async () => { s.key('ArrowDown', { shiftKey: true }); await settle(); s.h.runTimers(500); });
        s.pick(A, B);
        await say('several steps move', async () => { s.key('ArrowLeft', { shiftKey: true }); await settle(); s.h.runTimers(500); });
        // A pointer drag moves the card and says nothing.
        const before = s.ed.model.node(A).position.x;
        await say('a pointer drag', async () => {
            s.pick(A);
            s.card(A).fire('pointerdown', s.h.pe(1, 100, 100));
            s.canvasEl.fire('pointermove', s.h.pe(1, 160, 100));
            s.canvasEl.fire('pointerup', s.h.pe(1, 160, 100));
            await settle();
            s.h.runTimers(500);
        });
        const dragged = s.ed.model.node(A).position.x !== before;
        await say('undo', () => s.key('z', { ctrlKey: true }));
        await say('redo', () => s.key('z', { ctrlKey: true, shiftKey: true }));
        await say('undo from the menu', () => s.menu('undo').action());
        await say('redo from the menu', () => s.menu('redo').action());
        eq('a1023b U3 the live region follows keyboard moves, undo and redo, and a drag stays silent', [rows, x1 - x0, dragged, s.h.logged], [[
            ['undo with nothing to undo', ''],
            ['two moves, still moving', ''],
            ['the moves settle', 'announce_moved_one:Alpha'],
            ['another move settles on its own', 'announce_moved_one:Alpha'],
            ['several steps move', 'announce_moved_many:2'],
            ['a pointer drag', ''],
            ['undo', 'announce_undo'],
            ['redo', 'announce_redo'],
            ['undo from the menu', 'announce_undo'],
            ['redo from the menu', 'announce_redo']
        ], 16, true, []]);
        s.editor.dispose();
    });

    await guardAsync('a1023b U3 removed connections are announced from the wire, the Delete key and the menu', async () => {
        const s = await session();
        const rows = [];
        const say = async (name, act) => rows.push([name, await s.heard(act)]);
        const wire = id => s.canvasEl.querySelectorAll('.ed-edge').find(g => g.dataset.edgeId === id);
        await say('the wire\'s delete button', () => { wire('e2').fire('pointerover'); s.editor.el.querySelector('[data-ed-wire="delete"]').fire('click'); });
        s.ed.model.undo();
        await say('Delete on a selected wire', () => { s.ed.selection.clear(); s.ed.selectedEdge = 'e1'; s.key('Delete'); });
        eq('a1023b U3 the live region names the two ends of a removed connection', [rows, s.ed.model.doc.edges.length, s.h.logged],
            [[['the wire\'s delete button', 'announce_disconnected:Alpha,Beta'], ['Delete on a selected wire', 'announce_disconnected:Start,Alpha']], 1, []]);
        s.editor.dispose();
    });

    await guardAsync('a1023b U3 a started test or live run and each failing step are announced once', async () => {
        const s = await session(req => {
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test') return { run_id: 'r1' };
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run') return { run_id: 'r2' };
            if (req.method === 'GET' && req.url === '/api/desktop/flows/runs/r1') {
                return { run: { id: 'r1', status: 'error', mode: 'test', started_at: '2026-10-23T10:00:00Z' }, steps: [{ node_id: B, status: 'error' }] };
            }
            return undefined;
        }, { enabled: true, flow: { live: flowDoc(), live_revision: 2, published_draft_revision: 3 } });
        const rows = [];
        const say = async (name, act) => rows.push([name, await s.heard(act)]);
        const stream = () => s.h.sources[0];
        const step = (seq, id, status) => stream().emit('event', { seq, type: 'step_finished', node_id: id, step: { node_id: id, status } });
        await say('start a test', async () => {
            s.editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
            await settle();
            s.h.dialogs[s.h.dialogs.length - 1].el.querySelector('[data-ed-action="run"]').fire('click');
        });
        await say('snapshot', () => stream().emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [] }));
        await say('a step starts', () => stream().emit('event', { seq: 1, type: 'step_started', node_id: A }));
        await say('a step succeeds', () => step(2, A, 'success'));
        await say('a step fails', () => step(3, B, 'error'));
        await say('the same step reports again', () => step(4, B, 'error'));
        await say('a reconnect snapshot lists it', () => stream().emit('snapshot', { run: { id: 'r1', status: 'running' }, steps: [{ node_id: B, status: 'error' }] }));
        await say('the run ends', () => stream().emit('end', {}));
        await say('start a live run', () => s.editor.el.querySelector('[data-ed-cmd="run-now"]').fire('click'));
        eq('a1023b U3 the live region says that a run started and which step failed, once, and keeps the final result', [rows, s.h.logged], [[
            ['start a test', 'announce_test_started'],
            ['snapshot', ''],
            ['a step starts', ''],
            ['a step succeeds', ''],
            ['a step fails', 'announce_step_failed:Beta'],
            ['the same step reports again', ''],
            ['a reconnect snapshot lists it', ''],
            ['the run ends', 'run_failed_at:Beta'],
            ['start a live run', 'announce_run_started']
        ], []]);
        eq('a1023b U3 the announcements exist in all 16 locales', missingKeys(apps, NEW_KEYS.u3), []);
        s.editor.dispose();
    });
}
