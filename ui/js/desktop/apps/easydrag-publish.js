// EasyDrag publishing: live hints (publish-rule validation of the draft), issues panel,
// publish dialog (problems, effects, untrusted data, change summary) and the active switch.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const EFFECT_ICONS = { sends_message: 'brand-telegram', writes_files: 'file-pencil', controls_devices: 'home', runs_code: 'api', deletes: 'trash', system_change: 'settings' };

    function create(ed) {
        const core = ED.core;
        const { t, esc } = ed;
        const bag = core.bag();
        let popover = null;
        let seq = 0;

        const refreshIssues = core.debounce(async () => {
            const mine = ++seq;
            try {
                const res = await ed.api.validate(ed.model.toJSON(), 'publish');
                if (mine !== seq) return;
                ed.issues = res.issues || [];
                ed.bus.emit('issues', ed.issues);
            } catch (err) { /* hints are best effort */ }
        }, 700);

        function nodeLabel(id) {
            const n = ed.model.node(id);
            return n ? (n.label || n.type) : '';
        }

        function issuesMarkup(list) {
            const sorted = list.slice().sort((a, b) => (a.severity === 'error' ? 0 : 1) - (b.severity === 'error' ? 0 : 1));
            return '<ul class="ed-issues">' + sorted.map((is, i) =>
                '<li><button type="button" class="ed-issue ed-issue--' + esc(is.severity) + '" data-ed-issue="' + i + '">' + core.icon(is.severity === 'error' ? 'alert' : 'info') +
                '<span class="ed-issue-text">' + (is.node_id ? '<strong>' + esc(nodeLabel(is.node_id)) + '</strong> ' : '') + esc(core.issueText(t, is)) + '</span></button></li>').join('') + '</ul>';
        }

        function focusIssue(is) {
            if (!is || !is.node_id) return;
            ed.bus.emit('focus-node', is.node_id);
            ed.bus.emit('open-detail', { nodeId: is.node_id, param: is.param ? is.param.split(/[.[]/)[0] : undefined });
        }

        function openIssues(anchor) {
            closeIssues();
            const list = ed.issues || [];
            popover = core.el('<div class="ed-popover ed-issues-popover" role="dialog" aria-label="' + esc(t('easydrag.ui.issues_title')) + '"><header><h3>' + esc(t('easydrag.ui.issues_title')) + '</h3></header>' +
                (list.length ? issuesMarkup(list) : '<p class="ed-hint">' + esc(t('easydrag.ui.issues_none')) + '</p>') + '</div>');
            ed.root.appendChild(popover);
            const host = ed.root.getBoundingClientRect();
            const rect = anchor.getBoundingClientRect();
            popover.style.right = Math.max(8, host.right - rect.right) + 'px';
            popover.style.bottom = Math.max(8, host.bottom - rect.top + 8) + 'px';
            const sorted = list.slice().sort((a, b) => (a.severity === 'error' ? 0 : 1) - (b.severity === 'error' ? 0 : 1));
            popover.addEventListener('click', (event) => {
                const b = event.target.closest('[data-ed-issue]');
                if (!b) return;
                closeIssues();
                focusIssue(sorted[Number(b.dataset.edIssue)]);
            });
            const first = popover.querySelector('button');
            if (first) first.focus();
            setTimeout(() => document.addEventListener('pointerdown', outside, true), 0);
        }

        function outside(event) { if (popover && !popover.contains(event.target)) closeIssues(); }

        function closeIssues() {
            document.removeEventListener('pointerdown', outside, true);
            if (popover) { popover.remove(); popover = null; }
        }

        function diffText(diff) {
            if (!diff || diff.first_publish) return t('easydrag.ui.diff_first');
            const parts = [];
            if (diff.added_nodes) parts.push(t('easydrag.ui.diff_added', { count: diff.added_nodes }));
            if (diff.changed_nodes) parts.push(t('easydrag.ui.diff_changed', { count: diff.changed_nodes }));
            if (diff.removed_nodes) parts.push(t('easydrag.ui.diff_removed', { count: diff.removed_nodes }));
            if (diff.changed_edges) parts.push(t('easydrag.ui.diff_edges', { count: diff.changed_edges }));
            return parts.length ? parts.join(' · ') : t('easydrag.ui.diff_none');
        }

        async function openDialog() {
            if (ed.readonly || ed.runView) return;
            if (ed.saver && !(await ed.saver.flush())) {
                ed.ctx.notify({ title: t('easydrag.ui.publish'), message: t('easydrag.ui.publish_unsaved'), type: 'error' });
                return;
            }
            let preview;
            try { preview = await ed.api.preview(ed.flow.id); } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.publish'), message: core.errorText(t, err), type: 'error' });
                return;
            }
            ed.issues = preview.issues || [];
            ed.bus.emit('issues', ed.issues);
            const errors = ed.issues.filter(is => is.severity === 'error');
            const untrusted = ed.issues.filter(is => is.code === 'UNTRUSTED_DATA_TO_SINK');
            const warnings = ed.issues.filter(is => is.severity !== 'error' && is.code !== 'UNTRUSTED_DATA_TO_SINK');
            const enabled = !!ed.flowEnabled;
            let body = '';
            if (errors.length) {
                body += '<div class="ed-callout ed-callout--error">' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.publish_blocked', { count: errors.length })) + '</strong>' + issuesMarkup(errors) + '</div></div>';
            } else {
                body += '<p class="ed-publish-lead">' + esc(t('easydrag.ui.publish_lead')) + '</p>';
            }
            const fx = preview.effects || [];
            if (fx.length) {
                body += '<h4 class="ed-section-title">' + esc(t('easydrag.ui.publish_effects')) + '</h4><ul class="ed-effects">' + fx.map(e =>
                    '<li>' + core.icon(EFFECT_ICONS[e.effect] || 'bolt') + '<span>' + esc(core.tr(t, 'easydrag.ui.effect_' + e.effect, e.effect)) + '</span><span class="ed-muted">' +
                    esc((e.node_ids || []).map(nodeLabel).filter(Boolean).join(', ')) + '</span></li>').join('') + '</ul>';
            }
            if (untrusted.length) {
                body += '<div class="ed-callout ed-callout--warn">' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.publish_untrusted')) + '</strong>' + issuesMarkup(untrusted) + '</div></div>';
            }
            if (warnings.length) body += '<details class="ed-details"><summary>' + esc(t('easydrag.ui.publish_warnings', { count: warnings.length })) + '</summary>' + issuesMarkup(warnings) + '</details>';
            body += '<p class="ed-diff">' + core.icon('history') + '<span>' + esc(diffText(preview.diff)) + '</span></p>';
            if (!enabled && !errors.length) body += '<label class="ed-check"><input type="checkbox" data-ed-activate checked> ' + esc(t('easydrag.ui.publish_activate')) + '</label>';
            const dialog = core.modal(ed.root, {
                title: t('easydrag.ui.publish_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--publish', body,
                actions: errors.length
                    ? [{ id: 'close-dialog', label: t('easydrag.ui.close') }]
                    : [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'publish', label: t('easydrag.ui.publish'), primary: true, icon: 'rocket' }],
                onAction: async (action, d) => {
                    if (action !== 'publish') return true;
                    const activate = d.body.querySelector('[data-ed-activate]');
                    return publish(!!(activate && activate.checked));
                }
            });
            dialog.el.addEventListener('click', (event) => {
                const b = event.target.closest('[data-ed-issue]');
                if (!b) return;
                const list = b.closest('.ed-callout--error') ? errors : b.closest('.ed-callout--warn') ? untrusted : warnings;
                const sorted = list.slice().sort((a, c) => (a.severity === 'error' ? 0 : 1) - (c.severity === 'error' ? 0 : 1));
                dialog.close(null);
                focusIssue(sorted[Number(b.dataset.edIssue)]);
            });
            return dialog;
        }

        async function publish(activate) {
            try {
                const res = await ed.api.publish(ed.flow.id, ed.saver ? ed.saver.revision : ed.flow.draft_revision);
                ed.flow = res.flow;
                if (activate) await setActive(true, true);
                ed.bus.emit('published', ed.flow);
                ed.ctx.notify({ title: t('easydrag.ui.publish'), message: t('easydrag.ui.publish_done', { name: ed.model.doc.name }) });
                return true;
            } catch (err) {
                if (core.errorCode(err) === 'FLOW_INVALID') {
                    ed.issues = (err.body && err.body.issues) || [];
                    ed.bus.emit('issues', ed.issues);
                }
                ed.ctx.notify({ title: t('easydrag.ui.publish'), message: core.errorText(t, err), type: 'error' });
                return false;
            }
        }

        async function setActive(on, quiet) {
            try {
                await ed.api.setEnabled(ed.flow.id, on);
                ed.flowEnabled = on;
                ed.bus.emit('enabled', on);
                if (!quiet) ed.ctx.notify({ title: ed.model.doc.name, message: on ? t('easydrag.ui.active_on') : t('easydrag.ui.active_off') });
                return true;
            } catch (err) {
                ed.bus.emit('enabled', ed.flowEnabled);
                if (core.errorCode(err) === 'FLOW_NOT_PUBLISHED') {
                    const ok = await ed.ctx.confirmDialog(t('easydrag.ui.active_needs_publish_title'), t('easydrag.ui.active_needs_publish'));
                    if (ok) openDialog();
                    return false;
                }
                ed.ctx.notify({ title: ed.model.doc.name, message: core.errorText(t, err), type: 'error' });
                return false;
            }
        }

        bag.add(() => { refreshIssues.cancel(); closeIssues(); });

        return { refreshIssues, openIssues, closeIssues, openDialog, publish, setActive, focusIssue, dispose() { bag.dispose(); } };
    }

    ED.publish = { create };
})();
