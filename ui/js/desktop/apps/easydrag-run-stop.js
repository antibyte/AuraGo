// EasyDrag run stop: Run now and posting a test run; cancelling and stopping runs from the header,
// the drawer and the run view (also runs this window did not start), with the polls that follow a
// stop and a parked run; the run view refresh and the last run for the input tree. One of the four
// run parts that create() in easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

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

        return {
            baseRevision, run, runLive, cancel, settleFinished, setStopPending, stopRun, settleDurableStop, watchParked, followStop, showsActive, confirmStop,
            runFinished, refreshShown, refreshView, loadLast
        };
    }

    (ED.runParts = ED.runParts || {}).stop = installStop;
})();
