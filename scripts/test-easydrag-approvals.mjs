import assert from 'node:assert/strict';
import { approvalItem, loadApprovals } from './easydrag-approvals-test-helper.mjs';

const approvals = loadApprovals();
assert.equal(approvals.validItem(approvalItem()), true, 'accepts the direct pending gate DTO');
assert.equal(approvals.validItem({ item: approvalItem() }), false, 'does not accept an unsupported item envelope');

for (const [field, value] of [
    ['gate_id', 'g'.repeat(129)], ['run_id', 'r'.repeat(65)], ['flow_id', 'invalid id'], ['node_id', 'n_invalid'],
    ['node_key', 'a'.repeat(41)], ['flow_name', '😀'.repeat(121)], ['gate_generation', 0], ['content_digest', 'A'.repeat(64)],
    ['blocked_reason', 'unknown_reason'], ['actor', 'system']
]) {
    assert.equal(approvals.validItem(approvalItem({ [field]: value })), false, `rejects invalid ${field}`);
}

assert.equal(approvals.validItem(approvalItem({ state: 'expired', decision: '', actor: '', review_available: false, can_decide: false, blocked_reason: 'deadline_elapsed' })), true,
    'accepts automatic expiry metadata without inventing a human actor or decision');
assert.equal(approvals.reasonKey('approval_auth_unavailable'), 'easydrag.ui.approval_blocked_approval_auth_unavailable');
assert.notEqual(approvals.reasonKey('approval_auth_unavailable'), approvals.reasonKey('session_required'), 'keeps disabled login distinct from a missing session');

assert.equal(approvals.validReview({ summary: 'Prüfe die Anfrage', review_data: { amount: 9007199254740991, lines: ['a', 'b'] } }), true);
assert.equal(approvals.validReview({ summary: 'Prüfe die Anfrage', review_data: { amount: JSON.parse('9007199254740993') } }), false,
    'fails closed when JSON parsing has rounded an unsafe integer');
assert.equal(approvals.validReview({ summary: ' ', review_data: {} }), false, 'requires a nonblank summary');
assert.equal(approvals.validReview({ summary: 'Prüfe', review_data: { payload: 'x'.repeat(16384) } }), false, 'keeps the complete review inside its byte bound');

console.log('EasyDrag approvals: DTO bounds, exact run metadata, review safety, and blocked-reason semantics passed.');
