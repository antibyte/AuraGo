// Browser-only child history fixture; real collection and pin checks use server tests.
(function () {
    'use strict';

    const previous = window.fetch.bind(window);
    const clone = value => JSON.parse(JSON.stringify(value));
    const state = { traces: new Map(), documents: new Map(), simulations: new Map(), requests: [], simulationRequests: [], denied: false, unavailable: false };
    const key = (run, seq) => run + '/' + seq;
    const reply = (value, status = 200) => new Response(JSON.stringify(value), {
        status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }
    });
    const fail = (status, code) => reply({ code, error: code }, status);

    window.edChildHistoryFixture = {
        state,
        seed(runID, parentSeq, trace, documents) {
            const owner = key(runID, parentSeq);
            state.traces.set(owner, clone(trace));
            for (const [invocationID, document] of Object.entries(documents || {})) {
                state.documents.set(owner + '/' + invocationID, clone(document));
            }
        },
        clear() { state.traces.clear(); state.documents.clear(); state.simulations.clear(); state.requests.length = 0; state.simulationRequests.length = 0; state.denied = false; state.unavailable = false; }
    };

    // A completed parent run with a nested call; values and documents are local test data.
    // The actual collector, retention, and pin verification are covered by HTTP tests.
    window.edChildHistoryFixture.seedSample = function () {
        const flowID = edFixture.addEmptyDraft('Child history parent');
        const entry = edFixture.flow(flowID), doc = entry.rec.draft;
        const outerID = 'flow_aaaaaaaaaaaaaaaaaaaaaaaaaa', leafID = 'flow_bbbbbbbbbbbbbbbbbbbbbbbbbb';
        doc.nodes = [
            { id: 'n_aaaaaaaa', key: 'start', type: 'trigger.manual', type_version: 1, params: {}, settings: {}, position: { x: 0, y: 0 } },
            { id: 'n_bbbbbbbb', key: 'call', type: 'component.call', type_version: 1, params: { flow_id: outerID, revision: 1, inputs: { name: 'Ada' } }, settings: {}, position: { x: 360, y: 0 } }
        ];
        doc.edges = [{ id: 'e_aaaaaaaa', source: { node: 'n_aaaaaaaa', port: 'out' }, target: { node: 'n_bbbbbbbb', port: 'in' } }];
        entry.rec.live = clone(doc); entry.rec.live_revision = 1; entry.rec.published_draft_revision = entry.rec.draft_revision;
        const runID = edFixture.addWaitingRun(flowID), record = edFixture.state.runs.get(runID), at = record.run.started_at;
        record.run.status = 'success'; record.run.finished_at = at; record.run.trigger_type = 'manual'; record.run.child_history_state = 'complete'; record.done = true;
        const history = { parent_seq: 12, state: 'complete', retained_invocations: 2, total_invocations: 2, retained_steps: 2, total_steps: 2 };
        const step = (id, key, output) => ({ node_id: id, node_key: key, attempt: 1, status: 'success', started_at: at, finished_at: at, output });
        record.steps = [step('n_aaaaaaaa', 'start', { data: { name: 'Ada' } }), Object.assign(step('n_bbbbbbbb', 'call', { greeting: 'Hello Ada' }), { child_history: history })];
        const hash = 'a'.repeat(64), leafHash = 'b'.repeat(64);
        const frames = [
            { invocation_id: 'i_1', call_node_id: 'n_bbbbbbbb', call_attempt: 1, block_id: outerID, revision: 1, hash, entered: true, status: 'success', steps: [{ ordinal: 1, step: step('n_childaaa', 'child', { greeting: 'Hello Ada' }) }] },
            { invocation_id: 'i_2', parent_invocation_id: 'i_1', call_node_id: 'n_childaaa', call_attempt: 1, block_id: leafID, revision: 1, hash: leafHash, entered: true, status: 'success', steps: [{ ordinal: 1, step: step('n_leafaaaa', 'leaf', { greeting: 'Hello Ada' }) }] }
        ];
        const blockDoc = (id, name, node) => ({ id, schema: 1, kind: 'block', name, nodes: [node], edges: [] });
        const documents = {
            i_1: { block_id: outerID, revision: 1, hash, doc: blockDoc(outerID, 'Outer block', { id: 'n_childaaa', key: 'child', type: 'component.call', type_version: 1, params: {}, position: { x: 0, y: 0 } }) },
            i_2: { block_id: leafID, revision: 1, hash: leafHash, doc: blockDoc(leafID, 'Leaf block', { id: 'n_leafaaaa', key: 'leaf', type: 'logic.set', type_version: 1, params: {}, position: { x: 0, y: 0 } }) }
        };
        window.edChildHistoryFixture.seed(runID, 12, { schema: 1, run_id: runID, parent_seq: 12, history, frames }, documents);
        return { flow_id: flowID, run_id: runID };
    };

    // The simulated batch has no persistent run or child-history GET URLs.
    window.edChildHistoryFixture.seedSimulation = function () {
        const sample = window.edChildHistoryFixture.seedSample(), entry = edFixture.flow(sample.flow_id);
        const run = edFixture.state.runs.get(sample.run_id), owner = key(sample.run_id, 12);
        const trace = clone(state.traces.get(owner)); delete trace.run_id;
        trace.frames[0].item_index = 0;
        trace.frames[0].steps[0].step.output = { _preview: '', _omitted: 'child_history_omitted' };
        trace.frames[0].steps[0].step.output_truncated = true;
        trace.history.state = 'incomplete'; trace.history.truncated = true;
        trace.history.total_invocations = null; trace.history.total_steps = null;
        const node = entry.rec.draft.nodes[1]; node.type = 'component.each';
        node.params = { flow_id: trace.frames[0].block_id, revision: 1, items: ['Ada'], item_field: 'name', inputs: {}, max_items: 10, concurrency: 1, on_item_error: 'collect' };
        const steps = clone(run.steps);
        steps[1].child_history = clone(trace.history);
        steps[1].output = { items: [{ index: 0, status: 'success', output: { greeting: 'Hello Ada' } }], count: 1, succeeded: 1, failed: 0 };
        const documents = [state.documents.get(owner + '/i_1'), state.documents.get(owner + '/i_2')].map(clone);
        state.simulations.set(sample.flow_id, { mode: 'simulation', revision: 1, started_at: run.run.started_at, status: 'success', steps, child_traces: [trace], child_documents: documents });
        edFixture.state.runs.delete(sample.run_id);
        return sample.flow_id;
    };

    window.fetch = async (input, options) => {
        const url = new URL(String(input), location.origin);
        const method = String((options && options.method) || 'GET').toUpperCase();
        const simulation = url.pathname.match(/^\/api\/desktop\/flows\/([^/]+)\/simulate$/);
        if (simulation && method === 'POST' && state.simulations.has(simulation[1])) {
            state.simulationRequests.push({ method, path: url.pathname });
            return reply(clone(state.simulations.get(simulation[1])));
        }
        const match = url.pathname.match(/^\/api\/desktop\/flows\/runs\/([^/]+)\/steps\/(\d+)\/children(?:\/(i_[1-9]\d*)\/document)?$/);
        if (!match) return previous(input, options);
        state.requests.push({ method, path: url.pathname });
        if (state.denied) return fail(403, 'desktop_scope_required');
        if (state.unavailable) return fail(503, 'FLOW_CHILD_HISTORY_UNAVAILABLE');
        if (method !== 'GET') return fail(405, 'FLOW_BAD_REQUEST');
        const owner = key(decodeURIComponent(match[1]), Number(match[2]));
        if (!state.traces.has(owner)) return fail(404, 'FLOW_CHILD_HISTORY_NOT_FOUND');
        if (!match[3]) return reply(clone(state.traces.get(owner)));
        const document = state.documents.get(owner + '/' + match[3]);
        return document ? reply(clone(document)) : fail(404, 'FLOW_CHILD_HISTORY_NOT_FOUND');
    };
})();
