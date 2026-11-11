// EasyDrag credential purge: the start-page hint (and, from the purge task on, the guided dialog).
// The scan answers only counts and names, never a credential value.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    let bound = null;            // the app whose api and t() the module uses; set by mountHint
    const hints = new Set();     // the mounted hint handles

    // needsAttention: plaintext remains anywhere the purge can reach (encrypted backups do not count).
    function needsAttention(report) {
        if (!report || typeof report !== 'object') return false;
        const list = value => Array.isArray(value) && value.length > 0;
        const count = value => Number.isFinite(value) && value > 0;
        return list(report.drafts) || list(report.live) || count(report.versions_purgeable) || count(report.versions_pinned) ||
            count(report.runs) || count(report.steps) || count(report.test_rows) || list(report.snapshots) || list(report.leftovers);
    }

    function hintMarkup(t, report) {
        if (!needsAttention(report)) return '';
        const esc = ED.core.esc;
        return '<h3>' + esc(t('easydrag.ui.cred_hint_title')) + '</h3><p>' + esc(t('easydrag.ui.cred_hint_text')) + '</p>' +
            '<button type="button" class="ed-btn" data-ed-credentials-open>' + esc(t('easydrag.ui.cred_hint_action')) + '</button>';
    }

    // mountHint puts the hint card into the start page. It asks the scan endpoint, which is admin only:
    // a 401, 403 or any failure shows nothing.
    function mountHint(container, app) {
        if (app) bound = app;
        const card = ED.core.el('<div class="ed-home-stale" role="status" data-ed-credentials-hint hidden></div>');
        const anchor = container.querySelector('.ed-home-toolbar');
        if (anchor) container.insertBefore(card, anchor); else container.appendChild(card);
        let disposed = false, sequence = 0;
        async function refresh() {
            const mine = ++sequence;
            let report = null;
            try { report = (await bound.api.credentialScan(false)).report; } catch (err) { report = null; }
            if (disposed || mine !== sequence) return;
            const markup = hintMarkup(bound.t, report);
            card.innerHTML = markup;
            card.hidden = !markup;
        }
        card.addEventListener('click', event => {
            if (event.target.closest('[data-ed-credentials-open]') && typeof ED.credentialsPurge.open === 'function') void ED.credentialsPurge.open();
        });
        const handle = { refresh, dispose() { disposed = true; hints.delete(handle); card.remove(); } };
        hints.add(handle);
        void refresh();
        return handle;
    }

    // offer is what the editor calls after a move and a publish: the hint refreshes at once.
    function offer(flowId) {
        hints.forEach(handle => { void handle.refresh(); });
        return flowId;
    }

    ED.credentialsPurge = { needsAttention, hintMarkup, mountHint, offer };
})();
