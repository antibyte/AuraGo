// EasyDrag editor header and footer: the flow name, the state, revision, live and timer chips,
// the action buttons, the save state, the issues count and the last run. easydrag-editor.js
// creates it once per editor and keeps the run view (renderBanner), the commands and the bus wiring.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const SAVE_ICONS = { saved: 'check', dirty: 'pencil', saving: 'refresh', invalid: 'alert', offline: 'alert', failed: 'alert', conflict: 'alert' };

    // create(host) builds the header renderers of one editor. host: {ed, el, isBlock, canvas, runs,
    // palette, publish, nameInput, renderBanner(), isDisposed(), lastRecord()}; t, esc and ctx come from ed.
    function create(host) {
        const core = ED.core;
        const { ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner } = host;
        const { t, esc, ctx } = ed;

        // stateOf names the flow's state with the words of Mission Control and the missions page:
        // state_draft reads "Not published yet" (a flow never published) and state_inactive
        // "Paused" (published, switched off). "Draft" stays the word for the editable version.
        function stateOf() {
            // A partial publish is live, but Mission Control or the timers were not updated: the chip
            // says why and Publish stays offered (publishing again finishes it).
            if (ed.publishIncomplete && !ed.runView) return { cls: 'warn', key: 'easydrag.ui.state_publish_incomplete', title: publish.partialText(ed.publishIncomplete) };
            if (ed.runView) return { cls: 'muted', key: 'easydrag.ui.state_run_view' };
            if (!ed.flow.live) return { cls: 'muted', key: 'easydrag.ui.state_draft' };
            if (ed.flow.draft_revision !== ed.flow.published_draft_revision) return { cls: 'accent', key: 'easydrag.ui.state_changes' };
            return { cls: 'ok', key: 'easydrag.ui.state_published' };
        }

        // runNowState: Run now starts the published version, and only while the flow is switched on
        // (a paused flow is refused with FLOW_DISABLED, as Mission Control refuses a paused mission).
        // A paused flow's item says to switch it on first.
        function runNowState() {
            const ro = !!(ed.readonly || ed.runView);
            const paused = !!ed.flow.live && !ed.flowEnabled;
            return { disabled: ro || !ed.flow.live || paused, hint: paused && !ro ? t('easydrag.ui.error_flow_disabled') : '' };
        }

        // revisionText is the persistent draft-versus-live line. Partial publish stays on the state chip.
        function revisionText() {
            // The run view's chip says "Past run" already; the line stays empty and hidden there.
            if (ed.runView) return '';
            if (!ed.flow.live) return '';
            const liveN = Number(ed.flow.live_revision) || 0;
            if (ed.flow.draft_revision !== ed.flow.published_draft_revision) return t('easydrag.ui.revision_ahead', { n: liveN });
            return t('easydrag.ui.revision_live', { n: liveN });
        }

        function runNowTitle() {
            const state = runNowState();
            if (state.hint) return state.hint;
            if (ed.runView) {
                // The chip beside it says "Past run"; the tooltip names which run, by its start time.
                const started = core.fmt.dateTime(ed.runView.run && ed.runView.run.started_at);
                return started ? t('easydrag.ui.state_run_view_started', { time: started }) : t('easydrag.ui.state_run_view');
            }
            if (ed.readonly) return t('easydrag.ui.run_readonly');
            return '';
        }

        function paintRevision() {
            const node = el.querySelector('[data-ed-revisions]');
            if (!node) return;
            const draftN = String(Number(ed.flow.draft_revision) || 0);
            const liveN = String(Number(ed.flow.live_revision) || 0);
            const kind = !ed.flow.live ? 'draft' : ed.flow.draft_revision !== ed.flow.published_draft_revision ? 'newer' : 'live';
            const paused = !isBlock && !!(ed.flow.live && !ed.flowEnabled);
            node.dataset.kind = kind;
            node.dataset.draft = draftN;
            node.dataset.live = liveN;
            node.dataset.paused = paused ? 'true' : 'false';
            node.hidden = !!ed.runView || !ed.flow.live;
            node.textContent = revisionText();
        }

        // liveChip marks a published version that no longer passes the server's current checks
        // (its start-up check), only while that version is still live: a new publication clears it.
        function liveChip() {
            const outdated = !ed.runView && !!ed.flow.live && ed.liveIssues.length > 0 && Number(ed.flow.live_revision) === ed.liveIssuesRevision;
            if (!outdated) return '';
            const hint = t('easydrag.ui.live_outdated_hint');
            return '<span class="ed-chip ed-chip--risk" data-ed-live-outdated title="' + esc(hint) + '">' + esc(t('easydrag.ui.state_live_outdated')) +
                '<span class="ed-sr-only"> ' + esc(hint) + '</span></span>';
        }

        // timersChip: the server's Date & time triggers do not fire (its flow store did not start).
        function timersChip() {
            if (isBlock) return '';
            if (ed.runView || ed.timersRunning !== false) return '';
            const text = t('easydrag.ui.timers_stopped_text');
            return '<span class="ed-chip ed-chip--warn" data-ed-timers-off title="' + esc(text) + '">' + esc(t('easydrag.ui.timers_stopped_title')) +
                '<span class="ed-sr-only"> ' + esc(text) + '</span></span>';
        }

        function renderHeader() {
            // A save answered after dispose must not point the window context at this flow again.
            if (host.isDisposed()) return;
            renderBanner();
            if (document.activeElement !== nameInput) nameInput.value = ed.model.doc.name || '';
            nameInput.readOnly = !!(ed.readonly || ed.runView);
            const s = stateOf();
            const inactive = !isBlock && ed.flow.live && !ed.flowEnabled && !ed.runView;
            el.querySelector('[data-ed-state]').innerHTML = '<span class="ed-chip ed-chip--' + s.cls + '"' + (s.title ? ' title="' + esc(s.title) + '"' : '') + '>' + esc(t(s.key)) + '</span>' +
                (inactive ? '<span class="ed-chip ed-chip--muted" data-ed-paused>' + esc(t('easydrag.ui.state_inactive')) + '</span>' : '') + (isBlock ? '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.block_pure')) + '</span>' : '') + liveChip() + timersChip();
            paintRevision();
            syncSaveChip();
            const ro = !!(ed.readonly || ed.runView);
            const running = runs.isRunning();
            const btn = cmd => el.querySelector('[data-ed-cmd="' + cmd + '"]');
            if (isBlock) {
                btn('test').querySelector('span').textContent = t('easydrag.ui.block_simulate');
                btn('test').title = t('easydrag.ui.block_simulate');
                btn('runs').hidden = true; btn('active').hidden = true;
                el.querySelector('[data-ed-cmd="templates"]').hidden = true;
                nameInput.setAttribute('aria-label', t('easydrag.ui.block_name'));
            }
            btn('test').disabled = ro || running;
            btn('test').classList.toggle('is-busy', running);
            btn('test').classList.toggle('ed-btn--primary', !ed.flow.live && !ro);
            btn('cancel').hidden = !running;
            btn('cancel').disabled = !!ed.run?.durableStatusError || btn('cancel').getAttribute('aria-busy') === 'true';
            btn('publish').disabled = ro;
            btn('publish').classList.toggle('has-changes', s.cls !== 'ok' && !ro);
            btn('publish').classList.toggle('ed-btn--primary', !!ed.flow.live && !ro);
            const runBtn = btn('run-now');
            if (runBtn) {
                const run = runNowState();
                const reason = runNowTitle();
                runBtn.hidden = isBlock || !ed.flow.live;
                runBtn.disabled = !!run.disabled;
                if (reason && ed.flow.live) { runBtn.title = reason; runBtn.setAttribute('title', reason); }
                else { runBtn.title = ''; runBtn.removeAttribute('title'); }
            }
            const sw = btn('active');
            sw.setAttribute('aria-checked', String(!!ed.flowEnabled));
            sw.disabled = !!ed.readonly || !!ed.runView;
            btn('runs').setAttribute('aria-pressed', String(runs.drawerOpen()));
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
            dot.className = 'ed-run-dot' + (r ? ' ed-run-dot--' + r.status : '');
            const failed = r && r.status === 'error' && ed.run && ed.run.steps ? Array.from(ed.run.steps.values()).find(s => s.status === 'error') : null;
            const failedNode = failed && ed.model.node(failed.node_id);
            const head = failedNode ? t('easydrag.ui.run_failed_at', { node: failedNode.label || failedNode.type }) : r ? ED.durable ? ED.durable.label(t, ed.run || r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status) : '';
            text.textContent = r
                ? head + (r.started_at ? ' · ' + core.fmt.relative(r.started_at) : '') + (r.duration_ms ? ' · ' + core.fmt.duration(r.duration_ms) : '')
                : t(isBlock ? 'easydrag.ui.block_not_simulated' : 'easydrag.ui.home_never_ran');
            // Narrow footers cut the text: the button keeps the whole of it as its name and tooltip.
            text.parentElement.setAttribute('aria-label', text.textContent);
            text.parentElement.title = text.textContent;
        }

        return { renderHeader, renderSaveState, renderIssues, renderLastRun, runNowState };
    }

    ED.header = { create };
})();
