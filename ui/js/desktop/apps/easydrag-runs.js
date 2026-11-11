// EasyDrag runs: test dialog (trigger, sample data, real-effects warning), live run stream
// (SSE with reconnect), step and wire states, run drawer and read-only run view, stopping their runs.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const EFFECTS_KEY = 'aurago.easydrag.effects-ok.';
    // A failed or refused stream (429 FLOW_RUN_LIMIT) reconnects after 1.5 s, doubling up to 15 s.
    const RETRY_MS = 1500;
    const RETRY_MAX_MS = 15000;
    // After MAX_FAILURES failures in a row without a message the server is asked about the run.
    const MAX_FAILURES = 5;
    // After a stop of a run that streams elsewhere (or nowhere), its stored state is asked again
    // after 1, 2, 4, 8 and 16 s until it ended: a test run announces its end to no other window,
    // and the engine waits up to 30 s for a step that ignores the cancel.
    const STOP_POLL_MS = [1000, 2000, 4000, 8000, 16000];
    // While a live run is parked behind a run view its stored state is asked every PARK_POLL_MS: its stream is closed, so it ends unseen.
    const PARK_POLL_MS = 3000;
    // REDACTED is the server's placeholder for secret values in scrubbed test data.
    const REDACTED = '[redacted]';
    const EFFECT_ICONS = { sends_message: 'brand-telegram', writes_files: 'file-pencil', controls_devices: 'home', runs_code: 'api', deletes: 'trash', system_change: 'settings' };

    function isFinal(status) { return ['success', 'error', 'cancelled'].includes(status); }
    function isActive(status) { return ['queued', 'waiting', 'running'].includes(status); }

    const triggers = (...args) => ED.testing.triggers(...args);
    const realEffects = (...args) => ED.testing.realEffects(...args);
    const effects = (...args) => ED.testing.effects(...args);
    const unknownEffects = (...args) => ED.testing.unknownEffects(...args);

    function edgeStates(ed) {
        const edges = new Map();
        ed.model.doc.edges.forEach(e => {
            const src = ed.run.steps.get(e.source.node);
            const dst = ed.run.steps.get(e.target.node);
            if (!src) return;
            if (src.status === 'skipped' || (src.status === 'success' && Array.isArray(src.ports) && !src.ports.includes(e.source.port))) { edges.set(e.id, { state: 'skipped' }); return; }
            if (src.status === 'success' || (src.status === 'error' && e.source.port === 'error')) {
                const flowing = dst && dst.status === 'running';
                edges.set(e.id, { state: flowing ? 'flowing' : 'delivered', count: src.item_count || 0 });
            }
        });
        return edges;
    }

    function create(ed, canvas) {
        const core = ED.core;
        const { t, esc } = ed;
        const bag = core.bag();

        // stopButtons are every control for this still-active run: drawer, header and run view.
        function stopButtons(id) {
            if (!ed.root || !ed.root.querySelectorAll) return [];
            return Array.from(ed.root.querySelectorAll('[data-ed-run-stop], [data-ed-cmd="cancel"], [data-ed-cmd="stop-viewed"]')).filter(b => {
                if (b.dataset.edRunStop) return b.dataset.edRunStop === String(id);
                if (b.dataset.edCmd === 'cancel') return !!(ed.run && ed.run.id === id && !ed.run.view);
                return !!(ed.runView && ed.runView.run && ed.runView.run.id === id);
            });
        }

        // rc is the run core. It holds the state that the run parts share (easydrag-run-stream.js,
        // -start.js, -stop.js and -drawer.js), the helpers and limits they use and, once a part is
        // installed, that part's functions: a part reaches state and the other parts through rc only.
        const rc = {
            ed, canvas, core, t, esc, stopButtons,
            isFinal, isActive, edgeStates, triggers, realEffects, effects, unknownEffects,
            RETRY_MS, RETRY_MAX_MS, MAX_FAILURES, STOP_POLL_MS, PARK_POLL_MS, EFFECTS_KEY, REDACTED, EFFECT_ICONS,
            // The live run's stream: its EventSource, the last event seen, the reconnect timer, the
            // backoff and the failures in a row without a message.
            source: null,
            lastSeq: 0,
            reconnectTimer: 0,
            retryDelay: RETRY_MS,
            failures: 0,
            // failedSpoken holds the steps of the live run whose failure was announced: a step that
            // reports again (the snapshot of a reconnect) is not announced twice. Steps that went well
            // are not announced at all; the end of the run is (finish).
            failedSpoken: new Set(),
            // parked is a live run that a run view replaced; clearRun() attaches it again.
            parked: null,
            parkTimer: 0,
            // starting is true from a startTest call until its run was posted or its dialog closed:
            // one test at a time, and none while a run is active (it would orphan that run's stream).
            starting: false,
            // startPost is the test or live POST in flight. A second call waits on the same
            // promise and does not send again. An uncertain result is not sent again.
            startPost: null,
            // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
            stopping: new Set(),
            durableStopping: new Set(),
            // stopPolls holds the timers of followStop.
            stopPolls: new Set(),
            // The run view: the run being opened (openRunView) and the numbers of the latest open and
            // of the latest refresh (refreshView), so that late replies cannot replace a newer view.
            viewing: null,
            viewSeq: 0,
            viewRefreshSeq: 0,
            // The run drawer: its element, the control that opened it, its filter and search, its
            // latest request and paging, and the runs it lists by id (for their Stop buttons).
            drawer: null,
            drawerOpener: null,
            drawerFilter: 'all',
            runsQuery: '',
            runsSeq: 0,
            runsOffset: 0,
            runsMore: false,
            runsLoading: false,
            listedRuns: new Map(),
            disposed: false
        };

        Object.assign(rc, ED.runParts.stream(rc));
        Object.assign(rc, ED.runParts.start(rc));
        Object.assign(rc, ED.runParts.stop(rc));
        // The code that has not moved into a part yet calls the parts' functions by these names.
        const { attach, closeStream, runData, setRun, setStopPending, stopRun, watchParked } = rc;

        function triggerLabel(r) {
            const n = ed.model.node(r.trigger_node);
            return n ? (n.label || n.type) : t('easydrag.ui.trigger_removed');
        }

        function runRow(r) {
            const row = '<button type="button" class="ed-run-row" data-ed-run="' + esc(r.id) + '"><span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span>' +
                '<span class="ed-run-main"><span>' + esc(ED.durable ? ED.durable.label(t, r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(triggerLabel(r)) + '</span>' +
                '<span class="ed-muted">' + esc(core.fmt.dateTime(r.started_at)) + (r.duration_ms ? ' · ' + esc(core.fmt.duration(r.duration_ms)) : '') + '</span></span>' +
                '<span class="ed-chip' + (r.mode === 'test' ? ' ed-chip--muted' : '') + '">' + (r.durable ? core.icon('repeat') + '<span class="ed-sr-only">' + esc(t('easydrag.ui.durable_title')) + ' · </span>' : '') + esc(r.mode === 'test' ? t('easydrag.ui.run_mode_test') : t('easydrag.ui.run_mode_live')) + '</span></button>';
            // A run that has not ended can be stopped from its row, also on a read-only desktop.
            if (!isActive(r.status)) return row;
            return '<div class="ed-run-item">' + row + '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-run-stop="' + esc(r.id) + '" aria-label="' +
                esc(t('easydrag.ui.run_stop_label', { time: core.fmt.dateTime(r.started_at, true) })) + '">' + core.icon('player-stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button></div>';
        }

        function matchingRuns() {
            return Array.from(rc.listedRuns.values()).filter(r => !rc.runsQuery || [r.id, r.status, core.tr(t, 'easydrag.ui.status_' + r.status, r.status), triggerLabel(r), core.fmt.dateTime(r.started_at), r.revision].join(' ').toLocaleLowerCase().includes(rc.runsQuery));
        }

        function drawRuns() {
            if (!rc.drawer) return;
            const list = rc.drawer.querySelector('.ed-runs-list');
            const rows = matchingRuns();
            list.innerHTML = rows.length ? rows.map(runRow).join('') : '<p class="ed-hint">' + esc(t('easydrag.ui.runs_empty')) + '</p>';
            new Set([...rc.stopping, ...rc.durableStopping]).forEach(id => setStopPending(id, true));
            const more = rc.drawer.querySelector('[data-ed-runs-more]');
            more.hidden = !rc.runsMore;
            more.disabled = rc.runsLoading;
            const compare = rc.drawer.querySelector('[data-ed-runs-compare]');
            if (compare) compare.disabled = !ED.compare || rows.filter(r => isFinal(r.status)).length < 2;
        }

        // loadRuns fills the drawer; only the answer to the latest request (filter) is shown. A
        // refresh keeps the rows until its answer. Focus in the list stays in the drawer: on the
        // same run's row while it is listed.
        async function loadRuns(refresh, append) {
            if (!rc.drawer) return;
            // Keep an inspected history prefix stable; explicit Refresh starts a new list.
            if (refresh && rc.runsOffset > 50) return;
            if (append && (rc.runsLoading || !rc.runsMore)) return;
            const mine = ++rc.runsSeq;
            const list = rc.drawer.querySelector('.ed-runs-list');
            const params = { limit: 50, offset: append ? rc.runsOffset : undefined };
            if (rc.drawerFilter === 'errors') params.status = 'error';
            if (rc.drawerFilter === 'tests') params.mode = 'test';
            if (rc.drawerFilter === 'live') params.mode = 'live';
            rc.runsLoading = true;
            rc.drawer.querySelector('[data-ed-runs-more]').disabled = true;
            rc.drawer.querySelector('[data-ed-runs-error]').hidden = true;
            if (!refresh && !append) list.innerHTML = '<p class="ed-hint">' + esc(t('easydrag.ui.loading')) + '</p>';
            try {
                const response = await ed.api.runs(ed.flow.id, params);
                const runs = response.runs || [];
                if (mine !== rc.runsSeq || !rc.drawer) return;
                if (!append) rc.listedRuns = new Map();
                runs.forEach(r => { if (!append || !rc.listedRuns.has(r.id)) rc.listedRuns.set(r.id, r); });
                rc.runsOffset = Number.isInteger(response.next_offset) ? response.next_offset : (params.offset || 0) + runs.length;
                rc.runsMore = typeof response.has_more === 'boolean' ? response.has_more : runs.length === 50;
            } catch (err) {
                if (mine !== rc.runsSeq || !rc.drawer) return;
                if (!append && !refresh) { rc.listedRuns = new Map(); rc.runsOffset = 0; rc.runsMore = false; }
                const error = rc.drawer.querySelector('[data-ed-runs-error]');
                error.textContent = core.errorText(t, err); error.hidden = false;
            }
            if (mine !== rc.runsSeq || !rc.drawer) return;
            rc.runsLoading = false;
            const active = document.activeElement;
            const inList = !!(active && list.contains(active));
            const focusRun = inList ? (active.dataset.edRunStop || active.dataset.edRun || '') : null;
            const onStop = inList && active.dataset.edRunStop !== undefined;
            drawRuns();
            if (focusRun === null) return;
            // The same control when it is still there (the Stop of a run that goes on), else the row.
            const stop = onStop && Array.from(list.querySelectorAll('[data-ed-run-stop]')).find(b => b.dataset.edRunStop === focusRun);
            const target = stop || Array.from(list.querySelectorAll('[data-ed-run]')).find(b => b.dataset.edRun === focusRun) || rc.drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]');
            if (target) target.focus();
        }

        // The drawer takes focus when it opens and gives it back to its opener when it closes.
        function toggleDrawer(force) {
            const open = force === undefined ? !rc.drawer : force;
            if (!open) {
                if (!rc.drawer) return;
                const hadFocus = rc.drawer.contains(document.activeElement);
                rc.drawer.remove();
                rc.drawer = null;
                ++rc.runsSeq; rc.runsLoading = false; rc.runsOffset = 0;
                ed.root.classList.remove('has-drawer');
                if (hadFocus && rc.drawerOpener && typeof rc.drawerOpener.focus === 'function') rc.drawerOpener.focus();
                rc.drawerOpener = null;
                return;
            }
            if (rc.drawer) return;
            rc.drawerOpener = document.activeElement;
            rc.drawer = core.el('<aside class="ed-drawer" aria-label="' + esc(t('easydrag.ui.runs_title')) + '"><header class="ed-drawer-head"><h3>' + esc(t('easydrag.ui.runs_title')) + '</h3>' +
                '<button type="button" class="ed-icon-btn" data-ed-drawer-close aria-label="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></header>' +
                '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.runs_filter_label')) + '">' + ['all', 'errors', 'tests', 'live'].map(f => '<button type="button" role="radio" aria-checked="' + (f === rc.drawerFilter) + '" data-ed-runs-filter="' + f + '">' + esc(t('easydrag.ui.runs_filter_' + f)) + '</button>').join('') + '</div>' +
                '<div class="ed-runs-tools"><div class="ed-search">' + core.icon('search') + '<input type="search" data-ed-runs-search aria-label="' + esc(t('easydrag.ui.runs_search')) + '" placeholder="' + esc(t('easydrag.ui.runs_search')) + '"></div><button type="button" class="ed-icon-btn" data-ed-runs-refresh title="' + esc(t('easydrag.ui.runs_refresh')) + '" aria-label="' + esc(t('easydrag.ui.runs_refresh')) + '">' + core.icon('refresh') + '</button></div>' +
                '<button type="button" class="ed-btn ed-runs-pager" data-ed-runs-compare disabled>' + esc(t('easydrag.ui.compare_runs')) + '</button><p class="ed-error" data-ed-runs-error role="alert" hidden></p><div class="ed-runs-list"></div><button type="button" class="ed-btn ed-runs-pager" data-ed-runs-more hidden>' + esc(t('easydrag.ui.runs_load_more')) + '</button></aside>');
            (ed.root.querySelector('.ed-body') || ed.root).appendChild(rc.drawer);
            ed.root.classList.add('has-drawer');
            rc.drawer.addEventListener('click', (event) => {
                if (event.target.closest('[data-ed-drawer-close]')) { toggleDrawer(false); return; }
                if (event.target.closest('[data-ed-runs-more]')) { loadRuns(false, true); return; }
                if (event.target.closest('[data-ed-runs-refresh]')) { loadRuns(false); return; }
                if (event.target.closest('[data-ed-runs-compare]')) { if (ED.compare) ED.compare.open(ed, matchingRuns()); return; }
                const f = event.target.closest('[data-ed-runs-filter]');
                if (f) {
                    rc.drawerFilter = f.dataset.edRunsFilter;
                    rc.drawer.querySelectorAll('[data-ed-runs-filter]').forEach(b => b.setAttribute('aria-checked', String(b === f)));
                    loadRuns();
                    return;
                }
                const stop = event.target.closest('[data-ed-run-stop]');
                if (stop) { stopRun(rc.listedRuns.get(stop.dataset.edRunStop)); return; }
                const row = event.target.closest('[data-ed-run]');
                if (row) openRunView(row.dataset.edRun);
            });
            rc.drawer.querySelector('[data-ed-runs-search]').value = rc.runsQuery;
            rc.drawer.querySelector('[data-ed-runs-search]').addEventListener('input', event => { rc.runsQuery = event.target.value.trim().toLocaleLowerCase(); drawRuns(); });
            loadRuns();
            const first = rc.drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]') || rc.drawer.querySelector('button');
            if (first) first.focus();
        }

        // Ignore duplicate opens and retain only the latest requested view.
        async function openRunView(runId, gateId) {
            if (rc.disposed || rc.viewing === runId) return;
            if (ed.runView && ed.runView.run.id !== runId && ED.approvals) ED.approvals.closeRunReview(ed);
            rc.viewing = runId;
            const mine = ++rc.viewSeq;
            try {
                const detail = await ed.api.run(runId, true);
                if (rc.disposed || mine !== rc.viewSeq) return;
                if (!detail.doc) {
                    ED.childHistory.missing(ed, detail);
                    return;
                }
                ed.enterRunView(detail);
                if (gateId && ED.approvals && ed.runView && ed.runView.run.id === runId)
                    ED.approvals.openRunReview(ed, gateId, { runId, flowId: ed.flow.id, revision: ed.runView.run.revision });
            } catch (err) {
                if (!rc.disposed && mine === rc.viewSeq) ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: core.errorText(t, err), type: 'error' });
            } finally {
                if (rc.viewing === runId) rc.viewing = null;
            }
        }

        function stepsFrom(steps, record, durableStatus, statusError) {
            const map = new Map();
            (steps || []).forEach(s => map.set(s.node_id, s));
            const observed = ED.durable ? ED.durable.status(record, durableStatus) : null;
            return { id: record.id, status: record.status, mode: record.mode, steps: map, changes: [], record, durableStatus: observed, durableKnown: !!observed, durableStatusError: statusError === 'storage_unavailable', durableUnknown: statusError === 'storage_unavailable', durableStopping: rc.durableStopping.has(record.id) };
        }

        // Keep an active run parked until clearRun restores its stream.
        function applyRunView(detail) {
            if (ed.run && !ed.run.view && !isFinal(ed.run.status) && !ed.run.stale) { rc.parked = { id: ed.run.id, mode: ed.run.mode }; watchParked(); }
            closeStream();
            const shown = stepsFrom(detail.steps, detail.run, detail.durable_status, detail.durable_status_error);
            shown.view = true;
            shown.finished = true;
            if (ed.runView) ed.runView.data = runData(detail.run, detail.steps || []);
            setRun(shown);
        }

        function clearRun() {
            closeStream();
            const back = rc.parked;
            rc.parked = null;
            clearTimeout(rc.parkTimer);
            if (back && !rc.disposed) { attach(back.id, { mode: back.mode }); return; }
            setRun(null);
        }

        // The functions that have not moved into a part yet are on rc too: the parts call them through rc.
        Object.assign(rc, {
            triggerLabel, runRow, matchingRuns, drawRuns, loadRuns, toggleDrawer, openRunView, stepsFrom, applyRunView, clearRun
        });

        bag.add(rc.closeStream);
        bag.add(ed.bus.on('refresh-durable-run', rc.refreshDurableShown));
        bag.add(() => rc.runReads.dispose());
        bag.add(() => clearTimeout(rc.parkTimer));
        bag.add(() => { rc.stopPolls.forEach(clearTimeout); rc.stopPolls.clear(); });
        bag.add(() => rc.runFinished.cancel());
        bag.add(() => rc.toggleDrawer(false));
        bag.add(() => { if (ED.compare) ED.compare.close(ed); });

        return {
            startTest: rc.startTest, attach: rc.attach, runLive: rc.runLive, cancel: rc.cancel, stopRun: rc.stopRun, refreshShown: rc.refreshShown, runFinished: rc.runFinished,
            loadLast: rc.loadLast, toggleDrawer: rc.toggleDrawer, openRunView: rc.openRunView, effects, stepsFrom: rc.stepsFrom, applyRunView: rc.applyRunView, clearRun: rc.clearRun,
            isRunning: rc.isRunning, hasParked: () => !!rc.parked,
            drawerOpen: () => !!rc.drawer,
            dispose() { rc.disposed = true; rc.parked = null; bag.dispose(); }
        };
    }

    ED.runs = { create, effects, unknownEffects, triggers, edgeStates, isFinal, isActive };
})();
