// EasyDrag runs: the ED.runs facade, the wire states of a run (edgeStates) and the run core.
// create() builds the core (shared run state, helpers, limits) and installs the four run parts
// on it: easydrag-run-stream.js (live run stream), easydrag-run-start.js (test dialog),
// easydrag-run-stop.js (run, cancel, stop) and easydrag-run-drawer.js (run drawer, run view).
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
        Object.assign(rc, ED.runParts.drawer(rc));

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
