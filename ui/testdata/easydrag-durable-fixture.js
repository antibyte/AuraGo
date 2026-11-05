// Browser-only durable metadata and generation CAS. No real config or runtime is enabled.
(function () {
    'use strict';
    const previous = window.fetch.bind(window), copy = value => JSON.parse(JSON.stringify(value));
    const state = { requests: [], phases: new Map(), sequence: new Map(), conflictNext: false, capability: { configured: false, effective: false, recovery_available: false, reason: 'disabled', waiting_count: 0, claimed_count: 0, blocked_count: 0 } };
    const response = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    edFixture.registerNode({ type: 'logic.durable_wait', type_version: 1, label: 'Fortsetzbare Wartezeit', description: 'Gespeicherte Wartezeit für einen rein lokalen Flow.', icon: 'clock', category: 'logic', inputs: ['in'], outputs: ['out'], params: [{ name: 'seconds', kind: 'number', required: true, min: 1, max: 86400 }], output_fields: [{ name: 'waited_seconds', type: 'number', primary: true }], availability: { state: 'needs_setup', code: 'FLOW_DURABLE_DISABLED', config_section: 'flows' } });
    function seed(name, legacy, withoutRun) {
        const id = edFixture.addEmptyDraft(name), entry = edFixture.flow(id), doc = entry.rec.draft;
        doc.nodes = [
            { id: 'n_aaaaaaaa', key: 'start', type: 'trigger.manual', type_version: 1, params: {}, settings: {}, position: { x: 0, y: 0 } },
            { id: 'n_bbbbbbbb', key: 'wait', type: 'logic.durable_wait', type_version: 1, params: { seconds: 60 }, settings: {}, position: { x: 350, y: 0 } }
        ];
        doc.edges = [{ id: 'e_aaaaaaaa', source: { node: 'n_aaaaaaaa', port: 'out' }, target: { node: 'n_bbbbbbbb', port: 'in' } }];
        entry.rec.live = copy(doc); entry.rec.live_revision = 1; entry.rec.published_draft_revision = entry.rec.draft_revision; entry.enabled = true;
        if (withoutRun) return { flow_id: id, run_id: '' };
        const runId = edFixture.addWaitingRun(id), record = edFixture.state.runs.get(runId);
        record.run.durable = !legacy; record.run.status = legacy ? 'error' : 'waiting'; record.run.trigger_type = 'manual';
        record.steps = [{ node_id: 'n_aaaaaaaa', node_key: 'start', status: 'success', output: { data: { sample: true } }, started_at: record.run.started_at }, { node_id: 'n_bbbbbbbb', node_key: 'wait', status: 'waiting', started_at: record.run.started_at }];
        state.phases.set(runId, { run_id: runId, state: 'blocked', blocked_code: legacy ? 'incompatible' : 'disabled', generation: 2, cancel_available: true, wake_at: new Date(Date.now() + 60000).toISOString(), deadline: new Date(Date.now() + 600000).toISOString() });
        state.sequence.set(runId, 2);
        state.capability.blocked_count++;
        return { flow_id: id, run_id: runId };
    }
    function start(flowId) {
        const runId = edFixture.addWaitingRun(flowId), record = edFixture.state.runs.get(runId);
        record.run.durable = true; record.run.trigger_type = 'manual';
        record.steps = [{ node_id: 'n_aaaaaaaa', node_key: 'start', status: 'success', output: { data: { sample: true } }, started_at: record.run.started_at }, { node_id: 'n_bbbbbbbb', node_key: 'wait', status: 'waiting', started_at: record.run.started_at }];
        state.phases.set(runId, { run_id: runId, state: 'waiting', generation: 1, cancel_available: true, wake_at: new Date(Date.now() + 60000).toISOString(), deadline: new Date(Date.now() + 600000).toISOString() });
        state.sequence.set(runId, 0); state.capability.waiting_count++;
        return runId;
    }
    function event(runId, type, extra) {
        const record = edFixture.state.runs.get(runId), seq = (state.sequence.get(runId) || 0) + 1;
        state.sequence.set(runId, seq);
        const ev = Object.assign({ seq, run_id: runId, type, time: new Date().toISOString() }, extra);
        record.events.push(ev); [...record.listeners].forEach(listener => listener(copy(ev)));
    }
    function completeCancel(runId) {
        const record = edFixture.state.runs.get(runId);
        if (!record.pendingDurableCancel) throw new Error('No accepted cancel');
        record.pendingDurableCancel = false;
        const previousPhase = state.phases.get(runId);
        const counter = previousPhase?.state === 'blocked' ? 'blocked_count' : previousPhase?.state === 'running' ? 'claimed_count' : 'waiting_count';
        state.capability[counter] = Math.max(0, state.capability[counter] - 1); state.phases.delete(runId);
        if (record.run.durable) { record.run.status = 'cancelled'; record.run.finished_at = new Date().toISOString(); }
        record.done = true;
        event(runId, 'run_finished', { run: copy(record.run) });
        [...record.listeners].forEach(listener => listener(null)); record.listeners.clear();
    }
    window.fetch = async (input, options) => {
        const u = new URL(String(input), location.origin), method = (options && options.method || 'GET').toUpperCase();
        if (u.pathname === '/api/desktop/flow-runtime') { state.requests.push({ method, path: u.pathname }); return response(state.capability); }
        const cancel = /^\/api\/desktop\/flows\/runs\/([^/]+)\/durable\/cancel$/.exec(u.pathname);
        const run = /^\/api\/desktop\/flows\/([^/]+)\/run$/.exec(u.pathname);
        if (run && method === 'POST' && edFixture.flow(run[1])?.rec.live.nodes.some(node => node.type === 'logic.durable_wait')) {
            state.requests.push({ method, path: u.pathname });
            if (!state.capability.effective || !state.capability.recovery_available) return response({ code: 'FLOW_DURABLE_DISABLED' }, 503);
            return response({ run_id: start(run[1]) }, 202);
        }
        if (cancel && method === 'POST') {
            const body = JSON.parse(options.body || '{}'), runId = cancel[1], current = state.phases.get(runId);
            state.requests.push({ method, path: u.pathname, body });
            if (state.conflictNext) { state.conflictNext = false; if (current) current.generation++; return response({ code: 'FLOW_DURABLE_GENERATION_CONFLICT' }, 409); }
            if (!current || body.generation !== current.generation) return response({ code: 'FLOW_DURABLE_GENERATION_CONFLICT' }, 409);
            edFixture.state.runs.get(runId).pendingDurableCancel = true;
            return response({ accepted: true });
        }
        const get = /^\/api\/desktop\/flows\/runs\/([^/]+)$/.exec(u.pathname);
        if (get && method === 'GET') {
            state.requests.push({ method, path: u.pathname });
            const result = await previous(input, options);
            if (!result.ok) return result;
            const body = await result.json(), phase = state.phases.get(get[1]);
            if (phase) body.durable_status = copy(phase);
            return response(body);
        }
        if (method !== 'GET' && u.pathname.startsWith('/api/desktop/flows')) state.requests.push({ method, path: u.pathname, body: options && options.body ? JSON.parse(options.body) : undefined });
        return previous(input, options);
    };
    window.edDurableFixture = { state, seed, completeCancel,
        disconnect(runId) { edFixture.state.streams.filter(stream => !stream.closed && stream.url.includes(runId)).forEach(stream => stream.fail()); },
        beginWait(runId) { const record = edFixture.state.runs.get(runId); event(runId, 'step_finished', { node_id: 'n_bbbbbbbb', step: copy(record.steps[1]) }); },
        wake(runId) {
            const record = edFixture.state.runs.get(runId), phase = state.phases.get(runId);
            record.run.status = 'running'; phase.state = 'running'; phase.generation++;
            record.steps[1] = { ...record.steps[1], status: 'success', output: { waited_seconds: 60 } };
            state.capability.waiting_count--; state.capability.claimed_count++;
            event(runId, 'step_finished', { node_id: 'n_bbbbbbbb', step: copy(record.steps[1]) });
        },
        finish(runId) {
            const record = edFixture.state.runs.get(runId); record.run.status = 'success'; record.run.finished_at = new Date().toISOString(); record.done = true;
            state.phases.delete(runId); state.capability.claimed_count = Math.max(0, state.capability.claimed_count - 1);
            event(runId, 'run_finished', { run: copy(record.run) }); [...record.listeners].forEach(listener => listener(null)); record.listeners.clear();
        },
        forgetBacklog(runId) { edFixture.state.runs.get(runId).events = []; }
    };
})();
