// Browser fixture for the durable approval inbox and one-shot decision flow.
(function () {
    'use strict';

    const previous = window.fetch.bind(window), copy = value => JSON.parse(JSON.stringify(value));
    const response = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' } });
    const state = { requests: [], gates: new Map(), pages: 0, dropNextDecisionReply: true, pendingDecision: null, failNextDetails: new Map(), delayGateID: '', delayMs: 0, delayedReads: 0 };

    function waitingRun(flowID) {
        const id = edFixture.addWaitingRun(flowID), record = edFixture.state.runs.get(id);
        record.run.durable = true;
        record.run.status = 'waiting';
        record.run.trigger_type = 'manual';
        record.steps = [{ node_id: 'n_aaaaaaaa', node_key: 'approval', status: 'waiting', started_at: record.run.started_at }];
        return id;
    }

    function addGate(id, runID, letter, blockedReason, summary) {
        const flow = edFixture.flow();
        const item = {
            gate_id: id, run_id: runID, flow_id: flow.rec.id, flow_name: flow.rec.name, revision: 1,
            node_id: 'n_aaaaaaaa', node_key: 'approval', node_label: 'Freigabe prüfen',
            admission_owner: 'mission_' + flow.rec.id, gate_generation: 2, content_digest: letter.repeat(64),
            expires_at: new Date(Date.now() + 600000).toISOString(), state: 'pending', created_at: new Date().toISOString(),
            review_available: true, can_decide: !blockedReason, blocked_reason: blockedReason || ''
        };
        const review = { summary, review_data: { amount: 42, currency: 'EUR', note: '<img src=x onerror="alert(1)">', payload: letter.repeat(9000) } };
        state.gates.set(id, { item, review });
        return item;
    }

    function seed() {
        const flowID = edFixture.seededID;
        const run1 = waitingRun(flowID), run2 = waitingRun(flowID), run3 = waitingRun(flowID);
        const first = addGate('gate_aaaaaaaa', run1, 'a', '', 'Freigabe 1: <script>window.pwned=true</script>');
        const second = addGate('gate_bbbbbbbb', run2, 'b', 'session_required', 'Freigabe 2: Sitzung erforderlich');
        const third = addGate('gate_cccccccc', run3, 'c', '', 'Freigabe 3: Direkte Bestätigung');
        return { gate1: first.gate_id, run1, gate2: second.gate_id, run2, gate3: third.gate_id, run3 };
    }

    function gateForRun(runID) {
        return Array.from(state.gates.values()).find(gate => gate.item.run_id === runID);
    }

    function commitDecision(gate, decision) {
        Object.assign(gate.item, { state: decision === 'approve' ? 'approved' : 'rejected', decision, actor: 'local_admin', decided_at: new Date().toISOString(), review_available: false, can_decide: false, blocked_reason: 'decision_complete' });
        const run = edFixture.state.runs.get(gate.item.run_id);
        if (run) { run.run.status = decision === 'approve' ? 'success' : 'error'; run.run.finished_at = gate.item.decided_at; run.done = true; }
        gate.review = null;
        state.pendingDecision = null;
    }

    window.fetch = async (input, options) => {
        const u = new URL(String(input), location.origin), method = String(options && options.method || 'GET').toUpperCase();
        if (u.pathname === '/api/desktop/flows/approvals' && method === 'GET') {
            state.requests.push({ method, path: u.pathname, cursor: u.searchParams.get('cursor') || '' });
            const gates = Array.from(state.gates.values());
            if (!u.searchParams.has('cursor')) {
                state.pages++;
                return response({ items: [copy(gates[0].item)], next_cursor: 'next' });
            }
            state.pages++;
            return response({ items: [copy(gates[0].item), copy(gates[1].item), copy(gates[2].item)] });
        }

        const approval = /^\/api\/desktop\/flows\/approvals\/([^/]+)(?:\/(decision))?$/.exec(u.pathname);
        if (approval) {
            const gateID = decodeURIComponent(approval[1]), gate = state.gates.get(gateID);
            state.requests.push({ method, path: u.pathname, gateID });
            if (!gate) return response({ error: 'not found', code: 'FLOW_DURABLE_APPROVAL_NOT_FOUND' }, 404);
            if (!approval[2] && method === 'GET') {
                if (state.delayGateID === gateID) {
                    await new Promise(resolve => setTimeout(resolve, state.delayMs));
                    state.delayedReads++;
                    state.delayGateID = '';
                }
                if (state.failNextDetails.get(gateID)) {
                    state.failNextDetails.set(gateID, 0);
                    return response({ error: 'status unavailable', code: 'FLOW_DURABLE_APPROVAL_UNAVAILABLE' }, 503);
                }
                if (state.pendingDecision && state.pendingDecision.gateID === gateID && ++state.pendingDecision.reads >= 3) commitDecision(gate, state.pendingDecision.decision);
                return response(Object.assign(copy(gate.item), gate.item.review_available ? { review: copy(gate.review) } : {}));
            }
            if (approval[2] && method === 'POST') {
                const body = JSON.parse(options.body || '{}');
                state.requests[state.requests.length - 1].body = body;
                if (body.gate_generation !== gate.item.gate_generation || body.content_digest !== gate.item.content_digest || !['approve', 'reject'].includes(body.decision)) {
                    return response({ error: 'decision conflict', code: 'FLOW_DURABLE_APPROVAL_CONFLICT' }, 409);
                }
                if (state.dropNextDecisionReply) {
                    state.dropNextDecisionReply = false;
                    state.pendingDecision = { gateID, decision: body.decision, reads: 0 };
                    throw new DOMException('accepted response was lost', 'AbortError');
                }
                commitDecision(gate, body.decision);
                const result = Object.assign(copy(gate.item), { run_state: 'waiting', resume_generation: gate.item.gate_generation + 1, idempotent: false });
                return response(result);
            }
        }

        const run = /^\/api\/desktop\/flows\/runs\/([^/]+)$/.exec(u.pathname);
        if (run && method === 'GET') {
            const result = await previous(input, options);
            if (!result.ok) return result;
            const body = await result.json(), gate = gateForRun(decodeURIComponent(run[1]));
            if (gate) {
                const record = edFixture.state.runs.get(gate.item.run_id), terminal = record && ['success', 'error', 'cancelled'].includes(record.run.status);
                body.durable_status = { run_id: gate.item.run_id, state: terminal ? record.run.status : 'approval_pending', generation: terminal ? 0 : gate.item.gate_generation, cancel_available: !terminal, approval_gate_id: gate.item.gate_id };
            }
            return response(body);
        }
        return previous(input, options);
    };

    window.edApprovalFixture = {
        state, seed,
        failNextDetail(gateID) { state.failNextDetails.set(gateID, 1); },
        delayNextDetail(gateID, milliseconds) { state.delayGateID = gateID; state.delayMs = milliseconds; }
    };
})();
