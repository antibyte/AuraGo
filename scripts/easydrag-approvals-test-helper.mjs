import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

const source = readFileSync(new URL('../ui/js/desktop/apps/easydrag-approvals.js', import.meta.url), 'utf8');

export function loadApprovals() {
    const window = { EasyDrag: { core: {} } };
    runInNewContext(source, { window, TextEncoder }, { filename: 'easydrag-approvals.js' });
    return window.EasyDrag.approvals;
}

export function approvalItem(overrides = {}) {
    const now = new Date().toISOString();
    return Object.assign({
        gate_id: 'gate_aaaaaaaa', run_id: 'run_aaaaaaaa', flow_id: 'flow_aaaaaaaa', flow_name: 'Freigabe prüfen', revision: 1,
        node_id: 'n_aaaaaaaa', node_key: 'approval', admission_owner: 'mission_owner', gate_generation: 1, content_digest: 'a'.repeat(64),
        expires_at: now, state: 'pending', created_at: now, review_available: true, can_decide: true, blocked_reason: ''
    }, overrides);
}
