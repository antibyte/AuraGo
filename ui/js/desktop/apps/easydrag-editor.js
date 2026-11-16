// EasyDrag editor screen: palette + canvas, window menus, keyboard shortcuts, run view and the
// wiring of all modules. The header and footer (name, state, actions, last run, issues, save state) are easydrag-header.js.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const VIEW_KEY = 'aurago.easydrag.view.';

    // create builds the editor for a loaded flow. app: {ctx, t, esc, api, catalog, windowId,
    // readonly, openHome(), openFlow(id)}; loaded: GET /flows/{id} response.
    function create(app, loaded, opts) {
        // abort is set once the cleanup bag exists: a build that throws after that (a late render) releases what it set up.
        const guard = { abort: null };
        try {
            return build(app, loaded, opts, guard);
        } catch (err) {
            if (guard.abort) guard.abort();
            throw err;
        }
    }

    function build(app, loaded, opts, guard) {
        const core = ED.core;
        const isBlock = !!app.isBlock;
        const { t, esc, ctx } = app;
        const bag = core.bag();
        let canvas, wires, interact, palette, runs, publish, history, workspace, authoring, extraction, emailBinding, blockTracker;
        // A build that fails later releases what it set up (dispose skips what does not exist yet).
        guard.abort = () => { try { dispose(); } catch (err) { bag.dispose(); } };
        const draftModel = ED.model.create(loaded.flow.draft, app.catalog);
        // One notice for the draft only. The run view builds its own model and must not say this.
        // A later restore (replaceDoc emits reset) says it if the first document did not.
        let duplicatesNoted = false;
        function noteDroppedDuplicates() {
            if (duplicatesNoted || !draftModel.repairs().droppedNodes) return;
            duplicatesNoted = true;
            if (typeof ctx.notify === 'function') ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.issue_node_id_duplicate') });
        }
        noteDroppedDuplicates();
        bag.add(draftModel.on(change => { if (change && change.kind === 'reset') noteDroppedDuplicates(); }));
        let savedView = null;
        let contentDirty = false;
        let sessionEditRevision = 0;
        let sessionActionFlushing = false;
        let lastRecord = null;
        // partialRevision is the live revision of the last partial publish (ed.publishIncomplete).
        let partialRevision = 0;
        // restoring: the restore offer waits for an answer; the window menus change nothing meanwhile.
        let restoring = false;
        let deleting = false;
        let disposed = false;

        const lab = id => esc(t(ED.actions.row(id).label));
        const el = core.el('<div class="ed-editor">' +
            '<header class="ed-head">' +
            '<button type="button" class="ed-icon-btn ed-back" data-ed-cmd="home" title="' + esc(t('easydrag.ui.back_home')) + '" aria-label="' + esc(t('easydrag.ui.back_home')) + '">' + ED.icons.of('back') + '</button>' +
            '<div class="ed-title"><input class="ed-title-input" data-ed-name maxlength="120" spellcheck="false" aria-label="' + esc(t('easydrag.ui.flow_name')) + '"><span class="ed-state" data-ed-state></span><span class="ed-revision" data-ed-revision data-ed-revisions></span><span class="ed-foot-item ed-save" data-ed-save aria-live="polite"></span></div>' +
            '<div class="ed-head-history" role="group" data-ed-history-host></div>' +
            '<div class="ed-head-actions">' +
            '<button type="button" class="ed-btn ed-btn--ghost" data-ed-cmd="runs" aria-pressed="false">' + ED.icons.of('runs') + '<span>' + esc(t('easydrag.ui.runs_title')) + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-cmd="test">' + ED.icons.of('test') + '<span>' + lab('flow.test') + '</span></button>' +
            // Below 900 px the text of Stop and of the Active switch (below) is hidden: the name and the tooltip say the same words.
            '<button type="button" class="ed-btn ed-btn--danger" data-ed-cmd="cancel" aria-label="' + lab('flow.stop') + '" hidden>' + ED.icons.of('stop') + '<span>' + lab('flow.stop') + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--primary" data-ed-cmd="publish">' + ED.icons.of('publish') + '<span>' + lab('flow.publish') + '</span></button>' +
            '<button type="button" class="ed-btn" data-ed-cmd="run-now" hidden>' + ED.icons.of('run') + '<span>' + lab('flow.run_now') + '</span></button>' +
            '<button type="button" class="ed-switch-btn" role="switch" aria-checked="false" data-ed-cmd="active" aria-label="' + lab('flow.active') + '"><span class="ed-switch" aria-hidden="true"><span></span></span><span>' + lab('flow.active') + '</span></button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="more" aria-label="' + esc(t('easydrag.ui.more')) + '" title="' + esc(t('easydrag.ui.more')) + '">' + ED.icons.of('more') + '</button>' +
            '</div></header>' +
            '<div class="ed-runview-banner" role="status" hidden>' + ED.icons.of('runs') + '<span data-ed-runview-text></span>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-cmd="approval-review" hidden>' + ED.icons.of('approve') + '<span>' + esc(t('easydrag.ui.approval_review')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-cmd="stop-viewed" hidden>' + ED.icons.of('stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-cmd="rerun-viewed" hidden>' + ED.icons.of('test') + '<span>' + esc(t('easydrag.ui.run_rerun_test')) + '</span></button>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-cmd="exit-run-view">' + ED.icons.of('back') + '<span>' + esc(t('easydrag.ui.run_view_exit')) + '</span></button></div>' +
            '<div class="ed-body"><nav class="ed-rail" aria-label="' + esc(t('easydrag.ui.rail_label')) + '">' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="palette" aria-pressed="true" title="' + esc(t('easydrag.ui.palette_title')) + ' (' + core.shortcut('Ctrl+K') + ')">' + ED.icons.of('palette') + ED.icons.caption(t, 'palette') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="home" title="' + esc(t('easydrag.ui.back_home')) + '">' + ED.icons.of('home') + ED.icons.caption(t, 'home') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="templates" title="' + esc(t('easydrag.ui.home_templates')) + '">' + ED.icons.of('templates') + ED.icons.caption(t, 'templates') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="groups" title="' + esc(t('easydrag.ui.groups')) + '">' + ED.icons.of('groups') + ED.icons.caption(t, 'groups') + '</button>' +
            '<span class="ed-rail-spacer"></span>' +
            '<button type="button" class="ed-icon-btn" data-ed-cmd="keys" title="' + esc(t('easydrag.ui.keys_title')) + ' (?)">' + ED.icons.of('keys') + ED.icons.caption(t, 'keys') + '</button>' +
            '</nav></div>' +
            '<footer class="ed-foot">' +
            '<button type="button" class="ed-foot-item" data-ed-cmd="last-run"><span class="ed-last-mark" data-ed-last-dot></span><span data-ed-last-run></span></button>' +
            '<span class="ed-foot-spacer"></span>' +
            '<button type="button" class="ed-foot-item" data-ed-cmd="issues" aria-haspopup="dialog"><span data-ed-issues></span></button>' +
            '</footer></div>');
        const nameInput = el.querySelector('[data-ed-name]'); bag.add(ED.kit.limit(nameInput, { max: 120, counter: false, t }).detach);
        const body = el.querySelector('.ed-body');
        const banner = el.querySelector('.ed-runview-banner');

        const ed = {
            ctx, t, esc, api: app.api, blockApi: app.blockApi, catalog: app.catalog, windowId: app.windowId, readonly: !!app.readonly,
            isBlock, openBlock: app.openBlock,
            // model is the document shown (a stored run's in the run view); draftModel is the draft.
            root: el, flow: loaded.flow, flowEnabled: !!loaded.enabled, model: draftModel, draftModel,
            selection: new Set(), selectedEdge: null, hoverNode: null, hoverEdge: null, view: { x: 0, y: 0, zoom: 1 },
            run: null, runView: null, issues: loaded.issues || [],
            timersRunning: loaded.timers_running !== false, liveIssues: Array.isArray(loaded.live_issues) ? loaded.live_issues : [], liveIssuesRevision: Number(loaded.flow.live_revision) || 0, lastRunData: null, detail: null, quickAdd: null,
            dragging: false, initialRender: true, effectsConfirmed: new Set(), publishIncomplete: '', bus: core.emitter(), saver: null,
            enterRunView, exitRunView
        };
        ed.sessionActionEdit = () => { if (!sessionActionFlushing) sessionEditRevision++; };
        ed.sessionActionCommit = fn => {
            const wasFlushing = sessionActionFlushing;
            sessionActionFlushing = true;
            try { return fn(); } finally { sessionActionFlushing = wasFlushing; }
        };
        bag.add(() => { if (ed.packExportControl) ed.packExportControl.abort(); });

        canvas = ED.canvas.create(ed);
        if (ED.groups) { ed.groups = ED.groups.create(ed, canvas); bag.add(() => ed.groups.dispose()); }
        wires = ED.wires.create(ed, canvas);
        interact = ED.interact.create(ed, canvas, wires);
        palette = ED.palette.createPanel(ed, canvas);
        if (!ed.catalog.blockContracts) ed.catalog.blockContracts = new Map();
        if (ED.blockFields && ed.blockApi) blockTracker = ED.blockFields.track(ed);
        el.classList.toggle('is-block-editor', isBlock);
        runs = isBlock ? ED.blocks.runController(ed, canvas) : ED.runs.create(ed, canvas);
        publish = ED.publish.create(ed);
        authoring = !isBlock && ED.authoring && ED.authoring.create(ed);
        body.appendChild(palette.el);
        body.appendChild(canvas.el);
        body.appendChild(canvas.listRegion);
        workspace = ED.workspace && ED.workspace.create(ed, canvas, palette);
        // Undo/Redo in the header and the "deleted – Undo" toast (easydrag-history.js).
        history = ED.history.create(ed, {
            host: el.querySelector('[data-ed-history-host]'), toastHost: body,
            undo: () => interact.undo(), redo: () => interact.redo(),
            focusCanvas: () => canvas.el.focus({ preventScroll: true })
        });

        // The header and the footer (name, state and revision chips, actions, save state, issues,
        // last run) render in easydrag-header.js. The run view's banner stays here; renderHeader calls it.
        const headerView = ED.header.create({
            ed, el, isBlock, canvas, runs, palette, publish, nameInput, renderBanner,
            isDisposed: () => disposed, lastRecord: () => lastRecord
        });
        const { renderHeader, renderSaveState, renderIssues, renderLastRun } = headerView;


        ed.saver = ED.saver.create({
            api: ed.api, flowId: ed.flow.id, model: draftModel, revision: ed.flow.draft_revision,
            onState: renderSaveState,
            // An edit made while the request ran is still unsaved: the saver sends it next.
            onSaved: ({ revision }) => { ed.flow.draft_revision = revision; contentDirty = ed.saver.isDirty(); renderHeader(); },
            onInvalid: issues => { ed.issues = issues; ed.bus.emit('issues', issues); },
            onConflict: () => ED.dialogs.conflict(ed)
        });
        extraction = !isBlock && ED.extraction && ED.extraction.create(ed, canvas);
        ed.credentialsMove = !isBlock && ED.credentialsMove ? ED.credentialsMove.create(ed) : null;
        ed.runtime = !isBlock && ED.durable && ED.durable.supported(ed) ? ED.durable.observe(ed) : null;
        ed.childHistory = ED.childHistory ? ED.childHistory.create(ed) : null;

        // ── run view ────────────────────────────────────────────────────────────

        // renderBanner names the viewed run and offers Stop while it has not ended (not on a
        // read-only desktop). A Stop that goes away hands its focus to "Back to draft". The text
        // changes only with the run: the banner is a live region, and every header render calls this.
        function renderBanner() {
            const run = ed.runView && ed.runView.run;
            banner.hidden = !run;
            if (!run) return;
            const text = t('easydrag.ui.run_view_banner', {
                status: ed.runView.stopped ? ED.status.word(t, 'stopped') : ED.durable.label(t, ed.run || run),
                time: core.fmt.dateTime(run.started_at),
                revision: run.revision
            });
            const node = banner.querySelector('[data-ed-runview-text]');
            const shown = text; if (node.textContent !== shown) node.textContent = shown; node.title = ed.runView.stopped ? t('easydrag.ui.run_stopped_by', { step: ed.runView.stopped.label }) : '';
            const stop = banner.querySelector('[data-ed-cmd="stop-viewed"]');
            const approval = banner.querySelector('[data-ed-cmd="approval-review"]');
            const durableStatus = ED.durable && ED.durable.info(ed.run);
            const gateId = durableStatus && durableStatus.approval_gate_id;
            approval.hidden = !gateId;
            if (approval.hidden && document.activeElement === approval) banner.querySelector('[data-ed-cmd="exit-run-view"]').focus();
            // Stop shows on a read-only desktop too (a stop is no write); nothing else does.
            const off = !!ed.run?.durableStatusError || (durableStatus ? !durableStatus.cancel_available : !!(run.durable) || !ED.runs.isActive(run.status));
            if (off && document.activeElement === stop) banner.querySelector('[data-ed-cmd="exit-run-view"]').focus();
            stop.hidden = off;
            banner.querySelector('[data-ed-cmd="rerun-viewed"]').hidden = !canRerun(run);
        }

        // canRerun: the viewed run brought its trigger data (GET runs/{id} returns it), the draft still
        // has that trigger, switched on, and the desktop may test.
        function canRerun(run) {
            if (!run || ed.readonly) return false;
            const data = run.trigger_data;
            if (!data || typeof data !== 'object' || Array.isArray(data)) return false;
            // The store keeps oversized trigger data as a stub ({_preview} or {_omitted}): testing it would run real effects on the stub.
            if ('_preview' in data || '_omitted' in data) return false;
            // A run is going (the viewed one or a parked live run): leaving the view would lose it, and no test can start.
            if (ED.runs.isActive(run.status) || runs.hasParked()) return false;
            const n = draftModel.node(run.trigger_node);
            const i = n && draftModel.info(n.type);
            return !!(i && i.trigger && !(n.settings && n.settings.disabled));
        }

        // rerunViewed leaves the run view and opens the test dialog with the viewed run's trigger data.
        // The dialog lists the real effects and asks before anything runs, as every test does.
        function rerunViewed() {
            const run = ed.runView && ed.runView.run;
            if (!canRerun(run)) {
                if (run && (ED.runs.isActive(run.status) || runs.hasParked())) ctx.notify({ title: draftModel.doc.name, message: t('easydrag.ui.run_going') });
                return;
            }
            const triggerNode = run.trigger_node;
            const data = JSON.parse(JSON.stringify(run.trigger_data));
            exitRunView();
            if (ed.runView) return;
            runs.startTest({ triggerNode, data });
        }

        function enterRunView(detail) {
            if (!ed.flushPendingEdits()) return;
            if (ed.runView && ED.approvals) ED.approvals.closeRunReview(ed);
            ED.detail.close(ed);
            ED.palette.closeQuickAdd(ed);
            if (!ed.runView) savedView = Object.assign({}, ed.view);
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
            renderIssues();
        }

        function exitRunView() {
            if (!ed.runView) return;
            if (!ed.flushPendingEdits()) return;
            if (ED.approvals) ED.approvals.closeRunReview(ed);
            ED.detail.close(ed);
            ed.runView = null;
            ed.model = draftModel;
            ed.selection = new Set();
            el.classList.remove('is-run-view');
            runs.clearRun();
            ed.bus.emit('model', { kind: 'reset', nodes: [], edges: [], meta: true, structural: true });
            if (savedView) canvas.setView(savedView, { animate: true });
            // A docked palette comes back by the stored choice; a floating one stays closed.
            palette.restoreDocked();
            renderHeader();
            setMenus();
            renderIssues();
            // The hints were not checked while the run view showed (an edit there, a restore).
            publish.refreshIssues();
            // The banner and its buttons are gone: keyboard focus goes to the draft's canvas.
            canvas.el.focus({ preventScroll: true });
        }

        // ── commands ────────────────────────────────────────────────────────────

        async function saveNow() {
            if (ed.readonly || ed.runView) return;
            if (!ed.flushPendingEdits()) return;
            await ed.saver.save();
            if (ed.saver.state === 'saved') canvas.announce(t('easydrag.ui.save_saved'));
        }

        function test() {
            if (!ed.flushPendingEdits()) return;
            if (!runs.isRunning()) runs.startTest({});
        }

        function publishNow() { if (ed.flushPendingEdits()) publish.openDialog(); }
        function runLiveNow() { if (!isBlock && ed.flushPendingEdits()) runs.runLive(); }

        // The palette stays hidden in the run view; its toggle and the search key wait until it ends.
        function togglePalette() { if (!ed.runView) palette.setOpen(!palette.isOpen()); }
        function searchPalette() {
            if (ed.runView || !ed.flushPendingEdits()) return;
            const one = ed.selection.size === 1 ? Array.from(ed.selection)[0] : null;
            const src = one && ed.model.node(one);
            const outs = src && typeof ed.model.outputs === 'function' ? ed.model.outputs(src) : [];
            if (src && outs.length && !ed.readonly) {
                const card = canvas.nodeEl(one);
                const rect = card && card.getBoundingClientRect ? card.getBoundingClientRect() : null;
                const x = rect && Number.isFinite(rect.right) ? rect.right + 24 : 0;
                const y = rect && Number.isFinite(rect.top) ? rect.top : 0;
                ED.palette.openQuickAdd(ed, canvas, { x, y, from: { node: one, port: outs[0] } });
                setMenus();
                return;
            }
            palette.focusSearch();
        }

        function exportFlow() {
            if (isBlock) return;
            const a = document.createElement('a');
            a.href = ed.api.exportUrl(ed.flow.id);
            // The export is the draft, also in the run view: it carries the draft's name.
            a.download = (draftModel.doc.name || 'flow') + '.easydrag.json';
            document.body.appendChild(a);
            a.click();
            a.remove();
        }

        function exportLocalPack() { if (ED.pack) return ED.pack.exportEditor(ed); }

        // deleteFlow deletes the flow; while the request runs, its own "deleted" broadcast is not
        // news (deleting).
        async function deleteFlow() {
            if (isBlock) return ED.blocks.remove(ed, () => { core.forgetFlow(ed.flow.id); app.openHome(); });
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

        // duplicating: one copy at a time; a double click (or the menu and the key) copies once.
        let duplicating = false;
        async function duplicateFlow() {
            if (duplicating) return;
            duplicating = true;
            try {
                if (!(await leave()) || disposed) return;
                // The draft, also in the run view (whose model is a stored run's document).
                const doc = draftModel.toJSON();
                doc.name = t('easydrag.ui.copy_of', { name: doc.name });
                const res = await ed.api.create({ import: doc });
                app.openFlow(res.flow.id);
            } catch (err) {
                ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
            } finally {
                duplicating = false;
            }
        }

        // leave saves pending content before the editor closes; false keeps it open. A drag still in
        // progress is cancelled first (its nodes move back), so no mid-drag state is saved. Calls that
        // overlap (two navigations waiting on a failed save) share one answer: at most one dialog asks.
        let leaving = null;
        function leave() {
            if (!leaving) leaving = leaveOnce().finally(() => { leaving = null; });
            return leaving;
        }

        // flushPendingEdits writes the open step's field and note. False keeps the text in place.
        function flushPendingEdits() {
            const detail = ed.detail;
            if (!detail) return true;
            try {
                if (typeof detail.flushPendingEdits === 'function') return detail.flushPendingEdits() !== false;
                if (typeof detail.flushNote === 'function') detail.flushNote();
                return true;
            } catch (err) {
                if (typeof ctx.notify === 'function') ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' });
                return false;
            }
        }
        ed.flushPendingEdits = flushPendingEdits;
        emailBinding = !isBlock && window.AuraDesktopEmail?.attach(app.windowId, () => null, () => ({
            key:disposed?null:ed.flow.id,disabled:disposed||restoring||deleting,
            subject:draftModel.doc.name,formats:[{id:'json',label:'.easydrag.json'}],
            createFiles:async(_format,signal)=>{
                if(!ed.flushPendingEdits()||!await ed.saver.flush())return false;
                return window.AuraDesktopEmail.files([{url:ed.api.exportUrl(ed.flow.id),name:(draftModel.doc.name||'flow')+'.easydrag.json'}],signal);
            }
        }));

        async function leaveOnce() {
            interact.abortGesture();
            // A pending field or note is written before the saver runs. A failed check keeps the editor open.
            if (!ed.flushPendingEdits()) return false;
            if (extraction && extraction.pending()) return extraction.leave();
            if (ed.readonly || !contentDirty) return true;
            const ok = await ed.saver.flush();
            if (ok) return true;
            return ctx.confirmDialog(t('easydrag.ui.leave_title'), t('easydrag.ui.leave_text'));
        }

        // headerNarrow: the editor is 560 px wide or narrower, where the stylesheet hides Runs, Test and
        // Run now in the header (@container max-width 560px) and the ⋯ menu offers them.
        function headerNarrow() {
            const width = el.getBoundingClientRect().width;
            return width > 0 && width <= 560;
        }

        const MORE_MENU = ['flow.run_now', 'flow.versions', 'flow.settings', 'flow.organize', 'flow.describe', 'flow.repair_mission', 'flow.duplicate', 'flow.export',
            'flow.export_pack', 'flow.export_email', 'flow.mission_control', 'view.liveliness', '-', 'flow.delete'];
        const BLOCK_MORE_MENU = ['flow.versions', 'flow.open_draft', 'view.liveliness', 'flow.delete'];

        function moreMenu(anchor) {
            const r = anchor.getBoundingClientRect();
            const moved = !isBlock && headerNarrow() ? ['view.runs', 'flow.test', '-'] : [];
            ctx.showContextMenu(r.left, r.bottom + 4, ED.actions.items(isBlock ? BLOCK_MORE_MENU : moved.concat(MORE_MENU), { host: ed }));
        }

        async function repairMission() {
            if (ed.readonly || ed.runView || !ed.flushPendingEdits() || !(await ed.saver.flush())) return;
            const revision = ed.flow.draft_revision, liveRevision = ed.flow.live_revision;
            core.modal(ed.root, {
                title: t('easydrag.ui.repair_mission'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.repair_mission_hint')) + '</p>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'repair', label: t('easydrag.ui.repair_mission'), primary: true }],
                onAction: async action => {
                    if (action !== 'repair') return true;
                    try {
                        await ed.api.repairMission(ed.flow.id, revision, liveRevision);
                        if (disposed) return true;
                        ed.publishIncomplete = ''; ed.flowEnabled = false;
                        await refreshRecord();
                        renderHeader(); setMenus();
                        canvas.announce(t('easydrag.ui.repair_mission_done'));
                        return true;
                    } catch (err) { if (!disposed) ctx.notify({ title: 'EasyDrag', message: core.errorText(t, err), type: 'error' }); return false; }
                }
            });
        }

        // ── actions and window menus ────────────────────────────────────────────

        // The registry (easydrag-actions.js) holds every action's label, shortcut and reasons. The editing and zoom
        // actions are bound in easydrag-interact.js, the flow and view actions here. The desktop runs a window-menu
        // item's shortcut before onKeyDown sees the key, past onKeyDown's guards, and prevents it: Ctrl+S, Ctrl+Enter
        // and Ctrl+K are real menu shortcuts (Ctrl+K so that the desktop's search does not take it) and do nothing
        // while a dialog, the restore offer or a desktop context menu is open (the shell runs them before onKeyDown
        // can see that menu: the registry's "free" check reads actionFacts). Edits also wait for quick-add ("quiet").
        // Every other shortcut is only drawn by the menus and fired by ED.actions.key in onKeyDown. The step
        // inspector is not a dialog.
        function modalOpen() { return !!el.querySelector('.ed-modal-backdrop:not(.is-closing)'); }
        function contextMenuOpen() { return !!document.querySelector('.vd-context-menu:not(.vd-context-menu-closing)'); }
        ed.actionFacts = () => ({ restoring, dialog: modalOpen() || contextMenuOpen(), running: runs.isRunning() });

        const bind = (id, impl) => ED.actions.bind(id, impl, ed);
        const noPalette = () => (ed.runView ? 'easydrag.ui.reason_palette_hidden' : '');
        bind('flow.home', { run: () => goHome() });
        bind('flow.save', { run: () => saveNow() });
        bind('flow.test', { run: () => test(), label: () => (isBlock ? 'easydrag.ui.block_simulate' : '') });
        bind('flow.stop', { run: () => runs.cancel(), check: () => (ed.run?.durableStatusError || el.querySelector('[data-ed-cmd="cancel"]').getAttribute('aria-busy') === 'true' ? 'easydrag.ui.reason_stop_unavailable' : '') });
        bind('flow.publish', { run: () => publishNow() });
        bind('flow.versions', { run: () => (isBlock ? ED.blocks.versions(ed, app) : publish.openVersions()) });
        bind('flow.export_pack', { run: () => exportLocalPack(), check: () => (ed.readonly && ed.saver.isDirty() ? 'easydrag.ui.reason_export_unsaved' : '') });
        bind('flow.mission_control', { run: () => ctx.openApp && ctx.openApp('mission-control') });
        bind('flow.delete', { run: () => deleteFlow(), label: () => (isBlock ? 'easydrag.ui.block_delete' : '') });
        bind('view.find_step', { run: () => ED.dialogs.findStep(ed, canvas, interact), check: () => (ed.model.doc.nodes.length ? '' : 'easydrag.ui.reason_no_steps') });
        // Ctrl+K adds a step from one selected step, and searches otherwise (searchPalette).
        bind('view.search', { run: () => searchPalette(), check: noPalette, label: () => (ed.selection.size === 1 && !ed.quickAdd ? 'easydrag.ui.keys_add' : '') });
        bind('view.palette', { run: () => togglePalette(), check: noPalette, checked: () => palette.isOpen() });
        bind('view.shortcuts', { run: () => ED.dialogs.shortcuts(ed) });
        if (isBlock) bind('flow.open_draft', { run: () => app.openBlock(ed.flow.id) });
        else {
            bind('flow.run_now', { run: () => runLiveNow() });
            bind('flow.active', { run: () => publish.setActive(!ed.flowEnabled) });
            bind('flow.settings', { run: () => ED.dialogs.flowSettings(ed) });
            bind('flow.duplicate', { run: () => duplicateFlow() });
            bind('flow.export', { run: () => exportFlow() });
            bind('flow.export_email', { run: () => emailBinding.activate(), show: () => !!emailBinding, check: () => (ed.readonly ? 'easydrag.ui.reason_readonly' : '') });
            bind('flow.repair_mission', { run: () => repairMission(), show: () => ed.publishIncomplete === 'FLOW_MISSION_MISSING' || ed.liveIssues.some(issue => issue.code === 'FLOW_MISSION_MISSING') });
            bind('view.runs', { run: () => { runs.toggleDrawer(); renderHeader(); setMenus(); }, checked: () => runs.drawerOpen() });
            if (ED.metadata) bind('flow.organize', { run: () => ED.metadata.organization(Object.assign({}, ed, { readonly: !!(ed.readonly || ed.runView), canWrite: () => !disposed && !ed.readonly && !ed.runView }), ed.flow.id) });
            if (authoring) bind('flow.describe', { run: () => authoring.open() });
        }

        // An action that is not bound here (Run now in a block, Extract in a block) is left out of the menus.
        const FLOW_MENU = ['flow.home', '-', 'flow.save', 'flow.test', 'flow.run_now', 'flow.publish', 'flow.versions', '-', 'flow.settings', 'flow.export', 'flow.export_pack', 'flow.delete'];
        const EDIT_MENU = ['edit.undo', 'edit.redo', '-', 'edit.cut', 'edit.copy', 'edit.paste', 'edit.duplicate', 'edit.extract', 'edit.delete', '-', 'edit.select_all', 'edit.auto_layout'];
        const VIEW_MENU = ['view.zoom_in', 'view.zoom_out', 'view.zoom_fit', '-', 'view.find_step', 'view.search', 'view.palette', 'view.runs', '-', 'view.shortcuts'];

        function setMenus() {
            if (disposed || typeof ctx.setWindowMenus !== 'function') return;
            const c = { host: ed, keyed: true };
            ctx.setWindowMenus(ed.windowId, [
                { id: 'flow', labelKey: 'easydrag.ui.menu_flow', items: ED.actions.items(FLOW_MENU, c) },
                { id: 'edit', labelKey: 'easydrag.ui.menu_edit', items: ED.actions.items(EDIT_MENU, c) },
                { id: 'view', labelKey: 'easydrag.ui.menu_view', items: ED.actions.items(VIEW_MENU, c) }
            ]);
        }
        const setMenusSoon = core.debounce(setMenus, 120);
        // Dialogs, the detail view, quick-add and the run drawer open as children of the editor:
        // the menus follow them (disabled edits, checked drawer).
        if (typeof MutationObserver === 'function') {
            const overlays = new MutationObserver(() => { setMenusSoon(); headerView.paintActions(); history.sync(true); interact.paintTools(); });
            overlays.observe(el, { childList: true });
            bag.add(() => overlays.disconnect());
        }

        // ── events ──────────────────────────────────────────────────────────────

        // Every change of the draft is saved, also one made while the run view shows another
        // document (a restore answered there); the rest follows the model shown. Pans and zooms
        // are no changes: the view is stored per device (storeView), never in the document.
        bag.add(draftModel.on(change => {
            if (!sessionActionFlushing) sessionEditRevision++;
            contentDirty = true;
            ed.saver.schedule();
            if (ed.model !== draftModel) return;
            ed.bus.emit('model', change);
            publish.refreshIssues();
            if (change.meta || change.kind !== 'change') renderHeader();
            if (change.structural || change.kind !== 'change') pruneSelection();
            setMenusSoon();
        }));

        // A stored draft whose sticky notes or group texts normalize() cut or cleaned (UI-11) is
        // saved right away: the server refuses such a draft, and after this save the notice does
        // not return. One notice per editor; a reset (restore, conflict reload) that brings such
        // a document again saves it again.
        let notesTrimNoted = false;
        function saveTrimmedNotes() {
            if (ed.readonly || !draftModel.repairs().notesTrimmed) return;
            if (!notesTrimNoted) {
                notesTrimNoted = true;
                if (typeof ctx.notify === 'function') ctx.notify({ title: 'EasyDrag', message: t('easydrag.ui.notes_trimmed') });
            }
            contentDirty = true;
            ed.saver.schedule();
        }
        saveTrimmedNotes();
        bag.add(draftModel.on(change => { if (change && change.kind === 'reset') saveTrimmedNotes(); }));

        function pruneSelection() {
            const before = ed.selection.size;
            ed.selection.forEach(id => { if (!ed.model.node(id)) ed.selection.delete(id); });
            if (ed.selectedEdge && !ed.model.edge(ed.selectedEdge)) ed.selectedEdge = null;
            if (ed.selection.size !== before) ed.bus.emit('selection', ed.selection);
        }

        // A drag holds back emergency copies (at most one per 500 ms); its end writes the last one,
        // and so does a page that goes away or into the background (closed, reloaded, switched
        // away from on a phone), where no later timer may run. A note still waiting for its
        // 400 ms in the step dialog goes into the draft first, so the copy holds it.
        const pageAway = () => {
            ed.flushPendingEdits();
            ed.saver.flushCopy();
        };
        bag.add(ed.bus.on('gesture-end', () => ed.saver.flushCopy()));
        bag.listen(window, 'pagehide', pageAway);
        bag.listen(document, 'visibilitychange', () => { if (document.visibilityState === 'hidden') pageAway(); });
        bag.add(ed.bus.on('quick-add', req => { if (!ed.readonly && !ed.runView) { ED.palette.openQuickAdd(ed, canvas, req); setMenus(); } }));
        bag.add(ed.bus.on('open-detail', req => {
            if (!ed.flushPendingEdits()) return;
            if (ed.groups) ed.groups.expandFor(req.nodeId);
            ED.detail.open(ed, req.nodeId, { param: req.param, column: req.column });
            setMenus();
        }));
        bag.add(ed.bus.on('detail-closed', () => { canvas.el.focus({ preventScroll: true }); setMenus(); }));
        bag.add(ed.bus.on('node-test', req => { if (!isBlock) runs.startTest({ onlyNode: req.nodeId }); }));
        bag.add(ed.bus.on('focus-node', id => { interact.select([id]); canvas.centerOn(id, { animate: true }); }));
        bag.add(ed.bus.on('connect-picker', req => ED.dialogs.connectPicker(ed, canvas, req.nodeId, req)));
        bag.add(ed.bus.on('find-step', () => ED.dialogs.findStep(ed, canvas, interact)));
        bag.add(ed.bus.on('selection', setMenusSoon));
        bag.add(ed.bus.on('palette', () => { renderHeader(); setMenusSoon(); }));
        bag.add(ed.bus.on('issues', renderIssues));
        bag.add(ed.bus.on('run', () => { renderHeader(); renderLastRun(); setMenusSoon(); }));
        bag.add(ed.bus.on('last-run', record => { lastRecord = record; renderLastRun(); }));
        bag.add(ed.bus.on('published', flow => {
            ed.flow = Object.assign(ed.flow, flow);
            // A partial publish remembers its live revision: only a higher one finishes it (refreshRecord).
            partialRevision = ed.publishIncomplete ? Number(ed.flow.live_revision) || 0 : 0;
            renderHeader();
            setMenus();
            if (ed.credentialsMove) ed.credentialsMove.afterPublish();
        }));
        bag.add(ed.bus.on('restored', res => {
            if (!res || !res.flow) return;
            if (ed.runView) exitRunView();
            ed.flow = Object.assign(ed.flow, res.flow);
            draftModel.replaceDoc(res.flow.draft);
            contentDirty = false;
            if (ed.saver) {
                ed.saver.revision = res.flow.draft_revision;
                ed.saver.markSaved();
                ED.saver.dropEmergencyCopy(ed.flow.id);
            }
            ed.issues = res.issues || [];
            ed.bus.emit('issues', ed.issues);
            renderHeader();
            setMenus();
            canvas.fit({ animate: true, readable: true });
        }));
        // The Active switch also decides whether Run now is offered.
        bag.add(ed.bus.on('enabled', () => { renderHeader(); setMenus(); }));
        bag.add(ed.bus.on('proposal-applied', () => { ED.detail.close(ed); canvas.fit({ readable: true }); }));
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
            else if (cmd === 'publish') publishNow();
            else if (cmd === 'run-now') runLiveNow();
            else if (cmd === 'active') publish.setActive(!ed.flowEnabled);
            else if (cmd === 'more') moreMenu(b);
            else if (cmd === 'exit-run-view') exitRunView();
            else if (cmd === 'approval-review') {
                const gateId = ED.durable && ED.durable.info(ed.run)?.approval_gate_id;
                if (gateId && ED.approvals) ED.approvals.openRunReview(ed, gateId, { runId: ed.runView?.run.id, flowId: ed.flow.id, revision: ed.runView?.run.revision });
            }
            else if (cmd === 'stop-viewed') runs.stopRun(ed.runView && ed.runView.run);
            else if (cmd === 'rerun-viewed') rerunViewed();
            else if (cmd === 'last-run') { if (ed.run || lastRecord) runs.toggleDrawer(true); renderHeader(); }
            else if (cmd === 'issues') publish.openIssues(b);
            else if (cmd === 'keys') ED.dialogs.shortcuts(ed);
            else if (cmd === 'palette') togglePalette();
            else if (cmd === 'templates') goHome({ section: 'templates' });
            else if (cmd === 'groups' && ed.groups) ed.groups.openList();
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
            // Mod+S belongs to the editor even when it cannot save (a real dialog or a context
            // menu is open, or Save is disabled): the browser's "Save Page" never opens from here.
            // It saves only when the desktop did not run the Save item (prevented keys return
            // above) and Save is enabled (saveNow checks read-only and the run view; the restore
            // offer here). The step inspector does not block these keys. Under a real EasyDrag
            // dialog Ctrl+Enter and Ctrl+K are prevented too (Chrome takes Ctrl+K to the address
            // bar). Text fields keep native undo and IME. No dialog field uses Mod+Enter.
            if (mod && key === 's') event.preventDefault();
            const dialogOpen = !!el.querySelector('.ed-modal-backdrop');
            if (dialogOpen && mod && (key === 'k' || key === 'f' || event.key === 'Enter')) event.preventDefault();
            if (dialogOpen || document.querySelector('.vd-context-menu')) return;
            // The registry's shortcuts (ED.actions.key) keep the rules each key had: Ctrl+S, Ctrl+Enter, Ctrl+K and Ctrl+F also
            // work from a text field, Alt+Shift+F from outside one, and the rest only while the canvas or a card has the focus
            // and quick-add is closed. A refused action (read-only, a run in progress ...) still takes its key.
            const active = document.activeElement;
            const onCanvas = canvas.el.contains(active) || active === document.body || active === el;
            const control = !!(event.target && event.target !== canvas.el && event.target.closest && event.target.closest('button, a, input, textarea, select'));
            if (ED.actions.key(event, { host: ed, editable: core.isEditable(target), quickAdd: !!ed.quickAdd, onCanvas, control })) return;
            if (core.isEditable(target) || ed.quickAdd) return;
            if (event.key === 'Escape' && ed.runView) { event.preventDefault(); exitRunView(); return; }
            if (onCanvas && interact.handleKey(event)) event.preventDefault();
        }
        bag.listen(document, 'keydown', onKeyDown);
        // Search is disabled while a dialog or the restore offer is open, so the desktop would take Ctrl+K for its own
        // Spotlight: the capture listener swallows the key first (the menus still say why Search is off).
        bag.listen(document, 'keydown', event => {
            if (disposed || !el.isConnected || typeof event.key !== 'string' || event.key.toLowerCase() !== 'k') return;
            if (typeof ctx.isActive === 'function' && !ctx.isActive()) return;
            if ((modalOpen() || restoring) && core.isMod(event)) { event.preventDefault(); event.stopPropagation(); }
        }, true);
        bag.listen(document, 'keyup', event => interact.handleKeyUp(event));
        // Deletes and switches from Mission Control and the missions page name the flow (flow_id).
        // One without an id (no single flow held the mission) may still be this flow: the record
        // is read again, and FLOW_NOT_FOUND means it was deleted.
        bag.listen(document, 'aurago:flows-changed', (event) => {
            const d = event.detail || {};
            const mine = d.flow_id === ed.flow.id;
            const maybe = !d.flow_id && (d.reason === 'deleted' || d.reason === 'enabled' || d.reason === 'checked');
            if (!mine && !maybe) return;
            if (d.reason === 'deleted') {
                if (deleting) return;
                if (mine) goneElsewhere(); else refreshRecord();
                return;
            }
            // A live run ended: the last run, the drawer and a run view follow (debounced in runs).
            if (d.reason === 'run_finished') runs.runFinished();
            if (d.reason === 'enabled' || d.reason === 'published' || d.reason === 'checked') refreshRecord();
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
            ed.liveIssues = Array.isArray(res.live_issues) ? res.live_issues : [];
            ed.liveIssuesRevision = Number(res.flow.live_revision) || 0;
            ed.timersRunning = res.timers_running !== false;
            if (ed.publishIncomplete && Number(res.flow.live_revision) > partialRevision) ed.publishIncomplete = '';
            ed.flowEnabled = !!res.enabled;
            renderHeader();
            setMenus();
        }

        if (typeof ctx.setWindowBeforeClose === 'function') ctx.setWindowBeforeClose(ed.windowId, leave);
        if (typeof ctx.registerSessionAction === 'function') {
            const sessionContentRevision = () => JSON.stringify([ed.flow.id, sessionEditRevision]);
            ed.sessionActionOff = ctx.registerSessionAction(ed.windowId, {
                inspect: () => ({
                    dirty: !!(ed.saver?.isDirty() || contentDirty || ed.detail?.hasPendingEdits?.()),
                    revision: sessionContentRevision(),
                    label: draftModel.doc.name || 'EasyDrag'
                }),
                decide: snapshot => ctx.decideSessionAction?.({ title: snapshot.title, label: snapshot.label, dirty: snapshot.dirty }),
                save: async expectedRevision => {
                    if (disposed || deleting || ed.readonly || ed.runView || !ed.saver ||
                        sessionContentRevision() !== expectedRevision) return false;
                    sessionActionFlushing = true;
                    let flushed = false;
                    try { flushed = ed.flushPendingEdits(); }
                    finally { sessionActionFlushing = false; }
                    if (!flushed) return false;
                    if (sessionContentRevision() !== expectedRevision) return false;
                    if (!(await ed.saver.flush())) return false;
                    return !ed.saver.isDirty() && !contentDirty &&
                        sessionContentRevision() === expectedRevision
                        ? { revision: expectedRevision }
                        : false;
                }
            });
        }

        // ── start ───────────────────────────────────────────────────────────────

        // start offers an emergency copy that differs from the server draft, also one made on an
        // older server revision (the dialog warns that it replaces the newer draft; the restore
        // saves against the current revision, the saver's). Only an explicit "discard" drops it;
        // any other answer (null: the dialog closed with the editor) keeps the copy and the draft
        // as they are, so the offer comes again when the flow opens next.
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
                // This explicit replacement must also clear groups absent from an older copy.
                if (choice === 'restore') draftModel.replaceDoc(Object.assign({}, copy.doc, { groups: copy.doc.groups == null ? [] : copy.doc.groups }));
                setMenus();
            } else if (copy) {
                ED.saver.dropEmergencyCopy(ed.flow.id);
            }
            if (isBlock) return;
            if (opts && opts.runId) runs.openRunView(opts.runId, opts.gateId);
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
            if (ed.runtime) ed.runtime.refresh();
            canvas.el.focus({ preventScroll: true });
        });
        setMenus();
        if (!ed.issues.length) publish.refreshIssues();
        start();

        // dispose saves pending content last: the detail view flushes its note on close, and
        // interact cancels a drag in progress (its nodes move back), so neither is lost nor
        // saved in a mid-drag state. Each step runs on its own: one that throws cannot keep
        // the saver, the listeners or the window's close guard from being released.
        // It is also the cleanup of a build that failed: modules not created yet are skipped.
        function dispose() {
            if (disposed) return;
            disposed = true;
            const safely = fn => { try { fn(); } catch (err) { console.error('EasyDrag editor cleanup failed', err); } };
            safely(() => { if (ED.approvals) ED.approvals.closeRunReview(ed); });
            safely(() => { if (ED.metadata) ED.metadata.dispose(el); });
            safely(() => ED.detail.close(ed));
            safely(() => ED.palette.closeQuickAdd(ed));
            safely(() => { if (interact) interact.dispose(); });
            safely(() => { if (contentDirty && ed.saver) ed.saver.save(); });
            safely(() => storeView.flush());
            [ed.childHistory, ed.runtime, extraction, blockTracker, authoring, workspace, wires, canvas, palette, runs, publish, history].forEach(m => { if (m) safely(() => m.dispose()); });
            if (emailBinding) safely(() => emailBinding.dispose());
            safely(() => { if (ed.saver) ed.saver.dispose(); });
            ed.sessionActionEdit = null;
            ed.sessionActionCommit = null;
            safely(() => setMenusSoon.cancel());
            safely(() => bag.dispose());
            safely(() => ed.sessionActionOff?.());
            if (typeof ctx.setWindowBeforeClose === 'function') safely(() => ctx.setWindowBeforeClose(ed.windowId, null));
            el.remove();
        }
        return {
            el, ed, leave,
            showRun(runId, gateId) { runs.openRunView(runId, gateId); },
            dispose
        };
    }

    ED.editor = { create };
})();
