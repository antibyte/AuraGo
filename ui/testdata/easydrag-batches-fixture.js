// Browser-only synchronous batch fixture. The pinned child runs through the block fixture.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), clone = value => JSON.parse(JSON.stringify(value));
    const base = '/api/desktop/flows', requests = [];
    const reply = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const fail = (status, code) => reply({ code, error: code }, status);
    window.edFixture.registerNode({
        type: 'component.each', version: 1, category: 'logic', icon: 'repeat', label: '', description: '',
        inputs: ['in'], outputs: ['out'], availability: { state: 'available' },
        params: [
            { name: 'items', kind: 'json', templatable: true, required: true },
            { name: 'max_items', kind: 'number', default: 100, min: 1, max: 1000 },
            { name: 'on_item_error', kind: 'select', default: 'stop', options: [{ value: 'stop', label: 'Stop' }, { value: 'collect', label: 'Collect' }] }
        ],
        output_fields: [{ name: 'items', type: 'list' }, { name: 'count', type: 'number' }, { name: 'succeeded', type: 'number' }, { name: 'failed', type: 'number' }]
    });
    function simulate(entry, body) {
        const doc = entry.rec.draft;
        if (body.base_revision !== entry.rec.draft_revision) return fail(409, 'FLOW_REVISION_CONFLICT');
        const trigger = doc.nodes.find(node => node.id === body.trigger_node && node.type === 'trigger.manual');
        if (!trigger) return fail(409, 'FLOW_NO_TRIGGER');
        const node = doc.nodes.find(item => item.type === 'component.each');
        if (!node || body.fixtures && body.fixtures[node.id]) return fail(400, 'FLOW_BAD_REQUEST');
        const at = body.at || new Date().toISOString();
        const data = body.trigger_data || window.edFixture.state.testData[entry.rec.id + '/' + trigger.id] || trigger.params.data || {};
        const roots = { trigger: { data, fired_at: at }, run: { mode: 'simulation', started_at: at }, flow: { id: entry.rec.id, name: entry.rec.name } };
        const params = node.params || {}, evaluate = value => window.EasyDrag.template.evaluate(value, roots);
        const items = typeof params.items === 'string' ? evaluate(params.items) : clone(params.items);
        const shared = evaluate(params.inputs || {}), max = params.max_items;
        let code = '', index = -1, childCode = '';
        const results = [];
        if (!Array.isArray(items) || !Number.isInteger(max) || max < 1 || max > 1000 || items.length > max || params.concurrency !== 1 || !['stop', 'collect'].includes(params.on_item_error) || Object.prototype.hasOwnProperty.call(shared, params.item_field)) code = 'FLOW_PARAM_INVALID';
        else for (let i = 0; i < items.length; i++) {
            const result = window.edBlockFixture.invokeVersion(params.flow_id, params.revision, Object.assign({}, shared, { [params.item_field]: items[i] }), at);
            if (result.status === 'success') results.push({ index: i, status: 'success', output: result.outputs });
            else if (params.on_item_error === 'collect') results.push({ index: i, status: 'error', error_code: result.error_code || 'FLOW_NODE_FAILED' });
            else { code = 'FLOW_BATCH_ITEM_FAILED'; index = i; childCode = result.error_code || 'FLOW_NODE_FAILED'; break; }
        }
        const failed = results.filter(item => item.status === 'error').length;
        const output = code ? undefined : { items: results, count: results.length, succeeded: results.length - failed, failed };
        const step = { node_id: node.id, node_key: node.key, status: code ? 'error' : 'success', started_at: at, finished_at: at, duration_ms: 0, params: Object.assign({}, params, { items, inputs: shared }), output, ports: code ? [] : ['out'] };
        if (code) { step.error_code = code; if (index >= 0) step.error_message = 'batch item ' + index + ' failed (' + childCode + ')'; }
        return reply({ mode: 'simulation', revision: entry.rec.draft_revision, started_at: at, status: code ? 'error' : 'success', error_code: code || undefined, error_node_id: code ? node.id : undefined, steps: [{ node_id: trigger.id, node_key: trigger.key, status: 'success', started_at: at, finished_at: at, duration_ms: 0, output: { data, fired_at: at }, ports: ['out'] }, step] });
    }
    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin), opts = options || {}, method = String(opts.method || 'GET').toUpperCase();
        if (url.pathname === base + '/node-types' && method === 'GET') {
            const response = await previous(input, opts);
            if (!response.ok) return response;
            const catalog = await response.json();
            catalog.node_types = catalog.node_types.map(info => info.type === 'component.each' ? Object.assign({}, info, { label: window.t('easydrag.node.component_each.label'), description: window.t('easydrag.node.component_each.description') }) : info);
            return reply(catalog, response.status);
        }
        const match = url.pathname.match(/^\/api\/desktop\/flows\/([^/]+)\/simulate$/);
        if (!match || method !== 'POST') return previous(input, opts);
        const entry = window.edFixture.state.flows.get(decodeURIComponent(match[1]));
        if (!entry || !entry.rec.draft.nodes.some(node => node.type === 'component.each')) return previous(input, opts);
        let body; try { body = opts.body ? JSON.parse(opts.body) : {}; } catch (_) { return fail(400, 'FLOW_BAD_REQUEST'); }
        requests.push({ method, path: url.pathname, body: clone(body) });
        return simulate(entry, body);
    };
    window.edBatchFixture = { requests };
})();
