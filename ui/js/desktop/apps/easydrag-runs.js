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
        let source = null;
        let lastSeq = 0;
        let reconnectTimer = 0;
        let retryDelay = RETRY_MS;
        let failures = 0;
        // parked is a live run that a run view replaced; clearRun() attaches it again.
        let parked = null;
        let parkTimer = 0;
        let drawer = null;
        let drawerOpener = null;
        let drawerFilter = 'all';
        let runsSeq = 0;
        let viewSeq = 0;
        let viewing = null;
        let disposed = false;
        const durable = run => !!(ED.durable && ED.durable.has(run));
        const runReads = ED.durable.observeRun({
            api: ed.api, root: ed.root, ctx: ed.ctx, current: () => ed.run, isFinal,
            onDetail(detail, current) {
                snapshot(detail, current, true);
                if (isFinal(current.status) && !current.view) finish(current.id, detail);
                else if (!current.view && !source) connect(current.id);
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
            if (disposed || !streamed(runId)) return;
            if (durable(ed.run)) {
                const current = ed.run;
                try {
                    const detail = await readDurable(runId);
                    if (disposed || ed.run !== current || !streamed(runId)) return;
                    snapshot(detail, current, true);
                    if (isFinal(current.status)) { finish(runId, detail); return; }
                } catch (err) {
                    if (disposed || ed.run !== current || err.name === 'AbortError') return;
                    unknownDurable(current, err); if (!current.durableAccessLost) reconnectLater(runId); return;
                }
            }
            connect(runId);
        }

        async function refreshDurableShown() {
            const current = ed.run;
            if (!durable(current) || disposed) return;
            try {
                const detail = await readDurable(current.id);
                if (disposed || ed.run !== current) return;
                snapshot(detail, current, true);
                if (isFinal(current.status) && !current.view) finish(current.id, detail);
            } catch (err) {
                if (!disposed && ed.run === current && err.name !== 'AbortError') unknownDurable(current, err);
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
            clearTimeout(reconnectTimer);
            if (source) { source.close(); source = null; }
        }

        // failedSpoken holds the steps of the live run whose failure was announced: a step that
        // reports again (the snapshot of a reconnect) is not announced twice. Steps that went well
        // are not announced at all; the end of the run is (finish).
        let failedSpoken = new Set();

        function announceFailed(nodeId, step) {
            if (!step || step.status !== 'error' || failedSpoken.has(nodeId)) return;
            failedSpoken.add(nodeId);
            const node = ed.model.node(nodeId);
            const name = node ? (node.label || node.type) : step.node_key;
            if (name) canvas.announce(t('easydrag.ui.announce_step_failed', { name }));
        }

        // attach follows a live run. A run started while the run view shows a stored run (its
        // request answered after the view opened) must not paint over it: it is parked, and
        // clearRun() attaches it when the run view ends.
        function attach(runId, meta) {
            const mode = (meta && meta.mode) || 'test';
            if (ed.runView) { parked = { id: runId, mode }; watchParked(); return; }
            closeStream();
            lastSeq = 0;
            retryDelay = RETRY_MS;
            failures = 0;
            parked = null;
            clearTimeout(parkTimer);
            failedSpoken = new Set();
            setRun({ id: runId, status: 'queued', mode, steps: new Map(), changes: [], record: null, error: '', draftRevision: meta && meta.draftRevision });
            connect(runId);
        }

        // streamed reports whether runId is the live run shown. A run view of the same run (the
        // live run parked) does not count: nothing may stream into the viewed run.
        function streamed(runId) { return !!(ed.run && ed.run.id === runId && !ed.run.view); }

        function connect(runId) {
            if (!streamed(runId) || source) return;
            clearTimeout(reconnectTimer);
            const es = new EventSource(ed.api.eventsUrl(runId, lastSeq || undefined));
            source = es;
            // Every message shows the stream works: the next failure waits RETRY_MS again.
            const alive = () => { retryDelay = RETRY_MS; failures = 0; };
            const failed = () => {
                if (disposed || source !== es) return;
                es.close();
                source = null;
                if (!streamed(runId)) return;
                // The stream broke after run_finished but before "end": load the result now.
                if (isFinal(ed.run.status)) { finish(runId); return; }
                failures++;
                if (failures >= MAX_FAILURES) { failures = 0; probe(runId); return; }
                reconnectLater(runId);
            };
            es.addEventListener('snapshot', (event) => {
                if (disposed || source !== es) return;
                try {
                    const detail = JSON.parse(event.data);
                    if (!streamed(runId)) return;
                    if (!snapshot(detail, ed.run, false)) throw new Error('invalid flow run snapshot');
                    alive();
                    if (durable(ed.run) && !isFinal(ed.run.status) && !Object.hasOwn(detail, 'durable_status')) refreshDurableShown();
                } catch (err) { failed(); }
            });
            es.addEventListener('event', (event) => {
                if (disposed || source !== es) return;
                try {
                    const ev = JSON.parse(event.data);
                    if (!streamed(runId)) return;
                    if (!ev || typeof ev !== 'object' || !Number.isSafeInteger(ev.seq) || ev.seq < 1 || typeof ev.type !== 'string') throw new Error('invalid flow run event');
                    applyEvent(ev);
                    lastSeq = Math.max(lastSeq, ev.seq);
                    alive();
                } catch (err) { failed(); }
            });
            // resync: the server dropped this stream (a slow client), but the run goes on. A new
            // stream continues at once after the last event; a finished run answers it with "end".
            es.addEventListener('resync', (event) => {
                if (disposed || source !== es) return;
                let payload;
                try {
                    payload = JSON.parse(event.data);
                    if (!payload || !Number.isSafeInteger(payload.after) || payload.after < 0) throw new Error('invalid flow resync event');
                } catch (err) { failed(); return; }
                alive();
                es.close();
                if (source === es) source = null;
                if (!streamed(runId)) return;
                lastSeq = Math.max(lastSeq, payload.after);
                reconnect(runId);
            });
            es.addEventListener('end', () => { if (disposed || source !== es) return; alive(); es.close(); source = null; finish(runId); });
            es.onerror = failed;
        }

        function reconnectLater(runId) {
            clearTimeout(reconnectTimer);
            reconnectTimer = setTimeout(() => reconnect(runId), retryDelay);
            retryDelay = Math.min(retryDelay * 2, RETRY_MAX_MS);
        }

        // probe asks the server about a run whose stream kept failing without a message. A
        // finished run shows its result; an unknown run (404) or disabled flows (503) stop the
        // stream and say so; anything else keeps reconnecting.
        async function probe(runId) {
            let detail = null;
            let failure = null;
            try { detail = await ed.api.run(runId, false); } catch (err) { failure = err; }
            if (disposed || !streamed(runId)) return;
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
            if (disposed || ed.run !== current) return;
            if (failure && !isFinal(current.status)) {
                current.finished = false;
                current.finishFailures = (current.finishFailures || 0) + 1;
                if (current.finishFailures >= MAX_FAILURES) { streamLost(failure); return; }
                retryDelay = Math.min(RETRY_MS * Math.pow(2, current.finishFailures - 1), RETRY_MAX_MS);
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
            if (drawer) loadRuns(true);
        }

        // ── starting runs ───────────────────────────────────────────────────────

        // Real effects are confirmed one by one (sends_message, deletes, …): confirming the effects
        // of one step does not cover other effects of the flow, and only a test that started
        // confirms them. ed.effectsConfirmed holds this session's set; "remember" stores the list
        // under EFFECTS_KEY. An older stored `true` (one switch for the whole flow) counts as
        // nothing confirmed, so the warning comes once more.
        function storedEffects() {
            const stored = core.storage.get(EFFECTS_KEY + ed.flow.id, []);
            return Array.isArray(stored) ? stored.filter(x => typeof x === 'string') : [];
        }

        function confirmedEffects() {
            const set = new Set(storedEffects());
            if (ed.effectsConfirmed instanceof Set) ed.effectsConfirmed.forEach(x => set.add(x));
            return set;
        }

        function confirmEffects(list, remember) {
            if (!(ed.effectsConfirmed instanceof Set)) ed.effectsConfirmed = new Set();
            list.forEach(x => ed.effectsConfirmed.add(x));
            if (remember) core.storage.set(EFFECTS_KEY + ed.flow.id, Array.from(new Set(storedEffects().concat(list))));
        }

        function uncheckedMarkup() {
            return '<p class="ed-callout ed-callout--warn" data-ed-effects-unchecked>' + core.icon('alert') + '<span>' + esc(t('easydrag.ui.effects_unchecked')) + '</span></p>';
        }

        function effectsMarkup(list) {
            if (!list.size) return '';
            return '<div class="ed-callout ed-callout--warn" data-ed-test-effects>' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.effects_title')) + '</strong><p data-ed-effects-real>' + esc(t('easydrag.ui.effects_real_stop')) + '</p><ul class="ed-effects">' +
                Array.from(list.entries()).map(([effect, nodes]) => '<li data-ed-effect="' + esc(effect) + '">' + core.icon(EFFECT_ICONS[effect] || 'bolt') + '<span>' + esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect)) + '</span><span class="ed-muted">' + esc(nodes.join(', ')) + '</span></li>').join('') +
                '</ul><label class="ed-check"><input type="checkbox" data-ed-effects-skip> ' + esc(t('easydrag.ui.effects_remember')) + '</label></div></div>';
        }

        // starting is true from a startTest call until its run was posted or its dialog closed:
        // one test at a time, and none while a run is active (it would orphan that run's stream).
        let starting = false;
        // startPost is the test or live POST in flight. A second call waits on the same
        // promise and does not send again. An uncertain result is not sent again.
        let startPost = null;

        function trackStart(work) {
            if (startPost) return startPost;
            const pending = work().finally(() => { if (startPost === pending) startPost = null; });
            startPost = pending;
            return pending;
        }

        function editsReady() {
            return !(ed.flushPendingEdits && ed.flushPendingEdits() === false);
        }

        async function startTest(opts) {
            if (disposed || ed.readonly || ed.runView || starting || startPost || isRunning()) return undefined;
            starting = true;
            let dialog;
            try {
                dialog = await openTest(opts || {});
            } finally {
                if (!dialog) starting = false;
            }
            if (dialog) dialog.done.then(() => { starting = false; });
            return dialog;
        }

        async function openTest(o) {
            // Field and note edits land before the draft is saved for the test.
            if (!editsReady()) return undefined;
            // A test runs the saved draft: an invalid, offline or conflicting draft is not tested
            // silently in its last saved version.
            if (ed.saver && !(await ed.saver.flush())) {
                if (!disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                return undefined;
            }
            if (disposed) return undefined;
            // version is the draft the effects were checked for (the one just saved).
            let version = ed.model.version;
            const list = triggers(ed);
            if (!list.length) { ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_no_trigger'), type: 'error' }); return undefined; }
            const remembered = core.storage.get('aurago.easydrag.test-trigger.' + ed.flow.id, '');
            let trigger = list.find(n => n.id === (o.triggerNode || remembered)) || list.find(n => n.type === 'trigger.manual') || list[0];
            // The catalog describes each step with its default settings; the publish preview (a
            // read-only check of the saved draft) names the effects of the real ones. Without it
            // the dialog says that the effects could not be fully checked, and a test never starts
            // without the dialog.
            let real = null;
            const loadReal = async () => {
                try { real = realEffects(await ed.api.preview(ed.flow.id)); } catch (err) { real = null; }
            };
            await loadReal();
            if (disposed) return undefined;
            const confirmed = confirmedEffects();
            // A step test runs what its trigger reaches: the effects follow the chosen trigger.
            const pending = id => new Map(Array.from(effects(ed, o.onlyNode, id, real)).filter(([effect]) => !confirmed.has(effect)));
            // unchecked: the effects could not be named in full (no preview, or a step whose effects
            // hook failed); the dialog then says so, and a test never starts without it.
            const unchecked = id => !real || unknownEffects(ed, o.onlyNode, id);
            let fx = pending(trigger.id);
            // recheck follows an edit that landed after the flush: it saves again and asks the
            // preview again. False when the draft cannot be saved.
            const recheck = async () => {
                if (!editsReady()) return false;
                if (ed.saver && !(await ed.saver.flush())) return false;
                version = ed.model.version;
                await loadReal();
                return true;
            };
            if (o.quick && ed.model.version !== version) {
                if (!(await recheck())) {
                    if (!disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                    return undefined;
                }
                if (disposed) return undefined;
                fx = pending(trigger.id);
            }
            if (o.quick && !fx.size && !unchecked(trigger.id) && ed.model.version === version) { await run(trigger.id, null, o.onlyNode, false); return undefined; }
            // A rerun from the run view (o.data) starts from that run's trigger data instead of the
            // stored sample. It runs as given even unchanged, and is not remembered unless ticked.
            let prefilled = !!(o.data && typeof o.data === 'object' && !Array.isArray(o.data));
            let sample = {};
            if (prefilled) sample = o.data;
            else {
                try { sample = (await ed.api.testData(ed.flow.id, trigger.id)).data || {}; } catch (err) { sample = {}; }
            }
            if (disposed) return undefined;
            // The sample comes scrubbed (secret values read "[redacted]") and must never be saved
            // back: unchanged text runs without trigger_data, so the server uses the stored sample.
            let shown = JSON.stringify(sample, null, 2);
            let simulation = null;
            const dialog = core.modal(ed.root, {
                title: o.onlyNode ? t('easydrag.ui.test_node_title') : t('easydrag.ui.test_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--test',
                body: (list.length > 1 ? '<label class="ed-label">' + esc(t('easydrag.ui.test_trigger')) + '<select class="ed-input" data-ed-test-trigger>' +
                    list.map(n => '<option value="' + esc(n.id) + '"' + (n.id === trigger.id ? ' selected' : '') + '>' + esc(n.label || n.type) + '</option>').join('') + '</select></label>' : '') +
                    '<label class="ed-label">' + esc(t('easydrag.ui.test_data')) + '<textarea class="ed-input ed-code" rows="9" spellcheck="false" data-ed-test-data>' + esc(shown) + '</textarea></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.test_data_hint')) + '</p><p class="ed-error" role="alert" hidden></p>' +
                    '<label class="ed-check"><input type="checkbox" data-ed-test-remember' + (prefilled ? '' : ' checked') + '> ' + esc(t('easydrag.ui.test_remember')) + '</label>' +
                    (prefilled && shown.includes(REDACTED) ? '<p class="ed-callout ed-callout--warn" data-ed-rerun-redacted>' + core.icon('alert') + '<span>' + esc(t('easydrag.ui.rerun_redacted')) + '</span></p>' : '') +
                    (unchecked(trigger.id) ? uncheckedMarkup() : '') + effectsMarkup(fx),
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'run', label: t('easydrag.ui.test_run'), primary: true, icon: 'play' }],
                onAction: async (action, d) => {
                    if (action !== 'run') return true;
                    const err = d.body.querySelector('.ed-error');
                    // An edit landed after the effects were checked: check them again and let the
                    // user confirm the new list.
                    if (ed.model.version !== version) {
                        const saved = await recheck();
                        if (disposed) return false;
                        const pick = d.body.querySelector('[data-ed-test-trigger]');
                        showChecks(d.body, pick ? pick.value : trigger.id);
                        if (!saved) { err.hidden = false; err.textContent = t('easydrag.ui.test_unsaved'); }
                        return false;
                    }
                    if (simulation && !simulation.flush()) return false;
                    if (simulation && simulation.isSelected()) return trackStart(() => simulation.run());
                    const text = d.body.querySelector('[data-ed-test-data]').value;
                    const edited = text !== shown || prefilled;
                    let data = null;
                    if (edited) {
                        try { data = JSON.parse(text || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.json_invalid'); return false; }
                        if (!data || typeof data !== 'object' || Array.isArray(data)) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_object'); return false; }
                        if (ED.testing) {
                            try { ED.testing.parseSample(text || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_incomplete'); return false; }
                        }
                    }
                    const sel = d.body.querySelector('[data-ed-test-trigger]');
                    if (sel) trigger = ed.model.node(sel.value) || trigger;
                    // The effects shown count as confirmed once the test started: a refused or failed
                    // POST keeps the dialog open, and the next attempt asks again.
                    const skip = d.body.querySelector('[data-ed-effects-skip]');
                    const shownEffects = Array.from(fx.keys());
                    const rememberEffects = !!(skip && skip.checked);
                    core.storage.set('aurago.easydrag.test-trigger.' + ed.flow.id, trigger.id);
                    // Edited data that still holds a placeholder runs, but is not remembered: it
                    // would replace the stored secret values with "[redacted]".
                    const remember = edited && d.body.querySelector('[data-ed-test-remember]').checked;
                    const redacted = remember && text.includes(REDACTED);
                    return run(trigger.id, data, o.onlyNode, remember && !redacted).then(ok => {
                        if (ok && shownEffects.length) confirmEffects(shownEffects, rememberEffects);
                        if (ok && redacted) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_data_redacted') });
                        return ok ? true : false;
                    });
                }
            });
            dialog.body.querySelector('[data-ed-test-data]').value = shown;
            if (ED.testing && ed.api.simulate) simulation = ED.testing.decorate(ed, dialog, {
                trigger: () => { const select = dialog.body.querySelector('[data-ed-test-trigger]'); return select ? select.value : trigger.id; },
                onlyNode: o.onlyNode, revision: baseRevision,
                isCurrent: () => !disposed && !ed.runView && ed.model.version === version,
                apply: (result, data) => {
                    const record = { id: 'simulation', mode: 'simulation', revision: result.revision, status: result.status, error_code: result.error_code, error_message: result.error_message, assertions: result.assertions, started_at: result.started_at || new Date().toISOString(), trigger_node: dialog.body.querySelector('[data-ed-test-trigger]') ? dialog.body.querySelector('[data-ed-test-trigger]').value : trigger.id, trigger_data: data };
                    const run = stepsFrom(result.steps || [], record);
                    if (ED.childHistory) ED.childHistory.rememberSimulation(run, result);
                    ed.lastRunData = { label: t('easydrag.ui.simulation_result'), roots: rootsFrom(result.steps || [], record) };
                    ed.testEvidence = { revision: result.revision, status: (result.assertions || []).some(assertion => !assertion.passed) ? 'error' : result.status, mode: 'simulation' };
                    setRun(run);
                    ed.bus.emit('last-run', record);
                    ed.bus.emit('last-run-data', ed.lastRunData);
                    canvas.announce(t('easydrag.ui.simulation_result'));
                }
            });
            // showChecks shows the hint and the effects for a trigger again (another trigger, a newer
            // check); the "don't ask again" tick stays as it was.
            function showChecks(body, id) {
                const skip = body.querySelector('[data-ed-effects-skip]');
                const remember = !!(skip && skip.checked);
                body.querySelectorAll('[data-ed-effects-unchecked], [data-ed-test-effects]').forEach(n => n.remove());
                fx = pending(id);
                if (unchecked(id)) body.appendChild(core.el(uncheckedMarkup()));
                if (fx.size) body.appendChild(core.el(effectsMarkup(fx))).querySelector('[data-ed-effects-skip]').checked = remember;
                if (simulation) simulation.syncMode();
            }
            // Another trigger shows its own sample, unless the text was edited. For a step test it
            // also shows the effects of what that trigger reaches.
            const select = dialog.body.querySelector('[data-ed-test-trigger]');
            if (select) {
                select.addEventListener('change', async () => {
                    if (o.onlyNode) showChecks(dialog.body, select.value);
                    const area = dialog.body.querySelector('[data-ed-test-data]');
                    const id = select.value;
                    if (area.value !== shown) return;
                    let next = {};
                    try { next = (await ed.api.testData(ed.flow.id, id)).data || {}; } catch (err) { next = {}; }
                    if (select.value !== id || area.value !== shown) return;
                    shown = JSON.stringify(next, null, 2);
                    area.value = shown;
                    if (simulation) simulation.refreshData();
                    prefilled = false;
                });
            }
            return dialog;
        }

        // baseRevision is the draft revision a test runs: the saver's (it saved just before) or the
        // flow's. The server refuses the test with 409 when the draft moved on in another window.
        function baseRevision() {
            const rev = ed.saver && Number.isInteger(ed.saver.revision) ? ed.saver.revision : ed.flow && ed.flow.draft_revision;
            return Number.isInteger(rev) && rev > 0 ? rev : 0;
        }

        function run(triggerNode, data, onlyNode, remember) {
            return trackStart(async () => {
                try {
                    const body = { trigger_node: triggerNode, only_node: onlyNode || undefined, remember_data: !!remember };
                    const rev = baseRevision();
                    if (rev) body.base_revision = rev;
                    if (data) body.trigger_data = data;
                    const res = await ed.api.test(ed.flow.id, body);
                    if (res && res.run_id && !disposed) { attach(res.run_id, { mode: 'test', draftRevision: rev }); canvas.announce(t('easydrag.ui.announce_test_started')); }
                    return true;
                } catch (err) {
                    ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: core.errorText(t, err), type: 'error' });
                    return false;
                }
            });
        }

        function runLive() {
            if (disposed || ed.runView || starting || startPost || isRunning()) return undefined;
            return trackStart(async () => {
                try {
                    if (ED.durable && ED.durable.requires(ed.flow.live)) {
                        const capability = ed.runtime && await ed.runtime.refresh(true);
                        if (disposed || ed.readonly || ed.runView) return;
                        if (!capability || !capability.effective || !capability.recovery_available) {
                            ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: capability ? ED.durable.reason(t, capability.reason) : t('easydrag.ui.durable_unknown'), type: 'error' });
                            return;
                        }
                    }
                    const res = await ed.api.runNow(ed.flow.id);
                    if (disposed) return;
                    if (res && res.run_id) { attach(res.run_id, { mode: 'live' }); canvas.announce(t('easydrag.ui.announce_run_started')); return; }
                    // Concurrency "skip" with a run still going: 202 {status: "skipped"} and no run to follow (runner.go Start).
                    if (res && res.status === 'skipped') {
                        ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: t('easydrag.ui.run_now_skipped') });
                        canvas.announce(t('easydrag.ui.run_now_skipped'));
                    }
                } catch (err) {
                    ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: core.errorText(t, err), type: 'error' });
                }
            });
        }

        async function cancel() {
            if (!ed.run || isFinal(ed.run.status) && !ED.durable?.info(ed.run)?.cancel_available) return;
            const record = Object.assign({ id: ed.run.id, status: ed.run.status, mode: ed.run.mode || 'test' }, ed.run.record || {});
            return stopRun(record);
        }

        // settleFinished shows the stored result of a run that ended before its stream said so:
        // the stream closes and finish() applies the final state. While the stored run is not
        // final yet (or cannot be read), the stream stays open and brings the end itself.
        async function settleFinished(runId) {
            let detail = null;
            try { detail = await ed.api.run(runId, false); } catch (err) { return; }
            if (disposed || !detail || !detail.run || !isFinal(detail.run.status)) return;
            if (!ed.run || ed.run.id !== runId || isFinal(ed.run.status)) return;
            closeStream();
            await finish(runId, detail);
        }

        // stopping holds the runs whose stop is being confirmed or sent: one stop per run at a time.
        const stopping = new Set();
        const durableStopping = new Set();
        // stopPolls holds the timers of followStop.
        const stopPolls = new Set();

        // stopButtons are every control for this still-active run: drawer, header and run view.
        function stopButtons(id) {
            if (!ed.root || !ed.root.querySelectorAll) return [];
            return Array.from(ed.root.querySelectorAll('[data-ed-run-stop], [data-ed-cmd="cancel"], [data-ed-cmd="stop-viewed"]')).filter(b => {
                if (b.dataset.edRunStop) return b.dataset.edRunStop === String(id);
                if (b.dataset.edCmd === 'cancel') return !!(ed.run && ed.run.id === id && !ed.run.view);
                return !!(ed.runView && ed.runView.run && ed.runView.run.id === id);
            });
        }

        // setStopPending shows the stop request on those buttons until it settles.
        function setStopPending(id, on) {
            stopButtons(id).forEach(b => {
                b.disabled = !!on;
                if (on) b.setAttribute('aria-busy', 'true');
                else b.removeAttribute('aria-busy');
                b.classList.toggle('is-busy', !!on);
                // The header's Stop is named after its text (its text is hidden on narrow headers):
                // the name says "Stopping…" as long as the text does.
                if (b.dataset.edCmd === 'cancel') {
                    const name = on ? t('easydrag.ui.run_stopping') : t('easydrag.ui.run_cancel');
                    b.setAttribute('aria-label', name);
                    b.setAttribute('title', name);
                }
                const span = b.querySelector('span');
                if (!span) return;
                if (on) {
                    if (!b.dataset.edStopLabel) b.dataset.edStopLabel = span.textContent;
                    span.textContent = t('easydrag.ui.run_stopping');
                } else if (b.dataset.edStopLabel) {
                    span.textContent = b.dataset.edStopLabel;
                    delete b.dataset.edStopLabel;
                }
            });
        }

        // stopRun stops a queued, waiting or running run of the drawer, the header or the run
        // view, also one this window did not start. A test stops at once. Any other mode asks
        // first. While the cancel is in flight the button stays disabled. 202, and 409
        // FLOW_RUN_FINISHED (it ended meanwhile), refresh quietly; other errors are shown.
        // Stop does not undo a step that already ran. A read-only desktop stops runs too.
        async function stopRun(record) {
            const shown = ed.run && ed.run.id === record?.id ? ed.run : record;
            const checkpoint = ED.durable && ED.durable.info(shown);
            const durableRun = !!(record?.durable || checkpoint || shown?.durableKnown || shown?.durableStatusError);
            if (disposed || !record || !isActive(record.status) && !checkpoint?.cancel_available || stopping.has(record.id) || durableStopping.has(record.id)) return false;
            stopping.add(record.id);
            let pending = false;
            try {
                let generation;
                if (durableRun) {
                    // Refresh before confirming; never manufacture generation zero or infer
                    // cancellation permission from a display phase or the persisted marker.
                    const detail = await readDurable(record.id);
                    if (disposed || !detail?.run || detail.run.id !== record.id) return false;
                    const state = ED.durable.status(detail.run, detail.durable_status);
                    if (ed.run?.id === record.id) snapshot(detail, ed.run, true);
                    if (!state?.cancel_available) return false;
                    generation = state.generation;
                    record = detail.run;
                }
                if (record.mode !== 'test' && !(await confirmStop(record))) return false;
                if (disposed) return false;
                pending = true;
                setStopPending(record.id, true);
                let accepted = true;
                try {
                    if (durableRun) {
                        const answer = await ed.api.cancelDurable(record.id, generation);
                        if (!answer || answer.accepted !== true) throw new Error(t('easydrag.ui.durable_unknown'));
                        durableStopping.add(record.id);
                        if (ed.run?.id === record.id) { ed.run.durableStopping = true; setRun(ed.run); }
                        const detail = await readDurable(record.id);
                        if (!disposed && ed.run?.id === record.id) snapshot(detail, ed.run, true);
                        settleDurableStop(detail);
                    } else await ed.api.cancel(record.id);
                } catch (err) {
                    if (durableRun && core.errorCode(err) === 'FLOW_DURABLE_GENERATION_CONFLICT') {
                        try { const detail = await readDurable(record.id); if (!disposed && ed.run?.id === record.id) snapshot(detail, ed.run, true); } catch (_) { /* Preserve unknown state. */ }
                    }
                    if (core.errorCode(err) !== 'FLOW_RUN_FINISHED') {
                        if (durableRun && ed.run?.id === record.id) { ed.run.durableUnknown = true; setRun(ed.run); }
                        if (!disposed) ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' });
                        return false;
                    }
                    accepted = false;
                    if (streamed(record.id)) await settleFinished(record.id);
                }
                refreshShown();
                // A running run ends a moment after the cancel; only this window's own stream shows it.
                if (accepted && !streamed(record.id)) followStop(record.id, 0);
                return true;
            } catch (err) {
                if (durableRun && !disposed && ed.run?.id === record.id) unknownDurable(ed.run, err);
                if (!disposed) ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' });
                return false;
            } finally {
                if (pending && !durableStopping.has(record.id)) setStopPending(record.id, false);
                stopping.delete(record.id);
            }
        }

        function settleDurableStop(detail) {
            if (!detail?.run || !durableStopping.has(detail.run.id)) return;
            const state = ED.durable.status(detail.run, detail.durable_status);
            if (isFinal(detail.run.status) && !state?.cancel_available) {
                durableStopping.delete(detail.run.id); setStopPending(detail.run.id, false);
                if (ed.run?.id === detail.run.id && ed.run.durableStopping) { ed.run.durableStopping = false; setRun(ed.run); }
            }
        }

        // watchParked asks for the stored state of the parked live run every PARK_POLL_MS. Nothing else learns that
        // it ended (its stream is closed). Once final it is released (hasParked drives the run view's Test again and
        // the run-going note), becomes the last run, and the header repaints through the run event.
        function watchParked() {
            clearTimeout(parkTimer);
            if (!parked || disposed) return;
            const id = parked.id;
            parkTimer = setTimeout(async () => {
                let detail = null;
                let gone = false;
                try { detail = await ed.api.run(id, false); } catch (err) { gone = Number(err && err.status) === 404; }
                if (disposed || !parked || parked.id !== id) return;
                if (detail && detail.run && isFinal(detail.run.status)) {
                    parked = null;
                    rememberRun(detail.run, detail.steps || []);
                    ed.bus.emit('last-run', detail.run);
                    ed.bus.emit('run', ed.run);
                    if (drawer) loadRuns(true);
                    return;
                }
                if (gone) { parked = null; ed.bus.emit('run', ed.run); return; }
                watchParked();
            }, PARK_POLL_MS);
        }

        // followStop asks for a stopped run's stored state after STOP_POLL_MS[attempt] and refreshes
        // the drawer and the run view once it ended, while they still show it as not ended.
        function followStop(runId, attempt) {
            const timer = setTimeout(async () => {
                stopPolls.delete(timer);
                let detail = null;
                try { detail = await ed.api.run(runId, false); } catch (err) { detail = null; }
                if (disposed) return;
                settleDurableStop(detail);
                if (detail && detail.run && isFinal(detail.run.status)) {
                    if (showsActive(runId)) refreshShown();
                    return;
                }
                if (attempt + 1 < STOP_POLL_MS.length || durableStopping.has(runId)) followStop(runId, Math.min(attempt + 1, STOP_POLL_MS.length - 1));
            }, STOP_POLL_MS[attempt]);
            stopPolls.add(timer);
        }

        // showsActive: the drawer or the run view shows runId as not ended.
        function showsActive(runId) {
            const listed = drawer && listedRuns.get(runId);
            const viewed = ed.runView && ed.runView.run;
            return !!((listed && isActive(listed.status)) || (viewed && viewed.id === runId && isActive(viewed.status)));
        }

        async function confirmStop(record) {
            const answer = await core.modal(ed.root, {
                title: t('easydrag.ui.run_stop_title'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.run_stop_text', { time: core.fmt.dateTime(record.started_at) })) + '</p>',
                actions: [{ id: 'keep', label: t('easydrag.ui.cancel') }, { id: 'stop', label: t('easydrag.ui.run_cancel'), danger: true, icon: 'player-stop' }]
            }).done;
            return answer === 'stop';
        }

        // runFinished answers flows_changed "run_finished" (a live run ended, also one stopped here):
        // the last run, the drawer and a run view follow. A burst of ends refreshes once, 250 ms
        // after the last one, and a steady stream of ends still refreshes once a second.
        const runFinished = core.debounce(() => {
            if (disposed) return;
            if (!isRunning()) loadLast();
            refreshShown();
        }, 250, 1000);

        // refreshShown brings the drawer's list, and the run view of a run that had not ended, up
        // to date: after a stop (at once), and when a live run ended (runFinished).
        function refreshShown() {
            if (disposed) return;
            if (drawer) loadRuns(true);
            const shown = ed.runView && ed.runView.run;
            if (shown && isActive(shown.status)) refreshView(shown.id);
        }

        // refreshView shows the stored state of the viewed run again (status, steps, banner), unless
        // another run view replaced it or a newer refresh was asked meanwhile. An answer never takes
        // back the end of a run the view shows already.
        let viewRefreshSeq = 0;
        async function refreshView(runId) {
            const mine = viewSeq;
            const ask = ++viewRefreshSeq;
            let detail = null;
            try { detail = await ed.api.run(runId, false); } catch (err) { return; }
            const shown = ed.runView && ed.runView.run;
            if (disposed || mine !== viewSeq || ask !== viewRefreshSeq || !shown || shown.id !== runId || !detail || !detail.run) return;
            if (isFinal(shown.status) && !isFinal(detail.run.status)) return;
            ed.runView.run = detail.run;
            applyRunView(detail);
        }

        // loadLast fills "Last run" data for the input tree when the editor opens.
        async function loadLast() {
            try {
                const runs = (await ed.api.runs(ed.flow.id, { limit: 1 })).runs || [];
                if (!runs.length || disposed) return;
                const detail = await ed.api.run(runs[0].id, false);
                if (disposed) return;
                rememberRun(detail.run, detail.steps || []);
                ed.bus.emit('last-run', detail.run);
            } catch (err) { /* no history yet */ }
        }

        // ── drawer ──────────────────────────────────────────────────────────────

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

        // listedRuns holds the runs the drawer shows, by id (for their Stop buttons).
        let listedRuns = new Map();
        let runsOffset = 0, runsMore = false, runsLoading = false, runsQuery = '';

        function matchingRuns() {
            return Array.from(listedRuns.values()).filter(r => !runsQuery || [r.id, r.status, core.tr(t, 'easydrag.ui.status_' + r.status, r.status), triggerLabel(r), core.fmt.dateTime(r.started_at), r.revision].join(' ').toLocaleLowerCase().includes(runsQuery));
        }

        function drawRuns() {
            if (!drawer) return;
            const list = drawer.querySelector('.ed-runs-list');
            const rows = matchingRuns();
            list.innerHTML = rows.length ? rows.map(runRow).join('') : '<p class="ed-hint">' + esc(t('easydrag.ui.runs_empty')) + '</p>';
            new Set([...stopping, ...durableStopping]).forEach(id => setStopPending(id, true));
            const more = drawer.querySelector('[data-ed-runs-more]');
            more.hidden = !runsMore;
            more.disabled = runsLoading;
            const compare = drawer.querySelector('[data-ed-runs-compare]');
            if (compare) compare.disabled = !ED.compare || rows.filter(r => isFinal(r.status)).length < 2;
        }

        // loadRuns fills the drawer; only the answer to the latest request (filter) is shown. A
        // refresh keeps the rows until its answer. Focus in the list stays in the drawer: on the
        // same run's row while it is listed.
        async function loadRuns(refresh, append) {
            if (!drawer) return;
            // Keep an inspected history prefix stable; explicit Refresh starts a new list.
            if (refresh && runsOffset > 50) return;
            if (append && (runsLoading || !runsMore)) return;
            const mine = ++runsSeq;
            const list = drawer.querySelector('.ed-runs-list');
            const params = { limit: 50, offset: append ? runsOffset : undefined };
            if (drawerFilter === 'errors') params.status = 'error';
            if (drawerFilter === 'tests') params.mode = 'test';
            if (drawerFilter === 'live') params.mode = 'live';
            runsLoading = true;
            drawer.querySelector('[data-ed-runs-more]').disabled = true;
            drawer.querySelector('[data-ed-runs-error]').hidden = true;
            if (!refresh && !append) list.innerHTML = '<p class="ed-hint">' + esc(t('easydrag.ui.loading')) + '</p>';
            try {
                const response = await ed.api.runs(ed.flow.id, params);
                const runs = response.runs || [];
                if (mine !== runsSeq || !drawer) return;
                if (!append) listedRuns = new Map();
                runs.forEach(r => { if (!append || !listedRuns.has(r.id)) listedRuns.set(r.id, r); });
                runsOffset = Number.isInteger(response.next_offset) ? response.next_offset : (params.offset || 0) + runs.length;
                runsMore = typeof response.has_more === 'boolean' ? response.has_more : runs.length === 50;
            } catch (err) {
                if (mine !== runsSeq || !drawer) return;
                if (!append && !refresh) { listedRuns = new Map(); runsOffset = 0; runsMore = false; }
                const error = drawer.querySelector('[data-ed-runs-error]');
                error.textContent = core.errorText(t, err); error.hidden = false;
            }
            if (mine !== runsSeq || !drawer) return;
            runsLoading = false;
            const active = document.activeElement;
            const inList = !!(active && list.contains(active));
            const focusRun = inList ? (active.dataset.edRunStop || active.dataset.edRun || '') : null;
            const onStop = inList && active.dataset.edRunStop !== undefined;
            drawRuns();
            if (focusRun === null) return;
            // The same control when it is still there (the Stop of a run that goes on), else the row.
            const stop = onStop && Array.from(list.querySelectorAll('[data-ed-run-stop]')).find(b => b.dataset.edRunStop === focusRun);
            const target = stop || Array.from(list.querySelectorAll('[data-ed-run]')).find(b => b.dataset.edRun === focusRun) || drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]');
            if (target) target.focus();
        }

        // The drawer takes focus when it opens and gives it back to its opener when it closes.
        function toggleDrawer(force) {
            const open = force === undefined ? !drawer : force;
            if (!open) {
                if (!drawer) return;
                const hadFocus = drawer.contains(document.activeElement);
                drawer.remove();
                drawer = null;
                ++runsSeq; runsLoading = false; runsOffset = 0;
                ed.root.classList.remove('has-drawer');
                if (hadFocus && drawerOpener && typeof drawerOpener.focus === 'function') drawerOpener.focus();
                drawerOpener = null;
                return;
            }
            if (drawer) return;
            drawerOpener = document.activeElement;
            drawer = core.el('<aside class="ed-drawer" aria-label="' + esc(t('easydrag.ui.runs_title')) + '"><header class="ed-drawer-head"><h3>' + esc(t('easydrag.ui.runs_title')) + '</h3>' +
                '<button type="button" class="ed-icon-btn" data-ed-drawer-close aria-label="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></header>' +
                '<div class="ed-seg ed-seg--small" role="radiogroup" aria-label="' + esc(t('easydrag.ui.runs_filter_label')) + '">' + ['all', 'errors', 'tests', 'live'].map(f => '<button type="button" role="radio" aria-checked="' + (f === drawerFilter) + '" data-ed-runs-filter="' + f + '">' + esc(t('easydrag.ui.runs_filter_' + f)) + '</button>').join('') + '</div>' +
                '<div class="ed-runs-tools"><div class="ed-search">' + core.icon('search') + '<input type="search" data-ed-runs-search aria-label="' + esc(t('easydrag.ui.runs_search')) + '" placeholder="' + esc(t('easydrag.ui.runs_search')) + '"></div><button type="button" class="ed-icon-btn" data-ed-runs-refresh title="' + esc(t('easydrag.ui.runs_refresh')) + '" aria-label="' + esc(t('easydrag.ui.runs_refresh')) + '">' + core.icon('refresh') + '</button></div>' +
                '<button type="button" class="ed-btn ed-runs-pager" data-ed-runs-compare disabled>' + esc(t('easydrag.ui.compare_runs')) + '</button><p class="ed-error" data-ed-runs-error role="alert" hidden></p><div class="ed-runs-list"></div><button type="button" class="ed-btn ed-runs-pager" data-ed-runs-more hidden>' + esc(t('easydrag.ui.runs_load_more')) + '</button></aside>');
            (ed.root.querySelector('.ed-body') || ed.root).appendChild(drawer);
            ed.root.classList.add('has-drawer');
            drawer.addEventListener('click', (event) => {
                if (event.target.closest('[data-ed-drawer-close]')) { toggleDrawer(false); return; }
                if (event.target.closest('[data-ed-runs-more]')) { loadRuns(false, true); return; }
                if (event.target.closest('[data-ed-runs-refresh]')) { loadRuns(false); return; }
                if (event.target.closest('[data-ed-runs-compare]')) { if (ED.compare) ED.compare.open(ed, matchingRuns()); return; }
                const f = event.target.closest('[data-ed-runs-filter]');
                if (f) {
                    drawerFilter = f.dataset.edRunsFilter;
                    drawer.querySelectorAll('[data-ed-runs-filter]').forEach(b => b.setAttribute('aria-checked', String(b === f)));
                    loadRuns();
                    return;
                }
                const stop = event.target.closest('[data-ed-run-stop]');
                if (stop) { stopRun(listedRuns.get(stop.dataset.edRunStop)); return; }
                const row = event.target.closest('[data-ed-run]');
                if (row) openRunView(row.dataset.edRun);
            });
            drawer.querySelector('[data-ed-runs-search]').value = runsQuery;
            drawer.querySelector('[data-ed-runs-search]').addEventListener('input', event => { runsQuery = event.target.value.trim().toLocaleLowerCase(); drawRuns(); });
            loadRuns();
            const first = drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]') || drawer.querySelector('button');
            if (first) first.focus();
        }

        // openRunView fetches a stored run for the run view: a second click on the same run while
        // it loads does nothing, and only the run clicked last is shown.
        async function openRunView(runId, gateId) {
            if (disposed || viewing === runId) return;
            if (ed.runView && ed.runView.run.id !== runId && ED.approvals) ED.approvals.closeRunReview(ed);
            viewing = runId;
            const mine = ++viewSeq;
            try {
                const detail = await ed.api.run(runId, true);
                if (disposed || mine !== viewSeq) return;
                if (!detail.doc) {
                    ED.childHistory.missing(ed, detail);
                    return;
                }
                ed.enterRunView(detail);
                if (gateId && ED.approvals && ed.runView && ed.runView.run.id === runId)
                    ED.approvals.openRunReview(ed, gateId, { runId, flowId: ed.flow.id, revision: ed.runView.run.revision });
            } catch (err) {
                if (!disposed && mine === viewSeq) ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: core.errorText(t, err), type: 'error' });
            } finally {
                if (viewing === runId) viewing = null;
            }
        }

        function stepsFrom(steps, record, durableStatus, statusError) {
            const map = new Map();
            (steps || []).forEach(s => map.set(s.node_id, s));
            const observed = ED.durable ? ED.durable.status(record, durableStatus) : null;
            return { id: record.id, status: record.status, mode: record.mode, steps: map, changes: [], record, durableStatus: observed, durableKnown: !!observed, durableStatusError: statusError === 'storage_unavailable', durableUnknown: statusError === 'storage_unavailable', durableStopping: durableStopping.has(record.id) };
        }

        // applyRunView shows a stored run in the run view, with its own data in the input tree
        // (ed.runView.data). A live run that is still going is parked: its stream closes, and
        // clearRun() (leaving the run view) attaches it again.
        function applyRunView(detail) {
            if (ed.run && !ed.run.view && !isFinal(ed.run.status) && !ed.run.stale) { parked = { id: ed.run.id, mode: ed.run.mode }; watchParked(); }
            closeStream();
            const shown = stepsFrom(detail.steps, detail.run, detail.durable_status, detail.durable_status_error);
            shown.view = true;
            shown.finished = true;
            if (ed.runView) ed.runView.data = runData(detail.run, detail.steps || []);
            setRun(shown);
        }

        function clearRun() {
            closeStream();
            const back = parked;
            parked = null;
            clearTimeout(parkTimer);
            if (back && !disposed) { attach(back.id, { mode: back.mode }); return; }
            setRun(null);
        }

        bag.add(closeStream);
        bag.add(ed.bus.on('refresh-durable-run', refreshDurableShown));
        bag.add(() => runReads.dispose());
        bag.add(() => clearTimeout(parkTimer));
        bag.add(() => { stopPolls.forEach(clearTimeout); stopPolls.clear(); });
        bag.add(() => runFinished.cancel());
        bag.add(() => toggleDrawer(false));
        bag.add(() => { if (ED.compare) ED.compare.close(ed); });

        return {
            startTest, attach, runLive, cancel, stopRun, refreshShown, runFinished, loadLast, toggleDrawer, openRunView, effects, stepsFrom, applyRunView, clearRun,
            isRunning, hasParked: () => !!parked,
            drawerOpen: () => !!drawer,
            dispose() { disposed = true; parked = null; bag.dispose(); }
        };
    }

    ED.runs = { create, effects, unknownEffects, triggers, edgeStates, isFinal, isActive };
})();
