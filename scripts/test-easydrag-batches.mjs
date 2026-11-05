// Native form/mapping behavior for the bounded serial block-list node; execution is server-owned.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const field = (name, type = 'string', required = true) => ({ name, type, required, nullable: false });
    const shape = { inputs: [field('name'), field('suffix', 'string', false)], outputs: [field('greeting')] };
    const newer = { inputs: [field('person'), field('suffix', 'string', false)], outputs: [field('greeting')] };
    function fixture(readonly = false, extra) {
        const h = sandbox(req => {
            const result = extra && extra(req); if (result !== undefined) return result;
            if (req.url.startsWith('/api/desktop/flow-blocks?')) return { blocks: [{ id: 'block_a', name: 'Greeting', live_revision: 2 }] };
            if (req.url.includes('/versions?')) return { versions: [{ revision: 2 }, { revision: 1 }] };
            if (req.url.endsWith('/versions/1')) return { doc: { kind: 'block' }, contract: shape };
            if (req.url.endsWith('/versions/2')) return { doc: { kind: 'block' }, contract: newer };
            if (req.url.includes('/dependents?')) return { items: [] };
        });
        const types = new Map(h.catalog.types);
        types.set('component.each', { type: 'component.each', type_version: 1, label: 'For each item', category: 'logic', availability: { state: 'available' }, inputs: ['in'], outputs: ['out'], params: ['items', 'max_items', 'concurrency', 'on_item_error'].map(name => ({ name, required: true })), output_fields: [{ name: 'items', type: 'list', primary: true }, ...['count', 'succeeded', 'failed'].map(name => ({ name, type: 'number' }))] });
        h.catalog = { types, list: [...types.values()], categories: [{ id: 'logic', label: 'Logic' }], blockContracts: new Map() }; h.blockApi = h.ED.blocks.api(h.transport);
        const editor = openEditor(h, { readonly });
        const id = editor.ed.model.addNode('component.each', { x: 900, y: 0 }, { flow_id: 'block_a', revision: 1, item_field: 'name', items: ['Anna', 'Ben'], inputs: { suffix: '!' } });
        editor.ed.model.connect('n_bbbbbbbb', 'out', id, 'in');
        const node = () => editor.ed.model.node(id);
        const open = async () => { editor.ed.bus.emit('open-detail', { nodeId: id }); await settle(); };
        return { h, editor, id, node, open };
    }
    await guardAsync('batches authoring uses exact defaults and reserves an explicit typed input for each whole item', async () => {
        const { h, editor, node, open } = fixture(); await open();
        eq('batches productive repeat icon resolves without the generic tool fallback', h.ED.core.icon('repeat') === h.ED.core.icon('tool'), false);
        eq('batches defaults are persisted and execution stays serial', [node().params.max_items, node().params.concurrency, node().params.on_item_error], [100, 1, 'stop']);
        eq('batches per-item target is explicit and absent from shared controls', [editor.el.querySelector('[data-ed-batch-item-field]').value, !!editor.el.querySelector('.ed-field[data-param="inputs.name"]'), !!editor.el.querySelector('.ed-field[data-param="inputs.suffix"]')], ['name', false, true]);
        eq('batches schemas contain only the static list and counts', editor.ed.model.fieldsOf(node()), [{ name: 'items', type: 'list', primary: true }, { name: 'count', type: 'number' }, { name: 'succeeded', type: 'number' }, { name: 'failed', type: 'number' }]);
        const select = editor.el.querySelector('[data-ed-batch-item-field]'); select.value = 'suffix'; select.fire('change'); await settle();
        eq('batches choosing an item field never silently drops its former shared value', [node().params.item_field, node().params.inputs, !!editor.el.querySelector('[data-ed-block-unknown-remove]')], ['suffix', { suffix: '!' }, true]);
        editor.el.querySelector('[data-ed-block-unknown-remove]').fire('click'); await settle();
        eq('batches a conflicting shared binding is removed only by explicit action', node().params.inputs, {});
        eq('batches configuring the batch performs no execution requests', h.requests.some(req => /\/test$|\/simulate$|\/publish$/.test(req.url)), false);
        editor.dispose();
    });
    await guardAsync('batches list JSON parsing is preflighted and rejected text stays visible', async () => {
        const { editor, node, open } = fixture(); await open();
        let area = editor.el.querySelector('.ed-field[data-param="items"] textarea');
        for (const raw of ['{', 'null', '["' + 'x'.repeat(3 * 1024 * 1024) + '"]', '['.repeat(70) + '0' + ']'.repeat(70)]) {
            area.value = raw; area.fire('input');
            eq('batches invalid/oversized JSON is preserved without replacing the prior list ' + raw.length, [area.value.length, area.getAttribute('aria-invalid'), node().params.items, editor.ed.flushPendingEdits()], [raw.length, 'true', ['Anna', 'Ben'], false]);
        }
        area.value = '["{{literal data}}",null,0,false]'; area.fire('input'); area.fire('change'); await settle();
        eq('batches list items preserve templates null zero and false as data', node().params.items, ['{{literal data}}', null, 0, false]);
        const limit = editor.el.querySelector('.ed-field[data-param="max_items"] input'); limit.min = '1'; limit.max = '1000'; limit.value = '1001'; limit.fire('change');
        eq('batches out-of-range limit remains an invalid edit instead of being clamped', [node().params.max_items, editor.ed.flushPendingEdits()], [100, false]);
        limit.value = '3'; limit.fire('change');
        const policy = editor.el.querySelector('.ed-field[data-param="on_item_error"] select'); policy.value = 'collect'; policy.fire('change'); await settle();
        eq('batches max-items and collect are explicit saved options', [node().params.max_items, node().params.on_item_error, node().params.items.length], [3, 'collect', 4]);
        editor.dispose();
    });
    await guardAsync('batches literal parameter budgets use the document scale rather than preview scale', async () => {
        const { h, editor } = fixture(); const parse = h.ED.batches.parseItems;
        eq('batches literal lists above64KiB are allowed without changing their data', parse('["' + 'x'.repeat(1024 * 1024) + '"]')[0].length, 1024 * 1024);
        let utf8 = false, deep = false; try { parse('["' + '😀'.repeat(550000) + '"]'); } catch (_) { utf8 = true; } try { parse('['.repeat(70) + '0' + ']'.repeat(70)); } catch (_) { deep = true; }
        eq('batches parameter preflight rejects oversized UTF8 and deeply nested data before clone', [utf8, deep], [true, true]); editor.dispose();
    });
    await guardAsync('batches literal items survive key rename copy paste and local field previews', async () => {
        const { editor, node, id, open } = fixture(); const model = editor.ed.model, source = 'n_aaaaaaaa';
        const literal = ['{{alpha.results}}', { nested: '{{missing.field}}' }];
        model.setParam(id, 'items', literal); model.setParam(id, 'inputs', { suffix: '{{alpha.results}}' });
        editor.ed.lastRunData = { label: 'Last run', roots: { alpha: { results: ['real value'] } } }; await open();
        eq('batches literal arrays never receive false missing-template preview errors', editor.el.querySelector('.ed-field[data-param="items"] .ed-field-preview').classList.contains('is-error'), false);
        model.setKey(source, 'renamed');
        eq('batches key renames update shared templates while preserving literal items', [node().params.items, node().params.inputs.suffix], [literal, '{{renamed.results}}']); model.undo();
        const pasted = model.paste(model.fragment([source, id]), { x: 1200, y: 400 });
        const copy = pasted.map(key => model.node(key)).find(n => n.type === 'component.each'), copySource = pasted.map(key => model.node(key)).find(n => n.type !== 'component.each');
        eq('batches paste rewrites only real templates and never strings inside an item', [copy.params.items, copy.params.inputs.suffix], [literal, '{{' + copySource.key + '.results}}']);
        model.setParam(id, 'items', '{{alpha.results}}'); model.setKey(source, 'renamed');
        eq('batches the top-level list expression still follows key renames', node().params.items, '{{renamed.results}}'); editor.dispose();
    });
    await guardAsync('batches ordinary mapping replaces a fixed empty list with one whole expression', async () => {
        const { editor, node, id, open } = fixture(); editor.ed.model.setParam(id, 'items', []); await open();
        const picker = editor.el.querySelector('[data-ed-mapping-target]'); picker.value = 'items'; picker.fire('change');
        const row = editor.el.querySelectorAll('[data-ref]').find(element => element.dataset.ref === 'alpha.results'); row.fire('click');
        eq('batches mapping into [] replaces the entire field instead of concatenating text', node().params.items, '{{alpha.results}}');
        eq('batches mapped list retains ordinary form save semantics', editor.ed.flushPendingEdits(), true); editor.dispose();
    });
    await guardAsync('batches list mapping uses the existing target and preserves an unfinished composition', async () => {
        const { h, editor, node, open } = fixture(); await open();
        const picker = editor.el.querySelector('[data-ed-mapping-target]');
        eq('batches mapping includes the list and only actual shared input fields', picker.querySelectorAll('option').map(option => option.getAttribute('value')), ['', 'items', 'inputs.suffix']);
        picker.value = 'items'; picker.fire('change');
        editor.el.querySelector('[data-ed-mode="items"]').fire('click');
        const view = editor.el.querySelector('.ed-field[data-param="items"] .ed-tpl-view'); view.fire('click');
        const area = editor.el.querySelector('.ed-field[data-param="items"] textarea'); area.value = '{{alpha.text}}'; area.fire('focusin'); area.fire('compositionstart');
        eq('batches a pending IME composition cannot be cut off for save or version review', editor.ed.flushPendingEdits(), false);
        area.fire('compositionend'); area.fire('change'); editor.ed.flushPendingEdits();
        eq('batches one full expression is stored without re-evaluating item strings', node().params.items, '{{alpha.text}}');
        area.value = 'prefix {{alpha.text}}'; area.fire('change');
        eq('batches interpolated text is not accepted as a list expression', editor.ed.flushPendingEdits(), false);
        eq('batches local mapping does not send data to a backend endpoint', h.requests.every(req => req.method === 'GET' || req.url.endsWith('/validate')), true);
        editor.dispose();
    });
    await guardAsync('batches immutable upgrades preserve options and visibly retain an unavailable item field', async () => {
        const { h, editor, node, open } = fixture(); await open();
        const before = JSON.parse(JSON.stringify(node().params));
        const select = editor.el.querySelector('[data-ed-call-version]'); select.value = '2'; select.fire('change');
        editor.el.querySelector('[data-ed-call-apply]').fire('click'); await settle();
        eq('batches staged version review leaves the parent unchanged', node().params, before);
        h.dialogs.at(-1).el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('batches version upgrade retains list limits policy and the missing item-field identity', [node().params.revision, node().params.items, node().params.max_items, node().params.concurrency, node().params.on_item_error, node().params.item_field, editor.el.querySelector('[data-ed-batch-item-field]').getAttribute('aria-invalid')], [2, before.items, 100, 1, 'stop', 'name', 'true']);
        editor.ed.model.undo(); await settle();
        eq('batches one undo restores the complete pinned configuration', node().params, before);
        editor.dispose();
    });
    await guardAsync('batches retained item diagnostics are bounded and distinguish total work from visible data', async () => {
        const { h, editor } = fixture(); await settle(); const b = h.ED.batches;
        const step = { status: 'success', output: { items: [{ index: 0, status: 'success', output: '<img src=x>' }, { index: 1, status: 'error', error_code: 'FLOW_VALUE_TYPE' }], count: 2, succeeded: 1, failed: 1 } };
        const html = b.resultMarkup(editor.ed, step, false);
        eq('batches result markup escapes payload and labels a successful batch containing item failures', [html.includes('&lt;img'), html.includes('data-ed-batch-results'), html.includes('value_type')], [true, true, true]);
        const many = { status: 'success', output: { items: Array.from({ length: 1000 }, (_, index) => ({ index, status: 'success', output: {} })), count: 1000, succeeded: 1000, failed: 0 } };
        eq('batches large output draws at most50item rows and retains the visible-versus-total explanation', [(b.resultMarkup(editor.ed, many, false).match(/<li>/g) || []).length, b.resultMarkup(editor.ed, many, false).includes('batch_result_visible')], [50, true]);
        const omitted = b.resultMarkup(editor.ed, { status: 'success', output_truncated: true, output: { _preview: 'partial', _omitted: 'run_storage_budget' } }, false);
        eq('batches omitted counters remain unknown and a stopped batch promises no partial output', [omitted.includes('batch_result_incomplete'), omitted.includes('batch_result_summary'), b.resultMarkup(editor.ed, { status: 'error', output: step.output }, false)], [true, false, '']);
        eq('batches only the stable index-and-code grammar is translated', [h.ED.core.stepErrorText(editor.ed.t, { error_code: 'FLOW_BATCH_ITEM_FAILED', error_message: 'batch item 17 failed (FLOW_VALUE_TYPE)' }), h.ED.core.stepErrorText(editor.ed.t, { error_code: 'FLOW_BATCH_ITEM_FAILED', error_message: 'malicious <payload>' })], ['batch_item_failed_at:17 error_flow_value_type', 'error_flow_batch_item_failed']);
        editor.dispose();
    });
    await guardAsync('batches readonly controls and retry repair retain the existing call safety contracts', async () => {
        const { editor, node, open } = fixture(); await open(); editor.ed.model.setSettings(node().id, { retry: { count: 5, delay_seconds: 9 } });
        editor.el.querySelector('[data-ed-pane="settings"]').fire('click');
        const count = editor.el.querySelector('[data-k="count"]'), timeout = editor.el.querySelector('[data-k="timeout"]'); timeout.value = '18'; timeout.fire('change');
        eq('batches timeout edits preserve invalid imported retries and retry UI remains locked', [count.disabled, node().settings.retry.count], [true, 5]);
        editor.el.querySelector('[data-ed-block-reset-retry]').fire('click'); eq('batches explicit retry repair is undoable', node().settings.retry.count, 0); editor.ed.model.undo(); eq('batches undo restores imported retry evidence', node().settings.retry.count, 5); editor.dispose();
        const ro = fixture(true); await ro.open();
        eq('batches readonly version item target list and policy controls cannot mutate', [ro.editor.el.querySelector('[data-ed-call-apply]').disabled, ro.editor.el.querySelector('[data-ed-batch-item-field]').disabled, ro.editor.el.querySelector('.ed-field[data-param="items"] textarea').disabled, ro.editor.el.querySelector('.ed-field[data-param="on_item_error"] select').disabled], [true, true, true, true]);
        ro.editor.dispose();
    });
    await guardAsync('batches unsupported imported settings remain visible until explicit repair', async () => {
        const { editor, node, id, open } = fixture(); editor.ed.model.setParam(id, 'concurrency', 2); editor.ed.model.setParam(id, 'on_item_error', 'future_policy'); await open();
        eq('batches unsupported policy remains selected and unsupported concurrency is visible', [editor.el.querySelector('.ed-field[data-param="on_item_error"] select').value, editor.el.querySelector('[data-ed-batch-reset-concurrency]').parentElement.hidden, node().params.concurrency], ['future_policy', false, 2]);
        editor.el.querySelector('[data-ed-batch-reset-concurrency]').fire('click');
        eq('batches serial repair changes only its explicitly selected setting', [node().params.concurrency, node().params.on_item_error], [1, 'future_policy']); editor.ed.model.undo();
        eq('batches serial repair is one undo', node().params.concurrency, 2); editor.dispose();
    });
    await guardAsync('batches actual palette creation supplies defaults without repairing loaded damaged documents', async () => {
        const { h, editor, id } = fixture(); await settle(); editor.ed.selection.clear(); editor.ed.bus.emit('selection', editor.ed.selection);
        const item = editor.el.querySelector('[data-ed-type="component.each"]'); item.fire('keydown', { key: 'Enter' }); await settle();
        const added = editor.ed.model.doc.nodes.find(n => n.type === 'component.each' && n.id !== id);
        eq('batches palette-created nodes have explicit safe values despite required catalog params without defaults', added.params, { items: [], item_field: '', inputs: {}, max_items: 100, concurrency: 1, on_item_error: 'stop' });
        editor.ed.bus.emit('open-detail', { nodeId: added.id }); await settle();
        eq('batches newly created nodes have no concurrency repair warning', editor.el.querySelector('[data-ed-batch-reset-concurrency]').parentElement.hidden, true);
        const doc = editor.ed.model.toJSON(); delete doc.nodes.find(n => n.id === added.id).params.concurrency;
        const loaded = h.ED.model.create(doc, h.catalog);
        eq('batches loading an incomplete imported node never fills required settings', Object.hasOwn(loaded.node(added.id).params, 'concurrency'), false); editor.dispose();
    });
    await guardAsync('batches work-budget issues explain the remedy and focus Maximum items directly', async () => {
        const { h, editor, id } = fixture(); await settle();
        const issue = { code: 'BATCH_WORK_LIMIT', node_id: id, param: 'max_items', severity: 'error', message: 'Do not interpret this free-form text' };
        editor.ed.issues = [issue];
        const publish = h.ED.publish.create(editor.ed); publish.focusIssue(issue); await settle(); h.flushFrames();
        eq('batches budget guidance uses the stable translated issue code and targets the actual numeric field', [h.ED.core.issueText(editor.ed.t, issue), h.dom.document.activeElement.closest('.ed-field').dataset.param], ['issue_batch_work_limit', 'max_items']);
        publish.dispose(); editor.dispose();
    });
    await guardAsync('batches damaged object-valued selections remain repairable without coercion crashes', async () => {
        const { h, editor, node, id, open } = fixture(); const broken = { toString: 'not callable' };
        editor.ed.model.setParam(id, 'on_item_error', broken); editor.ed.model.setParam(id, 'item_field', broken); await open();
        eq('batches malformed selection values never crash the inspector or replace stored data', [!!editor.ed.detail, h.logged.length, node().params.on_item_error, !!editor.el.querySelector('[data-ed-batch-item-field]')], [true, 0, broken, true]);
        const select = editor.el.querySelector('.ed-field[data-param="on_item_error"] select'); select.focus(); editor.ed.flushPendingEdits();
        eq('batches flushing an invalid display placeholder preserves the authoritative raw value', node().params.on_item_error, broken);
        editor.el.querySelector('[data-ed-batch-item-field]').fire('change');
        eq('batches a synthetic change on the opaque item-field option cannot overwrite its raw object', node().params.item_field, broken);
        select.value = 'collect'; select.fire('change'); const item = editor.el.querySelector('[data-ed-batch-item-field]'); item.value = 'name'; item.fire('change');
        eq('batches malformed selections can be explicitly repaired through their normal controls', [node().params.on_item_error, node().params.item_field], ['collect', 'name']);
        const forged = h.ED.batches.resultMarkup(editor.ed, { status: 'success', output: { items: [{ index: 0, status: 'success', output: { attachment: { $type: 'file', name: broken } } }], count: 1, succeeded: 1, failed: 0 } }, false);
        eq('batches object-valued forged file names stay inert bounded text', [forged.includes('data-ed-batch-results'), forged.includes('<img')], [true, false]); editor.dispose();
    });
    await guardAsync('batches historical run view labels dirty offline and failed states as the draft without rebinding its saver', async () => {
        for (const state of ['dirty', 'offline', 'failed']) {
            const { editor, h } = fixture(false, req => { if (req.method === 'PUT' && !req.url.endsWith('/validate')) { if (state === 'offline') throw new TypeError('Failed to fetch'); if (state === 'failed') throw Object.assign(new Error('locked'), { body: { code: 'FLOW_LOCKED' } }); } });
            await settle(); const ed = editor.ed, saver = ed.saver;
            const text = () => editor.el.querySelector('[data-ed-save] .ed-foot-text').textContent;
            eq('batches the ordinary draft has its ordinary dirty label for ' + state, text(), 'save_dirty');
            ed.enterRunView({ run: { id: 'r1', flow_id: 'f1', revision: 3, mode: 'test', status: 'success', started_at: '2026-11-05T00:00:00Z' }, doc: ed.model.toJSON(), steps: [] });
            if (state !== 'dirty') await saver.flush(); await settle();
            eq('batches run view qualifies the real draft save state ' + state, [text(), editor.el.querySelector('[data-ed-save]').hidden, ed.saver === saver, ed.saver.state], ['save_draft_status:save_' + state, false, true, state]);
            ed.exitRunView(); await settle(); eq('batches leaving run view restores ordinary save text for ' + state, text(), 'save_' + state); editor.dispose();
        }
    });
}
