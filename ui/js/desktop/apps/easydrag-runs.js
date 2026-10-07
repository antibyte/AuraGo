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
    // after 1, 2 and 4 s until it ended: a test run announces its end to no other window.
    const STOP_POLL_MS = [1000, 2000, 4000];
    // REDACTED is the server's placeholder for secret values in scrubbed test data.
    const REDACTED = '[redacted]';
    const EFFECT_ICONS = { sends_message: 'brand-telegram', writes_files: 'file-pencil', controls_devices: 'home', runs_code: 'api', deletes: 'trash', system_change: 'settings' };

    function isFinal(status) { return ['success', 'error', 'cancelled'].includes(status); }
    // isActive: the run has not ended yet (queued, waiting for a slot, or running).
    function isActive(status) { return ['queued', 'waiting', 'running'].includes(status); }

    function triggers(ed) {
        return ed.model.doc.nodes.filter(n => { const i = ed.model.info(n.type); return i && i.trigger && !n.settings.disabled; });
    }

    // realEffects reads the effects of each step's real settings from the publish preview
    // (CollectEffects: [{effect, node_ids}]) as node id -> effects; null when the answer has none.
    function realEffects(preview) {
        if (!preview || !Array.isArray(preview.effects)) return null;
        const out = new Map();
        preview.effects.forEach(item => {
            if (!item || typeof item.effect !== 'string' || !Array.isArray(item.node_ids)) return;
            item.node_ids.forEach(id => {
                if (!out.has(id)) out.set(id, []);
                out.get(id).push(item.effect);
            });
        });
        return out;
    }

    // effects lists the outward effects of the steps a test runs (for the real-effects warning).
    // Each step counts with its catalog effects, which describe its default settings, and with
    // the effects of its real settings when the server named them (real: node id -> effects,
    // from realEffects): an HTTP request with POST sends, while its catalog entry (GET) does not.
    // A test of the whole flow counts every enabled step. A step test (onlyNode) runs what the
    // engine runs (internal/flows/engine_state.go, run, fireTrigger and collectReady): the step
    // and its ancestors, and of those only the steps the test's trigger reaches. Other triggers
    // are skipped, and a disabled step is skipped with what only it feeds. Every wire counts,
    // whatever its port, so the walk deliberately over-estimates what a branch will run.
    // triggerId is the test's trigger; without one, every enabled trigger counts.
    function effects(ed, onlyNode, triggerId, real) {
        const doc = ed.model.doc;
        const out = new Map();
        const add = n => {
            const i = ed.model.info(n.type);
            const kinds = new Set((i && i.effects) || []);
            ((real && real.get(n.id)) || []).forEach(effect => kinds.add(effect));
            kinds.forEach(effect => {
                if (!out.has(effect)) out.set(effect, []);
                out.get(effect).push(n.label || n.type);
            });
        };
        if (!onlyNode) {
            doc.nodes.forEach(n => { if (!n.settings.disabled) add(n); });
            return out;
        }
        const isTrigger = n => { const i = ed.model.info(n.type); return !!(i && i.trigger); };
        // Wires to a missing step are ignored, as buildGraph does.
        const incoming = new Map();
        const outgoing = new Map();
        doc.edges.forEach(e => {
            if (!ed.model.node(e.source.node) || !ed.model.node(e.target.node)) return;
            if (!incoming.has(e.target.node)) incoming.set(e.target.node, []);
            if (!outgoing.has(e.source.node)) outgoing.set(e.source.node, []);
            incoming.get(e.target.node).push(e.source.node);
            outgoing.get(e.source.node).push(e.target.node);
        });
        // The scope: onlyNode and every step with a path to it (graph.ancestors).
        const scope = new Set([onlyNode]);
        for (const stack = [onlyNode]; stack.length;) {
            (incoming.get(stack.pop()) || []).forEach(id => { if (!scope.has(id)) { scope.add(id); stack.push(id); } });
        }
        // What runs: the steps of the scope that the trigger reaches through enabled steps.
        const starts = (triggerId ? [ed.model.node(triggerId)] : triggers(ed)).filter(n => n && isTrigger(n) && !n.settings.disabled).map(n => n.id);
        const runs = new Set();
        for (const stack = starts.slice(); stack.length;) {
            (outgoing.get(stack.pop()) || []).forEach(id => {
                const n = ed.model.node(id);
                if (runs.has(id) || !scope.has(id) || n.settings.disabled || isTrigger(n)) return;
                runs.add(id);
                stack.push(id);
            });
        }
        doc.nodes.forEach(n => { if (runs.has(n.id)) add(n); });
        return out;
    }

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
        let drawer = null;
        let drawerOpener = null;
        let drawerFilter = 'all';
        let runsSeq = 0;
        let viewSeq = 0;
        let viewing = null;
        let disposed = false;

        function setRun(next) {
            ed.run = next;
            if (ed.run) ed.run.edges = edgeStates(ed);
            ed.bus.emit('run', ed.run);
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

        // ── stream ──────────────────────────────────────────────────────────────

        function closeStream() {
            clearTimeout(reconnectTimer);
            if (source) { source.close(); source = null; }
        }

        function attach(runId, meta) {
            closeStream();
            lastSeq = 0;
            retryDelay = RETRY_MS;
            failures = 0;
            parked = null;
            setRun({ id: runId, status: 'queued', mode: (meta && meta.mode) || 'test', steps: new Map(), record: null, error: '' });
            connect(runId);
        }

        // streamed reports whether runId is the live run shown. A run view of the same run (the
        // live run parked) does not count: nothing may stream into the viewed run.
        function streamed(runId) { return !!(ed.run && ed.run.id === runId && !ed.run.view); }

        function connect(runId) {
            if (!streamed(runId)) return;
            const es = new EventSource(ed.api.eventsUrl(runId, lastSeq || undefined));
            source = es;
            // Every message shows the stream works: the next failure waits RETRY_MS again.
            const alive = () => { retryDelay = RETRY_MS; failures = 0; };
            es.addEventListener('snapshot', (event) => {
                alive();
                const detail = JSON.parse(event.data);
                if (!streamed(runId)) return;
                ed.run.record = detail.run;
                ed.run.status = detail.run.status;
                (detail.steps || []).forEach(s => ed.run.steps.set(s.node_id, s));
                setRun(ed.run);
            });
            es.addEventListener('event', (event) => {
                alive();
                const ev = JSON.parse(event.data);
                if (!streamed(runId)) return;
                lastSeq = Math.max(lastSeq, ev.seq || 0);
                applyEvent(ev);
            });
            // resync: the server dropped this stream (a slow client), but the run goes on. A new
            // stream continues at once after the last event; a finished run answers it with "end".
            es.addEventListener('resync', (event) => {
                alive();
                es.close();
                if (source === es) source = null;
                if (!streamed(runId)) return;
                let after = NaN;
                try { after = Number(JSON.parse(event.data).after); } catch (err) { /* keep lastSeq */ }
                if (Number.isFinite(after)) lastSeq = Math.max(lastSeq, after);
                connect(runId);
            });
            es.addEventListener('end', () => { alive(); es.close(); source = null; finish(runId); });
            es.onerror = () => {
                es.close();
                source = null;
                if (!streamed(runId)) return;
                // The stream broke after run_finished but before "end": load the result now.
                if (isFinal(ed.run.status)) { finish(runId); return; }
                failures++;
                if (failures >= MAX_FAILURES) { failures = 0; probe(runId); return; }
                reconnectLater(runId);
            };
        }

        function reconnectLater(runId) {
            clearTimeout(reconnectTimer);
            reconnectTimer = setTimeout(() => connect(runId), retryDelay);
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
            const status = Number(failure && failure.status) || 0;
            const code = core.errorCode(failure);
            if (failure && (status === 404 || status === 503 || code === 'FLOW_RUN_NOT_FOUND' || code === 'FLOWS_DISABLED')) {
                closeStream();
                ed.run.stale = true;
                ed.run.error = core.errorText(t, failure);
                setRun(ed.run);
                ed.ctx.notify({ title: t('easydrag.ui.run_stream_lost'), message: ed.run.error, type: 'error' });
                return;
            }
            reconnectLater(runId);
        }

        // isRunning: a live run is active (not a run view, not one whose stream was given up).
        function isRunning() { return !!(ed.run && !ed.run.view && !ed.run.stale && !isFinal(ed.run.status)); }

        function applyEvent(ev) {
            if (ev.type === 'run_started') ed.run.status = 'running';
            else if (ev.type === 'step_started') ed.run.steps.set(ev.node_id, Object.assign({}, ed.run.steps.get(ev.node_id), { node_id: ev.node_id, status: 'running' }));
            else if (ev.type === 'step_finished' && ev.step) ed.run.steps.set(ev.node_id, ev.step);
            else if (ev.type === 'run_finished' && ev.run) {
                ed.run.status = ev.run.status;
                ed.run.error = ev.run.error_message || '';
                ed.run.errorCode = ev.run.error_code || '';
                ed.run.duration = ev.run.duration_ms;
            }
            setRun(ed.run);
        }

        // finish loads the stored result of a run (known: a detail fetched already) and announces
        // it, once per run: "end" and a cancel that found the run finished may both get here. A
        // run that another run (or a run view) replaced during the fetch is left alone.
        async function finish(runId, known) {
            const current = ed.run;
            if (!current || current.id !== runId || current.finished) return;
            current.finished = true;
            let detail = known || null;
            if (!detail) {
                try { detail = await ed.api.run(runId, false); } catch (err) { detail = null; /* keep the streamed state */ }
            }
            if (disposed || ed.run !== current) return;
            if (detail && detail.run) {
                current.record = detail.run;
                current.status = detail.run.status;
                (detail.steps || []).forEach(s => current.steps.set(s.node_id, s));
                rememberRun(detail.run, detail.steps || []);
                // The finished run is the last run now: the footer keeps naming it once the run
                // state is cleared (leaving a run view), as after loadLast.
                ed.bus.emit('last-run', detail.run);
            }
            setRun(ed.run);
            const failed = Array.from(ed.run.steps.values()).find(s => s.status === 'error');
            const node = failed && ed.model.node(failed.node_id);
            canvas.announce(ed.run.status === 'success' ? t('easydrag.ui.run_done') : node ? t('easydrag.ui.run_failed_at', { node: node.label || node.type }) : t('easydrag.ui.run_failed'));
            if (drawer) loadRuns(true);
        }

        // ── starting runs ───────────────────────────────────────────────────────

        // Real effects are confirmed one by one (sends_message, deletes, …): confirming the effects
        // of one step does not cover other effects of the flow. ed.effectsConfirmed holds this
        // session's set; "remember" stores the list under EFFECTS_KEY. An older stored `true` (one
        // switch for the whole flow) counts as nothing confirmed, so the warning comes once more.
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

        function effectsMarkup(list) {
            if (!list.size) return '';
            return '<div class="ed-callout ed-callout--warn" data-ed-test-effects>' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.effects_title')) + '</strong><ul class="ed-effects">' +
                Array.from(list.entries()).map(([effect, nodes]) => '<li data-ed-effect="' + esc(effect) + '">' + core.icon(EFFECT_ICONS[effect] || 'bolt') + '<span>' + esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect)) + '</span><span class="ed-muted">' + esc(nodes.join(', ')) + '</span></li>').join('') +
                '</ul><label class="ed-check"><input type="checkbox" data-ed-effects-skip> ' + esc(t('easydrag.ui.effects_remember')) + '</label></div></div>';
        }

        // starting is true from a startTest call until its run was posted or its dialog closed:
        // one test at a time, and none while a run is active (it would orphan that run's stream).
        let starting = false;

        async function startTest(opts) {
            if (disposed || ed.readonly || ed.runView || starting || isRunning()) return undefined;
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
            // A test runs the saved draft: an invalid, offline or conflicting draft is not tested
            // silently in its last saved version.
            if (ed.saver && !(await ed.saver.flush())) {
                if (!disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                return undefined;
            }
            if (disposed) return undefined;
            const list = triggers(ed);
            if (!list.length) { ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_no_trigger'), type: 'error' }); return undefined; }
            const remembered = core.storage.get('aurago.easydrag.test-trigger.' + ed.flow.id, '');
            let trigger = list.find(n => n.id === (o.triggerNode || remembered)) || list.find(n => n.type === 'trigger.manual') || list[0];
            // The catalog describes each step with its default settings; the publish preview (a
            // read-only check of the saved draft) names the effects of the real ones. Without it
            // the dialog says that the effects could not be fully checked, and a test never starts
            // without the dialog.
            let real = null;
            try { real = realEffects(await ed.api.preview(ed.flow.id)); } catch (err) { real = null; }
            if (disposed) return undefined;
            const confirmed = confirmedEffects();
            // A step test runs what its trigger reaches: the effects follow the chosen trigger.
            const pending = id => new Map(Array.from(effects(ed, o.onlyNode, id, real)).filter(([effect]) => !confirmed.has(effect)));
            let fx = pending(trigger.id);
            if (o.quick && !fx.size && real) { await run(trigger.id, null, o.onlyNode, false); return undefined; }
            let sample = {};
            try { sample = (await ed.api.testData(ed.flow.id, trigger.id)).data || {}; } catch (err) { sample = {}; }
            if (disposed) return undefined;
            // The sample comes scrubbed (secret values read "[redacted]") and must never be saved
            // back: unchanged text runs without trigger_data, so the server uses the stored sample.
            let shown = JSON.stringify(sample, null, 2);
            const dialog = core.modal(ed.root, {
                title: o.onlyNode ? t('easydrag.ui.test_node_title') : t('easydrag.ui.test_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--test',
                body: (list.length > 1 ? '<label class="ed-label">' + esc(t('easydrag.ui.test_trigger')) + '<select class="ed-input" data-ed-test-trigger>' +
                    list.map(n => '<option value="' + esc(n.id) + '"' + (n.id === trigger.id ? ' selected' : '') + '>' + esc(n.label || n.type) + '</option>').join('') + '</select></label>' : '') +
                    '<label class="ed-label">' + esc(t('easydrag.ui.test_data')) + '<textarea class="ed-input ed-code" rows="9" spellcheck="false" data-ed-test-data>' + esc(shown) + '</textarea></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.test_data_hint')) + '</p><p class="ed-error" role="alert" hidden></p>' +
                    '<label class="ed-check"><input type="checkbox" data-ed-test-remember checked> ' + esc(t('easydrag.ui.test_remember')) + '</label>' +
                    (real ? '' : '<p class="ed-callout ed-callout--warn" data-ed-effects-unchecked>' + core.icon('alert') + '<span>' + esc(t('easydrag.ui.effects_unchecked')) + '</span></p>') +
                    effectsMarkup(fx),
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'run', label: t('easydrag.ui.test_run'), primary: true, icon: 'play' }],
                onAction: async (action, d) => {
                    if (action !== 'run') return true;
                    const err = d.body.querySelector('.ed-error');
                    const text = d.body.querySelector('[data-ed-test-data]').value;
                    const edited = text !== shown;
                    let data = null;
                    if (edited) {
                        try { data = JSON.parse(text || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.json_invalid'); return false; }
                        if (!data || typeof data !== 'object' || Array.isArray(data)) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_object'); return false; }
                    }
                    const sel = d.body.querySelector('[data-ed-test-trigger]');
                    if (sel) trigger = ed.model.node(sel.value) || trigger;
                    const skip = d.body.querySelector('[data-ed-effects-skip]');
                    if (fx.size) confirmEffects(Array.from(fx.keys()), !!(skip && skip.checked));
                    core.storage.set('aurago.easydrag.test-trigger.' + ed.flow.id, trigger.id);
                    // Edited data that still holds a placeholder runs, but is not remembered: it
                    // would replace the stored secret values with "[redacted]".
                    const remember = edited && d.body.querySelector('[data-ed-test-remember]').checked;
                    const redacted = remember && text.includes(REDACTED);
                    return run(trigger.id, data, o.onlyNode, remember && !redacted).then(ok => {
                        if (ok && redacted) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_data_redacted') });
                        return ok ? true : false;
                    });
                }
            });
            // Another trigger shows its own sample, unless the text was edited. For a step test it
            // also shows the effects of what that trigger reaches.
            const select = dialog.body.querySelector('[data-ed-test-trigger]');
            if (select) {
                select.addEventListener('change', async () => {
                    if (o.onlyNode) {
                        const old = dialog.body.querySelector('[data-ed-test-effects]');
                        const skip = old && old.querySelector('[data-ed-effects-skip]');
                        const remember = !!(skip && skip.checked);
                        if (old) old.remove();
                        fx = pending(select.value);
                        if (fx.size) {
                            const box = dialog.body.appendChild(core.el(effectsMarkup(fx)));
                            box.querySelector('[data-ed-effects-skip]').checked = remember;
                        }
                    }
                    const area = dialog.body.querySelector('[data-ed-test-data]');
                    const id = select.value;
                    if (area.value !== shown) return;
                    let next = {};
                    try { next = (await ed.api.testData(ed.flow.id, id)).data || {}; } catch (err) { next = {}; }
                    if (select.value !== id || area.value !== shown) return;
                    shown = JSON.stringify(next, null, 2);
                    area.value = shown;
                });
            }
            return dialog;
        }

        async function run(triggerNode, data, onlyNode, remember) {
            try {
                const body = { trigger_node: triggerNode, only_node: onlyNode || undefined, remember_data: !!remember };
                if (data) body.trigger_data = data;
                const res = await ed.api.test(ed.flow.id, body);
                if (res.run_id && !disposed) attach(res.run_id, { mode: 'test' });
                return true;
            } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: core.errorText(t, err), type: 'error' });
                return false;
            }
        }

        async function runLive() {
            if (disposed || ed.runView || starting || isRunning()) return;
            try {
                const res = await ed.api.runNow(ed.flow.id);
                if (res.run_id && !disposed) attach(res.run_id, { mode: 'live' });
            } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: core.errorText(t, err), type: 'error' });
            }
        }

        async function cancel() {
            if (!ed.run || isFinal(ed.run.status)) return;
            const runId = ed.run.id;
            try { await ed.api.cancel(runId); } catch (err) {
                // The run ended before the cancel arrived: show its result, not an error.
                if (core.errorCode(err) === 'FLOW_RUN_FINISHED') { await settleFinished(runId); return; }
                ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' });
            }
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
        // stopPolls holds the timers of followStop.
        const stopPolls = new Set();

        // stopRun stops a run of the drawer or the run view, also one this window did not start: a
        // live run of a trigger that waits for a slot, or another window's test. Any run but a
        // test stops only after a confirmation. 202, and 409 FLOW_RUN_FINISHED (it ended
        // meanwhile), refresh the list and the viewed run quietly; other errors are shown.
        async function stopRun(record) {
            if (disposed || ed.readonly || !record || !isActive(record.status) || stopping.has(record.id)) return false;
            stopping.add(record.id);
            try {
                if (record.mode !== 'test' && !(await confirmStop(record))) return false;
                if (disposed) return false;
                let accepted = true;
                try { await ed.api.cancel(record.id); } catch (err) {
                    if (core.errorCode(err) !== 'FLOW_RUN_FINISHED') {
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
            } finally {
                stopping.delete(record.id);
            }
        }

        // followStop asks for a stopped run's stored state after STOP_POLL_MS[attempt] and refreshes
        // the drawer and the run view once it ended, while they still show it as not ended.
        function followStop(runId, attempt) {
            const timer = setTimeout(async () => {
                stopPolls.delete(timer);
                let detail = null;
                try { detail = await ed.api.run(runId, false); } catch (err) { detail = null; }
                if (disposed) return;
                if (detail && detail.run && isFinal(detail.run.status)) {
                    if (showsActive(runId)) refreshShown();
                    return;
                }
                if (attempt + 1 < STOP_POLL_MS.length) followStop(runId, attempt + 1);
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

        // refreshShown brings the drawer's list, and the run view of a run that had not ended, up
        // to date: after a stop, and when a live run ended (flows_changed "run_finished").
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
                '<span class="ed-run-main"><span>' + esc(core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(triggerLabel(r)) + '</span>' +
                '<span class="ed-muted">' + esc(core.fmt.dateTime(r.started_at)) + (r.duration_ms ? ' · ' + esc(core.fmt.duration(r.duration_ms)) : '') + '</span></span>' +
                '<span class="ed-chip' + (r.mode === 'test' ? ' ed-chip--muted' : '') + '">' + esc(r.mode === 'test' ? t('easydrag.ui.run_mode_test') : t('easydrag.ui.run_mode_live')) + '</span></button>';
            // A run that has not ended can be stopped from its row (not on a read-only desktop).
            if (ed.readonly || !isActive(r.status)) return row;
            return '<div class="ed-run-item">' + row + '<button type="button" class="ed-btn ed-btn--small ed-btn--danger" data-ed-run-stop="' + esc(r.id) + '" aria-label="' +
                esc(t('easydrag.ui.run_stop_label', { time: core.fmt.dateTime(r.started_at, true) })) + '">' + core.icon('player-stop') + '<span>' + esc(t('easydrag.ui.run_cancel')) + '</span></button></div>';
        }

        // listedRuns holds the runs the drawer shows, by id (for their Stop buttons).
        let listedRuns = new Map();

        // loadRuns fills the drawer; only the answer to the latest request (filter) is shown. A
        // refresh keeps the rows until its answer. Focus in the list stays in the drawer: on the
        // same run's row while it is listed.
        async function loadRuns(refresh) {
            if (!drawer) return;
            const mine = ++runsSeq;
            const list = drawer.querySelector('.ed-runs-list');
            const params = { limit: 50 };
            if (drawerFilter === 'errors') params.status = 'error';
            if (drawerFilter === 'tests') params.mode = 'test';
            if (drawerFilter === 'live') params.mode = 'live';
            if (!refresh) list.innerHTML = '<p class="ed-hint">' + esc(t('easydrag.ui.loading')) + '</p>';
            let markup;
            try {
                const runs = (await ed.api.runs(ed.flow.id, params)).runs || [];
                if (mine !== runsSeq || !drawer) return;
                listedRuns = new Map(runs.map(r => [r.id, r]));
                markup = runs.length ? runs.map(runRow).join('') : '<p class="ed-hint">' + esc(t('easydrag.ui.runs_empty')) + '</p>';
            } catch (err) {
                if (mine !== runsSeq || !drawer) return;
                listedRuns = new Map();
                markup = '<p class="ed-error">' + esc(core.errorText(t, err)) + '</p>';
            }
            const active = document.activeElement;
            const inList = !!(active && list.contains(active));
            const focusRun = inList ? (active.dataset.edRunStop || active.dataset.edRun || '') : null;
            const onStop = inList && active.dataset.edRunStop !== undefined;
            list.innerHTML = markup;
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
                ed.root.classList.remove('has-drawer');
                if (hadFocus && drawerOpener && typeof drawerOpener.focus === 'function') drawerOpener.focus();
                drawerOpener = null;
                return;
            }
            if (drawer) return;
            drawerOpener = document.activeElement;
            drawer = core.el('<aside class="ed-drawer" aria-label="' + esc(t('easydrag.ui.runs_title')) + '"><header class="ed-drawer-head"><h3>' + esc(t('easydrag.ui.runs_title')) + '</h3>' +
                '<button type="button" class="ed-icon-btn" data-ed-drawer-close aria-label="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></header>' +
                '<div class="ed-seg ed-seg--small" role="radiogroup">' + ['all', 'errors', 'tests', 'live'].map(f => '<button type="button" role="radio" aria-checked="' + (f === drawerFilter) + '" data-ed-runs-filter="' + f + '">' + esc(t('easydrag.ui.runs_filter_' + f)) + '</button>').join('') + '</div>' +
                '<div class="ed-runs-list"></div></aside>');
            ed.root.appendChild(drawer);
            ed.root.classList.add('has-drawer');
            drawer.addEventListener('click', (event) => {
                if (event.target.closest('[data-ed-drawer-close]')) { toggleDrawer(false); return; }
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
            loadRuns();
            const first = drawer.querySelector('[data-ed-runs-filter][aria-checked="true"]') || drawer.querySelector('button');
            if (first) first.focus();
        }

        // openRunView fetches a stored run for the run view: a second click on the same run while
        // it loads does nothing, and only the run clicked last is shown.
        async function openRunView(runId) {
            if (disposed || viewing === runId) return;
            viewing = runId;
            const mine = ++viewSeq;
            try {
                const detail = await ed.api.run(runId, true);
                if (disposed || mine !== viewSeq) return;
                if (!detail.doc) { ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: t('easydrag.ui.run_view_missing'), type: 'error' }); return; }
                ed.enterRunView(detail);
            } catch (err) {
                if (!disposed && mine === viewSeq) ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: core.errorText(t, err), type: 'error' });
            } finally {
                if (viewing === runId) viewing = null;
            }
        }

        function stepsFrom(steps, record) {
            const map = new Map();
            (steps || []).forEach(s => map.set(s.node_id, s));
            return { id: record.id, status: record.status, mode: record.mode, steps: map, record };
        }

        // applyRunView shows a stored run in the run view, with its own data in the input tree
        // (ed.runView.data). A live run that is still going is parked: its stream closes, and
        // clearRun() (leaving the run view) attaches it again.
        function applyRunView(detail) {
            if (ed.run && !ed.run.view && !isFinal(ed.run.status) && !ed.run.stale) parked = { id: ed.run.id, mode: ed.run.mode };
            closeStream();
            const shown = stepsFrom(detail.steps, detail.run);
            shown.view = true;
            shown.finished = true;
            if (ed.runView) ed.runView.data = runData(detail.run, detail.steps || []);
            setRun(shown);
        }

        function clearRun() {
            closeStream();
            const back = parked;
            parked = null;
            if (back && !disposed) { attach(back.id, { mode: back.mode }); return; }
            setRun(null);
        }

        bag.add(closeStream);
        bag.add(() => { stopPolls.forEach(clearTimeout); stopPolls.clear(); });
        bag.add(() => toggleDrawer(false));

        return {
            startTest, attach, runLive, cancel, stopRun, refreshShown, loadLast, toggleDrawer, openRunView, effects, stepsFrom, applyRunView, clearRun,
            isRunning,
            drawerOpen: () => !!drawer,
            dispose() { disposed = true; parked = null; bag.dispose(); }
        };
    }

    ED.runs = { create, effects, triggers, edgeStates, isFinal, isActive };
})();
