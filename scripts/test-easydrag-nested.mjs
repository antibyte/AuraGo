// Nested calls reuse the block editor's typed controls and immutable-reference commands.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const I = 'n_iiiiiiii', C = 'n_cccccccc', O = 'n_oooooooo';
    const field = (name, type = 'string', required = true, nullable = false) => ({ name, type, required, nullable });
    const shape = revision => ({ inputs: [field('name')], outputs: [field('greeting'), ...(revision === 2 ? [field('length', 'number')] : [])] });
    const boundary = { inputs: [field('name'), field('flag', 'boolean'), field('amount', 'number'), field('optional', 'string', false, true)], outputs: [field('greeting')] };
    function fixture(options = {}) {
        const owner = options.owner || 'middle', source = options.self ? owner : 'leaf';
        let revision = 3;
        const h = sandbox(req => {
            const custom = options.answer && options.answer(req); if (custom !== undefined) return custom;
            if (req.url === '/api/desktop/flow-blocks/validate') return { valid: true, issues: [] };
            if (req.method === 'PUT' && req.url === '/api/desktop/flow-blocks/' + owner) return { draft_revision: ++revision, issues: [] };
            if (req.url.startsWith('/api/desktop/flow-blocks?')) return { blocks: [{ id: owner, name: 'Current block', live_revision: 1 }, { id: 'leaf', name: 'Leaf', live_revision: 2 }], next_cursor: options.page ? 'older' : undefined };
            if (req.url.includes('/versions?')) return { versions: [{ revision: 2 }, { revision: 1 }] };
            if (/\/versions\/[12]$/.test(req.url)) return { doc: { kind: 'block' }, contract: shape(Number(req.url.at(-1))) };
            if (req.url.includes('/dependents?')) return { items: [] };
            if (req.url.endsWith('/publish-preview')) return { draft_revision: revision, contract: boundary, issues: [], effects: [], diff: {} };
            if (req.url.endsWith('/simulate')) return { mode: 'simulation', revision, started_at: '2026-11-05T12:00:00Z', status: 'success', outputs: { greeting: 'Ada hello' }, steps: [{ node_id: I, status: 'success', output: { data: req.body.inputs } }, { node_id: C, status: 'success', output: { greeting: 'Ada hello' } }, { node_id: O, status: 'success', output: { greeting: 'Ada hello' } }] };
        });
        const types = new Map(h.catalog.types);
        for (const [type, inputs, outputs] of [['block.input', [], ['out']], ['block.output', ['in'], []], ['component.call', ['in'], ['out']]]) types.set(type, { type, label: type, type_version: 1, category: 'logic', inputs, outputs, params: [], output_fields: [], availability: { state: 'available' } });
        h.catalog = { types, list: Array.from(types.values()), categories: [], blockContracts: new Map() }; h.blockApi = h.ED.blocks.api(h.transport);
        const doc = { schema: 1, kind: 'block', name: owner, nodes: [
            { id: I, key: 'block_input', type: 'block.input', position: { x: 0, y: 0 }, params: { fields: boundary.inputs }, settings: {} },
            { id: C, key: 'child', type: 'component.call', position: { x: 300, y: 0 }, params: { flow_id: source, revision: 1, inputs: { name: '{{block_input.data.name}}' } }, settings: { retry: { count: 0 } } },
            { id: O, key: 'block_output', type: 'block.output', position: { x: 600, y: 0 }, params: { fields: boundary.outputs, values: { greeting: '{{child.greeting}}' } }, settings: {} }
        ], edges: [{ id: 'e_inputcall', source: { node: I, port: 'out' }, target: { node: C, port: 'in' } }, { id: 'e_calloutput', source: { node: C, port: 'out' }, target: { node: O, port: 'in' } }], groups: [{ id: 'g_abcdefgh', title: 'Preserve group', color: 'blue', members: [C, O] }] };
        const editor = openEditor(h, { block: true, readonly: !!options.readonly, flow: { id: owner, draft: doc, draft_revision: revision } });
        return { h, editor, model: editor.ed.model, open() { editor.ed.bus.emit('open-detail', { nodeId: C }); } };
    }
    await guardAsync('nested unopened block calls expose only verified direct output fields', async () => {
        const f = fixture(); await settle();
        eq('nested block editor tracks unopened exact references', f.h.requests.filter(req => req.url.endsWith('/leaf/versions/1')).length, 1);
        eq('nested output mapping uses direct contract fields without a data wrapper', f.model.fieldsOf(f.model.node(C)).map(item => item.name), ['greeting']);
        const reads = f.h.requests.length;
        f.editor.ed.run = { id: 'sim', mode: 'simulation', steps: new Map(), changes: [] };
        for (let i = 0; i < 20; i++) f.editor.ed.bus.emit('run', f.editor.ed.run);
        eq('nested run ticks never fetch contract metadata or create run history', [f.h.requests.length, f.h.requests.some(req => /\/runs|\/enabled|\/run$/.test(req.url))], [reads, false]);
        f.editor.dispose();
    });
    await guardAsync('nested picker excludes new self choices while preserving invalid stored references', async () => {
        const f = fixture(); f.open(); await settle();
        const select = f.editor.el.querySelector('[data-ed-call-block]');
        eq('nested normal choices omit the current block ID', select.querySelectorAll('option').map(option => option.value), ['', 'leaf']);
        eq('nested depth and simulation scope guidance uses the existing details pattern', !!f.editor.el.querySelector('[data-ed-nested-call-help] .ed-hint'), true);
        f.editor.dispose();
        const bad = fixture({ self: true }); bad.open(); await settle();
        const own = bad.editor.el.querySelector('[data-ed-call-block]'), option = own.querySelector('option[value="middle"]'), apply = bad.editor.el.querySelector('[data-ed-call-apply]');
        eq('nested imported self reference stays selected visible and unavailable for new choices', [own.value, option.hasAttribute('disabled'), own.getAttribute('aria-invalid'), bad.editor.el.querySelector('[data-ed-call-self]').hidden, apply.disabled], ['middle', true, 'true', false, true]);
        const before = bad.model.toJSON(); apply.fire('click'); await settle();
        eq('nested scripted Apply cannot bypass the self-reference fence', [bad.model.toJSON(), bad.h.dialogs.length], [before, 0]);
        own.value = 'leaf'; own.fire('change'); await settle(); const version = bad.editor.el.querySelector('[data-ed-call-version]'); version.value = '1'; version.fire('change'); apply.fire('click'); await settle();
        bad.h.dialogs.at(-1).el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('nested repairing a self reference is explicit and retains named input mapping', [bad.model.node(C).params.flow_id, bad.model.node(C).params.inputs, bad.editor.el.querySelector('[data-ed-call-self]').hidden], ['leaf', { name: '{{block_input.data.name}}' }, true]);
        bad.model.undo(); await settle(); eq('nested one Undo restores invalid author data visibly without rewriting groups', [bad.model.toJSON(), bad.editor.el.querySelector('[data-ed-call-self]').hidden], [before, false]); bad.editor.dispose();
    });
    await guardAsync('nested version review upgrades one owner and preserves immutable callers and groups', async () => {
        const first = fixture(), other = fixture({ owner: 'other' }); await settle(); first.open(); await settle();
        const before = first.model.toJSON(), select = first.editor.el.querySelector('[data-ed-call-version]'); select.value = '2'; select.fire('change');
        eq('nested discovering a newer version never updates either owner', [first.model.node(C).params.revision, other.model.node(C).params.revision], [1, 1]);
        first.editor.el.querySelector('[data-ed-call-apply]').fire('click'); await settle();
        const review = first.h.dialogs.at(-1); eq('nested upgrade review explains indirect dependency owners', !!review.body.querySelector('[data-ed-block-dependencies-hint]'), true);
        review.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('nested explicit upgrade changes only one reference and retains the named binding', [first.model.node(C).params, other.model.node(C).params.revision, first.model.doc.groups], [{ flow_id: 'leaf', revision: 2, inputs: { name: '{{block_input.data.name}}' } }, 1, before.groups]);
        first.model.undo(); await settle(); eq('nested one Undo restores the exact version mapping and visual metadata', first.model.toJSON(), before); first.editor.dispose(); other.editor.dispose();
    });
    await guardAsync('nested simulation submits typed parent inputs and displays only parent block steps', async () => {
        const f = fixture(); await settle(); f.editor.el.querySelector('[data-ed-cmd="test"]').fire('click'); await settle(); const dialog = f.h.dialogs.at(-1);
        const presence = name => dialog.el.querySelector('.ed-field[data-param="' + name + '"] [data-ed-block-presence]');
        for (const name of ['name', 'flag', 'amount']) { presence(name).value = 'value'; presence(name).fire('change'); }
        const input = dialog.el.querySelector('.ed-field[data-param="name"] input'); input.value = 'Ada'; input.fire('change');
        dialog.el.querySelector('[data-ed-action="simulate"]').fire('click'); await settle();
        const req = f.h.requests.find(req => req.url.endsWith('/simulate'));
        eq('nested simulation keeps absent optional values false and zero without fake child fixtures', req.body, { base_revision: 3, inputs: { name: 'Ada', flag: false, amount: 0 } });
        eq('nested results retain parent scope and exact simulation evidence', [Array.from(f.editor.ed.run.steps.keys()), f.editor.ed.run.outputs, f.editor.ed.testEvidence, f.h.requests.some(req => /\/runs/.test(req.url))], [[I, C, O], { greeting: 'Ada hello' }, { mode: 'simulation', revision: 3, status: 'success' }, false]); f.editor.dispose();
    });
    await guardAsync('nested generic validation focuses the call and readonly inspection keeps repair data', async () => {
        const f = fixture(); f.editor.ed.issues = [{ severity: 'error', code: 'PARAM_INVALID', node_id: C, param: 'flow_id', message: 'untrusted backend text' }]; f.editor.ed.bus.emit('issues', f.editor.ed.issues);
        f.editor.el.querySelector('[data-ed-cmd="issues"]').fire('click'); f.editor.el.querySelector('[data-ed-issue="0"]').fire('click'); await settle(); f.h.flushFrames();
        eq('nested validation uses existing issue navigation and offers local repair guidance without parsing messages', [f.h.dom.document.activeElement === f.editor.el.querySelector('[data-ed-call-block]'), !!f.editor.el.querySelector('[data-ed-nested-call-help]')], [true, true]); f.editor.dispose();
        const read = fixture({ self: true, readonly: true }); read.open(); await settle(); const before = read.model.toJSON(); read.editor.el.querySelector('[data-ed-call-apply]').fire('click'); await settle();
        eq('nested readonly invalid references remain inspectable without writes or success notices', [read.editor.el.querySelector('[data-ed-call-block]').disabled, read.editor.el.querySelector('[data-ed-call-self]').hidden, read.model.toJSON(), read.h.puts().length], [true, false, before, 0]); read.editor.dispose();
    });
    await guardAsync('nested metadata disposal cannot install a late contract in a departed block', async () => {
        let resolve;
        const pending = new Promise(done => { resolve = done; });
        const f = fixture({ answer: req => req.url.endsWith('/leaf/versions/1') ? pending : undefined });
        f.editor.dispose(); resolve({ contract: shape(1), doc: { kind: 'block' } }); await settle();
        eq('nested late contract reads cannot revive a disposed editor schema', f.h.catalog.blockContracts.size, 0);
    });
    await guardAsync('nested paginated choices keep self excluded and ordinary failures remain visible', async () => {
        const f = fixture({ page: true, answer: req => req.url.includes('cursor=older') ? { blocks: [{ id: 'middle', name: 'Current block', live_revision: 1 }, { id: 'older', name: 'Older leaf', live_revision: 1 }] } : undefined });
        f.open(); await settle(); f.editor.el.querySelector('[data-ed-call-more]').fire('click'); await settle();
        eq('nested every picker page filters the owner without discarding the current reference', [f.editor.el.querySelector('[data-ed-call-block]').querySelectorAll('option').map(option => option.value), f.model.node(C).params.flow_id], [['', 'leaf', 'older'], 'leaf']); f.editor.dispose();
        const failed = fixture({ answer: req => req.url.endsWith('/leaf/versions/1') ? Promise.reject(Object.assign(new Error('unavailable'), { body: { code: 'FLOW_BLOCK_UNAVAILABLE' } })) : undefined });
        failed.open(); await settle(); const warnings = failed.editor.el.querySelectorAll('.ed-block-call .ed-error'), error = warnings.find(box => !box.hasAttribute('data-ed-call-self'));
        eq('nested ordinary contract errors stay separate from the self-reference warning', [error.hidden, error.textContent, failed.editor.el.querySelector('[data-ed-call-self]').hidden], [false, 'error_flow_block_unavailable', true]); failed.editor.dispose();
    });
}
