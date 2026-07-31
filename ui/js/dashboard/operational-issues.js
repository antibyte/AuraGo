// Administrative operational-issue lifecycle controls.
(() => {
    'use strict';

    const state = {
        offset: 0,
        limit: 25,
        total: 0,
        pendingAction: null,
        initialized: false,
    };

    const element = id => document.getElementById(id);

    function localized(key, fallback, values) {
        let value = typeof t === 'function' ? t(key) : key;
        if (!value || value === key) value = fallback;
        Object.entries(values || {}).forEach(([name, replacement]) => {
            value = value.replaceAll(`{{${name}}}`, String(replacement));
        });
        return value;
    }

    function filters() {
        return {
            status: element('operational-issues-status')?.value || 'active',
            kind: element('operational-issues-kind')?.value || '',
            severity: element('operational-issues-severity')?.value || '',
            source: element('operational-issues-source')?.value.trim() || '',
        };
    }

    function queryURL() {
        const params = new URLSearchParams(filters());
        params.set('limit', String(state.limit));
        params.set('offset', String(state.offset));
        return '/api/operational-issues?' + params.toString();
    }

    async function loadOperationalIssues(resetOffset) {
        if (resetOffset === true) state.offset = 0;
        CardState.setLoading('card-operational-issues');
        try {
            const response = await fetch(queryURL(), { credentials: 'same-origin', cache: 'no-store' });
            if (!response.ok) throw new Error(String(response.status));
            const payload = await response.json();
            renderOperationalIssues(payload);
            CardState.setLoaded('card-operational-issues');
        } catch (_) {
            const card = element('card-operational-issues');
            if (card) card.setAttribute('data-state', 'error');
            notify('dashboard.operational_issues_load_failed', 'Operational issues could not be loaded.', 'error');
        }
    }

    function renderOperationalIssues(payload) {
        const items = Array.isArray(payload?.items) ? payload.items : [];
        state.total = Number(payload?.total || 0);
        const tbody = element('operational-issues-tbody');
        const empty = element('operational-issues-empty');
        if (!tbody || !empty) return;
        tbody.replaceChildren();
        empty.classList.toggle('is-hidden', items.length !== 0);
        items.forEach(item => tbody.appendChild(buildIssueRow(item)));

        const counts = payload?.status_counts || {};
        const countNode = element('operational-issues-counts');
        if (countNode) {
            countNode.textContent = localized(
                'dashboard.operational_issues_counts',
                '{{active}} active · {{archived}} archived',
                { active: Number(counts.active || 0), archived: Number(counts.archived || 0) },
            );
        }
        const meta = element('operational-issues-page-meta');
        if (meta) {
            const start = state.total === 0 ? 0 : state.offset + 1;
            const end = Math.min(state.offset + items.length, state.total);
            meta.textContent = localized(
                'dashboard.operational_issues_page',
                '{{start}}–{{end}} / {{total}}',
                { start, end, total: state.total },
            );
        }
        const prev = element('operational-issues-prev');
        const next = element('operational-issues-next');
        if (prev) prev.disabled = state.offset <= 0;
        if (next) next.disabled = state.offset + items.length >= state.total;
    }

    function buildIssueRow(item) {
        const row = document.createElement('tr');
        appendCell(row, formatDate(item.last_seen), '', 'dashboard.operational_issues_col_last_seen');

        const issueCell = document.createElement('td');
        issueCell.dataset.label = localized('dashboard.operational_issues_col_issue', 'Issue', {});
        const title = document.createElement('strong');
        title.textContent = item.title || '—';
        const detail = document.createElement('div');
        detail.className = 'operational-issue-detail';
        detail.textContent = item.detail || '';
        issueCell.append(title, detail);
        row.appendChild(issueCell);

        appendCell(row, item.source || '—', '', 'dashboard.operational_issues_col_source');
        appendCell(row, labelFor('kind', item.kind), '', 'dashboard.operational_issues_col_kind');
        appendCell(row, labelFor('severity', item.severity), 'operational-issue-severity operational-issue-severity-' + safeClass(item.severity), 'dashboard.operational_issues_col_severity');
        appendCell(row, String(Number(item.occurrences || 0)), '', 'dashboard.operational_issues_col_occurrences');
        appendCell(row, labelFor('status', item.status), '', 'dashboard.operational_issues_col_status');

        const actions = document.createElement('td');
        actions.className = 'operational-issue-actions';
        actions.dataset.label = localized('dashboard.operational_issues_col_actions', 'Actions', {});
        if (item.status === 'open' || item.status === 'in_progress') {
            actions.append(
                actionButton('dashboard.operational_issues_resolve', () => confirmItemAction(item, 'resolve')),
                actionButton('dashboard.operational_issues_archive', () => confirmItemAction(item, 'archive')),
            );
        } else {
            actions.textContent = '—';
        }
        row.appendChild(actions);
        return row;
    }

    function appendCell(row, value, className, labelKey) {
        const cell = document.createElement('td');
        if (className) cell.className = className;
        if (labelKey) cell.dataset.label = localized(labelKey, labelKey, {});
        cell.textContent = value;
        row.appendChild(cell);
    }

    function actionButton(key, onClick) {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'log-btn operational-issue-action';
        button.textContent = localized(key, key, {});
        button.addEventListener('click', onClick);
        return button;
    }

    function confirmItemAction(item, action) {
        const key = action === 'resolve'
            ? 'dashboard.operational_issues_confirm_resolve'
            : 'dashboard.operational_issues_confirm_archive';
        showConfirmation(
            localized(key, '{{title}}', { title: item.title || '—' }),
            () => mutateItem(item.id, action),
        );
    }

    async function mutateItem(id, action) {
        const body = action === 'resolve'
            ? { resolution: 'Resolved by administrator.' }
            : { reason: 'Archived by administrator.' };
        const response = await fetch(`/api/operational-issues/${encodeURIComponent(id)}/${action}`, {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
        if (!response.ok) throw new Error(String(response.status));
        notify('dashboard.operational_issues_updated', 'Operational issue updated.', 'success');
        await loadOperationalIssues(false);
    }

    async function previewStale() {
        try {
            const response = await fetch('/api/operational-issues/stale-preview', { credentials: 'same-origin', cache: 'no-store' });
            if (!response.ok) throw new Error(String(response.status));
            const payload = await response.json();
            const count = Number(payload?.count || 0);
            if (count === 0) {
                notify('dashboard.operational_issues_no_stale', 'No stale operational issues found.', 'info');
                return;
            }
            showConfirmation(
                localized(
                    'dashboard.operational_issues_confirm_stale',
                    'Archive {{count}} stale operational issues without deleting them?',
                    { count },
                ),
                archiveStale,
            );
        } catch (_) {
            notify('dashboard.operational_issues_action_failed', 'The action failed.', 'error');
        }
    }

    async function archiveStale() {
        const response = await fetch('/api/operational-issues/archive-stale', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ confirm: 'ARCHIVE_STALE_ISSUES' }),
        });
        if (!response.ok) throw new Error(String(response.status));
        const payload = await response.json();
        notify(
            'dashboard.operational_issues_archived_result',
            '{{count}} operational issues archived.',
            'success',
            { count: Number(payload?.archived || 0) },
        );
        state.offset = 0;
        await loadOperationalIssues(false);
    }

    function showConfirmation(message, action) {
        state.pendingAction = action;
        const panel = element('operational-issues-confirm');
        const text = element('operational-issues-confirm-text');
        if (text) text.textContent = message;
        panel?.classList.remove('is-hidden');
        element('operational-issues-confirm-apply')?.focus();
    }

    function hideConfirmation() {
        state.pendingAction = null;
        element('operational-issues-confirm')?.classList.add('is-hidden');
    }

    async function applyConfirmation() {
        const action = state.pendingAction;
        hideConfirmation();
        if (!action) return;
        try {
            await action();
        } catch (_) {
            notify('dashboard.operational_issues_action_failed', 'The action failed.', 'error');
        }
    }

    function labelFor(group, value) {
        const normalized = safeClass(value || 'unknown');
        let suffix = normalized;
        if (group === 'kind') {
            suffix = {
                runtime_failure: 'runtime',
                tool_failure: 'tool',
                review_required: 'review',
            }[normalized] || normalized;
        } else if (group === 'severity') {
            if (['critical', 'error', 'high'].includes(normalized)) suffix = 'error';
            else if (['warning', 'warn', 'medium'].includes(normalized)) suffix = 'warning';
            else if (['info', 'low'].includes(normalized)) suffix = 'info';
        }
        return localized(`dashboard.operational_issues_${group}_${suffix}`, value || '—', {});
    }

    function safeClass(value) {
        return String(value || '').toLowerCase().replace(/[^a-z0-9_-]/g, '');
    }

    function formatDate(value) {
        const date = new Date(value || '');
        return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString();
    }

    function notify(key, fallback, type, values) {
        if (typeof showToast === 'function') showToast(localized(key, fallback, values), type, 4500);
    }

    function initialize() {
        if (state.initialized) return;
        state.initialized = true;
        ['operational-issues-status', 'operational-issues-kind', 'operational-issues-severity'].forEach(id => {
            element(id)?.addEventListener('change', () => loadOperationalIssues(true));
        });
        element('operational-issues-source')?.addEventListener('change', () => loadOperationalIssues(true));
        element('operational-issues-refresh')?.addEventListener('click', () => loadOperationalIssues(false));
        element('operational-issues-archive-stale')?.addEventListener('click', previewStale);
        element('operational-issues-confirm-cancel')?.addEventListener('click', hideConfirmation);
        element('operational-issues-confirm-apply')?.addEventListener('click', applyConfirmation);
        element('operational-issues-prev')?.addEventListener('click', () => {
            state.offset = Math.max(0, state.offset - state.limit);
            loadOperationalIssues(false);
        });
        element('operational-issues-next')?.addEventListener('click', () => {
            if (state.offset + state.limit < state.total) {
                state.offset += state.limit;
                loadOperationalIssues(false);
            }
        });
    }

    window.loadOperationalIssues = async resetOffset => {
        initialize();
        return loadOperationalIssues(resetOffset);
    };
    document.addEventListener('DOMContentLoaded', initialize, { once: true });
})();
