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
        // The code that has not moved into a part yet calls the parts' functions by these names.
        const { attach, closeStream, finish, isRunning, readDurable, rememberRun, rootsFrom, runData, setRun, snapshot, streamed, unknownDurable } = rc;

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

        function trackStart(work) {
            if (rc.startPost) return rc.startPost;
            const pending = work().finally(() => { if (rc.startPost === pending) rc.startPost = null; });
            rc.startPost = pending;
            return pending;
        }

        function editsReady() {
            return !(ed.flushPendingEdits && ed.flushPendingEdits() === false);
        }

        async function startTest(opts) {
            if (rc.disposed || ed.readonly || ed.runView || rc.starting || rc.startPost || isRunning()) return undefined;
            rc.starting = true;
            let dialog;
            try {
                dialog = await openTest(opts || {});
            } finally {
                if (!dialog) rc.starting = false;
            }
            if (dialog) dialog.done.then(() => { rc.starting = false; });
            return dialog;
        }

        async function openTest(o) {
            // Field and note edits land before the draft is saved for the test.
            if (!editsReady()) return undefined;
            // A test runs the saved draft: an invalid, offline or conflicting draft is not tested
            // silently in its last saved version.
            if (ed.saver && !(await ed.saver.flush())) {
                if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                return undefined;
            }
            if (rc.disposed) return undefined;
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
            if (rc.disposed) return undefined;
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
                    if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                    return undefined;
                }
                if (rc.disposed) return undefined;
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
            if (rc.disposed) return undefined;
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
                        if (rc.disposed) return false;
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
                isCurrent: () => !rc.disposed && !ed.runView && ed.model.version === version,
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
                    if (res && res.run_id && !rc.disposed) { attach(res.run_id, { mode: 'test', draftRevision: rev }); canvas.announce(t('easydrag.ui.announce_test_started')); }
                    return true;
                } catch (err) {
                    ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: core.errorText(t, err), type: 'error' });
                    return false;
                }
            });
        }

        function runLive() {
            if (rc.disposed || ed.runView || rc.starting || rc.startPost || isRunning()) return undefined;
            return trackStart(async () => {
                try {
                    if (ED.durable && ED.durable.requires(ed.flow.live)) {
                        const capability = ed.runtime && await ed.runtime.refresh(true);
                        if (rc.disposed || ed.readonly || ed.runView) return;
                        if (!capability || !capability.effective || !capability.recovery_available) {
                            ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: capability ? ED.durable.reason(t, capability.reason) : t('easydrag.ui.durable_unknown'), type: 'error' });
                            return;
                        }
                    }
                    const res = await ed.api.runNow(ed.flow.id);
                    if (rc.disposed) return;
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
            if (rc.disposed || !detail || !detail.run || !isFinal(detail.run.status)) return;
            if (!ed.run || ed.run.id !== runId || isFinal(ed.run.status)) return;
            closeStream();
            await finish(runId, detail);
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
            if (rc.disposed || !record || !isActive(record.status) && !checkpoint?.cancel_available || rc.stopping.has(record.id) || rc.durableStopping.has(record.id)) return false;
            rc.stopping.add(record.id);
            let pending = false;
            try {
                let generation;
                if (durableRun) {
                    // Refresh before confirming; never manufacture generation zero or infer
                    // cancellation permission from a display phase or the persisted marker.
                    const detail = await readDurable(record.id);
                    if (rc.disposed || !detail?.run || detail.run.id !== record.id) return false;
                    const state = ED.durable.status(detail.run, detail.durable_status);
                    if (ed.run?.id === record.id) snapshot(detail, ed.run, true);
                    if (!state?.cancel_available) return false;
                    generation = state.generation;
                    record = detail.run;
                }
                if (record.mode !== 'test' && !(await confirmStop(record))) return false;
                if (rc.disposed) return false;
                pending = true;
                setStopPending(record.id, true);
                let accepted = true;
                try {
                    if (durableRun) {
                        const answer = await ed.api.cancelDurable(record.id, generation);
                        if (!answer || answer.accepted !== true) throw new Error(t('easydrag.ui.durable_unknown'));
                        rc.durableStopping.add(record.id);
                        if (ed.run?.id === record.id) { ed.run.durableStopping = true; setRun(ed.run); }
                        const detail = await readDurable(record.id);
                        if (!rc.disposed && ed.run?.id === record.id) snapshot(detail, ed.run, true);
                        settleDurableStop(detail);
                    } else await ed.api.cancel(record.id);
                } catch (err) {
                    if (durableRun && core.errorCode(err) === 'FLOW_DURABLE_GENERATION_CONFLICT') {
                        try { const detail = await readDurable(record.id); if (!rc.disposed && ed.run?.id === record.id) snapshot(detail, ed.run, true); } catch (_) { /* Preserve unknown state. */ }
                    }
                    if (core.errorCode(err) !== 'FLOW_RUN_FINISHED') {
                        if (durableRun && ed.run?.id === record.id) { ed.run.durableUnknown = true; setRun(ed.run); }
                        if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' });
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
                if (durableRun && !rc.disposed && ed.run?.id === record.id) unknownDurable(ed.run, err);
                if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' });
                return false;
            } finally {
                if (pending && !rc.durableStopping.has(record.id)) setStopPending(record.id, false);
                rc.stopping.delete(record.id);
            }
        }

        function settleDurableStop(detail) {
            if (!detail?.run || !rc.durableStopping.has(detail.run.id)) return;
            const state = ED.durable.status(detail.run, detail.durable_status);
            if (isFinal(detail.run.status) && !state?.cancel_available) {
                rc.durableStopping.delete(detail.run.id); setStopPending(detail.run.id, false);
                if (ed.run?.id === detail.run.id && ed.run.durableStopping) { ed.run.durableStopping = false; setRun(ed.run); }
            }
        }

        // Parked runs have no stream, so poll until they finish or disappear.
        function watchParked() {
            clearTimeout(rc.parkTimer);
            if (!rc.parked || rc.disposed) return;
            const id = rc.parked.id;
            rc.parkTimer = setTimeout(async () => {
                let detail = null;
                let gone = false;
                try { detail = await ed.api.run(id, false); } catch (err) { gone = Number(err && err.status) === 404; }
                if (rc.disposed || !rc.parked || rc.parked.id !== id) return;
                if (detail && detail.run && isFinal(detail.run.status)) {
                    rc.parked = null;
                    rememberRun(detail.run, detail.steps || []);
                    ed.bus.emit('last-run', detail.run);
                    ed.bus.emit('run', ed.run);
                    if (rc.drawer) loadRuns(true);
                    return;
                }
                if (gone) { rc.parked = null; ed.bus.emit('run', ed.run); return; }
                watchParked();
            }, PARK_POLL_MS);
        }

        // A stopped run may finish without a stream reaching this window.
        function followStop(runId, attempt) {
            const timer = setTimeout(async () => {
                rc.stopPolls.delete(timer);
                let detail = null;
                try { detail = await ed.api.run(runId, false); } catch (err) { detail = null; }
                if (rc.disposed) return;
                settleDurableStop(detail);
                if (detail && detail.run && isFinal(detail.run.status)) {
                    if (showsActive(runId)) refreshShown();
                    return;
                }
                if (attempt + 1 < STOP_POLL_MS.length || rc.durableStopping.has(runId)) followStop(runId, Math.min(attempt + 1, STOP_POLL_MS.length - 1));
            }, STOP_POLL_MS[attempt]);
            rc.stopPolls.add(timer);
        }

        function showsActive(runId) {
            const listed = rc.drawer && rc.listedRuns.get(runId);
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

        // Coalesce bursts without starving refresh during a steady stream of completions.
        const runFinished = core.debounce(() => {
            if (rc.disposed) return;
            if (!isRunning()) loadLast();
            refreshShown();
        }, 250, 1000);

        function refreshShown() {
            if (rc.disposed) return;
            if (rc.drawer) loadRuns(true);
            const shown = ed.runView && ed.runView.run;
            if (shown && isActive(shown.status)) refreshView(shown.id);
        }

        // Late replies cannot replace a newer view or undo a terminal state.
        async function refreshView(runId) {
            const mine = rc.viewSeq;
            const ask = ++rc.viewRefreshSeq;
            let detail = null;
            try { detail = await ed.api.run(runId, false); } catch (err) { return; }
            const shown = ed.runView && ed.runView.run;
            if (rc.disposed || mine !== rc.viewSeq || ask !== rc.viewRefreshSeq || !shown || shown.id !== runId || !detail || !detail.run) return;
            if (isFinal(shown.status) && !isFinal(detail.run.status)) return;
            ed.runView.run = detail.run;
            applyRunView(detail);
        }

        // loadLast fills "Last run" data for the input tree when the editor opens.
        async function loadLast() {
            try {
                const runs = (await ed.api.runs(ed.flow.id, { limit: 1 })).runs || [];
                if (!runs.length || rc.disposed) return;
                const detail = await ed.api.run(runs[0].id, false);
                if (rc.disposed) return;
                rememberRun(detail.run, detail.steps || []);
                ed.bus.emit('last-run', detail.run);
            } catch (err) { /* no history yet */ }
        }

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
            storedEffects, confirmedEffects, confirmEffects, uncheckedMarkup, effectsMarkup, trackStart, editsReady, startTest, openTest,
            baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop, runFinished, refreshShown, refreshView, loadLast,
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
