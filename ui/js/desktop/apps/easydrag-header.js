// EasyDrag editor header and footer: the flow name, the pill (easydrag-pill.js: the state, its popover),
// the action buttons, the save state, the issues count and the last run. easydrag-editor.js
// creates it once per editor and keeps the run view (renderBanner), the commands and the bus wiring.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const SAVE_ICONS = { saved: 'check', dirty: 'pencil', saving: 'refresh', invalid: 'alert', offline: 'alert', failed: 'alert', conflict: 'alert' };

    // create(host) builds the header renderers of one editor. host: {ed, el, isBlock, canvas, runs,
    // palette, publish, nameInput, bag, renderBanner(), isDisposed(), lastRecord()}; t, esc and ctx come from ed.
    function create(host) {
        const core = ED.core;
        const { ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner } = host;
        const { t, esc, ctx } = ed;

        // The pill (the flow's state in one text, and its popover with Active, the next run, Run now and Runs) is easydrag-pill.js.
        const pill = ED.pill.create({ ed, el, runs, publish });

        // paintActions draws the action buttons from the registry: the title (label and shortcut), and for an unavailable button
        // aria-disabled with its reason in the title and in aria-description. A button never says "disabled": a keyboard user can
        // still reach it and read why, and the click handler (and ED.actions.run) refuse it.
        function paintActions() {
            const c = { host: ed };
            [['test', 'flow.test'], ['cancel', 'flow.stop'], ['publish', 'flow.publish']].forEach(([cmd, id]) => {
                const button = el.querySelector('[data-ed-cmd="' + cmd + '"]');
                if (button) ED.actions.paint(button, id, c);
            });
            // The pill's popover shows the same reasons (a dialog or a menu opening or closing changes them).
            pill.paint();
        }

        function renderHeader() {
            // A save answered after dispose must not point the window context at this flow again.
            if (host.isDisposed()) return;
            renderBanner();
            if (document.activeElement !== nameInput) nameInput.value = ed.model.doc.name || ''; if (nameInput.edLimit) nameInput.edLimit.update();
            nameInput.readOnly = !!(ed.readonly || ed.runView);
            syncSaveChip();
            const ro = !!(ed.readonly || ed.runView);
            const running = runs.isRunning();
            const btn = cmd => el.querySelector('[data-ed-cmd="' + cmd + '"]');
            if (isBlock) {
                btn('test').querySelector('span').textContent = ED.actions.label('flow.test', { host: ed });
                el.querySelector('[data-ed-cmd="templates"]').hidden = true;
                nameInput.setAttribute('aria-label', t('easydrag.ui.block_name'));
            }
            btn('test').classList.toggle('is-busy', running);
            btn('test').classList.toggle('ed-btn--primary', !ed.flow.live && !ro);
            btn('cancel').hidden = !running;
            btn('publish').classList.toggle('has-changes', (!ed.flow.live || ED.pill.changed(ed) || !!ed.publishIncomplete) && !ro);
            btn('publish').classList.toggle('ed-btn--primary', !!ed.flow.live && !ro);
            paintActions();
            btn('palette').setAttribute('aria-pressed', String(palette.isOpen()));
            // The run view keeps the palette hidden: a stored run takes no new steps.
            btn('palette').setAttribute('aria-disabled', String(!!ed.runView));
            // flow_id/run_id from a notification are cleared, so a later render does not reopen
            // that run, or a run of another flow after openFlow switched flows.
            const followed = ED.durable && ED.durable.has(ed.run) && (ED.runs.isActive(ed.run.status) || ED.durable.info(ed.run)?.cancel_available) ? ed.run.id : null;
            ctx.updateWindowContext && ctx.updateWindowContext(ed.windowId, isBlock ? { blockId: ed.flow.id, flowId: null, flow_id: null, run_id: null } : { blockId: null, flowId: ed.flow.id, flow_id: null, run_id: followed });
        }

        // renderSaveState shows the saver's state. A failed or offline save names its error (an
        // offline one without an error keeps the retry hint). A failed save is not retried by
        // itself, so its chip is a button that tries again (saveNow).
        function renderSaveState(state) {
            const s = state || ed.saver.state;
            const node = el.querySelector('[data-ed-save]');
            const err = ed.saver && ed.saver.error;
            const title = s === 'failed' || (s === 'offline' && err) ? core.errorText(t, err) : s === 'offline' ? t('easydrag.ui.save_offline_hint') : '';
            // .ed-foot-text: narrow footers show the icon only, the text stays for screen readers.
            // A failed save, and an offline one that has an error, show the reason, not only as tooltip.
            const reason = s === 'failed' || (s === 'offline' && err) ? core.errorText(t, err) : '';
            const label = core.icon(SAVE_ICONS[s] || 'check') + '<span class="ed-foot-text">' + esc(t('easydrag.ui.save_' + s)) + '</span>' +
                (reason ? '<span class="ed-save-reason">' + esc(reason) + '</span>' : '');
            const hadFocus = node.contains(document.activeElement);
            node.className = 'ed-foot-item ed-save ed-save--' + s;
            node.innerHTML = s === 'failed'
                ? '<button type="button" class="ed-save-retry" data-ed-cmd="save" title="' + esc(title) + '">' + label + '<span class="ed-save-action">' + esc(t('easydrag.ui.retry')) + '</span></button>'
                : label;
            node.title = s === 'failed' ? '' : title;
            if (!host.isDisposed()) pill.paint();
            // Each state replaces the retry button: keyboard focus moves to the new one or, once the
            // save went through, to the canvas.
            if (hadFocus && !host.isDisposed()) (node.querySelector('[data-ed-cmd="save"]') || canvas.el).focus({ preventScroll: true });
            syncSaveChip();
        }

        // syncSaveChip hides "Saved" in the run view (nothing there can be saved); any other save
        // state, a failure above all, stays visible there.
        function syncSaveChip() {
            const node = el.querySelector('[data-ed-save]');
            if (node && ed.saver) {
                node.hidden = !!ed.runView && ed.saver.state === 'saved';
                const text = node.querySelector('.ed-foot-text'), state = t('easydrag.ui.save_' + ed.saver.state);
                const label = ed.runView ? t('easydrag.ui.save_draft_status', { status: state }) : state;
                if (text && text.textContent !== label) text.textContent = label;
            }
        }

        function renderIssues() {
            const list = ed.issues || [];
            const errors = list.filter(i => i.severity === 'error').length;
            const warnings = list.length - errors;
            const node = el.querySelector('[data-ed-issues]');
            node.parentElement.className = 'ed-foot-item' + (errors ? ' has-errors' : warnings ? ' has-warnings' : ' is-clean');
            node.parentElement.title = list.length ? t('easydrag.ui.issues_count', { errors, warnings }) : '';
            node.parentElement.setAttribute('aria-label', list.length ? node.parentElement.title : t('easydrag.ui.issues_none_short'));
            node.innerHTML = list.length
                ? (errors ? '<span class="ed-count-badge ed-count-badge--error">' + core.icon('alert') + errors + '</span>' : '') +
                  (warnings ? '<span class="ed-count-badge ed-count-badge--warn">' + core.icon('info') + warnings + '</span>' : '')
                : core.icon('check') + '<span class="ed-foot-text">' + esc(t('easydrag.ui.issues_none_short')) + '</span>';
            // The run view shows a stored run, not the draft the issues describe. The foot button
            // stays inert there; leaving the view renders it again.
            const button = node.closest('[data-ed-cmd]') || node;
            if (ed.runView) button.setAttribute('aria-disabled', 'true');
            else button.removeAttribute('aria-disabled');
        }

        function renderLastRun() {
            const r = ed.run && ed.run.record ? ed.run.record : (ed.run ? { status: ed.run.status, started_at: null } : host.lastRecord());
            const dot = el.querySelector('[data-ed-last-dot]');
            const text = el.querySelector('[data-ed-last-run]');
            const mark = r ? ED.status.disc(ED.testing.statusOf(ed, ed.run || r), 14, { t, hidden: true }) : ''; if (dot.dataset.mark !== mark) { dot.dataset.mark = mark; dot.innerHTML = mark; }
            const failed = r && r.status === 'error' && ed.run && ed.run.steps ? Array.from(ed.run.steps.values()).find(s => s.status === 'error') : null;
            const failedNode = failed && ed.model.node(failed.node_id);
            const head = failedNode ? t('easydrag.ui.run_failed_at', { node: failedNode.label || failedNode.type }) : r ? ED.testing.statusLabel(t, ed, ed.run || r) : '';
            text.textContent = r
                ? head + (r.started_at ? ' · ' + core.fmt.relative(r.started_at) : '') + (r.duration_ms ? ' · ' + core.fmt.duration(r.duration_ms) : '')
                : t(isBlock ? 'easydrag.ui.block_not_simulated' : 'easydrag.ui.home_never_ran');
            // Narrow footers cut the text: the button keeps the whole of it as its name and tooltip.
            text.parentElement.setAttribute('aria-label', text.textContent);
            text.parentElement.title = text.textContent;
        }

        // The facts of the server (next run, tested, changes) arrive after the first paint: the pill follows them.
        host.bag.add(ed.bus.on('flow-facts', renderHeader));

        return { renderHeader, renderSaveState, renderIssues, renderLastRun, paintActions, dispose() { pill.dispose(); } };
    }

    ED.header = { create };
})();
