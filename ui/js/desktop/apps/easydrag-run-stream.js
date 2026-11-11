// EasyDrag run stream: follows a live run (SSE with reconnect and backoff, durable reads), applies
// its events to the run state, announces failed steps and loads the stored result when it ends.
// One of the four run parts that create() in easydrag-runs.js installs on its run core; it loads
// before easydrag-runs.js.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // installStream installs the stream part on the run core rc and returns its functions, which
    // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
    // parts are called through rc when they run, after every part is installed.
    function installStream(rc) {
        const { ed, canvas, core, t, isFinal, edgeStates, RETRY_MS, RETRY_MAX_MS, MAX_FAILURES } = rc;
        const settleDurableStop = (...args) => rc.settleDurableStop(...args);
        const watchParked = (...args) => rc.watchParked(...args);
        const loadRuns = (...args) => rc.loadRuns(...args);

        const durable = run => !!(ED.durable && ED.durable.has(run));
        const runReads = ED.durable.observeRun({
            api: ed.api, root: ed.root, ctx: ed.ctx, current: () => ed.run, isFinal,
            onDetail(detail, current) {
                snapshot(detail, current, true);
                if (isFinal(current.status) && !current.view) finish(current.id, detail);
                else if (!current.view && !rc.source) connect(current.id);
            },
            onError: unknownDurable
        });
        const { read: readDurable, watch: watchDurable } = runReads;

        // noteStep records that a step of run changed (ed.run.changes): the canvas and the wires repaint
        // only those steps on the next run event (F6). A run object without a log is repainted whole.
        function noteStep(run, id) { if (run && Array.isArray(run.changes)) run.changes.push(id); }

        function setRun(next) {
            if (next !== ed.run) runReads.reset();
            ed.run = next;
            if (ed.run) ed.run.edges = edgeStates(ed);
            ed.bus.emit('run', ed.run);
            watchDurable();
        }

        function snapshot(detail, current, authoritative) {
            if (!detail || !detail.run || !current || detail.run.id !== current.id) return false;
            const wasDurable = durable(current);
            const keepFinal = isFinal(current.status) && !isFinal(detail.run.status);
            if (!keepFinal) { current.record = detail.run; current.status = detail.run.status; }
            if (ED.durable && (authoritative || Object.hasOwn(detail, 'durable_status'))) {
                current.durableStatus = ED.durable.status(current.record || detail.run, detail.durable_status);
                current.durableStatusError = detail.durable_status_error === 'storage_unavailable';
                if (current.durableStatus) current.durableKnown = true;
                else if (current.durableStatusError && wasDurable) current.durableKnown = true;
                else if (authoritative && !current.durableStatusError) current.durableKnown = false;
            }
            if (authoritative || Object.hasOwn(detail, 'durable_status')) { current.durableUnknown = !!current.durableStatusError; current.durableAccessLost = false; if (authoritative) current.stale = false; }
            if (Array.isArray(detail.steps) && !keepFinal) {
                const old = current.steps;
                if (authoritative || durable(current)) current.steps = new Map();
                detail.steps.forEach(step => { current.steps.set(step.node_id, step); noteStep(current, step.node_id); announceFailed(step.node_id, step); });
                old.forEach((_, id) => { if (!current.steps.has(id)) noteStep(current, id); });
            }
            if (ed.runView && ed.runView.run.id === current.id) ed.runView.run = current.record;
            setRun(current);
            settleDurableStop(detail);
            return true;
        }

        function unknownDurable(current, err) {
            current.durableUnknown = true;
            if ([401, 403].includes(Number(err && err.status))) {
                current.durableAccessLost = true; closeStream();
                current.steps.forEach((_, id) => noteStep(current, id)); current.steps.clear();
                current.record = { id: current.id, status: current.status, mode: current.mode, durable: !!current.record?.durable };
                current.durableStatus = null; ed.lastRunData = null;
                if (ed.runView) ed.runView.data = { runId: current.id, label: t('easydrag.ui.durable_unknown'), roots: {} };
            }
            setRun(current);
        }

        async function reconnect(runId) {
            if (rc.disposed || !streamed(runId)) return;
            if (durable(ed.run)) {
                const current = ed.run;
                try {
                    const detail = await readDurable(runId);
                    if (rc.disposed || ed.run !== current || !streamed(runId)) return;
                    snapshot(detail, current, true);
                    if (isFinal(current.status)) { finish(runId, detail); return; }
                } catch (err) {
                    if (rc.disposed || ed.run !== current || err.name === 'AbortError') return;
                    unknownDurable(current, err); if (!current.durableAccessLost) reconnectLater(runId); return;
                }
            }
            connect(runId);
        }

        async function refreshDurableShown() {
            const current = ed.run;
            if (!durable(current) || rc.disposed) return;
            try {
                const detail = await readDurable(current.id);
                if (rc.disposed || ed.run !== current) return;
                snapshot(detail, current, true);
                if (isFinal(current.status) && !current.view) finish(current.id, detail);
            } catch (err) {
                if (!rc.disposed && ed.run === current && err.name !== 'AbortError') unknownDurable(current, err);
            }
        }

        function rootsFrom(steps, run) {
            const roots = {};
            steps.forEach(step => { if (step.node_key && step.output) roots[step.node_key] = step.output; });
            if (run && run.trigger_data) roots.trigger = { data: run.trigger_data, type: run.trigger_type, node: run.trigger_node, fired_at: run.started_at };
            roots.run = { id: run ? run.id : '', started_at: run ? run.started_at : '', mode: run ? run.mode : 'test' };
            roots.flow = { id: ed.flow.id, name: ed.model.doc.name };
            return roots;
        }

        // runData is a run's data for the input tree: a label and the outputs by node key.
        function runData(run, steps) {
            const label = run.mode === 'test'
                ? t('easydrag.ui.input_last_test', { time: core.fmt.relative(run.started_at) })
                : t('easydrag.ui.input_last_live', { time: core.fmt.relative(run.started_at) });
            return { runId: run.id, label, roots: rootsFrom(steps, run) };
        }

        function rememberRun(run, steps) { ed.lastRunData = runData(run, steps); }

        function closeStream() {
            clearTimeout(rc.reconnectTimer);
            if (rc.source) { rc.source.close(); rc.source = null; }
        }

        function announceFailed(nodeId, step) {
            if (!step || step.status !== 'error' || rc.failedSpoken.has(nodeId)) return;
            rc.failedSpoken.add(nodeId);
            const node = ed.model.node(nodeId);
            const name = node ? (node.label || node.type) : step.node_key;
            if (name) canvas.announce(t('easydrag.ui.announce_step_failed', { name }));
        }

        // attach follows a live run. A run started while the run view shows a stored run (its
        // request answered after the view opened) must not paint over it: it is parked, and
        // clearRun() attaches it when the run view ends.
        function attach(runId, meta) {
            const mode = (meta && meta.mode) || 'test';
            if (ed.runView) { rc.parked = { id: runId, mode }; watchParked(); return; }
            closeStream();
            rc.lastSeq = 0;
            rc.retryDelay = RETRY_MS;
            rc.failures = 0;
            rc.parked = null;
            clearTimeout(rc.parkTimer);
            rc.failedSpoken = new Set();
            setRun({ id: runId, status: 'queued', mode, steps: new Map(), changes: [], record: null, error: '', draftRevision: meta && meta.draftRevision });
            connect(runId);
        }

        // streamed reports whether runId is the live run shown. A run view of the same run (the
        // live run parked) does not count: nothing may stream into the viewed run.
        function streamed(runId) { return !!(ed.run && ed.run.id === runId && !ed.run.view); }

        function connect(runId) {
            if (!streamed(runId) || rc.source) return;
            clearTimeout(rc.reconnectTimer);
            const es = new EventSource(ed.api.eventsUrl(runId, rc.lastSeq || undefined));
            rc.source = es;
            // Every message shows the stream works: the next failure waits RETRY_MS again.
            const alive = () => { rc.retryDelay = RETRY_MS; rc.failures = 0; };
            const failed = () => {
                if (rc.disposed || rc.source !== es) return;
                es.close();
                rc.source = null;
                if (!streamed(runId)) return;
                // The stream broke after run_finished but before "end": load the result now.
                if (isFinal(ed.run.status)) { finish(runId); return; }
                rc.failures++;
                if (rc.failures >= MAX_FAILURES) { rc.failures = 0; probe(runId); return; }
                reconnectLater(runId);
            };
            es.addEventListener('snapshot', (event) => {
                if (rc.disposed || rc.source !== es) return;
                try {
                    const detail = JSON.parse(event.data);
                    if (!streamed(runId)) return;
                    if (!snapshot(detail, ed.run, false)) throw new Error('invalid flow run snapshot');
                    alive();
                    if (durable(ed.run) && !isFinal(ed.run.status) && !Object.hasOwn(detail, 'durable_status')) refreshDurableShown();
                } catch (err) { failed(); }
            });
            es.addEventListener('event', (event) => {
                if (rc.disposed || rc.source !== es) return;
                try {
                    const ev = JSON.parse(event.data);
                    if (!streamed(runId)) return;
                    if (!ev || typeof ev !== 'object' || !Number.isSafeInteger(ev.seq) || ev.seq < 1 || typeof ev.type !== 'string') throw new Error('invalid flow run event');
                    applyEvent(ev);
                    rc.lastSeq = Math.max(rc.lastSeq, ev.seq);
                    alive();
                } catch (err) { failed(); }
            });
            // resync: the server dropped this stream (a slow client), but the run goes on. A new
            // stream continues at once after the last event; a finished run answers it with "end".
            es.addEventListener('resync', (event) => {
                if (rc.disposed || rc.source !== es) return;
                let payload;
                try {
                    payload = JSON.parse(event.data);
                    if (!payload || !Number.isSafeInteger(payload.after) || payload.after < 0) throw new Error('invalid flow resync event');
                } catch (err) { failed(); return; }
                alive();
                es.close();
                if (rc.source === es) rc.source = null;
                if (!streamed(runId)) return;
                rc.lastSeq = Math.max(rc.lastSeq, payload.after);
                reconnect(runId);
            });
            es.addEventListener('end', () => { if (rc.disposed || rc.source !== es) return; alive(); es.close(); rc.source = null; finish(runId); });
            es.onerror = failed;
        }

        function reconnectLater(runId) {
            clearTimeout(rc.reconnectTimer);
            rc.reconnectTimer = setTimeout(() => reconnect(runId), rc.retryDelay);
            rc.retryDelay = Math.min(rc.retryDelay * 2, RETRY_MAX_MS);
        }

        // probe asks the server about a run whose stream kept failing without a message. A
        // finished run shows its result; an unknown run (404) or disabled flows (503) stop the
        // stream and say so; anything else keeps reconnecting.
        async function probe(runId) {
            let detail = null;
            let failure = null;
            try { detail = await ed.api.run(runId, false); } catch (err) { failure = err; }
            if (rc.disposed || !streamed(runId)) return;
            if (detail && detail.run && isFinal(detail.run.status)) { finish(runId, detail); return; }
            if (detail && durable(ed.run)) snapshot(detail, ed.run, true);
            const status = Number(failure && failure.status) || 0;
            const code = core.errorCode(failure);
            if (failure && (status === 404 || status === 503 || code === 'FLOW_RUN_NOT_FOUND' || code === 'FLOWS_DISABLED')) {
                streamLost(failure);
                return;
            }
            reconnectLater(runId);
        }

        // streamLost gives up following the run: its stored state cannot be had. isRunning ends
        // (the run is stale) and the error is shown.
        function streamLost(failure) {
            closeStream();
            ed.run.stale = true;
            ed.run.error = core.errorText(t, failure);
            setRun(ed.run);
            ed.ctx.notify({ title: t('easydrag.ui.run_stream_lost'), message: ed.run.error, type: 'error' });
        }

        // isRunning: a live run is active (not a run view, not one whose stream was given up).
        function isRunning() { return !!(ed.run && !ed.run.view && (!ed.run.stale || durable(ed.run)) && !isFinal(ed.run.status)); }

        function applyEvent(ev) {
            if (ev.type === 'run_started') { ed.run.status = 'running'; if (durable(ed.run) && ed.run.record) ed.run.record.status = 'running'; }
            else if (ev.type === 'step_started') { ed.run.steps.set(ev.node_id, Object.assign({}, ed.run.steps.get(ev.node_id), { node_id: ev.node_id, status: 'running' })); noteStep(ed.run, ev.node_id); }
            else if (ev.type === 'step_finished' && ev.step) {
                const before = ed.run.steps.get(ev.node_id);
                ed.run.steps.set(ev.node_id, ev.step); noteStep(ed.run, ev.node_id); announceFailed(ev.node_id, ev.step);
                if (durable(ed.run) && (ev.step.status === 'waiting' || before?.status === 'waiting' && ev.step.status === 'success')) {
                    ed.run.status = ev.step.status === 'waiting' ? 'waiting' : 'running';
                    if (ed.run.record) ed.run.record.status = ed.run.status;
                    ed.run.durableStatus = null; refreshDurableShown();
                }
            }
            else if (ev.type === 'run_finished' && ev.run) {
                ed.run.status = ev.run.status;
                ed.run.error = ev.run.error_message || '';
                ed.run.errorCode = ev.run.error_code || '';
                ed.run.duration = ev.run.duration_ms;
                if (durable(ed.run)) { ed.run.record = ev.run; ed.run.durableStatus = null; }
            }
            setRun(ed.run);
        }

        // finish loads the stored result of a run (known: a detail fetched already) and announces
        // it, once per run: "end" and a cancel that found the run finished may both get here. A
        // run that another run (or a run view) replaced during the fetch is left alone. When the
        // fetch fails and the streamed state is not final, the run is not finished as far as the
        // editor knows: finished is reset so that a later finish or probe can complete it, and the
        // stream reconnects (backing off) to end again and ask again. After MAX_FAILURES failed
        // fetches the run counts as lost.
        async function finish(runId, known) {
            const current = ed.run;
            if (!current || current.id !== runId || current.finished) return;
            current.finished = true;
            let detail = known || null;
            let failure = null;
            if (!detail) {
                try { detail = await ed.api.run(runId, false); } catch (err) { detail = null; failure = err; /* keep the streamed state */ }
            }
            if (rc.disposed || ed.run !== current) return;
            if (failure && !isFinal(current.status)) {
                current.finished = false;
                current.finishFailures = (current.finishFailures || 0) + 1;
                if (current.finishFailures >= MAX_FAILURES) { streamLost(failure); return; }
                rc.retryDelay = Math.min(RETRY_MS * Math.pow(2, current.finishFailures - 1), RETRY_MAX_MS);
                reconnectLater(runId);
                return;
            }
            if (detail && detail.run) {
                if (durable(current) && !isFinal(detail.run.status)) {
                    current.finished = false;
                    snapshot(detail, current, true);
                    reconnectLater(runId);
                    return;
                }
                current.record = detail.run;
                current.status = detail.run.status;
                if (ED.durable) current.durableStatus = ED.durable.status(detail.run, detail.durable_status);
                (detail.steps || []).forEach(s => { current.steps.set(s.node_id, s); noteStep(current, s.node_id); });
                rememberRun(detail.run, detail.steps || []);
                // The finished run is the last run now: the footer keeps naming it once the run
                // state is cleared (leaving a run view), as after loadLast.
                ed.bus.emit('last-run', detail.run);
            }
            setRun(ed.run);
            if (current.mode === 'test' && current.draftRevision && isFinal(current.status)) ed.testEvidence = { revision: current.draftRevision, status: current.status, id: current.id };
            const failed = Array.from(ed.run.steps.values()).find(s => s.status === 'error');
            const node = failed && ed.model.node(failed.node_id);
            canvas.announce(ed.run.status === 'success' ? t('easydrag.ui.run_done') : node ? t('easydrag.ui.run_failed_at', { node: node.label || node.type }) : t('easydrag.ui.run_failed'));
            if (rc.drawer) loadRuns(true);
        }

        return {
            durable, runReads, readDurable, watchDurable, noteStep, setRun, snapshot, unknownDurable, reconnect, refreshDurableShown, rootsFrom, runData, rememberRun,
            closeStream, announceFailed, attach, streamed, connect, reconnectLater, probe, streamLost, isRunning, applyEvent, finish
        };
    }

    (ED.runParts = ED.runParts || {}).stream = installStream;
})();
