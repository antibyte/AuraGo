// EasyDrag durable approval inbox and exact-run review; opening and refreshing are read-only.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const STATES = new Set(['pending', 'approved', 'rejected', 'expired', 'cancelled', 'stale', 'unknown']);
    const REASONS = new Set(['approval_auth_unavailable', 'session_required', 'readonly', 'durable_disabled', 'stale', 'key_unavailable', 'checkpoint_unavailable', 'decision_complete', 'deadline_elapsed']);
    const RUN_STATES = new Set(['pending', 'queued', 'running', 'waiting', 'success', 'error', 'cancelled']);
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;
    const GATE_ID_PATTERN = /^[A-Za-z0-9_-]{1,128}$/;
    const NODE_ID_PATTERN = /^n_[a-z2-7]{8}$/;
    const NODE_KEY_PATTERN = /^[a-z][a-z0-9_]{0,39}$/;
    const PAGE_SIZE = 50;
    const REQUEST_MS = 10000;
    const runDialogs = new WeakMap();

    function object(value) { return !!value && typeof value === 'object' && !Array.isArray(value); }
    function nonempty(value, max) { return typeof value === 'string' && value.length > 0 && value.length <= max; }
    function integer(value, min) { return Number.isSafeInteger(value) && value >= min; }
    function date(value) { return typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value)); }
    function bytes(value) { return new TextEncoder().encode(value).length; }
    function jsonDepth(value, depth) {
        if (depth > 32) return false;
        if (Array.isArray(value)) return value.every(item => jsonDepth(item, depth + 1));
        if (object(value)) return Object.values(value).every(item => jsonDepth(item, depth + 1));
        if (typeof value === 'number') return Number.isFinite(value) && (!Number.isInteger(value) || Number.isSafeInteger(value));
        return value === null || ['string', 'boolean'].includes(typeof value);
    }
    function validItem(item) {
        return object(item) && GATE_ID_PATTERN.test(item.gate_id || '') && ID_PATTERN.test(item.run_id || '') && ID_PATTERN.test(item.flow_id || '') &&
            typeof item.flow_name === 'string' && Array.from(item.flow_name).length <= 120 && integer(item.revision, 1) &&
            NODE_ID_PATTERN.test(item.node_id || '') && NODE_KEY_PATTERN.test(item.node_key || '') && (item.node_label === undefined || typeof item.node_label === 'string' && Array.from(item.node_label).length <= 120) &&
            nonempty(item.admission_owner, 128) && integer(item.gate_generation, 1) &&
            /^[a-f0-9]{64}$/.test(item.content_digest || '') && date(item.expires_at) && STATES.has(item.state) && date(item.created_at) &&
            typeof item.review_available === 'boolean' && typeof item.can_decide === 'boolean' &&
            (item.blocked_reason === '' || REASONS.has(item.blocked_reason)) &&
            (item.actor === undefined || item.actor === '' || item.actor === 'local_admin') &&
            (item.decided_at === undefined || date(item.decided_at)) &&
            (item.decision === undefined || item.decision === '' || item.decision === 'approve' || item.decision === 'reject');
    }
    function validReview(review) {
        if (!object(review) || typeof review.summary !== 'string' || !review.summary.trim() || bytes(review.summary) > 2048 || !Object.prototype.hasOwnProperty.call(review, 'review_data')) return false;
        try {
            const serialized = JSON.stringify(review.review_data);
            return typeof serialized === 'string' && bytes(serialized) <= 16384 && jsonDepth(review.review_data, 0);
        } catch (_) { return false; }
    }
    function asItem(value) { return value; }
    function stateKey(state) { return 'easydrag.ui.approval_state_' + (STATES.has(state) ? state : 'unknown'); }
    function reasonKey(reason) { return 'easydrag.ui.approval_blocked_' + (REASONS.has(reason) ? reason : 'checkpoint_unavailable'); }
    function dateLabel(t, value) { return ED.core.fmt.dateTime(value); }

    function timeoutRequest(signal, invoke) {
        const controller = new AbortController();
        const abort = () => controller.abort();
        if (signal) {
            if (signal.aborted) controller.abort();
            else signal.addEventListener('abort', abort, { once: true });
        }
        let timer, timedOut = false;
        const deadline = new Promise((_, reject) => {
            timer = setTimeout(() => { timedOut = true; controller.abort(); reject(Object.assign(new Error('approval request timed out'), { name: 'TimeoutError' })); }, REQUEST_MS);
        });
        const request = Promise.resolve().then(() => {
            if (controller.signal.aborted) throw Object.assign(new Error('request cancelled'), { name: 'AbortError' });
            return invoke(controller.signal);
        });
        return Promise.race([request, deadline]).catch(err => {
            if (timedOut && err && err.name === 'AbortError') throw Object.assign(new Error('approval request timed out'), { name: 'TimeoutError' });
            throw err;
        }).finally(() => {
            clearTimeout(timer);
            if (signal) signal.removeEventListener('abort', abort);
        });
    }

    function createHome(app, host) {
        const button = host.querySelector('[data-ed-approval-inbox]');
        let dialog = null, disposed = false, sequence = 0;
        const onClick = () => { void openInbox(); };
        if (button) button.addEventListener('click', onClick);

        function alive(op) { return !disposed && dialog === op.dialog && !op.closed && op.dialog.el.isConnected; }
        function openInbox() {
            if (disposed) return;
            if (dialog && dialog.el.isConnected) { dialog.el.querySelector('[data-ed-action="refresh"]')?.focus(); return; }
            const op = { items: [], cursor: '', next: '', loading: false, error: '', dialog: null, closed: false, controller: null, sequence: ++sequence };
            const owned = dialog = ED.core.modal(host, {
                title: app.t('easydrag.ui.approval_inbox'), closeLabel: app.t('easydrag.ui.close'), className: 'ed-modal--approvals ed-modal--approval-inbox',
                body: '<p class="ed-hint" data-ed-approval-status role="status" aria-live="polite"></p><div data-ed-approval-list></div>',
                actions: [{ id: 'close', label: app.t('easydrag.ui.close') }, { id: 'refresh', label: app.t('easydrag.ui.approval_refresh') }],
                onAction(id, instance) {
                    if (id === 'close') { instance.close('close'); return false; }
                    if (id === 'refresh') { void load(op, '', false); return false; }
                    return false;
                }
            });
            op.dialog = owned;
            owned.body.addEventListener('click', event => {
                const open = event.target.closest('[data-ed-approval-open]');
                if (open && alive(op)) {
                    const item = op.items[Number(open.dataset.edApprovalOpen)];
                    if (!validItem(item) || !item.flow_id || !item.run_id || !item.gate_id) return;
                    owned.close('open');
                    if (typeof app.openFlow === 'function') app.openFlow(item.flow_id, { runId: item.run_id, gateId: item.gate_id });
                    return;
                }
                if (event.target.closest('[data-ed-approval-more]')) void load(op, op.next, true);
            });
            owned.done.then(() => {
                op.closed = true;
                if (op.controller) op.controller.abort();
                if (dialog === owned) dialog = null;
            });
            renderInbox(op);
            void load(op, '', false);
        }

        function renderInbox(op) {
            if (!alive(op)) return;
            const t = app.t, esc = app.esc || ED.core.esc;
            const status = op.loading ? t('easydrag.ui.approval_loading') : op.error ? ED.core.errorText(t, op.error) : (!op.items.length ? t('easydrag.ui.approval_empty') : '');
            op.dialog.body.querySelector('[data-ed-approval-status]').textContent = status;
            const list = op.dialog.body.querySelector('[data-ed-approval-list]');
            list.innerHTML = op.items.map((item, index) => '<article class="ed-approval-card">' +
                '<div class="ed-approval-card__top"><strong>' + esc(item.flow_name) + '</strong><span class="ed-approval-state ed-approval-state--' + esc(item.state) + '">' + esc(t(stateKey(item.state))) + '</span></div>' +
                '<dl class="ed-approval-meta"><dt>' + esc(t('easydrag.ui.approval_revision')) + '</dt><dd>' + esc(String(item.revision)) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_node')) + '</dt><dd>' + esc(item.node_label || item.node_key) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_created')) + '</dt><dd>' + esc(dateLabel(t, item.created_at)) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_expires')) + '</dt><dd>' + esc(dateLabel(t, item.expires_at)) + '</dd></dl>' +
                '<button type="button" class="ed-btn ed-btn--small" data-ed-approval-open="' + index + '"' + (op.loading ? ' disabled' : '') + '>' + esc(t('easydrag.ui.approval_review')) + '</button></article>').join('') +
                (op.next && !op.loading ? '<button type="button" class="ed-btn ed-btn--small" data-ed-approval-more>' + esc(t('easydrag.ui.approval_more')) + '</button>' : '');
            const refresh = op.dialog.el.querySelector('[data-ed-action="refresh"]');
            if (refresh) refresh.disabled = op.loading;
        }

        async function load(op, cursor, append) {
            if (!alive(op) || op.loading) return;
            op.loading = true; op.error = ''; renderInbox(op);
            const controller = op.controller = new AbortController(), mine = ++op.sequence;
            try {
                const page = await timeoutRequest(controller.signal, signal => app.api.approvalList(cursor, signal));
                if (!alive(op) || mine !== op.sequence || controller.signal.aborted) return;
                if (!object(page) || !Array.isArray(page.items) || page.items.length > PAGE_SIZE || page.items.some(item => !validItem(item)) ||
                    (page.next_cursor !== undefined && page.next_cursor !== '' && !nonempty(page.next_cursor, 256))) throw new Error('invalid approval page');
                const merged = append ? new Map(op.items.map(item => [item.gate_id, item])) : new Map();
                page.items.forEach(item => { if (!merged.has(item.gate_id)) merged.set(item.gate_id, item); });
                op.items = Array.from(merged.values()).slice(0, 500);
                op.next = page.next_cursor || '';
                if (op.items.length >= 500) op.next = '';
            } catch (err) {
                if (!alive(op) || controller.signal.aborted) return;
                op.error = err;
            } finally {
                if (alive(op) && mine === op.sequence) { op.loading = false; op.controller = null; renderInbox(op); }
            }
        }

        return {
            open: openInbox,
            dispose() {
                if (disposed) return;
                disposed = true; sequence++;
                if (button) button.removeEventListener('click', onClick);
                if (dialog) { const current = dialog; dialog = null; current.close('dispose'); }
            }
        };
    }

    function openReview(ed, gateId, expected) {
        if (!ed || !ed.root || !GATE_ID_PATTERN.test(gateId || '')) return null;
        const runView = ed.runView, run = runView && runView.run;
        if (!run || !ed.flow) return null;
        const route = Object.assign({ runId: run.id, flowId: ed.flow.id, revision: run.revision }, expected || {});
        if (route.runId !== run.id || route.flowId !== ed.flow.id || route.revision !== run.revision || !ID_PATTERN.test(route.runId || '') ||
            !ID_PATTERN.test(route.flowId || '') || !integer(route.revision, 1)) return null;
        const previous = runDialogs.get(ed);
        if (previous && alive(previous.op)) {
            if (previous.op.gateId === gateId && previous.op.expected.runId === route.runId) return previous.dialog;
            closeRunReview(ed);
        }
        const core = ED.core, t = ed.t, esc = ed.esc || core.esc;
        const op = { ed, root: ed.root, runView, gateId, dialog: null, closed: false, controller: null, item: null, review: null, expected: route, phase: 'loading', error: '', decision: '' };
        const owned = op.dialog = core.modal(ed.root, {
            title: t('easydrag.ui.approval_review'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--approvals ed-modal--approval-review', body: '<p class="ed-hint" data-ed-approval-status role="status" aria-live="polite"></p><div data-ed-approval-detail></div>',
            actions: [{ id: 'close', label: t('easydrag.ui.close') }, { id: 'refresh', label: t('easydrag.ui.approval_check_status') }],
            onAction: async (id, dialog) => {
                if (id === 'close') { dialog.close('close'); return false; }
                if (id === 'refresh') { void read(op); return false; }
                return false;
            }
        });
        runDialogs.set(ed, { op, dialog: owned });
        owned.body.addEventListener('click', event => {
            const start = event.target.closest('[data-ed-approval-start]');
            if (start && canDecide(op, ed)) {
                op.phase = 'confirm-' + start.dataset.edApprovalStart;
                renderReview(op, ed);
                owned.body.querySelector('[data-ed-approval-confirm]')?.focus();
                return;
            }
            if (event.target.closest('[data-ed-approval-cancel]')) {
                if (op.phase === 'confirm-approve' || op.phase === 'confirm-reject') { op.phase = 'review'; renderReview(op, ed); }
                return;
            }
            const confirm = event.target.closest('[data-ed-approval-confirm]');
            if (confirm && canConfirm(op, ed, confirm.dataset.edApprovalConfirm)) void decide(op, ed, confirm.dataset.edApprovalConfirm);
        });
        owned.done.then(() => {
            op.closed = true;
            if (op.controller) op.controller.abort();
            if (runDialogs.get(ed)?.op === op) runDialogs.delete(ed);
        });
        renderReview(op, ed);
        void read(op);
        return owned;
    }

    function alive(op) {
        if (!op || op.closed || !op.dialog || !op.dialog.el.isConnected || op.dialog.el.closest('.is-closing')) return false;
        const run = op.runView && op.runView.run, ed = op.ed;
        return ed.root === op.root && op.root.isConnected && ed.flow && ed.flow.id === op.expected.flowId &&
            ed.runView === op.runView && run && run.id === op.expected.runId && run.revision === op.expected.revision;
    }
    function closeRunReview(ed) {
        const current = ed && runDialogs.get(ed);
        if (!current) return;
        runDialogs.delete(ed);
        current.dialog.close('navigation');
    }
    function sameReviewIdentity(item, identity) {
        return !identity || ['gate_id', 'run_id', 'flow_id', 'revision', 'node_id', 'node_key', 'admission_owner', 'gate_generation', 'content_digest']
            .every(key => item[key] === identity[key]);
    }
    function expectedMatches(item, expected, gateId, submittedIdentity) {
        return item.gate_id === gateId && item.run_id === expected.runId && item.flow_id === expected.flowId && item.revision === expected.revision &&
            sameReviewIdentity(item, submittedIdentity);
    }
    function reviewIsAuthorized(op, ed) {
        return alive(op) && op.item && op.item.state === 'pending' && op.item.can_decide === true &&
            op.item.blocked_reason === '' && validReview(op.review) && !ed.readonly && !op.decision;
    }
    function canDecide(op, ed) { return reviewIsAuthorized(op, ed) && op.phase === 'review'; }
    function canConfirm(op, ed, decision) { return reviewIsAuthorized(op, ed) && op.phase === 'confirm-' + decision; }
    function setAction(dialog, id, show, disabled) {
        const button = dialog.el.querySelector('[data-ed-action="' + id + '"]');
        if (!button) return;
        button.hidden = !show;
        button.disabled = !!disabled;
    }
    function focusReviewRefresh(op) {
        if (!op.focusRefresh || !alive(op)) return;
        const button = op.dialog.el.querySelector('[data-ed-action="refresh"]');
        if (!button || button.disabled) return;
        op.focusRefresh = false;
        button.focus({ preventScroll: true });
    }

    function renderReview(op, ed) {
        if (!alive(op)) return;
        const t = ed.t, esc = ed.esc || ED.core.esc, body = op.dialog.body.querySelector('[data-ed-approval-detail]');
        const status = op.error ? ED.core.errorText(t, op.error) : op.phase === 'posting' ? t('easydrag.ui.loading') : op.phase === 'loading' || op.phase === 'refreshing' ? t('easydrag.ui.approval_loading') : '';
        op.dialog.body.querySelector('[data-ed-approval-status]').textContent = status;
        let markup = '';
        const item = op.item;
        if (item) {
            const stateLabel = t(stateKey(item.state));
            markup += '<section class="ed-approval-summary"><div class="ed-approval-card__top"><h3>' + esc(item.flow_name) + '</h3><span class="ed-approval-state ed-approval-state--' + esc(item.state) + '">' + esc(stateLabel) + '</span></div>' +
                '<dl class="ed-approval-meta"><dt>' + esc(t('easydrag.ui.approval_revision')) + '</dt><dd>' + esc(String(item.revision)) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_node')) + '</dt><dd>' + esc(item.node_label || item.node_key) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_created')) + '</dt><dd>' + esc(dateLabel(t, item.created_at)) + '</dd>' +
                '<dt>' + esc(t('easydrag.ui.approval_expires')) + '</dt><dd>' + esc(dateLabel(t, item.expires_at)) + '</dd></dl></section>';
            if (item.state === 'stale') markup += '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_stale_help')) + '</p>';
            if (item.state === 'unknown') markup += '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_unknown_help')) + '</p>';
            if (op.phase === 'uncertain') markup += '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_uncertain')) + '</p>';
            if (item.blocked_reason && item.blocked_reason !== 'decision_complete' && REASONS.has(item.blocked_reason)) markup += '<p class="ed-callout ed-callout--warn">' + esc(t(reasonKey(item.blocked_reason))) + '</p>';
            if (ed.readonly) markup += '<p class="ed-callout">' + esc(t('easydrag.ui.approval_readonly')) + '</p>';
            if (item.state === 'pending' && item.review_available && op.review && validReview(op.review)) {
                markup += '<section class="ed-approval-review-data"><h3>' + esc(t('easydrag.ui.approval_summary')) + '</h3><p class="ed-approval-literal" data-ed-approval-summary></p>' +
                    '<h3>' + esc(t('easydrag.ui.approval_review_data')) + '</h3><pre class="ed-approval-json" data-ed-approval-json></pre></section>';
            } else if (['approved', 'rejected', 'expired', 'cancelled'].includes(item.state)) {
                markup += '<p class="ed-hint">' + esc(t('easydrag.ui.approval_review_not_retained')) + '</p>';
            } else if (item.state !== 'pending' || !item.review_available || !op.review) {
                markup += '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_review_unavailable')) + '</p>';
            }
            if (op.phase === 'confirm-approve') markup += '<p class="ed-callout ed-callout--warn" data-ed-approval-confirm-copy>' + esc(t('easydrag.ui.approval_confirm_approve')) + '</p>';
            if (op.phase === 'confirm-reject') markup += '<p class="ed-callout ed-callout--warn" data-ed-approval-confirm-copy>' + esc(t('easydrag.ui.approval_confirm_reject')) + '</p>';
            if (op.phase === 'confirm-approve' || op.phase === 'confirm-reject') markup += '<div class="ed-approval-actions"><button type="button" class="ed-btn" data-ed-approval-cancel>' + esc(t('easydrag.ui.close')) + '</button><button type="button" class="ed-btn ' + (op.phase === 'confirm-reject' ? 'ed-btn--danger' : 'ed-btn--primary') + '" data-ed-approval-confirm="' + (op.phase === 'confirm-approve' ? 'approve' : 'reject') + '">' + esc(t(op.phase === 'confirm-approve' ? 'easydrag.ui.approval_approve' : 'easydrag.ui.approval_reject')) + '</button></div>';
            else if (canDecide(op, ed)) markup += '<div class="ed-approval-actions"><button type="button" class="ed-btn ed-btn--primary" data-ed-approval-start="approve">' + esc(t('easydrag.ui.approval_approve')) + '</button><button type="button" class="ed-btn ed-btn--danger" data-ed-approval-start="reject">' + esc(t('easydrag.ui.approval_reject')) + '</button></div>';
            if (op.phase === 'recorded' || ['approved', 'rejected', 'expired', 'cancelled'].includes(item.state)) markup += '<p class="ed-callout" role="status">' + esc(t('easydrag.ui.approval_decision_saved', { state: t(stateKey(item.state)) })) + '</p>' + (op.runState && RUN_STATES.has(op.runState) ? '<p class="ed-hint">' + esc(t('easydrag.ui.approval_run')) + ': ' + esc(ED.core.tr(t, 'easydrag.ui.status_' + op.runState, op.runState)) + '</p>' : '') + (item.actor === 'local_admin' ? '<p class="ed-hint">' + esc(t('easydrag.ui.approval_actor')) + '</p>' : '') + (item.decided_at ? '<p class="ed-hint">' + esc(t('easydrag.ui.approval_decided_at')) + ': ' + esc(dateLabel(t, item.decided_at)) + '</p>' : '');
        } else if (op.phase === 'unknown' || op.phase === 'uncertain') markup = '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_unknown_help')) + '</p>';
        else if (op.phase === 'unavailable' || !op.error && op.phase !== 'loading' && op.phase !== 'refreshing') markup = '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.approval_review_unavailable')) + '</p>';
        body.innerHTML = markup;
        if (item && item.state === 'pending' && item.review_available && op.review && validReview(op.review)) {
            body.querySelector('[data-ed-approval-summary]').textContent = op.review.summary;
            body.querySelector('[data-ed-approval-json]').textContent = JSON.stringify(op.review.review_data, null, 2);
        }
        setAction(op.dialog, 'refresh', true, op.phase === 'posting' || op.phase === 'loading' || op.phase === 'refreshing');
        focusReviewRefresh(op);
    }

    async function read(op) {
        if (!alive(op) || op.phase === 'posting' || op.phase === 'loading' && op.controller) return;
        const previous = op.phase;
        op.phase = op.item ? 'refreshing' : 'loading'; op.error = ''; renderReview(op, op.ed);
        if (op.controller) op.controller.abort();
        const controller = op.controller = new AbortController();
        try {
            const reply = await timeoutRequest(controller.signal, signal => op.ed.api.approvalDetail(op.gateId, signal));
            if (!alive(op) || controller.signal.aborted) return;
            const item = asItem(reply);
            if (!validItem(item) || !expectedMatches(item, op.expected, op.gateId, op.submittedIdentity)) throw Object.assign(new Error('approval identity did not match'), { body: { code: 'FLOW_DURABLE_APPROVAL_CONFLICT' } });
            op.item = item;
            op.review = item.state === 'pending' && item.review_available && validReview(reply.review) ? reply.review : null;
            op.phase = op.decision && item.state === 'pending' ? 'uncertain' : item.state === 'pending' && item.review_available && !op.review ? 'unavailable' : 'review';
            if (op.phase === 'uncertain') op.item = Object.assign({}, item, { can_decide: false });
            op.error = '';
            if (op.phase === 'unavailable') op.error = Object.assign(new Error('complete approval review unavailable'), { body: { code: 'FLOW_DURABLE_APPROVAL_UNAVAILABLE' } });
        } catch (err) {
            if (!alive(op) || controller.signal.aborted) return;
            op.error = err;
            op.phase = previous === 'uncertain' ? 'uncertain' : 'unknown';
            if (previous === 'uncertain') op.item = Object.assign({}, op.item, { state: 'unknown', can_decide: false });
            else op.item = null;
        } finally {
            if (alive(op) && op.controller === controller) { op.controller = null; renderReview(op, op.ed); }
        }
    }

    async function decide(op, ed, decision) {
        if (!canConfirm(op, ed, decision)) return;
        const reviewed = op.item, review = op.review;
        op.submittedIdentity = {
            gate_id: reviewed.gate_id, run_id: reviewed.run_id, flow_id: reviewed.flow_id, revision: reviewed.revision,
            node_id: reviewed.node_id, node_key: reviewed.node_key, admission_owner: reviewed.admission_owner,
            gate_generation: reviewed.gate_generation, content_digest: reviewed.content_digest
        };
        op.phase = 'posting'; op.error = ''; renderReview(op, ed);
        const controller = op.controller = new AbortController();
        try {
            const reply = await timeoutRequest(controller.signal, signal => ed.api.decideApproval(op.gateId, {
                gate_generation: reviewed.gate_generation, content_digest: reviewed.content_digest, decision
            }, signal));
            if (!alive(op) || controller.signal.aborted) return;
            const resultItem = asItem(reply), expectedState = decision === 'approve' ? 'approved' : 'rejected';
            if (!validItem(resultItem) || !sameReviewIdentity(resultItem, op.submittedIdentity) || resultItem.state !== expectedState || resultItem.decision !== decision || reply.decision !== decision || resultItem.actor !== 'local_admin' || !date(resultItem.decided_at) || resultItem.review_available !== false || resultItem.can_decide !== false || !RUN_STATES.has(reply.run_state)) {
                throw Object.assign(new Error('approval acknowledgement was incomplete'), { name: 'UncertainApprovalError' });
            }
            op.item = resultItem; op.runState = reply.run_state; op.review = null; op.phase = 'recorded'; op.decision = decision; op.error = '';
        } catch (err) {
            if (!alive(op) || controller.signal.aborted) return;
            op.phase = 'uncertain'; op.error = err; op.item = Object.assign({}, reviewed, { can_decide: false }); op.review = review; op.decision = decision;
        } finally {
            if (alive(op) && op.controller === controller) { op.controller = null; renderReview(op, ed); }
        }
    }

    function openRunReview(ed, gateId, expected) {
        const dialog = openReview(ed, gateId, expected);
        const current = dialog && runDialogs.get(ed);
        if (current && current.dialog === dialog) {
            current.op.focusRefresh = true;
            focusReviewRefresh(current.op);
        }
        return dialog;
    }

    ED.approvals = { createHome, openReview, openRunReview, closeRunReview, validItem, validReview, reasonKey };
})();
