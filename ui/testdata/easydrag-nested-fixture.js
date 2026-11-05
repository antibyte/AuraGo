// Browser-only nested pure-block fixture; production execution is covered by server tests.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), state = window.edBlockFixture.state;
    const base = '/api/desktop/flow-blocks', copy = value => JSON.parse(JSON.stringify(value));
    const response = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code) => response({ code, error: code }, status);
    const contract = doc => {
        const input = doc.nodes.find(node => node.type === 'block.input');
        const output = doc.nodes.find(node => node.type === 'block.output');
        if (!input || !output) throw Error('missing boundary');
        return window.EasyDrag.blocks.contract({ inputs: input.params.fields, outputs: output.params.fields });
    };
    function validate(doc, ancestors) {
        if (!doc || doc.kind !== 'block' || !Array.isArray(doc.nodes)) throw Error('block');
        const path = ancestors || new Set();
        if (path.has(doc.id) || path.size > 8) throw Error('cycle');
        const next = new Set(path); next.add(doc.id);
        contract(doc);
        for (const node of doc.nodes) {
            if (!['block.input', 'block.output', 'logic.set', 'logic.if', 'logic.switch', 'logic.merge', 'component.call'].includes(node.type)) throw Error('unsafe node');
            if (node.type === 'component.call') {
                const ref = state.versions.get(node.params.flow_id + ':' + node.params.revision);
                if (!ref || node.params.flow_id === doc.id) throw Error('missing child');
                validate(ref.doc, next);
            }
        }
    }
    function evaluate(doc, inputs, at, path) {
        const roots = { trigger: { data: inputs }, run: { mode: 'simulation', started_at: at } }, steps = [], done = new Set(), pending = doc.nodes.slice();
        try {
            validate(doc, path);
            while (pending.length) {
                const index = pending.findIndex(node => doc.edges.filter(edge => edge.target.node === node.id).every(edge => done.has(edge.source.node)));
                if (index < 0) throw Error('cycle');
                const node = pending.splice(index, 1)[0]; let output;
                if (node.type === 'block.input') output = { data: inputs };
                else if (node.type === 'block.output') output = window.EasyDrag.template.evaluate(node.params.values || {}, roots);
                else if (node.type === 'logic.set') output = Object.fromEntries((node.params.fields || []).map(field => [field.name, window.EasyDrag.template.evaluate(field.value, roots)]));
                else if (node.type === 'component.call') {
                    const params = window.EasyDrag.template.evaluate(node.params, roots), ref = state.versions.get(params.flow_id + ':' + params.revision);
                    const next = new Set(path || []); next.add(doc.id);
                    const child = evaluate(ref.doc, params.inputs, at, next); if (child.status !== 'success') throw Error('child'); output = child.outputs;
                } else throw Error('unimplemented pure fixture node');
                roots[node.key] = output; done.add(node.id);
                steps.push({ node_id: node.id, node_key: node.key, status: 'success', output, ports: node.type === 'block.output' ? [] : ['out'], started_at: at, finished_at: at, duration_ms: 0 });
            }
            const end = doc.nodes.find(node => node.type === 'block.output');
            return { mode: 'simulation', started_at: at, status: 'success', outputs: roots[end.key], steps };
        } catch (_) { return { mode: 'simulation', started_at: at, status: 'error', error_code: 'FLOW_BLOCK_UNAVAILABLE', outputs: null, steps }; }
    }
    window.edBlockFixture.invokeVersion = (id, revision, inputs, at) => {
        const ref = state.versions.get(id + ':' + revision);
        return ref ? evaluate(ref.doc, inputs, at || new Date().toISOString(), new Set()) : { status: 'error', error_code: 'FLOW_BLOCK_UNAVAILABLE', outputs: null, steps: [] };
    };
    window.fetch = async (value, options) => {
        const url = new URL(String(value), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        if (url.pathname === base + '/node-types' && method === 'GET') {
            const result = await previous(value, options), body = await result.json();
            const call = window.edFixture.catalog.node_types.find(node => node.type === 'component.call');
            if (call) body.node_types.push(copy(call)); return response(body, result.status);
        }
        if (!url.pathname.startsWith(base + '/')) return previous(value, options);
        const parts = url.pathname.slice(base.length + 1).split('/'), rec = state.blocks.get(parts[0]);
        let body; try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (_) { return fail(400, 'FLOW_BAD_REQUEST'); }
        if (parts[0] === 'validate' && method === 'POST') {
            try { validate(body.doc); return response({ valid: true, issues: [] }); }
            catch (_) { return response({ valid: false, issues: [{ code: 'PARAM_INVALID', severity: 'error' }] }); }
        }
        if (rec && parts.length === 2 && parts[1] === 'dependents' && method === 'GET') {
            const wanted = Number(url.searchParams.get('revision')) || 0, items = [];
            function references(doc, seen) {
                const found = new Set();
                for (const node of doc.nodes || []) if (node.type === 'component.call') {
                    const id = node.params.flow_id, revision = node.params.revision, key = id + ':' + revision;
                    if (id === rec.id && (!wanted || wanted === revision)) found.add(revision);
                    if (!seen.has(key)) { const child = state.versions.get(key); if (child) { const next = new Set(seen); next.add(key); for (const value of references(child.doc, next)) found.add(value); } }
                }
                return found;
            }
            for (const [key, item] of state.versions) {
                const split = key.lastIndexOf(':'), owner = key.slice(0, split), revision = Number(key.slice(split + 1));
                for (const blockRevision of references(item.doc, new Set([key]))) items.push({ kind: 'published', flow_id: owner, revision, block_revision: blockRevision });
            }
            for (const entry of window.edFixture.state.flows.values()) if (entry.rec.live) {
                for (const blockRevision of references(entry.rec.live, new Set())) items.push({ kind: 'published', flow_id: entry.rec.id, revision: entry.rec.live_revision, block_revision: blockRevision });
            }
            items.sort((a, b) => a.flow_id.localeCompare(b.flow_id) || a.revision - b.revision || a.block_revision - b.block_revision);
            const start = Number((url.searchParams.get('cursor') || 'cursor_0').split('_')[1]) || 0, limit = Math.min(100, Number(url.searchParams.get('limit')) || 50);
            return response({ items: items.slice(start, start + limit), next_cursor: start + limit < items.length ? 'cursor_' + (start + limit) : undefined });
        }
        if (rec && parts.length === 1 && (method === 'GET' || method === 'PUT')) {
            const result = await previous(value, options);
            if (!result.ok || !rec.draft.nodes.some(node => node.type === 'component.call')) return result;
            const data = await result.json();
            try { validate(rec.draft); data.issues = []; } catch (_) { data.issues = [{ code: 'PARAM_INVALID', severity: 'error' }]; }
            return response(data, result.status);
        }
        if (!rec || parts.length !== 2 || !rec.draft.nodes.some(node => node.type === 'component.call')) return previous(value, options);
        if (parts[1] === 'publish-preview' && method === 'GET') {
            let issues = []; try { validate(rec.draft); } catch (_) { issues = [{ code: 'PARAM_INVALID', severity: 'error' }]; }
            return response({ draft_revision: rec.draft_revision, live_revision: rec.live_revision, published_draft_revision: rec.published_draft_revision, contract: contract(rec.draft), issues, effects: [], can_publish: !issues.length, diff: { first_publish: !rec.live, details: [] } });
        }
        if (parts[1] === 'publish' && method === 'POST') {
            if (body.base_revision !== rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT');
            try { validate(rec.draft); } catch (_) { return fail(422, 'FLOW_INVALID'); }
            if (rec.published_draft_revision !== rec.draft_revision) {
                rec.live = copy(rec.draft); rec.live_revision++; rec.published_draft_revision = rec.draft_revision; rec.published_at = new Date().toISOString();
                state.versions.set(rec.id + ':' + rec.live_revision, { doc: copy(rec.live), contract: contract(rec.live), published_at: rec.published_at });
            }
            return response({ block: copy(rec), issues: [] });
        }
        if (parts[1] === 'simulate' && method === 'POST') {
            if (body.base_revision !== rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT');
            return response(Object.assign({ revision: rec.draft_revision }, evaluate(rec.draft, body.inputs || {}, body.at || new Date().toISOString(), new Set())));
        }
        return previous(value, options);
    };
})();
