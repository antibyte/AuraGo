// EasyDrag run stop: Run now and posting a test run; cancelling and stopping runs from the header,
// the run view (also runs this window did not start), with the polls that follow a
// stop and a parked run; the run view refresh and the last run for the input tree. One of the four
// run parts that create() in easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
// It also owns the one Stop (ED.runStop below): the header's button, for the run in focus.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FINAL = ['success', 'error', 'cancelled'];
    const ACTIVE = ['queued', 'waiting', 'running'];

    // ── the one Stop ─────────────────────────────────────────────────────────────
    // The header holds one Stop button (data-ed-cmd="cancel", in the reserved slot). It stops the run in
    // focus: the run the review shows while that run is still active and can be cancelled (a durable
    // run too: its checkpoint says so), else the editor's own run, also while a past run is open (the
    // own run is parked behind the review, ed.parkedRun). focusRun returns that run as a record that
    // stopRun takes, or null: then the slot is empty and keeps its width.

    // viewedStoppable: the rule the banner's Stop used. A durable run asks its checkpoint; a stored
    // run that has no checkpoint but says "durable" cannot be stopped from here; any other run can
    // while it is queued, waiting or running. A lost checkpoint read takes the proof away.
    function viewedStoppable(ed, run) {
        if (ed.run && ed.run.durableStatusError) return false;
        const status = ED.durable && ED.durable.info(ed.run);
        return status ? !!status.cancel_available : !run.durable && ACTIVE.includes(run.status);
    }

    // ownStoppable is the rule of runs.isRunning(): a live run, not a review, not one whose stream was given up.
    function ownStoppable(ed) {
        const run = ed.run;
        if (!run || run.view || FINAL.includes(run.status)) return false;
        return !run.stale || !!(ED.durable && ED.durable.has(run));
    }

    function focusRun(ed) {
        const view = ed.runView && ed.runView.run;
        if (view && viewedStoppable(ed, view)) return view;
        if (view) {
            const parked = ed.parkedRun;
            return parked && parked.id !== view.id ? Object.assign({ id: parked.id, status: 'running', mode: parked.mode || 'test' }, parked.record || {}) : null;
        }
        return ownStoppable(ed) ? Object.assign({ id: ed.run.id, status: ed.run.status, mode: ed.run.mode || 'test' }, ed.run.record || {}) : null;
    }

    // paint gives the Stop its words: "Stop" (always shown, at every width), "Stopping…" while the
    // request is out. The name follows the text, so a screen reader hears what the eye reads.
    function paint(button, t, pending, ed) {
        const name = t(pending ? 'easydrag.ui.run_stopping' : 'easydrag.ui.run_cancel');
        button.disabled = !!pending;
        if (pending) button.setAttribute('aria-busy', 'true'); else button.removeAttribute('aria-busy');
        button.classList.toggle('is-busy', !!pending);
        button.setAttribute('aria-label', name);
        button.setAttribute('title', name);
        const span = button.querySelector('span');
        if (span && span.textContent !== name) span.textContent = name;
        // The registry paints the availability (aria-disabled and its reason) from the same facts, now that the busy state changed.
        if (ed && ED.actions) ED.actions.paint(button, 'flow.stop', { host: ed });
    }

    // sync shows the Stop for the run in focus (or hides it) and paints its pending state; the editor's
    // header calls it on every render. pendingOf(id) tells whether the stop of that run is in flight.
    // A Stop that goes away while it has the focus hands the focus on: "Back to draft" in the review,
    // else the canvas.
    function sync(ed, button, t, pendingOf) {
        const focus = focusRun(ed);
        const hadFocus = button === document.activeElement;
        button.hidden = !focus;
        paint(button, t, !!focus && !!pendingOf(focus.id), ed);
        if (!focus && hadFocus) {
            const next = (ed.runView && ed.root.querySelector('[data-ed-cmd="exit-run-view"]')) || ed.root.querySelector('.ed-canvas');
            if (next && next.focus) next.focus({ preventScroll: true });
        }
        return focus;
    }

    // installStop installs the stop part on the run core rc and returns its functions, which
    // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
    // parts are called through rc when they run, after every part is installed.
    function installStop(rc) {
        const { ed, canvas, core, t, esc, isFinal, isActive, stopButtons, STOP_POLL_MS, PARK_POLL_MS } = rc;
        const trackStart = (...args) => rc.trackStart(...args);
        const attach = (...args) => rc.attach(...args);
        const isRunning = (...args) => rc.isRunning(...args);
        const closeStream = (...args) => rc.closeStream(...args);
        const finish = (...args) => rc.finish(...args);
        const readDurable = (...args) => rc.readDurable(...args);
        const snapshot = (...args) => rc.snapshot(...args);
        const setRun = (...args) => rc.setRun(...args);
        const streamed = (...args) => rc.streamed(...args);
        const unknownDurable = (...args) => rc.unknownDurable(...args);
        const rememberRun = (...args) => rc.rememberRun(...args);
        const loadRuns = (...args) => rc.loadRuns(...args);
        const applyRunView = (...args) => rc.applyRunView(...args);

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

        // cancel is the header Stop: it stops the run in focus (focusRun above).
        async function cancel() {
            const record = ED.runStop.focusRun(ed);
            return record ? stopRun(record) : undefined;
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

        // setStopPending shows the stop request on the one Stop, when it belongs to that run, until it settles.
        // The header paints the same state again when the run in focus changes (ED.runStop.sync).
        function setStopPending(id, on) {
            if (on) rc.pendingStops.add(id); else rc.pendingStops.delete(id);
            stopButtons(id).forEach(b => ED.runStop.paint(b, t, !!on, ed));
        }

        // stopRun stops a queued, waiting or running run (the one Stop passes the run in focus), also
        // one this window did not start. A test stops at once. Any other mode asks
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

        return {
            baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop,
            runFinished, refreshShown, refreshView, loadLast
        };
    }

    ED.runStop = { focusRun, paint, sync };
    (ED.runParts = ED.runParts || {}).stop = installStop;
})();
