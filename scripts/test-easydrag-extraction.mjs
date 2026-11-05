// Atomic extraction uses the real editor/model/saver and a controlled CAS transport.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const T = 'n_tttttttt', A = 'n_aaaaaaaa', B = 'n_bbbbbbbb', C = 'n_cccccccc';
    const BLOCK = 'flow_' + 'a'.repeat(26), KEY = 'aurago.easydrag.draft.f1';
    const copy = value => JSON.parse(JSON.stringify(value));
    const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
    const ids = { block_id: BLOCK, call_id: C, input_id: 'n_dddddddd', output_id: 'n_eeeeeeee', entry_edge_id: 'e_ffffffff', exit_edge_id: 'e_gggggggg' };
    const preview = () => ({ base_revision: 3, ids: copy(ids), digest: 'a'.repeat(64), parent_hash: 'b'.repeat(64), block_hash: 'c'.repeat(64), block_revision: 1, review: { inputs: [], outputs: [{ name: 'greeting', type: 'string', required: true, nullable: false }], groups_moved: 1, groups_shrunk: 0, extra_nodes: 3, extra_edges: 2, budget_note: true } });
    function doc() {
        return { schema: 1, kind: 'flow', name: 'Parent', description: 'Retain metadata', notes: [{ id: 'note', text: 'Retain note', position: { x: 3, y: 4 } }], settings: {}, nodes: [
            { id: T, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: A, key: 'greeting', type: 'logic.set', label: '<Chosen & step>', position: { x: 300, y: 0 }, params: { keep_input: false, fields: [{ name: 'greeting', type: 'text', value: 'Hello' }] }, settings: {} },
            { id: B, key: 'next', type: 'logic.set', label: 'Next', position: { x: 600, y: 0 }, params: { keep_input: false, fields: [] }, settings: {} }
        ], edges: [{ id: 'e_aaaaaaaa', source: { node: T, port: 'out' }, target: { node: A, port: 'in' } }, { id: 'e_bbbbbbbb', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }], groups: [{ id: 'g_aaaaaaaa', title: 'Moved', color: 'blue', members: [A] }] };
    }
    function fixture(options = {}) {
        let stored = doc(), revision = 3, committed = false;
        let answer;
        const h = sandbox(req => {
            if (answer) { const value = answer(req); if (value !== undefined) return value; }
            if (req.url.endsWith('/extract/preview')) return options.preview || Object.assign(preview(), { base_revision: revision });
            if (req.url.endsWith('/extract/apply')) {
                const parent = copy(stored); parent.nodes = parent.nodes.filter(node => node.id !== A);
                // Intentionally put the new call first: exact ordering must survive Ack/Undo/Redo.
                parent.nodes.unshift({ id: C, key: 'greeting', type: 'component.call', label: 'Extracted', position: { x: 300, y: 0 }, params: { flow_id: BLOCK, revision: 1, inputs: {} }, settings: { retry: { count: 0 } } });
                parent.edges.forEach(edge => { if (edge.source.node === A) edge.source.node = C; if (edge.target.node === A) edge.target.node = C; });
                parent.groups = []; stored = parent; revision++; committed = true;
                const result = records();
                if (options.apply) return options.apply(result);
                return result;
            }
            if (req.url.endsWith('/extract/reconcile')) return committed ? Object.assign({ status: 'applied_state_confirmed' }, records()) : { status: 'unknown_or_conflict' };
            if (req.url === '/api/desktop/flows/f1' && req.method === 'GET') return { flow: records().flow };
            if (req.url === '/api/desktop/flows/f1' && req.method === 'PUT') {
                if (req.body.base_revision !== revision) throw Object.assign(new Error('conflict'), { status: 409, body: { code: 'FLOW_REVISION_CONFLICT' } });
                stored = copy(req.body.doc); return { draft_revision: ++revision, issues: [] };
            }
        });
        function records() { return { flow: { id: 'f1', draft: copy(stored), draft_revision: revision, live_revision: 0, published_draft_revision: 0, live: null }, block: { id: BLOCK, live_revision: 1, draft_revision: 1 } }; }
        const modal = h.ED.core.modal;
        h.ED.core.modal = (host, options) => { h.lastModalBody = options.body; return modal(host, options); };
        const editor = openEditor(h, { readonly: options.readonly, flow: { draft: doc() } });
        editor.ed.selection.add(A);
        const controller = () => h.menus.find(menu => menu.id === 'edit').items.find(item => item.id === 'extract-selection');
        return { h, editor, records, stored: () => copy(stored), setAnswer(fn) { answer = fn; }, async open() { await controller().action(); await settle(); return h.dialogs.at(-1); } };
    }
    async function click(d, action) { d.el.querySelector('[data-ed-action="' + action + '"]').fire('click'); await settle(); }
    const extracts = f => f.h.requests.filter(req => req.url.includes('/extract/'));

    await guardAsync('extraction exact acknowledgement is saved once and remains undoable', async () => {
        const f = fixture(), { h, editor } = f, before = editor.ed.model.toJSON(), d = await f.open();
        eq('extraction review names the actual selected step and key as escaped text', [h.lastModalBody.includes('&lt;Chosen &amp; step&gt;'), h.lastModalBody.includes('<code>greeting</code>'), d.body.querySelector('.ed-extraction-selection').querySelectorAll('chosen').length], [true, true, 0]);
        await click(d, 'preview');
        eq('extraction validates the real 26-character block ID and displays the contract', [d.el.querySelector('[data-ed-action="apply"]').hidden, d.body.querySelector('[data-ed-extract-review]').html.includes('greeting')], [false, true]);
        eq('extraction constant-only review explicitly shows no required block inputs', d.body.querySelector('[data-ed-extract-review]').html.includes('<p class="ed-hint">option_none</p>'), true);
        await click(d, 'apply'); await editor.ed.saver.flush();
        eq('extraction ack adopts exact order and revision without a second PUT', [editor.ed.model.toJSON(), editor.ed.saver.revision, editor.ed.saver.state, h.puts().length, h.store.has(KEY), [...editor.ed.selection]], [f.stored(), 4, 'saved', 0, false, [C]]);
        editor.ed.model.undo(); await editor.ed.saver.flush();
        eq('extraction Undo restores the complete parent at the acknowledged CAS', [editor.ed.model.toJSON(), h.puts().at(-1).body.base_revision, f.stored(), editor.ed.saver.revision], [before, 4, before, 5]);
        editor.ed.model.redo(); await editor.ed.saver.flush();
        eq('extraction Redo edits the parent without creating another block', [editor.ed.model.doc.nodes[0].id, extracts(f).filter(req => req.url.endsWith('/apply')).length], [C, 1]); editor.dispose();
    });

    await guardAsync('extraction uncertain response holds autosave until a read-only reconciliation', async () => {
        const f = fixture({ apply: () => { throw new TypeError('lost response'); } }), d = await f.open();
        await click(d, 'preview'); await click(d, 'apply');
        eq('extraction uncertainty preserves original model and emergency copy', [f.editor.ed.model.node(A).key, await f.editor.ed.saver.flush(), f.h.puts().length, f.h.store.has(KEY), f.editor.el.querySelector('[data-ed-extraction-status]').hidden], ['greeting', false, 0, true, false]);
        await click(d, 'reconcile');
        eq('extraction reconciliation sends only the original proof and adopts a verified snapshot', [extracts(f).at(-1).body, f.editor.ed.model.toJSON(), f.editor.ed.saver.state, f.h.puts().length], [{ base_revision: 3, block_id: BLOCK, parent_hash: 'b'.repeat(64), block_hash: 'c'.repeat(64) }, f.stored(), 'saved', 0]); f.editor.dispose();
    });

    await guardAsync('extraction late acknowledgement preserves newer edits and requires explicit recovery', async () => {
        const waiting = deferred(), f = fixture({ apply: result => { waiting.result = result; return waiting.promise; } }), d = await f.open();
        await click(d, 'preview'); await click(d, 'apply');
        f.editor.ed.model.setFlow({ description: 'Later local edit' }); f.h.runTimers(1000); await settle();
        waiting.resolve(waiting.result); await settle();
        eq('extraction late ack never overwrites later edits or starts autosave', [f.editor.ed.model.doc.description, !!f.editor.ed.model.node(A), f.h.puts().length, d.body.querySelector('.ed-error').textContent], ['Later local edit', true, 0, 'extraction_conflict']);
        await click(d, 'reconcile');
        eq('extraction reconcile cannot overwrite the changed model either', [f.editor.ed.model.doc.description, f.h.puts().length], ['Later local edit', 0]);
        f.h.confirmAnswer = true; await click(d, 'recover');
        const conflict = f.h.dialogs.at(-1); await click(conflict, 'keep'); await settle();
        eq('extraction explicit Keep saves local edits against the server revision without another Apply', [f.stored().description, f.h.puts().at(-1).body.base_revision, extracts(f).filter(req => req.url.endsWith('/apply')).length], ['Later local edit', 4, 1]); f.editor.dispose();
    });

    await guardAsync('extraction preview fences names selection IME readonly and unsupported reasons', async () => {
        const f = fixture(), d = await f.open(), input = d.body.querySelector('[data-ed-extract-name]');
        input.fire('compositionstart'); await click(d, 'preview');
        eq('extraction IME never posts a partial block name', extracts(f).length, 0);
        input.fire('compositionend'); input.value = '\uD800'; await click(d, 'preview');
        eq('extraction invalid Unicode remains visible without a request', [input.value, extracts(f).length], ['\uD800', 0]);
        input.value = 'Valid'; f.editor.ed.selection.clear(); await click(d, 'preview');
        eq('extraction selection changes invalidate the review baseline', [extracts(f).length, d.body.querySelector('.ed-error').textContent], [0, 'extraction_stale']); f.editor.dispose();
        const ro = fixture({ readonly: true }); await ro.open();
        eq('extraction readonly public command opens no dialog and sends nothing', [ro.h.dialogs.length, extracts(ro).length], [0, 0]); ro.editor.dispose();
        const bad = fixture(); bad.setAnswer(req => { if (req.url.endsWith('/extract/preview')) throw Object.assign(new Error('<private>'), { status: 422, body: { code: 'FLOW_EXTRACTION_UNSUPPORTED', reason: 'branch' } }); });
        const bd = await bad.open(); await click(bd, 'preview');
        eq('extraction unsupported reason uses only a fixed localized action hint', bd.body.querySelector('.ed-error').textContent, 'extraction_reason_branch'); bad.editor.dispose();
    });

    await guardAsync('extraction definitive refusal releases the hold while unknown or closed replies do not', async () => {
        const f = fixture(), d = await f.open();
        f.setAnswer(req => { if (req.url.endsWith('/extract/apply')) throw Object.assign(new Error('conflict'), { status: 409, body: { code: 'FLOW_EXTRACTION_CONFLICT' } }); });
        await click(d, 'preview'); await click(d, 'apply');
        eq('extraction coded refusal leaves parent unchanged and saved', [f.editor.ed.saver.state, f.editor.ed.model.node(A).key, f.h.store.has(KEY)], ['saved', 'greeting', false]); f.editor.dispose();
        const waiting = deferred(), late = fixture({ apply: result => { waiting.result = result; return waiting.promise; } }), ld = await late.open();
        await click(ld, 'preview'); await click(ld, 'apply'); ld.close(); await settle(); waiting.resolve(waiting.result); await settle();
        eq('extraction closing an in-flight Apply keeps the hold and ignores the late reply', [late.editor.ed.model.node(A).key, await late.editor.ed.saver.flush(), late.h.puts().length, late.editor.el.querySelector('[data-ed-extraction-status]').hidden], ['greeting', false, 0, false]);
        late.h.confirmAnswer = false; eq('extraction leave explicitly warns even when the original model was clean', [await late.editor.leave(), late.h.confirms.at(-1)], [false, 'extraction_leave']);
        late.editor.dispose(); eq('extraction forced disposal retains the local emergency draft', late.h.store.has(KEY), true);
    });

    await guardAsync('extraction scrubbed review names remain display-only and bounded', async () => {
        const p = preview(); p.review.outputs[0].name = '[redacted]_field';
        const f = fixture({ preview: p }), d = await f.open(); await click(d, 'preview');
        eq('extraction masked technical field is visible with a redaction notice', [d.el.querySelector('[data-ed-action="apply"]').hidden, d.body.querySelector('[data-ed-extract-review]').html.includes('[redacted]_field'), d.body.querySelector('[data-ed-extract-review]').html.includes('metadata_hidden')], [false, true, true]);
        await click(d, 'apply');
        eq('extraction Apply never sends scrubbed display fields', Object.keys(extracts(f).find(req => req.url.endsWith('/apply')).body).sort(), ['base_revision', 'digest', 'ids', 'name', 'node_ids']);
        p.review.outputs[0].name = 'x'.repeat(2000) + '[redacted]';
        eq('extraction oversized display names are rejected', f.h.ED.extraction.validPreview(p, 3), false); f.editor.dispose();
    });

    await guardAsync('extraction saves pending edits before preview and fences late read replies', async () => {
        const f = fixture(); f.editor.ed.model.setFlow({ description: 'Pending before preview' });
        const d = await f.open(); await click(d, 'preview');
        const writes = f.h.requests.filter(req => req.method !== 'GET');
        eq('extraction saves the current draft before observing the preview revision', [writes[0].method, writes[0].body.base_revision, writes.at(-1).body.base_revision, f.editor.ed.saver.revision], ['PUT', 3, 4, 4]); f.editor.dispose();
        const late = fixture(), wait = deferred(); late.setAnswer(req => req.url.endsWith('/extract/preview') ? wait.promise : undefined);
        const ld = await late.open(); await click(ld, 'preview'); ld.close(); await settle(); wait.resolve(preview()); await settle();
        eq('extraction a closed preview cannot enable Apply or change the model', [ld.el.querySelector('[data-ed-action="apply"]').hidden, !!late.editor.ed.model.node(A), late.h.puts().length], [true, true, 0]); late.editor.dispose();
    });

    await guardAsync('extraction malformed success and unknown reconcile never release autosave', async () => {
        const f = fixture({ apply: () => ({ flow: null }) }), d = await f.open(); await click(d, 'preview'); await click(d, 'apply');
        f.setAnswer(req => req.url.endsWith('/extract/reconcile') ? { status: 'unknown_or_conflict' } : undefined);
        await click(d, 'reconcile'); f.h.runTimers(); await settle();
        eq('extraction malformed acknowledgement and absent proof remain honestly uncertain', [await f.editor.ed.saver.flush(), f.h.puts().length, !!f.editor.ed.model.node(A), d.body.querySelector('.ed-error').textContent], [false, 0, true, 'extraction_unknown']); f.editor.dispose();
    });

    await guardAsync('extraction dismissed conflict question preserves the hold for explicit recovery', async () => {
        const f = fixture({ apply: () => { throw new Error('lost'); } }), d = await f.open(); await click(d, 'preview'); await click(d, 'apply');
        const ticket = f.editor.ed.saver.beginExternal(3, f.editor.ed.model.version);
        eq('extraction cannot open a second hold while the transaction is uncertain', ticket, null);
        f.h.confirmAnswer = true; await click(d, 'recover');
        f.h.dialogs.at(-1).close(null); await settle(); f.h.runTimers(); await settle();
        eq('extraction a dismissed conflict choice neither retries writes nor forgets uncertainty', [await f.editor.ed.saver.flush(), f.h.puts().length, f.editor.el.querySelector('[data-ed-extraction-status]').hidden], [false, 0, false]);
        const again = await f.open(); await click(again, 'recover'); await click(f.h.dialogs.at(-1), 'reload'); await settle();
        eq('extraction a later explicit Reload adopts the current server draft without Apply or PUT', [f.editor.ed.model.toJSON(), f.editor.ed.saver.revision, f.h.puts().length, extracts(f).filter(req => req.url.endsWith('/apply')).length], [f.stored(), 4, 0, 1]); f.editor.dispose();
    });

    await guardAsync('extraction synchronous edits during acknowledgement cannot be marked saved', async () => {
        const f = fixture(), d = await f.open(); await click(d, 'preview');
        let edited = false;
        const off = f.editor.ed.model.on(() => {
            if (!edited && f.editor.ed.model.node(C)) { edited = true; f.editor.ed.model.setFlow({ description: 'Reentrant local edit' }); }
        });
        await click(d, 'apply'); off();
        eq('extraction an extra synchronous edit stays dirty at the old CAS with no blind PUT', [f.editor.ed.model.doc.description, f.editor.ed.saver.revision, f.editor.ed.saver.isDirty(), await f.editor.ed.saver.flush(), f.h.puts().length, d.body.querySelector('.ed-error').textContent], ['Reentrant local edit', 3, true, false, 0, 'extraction_conflict']);
        eq('extraction reentrant recovery retains the latest emergency copy', JSON.parse(f.h.store.get(KEY)).doc.description, 'Reentrant local edit');
        f.editor.dispose();
    });
}
