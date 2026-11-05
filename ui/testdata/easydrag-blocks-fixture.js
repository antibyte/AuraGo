// Browser-only block service fixture. Published documents are independent immutable copies;
// the exercised input/output/set path uses the production template evaluator.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), clone = value => JSON.parse(JSON.stringify(value));
    const state = { blocks: new Map(), versions: new Map(), requests: [], sequence: 0 };
    const base = '/api/desktop/flow-blocks', normal = '/api/desktop/flows';
    const response = (data, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const error = (status, code, issues) => response({ code, error: code, issues }, status);
    const now = () => new Date().toISOString(), tr = key => window.t(key);
    const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
    function marker(type, input) { return { type, version: 1, label: tr('easydrag.node.' + (input ? 'block_input' : 'block_output') + '.label'), description: tr('easydrag.node.' + (input ? 'block_input' : 'block_output') + '.description'), category: 'logic', icon: input ? 'upload' : 'download', trigger: input, inputs: input ? [] : ['in'], outputs: input ? ['out'] : [], params: [{ name: 'fields', kind: 'json' }], output_fields: input ? [{ name: 'data', type: 'object' }] : [], availability: { state: 'available' } }; }
    window.edFixture.registerNode({ type: 'component.call', version: 1, category: 'logic', icon: 'grid', label: 'Baustein aufrufen', inputs: ['in'], outputs: ['out'], params: [], output_fields: [], availability: { state: 'available' } });
    function contract(doc) {
        const input = doc.nodes.filter(node => node.type === 'block.input'), output = doc.nodes.filter(node => node.type === 'block.output');
        if (input.length !== 1 || output.length !== 1) throw new Error('contract');
        return window.EasyDrag.blocks.contract({ inputs: input[0].params.fields, outputs: output[0].params.fields });
    }
    function values(fields, value, templates) {
        if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
        if (Object.keys(value).some(name => !fields.some(field => field.name === name))) return false;
        return fields.every(field => {
            if (!own(value, field.name)) return !field.required;
            const item = value[field.name];
            if (item === null) return field.nullable;
            if (templates && typeof item === 'string' && item.includes('{{')) return true;
            return field.type === 'array' ? Array.isArray(item) : field.type === 'object' ? typeof item === 'object' && !Array.isArray(item) : typeof item === field.type;
        });
    }
    function issues(doc, mode) {
        const found = [];
        if (!doc || doc.kind !== 'block' || !Array.isArray(doc.nodes) || doc.nodes.some(node => !['block.input', 'block.output', 'logic.set', 'logic.if', 'logic.switch', 'logic.merge'].includes(node.type))) return [{ code: 'NODE_TYPE_UNKNOWN', severity: 'error' }];
        try { const shape = contract(doc), out = doc.nodes.find(node => node.type === 'block.output'); if (!values(shape.outputs, out.params.values || {}, true)) throw new Error('bindings'); }
        catch (_) { found.push({ code: 'PARAM_INVALID', severity: mode === 'publish' ? 'error' : 'warning', message: 'Check the input and output contract.' }); }
        return found;
    }
    function summary(rec) { return { id: rec.id, name: rec.name, description: rec.description, draft_revision: rec.draft_revision, live_revision: rec.live_revision, published: !!rec.live, updated_at: rec.updated_at }; }
    function deps(id, revision) {
        const items = [];
        for (const entry of window.edFixture.state.flows.values()) {
            const rec = entry.rec;
            for (const node of rec.live && rec.live.nodes || []) if (node.type === 'component.call' && node.params.flow_id === id && (!revision || node.params.revision === revision)) items.push({ kind: 'published', flow_id: rec.id, revision: rec.live_revision, block_revision: node.params.revision });
        }
        return items.sort((a, b) => a.flow_id.localeCompare(b.flow_id));
    }
    function evaluate(doc, inputs, at) {
        const started = at || now(), steps = [], roots = { trigger: { data: inputs }, run: { mode: 'simulation', started_at: started } };
        let shape, outputs = null;
        try {
            shape = contract(doc); if (!values(shape.inputs, inputs, false)) throw new Error('input');
            const pending = doc.nodes.slice(), done = new Set();
            while (pending.length) {
                const index = pending.findIndex(node => doc.edges.filter(edge => edge.target.node === node.id).every(edge => done.has(edge.source.node)));
                if (index < 0) throw new Error('cycle');
                const node = pending.splice(index, 1)[0]; let output;
                if (node.type === 'block.input') output = { data: inputs };
                else if (node.type === 'logic.set') output = Object.fromEntries((node.params.fields || []).map(field => [field.name, window.EasyDrag.template.evaluate(field.value, roots)]));
                else if (node.type === 'block.output') { output = window.EasyDrag.template.evaluate(node.params.values || {}, roots); if (!values(shape.outputs, output, false)) throw new Error('output'); outputs = output; }
                else throw new Error('Browser fixture path is not implemented');
                roots[node.key] = output; done.add(node.id); steps.push({ node_id: node.id, node_key: node.key, status: 'success', output, ports: node.type === 'block.output' ? [] : ['out'], started_at: started, finished_at: started, duration_ms: 0 });
            }
            return { mode: 'simulation', started_at: started, status: 'success', outputs, steps };
        } catch (_) { return { mode: 'simulation', started_at: started, status: 'error', error_code: 'FLOW_PARAM_INVALID', outputs: null, steps }; }
    }
    function invokeVersion(id, revision, inputs, at) {
        const doc = state.versions.get(id + ':' + revision);
        return doc ? evaluate(doc.doc, inputs, at) : { status: 'error', error_code: 'FLOW_BLOCK_UNAVAILABLE', outputs: null, steps: [] };
    }
    function route(method, url, body) {
        const rest = url.pathname.slice(base.length).split('/').filter(Boolean);
        if (!rest.length) {
            if (method === 'GET') { const rows = Array.from(state.blocks.values()).sort((a, b) => a.id.localeCompare(b.id)), offset = Number((url.searchParams.get('cursor') || 'cursor_0').split('_')[1]) || 0, limit = Math.min(100, Number(url.searchParams.get('limit')) || 50); return response({ blocks: rows.slice(offset, offset + limit).map(summary), next_cursor: offset + limit < rows.length ? 'cursor_' + (offset + limit) : undefined }); }
            if (method === 'POST') {
                const id = 'flow_block' + String(++state.sequence).padStart(5, '0');
                const doc = { schema: 1, kind: 'block', id, name: body.name, settings: {}, nodes: [
                    { id: 'n_blockinput01', type: 'block.input', type_version: 1, key: 'block_input', label: '', params: { fields: [] }, settings: {}, position: { x: 0, y: 0 } },
                    { id: 'n_blockoutput1', type: 'block.output', type_version: 1, key: 'block_output', label: '', params: { fields: [], values: {} }, settings: {}, position: { x: 360, y: 0 } }
                ], edges: [{ id: 'e_blocklink01', source: { node: 'n_blockinput01', port: 'out' }, target: { node: 'n_blockoutput1', port: 'in' } }] };
                const rec = { id, kind: 'block', name: body.name, draft: doc, draft_revision: 1, live_revision: 0, published_draft_revision: 0, published_at: '0001-01-01T00:00:00Z', created_at: now(), updated_at: now() }; state.blocks.set(id, rec); return response({ block: clone(rec) });
            }
        }
        if (rest[0] === 'node-types' && method === 'GET') return response({ node_types: [marker('block.input', true), marker('block.output', false)].concat(window.edFixture.catalog.node_types.filter(info => ['logic.set', 'logic.if'].includes(info.type))), categories: [{ id: 'logic', label: 'Logik' }] });
        if (rest[0] === 'validate' && method === 'POST') { const list = issues(body.doc, body.mode); return response({ valid: !list.some(item => item.severity === 'error'), issues: list }); }
        const rec = state.blocks.get(rest[0]); if (!rec) return error(404, 'FLOW_NOT_FOUND');
        if (rest.length === 1) {
            if (method === 'GET') { let shape; try { shape = contract(rec.draft); } catch (_) {} return response({ block: clone(rec), issues: issues(rec.draft, 'draft'), contract: shape }); }
            if (method === 'PUT') { if (body.base_revision !== rec.draft_revision) return error(409, 'FLOW_REVISION_CONFLICT'); if (!body.doc || body.doc.kind !== 'block') return error(400, 'FLOW_BAD_REQUEST'); rec.draft = clone(body.doc); rec.name = rec.draft.name; rec.draft_revision++; rec.updated_at = now(); return response({ draft_revision: rec.draft_revision, issues: issues(rec.draft, 'draft') }); }
            if (method === 'DELETE') { if (body.base_revision !== rec.draft_revision || body.base_live_revision !== rec.live_revision) return error(409, 'FLOW_REVISION_CONFLICT'); if (deps(rec.id, 0).length) return error(409, 'FLOW_BLOCK_IN_USE'); state.blocks.delete(rec.id); return response({ status: 'deleted' }); }
        }
        if (rest[1] === 'publish-preview' && method === 'GET') { const list = issues(rec.draft, 'publish'); let shape; try { shape = contract(rec.draft); } catch (_) {} const before = rec.live && rec.live.nodes.find(node => node.type === 'block.output'), after = rec.draft.nodes.find(node => node.type === 'block.output'); return response({ draft_revision: rec.draft_revision, issues: list, contract: shape, effects: [], diff: { first_publish: !rec.live, changed_nodes: rec.live ? 1 : 0, details: before && after ? [{ kind: 'node_changed', node_id: after.id, label: tr('easydrag.node.block_output.label'), field: 'values', before: before.params.values, after: after.params.values }] : [] } }); }
        if (rest[1] === 'publish' && method === 'POST') { if (body.base_revision !== rec.draft_revision) return error(409, 'FLOW_REVISION_CONFLICT'); const list = issues(rec.draft, 'publish'); if (list.some(item => item.severity === 'error')) return error(422, 'FLOW_INVALID', list); if (rec.published_draft_revision !== rec.draft_revision) { rec.live = clone(rec.draft); rec.live_revision++; rec.published_draft_revision = rec.draft_revision; rec.published_at = now(); state.versions.set(rec.id + ':' + rec.live_revision, { doc: clone(rec.live), contract: contract(rec.live), published_at: rec.published_at }); } return response({ block: clone(rec), issues: [] }); }
        if (rest[1] === 'versions' && method === 'GET') { if (rest[2]) { const item = state.versions.get(rec.id + ':' + rest[2]); return item ? response(clone(item)) : error(409, 'FLOW_BLOCK_UNAVAILABLE'); } const versions = Array.from(state.versions).filter(([key]) => key.startsWith(rec.id + ':')).map(([key, item]) => ({ revision: Number(key.split(':')[1]), published_at: item.published_at, node_count: item.doc.nodes.length, edge_count: item.doc.edges.length })).sort((a, b) => b.revision - a.revision); return response({ versions }); }
        if (rest[1] === 'dependents' && method === 'GET') return response({ items: deps(rec.id, Number(url.searchParams.get('revision')) || 0) });
        if (rest[1] === 'simulate' && method === 'POST') { if (body.base_revision !== rec.draft_revision) return error(409, 'FLOW_REVISION_CONFLICT'); return response(Object.assign({ revision: rec.draft_revision }, evaluate(rec.draft, body.inputs, body.at))); }
        const message = 'Unrouted block fixture: ' + method + ' ' + url.pathname; window.fixtureErrors.push(message); return error(404, 'FLOW_NOT_FOUND');
    }
    window.fetch = async (value, options) => {
        const url = new URL(String(value), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        if (url.pathname === base || url.pathname.startsWith(base + '/')) { state.requests.push(method + ' ' + url.pathname + url.search); return route(method, url, opts.body ? JSON.parse(opts.body) : {}); }
        if (url.pathname === normal + '/node-types') { const res = await previous(value, opts), data = await res.json(); data.node_types = data.node_types.map(info => info.type === 'component.call' ? Object.assign({}, info, { label: tr('easydrag.node.component_call.label'), description: tr('easydrag.node.component_call.description') }) : info); return response(data, res.status); }
        return previous(value, opts);
    };
    window.edBlockFixture = { state, invokeVersion };
})();
