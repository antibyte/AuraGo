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

    const STEP_KEYS = ['backup', 'versions', 'redact', 'compact', 'snapshots', 'verify'];
    let dialog = null;

    function sizeText(bytes) {
        const n = Number(bytes) || 0;
        return n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB';
    }
    function dateText(value) { const d = new Date(value); return Number.isFinite(d.getTime()) ? d.toLocaleDateString() : ''; }

    // dialogMarkup says in plain words what will be deleted, blacked out, and what stays and why.
    function dialogMarkup(t, report) {
        const esc = ED.core.esc;
        const flowItem = (ref, key) => '<li><button type="button" class="ed-btn" data-ed-credentials-flow="' + esc(ref.flow_id) + '">' + esc(ref.name) + '</button> ' + esc(t(key)) + '</li>';
        const drafts = report.drafts || [], live = (report.live || []).filter(ref => !drafts.some(d => d.flow_id === ref.flow_id));
        let html = '<p>' + esc(t('easydrag.ui.cred_purge_versions', { count: report.versions_purgeable || 0 })) + '</p>' +
            '<p>' + esc(t('easydrag.ui.cred_purge_rows', { runs: report.runs || 0, rows: (report.steps || 0) + (report.test_rows || 0) })) + '</p>';
        if (report.versions_pinned > 0) html += '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_pinned', { count: report.versions_pinned })) + '</p>';
        const stays = drafts.map(ref => flowItem(ref, 'easydrag.ui.cred_purge_move_first')).concat(live.map(ref => flowItem(ref, 'easydrag.ui.cred_purge_republish')));
        if (stays.length) html += '<ul>' + stays.join('') + '</ul>';
        const snapshots = report.snapshots || [];
        if (snapshots.length) {
            html += '<h3>' + esc(t('easydrag.ui.cred_purge_snapshots')) + '</h3><ul>' + snapshots.map(s => '<li><label><input type="checkbox" checked data-ed-credentials-snapshot="' + esc(s.name) + '"> ' +
                esc(t('easydrag.ui.cred_purge_snapshot_line', { name: s.name, date: dateText(s.mod_time), size: sizeText(s.bytes) })) + '</label></li>').join('') + '</ul>';
        }
        return html + '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_outside')) + '</p><div data-ed-credentials-status role="status" aria-live="polite"></div>';
    }

    function stateMarkup(t, state) {
        const esc = ED.core.esc;
        const step = key => ED.core.tr(t, 'easydrag.ui.cred_step_' + key, key);
        let html = '';
        if (state.status === 'running') html = '<p>' + esc(t('easydrag.ui.cred_purge_step', { step: step(state.step) })) + '</p>';
        else if (state.status === 'failed') html = '<p class="ed-error">' + esc(t('easydrag.ui.cred_purge_failed', { step: step(state.failed_step) })) + '</p>';
        else if (state.status === 'done' && state.clean) html = '<p>' + esc(t('easydrag.ui.cred_purge_clean')) + '</p>';
        else if (state.status === 'done') html = '<p>' + esc(t('easydrag.ui.cred_purge_left')) + '</p><ul>' + (state.remaining || []).map(r => '<li>' + esc(r.where) + '</li>').join('') + '</ul>';
        if (state.backup && state.status !== 'running') html += '<p class="ed-hint">' + esc(t('easydrag.ui.cred_purge_backup', { name: state.backup })) + '</p>';
        return html;
    }

    // open shows the dialog with a fresh scan. With onlyIfNeeded it stays closed when nothing needs cleaning
    // (the editor offers it after every publish that followed a move).
    async function open(options) {
        const app = bound;
        if (!app || dialog) return;
        const t = app.t, core = ED.core;
        let report;
        try { report = (await app.api.credentialScan(true)).report; } catch (err) { return; }
        if (options && options.onlyIfNeeded && !needsAttention(report)) return;
        if (dialog) return;
        const host = document.querySelector('.ed-app') || document.body;
        let closed = false;
        const owned = dialog = core.modal(host, {
            title: t('easydrag.ui.cred_purge_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--credentials', body: dialogMarkup(t, report),
            actions: [{ id: 'close', label: t('easydrag.ui.close') }, { id: 'purge', label: t('easydrag.ui.cred_purge_go'), primary: true }],
            onAction(id, instance) {
                if (id === 'close') { instance.close('close'); return false; }
                if (id === 'purge') return start().then(() => false); // the modal keeps the buttons busy while it runs
                return false;
            }
        });
        owned.done.then(() => { closed = true; dialog = null; offer(); });
        const status = () => owned.body.querySelector('[data-ed-credentials-status]');
        owned.body.addEventListener('click', event => {
            const flow = event.target.closest('[data-ed-credentials-flow]');
            if (flow) { const id = flow.dataset.edCredentialsFlow; owned.close('flow'); app.openFlow(id); }
        });
        async function start() {
            const names = [...owned.body.querySelectorAll('[data-ed-credentials-snapshot]')].filter(box => box.checked).map(box => box.dataset.edCredentialsSnapshot);
            owned.setBusy(true);
            try { await app.api.credentialPurgeStart(names); } catch (err) {
                owned.setBusy(false);
                status().innerHTML = '<p class="ed-error">' + core.esc(core.errorText(t, err)) + '</p>';
                return;
            }
            await follow();
        }
        async function follow() {
            while (!closed) {
                let state;
                try { state = (await app.api.credentialPurgeState()).state; } catch (err) { state = null; }
                if (closed) return;
                if (state) {
                    status().innerHTML = stateMarkup(t, state);
                    if (state.status !== 'running') { owned.setBusy(false); return; }
                }
                await new Promise(resolve => setTimeout(resolve, 1000));
            }
        }
    }

    // offer is what the editor calls after a move and a publish: the hints refresh and, when something
    // can be cleaned, the dialog opens.
    function offer(flowId) {
        hints.forEach(handle => { void handle.refresh(); });
        if (flowId !== undefined && bound) void open({ onlyIfNeeded: true });
        return flowId;
    }

    ED.credentialsPurge = { needsAttention, hintMarkup, dialogMarkup, stateMarkup, mountHint, open, offer };
})();
