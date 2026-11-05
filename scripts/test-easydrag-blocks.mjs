// User-level block editing, typed-value and immutable call regressions on the existing DOM harness.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const A = 'n_aaaaaaaa', B = 'n_bbbbbbbb';
    const field = (name, type = 'string', required = true, nullable = false) => ({ name, type, required, nullable });
    function fixture(answer = () => undefined, options = {}) {
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flow-blocks/validate') return { valid: true, issues: [] };
            return answer(req);
        });
        const types = new Map(h.catalog.types);
        for (const [type, label, inputs, outputs] of [['block.input', 'Block input', [], ['out']], ['block.output', 'Block output', ['in'], []], ['component.call', 'Call block', ['in'], ['out']]]) types.set(type, { type, label, type_version: 1, category: 'logic', inputs, outputs, params: [], output_fields: [], availability: { state: 'available' } });
        h.catalog = { types, list: Array.from(types.values()), categories: [], blockContracts: new Map() };
        h.blockApi = h.ED.blocks.api(h.transport);
        let opts = options;
        if (options.block) opts = Object.assign({}, options, { flow: { id: 'f1', name: 'Pure block', draft_revision: 3, live_revision: 0, draft: { schema: 1, kind: 'block', name: 'Pure block', nodes: [
            { id: A, type: 'block.input', type_version: 1, key: 'block_input', label: '', params: { fields: [] }, settings: {}, position: { x: 0, y: 0 } },
            { id: B, type: 'block.output', type_version: 1, key: 'block_output', label: '', params: { fields: [], values: {} }, settings: {}, position: { x: 300, y: 0 } }
        ], edges: [{ id: 'e1', source: { node: A, port: 'out' }, target: { node: B, port: 'in' } }] } } });
        return { h, editor: openEditor(h, opts) };
    }
    await guardAsync('blocks contract review names empty and populated field lists explicitly', async () => {
        const { h, editor } = fixture();
        const empty = h.ED.blocks.contractMarkup(editor.ed, { inputs: [], outputs: [] });
        eq('blocks empty inputs and outputs each have a localized None hint', [(empty.match(/class="ed-hint"/g) || []).length, (empty.match(/option_none/g) || []).length, empty.includes('<ul')], [2, 2, false]);
        const mixed = h.ED.blocks.contractMarkup(editor.ed, { inputs: [], outputs: [field('message')] });
        eq('blocks populated contract fields retain their type and required state beside an empty input hint', [(mixed.match(/option_none/g) || []).length, mixed.includes('<code>message</code>'), mixed.includes('type_text'), mixed.includes('block_required')], [1, true, true, true]);
        editor.dispose();
    });
    await guardAsync('blocks typed values distinguish omitted null false and zero', async () => {
        const { h, editor } = fixture(); await settle();
        const fields = [field('count', 'number'), field('enabled', 'boolean'), field('optional', 'string', false, true), field('null_value', 'string', false, true)];
        const form = h.ED.blockFields.values({ ed: editor.ed, fields, value: { count: 0, enabled: false, null_value: null }, requireComplete: true });
        editor.el.appendChild(form.el);
        eq('blocks typed values preserve zero false and explicit null without adding optional keys', form.values(), { count: 0, enabled: false, null_value: null });
        eq('blocks complete typed sample flushes', form.flush(), true);
        eq('blocks sample limits remain available in collapsed preview notes', [form.el.querySelector('.ed-details').getAttribute('open'), !!form.el.querySelector('.ed-details .ed-hint')], [null, true]);
        const missing = h.ED.blockFields.values({ ed: editor.ed, fields, value: {}, requireComplete: true });
        eq('blocks simulation refuses omitted required values', missing.flush(), false);
        const draft = h.ED.blockFields.values({ ed: editor.ed, fields, value: {} });
        eq('blocks incomplete authoring remains repairable', draft.flush(), true);
        editor.el.appendChild(draft.el);
        const choose = (name, state) => { const select = draft.el.querySelector('.ed-field[data-param="' + name + '"] [data-ed-block-presence]'); select.focus(); select.value = state; select.fire('change'); };
        choose('count', 'value');
        eq('blocks fixed numeric zero is visible and retains its exact type', [draft.el.querySelector('.ed-field[data-param="count"] input').value, draft.values().count], ['0', 0]);
        choose('optional', 'null'); choose('optional', 'absent');
        eq('blocks explicit null can return to omission without disturbing zero', draft.values(), { count: 0 });
        const literal = h.ED.blockFields.values({ ed: editor.ed, fields: [field('text')], value: { text: '{{literal text}}' }, requireComplete: true }); editor.el.appendChild(literal.el);
        eq('blocks simulation text is a literal input without template chips', [literal.el.querySelector('input').value, !!literal.el.querySelector('.ed-tpl'), literal.values().text], ['{{literal text}}', false, '{{literal text}}']);
        literal.dispose();
        form.dispose(); missing.dispose(); draft.dispose(); editor.dispose();
    });
    await guardAsync('blocks required unbound output can be renamed and removed without a repair dead end', async () => {
        const { h, editor } = fixture(undefined, { block: true }); await settle();
        editor.ed.bus.emit('open-detail', { nodeId: B });
        editor.el.querySelector('[data-ed-block-field-add]').fire('click'); await settle();
        eq('blocks adds an explicit typed required output', editor.ed.model.node(B).params.fields, [field('field_1')]);
        const input = editor.el.querySelector('[data-ed-block-key="name"]'); input.value = 'greeting'; input.fire('input'); input.fire('change'); await settle();
        eq('blocks renames an incomplete required output', editor.ed.model.node(B).params.fields[0].name, 'greeting');
        const presence = editor.el.querySelector('.ed-field[data-param="values.greeting"] [data-ed-block-presence]'); presence.focus(); presence.value = 'value'; presence.fire('change');
        editor.el.querySelector('.ed-field[data-param="values.greeting"] .ed-tpl-view').fire('click');
        const area = editor.el.querySelector('.ed-field[data-param="values.greeting"] textarea'); area.fire('focusin'); area.value = 'Hello '; area.setSelectionRange(3, 3);
        editor.el.querySelector('.ed-canvas').focus(); editor.ed.flushPendingEdits();
        eq('blocks renaming a boundary field does not later rebuild its typed buffer or caret on blur', [editor.el.querySelector('.ed-field[data-param="values.greeting"] textarea') === area, area.selectionStart, editor.ed.model.node(B).params.values.greeting], [true, 3, 'Hello ']);
        editor.el.querySelector('[data-ed-block-field-remove]').fire('click'); await settle();
        eq('blocks removes an incomplete required output', editor.ed.model.node(B).params.fields, []);
        eq('blocks editor never loads independent run history or mission controls', h.requests.some(req => /\/runs|\/enabled|\/run$|\/repair-mission/.test(req.url)), false);
        eq('blocks editor hides activation and history', [editor.el.querySelector('[data-ed-cmd="active"]').hidden, editor.el.querySelector('[data-ed-cmd="runs"]').hidden], [true, true]);
        editor.dispose();
    });
    await guardAsync('blocks simulation opens once and a closed dialog cannot publish a late result', async () => {
        let resolvePreview, resolveSimulation, previews = 0, simulations = 0;
        const pendingPreview = new Promise(resolve => { resolvePreview = resolve; }), pendingSimulation = new Promise(resolve => { resolveSimulation = resolve; });
        const { h, editor } = fixture(req => {
            if (req.url.endsWith('/publish-preview')) { previews++; return pendingPreview; }
            if (req.url.endsWith('/simulate')) { simulations++; return pendingSimulation; }
        }, { block: true }); await settle();
        const test = editor.el.querySelector('[data-ed-cmd="test"]'); test.fire('click'); test.fire('click'); await settle();
        eq('blocks duplicate start shares one pending preview', previews, 1);
        resolvePreview({ draft_revision: 3, contract: { inputs: [], outputs: [] }, issues: [], diff: {} }); await settle();
        eq('blocks opens one simulation dialog', h.dialogs.length, 1);
        const dialog = h.dialogs[0]; dialog.el.querySelector('[data-ed-action="simulate"]').fire('click'); await settle();
        eq('blocks submits exact draft revision and explicit typed inputs', h.requests.find(req => req.url.endsWith('/simulate')).body, { base_revision: 3, inputs: {} });
        dialog.close(null); await settle();
        resolveSimulation({ mode: 'simulation', revision: 3, status: 'success', outputs: {}, steps: [] }); await settle();
        eq('blocks cancelled simulation never installs a late result or evidence', [simulations, editor.ed.run, editor.ed.testEvidence], [1, null, undefined]);
        editor.dispose();
    });
    await guardAsync('blocks two parents retain an immutable revision and upgrade only one explicitly as one undo', async () => {
        const shape = { inputs: [field('name')], outputs: [field('greeting')] };
        const newer = { inputs: [field('name'), field('suffix', 'string', false)], outputs: [field('greeting'), field('length', 'number')] };
        const answer = req => {
            if (req.url.startsWith('/api/desktop/flow-blocks?')) return { blocks: [{ id: 'block_a', name: 'Greeting', live_revision: 2 }] };
            if (req.url.includes('/versions?')) return { versions: [{ revision: 2 }, { revision: 1 }] };
            if (req.url.endsWith('/versions/1')) return { doc: { kind: 'block' }, contract: shape };
            if (req.url.endsWith('/versions/2')) return { doc: { kind: 'block' }, contract: newer };
            if (req.url.includes('/dependents?')) return { items: [{ flow_id: 'parent_b', revision: 1, kind: 'published', block_revision: 1 }] };
        };
        const first = fixture(answer), second = fixture(answer);
        for (const { editor } of [first, second]) editor.ed.model.addNode('component.call', { x: 900, y: 0 }, { flow_id: 'block_a', revision: 1, inputs: { name: '{{alpha.text}}' } });
        await settle();
        const callOf = editor => editor.ed.model.doc.nodes.find(node => node.type === 'component.call');
        first.editor.ed.bus.emit('open-detail', { nodeId: callOf(first.editor).id }); await settle();
        eq('blocks mapping uses direct verified output fields without a data wrapper', first.editor.ed.model.fieldsOf(callOf(first.editor)).map(item => item.name), ['greeting']);
        eq('blocks discovering a later publication never updates either parent', [callOf(first.editor).params.revision, callOf(second.editor).params.revision], [1, 1]);
        const select = first.editor.el.querySelector('[data-ed-call-version]'); select.value = '2'; select.fire('change');
        first.editor.el.querySelector('[data-ed-call-apply]').fire('click'); await settle();
        eq('blocks version selection requires explicit review before any model edit', callOf(first.editor).params.revision, 1);
        const dialog = first.h.dialogs[first.h.dialogs.length - 1];
        dialog.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('blocks explicit upgrade changes only one parent and retains matching bindings', [callOf(first.editor).params, callOf(second.editor).params.revision], [{ flow_id: 'block_a', revision: 2, inputs: { name: '{{alpha.text}}' } }, 1]);
        eq('blocks call retry stays zero after upgrade', callOf(first.editor).settings.retry.count, 0);
        first.editor.ed.model.undo(); await settle();
        eq('blocks one undo restores the exact original reference and mappings', callOf(first.editor).params, { flow_id: 'block_a', revision: 1, inputs: { name: '{{alpha.text}}' } });
        first.editor.dispose(); second.editor.dispose();
    });
    await guardAsync('blocks late call contracts and version lists cannot repaint a newer reference', async () => {
        let resolveA;
        const pendingA = new Promise(resolve => { resolveA = resolve; });
        const { h, editor } = fixture(req => {
            if (req.url.startsWith('/api/desktop/flow-blocks?')) return { blocks: [{ id: 'block_a', name: 'A' }, { id: 'block_b', name: 'B' }] };
            if (req.url.includes('/versions?')) return { versions: [{ revision: 1 }] };
            if (req.url.includes('/block_a/versions/1')) return pendingA;
            if (req.url.includes('/block_b/versions/1')) return { contract: { inputs: [field('fresh')], outputs: [field('result')] }, doc: {} };
        });
        const id = editor.ed.model.addNode('component.call', { x: 900, y: 0 }, { flow_id: 'block_a', revision: 1, inputs: {} });
        editor.ed.bus.emit('open-detail', { nodeId: id }); await settle();
        editor.ed.model.setParam(id, 'flow_id', 'block_b'); await settle();
        resolveA({ contract: { inputs: [field('stale')], outputs: [field('old_result')] }, doc: {} }); await settle();
        eq('blocks late contract does not restore old inputs or output schema', [!!editor.el.querySelector('.ed-field[data-param="inputs.fresh"]'), !!editor.el.querySelector('.ed-field[data-param="inputs.stale"]'), editor.ed.model.fieldsOf(editor.ed.model.node(id)).map(item => item.name)], [true, false, ['result']]);
        eq('blocks no script errors while changing call references', h.logged.filter(line => /failed/.test(line)).length, 0);
        editor.dispose();
    });
    await guardAsync('blocks A B A metadata requests ignore an older failure after a newer verified response', async () => {
        let rejectOld, resolveNew, count = 0;
        const old = new Promise((resolve, reject) => { rejectOld = reject; }), fresh = new Promise(resolve => { resolveNew = resolve; });
        const { editor } = fixture(req => {
            if (req.url.includes('/block_a/versions/1')) return ++count === 1 ? old : fresh;
            if (req.url.includes('/block_b/versions/1')) return { contract: { inputs: [], outputs: [field('temporary')] } };
        });
        const id = editor.ed.model.addNode('component.call', { x: 900, y: 0 }, { flow_id: 'block_a', revision: 1, inputs: {} });
        await settle(); editor.ed.model.setParam(id, 'flow_id', 'block_b'); await settle(); editor.ed.model.setParam(id, 'flow_id', 'block_a'); await settle();
        resolveNew({ contract: { inputs: [], outputs: [field('fresh')] } }); await settle();
        rejectOld(new Error('Old request failed')); await settle();
        eq('blocks old A failure cannot erase the current A contract', [count, editor.ed.model.fieldsOf(editor.ed.model.node(id)).map(field => field.name)], [2, ['fresh']]);
        editor.dispose();
    });
    await guardAsync('blocks cancelled preview can reopen immediately and an older rejection stays silent', async () => {
        let rejectOld, count = 0; const signals = [];
        const pending = new Promise((resolve, reject) => { rejectOld = reject; });
        const { h, editor } = fixture(undefined, { block: true }); await settle();
        h.blockApi.preview = (id, signal) => { signals.push(signal); return ++count === 1 ? pending : Promise.resolve({ draft_revision: 3, contract: { inputs: [], outputs: [] } }); };
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle();
        editor.el.querySelector('[data-ed-cmd="cancel"]').fire('click');
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle();
        rejectOld(new Error('Late cancelled preview error')); await settle();
        eq('blocks cancel aborts preview and allows a new dialog without late errors', [count, signals[0].aborted, h.dialogs.length, h.notes.length], [2, true, 1, 0]);
        editor.dispose();
    });
    await guardAsync('blocks sample JSON refuses oversized raw paste and aggregate data while retaining visible edits', async () => {
        const { h, editor } = fixture(); await settle();
        const form = h.ED.blockFields.values({ ed: editor.ed, fields: [field('first', 'object', false), field('second', 'object', false)], value: { first: { kept: true } }, requireComplete: true });
        editor.el.appendChild(form.el);
        let area = form.el.querySelector('.ed-field[data-param="first"] textarea');
        area.value = '{"text":"' + 'x'.repeat(1000000) + '"}'; area.fire('input');
        eq('blocks raw one megabyte sample is refused without replacing the prior value', [form.flush(), form.values(), area.value.length > 1000000], [false, { first: { kept: true } }, true]);
        area.value = JSON.stringify({ text: 'x'.repeat(40000) }); area.fire('input');
        const select = form.el.querySelector('.ed-field[data-param="second"] [data-ed-block-presence]'); select.focus(); select.value = 'value'; select.fire('change');
        area = form.el.querySelector('.ed-field[data-param="second"] textarea'); area.value = JSON.stringify({ text: 'y'.repeat(40000) }); area.fire('input');
        eq('blocks aggregate limit blocks submission while retaining the rejected raw field', [form.flush(), form.values().first.text.length, form.values().second, area.value.length > 40000, h.requests.some(req => req.url.endsWith('/simulate'))], [false, 40000, {}, true, false]);
        form.dispose(); editor.dispose();
    });
    await guardAsync('blocks boolean mappings stay templates on insertion reload and mode changes', async () => {
        const { h, editor } = fixture(); await settle();
        const fields = [field('enabled', 'boolean')];
        const form = h.ED.blockFields.values({ ed: editor.ed, fields, value: { enabled: false }, templatable: true }); editor.el.appendChild(form.el);
        eq('blocks fixed false is a false boolean switch', [form.values().enabled, form.el.querySelector('.ed-switch').getAttribute('aria-checked')], [false, 'false']);
        form.insert('enabled', 'trigger.data.enabled');
        eq('blocks mapping a boolean renders a template without coercion', [form.values().enabled, !!form.el.querySelector('.ed-tpl'), !!form.el.querySelector('.ed-switch')], ['{{trigger.data.enabled}}', true, false]);
        const reloaded = h.ED.blockFields.values({ ed: editor.ed, fields, value: form.values(), templatable: true }); editor.el.appendChild(reloaded.el);
        eq('blocks saved boolean template reloads as a template', [!!reloaded.el.querySelector('.ed-tpl'), !!reloaded.el.querySelector('.ed-switch'), reloaded.values().enabled], [true, false, '{{trigger.data.enabled}}']);
        reloaded.el.querySelector('.ed-tpl-view').fire('click'); const input = reloaded.el.querySelector('.ed-tpl-input'); input.fire('compositionstart');
        reloaded.el.querySelector('[data-ed-mode]').fire('click');
        eq('blocks changing boolean mode never cuts off an IME composition', reloaded.values().enabled, '{{trigger.data.enabled}}');
        input.fire('compositionend'); const mode = reloaded.el.querySelector('[data-ed-mode]'); mode.focus(); mode.fire('click');
        eq('blocks explicit fixed-mode change retains presence controls and removes the mapping', [Object.hasOwn(reloaded.values(), 'enabled'), !!reloaded.el.querySelector('[data-ed-block-presence]'), !!reloaded.el.querySelector('.ed-switch')], [false, true, true]);
        form.dispose(); reloaded.dispose(); editor.dispose();
    });
    await guardAsync('blocks incompatible legacy values remain visible and unknown bindings require explicit removal', async () => {
        const { h, editor } = fixture(); await settle();
        const value = { nullable_before: null, string_now: 7, object_now: 'old value', extra: { keep: true } };
        const form = h.ED.blockFields.values({ ed: editor.ed, fields: [field('nullable_before', 'number'), field('string_now'), field('object_now', 'object')], value, requireComplete: true }); editor.el.appendChild(form.el);
        const select = form.el.querySelector('.ed-field[data-param="nullable_before"] [data-ed-block-presence]');
        eq('blocks obsolete null is visible as invalid instead of an empty choice', [select.value, select.getAttribute('aria-invalid'), !!select.querySelector('option[value="null"]')], ['null', 'true', true]);
        eq('blocks type changes retain exact values and block simulation', [form.values(), form.flush()], [value, false]);
        form.el.querySelector('[data-ed-block-unknown-remove]').fire('click');
        eq('blocks unknown stored binding is removed only by explicit action', Object.hasOwn(form.values(), 'extra'), false);
        form.dispose(); editor.dispose();
        const readonly = fixture(undefined, { block: true, readonly: true }); await settle(); readonly.editor.ed.bus.emit('open-detail', { nodeId: A });
        eq('blocks read-only boundary inspection never blocks leaving or writes a draft', [await readonly.editor.leave(), readonly.h.puts().length], [true, 0]);
        readonly.editor.dispose();
    });
    await guardAsync('blocks imported retries survive timeout edits and explicit reset is one reversible correction', async () => {
        const { h, editor } = fixture(); await settle();
        const id = editor.ed.model.addNode('component.call', { x: 900, y: 0 }, {});
        editor.ed.model.setSettings(id, { retry: { count: 5, delay_seconds: 7 } });
        editor.ed.bus.emit('open-detail', { nodeId: id });
        editor.el.querySelector('[data-ed-pane="settings"]').fire('click');
        const timeout = editor.el.querySelector('[data-k="timeout"]'); timeout.focus(); timeout.value = '45'; timeout.fire('change');
        eq('blocks editing timeout retains the invalid imported retry count and disables retry controls', [editor.ed.model.node(id).settings.retry.count, editor.el.querySelector('[data-k="count"]').disabled], [5, true]);
        editor.el.querySelector('[data-ed-block-reset-retry]').fire('click');
        eq('blocks explicit retry reset changes only the count and updates its visible control', [editor.ed.model.node(id).settings.retry.count, editor.ed.model.node(id).settings.timeout_seconds, editor.el.querySelector('[data-k="count"]').value], [0, 45, '0']);
        editor.el.querySelector('.ed-canvas').focus(); editor.ed.model.undo();
        eq('blocks undo restores imported retry information without undoing the timeout change', [editor.ed.model.node(id).settings.retry.count, editor.ed.model.node(id).settings.timeout_seconds], [5, 45]);
        editor.dispose();
    });
    await guardAsync('blocks publish applies only the exact reviewed saved model', async () => {
        const { h, editor } = fixture(undefined, { block: true }); await settle();
        h.blockApi.preview = async () => ({ draft_revision: 3, contract: { inputs: [], outputs: [] }, issues: [], effects: [], diff: {} });
        editor.el.querySelector('[data-ed-cmd="publish"]').fire('click'); await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        editor.ed.model.setFlow({ name: 'Changed after review' });
        dialog.el.querySelector('[data-ed-action="publish"]').fire('click'); await settle();
        eq('blocks a local edit after review prevents publishing another revision', h.requests.filter(req => /\/publish$/.test(req.url)).length, 0);
        editor.dispose();
    });
    await guardAsync('blocks dependent metadata separates published parents from retained run pins', async () => {
        const { h, editor } = fixture(); await settle();
        const html = h.ED.blocks.dependentsMarkup(editor.ed, [{ kind: 'published', flow_id: 'parent_a', revision: 4, block_revision: 2 }, { kind: 'run', flow_id: 'parent_b', revision: 0, run_id: 'run_abc', block_revision: 1 }]);
        eq('blocks dependency labels are localized and do not call run pins published v0', [html.includes('parent_a · v4'), html.includes('run_abc'), html.includes('v0'), html.includes('>published ·'), html.includes('v2')], [true, true, false, false, true]);
        editor.dispose();
    });
    await guardAsync('blocks version selection loads older retained versions without changing the pinned reference', async () => {
        const { editor } = fixture(req => {
            if (req.url.startsWith('/api/desktop/flow-blocks?')) return { blocks: [{ id: 'block_a', name: 'A' }] };
            if (req.url.includes('/versions?')) return req.url.includes('before_revision=51') ? { versions: [{ revision: 50 }, { revision: 1 }] } : { versions: [{ revision: 52 }, { revision: 51 }], next_cursor: 51 };
            if (req.url.endsWith('/versions/1')) return { contract: { inputs: [], outputs: [] } };
        });
        const id = editor.ed.model.addNode('component.call', { x: 900, y: 0 }, { flow_id: 'block_a', revision: 1, inputs: {} });
        editor.ed.bus.emit('open-detail', { nodeId: id }); await settle();
        editor.el.querySelector('[data-ed-call-version-more]').fire('click'); await settle();
        const options = Array.from(editor.el.querySelectorAll('[data-ed-call-version] option')).map(option => option.value);
        eq('blocks older version page appends deduplicated choices and keeps the current pin', [options, editor.ed.model.node(id).params.revision, editor.el.querySelector('[data-ed-call-version-more]').hidden], [['', '52', '51', '50', '1'], 1, true]);
        editor.dispose();
    });
    await guardAsync('blocks malformed declarations remain visible and need explicit bounded repair', async () => {
        const malformed = [[null], [7], [{ name: 'old', type: 'unknown' }], [{ name: 'old', type: 'string', required: 'true' }], [{ name: 'old', type: 'string', extra: true }], Array.from({ length: 65 }, (_, i) => field('field_' + i)), { preserved: true }];
        for (const raw of malformed) {
            for (const readonly of [false, true]) {
                const { h, editor } = fixture(undefined, { block: true, readonly }); await settle();
                editor.ed.model.setParam(A, 'fields', raw);
                editor.ed.bus.emit('open-detail', { nodeId: A });
                eq('blocks malformed declaration uses repair rather than unbounded row rendering', [!!editor.el.querySelector('[data-ed-block-repair-editor]'), editor.el.querySelectorAll('.ed-block-field').length, editor.ed.model.node(A).params.fields], [true, 0, raw]);
                const area = editor.el.querySelector('[data-ed-block-repair-editor] textarea'), apply = editor.el.querySelector('[data-ed-block-repair-apply]');
                eq('blocks viewing malformed stored data does not write or block read-only leave', [editor.ed.flushPendingEdits(), editor.ed.model.node(A).params.fields, area.disabled], [true, raw, readonly]);
                if (!readonly) {
                    area.value = '[]'; area.fire('input');
                    eq('blocks valid repair remains unapplied until its explicit button', editor.ed.model.node(A).params.fields, raw);
                    apply.fire('click');
                    eq('blocks explicit repair replaces only the declaration', editor.ed.model.node(A).params.fields, []);
                    editor.ed.model.undo();
                    eq('blocks undo safely restores the exact malformed declaration', [editor.ed.model.node(A).params.fields, !!editor.el.querySelector('[data-ed-block-repair-editor]')], [raw, true]);
                }
                editor.dispose();
            }
        }
        const valid = fixture(undefined, { block: true }); await settle();
        valid.editor.ed.model.setParam(A, 'fields', [{ name: 'optional', type: 'string' }]);
        valid.editor.ed.bus.emit('open-detail', { nodeId: A });
        eq('blocks omitted bool flags keep the backend false-default meaning without normalization on open', [valid.editor.el.querySelectorAll('.ed-block-field').length, valid.editor.ed.model.node(A).params.fields], [1, [{ name: 'optional', type: 'string' }]]);
        valid.editor.dispose();
    });
    await guardAsync('blocks simulation results use localized boundary labels and the pure block explanation', async () => {
        const { editor } = fixture(undefined, { block: true }); await settle();
        editor.ed.run = { mode: 'simulation', status: 'success', outputs: {}, steps: new Map([A, B].map(id => [id, { node_id: id, status: 'success', output: {} }])) };
        editor.ed.bus.emit('run', editor.ed.run);
        const result = editor.el.querySelector('.ed-run-results-body').html;
        eq('blocks results do not claim external fixture/model execution or expose technical boundary types', [result.includes('Block input'), result.includes('Block output'), result.includes(editor.ed.t('easydrag.ui.block_simulation_hint')), result.includes('block.input'), result.includes('>' + editor.ed.t('easydrag.ui.simulation_hint') + '<')], [true, true, true, false, false]);
        editor.dispose();
    });
    await guardAsync('blocks version restore replaces a grouped draft through confirmation and the observed CAS', async () => {
        let revision = 3, historical;
        const { h, editor } = fixture(req => {
            if (req.method === 'PUT' && req.url === '/api/desktop/flow-blocks/f1') return { draft_revision: ++revision, issues: [] };
            if (req.method === 'GET' && req.url.startsWith('/api/desktop/flow-blocks/f1/versions?')) return { versions: [{ revision: 1, node_count: 2, edge_count: 1 }] };
            if (req.method === 'POST' && req.url === '/api/desktop/flow-blocks/f1/versions') return { block: { id: 'f1', draft_revision: ++revision, draft: Object.assign({}, historical, { groups: [] }) }, issues: [] };
        }, { block: true });
        historical = editor.ed.model.toJSON(); editor.ed.model.createGroup([A, B], 'Current group'); await editor.ed.saver.flush(); h.confirmAnswer = true;
        const dialog = await h.ED.blocks.versions(editor.ed, { openBlock() {} }); dialog.body.querySelector('[data-ed-block-restore="1"]').fire('click'); await settle();
        const requests = h.requests.filter(req => req.method === 'POST' && req.url.endsWith('/versions'));
        eq('blocks restoring a legacy version sends the current revision and clears groups without Mission effects', [requests.map(req => req.body), editor.ed.model.doc.groups, editor.ed.model.doc.nodes.length, editor.ed.saver.state, h.requests.some(req => /enabled|\/run$|repair-mission/.test(req.url))], [[{ revision: 1, base_revision: 4 }], [], 2, 'saved', false]);
        editor.dispose();
        const read = fixture(req => req.url.includes('/versions?') ? { versions: [{ revision: 1 }] } : undefined, { block: true, readonly: true });
        const view = await read.h.ED.blocks.versions(read.editor.ed, { openBlock() {} }); eq('blocks readonly version history retains inspection and omits restore writes', [!!view.body.querySelector('[data-ed-block-version]'), !!view.body.querySelector('[data-ed-block-restore]')], [true, false]); read.editor.dispose();
    });
}
