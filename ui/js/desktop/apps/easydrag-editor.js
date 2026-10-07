// EasyDrag editor screen: header (name, state, actions), palette + canvas, footer (last run,
// issues, save state), window menus, keyboard shortcuts, run view and the wiring of all modules.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const VIEW_KEY = 'aurago.easydrag.view.';
    const SAVE_ICONS = { saved: 'check', dirty: 'pencil', saving: 'refresh', invalid: 'alert', offline: 'alert', failed: 'alert', conflict: 'alert' };

    // create builds the editor for a loaded flow. app: {ctx, t, esc, api, catalog, windowId,
    // readonly, openHome(), openFlow(id)}; loaded: GET /flows/{id} response.
    function create(app, loaded, opts) {
        const core = ED.core;
        const { t, esc, ctx } = app;
        const bag = core.bag();
        const draftModel = ED.model.create(loaded.flow.draft, app.catalog);
        let savedView = null;
        // paletteBefore: the palette was open when the run view hid it; leaving the run view shows it again.
        let paletteBefore = false;
        let contentDirty = false;
        let lastRecord = null;
        // partialRevision is the live revision of the last partial publish (ed.publishIncomplete).
        let partialRevision = 0;
        // restoring: the restore offer waits for an answer; the window menus change nothing meanwhile.
        let restoring = false;
        let deleting = false;
        let disposed = false;

        const el = core.el('<div class="ed-editor">' +
            '<header class="ed-head">' +
            '<button type="button" class="ed-icon-btn ed-back" data-ed-cmd="home" title="' + esc(t('easydrag.ui.back_home')) + '" aria-label="' + esc(t('easydrag.ui.back_home')) + '">' + core.icon('arrow-left') + '</button>' +
            '<div class="ed-title"><input class="ed-title-input" data-ed-name maxlength="120" spellcheck="false" aria-label="' + esc(t('easydrag.ui.flow_name')) + '"><span class="ed-state" data-ed-state></span></div>' +
            '<div class="ed-head-actions">' +
            '<button type="button" class="ed-btn ed-btn--ghost" data-ed-cmd="runs" aria-pressed="false">' + core.icon('history') + '<span>' + esc(t('easydrag.ui.runs_title')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-cmd="test" title="' + esc(t('easydrag.ui.test') + ' (' + core.shortcut('Ctrl+Enter') + ')') + '">' + core.icon('flask') + '<span>' + esc(t('easydrag.ui.test')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" hidden>' + core.icon('player-stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-cmd="publish">' + core.icon('rocket') + '<span>' + esc(t('easydrag.ui.publish')) + '</span></button>' +
            '<button type="button" class="ed-switch-btn" role="switch" aria-checked="false" data-ed-cmd="active"><span class="ed-switch" aria-hidden="true"><span></span></span><span>' + esc(t('easydrag.ui.active')) + '</span></button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="more" aria-label="' + esc(t('easydrag.ui.more')) + '" title="' + esc(t('easydrag.ui.more')) + '">' + core.icon('dots') + '</button>' +
            '</div></header>' +
            '<div class="ed-runview-banner" role="status" hidden>' + core.icon('history') + '<span data-ed-runview-text></span>' +
            '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-cmd="stop-viewed" hidden>' + core.icon('player-stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-cmd="exit-run-view">' + core.icon('arrow-left') + '<span>' + esc(t('easydrag.ui.run_view_exit')) + '</span></button></div>' +
            '<div class="ed-body"><nav class="ed-rail" aria-label="' + esc(t('easydrag.ui.rail_label')) + '">' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="palette" aria-pressed="true" title="' + esc(t('easydrag.ui.palette_title')) + ' (' + core.shortcut('Ctrl+K') + ')" aria-label="' + esc(t('easydrag.ui.palette_title')) + '">' + core.icon('plus') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="home" title="' + esc(t('easydrag.ui.back_home')) + '" aria-label="' + esc(t('easydrag.ui.back_home')) + '">' + core.icon('grid') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="templates" title="' + esc(t('easydrag.ui.home_templates')) + '" aria-label="' + esc(t('easydrag.ui.home_templates')) + '">' + core.icon('sparkles') + '</button>' +
            '<span class="ed-rail-spacer"></span>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="keys" title="' + esc(t('easydrag.ui.keys_title')) + ' (?)" aria-label="' + esc(t('easydrag.ui.keys_title')) + '">' + core.icon('keyboard') + '</button>' +
            '</nav></div>' +
            '<footer class="ed-foot">' +
            '<button type="button" class="ed-foot-item" data-ed-cmd="last-run"><span class="ed-run-dot" data-ed-last-dot></span><span data-ed-last-run></span></button>' +
            '<span class="ed-foot-spacer"></span>' +
            '<button type="button" class="ed-foot-item" data-ed-cmd="issues" aria-haspopup="dialog"><span data-ed-issues></span></button>' +
            '<span class="ed-foot-item ed-save" data-ed-save aria-live="polite"></span>' +
            '</footer></div>');
        const nameInput = el.querySelector('[data-ed-name]');
        const body = el.querySelector('.ed-body');
        const banner = el.querySelector('.ed-runview-banner');

        const ed = {
            ctx, t, esc, api: app.api, catalog: app.catalog, windowId: app.windowId, readonly: !!app.readonly,
            // model is the document shown (a stored run's in the run view); draftModel is the draft.
            root: el, flow: loaded.flow, flowEnabled: !!loaded.enabled, model: draftModel, draftModel,
            selection: new Set(), selectedEdge: null, hoverNode: null, hoverEdge: null, view: { x: 0, y: 0, zoom: 1 },
            run: null, runView: null, issues: loaded.issues || [], lastRunData: null, detail: null, quickAdd: null,
            dragging: false, initialRender: true, effectsConfirmed: new Set(), publishIncomplete: '', bus: core.emitter(), saver: null,
            enterRunView, exitRunView
        };

        const canvas = ED.canvas.create(ed);
        const wires = ED.wires.create(ed, canvas);
        const interact = ED.interact.create(ed, canvas, wires);
        const palette = ED.palette.createPanel(ed, canvas);
        const runs = ED.runs.create(ed, canvas);
        const publish = ED.publish.create(ed);
        body.appendChild(palette.el);
        body.appendChild(canvas.el);

        ed.saver = ED.saver.create({
            api: ed.api, flowId: ed.flow.id, model: draftModel, revision: ed.flow.draft_revision,
            onState: renderSaveState,
            // An edit made while the request ran is still unsaved: the saver sends it next.
            onSaved: ({ revision }) => { ed.flow.draft_revision = revision; contentDirty = ed.saver.isDirty(); renderHeader(); },
            onInvalid: issues => { ed.issues = issues; ed.bus.emit('issues', issues); },
            onConflict: () => ED.dialogs.conflict(ed)
        });

        // ── header and footer ───────────────────────────────────────────────────

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

        function renderHeader() {
            // A save answered after dispose must not point the window context at this flow again.
            if (disposed) return;
            renderBanner();
            if (document.activeElement !== nameInput) nameInput.value = ed.model.doc.name || '';
            nameInput.readOnly = !!(ed.readonly || ed.runView);
            const s = stateOf();
            const inactive = ed.flow.live && !ed.flowEnabled && !ed.runView;
            el.querySelector('[data-ed-state]').innerHTML = '<span class="ed-chip ed-chip--' + s.cls + '"' + (s.title ? ' title="' + esc(s.title) + '"' : '') + '>' + esc(t(s.key)) + '</span>' +
                (inactive ? '<span class="ed-chip ed-chip--muted">' + esc(t('easydrag.ui.state_inactive')) + '</span>' : '');
            const ro = !!(ed.readonly || ed.runView);
            const running = runs.isRunning();
            const btn = cmd => el.querySelector('[data-ed-cmd="' + cmd + '"]');
            btn('test').disabled = ro || running;
            btn('test').classList.toggle('is-busy', running);
            btn('cancel').hidden = !running;
            btn('publish').disabled = ro;
            btn('publish').classList.toggle('has-changes', s.cls !== 'ok' && !ro);
            const sw = btn('active');
            sw.setAttribute('aria-checked', String(!!ed.flowEnabled));
            sw.disabled = !!ed.readonly || !!ed.runView;
            btn('runs').setAttribute('aria-pressed', String(runs.drawerOpen()));
            btn('palette').setAttribute('aria-pressed', String(palette.isOpen()));
            // The run view keeps the palette hidden: a stored run takes no new steps.
            btn('palette').setAttribute('aria-disabled', String(!!ed.runView));
            // flow_id/run_id from a notification are cleared, so a later render does not reopen
            // that run, or a run of another flow after openFlow switched flows.
            ctx.updateWindowContext && ctx.updateWindowContext(ed.windowId, { flowId: ed.flow.id, flow_id: null, run_id: null });
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
            const label = core.icon(SAVE_ICONS[s] || 'check') + '<span class="ed-foot-text">' + esc(t('easydrag.ui.save_' + s)) + '</span>';
            const hadFocus = node.contains(document.activeElement);
            node.className = 'ed-foot-item ed-save ed-save--' + s;
            node.innerHTML = s === 'failed'
                ? '<button type="button" class="ed-save-retry" data-ed-cmd="save" title="' + esc(title) + '">' + label + '<span class="ed-save-action">' + esc(t('easydrag.ui.retry')) + '</span></button>'
                : label;
            node.title = s === 'failed' ? '' : title;
            // Each state replaces the retry button: keyboard focus moves to the new one or, once the
            // save went through, to the canvas.
            if (hadFocus && !disposed) (node.querySelector('[data-ed-cmd="save"]') || canvas.el).focus({ preventScroll: true });
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
        }

        function renderLastRun() {
            const r = ed.run && ed.run.record ? ed.run.record : (ed.run ? { status: ed.run.status, started_at: null } : lastRecord);
            const dot = el.querySelector('[data-ed-last-dot]');
            const text = el.querySelector('[data-ed-last-run]');
            dot.className = 'ed-run-dot' + (r ? ' ed-run-dot--' + r.status : '');
            const failed = r && r.status === 'error' && ed.run && ed.run.steps ? Array.from(ed.run.steps.values()).find(s => s.status === 'error') : null;
            const failedNode = failed && ed.model.node(failed.node_id);
            const head = failedNode ? t('easydrag.ui.run_failed_at', { node: failedNode.label || failedNode.type }) : r ? core.tr(t, 'easydrag.ui.status_' + r.status, r.status) : '';
            text.textContent = r
                ? head + (r.started_at ? ' · ' + core.fmt.relative(r.started_at) : '') + (r.duration_ms ? ' · ' + core.fmt.duration(r.duration_ms) : '')
                : t('easydrag.ui.home_never_ran');
            // Narrow footers cut the text: the button keeps the whole of it as its name and tooltip.
            text.parentElement.setAttribute('aria-label', text.textContent);
            text.parentElement.title = text.textContent;
        }

        // ── run view ────────────────────────────────────────────────────────────

        // renderBanner names the viewed run and offers Stop while it has not ended (not on a
        // read-only desktop). A Stop that goes away hands its focus to "Back to draft". The text
        // changes only with the run: the banner is a live region, and every header render calls this.
        function renderBanner() {
            const run = ed.runView && ed.runView.run;
            banner.hidden = !run;
            if (!run) return;
            const text = t('easydrag.ui.run_view_banner', {
                status: core.tr(t, 'easydrag.ui.status_' + run.status, run.status),
                time: core.fmt.dateTime(run.started_at),
                revision: run.revision
            });
            const node = banner.querySelector('[data-ed-runview-text]');
            if (node.textContent !== text) node.textContent = text;
            const stop = banner.querySelector('[data-ed-cmd="stop-viewed"]');
            const off = !!ed.readonly || !ED.runs.isActive(run.status);
            if (off && document.activeElement === stop) banner.querySelector('[data-ed-cmd="exit-run-view"]').focus();
            stop.hidden = off;
        }

        function enterRunView(detail) {
            ED.detail.close(ed);
            ED.palette.closeQuickAdd(ed);
            if (!ed.runView) {
                savedView = Object.assign({}, ed.view);
                paletteBefore = palette.isOpen();
            }
            ed.runView = { run: detail.run, doc: detail.doc };
            ed.model = ED.model.create(detail.doc, ed.catalog);
            ed.selection = new Set();
            el.classList.add('is-run-view');
            ed.bus.emit('model', { kind: 'reset', nodes: [], edges: [], meta: true, structural: true });
            runs.applyRunView(detail);
            // The run view needs the room: the palette hides (a stored run takes no new steps) and
            // comes back on exit. A failed run shows its failed step, any other run a readable fit.
            const run = ed.runView;
            const viewAtEnter = ed.view;
            // A pan or zoom the user made meanwhile wins over the placement.
            const place = () => {
                if (disposed || ed.runView !== run || ed.view !== viewAtEnter) return;
                const failed = (detail.steps || []).find(st => st.status === 'error' && ed.model.node(st.node_id));
                if (failed) canvas.centerOn(failed.node_id, { animate: true });
                else canvas.fit({ animate: true, readable: true });
            };
            if (palette.isOpen()) { palette.setOpen(false, true); palette.afterMove(place); } else place();
            renderHeader();
            setMenus();
        }

        function exitRunView() {
            if (!ed.runView) return;
            ED.detail.close(ed);
            ed.runView = null;
            ed.model = draftModel;
            ed.selection = new Set();
            el.classList.remove('is-run-view');
            runs.clearRun();
            ed.bus.emit('model', { kind: 'reset', nodes: [], edges: [], meta: true, structural: true });
            if (savedView) canvas.setView(savedView, { animate: true });
            if (paletteBefore && !palette.isOpen() && !palette.isOverlay()) palette.setOpen(true, true);
            paletteBefore = false;
            renderHeader();
            setMenus();
            // The hints were not checked while the run view showed (an edit there, a restore).
            publish.refreshIssues();
            // The banner and its buttons are gone: keyboard focus goes to the draft's canvas.
            canvas.el.focus({ preventScroll: true });
        }

        // ── commands ────────────────────────────────────────────────────────────

        async function saveNow() {
            if (ed.readonly || ed.runView) return;
            await ed.saver.save();
            if (ed.saver.state === 'saved') canvas.announce(t('easydrag.ui.save_saved'));
        }

        function test() { if (!runs.isRunning()) runs.startTest({}); }

        // The palette stays hidden in the run view; its toggle and the search key wait until it ends.
        function togglePalette() { if (!ed.runView) palette.setOpen(!palette.isOpen()); }
        function searchPalette() { if (!ed.runView) palette.focusSearch(); }

        function exportFlow() {
            const a = document.createElement('a');
            a.href = ed.api.exportUrl(ed.flow.id);
            // The export is the draft, also in the run view: it carries the draft's name.
            a.download = (draftModel.doc.name || 'flow') + '.easydrag.json';
            document.body.appendChild(a);
            a.click();
            a.remove();
        }

        // deleteFlow deletes the flow; while the request runs, its own "deleted" broadcast is not
        // news (deleting).
        async function deleteFlow() {
            const ok = await ctx.confirmDialog(t('easydrag.ui.delete_title'), t('easydrag.ui.delete_text', { name: draftModel.doc.name }));
            if (!ok || deleting) return;
            const id = ed.flow.id;
            deleting = true;
            try {
                await ed.api.remove(id);
            } catch (err) {
                // FLOW_NOT_FOUND: a delete made elsewhere came first. The flow is gone all the
                // same, so this delete is done. Any other error: the flow still exists and the
                // editor stays usable, its saver keeps running.
                if (core.errorCode(err) !== 'FLOW_NOT_FOUND') {
                    deleting = false;
                    ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
                    return;
                }
            }
            forget(id);
            if (disposed) return;
            app.openHome();
        }

        // goneElsewhere: the open flow was deleted in another window, in Mission Control, on the
        // missions page or by the agent. The editor says so once and shows the start page.
        function goneElsewhere() {
            if (deleting || disposed) return;
            deleting = true;
            forget(ed.flow.id);
            ctx.notify({ title: draftModel.doc.name, message: t('easydrag.ui.flow_deleted_elsewhere') });
            app.openHome();
        }

        // forget stops saving a flow that is gone (a save would only answer FLOW_NOT_FOUND) and drops
        // what this browser kept for it (core.forgetFlow): the emergency copy, the stored view, the
        // confirmed test effects and the last test trigger. Edits made after the delete do not
        // pile up under a dead id.
        function forget(id) {
            contentDirty = false;
            ed.saver.dispose();
            storeView.cancel();
            pendingView = null;
            core.forgetFlow(id);
        }

        // goHome and duplicateFlow stop when the editor was disposed while leave() waited (the
        // window closed): nothing may be built on a closed window.
        async function goHome(opts) {
            if (!(await leave()) || disposed) return;
            app.openHome(opts);
        }

        async function duplicateFlow() {
            if (!(await leave()) || disposed) return;
            try {
                // The draft, also in the run view (whose model is a stored run's document).
                const doc = draftModel.toJSON();
                doc.name = t('easydrag.ui.copy_of', { name: doc.name });
                const res = await ed.api.create({ import: doc });
                app.openFlow(res.flow.id);
            } catch (err) { ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); }
        }

        // leave saves pending content before the editor closes; false keeps it open. A drag still in
        // progress is cancelled first (its nodes move back), so no mid-drag state is saved. Calls that
        // overlap (two navigations waiting on a failed save) share one answer: at most one dialog asks.
        let leaving = null;
        function leave() {
            if (!leaving) leaving = leaveOnce().finally(() => { leaving = null; });
            return leaving;
        }

        async function leaveOnce() {
            interact.abortGesture();
            if (ed.readonly || !contentDirty) return true;
            const ok = await ed.saver.flush();
            if (ok) return true;
            return ctx.confirmDialog(t('easydrag.ui.leave_title'), t('easydrag.ui.leave_text'));
        }

        function moreMenu(anchor) {
            const r = anchor.getBoundingClientRect();
            const run = runNowState();
            ctx.showContextMenu(r.left, r.bottom + 4, [
                { icon: 'play', label: t('easydrag.ui.run_now'), disabled: run.disabled, disabledHint: run.hint, action: () => runs.runLive() },
                { icon: 'settings', label: t('easydrag.ui.flow_settings'), action: () => ED.dialogs.flowSettings(ed) },
                { icon: 'copy', label: t('easydrag.ui.home_duplicate'), disabled: !!ed.readonly, action: duplicateFlow },
                { icon: 'download', label: t('easydrag.ui.home_export'), action: exportFlow },
                { icon: 'list', label: t('easydrag.ui.home_mission_control'), action: () => ctx.openApp && ctx.openApp('mission-control') },
                { separator: true },
                { icon: 'trash', label: t('easydrag.ui.home_delete'), disabled: !!ed.readonly, action: deleteFlow }
            ]);
        }

        // ── window menus ────────────────────────────────────────────────────────

        // Menu keys are canonical strings ("Ctrl+S"), as in every desktop app: the desktop draws
        // them as given and matches Ctrl as Ctrl or ⌘, so core.shortcut's "⌘S" would never match.
        // The desktop runs a menu item's shortcut before the editor sees the key, past onKeyDown's
        // guards (dialogs, context menus, detail view, quick-add), and prevents it, so onKeyDown
        // does not run it a second time. Keys the canvas handles itself (interact, "?") are
        // therefore a shortcutHint, which the desktop draws but does not dispatch. Ctrl+S,
        // Ctrl+Enter and Ctrl+K are real shortcuts (Ctrl+K so that the desktop's search does not
        // take it); they do nothing while a dialog is open. Edits wait while a dialog, the detail
        // view or quick-add covers the canvas, and while the restore offer waits for its answer.
        function modalOpen() { return !!el.querySelector('.ed-modal-backdrop:not(.is-closing)'); }
        function overlayOpen() { return !!(ed.detail || ed.quickAdd || modalOpen()); }
        const unlessModal = fn => () => { if (!modalOpen()) fn(); };
        const unlessOverlay = fn => () => { if (!overlayOpen() && !restoring) fn(); };

        function setMenus() {
            if (disposed || typeof ctx.setWindowMenus !== 'function') return;
            const ro = !!(ed.readonly || ed.runView || restoring);
            const busy = overlayOpen();
            const sel = ed.selection.size > 0;
            const run = runNowState();
            ctx.setWindowMenus(ed.windowId, [
                {
                    id: 'flow', labelKey: 'easydrag.ui.menu_flow', items: [
                        { id: 'home', labelKey: 'easydrag.ui.back_home', icon: 'list', action: goHome },
                        { type: 'separator' },
                        { id: 'save', labelKey: 'easydrag.ui.save_now', icon: 'save', shortcut: 'Ctrl+S', disabled: ro, action: unlessModal(saveNow) },
                        { id: 'test', labelKey: 'easydrag.ui.test', icon: 'run', shortcut: 'Ctrl+Enter', disabled: ro, action: unlessModal(test) },
                        { id: 'run', labelKey: 'easydrag.ui.run_now', icon: 'play', disabled: restoring || run.disabled, disabledHint: run.hint, action: unlessModal(() => runs.runLive()) },
                        { id: 'publish', labelKey: 'easydrag.ui.publish', icon: 'upload', disabled: ro, action: unlessModal(() => publish.openDialog()) },
                        { type: 'separator' },
                        { id: 'settings', labelKey: 'easydrag.ui.flow_settings', icon: 'settings', disabled: restoring, action: unlessModal(() => ED.dialogs.flowSettings(ed)) },
                        { id: 'export', labelKey: 'easydrag.ui.home_export', icon: 'download', action: exportFlow },
                        { id: 'delete', labelKey: 'easydrag.ui.home_delete', icon: 'trash', disabled: !!ed.readonly || restoring, action: unlessModal(deleteFlow) }
                    ]
                },
                {
                    id: 'edit', labelKey: 'easydrag.ui.menu_edit', items: [
                        { id: 'undo', labelKey: 'easydrag.ui.undo', icon: 'undo', shortcutHint: 'Ctrl+Z', disabled: ro || busy || !ed.model.canUndo(), action: unlessOverlay(() => ed.model.undo()) },
                        { id: 'redo', labelKey: 'easydrag.ui.redo', icon: 'redo', shortcutHint: 'Ctrl+Shift+Z', disabled: ro || busy || !ed.model.canRedo(), action: unlessOverlay(() => ed.model.redo()) },
                        { type: 'separator' },
                        { id: 'cut', labelKey: 'easydrag.ui.cut', icon: 'scissors', shortcutHint: 'Ctrl+X', disabled: ro || busy || !sel, action: unlessOverlay(() => { if (interact.copySelection()) interact.removeSelection(); }) },
                        { id: 'copy', labelKey: 'easydrag.ui.copy', icon: 'copy', shortcutHint: 'Ctrl+C', disabled: busy || !sel, action: unlessOverlay(() => interact.copySelection()) },
                        { id: 'paste', labelKey: 'easydrag.ui.paste', icon: 'clipboard', shortcutHint: 'Ctrl+V', disabled: ro || busy, action: unlessOverlay(() => interact.pasteAt(null)) },
                        { id: 'duplicate', labelKey: 'easydrag.ui.duplicate', icon: 'copy', shortcutHint: 'Ctrl+D', disabled: ro || busy || !sel, action: unlessOverlay(() => interact.select(ed.model.duplicate(Array.from(ed.selection)))) },
                        { id: 'delete-sel', labelKey: 'easydrag.ui.delete', icon: 'trash', shortcutHint: 'Del', disabled: ro || busy || (!sel && !ed.selectedEdge), action: unlessOverlay(() => interact.removeSelection()) },
                        { type: 'separator' },
                        { id: 'select-all', labelKey: 'easydrag.ui.select_all', icon: 'check-square', shortcutHint: 'Ctrl+A', disabled: busy, action: unlessOverlay(() => interact.selectAll()) }
                    ]
                },
                {
                    id: 'view', labelKey: 'easydrag.ui.menu_view', items: [
                        { id: 'zoom-in', labelKey: 'easydrag.ui.zoom_in', icon: 'zoom-in', shortcutHint: '+', action: () => canvas.zoomBy(1.2) },
                        { id: 'zoom-out', labelKey: 'easydrag.ui.zoom_out', icon: 'zoom-out', shortcutHint: '−', action: () => canvas.zoomBy(1 / 1.2) },
                        { id: 'zoom-fit', labelKey: 'easydrag.ui.zoom_fit', icon: 'maximize', shortcutHint: 'Shift+1', action: () => canvas.fit({ animate: true }) },
                        { type: 'separator' },
                        { id: 'search', labelKey: 'easydrag.ui.keys_search', icon: 'search', shortcut: 'Ctrl+K', disabled: !!ed.runView, action: unlessModal(searchPalette) },
                        { id: 'palette', labelKey: 'easydrag.ui.palette_title', icon: 'sidebar', checked: palette.isOpen(), disabled: !!ed.runView, action: () => togglePalette() },
                        { id: 'runs', labelKey: 'easydrag.ui.runs_title', icon: 'list', checked: runs.drawerOpen(), action: () => { runs.toggleDrawer(); renderHeader(); setMenus(); } },
                        { type: 'separator' },
                        { id: 'keys', labelKey: 'easydrag.ui.keys_title', icon: 'help', shortcutHint: '?', action: unlessModal(() => ED.dialogs.shortcuts(ed)) }
                    ]
                }
            ]);
        }
        const setMenusSoon = core.debounce(setMenus, 120);
        // Dialogs, the detail view, quick-add and the run drawer open as children of the editor:
        // the menus follow them (disabled edits, checked drawer).
        if (typeof MutationObserver === 'function') {
            const overlays = new MutationObserver(() => setMenusSoon());
            overlays.observe(el, { childList: true });
            bag.add(() => overlays.disconnect());
        }

        // ── events ──────────────────────────────────────────────────────────────

        // Every change of the draft is saved, also one made while the run view shows another
        // document (a restore answered there); the rest follows the model shown. Pans and zooms
        // are no changes: the view is stored per device (storeView), never in the document.
        bag.add(draftModel.on(change => {
            contentDirty = true;
            ed.saver.schedule();
            if (ed.model !== draftModel) return;
            ed.bus.emit('model', change);
            publish.refreshIssues();
            if (change.meta || change.kind !== 'change') renderHeader();
            if (change.structural || change.kind !== 'change') pruneSelection();
            setMenusSoon();
        }));

        function pruneSelection() {
            const before = ed.selection.size;
            ed.selection.forEach(id => { if (!ed.model.node(id)) ed.selection.delete(id); });
            if (ed.selectedEdge && !ed.model.edge(ed.selectedEdge)) ed.selectedEdge = null;
            if (ed.selection.size !== before) ed.bus.emit('selection', ed.selection);
        }

        // A drag holds back emergency copies (at most one per 500 ms); its end writes the last one,
        // and so does a page that goes away or into the background (closed, reloaded, switched
        // away from on a phone), where no later timer may run.
        bag.add(ed.bus.on('gesture-end', () => ed.saver.flushCopy()));
        bag.listen(window, 'pagehide', () => ed.saver.flushCopy());
        bag.listen(document, 'visibilitychange', () => { if (document.visibilityState === 'hidden') ed.saver.flushCopy(); });
        bag.add(ed.bus.on('quick-add', req => { if (!ed.readonly && !ed.runView) { ED.palette.openQuickAdd(ed, canvas, req); setMenus(); } }));
        bag.add(ed.bus.on('open-detail', req => { ED.detail.open(ed, req.nodeId, { param: req.param }); setMenus(); }));
        bag.add(ed.bus.on('detail-closed', () => { canvas.el.focus({ preventScroll: true }); setMenus(); }));
        bag.add(ed.bus.on('node-test', req => runs.startTest({ onlyNode: req.nodeId })));
        bag.add(ed.bus.on('focus-node', id => { interact.select([id]); canvas.centerOn(id, { animate: true }); }));
        bag.add(ed.bus.on('connect-picker', req => ED.dialogs.connectPicker(ed, canvas, req.nodeId)));
        bag.add(ed.bus.on('selection', setMenusSoon));
        bag.add(ed.bus.on('palette', () => { renderHeader(); setMenusSoon(); }));
        bag.add(ed.bus.on('issues', renderIssues));
        bag.add(ed.bus.on('run', () => { renderHeader(); renderLastRun(); }));
        bag.add(ed.bus.on('last-run', record => { lastRecord = record; renderLastRun(); }));
        bag.add(ed.bus.on('published', flow => {
            ed.flow = Object.assign(ed.flow, flow);
            // A partial publish remembers its live revision: only a higher one finishes it (refreshRecord).
            partialRevision = ed.publishIncomplete ? Number(ed.flow.live_revision) || 0 : 0;
            renderHeader();
            setMenus();
        }));
        // The Active switch also decides whether Run now is offered.
        bag.add(ed.bus.on('enabled', () => { renderHeader(); setMenus(); }));
        // A refused paste is announced to screen readers by interact; this shows it to everyone.
        bag.add(ed.bus.on('paste-refused', () => ctx.notify({ title: t('easydrag.ui.paste'), message: t('easydrag.ui.paste_refused') })));
        // The viewport is stored per flow once panning or zooming pauses for 400 ms, not on every
        // frame; dispose stores one still pending. It is stored as the world point in the middle of
        // the canvas and the zoom ({cx, cy, zoom}), so it suits another window size (placeView).
        let pendingView = null;
        const storeView = core.debounce(() => {
            if (pendingView) core.storage.set(VIEW_KEY + ed.flow.id, pendingView);
            pendingView = null;
        }, 400);
        bag.add(ed.bus.on('view', () => {
            if (ed.runView) return;
            pendingView = canvas.center();
            storeView();
        }));

        bag.listen(el, 'click', (event) => {
            const b = event.target.closest('[data-ed-cmd]');
            if (!b || b.disabled || b.getAttribute('aria-disabled') === 'true') return;
            const cmd = b.dataset.edCmd;
            if (cmd === 'home') goHome();
            else if (cmd === 'runs') { runs.toggleDrawer(); renderHeader(); setMenus(); }
            else if (cmd === 'test') test();
            else if (cmd === 'cancel') runs.cancel();
            else if (cmd === 'publish') publish.openDialog();
            else if (cmd === 'active') publish.setActive(!ed.flowEnabled);
            else if (cmd === 'more') moreMenu(b);
            else if (cmd === 'exit-run-view') exitRunView();
            else if (cmd === 'stop-viewed') runs.stopRun(ed.runView && ed.runView.run);
            else if (cmd === 'last-run') { if (ed.run || lastRecord) runs.toggleDrawer(true); renderHeader(); }
            else if (cmd === 'issues') publish.openIssues(b);
            else if (cmd === 'keys') ED.dialogs.shortcuts(ed);
            else if (cmd === 'palette') togglePalette();
            else if (cmd === 'templates') goHome({ section: 'templates' });
            else if (cmd === 'save') saveNow();
        });

        bag.listen(nameInput, 'keydown', (event) => {
            if (event.key === 'Enter') { event.preventDefault(); nameInput.blur(); }
            if (event.key === 'Escape') { nameInput.value = ed.model.doc.name || ''; nameInput.blur(); }
        });
        bag.listen(nameInput, 'change', () => {
            const name = nameInput.value.trim();
            if (!name) { nameInput.value = ed.model.doc.name || ''; return; }
            if (name !== ed.model.doc.name && !ed.readonly && !ed.runView) ed.model.setFlow({ name });
        });

        function onKeyDown(event) {
            // Some keydown events carry no key (Chrome's autofill): there is nothing to handle.
            if (disposed || event.defaultPrevented || !el.isConnected || typeof event.key !== 'string') return;
            // The editor's keys come from inside it, or from the body when nothing has focus.
            const target = typeof event.composedPath === 'function' ? event.composedPath()[0] : event.target;
            if (!target || (target !== document.body && !el.contains(target))) return;
            if (typeof ctx.isActive === 'function' && !ctx.isActive()) return;
            const mod = core.isMod(event);
            const key = event.key.toLowerCase();
            // Mod+S belongs to the editor even when it cannot save (a dialog or context menu is
            // open, or Save is disabled): the browser's "Save Page" never opens from here. It saves
            // only when the desktop did not run the Save item (prevented keys return above) and
            // Save is enabled (saveNow checks read-only and the run view; the restore offer here).
            if (mod && key === 's') event.preventDefault();
            if (el.querySelector('.ed-modal-backdrop') || document.querySelector('.vd-context-menu')) return;
            if (mod && key === 's') { if (!restoring) saveNow(); return; }
            if (mod && event.key === 'Enter') { event.preventDefault(); test(); return; }
            if (mod && key === 'k') { event.preventDefault(); searchPalette(); return; }
            if (core.isEditable(target) || ed.detail || ed.quickAdd) return;
            const active = document.activeElement;
            const onCanvas = canvas.el.contains(active) || active === document.body || active === el;
            if (event.key === '?' && onCanvas) { event.preventDefault(); ED.dialogs.shortcuts(ed); return; }
            if (event.key === 'Escape' && ed.runView) { event.preventDefault(); exitRunView(); return; }
            if (onCanvas && interact.handleKey(event)) event.preventDefault();
        }
        bag.listen(document, 'keydown', onKeyDown);
        bag.listen(document, 'keyup', event => interact.handleKeyUp(event));
        // Deletes and switches from Mission Control and the missions page name the flow (flow_id).
        // One without an id (no single flow held the mission) may still be this flow: the record
        // is read again, and FLOW_NOT_FOUND means it was deleted.
        bag.listen(document, 'aurago:flows-changed', (event) => {
            const d = event.detail || {};
            const mine = d.flow_id === ed.flow.id;
            const maybe = !d.flow_id && (d.reason === 'deleted' || d.reason === 'enabled');
            if (!mine && !maybe) return;
            if (d.reason === 'deleted') {
                if (deleting) return;
                if (mine) goneElsewhere(); else refreshRecord();
                return;
            }
            // A live run ended: the last run, the drawer and a run view follow (debounced in runs).
            if (d.reason === 'run_finished') runs.runFinished();
            if (d.reason === 'enabled' || d.reason === 'published') refreshRecord();
        });

        // refreshRecord re-reads publication state changed elsewhere (another window, the agent,
        // Mission Control): the state chip, the Active switch and Run now follow. "published" is
        // broadcast for partial publishes too, and publishing the same revision again changes
        // nothing in the store: only a higher live revision finishes a partial one. A flow that is
        // gone was deleted elsewhere; any other failure keeps the current state.
        async function refreshRecord() {
            let res;
            try {
                res = await ed.api.get(ed.flow.id);
            } catch (err) {
                if (core.errorCode(err) === 'FLOW_NOT_FOUND') goneElsewhere();
                return;
            }
            if (disposed || deleting || !res || !res.flow) return;
            ed.flow.live = res.flow.live;
            ed.flow.published_draft_revision = res.flow.published_draft_revision;
            ed.flow.live_revision = res.flow.live_revision;
            if (ed.publishIncomplete && Number(res.flow.live_revision) > partialRevision) ed.publishIncomplete = '';
            ed.flowEnabled = !!res.enabled;
            renderHeader();
            setMenus();
        }

        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(ed.windowId, leave);

        // ── start ───────────────────────────────────────────────────────────────

        // start offers an emergency copy that differs from the server draft. Only an explicit
        // "discard" drops it; any other answer (null: the dialog closed with the editor) keeps the
        // copy and the draft as they are, so the offer comes again when the flow opens next.
        async function start() {
            const copy = !ed.readonly && ED.saver.emergencyCopy(ed.flow.id, ed.flow.draft_revision);
            if (copy && differs(copy.doc, draftModel.toJSON())) {
                restoring = true;
                const offer = ED.dialogs.restore(ed, copy);
                setMenus();
                const choice = await offer;
                restoring = false;
                if (choice === 'discard') ED.saver.dropEmergencyCopy(ed.flow.id);
                if (disposed) return;
                if (choice === 'restore') draftModel.replaceDoc(copy.doc);
                setMenus();
            } else if (copy) {
                ED.saver.dropEmergencyCopy(ed.flow.id);
            }
            if (opts && opts.runId) runs.openRunView(opts.runId);
            else runs.loadLast();
        }

        // placeView restores the view stored for this flow ({cx, cy, zoom}, see storeView). A view
        // stored in the older form {x, y, zoom}, and the draft's viewport, are screen offsets of
        // another window size and are not used. Without a stored view, when it shows no step, and in
        // an editor 560 px wide or narrower, the flow gets the readable fit.
        function placeView() {
            const stored = core.storage.get(VIEW_KEY + ed.flow.id, null);
            const wide = el.getBoundingClientRect().width > 560;
            if (wide && ed.model.doc.nodes.length && canvas.restoreView(stored)) return;
            canvas.fit({ readable: true });
        }

        function differs(a, b) {
            const strip = d => JSON.stringify(Object.assign({}, d, { viewport: null }));
            return strip(a) !== strip(b);
        }

        renderHeader();
        renderSaveState('saved');
        renderIssues();
        renderLastRun();
        canvas.render();
        wires.render();
        requestAnimationFrame(() => {
            if (disposed) return;
            // The palette settles first: one floating over the canvas (narrow windows) closes.
            palette.syncLayout();
            placeView();
            ed.initialRender = false;
            canvas.el.focus({ preventScroll: true });
        });
        setMenus();
        if (!ed.issues.length) publish.refreshIssues();
        start();

        return {
            el, ed, leave,
            showRun(runId) { runs.openRunView(runId); },
            // dispose saves pending content last: the detail view flushes its note on close, and
            // interact cancels a drag in progress (its nodes move back), so neither is lost nor
            // saved in a mid-drag state. Each step runs on its own: one that throws cannot keep
            // the saver, the listeners or the window's close guard from being released.
            dispose() {
                if (disposed) return;
                disposed = true;
                const safely = fn => { try { fn(); } catch (err) { console.error('EasyDrag editor cleanup failed', err); } };
                safely(() => ED.detail.close(ed));
                safely(() => ED.palette.closeQuickAdd(ed));
                safely(() => interact.dispose());
                safely(() => { if (contentDirty) ed.saver.save(); });
                safely(() => storeView.flush());
                [wires, canvas, palette, runs, publish].forEach(m => safely(() => m.dispose()));
                safely(() => ed.saver.dispose());
                setMenusSoon.cancel();
                safely(() => bag.dispose());
                if (typeof ctx.setWindowBeforeClose === 'function') safely(() => ctx.setWindowBeforeClose(ed.windowId, null));
                el.remove();
            }
        };
    }

    ED.editor = { create };
})();
