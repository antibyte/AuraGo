// Browser-only extraction service fixture. Real transaction and semantic checks are covered by Go tests.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), copy = value => JSON.parse(JSON.stringify(value));
    const prefix = '/api/desktop/flows/', state = { requests: [], loseNextApply: false, sequence: 0 };
    const response = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code, reason) => response(Object.assign({ code, error: code }, reason ? { reason } : {}), status);
    const hex = bytes => Array.from(new Uint8Array(bytes), x => x.toString(16).padStart(2, '0')).join('');
    const hash = async value => hex(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(JSON.stringify(value))));
    function suffix(value, count) { const a = 'abcdefghijklmnopqrstuvwxyz234567'; let text = ''; do { text = a[value % 32] + text; value = Math.floor(value / 32); } while (value); return text.padStart(count, 'a'); }
    function ids() {
        const n = ++state.sequence * 8;
        return { block_id: 'flow_' + suffix(n, 26), call_id: 'n_' + suffix(n + 1, 8), input_id: 'n_' + suffix(n + 2, 8), output_id: 'n_' + suffix(n + 3, 8), entry_edge_id: 'e_' + suffix(n + 4, 8), exit_edge_id: 'e_' + suffix(n + 5, 8) };
    }
    async function propose(rec, req, assigned) {
        if (!rec || rec.draft_revision !== req.base_revision) throw 'revision';
        if (!Array.isArray(req.node_ids) || req.node_ids.length !== 1 || typeof req.name !== 'string' || !req.name.trim() || req.name.length > 80) throw 'selection';
        const parent = copy(rec.draft), selected = parent.nodes.find(node => node.id === req.node_ids[0]);
        if (!selected || selected.type !== 'logic.set' || selected.settings?.disabled || selected.params.keep_input !== false) throw 'node';
        const incoming = parent.edges.filter(edge => edge.target.node === selected.id), outgoing = parent.edges.filter(edge => edge.source.node === selected.id);
        if (incoming.length !== 1 || outgoing.length > 1) throw 'boundary';
        const fields = selected.params.fields || [], types = { text: 'string', number: 'number', bool: 'boolean', object: 'object', list: 'array' };
        if (!fields.length || fields.some(field => !types[field.type] || typeof field.value === 'string' && field.value.includes('{{'))) throw 'fields';
        const outputs = fields.map(field => ({ name: field.name, type: types[field.type], required: true, nullable: false }));
        const values = Object.fromEntries(fields.map(field => [field.name, '{{' + selected.key + '.' + field.name + '}}']));
        const block = { schema: 1, kind: 'block', id: assigned.block_id, name: req.name, settings: {}, nodes: [
            { id: assigned.input_id, key: 'block_input', type: 'block.input', type_version: 1, params: { fields: [] }, settings: {}, position: { x: 0, y: 0 } },
            copy(selected),
            { id: assigned.output_id, key: 'block_output', type: 'block.output', type_version: 1, params: { fields: outputs, values }, settings: {}, position: { x: 600, y: 0 } }
        ], edges: [
            { id: assigned.entry_edge_id, source: { node: assigned.input_id, port: 'out' }, target: { node: selected.id, port: 'in' } },
            { id: assigned.exit_edge_id, source: { node: selected.id, port: 'out' }, target: { node: assigned.output_id, port: 'in' } }
        ] };
        parent.nodes = parent.nodes.filter(node => node.id !== selected.id);
        parent.nodes.push({ id: assigned.call_id, key: selected.key, type: 'component.call', type_version: 1, params: { flow_id: assigned.block_id, revision: 1, inputs: {} }, settings: { retry: { count: 0 } }, position: selected.position });
        parent.edges.forEach(edge => { if (edge.source.node === selected.id) edge.source.node = assigned.call_id; if (edge.target.node === selected.id) edge.target.node = assigned.call_id; });
        let moved = 0, shrunk = 0;
        if (Array.isArray(parent.groups)) {
            const kept = [], transferred = [];
            for (const group of parent.groups) {
                if (!group.members.includes(selected.id)) kept.push(group);
                else if (group.members.length === 1) { transferred.push(group); moved++; }
                else { group.members = group.members.filter(id => id !== selected.id); kept.push(group); shrunk++; }
            }
            parent.groups = kept; if (transferred.length) block.groups = transferred;
        }
        const parentHash = await hash(parent), blockHash = await hash(block);
        const digest = await hash({ version: 1, flow_id: rec.id, base_revision: req.base_revision, node_ids: req.node_ids, name: req.name, ids: assigned, parent_hash: parentHash, block_hash: blockHash });
        return { parent, block, preview: { base_revision: req.base_revision, ids: assigned, digest, parent_hash: parentHash, block_hash: blockHash, block_revision: 1, review: { inputs: [], outputs, groups_moved: moved, groups_shrunk: shrunk, extra_nodes: 3, extra_edges: 2, budget_note: true } } };
    }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        const match = url.pathname.match(/^\/api\/desktop\/flows\/([^/]+)\/extract\/(preview|apply|reconcile)$/);
        if (!match || method !== 'POST') return previous(input, options);
        let req; try { req = JSON.parse(opts.body || '{}'); } catch (_) { return fail(400, 'FLOW_EXTRACTION_INVALID'); }
        state.requests.push({ action: match[2], flow_id: match[1], body: copy(req) });
        const entry = window.edFixture.state.flows.get(match[1]), rec = entry && entry.rec;
        if (!rec) return fail(404, 'FLOW_NOT_FOUND');
        if (match[2] === 'reconcile') {
            const block = window.edBlockFixture.state.blocks.get(req.block_id), version = window.edBlockFixture.state.versions.get(req.block_id + ':1');
            const confirmed = rec.draft_revision === req.base_revision + 1 && version && await hash(rec.draft) === req.parent_hash && await hash(version.doc) === req.block_hash;
            return response(confirmed ? { status: 'applied_state_confirmed', flow: copy(rec), block: copy(block) } : { status: 'unknown_or_conflict' });
        }
        try {
            const candidate = await propose(rec, req, match[2] === 'preview' ? ids() : req.ids);
            if (match[2] === 'preview') return response(candidate.preview);
            if (candidate.preview.digest !== req.digest || window.edBlockFixture.state.blocks.has(req.ids.block_id)) return fail(409, 'FLOW_EXTRACTION_CONFLICT');
            const now = new Date().toISOString(), block = { id: req.ids.block_id, kind: 'block', name: candidate.block.name, mission_id: '', draft: copy(candidate.block), live: copy(candidate.block), draft_revision: 1, live_revision: 1, published_draft_revision: 1, published_at: now, created_at: now, updated_at: now };
            rec.draft = copy(candidate.parent); rec.draft_revision++; rec.updated_at = now;
            window.edBlockFixture.state.blocks.set(block.id, block);
            window.edBlockFixture.state.versions.set(block.id + ':1', { doc: copy(block.live), contract: { inputs: [], outputs: candidate.preview.review.outputs }, published_at: now });
            if (state.loseNextApply) { state.loseNextApply = false; throw new TypeError('simulated lost response'); }
            return response({ flow: copy(rec), block: copy(block) });
        } catch (error) {
            if (error instanceof Error) throw error;
            return error === 'revision' ? fail(409, 'FLOW_REVISION_CONFLICT') : fail(422, 'FLOW_EXTRACTION_UNSUPPORTED', String(error));
        }
    };
    window.edExtractionFixture = { state };
})();
