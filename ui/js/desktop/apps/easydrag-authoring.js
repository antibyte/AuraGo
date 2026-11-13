// Goal-only AI proposals stay previews until one explicit, revision-fenced draft command.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const GOAL_MAX = 4000;
    let goalCounters = 0;

    // goalCounter counts the goal the way the server does (flows_ai_proposal.go: trimmed, in
    // code points, at most 4,000) and formats "count / max" in the UI language.
    function goalCounter(value, t, locale) {
        const count = Array.from(String(value || '').trim()).length;
        const opts = locale ? { locale } : undefined;
        return { count, over: count > GOAL_MAX, text: t('easydrag.ui.goal_counter', { count: ED.format.number(count, opts), max: ED.format.number(GOAL_MAX, opts) }) };
    }

    function applyProposal(model, doc, expectedVersion, label) {
        if (model.version !== expectedVersion || !doc || !Array.isArray(doc.nodes) || !Array.isArray(doc.edges)) return false;
        const checked = ED.model.create(doc, { types: new Map() });
        if (checked.repairs().droppedNodes) return false;
        const next = checked.toJSON();
        if (!ED.model.validGroups(next.groups, new Set(next.nodes.map(node => node.id)))) return false;
        model.change(label, rec => {
            model.doc.edges.forEach(edge => rec.removeEdge(edge.id));
            model.doc.nodes.forEach(node => rec.removeNode(node.id));
            next.nodes.forEach(node => rec.addNode(node));
            next.edges.forEach(edge => rec.addEdge(edge));
            rec.meta({ name: next.name, description: next.description || '', settings: next.settings || {}, groups: next.groups || [] });
        });
        return true;
    }

    function create(ed, home) {
        const core = ED.core, { t, esc } = ed;
        let dialog = null, controller = null, disposed = false, opening = false;

        async function open() {
            if (disposed || dialog || opening || ed.readonly || ed.runView || (home && !home.mayCreate())) return;
            opening = true;
            try {
            if (ed.flushPendingEdits && ed.flushPendingEdits() === false) return;
            if (ed.saver && !(await ed.saver.flush())) { ed.ctx.notify({ title: t('easydrag.ui.authoring_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' }); return; }
            if (disposed) return;
            let proposal = null;
            const model = home ? null : ed.model, version = model && model.version;
            const revision = home ? null : ed.saver ? ed.saver.revision : ed.flow.draft_revision;
            controller = new AbortController();
            const requestController = controller;
            const counterId = 'ed-goal-counter-' + (++goalCounters);
            // syncGoal is set once the dialog exists; an action re-applies it, because the
            // dialog's setBusy(false) enables every action button again.
            let syncGoal = () => {};
            // busy: a generate or apply request is running. The modal only disables its action
            // buttons, and typing in the goal re-applies syncGoal, so Generate must stay off here too.
            let busy = false;
            const current = () => !disposed && dialog && (home ? home.mayCreate() : !ed.runView && !ed.readonly && ed.model === model && model.version === version && Number(ed.saver ? ed.saver.revision : ed.flow.draft_revision) === Number(revision));
            dialog = core.modal(home ? home.root.closest('.ed-app') || home.root : ed.root, {
                title: t('easydrag.ui.authoring_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--authoring',
                body: '<p class="ed-callout">' + esc(t('easydrag.ui.authoring_notice')) + '</p><label class="ed-label">' + esc(t('easydrag.ui.authoring_goal')) + '<textarea class="ed-input" rows="5" aria-describedby="' + counterId + '" data-ed-authoring-goal></textarea></label><p class="ed-goal-counter" id="' + counterId + '" data-ed-goal-counter></p><p class="ed-error" role="alert" hidden></p><section data-ed-authoring-preview hidden></section>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'generate', label: t('easydrag.ui.authoring_generate'), primary: true, icon: 'sparkles' }, { id: 'apply', label: t('easydrag.ui.authoring_apply'), primary: true }],
                onAction: (action, view) => {
                    if (action !== 'cancel') { if (busy) return false; busy = true; }
                    return (async () => {
                    if (action === 'cancel') return true;
                    const error = view.body.querySelector('.ed-error');
                    error.hidden = true;
                    if (!current() || (ed.flushPendingEdits && ed.flushPendingEdits() === false) || (model && model.version !== version)) { error.hidden = false; error.textContent = t('easydrag.ui.authoring_stale'); return false; }
                    if (action === 'generate') {
                        const goal = view.body.querySelector('[data-ed-authoring-goal]').value.trim();
                        if (!goal || Array.from(goal).length > 4000) { error.hidden = false; error.textContent = t('easydrag.ui.authoring_invalid'); return false; }
                        try {
                            const result = await (home ? ed.api.proposeNew({ goal }, requestController.signal) : ed.api.propose(ed.flow.id, { goal, base_revision: revision }, requestController.signal));
                            if (!current() || requestController.signal.aborted || view.body.querySelector('[data-ed-authoring-goal]').value.trim() !== goal) { if (dialog) { error.hidden = false; error.textContent = t('easydrag.ui.authoring_stale'); } return false; }
                            if ((!home && Number(result.base_revision) !== Number(revision)) || !result.doc || !Array.isArray(result.doc.nodes) || !result.doc.nodes.length || (result.issues || []).some(issue => issue.severity === 'error')) { error.hidden = false; error.textContent = t('easydrag.ui.authoring_empty'); return false; }
                            proposal = result;
                            const preview = view.body.querySelector('[data-ed-authoring-preview]');
                            preview.hidden = false;
                            preview.innerHTML = markup(result);
                            view.el.querySelector('[data-ed-action="apply"]').hidden = false;
                            view.el.querySelector('[data-ed-action="generate"]').hidden = true;
                            return false;
                        } catch (err) { if (dialog && !requestController.signal.aborted) { error.hidden = false; error.textContent = core.errorText(t, err); } return false; }
                    }
                    if (action !== 'apply' || !proposal) return false;
                    if (home) {
                        try {
                            const created = await ed.api.create({ import: proposal.doc });
                            if (current()) ed.openFlow(created.flow.id);
                            return true;
                        } catch (err) { if (current()) { error.hidden = false; error.textContent = core.errorText(t, err); } return false; }
                    }
                    if (!applyProposal(model, proposal.doc, version, t('easydrag.ui.authoring_apply'))) { error.hidden = false; error.textContent = t('easydrag.ui.authoring_stale'); return false; }
                    ed.bus.emit('proposal-applied');
                    ed.ctx.notify({ title: t('easydrag.ui.authoring_title'), message: t('easydrag.ui.authoring_applied') });
                    return true;
                    })().finally(() => { if (action !== 'cancel') busy = false; setTimeout(syncGoal, 0); });
                }
            });
            const owned = dialog;
            owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
            const goalInput = owned.body.querySelector('[data-ed-authoring-goal]');
            const counter = owned.body.querySelector('[data-ed-goal-counter]');
            const locale = (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || undefined;
            // Above the limit the counter marks a state (warn ink) and Generate is disabled; the
            // length check in onAction stays as a backstop.
            syncGoal = () => {
                const state = goalCounter(goalInput.value, t, locale);
                counter.textContent = state.text;
                if (state.over) counter.classList.add('is-over'); else counter.classList.remove('is-over');
                owned.el.querySelector('[data-ed-action="generate"]').disabled = state.over || busy;
            };
            syncGoal();
            goalInput.addEventListener('input', () => {
                proposal = null;
                owned.el.querySelector('[data-ed-action="apply"]').hidden = true;
                owned.el.querySelector('[data-ed-action="generate"]').hidden = false;
                owned.body.querySelector('[data-ed-authoring-preview]').hidden = true;
                syncGoal();
            });
            owned.done.then(() => { requestController.abort(); if (dialog === owned) dialog = null; if (controller === requestController) controller = null; });
            } finally { opening = false; }
        }

        function markup(result) {
            const nodes = result.doc.nodes;
            const indices = new Map(nodes.map((node, index) => [node.id, index]));
            const info = type => ed.catalog.types.get(type);
            const preview = ED.home.previewSVG(nodes.map(node => ({ x: (node.position || {}).x || 0, y: (node.position || {}).y || 0, category: core.catOf(info(node.type)) })), esc, (result.doc.edges || []).map(edge => [indices.get(edge.source.node), indices.get(edge.target.node)]));
            return '<p class="ed-hint">' + esc(t('easydrag.ui.authoring_review')) + '</p><div class="ed-flow-preview" aria-hidden="true">' + preview + '</div><ol class="ed-proposal-steps">' + nodes.slice(0, 500).map(node => '<li><strong>' + esc(node.label || node.key || node.type) + '</strong> <span class="ed-muted">' + esc((info(node.type) || {}).label || node.type) + '</span></li>').join('') + '</ol>' +
                ((result.issues || []).length ? '<ul class="ed-issues">' + result.issues.slice(0, 500).map(issue => '<li>' + esc(core.issueText(t, issue)) + '</li>').join('') + '</ul>' : '') +
                ((result.effects || []).length ? '<h4>' + esc(t('easydrag.ui.recipe_effects')) + '</h4><ul class="ed-effects">' + result.effects.map(effect => '<li>' + esc(core.tr(t, 'easydrag.ui.effect_' + effect.effect, effect.effect)) + '</li>').join('') + '</ul>' : '') +
                (result.diff ? '<details class="ed-details"><summary>' + esc(t('easydrag.ui.publish_changes')) + '</summary><pre class="ed-code ed-output-json">' + esc(JSON.stringify(result.diff.details || [], null, 2)) + '</pre></details>' : '');
        }
        return { open, dispose() { disposed = true; if (controller) controller.abort(); if (dialog) dialog.close(); dialog = null; } };
    }
    ED.authoring = { create, createNew: (app, root, mayCreate) => create(app, { root, mayCreate }), applyProposal, goalCounter };
})();
